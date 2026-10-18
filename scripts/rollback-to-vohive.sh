#!/usr/bin/env bash
# Safely switch an installed QMI Web hardware deployment back to no-device
# mode and return the modem to an explicitly named VoHive deployment.
#
# This script deliberately requires deployment metadata and explicit VoHive
# identifiers. It never guesses a device path, container name, or systemd
# unit, never deletes QMI Web data, and never changes the complete Docker stack.
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
source "$SCRIPT_DIR/common.sh"
ROOT=$(qmi_root)

install_dir=/opt/qmi-web
vohive_container=
vohive_timer=
confirm=false

usage() {
  cat <<'EOF'
usage: rollback-to-vohive.sh --confirm \
  --vohive-container NAME --vohive-timer UNIT [--install-dir /opt/qmi-web]

The command backs up QMI Web data, stops only the installed qmi-web service,
recreates it in no-device mode, restores the saved QMI device ACL, then
enables/starts the explicitly named VoHive hardware automation and container.
Persistent QMI Web data, config, secrets, images, and backups are retained.
EOF
}

while (($#)); do
  case $1 in
    --install-dir) install_dir=${2:-}; shift 2 ;;
    --vohive-container) vohive_container=${2:-}; shift 2 ;;
    --vohive-timer) vohive_timer=${2:-}; shift 2 ;;
    --confirm) confirm=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) qmi_die "unknown option: $1" ;;
  esac
done

qmi_validate_install_dir "$install_dir"
[[ $confirm == true ]] || qmi_die 'rollback requires --confirm'
[[ $EUID -eq 0 ]] || qmi_die 'run rollback as root'
[[ $vohive_container =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || qmi_die 'invalid --vohive-container'
[[ $vohive_timer =~ ^[A-Za-z0-9_.@:-]+$ ]] || qmi_die 'invalid --vohive-timer'
qmi_require docker
qmi_require systemctl
qmi_require setfacl
qmi_require fuser

runtime="$install_dir/runtime"
metadata="$runtime/installed.env"
compose="$install_dir/compose.yaml"
identity="$runtime/device-identity.env"
acl_backup="$runtime/device-acl.before"
[[ -f $metadata && -f $compose ]] || qmi_die 'installed QMI Web metadata/compose is missing'

mode=$(sed -n 's/^MODE=//p' "$metadata")
port=$(sed -n 's/^PORT=//p' "$metadata")
container_name=$(sed -n 's/^CONTAINER_NAME=//p' "$metadata")
[[ $mode == hardware ]] || qmi_die "installed mode is '$mode'; refusing a non-hardware rollback"
[[ $port =~ ^[0-9]+$ && $port -ge 1 && $port -le 65535 ]] || qmi_die 'installed port metadata is invalid'
[[ $container_name =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || qmi_die 'installed container metadata is invalid'
[[ $container_name != "$vohive_container" ]] || qmi_die 'QMI Web and VoHive container names must differ'
grep -Eq '^[[:space:]]+qmi-web:' "$compose" || qmi_die 'installed compose does not contain the qmi-web service'

docker inspect "$container_name" >/dev/null 2>&1 || qmi_die 'installed QMI Web container is missing'
compose_service=$(docker inspect --format '{{index .Config.Labels "com.docker.compose.service"}}' "$container_name" 2>/dev/null || true)
[[ $compose_service == qmi-web ]] || qmi_die 'refusing a container without the expected qmi-web compose identity'
docker inspect "$vohive_container" >/dev/null 2>&1 || qmi_die 'VoHive container does not exist'
systemctl cat "$vohive_timer" >/dev/null 2>&1 || qmi_die 'VoHive automation unit does not exist'

[[ -f $identity && -f $acl_backup ]] || qmi_die 'saved hardware device identity/ACL is missing'
device=$(sed -n 's/^DEVICE=//p' "$identity")
expected_vidpid=$(sed -n 's/^VIDPID=//p' "$identity")
[[ $device == /dev/cdc-wdm* && -n $expected_vidpid ]] || qmi_die 'saved device identity is incomplete'
record=$("$SCRIPT_DIR/detect-qmi-device.sh" --record --device "$device") || qmi_die 'saved device is no longer the same QMI node'
IFS=$'\t' read -r actual_device _ _ actual_vidpid _ <<< "$record"
[[ $actual_device == "$device" && $actual_vidpid == "$expected_vidpid" ]] || qmi_die 'device identity changed; refusing rollback on a different node'

vohive_was_running=$(docker inspect --format '{{.State.Running}}' "$vohive_container")
timer_was_active=$(systemctl is-active "$vohive_timer" 2>/dev/null || true)
timer_was_enabled=$(systemctl is-enabled "$vohive_timer" 2>/dev/null || true)
qmi_was_running=$(docker inspect --format '{{.State.Running}}' "$container_name")
[[ $qmi_was_running == true ]] || qmi_die 'QMI Web hardware container is not running; refusing an ambiguous rollback'
[[ $vohive_was_running == false ]] || qmi_die 'VoHive is already running; stop it and retry to avoid two hardware owners'

stamp=$(date +%Y%m%d-%H%M%S)
compose_backup="$runtime/rollback-compose-$stamp.yaml"
metadata_backup="$runtime/rollback-installed-$stamp.env"
umask 077
cp -p -- "$compose" "$compose_backup"
cp -p -- "$metadata" "$metadata_backup"

restore_previous_state() {
  local status=$1
  set +e
  systemctl stop "$vohive_timer" >/dev/null 2>&1 || true
  docker stop "$vohive_container" >/dev/null 2>&1 || true
  docker stop "$container_name" >/dev/null 2>&1 || true
  cp -p -- "$compose_backup" "$compose" >/dev/null 2>&1 || true
  cp -p -- "$metadata_backup" "$metadata" >/dev/null 2>&1 || true
  setfacl --restore="$acl_backup" >/dev/null 2>&1 || true
  qmi_compose -f "$compose" up -d --no-build >/dev/null 2>&1 || true
  if [[ $timer_was_enabled == enabled || $timer_was_enabled == enabled-runtime ]]; then
    systemctl enable "$vohive_timer" >/dev/null 2>&1 || true
  else
    systemctl disable "$vohive_timer" >/dev/null 2>&1 || true
  fi
  if [[ $timer_was_active == active ]]; then systemctl start "$vohive_timer" >/dev/null 2>&1 || true; fi
  if [[ $vohive_was_running == true ]]; then docker start "$vohive_container" >/dev/null 2>&1 || true; fi
  printf 'ROLLBACK_TO_VOHIVE=FAIL\nRECOVERY_ATTEMPTED=yes\n' >&2
  exit "$status"
}

trap 'status=$?; if (( status != 0 )); then restore_previous_state "$status"; fi' EXIT

"$SCRIPT_DIR/backup.sh" --install-dir "$install_dir" >/dev/null
systemctl stop "$vohive_timer" >/dev/null 2>&1 || qmi_die 'failed to stop the named VoHive automation unit'
qmi_compose -f "$compose" stop qmi-web >/dev/null || qmi_die 'failed to stop only the installed qmi-web service'
[[ $(docker inspect --format '{{.State.Running}}' "$container_name" 2>/dev/null || true) == false ]] || qmi_die 'qmi-web is still running'

# Reuse the supported lifecycle path. It preserves data/config/admin and
# writes a no-device compose without touching the rest of the Docker stack.
"$ROOT/install.sh" --mode no-device --install-dir "$install_dir" --port "$port" \
  --container-name "$container_name" --upgrade

new_mode=$(sed -n 's/^MODE=//p' "$metadata")
[[ $new_mode == no-device ]] || qmi_die 'QMI Web did not finish in no-device mode'
new_mounts=$(docker inspect --format '{{json .Mounts}}' "$container_name")
new_rules=$(docker inspect --format '{{json .HostConfig.DeviceCgroupRules}}' "$container_name")
new_devices=$(docker inspect --format '{{json .HostConfig.Devices}}' "$container_name")
[[ $new_mounts != *'/dev/cdc-wdm'* && $new_mounts != *'/dev/ttyUSB'* ]] || qmi_die 'no-device container still has a modem device mount'
[[ $new_rules == 'null' || $new_rules == '[]' ]] || qmi_die 'no-device container still has device cgroup rules'
[[ $new_devices == 'null' || $new_devices == '[]' ]] || qmi_die 'no-device container still has Docker devices'
qmi_wait_http_200 "$port" /health 30 || qmi_die 'no-device health check failed'
qmi_wait_http_200 "$port" /ready 10 || qmi_die 'no-device ready check failed'

"$SCRIPT_DIR/revoke-device-acl.sh" --install-dir "$install_dir" >/dev/null
systemctl enable --now "$vohive_timer" >/dev/null || qmi_die 'failed to enable/start the named VoHive automation unit'
docker start "$vohive_container" >/dev/null 2>&1 || {
  [[ $(docker inspect --format '{{.State.Running}}' "$vohive_container" 2>/dev/null || true) == true ]] || qmi_die 'failed to start VoHive container'
}

for _ in {1..30}; do
  [[ $(docker inspect --format '{{.State.Running}}' "$vohive_container" 2>/dev/null || true) == true ]] && break
  sleep 1
done
[[ $(docker inspect --format '{{.State.Running}}' "$vohive_container") == true ]] || qmi_die 'VoHive container did not become running'
[[ $(docker inspect --format '{{.State.Running}}' "$container_name") == true ]] || qmi_die 'QMI Web no-device container is not running'
holder=$(fuser "$device" 2>/dev/null || true)
[[ -n $holder ]] || qmi_die 'VoHive device-owner verification is inconclusive'

trap - EXIT
printf 'ROLLBACK_TO_VOHIVE=PASS\nQMI_MODE=no-device\nQMI_CONTAINER=%s\nVOHIVE_RUNNING=true\nVOHIVE_TIMER=active\nDEVICE_OWNER=present\nQMI_DATA_RETAINED=yes\n' "$container_name"
