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
  await expect(primary).toBeDisabled();
  await expect(page.getByText('Станет доступно на следующем шаге настройки.')).toBeVisible();

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
