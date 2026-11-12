#!/usr/bin/env python3
"""Check the shareable source tree; report locations/categories, never contents."""
import argparse
import ipaddress
import re
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[1])
args = parser.parse_args()
root = args.root.resolve()
excluded = {'.git', 'node_modules', 'bin', 'dist', 'test-results', 'playwright-report'}
checks = [
    ('personal path', re.compile(r'/(?:Users|home)/[A-Za-z0-9_.-]+/')),
    ('private key material', re.compile(r'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----')),
    ('internal domain', re.compile(r'\b[\w.-]+\.(?:corp|internal|intranet)\b', re.I)),
    ('credential-bearing URL', re.compile(r'https?://[^\s/@:]+:[^\s/@]+@')),
]
failures = []
count = 0
for path in sorted(root.rglob('*')):
    relative = path.relative_to(root)
    if any(part in excluded for part in relative.parts) or not path.is_file():
        continue
    count += 1
    if path.is_symlink():
        failures.append((str(relative), 'symlink in distribution'))
        continue
    name = path.name.lower()
    if (name.startswith('.env') or 'password' in name or 'kubeconfig' in name
            or path.suffix.lower() in {'.pem', '.key', '.kubeconfig', '.pyc', '.log'}
            or '__pycache__' in relative.parts):
        failures.append((str(relative), 'credential or local artifact filename'))
        continue
    try:
        content = path.read_text(encoding='utf-8')
    except UnicodeDecodeError:
        if path.suffix.lower() not in {'.png', '.jpg', '.jpeg', '.gif', '.svg'}:
            failures.append((str(relative), 'unexpected binary artifact'))
        continue
    for category, pattern in checks:
        if pattern.search(content):
            failures.append((str(relative), category))
    # Publicly routed addresses are inappropriate in local fixture/source defaults.
    # Synthetic documentation ranges, loopback and private examples are permitted.
    for value in set(re.findall(r'(?<![\w.])(?:\d{1,3}\.){3}\d{1,3}(?![\w.])', content)):
        try:
            address = ipaddress.ip_address(value)
        except ValueError:
            continue
        if address.is_global:
            failures.append((str(relative), 'globally routed address'))
            break
if failures:
    for location, category in failures:
        print(f'FAIL: {location}: {category}')
    raise SystemExit(1)
print(f'PASS: checked {count} source/distribution files; no flagged content or local artifacts')
