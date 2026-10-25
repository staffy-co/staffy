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

// The Story 4.7 chain at 320px: the manager verification of the committed
// client order — the Delegated Task over the committed receipt (the task
// is built over the CREATED order only), the assignment (назначение ≠
// принятие — the delivery boundary word of the surface, the external
// channel does not exist, AC-1/AC-2), the explicit acceptance (the
// assignee's own act), the decision-ready context (the LIVE composition
// with the pins, the purchasing hidden by the honest word — AC-4), the
// «Подтвердить проверку» decision (the Human Verification Receipt behind
// the live fence — the digest pin of the exact version) with the
// MANDATORY manual-contour word (AC-10/AC-11) and the fence honesty word
// (AC-13); the a11y close (320px without the horizontal scroll) and the
// secrets probe.
test('order-verification: the manager task over the committed receipt from the assignment to the manual-contour word at 320px', async ({ browser }) => {
  test.setTimeout(360_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 0. The fixture switches of the vertical (the z43 precedents): the
    // preview mode, the ORDER world and the LIVE entitlement walk.
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '1' } });
    await api.post('/fixture/order-world', { form: { enabled: '1' } });
    const entitlement = await (await api.get(`${apiBase}/entitlement`)).json();
    if (entitlement.state === 'paused') {
      const resume = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'resume', basis: 'Возобновление для проверки заказа', confirm: true } });
      expect(resume.status()).toBeLessThan(300);
    } else if (entitlement.state !== 'active') {
      const terms = await api.post(`${apiBase}/entitlement/terms`, { data: {
        currency: 'RUB', tax_regime: 'УСН', period_start: '2026-10-01T00:00:00Z', period_end: '2026-12-31T23:59:59Z',
        included_employees: ['consultant-01'], included_channels: ['jivo'], included_connectors: ['abcp'],
        launch_price: 49000, subscription_price: 15000, change_note: 'Коммерческое предложение проверки заказа'
      } });
      expect(terms.status()).toBeLessThan(300);
      const activate = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'activate', basis: 'Договор №82 от 01.10.2026', confirm: true } });
      expect(activate.status()).toBeLessThan(300);
    }

    // 1..6. The committed order chain of the z43 family (the case, the
    // classification, the part request facts, the sales mode, the proven
    // fitment, the LIVE offer, the selection, the build, the
    // confirmation, the authorization, the dispatch with the committed
    // verdict): the receipt lands — the anchor of the verification task.
    const first = await postIngress(api, textInboundEvent('journey-offers-evt-v44', 'journey-offers-sess-v44', { say: 'Хочу заказать салонный фильтр WF2B196K666AA, нужно 2 штуки' }), 'journey-verify-key-1');
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
    await fitmentForm.locator('input[name="source_record_id"]').fill('rec-verify-fitment-1');
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
    await offerForm.locator('input[name="source_record_id"]').fill('rec-verify-1');
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
    await authDetails.locator('input[name="idempotency_key"]').fill('journey-verify-key-42');
    await authDetails.getByRole('button', { name: /Авторизовать Operation/ }).click();
    await expect(page).toHaveURL(/\?authorized=[0-9a-f-]+$/);

    await api.post('/fixture/order-dispatch-evidence', { form: { external_object_id: 'ABCP-ORDER-42', external_object_version: '3' } });
    await api.post('/fixture/order-dispatch-outcome', { form: { outcome: 'committed', correlation: 'journey-verify-corr-42', guarantee: '1' } });
    await page.goto(reviewPath);
    const drainDetails = page.locator('details', { hasText: 'Запустить дренаж очереди отправок' });
    await drainDetails.locator('summary').click();
    await drainDetails.locator('input[name="confirm_drain"]').check();
    await drainDetails.locator('button.primary', { name: 'Дренировать очередь' }).click();
    await expect(page).toHaveURL(/\?drained=[0-9]+$/);

    // 7. The committed receipt on the case page — the ANCHOR of the
    // verification task (the external ABCP Order ID of the readback).
    await page.goto(pathA);
    const receiptPanel = page.locator('section[aria-label="Квитанции заказа клиента"]');
    await expect(receiptPanel).toContainText('ABCP-ORDER-42');

    // 8. The verification section BEFORE the assignment: the honest
    // boundary words of the contour (the fence honesty AC-13 — the bypass
    // by hands in ABCP is possible, Staffy guarantees its own contour
    // only; the delivery separation AC-2). No task card exists yet.
    const verifySection = page.locator('#order-verification');
    await expect(page.getByRole('heading', { name: 'Проверка заказа' })).toBeVisible();
    await expect(verifySection).toContainText('обход проверки руками в ABCP возможен');
    await expect(verifySection).toContainText('автоматической отправки поставщику не существует');
    await expect(verifySection).toContainText('назначен ≠ доставлено ≠ принял');
    await expect(page.locator('details:has(summary:has-text("Задача №"))')).toHaveCount(0);

    // 9. The ASSIGNMENT through the page form (the owner's act — the
    // assignee and the backup by id, the deadline word): the PRG landing
    // speaks the assigned word. The task anchors the committed operation —
    // the operation id reads from the authorization view of the case.
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

    // 10. The assigned card: the state word «назначен, ещё не принял»
    // (AC-1 — the assignment is NOT the acceptance), the round material
    // (the assignee and the backup by id, the deadline word with its
    // text), the epoch and the technical line (the operation, the
    // idempotency key, the external ABCP Order ID, the 1C-unknown word).
    await page.goto(pathA);
    const taskCard = page.locator('details:has(summary:has-text("Задача №1"))').first();
    await expect(taskCard).toContainText('назначен, ещё не принял');
    await expect(taskCard).toContainText(ownerID);
    await expect(taskCard).toContainText(backupID);
    await expect(taskCard).toContainText('Проверка заказа в течение четырёх часов');
    await expect(taskCard).toContainText(operationID);
    await expect(taskCard).toContainText('ABCP-ORDER-42');
    await expect(taskCard).toContainText('проекция в 1С неизвестна');
    // The LIVE context of the decision: the position with the pins; the
    // purchasing price rides the Commercial scope of the acting principal
    // (the pilot owner carries it — the recorded-absence word of the
    // optional internal data; the hiding word belongs to the scope-less
    // actor, pinned by the wire tests).
    await expect(verifySection).toContainText('Mann / WF2B196K666AA');
    await expect(verifySection).toContainText('закупочная цена не записана');
    // The undelivered notification probe: the assigned word never claims
    // «уведомлён» (AC-2 — the delivery is a DIFFERENT truth).
    const cardText = await taskCard.innerText();
    expect(cardText).not.toContain('уведомлён');

    // 11. The ACCEPTANCE through the page form (the assignee's own act):
    // the PRG landing speaks the accepted word; the card moves to
    // «принял» with the TWO DIFFERENT decision forms (AC-16).
    await page.goto(pathA);
    await page.locator('form[action$="/order-verifications/1/acknowledgements"]').first().getByRole('button', { name: /Принять задачу проверки/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?verification_acknowledged=1`));
    await page.goto(pathA);
    const acceptedCard = page.locator('details:has(summary:has-text("Задача №1"))').first();
    await expect(acceptedCard).toContainText('принял: менеджер подтвердил владение проверкой');
    await expect(page.locator('form[action$="/order-verifications/1/approvals"]')).toHaveCount(1);
    await expect(page.locator('form[action$="/order-verifications/1/blocks"]')).toHaveCount(1);
    // The approve form carries the digest pin of the exact version and
    // the MANDATORY manual-contour word BEFORE the decision (the honest
    // pre-decision disclosure).
    const approveForm = page.locator('form[action$="/order-verifications/1/approvals"]').first();
    await expect(approveForm.locator('input[name="expected_order_digest"]')).toHaveValue(/^[0-9a-f]{64}$/);
    await expect(approveForm).toContainText('формирование и отправка поставщику — ручное действие менеджера');

    // 12. The «Подтвердить проверку» decision (the DIFFERENT action of
    // «Нужна коррекция» — AC-16): the PRG landing speaks the confirmed
    // word WITH the MANDATORY manual-contour word (AC-10/AC-11).
    await approveForm.getByRole('button', { name: /Подтвердить проверку/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?verification_approved=1`));
    // The PRG landing page carries the approved status flag WITH the
    // MANDATORY manual-contour word (AC-10/AC-11) — asserted BEFORE the
    // plain reload drops the query flag.
    await expect(page.locator('#verification-approved')).toContainText('Проверка подтверждена');
    await expect(page.locator('#verification-approved')).toContainText('формирование и отправка поставщику — ручное действие менеджера в контуре магазина');
    await expect(page.locator('#verification-approved')).toContainText('Staffy не отправляет автоматически');
    await page.goto(pathA);
    // The decided card: the Human Verification Receipt pins (the digest,
    // the evidence anchors) — the decision applied to the exact version.
    const decidedCard = page.locator('details:has(summary:has-text("Задача №1"))').first();
    await expect(decidedCard).toContainText('проверка подтверждена');
    await expect(decidedCard).toContainText(/пин версии [0-9a-f]{64}/);
    await expect(decidedCard).toContainText('position:WF2B196K666AA');
    await expect(decidedCard).toContainText('формирование и отправка поставщику — ручное действие менеджера');
    // The decided task carries NO decision forms anymore (the closed
    // lifecycle — the repeated check is the NEW task by the owner's
    // assignment).
    await expect(page.locator('form[action$="/order-verifications/1/approvals"]')).toHaveCount(0);
    await expect(page.locator('form[action$="/order-verifications/1/blocks"]')).toHaveCount(0);

    // 13. The a11y close: the verification section never widens the
    // layout (the tables scroll inside their wraps at 320px — the family
    // precedent of the z42/z43 close).
    const sectionOverflows = await page.evaluate(() => {
      const verify = document.getElementById('order-verification');
      return [verify].filter((el): el is HTMLElement => el !== null)
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
