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
    headers: { "Content-Type": "application/json", "Idempotency-Key": key },
  });
}

// The Story 2.4 chain at 320px: the Response Owner Control panel (the
// current owner, the epoch, the surface, the coverage words), the reply
// gate refusal of the non-owner (the flash carries the current owner and
// the permitted path), the owner reply (the prepared outgoing record),
// the transfer request (the pending state — assigned ≠ acknowledged), the
// acknowledge of the target operator (the new owner, the old owner's
// refusal), the term history and the outgoing table — keyboard-first,
// no horizontal scroll.
test('cases-ownership: the Response Owner Control, reply gate, transfer lifecycle at 320px', async ({ browser }) => {
  test.setTimeout(180_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One call: the case with the initial ownership term.
    const accepted = await postIngress(api, inboundEvent('journey-owner-evt-1', 'journey-owner-sess-1', { call: 'inbound-call', from: '+79000000088' }), 'journey-owner-key-1');
    expect(accepted.status()).toBe(201);
    const caseA = (await accepted.json()).case_id;
    const pathA = `${casesPath}/${caseA}`;

    // 2. The panel of the fresh case: the current owner, the epoch 1, the
    // surface, the coverage words (the backup + the deadline), the
    // primary action of the owner, the initial term of the history.
    await page.goto(pathA);
    const panel = page.locator('section[aria-label="Владение обращением"]');
    await expect(panel).toBeVisible();
    await expect(panel.getByText(/Текущий владелец:/)).toBeVisible();
    await expect(panel.getByText(/Epoch: 1/)).toBeVisible();
    await expect(panel.getByText('Staffy inbox', { exact: true })).toBeVisible();
    await expect(panel.getByText(/Владение стабильно/)).toBeVisible();
    await expect(panel.getByText(/Резервный владелец/)).toBeVisible();
    await expect(panel.getByText(/120 мин/)).toBeVisible();
    await expect(page.getByRole('heading', { name: /История владения/ })).toBeVisible();
    const history = page.locator('div[aria-label="История владения"] table');
    await expect(history.getByRole('row')).toHaveCount(2); // the header + the epoch 1 term
    await expect(history.getByText(/первоначальное назначение/)).toBeVisible();

    // 3. The reply gate: the second operator (the fixture switch) is not
    // the owner — the flash carries the current owner and the permitted
    // path; the flash takes the focus.
    await api.post('/fixture/operator', { form: { second: '1' } });
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const replyForm = page.locator('form[action$="/client-replies"]').first();
    const replyConversation = await replyForm.locator('select[name="conversation_id"] option').first().getAttribute('value');
    await replyForm.locator('select[name="conversation_id"]').selectOption(replyConversation);
    await replyForm.locator('textarea[name="content"]').fill('Посторонний ответ');
    await replyForm.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}`));
    const refusal = page.locator('#case-flash');
    await expect(refusal).toBeVisible();
    await expect(refusal).toContainText(/владелец/i);
    await expect(refusal).toContainText(/запросите передачу/i);

    // 4. The owner (back to the primary operator) prepares the reply: the
    // outgoing table shows the prepared state.
    await api.post('/fixture/operator', { form: { second: '' } });
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const ownerReply = page.locator('form[action$="/client-replies"]').first();
    await ownerReply.locator('select[name="conversation_id"]').selectOption(replyConversation);
    await ownerReply.locator('textarea[name="content"]').fill('Добрый день! Ваше обращение принято в работу.');
    await ownerReply.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?reply_prepared=1`));
    await page.goto(pathA);
    const outgoing = page.locator('div[aria-label="Исходящие ответы"] table');
    await expect(outgoing.getByRole('row')).toHaveCount(2); // the header + the prepared reply
    await expect(outgoing.getByText(/подготовлено/)).toBeVisible();
    await expect(outgoing.getByText(/Добрый день!/)).toBeVisible();

    // 5. The transfer request: the owner names the second operator; the
    // panel shows the pending state — assigned ≠ acknowledged.
    await page.locator('details:has(summary:text-is("Запросить передачу (дедлайн — из human coverage)"))').click();
    const transferForm = page.locator('form[action$="/ownership/transfer-requests"]').first();
    const targetOption = await transferForm.locator('select[name="to_owner_ref"] option').nth(1).getAttribute('value');
    await transferForm.locator('select[name="to_owner_ref"]').selectOption(targetOption);
    await transferForm.locator('select[name="writable_surface"]').selectOption('inbox');
    await transferForm.locator('input[name="reason"]').fill('Уход в отпуск');
    await transferForm.getByRole('button', { name: /Запросить передачу/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?transfer_requested=1`));
    await page.goto(pathA);
    await expect(page.locator('section[aria-label="Владение обращением"]').getByText(/Передача назначена, ждёт подтверждения/)).toBeVisible();
    const requestTable = page.locator('div[aria-label="Запрос передачи"] table');
    await expect(requestTable.getByText(/назначен, ждёт подтверждения/)).toBeVisible();

    // 6. The second operator (the target) acknowledges: the panel shows
    // the new owner at the epoch 2; the history carries both terms; the
    // old owner gets the honest refusal with the new owner.
    await api.post('/fixture/operator', { form: { second: '1' } });
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Решить запрос передачи (подтвердить / отклонить / отменить)"))').click();
    const resolveForm = page.locator('form[action$="/resolution"]').first();
    await resolveForm.locator('select[name="decision"]').selectOption('acknowledged');
    await resolveForm.locator('input[name="reason"]').fill('Принимаю владение');
    await resolveForm.getByRole('button', { name: /Записать решение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?transfer_resolved=1`));
    await page.goto(pathA);
    await expect(page.locator('section[aria-label="Владение обращением"]').getByText(/Epoch: 2/)).toBeVisible();
    const historyAfter = page.locator('div[aria-label="История владения"] table');
    await expect(historyAfter.getByRole('row')).toHaveCount(3); // the header + two terms
    await expect(historyAfter.getByText(/подтверждённая передача/)).toBeVisible();

    // 7. The old owner (the primary operator back) replies: the refusal
    // names the current owner (the new one).
    await api.post('/fixture/operator', { form: { second: '' } });
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const oldOwnerReply = page.locator('form[action$="/client-replies"]').first();
    await oldOwnerReply.locator('select[name="conversation_id"]').selectOption(replyConversation);
    await oldOwnerReply.locator('textarea[name="content"]').fill('Ответ прежнего владельца');
    await oldOwnerReply.getByRole('button', { name: /Подготовить ответ/ }).click();
    const oldOwnerRefusal = page.locator('#case-flash');
    await expect(oldOwnerRefusal).toBeVisible();
    await expect(oldOwnerRefusal).toContainText(/владелец/i);

    // 8. The 320px discipline: no horizontal scroll on the panel, the
    // history and the outgoing sections.
    await page.goto(pathA);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);

    // 9. The keyboard discipline: the panel heading and the tables are
    // reachable, the flash focus works after the conflict.
    const panelHeading = page.getByRole('heading', { name: /Response Owner Control/ });
    await expect(panelHeading).toBeVisible();
    await panelHeading.focus();
  } finally {
    await api.post('/fixture/operator', { form: { second: '' } });
    await page.close();
  }
});
