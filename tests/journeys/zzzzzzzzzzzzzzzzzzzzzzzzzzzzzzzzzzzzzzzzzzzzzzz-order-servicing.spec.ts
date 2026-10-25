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

// The Story 4.10 chain at 320px: the safe servicing of the EXISTING
// order — the repeat-contact request recorded by the operator (the
// append-only event with the LIVE proof pins of the contact case), the
// Identity Proof Gate over the 2-3 assurance derivation (the disclosure
// — including the very existence of the order — only at the LIVE
// verified level; the Caller ID and the order-number knowledge are
// NEVER a proof), the lookup BY KEY over the recorded material (the
// receipts ∪ the rereads ∪ the facts — AG-4/OQ-18), the trust ladder
// (the same command after the verification lands as a NEW request —
// both rows live), the LIVE client words of the allowed disclosure
// (the projection words of 4-9 + the intent boundary word) and the
// cancel feasibility in its FOUR states after the sent fact («затронул
// поставщика — передаётся человеку», AG-4/OQ-3). The a11y close and the
// secrets probe end the journey.
test('order-servicing: the repeat contact from the gate word to the four cancel states at 320px', async ({ browser }) => {
  test.setTimeout(360_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 0. The fixture switches of the vertical (the z46 precedent): the
    // preview mode, the ORDER world and the LIVE entitlement walk.
    await api.post('/fixture/actions', { form: { mode: 'preview', enabled: '1' } });
    await api.post('/fixture/order-world', { form: { enabled: '1' } });
    const entitlement = await (await api.get(`${apiBase}/entitlement`)).json();
    if (entitlement.state === 'paused') {
      const resume = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'resume', basis: 'Возобновление для обслуживания заказа', confirm: true } });
      expect(resume.status()).toBeLessThan(300);
    } else if (entitlement.state !== 'active') {
      const terms = await api.post(`${apiBase}/entitlement/terms`, { data: {
        currency: 'RUB', tax_regime: 'УСН', period_start: '2026-10-01T00:00:00Z', period_end: '2026-12-31T23:59:59Z',
        included_employees: ['consultant-01'], included_channels: ['jivo'], included_connectors: ['abcp'],
        launch_price: 49000, subscription_price: 15000, change_note: 'Коммерческое предложение обслуживания заказа'
      } });
      expect(terms.status()).toBeLessThan(300);
      const activate = await api.post(`${apiBase}/entitlement/state`, { data: { action: 'activate', basis: 'Договор №85 от 01.10.2026', confirm: true } });
      expect(activate.status()).toBeLessThan(300);
    }

    // 1..6. The committed order chain of the z46 family (the case, the
    // classification, the part request facts, the sales mode, the proven
    // fitment, the LIVE offer, the selection, the build, the
    // confirmation, the authorization, the dispatch with the committed
    // verdict): the receipt lands — the anchor of the verification task
    // and its approved decision.
    const first = await postIngress(api, textInboundEvent('journey-offers-evt-v47', 'journey-offers-sess-v47', { say: 'Повторно обращаюсь по заказу салонного фильтра WF2B196K666AA' }), 'journey-servicing-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const pathA = `${casesPath}/${caseA}`;

    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Классифицировать / переклассифицировать"))').first().click();
    const classifyForm = page.locator('form[action$="/classify"]').first();
    await classifyForm.locator('select[name="type"]').selectOption('поиск детали');
    await classifyForm.locator('input[name="title"]').fill('Салонный фильтр');
    await classifyForm.locator('input[name="reason"]').fill('Клиент повторно обращается по заказу фильтра');
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
    await proposeForm.locator('input[name="reason"]').fill('Клиент повторно обращается по заказу детали');
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
    await fitmentForm.locator('input[name="source_record_id"]').fill('rec-servicing-fitment-1');
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
    await offerForm.locator('input[name="source_record_id"]').fill('rec-servicing-1');
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
    await authDetails.locator('input[name="idempotency_key"]').fill('journey-servicing-key-42');
    await authDetails.getByRole('button', { name: /Авторизовать Operation/ }).click();
    await expect(page).toHaveURL(/\?authorized=[0-9a-f-]+$/);

    await api.post('/fixture/order-dispatch-evidence', { form: { external_object_id: 'ABCP-ORDER-47', external_object_version: '3' } });
    await api.post('/fixture/order-dispatch-outcome', { form: { outcome: 'committed', correlation: 'journey-servicing-corr-47', guarantee: '1' } });
    await page.goto(reviewPath);
    const drainDetails = page.locator('details', { hasText: 'Запустить дренаж очереди отправок' });
    await drainDetails.locator('summary').click();
    await drainDetails.locator('input[name="confirm_drain"]').check();
    await drainDetails.locator('button.primary', { name: 'Дренировать очередь' }).click();
    await expect(page).toHaveURL(/\?drained=[0-9]+$/);

    // 7. The verification task and the APPROVED decision: the assignment,
    // the acceptance, then the «Подтвердить проверку» decision.
    await page.goto(pathA);
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

    // 8. The supplier facts of the order (the human receipts of 4-9): the
    // formed and the sent records of the SAME supplier order ID — the
    // material of the cancel fourth state.
    await page.goto(pathA);
    const factForm = page.locator('form[action$="/order-supplier-facts"]').first();
    await factForm.locator('select[name="kind"]').selectOption('formed');
    await factForm.locator('input[name="supplier_order_id"]').fill('SUP-47');
    await factForm.locator('input[name="happened_at"]').fill('2026-10-25T10:00');
    await factForm.getByRole('button', { name: /Записать факт/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?supplier_fact_formed=1`));

    await page.goto(pathA);
    const sentForm = page.locator('form[action$="/order-supplier-facts"]').first();
    await sentForm.locator('select[name="kind"]').selectOption('sent');
    await sentForm.locator('input[name="supplier_order_id"]').fill('SUP-47');
    await sentForm.locator('input[name="happened_at"]').fill('2026-10-25T10:30');
    await sentForm.getByRole('button', { name: /Записать факт/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?supplier_fact_sent=1`));

    // 9. The SERVICING panel BEFORE the identity check: the repeat-contact
    // request of the client that named the ABCP order number records (the
    // append-only event — AC-6), the proof pins carry the honest none,
    // the disclosure stays denied and the client words carry ONLY the
    // gate word with the live methods and the fallback words of 2-3 (the
    // lookup outcome NEVER rides them — the very existence of the order
    // is not disclosed, AC-1).
    await page.goto(pathA);
    const servicingSection = page.locator('#order-servicing');
    await expect(page.getByRole('heading', { name: 'Обслуживание заказа' })).toBeVisible();
    await expect(servicingSection).toContainText('контур 4-10');
    await expect(servicingSection).toContainText('Caller ID, имя профиля чата и знание номера заказа — не доказательство');
    await expect(servicingSection).toContainText('поиск по телефону или email не существует');
    await expect(servicingSection).toContainText('сквозной профиль по кейсам не строится (OQ-16)');
    await expect(servicingSection).toContainText('AG-4/OQ-18');
    await expect(servicingSection).toContainText('уведомления клиенту не отправляются');
    const requestForm = page.locator('form[action$="/order-servicing-requests"]').first();
    await requestForm.locator('select[name="intent"]').selectOption('order_status');
    await requestForm.locator('select[name="identifier_kind"]').selectOption('abcp_client_order_id');
    await requestForm.locator('input[name="identifier_value"]').fill('ABCP-ORDER-47');
    await requestForm.getByRole('button', { name: /Записать обращение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?servicing_request_recorded=1`));
    await expect(servicingSection).toContainText('Обращение по заказу записано (append-only)');

    await page.goto(pathA);
    await expect(servicingSection).toContainText('ABCP-ORDER-47');
    await expect(servicingSection).toContainText('none (нет записи проверки)');
    await expect(servicingSection).toContainText('раскрытие закрыто до проверки личности');
    // The denied client words: the gate word with the LIVE methods of the
    // channel (the codes the 2-3 panel renders) + the fallback words —
    // NEVER the lookup outcome or the order material (AC-1).
    await expect(servicingSection).toContainText('раскрытие требует проверки личности (уровень verified)');
    await expect(servicingSection).toContainText('Повторите проверку разрешённым способом.');
    // The AC-1 leak probe scopes to the CLIENT WORDS block of the denied
    // request (the operator row legitimately carries the outcome — the
    // review right of the operator; the CLIENT words never do).
    const deniedWords = await servicingSection
      .locator('h3:has-text("Клиентские слова обращения №1") ~ p.status').allInnerTexts();
    const deniedJoined = deniedWords.join(' ');
    expect(deniedJoined).toContain('раскрытие требует проверки личности');
    expect(deniedJoined).not.toContain('найден единственный заказ');
    expect(deniedJoined).not.toContain('заказ клиента создан');

    // 10. The IDENTITY VERIFICATION (the 2-3 form of the same page — the
    // equal text alternative): the callback passes — the assurance panel
    // shows verified with the method and the expiry.
    await page.locator('details:has(summary:text-is("Записать проверку (канонический исход, срок по методу)"))').click();
    const verifyForm = page.locator('form[action$="/verifications"]').first();
    const verifyConversation = await verifyForm.locator('select[name="conversation_id"] option').first().getAttribute('value');
    await verifyForm.locator('select[name="conversation_id"]').selectOption(verifyConversation);
    await verifyForm.locator('select[name="kind"]').selectOption('identity');
    await verifyForm.locator('select[name="method"]').selectOption('known_address_reply');
    await verifyForm.locator('select[name="provider_code"]').selectOption('ok');
    await verifyForm.locator('input[name="subject_handle"]').fill('позвонивший');
    await verifyForm.locator('input[name="note"]').fill('Клиент ответил письмом с известного адреса, подтвердил номер заказа');
    await verifyForm.locator('select[name="evidence_event_id"]').selectOption('');
    await verifyForm.getByRole('button', { name: /Записать проверку/ }).click();
    await expect(page).toHaveURL(new RegExp('verification_recorded=1'));

    // 11. The TRUST LADDER: the SAME request command AFTER the
    // verification lands as a NEW request (the changed proof state is a
    // DIFFERENT key — both rows live, the ladder stays visible); the
    // second row opens the disclosure and the client words carry the
    // LIVE projection words + the intent boundary word (AC-5).
    await page.goto(pathA);
    const repeatForm = page.locator('form[action$="/order-servicing-requests"]').first();
    await repeatForm.locator('select[name="intent"]').selectOption('order_status');
    await repeatForm.locator('select[name="identifier_kind"]').selectOption('abcp_client_order_id');
    await repeatForm.locator('input[name="identifier_value"]').fill('ABCP-ORDER-47');
    await repeatForm.getByRole('button', { name: /Записать обращение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?servicing_request_recorded=1`));
    await expect(servicingSection).toContainText('Обращение по заказу записано (append-only)');

    await page.goto(pathA);
    // Both rows live: the ladder of the trust stays visible.
    await expect(servicingSection).toContainText('№1');
    await expect(servicingSection).toContainText('№2');
    await expect(servicingSection).toContainText('личность подтверждена (verified)');
    await expect(servicingSection).toContainText('раскрытие разрешено (уровень verified на момент записи)');
    await expect(servicingSection).toContainText('unique_match — найден единственный заказ по названному признаку');
    await expect(servicingSection).toContainText('system_receipt (системная квитанция диспетчерского контура)');
    // The client words of the ALLOWED disclosure: the live projection
    // words of the resolved operation + the intent boundary word.
    await expect(servicingSection).toContainText('заказ поставщику отправлен (все записанные формирования покрыты записями отправки)');
    await expect(servicingSection).toContainText('только подтверждённые сведения');

    // 12. The CANCEL after the sent fact: the request of the cancel intent
    // by the SUPPLIER order number — the lookup BY KEY over the recorded
    // facts; the four states ride the words: the recorded request, the
    // absent prepared action (OQ-3 + deferred №215), the absent
    // confirmation (never promised) and the FOURTH state — «затронул
    // поставщика — передаётся человеку» (AG-4).
    await page.goto(pathA);
    const cancelForm = page.locator('form[action$="/order-servicing-requests"]').first();
    await cancelForm.locator('select[name="intent"]').selectOption('cancel');
    await cancelForm.locator('select[name="identifier_kind"]').selectOption('supplier_order_id');
    await cancelForm.locator('input[name="identifier_value"]').fill('SUP-47');
    await cancelForm.getByRole('button', { name: /Записать обращение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?servicing_request_recorded=1`));
    await expect(servicingSection).toContainText('Обращение по заказу записано (append-only)');

    await page.goto(pathA);
    await expect(servicingSection).toContainText('Отмена обращения №3: четыре состояния');
    await expect(servicingSection).toContainText('запрос на отмену записан (append-only)');
    await expect(servicingSection).toContainText('cancel отсутствует в каталоге действий (OQ-3');
    await expect(servicingSection).toContainText('deferred-work №215');
    await expect(servicingSection).toContainText('подтверждённая отмена не существует');
    await expect(servicingSection).toContainText('отмена не обещается');
    await expect(servicingSection).toContainText('заказ затронул поставщика (записи формирования/отправки есть)');
    await expect(servicingSection).toContainText('отмену выполняет менеджер вручную (AG-4)');
    await expect(servicingSection).toContainText('до человеческого подтверждения заказ НЕ считается отменённым');

    // 13. The stable word-links of the human contours (AC-16): «Позвать
    // человека» routes to the handoff section of 2-13; «Вернуться в
    // разговор» routes to the conversation mode surface.
    await expect(servicingSection.getByRole('link', { name: 'Позвать человека' })).toHaveAttribute('href', '#handoff');
    await expect(servicingSection.getByRole('link', { name: 'Вернуться в разговор' })).toHaveAttribute('href', '#mode-active');

    // 14. The a11y close: the servicing section never widens the layout
    // (the tables scroll inside their wraps at 320px — the family
    // precedent of the z43/z44/z45/z46 close).
    const sectionOverflows = await page.evaluate(() => {
      const section = document.getElementById('order-servicing');
      return [section].filter((el): el is HTMLElement => el !== null)
        .map((el) => el.scrollWidth - el.clientWidth);
    });
    for (const overflow of sectionOverflows) {
      expect(overflow).toBeLessThanOrEqual(0);
    }

    // 15. The secrets never render (the manager page carries no
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
