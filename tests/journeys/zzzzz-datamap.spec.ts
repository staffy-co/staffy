import { expect, test } from '@playwright/test';

const tenantPath = '/app/tenants/01994f64-6c00-7000-8000-000000000002/datamap';

test('data map documents classes, runs a confirmed deletion and shows an honest receipt at 320px with keyboard', async ({ browser }) => {
  test.setTimeout(90_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  await page.goto(tenantPath);

  // The honest registry opens with every class of the system dictionary.
  await expect(page.getByRole('heading', { name: 'Карта данных (Artifact Data Map)' })).toBeVisible();
  const table = page.locator('.datamap-table').first();
  await expect(table.getByRole('columnheader', { name: 'Класс' })).toBeVisible();
  await expect(table.getByRole('columnheader', { name: 'Статус карты' })).toBeVisible();
  await expect(table.locator('tr', { hasText: 'knowledge-source-payload' })).toBeVisible();
  await expect(table.locator('tr', { hasText: 'connection-secret-seal' })).toBeVisible();
  await expect(table.locator('.map-undocumented .map-state').first()).toHaveText('не описан');

  // Publish a complete map entry for the knowledge class through the form.
  const entryForm = page.locator('details.map-form', { hasText: 'Новая версия записи класса' });
  await entryForm.locator('summary').click();
  await entryForm.getByLabel('Класс артефактов').selectOption('knowledge-source-payload');
  await entryForm.getByLabel(/Назначение обработки/).fill('Обработка обращений магазина');
  await entryForm.getByLabel('Правовое основание').fill('Договор с клиентом');
  await entryForm.getByLabel('Контролёр данных').fill('ООО Магазин Олега');
  await entryForm.getByLabel('Обработчик данных').fill('Staffy');
  await entryForm.getByLabel(/Получатели/).fill('Сотрудники магазина');
  await entryForm.getByLabel(/Суб-обработчики/).fill('Хостинг Staffy');
  await entryForm.getByLabel(/Место обработки и хранения/).fill('Локальная база данных (EU)');
  await entryForm.locator('label', { hasText: ' Owner' }).locator('input[type=checkbox]').check();
  await entryForm.getByLabel(/Известные копии/).fill('postgres: employee');
  await entryForm.getByLabel(/Ссылка на retention-политику/).fill('ret-knowledge-01');
  await entryForm.getByLabel('Способ удаления').fill('Локальная очистка модулем employee');
  await entryForm.getByLabel('Способ экспорта').fill('Выгрузка администратором');
  await entryForm.getByRole('button', { name: 'Опубликовать запись' }).click();

  // The notice is announced and focused, the class turns complete.
  const notice = page.locator('#page-notice');
  await expect(notice).toBeFocused();
  await expect(notice).toContainText('Запись карты данных опубликована');
  await expect(table.locator('tr', { hasText: 'knowledge-source-payload' }).locator('.map-state')).toHaveText('заполнена');

  // Publish a retention policy for the same class (zero terms: due now).
  const policyForm = page.locator('details.map-form', { hasText: 'Новая версия политики класса' });
  await policyForm.locator('summary').click();
  await policyForm.getByLabel(/Класс артефактов/).selectOption('knowledge-source-payload');
  await policyForm.getByLabel('Назначение политики').fill('Хранение источников знаний');
  await policyForm.getByLabel('Правовое основание').fill('Баланс интересов');
  await policyForm.getByLabel('Источник').selectOption('tenant_decision');
  await policyForm.getByLabel(/Терминальное событие отсчёта/).selectOption('source_retired');
  await policyForm.getByLabel(/Активное использование, дней/).fill('0');
  await policyForm.getByLabel(/Архив, дней/).fill('0');
  await policyForm.getByLabel(/Максимальный срок хранения, дней/).fill('365');
  await policyForm.getByLabel(/Истечение копий в бэкапах, дней/).fill('30');
  await policyForm.getByLabel('Исключение legal hold').fill('Судебное удержание продлевает срок');
  await policyForm.getByLabel('Владелец политики').fill('Владелец данных магазина');
  await policyForm.getByRole('button', { name: 'Опубликовать политику' }).click();
  await expect(page.locator('#page-notice')).toContainText('Retention-политика опубликована');

  // The deletion form guards the client with the required checkbox; the
  // server must also stay fail-closed when a client bypasses validation.
  const deletionForm = page.locator('form.deletion-form');
  await deletionForm.locator('label', { hasText: 'Источники знаний и их содержимое' }).locator('input[type=checkbox]').check();
  await deletionForm.locator('input[name=confirm_deletion]').evaluate((el) => el.removeAttribute('required'));
  await deletionForm.getByRole('button', { name: 'Выполнить удаление' }).click();
  await expect(page.locator('.alert')).toContainText('Подтвердите необратимое удаление');

  // Confirmed, the workflow runs and lands on the honest receipt page. The
  // rejected attempt re-rendered the form, so the class is picked again.
  await deletionForm.locator('label', { hasText: 'Источники знаний и их содержимое' }).locator('input[type=checkbox]').check();
  await deletionForm.locator('label', { hasText: 'Подтверждаю необратимое удаление' }).locator('input[type=checkbox]').check();
  await deletionForm.getByRole('button', { name: 'Выполнить удаление' }).click();
  const heading = page.locator('#receipt-heading');
  await expect(heading).toBeVisible();
  await expect(heading).toBeFocused();
  await expect(page.locator('.outcome-list')).toContainText('удалено');
  await expect(page.locator('.receipt-hash')).toBeVisible();
  // The receipt never echoes deleted content, only counters and the hash.
  await expect(page.locator('.outcome-list')).not.toContainText('Обработка обращений магазина');

  // The way back to the map stays keyboard-focusable.
  const back = page.getByRole('link', { name: 'Вернуться к карте данных' });
  await expect(back).toBeVisible();
  await back.focus();
  await expect(back).toBeFocused();

  // The map lists the receipt with its honest summary.
  await back.click();
  await expect(page.locator('.datamap-table').last().locator('tbody tr').first()).toContainText('выполнено');
});
