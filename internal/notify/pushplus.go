// Package notify implements extensible asynchronous notification delivery.
package notify

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"qmi-web/internal/database"
	"qmi-web/internal/security"
)

type NotificationEvent struct {
	Kind     string
	Title    string
	Body     string
	DedupKey string
	Sender   string
	Text     string
}

type Notifier interface {
	Name() string
	Send(context.Context, NotificationEvent) error
}

type PushPlusNotifier struct {
	Token    func(context.Context) (string, error)
	Template func(context.Context) (string, error)
	Client   *http.Client
	Endpoint string
}

func (p *PushPlusNotifier) Name() string { return "pushplus" }

func (p *PushPlusNotifier) Send(ctx context.Context, event NotificationEvent) error {
	if p.Token == nil {
		return errors.New("PushPlus token provider is not configured")
	}
	token, err := p.Token(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("PushPlus is enabled without a token")
	}
	template := "html"
	if p.Template != nil {
		value, err := p.Template(ctx)
		if err != nil {
			return err
		}
		if strings.TrimSpace(value) != "" {
			template = value
		}
	}
	payload, err := json.Marshal(map[string]string{
		"token": token, "title": event.Title, "content": truncate(event.Body, 3000), "template": template,
	})
	if err != nil {
		return err
	}
	endpoint := p.Endpoint
	if endpoint == "" {
		endpoint = "https://www.pushplus.plus/send"
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("PushPlus HTTP %d", resp.StatusCode)
	}
	var reply struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal(body, &reply) == nil && reply.Code != 0 && reply.Code != 200 {
		return fmt.Errorf("PushPlus rejected notification: %s", reply.Msg)
	}
	return nil
}

type Service struct {
	store   *database.Store
	cipher  *security.Cipher
	guard   func(context.Context) security.GuardState
	push    *PushPlusNotifier
	WebPush *WebPush
}

type PublicConfig struct {
	Enabled         bool   `json:"enabled"`
	Template        string `json:"template"`
	TokenConfigured bool   `json:"token_configured"`
}

func NewService(store *database.Store, cipher *security.Cipher, guard func(context.Context) security.GuardState) *Service {
	s := &Service{store: store, cipher: cipher, guard: guard}
	s.push = &PushPlusNotifier{Token: s.token, Template: s.template}
	s.WebPush = newWebPush(store, cipher, guard)
	return s
}

func (s *Service) Config(ctx context.Context) (PublicConfig, error) {
	enabled, _ := s.store.Setting(ctx, "pushplus_enabled")
	template, _ := s.store.Setting(ctx, "pushplus_template")
	token, err := s.store.Setting(ctx, "pushplus_token_encrypted")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return PublicConfig{}, err
	}
	return PublicConfig{Enabled: enabled == "true", Template: template, TokenConfigured: token != ""}, nil
}

func (s *Service) Update(ctx context.Context, enabled bool, token, template string) error {
	if strings.TrimSpace(token) != "" {
		encrypted, err := s.cipher.Encrypt(token)
		if err != nil {
			return err
		}
		if err := s.store.SetSetting(ctx, "pushplus_token_encrypted", encrypted); err != nil {
			return err
		}
	}
	if enabled {
		value, err := s.store.Setting(ctx, "pushplus_token_encrypted")
		if err != nil || value == "" {
			return errors.New("a PushPlus token is required before enabling notifications")
		}
	}
	if err := s.store.SetSetting(ctx, "pushplus_enabled", fmt.Sprintf("%t", enabled)); err != nil {
		return err
	}
	return s.store.SetSetting(ctx, "pushplus_template", strings.TrimSpace(template))
}

func (s *Service) Enqueue(ctx context.Context, event NotificationEvent) (bool, error) {
	webCreated, webErr := s.WebPush.Enqueue(ctx, event, "")
	if webErr != nil {
		return false, webErr
	}
	config, err := s.Config(ctx)
	if err != nil {
		return webCreated, err
	}
	if !config.Enabled {
		return webCreated, nil
	}
	_, created, err := s.store.CreateNotification(ctx, database.Notification{Kind: event.Kind, Title: event.Title, Body: event.Body, DedupKey: event.DedupKey})
	return created || webCreated, err
}

func (s *Service) SendTest(ctx context.Context) error {
	config, err := s.Config(ctx)
	if err != nil {
		return err
	}
	if !config.Enabled {
		return errors.New("PushPlus is disabled")
	}
	return s.send(ctx, NotificationEvent{Kind: "test", Title: "QMI Web test notification", Body: "PushPlus connectivity test from QMI Web.", DedupKey: "test-" + time.Now().UTC().Format(time.RFC3339Nano)})
}

func (s *Service) Run(ctx context.Context) {
	go s.WebPush.Run(ctx)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.deliverDue(ctx)
		}
	}
}

func (s *Service) deliverDue(ctx context.Context) {
	items, err := s.store.DueNotifications(ctx, 10)
	if err != nil {
		return
	}
	for _, item := range items {
		event := NotificationEvent{Kind: item.Kind, Title: item.Title, Body: item.Body, DedupKey: item.DedupKey}
		err := s.send(ctx, event)
		attempts := item.Attempts + 1
		status, deliveryStatus, detail := "success", "success", ""
		next := time.Now().UTC()
		if err != nil {
			status, deliveryStatus, detail = "failed", "failed", err.Error()
			next = time.Now().UTC().Add(backoff(attempts))
		}
		_ = s.store.UpdateNotification(ctx, item.ID, status, attempts, next, s.push.Name(), deliveryStatus, detail)
	}
}

func (s *Service) send(ctx context.Context, event NotificationEvent) error {
	if s.guard != nil {
		guard := s.guard(ctx)
		if guard.HasFinding("CellularDefaultRoute") {
			return errors.New("PushPlus delivery suppressed: cellular default route is active")
		}
	}
	return s.push.Send(ctx, event)
}

func (s *Service) token(ctx context.Context) (string, error) {
	encrypted, err := s.store.Setting(ctx, "pushplus_token_encrypted")
	if err != nil {
		return "", err
	}
	return s.cipher.Decrypt(encrypted)
}

func (s *Service) template(ctx context.Context) (string, error) {
	value, err := s.store.Setting(ctx, "pushplus_template")
	if errors.Is(err, sql.ErrNoRows) {
		return "html", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func backoff(attempt int) time.Duration {
	switch attempt {
	case 1:
		return time.Minute
	case 2:
		return 5 * time.Minute
	default:
		return 15 * time.Minute
	}
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "…"
}
