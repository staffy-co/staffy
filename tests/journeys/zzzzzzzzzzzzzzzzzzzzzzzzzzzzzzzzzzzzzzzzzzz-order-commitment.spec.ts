import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const chatConnectionID = '01994f64-6c00-7000-8000-000000000016';
const casesPath = `/app/tenants/${tenantID}/cases`;
const apiBase = `/v1/tenants/${tenantID}`;
const jsonHeaders = { "Content-Type": "application/json", "Idempotency-Key": "journey-order-json" };

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

// The Story 4.6 chain at 320px: the client order commitment through the
// action contour — the LIVE selection (the 4.5 snapshot with the pins of
// ALL FOUR inputs), the builder proposal through the page form (the
// material derives LIVE — the operator never types the composition), the
// confirmation card with the MANDATORY manager-check word (AC-1) and the
// TWO DIFFERENT buttons of the two DIFFERENT actions (the confirmation
// through the existing 3-4 route with step-up; the correction through the
// builder re-derivation — AC-16), the machine confirmation and
// authorization, the synthetic dispatch with the committed verdict, the
// RECEIPT with the derived words (pending_manager_verification, the
// client words, the honest 1C-unknown inside role=alert — AC-8/AC-13/
// AC-14); the a11y close (320px without the horizontal scroll, the words
// as the alert TEXT) and the secrets probe.
test('order: the client order commitment from the live selection to the receipt words at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 0. The fixture switches of the vertical (the z42/z33 precedents):
    // the preview mode, the ORDER world (the composition over the
    // PRODUCT action dictionary — the first product entry — with the
    // order-serving synthetic adapters and the LIVE order version
    // source) and the LIVE entitlement walk.
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '1' } });
    await api.post('/fixture/order-world', { form: { enabled: '1' } });
    const entitlement = await (await api.get(`${apiBase}/entitlement`)).json();
    if (entitlement.state === 'paused') {
      const resume = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'resume', basis: 'Возобновление для заказа', confirm: true } });
      expect(resume.status()).toBeLessThan(300);
    } else if (entitlement.state !== 'active') {
      const terms = await api.post(`${apiBase}/entitlement/terms`, { data: {
        currency: 'RUB', tax_regime: 'УСН', period_start: '2026-10-01T00:00:00Z', period_end: '2026-12-31T23:59:59Z',
        included_employees: ['consultant-01'], included_channels: ['jivo'], included_connectors: ['abcp'],
        launch_price: 49000, subscription_price: 15000, change_note: 'Коммерческое предложение заказа'
      } });
      expect(terms.status()).toBeLessThan(300);
      const activate = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'activate', basis: 'Договор №81 от 01.10.2026', confirm: true } });
      expect(activate.status()).toBeLessThan(300);
    }

    // 1. One TEXT turn opens the case of the chain (the fixture family
    // precedent).
    const first = await postIngress(api, textInboundEvent('journey-offers-evt-o1', 'journey-offers-sess-o1', { say: 'Хочу заказать салонный фильтр WF2B196K666AA, нужно 2 штуки' }), 'journey-order-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const pathA = `${casesPath}/${caseA}`;

    // 2. The classification of the work item (the search contour).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Классифицировать / переклассифицировать"))').first().click();
    const classifyForm = page.locator('form[action$="/classify"]').first();
    await classifyForm.locator('select[name="type"]').selectOption('поиск детали');
    await classifyForm.locator('input[name="title"]').fill('Салонный фильтр');
    await classifyForm.locator('input[name="reason"]').fill('Клиент заказывает фильтр по номеру');
    await classifyForm.getByRole('button', { name: /Классифицировать/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?classified=1`));

    // 3. The part request material with the confirmations (the confirmed
    // set carries the PartRequestDigest the order pins).
    const recordFact = async (key: string, value: string) => {
      await page.goto(pathA);
      await page.locator('details:has(summary:text-is("Записать факт"))').click();
      const factForm = page.locator('form[action$="/facts"]').first();
      await factForm.locator('select[name="source_type"]').selectOption('operator');
      await factForm.locator('input[name="material_key"]').fill(key);
      await factForm.locator('textarea[name="value"]').fill(value);
      await factForm.getByRole('button', { name: /Записать факт/ }).click();
      await expect(page).toHaveURL(new RegExp(`${caseA}\\?fact_recorded=1`));
    };
    await recordFact('part.kind', '"фильтр салона"');
    await recordFact('part.oem_number', '"WF2B196K666AA"');
    await recordFact('part.quantity', '2');
    const confirmFact = async (key: string, note: string) => {
      await page.goto(pathA);
      const fact = page.locator(`details:has(summary:text-is("${key} — Без подтверждения"))`).first();
      await fact.click();
      const confirmDetails = fact.locator('details:has(summary:text-is("Подтвердить точную версию факта ${key}"))'.replace('${key}', key));
      await confirmDetails.click();
      const confirmForm = confirmDetails.locator('form[action$="/confirmations"]');
      await confirmForm.locator('select[name="version_no"]').selectOption('1');
      await confirmForm.locator('select[name="evidence_event_id"]').selectOption({ label: '— без события, использовать ноту —' });
      await confirmForm.locator('input[name="note"]').fill(note);
      await confirmForm.getByRole('button', { name: /Подтвердить версию/ }).click();
      await expect(page).toHaveURL(new RegExp(`${caseA}\\?confirmed=1`));
    };
    await confirmFact('part.kind', 'Клиент назвал деталь по номеру');
    await confirmFact('part.oem_number', 'Номер из разговора');
    await confirmFact('part.quantity', 'Количество названо клиентом');

    // 4. The sales mode of the case (the order definition admits the
    // sales conversation only): the raise from the admission floor.
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Предложить режим (сигнал — предложение только поднимает)"))').click();
    const proposeForm = page.locator('form[action$="/mode/proposals"]').first();
    await proposeForm.locator('select[name="mode"]').selectOption('sales');
    await proposeForm.locator('select[name="scope"]').selectOption('conversation');
    await proposeForm.locator('input[name="reason"]').fill('Клиент хочет заказать деталь');
    await proposeForm.getByRole('button', { name: /Предложить режим/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?mode_proposed=1`));

    // 5. The proven fitment and the LIVE offer of the supplier (the
    // 4.4/4.5 forms — the material inputs of the order).
    await page.goto(pathA);
    const caseViewA = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const workItemA = caseViewA.work_items[0].WorkItemID;
    const fitmentForm = page.locator(`form[action$="/work-items/${workItemA}/fitment-evidences"]`).first();
    await fitmentForm.locator('input[name="position_key"]').fill('WF2B196K666AA');
    await fitmentForm.locator('input[name="catalog_node"]').fill('Фильтр салона');
    await fitmentForm.locator('input[name="article"]').fill('WF2B196K666AA');
    await fitmentForm.locator('select[name="link_type"]').selectOption('original');
    await fitmentForm.locator('textarea[name="chain"]').fill('Фильтр салона (оригинальная позиция)');
    await fitmentForm.locator('textarea[name="catalog_vehicle_ids"]').fill('ABCP-12345');
    await fitmentForm.locator('select[name="source_system"]').selectOption('abcp_catalog');
    await fitmentForm.locator('input[name="source_record_id"]').fill('rec-order-fitment-1');
    await fitmentForm.locator('input[name="source_version"]').fill('v2026.10');
    await fitmentForm.locator('input[name="source_observed_at"]').fill('2026-09-16T11:00:00Z');
    await fitmentForm.getByRole('button', { name: /Записать доказательство/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?fitment_evidence_recorded=1`));

    await page.goto(pathA);
    const offerForm = page.locator(`form[action$="/work-items/${workItemA}/supplier-offers"]`).first();
    await offerForm.locator('input[name="position_key"]').fill('WF2B196K666AA');
    await offerForm.locator('input[name="supplier_id"]').fill('SUP-2');
    await offerForm.locator('input[name="supplier_name"]').fill('Поставщик два');
    await offerForm.locator('input[name="brand"]').fill('Mann');
    await offerForm.locator('input[name="article"]').fill('WF2B196K666AA');
    await offerForm.locator('input[name="client_price"]').fill('4200');
    await offerForm.locator('input[name="purchase_price"]').fill('');
    await offerForm.locator('select[name="currency"]').selectOption('RUB');
    await offerForm.locator('input[name="quantity_available"]').fill('6');
    await offerForm.locator('input[name="term_days"]').fill('1');
    await offerForm.locator('input[name="conditions"]').fill('самовывоз со склада');
    await offerForm.locator('select[name="source_system"]').selectOption('abcp_catalog');
    await offerForm.locator('input[name="source_record_id"]').fill('rec-order-1');
    await offerForm.locator('input[name="source_version"]').fill('v2026.10');
    await offerForm.locator('input[name="source_observed_at"]').fill('2026-09-16T11:30:00Z');
    await offerForm.getByRole('button', { name: /Записать предложение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?supplier_offer_recorded=1`));

    // 6. The LIVE selection (the snapshot with the pins of ALL FOUR
    // inputs — the material the builder derives the order from).
    await page.goto(pathA);
    const selectionForm = page.locator(`form[action$="/work-items/${workItemA}/selection-snapshots"]`).first();
    await selectionForm.locator('input[name="position_key"]').fill('WF2B196K666AA');
    await selectionForm.locator('input[name="supplier_id"]').fill('SUP-2');
    await selectionForm.locator('input[name="offer_no"]').fill('1');
    await selectionForm.locator('input[name="quantity"]').fill('2');
    await selectionForm.getByRole('button', { name: /Записать выбор/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?client_selection_recorded=1`));

    // 7. The order panel BEFORE the build: the buildable state with the
    // honest discipline note (the correction rebuilds the material LIVE —
    // the composition is never typed).
    await page.goto(pathA);
    const orderPanel = page.locator('section[aria-label="Заказ клиента"], #order').first();
    await expect(page.locator('#order form[action$="/order-proposals"]')).toHaveCount(1); // the build affordance, no card yet
    await expect(page.getByRole('heading', { name: 'Заказ клиента' })).toBeVisible();
    await expect(page.locator('body')).toContainText('оператор НЕ вводит состав заказа');
    await expect(page.locator('body')).toContainText('заказ сначала будет проверен менеджером и ещё не отправляется поставщику');

    // 8. The BUILD through the page form (the anchors only — the
    // material derives LIVE): the PRG landing speaks the built word.
    const buildForm = page.locator(`form[action$="/work-items/${workItemA}/order-proposals"]`).first();
    await buildForm.getByRole('button', { name: /Собрать заказ из живого выбора/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?order_proposal_built=1`));

    // 9. The confirmation card: the material lines (the position with the
    // brand/article, the originality word, the quantity, the price), the
    // known sum with the boundary word, the MANDATORY manager-check word
    // (AC-1), the digest pin and the TWO DIFFERENT buttons (AC-16).
    await page.goto(pathA);
    const card = page.locator('details:has(summary:has-text("Карточка подтверждения заказа"))').first();
    await expect(card).toBeVisible();
    await expect(card).toContainText('create_abcp_customer_order@1.0.0');
    await expect(card).toContainText('позиция WF2B196K666AA: Mann / WF2B196K666AA (оригинал); кол-во 2; цена 4200.00 RUB');
    await expect(card).toContainText('итого: 8400.00 RUB');
    await expect(card).toContainText('без доставки и сбора — окончательная стоимость не рассчитана');
    await expect(card).toContainText('заказ сначала будет проверен менеджером и ещё не отправляется поставщику');
    await expect(card).toContainText('назначается проверяющему менеджеру');
    await expect(card.locator('.digest-block').first()).toContainText(/^[0-9a-f]{64}$/);
    const confirmOrderForm = page.locator('form[action$="/operations/confirmations"]').first();
    await expect(confirmOrderForm.getByRole('button', { name: /Подтвердить точную версию заказа/ })).toBeVisible();
    const rebuildForm = card.locator('form[action$="/order-proposals"]').first();
    await expect(rebuildForm.getByRole('button', { name: /Пересобрать заказ из живого выбора/ })).toBeVisible();
    // The creation is never masked as a supplier send (AC-16 tail).
    const cardText = await card.innerText();
    expect(cardText).not.toContain('отправить поставщику');

    // 10. The machine confirmation through the panel form (the existing
    // 3-4 route with step-up — the deliberate human act).
    await api.post('/fixture/step-up', { form: { enabled: '1' } });
    await confirmOrderForm.locator('input[name="lifetime_seconds"]').fill('600');
    await confirmOrderForm.getByRole('button', { name: /Подтвердить точную версию заказа/ }).click();
    // The PRG landing of the consent is the OPERATIONS review page (the existing 3-4 route).
    await expect(page).toHaveURL(/\/operations\?confirmed=[0-9]+$/);

    // 11. The machine authorization through the operations page (the
    // idempotency key of the client's ask) and the synthetic dispatch
    // with the committed verdict + the structural evidence.
    const reviewPath = `${casesPath}/${caseA}/operations`;
    await page.goto(reviewPath);
    const consentCard = page.locator('.action-card', { hasText: 'Confirmation' }).first();
    await expect(consentCard).toContainText('Состояние: valid');
    const authDetails = consentCard.locator('details', { hasText: 'Авторизовать Operation по этому подтверждению' });
    await authDetails.locator('summary').click();
    await authDetails.locator('input[name="idempotency_key"]').fill('journey-order-key-42');
    await authDetails.getByRole('button', { name: /Авторизовать Operation/ }).click();
    await expect(page).toHaveURL(/\?authorized=[0-9a-f-]+$/);

    // 12. The drain through the page form (the explicit checkbox
    // discipline): the committed verdict with the evidence triple.
    await api.post('/fixture/order-dispatch-evidence', { form: { external_object_id: 'ABCP-ORDER-42', external_object_version: '3' } });
    await api.post('/fixture/order-dispatch-outcome', { form: { outcome: 'committed', correlation: 'journey-order-corr-42', guarantee: '1' } });
    await page.goto(reviewPath);
    const drainDetails = page.locator('details', { hasText: 'Запустить дренаж очереди отправок' });
    await drainDetails.locator('summary').click();
    await drainDetails.locator('input[name="confirm_drain"]').check();
    await drainDetails.locator('button.primary', { name: 'Дренировать очередь' }).click();
    await expect(page).toHaveURL(/\?drained=[0-9]+$/);
    await expect(page.locator('#page-notice')).toContainText('Очередь отправок дренирована: рассмотрено заданий');

    // 13. The RECEIPT on the case page: the external order triple, the
    // derived state word pending_manager_verification with the client
    // words, the honest 1C-unknown INSIDE role=alert and the manager
    // assignment word (AC-8/AC-13/AC-14).
    await page.goto(pathA);
    const receiptPanel = page.locator('section[aria-label="Квитанции заказа клиента"]');
    await expect(receiptPanel).toBeVisible();
    await expect(receiptPanel).toContainText('Квитанция №1');
    await expect(receiptPanel).toContainText('ABCP-ORDER-42');
    await expect(receiptPanel).toContainText('pending_manager_verification');
    await expect(receiptPanel).toContainText('заказ создан в магазине и ожидает проверки менеджера');
    await expect(receiptPanel).toContainText('не заказан у поставщика, не выкуплен, не отправлен, не гарантирован');
    await expect(receiptPanel).toContainText('проекция в 1С неизвестна');
    await expect(receiptPanel).toContainText('синхронизация не доказана');
    await expect(receiptPanel).toContainText('заказ назначается проверяющему менеджеру');
    const oneCAlert = receiptPanel.locator('.alert[role="alert"]', { hasText: 'проекция в 1С неизвестна' });
    await expect(oneCAlert).toHaveCount(1);

    // 14. The a11y close: the order section never widens the layout (the
    // tables scroll inside their wraps at 320px), the honest words are
    // the TEXT (never only color).
    // The section-level check of the family precedent (the z42 offers
    // close): the order sections scroll INSIDE their wraps — the page
    // layout never widens because of the order material.
    const sectionOverflows = await page.evaluate(() => {
      const order = document.getElementById('order');
      const receipts = document.getElementById('order-receipts');
      return [order, receipts].filter((el): el is HTMLElement => el !== null)
        .map((el) => el.scrollWidth - el.clientWidth);
    });
    for (const overflow of sectionOverflows) {
      expect(overflow).toBeLessThanOrEqual(0);
    }

    // 15. The secrets never render (the operator page carries no
    // credentials).
    const body = await page.locator('body').innerText();
    expect(body).not.toContain('api_key');
    expect(body).not.toContain('sandbox-');
    expect(body).not.toContain('github_pat_');
  } finally {
    await api.post('/fixture/order-world', { form: { enabled: '0' } });
    await page.close();
  }
});
