#!/usr/bin/env python3
"""Audit publishable files without printing matched credentials or SMS text."""
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
SKIP = {'.git', 'node_modules', '__pycache__', '.private', 'build', 'output', 'logs', 'data', 'backups'}
TEXT_SUFFIXES = {'.go', '.md', '.ts', '.vue', '.js', '.mjs', '.json', '.yaml', '.yml', '.toml', '.sh', '.py', '.txt', '.example'}
RULES = {
    'private-key': re.compile(r'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----'),
    'github-token': re.compile(r'\b(?:gh[pousr]_[A-Za-z0-9_]{30,}|github_pat_[A-Za-z0-9_]{30,})\b'),
    'aws-access-key': re.compile(r'\bAKIA[0-9A-Z]{16}\b'),
    'slack-token': re.compile(r'\bxox[baprs]-[A-Za-z0-9-]{20,}\b'),
    'absolute-user-home': re.compile(r'/(?:Users|home)/[A-Za-z0-9_.-]+/|[A-Z]:\\Users\\[^\\]+\\'),
}
# Exact public third-party test fixtures; no deployment keys are permitted.
FIXTURE_PREFIX = 'vendor/github.com/golang-jwt/jwt/v5/test/'


def files():
    try:
        result = subprocess.run(['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], cwd=ROOT, check=True, capture_output=True)
        return sorted({ROOT / p.decode() for p in result.stdout.split(b'\0') if p})
    except (OSError, subprocess.CalledProcessError):
        return sorted(p for p in ROOT.rglob('*') if p.is_file() and not any(x in SKIP for x in p.relative_to(ROOT).parts) and 'internal/web/dist' not in str(p))


def check():
    problems = []
    inspected = 0
    for path in files():
        rel = path.relative_to(ROOT).as_posix()
        if path.is_symlink():
            problems.append((rel, 'symlink-not-permitted')); continue
        if not path.is_file():
            continue
        inspected += 1
        parts = pathlib.PurePosixPath(rel).parts
        if any(x in {'data', 'logs', 'backups', '.private', 'node_modules'} for x in parts) or path.suffix in {'.db', '.sqlite', '.sqlite3', '.log', '.jsonl', '.p12', '.pfx'} or path.name in {'master.key', 'setup-token', '.env', 'config.yaml'}:
            problems.append((rel, 'runtime-or-secret-file'))
        if path.suffix not in TEXT_SUFFIXES and path.name not in {'LICENSE', 'Makefile'}:
            continue
        try:
            source = path.read_text()
        except (UnicodeError, OSError):
            continue
        for rule, pattern in RULES.items():
            upstream_generated_path = rule == 'absolute-user-home' and rel.startswith('vendor/modernc.org/sqlite/lib/')
            if pattern.search(source) and not upstream_generated_path and not (rule == 'private-key' and rel.startswith(FIXTURE_PREFIX)):
                problems.append((rel, rule))
        if path.suffix == '.md' and not rel.startswith('vendor/'):
            for target in re.findall(r'\]\(([^)]+)\)', source):
                target = target.split()[0].split('#')[0]
                if not target or re.match(r'[a-zA-Z][a-zA-Z0-9+.-]*:', target):
                    continue
                if not (path.parent / target).exists():
                    problems.append((rel, 'broken-local-link:' + target))
    for rel, rule in problems:
        print(f'FAIL {rel}: {rule}', file=sys.stderr)
    print(f'Public-tree check: {inspected} files, {len(problems)} findings; matched values were not printed.')
    return bool(problems)


if __name__ == '__main__':
    sys.exit(check())
