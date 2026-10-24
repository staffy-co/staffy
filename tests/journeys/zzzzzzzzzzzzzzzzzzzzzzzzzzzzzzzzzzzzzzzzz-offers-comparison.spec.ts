import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const chatConnectionID = '01994f64-6c00-7000-8000-000000000016';
const casesPath = `/app/tenants/${tenantID}/cases`;

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

// The Story 4.5 chain at 320px: the offers and comparison panel of the
// case card (the read-time comparison over the append-only offer records
// of the 6.0.0 pack policy — the honest case-level unknown with the
// offers research recommendation, the price authority and the term
// estimate words), the manager's task through the existing request_human
// form with the 00024 reason word (the conditions research — the
// decision-ready task of the manager), the proven fitment of the
// position (the 4.4 contour the comparison READS), the LIVE offers of
// TWO suppliers through the PRG form (the original position with the
// whole traceability material), the budget ranking with the explanation
// words and the shortfall word of the partial availability, the voice
// block of the phone, the operator record section with the purchase
// price and its internal-data word (AC-2), the LIVE selection through
// the PRG form (the snapshot with the pins of ALL FOUR inputs and the
// known sum arithmetic with the boundary words), the ИСПРАВЛЕНИЕ of the
// request (the new version of the confirmed budget changes the
// PartRequestDigest — every dependent offer turns stale with the honest
// words, AC-10); the a11y close (320px without the horizontal scroll,
// the stale words as the alert TEXT) and the secrets probe.
test('offers: the manager research task, the live offers, the budget ranking, the selection snapshot and the stale expiry at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One TEXT turn of one web_chat session (the fixture world pins
    // auto-parts@6.0.0 — the comparison-policy-bearing pack of the case).
    const first = await postIngress(api, textInboundEvent('journey-offers-evt-1', 'journey-offers-sess-1', { say: 'Нужен салонный фильтр WF2B196K666AA, бюджет до 5000, нужно 6 штук' }), 'journey-offers-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const pathA = `${casesPath}/${caseA}`;

    // 2. The classification (the fact scope of the part request facts).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Классифицировать / переклассифицировать"))').first().click();
    const classifyForm = page.locator('form[action$="/classify"]').first();
    await classifyForm.locator('select[name="type"]').selectOption('поиск детали');
    await classifyForm.locator('input[name="title"]').fill('Салонный фильтр');
    await classifyForm.locator('input[name="reason"]').fill('Клиент ищет фильтр салона по номеру с бюджетом');
    await classifyForm.getByRole('button', { name: /Классифицировать/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?classified=1`));

    // 3. The operator records the part request material through the
    // штатная fact form: the kind, the OEM number, the budget and the
    // quantity (the confirmed criteria material of the ranking — AC-6).
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
    await recordFact('part.budget', '"до 5000"');
    await recordFact('part.quantity', '6');

    // 3b. The operator confirmations of the exact versions (the
    // confirmed criteria set carries the PartRequestDigest the offers
    // pin; a later new version of a confirmed fact changes the digest —
    // the AC-10 stale expiry of the dependent offers).
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
    await confirmFact('part.budget', 'Бюджет назван клиентом');
    await confirmFact('part.quantity', 'Количество названо клиентом');

    // 4. The offers panel of the absent records: the honest case-level
    // unknown word (AC-12 — the old price never repeats as the current
    // one), the offers research recommendation with the 00024 word, the
    // price authority word (AC-3) and the term estimate word (AC-8).
    await page.goto(pathA);
    let offersPanel = page.locator('section[aria-label="Предложения и сравнение"]');
    await expect(offersPanel).toBeVisible();
    await expect(offersPanel).toContainText('актуальные коммерческие условия неизвестны: предложений нет');
    await expect(offersPanel).toContainText('старая цена не повторяется как действующая');
    await expect(offersPanel).toContainText('offers_research');
    await expect(offersPanel).toContainText('Запрос актуальных условий');
    await expect(offersPanel).toContainText('цена по разрешению ABCP — рассчитана Owning System, НЕ моделью');
    await expect(offersPanel).toContainText('оценка поставщика — не обещание магазина');

    // 5. The manager's task through the existing request_human form: the
    // operator selects the 00024 research word (the recommendation of the
    // panel — the decision-ready task).
    const requestDetails = page.locator('details:has(summary:text-is("Запросить человека (request_human — безусловная просьба клиента)"))');
    await requestDetails.click();
    const requestForm = requestDetails.locator('form[action$="/human-requests"]');
    await requestForm.locator('select[name="turn_ref"]').selectOption({ index: 1 });
    await requestForm.locator('input[name="target_roles"]').first().check();
    await requestForm.locator('select[name="ack_deadline"]').selectOption('ack_15m');
    await requestForm.locator('select[name="reason"]').selectOption('offers_research');
    await requestForm.getByRole('button', { name: /Записать просьбу клиента/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?human_requested_recorded=1`));
    await page.goto(pathA);
    const handoffPanel = page.locator('section[aria-label="Запрос человека"]');
    await expect(handoffPanel).toContainText('Запрос актуальных условий');

    // 6. The proven fitment of the position through the 4.4 form (the
    // comparison READS the fitment output — the availability never
    // changes the fitment status, AC-1).
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
    await fitmentForm.locator('input[name="source_record_id"]').fill('rec-offers-fitment-1');
    await fitmentForm.locator('input[name="source_version"]').fill('v2026.10');
    await fitmentForm.locator('input[name="source_observed_at"]').fill('2026-09-16T11:00:00Z');
    await fitmentForm.getByRole('button', { name: /Записать доказательство/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?fitment_evidence_recorded=1`));

    // 7. The LIVE offers of the two suppliers through the PRG form: the
    // first supplier (Bosch, 4850, availability 4, the optional purchase
    // price 3900 of the internal data) and the second supplier (Mann,
    // 4200, the full availability 6, no purchase price — the honest
    // absence).
    const recordOffer = async (supplier: string, name: string, brand: string, price: string, purchase: string, quantity: string, term: string, record: string) => {
      await page.goto(pathA);
      const offerForm = page.locator(`form[action$="/work-items/${workItemA}/supplier-offers"]`).first();
      await offerForm.locator('input[name="position_key"]').fill('WF2B196K666AA');
      await offerForm.locator('input[name="supplier_id"]').fill(supplier);
      await offerForm.locator('input[name="supplier_name"]').fill(name);
      await offerForm.locator('input[name="brand"]').fill(brand);
      await offerForm.locator('input[name="article"]').fill('WF2B196K666AA');
      await offerForm.locator('input[name="client_price"]').fill(price);
      await offerForm.locator('input[name="purchase_price"]').fill(purchase);
      await offerForm.locator('select[name="currency"]').selectOption('RUB');
      await offerForm.locator('input[name="quantity_available"]').fill(quantity);
      await offerForm.locator('input[name="term_days"]').fill(term);
      await offerForm.locator('input[name="conditions"]').fill('самовывоз со склада');
      await offerForm.locator('select[name="source_system"]').selectOption('abcp_catalog');
      await offerForm.locator('input[name="source_record_id"]').fill(record);
      await offerForm.locator('input[name="source_version"]').fill('v2026.10');
      await offerForm.locator('input[name="source_observed_at"]').fill('2026-09-16T11:30:00Z');
      await offerForm.getByRole('button', { name: /Записать предложение/ }).click();
      await expect(page).toHaveURL(new RegExp(`${caseA}\\?supplier_offer_recorded=1`));
    };
    await recordOffer('SUP-1', 'Поставщик один', 'Bosch', '4850', '3900', '4', '2', 'rec-offers-1');
    await recordOffer('SUP-2', 'Поставщик два', 'Mann', '4200', '', '6', '1', 'rec-offers-2');

    // 8. The comparison panel: the current position with the budget
    // ranking (the confirmed criterion — AC-6): the cheaper variant of
    // the rank 1 with the explanation word, the shortfall word of the
    // partial availability (4 of 6 — AC-11), the voice block (AC-14) and
    // the operator record section with the purchase price and the
    // internal-data word (AC-2).
    await page.goto(pathA);
    offersPanel = page.locator('section[aria-label="Предложения и сравнение"]');
    await expect(offersPanel).toContainText('актуальные предложения записаны');
    await expect(offersPanel).toContainText('оригинал');
    await expect(offersPanel).toContainText('подтверждённо подходит');
    await expect(offersPanel).toContainText('вариант соответствует указанному приоритету: цена в пределах бюджета');
    await expect(offersPanel).toContainText('у этого поставщика меньше запрошенного: 4 из 6');
    await expect(offersPanel).toContainText('Слово для телефона');
    await expect(offersPanel).toContainText('цена: от 4200 до 4850 RUB');
    await expect(offersPanel).toContainText('рекомендуемый вариант: Mann');
    await expect(offersPanel).toContainText('полное сравнение отправим в разрешённый текстовый либо защищённый канал');
    await expect(offersPanel).toContainText('внутренние данные: клиенту не раскрывается');
    await expect(offersPanel).toContainText('3900');
    await expect(offersPanel).toContainText('оценка поставщика — не обещание магазина');

    // 9. The LIVE selection through the PRG form: the chosen CURRENT offer
    // of the second supplier with the quantity 2 — the snapshot pins ALL
    // FOUR inputs (AC-13), the known sum 2 × 4200 = 8400 with the
    // boundary words (AC-4) and the order boundary (the fence OQ-3/OQ-4).
    await page.goto(pathA);
    const selectionForm = page.locator(`form[action$="/work-items/${workItemA}/selection-snapshots"]`).first();
    await selectionForm.locator('input[name="position_key"]').fill('WF2B196K666AA');
    await selectionForm.locator('input[name="supplier_id"]').fill('SUP-2');
    await selectionForm.locator('input[name="offer_no"]').fill('1');
    await selectionForm.locator('input[name="quantity"]').fill('2');
    await selectionForm.getByRole('button', { name: /Записать выбор/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?client_selection_recorded=1`));
    await page.goto(pathA);
    offersPanel = page.locator('section[aria-label="Предложения и сравнение"]');
    await expect(offersPanel).toContainText('Снапшот №1');
    await expect(offersPanel).toContainText('2 × 4200');
    await expect(offersPanel).toContainText('8400');
    await expect(offersPanel).toContainText('без доставки и сбора — окончательная стоимость не рассчитана');
    await expect(offersPanel).toContainText('ABCP Order не создаётся; поставщику не отправляется');

    // 10. The ИСПРАВЛЕНИЕ of the request (AC-10): the operator records a
    // NEW version of the confirmed budget — the PartRequestDigest
    // changes, every dependent offer turns stale with the honest words
    // (the request stale word).
    await recordFact('part.budget', '"до 6000"');
    await page.goto(pathA);
    offersPanel = page.locator('section[aria-label="Предложения и сравнение"]');
    await expect(offersPanel).toContainText('предложение относилось к прежней версии запроса');
    await expect(offersPanel).toContainText('актуальные коммерческие условия неизвестны');

    // 11. The a11y close of the panel: the offers SECTION never widens
    // the layout (the tables scroll inside their wraps), the stale words
    // and the boundaries are the alert TEXT (never only color — AC-16).
    await expect(page.locator('#offers')).toBeVisible();
    const sectionOverflow = await page.evaluate(() => {
      const section = document.getElementById('offers')!;
      return section.scrollWidth - section.clientWidth;
    });
    expect(sectionOverflow).toBeLessThanOrEqual(0);
    // The secrets never render (the operator page carries no credentials).
    const body = await page.locator('body').innerText();
    expect(body).not.toContain('api_key');
    expect(body).not.toContain('sandbox-');
  } finally {
    await page.close();
  }
});
