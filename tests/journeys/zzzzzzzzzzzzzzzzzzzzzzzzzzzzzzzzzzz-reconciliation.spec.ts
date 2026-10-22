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

// The machine reconcile command of the tenant pending targets (the same
// strict EMPTY body — the command carries NO parameters at all).
async function postReconcile(api: any) {
  return api.post(`${apiBase}/operations/reconcile`, { data: {}, headers: jsonHeaders });
}

// The Story 3.7 chain (z35) at 320px: the z34 live path reused to the
// honest unknown (the broken adapter call of the dispatch answers the
// write-ahead resting attempt with the receipt v1 unknown), then the
// RECONCILIATION contour — the strict empty machine command with the
// honest line report (the in_progress intermediate keeps the operation
// unknown, the active lease of a parallel command answers the racing
// word, the fixture clock walks past the lease window), the page
// reconcile form with the checkbox discipline and the PRG notice, the
// scriptable verdicts of the synthetic read-only adapter (the committed
// verdict demands the structural evidence and appends the receipt v2
// with the identifier readback; the no_effect verdict proves the
// ABSENCE of the external effect, lands the failed pair and speaks the
// retry-unlock word), the rights rows (the operations scope 403 of the
// reconcile gate, the cross-tenant 404 on both surfaces) and the a11y
// close (the CLOSED reconcile-attempts disclosure walks the keyboard,
// the 320px column never scrolls sideways).
//
// NOTE (the step-04 fixture upgrade): the journeys dispatch
// classification ports the production noEffect fact since the step-04
// fixture unlock upgrade (classifyDispatchJob 5-arg + the
// reconcileNoEffectLocked bridge + the store-derived receipt version
// max+1) — the failed+no_effect operation unlocks the retry WITHOUT the
// guarantee, the re-dispatch tail of the unlock walks the FULL ladder to
// the committed v3 of the SAME operation (the same external key and
// digest, no new intent, no new reservation).
test('reconciliation: the honest verdict contour of the unknown operation at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 0. The fixture switches of the vertical (the z34 precedent): the
    // preview dictionary, the REAL telephony connection with the
    // verified read-only channel word and the LIVE entitlement walk;
    // the step-up stays on for the machine confirmations of the chain.
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '1' } });
    await api.post('/fixture/step-up', { form: { enabled: '1' } });
    const created = await api.post(`${apiBase}/connections`, { data: {
      connector_id: 'staffy-synthetic-telephony', connector_version: '1.1.0', label: 'Телефония сверки', environment: 'sandbox',
      config: { endpoint: 'https://reconcile.sandbox.example/api' }
    } });
    expect(created.status()).toBe(201);
    const createdView = await created.json();
    const connectionID = createdView.connection.connection_id;
    const connect = await api.post(`${apiBase}/connections/${connectionID}/connect`, { data: { version: createdView.connection.version, secrets: { api_key: 'sandbox-reconcile-key' } } });
    expect(connect.status()).toBe(200);
    const verify = await api.post(`${apiBase}/connections/${connectionID}/capabilities/query.trunk-status/verify`, { data: {} });
    expect(verify.status()).toBe(200);

    const entitlement = await (await api.get(`${apiBase}/entitlement`)).json();
    if (entitlement.state === 'paused') {
      const resume = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'resume', basis: 'Возобновление для сверки', confirm: true } });
      expect(resume.status()).toBeLessThan(300);
    } else if (entitlement.state !== 'active') {
      const terms = await api.post(`${apiBase}/entitlement/terms`, { data: {
        currency: 'RUB', tax_regime: 'УСН', period_start: '2026-10-01T00:00:00Z', period_end: '2026-12-31T23:59:59Z',
        included_employees: ['consultant-01'], included_channels: ['jivo'], included_connectors: ['abcp'],
        launch_price: 49000, subscription_price: 15000, change_note: 'Коммерческое предложение сверки'
      } });
      expect(terms.status()).toBeLessThan(300);
      const activate = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'activate', basis: 'Договор №81 от 01.10.2026', confirm: true } });
      expect(activate.status()).toBeLessThan(300);
    }

    // 1. The unknown world of the z34 chain reused: one TEXT turn opens
    // the case, the machine proposal builds preview version 1, the
    // honest empty page names the absence, the machine confirmation and
    // authorization land the queued operation, the broken adapter call
    // of the drain answers the honest unknown — the write-ahead lease
    // and the submitted ledger entry stay, NO outcome row speaks a
    // claim the provider never made, the receipt v1 unknown never
    // claims a success.
    const first = await postIngress(api, textInboundEvent('journey-rec-evt-1', 'journey-rec-sess-1', { say: 'Подтвердите перенос созвона и авторизуйте уведомление' }), 'journey-rec-ingress-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const caseView0 = await (await api.get(`${apiBase}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;
    const machinePath = `${apiBase}/cases/${caseA}/operations`;
    const reviewPath = `/app/tenants/${tenantID}/cases/${caseA}/operations`;
    const proposal = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a701',
      typed_input: { phone: '+79000000711' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal.status()).toBe(200);
    expect((await proposal.json()).proposal.verdict).toBe('valid');
    const preview1ID = (await proposal.json()).proposal.preview_id;

    await page.goto(reviewPath);
    await expect(page.locator('h1')).toContainText('Operation Review');
    await expect(page.locator('#operations-empty-heading')).toHaveText('Подтверждений и операций нет');
    const emptyView = await (await api.get(machinePath)).json();
    expect(emptyView.authorizations.confirmations).toEqual([]);
    expect(emptyView.authorizations.operations).toEqual([]);

    const confirm1 = await api.post(`${apiBase}/cases/${caseA}/action-previews/${preview1ID}/versions/1/confirmations`, { data: { lifetime_seconds: 3600 } });
    expect(confirm1.status()).toBe(201);
    const confirmation1ID = (await confirm1.json()).confirmation.confirmation_id;
    const authorize1 = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmation1ID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-rec-key-1' } });
    expect(authorize1.status()).toBe(201);
    const operation1ID = (await authorize1.json()).authorization.operation.operation_id;
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'timeout', failure: '1', guarantee: '0' } });
    const unknownDrain = await postDrain(api);
    expect(unknownDrain.status()).toBe(201);
    const unknownReport = (await unknownDrain.json()).drain;
    const unknownLines = unknownReport.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(unknownLines.length).toBe(1);
    expect(unknownLines[0].text).toContain('Исход неизвестен (unknown)');
    const unknownView = await (await api.get(machinePath)).json();
    const unknownOp = unknownView.authorizations.operations.find((o: any) => o.operation_id === operation1ID);
    expect(unknownOp.state).toBe('unknown');
    expect(unknownOp.receipts.length).toBe(1);
    expect(unknownOp.receipts[0].receipt_version_no).toBe(1);
    expect(unknownOp.receipts[0].outcome).toBe('unknown');

    // 2. The machine reconcile command: the strict decode refuses the
    // free field BEFORE the service (the command carries NO parameters),
    // then the empty body answers the honest report — every considered
    // target of the bounded batch leaves its spoken line; the FIRST run
    // queries the fresh unknown target with the DEFAULT script of the
    // read-only adapter (in_progress — the honest intermediate word),
    // the attempt records its guarded truth and the operation honestly
    // stays unknown (reconciling is an activity in the history, never a
    // state).
    const freeField = await api.post(`${apiBase}/operations/reconcile`, { data: { limit: 50 }, headers: jsonHeaders });
    expect(freeField.status()).toBe(400);
    expect((await freeField.json()).message).toContain('один объект с известными полями');
    const progressRun = await postReconcile(api);
    expect(progressRun.status()).toBe(201);
    const progressReport = (await progressRun.json()).reconcile;
    expect(progressReport.command).toBe('reconcile_unknown_operations');
    expect(progressReport.considered).toBe(progressReport.reports.length);
    const progressLines = progressReport.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(progressLines.length).toBe(1);
    expect(progressLines[0].attempt_no).toBe(1);
    expect(progressLines[0].text).toContain('Система-владелец обрабатывает (in_progress)');
    const progressView = await (await api.get(machinePath)).json();
    const progressOp = progressView.authorizations.operations.find((o: any) => o.operation_id === operation1ID);
    expect(progressOp.state).toBe('unknown');
    expect(progressOp.receipts.length).toBe(1);
    expect(progressOp.receipts[0].outcome).toBe('unknown');
    expect(progressOp.reconciliation_attempts.length).toBe(1);
    expect(progressOp.reconciliation_attempts[0].verdict).toBe('in_progress');
    expect(progressOp.reconciliation_attempts[0].verdict_recorded).toBe(true);
    expect(progressOp.reconciliation_attempts[0].correlation_id.startsWith('syn-')).toBe(true);
    expect(progressOp.reconciliation_attempts[0].external_object_id).toBe('');

    // 3. The active lease of a parallel command: the immediate second
    // machine reconcile answers the honest racing word — the maximum ONE
    // active chain of reconciliation, no second right, no query, no new
    // attempt (the fixture clock governs the window; the backoff word
    // rides the closed attempt facts of the card below).
    const racingRun = await postReconcile(api);
    expect(racingRun.status()).toBe(201);
    const racingReport = (await racingRun.json()).reconcile;
    expect(racingReport.considered).toBe(racingReport.reports.length);
    const racingLines = racingReport.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(racingLines.length).toBe(1);
    expect(racingLines[0].text).toContain('Сверка уже идёт: активная lease держит право');

    // 4. The page reconcile form: the explicit checkbox discipline (the
    // unchecked form never posts), the confirmed command lands the PRG
    // notice with the considered counter and the focus move; the absent
    // word names the explicit-command discipline (no daemon exists).
    await page.goto(reviewPath);
    const reconcileDetails = page.locator('details', { hasText: 'Запустить сверку unknown-операций' });
    await reconcileDetails.locator('summary').click();
    await expect(reconcileDetails.locator('p.next-note')).toContainText('Reconcile — явная команда (демона нет)');
    await reconcileDetails.locator('button.primary', { name: 'Запустить сверку' }).click();
    await expect(page).not.toHaveURL(/\?reconciled=/);
    await reconcileDetails.locator('input[name="confirm_reconcile"]').check();
    await reconcileDetails.locator('button.primary', { name: 'Запустить сверку' }).click();
    await expect(page).toHaveURL(/\?reconciled=[0-9]+$/);
    await expect(page.locator('#page-notice')).toContainText('Сверка unknown-операций выполнена: рассмотрено целей');
    await expect(page.locator('#page-notice')).toBeFocused();

    // 5. The in_progress card of the page surface: the operation stays
    // unknown with the life word and the 3-7 next step, the CLOSED
    // attempts disclosure carries the write-ahead lease facts (the
    // window, the next eligibility of the backoff) and the honest
    // in_progress verdict word with the minted provider correlation.
    const progressCard = page.locator('.action-card').filter({ hasText: operation1ID });
    await expect(progressCard).toContainText('Состояние жизни: unknown');
    await expect(progressCard).toContainText('Следующий шаг: сверка исхода командой reconcile');
    await expect(progressCard).toContainText('Reconcile — явная команда сверки');
    const progressAttempts = progressCard.locator('details', { hasText: 'Попытки сверки' });
    await expect(progressAttempts).not.toHaveAttribute('open');
    await progressAttempts.locator('summary').focus();
    await progressAttempts.locator('summary').press('Enter');
    await expect(progressAttempts).toHaveAttribute('open', '');
    await expect(progressAttempts).toContainText('Попытка 1: право и момент запроса закреплены');
    await expect(progressAttempts).toContainText('окно lease до');
    await expect(progressAttempts).toContainText('следующая доступность');
    await expect(progressAttempts).toContainText('вердикт in_progress: Система-владелец обрабатывает (in_progress)');
    await expect(progressAttempts).toContainText('Корреляция провайдера: syn-');

    // 6. The committed verdict demands the structural evidence: the
    // fixture clock walks past the lease window (the expiry recovers
    // the chain within the bound — the second attempt is eligible), the
    // scripted evidence triple and the verdict land the receipt v2 with
    // the identifier readback — the versions of the chain stay visible,
    // the sensitive material NEVER enters the readback.
    await api.post('/fixture/reconcile-evidence', { form: { object_id: 'ext-journey-rec-77', object_version: '3' } });
    await api.post('/fixture/reconcile-verdict', { form: { verdict: 'committed', correlation: 'journey-rec-recon-77' } });
    await api.post('/fixture/clock', { form: { minutes: '1' } });
    const committedRun = await postReconcile(api);
    expect(committedRun.status()).toBe(201);
    const committedReport = (await committedRun.json()).reconcile;
    const committedLines = committedReport.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(committedLines.length).toBe(1);
    expect(committedLines[0].attempt_no).toBe(2);
    expect(committedLines[0].text).toContain('Сверка подтвердила терминальный успех (committed)');
    const committedView = await (await api.get(machinePath)).json();
    const committedOp = committedView.authorizations.operations.find((o: any) => o.operation_id === operation1ID);
    expect(committedOp.state).toBe('committed');
    expect(committedOp.receipts.length).toBe(2);
    expect(committedOp.receipts[0].receipt_version_no).toBe(1);
    expect(committedOp.receipts[0].outcome).toBe('unknown');
    expect(committedOp.receipts[1].receipt_version_no).toBe(2);
    expect(committedOp.receipts[1].outcome).toBe('committed');
    expect(committedOp.receipts[1].external_object_id).toBe('ext-journey-rec-77');
    expect(committedOp.receipts[1].external_object_version).toBe('3');
    expect(committedOp.receipts[1].case_id).toBe(caseA);
    expect(committedOp.receipts[1].work_item_id).toBe(workItemID);
    expect(committedOp.receipts[1].preview_id).toBe(preview1ID);
    expect(committedOp.receipts[1].confirmation_id).toBe(confirmation1ID);
    expect(committedOp.receipts[1].definition_id).toBe('notify.manual-only');
    expect(committedOp.reconciliation_attempts.length).toBe(2);
    expect(committedOp.reconciliation_attempts[1].verdict).toBe('committed');
    expect(committedOp.reconciliation_attempts[1].correlation_id).toBe('journey-rec-recon-77');
    expect(committedOp.reconciliation_attempts[1].external_object_id).toBe('ext-journey-rec-77');
    const committedReceiptJSON = JSON.stringify(committedOp.receipts[1]);
    expect(committedReceiptJSON).not.toContain('+79000000711');
    await page.goto(reviewPath);
    const committedCard = page.locator('.action-card').filter({ hasText: operation1ID });
    await expect(committedCard).toContainText('Состояние жизни: committed');
    await expect(committedCard).toContainText('Следующего шага нет: терминальный успех');
    const receiptCard = committedCard.locator('details', { hasText: 'Карточка Action Receipt' });
    await expect(receiptCard).toHaveAttribute('open', '');
    await expect(receiptCard).toContainText('Версия квитанции 1 — исход unknown');
    await expect(receiptCard).toContainText('Версия квитанции 2 — исход committed');
    await expect(receiptCard).toContainText('Внешний объект: ext-journey-rec-77 (версия 3)');
    await expect(receiptCard).toContainText(`Кейс ${caseA}`);
    await expect(receiptCard).toContainText(`рабочий элемент ${workItemID}`);
    await expect(receiptCard).toContainText(`пин preview ${preview1ID}`);
    await expect(receiptCard).toContainText(`пин Confirmation ${confirmation1ID}`);
    await expect(receiptCard).toContainText('определение notify.manual-only@1.0.0');
    const receiptText = await receiptCard.innerText();
    expect(receiptText).not.toContain('+79000000711');
    const committedAttempts = committedCard.locator('details', { hasText: 'Попытки сверки' });
    await expect(committedAttempts).not.toHaveAttribute('open');
    await committedAttempts.locator('summary').click();
    await expect(committedAttempts).toContainText('вердикт in_progress: Система-владелец обрабатывает');
    await expect(committedAttempts).toContainText('вердикт committed: Сверка подтвердила терминальный успех');
    await expect(committedAttempts).toContainText('Корреляция провайдера: journey-rec-recon-77');
    await expect(committedAttempts).toContainText('Внешний объект сверки: ext-journey-rec-77');

    // 7. The no_effect verdict proves the ABSENCE: the second operation
    // of the case walks the same live chain into the honest unknown,
    // the scripted no_effect lands the failed pair {ledger failed,
    // receipt failed} whose CONTENT carries the absence proof (the
    // dictionaries never grow — the proof rides the receipt), the
    // report line speaks the retry-unlock word; the unlock drain tail
    // asserts the invariant words only (the fixture gap note above —
    // the production classification unlocks the retry, the journeys
    // classification honestly skips today).
    const proposal2 = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a702',
      typed_input: { phone: '+79000000712' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal2.status()).toBe(200);
    const preview2ID = (await proposal2.json()).proposal.preview_id;
    const confirm2 = await api.post(`${apiBase}/cases/${caseA}/action-previews/${preview2ID}/versions/1/confirmations`, { data: { lifetime_seconds: 3600 } });
    expect(confirm2.status()).toBe(201);
    const confirmation2ID = (await confirm2.json()).confirmation.confirmation_id;
    const authorize2 = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmation2ID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-rec-key-2' } });
    expect(authorize2.status()).toBe(201);
    const operation2ID = (await authorize2.json()).authorization.operation.operation_id;
    const unknownDrain2 = await postDrain(api);
    expect(unknownDrain2.status()).toBe(201);
    const unknown2Report = (await unknownDrain2.json()).drain;
    const unknown2Lines = unknown2Report.reports.filter((l: any) => l.operation_id === operation2ID);
    expect(unknown2Lines.length).toBe(1);
    expect(unknown2Lines[0].text).toContain('Исход неизвестен (unknown)');
    // The evidence triple resets to the honest empty (the committed word
    // of the previous verdict never leaks into the absence proof — the
    // no_effect receipt carries NO external object).
    await api.post('/fixture/reconcile-evidence', { form: { object_id: '', object_version: '' } });
    await api.post('/fixture/reconcile-verdict', { form: { verdict: 'no_effect', correlation: 'journey-rec-recon-81' } });
    const noEffectRun = await postReconcile(api);
    expect(noEffectRun.status()).toBe(201);
    const noEffectReport = (await noEffectRun.json()).reconcile;
    const noEffectLines = noEffectReport.reports.filter((l: any) => l.operation_id === operation2ID);
    expect(noEffectLines.length).toBe(1);
    expect(noEffectLines[0].text).toContain('Сверка доказала отсутствие эффекта (no_effect)');
    expect(noEffectLines[0].text).toContain('повтор разрешён решением политики');
    const noEffectView = await (await api.get(machinePath)).json();
    const noEffectOp = noEffectView.authorizations.operations.find((o: any) => o.operation_id === operation2ID);
    expect(noEffectOp.state).toBe('failed');
    expect(noEffectOp.receipts.length).toBe(2);
    expect(noEffectOp.receipts[0].outcome).toBe('unknown');
    expect(noEffectOp.receipts[1].receipt_version_no).toBe(2);
    expect(noEffectOp.receipts[1].outcome).toBe('failed');
    expect(noEffectOp.receipts[1].external_object_id).toBe('');
    expect(noEffectOp.reconciliation_attempts[0].verdict).toBe('no_effect');
    expect(noEffectOp.reconciliation_attempts[0].correlation_id).toBe('journey-rec-recon-81');
    await page.goto(reviewPath);
    const noEffectCard = page.locator('.action-card').filter({ hasText: operation2ID });
    await expect(noEffectCard).toContainText('Состояние жизни: failed');
    await expect(noEffectCard.locator('details', { hasText: 'Карточка Action Receipt' })).toContainText('Версия квитанции 2 — исход failed');
    await expect(noEffectCard.locator('details', { hasText: 'Карточка Action Receipt' })).toContainText('Сверка доказала отсутствие эффекта (no_effect)');
    const noEffectAttempts = noEffectCard.locator('details', { hasText: 'Попытки сверки' });
    await noEffectAttempts.locator('summary').click();
    await expect(noEffectAttempts).toContainText('вердикт no_effect: Сверка доказала отсутствие эффекта');
    await expect(noEffectAttempts).toContainText('Корреляция провайдера: journey-rec-recon-81');
    await expect(noEffectAttempts).toContainText('Внешний объект сверки не выдан');

    // The unlock drain tail (the full production port since the step-04
    // fixture upgrade): the clock walks past the dispatch lease window,
    // the failed+no_effect operation unlocks the retry WITHOUT the
    // guarantee (the proven absence replaces the guarantee role), the
    // FULL 3-6 ladder re-runs — the NEW dispatch attempt of the SAME
    // operation (the same external key and digest, no new intent, no new
    // reservation), the scripted committed verdict lands the terminal
    // success with the receipt v3 appended over the recorded history
    // (the versions stay monotonic).
    await api.post('/fixture/clock', { form: { minutes: '2' } });
    await api.post('/fixture/action-dispatch-evidence', { form: { object_id: 'ext-journey-rec-42', object_version: '5' } });
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'committed', correlation: 'journey-rec-corr-42', guarantee: '0' } });
    const unlockDrain = await postDrain(api);
    expect(unlockDrain.status()).toBe(201);
    const unlockReport = (await unlockDrain.json()).drain;
    expect(unlockReport.considered).toBe(unlockReport.reports.length);
    const unlockLines = unlockReport.reports.filter((l: any) => l.operation_id === operation2ID);
    expect(unlockLines.length).toBe(1);
    expect(unlockLines[0].text).toContain('Завершено: внешний объект создан (committed)');
    const unlockView = await (await api.get(machinePath)).json();
    const unlockOp = unlockView.authorizations.operations.find((o: any) => o.operation_id === operation2ID);
    expect(unlockOp.state).toBe('committed');
    expect(unlockOp.receipts.length).toBe(3);
    expect(unlockOp.receipts[0].outcome).toBe('unknown');
    expect(unlockOp.receipts[1].outcome).toBe('failed');
    expect(unlockOp.receipts[2].receipt_version_no).toBe(3);
    expect(unlockOp.receipts[2].outcome).toBe('committed');
    expect(unlockOp.receipts[2].external_object_id).toBe('ext-journey-rec-42');
    await page.goto(reviewPath);
    const unlockCard = page.locator('.action-card').filter({ hasText: operation2ID });
    await expect(unlockCard).toContainText('Состояние жизни: committed');
    await expect(unlockCard.locator('details', { hasText: 'Карточка Action Receipt' })).toContainText('Версия квитанции 3 — исход committed');

    // 8. The rights rows: the member without the operations scope hears
    // the honest 403 with the reconcile scope words (the reconciliation
    // is an operator command), the cross-tenant reconcile and page
    // reads are the plain 404 (no disclosure that the tenant reconciles
    // actions).
    await api.post('/fixture/operator', { form: { second: 'ops' } });
    const opsReconcile = await postReconcile(api);
    expect(opsReconcile.status()).toBe(403);
    expect((await opsReconcile.json()).message).toContain('полномочия operations');
    await api.post('/fixture/operator', { form: { second: '' } });
    const foreignReconcile = await api.post(`/v1/tenants/${foreignTenantID}/operations/reconcile`, { data: {}, headers: jsonHeaders });
    expect(foreignReconcile.status()).toBe(404);
    const foreignPageView = await api.get(`/app/tenants/${foreignTenantID}/cases/${caseA}/operations`);
    expect(foreignPageView.status()).toBe(404);

    // 9. The a11y close at 320px: the skip-link, the semantic heading
    // ladder, the ≥44×44 targets of the reconcile disclosure summary,
    // the confirm checkbox and the primary button, the KEYBOARD toggle
    // of a CLOSED reconcile-attempts disclosure and the single column
    // without the horizontal scroll.
    await page.goto(reviewPath);
    await expect(page.locator('a.skip-link')).toBeVisible();
    await expect(page.locator('h1')).toContainText('Operation Review');
    const headings = await page.locator('h2').all();
    expect(headings.length).toBeGreaterThan(0);
    const reconcileSummary = page.locator('details', { hasText: 'Запустить сверку unknown-операций' }).first().locator('summary');
    const reconcileSummaryBox = await reconcileSummary.boundingBox();
    expect(reconcileSummaryBox!.height).toBeGreaterThanOrEqual(44);
    const reconcileCheckbox = page.locator('details', { hasText: 'Запустить сверку unknown-операций' }).first().locator('input[name="confirm_reconcile"]');
    const reconcileCheckboxBox = await reconcileCheckbox.boundingBox();
    expect(reconcileCheckboxBox!.height).toBeGreaterThanOrEqual(44);
    expect(reconcileCheckboxBox!.width).toBeGreaterThanOrEqual(44);
    const reconcileButton = page.locator('details', { hasText: 'Запустить сверку unknown-операций' }).first().locator('button.primary', { name: 'Запустить сверку' });
    const reconcileButtonBox = await reconcileButton.boundingBox();
    expect(reconcileButtonBox!.height).toBeGreaterThanOrEqual(44);
    expect(reconcileButtonBox!.width).toBeGreaterThanOrEqual(44);
    const closedReconcileAttempts = page.locator('.action-card').filter({ hasText: operation1ID }).locator('details', { hasText: 'Попытки сверки' });
    await expect(closedReconcileAttempts).not.toHaveAttribute('open');
    const closedReconcileSummary = closedReconcileAttempts.locator('summary');
    await closedReconcileSummary.focus();
    await closedReconcileSummary.press('Enter');
    await expect(closedReconcileAttempts).toHaveAttribute('open', '');
    await expect(closedReconcileAttempts).toContainText('вердикт committed: Сверка подтвердила терминальный успех');
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    // The fixture reset: the next journey starts at the honest
    // production default (the empty dictionary, the primary owner, the
    // live clock, the step-up off, the default adapter scripts of BOTH
    // contours — the read-only in_progress word, the empty evidence).
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '0' } });
    await api.post('/fixture/operator', { form: { second: '' } });
    await api.post('/fixture/step-up', { form: { enabled: '0' } });
    await api.post('/fixture/clock', { form: { minutes: '0' } });
    await api.post('/fixture/preview-material', { form: { field: 'order_status', status: '' } });
    await api.post('/fixture/action-dispatch-evidence', { form: { object_id: '', object_version: '' } });
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'accepted', guarantee: '0' } });
    await api.post('/fixture/reconcile-evidence', { form: { object_id: '', object_version: '' } });
    await api.post('/fixture/reconcile-verdict', { form: { verdict: 'in_progress' } });
  }
});
