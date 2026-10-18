# Security policy

Do not post SMS bodies/codes, phone/SIM identifiers, subscriber endpoints, passwords, cookies, keys, logs or deployment backups in public issues. Use GitHub private vulnerability reporting for reproducible security findings. If that option is unavailable, open a redacted issue requesting a private reporting channel without including exploit secrets or personal data.

Supported public baseline: the latest release of this repository. Security updates may require rebuilding; the PWA update button only applies web assets already deployed by your server administrator.

The initial account is admin/admin for loopback-only bootstrap. Change it before any remote exposure. Keep HTTPS and an independent access-control layer for Internet access. Do not port-forward an unprotected dashboard. SMS, including OTP, is sensitive; notification previews may disclose it on lock screens.

The master key is stored beside the database, so application encryption is not full-disk encryption and does not protect against root/host compromise or theft of an unencrypted backup. Restore the database and master key as a pair. Never upload a production backup to GitHub.

The app never initiates a packet data session. This does not disable unrelated host software or carrier billing. Read-only WDS checks and cellular network registration are distinct from opening a paid Internet connection.
