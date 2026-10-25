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

// The Story 4.8 chain at 320px: the order correction — the Correction
// Work Item over the LAST CLOSED blocked decision of the manager
// verification (4.7). The journey walks the blocked decision (the
// «Нужна коррекция» verdict with the positions and the reason word),
// the correction section BEFORE the open (the honest boundary words —
// the manual contour AG-4, the two structural gates), the OPEN through
// the page form (the carried blocked material BY NAME — the SQL-truth
// carry), the recommended explanation (the new-confirmation word, the
// «прежде не отправлялся» boundary, the honest channel word), the
// surgical pending words of the verification/order panels (the gates
// hold while the correction lives), the assign-gate probe (the honest
// flash word), the recompose and the customer confirmation of the new
// version (the EDIT ladder of the SAME preview thread), the «Записать
// перечит» closure with the NEW confirmed version pins (the SAFE pin
// set — the manager's words, AG-4), the corrected closure with the
// return-to-verification word and the NEW verification task (the ladder
// verification_no=2); the a11y close (320px without the horizontal
// scroll) and the secrets probe.
test('order-correction: the Correction Work Item from the blocked decision to the reread closure at 320px', async ({ browser }) => {
  test.setTimeout(360_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 0. The fixture switches of the vertical (the z44 precedents): the
    // preview mode, the ORDER world and the LIVE entitlement walk.
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '1' } });
    await api.post('/fixture/order-world', { form: { enabled: '1' } });
    const entitlement = await (await api.get(`${apiBase}/entitlement`)).json();
    if (entitlement.state === 'paused') {
      const resume = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'resume', basis: 'Возобновление для коррекции заказа', confirm: true } });
      expect(resume.status()).toBeLessThan(300);
    } else if (entitlement.state !== 'active') {
      const terms = await api.post(`${apiBase}/entitlement/terms`, { data: {
        currency: 'RUB', tax_regime: 'УСН', period_start: '2026-10-01T00:00:00Z', period_end: '2026-12-31T23:59:59Z',
        included_employees: ['consultant-01'], included_channels: ['jivo'], included_connectors: ['abcp'],
        launch_price: 49000, subscription_price: 15000, change_note: 'Коммерческое предложение коррекции заказа'
      } });
      expect(terms.status()).toBeLessThan(300);
      const activate = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'activate', basis: 'Договор №83 от 01.10.2026', confirm: true } });
      expect(activate.status()).toBeLessThan(300);
    }

    // 1..6. The committed order chain of the z43/z44 family (the case,
    // the classification, the part request facts, the sales mode, the
    // proven fitment, the LIVE offer, the selection, the build, the
    // confirmation, the authorization, the dispatch with the committed
    // verdict): the receipt lands — the anchor of the verification task
    // and its blocked decision.
    const first = await postIngress(api, textInboundEvent('journey-offers-evt-v45', 'journey-offers-sess-v45', { say: 'Хочу заказать салонный фильтр WF2B196K666AA, нужно 2 штуки' }), 'journey-correct-key-1');
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
    await fitmentForm.locator('input[name="source_record_id"]').fill('rec-correct-fitment-1');
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
    await offerForm.locator('input[name="source_record_id"]').fill('rec-correct-1');
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
    await authDetails.locator('input[name="idempotency_key"]').fill('journey-correct-key-42');
    await authDetails.getByRole('button', { name: /Авторизовать Operation/ }).click();
    await expect(page).toHaveURL(/\?authorized=[0-9a-f-]+$/);

    await api.post('/fixture/order-dispatch-evidence', { form: { external_object_id: 'ABCP-ORDER-42', external_object_version: '3' } });
    await api.post('/fixture/order-dispatch-outcome', { form: { outcome: 'committed', correlation: 'journey-correct-corr-42', guarantee: '1' } });
    await page.goto(reviewPath);
    const drainDetails = page.locator('details', { hasText: 'Запустить дренаж очереди отправок' });
    await drainDetails.locator('summary').click();
    await drainDetails.locator('input[name="confirm_drain"]').check();
    await drainDetails.locator('button.primary', { name: 'Дренировать очередь' }).click();
    await expect(page).toHaveURL(/\?drained=[0-9]+$/);

    // 7. The committed receipt on the case page — the ANCHOR of the
    // verification task and its blocked decision.
    await page.goto(pathA);
    const receiptPanel = page.locator('section[aria-label="Квитанции заказа клиента"]');
    await expect(receiptPanel).toContainText('ABCP-ORDER-42');

    // 8. The verification task and the BLOCKED decision («Нужна
    // коррекция» — the DIFFERENT action, AC-16 of 4-7): the assignment,
    // the acceptance, then the correction decision with the position and
    // the reason word — the blocked word of the launch.
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
    const blockForm = page.locator('form[action$="/order-verifications/1/blocks"]').first();
    await blockForm.locator('input[name="positions"]').first().check();
    await blockForm.locator('select[name="reason"]').selectOption('wrong_part');
    await blockForm.getByRole('button', { name: /Нужна коррекция/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?verification_blocked=1`));
    // The blocked task card carries the correction launch word (the state text of the blocked decision).
    await expect(page.locator('details:has(summary:has-text("Задача №1"))').first()).toContainText('запущена коррекция — контур 4-8');

    // 9. The correction section BEFORE the open: the honest boundary
    // words (the manual contour AG-4 — the ABCP change is the manager's
    // manual act; the two structural gates alert). No correction card
    // exists yet — the honest no-present word.
    await page.goto(pathA);
    const correctSection = page.locator('#order-correction');
    await expect(page.getByRole('heading', { name: 'Коррекция заказа' })).toBeVisible();
    await expect(correctSection).toContainText('изменение существующего заказа ABCP выполняется вручную в контуре магазина');
    await expect(correctSection).toContainText('новый дублирующий заказ не создаётся');
    await expect(correctSection).toContainText('новая задача проверки не создаётся до закрытия коррекции');
    await expect(correctSection).toContainText('заблокированного решения нет, открывать нечего');
    await expect(page.locator('details:has(summary:has-text("Коррекция №"))')).toHaveCount(0);

    // 10. The OPEN through the page form (the operation anchor, the
    // owner, the deadline word): the PRG landing speaks the opened word.
    const openForm = page.locator('form[action$="/order-corrections"]').first();
    await openForm.locator('input[name="operation_id"]').fill(operationID);
    await openForm.locator('input[name="owner"]').fill(ownerID);
    await openForm.locator('select[name="deadline"]').selectOption('correct_4h');
    await openForm.getByRole('button', { name: /Открыть коррекцию/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?correction_opened=1`));
    await expect(page.locator('#correction-opened')).toContainText('заказ заблокирован от продвижения до исправления и новой проверки');

    // 11. The opened card: the state word, the owner by id, the deadline
    // word with its text, the epoch, the CARRIED blocked material (the
    // position and the reason word BY NAME — the SQL-truth carry), the
    // technical line (the operation, the blocked verification №1, the
    // original external ABCP Order ID) and the «назначение ≠ начало
    // работы» boundary word.
    await page.goto(pathA);
    const correctCard = page.locator('details:has(summary:has-text("Коррекция №1"))').first();
    await expect(correctCard).toContainText('коррекция открыта: заказ заблокирован от продвижения');
    await expect(correctCard).toContainText(ownerID);
    await expect(correctCard).toContainText('Коррекция подбора в течение четырёх часов');
    await expect(correctCard).toContainText('WF2B196K666AA');
    await expect(correctCard).toContainText('Неподходящая деталь');
    await expect(correctCard).toContainText(operationID);
    await expect(correctCard).toContainText('ABCP-ORDER-42');
    await expect(correctSection).toContainText('назначение коррекции ≠ начало работы');

    // 12. The recommended explanation: the boundary words (the former
    // variant was NEVER sent — AC-6; the new confirmation word — AC-7;
    // the honest channel word — AC-5); the explicit diff table renders
    // the no-difference word (the live composition still equals the
    // confirmed version — the recompose has not happened yet).
    await expect(correctSection).toContainText('прежний вариант поставщику не отправлялся');
    await expect(correctSection).toContainText('требуется новое подтверждение клиента: прежнее подтверждение не переносится');
    await expect(correctSection).toContainText('канал связи с клиентом — ручной контур магазина');
    await expect(correctSection).toContainText('Явных различий состава нет');

    // 13. The SURGICAL pending words of the verification/order panels:
    // both panels carry the open-correction state word (the gates hold
    // while the correction lives).
    const verifySection = page.locator('#order-verification');
    await expect(verifySection).toContainText('новая задача проверки вернётся после исправления');
    const orderPanel = page.locator('#order').or(page.locator('section[aria-label="Заказ клиента"]'));
    await expect(orderPanel.first()).toContainText('новый заказ не создаётся: изменение существующего заказа выполняется вручную');

    // 14. The ASSIGN-GATE probe (AC-2): the assign form of a new
    // verification task while the correction is open — the honest flash
    // word of the gate (the task is NEVER created).
    await assignForm.locator('input[name="work_item_id"]').fill(workItemA);
    await assignForm.locator('input[name="operation_id"]').fill(operationID);
    await assignForm.locator('input[name="assignee"]').fill(ownerID);
    await assignForm.locator('input[name="backup_owner"]').fill(backupID);
    await assignForm.locator('select[name="deadline"]').selectOption('verify_4h');
    await assignForm.getByRole('button', { name: /Назначить задачу проверки/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}$`));
    await expect(page.locator('#case-flash')).toContainText('коррекция заказа открыта — проверка вернётся после исправления');

    // 15. The RECOMPOSE and the customer confirmation of the new
    // version: the builder re-runs over the same case (the EDIT ladder
    // lands the preview version+1 of the SAME thread); the customer
    // confirms the new version (the confirmation thread version+1 — the
    // old confirmation becomes superseded).
    await page.goto(pathA);
    await buildForm.getByRole('button', { name: /Пересобрать заказ из живого выбора|Собрать заказ из живого выбора/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?order_proposal_built=1`));
    await page.goto(reviewPath);
    const rebuildConfirm = page.locator('form[action$="/operations/confirmations"]').first();
    const confirmationsBefore = await (await api.get(`${apiBase}/cases/${caseA}/operations`)).json();
    const orderConfirmation = confirmationsBefore.authorizations.confirmations.find((c: any) => c.preview_id === operation.preview_id && c.preview_version_no === 1);
    const previewID = orderConfirmation.preview_id;
    await rebuildConfirm.locator('input[name="preview_id"]').fill(previewID);
    await rebuildConfirm.locator('input[name="version_no"]').fill('2');
    await rebuildConfirm.locator('input[name="lifetime_seconds"]').fill('600');
    await rebuildConfirm.getByRole('button', { name: /Подтвердить версию preview/ }).click();
    await expect(page).toHaveURL(/\/operations\?confirmed=[0-9]+$/);

    // 16. The new confirmation pins (the reread command material): the
    // confirmation card of the preview version 2.
    const confirmationsAfter = await (await api.get(`${apiBase}/cases/${caseA}/operations`)).json();
    const newConfirmation = confirmationsAfter.authorizations.confirmations.find((c: any) => c.preview_id === previewID && c.preview_version_no === 2);
    expect(newConfirmation).toBeTruthy();
    expect(newConfirmation.state).toBe('valid');
    const newConfirmationID = newConfirmation.confirmation_id;

    // 17. The «Записать перечит» closure through the page form (the
    // DIFFERENT action of «Прекратить коррекцию» — AC-16): the external
    // object ID of the reread order and the pinned NEW confirmed version
    // triple {proposal, version, confirmation} — the SAFE pin set only.
    await page.goto(pathA);
    const rereadForm = page.locator('form[action$="/order-corrections/1/rereads"]').first();
    await rereadForm.locator('input[name="external_object_id"]').fill('ABCP-ORDER-42-EDITED');
    await rereadForm.locator('input[name="proposal_id"]').fill(previewID);
    await rereadForm.locator('input[name="version_no"]').fill('2');
    await rereadForm.locator('input[name="confirmation_id"]').fill(newConfirmationID);
    await rereadForm.getByRole('button', { name: /Записать перечит/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?correction_reread=1`));
    // The PRG landing page carries the reread status flag WITH the
    // MANDATORY reread word (AG-4) — asserted BEFORE the plain reload
    // drops the query flag.
    await expect(page.locator('#correction-reread')).toContainText('Перечит записан');
    await expect(page.locator('#correction-reread')).toContainText('перечит подтверждён словами менеджера: Staffy не читает ABCP');

    // 18. The corrected card: the closure material (the SAFE pins of the
    // reread — the external object ID and the identity triple), the
    // corrected state word with the return to the manager verification
    // (pending_manager_verification — the NEW task by the owner's
    // decision) and NO decision forms anymore (the closed lifecycle).
    await page.goto(pathA);
    const correctedCard = page.locator('details:has(summary:has-text("Коррекция №1"))').first();
    await expect(correctedCard).toContainText('исправлен: заказ возвращается на проверку менеджера (pending_manager_verification)');
    await expect(correctedCard).toContainText('ABCP-ORDER-42-EDITED');
    await expect(correctedCard).toContainText(previewID);
    await expect(correctedCard).toContainText(newConfirmationID);
    await expect(page.locator('form[action$="/order-corrections/1/rereads"]')).toHaveCount(0);
    await expect(page.locator('form[action$="/order-corrections/1/cessations"]')).toHaveCount(0);
    // The surgical pending words are GONE (the gates no longer hold).
    await expect(verifySection).not.toContainText('новая задача проверки вернётся после исправления');

    // 19. The NEW verification task after the closure (the ladder
    // verification_no=2 — the assign gate no longer holds): the manager
    // assigns the repeated FULL check.
    await page.locator('form[action$="/order-verifications"]').first().locator('input[name="work_item_id"]').fill(workItemA);
    await page.locator('form[action$="/order-verifications"]').first().locator('input[name="operation_id"]').fill(operationID);
    await page.locator('form[action$="/order-verifications"]').first().locator('input[name="assignee"]').fill(ownerID);
    await page.locator('form[action$="/order-verifications"]').first().locator('input[name="backup_owner"]').fill(backupID);
    await page.locator('form[action$="/order-verifications"]').first().getByRole('button', { name: /Назначить задачу проверки/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?verification_assigned=1`));
    await page.goto(pathA);
    await expect(page.locator('details:has(summary:has-text("Задача №2"))')).toContainText('назначен, ещё не принял');

    // 20. The a11y close: the correction section never widens the layout
    // (the tables scroll inside their wraps at 320px — the family
    // precedent of the z43/z44 close).
    const sectionOverflows = await page.evaluate(() => {
      const correct = document.getElementById('order-correction');
      return [correct].filter((el): el is HTMLElement => el !== null)
        .map((el) => el.scrollWidth - el.clientWidth);
    });
    for (const overflow of sectionOverflows) {
      expect(overflow).toBeLessThanOrEqual(0);
    }

    // 21. The secrets never render (the manager page carries no
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
