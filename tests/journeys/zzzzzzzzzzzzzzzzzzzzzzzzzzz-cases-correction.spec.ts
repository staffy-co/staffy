import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const chatConnectionID = '01994f64-6c00-7000-8000-000000000016';
const ownerID = '01994f64-6c00-7000-8000-000000000001';
const secondOperatorID = '01994f64-6c00-7000-8000-00000000000a';
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

// The Story 2.15 chain at 320px: the client's words (the anchor turn with
// the true address and the true price — the correction evidence), the
// recorded fact with the TYPO (the confirmed error — the honest human
// story: the operator confirmed too hastily), the price of the catalog
// (the matrix field), the AI summary over the recorded state (the
// staleness target), the control branch of the ZERO correction state
// (the fact rows and the whole section byte-identical to 2.14 — the
// markers appear only with the material), the operator sees the error,
// opens the PRG form «Исправить» on the fact row of the Work Control
// Card (the pinned exact version, the proposed value, the byte-exact
// span of the client's words, the summary link, the mandatory reason),
// the submit lands on the #decision-context anchor with the honest
// status line, the applied marker (the number, the outcome word, the
// reason, the event source, the span counter behind the progressive
// disclosure), the read-time conflict (the row moves to the Conflicting
// group — NEVER an automatic confirmation), the poured version with the
// correction:<id> source key in the ladder, the LIVE stale badge of the
// summary with the honest fact-version counter, the machine dispute of
// the matrix price through the JSON path (the server-decided disputed
// outcome, the plain 404-free replay with the Idempotency-Replayed
// header — the Owning System value is never rewritten, the watermark
// never moves), the human resolution through the EXISTING confirm
// ladder (the evidence event, the operator_relay confirmer — the
// machine proposes, the human confirms), the confirmed row that KEEPS
// the correction marker (the append-only history), the control branch
// of the fresh case (zero correction artifacts) and the a11y close
// (the polite live regions, the role=status markers, the ≥44×44 form
// target, the 320px column without the horizontal scroll).
test('cases-correction: the confirmed typo, the PRG correction with the span, the applied marker, the read-time conflict, the stale summary, the machine dispute of the matrix price and the human confirm at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One TEXT turn of one web_chat session: the client names the
    // address and the price — the turn is the anchor of the correction
    // evidence (the byte-exact span source) and of the confirmations.
    const first = await postIngress(api, textInboundEvent('journey-cor-evt-1', 'journey-cor-sess-1', { say: 'адрес доставки: Казань, Кремлёвская 15; цена доставки 1200' }), 'journey-cor-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const turnOne = (await first.json()).event_id;
    const pathA = `${casesPath}/${caseA}`;
    const caseView0 = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;

    // 2. The recorded fact with the TYPO (the operator heard the client
    // and typed the wrong house — then confirmed it hastily: the honest
    // human story the correction discipline exists for) and the matrix
    // price of the catalog (the dispute target).
    const address = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/facts`, {
      data: {
        work_item_id: workItemID,
        material_key: 'address',
        value: '"Казань, Кремлёвская 1"',
        source_type: 'operator',
        reason: 'оператор записал адрес клиента',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-cor-fact-address-1" },
    });
    expect(address.status()).toBe(201);
    const addressOutcome = await address.json();
    const addressFactID = addressOutcome.fact_id;
    const confirmTypo = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/facts/${addressFactID}/confirmations`, {
      data: {
        version_no: addressOutcome.fact_version,
        confirmer_type: 'operator_relay',
        evidence_event_id: turnOne,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-cor-confirm-typo-1" },
    });
    expect(confirmTypo.status()).toBe(201);
    const price = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/facts`, {
      data: {
        work_item_id: workItemID,
        material_key: 'price',
        value: '"1500.00"',
        source_type: 'operator',
        reason: 'цена из каталога',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-cor-fact-price-1" },
    });
    expect(price.status()).toBe(201);
    const priceFactID = (await price.json()).fact_id;

    // 3. The ownership prelude of the summary producer (the 2.13/2.14
    // precedent): the unconditional ask through the machine path, the
    // assignment of the second operator (the backup owner), the
    // acknowledgement of the assignee (the epoch 2 human owner) and the
    // explicit return to Staffy (the epoch 3 staffy owner) — the summary
    // is the act of the owner-Staffy of the moment.
    const ask = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests`, {
      data: {
        turn_ref: turnOne,
        expected_mode_version: 1,
        target_roles: ['operator'],
        ack_deadline: 'ack_1h',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-cor-ask-1" },
    });
    expect(ask.status()).toBe(201);
    const assignment = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests/assignment`, {
      data: {
        assignee: secondOperatorID,
        backup: ownerID,
        ack_deadline: 'ack_1h',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-cor-assign-1" },
    });
    expect(assignment.status()).toBe(201);
    await api.post('/fixture/operator', { form: { second: '1' } });
    const ack = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests/acknowledgement`, {
      data: { expected_epoch: 1 },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-cor-ack-1" },
    });
    expect(ack.status()).toBe(201);
    const returned = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests/return`, {
      data: { result: 'issue_resolved', expected_epoch: 2 },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-cor-return-1" },
    });
    expect(returned.status()).toBe(201);
    await api.post('/fixture/operator', { form: { second: '' } });

    // 4. The AI summary over the recorded state (the 2.14 staleness
    // target: the watermark pins the pre-correction truth — one fact
    // version of the address, one of the price).
    const summaryText = 'Клиент назвал адрес доставки: Казань, Кремлёвская 1.';
    const summary = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/summaries`, {
      data: {
        kind: 'ai_summary',
        text: summaryText,
        claims: [
          { ref_type: 'fact_version', ref_id: addressFactID, text: 'Клиент назвал адрес доставки: Казань, Кремлёвская 1.' },
        ],
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-cor-summary-1" },
    });
    expect(summary.status()).toBe(201);

    // 4. The control branch BEFORE any correction: the page renders the
    // ZERO correction state — no marker rows, no status line, no
    // correction artifacts anywhere (the byte-identical 2.14 render:
    // the markers appear only with the material), the confirmed address
    // row and the fresh summary word.
    await page.goto(pathA);
    await expect(page.locator('tr[id^="ctx-correction-"]')).toHaveCount(0);
    await expect(page.locator('#ctx-correction-recorded')).toHaveCount(0);
    await expect(page.locator('#ctx-fact-' + addressFactID)).toContainText('Подтверждено: точная версия подтверждена');
    await expect(page.locator('#ctx-summary-staleness')).toContainText('Сводка актуальна (actual)');

    // 5. The operator sees the error and opens the PRG form «Исправить»
    // on the fact row of the Work Control Card: the pinned exact
    // version, the proposed value, the byte-exact span of the client's
    // words, the summary link (0 = the event evidence) and the
    // mandatory reason (the deliberate act).
    await page.locator('details:has(summary:text-is("address — Подтверждено"))').first().click();
    await page.getByText('Исправить факт address (операторская коррекция)').first().click();
    const correctionForm = page.locator(`form[action$="/facts/${addressFactID}/corrections"]`).first();
    await correctionForm.locator('select[name="version_no"]').selectOption('1');
    await correctionForm.locator('textarea[name="value"]').fill('"Казань, Кремлёвская 15"');
    await correctionForm.locator('input[name="span"]').fill('адрес доставки: Казань, Кремлёвская 15');
    await correctionForm.locator('input[name="summary_no"]').fill('0');
    await correctionForm.locator('input[name="reason"]').fill('оператор сверился с репликой клиента');
    await correctionForm.getByRole('button', { name: /Исправить \(акт оператора\)/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?correction_recorded=1`));

    // 6. The honest status line of the PRG landing (the #decision-context
    // anchor) and the applied marker of the fact row: the number, the
    // outcome word of the matrix gate, the reason, the event source, the
    // span counter — the spans behind the progressive disclosure.
    await expect(page.locator('#ctx-correction-recorded')).toContainText('Коррекция записана (append-only)');
    const addressMarker = page.locator('#ctx-correction-' + addressFactID);
    await expect(addressMarker).toContainText('Исправление факта address: исправлено — коррекция №1 (исход applied)');
    await expect(addressMarker).toContainText('Причина: оператор сверился с репликой клиента');
    await expect(addressMarker).toContainText('источник исправления: событие');
    await expect(addressMarker).toContainText('1 выдержка');
    await expect(addressMarker).toContainText('Подтверждение — отдельная команда точной версии');

    // 7. The read-time conflict: the row moves to the Conflicting group
    // (NEVER an automatic confirmation — the machine proposes, the
    // human confirms); the poured version carries the correction:<id>
    // source key in the ladder of the Work Control Card.
    await expect(page.locator('#ctx-fact-' + addressFactID)).toContainText('Противоречие: значения расходятся');
    const addressDetails = page.locator('details:has(summary:text-is("address — Конфликт значений"))').first();
    await expect(addressDetails).toBeVisible();
    await expect(addressDetails).toContainText('correction:');

    // 8. The LIVE stale badge of the summary: the applied pour grew the
    // watermark — the honest counter of the new fact versions.
    await expect(page.locator('#ctx-summary-staleness')).toContainText('Источники изменились после сводки.');
    await expect(page.locator('#ctx-summary-staleness')).toContainText('новых версий фактов: 1');

    // 9. The machine dispute of the matrix price through the JSON path:
    // the OUTCOME is the SERVER decision (the matrix gate — the request
    // never carries it), the fresh record is the 201, the exact repeat
    // the 200 with the Idempotency-Replayed header (the replay of the
    // whole-material hash).
    const disputeBody = {
      version_no: 1,
      value: '"1200.00"',
      spans: ['цена доставки 1200'],
      reason: 'клиент называет другую цену',
    };
    const dispute = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/facts/${priceFactID}/corrections`, {
      data: disputeBody,
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-cor-dispute-1" },
    });
    expect(dispute.status()).toBe(201);
    const disputeOutcome = await dispute.json();
    expect(disputeOutcome.command).toBe('record_correction');
    expect(disputeOutcome.correction_outcome).toBe('disputed');
    const disputeReplay = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/facts/${priceFactID}/corrections`, {
      data: disputeBody,
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-cor-dispute-1" },
    });
    expect(disputeReplay.status()).toBe(200);
    expect(disputeReplay.headers()['idempotency-replayed']).toBe('true');

    // 10. The dispute marker of the matrix row: the honest words (the
    // value of the Owning System is never rewritten), the dispute
    // NEVER pours (the price keeps ONE version) and NEVER grows the
    // watermark (the dispute is not a rewrite — the summary stays
    // honest about what actually changed).
    await page.goto(pathA);
    const priceMarker = page.locator('#ctx-correction-' + priceFactID);
    await expect(priceMarker).toContainText('Исправление факта price: оспорено (значение системы не изменено) — коррекция №2 (исход disputed)');
    await expect(priceMarker).toContainText('источник исправления: событие');
    await expect(page.locator('#ctx-fact-' + priceFactID)).toContainText('Не подтверждено: значение без подтверждения');
    await expect(page.locator('#ctx-summary-staleness')).toContainText('новых версий фактов: 1');

    // 11. The human resolution through the EXISTING confirm ladder: the
    // operator confirms the EXACT poured version with the evidence
    // event (the operator_relay confirmer) — the live command
    // separation of the slice: the machine proposes, the human
    // confirms.
    await page.getByText('Подтвердить точную версию факта address').first().click();
    const confirmForm = page.locator(`form[action$="/facts/${addressFactID}/confirmations"]`).first();
    await confirmForm.locator('select[name="version_no"]').selectOption('2');
    await confirmForm.locator('select[name="evidence_event_id"]').selectOption(turnOne);
    await confirmForm.getByRole('button', { name: /Подтвердить версию/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?confirmed=1`));

    // 12. The confirmed row KEEPS the correction marker (the append-only
    // history never disappears), the conflict is resolved read-time and
    // the watermark stays (a confirmation is not a new version).
    await page.goto(pathA);
    await expect(page.locator('#ctx-fact-' + addressFactID)).toContainText('Подтверждено: точная версия подтверждена');
    await expect(page.locator('#ctx-correction-' + addressFactID)).toContainText('коррекция №1 (исход applied)');
    await expect(page.locator('#ctx-summary-staleness')).toContainText('новых версий фактов: 1');

    // 13. The control branch: a FRESH case without corrections renders
    // ZERO correction artifacts (the byte-identical page of the previous
    // stories — the markers appear only with the material).
    const control = await postIngress(api, textInboundEvent('journey-cor-evt-2', 'journey-cor-sess-2', { say: 'спасибо' }), 'journey-cor-key-2');
    expect(control.status()).toBe(201);
    const caseB = (await control.json()).case_id;
    await page.goto(`${casesPath}/${caseB}`);
    await expect(page.locator('tr[id^="ctx-correction-"]')).toHaveCount(0);
    await expect(page.locator('#ctx-correction-recorded')).toHaveCount(0);

    // 14. The a11y close of the operator page: the polite live region of
    // the stale badge, the role=status markers, the ≥44×44 target of
    // the correction form button and the 320px single column (no
    // horizontal scroll).
    await page.goto(pathA);
    await expect(page.locator('#ctx-summary-staleness')).toHaveAttribute('aria-live', 'polite');
    await expect(page.locator('#ctx-correction-' + addressFactID + ' p[role="status"]')).toHaveCount(1);
    await page.locator('details:has(summary:text-is("address — Подтверждено"))').first().click();
    await page.getByText('Исправить факт address (операторская коррекция)').first().click();
    const submitBox = await page.locator(`form[action$="/facts/${addressFactID}/corrections"] button.primary`).first().boundingBox();
    expect(submitBox!.height).toBeGreaterThanOrEqual(44);
    expect(submitBox!.width).toBeGreaterThanOrEqual(44);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    await page.close();
  }
});
