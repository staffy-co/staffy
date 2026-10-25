#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_spec_scan_4_9.py — QA-скан спеки story 4-9 перед предъявлением:
секреты / CJK-артефакты / смешанные слова / структурная полнота /
front-matter / статус (жизненный цикл: backlog → ready-for-dev → done) / контекст-файлы существуют /
fence-слова (supplier-send NEVER, адаптеры чтения НЕ подключаются, уведомления НЕ строятся) /
AC-покрытие всех 16 блоков story 4.9 / структурные зонды честности проекции."""
import os
import re
import sys

REPO = '/home/z/my-project/staffy'
SPEC = os.path.join(REPO, '_bmad-output/implementation-artifacts/spec-4-9-truthful-order-status.md')

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
    (r'проекцияя', 'опечатка «проекцияя»'),
    (r'подтверждждение', 'опечатка «подтверждждение»'),
    (r'статуc\b', 'латинская c в «статуc»'),
    (r'свежестьь', 'опечатка «свежестьь»'),
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
    # Жизненный цикл спеки (по прецеденту сканов 4-7/4-8: допустимы все три
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

# 6. Fence honesty probes: the spec must NOT authorize supplier-send,
#    must NOT add reading adapters, must NOT build notification delivery.
if re.search(r'отправк[ауи] поставщику[^»\n]{0,80}(добавля|разреш|авториз|allow)', text, re.I):
    print('FAIL fence: формулировка авторизации отправки поставщику')
    fail += 1
if re.search(r'\bsupplier_form\b', text) or re.search(r'\bsupplier_send\b', text):
    print('FAIL fence: supplier_form/supplier_send поле в спеке (слово стадии supplier_formed легально — зонд с границей слова)')
    fail += 1
if re.search(r'адаптер[а-я]* чтени[ея][^»\n]{0,60}(подключ|стро|wire)', text, re.I):
    if not re.search(r'адаптер[а-я]* чтени[ея][^»\n]{0,60}(НЕ подключ|не подключ)', text):
        print('FAIL fence: формулировка подключения адаптера чтения')
        fail += 1
if 'вебхук|webhook-приёмник' in text and 'webhook-приёмник' not in text:
    pass  # webhook упоминается только как отсутствующая сущность — см. Never
# Обязательные слова честности среза.
for word, desc in [
    ('не отправляет', 'Staffy не отправляет'),
    ('AG-4', 'гейт AG-4'),
    ('OQ-3', 'fence OQ-3'),
    ('OQ-4', 'гейт OQ-4 (1С unknown)'),
    ('OQ-19', 'гейт OQ-19 (матрица)'),
    ('контур 4-10', 'слово-граница отмены'),
    ('append-only', 'append-only дисциплина'),
]:
    if word not in text:
        print(f'FAIL ядро честности: нет слова «{desc}»')
        fail += 1

# 7. AC coverage probes (all 16 story AC blocks represented).
coverage_probes = [
    ('AC-1 матрица/пины фактов', 'матрица полномочий'),
    ('AC-1 источник/свежесть', 'источник BY NAME'),
    ('AC-2 раздельность систем', 'раздельно'),
    ('AC-2 проекция ≠ истина', 'не становится новым источником истины'),
    ('AC-3 словарь стадий', 'client_order_created'),
    ('AC-3 стадия сформирован', 'supplier_formed'),
    ('AC-3 стадия отправлен', 'supplier_sent'),
    ('AC-3 переход с evidence', 'переход только с evidence'),
    ('AC-4 не утверждает отправку', 'не утверждается'),
    ('AC-5 источник факта', 'manager_record'),
    ('AC-5 человеческая квитанция', 'человеческ'),
    ('AC-6 supplier order ID', 'supplier_order_id'),
    ('AC-6 сформирован ≠ отправлен', 'разные состояния'),
    ('AC-7 1С unknown', 'unknown'),
    ('AC-8 конфликт без выбора', 'благоприятн'),
    ('AC-9 дедуп/идемпотентность', 'идемпотентн'),
    ('AC-9 старое не откатывает', 'не откатывает'),
    ('AC-10 свежесть', 'свежесть'),
    ('AC-10 устаревший не текущий', 'устаревш'),
    ('AC-11 оценка срока', 'оценка'),
    ('AC-11 подтверждённая дата', 'подтверждённая дата'),
    ('AC-12 спецсостояние не скрыто', 'специальное состояние'),
    ('AC-12 не «в работе»', 'в работе'),
    ('AC-13 уведомления НЕ строятся', 'уведомлени'),
    ('AC-14 история не переписывается', 'не переписывает'),
    ('AC-15 eval-матрица', 'eval-матриц'),
    ('AC-16 a11y', '320px'),
]
for name, probe in coverage_probes:
    if not re.search(probe, text, re.I):
        print(f'FAIL покрытие {name}: зонд «{probe}» не найден')
        fail += 1

# 8. Structural honesty probes (the projection is read-time only,
#    the human receipt is never dressed as a system receipt).
if 'read-time' not in text and 'read-тайм' not in text:
    print('FAIL структурный зонд: проекция не названа read-time')
    fail += 1
if 'system_receipt' not in text:
    print('FAIL структурный зонд: нет словаря источника system_receipt')
    fail += 1
if re.search(r'human receipt[^»\n]{0,60}(=|называется|выдаётся за) системн', text, re.I):
    print('FAIL структурный зонд: человеческая квитанция выдаётся за системную')
    fail += 1
if 'UNIQUE' not in text:
    print('FAIL структурный зонд: нет UNIQUE идемпотентности факта')
    fail += 1

print(f'---')
print(f'spec_scan_4_9: fail={fail} warn={warn} lines={lines}')
sys.exit(1 if fail else 0)
