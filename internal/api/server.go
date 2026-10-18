// Package api exposes the same-origin HTTP API and embedded WebUI.
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"qmi-web/internal/app"
	"qmi-web/internal/auth"
	"qmi-web/internal/database"
	"qmi-web/internal/notify"
	"qmi-web/internal/version"
	"qmi-web/internal/web"
)

type Server struct {
	app    *app.App
	auth   *auth.Service
	static fs.FS
}

func New(application *app.App, authentication *auth.Service) (*Server, error) {
	static, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		return nil, err
	}
	return &Server{app: application, auth: authentication, static: static}, nil
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Frame-Options", "DENY")
	if r.URL.Path == "/health" && r.Method == http.MethodGet {
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}
	if r.URL.Path == "/ready" && r.Method == http.MethodGet {
		w.Header().Set("Cache-Control", "no-store")
		if err := s.app.Store.Healthy(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "not_ready", "database is not ready")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ready": true, "device_required": false})
		return
	}
	if r.URL.Path == "/version" && r.Method == http.MethodGet {
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, version.Current())
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		w.Header().Set("Cache-Control", "no-store")
		s.handleAPI(w, r)
		return
	}
	s.serveStatic(w, r)
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	if isRetiredSetupRoute(r.URL.Path) {
		writeError(w, http.StatusNotFound, "not_found", "API endpoint not found")
		return
	}
	if r.URL.Path == "/api/v1/auth/login" && r.Method == http.MethodPost {
		if err := requireOrigin(r); err != nil {
			writeError(w, http.StatusForbidden, "origin_invalid", err.Error())
			return
		}
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid login request")
			return
		}
		cookie, csrf, err := s.auth.Login(r.Context(), r.RemoteAddr, input.Username, input.Password)
		if err != nil {
			status := http.StatusUnauthorized
			if strings.Contains(err.Error(), "too many") {
				status = http.StatusTooManyRequests
			}
			writeError(w, status, "login_failed", err.Error())
			return
		}
		http.SetCookie(w, cookie)
		s.app.Store.AddAudit(r.Context(), "login", input.Username, r.RemoteAddr, "{}")
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "csrf_token": csrf})
		return
	}
	session, err := s.auth.Authenticate(r.Context(), r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}
	if r.URL.Path == "/api/v1/auth/me" && r.Method == http.MethodGet {
		w.Header().Set("X-CSRF-Token", session.CSRFToken)
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": auth.DefaultUsername})
		return
	}
	if err := s.auth.VerifyMutation(r, session); err != nil {
		writeError(w, http.StatusForbidden, "csrf_failed", err.Error())
		return
	}
	if r.URL.Path == "/api/v1/auth/logout" && r.Method == http.MethodPost {
		http.SetCookie(w, s.auth.Logout(r.Context(), r))
		writeJSON(w, http.StatusOK, map[string]any{"logged_out": true})
		return
	}
	if r.URL.Path == "/api/v1/auth/password" && r.Method == http.MethodPost {
		var input struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
			Confirmation    string `json:"confirmation"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid password change request")
			return
		}
		if err := s.auth.ChangePassword(r.Context(), input.CurrentPassword, input.NewPassword, input.Confirmation); err != nil {
			writeError(w, http.StatusBadRequest, "password_change_failed", err.Error())
			return
		}
		http.SetCookie(w, s.auth.Logout(r.Context(), r))
		s.app.Store.AddAudit(r.Context(), "admin_password_changed", session.Username, r.RemoteAddr, "{}")
		writeJSON(w, http.StatusOK, map[string]any{"password_changed": true})
		return
	}
	switch {
	case r.URL.Path == "/api/v1/dashboard" && r.Method == http.MethodGet:
		dashboard, err := s.app.Dashboard(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "dashboard_failed", "unable to load dashboard")
			return
		}
		writeJSON(w, http.StatusOK, dashboard)
	case r.URL.Path == "/api/v1/devices" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, s.app.DevicesSnapshot())
	case r.URL.Path == "/api/v1/devices/scan" && r.Method == http.MethodPost:
		_, err := s.app.Devices.Scan(r.Context())
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "scan_failed", "device scan failed")
			return
		}
		s.app.Store.AddAudit(r.Context(), "device_scan", session.Username, r.RemoteAddr, "{}")
		writeJSON(w, http.StatusOK, s.app.DevicesSnapshot())
	case r.URL.Path == "/api/v1/sim" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, s.app.SIMSnapshot())
	case r.URL.Path == "/api/v1/signal" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, s.app.SignalSnapshot())
	case r.URL.Path == "/api/v1/sms" && r.Method == http.MethodGet:
		s.listSMS(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/v1/sms/"):
		s.smsDetail(w, r)
	case r.URL.Path == "/api/v1/settings" && r.Method == http.MethodGet:
		s.settings(w, r)
	case r.URL.Path == "/api/v1/settings" && r.Method == http.MethodPatch:
		s.updateSettings(w, r, session)
	case strings.HasPrefix(r.URL.Path, "/api/v1/webpush/"):
		s.webPush(w, r)
	case r.URL.Path == "/api/v1/notifications" && r.Method == http.MethodGet:
		s.notifications(w, r)
	case r.URL.Path == "/api/v1/notifications/pushplus/test" && r.Method == http.MethodPost:
		if err := s.app.Notifications.SendTest(r.Context()); err != nil {
			writeError(w, http.StatusBadRequest, "notification_failed", err.Error())
			return
		}
		s.app.Store.AddAudit(r.Context(), "pushplus_test", session.Username, r.RemoteAddr, "{}")
		writeJSON(w, http.StatusOK, map[string]any{"sent": true})
	case r.URL.Path == "/api/v1/logs" && r.Method == http.MethodGet:
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		writeJSON(w, http.StatusOK, map[string]any{"items": s.app.Log.Recent(limit)})
	case r.URL.Path == "/api/v1/logs/stream" && r.Method == http.MethodGet:
		s.streamEvents(w, r)
	case r.URL.Path == "/api/v1/diagnostics" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, s.app.Diagnostics(r.Context()))
	default:
		writeError(w, http.StatusNotFound, "not_found", "API endpoint not found")
	}
}

func (s *Server) listSMS(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if perPage == 0 {
		perPage, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	}
	items, total, err := s.app.Store.ListSMS(r.Context(), page, perPage, r.URL.Query().Get("q"), r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sms_query_failed", "unable to list SMS")
		return
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}
	unread, err := s.app.Store.UnreadSMSCount(r.Context())
	if err != nil {
		writeError(w, 500, "sms_query_failed", "unable to count unread SMS")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unread_total": unread, "items": items, "page": page, "page_size": perPage, "per_page": perPage, "total": total})
}

func (s *Server) smsDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/sms/")
	parts := strings.Split(path, "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_id", "SMS id is invalid")
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		item, err := s.app.Store.SMS(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "SMS not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "sms_query_failed", "unable to read SMS")
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 2 && parts[1] == "read" && r.Method == http.MethodPatch {
		var input struct {
			Read bool `json:"read"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid read state")
			return
		}
		if err := s.app.Store.MarkSMSRead(r.Context(), id, input.Read); errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "SMS not found")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "sms_update_failed", "unable to update SMS")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": true})
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "API endpoint not found")
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	push, err := s.app.Notifications.Config(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_failed", "unable to load settings")
		return
	}
	runtime := s.app.RuntimeSettings()
	writeJSON(w, http.StatusOK, map[string]any{
		"general":  map[string]any{"backend": runtime.Backend, "scan_interval": runtime.ScanInterval.String(), "backend_restart_required": true},
		"security": map[string]any{"sms_only": true, "immutable": true, "message": "SMS-only mode does not support mobile data."},
		"sms":      map[string]any{"sending_enabled": false, "store_raw_pdu": false},
		"pushplus": push, "logging": map[string]any{"level": runtime.LogLevel},
	})
}

func (s *Server) notifications(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.app.Store.ListNotifications(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "notifications_query_failed", "unable to list notification metadata")
		return
	}
	push, err := s.app.Notifications.Config(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "notifications_query_failed", "unable to load notification settings")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		PushPlus notify.PublicConfig     `json:"pushplus"`
		Items    []database.Notification `json:"items"`
	}{PushPlus: push, Items: items})
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request, session database.Session) {
	var input struct {
		General *struct {
			ScanInterval string `json:"scan_interval"`
		} `json:"general"`
		Logging *struct {
			Level string `json:"level"`
		} `json:"logging"`
		PushPlus *struct {
			Enabled  bool   `json:"enabled"`
			Token    string `json:"token"`
			Template string `json:"template"`
		} `json:"pushplus"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid settings request")
		return
	}
	if input.General == nil && input.Logging == nil && input.PushPlus == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "no supported settings were supplied")
		return
	}
	scanInterval, logLevel := "", ""
	if input.General != nil {
		scanInterval = input.General.ScanInterval
	}
	if input.Logging != nil {
		logLevel = input.Logging.Level
	}
	if err := s.app.UpdateRuntimeSettings(scanInterval, logLevel); err != nil {
		writeError(w, http.StatusBadRequest, "settings_rejected", err.Error())
		return
	}
	if input.PushPlus != nil {
		if err := s.app.Notifications.Update(r.Context(), input.PushPlus.Enabled, input.PushPlus.Token, input.PushPlus.Template); err != nil {
			writeError(w, http.StatusBadRequest, "settings_rejected", err.Error())
			return
		}
	}
	s.app.Store.AddAudit(r.Context(), "settings_update", session.Username, r.RemoteAddr, `{"section":"pushplus"}`)
	s.settings(w, r)
}

func (s *Server) streamEvents(w http.ResponseWriter, r *http.Request) {
	_, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_unsupported", "streaming is unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")
	controller := http.NewResponseController(w)
	defer controller.SetWriteDeadline(time.Time{})
	write := func(message string) bool {
		// Bound a stalled client's writes without imposing a lifetime on SSE.
		_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := io.WriteString(w, message); err != nil {
			return false
		}
		return controller.Flush() == nil
	}
	ch, unsubscribe := s.app.Events.Subscribe(32)
	defer unsubscribe()
	if !write("retry: 3000\n\n") {
		return
	}
	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			if _, err := s.auth.Authenticate(r.Context(), r); err != nil {
				return
			}
			if !write(": keepalive\n\n") {
				return
			}
		case event, ok := <-ch:
			if !ok {
				return
			}
			// An open stream must stop disclosing events after logout, session
			// expiry, or a password change that revokes its session.
			if _, err := s.auth.Authenticate(r.Context(), r); err != nil {
				return
			}
			payload, _ := json.Marshal(event)
			if !write(fmt.Sprintf("event: %s\ndata: %s\n\n", event.Type, payload)) {
				return
			}
		}
	}
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Cache-Control", "no-store")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method is not allowed")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	content, err := fs.ReadFile(s.static, path)
	servedPath := path
	if err != nil {
		content, err = fs.ReadFile(s.static, "index.html")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "webui_unavailable", "WebUI assets are unavailable")
			return
		}
		servedPath = "index.html"
	}
	if servedPath == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
	} else if immutableStaticAsset(servedPath) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	if filepath.Ext(servedPath) == ".webmanifest" {
		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	} else if contentType := mime.TypeByExtension(filepath.Ext(servedPath)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else if servedPath == "index.html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	_, _ = w.Write(content)
}

func immutableStaticAsset(path string) bool {
	if !strings.HasPrefix(path, "assets/") {
		return false
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return strings.Contains(name, "-")
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(noopResponseWriter{}, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func requireOrigin(r *http.Request) error {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return errors.New("Origin header is required")
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host != r.Host {
		return errors.New("Origin is not allowed")
	}
	return nil
}

func isRetiredSetupRoute(path string) bool {
	if path == "/api/v1/bootstrap" {
		return true
	}
	return strings.HasPrefix(path, "/api/v1/setup") || strings.HasPrefix(path, "/api/v1/auth/setup")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "details": map[string]any{}}})
}

type noopResponseWriter struct{}

func (noopResponseWriter) Header() http.Header       { return http.Header{} }
func (noopResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (noopResponseWriter) WriteHeader(int)           {}
