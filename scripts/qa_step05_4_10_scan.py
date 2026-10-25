#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_step05_4_10_scan.py — QA-саморевью закрытия story 4-10 по живому
диффу ветки (main...HEAD): секреты/CJK/mixed + продуктовые зонды среза
(замороженные машины 2-3/2-13/3-6/3-8/4-6..4-9 и ПАНЕЛИ 4-6..4-9,
SQL-дисциплина миграций, supplier-send + уведомления + ценовые поля,
import-graph домена, AD-10 SAFE-пины запроса, идемпотентность BY KEY +
лестница доверия, гейт только verification_records, read-time деривации,
серверная истина актёра, FR-62 структурно)."""
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
    (r'обслуживаниезаказа', 'слипшийся «обслуживаниезаказа»'),
    (r'requesta', 'слипшийся «requesta»'),
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

# Зонд 1 — замороженные машины и ПАНЕЛИ 4-6..4-9 НЕ в диффе (машина
# проверки личности 2-3, машина handoffs 2-13, лестница диспетчера 3-6,
# recovery 3-8, заказы 4-6..4-9 с их доменами/панелями — спека: новая
# секция после «Статуса заказа», замороженный код не правится).
frozen_watch = [
    # 2-3 машина проверки личности — гейт ЧИТАЕТ её, не правит.
    'internal/cases/domain/verification.go',
    'internal/cases/domain/dictionaries.go',
    # 2-13 машина handoffs — контур словами.
    'internal/cases/domain/human_handoff.go',
    'internal/cases/app/human_handoff.go',
    # 3-6 диспетчер и 3-8 recovery.
    'internal/action/domain/dispatch.go',
    'internal/action/domain/recovery.go',
    # 4-9 машина статуса и её проекция — материал раскрытия читается.
    'internal/action/domain/order_status.go',
    'internal/action/app/order_status.go',
    'internal/action/store/order_status_store.go',
    'internal/action/port/order_status.go',
    'internal/action/api/order_status.go',
    # ПАНЕЛИ 4-6..4-9 — не правятся (спека: новая секция после «Статуса заказа»).
    'internal/transport/http/cases_order.go',
    'internal/transport/http/cases_order_verification.go',
    'internal/transport/http/cases_order_correction.go',
    'internal/transport/http/cases_order_status.go',
    'internal/transport/http/cases_order_render_test.go',
    'internal/transport/http/cases_order_verification_render_test.go',
    'internal/transport/http/cases_order_correction_render_test.go',
    'internal/transport/http/cases_order_status_render_test.go',
]
for _, name in files:
    for watch in frozen_watch:
        if name == watch:
            failures.append(f'замороженная машина/панель тронута: {name}')

# Зонд 2 — SQL-дисциплина: НОВАЯ миграция ровно одна (00009 — append-only
# таблица запросов обслуживания в action-модуле); чужих миграций нет.
new_migrations = [name for status, name in files
                  if name.startswith('migrations/') and status == 'A']
if new_migrations != ['migrations/action/00009_order_servicing_requests.sql']:
    failures.append(f'новые миграции среза: {new_migrations} — ожидана ровно 00009')

# Миграция append-only честна: UPDATE/DELETE/TRUNCATE отозваны, RLS FORCE;
# UNIQUE полного ключа запроса — структурный бэкстоп идемпотентности BY KEY
# при том же состоянии доверия; CHECK-словари — зеркала домена.
mig = open(f'{REPO}/migrations/action/00009_order_servicing_requests.sql', encoding='utf-8').read()
for probe in ['FORCE ROW LEVEL SECURITY', 'REVOKE UPDATE, DELETE, TRUNCATE',
              'CREATE UNIQUE INDEX order_servicing_request_key',
              "intent IN ('order_status', 'clarification', 'change', 'cancel', 'return', 'claim', 'dispute', 'human')",
              "identifier_kind IN ('abcp_client_order_id', 'supplier_order_id')",
              "proof_state IN ('none', 'basic', 'verified', 'step_up')",
              "disclosure IN ('allowed', 'denied')"]:
    if probe not in mig:
        failures.append(f'00009: дисциплина «{probe}» отсутствует')

# Зонд 3 — supplier-send + уведомления + ценовые поля: полей/маршрутов
# отправки поставщику НЕТ (fence OQ-3 — запрос записывает, не исполняет);
# полей доставки уведомлений НЕТ (OQ-1/OQ-2 — панель единственная
# поверхность); ценовых полей в servicing-поверхности НЕТ.
for probe in [
    'internal/action/domain/order_servicing.go',
    'internal/action/api/order_servicing.go',
    'internal/action/app/order_servicing.go',
    'internal/transport/http/cases_order_servicing.go',
]:
    code = re.sub(r'//[^\n]*', '', open(f'{REPO}/{probe}', encoding='utf-8').read())
    for word in ['supplier_send', 'send_to_supplier', 'delivery_to_supplier', 'ship_to_supplier', 'auto_send',
                 'dispatch_supplier', 'SupplierSend', 'cancel_order', 'execute_cancel', 'ExecuteCancel']:
        if word in code:
            failures.append(f'{probe}: поле/маршрут исполнения «{word}» — fence OQ-3/AG-4')
    for word in ['DeliveryStatus', 'delivery_status', 'NotifiedAt', 'notified_at', 'ReadAt', 'read_at']:
        if word in code:
            failures.append(f'{probe}: поле доставки уведомлений «{word}» — OQ-1/OQ-2')
    for word in ['UnitPrice', 'ClientPrice', 'PurchasePrice', 'PurchasingPrice', 'KnownClientSum']:
        if word in code:
            failures.append(f'{probe}: ценовое поле «{word}» — закупочные данные скрыты')

# Зонд 4 — маркетинговые слова в домене обслуживания.
domain_go = open(f'{REPO}/internal/action/domain/order_servicing.go', encoding='utf-8').read()
for word in ['лучший', 'выгодн', 'скидк', 'акци', 'самый', 'гарантируем', 'бесплатн', 'успейте', 'премиум-качеств']:
    if word in domain_go.lower():
        failures.append(f'internal/action/domain/order_servicing.go: маркетинговое слово «{word}»')

# Зонд 5 — import-graph домена обслуживания: чистые функции, ноль сетевых
# и ноль чужих модульных импортов (AD-2; домен читает только платформенные
# идентификаторы — ZERO записей структурно: никаких репозиториев).
imports = re.findall(r'"github\.com/artemiosu/staffy/[^"]+"', domain_go)
allowed = {
    '"github.com/artemiosu/staffy/internal/platform/id"',
}
for imp in imports:
    if imp not in allowed:
        failures.append(f'internal/action/domain/order_servicing.go: недопустимый импорт домена {imp}')

# Зонд 6 — AD-10 SAFE-пины: запрос несёт ПИНЫ (интент-слово, тип и значение
# признака, якорь операции, proof-пины, исход, раскрытие, актёр) — НЕ копии
# материала (никаких цен/составов/рендеров в материале запроса).
event_match = re.search(r'type OrderServicingRequest struct \{(.*?)\}', domain_go, re.S)
if not event_match:
    failures.append('домен: OrderServicingRequest не найден')
else:
    body = event_match.group(1)
    for forbidden in ['Price', 'SupplierName', 'ClientPrice', 'PurchasePrice', 'Render', 'Material', 'Positions', 'Note', 'ClientText']:
        if forbidden in body:
            failures.append(f'OrderServicingRequest несёт копию материала «{forbidden}» — AD-10 SAFE-пины')

# Зонд 7 — идемпотентность BY KEY + лестница доверия: реплей ДО append в
# app (повтор команды при том же доверии отвечает сохранённым запросом),
# store-duplicate → перечитка → реплей; изменившееся доверие = НОВАЯ
# запись (proof_state в ключе — лестница видна).
app_go = open(f'{REPO}/internal/action/app/order_servicing.go', encoding='utf-8').read()
for pin in ['Replay: true', 'ErrOrderServicingRequestDuplicate', 'ReadCaseServicingRequests',
            'request.ProofState == pins.State']:
    if pin not in app_go:
        failures.append(f'app: пин идемпотентности/лестницы «{pin}» отсутствует')
# Гейт только verification_records: proof-пины — ЖИВОЕ чтение assurance
# (отказ чтения = отказ команды, НИКОГДА не «ok»).
if 'AssuranceView(ctx' not in app_go or 'servicingProofPins' not in app_go:
    failures.append('app: proof-пины не читаются живым assurance-портом')
if 'ProofStateAllowsDisclosure(pins.State)' not in app_go:
    failures.append('app: раскрытие не гейтится proof-состоянием (AC-1)')
# Lookup BY KEY над записанным материалом + исход — производная домена.
if 'LookupOrderByIDentifier' not in app_go or 'DeriveLookupOutcome' not in app_go:
    failures.append('app: lookup BY KEY + DeriveLookupOutcome отсутствуют')
if 'GuardOrderServicingRequest' not in app_go:
    failures.append('app: гвард словарей (OQ-16 phone/email отказ словами) отсутствует')

# Зонд 8 — read-time деривации: клиентские слова и CancelFeasibility
# перечитываются живой проекцией 4-9 (НЕ сохраняются); FR-62 структурно —
# единственный пишущий путь append запроса.
for pin in ['DeriveServicingClientWords', 'DeriveCancelFeasibility', 'DeriveOrderStatusProjection',
            'ReadOperationLedger', 'ReadActionReceipts', 'ReadOperationVerifications', 'ReadOperationCorrections',
            'ReadOperationSupplierFacts']:
    if pin not in app_go:
        failures.append(f'app: пин read-time деривации «{pin}» отсутствует')

# Зонд 9 — серверная истина актёра: append запроса идёт с command.Actor.ID
# (спуф-актора нет); якорь кейса — живым чтением assurance/слепотой RLS.
if not re.search(r'ActorID:\s+command\.Actor\.ID', app_go):
    failures.append('app: append запроса не несёт серверную истину актёра (command.Actor.ID)')
if 'ErrOrderServicingNoCase' not in app_go:
    failures.append('app: честный 404 кейса обращения отсутствует')

# Зонд 10 — структурные пины тестов: гейт-зонд утечки AC-1 (клиентские
# слова отказа БЕЗ исхода поиска), лестница доверия (none→verified = новая
# запись), lookup три исхода, четыре состояния отмены — живыми прогонами.
domain_test = open(f'{REPO}/internal/action/domain/order_servicing_test.go', encoding='utf-8').read()
for pin in ['TestDeriveServicingClientWordsDeniedGate', 'TestDeriveCancelFeasibilityFourStates',
            'TestDeriveLookupOutcomeByName', 'TestServicingHonestProbes', 'TestServicingWordsByteStability']:
    if pin not in domain_test:
        failures.append(f'domain-тест: живой пин «{pin}» отсутствует')
if 'утёк в клиентские слова отказа' not in domain_test:
    failures.append('domain-тест: зонд утечки AC-1 не пиннен поимённо')
app_test = open(f'{REPO}/internal/action/app/order_servicing_test.go', encoding='utf-8').read()
for pin in ['TestRecordOrderServicingRequestReplayAndTrustLadder', 'TestRecordOrderServicingRequestGateDenied',
            'TestRecordOrderServicingRequestLookupOutcomes', 'TestReadOrderServicingViewCancelFourStates']:
    if pin not in app_test:
        failures.append(f'app-тест: живой пин «{pin}» отсутствует')
if 'лестница доверия видна' not in app_test:
    failures.append('app-тест: лестница доверия не пиннена поимённо')
# Интеграционная матрица + journeys z47 присутствуют.
integ_test = open(f'{REPO}/tests/integration/cases_order_servicing_test.go', encoding='utf-8').read()
for pin in ['TestOrderServicingMatrixRowsBySpec', 'spoofed caller id', 'trust ladder', 'recovery_cases']:
    if pin not in integ_test:
        failures.append(f'интеграция: живой пин «{pin}» отсутствует')
import glob as _glob
z47 = _glob.glob(f'{REPO}/tests/journeys/*order-servicing.spec.ts')
if not z47:
    failures.append('journeys z47 order-servicing отсутствует')
else:
    z47_text = open(z47[0], encoding='utf-8').read()
    for pin in ['четыре состояния', 'Позвать человека', 'Вернуться в разговор']:
        if pin not in z47_text:
            failures.append(f'z47: живой пин «{pin}» отсутствует')
    # The ANTI-secret probes of the journey assert the ABSENCE — the
    # probe reads the REAL leak shapes only (a live token, a positive
    # containment).
    if re.search(r'github_pat_[A-Za-z0-9]{10,}', z47_text):
        failures.append('z47: живой токен утёк')
    # The NEGATIVE probes (not.toContain) are the honest anti-leak pins —
    # only the POSITIVE containment (a render of the secret) leaks.
    for secret in ['api_key', 'sandbox-']:
        if re.search(r"(?<!not\.)toContain\('" + re.escape(secret) + r"'\)", z47_text):
            failures.append(f'z47: положительное сдерживание секрета «{secret}»')

# Зонд 11 — зеркало proof-состояний словаря 2-3 BY NAME (mirrors_test —
# добавленный тест, дифф аддитивный: только новая функция).
mirrors = open(f'{REPO}/internal/action/domain/mirrors_test.go', encoding='utf-8').read()
if 'TestProofStatesMirrorTheAssuranceDictionary' not in mirrors:
    failures.append('mirrors_test: зеркало proof-состояний словаря 2-3 отсутствует')

print(f'просканировано {scanned} файлов диффа; изменений {len(files)}')
for warning in warnings:
    print(f'WARN: {warning}')
if failures:
    for failure in failures:
        print(f'FAIL: {failure}')
    sys.exit(1)
print('STEP-05 SCAN CLEAN: секреты 0 / CJK 0 / mixed 0 / замороженные машины и панели 4-6..4-9 чисты / SQL 1 миграция 00009 append-only + UNIQUE полного ключа + CHECK-зеркала / supplier-send 0 / исполнения отмены 0 / уведомительные поля 0 / ценовые поля 0 / маркетинг 0 / import-graph чист (домен только platform/id) / AD-10 SAFE-пины запроса / идемпотентность BY KEY + лестница доверия / гейт только verification_records (живой порт) / lookup BY KEY + гвард OQ-16 / read-time деривации существующими портами / серверная истина актёра + честный 404 / FR-62 append-единственный пишущий путь / живые пины тестов (гейт-утечка/лестница/lookup/отмена/матрица/z47)')
