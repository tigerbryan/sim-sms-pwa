# API contract

The complete, test-verified route inventory is in ROUTES.md. Public health,
readiness, and version endpoints require no session. All application API routes
use same-origin session and CSRF controls where they mutate local application
state. There are no routes for dialing, data sessions, APN/profile changes, AT
commands, or SMS transmission.
