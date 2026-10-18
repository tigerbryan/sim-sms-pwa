#!/usr/bin/env python3
"""Opt-in real-SMS gate runner for explicit hardware validation.

The API secret, target, and sender are read only from the current process
environment.  They are never accepted as command-line arguments or printed.
The runner deliberately separates external API acceptance from modem/QMI and
SQLite observation.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import secrets
import sqlite3
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

API_ENDPOINT = "https://api.nekoko.tel/sms/send"
DEFAULT_TIMEOUT_SECONDS = 20
DEFAULT_POLL_SECONDS = 5
DEFAULT_WAIT_SECONDS = 600
DEFAULT_MIN_INTERVAL_SECONDS = 300
RETRY_DELAYS_SECONDS = (0, 10, 30)
POSITIVE_RESPONSE_WORDS = re.compile(r"\b(ok|success|successful|sent|accepted|queued)\b", re.I)
NEGATIVE_RESPONSE_WORDS = re.compile(r"\b(error|failed|failure|invalid|denied|false)\b", re.I)


@dataclass(frozen=True)
class SendResult:
    test_id: str
    body: str
    encoding: str
    request_time: str
    accepted_at: str | None
    http_status: int | None
    attempts: int
    accepted: bool


@dataclass(frozen=True)
class SmsObservation:
    encoding: str
    body_length: int
    received_at: str
    sqlite_saved_at: str


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


def iso_now() -> str:
    return utc_now().isoformat(timespec="seconds").replace("+00:00", "Z")


def make_test_id(stage: str, when: datetime | None = None) -> str:
    clean = re.sub(r"[^A-Za-z0-9]+", "-", stage.strip().upper()).strip("-")
    if not clean:
        raise ValueError("stage must contain at least one ASCII letter or digit")
    stamp = (when or utc_now()).strftime("%Y%m%dT%H%M%S")
    return f"QMIWEB-{clean}-{stamp}-{secrets.token_hex(2).upper()}"


def build_message(test_id: str, encoding: str) -> str:
    if encoding == "ascii":
        return test_id
    if encoding == "ucs2":
        return f"QMI Web 中文短信测试 {test_id}"
    if encoding == "multipart":
        # The test ID is included in the first segment and the body is long
        # enough to exercise multipart reassembly without becoming a flood.
        filler = "中文短信分段验证。"
        return f"QMI Web multipart {test_id} " + filler * 90
    raise ValueError(f"unsupported encoding: {encoding}")


def _required_env(name: str) -> str:
    value = os.environ.get(name, "").strip()
    if not value:
        raise RuntimeError(f"missing required runtime secret/configuration: {name}")
    return value


def _response_is_success(payload: bytes) -> bool:
    # Keep the response in memory only and never echo it.  Nekoko deployments
    # have returned both JSON and short text responses, so accept only an
    # explicit positive result and reject explicit failure markers.
    text = payload[:8192].decode("utf-8", errors="replace").strip()
    if not text:
        return False
    try:
        value: Any = json.loads(text)
    except json.JSONDecodeError:
        value = None

    def walk(item: Any) -> bool:
        if isinstance(item, bool):
            return item
        if isinstance(item, (int, float)) and not isinstance(item, bool):
            return 200 <= item < 300
        if isinstance(item, str):
            lowered = item.strip().lower()
            if lowered.isdigit():
                return 200 <= int(lowered) < 300
            if lowered in {"ok", "success", "successful", "sent", "accepted", "queued", "true"}:
                return True
            return bool(POSITIVE_RESPONSE_WORDS.search(lowered) and not NEGATIVE_RESPONSE_WORDS.search(lowered))
        if isinstance(item, dict):
            for key in ("code", "status_code", "http_status"):
                if key in item and walk(item[key]):
                    return True
            for key in ("success", "ok", "sent", "accepted", "queued"):
                if key in item and walk(item[key]):
                    return True
            for key in ("status", "result", "message", "msg"):
                if key in item and walk(item[key]):
                    return True
        if isinstance(item, list):
            return any(walk(child) for child in item)
        return False

    if value is not None:
        return walk(value)
    if NEGATIVE_RESPONSE_WORDS.search(text):
        return False
    return bool(POSITIVE_RESPONSE_WORDS.search(text))


def _target_fingerprint(target: str) -> str:
    return hashlib.sha256(target.encode("utf-8")).hexdigest()[:16]


def _check_rate_state(path: Path, target: str, minimum_seconds: int) -> int:
    if minimum_seconds <= 0 or not path.exists():
        return 0
    try:
        state = json.loads(path.read_text(encoding="utf-8"))
        if state.get("target_fingerprint") != _target_fingerprint(target):
            return 0
        last_success = float(state.get("last_success_epoch", 0))
    except (OSError, ValueError, TypeError, json.JSONDecodeError):
        return 0
    remaining = int(minimum_seconds - (time.time() - last_success))
    return max(0, remaining)


def _record_rate_success(path: Path, target: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(f".{path.name}.{os.getpid()}.tmp")
    payload = {
        "target_fingerprint": _target_fingerprint(target),
        "last_success_epoch": time.time(),
    }
    temporary.write_text(json.dumps(payload, separators=(",", ":")), encoding="utf-8")
    os.replace(temporary, path)
    try:
        os.chmod(path, 0o600)
    except OSError:
        pass


def _ethernet_default_route() -> bool:
    """Return true only when an IPv4 default route is not cellular-facing."""

    try:
        completed = subprocess.run(
            ["ip", "-4", "route", "show", "default"],
            check=False,
            capture_output=True,
            text=True,
            timeout=5,
        )
    except (OSError, subprocess.SubprocessError):
        return False
    if completed.returncode != 0 or not completed.stdout.strip():
        return False
    cellular = re.compile(r"\b(?:wwan|rmnet|usb\d+|ccmni|pdp\d+|cdc\w*)\w*\b", re.I)
    return not any(cellular.search(line) for line in completed.stdout.splitlines())


def send_real_test_sms(
    stage: str,
    *,
    encoding: str = "ascii",
    timeout_seconds: int = DEFAULT_TIMEOUT_SECONDS,
    max_attempts: int = len(RETRY_DELAYS_SECONDS),
    retry_delays: tuple[int, ...] = RETRY_DELAYS_SECONDS,
    rate_state: Path | None = None,
    minimum_interval_seconds: int = DEFAULT_MIN_INTERVAL_SECONDS,
) -> SendResult:
    """Send one uniquely identified SMS using only process-local env vars."""

    api_key = _required_env("NEKOKO_SMS_API_KEY")
    target = _required_env("NEKOKO_SMS_TARGET")
    sender = _required_env("NEKOKO_SMS_FROM")
    test_id = make_test_id(stage)
    body = build_message(test_id, encoding)
    if rate_state is not None:
        remaining = _check_rate_state(rate_state, target, minimum_interval_seconds)
        if remaining:
            raise RuntimeError(f"SMS_RATE_LIMIT_WAIT_SECONDS={remaining}")

    request_time = iso_now()
    last_status: int | None = None
    for attempt in range(min(max_attempts, len(retry_delays))):
        if attempt:
            time.sleep(retry_delays[attempt])
        query = urllib.parse.urlencode({"apikey": api_key, "from": sender, "body": body})
        endpoint = f"{API_ENDPOINT}/{urllib.parse.quote(target, safe='')}?{query}"
        request = urllib.request.Request(endpoint, method="GET", headers={"Accept": "application/json, text/plain"})
        try:
            with urllib.request.urlopen(request, timeout=timeout_seconds) as response:
                last_status = int(response.status)
                response_body = response.read(8192)
            accepted = 200 <= last_status < 300 and _response_is_success(response_body)
        except urllib.error.HTTPError as error:
            last_status = int(error.code)
            try:
                error.read(8192)
            except OSError:
                pass
            accepted = False
        except (urllib.error.URLError, TimeoutError, OSError):
            accepted = False
        if accepted:
            accepted_at = iso_now()
            if rate_state is not None:
                _record_rate_success(rate_state, target)
            return SendResult(test_id, body, encoding, request_time, accepted_at, last_status, attempt + 1, True)

    return SendResult(test_id, body, encoding, request_time, None, last_status, min(max_attempts, len(retry_delays)), False)


def _parse_time(value: str | None) -> datetime | None:
    if not value:
        return None
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
    return parsed.replace(tzinfo=timezone.utc) if parsed.tzinfo is None else parsed.astimezone(timezone.utc)


def _read_sms(db_path: Path, test_id: str) -> SmsObservation | None:
    uri = f"file:{urllib.parse.quote(str(db_path), safe='/')}?mode=ro"
    connection: sqlite3.Connection | None = None
    try:
        connection = sqlite3.connect(uri, uri=True, timeout=2)
        connection.execute("PRAGMA query_only=ON")
        row = connection.execute(
            "SELECT encoding, body, received_at, created_at "
            "FROM sms_messages WHERE instr(body, ?) > 0 "
            "ORDER BY id DESC LIMIT 1",
            (test_id,),
        ).fetchone()
    except (OSError, sqlite3.Error):
        return None
    finally:
        if connection is not None:
            connection.close()
    if not row:
        return None
    return SmsObservation(str(row[0]), len(str(row[1])), str(row[2]), str(row[3]))


def _status_results(status_path: Path) -> tuple[str, str, str]:
    try:
        status = json.loads(status_path.read_text(encoding="utf-8"))
    except (OSError, ValueError, json.JSONDecodeError):
        return "FAIL", "FAIL", "FAIL"
    wms_keys = ("wms_list", "wms_subscribe", "wms_set_event_report", "wms_indication_register")
    wms = "PASS" if all(status.get(key) == "pass" for key in wms_keys) else "FAIL"
    decode = "PASS" if status.get("read_message") == "pass" and status.get("decoder") == "pass" else "FAIL"
    sqlite_result = "PASS" if status.get("sqlite") == "pass" else "FAIL"
    return wms, decode, sqlite_result


def _latency_seconds(start: str | None, end: str | None) -> str:
    first, second = _parse_time(start), _parse_time(end)
    if first is None or second is None:
        return "N/A"
    return f"{max(0.0, (second - first).total_seconds()):.3f}"


def run_gate(
    stage: str,
    *,
    db_path: Path,
    status_path: Path,
    encoding: str = "ascii",
    wait_seconds: int = DEFAULT_WAIT_SECONDS,
    poll_seconds: int = DEFAULT_POLL_SECONDS,
    rate_state: Path | None = None,
    minimum_interval_seconds: int = DEFAULT_MIN_INTERVAL_SECONDS,
) -> int:
    if not _ethernet_default_route():
        print("DEFAULT_ROUTE_BEFORE=FAIL\nEXTERNAL_API_UNAVAILABLE\nSMS_TEST_BLOCKED_BY_EXTERNAL_API")
        return 20
    print("DEFAULT_ROUTE_BEFORE=PASS")
    try:
        result = send_real_test_sms(stage, encoding=encoding, rate_state=rate_state, minimum_interval_seconds=minimum_interval_seconds)
    except RuntimeError as error:
        message = str(error)
        if message.startswith("SMS_RATE_LIMIT_WAIT_SECONDS="):
            print("SMS_API_REQUEST=NOT_SENT\nSMS_API_SEND=NOT_SENT\nSMS_RATE_LIMIT_BLOCKED")
        else:
            print("SMS_API_REQUEST=FAIL\nSMS_API_SEND=FAIL\nHTTP_STATUS=N/A\nTEST_INFRA_SMS_API_CONFIGURATION\nSMS_TEST_BLOCKED_BY_EXTERNAL_API")
        return 20

    print(f"SMS_TEST_ID={result.test_id}")
    print(f"SMS_API_RESULT={'PASS' if result.accepted else 'FAIL'}")
    print(f"SMS_API_REQUEST={'PASS' if result.accepted else 'FAIL'}")
    print(f"HTTP_STATUS={result.http_status if result.http_status is not None else 'N/A'}")
    print(f"SEND_TIME={result.request_time}")
    print(f"SMS_API_ACCEPTED_AT={result.accepted_at or 'N/A'}")
    print(f"SMS_API_ATTEMPTS={result.attempts}")
    print(f"ENCODING={result.encoding}")
    print(f"LENGTH={len(result.body)}")
    print("API_KEY_REDACTED=yes")
    print("TARGET_NUMBER_REDACTED=yes")
    if not result.accepted:
        print("TEST_INFRA_SMS_API_FAILURE")
        print("SMS_TEST_BLOCKED_BY_EXTERNAL_API")
        return 20
    if not _ethernet_default_route():
        print("DEFAULT_ROUTE_AFTER=FAIL\nEXTERNAL_API_UNAVAILABLE\nSMS_TEST_BLOCKED_BY_EXTERNAL_API")
        return 20
    print("DEFAULT_ROUTE_AFTER=PASS")

    observed: SmsObservation | None = None
    deadline = time.monotonic() + max(0, wait_seconds)
    while time.monotonic() <= deadline:
        observed = _read_sms(db_path, result.test_id)
        if observed is not None:
            break
        time.sleep(max(1, poll_seconds))
    if observed is None:
        print("REAL_SMS_RX_FAIL")
        print("SMS_MODEM_OBSERVED_AT=N/A\nSMS_SQLITE_SAVED_AT=N/A\nTOTAL_LATENCY=N/A")
        print("WMS_RESULT=FAIL\nDECODE_RESULT=FAIL\nSQLITE_RESULT=FAIL")
        return 30

    wms, decode, sqlite_result = _status_results(status_path)
    print(f"SMS_MODEM_OBSERVED_AT={observed.received_at}")
    print(f"SMS_SQLITE_SAVED_AT={observed.sqlite_saved_at}")
    print(f"MODEM_RECEIVE_TIME={observed.received_at}")
    print(f"SQLITE_TIME={observed.sqlite_saved_at}")
    print(f"API_TO_MODEM_LATENCY={_latency_seconds(result.accepted_at, observed.received_at)}")
    print(f"MODEM_TO_SQLITE_LATENCY={_latency_seconds(observed.received_at, observed.sqlite_saved_at)}")
    print(f"TOTAL_LATENCY={_latency_seconds(result.accepted_at, observed.sqlite_saved_at)}")
    print(f"WMS_RESULT={wms}")
    print(f"DECODE_RESULT={decode}")
    print(f"SQLITE_RESULT={sqlite_result}")
    if wms == decode == sqlite_result == "PASS":
        print("REAL_SMS_PASS")
        return 0
    print("APPLICATION_FAILURE")
    return 31


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Opt-in real SMS API to modem/SQLite validation gate")
    parser.add_argument("--stage", required=True, help="CANARY, PROD, RESTART, SOAK-A, UCS2, or MULTIPART")
    parser.add_argument("--db", required=True, type=Path, help="QMI Web SQLite path")
    parser.add_argument("--status-file", type=Path, help="QMI Web app-status.json path")
    parser.add_argument("--encoding", choices=("ascii", "ucs2", "multipart"), default="ascii")
    parser.add_argument("--wait-seconds", type=int, default=DEFAULT_WAIT_SECONDS)
    parser.add_argument("--poll-seconds", type=int, default=DEFAULT_POLL_SECONDS)
    parser.add_argument("--rate-state", type=Path, help="non-secret local rate state file")
    parser.add_argument("--min-interval-seconds", type=int, default=DEFAULT_MIN_INTERVAL_SECONDS)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = _parser().parse_args(argv)
    status_path = args.status_file or args.db.with_name("app-status.json")
    try:
        return run_gate(
            args.stage,
            db_path=args.db,
            status_path=status_path,
            encoding=args.encoding,
            wait_seconds=args.wait_seconds,
            poll_seconds=args.poll_seconds,
            rate_state=args.rate_state,
            minimum_interval_seconds=args.min_interval_seconds,
        )
    finally:
        # This only clears the child process environment; the parent shell is
        # never modified and no secret is retained in a report or state file.
        for name in ("NEKOKO_SMS_API_KEY", "NEKOKO_SMS_TARGET", "NEKOKO_SMS_FROM"):
            os.environ.pop(name, None)


if __name__ == "__main__":
    raise SystemExit(main())
