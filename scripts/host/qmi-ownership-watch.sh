#!/usr/bin/env bash
# Observe short-lived foreign QMI clients without issuing any modem command.
# This deployment-layer helper is intentionally generic: it does not know
# about a particular host or a particular container name.
set -Eeuo pipefail

device=
duration=300
interval=1
log_file=

usage() {
  cat <<'EOF'
usage: qmi-ownership-watch.sh --device /dev/cdc-wdmX [options]

Options:
  --duration SECONDS   observation length (default: 300)
  --interval SECONDS   /proc scan interval (default: 1)
  --log PATH            append evidence to PATH
  --help               show this help

The watcher never invokes qmicli, uqmi, qmi-network, mbimcli, or ModemManager.
It reports new foreign process starts, active ModemManager, and non-QMI-Web
processes holding the selected control node. A non-zero exit means a gate
failed.
EOF
}

while (($#)); do
  case $1 in
    --device) device=${2:-}; shift 2 ;;
    --duration) duration=${2:-}; shift 2 ;;
    --interval) interval=${2:-}; shift 2 ;;
    --log) log_file=${2:-}; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
  esac
done

[[ $device =~ ^/dev/cdc-wdm[0-9]+$ && -c $device ]] || { echo 'invalid or missing --device' >&2; exit 2; }
[[ $duration =~ ^[0-9]+$ ]] || { echo '--duration must be an integer' >&2; exit 2; }
[[ $interval =~ ^([0-9]+([.][0-9]+)?|[.][0-9]+)$ ]] || { echo '--interval must be seconds' >&2; exit 2; }
command -v ps >/dev/null 2>&1 || { echo 'required command not found: ps' >&2; exit 1; }
command -v pgrep >/dev/null 2>&1 || { echo 'required command not found: pgrep' >&2; exit 1; }
command -v fuser >/dev/null 2>&1 || { echo 'required command not found: fuser' >&2; exit 1; }

if [[ -n $log_file ]]; then
  mkdir -p -- "$(dirname -- "$log_file")"
  : >> "$log_file"
  chmod 0600 "$log_file"
fi

log_event() {
  [[ -n $log_file ]] || return 0
  printf '%s\n' "$*" >> "$log_file"
}

proc_field() {
  local pid=$1 field=$2
  [[ -r /proc/$pid/stat ]] || return 0
  awk -v field="$field" '{print $field}' "/proc/$pid/stat" 2>/dev/null || true
}

proc_cmdline() {
  local pid=$1 value
  value=$(tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true)
  printf '%s' "${value:-$(ps -o args= -p "$pid" 2>/dev/null || true)}"
}

proc_detail() {
  local pid=$1 name=$2 ppid start uid gid cmd exe cwd cgroup parent_cmd grandparent_cmd
  [[ -d /proc/$pid ]] || return 0
  ppid=$(proc_field "$pid" 4)
  start=$(proc_field "$pid" 22)
  uid=$(awk '/^Uid:/{print $2}' "/proc/$pid/status" 2>/dev/null || true)
  gid=$(awk '/^Gid:/{print $2}' "/proc/$pid/status" 2>/dev/null || true)
  cmd=$(proc_cmdline "$pid")
  exe=$(readlink "/proc/$pid/exe" 2>/dev/null || true)
  cwd=$(readlink "/proc/$pid/cwd" 2>/dev/null || true)
  cgroup=$(tr '\n' ';' < "/proc/$pid/cgroup" 2>/dev/null || true)
  parent_cmd=$( [[ $ppid =~ ^[0-9]+$ ]] && proc_cmdline "$ppid" || true )
  grandparent_cmd=$( [[ $ppid =~ ^[0-9]+$ ]] && proc_cmdline "$(proc_field "$ppid" 4)" || true )
  log_event "FOREIGN_PROCESS timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ) name=$name pid=$pid ppid=$ppid start=$start uid=$uid gid=$gid exe=$exe cwd=$cwd cgroup=$cgroup cmd=$cmd parent_cmd=$parent_cmd grandparent_cmd=$grandparent_cmd"
}

is_qmi_web_pid() {
  local pid=$1 cmd
  cmd=$(proc_cmdline "$pid")
  [[ $cmd =~ (^|[[:space:]/])qmi-web([[:space:]]|$) || $cmd == */qmi-web* ]]
}

declare -A seen_exec=()
declare -A seen_open=()
foreign_qmi_exec_count=0
foreign_modemmanager_count=0
foreign_target_open_count=0
samples=0
started=$(date +%s)
((duration > 0)) && end=$((started + duration)) || end=$((started + 1))

while (( $(date +%s) < end )); do
  samples=$((samples + 1))
  for name in qmicli uqmi qmi-network mbimcli ModemManager; do
    while read -r pid; do
      [[ $pid =~ ^[0-9]+$ && -d /proc/$pid ]] || continue
      start=$(proc_field "$pid" 22)
      key="$name:$pid:$start"
      [[ ${seen_exec[$key]+yes} == yes ]] && continue
      seen_exec[$key]=1
      proc_detail "$pid" "$name"
      if [[ $name == ModemManager ]]; then
        foreign_modemmanager_count=$((foreign_modemmanager_count + 1))
      else
        foreign_qmi_exec_count=$((foreign_qmi_exec_count + 1))
      fi
    done < <(pgrep -x "$name" 2>/dev/null || true)
  done

  open_pids=()
  if command -v lsof >/dev/null 2>&1; then
    mapfile -t open_pids < <(lsof -t "$device" 2>/dev/null | sort -u || true)
  else
    mapfile -t open_pids < <(fuser -v "$device" 2>/dev/null | awk 'NR > 1 {for (i=1; i<=NF; i++) if ($i ~ /^[0-9]+$/) print $i}' | sort -u || true)
  fi
  for pid in "${open_pids[@]:-}"; do
    [[ $pid =~ ^[0-9]+$ && -d /proc/$pid ]] || continue
    is_qmi_web_pid "$pid" && continue
    start=$(proc_field "$pid" 22)
    key="open:$pid:$start"
    [[ ${seen_open[$key]+yes} == yes ]] && continue
    seen_open[$key]=1
    foreign_target_open_count=$((foreign_target_open_count + 1))
    proc_detail "$pid" "target-open"
  done
  sleep "$interval"
done

elapsed=$(( $(date +%s) - started ))
result=PASS
if ((foreign_qmi_exec_count > 0 || foreign_modemmanager_count > 0 || foreign_target_open_count > 0)); then result=FAIL; fi
printf 'FOREIGN_QMI_EXEC_COUNT=%s\nFOREIGN_MODEMMANAGER_COUNT=%s\nFOREIGN_TARGET_DEVICE_OPEN_COUNT=%s\nWATCH_SAMPLES=%s\nWATCH_DURATION_SECONDS=%s\nWATCH_RESULT=%s\n' \
  "$foreign_qmi_exec_count" "$foreign_modemmanager_count" "$foreign_target_open_count" "$samples" "$elapsed" "$result"
[[ $result == PASS ]]
