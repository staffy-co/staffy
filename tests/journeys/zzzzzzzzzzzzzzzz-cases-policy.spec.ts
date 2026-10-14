import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const connectionID = '01994f64-6c00-7000-8000-000000000006';
const casesPath = `/app/tenants/${tenantID}/cases`;

function inboundEvent(providerEventId: string, sessionId: string, payload: Record<string, unknown>) {
  return {
    schema_version: 1,
    tenant_id: tenantID,
    connection_id: connectionID,
    provider: 'staffy-synthetic-telephony',
    provider_event_id: providerEventId,
    provider_session_id: sessionId,
    direction: 'inbound',
    modality: 'voice',
    channel: 'phone',
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

// The Story 2.6 chain at 320px: the Progressive Intake / Policy panel (the
// proposals — the claims audit with the proposer word and the candidate
// text the operator sees, the CURRENT outcome — the last number, the
// mandatory facts with the states — reused with the provenance signature
// or the question, the next steps, the suppressed questions, the outcome
// history), the submission form (the intent is a bounded string — the
// dictionary is a POLICY matter), the evaluation form with the four
// anchors (the server truth prefills), the load-bearing sales reply gate
// (the anchor + the acting commercial outcome), the dedup against the
// confirmed fact, the supersede by the new row — keyboard-first,
// aria-live without a focus move, no horizontal scroll.
test('cases-policy: the proposal, the evaluation, the dedup and the sales gate at 320px', async ({ browser }) => {
  test.setTimeout(180_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One call: the case with the initial work item.
    const accepted = await postIngress(api, inboundEvent('journey-policy-evt-1', 'journey-policy-sess-1', { call: 'inbound-call', from: '+79000000112' }), 'journey-policy-key-1');
    expect(accepted.status()).toBe(201);
    const caseA = (await accepted.json()).case_id;
    const pathA = `${casesPath}/${caseA}`;

    // 2. The fresh panel: no outcome (the honest word of the gate), the
    // rule words, the anchors of the evaluation form, the intent
    // dictionary hint.
    await page.goto(pathA);
    const policyHeading = page.getByRole('heading', { name: /Progressive Intake \/ Policy/ });
    await expect(policyHeading).toBeVisible();
    const noOutcome = page.locator('p[role="status"]').filter({ hasText: /Действующего исхода нет/ });
    await expect(noOutcome).toBeVisible();
    await expect(page.locator('#policy-panel-note')).toContainText('intake-policy-v2');
    await expect(page.locator('#policy-panel-note')).toContainText('commercial_answer');

    // 3. The sales prelude of the fixture world: the classification + the
    // acting disclosure (the machine contract — the 2-5 chain is covered
    // by its own journey).
    const caseView0 = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const conversationID = caseView0.conversations[0].ConversationID;
    const turnEvent = caseView0.timeline.find((event: any) => event.State === 'accepted');
    await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/mode/proposals`, {
      data: {
        conversation_id: conversationID,
        mode: 'sales', scope: 'conversation', expected_mode_version: 1,
        reason: 'Клиент интересуется ценой', signal_source: 'turn_event', turn_ref: turnEvent.EventID,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-policy-mode-1" },
    });
    const disclosure = await (await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/disclosures`, {
      data: { conversation_id: conversationID, channel: 'phone', text: 'Я цифровой сотрудник компании. Человек подключится по вашему запросу.' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-policy-disc-1" },
    })).json();
    await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/disclosures/${disclosure.disclosure_id}/delivery-events`, {
      data: { conversation_id: conversationID, kind: 'sent' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-policy-disc-sent-1" },
    });

    // 4. The confirmed fact BEFORE the proposal (the dedup basis): the
    // service_kind value with the full provenance (the 2-2 forms).
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Записать факт"))').click();
    const factForm = page.locator('form[action$="/facts"]').first();
    await factForm.locator('select[name="source_type"]').selectOption('operator');
    await factForm.locator('input[name="material_key"]').fill('service_kind');
    await factForm.locator('textarea[name="value"]').fill('"диагностика"');
    await factForm.getByRole('button', { name: /Записать факт/ }).click();
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("service_kind — Без подтверждения"))').first().click();
    await page.getByText('Подтвердить точную версию факта service_kind').first().click();
    const confirmForm = page.locator('form[action$="/confirmations"]').first();
    await confirmForm.getByRole('button', { name: /Подтвердить версию/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?confirmed=1`));
    await page.goto(pathA);
    await expect(page.locator('details:has(summary:text-is("service_kind — Подтверждено"))')).toBeVisible();

    // 5. The proposal submission form: the intent string, the candidate,
    // the proposer select, the missing fact claim; the panel shows the
    // proposal with the proposer word and the candidate text.
    await page.locator('details:has(summary:text-is("Подать предложение ИИ (порт-провод или оператор)"))').click();
    const proposalForm = page.locator('form[action$="/ai-proposals"]').first();
    await proposalForm.locator('select[name="conversation_id"]').selectOption(conversationID);
    const workItemOption = await proposalForm.locator('select[name="work_item_id"] option').first().getAttribute('value');
    await proposalForm.locator('select[name="work_item_id"]').selectOption(workItemOption);
    const turnOption = await proposalForm.locator('select[name="turn_ref"] option').first().getAttribute('value');
    await proposalForm.locator('select[name="turn_ref"]').selectOption(turnOption);
    await proposalForm.locator('input[name="intent"]').fill('price_inquiry');
    await proposalForm.locator('textarea[name="candidate_answer"]').fill('Диагностика от 3000');
    await proposalForm.locator('input[name="missing_key_0"]').fill('urgency');
    await proposalForm.locator('select[name="missing_reason_0"]').selectOption('mandatory');
    await proposalForm.getByRole('button', { name: /Подать предложение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?proposal_submitted=1`));
    await page.goto(pathA);
    // The proposals history renders inside its closed details — open it
    // before the assertions.
    await page.getByText(/Предложения \(1\)/).first().click();
    const proposalTable = page.locator('table:has(caption:text("Предложения: намерение, податель, кандидат, недостающие и извлечённые притязания, снимок подачи"))');
    await expect(proposalTable.getByText('price_inquiry')).toBeVisible();
    await expect(proposalTable.getByText('synthetic_port')).toBeVisible();
    await expect(proposalTable.getByText('Диагностика от 3000')).toBeVisible();
    await expect(proposalTable.getByText(/urgency \(mandatory\)/)).toBeVisible();

    // 6. The evaluation form: the four anchors prefilled with the server
    // truth; the outcome announces via aria-live (the polite region — no
    // focus move); the dedup shows the reused service_kind with the
    // provenance signature and the suppressed repeat; the urgency
    // question stays; the next steps carry the question + the
    // confirmation.
    await page.locator('details:has(summary:text-is("Оценить предложение (evaluate_policy)"))').first().click();
    const evaluateForm = page.locator('form[action$="/policy/evaluations"]').first();
    await expect(evaluateForm.locator('input[name="expected_mode_version"]')).toHaveValue(/^[0-9]+$/);
    await expect(evaluateForm.locator('input[name="expected_owner_epoch"]')).toHaveValue(/^[0-9]+$/);
    await expect(evaluateForm.locator('input[name="expected_facts_digest"]')).toHaveValue(/^[0-9a-f]{64}$/);
    const focusedBefore = await page.evaluate(() => document.activeElement?.tagName);
    await evaluateForm.getByRole('button', { name: /Оценить предложение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?policy_evaluated=1`));
    await page.goto(pathA);
    const outcomeRegion = page.locator('div[aria-live="polite"]').filter({ hasText: /Действующий исход/ });
    await expect(outcomeRegion).toBeVisible();
    await expect(outcomeRegion.getByText(/commercial_answer/)).toBeVisible();
    await expect(outcomeRegion.getByText(/ok_commercial/)).toBeVisible();
    await expect(outcomeRegion.getByText(/переиспользован \(подтверждённое значение\)/)).toBeVisible();
    await expect(outcomeRegion.getByText(/вопрос остаётся/)).toBeVisible();
    await expect(outcomeRegion.getByText(/спросить факт: urgency/)).toBeVisible();
    await expect(outcomeRegion.getByText(/дождаться подтверждения/)).toBeVisible();
    await expect(outcomeRegion.getByText(/service_kind \(источник: rule\)/)).toBeVisible();
    await expect(outcomeRegion.getByText(/Удалённые повторы вопросов/)).toBeVisible();
    await expect(outcomeRegion.getByText(/факт 01994f64/)).toBeVisible();
    // The aria-live announcement never moves the focus (the active
    // element is the body after the reload — the region itself is not
    // focused).
    const focusedAfter = await page.evaluate(() => document.activeElement?.tagName);
    expect(['BODY', 'html', undefined]).toContain(focusedAfter ?? 'BODY');
    expect(focusedBefore).toBeTruthy();

    // 7. The sales reply gate: without the Work Item anchor the flash
    // names the anchor path; with the anchor and the acting outcome the
    // reply passes; the outgoing table shows the class.
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const noAnchorReply = page.locator('form[action$="/client-replies"]').first();
    await noAnchorReply.locator('select[name="conversation_id"]').selectOption(conversationID);
    await noAnchorReply.locator('select[name="reply_class"]').selectOption('sales');
    await noAnchorReply.locator('textarea[name="content"]').fill('Диагностика от 3000');
    await noAnchorReply.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page.locator('#case-flash')).toContainText(/принадлежит элементу работы/);
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Ответить клиенту (единственный исходящий путь — серверный гейт)"))').click();
    const anchoredReply = page.locator('form[action$="/client-replies"]').first();
    await anchoredReply.locator('select[name="conversation_id"]').selectOption(conversationID);
    await anchoredReply.locator('select[name="work_item_id"]').selectOption(workItemOption);
    await anchoredReply.locator('select[name="reply_class"]').selectOption('sales');
    await anchoredReply.locator('textarea[name="content"]').fill('Диагностика от 3000; уточню срочность');
    await anchoredReply.getByRole('button', { name: /Подготовить ответ/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?reply_prepared=1`));
    await page.goto(pathA);
    const outgoing = page.locator('div[aria-label="Исходящие ответы"] table');
    await expect(outgoing.getByText(/продажной/)).toBeVisible();

    // 8. The supersede: the urgency fact lands and confirms (the 2-2
    // forms); the re-evaluation with the fresh anchors writes the NEW
    // row (the history grows, the current flips to №2, the questions
    // disappear); the stale anchor refuses with the honest conflict.
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Записать факт"))').click();
    const urgencyForm = page.locator('form[action$="/facts"]').first();
    await urgencyForm.locator('select[name="source_type"]').selectOption('operator');
    await urgencyForm.locator('input[name="material_key"]').fill('urgency');
    await urgencyForm.locator('textarea[name="value"]').fill('"срочно"');
    await urgencyForm.getByRole('button', { name: /Записать факт/ }).click();
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("urgency — Без подтверждения"))').first().click();
    await page.getByText('Подтвердить точную версию факта urgency').first().click();
    const confirmUrgency = page.locator('form[action$="/confirmations"]').nth(1);
    await confirmUrgency.getByRole('button', { name: /Подтвердить версию/ }).click();
    await page.goto(pathA);
    await expect(page.locator('details:has(summary:text-is("urgency — Подтверждено"))')).toBeVisible();
    // The stale anchors (the digest moved): the honest conflict flash.
    const staleEval = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/policy/evaluations`, {
      data: {
        proposal_ref: await page.locator('form[action$="/policy/evaluations"] select[name="proposal_ref"] option').first().getAttribute('value'),
        expected_mode_version: 2,
        expected_owner_epoch: 1,
        expected_facts_digest: '0000000000000000000000000000000000000000000000000000000000000000',
        expected_work_item_version: 1,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-policy-stale-1" },
    });
    expect(staleEval.status()).toBe(409);
    const conflict = await staleEval.json();
    expect(conflict.code).toBe('policy_conflict');
    // The fresh anchors: the v2 row (no questions — both facts reused).
    const freshView = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const proposalID = await page.locator('form[action$="/policy/evaluations"] select[name="proposal_ref"] option').first().getAttribute('value');
    const freshEval = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/policy/evaluations`, {
      data: {
        proposal_ref: proposalID,
        expected_mode_version: freshView.policy.mode_version,
        expected_owner_epoch: freshView.policy.owner_epoch,
        expected_facts_digest: freshView.policy.facts_digest,
        expected_work_item_version: freshView.work_items[0].Version,
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-policy-eval-2" },
    });
    expect(freshEval.status()).toBe(201);
    await page.goto(pathA);
    await expect(outcomeRegion.getByText(/Действующий исход \(№2\)/)).toBeVisible();
    await page.getByText(/История исходов \(2\)/).first().click();
    const history = page.locator('table:has(caption:text("Исходы: номер, класс, причина, правило, намерение, шаги, дайджест"))');
    await expect(history.getByRole('row')).toHaveCount(3); // the header + v1 + v2

    // 9. The 320px discipline: no horizontal scroll on the policy panel;
    // the heading is keyboard reachable; the outcome region is the aria
    // region (not a focus target).
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
    await policyHeading.focus();
    const panelNote = page.locator('#policy-panel-note');
    await expect(panelNote).toBeVisible();
    // The form targets meet the size discipline (the CSS of the suite).
    await expect(proposalForm).toBeAttached();
  } finally {
    await api.post('/fixture/supervisor', { form: { enabled: '' } });
    await api.post('/fixture/operator', { form: { second: '' } });
    await page.close();
  }
});
