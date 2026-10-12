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

// The Story 2.7 chain at 320px: the «Материальные поля Owning System»
// panel (the matrix words, the honest per-field cards — never read, fresh
// with the value and the carrier words, stale with the re-read action
// text, the conflict badge against the confirmed fact of the same
// material key), the read form (the fixed matrix dictionary select, the
// observation anchor of the double-submit protection), the PRG redirect
// with the aria-live announcement — keyboard-first, no horizontal scroll,
// the stale state never distinguishable from fresh by the color alone.
test('cases-observation: the owning-system reads, the freshness and the conflict badge at 320px', async ({ browser }) => {
  test.setTimeout(180_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. One call: the case with the initial work item.
    const accepted = await postIngress(api, inboundEvent('journey-observation-evt-1', 'journey-observation-sess-1', { call: 'inbound-call', from: '+79000000120' }), 'journey-observation-key-1');
    expect(accepted.status()).toBe(201);
    const caseA = (await accepted.json()).case_id;
    const pathA = `${casesPath}/${caseA}`;

    // 2. The fresh panel: the matrix version word, the honest «поле не
    // читалось» cards of every matrix field, the one-attempt honesty
    // note.
    await page.goto(pathA);
    const observationHeading = page.getByRole('heading', { name: /Материальные поля Owning System/ });
    await expect(observationHeading).toBeVisible();
    await expect(page.locator('.next-note').filter({ hasText: /state-authority-matrix-v1/ })).toBeVisible();
    for (const field of ['price', 'availability', 'order_status']) {
      await expect(page.locator(`details summary`).filter({ hasText: new RegExp(`^${field} — поле не читалось$`) })).toBeVisible();
    }
    await expect(page.locator('.next-note').filter({ hasText: /не предполагается/ })).toBeVisible();

    // 3. The confirmed price fact with DIFFERENT canonical bytes (the
    // conflict basis: the synthetic owning system answers
    // {"price_quote":"3000"}, the fact carries 2900).
    await page.locator('details:has(summary:text-is("Записать факт"))').click();
    const factForm = page.locator('form[action$="/facts"]').first();
    await factForm.locator('select[name="source_type"]').selectOption('operator');
    await factForm.locator('input[name="material_key"]').fill('price');
    await factForm.locator('textarea[name="value"]').fill('{"price_quote":"2900"}');
    await factForm.getByRole('button', { name: /Записать факт/ }).click();
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("price — Без подтверждения"))').first().click();
    await page.getByText('Подтвердить точную версию факта price').first().click();
    const confirmForm = page.locator('form[action$="/confirmations"]').first();
    await confirmForm.getByRole('button', { name: /Подтвердить версию/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?confirmed=1`));

    // 4. The read form of the price: the fixed matrix dictionary select
    // (three words only), the PRG redirect with the honest flag.
    await page.goto(pathA);
    await page.locator('details:has(summary:text-is("Перечитать поле Owning System (read_owning_system)"))').first().click();
    const readForm = page.locator('form[action$="/owning-system-reads"]').first();
    const fieldOptions = readForm.locator('select[name="material_key"] option');
    await expect(fieldOptions).toHaveCount(3);
    await readForm.locator('select[name="material_key"]').selectOption('price');
    await readForm.getByRole('button', { name: /Перечитать поле/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?owning_system_read=1`));

    // 5. The fresh price card: the value snapshot, the carrier words (the
    // external id, the provider version), both times, the honest
    // «свежее» state and the CONFLICT badge against the confirmed fact
    // (the bytes differ — the honest «расхождение», the resolution stays
    // with the human).
    await page.goto(pathA);
    const priceCard = page.locator('details').filter({ has: page.locator('summary', { hasText: /^price — свежее/ }) }).first();
    await priceCard.locator('summary').click();
    await expect(priceCard).toContainText('ok (значение прочитано)');
    await expect(priceCard.locator('.digest-block').filter({ hasText: /\{"price_quote":"3000"\}/ })).toBeVisible();
    await expect(priceCard).toContainText('catalog');
    await expect(priceCard).toContainText('v1');
    await expect(priceCard).toContainText('№1');
    await expect(priceCard.locator('.alert').filter({ hasText: /Конфликт-бейдж/ })).toContainText('расхождение с фактом');
    await expect(priceCard).not.toContainText('устаревшее');

    // 6. The availability read: the fixture owning system answers with
    // the provider age past the 60-second window — the honest STALE
    // derivation with the re-read action text (never the color alone).
    await page.locator('details:has(summary:text-is("Перечитать поле Owning System (read_owning_system)"))').first().click();
    const readForm2 = page.locator('form[action$="/owning-system-reads"]').first();
    await readForm2.locator('select[name="material_key"]').selectOption('availability');
    await readForm2.getByRole('button', { name: /Перечитать поле/ }).click();
    await expect(page).toHaveURL(new RegExp(`${caseA}\\?owning_system_read=1`));
    await page.goto(pathA);
    const availabilityCard = page.locator('details').filter({ has: page.locator('summary', { hasText: /^availability — устаревшее/ }) }).first();
    await availabilityCard.locator('summary').click();
    await expect(availabilityCard.locator('.alert').filter({ hasText: /Устаревшее/ })).toContainText('перечитайте перед материальным ответом');
    // The price card stays fresh — the per-field isolation (one window
    // never masks another field).
    await expect(page.locator('details summary').filter({ hasText: /^price — свежее/ })).toBeVisible();

    // 7. The aria-live announcement region of the observation panel (the
    // state changes never move the focus).
    const liveRegion = page.locator('div[aria-live="polite"]').filter({ hasText: /price —|availability —|order_status —/ });
    await expect(liveRegion.first()).toBeAttached();

    // 8. No horizontal scroll at 320px (the long digests and the carrier
    // words wrap — the panel never overflows the viewport).
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    await page.close();
  }
});
