#!/usr/bin/env bash
# Shared helpers for QMI Web release and lifecycle scripts. This file performs
# no network operation and deliberately contains no deployment-specific data.
set -Eeuo pipefail

qmi_root() {
  cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P
}

qmi_die() {
  printf 'qmi-web: %s\n' "$*" >&2
  exit 1
}

qmi_require() {
  command -v "$1" >/dev/null 2>&1 || qmi_die "required command not found: $1"
}

qmi_version() {
  local root=$1
  tr -d '[:space:]' < "$root/VERSION"
}

qmi_validate_install_dir() {
  local target=$1
  [[ $target == /* && $target != / ]] || qmi_die "install directory must be an absolute path other than /"
}

qmi_compose() {
  docker compose "$@"
}

qmi_manifest_verify() {
  local root=$1
  local manifest="$root/MANIFEST.sha256"
  [[ -f $manifest ]] || qmi_die "offline bundle manifest missing; use an official Offline Bundle or pass --prepare-online"
  qmi_require sha256sum
  (
    cd -- "$root"
    sha256sum --strict --check MANIFEST.sha256
  )
}

qmi_http_status() {
  local port=$1 path=$2 line
  if ! { exec 3<>"/dev/tcp/127.0.0.1/$port"; }; then
    return 1
  fi
  printf 'GET %s HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n' "$path" >&3
  IFS=$'\r' read -r line <&3 || true
  exec 3>&-
  exec 3<&-
  [[ $line =~ ^HTTP/[0-9.]+[[:space:]]+([0-9]{3})[[:space:]] ]]
  printf '%s\n' "${BASH_REMATCH[1]}"
}

qmi_wait_http_200() {
  local port=$1 path=$2 attempts=${3:-30} status
  while (( attempts > 0 )); do
    status=$(qmi_http_status "$port" "$path" 2>/dev/null || true)
    [[ $status == 200 ]] && return 0
    sleep 1
    ((attempts--))
  done
  return 1
}

qmi_is_truthy() {
  case ${1:-} in
    1|true|TRUE|yes|YES) return 0 ;;
    *) return 1 ;;
  esac
}
