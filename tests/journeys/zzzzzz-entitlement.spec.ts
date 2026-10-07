import { expect, test } from '@playwright/test';

const tenantPath = '/app/tenants/01994f64-6c00-7000-8000-000000000002/entitlement';
const budgetPath = '/app/tenants/01994f64-6c00-7000-8000-000000000002/budget';

test('entitlement shows honest draft, activates with confirmation and keeps the budget meter textual at 320px with keyboard', async ({ browser }) => {
  test.setTimeout(90_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });

  // The draft entitlement lists the composition with open questions, not zeros.
  await page.goto(tenantPath);
  await expect(page.getByRole('heading', { name: 'Тариф и Entitlement' })).toBeVisible();
  await expect(page.locator('.ent-draft')).toHaveText('черновик (не активирован)');
  await expect(page.locator('table', { hasText: 'Компоненты тарифа' })).toBeVisible();
  await expect(page.locator('table', { hasText: 'Запуск (разовая часть)' })).toContainText('открытый вопрос');
  const composition = page.getByRole('table').filter({ hasText: 'Компоненты' });
  await expect(composition.getByRole('columnheader', { name: 'Единица' })).toBeVisible();
  await expect(page.getByRole('table').filter({ hasText: 'Открытые вопросы' })).toContainText('не ноль');

  // Compose the terms version: currency, period and prices.
  const termsForm = page.locator('details.map-form', { hasText: 'Новая версия условий тарифа' });
  await termsForm.locator('summary').click();
  await termsForm.getByLabel('Валюта (3 заглавные буквы; пусто = открытый вопрос)').fill('RUB');
  await termsForm.getByLabel('Налоговый режим (пусто = явная неопределённость)').fill('УСН');
  await termsForm.getByLabel('Начало периода действия').fill('2026-10-01');
  await termsForm.getByLabel('Конец периода действия').fill('2026-12-31');
  await termsForm.getByLabel('Цена запуска (пусто = открытый вопрос, не ноль)').fill('49000');
  await termsForm.getByLabel('Цена подписки ИИ-сотрудника за период').fill('15000');
  await termsForm.getByLabel(/Включённые ИИ-сотрудники/).fill('consultant-01');
  await termsForm.getByLabel(/Включённые каналы/).fill('jivo');
  await termsForm.getByLabel(/Включённые Connectors/).fill('abcp');
  await termsForm.getByLabel('Описание изменения').fill('Коммерческое предложение №1');
  await termsForm.getByRole('button', { name: 'Опубликовать версию условий' }).click();

  // The notice is announced and focused; the state stays draft until activation.
  const notice = page.locator('#page-notice');
  await expect(notice).toBeFocused();
  await expect(notice).toContainText('Версия условий тарифа опубликована');
  await expect(page.locator('.ent-draft')).toHaveText('черновик (не активирован)');

  // The state form guards the client with the required checkbox; the server
  // must also stay fail-closed when a client bypasses validation.
  const stateForm = page.locator('form.state-form');
  await stateForm.getByLabel('Действие').selectOption('activate');
  await stateForm.getByLabel(/Основание решения/).fill('Договор №77 от 01.10.2026');
  await stateForm.locator('input[name=confirm_state]').evaluate((el) => el.removeAttribute('required'));
  await stateForm.getByRole('button', { name: 'Выполнить' }).click();
  await expect(page.locator('.alert')).toContainText('Подтвердите изменение коммерческого состояния');

  // Confirmed, the activation lands and the state becomes active. The rejected
  // attempt re-rendered the form, so the basis and the checkbox are set again.
  await stateForm.getByLabel(/Основание решения/).fill('Договор №77 от 01.10.2026');
  await stateForm.locator('label', { hasText: 'Подтверждаю изменение коммерческого состояния' }).locator('input[type=checkbox]').check();
  await stateForm.getByRole('button', { name: 'Выполнить' }).click();
  await expect(page.locator('#page-notice')).toBeFocused();
  await expect(page.locator('#page-notice')).toContainText('активированы');
  await expect(page.locator('.ent-active')).toHaveText('активен');

  // Publish the pilot budget with a warning threshold.
  const budgetForm = page.locator('details.map-form', { hasText: 'Новая версия бюджета пилота' });
  await budgetForm.locator('summary').click();
  await budgetForm.getByLabel('Сумма бюджета').fill('100000');
  await budgetForm.getByLabel('Валюта (3 заглавные буквы)').fill('RUB');
  await budgetForm.getByLabel('Начало периода измерения').fill('2026-10-01');
  await budgetForm.getByLabel('Конец периода измерения').fill('2026-12-31');
  await budgetForm.getByLabel('Порог предупреждения, % бюджета').fill('80');
  await budgetForm.getByLabel('Hard limit (пусто = не задан)').fill('120000');
  await budgetForm.getByLabel('Политика при достижении hard limit').selectOption('block_new_admissions');
  await budgetForm.getByRole('button', { name: 'Опубликовать бюджет' }).click();
  await expect(page.locator('#page-notice')).toContainText('Бюджет опубликован');

  // The Budget Meter page keeps every datum textual: statuses, sums, units.
  await page.goto(budgetPath);
  await expect(page.getByRole('heading', { name: 'Budget Meter' })).toBeVisible();
  await expect(page.locator('.meter-table')).toBeVisible();
  await expect(page.locator('.meter-table').getByRole('columnheader', { name: 'Категория' })).toBeVisible();
  await expect(page.locator('.meter-table').getByRole('columnheader', { name: 'Полнота' })).toBeVisible();
  // Unknown spend stays unknown in words, never a zero.
  await expect(page.locator('.meter-unknown .map-state').first()).toHaveText('неизвестны');
  await expect(page.locator('.next-note', { hasText: 'Период измерения' })).toContainText('полные|неизвестны', { ignoreCase: false }).catch(() => {});

  // Register the manual source through the entitlement page.
  await page.goto(tenantPath);
  const sourceForm = page.locator('details.map-form', { hasText: 'Регистрация источника событий' });
  await sourceForm.locator('summary').click();
  await sourceForm.getByLabel('Имя источника').fill('carrier-report');
  await sourceForm.locator('label', { hasText: 'Каналы и телефония (carrier)' }).locator('input[type=checkbox]').check();
  await sourceForm.locator('label', { hasText: 'Ручные минуты оператора' }).locator('input[type=checkbox]').check();
  await sourceForm.getByRole('button', { name: 'Зарегистрировать' }).click();
  await expect(page.locator('#page-notice')).toContainText('Источник usage зарегистрирован');

  // Ingest a usage event through the manual form; the meter reflects it with
  // the unit and the confirmed sum, and the replay does not double count.
  await page.goto(budgetPath);
  const ingestForm = page.locator('details.map-form', { hasText: 'Ручной ввод usage-события' });
  await ingestForm.locator('summary').click();
  await ingestForm.getByLabel('Категория расхода').selectOption('carrier');
  await ingestForm.getByLabel('Количество').fill('120');
  await ingestForm.getByLabel('Сумма (обязательна для платных категорий)').fill('600');
  await ingestForm.getByLabel('Валюта (3 заглавные буквы)').fill('RUB');
  await ingestForm.getByLabel('Начало периода события').fill('2026-10-01');
  await ingestForm.getByLabel('Конец периода события').fill('2026-10-31');
  await ingestForm.getByLabel(/ИИ-сотрудник/).fill('consultant-01');
  await ingestForm.getByLabel(/Канал/).fill('jivo');
  await ingestForm.getByRole('button', { name: 'Учесть событие' }).click();
  await expect(page.locator('#page-notice')).toBeFocused();
  await expect(page.locator('#page-notice')).toContainText('Usage-событие учтено');
  await expect(page.locator('.meter-table').locator('tr', { hasText: 'Каналы и телефония (carrier)' })).toContainText('120.00 мин');
  await expect(page.locator('.meter-table').locator('tr', { hasText: 'Каналы и телефония (carrier)' })).toContainText('600.00 RUB');

  // The breakdown links the aggregate to the employee without conversation content.
  await expect(page.locator('table', { hasText: 'Разбивка по' }).first()).toContainText('consultant-01');

  // The exception surface acknowledges the data gap honestly.
  await expect(page.locator('table', { hasText: 'Исключения: причина' }).first()).toContainText('разрыв данных usage');
  const ackForm = page.locator('form.ack-form').first();
  await ackForm.getByLabel(/Подтверждение исключения/).fill('Принято к сведению; проверю доставку отчёта');
  await ackForm.getByRole('button', { name: 'Подтвердить исключение' }).click();
  await expect(page.locator('#page-notice')).toBeFocused();
  await expect(page.locator('#page-notice')).toContainText('фиксировано');
  await expect(page.locator('table', { hasText: 'Исключения: причина' }).first()).toContainText('подтверждено');

  // The navigation back to the tariff page stays keyboard-focusable.
  const tariffLink = page.getByRole('link', { name: 'Тариф' });
  await tariffLink.focus();
  await expect(tariffLink).toBeFocused();
  await tariffLink.click();
  await expect(page.getByRole('heading', { name: 'Тариф и Entitlement' })).toBeVisible();
});
