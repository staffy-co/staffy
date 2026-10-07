import { expect, test } from '@playwright/test';

const employeePath = '/app/tenants/01994f64-6c00-7000-8000-000000000002/employees';
const profilePath = `${employeePath}/01994f64-6c00-7000-8000-000000000004/operating-profile`;
const releasePath = `${employeePath}/01994f64-6c00-7000-8000-000000000004/release`;

const profileContent = {
  work_scope: {
    routes: [{ channel: 'phone', direction: 'inbound' }, { channel: 'web_chat', direction: 'inbound' }],
    forbidden_routes: [{ route: { channel: 'telegram', direction: 'outbound' }, reason: 'Канал не подключён и не оплачивается.' }],
    languages: ['ru'],
    window: { days: ['mon', 'tue'], start: '09:00', end: '18:00', time_zone: 'Europe/Moscow' },
    limits: {
      max_concurrent_conversations_tenant: 4,
      max_concurrent_calls_tenant: 2,
      per_route: [
        { route: { channel: 'phone', direction: 'inbound' }, max_concurrent_conversations: 2, max_concurrent_calls: 1 },
        { route: { channel: 'web_chat', direction: 'inbound' }, max_concurrent_conversations: 2 }
      ]
    },
    admission_stop_conditions: ['Достижение hard limit бюджета.', 'Перевод Entitlement в paused.']
  },
  human_coverage: [
    { route: { channel: 'phone', direction: 'inbound' }, response_owner_id: '01994f64-6c00-7000-8000-000000000001', backup_owner_id: '01994f64-6c00-7000-8000-000000000001', days: ['mon', 'tue'], start: '09:00', end: '18:00', ack_deadline_minutes: 15, fallback: 'voicemail', fallback_evidence: 'capability://jivo/voicemail/verified' },
    { route: { channel: 'web_chat', direction: 'inbound' }, response_owner_id: '01994f64-6c00-7000-8000-000000000001', backup_owner_id: '01994f64-6c00-7000-8000-000000000001', days: ['mon', 'tue'], start: '09:00', end: '18:00', ack_deadline_minutes: 10, fallback: 'human_callback' }
  ],
  privacy_assignments: [
    { route: { channel: 'phone', direction: 'inbound' }, purpose: 'recording', in_use: true, legal_basis: 'Договор с клиентом', consent_ref: 'consent://tenant/recording', disclosure: 'Озвучивается в начале звонка', retention: '90 дней', deletion: 'Workflow Story 1-10', withdrawal_behavior: 'Остановка по запросу клиента', oq11_state: 'pass', oq11_evidence: 'privacy://oq11/recording-pass' },
    { route: { channel: 'phone', direction: 'inbound' }, purpose: 'stt_model_processing', in_use: false, oq11_state: 'open' },
    { route: { channel: 'phone', direction: 'inbound' }, purpose: 'media_processing', in_use: false, oq11_state: 'open' },
    { route: { channel: 'phone', direction: 'inbound' }, purpose: 'transfer', in_use: false, oq11_state: 'open' },
    { route: { channel: 'phone', direction: 'inbound' }, purpose: 'marketing_contact', in_use: false, oq11_state: 'open' },
    { route: { channel: 'web_chat', direction: 'inbound' }, purpose: 'recording', in_use: false, oq11_state: 'open' },
    { route: { channel: 'web_chat', direction: 'inbound' }, purpose: 'stt_model_processing', in_use: false, oq11_state: 'open' },
    { route: { channel: 'web_chat', direction: 'inbound' }, purpose: 'media_processing', in_use: false, oq11_state: 'open' },
    { route: { channel: 'web_chat', direction: 'inbound' }, purpose: 'transfer', in_use: false, oq11_state: 'open' },
    { route: { channel: 'web_chat', direction: 'inbound' }, purpose: 'marketing_contact', in_use: false, oq11_state: 'open' }
  ],
  operational_level: { level: 'best_effort' },
  measurement_dictionary: [{
    id: 'csat', name: 'CSAT после закрытия', business_meaning: 'Удовлетворённость клиента обслуживанием',
    numerator: 'Положительные оценки', denominator: 'Все оценки с ответом', source_of_truth: 'Опрос после закрытия case',
    terminal_timing: 'Через 24 часа после закрытия', slices: ['journey', 'modality', 'conversation_mode', 'release_version', 'autonomy_class'],
    owner_id: '01994f64-6c00-7000-8000-000000000001', dispute_resolution: 'Разногласия решает владелец метрики',
    route_failure_visibility: 'Доля route-blocking failure показывается отдельной строкой, не средним'
  }],
  gate_declarations: [{
    gate_id: 'knowledge_eval', evidence_owner_id: '01994f64-6c00-7000-8000-000000000001',
    decision_deadline: 'До старта пилота', fail_closed_fallback: 'block_route', allow_role_coincidence: false
  }],
  coverage_matrix: [
    { journey: 'поиск детали', route: { channel: 'phone', direction: 'inbound' }, modality: 'voice', conversation_mode: 'sales', autonomy_class: 'assisted', eval_ref: 'eval://auto-parts/search/voice-v1', gate_refs: ['knowledge_eval'], human_fallback: 'human_callback по регламенту' },
    { journey: 'поиск детали', route: { channel: 'web_chat', direction: 'inbound' }, modality: 'text', conversation_mode: 'sales', autonomy_class: 'assisted', eval_ref: 'eval://auto-parts/search/text-v1', human_fallback: 'human_callback по регламенту' },
    { journey: 'уточнение аналога', route: { channel: 'phone', direction: 'inbound' }, modality: 'voice', conversation_mode: 'sales', autonomy_class: 'assisted', not_admitted: true, not_admitted_reason: 'Голосовой сценарий аналогов требует доработки pack.' },
    { journey: 'уточнение аналога', route: { channel: 'web_chat', direction: 'inbound' }, modality: 'text', conversation_mode: 'sales', autonomy_class: 'assisted', eval_ref: 'eval://auto-parts/analog/text-v1', human_fallback: 'human_callback по регламенту' }
  ],
  conversation_modes: ['sales'],
  autonomy_classes: ['assisted']
};

const tenantUUID = '01994f64-6c00-7000-8000-000000000002';
const employeeUUID = '01994f64-6c00-7000-8000-000000000004';

test('operating profile: draft→approval with step-up, thresholds, matrix and honest boundaries at 320px', async ({ browser }) => {
  test.setTimeout(120_000);
  const page = await browser.newPage({ viewport: { width: 320, height: 800 } });
  // The fixture step-up flag is global server state: the finally block
  // guarantees the reset even when the journey fails midway, so the later
  // journeys never inherit the enabled step-up (state pollution guard).
  try {
  // Create (or reuse) the fixture employee, then pin auto-parts so the
  // manifest's pack journeys are deterministic regardless of the earlier
  // journeys (the domain-pack journey may have switched the pack).
  await page.goto(`${employeePath}`);
  await page.getByRole('button', { name: 'Создать ИИ-сотрудника' }).click();
  await expect(page.getByRole('heading', { name: 'Domain Pack' })).toBeVisible();
  await page.getByLabel('Пакет и версия').selectOption('auto-parts@1.0.0');
  await page.getByRole('group', { name: 'Варианты Автозапчасти · 1.0.0' }).getByLabel('Интерес к деталям').selectOption('both');
  const confirmBox = page.getByRole('checkbox', { name: /подтверждаю смену пакета или версии/ });
  if (await confirmBox.isVisible()) {
    await confirmBox.check();
    await page.getByRole('button', { name: 'Сохранить подтверждённую версию' }).click();
  } else {
    await page.getByRole('button', { name: 'Закрепить пакет для настройки' }).click();
  }
  await expect(page.getByRole('status').getByText('Закреплён пакет auto-parts, версия 1.0.0')).toBeVisible();

  // Seal a fresh manifest through the JSON API: the revisions are read from
  // the view, so the flow works on any earlier journey state. The newest
  // manifest carries the auto-parts pack reference the profile matrix needs.
  const api = page.request;
  const releaseView = await (await api.get(`/v1/tenants/${tenantUUID}/employees/${employeeUUID}/release`)).json();
  const revision = releaseView.draft?.revision ?? 0;
  await api.post(`/v1/tenants/${tenantUUID}/employees/${employeeUUID}/release/draft`, { data: { expected_revision: revision } });
  const versionResponse = await api.post(`/v1/tenants/${tenantUUID}/employees/${employeeUUID}/release/versions`, { data: { expected_revision: revision + 1, description: 'Профильная конфигурация' } });
  const version = (await versionResponse.json()).version;
  const sealResponse = await api.post(`/v1/tenants/${tenantUUID}/employees/${employeeUUID}/release/manifests`, { data: { config_version: version, expected_revision: revision + 2 } });
  const manifest = await sealResponse.json();
  expect(manifest.digest).toMatch(/^[0-9a-f]{64}$/);

  // The profile page states its honest boundary: rules, not traffic admission.
  await page.goto(`${profilePath}`);
  await expect(page.getByRole('heading', { name: 'Operating Profile и правила измерения' })).toBeVisible();
  await expect(page.getByText('Профиль — не допуск трафика')).toBeVisible();
  await expect(page.getByText('Story 1.14')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Утверждённой версии нет' })).toBeVisible();

  // The manifest link fields are prefilled from the latest sealed row.
  const manifestInput = page.getByLabel('manifest_id запечатанного манифеста');
  await expect(manifestInput).toHaveValue(/^[0-9a-f-]{36}$/);
  await expect(page.getByLabel('digest манифеста (64 hex)')).toHaveValue(/^[0-9a-f]{64}$/);

  // A broken link is an honest refusal: the draft is not saved.
  await page.locator('#profile-content').fill(JSON.stringify(profileContent));
  await manifestInput.evaluate((el) => (el.value = '01994f64-6c00-7000-8000-000000000abc'));
  await page.getByRole('button', { name: 'Создать черновик профиля' }).click();
  await expect(page.locator('.alert')).toContainText('Манифест (manifest_id + config_version) не найден');

  // The valid link saves the draft; readiness is complete.
  await page.goto(`${profilePath}`);
  await page.locator('#profile-content').fill(JSON.stringify(profileContent));
  await page.getByRole('button', { name: 'Создать черновик профиля' }).click();
  await expect(page.locator('#page-notice')).toBeFocused();
  await expect(page.locator('#page-notice')).toContainText('Черновик профиля сохранён');
  await expect(page.getByRole('heading', { name: /Черновик профиля \(ревизия 1\)/ })).toBeVisible();
  await expect(page.getByText('Все обязательные разделы заполнены и валидны')).toBeVisible();

  // The route states and the matrix are derived with honest texts.
  const routeTable = page.locator('table', { hasText: 'Выводные состояния маршрутов' });
  await expect(routeTable.getByRole('row', { name: /phone:inbound/ })).toContainText('clear');
  const matrixTable = page.locator('table', { hasText: 'Полнота матрицы' });
  await expect(matrixTable.getByRole('row', { name: /уточнение аналога.*phone:inbound/ })).toContainText('не допущено');
  await expect(matrixTable.getByRole('columnheader', { name: 'Conversation Mode' })).toBeVisible();

  // Thresholds start honestly at pending baseline.
  const thresholdTable = page.locator('table', { hasText: 'Пороги: pending baseline' });
  await expect(thresholdTable.getByRole('row', { name: /knowledge_eval/ })).toContainText('pending baseline');
  await expect(thresholdTable.getByRole('row', { name: /knowledge_eval/ })).toContainText('совпадение ролей: не разрешено');

  // Approval without step-up: the honest refusal (aria-live alert).
  await page.getByLabel('Описание-объяснение утверждения (1–300)').fill('Утверждение профиля v1 по договору №77');
  await page.locator('#approval-evidence').fill('manifest-digest://v1\nthreshold://none');
  await page.getByRole('button', { name: 'Утвердить профиль (требует step-up)' }).click();
  await expect(page.locator('.alert')).toContainText('step-up подтверждения');

  // Enable step-up through the fixture endpoint and approve: the immutable
  // version with digest and approver roles appears. The refusal reloaded the
  // page, so the form fields are filled again before the submit — the honest
  // browser validation rejects an empty confirmation, not the server.
  await page.request.post('/fixture/step-up', { form: { enabled: '1' } });
  await page.getByLabel('Описание-объяснение утверждения (1–300)').fill('Утверждение профиля v1 по договору №77');
  await page.locator('#approval-evidence').fill('manifest-digest://v1\nthreshold://none');
  await page.getByRole('button', { name: 'Утвердить профиль (требует step-up)' }).click();
  await expect(page.locator('#page-notice')).toBeFocused();
  await expect(page.locator('#page-notice')).toContainText('Профиль утверждён');
  await expect(page.getByRole('heading', { name: 'Утверждённая версия 1 (неизменяемая)' })).toBeVisible();
  await expect(page.getByText('Состояние применимости').locator('..')).toContainText('применим');
  const digestBlock = page.locator('dd.digest-block').first();
  await expect(digestBlock).toContainText(/^[0-9a-f]{64}$/);
  await expect(page.getByText('роли: owner')).toBeVisible();

  // A post-approval edit surfaces as unapproved changes; the version stays.
  const contentEdit = { ...profileContent, conversation_modes: ['sales', 'service_status'] };
  await page.locator('#profile-content').fill(JSON.stringify(contentEdit));
  await page.getByLabel('Ожидаемая ревизия (CAS)').fill('2');
  await page.getByRole('button', { name: 'Сохранить черновик' }).click();
  await expect(page.locator('#page-notice')).toContainText('Черновик профиля сохранён');
  await expect(page.getByText('есть неутверждённые изменения', { exact: true })).toBeVisible();
  await expect(digestBlock).toContainText(/^[0-9a-f]{64}$/);

  // The matrix now honestly shows the cartesian gap of the new mode.
  await expect(matrixTable.getByRole('row', { name: /service_status/ }).first()).toContainText('gap');

  // A threshold decision by the declared evidence owner: pending → decided.
  await page.getByLabel('Идентификатор gate (из деклараций утверждённого профиля)').fill('knowledge_eval');
  await page.getByLabel('Значение порога (1–300; система значений не подставляет)').fill('0.9');
  await page.getByLabel('Решение/пояснение (1–300)').fill('Порог по контрольному baseline');
  await page.locator('#threshold-evidence').fill('baseline://eval-1');
  await page.getByRole('button', { name: 'Записать версию решения порога' }).click();
  await expect(page.locator('#page-notice')).toBeFocused();
  await expect(page.locator('#page-notice')).toContainText('Порог записан');
  await expect(thresholdTable.getByRole('row', { name: /knowledge_eval/ })).toContainText('порог утверждён');
  await expect(thresholdTable.getByRole('row', { name: /knowledge_eval/ })).toContainText('0.9');

  // The digest wraps inside its block: no horizontal page scroll at 320 px.
  const scrolled = await page.evaluate(() => document.documentElement.scrollWidth);
  expect(scrolled).toBeLessThanOrEqual(320);
  const box = await digestBlock.boundingBox();
  expect(box?.width).toBeLessThanOrEqual(320);

  // Keyboard access: the matrix table wrap is focusable and readable.
  await page.locator('div.datamap-table-wrap').first().focus();
  await expect(page.locator('caption').first()).toBeVisible();
  const scrolledAgain = await page.evaluate(() => document.documentElement.scrollWidth);
  expect(scrolledAgain).toBeLessThanOrEqual(320);


  } finally {
    // Reset the step-up flag for the other journeys (fixture hygiene): the
    // flag is global server state, an midway failure must not pollute the
    // later journeys.
    await page.request.post('/fixture/step-up', { form: { enabled: '0' } });
  }
});
