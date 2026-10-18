#!/usr/bin/env bash
# Install only an explicitly reviewed local binary. No network or firmware writes.
set -Eeuo pipefail
binary=./build/qmi-web
device=
mode=
while (($#)); do
  case "$1" in
    --binary) binary=${2:?binary path required}; shift 2 ;;
    --device) device=${2:?device required}; mode=qmi; shift 2 ;;
    --mock) mode=mock; shift ;;
    -h|--help) printf '%s\n' 'sudo ./scripts/install-native.sh --binary ./build/qmi-web (--mock | --device /dev/cdc-wdmX)'; exit 0 ;;
    *) printf 'Unknown option: %s\n' "$1" >&2; exit 2 ;;
  esac
done
[[ $(uname -s) == Linux && $EUID == 0 ]] || { echo 'Linux and sudo are required.' >&2; exit 1; }
[[ $mode == qmi || $mode == mock ]] || { echo 'Choose --mock or one explicit --device.' >&2; exit 1; }
[[ $mode != mock || -z $device ]] || { echo '--mock and --device are mutually exclusive.' >&2; exit 1; }
for command in systemctl install python3 curl; do command -v "$command" >/dev/null || { echo "Missing dependency: $command" >&2; exit 1; }; done
[[ -x $binary ]] || { echo 'Build a local binary with make build first.' >&2; exit 1; }
binary=$(realpath -- "$binary")
"$binary" --version
marker=/etc/qmi-web/public-installation
if [[ -f /usr/local/bin/qmi-web || -f /etc/systemd/system/qmi-web.service || -f /var/lib/qmi-web/qmi-web.db ]]; then
  [[ -f $marker ]] || { echo 'Existing installation is not managed by this installer; migrate manually after a backup.' >&2; exit 1; }
fi
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
previous_active=false
rollback_dir=
if [[ -f $marker ]]; then
  systemctl is-active --quiet qmi-web && previous_active=true
  rollback_dir=$(mktemp -d /var/tmp/sim-sms-install.XXXXXX)
  chmod 0700 "$rollback_dir"
  cp -p /usr/local/bin/qmi-web "$rollback_dir/qmi-web"
  cp -p /etc/systemd/system/qmi-web.service "$rollback_dir/qmi-web.service"
  if [[ -f /var/lib/qmi-web/qmi-web.db ]]; then python3 "$root/ops/sms-backup.py"; fi
fi
recover_install() {
  local status=$?
  trap - ERR
  if [[ -n $rollback_dir ]]; then
    systemctl stop qmi-web || true
    cp -p "$rollback_dir/qmi-web" /usr/local/bin/qmi-web
    cp -p "$rollback_dir/qmi-web.service" /etc/systemd/system/qmi-web.service
    systemctl daemon-reload
    if [[ $previous_active == true ]]; then systemctl start qmi-web || true; fi
    rm -rf -- "$rollback_dir"
  fi
  echo 'Installation failed; prior binary/service restored when available. Review the private database backup before any further recovery.' >&2
  exit "$status"
}
trap recover_install ERR
if [[ $mode == qmi ]]; then
  [[ $device =~ ^/dev/cdc-wdm[0-9]+$ && -c $device ]] || { echo 'Select an existing /dev/cdc-wdmX character device.' >&2; exit 1; }
  command -v fuser >/dev/null || { echo 'Install psmisc for fuser before installing hardware mode.' >&2; exit 1; }
  # For upgrades, release only our own process before the final ownership check.
  if [[ -f $marker ]]; then systemctl stop qmi-web; fi
  if fuser "$device" >/dev/null 2>&1; then
    echo 'Device is busy. Resolve the owner explicitly; no other service was stopped.' >&2
    false
  fi
fi
install -d -m 0750 /etc/qmi-web /var/lib/qmi-web /var/log/qmi-web
if [[ ! -e /etc/qmi-web/config.yaml ]]; then
  install -m 0640 "$root/config/config.example.yaml" /etc/qmi-web/config.yaml
fi
# Preserve all user configuration. Backend and explicit node are service overrides.
install -m 0755 "$binary" /usr/local/bin/qmi-web.new
if [[ -f $marker ]]; then systemctl stop qmi-web; fi
mv /usr/local/bin/qmi-web.new /usr/local/bin/qmi-web
cat > /etc/systemd/system/qmi-web.service <<EOF
[Unit]
Description=SIM SMS PWA receive-only inbox
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=300
StartLimitBurst=5
[Service]
Type=simple
User=root
WorkingDirectory=/var/lib/qmi-web
Environment=QMI_WEB_BACKEND=$mode
Environment=QMI_WEB_SMS_ONLY=true
Environment=QMI_WEB_DEVICE=$device
ExecStart=/usr/local/bin/qmi-web --config /etc/qmi-web/config.yaml --data /var/lib/qmi-web --logs /var/log/qmi-web
Restart=on-failure
RestartSec=10
TimeoutStopSec=30
UMask=0077
NoNewPrivileges=yes
CapabilityBoundingSet=
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=/etc/qmi-web /var/lib/qmi-web /var/log/qmi-web
DevicePolicy=closed
EOF
if [[ -n $device ]]; then printf 'DeviceAllow=%s rw\n' "$device" >> /etc/systemd/system/qmi-web.service; fi
cat >> /etc/systemd/system/qmi-web.service <<'EOF'
[Install]
WantedBy=multi-user.target
EOF
printf '%s\n' 'SIM SMS PWA native installer' > "$marker"
chmod 0600 "$marker"
systemctl daemon-reload
systemctl enable --now qmi-web
ready=false
for ((i=0;i<30;i++)); do
  if curl -fsS --max-time 2 http://127.0.0.1:7580/ready | python3 -c 'import json,sys;sys.exit(0 if json.load(sys.stdin).get("ready") is True else 1)' >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
if [[ $ready != true ]]; then
  echo 'Application not ready. Inspect systemctl status qmi-web and journalctl -u qmi-web; do not expose remotely.' >&2
  false
fi
trap - ERR
if [[ -n $rollback_dir ]]; then rm -rf -- "$rollback_dir"; fi
echo 'Installed and ready on loopback port 7580. Change the initial password via SSH forwarding, then configure HTTPS and access control.'
echo 'HTTP readiness alone does NOT prove SIM registration or real SMS receipt. Complete docs/DEPLOYMENT.md acceptance checks.'
