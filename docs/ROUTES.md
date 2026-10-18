# QMI Web v1.0.0 backend route inventory
# Generated from internal/api.RouteContracts; verified by TestRoutesContract.

METHOD | PATH | HANDLER | ACCESS | SIDE EFFECT
--- | --- | --- | --- | ---
GET | /health | Server.serveHTTP | public | none
GET | /ready | Server.serveHTTP | public | none
GET | /version | Server.serveHTTP | public | none
POST | /api/v1/auth/login | Server.handleAPI | same-origin | creates server session
GET | /api/v1/auth/me | Server.handleAPI | session | none
POST | /api/v1/auth/logout | Server.handleAPI | session + CSRF | invalidates current session
POST | /api/v1/auth/password | Server.handleAPI | session + CSRF | changes local admin password and invalidates sessions
GET | /api/v1/dashboard | Server.handleAPI | session | read-only snapshot
GET | /api/v1/devices | Server.handleAPI | session | returns cached discovery snapshot
POST | /api/v1/devices/scan | Server.handleAPI | session + CSRF | read-only device discovery
GET | /api/v1/sim | Server.handleAPI | session | read-only SIM snapshot
GET | /api/v1/signal | Server.handleAPI | session | read-only signal snapshot
GET | /api/v1/sms | Server.listSMS | session | read-only paginated SMS query
GET | /api/v1/sms/{id} | Server.smsDetail | session | read-only SMS query
PATCH | /api/v1/sms/{id}/read | Server.smsDetail | session + CSRF | updates local read status only
GET | /api/v1/settings | Server.settings | session | returns safe settings metadata
PATCH | /api/v1/settings | Server.updateSettings | session + CSRF | updates runtime logging, scan interval, and encrypted PushPlus config
GET | /api/v1/webpush/status | Server.webPush | session | returns public VAPID key and delivery metadata
POST | /api/v1/webpush/subscribe | Server.webPush | session + CSRF | stores encrypted device subscription
DELETE | /api/v1/webpush/subscribe | Server.webPush | session + CSRF | removes device subscription and pending jobs
POST | /api/v1/webpush/test | Server.webPush | session + CSRF | queues one native push test for this device
GET | /api/v1/notifications | Server.notifications | session | returns notification delivery metadata without bodies
POST | /api/v1/notifications/pushplus/test | Server.handleAPI | session + CSRF | queues one PushPlus test when enabled
GET | /api/v1/logs | Server.handleAPI | session | returns bounded structured log buffer
GET | /api/v1/logs/stream | Server.streamEvents | session | opens same-origin SSE stream
GET | /api/v1/diagnostics | Server.handleAPI | session | read-only diagnostic snapshot
