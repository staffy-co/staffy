import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const foreignTenantID = '01994f64-6c00-7000-8000-00000000a212';
const chatConnectionID = '01994f64-6c00-7000-8000-000000000016';
const apiBase = `/v1/tenants/${tenantID}`;
const jsonHeaders = { 'Content-Type': 'application/json' };

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

// The machine drain command of the tenant queue (the strict EMPTY body —
// a free field never reaches the service).
async function postDrain(api: any) {
  return api.post(`${apiBase}/operations/drain`, { data: {}, headers: jsonHeaders });
}

// The Story 3.6 chain (z34) at 320px: the z33 live path reused to the
// authorized/queued operation, then the EXECUTION contour — the honest
// empty drain FIRST (every considered job speaks its line, never a
// masked silence), the page drain form with the checkbox discipline and
// the PRG notice, the scriptable verdicts of the synthetic mutation
// adapter (the accepted reception is NON-terminal and never repeats —
// the decided-verify skip of the at-least-once redelivery; the
// committed verdict demands the structural evidence and lands the
// receipt v1 card with the identifier readback; the failed verdict is
// terminal with the new-Confirmation next step; the broken adapter call
// answers the honest unknown with the write-ahead resting attempt), the
// blind-repeat park of the unknown without the recorded guarantee, the
// rights rows (the operations scope 403 of the drain gate, the
// cross-tenant 404 on both surfaces) and the a11y close (the CLOSED
// attempts disclosure walks the keyboard, the 320px column never
// scrolls sideways).
test('action-dispatch: the execution contour of the authorized intent at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 0. The fixture switches of the vertical (the z33 precedent): the
    // preview dictionary, the REAL telephony connection with the
    // verified read-only channel capability (the mutating destination
    // never probe-verifies before the Action Gateway — the honest
    // step-04 finding) and the LIVE entitlement walk.
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '1' } });
    const created = await api.post(`${apiBase}/connections`, { data: {
      connector_id: 'staffy-synthetic-telephony', connector_version: '1.1.0', label: 'Телефония исполнения', environment: 'sandbox',
      config: { endpoint: 'https://execution.sandbox.example/api' }
    } });
    expect(created.status()).toBe(201);
    const createdView = await created.json();
    const connectionID = createdView.connection.connection_id;
    const connect = await api.post(`${apiBase}/connections/${connectionID}/connect`, { data: { version: createdView.connection.version, secrets: { api_key: 'sandbox-execution-key' } } });
    expect(connect.status()).toBe(200);
    const verify = await api.post(`${apiBase}/connections/${connectionID}/capabilities/query.trunk-status/verify`, { data: {} });
    expect(verify.status()).toBe(200);

    const entitlement = await (await api.get(`${apiBase}/entitlement`)).json();
    if (entitlement.state === 'paused') {
      const resume = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'resume', basis: 'Возобновление для исполнения', confirm: true } });
      expect(resume.status()).toBeLessThan(300);
    } else if (entitlement.state !== 'active') {
      const terms = await api.post(`${apiBase}/entitlement/terms`, { data: {
        currency: 'RUB', tax_regime: 'УСН', period_start: '2026-10-01T00:00:00Z', period_end: '2026-12-31T23:59:59Z',
        included_employees: ['consultant-01'], included_channels: ['jivo'], included_connectors: ['abcp'],
        launch_price: 49000, subscription_price: 15000, change_note: 'Коммерческое предложение исполнения'
      } });
      expect(terms.status()).toBeLessThan(300);
      const activate = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'activate', basis: 'Договор №79 от 01.10.2026', confirm: true } });
      expect(activate.status()).toBeLessThan(300);
    }

    // 1. The honest EMPTY drain FIRST: the strict decode refuses the
    // free field BEFORE the service (the drain carries NO parameters),
    // then the empty body answers the honest report — every considered
    // job of the bounded batch leaves its spoken line (the suite order
    // may carry the queued z33 leftover — the arithmetic identity holds
    // either way, the solo run answers the honest zero).
    const freeField = await api.post(`${apiBase}/operations/drain`, { data: { limit: 50 }, headers: jsonHeaders });
    expect(freeField.status()).toBe(400);
    expect((await freeField.json()).message).toContain('один объект с известными полями');
    const emptyDrain = await postDrain(api);
    expect(emptyDrain.status()).toBe(201);
    const emptyReport = (await emptyDrain.json()).drain;
    expect(emptyReport.command).toBe('dispatch_queued_operations');
    expect(emptyReport.considered).toBe(emptyReport.reports.length);
    expect(emptyReport.dispatched + emptyReport.blocked + emptyReport.skipped).toBe(emptyReport.considered);

    // 2. One TEXT turn opens the case of the chain; the machine proposal
    // builds preview version 1 of the manual-only vertical (the
    // single_use unsafe class of the fixture dictionary).
    const first = await postIngress(api, textInboundEvent('journey-dis-evt-1', 'journey-dis-sess-1', { say: 'Подтвердите отмену созвона и авторизуйте уведомление' }), 'journey-dis-ingress-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const caseView0 = await (await api.get(`${apiBase}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;
    const machinePath = `${apiBase}/cases/${caseA}/operations`;
    const reviewPath = `/app/tenants/${tenantID}/cases/${caseA}/operations`;
    const proposal = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a401',
      typed_input: { phone: '+79000000411' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal.status()).toBe(200);
    const proposalView = (await proposal.json()).proposal;
    expect(proposalView.verdict).toBe('valid');
    const previewID = proposalView.preview_id;

    // 3. The honest empty world of THIS case: the page names the absence,
    // the machine view answers the honest empty arrays.
    await page.goto(reviewPath);
    await expect(page.locator('h1')).toContainText('Operation Review');
    await expect(page.locator('#operations-empty-heading')).toHaveText('Подтверждений и операций нет');
    const emptyView = await (await api.get(machinePath)).json();
    expect(emptyView.authorizations.confirmations).toEqual([]);
    expect(emptyView.authorizations.operations).toEqual([]);

    // 4. The z33 live path reused: the primary transition lands the
    // confirm form with the prefill, the human consent demands the LIVE
    // step-up (the refusal first, the confirmed append after).
    const previewPage = `/app/tenants/${tenantID}/cases/${caseA}/action-previews`;
    await page.goto(previewPage);
    const primaryLink = page.locator('a.primary', { hasText: 'Перейти к подтверждению и авторизации' }).first();
    await expect(primaryLink).toHaveAttribute('href', `/app/tenants/${tenantID}/cases/${caseA}/operations?confirm=${previewID}@1`);
    await primaryLink.click();
    await expect(page.locator('#confirm_preview_id')).toHaveValue(previewID);
    await expect(page.locator('#confirm_version_no')).toHaveValue('1');
    await page.getByRole('button', { name: 'Подтвердить версию preview' }).click();
    await expect(page.locator('.alert')).toContainText('повторное подтверждение личности');
    await api.post('/fixture/step-up', { form: { enabled: '1' } });
    await page.goto(reviewPath + '?confirm=' + previewID + '@1');
    await page.getByRole('button', { name: 'Подтвердить версию preview' }).click();
    await expect(page).toHaveURL(/\?confirmed=1$/);
    await expect(page.locator('#page-notice')).toContainText('Confirmation версии 1 построена');
    await expect(page.locator('#page-notice')).toBeFocused();
    const consentCard = page.locator('.action-card', { hasText: 'Confirmation' }).first();
    await expect(consentCard).toContainText('Состояние: valid');

    // 5. The machine authorize through the page form: the ONE transaction
    // answers the authorized/queued card — the life word with the next
    // step of the drain command.
    const authForm = consentCard.locator('details', { hasText: 'Авторизовать Operation по этому подтверждению' });
    await authForm.locator('summary').click();
    await authForm.locator('input[name="idempotency_key"]').fill('journey-dis-key-1');
    await authForm.locator('button.primary', { name: 'Авторизовать Operation' }).click();
    await expect(page).toHaveURL(/\?authorized=[0-9a-f-]+$/);
    await expect(page.locator('#page-notice')).toContainText('авторизована и в очереди на будущий dispatch');
    const operationCard = page.locator('.action-card').filter({ hasText: 'Состояние жизни: authorized' }).first();
    await expect(operationCard).toContainText('Состояние жизни: authorized');
    await expect(operationCard).toContainText('Следующий шаг: оператор запускает команду drain');
    await expect(operationCard).toContainText('Внешний идентификатор не выдан');
    const machineView = await (await api.get(machinePath)).json();
    const operation1ID = machineView.authorizations.operations[0].operation_id;
    expect(machineView.authorizations.operations[0].idempotency_key).toBe('journey-dis-key-1');
    expect(machineView.authorizations.operations[0].state).toBe('authorized');

    // 6. The page drain form: the explicit checkbox discipline (the
    // unchecked form never posts), the scriptable accepted verdict of
    // the synthetic adapter, the PRG notice with the considered counter
    // and the focus move.
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'accepted', guarantee: '0' } });
    await page.goto(reviewPath);
    const drainDetails = page.locator('details', { hasText: 'Запустить дренаж очереди отправок' });
    await drainDetails.locator('summary').click();
    await drainDetails.locator('button.primary', { name: 'Дренировать очередь' }).click();
    await expect(page).not.toHaveURL(/\?drained=/);
    await drainDetails.locator('input[name="confirm_drain"]').check();
    await drainDetails.locator('button.primary', { name: 'Дренировать очередь' }).click();
    await expect(page).toHaveURL(/\?drained=[0-9]+$/);
    await expect(page.locator('#page-notice')).toContainText('Очередь отправок дренирована: рассмотрено заданий');
    await expect(page.locator('#page-notice')).toBeFocused();

    // 7. The accepted card: the NON-terminal provider reception — the
    // life word with the 3-7 next step, the CLOSED attempts disclosure
    // (the write-ahead lease facts, the keyed digest, the provider
    // correlation minted from the external idempotency key, the
    // un-recorded guarantee word), NO receipt card (the business result
    // is not confirmed yet); the machine view speaks the same words.
    const acceptedCard = page.locator('.action-card').filter({ hasText: 'Состояние жизни: accepted' }).first();
    await expect(acceptedCard).toContainText('Принято провайдером, бизнес-результат не подтверждён');
    await expect(acceptedCard).toContainText('Следующий шаг: ожидание подтверждения бизнес-результата контуром сверки (3-7)');
    await expect(acceptedCard).toContainText('Owning System): erp');
    await expect(acceptedCard).toContainText('Момент состояния:');
    await expect(acceptedCard.locator('details', { hasText: 'Карточка Action Receipt' })).toHaveCount(0);
    const attemptsDetails = acceptedCard.locator('details', { hasText: 'Попытки отправки' });
    await expect(attemptsDetails).not.toHaveAttribute('open');
    const attemptsSummary = attemptsDetails.locator('summary');
    await attemptsSummary.focus();
    await attemptsSummary.press('Enter');
    await expect(attemptsDetails).toHaveAttribute('open', '');
    await expect(attemptsDetails).toContainText('Попытка 1: право и момент отправки закреплены');
    await expect(attemptsDetails).toContainText('keyed digest');
    await expect(attemptsDetails).toContainText('исход accepted: Принято провайдером');
    await expect(attemptsDetails).toContainText('Корреляция провайдера: syn-');
    await expect(attemptsDetails).toContainText('Гарантия провайдера не записана: слепой повтор запрещён');
    const acceptedView = await (await api.get(machinePath)).json();
    const acceptedOp = acceptedView.authorizations.operations.find((o: any) => o.operation_id === operation1ID);
    expect(acceptedOp.state).toBe('accepted');
    expect(acceptedOp.attempts.length).toBe(1);
    expect(acceptedOp.attempts[0].attempt_no).toBe(1);
    expect(acceptedOp.attempts[0].outcome).toBe('accepted');
    expect(acceptedOp.attempts[0].outcome_recorded).toBe(true);
    expect(acceptedOp.attempts[0].correlation_id.startsWith('syn-')).toBe(true);
    expect(acceptedOp.attempts[0].request_digest.length).toBe(64);
    expect(acceptedOp.receipts).toEqual([]);

    // 8. The at-least-once redelivery of the DECIDED operation: the
    // fixture clock walks past the lease window (35 seconds of the
    // write-ahead right), the second drain answers the honest
    // decided-verify skip — no second send, no new intent, the attempt
    // count stays one.
    await api.post('/fixture/clock', { form: { minutes: '1' } });
    const replayDrain = await postDrain(api);
    expect(replayDrain.status()).toBe(201);
    const replayReport = (await replayDrain.json()).drain;
    const replayLines = replayReport.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(replayLines.length).toBeGreaterThan(0);
    expect(replayLines[0].text).toContain('Повтор задания: операция уже решена');
    const afterReplayView = await (await api.get(machinePath)).json();
    const afterReplayOp = afterReplayView.authorizations.operations.find((o: any) => o.operation_id === operation1ID);
    expect(afterReplayOp.attempts.length).toBe(1);

    // 9. The committed verdict demands the structural evidence: the
    // second operation of the case (a fresh consent chain — the
    // single_use consumption never repeats), the scripted external
    // object identity, the fenced verdict line, the receipt v1 card
    // with the identifier readback (the case, the work item, the two
    // pins, the definition, the external object) — the sensitive
    // material NEVER enters the readback.
    const proposal2 = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a402',
      typed_input: { phone: '+79000000412' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal2.status()).toBe(200);
    const preview2ID = (await proposal2.json()).proposal.preview_id;
    const confirm2 = await api.post(`${apiBase}/cases/${caseA}/action-previews/${preview2ID}/versions/1/confirmations`, { data: { lifetime_seconds: 3600 } });
    expect(confirm2.status()).toBe(201);
    const confirmation2ID = (await confirm2.json()).confirmation.confirmation_id;
    const authorize2 = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmation2ID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-dis-key-2' } });
    expect(authorize2.status()).toBe(201);
    const operation2ID = (await authorize2.json()).authorization.operation.operation_id;
    await api.post('/fixture/action-dispatch-evidence', { form: { object_id: 'ext-journey-dis-42', object_version: '7' } });
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'committed', correlation: 'journey-dis-corr-42', guarantee: '0' } });
    const committedDrain = await postDrain(api);
    expect(committedDrain.status()).toBe(201);
    const committedReport = (await committedDrain.json()).drain;
    const committedLines = committedReport.reports.filter((l: any) => l.operation_id === operation2ID);
    expect(committedLines.length).toBe(1);
    expect(committedLines[0].text).toContain('Завершено: внешний объект создан (committed)');
    const committedView = await (await api.get(machinePath)).json();
    const committedOp = committedView.authorizations.operations.find((o: any) => o.operation_id === operation2ID);
    expect(committedOp.state).toBe('committed');
    expect(committedOp.attempts[0].outcome).toBe('committed');
    expect(committedOp.attempts[0].correlation_id).toBe('journey-dis-corr-42');
    expect(committedOp.receipts.length).toBe(1);
    expect(committedOp.receipts[0].receipt_version_no).toBe(1);
    expect(committedOp.receipts[0].outcome).toBe('committed');
    expect(committedOp.receipts[0].external_object_id).toBe('ext-journey-dis-42');
    expect(committedOp.receipts[0].external_object_version).toBe('7');
    expect(committedOp.receipts[0].case_id).toBe(caseA);
    expect(committedOp.receipts[0].work_item_id).toBe(workItemID);
    expect(committedOp.receipts[0].preview_id).toBe(preview2ID);
    expect(committedOp.receipts[0].confirmation_id).toBe(confirmation2ID);
    expect(committedOp.receipts[0].definition_id).toBe('notify.manual-only');
    const committedReceiptJSON = JSON.stringify(committedOp.receipts[0]);
    expect(committedReceiptJSON).not.toContain('+79000000412');
    await page.goto(reviewPath);
    const committedCard = page.locator('.action-card').filter({ hasText: 'Состояние жизни: committed' }).first();
    await expect(committedCard).toContainText('Завершено: внешний объект создан (committed)');
    await expect(committedCard).toContainText('Следующего шага нет: терминальный успех');
    await expect(committedCard).toContainText('№2 submitted');
    await expect(committedCard).toContainText('№3 committed');
    const receiptCard = committedCard.locator('details', { hasText: 'Карточка Action Receipt' });
    await expect(receiptCard).toHaveAttribute('open', '');
    await expect(receiptCard).toContainText('Версия квитанции 1');
    await expect(receiptCard).toContainText('Внешний объект: ext-journey-dis-42 (версия 7)');
    await expect(receiptCard).toContainText(`Кейс ${caseA}`);
    await expect(receiptCard).toContainText(`рабочий элемент ${workItemID}`);
    await expect(receiptCard).toContainText(`пин preview ${preview2ID}`);
    await expect(receiptCard).toContainText(`пин Confirmation ${confirmation2ID}`);
    await expect(receiptCard).toContainText('определение notify.manual-only@1.0.0');
    const committedAttempts = committedCard.locator('details', { hasText: 'Попытки отправки' });
    await committedAttempts.locator('summary').click();
    await expect(committedAttempts).toContainText('исход committed: Завершено');
    await expect(committedAttempts).toContainText('Корреляция провайдера: journey-dis-corr-42');
    const receiptText = await receiptCard.innerText();
    expect(receiptText).not.toContain('+79000000412');

    // 10. The failed verdict is terminal: no auto-retry exists, the next
    // step demands the NEW Confirmation of the 3-5 contour, the receipt
    // v1 speaks the failed word with the honest absence of the external
    // object.
    const proposal3 = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a403',
      typed_input: { phone: '+79000000413' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal3.status()).toBe(200);
    const preview3ID = (await proposal3.json()).proposal.preview_id;
    const confirm3 = await api.post(`${apiBase}/cases/${caseA}/action-previews/${preview3ID}/versions/1/confirmations`, { data: { lifetime_seconds: 3600 } });
    expect(confirm3.status()).toBe(201);
    const confirmation3ID = (await confirm3.json()).confirmation.confirmation_id;
    const authorize3 = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmation3ID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-dis-key-3' } });
    expect(authorize3.status()).toBe(201);
    const operation3ID = (await authorize3.json()).authorization.operation.operation_id;
    await api.post('/fixture/action-dispatch-evidence', { form: { object_id: '', object_version: '' } });
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'failed', guarantee: '0' } });
    const failedDrain = await postDrain(api);
    expect(failedDrain.status()).toBe(201);
    const failedReport = (await failedDrain.json()).drain;
    const failedLines = failedReport.reports.filter((l: any) => l.operation_id === operation3ID);
    expect(failedLines.length).toBe(1);
    expect(failedLines[0].text).toContain('Не выполнено без эффекта (failed)');
    const failedView = await (await api.get(machinePath)).json();
    const failedOp = failedView.authorizations.operations.find((o: any) => o.operation_id === operation3ID);
    expect(failedOp.state).toBe('failed');
    expect(failedOp.receipts[0].outcome).toBe('failed');
    expect(failedOp.receipts[0].external_object_id).toBe('');
    await page.goto(reviewPath);
    const failedCard = page.locator('.action-card').filter({ hasText: 'Состояние жизни: failed' }).first();
    await expect(failedCard).toContainText('Не выполнено без эффекта (failed)');
    await expect(failedCard).toContainText('Следующий шаг: новый запуск требует новой Confirmation');
    await expect(failedCard.locator('details', { hasText: 'Карточка Action Receipt' })).toContainText('Внешний объект не выдан');

    // 11. The broken adapter call answers the honest unknown: the
    // write-ahead lease and the submitted ledger entry stay (the right
    // and the moment of the send were material BEFORE the call), NO
    // outcome row speaks a claim the provider never made, the receipt v1
    // unknown never claims a success, the attempt view renders the
    // resting word.
    const proposal4 = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a404',
      typed_input: { phone: '+79000000414' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal4.status()).toBe(200);
    const preview4ID = (await proposal4.json()).proposal.preview_id;
    const confirm4 = await api.post(`${apiBase}/cases/${caseA}/action-previews/${preview4ID}/versions/1/confirmations`, { data: { lifetime_seconds: 3600 } });
    expect(confirm4.status()).toBe(201);
    const confirmation4ID = (await confirm4.json()).confirmation.confirmation_id;
    const authorize4 = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmation4ID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-dis-key-4' } });
    expect(authorize4.status()).toBe(201);
    const operation4ID = (await authorize4.json()).authorization.operation.operation_id;
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'timeout', failure: '1', guarantee: '0' } });
    const unknownDrain = await postDrain(api);
    expect(unknownDrain.status()).toBe(201);
    const unknownReport = (await unknownDrain.json()).drain;
    const unknownLines = unknownReport.reports.filter((l: any) => l.operation_id === operation4ID);
    expect(unknownLines.length).toBe(1);
    expect(unknownLines[0].text).toContain('Исход неизвестен (unknown)');
    const unknownView = await (await api.get(machinePath)).json();
    const unknownOp = unknownView.authorizations.operations.find((o: any) => o.operation_id === operation4ID);
    expect(unknownOp.state).toBe('unknown');
    expect(unknownOp.attempts[0].outcome ?? '').toBe('');
    expect(unknownOp.attempts[0].outcome_recorded).toBe(false);
    expect(unknownOp.receipts[0].outcome).toBe('unknown');
    await page.goto(reviewPath);
    const unknownCard = page.locator('.action-card').filter({ hasText: 'Состояние жизни: unknown' }).first();
    await expect(unknownCard).toContainText('Исход неизвестен (unknown)');
    await expect(unknownCard).toContainText('Следующий шаг: сверка исхода контуром сверки (3-7)');
    await expect(unknownCard).toContainText('№3 unknown');
    const unknownAttempts = unknownCard.locator('details', { hasText: 'Попытки отправки' });
    await unknownAttempts.locator('summary').click();
    await expect(unknownAttempts).toContainText('исход не записан');

    // 12. The blind repeat of the unknown without the recorded guarantee
    // is FORBIDDEN: the clock walks past the lease window, the final
    // drain sends NOTHING (every line is an honest skip or park word,
    // the decided operations answer the replay skip, the unknown parks
    // with the exact word), the attempt count never grows.
    await api.post('/fixture/clock', { form: { minutes: '2' } });
    const finalDrain = await postDrain(api);
    expect(finalDrain.status()).toBe(201);
    const finalReport = (await finalDrain.json()).drain;
    expect(finalReport.dispatched).toBe(0);
    expect(finalReport.considered).toBe(finalReport.reports.length);
    const finalUnknownLines = finalReport.reports.filter((l: any) => l.operation_id === operation4ID);
    expect(finalUnknownLines.length).toBe(1);
    expect(finalUnknownLines[0].text).toContain('слепой повтор запрещён');
    const finalView = await (await api.get(machinePath)).json();
    const finalUnknownOp = finalView.authorizations.operations.find((o: any) => o.operation_id === operation4ID);
    expect(finalUnknownOp.attempts.length).toBe(1);

    // 13. The rights rows: the operations-only member hears the honest
    // 403 with the drain scope words (the queue is an operator command),
    // the cross-tenant drain and page reads are the plain 404 (no
    // disclosure that the tenant dispatches actions).
    await api.post('/fixture/operator', { form: { second: 'ops' } });
    const opsDrain = await api.post(`${apiBase}/operations/drain`, { data: {}, headers: jsonHeaders });
    expect(opsDrain.status()).toBe(403);
    expect((await opsDrain.json()).message).toContain('полномочия operations');
    await api.post('/fixture/operator', { form: { second: '' } });
    const foreignDrain = await api.post(`/v1/tenants/${foreignTenantID}/operations/drain`, { data: {}, headers: jsonHeaders });
    expect(foreignDrain.status()).toBe(404);
    const foreignPageView = await api.get(`/app/tenants/${foreignTenantID}/cases/${caseA}/operations`);
    expect(foreignPageView.status()).toBe(404);

    // 14. The a11y close at 320px: the skip-link, the semantic heading
    // ladder, the ≥44×44 targets of the drain disclosure summary and the
    // primary button, the KEYBOARD toggle of a CLOSED attempts
    // disclosure and the single column without the horizontal scroll.
    await page.goto(reviewPath);
    await expect(page.locator('a.skip-link')).toBeVisible();
    await expect(page.locator('h1')).toContainText('Operation Review');
    const headings = await page.locator('h2').all();
    expect(headings.length).toBeGreaterThan(0);
    const drainSummary = page.locator('details', { hasText: 'Запустить дренаж очереди отправок' }).first().locator('summary');
    const drainSummaryBox = await drainSummary.boundingBox();
    expect(drainSummaryBox!.height).toBeGreaterThanOrEqual(44);
    const closedAttempts = page.locator('.action-card').filter({ hasText: 'Состояние жизни: committed' }).first().locator('details', { hasText: 'Попытки отправки' });
    await expect(closedAttempts).not.toHaveAttribute('open');
    const closedSummary = closedAttempts.locator('summary');
    await closedSummary.focus();
    await closedSummary.press('Enter');
    await expect(closedAttempts).toHaveAttribute('open', '');
    const primaryBox = await page.getByRole('button', { name: 'Подтвердить версию preview' }).boundingBox();
    expect(primaryBox!.height).toBeGreaterThanOrEqual(44);
    expect(primaryBox!.width).toBeGreaterThanOrEqual(44);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    // The fixture reset: the next journey starts at the honest
    // production default (the empty dictionary, the primary owner, the
    // live clock, the step-up off, the default adapter script).
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '0' } });
    await api.post('/fixture/operator', { form: { second: '' } });
    await api.post('/fixture/step-up', { form: { enabled: '0' } });
    await api.post('/fixture/clock', { form: { minutes: '0' } });
    await api.post('/fixture/preview-material', { form: { field: 'order_status', status: '' } });
    await api.post('/fixture/action-dispatch-evidence', { form: { object_id: '', object_version: '' } });
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'accepted', guarantee: '0' } });
  }
});
