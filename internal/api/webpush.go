package api

import (
	"net/http"
	"qmi-web/internal/notify"
)

// All routes reach this handler only after session and mutation CSRF validation.
func (s *Server) webPush(w http.ResponseWriter, r *http.Request) {
	push := s.app.Notifications.WebPush
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/webpush/status":
		status, err := push.Status(r.Context(), r.URL.Query().Get("id"))
		if err != nil {
			writeError(w, 500, "push_unavailable", "通知服务暂时不可用")
			return
		}
		writeJSON(w, 200, status)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/webpush/subscribe":
		var sub notify.PushSubscription
		if err := decodeJSON(r, &sub); err != nil {
			writeError(w, 400, "invalid_subscription", "推送订阅格式无效")
			return
		}
		id, err := push.Subscribe(r.Context(), sub)
		if err != nil {
			writeError(w, 400, "subscription_failed", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"id": id, "subscribed": true})
	case (r.Method == http.MethodPost && r.URL.Path == "/api/v1/webpush/test") || (r.Method == http.MethodDelete && r.URL.Path == "/api/v1/webpush/subscribe"):
		var input struct {
			ID string `json:"id"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, 400, "invalid_request", "订阅编号无效")
			return
		}
		var err error
		if r.Method == http.MethodDelete {
			err = push.Unsubscribe(r.Context(), input.ID)
		} else {
			err = push.Test(r.Context(), input.ID)
		}
		if err != nil {
			writeError(w, 400, "push_failed", err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "queued": r.Method == http.MethodPost})
	default:
		writeError(w, 404, "not_found", "通知接口不存在")
	}
}
