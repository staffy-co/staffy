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

// The Story 2.10 chain at 320px: the purpose-bound continuation link of one
// Work Item (the operator issue form with the ONE-TIME URL card — the token
// exists only there), the honest replay mask (the machine path: the stored
// outcome, the empty URL, the Idempotency-Replayed header), the delivery
// binding of the sales reply (the FR-16 delivered badge of the panel), the
// protected client page (the tenant name and the role constant, the
// explanation, the confirmed facts of THIS work item, the open question,
// the correction form and the persistent «Позвать человека» /
// «Вернуться в разговор»), the PRG correction with the aria-live landing
// («отправлено — оператор проверит»), the terminal consumed page (the
// stable repeat, the honest dead-link redirect), the unverified version
// visible to the operator (the штатный confirmation stays the operator's
// command), the free events (human/return never consume the limit), the
// value-bound re-render with the preserved input (the data never
// disappears, the limit stays), the revoke leg (the PRG flash, the
// operator panel words, the client generic refusal), the expired leg (the
// fixture clock shift — the ttl window is the honest server time) and the
// a11y close (320px without the horizontal scroll, the touch targets
// 44px, aria-live, no autofocus, no preselect).
test('cases-continuation: the purpose-bound link, the protected page, the exact correction and the honest lifecycle at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. Two TEXT turns of one web_chat session: the request and the
    // consent words (the modality boundary of the slice is text).
    const first = await postIngress(api, textInboundEvent('journey-continuation-evt-1', 'journey-continuation-sess-1', { say: 'Хочу заменить масло и фильтр, сколько стоит?' }), 'journey-continuation-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const turnOne = (await first.json()).event_id;
    const second = await postIngress(api, textInboundEvent('journey-continuation-evt-2', 'journey-continuation-sess-1', { say: 'Согласен, подтверждаю: подготовить заказ за 3000' }), 'journey-continuation-key-2');
    expect(second.status()).toBe(201);
    const turnTwo = (await second.json()).event_id;
    const pathA = `${casesPath}/${caseA}`;
    const caseView0 = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;
    const conversationID = caseView0.conversations[0].ConversationID;

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
    // frozen ladder the binding reply must pass byte-for-byte.
    await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/mode/proposals`, {
      data: {
        conversation_id: conversationID,
        mode: 'sales', scope: 'conversation', expected_mode_version: 1,
        reason: 'Клиент интересуется ценой замены', signal_source: 'turn_event', turn_ref: turnOne,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-continuation-mode-1" },
    });
    const disclosure = await (await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/disclosures`, {
      data: { conversation_id: conversationID, channel: 'web_chat', text: 'Я цифровой сотрудник компании. Человек подключится по вашему запросу.' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-continuation-disc-1" },
    })).json();
    await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/disclosures/${disclosure.disclosure_id}/delivery-events`, {
      data: { conversation_id: conversationID, kind: 'sent' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-continuation-disc-sent-1" },
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

    // 4. The issue form of the first link: the purpose checkbox of the v1
    // dictionary, the open question, the explanation, the participant
    // reference, the ttl and the limit; the honest answer is the RE-RENDER
    // of the case page with the one-time URL card (no PRG — the flash
    // would lose the secret; no-store and no-referrer keep it out of the
    // caches).
    await page.locator('details:has(summary:text-is("Выдать ссылку продолжения (issue_continuation_link)"))').click();
    const issueForm = page.locator(`form[action$="/work-items/${workItemID}/continuation-links"]`).first();
    await issueForm.locator('input[name="purpose_fact_correction"]').check();
    await issueForm.locator('input[name="question"]').fill('Вид обслуживания верный?');
    await issueForm.locator('textarea[name="explanation"]').fill('Перейдите по ссылке, чтобы исправить один факт: оператор проверит отправленное и подтвердит его штатным путём.');
    await issueForm.locator('input[name="participant_ref"]').fill('client+79000000201');
    await issueForm.locator('input[name="ttl_minutes"]').fill('30');
    await issueForm.locator('input[name="consumption_limit"]').fill('1');
    await issueForm.getByRole('button', { name: /Выдать ссылку/ }).click();
    await expect(page).toHaveURL(/work-items\/[^/]+\/continuation-links$/);
    const card = page.locator('#continuation-issued');
    await expect(card).toBeVisible();
    await expect(card).toContainText('Ссылка продолжения выдана');
    await expect(card).toContainText('восстановление невозможно');
    const url1 = await page.locator('#continuation-url').inputValue();
    expect(url1).toMatch(/^\/continue\/[A-Za-z0-9_-]{64}$/);
    // The focus moves to the card (the one-time material announces itself).
    await expect(card).toBeFocused();
    // The panel card of the fresh link: the derived «действует» state and
    // the FR-16 «ожидается ответ» word (no uses yet).
    const panel = page.locator(`#continuations-${workItemID}`);
    await expect(panel).toContainText('действует');
    await expect(panel).toContainText('ожидается ответ');

    // 5. The honest replay mask (the machine path of the second link): the
    // exact repeat of the issuance answers the STORED outcome — 200 with
    // the Idempotency-Replayed header, the EMPTY url/token (the preimage
    // is not recoverable from the stored hash — a lost URL means the
    // revocation and a fresh link).
    const issueBody = {
      purposes: ['fact_correction'],
      question: 'Проверьте марку',
      explanation: 'Второй переход для событий.',
      participant_ref: 'client+79000000201',
      ttl_minutes: 30,
    };
    const fresh = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/work-items/${workItemID}/continuation-links`, {
      data: issueBody,
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-continuation-issue-2" },
    });
    expect(fresh.status()).toBe(201);
    const freshJSON = await fresh.json();
    const url2: string = freshJSON.url;
    const secondLinkID: string = freshJSON.outcome.link_id;
    expect(url2).toMatch(/^\/continue\/[A-Za-z0-9_-]{64}$/);
    const replay = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/work-items/${workItemID}/continuation-links`, {
      data: issueBody,
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-continuation-issue-2" },
    });
    expect(replay.status()).toBe(200);
    expect(replay.headers()['idempotency-replayed']).toBe('true');
    const replayJSON = await replay.json();
    expect(replayJSON.url || replayJSON.token || '').toBe('');

    // 6. The delivery binding: the sales reply carries the link of the
    // select (the VALID undelivered links only) and weaves the URL text —
    // the reply passes the frozen ladder and lands the delivered badge.
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const replyForm = page.locator('form[action$="/client-replies"]').first();
    const linkOptions = replyForm.locator('select[name="continuation_link_id"] option');
    await expect(linkOptions).toHaveCount(3); // the empty option + the two valid undelivered links
    const firstLinkID = await linkOptions.nth(1).getAttribute('value');
    await replyForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await replyForm.locator('select[name="reply_class"]').selectOption('sales');
    await replyForm.locator('select[name="work_item_id"]').selectOption(workItemID);
    await replyForm.locator('select[name="continuation_link_id"]').selectOption(firstLinkID!);
    await replyForm.locator('textarea[name="content"]').fill(`Продолжим в удобном месте: ${url1} — там можно исправить один факт.`);
    await replyForm.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?reply_prepared=1`));
    await page.goto(pathA);
    await expect(page.locator(`#continuations-${workItemID}`)).toContainText('доставлена (связана с исходящим ответом)');

    // 7. The protected client page of the first link at 320px: the token
    // IS the credential (no session, no operator routes); the page honestly
    // shows whose conversation it is (the tenant name + the role constant),
    // the explanation, the confirmed facts of THIS work item, the open
    // question, the correction form and the two persistent actions; the
    // page never caches (no-store) and never leaks the referrer.
    const pageResponse = await page.goto(url1);
    expect(pageResponse?.status()).toBe(200);
    expect(pageResponse?.headers()['cache-control']).toContain('no-store');
    expect(pageResponse?.headers()['referrer-policy']).toBe('no-referrer');
    await expect(page.getByRole('heading', { name: 'Продолжение разговора' })).toBeVisible();
    await expect(page.getByText('Магазин Олега')).toBeVisible();
    await expect(page.getByText('ИИ-сотрудник')).toBeVisible();
    await expect(page.getByText('Перейдите по ссылке, чтобы исправить один факт: оператор проверит отправленное и подтвердит его штатным путём.')).toBeVisible();
    await expect(page.getByText('Вид обслуживания верный?')).toBeVisible();
    await expect(page.locator('.continuation-facts li')).toContainText('service_kind');
    await expect(page.locator('.continuation-facts li')).toContainText('диагностика');
    await expect(page.getByText('Показаны только подтверждённые факты этого разговора')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Отправить исправление' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Позвать человека' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Вернуться в разговор' })).toBeVisible();
    await expect(page.getByText('Осталось отправок по этой ссылке: 1')).toBeVisible();

    // 8. The a11y close of the client page: the touch targets of the three
    // actions are at least 44×44px, the status region announces politely,
    // the primary action is never focused nor preselected (the textarea
    // renders empty, the select carries no explicit selected option, the
    // focus stays on the page) and the long material wraps — no horizontal
    // scroll at 320px.
    const actions = page.locator('.continuation-form button, .continuation-action button');
    await expect(actions).toHaveCount(3);
    for (let i = 0; i < 3; i++) {
      const box = await actions.nth(i).boundingBox();
      expect(box!.height).toBeGreaterThanOrEqual(44);
      expect(box!.width).toBeGreaterThanOrEqual(44);
    }
    await expect(page.locator('#continuation-status')).toHaveAttribute('aria-live', 'polite');
    await expect(page.locator('#continuation-value')).toHaveValue('');
    await expect(page.locator('#continuation-fact option[selected]')).toHaveCount(0);
    await expect(page.locator('.continuation-form button')).not.toHaveAttribute('autofocus');
    const activeTag = await page.evaluate(() => document.activeElement?.tagName);
    expect(['BODY', 'html', undefined]).toContain(activeTag ?? 'BODY');
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);

    // 9. The correction: the client selects the confirmed fact and types
    // the new value (the text modality — the plain text becomes the quoted
    // canonical value); the PRG landing announces «отправлено — оператор
    // проверит» in the polite live region with the focus transfer.
    const factID = await page.locator('#continuation-fact option').first().getAttribute('value');
    await page.locator('#continuation-fact').selectOption(factID!);
    await page.locator('#continuation-value').fill('не диагностика, а замена масла');
    await page.getByRole('button', { name: 'Отправить исправление' }).click();
    await expect(page).toHaveURL(new RegExp('submitted=1'));
    const notice = page.locator('#continuation-notice');
    await expect(notice).toContainText('оператор проверит');
    await expect(notice).toBeFocused();
    // The link is finished: the terminal page is the honest answer of the
    // repeat GET (stable — the submission never resurrects).
    const terminal = await page.goto(url1);
    expect(terminal?.status()).toBe(200);
    await expect(page.getByRole('heading', { name: 'Материал отправлен' })).toBeVisible();
    await expect(page.getByText('повторная отправка по ней невозможна')).toBeVisible();
    // The dead-link correction: the redirect to the derived page (the
    // double-click of the limit is the terminal page — PRG honest).
    const deadPost = await api.post(`${url1}/correct`, {
      data: new URLSearchParams({ fact_id: factID!, value: 'ещё раз' }).toString(),
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    });
    expect(deadPost.status()).toBe(200);
    expect(await deadPost.text()).toContain('Материал отправлен');

    // 10. The operator side after the correction: the panel card of the
    // first link derives «отправка завершена» with the 1/1 accounting and
    // the event feed row; the unverified version of the fact is VISIBLE to
    // the operator (the surface never confirms — the штатная команда 2.2
    // stays the operator's); the revoked form is gone (a dead link is not
    // revocable — the expectation ended).
    await page.goto(pathA);
    const panelAfter = page.locator(`#continuations-${workItemID}`);
    await expect(panelAfter).toContainText('отправка завершена');
    await expect(panelAfter).toContainText('Отправок: 1 / 1');
    await expect(panelAfter).toContainText('исправление отправлено');
    await expect(panelAfter).toContainText('доставлена (связана с исходящим ответом)');
    await expect(page.locator(`form[action$="/continuation-links/${firstLinkID}/revoke"]`)).toHaveCount(0);
    // The fact derivation flips to «Конфликт значений» (the confirmed v1
    // against the new unverified v2 — the honest 2.2 status; the details
    // auto-open); the version 2 row carries the client value with the
    // protected-continuation provenance (system source, the link ref, the
    // channel word) and «Без подтверждения».
    const correctedFact = page.locator('details:has(summary:text-is("service_kind — Конфликт значений"))').first();
    await expect(correctedFact).toBeVisible();
    const versionsTable = correctedFact.getByRole('table');
    await expect(versionsTable).toContainText('не диагностика, а замена масла');
    await expect(versionsTable).toContainText('protected_continuation');
    const versionRows = versionsTable.getByRole('row').filter({ hasText: 'Без подтверждения' });
    await expect(versionRows).toHaveCount(1);
    await expect(versionRows).toContainText('system');

    // 11. The free events of the second link: «Позвать человека» and
    // «Вернуться в разговор» are the PRG events WITHOUT material and
    // WITHOUT consumption — the limit stays honest; the landings announce
    // the operator words; the panel feed carries both rows.
    await page.goto(url2);
    await page.getByRole('button', { name: 'Позвать человека' }).click();
    await expect(page).toHaveURL(new RegExp('human=1'));
    await expect(page.locator('#continuation-notice')).toContainText('оператор свяжется с вами');
    await page.getByRole('button', { name: 'Вернуться в разговор' }).click();
    await expect(page).toHaveURL(new RegExp('returned=1'));
    await expect(page.locator('#continuation-notice')).toContainText('Возвращайтесь в ваш разговор');
    await expect(page.getByText('Осталось отправок по этой ссылке: 1')).toBeVisible();
    await page.goto(pathA);
    await expect(page.locator(`#continuations-${workItemID}`)).toContainText('позвать человека');
    await expect(page.locator(`#continuations-${workItemID}`)).toContainText('вернулся в разговор');
    await expect(page.locator(`#continuations-${workItemID}`)).toContainText('Отправок: 0 / 1');

    // 12. The value-bound refusal of the second link: the canonical value
    // over the 2 KiB bound re-renders the form with the PRESERVED input
    // and the honest rule word (the data never disappears); the limit is
    // NOT consumed (the refusal never spends the capability).
    const longValue = 'ы'.repeat(2100);
    const refusal = await api.post(`${url2}/correct`, {
      data: new URLSearchParams({ fact_id: factID!, value: longValue }).toString(),
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    });
    expect(refusal.status()).toBe(400);
    const refusalHTML = await refusal.text();
    expect(refusalHTML).toContain('длиннее 2048');
    expect(refusalHTML).toContain('введённое сохранено');
    expect(refusalHTML).toContain('role="alert"');
    await page.goto(url2);
    await expect(page.getByText('Осталось отправок по этой ссылке: 1')).toBeVisible();

    // 13. The revoke leg of the third link: the operator form with the
    // honest reason (the PRG flash «Ожидание прекращено»), the panel card
    // derives the revoked word; the client page answers the SINGLE generic
    // refusal (404 — no cause disclosure, the same words as every dead
    // link).
    const third = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/work-items/${workItemID}/continuation-links`, {
      data: {
        purposes: ['fact_correction'],
        question: 'Проверьте VIN',
        explanation: 'Третий переход для отзыва.',
        participant_ref: 'client+79000000201',
        ttl_minutes: 30,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-continuation-issue-3" },
    });
    expect(third.status()).toBe(201);
    const thirdJSON = await third.json();
    const url3: string = thirdJSON.url;
    const thirdLinkID: string = thirdJSON.outcome.link_id;
    await page.goto(pathA);
    const thirdCard = page.locator('details.connector-connect').filter({ has: page.locator(`form[action$="/continuation-links/${thirdLinkID}/revoke"]`) }).first();
    await thirdCard.locator('summary').click();
    const revokeForm = page.locator(`form[action$="/continuation-links/${thirdLinkID}/revoke"]`);
    await revokeForm.locator('input[name="reason"]').fill('клиент вернулся в разговор — ожидание прекращено');
    await revokeForm.getByRole('button', { name: /Прекратить ожидание/ }).click();
    await expect(page).toHaveURL(new RegExp('continuation_revoked=1'));
    await expect(page.getByText('Ожидание прекращено: ссылка отозвана')).toBeVisible();
    await expect(page.locator(`#continuations-${workItemID}`)).toContainText('отозвана (ожидание прекращено)');
    const revokedPage = await page.goto(url3);
    expect(revokedPage?.status()).toBe(404);
    await expect(page.getByRole('heading', { name: 'Ссылка недействительна или истекла' })).toBeVisible();
    await expect(page.getByText('Свяжитесь с нами по каналу обращения')).toBeVisible();
    // The generic refusal never discloses the cause words.
    const refusalText = await page.locator('section[aria-label="Защищённая страница продолжения"]').innerText();
    for (const cause of ['отозвана', 'истёк срок', 'владение', 'expired', 'revoked', 'owner']) {
      expect(refusalText).not.toContain(cause);
    }

    // 14. The expired leg of the second link: the fixture clock moves past
    // the ttl window (the honest server-side time window of the link) — the
    // client page answers the SAME generic refusal (404, the identical
    // words — the client never sees which cause died the link); the event
    // POST of the dead link redirects to the derived page.
    await api.post('/fixture/clock', {
      data: 'minutes=31',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    });
    const expiredPage = await page.goto(url2);
    expect(expiredPage?.status()).toBe(404);
    await expect(page.getByRole('heading', { name: 'Ссылка недействительна или истекла' })).toBeVisible();
    const deadEvent = await api.post(`${url2}/human`, {
      data: '',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    });
    expect(deadEvent.status()).toBe(404);
    expect(await deadEvent.text()).toContain('Ссылка недействительна или истекла');
    // The clock resets (the fixture world returns to the base moment).
    await api.post('/fixture/clock', {
      data: 'minutes=0',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    });

    // 15. The unknown token: the well-formed but unknown hash (one flipped
    // character of a real token) answers the same generic refusal — the
    // uniform 404 of every dead door (the AC-9 honesty).
    const flipped = url2.startsWith('/continue/A')
      ? url2.replace('/continue/A', '/continue/B')
      : url2.replace('/continue/B', '/continue/A');
    expect(flipped).not.toBe(url2);
    const unknownPage = await page.goto(flipped);
    expect(unknownPage?.status()).toBe(404);
    await expect(page.getByRole('heading', { name: 'Ссылка недействительна или истекла' })).toBeVisible();
  } finally {
    await api.post('/fixture/clock', {
      data: 'minutes=0',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    }).catch(() => {});
    await page.close();
  }
});
