#!/usr/bin/env bash
# Explicit opt-in preparation for maintainers building their own offline bundle.
# It is never called by offline-build.sh and never runs unless requested with
# install.sh --prepare-online or directly by a maintainer.
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
source "$SCRIPT_DIR/common.sh"
ROOT=$(qmi_root)

[[ $(uname -s) == Linux ]] || qmi_die "prepare-online currently supports Linux toolchains only"
case $(uname -m) in
  x86_64|amd64)
    go_file=go1.26.3.linux-amd64.tar.gz
    go_sha256=2b2cfc7148493da5e73981bffbf3353af381d5f93e789c82c79aff64962eb556
    node_file=node-v26.7.0-linux-x64.tar.xz
    node_sha256=982aa24dd8be4c889c6a8ab337ddff3b0896645b20f4239356e80552c16277ee
    ;;
  *) qmi_die "no validated portable toolchain for architecture $(uname -m)" ;;
esac

qmi_require tar
qmi_require sha256sum
if command -v curl >/dev/null 2>&1; then
  download() { curl --fail --location --proto '=https' --tlsv1.2 --output "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  download() { wget --https-only --output-document="$2" "$1"; }
else
  qmi_die "--prepare-online needs curl or wget to obtain verified portable toolchains"
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/qmi-web-prepare.XXXXXX")
trap 'rm -rf -- "$tmp"' EXIT
mkdir -p -- "$ROOT/toolchains" "$ROOT/offline/npm-cache"
if [[ -r /etc/ssl/certs/ca-certificates.crt ]]; then
  cp -p -- /etc/ssl/certs/ca-certificates.crt "$ROOT/offline/ca-certificates.crt"
else
  qmi_die 'host CA bundle is unavailable; provide a trusted CA bundle before packaging'
fi

echo 'Preparing verified Go and Node toolchains (explicit online mode).'
download "https://go.dev/dl/$go_file" "$tmp/$go_file"
download "https://nodejs.org/dist/v26.7.0/$node_file" "$tmp/$node_file"
printf '%s  %s\n' "$go_sha256" "$tmp/$go_file" | sha256sum --strict --check -
printf '%s  %s\n' "$node_sha256" "$tmp/$node_file" | sha256sum --strict --check -

rm -rf -- "$ROOT/toolchains/go" "$ROOT/toolchains/node"
tar -C "$tmp" -xzf "$tmp/$go_file"
tar -C "$tmp" -xJf "$tmp/$node_file"

# Keep portable Go outside the module tree while release validation runs.
# Otherwise Go recursively sees the toolchain's own test directories as
# application code.
GO_BIN="$tmp/go/bin/go"
NPM_BIN="$tmp/node-v26.7.0-linux-x64/bin/npm"
export PATH="$tmp/go/bin:$tmp/node-v26.7.0-linux-x64/bin:$PATH"
# A release source tree includes reviewed vendor/. Do not make an online
# preparation step silently rewrite go.mod, go.sum, or vendor; maintainers
# refresh those deliberately before committing a release source tree.
[[ -d $ROOT/vendor ]] || qmi_die 'Go vendor directory is missing from the release source'
export GOPROXY=off
export GOSUMDB=off
"$GO_BIN" list -mod=vendor ./cmd/... ./internal/... >/dev/null
if [[ -d $ROOT/.git ]] && [[ -n $(git -C "$ROOT" status --porcelain) ]]; then
  qmi_die 'online preparation changed tracked source; review it before packaging'
fi

pushd "$ROOT/web" >/dev/null
"$NPM_BIN" ci --cache "$ROOT/offline/npm-cache" --include=dev
popd >/dev/null
rm -rf -- "$ROOT/web/node_modules"

mv -- "$tmp/go" "$ROOT/toolchains/go"
mv -- "$tmp/node-v26.7.0-linux-x64" "$ROOT/toolchains/node"
printf 'go=1.26.3\nnode=26.7.0\n' > "$ROOT/toolchains/TOOLCHAIN_VERSIONS"
echo 'Online preparation complete. Run scripts/offline-build.sh with networking disabled to verify the resulting bundle inputs.'
