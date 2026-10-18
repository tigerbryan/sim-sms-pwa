#!/usr/bin/env bash
# Create a permission-restricted, SQLite-consistent QMI Web backup.
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
source "$SCRIPT_DIR/common.sh"

install_dir=/opt/qmi-web
output_dir=
while (($#)); do
  case $1 in
    --install-dir) install_dir=${2:-}; shift 2 ;;
    --output-dir) output_dir=${2:-}; shift 2 ;;
    -h|--help) echo 'usage: backup.sh [--install-dir /opt/qmi-web] [--output-dir DIR]'; exit 0 ;;
    *) qmi_die "unknown option: $1" ;;
  esac
done
qmi_validate_install_dir "$install_dir"
data="$install_dir/data"
config="$install_dir/config"
runtime="$install_dir/runtime"
database="$data/qmi-web.db"
helper="$runtime/qmi-backup"
checker="$runtime/qmi-dbcheck"
[[ -f $database ]] || qmi_die "database does not exist: $database"
[[ -x $helper && -x $checker ]] || qmi_die "installed backup helpers are missing; reinstall from the same offline bundle"
output_dir=${output_dir:-$install_dir/backups}
qmi_validate_install_dir "$output_dir"
qmi_require tar
umask 077
mkdir -p -- "$output_dir"
stage=$(mktemp -d "$output_dir/.qmi-web-backup.XXXXXX")
trap 'rm -rf -- "$stage"' EXIT
mkdir -p -- "$stage/config" "$stage/data/secrets" "$stage/meta"

"$helper" "$database" "$stage/data/qmi-web.db"
"$checker" "$stage/data/qmi-web.db"
if [[ -f $config/config.yaml ]]; then cp -p -- "$config/config.yaml" "$stage/config/config.yaml"; fi
if [[ -f $data/secrets/master.key ]]; then cp -p -- "$data/secrets/master.key" "$stage/data/secrets/master.key"; fi
if [[ -f $runtime/installed.env ]]; then cp -p -- "$runtime/installed.env" "$stage/meta/installed.env"; fi
printf 'QMI Web backup\ncreated_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$stage/manifest.txt"

stamp=$(date -u +%Y%m%dT%H%M%SZ)
archive="$output_dir/qmi-web-$stamp.tar.gz"
tar -C "$stage" -czf "$archive" .
chmod 600 "$archive"
printf '%s\n' "$archive"
