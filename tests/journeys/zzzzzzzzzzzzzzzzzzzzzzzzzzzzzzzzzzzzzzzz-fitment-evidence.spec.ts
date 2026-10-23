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

// The Story 4.4 chain at 320px: the applicability panel of the case card
// (the read-time state ladder over the append-only evidence records of
// the 5.0.0 pack policy — the honest case-level unknown with the
// research recommendation, the constant number-match word, the price and
// the scope words), the manager's task through the existing request_human
// form with the 00021 reason word (the research recommendation — the
// decision-ready task of the manager), the LIVE record through the PRG
// form (the found original proof with the whole traceability material —
// the source, the record id, the version, the time), the proven state
// with the FitmentDigest and the source details, the ИСПРАВЛЕНИЕ of the
// request (the new version of the confirmed part kind changes the
// PartRequestDigest — every dependent proof turns stale with the honest
// words, AC-11), the fresh record restoring proven; the a11y close
// (320px without the horizontal scroll, the stale words as the alert
// TEXT) and the secrets probe.
test('fitment: the manager research task, the live record, the proven state and the stale expiry at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One TEXT turn of one web_chat session (the fixture world pins
    // auto-parts@5.0.0 — the fitment-policy-bearing pack of the case).
    const first = await postIngress(api, textInboundEvent('journey-fitment-evt-1', 'journey-fitment-sess-1', { say: 'Нужен салонный фильтр, номер WF2B196K666AA' }), 'journey-fitment-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const pathA = `${casesPath}/${caseA}`;

    // 2. The classification (the fact scope of the part request facts).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Классифицировать / переклассифицировать"))').first().click();
    const classifyForm = page.locator('form[action$="/classify"]').first();
    await classifyForm.locator('select[name="type"]').selectOption('поиск детали');
    await classifyForm.locator('input[name="title"]').fill('Салонный фильтр');
    await classifyForm.locator('input[name="reason"]').fill('Клиент ищет фильтр салона по номеру');
    await classifyForm.getByRole('button', { name: /Классифицировать/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?classified=1`));

    // 3. The operator records the part request material through the
    // штатная fact form: the kind and the OEM number (the exact repeat
    // material of the request derivation).
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

    // 3b. The operator confirmation of the exact kind version (the
    // штатная confirm form — operator_relay): the confirmed set carries
    // the PartRequestDigest the evidence records pin (a later new
    // version of the confirmed kind changes the digest — the AC-11 stale
    // expiry of the dependent proofs).
    await page.goto(pathA);
    const kindFact = page.locator('details:has(summary:text-is("part.kind — Без подтверждения"))').first();
    await kindFact.click();
    const confirmDetails = kindFact.locator('details:has(summary:text-is("Подтвердить точную версию факта part.kind"))');
    await confirmDetails.click();
    const confirmForm = confirmDetails.locator('form[action$="/confirmations"]');
    await confirmForm.locator('select[name="version_no"]').selectOption('1');
    await confirmForm.locator('select[name="evidence_event_id"]').selectOption({ label: '— без события, использовать ноту —' });
    await confirmForm.locator('input[name="note"]').fill('Клиент назвал деталь по номеру');
    await confirmForm.getByRole('button', { name: /Подтвердить версию/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?confirmed=1`));

    // 4. The applicability panel of the absent proofs: the honest
    // case-level unknown word (AC-10 — the absence is never a negative or
    // a positive), the research recommendation with the 00021 word, the
    // constant number-match word (the exact match is not a proof), the
    // price note and the scope word.
    await page.goto(pathA);
    let fitmentPanel = page.locator('section[aria-label="Применимость"]');
    await expect(fitmentPanel).toBeVisible();
    await expect(fitmentPanel).toContainText('применимость не проверена — доказательства нет');
    await expect(fitmentPanel).toContainText('отсутствие доказательства не является ни подтверждением, ни отрицанием');
    await expect(fitmentPanel).toContainText('fitment_research');
    await expect(fitmentPanel).toContainText('Каталогическое исследование применимости');
    await expect(fitmentPanel).toContainText('точное совпадение номера не считается доказательством применимости');
    await expect(fitmentPanel).toContainText('не зафиксированные обязательства');
    await expect(fitmentPanel).toContainText('в пределах одного обращения');

    // 5. The manager's task through the existing request_human form: the
    // operator selects the 00021 research word (the recommendation of the
    // panel — the decision-ready task; the empty choice stays the
    // unconditional client ask).
    const requestDetails = page.locator('details:has(summary:text-is("Запросить человека (request_human — безусловная просьба клиента)"))');
    await requestDetails.click();
    const requestForm = requestDetails.locator('form[action$="/human-requests"]');
    await requestForm.locator('select[name="turn_ref"]').selectOption({ index: 1 });
    await requestForm.locator('input[name="target_roles"]').first().check();
    await requestForm.locator('select[name="ack_deadline"]').selectOption('ack_15m');
    await requestForm.locator('select[name="reason"]').selectOption('fitment_research');
    await requestForm.getByRole('button', { name: /Записать просьбу клиента/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?human_requested_recorded=1`));
    await page.goto(pathA);
    const handoffPanel = page.locator('section[aria-label="Запрос человека"]');
    await expect(handoffPanel).toContainText('Каталогическое исследование применимости');

    // 6. The LIVE record through the PRG form: the manager records the
    // found original proof with the whole traceability material (the
    // source, the record id, the version, the observed time — the 2.7
    // discipline; the catalog vehicle id of the researched catalog).
    await page.goto(pathA);
    const caseViewA = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const workItemA = caseViewA.work_items[0].WorkItemID;
    const recordForm = page.locator(`form[action$="/work-items/${workItemA}/fitment-evidences"]`).first();
    await recordForm.locator('input[name="position_key"]').fill('WF2B196K666AA');
    await recordForm.locator('input[name="catalog_node"]').fill('Фильтр салона');
    await recordForm.locator('input[name="article"]').fill('WF2B196K666AA');
    await recordForm.locator('select[name="link_type"]').selectOption('original');
    await recordForm.locator('textarea[name="chain"]').fill('Фильтр салона (оригинальная позиция)');
    await recordForm.locator('textarea[name="catalog_vehicle_ids"]').fill('ABCP-12345');
    await recordForm.locator('select[name="source_system"]').selectOption('abcp_catalog');
    await recordForm.locator('input[name="source_record_id"]').fill('rec-fitment-1');
    await recordForm.locator('input[name="source_version"]').fill('v2026.10');
    await recordForm.locator('input[name="source_observed_at"]').fill('2026-09-16T11:00:00Z');
    await recordForm.getByRole('button', { name: /Записать доказательство/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?fitment_evidence_recorded=1`));

    // 7. The proven state: the position carries the honest word, the
    // FitmentDigest and the material of the proof record (the source with
    // the record id and the version — the details without leaving the
    // case, AC-14).
    await page.goto(pathA);
    fitmentPanel = page.locator('section[aria-label="Применимость"]');
    await expect(fitmentPanel).toContainText('подтверждённо подходит');
    await expect(fitmentPanel).toContainText('WF2B196K666AA');
    await expect(fitmentPanel).toContainText('rec-fitment-1');
    await expect(fitmentPanel).toContainText('v2026.10');
    await expect(fitmentPanel).toContainText('ABCP-12345');
    await expect(fitmentPanel).toContainText('Доказательство №1');
    await expect(fitmentPanel).toContainText('профиль');
    const digestBlocks = fitmentPanel.locator('.digest-block');
    expect(await digestBlocks.count()).toBeGreaterThan(3);

    // 8. The ИСПРАВЛЕНИЕ of the request (AC-11): the operator records a
    // NEW version of the part kind — the PartRequestDigest changes, every
    // dependent proof turns stale with the honest words (the request
    // stale word + the re-check word).
    await recordFact('part.kind', '"салонный фильтр угольный"');
    await page.goto(pathA);
    fitmentPanel = page.locator('section[aria-label="Применимость"]');
    await expect(fitmentPanel).toContainText('доказательство относилось к прежней версии запроса');
    await expect(fitmentPanel).toContainText('перед новым предложением выполняется повторная каталогическая проверка');
    await expect(fitmentPanel).toContainText('доказательство устарело');

    // 9. The fresh record restores the proven state (AC-11 — the new
    // record carries the CURRENT input versions): the manager re-checks
    // the catalog and records the fresh proof.
    await page.goto(pathA);
    const recordFormTwo = page.locator(`form[action$="/work-items/${workItemA}/fitment-evidences"]`).first();
    await recordFormTwo.locator('input[name="position_key"]').fill('WF2B196K666AA');
    await recordFormTwo.locator('input[name="catalog_node"]').fill('Фильтр салона');
    await recordFormTwo.locator('input[name="article"]').fill('WF2B196K666AA');
    await recordFormTwo.locator('select[name="link_type"]').selectOption('original');
    await recordFormTwo.locator('textarea[name="chain"]').fill('Фильтр салона (оригинальная позиция)');
    await recordFormTwo.locator('textarea[name="catalog_vehicle_ids"]').fill('ABCP-12345');
    await recordFormTwo.locator('select[name="source_system"]').selectOption('abcp_catalog');
    await recordFormTwo.locator('input[name="source_record_id"]').fill('rec-fitment-2');
    await recordFormTwo.locator('input[name="source_version"]').fill('v2026.10');
    await recordFormTwo.locator('input[name="source_observed_at"]').fill('2026-09-16T12:00:00Z');
    await recordFormTwo.getByRole('button', { name: /Записать доказательство/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?fitment_evidence_recorded=1`));
    await page.goto(pathA);
    fitmentPanel = page.locator('section[aria-label="Применимость"]');
    await expect(fitmentPanel).toContainText('подтверждённо подходит');
    await expect(fitmentPanel).toContainText('Доказательство №2');
    await expect(fitmentPanel).not.toContainText('доказательство относилось к прежней версии запроса');

    // 10. The a11y close of the panel: the fitment SECTION never widens
    // the layout (the tables scroll inside their wraps), the stale words
    // and the boundaries are the alert TEXT (never only color — AC-13/
    // AC-14).
    await expect(page.locator('#fitment')).toBeVisible();
    const sectionOverflow = await page.evaluate(() => {
      const section = document.getElementById('fitment')!;
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
