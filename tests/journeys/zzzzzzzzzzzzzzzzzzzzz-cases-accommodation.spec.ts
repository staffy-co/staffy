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

// The Story 2.11 chain at 320px: the recorded communication preferences
// (the client words «не могу отправлять фото» become the append-only
// revision rows of the «Предпочтения связи» panel — an OPERATIONAL
// setting, never a fact), the honest alternatives of the Policy Outcome
// (the missing mandatory media-usual fact + the active no_photo → the
// outcome carries the four pack words with the fixed limitation texts;
// the question STAYS ask — an alternative never replaces the question),
// the accuracy qualifier gate of the sales reply (the required word of
// the outcome's active set — the absence of media never becomes an
// invented exact result), the transfer card (the current preferences
// are visible to the receiving operator BEFORE the ack), the clearing
// revision (the key leaves the current set, the history keeps both rows
// honestly), the honest supersede (the re-evaluation over the cleared
// preferences carries no alternatives) and the control branch (the
// qualifier of the alternatives-free outcome refuses with the foreign
// word; the empty word passes the old path). The machine legs walk the
// replay mask and the command key conflict of the record command; the
// a11y close pins the 320px discipline, the polite live regions and the
// conscious selects (no preselect — the operator chooses).
test('cases-accommodation: recorded preferences, honest alternatives, qualified reply, transfer card, cleared revision and the control branch at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. Two TEXT turns of one web_chat session: the request carries the
    // client words of the demo path («не могу отправлять фото»).
    const first = await postIngress(api, textInboundEvent('journey-accommodation-evt-1', 'journey-accommodation-sess-1', { say: 'не могу отправлять фото, сколько стоит деталь' }), 'journey-accommodation-key-1');
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
    await classifyForm.locator('input[name="title"]').fill('Поиск детали без фото');
    await classifyForm.locator('input[name="reason"]').fill('Клиент не может присылать фото');
    await classifyForm.getByRole('button', { name: /Классифицировать/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?classified=1`));

    // 3. The sales prelude of the fixture world (the 2.6 pattern): the
    // mode raise and the acting disclosure of the channel. The
    // service_kind fact is NEVER confirmed here — the mandatory
    // media-usual fact stays missing (the trigger pair of the row 10).
    await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/mode/proposals`, {
      data: {
        conversation_id: conversationID,
        mode: 'sales', scope: 'conversation', expected_mode_version: 1,
        reason: 'Клиент интересуется ценой детали', signal_source: 'turn_event', turn_ref: turnOne,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-accommodation-mode-1" },
    });
    const disclosure = await (await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/disclosures`, {
      data: { conversation_id: conversationID, channel: 'web_chat', text: 'Я цифровой сотрудник компании. Человек подключится по вашему запросу.' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-accommodation-disc-1" },
    })).json();
    await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/disclosures/${disclosure.disclosure_id}/delivery-events`, {
      data: { conversation_id: conversationID, kind: 'sent' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-accommodation-disc-sent-1" },
    });

    // 4. The empty panel: the honest absence (never an invented default).
    await page.goto(pathA);
    await expect(page.locator('#accommodations-current')).toContainText('Записанных предпочтений нет');
    await expect(page.locator('#accommodations-panel-note')).toHaveAttribute('aria-live', 'polite');
    await expect(page.locator('#accommodations-current')).toHaveAttribute('aria-live', 'polite');

    // 5. The operator records no_photo through the PRG form (the key
    // select carries NO preselect — the empty «— выберите ключ —» word
    // stays until the conscious choice; the note stays in the row).
    await page.locator('details:has(summary:text-is("Записать предпочтение связи (record_accommodation)"))').click();
    const recordForm = page.locator('form[action$="/accommodations"]').first();
    await expect(recordForm.locator('#accommodation-key option[selected]')).toHaveCount(0);
    await recordForm.locator('select[name="accommodation_key"]').selectOption('no_photo');
    await recordForm.locator('textarea[name="note"]').fill('клиент сказал: не могу фото');
    await recordForm.getByRole('button', { name: /Записать ревизию/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?accommodation_recorded=1`));
    await page.goto(pathA);
    await expect(page.locator('#accommodations-current')).toContainText('no_photo — клиент не может присылать фото');
    await expect(page.locator('#accommodations-current')).toContainText('(ревизия 1)');
    // The value-guard leg of the form path: the flag key with a value
    // refuses with the honest dictionary words (the flash alert).
    await page.locator('details:has(summary:text-is("Записать предпочтение связи (record_accommodation)"))').click();
    const guardForm = page.locator('form[action$="/accommodations"]').first();
    await guardForm.locator('select[name="accommodation_key"]').selectOption('no_web');
    await guardForm.locator('input[name="value"]').fill('yes');
    await guardForm.getByRole('button', { name: /Записать ревизию/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}(\\?|$)`));
    await expect(page.locator('#case-flash')).toContainText('не несёт значение');
    await page.goto(pathA);
    await expect(page.locator('#accommodations-current')).not.toContainText('no_web');

    // 6. The machine path of the valued keys: the preferred channel word
    // and the availability line land through the JSON contract (201);
    // the exact repeat replays the stored outcome (200 + the honest
    // header); the same key with another material is the command key
    // conflict (409).
    const recordBody = { accommodation_key: 'preferred_channel', value: 'phone', note: 'клиент просит звонить' };
    const freshRecord = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/accommodations`, {
      data: recordBody,
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-accommodation-record-2" },
    });
    expect(freshRecord.status()).toBe(201);
    const replayRecord = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/accommodations`, {
      data: recordBody,
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-accommodation-record-2" },
    });
    expect(replayRecord.status()).toBe(200);
    expect(replayRecord.headers()['idempotency-replayed']).toBe('true');
    const conflictRecord = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/accommodations`, {
      data: { accommodation_key: 'preferred_channel', value: 'email', note: 'другой материал той же команды' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-accommodation-record-2" },
    });
    expect(conflictRecord.status()).toBe(409);
    const timeRecord = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/accommodations`, {
      data: { accommodation_key: 'available_time', value: 'звоните после 18:00' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-accommodation-record-3" },
    });
    expect(timeRecord.status()).toBe(201);
    await page.goto(pathA);
    await expect(page.locator('#accommodations-current')).toContainText('preferred_channel — предпочтительный канал связи = phone');
    await expect(page.locator('#accommodations-current')).toContainText('available_time — доступное время для связи = звоните после 18:00');

    // 7. The proposal and the evaluation: the candidate claims the
    // answer, the service_kind question stays (the rule table mandatory
    // key of the media-usual modality is NOT confirmed) — the recorded
    // no_photo fires the honest alternatives of the outcome.
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
    await expect(page.locator('div[aria-live="polite"]').filter({ hasText: /commercial_answer/ })).toBeVisible();
    // The honest alternatives of the outcome: the four pack words with
    // the fixed limitation texts; the qualifier duty words (AC-11).
    const outcomePanel = page.locator(`#policy-outcome-${workItemID}`);
    await expect(outcomePanel).toContainText('Активные альтернативы (честные пути без media)');
    await expect(outcomePanel).toContainText('text_description — точность ниже фото');
    await expect(outcomePanel).toContainText('Точность ниже фото: определение по текстовому описанию может ошибаться — итог подтверждает человек.');
    await expect(outcomePanel).toContainText('range_estimate — только диапазон, не точный результат');
    await expect(outcomePanel).toContainText('callback — уточнение голосом с человеком');
    await expect(outcomePanel).toContainText('human_handoff — продолжит человек');
    await expect(outcomePanel).toContainText('Продолжит человек: сотрудник примет обращение и доведёт его до результата.');
    await expect(outcomePanel).toContainText('Продажный ответ обязан заявить одно из этих ограничений точности');
    // The question NEVER leaves the plan (an alternative is a permitted
    // path, never a replacement of the question).
    await expect(outcomePanel).toContainText('service_kind');

    // 8. The accuracy gate of the sales reply: the form WITHOUT the
    // qualifier refuses with the honest required words (the flash alert
    // carries the active set); the select carries no preselect — the
    // empty word stays until the conscious choice. The qualified reply
    // passes the frozen ladder and the row carries the word.
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const replyForm = page.locator('form[action$="/client-replies"]').first();
    await expect(replyForm.locator('#reply-qualifier option[selected]')).toHaveCount(0);
    const qualifierOptions = replyForm.locator('select[name="accuracy_qualifier"] option');
    await expect(qualifierOptions).toHaveCount(5); // the empty option + the four active words
    await replyForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await replyForm.locator('select[name="reply_class"]').selectOption('sales');
    await replyForm.locator('select[name="work_item_id"]').selectOption(workItemID);
    await replyForm.locator('textarea[name="content"]').fill('Сориентирую по описанию: точность ниже фото, итог подтвердит человек');
    await replyForm.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}(\\?|$)`));
    // The form flash names the rule (the one-shot cookie bound refuses
    // the full reason with the active-set listing — the words live in
    // the qualifier select and the outcome panel).
    await expect(page.locator('#case-flash')).toContainText('обязан заявить ограничение точности');
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const qualifiedForm = page.locator('form[action$="/client-replies"]').first();
    await qualifiedForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await qualifiedForm.locator('select[name="reply_class"]').selectOption('sales');
    await qualifiedForm.locator('select[name="work_item_id"]').selectOption(workItemID);
    await qualifiedForm.locator('select[name="accuracy_qualifier"]').selectOption('text_description');
    await qualifiedForm.locator('textarea[name="content"]').fill('Сориентирую по описанию: точность ниже фото, итог подтвердит человек');
    await qualifiedForm.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?reply_prepared=1`));
    await page.goto(pathA);
    const replyTable = page.locator('div[aria-label="Исходящие ответы"]');
    await expect(replyTable).toContainText('text_description — точность ниже фото');

    // 9. The transfer card: the pending request shows the current
    // preferences to the RECEIVING operator BEFORE the ack (AC-1
    // «передаются каждому последующему Response Owner»); the second
    // operator acknowledges — the epoch grows to 2.
    await page.locator('details:has(summary:text-is("Запросить передачу (дедлайн — из human coverage)"))').click();
    const transferForm = page.locator('form[action$="/ownership/transfer-requests"]').first();
    const targetOption = await transferForm.locator('select[name="to_owner_ref"] option').nth(1).getAttribute('value');
    await transferForm.locator('select[name="to_owner_ref"]').selectOption(targetOption!);
    await transferForm.locator('select[name="writable_surface"]').selectOption('inbox');
    await transferForm.locator('input[name="reason"]').fill('Передача второму оператору');
    await transferForm.getByRole('button', { name: /Запросить передачу/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?transfer_requested=1`));
    await page.goto(pathA);
    const transferCard = page.locator('div[aria-label="Запрос передачи"]');
    await expect(transferCard).toBeVisible();
    await expect(page.getByText('Предпочтения связи клиента (видны принимающему до подтверждения)')).toBeVisible();
    await expect(page.getByText('Предпочтения связи клиента (видны принимающему до подтверждения)')).toContainText('no_photo — клиент не может присылать фото');
    await expect(page.getByText('Предпочтения связи клиента (видны принимающему до подтверждения)')).toContainText('preferred_channel — предпочтительный канал связи = phone');
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

    // 10. The clearing revision (the receiving operator records it — the
    // preferences travelled with the case): the client learned to send
    // photos — the no_photo leaves the current set; the history keeps
    // BOTH rows honestly (the append-only truth, the cleared row shows
    // the honest word).
    await page.locator('details:has(summary:text-is("Записать предпочтение связи (record_accommodation)"))').click();
    const clearForm = page.locator('form[action$="/accommodations"]').first();
    await clearForm.locator('select[name="accommodation_key"]').selectOption('no_photo');
    await clearForm.locator('input[name="cleared"]').check();
    await clearForm.locator('textarea[name="note"]').fill('клиент научился присылать фото');
    await clearForm.getByRole('button', { name: /Записать ревизию/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?accommodation_recorded=1`));
    await page.goto(pathA);
    await expect(page.locator('#accommodations-current')).not.toContainText('no_photo');
    await expect(page.locator('#accommodations-current')).toContainText('preferred_channel — предпочтительный канал связи = phone');
    const history = page.locator('details:has(summary:text-is("История ревизий (4) — append-only, включая снятия"))');
    await history.click();
    const historyTable = history.getByRole('table');
    await expect(historyTable).toContainText('снято (ключ покидает текущий набор)');
    await expect(historyTable).toContainText('клиент научился присылать фото');
    await expect(historyTable).toContainText('клиент сказал: не могу фото');

    // 11. The honest supersede: the re-evaluation of the SAME proposal
    // over the cleared preferences (the fresh command key of the machine
    // path) → the outcome carries NO alternatives; the number grows; the
    // outcome history keeps the predecessor.
    const freshView = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const proposalRef = await page.locator('form[action$="/policy/evaluations"] select[name="proposal_ref"] option').first().getAttribute('value');
    const superseded = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/policy/evaluations`, {
      data: {
        proposal_ref: proposalRef,
        expected_mode_version: freshView.policy.mode_version,
        expected_owner_epoch: freshView.policy.owner_epoch,
        expected_facts_digest: freshView.policy.facts_digest,
        expected_work_item_version: freshView.work_items[0].Version,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-accommodation-reevaluate-1" },
    });
    expect(superseded.status()).toBe(201);
    await page.goto(pathA);
    await expect(page.locator(`#policy-outcome-${workItemID}`)).not.toContainText('Активные альтернативы (честные пути без media)');
    await expect(page.getByText(/Действующий исход \(№2\)/)).toBeVisible();

    // 12. The control branch: the form select honestly offers the EMPTY
    // option only (the active set is gone — the UI never invents a
    // word); the MACHINE path carrying a foreign word refuses with the
    // honest 400 (the field has exactly one meaning); the empty word of
    // the form passes the old path byte-identically.
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const controlForm = page.locator('form[action$="/client-replies"]').first();
    const controlOptions = controlForm.locator('select[name="accuracy_qualifier"] option');
    await expect(controlOptions).toHaveCount(1); // the empty option only — the active set is gone
    const freshControlView = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const controlRefusal = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/client-replies`, {
      data: {
        conversation_id: conversationID,
        expected_epoch: freshControlView.ownership.state.Current.Epoch,
        reply_class: 'sales',
        work_item_id: workItemID,
        accuracy_qualifier: 'text_description',
        content: 'Ответ с лишним словом ограничения',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-accommodation-control-1" },
    });
    expect(controlRefusal.status()).toBe(400);
    const controlProblem = await controlRefusal.json();
    expect(controlProblem.code).toBe('accuracy_qualifier_foreign');
    expect(controlProblem.message).toContain('альтернатив в действующем исходе нет');
    const passForm = controlForm;
    await passForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await passForm.locator('select[name="reply_class"]').selectOption('sales');
    await passForm.locator('select[name="work_item_id"]').selectOption(workItemID);
    await passForm.locator('textarea[name="content"]').fill('Ответ прежним путём без слова ограничения');
    await passForm.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?reply_prepared=1`));

    // 13. The a11y close of the operator page: the 320px single column
    // (no horizontal scroll), the polite live regions of the preferences
    // panel, the conscious selects (no preselect — neither the record
    // key nor the reply qualifier) and no autofocus of the primary
    // actions.
    await page.goto(pathA);
    await expect(page.locator('#accommodation-key option[selected]')).toHaveCount(0);
    await expect(page.locator('#reply-qualifier option[selected]')).toHaveCount(0);
    await expect(page.locator('#accommodations-panel-note')).toHaveAttribute('aria-live', 'polite');
    await expect(page.locator('#accommodations-current')).toHaveAttribute('aria-live', 'polite');
    const recordButtons = page.locator('form[action$="/accommodations"] button');
    await expect(recordButtons).not.toHaveAttribute('autofocus');
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    await api.post('/fixture/operator', { form: { second: '' } }).catch(() => {});
    await page.close();
  }
});
