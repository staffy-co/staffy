#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_spec_scan_4_8.py — QA-скан спеки story 4-8 перед предъявлением:
секреты / CJK-артефакты / смешанные слова / структурная полнота /
front-matter / статус (жизненный цикл: backlog → ready-for-dev → done) / контекст-файлы существуют / fence-слова / структурный гейт дубля."""
import os
import re
import sys

REPO = '/home/z/my-project/staffy'
SPEC = os.path.join(REPO, '_bmad-output/implementation-artifacts/spec-4-8-order-correction-before-purchase.md')

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
    (r'коррекцияя', 'опечатка «коррекцияя»'),
    (r'подтверждждение', 'опечатка «подтверждждение»'),
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
    # Жизненный цикл спеки (по прецеденту скана 4-7: допустимы все три
    # живых состояния, чужое слово — отказ).
    if not m or m.group(1) not in ('backlog', 'ready-for-dev', 'done'):
        print(f"FAIL статус: ожидается backlog|ready-for-dev|done, найдено {m.group(1) if m else 'нет'}")
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

# 6. Fence honesty probes: the spec must NOT authorize supplier-send
#    and must NOT admit a change-action into the catalog.
if re.search(r'отправк[ауи] поставщику[^»\n]{0,80}(добавля|разреш|авториз|allow)', text, re.I):
    print('FAIL fence: формулировка авторизации отправки поставщику')
    fail += 1
if 'supplier_form' in text or 'supplier_send' in text:
    print('FAIL fence: supplier_form/supplier_send поле в спеке')
    fail += 1
if re.search(r'изменени[ея] заказа[^»\n]{0,60}(действие каталога добавля|каталог.*добавля)', text, re.I):
    print('FAIL fence: формулировка добавления change-действия в каталог')
    fail += 1
# The mandatory manual-contour word must be pinned.
if 'ручное действие менеджера' not in text:
    print('FAIL ядро честности: нет обязательного слова ручного контура')
    fail += 1
if 'контур 4-10' not in text:
    print('FAIL граница: нет слова-рекомендации отмены «контур 4-10»')
    fail += 1
if 'не имитирует' not in text:
    print('FAIL ядро честности: нет слова «Staffy не имитирует изменение»')
    fail += 1

# 7. AC coverage probes (all 16 story AC blocks represented).
coverage_probes = [
    ('AC-1 коррекция открыта', 'Correction Work Item'),
    ('AC-2 блок продвижения', 'pending_correction'),
    ('AC-3 версии входов/stale', 'EDIT-лестница'),
    ('AC-4 replacement/связь', 'замена'),
    ('AC-5 контекст клиенту', 'рекомендуемое объяснение'),
    ('AC-6 не скрывает изменение', 'не отправлялся'),
    ('AC-7 прежде Confirmation', 'не переносится'),
    ('AC-8 новое Confirmation', 'подтвержд'),
    ('AC-9 дубль не создаётся', 'дублирующий заказ не создаётся'),
    ('AC-10 не имитирует', 'pending_correction до записи перечита|перечит'),
    ('AC-11 отмена человеком', 'контур 4-10'),
    ('AC-12 возврат на проверку', 'pending_manager_verification'),
    ('AC-13 скрытое ухудшение', 'скрытое ухудшение запрещено'),
    ('AC-14 итог/длительность', 'длительность'),
    ('AC-15 eval-матрица', 'eval-матриц'),
    ('AC-16 a11y', '320px'),
]
for name, probe in coverage_probes:
    if not re.search(probe, text, re.I):
        print(f'FAIL покрытие {name}: зонд «{probe}» не найден')
        fail += 1

# 8. Structural gate probes (the no-duplicate order guarantee).
if 'новый заказ не создаётся' not in text:
    print('FAIL структурный гейт: нет слова отказа авторизации дубля')
    fail += 1
if 'superseded BY NAME' not in text:
    print('FAIL структурный гейт: нет superseded-слова старого Confirmation')
    fail += 1

print(f'---')
print(f'spec_scan_4_8: fail={fail} warn={warn} lines={lines}')
sys.exit(1 if fail else 0)
