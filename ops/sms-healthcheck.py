#!/usr/bin/python3
"""Check local SMS app/database and HTTPS proxy; recover after sustained failures."""
import json
import pathlib
import ssl
import subprocess
import time
import urllib.request

STATE = pathlib.Path('/run/sms-healthcheck')


def load_state(path):
    try:
        state = json.loads(path.read_text())
        if not isinstance(state, dict):
            raise ValueError('invalid state')
        return {'failures': max(0, int(state['failures'])),
                'last_restart': max(0, float(state['last_restart']))}
    except (OSError, ValueError, TypeError, KeyError):
        return {'failures': 0, 'last_restart': 0}


def save_state(path, state):
    temporary = path.with_suffix('.tmp')
    temporary.write_text(json.dumps(state))
    temporary.replace(path)


def healthy(url):
    try:
        # Loopback proxy uses the existing self-signed LAN certificate.
        context = ssl._create_unverified_context() if url.startswith('https:') else None
        with urllib.request.urlopen(url, timeout=5, context=context) as response:
            return response.status == 200 and json.load(response).get('ready') is True
    except (OSError, ValueError, AttributeError):
        return False


def check(state_dir=STATE):
    state_dir.mkdir(mode=0o750, exist_ok=True)
    app_healthy = False
    for service, url in [('qmi-web', 'http://127.0.0.1:7580/ready'), ('apache2', 'https://127.0.0.1/ready')]:
        statefile = state_dir / (service + '.json')
        state = load_state(statefile)
        ok = healthy(url)
        state['failures'] = 0 if ok else state['failures'] + 1
        now = time.time()
        eligible = service == 'qmi-web' or app_healthy
        if eligible and state['failures'] >= 3 and now - state['last_restart'] >= 600:
            # Persist cooldown before restarting: a blocked service manager must
            # not cause another restart attempt every minute.
            state.update(last_restart=now, failures=0)
            save_state(statefile, state)
            print(f'{service}: repeated health check failure; scheduling recovery', flush=True)
            try:
                result = subprocess.run(['systemctl', '--no-block', 'restart', service], timeout=10, check=False)
                print(f'{service}: recovery command exit={result.returncode}', flush=True)
            except (OSError, subprocess.TimeoutExpired):
                print(f'{service}: recovery command unavailable; cooldown retained', flush=True)
        save_state(statefile, state)
        if service == 'qmi-web':
            app_healthy = ok


if __name__ == '__main__':
    check()
