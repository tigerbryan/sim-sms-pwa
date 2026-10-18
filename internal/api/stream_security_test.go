package api

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"qmi-web/internal/app"
	"qmi-web/internal/auth"
	"qmi-web/internal/database"
	"qmi-web/internal/events"
)

func TestLogStreamStopsBeforeDeliveringAfterSessionRevocation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "stream.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.CreateDefaultAdmin(ctx, "unused-test-hash"); err != nil {
		t.Fatal(err)
	}
	admin, err := store.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(ctx, "test-session", admin, "test-csrf", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	bus := events.New()
	server, err := New(&app.App{Store: store, Events: bus}, auth.New(store, dir, false))
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	client := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/api/v1/logs/stream", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "test-session"})
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("stream privacy/buffering contract: status=%d headers=%v", response.StatusCode, response.Header)
	}
	reader := bufio.NewReader(response.Body)
	if line, err := reader.ReadString('\n'); err != nil || line != "retry: 3000\n" {
		t.Fatalf("stream preamble: %q %v", line, err)
	}
	if err := store.DeleteSession(ctx, "test-session"); err != nil {
		t.Fatal(err)
	}
	bus.Publish(events.Event{Type: events.SMSReceived, Data: map[string]any{"id": 123}})
	remainder, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("revoked stream failed to close promptly: %v", err)
	}
	if strings.Contains(string(remainder), "SMSReceived") {
		t.Fatal("revoked session still received a private event")
	}
}

type failedStreamWriter struct{ header http.Header }

func (w *failedStreamWriter) Header() http.Header { return w.header }
func (*failedStreamWriter) WriteHeader(int)       {}
func (*failedStreamWriter) Flush()                {}
func (*failedStreamWriter) Write([]byte) (int, error) {
	return 0, errors.New("connection closed")
}

func TestLogStreamStopsOnFailedWrite(t *testing.T) {
	server := &Server{app: &app.App{Events: events.New()}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/stream", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		server.streamEvents(&failedStreamWriter{header: make(http.Header)}, req)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream kept waiting after a failed write")
	}
}

func TestDecodeJSONRejectsTrailingValuesAndOverLimitPadding(t *testing.T) {
	for _, body := range []string{
		`{"read":true} {"read":false}`,
		`{"read":true} trailing`,
		`{"read":true}` + strings.Repeat(" ", 64<<10),
	} {
		var input struct {
			Read bool `json:"read"`
		}
		req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(body))
		if err := decodeJSON(req, &input); err == nil {
			t.Fatal("accepted malformed or oversized request")
		}
	}
	var input struct {
		Read bool `json:"read"`
	}
	req := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader("{\"read\":true}\n "))
	if err := decodeJSON(req, &input); err != nil || !input.Read {
		t.Fatalf("valid JSON with trailing whitespace rejected: %v", err)
	}
}
