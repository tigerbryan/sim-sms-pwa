# Third-party notices

QMI Web is MIT-licensed. It distributes its own source under MIT and preserves
the license or notice files supplied with every vendored Go module.

Derived from [xuanxuan1125/qmi-web](https://github.com/xuanxuan1125/qmi-web); original MIT notice retained in LICENSE. See NOTICE.md.

## Direct Go dependencies

- github.com/SherClockHolmes/webpush-go v1.4.0 — MIT; license retained in vendor.
- github.com/iniwex5/qmi-go v0.6.4 — MIT; license retained in vendor.
- github.com/warthog618/sms v0.3.0 — MIT; license retained in vendor.
- golang.org/x/crypto v0.54.0 and golang.org/x/term v0.45.0 — BSD-style Go
  Authors notices retained in vendor.
- gopkg.in/yaml.v3 v3.0.1 — MIT and Apache-2.0 notice retained in vendor.
- modernc.org/sqlite v1.56.0 — BSD-style notice retained in vendor.

Transitive Go modules are recorded in vendor/modules.txt. Their shipped
license files remain in the corresponding vendor directories. QMI Web carries
no patch to third-party source; see docs/THIRD_PARTY_PATCHES.md.

## Frontend dependencies

The locked npm dependency graph is in web/package-lock.json. The Offline
Bundle carries npm's package metadata/cache required for its exact lockfile.
Vue, Vue Router, Pinia, Vite, Vitest, Vue Test Utils, jsdom, and TypeScript are
used under their upstream license terms. The bundle does not relabel or remove
their notices.

See docs/DEPENDENCIES.md for the machine-readable source locations and direct
dependency inventory.
