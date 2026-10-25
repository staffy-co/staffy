#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""qa_spec_scan_4_11.py — QA-скан спеки story 4-11 перед предъявлением:
секреты / CJK-артефакты / смешанные слова / структурная полнота /
front-matter / статус (жизненный цикл: backlog → ready-for-dev → done) / контекст-файлы существуют /
fence-слова (записи выручки НЕ заводятся OQ-4, baseline НЕ строится OQ-7, campaign НЕ добавляется
в envelope, usage НЕ атрибутируется на кейсы, минуты НЕ выводятся из lifecycle, экспорт НЕ строится,
срез read-only 0 миграций) / AC-покрытие всех 16 блоков story 4.11 / структурные зонды честности."""
import os
import re
import sys

REPO = '/home/z/my-project/staffy'
SPEC = os.path.join(REPO, '_bmad-output/implementation-artifacts/spec-4-11-pilot-business-outcome.md')

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
    (r'статуc\b', 'латинская c в «статуc»'),
    (r'FR-64', 'мусорная ссылка FR-64 (правильно FR-63)'),
    (r'FR-6[0-2]\b', 'мусорная ссылка FR-60..62 (правильно FR-63)'),
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
    # Жизненный цикл спеки (по прецеденту сканов 4-7..4-10: допустимы все три
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

# 6. Fence honesty probes: the spec must NOT authorize manual revenue
#    records, must NOT build baselines/comparison claims, must NOT add
#    campaign/referrer to the ingress envelope, must NOT attribute tenant
#    usage to cases, must NOT derive human minutes, must NOT build export.
if re.search(r'запис[ьи] (выручк|оплат|возврат)[а-я]*[^»\n]{0,60}менеджер[^»\n]{0,40}(завод|созда|добавл)', text, re.I):
    if not re.search(r'НЕ заводит[ья][^»\n]{0,5}записи выручки/оплаты/возврата менеджером', text, re.I):
        print('FAIL fence: формулировка заведения менеджерских записей выручки')
        fail += 1
if re.search(r'baseline[^»\n]{0,80}(стро|созд|утвержд)[а-я]*', text, re.I):
    if not re.search(r'baseline/когорты/пороги[^»\n]{0,80}НЕ стро', text, re.I) and \
       not re.search(r'НЕ строить baseline', text):
        print('FAIL fence: формулировка построения baseline (OQ-7 не пройден)')
        fail += 1
if re.search(r'campaign[^»\n]{0,60}(добавля|возвращает|восстанавлив)', text, re.I):
    if not re.search(r'campaign/refreferrer', text, re.I) and not re.search(r'НЕ восстанавливается догадкой', text):
        print('FAIL fence: формулировка восстановления campaign атрибуции')
        fail += 1
if re.search(r'usage[^»\n]{0,60}(атрибуц|привяз[^»\n]{0,20}к (кейсу|обращ))', text, re.I):
    if not re.search(r'usage-затраты[^»\n]{0,80}НЕ атрибутируются', text, re.I) and \
       not re.search(r'НЕ атрибутируются на обращения', text):
        print('FAIL fence: формулировка атрибуции usage на кейсы (1-11 связи нет)')
        fail += 1
if re.search(r'(человеческ|human) мину[^»\n]{0,60}(рассчит|вывод|вычисл)', text, re.I):
    if not re.search(r'минуты НЕ выводятся', text) and not re.search(r'НЕ выводить человеческие минуты', text):
        print('FAIL fence: формулировка вывода человеческих минут (утверждённого способа нет)')
        fail += 1
# Обязательные слова честности среза (регистронезависимо — заглавное НЕ тоже честное).
core_words = [
    (r'OQ-4', 'гейт OQ-4 (1С Owning System)'),
    (r'OQ-7', 'гейт OQ-7 (baseline/сравнение)'),
    (r'OQ-3', 'fence OQ-3 (cancel-исполнение)'),
    (r'AG-4', 'гейт AG-4 (runtime credentials)'),
    (r'AG-1', 'конверт AG-1 (объём пилота)'),
    (r'Measurement Dictionary', 'словарь измерений'),
    (r'unknown_pending', 'категория unknown/pending'),
    (r'размером выборки', 'выборка у чисел'),
    (r'период', 'период у чисел'),
    (r'корреляция — не причинность', 'корреляция ≠ причинность'),
    (r'append-only', 'append-only дисциплина'),
    (r'read-time', 'read-time деривация'),
    (r'0 миграций', 'read-only срез: 0 миграций'),
    (r'BY NAME', 'словари BY NAME'),
    (r'320px', 'a11y 320px'),
    (r'№215', 'deferred-work №215'),
    (r'не подставляется', 'нуль не подставляется'),
    (r'не восстанавливается догадкой', 'атрибуция не угадывается'),
]

# 7. AC coverage probes (all 16 story 4.11 AC blocks represented).
coverage_probes = [
    ('AC-1 атрибуция источника', 'атрибуц'),
    ('AC-1 campaign/referrer unknown', 'campaign/referrer'),
    ('AC-1 channel event ID', 'provider_event_id'),
    ('AC-2 цепочка исхода', 'цепочка исхода'),
    ('AC-2 не зависит от телефона', 'НЕ телефон'),
    ('AC-3 корреляция BY KEY', 'внешн'),
    ('AC-4 выручка Owning System', 'Owning System'),
    ('AC-4 заказ ≠ выручка', 'НЕ выручка'),
    ('AC-5 маржа неизвестна', 'марж'),
    ('AC-5 без среднерыночных', 'неизвестн'),
    ('AC-6 поздние факты перечитка', 'поздний факт'),
    ('AC-7 usage тарифная версия', 'usage'),
    ('AC-8 lifecycle минуты', 'человеческие минуты'),
    ('AC-8 ожидание vs работа', 'ожидание'),
    ('AC-9 словарь определений', 'numerator'),
    ('AC-9 окно зрелости', 'окно зрелости'),
    ('AC-10 десять стадий', 'inquiry_accepted'),
    ('AC-10 supplier_sent', 'supplier_sent'),
    ('AC-11 агрегаты пилота', 'объём обращений'),
    ('AC-11 счётчики коррекций/handoff/recovery', 'recovery_count'),
    ('AC-12 без заявления об улучшении', 'без заявления об улучшении'),
    ('AC-13 история не переписывается', 'append-only не тронута'),
    ('AC-14 доля неизвестных', 'unknown_outcome_share'),
    ('AC-15 VIN не в агрегатах', 'VIN'),
    ('AC-15 минимизация', 'структурный зонд'),
    ('AC-16 drill-down ссылками', 'drill-down'),
    ('AC-16 словарь на странице', 'рендерится на странице'),
    ('AC-17 двойной учёт невозможен', 'двойной учёт'),
    ('AC-17 eval-матрица', 'eval-матриц'),
    ('AC-18 a11y текстом', 'различимы БЕЗ опоры на цвет'),
]
for name, probe in coverage_probes:
    if not re.search(re.escape(probe), text, re.I):
        print(f'FAIL покрытие {name}: зонд «{probe}» не найден')
        fail += 1

for word, desc in core_words:
    if not re.search(word, text, re.I):
        print(f'FAIL ядро честности: нет слова «{desc}»')
        fail += 1

# 8. Structural honesty probes (the outcome is a read-time derivation,
#    unmeasurable metrics carry words not numbers, the slice writes nothing).
if 'DeriveCaseOutcome' not in text:
    print('FAIL структурный зонд: нет DeriveCaseOutcome')
    fail += 1
if 'DerivePilotReport' not in text:
    print('FAIL структурный зонд: нет DerivePilotReport')
    fail += 1
if 'PilotCaseSource' not in text:
    print('FAIL структурный зонд: нет порта PilotCaseSource')
    fail += 1
if 'pilot-outcome' not in text:
    print('FAIL структурный зонд: нет маршрута pilot-outcome')
    fail += 1
if re.search(r'revenue[:=]\s*[0-9]', text) or re.search(r'выручк[а-я]*[:=]\s*[0-9]', text):
    print('FAIL структурный зонд: числовое значение выручки в спеке (запрещено)')
    fail += 1
if 'view-скоуп' not in text and 'view-скоупа' not in text:
    print('FAIL структурный зонд: нет view-скоуп гейта поверхностей')
    fail += 1
if 'деградация' not in text:
    print('FAIL структурный зонд: нет честной деградации панелей')
    fail += 1
if 'конверсии' not in text:
    print('FAIL структурный зонд: нет конверсий между доказанными стадиями')
    fail += 1

print(f'---')
print(f'spec_scan_4_11: fail={fail} warn={warn} lines={lines}')
sys.exit(1 if fail else 0)
