package notify

import (
	"context"
	"crypto/elliptic"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	webpush "github.com/SherClockHolmes/webpush-go"
	"qmi-web/internal/database"
	"qmi-web/internal/security"
)

type PushSubscription struct {
	Endpoint       string       `json:"endpoint"`
	ExpirationTime *float64     `json:"expirationTime,omitempty"`
	Keys           webpush.Keys `json:"keys"`
}
type WebPush struct {
	store     *database.Store
	cipher    *security.Cipher
	guard     func(context.Context) security.GuardState
	mu        sync.Mutex
	client    webpush.HTTPClient
	lastPrune time.Time
	wake      chan struct{}
}
type vapidKeys struct {
	Private string `json:"private"`
	Public  string `json:"public"`
}

func newWebPush(store *database.Store, cipher *security.Cipher, guard func(context.Context) security.GuardState) *WebPush {
	return &WebPush{wake: make(chan struct{}, 1), store: store, cipher: cipher, guard: guard, client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (p *WebPush) keys(ctx context.Context) (vapidKeys, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	value, err := p.store.Setting(ctx, "webpush_vapid_encrypted")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return vapidKeys{}, err
	}
	if value != "" {
		raw, err := p.cipher.Decrypt(value)
		if err != nil {
			return vapidKeys{}, err
		}
		var keys vapidKeys
		err = json.Unmarshal([]byte(raw), &keys)
		return keys, err
	}
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return vapidKeys{}, err
	}
	keys := vapidKeys{Private: private, Public: public}
	raw, _ := json.Marshal(keys)
	encrypted, err := p.cipher.Encrypt(string(raw))
	if err != nil {
		return vapidKeys{}, err
	}
	return keys, p.store.SetSetting(ctx, "webpush_vapid_encrypted", encrypted)
}
func (p *WebPush) Status(ctx context.Context, id string) (map[string]any, error) {
	keys, err := p.keys(ctx)
	if err != nil {
		return nil, err
	}
	var count int
	if err = p.store.DB().QueryRowContext(ctx, "SELECT count(*) FROM web_push_subscriptions").Scan(&count); err != nil {
		return nil, err
	}
	out := map[string]any{"public_key": keys.Public, "subscribers": count, "subscribed": false}
	if validPushID(id) {
		var found int
		if err = p.store.DB().QueryRowContext(ctx, "SELECT count(*) FROM web_push_subscriptions WHERE id=?", id).Scan(&found); err != nil {
			return nil, err
		}
		out["subscribed"] = found > 0
		var status string
		err = p.store.DB().QueryRowContext(ctx, "SELECT status FROM web_push_jobs WHERE subscription_id=? ORDER BY id DESC LIMIT 1", id).Scan(&status)
		if err == nil {
			out["last_delivery"] = status
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	return out, nil
}
func validPushID(id string) bool { b, err := hex.DecodeString(id); return err == nil && len(b) == 32 }
func validateSubscription(sub PushSubscription) error {
	u, err := url.Parse(sub.Endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") || len(sub.Endpoint) > 4096 {
		return errors.New("推送订阅地址无效")
	}
	host := strings.ToLower(u.Hostname())
	allowed := host == "web.push.apple.com" || strings.HasSuffix(host, ".push.apple.com") || host == "fcm.googleapis.com" || host == "updates.push.services.mozilla.com" || strings.HasSuffix(host, ".notify.windows.com")
	if !allowed {
		return errors.New("不支持此浏览器的推送服务")
	}
	key, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(sub.Keys.P256dh, "="))
	if err != nil || len(key) != 65 {
		return errors.New("推送公钥无效")
	}
	x, _ := elliptic.Unmarshal(elliptic.P256(), key)
	if x == nil {
		return errors.New("推送公钥无效")
	}
	auth, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(sub.Keys.Auth, "="))
	if err != nil || len(auth) != 16 {
		return errors.New("推送密钥无效")
	}
	return nil
}
func (p *WebPush) Subscribe(ctx context.Context, sub PushSubscription) (string, error) {
	if err := validateSubscription(sub); err != nil {
		return "", err
	}
	if _, err := p.keys(ctx); err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(sub.Endpoint))
	id := hex.EncodeToString(digest[:])
	raw, _ := json.Marshal(sub)
	encrypted, err := p.cipher.Encrypt(string(raw))
	if err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = p.store.DB().ExecContext(ctx, `INSERT INTO web_push_subscriptions(id,encrypted,created_at,updated_at) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET encrypted=excluded.encrypted,updated_at=excluded.updated_at`, id, encrypted, stamp, stamp)
	return id, err
}
func (p *WebPush) Unsubscribe(ctx context.Context, id string) error {
	if !validPushID(id) {
		return errors.New("订阅编号无效")
	}
	_, err := p.store.DB().ExecContext(ctx, "DELETE FROM web_push_subscriptions WHERE id=?", id)
	return err
}

// Queues are per subscription: one phone's failure cannot repeat delivery to another phone.
func (p *WebPush) Enqueue(ctx context.Context, event NotificationEvent, onlyID string) (bool, error) {
	rows, err := p.store.DB().QueryContext(ctx, "SELECT id FROM web_push_subscriptions WHERE (?='' OR id=?)", onlyID, onlyID)
	if err != nil {
		return false, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return false, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	if len(ids) == 0 {
		return false, nil
	}
	unread, err := p.store.UnreadSMSCount(ctx)
	if err != nil {
		return false, err
	}
	title, body := nativePushPreview(event)
	payload, err := json.Marshal(map[string]any{"title": title, "body": body, "tag": event.DedupKey, "url": "/sms", "unread": unread})
	if err != nil {
		return false, err
	}
	// Leave room for AES128GCM framing below the provider's 4096-byte limit.
	// Normal previews are bounded even for JSON's worst-case six-byte escapes.
	if len(payload) > 3800 {
		return false, errors.New("native push payload exceeds size limit")
	}
	encrypted, err := p.cipher.Encrypt(string(payload))
	if err != nil {
		return false, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	created := false
	for _, id := range ids {
		res, e := p.store.DB().ExecContext(ctx, `INSERT OR IGNORE INTO web_push_jobs(subscription_id,dedup_key,payload,next_attempt_at,created_at,updated_at) VALUES(?,?,?,?,?,?)`, id, event.DedupKey, encrypted, now, now, now)
		if e != nil {
			return created, e
		}
		n, _ := res.RowsAffected()
		created = created || n > 0
	}
	if created {
		select {
		case p.wake <- struct{}{}:
		default:
		}
	}
	return created, nil
}

func nativePushPreview(event NotificationEvent) (title, body string) {
	title, body = "信息", "收到一条新短信，点按查看。"
	switch event.Kind {
	case "sms":
		// Content previews were explicitly enabled by the owner. Keep the raw
		// sender/text separate from the formatted legacy PushPlus body.
		if sender := cleanPushPreview(event.Sender, 80); sender != "" {
			title = sender
		}
		if text := cleanPushPreview(event.Text, 320); text != "" {
			body = text
		}
	case "test":
		// A test never borrows a real inbox message or its sender.
		title = "演示发件人"
		body = "这是一条演示短信：内容预览已开启。"
	case "security":
		body = "短信服务检测到网络安全异常，请打开管理页面检查。"
	}
	return title, body
}

func cleanPushPreview(value string, limit int) string {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	runes := []rune(cleaned)
	if len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return cleaned
}

func (p *WebPush) Test(ctx context.Context, id string) error {
	if !validPushID(id) {
		return errors.New("请先在这台设备启用通知")
	}
	created, err := p.Enqueue(ctx, NotificationEvent{Kind: "test", DedupKey: fmt.Sprintf("test:%d", time.Now().UnixNano())}, id)
	if err != nil {
		return err
	}
	if !created {
		return errors.New("这台设备尚未订阅通知")
	}
	return nil
}
func (p *WebPush) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.wake:
			p.deliverDue(ctx)
		case <-ticker.C:
			p.deliverDue(ctx)
		}
	}
}
func (p *WebPush) deliverDue(ctx context.Context) {
	now := time.Now().UTC()
	if now.Sub(p.lastPrune) >= time.Minute {
		p.store.DB().ExecContext(ctx, "UPDATE web_push_jobs SET status='expired' WHERE status IN ('pending','retry') AND created_at < ?", now.Add(-24*time.Hour).Format(time.RFC3339Nano))
		p.store.DB().ExecContext(ctx, "DELETE FROM web_push_jobs WHERE created_at < ?", now.Add(-7*24*time.Hour).Format(time.RFC3339Nano))
		p.lastPrune = now
	}
	rows, err := p.store.DB().QueryContext(ctx, `SELECT j.id,j.subscription_id,j.payload,j.attempts,s.encrypted FROM web_push_jobs j JOIN web_push_subscriptions s ON s.id=j.subscription_id WHERE j.status IN ('pending','retry') AND j.next_attempt_at<=? AND j.created_at>=? ORDER BY j.id LIMIT 10`, now.Format(time.RFC3339Nano), now.Add(-24*time.Hour).Format(time.RFC3339Nano))
	if err != nil {
		return
	}
	type job struct {
		id                  int64
		subID, payload, sub string
		attempts            int
	}
	var jobs []job
	for rows.Next() {
		var j job
		if err = rows.Scan(&j.id, &j.subID, &j.payload, &j.attempts, &j.sub); err != nil {
			break
		}
		jobs = append(jobs, j)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return
	}
	if len(jobs) == 0 {
		return
	}
	if p.guard != nil && p.guard(ctx).HasFinding("CellularDefaultRoute") {
		return
	}
	keys, err := p.keys(ctx)
	if err != nil {
		return
	}
	for _, j := range jobs {
		unread, err := p.store.UnreadSMSCount(ctx)
		if err != nil {
			return // Leave durable jobs pending when their source data is unavailable.
		}
		raw, e := p.cipher.Decrypt(j.sub)
		var sub PushSubscription
		if e == nil {
			e = json.Unmarshal([]byte(raw), &sub)
		}
		// Revalidate persisted endpoints before any outbound request.
		if e == nil {
			e = validateSubscription(sub)
		}
		payload := ""
		if e == nil {
			payload, e = p.cipher.Decrypt(j.payload)
		}
		// A retry may happen after messages have been read elsewhere. Recompute
		// the badge immediately before sending instead of restoring stale counts.
		if e == nil {
			payload, e = refreshUnread(payload, unread)
		}
		status := "failed"
		var code int
		if e == nil {
			// Never log the request URL or provider response body: endpoint paths are secrets.
			resp, sendErr := webpush.SendNotificationWithContext(ctx, []byte(payload), &webpush.Subscription{Endpoint: sub.Endpoint, Keys: sub.Keys}, &webpush.Options{Subscriber: "https://github.com/tigerbryan/sim-sms-pwa", VAPIDPrivateKey: keys.Private, VAPIDPublicKey: keys.Public, TTL: 3600, Urgency: webpush.UrgencyHigh, HTTPClient: p.client})
			if sendErr == nil {
				code = resp.StatusCode
				io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
				resp.Body.Close()
			}
			if code == 404 || code == 410 {
				p.Unsubscribe(ctx, j.subID)
				continue
			}
			if code >= 200 && code < 300 {
				status = "success"
			} else if sendErr != nil || code == 429 || code >= 500 {
				status = "retry"
			}
		}
		delay := webPushRetryDelay(j.attempts)
		p.store.DB().ExecContext(ctx, "UPDATE web_push_jobs SET status=?,attempts=attempts+1,next_attempt_at=?,updated_at=? WHERE id=?", status, time.Now().UTC().Add(delay).Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), j.id)
	}
}

func refreshUnread(payload string, unread int) (string, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return "", err
	}
	if data == nil {
		return "", errors.New("empty push payload")
	}
	data["unread"] = unread
	raw, err := json.Marshal(data)
	return string(raw), err
}

func webPushRetryDelay(attempts int) time.Duration {
	// Retry temporary outages for the job's 24-hour lifetime without spinning
	// or overflowing the exponential delay after many failed attempts.
	if attempts >= 6 {
		return 5 * time.Minute
	}
	return time.Duration(1<<max(attempts, 0)) * 5 * time.Second
}
