# Dependency inventory

The authoritative version records are `go.mod`, `go.sum`, `vendor/modules.txt`, `web/package.json` and `web/package-lock.json`.

Full upstream/project attribution and dependency links are maintained in [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md). Verbatim npm license texts and copyright notices are in [licenses/npm/](../licenses/npm); the collected machine-readable inventory is [npm-inventory.json](../licenses/npm-inventory.json). Go notices remain inside `vendor/` and are copied into binary release archives.

The public repository does not ship an npm offline cache. After a reviewed dependency change, install the exact lockfile and run `python3 scripts/collect-npm-licenses.py`, then review newly introduced licenses/notices. Do not drop copyright notices or relabel dependencies as our original work.

See [THIRD_PARTY_PATCHES.md](THIRD_PARTY_PATCHES.md) before modifying a dependency's source. No dependency source was changed by the license-notice correction.
