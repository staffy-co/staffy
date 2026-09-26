import { expect, test } from '@playwright/test';

test('owner can navigate people at 320px and sees honest step-up recovery', async ({ browser }) => {
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  await page.goto('/app/tenants/01994f64-6c00-7000-8000-000000000002');
  await page.getByRole('link', { name: 'Участники' }).click();
  await expect(page.getByRole('heading', { name: 'Участники' })).toBeVisible();
  await expect(page.getByText('Статус: active')).toBeVisible();
  await expect(page.getByText('Пока провайдер недоступен, эти назначения будут отклонены.')).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBe(0);
  for (const control of await page.locator('a:visible, button:visible, input:visible').all()) {
    const box = await control.boundingBox();
    expect(box?.height).toBeGreaterThanOrEqual(44);
    expect(box?.width).toBeGreaterThanOrEqual(44);
  }
  await page.getByLabel('Email получателя').fill('colleague@example.test');
  await page.getByLabel('Owner — все области').check();
  for (const name of ['Настройка', 'Интеграции', 'Операции', 'Проверка', 'Техническая экспертиза', 'Претензии', 'Аудит']) {
    await page.getByRole('group', { name: 'Области' }).getByLabel(name).check();
  }
  await page.getByRole('button', { name: 'Создать приглашение' }).click();
  await expect(page.getByRole('alert')).toContainText('повторное подтверждение личности');
  await page.close();
});
