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

// The Story 2.5 chain at 320px: the mode badge (the active mode + the AI
// role of the owner, text only), the initial unclassified word of the
// admission, the classification proposal (the raise from the admission
// floor), the disclosure gate of the sales reply (the record + the sent
// event open it), the safety raise (the turn signal), the forbidden sales
// word with the exit path, the downgrade refusal, the supervised exit,
// the interruption and the re-disclosure with the parent reference —
// keyboard-first, aria-live without a focus move, no horizontal scroll.
test('cases-mode: the ladder, the disclosure gate, the exit and the re-disclosure at 320px', async ({ browser }) => {
  test.setTimeout(180_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One call: the case with the initial unclassified mode term.
    const accepted = await postIngress(api, inboundEvent('journey-mode-evt-1', 'journey-mode-sess-1', { call: 'inbound-call', from: '+79000000077' }), 'journey-mode-key-1');
    expect(accepted.status()).toBe(201);
    const caseA = (await accepted.json()).case_id;
    const pathA = `${casesPath}/${caseA}`;

    // 2. The fresh case: the badge shows the unclassified word and the
    // human role as text (never a color); the sales replies are
    // impossible; the mode history carries the admission term.
    await page.goto(pathA);
    const badge = page.locator('#mode-badge');
    await expect(badge).toBeVisible();
    await expect(badge).toContainText('не классифицирован');
    await expect(badge).toContainText('участник-человек');
    const modePanel = page.locator('section[aria-label="Режим диалога"]');
    await expect(modePanel).toBeVisible();
    await expect(modePanel.getByText(/Продажные ответы недоступны/)).toBeVisible();
    await expect(page.getByRole('heading', { name: /История режимов/ })).toBeVisible();
    const modeHistory = page.locator('div[aria-label="История режимов"] table');
    await expect(modeHistory.getByRole('row')).toHaveCount(2); // the header + the admission term
    await expect(modeHistory.getByText(/первоначальное слово/)).toBeVisible();
    await expect(modeHistory.getByText(/unclassified/)).toBeVisible();

    // 3. The classification proposal: the raise from the admission floor
    // opens the sales path; the badge announces via aria-live (the polite
    // word, no focus move).
    await page.locator('details:has(summary:text-is("Предложить режим (сигнал — предложение только поднимает)"))').click();
    const proposeForm = page.locator('form[action$="/mode/proposals"]').first();
    await proposeForm.locator('select[name="mode"]').selectOption('sales');
    await proposeForm.locator('select[name="scope"]').selectOption('conversation');
    await proposeForm.locator('input[name="reason"]').fill('Клиент хочет заказать деталь');
    await proposeForm.getByRole('button', { name: /Предложить режим/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?mode_proposed=1`));
    await page.goto(pathA);
    await expect(badge).toContainText('sales');
    await expect(modeHistory.getByRole('row')).toHaveCount(3);
    await expect(modeHistory.getByText(/предложение сигнала/)).toBeVisible();

    // 4. The sales reply: the disclosure gate refuses first (the flash
    // names the record path); the reply form requires the class (the
    // producer's mark — no silent default).
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const salesReply = page.locator('form[action$="/client-replies"]').first();
    const replyConversation = await salesReply.locator('select[name="conversation_id"] option').first().getAttribute('value');
    await salesReply.locator('select[name="conversation_id"]').selectOption(replyConversation);
    await salesReply.locator('select[name="reply_class"]').selectOption('sales');
    await salesReply.locator('textarea[name="content"]').fill('Предлагаю оригинальную деталь');
    await salesReply.getByRole('button', { name: /Подготовить ответ/ }).click();
    const gateFlash = page.locator('#case-flash');
    await expect(gateFlash).toBeVisible();
    await expect(gateFlash).toContainText(/раскрытия/i);

    // 5. The disclosure record: born generated (it does not act yet) —
    // the sales reply still refuses; the sent event opens it.
    await page.locator('details:has(summary:text-is("Записать раскрытие (текст цифрового сотрудника)"))').click();
    const disclosureForm = page.locator('form[action$="/disclosures"]').first();
    await disclosureForm.locator('select[name="channel"]').selectOption('phone');
    await disclosureForm.getByRole('button', { name: /Записать раскрытие/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?disclosure_recorded=1`));
    await page.goto(pathA);
    const disclosures = page.locator('div[aria-label="Раскрытия"] table');
    await expect(disclosures.getByText(/записано \(ещё не действует\)/)).toBeVisible();
    await expect(disclosures.getByText(/Я цифровой сотрудник/)).toBeVisible();

    // The sales reply still refuses (generated does not act).
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const stillBlocked = page.locator('form[action$="/client-replies"]').first();
    await stillBlocked.locator('select[name="conversation_id"]').selectOption(replyConversation);
    await stillBlocked.locator('select[name="reply_class"]').selectOption('sales');
    await stillBlocked.locator('textarea[name="content"]').fill('Предлагаю оригинальную деталь');
    await stillBlocked.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page.locator('#case-flash')).toContainText(/раскрытия/i);

    // The sent event: the disclosure acts.
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Событие доставки раскрытия №1"))').click();
    const eventForm = page.locator('form[action$="/delivery-events"]').first();
    await eventForm.locator('select[name="kind"]').selectOption('sent');
    await eventForm.getByRole('button', { name: /Записать событие/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?disclosure_event_recorded=1`));
    await page.goto(pathA);
    await expect(disclosures.getByText(/отправлено \(действует\)/)).toBeVisible();

    // 6. The sales reply passes now; the outgoing table shows the class.
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const openSales = page.locator('form[action$="/client-replies"]').first();
    await openSales.locator('select[name="conversation_id"]').selectOption(replyConversation);
    await openSales.locator('select[name="reply_class"]').selectOption('sales');
    await openSales.locator('textarea[name="content"]').fill('Предлагаю оригинальную деталь');
    await openSales.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?reply_prepared=1`));
    await page.goto(pathA);
    const outgoing = page.locator('div[aria-label="Исходящие ответы"] table');
    await expect(outgoing.getByText(/продажной/)).toBeVisible();

    // 7. The safety raise (the operator observation word) — the badge and
    // the active word announce the forbidden sales path with the exit
    // path; the aria-live badge has no focus move (the active element is
    // the page body after the reload — the badge itself is not focused).
    await page.locator('details:has(summary:text-is("Предложить режим (сигнал — предложение только поднимает)"))').click();
    const safetyForm = page.locator('form[action$="/mode/proposals"]').first();
    await safetyForm.locator('select[name="mode"]').selectOption('safety');
    await safetyForm.locator('input[name="reason"]').fill('Сигнал безопасности в реплике');
    await safetyForm.getByRole('button', { name: /Предложить режим/ }).click();
    await page.goto(pathA);
    await expect(badge).toContainText('safety');
    await expect(modePanel.getByText(/Активный режим/)).toContainText('safety');

    // 8. The sales reply under safety: the mode gate refusal with the
    // exit path (the machine-readable words travel inside the flash).
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const blockedSales = page.locator('form[action$="/client-replies"]').first();
    await blockedSales.locator('select[name="conversation_id"]').selectOption(replyConversation);
    await blockedSales.locator('select[name="reply_class"]').selectOption('sales');
    await blockedSales.locator('textarea[name="content"]').fill('Всё же предлагаю');
    await blockedSales.getByRole('button', { name: /Подготовить ответ/ }).click();
    const modeFlash = page.locator('#case-flash');
    await expect(modeFlash).toBeVisible();
    await expect(modeFlash).toContainText(/недоступны в текущем режиме/);

    // 9. The downgrade proposal refuses honestly (no surface can ever
    // lower by proposing): the honest form offers NO targets above safety
    // (the empty select IS the refusal of the surface); the machine
    // contract refuses the lowering word with the honest 400.
    await page.locator('details:has(summary:text-is("Предложить режим (сигнал — предложение только поднимает)"))').click();
    const proposeSelect = page.locator('form[action$="/mode/proposals"]').first().locator('select[name="mode"]');
    await expect(proposeSelect.locator('option')).toHaveCount(0); // nothing is above safety
    const downgrade = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/mode/proposals`, {
      data: {
        conversation_id: replyConversation,
        mode: 'sales',
        scope: 'conversation',
        expected_mode_version: 3,
        reason: 'Понизить',
        signal_source: 'operator',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-mode-downgrade-1" },
    });
    expect(downgrade.status()).toBe(400);
    expect((await downgrade.json()).message).toContain('понизить');

    // 10. The supervised exit (the fixture supervisor switch — the
    // configuration word): safety lowers back to sales; the term carries
    // the exit origin and the supervisor authority.
    await api.post('/fixture/supervisor', { form: { enabled: '1' } });
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Явный выход (полномочие: safety — надзор; остальные — владелец или надзор)"))').click();
    const exitForm = page.locator('form[action$="/mode/exit"]').first();
    await exitForm.locator('select[name="mode"]').selectOption('sales');
    await exitForm.locator('input[name="reason"]').fill('Безопасность отработана надзором');
    await exitForm.getByRole('button', { name: /Записать выход/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?mode_exited=1`));
    await api.post('/fixture/supervisor', { form: { enabled: '' } });
    await page.goto(pathA);
    await expect(badge).toContainText('sales');
    await expect(modeHistory.getByText(/явный выход/)).toBeVisible();
    await expect(modeHistory.getByText(/supervisor/)).toBeVisible();

    // 11. The interruption voids the acting disclosure; the re-disclosure
    // with the parent reference acts after its sent event (the sales
    // reply passes again).
    await page.locator('details:has(summary:text-is("Событие доставки раскрытия №1"))').click();
    const interruptForm = page.locator('form[action$="/delivery-events"]').first();
    await interruptForm.locator('select[name="kind"]').selectOption('interrupted');
    await interruptForm.getByRole('button', { name: /Записать событие/ }).click();
    await page.goto(pathA);
    await expect(disclosures.getByText(/прервано/)).toBeVisible();
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const voidedSales = page.locator('form[action$="/client-replies"]').first();
    await voidedSales.locator('select[name="conversation_id"]').selectOption(replyConversation);
    await voidedSales.locator('select[name="reply_class"]').selectOption('sales');
    await voidedSales.locator('textarea[name="content"]').fill('Ещё раз предлагаю');
    await voidedSales.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page.locator('#case-flash')).toContainText(/раскрытия/i);

    // The re-disclosure with the parent reference: the record id travels
    // in the action word of the delivery-event form (the row shows the
    // number, the text and the status — the identifier is the form's).
    const eventAction = await page.locator('form[action$="/delivery-events"]').first().getAttribute('action');
    const firstDisclosureID = (eventAction || '').split('/').slice(-2, -1)[0];
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Записать раскрытие (текст цифрового сотрудника)"))').click();
    const reDisclosureForm = page.locator('form[action$="/disclosures"]').first();
    await reDisclosureForm.locator('select[name="channel"]').selectOption('phone');
    await reDisclosureForm.locator('textarea[name="text"]').fill('Я цифровой сотрудник компании (повторяю).');
    await reDisclosureForm.locator('input[name="parent_ref"]').fill(String(firstDisclosureID).trim());
    await reDisclosureForm.getByRole('button', { name: /Записать раскрытие/ }).click();
    await page.goto(pathA);
    await expect(disclosures.getByRole('row')).toHaveCount(3); // the header + two records
    await page.locator('details:has(summary:text-is("Событие доставки раскрытия №2"))').click();
    const reSendForm = page.locator('form[action$="/delivery-events"]').nth(1);
    await reSendForm.locator('select[name="kind"]').selectOption('sent');
    await reSendForm.getByRole('button', { name: /Записать событие/ }).click();
    await page.goto(pathA);
    await expect(disclosures.getByText(/отправлено \(действует\)/)).toHaveCount(1);

    // 12. The 320px discipline: no horizontal scroll on the mode panel,
    // the disclosure table and the mode history; the badge and the panel
    // heading are keyboard reachable.
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
    const modeHeading = page.getByRole('heading', { name: /Conversation Mode/ });
    await expect(modeHeading).toBeVisible();
    await modeHeading.focus();

    // 13. The unclassified and the sales words are never alike: the badge
    // text of this case is the sales word now; the ladder table shows
    // every mode word distinctly.
    await expect(badge).not.toContainText('не классифицирован');
    await expect(modePanel.getByText(/Лестница/)).toBeVisible();
  } finally {
    await api.post('/fixture/supervisor', { form: { enabled: '' } });
    await api.post('/fixture/operator', { form: { second: '' } });
    await page.close();
  }
});
