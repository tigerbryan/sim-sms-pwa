#!/usr/bin/env bash
# Build and test entirely from a supplied offline bundle. This script contains
# no downloader and has no online fallback by design.
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
source "$SCRIPT_DIR/common.sh"
ROOT=$(qmi_root)
VERSION=$(qmi_version "$ROOT")
if [[ -n ${QMI_WEB_COMMIT:-} ]]; then
  COMMIT=$QMI_WEB_COMMIT
elif [[ -f $ROOT/RELEASE_COMMIT ]]; then
  COMMIT=$(tr -d '[:space:]' < "$ROOT/RELEASE_COMMIT")
elif [[ -d $ROOT/.git ]]; then
  COMMIT=$(git -C "$ROOT" rev-parse HEAD)
else
  COMMIT=unknown
fi
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)

[[ $(uname -s) == Linux ]] || qmi_die "offline bundle is currently validated for Linux only"
case $(uname -m) in
  x86_64|amd64) ;;
  *) qmi_die "offline bundle architecture is linux-amd64; found $(uname -m)" ;;
esac

GO_BIN="$ROOT/toolchains/go/bin/go"
NODE_BIN="$ROOT/toolchains/node/bin/node"
NPM_BIN="$ROOT/toolchains/node/bin/npm"
NPM_CACHE="$ROOT/offline/npm-cache"
CA_BUNDLE="$ROOT/offline/ca-certificates.crt"
for required in "$GO_BIN" "$NODE_BIN" "$NPM_BIN" "$CA_BUNDLE"; do
  [[ -e $required ]] || qmi_die "offline build input missing: ${required#$ROOT/}"
done
[[ -d $NPM_CACHE ]] || qmi_die "offline npm cache missing; use an official Offline Bundle"
[[ -d $ROOT/vendor ]] || qmi_die "Go vendor directory missing; use an official Offline Bundle"
qmi_require docker

export PATH="$ROOT/toolchains/go/bin:$ROOT/toolchains/node/bin:$PATH"
export GOROOT="$ROOT/toolchains/go"
export GOPATH="$ROOT/build/go-path"
export GOMODCACHE="$ROOT/build/go-mod-cache"
export GOCACHE="$ROOT/build/go-cache"
export GOPROXY=off
export GOSUMDB=off
export GOFLAGS='-mod=vendor'
export HOME="$ROOT/build/offline-home"
export npm_config_cache="$NPM_CACHE"
export npm_config_offline=true
export npm_config_audit=false
export npm_config_fund=false
export npm_config_update_notifier=false
export npm_config_prefer_offline=true
export npm_config_ignore_scripts=false

restore_web_assets() {
  rm -rf -- "$ROOT/web/node_modules" "$ROOT/web/dist" "$ROOT/internal/web/dist"
  mkdir -p -- "$ROOT/internal/web/dist/assets"
  cp -- "$ROOT/internal/web/placeholder/index.html" "$ROOT/internal/web/dist/index.html"
  cp -- "$ROOT/internal/web/placeholder/assets/app-placeholder.js" "$ROOT/internal/web/dist/assets/app-placeholder.js"
}
trap restore_web_assets EXIT

mkdir -p -- "$GOPATH" "$GOMODCACHE" "$GOCACHE" "$HOME" "$ROOT/build/offline"
restore_web_assets

"$NODE_BIN" --version
"$GO_BIN" version

pushd "$ROOT/web" >/dev/null
"$NPM_BIN" ci --offline --include=dev
"$NPM_BIN" run typecheck
"$NPM_BIN" test
"$NPM_BIN" run build
popd >/dev/null

[[ -f $ROOT/internal/web/dist/index.html ]] || qmi_die 'Vite did not produce internal/web/dist/index.html'
find "$ROOT/internal/web/dist/assets" -maxdepth 1 -type f \( -name '*.js' -o -name '*.css' \) -print -quit | grep -q . || qmi_die 'Vite did not produce static assets'

GO_PACKAGES=(./cmd/... ./internal/...)
"$GO_BIN" test -mod=vendor "${GO_PACKAGES[@]}"
"$GO_BIN" vet -mod=vendor "${GO_PACKAGES[@]}"

ldflags="-s -w -X qmi-web/internal/version.Version=$VERSION -X qmi-web/internal/version.Commit=$COMMIT -X qmi-web/internal/version.BuildTime=$BUILD_TIME"
build_binary() {
  local output=$1 package=$2
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "$GO_BIN" build -mod=vendor -buildvcs=false -trimpath -ldflags="$ldflags" -o "$ROOT/build/offline/$output" "$package"
  [[ -s $ROOT/build/offline/$output ]] || qmi_die "failed to create $output"
  chmod 0755 -- "$ROOT/build/offline/$output"
}
build_binary qmi-web ./cmd/server
build_binary qmi-probe ./cmd/qmi-probe
build_binary qmi-dbcheck ./cmd/qmi-dbcheck
build_binary qmi-backup ./cmd/qmi-backup
cp -- "$CA_BUNDLE" "$ROOT/build/offline/ca-certificates.crt"
printf 'VERSION=%s\nCOMMIT=%s\nBUILD_TIME=%s\nOFFLINE_BUILD_NETWORK_ACCESS=NONE\n' "$VERSION" "$COMMIT" "$BUILD_TIME" > "$ROOT/build/offline/build-metadata.env"

docker build --network none --pull=false \
  --build-arg VERSION="$VERSION" --build-arg COMMIT="$COMMIT" --build-arg BUILD_TIME="$BUILD_TIME" \
  -f "$ROOT/Dockerfile.offline" -t "local/qmi-web:$VERSION" "$ROOT"

[[ $(docker image inspect --format '{{.Config.User}}' "local/qmi-web:$VERSION") == '65532:65532' ]] || qmi_die "offline image is not configured as UID 65532"
echo 'offline build passed: npm, Go tests/vet/build, and scratch image used no network access'
