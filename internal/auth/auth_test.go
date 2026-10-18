package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qmi-web/internal/database"
)

func newTestService(t *testing.T) (*Service, *database.Store, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "qmi-web.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return New(store, dir, false), store, dir
}

func TestFreshDatabaseCreatesAdmin(t *testing.T) {
	ctx := context.Background()
	service, store, _ := newTestService(t)
	if _, err := service.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	admin, err := store.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if admin.Username != DefaultUsername {
		t.Fatalf("username = %q", admin.Username)
	}
	if !VerifyPassword(DefaultPassword, admin.PasswordHash) {
		t.Fatal("default password does not verify")
	}
	if strings.Contains(admin.PasswordHash, DefaultPassword) {
		t.Fatal("database hash contains the plaintext default password")
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, "SELECT COUNT(1) FROM local_admin").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("local administrator count = %d", count)
	}
}

func TestDefaultAdminLogin(t *testing.T) {
	ctx := context.Background()
	service, store, _ := newTestService(t)
	if _, err := service.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	cookie, csrf, err := service.Login(ctx, "127.0.0.1:12345", DefaultUsername, DefaultPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure || cookie.MaxAge != int(defaultSessionLifetime.Seconds()) || len(csrf) < 32 {
		t.Fatal("session cookie protections, lifetime, or CSRF token are missing")
	}
	request := httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/dashboard", nil)
	request.AddCookie(cookie)
	session, err := service.Authenticate(ctx, request)
	if err != nil || session.Username != DefaultUsername {
		t.Fatalf("server-side session was not readable: %v", err)
	}
	if err := service.VerifyMutation(request, session); err != nil {
		t.Fatalf("GET should not require CSRF: %v", err)
	}
	request = httptest.NewRequest(http.MethodPatch, "http://example.test/api/v1/settings", nil)
	request.AddCookie(cookie)
	request.Header.Set("Origin", "http://example.test")
	request.Header.Set("X-CSRF-Token", csrf)
	if err := service.VerifyMutation(request, session); err != nil {
		t.Fatalf("valid same-origin mutation rejected: %v", err)
	}
	if _, err := store.Session(ctx, cookie.Value); err != nil {
		t.Fatalf("session is not persisted server-side: %v", err)
	}
}

func TestPasswordChangeInvalidatesSessionsAndSurvivesReopen(t *testing.T) {
	ctx := context.Background()
	service, store, dir := newTestService(t)
	if _, err := service.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	cookie, _, err := service.Login(ctx, "127.0.0.1:12345", DefaultUsername, DefaultPassword)
	if err != nil {
		t.Fatal(err)
	}
	const next = "simple7"
	if err := service.ChangePassword(ctx, DefaultPassword, next, next); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Session(ctx, cookie.Value); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("old session still exists: %v", err)
	}
	if _, _, err := service.Login(ctx, "127.0.0.1:12345", DefaultUsername, DefaultPassword); err == nil {
		t.Fatal("old password still works")
	}
	if _, _, err := service.Login(ctx, "127.0.0.1:12345", DefaultUsername, next); err != nil {
		t.Fatalf("new password login failed: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := database.Open(filepath.Join(dir, "qmi-web.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted := New(reopened, dir, false)
	if err := restarted.EnsureDefaultAdmin(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := restarted.Login(ctx, "127.0.0.1:12345", DefaultUsername, next); err != nil {
		t.Fatalf("new password did not survive reopen: %v", err)
	}
}

func TestInitializeRemovesLegacyTokenWithoutReadingIt(t *testing.T) {
	ctx := context.Background()
	service, _, dir := newTestService(t)
	path := filepath.Join(dir, legacySetupTokenName)
	if err := os.WriteFile(path, []byte("test-only-marker"), 0o600); err != nil {
		t.Fatal(err)
	}
	removed, err := service.Initialize(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("legacy token was not reported as removed")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy token remains: %v", err)
	}
}

func TestPasswordHashAndRateLimit(t *testing.T) {
	hash, err := HashPassword("very long passphrase")
	if err != nil || !VerifyPassword("very long passphrase", hash) || VerifyPassword("wrong", hash) {
		t.Fatal("Argon2id password verification failed")
	}
	service, _, _ := newTestService(t)
	for range 5 {
		service.recordFailure("127.0.0.1")
	}
	if _, _, err := service.Login(context.Background(), "127.0.0.1:42", "nobody", "anything"); err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("expected local rate limit, got %v", err)
	}
}
