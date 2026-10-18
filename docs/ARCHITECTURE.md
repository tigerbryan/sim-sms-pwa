# Architecture

QMI Web serves a compiled Vue/TypeScript application from a small Go HTTP
server. The server owns SQLite persistence, local administrator authentication,
encrypted notification settings, structured logs, and the receive-only QMI
adapter.

The WebUI is built into internal/web/dist and embedded in the Go binary. The
version endpoint is the single build-metadata authority for the CLI, API, and
WebUI.

The QMI adapter separates discovery from opening a single selected control
node. Its operational surface is intentionally limited to DMS, UIM, NAS,
read-only WDS status, WMS inbound reads, and event subscription. The service
does not create a cellular data session or expose modem write controls.

The Docker runtime is an unprivileged scratch image. No-device and hardware
Compose definitions are separate so the safe default cannot inherit a device
mapping.
