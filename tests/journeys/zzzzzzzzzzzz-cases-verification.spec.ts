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

// The Story 2.3 chain at 320px: the assurance panel (the honest none →
// basic → verified), the binding gate with the allowed methods, the
// consent binding with the client confirmation text, the unified timeline
// of the bound conversation, the duplicate candidate card with the masked
// value, the separate resolution, the flash focus and the no-horizontal-
// scroll discipline.
test('cases-verification: the assurance panel, binding gate, channel binding, duplicate card at 320px', async ({ browser }) => {
  test.setTimeout(180_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. Two calls: the case A of the client and the case B of the same
    // phone (the duplicate material).
    const first = await postIngress(api, inboundEvent('journey-verify-evt-1', 'journey-verify-sess-1', { call: 'inbound-call', from: '+79000000077' }), 'journey-verify-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const second = await postIngress(api, inboundEvent('journey-verify-evt-2', 'journey-verify-sess-2', { call: 'inbound-call', from: '+79000000077' }), 'journey-verify-key-2');
    expect(second.status()).toBe(201);
    const caseB = (await second.json()).case_id;
    const pathA = `${casesPath}/${caseA}`;
    const pathB = `${casesPath}/${caseB}`;

    // 2. The assurance panel of the fresh case: the honest «не проверена»,
    // the allowed methods of the channel, the public path unblocked.
    await page.goto(pathA);
    const assurance = page.locator('section[aria-label="Уровень доверия обращения"]');
    await expect(assurance).toBeVisible();
    await expect(assurance.getByText('Личность:')).toBeVisible();
    await expect(assurance.getByText('не проверена').first()).toBeVisible();
    await expect(assurance.getByText('Проверок не было')).toBeVisible();
    await expect(assurance.getByText(/Разрешённые методы этого канала/)).toBeVisible();
    await expect(page.getByRole('heading', { name: /Work Control Card/ })).toBeVisible();

    // 3. The binding gate: the bind form without a verification flashes the
    // honest gate refusal (the required level + the methods of the
    // channel); the flash takes the focus.
    await page.locator('details:has(summary:text-is("Связать канал (по согласию клиента; требует verified)"))').click();
    const bindForm = page.locator('form[action$="/bindings"]').first();
    const targetConversation = await bindForm.locator('select[name="conversation_id"] option').first().getAttribute('value');
    await bindForm.locator('select[name="conversation_id"]').selectOption(targetConversation);
    await bindForm.locator('input[name="bound_case_id"]').fill(caseB);
    // The bound conversation of the case B: the conversation option of its
    // own verification form (the UUID value of the select).
    await page.goto(pathB);
    const boundDetail = page.locator('section[aria-label="Уровень доверия обращения"]');
    await expect(boundDetail).toBeVisible();
    const boundConversation = await page.locator('form[action$="/verifications"] select[name="conversation_id"] option').first().getAttribute('value');
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Связать канал (по согласию клиента; требует verified)"))').click();
    const bindFormRetry = page.locator('form[action$="/bindings"]').first();
    await bindFormRetry.locator('select[name="conversation_id"]').selectOption(targetConversation);
    await bindFormRetry.locator('input[name="bound_case_id"]').fill(caseB);
    await bindFormRetry.locator('input[name="bound_conversation_id"]').fill(boundConversation);
    await bindFormRetry.locator('input[name="purpose"]').fill('Клиент продолжит в WhatsApp');
    await bindFormRetry.locator('input[name="subject_handle"]').fill('позвонивший');
    await bindFormRetry.locator('input[name="note"]').fill('Клиент согласился продолжить в мессенджере');
    await bindFormRetry.locator('select[name="evidence_event_id"]').selectOption('');
    await bindFormRetry.getByRole('button', { name: /Связать/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}$`));
    const flash = page.locator('#case-flash');
    await expect(flash).toBeVisible();
    await expect(flash).toContainText('verified');
    await expect(flash).toBeFocused();

    // 4. The knowledge check passes: the panel shows basic (the ladder is
    // honest — the binding still requires verified).
    await page.locator('details:has(summary:text-is("Записать проверку (канонический исход, срок по методу)"))').click();
    const verifyForm = page.locator('form[action$="/verifications"]').first();
    const verifyConversation = await verifyForm.locator('select[name="conversation_id"] option').first().getAttribute('value');
    await verifyForm.locator('select[name="conversation_id"]').selectOption(verifyConversation);
    await verifyForm.locator('select[name="kind"]').selectOption('identity');
    await verifyForm.locator('select[name="method"]').selectOption('knowledge_check');
    await verifyForm.locator('select[name="provider_code"]').selectOption('ok');
    await verifyForm.locator('input[name="subject_handle"]').fill('позвонивший');
    await verifyForm.locator('input[name="note"]').fill('Клиент назвал модель и год автомобиля');
    await verifyForm.locator('select[name="evidence_event_id"]').selectOption('');
    await verifyForm.getByRole('button', { name: /Записать проверку/ }).click();
    await expect(page).toHaveURL(new RegExp('verification_recorded=1'));
    await expect(assurance.getByText('basic (проверка знанием)')).toBeVisible();

    // 5. The callback passes: the panel shows verified with the method, the
    // subject and the expiry; the binding succeeds.
    await page.locator('details:has(summary:text-is("Записать проверку (канонический исход, срок по методу)"))').click();
    const verifyForm2 = page.locator('form[action$="/verifications"]').first();
    await verifyForm2.locator('select[name="conversation_id"]').selectOption(verifyConversation);
    await verifyForm2.locator('select[name="kind"]').selectOption('identity');
    await verifyForm2.locator('select[name="method"]').selectOption('known_number_callback');
    await verifyForm2.locator('select[name="provider_code"]').selectOption('ok');
    await verifyForm2.locator('input[name="subject_handle"]').fill('позвонивший');
    await verifyForm2.locator('input[name="note"]').fill('Перезвонили на номер из обращения, абонент подтвердил детали');
    await verifyForm2.locator('select[name="evidence_event_id"]').selectOption('');
    await verifyForm2.getByRole('button', { name: /Записать проверку/ }).click();
    await expect(page).toHaveURL(new RegExp('verification_recorded=1'));
    await expect(assurance.getByText('verified (подтверждена)')).toBeVisible();
    await expect(assurance.getByText('known_number_callback').first()).toBeVisible();
    await expect(assurance.getByText('перезвонили на номер из обращения', { exact: false })).toBeVisible();

    // 6. The consent binding: the unified view joins the second
    // conversation; the client confirmation text appears without internal
    // identifiers.
    await page.locator('details:has(summary:text-is("Связать канал (по согласию клиента; требует verified)"))').click();
    const bindForm2 = page.locator('form[action$="/bindings"]').first();
    await bindForm2.locator('select[name="conversation_id"]').selectOption(targetConversation);
    await bindForm2.locator('input[name="bound_case_id"]').fill(caseB);
    await bindForm2.locator('input[name="bound_conversation_id"]').fill(boundConversation);
    await bindForm2.locator('input[name="purpose"]').fill('Клиент продолжит в WhatsApp');
    await bindForm2.locator('input[name="subject_handle"]').fill('позвонивший');
    await bindForm2.locator('input[name="note"]').fill('Клиент согласился продолжить в мессенджере');
    await bindForm2.locator('select[name="evidence_event_id"]').selectOption('');
    await bindForm2.getByRole('button', { name: /Связать/ }).click();
    await expect(page).toHaveURL(new RegExp('channel_bound=1'));
    await expect(page.getByText('Связывание каналов')).toBeVisible();
    await expect(page.getByText('согласие клиента (consent_handoff)')).toBeVisible();
    await expect(page.getByText(/Ваше обращение продолжается в этом канале/)).toBeVisible();
    await expect(page.getByRole('group', { name: 'Диалоги обращения' }).locator('tbody tr')).toHaveCount(2);
    await expect(page.getByRole('group', { name: 'Диалоги обращения' }).locator('tbody tr').nth(1).getByText('Связанный канал')).toBeVisible();
    await expect(page.getByRole('group', { name: 'Таймлайн событий' }).locator('tbody tr')).toHaveCount(2);

    // 7. The continuation event of the bound session: the unified timeline
    // grows without a new case.
    const continuation = await postIngress(api, inboundEvent('journey-verify-evt-3', 'journey-verify-sess-2', { call: 'inbound-call', from: '+79000000077' }), 'journey-verify-key-3');
    expect(continuation.status()).toBe(201);
    await page.goto(pathA);
    await expect(page.getByRole('group', { name: 'Таймлайн событий' }).locator('tbody tr')).toHaveCount(3);

    // 8. The duplicate candidates: the same phone fact in both cases (the
    // masked value only, no protected content).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Записать факт"))').click();
    const factFormA = page.locator('form[action$="/facts"]').first();
    const workItemA = await factFormA.locator('select[name="work_item_id"] option').first().getAttribute('value');
    await factFormA.locator('select[name="work_item_id"]').selectOption(workItemA);
    await factFormA.locator('input[name="material_key"]').fill('phone');
    await factFormA.locator('textarea[name="value"]').fill('"+7 (900) 000-00-77"');
    await factFormA.locator('select[name="source_type"]').selectOption('operator');
    await factFormA.getByRole('button', { name: /Записать факт/ }).click();
    await expect(page).toHaveURL(new RegExp('fact_recorded=1'));
    await page.goto(pathB);
    await page.locator('details:has(summary:text-is("Записать факт"))').click();
    const factFormB = page.locator('form[action$="/facts"]').first();
    const workItemB = await factFormB.locator('select[name="work_item_id"] option').first().getAttribute('value');
    await factFormB.locator('select[name="work_item_id"]').selectOption(workItemB);
    await factFormB.locator('input[name="material_key"]').fill('phone');
    await factFormB.locator('textarea[name="value"]').fill('"+7 (900) 000-00-77"');
    await factFormB.locator('select[name="source_type"]').selectOption('operator');
    await factFormB.getByRole('button', { name: /Записать факт/ }).click();
    await expect(page).toHaveURL(new RegExp('fact_recorded=1'));
    await expect(page.getByText('Возможные дубликаты')).toBeVisible();
    const duplicateCard = page.locator('section[aria-label^="Кандидат дубля"]').first();
    await expect(duplicateCard).toBeVisible();
    await expect(duplicateCard.getByText('phone')).toBeVisible();
    await expect(duplicateCard.getByText('•••••••••77')).toBeVisible();
    await expect(duplicateCard.getByText(/маска/)).toBeVisible();
    await expect(duplicateCard.getByText('exact_identifier')).toBeVisible();

    // 9. The separate resolution: the cases stay apart, the review closes.
    await duplicateCard.locator('details:has(summary:text-is("Решить дубль"))').click();
    const resolveForm = duplicateCard.locator('form[action$="/resolution"]');
    await resolveForm.locator('select[name="decision"]').selectOption('separate');
    await resolveForm.locator('input[name="note"]').fill('Телефон общий, обращения о разных услугах');
    await resolveForm.locator('input[name="subject_handle"]').fill('клиент по телефону');
    await resolveForm.locator('input[name="conversation_id"]').fill(targetConversation);
    await resolveForm.locator('input[name="bound_conversation_id"]').fill(boundConversation);
    await resolveForm.getByRole('button', { name: /Решить/ }).click();
    await expect(page).toHaveURL(new RegExp('duplicate_resolved=1'));
    await expect(page.getByText('разные клиенты — раздельны')).toBeVisible();
    await expect(page.getByRole('group', { name: 'Диалоги обращения' }).locator('tbody tr')).toHaveCount(1);

    // 10. The 320px discipline: no horizontal scroll on the whole chain.
    await page.goto(pathA);
    const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    expect(scrollWidth).toBeLessThanOrEqual(320);
    const touchTargets = await page.locator('button').evaluateAll((buttons) =>
      buttons.filter((button) => getComputedStyle(button).display !== 'none'),
    );
    expect(touchTargets.length).toBeGreaterThan(0);
  } finally {
    await page.close();
  }
});
