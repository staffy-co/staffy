#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_step05_4_9_scan.py — QA-саморевью закрытия story 4-9 по живому диффу
ветки (main...HEAD): секреты/CJK/mixed + продуктовые зонды среза
(замороженные машины 2-7/2-13/3-6/persist-квитанций/термы 2-4/команды
2-15 и ПАНЕЛИ 4-6..4-8, SQL-дисциплина миграций, supplier-send +
уведомления + ценовые поля, import-graph домена, AD-10 SAFE-пины факта,
идемпотентность BY KEY, ZERO записей деривации, серверная истина
актёра)."""
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
    (r'статуспроекция', 'слипшийся «статуспроекция»'),
]

failures = []
warnings = []
scanned = 0
for status, name in files:
    if not name.endswith(('.go', '.sql', '.yaml', '.md', '.html', '.ts', '.py', '.css')):
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

# Зонд 1 — замороженные машины и ПАНЕЛИ 4-6..4-8 НЕ в диффе (наблюдения
# 2-7, машина handoffs 2-13, лестница диспетчера 3-6, persist-квитанций,
# термы владения 2-4, команды операторской коррекции фактов 2-15 —
# ДРУГОЙ контур; панели 4-6/4-7/4-8 не правились — минимизация диффа).
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
    # 2-7 наблюдения владельца систем — провайдер-observed прецедент читается, не правится.
    'internal/cases/domain/owning_system_observation.go',
    # ПАНЕЛИ 4-6..4-8 — не правятся (спека: новая секция после «Коррекции заказа»).
    'internal/transport/http/cases_order.go',
    'internal/transport/http/cases_order_verification.go',
    'internal/transport/http/cases_order_correction.go',
    'internal/transport/http/cases_order_render_test.go',
    'internal/transport/http/cases_order_verification_render_test.go',
    'internal/transport/http/cases_order_correction_render_test.go',
]
for _, name in files:
    for watch in frozen_watch:
        if name == watch:
            failures.append(f'замороженная машина/панель тронута: {name}')

# Зонд 2 — SQL-дисциплина: НОВАЯ миграция ровно одна (00008 — append-only
# таблица фактов поставщика в action-модуле); чужих миграций нет.
new_migrations = [name for status, name in files
                  if name.startswith('migrations/') and status == 'A']
if new_migrations != ['migrations/action/00008_order_supplier_facts.sql']:
    failures.append(f'новые миграции среза: {new_migrations} — ожидана ровно 00008')

# Миграция append-only честна: UPDATE/DELETE/TRUNCATE отозваны, RLS FORCE;
# UNIQUE полного ключа факта — структурный бэкстоп идемпотентности BY KEY.
mig = open(f'{REPO}/migrations/action/00008_order_supplier_facts.sql', encoding='utf-8').read()
for probe in ['FORCE ROW LEVEL SECURITY', 'REVOKE UPDATE, DELETE, TRUNCATE',
              'CREATE UNIQUE INDEX order_supplier_fact_key',
              'kind IN (\'formed\', \'sent\')']:
    if probe not in mig:
        failures.append(f'00008: дисциплина «{probe}» отсутствует')

# Зонд 3 — supplier-send + уведомления + ценовые поля: полей/маршрутов
# отправки поставщику НЕТ (fence OQ-3 — структурно); полей доставки
# уведомлений НЕТ (OQ-1/OQ-2 — панель единственная поверхность); ценовых
# полей в статус-вью НЕТ (закупочные данные скрыты Commercial-гейтом
# соседних поверхностей).
for probe in [
    'internal/action/domain/order_status.go',
    'internal/action/api/order_status.go',
    'internal/action/app/order_status.go',
    'internal/transport/http/cases_order_status.go',
]:
    code = re.sub(r'//[^\n]*', '', open(f'{REPO}/{probe}', encoding='utf-8').read())
    for word in ['supplier_send', 'send_to_supplier', 'delivery_to_supplier', 'ship_to_supplier', 'auto_send',
                 'dispatch_supplier', 'SupplierSend']:
        if word in code:
            failures.append(f'{probe}: поле/маршрут отправки «{word}» — fence OQ-3')
    for word in ['DeliveryStatus', 'delivery_status', 'NotifiedAt', 'notified_at', 'ReadAt', 'read_at']:
        if word in code:
            failures.append(f'{probe}: поле доставки уведомлений «{word}» — OQ-1/OQ-2')
    for word in ['UnitPrice', 'ClientPrice', 'PurchasePrice', 'PurchasingPrice', 'KnownClientSum']:
        if word in code:
            failures.append(f'{probe}: ценовое поле «{word}» — закупочные данные скрыты')

# Зонд 4 — маркетинговые слова в домене статуса.
domain_go = open(f'{REPO}/internal/action/domain/order_status.go', encoding='utf-8').read()
for word in ['лучший', 'выгодн', 'скидк', 'акци', 'самый', 'гарантируем', 'бесплатн', 'успейте', 'премиум-качеств']:
    if word in domain_go.lower():
        failures.append(f'internal/action/domain/order_status.go: маркетинговое слово «{word}»')

# Зонд 5 — import-graph домена статуса: чистые функции, ноль сетевых и
# ноль чужих модульных импортов (AD-2; домен читает только платформенные
# идентификаторы — ZERO записей структурно: никаких репозиториев).
imports = re.findall(r'"github\.com/artemiosu/staffy/[^"]+"', domain_go)
allowed = {
    '"github.com/staffy-co/staffy/internal/platform/id"',
}
for imp in imports:
    if imp not in allowed:
        failures.append(f'internal/action/domain/order_status.go: недопустимый импорт домена {imp}')

# Зонд 6 — AD-10 SAFE-пины: факт несёт ПИНЫ (вид слова, номер заказа
# поставщика, два времени, актёр) — НЕ копии материала (никаких
# цен/составов/рендеров в материале факта).
event_match = re.search(r'type OrderSupplierFact struct \{(.*?)\}', domain_go, re.S)
if not event_match:
    failures.append('домен: OrderSupplierFact не найден')
else:
    body = event_match.group(1)
    for forbidden in ['Price', 'SupplierName', 'ClientPrice', 'PurchasePrice', 'Render', 'Material', 'Positions']:
        if forbidden in body:
            failures.append(f'OrderSupplierFact несёт копию материала «{forbidden}» — AD-10 SAFE-пины')

# Зонд 7 — идемпотентность BY KEY: реплей ДО гварда в app (повтор
# команды отвечает сохранённым фактом), store-duplicate → перечитка →
# реплей; UNIQUE полного ключа — бэкстоп (зонд 2 уже пиннен).
app_go = open(f'{REPO}/internal/action/app/order_status.go', encoding='utf-8').read()
for pin in ['Replay: true', 'ErrOrderSupplierFactDuplicate', 'ReadOperationSupplierFacts']:
    if pin not in app_go:
        failures.append(f'app: пин идемпотентности «{pin}» отсутствует')
# Лестница sent-after-formed ДО append — гвард поимённо.
if 'GuardOrderSupplierFact(command.Kind' not in app_go:
    failures.append('app: гвард лестницы sent-after-formed отсутствует')

# Зонд 8 — ZERO записей деривации: проекция собирается ТОЛЬКО чтениями
# существующих портов; в домене нет store-вызовов (зонд 5); в app сборка
# читает operations/dispatches/verifications/corrections/orderFacts и
# НЕ пишет ничего кроме append факта.
for pin in ['ReadCaseOperations', 'ReadActionReceipts', 'ReadOperationVerifications', 'ReadOperationCorrections',
            'ReadOperationSupplierFacts', 'DeriveOrderStatusProjection']:
    if pin not in app_go:
        failures.append(f'app: пин read-time сборки «{pin}» отсутствует (ZERO записей AC-2)')
# Оценка срока — живым материалом (reuse DeriveOrderCommitmentMaterial).
if 'DeriveOrderCommitmentMaterial' not in app_go:
    failures.append('app: оценка срока не читает живой материал заказа (reuse DeriveOrderCommitmentMaterial)')

# Зонд 9 — серверная истина актёра: append факта идёт с command.Actor.ID
# (спуф-актора нет); якорь операции — живым чтением кейса.
if 'ActorID:         command.Actor.ID' not in app_go and 'ActorID: command.Actor.ID' not in app_go:
    failures.append('app: append факта не несёт серверную истину актёра (command.Actor.ID)')
if 'orderOperationAnchor' not in app_go or 'domain.OrderActionID' not in app_go:
    failures.append('app: якорь живой операции заказа отсутствует')

# Зонд 10 — структурные пины тестов: реплей/гонка/лестница пиннены живыми
# прогонами app-тестов; тасование AC-9 — DeepEqual в domain-тестах.
app_test = open(f'{REPO}/internal/action/app/order_status_test.go', encoding='utf-8').read()
for pin in ['replayed.Replay', 'outcome.Replay', 'len(f.facts.facts); got != 1', 'сначала запишите формирование']:
    if pin not in app_test:
        failures.append(f'order_status_test: живой пин «{pin[:50]}…» отсутствует')
domain_test = open(f'{REPO}/internal/action/domain/order_status_test.go', encoding='utf-8').read()
if 'reflect.DeepEqual' not in domain_test:
    failures.append('order_status_test: тасование AC-9 не пиннено DeepEqual')

print(f'просканировано {scanned} файлов диффа; изменений {len(files)}')
for warning in warnings:
    print(f'WARN: {warning}')
if failures:
    for failure in failures:
        print(f'FAIL: {failure}')
    sys.exit(1)
print('STEP-05 SCAN CLEAN: секреты 0 / CJK 0 / mixed 0 / замороженные машины и панели 4-6..4-8 чисты / SQL 1 миграция 00008 append-only + UNIQUE полного ключа / supplier-send 0 / уведомительные поля 0 / ценовые поля 0 / маркетинг 0 / import-graph чист (домен только platform/id — ZERO записей) / AD-10 SAFE-пины факта / идемпотентность BY KEY + лестница / read-time сборка существующими портами + reuse живого материала оценки / серверная истина актёра / живые пины тестов (реплей/гонка/лестница/тасование)')
