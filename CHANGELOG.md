# Changelog

## 1.1.1 — 2026-10-18

- Complete frontend attribution, including Lucide ISC and Feather-derived MIT notices.
- Preserve full collected npm license texts in source and Linux release packages, alongside existing Go notices.
- Correct outdated offline-cache statements; retain original 1.1.0 binaries and provide supplemental licenses separately.
- Upgrade transitive `github.com/golang-jwt/jwt/v5` from 5.2.1 to 5.2.2 to address GHSA-mh63-6h87-95cp.
- SMS reception and modem operations are unchanged by this release.

## 1.1.0 — 2026-10-18

First independent public source snapshot derived from MIT-licensed QMI Web.

- Mobile-first receive-only inbox, search, conversation reading and dark/light themes.
- Persistent encrypted Web Push delivery queue, retries and supported-platform unread badges.
- Manual PWA asset updates; clearer limits for installed icon changes.
- SMS reconciliation and bounded device reconnect behavior; static receive-only safety tests.
- Generic hardware/buying guide, full Linux deployment, operations/backup guide and copyable agent prompt.
- Personal deployment records and identifiers excluded; fresh public history and upstream notices retained.

Hardware and notification behavior depend on the user's module, carrier, operating system and permissions. See README for verified scope and limitations.
