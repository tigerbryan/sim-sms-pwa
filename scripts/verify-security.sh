#!/usr/bin/env bash
# Static security, PII, and deployment-policy checks. This does not touch QMI
# hardware and does not install scanning tools or contact the network.
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
source "$SCRIPT_DIR/common.sh"
ROOT=$(qmi_root)
failed=0

fail_if_matches() {
  local description=$1 pattern=$2
  shift 2
  if grep -RInE --include='*.go' --exclude='*_test.go' "$pattern" "$@" 2>/dev/null; then
    echo "FAILED: $description" >&2
    failed=1
  fi
}

# Production Go source is intentionally receive-only. Test fixtures are not
# searched because they may document rejected operations.
fail_if_matches 'data-session operation found in production Go source' 'StartNetwork|SetProfile|CreateProfile|ModifyProfile|SetAutoconnect|SetAPN|WDSStart|qmi-network[[:space:]]+start|uqmi' "$ROOT/cmd" "$ROOT/internal"
fail_if_matches 'routing, DHCP, NAT, or AT executor found in production Go source' 'ip route add default|dhclient|udhcpc|MASQUERADE|AT[+]|exec\.Command|usb_modeswitch' "$ROOT/cmd" "$ROOT/internal"
fail_if_matches 'SMS send/delete/configuration operation found in production Go source' 'SendRawMessage|SendFromStorage|RawWriteMessage|DeleteMessage|ModifyMessageTag|SetRoutes|SetSMSC|SendAck' "$ROOT/cmd" "$ROOT/internal"

for compose in "$ROOT/compose.no-device.yaml" "$ROOT/compose.hardware.yaml"; do
  for required in 'user: "65532:65532"' 'cap_add: []' 'cap_drop:' '- ALL' 'read_only: true' 'privileged: false' 'network_mode: bridge'; do
    grep -Fq -- "$required" "$compose" || { echo "FAILED: $(basename "$compose") lacks $required" >&2; failed=1; }
  done
  if grep -nE 'user: "0:0"|/dev:/dev|ttyUSB|ttyACM|docker\.sock|c \*:\*|:\* rw| rwm|devices:' "$compose"; then
    echo "FAILED: unsafe device or privilege setting in $(basename "$compose")" >&2
    failed=1
  fi
done
if grep -nE 'QMI_WEB_DEVICE|device_cgroup_rules|devices:' "$ROOT/compose.no-device.yaml"; then
  echo 'FAILED: no-device compose contains modem access' >&2
  failed=1
fi
grep -Fq 'device_cgroup_rules:' "$ROOT/compose.hardware.yaml" || { echo 'FAILED: hardware compose lacks exact cgroup rule' >&2; failed=1; }
grep -Fq 'type: bind' "$ROOT/compose.hardware.yaml" || { echo 'FAILED: hardware compose lacks exact bind mount' >&2; failed=1; }

# High-confidence credentials and local identifiers. Only locations are shown;
# values are never echoed by this checker.
scan_files=("$ROOT/README.md" "$ROOT/LICENSE" "$ROOT/THIRD_PARTY_NOTICES.md" "$ROOT/install.sh")
while IFS= read -r file; do scan_files+=("$file"); done < <(find "$ROOT/cmd" "$ROOT/config" "$ROOT/docs" "$ROOT/internal" "$ROOT/scripts" "$ROOT/tests" "$ROOT/web" -type f \( -name '*.go' -o -name '*.md' -o -name '*.sh' -o -name '*.py' -o -name '*.yaml' -o -name '*.yml' -o -name '*.ts' -o -name '*.vue' -o -name '*.json' \) ! -name 'verify-security.sh' ! -path '*/node_modules/*' ! -path '*/dist/*')
private_identifiers='(^|[^A-Za-z])(''f''qxku|xu''an)([^A-Za-z]|$)'
hardware_identifier='A''235'
windows_user_path='C:\\''Users\\'
nas_application_path='/vol1/''docker/'
credential_object='PS''Credential'
for pattern in '-----BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY-----' 'gh[pousr]_[A-Za-z0-9_]{20,}' 'AKIA[0-9A-Z]{16}' 'xox[baprs]-[A-Za-z0-9-]{20,}' "$private_identifiers" "$hardware_identifier" "$windows_user_path" "$nas_application_path" "$credential_object"; do
  for file in "${scan_files[@]}"; do
    if grep -nEi -- "$pattern" "$file" >/dev/null 2>&1; then
      echo "FAILED: sensitive-pattern match in ${file#$ROOT/}" >&2
      failed=1
    fi
  done
done

if command -v shellcheck >/dev/null 2>&1; then
  # Follow the local common.sh helper. The excluded findings are reviewed
  # information-only false positives from intentional path-fragment scanning
  # and the manifest's explicit self-exclusion; warnings and errors still fail.
  shellcheck -x -e SC1003,SC1091,SC2094,SC2295 "$ROOT/install.sh" "$ROOT/scripts/"*.sh "$ROOT/scripts/host/"*.sh
else
  bash -n "$ROOT/install.sh" "$ROOT/scripts/"*.sh "$ROOT/scripts/host/"*.sh
fi

if (( failed != 0 )); then
  exit 1
fi
echo 'security verification passed: static SMS-only, least privilege, and secret/PII checks are clean'
