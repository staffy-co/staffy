#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_step05_4_11_scan.py — QA-саморевью закрытия story 4-11 (Dev шаг 05,
часть A) по живому диффу ветки (main...HEAD): read-time срез «Сквозной
бизнес-результат пилота» — 0 миграций / 0 пишущих путей нового контура /
замороженные машины 2-3/2-13/3-6/3-8/4-6..4-10 name-only (разрешённый
список диффа задокументирован ниже) / ценовые поля вью 0 + VIN/диалог/
телефон поля агрегатных вью 0 / import-graph домена только stdlib +
platform/id (AD-10) / секреты 0 + CJK/mixed 0 (гигиена, унаследована от
прецедента 4-10) / [skip ci]-дисциплина коммитов (отчёт)."""
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

failures = []
warnings = []

# ---------------------------------------------------------------------------
# Зонд 1 — MIGRATIONS ZERO: срез read-only, дифф ветки не содержит НИ ОДНОГО
# файла под migrations/ (спека Never: «НЕ записывать НИЧЕГО — зонд:
# SQL-дифф ветки = 0 файлов миграций»).
# ---------------------------------------------------------------------------
migrations_touched = [name for _, name in files if name.startswith('migrations/')]
if migrations_touched:
    failures.append(f'миграции в диффе: {migrations_touched} — срез read-only, 0 миграций')

# ---------------------------------------------------------------------------
# Зонд 2 — WRITE PATHS ZERO: новый контур (домен/порт/api/app/транспорт
# pilot_outcome) не вызывает НИ ОДНОГО пишущего метода — ни поимённые
# store/port-пути кодовой базы (Append*/Record*/Create*), ни родовые
# write-глаголы как вызовы методов; пилотный контур читает ТОЛЬКО Read*
# методы + чтения порта (положительные пины ниже).
# ---------------------------------------------------------------------------
new_contour = [
    'internal/action/domain/pilot_outcome.go',
    'internal/action/port/pilot_outcome.go',
    'internal/action/api/pilot_outcome.go',
    'internal/action/app/pilot_outcome.go',
    'internal/transport/http/cases_pilot_outcome.go',
]
store_write_methods = [
    # Именованные пишущие пути портов/сторов action-модуля (живой реестр).
    'AppendOrderSupplierFact', 'AppendOrderCorrection', 'AppendOrderCorrectionEvent',
    'AppendOrderVerification', 'AppendOrderVerificationEvent', 'AppendOrderServicingRequest',
    'AppendDispatchBlocked', 'AppendOutboxReceiptCreated', 'AppendConfirmationVersion',
    'AppendConfirmationEvent', 'CreateRecoveryCase', 'RecordDispatchOutcome',
    'RecordReconciliationVerdict', 'RecordProposalAttempt', 'RecordDryRunEvent',
    # Командные пишущие методы app-Service заказного семейства.
    'RecordOrderSupplierFormed', 'RecordOrderSupplierSent', 'RecordOrderCorrectionReread',
    'RecordOrderServicingRequest', 'RecordDryRun',
]
write_call = re.compile(r'\.(?:Append|Insert|Update|Create|Write|Delete|Save|Upsert|Record)\w*\(')
for name in new_contour:
    code = re.sub(r'//[^\n]*', '', open(f'{REPO}/{name}', encoding='utf-8').read())
    for method in store_write_methods:
        if method in code:
            failures.append(f'{name}: пишущий метод «{method}» — срез read-only')
    for match in write_call.finditer(code):
        failures.append(f'{name}: write-вызов «{match.group(0)}» — срез read-only')

# Положительные пины читающей поверхности app-контура (порт читает
# листинг/кейс, сторы — только Read*-семейство OrderStatusView-паттерна).
app_go = open(f'{REPO}/internal/action/app/pilot_outcome.go', encoding='utf-8').read()
for pin in ['s.pilotCases.CasesView(', 's.pilotCases.CaseView(',
            's.operations.ReadCaseOperations(', 's.dispatches.ReadActionReceipts(',
            's.verifications.ReadOperationVerifications(', 's.verifications.ReadVerificationEvents(',
            's.orderFacts.ReadOperationSupplierFacts(', 's.corrections.ReadOperationCorrections(',
            's.recoveries.ReadRecoveryCaseByOperation(']:
    if pin not in app_go:
        failures.append(f'app: пин читающего пути «{pin}» отсутствует')

# ---------------------------------------------------------------------------
# Зонд 3 — FROZEN MACHINES NAME-ONLY: замороженные машины 2-3/2-13/3-6/3-8/
# 4-6..4-10 (домены/приложения/сторы/порты/api + панели и их render-тесты)
# НЕ в диффе; дифф касается ТОЛЬКО разрешённого списка (новые файлы 4-11 +
# точки проводки + фикстуры/тесты + спека/sprint-status/DEVLOG/сканы).
# ---------------------------------------------------------------------------
frozen_machines = [
    # 2-3 — машина проверки личности (зонды читают её, не правят).
    'internal/cases/domain/verification.go',
    'internal/cases/domain/dictionaries.go',
    # 2-13 — машина безусловной передачи человеку.
    'internal/cases/domain/human_handoff.go',
    'internal/cases/app/human_handoff.go',
    # 3-6 — машина диспетчера.
    'internal/action/domain/dispatch.go',
    'internal/action/app/dispatch.go',
    'internal/action/store/dispatch_store.go',
    'internal/action/port/dispatch.go',
    'internal/action/api/dispatch.go',
    # 3-8 — машина recovery.
    'internal/action/domain/recovery.go',
    'internal/action/domain/recoverypolicy.go',
    'internal/action/app/recovery.go',
    'internal/action/store/recovery_store.go',
    'internal/action/port/recovery.go',
    'internal/action/api/recovery.go',
    # 4-6 — машина клиентского заказа.
    'internal/action/domain/order.go',
    'internal/action/domain/operation.go',
    'internal/action/domain/model.go',
    'internal/action/app/order.go',
    'internal/action/store/operation_store.go',
    'internal/action/port/order.go',
    # 4-7 — машина проверки заказа.
    'internal/action/domain/order_verification.go',
    'internal/action/app/order_verification.go',
    'internal/action/store/order_verification_store.go',
    'internal/action/port/order_verification.go',
    'internal/action/api/order_verification.go',
    # 4-8 — машина коррекции до закупки.
    'internal/action/domain/order_correction.go',
    'internal/action/app/order_correction.go',
    'internal/action/store/order_correction_store.go',
    'internal/action/port/order_correction.go',
    'internal/action/api/order_correction.go',
    # 4-9 — машина правдивого статуса заказа.
    'internal/action/domain/order_status.go',
    'internal/action/app/order_status.go',
    'internal/action/store/order_status_store.go',
    'internal/action/port/order_status.go',
    'internal/action/api/order_status.go',
    # 4-10 — машина безопасного обслуживания заказа (+ её миграция).
    'internal/action/domain/order_servicing.go',
    'internal/action/app/order_servicing.go',
    'internal/action/store/order_servicing_store.go',
    'internal/action/port/order_servicing.go',
    'internal/action/api/order_servicing.go',
    'migrations/action/00009_order_servicing_requests.sql',
    # Панели 4-6..4-10 и их render-тесты — не правятся.
    'internal/transport/http/cases_order.go',
    'internal/transport/http/cases_order_verification.go',
    'internal/transport/http/cases_order_correction.go',
    'internal/transport/http/cases_order_status.go',
    'internal/transport/http/cases_order_servicing.go',
    'internal/transport/http/cases_order_render_test.go',
    'internal/transport/http/cases_order_verification_render_test.go',
    'internal/transport/http/cases_order_correction_render_test.go',
    'internal/transport/http/cases_order_status_render_test.go',
    'internal/transport/http/cases_order_servicing_render_test.go',
]
for _, name in files:
    if name in frozen_machines:
        failures.append(f'замороженная машина/панель тронута: {name}')

# Разрешённый список диффа: новые файлы среза + точки проводки + фикстуры
# + тесты + документы/сканы. ВСЁ вне списка — отказ (name-only дисциплина).
allowed_diff = {
    # Новые файлы контура 4-11 (read-time срез).
    'internal/action/domain/pilot_outcome.go',
    'internal/action/domain/pilot_outcome_test.go',
    'internal/action/port/pilot_outcome.go',
    'internal/action/api/pilot_outcome.go',
    'internal/action/app/pilot_outcome.go',
    'internal/action/app/pilot_outcome_test.go',
    'internal/transport/http/cases_pilot_outcome.go',
    'internal/transport/http/cases_pilot_outcome_render_test.go',
    'web/templates/pilot_outcome.html',
    'tests/integration/cases_pilot_outcome_test.go',
    'tests/journeys/pilot_outcome_fixture.go',
    'tests/journeys/zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-pilot-outcome.spec.ts',
    # Точки проводки среза (ЕДИНСТВЕННЫЕ легаси-файлы в диффе).
    'internal/action/app/service.go',          # поле pilotCases + WithPilotOutcome
    'cmd/staffy-core/main.go',                 # composition-root адаптер
    'internal/transport/http/cases.go',        # вызов панели в caseDetailPage
    'internal/transport/http/cases_view.go',  # поле панели во вью страницы кейса
    'internal/transport/http/server.go',      # бинд + GET-маршрут отчёта
    'web/templates/case.html',                # хирургическая вставка секции
    'tests/integration/cases_order_test.go',    # интеграционная проводка WithPilotOutcome
    'tests/journeys/action_order_fixture.go',  # journeys-адаптер порта
    'tests/journeys/fixture_server.go',        # journeys-сид тоггл мира пилота
    # Документы и сканы истории.
    'DEVLOG.md',
    '_bmad-output/implementation-artifacts/spec-4-11-pilot-business-outcome.md',
    '_bmad-output/implementation-artifacts/sprint-status.yaml',
    'scripts/qa_spec_scan_4_11.py',
    'scripts/qa_step05_4_11_scan.py',
}
for _, name in files:
    if name not in allowed_diff:
        failures.append(f'файл вне разрешённого списка диффа: {name} — name-only нарушен')

# ---------------------------------------------------------------------------
# Зонд 4 — PRICE/VIN/PRIVACY PROBES: в новых view/domain-файлах НЕТ ценовых
# полей (закупочные данные за фенсом 4-9: price/amount/cost/revenue/margin
# как ИМЕНА полей структур; числовые поля с ценоподобными именами; живые
# ценовые поля кодовой базы UnitPrice/ClientPrice/PurchasePrice/…) и НЕТ
# VIN/диалог/телефон полей в агрегатных вью (AC-15: counts только).
# Исключение по конструкции: ReadCostWord — СТРОКОВОЕ слово стоимости
# чтения отчёта (честное слово Implementation Notes), НЕ денежное значение.
# ---------------------------------------------------------------------------
price_field_exact = {'Price', 'Amount', 'Cost', 'Revenue', 'Margin', 'Sum', 'Money',
                     'UnitPrice', 'ClientPrice', 'PurchasePrice', 'PurchasingPrice', 'KnownClientSum'}
price_like_name = re.compile(r'(?i)price|amount|cost|revenue|margin|sum|money')
numeric_type = re.compile(r'^(int|uint|float)')
vin_like = [re.compile(r'\bVIN\b'), re.compile(r'\bVin\b'), re.compile(r'\bPhone\b'),
            re.compile(r'\bPhoneNumber\b'), re.compile(r'\bDialogText\b'), re.compile(r'\bTranscript\b')]
for name in ['internal/action/domain/pilot_outcome.go',
             'internal/transport/http/cases_pilot_outcome.go']:
    text = open(f'{REPO}/{name}', encoding='utf-8').read()
    code = re.sub(r'//[^\n]*', '', text)
    for word in ['UnitPrice', 'ClientPrice', 'PurchasePrice', 'PurchasingPrice', 'KnownClientSum']:
        if word in code:
            failures.append(f'{name}: ценовое поле «{word}» — фенс 4-9')
    for pattern in vin_like:
        if pattern.search(code):
            failures.append(f'{name}: VIN/диалог/телефон поле «{pattern.pattern}» — AC-15')
    for match in re.finditer(r'type (\w+) struct \{(.*?)\n\}', code, re.S):
        struct_name, body = match.group(1), match.group(2)
        for line in body.splitlines():
            parts = line.strip().split()
            if len(parts) < 2 or parts[0] == '//':
                continue
            field, field_type = parts[0], ' '.join(parts[1:])
            if field in price_field_exact:
                failures.append(f'{name}: {struct_name}.{field} — ценовое поле (фенс 4-9)')
            elif numeric_type.match(field_type) and price_like_name.search(field):
                failures.append(f'{name}: {struct_name}.{field} {field_type} — числовое ценоподобное поле')

# Неизмеримые показатели словаря — строки-слова БЕЗ числовых полей
# (AC-4/AC-5/AC-7/AC-8: отсутствующие данные НЕ заменяются нулём).
domain_go = open(f'{REPO}/internal/action/domain/pilot_outcome.go', encoding='utf-8').read()
not_measurable = re.search(r'type PilotNotMeasurableIndicator struct \{(.*?)\n\}', domain_go, re.S)
if not not_measurable:
    failures.append('домен: PilotNotMeasurableIndicator не найден')
else:
    for line in not_measurable.group(1).splitlines():
        parts = line.strip().split()
        if len(parts) >= 2 and numeric_type.match(parts[1]):
            failures.append(f'PilotNotMeasurableIndicator.{parts[0]} {parts[1]} — числовое поле неизмеримого показателя')

# ---------------------------------------------------------------------------
# Зонд 5 — IMPORT GRAPH (AD-10): домен pilot_outcome импортирует ТОЛЬКО
# стандартную библиотеку + platform/id (чистые функции, ноль чужих
# модульных импортов — ZERO записей структурно).
# ---------------------------------------------------------------------------
import_block = re.search(r'import \((.*?)\)', domain_go, re.S)
if import_block:
    domain_imports = re.findall(r'"([^"]+)"', import_block.group(1))
else:
    domain_imports = re.findall(r'import "([^"]+)"', domain_go)
if not domain_imports:
    failures.append('домен: import-блок не найден')
for imp in domain_imports:
    if imp == 'github.com/staffy-co/staffy/internal/platform/id':
        continue
    if imp.startswith('github.com/staffy-co/staffy/'):
        failures.append(f'домен: чужой модульный импорт {imp} — AD-10')
    elif '/' in imp or '.' in imp.split('/')[0]:
        failures.append(f'домен: нестандартный импорт {imp} — AD-10')

# ---------------------------------------------------------------------------
# Зонд 6 — SECRETS ZERO (+ унаследованная гигиена CJK/mixed прецедента
# 4-10): добавленные строки диффа не несут секретоподобных значений
# (github_pat_/ghp_/AKIA/sk- живой длины); скрипты сканов и DEVLOG —
# нарративные поверхности — исключены (они несут САМИ слова зондов); z48-спека несёт
# только ОТРИЦАТЕЛЬНЫЕ anti-leak пины (not.toContain) — положительное
# сдерживание секрета было бы утечкой (прецедент z47).
# ---------------------------------------------------------------------------
patch = subprocess.run(
    ['git', 'diff', 'main...HEAD'],
    cwd=REPO, capture_output=True, text=True, check=True,
).stdout
secret_patterns = [
    (re.compile(r'github_pat_[A-Za-z0-9_]{20,}'), 'GitHub PAT'),
    (re.compile(r'ghp_[A-Za-z0-9]{30,}'), 'classic PAT'),
    (re.compile(r'AKIA[0-9A-Z]{16}'), 'AWS key'),
    (re.compile(r'sk-[A-Za-z0-9]{20,}'), 'sk- key'),
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
current_file = None
added_by_file = {}
for line in patch.splitlines():
    if line.startswith('+++ b/'):
        current_file = line[6:]
        added_by_file.setdefault(current_file, [])
    elif line.startswith('+') and not line.startswith('+++') and current_file:
        added_by_file[current_file].append(line[1:])
for name, added in added_by_file.items():
    if name.startswith('scripts/qa_') or name == 'DEVLOG.md':
        continue  # the narrative and the scans themselves carry the probe words
    joined = '\n'.join(added)
    for pattern, label in secret_patterns:
        if pattern.search(joined):
            failures.append(f'{name}: секрет {label}')
    for match in cjk_range.finditer(joined):
        failures.append(f'{name}: CJK-артефакт {match.group(0)}')
        break
    for pattern, label in mixed_patterns:
        if re.search(pattern, joined):
            failures.append(f'{name}: {label}')

# z48 anti-leak: только положительное сдерживание (рендер секрета) утекает.
z48_path = f'{REPO}/tests/journeys/zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-pilot-outcome.spec.ts'
z48_text = open(z48_path, encoding='utf-8').read()
for secret in ['api_key', 'sandbox-', 'github_pat_']:
    if re.search(r"(?<!not\.)toContain\('" + re.escape(secret) + r"'\)", z48_text):
        failures.append(f'z48: положительное сдерживание секрета «{secret}»')

# ---------------------------------------------------------------------------
# Зонд 7 — [skip ci]-ДИСЦИПЛИНА (ОТЧЁТ, не отказ): список коммитов ветки с
# маркерами; closure-коммит части Б (step-05 Б, feat БЕЗ маркера — CI ×3
# на CHECKPOINT 2) придёт ПОСЛЕ этого скана — ожидаемое состояние.
# ---------------------------------------------------------------------------
commit_log = subprocess.run(
    ['git', 'log', 'main..HEAD', '--format=%h %s'],
    cwd=REPO, capture_output=True, text=True, check=True,
).stdout.strip().splitlines()
with_marker = [line for line in commit_log if '[skip ci]' in line]
without_marker = [line for line in commit_log if '[skip ci]' not in line]
print(f'[skip ci]-дисциплина (отчёт): коммитов {len(commit_log)}, '
      f'с маркером {len(with_marker)}, без маркера {len(without_marker)}')
for line in commit_log:
    marker = 'есть' if '[skip ci]' in line else 'НЕТ'
    print(f'  [{marker}] {line}')
if without_marker:
    print('  замечание: коммиты без маркера — промежуточные SOS-чекпоинты умерших писателей '
          '(орфан-протокол DEV-фазы); closure-коммит части Б ожидается БЕЗ маркера по спеке')
else:
    print('  замечание: closure-коммит части Б (step-05 Б) придёт БЕЗ маркера — ожидаемое состояние')

print(f'просканировано веток диффа {len(files)} изменений; новых контурных файлов {len(new_contour)}')
for warning in warnings:
    print(f'WARN: {warning}')
if failures:
    for failure in failures:
        print(f'FAIL: {failure}')
    print(f'fail={len(failures)} warn={len(warnings)}')
    sys.exit(1)
print(f'fail={len(failures)} warn={len(warnings)}')
print('STEP-05 SCAN CLEAN: миграции 0 / пишущие пути нового контура 0 (именованнные store-пути + родовые write-глаголы; положительные пины Read*-семейства) / замороженные машины 2-3/2-13/3-6/3-8/4-6..4-10 name-only + дифф строго в разрешённом списке / ценовые поля 0 (имена полей + числовые ценоподобные + UnitPrice/ClientPrice/PurchasePrice/…) / VIN/диалог/телефон поля агрегатных вью 0 (AC-15) / неизмеримые показатели — слова без числовых полей (AC-4/5/7/8) / import-graph домена stdlib + platform/id (AD-10) / секреты 0 + CJK 0 + mixed 0 / z48 anti-leak только отрицательные пины / [skip ci]-дисциплина отчитана')
