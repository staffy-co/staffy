import { expect, test } from '@playwright/test';

const tenantPath = '/app/tenants/01994f64-6c00-7000-8000-000000000002/integrations';

test('integrations catalog shows honest statuses and connects a synthetic account at 320px with keyboard', async ({ browser }) => {
  test.setTimeout(90_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  await page.goto(tenantPath);

  // Catalog: every card exposes its support status as text and unimplemented
  // adapters never offer a connect form.
  await expect(page.getByRole('heading', { name: 'Каталог Connector' })).toBeVisible();
  await expect(page.locator('.connector-status').filter({ hasText: 'supported' })).toBeVisible();
  await expect(page.locator('.connector-status').filter({ hasText: 'candidate' }).first()).toBeVisible();
  await expect(page.locator('.connector-status').filter({ hasText: 'deferred' }).first()).toBeVisible();
  await expect(page.locator('.connector-status').filter({ hasText: 'unavailable' })).toBeVisible();
  await expect(page.getByText('Подключение недоступно: адаптер не реализован').first()).toBeVisible();
  await expect(page.getByText('Приоритетный кандидат телефонии Montagio')).toHaveCount(0);

  // Only the synthetic card opens a connect form; keyboard reaches it.
  const synthetic = page.locator('.connector-card', { hasText: 'Staffy Synthetic Telephony' });
  await synthetic.locator('summary').focus();
  await page.keyboard.press('Enter');
  await expect(synthetic.getByLabel('Назначение подключения')).toBeVisible();
  await expect(synthetic.getByLabel('Endpoint (обязательно)')).toBeVisible();
  await expect(synthetic.getByLabel('API-ключ (sandbox)')).toHaveCount(0);

  // Production environment is visibly closed.
  const environment = synthetic.getByLabel('Среда');
  await expect(environment.locator('option[value="production"]')).toBeDisabled();

  await synthetic.getByLabel('Назначение подключения').fill('Тестовая телефония Олега');
  await synthetic.getByLabel('Endpoint (обязательно)').fill('https://demo.sandbox.example');
  await synthetic.getByLabel('Метка транка').fill('Основной транк');
  await synthetic.getByRole('button', { name: 'Создать черновик подключения' }).click();

  // The draft connection appears with pending states, no false success.
  const connection = page.locator('.connection-list .employee-card', { hasText: 'Тестовая телефония Олега' });
  await expect(connection).toContainText('draft');
  await expect(connection).toContainText('pending');
  await expect(connection).toContainText('Черновик: секрет ещё не проверялся');

  // Submit the secret once through the keyboard-accessible form.
  await connection.locator('summary', { hasText: 'Подключить секрет' }).click();
  const secretField = connection.getByLabel('API-ключ (sandbox)');
  await secretField.fill('sandbox-journey-key');
  await expect(connection.getByText('Значение принимается один раз и больше не показывается')).toBeVisible();
  await connection.getByRole('button', { name: 'Проверить и подключить' }).click();

  // The page re-renders with the honest connected/unverified status and only
  // the fingerprint of the secret.
  const updated = page.locator('.connection-list .employee-card', { hasText: 'Тестовая телефония Олега' });
  await expect(updated).toContainText('connected_unverified');
  await expect(updated).toContainText('Аутентификация: verified');
  await expect(updated).toContainText('Транспорт: verified');
  await expect(updated).toContainText('возможности не проверены');
  await expect(updated).toContainText('отпечаток a1b2c3d4e5f6');
  await expect(updated).toContainText('значение недоступно');
  await expect(page.locator('.connection-list')).not.toContainText('sandbox-journey-key');
  for (const input of await page.locator('.connection-list input[type="password"]').all()) {
    await expect(input).toHaveValue('');
  }

  // A failed secret produces an honest failure without false success.
  await synthetic.locator('summary').click();
  await synthetic.getByLabel('Назначение подключения').fill('Второй аккаунт');
  await synthetic.getByLabel('Endpoint (обязательно)').fill('https://two.sandbox.example');
  await synthetic.getByLabel('Метка транка').fill('Резерв');
  await synthetic.getByRole('button', { name: 'Создать черновик подключения' }).click();
  const secondCard = page.locator('.connection-list .employee-card', { hasText: 'Второй аккаунт' });
  await secondCard.locator('summary', { hasText: 'Подключить секрет' }).click();
  await secondCard.getByLabel('API-ключ (sandbox)').fill('wrong-production-key');
  await secondCard.getByRole('button', { name: 'Проверить и подключить' }).click();
  const failedCard = page.locator('.connection-list .employee-card', { hasText: 'Второй аккаунт' });
  await expect(failedCard).toContainText('failed');
  await expect(failedCard).toContainText('не пройдена');
  await expect(page.locator('.connection-list')).not.toContainText('wrong-production-key');

  // Rotation is offered for the sealed connection and revokes the old secret.
  const sealedCard = page.locator('.connection-list .employee-card', { hasText: 'Тестовая телефония Олега' });
  await sealedCard.locator('summary', { hasText: 'Ротация секрета' }).click();
  await sealedCard.getByLabel('API-ключ (sandbox)').fill('sandbox-rotated-journey');
  await sealedCard.getByRole('button', { name: 'Заменить секрет' }).click();
  const rotatedCard = page.locator('.connection-list .employee-card', { hasText: 'Тестовая телефония Олега' });
  await expect(rotatedCard).toContainText('connected_unverified');
  await expect(rotatedCard).toContainText('отпечаток a1b2c3d4e5f6');

  // Revoke requires an explicit confirmation and is terminal.
  await rotatedCard.locator('summary', { hasText: 'Отозвать подключение' }).click();
  await rotatedCard.getByLabel('Причина отзыва').fill('Тест завершён');
  await rotatedCard.getByRole('checkbox', { name: 'Подтверждаю отзыв этого подключения' }).check();
  await rotatedCard.getByRole('button', { name: 'Отозвать', exact: true }).click();
  const confirmCard = page.locator('.connection-list .employee-card', { hasText: 'Тестовая телефония Олега' });
  await expect(confirmCard).toContainText('revoked');

  // Controls keep 44px targets at 320px.
  for (const control of await page.locator('.connection-list button, .connector-connect button').all()) {
    const box = await control.boundingBox();
    if (box) {
      expect(box.height).toBeGreaterThanOrEqual(44);
    }
  }
});

test('capability state shows declared vs proven, verifies a read-only probe, blocks mutating capabilities honestly', async ({ browser }) => {
  test.setTimeout(90_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  await page.goto(tenantPath);

  // Create and connect the synthetic account.
  const synthetic = page.locator('.connector-card', { hasText: 'Staffy Synthetic Telephony' });
  await synthetic.locator('summary').focus();
  await page.keyboard.press('Enter');
  await synthetic.getByLabel('Назначение подключения').fill('Возможности Олега');
  await synthetic.getByLabel('Endpoint (обязательно)').fill('https://caps.sandbox.example');
  await synthetic.getByRole('button', { name: 'Создать черновик подключения' }).click();
  const connection = page.locator('.connection-list .employee-card', { hasText: 'Возможности Олега' });
  await connection.locator('summary', { hasText: 'Подключить секрет' }).click();
  await connection.getByLabel('API-ключ (sandbox)').fill('sandbox-caps-journey');
  await connection.getByRole('button', { name: 'Проверить и подключить' }).click();

  // The capability panel appears with declared vs proven semantics.
  const connected = page.locator('.connection-list .employee-card', { hasText: 'Возможности Олега' });
  const panel = connected.locator('.capability-state');
  await expect(panel.getByRole('heading', { name: 'Возможности (Capability State)' })).toBeVisible();
  await expect(panel).toContainText('Заявленные контрактом 1.1.0');
  const trunk = panel.locator('.capability', { hasText: 'Статус транка' });
  await expect(trunk.locator('.cap-state')).toHaveText('не проверено');
  await expect(trunk).toContainText('исходящая · query');
  await expect(trunk).toContainText('таймаут 5000 мс');

  // Mutating capability carries the honest managed block, no verify button.
  const sms = panel.locator('.capability', { hasText: 'Отправка SMS' });
  await expect(sms.locator('.cap-state')).toHaveText('не проверено');
  await expect(sms.locator('.cap-block')).toContainText('требуется Action Gateway (Epic 3)');
  await expect(sms.locator('.cap-block')).toContainText('гейт AG-4');
  await expect(sms.locator('.cap-block')).toContainText('Technical Lead + Security');
  await expect(sms.locator('.cap-verify')).toHaveCount(0);

  // Inbound event stays honestly unverified without a block button.
  const call = panel.locator('.capability', { hasText: 'Входящий звонок' });
  await expect(call.locator('.cap-state')).toHaveText('не проверено');
  await expect(call).toContainText('путь проверки не реализован в текущем срезе');

  // Keyboard: the verify control reaches 44px and announces the update.
  const verifyButton = trunk.getByRole('button', { name: 'Проверить «Статус транка»' });
  const box = await verifyButton.boundingBox();
  if (box) {
    expect(box.height).toBeGreaterThanOrEqual(44);
  }
  await verifyButton.click();

  // The state flips to verified inside a live region, evidence discloses progressively.
  const verifiedPanel = page.locator('.connection-list .employee-card', { hasText: 'Возможности Олега' }).locator('.capability-state');
  await expect(verifiedPanel.locator('.capability', { hasText: 'Статус транка' }).locator('.cap-state')).toHaveText('проверено');
  await expect(page.locator('.status')).toContainText('Проверка возможности выполнена');
  // The update is announced without losing focus: the notice receives it after the redirect.
  await expect(page.locator('#page-notice')).toBeFocused();
  const evidence = verifiedPanel.locator('.capability', { hasText: 'Статус транка' }).locator('.capability-evidence');
  await evidence.locator('summary').click();
  await expect(evidence).toContainText('Уровень доставки');
  await expect(evidence).toContainText('delivered');
  await expect(evidence).toContainText('Действует до');
  // Redacted fields never carry the secret.
  await expect(evidence).not.toContainText('sandbox-caps-journey');

  // The mutating block survives independent verification of the read-only
  // capability: read-only proof does not leak into send-sms.
  const smsAfter = verifiedPanel.locator('.capability', { hasText: 'Отправка SMS' });
  await expect(smsAfter.locator('.cap-state')).toHaveText('не проверено');
  await expect(smsAfter.locator('.cap-verify')).toHaveCount(0);

  // Rotation marks the proof stale with the concrete reason.
  await page.locator('.connection-list .employee-card', { hasText: 'Возможности Олега' }).locator('summary', { hasText: 'Ротация секрета' }).click();
  await page.locator('.connection-list .employee-card', { hasText: 'Возможности Олега' }).getByLabel('API-ключ (sandbox)').fill('sandbox-rotated-caps');
  await page.locator('.connection-list .employee-card', { hasText: 'Возможности Олега' }).getByRole('button', { name: 'Заменить секрет' }).click();
  const rotatedPanel = page.locator('.connection-list .employee-card', { hasText: 'Возможности Олега' }).locator('.capability-state');
  await expect(rotatedPanel.locator('.capability', { hasText: 'Статус транка' }).locator('.cap-state')).toHaveText('устарело');
  await expect(rotatedPanel.locator('.capability', { hasText: 'Статус транка' })).toContainText('секрет ротирован или отозван');
});
