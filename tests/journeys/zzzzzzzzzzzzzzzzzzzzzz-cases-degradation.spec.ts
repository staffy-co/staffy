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

// The Story 2.12 chain at 320px: the recorded route failure (the channel
// word «timeout» becomes the append-only revision row of the «Состояние
// маршрутов» panel — an OPERATIONAL record, never a fact), the degraded
// evaluation of the commercial intent (the class degraded_mode with the
// five permitted path words and the honest texts; the question plan
// STAYS — questions are not promises), the sales gate against the
// degraded outcome (the machine 403 — the promises on the degraded route
// are closed), the honest promised reply (the classless timing gate: the
// word of the approved policy is REQUIRED on the degraded route, the
// foreign word never passes), the drain and the delivery evidence (the
// dispatched reply without observations stays the honest UNKNOWN — never
// the delivered; the confirmed receipt derives the confirmed state), the
// recovery revision (the route leaves the current set, the history keeps
// both rows honestly) and the control branch (the healthy route: the
// empty word passes the old path, the promise word refuses). The machine
// legs walk the replay mask and the command key conflict; the a11y close
// pins the 320px discipline, the polite live regions and the conscious
// selects (no preselect — the operator chooses).
test('cases-degradation: route failure record, degraded evaluation, promised reply, delivery evidence, recovery and the control branch at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One TEXT turn of one web_chat session: the request carries the
    // commercial interest of the demo path.
    const first = await postIngress(api, textInboundEvent('journey-degradation-evt-1', 'journey-degradation-sess-1', { say: 'сколько стоит деталь' }), 'journey-degradation-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const turnOne = (await first.json()).event_id;
    const pathA = `${casesPath}/${caseA}`;
    const caseView0 = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;
    const conversationID = caseView0.conversations[0].ConversationID;

    // 2. The classification: the Work Item enters in_work.
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Классифицировать / переклассифицировать"))').first().click();
    const classifyForm = page.locator('form[action$="/classify"]').first();
    await classifyForm.locator('select[name="type"]').selectOption('поиск детали');
    await classifyForm.locator('input[name="title"]').fill('Поиск детали на сбойном канале');
    await classifyForm.locator('input[name="reason"]').fill('Клиент спрашивает цену');
    await classifyForm.getByRole('button', { name: /Классифицировать/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?classified=1`));

    // 3. The sales prelude of the fixture world (the 2.6 pattern): the
    // mode raise and the acting disclosure of the channel.
    await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/mode/proposals`, {
      data: {
        conversation_id: conversationID,
        mode: 'sales', scope: 'conversation', expected_mode_version: 1,
        reason: 'Клиент интересуется ценой детали', signal_source: 'turn_event', turn_ref: turnOne,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-mode-1" },
    });
    const disclosure = await (await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/disclosures`, {
      data: { conversation_id: conversationID, channel: 'web_chat', text: 'Я цифровой сотрудник компании. Человек подключится по вашему запросу.' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-disc-1" },
    })).json();
    await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/disclosures/${disclosure.disclosure_id}/delivery-events`, {
      data: { conversation_id: conversationID, kind: 'sent' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-disc-sent-1" },
    });

    // 4. The empty routes panel: the honest absence (never an invented
    // default); the polite live region of the section.
    await page.goto(pathA);
    await expect(page.locator('#degradations-current')).toContainText('Деградированных маршрутов нет');
    await expect(page.locator('#degradations-panel-note')).toHaveAttribute('aria-live', 'polite');
    await expect(page.locator('#degradations-current')).toHaveAttribute('aria-live', 'polite');

    // 5. The operator records the route failure through the PRG form (the
    // connection, the channel and the cause selects carry NO preselect —
    // the empty «— выберите —» words stay until the conscious choice; the
    // note stays in the row).
    await page.locator('details:has(summary:text-is("Записать деградацию маршрута (record_route_degradation)"))').click();
    const recordForm = page.locator('form[action$="/route-degradations"]').first();
    await expect(recordForm.locator('#degradation-connection option[selected]')).toHaveCount(0);
    await expect(recordForm.locator('#degradation-channel option[selected]')).toHaveCount(0);
    await expect(recordForm.locator('#degradation-cause option[selected]')).toHaveCount(0);
    await recordForm.locator('select[name="connection_id"]').selectOption(chatConnectionID);
    await recordForm.locator('select[name="channel"]').selectOption('web_chat');
    await recordForm.locator('select[name="cause"]').selectOption('timeout');
    await recordForm.locator('textarea[name="note"]').fill('канал молчит');
    await recordForm.getByRole('button', { name: /Записать ревизию маршрута/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?route_degradation_recorded=1`));
    await page.goto(pathA);
    await expect(page.locator('#degradations-current')).toContainText('web_chat');
    await expect(page.locator('#degradations-current')).toContainText('Таймаут маршрута: канал не ответил вовремя.');
    await expect(page.locator('#degradations-current')).toContainText('(ревизия 1)');

    // 6. The machine legs of the record: the dictionary guard refuses the
    // invented cause word with the honest 400; the exact repeat of the
    // same material replays the stored outcome (200 + the honest header);
    // the same key with another material is the command key conflict
    // (409).
    const guardRecord = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/route-degradations`, {
      data: { connection_id: chatConnectionID, channel: 'web_chat', cause: 'provider_napping', note: '' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-guard-1" },
    });
    expect(guardRecord.status()).toBe(400);
    const guardProblem = await guardRecord.json();
    expect(guardProblem.message).toContain('не входит в словарь');
    const recordBody = { connection_id: chatConnectionID, channel: 'web_chat', cause: 'disconnect', note: 'второй сбой' };
    const freshRecord = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/route-degradations`, {
      data: recordBody,
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-record-2" },
    });
    expect(freshRecord.status()).toBe(201);
    const replayRecord = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/route-degradations`, {
      data: recordBody,
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-record-2" },
    });
    expect(replayRecord.status()).toBe(200);
    expect(replayRecord.headers()['idempotency-replayed']).toBe('true');
    const conflictRecord = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/route-degradations`, {
      data: { connection_id: chatConnectionID, channel: 'web_chat', cause: 'rate_limited', note: 'другой материал той же команды' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-record-2" },
    });
    expect(conflictRecord.status()).toBe(409);
    await page.goto(pathA);
    await expect(page.locator('#degradations-current')).toContainText('(ревизия 2)');
    await expect(page.locator('#degradations-current')).toContainText('Обрыв связи: подключение канала потеряно.');

    // 7. The proposal and the evaluation over the degraded route: the
    // candidate claims the answer, the mandatory questions stay — the
    // acting outcome is the degraded_mode class with the five permitted
    // path words and the honest texts (AC-3); the question plan NEVER
    // leaves the outcome (questions are not promises).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Подать предложение ИИ (порт-провод или оператор)"))').click();
    const proposalForm = page.locator('form[action$="/ai-proposals"]').first();
    await proposalForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await proposalForm.locator('select[name="work_item_id"]').selectOption(workItemID);
    await proposalForm.locator('select[name="turn_ref"]').selectOption(turnOne);
    await proposalForm.locator('input[name="intent"]').fill('price_inquiry');
    await proposalForm.locator('textarea[name="candidate_answer"]').fill('Сориентирую по описанию');
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
    const outcomePanel = page.locator(`#policy-outcome-${workItemID}`);
    await expect(outcomePanel).toContainText('degraded_mode (маршрут деградирован — обещаний нет, разрешённые пути в исходе)');
    await expect(outcomePanel).toContainText('degraded_route (маршрут диалога деградирован)');
    await expect(outcomePanel).toContainText('Разрешённые пути деградированного режима');
    await expect(outcomePanel).toContainText('accept_info_no_promise — Информация без обещания: сообщу только то, что известно, — без обещаний цены, срока и результата.');
    await expect(outcomePanel).toContainText('verified_channel_switch — Проверенный канал: продолжим на другом подключённом канале, история обращения сохранена.');
    await expect(outcomePanel).toContainText('human_handoff — Передача человеку: сотрудник примет обращение и доведёт его до результата.');
    await expect(outcomePanel).toContainText('agree_callback — Согласованный обратный звонок: сотрудник сам свяжется в обещанное окно.');
    await expect(outcomePanel).toContainText('stop_path — Останов пути: обращение закрывается без давления, возобновить можно в любой момент.');
    await expect(outcomePanel).toContainText('Обещания на деградированном маршруте закрыты: продажный ответ не выдаётся, вопросы остаются (вопросы — не обещания).');
    // The question NEVER leaves the plan (a permitted path is never a
    // replacement of the question).
    await expect(outcomePanel).toContainText('service_kind');

    // 8. The sales gate against the degraded outcome: the machine path
    // refuses the sales reply with the honest 403 of the 2-6 gate (only
    // commercial_answer opens the sales path — the promises on the
    // degraded route are closed machine-wise).
    const salesRefusal = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/client-replies`, {
      data: {
        conversation_id: conversationID,
        expected_epoch: 1,
        reply_class: 'sales',
        work_item_id: workItemID,
        content: 'Сориентирую по цене',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-reply-sales-1" },
    });
    expect(salesRefusal.status()).toBe(403);
    const salesProblem = await salesRefusal.json();
    expect(salesProblem.code).toBe('reply_policy_gate');
    expect(salesProblem.message).toContain('commercial_answer');

    // 9. The honest timing gate of the reply (classless — the short
    // failure message is neutral): the form WITHOUT the promise word
    // refuses with the honest required words (the flash alert); the
    // machine path with the foreign word refuses with the honest 400; the
    // promised reply passes the frozen ladder and the reply row carries
    // the word with the honest text.
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const replyForm = page.locator('form[action$="/client-replies"]').first();
    await expect(replyForm.locator('#reply-promise option[selected]')).toHaveCount(0);
    const promiseOptions = replyForm.locator('select[name="update_promise"] option');
    await expect(promiseOptions).toHaveCount(5); // the empty option + the four policy words
    await replyForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await replyForm.locator('select[name="reply_class"]').selectOption('neutral');
    await replyForm.locator('textarea[name="content"]').fill('Канал сбоит — продолжим');
    await replyForm.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}(\\?|$)`));
    await expect(page.locator('#case-flash')).toContainText('обязано обещать время следующего обновления');
    const foreignReply = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/client-replies`, {
      data: {
        conversation_id: conversationID,
        expected_epoch: 1,
        reply_class: 'neutral',
        update_promise: 'maybe_tomorrow',
        content: 'Канал сбоит',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-reply-foreign-1" },
    });
    expect(foreignReply.status()).toBe(400);
    const foreignProblem = await foreignReply.json();
    expect(foreignProblem.code).toBe('update_promise_foreign');
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const promisedForm = page.locator('form[action$="/client-replies"]').first();
    await promisedForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await promisedForm.locator('select[name="reply_class"]').selectOption('neutral');
    await promisedForm.locator('select[name="update_promise"]').selectOption('within_4h');
    await promisedForm.locator('textarea[name="content"]').fill('Канал сбоит — продолжим');
    await promisedForm.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?reply_prepared=1`));
    await page.goto(pathA);
    const replyTable = page.locator('div[aria-label="Исходящие ответы"]');
    await expect(replyTable).toContainText('Следующее обновление — в течение четырёх часов.');

    // 10. The drain: the prepared reply leaves (the 2-4 record of the
    // record); the honest delivery state of the dispatched reply without
    // any observation is the UNKNOWN — never the delivered.
    const drain = await api.post(`/v1/tenants/${tenantID}/cases/client-replies/drain`, { data: {} });
    expect(drain.status()).toBe(200);
    const drainResult = await drain.json();
    expect(drainResult.dispatched).toBeGreaterThanOrEqual(1);
    await page.goto(pathA);
    await expect(replyTable).toContainText('неизвестна (квитанции нет)');

    // 11. The delivery evidence: the operator relays the confirmed
    // receipt of the connector through the PRG form (the reply select
    // carries NO preselect; the kind select is the fixed dictionary) —
    // the delivery column derives the confirmed state with the timeline
    // anchor of the observation.
    await page.locator('details:has(summary:text-is("Записать наблюдение доставки (record_delivery_evidence)"))').click();
    const evidenceForm = page.locator('form[action$="/delivery-evidence"]').first();
    const replyOption = await evidenceForm.locator('select[name="reply_id"] option').nth(1).getAttribute('value');
    await expect(evidenceForm.locator('#evidence-reply option[selected]')).toHaveCount(0);
    await expect(evidenceForm.locator('#evidence-kind option[selected]')).toHaveCount(0);
    await evidenceForm.locator('select[name="reply_id"]').selectOption(replyOption!);
    await evidenceForm.locator('select[name="kind"]').selectOption('delivered_confirmed');
    await evidenceForm.getByRole('button', { name: /Записать наблюдение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?delivery_evidence_recorded=1`));
    await page.goto(pathA);
    await expect(replyTable).toContainText('подтверждена (квитанция есть)');
    await expect(replyTable).toContainText('Доставка подтверждена: квитанция канала получена.');
    // The machine leg: the same evidence command replays; the same key
    // with another kind word is the command key conflict.
    const evidenceBody = { reply_id: replyOption, kind: 'delivery_unknown' };
    const evidenceMachine = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/replies/${replyOption}/delivery-evidence`, {
      data: evidenceBody,
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-evidence-2" },
    });
    expect(evidenceMachine.status()).toBe(201);
    const evidenceReplay = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/replies/${replyOption}/delivery-evidence`, {
      data: evidenceBody,
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-evidence-2" },
    });
    expect(evidenceReplay.status()).toBe(200);
    expect(evidenceReplay.headers()['idempotency-replayed']).toBe('true');
    const evidenceConflict = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/replies/${replyOption}/delivery-evidence`, {
      data: { reply_id: replyOption, kind: 'provider_failed' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-evidence-2" },
    });
    expect(evidenceConflict.status()).toBe(409);

    // 12. The recovery revision: the operator records the cleared
    // revision of the same route — the route leaves the current set; the
    // history keeps BOTH rows honestly (the append-only truth, the
    // recovery row shows the honest word).
    await page.locator('details:has(summary:text-is("Записать деградацию маршрута (record_route_degradation)"))').click();
    const clearForm = page.locator('form[action$="/route-degradations"]').first();
    await clearForm.locator('select[name="connection_id"]').selectOption(chatConnectionID);
    await clearForm.locator('select[name="channel"]').selectOption('web_chat');
    await clearForm.locator('select[name="cause"]').selectOption('disconnect');
    await clearForm.locator('input[name="cleared"]').check();
    await clearForm.locator('textarea[name="note"]').fill('канал ожил');
    await clearForm.getByRole('button', { name: /Записать ревизию маршрута/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?route_degradation_recorded=1`));
    await page.goto(pathA);
    await expect(page.locator('#degradations-current')).toContainText('Деградированных маршрутов нет');
    const history = page.locator('details:has(summary:text-is("История ревизий (3) — append-only, включая восстановления"))');
    await history.click();
    const historyTable = history.getByRole('table');
    await expect(historyTable).toContainText('восстановлен (маршрут покидает деградированные)');
    await expect(historyTable).toContainText('канал молчит');
    await expect(historyTable).toContainText('канал ожил');

    // 13. The control branch: the recovered (healthy) route — the reply
    // WITHOUT the promise word passes the old path; the MACHINE path
    // carrying the promise word refuses with the honest 400 (the field
    // has exactly one meaning).
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const controlForm = page.locator('form[action$="/client-replies"]').first();
    await controlForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await controlForm.locator('select[name="reply_class"]').selectOption('neutral');
    await controlForm.locator('textarea[name="content"]').fill('Канал восстановлен — отвечаем прежним путём');
    await controlForm.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?reply_prepared=1`));
    const controlRefusal = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/client-replies`, {
      data: {
        conversation_id: conversationID,
        expected_epoch: 1,
        reply_class: 'neutral',
        update_promise: 'within_1h',
        content: 'Ответ с лишним словом обещания',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-degradation-reply-control-1" },
    });
    expect(controlRefusal.status()).toBe(400);
    const controlProblem = await controlRefusal.json();
    expect(controlProblem.code).toBe('update_promise_foreign');
    expect(controlProblem.message).toContain('маршрут не деградирован');

    // 14. The a11y close of the operator page: the 320px single column
    // (no horizontal scroll), the polite live regions of the routes
    // panel, the conscious selects (no preselect — neither the record
    // connection/channel/cause nor the evidence selects nor the promise
    // select) and no autofocus of the primary actions.
    await page.goto(pathA);
    await expect(page.locator('#degradation-connection option[selected]')).toHaveCount(0);
    await expect(page.locator('#degradation-channel option[selected]')).toHaveCount(0);
    await expect(page.locator('#degradation-cause option[selected]')).toHaveCount(0);
    await expect(page.locator('#reply-promise option[selected]')).toHaveCount(0);
    await expect(page.locator('#degradations-panel-note')).toHaveAttribute('aria-live', 'polite');
    await expect(page.locator('#degradations-current')).toHaveAttribute('aria-live', 'polite');
    const recordButtons = page.locator('form[action$="/route-degradations"] button');
    await expect(recordButtons).not.toHaveAttribute('autofocus');
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    await page.close();
  }
});
