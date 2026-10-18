# Contributor / agent instructions

This is a public, receive-only SMS gateway derived from QMI Web (MIT). Read README.md, NOTICE.md, SECURITY.md and the deployment/PWA guides before changing behavior.

- Preserve SMS-only behavior: no send/delete SMS, packet-data startup, APN/route/DNS/NAT changes, modem resets, AT writes or firmware flashing.
- Real QMI hardware support is Linux-only. A successful mock run is not hardware validation.
- Never commit databases, real SMS, OTP, logs, backups, credentials, cookies, push subscriptions or device identifiers. Use fictional fixtures and screenshots.
- Retain upstream attribution and all vendored licenses. Do not edit vendored sources without documenting patches.
- Keep manifest identity, start URL, worker registration, encrypted database and master key stable across updates. No browser update API can guarantee replacing an OS-owned installed icon.
- Prefer existing APIs/SSH/official CLIs. User gestures are required for credentials and notification permission.
- Run the relevant Go/frontend tests, build, static safety checks, public-tree privacy checker and ops tests. Report tested versus untested hardware paths clearly.
- Do not modify a running installation just to prepare a source release. Build artifacts from the sanitized source tree only.
