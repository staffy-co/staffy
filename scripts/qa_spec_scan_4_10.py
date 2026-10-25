#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_spec_scan_4_10.py — QA-скан спеки story 4-10 перед предъявлением:
секреты / CJK-артефакты / смешанные слова / структурная полнота /
front-matter / статус (жизненный цикл: backlog → ready-for-dev → done) / контекст-файлы существуют /
fence-слова (каналы не подключены OQ-1/OQ-2, уведомления НЕ строятся, поиск по телефону НЕ существует OQ-16,
cancel НЕ в каталоге OQ-3, адаптеры чтения НЕ подключаются AG-4, Caller ID НЕ доказательство) /
AC-покрытие всех 16 блоков story 4.10 / структурные зонды честности servicing-контура."""
import os
import re
import sys

REPO = '/home/z/my-project/staffy'
SPEC = os.path.join(REPO, '_bmad-output/implementation-artifacts/spec-4-10-safe-existing-order-servicing.md')

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
    (r'обслуживани[еяя]{2}\b', 'опечатка «обслуживаниея»'),
    (r'раскрыти[ея]\b.*раскрыти[ея]\b.*раскрыти[ея]\b.*раскрыти[ея]\b', 'повтор раскрытие x4 в строке'),
    (r'статуc\b', 'латинская c в «статуc»'),
    (r'интентt\b', 'опечатка «интентt»'),
    (r'FR-42', 'мусорная ссылка FR-42 (правильно FR-62)'),
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
    # Жизненный цикл спеки (по прецеденту сканов 4-7..4-9: допустимы все три
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
#    must NOT add reading adapters, must NOT build notification delivery,
#    must NOT build phone search, must NOT promise or execute cancel.
if re.search(r'отправк[ауи] поставщику[^»\n]{0,80}(добавля|разреш|авториз|allow)', text, re.I):
    print('FAIL fence: формулировка авторизации отправки поставщику')
    fail += 1
if re.search(r'\bsupplier_form\b', text) or re.search(r'\bsupplier_send\b', text):
    print('FAIL fence: supplier_form/supplier_send поле в спеке (слова стадий 4-9 supplier_formed/supplier_sent легальны — зонд с границей слова)')
    fail += 1
if re.search(r'адаптер[а-я]* чтени[ея][^»\n]{0,60}(подключ|стро|wire)', text, re.I):
    if not re.search(r'адаптер[а-я]* чтени[ея][^»\n]{0,60}(НЕ подключ|не подключ|нет)', text):
        print('FAIL fence: формулировка подключения адаптера чтения')
        fail += 1
phone_build = re.search(r'поиск по телефону[^»\n]{0,60}(стро|добавл|реализ)', text, re.I)
if phone_build:
    # Честная граница: «НЕ существует» / «НЕ строить» / OQ-16 слово-граница рядом.
    if not (re.search(r'поиск по телефону[^»\n]{0,80}НЕ существует', text) or
            re.search(r'НЕ строить поиск по телефону', text) or
            re.search(r'поиск по телефону[^»\n]{0,80}OQ-16', text)):
        print('FAIL fence: формулировка построения поиска по телефону')
        fail += 1
if re.search(r'отмен[аыу][^»\n]{0,60}(исполня|выполня|совершается автоматич|executes?)', text, re.I):
    if not re.search(r'отмен[аыу][^»\n]{0,80}(НЕ исполня|не исполня|НЕ выполня|не выполня|руками)', text):
        print('FAIL fence: формулировка исполнения отмены')
        fail += 1
# Обязательные слова честности среза (регистронезависимо — заглавное НЕ тоже честное).
core_words = [
    (r'не отправляются', 'уведомления не отправляются'),
    (r'AG-4', 'гейт AG-4'),
    (r'OQ-1', 'гейт OQ-1 (телефон)'),
    (r'OQ-2', 'гейт OQ-2 (Jivo)'),
    (r'OQ-3', 'fence OQ-3 (supplier-send)'),
    (r'OQ-16', 'гейт OQ-16 (cross-case профиль)'),
    (r'append-only', 'append-only дисциплина'),
    (r'verified', 'уровень verified гейта'),
    (r'контур 2-13', 'слово-граница handoff'),
    (r'контур 3-8', 'слово-граница recovery'),
    (r'контур 4-8', 'слово-граница коррекции'),
]
for word, desc in core_words:
    if not re.search(word, text, re.I):
        print(f'FAIL ядро честности: нет слова «{desc}»')
        fail += 1

# 7. AC coverage probes (all 16 story 4.10 AC blocks represented).
coverage_probes = [
    ('AC-1 словарь интентов 8', 'order_status'),
    ('AC-1 интент отмены', 'cancel'),
    ('AC-1 интент возврата', 'return'),
    ('AC-1 интент рекламации', 'claim'),
    ('AC-1 интент спора', 'dispute'),
    ('AC-1 не раскрывает до проверки', 'не раскрывает'),
    ('AC-2 отсутствие результата не раскрывает', 'не раскрывает сведения о другом клиенте'),
    ('AC-2 различение типом и источником', 'по типу и источнику'),
    ('AC-3 Caller ID не доказательство', 'Caller ID'),
    ('AC-3 знание номера не доказательство', 'знание номера'),
    ('AC-4 proof-пины', 'proof-пины'),
    ('AC-4 scope/уровень/время', 'scope/уровень/время'),
    ('AC-5 раскрытие из проекции 4-9', 'OrderStatusView'),
    ('AC-5 закупочные не показываются', 'закупочн'),
    ('AC-6 факты vs оценка', 'факты системы'),
    ('AC-7 фиксирует, не утверждает', 'не утверждает'),
    ('AC-8 четыре состояния отмены', 'подготовленное действие'),
    ('AC-8 не обещает отмену', 'не обещ'),
    ('AC-9 юридическая ответственность', 'юридическая ответственность'),
    ('AC-9 исходные слова клиента', 'исходные слова клиента'),
    ('AC-10 owner и следующее обновление', 'owner'),
    ('AC-11 продолжение слово-граница', 'продолжение телефон→Jivo'),
    ('AC-12 не подтверждает/не опровергает', 'НЕ подтверждает и НЕ опровергает'),
    ('AC-13 уведомления НЕ строятся', 'уведомлени'),
    ('AC-13 Commitment не трогается', 'Commitment'),
    ('AC-14 eval-матрица', 'eval-матриц'),
    ('AC-14 неверный номер', 'неверный номер'),
    ('AC-14 подменённый Caller ID', 'подменённый Caller ID'),
    ('AC-15 Recovery Case', 'Recovery Case'),
    ('AC-16 a11y', '320px'),
    ('AC-16 Позвать человека', 'Позвать человека'),
    ('AC-16 Вернуться в разговор', 'Вернуться в разговор'),
]
for name, probe in coverage_probes:
    if not re.search(re.escape(probe), text, re.I):
        print(f'FAIL покрытие {name}: зонд «{probe}» не найден')
        fail += 1

# 8. Structural honesty probes (the gate reads only verification_records,
#    the lookup never enumerates, the request never creates an operation).
if 'read-time' not in text and 'read-тайм' not in text:
    print('FAIL структурный зонд: деривации не названы read-time')
    fail += 1
if 'verification_records' not in text:
    print('FAIL структурный зонд: гейт не привязан к verification_records 2-3')
    fail += 1
if 'UNIQUE' not in text:
    print('FAIL структурный зонд: нет UNIQUE идемпотентности запроса')
    fail += 1
if 'RLS' not in text:
    print('FAIL структурный зонд: нет RLS')
    fail += 1
if 'не создаёт новую операцию' not in text and 'не создаёт операцию' not in text:
    print('FAIL структурный зонд: FR-62 (повторное обращение не создаёт операцию)')
    fail += 1
if 'Ambiguous' in text or 'ambiguous' not in text:
    if 'ambiguous' not in text:
        print('FAIL структурный зонд: нет исхода поиска ambiguous')
        fail += 1
if 'неуверифицированного' not in text and 'не-verified' not in text:
    print('FAIL структурный зонд: нет зонда утечки (клиентские слова без исхода поиска)')
    fail += 1

print(f'---')
print(f'spec_scan_4_10: fail={fail} warn={warn} lines={lines}')
sys.exit(1 if fail else 0)
