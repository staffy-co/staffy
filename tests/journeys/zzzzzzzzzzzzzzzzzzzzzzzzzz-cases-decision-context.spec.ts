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

// The Story 2.14 chain at 320px: the client's ask («передайте меня
// специалисту» — the anchor turn), the material of the section (the
// confirmed address fact with the provenance, the unconfirmed urgency,
// the conflicting phone of two values, the observation, the slowed_pace
// preference visible BEFORE the first reply), the sales prelude with the
// evaluated proposal (the recommendation basis), the ask and the
// assignment through the PRG forms, the assigned operator sees the
// «Контекст передачи» section BEFORE the acknowledgement (the AC-1
// blocks: the reason with the anchor and the deadline, the active work
// item, the mode word, the assurance, the owner), the acceptance and
// the first reply of the new owner (the neutral status message — the
// slowed_pace respect: one considered message, the sales path stays
// closed by the ladder), the return to Staffy and the second ask (the
// staffy owner with the open handoff), the producer summary (the
// machine JSON with the byte-exact claims: the linked fact anchor and
// the honestly disputed span), the section with the marking badge and
// the claim links, the fresh word (actual — the control branch), the
// new client event (the stale badge with the honest counters), the
// refresh by the #decision-context anchor (the section position
// survives), the recommendation projection (the plan word with the
// checks and the mode constraint), the absent-section control branch of
// the fresh case and the a11y close (the polite live regions, the
// ≥44×44 refresh target, the 320px column without the horizontal
// scroll).
test('cases-decision-context: ask, assignment, the context before ack, the Staffy summary, the stale counters and the recommendation at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One TEXT turn of one web_chat session: the client asks for a
    // specialist and names the address — the turn is the anchor of the
    // ask and the provenance of the confirmed address fact.
    const first = await postIngress(api, textInboundEvent('journey-ctx-evt-1', 'journey-ctx-sess-1', { say: 'передайте меня специалисту; адрес: Казань, Кремлёвская 1' }), 'journey-ctx-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const turnOne = (await first.json()).event_id;
    const pathA = `${casesPath}/${caseA}`;
    const caseView0 = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;
    const conversationID = caseView0.conversations[0].ConversationID;

    // 2. The material of the section: the confirmed address fact (the
    // client event provenance), the unconfirmed urgency, the conflicting
    // phone of two distinct values and the slowed_pace preference.
    const address = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/facts`, {
      data: {
        work_item_id: workItemID,
        material_key: 'address',
        value: '"Казань, Кремлёвская 1"',
        source_type: 'channel_event',
        source_id: turnOne,
        reason: 'Клиент назвал адрес в реплике',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-ctx-fact-address-1" },
    });
    expect(address.status()).toBe(201);
    const addressOutcome = await address.json();
    const addressFactID = addressOutcome.fact_id;
    const confirmAddress = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/facts/${addressFactID}/confirmations`, {
      data: {
        version_no: addressOutcome.fact_version,
        confirmer_type: 'operator_relay',
        evidence_event_id: turnOne,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-ctx-confirm-address-1" },
    });
    expect(confirmAddress.status()).toBe(201);
    const urgency = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/facts`, {
      data: {
        work_item_id: workItemID,
        material_key: 'urgency',
        value: '"срочно"',
        source_type: 'operator',
        reason: 'Клиент торопится',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-ctx-fact-urgency-1" },
    });
    expect(urgency.status()).toBe(201);
    for (const [i, value] of ['"+79000000240"', '"+79000000241"'].entries()) {
      const phone = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/facts`, {
        data: {
          work_item_id: workItemID,
          material_key: 'contact_phone',
          value,
          source_type: 'operator',
          reason: 'Два значения телефона',
        },
        headers: { "Content-Type": "application/json", "Idempotency-Key": `journey-ctx-fact-phone-${i}` },
      });
      expect(phone.status()).toBe(201);
    }
    const preference = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/accommodations`, {
      data: {
        accommodation_key: 'slowed_pace',
        note: 'клиент просит не торопиться',
        cleared: false,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-ctx-accommodation-1" },
    });
    expect(preference.status()).toBe(201);

    // 3. The sales prelude with the evaluated proposal (the
    // recommendation basis of the section): the mode raise, the acting
    // disclosure, the price proposal and the commercial outcome.
    const sales = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/mode/proposals`, {
      data: {
        conversation_id: conversationID,
        mode: 'sales',
        scope: 'conversation',
        reason: 'клиент интересуется ценой',
        signal_source: 'turn_event',
        turn_ref: turnOne,
        expected_mode_version: 1,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-ctx-mode-sales-1" },
    });
    expect(sales.status()).toBe(201);
    const caseView1 = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const disclosure = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/disclosures`, {
      data: {
        conversation_id: conversationID,
        channel: 'web_chat',
        text: 'Я цифровой сотрудник компании. Человек подключится по вашему запросу.',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-ctx-disclosure-1" },
    });
    expect(disclosure.status()).toBe(201);
    const disclosureID = (await disclosure.json()).disclosure_id;
    const disclosureSent = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/disclosures/${disclosureID}/delivery-events`, {
      data: { conversation_id: conversationID, kind: 'sent' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-ctx-disclosure-sent-1" },
    });
    expect(disclosureSent.status()).toBe(201);
    const proposal = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/ai-proposals`, {
      data: {
        conversation_id: conversationID,
        work_item_id: workItemID,
        turn_ref: turnOne,
        intent: 'price_inquiry',
        candidate_answer: 'Диагностика от 3000',
        missing_facts: [
          { material_key: 'service_kind', reason: 'mandatory' },
          { material_key: 'urgency', reason: 'mandatory' },
        ],
        proposer: 'synthetic_port',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-ctx-proposal-1" },
    });
    expect(proposal.status()).toBe(201);
    const proposalID = (await proposal.json()).proposal_id;
    const evaluate = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/policy/evaluations`, {
      data: {
        proposal_ref: proposalID,
        expected_mode_version: caseView1.policy.mode_version,
        expected_owner_epoch: caseView1.policy.owner_epoch,
        expected_work_item_version: caseView1.work_items[0].Version,
        expected_facts_digest: caseView1.policy.facts_digest,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-ctx-evaluate-1" },
    });
    expect(evaluate.status()).toBe(201);

    // 4. The ask through the PRG form (the anchor turn of the client).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Запросить человека (request_human — безусловная просьба клиента)"))').click();
    const askForm = page.locator('form[action$="/human-requests"]').first();
    await askForm.locator('select[name="turn_ref"]').selectOption(turnOne);
    await askForm.locator('input[name="target_roles"][value="operator"]').check();
    await askForm.locator('select[name="ack_deadline"]').selectOption('ack_1h');
    await askForm.getByRole('button', { name: /Записать просьбу клиента/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?human_requested_recorded=1`));

    // 5. The assignment through the PRG form (the second operator is the
    // assignee, the owner is the backup).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Назначить специалиста (assign_human_handoff — назначение ≠ принятие)"))').click();
    const assignForm = page.locator('form[action$="/human-requests/assignment"]').first();
    await assignForm.locator('select[name="assignee"]').selectOption(secondOperatorID);
    await assignForm.locator('select[name="backup"]').selectOption(ownerID);
    await assignForm.locator('select[name="ack_deadline"]').selectOption('ack_1h');
    await assignForm.getByRole('button', { name: /Назначить раунд/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?human_handoff_assigned=1`));

    // 6. The assigned operator sees the section BEFORE the
    // acknowledgement (AC-1): the second operator logs in — the
    // «Контекст передачи» section with the reason of the ask, the anchor
    // turn, the deadline of the round, the active work item, the mode
    // word, the three fact groups with the provenance, the observation,
    // the commitment receipt line, the slowed_pace preference (visible
    // BEFORE the first reply) and the honest absence of the summary.
    await api.post('/fixture/operator', { form: { second: '1' } });
    await page.goto(pathA);
    const section = page.locator('#decision-context');
    await expect(section).toContainText('Что происходит');
    await expect(section.locator('#ctx-handoff')).toContainText('Причина передачи');
    await expect(section.locator('#ctx-handoff')).toContainText('Безусловная просьба клиента');
    await expect(section.locator('#ctx-handoff')).toContainText(turnOne);
    await expect(section.locator('#ctx-handoff')).toContainText('assigned');
    await expect(section).toContainText('untriaged');
    await expect(section).toContainText('human_requested');
    await expect(section).toContainText('Подтверждено: точная версия подтверждена');
    await expect(page.locator('#ctx-fact-' + addressFactID)).toContainText('address');
    await expect(section).toContainText('Не подтверждено: значение без подтверждения');
    await expect(section).toContainText('urgency');
    await expect(section).toContainText('Противоречие: значения расходятся');
    await expect(section).toContainText('contact_phone');
    await expect(page.locator('#ctx-observations')).toContainText('Наблюдения систем');
    await expect(section).toContainText('Предпочтения клиента');
    await expect(section).toContainText('slowed_pace');
    await expect(section).toContainText('Предпочтения видны до первого ответа');
    await expect(section).toContainText('Сводка AI');
    await expect(section).toContainText('Сводка AI ещё не записана');
    await expect(section).toContainText('Рекомендуемый шаг');
    await expect(section).toContainText('Рекомендация — проекция слов плана');

    // 7. The acceptance (the epoch 2 human owner) and the FIRST reply of
    // the new owner: the neutral status message — the slowed_pace
    // respect (one considered message; the sales path stays closed by
    // the ladder).
    const ack = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests/acknowledgement`, {
      data: { expected_epoch: 1 },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-ctx-ack-1" },
    });
    expect(ack.status()).toBe(201);
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const replyForm = page.locator('form[action$="/client-replies"]').first();
    await replyForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    await replyForm.locator('select[name="reply_class"]').selectOption('neutral');
    await replyForm.locator('textarea[name="content"]').fill('Специалист принял обращение и разбирается не спеша.');
    await replyForm.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?reply_prepared=1`));

    // 8. The return to Staffy (the epoch 3 staffy owner) and the second
    // unconditional ask: the staffy owner with the OPEN handoff — the
    // producer moment of the summary.
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Вернуть обращение Staffy (return_to_staffy — после подтверждения)"))').click();
    const returnForm = page.locator('form[action$="/human-requests/return"]').first();
    await returnForm.locator('select[name="result"]').selectOption('issue_resolved');
    await returnForm.getByRole('button', { name: /Вернуть цифровому сотруднику/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?human_handoff_returned=1`));
    const secondAsk = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests`, {
      data: {
        turn_ref: turnOne,
        expected_mode_version: 3,
        target_roles: ['operator'],
        ack_deadline: 'ack_1h',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-ctx-ask-2" },
    });
    expect(secondAsk.status()).toBe(201);

    // 9. The producer summary (the machine JSON path — the PRG form does
    // NOT exist): the claims carry the byte-exact spans — the linked
    // fact anchor and the honestly disputed span; the request never
    // carries the summary number or the watermark (the server truth).
    const summaryText = 'Клиент назвал адрес доставки: Казань, Кремлёвская 1. Срочность не подтверждена источником.';
    const summary = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/summaries`, {
      data: {
        kind: 'ai_summary',
        text: summaryText,
        claims: [
          { ref_type: 'fact_version', ref_id: addressFactID, text: 'Клиент назвал адрес доставки: Казань, Кремлёвская 1.' },
          { disputed: true, text: 'Срочность не подтверждена источником.' },
        ],
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-ctx-summary-1" },
    });
    expect(summary.status()).toBe(201);
    const summaryOutcome = await summary.json();
    expect(summaryOutcome.command).toBe('record_case_summary');
    expect(summaryOutcome.summary_kind).toBe('ai_summary');
    expect(summaryOutcome.summary_claims).toBe(2);

    // 10. The section with the summary (AC-5/AC-6): the marking badge of
    // the AI-derived representation, the claim links to the fact rows,
    // the honest fresh word (actual — the sources did not move after the
    // record: the control branch of the chain).
    await page.goto(pathA);
    await expect(page.locator('#ctx-summary-staleness')).toContainText('Сводка актуальна (actual)');
    await expect(section).toContainText('AI-производное представление — сверяйтесь с источниками ниже.');
    await expect(section).toContainText('Сводка №1 (ai_summary)');
    await expect(section).toContainText(summaryText);
    const claimRow = page.locator('#decision-context table.datamap-table tbody tr').filter({ hasText: 'адрес доставки' });
    await expect(claimRow).toContainText('факт');
    await expect(claimRow.locator(`a[href="#ctx-fact-${addressFactID}"]`)).toHaveCount(1);
    await expect(section).toContainText('помечено спорным');

    // 11. The new client event: the stale badge with the honest counters
    // (the sources moved — the summary NEVER mutates; the update is the
    // GET link of the #decision-context anchor).
    const secondTurn = await postIngress(api, textInboundEvent('journey-ctx-evt-2', 'journey-ctx-sess-1', { say: 'когда доставка?' }), 'journey-ctx-key-2');
    expect(secondTurn.status()).toBe(201);
    await page.goto(pathA);
    await expect(page.locator('#ctx-summary-staleness')).toContainText('Источники изменились после сводки.');
    await expect(page.locator('#ctx-summary-staleness')).toContainText('новых событий: 1');

    // 12. The refresh by the anchor: the «Обновить контекст» link
    // navigates to the page WITH the #decision-context anchor — the
    // section stays in place (the position of the section survives, the
    // exact scroll position does not — the no-JS boundary).
    const refreshLink = page.locator('#decision-context a.primary[href$="#decision-context"]');
    await expect(refreshLink).toHaveCount(1);
    await refreshLink.click();
    await expect(page).toHaveURL(new RegExp(`${caseA}#decision-context$`));
    await expect(page.locator('#decision-context')).toBeVisible();
    await expect(page.locator('#ctx-summary-staleness')).toContainText('новых событий: 1');

    // 13. The recommendation (AC-7): the plan word of the last outcome
    // with the mandatory checks and the mode constraint — the projection
    // never grants an authority.
    await expect(section).toContainText('Рекомендуемый шаг');
    await expect(section).toContainText('Причина:');
    await expect(section).toContainText('Ограничения режима: human_requested');
    await expect(section).toContainText('не команда и не полномочие');

    // 14. The control branch: a FRESH case without the open handoff and
    // without the summary renders NO section (the honest absence — the
    // byte-for-byte page of the previous stories).
    await api.post('/fixture/operator', { form: { second: '' } });
    const control = await postIngress(api, textInboundEvent('journey-ctx-evt-3', 'journey-ctx-sess-3', { say: 'спасибо' }), 'journey-ctx-key-3');
    expect(control.status()).toBe(201);
    const caseB = (await control.json()).case_id;
    await page.goto(`${casesPath}/${caseB}`);
    await expect(page.locator('#decision-context')).toHaveCount(0);
    await expect(page.locator('#decision-context-note')).toHaveCount(0);

    // 15. The a11y close of the operator page: the polite live regions
    // of the section note and the staleness badge, the ≥44×44 target of
    // the refresh link, the semantic headings and the 320px single
    // column (no horizontal scroll).
    await page.goto(pathA);
    await expect(page.locator('#decision-context-note')).toHaveAttribute('aria-live', 'polite');
    await expect(page.locator('#ctx-summary-staleness')).toHaveAttribute('aria-live', 'polite');
    await expect(page.getByRole('heading', { level: 2, name: 'Контекст передачи' })).toHaveCount(1);
    await expect(page.locator('#decision-context h3').first()).toHaveText('Что происходит');
    const box = await page.locator('#decision-context a.primary').boundingBox();
    expect(box!.height).toBeGreaterThanOrEqual(44);
    expect(box!.width).toBeGreaterThanOrEqual(44);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    await page.close();
  }
});
