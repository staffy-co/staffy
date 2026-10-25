#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_step01_scan_4_7.py — QA-скан Dev step-01 story 4-7 (миграция 00006 +
домен order_verification + unit-тесты + регистрация TestMain): секреты /
CJK-артефакты / опечатки / supplier-send зонд / структурные пины шага."""
import os
import re
import subprocess
import sys

REPO = '/home/z/my-project/staffy'

fail = 0
warn = 0

# Живой дифф шага: изменённые/новые файлы против HEAD (незакоммиченный шаг).
diff = subprocess.run(
    ['git', 'diff', '--name-only', 'HEAD'],
    cwd=REPO, capture_output=True, text=True).stdout.split()
new_files = subprocess.run(
    ['git', 'ls-files', '--others', '--exclude-standard'],
    cwd=REPO, capture_output=True, text=True).stdout.split()
files = sorted(set(diff) | set(new_files))
step_files = [f for f in files if f in (
    'migrations/action/00006_order_verifications.sql',
    'internal/action/domain/order_verification.go',
    'internal/action/domain/order_verification_test.go',
    'tests/integration/tenant_test.go',
)]
if len(step_files) != 4:
    print(f'FAIL состав шага: {len(step_files)} из 4 файлов: {step_files}')
    fail += 1
else:
    print(f'OK состав шага: 4/4 файла')

blob = ''
for path in step_files:
    with open(os.path.join(REPO, path), encoding='utf-8') as fh:
        blob += fh.read()
# Секреты проверяются по ДОБАВЛЕННЫМ строкам живого диффа (правка
# существующего файла не наследует чужие литералы — урок QA-4-6).
added = subprocess.run(
    ['git', 'diff', '-U0', 'HEAD', '--'] + step_files,
    cwd=REPO, capture_output=True, text=True).stdout
added_lines = '\n'.join(line[1:] for line in added.splitlines()
                        if line.startswith('+') and not line.startswith('+++'))
new_blob = ''
for path in step_files:
    if path in ('migrations/action/00006_order_verifications.sql',
                'internal/action/domain/order_verification.go',
                'internal/action/domain/order_verification_test.go'):
        with open(os.path.join(REPO, path), encoding='utf-8') as fh:
            new_blob += fh.read()
scan_blob = added_lines + new_blob

# 1. Secrets (the PAT of this session must never appear anywhere).
secret_patterns = [
    (r'github_pat_[A-Za-z0-9_]{20,}', 'GitHub PAT'),
    (r'ghp_[A-Za-z0-9]{30,}', 'classic PAT'),
    (r'AKIA[0-9A-Z]{16}', 'AWS key'),
    (r'sk-[A-Za-z0-9]{20,}', 'sk- key'),
    (r'staffy-runtime-test', 'runtime password literal'),
]
for pat, name in secret_patterns:
    found = re.findall(pat, scan_blob)
    if found:
        print(f'FAIL secret {name}: {len(found)}')
        fail += 1
if not any(re.findall(p, scan_blob) for p, _ in secret_patterns):
    print('OK секреты: 0')

# 2. CJK artifacts.
cjk = re.findall(r'[\u4e00-\u9fff]', scan_blob)
if cjk:
    print(f'FAIL CJK: {len(cjk)} символов')
    fail += 1
else:
    print('OK CJK: 0')

# 3. Supplier-send probe (OQ-3): the migration and the domain carry NO
# supplier-send column/field/route; the only send mentions are the honest
# negation words of the domain constants.
sql = ''
with open(os.path.join(REPO, 'migrations/action/00006_order_verifications.sql'), encoding='utf-8') as fh:
    sql += fh.read()
for forbidden in ('supplier_send', 'send_to_supplier', 'dispatch_supplier', 'auto_send'):
    if forbidden in sql.lower():
        print(f'FAIL supplier-send зонд SQL: {forbidden}')
        fail += 1
if not any(f in sql.lower() for f in ('supplier_send', 'send_to_supplier', 'dispatch_supplier', 'auto_send')):
    print('OK supplier-send зонд SQL: 0')

# 4. The SQL CHECK mirrors of the compile-time dictionaries.
for word in ("'assigned'", "'acknowledged'", "'approved'", "'blocked'"):
    if word not in sql:
        print(f'FAIL SQL-зеркало kind {word}')
        fail += 1
for word in ("'verify_4h'", "'verify_1d'"):
    if word not in sql:
        print(f'FAIL SQL-зеркало deadline {word}')
        fail += 1
for word in ("'wrong_part'", "'wrong_vehicle'", "'price_changed'", "'availability_changed'", "'confirmation_issue'"):
    if word not in sql:
        print(f'FAIL SQL-зеркало reason {word}')
        fail += 1
print('OK SQL-зеркала словарей: kind 4/4, deadline 2/2, reason 5/5' if fail == 0 else 'СМ. ВЫШЕ')

# 5. Append-only discipline: RLS FORCE + INSERT/SELECT grants + REVOKE.
for needle in ('ENABLE ROW LEVEL SECURITY', 'FORCE ROW LEVEL SECURITY',
               'GRANT SELECT, INSERT', 'REVOKE UPDATE, DELETE, TRUNCATE'):
    count = sql.count(needle)
    if count < 2:
        print(f'FAIL append-only дисциплина: {needle} x{count} (ожидалось 2)')
        fail += 1
print('OK append-only: RLS FORCE x2, гранты INSERT/SELECT x2, REVOKE x2' if fail == 0 else 'СМ. ВЫШЕ')

# 6. The FK anchor stays INSIDE the action module (AD-2).
if 'REFERENCES action.operations' not in sql:
    print('FAIL якорь FK операции: модульная ссылка отсутствует')
    fail += 1
else:
    print('OK якорь FK: action.operations (внутри модуля, AD-2)')

# 7. The frozen machines untouched: the diff of the step touches ONLY the
# four files of the step (2-13/human_handoff, лестница 3-6/dispatch.go,
# persist-квитанции — name-only probe).
frozen = [f for f in files if f not in step_files]
if frozen:
    print(f'WARN файлы вне шага в диффе: {frozen}')
    warn += 1
else:
    print('OK замороженные машины: дифф шага не трогает (name-only 0)')

# 8. TestMain registration of 00006.
tenant_test = ''
with open(os.path.join(REPO, 'tests/integration/tenant_test.go'), encoding='utf-8') as fh:
    tenant_test += fh.read()
if 'migrations/action/00006_order_verifications.sql' not in tenant_test:
    print('FAIL регистрация TestMain: 00006 отсутствует')
    fail += 1
else:
    print('OK регистрация TestMain: 00006 в hard list')

print(f'---')
print(f'qa_step01_scan_4_7: fail={fail} warn={warn}')
sys.exit(1 if fail else 0)
