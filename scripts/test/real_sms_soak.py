#!/usr/bin/env python3
"""Schedule the four low-rate real-SMS checks used by a 60-minute soak."""

from __future__ import annotations

import argparse
import os
import time
from pathlib import Path

from real_sms_gate import run_gate


SCHEDULE = (("SOAK-A", 300), ("SOAK-B", 1200), ("SOAK-C", 2100), ("SOAK-D", 3000))


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Run four rate-safe SMS gates during a 60-minute soak")
    parser.add_argument("--db", required=True, type=Path)
    parser.add_argument("--status-file", type=Path)
    parser.add_argument("--rate-state", type=Path)
    parser.add_argument("--wait-seconds", type=int, default=600)
    parser.add_argument("--poll-seconds", type=int, default=5)
    args = parser.parse_args(argv)
    status_path = args.status_file or args.db.with_name("app-status.json")
    started = time.monotonic()
    try:
        for stage, offset in SCHEDULE:
            remaining = offset - (time.monotonic() - started)
            if remaining > 0:
                time.sleep(remaining)
            print(f"SOAK_GATE_START={stage}", flush=True)
            rc = run_gate(
                stage,
                db_path=args.db,
                status_path=status_path,
                wait_seconds=args.wait_seconds,
                poll_seconds=args.poll_seconds,
                rate_state=args.rate_state,
            )
            if rc == 20:
                print("SMS_TEST_BLOCKED_BY_EXTERNAL_API", flush=True)
                return 20
            if rc != 0:
                print("REAL_SMS_RX_FAIL", flush=True)
                return rc
            print(f"SOAK_GATE_RESULT={stage}:PASS", flush=True)
        print("REAL_SMS_SOAK_SMS_GATES=PASS", flush=True)
        return 0
    finally:
        for name in ("NEKOKO_SMS_API_KEY", "NEKOKO_SMS_TARGET", "NEKOKO_SMS_FROM"):
            os.environ.pop(name, None)


if __name__ == "__main__":
    raise SystemExit(main())
