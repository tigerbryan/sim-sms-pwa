#!/usr/bin/env python3
"""Collect installed, lockfile-resolved npm notices without local machine paths."""
import json
import pathlib
import re
import shutil

root = pathlib.Path(__file__).resolve().parent.parent
lock = json.loads((root / 'web/package-lock.json').read_text())
output = root / 'licenses/npm'
output.mkdir(parents=True, exist_ok=True)
entries = []
for key, metadata in sorted(lock['packages'].items()):
    if not key:
        continue
    package = root / 'web' / key
    if not (package / 'package.json').is_file():
        continue  # Uninstalled optional-platform binaries are not distributed.
    data = json.loads((package / 'package.json').read_text())
    name, version = data['name'], data['version']
    destination = output / (name + '@' + version)
    destination.mkdir(parents=True, exist_ok=True)
    notices = []
    sources = [package]
    inherited = False
    if name.startswith('@rolldown/binding-'):
        # Native binding metadata points to packages/rolldown in the same repo;
        # the binding package omits a local license file. Preserve its parent
        # project's own and third-party notices rather than inventing a notice.
        sources.append(root / 'web/node_modules/rolldown')
        inherited = True
    for source in sources:
        for path in sorted(source.iterdir()):
            if path.is_file() and re.search(r'licen[cs]e|notice|copying|copyright', path.name, re.I):
                shutil.copy2(path, destination / path.name)
                notices.append((destination / path.name).relative_to(root).as_posix())
    entries.append({'name': name, 'version': version, 'license': data.get('license', metadata.get('license', 'UNSPECIFIED')),
                    'development_only': bool(metadata.get('dev')), 'notices': sorted(set(notices)),
                    'inherited_project_notice': inherited})
(root / 'licenses/npm-inventory.json').write_text(json.dumps(entries, ensure_ascii=False, indent=2) + '\n')
direct = json.loads((root / 'web/package.json').read_text())['dependencies']
for name in direct:
    assert any(x['name'] == name and x['notices'] for x in entries), 'Missing direct dependency license: ' + name
print(f'Preserved {sum(len(x["notices"]) for x in entries)} notice files across {len(entries)} installed packages.')
