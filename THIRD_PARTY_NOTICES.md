# Third-party notices

SIM SMS PWA is derived from [xuanxuan1125/qmi-web](https://github.com/xuanxuan1125/qmi-web). Its original MIT notice, `Copyright (c) 2026 QMI Web Contributors`, is retained verbatim in [LICENSE](LICENSE). See [NOTICE.md](NOTICE.md) for provenance. We do not claim this codebase was written from scratch.

Each third-party component retains its own license. Our MIT license does not replace dependency licenses or their original copyright notices.

## Go dependencies

| Module | Version | License / full text location |
| --- | --- | --- |
| [qmi-go](https://github.com/iniwex5/qmi-go) | v0.6.4 | MIT — `vendor/github.com/iniwex5/qmi-go/LICENSE` |
| [sms](https://github.com/warthog618/sms) | v0.3.0 | MIT — `vendor/github.com/warthog618/sms/LICENSE` |
| [webpush-go](https://github.com/SherClockHolmes/webpush-go) | v1.4.0 | MIT — `vendor/github.com/SherClockHolmes/webpush-go/LICENSE` |
| [Go crypto](https://pkg.go.dev/golang.org/x/crypto), [term](https://pkg.go.dev/golang.org/x/term) | v0.54.0 / v0.45.0 | BSD-style Go Authors — respective `vendor/golang.org/x/*/LICENSE` |
| [yaml.v3](https://github.com/go-yaml/yaml) | v3.0.1 | MIT / Apache-2.0 notices — `vendor/gopkg.in/yaml.v3/LICENSE` |
| [modernc SQLite](https://pkg.go.dev/modernc.org/sqlite) | v1.56.0 | Project BSD-style notice and SQLite notice — `vendor/modernc.org/sqlite/LICENSE`, `SQLITE-LICENSE` |

Transitive versions and supplied notices remain in `vendor/modules.txt` and the corresponding vendor directories. Release archives copy those notice files to `third-party-licenses/`. No dependency-source patch is claimed; see [patch policy](docs/THIRD_PARTY_PATCHES.md).

## Direct frontend runtime dependencies

| Package | Version | Upstream | Full license text |
| --- | --- | --- | --- |
| Vue | 3.5.43 | https://github.com/vuejs/core | [MIT](licenses/npm/vue@3.5.43/LICENSE) |
| Vue Router | 5.2.0 | https://github.com/vuejs/router | [MIT](licenses/npm/vue-router@5.2.0/LICENSE) |
| Pinia | 4.0.2 | https://github.com/vuejs/pinia | [MIT](licenses/npm/pinia@4.0.2/LICENSE) |
| VueUse | 14.4.0 | https://github.com/vueuse/vueuse | [MIT](licenses/npm/@vueuse/core@14.4.0/LICENSE) |
| date-fns | 4.4.0 | https://github.com/date-fns/date-fns | [MIT](licenses/npm/date-fns@4.4.0/LICENSE.md) |
| Lucide Vue icons | 1.0.0 | https://github.com/lucide-icons/lucide | [ISC, including Feather-derived MIT notice](licenses/npm/lucide-vue-next@1.0.0/LICENSE) |

In particular, Lucide's notice names Lucide Contributors and retains Cole Bemis / Feather's notice for derived portions. It is not presented as an original icon library by this project.

## Transitives and development tools

[The inventory](licenses/npm-inventory.json) lists collected package versions, license metadata and notice paths. [licenses/npm/](licenses/npm) contains verbatim full notices, including Vue subpackages, Vite/Rolldown, Vitest, TypeScript, jsdom, Vue Test Utils and other installed tools/transitives. Platform-specific npm build binaries are not distributed in our release archives; applicable collected notices are retained even when a package is used only during development.

[web/package-lock.json](web/package-lock.json) remains the authoritative full locked dependency graph. The repository no longer claims to ship an npm offline cache. [Collection instructions](licenses/README.md) describe scope and the explicit Rolldown native-binding notice inheritance.

## Distribution

From 1.1.1, both Linux release archives include `LICENSE`, `NOTICE.md`, `THIRD_PARTY_NOTICES.md`, the npm notices under `licenses/`, and Go notices under `third-party-licenses/`. The 1.1.0 release receives a separate supplemental license archive; its original binary asset checksums are not silently changed.
