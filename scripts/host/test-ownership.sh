#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
for script in "$SCRIPT_DIR/qmi-claim.sh" "$SCRIPT_DIR/qmi-release.sh" "$SCRIPT_DIR/qmi-ownership-watch.sh"; do
  bash -n "$script"
done
bash "$SCRIPT_DIR/qmi-claim.sh" status --help >/dev/null
bash "$SCRIPT_DIR/qmi-release.sh" --help >/dev/null 2>&1 || true
bash "$SCRIPT_DIR/qmi-ownership-watch.sh" --help >/dev/null
if bash "$SCRIPT_DIR/qmi-ownership-watch.sh" --device /dev/null --duration 1 >/dev/null 2>&1; then
  echo 'ownership test: invalid device was accepted' >&2
  exit 1
fi
if bash "$SCRIPT_DIR/qmi-claim.sh" status --device /dev/null >/dev/null 2>&1; then
  echo 'ownership test: invalid device was accepted by status' >&2
  exit 1
fi
if grep -RInE '(^|[^A-Za-z])(vohive|qmi-web-legacy)([^A-Za-z]|$)' "$SCRIPT_DIR/qmi-claim.sh" "$SCRIPT_DIR/qmi-release.sh" "$SCRIPT_DIR/qmi-ownership-watch.sh" >/dev/null 2>&1; then
  echo 'ownership test: deployment-specific name leaked into generic helper' >&2
  exit 1
fi
echo 'ownership helper tests passed'
