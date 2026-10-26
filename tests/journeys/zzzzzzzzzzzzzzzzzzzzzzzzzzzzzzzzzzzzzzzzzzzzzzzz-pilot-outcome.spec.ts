import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const casesPath = `/app/tenants/${tenantID}/cases`;
const reportPath = `/app/tenants/${tenantID}/pilot-outcome`;
// The fixture clock month of the seeded world (the base moment
// 2026-09-16 12:00 UTC — the honest reset of the /fixture/clock below
// keeps the seeded month deterministic in the serial suite).
const seededPeriod = '2026-09';
// The seed names of the z48 journey (the order-family register: the
// UNIQUE provider event ids, the external ABCP order id and the supplier
// order id of the sent-after-formed ladder).
const seedEventID = 'journey-offers-evt-v48';
const freshEventID = 'journey-offers-evt-v48-fresh';
const externalOrderID = 'ABCP-ORDER-48';
const supplierOrderID = 'SUP-48';
const positionKey = 'WF2B196K666AA';
// The ten stage words of the outcome funnel (the dictionary render — the
// stage cell speaks the Russian word AND the id).
const stageWords: Array<[string, string]> = [
  ['Обращение принято', 'inquiry_accepted'],
  ['Профиль автомобиля подтверждён', 'vehicle_profile_confirmed'],
  ['Применимость доказана', 'applicability_proven'],
  ['Заказ клиента создан', 'client_order_created'],
  ['Проверка менеджером подтверждена', 'manager_verified'],
  ['Отправка поставщику записана', 'supplier_sent'],
  ['Исполнен', 'fulfilled'],
  ['Оплачен', 'paid'],
  ['Отменён', 'cancelled'],
  ['Возвращён', 'returned'],
];

// The Story 4.11 chain at 320px: the manager reads the «Исход обращения»
// panel of the FULL-JOURNEY case (the attribution of the first accepted
// inbound event with the honest unknown campaign/referrer words, the
// AC-2 outcome chain with the external order id, the ten-stage funnel
// with the state WORDS and the unknown pins OQ-4/OQ-3) and of the FRESH
// inquiry-only case (the honest not_reached ladder), then the «Отчёт
// пилота» page: the OQ-7 boundary word, the sample and the period riding
// every number, the funnel with the terminal «не измеряется» rows, the
// conversions between the PROVEN neighbours only, the non-measurable
// rows as WORDS («Выручка»/«Маржа»), the Measurement Dictionary v1 and
// the honest period filter (the seeded month counts both cases; a
// foreign month answers the honest zero word). The a11y close (captions,
// the 320px layout) and the secrets probe end the journey.
test('pilot-outcome: the case outcome panel and the pilot report from attribution to the honest period filter at 320px', async ({ browser }) => {
  test.setTimeout(360_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 0. The deterministic base moment (the family clock reset — the
    // seeded month is always the fixture base month) and the fixture
    // toggle that SEEDS the world of the z48 journey through the REAL
    // command paths server-side: the entitlement admission, the accepted
    // inbound event, the confirmed vehicle profile, the confirmed part
    // request, the sales mode, the proven fitment, the LIVE offer and the
    // client selection, the committed order receipt with the external
    // ABCP order id, the approved verification decision, the supplier
    // formed/sent facts — plus the SECOND fresh inquiry-only case.
    await api.post('/fixture/clock', { form: { minutes: '0' } });
    const seed = await api.post('/fixture/pilot-outcome-world', { form: { enabled: '1' } });
    expect(seed.status()).toBe(200);
    const anchors = await seed.json();
    const caseID: string = anchors.case_id;
    const freshCaseID: string = anchors.fresh_case_id;
    expect(caseID).toBeTruthy();
    expect(freshCaseID).toBeTruthy();
    const pathFull = `${casesPath}/${caseID}`;
    const pathFresh = `${casesPath}/${freshCaseID}`;

    // 1. The FULL-JOURNEY case: the panel «Исход обращения» renders after
    // the order servicing section (id="case-outcome" — the honest wired
    // contour word of 4-11).
    await page.goto(pathFull);
    const outcomeSection = page.locator('#case-outcome');
    await expect(page.getByRole('heading', { name: 'Исход обращения' })).toBeVisible();
    await expect(outcomeSection).toContainText('контур 4-11');
    await expect(outcomeSection).toContainText('чистая read-time деривация');
    await expect(outcomeSection).toContainText('терминальные стадии — unknown_pending всегда');

    // 2. The ATTRIBUTION of the first accepted inbound channel event:
    // only what the source carried (provider, channel, the event id);
    // the campaign/referrer stay the honest unknown words — never
    // restored by a guess (AC-1).
    await expect(outcomeSection).toContainText('источник обращения: провайдер staffy-synthetic-telephony');
    await expect(outcomeSection).toContainText('канал web_chat');
    await expect(outcomeSection).toContainText(`ID события провайдера ${seedEventID}`);
    await expect(outcomeSection).toContainText('кампания: источник не передал — не восстанавливается догадкой');
    await expect(outcomeSection).toContainText('реферер: источник не передал — не восстанавливается догадкой');

    // 3. The OUTCOME CHAIN (the AC-2 graph): the source word, the dialog,
    // the case, the work item, the confirmed vehicle profile, the part
    // request, the proven applicability, the operation and the EXTERNAL
    // ORDER ID of the committed receipt.
    await expect(page.getByRole('heading', { name: 'Цепочка исхода' })).toBeVisible();
    await expect(outcomeSection).toContainText('источник: источник обращения: провайдер staffy-synthetic-telephony');
    await expect(outcomeSection).toContainText('диалог: 1 принятых канальных событий');
    await expect(outcomeSection).toContainText('кейс: создан (номер ');
    await expect(outcomeSection).toContainText('рабочий элемент: 1');
    await expect(outcomeSection).toContainText('профиль автомобиля: подтверждён (vehicle.vin, vehicle.make, vehicle.model, vehicle.year_of_manufacture)');
    await expect(outcomeSection).toContainText('запрос детали: подтверждён (контур 4-3)');
    await expect(outcomeSection).toContainText(`подтверждение применимости: доказано (позиция ${positionKey})`);
    await expect(outcomeSection).toContainText('операция заказа: создана');
    await expect(outcomeSection).toContainText(`внешний ID заказа: ${externalOrderID}`);

    // 4. The TEN-STAGE FUNNEL: every stage renders its dictionary WORD
    // and its id (the text, never a bare id); the six measurable stages
    // are proven with the evidence words BY NAME (the external order id
    // of the receipt, the supplier order id of the sent fact).
    for (const [word, id] of stageWords) {
      await expect(outcomeSection).toContainText(`${word} (${id})`);
    }
    await expect(outcomeSection).toContainText(`(внешний ID ${externalOrderID})`);
    await expect(outcomeSection).toContainText(`(заказ поставщика ${supplierOrderID})`);
    await expect(outcomeSection).toContainText('обращение принято: кейс создан первым принятым канальным событием');
    await expect(outcomeSection).toContainText('профиль автомобиля подтверждён: атрибуты идентификации подтверждены');
    await expect(outcomeSection).toContainText('применимость доказана: лестница доказательств контура 4-4');
    await expect(outcomeSection).toContainText('заказ клиента создан: операция с подтверждённой квитанцией и внешним ID');
    await expect(outcomeSection).toContainText('проверка менеджером подтверждена: последнее закрытое решение — «Подтвердить проверку»');
    await expect(outcomeSection).toContainText('отправка поставщику записана: запись менеджера с номером заказа поставщика');

    // 5. The STATE WORDS as text (AC-18 — never a color only) and the
    // evidence times/sources of the proven ladder.
    await expect(outcomeSection).toContainText('доказано (proven)');
    await expect(outcomeSection).toContainText('неизвестно/ожидает (unknown_pending)');
    await expect(outcomeSection).toContainText('канальное событие (принятое inbound)');
    await expect(outcomeSection).toContainText('подтверждение атрибута идентификации (контур 4-2)');
    await expect(outcomeSection).toContainText('запись доказательства применимости (контур 4-4)');
    await expect(outcomeSection).toContainText('квитанция операции заказа (контур 4-6)');
    await expect(outcomeSection).toContainText('закрытое решение проверки (контур 4-7)');
    await expect(outcomeSection).toContainText('запись факта поставщика менеджером (контур 4-9)');
    await expect(outcomeSection).toContainText('16.09.2026 12:00:00 UTC');

    // 6. The UNKNOWN PINS of the four terminal stages (OQ-4/OQ-3/№215 —
    // «не доказано и не опровергнуто», never zero and never a silent
    // exclusion, AC-14).
    await expect(outcomeSection).toContainText('исполнение заказа — владеющий источник вне контура (1С/исполнение поставщика, OQ-4/AG-4): не доказано и не опровергнуто');
    await expect(outcomeSection).toContainText('оплата — владеющий источник вне контура (1С, OQ-4): не доказано и не опровергнуто');
    await expect(outcomeSection).toContainText('исполнение отмены не существует в контуре (OQ-3 fence + отложенная работа №215; CancelFeasibility 4-10 — только запрос): не доказано и не опровергнуто');
    await expect(outcomeSection).toContainText('материала возврата нет (претензии — контур 3-8 словами): не доказано и не опровергнуто');

    // 7. The FRESH inquiry-only case: the honest not_reached ladder —
    // the attribution of its own event and NOTHING proven beyond the
    // accepted inquiry (the interface truth of the absence).
    await page.goto(pathFresh);
    const freshSection = page.locator('#case-outcome');
    await expect(page.getByRole('heading', { name: 'Исход обращения' })).toBeVisible();
    await expect(freshSection).toContainText(`ID события провайдера ${freshEventID}`);
    await expect(freshSection).toContainText('кампания: источник не передал — не восстанавливается догадкой');
    await expect(freshSection).toContainText('профиль автомобиля: не подтверждён');
    await expect(freshSection).toContainText('подтверждение применимости: не доказано');
    await expect(freshSection).toContainText('операция заказа: нет');
    await expect(freshSection).toContainText('внешний ID заказа: нет');
    await expect(freshSection).toContainText('материал идентификации отсутствует — обращение не дошло до подтверждения профиля');
    await expect(freshSection).toContainText('применимость не проверена: доказательств контура 4-4 нет');
    await expect(freshSection).toContainText('операции заказа в кейсе нет');
    await expect(freshSection).toContainText('подтверждённого решения проверки нет');
    await expect(freshSection).toContainText('записей об отправке поставщику нет');
    await expect(freshSection).toContainText('не достигнуто (not_reached)');
    await expect(freshSection).toContainText('неизвестно/ожидает (unknown_pending)');

    // 8. The «Отчёт пилота» page of the seeded month: the OQ-7 boundary
    // word (AC-12), the sample and the period riding every number
    // (AC-11), the funnel with the terminal «не измеряется» rows, the
    // conversions between the PROVEN neighbours, the counters, the
    // unknown share, the non-measurable rows as WORDS and the
    // Measurement Dictionary v1 (AC-9).
    await page.goto(`${reportPath}?period=${seededPeriod}`);
    const reportMain = page.locator('#main');
    await expect(page.getByRole('heading', { name: 'Отчёт пилота автозапчастей' })).toBeVisible();
    await expect(reportMain).toContainText('чистая read-time деривация');
    await expect(reportMain).toContainText('утверждённых baseline/когорт/порогов НЕТ — наблюдаемые показатели без заявления об улучшении; корреляция — не причинность');
    await expect(reportMain).toContainText('Ваши полномочия на этой странице: review — чтение исхода обращения и отчёта пилота');
    const reportText = await reportMain.innerText();
    const sampleMatch = reportText.match(/Выборка:\s*(\d+)/);
    expect(sampleMatch).not.toBeNull();
    const sample = Number(sampleMatch![1]);
    // The honest zero word NEVER rides the seeded month — both seeded
    // cases (the full-journey case and the fresh inquiry-only case) are
    // counted: the sample is at least two.
    expect(sample).toBeGreaterThanOrEqual(2);
    await expect(reportMain).toContainText(`; период: ${seededPeriod}.`);
    await expect(reportMain).not.toContainText('кейсов периода нет');

    // 8a. The funnel table: the ten dictionary rows; the measurable
    // counts (the inquiry count equals the whole sample — every case of
    // the period was accepted; the supplier_sent count proves the
    // full-journey case reached the end of the measurable ladder); the
    // four terminal rows speak «не измеряется» — NEVER a number.
    const funnelRows = page.locator('#main table').first().locator('tbody tr');
    await expect(funnelRows).toHaveCount(10);
    for (const [word, id] of stageWords) {
      await expect(page.locator('#main table').first()).toContainText(`${word} (${id})`);
    }
    const inquiryCount = Number(await funnelRows.nth(0).locator('td').last().innerText());
    expect(inquiryCount).toBe(sample);
    const supplierCount = Number(await funnelRows.nth(5).locator('td').last().innerText());
    expect(supplierCount).toBeGreaterThanOrEqual(1);
    for (const index of [6, 7, 8, 9]) {
      await expect(funnelRows.nth(index).locator('td').last()).toHaveText('не измеряется');
    }
    await expect(page.locator('#main table').first().locator('caption')).toContainText('Воронка десяти стадий: доказанные counts (unknown_pending-стадии в знаменатели конверсий не входят)');

    // 8b. The conversions table: only between the PROVEN neighbours —
    // the five adjacent pairs of the measurable ladder, the first
    // denominator equals the whole sample (every case reached the
    // inquiry), the first numerator is at least one (the full-journey
    // case reached the confirmed profile).
    const conversionRows = page.locator('#main table').nth(1).locator('tbody tr');
    await expect(conversionRows).toHaveCount(5);
    await expect(conversionRows.nth(0)).toContainText('inquiry_accepted');
    await expect(conversionRows.nth(0)).toContainText('vehicle_profile_confirmed');
    const firstDenominator = Number(await conversionRows.nth(0).locator('td').nth(2).innerText());
    expect(firstDenominator).toBe(sample);
    const firstNumerator = Number(await conversionRows.nth(0).locator('td').nth(1).innerText());
    expect(firstNumerator).toBeGreaterThanOrEqual(1);
    await expect(conversionRows.nth(4)).toContainText('manager_verified');
    await expect(conversionRows.nth(4)).toContainText('supplier_sent');
    await expect(page.locator('#main table').nth(1).locator('caption')).toContainText('Конверсии между ДОКАЗАННЫМИ соседними стадиями (numerator/denominator BY NAME)');

    // 8c. The counters of the contours and the unknown share (AC-14 —
    // the influence of the unknown is visible, never hidden): every
    // counter rides the sample and the period (AC-11); the share is the
    // whole sample honestly (the terminal outcomes are never measurable
    // in the contour).
    const counterWords = ['Коррекции (контур 4-8)', 'Передачи человеку (контур 2-13)', 'Recovery-кейсы (контур 3-8)'];
    for (const counter of counterWords) {
      expect(reportText).toMatch(new RegExp(`${counter.replace(/[()]/g, '\\$&')}: \\d+ — выборка: ${sample}; период: ${seededPeriod}`));
    }
    await expect(reportMain).toContainText(`Доля неизвестных исходов: ${sample} из ${sample} — терминальные исходы не измеримы в контуре — достоверность результата ограничена`);

    // 8d. The NON-MEASURABLE rows as WORDS of reasons — «Выручка» and
    // «Маржа» among them; the numbers are NEVER substituted for the
    // missing components (AC-4/AC-5/AC-7/AC-8).
    await expect(page.getByRole('heading', { name: 'Неизмеримые показатели' })).toBeVisible();
    await expect(page.locator('#main table').nth(2)).toContainText('Выручка');
    await expect(page.locator('#main table').nth(2)).toContainText('Маржа');
    await expect(page.locator('#main table').nth(2)).toContainText('Переменные расходы');
    await expect(page.locator('#main table').nth(2)).toContainText('Человеческие минуты');
    await expect(page.locator('#main table').nth(2)).toContainText('владеющий источник — Owning System 1С — не подключён (OQ-4); созданный клиентский заказ НЕ выручка (AC-4)');
    await expect(page.locator('#main table').nth(2)).toContainText('маржа «неизвестна» — НЕ 0');
    await expect(page.locator('#main table').nth(2).locator('caption')).toContainText('Неизмеримые показатели — строками причин, чисел нет (отсутствующая составляющая не заменяется нулём)');

    // 8e. The Measurement Dictionary v1 (AC-9): the case_volume indicator
    // with the version word v1 and the owner word of the spec approval.
    await expect(page.getByRole('heading', { name: 'Словарь измерений v1' })).toBeVisible();
    const dictionaryTable = page.locator('#main table').nth(3);
    await expect(dictionaryTable).toContainText('case_volume');
    await expect(dictionaryTable).toContainText('Объём обращений');
    await expect(dictionaryTable).toContainText('v1');
    await expect(dictionaryTable).toContainText('словарь утверждается одобрением спеки (CHECKPOINT 1)');
    await expect(dictionaryTable.locator('caption')).toContainText('Словарь измерений v1 (утверждается одобрением спеки; смена определения — новая версия, не молча)');
    await expect(reportMain).toContainText('Отчёт перечитывает листинг кейсов периода с по-кейсовыми чтениями (объём пилота мал — конверт AG-1)');

    // 9. The honest PERIOD FILTER: a foreign month answers the honest
    // zero word — the zero is allowed ONLY as the case count (AC-11),
    // the measurable rows stay the honest zero counts, the terminal
    // rows stay «не измеряется» and the conversions table carries NO
    // rows (nothing is divided).
    await page.goto(`${reportPath}?period=2020-01`);
    const emptyMain = page.locator('#main');
    await expect(page.getByRole('heading', { name: 'Отчёт пилота автозапчастей' })).toBeVisible();
    await expect(emptyMain).toContainText('Выборка: 0');
    await expect(emptyMain).toContainText('период: 2020-01.');
    await expect(emptyMain).toContainText('кейсов периода нет: честная нулевая выборка (ноль допустим только как счёт кейсов)');
    const emptyFunnelRows = page.locator('#main table').first().locator('tbody tr');
    await expect(emptyFunnelRows).toHaveCount(10);
    for (const index of [6, 7, 8, 9]) {
      await expect(emptyFunnelRows.nth(index).locator('td').last()).toHaveText('не измеряется');
    }
    await expect(page.locator('#main table').nth(1).locator('tbody tr')).toHaveCount(0);
    await expect(emptyMain).toContainText('Доля неизвестных исходов: 0 из 0 — терминальные исходы не измеримы в контуре — достоверность результата ограничена');

    // 10. The absent period is the whole observation period (the period
    // word rides the report — AC-11).
    await page.goto(reportPath);
    await expect(page.locator('#main')).toContainText('Выборка: ');
    await expect(page.locator('#main')).toContainText('весь период наблюдения.');

    // 11. The a11y close: the panel and the report never widen the
    // layout (the tables scroll inside their wraps at 320px — the
    // family precedent of the z43..z47 close); the table captions are
    // the honest text alternatives of the data tables.
    await page.goto(pathFull);
    await expect(page.locator('#case-outcome table caption')).toContainText('Десять стадий воронки обращения: состояние словом, доказательство, время, источник, пин неизвестности');
    const panelOverflow = await page.evaluate(() => {
      const section = document.getElementById('case-outcome');
      return section ? [section.scrollWidth - section.clientWidth] : [];
    });
    for (const overflow of panelOverflow) {
      expect(overflow).toBeLessThanOrEqual(0);
    }
    await page.goto(`${reportPath}?period=${seededPeriod}`);
    const reportOverflow = await page.evaluate(() => {
      const main = document.getElementById('main');
      return main ? [main.scrollWidth - main.clientWidth] : [];
    });
    for (const overflow of reportOverflow) {
      expect(overflow).toBeLessThanOrEqual(0);
    }

    // 12. The secrets never render (the manager pages carry no
    // credentials).
    const caseBody = await (async () => { await page.goto(pathFull); return page.locator('body').innerText(); })();
    expect(caseBody).not.toContain('api_key');
    expect(caseBody).not.toContain('sandbox-');
    expect(caseBody).not.toContain('github_pat_');
    const reportBody = await (async () => { await page.goto(`${reportPath}?period=${seededPeriod}`); return page.locator('body').innerText(); })();
    expect(reportBody).not.toContain('api_key');
    expect(reportBody).not.toContain('sandbox-');
    expect(reportBody).not.toContain('github_pat_');
  } finally {
    await api.post('/fixture/pilot-outcome-world', { form: { enabled: '0' } }).catch(() => {});
    await page.close();
  }
});
