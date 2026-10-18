#!/usr/bin/env python3
"""Package a reviewed Linux binary with instructions, installer and licenses."""
import argparse
import hashlib
import pathlib
import shutil
import tarfile
import tempfile

root = pathlib.Path(__file__).resolve().parent.parent
parser = argparse.ArgumentParser()
parser.add_argument('--arch', choices=['amd64', 'arm64'], required=True)
parser.add_argument('--output', type=pathlib.Path, required=True)
args = parser.parse_args()
version = (root / 'VERSION').read_text().strip()
binary = root / 'build' / f'qmi-web-linux-{args.arch}'
raw = binary.read_bytes()
expected_machine = 62 if args.arch == 'amd64' else 183
if raw[:4] != b'\x7fELF' or int.from_bytes(raw[18:20], 'little') != expected_machine:
    raise SystemExit('Input binary is not the requested Linux ELF architecture')
name = f'sim-sms-pwa-{version}-linux-{args.arch}'
args.output.mkdir(parents=True, exist_ok=True)
with tempfile.TemporaryDirectory(prefix='sim-sms-package-') as tmp:
    target = pathlib.Path(tmp) / name
    (target / 'build').mkdir(parents=True)
    shutil.copy2(binary, target / 'build/qmi-web')
    for name_file in ['LICENSE', 'NOTICE.md', 'THIRD_PARTY_NOTICES.md', 'README.md', 'README.en.md', 'AGENT_PROMPT.md', 'SECURITY.md', 'CONTRIBUTING.md', 'AGENTS.md', 'VERSION', 'install.sh']:
        shutil.copy2(root / name_file, target / name_file)
    for directory in ['docs', 'config']:
        shutil.copytree(root / directory, target / directory)
    (target / 'scripts').mkdir()
    for name_file in ['install-native.sh', 'install.sh', 'update.sh', 'uninstall.sh']:
        shutil.copy2(root / 'scripts' / name_file, target / 'scripts' / name_file)
    shutil.copytree(root / 'ops', target / 'ops', ignore=shutil.ignore_patterns('__pycache__', '*.pyc'))
    for path in (root / 'vendor').rglob('*'):
        if path.is_file() and any(key in path.name.upper() for key in ['LICENSE', 'NOTICE', 'COPYING', 'COPYRIGHT']):
            dest = target / 'third-party-licenses' / path.relative_to(root / 'vendor')
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(path, dest)
    archive = args.output / (name + '.tar.gz')
    def public_metadata(info):
        info.uid = info.gid = 0
        info.uname = info.gname = ''
        return info
    with tarfile.open(archive, 'w:gz') as tar:
        tar.add(target, arcname=name, filter=public_metadata)
    print(archive.name, hashlib.sha256(archive.read_bytes()).hexdigest())
