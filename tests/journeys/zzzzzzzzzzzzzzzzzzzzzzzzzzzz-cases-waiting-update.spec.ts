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

// The Story 2.16 chain at 320px: the client's ask (the unconditional
// request of the human), the control branch of the ZERO waiting state
// (the page byte-identical to 2.13/2.14/2.15 — the «Ожидание человека»
// block appears only with the promise material), the operator promises
// the 1h update through the PRG form (the honest status line of the
// landing, the first client distinction «ищем специалиста», the
// promise line №1 with the server-computed deadline word), the
// assignment flips the second distinction («назначен ≠ принял»), the
// machine promise of the honest unknown word (the 201, the replay 200
// with the Idempotency-Replayed header, the key conflict 409, the
// strict decode refusal of the server-truth field 400 — a fabricated
// timestamp NEVER passes), the promise line №2 without the promised
// time (the material honesty of FR-41), the assignee reads the promise
// line BEFORE the acknowledgement (the shared material of the round),
// the acknowledgement flips the third distinction («специалист принял
// обращение»), the new 1h promise of the accepted round (the supersede
// derivation-only — the former rows live, the LAST speaks), the
// fixture-clock shift past the promised time (the honest read-time
// missed word with the next step «existing reply path» — NEVER a
// timer, NEVER an automatic send; the round stays acknowledged — the
// promise NEVER resolves or touches the round), the honest reset of
// the fixture clock, the control branch of the fresh case (zero
// waiting artifacts) and the a11y close (the polite live regions, the
// role=status markers, the ≥44×44 form target, the 320px column
// without the horizontal scroll).
test('cases-waiting-update: the honest promise with the deadline word, the three client distinctions, the unknown word, the read-time missed and the a11y at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One TEXT turn of one web_chat session — the anchor of the ask
    // (the accepted event the request cites).
    const first = await postIngress(api, textInboundEvent('journey-wu-evt-1', 'journey-wu-sess-1', { say: 'позовите человека' }), 'journey-wu-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const turnOne = (await first.json()).event_id;
    const pathA = `${casesPath}/${caseA}`;

    // 2. The control branch BEFORE any promise: the case with the ask
    // but WITHOUT promises renders the ZERO waiting view — no
    // «Ожидание человека» block, no PRG status line (the page renders
    // byte-identically to the 2.13 surface; the promise form dictionary
    // renders in the operator area — the material contributes NOTHING).
    await page.goto(pathA);
    await expect(page.locator('#human-waiting')).toHaveCount(0);
    await expect(page.locator('#waiting-promised')).toHaveCount(0);
    await expect(page.locator('#waiting-missed')).toHaveCount(0);
    await expect(page.locator('#handoffs-current')).toBeVisible();

    // 3. The unconditional ask through the machine path (the ack window
    // of 4h — the acceptance never races the update promise of the
    // chain: the two deadlines are two different promises).
    const ask = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests`, {
      data: {
        turn_ref: turnOne,
        expected_mode_version: 1,
        target_roles: ['operator'],
        ack_deadline: 'ack_4h',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-wu-ask-1" },
    });
    expect(ask.status()).toBe(201);

    // 4. The operator promises the 1h update through the PRG form: the
    // deadline select of the fixed dictionary (the honest unknown word
    // included — no silent defaults), the submit lands on the #handoff
    // anchor with the honest status line, the first client distinction
    // «ищем специалиста» (the round is requested — the human is not
    // connected yet) and the promise line №1 with the deadline word
    // text and the pending state.
    await page.locator('details:has(summary:text-is("Обещать обновление (promise_handoff_update — честный срок или честное неизвестное)"))').first().click();
    const promiseForm = page.locator('form[action$="/update-promises"]').first();
    await promiseForm.locator('select[name="deadline_word"]').selectOption('update_1h');
    await promiseForm.getByRole('button', { name: 'Обещать обновление' }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?update_promised=1`));
    await expect(page.locator('#waiting-promised')).toContainText('Обещание обновления записано (append-only)');
    await expect(page.locator('#human-waiting')).toBeVisible();
    await expect(page.locator('#waiting-status')).toContainText('Ищем специалиста');
    await expect(page.locator('#waiting-promise')).toContainText('Обещание обновления №1');
    await expect(page.locator('#waiting-promise')).toContainText('Обновление обещано: в течение часа');
    await expect(page.locator('#waiting-promise')).toContainText('Ожидаем обновления к обещанному сроку');
    await expect(page.locator('#waiting-client-line')).toContainText('Клиенту: Обновление обещано: в течение часа');

    // 5. The assignment flips the SECOND distinction («назначен ≠
    // принял» — two honest words, never one); the promise line keeps
    // its own truth (the waiting status and the promise state are two
    // different derivations over the same material).
    const assignment = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests/assignment`, {
      data: {
        assignee: secondOperatorID,
        backup: ownerID,
        ack_deadline: 'ack_4h',
      },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-wu-assign-1" },
    });
    expect(assignment.status()).toBe(201);
    await page.goto(pathA);
    await expect(page.locator('#waiting-status')).toContainText('Специалист назначен: ожидаем принятия');
    await expect(page.locator('#waiting-promise')).toContainText('Обещание обновления №1');

    // 6. The machine promise of the honest unknown word: the fresh
    // record is the 201 (the outcome carries the command word with the
    // deadline word ONLY — the promised time and the promise number are
    // the server truths), the exact repeat the 200 with the
    // Idempotency-Replayed header, the same key with ANOTHER word the
    // honest 409, and the strict decode refusal of the server-truth
    // field the honest 400 (a fabricated promise number NEVER passes —
    // the server computes it under the locks).
    const unknownBody = { deadline_word: 'update_unknown' };
    const unknown = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/update-promises`, {
      data: unknownBody,
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-wu-unknown-1" },
    });
    expect(unknown.status()).toBe(201);
    const unknownOutcome = await unknown.json();
    expect(unknownOutcome.command).toBe('promise_handoff_update');
    expect(unknownOutcome.promise_deadline_word).toBe('update_unknown');
    expect(unknownOutcome.promise_no).toBeUndefined();
    const unknownReplay = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/update-promises`, {
      data: unknownBody,
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-wu-unknown-1" },
    });
    expect(unknownReplay.status()).toBe(200);
    expect(unknownReplay.headers()['idempotency-replayed']).toBe('true');
    const conflict = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/update-promises`, {
      data: { deadline_word: 'update_1h' },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-wu-unknown-1" },
    });
    expect(conflict.status()).toBe(409);
    const fabricated = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/update-promises`, {
      data: { deadline_word: 'update_1h', promise_no: 3 },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-wu-fabricated-1" },
    });
    expect(fabricated.status()).toBe(400);

    // 7. The promise line №2 of the honest unknown word: the promised
    // time is NOT named (the material honesty pair of FR-41 — the
    // honest «срок неизвестен» NEVER carries a fabricated timestamp),
    // the state «Обновление ожидается», the LAST promise speaks (the
    // №1 row lives append-only — the superseded derivation).
    await page.goto(pathA);
    await expect(page.locator('#waiting-promise')).toContainText('Обещание обновления №2');
    await expect(page.locator('#waiting-promise')).toContainText('Срок следующего обновления неизвестен');
    await expect(page.locator('#waiting-promise')).toContainText('Обещанный срок не называется: честное неизвестное, без выдуманного времени');
    await expect(page.locator('#waiting-promise')).toContainText('Обновление ожидается');

    // 8. The assignee reads the promise line BEFORE the acknowledgement
    // (the waiting view is the shared material of the round — the
    // promise never waits for the acceptance; the assignment card of
    // the assignee is the 2.13 surface).
    await api.post('/fixture/operator', { form: { second: '1' } });
    await page.goto(pathA);
    await expect(page.locator('#waiting-promise')).toContainText('Обещание обновления №2');
    await expect(page.locator('#waiting-status')).toContainText('Специалист назначен: ожидаем принятия');

    // 9. The acknowledgement flips the THIRD distinction («специалист
    // принял обращение» — the human works; the updates are still
    // promised while the round lives).
    const ack = await api.post(`/v1/tenants/${tenantID}/cases/${caseA}/human-requests/acknowledgement`, {
      data: { expected_epoch: 1 },
      headers: { "Content-Type": "application/json", "Idempotency-Key": "journey-wu-ack-1" },
    });
    expect(ack.status()).toBe(201);
    await api.post('/fixture/operator', { form: { second: '' } });
    await page.goto(pathA);
    await expect(page.locator('#waiting-status')).toContainText('Специалист принял обращение');

    // 10. The new 1h promise of the accepted round through the PRG form
    // (the updates are still owed while the human works): the promise
    // №3 derives pending — the LAST promise speaks; the №1/№2 rows live
    // append-only, the supersede is the read-time derivation.
    await page.locator('details:has(summary:text-is("Обещать обновление (promise_handoff_update — честный срок или честное неизвестное)"))').first().click();
    const healForm = page.locator('form[action$="/update-promises"]').first();
    await healForm.locator('select[name="deadline_word"]').selectOption('update_1h');
    await healForm.getByRole('button', { name: 'Обещать обновление' }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?update_promised=1`));
    await expect(page.locator('#waiting-promise')).toContainText('Обещание обновления №3');
    await expect(page.locator('#waiting-promise')).toContainText('Ожидаем обновления к обещанному сроку');
    await expect(page.locator('#waiting-missed')).toHaveCount(0);

    // 11. The fixture-clock shift past the promised time: the missed
    // word is the READ-TIME truth of the moment (the row and the round
    // NEVER mutate — no timer, no automatic send, no escalation); the
    // honest next step names the existing reply path (the update is the
    // EXTERNAL producer — the human sends it); the client status keeps
    // the third distinction (the missed update NEVER touches the round
    // — AC-3: the promise never resolves the handoff).
    await api.post('/fixture/clock', { form: { minutes: '90' } });
    await page.goto(pathA);
    await expect(page.locator('#waiting-promise')).toContainText('Обновление просрочено — решения ещё нет');
    await expect(page.locator('#waiting-missed')).toContainText('Следующий шаг: сообщите статус существующим ответом клиенту');
    await expect(page.locator('#waiting-client-line')).toContainText('Обновление просрочено');
    await expect(page.locator('#waiting-status')).toContainText('Специалист принял обращение');
    // The a11y of the missed word: the polite live region and the
    // role=status marker (the honest announcement without a reload).
    await expect(page.locator('#waiting-note')).toHaveAttribute('aria-live', 'polite');
    await expect(page.locator('#waiting-missed[role="status"]')).toHaveCount(1);

    // 12. The honest reset of the fixture clock (the fixture clock is
    // the shared server truth — the later journeys read the same fixed
    // moment the commands were stamped with).
    await api.post('/fixture/clock', { form: { minutes: '0' } });

    // 13. The control branch: a FRESH case without the ask renders ZERO
    // waiting artifacts (the byte-identical page of the previous
    // stories — the block appears only with the promise material).
    const control = await postIngress(api, textInboundEvent('journey-wu-evt-2', 'journey-wu-sess-2', { say: 'спасибо' }), 'journey-wu-key-2');
    expect(control.status()).toBe(201);
    const caseB = (await control.json()).case_id;
    await page.goto(`${casesPath}/${caseB}`);
    await expect(page.locator('#human-waiting')).toHaveCount(0);
    await expect(page.locator('#waiting-promised')).toHaveCount(0);
    await expect(page.locator('#waiting-missed')).toHaveCount(0);

    // 14. The a11y close of the operator page: the polite live regions
    // of the waiting block, the role=status markers, the ≥44×44 target
    // of the promise form button and the 320px single column (no
    // horizontal scroll). The clock is reset — the missed marker is
    // absent again (the missed word is the truth of the MOMENT, never a
    // stored status).
    await page.goto(pathA);
    await expect(page.locator('#waiting-note')).toHaveAttribute('aria-live', 'polite');
    await expect(page.locator('#waiting-status')).toHaveAttribute('aria-live', 'polite');
    await expect(page.locator('#waiting-client-line')).toHaveAttribute('aria-live', 'polite');
    await expect(page.locator('#waiting-promise[role="status"]')).toHaveCount(1);
    await expect(page.locator('#waiting-missed')).toHaveCount(0);
    await page.locator('details:has(summary:text-is("Обещать обновление (promise_handoff_update — честный срок или честное неизвестное)"))').first().click();
    const submitBox = await page.locator('form[action$="/update-promises"] button.primary').first().boundingBox();
    expect(submitBox!.height).toBeGreaterThanOrEqual(44);
    expect(submitBox!.width).toBeGreaterThanOrEqual(44);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    await api.post('/fixture/clock', { form: { minutes: '0' } });
    await api.post('/fixture/operator', { form: { second: '' } });
    await page.close();
  }
});
