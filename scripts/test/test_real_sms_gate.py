from __future__ import annotations

import os
import sqlite3
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import real_sms_gate as gate


class FakeResponse:
    status = 200

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        return False

    def read(self, _size=-1):
        return b'{"success":true}'


class RealSmsGateTests(unittest.TestCase):
    def setUp(self):
        self.env = patch.dict(
            os.environ,
            {
                "NEKOKO_SMS_API_KEY": "test-only-key",
                "NEKOKO_SMS_TARGET": "10000000000",
                "NEKOKO_SMS_FROM": "10000000000",
            },
            clear=False,
        )
        self.env.start()

    def tearDown(self):
        self.env.stop()

    def test_test_id_and_multipart_body_are_unique_and_identifiable(self):
        first = gate.make_test_id("soak-a")
        second = gate.make_test_id("soak-a")
        self.assertNotEqual(first, second)
        body = gate.build_message(first, "multipart")
        self.assertIn(first, body)
        self.assertGreater(len(body), 160)

    @patch("real_sms_gate.urllib.request.urlopen", return_value=FakeResponse())
    def test_api_success_never_prints_secret(self, urlopen):
        result = gate.send_real_test_sms("CANARY", rate_state=None)
        self.assertTrue(result.accepted)
        self.assertEqual(result.http_status, 200)
        request = urlopen.call_args.args[0]
        self.assertIn("test-only-key", request.full_url)
        self.assertIn("10000000000", request.full_url)

    def test_api_201_success_phrase_is_accepted(self):
        self.assertTrue(gate._response_is_success(b'{"code":201,"message":"SMS sent successfully"}'))
        self.assertFalse(gate._response_is_success(b'{"code":400,"message":"send failed"}'))

    def test_sqlite_matching_uses_test_id_not_count_delta(self):
        with tempfile.TemporaryDirectory() as directory:
            db = Path(directory) / "qmi-web.db"
            connection = sqlite3.connect(db)
            connection.execute(
                "CREATE TABLE sms_messages (id INTEGER PRIMARY KEY, encoding TEXT, body TEXT, received_at TEXT, created_at TEXT)"
            )
            connection.execute(
                "INSERT INTO sms_messages(encoding,body,received_at,created_at) VALUES(?,?,?,?)",
                ("GSM7", "old message", "2026-01-01T00:00:00Z", "2026-01-01T00:00:01Z"),
            )
            connection.execute(
                "INSERT INTO sms_messages(encoding,body,received_at,created_at) VALUES(?,?,?,?)",
                ("UCS2", "QMIWEB-CANARY-UNIQUE", "2026-01-01T00:01:00Z", "2026-01-01T00:01:01Z"),
            )
            connection.commit()
            connection.close()
            found = gate._read_sms(db, "QMIWEB-CANARY-UNIQUE")
            self.assertIsNotNone(found)
            self.assertEqual(found.encoding, "UCS2")
            self.assertIsNone(gate._read_sms(db, "QMIWEB-CANARY-MISSING"))


if __name__ == "__main__":
    unittest.main()
