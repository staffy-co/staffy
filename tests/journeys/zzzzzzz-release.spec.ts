import { expect, test } from '@playwright/test';

const employeePath = '/app/tenants/01994f64-6c00-7000-8000-000000000002/employees';
const releasePath = `${employeePath}/01994f64-6c00-7000-8000-000000000004/release`;

test('release versions and manifest stay honest, immutable and accessible at 320px with keyboard', async ({ browser }) => {
  test.setTimeout(90_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });

  // Create (or reuse) the fixture employee, then open the release page.
  await page.goto(`${employeePath}`);
  await page.getByRole('button', { name: 'Создать ИИ-сотрудника' }).click();
  await page.getByRole('link', { name: 'Релиз' }).click();
  await expect(page.getByRole('heading', { name: 'Версии конфигурации и Release Manifest' })).toBeVisible();

  // The unassembled page states the honest precondition.
  await expect(page.getByText('Черновик конфигурации ещё не собран')).toBeVisible();

  // Assemble the draft: every component carries an honest status with a
  // reason — a recorded absence in a fresh state, a pinned version after
  // the earlier journeys. The assertions stay state-tolerant.
  const assembleForm = page.locator('details.map-form', { hasText: 'Собрать / обновить черновик конфигурации' });
  await assembleForm.locator('summary').click();
  await assembleForm.getByRole('button', { name: 'Собрать черновик из текущих фактов' }).click();
  await expect(page.locator('#page-notice')).toBeFocused();
  await expect(page.locator('#page-notice')).toContainText('Черновик конфигурации собран');
  const draftTable = page.locator('table', { hasText: 'Точные ссылки на версии компонентов' });
  await expect(draftTable).toBeVisible();
  for (const component of ['Employee Brief', 'Domain Pack', 'Поведение', 'Знания', 'Голос']) {
    const row = draftTable.getByRole('row', { name: new RegExp(component) });
    await expect(row).toBeVisible();
    await expect(row.locator('td').first()).toContainText(/настроен|неприменим/);
  }
  const packRow = draftTable.getByRole('row', { name: /Domain Pack/ });
  if ((await packRow.locator('td').first().innerText()).includes('не настроен')) {
    await expect(packRow).toContainText('выберите пакет');
  } else {
    await expect(packRow.locator('td').nth(1)).toContainText(/ \d+\.\d+\.\d+/);
  }

  // Create the immutable version.
  const versionForm = page.locator('details.map-form', { hasText: 'Новая конфигурационная версия' });
  await versionForm.locator('summary').click();
  await versionForm.getByLabel('Описание изменения (1–300 символов)').fill('Пилотная конфигурация №1');
  await versionForm.getByRole('button', { name: 'Создать неизменяемую версию' }).click();
  await expect(page.locator('#page-notice')).toBeFocused();
  await expect(page.locator('#page-notice')).toContainText('Конфигурационная версия создана');
  const history = page.locator('table', { hasText: 'Неизменяемые версии конфигурации' });
  await expect(history.getByRole('row', { name: /^v1 / })).toContainText('не запечатан');

  // Seal the manifest: the honest incomplete status with runtime gaps.
  const sealForm = page.locator('details.map-form', { hasText: 'Запечатать Release Manifest' });
  await sealForm.locator('summary').click();
  await sealForm.getByLabel('Номер конфигурационной версии').fill('1');
  await sealForm.getByRole('button', { name: 'Запечатать манифест' }).click();
  await expect(page.locator('#page-notice')).toBeFocused();
  await expect(page.locator('#page-notice')).toContainText('Манифест запечатан');

  // The manifest block: candidate wording, digest, runtime gap owners.
  await expect(page.locator('#candidate-note')).toContainText('Проверяемый кандидат, а не допущенный release');
  await expect(page.locator('#candidate-note')).toContainText('Story 1-14');
  await expect(page.locator('.rel-status-incomplete').first()).toContainText('incomplete');
  const digestBlock = page.locator('#manifest-digest');
  await expect(digestBlock).toBeVisible();
  await expect(digestBlock).toContainText(/^[0-9a-f]{64}$/);
  const runtimeTable = page.locator('table', { hasText: 'каждый отсутствующий' });
  await expect(runtimeTable.getByRole('row', { name: /model_route/ })).toContainText('отсутствует');
  // The owner label substring survives epic-label evolution; the
  // Story 8-2 present rows render live (the assembly and the runtime
  // identity of the new seal).
  await expect(runtimeTable.getByRole('row', { name: /autonomy_envelope/ })).toContainText('AI Runtime');
  await expect(runtimeTable.getByRole('row', { name: /prompt_assembly/ })).toContainText('присутствует: assembly/1.0.0');
  await expect(runtimeTable.getByRole('row', { name: /runtime_build/ })).toContainText('присутствует: planner/1.0.0+assembly/1.0.0');
  const dependencies = page.locator('ul.launch-plan').first();
  await expect(dependencies).toContainText('Operating Profile');
  await expect(dependencies).toContainText('GATE-REGISTRY');

  // Connections in the manifest: either the honest empty statement or the
  // frozen facts with capability states (the earlier journeys connect an
  // account, so the sealed manifest lists it in the full-suite order).
  const connectionsTable = page.locator('table', { hasText: 'Подключения на момент запечатывания' });
  if (await connectionsTable.count()) {
    const firstConnection = connectionsTable.getByRole('row').nth(1);
    await expect(firstConnection.locator('.digest-block').first()).toContainText(/^[0-9a-f-]{36}$/);
    await expect(firstConnection).toContainText(/не проверена|подтверждена|устарело|неопределён|не декларирует/);
  } else {
    await expect(page.getByText('Подключений у пространства нет')).toBeVisible();
  }

  // The long digest wraps inside its block: no horizontal page scroll at 320 px.
  const scrolled = await page.evaluate(() => document.documentElement.scrollWidth);
  expect(scrolled).toBeLessThanOrEqual(320);
  const box = await digestBlock.boundingBox();
  expect(box?.width).toBeLessThanOrEqual(320);

  // A stale-revision seal is refused with the honest conflict.
  await sealForm.locator('summary').click();
  await sealForm.getByLabel('Номер конфигурационной версии').fill('1');
  await page.locator('#seal-revision').evaluate((el) => (el.value = '99'));
  await sealForm.getByRole('button', { name: 'Запечатать манифест' }).click();
  await expect(page.locator('.alert')).toContainText('Конфликт: ожидаемая ревизия');

  // Restore from v1: the new draft references the version, the past stays.
  const restoreForm = page.locator('details.map-form', { hasText: 'Восстановить черновик из прошлой версии' });
  await restoreForm.locator('summary').click();
  await restoreForm.getByLabel('Номер конфигурационной версии-источника').fill('1');
  await restoreForm.getByRole('button', { name: 'Восстановить из версии' }).click();
  await expect(page.locator('#page-notice')).toBeFocused();
  await expect(page.locator('#page-notice')).toContainText('Черновик восстановлен');
  await expect(page.locator('#restored-note')).toContainText('восстановлен из конфигурационной версии v1');
  // The sealed manifest is still readable with its digest after the restore.
  await expect(digestBlock).toContainText(/^[0-9a-f]{64}$/);
  await expect(page.locator('#candidate-note')).toBeVisible();

  // Keyboard access: the digest block is reachable and the page stays usable.
  await page.locator('#manifest-digest').focus();
  await expect(page.locator('#manifest-title')).toBeVisible();
  const scrolledAgain = await page.evaluate(() => document.documentElement.scrollWidth);
  expect(scrolledAgain).toBeLessThanOrEqual(320);
});
