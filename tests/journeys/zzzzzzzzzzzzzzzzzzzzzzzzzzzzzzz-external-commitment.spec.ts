import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const foreignTenantID = '01994f64-6c00-7000-8000-00000000a212';
const chatConnectionID = '01994f64-6c00-7000-8000-000000000016';

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

// The Story 3-3 chain at 320px: the honest empty world FIRST (the page
// names the absence and the discipline of the slice), the envelope
// lifecycle through the REAL page forms over the fixture world (the
// create with the explicit stable id, the enqueue refusal without the
// approval, the step-up approval of the EXACT version, the
// persist-before-emit queue with the outbox evidence words, the delivered
// two-event truth of ONE dispatch call, the operator relay of the
// acknowledged observation), the execute-time stale fences by name (the
// superseded queue gases with the «слова не ушли» words, the LIVE
// control pause gases through the enforcement point and the resumed
// accept opens the NEW version path), the timeout of the possible-send
// ambiguity (the recorded no-guarantee word, the structurally absent
// blind retry), the a11y close (the skip-link, the heading ladder, the
// 44×44 targets, the keyboard disclosure), the cross-tenant 404 and the
// no-scope 403 without any envelope material.
test('external-commitment: the envelope lifecycle, the dispatch outcome words, the stale fences and the a11y at 320px', async ({ browser }) => {
  test.setTimeout(240_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One TEXT turn opens the case of the chain (the fixture family
    // precedent: the ingress through the REAL transport, the Work Item of
    // the view material).
    const first = await postIngress(api, textInboundEvent('journey-external-evt-1', 'journey-external-sess-1', { say: 'Обещайте отгрузку запчастей и держите меня в курсе статуса' }), 'journey-external-key-1');
    expect(first.status()).toBe(201);
    const caseA = (await first.json()).case_id;
    const caseView0 = await (await api.get(`/v1/tenants/${tenantID}/cases/${caseA}`)).json();
    const workItemID = caseView0.work_items[0].WorkItemID;
    const reviewPath = `/app/tenants/${tenantID}/cases/${caseA}/external-commitments`;
    const machinePath = `/v1/tenants/${tenantID}/cases/${caseA}/external-commitments`;

    // 2. The honest empty world: the page names the absence and explains
    // the discipline — the operator with review or configuration creates
    // the first version, the words stay inside until the queue.
    await page.goto(reviewPath);
    await expect(page.locator('h1')).toHaveText('Внешние обязательства');
    await expect(page.locator('#external-empty-heading')).toHaveText('Конвертов нет');
    await expect(page.locator('.action-empty .next-note')).toContainText('Внешних обязательств в этом кейсе нет');
    const emptyView = await api.get(machinePath);
    expect(emptyView.status()).toBe(200);
    const emptySections = (await emptyView.json()).external_commitments.items;
    expect(emptySections).toHaveLength(1);
    expect(emptySections[0].versions).toEqual([]);

    // 3. The create lands the version №1 of the stable envelope id: the
    // PRG notice speaks the persisted discipline AND TAKES the focus, the
    // card carries the state word with the fixed render text, the honest
    // non-queue, the non-approval and the ready readiness.
    const extOne = '019a3f33-7000-7000-8000-00000000a101';
    const create = (ext: string, content: string) => ({
      fill: async () => {
        await page.locator('#work_item_id').selectOption(workItemID);
        await page.locator('#ext_commitment_id').fill(ext);
        await page.locator('#purpose').selectOption('promise');
        await page.locator('input[name="recipient_kind"][value="client"]').check();
        await page.locator('#channel').selectOption('phone');
        await page.locator('#content_text').fill(content);
        await page.locator('#valid_until').fill('2027-01-01T00:00:00Z');
        await page.locator('#expected_next_step').fill('оператор запускает диспетчеризацию очереди');
        await page.locator('input[name="ack_requirement"][value="not_required"]').check();
        await page.getByRole('button', { name: 'Создать версию' }).click();
      },
    });
    await create(extOne, 'Обещаем отгрузку заказанных запчастей до конца недели').fill();
    await expect(page).toHaveURL(/\?created=1$/);
    await expect(page.locator('#page-notice')).toContainText('Версия 1 конверта сохранена (persisted)');
    await expect(page.locator('#page-notice')).toBeFocused();
    const cardOne = page.locator('.action-card', { hasText: extOne });
    await expect(cardOne).toHaveCount(1);
    await expect(cardOne).toContainText('Состояние: persisted — Сохранено; отправки не было — в очереди не состоит.');
    await expect(cardOne).toContainText('Очередь: не в очереди');
    await expect(cardOne).toContainText('Утверждение: не утверждена');
    await expect(cardOne).toContainText('Готовность к отправке: ready');

    // 4. The enqueue without the approval is the honest flash refusal
    // with the exact-version words (AC-9): nothing lands in the queue.
    await cardOne.locator('details:has(form[action$="/versions/1/enqueue"]) > summary').click();
    await cardOne.locator('details form[action$="/versions/1/enqueue"] input[name="confirm_enqueue"]').check();
    await cardOne.locator('details form[action$="/versions/1/enqueue"] button.primary').click();
    await expect(page.locator('.alert')).toContainText('Требуется утверждение текущим человеком-владельцем');

    // 5. The step-up approval of the EXACT version opens the queue: the
    // fixture-only step-up switch walks the confirmed identity, the PRG
    // notice speaks the approval and takes the focus, the card carries
    // the approver material.
    await api.post('/fixture/step-up', { form: { enabled: '1' } });
    await cardOne.locator('details:has(form[action$="/versions/1/approve"]) > summary').click();
    await cardOne.locator('details form[action$="/versions/1/approve"] input[name="confirm_approve"]').check();
    await cardOne.locator('details form[action$="/versions/1/approve"] button.primary').click();
    await expect(page).toHaveURL(/\?approved=1$/);
    await expect(page.locator('#page-notice')).toContainText('Версия 1 утверждена текущим человеком-владельцем');
    await expect(page.locator('#page-notice')).toBeFocused();
    await expect(cardOne).toContainText('Утверждение: утверждена:');

    // 6. The persist-before-emit enqueue: the card shows the queue
    // evidence words (the outbox record, the words never left), the state
    // derivation reads queued, the machine view carries the queue flag.
    await cardOne.locator('details:has(form[action$="/versions/1/enqueue"]) > summary').click();
    await cardOne.locator('details form[action$="/versions/1/enqueue"] input[name="confirm_enqueue"]').check();
    await cardOne.locator('details form[action$="/versions/1/enqueue"] button.primary').click();
    await expect(page).toHaveURL(/\?enqueued=1$/);
    await expect(page.locator('#page-notice')).toContainText('поставлена в очередь (queued)');
    await expect(cardOne).toContainText('Очередь: в очереди — outbox запись, слова не уходили');
    await expect(cardOne).toContainText('Состояние: queued — В очереди на отправку');

    // 7. The delivered outcome is the two-event truth of ONE dispatch
    // call (AC-7/AC-8): the attempt evidence with the provider
    // correlation and the RECORDED idempotent guarantee of the live
    // capability declaration, the channel receipt, the materially
    // distinct delivered text (never the acceptance).
    await api.post('/fixture/dispatch-outcome', { form: { outcome: 'delivered' } });
    await page.locator('details:has(form[action$="/cases/external-commitments/drain"]) > summary').click();
    await page.locator('form[action$="/cases/external-commitments/drain"] input[name="confirm_drain"]').check();
    await page.locator('form[action$="/cases/external-commitments/drain"] button.primary').click();
    await expect(page).toHaveURL(/\?drain=1$/);
    await expect(page.locator('#page-notice')).toContainText('Диспетчеризация выполнена: рассмотрено 1 записей');
    await expect(cardOne).toContainText('Состояние: delivered — Доставка подтверждена квитанцией канала; принятия получателем НЕТ.');
    await expect(cardOne).toContainText('№1 attempt_submitted');
    await expect(cardOne).toContainText('корреляция: syn-');
    await expect(cardOne).toContainText('идемпотентная гарантия записана');
    await expect(cardOne).toContainText('№2 receipt_delivered');
    await expect(cardOne).toContainText('квитанция канала: доставка');

    // 8. The operator relay of the acknowledged observation (the 2.12
    // precedent — the human passes the machine observation): the third
    // append-only event, the acceptance as the distinct word.
    await cardOne.locator('details:has(form[action$="/versions/1/dispatch-receipts"]) > summary').click();
    await cardOne.locator('details form[action$="/versions/1/dispatch-receipts"] input[name="observation"][value="acknowledged"]').check();
    await cardOne.locator('details form[action$="/versions/1/dispatch-receipts"] button.primary').click();
    await expect(page).toHaveURL(/\?relayed=1$/);
    await expect(page.locator('#page-notice')).toContainText('Наблюдение канала записано в историю версии 1');
    await expect(cardOne).toContainText('Состояние: acknowledged — Принято получателем — квитанция подтверждает принятие.');
    await expect(cardOne).toContainText('№3 receipt_acknowledged');

    // 9. The execute-time stale fence newer_version: the superseded QUEUE
    // gases with the honest «слова не ушли» words — the append-only event
    // carries the reason material, the state returns to persisted, the
    // NEW version is the current one.
    const extTwo = '019a3f33-7000-7000-8000-00000000a102';
    await create(extTwo, 'Обещаем отгрузку запчастей со склада поставщика в понедельник').fill();
    await expect(page).toHaveURL(/\?created=1$/);
    const cardTwo = page.locator('.action-card', { hasText: extTwo }).filter({ hasText: '— версия 1' });
    await cardTwo.locator('details:has(form[action$="/versions/1/approve"]) > summary').click();
    await cardTwo.locator('details form[action$="/versions/1/approve"] input[name="confirm_approve"]').check();
    await cardTwo.locator('details form[action$="/versions/1/approve"] button.primary').click();
    await expect(page).toHaveURL(/\?approved=1$/);
    await cardTwo.locator('details:has(form[action$="/versions/1/enqueue"]) > summary').click();
    await cardTwo.locator('details form[action$="/versions/1/enqueue"] input[name="confirm_enqueue"]').check();
    await cardTwo.locator('details form[action$="/versions/1/enqueue"] button.primary').click();
    await expect(page).toHaveURL(/\?enqueued=1$/);
    await create(extTwo, 'Обещаем отгрузку запчастей со склада поставщика в понедельник (срок пересогласован)').fill();
    await expect(page).toHaveURL(/\?created=2$/);
    await page.locator('details:has(form[action$="/cases/external-commitments/drain"]) > summary').click();
    await page.locator('form[action$="/cases/external-commitments/drain"] input[name="confirm_drain"]').check();
    await page.locator('form[action$="/cases/external-commitments/drain"] button.primary').click();
    await expect(page).toHaveURL(/\?drain=1$/);
    await expect(cardTwo).toContainText('№1 cancelled_stale — Существует новее версия — слова не ушли.');
    await expect(cardTwo).toContainText('Состояние: persisted — Сохранено; отправки не было');
    await expect(cardTwo).toContainText('Актуальность: заменена новой версией');
    await expect(page.locator('.action-card', { hasText: '— версия 2' })).toContainText('актуальная версия');

    // 10. The LIVE control fence (the promised 3.2 enforcement point):
    // the seeded channel pause gases the queued record with the
    // control_mode words; the lifted control opens the honest recovery —
    // the NEW version of the same envelope passes the resumed accept
    // through the whole queue discipline.
    const extThree = '019a3f33-7000-7000-8000-00000000a103';
    await create(extThree, 'Обещаем выгрузку статуса заказа во внешнюю систему учёта').fill();
    await expect(page).toHaveURL(/\?created=1$/);
    const cardThree = page.locator('.action-card', { hasText: extThree }).filter({ hasText: '— версия 1' });
    const cardThreeV2 = page.locator('.action-card', { hasText: extThree }).filter({ hasText: '— версия 2' });
    await cardThree.locator('details:has(form[action$="/versions/1/approve"]) > summary').click();
    await cardThree.locator('details form[action$="/versions/1/approve"] input[name="confirm_approve"]').check();
    await cardThree.locator('details form[action$="/versions/1/approve"] button.primary').click();
    await expect(page).toHaveURL(/\?approved=1$/);
    await cardThree.locator('details:has(form[action$="/versions/1/enqueue"]) > summary').click();
    await cardThree.locator('details form[action$="/versions/1/enqueue"] input[name="confirm_enqueue"]').check();
    await cardThree.locator('details form[action$="/versions/1/enqueue"] button.primary').click();
    await expect(page).toHaveURL(/\?enqueued=1$/);
    await api.post('/fixture/controls', { form: { enabled: '1' } });
    await page.locator('details:has(form[action$="/cases/external-commitments/drain"]) > summary').click();
    await page.locator('form[action$="/cases/external-commitments/drain"] input[name="confirm_drain"]').check();
    await page.locator('form[action$="/cases/external-commitments/drain"] button.primary').click();
    await expect(page).toHaveURL(/\?drain=1$/);
    await expect(cardThree).toContainText('№1 cancelled_stale — Контроль маршрута запретил отправку — слова не ушли.');
    await api.post('/fixture/controls', { form: { enabled: '' } });
    await create(extThree, 'Обещаем выгрузку статуса заказа во внешнюю систему учёта (после снятия ограничения)').fill();
    await expect(page).toHaveURL(/\?created=2$/);
    await cardThreeV2.locator('details:has(form[action$="/versions/2/approve"]) > summary').click();
    await cardThreeV2.locator('details form[action$="/versions/2/approve"] input[name="confirm_approve"]').check();
    await cardThreeV2.locator('details form[action$="/versions/2/approve"] button.primary').click();
    await expect(page).toHaveURL(/\?approved=2$/);
    await cardThreeV2.locator('details:has(form[action$="/versions/2/enqueue"]) > summary').click();
    await cardThreeV2.locator('details form[action$="/versions/2/enqueue"] input[name="confirm_enqueue"]').check();
    await cardThreeV2.locator('details form[action$="/versions/2/enqueue"] button.primary').click();
    await expect(page).toHaveURL(/\?enqueued=2$/);
    await api.post('/fixture/dispatch-outcome', { form: { outcome: 'accepted' } });
    await page.locator('details:has(form[action$="/cases/external-commitments/drain"]) > summary').click();
    await page.locator('form[action$="/cases/external-commitments/drain"] input[name="confirm_drain"]').check();
    await page.locator('form[action$="/cases/external-commitments/drain"] button.primary').click();
    await expect(page).toHaveURL(/\?drain=1$/);
    await expect(cardThreeV2).toContainText('Состояние: submitted — Отправлено в канал; доставки и принятия НЕТ.');

    // 11. The timeout of the possible-send ambiguity: the recorded
    // no-guarantee attempt (the scripted provider declares no
    // idempotency) parks honestly — the unknown state names the
    // reconciliation path, and the SECOND drain considers NOTHING (the
    // blind retry is structurally absent, the words never re-send).
    const extFour = '019a3f33-7000-7000-8000-00000000a104';
    await create(extFour, 'Обещаем счёт на оплату после согласования состава заказа').fill();
    await expect(page).toHaveURL(/\?created=1$/);
    const cardFour = page.locator('.action-card', { hasText: extFour });
    await cardFour.locator('details:has(form[action$="/versions/1/approve"]) > summary').click();
    await cardFour.locator('details form[action$="/versions/1/approve"] input[name="confirm_approve"]').check();
    await cardFour.locator('details form[action$="/versions/1/approve"] button.primary').click();
    await expect(page).toHaveURL(/\?approved=1$/);
    await cardFour.locator('details:has(form[action$="/versions/1/enqueue"]) > summary').click();
    await cardFour.locator('details form[action$="/versions/1/enqueue"] input[name="confirm_enqueue"]').check();
    await cardFour.locator('details form[action$="/versions/1/enqueue"] button.primary').click();
    await expect(page).toHaveURL(/\?enqueued=1$/);
    await api.post('/fixture/dispatch-outcome', { form: { outcome: 'timeout', guarantee: '0' } });
    await page.locator('details:has(form[action$="/cases/external-commitments/drain"]) > summary').click();
    await page.locator('form[action$="/cases/external-commitments/drain"] input[name="confirm_drain"]').check();
    await page.locator('form[action$="/cases/external-commitments/drain"] button.primary').click();
    await expect(page).toHaveURL(/\?drain=1$/);
    await expect(cardFour).toContainText('Состояние: unknown — Итог неопределён');
    await expect(cardFour).toContainText('№1 attempt_unknown');
    await expect(cardFour).not.toContainText('идемпотентная гарантия записана');
    await page.locator('details:has(form[action$="/cases/external-commitments/drain"]) > summary').click();
    await page.locator('form[action$="/cases/external-commitments/drain"] input[name="confirm_drain"]').check();
    await page.locator('form[action$="/cases/external-commitments/drain"] button.primary').click();
    await expect(page).toHaveURL(/\?drain=0$/);
    await expect(page.locator('#page-notice')).toContainText('рассмотрено 0 записей');

    // 12. The a11y close at 320px: the single column without the
    // horizontal scroll, the skip-link, the semantic heading ladder
    // (h1 → h2 → h3), the ≥44×44 targets of the disclosure summaries and
    // the primary buttons, and the KEYBOARD toggle of the disclosure.
    await page.goto(reviewPath);
    await expect(page.locator('a.skip-link')).toBeVisible();
    await expect(page.locator('h1')).toHaveText('Внешние обязательства');
    const disclosure = page.locator('.action-card details > summary', { hasText: 'Утвердить версию' }).first();
    const disclosureDetails = disclosure.locator('..');
    await disclosure.focus();
    await disclosure.press('Enter');
    await expect(disclosureDetails).toHaveAttribute('open', '');
    const disclosureBox = await disclosure.boundingBox();
    expect(disclosureBox!.height).toBeGreaterThanOrEqual(44);
    const submitBox = await page.getByRole('button', { name: 'Создать версию' }).boundingBox();
    expect(submitBox!.height).toBeGreaterThanOrEqual(44);
    expect(submitBox!.width).toBeGreaterThanOrEqual(44);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);

    // 13. The cross-tenant read is closed: the foreign tenant gets the
    // plain 404 on the page and the machine surface — no disclosure that
    // the case or the envelopes exist.
    const foreignPage = await api.get(`/app/tenants/${foreignTenantID}/cases/${caseA}/external-commitments`);
    expect(foreignPage.status()).toBe(404);
    const foreignView = await api.get(`/v1/tenants/${foreignTenantID}/cases/${caseA}/external-commitments`);
    expect(foreignView.status()).toBe(404);

    // 14. The scope boundary the fixture CAN express: the second operator
    // (review+audit, no configuration) READS the surface — the view stays
    // open to the review member — but the tenant-level DRAIN command
    // demands the configuration scope and refuses with the honest 403
    // BEFORE any queue mutation.
    await api.post('/fixture/operator', { form: { second: '1' } });
    const readable = await page.goto(reviewPath);
    expect(readable!.status()).toBe(200);
    await expect(page.locator('h1')).toHaveText('Внешние обязательства');
    const refusedDrain = await api.post(`/app/tenants/${tenantID}/cases/external-commitments/drain`, { form: { case_id: caseA, confirm_drain: 'yes' } });
    expect(refusedDrain.status()).toBe(403);
  } finally {
    // The fixture hygiene: the dispatcher script returns to the accepted
    // default with the recorded guarantee of the live declaration, the
    // control world returns to the honest empty truth, the operator
    // switch returns to the owner and the step-up flag returns to the
    // unconfirmed state (the serial-worker discipline — the next journey
    // starts byte-identically).
    await api.post('/fixture/dispatch-outcome', { form: { outcome: 'accepted', guarantee: '1' } });
    await api.post('/fixture/controls', { form: { enabled: '' } });
    await api.post('/fixture/operator', { form: { second: '' } });
    await api.post('/fixture/step-up', { form: { enabled: '' } });
    await page.close();
  }
});
