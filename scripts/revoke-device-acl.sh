#!/usr/bin/env bash
# Restore the exact saved ACL for the one device selected by hardware mode.
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
source "$SCRIPT_DIR/common.sh"

install_dir=/opt/qmi-web
while (($#)); do
  case $1 in
    --install-dir) install_dir=${2:-}; shift 2 ;;
    -h|--help) echo 'usage: revoke-device-acl.sh [--install-dir /opt/qmi-web]'; exit 0 ;;
    *) qmi_die "unknown option: $1" ;;
  esac
done
qmi_validate_install_dir "$install_dir"
runtime="$install_dir/runtime"
backup="$runtime/device-acl.before"
identity="$runtime/device-identity.env"
[[ -f $backup && -f $identity ]] || { echo 'No saved temporary device ACL to restore.'; exit 0; }
qmi_require setfacl

device=$(sed -n 's/^DEVICE=//p' "$identity")
expected_vidpid=$(sed -n 's/^VIDPID=//p' "$identity")
[[ -n $device && -n $expected_vidpid ]] || qmi_die 'saved device identity is incomplete'
record=$("$SCRIPT_DIR/detect-qmi-device.sh" --record --device "$device") || qmi_die 'selected device is no longer a matching cdc-wdm node; refusing ACL restore on a different target'
IFS=$'\t' read -r actual_device _ _ actual_vidpid _ <<< "$record"
[[ $actual_device == "$device" && $actual_vidpid == "$expected_vidpid" ]] || qmi_die 'device identity changed; refusing ACL restore on a different target'
setfacl --restore="$backup"
echo 'Restored the saved ACL for the selected QMI device.'
