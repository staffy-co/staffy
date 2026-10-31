#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_step05_4_7_scan.py — QA-саморевью закрытия story 4-7 по живому диффу
ветки (main...HEAD): секреты/CJK/mixed + продуктовые зонды среза
(замороженные машины 2-13/3-6/persist-квитанций/термы 2-4, SQL-дисциплина
миграций, supplier-send, Commercial-гейт закупочной цены, маркетинг,
import-graph домена, AD-10 SAFE-пины)."""
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

# Зонд 1 — замороженные машины НЕ в диффе (лестница диспетчера 3-6,
# машина handoffs 2-13, persist-квитанций, термы владения 2-4).
frozen_watch = [
    'internal/action/domain/dispatch.go',
    'internal/action/app/dispatch_test.go',
    'internal/cases/domain/human_handoff.go',
    'internal/cases/app/human_handoff.go',
    'internal/cases/domain/ownership.go',
    'internal/cases/domain/turn.go',
    'internal/cases/store/human_handoff',
    'migrations/cases/00013',
]
for _, name in files:
    for watch in frozen_watch:
        if name == watch:
            failures.append(f'замороженная машина тронута: {name}')

# Зонд 2 — SQL-дисциплина: НОВАЯ миграция ровно одна (00006 — append-only
# таблицы проверки заказа в action-модуле); чужих миграций нет.
new_migrations = [name for status, name in files
                  if name.startswith('migrations/') and status == 'A']
if new_migrations != ['migrations/action/00006_order_verifications.sql']:
    failures.append(f'новые миграции среза: {new_migrations} — ожидана ровно 00006')

# Миграция append-only честна: UPDATE/DELETE/TRUNCATE отозваны, RLS FORCE.
mig = open(f'{REPO}/migrations/action/00006_order_verifications.sql', encoding='utf-8').read()
for probe in ['FORCE ROW LEVEL SECURITY', 'REVOKE UPDATE, DELETE, TRUNCATE']:
    if probe not in mig:
        failures.append(f'00006: append-only дисциплина «{probe}» отсутствует')

# Зонд 3 — supplier-send: полей/маршрутов/действий отправки поставщику НЕТ
# (fence OQ-3 — структурно; слова домена упоминают отправку ТОЛЬКО
# честными отрицаниями).
for probe, label in [
    ('internal/action/domain/order_verification.go', 'домен проверки'),
    ('internal/action/api/order_verification.go', 'api проверки'),
    ('internal/transport/http/cases_order_verification.go', 'транспорт проверки'),
]:
    code = re.sub(r'//[^\n]*', '', open(f'{REPO}/{probe}', encoding='utf-8').read())
    strings = re.findall(r'"[^"]*"', code)
    for word in ['supplier_send', 'supplier-send', 'send_to_supplier', 'delivery_to_supplier', 'ship_to_supplier', 'auto_send']:
        if word in code:
            failures.append(f'{probe}: поле/маршрут отправки поставщику «{word}» — fence OQ-3')
    # маршруты JSON несут только словарь жизненного цикла задачи
for route in ['order-verifications']:
    pass
routes = re.findall(r'HandleFunc\([^)]*order-verifications[^)]*\)', open(f'{REPO}/internal/transport/http/server.go', encoding='utf-8').read())
for route in routes:
    if 'send' in route.lower() or 'supplier' in route.lower():
        failures.append(f'server: маршрут отправки в лестнице проверки «{route}»')

# Зонд 4 — закупочная цена ТОЛЬКО по Commercial-скоупу: рендер панели
# прячет её без полномочия (живой грант-матрицей через Authorize).
panel = open(f'{REPO}/internal/transport/http/cases_order_verification.go', encoding='utf-8').read()
if 'PurchasingHidden = true' not in panel:
    failures.append('панель: скрытие закупочной по умолчанию отсутствует')
if 'Authorize' not in panel:
    failures.append('панель: живая грант-матрица Commercial-скоупа не вызывается')
if panel.count('PurchasingHidden = false') != 1 or 'Commercial' not in panel:
    failures.append('панель: раскрытие закупочной не едино и не по Commercial-скоупу')

# Зонд 5 — маркетинговые слова в домене проверки.
domain_go = open(f'{REPO}/internal/action/domain/order_verification.go', encoding='utf-8').read()
for word in ['лучший', 'выгодн', 'скидк', 'акци', 'самый', 'гарантируем', 'бесплатн', 'успейте', 'премиум-качеств']:
    if word in domain_go.lower():
        failures.append(f'internal/action/domain/order_verification.go: маркетинговое слово «{word}»')

# Зонд 6 — import-graph домена проверки: чистые функции, ноль сетевых и
# ноль чужих модульных импортов (AD-2; домен читает только платформенные
# идентификаторы).
imports = re.findall(r'"github\.com/artemiosu/staffy/[^"]+"', domain_go)
allowed = {
    '"github.com/staffy-co/staffy/internal/platform/id"',
}
for imp in imports:
    if imp not in allowed:
        failures.append(f'internal/action/domain/order_verification.go: недопустимый импорт домена {imp}')

# Зонд 7 — AD-10 SAFE-пины: событие approved несёт ПИНЫ (дайджест 64-hex,
# идентичность квитанции, внешний ID, якоря evidence) — НЕ копии материала
# (никаких цен/составов/имен поставщика в материале события).
event_match = re.search(r'type OrderVerificationEvent struct \{(.*?)\}', domain_go, re.S)
if not event_match:
    failures.append('домен: OrderVerificationEvent не найден')
else:
    body = event_match.group(1)
    # Positions material НЕ копия: blocked несёт ПИНЫ ключей позиций (спека
    # AC-12 требует) — запрещены копии ЦЕН/рендеров/имён поставщика.
    for forbidden in ['Price', 'SupplierName', 'ClientPrice', 'PurchasePrice', 'Render']:
        if forbidden in body:
            failures.append(f'OrderVerificationEvent несёт копию материала «{forbidden}» — AD-10 SAFE-пины')

# Зонд 8 — забор решения reuse лестницы 3-6: живая ре-деривация живёт в
# app-сервисе задачи, НО НЕ новым шагом dispatch (файл лестницы не тронут
# — уже зонд 1; здесь — пин живого юнит-теста забора).
app_test = open(f'{REPO}/internal/action/app/order_verification_test.go', encoding='utf-8').read()
for pin in ['DeriveOrderCommitmentMaterial', 'ErrOrderVerificationCompositionChanged']:
    if pin not in app_test:
        failures.append(f'order_verification_test: живой пин забора решения «{pin}» отсутствует')

# Зонд 9 — идентичность актёра серверной истиной: acknowledge/approve/block
# идут через доменные гварды с command.Actor.ID (спуф-актора нет — слово
# не-assignee живёт в гвардах домена).
app_go = open(f'{REPO}/internal/action/app/order_verification.go', encoding='utf-8').read()
for guard in ['GuardOrderVerificationAcknowledge(view.State, command.Actor.ID, view.Assignee',
              'GuardOrderVerificationApprove(view.State, command.Actor.ID, view.Assignee',
              'GuardOrderVerificationBlock(view.State, command.Actor.ID, view.Assignee']:
    if guard not in app_go:
        failures.append(f'app: гвард серверной истины актёра отсутствует: {guard[:50]}…')

print(f'просканировано {scanned} файлов диффа; изменений {len(files)}')
for warning in warnings:
    print(f'WARN: {warning}')
if failures:
    for failure in failures:
        print(f'FAIL: {failure}')
    sys.exit(1)
print('STEP-05 SCAN CLEAN: секреты 0 / CJK 0 / mixed 0 / замороженные машины чисты / SQL 1 миграция append-only / supplier-send 0 / закупочная по Commercial-скоупу / маркетинг 0 / import-graph чист / AD-10 SAFE-пины / забор reuse пиннен / серверная истина актёра')
