# Attribution and provenance

SIM SMS PWA is derived from **QMI Web**, originally published at:
https://github.com/xuanxuan1125/qmi-web

The original MIT copyright notice, `Copyright (c) 2026 QMI Web Contributors`, is retained verbatim in LICENSE. Additional work is released under the same MIT terms by SIM SMS PWA contributors. This repository starts with a clean, sanitized source snapshot; it does not claim to preserve upstream Git history or authorship metadata. See the upstream repository for its history.

The Go module remains named `qmi-web` so existing imports and build metadata continue to work. The internal binary/service names remain `qmi-web`. Public repository branding is SIM SMS PWA.

All vendored Go dependency notices remain in `vendor/`. Full collected npm license texts and original copyright notices are retained under `licenses/npm/`; see THIRD_PARTY_NOTICES.md. Known example cryptographic test keys inside upstream dependency test fixtures are public third-party fixtures, not operational keys. No running deployment's keys, database, sessions or subscription endpoints are included.

Apple, Raspberry Pi, Quectel and SunFounder names identify platforms/products only. The interface is an independent SMS inbox, not iMessage and not an official Apple app. No vendor endorsement is implied.
