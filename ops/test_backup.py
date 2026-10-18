import importlib.util
import pathlib
import sqlite3
import tarfile
import tempfile
import unittest

spec=importlib.util.spec_from_file_location('backup', pathlib.Path(__file__).with_name('sms-backup.py'))
backup=importlib.util.module_from_spec(spec)
spec.loader.exec_module(backup)


class BackupTest(unittest.TestCase):
    def test_wal_snapshot_contains_latest_commit_and_private_restore_files(self):
        with tempfile.TemporaryDirectory() as d:
            root=pathlib.Path(d)
            source=root/'live.db'
            with sqlite3.connect(source) as writer:
                writer.execute('PRAGMA journal_mode=WAL')
                writer.execute('CREATE TABLE sample (id INTEGER)')
                writer.execute('INSERT INTO sample VALUES (7)')
                writer.commit()
                config=root/'config.yaml'; config.write_text('fixture-config')
                key=root/'master.key'; key.write_bytes(b'fixture-only')
                destination=root/'backups'
                for _ in range(8):
                    archive=backup.create_backup(source,destination,config,key)
                self.assertEqual(len(list(destination.glob('sms-*.tar.gz'))),7)
                self.assertEqual(archive.stat().st_mode & 0o777,0o600)
                with tarfile.open(archive) as bundle:
                    self.assertEqual(set(bundle.getnames()),{'qmi-web.db','config.yaml','secrets/master.key'})
                    restored=root/'restored.db'
                    restored.write_bytes(bundle.extractfile('qmi-web.db').read())
                    self.assertEqual(bundle.extractfile('secrets/master.key').read(),b'fixture-only')
                with sqlite3.connect(restored) as db:
                    self.assertEqual(db.execute('SELECT id FROM sample').fetchone()[0],7)


if __name__=='__main__': unittest.main()
