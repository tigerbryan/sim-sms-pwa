package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"qmi-web/internal/app"
	"qmi-web/internal/auth"
	"qmi-web/internal/config"
	"qmi-web/internal/database"
	"qmi-web/internal/device"
	"qmi-web/internal/logging"
	"qmi-web/internal/qmi"
	"qmi-web/internal/security"
	"qmi-web/internal/web"
)

type emptyBackend struct{}

func (emptyBackend) Name() string { return "empty" }

func (emptyBackend) Scan(context.Context) ([]device.Device, error) { return []device.Device{}, nil }

func (emptyBackend) Open(context.Context, string) (device.Modem, error) {
	return nil, device.ErrNoDevice
}

func testHandler(t *testing.T) (http.Handler, context.CancelFunc) {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "qmi-web.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cipher, err := security.LoadCipher(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Backend.Type = "mock"
	manager, err := device.NewManager("mock", qmi.NewBackend(nil), device.NewMockBackend())
	if err != nil {
		t.Fatal(err)
	}
	logger, err := logging.New(filepath.Join(dir, "logs", "qmi-web.jsonl"), "info", nil)
	if err != nil {
		t.Fatal(err)
	}
	application := app.New(cfg, store, manager, cipher, logger)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = application.Shutdown()
		_ = logger.Close()
	})
	authentication := auth.New(store, dir, false)
	if _, err := authentication.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	application.Start(ctx)
	server, err := New(application, authentication)
	if err != nil {
		t.Fatal(err)
	}
	return server.Handler(), cancel
}

func testNoHardwareHandler(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "qmi-web.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cipher, err := security.LoadCipher(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Backend.Type = "auto"
	manager, err := device.NewManager("auto", emptyBackend{}, device.NewMockBackend())
	if err != nil {
		t.Fatal(err)
	}
	logger, err := logging.New(filepath.Join(dir, "logs", "qmi-web.jsonl"), "info", nil)
	if err != nil {
		t.Fatal(err)
	}
	application := app.New(cfg, store, manager, cipher, logger)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = application.Shutdown()
		_ = logger.Close()
	})
	authentication := auth.New(store, dir, false)
	if _, err := authentication.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	application.Start(ctx)
	server, err := New(application, authentication)
	if err != nil {
		t.Fatal(err)
	}
	return server.Handler()
}

func request(t *testing.T, handler http.Handler, method, path string, payload any, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, "http://example.test"+path, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet && method != http.MethodHead {
		req.Header.Set("Origin", "http://example.test")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func login(t *testing.T, handler http.Handler, password string) (*http.Cookie, string) {
	t.Helper()
	reply := request(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": auth.DefaultUsername, "password": password,
	}, nil, "")
	if reply.Code != http.StatusOK {
		t.Fatalf("login = %d: %s", reply.Code, reply.Body.String())
	}
	var result struct {
		CSRF string `json:"csrf_token"`
	}
	if err := json.NewDecoder(reply.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	cookies := reply.Result().Cookies()
	if len(cookies) != 1 || result.CSRF == "" {
		t.Fatal("login did not establish a cookie and CSRF token")
	}
	return cookies[0], result.CSRF
}

func TestDefaultAdminLogin(t *testing.T) {
	handler, _ := testHandler(t)
	if response := request(t, handler, http.MethodGet, "/health", nil, nil, ""); response.Code != http.StatusOK {
		t.Fatalf("health = %d", response.Code)
	}
	if response := request(t, handler, http.MethodGet, "/ready", nil, nil, ""); response.Code != http.StatusOK {
		t.Fatalf("ready = %d", response.Code)
	}
	if failed := request(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": auth.DefaultUsername, "password": "incorrect",
	}, nil, ""); failed.Code != http.StatusUnauthorized {
		t.Fatalf("bad password = %d", failed.Code)
	}
	cookie, csrf := login(t, handler, auth.DefaultPassword)
	me := request(t, handler, http.MethodGet, "/api/v1/auth/me", nil, cookie, "")
	if me.Code != http.StatusOK || me.Header().Get("X-CSRF-Token") != csrf {
		t.Fatalf("auth me = %d, csrf header present=%v", me.Code, me.Header().Get("X-CSRF-Token") != "")
	}
	var body map[string]any
	if err := json.NewDecoder(me.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["authenticated"] != true || body["username"] != auth.DefaultUsername || len(body) != 2 {
		t.Fatalf("unexpected auth state: %#v", body)
	}
}

func TestUnauthenticatedDashboard(t *testing.T) {
	handler, _ := testHandler(t)
	if dashboard := request(t, handler, http.MethodGet, "/api/v1/dashboard", nil, nil, ""); dashboard.Code != http.StatusUnauthorized {
		t.Fatalf("dashboard without session = %d", dashboard.Code)
	}
}

func TestDevicesNoHardware(t *testing.T) {
	handler := testNoHardwareHandler(t)
	if unauthenticated := request(t, handler, http.MethodGet, "/api/v1/devices", nil, nil, ""); unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("devices without session = %d", unauthenticated.Code)
	}
	cookie, _ := login(t, handler, auth.DefaultPassword)
	reply := request(t, handler, http.MethodGet, "/api/v1/devices", nil, cookie, "")
	if reply.Code != http.StatusOK {
		t.Fatalf("devices no-hardware = %d: %s", reply.Code, reply.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.NewDecoder(reply.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	var devices []device.Device
	if err := json.Unmarshal(body["devices"], &devices); err != nil {
		t.Fatalf("decode devices: %v", err)
	}
	if len(devices) != 0 || string(body["devices"]) != "[]" {
		t.Fatalf("no-device response must be devices:[]; got %s", body["devices"])
	}
	for _, forbidden := range []string{"version", "commit", "go_version"} {
		if _, exists := body[forbidden]; exists {
			t.Fatalf("device response leaked %q: %s", forbidden, reply.Body.String())
		}
	}
}

func TestAuthenticatedDashboard(t *testing.T) {
	handler, _ := testHandler(t)
	cookie, csrf := login(t, handler, auth.DefaultPassword)
	if dashboard := request(t, handler, http.MethodGet, "/api/v1/dashboard", nil, cookie, ""); dashboard.Code != http.StatusOK {
		t.Fatalf("dashboard = %d: %s", dashboard.Code, dashboard.Body.String())
	}
	if settings := request(t, handler, http.MethodPatch, "/api/v1/settings", map[string]any{
		"general": map[string]string{"scan_interval": "20s"},
		"logging": map[string]string{"level": "debug"},
	}, cookie, csrf); settings.Code != http.StatusOK {
		t.Fatalf("runtime settings = %d: %s", settings.Code, settings.Body.String())
	}
	for _, path := range []string{
		"/api/v1/dial",
		"/api/v1/start-network",
		"/api/v1/connect-network",
		"/api/v1/wds-start",
		"/api/v1/apn",
	} {
		if unsafe := request(t, handler, http.MethodPost, path, map[string]any{}, cookie, csrf); unsafe.Code != http.StatusNotFound {
			t.Fatalf("unsafe route %s unexpectedly exists: %d %s", path, unsafe.Code, unsafe.Body.String())
		}
	}
	if badCSRF := request(t, handler, http.MethodPatch, "/api/v1/settings", map[string]any{}, cookie, "bad"); badCSRF.Code != http.StatusForbidden {
		t.Fatalf("bad CSRF = %d", badCSRF.Code)
	}
}

func TestPasswordChange(t *testing.T) {
	handler, _ := testHandler(t)
	cookie, csrf := login(t, handler, auth.DefaultPassword)
	const next = "simple7"
	changed := request(t, handler, http.MethodPost, "/api/v1/auth/password", map[string]string{
		"current_password": auth.DefaultPassword,
		"new_password":     next,
		"confirmation":     next,
	}, cookie, csrf)
	if changed.Code != http.StatusOK {
		t.Fatalf("password change = %d: %s", changed.Code, changed.Body.String())
	}
	if dashboard := request(t, handler, http.MethodGet, "/api/v1/dashboard", nil, cookie, ""); dashboard.Code != http.StatusUnauthorized {
		t.Fatalf("old session after password change = %d", dashboard.Code)
	}
	if old := request(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": auth.DefaultUsername, "password": auth.DefaultPassword,
	}, nil, ""); old.Code != http.StatusUnauthorized {
		t.Fatalf("old password after change = %d", old.Code)
	}
	login(t, handler, next)
}

func TestNoSetupAPI(t *testing.T) {
	handler, _ := testHandler(t)
	for _, path := range []string{
		"/api/v1/auth/setup",
		"/api/v1/auth/setup/status",
		"/api/v1/setup",
		"/api/v1/setup/status",
		"/api/v1/bootstrap",
	} {
		if response := request(t, handler, http.MethodPost, path, map[string]any{}, nil, ""); response.Code != http.StatusNotFound {
			t.Fatalf("%s = %d", path, response.Code)
		}
	}
}

func TestRoutesContract(t *testing.T) {
	handler, _ := testHandler(t)
	cookie, csrf := login(t, handler, auth.DefaultPassword)
	for _, item := range []struct {
		method string
		path   string
		body   any
		status int
	}{
		{http.MethodGet, "/health", nil, http.StatusOK},
		{http.MethodGet, "/ready", nil, http.StatusOK},
		{http.MethodGet, "/version", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/dashboard", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/devices", nil, http.StatusOK},
		{http.MethodPost, "/api/v1/devices/scan", map[string]any{}, http.StatusOK},
		{http.MethodGet, "/api/v1/sim", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/signal", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/sms?page=1&page_size=10", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/settings", nil, http.StatusOK},
		{http.MethodPatch, "/api/v1/settings", map[string]any{"logging": map[string]string{"level": "info"}}, http.StatusOK},
		{http.MethodGet, "/api/v1/notifications", nil, http.StatusOK},
		{http.MethodPost, "/api/v1/notifications/pushplus/test", map[string]any{}, http.StatusBadRequest},
		{http.MethodGet, "/api/v1/logs?limit=10", nil, http.StatusOK},
		{http.MethodGet, "/api/v1/diagnostics", nil, http.StatusOK},
	} {
		reply := request(t, handler, item.method, item.path, item.body, cookie, csrf)
		if reply.Code != item.status {
			t.Fatalf("%s %s = %d, want %d: %s", item.method, item.path, reply.Code, item.status, reply.Body.String())
		}
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to locate route contract test source")
	}
	want, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "docs", "ROUTES.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != RouteReport() {
		t.Fatal("docs/ROUTES.md is not generated from RouteContracts")
	}
}

func TestSessionCSRFAndOriginForScan(t *testing.T) {
	handler, _ := testHandler(t)
	cookie, csrf := login(t, handler, auth.DefaultPassword)
	if accepted := request(t, handler, http.MethodPost, "/api/v1/devices/scan", map[string]any{}, cookie, csrf); accepted.Code != http.StatusOK {
		t.Fatalf("scan with session and csrf = %d: %s", accepted.Code, accepted.Body.String())
	}
	req := httptest.NewRequest(http.MethodPost, "http://example.test/api/v1/devices/scan", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("X-CSRF-Token", csrf)
	req.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("scan with malicious origin = %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestVersionAndStaticCacheContract(t *testing.T) {
	handler, _ := testHandler(t)
	versionReply := request(t, handler, http.MethodGet, "/version", nil, nil, "")
	if versionReply.Code != http.StatusOK {
		t.Fatalf("version = %d", versionReply.Code)
	}
	var versionBody map[string]json.RawMessage
	if err := json.NewDecoder(versionReply.Body).Decode(&versionBody); err != nil {
		t.Fatal(err)
	}
	if _, ok := versionBody["version"]; !ok {
		t.Fatal("version response has no version field")
	}
	if _, ok := versionBody["devices"]; ok {
		t.Fatal("version response must not contain devices")
	}
	if versionReply.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("version cache policy = %q", versionReply.Header().Get("Cache-Control"))
	}
	if index := request(t, handler, http.MethodGet, "/", nil, nil, ""); index.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("index cache policy = %q", index.Header().Get("Cache-Control"))
	}
	entries, err := fs.ReadDir(web.Assets, "dist/assets")
	if err != nil {
		t.Fatal(err)
	}
	asset := ""
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".js") || strings.HasSuffix(entry.Name(), ".css") {
			asset = entry.Name()
			break
		}
	}
	if asset == "" {
		t.Fatal("no hashed static asset embedded for cache test")
	}
	assetReply := request(t, handler, http.MethodGet, "/assets/"+asset, nil, nil, "")
	if assetReply.Code != http.StatusOK || assetReply.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("asset cache contract = %d %q", assetReply.Code, assetReply.Header().Get("Cache-Control"))
	}
}

func TestLogStreamSendsSSEPreamble(t *testing.T) {
	handler, _ := testHandler(t)
	cookie, _ := login(t, handler, auth.DefaultPassword)
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "http://example.test/api/v1/logs/stream", nil).WithContext(ctx)
	req.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(recorder, req)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SSE request did not stop with its context")
	}
	if recorder.Header().Get("Content-Type") != "text/event-stream" || !strings.Contains(recorder.Body.String(), "retry: 3000") {
		t.Fatalf("unexpected SSE response: headers=%v body=%q", recorder.Header(), recorder.Body.String())
	}
}
