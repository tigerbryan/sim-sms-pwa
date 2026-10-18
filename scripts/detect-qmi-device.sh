#!/usr/bin/env bash
# Discover only QMI cdc-wdm control nodes. This script never opens a device,
# starts data networking, sends AT commands, or changes modem state.
set -Eeuo pipefail

record_only=false
selected=

usage() {
  cat >&2 <<'EOF'
usage: detect-qmi-device.sh [--record] [--device /dev/cdc-wdmX]

Without --device, list QMI cdc-wdm character-device candidates. --record emits
tab-separated path, major, minor, VID:PID, and driver for installer use.
EOF
  exit 2
}

while (($#)); do
  case $1 in
    --record) record_only=true; shift ;;
    --device) selected=${2:-}; shift 2 ;;
    -h|--help) usage ;;
    *) usage ;;
  esac
done

describe() {
  local path=$1 base sysfs parent candidate driver vid pid major_hex minor_hex major minor
  [[ $path == /dev/cdc-wdm* && $(dirname -- "$path") == /dev ]] || return 1
  [[ -c $path ]] || return 1
  base=$(basename -- "$path")
  sysfs=$(readlink -f -- "/sys/class/usbmisc/$base/device" 2>/dev/null || true)
  [[ -n $sysfs ]] || sysfs="/sys/class/usbmisc/$base/device"
  parent=$sysfs
  for _ in {1..10}; do
    [[ -f $parent/idVendor && -f $parent/idProduct ]] && break
    candidate=$(dirname -- "$parent")
    [[ $candidate != "$parent" ]] || break
    parent=$candidate
  done
  vid=$(tr -d '[:space:]' < "$parent/idVendor" 2>/dev/null || true)
  pid=$(tr -d '[:space:]' < "$parent/idProduct" 2>/dev/null || true)
  driver=$(basename -- "$(readlink -f -- "$sysfs/driver" 2>/dev/null || printf 'unknown')")
  major_hex=$(stat -c '%t' -- "$path")
  minor_hex=$(stat -c '%T' -- "$path")
  major=$((16#$major_hex))
  minor=$((16#$minor_hex))
  if "$record_only"; then
    printf '%s\t%s\t%s\t%s:%s\t%s\n' "$path" "$major" "$minor" "${vid:-unknown}" "${pid:-unknown}" "$driver"
  else
    printf 'QMI candidate: %s (major=%s minor=%s USB=%s:%s driver=%s)\n' "$path" "$major" "$minor" "${vid:-unknown}" "${pid:-unknown}" "$driver"
  fi
}

if [[ -n $selected ]]; then
  describe "$selected" || { printf 'No valid QMI cdc-wdm device: %s\n' "$selected" >&2; exit 1; }
  exit 0
fi

shopt -s nullglob
candidates=(/dev/cdc-wdm*)
shopt -u nullglob
if ((${#candidates[@]} == 0)); then
  echo 'No QMI cdc-wdm device found.' >&2
  exit 1
fi

found=0
for path in "${candidates[@]}"; do
  if describe "$path"; then
    ((++found))
  fi
done
if ((found == 0)); then
  echo 'No QMI cdc-wdm character device found.' >&2
  exit 1
fi
