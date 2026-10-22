import { expect, test } from '@playwright/test';

// z38 — Story 4-1 (Epic 4): the Pilot Admission Registry journey. The
// machine view answers the read-time derivation over the fixture fact
// sources (the sealed pack pin, the derived capability states, the gate
// instance history); the pilot page renders the route matrix with the
// stages as text, the next owners, the order fence words and the ABCP→1C
// matrix in closed details. The page is read-only: no button of the pilot
// surface writes anything (the honest words of AG-4 carry the future
// slices). Frozen words of the neighboring pages stay byte-stable (their
// journeys pass untouched — rule 161).

const tenantUUID = '01994f64-6c00-7000-8000-000000000002';
const employeeUUID = '01994f64-6c00-7000-8000-000000000004';
const ownerUUID = '01994f64-6c00-7000-8000-000000000001';
const apiBase = `/v1/tenants/${tenantUUID}`;
const pilotPath = `/app/tenants/${tenantUUID}/pilot`;
const evidenceHash = 'a'.repeat(64);

test('pilot admission: the read-time route matrix, honest words and the machine view at 320px', async ({ browser }) => {
  test.setTimeout(120_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;

  // The candidate over the JSON API: the 2.0.0 pack pin (the pilot
  // minimum — the journeys of the profile pin 1.0.0, so the pin walks the
  // incompatible upgrade with the confirmed impact).
  const currentVersion = async () => {
    const view = await (await api.get(`${apiBase}/employees/${employeeUUID}`)).json();
    return view.employee.version;
  };
  await api.post(`${apiBase}/employees`, { data: { name: 'Аня' } });
  await api.put(`${apiBase}/employees/${employeeUUID}/domain-pack`, { data: {
    version: await currentVersion(), pack_id: 'auto-parts', pack_version: '2.0.0',
    variants: { parts_interest: 'both', matching_depth: 1 }, confirmed: true
  } });
  // The synthetic telephony connection of the world with ALL SIX SIP
  // queries verified live (trunk-status rides the integrations journey;
  // the five new queries ride the pilot).
  // The 1.2.0 contract connection: the five new SIP queries exist only
  // there (the neighboring journeys may leave an older 1.1.0 connection —
  // its contract declares none of them).
  const connections = await (await api.get(`${apiBase}/connections`)).json();
  const usable = (connections.connections ?? []).filter((item) => item.status !== 'revoked' && item.status !== 'failed' && item.connector_version === '1.2.0');
  let connectionID = usable[0]?.connection_id;
  if (!connectionID) {
    const created = await api.post(`${apiBase}/connections`, { data: {
      connector_id: 'staffy-synthetic-telephony', connector_version: '1.2.0', label: 'Телефония пилота', environment: 'sandbox',
      config: { endpoint: 'https://pilot.sandbox.example/api' }
    } });
    const createdView = await created.json();
    connectionID = createdView.connection.connection_id;
    await api.post(`${apiBase}/connections/${connectionID}/connect`, { data: { version: createdView.connection.version, secrets: { api_key: 'sandbox-pilot-key' } } });
  }
  for (const capability of ['query.trunk-status', 'query.transfer-bridge', 'query.callback-state', 'query.dtmf-support', 'query.playback-delivery', 'query.failure-states']) {
    const capabilities = await (await api.get(`${apiBase}/connections/${connectionID}/capabilities`)).json();
    const current = (capabilities.capabilities ?? []).find((item) => item.id === capability);
    if (!current || current.state !== 'verified') {
      await api.post(`${apiBase}/connections/${connectionID}/capabilities/${capability}/verify`, { data: {}, headers: { 'Idempotency-Key': `pilot-verify-${capability}` } });
    }
  }
  // The sealed manifest carries the 2.0.0 pin (the release chain over the
  // upgraded refs).
  const releaseView = await (await api.get(`${apiBase}/employees/${employeeUUID}/release`)).json();
  const revision = releaseView.draft?.revision ?? 0;
  await api.post(`${apiBase}/employees/${employeeUUID}/release/draft`, { data: { expected_revision: revision } });
  const versionResponse = await api.post(`${apiBase}/employees/${employeeUUID}/release/versions`, { data: { expected_revision: revision + 1, description: 'Пин пилота 2.0.0' } });
  const version = (await versionResponse.json()).version;
  await api.post(`${apiBase}/employees/${employeeUUID}/release/manifests`, { data: { config_version: version, expected_revision: revision + 2 } });

  // The machine view: the read-time derivation answers the fixed route
  // dictionary with the honest stages.
  const admissionResponse = await api.get(`${apiBase}/pilot/admission`);
  expect(admissionResponse.status()).toBe(200);
  const admission = await admissionResponse.json();
  expect(admission.pilot_admission.pack_pinned).toBe(true);
  expect(admission.pilot_admission.pack_word).toContain('auto-parts@2.0.0');
  expect(admission.can_manage).toBe(true);
  expect(admission.view_scope).toBe('configuration');
  const routes = admission.pilot_admission.routes;
  expect(routes.map((route) => route.route_id)).toEqual([
    'text_jivo', 'voice_sip', 'parts_catalog_abcp', 'parts_order_abcp', 'abcp_1c_sync', 'human_fallback'
  ]);
  // The voice route: all six SIP queries proven → verified (the gates stay
  // open — the honest words below).
  const voice = routes.find((route) => route.route_id === 'voice_sip');
  expect(voice.stage).toBe('verified');
  const oq1 = voice.blockers.find((blocker) => blocker.id === 'OQ-1');
  expect(oq1.word).toContain('провайдер не подтверждён фактическими данными аккаунта');
  expect(oq1.safe_state).toContain('awaiting_verification/limited');
  expect(oq1.next_owner).toBe('Олег + Technical Lead');
  // The text route: the honest adapter word (AG-4).
  const text = routes.find((route) => route.route_id === 'text_jivo');
  expect(text.stage).toBe('candidate');
  expect(text.blockers.some((blocker) => blocker.word.startsWith('Jivo-адаптер не реализован (AG-4)'))).toBe(true);
  // The order route: the fence word travels the machine view.
  const order = routes.find((route) => route.route_id === 'parts_order_abcp');
  expect(order.fence_words[0]).toContain('автоматическая отправка поставщику не выполняется');
  // The matrix: eight fields, every one unknown (AC-10).
  expect(admission.pilot_admission.matrix.map((field) => field.field)).toEqual([
    'client_order_state', 'client_order_id', 'external_1c_id', 'state_version_time',
    'staffy_projection', 'conflict_rule', 'reconcile_query', 'terminal_evidence'
  ]);
  for (const field of admission.pilot_admission.matrix) {
    expect(field.state_word).toContain('терминальное доказательство отсутствует');
  }
  // The idempotent read: the same world answers the same JSON.
  const repeatResponse = await api.get(`${apiBase}/pilot/admission`);
  const repeat = await repeatResponse.json();
  expect(JSON.stringify(repeat)).toBe(JSON.stringify(admission));

  // The gate contour walk (the existing test & release surface): AG-2 and
  // AG-4 PASS close the human fallback route — the only admitted path.
  await api.post('/fixture/step-up', { form: { enabled: '1' } });
  try {
    for (const gate of [
      { definition: 'AG-2', roles: [['ProductOwner', ownerUUID], ['UXQA', '01994f64-6c00-7000-8000-00000000000c']] },
      { definition: 'AG-4', roles: [['TechnicalLead', ownerUUID], ['Security', '01994f64-6c00-7000-8000-00000000000c']] }
    ]) {
      const opened = await (await api.post(`${apiBase}/employees/${employeeUUID}/test-release/gate-instances`, { data: {
        definition_id: gate.definition, state: 'open'
      } })).json();
      const inTest = await (await api.post(`${apiBase}/employees/${employeeUUID}/test-release/gate-instances`, { data: {
        definition_id: gate.definition, state: 'in_test', supersedes_instance_id: opened.instance_id
      } })).json();
      await api.post(`${apiBase}/employees/${employeeUUID}/test-release/gate-instances`, { data: {
        definition_id: gate.definition, state: 'pass', supersedes_instance_id: inTest.instance_id,
        approved_by: gate.roles.map(([role, participant]) => ({ participant_id: participant, role })),
        evidence_refs: [`evidence://${gate.definition.toLowerCase()}/pilot-journey`], evidence_hash: evidenceHash
      } });
    }
  } finally {
    await api.post('/fixture/step-up', { form: { enabled: '0' } });
  }
  const afterGates = await (await api.get(`${apiBase}/pilot/admission`)).json();
  const fallback = afterGates.pilot_admission.routes.find((route) => route.route_id === 'human_fallback');
  expect(fallback.stage).toBe('admitted');
  expect(fallback.blockers ?? []).toEqual([]); // omitempty: an empty blocker list stays absent

  // The page: the route matrix renders with the stages as text (never
  // color or position alone), the next owners, the fence words and the
  // matrix details.
  await page.goto(pilotPath);
  await expect(page.getByRole('heading', { name: 'Пилот автозапчастей: реестр допуска' })).toBeVisible();
  // The skip link of the a11y skeleton.
  await expect(page.locator('.skip-link')).toBeVisible();
  // The pack word of the live derivation.
  await expect(page.getByRole('status').first()).toContainText('auto-parts@2.0.0');
  // The stages read as text: the admitted fallback and the verified voice.
  const matrix = page.locator('.datamap-table').first();
  await expect(matrix.locator('tr', { hasText: 'минимальная операторская поверхность' })).toContainText('допущен');
  await expect(matrix.locator('tr', { hasText: 'голос: SIP-линия' })).toContainText('проверен');
  await expect(matrix.locator('tr', { hasText: 'текст: Jivo Human Inbox' })).toContainText('кандидат');
  // The exact blocker words with the next owners and the safe states.
  await expect(matrix).toContainText('провайдер не подтверждён фактическими данными аккаунта');
  await expect(matrix).toContainText('Олег + Technical Lead');
  await expect(matrix).toContainText('нейтральный pending_manager_verification');
  // The order fence word renders as the alert note (AC-9).
  await expect(page.getByRole('note').first()).toContainText('автоматическая отправка поставщику не выполняется');
  // The matrix rows: closed details with the unknown state word (AC-10).
  const matrixDetails = page.locator('details');
  await expect(matrixDetails).toHaveCount(8);
  await expect(matrixDetails.first()).toContainText('unknown — терминальное доказательство отсутствует');
  // The keyboard walk: the details toggle opens by Enter (the summary is a
  // focusable target ≥44px).
  const firstSummary = matrixDetails.first().locator('summary');
  await firstSummary.focus();
  await page.keyboard.press('Enter');
  await expect(matrixDetails.first()).toBeVisible();
  await expect(matrixDetails.first().getByText('Состояние клиентского заказа')).toBeVisible();
  // No horizontal scroll at 320px.
  await expect(page.locator('main')).not.toHaveCSS('overflow-x', /scroll|auto/);
  // The secrets never travel: the page carries no connection field values.
  const body = await page.locator('body').textContent();
  expect(body).not.toContain('sandbox-');
  expect(body).not.toContain('api_key');

  // The honest refusals of the reused rights: an unknown tenant answers
  // the plain 404 before any material.
  const foreign = await api.get('/v1/tenants/01994f64-6c00-7000-8000-000000000099/pilot/admission');
  expect(foreign.status()).toBe(404);
  // The free query fields stay ignored (the empty read never answers 400).
  const freeForm = await api.get(`${apiBase}/pilot/admission?stage=live&detail=full`);
  expect(freeForm.status()).toBe(200);
});
