import { expect, test } from '@playwright/test';

const tenantUUID = '01994f64-6c00-7000-8000-000000000002';
const employeeUUID = '01994f64-6c00-7000-8000-000000000004';
const ownerUUID = '01994f64-6c00-7000-8000-000000000001';
const employeePath = `/app/tenants/${tenantUUID}/employees/${employeeUUID}`;
const testReleasePath = `${employeePath}/test-release`;
const apiBase = `/v1/tenants/${tenantUUID}`;
const evidenceHash = 'a'.repeat(64);

const brief = {
  purpose: 'Помогать клиентам магазина автозапчастей',
  name: 'Аня',
  languages: ['ru'],
  time_zone: 'Europe/Moscow',
  working_hours: { days: ['mon', 'tue'], start: '09:00', end: '18:00' },
  desired_channels: ['phone', 'web_chat'],
  primary_tasks: ['Консультации по деталям']
};

const behavior = {
  role: 'Консультант',
  purpose: 'Общие ответы по автозапчастям',
  ai_disclosure: true,
  tone: 'neutral',
  modes: ['answer'],
  confirmation: 'required',
  handoff: 'required'
};

const voiceConfig = {
  profile_id: 'ru-neutral-1',
  language: 'ru',
  style: 'neutral',
  tempo: 100,
  dictionary: []
};

const profileContent = {
  work_scope: {
    routes: [{ channel: 'phone', direction: 'inbound' }, { channel: 'web_chat', direction: 'inbound' }],
    forbidden_routes: [{ route: { channel: 'telegram', direction: 'outbound' }, reason: 'Канал не подключён и не оплачивается.' }],
    languages: ['ru'],
    window: { days: ['mon', 'tue'], start: '09:00', end: '18:00', time_zone: 'Europe/Moscow' },
    limits: {
      max_concurrent_conversations_tenant: 4,
      max_concurrent_calls_tenant: 2,
      per_route: [
        { route: { channel: 'phone', direction: 'inbound' }, max_concurrent_conversations: 2, max_concurrent_calls: 1 },
        { route: { channel: 'web_chat', direction: 'inbound' }, max_concurrent_conversations: 2 }
      ]
    },
    admission_stop_conditions: ['Достижение hard limit бюджета.', 'Перевод Entitlement в paused.']
  },
  human_coverage: [
    { route: { channel: 'phone', direction: 'inbound' }, response_owner_id: ownerUUID, backup_owner_id: ownerUUID, days: ['mon', 'tue'], start: '09:00', end: '18:00', ack_deadline_minutes: 15, fallback: 'voicemail', fallback_evidence: 'capability://jivo/voicemail/verified' },
    { route: { channel: 'web_chat', direction: 'inbound' }, response_owner_id: ownerUUID, backup_owner_id: ownerUUID, days: ['mon', 'tue'], start: '09:00', end: '18:00', ack_deadline_minutes: 10, fallback: 'human_callback' }
  ],
  privacy_assignments: [
    { route: { channel: 'phone', direction: 'inbound' }, purpose: 'recording', in_use: true, legal_basis: 'Договор с клиентом', consent_ref: 'consent://tenant/recording', disclosure: 'Озвучивается в начале звонка', retention: '90 дней', deletion: 'Workflow Story 1-10', withdrawal_behavior: 'Остановка по запросу клиента', oq11_state: 'pass', oq11_evidence: 'privacy://oq11/recording-pass' },
    { route: { channel: 'phone', direction: 'inbound' }, purpose: 'stt_model_processing', in_use: false, oq11_state: 'open' },
    { route: { channel: 'phone', direction: 'inbound' }, purpose: 'media_processing', in_use: false, oq11_state: 'open' },
    { route: { channel: 'phone', direction: 'inbound' }, purpose: 'transfer', in_use: false, oq11_state: 'open' },
    { route: { channel: 'phone', direction: 'inbound' }, purpose: 'marketing_contact', in_use: false, oq11_state: 'open' },
    { route: { channel: 'web_chat', direction: 'inbound' }, purpose: 'recording', in_use: false, oq11_state: 'open' },
    { route: { channel: 'web_chat', direction: 'inbound' }, purpose: 'stt_model_processing', in_use: false, oq11_state: 'open' },
    { route: { channel: 'web_chat', direction: 'inbound' }, purpose: 'media_processing', in_use: false, oq11_state: 'open' },
    { route: { channel: 'web_chat', direction: 'inbound' }, purpose: 'transfer', in_use: false, oq11_state: 'open' },
    { route: { channel: 'web_chat', direction: 'inbound' }, purpose: 'marketing_contact', in_use: false, oq11_state: 'open' }
  ],
  operational_level: { level: 'best_effort' },
  measurement_dictionary: [{
    id: 'csat', name: 'CSAT после закрытия', business_meaning: 'Удовлетворённость клиента обслуживанием',
    numerator: 'Положительные оценки', denominator: 'Все оценки с ответом', source_of_truth: 'Опрос после закрытия case',
    terminal_timing: 'Через 24 часа после закрытия', slices: ['journey', 'modality', 'conversation_mode', 'release_version', 'autonomy_class'],
    owner_id: ownerUUID, dispute_resolution: 'Разногласия решает владелец метрики',
    route_failure_visibility: 'Доля route-blocking failure показывается отдельной строкой, не средним'
  }],
  gate_declarations: [{
    gate_id: 'knowledge_eval', evidence_owner_id: ownerUUID,
    decision_deadline: 'До старта пилота', fail_closed_fallback: 'block_route', allow_role_coincidence: false
  }],
  coverage_matrix: [
    { journey: 'поиск детали', route: { channel: 'phone', direction: 'inbound' }, modality: 'voice', conversation_mode: 'sales', autonomy_class: 'assisted', eval_ref: 'eval://auto-parts/search/voice-v1', gate_refs: ['knowledge_eval'], human_fallback: 'human_callback по регламенту' },
    { journey: 'поиск детали', route: { channel: 'web_chat', direction: 'inbound' }, modality: 'text', conversation_mode: 'sales', autonomy_class: 'assisted', eval_ref: 'eval://auto-parts/search/text-v1', gate_refs: ['knowledge_eval'], human_fallback: 'human_callback по регламенту' },
    { journey: 'уточнение аналога', route: { channel: 'phone', direction: 'inbound' }, modality: 'voice', conversation_mode: 'sales', autonomy_class: 'assisted', not_admitted: true, not_admitted_reason: 'Голосовой сценарий аналогов требует доработки pack.' },
    { journey: 'уточнение аналога', route: { channel: 'web_chat', direction: 'inbound' }, modality: 'text', conversation_mode: 'sales', autonomy_class: 'assisted', eval_ref: 'eval://auto-parts/analog/text-v1', human_fallback: 'human_callback по регламенту' }
  ],
  conversation_modes: ['sales'],
  autonomy_classes: ['assisted']
};

// ensureCandidate assembles the complete release candidate over the JSON
// API: brief, pack, knowledge, behavior, voice, connection with the acting
// capability, active entitlement, sealed manifest and approved profile. Every
// step re-reads the draft version, so the flow works on any earlier journey
// state (fresh or shared fixture world).
async function ensureCandidate(api) {
  const currentVersion = async () => {
    const view = await (await api.get(`${apiBase}/employees/${employeeUUID}`)).json();
    return view.employee.version;
  };
  // The fixture employee (create reuses it) with the phone+web_chat brief.
  await api.post(`${apiBase}/employees`, { data: { name: 'Аня' } });
  await api.patch(`${apiBase}/employees/${employeeUUID}/brief`, { data: { version: await currentVersion(), brief } });
  // Domain pack: auto-parts (the matrix journeys of the profile).
  await api.put(`${apiBase}/employees/${employeeUUID}/domain-pack`, { data: {
    version: await currentVersion(), pack_id: 'auto-parts', pack_version: '1.0.0',
    variants: { parts_interest: 'both' }, confirmed: true
  } });
  // Knowledge: the synthetic fixture document to the preview state.
  const sources = await (await api.get(`${apiBase}/employees/${employeeUUID}/knowledge-sources`)).json();
  const listing = sources.sources ?? [];
  const previewSource = Array.isArray(listing) && listing.find((source) => source.status === 'preview' && source.source_id === '01994f64-6c00-7000-8000-000000000005');
  if (!previewSource) {
    const ingest = await api.post(`${apiBase}/employees/${employeeUUID}/knowledge-sources/ingest`, { data: {
      type: 'document', title: 'Синтетический документ', owner: 'Тест', provenance: 'Фикстура', scope: 'Тест', priority: 50, language: 'ru', fixture: true
    } });
    const ingested = await ingest.json();
    if (!ingested.source) {
      throw new Error(`knowledge ingest failed: ${JSON.stringify(ingested)}`);
    }
    await api.post(`${apiBase}/employees/${employeeUUID}/knowledge-sources/${ingested.source.source_id}/ingest-fixture`, { data: { version: ingested.source.version, fixture_id: 'sample-v1' } });
  }
  // Behavior and voice: the manifest refs need one saved version of each.
  await api.put(`${apiBase}/employees/${employeeUUID}/behavior`, { data: { version: await currentVersion(), change_summary: 'Поведение кандидата', config: behavior } });
  await api.put(`${apiBase}/employees/${employeeUUID}/voice`, { data: { version: await currentVersion(), change_summary: 'Голос кандидата', config: voiceConfig } });
  // Connection: the synthetic telephony with the verified read-only probe
  // (the acting capability; send-sms stays an honest AG-4 note). The
  // integrations journey revokes its connection at the end, so a fresh one
  // is created whenever no connected one remains.
  const connections = await (await api.get(`${apiBase}/connections`)).json();
  const usable = (connections.connections ?? []).filter((item) => item.status !== 'revoked' && item.status !== 'failed');
  let connection = usable[0];
  if (!connection) {
    const created = await api.post(`${apiBase}/connections`, { data: {
      connector_id: 'staffy-synthetic-telephony', connector_version: '1.1.0', label: 'Телефония кандидата', environment: 'sandbox',
      config: { endpoint: 'https://readiness.sandbox.example/api' }
    } });
    const createdView = await created.json();
    const connectionID = createdView.connection.connection_id;
    await api.post(`${apiBase}/connections/${connectionID}/connect`, { data: { version: createdView.connection.version, secrets: { api_key: 'sandbox-readiness-key' } } });
    await api.post(`${apiBase}/connections/${connectionID}/capabilities/query.trunk-status/verify`, { data: {} });
  } else {
    const connectionID = connection.connection_id;
    const capabilities = await (await api.get(`${apiBase}/connections/${connectionID}/capabilities`)).json();
    const trunk = (capabilities.capabilities ?? []).find((capability) => capability.id === 'query.trunk-status');
    if (!trunk || trunk.state !== 'verified') {
      await api.post(`${apiBase}/connections/${connectionID}/capabilities/query.trunk-status/verify`, { data: {} });
    }
  }
  // Entitlement: the active perimeter (synthetic publishes without it, the
  // closed account stage demands it).
  const entitlement = await (await api.get(`${apiBase}/entitlement`)).json();
  if (entitlement.state !== 'active') {
    await api.post(`${apiBase}/entitlement/terms`, { data: {
      currency: 'RUB', tax_regime: 'УСН', period_start: '2026-10-01T00:00:00Z', period_end: '2026-12-31T23:59:59Z',
      included_employees: ['consultant-01'], included_channels: ['jivo'], included_connectors: ['abcp'],
      launch_price: 49000, subscription_price: 15000, change_note: 'Кандидат публикации'
    } });
    await api.post(`${apiBase}/entitlement/state`, { data: { action: 'activate', basis: 'Договор №77 от 01.10.2026', confirm: true } });
  }
  // The manifest: fresh chain over the current refs.
  const releaseView = await (await api.get(`${apiBase}/employees/${employeeUUID}/release`)).json();
  const revision = releaseView.draft?.revision ?? 0;
  await api.post(`${apiBase}/employees/${employeeUUID}/release/draft`, { data: { expected_revision: revision } });
  const versionResponse = await api.post(`${apiBase}/employees/${employeeUUID}/release/versions`, { data: { expected_revision: revision + 1, description: 'Кандидат публикации' } });
  const version = (await versionResponse.json()).version;
  const sealResponse = await api.post(`${apiBase}/employees/${employeeUUID}/release/manifests`, { data: { config_version: version, expected_revision: revision + 2 } });
  const manifest = await sealResponse.json();
  // The approved profile over exactly this manifest (step-up on for the
  // approval command only).
  await api.post('/fixture/step-up', { form: { enabled: '1' } });
  const profileView = await (await api.get(`${apiBase}/employees/${employeeUUID}/operating-profile`)).json();
  const profileRevision = profileView.draft?.revision ?? 0;
  const draftResponse = await api.post(`${apiBase}/employees/${employeeUUID}/operating-profile/draft`, { data: {
    manifest: { manifest_id: manifest.manifest_id, config_version: manifest.config_version, digest: manifest.digest },
    content: profileContent, expected_revision: profileRevision
  } });
  const draftView = await draftResponse.json();
  if (!draftView.revision) {
    throw new Error(`profile draft failed: ${JSON.stringify(draftView)}`);
  }
  const approvalResponse = await api.post(`${apiBase}/employees/${employeeUUID}/operating-profile/approval`, { data: {
    description: 'Профиль кандидата публикации', evidence_refs: ['manifest-digest://candidate', 'threshold://v1'], expected_revision: draftView.revision
  } });
  const approved = await approvalResponse.json();
  if (!approved.version) {
    throw new Error(`profile approval failed: ${JSON.stringify(approved)}`);
  }
  await api.post('/fixture/step-up', { form: { enabled: '0' } });
  return manifest;
}

test('test & release: readiness checklist, controlled publication ladder and honest blocks at 320px', async ({ browser }) => {
  test.setTimeout(120_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  // The fixture step-up flag is global server state: the finally block
  // guarantees the reset even when the journey fails midway.
  let stepUpEnabled = false;
  try {
    const api = page.request;
    await ensureCandidate(api);

    // The panel: the navigation link of the release page leads to it, then
    // the dry-run preview and the honest checklist.
    await page.goto(`${employeePath}/release`);
    const panelLink = page.getByRole('navigation', { name: 'Основная навигация' }).getByRole('link', { name: 'Test & Release' });
    await expect(panelLink).toBeVisible();
    await panelLink.click();
    await expect(page).toHaveURL(new RegExp('/test-release$'));
    await expect(page.getByRole('heading', { name: 'Test & Release Panel' })).toBeVisible();
    // The default stage is the first available step: synthetic.
    await expect(page.getByRole('status').first()).toContainText('synthetic');
    await expect(page.getByRole('status').filter({ hasText: 'Общий статус' })).toContainText('ready');
    // The checklist: every mandatory element with its state and source.
    const elements = page.locator('.datamap-table').first();
    await expect(elements).toBeVisible();
    for (const row of [
      ['Утверждённый Operating Profile', 'готов'],
      ['Release Manifest', 'готов'],
      ['Connections', 'готов'],
      ['Capabilities', 'готов'],
      ['Entitlement', 'готов'],
      ['Gates стадии', 'неприменим'],
      ['Eval', 'связано, не исполнено']
    ]) {
      await expect(elements.locator('tr', { hasText: row[0] }).first()).toContainText(row[1], { ignoreCase: true });
    }
    // The capabilities element carries the honest note about the
    // AG-4-guarded leftovers.
    await expect(elements.locator('tr', { hasText: 'Capabilities' }).first()).toContainText('AG-4');
    // The routes: both admitted, no blocks.
    const routes = page.locator('.datamap-table').nth(1);
    await expect(routes.locator('tr', { hasText: 'phone' })).toContainText('admitted');
    await expect(routes.locator('tr', { hasText: 'web_chat' })).toContainText('admitted');
    // No horizontal scroll at 320 px.
    await expect(page.locator('main')).not.toHaveCSS('overflow-x', /scroll|auto/);

    // The publication without step-up: the honest refusal (aria-live alert).
    await page.getByRole('button', { name: /Подтвердить публикацию стадии/ }).click();
    const dialog = page.locator('#publish-confirm');
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: 'Записать решение о допуске' }).click();
    await expect(page.locator('.alert')).toContainText('step-up подтверждения');

    // Step-up on: the atomic publication of synthetic.
    await page.request.post('/fixture/step-up', { form: { enabled: '1' } });
    stepUpEnabled = true;
    await page.getByRole('button', { name: /Подтвердить публикацию стадии/ }).click();
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: 'Записать решение о допуске' }).click();
    await expect(page.getByRole('status').filter({ hasText: /Публикация записана|Общий статус/ }).first()).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Действующее решение (активный указатель)' })).toBeVisible();
    await expect(page.getByRole('term').first()).toContainText('Решение №1');
    const digestText = (await page.locator('dl .digest-block').last().textContent()) ?? '';
    expect(digestText.trim()).toMatch(/^[0-9a-f]{64}$/);

    // The repeat of the same stage is an honest conflict.
    await page.getByRole('radio', { name: /internal · closed_account_test/ }).check();
    await page.getByRole('button', { name: 'Предпросмотреть стадию' }).click();
    await expect(page.getByRole('status').filter({ hasText: 'Общий статус' })).toContainText('ready');
    await page.getByRole('button', { name: /Подтвердить публикацию стадии/ }).click();
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: 'Записать решение о допуске' }).click();
    await expect(page.getByRole('heading', { name: 'Действующее решение (активный указатель)' })).toBeVisible();
    await expect(page.getByRole('term').first()).toContainText('Решение №2');

    // The higher stages stay honestly blocked: gates without PASS and eval
    // linked, not executed.
    await page.getByRole('radio', { name: /controlled_real_customer/ }).check();
    await page.getByRole('button', { name: 'Предпросмотреть стадию' }).click();
    await expect(page.getByRole('status').filter({ hasText: 'Общий статус' })).toContainText('blocked');
    const blockedRoutes = page.locator('.datamap-table').nth(1);
    await expect(blockedRoutes.locator('tr', { hasText: 'phone' })).toContainText('blocked');
    await expect(blockedRoutes.locator('.form-error').first()).toContainText('gate:OQ-11');
    await expect(blockedRoutes.locator('.form-error').first()).toContainText('eval');
    // The only primary action is unblocking, not a stage switch.
    await expect(page.getByRole('button', { name: /Подтвердить публикацию стадии/ })).toHaveCount(0);
    await expect(page.locator('p[role="status"]').filter({ hasText: 'Публикация недоступна' })).toBeVisible();

    // The gate PASS discipline: OQ-12 allows the single approver
    // (allow_role_coincidence) — the fixture world has one member. The form
    // resets on every redirect, so each transition re-fills every field.
    const gateForm = {
      definition: () => page.getByLabel('Определение (словарь платформы: OQ-11, OQ-12, OQ-13B, OQ-14, OQ-7B, AG-1, AG-2, AG-4)'),
      state: () => page.getByLabel(/Целевое состояние/),
      supersedes: () => page.getByLabel(/supersedes_instance_id/)
    };
    const recordGate = async (state, supersedes, extra) => {
      await gateForm.definition().fill('OQ-12');
      await gateForm.state().fill(state);
      await gateForm.supersedes().fill(supersedes);
      if (extra) {
        await page.getByLabel(/approved_by для PASS/).fill(extra.approvedBy);
        await page.getByLabel(/Evidence-ссылки для PASS/).fill(extra.evidence);
        await page.getByLabel(/Hash доказательства для PASS/).fill(extra.hash);
      }
      await page.getByRole('button', { name: 'Записать переход инстанса' }).click();
      await expect(page.getByRole('status').filter({ hasText: 'Переход gate-инстанса записан' })).toBeVisible();
    };
    // The chain: open → in_test → pass with the full PASS payload.
    await recordGate('open', '');
    await recordGate('in_test', '01994f64-6c00-7000-8000-000000000201');
    await recordGate('pass', '01994f64-6c00-7000-8000-000000000202', {
      approvedBy: `${ownerUUID} VoiceTechnicalLead`,
      evidence: 'evidence://oq12/voice-baseline',
      hash: evidenceHash
    });
    // The instance table: the chain with the PASS and its evidence.
    const instanceTable = page.locator('.datamap-table').filter({ hasText: 'scope точный' });
    await expect(instanceTable).toBeVisible();
    await expect(instanceTable.locator('tr', { hasText: '000000000203' })).toContainText('PASS');
    await expect(instanceTable.locator('tr', { hasText: '000000000203' })).toContainText('VoiceTechnicalLead');
    // The superseded open record is honest history, never a permission.
    await expect(instanceTable.locator('tr', { hasText: '000000000201' }).first()).toContainText('начало цепочки');
  } finally {
    // Reset the fixture step-up flag for the other journeys.
    if (stepUpEnabled) {
      await page.request.post('/fixture/step-up', { form: { enabled: '0' } });
    }
  }
});
