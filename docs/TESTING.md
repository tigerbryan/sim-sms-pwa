# Testing

Normal tests are hardware-free:

    go test -mod=vendor ./...
    go vet -mod=vendor ./...
    cd web && npm test && npm run typecheck && npm run build
    ./scripts/verify-security.sh

The public CI runs Go tests/vet, frontend typecheck/tests/build, static
security checks, license/dependency checks, and a Linux amd64 build. CI does
not access a real QMI device and has no production secrets.

The Offline Bundle validation is stronger than a source compile. It starts from
the final bundle, removes generated node_modules, uses an isolated HOME,
GOMODCACHE, GOCACHE, and npm cache, sets all Go/npm offline settings, builds a
scratch image with Docker networking disabled, then installs no-device mode and
checks health, readiness, version, persistence, and container settings.

Real hardware validation is opt-in and excluded from default test and CI
execution. It must use only an isolated, explicitly selected QMI control node
and must not send a message or alter modem/network state.
