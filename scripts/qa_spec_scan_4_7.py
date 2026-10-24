#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_spec_scan_4_7.py — QA-скан спеки story 4-7 перед предъявлением:
секреты / CJK-артефакты / смешанные слова / структурная полнота /
front-matter / статус backlog / контекст-файлы существуют / fence-слова."""
import os
import re
import sys

REPO = '/home/z/my-project/staffy'
SPEC = os.path.join(REPO, '_bmad-output/implementation-artifacts/spec-4-7-decision-ready-order-verification.md')

fail = 0
warn = 0

with open(SPEC, encoding='utf-8') as fh:
    text = fh.read()

# 1. Secrets (the PAT of this session must never appear anywhere).
secret_patterns = [
    (r'github_pat_[A-Za-z0-9_]{20,}', 'GitHub PAT'),
    (r'ghp_[A-Za-z0-9]{30,}', 'classic PAT'),
    (r'AKIA[0-9A-Z]{16}', 'AWS key'),
    (r'sk-[A-Za-z0-9]{20,}', 'sk- key'),
]
for pat, name in secret_patterns:
    found = re.findall(pat, text)
    if found:
        print(f'FAIL secret {name}: {len(found)}')
        fail += 1

# 2. CJK artifacts.
cjk = re.findall(r'[\u4e00-\u9fff]', text)
if cjk:
    print(f'FAIL CJK: {len(cjk)} символов')
    fail += 1

# 3. Mixed-script artifacts and typos.
mixed_patterns = [
    (r'симptom', 'mixed «симptom»'),
    (r'not-найдено', 'гибрид «not-найдено»'),
    (r'\bthe the\b', 'дублированный the'),
    (r'\bи и\b', 'дублированный и'),
    (r'утверждждение', 'опечатка «утверждждение»'),
    (r'проверкка', 'опечатка «проверкка»'),
]
for pat, name in mixed_patterns:
    found = re.findall(pat, text)
    if found:
        print(f'FAIL mixed: {name} x{len(found)}')
        fail += 1

# 4. Structure completeness.
required_sections = [
    '## Intent', '## Boundaries & Constraints', '## I/O & Edge-Case Matrix',
    '## Code Map', '## Tasks & Acceptance', '## Implementation Notes',
    '## Spec Change Log',
]
for sec in required_sections:
    if sec not in text:
        print(f'FAIL структура: нет секции {sec}')
        fail += 1
lines = text.count('\n')
if not (60 <= lines <= 140):
    print(f'WARN структура: {lines} строк вне ожидаемого диапазона 60..140')
    warn += 1

# 5. Front-matter fields.
fm = re.match(r"^---\n(.*?)\n---\n", text, re.S)
if not fm:
    print('FAIL front-matter отсутствует')
    fail += 1
else:
    block = fm.group(1)
    for field in ('title:', 'type:', 'created:', 'status:', 'route:', 'baseline_commit:', 'context:'):
        if field not in block:
            print(f'FAIL front-matter: нет поля {field}')
            fail += 1
    m = re.search(r"status: '([^']+)'", block)
    if not m or m.group(1) != 'backlog':
        print(f"FAIL статус: ожидается backlog, найдено {m.group(1) if m else 'нет'}")
        fail += 1
    m = re.search(r"baseline_commit: '([0-9a-f]{40})'", block)
    if not m:
        print('FAIL baseline_commit не sha40')
        fail += 1
    # context files exist.
    for path in re.findall(r"'?\{project-root\}/([^']+)'?", block):
        full = os.path.join(REPO, path)
        if not os.path.exists(full):
            print(f'FAIL контекст-файл не существует: {path}')
            fail += 1

# 6. Fence honesty probes: the spec must NOT authorize supplier-send.
if re.search(r'отправк[ауи] поставщику[^»\n]{0,80}(добавля|разреш|авториз|allow)', text, re.I):
    print('FAIL fence: формулировка авторизации отправки поставщику')
    fail += 1
if 'supplier_form' in text or 'supplier_send' in text:
    print('FAIL fence: supplier_form/supplier_send поле в спеке')
    fail += 1
# The mandatory manual-contour word must be pinned.
if 'ручное действие менеджера' not in text:
    print('FAIL ядро честности: нет обязательного слова ручного контура')
    fail += 1
if 'контур 4-8' not in text:
    print('FAIL граница: нет слова запуска коррекции «контур 4-8»')
    fail += 1

# 7. AC coverage probes (all 16 story AC blocks represented).
coverage_probes = [
    ('AC-1 задача/назначение', 'Delegated Task|задача'),
    ('AC-2 уведомление честно', 'назначен ≠ доставлено'),
    ('AC-3 экран контекст', 'VIN'),
    ('AC-4 закупочные', 'Commercial-скоуп'),
    ('AC-5 версии подтверждения', 'Confirmation'),
    ('AC-6 техническая строка', 'correlation'),
    ('AC-7 параллельное решение', 'CAS'),
    ('AC-8 дедлайн', 'overdue'),
    ('AC-9 revalidation', 'ре-деривация'),
    ('AC-10 receipt', 'Human Verification Receipt'),
    ('AC-11 ручной контур', 'Staffy не отправляет|НЕ отправляет заказ поставщику'),
    ('AC-12 коррекция', 'Нужна коррекция|blocked'),
    ('AC-13 fence честность', 'обход руками'),
    ('AC-14 неприменимость', 'неприменимо'),
    ('AC-15 eval-матрица', 'eval-матриц'),
    ('AC-16 a11y', '320px'),
]
for name, probe in coverage_probes:
    if not re.search(probe, text, re.I):
        print(f'FAIL покрытие {name}: зонд «{probe}» не найден')
        fail += 1

print(f'---')
print(f'spec_scan_4_7: fail={fail} warn={warn} lines={lines}')
sys.exit(1 if fail else 0)
