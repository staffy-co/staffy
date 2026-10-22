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

// The machine recovery-case creation command of ONE operation (the same
// strict EMPTY body — the words of the assignment NEVER come from a
// command parameter).
async function postRecoveryCase(api: any, operationID: string) {
  return api.post(`${apiBase}/operations/${operationID}/recovery-case`, { data: {}, headers: jsonHeaders });
}

// The Story 3.8 chain (z36) at 320px: the human contour of the spent
// automation. The z35 unknown world reused to the honest EXHAUSTION
// (three live no_answer attempts of the reconciliation with the fixture
// clock walks, the fourth command speaks the bounded exhaustion word),
// the eligible operation card flips its NEXT-STEP word to the recovery
// instruction (rule 161 — the eligible card speaks the NEW word, the
// not-yet-eligible card keeps the OLD 3-7 word), the deliberate checkbox
// discipline of the creation form with the PRG notice and the focus
// move, the case card readback of the SAFE pins (the number, the owner
// of the live ownership term, the backup and the deadline of the human
// coverage — the phone and the material NEVER enter), the panel list,
// the idempotent repeat of the UNCHANGED digest, the AG-4 world birth on
// the reconciliation_unavailable trigger (the nil-adapter composition —
// the integration matrix row-3 structure), the later wired
// reconciliation («сверка подключается») landing the committed verdict
// with the receipt v2, the evidence repeat — ONE append-only
// evidence_appended row with the SAFE delta over the never-rewritten
// bundle, the honest ladder refusals (the committed word BEFORE any
// write, the not-exhausted word of the live automation right), the
// rights rows and the a11y close (the skip-link, the ≥44×44 targets of
// the form, the CLOSED attempts disclosure walking the keyboard, the
// 320px column without the sideways scroll).
test('recovery case: the human contour of the spent automation at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 0. The fixture switches of the vertical (the z35 precedent): the
    // preview dictionary, the REAL telephony connection with the
    // verified read-only channel word and the LIVE entitlement walk; the
    // step-up stays on for the machine confirmations of the chain.
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '1' } });
    await api.post('/fixture/step-up', { form: { enabled: '1' } });
    const created = await api.post(`${apiBase}/connections`, { data: {
      connector_id: 'staffy-synthetic-telephony', connector_version: '1.1.0', label: 'Телефония восстановления', environment: 'sandbox',
      config: { endpoint: 'https://recovery.sandbox.example/api' }
    } });
    expect(created.status()).toBe(201);
    const createdView = await created.json();
    const connectionID = createdView.connection.connection_id;
    const connect = await api.post(`${apiBase}/connections/${connectionID}/connect`, { data: { version: createdView.connection.version, secrets: { api_key: 'sandbox-recovery-key' } } });
    expect(connect.status()).toBe(200);
    const verify = await api.post(`${apiBase}/connections/${connectionID}/capabilities/query.trunk-status/verify`, { data: {} });
    expect(verify.status()).toBe(200);

    const entitlement = await (await api.get(`${apiBase}/entitlement`)).json();
    if (entitlement.state === 'paused') {
      const resume = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'resume', basis: 'Возобновление для восстановления', confirm: true } });
      expect(resume.status()).toBeLessThan(300);
    } else if (entitlement.state !== 'active') {
      const terms = await api.post(`${apiBase}/entitlement/terms`, { data: {
        currency: 'RUB', tax_regime: 'УСН', period_start: '2026-10-01T00:00:00Z', period_end: '2026-12-31T23:59:59Z',
        included_employees: ['consultant-01'], included_channels: ['jivo'], included_connectors: ['abcp'],
        launch_price: 49000, subscription_price: 15000, change_note: 'Коммерческое предложение восстановления'
      } });
      expect(terms.status()).toBeLessThan(300);
      const activate = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'activate', basis: 'Договор №82 от 01.10.2026', confirm: true } });
      expect(activate.status()).toBeLessThan(300);
    }

    // 1. The eligible operation born live: one TEXT turn opens the case,
    // the machine proposal, the confirmation and the authorization land
    // the queued operation, the broken adapter call of the drain answers
    // the honest unknown (the receipt v1). The recovery ladder refuses
    // the premature command by the honest not-exhausted word (the
    // automation right is not taken away), the card keeps the OLD 3-7
    // next-step word (rule 161), the strict decode refuses the free
    // field BEFORE the service and the panel names the honest absence.
    const first = await postIngress(api, textInboundEvent('journey-rec8-evt-1', 'journey-rec8-sess-1', { say: 'Подтвердите перенос созвона и авторизуйте уведомление' }), 'journey-rec8-ingress-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const caseView0 = await (await api.get(`${apiBase}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;
    const machinePath = `${apiBase}/cases/${caseA}/operations`;
    const reviewPath = `/app/tenants/${tenantID}/cases/${caseA}/operations`;
    const proposal = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a801',
      typed_input: { phone: '+79000000721' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal.status()).toBe(200);
    expect((await proposal.json()).proposal.verdict).toBe('valid');
    const preview1ID = (await proposal.json()).proposal.preview_id;
    const confirm1 = await api.post(`${apiBase}/cases/${caseA}/action-previews/${preview1ID}/versions/1/confirmations`, { data: { lifetime_seconds: 3600 } });
    expect(confirm1.status()).toBe(201);
    const confirmation1ID = (await confirm1.json()).confirmation.confirmation_id;
    const authorize1 = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmation1ID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-rec8-key-1' } });
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

    // The strict decode of the creation command: the free field never
    // reaches the service (the assignment words never come from a
    // parameter).
    const freeField = await api.post(`${apiBase}/operations/${operation1ID}/recovery-case`, { data: { note: 'x' }, headers: jsonHeaders });
    expect(freeField.status()).toBe(400);
    expect((await freeField.json()).message).toContain('один объект с известными полями');

    // The premature ladder gas: the unknown operation with the attempts
    // NOT spent and the live reconciliation wired and declared — the
    // honest refusal word of the exact gas, NOTHING is written.
    const premature = await postRecoveryCase(api, operation1ID);
    expect(premature.status()).toBe(403);
    expect((await premature.json()).message).toContain('Автоматическая сверка не исчерпана');

    // The honest empty panel of the vertical.
    await page.goto(`/app/tenants/${tenantID}/recovery-cases`);
    await expect(page.locator('h1')).toContainText('Recovery Panel');
    await expect(page.locator('.status')).toContainText('Recovery Case пока нет');

    // The OLD word of the not-yet-eligible card (rule 161: the slice
    // changes the next-step word of the ELIGIBLE operations only — the
    // inherited state texts never move).
    await page.goto(reviewPath);
    const earlyCard = page.locator('.action-card').filter({ hasText: operation1ID });
    await expect(earlyCard).toContainText('Состояние жизни: unknown');
    await expect(earlyCard).toContainText('Следующий шаг: сверка исхода командой reconcile');
    await expect(earlyCard.locator('span', { hasText: 'Триггер восстановления' })).toHaveCount(0);
    await expect(earlyCard.locator('details', { hasText: 'Создать Recovery Case' })).toHaveCount(0);

    // 2. The live exhaustion: three no_answer attempts with the fixture
    // clock walks (the backoff windows of the lease ladder), the fourth
    // command speaks the bounded exhaustion word (no query, no new
    // attempt), the card flips to the recovery word with the trigger
    // span and the creation form (rule 161).
    await api.post('/fixture/reconcile-verdict', { form: { verdict: 'no_answer' } });
    const run1 = await postReconcile(api);
    expect(run1.status()).toBe(201);
    const report1 = (await run1.json()).reconcile;
    const lines1 = report1.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(lines1.length).toBe(1);
    expect(lines1[0].attempt_no).toBe(1);
    expect(lines1[0].text).toContain('Сверка не дала вердикта (no_answer)');
    // The unknown-with-attempts gas: the ladder keeps refusing honestly
    // (the recorded attempts below the bound do not open the human word).
    const premature2 = await postRecoveryCase(api, operation1ID);
    expect(premature2.status()).toBe(403);
    expect((await premature2.json()).message).toContain('Автоматическая сверка не исчерпана');
    await api.post('/fixture/clock', { form: { minutes: '1' } });
    const run2 = await postReconcile(api);
    expect(run2.status()).toBe(201);
    const lines2 = (await run2.json()).reconcile.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(lines2.length).toBe(1);
    expect(lines2[0].attempt_no).toBe(2);
    expect(lines2[0].text).toContain('Сверка не дала вердикта (no_answer)');
    await api.post('/fixture/clock', { form: { minutes: '2' } });
    const run3 = await postReconcile(api);
    expect(run3.status()).toBe(201);
    const lines3 = (await run3.json()).reconcile.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(lines3.length).toBe(1);
    expect(lines3[0].attempt_no).toBe(3);
    expect(lines3[0].text).toContain('Сверка не дала вердикта (no_answer)');
    // The clock walks past the third lease window: the bounded batch
    // honestly closes the automatic contour — the fourth command speaks
    // the exhaustion word (no query, no new attempt, no lease).
    await api.post('/fixture/clock', { form: { minutes: '3' } });
    const run4 = await postReconcile(api);
    expect(run4.status()).toBe(201);
    const lines4 = (await run4.json()).reconcile.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(lines4.length).toBe(1);
    expect(lines4[0].text).toContain('Автоматические попытки сверки исчерпаны');
    const spentView = await (await api.get(machinePath)).json();
    const spentOp = spentView.authorizations.operations.find((o: any) => o.operation_id === operation1ID);
    expect(spentOp.state).toBe('unknown');
    expect(spentOp.reconciliation_attempts.length).toBe(3);
    expect(spentOp.reconciliation_attempts[2].verdict).toBe('no_answer');

    // The eligible card: the NEW next-step word, the trigger span with
    // its fixed text and the deliberate creation form.
    await page.goto(reviewPath);
    const eligibleCard = page.locator('.action-card').filter({ hasText: operation1ID });
    await expect(eligibleCard).toContainText('Состояние жизни: unknown');
    await expect(eligibleCard).toContainText('Следующий шаг: требуется человек — создайте Recovery Case (контур 3-8)');
    await expect(eligibleCard).toContainText('Триггер восстановления: reconciliation_exhausted');
    await expect(eligibleCard).toContainText('автоматические попытки сверки исчерпаны (3/3)');

    // 3. The deliberate checkbox discipline of the creation form (the
    // unchecked form never posts), the a11y targets of the form at 320px,
    // the confirmed command lands the PRG notice with the focus move, the
    // case card readback speaks the SAFE pins only — the phone and the
    // material NEVER enter.
    const recoveryDetails = eligibleCard.locator('details', { hasText: 'Создать Recovery Case' });
    const recoverySummaryBox = await recoveryDetails.locator('summary').boundingBox();
    expect(recoverySummaryBox!.height).toBeGreaterThanOrEqual(44);
    const recoveryCheckbox = recoveryDetails.locator('input[name="confirm_recovery"]');
    const recoveryCheckboxBox = await recoveryCheckbox.boundingBox();
    expect(recoveryCheckboxBox!.height).toBeGreaterThanOrEqual(44);
    expect(recoveryCheckboxBox!.width).toBeGreaterThanOrEqual(44);
    const recoveryButton = recoveryDetails.locator('button.primary', { name: 'Создать Recovery Case' });
    const recoveryButtonBox = await recoveryButton.boundingBox();
    expect(recoveryButtonBox!.height).toBeGreaterThanOrEqual(44);
    expect(recoveryButtonBox!.width).toBeGreaterThanOrEqual(44);
    await recoveryDetails.locator('summary').click();
    await recoveryButton.click();
    await expect(page).not.toHaveURL(/\?recovery=/);
    await recoveryCheckbox.check();
    await recoveryButton.click();
    await expect(page).toHaveURL(/\/app\/tenants\/[0-9a-f-]+\/recovery-cases\/[0-9a-f-]+\?recovery=created$/);
    await expect(page.locator('#page-notice')).toContainText('Recovery Case создан: назначение живыми чтениями');
    await expect(page.locator('#page-notice')).toBeFocused();
    const caseID = page.url().match(/recovery-cases\/([0-9a-f-]{36})/)[1];

    // 4. The case card readback: the read-time open state with its honest
    // text, the trigger word, the assignment of the LIVE readings (the
    // owner triple of the current ownership term, the backup and the
    // deadline of the human coverage), the SAFE link pins with the living
    // receipt chain, the append-only history and the honest boundary
    // texts of the surface.
    await expect(page.locator('h1')).toContainText('Recovery Case №1');
    const machineCard = await (await api.get(`${apiBase}/recovery-cases/${caseID}`)).json();
    const cardView = machineCard.recovery_case;
    expect(cardView.recovery_case_no).toBe(1);
    expect(cardView.trigger).toBe('reconciliation_exhausted');
    expect(cardView.state).toBe('open');
    expect(cardView.assignment.owner_kind).toBe('human');
    expect(cardView.assignment.owner_epoch).toBe(1);
    expect(cardView.assignment.backup_owner_id).toBe('01994f64-6c00-7000-8000-00000000000a');
    expect(cardView.assignment.ack_deadline_minutes).toBe(120);
    expect(cardView.links.receipt_versions_count).toBe(1);
    expect(cardView.links.latest_receipt_version_no).toBe(1);
    expect(cardView.links.latest_receipt_outcome).toBe('unknown');
    expect(cardView.links.dispatch_attempts).toBe(1);
    expect(cardView.links.reconcile_attempts).toBe(3);
    expect(cardView.events.length).toBe(1);
    expect(cardView.events[0].kind).toBe('created');
    expect(cardView.events[0].details).toContain('триггер reconciliation_exhausted');
    await expect(page.locator('#main')).toContainText('Состояние: open — Recovery Case открыт (open)');
    await expect(page.locator('#main')).toContainText('Триггер: reconciliation_exhausted');
    await expect(page.locator('#main')).toContainText(`Операция: ${operation1ID}`);
    await expect(page.locator('#main')).toContainText('Определение: notify.manual-only@1.0.0');
    await expect(page.locator('#main')).toContainText(`Исходный кейс: ${caseA}`);
    await expect(page.locator('#main')).toContainText(`рабочий элемент: ${workItemID}`);
    await expect(page.locator('#main')).toContainText(`human ${cardView.assignment.owner_ref} (эпоха 1)`);
    await expect(page.locator('#main')).toContainText(`Backup (human coverage маршрута): ${cardView.assignment.backup_owner_id}`);
    await expect(page.locator('#main')).toContainText('Дедлайн подтверждения:');
    await expect(page.locator('#main')).toContainText(`Пин preview: ${preview1ID}:1`);
    await expect(page.locator('#main')).toContainText(`Пин Confirmation: ${confirmation1ID}:1`);
    await expect(page.locator('#main')).toContainText('Цепочка квитанций: версий 1, последняя v1 — исход unknown');
    await expect(page.locator('#main')).toContainText('Попытки: dispatch 1/3, сверка 3/3');
    await expect(page.locator('#main')).toContainText('Внешний объект не выдан');
    await expect(page.locator('#main')).toContainText('Owning System: erp');
    await expect(page.locator('#main')).toContainText('Пины наблюдений Owning System: нет');
    await expect(page.locator('#main')).toContainText('Платёжное состояние unknown');
    await expect(page.locator('#main')).toContainText('Локальная проекция — не актуальное внешнее состояние');
    await expect(page.locator('#main')).toContainText('Событие №1 — created: Recovery Case создан: триггер reconciliation_exhausted');
    const caseCardText = await page.locator('#main').innerText();
    expect(caseCardText).not.toContain('+79000000721');

    // 5. The panel readback and the machine list of the tenant, then the
    // idempotent repeat: the UNCHANGED digest answers the EXISTING case —
    // NO second row, NO event.
    await page.goto(`/app/tenants/${tenantID}/recovery-cases`);
    await expect(page.locator('h1')).toContainText('Recovery Panel');
    const panelRow = page.locator('li.action-card').filter({ hasText: operation1ID });
    await expect(panelRow).toContainText('Recovery Case №1');
    await expect(panelRow).toContainText('Триггер: reconciliation_exhausted');
    await expect(panelRow).toContainText(`human ${cardView.assignment.owner_ref}`);
    await expect(panelRow).toContainText(cardView.assignment.backup_owner_id);
    await expect(panelRow).toContainText('Дедлайн подтверждения:');
    const listJSON = await (await api.get(`${apiBase}/recovery-cases`)).json();
    const listRow = listJSON.recovery_cases.find((c: any) => c.operation_id === operation1ID);
    expect(listRow.recovery_case_no).toBe(1);
    expect(listRow.trigger).toBe('reconciliation_exhausted');
    expect(listRow.state).toBe('open');
    const repeat = await postRecoveryCase(api, operation1ID);
    expect(repeat.status()).toBe(200);
    const repeatView = (await repeat.json()).recovery_case;
    expect(repeatView.decision).toBe('idempotent');
    expect(repeatView.text).toContain('Recovery Case уже существует');
    expect(repeatView.recovery_case.recovery_case_id).toBe(caseID);
    const cardAfterRepeat = await (await api.get(`${apiBase}/recovery-cases/${caseID}`)).json();
    expect(cardAfterRepeat.recovery_case.events.length).toBe(1);

    // 6. The later terminal evidence of the AG-4 world: the second
    // operation walks the nil-adapter composition (the integration
    // matrix row-3 structure) — the reconcile command refuses in words
    // there, the recovery eligibility answers reconciliation_unavailable
    // (the human is mandatory, the attempts were never spent), the case
    // is born on the UNAVAILABLE trigger. Then «сверка подключается»:
    // the world returns to the wired composition, the committed verdict
    // lands the receipt v2, the repeat answers the EVIDENCE append — ONE
    // append-only row with the SAFE delta over the never-rewritten
    // bundle, the fresh card derives the LIVING chain (AC-10).
    await api.post('/fixture/recovery-ag4', { form: { enabled: '1' } });
    const ag4Reconcile = await postReconcile(api);
    expect(ag4Reconcile.status()).toBe(403);
    expect((await ag4Reconcile.json()).message).toContain('Сверка не подключена');
    const proposal2 = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a802',
      typed_input: { phone: '+79000000722' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal2.status()).toBe(200);
    const preview2ID = (await proposal2.json()).proposal.preview_id;
    const confirm2 = await api.post(`${apiBase}/cases/${caseA}/action-previews/${preview2ID}/versions/1/confirmations`, { data: { lifetime_seconds: 3600 } });
    expect(confirm2.status()).toBe(201);
    const confirmation2ID = (await confirm2.json()).confirmation.confirmation_id;
    const authorize2 = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmation2ID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-rec8-key-2' } });
    expect(authorize2.status()).toBe(201);
    const operation2ID = (await authorize2.json()).authorization.operation.operation_id;
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'timeout', failure: '1', guarantee: '0' } });
    const unknownDrain2 = await postDrain(api);
    expect(unknownDrain2.status()).toBe(201);
    const unknown2Lines = (await unknownDrain2.json()).drain.reports.filter((l: any) => l.operation_id === operation2ID);
    expect(unknown2Lines.length).toBe(1);
    expect(unknown2Lines[0].text).toContain('Исход неизвестен (unknown)');
    const ag4Birth = await postRecoveryCase(api, operation2ID);
    expect(ag4Birth.status()).toBe(201);
    const birthView = (await ag4Birth.json()).recovery_case;
    expect(birthView.decision).toBe('created');
    expect(birthView.text).toContain('исходные записи не переписаны');
    expect(birthView.recovery_case.trigger).toBe('reconciliation_unavailable');
    expect(birthView.recovery_case.recovery_case_no).toBe(2);
    expect(birthView.recovery_case.links.reconcile_attempts).toBe(0);
    expect(birthView.recovery_case.links.receipt_versions_count).toBe(1);
    // «Сверка подключается»: the wired composition returns, the committed
    // verdict with the structural evidence lands the receipt v2.
    await api.post('/fixture/recovery-ag4', { form: { enabled: '0' } });
    await api.post('/fixture/reconcile-evidence', { form: { object_id: 'ext-journey-rec8-77', object_version: '9' } });
    await api.post('/fixture/reconcile-verdict', { form: { verdict: 'committed', correlation: 'journey-rec8-recon-77' } });
    const wiredRun = await postReconcile(api);
    expect(wiredRun.status()).toBe(201);
    const wiredLines = (await wiredRun.json()).reconcile.reports.filter((l: any) => l.operation_id === operation2ID);
    expect(wiredLines.length).toBe(1);
    expect(wiredLines[0].attempt_no).toBe(1);
    expect(wiredLines[0].text).toContain('Сверка подтвердила терминальный успех (committed)');
    const committed2View = await (await api.get(machinePath)).json();
    const committed2Op = committed2View.authorizations.operations.find((o: any) => o.operation_id === operation2ID);
    expect(committed2Op.state).toBe('committed');
    expect(committed2Op.receipts.length).toBe(2);
    expect(committed2Op.receipts[1].receipt_version_no).toBe(2);
    expect(committed2Op.receipts[1].outcome).toBe('committed');
    expect(committed2Op.receipts[1].external_object_id).toBe('ext-journey-rec8-77');
    // The evidence repeat: the existing case bypasses the eligibility
    // gate (the repeat is a COMPARATOR question — AC-5), the moved digest
    // appends ONE row with the SAFE delta.
    const delta = await postRecoveryCase(api, operation2ID);
    expect(delta.status()).toBe(200);
    const deltaView = (await delta.json()).recovery_case;
    expect(deltaView.decision).toBe('evidence');
    expect(deltaView.text).toContain('Новые доказательства добавлены');
    expect(deltaView.recovery_case.events.length).toBe(2);
    expect(deltaView.recovery_case.events[1].kind).toBe('evidence_appended');
    expect(deltaView.recovery_case.events[1].details).toContain('reconcile-попытки 0→1');
    expect(deltaView.recovery_case.events[1].details).toContain('квант-квитанций 1→2');
    expect(deltaView.recovery_case.events[1].details).toContain('исход-квитанции unknown→committed');
    expect(deltaView.recovery_case.events[1].details).toContain('внешний-id →ext-journey-rec8-77');
    expect(deltaView.recovery_case.links.receipt_versions_count).toBe(2);
    expect(deltaView.recovery_case.links.latest_receipt_outcome).toBe('committed');
    expect(deltaView.recovery_case.links.reconcile_attempts).toBe(1);
    const case2ID = deltaView.recovery_case.recovery_case_id;
    await page.goto(`/app/tenants/${tenantID}/recovery-cases/${case2ID}`);
    await expect(page.locator('#main')).toContainText('Recovery Case №2');
    await expect(page.locator('#main')).toContainText('Цепочка квитанций: версий 2, последняя v2 — исход committed');
    await expect(page.locator('#main')).toContainText('Попытки: dispatch 1/3, сверка 1/3');
    await expect(page.locator('#main')).toContainText('Внешний объект: ext-journey-rec8-77 (версия 9)');
    await expect(page.locator('#main')).toContainText('Событие №2 — evidence_appended');
    const case2Text = await page.locator('#main').innerText();
    expect(case2Text).not.toContain('+79000000722');
    // The operation card after the terminal success: the case link stays
    // (the recovery case is forever), the trigger span honestly
    // disappears, the next step is the terminal word.
    await page.goto(reviewPath);
    const committedCard = page.locator('.action-card').filter({ hasText: operation2ID });
    await expect(committedCard).toContainText('Состояние жизни: committed');
    await expect(committedCard).toContainText('Следующего шага нет: терминальный успех');
    await expect(committedCard).toContainText('Recovery Case: №2');
    await expect(committedCard.locator('span', { hasText: 'Триггер восстановления' })).toHaveCount(0);

    // 7. The committed refusal: a NEW operation walks the FIRST committed
    // dispatch (NO case exists) — the ladder answers the honest committed
    // word BEFORE any write; the machine list carries no case for it.
    const proposal3 = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a803',
      typed_input: { phone: '+79000000723' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal3.status()).toBe(200);
    const preview3ID = (await proposal3.json()).proposal.preview_id;
    const confirm3 = await api.post(`${apiBase}/cases/${caseA}/action-previews/${preview3ID}/versions/1/confirmations`, { data: { lifetime_seconds: 3600 } });
    expect(confirm3.status()).toBe(201);
    const confirmation3ID = (await confirm3.json()).confirmation.confirmation_id;
    const authorize3 = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmation3ID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-rec8-key-3' } });
    expect(authorize3.status()).toBe(201);
    const operation3ID = (await authorize3.json()).authorization.operation.operation_id;
    await api.post('/fixture/action-dispatch-evidence', { form: { object_id: 'ext-journey-rec8-33', object_version: '4' } });
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'committed', correlation: 'journey-rec8-corr-33', guarantee: '0' } });
    const committedDrain = await postDrain(api);
    expect(committedDrain.status()).toBe(201);
    const committed3Lines = (await committedDrain.json()).drain.reports.filter((l: any) => l.operation_id === operation3ID);
    expect(committed3Lines.length).toBe(1);
    expect(committed3Lines[0].text).toContain('Завершено: внешний объект создан (committed)');
    const refused = await postRecoveryCase(api, operation3ID);
    expect(refused.status()).toBe(403);
    expect((await refused.json()).message).toContain('Восстановление не требуется: итог committed');
    const listAfterRefusal = await (await api.get(`${apiBase}/recovery-cases`)).json();
    expect(listAfterRefusal.recovery_cases.find((c: any) => c.operation_id === operation3ID)).toBeUndefined();

    // 8. The rights rows: the member without the operations scope hears
    // the honest 403 with the scope words (the recovery is an operator
    // command), the cross-tenant surfaces are the plain 404 (no
    // disclosure that the tenant recovers actions at all).
    await api.post('/fixture/operator', { form: { second: 'ops' } });
    const opsRecovery = await postRecoveryCase(api, operation1ID);
    expect(opsRecovery.status()).toBe(403);
    expect((await opsRecovery.json()).message).toContain('полномочия operations');
    await api.post('/fixture/operator', { form: { second: '' } });
    const foreignCreate = await api.post(`/v1/tenants/${foreignTenantID}/operations/${operation1ID}/recovery-case`, { data: {}, headers: jsonHeaders });
    expect(foreignCreate.status()).toBe(404);
    const foreignList = await api.get(`/v1/tenants/${foreignTenantID}/recovery-cases`);
    expect(foreignList.status()).toBe(404);
    const foreignPanel = await api.get(`/app/tenants/${foreignTenantID}/recovery-cases`);
    expect(foreignPanel.status()).toBe(404);
    const foreignCard = await api.get(`/app/tenants/${foreignTenantID}/recovery-cases/${caseID}`);
    expect(foreignCard.status()).toBe(404);

    // 9. The a11y close at 320px: the skip-link, the semantic heading
    // ladder, the KEYBOARD toggle of a CLOSED attempts disclosure and the
    // single column without the horizontal scroll on every page of the
    // vertical.
    await page.goto(reviewPath);
    await expect(page.locator('a.skip-link')).toBeVisible();
    await expect(page.locator('h1')).toContainText('Operation Review');
    const headings = await page.locator('h2').all();
    expect(headings.length).toBeGreaterThan(0);
    const closedAttempts = page.locator('.action-card').filter({ hasText: operation1ID }).locator('details', { hasText: 'Попытки сверки' });
    await expect(closedAttempts).not.toHaveAttribute('open');
    await closedAttempts.locator('summary').focus();
    await closedAttempts.locator('summary').press('Enter');
    await expect(closedAttempts).toHaveAttribute('open', '');
    await expect(closedAttempts).toContainText('вердикт no_answer: Сверка не дала вердикта');
    const overflowOps = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflowOps).toBeLessThanOrEqual(0);
    await page.goto(`/app/tenants/${tenantID}/recovery-cases/${caseID}`);
    const overflowCase = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflowCase).toBeLessThanOrEqual(0);
    await page.goto(`/app/tenants/${tenantID}/recovery-cases`);
    const overflowPanel = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflowPanel).toBeLessThanOrEqual(0);
  } finally {
    // The fixture reset: the next journey starts at the honest
    // production default (the empty dictionary, the primary owner, the
    // live clock, the step-up off, the default adapter scripts of ALL
    // the contours — the read-only in_progress word, the empty evidence,
    // the wired recovery world).
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '0' } });
    await api.post('/fixture/operator', { form: { second: '' } });
    await api.post('/fixture/step-up', { form: { enabled: '0' } });
    await api.post('/fixture/clock', { form: { minutes: '0' } });
    await api.post('/fixture/preview-material', { form: { field: 'order_status', status: '' } });
    await api.post('/fixture/action-dispatch-evidence', { form: { object_id: '', object_version: '' } });
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'accepted', guarantee: '0' } });
    await api.post('/fixture/reconcile-evidence', { form: { object_id: '', object_version: '' } });
    await api.post('/fixture/reconcile-verdict', { form: { verdict: 'in_progress' } });
    await api.post('/fixture/recovery-ag4', { form: { enabled: '0' } });
  }
});
