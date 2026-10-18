#!/usr/bin/env bash
# Remove this native service only. Keep SMS, credentials and configuration.
set -Eeuo pipefail
[[ $(uname -s) == Linux && $EUID == 0 ]] || { echo 'Linux and sudo required.' >&2; exit 1; }
[[ $# == 0 ]] || { echo 'No purge option: user data is always retained.' >&2; exit 2; }
[[ -f /etc/qmi-web/public-installation ]] || { echo 'Refusing to remove an installation not managed by this project.' >&2; exit 1; }
systemctl disable --now qmi-web
rm -f /etc/systemd/system/qmi-web.service /usr/local/bin/qmi-web
systemctl daemon-reload
echo 'Native service removed. Configuration, database, master key, logs and backups remain in place. Review optional backup/health timers separately.'
