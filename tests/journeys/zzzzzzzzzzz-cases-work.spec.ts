import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const connectionID = '01994f64-6c00-7000-8000-000000000006';
const casesPath = `/app/tenants/${tenantID}/cases`;

function inboundEvent(providerEventId: string, sessionId: string, payload: Record<string, unknown>) {
  return {
    schema_version: 1,
    tenant_id: tenantID,
    connection_id: connectionID,
    provider: 'staffy-synthetic-telephony',
    provider_event_id: providerEventId,
    provider_session_id: sessionId,
    direction: 'inbound',
    modality: 'voice',
    channel: 'phone',
    provider_timestamp: '2026-09-16T11:59:00Z',
    payload,
  };
}

async function postIngress(api: any, body: unknown, key: string) {
  return api.post('/v1/ingress/channel-events', {
    data: body,
    headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key },
  });
}

test('cases-work: the Work Control Card chain at 320px — classify, facts, conflict, confirm, split, rescope, merge, dispute', async ({ browser }) => {
  test.setTimeout(180_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. The accepted call creates the case with the untriaged work item and
    // the auto-parts pack pin.
    const first = await postIngress(api, inboundEvent('journey-work-evt-1', 'journey-work-sess-1', { call: 'inbound-call', from: '+79000000007', model: 'Civic', year: 2020 }), 'journey-work-key-1');
    expect(first.status()).toBe(201);
    const accepted = await first.json();
    const casePath = `/app/tenants/${tenantID}/cases/${accepted.case_id}`;

    // 2. The Work Control Card of the untriaged item: the honest next
    // action, the classification form with the pack taxonomy.
    await page.goto(casePath);
    await expect(page.getByRole('heading', { name: /Обращение №\d+/ })).toBeVisible();
    const card = page.locator('section[aria-label^="Рабочий элемент"]').first();
    await expect(card).toBeVisible();
    await expect(card.getByText('Запрос ещё не классифицирован').first()).toBeVisible();
    await expect(card.getByText('Классифицируйте запрос')).toBeVisible();
    await expect(page.getByText('auto-parts@1.0.0').first()).toBeVisible();

    // 3. Classify through the form: the type from the pack taxonomy, the
    // title; the PRG returns with the version 2.
    await card.getByText('Классифицировать / переклассифицировать').click();
    const classifyForm = card.locator('form[action$="/classify"]');
    await classifyForm.locator('select[name="type"]').selectOption('поиск детали');
    await classifyForm.locator('input[name="title"]').fill('Подбор детали на Civic');
    await classifyForm.locator('input[name="reason"]').fill('Клиент ищет деталь');
    await classifyForm.getByRole('button', { name: /Классифицировать/ }).click();
    await expect(page).toHaveURL(new RegExp(`${accepted.case_id}\\?classified=1$`));
    await expect(page.getByRole('heading', { name: /Обращение №\d+/ })).toBeVisible();
    const classifiedCard = page.locator('section[aria-label^="Рабочий элемент"]').first();
    await expect(classifiedCard.getByText('В работе').first()).toBeVisible();
    await expect(classifiedCard.getByText('поиск детали').first()).toBeVisible();
    await expect(classifiedCard.getByText('Подбор детали на Civic').first()).toBeVisible();

    // 4. Record the fact from the event payload: the material key, the
    // JSON value, the channel_event source with the event id.
    await page.locator('details:has(summary:text-is("Записать факт"))').click();
    const factForm = page.locator('form[action$="/facts"]').first();
    // The material key hint comes from the CriticalEntities dictionary of
    // the PINNED pack (auto-parts), never a hardcoded list.
    await expect(factForm.locator('label[for="fact-key"]')).toContainText('деталь, артикул, автомобиль');
    const workItemValue = await factForm.locator('select[name="work_item_id"] option').first().getAttribute('value');
    const eventValue = await factForm.locator('select[name="source_id"] option').first().getAttribute('value');
    await factForm.locator('select[name="work_item_id"]').selectOption(workItemValue);
    await factForm.locator('input[name="material_key"]').fill('автомобиль');
    await factForm.locator('textarea[name="value"]').fill('{"model":"Civic","year":2020}');
    await factForm.locator('select[name="source_type"]').selectOption('channel_event');
    await factForm.locator('select[name="source_id"]').selectOption(eventValue);
    await factForm.getByRole('button', { name: /Записать факт/ }).click();
    await expect(page).toHaveURL(new RegExp(`${accepted.case_id}\\?fact_recorded=1$`));
    const factCard = page.locator('details:has(summary:text-is("автомобиль — Без подтверждения"))');
    await expect(factCard).toBeVisible();
    await factCard.locator("summary").first().click();
    await expect(factCard.locator('td code').first()).toContainText('Civic');
    await expect(factCard.getByText('phone').first()).toBeVisible();

    // 4b. The honest error path of the form: an invalid JSON value is the
    // rule refusal — the flash lands on the case page and TAKES THE FOCUS
    // (the frozen UI clause: the focus is predictable after an error).
    await page.locator('details:has(summary:text-is("Записать факт"))').click();
    const errorForm = page.locator('form[action$="/facts"]').first();
    await errorForm.locator('select[name="work_item_id"]').selectOption(workItemValue);
    await errorForm.locator('input[name="material_key"]').fill('деталь');
    await errorForm.locator('textarea[name="value"]').fill('not-a-json');
    await errorForm.locator('select[name="source_type"]').selectOption('operator');
    await errorForm.locator('select[name="source_id"]').selectOption('');
    await errorForm.getByRole('button', { name: /Записать факт/ }).click();
    await expect(page).toHaveURL(new RegExp(`${accepted.case_id}(\\?|$)`));
    await expect(page.locator('#case-flash')).toBeVisible();
    await expect(page.locator('#case-flash')).toBeFocused();

    // 5. The second event of the session brings another value: the field
    // conflicts, both versions with provenance stay visible.
    const second = await postIngress(api, inboundEvent('journey-work-evt-2', 'journey-work-sess-1', { call: 'inbound-call', note: 'год 2019' }), 'journey-work-key-2');
    expect(second.status()).toBe(201);
    expect((await second.json()).new_case).toBe(false);
    await page.goto(casePath);
    await page.locator('details:has(summary:text-is("Записать факт"))').click();
    const conflictForm = page.locator('form[action$="/facts"]').first();
    const secondEventValue = await conflictForm.locator('select[name="source_id"] option').nth(1).getAttribute('value');
    await conflictForm.locator('select[name="work_item_id"]').selectOption(workItemValue);
    await conflictForm.locator('input[name="material_key"]').fill('автомобиль');
    await conflictForm.locator('textarea[name="value"]').fill('{"model":"Civic","year":2019}');
    await conflictForm.locator('select[name="source_type"]').selectOption('channel_event');
    await conflictForm.locator('select[name="source_id"]').selectOption(secondEventValue);
    await conflictForm.getByRole('button', { name: /Записать факт/ }).click();
    const conflicting = page.locator('details:has(summary:text-is("автомобиль — Конфликт значений"))');
    await expect(conflicting).toBeVisible();
    await expect(conflicting.getByRole('row')).toHaveCount(3); // header + two versions
    await expect(conflicting.getByText('Разрешите конфликт')).toBeVisible();

    // 6. Confirm the exact latest version with the event evidence: the
    // conflict resolves, both histories stay.
    await conflicting.getByText('Подтвердить точную версию').first().click();
    const confirmForm = conflicting.locator('form[action$="/confirmations"]');
    await confirmForm.locator('select[name="version_no"]').selectOption('2');
    await confirmForm.getByRole('button', { name: /Подтвердить версию/ }).click();
    await expect(page).toHaveURL(new RegExp(`${accepted.case_id}\\?confirmed=1$`));
    const confirmed = page.locator('details:has(summary:text-is("автомобиль — Подтверждено"))');
    await expect(confirmed).toBeVisible();
    await confirmed.locator('summary').first().click();
    await expect(confirmed.getByRole('row')).toHaveCount(3); // the ladder history stays

    // 7. Split into two intentions: the source goes terminal split, the
    // targets are born in_work with the backward references.
    await page.goto(casePath);
    const splitCard = page.locator('section[aria-label^="Рабочий элемент"]').first();
    await splitCard.getByText('Разделить на два намерения').click();
    const splitForm = splitCard.locator('form[action$="/split"]');
    await splitForm.locator('select[name="target_1_type"]').selectOption('поиск детали');
    await splitForm.locator('input[name="target_1_title"]').fill('Деталь на Civic');
    await splitForm.locator('select[name="target_2_type"]').selectOption('уточнение аналога');
    await splitForm.locator('input[name="target_2_title"]').fill('Подбор аналога');
    await splitForm.getByRole('button', { name: /Разделить/ }).click();
    await expect(page).toHaveURL(new RegExp(`${accepted.case_id}\\?split=1$`));
    await expect(page.locator('section[aria-label^="Рабочий элемент"]')).toHaveCount(3);
    const splitSource = page.locator('section[aria-label^="Рабочий элемент"]').first();
    await expect(splitSource.getByText('Разделён').first()).toBeVisible();

    // 8. The fact stays on the split source; the explicit rescope moves it
    // onto the first target by a new version.
    const sourceFact = splitSource.locator('details:has(summary:text-is("автомобиль — Подтверждено"))');
    await expect(sourceFact).toBeVisible();
    await sourceFact.locator('summary').first().click();
    await sourceFact.getByText('Перенести факт').first().click();
    const rescopeForm = sourceFact.locator('form[action$="/rescope"]');
    const rescopeTarget = await rescopeForm.locator('select[name="target_work_item_id"] option').first().getAttribute('value');
    await rescopeForm.locator('select[name="target_work_item_id"]').selectOption(rescopeTarget);
    await rescopeForm.getByRole('button', { name: /Перенести/ }).click();
    await expect(page).toHaveURL(new RegExp(`${accepted.case_id}\\?rescoped=1$`));
    // The fact now lives on the first target card (the scope moved; the
    // rescoped version repeats the confirmed value — the status stays
    // confirmed, the ladder carries the move).
    const targetCard = page.locator('section[aria-label^="Рабочий элемент"]').nth(1);
    await expect(targetCard.locator('details:has(summary:text-is("автомобиль — Подтверждено"))')).toBeVisible();
    const splitSourceAfter = page.locator('section[aria-label^="Рабочий элемент"]').first();
    await expect(splitSourceAfter.locator('details:has(summary:text-is("автомобиль — Подтверждено"))')).toHaveCount(0);

    // 9. The second target merges into the first: the source goes merged
    // with merged_into; the dispute raises the exception for the human.
    await page.goto(casePath);
    const mergeTarget = page.locator('section[aria-label^="Рабочий элемент"]').nth(1);
    await mergeTarget.getByText('Объединить другой элемент сюда').click();
    const mergeForm = mergeTarget.locator('form[action$="/merge"]');
    const mergeSource = await mergeForm.locator('select[name="source"] option').first().getAttribute('value');
    await mergeForm.locator('select[name="source"]').selectOption(mergeSource);
    await mergeForm.getByRole('button', { name: /Объединить/ }).click();
    await expect(page).toHaveURL(new RegExp(`${accepted.case_id}\\?merged=1$`));
    await expect(page.getByText('Объединён').first()).toBeVisible();

    // 10. The dispute: the append-only exception visible on the case page.
    const disputeCard = page.locator('section[aria-label^="Рабочий элемент"]').nth(1);
    await disputeCard.getByText('Оспорить').first().click();
    const disputeForm = disputeCard.locator('form[action$="/dispute"]');
    await disputeForm.locator('input[name="note"]').fill('Объединение спорно');
    await disputeForm.getByRole('button', { name: /Оспорить/ }).click();
    await expect(page).toHaveURL(new RegExp(`${accepted.case_id}\\?disputed=1$`));
    await expect(page.getByText('Спор рабочего элемента').first()).toBeVisible();

    // 11. The horizontal scroll discipline at 320 px: the whole chain stays
    // without it.
    await expect(page.locator('main')).not.toHaveCSS('overflow-x', /scroll|auto/);
  } finally {
    await page.close();
  }
});
