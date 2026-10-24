#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_step05_4_6_scan.py — QA-саморевью закрытия story 4-6 по живому диффу
ветки (main...HEAD): секреты/CJK/mixed + продуктовые зонды среза
(замороженные секции, поля отправки поставщику, ввод оператора,
маркетинг, import-graph, SQL-дисциплина миграций)."""
import re
import subprocess
import sys

REPO = '/home/z/my-project/staffy'

diff = subprocess.run(
    ['git', 'diff', '--name-status', 'main...HEAD'],
    cwd=REPO, capture_output=True, text=True, check=True,
).stdout

files = []
for line in diff.strip().splitlines():
    parts = line.split('\t')
    if len(parts) >= 2:
        files.append((parts[0], parts[-1]))

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
    (r'утверждждение', 'опечатка «утверждждение»'),
]

failures = []
warnings = []
scanned = 0
for status, name in files:
    if not name.endswith(('.go', '.sql', '.yaml', '.md', '.html', '.ts', '.py')):
        continue
    if name.startswith('scripts/qa_') or name == 'DEVLOG.md':
        continue  # the narrative and the scan itself
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

# Зонд 1 — замороженные секции: файлы замороженных контуров НЕ в диффе
# (кроме задокументированных касаний).
frozen_allowed = {
    'internal/cases/domain/human_handoff.go',      # reason +1 (спека)
    'migrations/cases/00025_order_creation_reason.sql',  # словарное зеркало
}
frozen_watch = [
    'internal/cases/domain/observation.go',
    'internal/cases/app/decision_context.go',
    'internal/cases/domain/work.go',
    'internal/cases/domain/commitment.go',
    'internal/cases/domain/external_commitment.go',
    'internal/employee/packs/catalog.go',
    'migrations/cases/00002',
]
for _, name in files:
    for watch in frozen_watch:
        if name.startswith(watch) and name not in frozen_allowed:
            failures.append(f'замороженный файл тронут: {name}')

# Зонд 2 — SQL-дисциплина: НОВАЯ миграция ровно одна (00025 — словарное
# расширение CHECK по прецедентам 00019/00021/00024); чужих миграций нет.
new_migrations = [name for status, name in files
                  if name.startswith('migrations/') and status == 'A']
if new_migrations != ['migrations/cases/00025_order_creation_reason.sql']:
    failures.append(f'новые миграции среза: {new_migrations} — ожидана ровно 00025')

# Зонд 3 — поля отправки поставщику отсутствуют в КОДЕ каталога и домена.
for probe, label in [
    ('internal/action/catalog/catalog.go', 'каталог действий'),
    ('internal/action/domain/order.go', 'домен заказа'),
]:
    code = re.sub(r'//[^\n]*', '', open(f'{REPO}/{probe}', encoding='utf-8').read())
    for word in ['supplier_form', 'supplier_send', 'send_to_supplier', 'delivery_to_supplier', 'ship_to_supplier']:
        if word in code:
            failures.append(f'{probe}: поле отправки поставщику «{word}» — fence OQ-3')

# Зонд 4 — оператор НЕ вводит материал заказа: команда билдера несёт
# ТОЛЬКО якоря (структурный зонд над api.PrepareOrderProposalCommand).
api_go = open(f'{REPO}/internal/action/api/preview.go', encoding='utf-8').read()
cmd_match = re.search(r'type PrepareOrderProposalCommand struct \{(.*?)\}', api_go, re.S)
if not cmd_match:
    failures.append('api: PrepareOrderProposalCommand не найден')
else:
    body = cmd_match.group(1)
    for forbidden in ['TypedInput', 'Digest', 'Price', 'Sum', 'Material', 'Pin']:
        if forbidden in body:
            failures.append(f'api.PrepareOrderProposalCommand несёт материал «{forbidden}» — оператор не вводит состав')

# Зонд 5 — маркетинговые слова в домене заказа.
order_go = open(f'{REPO}/internal/action/domain/order.go', encoding='utf-8').read()
for word in ['лучший', 'выгодн', 'скидк', 'акци', 'самый', 'гарантируем', 'бесплатн', 'успейте', 'премиум-качеств']:
    if word in order_go.lower():
        failures.append(f'internal/action/domain/order.go: маркетинговое слово «{word}»')

# Зонд 6 — import-graph домена заказа: чистые функции, ноль сетевых и
# ноль модульных импортов (AD-2).
order_imports = re.findall(r'"github\.com/artemiosu/staffy/[^"]+"', order_go)
for imp in order_imports:
    if imp.endswith('/action/domain') or imp == '"github.com/artemiosu/staffy"':
        continue
    failures.append(f'internal/action/domain/order.go: недопустимый импорт домена {imp}')

# Зонд 7 — лестница диспетчера 3-6: порядок существующих шагов
# неприкосновенен (живой пин юнит-теста TestDispatchOrderRevalidationRunsBeforeCaseLiveness).
ladder_pin = open(f'{REPO}/internal/action/app/order_test.go', encoding='utf-8').read()
if 'TestDispatchOrderRevalidationRunsBeforeCaseLiveness' not in ladder_pin:
    failures.append('order_test: живой пин порядка шагов лестницы отсутствует')

print(f'просканировано {scanned} файлов диффа; изменений {len(files)}')
for warning in warnings:
    print(f'WARN: {warning}')
if failures:
    for failure in failures:
        print(f'FAIL: {failure}')
    sys.exit(1)
print('STEP-05 SCAN CLEAN: секреты 0 / CJK 0 / mixed 0 / замороженные чисты / SQL 1 миграция / supplier-send 0 / оператор не вводит / маркетинг 0 / import-graph чист / лестница пиннена')
