import { expect, test } from '@playwright/test';

test('owner workspace is honest, keyboard accessible, and reflows at 320px', async ({ browser }) => {
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  await page.goto('/app/tenants/01994f64-6c00-7000-8000-000000000002');

  await expect(page.getByRole('heading', { name: 'Начните с ИИ-сотрудника' })).toBeVisible();
  await expect(page.getByText('Магазин Олега')).toBeVisible();
  await expect(page.getByText('Роль: Owner')).toBeVisible();
  await expect(page.getByText('Сотрудники, каналы и интеграции ещё не настроены.')).toBeVisible();
  const primary = page.getByRole('button', { name: 'Создать ИИ-сотрудника' });
  await expect(primary).toBeVisible();
  await expect(primary).toBeEnabled();

  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBe(0);

  for (const control of await page.locator('a:visible, button:visible, input:visible').all()) {
    const box = await control.boundingBox();
    expect(box?.height).toBeGreaterThanOrEqual(44);
    expect(box?.width).toBeGreaterThanOrEqual(44);
  }

  await page.keyboard.press('Tab');
  const focused = page.locator(':focus');
  await expect(focused).toHaveText('К основному содержанию');
  const outline = await focused.evaluate((element) => getComputedStyle(element).outlineStyle);
  expect(outline).not.toBe('none');

  await page.keyboard.press('Tab');
  await expect(page.locator(':focus')).toHaveAccessibleName('Staffy, главная');

  await page.close();
});

test('guided Employee Brief saves and resumes at 320px with keyboard and explicit blockers', async ({ browser }) => {
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  await page.goto('/app/tenants/01994f64-6c00-7000-8000-000000000002');
  await page.getByRole('button', { name: 'Создать ИИ-сотрудника' }).click();
  await expect(page.getByRole('heading', { name: 'Brief ИИ-сотрудника' })).toBeVisible();
  await expect(page.getByText('Черновик · настройка не завершена')).toBeVisible();
  await expect(page.getByText('Желаемые каналы пока не проверены и не подключены.')).toBeVisible();
  await page.getByLabel('Назначение').fill('Помогать гостям');
  await page.getByLabel('Имя').fill('Аня');
  await page.getByLabel('Языки, по одному на строке').fill('ru');
  await page.getByLabel('Часовой пояс (например, Europe/Moscow)').fill('Europe/Moscow');
  await page.getByLabel('Понедельник').check();
  await page.getByLabel('Начало рабочего дня').fill('09:00');
  await page.getByLabel('Конец рабочего дня').fill('18:00');
  await page.getByLabel('Телефон').check();
  await page.getByLabel('MAX').check();
  await expect(page.getByLabel('Jivo')).toBeVisible();
  await expect(page.getByLabel('Avito')).toBeVisible();
  await expect(page.getByLabel('Чат мобильного приложения')).toBeVisible();
  await page.getByLabel('Основные задачи, по одной на строке').fill('Запись');
  for (const control of await page.locator('a:visible, button:visible, input:visible, textarea:visible').all()) {
    const box = await control.boundingBox();
    expect(box?.height).toBeGreaterThanOrEqual(44);
    expect(box?.width).toBeGreaterThanOrEqual(44);
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0);
  await page.keyboard.press('Tab');
  await expect(page.locator(':focus')).toHaveAccessibleName('Сохранить Brief');
  await expect(page.locator(':focus')).toHaveCSS('outline-style', /^(?!none$).+/);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('status').getByText('Brief сохранён.')).toBeVisible();
  await expect(page.getByLabel('Имя')).toHaveValue('Аня');
  await expect(page.getByText('Версия: 2')).toBeVisible();
  await expect(page.getByText('Следующий шаг: просмотреть план запуска и его блокеры.')).toBeVisible();
  await expect(page.getByRole('listitem').filter({ hasText: 'Domain Pack' })).toContainText('Заблокировано');
  await page.reload();
  await expect(page.getByLabel('Имя')).toHaveValue('Аня');
  await expect(page.getByLabel('Понедельник')).toBeChecked();
  await expect(page.getByLabel('MAX')).toBeChecked();
  await page.close();
});

test('stale editor compares every saved field and explicitly resolves without reload', async ({ browser }) => {
  const first = await browser.newPage({ viewport: { width: 320, height: 800 } });
  const second = await browser.newPage({ viewport: { width: 320, height: 800 } });
  await first.goto('/app/tenants/01994f64-6c00-7000-8000-000000000002/employees');
  await first.getByRole('button', { name: 'Создать ИИ-сотрудника' }).click();
  const draftURL = first.url();
  await second.goto(draftURL);
  async function fillBrief(page: typeof first, name: string) {
    await page.getByLabel('Назначение').fill('Помогать гостям');
    await page.getByLabel('Имя').fill(name);
    await page.getByLabel('Языки, по одному на строке').fill('ru');
    await page.getByLabel('Часовой пояс (например, Europe/Moscow)').fill('Europe/Moscow');
    await page.getByLabel('Понедельник').check();
    await page.getByLabel('Начало рабочего дня').fill('09:00');
    await page.getByLabel('Конец рабочего дня').fill('18:00');
    await page.getByLabel('MAX').check();
    await page.getByLabel('Основные задачи, по одной на строке').fill('Запись');
  }
  await fillBrief(first, 'Версия А');
  await fillBrief(second, 'Версия Б');
  await first.getByRole('button', { name: 'Сохранить Brief' }).click();
  await expect(first.getByRole('status').getByText('Brief сохранён.')).toBeVisible();
  await second.getByRole('button', { name: 'Сохранить Brief' }).click();
  await expect(second.getByRole('alert')).toContainText('Черновик изменён');
  const snapshot = second.locator('.saved-comparison');
  await expect(snapshot).toContainText('Версия А');
  for (const field of ['Назначение', 'Языки', 'Часовой пояс', 'Рабочие дни', 'Начало рабочего дня', 'Конец рабочего дня', 'Желаемые каналы', 'Основные задачи']) {
    await expect(snapshot).toContainText(field);
  }
  await expect(second.getByLabel('Имя')).toHaveValue('Версия Б');
  await expect(second.getByText('мои значения заменят сохранённый Brief')).toBeVisible();
  await second.getByRole('checkbox', { name: /Я сравнил версии/ }).check();
  await second.getByRole('button', { name: 'Заменить сохранённый Brief' }).click();
  await expect(second.getByRole('status').getByText('Brief сохранён.')).toBeVisible();
  await expect(second.getByLabel('Имя')).toHaveValue('Версия Б');
  await first.close();
  await second.close();
});

test('setup names the isolation boundary and has one primary action', async ({ page }) => {
  await page.goto('/app');
  await expect(page.getByRole('heading', { name: 'Создайте пространство компании' })).toBeVisible();
  await expect(page.getByText('ИИ-сотрудник и интеграции пока не создаются.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Создать пространство' })).toHaveCount(1);
  for (const control of await page.locator('a:visible, button:visible, input:visible').all()) {
    const box = await control.boundingBox();
    expect(box?.height).toBeGreaterThanOrEqual(44);
    expect(box?.width).toBeGreaterThanOrEqual(44);
  }
  for (const accessibleName of ['К основному содержанию', 'Staffy, главная', 'Название компании', 'Создать пространство']) {
    await page.keyboard.press('Tab');
    await expect(page.locator(':focus')).toHaveAccessibleName(accessibleName);
    await expect(page.locator(':focus')).toHaveCSS('outline-style', /^(?!none$).+/);
  }
});
