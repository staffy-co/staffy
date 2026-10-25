import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const chatConnectionID = '01994f64-6c00-7000-8000-000000000016';
const casesPath = `/app/tenants/${tenantID}/cases`;
const apiBase = `/v1/tenants/${tenantID}`;
const ownerID = '01994f64-6c00-7000-8000-000000000001';
const backupID = '01994f64-6c00-7000-8000-00000000000a';

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

// The Story 4.9 chain at 320px: the order status — the truthful
// projection of the order lifecycle over the recorded material of the
// three machines (the operation+receipt of 4-6, the verification events
// of 4-7, the correction events of 4-8) and the Supplier Order Fact
// Receipt of this slice. The journey walks the committed order chain
// and the APPROVED verification decision, the status panel BEFORE the
// facts (the five stages BY NAME with the confirmation state as TEXT —
// the supplier stages carry the honest «не утверждается» words, AC-4;
// the per-system blocks stay separate — ABCP with the receipt fact,
// 1С unknown OQ-4, the supplier «не записано», AC-2; the freshness and
// the three deadline words, AC-10/AC-11), the RECORD of the formed fact
// through the page form (the human receipt with the supplier order ID
// and the named time — AG-4), the REPLAY probe (the idempotent repeat
// BY KEY answers the saved fact, AC-9), the RECORD of the sent fact
// (the honest aggregate confirms the stage — the false send claim is
// impossible structurally, AC-15), the timeline of the transitions, the
// client retell words (AC-16); the a11y close (320px without the
// horizontal scroll) and the secrets probe.
test('order-status: the truthful projection from the approved verification to the recorded send at 320px', async ({ browser }) => {
  test.setTimeout(360_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 0. The fixture switches of the vertical (the z44/z45 precedents):
    // the preview mode, the ORDER world and the LIVE entitlement walk.
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '1' } });
    await api.post('/fixture/order-world', { form: { enabled: '1' } });
    const entitlement = await (await api.get(`${apiBase}/entitlement`)).json();
    if (entitlement.state === 'paused') {
      const resume = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'resume', basis: 'Возобновление для статуса заказа', confirm: true } });
      expect(resume.status()).toBeLessThan(300);
    } else if (entitlement.state !== 'active') {
      const terms = await api.post(`${apiBase}/entitlement/terms`, { data: {
        currency: 'RUB', tax_regime: 'УСН', period_start: '2026-10-01T00:00:00Z', period_end: '2026-12-31T23:59:59Z',
        included_employees: ['consultant-01'], included_channels: ['jivo'], included_connectors: ['abcp'],
        launch_price: 49000, subscription_price: 15000, change_note: 'Коммерческое предложение статуса заказа'
      } });
      expect(terms.status()).toBeLessThan(300);
      const activate = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'activate', basis: 'Договор №84 от 01.10.2026', confirm: true } });
      expect(activate.status()).toBeLessThan(300);
    }

    // 1..6. The committed order chain of the z43/z44 family (the case,
    // the classification, the part request facts, the sales mode, the
    // proven fitment, the LIVE offer, the selection, the build, the
    // confirmation, the authorization, the dispatch with the committed
    // verdict): the receipt lands — the anchor of the verification task
    // and its approved decision.
    const first = await postIngress(api, textInboundEvent('journey-offers-evt-v46', 'journey-offers-sess-v46', { say: 'Хочу заказать салонный фильтр WF2B196K666AA, нужно 2 штуки' }), 'journey-status-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const pathA = `${casesPath}/${caseA}`;

    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Классифицировать / переклассифицировать"))').first().click();
    const classifyForm = page.locator('form[action$="/classify"]').first();
    await classifyForm.locator('select[name="type"]').selectOption('поиск детали');
    await classifyForm.locator('input[name="title"]').fill('Салонный фильтр');
    await classifyForm.locator('input[name="reason"]').fill('Клиент заказывает фильтр по номеру');
    await classifyForm.getByRole('button', { name: /Классифицировать/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?classified=1`));

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

    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Предложить режим (сигнал — предложение только поднимает)"))').click();
    const proposeForm = page.locator('form[action$="/mode/proposals"]').first();
    await proposeForm.locator('select[name="mode"]').selectOption('sales');
    await proposeForm.locator('select[name="scope"]').selectOption('conversation');
    await proposeForm.locator('input[name="reason"]').fill('Клиент хочет заказать деталь');
    await proposeForm.getByRole('button', { name: /Предложить режим/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?mode_proposed=1`));

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
    await fitmentForm.locator('input[name="source_record_id"]').fill('rec-status-fitment-1');
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
    await offerForm.locator('input[name="source_record_id"]').fill('rec-status-1');
    await offerForm.locator('input[name="source_version"]').fill('v2026.10');
    await offerForm.locator('input[name="source_observed_at"]').fill('2026-09-16T11:30:00Z');
    await offerForm.getByRole('button', { name: /Записать предложение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?supplier_offer_recorded=1`));

    await page.goto(pathA);
    const selectionForm = page.locator(`form[action$="/work-items/${workItemA}/selection-snapshots"]`).first();
    await selectionForm.locator('input[name="position_key"]').fill('WF2B196K666AA');
    await selectionForm.locator('input[name="supplier_id"]').fill('SUP-2');
    await selectionForm.locator('input[name="offer_no"]').fill('1');
    await selectionForm.locator('input[name="quantity"]').fill('2');
    await selectionForm.getByRole('button', { name: /Записать выбор/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?client_selection_recorded=1`));

    await page.goto(pathA);
    const buildForm = page.locator(`form[action$="/work-items/${workItemA}/order-proposals"]`).first();
    await buildForm.getByRole('button', { name: /Собрать заказ из живого выбора/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?order_proposal_built=1`));

    await page.goto(pathA);
    const confirmOrderForm = page.locator('form[action$="/operations/confirmations"]').first();
    await api.post('/fixture/step-up', { form: { enabled: '1' } });
    await confirmOrderForm.locator('input[name="lifetime_seconds"]').fill('600');
    await confirmOrderForm.getByRole('button', { name: /Подтвердить точную версию заказа/ }).click();
    await expect(page).toHaveURL(/\/operations\?confirmed=[0-9]+$/);

    const reviewPath = `${casesPath}/${caseA}/operations`;
    await page.goto(reviewPath);
    const consentCard = page.locator('.action-card', { hasText: 'Confirmation' }).first();
    const authDetails = consentCard.locator('details', { hasText: 'Авторизовать Operation по этому подтверждению' });
    await authDetails.locator('summary').click();
    await authDetails.locator('input[name="idempotency_key"]').fill('journey-status-key-42');
    await authDetails.getByRole('button', { name: /Авторизовать Operation/ }).click();
    await expect(page).toHaveURL(/\?authorized=[0-9a-f-]+$/);

    await api.post('/fixture/order-dispatch-evidence', { form: { external_object_id: 'ABCP-ORDER-46', external_object_version: '3' } });
    await api.post('/fixture/order-dispatch-outcome', { form: { outcome: 'committed', correlation: 'journey-status-corr-46', guarantee: '1' } });
    await page.goto(reviewPath);
    const drainDetails = page.locator('details', { hasText: 'Запустить дренаж очереди отправок' });
    await drainDetails.locator('summary').click();
    await drainDetails.locator('input[name="confirm_drain"]').check();
    await drainDetails.locator('button.primary', { name: 'Дренировать очередь' }).click();
    await expect(page).toHaveURL(/\?drained=[0-9]+$/);

    // 7. The committed receipt on the case page — the ANCHOR of the
    // verification task and its approved decision.
    await page.goto(pathA);
    const receiptPanel = page.locator('section[aria-label="Квитанции заказа клиента"]');
    await expect(receiptPanel).toContainText('ABCP-ORDER-46');

    // 8. The verification task and the APPROVED decision: the assignment,
    // the acceptance, then the «Подтвердить проверку» decision — the
    // anchor state of the fact ladder.
    const authView = await (await api.get(`${apiBase}/cases/${caseA}/operations`)).json();
    const operation = authView.authorizations.operations.find((op: any) => op.definition_id === 'create_abcp_customer_order');
    const operationID = operation.operation_id;
    const assignForm = page.locator('form[action$="/order-verifications"]').first();
    await assignForm.locator('input[name="work_item_id"]').fill(workItemA);
    await assignForm.locator('input[name="operation_id"]').fill(operationID);
    await assignForm.locator('input[name="assignee"]').fill(ownerID);
    await assignForm.locator('input[name="backup_owner"]').fill(backupID);
    await assignForm.locator('select[name="deadline"]').selectOption('verify_4h');
    await assignForm.getByRole('button', { name: /Назначить задачу проверки/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?verification_assigned=1`));

    await page.goto(pathA);
    await page.locator('form[action$="/order-verifications/1/acknowledgements"]').first().getByRole('button', { name: /Принять задачу проверки/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?verification_acknowledged=1`));

    await page.goto(pathA);
    const approveForm = page.locator('form[action$="/order-verifications/1/approvals"]').first();
    await approveForm.getByRole('button', { name: /Подтвердить проверку/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?verification_approved=1`));

    // 8. The status panel BEFORE the facts: the five stages BY NAME with
    // the confirmation state as TEXT — the created stage carries the
    // receipt evidence (the system receipt BY NAME), the verified stage
    // carries the approved decision, the supplier stages carry the
    // honest «не утверждается» words (AC-4); the per-system blocks stay
    // separate (AC-2): ABCP with the receipt fact, 1С unknown (OQ-4),
    // the supplier «не записано» (AG-4); the next step names the manual
    // formation; the freshness and the three deadline words ride the
    // boundary blocks (AC-10/AC-11).
    await page.goto(pathA);
    const statusSection = page.locator('#order-status');
    await expect(page.getByRole('heading', { name: 'Статус заказа' })).toBeVisible();
    await expect(statusSection).toContainText('контур 4-9');
    await expect(statusSection).toContainText('расчётное представление не становится новым источником истины');
    await expect(statusSection).toContainText('client_order_created');
    await expect(statusSection).toContainText('pending_manager_verification');
    await expect(statusSection).toContainText('manager_verified');
    await expect(statusSection).toContainText('supplier_formed');
    await expect(statusSection).toContainText('supplier_sent');
    // The current stage word: the manager-verified stage — the honest
    // highest confirmed stage BEFORE the facts.
    await expect(statusSection).toContainText('manager_verified — проверка менеджером подтверждена решением «Подтвердить проверку»');
    // The supplier stages stay unclaimed (AC-4): the honest words.
    await expect(statusSection).toContainText('записи о формировании у поставщика нет: утверждение о формировании не делается');
    await expect(statusSection).toContainText('запись об отправке поставщику отсутствует: утверждение об отправке не делается');
    // The per-system blocks: ABCP with the receipt fact (the system
    // receipt BY NAME), 1С unknown (OQ-4), the supplier «не записано».
    await expect(statusSection).toContainText('ABCP (магазин)');
    await expect(statusSection).toContainText('создан (квитанция диспетчерского контура)');
    await expect(statusSection).toContainText('system_receipt');
    await expect(statusSection).toContainText('ABCP-ORDER-46');
    await expect(statusSection).toContainText('1С (бухгалтерия)');
    await expect(statusSection).toContainText('проекция в 1С неизвестна (unknown)');
    await expect(statusSection).toContainText('OQ-4');
    await expect(statusSection).toContainText('Поставщик');
    await expect(statusSection).toContainText('не записано: фактов формирования/отправки поставщику нет');
    // The next step names the manual formation (AG-4).
    await expect(statusSection).toContainText('сформировать заказ у поставщика в ABCP вручную');
    // The freshness and the deadline words (AC-10/AC-11).
    await expect(statusSection).toContainText('живая деривация при открытии панели');
    await expect(statusSection).toContainText('расписание не существует');
    await expect(statusSection).toContainText('подтверждённая дата — не существует: подтверждения поставщиком нет');
    // The boundary blocks: the notifications never send (OQ-1/OQ-2), the
    // dispute never rewrites (AC-14), the cancellation lives in 4-10.
    await expect(statusSection).toContainText('уведомления о переходах не отправляются');
    await expect(statusSection).toContainText('новая версия статуса не переписывает историю');
    await expect(statusSection).toContainText('отмена заказа — контур 4-10');
    // The timeline renders the transitions of the machines (AC-9).
    await expect(statusSection).toContainText('receipt_committed');
    await expect(statusSection).toContainText('verification_approved');

    // 9. The RECORD of the formed fact through the page form: the kind
    // select, the supplier order ID and the named time — the human
    // receipt of what ALREADY happened in ABCP (AG-4).
    const factForm = page.locator('form[action$="/order-supplier-facts"]').first();
    await expect(factForm.locator('input[name="operation_id"]')).toHaveValue(operationID);
    await factForm.locator('select[name="kind"]').selectOption('formed');
    await factForm.locator('input[name="supplier_order_id"]').fill('SUP-46');
    await factForm.locator('input[name="happened_at"]').fill('2026-10-25T10:00');
    await factForm.getByRole('button', { name: /Записать факт/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?supplier_fact_formed=1`));
    await expect(page.locator('#order-status')).toContainText('Факт формирования записан менеджером — человеческая квитанция контура магазина');

    // 10. The formed stage confirms with the manager source; the sent
    // stage stays unclaimed (the partial aggregate — AC-15).
    await page.goto(pathA);
    await expect(statusSection).toContainText('supplier_formed — заказ поставщику сформирован (запись менеджера с номером заказа поставщика)');
    await expect(statusSection).toContainText('SUP-46');
    await expect(statusSection).toContainText('manager_record');
    await expect(statusSection).toContainText('записано менеджером: чтение ABCP адаптером не подключено (AG-4)');

    // 11. The REPLAY probe (AC-9): the repeated formed record of the
    // SAME key answers the saved fact — the idempotent word lands, no
    // second row exists.
    await page.goto(pathA);
    const replayForm = page.locator('form[action$="/order-supplier-facts"]').first();
    await replayForm.locator('select[name="kind"]').selectOption('formed');
    await replayForm.locator('input[name="supplier_order_id"]').fill('SUP-46');
    await replayForm.locator('input[name="happened_at"]').fill('2026-10-25T10:00');
    await replayForm.getByRole('button', { name: /Записать факт/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?supplier_fact_replay=1`));
    await expect(page.locator('#order-status')).toContainText('Повтор команды факта: ответ идемпотентен — возвращён сохранённый факт, второй строки нет');
    // The supplier system block carries exactly ONE formed fact of
    // SUP-46 (the replay never doubles).
    await page.goto(pathA);
    const supplierRows = await page.locator('#order-status table').filter({ hasText: 'SUP-46' }).count();
    expect(supplierRows).toBeGreaterThan(0);

    // 12. The RECORD of the sent fact: the honest aggregate confirms the
    // sent stage — the current stage word moves to supplier_sent, the
    // actual deadline event records with its time (AC-11/AC-15).
    await page.goto(pathA);
    const sentForm = page.locator('form[action$="/order-supplier-facts"]').first();
    await sentForm.locator('select[name="kind"]').selectOption('sent');
    await sentForm.locator('input[name="supplier_order_id"]').fill('SUP-46');
    await sentForm.locator('input[name="happened_at"]').fill('2026-10-25T10:30');
    await sentForm.getByRole('button', { name: /Записать факт/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?supplier_fact_sent=1`));
    await page.goto(pathA);
    await expect(statusSection).toContainText('supplier_sent — заказ поставщику отправлен (все записанные формирования покрыты записями отправки)');
    // The timeline carries both supplier fact transitions (AC-9).
    await expect(statusSection).toContainText('supplier_fact_formed');
    await expect(statusSection).toContainText('supplier_fact_sent');
    // The client retell words ride the panel (AC-16): the current stage
    // word and the 1C unknown word — the boundary word is GONE after the
    // confirmed send (the «не отправлен» claim would be FALSE now — the
    // honest client words follow the projection).
    await expect(statusSection).toContainText('Клиентские слова');
    await expect(statusSection).toContainText('проекция в 1С неизвестна (unknown)');

    // 13. The a11y close: the status section never widens the layout
    // (the tables scroll inside their wraps at 320px — the family
    // precedent of the z43/z44/z45 close).
    const sectionOverflows = await page.evaluate(() => {
      const status = document.getElementById('order-status');
      return [status].filter((el): el is HTMLElement => el !== null)
        .map((el) => el.scrollWidth - el.clientWidth);
    });
    for (const overflow of sectionOverflows) {
      expect(overflow).toBeLessThanOrEqual(0);
    }

    // 14. The secrets never render (the manager page carries no
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
