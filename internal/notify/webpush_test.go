package notify

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	webpush "github.com/SherClockHolmes/webpush-go"
	"golang.org/x/crypto/hkdf"
	"io"
	"net/http"
	"path/filepath"
	"qmi-web/internal/database"
	"qmi-web/internal/security"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

type pushTransport func(*http.Request) (*http.Response, error)

func (f pushTransport) Do(r *http.Request) (*http.Response, error) { return f(r) }
func fixturePush(t *testing.T, endpoint string) (PushSubscription, *ecdh.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := bytes.Repeat([]byte{42}, 16)
	return PushSubscription{Endpoint: endpoint, Keys: webpush.Keys{P256dh: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), Auth: base64.RawURLEncoding.EncodeToString(auth)}}, key, auth
}
func setupPush(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	store, err := database.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	cipher, err := security.LoadCipher(dir)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(store, cipher, func(context.Context) security.GuardState { return security.GuardState{State: "safe"} })
}
func decryptPush(t *testing.T, body []byte, key *ecdh.PrivateKey, auth []byte) map[string]any {
	t.Helper()
	if len(body) < 86 {
		t.Fatal("short web push payload")
	}
	salt := body[:16]
	pub := body[21 : 21+int(body[20])]
	peer, err := ecdh.P256().NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := key.ECDH(peer)
	if err != nil {
		t.Fatal(err)
	}
	derive := func(secret, salt, info []byte, n int) []byte {
		out := make([]byte, n)
		if _, e := io.ReadFull(hkdf.New(sha256.New, secret, salt, info), out); e != nil {
			t.Fatal(e)
		}
		return out
	}
	info := append([]byte("WebPush: info\x00"), key.PublicKey().Bytes()...)
	info = append(info, pub...)
	ikm := derive(shared, auth, info, 32)
	cek := derive(ikm, salt, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := derive(ikm, salt, []byte("Content-Encoding: nonce\x00"), 12)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, body[21+int(body[20]):], nil)
	if err != nil {
		t.Fatal(err)
	}
	plain = bytes.TrimRight(plain, "\x00")
	if plain[len(plain)-1] != 2 {
		t.Fatal("wrong payload delimiter")
	}
	plain = plain[:len(plain)-1]
	var result map[string]any
	if err = json.Unmarshal(plain, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestNativePushIndependentEncryptedAndDeduplicated(t *testing.T) {
	s := setupPush(t)
	ctx := context.Background()
	sub, key, auth := fixturePush(t, "https://web.push.apple.com/test-only")
	id, err := s.WebPush.Subscribe(ctx, sub)
	if err != nil {
		t.Fatal(err)
	}
	var encrypted string
	s.store.DB().QueryRow("SELECT encrypted FROM web_push_subscriptions WHERE id=?", id).Scan(&encrypted)
	if strings.Contains(encrypted, sub.Endpoint) || strings.Contains(encrypted, sub.Keys.Auth) {
		t.Fatal("subscription secret stored in plaintext")
	}
	var calls int
	s.WebPush.client = pushTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Content-Encoding") != "aes128gcm" || r.Header.Get("Urgency") != "high" || r.Header.Get("Authorization") == "" {
			t.Fatal("missing web push headers")
		}
		raw, _ := io.ReadAll(r.Body)
		decoded := decryptPush(t, raw, key, auth)
		if decoded["url"] != "/sms" || decoded["title"] != "演示联系人" || decoded["body"] != "示例短信：今天见 🙂" || decoded["tag"] != "sms:1" {
			t.Fatalf("unexpected SMS preview: %v", decoded)
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	event := NotificationEvent{Kind: "sms", DedupKey: "sms:1", Sender: "演示联系人", Text: "示例短信：今天见 🙂", Body: "legacy metadata must not appear in native preview"}
	created, err := s.Enqueue(ctx, event)
	if err != nil || !created {
		t.Fatalf("native enqueue with PushPlus off: %v %v", created, err)
	}
	var encryptedPayload string
	if err := s.store.DB().QueryRow("SELECT payload FROM web_push_jobs").Scan(&encryptedPayload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encryptedPayload, event.Text) || strings.Contains(encryptedPayload, event.Sender) {
		t.Fatal("SMS preview stored in plaintext")
	}
	created, err = s.Enqueue(ctx, event)
	if err != nil || created {
		t.Fatal("duplicate SMS queued twice")
	}
	s.WebPush.deliverDue(ctx)
	s.WebPush.deliverDue(ctx)
	if calls != 1 {
		t.Fatalf("sent %d times", calls)
	}
	var status string
	s.store.DB().QueryRow("SELECT status FROM web_push_jobs").Scan(&status)
	if status != "success" {
		t.Fatal(status)
	}
	first, _ := s.WebPush.keys(ctx)
	other := newWebPush(s.store, s.cipher, s.guard)
	second, _ := other.keys(ctx)
	if first != second {
		t.Fatal("VAPID identity did not survive service recreation")
	}
}
func TestWebPushRetriesAndRemovesExpiredSubscriptionIndependently(t *testing.T) {
	s := setupPush(t)
	ctx := context.Background()
	a, _, _ := fixturePush(t, "https://web.push.apple.com/expired")
	b, _, _ := fixturePush(t, "https://fcm.googleapis.com/retry")
	idA, _ := s.WebPush.Subscribe(ctx, a)
	idB, _ := s.WebPush.Subscribe(ctx, b)
	calls := map[string]int{}
	s.WebPush.client = pushTransport(func(r *http.Request) (*http.Response, error) {
		calls[r.URL.Path]++
		code := 410
		if r.URL.Path == "/retry" {
			code = 503
			if calls[r.URL.Path] > 1 {
				code = 201
			}
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	s.Enqueue(ctx, NotificationEvent{Kind: "sms", DedupKey: "sms:2"})
	s.WebPush.deliverDue(ctx)
	aStatus, _ := s.WebPush.Status(ctx, idA)
	if aStatus["subscribed"] != false {
		t.Fatal("expired device retained")
	}
	var status string
	s.store.DB().QueryRow("SELECT status FROM web_push_jobs WHERE subscription_id=?", idB).Scan(&status)
	if status != "retry" {
		t.Fatal("transient failure not retried")
	}
	s.store.DB().Exec("UPDATE web_push_jobs SET next_attempt_at=?", time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano))
	s.WebPush.deliverDue(ctx)
	s.store.DB().QueryRow("SELECT status FROM web_push_jobs WHERE subscription_id=?", idB).Scan(&status)
	if status != "success" || calls["/expired"] != 1 || calls["/retry"] != 2 {
		t.Fatal("device delivery state not isolated")
	}
}
func TestWebPushRejectsUnsafeEndpointsAndHonorsDataGuard(t *testing.T) {
	s := setupPush(t)
	ctx := context.Background()
	sub, _, _ := fixturePush(t, "https://127.0.0.1/admin")
	if _, err := s.WebPush.Subscribe(ctx, sub); err == nil {
		t.Fatal("accepted arbitrary local endpoint")
	}
	sub.Endpoint = "https://web.push.apple.com/test"
	s.WebPush.Subscribe(ctx, sub)
	calls := 0
	s.WebPush.client = pushTransport(func(*http.Request) (*http.Response, error) { calls++; return nil, nil })
	s.WebPush.guard = func(context.Context) security.GuardState {
		return security.GuardState{Findings: []security.Finding{{Code: "CellularDefaultRoute"}}}
	}
	s.Enqueue(ctx, NotificationEvent{Kind: "sms", DedupKey: "sms:3"})
	s.WebPush.deliverDue(ctx)
	if calls != 0 {
		t.Fatal("used a cellular default route")
	}
}

func TestWebPushWorkerDeliversWithoutWaitingForPoll(t *testing.T) {
	s := setupPush(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub, _, _ := fixturePush(t, "https://web.push.apple.com/test-immediate")
	if _, err := s.WebPush.Subscribe(ctx, sub); err != nil {
		t.Fatal(err)
	}
	sent := make(chan struct{}, 1)
	s.WebPush.client = pushTransport(func(r *http.Request) (*http.Response, error) {
		sent <- struct{}{}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if _, err := s.Enqueue(ctx, NotificationEvent{Kind: "sms", DedupKey: "sms:immediate"}); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { s.WebPush.Run(ctx); close(done) }()
	select {
	case <-sent:
	case <-time.After(500 * time.Millisecond):
		t.Error("new SMS waited for periodic poll")
	}
	cancel()
	<-done
}

func TestWebPushRetryRefreshesBadgeAfterMessageRead(t *testing.T) {
	s := setupPush(t)
	ctx := context.Background()
	sub, key, auth := fixturePush(t, "https://web.push.apple.com/badge-retry")
	if _, err := s.WebPush.Subscribe(ctx, sub); err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.InsertSMS(ctx, database.SMSMessage{DeviceID: "fixture", RawHash: "badge-message", Body: "fixture only"})
	if err != nil {
		t.Fatal(err)
	}
	var counts []float64
	s.WebPush.client = pushTransport(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		counts = append(counts, decryptPush(t, raw, key, auth)["unread"].(float64))
		code := 503
		if len(counts) > 1 {
			code = 201
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if _, err := s.Enqueue(ctx, NotificationEvent{Kind: "sms", DedupKey: "sms:badge"}); err != nil {
		t.Fatal(err)
	}
	s.WebPush.deliverDue(ctx)
	if err := s.store.MarkSMSRead(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	s.store.DB().Exec("UPDATE web_push_jobs SET next_attempt_at=?", time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano))
	s.WebPush.deliverDue(ctx)
	if len(counts) != 2 || counts[0] != 1 || counts[1] != 0 {
		t.Fatalf("retry restored stale unread badge: %v", counts)
	}
}

func TestWebPushTemporaryOutageRetriesUntilExpiry(t *testing.T) {
	s := setupPush(t)
	ctx := context.Background()
	sub, _, _ := fixturePush(t, "https://web.push.apple.com/long-outage")
	if _, err := s.WebPush.Subscribe(ctx, sub); err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.WebPush.client = pushTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		code := 503
		if calls > 8 {
			code = 201
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if _, err := s.Enqueue(ctx, NotificationEvent{Kind: "sms", DedupKey: "sms:outage"}); err != nil {
		t.Fatal(err)
	}
	for range 9 {
		s.store.DB().Exec("UPDATE web_push_jobs SET next_attempt_at=?", time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano))
		s.WebPush.deliverDue(ctx)
	}
	var status string
	if err := s.store.DB().QueryRow("SELECT status FROM web_push_jobs").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if calls != 9 || status != "success" {
		t.Fatalf("temporary outage lost notification: calls=%d status=%s", calls, status)
	}
	if _, err := s.Enqueue(ctx, NotificationEvent{Kind: "sms", DedupKey: "sms:expired"}); err != nil {
		t.Fatal(err)
	}
	s.store.DB().Exec("UPDATE web_push_jobs SET created_at=? WHERE dedup_key='sms:expired'", time.Now().Add(-25*time.Hour).UTC().Format(time.RFC3339Nano))
	// Expiry is enforced by the due query even between periodic cleanup runs.
	s.WebPush.deliverDue(ctx)
	if calls != 9 {
		t.Fatal("delivered a notification older than its maximum lifetime")
	}
	if delay := webPushRetryDelay(500); delay != 5*time.Minute {
		t.Fatalf("retry delay was not capped: %s", delay)
	}
}

func TestWebPushSecurityWarningIsNotLabeledNewSMS(t *testing.T) {
	s := setupPush(t)
	ctx := context.Background()
	sub, key, auth := fixturePush(t, "https://web.push.apple.com/security")
	if _, err := s.WebPush.Subscribe(ctx, sub); err != nil {
		t.Fatal(err)
	}
	s.WebPush.client = pushTransport(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		body := decryptPush(t, raw, key, auth)["body"].(string)
		if !strings.Contains(body, "安全异常") || strings.Contains(body, "新短信") {
			t.Fatalf("security warning misrepresented as SMS: %s", body)
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if _, err := s.Enqueue(ctx, NotificationEvent{Kind: "security", DedupKey: "security:test"}); err != nil {
		t.Fatal(err)
	}
	s.WebPush.deliverDue(ctx)
}

func TestNativePushPreviewCleansUnicodeAndHandlesEmptyOrUnknownEvents(t *testing.T) {
	for _, tc := range []struct {
		name        string
		event       NotificationEvent
		title, body string
	}{
		{"unicode", NotificationEvent{Kind: "sms", Sender: "  演示联系人\r\n\t", Text: "\x00  你好\n\t世界 👨‍👩‍👧‍👦  \x07"}, "演示联系人", "你好 世界 👨‍👩‍👧‍👦"},
		{"empty", NotificationEvent{Kind: "sms", Sender: "\x00 \t", Text: "\r\n\x07"}, "信息", "收到一条新短信，点按查看。"},
		{"unknown", NotificationEvent{Kind: "unrecognized", Sender: "ignored", Text: "ignored"}, "信息", "收到一条新短信，点按查看。"},
		{"security", NotificationEvent{Kind: "security", Sender: "ignored", Text: "ignored"}, "信息", "短信服务检测到网络安全异常，请打开管理页面检查。"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			title, body := nativePushPreview(tc.event)
			if title != tc.title || body != tc.body {
				t.Fatalf("preview mismatch: title=%q body=%q", title, body)
			}
		})
	}
}

func TestNativePushLongEscapedUnicodePreviewFitsEncryptedDelivery(t *testing.T) {
	s := setupPush(t)
	ctx := context.Background()
	sub, key, auth := fixturePush(t, "https://web.push.apple.com/long-unicode")
	if _, err := s.WebPush.Subscribe(ctx, sub); err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.WebPush.client = pushTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) > 4096 {
			t.Fatalf("encrypted payload exceeds provider limit: %d", len(raw))
		}
		decoded := decryptPush(t, raw, key, auth)
		title, body := decoded["title"].(string), decoded["body"].(string)
		if !utf8.ValidString(title) || !utf8.ValidString(body) || !strings.HasSuffix(title, "…") || !strings.HasSuffix(body, "…") {
			t.Fatal("truncation damaged Unicode or omitted ellipsis")
		}
		if utf8.RuneCountInString(title) > 81 || utf8.RuneCountInString(body) > 321 {
			t.Fatal("preview exceeded its Unicode limit")
		}
		if !strings.Contains(body, "🙂") {
			t.Fatal("readable emoji was removed")
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if _, err := s.Enqueue(ctx, NotificationEvent{Kind: "sms", Sender: strings.Repeat("<&>标题🙂", 100), Text: strings.Repeat("<&>示例🙂", 1000), DedupKey: "sms:long-unicode"}); err != nil {
		t.Fatal(err)
	}
	s.WebPush.deliverDue(ctx)
	if calls != 1 {
		t.Fatal("long preview never reached the provider")
	}
}

func TestNativePushTestNotificationUsesOnlyFictionalPreview(t *testing.T) {
	s := setupPush(t)
	ctx := context.Background()
	sub, key, auth := fixturePush(t, "https://web.push.apple.com/demo-preview")
	id, err := s.WebPush.Subscribe(ctx, sub)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.WebPush.client = pushTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		raw, _ := io.ReadAll(r.Body)
		decoded := decryptPush(t, raw, key, auth)
		if decoded["title"] != "演示发件人" || decoded["body"] != "这是一条演示短信：内容预览已开启。" {
			t.Fatal("test did not use the fictional preview")
		}
		if !strings.HasPrefix(decoded["tag"].(string), "test:") || decoded["url"] != "/sms" {
			t.Fatal("test notification navigation or identity changed")
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if err := s.WebPush.Test(ctx, id); err != nil {
		t.Fatal(err)
	}
	s.WebPush.deliverDue(ctx)
	if calls != 1 {
		t.Fatal("test preview was not delivered")
	}
}
