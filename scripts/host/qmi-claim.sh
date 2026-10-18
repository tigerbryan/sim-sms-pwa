#!/usr/bin/env bash
# Host-side, deployment-specific QMI claim helper. It never runs inside the
# QMI Web container and never broadens device or capability access.
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
watcher="$SCRIPT_DIR/qmi-ownership-watch.sh"
state_file=/var/lib/qmi-web/qmi-cutover-state.json
device=
mode=status
duration=300
log_file=
container=
selected_units=()
mask_modemmanager=false

usage() {
  cat <<'EOF'
usage: qmi-claim.sh status --device /dev/cdc-wdmX
       qmi-claim.sh observe --device /dev/cdc-wdmX [--duration 300] [--log PATH]
       qmi-claim.sh isolate --device /dev/cdc-wdmX --state PATH \
         [--container NAME] [--unit UNIT]... [--modemmanager]

status and observe are read-only. isolate is intentionally explicit and only
stops/masks named units selected by the operator; it does not guess a product or
ModemManager state. Use qmi-release.sh with the saved state to restore it.
EOF
}

[[ $# -ge 1 ]] || { usage >&2; exit 2; }
mode=$1; shift
while (($#)); do
  case $1 in
    --device) device=${2:-}; shift 2 ;;
    --duration) duration=${2:-}; shift 2 ;;
    --log) log_file=${2:-}; shift 2 ;;
    --state) state_file=${2:-}; shift 2 ;;
    --container) container=${2:-}; shift 2 ;;
    --unit) selected_units+=("${2:-}"); shift 2 ;;
    --modemmanager) mask_modemmanager=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
  esac
done

[[ $device =~ ^/dev/cdc-wdm[0-9]+$ ]] || { echo 'an explicit --device /dev/cdc-wdmX is required' >&2; exit 2; }
[[ -x $watcher ]] || { echo "missing watcher: $watcher" >&2; exit 1; }

device_info() {
  local sysfs driver parent vid pid iface
  sysfs=$(readlink -f "/sys/class/usbmisc/$(basename "$device")/device" 2>/dev/null || true)
  driver=$(basename "$(readlink -f "$sysfs/driver" 2>/dev/null || printf unknown)")
  parent=$sysfs
  for _ in {1..10}; do
    [[ -f $parent/idVendor && -f $parent/idProduct ]] && break
    parent=$(dirname "$parent")
  done
  vid=$(tr -d '[:space:]' < "$parent/idVendor" 2>/dev/null || true)
  pid=$(tr -d '[:space:]' < "$parent/idProduct" 2>/dev/null || true)
  iface=none
  for net in /sys/class/net/*; do
    [ -e "$net/device" ] || continue
    if [[ $(readlink -f "$net/device" 2>/dev/null || true) == "$sysfs" ]]; then iface=$(basename "$net"); break; fi
  done
  printf '%s\n' "DEVICE=$device" "MAJOR_MINOR=$(stat -c '%t:%T' "$device" 2>/dev/null || true)" "DRIVER=$driver" "VIDPID=${vid:-unknown}:${pid:-unknown}" "IFACE=$iface"
}

units=()

case $mode in
  status)
    device_info
    echo 'CONFLICTING_UNITS'
    for unit in "${selected_units[@]}"; do
      printf '%s enabled=%s active=%s\n' "$unit" "$(systemctl is-enabled "$unit" 2>/dev/null || true)" "$(systemctl is-active "$unit" 2>/dev/null || true)"
    done
    echo 'FOREIGN_PROCESSES'
    pgrep -a -x qmicli || true
    pgrep -a -x uqmi || true
    pgrep -a -x qmi-network || true
    pgrep -a -x ModemManager || true
    ;;
  observe)
    exec "$watcher" --device "$device" --duration "$duration" ${log_file:+--log "$log_file"}
    ;;
  isolate)
    [[ $EUID -eq 0 ]] || { echo 'isolate requires root' >&2; exit 1; }
    [[ -n $state_file && $state_file == /* ]] || { echo 'isolate requires absolute --state PATH' >&2; exit 2; }
    if ((${#selected_units[@]} == 0)) && [[ $mask_modemmanager != true && -z $container ]]; then
      echo 'isolate requires explicit --unit, --modemmanager, or --container' >&2
      exit 2
    fi
    for unit in "${selected_units[@]}"; do
      [[ $unit =~ ^[A-Za-z0-9_.@:-]+$ ]] || { echo "invalid unit: $unit" >&2; exit 2; }
    done
    if [[ -n $container ]]; then
      [[ $container =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || { echo 'invalid container name' >&2; exit 2; }
    fi
    mkdir -p -- "$(dirname "$state_file")"
    umask 077
    python3 - "$state_file" "$device" "$container" "$mask_modemmanager" "${selected_units[@]}" <<'PY'
import json, pathlib, subprocess, sys, time
path=pathlib.Path(sys.argv[1])
device=sys.argv[2]
container=sys.argv[3]
mask_modemmanager=sys.argv[4] == "true"
units=sys.argv[5:]
def run(*args):
    return subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=False).stdout.strip()
state={"timestamp":time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),"device":device,"major_minor":run("stat","-c","%t:%T",device),"container":container,"runtime_masks":[],"units":{}}
for unit in units:
    state["units"][unit]={"enabled":run("systemctl","is-enabled",unit),"active":run("systemctl","is-active",unit)}
if mask_modemmanager:
    state["modemmanager"]={"enabled":run("systemctl","is-enabled","ModemManager.service"),"active":run("systemctl","is-active","ModemManager.service")}
if container:
    state["container_running"] = run("docker","inspect","--format","{{.State.Running}}",container) == "true"
path.write_text(json.dumps(state,indent=2)+"\n",encoding="utf-8")
PY
    for unit in "${selected_units[@]}"; do
      systemctl stop "$unit" >/dev/null 2>&1 || true
      systemctl mask --runtime "$unit" >/dev/null
      printf '%s\n' "$unit" >> "$state_file.runtime-masks"
      printf 'RUNTIME_MASKED=%s\n' "$unit"
    done
    if [[ $mask_modemmanager == true ]]; then
      systemctl stop ModemManager.service >/dev/null 2>&1 || true
      systemctl mask --runtime ModemManager.service >/dev/null
      printf '%s\n' ModemManager.service >> "$state_file.runtime-masks"
      printf 'RUNTIME_MASKED=ModemManager.service\n'
    fi
    if [[ -n $container ]]; then
      docker stop --time 30 "$container" >/dev/null 2>&1 || true
      printf 'CONTAINER_STOPPED=%s\n' "$container"
    fi
    python3 - "$state_file" "$state_file.runtime-masks" <<'PY'
import json, pathlib, sys
path, masks = map(pathlib.Path, sys.argv[1:])
data=json.loads(path.read_text(encoding="utf-8"))
data["runtime_masks"]=[line for line in masks.read_text(encoding="utf-8").splitlines() if line]
masks.unlink(missing_ok=True)
path.write_text(json.dumps(data,indent=2)+"\n",encoding="utf-8")
PY
    if "$watcher" --device "$device" --duration 1; then
      echo 'DEVICE_FREE=YES'
    else
      echo 'DEVICE_FREE=NO' >&2
      exit 1
    fi
    echo "SAVED_STATE=$state_file"
    ;;
  *) usage >&2; exit 2 ;;
esac
