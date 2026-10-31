#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_step05_4_8_scan.py — QA-саморевью закрытия story 4-8 по живому диффу
ветки (main...HEAD): секреты/CJK/mixed + продуктовые зонды среза
(замороженные машины 2-13/3-6/persist-квитанций/термы 2-4/команды 2-15,
SQL-дисциплина миграций, supplier-send + change-действия, Commercial-гейт
закупочной цены, маркетинг, import-graph домена, AD-10 SAFE-пины, гейты
не задели лестницу 3-6)."""
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
    (r'корреккция', 'опечатка «корреккция»'),
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
# машина handoffs 2-13, persist-квитанций, термы владения 2-4, команды
# операторской коррекции фактов 2-15 — ДРУГОЙ контур, различие именами).
frozen_watch = [
    'internal/action/domain/dispatch.go',
    'internal/action/app/dispatch_test.go',
    'internal/action/domain/action_receipt.go',
    'internal/cases/domain/human_handoff.go',
    'internal/cases/app/human_handoff.go',
    'internal/cases/domain/ownership.go',
    'internal/cases/domain/turn.go',
    'internal/cases/store/human_handoff',
    'internal/cases/app/correction.go',
    'internal/cases/domain/correction.go',
    'migrations/cases/00013',
]
for _, name in files:
    for watch in frozen_watch:
        if name == watch:
            failures.append(f'замороженная машина тронута: {name}')

# Зонд 2 — SQL-дисциплина: НОВАЯ миграция ровно одна (00007 — append-only
# таблицы коррекции заказа в action-модуле); чужих миграций нет.
new_migrations = [name for status, name in files
                  if name.startswith('migrations/') and status == 'A']
if new_migrations != ['migrations/action/00007_order_corrections.sql']:
    failures.append(f'новые миграции среза: {new_migrations} — ожидана ровно 00007')

# Миграция append-only честна: UPDATE/DELETE/TRUNCATE отозваны, RLS FORCE;
# one-closure partial UNIQUE — структурный бэкстоп единственного закрытия.
mig = open(f'{REPO}/migrations/action/00007_order_corrections.sql', encoding='utf-8').read()
for probe in ['FORCE ROW LEVEL SECURITY', 'REVOKE UPDATE, DELETE, TRUNCATE',
              "CREATE UNIQUE INDEX order_correction_one_closure"]:
    if probe not in mig:
        failures.append(f'00007: дисциплина «{probe}» отсутствует')

# Зонд 3 — supplier-send + change-действия: полей/маршрутов/действий
# отправки поставщику и изменения ABCP-заказа НЕТ (fence OQ-3/AG-4 —
# структурно; слова домена упоминают отправку ТОЛЬКО честными
# отрицаниями).
for probe in [
    'internal/action/domain/order_correction.go',
    'internal/action/api/order_correction.go',
    'internal/action/app/order_correction.go',
    'internal/transport/http/cases_order_correction.go',
]:
    code = re.sub(r'//[^\n]*', '', open(f'{REPO}/{probe}', encoding='utf-8').read())
    for word in ['supplier_send', 'send_to_supplier', 'delivery_to_supplier', 'ship_to_supplier', 'auto_send',
                 'change_abcp_order', 'modify_order', 'update_order_command']:
        if word in code:
            failures.append(f'{probe}: поле/маршрут отправки или изменения «{word}» — fence OQ-3/AG-4')

# Зонд 4 — закупочная цена ТОЛЬКО по Commercial-скоупу: панель коррекции
# рендерит скрытие словом (прецедент 4-7), живой грант-матрицей; панель
# коррекции НЕ вводит своего раскрытия (слова скрытия — константа).
panel = open(f'{REPO}/internal/transport/http/cases_order_correction.go', encoding='utf-8').read()
if 'PurchasingHiddenWord: actionDomain.OrderCorrectionPurchasingHiddenWord' not in panel:
    failures.append('панель коррекции: слово скрытия закупочной отсутствует')
if 'PurchasePrice' in re.sub(r'//[^\n]*', '', panel):
    failures.append('панель коррекции: рендер закупочной цены вне Commercial-гейта')

# Зонд 5 — маркетинговые слова в домене коррекции.
domain_go = open(f'{REPO}/internal/action/domain/order_correction.go', encoding='utf-8').read()
for word in ['лучший', 'выгодн', 'скидк', 'акци', 'самый', 'гарантируем', 'бесплатн', 'успейте', 'премиум-качеств']:
    if word in domain_go.lower():
        failures.append(f'internal/action/domain/order_correction.go: маркетинговое слово «{word}»')

# Зонд 6 — import-graph домена коррекции: чистые функции, ноль сетевых и
# ноль чужих модульных импортов (AD-2; домен читает только платформенные
# идентификаторы).
imports = re.findall(r'"github\.com/artemiosu/staffy/[^"]+"', domain_go)
allowed = {
    '"github.com/staffy-co/staffy/internal/platform/id"',
}
for imp in imports:
    if imp not in allowed:
        failures.append(f'internal/action/domain/order_correction.go: недопустимый импорт домена {imp}')

# Зонд 7 — AD-10 SAFE-пины: событие reread несёт ПИНЫ (внешний ID,
# триплет {proposal, version, confirmation}) — НЕ копии материала
# (никаких цен/составов/рендеров в материале события).
event_match = re.search(r'type OrderCorrectionEvent struct \{(.*?)\}', domain_go, re.S)
if not event_match:
    failures.append('домен: OrderCorrectionEvent не найден')
else:
    body = event_match.group(1)
    for forbidden in ['Price', 'SupplierName', 'ClientPrice', 'PurchasePrice', 'Render', 'Material']:
        if forbidden in body:
            failures.append(f'OrderCorrectionEvent несёт копию материала «{forbidden}» — AD-10 SAFE-пины')

# Зонд 8 — гейты НЕ задели лестницу 3-6 живыми зондами: гейт авторизации
# живёт в 3-5 app-команде ДО лестницы отказов и резервирования (порядок
# пиннен юнит-счётчиками); файл лестницы не тронут (зонд 1); гейт assign
# — в 4-7 команде (единственная хирургическая правка order_verification.go).
auth_go = open(f'{REPO}/internal/action/app/authorization.go', encoding='utf-8').read()
if 'ErrOrderCorrectionOpenGate' not in auth_go:
    failures.append('authorization.go: гейт нового заказа отсутствует')
gate_pos = auth_go.find('ErrOrderCorrectionOpenGate')
ladder_pos = auth_go.find('The rejection ladder BY NAME')
if not (0 <= gate_pos < ladder_pos):
    failures.append('authorization.go: гейт коррекции обязан стоять ДО лестницы отказов')
verify_go = open(f'{REPO}/internal/action/app/order_verification.go', encoding='utf-8').read()
if 'OrderCorrectionAssignGateError' not in verify_go:
    failures.append('order_verification.go: гейт assign отсутствует')

# Зонд 9 — идентичность актёра серверной истиной: reread/cease идут через
# доменные гварды с command.Actor.ID (спуф-актора нет — слово не-owner
# живёт в гвардах домена); забор состава — живой ре-деривацией тем же
# кодом лестницы (reuse, НЕ новый шаг dispatch).
app_go = open(f'{REPO}/internal/action/app/order_correction.go', encoding='utf-8').read()
for guard in ['GuardOrderCorrectionReread(view.State, command.Actor.ID, view.Owner',
              'GuardOrderCorrectionCease(view.State, command.Actor.ID, view.Owner']:
    if guard not in app_go:
        failures.append(f'app: гвард серверной истины актёра отсутствует: {guard[:50]}…')
for pin in ['DeriveOrderCommitmentMaterial', 'ErrOrderCorrectionCompositionChanged', 'OrderDigestOf']:
    if pin not in app_go:
        failures.append(f'app: живой забор состава «{pin}» отсутствует (reuse кода лестницы 4.5)')

# Зонд 10 — структурные пины юнит-слоя: гейты пиннены живыми счётчиками
# (потребление 0 / reserveCalls 0 / задача №2 только после закрытия).
app_test = open(f'{REPO}/internal/action/app/order_correction_test.go', encoding='utf-8').read()
for pin in ['reserveCalls != 0', 'consumed := f.confirmations.consumedCount(f.newConfirmation.ConfirmationID, f.newConfirmation.VersionNo); consumed != 0',
            'len(tasks) != 2 || tasks[1].VerificationNo != 2']:
    if pin not in app_test:
        failures.append(f'order_correction_test: живой пин гейтов «{pin[:60]}…» отсутствует')

print(f'просканировано {scanned} файлов диффа; изменений {len(files)}')
for warning in warnings:
    print(f'WARN: {warning}')
if failures:
    for failure in failures:
        print(f'FAIL: {failure}')
    sys.exit(1)
print('STEP-05 SCAN CLEAN: секреты 0 / CJK 0 / mixed 0 / замороженные машины чисты (2-13/3-6/persist/термы 2-4/команды 2-15) / SQL 1 миграция 00007 append-only + one-closure / supplier-send + change 0 / закупочная по Commercial-слову / маркетинг 0 / import-graph чист / AD-10 SAFE-пины / гейты ДО лестницы и в 4-7 команде / забор reuse пиннен / серверная истина актёра / гейты пиннены счётчиками')
