import { expect, test } from '@playwright/test';

test('knowledge row previews versions, stale and conflict at 320px with keyboard', async ({ browser }) => {
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  await page.goto('/app/tenants/01994f64-6c00-7000-8000-000000000002/employees');
  await page.getByRole('button', { name: 'Создать ИИ-сотрудника' }).click();
  // The shared fixture world may carry a pinned pack AND a draft mid-settle:
  // the heading races the page render, so the guard WAITS for it briefly and
  // pins only when the pin button itself is present (the pinned world never
  // renders the button — the heading is the truth, the button is the need).
  const knowledgeHeading = page.getByRole('heading', { name: 'Источники знаний' });
  if (!(await knowledgeHeading.waitFor({ state: 'visible', timeout: 5000 }).then(() => true, () => false))) {
    await page.getByLabel('Пакет и версия').selectOption('auto-parts@1.0.0');
    await page.locator('.pack-variants:not([hidden])').getByLabel('Интерес к деталям').selectOption('both');
    await page.getByRole('button', { name: 'Закрепить пакет для настройки' }).click();
  }
  await expect(page.getByRole('heading', { name: 'Источники знаний' })).toBeVisible();
  const textForm = page.locator('form.knowledge-form').filter({ hasText: 'Добавить текстовый источник' });
  const ingestForm = page.locator('form.knowledge-form').filter({ hasText: 'Добавить страницу или документ' });
  await expect(page.getByText('TXT до 256 КиБ / PDF с текстовым слоем до 2 МиБ')).toBeVisible();
  await expect(ingestForm.getByLabel('Точный HTTPS-адрес (для страницы)')).toBeVisible();
  await expect(ingestForm.getByLabel('TXT или PDF (для документа)')).toBeHidden();
  await expect(ingestForm.getByLabel('Точный HTTPS-адрес (для страницы)')).toHaveAttribute('required', '');
  await ingestForm.getByLabel('Источник').selectOption('document');
  await expect(ingestForm.getByLabel('Точный HTTPS-адрес (для страницы)')).toBeHidden();
  await expect(ingestForm.getByLabel('TXT или PDF (для документа)')).toBeVisible();
  await expect(ingestForm.getByLabel('TXT или PDF (для документа)')).toHaveAttribute('required', '');
  await expect(ingestForm.getByText('файл не сохранится')).toBeVisible();
  for (const control of await ingestForm.locator('button:visible, input:visible, select:visible').all()) {
    const box = await control.boundingBox();
    expect(box?.height).toBeGreaterThanOrEqual(44);
  }
  await textForm.getByLabel('Название', { exact: true }).fill('Сервисный FAQ');
  await textForm.getByLabel('Владелец', { exact: true }).fill('Редактор');
  await textForm.getByLabel('Происхождение', { exact: true }).fill('Внутренний справочник');
  await textForm.getByLabel('Область применения').fill('Общие ответы');
  await textForm.getByLabel('Актуально до (RFC3339, необязательно)').fill('2026-09-15T00:00:00Z');
  const create = textForm.getByRole('button', { name: 'Создать источник' });
  await create.focus();
  await expect(create).toHaveCSS('outline-style', /^(?!none$).+/);
  await page.keyboard.press('Enter');
  const row = page.locator('.knowledge-row').filter({ hasText: 'Сервисный FAQ' });
  await expect(row).toContainText('Версия записи: 1');
  await expect(row).toContainText('Текущая версия текста: 0');
  await expect(row).toContainText('Внутренний справочник');
  await row.getByLabel('Текст UTF-8 (до 256 КиБ)').fill('Да');
  await row.getByRole('button', { name: 'Обработать текст' }).click();
  await expect(row).toContainText('Источник устарел: срок актуальности истёк');
  await expect(row).toContainText('блокирует публикацию');
  await row.getByText('Версия 1 · checksum').click();
  await expect(row.getByText('Полный текст:')).toBeVisible();
  await expect(row.getByText('Да', { exact: true })).toBeVisible();
  await row.getByLabel('Текст UTF-8 (до 256 КиБ)').fill('Нет');
  await row.getByRole('button', { name: 'Обработать текст' }).click();
  await row.getByText('Версия 2 · checksum').click();
  await expect(row).toContainText('Удалено относительно предыдущей версии');
  await expect(row).toContainText('Добавлено относительно предыдущей версии');
  await row.getByLabel('Причина конфликта').fill('В новом регламенте иначе');
  await row.getByLabel('Происхождение второго утверждения').fill('Сообщение владельца');
  await row.getByRole('button', { name: 'Отметить конфликт' }).click();
  await expect(row).toContainText('Исходное происхождение');
  await expect(row).toContainText('Внутренний справочник');
  await expect(row).toContainText('Конфликтующее происхождение');
  await expect(row).toContainText('Сообщение владельца');
  await expect(row).toContainText('блокирует публикацию');
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0);
  for (const control of await row.locator('button:visible, input:visible, textarea:visible, summary:visible').all()) {
    const box = await control.boundingBox();
    expect(box?.height).toBeGreaterThanOrEqual(44);
    expect(box?.width).toBeGreaterThanOrEqual(44);
  }
  await row.getByRole('button', { name: 'Вывести из эксплуатации' }).click();
  await expect(row).toContainText('Источник выведен из эксплуатации');
  await page.close();
});

test('synthetic source visibly moves from processing to non-publishable preview', async ({ browser }) => {
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  await page.goto('/app/tenants/01994f64-6c00-7000-8000-000000000002/employees');
  await page.getByRole('button', { name: 'Создать ИИ-сотрудника' }).click();
  // The same settling guard as the first row: the pinned world answers the
  // heading (never the pin button) — the wait kills the render race.
  const knowledgeHeading2 = page.getByRole('heading', { name: 'Источники знаний' });
  if (!(await knowledgeHeading2.waitFor({ state: 'visible', timeout: 5000 }).then(() => true, () => false))) {
    await page.getByLabel('Пакет и версия').selectOption('auto-parts@1.0.0');
    await page.locator('.pack-variants:not([hidden])').getByLabel('Интерес к деталям').selectOption('both');
    await page.getByRole('button', { name: 'Закрепить пакет для настройки' }).click();
    await expect(knowledgeHeading2).toBeVisible();
  }
  await page.getByRole('button', { name: 'Добавить тестовый источник' }).click();
  const row = page.locator('.knowledge-row').filter({ hasText: 'Тестовый документ' });
  await expect(row).toContainText('Статус: processing');
  await row.getByRole('button', { name: 'Обработать тестовый источник' }).click();
  await expect(row).toContainText('Статус: preview');
  await expect(row).toContainText('публикация заблокирована');
  await row.getByLabel('Версия тестового текста').selectOption('sample-v2');
  await row.getByRole('button', { name: 'Обработать тестовый источник' }).click();
  await expect(row).toContainText('Текущая версия текста: 2');
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0);
  await page.close();
});
