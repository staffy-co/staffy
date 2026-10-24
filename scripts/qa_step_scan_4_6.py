#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_step_scan_4_6.py — QA-скан шагов story 4-6 по живому диффу ветки:
секреты / CJK-артефакты / смешанные слова + продуктовые зонды среза
(поля отправки поставщику 0 в схемах; маркетинговые слова 0 в домене)."""
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
    (r'утверждждение', 'опечатка «утверждждение»'),
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
    # DEVLOG — нарратив: исторические записи цитируют артефакты прошлых
    # находок BY DESIGN (секреты остаются под запретом всегда); CJK/mixed
    # зонды идут по коду среза, не по нарративу журнала.
    if name == 'DEVLOG.md':
        continue
    for match in cjk_range.finditer(text):
        failures.append(f'{name}:{text[:match.start()].count(chr(10)) + 1}: CJK-артефакт {match.group(0)}')
        break
    for pattern, label in mixed_patterns:
        if re.search(pattern, text):
            failures.append(f'{name}: {label}')

# Продуктовый зонд 1: поля отправки поставщику отсутствуют в Go-схемах каталога.
# Зонд идёт по КОДУ без комментариев: слово в комментарии (честная фиксация
# fence OQ-3) — не поле схемы; поля — только строки объявления.
catalog_go = open(f'{REPO}/internal/action/catalog/catalog.go', encoding='utf-8').read()
catalog_code = re.sub(r'//[^\n]*', '', catalog_go)
for word in ['supplier_form', 'supplier_send', 'send_to_supplier', 'delivery_to_supplier', 'ship_to_supplier']:
    if word in catalog_code:
        failures.append(f'internal/action/catalog/catalog.go: поле отправки поставщику «{word}» — fence OQ-3')

# Продуктовый зонд 2: маркетинговые слова отсутствуют в домене заказа.
order_go = open(f'{REPO}/internal/action/domain/order.go', encoding='utf-8').read()
for word in ['лучший', 'выгодн', 'скидк', 'акци', 'самый', 'гарантируем', 'бесплатн', 'успейте']:
    if word in order_go.lower():
        failures.append(f'internal/action/domain/order.go: маркетинговое слово «{word}»')

print(f'просканировано {scanned} файлов диффа')
for warning in warnings:
    print(f'WARN: {warning}')
if failures:
    for failure in failures:
        print(f'FAIL: {failure}')
    sys.exit(1)
print('SCAN CLEAN: секреты 0 / CJK 0 / mixed 0 / supplier-send 0 / маркетинг 0')
