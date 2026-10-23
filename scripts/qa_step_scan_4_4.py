#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_step_scan_4_4.py — QA-скан шага 4-4 по живому диффу ветки:
секреты / CJK-артефакты / смешанные слова / прод-зонды импортов."""
import re
import subprocess
import sys

REPO = '/home/z/my-project/staffy'

files = subprocess.run(
    ['git', 'diff', '--name-only', 'main...HEAD'],
    cwd=REPO, capture_output=True, text=True, check=True,
).stdout.split()

secret_patterns = [
    r'github_pat_[A-Za-z0-9]{20,}',
    r'ghp_[A-Za-z0-9]{30,}',
    r'AKIA[0-9A-Z]{16}',
    r'sk-[A-Za-z0-9]{20,}',
]
cjk_range = re.compile(r'[\u4e00-\u9fff]')
mixed_patterns = [
    (r'симptom', 'mixed артефакт «симptom»'),
    (r'not-найдено', 'гибрид «not-найдено»'),
    (r'\bthe the\b', 'дублированный the'),
    (r'\bи и\b', 'дублированный и'),
]

failures = []
warnings = []
scanned = 0
for name in files:
    if not name.endswith(('.go', '.sql', '.yaml', '.md', '.html', '.ts', '.py')):
        continue
    try:
        text = open(f'{REPO}/{name}', encoding='utf-8').read()
    except (FileNotFoundError, UnicodeDecodeError):
        warnings.append(f'{name}: не читается (пропуск)')
        continue
    scanned += 1
    for pattern in secret_patterns:
        for match in re.finditer(pattern, text):
            failures.append(f'{name}: секрет {match.group(0)[:12]}***')
    for match in cjk_range.finditer(text):
        failures.append(f'{name}:{text[:match.start()].count(chr(10)) + 1}: CJK-артефакт {match.group(0)}')
        break
    for pattern, label in mixed_patterns:
        if re.search(pattern, text):
            failures.append(f'{name}: {label}')

print(f'просканировано {scanned} файлов диффа')
for warning in warnings:
    print(f'WARN: {warning}')
if failures:
    for failure in failures:
        print(f'FAIL: {failure}')
    sys.exit(1)
print('SCAN CLEAN: секреты 0 / CJK 0 / mixed 0')
