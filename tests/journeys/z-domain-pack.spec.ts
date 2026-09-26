import { expect, test } from '@playwright/test';

test('Domain Slot pins a reference pack and keeps setup blocked at 320px', async ({ browser }) => {
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  await page.goto('/app/tenants/01994f64-6c00-7000-8000-000000000002/employees');
  await page.getByRole('button', { name: 'Создать ИИ-сотрудника' }).click();
  await expect(page.getByRole('heading', { name: 'Domain Pack' })).toBeVisible();
  await expect(page.getByText('Reference packs доступны только для настройки.')).toBeVisible();
  await page.getByLabel('Пакет и версия').selectOption('auto-parts@1.0.0');
  await page.locator('.pack-variants:not([hidden])').getByLabel('Интерес к деталям').selectOption('both');
  const pin = page.getByRole('button', { name: 'Закрепить пакет для настройки' });
  await pin.focus();
  await expect(pin).toHaveCSS('outline-style', /^(?!none$).+/);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('status').getByText('Закреплён пакет auto-parts, версия 1.0.0')).toBeVisible();
  await expect(page.getByText('Domain Pack закреплён для настройки. Следующий шаг: проверить Connectors и знания; runtime пока заблокирован.')).toBeVisible();
  await expect(page.getByRole('listitem').filter({ hasText: 'Публикация' })).toContainText('Заблокировано');
  await page.reload();
  await expect(page.getByRole('status').getByText('Закреплён пакет auto-parts, версия 1.0.0')).toBeVisible();
  await expect(page.locator('.pack-variants:not([hidden])').getByLabel('Интерес к деталям')).toHaveValue('both');
  await page.getByLabel('Пакет и версия').selectOption('auto-parts@2.0.0');
  await expect(page.getByRole('note')).toContainText('schema');
  await expect(page.getByRole('note')).toContainText('policy');
  await expect(page.getByRole('note')).toContainText('eval');
  await page.locator('.pack-variants:not([hidden])').getByLabel('Интерес к деталям').selectOption('both');
  await page.locator('.pack-variants:not([hidden])').getByLabel('Глубина сопоставления').fill('2');
  await page.getByRole('checkbox', { name: /подтверждаю смену пакета или версии/ }).check();
  await page.getByRole('button', { name: 'Сохранить подтверждённую версию' }).click();
  await expect(page.getByRole('status').getByText('Закреплён пакет auto-parts, версия 2.0.0')).toBeVisible();
  for (const slot of ['Domain Pack', 'Connectors', 'Знания и поведение']) {
    await expect(page.getByRole('listitem').filter({ hasText: slot })).toContainText('Требует проверки');
  }
  await expect(page.getByRole('alert', { name: '' })).toContainText('connectors');
  await expect(page.locator('#pack-review')).toBeFocused();
  const confirmChange=page.getByRole('checkbox', { name: /подтверждаю смену пакета или версии/ });
  await confirmChange.check();
  await page.getByLabel('Пакет и версия').selectOption('montagio-services@1.0.0');
  await expect(confirmChange).not.toBeChecked();
  await expect(page.getByRole('note')).toContainText('domain');
  await page.locator('.pack-variants:not([hidden])').getByLabel('Инженерная сантехника').selectOption('true');
  await page.locator('.pack-variants:not([hidden])').getByLabel('Отопление').selectOption('false');
  await page.getByRole('checkbox', { name: /подтверждаю смену пакета или версии/ }).check();
  await page.getByRole('button', { name: 'Сохранить подтверждённую версию' }).click();
  await expect(page.getByRole('status').getByText('Закреплён пакет montagio-services, версия 1.0.0')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0);
  for (const control of await page.locator('.pack-form button:visible, .pack-form input:visible, .pack-form select:visible').all()) {
    const box = await control.boundingBox();
    expect(box?.height).toBeGreaterThanOrEqual(44);
    expect(box?.width).toBeGreaterThanOrEqual(44);
  }
  await page.close();
});
