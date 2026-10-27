package catalog

import (
	"github.com/artemiosu/staffy/internal/connector/domain"
)

// definitions is the immutable binary catalog of published Connector definitions.
// Adding a provider means adding a definition (and an adapter) — never a Tenant branch.
// Older versions stay resolvable for pinned connections (Get pins an exact
// version); List only offers the newest version of every connector, so new
// Connections always pin the current contract.
var definitions = []domain.ConnectorDefinition{
	{
		// Spec 4-1 (Pilot Admission Registry): the SIP probe set of
		// AC-6 — five read-only query capabilities (transfer/bridge,
		// callback, DTMF, playback/delivery, failure states) over
		// the synthetic contour. The only live probe surface of the
		// pilot (AdapterImplemented true).
		ID:                  "staffy-synthetic-telephony",
		Version:             "1.2.0",
		Type:                domain.TypeSIP,
		Name:                "Staffy Synthetic Telephony",
		Purpose:             "Локальный синтетический SIP-контур для безопасной отладки подключений и проверки capabilities без реального провайдера: пять SIP query (AC-6) поверх прежних трёх.",
		SupportStatus:       domain.StatusSupported,
		AdapterImplemented:  true,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "synthetic",
		DocumentationURL:    "https://docs.bmad-method.org/",
		Fields: []domain.FieldSchema{
			{Name: "endpoint", Label: "Endpoint", Type: domain.FieldURL, Required: true, Description: "Финальный адрес вида https://<имя>.sandbox.example/api (для сценария таймаута: https://timeout.<имя>.sandbox.example/api)"},
			{Name: "trunk_label", Label: "Метка транка", Type: domain.FieldText, Required: false, Description: "Понятное имя тестового транка"},
			{Name: "api_key", Label: "API-ключ (sandbox)", Type: domain.FieldText, Required: true, Secret: true, Description: "Формат sandbox-… Значение принимается один раз и больше не показывается."},
		},
		Capabilities: []domain.CapabilityDeclaration{
			{
				ID: "query.trunk-status", Title: "Статус транка", Description: "Чтение текущего статуса тестового транка (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.transfer-bridge", Title: "Трансфер/bridge", Description: "Чтение состояния возможности перевода/соединения вызова (read-only, AC-6).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.callback-state", Title: "Состояние обратного звонка", Description: "Чтение состояния callback-сценария контура (read-only, AC-6).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.dtmf-support", Title: "Поддержка DTMF", Description: "Чтение поддерживаемых DTMF-сценариев контура (read-only, AC-6).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.playback-delivery", Title: "Доставка воспроизведения", Description: "Чтение состояния доставки воспроизведения медиа (read-only, AC-6).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.failure-states", Title: "Состояния отказов", Description: "Чтение воспроизводимых состояний отказов контура (read-only, AC-6).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "command.send-sms", Title: "Отправка SMS", Description: "Мутирующая операция отправки SMS; проверка заблокирована до Action Gateway.",
				Direction: domain.DirectionOutbound, Operation: domain.OperationCommand,
				TimeoutMS: 10000, Ordering: "fifo", RateLimit: "30 сообщений/мин", SizeLimit: "1 КБ полезной нагрузки",
				Idempotent: true, Receipt: "accepted → delivered → committed", ReceiptLevel: domain.DeliveryCommitted,
				CancelCompensation: "отмена недоступна после accepted; компенсация не поддерживается провайдером", Reconciliation: "query_reconcilable по GATE-REGISTRY: статус доставки сверяется повторным запросом", ReconciliationOwner: "Technical Lead",
				ProbeAllowed: false,
			},
			{
				ID: "event.inbound-call", Title: "Входящий звонок", Description: "Приём события входящего звонка; путь проверки не реализован в текущем срезе.",
				Direction: domain.DirectionInbound, Operation: domain.OperationEvent,
				TimeoutMS: 5000, Ordering: "fifo", RateLimit: "120 событий/мин", SizeLimit: "16 КБ события",
				Idempotent: false, Receipt: "event_ack", ReceiptLevel: domain.DeliveryAccepted,
				CancelCompensation: "не применимо (inbound)", Reconciliation: "дедупликация по event_id на стороне staffy", ReconciliationOwner: "Technical Lead",
				ProbeAllowed: false,
			},
		},
	},
	{
		ID:                  "staffy-synthetic-telephony",
		Version:             "1.1.0",
		Type:                domain.TypeSIP,
		Name:                "Staffy Synthetic Telephony",
		Purpose:             "Локальный синтетический SIP-контур для безопасной отладки подключений и проверки capabilities без реального провайдера.",
		SupportStatus:       domain.StatusSupported,
		AdapterImplemented:  true,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "synthetic",
		DocumentationURL:    "https://docs.bmad-method.org/",
		Fields: []domain.FieldSchema{
			{Name: "endpoint", Label: "Endpoint", Type: domain.FieldURL, Required: true, Description: "Финальный адрес вида https://<имя>.sandbox.example/api (для сценария таймаута: https://timeout.<имя>.sandbox.example/api)"},
			{Name: "trunk_label", Label: "Метка транка", Type: domain.FieldText, Required: false, Description: "Понятное имя тестового транка"},
			{Name: "api_key", Label: "API-ключ (sandbox)", Type: domain.FieldText, Required: true, Secret: true, Description: "Формат sandbox-… Значение принимается один раз и больше не показывается."},
		},
		Capabilities: []domain.CapabilityDeclaration{
			{
				ID: "query.trunk-status", Title: "Статус транка", Description: "Чтение текущего статуса тестового транка (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "command.send-sms", Title: "Отправка SMS", Description: "Мутирующая операция отправки SMS; проверка заблокирована до Action Gateway.",
				Direction: domain.DirectionOutbound, Operation: domain.OperationCommand,
				TimeoutMS: 10000, Ordering: "fifo", RateLimit: "30 сообщений/мин", SizeLimit: "1 КБ полезной нагрузки",
				Idempotent: true, Receipt: "accepted → delivered → committed", ReceiptLevel: domain.DeliveryCommitted,
				CancelCompensation: "отмена недоступна после accepted; компенсация не поддерживается провайдером", Reconciliation: "query_reconcilable по GATE-REGISTRY: статус доставки сверяется повторным запросом", ReconciliationOwner: "Technical Lead",
				ProbeAllowed: false,
			},
			{
				ID: "event.inbound-call", Title: "Входящий звонок", Description: "Приём события входящего звонка; путь проверки не реализован в текущем срезе.",
				Direction: domain.DirectionInbound, Operation: domain.OperationEvent,
				TimeoutMS: 5000, Ordering: "fifo", RateLimit: "120 событий/мин", SizeLimit: "16 КБ события",
				Idempotent: false, Receipt: "event_ack", ReceiptLevel: domain.DeliveryAccepted,
				CancelCompensation: "не применимо (inbound)", Reconciliation: "дедупликация по event_id на стороне staffy", ReconciliationOwner: "Technical Lead",
				ProbeAllowed: false,
			},
		},
	},
	{
		ID:                  "staffy-synthetic-telephony",
		Version:             "1.0.0",
		Type:                domain.TypeSIP,
		Name:                "Staffy Synthetic Telephony",
		Purpose:             "Локальный синтетический SIP-контур для безопасной отладки подключений без реального провайдера.",
		SupportStatus:       domain.StatusSupported,
		AdapterImplemented:  true,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "synthetic",
		DocumentationURL:    "https://docs.bmad-method.org/",
		Fields: []domain.FieldSchema{
			{Name: "endpoint", Label: "Endpoint", Type: domain.FieldURL, Required: true, Description: "Финальный адрес вида https://<имя>.sandbox.example/api"},
			{Name: "trunk_label", Label: "Метка транка", Type: domain.FieldText, Required: false, Description: "Понятное имя тестового транка"},
			{Name: "api_key", Label: "API-ключ (sandbox)", Type: domain.FieldText, Required: true, Secret: true, Description: "Формат sandbox-… Значение принимается один раз и больше не показывается."},
		},
	},
	{
		// Spec 5-1 (Montagio Admission Registry): the twelve AC-3
		// capabilities of the Novofon account-scoped probe set, declared
		// by name with the full operational semantics — the general
		// documentation of the provider is never a proof of the concrete
		// tariff and account, so the adapter stays honestly not
		// implemented (AG-4) and only the read-only queries may ever be
		// probed.
		ID:                  "novofon-sip",
		Version:             "1.1.0",
		Type:                domain.TypeSIP,
		Name:                "Novofon",
		Purpose:             "Телефония для пилота Montagio: двенадцать declared capabilities (AC-3). Адаптер не реализован — возможности конкретного тарифа и аккаунта не проверяемы (AG-4); общая документация Novofon не считается доказательством.",
		SupportStatus:       domain.StatusCandidate,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "novofon_business",
		DocumentationURL:    "https://novofon.com/",
		// PriorityNote is only rendered when AdapterImplemented is true.
		PriorityNote: "Приоритетный кандидат телефонии Montagio",
		Fields:       []domain.FieldSchema{},
		Capabilities: []domain.CapabilityDeclaration{
			{
				ID: "query.inbound-calls", Title: "Входящие вызовы", Description: "Чтение входящих вызовов аккаунта (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "64 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Voice/Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.outbound-calls", Title: "Исходящие вызовы", Description: "Чтение исходящих вызовов аккаунта (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "64 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Voice/Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.media-codec", Title: "Кодеки и медиа", Description: "Чтение поддерживаемых кодеков и медиа-параметров маршрута (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Voice/Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.streaming", Title: "Потоковая передача", Description: "Чтение возможностей потоковой передачи медиа (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Voice/Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.dtmf-support", Title: "Поддержка DTMF", Description: "Чтение поддерживаемых DTMF-сценариев маршрута (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Voice/Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.transfer-bridge", Title: "Трансфер/bridge", Description: "Чтение состояния возможности перевода/соединения вызова (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Voice/Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.callback-state", Title: "Состояние callback", Description: "Чтение состояния сценария обратного звонка (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Voice/Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.caller-id", Title: "Caller ID", Description: "Чтение настроек идентификации вызывающей стороны аккаунта (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Voice/Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.recording", Title: "Запись разговоров", Description: "Чтение правил и состояния записи разговоров (read-only); purpose-specific consent — гейт-требование AC-4, не заявляется этой декларацией.",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Voice/Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.webhooks", Title: "Webhook-конечные точки", Description: "Чтение зарегистрированных webhook-конечных точек аккаунта (read-only); ингресс — будущий срез.",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Voice/Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.delivery-states", Title: "Состояния доставки", Description: "Чтение состояний доставки уведомлений и сообщений аккаунта (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Voice/Technical Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.failure-states", Title: "Состояния отказов", Description: "Чтение воспроизводимых состояний отказов маршрута (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Voice/Technical Lead",
				ProbeAllowed: true,
			},
		},
	},
	{
		ID:                  "novofon-sip",
		Version:             "1.0.0",
		Type:                domain.TypeSIP,
		Name:                "Novofon",
		Purpose:             "Телефония для пилота Montagio. Адаптер ещё не реализован.",
		SupportStatus:       domain.StatusCandidate,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "novofon_business",
		DocumentationURL:    "https://novofon.com/",
		// PriorityNote is only rendered when AdapterImplemented is true.
		PriorityNote: "Приоритетный кандидат телефонии Montagio",
		Fields:       []domain.FieldSchema{},
	},
	{
		ID:                  "mango-office-sip",
		Version:             "1.0.0",
		Type:                domain.TypeSIP,
		Name:                "Mango Office",
		Purpose:             "Телефония для магазина Олега. Кандидат до проверки фактического аккаунта.",
		SupportStatus:       domain.StatusCandidate,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "mango_office",
		DocumentationURL:    "https://www.mango-office.ru/",
		Fields:              []domain.FieldSchema{},
	},
	{
		// Spec 5-1 (Montagio Admission Registry): the nine AC-5
		// capabilities of the Montagio Django site surface — contract
		// schema, authentication, inbound submissions, the outbound
		// reply (mutating, blocked until the Action Gateway — AG-4),
		// correlation state, media support, delivery, rate limits and
		// failure states. Retries and idempotency live in the
		// declarative semantic fields, never as separate capabilities.
		ID:                  "montagio-web",
		Version:             "1.0.0",
		Type:                domain.TypeChat,
		Name:                "Сайт Montagio (Django)",
		Purpose:             "Поверхность сайта Montagio для пилота услуг: девять declared capabilities (AC-5). Адаптер не реализован (AG-4); неполная заявка не превращается автоматически в готовый к продаже Order — машины заказа строятся будущими stories.",
		SupportStatus:       domain.StatusCandidate,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "montagio_site",
		DocumentationURL:    "https://montagio.example/",
		Fields:              []domain.FieldSchema{},
		Capabilities: []domain.CapabilityDeclaration{
			{
				ID: "query.contract-schema", Title: "Схема контракта", Description: "Чтение схемы контракта форм/чата/callback сайта (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "32 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.authentication", Title: "Аутентификация поверхности", Description: "Чтение методов аутентификации поверхности сайта (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "event.inbound-submission", Title: "Входящая заявка", Description: "Событие входящей заявки/сообщения с сайта; путь приёма не реализован в текущем срезе (AG-4).",
				Direction: domain.DirectionInbound, Operation: domain.OperationEvent,
				TimeoutMS: 5000, Ordering: "fifo", RateLimit: "300 событий/мин", SizeLimit: "64 КБ события (вложения — ссылками)",
				Idempotent: true, Receipt: "event_ack", ReceiptLevel: domain.DeliveryAccepted,
				CancelCompensation: "не применимо (inbound)", Reconciliation: "дедупликация по submission_id на стороне Staffy", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: false,
			},
			{
				ID: "command.outbound-reply", Title: "Исходящий ответ", Description: "Мутирующая отправка ответа/статуса заявки на сайт; проверка заблокирована до Action Gateway (AG-4).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationCommand,
				TimeoutMS: 10000, Ordering: "fifo", RateLimit: "120 команд/мин", SizeLimit: "16 КБ payload",
				Idempotent: true, Receipt: "accepted → delivered", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "отмена недоступна после accepted; правка до delivered — отдельная команда", Reconciliation: "query.delivery-status сверяет состояние доставки повторным чтением", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: false,
			},
			{
				ID: "query.correlation-state", Title: "Состояние корреляции", Description: "Чтение состояния корреляции заявки с Case (read-only); связывание заявки с тем же Case — контур нормализации story 5-2.",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.media-upload-support", Title: "Поддержка медиа", Description: "Чтение поддерживаемых типов и размеров медиа-загрузок поверхности (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.delivery-status", Title: "Статус доставки", Description: "Чтение статуса доставки исходящего ответа/статуса (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.rate-limits", Title: "Лимиты запросов", Description: "Чтение фактических лимитов запросов поверхности сайта (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.failure-states", Title: "Состояния отказов", Description: "Чтение воспроизводимых состояний отказов поверхности (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
		},
	},
	{
		// Spec 5-1 (Montagio Admission Registry): the five AC-6
		// capabilities of the partner intake — the partner lead event,
		// the contract version, consent evidence, delivery and failure
		// states. The incomplete partner lead never turns into a
		// ready-to-sell Order: the order machines are future stories.
		ID:                  "partner-intake",
		Version:             "1.0.0",
		Type:                domain.TypeBusiness,
		Name:                "Партнёрский intake",
		Purpose:             "Приём заявок партнёрских магазинов пилота Montagio: пять declared capabilities (AC-6). Адаптер не реализован (AG-4); неполная партнёрская заявка не превращается автоматически в готовый к продаже Order.",
		SupportStatus:       domain.StatusCandidate,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "partner_shop",
		DocumentationURL:    "https://partner.montagio.example/",
		Fields:              []domain.FieldSchema{},
		Capabilities: []domain.CapabilityDeclaration{
			{
				ID: "event.inbound-partner-lead", Title: "Партнёрская заявка", Description: "Событие входящей партнёрской заявки; путь приёма не реализован в текущем срезе (AG-4).",
				Direction: domain.DirectionInbound, Operation: domain.OperationEvent,
				TimeoutMS: 5000, Ordering: "fifo", RateLimit: "120 событий/мин", SizeLimit: "64 КБ события",
				Idempotent: true, Receipt: "event_ack", ReceiptLevel: domain.DeliveryAccepted,
				CancelCompensation: "не применимо (inbound)", Reconciliation: "дедупликация по partner_lead_id на стороне Staffy", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: false,
			},
			{
				ID: "query.lead-contract-version", Title: "Версия контракта заявки", Description: "Чтение версии контракта партнёрской заявки (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.consent-evidence", Title: "Доказательства согласия", Description: "Чтение доказательств согласия партнёрского источника (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.delivery-status", Title: "Статус доставки", Description: "Чтение статуса доставки квитанций партнёрского контура (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.failure-states", Title: "Состояния отказов", Description: "Чтение воспроизводимых состояний отказов партнёрского контура (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
		},
	},
	{
		// Spec 5-1 (Montagio Admission Registry): the five AC-7
		// capabilities of the B2B intake — the organization request
		// event, the organization fields, authority evidence, delivery
		// and failure states. The absent budget/payer authority stays
		// unknown — never implied.
		ID:                  "b2b-intake",
		Version:             "1.0.0",
		Type:                domain.TypeBusiness,
		Name:                "B2B-вход",
		Purpose:             "Приём B2B-заявок организаций пилота Montagio: пять declared capabilities (AC-7). Адаптер не реализован (AG-4); отсутствие полномочий согласующего бюджет или плательщика сохраняется как unknown, а не подразумевается.",
		SupportStatus:       domain.StatusCandidate,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "b2b_organization",
		DocumentationURL:    "https://b2b.montagio.example/",
		Fields:              []domain.FieldSchema{},
		Capabilities: []domain.CapabilityDeclaration{
			{
				ID: "event.inbound-b2b-request", Title: "B2B-заявка", Description: "Событие входящей B2B-заявки организации; путь приёма не реализован в текущем срезе (AG-4).",
				Direction: domain.DirectionInbound, Operation: domain.OperationEvent,
				TimeoutMS: 5000, Ordering: "fifo", RateLimit: "120 событий/мин", SizeLimit: "64 КБ события",
				Idempotent: true, Receipt: "event_ack", ReceiptLevel: domain.DeliveryAccepted,
				CancelCompensation: "не применимо (inbound)", Reconciliation: "дедупликация по b2b_request_id на стороне Staffy", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: false,
			},
			{
				ID: "query.organization-fields", Title: "Поля организации", Description: "Чтение полей организации заявителя (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "16 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.authority-evidence", Title: "Сведения о полномочиях", Description: "Чтение доступных сведений о полномочиях заявителя (read-only); отсутствие полномочий — unknown, не подразумевается.",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.delivery-status", Title: "Статус доставки", Description: "Чтение статуса доставки квитанций B2B-контура (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.failure-states", Title: "Состояния отказов", Description: "Чтение воспроизводимых состояний отказов B2B-контура (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
		},
	},
	{
		// Spec 4-1 (Pilot Admission Registry): the eight Jivo
		// capabilities of AC-3, declared with the full operational
		// semantics — a product page or a documentation claim is
		// never a proof, so the adapter stays honestly not
		// implemented and the card is not connectable.
		ID:                  "jivo-chat",
		Version:             "1.1.0",
		Type:                domain.TypeChat,
		Name:                "Jivo",
		Purpose:             "Чат-канал Jivo для пилота магазина Олега: восемь declared capabilities (AC-3). Адаптер не реализован — capabilities не проверяемы (AG-4).",
		SupportStatus:       domain.StatusCandidate,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "jivo_site",
		DocumentationURL:    "https://www.jivo.ru/",
		Fields:              []domain.FieldSchema{},
		Capabilities: []domain.CapabilityDeclaration{
			{
				ID: "event.inbound-message", Title: "Входящее сообщение", Description: "Событие входящего сообщения клиента; путь приёма не реализован в текущем срезе (AG-4).",
				Direction: domain.DirectionInbound, Operation: domain.OperationEvent,
				TimeoutMS: 5000, Ordering: "fifo", RateLimit: "300 событий/мин", SizeLimit: "64 КБ сообщения (вложения — ссылками)",
				Idempotent: true, Receipt: "event_ack", ReceiptLevel: domain.DeliveryAccepted,
				CancelCompensation: "не применимо (inbound)", Reconciliation: "дедупликация по event_id на стороне Staffy", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: false,
			},
			{
				ID: "command.outbound-reply", Title: "Исходящий ответ", Description: "Мутирующая отправка ответа клиенту; проверка заблокирована до Action Gateway (AG-4).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationCommand,
				TimeoutMS: 10000, Ordering: "fifo", RateLimit: "120 сообщений/мин", SizeLimit: "16 КБ текста",
				Idempotent: true, Receipt: "accepted → delivered", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "отмена недоступна после accepted; правка до delivered — отдельная команда", Reconciliation: "query.delivery-status сверяет состояние доставки повторным чтением", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: false,
			},
			{
				ID: "query.delivery-status", Title: "Статус доставки", Description: "Чтение статуса доставки исходящего сообщения (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "command.assign-to-human", Title: "Назначение человеку", Description: "Мутирующая передача диалога человеку-оператору; проверка заблокирована до Action Gateway (AG-4).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationCommand,
				TimeoutMS: 5000, Ordering: "fifo", RateLimit: "60 команд/мин", SizeLimit: "2 КБ payload",
				Idempotent: true, Receipt: "accepted → delivered", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "повторное назначение другого оператора — новая команда", Reconciliation: "query.one-writer-state сверяет текущего владельца диалога", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: false,
			},
			{
				ID: "event.acknowledgement", Title: "Квитанция прочтения", Description: "Событие подтверждения (acknowledgement) канала; путь приёма не реализован (AG-4).",
				Direction: domain.DirectionInbound, Operation: domain.OperationEvent,
				TimeoutMS: 5000, Ordering: "fifo", RateLimit: "300 событий/мин", SizeLimit: "2 КБ события",
				Idempotent: true, Receipt: "event_ack", ReceiptLevel: domain.DeliveryAccepted,
				CancelCompensation: "не применимо (inbound)", Reconciliation: "дедупликация по ack_id", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: false,
			},
			{
				ID: "query.one-writer-state", Title: "Состояние единственного писателя", Description: "Чтение текущего владельца диалога — одновременная запись исключена (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "2 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.media-upload-support", Title: "Поддержка вложений", Description: "Чтение поддерживаемых типов и размеров вложений канала (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "8 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.fallback-state", Title: "Состояние fallback", Description: "Чтение состояния резервного пути канала (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "2 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
		},
	},
	{
		ID:                  "jivo-chat",
		Version:             "1.0.0",
		Type:                domain.TypeChat,
		Name:                "Jivo",
		Purpose:             "Чат-канал. Реализованные типы подключения отсутствуют.",
		SupportStatus:       domain.StatusDeferred,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "jivo_site",
		DocumentationURL:    "https://www.jivo.ru/",
		Fields:              []domain.FieldSchema{},
	},
	{
		// Spec 4-1 (Pilot Admission Registry): the six read-only ABCP
		// query capabilities of AC-7 plus the one mutating
		// prepare-client-order of AC-8 (ProbeAllowed false — the
		// Action Gateway/AG-4 fence; no supplier send exists in the
		// pilot). The manager-only interface actions are enumerated
		// separately from the API capabilities (AC-7 tail).
		ID:                  "abcp-parts",
		Version:             "1.1.0",
		Type:                domain.TypeBusiness,
		Name:                "ABCP / Laximo",
		Purpose:             "Бизнес-система автозапчастей для пилота магазина Олега: шесть read-only query capabilities (AC-7) и подготовка клиентского заказа (AC-8, без отправки поставщику). Адаптер не реализован (AG-4).",
		SupportStatus:       domain.StatusCandidate,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "abcp_partner",
		DocumentationURL:    "https://www.abcp.ru/",
		Fields:              []domain.FieldSchema{},
		ManagerOnlyActions:  []string{"поиск/заказ через интерфейс менеджера ABCP — не API capability"},
		Capabilities: []domain.CapabilityDeclaration{
			{
				ID: "query.identify-vehicle", Title: "Идентификация автомобиля", Description: "Идентификация автомобиля по VIN/FRAME с возвратом характеристик (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 15000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "16 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.catalog-navigation", Title: "Навигация каталога", Description: "Обход дерева каталога запчастей от узла к узлу (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 15000, Ordering: "unordered", RateLimit: "120 запросов/мин", SizeLimit: "64 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.fitment-applicability", Title: "Применимость детали", Description: "Проверка применимости детали к идентифицированному автомобилю/узлу (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 15000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "32 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.oem-cross", Title: "OEM и кросс-номера", Description: "Поиск OEM-номеров и кросс-замен по позиции каталога (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 15000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "32 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.supplier-offers", Title: "Предложения поставщиков", Description: "Предложения поставщиков по позиции каталога (read-only агрегация).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 30000, Ordering: "unordered", RateLimit: "30 запросов/мин", SizeLimit: "128 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "query.price-availability-term", Title: "Цена, наличие, срок", Description: "Цена, наличие и срок поставки по предложению (read-only).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
				TimeoutMS: 15000, Ordering: "unordered", RateLimit: "120 запросов/мин", SizeLimit: "32 КБ ответа",
				Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Integration Lead",
				ProbeAllowed: true,
			},
			{
				ID: "command.prepare-client-order", Title: "Подготовка клиентского заказа", Description: "Мутирующая подготовка клиентского заказа в ABCP без отправки поставщику; проверка заблокирована до Action Gateway (AG-4).",
				Direction: domain.DirectionOutbound, Operation: domain.OperationCommand,
				TimeoutMS: 30000, Ordering: "fifo", RateLimit: "10 команд/мин", SizeLimit: "32 КБ payload",
				Idempotent: true, Receipt: "accepted → delivered (prepared order id)", ReceiptLevel: domain.DeliveryDelivered,
				CancelCompensation:  "prepared order не отправляется поставщику автоматически; отмена подготовки — отдельная команда до отправки",
				Reconciliation:      "query_reconcilable по GATE-REGISTRY OQ-3: reread статуса prepared order; fence — явное одобрение менеджера",
				ReconciliationOwner: "Олег + ABCP integrator",
				ProbeAllowed:        false,
			},
		},
	},
	{
		ID:                  "abcp-parts",
		Version:             "1.0.0",
		Type:                domain.TypeBusiness,
		Name:                "ABCP / Laximo",
		Purpose:             "Бизнес-система автозапчастей для пилота магазина Олега. Реализованные типы подключения отсутствуют.",
		SupportStatus:       domain.StatusDeferred,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "abcp_partner",
		DocumentationURL:    "https://www.abcp.ru/",
		Fields:              []domain.FieldSchema{},
	},
	{
		ID:                  "yandex-speechkit",
		Version:             "1.0.0",
		Type:                domain.TypeAISpeech,
		Name:                "Yandex SpeechKit",
		Purpose:             "Речевые сервисы. Разрешённые типы подключения не реализованы.",
		SupportStatus:       domain.StatusDeferred,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "yandex_cloud",
		DocumentationURL:    "https://yandex.cloud/ru/services/speechkit",
		Fields:              []domain.FieldSchema{},
	},
	{
		ID:                  "exolve-sip",
		Version:             "1.0.0",
		Type:                domain.TypeSIP,
		Name:                "Exolve",
		Purpose:             "Определение опубликовано, но подключение пока не предлагается.",
		SupportStatus:       domain.StatusUnavailable,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox},
		RequiredAccountType: "exolve_account",
		DocumentationURL:    "https://exolve.ru/",
		Fields:              []domain.FieldSchema{},
	},
}

// List returns a defensive copy of the newest published version of every
// connector. Pinned older versions stay resolvable through Get only, so the
// catalog cards always offer the current contract to new Connections.
func List() []domain.ConnectorDefinition {
	out := make([]domain.ConnectorDefinition, 0, len(definitions))
	seen := map[string]bool{}
	for _, d := range definitions {
		if seen[d.ID] {
			continue
		}
		seen[d.ID] = true
		out = append(out, clone(d))
	}
	return out
}

// Get pins one definition by id and version. Older versions remain immutable
// and resolvable for existing connections; they are never offered as new.
func Get(id, version string) (domain.ConnectorDefinition, bool) {
	for _, d := range definitions {
		if d.ID == id && d.Version == version {
			return clone(d), true
		}
	}
	return domain.ConnectorDefinition{}, false
}

// Latest returns the newest published version of a connector and reports
// whether the requested version is that current version. New Connections pin
// the latest contract; older pins are refused with an honest reason.
func Latest(id, requestedVersion string) (domain.ConnectorDefinition, bool, bool) {
	var newest domain.ConnectorDefinition
	found := false
	for _, d := range definitions {
		if d.ID != id {
			continue
		}
		if !found || compareVersions(d.Version, newest.Version) > 0 {
			newest = d
			found = true
		}
	}
	if !found {
		return domain.ConnectorDefinition{}, false, false
	}
	return clone(newest), requestedVersion == newest.Version, true
}

// compareVersions orders simple dotted semver-ish strings ("1.10.0" > "1.9.0").
func compareVersions(a, b string) int {
	partsA, partsB := splitVersion(a), splitVersion(b)
	for i := 0; i < 3; i++ {
		if partsA[i] != partsB[i] {
			if partsA[i] > partsB[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func splitVersion(v string) [3]int {
	var out [3]int
	current, index := 0, 0
	for _, r := range v {
		if r == '.' {
			if index < 2 {
				out[index] = current
				index++
				current = 0
			}
			continue
		}
		if r >= '0' && r <= '9' {
			current = current*10 + int(r-'0')
		}
	}
	out[index] = current
	return out
}

func clone(d domain.ConnectorDefinition) domain.ConnectorDefinition {
	d.AllowedEnvironments = append([]domain.Environment(nil), d.AllowedEnvironments...)
	d.Fields = append([]domain.FieldSchema(nil), d.Fields...)
	d.Capabilities = append([]domain.CapabilityDeclaration(nil), d.Capabilities...)
	return d
}

// GroupByType preserves the fixed PRD type order.
func GroupByType(items []domain.ConnectorDefinition) map[domain.ConnectorType][]domain.ConnectorDefinition {
	order := []domain.ConnectorType{domain.TypeSIP, domain.TypeChat, domain.TypeBusiness, domain.TypeAISpeech, domain.TypeStorage}
	grouped := map[domain.ConnectorType][]domain.ConnectorDefinition{}
	for _, t := range order {
		grouped[t] = []domain.ConnectorDefinition{}
	}
	for _, item := range items {
		grouped[item.Type] = append(grouped[item.Type], item)
	}
	return grouped
}
