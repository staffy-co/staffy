import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const foreignTenantID = '01994f64-6c00-7000-8000-00000000a212';
const actionsPath = `/app/tenants/${tenantID}/actions`;
const listPath = `/v1/tenants/${tenantID}/action-definitions`;

// The Story 3.1 chain at 320px: the control branch FIRST — the
// production binary dictionary is honestly EMPTY (AG-4: neither
// published pack is linked to a capability connector; the page explains
// the reason in words, never a silent blank — AC-11, the machine list
// answers the honest empty array), then the fixture dictionary of the
// vertical (two definitions plus two materially different versions of
// one id) serves the cards with the full condition system behind the
// progressive disclosure (the details/summary technique — the card never
// floods the reader), the machine list proposes the NEWEST of every id
// only (the 1-8 convention), the pin read serves the OLD version
// byte-exact (AC-8: the pinned past stays resolvable — the material for
// the future Manifest/Operation/Receipt references), the unknown and
// the malformed pins are the honest 404 of an invented future, the
// cross-tenant read is closed, the operator without the scopes gets the
// 403 with the scope words BEFORE any catalog material, and the a11y
// close walks the 320px single column, the ≥44×44 disclosure targets,
// the keyboard toggle of the disclosures and the semantic headings.
test('actions-catalog: the honest empty production state, the fixture cards with the full conditions, the pin read of the old version and the a11y at 320px', async ({ browser }) => {
  test.setTimeout(120_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. The control branch of the production state: the binary
    // dictionary is honestly empty — the page names the absence and
    // explains the reason in words (the definitions appear only with
    // published pack connectors — AG-4), no cards, no disclosures; the
    // machine list answers the honest EMPTY ARRAY (never null).
    await page.goto(actionsPath);
    await expect(page.locator('h1')).toHaveText('Каталог действий');
    await expect(page.locator('#action-empty-heading')).toHaveText('Определений пока нет');
    await expect(page.locator('.next-note')).toContainText('Каталог действий пуст');
    await expect(page.locator('.next-note')).toContainText('опубликованный Domain Pack');
    await expect(page.locator('.next-note')).toContainText('честное состояние словаря, а не ошибка');
    await expect(page.locator('.action-card')).toHaveCount(0);
    await expect(page.locator('details')).toHaveCount(0);
    const emptyList = await api.get(listPath);
    expect(emptyList.status()).toBe(200);
    expect((await emptyList.json()).action_definitions).toEqual([]);

    // 2. The fixture dictionary of the vertical switches on (the
    // fixture-only toggle — production never exposes it; both contours
    // are the REAL app service over the REAL resolvers).
    await api.post('/fixture/actions', { form: { enabled: '1' } });

    // 3. The machine list proposes the NEWEST of every id only: two
    // entries — the pinned old 1.0.0 is ABSENT from the list; the
    // material of the newest definition travels whole (the conditions,
    // the delivery contract, the declared schema with the secret
    // MARKING — the values never exist).
    const list = await api.get(listPath);
    expect(list.status()).toBe(200);
    const definitions = (await list.json()).action_definitions;
    expect(definitions.map((d: any) => `${d.id}@${d.version}`)).toEqual(['notify.order-ready@1.1.0', 'query.trunk-status@1.0.0']);
    const sms = definitions[0];
    expect(sms.name).toBe('Отправка SMS клиенту');
    expect(sms.owning_system).toBe('erp');
    expect(sms.conditions.risk_class).toBe('medium');
    expect(sms.conditions.timeout_ms).toBe(30000);
    expect(sms.conditions.confirmation).toBe('required');
    expect(sms.delivery_contract.capability_class).toBe('provider_idempotent');
    expect(sms.delivery_contract.reconciliation_query).toBe('query.trunk-status');
    expect(sms.typed_input_schema[1].secret).toBe(true);
    expect(sms.pack_ref).toEqual({ pack_id: 'auto-parts', pack_version: '2.0.0' });
    const trunk = definitions[1];
    expect(trunk.conditions.confirmation).toBe('not_required');
    expect(trunk.delivery_contract.capability_class).toBe('query_reconcilable');

    // 4. The page renders the cards: the headline words of both classes
    // — the command-class SMS (risk medium, the required confirmation,
    // the capability triple, the pack pin, the owning system, the
    // compile-time immutability state) and the query-class trunk status
    // (risk low, the not_required survival of the pair).
    await page.goto(actionsPath);
    await expect(page.locator('.action-card')).toHaveCount(2);
    const smsCard = page.locator('.action-card').first();
    await expect(smsCard).toContainText('Отправка SMS клиенту');
    await expect(smsCard).toContainText('Идентификатор: notify.order-ready · версия 1.1.0');
    await expect(smsCard).toContainText('Риск: medium');
    await expect(smsCard).toContainText('Стадии: closed_account_test; controlled_real_customer');
    await expect(smsCard).toContainText('Владеющая система: erp');
    await expect(smsCard).toContainText('Capability: staffy-synthetic-telephony@1.1.0/command.send-sms');
    await expect(smsCard).toContainText('Domain Pack: auto-parts@2.0.0');
    await expect(smsCard).toContainText('Состояние: опубликовано (compile-time словарь, версия неизменяема)');
    await expect(smsCard).toContainText('Назначение: Уведомить клиента о готовности заказа');
    const trunkCard = page.locator('.action-card').nth(1);
    await expect(trunkCard).toContainText('Статус телефонной линии');
    await expect(trunkCard).toContainText('Риск: low');

    // 5. The progressive disclosure: the full condition system, the
    // delivery contract and the declared schemas live behind the
    // details/summary of the SMS card — the card never floods the
    // reader with every word at once.
    await page.locator('details.action-conditions > summary').first().click();
    await expect(page.locator('details.action-conditions').first()).toHaveAttribute('open', '');
    const conditions = page.locator('details.action-conditions .action-conditions-grid').first();
    await expect(conditions).toBeVisible();
    await expect(conditions).toContainText('Класс автономности: assisted');
    await expect(conditions).toContainText('Режимы разговора: claim; cancellation');
    await expect(conditions).toContainText('Роли: owner; configurator');
    await expect(conditions).toContainText('Материальные зависимости: order_status (fresh)');
    await expect(conditions).toContainText('Подтверждение: required');
    await expect(conditions).toContainText('Обязательные гейты: OQ-7A; AG-1');
    await expect(conditions).toContainText('Таймаут исполнения: 30000 мс');
    await expect(conditions).toContainText('Класс материальности: material');
    await expect(conditions).toContainText('Правило потребления: single_use');
    await page.locator('details.action-delivery > summary').first().click();
    const delivery = page.locator('details.action-delivery .action-conditions-grid').first();
    await expect(delivery).toBeVisible();
    await expect(delivery).toContainText('Класс доставки: provider_idempotent');
    await expect(delivery).toContainText('Область идемпотентности: connector-ack');
    await expect(delivery).toContainText('Терминальное доказательство: delivery-receipt');
    await expect(delivery).toContainText('Reconciliation-запрос: query.trunk-status');
    await expect(delivery).toContainText('Семантика отмены/компенсации: не компенсируется — отказ до подтверждения');
    await page.locator('details.action-schema > summary').first().click();
    await expect(page.locator('details.action-schema h3').first()).toHaveText('Входные поля');
    const schema = page.locator('details.action-schema ul.action-fields').first();
    await expect(schema).toContainText('Телефон получателя (phone): тип text, обязательно: да, секрет: нет');
    await expect(schema).toContainText('Ссылка на шаблон (template_url): тип url, обязательно: нет, секрет: да — Шаблон сообщения');
    await expect(page.locator('details.action-schema h3').nth(1)).toHaveText('Выходные поля');

    // 6. The pin read of the OLD version: the 1.0.0 material is
    // MATERIALLY older (risk low, timeout 15000 — the newest is
    // medium/30000), served byte-exact — the pinned past stays
    // immutable and resolvable (AC-8).
    const pin = await api.get(`${listPath}/notify.order-ready/1.0.0`);
    expect(pin.status()).toBe(200);
    const pinned = (await pin.json()).action_definition;
    expect(pinned.version).toBe('1.0.0');
    expect(pinned.conditions.risk_class).toBe('low');
    expect(pinned.conditions.timeout_ms).toBe(15000);
    expect(pinned.name).toBe('Отправка SMS клиенту');

    // 7. The invented future does not exist: the unknown version, the
    // unknown id, the malformed charset, the incomplete triple and the
    // word instead of the triple are all the honest 404.
    for (const path of [
      `${listPath}/notify.order-ready/9.9.9`,
      `${listPath}/notify.unknown/1.0.0`,
      `${listPath}/BIG-ID/1.0.0`,
      `${listPath}/notify.order-ready/1.0`,
      `${listPath}/notify.order-ready/latest`,
    ]) {
      const refused = await api.get(path);
      expect(refused.status()).toBe(404);
    }

    // 8. The cross-tenant read is closed: the foreign tenant gets the
    // plain 404 — no disclosure that the tenant has an action catalog.
    const foreign = await api.get(`/v1/tenants/${foreignTenantID}/action-definitions`);
    expect(foreign.status()).toBe(404);

    // 9. The operator without the scopes gets the 403 with the scope
    // words BEFORE any catalog material: the page body speaks the two
    // admitting scopes and never the card material; the machine list
    // refuses the same way.
    await api.post('/fixture/operator', { form: { second: '1' } });
    const refusedPage = await page.goto(actionsPath);
    expect(refusedPage!.status()).toBe(403);
    await expect(page.locator('body')).toContainText('integrations');
    await expect(page.locator('body')).toContainText('configuration');
    await expect(page.locator('body')).not.toContainText('Отправка SMS');
    const refusedList = await api.get(listPath);
    expect(refusedList.status()).toBe(403);
    const refusedBody = await refusedList.text();
    expect(refusedBody).toContain('integrations');
    expect(refusedBody).toContain('configuration');
    expect(refusedBody).not.toContain('notify.order-ready');
    await api.post('/fixture/operator', { form: { second: '' } });

    // 10. The a11y close at 320px: the single column without the
    // horizontal scroll, the skip-link, the semantic heading ladder
    // (h1 → h3 inside the disclosures), the ≥44×44 targets of the
    // disclosure summaries and the KEYBOARD toggle of the disclosure
    // (focus + Enter opens the details — the progressive disclosure is
    // operable without the mouse).
    await page.goto(actionsPath);
    await expect(page.locator('a.skip-link')).toBeVisible();
    await expect(page.locator('h1')).toHaveText('Каталог действий');
    const deliverySummary = page.locator('details.action-delivery > summary').first();
    await deliverySummary.focus();
    await deliverySummary.press('Enter');
    await expect(page.locator('details.action-delivery').first()).toHaveAttribute('open', '');
    const summaryBox = await page.locator('details.action-conditions > summary').first().boundingBox();
    expect(summaryBox!.height).toBeGreaterThanOrEqual(44);
    expect(summaryBox!.width).toBeGreaterThanOrEqual(44);
    const schemaBox = await page.locator('details.action-schema > summary').first().boundingBox();
    expect(schemaBox!.height).toBeGreaterThanOrEqual(44);
    expect(schemaBox!.width).toBeGreaterThanOrEqual(44);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);
  } finally {
    // The fixture hygiene: the action-catalog switch returns to the
    // production truth (the honest empty binary), the operator switch
    // returns to the owner (the serial-worker discipline of the chains).
    await api.post('/fixture/actions', { form: { enabled: '' } });
    await api.post('/fixture/operator', { form: { second: '' } });
    await page.close();
  }
});
