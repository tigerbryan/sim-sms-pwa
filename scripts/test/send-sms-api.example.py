#!/usr/bin/env python3
"""Public entry point for the opt-in real-SMS gate.

The real values are supplied only at execution time through
NEKOKO_SMS_API_KEY, NEKOKO_SMS_TARGET, and NEKOKO_SMS_FROM.  Never replace the
environment-variable names with real values in this file.
"""

from real_sms_gate import main


if __name__ == "__main__":
    raise SystemExit(main())
