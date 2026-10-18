# Dependency inventory

The checked-in go.mod, go.sum, vendor/modules.txt, web/package.json, and
web/package-lock.json are the authoritative resolved dependency records.

## Direct Go dependencies

| Module | Version | License status |
| --- | --- | --- |
| github.com/iniwex5/qmi-go | v0.6.4 | MIT |
| github.com/warthog618/sms | v0.3.0 | MIT |
| golang.org/x/crypto | v0.54.0 | BSD-style Go Authors notice |
| golang.org/x/term | v0.45.0 | BSD-style Go Authors notice |
| gopkg.in/yaml.v3 | v3.0.1 | MIT and Apache-2.0 notices |
| modernc.org/sqlite | v1.56.0 | BSD-style notice |

## Direct npm dependencies

Vue, Vue Router, Pinia, Vite, Vitest, Vue Test Utils, jsdom, and their locked
transitives are recorded in web/package-lock.json. Their package metadata and
license fields are preserved in the release npm cache.

No third-party source is modified by QMI Web. See THIRD_PARTY_NOTICES.md and
THIRD_PARTY_PATCHES.md.
