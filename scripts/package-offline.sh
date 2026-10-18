#!/usr/bin/env bash
# Assemble a Linux amd64 Offline Bundle from the current, clean public source.
# It never downloads a toolchain or dependency.
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
source "$SCRIPT_DIR/common.sh"
ROOT=$(qmi_root)
VERSION=$(qmi_version "$ROOT")
OUTPUT_DIR=${QMI_WEB_RELEASE_DIR:-$ROOT/release-output}

[[ -d $ROOT/.git ]] || qmi_die 'package-offline requires a checked-out public Git repository'
[[ -z $(git -C "$ROOT" status --porcelain) ]] || qmi_die 'package-offline requires a clean public Git working tree'
qmi_require git
qmi_require tar
qmi_require sha256sum
[[ -d $ROOT/toolchains/go && -d $ROOT/toolchains/node && -d $ROOT/offline/npm-cache ]] || qmi_die 'offline toolchains or npm cache are missing; explicitly run scripts/prepare-online.sh first'
[[ -f $ROOT/offline/ca-certificates.crt ]] || qmi_die 'offline CA bundle is missing'
if ! tar --help 2>/dev/null | grep -q -- '--zstd'; then
  qmi_die 'tar with --zstd support is required to create the promised release asset'
fi

"$ROOT/scripts/offline-build.sh"
commit=$(git -C "$ROOT" rev-parse HEAD)
stage_parent=$(mktemp -d "${TMPDIR:-/tmp}/qmi-web-package.XXXXXX")
trap 'rm -rf -- "$stage_parent"' EXIT
bundle_name="qmi-web-offline-linux-amd64-v$VERSION"
bundle_root="$stage_parent/$bundle_name"
source_name="qmi-web-v$VERSION"
source_root="$stage_parent/$source_name"
mkdir -p -- "$bundle_root"

git -C "$ROOT" archive --format=tar HEAD | tar -C "$bundle_root" -xf -
mkdir -p -- "$bundle_root/toolchains" "$bundle_root/offline"
cp -a -- "$ROOT/toolchains/go" "$bundle_root/toolchains/go"
cp -a -- "$ROOT/toolchains/node" "$bundle_root/toolchains/node"
cp -a -- "$ROOT/offline/npm-cache" "$bundle_root/offline/npm-cache"
cp -p -- "$ROOT/offline/ca-certificates.crt" "$bundle_root/offline/ca-certificates.crt"
# npm creates and rotates diagnostic logs during every offline install. They are
# not package inputs and must not make a verified bundle fail on its next run.
rm -rf -- "$bundle_root/offline/npm-cache/_logs" "$bundle_root/offline/npm-cache/_cacache/tmp"
printf '%s\n' "$commit" > "$bundle_root/RELEASE_COMMIT"
printf 'VERSION=%s\nCOMMIT=%s\nGO=1.26.3\nNODE=26.7.0\nARCH=linux-amd64\n' "$VERSION" "$commit" > "$bundle_root/RELEASE_METADATA"
chmod 700 "$bundle_root/install.sh" "$bundle_root/scripts/"*.sh
(
  cd "$bundle_root"
  find . -type f \
    ! -name MANIFEST.sha256 \
    ! -path './offline/npm-cache/_logs/*' \
    ! -path './offline/npm-cache/_cacache/tmp/*' \
    -print0 | sort -z | xargs -0 sha256sum > MANIFEST.sha256
)

mkdir -p -- "$OUTPUT_DIR"
source_asset="$OUTPUT_DIR/qmi-web-source-v$VERSION.tar.gz"
offline_asset="$OUTPUT_DIR/qmi-web-offline-linux-amd64-v$VERSION.tar.zst"
dependency_asset="$OUTPUT_DIR/qmi-web-dependencies-v$VERSION.md"
mkdir -p -- "$source_root"
git -C "$ROOT" archive --format=tar HEAD | tar -C "$source_root" -xf -
printf '%s\n' "$commit" > "$source_root/RELEASE_COMMIT"
printf 'VERSION=%s\nCOMMIT=%s\nGO=1.26.3\nNODE=26.7.0\nARCH=source\n' "$VERSION" "$commit" > "$source_root/RELEASE_METADATA"
tar -C "$stage_parent" -czf "$source_asset" "$source_name"
tar --zstd -C "$stage_parent" -cf "$offline_asset" "$bundle_name"
cp -p -- "$ROOT/docs/DEPENDENCIES.md" "$dependency_asset"
(
  cd "$OUTPUT_DIR"
  sha256sum "$(basename "$source_asset")" "$(basename "$offline_asset")" "$(basename "$dependency_asset")" > SHA256SUMS
)
printf 'Created:\n%s\n%s\n%s\n%s\n' "$source_asset" "$offline_asset" "$dependency_asset" "$OUTPUT_DIR/SHA256SUMS"
