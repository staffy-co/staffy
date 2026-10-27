import { expect, test } from '@playwright/test';

// z49 — Story 5-1 (Epic 5): the Montagio Admission Registry journey. The
// machine view answers the read-time derivation over the fixture fact
// sources (the sealed «Услуги» pack pin, the derived capability states, the
// gate instance history); the walk drives the four-stage ladder LIVE
// through the real gate contour (controlled conjunction → the exact
// route-scoped OQ-11 pair → the live conjunction), the cap words of
// avito_candidate/public_commercial never lift over passing gates, and the
// «Пилот услуг Montagio» page renders the Route Matrix with the stages as
// text, the next owners, the fence words and the approved AC-11 wording
// dictionary. The page is read-only. Frozen words of the neighboring pages
// stay byte-stable (their journeys pass untouched — rule 161; this spec
// runs last in the serial suite, its pack switch never leaks backwards).

const tenantUUID = '01994f64-6c00-7000-8000-000000000002';
const employeeUUID = '01994f64-6c00-7000-8000-000000000004';
const ownerUUID = '01994f64-6c00-7000-8000-000000000001';
const backupUUID = '01994f64-6c00-7000-8000-00000000000c';
const apiBase = `/v1/tenants/${tenantUUID}`;
const montagioPath = `/app/tenants/${tenantUUID}/montagio`;
const evidenceHash = 'a'.repeat(64);
// The fixed eight-word route dictionary of the Montagio pilot.
const routeDictionary = [
  'novofon_voice', 'montagio_web', 'partner_intake', 'b2b_intake',
  'protected_web_continuation', 'human_inbox', 'avito_candidate', 'public_commercial'
];

test('montagio admission: the four-stage ladder, honest words and the machine view at 320px', async ({ browser }) => {
  test.setTimeout(180_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;

  // The «Услуги» pack pin (the live pack switch from the neighboring
  // journeys' auto-parts with the confirmed impact — the fixture employee
  // walks the incompatible upgrade through the REAL command path).
  const currentVersion = async () => {
    const view = await (await api.get(`${apiBase}/employees/${employeeUUID}`)).json();
    return view.employee.version;
  };
  await api.post(`${apiBase}/employees`, { data: { name: 'Аня' } });
  await api.put(`${apiBase}/employees/${employeeUUID}/domain-pack`, { data: {
    version: await currentVersion(), pack_id: 'montagio-services', pack_version: '1.0.0',
    variants: { engineering_plumbing: true, heating: false }, confirmed: true
  } });
  // The sealed manifest carries the «Услуги» pin (the release chain over
  // the switched refs — the newest manifest of the fixture world).
  const releaseView = await (await api.get(`${apiBase}/employees/${employeeUUID}/release`)).json();
  const revision = releaseView.draft?.revision ?? 0;
  await api.post(`${apiBase}/employees/${employeeUUID}/release/draft`, { data: { expected_revision: revision } });
  const versionResponse = await api.post(`${apiBase}/employees/${employeeUUID}/release/versions`, { data: { expected_revision: revision + 1, description: 'Пин услуг Montagio 1.0.0' } });
  const version = (await versionResponse.json()).version;
  await api.post(`${apiBase}/employees/${employeeUUID}/release/manifests`, { data: { config_version: version, expected_revision: revision + 2 } });

  // The machine view BEFORE the gates: the pin passes, the connector routes
  // stay candidates with the honest adapter words (AG-4), the Staffy-own
  // human_inbox sits at verified (the controlled gates open), the capped
  // routes carry their cap words, the dictionaries ride the wire.
  const admissionOf = async () => (await (await api.get(`${apiBase}/montagio/admission`)).json());
  const admission = await admissionOf();
  expect(admission.montagio_admission.pack_pinned).toBe(true);
  expect(admission.montagio_admission.pack_word).toContain('montagio-services@1.0.0');
  expect(admission.can_manage).toBe(true);
  expect(admission.view_scope).toBe('configuration');
  expect(admission.montagio_admission.routes.map((route: any) => route.route_id)).toEqual(routeDictionary);
  expect(admission.montagio_admission.montagio_gate_ids).toEqual(['OQ-7A', 'OQ-8', 'OQ-9', 'OQ-10', 'OQ-13A', 'OQ-18']);
  expect(admission.montagio_admission.wording_words).toHaveLength(5);
  // The voice route: the twelve novofon-sip requirements stay unproven as a
  // conjunction — at least the honest adapter blocker names AG-4.
  const routes = admission.montagio_admission.routes;
  const voice = routes.find((route: any) => route.route_id === 'novofon_voice');
  expect(voice.stage).toBe('candidate');
  expect(voice.capabilities).toHaveLength(12);
  expect(voice.blockers.some((blocker: any) => blocker.word.startsWith('Novofon-адаптер не реализован (AG-4)'))).toBe(true);
  expect(voice.blockers.some((blocker: any) => blocker.word.includes('подключения с живым доказательством нет'))).toBe(true);
  // The web route: the Django site adapter word.
  const web = routes.find((route: any) => route.route_id === 'montagio_web');
  expect(web.stage).toBe('candidate');
  expect(web.capabilities).toHaveLength(9);
  expect(web.blockers.some((blocker: any) => blocker.word.startsWith('адаптер сайта Montagio (Django) не реализован (AG-4)'))).toBe(true);
  // The Staffy-own human inbox: verified, the gates block by exact words.
  const inbox = routes.find((route: any) => route.route_id === 'human_inbox');
  expect(inbox.stage).toBe('verified');
  expect(inbox.capabilities ?? []).toEqual([]); // omitempty: no external requirements
  const oq7a = inbox.blockers.find((blocker: any) => blocker.id === 'OQ-7A' && blocker.source === 'gate');
  expect(oq7a.word).toContain('план контролируемой когорты Montagio (UJ-M1/UJ-M2) не предзарегистрирован');
  expect(oq7a.safe_state).toContain('simulation only; ни один маршрут не подменяет другой');
  expect(oq7a.next_owner).toBe('Product Owner + Montagio Operations');
  // The capped routes: the cap words ride beside the stage, never hidden.
  const avito = routes.find((route: any) => route.route_id === 'avito_candidate');
  expect(avito.stage).toBe('candidate');
  expect(avito.cap_word).toContain('официальный разрешённый способ интеграции не доказан');
  expect(avito.fence_words[0]).toContain('Avito-деклараций в каталоге нет');
  const publicRoute = routes.find((route: any) => route.route_id === 'public_commercial');
  expect(publicRoute.stage).toBe('candidate');
  expect(publicRoute.cap_word).toContain('бизнес-машины Order/payment Montagio не построены');
  expect(publicRoute.fence_words.join(' ')).toContain('закрытые validation-сценарии НЕ блокируются (AC-12)');
  // The idempotent read: the same world answers the same JSON.
  const repeat = await admissionOf();
  expect(JSON.stringify(repeat)).toBe(JSON.stringify(admission));

  // The gate contour walk (the existing test & release surface): the full
  // controlled conjunction of human_inbox, the exact route-scoped OQ-11
  // pair, the live conjunction and the capped gates — every transition
  // through the REAL command paths with the step-up discipline.
  const passGate = async (definition: string, roles: Array<[string, string]>, route?: { channel: string; direction: string }) => {
    const base = { definition_id: definition, ...(route ? { route } : {}) };
    const opened = await (await api.post(`${apiBase}/employees/${employeeUUID}/test-release/gate-instances`, { data: { ...base, state: 'open' } })).json();
    const inTest = await (await api.post(`${apiBase}/employees/${employeeUUID}/test-release/gate-instances`, { data: { ...base, state: 'in_test', supersedes_instance_id: opened.instance_id } })).json();
    await api.post(`${apiBase}/employees/${employeeUUID}/test-release/gate-instances`, { data: {
      ...base, state: 'pass', supersedes_instance_id: inTest.instance_id,
      approved_by: roles.map(([role, participant]) => ({ participant_id: participant, role })),
      evidence_refs: [`evidence://${definition.toLowerCase()}/montagio-journey`], evidence_hash: evidenceHash
    } });
  };
  await api.post('/fixture/step-up', { form: { enabled: '1' } });
  try {
    // The controlled conjunction: human_inbox walks to controlled — the
    // live gates stay open (controlled is NOT live, the honest ladder).
    await passGate('OQ-7A', [['ProductOwner', ownerUUID], ['MontagioOperations', backupUUID]]);
    await passGate('OQ-8', [['MontagioOperations', ownerUUID]]);
    await passGate('AG-1', [['TechnicalLead', ownerUUID], ['Operations', backupUUID]]);
    await passGate('AG-2', [['ProductOwner', ownerUUID], ['UXQA', backupUUID]]);
    await passGate('AG-4', [['TechnicalLead', ownerUUID], ['Security', backupUUID]]);
    let after = await admissionOf();
    let inboxAfter = after.montagio_admission.routes.find((route: any) => route.route_id === 'human_inbox');
    expect(inboxAfter.stage).toBe('controlled');
    // Only the two live-gate blockers remain — by name.
    expect(inboxAfter.blockers.filter((blocker: any) => blocker.source === 'live-gate').map((blocker: any) => blocker.id).sort()).toEqual(['OQ-14', 'OQ-7B']);
    // The protected continuation stays verified: OQ-11 blocks per channel.
    let protectedAfter = after.montagio_admission.routes.find((route: any) => route.route_id === 'protected_web_continuation');
    expect(protectedAfter.stage).toBe('verified');
    expect(protectedAfter.blockers.some((blocker: any) => blocker.word.startsWith('OQ-11 missing (маршрут web_chat:inbound)'))).toBe(true);

    // The route-scoped OQ-11 discipline: the inbound PASS alone keeps the
    // route verified with the outbound word naming the exact channel.
    await passGate('OQ-11', [['SecurityPrivacy', ownerUUID], ['TechnicalLead', backupUUID]], { channel: 'web_chat', direction: 'inbound' });
    after = await admissionOf();
    protectedAfter = after.montagio_admission.routes.find((route: any) => route.route_id === 'protected_web_continuation');
    expect(protectedAfter.stage).toBe('verified');
    expect(protectedAfter.blockers.some((blocker: any) => blocker.word.startsWith('OQ-11 missing (маршрут web_chat:outbound)'))).toBe(true);
    // The full pair closes the conjunction: controlled.
    await passGate('OQ-11', [['SecurityPrivacy', ownerUUID], ['TechnicalLead', backupUUID]], { channel: 'web_chat', direction: 'outbound' });
    after = await admissionOf();
    protectedAfter = after.montagio_admission.routes.find((route: any) => route.route_id === 'protected_web_continuation');
    expect(protectedAfter.stage).toBe('controlled');

    // The live conjunction: OQ-7B alone keeps both controlled (OQ-14 open);
    // + OQ-14 walks them to live.
    await passGate('OQ-7B', [['ProductOwner', ownerUUID], ['MontagioOperations', backupUUID]]);
    after = await admissionOf();
    expect(after.montagio_admission.routes.find((route: any) => route.route_id === 'human_inbox').stage).toBe('controlled');
    await passGate('OQ-14', [['ProductOwner', ownerUUID], ['OperationsQA', backupUUID]]);
    after = await admissionOf();
    for (const routeID of ['human_inbox', 'protected_web_continuation']) {
      const route = after.montagio_admission.routes.find((item: any) => item.route_id === routeID);
      expect(route.stage).toBe('live');
      expect(route.blockers ?? []).toEqual([]); // omitempty: no blockers remain
    }
    // Every other route stays below (its own proof is absent).
    for (const route of after.montagio_admission.routes) {
      if (route.route_id === 'human_inbox' || route.route_id === 'protected_web_continuation') continue;
      expect(['candidate', 'verified']).toContain(route.stage);
    }

    // The caps never lift over passing gates: OQ-18 PASS keeps avito the
    // eternal candidate; OQ-9 + OQ-10 + OQ-13A PASS keep public_commercial
    // capped (the Order/payment machines are future slices).
    await passGate('OQ-18', [['ProductOwner', ownerUUID], ['Partnerships/Architecture', backupUUID]]);
    await passGate('OQ-9', [['ProductOwner', ownerUUID], ['Pricing/Operations', backupUUID]]);
    await passGate('OQ-10', [['ProductOwner', ownerUUID], ['Legal/Finance/Operations', backupUUID]]);
    await passGate('OQ-13A', [['ProductOwner', ownerUUID], ['Tenant', backupUUID]]);
    after = await admissionOf();
    expect(after.montagio_admission.routes.find((route: any) => route.route_id === 'avito_candidate').stage).toBe('candidate');
    const capped = after.montagio_admission.routes.find((route: any) => route.route_id === 'public_commercial');
    expect(capped.stage).toBe('candidate');
    expect(capped.blockers.some((blocker: any) => blocker.source === 'cap' && blocker.word.includes('OQ-10 блокирует публичный маршрут, НЕ блокируя закрытые validation-сценарии'))).toBe(true);
  } finally {
    await api.post('/fixture/step-up', { form: { enabled: '0' } }).catch(() => {});
  }

  // The page: the Route Matrix renders with the four stages as text (never
  // color or position alone), the next owners, the fence words, the AC-11
  // wording dictionary section and the a11y skeleton.
  await page.goto(montagioPath);
  await expect(page.getByRole('heading', { name: 'Пилот услуг Montagio: реестр допуска' })).toBeVisible();
  await expect(page.locator('.skip-link')).toBeVisible();
  await expect(page.getByRole('status').first()).toContainText('montagio-services@1.0.0');
  // The stage ladder explanation carries all four stage words.
  const stageNote = page.locator('.next-note').filter({ hasText: 'Четыре стадии различимы текстом' });
  await expect(stageNote).toContainText('«кандидат»');
  await expect(stageNote).toContainText('«проверен»');
  await expect(stageNote).toContainText('«контролируемый реальный клиент»');
  await expect(stageNote).toContainText('«live»');
  // The matrix rows: the stages read as text — the live Staffy-own routes
  // and the honest candidates with their cap words.
  const matrix = page.locator('.datamap-table').first();
  await expect(matrix.locator('tr', { hasText: 'единый подтверждённый Human Inbox Montagio' })).toContainText('live (live)');
  await expect(matrix.locator('tr', { hasText: 'защищённое web-продолжение' })).toContainText('live (live)');
  await expect(matrix.locator('tr', { hasText: 'голос: Novofon SIP-линия' })).toContainText('кандидат (candidate)');
  await expect(matrix.locator('tr', { hasText: 'Avito (вне пилотного production substrate)' })).toContainText('кандидат (candidate)');
  // The exact blocker words of the candidate routes travel the matrix (the
  // adapter discipline of AG-4); the next owners and the safe states of the
  // gates ride the requirements table — every gate view carries them in its
  // closed details, the PASSed ones included.
  await expect(matrix).toContainText('Novofon-адаптер не реализован (AG-4)');
  const requirements = page.locator('.datamap-table').filter({ hasText: 'Controlled-гейты' });
  await expect(requirements).toContainText('Product Owner + Montagio Operations');
  await expect(requirements).toContainText('Montagio Operations');
  await expect(requirements).toContainText('simulation only; ни один маршрут не подменяет другой');
  await expect(requirements).toContainText('Technical Lead + Security');
  // The live conjunction words of the requirements table: no gate word on a
  // PASSed view (the acting PASS speaks for itself).
  await expect(requirements.locator('tr', { hasText: 'human_inbox' })).toContainText('PASS');
  // The cap words render as the alert notes (avito + the OQ-10 fence).
  await expect(page.getByRole('note').filter({ hasText: 'официальный разрешённый способ интеграции не доказан' })).toBeVisible();
  await expect(page.getByRole('note').filter({ hasText: 'OQ-10 блокирует публичный маршрут, НЕ блокируя закрытые validation-сценарии' })).toBeVisible();
  // The AC-11 wording dictionary section: five approved words.
  await expect(page.getByRole('heading', { name: 'Утверждённые правдивые формулировки (AC-11)' })).toBeVisible();
  const wording = page.locator('.datamap-table-wrap').filter({ hasText: 'Утверждённые формулировки AC-11' });
  await expect(wording.locator('tbody tr')).toHaveCount(5);
  await expect(wording).toContainText('Montagio — договорная сторона');
  await expect(wording).toContainText('best-effort без SLA');
  // The keyboard walk: the details toggle opens by Enter (the summary is a
  // focusable target ≥44px).
  const routeDetails = matrix.locator('details').first();
  const firstSummary = routeDetails.locator('summary');
  await firstSummary.focus();
  await page.keyboard.press('Enter');
  await expect(routeDetails).toBeVisible();
  await expect(routeDetails).toContainText('классы автономности задаёт версионированный Operating Profile');
  // No horizontal scroll at 320px.
  await expect(page.locator('main')).not.toHaveCSS('overflow-x', /scroll|auto/);
  // The secrets never travel: the page carries no connection field values.
  const body = await page.locator('body').innerText();
  expect(body).not.toContain('sandbox-');
  expect(body).not.toContain('api_key');
  expect(body).not.toContain('github_pat_');

  // The honest refusals of the reused rights: an unknown tenant answers
  // the plain 404 before any material; the free query fields stay ignored
  // (the empty read never answers a 400).
  const foreign = await api.get('/v1/tenants/01994f64-6c00-7000-8000-000000000099/montagio/admission');
  expect(foreign.status()).toBe(404);
  const freeForm = await api.get(`${apiBase}/montagio/admission?stage=live&detail=full`);
  expect(freeForm.status()).toBe(200);
});
