import importlib.util
import json
import pathlib
import subprocess
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('health', pathlib.Path(__file__).with_name('sms-healthcheck.py'))
health = importlib.util.module_from_spec(spec)
spec.loader.exec_module(health)


class RecoveryTest(unittest.TestCase):
    def test_failure_threshold_timeout_cooldown_and_dependency(self):
        with tempfile.TemporaryDirectory() as d, patch.object(health, 'healthy', return_value=False), patch.object(health.subprocess, 'run', side_effect=subprocess.TimeoutExpired('systemctl', 10)) as restart:
            directory = pathlib.Path(d)
            for _ in range(2):
                health.check(directory)
            self.assertEqual(restart.call_count, 0)
            health.check(directory)
            self.assertEqual(restart.call_count, 1)
            self.assertEqual(restart.call_args.args[0][-1], 'qmi-web')
            for _ in range(3):
                health.check(directory)
            self.assertEqual(restart.call_count, 1)
            self.assertGreater(json.loads((directory/'qmi-web.json').read_text())['last_restart'], 0)

    def test_corrupt_state_recovers_and_success_resets_failures(self):
        with tempfile.TemporaryDirectory() as d, patch.object(health, 'healthy', return_value=True):
            directory = pathlib.Path(d)
            (directory/'qmi-web.json').write_text('null')
            (directory/'apache2.json').write_text('{"failures": 2, "last_restart": 0}')
            health.check(directory)
            self.assertEqual(health.load_state(directory/'qmi-web.json')['failures'], 0)
            self.assertEqual(health.load_state(directory/'apache2.json')['failures'], 0)


if __name__ == '__main__':
    unittest.main()
