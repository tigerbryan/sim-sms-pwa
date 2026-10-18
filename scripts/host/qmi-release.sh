# Restore exactly the states captured by qmi-claim.sh isolate.
set -Eeuo pipefail
dry_run=false
state_file=
while (($#)); do
  case $1 in
    --dry-run) dry_run=true; shift ;;
    --state) state_file=${2:-}; shift 2 ;;
    -h|--help) echo 'usage: qmi-release.sh [--dry-run] --state /absolute/path/qmi-cutover-state.json'; exit 0 ;;
    *) [[ -z $state_file ]] || { echo 'unexpected argument' >&2; exit 2; }; state_file=$1; shift ;;
  esac
done
[[ $EUID -eq 0 ]] || { echo 'qmi-release.sh requires root' >&2; exit 1; }
[[ $state_file == /* && -f $state_file ]] || { echo 'usage: qmi-release.sh /absolute/path/qmi-cutover-state.json' >&2; exit 2; }
if [[ $dry_run == true ]]; then
  python3 - "$state_file" <<'PY'
import json, pathlib, sys
d=json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
print("DRY_RUN=YES")
print("RUNTIME_UNMASK=" + ",".join(d.get("runtime_masks", [])))
print("UNITS_RESTORE=" + ",".join(sorted(d.get("units", {}))))
print("MODEMMANAGER_RESTORE=" + ("recorded" if "modemmanager" in d else "not-recorded"))
print("CONTAINER_RESTORE=" + (d.get("container") or "none"))
PY
  exit 0
fi
python3 - "$state_file" <<'PY'
import json, pathlib, subprocess, sys
data=json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
def run(*args):
    subprocess.run(args, check=False, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
for unit in data.get("runtime_masks", []):
    run("systemctl","unmask","--runtime",unit)
for unit, values in data.get("units", {}).items():
    if values.get("enabled") in {"enabled", "enabled-runtime"}:
        run("systemctl","enable",unit)
    elif values.get("enabled") in {"disabled", "masked", "static"}:
        run("systemctl","disable",unit)
    if values.get("active") == "active":
        run("systemctl","start",unit)
    else:
        run("systemctl","stop",unit)
mm=data.get("modemmanager", {})
if mm.get("enabled") in {"enabled","enabled-runtime"}: run("systemctl","enable","ModemManager.service")
elif mm.get("enabled") in {"disabled","masked","static"}: run("systemctl","disable","ModemManager.service")
if mm.get("active") == "active": run("systemctl","start","ModemManager.service")
else: run("systemctl","stop","ModemManager.service")
container=data.get("container","")
if container and data.get("container_running"):
    run("docker","start",container)
PY
echo 'QMI_RELEASE=PASS'
