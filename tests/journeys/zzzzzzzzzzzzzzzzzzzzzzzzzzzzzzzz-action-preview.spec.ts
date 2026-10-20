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

// The Story 3-4 chain (z32) at 320px: the honest empty world FIRST (the
// page names the absence and the discipline of the slice — no dispatch
// exists, the primary action is the transition to the authorization that
// does not exist yet), the LIVE entitlement walk (the draft block word,
// then the activation through the REAL machine surface opens the path),
// the proposal ladder through the REAL page forms (the unknown field is
// honestly invalid with the full violation words, the repair builds
// version 1), the built card with the honest words (the Timeline, the
// ready readiness over the fresh snapshot, the staffy partition, the
// finite effects, the digest, the authorization-absence primary — NEVER a
// hidden dispatch), the dry-run evidence that never moves the Timeline,
// the material toggle (готовность ≠ существование: the blocked readiness
// keeps the preview readable), the edit ladder (v2 + the read-time diff +
// the superseded v1), the unsafe delivery class (the human_only
// partition with the recovery instruction and the mandatory human
// owner), the AG-4 capability boundary (the mutating send-sms never
// verifies — the honest policy word), the machine ladder (the strict
// decode, the exhausted attempts), the rights rows (the operations-only
// member 403, the cross-tenant 404) and the a11y close.
test('action-preview: the proposal ladder, the honest card, the dry-run evidence and the a11y at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 0. The fixture switches of the vertical: the preview dictionary of
    // the z32 world (the fixture-only toggle; production never exposes
    // it), the REAL telephony connection with the verified read-only
    // probe (the capability gate demands it) and the LIVE entitlement
    // walk (terms + activation through the REAL machine surface — the
    // draft state is the honest first block word of the journey).
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '1' } });
    const created = await api.post(`${apiBase}/connections`, { data: {
      connector_id: 'staffy-synthetic-telephony', connector_version: '1.1.0', label: 'Телефония подготовки', environment: 'sandbox',
      config: { endpoint: 'https://preview.sandbox.example/api' }
    } });
    expect(created.status()).toBe(201);
    const createdView = await created.json();
    const connectionID = createdView.connection.connection_id;
    const connect = await api.post(`${apiBase}/connections/${connectionID}/connect`, { data: { version: createdView.connection.version, secrets: { api_key: 'sandbox-preview-key' } } });
    expect(connect.status()).toBe(200);
    const verify = await api.post(`${apiBase}/connections/${connectionID}/capabilities/query.trunk-status/verify`, { data: {} });
    expect(verify.status()).toBe(200);

    // The LIVE entitlement walk: the earlier journeys of the family may
    // leave the perimeter draft OR paused (the activation only lifts the
    // draft — a paused world needs the honest resume first).
    const entitlement = await (await api.get(`${apiBase}/entitlement`)).json();
    if (entitlement.state === 'paused') {
      const resume = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'resume', basis: 'Возобновление для подготовки', confirm: true } });
      expect(resume.status()).toBeLessThan(300);
    } else if (entitlement.state !== 'active') {
      const terms = await api.post(`${apiBase}/entitlement/terms`, { data: {
        currency: 'RUB', tax_regime: 'УСН', period_start: '2026-10-01T00:00:00Z', period_end: '2026-12-31T23:59:59Z',
        included_employees: ['consultant-01'], included_channels: ['jivo'], included_connectors: ['abcp'],
        launch_price: 49000, subscription_price: 15000, change_note: 'Коммерческое предложение подготовки'
      } });
      expect(terms.status()).toBeLessThan(300);
      const activate = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'activate', basis: 'Договор №77 от 01.10.2026', confirm: true } });
      expect(activate.status()).toBeLessThan(300);
    }

    // 1. One TEXT turn opens the case of the chain (the fixture family
    // precedent: the ingress through the REAL transport, the Work Item
    // of the view material).
    const first = await postIngress(api, textInboundEvent('journey-preview-evt-1', 'journey-preview-sess-1', { say: 'Проверьте статус линии перед моим звонком' }), 'journey-preview-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const caseView0 = await (await api.get(`${apiBase}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;
    const reviewPath = `/app/tenants/${tenantID}/cases/${caseA}/action-previews`;
    const machinePath = `${apiBase}/cases/${caseA}/action-previews`;

    // 2. The honest empty world: the page names the absence, the machine
    // view answers the honest empty threads (an array, never a null) and
    // the work item anchors of the proposal form.
    await page.goto(reviewPath);
    await expect(page.locator('h1')).toContainText('Action Preview');
    await expect(page.locator('#preview-empty-heading')).toHaveText('Preview нет');
    const emptyView = await (await api.get(machinePath)).json();
    expect(emptyView.previews.threads).toEqual([]);
    expect(emptyView.previews.work_item_ids).toContain(workItemID);
    expect(emptyView.previews.panel_text).toContain('Preview');

    // 3. The typed-invalid attempt is honestly invalid with the full
    // violation words: the unknown field and the secret field never
    // enter the preview; the attempt ladder row lands on the page.
    const targetRef = '019a3f44-7000-7000-8000-00000000a201';
    const fillProposal = async (typed: string) => {
      await page.locator('#work_item_id').selectOption(workItemID);
      await page.locator('#definition_pin').selectOption('query.trunk-status@1.0.0');
      await page.locator('input[name="target_kind"][value="order"]').check();
      await page.locator('#target_ref').fill(targetRef);
      await page.locator('#typed_input').fill(typed);
      await page.locator('#owner_epoch').fill('1');
      await page.getByRole('button', { name: 'Подать proposal' }).click();
    };
    await page.locator('#submit-heading').scrollIntoViewIfNeeded();
    // One unknown field only: the honest violation list of the attempt
    // stays under the 200-rune flash guard (a longer list would display
    // as the generic refusal — the exact words always travel on the
    // machine surface).
    await fillProposal('unknown_extra=значение');
    await expect(page.locator('.alert')).toContainText('Невалидный материал (попытка 1)');
    await expect(page.locator('.alert')).toContainText('unknown_extra');
    // The attempt ladder of the blocked thread renders beside the cards
    // (the thread section owns it — the honest history of the AD-7
    // ladder, the third attempt does not exist).
    const ladder = page.locator('details', { hasText: 'Лестница попыток предложения' }).first();
    await expect(ladder).toContainText('Попытка 1 — вердикт invalid');
    await expect(ladder).toContainText('unknown_extra');

    // 4. The repair builds version 1: the PRG notice speaks the honest
    // discipline AND TAKES the focus; the card carries the Timeline word
    // previewed, the ready readiness, the fresh snapshot rows, the
    // staffy partition, the one conservative effect and the digest; the
    // primary action is the HONEST transition to the authorization —
    // the page never hides a dispatch (none exists).
    await fillProposal('phone=+79000000301');
    await expect(page).toHaveURL(/\?created=1$/);
    await expect(page.locator('#page-notice')).toContainText('Preview версия 1 построена из предложения');
    await expect(page.locator('#page-notice')).toBeFocused();
    const cardOne = page.locator('.action-card', { hasText: 'query.trunk-status' }).first();
    await expect(cardOne).toContainText('Timeline: previewed');
    await expect(cardOne).toContainText('Готовность к авторизации: ready');
    await expect(cardOne).toContainText('Разграничение: действие исполняется в контуре staffy');
    await expect(cardOne).toContainText('order_status');
    await expect(cardOne).toContainText('семантика отказа: abort_remaining');
    const pageText = await page.locator('#main').innerText();
    expect(pageText).not.toContain('dispatch');
    expect(pageText).not.toContain('authorized');
    expect(pageText).not.toContain('executed');
    const authLink = cardOne.locator('a.primary', { hasText: 'Перейти к подтверждению и авторизации' });
    await expect(authLink).toHaveCount(1);

    // 5. The dry-run evidence: the confirm checkbox is the UI discipline
    // (the machine API carries none); the evidence lands, the Timeline
    // words NEVER move (AC-8) and the page says so in the fixed words.
    const cardOneText = await cardOne.innerText();
    const dryRunForm = cardOne.locator('details:has(form[action$="/versions/1/dry-runs"])');
    await dryRunForm.locator('summary').click();
    await dryRunForm.locator('input[name="confirm_dry_run"]').check();
    await dryRunForm.locator('button.primary').click();
    await expect(page).toHaveURL(/\?validated=1$/);
    await expect(page.locator('#page-notice')).toContainText('Dry-run версии 1 записан как validation evidence');
    await expect(page.locator('#page-notice')).toContainText('слова Timeline не смещаются');
    const cardOneAgain = page.locator('.action-card', { hasText: 'query.trunk-status' }).first();
    await expect(cardOneAgain).toContainText('Timeline: previewed');
    expect(await cardOneAgain.innerText()).toContain('dry-run');
    expect(cardOneText.length).toBeGreaterThan(0);

    // 6. The material walk: the readiness is the truth of the BUILD-TIME
    // snapshot (the stored rows never mutate); the stale reader status of
    // the required field builds the honest blocked preview — готовность ≠
    // существование: the blocked version EXISTS and shows the truth; the
    // fresh reset re-opens the readiness of the NEXT build.
    await api.post('/fixture/preview-material', { form: { field: 'order_status', status: 'stale' } });
    const blockedSubmit = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'query.trunk-status', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a206',
      typed_input: { phone: '+79000000306' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(blockedSubmit.status()).toBe(200);
    await page.goto(reviewPath);
    const blockedCard = page.locator('.action-card', { hasText: '00000000a206' });
    await expect(blockedCard).toHaveCount(1);
    await expect(blockedCard).toContainText('Готовность к авторизации: blocked_material');
    await expect(blockedCard).toContainText('готовность не притворяется');
    await api.post('/fixture/preview-material', { form: { field: 'order_status', status: 'fresh' } });
    const readySubmit = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'query.trunk-status', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a207',
      typed_input: { phone: '+79000000307' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(readySubmit.status()).toBe(200);
    await page.goto(reviewPath);
    await expect(page.locator('.action-card', { hasText: '00000000a207' })).toContainText('Готовность к авторизации: ready');

    // 7. The edit ladder: the changed typed material re-runs the FULL
    // chain and lands version 2 with the read-time diff; version 1 stays
    // byte-readable and honestly superseded — the authorization
    // suitability dies through the derivation, never through deletion.
    // The thread of the edited target is addressed EXACTLY: the machine
    // view names its preview id (the fixed fixture clock ties the creation
    // times — the page thread order is the id tie-break, never the build
    // order).
    const viewForThread = (await (await api.get(machinePath)).json()).previews;
    const threadA201 = viewForThread.threads.find((t: any) => t.cards.some((c: any) => c.target_ref.endsWith('a201')));
    const a201ID = threadA201.preview_id;
    const editForm = page.locator(`details:has(form[action$="/${a201ID}/versions"])`);
    await expect(editForm).toHaveCount(1);
    await editForm.locator('summary').click();
    await editForm.locator('textarea[name="typed_input"]').fill('phone=+79000000302');
    await editForm.locator('button.primary').click();
    await expect(page).toHaveURL(/\?edited=2$/);
    await expect(page.locator('#page-notice')).toContainText('Версия 2 построена полной цепочкой');
    // The thread of the edited target keeps exactly its two versions (the
    // original superseded plus the built v2); the later threads own their
    // own cards.
    const cards = page.locator('.action-card', { hasText: '00000000a201' });
    await expect(cards).toHaveCount(2);
    const freshCard = page.locator('.action-card', { hasText: 'Поле «phone» изменено' });
    await expect(freshCard).toHaveCount(1);
    await expect(freshCard).toContainText('Timeline: previewed');
    const oldCard = page.locator('.action-card', { hasText: 'Timeline: superseded' });
    await expect(oldCard).toHaveCount(1);

    // 8. The unsafe delivery class through the machine surface: the
    // human_only partition with the honest markers — the recovery
    // instruction of the declared cancel semantics, the mandatory human
    // owner, and the page keeps the same authorization transition (the
    // autonomous dispatch path is never offered).
    const unsafeSubmit = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-only', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a202',
      typed_input: { phone: '+79000000303' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(unsafeSubmit.status()).toBe(200);
    const unsafeView = (await unsafeSubmit.json());
    expect(unsafeView.proposal.verdict).toBe('valid');
    await page.goto(reviewPath);
    const manualCard = page.locator('.action-card', { hasText: 'notify.manual-only' });
    await expect(manualCard).toHaveCount(1);
    await expect(manualCard).toContainText('Разграничение: human-only — действие исполняет только человек-владелец');
    await expect(manualCard).toContainText('вручную: созвон с клиентом и отметка в журнале');
    await expect(manualCard.locator('a.primary', { hasText: 'Перейти к подтверждению и авторизации' })).toHaveCount(1);

    // 9. The AG-4 capability boundary: the mutating send-sms capability
    // never verifies in the test contour — the honest policy word of the
    // gate (the preparation refuses, the page never pretends).
    const smsSubmit = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'notify.manual-sms', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a203',
      typed_input: { phone: '+79000000304' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    } });
    expect(smsSubmit.status()).toBe(403);
    const smsBody = (await smsSubmit.json());
    expect(smsBody.proposal.blocked_reason).toBe('capability_not_verified');

    // 10. The machine ladder honesty: the strict decode refuses the
    // unknown field BEFORE the service; the exhausted attempts of one
    // key answer the terminal 409 (the third attempt does not exist).
    const strictSubmit = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: {
      definition_id: 'query.trunk-status', definition_version: '1.0.0',
      target_kind: 'order', target_ref: '019a3f44-7000-7000-8000-00000000a204',
      typed_input: { phone: '+79000000305' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1, unknown_wire_field: 'x',
    } });
    expect(strictSubmit.status()).toBe(400);
    const badKey = '019a3f44-7000-7000-8000-00000000a205';
    const exhaustedData = {
      definition_id: 'query.trunk-status', definition_version: '1.0.0',
      target_kind: 'order', target_ref: badKey,
      typed_input: { unknown_extra: 'x' }, supporting_facts: [], supporting_observations: [], owner_epoch: 1,
    };
    const ex1 = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: exhaustedData });
    expect(ex1.status()).toBe(400);
    const ex2 = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: exhaustedData });
    expect(ex2.status()).toBe(409);
    const ex3 = await api.post(`${apiBase}/cases/${caseA}/work-items/${workItemID}/action-previews/proposals`, { data: exhaustedData });
    expect(ex3.status()).toBe(409);
    expect((await ex3.json()).message).toContain('исчерпаны');

    // 11. The rights rows: the operations-only member hears the honest
    // 403 with the scope words; the cross-tenant read is the plain 404
    // on both surfaces (no disclosure that the case prepares actions).
    await api.post('/fixture/operator', { form: { second: 'ops' } });
    const opsView = await api.get(machinePath);
    expect(opsView.status()).toBe(403);
    expect((await opsView.json()).message).toContain('review или configuration');
    await api.post('/fixture/operator', { form: { second: '' } });
    const foreignPageView = await api.get(`/app/tenants/${foreignTenantID}/cases/${caseA}/action-previews`);
    expect(foreignPageView.status()).toBe(404);
    const foreignMachineView = await api.get(`/v1/tenants/${foreignTenantID}/cases/${caseA}/action-previews`);
    expect(foreignMachineView.status()).toBe(404);

    // 12. The a11y close at 320px: the skip-link, the semantic heading
    // ladder (h1 → h2), the ≥44×44 targets of the disclosure summaries
    // and the primary buttons, the KEYBOARD toggle of the disclosure and
    // the single column without the horizontal scroll.
    await page.goto(reviewPath);
    await expect(page.locator('a.skip-link')).toBeVisible();
    await expect(page.locator('h1')).toContainText('Action Preview');
    const headings = await page.locator('h2').all();
    expect(headings.length).toBeGreaterThan(0);
    // The KEYBOARD toggle walks a CLOSED disclosure (the dry-run form of
    // the current card — the snapshot details render open by default).
    const disclosure = page.locator('details', { hasText: 'Запустить dry-run этой версии' }).first().locator('summary');
    const disclosureDetails = page.locator('details', { hasText: 'Запустить dry-run этой версии' }).first();
    await disclosure.focus();
    await disclosure.press('Enter');
    await expect(disclosureDetails).toHaveAttribute('open', '');
    const disclosureBox = await disclosure.boundingBox();
    expect(disclosureBox!.height).toBeGreaterThanOrEqual(44);
    const primaryBox = await page.getByRole('button', { name: 'Подать proposal' }).boundingBox();
    expect(primaryBox!.height).toBeGreaterThanOrEqual(44);
    expect(primaryBox!.width).toBeGreaterThanOrEqual(44);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    // The fixture reset: the next journey starts at the honest production
    // default (the empty dictionary, the primary owner).
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '0' } });
    await api.post('/fixture/operator', { form: { second: '' } });
    await api.post('/fixture/preview-material', { form: { field: 'order_status', status: '' } });
  }
});
