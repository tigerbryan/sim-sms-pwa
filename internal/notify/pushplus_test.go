package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"qmi-web/internal/database"
	"qmi-web/internal/security"
)

func TestPushPlusNotifierUsesJSONAndRejectsProviderErrors(t *testing.T) {
	var received map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		_, _ = w.Write([]byte(`{"code":200,"msg":"ok"}`))
	}))
	defer server.Close()
	notifier := &PushPlusNotifier{
		Endpoint: server.URL,
		Token:    func(context.Context) (string, error) { return "test-token", nil },
		Template: func(context.Context) (string, error) { return "txt", nil },
	}
	if err := notifier.Send(context.Background(), NotificationEvent{Title: "title", Body: "body"}); err != nil {
		t.Fatal(err)
	}
	if received["token"] != "test-token" || received["content"] != "body" || received["template"] != "txt" {
		t.Fatalf("unexpected PushPlus request: %#v", received)
	}
}

func TestNotificationGuardBlocksOutboundDelivery(t *testing.T) {
	store, err := database.Open(filepath.Join(t.TempDir(), "qmi-web.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cipher, err := security.LoadCipher(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, cipher, func(context.Context) security.GuardState {
		return security.GuardState{State: "critical", Findings: []security.Finding{{Code: "CellularDefaultRoute"}}}
	})
	if err := service.Update(context.Background(), true, "test-token", "html"); err != nil {
		t.Fatal(err)
	}
	if err := service.SendTest(context.Background()); err == nil {
		t.Fatal("critical data guard did not suppress PushPlus")
	}
}

func TestQueueDeliversOnceAndMarksSuccess(t *testing.T) {
	store, err := database.Open(filepath.Join(t.TempDir(), "qmi-web.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cipher, err := security.LoadCipher(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"code":200}`))
	}))
	defer server.Close()
	service := NewService(store, cipher, func(context.Context) security.GuardState {
		return security.GuardState{State: "safe"}
	})
	service.push.Endpoint = server.URL
	if err := service.Update(context.Background(), true, "test-token", "html"); err != nil {
		t.Fatal(err)
	}
	if created, err := service.Enqueue(context.Background(), NotificationEvent{Kind: "sms", Title: "new", Body: "body", DedupKey: "one"}); err != nil || !created {
		t.Fatalf("enqueue = %v %v", created, err)
	}
	if created, err := service.Enqueue(context.Background(), NotificationEvent{Kind: "sms", Title: "new", Body: "body", DedupKey: "one"}); err != nil || created {
		t.Fatalf("dedup enqueue = %v %v", created, err)
	}
	service.deliverDue(context.Background())
	var status string
	if err := store.DB().QueryRow("SELECT status FROM notifications WHERE dedup_key='one'").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "success" || calls != 1 {
		t.Fatalf("queue delivery status=%s calls=%d", status, calls)
	}
}
