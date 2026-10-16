import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const chatConnectionID = '01994f64-6c00-7000-8000-000000000016';
const ownerID = '01994f64-6c00-7000-8000-000000000001';
const secondOperatorID = '01994f64-6c00-7000-8000-00000000000a';
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

// The Story 2.13 chain at 320px: the unconditional human ask of the
// web_chat client («позовите человека» — the turn anchor of the accepted
// client event, the reason is NEVER an operator input), the machine
// raise of the human_requested mode (the ratchet term through the REAL
// domain constructor), the honest panel words («запрошен» with the
// client status two-words discipline — назначение ≠ принятие), the
// freeze halves (the sales reply is closed by the mode ladder, the
// commitment creation is frozen with the human_requested word), the
// assignment round (the assignee card BEFORE the acceptance — the owner
// NEVER moves), the race of the two confirmations (the stale CAS leg,
// the honest acceptance composing the 2.4 machinery — the epoch +1 human
// owner, the raced twin after the winner with the honest state word, the
// replay of the stored outcome), the neutral status message of the NEW
// owner (the former path), the return to Staffy (the staffy owner with
// the external inbox surface — the sales path never resumes), the
// control branch (the client cancels the ask BEFORE the acceptance —
// the ownership never moved) and the a11y close (the polite live
// regions, the conscious selects without preselect, the 320px column).
test('cases-human-handoff: unconditional ask, mode raise, freeze, assignment, confirmation race, return and the client cancellation at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One TEXT turn of one web_chat session: the client asks for a
    // human — the turn is the anchor of the unconditional ask.
    const first = await postIngress(api, textInboundEvent('journey-handoff-evt-1', 'journey-handoff-sess-1', { say: 'позовите человека' }), 'journey-handoff-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const turnOne = (await first.json()).event_id;
    const pathA = `${casesPath}/${caseA}`;
    const caseView0 = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;
    const conversationID = caseView0.conversations[0].ConversationID;

    // 2. The classification: the Work Item enters in_work (the honest
    // material of the commitment leg).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Классифицировать / переклассифицировать"))').first().click();
    const classifyForm = page.locator('form[action$="/classify"]').first();
    await classifyForm.locator('select[name="type"]').selectOption('поиск детали');
    await classifyForm.locator('input[name="title"]').fill('Поиск специалиста для клиента');
    await classifyForm.locator('input[name="reason"]').fill('Клиент просит человека');
    await classifyForm.getByRole('button', { name: /Классифицировать/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?classified=1`));

    // 3. The unconditional ask through the PRG form (the reason is NOT a
    // field: the client ask is unconditional; the turn select and the
    // deadline select carry NO preselect — the conscious choice; the
    // role checkboxes carry the fixed dictionary).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Запросить человека (request_human — безусловная просьба клиента)"))').click();
    const askForm = page.locator('form[action$="/human-requests"]').first();
    await expect(askForm.locator('#handoff-turn option[selected]')).toHaveCount(0);
    await expect(askForm.locator('#handoff-deadline option[selected]')).toHaveCount(0);
    await askForm.locator('select[name="turn_ref"]').selectOption(turnOne);
    await askForm.locator('input[name="target_roles"][value="operator"]').check();
    await askForm.locator('select[name="ack_deadline"]').selectOption('ack_1h');
    await askForm.getByRole('button', { name: /Записать просьбу клиента/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?human_requested_recorded=1`));

    // 4. The panel: the ask is registered with the honest words; the
    // mode badge carries the raised word (the ratchet term landed).
    await page.goto(pathA);
    await expect(page.locator('#handoffs-current')).toContainText('Человек запрошен: ожидание назначения.');
    await expect(page.locator('#handoffs-current')).toContainText('слово: requested');
    await expect(page.locator('#handoffs-current')).toContainText('Запрос человека зарегистрирован: специалист будет назначен.');
    await expect(page.locator('#handoffs-current')).toContainText('слово: human_requested');
    await expect(page.locator('#mode-badge')).toContainText('human_requested (запрошен человек)');
    const askRow = page.locator('details:has(summary:text-is("Запрос №1 — Человек запрошен: ожидание назначения. (открыт)"))');
    await askRow.click();
    await expect(askRow).toContainText('Безусловная просьба клиента');
    await expect(askRow).toContainText('Целевые роли: operator');
    await expect(askRow).toContainText(turnOne);

    // 5. The machine legs of the ask: the repeat ask with another key
    // refuses with the one-competing-task 409 (the CURRENT lifecycle
    // words — AC-4); the raced mode CAS twin (the stale ladder
    // expectation) refuses with the honest mode conflict.
    const repeatAsk = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests`, {
      data: {
        turn_ref: turnOne,
        expected_mode_version: 2,
        target_roles: ['operator'],
        ack_deadline: 'ack_1h',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-handoff-repeat-1" },
    });
    expect(repeatAsk.status()).toBe(409);
    const repeatProblem = await repeatAsk.json();
    expect(repeatProblem.code).toBe('human_active');
    expect(repeatProblem.field_errors.state).toBe('requested');
    expect(repeatProblem.field_errors.client_status).toBe('human_requested');
    const racedAsk = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests`, {
      data: {
        turn_ref: turnOne,
        expected_mode_version: 1,
        target_roles: ['operator'],
        ack_deadline: 'ack_1h',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-handoff-stale-1" },
    });
    expect(racedAsk.status()).toBe(409);
    const racedProblem = await racedAsk.json();
    expect(racedProblem.code).toBe('mode_conflict');
    expect(racedProblem.field_errors.active_mode).toBe('human_requested');

    // 6. The freeze halves: the sales reply is closed by the mode ladder
    // (the machine 403 with the honest mode words); the commitment
    // creation is frozen (the honest 400 with the human_requested word —
    // AC-2).
    const salesRefusal = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/client-replies`, {
      data: {
        conversation_id: conversationID,
        expected_epoch: 1,
        reply_class: 'sales',
        work_item_id: workItemID,
        content: 'Продам деталь со скидкой',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-handoff-reply-sales-1" },
    });
    expect(salesRefusal.status()).toBe(403);
    const salesProblem = await salesRefusal.json();
    expect(salesProblem.code).toBe('reply_mode_gate');
    expect(salesProblem.field_errors.active_mode).toBe('human_requested');
    expect(salesProblem.message).toContain('Продажные ответы недоступны');
    await page.goto(`${pathA}/work-items/${workItemID}/commitments`);
    const commitmentForm = page.locator('form.commitment-form[action$="/commitments"]').first();
    await commitmentForm.locator('select[name="status"]').selectOption('draft');
    await commitmentForm.locator('input[name="scope"]').fill('Замена детали под контролем человека');
    await commitmentForm.locator('textarea[name="service_items"]').fill('деталь 1 шт');
    await commitmentForm.locator('select[name="price_kind"]').selectOption('amount');
    await commitmentForm.locator('input[name="price_amount_cents"]').fill('300000');
    await commitmentForm.locator('input[name="payee"]').fill('ООО «Автозапчасть»');
    await commitmentForm.locator('input[name="warranty_role"]').fill('гарантия на работы 6 месяцев');
    await commitmentForm.locator('input[name="next_step"]').fill('запись на сервис');
    await commitmentForm.locator('input[name="consequences"]').fill('без изменений');
    await commitmentForm.locator('input[name="action_prepare_order"]').check();
    await commitmentForm.locator('input[name="valid_until"]').fill('2026-09-17T12:00');
    await commitmentForm.locator('input[name="consumption_limit"]').fill('1');
    await commitmentForm.getByRole('button', { name: /Создать версию/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}/work-items/${workItemID}/commitments(\\?|$)`));
    const freezeFlash = page.locator('#commitment-flash');
    await expect(freezeFlash).toContainText('заморожено');
    await expect(freezeFlash).toBeFocused();

    // 7. The assignment round through the PRG form (the assignee and the
    // backup selects carry the member options — NO preselect; the role
    // of the assignee is NEVER a form field: the server resolves it
    // from the membership truth). The owner NEVER moves (назначение ≠
    // принятие): the ownership panel keeps the operator owner and the
    // epoch 1.
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Назначить специалиста (assign_human_handoff — назначение ≠ принятие)"))').click();
    const assignForm = page.locator('form[action$="/human-requests/assignment"]').first();
    await expect(assignForm.locator('#handoff-assignee option[selected]')).toHaveCount(0);
    await expect(assignForm.locator('#handoff-backup option[selected]')).toHaveCount(0);
    await expect(assignForm.locator('#handoff-round-deadline option[selected]')).toHaveCount(0);
    await assignForm.locator('select[name="assignee"]').selectOption(secondOperatorID);
    await assignForm.locator('select[name="backup"]').selectOption(ownerID);
    // The round deadline word (ack_1h — the fixture view clock runs 30
    // minutes past the command clock: the 15-minute word would derive the
    // overdue state honestly, the 1-hour word keeps the assigned word —
    // the state the chain walks).
    await assignForm.locator('select[name="ack_deadline"]').selectOption('ack_1h');
    await assignForm.getByRole('button', { name: /Назначить раунд/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?human_handoff_assigned=1`));
    await page.goto(pathA);
    await expect(page.locator('#handoffs-current')).toContainText('Назначен: ожидание подтверждения специалистом.');
    await expect(page.locator('#handoffs-current')).toContainText('слово: assigned');
    await expect(page.locator('#handoffs-current')).toContainText('Специалист назначен и ещё не подтвердил принятие обращения.');
    await expect(page.locator('#handoffs-current')).toContainText('слово: human_assigned');
    const ownershipPanel = page.locator('section[aria-label="Владение обращением"]');
    await expect(ownershipPanel).toContainText('Epoch: 1');
    await expect(ownershipPanel).toContainText('owner@example.org');

    // 8. The assignee's card: the second operator logs in — the card
    // «Передача ждёт вашего подтверждения» with the honest deadline
    // (the assignment is NOT the acceptance — AC-3).
    await api.post('/fixture/operator', { form: { second: '1' } });
    await page.goto(pathA);
    const card = page.locator('#handoff-ack-card');
    await expect(card).toContainText('Передача ждёт вашего подтверждения');
    await expect(card).toContainText('Назначение — не принятие');
    await expect(card.locator('form[action$="/human-requests/acknowledgement"]')).toHaveCount(1);

    // 9. The race of the two confirmations: the STALE CAS leg first (the
    // twin with the foreign epoch expectation loses with the honest
    // 409 while the round is still assigned), then the honest
    // acceptance lands (the ONE transaction of the 2.4 machinery: the
    // epoch +1, the human owner, the inbox surface), the raced twin
    // AFTER the winner with the same epoch expectation refuses with the
    // honest state word (the lifecycle is already acknowledged), and
    // the exact repeat of the winner's key replays the stored outcome.
    const staleTwin = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests/acknowledgement`, {
      data: { expected_epoch: 7 },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-handoff-ack-stale-1" },
    });
    expect(staleTwin.status()).toBe(409);
    expect((await staleTwin.json()).code).toBe('stale_version');
    const winner = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests/acknowledgement`, {
      data: { expected_epoch: 1 },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-handoff-ack-1" },
    });
    expect(winner.status()).toBe(201);
    const winnerOutcome = await winner.json();
    expect(winnerOutcome.command).toBe('acknowledge_human_handoff');
    expect(winnerOutcome.epoch).toBe(2);
    expect(winnerOutcome.handoff_assignee).toBe(secondOperatorID);
    expect(winnerOutcome.transfer_request_id).toBeTruthy();
    const racedTwin = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests/acknowledgement`, {
      data: { expected_epoch: 1 },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-handoff-ack-race-1" },
    });
    expect(racedTwin.status()).toBe(400);
    const racedAck = await racedTwin.json();
    expect(racedAck.code).toBe('acknowledge_human_handoff');
    expect(racedAck.message).toContain('текущее состояние — «acknowledged»');
    const replay = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests/acknowledgement`, {
      data: { expected_epoch: 1 },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-handoff-ack-1" },
    });
    expect(replay.status()).toBe(200);
    expect(replay.headers()['idempotency-replayed']).toBe('true');
    const replayOutcome = await replay.json();
    expect(replayOutcome.epoch).toBe(2);

    // 10. The accepted panel: «принято» with the client word
    // (human_acknowledged — the specialist DID take the case), the
    // ownership moved to the HUMAN owner with the epoch 2 (the card is
    // gone — nothing waits for the confirmation).
    await page.goto(pathA);
    await expect(page.locator('#handoffs-current')).toContainText('Принято: специалист подтвердил передачу.');
    await expect(page.locator('#handoffs-current')).toContainText('Специалист подтвердил принятие обращения.');
    await expect(page.locator('#handoffs-current')).toContainText('слово: human_acknowledged');
    await expect(page.locator('#handoff-ack-card')).toHaveCount(0);
    await expect(ownershipPanel).toContainText('Epoch: 2');
    await expect(ownershipPanel).toContainText('backup@example.org');

    // 11. The NEW owner answers the client with the neutral status
    // message (the former path — the short status message is never a
    // promise; the sales class stays closed by the ladder).
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const replyForm = page.locator('form[action$="/client-replies"]').first();
    await replyForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await replyForm.locator('select[name="reply_class"]').selectOption('neutral');
    await replyForm.locator('textarea[name="content"]').fill('Специалист принял обращение и разбирается.');
    await replyForm.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?reply_prepared=1`));

    // 12. The return to Staffy (the owner-human decision with the result
    // word — the select carries NO preselect): the ONE transaction of
    // the 2.4 machinery (the staffy owner, the external inbox surface,
    // the epoch +1); the mode ladder NEVER lowers (the sales path never
    // resumes — the staffy owner gate answers the machine leg).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Вернуть обращение Staffy (return_to_staffy — после подтверждения)"))').click();
    const returnForm = page.locator('form[action$="/human-requests/return"]').first();
    await expect(returnForm.locator('#handoff-result option[selected]')).toHaveCount(0);
    await returnForm.locator('select[name="result"]').selectOption('issue_resolved');
    await returnForm.getByRole('button', { name: /Вернуть цифровому сотруднику/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?human_handoff_returned=1`));
    await page.goto(pathA);
    await expect(page.locator('#handoffs-current')).toContainText('Возвращено: человек вернул обращение цифровому сотруднику.');
    await expect(page.locator('#handoffs-current')).toContainText('Специалист подтвердил принятие обращения.');
    await expect(ownershipPanel).toContainText('Epoch: 3');
    await expect(ownershipPanel).toContainText('цифровой сотрудник');
    await expect(page.locator('#mode-badge')).toContainText('human_requested (запрошен человек)');
    const afterReturn = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/client-replies`, {
      data: {
        conversation_id: conversationID,
        expected_epoch: 3,
        reply_class: 'sales',
        work_item_id: workItemID,
        content: 'Всё же продам деталь',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-handoff-reply-sales-2" },
    });
    expect(afterReturn.status()).toBe(403);
    const afterReturnProblem = await afterReturn.json();
    expect(afterReturnProblem.code).toBe('reply_gate');
    expect(afterReturnProblem.field_errors.current_owner_kind).toBe('staffy');

    // 13. The control branch: the client cancels the ask BEFORE the
    // acceptance — the second client turn is the anchor of the
    // cancellation; the ownership NEVER moved (the epoch 1 owner stays
    // the asking operator); the history keeps the request and both
    // events honestly (append-only — never a deletion).
    await api.post('/fixture/operator', { form: { second: '' } });
    const controlAsk = await postIngress(api, textInboundEvent('journey-handoff-evt-2', 'journey-handoff-sess-2', { say: 'позовите человека скорее' }), 'journey-handoff-key-2');
    expect(controlAsk.status()).toBe(201);
    const caseB = (await controlAsk.json()).case_id;
    const controlTurn = (await controlAsk.json()).event_id;
    const pathB = `${casesPath}/${caseB}`;
    const caseViewB = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseB}`)).json();
    const controlConversation = caseViewB.conversations[0].ConversationID;
    const controlWorkItem = caseViewB.work_items[0].WorkItemID;
    const askB = await api.post(`/v1/tenants/${tenantID}/cases/${caseB}/human-requests`, {
      data: {
        turn_ref: controlTurn,
        expected_mode_version: 1,
        target_roles: ['operator'],
        ack_deadline: 'ack_1h',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-handoff-b-ask-1" },
    });
    expect(askB.status()).toBe(201);
    const assignB = await api.post(`/v1/tenants/${tenantID}/cases/${caseB}/human-requests/assignment`, {
      data: {
        assignee: secondOperatorID,
        backup: ownerID,
        ack_deadline: 'ack_1h',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-handoff-b-assign-1" },
    });
    expect(assignB.status()).toBe(201);
    const cancelTurnEvent = await postIngress(api, textInboundEvent('journey-handoff-evt-3', 'journey-handoff-sess-2', { say: 'передумал, не надо человека' }), 'journey-handoff-key-3');
    expect(cancelTurnEvent.status()).toBe(201);
    const cancelTurn = (await cancelTurnEvent.json()).event_id;
    await page.goto(pathB);
    await page.locator('details:has(summary:text-is("Отменить запрос клиента (cancel_human_request — якорь отмены)"))').click();
    const cancelForm = page.locator('form[action$="/human-requests/cancellation"]').first();
    await expect(cancelForm.locator('#handoff-cancel-turn option[selected]')).toHaveCount(0);
    await cancelForm.locator('select[name="turn_ref"]').selectOption(cancelTurn);
    await cancelForm.getByRole('button', { name: /Записать отмену/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseB}\\?human_request_cancelled=1`));
    await page.goto(pathB);
    await expect(page.locator('#handoffs-current')).toContainText('Отменено: клиент отменил запрос человека.');
    await expect(page.locator('#handoffs-current')).toContainText('Запрос человека отменён.');
    await expect(page.locator('#handoffs-current')).toContainText('слово: human_cancelled');
    const controlOwnership = page.locator('section[aria-label="Владение обращением"]');
    await expect(controlOwnership).toContainText('Epoch: 1');
    await expect(controlOwnership).toContainText('owner@example.org');
    const cancelRow = page.locator('details:has(summary:text-is("Запрос №1 — Отменено: клиент отменил запрос человека. (закрыт)"))');
    await cancelRow.click();
    await expect(cancelRow).toContainText('Отмена клиентом: запрос человека закрыт.');

    // 14. The a11y close of the operator page: the polite live regions
    // of the handoff panel, the conscious selects (no preselect — the
    // turn, the assignee, the backup, the round deadline and the return
    // result), no autofocus of the primary actions and the 320px single
    // column (no horizontal scroll).
    await page.goto(pathA);
    await expect(page.locator('#handoffs-panel-note')).toHaveAttribute('aria-live', 'polite');
    await expect(page.locator('#handoffs-current')).toHaveAttribute('aria-live', 'polite');
    await page.locator('details:has(summary:text-is("Запросить человека (request_human — безусловная просьба клиента)"))').click();
    await expect(page.locator('#handoff-turn option[selected]')).toHaveCount(0);
    await expect(page.locator('#handoff-deadline option[selected]')).toHaveCount(0);
    await page.locator('details:has(summary:text-is("Назначить специалиста (assign_human_handoff — назначение ≠ принятие)"))').click();
    await expect(page.locator('#handoff-assignee option[selected]')).toHaveCount(0);
    await expect(page.locator('#handoff-backup option[selected]')).toHaveCount(0);
    await expect(page.locator('#handoff-round-deadline option[selected]')).toHaveCount(0);
    await page.locator('details:has(summary:text-is("Вернуть обращение Staffy (return_to_staffy — после подтверждения)"))').click();
    await expect(page.locator('#handoff-result option[selected]')).toHaveCount(0);
    const askButtons = page.locator('form[action$="/human-requests"] button');
    await expect(askButtons.first()).not.toHaveAttribute('autofocus');
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    await page.close();
  }
});
