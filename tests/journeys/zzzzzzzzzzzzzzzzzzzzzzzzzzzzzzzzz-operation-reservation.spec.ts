import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const foreignTenantID = '01994f64-6c00-7000-8000-00000000a212';
const chatConnectionID = '01994f64-6c00-7000-8000-000000000016';
const apiBase = `/v1/tenants/${tenantID}`;

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

// The Story 3.5 chain (z33) at 320px: the honest empty world FIRST (the
// Operation Review page names the absence of the confirmations and the
// operations), the primary transition of the 3-4 card lands on the
// confirm form with the prefill of the EXACT preview pair, the human
// consent demands the LIVE step-up (the refusal first, the confirmed
// append after — the valid card with the zero consumption), the machine
// authorize through the page form lands the ONE transaction (the
// authorized/queued card with the append-only ledger — executed NEVER
// exists), the double-click and the replay job of the same client key
// answer the honest 200 replay through the machine surface, the second
// key of the consumed single_use consent hears the exhausted ladder word,
// the revocation (the human act with the checkbox discipline and the
// step-up) kills the future authorizations, the expired window walks
// through the fixture clock shift, the cross-case loser of the
// equivalent business intent reads the honest reservation reference,
// the rights rows (the operations-only member 403, the cross-tenant
// 404) and the a11y close.
test('operation-reservation: the consent ladder, the honest cards, the idempotent authorize and the a11y at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 0. The fixture switches of the vertical: the preview dictionary of
    // the z32/z33 world (the fixture-only toggle; production never
    // exposes it), the REAL telephony connection with the verified
    // read-only probe (the capability gate demands it) and the LIVE
    // entitlement walk (the earlier journeys may leave the perimeter
    // draft OR paused).
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '1' } });
    const created = await api.post(`${apiBase}/connections`, { data: {
      connector_id: 'staffy-synthetic-telephony', connector_version: '1.1.0', label: 'Телефония операций', environment: 'sandbox',
      config: { endpoint: 'https://operations.sandbox.example/api' }
    } });
    expect(created.status()).toBe(201);
    const createdView = await created.json();
    const connectionID = createdView.connection.connection_id;
    const connect = await api.post(`${apiBase}/connections/${connectionID}/connect`, { data: { version: createdView.connection.version, secrets: { api_key: 'sandbox-operations-key' } } });
    expect(connect.status()).toBe(200);
    const verify = await api.post(`${apiBase}/connections/${connectionID}/capabilities/query.trunk-status/verify`, { data: {} });
    expect(verify.status()).toBe(200);

    const entitlement = await (await api.get(`${apiBase}/entitlement`)).json();
    if (entitlement.state === 'paused') {
      const resume = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'resume', basis: 'Возобновление для операций', confirm: true } });
      expect(resume.status()).toBeLessThan(300);
    } else if (entitlement.state !== 'active') {
      const terms = await api.post(`${apiBase}/entitlement/terms`, { data: {
        currency: 'RUB', tax_regime: 'УСН', period_start: '2026-10-01T00:00:00Z', period_end: '2026-12-31T23:59:59Z',
        included_employees: ['consultant-01'], included_channels: ['jivo'], included_connectors: ['abcp'],
        launch_price: 49000, subscription_price: 15000, change_note: 'Коммерческое предложение операций'
      } });
      expect(terms.status()).toBeLessThan(300);
      const activate = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'activate', basis: 'Договор №78 от 01.10.2026', confirm: true } });
      expect(activate.status()).toBeLessThan(300);
    }

    // 1. One TEXT turn opens the case of the chain; the machine proposal
    // builds preview version 1 of the manual-only vertical (the
    // single_use unsafe class of the fixture dictionary).
    const first = await postIngress(api, textInboundEvent('journey-op-evt-1', 'journey-op-sess-1', { say: 'Подтвердите отмену созвона и авторизуйте уведомление' }), 'journey-op-ingress-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const caseView0 = await (await api.get(`${apiBase}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;
    const machinePath = `${apiBase}/cases/${caseA}/operations`;
    const reviewPath = `/app/tenants/${tenantID}/cases/${caseA}/operations`;
    const proposal = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a301',
      typed_input: { phone: '+79000000401' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal.status()).toBe(200);
    const proposalView = (await proposal.json()).proposal;
    expect(proposalView.verdict).toBe('valid');
    const previewID = proposalView.preview_id;

    // 2. The honest empty world: the Operation Review page names the
    // absence BEFORE any consent exists; the machine view answers the
    // honest empty arrays (never a null).
    await page.goto(reviewPath);
    await expect(page.locator('h1')).toContainText('Operation Review');
    await expect(page.locator('#operations-empty-heading')).toHaveText('Подтверждений и операций нет');
    const emptyView = await (await api.get(machinePath)).json();
    expect(emptyView.authorizations.confirmations).toEqual([]);
    expect(emptyView.authorizations.operations).toEqual([]);
    expect(emptyView.authorizations.empty_text).toContain('Подтверждений и авторизованных операций нет');

    // 3. The primary transition of the 3-4 card lands the confirm form
    // with the prefill of the EXACT preview pair (the page NEVER
    // generates the machine idempotency word — the consent is the human
    // act, the authorize is the machine surface).
    const previewPage = `/app/tenants/${tenantID}/cases/${caseA}/action-previews`;
    await page.goto(previewPage);
    const primaryLink = page.locator('a.primary', { hasText: 'Перейти к подтверждению и авторизации' }).first();
    await expect(primaryLink).toHaveAttribute('href', `/app/tenants/${tenantID}/cases/${caseA}/operations?confirm=${previewID}@1`);
    await primaryLink.click();
    await expect(page.locator('#confirm_preview_id')).toHaveValue(previewID);
    await expect(page.locator('#confirm_version_no')).toHaveValue('1');

    // 4. The human consent demands the LIVE step-up: the honest refusal
    // first (the fixture step-up is off), the confirmed append after.
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
    await expect(consentCard).toContainText('Потребление: Потреблено 0 из 1.');

    // 5. The machine authorize through the page form: the empty key
    // flashes BEFORE the service; the honest key lands the ONE
    // transaction — the authorized/queued card with the append-only
    // ledger, the two key identities and the honest absence texts
    // (executed NEVER exists).
    // The empty key flashes BEFORE the service through the machine
    // surface (the page form guards the required field with the HTML5
    // discipline — the honest server gate runs on the machine path).
    const consentView = (await (await api.get(machinePath)).json());
    const confirmationID = consentView.authorizations.confirmations[0].confirmation_id;
    const emptyKey = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmationID}/versions/1/authorizations`, { data: { idempotency_key: 'short' } });
    expect(emptyKey.status()).toBe(400);
    expect((await emptyKey.json()).message).toContain('Idempotency key');
    const authForm = consentCard.locator('details', { hasText: 'Авторизовать Operation по этому подтверждению' });
    await authForm.locator('summary').click();
    await authForm.locator('input[name="idempotency_key"]').fill('journey-op-key-1');
    await authForm.locator('button.primary', { name: 'Авторизовать Operation' }).click();
    await expect(page).toHaveURL(/\?authorized=[0-9a-f-]+$/);
    await expect(page.locator('#page-notice')).toContainText('авторизована и в очереди на будущий dispatch');
    await expect(page.locator('#page-notice')).toContainText('одной транзакцией');
    const operationCard = page.locator('.action-card').filter({ hasText: 'Состояние жизни: authorized' }).first();
    await expect(operationCard).toContainText('Состояние жизни: authorized');
    await expect(operationCard).toContainText('Operation Ledger (append-only)');
    await expect(operationCard).toContainText('authorized');
    await expect(operationCard).toContainText('Внешний идентификатор не выдан');
    await expect(operationCard).toContainText('Dispatch — явная команда drain');
    const machineView = await (await api.get(machinePath)).json();
    const operationID = machineView.authorizations.operations[0].operation_id;
    expect(machineView.authorizations.operations[0].idempotency_key).toBe('journey-op-key-1');
    expect(machineView.authorizations.operations[0].state).toBe('authorized');
    expect(machineView.authorizations.operations[0].queued).toBe(true);
    const pageText = await page.locator('#main').innerText();
    expect(pageText).not.toContain('executed');

    // 6. The replay contract of one client key through the machine
    // surface: the double-click and the replay job answer the honest
    // 200 replay of the ORIGINAL operation; the confirmation view stays
    // consumed ONCE (the single_use words of the consumed consent).
    const repeat = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmationID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-op-key-1' } });
    expect(repeat.status()).toBe(200);
    const repeatView = (await repeat.json()).authorization;
    expect(repeatView.replay).toBe(true);
    expect(repeatView.reservation_reference).toBe(false);
    expect(repeatView.outcome_text).toContain('возвращена первоначальная Operation');
    const replayJob = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmationID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-op-key-1' } });
    expect(replayJob.status()).toBe(200);
    const afterReplay = (await (await api.get(machinePath)).json());
    expect(afterReplay.authorizations.operations.length).toBe(1);
    expect(afterReplay.authorizations.operations[0].operation_id).toBe(operationID);
    expect(afterReplay.authorizations.confirmations[0].consumption_text).toContain('Потреблено 1 из 1');

    // 7. The second key of the consumed single_use consent hears the
    // exhausted ladder word (the machine 409 with the honest state
    // text — the consumption never repeats).
    const secondKey = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmationID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-op-key-2' } });
    expect(secondKey.status()).toBe(409);
    expect((await secondKey.json()).message).toContain('исчерпана');

    // 8. The expired window walks through the fixture clock shift: the
    // fresh consent of the second preview lives the minimum window (60
    // seconds of the server clamp), the shifted clock walks the expired
    // ladder word, the clock reset restores the live truth.
    const proposal2 = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a302',
      typed_input: { phone: '+79000000402' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposal2.status()).toBe(200);
    const preview2ID = (await proposal2.json()).proposal.preview_id;
    const confirm2 = await api.post(`${apiBase}/cases/${caseA}/action-previews/${preview2ID}/versions/1/confirmations`, { data: { lifetime_seconds: 5 } });
    expect(confirm2.status()).toBe(201);
    const confirm2View = (await confirm2.json()).confirmation;
    expect(confirm2View.valid_until).toBeTruthy();
    const confirmation2ID = confirm2View.confirmation_id;
    await api.post('/fixture/clock', { form: { minutes: '3' } });
    const expired = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmation2ID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-op-key-3' } });
    expect(expired.status()).toBe(409);
    expect((await expired.json()).message).toContain('истёк');
    await api.post('/fixture/clock', { form: { minutes: '0' } });

    // 9. The revocation (the human act with the checkbox discipline and
    // the step-up): the killed consent answers the revoked ladder word
    // on the machine surface; the past consumption stays the audit fact
    // (the first operation of the case stays authorized).
    await page.goto(reviewPath);
    const consent2Card = page.locator('.action-card', { hasText: confirmation2ID });
    await expect(consent2Card).toHaveCount(1);
    const revokeForm = consent2Card.locator('details', { hasText: 'Отозвать это подтверждение' });
    await revokeForm.locator('summary').click();
    // The checkbox discipline guards the submit client-side (the HTML5
    // required field — the unchecked form never posts).
    await revokeForm.locator('button.primary', { name: 'Отозвать подтверждение' }).click();
    await expect(page).not.toHaveURL(/\?revoked=/);
    const revokeCheckbox = revokeForm.locator('input[name="confirm_revocation"]');
    expect(await revokeCheckbox.evaluate((el) => (el as HTMLInputElement).required)).toBe(true);
    await revokeCheckbox.check();
    await revokeForm.locator('button.primary', { name: 'Отозвать подтверждение' }).click();
    await expect(page).toHaveURL(/\?revoked=1$/);
    await expect(page.locator('#page-notice')).toContainText('отозвана');
    await expect(consent2Card).toContainText('Состояние: revoked');
    const revokedAuth = await api.post(`${apiBase}/cases/${caseA}/confirmations/${confirmation2ID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-op-key-4' } });
    expect(revokedAuth.status()).toBe(409);
    expect((await revokedAuth.json()).message).toContain('отозвана');

    // 10. The cross-case loser of the equivalent business intent: the
    // second case prepares the SAME target and the SAME typed intent,
    // its OWN consent authorizes with a NEW client key — the unique
    // tenant reservation admits exactly ONE operation, the loser reads
    // the honest 200 reference of the existing operation (the loser's
    // consumption never happened).
    const second = await postIngress(api, textInboundEvent('journey-op-evt-2', 'journey-op-sess-2', { say: 'Тоже подтвердите отмену созвона' }), 'journey-op-ingress-2');
    expect(second.status()).toBe(201);
    const caseB = (await second.json()).case_id;
    const caseBView = await (await api.get(`${apiBase}/cases/${caseB}`)).json();
    const workB = caseBView.work_items[0].WorkItemID;
    const proposalB = await api.post(`${apiBase}/cases/${caseB}/work-items/${workB}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a301',
      typed_input: { phone: '+79000000401' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(proposalB.status()).toBe(200);
    const previewBID = (await proposalB.json()).proposal.preview_id;
    const confirmB = await api.post(`${apiBase}/cases/${caseB}/action-previews/${previewBID}/versions/1/confirmations`, { data: { lifetime_seconds: 3600 } });
    expect(confirmB.status()).toBe(201);
    const confirmBID = (await confirmB.json()).confirmation.confirmation_id;
    const loser = await api.post(`${apiBase}/cases/${caseB}/confirmations/${confirmBID}/versions/1/authorizations`, { data: { idempotency_key: 'journey-op-key-5' } });
    expect(loser.status()).toBe(200);
    const loserView = (await loser.json()).authorization;
    expect(loserView.reservation_reference).toBe(true);
    expect(loserView.replay).toBe(false);
    expect(loserView.outcome_text).toContain('уже зарезервировано');
    expect(loserView.operation.operation_id).toBe(operationID);
    const caseBOperations = (await (await api.get(`/v1/tenants/${tenantID}/cases/${caseB}/operations`)).json());
    expect(caseBOperations.authorizations.operations.length).toBe(0);
    expect(caseBOperations.authorizations.confirmations[0].consumption_text).toContain('Потреблено 0');

    // 11. The rights rows: the operations-only member hears the honest
    // 403 with the scope words; the cross-tenant read is the plain 404
    // on both surfaces (no disclosure that the case confirms actions).
    await api.post('/fixture/operator', { form: { second: 'ops' } });
    const opsView = await api.get(machinePath);
    expect(opsView.status()).toBe(403);
    expect((await opsView.json()).message).toContain('review или configuration');
    await api.post('/fixture/operator', { form: { second: '' } });
    const foreignPageView = await api.get(`/app/tenants/${foreignTenantID}/cases/${caseA}/operations`);
    expect(foreignPageView.status()).toBe(404);
    const foreignMachineView = await api.get(`/v1/tenants/${foreignTenantID}/cases/${caseA}/operations`);
    expect(foreignMachineView.status()).toBe(404);

    // 12. The a11y close at 320px: the skip-link, the semantic heading
    // ladder, the ≥44×44 targets of the disclosure summaries and the
    // primary buttons, the KEYBOARD toggle of a CLOSED disclosure and
    // the single column without the horizontal scroll.
    await page.goto(reviewPath);
    await expect(page.locator('a.skip-link')).toBeVisible();
    await expect(page.locator('h1')).toContainText('Operation Review');
    const headings = await page.locator('h2').all();
    expect(headings.length).toBeGreaterThan(0);
    const disclosure = page.locator('details', { hasText: 'Отозвать это подтверждение' }).first().locator('summary');
    const disclosureDetails = page.locator('details', { hasText: 'Отозвать это подтверждение' }).first();
    await disclosure.focus();
    await disclosure.press('Enter');
    await expect(disclosureDetails).toHaveAttribute('open', '');
    const disclosureBox = await disclosure.boundingBox();
    expect(disclosureBox!.height).toBeGreaterThanOrEqual(44);
    const primaryBox = await page.getByRole('button', { name: 'Подтвердить версию preview' }).boundingBox();
    expect(primaryBox!.height).toBeGreaterThanOrEqual(44);
    expect(primaryBox!.width).toBeGreaterThanOrEqual(44);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    // The fixture reset: the next journey starts at the honest production
    // default (the empty dictionary, the primary owner, the live clock, the
    // step-up off).
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '0' } });
    await api.post('/fixture/operator', { form: { second: '' } });
    await api.post('/fixture/step-up', { form: { enabled: '0' } });
    await api.post('/fixture/clock', { form: { minutes: '0' } });
    await api.post('/fixture/preview-material', { form: { field: 'order_status', status: '' } });
  }
});
