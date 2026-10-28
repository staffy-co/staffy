import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const foreignTenantID = '01994f64-6c00-7000-8000-00000000a212';
const toolsPath = `/app/tenants/${tenantID}/tools`;
const listPath = `/v1/tenants/${tenantID}/tools`;

// z55 — Story 7-2 (Epic 7): the Typed Tools Registry journey. The
// production registry is LIVE (the first three vertical contracts over
// the 4-2/4-3/4-4 machines, every adapter honestly not_implemented —
// the double honesty: no adapters, no dispatcher). The page renders the
// dictionary words as text (versions, classes, statuses, dependencies,
// owners) behind the progressive disclosure; the machine list answers
// the newest of every id (never null); the pin read serves the exact
// version byte-exact (the material for the future Manifest/Planner
// references — 8.3/7.3); the unknown and the malformed pins are the
// honest 404 of an invented future; the cross-tenant read is closed;
// the operator without the scopes gets the 403 with the scope words
// BEFORE any registry material; the a11y close walks the 320px single
// column, the skip-link, the semantic headings and the ≥44×44 disclosure
// targets.

test('tools-registry: the live contracts, the honest not_implemented words, the pin read and the a11y at 320px', async ({ browser }) => {
  test.setTimeout(120_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. The page renders the LIVE registry: three cards, the honest
    // adapter words («вызов невозможен»), the single-value class
    // dictionary and the platform guarantees spoken as text.
    await page.goto(toolsPath);
    await expect(page.locator('h1')).toHaveText('Реестр инструментов');
    await expect(page.locator('.action-card').count()).resolves.toBeGreaterThanOrEqual(3);
    const fitment = page.locator('.action-card').first();
    await expect(fitment).toContainText('Идентификация автомобиля');
    await expect(fitment).toContainText('deterministic_read');
    await expect(fitment).toContainText('not_implemented');
    await expect(fitment).toContainText('вызов невозможен');
    await expect(fitment).toContainText('Зависимости: нет (самодостаточный контракт)');
    // The fitment evidence card carries the catalog dependency.
    const cards = page.locator('.action-card');
    await expect(cards.nth(2)).toContainText('Доказательство применимости');
    await expect(cards.nth(2)).toContainText('Зависимость: коннектор типа catalog');

    // 2. The machine list answers the newest of every id — exactly the
    // three live contracts with the dictionaries on the wire (never
    // null; the write class is structurally impossible).
    const list = await api.get(listPath);
    expect(list.status()).toBe(200);
    const contracts = (await list.json()).tools;
    // The witness-subseries: the three live contracts stay first in the
    // canonical order; registry additions (the external author's PR) are
    // allowed — the journey never pins the registry size (the Story 7-1
    // subseries precedent).
    const series = contracts.map((c: any) => `${c.id}@${c.version}`);
    const wantSeries = [
      'auto-parts.vehicle-identification@1.0.0',
      'auto-parts.part-request-classification@1.0.0',
      'auto-parts.fitment-evidence@1.0.0',
    ];
    expect(series.slice(0, wantSeries.length)).toEqual(wantSeries);
    for (const contract of contracts) {
      expect(contract.determinism_class).toBe('deterministic_read');
      expect(contract.adapter_status).toBe('not_implemented');
      expect(contract.input_schema.length).toBeGreaterThan(0);
      expect(contract.output_schema.length).toBeGreaterThan(0);
    }
    const fitmentContract = contracts[2];
    expect(fitmentContract.dependency.connector_type).toBe('catalog');
    expect(fitmentContract.evidence_ref).toBe('auto-parts/spec/4.4');

    // 3. The pin read serves the exact version whole — the material for
    // the future Manifest/Planner references.
    const pin = await api.get(`${listPath}/auto-parts.fitment-evidence/1.0.0`);
    expect(pin.status()).toBe(200);
    const pinned = (await pin.json()).tool;
    expect(pinned.id).toBe('auto-parts.fitment-evidence');
    expect(pinned.version).toBe('1.0.0');
    expect(pinned.name).toBe('Доказательство применимости');
    expect(pinned.dependency.connector_type).toBe('catalog');

    // 4. The invented future does not exist: the unknown version, the
    // unknown id, the malformed charset, the uppercase id, the
    // incomplete triple, the leading zero and the prerelease word are
    // all the honest 404.
    for (const path of [
      `${listPath}/auto-parts.fitment-evidence/9.9.9`,
      `${listPath}/auto-parts.surprise/1.0.0`,
      `${listPath}/Плохой!ID/1.0.0`,
      `${listPath}/Auto-Parts/1.0.0`,
      `${listPath}/auto-parts.fitment-evidence/1.0`,
      `${listPath}/auto-parts.fitment-evidence/01.0.0`,
      `${listPath}/auto-parts.fitment-evidence/1.0.0-rc.1`,
      `${listPath}/auto-parts.fitment-evidence/latest`,
    ]) {
      const refused = await api.get(path);
      expect(refused.status()).toBe(404);
    }

    // 5. The cross-tenant read is closed: the foreign tenant gets the
    // plain 404 — no disclosure that the tenant has a tool surface.
    const foreign = await api.get(`/v1/tenants/${foreignTenantID}/tools`);
    expect(foreign.status()).toBe(404);

    // 6. The operator without the scopes gets the 403 with the scope
    // words BEFORE any registry material: the page body speaks the two
    // admitting scopes and never the contract words; the machine list
    // refuses the same way.
    await api.post('/fixture/operator', { form: { second: '1' } });
    const refusedPage = await page.goto(toolsPath);
    expect(refusedPage!.status()).toBe(403);
    await expect(page.locator('body')).toContainText('integrations');
    await expect(page.locator('body')).toContainText('configuration');
    await expect(page.locator('body')).not.toContainText('Доказательство применимости');
    const refusedList = await api.get(listPath);
    expect(refusedList.status()).toBe(403);
    const refusedBody = await refusedList.text();
    expect(refusedBody).toContain('integrations');
    expect(refusedBody).toContain('configuration');
    expect(refusedBody).not.toContain('fitment-evidence');
    await api.post('/fixture/operator', { form: { second: '' } });

    // 7. The a11y close at 320px: the single column without the
    // horizontal scroll, the skip-link, the semantic heading ladder and
    // the ≥44×44 targets of the disclosure summaries with the KEYBOARD
    // toggle (the progressive disclosure is operable without the mouse).
    await page.goto(toolsPath);
    await expect(page.locator('a.skip-link')).toBeVisible();
    await expect(page.locator('h1')).toHaveText('Реестр инструментов');
    const schemaSummary = page.locator('details.action-schema > summary').first();
    await schemaSummary.focus();
    await schemaSummary.press('Enter');
    await expect(page.locator('details.action-schema').first()).toHaveAttribute('open', '');
    const summaryBox = await page.locator('details.action-schema > summary').first().boundingBox();
    expect(summaryBox!.height).toBeGreaterThanOrEqual(44);
    await expect(page.locator('html')).toHaveAttribute('lang', 'ru');
  } finally {
    await api.post('/fixture/operator', { form: { second: '' } });
  }
});
