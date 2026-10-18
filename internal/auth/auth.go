// Package auth implements a single local administrator, Argon2id passwords,
// server-side sessions, and same-origin mutation protection. It deliberately
// does not issue JWTs or expose a user-management surface.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"qmi-web/internal/database"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

const (
	CookieName             = "qmi_web_session"
	DefaultUsername        = "admin"
	DefaultPassword        = "admin"
	legacySetupTokenName   = "setup-token"
	defaultSessionLifetime = 30 * 24 * time.Hour
)

type Service struct {
	store        *database.Store
	dataDir      string
	sessionTTL   time.Duration
	mu           sync.Mutex
	failures     map[string][]time.Time
	cookieSecure bool
}

func New(store *database.Store, dataDir string, cookieSecure bool) *Service {
	return &Service{
		store: store, dataDir: dataDir, sessionTTL: defaultSessionLifetime,
		failures: make(map[string][]time.Time), cookieSecure: cookieSecure,
	}
}

// Initialize creates the fixed local administrator only when none exists and
// removes the exact obsolete setup token file without reading its content.
func (s *Service) Initialize(ctx context.Context) (bool, error) {
	if err := s.EnsureDefaultAdmin(ctx); err != nil {
		return false, err
	}
	path := filepath.Join(s.dataDir, legacySetupTokenName)
	if err := os.Remove(path); err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		return false, err
	}
}

func (s *Service) EnsureDefaultAdmin(ctx context.Context) error {
	has, err := s.store.HasAdmin(ctx)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	hash, err := HashPassword(DefaultPassword)
	if err != nil {
		return err
	}
	_, err = s.store.CreateDefaultAdmin(ctx, hash)
	return err
}

func (s *Service) Login(ctx context.Context, remoteAddr, username, password string) (*http.Cookie, string, error) {
	ip := clientIP(remoteAddr)
	if s.rateLimited(ip) {
		return nil, "", errors.New("too many login attempts; retry later")
	}
	if strings.TrimSpace(username) != DefaultUsername {
		s.recordFailure(ip)
		return nil, "", errors.New("invalid username or password")
	}
	admin, err := s.store.Admin(ctx)
	if err != nil || !VerifyPassword(password, admin.PasswordHash) {
		s.recordFailure(ip)
		return nil, "", errors.New("invalid username or password")
	}
	s.clearFailures(ip)
	id, err := randomToken(32)
	if err != nil {
		return nil, "", err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return nil, "", err
	}
	if err := s.store.CreateSession(ctx, id, admin, csrf, time.Now().UTC().Add(s.sessionTTL)); err != nil {
		return nil, "", err
	}
	if err := s.store.RecordAdminLogin(ctx); err != nil {
		_ = s.store.DeleteSession(ctx, id)
		return nil, "", err
	}
	return s.sessionCookie(id, false), csrf, nil
}

// ChangePassword verifies the current password, updates the Argon2id hash, and
// invalidates every server-side session, including the current request's one.
func (s *Service) ChangePassword(ctx context.Context, current, next, confirmation string) error {
	if next != confirmation {
		return errors.New("new password confirmation does not match")
	}
	if err := validateNewPassword(next); err != nil {
		return err
	}
	admin, err := s.store.Admin(ctx)
	if err != nil || !VerifyPassword(current, admin.PasswordHash) {
		return errors.New("current password is invalid")
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	return s.store.UpdateAdminPassword(ctx, hash)
}

// ResetAdminPassword is reserved for the NAS-local recovery CLI. It never logs
// or stores the supplied plaintext beyond the Argon2id calculation.
func (s *Service) ResetAdminPassword(ctx context.Context, password string) error {
	if password != DefaultPassword {
		if err := validateNewPassword(password); err != nil {
			return err
		}
	}
	if err := s.EnsureDefaultAdmin(ctx); err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.store.UpdateAdminPassword(ctx, hash)
}

func (s *Service) Logout(ctx context.Context, r *http.Request) *http.Cookie {
	if c, err := r.Cookie(CookieName); err == nil {
		_ = s.store.DeleteSession(ctx, c.Value)
	}
	return s.sessionCookie("", true)
}

func (s *Service) Authenticate(ctx context.Context, r *http.Request) (database.Session, error) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return database.Session{}, errors.New("authentication required")
	}
	session, err := s.store.Session(ctx, c.Value)
	if errors.Is(err, sql.ErrNoRows) {
		return database.Session{}, errors.New("session expired")
	}
	return session, err
}

func (s *Service) VerifyMutation(r *http.Request, session database.Session) error {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return nil
	}
	if token := r.Header.Get("X-CSRF-Token"); token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) != 1 {
		return errors.New("CSRF token is invalid")
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return errors.New("Origin header is required")
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host != r.Host {
		return errors.New("Origin is not allowed")
	}
	return nil
}

func (s *Service) sessionCookie(value string, expired bool) *http.Cookie {
	cookie := &http.Cookie{
		Name: CookieName, Value: value, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: s.cookieSecure,
	}
	if expired {
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0).UTC()
		return cookie
	}
	cookie.MaxAge = int(s.sessionTTL.Seconds())
	cookie.Expires = time.Now().UTC().Add(s.sessionTTL)
	return cookie
}

func (s *Service) rateLimited(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	valid := s.recentFailuresLocked(ip, now)
	s.failures[ip] = valid
	return len(valid) >= 5
}

func (s *Service) recordFailure(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures[ip] = append(s.recentFailuresLocked(ip, time.Now()), time.Now())
}

func (s *Service) clearFailures(ip string) {
	s.mu.Lock()
	delete(s.failures, ip)
	s.mu.Unlock()
}

func (s *Service) recentFailuresLocked(ip string, now time.Time) []time.Time {
	var valid []time.Time
	for _, at := range s.failures[ip] {
		if now.Sub(at) < 5*time.Minute {
			valid = append(valid, at)
		}
	}
	return valid
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
	return "argon2id$v=19$m=65536,t=3,p=4$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash), nil
}

func VerifyPassword(password, encoded string) bool {
	// Preserve the password set in the previous local Kalkun inbox.
	if strings.HasPrefix(encoded, "$2y$") || strings.HasPrefix(encoded, "$2b$") || strings.HasPrefix(encoded, "$2a$") {
		return bcrypt.CompareHashAndPassword([]byte(encoded), []byte(password)) == nil
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=19" || parts[2] != "m=65536,t=3,p=4" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func validateNewPassword(password string) error {
	if len(password) < 6 || len(password) > 256 {
		return errors.New("password must be 6 to 256 characters")
	}
	return nil
}

func randomToken(bytes int) (string, error) {
	raw := make([]byte, bytes)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
