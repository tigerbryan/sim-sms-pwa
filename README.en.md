# SIM SMS PWA

A self-hosted, receive-only SIM inbox for Linux and a Quectel EC25 QMI USB modem. Leave the SIM at home and read incoming SMS in an installable phone/desktop web app, with encrypted Web Push and unread badges where supported.

Derived from MIT-licensed [QMI Web](https://github.com/xuanxuan1125/qmi-web), with upstream copyright retained. See [NOTICE](NOTICE.md) and [third-party notices](THIRD_PARTY_NOTICES.md).

[中文完整文档](README.md) · [Hardware and buying links](docs/HARDWARE.md) · [Deployment](docs/DEPLOYMENT.md) · [Copyable agent prompt](AGENT_PROMPT.md)

## Quick start (no modem)

Go (see `go.mod`), Node.js 24 and npm are required.

```bash
git clone https://github.com/tigerbryan/sim-sms-pwa.git
cd sim-sms-pwa
make build
mkdir -p .private/demo/logs
QMI_WEB_BACKEND=mock ./build/qmi-web --config config/config.example.yaml --data .private/demo/data --logs .private/demo/logs
```

Open `http://localhost:7580/sms`. The loopback-only initial account is `admin / admin`: change its password in web settings before enabling remote access. Mock mode does not validate real SMS reception.

On Linux, after reading [deployment instructions](docs/DEPLOYMENT.md), explicitly select one modem:

```bash
sudo ./scripts/install-native.sh --binary ./build/qmi-web --device /dev/cdc-wdm0
```

The device must actually exist and be unused. The installer never stops ModemManager, starts packet data or flashes firmware. It binds to loopback; add a trusted HTTPS reverse proxy and access control separately. A physical SIM and regional/carrier SMS compatibility are required. The Pi host uses Ethernet/Wi-Fi for Web Push, never a data session started by this app.

## Boundaries

- Receiving only: no SMS sending/deletion, APN changes, WDS data start, voice/SIP or MMS.
- Real modem support is Linux-only. macOS/Windows can access the PWA but are not supported modem servers; DSM driver support is not promised.
- iPhone push requires a Home Screen web app, HTTPS and a user gesture to grant permission. System delivery is best effort, not a seconds-level SLA.
- The web app cannot inject SMS into Apple Messages or enable cross-app OTP autofill.
- Updating web assets cannot force replacement of an OS-owned installed icon.
- No cloud SMS upload service, shared password, deployment database, personal network address or private push key is shipped.

## Development

```bash
go test -mod=vendor -buildvcs=false ./...
go vet -mod=vendor -buildvcs=false ./...
cd web && npm ci && npm test && npm run build
```

See [architecture](docs/ARCHITECTURE.md), [security](SECURITY.md), [privacy](docs/PRIVACY.md), [operations](docs/OPERATIONS.md) and [contributing](CONTRIBUTING.md). The full deployment/agent guides are currently in Chinese; an English translation is welcome.

Derived from [xuanxuan1125/qmi-web](https://github.com/xuanxuan1125/qmi-web), with its MIT notice retained. See [NOTICE](NOTICE.md) and [third-party licenses](THIRD_PARTY_NOTICES.md). Independent project; not affiliated with Apple, Quectel or hardware vendors.
