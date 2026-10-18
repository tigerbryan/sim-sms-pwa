#!/usr/bin/python3
"""Create a root-only, consistent SMS snapshot and retain the last seven copies."""
import datetime
import os
import pathlib
import shutil
import sqlite3
import tarfile
import tempfile

SOURCE = pathlib.Path('/var/lib/qmi-web/qmi-web.db')
DEST = pathlib.Path('/var/backups/qmi-web')


def create_backup(source=SOURCE, destination=DEST, config=pathlib.Path('/etc/qmi-web/config.yaml'), key=pathlib.Path('/var/lib/qmi-web/secrets/master.key')):
    os.umask(0o077)
    destination.mkdir(parents=True, mode=0o700, exist_ok=True)
    if destination.is_symlink():
        raise RuntimeError('backup directory cannot be a symlink')
    destination.chmod(0o700)
    if shutil.disk_usage(destination).free < max(source.stat().st_size * 3, 100 * 1024 * 1024):
        raise RuntimeError('not enough free space for backup')
    stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
    target = destination / ('sms-' + stamp + '.tar.gz')
    with tempfile.TemporaryDirectory(prefix='.snapshot-', dir=destination) as directory:
        temporary = pathlib.Path(directory)
        snapshot = temporary / 'qmi-web.db'
        with sqlite3.connect(source.as_uri() + '?mode=ro', uri=True, timeout=5) as src:
            with sqlite3.connect(snapshot) as dst:
                src.backup(dst, pages=256, sleep=0.1)
                if dst.execute('PRAGMA integrity_check').fetchone()[0] != 'ok':
                    raise RuntimeError('snapshot failed integrity check')
        archive = temporary / 'snapshot.tar.gz'
        with tarfile.open(archive, 'w:gz') as bundle:
            for path, name in [(snapshot, 'qmi-web.db'), (config, 'config.yaml'), (key, 'secrets/master.key')]:
                bundle.add(path, arcname=name, recursive=False)
        archive.chmod(0o600)
        archive.replace(target)
    for old in sorted(destination.glob('sms-*.tar.gz'), reverse=True)[7:]:
        old.unlink()
    print('SMS backup completed; SQLite integrity verified; retention=7', flush=True)
    return target


if __name__ == '__main__':
    create_backup()
