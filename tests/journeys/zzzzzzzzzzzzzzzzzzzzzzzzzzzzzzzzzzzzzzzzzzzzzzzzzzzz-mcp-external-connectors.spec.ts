import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const foreignTenantID = '01994f64-6c00-7000-8000-00000000a212';
const integrationsPath = `/app/tenants/${tenantID}/integrations`;
const dictionaryPath = `/app/tenants/${tenantID}/tools/external`;
const dictionaryAPI = `/v1/tenants/${tenantID}/tools/external`;
const journeyKey = 'sandbox-external-journey';
const rotatedKey = 'sandbox-rotated-journey';

// z52 — Story 7-4 (Epic 7): the external MCP connectors journey. The
// fixture world carries the scripted external MCP server of the pinned
// revision 2026-07-28 (/fixture/external-mcp — the same listener) and the
// REAL MAC mirror of the seal machine: the connection is created and
// connected with the sandbox key (the format BasicCheck of the router
// branch), the probes run AUTHENTICATED (the verify forms carry the probe
// key — the MAC check happens BEFORE the network round: the missing and
// the mismatching keys are the honest refusals, the foreign revision and
// the oversized inventory are the honest probe failures), the dictionary
// derives the tenant-scoped entries with the provenance (one admitted
// read-only tool, three honest rejections: the fractional typing, the
// missing read-only statement and the mutating tool), the rotation makes
// the entries DISAPPEAR (the stale anchor — the mirror of the 7-3
// dynamics), the repeated probe returns them, and the ladder speaks the
// scope words BEFORE any dictionary material.

test('mcp-external-connectors: the class connection, the authenticated probes, the dictionary and the honest dynamics', async ({ browser }) => {
  test.setTimeout(150_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    await page.goto(integrationsPath);
    const fixtureEndpoint = new URL(page.url()).origin + '/fixture/external-mcp';

    // 1. The class card exists in the business-systems group and opens
    // the create form: the endpoint, the server label and the secret of
    // the sandbox format.
    const card = page.locator('.connector-card', { hasText: 'Staffy External MCP Server' });
    await expect(card).toContainText('external-mcp');
    await expect(card).toContainText('sandbox, test');
    await card.locator('summary').first().click();
    await card.getByLabel('Назначение подключения').fill('Внешний сервер запчастей');
    await card.getByLabel('Endpoint (обязательно)').fill(fixtureEndpoint);
    await card.getByLabel('Метка сервера').fill('Демо-сервер');
    await card.getByRole('button', { name: 'Создать черновик подключения' }).click();

    // The draft connection appears with the pending states.
    const connection = page.locator('.connection-list .employee-card', { hasText: 'Внешний сервер запчастей' });
    await expect(connection).toContainText('draft');
    await expect(connection).toContainText('Черновик: секрет ещё не проверялся');

    // 2. The sandbox key connects the secret: the format BasicCheck of
    // the router branch (no network round on the connect path).
    await connection.locator('summary', { hasText: 'Подключить секрет' }).click();
    await connection.getByLabel('API-ключ (sandbox)').fill(journeyKey);
    await connection.getByRole('button', { name: 'Проверить и подключить' }).click();
    const connected = page.locator('.connection-list .employee-card', { hasText: 'Внешний сервер запчастей' });
    await expect(connected).toContainText('connected_unverified');
    await expect(connected).toContainText('Аутентификация: verified');
    await expect(connected).toContainText('Транспорт: verified');
    await expect(page.locator('.connection-list')).not.toContainText(journeyKey);

    // The connection id of the machine legs (the list API answer).
    const connectionsList = await (await api.get(`/v1/tenants/${tenantID}/connections`)).json();
    const external = connectionsList.connections.find((c: any) => c.connector_id === 'staffy-external-mcp-server');
    expect(external).toBeDefined();
    const connectionID = external.connection_id;
    const verifyPath = (capability: string) =>
      `/v1/tenants/${tenantID}/connections/${connectionID}/capabilities/${capability}/verify`;

    // 3. The unauthenticated probe is impossible fail-closed: the verify
    // command without the key answers the honest rule words BEFORE any
    // network round; the mismatching key answers the MAC words.
    const missing = await api.post(verifyPath('query.server-discover'), {
      headers: { 'Idempotency-Key': 'z52-verify-missing' },
    });
    expect(missing.status()).toBe(400);
    expect(await missing.text()).toContain('требует секрет подключения');

    const wrong = await api.post(verifyPath('query.server-discover'), {
      data: { secrets: { api_key: 'sandbox-wrong-key' } },
      headers: { 'Idempotency-Key': 'z52-verify-wrong', 'Content-Type': 'application/json' },
    });
    expect(wrong.status()).toBe(400);
    expect(await wrong.text()).toContain('не совпадает с принятым подключением');

    // 4. The live probes through the REAL forms: the discover and the
    // tools-list rounds with the MAC-checked key (the form carries the
    // probe key of the class — the conditional field of the Story 7-4
    // transport).
    const capabilitiesPath = `/v1/tenants/${tenantID}/connections/${connectionID}/capabilities`;
    const capabilityOf = async (id: string) => {
      const state = await (await api.get(capabilitiesPath)).json();
      return state.capabilities.find((c: any) => c.id === id);
    };

    const discoverForm = connected.locator('form', { hasText: 'Проверить «Обнаружение сервера»' });
    await discoverForm.getByLabel('API-ключ (sandbox) для пробы').fill(journeyKey);
    await discoverForm.getByRole('button', { name: 'Проверить «Обнаружение сервера»' }).click();
    // The form submission reloads the page: the state assertion waits
    // through the navigation (no URL racing of repeated redirects).
    const discovered = page.locator('.connection-list .employee-card', { hasText: 'Внешний сервер запчастей' });
    await expect(discovered.locator('.capability[data-capability="query.server-discover"]')).toContainText('проверено', { timeout: 20000 });
    // The probe material travels in the evidence row (the progressive
    // disclosure of the page): the machine read carries the server
    // identity of the live round.
    const discoverCapability = await capabilityOf('query.server-discover');
    expect(discoverCapability.state).toBe('verified');
    expect(discoverCapability.evidence.redacted_result).toContain('fixture-external-mcp');
    expect(discoverCapability.evidence.redacted_result).toContain('2026-07-28');

    const listForm = discovered.locator('form', { hasText: 'Проверить «Словарь инструментов сервера»' });
    await listForm.getByLabel('API-ключ (sandbox) для пробы').fill(journeyKey);
    await listForm.getByRole('button', { name: 'Проверить «Словарь инструментов сервера»' }).click();
    const inventoried = page.locator('.connection-list .employee-card', { hasText: 'Внешний сервер запчастей' });
    await expect(inventoried.locator('.capability[data-capability="query.tools-list"]')).toContainText('проверено', { timeout: 20000 });
    const listCapability = await capabilityOf('query.tools-list');
    expect(listCapability.state).toBe('verified');
    expect(listCapability.evidence.redacted_result).toContain('read-tools: 4');
    expect(listCapability.evidence.redacted_result).toContain('Инвентарь: {');

    // 5. The honest refusal legs of the probe matrix: the foreign revision
    // and the oversized inventory fail the probe by name (the toggles of
    // the scripted world — the reconcile-verdict precedent).
    await api.post('/fixture/external-mcp-foreign-revision', { form: { enabled: '1' } });
    const foreignProbe = await api.post(verifyPath('query.server-discover'), {
      data: { secrets: { api_key: journeyKey } },
      headers: { 'Idempotency-Key': 'z52-verify-foreign', 'Content-Type': 'application/json' },
    });
    expect(foreignProbe.status()).toBe(200);
    const foreignReport = (await foreignProbe.json()).capability;
    expect(foreignReport.state).toBe('failed');
    // The named honest refusal travels in the evidence material of the
    // failed probe (the state summary stays generic — the 1-8 domain
    // precedent); the dictionary honestly stays empty of the section.
    expect(foreignReport.evidence.redacted_result).toContain('чужая ревизия');
    await api.post('/fixture/external-mcp-foreign-revision', { form: { enabled: '' } });

    await api.post('/fixture/external-mcp-oversized', { form: { enabled: '1' } });
    const oversizedProbe = await api.post(verifyPath('query.tools-list'), {
      data: { secrets: { api_key: journeyKey } },
      headers: { 'Idempotency-Key': 'z52-verify-oversized', 'Content-Type': 'application/json' },
    });
    expect(oversizedProbe.status()).toBe(200);
    const oversizedReport = (await oversizedProbe.json()).capability;
    expect(oversizedReport.state).toBe('failed');
    // The budget refusal is the machine's own named error: the whole
    // answer is refused without truncation.
    expect(oversizedReport.evidence.redacted_result).toContain('размерный бюджет чтения ответа превышен');
    await api.post('/fixture/external-mcp-oversized', { form: { enabled: '' } });

    // The re-probe after the honest failures returns the verified fact.
    const recovered = await api.post(verifyPath('query.tools-list'), {
      data: { secrets: { api_key: journeyKey } },
      headers: { 'Idempotency-Key': 'z52-verify-recovered', 'Content-Type': 'application/json' },
    });
    expect((await recovered.json()).capability.state).toBe('verified');

    // 6. The dictionary page: the section of the connection with the
    // provenance of the entries and the honest rejections (the fractional
    // typing, the missing read-only statement and the mutating tool).
    await page.goto(dictionaryPath);
    await expect(page.locator('h1')).toHaveText('Словарь внешних инструментов');
    await expect(page.locator('.action-card', { hasText: 'Поиск детали' })).toContainText('допущен');
    await expect(page.locator('.action-card', { hasText: 'Поиск детали' })).toContainText('fixture-external-mcp');
    await expect(page.locator('.action-card', { hasText: 'Поиск детали' })).toContainText('staffy-external-mcp-server@1.0.0');
    await expect(page.locator('.action-card', { hasText: 'Дробная метрика' })).toContainText('отклонён');
    await expect(page.locator('.action-card', { hasText: 'Дробная метрика' })).toContainText('вне платформенного словаря типизации');
    await expect(page.locator('.action-card', { hasText: 'Без заявления read-only' })).toContainText('отклонён');
    await expect(page.locator('.action-card', { hasText: 'Без заявления read-only' })).toContainText('AR-30');
    await expect(page.locator('.action-card', { hasText: 'Отправка внешней команды' })).toContainText('отклонён');
    await expect(page.locator('.action-card', { hasText: 'Отправка внешней команды' })).toContainText('AR-30');
    // The page is strictly read-only: no forms at all.
    await expect(page.locator('form')).toHaveCount(0);

    // 7. The machine dictionary: the closed section form with the
    // admission states, the provenance and the typing of the admitted row.
    const dictionary = await (await api.get(dictionaryAPI)).json();
    expect(dictionary.sections.length).toBe(1);
    const section = dictionary.sections[0];
    expect(section.connector_id).toBe('staffy-external-mcp-server');
    expect(section.connector_version).toBe('1.0.0');
    expect(section.has_entries).toBe(true);
    expect(section.entries.length).toBe(4);
    const admitted = section.entries.find((e: any) => e.name === 'parts-lookup');
    expect(admitted.state).toBe('admitted');
    expect(admitted.provenance.server_name).toBe('fixture-external-mcp');
    expect(admitted.provenance.server_version).toBe('1.2.0');
    expect(admitted.provenance.connector_id).toBe('staffy-external-mcp-server');
    expect(admitted.input_schema.length).toBeGreaterThan(0);
    expect(admitted.output_schema.length).toBeGreaterThan(0);
    for (const name of ['parts-ratio', 'parts-unstated', 'parts-send']) {
      const rejected = section.entries.find((e: any) => e.name === name);
      expect(rejected.state).toBe('rejected');
      expect(rejected.reason.length).toBeGreaterThan(0);
      expect(rejected.input_schema.length).toBe(0);
    }

    // 8. The rotation makes the entries DISAPPEAR (the stale anchor — the
    // mirror of the 7-3 dynamics); the repeated probe with the new key
    // returns them.
    await page.goto(integrationsPath);
    const rotationCard = page.locator('.connection-list .employee-card', { hasText: 'Внешний сервер запчастей' });
    await rotationCard.locator('summary', { hasText: 'Ротация секрета' }).click();
    await rotationCard.locator('form', { hasText: 'Заменить секрет' }).locator('input[type="password"]').fill(rotatedKey);
    await rotationCard.getByRole('button', { name: 'Заменить секрет' }).click();
    await expect(page.locator('.connection-list .employee-card', { hasText: 'Внешний сервер запчастей' })).toContainText('connected_unverified');

    // The dictionary honestly empties: the stale words of the anchor.
    const emptied = await (await api.get(dictionaryAPI)).json();
    expect(emptied.sections.length).toBe(1);
    expect(emptied.sections[0].has_entries).toBe(false);
    expect(emptied.sections[0].entries.length).toBe(0);
    expect(emptied.sections[0].reason).toContain('ротирован');

    // The old key no longer matches the new seal.
    const oldKey = await api.post(verifyPath('query.tools-list'), {
      data: { secrets: { api_key: journeyKey } },
      headers: { 'Idempotency-Key': 'z52-verify-old-key', 'Content-Type': 'application/json' },
    });
    expect(oldKey.status()).toBe(400);
    expect(await oldKey.text()).toContain('не совпадает с принятым подключением');

    // The repeated probe with the rotated key returns the entries.
    const returned = await api.post(verifyPath('query.tools-list'), {
      data: { secrets: { api_key: rotatedKey } },
      headers: { 'Idempotency-Key': 'z52-verify-returned', 'Content-Type': 'application/json' },
    });
    expect(returned.status()).toBe(200);
    expect((await returned.json()).capability.state).toBe('verified');
    const refilled = await (await api.get(dictionaryAPI)).json();
    expect(refilled.sections[0].has_entries).toBe(true);
    expect(refilled.sections[0].entries.length).toBe(4);

    // 9. The ladder: the member without the scopes gets the 403 with the
    // scope words BEFORE any dictionary material; the foreign tenant gets
    // the plain 404 without any disclosure.
    await api.post('/fixture/operator', { form: { second: '1' } });
    const refusedPage = await page.goto(dictionaryPath);
    expect(refusedPage!.status()).toBe(403);
    await expect(page.locator('body')).toContainText('integrations');
    await expect(page.locator('body')).toContainText('configuration');
    await expect(page.locator('body')).not.toContainText('fixture-external-mcp');
    const refusedAPI = await api.get(dictionaryAPI);
    expect(refusedAPI.status()).toBe(403);
    expect(await refusedAPI.text()).not.toContain('fixture-external-mcp');
    await api.post('/fixture/operator', { form: { second: '' } });

    const foreign = await api.get(`/v1/tenants/${foreignTenantID}/tools/external`);
    expect(foreign.status()).toBe(404);
    expect(await foreign.text()).not.toContain('sections');

    // 10. The a11y close at 320px: the single column, the skip-link, the
    // semantic headings and the keyboard-operable disclosure of the
    // admitted entry schemas.
    await page.goto(dictionaryPath);
    await expect(page.locator('a.skip-link')).toBeVisible();
    await expect(page.locator('h1')).toHaveText('Словарь внешних инструментов');
    const schemaSummary = page.locator('details.action-schema > summary').first();
    await schemaSummary.focus();
    await schemaSummary.press('Enter');
    await expect(page.locator('details.action-schema').first()).toHaveAttribute('open', '');
    const summaryBox = await page.locator('details.action-schema > summary').first().boundingBox();
    expect(summaryBox!.height).toBeGreaterThanOrEqual(44);
    await expect(page.locator('html')).toHaveAttribute('lang', 'ru');
  } finally {
    await api.post('/fixture/operator', { form: { second: '' } });
    await api.post('/fixture/external-mcp-foreign-revision', { form: { enabled: '' } });
    await api.post('/fixture/external-mcp-oversized', { form: { enabled: '' } });
    await api.post('/fixture/external-mcp-leaky', { form: { enabled: '' } });
  }
});
