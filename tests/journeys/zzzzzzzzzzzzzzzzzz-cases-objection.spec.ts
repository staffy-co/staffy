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

// The Story 2.8 chain at 320px: the «Возражения и бюджет убеждения» panel
// (the honest empty state, the playbook version word, the record form with
// the FIXED dictionaries — three type words, four signal words), the PRG
// record of the objection over the accepted TEXT turn (the verbatim span
// excerpt — the byte-exact words of the client), the lifetime refusal
// budget progression (0 → 1 → 2 of 2) with the honest STOP badge of the
// stopped persuasion (the allowed endings, never the color alone), the
// span honesty refusal (the invented excerpt never lands), the aria-live
// announcement region and the no-horizontal-scroll guarantee.
test('cases-objection: the record, the budget and the persuasion stop at 320px', async ({ browser }) => {
  test.setTimeout(180_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One call: the case with the initial work item over the TEXT
    // (web_chat) turn — the objection source demands the text modality.
    const accepted = await postIngress(api, textInboundEvent('journey-objection-evt-1', 'journey-objection-sess-1', { say: 'Мне кажется дорого, у конкурента дешевле' }), 'journey-objection-key-1');
    expect(accepted.status()).toBe(201);
    const body = await accepted.json();
    const caseA = body.case_id;
    const turnOne = body.event_id;
    const pathA = `${casesPath}/${caseA}`;
    // The session continuation: the second TEXT turn of the same case
    // (the refusal words of the budget progression).
    const second = await postIngress(api, textInboundEvent('journey-objection-evt-2', 'journey-objection-sess-1', { say: 'Не буду брать' }), 'journey-objection-key-2');
    expect(second.status()).toBe(201);
    const secondBody = await second.json();
    const turnTwo = secondBody.event_id;

    // 2. The fresh panel: the heading, the playbook version word and the
    // honest empty state (the budget is not spent — nothing is stopped).
    await page.goto(pathA);
    await expect(page.getByRole('heading', { name: /Возражения и бюджет убеждения/ })).toBeVisible();
    await expect(page.locator('.next-note').filter({ hasText: /objection-playbook-v1/ })).toBeVisible();
    await expect(page.locator('div[aria-live="polite"]').filter({ hasText: /Возражений на элементе нет/ })).toBeVisible();

    // 3. The record form: the FIXED dictionaries — three type words and
    // four signal words (never invented options); the turn select carries
    // the accepted text events; the span input is the verbatim excerpt.
    await page.locator('details:has(summary:text-is("Записать возражение клиента (record_objection)"))').first().click();
    const recordForm = page.locator('form[action$="/objections"]').first();
    await expect(recordForm.locator('select[name="objection_type"] option')).toHaveCount(3);
    await expect(recordForm.locator('select[name="client_signal"] option')).toHaveCount(4);
    await recordForm.locator('select[name="turn_ref"]').selectOption(turnOne);
    await recordForm.locator('select[name="objection_type"]').selectOption('price_comparison');
    await recordForm.locator('select[name="client_signal"]').selectOption('concern');
    await recordForm.locator('input[name="span_1"]').fill('дорого');
    await recordForm.getByRole('button', { name: /Записать возражение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?objection_recorded=1`));

    // 4. The current objection card: the №1 with the type and the signal
    // words, the verbatim span excerpt (the blockquote of the CLIENT's
    // words), the budget line «отказов 0 из 2» — the concern does not
    // spend the refusal budget, nothing is stopped yet.
    await page.goto(pathA);
    const objectionCard = page.locator('details').filter({ has: page.locator('summary', { hasText: /^Возражение №1 — price_comparison/ }) }).first();
    await objectionCard.locator('summary').click();
    await expect(objectionCard).toContainText('сигнал: concern (опасение — убеждение продолжается)');
    await expect(objectionCard.locator('blockquote').filter({ hasText: /^дорого$/ })).toBeVisible();
    await expect(objectionCard).toContainText('отказов 0 из 2');
    await expect(objectionCard).not.toContainText('Убеждение остановлено');

    // 5. The first refusal: the budget moves to 1 of 2 — the persuasion
    // continues (the honest threshold is two refusal records, lifetime).
    await page.locator('details:has(summary:text-is("Записать возражение клиента (record_objection)"))').first().click();
    const refusalForm = page.locator('form[action$="/objections"]').first();
    await refusalForm.locator('select[name="turn_ref"]').selectOption(turnTwo);
    await refusalForm.locator('select[name="objection_type"]').selectOption('general_doubt');
    await refusalForm.locator('select[name="client_signal"]').selectOption('refusal');
    await refusalForm.locator('input[name="span_1"]').fill('Не буду');
    await refusalForm.getByRole('button', { name: /Записать возражение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?objection_recorded=1`));
    await page.goto(pathA);
    const refusalCard = page.locator('details').filter({ has: page.locator('summary', { hasText: /^Возражение №2 — general_doubt/ }) }).first();
    await refusalCard.locator('summary').click();
    await expect(refusalCard).toContainText('refusal (отказ — считает бюджет)');
    await expect(refusalCard).toContainText('отказов 1 из 2');
    await expect(refusalCard).not.toContainText('Убеждение остановлено');

    // 6. The second refusal: the budget is spent (2 of 2) — the honest
    // STOP badge with the allowed endings (the neutral completion, the
    // privacy request, the human handoff — never the color alone: the
    // words travel in the alert text).
    await page.locator('details:has(summary:text-is("Записать возражение клиента (record_objection)"))').first().click();
    const stopForm = page.locator('form[action$="/objections"]').first();
    await stopForm.locator('select[name="turn_ref"]').selectOption(turnTwo);
    await stopForm.locator('select[name="objection_type"]').selectOption('general_doubt');
    await stopForm.locator('select[name="client_signal"]').selectOption('refusal');
    await stopForm.locator('input[name="span_1"]').fill('брать');
    await stopForm.getByRole('button', { name: /Записать возражение/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?objection_recorded=1`));
    await page.goto(pathA);
    const stoppedCard = page.locator('details').filter({ has: page.locator('summary', { hasText: /^Возражение №3 — general_doubt/ }) }).first();
    await stoppedCard.locator('summary').click();
    await expect(stoppedCard).toContainText('отказов 2 из 2');
    const stopAlert = page.locator('div[aria-live="polite"] .alert').filter({ hasText: /Убеждение остановлено/ });
    await expect(stopAlert).toBeVisible();
    for (const ending of ['вежливое завершение', 'запрос приватности', 'передача человеку']) {
      await expect(stopAlert).toContainText(ending);
    }

    // 7. The span honesty: the INVENTED excerpt never lands — the refusal
    // names the honesty rule (the byte-exact containment against the
    // payload of the turn source; no fabricated evidence is possible).
    await page.locator('details:has(summary:text-is("Записать возражение клиента (record_objection)"))').first().click();
    const honestForm = page.locator('form[action$="/objections"]').first();
    await honestForm.locator('select[name="turn_ref"]').selectOption(turnTwo);
    await honestForm.locator('select[name="objection_type"]').selectOption('trust');
    await honestForm.locator('select[name="client_signal"]').selectOption('concern');
    await honestForm.locator('input[name="span_1"]').fill('этой фразы точно нет в payload');
    await honestForm.getByRole('button', { name: /Записать возражение/ }).click();
    // The PRG redirect carries the flash of the honest refusal (the
    // error text shows on the landing page — the one-shot flash).
    await expect(page).toHaveURL(new RegExp(`${caseA}(\\?|$)`));
    await expect(page.locator('#case-flash')).toContainText('не входит в слова клиента');
    // The objection count stays three: the dishonest record never landed.
    await page.goto(pathA);
    const afterRefusalCard = page.locator('details').filter({ has: page.locator('summary', { hasText: /^Возражение №3 — general_doubt/ }) }).first();
    await afterRefusalCard.locator('summary').click();
    await expect(afterRefusalCard).toContainText('отказов 2 из 2');

    // 8. The reply form carries the playbook step select (the five
    // dictionary words of the sales discipline + the honest empty option).
    const replyForm = page.locator('form[action$="/client-replies"]').first();
    await expect(replyForm.locator('select[name="playbook_step"] option')).toHaveCount(6);

    // 9. The aria-live announcement region of the objection panel (the
    // state changes never move the focus) and no horizontal scroll at
    // 320px (the long span excerpts and the carrier words wrap).
    const liveRegion = page.locator('div[aria-live="polite"]').filter({ hasText: /Возражение №/ });
    await expect(liveRegion.first()).toBeAttached();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    await page.close();
  }
});
