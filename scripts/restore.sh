#!/usr/bin/env bash
# Restore only a user-selected backup after validation. It never stops a
# container automatically and preserves a pre-restore backup first.
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
source "$SCRIPT_DIR/common.sh"

install_dir=/opt/qmi-web
archive=
confirm=false
while (($#)); do
  case $1 in
    --install-dir) install_dir=${2:-}; shift 2 ;;
    --file) archive=${2:-}; shift 2 ;;
    --confirm) confirm=true; shift ;;
    -h|--help) echo 'usage: restore.sh --file BACKUP.tar.gz --confirm [--install-dir /opt/qmi-web]'; exit 0 ;;
    *) qmi_die "unknown option: $1" ;;
  esac
done
qmi_validate_install_dir "$install_dir"
[[ $confirm == true && -n $archive && -f $archive ]] || qmi_die 'restore needs --file BACKUP.tar.gz --confirm'
qmi_require tar
runtime="$install_dir/runtime"
checker="$runtime/qmi-dbcheck"
[[ -x $checker ]] || qmi_die 'installed database checker is missing; reinstall from the same offline bundle'
if command -v docker >/dev/null 2>&1 && docker ps --format '{{.Names}}' | grep -qx 'qmi-web'; then
  qmi_die 'qmi-web is running; stop this known container manually before an explicit restore'
fi

while IFS= read -r entry; do
  [[ $entry != /* && $entry != *'..'* ]] || qmi_die 'backup archive contains an unsafe path'
done < <(tar -tzf "$archive")
stage=$(mktemp -d "${TMPDIR:-/tmp}/qmi-web-restore.XXXXXX")
trap 'rm -rf -- "$stage"' EXIT
tar -C "$stage" -xzf "$archive"
[[ -f $stage/data/qmi-web.db ]] || qmi_die 'backup does not contain data/qmi-web.db'
"$checker" "$stage/data/qmi-web.db"

if [[ -f $install_dir/data/qmi-web.db ]]; then
  "$SCRIPT_DIR/backup.sh" --install-dir "$install_dir" >/dev/null
fi
mkdir -p -- "$install_dir/data/secrets" "$install_dir/config"
tmp_db="$install_dir/data/.qmi-web.db.restore.$$"
cp -p -- "$stage/data/qmi-web.db" "$tmp_db"
chmod 600 "$tmp_db"
mv -f -- "$tmp_db" "$install_dir/data/qmi-web.db"
rm -f -- "$install_dir/data/qmi-web.db-wal" "$install_dir/data/qmi-web.db-shm"
if [[ -f $stage/config/config.yaml ]]; then cp -p -- "$stage/config/config.yaml" "$install_dir/config/config.yaml"; fi
if [[ -f $stage/data/secrets/master.key ]]; then cp -p -- "$stage/data/secrets/master.key" "$install_dir/data/secrets/master.key"; chmod 600 "$install_dir/data/secrets/master.key"; fi
echo 'restore complete; start qmi-web manually after reviewing the preserved pre-restore backup'
