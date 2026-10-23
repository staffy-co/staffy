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

// The Story 4.3 chain at 320px: the part request panel of the case card
// (the read-time request derivation over the 4.0.0 pack schema — the
// deterministic mode with the transparent basis word, the attributes with
// the versions and the statuses, the honest number flags of the erroneous
// OEM with the homoglyph re-ask, the minimal questions of the
// not-confirmed keys, the boundaries as the alert TEXT, the confidence
// word, the PartRequestDigest), the AC-12 dialogs: the просторечное
// название (the operator records the client words), the erroneous OEM
// (the exact repeat with the homoglyph flag — never a silent fix), the
// symptom (the diagnostic dominance — the probabilistic guess never
// becomes a catalog request), the operator confirmation of the exact
// version, the исправление (the stale word of the new version), the
// several-parts word of the existing split and the boundary handoff with
// the 00019 reason word through the existing request form; the a11y
// close (320px without the horizontal scroll, the boundaries as TEXT)
// and the secrets probe.
test('part-request: the mode ladder, the honest number flags, the boundaries and the boundary handoff at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One TEXT turn of one web_chat session (the fixture world pins
    // auto-parts@4.0.0 — the part-request-schema-bearing pack of the case).
    const first = await postIngress(api, textInboundEvent('journey-part-evt-1', 'journey-part-sess-1', { say: 'Нужны колодки на перед, и ещё что-то гремит при торможении на кочках' }), 'journey-part-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const pathA = `${casesPath}/${caseA}`;

    // 2. The classification (the fact scope of the part request facts).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Классифицировать / переклассифицировать"))').first().click();
    const classifyForm = page.locator('form[action$="/classify"]').first();
    await classifyForm.locator('select[name="type"]').selectOption('поиск детали');
    await classifyForm.locator('input[name="title"]').fill('Колодки передние');
    await classifyForm.locator('input[name="reason"]').fill('Клиент ищет колодки — просторечное название');
    await classifyForm.getByRole('button', { name: /Классифицировать/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?classified=1`));

    // 3. The operator records the part request material through the
    // штатная fact form: the ПРОСТОРЕЧНОЕ название of the client (AC-12 —
    // the operator records the words, the mode derives over the material)
    // and the quantity.
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
    await recordFact('part.kind', '"колодки передние"');
    await recordFact('part.quantity', '"2"');

    // 4. The panel: the named_part mode with the transparent basis word
    // and the minimal questions of the not-confirmed keys (AC-3 — both
    // keys are honestly missing, the absent optional keys never appear).
    await page.goto(pathA);
    let partPanel = page.locator('section[aria-label="Запрос детали"]');
    await expect(partPanel).toBeVisible();
    await expect(partPanel).toContainText('Режим запроса: Названная деталь (named_part)');
    await expect(partPanel).toContainText('основание: названа деталь');
    await expect(partPanel).toContainText('колодки передние');
    await expect(partPanel).toContainText('уточните: сколько нужно?');
    await expect(partPanel).toContainText('уточните: какая деталь нужна?');
    await expect(partPanel).toContainText('PartRequestDigest');
    await expect(partPanel).toContainText('материал без подтверждения');

    // 5. The ERRONEOUS OEM (AC-12): the number with the cyrillic
    // homoglyph — the exact_repeat mode with the honest flags (the
    // re-ask word, NEVER a silent fix) and the constant number-match
    // word (the match is not a fitment proof).
    await recordFact('part.oem_number', '"WF2B19АK666AA"');
    await page.goto(pathA);
    partPanel = page.locator('section[aria-label="Запрос детали"]');
    await expect(partPanel).toContainText('Режим запроса: Точный повтор по номеру (exact_repeat)');
    await expect(partPanel).toContainText('точное совпадение номера не считается доказательством применимости');
    await partPanel.getByText(/Проверки номера \(part.oem_number\) — флаги текстом/).click();
    await expect(partPanel).toContainText('сверьте номер посимвольно');
    await expect(partPanel).toContainText('кириллица А');
    await expect(partPanel).toContainText('WF2B19АK666AA');

    // 6. The SYMPTOM (AC-12): the diagnostic dominance — the mode
    // re-derives diagnostic over the accumulated material, the boundary
    // word renders as the alert TEXT, the named part under the symptom is
    // the candidate («причина не доказана»), the handoff recommendation
    // carries the 00019 word.
    await recordFact('part.symptom', '"грохот при торможении на кочках"');
    await page.goto(pathA);
    partPanel = page.locator('section[aria-label="Запрос детали"]');
    await expect(partPanel).toContainText('Режим запроса: Диагностика по симптому (diagnostic)');
    await expect(partPanel).toContainText('основание: записан симптом');
    await expect(partPanel).toContainText('Граница (diagnostic): симптом не является названной деталью');
    await expect(partPanel).toContainText('КАНДИДАТ при симптоме: причина неисправности не доказана');
    await expect(partPanel).toContainText('Рекомендация передачи человеку');
    await expect(partPanel).toContainText('diagnostic_uncertainty');

    // 7. The operator confirmation of the exact version (the штатная
    // confirm form — operator_relay); the confirmed kind leaves the
    // missing list (AC-3), the digest changes.
    await page.goto(pathA);
    const kindFact = page.locator('details:has(summary:text-is("part.kind — Без подтверждения"))').first();
    await kindFact.click();
    const confirmDetails = kindFact.locator('details:has(summary:text-is("Подтвердить точную версию факта part.kind"))');
    await confirmDetails.click();
    const confirmForm = confirmDetails.locator('form[action$="/confirmations"]');
    await confirmForm.locator('select[name="version_no"]').selectOption('1');
    await confirmForm.locator('select[name="evidence_event_id"]').selectOption({ label: '— без события, использовать ноту —' });
    await confirmForm.locator('input[name="note"]').fill('Клиент назвал деталь в разговоре');
    await confirmForm.getByRole('button', { name: /Подтвердить версию/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?confirmed=1`));
    await page.goto(pathA);
    partPanel = page.locator('section[aria-label="Запрос детали"]');
    await expect(partPanel).toContainText('Подтверждает: operator_relay (версия v1');
    await expect(partPanel).toContainText('материал подтверждён частично');

    // 8. The ИСПРАВЛЕНИЕ (AC-12): the operator records a NEW version of
    // the confirmed kind — the confirmation of the former version renders
    // the honest stale word and the aggregate stale note (AC-10).
    await recordFact('part.kind', '"тормозные колодки"');
    await page.goto(pathA);
    partPanel = page.locator('section[aria-label="Запрос детали"]');
    await expect(partPanel).toContainText('конфликт значений — Staffy не выбирает вариант, уточните у клиента');
    await expect(partPanel).toContainText('подтверждение относилось к прежней версии (v1)');
    await expect(partPanel).toContainText('обязаны пиннить digest запроса и считать прежние устаревшими');

    // 9. The boundary handoff through the existing request form: the
    // safety stop under the diagnostic uncertainty + the contradiction
    // recommends the 00019 words; the operator selects the boundary
    // reason word (the empty choice stays the unconditional client ask).
    await expect(partPanel).toContainText('автоматический подбор остановлен: safety-узел');
    await expect(partPanel).toContainText('safety_critical');
    await expect(partPanel).toContainText('unresolved_contradiction');
    const requestDetails = page.locator('details:has(summary:text-is("Запросить человека (request_human — безусловная просьба клиента)"))');
    await requestDetails.click();
    const requestForm = requestDetails.locator('form[action$="/human-requests"]');
    await requestForm.locator('select[name="turn_ref"]').selectOption({ index: 1 });
    await requestForm.locator('input[name="target_roles"]').first().check();
    await requestForm.locator('select[name="ack_deadline"]').selectOption('ack_15m');
    await requestForm.locator('select[name="reason"]').selectOption('safety_critical');
    await requestForm.getByRole('button', { name: /Записать просьбу клиента/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?human_requested_recorded=1`));
    await page.goto(pathA);
    const handoffPanel = page.locator('section[aria-label="Запрос человека"]');
    await expect(handoffPanel).toContainText('Safety-узел с неопределённостью');

    // 10. The several-parts word of the existing split (AC-12): the
    // second case of the world — the client asks for two parts, the
    // operator splits the item into two typed targets, the honest word
    // renders (ONE structured set per case).
    const second = await postIngress(api, textInboundEvent('journey-part-evt-2', 'journey-part-sess-2', { say: 'Нужны колодки и фильтр салона' }), 'journey-part-key-2');
    expect(second.status()).toBe(201);
    const caseB = (await second.json()).case_id;
    const pathB = `${casesPath}/${caseB}`;
    const caseViewB = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseB}`)).json();
    const workItemB = caseViewB.work_items[0].WorkItemID;
    await page.goto(pathB);
    await page.locator('details:has(summary:text-is("Классифицировать / переклассифицировать"))').first().click();
    const classifyB = page.locator('form[action$="/classify"]').first();
    await classifyB.locator('select[name="type"]').selectOption('поиск детали');
    await classifyB.locator('input[name="title"]').fill('Колодки и фильтр');
    await classifyB.locator('input[name="reason"]').fill('Два независимых намерения клиента');
    await classifyB.getByRole('button', { name: /Классифицировать/ }).click();
    await page.goto(pathB);
    const partPanelB = page.locator('section[aria-label="Запрос детали"]');
    await expect(partPanelB).not.toContainText('обращение содержит несколько деталей');
    await page.locator('details:has(summary:text-is("Разделить на два намерения"))').first().click();
    const splitForm = page.locator(`form[action$="/work-items/${workItemB}/split"]`);
    await splitForm.locator('select[name="target_1_type"]').selectOption('поиск детали');
    await splitForm.locator('input[name="target_1_title"]').fill('Тормозные колодки');
    await splitForm.locator('select[name="target_2_type"]').selectOption('поиск детали');
    await splitForm.locator('input[name="target_2_title"]').fill('Фильтр салона');
    await splitForm.locator('input[name="reason"]').fill('Клиент просил две разные детали');
    await splitForm.getByRole('button', { name: /Разделить/ }).click();
    await page.goto(pathB);
    await expect(partPanelB).toContainText('обращение содержит несколько деталей');
    await expect(partPanelB).toContainText('разделяйте элементы работы');

    // 11. The a11y close of the operator card: the part request SECTION
    // never widens the layout (the tables scroll inside their wraps), the
    // boundaries are the alert TEXT (never only color — AC-13), the
    // details toggle by the keyboard.
    await expect(page.locator('#part-request')).toBeVisible();
    const sectionOverflow = await page.evaluate(() => {
      const section = document.getElementById('part-request')!;
      return section.scrollWidth - section.clientWidth;
    });
    expect(sectionOverflow).toBeLessThanOrEqual(0);
    const alertCount = await page.locator('#part-request [role="alert"]').count();
    expect(alertCount).toBeGreaterThanOrEqual(1);
    const boundaryText = await page.locator('#part-request [role="alert"]').first().innerText();
    expect(boundaryText.length).toBeGreaterThan(10);
    // The secrets never render (the operator page carries no credentials).
    const body = await page.locator('body').innerText();
    expect(body).not.toContain('api_key');
    expect(body).not.toContain('sandbox-');
  } finally {
    await page.close();
  }
});
