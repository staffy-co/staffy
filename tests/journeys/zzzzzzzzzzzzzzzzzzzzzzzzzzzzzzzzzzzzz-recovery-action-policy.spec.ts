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

async function postDrain(api: any) {
  return api.post(`${apiBase}/operations/drain`, { data: {}, headers: jsonHeaders });
}

async function postReconcile(api: any) {
  return api.post(`${apiBase}/operations/reconcile`, { data: {}, headers: jsonHeaders });
}

async function postRecoveryCase(api: any, operationID: string) {
  return api.post(`${apiBase}/operations/${operationID}/recovery-case`, { data: {}, headers: jsonHeaders });
}

// The machine policy read of ONE case (the free query field is ignored
// by the router — the empty read never answers 400).
async function getPolicy(api: any, caseID: string, query = '') {
  return api.get(`${apiBase}/recovery-cases/${caseID}/policy${query}`);
}

// The kind-order dictionary of the live derivation (the compile-time
// dictionary IS the machine view order — AD-7 mirror).
const kindOrder = [
  'request_evidence',
  'show_recorded_conditions',
  'truthful_status_update',
  'offer_approved_options',
  'prepare_pause_cancel',
];

// The Story 3.9 chain (z37) at 320px: the read-time allowed set of the
// recovery case. The z36 exhaustion world reused to ONE case, then the
// MACHINE policy read (the five kinds in the dictionary order, the
// declared AC-2 attributes, the separate human-only class with the
// PRD FR-39 boundary words, the derivation idempotence — the repeated
// read answers the SAME view), the case card carries the NEW section
// «Разрешённые действия» (the kind words with the honest state texts,
// the material kinds with the separate-confirmation word, the closed
// attribute details, the human-only block APART) while every frozen
// 3-8 word of the card stays byte-stable (rule 161), the LIVE paused
// tenant control through the REAL 3-2 surface blocks the MATERIAL
// kinds with the EXACT control word while the read-only kinds stay
// allowed (and the lift heals the world), the AG-4 composition world
// rides the honest «Сверка не подключена» sentence on the status kind
// WITHOUT blocking the policy, the rights rows (the operations scope
// 403, the cross-tenant 404 blindness, the unknown case 404) and the
// a11y close (the skip-link, the ≥44×44 summary of the closed
// attribute details walking the keyboard, the 320px column without
// the sideways scroll).
test('recovery action policy: the read-time allowed set of the case card at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 0. The fixture switches of the vertical (the z36 precedent): the
    // preview dictionary, the REAL telephony connection with the
    // verified read-only channel word and the LIVE entitlement walk; the
    // step-up stays on for the machine confirmations of the chain and
    // the paused-control Set of the live row.
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '1' } });
    await api.post('/fixture/step-up', { form: { enabled: '1' } });
    const created = await api.post(`${apiBase}/connections`, { data: {
      connector_id: 'staffy-synthetic-telephony', connector_version: '1.1.0', label: 'Телефония политики восстановления', environment: 'sandbox',
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
      const resume = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'resume', basis: 'Возобновление для политики восстановления', confirm: true } });
      expect(resume.status()).toBeLessThan(300);
    } else if (entitlement.state !== 'active') {
      const terms = await api.post(`${apiBase}/entitlement/terms`, { data: {
        currency: 'RUB', tax_regime: 'УСН', period_start: '2026-10-01T00:00:00Z', period_end: '2026-12-31T23:59:59Z',
        included_employees: ['consultant-01'], included_channels: ['jivo'], included_connectors: ['abcp'],
        launch_price: 49000, subscription_price: 15000, change_note: 'Коммерческое предложение политики восстановления'
      } });
      expect(terms.status()).toBeLessThan(300);
      const activate = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'activate', basis: 'Договор №83 от 01.10.2026', confirm: true } });
      expect(activate.status()).toBeLessThan(300);
    }

    // 1. The exhausted case born live (the z36 chain): one TEXT turn
    // opens the case, the machine proposal, the confirmation and the
    // authorization land the queued operation, the broken adapter call
    // of the drain answers the honest unknown, three no_answer
    // reconciliation attempts with the fixture clock walks spend the
    // bounded right, the fourth command speaks the exhaustion word,
    // the creation command lands the case row.
    const first = await postIngress(api, textInboundEvent('journey-rec9-evt-1', 'journey-rec9-sess-1', { say: 'Подтвердите перенос созвона и авторизуйте уведомление' }), 'journey-rec9-ingress-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const caseView0 = await (await api.get(`${apiBase}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;
    const proposal = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a901',
      typed_input: { phone: '+79000000731' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal.status()).toBe(200);
    expect((await proposal.json()).proposal.verdict).toBe('valid');
    const preview1ID = (await proposal.json()).proposal.preview_id;
    const confirm1 = await api.post(`${apiBase}/cases/${caseA}/action-previews/${preview1ID}/versions/1/confirmations`, { data: { lifetime_seconds: 3600 } });
    expect(confirm1.status()).toBe(201);
    const confirmation1ID = (await confirm1.json()).confirmation.confirmation_id;
    const authorize1 = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmation1ID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-rec9-key-1' } });
    expect(authorize1.status()).toBe(201);
    const operation1ID = (await authorize1.json()).authorization.operation.operation_id;
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'timeout', failure: '1', guarantee: '0' } });
    const unknownDrain = await postDrain(api);
    expect(unknownDrain.status()).toBe(201);
    const unknownLines = (await unknownDrain.json()).drain.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(unknownLines.length).toBe(1);
    expect(unknownLines[0].text).toContain('Исход неизвестен (unknown)');
    await api.post('/fixture/reconcile-verdict', { form: { verdict: 'no_answer' } });
    const run1 = await postReconcile(api);
    expect(run1.status()).toBe(201);
    const lines1 = (await run1.json()).reconcile.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(lines1.length).toBe(1);
    expect(lines1[0].attempt_no).toBe(1);
    expect(lines1[0].text).toContain('Сверка не дала вердикта (no_answer)');
    await api.post('/fixture/clock', { form: { minutes: '1' } });
    const run2 = await postReconcile(api);
    expect(run2.status()).toBe(201);
    const lines2 = (await run2.json()).reconcile.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(lines2[0].attempt_no).toBe(2);
    await api.post('/fixture/clock', { form: { minutes: '2' } });
    const run3 = await postReconcile(api);
    expect(run3.status()).toBe(201);
    const lines3 = (await run3.json()).reconcile.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(lines3[0].attempt_no).toBe(3);
    // The clock walks past the third lease window: the bounded batch
    // honestly closes the automatic contour — the fourth command speaks
    // the exhaustion word (no query, no new attempt, no lease).
    await api.post('/fixture/clock', { form: { minutes: '3' } });
    const run4 = await postReconcile(api);
    expect(run4.status()).toBe(201);
    const lines4 = (await run4.json()).reconcile.reports.filter((l: any) => l.operation_id === operation1ID);
    expect(lines4[0].text).toContain('Автоматические попытки сверки исчерпаны');
    const born = await postRecoveryCase(api, operation1ID);
    expect(born.status()).toBe(201);
    const bornView = (await born.json()).recovery_case;
    const caseID = bornView.recovery_case.recovery_case_id;
    const caseNo = bornView.recovery_case.recovery_case_no;

    // 2. The machine policy read of the wired world: the FIVE kinds in
    // the dictionary order with the declared attributes, the honest
    // state words (the material request_evidence allowed with its
    // Confirmation, the read-only pair allowed, the options kind
    // human_only — no approved variants exist in the configuration,
    // the pause/cancel allowed by the declared LIVE semantics), the
    // human-only class of TWO words listed APART with the class text,
    // NO AG-4 sentence (the reconciliation is wired here). The repeat
    // read answers the SAME view (the derivation idempotence), the
    // free query field is ignored by the router (the empty read never
    // answers 400).
    const policy1 = await getPolicy(api, caseID);
    expect(policy1.status()).toBe(200);
    const policyJSON = await policy1.json();
    const view = policyJSON.recovery_policy;
    expect(view.recovery_case_id).toBe(caseID);
    expect(view.actions.map((a: any) => a.kind)).toEqual(kindOrder);
    const byKind: Record<string, any> = {};
    for (const action of view.actions) byKind[action.kind] = action;
    expect(byKind.request_evidence.state).toBe('allowed');
    expect(byKind.request_evidence.state_text).toContain('Разрешён: запрос доказательств — материальный вид');
    expect(byKind.request_evidence.confirmation_required).toBe(true);
    expect(byKind.show_recorded_conditions.state).toBe('allowed');
    expect(byKind.show_recorded_conditions.state_text).toContain('Разрешён (read-only): записанные условия');
    expect(byKind.show_recorded_conditions.confirmation_required).toBe(false);
    expect(byKind.truthful_status_update.state).toBe('allowed');
    expect(byKind.truthful_status_update.state_text).toContain('Разрешён (read-only): статусные слова');
    expect(byKind.truthful_status_update.state_text).not.toContain('Сверка не подключена');
    expect(byKind.truthful_status_update.confirmation_required).toBe(false);
    expect(byKind.offer_approved_options.state).toBe('human_only');
    expect(byKind.offer_approved_options.state_text).toContain('Утверждённых вариантов решения в конфигурации нет — только человек');
    expect(byKind.offer_approved_options.confirmation_required).toBe(true);
    expect(byKind.prepare_pause_cancel.state).toBe('allowed');
    expect(byKind.prepare_pause_cancel.state_text).toContain('LIVE-семантика CancelCompensationSem объявлена каталогом');
    expect(byKind.prepare_pause_cancel.confirmation_required).toBe(true);
    // AC-2: every kind answer carries the DECLARED attributes.
    for (const action of view.actions) {
      expect(action.scope_text.length).toBeGreaterThan(0);
      expect(action.eligibility_rule_text.length).toBeGreaterThan(0);
      expect(action.required_facts_text.length).toBeGreaterThan(0);
      expect(action.authority_text.length).toBeGreaterThan(0);
    }
    // The human-only class rides SEPARATELY (AC-1) with the class word.
    expect(view.human_only_decisions.map((d: any) => d.kind)).toEqual(['liability_or_fault', 'nonstandard_outcome']);
    expect(view.human_only_class_text).toBe('Признание вины, юридический вывод и нестандартный возврат/компенсация требуют человека.');
    expect(view.human_only_decisions[0].boundary_text).toContain('Только человек — признание вины и юридический вывод');
    expect(view.human_only_decisions[1].boundary_text).toContain('Только человек — нестандартная компенсация, возврат или исключение');
    // The derivation idempotence: the repeated read answers the SAME
    // view byte-stably (the policy is a read-time derivation, no row
    // lands anywhere).
    const policy2 = await getPolicy(api, caseID);
    expect(JSON.stringify(await policy2.json())).toBe(JSON.stringify(policyJSON));
    // The free query field is ignored by the router: the empty read
    // answers 200, never 400.
    const freeQuery = await getPolicy(api, caseID, '?foo=bar');
    expect(freeQuery.status()).toBe(200);

    // 3. The case card: the NEW section «Разрешённые действия» (the
    // kind words with the honest state texts, the material kinds with
    // the separate-confirmation word, the closed attribute details,
    // the human-only block APART with the class text and the boundary
    // words, the derivation note) — while every frozen 3-8 word of the
    // card stays byte-stable (rule 161: the slice EXTENDS the card, it
    // never rewrites it).
    await page.goto(`/app/tenants/${tenantID}/recovery-cases/${caseID}`);
    await expect(page.locator('h1')).toContainText(`Recovery Case №${caseNo}`);
    const card = page.locator('#main');
    await expect(card).toContainText('Состояние: open — Recovery Case открыт (open)');
    await expect(card).toContainText('Триггер: reconciliation_exhausted');
    await expect(card).toContainText(`Операция: ${operation1ID}`);
    await expect(card).toContainText('Определение: notify.manual-only@1.0.0');
    await expect(card).toContainText(`Исходный кейс: ${caseA}`);
    await expect(card).toContainText('Цепочка квитанций: версий 1, последняя v1 — исход unknown');
    await expect(card).toContainText('Попытки: dispatch 1/3, сверка 3/3');
    await expect(card).toContainText('Owning System: erp');
    await expect(card).toContainText('Платёжное состояние unknown');
    await expect(card).toContainText('Локальная проекция — не актуальное внешнее состояние');
    await expect(card).toContainText('Событие №1 — created: Recovery Case создан: триггер reconciliation_exhausted');
    // The policy section itself.
    const section = card.locator('li.action-card', { hasText: 'Разрешённые действия' });
    await expect(section).toContainText('request_evidence — allowed: Разрешён: запрос доказательств');
    await expect(section).toContainText('Требует отдельного явного подтверждения оператора.');
    await expect(section).toContainText('show_recorded_conditions — allowed: Разрешён (read-only): записанные условия');
    await expect(section).toContainText('truthful_status_update — allowed: Разрешён (read-only): статусные слова');
    await expect(section).toContainText('offer_approved_options — human_only: Утверждённых вариантов решения в конфигурации нет — только человек');
    await expect(section).toContainText('prepare_pause_cancel — allowed: Разрешён: LIVE-семантика CancelCompensationSem объявлена каталогом');
    await expect(section).toContainText('Политика — read-time вывод над записанным');
    await expect(section.locator('h4')).toContainText('Только человек — решения вне набора системы');
    await expect(section).toContainText('Признание вины, юридический вывод и нестандартный возврат/компенсация требуют человека.');
    await expect(section).toContainText('liability_or_fault — Только человек — признание вины и юридический вывод');
    await expect(section).toContainText('nonstandard_outcome — Только человек — нестандартная компенсация, возврат или исключение');
    // The declared attributes live in the CLOSED details of every kind
    // (the 3-4 lesson: the summary never floods the card).
    const details = section.locator('details', { hasText: 'Атрибуты вида (scope / eligibility / факты / authority / Confirmation)' });
    await expect(details).toHaveCount(5);
    await expect(details.first()).not.toHaveAttribute('open');
    // The phone material NEVER enters the card (AD-10: pins, not
    // copies).
    const cardText = await card.innerText();
    expect(cardText).not.toContain('+79000000731');

    // 4. The LIVE paused tenant control through the REAL 3-2 surface:
    // the Set lands the point, the material kinds turn blocked with
    // the EXACT control word (the mode and the reason), the read-only
    // kinds stay allowed, the options kind stays human_only — and the
    // machine view still answers 200 (the right limits itself in
    // words, never a refusal). The lift heals the world.
    const pausedReason = 'восстановление приостановлено: инвентаризация операций тенента';
    const setPaused = await api.post(`${apiBase}/controls`, { data: {
      scope_kind: 'tenant', scope_ref: tenantID, mode: 'paused', reason: pausedReason, expected_generation: 0,
    } });
    expect(setPaused.status()).toBeLessThan(300);
    const pausedPolicy = await getPolicy(api, caseID);
    expect(pausedPolicy.status()).toBe(200);
    const pausedView = (await pausedPolicy.json()).recovery_policy;
    const pausedByKind: Record<string, any> = {};
    for (const action of pausedView.actions) pausedByKind[action.kind] = action;
    expect(pausedByKind.request_evidence.state).toBe('blocked');
    expect(pausedByKind.request_evidence.state_text).toContain(`Заблокировано scoped control: paused: ${pausedReason}`);
    expect(pausedByKind.prepare_pause_cancel.state).toBe('blocked');
    expect(pausedByKind.prepare_pause_cancel.state_text).toContain(`Заблокировано scoped control: paused: ${pausedReason}`);
    expect(pausedByKind.show_recorded_conditions.state).toBe('allowed');
    expect(pausedByKind.truthful_status_update.state).toBe('allowed');
    expect(pausedByKind.offer_approved_options.state).toBe('human_only');
    // The paused card speaks the blocked word honestly (the read-only
    // words never move).
    await page.goto(`/app/tenants/${tenantID}/recovery-cases/${caseID}`);
    await expect(page.locator('li.action-card', { hasText: 'Разрешённые действия' })).toContainText(`request_evidence — blocked: Заблокировано scoped control: paused: ${pausedReason}`);
    await expect(page.locator('li.action-card', { hasText: 'Разрешённые действия' })).toContainText('show_recorded_conditions — allowed: Разрешён (read-only): записанные условия');
    // The lift heals the world (a paused tenant would park every later
    // row — the dispatch matrix lift precedent).
    const lift = await api.post(`${apiBase}/controls/resume`, { data: {
      scope_kind: 'tenant', scope_ref: tenantID, reason: 'инвентаризация завершена: контроль снят', expected_generation: 1,
    } });
    expect(lift.status()).toBeLessThan(300);
    const healedPolicy = await getPolicy(api, caseID);
    expect(healedPolicy.status()).toBe(200);
    const healedView = (await healedPolicy.json()).recovery_policy;
    const healedRequest = healedView.actions.find((a: any) => a.kind === 'request_evidence');
    expect(healedRequest.state).toBe('allowed');

    // 5. The AG-4 composition world (the nil reconciliation adapter):
    // the second operation walks the nil-adapter composition — the
    // reconcile command refuses in words, the case is born on the
    // UNAVAILABLE trigger — and the policy of that case rides the
    // honest «Сверка не подключена» sentence on the status kind
    // WITHOUT blocking anything (the composition fact, never a
    // refusal). The frozen 3-8 words of the second card stay put.
    await api.post('/fixture/recovery-ag4', { form: { enabled: '1' } });
    const proposal2 = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a902',
      typed_input: { phone: '+79000000732' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal2.status()).toBe(200);
    const preview2ID = (await proposal2.json()).proposal.preview_id;
    const confirm2 = await api.post(`${apiBase}/cases/${caseA}/action-previews/${preview2ID}/versions/1/confirmations`, { data: { lifetime_seconds: 3600 } });
    expect(confirm2.status()).toBe(201);
    const confirmation2ID = (await confirm2.json()).confirmation.confirmation_id;
    const authorize2 = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmation2ID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-rec9-key-2' } });
    expect(authorize2.status()).toBe(201);
    const operation2ID = (await authorize2.json()).authorization.operation.operation_id;
    await api.post('/fixture/action-dispatch-outcome', { form: { outcome: 'timeout', failure: '1', guarantee: '0' } });
    const unknownDrain2 = await postDrain(api);
    expect(unknownDrain2.status()).toBe(201);
    const ag4Birth = await postRecoveryCase(api, operation2ID);
    expect(ag4Birth.status()).toBe(201);
    const birthView = (await ag4Birth.json()).recovery_case;
    expect(birthView.decision).toBe('created');
    expect(birthView.recovery_case.trigger).toBe('reconciliation_unavailable');
    const case2ID = birthView.recovery_case.recovery_case_id;
    const case2No = birthView.recovery_case.recovery_case_no;
    const ag4Policy = await getPolicy(api, case2ID);
    expect(ag4Policy.status()).toBe(200);
    const ag4View = (await ag4Policy.json()).recovery_policy;
    const ag4ByKind: Record<string, any> = {};
    for (const action of ag4View.actions) ag4ByKind[action.kind] = action;
    expect(ag4ByKind.truthful_status_update.state).toBe('allowed');
    expect(ag4ByKind.truthful_status_update.state_text).toContain('Сверка не подключена (AG-4) — актуальность внешнего состояния не подтверждается');
    expect(ag4ByKind.request_evidence.state).toBe('allowed');
    expect(ag4ByKind.request_evidence.state_text).not.toContain('Сверка не подключена');
    // The AG-4 case card: the frozen unavailable-trigger words of 3-8
    // plus the policy section with the honest AG-4 sentence.
    await page.goto(`/app/tenants/${tenantID}/recovery-cases/${case2ID}`);
    await expect(page.locator('h1')).toContainText(`Recovery Case №${case2No}`);
    await expect(page.locator('#main')).toContainText('Триггер: reconciliation_unavailable');
    await expect(page.locator('#main')).toContainText('Попытки: dispatch 1/3, сверка 0/3');
    await expect(page.locator('li.action-card', { hasText: 'Разрешённые действия' })).toContainText('Сверка не подключена (AG-4)');
    const case2Text = await page.locator('#main').innerText();
    expect(case2Text).not.toContain('+79000000732');
    await api.post('/fixture/recovery-ag4', { form: { enabled: '0' } });

    // 6. The rights rows: the member without the operations scope
    // hears the honest 403 with the scope words (the policy is a
    // card-right read), the cross-tenant surfaces are the plain 404
    // (no disclosure that the tenant recovers actions at all), the
    // unknown case answers the honest 404.
    await api.post('/fixture/operator', { form: { second: 'ops' } });
    const opsPolicy = await getPolicy(api, caseID);
    expect(opsPolicy.status()).toBe(403);
    expect((await opsPolicy.json()).message).toContain('полномочия operations');
    await api.post('/fixture/operator', { form: { second: '' } });
    const foreignPolicy = await api.get(`/v1/tenants/${foreignTenantID}/recovery-cases/${caseID}/policy`);
    expect(foreignPolicy.status()).toBe(404);
    const foreignPanel = await api.get(`/app/tenants/${foreignTenantID}/recovery-cases`);
    expect(foreignPanel.status()).toBe(404);
    const foreignCard = await api.get(`/app/tenants/${foreignTenantID}/recovery-cases/${caseID}`);
    expect(foreignCard.status()).toBe(404);
    const unknownPolicy = await getPolicy(api, '01994f64-6c00-7000-8000-00000000beef');
    expect(unknownPolicy.status()).toBe(404);

    // 7. The a11y close at 320px: the skip-link, the semantic heading
    // ladder, the KEYBOARD toggle of a CLOSED attribute details of the
    // policy section and the single column without the horizontal
    // scroll on every page of the vertical.
    await page.goto(`/app/tenants/${tenantID}/recovery-cases/${caseID}`);
    await expect(page.locator('a.skip-link')).toBeVisible();
    await expect(page.locator('h1')).toContainText(`Recovery Case №${caseNo}`);
    const policyDetails = page.locator('li.action-card', { hasText: 'Разрешённые действия' }).locator('details', { hasText: 'Атрибуты вида (scope / eligibility / факты / authority / Confirmation)' }).first();
    await expect(policyDetails).not.toHaveAttribute('open');
    const summaryBox = await policyDetails.locator('summary').boundingBox();
    expect(summaryBox!.height).toBeGreaterThanOrEqual(44);
    await policyDetails.locator('summary').focus();
    await policyDetails.locator('summary').press('Enter');
    await expect(policyDetails).toHaveAttribute('open', '');
    await expect(policyDetails).toContainText('Scope:');
    await expect(policyDetails).toContainText('Authority:');
    const overflowCase = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflowCase).toBeLessThanOrEqual(0);
    await page.goto(`/app/tenants/${tenantID}/recovery-cases`);
    const overflowPanel = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflowPanel).toBeLessThanOrEqual(0);
    await page.goto(`/app/tenants/${tenantID}/cases/${caseA}/operations`);
    const overflowOps = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflowOps).toBeLessThanOrEqual(0);
  } finally {
    // The fixture reset (the z36 set): the next journey starts at the
    // honest production default — the empty dictionary, the primary
    // owner, the live clock, the step-up off, the default adapter
    // scripts of ALL the contours, the wired recovery world. The
    // paused control point is lifted best-effort (the row heals the
    // world itself before this line on the happy path).
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
    await api.post(`${apiBase}/controls/resume`, { data: {
      scope_kind: 'tenant', scope_ref: tenantID, reason: 'сброс journeys: контроль снят', expected_generation: 0,
    } }).catch(() => {});
  }
});
