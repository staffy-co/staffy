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

// The Story 2.9 chain at 320px: the Commitment Review page of one Work Item
// (the version cards with the material terms as text, the readiness and the
// current derivations, the token presence, the presentation word), the
// honest creation gates (the offered known price demands the fresh
// byte-equal observation; the draft never presents), the presentation link
// of the sales reply (the frozen 2.4..2.8 ladder), the PRG confirm of the
// EXACT shown version (the verbatim span excerpts of the client consent,
// the assurance ladder, the freshness pin), the Work Item transit to
// confirmed, the idempotent replay pair (one row, the stored outcome on
// both legs), the honest decline (append-only, nothing mutates), the new
// version after the change request (the superseded word of the former
// confirmation — never the color alone), the aria-live announcements with
// the focus transfer to the change description and the confirm button that
// is never focused nor preselected.
test('cases-commitment: the versioned offer, the presentation, the exact confirmation and the honest supersede at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. Three TEXT turns of one web_chat session: the request, the
    // explicit consent words (the confirm evidence) and the change request
    // (the decline evidence) — the modality boundary of the slice is text.
    const first = await postIngress(api, textInboundEvent('journey-commitment-evt-1', 'journey-commitment-sess-1', { say: 'Хочу заменить масло и фильтр, сколько стоит?' }), 'journey-commitment-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const turnOne = (await first.json()).event_id;
    const second = await postIngress(api, textInboundEvent('journey-commitment-evt-2', 'journey-commitment-sess-1', { say: 'Согласен, подтверждаю: подготовить заказ за 3000' }), 'journey-commitment-key-2');
    expect(second.status()).toBe(201);
    const turnTwo = (await second.json()).event_id;
    const third = await postIngress(api, textInboundEvent('journey-commitment-evt-3', 'journey-commitment-sess-1', { say: 'Передумал, давайте вместо замены только диагностику' }), 'journey-commitment-key-3');
    expect(third.status()).toBe(201);
    const turnThree = (await third.json()).event_id;
    const pathA = `${casesPath}/${caseA}`;
    const caseView0 = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;
    const conversationID = caseView0.conversations[0].ConversationID;
    const reviewPath = `${pathA}/work-items/${workItemID}/commitments`;

    // 2. The classification: the Work Item enters in_work (the confirm
    // ladder of 2.2).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Классифицировать / переклассифицировать"))').first().click();
    const classifyForm = page.locator('form[action$="/classify"]').first();
    await classifyForm.locator('select[name="type"]').selectOption('поиск детали');
    await classifyForm.locator('input[name="title"]').fill('Замена масла на Civic');
    await classifyForm.locator('input[name="reason"]').fill('Клиент просит замену масла');
    await classifyForm.getByRole('button', { name: /Классифицировать/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?classified=1`));

    // 3. The sales prelude of the fixture world (the 2.6 pattern): the
    // mode raise, the acting disclosure of the channel, the confirmed
    // service_kind fact, the AI proposal and the commercial outcome — the
    // frozen ladder the presentment reply must pass byte-for-byte.
    await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/mode/proposals`, {
      data: {
        conversation_id: conversationID,
        mode: 'sales', scope: 'conversation', expected_mode_version: 1,
        reason: 'Клиент интересуется ценой замены', signal_source: 'turn_event', turn_ref: turnOne,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-commitment-mode-1" },
    });
    const disclosure = await (await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/disclosures`, {
      data: { conversation_id: conversationID, channel: 'web_chat', text: 'Я цифровой сотрудник компании. Человек подключится по вашему запросу.' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-commitment-disc-1" },
    })).json();
    await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/disclosures/${disclosure.disclosure_id}/delivery-events`, {
      data: { conversation_id: conversationID, kind: 'sent' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-commitment-disc-sent-1" },
    });
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Записать факт"))').click();
    const factForm = page.locator('form[action$="/facts"]').first();
    await factForm.locator('select[name="source_type"]').selectOption('operator');
    await factForm.locator('input[name="material_key"]').fill('service_kind');
    await factForm.locator('textarea[name="value"]').fill('"диагностика"');
    await factForm.getByRole('button', { name: /Записать факт/ }).click();
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("service_kind — Без подтверждения"))').first().click();
    await page.getByText('Подтвердить точную версию факта service_kind').first().click();
    const confirmFact = page.locator('form[action$="/confirmations"]').first();
    await confirmFact.getByRole('button', { name: /Подтвердить версию/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?confirmed=1`));
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Подать предложение ИИ (порт-провод или оператор)"))').click();
    const proposalForm = page.locator('form[action$="/ai-proposals"]').first();
    await proposalForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await proposalForm.locator('select[name="work_item_id"]').selectOption(workItemID);
    await proposalForm.locator('select[name="turn_ref"]').selectOption(turnOne);
    await proposalForm.locator('input[name="intent"]').fill('price_inquiry');
    await proposalForm.locator('textarea[name="candidate_answer"]').fill('Замена масла 3000');
    await proposalForm.locator('input[name="missing_key_0"]').fill('urgency');
    await proposalForm.locator('select[name="missing_reason_0"]').selectOption('mandatory');
    await proposalForm.getByRole('button', { name: /Подать предложение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?proposal_submitted=1`));
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Оценить предложение (evaluate_policy)"))').first().click();
    const evaluateForm = page.locator('form[action$="/policy/evaluations"]').first();
    await evaluateForm.getByRole('button', { name: /Оценить предложение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?policy_evaluated=1`));
    await page.goto(pathA);
    await expect(page.locator('div[aria-live="polite"]').filter({ hasText: /commercial_answer/ })).toBeVisible();

    // 4. The assurance ladder of the confirm subject (the 2.3 forms): the
    // verified identity of the client — the TEXT modality ladder (the
    // address reply is the verified method of the text channels; the
    // callback lives on voice) — and, the price is positive, the acting
    // authority record (mandate_note, the payment scope).
    await page.locator('details:has(summary:text-is("Записать проверку (канонический исход, срок по методу)"))').click();
    const identityForm = page.locator('form[action$="/verifications"]').first();
    await identityForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await identityForm.locator('select[name="kind"]').selectOption('identity');
    await identityForm.locator('select[name="method"]').selectOption('known_address_reply');
    await identityForm.locator('select[name="provider_code"]').selectOption('ok');
    await identityForm.locator('input[name="subject_handle"]').fill('клиент');
    await identityForm.locator('input[name="note"]').fill('Ответ пришёл с известного адреса клиента, детали подтверждены');
    await identityForm.locator('select[name="evidence_event_id"]').selectOption('');
    await identityForm.getByRole('button', { name: /Записать проверку/ }).click();
    await expect(page).toHaveURL(new RegExp('verification_recorded=1'));
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Записать проверку (канонический исход, срок по методу)"))').click();
    const authorityForm = page.locator('form[action$="/verifications"]').first();
    await authorityForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await authorityForm.locator('select[name="kind"]').selectOption('authority');
    await authorityForm.locator('select[name="method"]').selectOption('mandate_note');
    await authorityForm.locator('select[name="provider_code"]').selectOption('ok');
    await authorityForm.locator('input[name="subject_handle"]').fill('клиент');
    await authorityForm.locator('input[name="authority_scope"]').fill('payment');
    await authorityForm.locator('input[name="note"]').fill('основание полномочий: письменная доверенность');
    await authorityForm.locator('select[name="evidence_event_id"]').selectOption('');
    await authorityForm.getByRole('button', { name: /Записать проверку/ }).click();
    await expect(page).toHaveURL(new RegExp('verification_recorded=1'));
    await page.goto(pathA);
    await expect(page.getByText('verified (подтверждена)').first()).toBeVisible();

    // 5. The Commitment Review page from the case panel link: the honest
    // empty state of the version history.
    await page.goto(pathA);
    await page.getByRole('link', { name: /Страница Commitment Review/ }).first().click();
    await expect(page).toHaveURL(new RegExp(reviewPath));
    await expect(page.getByRole('heading', { name: /Обязательства элемента работы/ })).toBeVisible();
    await expect(page.getByText('Версий обязательства нет')).toBeVisible();
    await expect(page.getByText(/Обязательство — версия с материальными условиями/)).toBeVisible();

    // 6. The draft version (a separate commitment): the honest draft card
    // — the status word and the readiness derivation say «не
    // подтверждаем» (the draft never presents, never confirms).
    const createForm = page.locator('form[action$="/commitments"]').first();
    await createForm.locator('select[name="status"]').selectOption('draft');
    await createForm.locator('input[name="scope"]').fill('Черновик замены масла');
    await createForm.locator('textarea[name="service_items"]').fill('масло 5 л');
    await createForm.locator('select[name="price_kind"]').selectOption('amount');
    await createForm.locator('input[name="price_amount_cents"]').fill('300000');
    await createForm.locator('input[name="payee"]').fill('ООО «Автозапчасть»');
    await createForm.locator('input[name="warranty_role"]').fill('гарантия на работы 6 месяцев');
    await createForm.locator('input[name="next_step"]').fill('запись на сервис');
    await createForm.locator('input[name="consequences"]').fill('без изменений');
    await createForm.locator('input[name="action_prepare_order"]').check();
    await createForm.locator('input[name="valid_until"]').fill('2026-09-17T12:00');
    await createForm.locator('input[name="consumption_limit"]').fill('1');
    await createForm.getByRole('button', { name: /Создать версию/ }).click();
    await expect(page).toHaveURL(new RegExp('commitment_created=1'));
    await page.goto(reviewPath);
    const draftCard = page.locator('section[aria-label="Версия 1 обязательства"]').filter({ hasText: 'черновик (не подтверждаем)' }).first();
    await expect(draftCard).toContainText('черновик — не подтверждаем');

    // 7. The honest creation refusal: the offered version with the KNOWN
    // price demands the fresh price observation — the flash names the
    // read_owning_system path (the honest 400 of the gate, never a guessed
    // value); the flash takes the focus.
    await page.locator('form[action$="/commitments"]').first().locator('select[name="status"]').selectOption('offered');
    const offeredCreate = page.locator('form[action$="/commitments"]').first();
    await offeredCreate.locator('input[name="scope"]').fill('Замена масла и фильтра');
    await offeredCreate.locator('textarea[name="service_items"]').fill('масло 5 л\nмасляный фильтр');
    await offeredCreate.locator('select[name="price_kind"]').selectOption('amount');
    await offeredCreate.locator('input[name="price_amount_cents"]').fill('300000');
    await offeredCreate.locator('input[name="payee"]').fill('ООО «Автозапчасть»');
    await offeredCreate.locator('input[name="warranty_role"]').fill('гарантия на работы 6 месяцев');
    await offeredCreate.locator('input[name="next_step"]').fill('запись на сервис');
    await offeredCreate.locator('input[name="consequences"]').fill('без изменений');
    await offeredCreate.locator('input[name="action_prepare_order"]').check();
    await offeredCreate.locator('input[name="valid_until"]').fill('2026-09-17T12:00');
    await offeredCreate.locator('input[name="consumption_limit"]').fill('1');
    await offeredCreate.getByRole('button', { name: /Создать версию/ }).click();
    await expect(page).toHaveURL(reviewPath);
    const createFlash = page.locator('#commitment-flash');
    await expect(createFlash).toContainText('требует свежего наблюдения price');
    await expect(createFlash).toBeFocused();

    // 8. The price observation of the Work Item: the fixture scripting of
    // the owning-system answer of exactly this work item (the canonical
    // price bytes), then the honest read — the fresh card carries the
    // byte-equal canonical value.
    const scriptPrice = await api.post('/fixture/price-quote', {
      data: `work_item=${workItemID}&amount_cents=300000`,
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    });
    expect(scriptPrice.status()).toBe(204);
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Перечитать поле Owning System (read_owning_system)"))').first().click();
    const readForm = page.locator('form[action$="/owning-system-reads"]').first();
    await readForm.locator('select[name="material_key"]').selectOption('price');
    await readForm.getByRole('button', { name: /Перечитать поле/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?owning_system_read=1`));
    await page.goto(pathA);
    const priceCard = page.locator('details').filter({ has: page.locator('summary', { hasText: /^price — свежее/ }) }).first();
    await priceCard.locator('summary').click();
    await expect(priceCard.locator('.digest-block').filter({ hasText: /\{"amount_cents":300000,"kind":"amount"\}/ })).toBeVisible();

    // 9. The offered version: the same form passes the gate now — the PRG
    // announcement region takes the focus (AC-12 — the change is said in
    // text and the focus moves to the description); the card shows the
    // material terms as text, the readiness word, the token presence and
    // the honest «не предъявлена».
    await page.goto(reviewPath);
    const offeredRetry = page.locator('form[action$="/commitments"]').first();
    await offeredRetry.locator('select[name="status"]').selectOption('offered');
    await offeredRetry.locator('input[name="scope"]').fill('Замена масла и фильтра');
    await offeredRetry.locator('textarea[name="service_items"]').fill('масло 5 л\nмасляный фильтр');
    await offeredRetry.locator('select[name="price_kind"]').selectOption('amount');
    await offeredRetry.locator('input[name="price_amount_cents"]').fill('300000');
    await offeredRetry.locator('input[name="payee"]').fill('ООО «Автозапчасть»');
    await offeredRetry.locator('input[name="warranty_role"]').fill('гарантия на работы 6 месяцев');
    await offeredRetry.locator('input[name="next_step"]').fill('запись на сервис');
    await offeredRetry.locator('input[name="consequences"]').fill('без изменений');
    await offeredRetry.locator('input[name="action_prepare_order"]').check();
    await offeredRetry.locator('input[name="valid_until"]').fill('2026-09-17T12:00');
    await offeredRetry.locator('input[name="consumption_limit"]').fill('1');
    await offeredRetry.getByRole('button', { name: /Создать версию/ }).click();
    await expect(page).toHaveURL(new RegExp('commitment_created=1'));
    const changedRegion = page.locator('#commitment-changed');
    await expect(changedRegion).toContainText('Версия обязательства создана');
    await expect(changedRegion).toBeFocused();
    await page.goto(reviewPath);
    const offeredCard = page.locator('section[aria-label="Версия 1 обязательства"]').filter({ hasText: 'предложена (offered)' }).first();
    await expect(offeredCard).toContainText('текущая');
    await expect(offeredCard).toContainText('подтверждаема');
    await expect(offeredCard).toContainText('не предъявлена');
    await expect(offeredCard).toContainText('привязка контента записана (HMAC)');
    await expect(offeredCard).toContainText('сумма 300000 коп.');
    await expect(offeredCard).toContainText('масляный фильтр');
    await expect(offeredCard).toContainText('нет (все обязательные термины известны)');
    await expect(offeredCard).toContainText('prepare_order');
    const commitmentB = (await offeredCard.locator('.digest-block').first().textContent())?.trim();
    expect(commitmentB).toMatch(/^[0-9a-f-]{36}$/);

    // 10. The pre-presentation refusal: the confirm of the version that
    // was never shown — the flash names the presentation link rule (the
    // honest 400 not-presented — only the shown version confirms).
    await offeredCard.locator('details:has(summary:text-is("Подтвердить эту версию (confirm_commitment)"))').click();
    const earlyConfirm = page.locator(`form[action$="/commitments/${commitmentB}/confirm"]`).first();
    await earlyConfirm.locator('input[name="subject_ref"]').fill('клиент');
    await earlyConfirm.locator('select[name="turn_ref"]').selectOption(turnTwo);
    await earlyConfirm.locator('input[name="span_1"]').fill('Согласен, подтверждаю');
    await earlyConfirm.getByRole('button', { name: /Подтвердить показанную версию/ }).click();
    await expect(page).toHaveURL(reviewPath);
    await expect(page.locator('#commitment-flash')).toContainText('не была предъявлена клиенту');

    // 11. The presentation: the sales reply with the commitment link —
    // the link select offers the current offered versions only (the draft
    // never appears); the reply passes the frozen ladder and carries the
    // exact pair.
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const replyForm = page.locator('form[action$="/client-replies"]').first();
    const linkOptions = replyForm.locator('select[name="commitment_link"] option');
    await expect(linkOptions).toHaveCount(2); // the empty option + the offered №1
    await replyForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await replyForm.locator('select[name="reply_class"]').selectOption('sales');
    await replyForm.locator('select[name="work_item_id"]').selectOption(workItemID);
    await replyForm.locator('select[name="commitment_link"]').selectOption(`${commitmentB}:1`);
    await replyForm.locator('textarea[name="content"]').fill('Предлагаем замену масла и фильтра за 3000 руб. с записью на сервис.');
    await replyForm.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?reply_prepared=1`));
    await page.goto(reviewPath);
    await expect(offeredCard).toContainText('предъявлена клиенту');

    // 12. The confirmation of the EXACT shown version: the subject that
    // matches the verified identity, the accepted text turn with the
    // verbatim span excerpt of the consent words; the Work Item transits
    // to confirmed; the announcement takes the focus; the confirmations
    // table shows the subject, the method and the acting validity.
    await offeredCard.locator('details:has(summary:text-is("Подтвердить эту версию (confirm_commitment)"))').click();
    const confirmForm = page.locator(`form[action$="/commitments/${commitmentB}/confirm"]`).first();
    await confirmForm.locator('input[name="subject_ref"]').fill('клиент');
    await confirmForm.locator('select[name="turn_ref"]').selectOption(turnTwo);
    await confirmForm.locator('input[name="span_1"]').fill('Согласен, подтверждаю');
    await confirmForm.getByRole('button', { name: /Подтвердить показанную версию/ }).click();
    await expect(page).toHaveURL(new RegExp('commitment_confirmed=1'));
    const confirmRegion = page.locator('#commitment-changed');
    await expect(confirmRegion).toContainText('элемент работы переведён в состояние confirmed');
    await expect(confirmRegion).toBeFocused();
    await page.goto(reviewPath);
    await expect(page.getByText('Подтверждено (согласие клиента записано)')).toBeVisible();
    const confirmationsTable = page.locator('table').filter({ has: page.getByText('Подтверждения: версия, субъект, метод, канал, срок, действительность') });
    const confirmationRow = confirmationsTable.getByRole('row').filter({ hasText: '№1' });
    await expect(confirmationRow).toContainText('client/клиент');
    await expect(confirmationRow).toContainText('explicit_text');
    await expect(confirmationRow).toContainText('действует');
    await expect(page.locator('#commitment-summary')).toContainText('Версий: 2. Подтверждений: 1. Отказов: 0.');

    // 13. The idempotent replay pair (the 2.8 pattern): the exact repeat
    // of the confirm command returns the STORED outcome — both legs land
    // the success flag, and the summary keeps ONE confirmation (no new
    // row, no new event).
    const confirmAction = `/app/tenants/${tenantID}/cases/${caseA}/work-items/${workItemID}/commitments/${commitmentB}/confirm`;
    const replayBody = new URLSearchParams({
      version_no: '1',
      expected_version: '2',
      subject_ref: 'клиент',
      turn_ref: turnTwo,
      span_1: 'Согласен, подтверждаю',
    }).toString();
    const firstPost = await api.post(confirmAction, { data: replayBody, headers: { 'Content-Type': 'application/x-www-form-urlencoded' } });
    expect(firstPost.status()).toBeLessThan(400);
    expect(firstPost.url()).toContain('commitment_confirmed=1');
    const secondPost = await api.post(confirmAction, { data: replayBody, headers: { 'Content-Type': 'application/x-www-form-urlencoded' } });
    expect(secondPost.status()).toBeLessThan(400);
    expect(secondPost.url()).toContain('commitment_confirmed=1');
    await page.goto(reviewPath);
    await expect(page.locator('#commitment-summary')).toContainText('Версий: 2. Подтверждений: 1. Отказов: 0.');

    // 14. The honest decline of the change request: the decline row lands
    // append-only — the outcome word and the verbatim excerpt of the
    // change words; nothing mutates (the versions and the confirmation
    // stay byte-identical).
    await page.goto(reviewPath);
    const declinedVersionCard = page.locator('section[aria-label="Версия 1 обязательства"]').filter({ hasText: 'предложена (offered)' }).first();
    await declinedVersionCard.locator('details:has(summary:text-is("Зафиксировать отказ или запрос изменения (record_commitment_decline)"))').click();
    const declineForm = page.locator(`form[action$="/commitments/${commitmentB}/decline"]`).first();
    await declineForm.locator('select[name="outcome"]').selectOption('change_requested');
    await declineForm.locator('select[name="turn_ref"]').selectOption(turnThree);
    await declineForm.locator('input[name="span_1"]').fill('Передумал');
    await declineForm.getByRole('button', { name: /Записать отказ/ }).click();
    await expect(page).toHaveURL(new RegExp('commitment_declined=1'));
    await expect(page.locator('#commitment-changed')).toContainText('Отказ записан: материальные условия не изменились');
    await page.goto(reviewPath);
    const declinesTable = page.locator('table').filter({ has: page.getByText('Отказы: версия, исход, источник, время') });
    const declineRow = declinesTable.getByRole('row').filter({ hasText: '№1' });
    await expect(declineRow).toContainText('запрос изменения');
    await expect(page.locator('#commitment-summary')).toContainText('Версий: 2. Подтверждений: 1. Отказов: 1.');

    // 15. The new version after the change request: the owning system
    // answers the changed price (the new observation with the new
    // canonical value), the operator creates version №2 of the SAME
    // commitment (the identifier field carries the link); the former
    // version is superseded IN TEXT (never the color alone) and the former
    // confirmation derivation flips to «заменена».
    await api.post('/fixture/price-quote', {
      data: `work_item=${workItemID}&amount_cents=310000`,
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    });
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Перечитать поле Owning System (read_owning_system)"))').first().click();
    const readForm2 = page.locator('form[action$="/owning-system-reads"]').first();
    await readForm2.locator('select[name="material_key"]').selectOption('price');
    await readForm2.getByRole('button', { name: /Перечитать поле/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?owning_system_read=1`));
    await page.goto(reviewPath);
    const versionTwoForm = page.locator('form[action$="/commitments"]').first();
    await versionTwoForm.locator('input[name="commitment_id"]').fill(commitmentB as string);
    await versionTwoForm.locator('select[name="status"]').selectOption('offered');
    await versionTwoForm.locator('input[name="scope"]').fill('Диагностика без замены');
    await versionTwoForm.locator('textarea[name="service_items"]').fill('компьютерная диагностика');
    await versionTwoForm.locator('select[name="price_kind"]').selectOption('amount');
    await versionTwoForm.locator('input[name="price_amount_cents"]').fill('310000');
    await versionTwoForm.locator('input[name="payee"]').fill('ООО «Автозапчасть»');
    await versionTwoForm.locator('input[name="warranty_role"]').fill('гарантия на работы 6 месяцев');
    await versionTwoForm.locator('input[name="next_step"]').fill('запись на диагностику');
    await versionTwoForm.locator('input[name="consequences"]').fill('без изменений');
    await versionTwoForm.locator('input[name="action_issue_document"]').check();
    await versionTwoForm.locator('input[name="valid_until"]').fill('2026-09-18T12:00');
    await versionTwoForm.locator('input[name="consumption_limit"]').fill('1');
    await versionTwoForm.getByRole('button', { name: /Создать версию/ }).click();
    await expect(page).toHaveURL(new RegExp('commitment_created=1'));
    await page.goto(reviewPath);
    const versionTwoCard = page.locator('section[aria-label="Версия 2 обязательства"]').first();
    await expect(versionTwoCard).toContainText('предложена (offered)');
    await expect(versionTwoCard).toContainText('текущая');
    await expect(versionTwoCard).toContainText('подтверждаема');
    await expect(versionTwoCard).toContainText('сумма 310000 коп.');
    await expect(versionTwoCard).toContainText('не предъявлена');
    const supersededCard = page.locator('section[aria-label="Версия 1 обязательства"]').filter({ hasText: 'предложена (offered)' }).first();
    await expect(supersededCard).toContainText('заменена новой версией');
    const supersededConfirmation = confirmationsTable.getByRole('row').filter({ hasText: '№1' });
    await expect(supersededConfirmation).toContainText('заменена (есть версия новее)');
    await expect(page.locator('#commitment-summary')).toContainText('Версий: 3. Подтверждений: 1. Отказов: 1.');

    // 16. The a11y close of the review page: the confirm button is never
    // focused nor preselected (the subject input renders empty, the button
    // carries no autofocus; a plain load leaves the focus on the page, not
    // on the primary action); the summary region announces politely; the
    // OPEN forms (the confirm details expanded) and the long digests and
    // the material terms wrap — no horizontal scroll at 320px.
    await page.goto(reviewPath);
    const plainConfirmForm = page.locator(`form[action$="/commitments/${commitmentB}/confirm"]`).first();
    await expect(plainConfirmForm.locator('input[name="subject_ref"]')).toHaveValue('');
    await expect(plainConfirmForm.locator('button').first()).not.toHaveAttribute('autofocus');
    const activeTag = await page.evaluate(() => document.activeElement?.tagName);
    expect(['BODY', 'html', undefined]).toContain(activeTag ?? 'BODY');
    const liveSummary = page.locator('#commitment-summary');
    await expect(liveSummary).toBeAttached();
    await expect(liveSummary).toHaveAttribute('aria-live', 'polite');
    await page.locator('section[aria-label="Версия 1 обязательства"]').filter({ hasText: 'предложена (offered)' }).first().locator('details:has(summary:text-is("Подтвердить эту версию (confirm_commitment)"))').click();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    await page.close();
  }
});
