import { test, expect } from '@playwright/test';

const tenantID = '01994f64-6c00-7000-8000-000000000002';
const foreignTenantID = '01994f64-6c00-7000-8000-00000000a212';
const controlsPath = `/app/tenants/${tenantID}/controls`;
const listPath = `/v1/tenants/${tenantID}/controls`;
const effectivePath = `/v1/tenants/${tenantID}/controls/effective`;
const setForm = `form[action="${controlsPath}"]`;

// The Story 3-2 chain at 320px: the empty world FIRST (the honest
// production-like state — the fixture server carries no tenant PG rows,
// the page explains the meaning of the absence in words), then the
// fixture world of the vertical (two seeded points over the REAL
// service), the calculator of the most-restrictive intersection with
// the full provenance (AC-3), the mutation ladder of the operator's
// own point (the dangerous-mode checkbox refusal, the step-up
// refusal, the confirmed installation with the PRG notice that TAKES
// the focus), the optimistic conflict with the CURRENT material and
// the repeat confirmation over the actual generation (AC-11), the
// read-time windows (the scheduled future does not act yet, the expiry
// stops acting — honestly walked with the fixture clock), the resume
// with the explicit confirmation (the history stays readable), the
// a11y close (the single column, the keyboard toggle, the focused
// notice), the cross-tenant 404 and the no-scope 403 with the words
// BEFORE any material.
test('scoped-controls: the honest empty world, the fixture cards, the mutation ladder, the conflict re-confirmation, the read-time windows and the a11y at 320px', async ({ browser }) => {
  test.setTimeout(120_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const api = page.request;
  try {
    // 1. The empty world of the default contour: the page names the
    // absence and explains the meaning — no control points means the
    // routes are unrestricted, a readable state, never a silent blank;
    // the machine list answers the honest EMPTY ARRAY (never null).
    await page.goto(controlsPath);
    await expect(page.locator('h1')).toHaveText('Аварийное ограничение работы');
    await expect(page.locator('#controls-empty-heading')).toHaveText('Контрольных точек нет');
    await expect(page.locator('.action-empty .next-note')).toContainText('маршруты не ограничены');
    await expect(page.locator('.action-empty .next-note')).toContainText('оператор с полномочием operations');
    await expect(page.locator('.action-card')).toHaveCount(0);
    const emptyList = await api.get(listPath);
    expect(emptyList.status()).toBe(200);
    expect((await emptyList.json()).controls).toEqual([]);

    // 2. The fixture world of the vertical switches on (the
    // fixture-only toggle — production never exposes it; the world is
    // constructed through the REAL Set path, a guard-refused seed
    // would panic the server before the first request).
    await api.post('/fixture/controls', { form: { enabled: '1' } });
    await page.goto(controlsPath);
    await expect(page.locator('.action-card')).toHaveCount(2);
    const phoneCard = page.locator('.action-card', { hasText: 'channel:phone' });
    await expect(phoneCard).toContainText('Режим: intake_only/no_promises');
    await expect(phoneCard).toContainText('Поколение: 1');

    // 3. The calculator of the most-restrictive intersection (AC-3):
    // two acting points meet (intake_only/no_promises and human_only)
    // — the rank maximum wins, the provenance keeps BOTH influencing
    // points (not only the winner), the winning reason travels, the
    // crossing words join in the ladder order.
    await page.locator('#keys').fill('channel:phone\nconnector:staffy-synthetic-telephony');
    await page.getByRole('button', { name: 'Вычислить итог' }).click();
    await expect(page.locator('#calc-result-heading')).toHaveText('Итоговый режим: human_only');
    const calcSection = page.locator('section[aria-labelledby="calc-result-heading"]');
    await expect(calcSection.locator('.action-card')).toHaveCount(2);
    await expect(calcSection).toContainText('Причина победившего правила: Регламентный обход поставщика телефонии');
    await expect(calcSection).toContainText('Слова пересечения (порядок лестницы): intake_only/no_promises; human_only');

    // 4. The machine surface: the list serves every point of the
    // tenant (the deterministic kind:ref order — the equal timestamps
    // of the fixture clock), the effective computation answers the
    // same intersection with the full provenance over the repeated
    // scope parameter.
    const list = await api.get(listPath);
    expect(list.status()).toBe(200);
    const points = (await list.json()).controls;
    expect(points.map((p: any) => `${p.scope_kind}:${p.scope_ref}`)).toEqual([
      'channel:phone',
      'connector:staffy-synthetic-telephony',
    ]);
    expect(points[0].mode).toBe('intake_only/no_promises');
    expect(points[0].generation).toBe(1);
    expect(points[0].state).toBe('active');
    const effective = await api.get(`${effectivePath}?scope=channel:phone&scope=connector:staffy-synthetic-telephony`);
    expect(effective.status()).toBe(200);
    const intersection = (await effective.json()).effective;
    expect(intersection.mode).toBe('human_only');
    expect(intersection.influencing).toHaveLength(2);
    expect(intersection.intersection).toEqual(['intake_only/no_promises', 'human_only']);
    expect(intersection.reason).toBe('Регламентный обход поставщика телефонии: до конца работ — только человек');

    // 5. The mutation ladder of the operator's own point
    // (service:billing.svc, paused): the dangerous mode WITHOUT the
    // explicit checkbox is the honest 400 of the UI discipline (the
    // server-side protection stays step-up + reason — the double
    // discipline); the submitted values travel back into the form.
    await page.locator('#scope_kind').selectOption('service');
    await page.locator('#scope_ref').fill('billing.svc');
    await page.locator('input[name="mode"][value="paused"]').check();
    await page.locator('#reason').fill('инцидент биллинга: полная остановка маршрута выставления счетов');
    await page.locator('#expected_generation').fill('0');
    await page.getByRole('button', { name: 'Установить контроль' }).click();
    await expect(page.locator('.alert')).toContainText('Опасный режим требует явного подтверждения');
    await expect(page.locator('#scope_ref')).toHaveValue('billing.svc');
    await expect(page.locator('.action-card')).toHaveCount(2);

    // 6. The checkbox is on, but the identity is not step-up confirmed:
    // the mutation gate answers the honest 403 with the confirmation
    // words (the membership precedent — the form values persist, the
    // operator corrects nothing).
    await page.locator('input[name="confirm_dangerous"]').check();
    await page.getByRole('button', { name: 'Установить контроль' }).click();
    await expect(page.locator('.alert')).toContainText('повторное подтверждение личности');
    await expect(page.locator('#scope_ref')).toHaveValue('billing.svc');
    await expect(page.locator('input[name="confirm_dangerous"]')).toBeChecked();

    // 7. The confirmed identity installs the point: the PRG redirect
    // carries the applied flag with the new generation, the notice
    // speaks the words AND TAKES the focus (the release precedent),
    // the card answers the mode word with the description, the owner,
    // the generation, the state and the stated reason.
    await api.post('/fixture/step-up', { form: { enabled: '1' } });
    await page.getByRole('button', { name: 'Установить контроль' }).click();
    await expect(page).toHaveURL(/\?applied=1$/);
    await expect(page.locator('#page-notice')).toContainText('Контрольная точка применена: поколение 1');
    await expect(page.locator('#page-notice')).toBeFocused();
    const billingCard = page.locator('.action-card', { hasText: 'service:billing.svc' });
    await expect(billingCard).toContainText('Режим: paused');
    await expect(billingCard).toContainText('полная остановка маршрута');
    await expect(billingCard).toContainText('Поколение: 1');
    await expect(billingCard).toContainText('Состояние: действует');
    const grown = await api.get(listPath);
    expect((await grown.json()).controls).toHaveLength(3);

    // 8. The optimistic conflict (AC-11): the second submission carries
    // the STALE expectation (0 — pretending the first installation),
    // the answer is the 409 re-render with the CURRENT material of the
    // point and the form REFILLED with the actual generation — the
    // operator re-confirms over the actual truth, nothing is silently
    // overwritten.
    await page.locator('#scope_kind').selectOption('service');
    await page.locator('#scope_ref').fill('billing.svc');
    await page.locator('input[name="mode"][value="human_only"]').check();
    await page.locator('#reason').fill('усиление ограничения биллинга после разбора инцидента');
    await page.locator('#expected_generation').fill('0');
    await page.locator('input[name="confirm_dangerous"]').check();
    await page.getByRole('button', { name: 'Установить контроль' }).click();
    await expect(page.locator('.control-conflict')).toContainText('Конкурентное изменение');
    await expect(page.locator('.control-conflict')).toContainText('Актуальный режим: paused');
    await expect(page.locator('.control-conflict')).toContainText('Поколение: 1');
    await expect(page.locator('#expected_generation')).toHaveValue('1');
    await page.getByRole('button', { name: 'Установить контроль' }).click();
    await expect(page).toHaveURL(/\?applied=2$/);
    await expect(page.locator('#page-notice')).toContainText('поколение 2');

    // 9. The read-time windows: the scheduled-future point exists on
    // the page but does NOT act yet (the calculator answers accept —
    // the route is unrestricted at this moment); the point with the
    // expiry bound acts now and stops acting after the fixture clock
    // walks past the boundary — the honest read-time semantics, the
    // expiry materializes nothing.
    await page.locator('#scope_kind').selectOption('campaign');
    await page.locator('#scope_ref').fill('autumn-sale');
    await page.locator('input[name="mode"][value="paused"]').check();
    await page.locator('#reason').fill('запланированная остановка осенней кампании на регламент');
    await page.locator('#expected_generation').fill('0');
    await page.locator('#scheduled_start_at').fill('2026-09-16T14:00:00Z');
    await page.locator('input[name="confirm_dangerous"]').check();
    await page.getByRole('button', { name: 'Установить контроль' }).click();
    await expect(page).toHaveURL(/\?applied=1$/);
    await page.locator('#keys').fill('campaign:autumn-sale');
    await page.getByRole('button', { name: 'Вычислить итог' }).click();
    await expect(page.locator('#calc-result-heading')).toHaveText('Итоговый режим: accept');
    await expect(page.locator('section[aria-labelledby="calc-result-heading"]')).toContainText('Маршрут не ограничен');

    await page.locator('#scope_kind').selectOption('campaign');
    await page.locator('#scope_ref').fill('summer-sale');
    await page.locator('input[name="mode"][value="paused"]').check();
    await page.locator('#reason').fill('остановка летней кампании до конца инцидента — окно получасовое');
    await page.locator('#expected_generation').fill('0');
    await page.locator('#scheduled_start_at').fill('');
    await page.locator('#expires_at').fill('2026-09-16T12:30:00Z');
    await page.locator('input[name="confirm_dangerous"]').check();
    await page.getByRole('button', { name: 'Установить контроль' }).click();
    await expect(page).toHaveURL(/\?applied=1$/);
    await page.locator('#keys').fill('campaign:summer-sale');
    await page.getByRole('button', { name: 'Вычислить итог' }).click();
    await expect(page.locator('#calc-result-heading')).toHaveText('Итоговый режим: paused');

    await api.post('/fixture/clock', { form: { minutes: '60' } });
    await page.locator('#keys').fill('campaign:summer-sale');
    await page.getByRole('button', { name: 'Вычислить итог' }).click();
    await expect(page.locator('#calc-result-heading')).toHaveText('Итоговый режим: accept');
    await api.post('/fixture/clock', { form: { minutes: '0' } });

    // 10. The resume with the explicit confirmation: the lift keeps the
    // row (the removed point is history, not absence — the generation
    // grows to 3), the PRG notice speaks the words with the focus, the
    // card stays readable with the removed state.
    const billingResume = billingCard.locator('details form[action$="/controls/resume"]');
    await billingCard.locator('details > summary').click();
    await billingResume.locator('textarea[name="reason"]').fill('инцидент биллинга закрыт — маршрут возобновляется');
    await billingResume.locator('input[name="confirm_resume"]').check();
    await billingResume.locator('button.primary').click();
    await expect(page).toHaveURL(/\?resumed=3$/);
    await expect(page.locator('#page-notice')).toContainText('Контрольная точка снята: поколение 3');
    await expect(page.locator('#page-notice')).toBeFocused();
    await expect(billingCard).toContainText('Состояние: снято — история и аудит сохранены');
    await expect(billingCard).toContainText('Поколение: 3');
    const historyList = await api.get(listPath);
    expect(historyList.status()).toBe(200);
    const history = (await historyList.json()).controls;
    expect(history).toHaveLength(5);
    expect(history.find((p: any) => p.scope_ref === 'billing.svc').state).toBe('removed');

    // 11. The a11y close at 320px: the single column without the
    // horizontal scroll, the skip-link, the semantic heading ladder
    // (h1 → h2 → h3), the ≥44×44 targets of the disclosure summaries
    // and the primary buttons, and the KEYBOARD toggle of the
    // disclosure (focus + Enter opens the details — the resume form is
    // operable without the mouse).
    await page.goto(controlsPath);
    await expect(page.locator('a.skip-link')).toBeVisible();
    await expect(page.locator('h1')).toHaveText('Аварийное ограничение работы');
    const disclosure = page.locator('.action-card details > summary').first();
    await disclosure.focus();
    await disclosure.press('Enter');
    await expect(page.locator('.action-card details').first()).toHaveAttribute('open', '');
    const disclosureBox = await disclosure.boundingBox();
    expect(disclosureBox!.height).toBeGreaterThanOrEqual(44);
    const submitBox = await page.getByRole('button', { name: 'Установить контроль' }).boundingBox();
    expect(submitBox!.height).toBeGreaterThanOrEqual(44);
    expect(submitBox!.width).toBeGreaterThanOrEqual(44);
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflow).toBeLessThanOrEqual(0);

    // 12. The cross-tenant read is closed: the foreign tenant gets the
    // plain 404 — no disclosure that the tenant has control points.
    const foreignPage = await api.get(`/app/tenants/${foreignTenantID}/controls`);
    expect(foreignPage.status()).toBe(404);
    const foreignList = await api.get(`/v1/tenants/${foreignTenantID}/controls`);
    expect(foreignList.status()).toBe(404);

    // 13. The operator without the operations scope gets the 403 with
    // the scope words BEFORE any control material: the page body speaks
    // the one admitting scope and never the point material; the machine
    // list refuses the same way.
    await api.post('/fixture/operator', { form: { second: '1' } });
    const refused = await page.goto(controlsPath);
    expect(refused!.status()).toBe(403);
    await expect(page.locator('body')).toContainText('operations');
    await expect(page.locator('body')).not.toContainText('billing.svc');
    await expect(page.locator('body')).not.toContainText('staffy-synthetic-telephony');
    const refusedList = await api.get(listPath);
    expect(refusedList.status()).toBe(403);
    const refusedBody = await refusedList.text();
    expect(refusedBody).toContain('operations');
    expect(refusedBody).not.toContain('billing.svc');
  } finally {
    // The fixture hygiene: the control world returns to the honest
    // empty truth, the operator switch returns to the owner, the
    // step-up flag returns to the unconfirmed state and the clock
    // offset resets to zero (the serial-worker discipline of the
    // chains — the next journey starts byte-identically).
    await api.post('/fixture/controls', { form: { enabled: '' } });
    await api.post('/fixture/operator', { form: { second: '' } });
    await api.post('/fixture/step-up', { form: { enabled: '' } });
    await api.post('/fixture/clock', { form: { minutes: '0' } });
    await page.close();
  }
});
