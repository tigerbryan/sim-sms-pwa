# Dependency license texts

- `npm/`: verbatim license/copyright/third-party notices from installed, lockfile-resolved frontend and build packages. These texts retain their upstream author names and notices; they are not relicensed as our own work.
- `npm-inventory.json`: package/version/license and relative notice paths. It records the platform-specific optional packages installed when collecting; it does not claim to bundle native npm tool binaries in the Go application.
- The Rolldown native binding refers to the same upstream Rolldown repository and does not ship a local license file. Its directory includes the parent project's LICENSE and THIRD-PARTY-LICENSE, explicitly marked in the inventory.
- Go dependency notices remain under `vendor/`; release packages include them under `third-party-licenses/`.

To refresh after a deliberate dependency update: `cd web && npm ci`, then `python3 scripts/collect-npm-licenses.py`. Review the resulting changes and retain all upstream notices. Installing another OS's optional build package is not needed just to copy its binary into a release: npm native tool binaries are not distributed by our Go release.
