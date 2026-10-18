package api

import (
	"net/http"
	"qmi-web/internal/auth"
	"testing"
)

func TestWebPushRoutesRequireSessionAndCSRF(t *testing.T) {
	handler, cancel := testHandler(t)
	defer cancel()
	if r := request(t, handler, http.MethodGet, "/api/v1/webpush/status", nil, nil, ""); r.Code != 401 {
		t.Fatal("VAPID API allowed without login")
	}
	cookie, csrf := login(t, handler, auth.DefaultPassword)
	if r := request(t, handler, http.MethodGet, "/api/v1/webpush/status", nil, cookie, ""); r.Code != 200 {
		t.Fatalf("status: %d", r.Code)
	}
	for _, item := range []struct{ method, path string }{{"POST", "/api/v1/webpush/subscribe"}, {"DELETE", "/api/v1/webpush/subscribe"}, {"POST", "/api/v1/webpush/test"}} {
		if r := request(t, handler, item.method, item.path, map[string]any{}, cookie, ""); r.Code != 403 {
			t.Fatalf("%s bypassed CSRF: %d", item.path, r.Code)
		}
	}
	if r := request(t, handler, "POST", "/api/v1/webpush/test", map[string]any{"id": "nonexistent"}, cookie, csrf); r.Code != 400 {
		t.Fatal("accepted test for nonexistent device")
	}
}
