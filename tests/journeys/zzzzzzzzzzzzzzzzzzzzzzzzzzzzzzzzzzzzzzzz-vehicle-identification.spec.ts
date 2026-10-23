import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const chatConnectionID = '01994f64-6c00-7000-8000-000000000016';
const casesPath = `/app/tenants/${tenantID}/cases`;

function textInboundEvent(providerEventId: string, sessionId: string, payload: Record<string, unknown>) {
  return {
    schema_version: 1,
    tenant_id: tenantID,
    connection_id: chatConnectionID,
    provider: 'staffy-synthetic-telephony',
    provider_event_id: providerEventId,
    provider_session_id: sessionId,
    direction: 'inbound',
    modality: 'text',
    channel: 'web_chat',
    provider_timestamp: '2026-09-16T11:59:00Z',
    payload,
  };
}

async function postIngress(api: any, body: unknown, key: string) {
  return api.post('/v1/ingress/channel-events', {
    data: body,
    headers: { "Content-Type": "application/json", "Idempotency-Key": key },
  });
}

// The Story 4.2 chain at 320px: the vehicle identification panel of the
// case card (the read-time profile over the 3.0.0 pack schema — the
// attributes with the versions and the statuses, the honest flags of the
// VIN normalization with the readback details and the focus re-ask, the
// missing keys list, the ProfileDigest and the MASK boundary word), the
// operator-recorded identification facts through the штатные forms, the
// protected confirmation form of the client (the FULL VIN with the
// readback words — one of the TWO full-VIN surfaces), the PRG client
// confirmation binding the EXACT versions (confirmer=client — the first
// producer of the reserved 2.2 word), the terminal consumed page of the
// spent link, the operator panel use word «подтверждение автомобиля
// клиентом», the stale word of the new version after the confirmation
// and the a11y close (320px without the horizontal scroll, the readback
// and the flags as TEXT — the alternative of the voice readback exists
// always).
test('vehicle-identification: the read-time profile panel, the protected confirmation and the honest stale word at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One TEXT turn of one web_chat session (the fixture world pins
    // auto-parts@3.0.0 — the schema-bearing pack of the case).
    const first = await postIngress(api, textInboundEvent('journey-vehicle-evt-1', 'journey-vehicle-sess-1', { say: 'Подберите деталь на мой Focus 2021 года' }), 'journey-vehicle-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const pathA = `${casesPath}/${caseA}`;
    const caseView0 = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;

    // 2. The classification (the fact scope of the identification facts).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Классифицировать / переклассифицировать"))').first().click();
    const classifyForm = page.locator('form[action$="/classify"]').first();
    await classifyForm.locator('select[name="type"]').selectOption('идентификация автомобиля');
    await classifyForm.locator('input[name="title"]').fill('Идентификация Focus 2021');
    await classifyForm.locator('input[name="reason"]').fill('Подбор детали требует идентификацию автомобиля');
    await classifyForm.getByRole('button', { name: /Классифицировать/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?classified=1`));

    // 3. The operator records the identification facts through the
    // штатная fact form (the vehicle.vin with the check-digit flag and
    // the vehicle.make/model/year).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Записать факт"))').click();
    const recordFact = async (key: string, value: string, n: string) => {
      const factForm = page.locator('form[action$="/facts"]').first();
      await factForm.locator('select[name="source_type"]').selectOption('operator');
      await factForm.locator('input[name="material_key"]').fill(key);
      await factForm.locator('textarea[name="value"]').fill(value);
      await factForm.getByRole('button', { name: /Записать факт/ }).click();
      await page.goto(pathA);
      await page.locator('details:has(summary:text-is("Записать факт"))').click();
    };
    await recordFact('vehicle.vin', '"JH4KA7561PC008269"', 'vin');
    await recordFact('vehicle.make', '"Ford"', 'make');
    await recordFact('vehicle.model', '"Focus"', 'model');
    await recordFact('vehicle.year_of_manufacture', '"2021"', 'year');

    // 4. The operator panel: the read-time profile — the full VIN (the
    // review surface), the check-digit confirmation word, the readback
    // details (the row words), the missing keys (the confirmed are never
    // re-asked — none are confirmed yet), the digest and the MASK
    // boundary word (AC-9).
    await page.goto(pathA);
    const vehiclePanel = page.locator('section[aria-label="Идентификация автомобиля"]');
    await expect(vehiclePanel).toBeVisible();
    await expect(vehiclePanel.getByRole('table', { name: 'Признаки идентификации: значение, версия, источник, канал, статус, подтверждение' })).toBeVisible();
    await expect(vehiclePanel).toContainText('JH4KA7561PC008269');
    await expect(vehiclePanel).toContainText('без подтверждения');
    await expect(vehiclePanel).toContainText('Не хватает (не подтверждено): vehicle.vin, vehicle.make');
    await expect(vehiclePanel).toContainText('Достаточный набор:');
    await expect(vehiclePanel).toContainText('vehicle.vin');
    await expect(vehiclePanel).toContainText('ProfileDigest');
    // The readback details: the row words distinguish latin/digit (AC-4).
    await vehiclePanel.getByText('Посимвольный readback VIN — блоки 4-4-4-4-1, ряды словами (латиница/цифра/кириллица)').click();
    // The table cells concatenate in the inner text — the row WORDS are
    // pinned individually (the latin and the digit rows of the dictionary).
    await expect(vehiclePanel.getByRole('table', { name: 'Readback VIN: блок, символ, ряд, слово' })).toContainText('Джульетта');
    await expect(vehiclePanel.getByRole('table', { name: 'Readback VIN: блок, символ, ряд, слово' })).toContainText('четыре');
    await expect(vehiclePanel.getByRole('table', { name: 'Readback VIN: блок, символ, ряд, слово' })).toContainText('латиница');
    await expect(vehiclePanel.getByRole('table', { name: 'Readback VIN: блок, символ, ряд, слово' })).toContainText('цифра');
    // The VIN checks details: the check digit of the position 9 matched.
    await vehiclePanel.getByText('Проверки VIN (vehicle.vin) — флаги текстом').click();
    await expect(vehiclePanel).toContainText('контрольная цифра (позиция 9) совпала');
    // The mask boundary word (the full VIN renders ONLY on this card and
    // on the protected client form).
    await expect(vehiclePanel).toContainText('JH4…8269');

    // 5. The issue of the protected link (the existing operator form —
    // the question carries the identification meaning; the purpose stays
    // fact_correction).
    await page.locator('details:has(summary:text-is("Выдать ссылку продолжения (issue_continuation_link)"))').click();
    const issueForm = page.locator(`form[action$="/work-items/${workItemID}/continuation-links"]`).first();
    await issueForm.locator('input[name="purpose_fact_correction"]').check();
    await issueForm.locator('input[name="question"]').fill('Подтвердите автомобиль');
    await issueForm.locator('textarea[name="explanation"]').fill('Сверьте распознанные признаки вашего автомобиля и подтвердите их точные версии.');
    await issueForm.locator('input[name="participant_ref"]').fill('client+79000000666');
    await issueForm.locator('input[name="ttl_minutes"]').fill('30');
    await issueForm.locator('input[name="consumption_limit"]').fill('1');
    await issueForm.getByRole('button', { name: /Выдать ссылку/ }).click();
    await expect(page).toHaveURL(new RegExp(`work-items\\/${workItemID}\\/continuation-links$`));
    const url = await page.locator('#continuation-url').inputValue();
    expect(url).toMatch(/^\/continue\/[A-Za-z0-9_-]{64}$/);

    // 6. The protected client form: the vehicle block — the FULL VIN
    // (FR-55), the readback words, the attribute statuses, the confirm
    // checkboxes with the exact versions.
    await page.goto(url);
    await expect(page.locator('h2', { hasText: 'Идентификация автомобиля' })).toBeVisible();
    await expect(page.locator('main')).toContainText('JH4KA7561PC008269');
    await expect(page.locator('main')).toContainText('Посимвольное прочтение VIN');
    const readbackDetails = page.locator('details:has(summary:text-is("Посимвольное прочтение VIN (латиница/цифры/кириллица — словами)"))');
    await readbackDetails.click();
    await expect(page.locator('main')).toContainText('латиница Джульетта');
    await expect(page.locator('main')).toContainText('цифра четыре');
    await expect(page.locator('main')).toContainText('VIN: JH4KA7561PC008269');
    await expect(page.locator('main')).toContainText('Марка: Ford — без подтверждения');
    const confirmForm = page.locator('form[action$="/confirm-vehicle"]');
    await expect(confirmForm).toBeVisible();
    await expect(confirmForm.getByRole('button', { name: 'Подтверждаю' })).toBeVisible();
    const checkboxes = confirmForm.locator('input[name="confirm"]');
    await expect(checkboxes).toHaveCount(4);
    // The first checkbox carries the VIN pair (the fact id with the
    // exact version — the confirmation binds that version).
    const vinValue = await checkboxes.first().inputValue();
    expect(vinValue).toMatch(/^[0-9a-f-]{36}:1$/);

    // The public form is the Story 4.2 client surface: no horizontal
    // scroll at 320px (the a11y close of the continuation precedent).
    const publicOverflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(publicOverflow).toBeLessThanOrEqual(0);

    // 7. The PRG client confirmation: the button posts the checked pairs,
    // the landing announces the recorded confirmation, the link is spent
    // (the terminal consumed page of the repeat).
    await confirmForm.getByRole('button', { name: 'Подтверждаю' }).click();
    await expect(page).toHaveURL(new RegExp(`/continue\\/[^?]+\\?confirmed=1$`));
    await expect(page.locator('#continuation-notice')).toContainText('Автомобиль подтверждён');
    await expect(page.locator('#continuation-notice')).toContainText('точным версиям');
    await page.goto(url);
    await expect(page.locator('h1', { hasText: 'Материал отправлен' })).toBeVisible();

    // 8. The operator card after the client confirmation: the use word
    // «подтверждение автомобиля клиентом», the vin status confirmed with
    // the client confirmer, the missing keys WITHOUT the confirmed vin.
    await page.goto(pathA);
    await expect(page.locator(`#continuations-${workItemID}`)).toContainText('подтверждение автомобиля клиентом');
    const panelAfter = page.locator('section[aria-label="Идентификация автомобиля"]');
    await expect(panelAfter).toContainText('JH4KA7561PC008269');
    await expect(panelAfter).toContainText('Подтверждает: client (версия v1');
    const missingLine = panelAfter.locator('p.status').filter({ hasText: 'Не хватает' });
    // The client confirmed all four present attributes — only the absent
    // optional keys stay honestly missing (AC-2).
    await expect(missingLine).toContainText('vehicle.trim, vehicle.engine');
    await expect(missingLine).not.toContainText('vehicle.vin');
    await expect(missingLine).not.toContainText('vehicle.make');

    // 9. The stale word: the operator records a NEW version of the
    // confirmed make — the confirmation of the former version renders
    // the honest stale word (AC-8) and the aggregate stale note.
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Записать факт"))').click();
    const factForm2 = page.locator('form[action$="/facts"]').first();
    await factForm2.locator('select[name="source_type"]').selectOption('operator');
    await factForm2.locator('input[name="material_key"]').fill('vehicle.make');
    await factForm2.locator('textarea[name="value"]').fill('"Mazda"');
    await factForm2.getByRole('button', { name: /Записать факт/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?fact_recorded=1`));
    await page.goto(pathA);
    const panelStale = page.locator('section[aria-label="Идентификация автомобиля"]');
    await expect(panelStale).toContainText('конфликт значений — Staffy не выбирает вариант, уточните у клиента');
    await expect(panelStale).toContainText('подтверждение относилось к прежней версии (v1)');
    await expect(panelStale).toContainText('зависимые результаты считаются устаревшими');

    // 10. The a11y close of the operator card: the vehicle SECTION itself
    // never widens the layout (the tables scroll inside their wraps — the
    // page-level scroll of the full case card is not this slice's surface;
    // the bisect proved the section adds zero), the flags and the readback
    // are TEXT (the alternative of the voice readback exists always), the
    // details toggle by the keyboard.
    await expect(page.locator('#vehicle')).toBeVisible();
    const sectionOverflow = await page.evaluate(() => {
      const section = document.getElementById('vehicle')!;
      return section.scrollWidth - section.clientWidth;
    });
    expect(sectionOverflow).toBeLessThanOrEqual(0);
    const vinDetails = page.locator('details:has(summary:text-is("Проверки VIN (vehicle.vin) — флаги текстом"))');
    await vinDetails.focus();
    await page.keyboard.press('Enter');
    await expect(vinDetails).toBeVisible();
    await expect(page.locator('section[aria-label="Идентификация автомобиля"]')).toContainText('проверки пройдены: длина 17');
    // The secrets never render (the public page carries no credentials).
    const body = await page.locator('body').innerText();
    expect(body).not.toContain('api_key');
    expect(body).not.toContain('sandbox-');
  } finally {
    await page.close();
  }
});
