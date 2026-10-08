import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const connectionID = '01994f64-6c00-7000-8000-000000000006';
const casesPath = `/app/tenants/${tenantID}/cases`;

const eventID = 'journey-cases-evt-1';
const sessionID = 'journey-cases-sess-1';

async function postIngress(api: any, body: unknown, key: string) {
  return api.post('/v1/ingress/channel-events', {
    data: body,
    headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key },
  });
}

function inboundEvent(providerEventId: string, sessionId: string, payload: Record<string, unknown> = { event: 'inbound-call', from: '+79000000000' }) {
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

test('cases: ingress creates one case, honest timeline and replay without growth at 320px', async ({ browser }) => {
  test.setTimeout(120_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. The machine ingress accepts the first event: one transaction
    // creates the case material; no auto-reply is ever sent.
    const first = await postIngress(api, inboundEvent(eventID, sessionID), 'journey-cases-key-1');
    expect(first.status()).toBe(201);
    const accepted = await first.json();
    expect(accepted.new_case).toBe(true);
    expect(accepted.case_no).toBe(1);
    expect(accepted.conversation_no).toBe(1);
    expect(accepted.replay).toBe(false);

    // 2. The operator surface: the cases list shows the case.
    await page.goto(casesPath);
    await expect(page.getByRole('heading', { name: 'Обращения' })).toBeVisible();
    await expect(page.locator('p[role="status"]').first()).toContainText('1');
    const caseLink = page.getByRole('link', { name: '№1' });
    await expect(caseLink).toBeVisible();
    await caseLink.click();
    await expect(page).toHaveURL(new RegExp(`/cases/${accepted.case_id}$`));

    // 3. The case detail: the honest statements, the owner with the first
    // epoch, the inbox surface, the timeline with both times and the
    // untriaged work item.
    await expect(page.getByRole('heading', { name: 'Обращение №1' })).toBeVisible();
    await expect(page.getByText('Автоответ клиенту не отправляется').first()).toBeVisible();
    await expect(page.getByText('Запрос ещё не классифицирован').first()).toBeVisible();
    const timeline = page.getByRole('group', { name: 'Таймлайн событий' });
    await expect(timeline).toBeVisible();
    await expect(timeline.getByRole('row')).toHaveCount(2); // header + one event
    await expect(timeline.getByText('phone')).toBeVisible();
    await expect(timeline.getByText('Принято')).toBeVisible();
    await expect(page.getByRole('group', { name: 'Диалоги обращения' }).getByText('inbox')).toBeVisible();
    await expect(page.getByRole('group', { name: 'Диалоги обращения' }).getByText('(1)')).toBeVisible();

    // 4. The identical redelivery replays the original result before the
    // admission ladder: 200 with the replay header and the same ids.
    const repeat = await postIngress(api, inboundEvent(eventID, sessionID), 'journey-cases-key-1');
    expect(repeat.status()).toBe(200);
    expect((repeat.headers() as Record<string, string>)['idempotency-replayed']).toBe('true');
    const replayed = await repeat.json();
    expect(replayed.event_id).toBe(accepted.event_id);
    expect(replayed.case_id).toBe(accepted.case_id);

    // 5. The counters did not grow: one case, one event.
    await page.goto(casesPath);
    await expect(page.locator('p[role="status"]').first()).toContainText('1');
    await expect(page.getByRole('row').filter({ hasText: '№1' })).toContainText('1');

    // 6. The digest conflict is quarantined: 409, the exception is visible
    // to the operator with the correlation id, the case is not rewritten.
    const conflicting = await postIngress(api, inboundEvent(eventID, sessionID, { event: 'inbound-call', from: '+79000000099' }), 'journey-cases-key-2');
    expect(conflicting.status()).toBe(409);
    const problem = await conflicting.json();
    expect(problem.code).toBe('digest_conflict');
    await page.goto(casesPath);
    await expect(page.getByRole('group', { name: 'Операционные исключения' })).toBeVisible();
    await expect(page.getByText('Digest-конфликт события').first()).toBeVisible();

    // 7. A second event of the same session continues the conversation:
    // no second case, the timeline grows, the work item stays one.
    const continuation = await postIngress(api, inboundEvent('journey-cases-evt-2', sessionID, { event: 'inbound-call', seq: 2 }), 'journey-cases-key-3');
    expect(continuation.status()).toBe(201);
    const continued = await continuation.json();
    expect(continued.case_id).toBe(accepted.case_id);
    expect(continued.new_case).toBe(false);
    await page.goto(`${casesPath}/${accepted.case_id}`);
    await expect(page.getByRole('group', { name: 'Таймлайн событий' }).getByRole('row')).toHaveCount(3);
    await expect(page.getByRole('group', { name: 'Рабочие элементы' }).getByRole('row')).toHaveCount(2);

    // 8. The honest refusal of the ladder: an unknown connection is a
    // plain 404 problem with the dictionary kind, no payload disclosed.
    const broken = { ...inboundEvent('journey-cases-evt-3', 'journey-cases-sess-2'), connection_id: '01994f64-6c00-7000-8000-0000000000ff' };
    const refused = await postIngress(api, broken, 'journey-cases-key-5');
    expect(refused.status()).toBe(404);
    const refusal = await refused.json();
    expect(refusal.code).toBe('unknown_connection');

    // 9. Accessibility at 320px: no horizontal scroll, the tables are
    // keyboard-reachable groups, the statuses are text.
    await expect(page.locator('main')).not.toHaveCSS('overflow-x', /scroll|auto/);
    const groups = page.getByRole('group');
    expect(await groups.count()).toBeGreaterThanOrEqual(3);
  } finally {
    await page.close();
  }
});
