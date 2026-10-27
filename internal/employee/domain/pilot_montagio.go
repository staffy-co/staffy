package domain

import (
	"fmt"
)

// ---------------------------------------------------------------------------
// Montagio Admission Registry (spec 5-1, Epic 5)
//
// The compile-time dictionaries of the Montagio services pilot: the fixed
// route vocabulary {novofon_voice, montagio_web, partner_intake, b2b_intake,
// protected_web_continuation, human_inbox, avito_candidate,
// public_commercial}, the approved truthful AC-11 wording dictionary and the
// READ-TIME admission derivation {candidate → verified → controlled → live}
// over live facts. Nothing here writes: admission is a derivation, not a
// record — a repeated read of an unchanged world returns the byte-stable
// same answer. The dictionaries are fixed in the domain (never
// tenant-configurable); extending them is a new spec version.
//
// The four-stage ladder differs from the three-stage ladder of the Oleg
// pilot (spec 4-1, pilot.go) deliberately: this slice distinguishes
// controlled_real_customer from live (GATE-REGISTRY Traffic stages, Execution
// order «Before controlled_real_customer … for Montagio also OQ-7A, OQ-8»
// and «After baseline, before autonomy/live/scale promotion: OQ-7B and
// OQ-14»). The two dictionaries coexist; the Oleg ladder is not touched.
// ---------------------------------------------------------------------------

// Admission stages of a Montagio route (AC-13): candidate (declared, the
// required capabilities are not proven by live evidence — or the route is
// honestly capped by this slice), verified (every required capability is
// proven, the controlled gates are not closed), controlled (verified and
// every controlled-stage gate carries an acting PASS — the
// controlled_real_customer boundary) and live (controlled and the live
// gates {OQ-7B, OQ-14} carry acting PASS). The stages are fixed words.
const (
	MontagioStageCandidate  = "candidate"
	MontagioStageVerified   = "verified"
	MontagioStageControlled = "controlled"
	MontagioStageLive       = "live"
)

// The required Domain Pack pin of every Montagio route (AC-1): the
// versioned «Услуги» pack of the shared core. A fork, a tenant module or a
// hidden Montagio logic of the core is structurally impossible
// (AD-1/AD-2/CI); the pilot rules are tenant-scoped configuration.
const (
	MontagioRequiredPackID         = "montagio-services"
	MontagioRequiredPackMinVersion = "1.0.0"
)

// MontagioPackMissingWord is the pinned honest word of the missing pack
// pin: the whole Montagio admission is fail-closed until the sealed
// manifest pins the required pack (row 1 of the spec's edge-case matrix).
const MontagioPackMissingWord = "Domain Pack «Услуги» не прикреплён к запечатанному контуру"

// montagioStageWords are the pinned honest stage words (AC-13/AC-15: the
// stage reads as text, never as color or position alone; the four stages
// are distinguishable in words and semantically).
var montagioStageWords = map[string]string{
	MontagioStageCandidate:  "маршрут заявлен; требуемые capabilities живыми доказательствами не подтверждены либо маршрут честно ограничен этим срезом",
	MontagioStageVerified:   "все требуемые capabilities подтверждены живой evidence; controlled-гейты допуска не закрыты",
	MontagioStageControlled: "capabilities подтверждены и controlled-гейты {OQ-7A, OQ-8, OQ-11 (канальные скоупы), AG-1, AG-2, AG-4 [+ голос: OQ-12, OQ-13B]} имеют действующие PASS — controlled_real_customer; live-пороги не закрыты",
	MontagioStageLive:       "controlled + live-гейты {OQ-7B, OQ-14} имеют действующие PASS — baseline-пороги live утверждены",
}

// MontagioWordingWords are the five approved truthful AC-11 wordings: the
// only public phrasings of the Montagio role and its specialists this
// slice carries — a compile-time dictionary, never a generation.
var MontagioWordingWords = []string{
	"Montagio — договорная сторона; специалист выполняет работу по поручению Montagio",
	"внутренняя партнёрская операционная схема не раскрывается, если это не требуется законом или договором",
	"ложные заявления о штатном статусе, квалификации, гарантиях или полномочиях специалиста запрещены",
	"best-effort без SLA — пилотная доступность без договорных обещаний непрерывности (OQ-13A)",
	"отсутствие полномочий согласующего бюджет или плательщика сохраняется как unknown, а не подразумевается",
}

// montagioAutonomyWord is the pinned honest Autonomy Classes column word of
// every route row: the classes are set by the versioned Operating Profile —
// the route matrix never claims autonomy above the admission stage.
const montagioAutonomyWord = "классы автономности задаёт версионированный Operating Profile — маршрутная матрица автономность выше стадии допуска НЕ заявляет"

// montagioHumanInboxWord is the pinned Human Inbox column word of every
// route row (AC-10): one confirmed human surface with the proven
// assignment/acknowledgement/owner epoch/deadline/backup owner/return
// machinery of the existing contours.
const montagioHumanInboxWord = "единый подтверждённый Human Inbox: assignment, acknowledgement, owner epoch, deadline, backup owner и возврат управления доказаны (контуры 2-4/2-13)"

// ---------------------------------------------------------------------------
// The route dictionary (eight fixed words)
// ---------------------------------------------------------------------------

// MontagioRoute is one route word of the fixed Montagio dictionary: the
// Route Matrix column words (modality, intents, autonomy, human inbox,
// fallback), the catalog capabilities it must prove, the controlled-stage
// and live-stage gate conjunctions, the route-scoped OQ-11 channel scopes
// and the pinned cap/fence/fallback words of the surface.
type MontagioRoute struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Route Matrix column words (AC-13): modality, intents, autonomy
	// (the shared honest word), human inbox (the shared honest word) and
	// the fallback route.
	ModalityWord      string `json:"modality_word"`
	IntentsWord       string `json:"intents_word"`
	AutonomyWord      string `json:"autonomy_word"`
	HumanInboxWord    string `json:"human_inbox_word"`
	FallbackRouteWord string `json:"fallback_route_word"`
	// RequiredCapabilities are the catalog capability IDs the route must
	// prove (empty means the route needs no external connector proof — the
	// honest vacuous case of a Staffy-own surface).
	RequiredCapabilities []PilotCapabilityRequirement `json:"required_capabilities,omitempty"`
	// RequiredGates are the dictionary gate IDs whose acting PASS the
	// controlled stage requires (conjunction — a disjunction does not
	// exist in the registry model).
	RequiredGates []string `json:"required_gates,omitempty"`
	// RequiredLiveGates are the dictionary gate IDs whose acting PASS the
	// live stage additionally requires (every route except the eternal
	// avito_candidate carries {OQ-7B, OQ-14}).
	RequiredLiveGates []string `json:"required_live_gates,omitempty"`
	// RequiredOQ11Routes are the exact channel routes whose route-scoped
	// OQ-11 PASS the route consumes. An OQ-11 requirement with no channel
	// mapping is the honest scope boundary: no applicable instance can
	// exist this slice (fail-closed, never a silent pass).
	RequiredOQ11Routes []Route `json:"required_oq11_routes,omitempty"`
	// CapWord is the pinned honest cap word of the routes this slice never
	// admits above candidate (avito_candidate — eternal; public_commercial —
	// capped until the Order/payment machines exist).
	CapWord string `json:"cap_word,omitempty"`
	// CapNextOwner is the honest next owner of the cap check.
	CapNextOwner string `json:"cap_next_owner,omitempty"`
	// FenceWords are the pinned boundary-fence words (AC-4/AC-6/AC-7/AC-12).
	FenceWords []string `json:"fence_words,omitempty"`
	// FallbackWords are the pinned human-surface words (AC-10/FR-83).
	FallbackWords []string `json:"fallback_words,omitempty"`
}

// montagioRoutes is the fixed order of the eight route words (the Route
// Matrix rows of the Montagio pilot sources and channels).
var montagioRoutes = []MontagioRoute{
	{
		ID:                "novofon_voice",
		Title:             "голос: Novofon SIP-линия",
		ModalityWord:      "голос (phone: in/out)",
		IntentsWord:       "входящие голосовые обращения: консультация, квалификация услуги, safety triage, подготовка предложения — без публичной цены (OQ-9)",
		AutonomyWord:      montagioAutonomyWord,
		HumanInboxWord:    montagioHumanInboxWord,
		FallbackRouteWord: "human_inbox — безусловная передача человека (2-13)",
		RequiredCapabilities: []PilotCapabilityRequirement{
			{ID: "query.inbound-calls", ConnectorID: "novofon-sip", UnprovenWord: "Novofon-адаптер не реализован (AG-4): возможности конкретного тарифа и аккаунта не проверяемы; общая документация Novofon не считается доказательством"},
			{ID: "query.outbound-calls", ConnectorID: "novofon-sip", UnprovenWord: "Novofon-адаптер не реализован (AG-4): возможности конкретного тарифа и аккаунта не проверяемы; общая документация Novofon не считается доказательством"},
			{ID: "query.media-codec", ConnectorID: "novofon-sip", UnprovenWord: "Novofon-адаптер не реализован (AG-4): возможности конкретного тарифа и аккаунта не проверяемы; общая документация Novofon не считается доказательством"},
			{ID: "query.streaming", ConnectorID: "novofon-sip", UnprovenWord: "Novofon-адаптер не реализован (AG-4): возможности конкретного тарифа и аккаунта не проверяемы; общая документация Novofon не считается доказательством"},
			{ID: "query.dtmf-support", ConnectorID: "novofon-sip", UnprovenWord: "Novofon-адаптер не реализован (AG-4): возможности конкретного тарифа и аккаунта не проверяемы; общая документация Novofon не считается доказательством"},
			{ID: "query.transfer-bridge", ConnectorID: "novofon-sip", UnprovenWord: "Novofon-адаптер не реализован (AG-4): возможности конкретного тарифа и аккаунта не проверяемы; общая документация Novofon не считается доказательством"},
			{ID: "query.callback-state", ConnectorID: "novofon-sip", UnprovenWord: "Novofon-адаптер не реализован (AG-4): возможности конкретного тарифа и аккаунта не проверяемы; общая документация Novofon не считается доказательством"},
			{ID: "query.caller-id", ConnectorID: "novofon-sip", UnprovenWord: "Novofon-адаптер не реализован (AG-4): возможности конкретного тарифа и аккаунта не проверяемы; общая документация Novofon не считается доказательством"},
			{ID: "query.recording", ConnectorID: "novofon-sip", UnprovenWord: "Novofon-адаптер не реализован (AG-4): возможности конкретного тарифа и аккаунта не проверяемы; общая документация Novofon не считается доказательством"},
			{ID: "query.webhooks", ConnectorID: "novofon-sip", UnprovenWord: "Novofon-адаптер не реализован (AG-4): возможности конкретного тарифа и аккаунта не проверяемы; общая документация Novofon не считается доказательством"},
			{ID: "query.delivery-states", ConnectorID: "novofon-sip", UnprovenWord: "Novofon-адаптер не реализован (AG-4): возможности конкретного тарифа и аккаунта не проверяемы; общая документация Novofon не считается доказательством"},
			{ID: "query.failure-states", ConnectorID: "novofon-sip", UnprovenWord: "Novofon-адаптер не реализован (AG-4): возможности конкретного тарифа и аккаунта не проверяемы; общая документация Novofon не считается доказательством"},
		},
		RequiredGates:      []string{"OQ-7A", "OQ-8", "OQ-11", "OQ-12", "OQ-13B", "AG-1", "AG-2", "AG-4"},
		RequiredLiveGates:  []string{"OQ-7B", "OQ-14"},
		RequiredOQ11Routes: []Route{{Channel: "phone", Direction: DirectionInbound}, {Channel: "phone", Direction: DirectionOutbound}},
		FenceWords: []string{
			"запись/обработка аудио требуют purpose-specific consent, уведомления об ИИ, retention, доступа, удаления и альтернативного незаписываемого или текстового пути; отсутствие legal/privacy решения блокирует маршрут (AC-4)",
			"реальные задержка/interruption/recovery — качественные voice-метрики OQ-12 живым probe, будущий срез; срез заявляет гейт-требование словами",
		},
	},
	{
		ID:                "montagio_web",
		Title:             "сайт Montagio (Django): форма, чат, callback",
		ModalityWord:      "текст (web_chat: in/out)",
		IntentsWord:       "обращения с сайта: консультация, квалификация услуги, заявка на услугу",
		AutonomyWord:      montagioAutonomyWord,
		HumanInboxWord:    montagioHumanInboxWord,
		FallbackRouteWord: "human_inbox — безусловная передача человека (2-13)",
		RequiredCapabilities: []PilotCapabilityRequirement{
			{ID: "query.contract-schema", ConnectorID: "montagio-web", UnprovenWord: "адаптер сайта Montagio (Django) не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.authentication", ConnectorID: "montagio-web", UnprovenWord: "адаптер сайта Montagio (Django) не реализован (AG-4): capabilities не проверяемы"},
			{ID: "event.inbound-submission", ConnectorID: "montagio-web", UnprovenWord: "адаптер сайта Montagio (Django) не реализован (AG-4): capabilities не проверяемы"},
			{ID: "command.outbound-reply", ConnectorID: "montagio-web", UnprovenWord: "адаптер сайта Montagio (Django) не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.correlation-state", ConnectorID: "montagio-web", UnprovenWord: "адаптер сайта Montagio (Django) не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.media-upload-support", ConnectorID: "montagio-web", UnprovenWord: "адаптер сайта Montagio (Django) не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.delivery-status", ConnectorID: "montagio-web", UnprovenWord: "адаптер сайта Montagio (Django) не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.rate-limits", ConnectorID: "montagio-web", UnprovenWord: "адаптер сайта Montagio (Django) не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.failure-states", ConnectorID: "montagio-web", UnprovenWord: "адаптер сайта Montagio (Django) не реализован (AG-4): capabilities не проверяемы"},
		},
		RequiredGates:      []string{"OQ-7A", "OQ-8", "OQ-11", "AG-1", "AG-2", "AG-4"},
		RequiredLiveGates:  []string{"OQ-7B", "OQ-14"},
		RequiredOQ11Routes: []Route{{Channel: "web_chat", Direction: DirectionInbound}, {Channel: "web_chat", Direction: DirectionOutbound}},
		FenceWords: []string{
			"связывание заявки с тем же Case при переходах web/телефон/защищённое продолжение — контур нормализации story 5-2; retries/idempotency — семантические поля деклараций каталога, НЕ отдельные capabilities",
		},
	},
	{
		ID:                "partner_intake",
		Title:             "партнёрский intake (заявки партнёрских магазинов)",
		ModalityWord:      "заявка (web_chat: in/out)",
		IntentsWord:       "партнёрские заявки: квалификация, проверка полноты контракта и consent evidence",
		AutonomyWord:      montagioAutonomyWord,
		HumanInboxWord:    montagioHumanInboxWord,
		FallbackRouteWord: "human_inbox — безусловная передача человека (2-13)",
		RequiredCapabilities: []PilotCapabilityRequirement{
			{ID: "event.inbound-partner-lead", ConnectorID: "partner-intake", UnprovenWord: "партнёрский адаптер не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.lead-contract-version", ConnectorID: "partner-intake", UnprovenWord: "партнёрский адаптер не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.consent-evidence", ConnectorID: "partner-intake", UnprovenWord: "партнёрский адаптер не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.delivery-status", ConnectorID: "partner-intake", UnprovenWord: "партнёрский адаптер не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.failure-states", ConnectorID: "partner-intake", UnprovenWord: "партнёрский адаптер не реализован (AG-4): capabilities не проверяемы"},
		},
		RequiredGates:      []string{"OQ-7A", "OQ-8", "OQ-11", "AG-1", "AG-2", "AG-4"},
		RequiredLiveGates:  []string{"OQ-7B", "OQ-14"},
		RequiredOQ11Routes: []Route{{Channel: "web_chat", Direction: DirectionInbound}, {Channel: "web_chat", Direction: DirectionOutbound}},
		FenceWords: []string{
			"неполная партнёрская заявка не превращается автоматически в готовый к продаже Order; машины заказа — stories 5-8..5-12 (AC-6)",
		},
	},
	{
		ID:                "b2b_intake",
		Title:             "B2B-вход (кафе, рестораны, гостиницы и другие организации)",
		ModalityWord:      "заявка (web_chat: in/out)",
		IntentsWord:       "B2B-заявки организаций: квалификация, проверка организации и полномочий",
		AutonomyWord:      montagioAutonomyWord,
		HumanInboxWord:    montagioHumanInboxWord,
		FallbackRouteWord: "human_inbox — безусловная передача человека (2-13)",
		RequiredCapabilities: []PilotCapabilityRequirement{
			{ID: "event.inbound-b2b-request", ConnectorID: "b2b-intake", UnprovenWord: "B2B-адаптер не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.organization-fields", ConnectorID: "b2b-intake", UnprovenWord: "B2B-адаптер не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.authority-evidence", ConnectorID: "b2b-intake", UnprovenWord: "B2B-адаптер не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.delivery-status", ConnectorID: "b2b-intake", UnprovenWord: "B2B-адаптер не реализован (AG-4): capabilities не проверяемы"},
			{ID: "query.failure-states", ConnectorID: "b2b-intake", UnprovenWord: "B2B-адаптер не реализован (AG-4): capabilities не проверяемы"},
		},
		RequiredGates:      []string{"OQ-7A", "OQ-8", "OQ-11", "AG-1", "AG-2", "AG-4"},
		RequiredLiveGates:  []string{"OQ-7B", "OQ-14"},
		RequiredOQ11Routes: []Route{{Channel: "web_chat", Direction: DirectionInbound}, {Channel: "web_chat", Direction: DirectionOutbound}},
		FenceWords: []string{
			"отсутствие полномочий согласующего бюджет или плательщика сохраняется как unknown, а не подразумевается (AC-7)",
		},
	},
	{
		ID:                 "protected_web_continuation",
		Title:              "защищённое web-продолжение (step-up, медиа, безопасный ввод)",
		ModalityWord:       "текст+медиа (web_chat: in/out)",
		IntentsWord:        "продолжение Case: медиа, step-up, безопасный ввод, документы",
		AutonomyWord:       montagioAutonomyWord,
		HumanInboxWord:     montagioHumanInboxWord,
		FallbackRouteWord:  "human_inbox — безусловная передача человека (2-13)",
		RequiredGates:      []string{"OQ-7A", "OQ-8", "OQ-11", "AG-1", "AG-2", "AG-4"},
		RequiredLiveGates:  []string{"OQ-7B", "OQ-14"},
		RequiredOQ11Routes: []Route{{Channel: "web_chat", Direction: DirectionInbound}, {Channel: "web_chat", Direction: DirectionOutbound}},
		FenceWords: []string{
			"клиент не начинает обращение заново: Case, подтверждённые факты, Response Owner и continuation token сохраняются (2-10)",
		},
	},
	{
		ID:                "human_inbox",
		Title:             "единый подтверждённый Human Inbox Montagio",
		ModalityWord:      "текст (поверхность Staffy)",
		IntentsWord:       "человеческая поверхность: приём обращений, fallback, возврат управления",
		AutonomyWord:      montagioAutonomyWord,
		HumanInboxWord:    montagioHumanInboxWord,
		FallbackRouteWord: "минимальная операторская поверхность Staffy — человеческий путь всегда доступен (FR-83)",
		RequiredGates:     []string{"OQ-7A", "OQ-8", "AG-1", "AG-2", "AG-4"},
		RequiredLiveGates: []string{"OQ-7B", "OQ-14"},
		FallbackWords: []string{
			"одна подтверждённая Human Inbox: поддерживаемая внешняя система либо минимальная операторская поверхность Staffy; неподтверждённое уведомление не считается принятым человеком (AC-10)",
			"канального OQ-11 нет: Staffy-own поверхность, платформенный периметр 1-10/1-11/oq-11-staffy-data честным словом",
		},
	},
	{
		ID:                "avito_candidate",
		Title:             "Avito (вне пилотного production substrate)",
		ModalityWord:      "— (вне пилотного substrate)",
		IntentsWord:       "— (маршрут не допущен: обращений нет)",
		AutonomyWord:      montagioAutonomyWord,
		HumanInboxWord:    montagioHumanInboxWord,
		FallbackRouteWord: "— (маршрут не допущен: обращений нет)",
		RequiredGates:     []string{"OQ-18"},
		CapWord:           "официальный разрешённый способ интеграции не доказан: Avito не входит в пилотный production substrate (OQ-18); неофициальный scraping, имитация клиента и обход ограничений платформы запрещены",
		CapNextOwner:      "Product Owner + Partnerships/Architecture (OQ-18)",
		FenceWords: []string{
			"Avito-деклараций в каталоге нет: заявлять capability неподтверждённого провайдера запрещено; Connector допускается позднее после отдельных capability/policy/compliance gates (AC-8)",
		},
	},
	{
		ID:                 "public_commercial",
		Title:              "публичный коммерческий маршрут (Order/payment)",
		ModalityWord:       "текст (web_chat: in/out)",
		IntentsWord:        "публичная продажа: каталог услуг, цена (OQ-9), Order, оплата, чек",
		AutonomyWord:       montagioAutonomyWord,
		HumanInboxWord:     montagioHumanInboxWord,
		FallbackRouteWord:  "human_inbox — безусловная передача человека (2-13)",
		RequiredGates:      []string{"OQ-7A", "OQ-8", "OQ-9", "OQ-10", "OQ-13A", "OQ-11", "AG-1", "AG-2", "AG-4"},
		RequiredLiveGates:  []string{"OQ-7B", "OQ-14"},
		RequiredOQ11Routes: []Route{{Channel: "web_chat", Direction: DirectionInbound}, {Channel: "web_chat", Direction: DirectionOutbound}},
		CapWord:            "бизнес-машины Order/payment Montagio не построены (stories 5-8..5-12/5-17 — будущие срезы); OQ-10 блокирует публичный маршрут, НЕ блокируя закрытые validation-сценарии",
		CapNextOwner:       "Product Owner + Legal/Finance/Operations (OQ-10) + build-out 5-8..5-12/5-17",
		FenceWords: []string{
			"OQ-10 открыт: public_contractor_order=intake_only/no_promises; закрытый тест правдиво называет фактические стороны — безопасные внутренние и закрытые validation-сценарии НЕ блокируются (AC-12)",
			"OQ-9 — цена показывается только этим маршрутом: верхнерыночная ценовая машина — story 5-6",
		},
	},
}

// MontagioRoutes returns the fixed route dictionary in order (a copy).
func MontagioRoutes() []MontagioRoute {
	return append([]MontagioRoute(nil), montagioRoutes...)
}

// MontagioRouteByID resolves one route word of the dictionary.
func MontagioRouteByID(routeID string) (MontagioRoute, bool) {
	for _, route := range montagioRoutes {
		if route.ID == routeID {
			return route, true
		}
	}
	return MontagioRoute{}, false
}

// ---------------------------------------------------------------------------
// Pinned honest words of the derivation
// ---------------------------------------------------------------------------

// montagioCapabilityStateWords are the pinned per-state honest words of an
// unproven capability — the same DERIVED states family of spec 1-9 the Oleg
// ladder carries (one honest dictionary of the domain).
var montagioCapabilityStateWords = pilotCapabilityStateWords

// montagioCapabilityMissingWord is the pinned word when no live fact exists.
const montagioCapabilityMissingWord = pilotCapabilityMissingWord

// montagioConnectorNextSteps are the pinned next-step words per connector.
var montagioConnectorNextSteps = map[string]string{
	"novofon-sip":    "реализация адаптера + account-scoped read-only probe после гейтов (AG-4); общая документация Novofon — не доказательство",
	"montagio-web":   "реализация адаптера сайта Montagio (Django) после AG-4",
	"partner-intake": "реализация партнёрского адаптера после AG-4",
	"b2b-intake":     "реализация B2B-адаптера после AG-4",
}

// montagioGateWords carries the pinned honest composition words of every
// gate referenced by a Montagio route: the close phrase (what exactly is
// not closed), the safe state and the next owner of the check (the
// accountable owner of the registry canon). The blocker word is
// "<ID> <state>: <close>"; the safe state is carried beside it.
var montagioGateWords = map[string]struct {
	Close     string
	SafeState string
	NextOwner string
}{
	"OQ-7A": {
		Close:     "план контролируемой когорты Montagio (UJ-M1/UJ-M2) не предзарегистрирован: allocation, denominator, eligibility, exclusions/crossover, matching и measurement owners",
		SafeState: "simulation only; ни один маршрут не подменяет другой",
		NextOwner: "Product Owner + Montagio Operations",
	},
	"OQ-8": {
		Close:     "владельцы fallback Montagio не назначены: supply, safety/technical, commercial/legal, recovery; acknowledgement windows, backup/escalation и drill",
		SafeState: "no immediate-help promise; named owner/next update or honest intake/callback",
		NextOwner: "Montagio Operations",
	},
	"OQ-9": {
		Close:     "верхнерыночная ценовая политика не утверждена: market cut, правило, cost floor, source/version, freshness, exceptions",
		SafeState: "approved range/measurement/human route; no fixed-price promise",
		NextOwner: "Product Owner + Pricing/Operations",
	},
	"OQ-10": {
		Close:     "договорная модель подрядчика не закрыта: contract party, payment, tax/KKT/receipt, insurance, warranty, claim/refund, subcontractor model; tabletop approval",
		SafeState: "public_contractor_order=intake_only/no_promises; закрытый тест правдиво называет фактические стороны",
		NextOwner: "Product Owner + Legal/Finance/Operations",
	},
	"OQ-11": {
		Close:     "data-маршрут не подтверждён purpose-specific consent",
		SafeState: "synthetic/non-PII only",
		NextOwner: "Security/Privacy + Technical Lead",
	},
	"OQ-12": {
		Close:     "маршрутные voice-метрики Novofon не измерены живым probe (задержка, interruption, recovery)",
		SafeState: "без числовых обещаний immediacy",
		NextOwner: "Voice/Technical Lead",
	},
	"OQ-13A": {
		Close:     "решение best-effort/без SLA не зафиксировано утверждённой формулировкой",
		SafeState: "no contractual uptime claim",
		NextOwner: "Product Owner + Tenant",
	},
	"OQ-13B": {
		Close:     "восстановимый drill медиа/голоса не записан",
		SafeState: "synthetic or closed-account test only",
		NextOwner: "Technical Lead + Operations",
	},
	"OQ-14": {
		Close:     "количественные baseline-пороги live не утверждены",
		SafeState: "manual scoped promotion",
		NextOwner: "Product Owner + Operations/QA",
	},
	"OQ-7B": {
		Close:     "baseline-derived пороги маршрута и достаточность выборки не утверждены",
		SafeState: "controlled learning only",
		NextOwner: "Product Owner + Montagio Operations",
	},
	"OQ-18": {
		Close:     "официальный и разрешённый способ интеграции не доказан: demand, official API/terms, account capability, residency/security, cost, failure semantics",
		SafeState: "not shown available; assisted extension only",
		NextOwner: "Product Owner + Partnerships/Architecture",
	},
	"AG-1": {
		Close:     "pilot load envelope не тестирован",
		SafeState: "simulation/load test only",
		NextOwner: "Technical Lead + Operations",
	},
	"AG-2": {
		Close:     "accessibility evidence отсутствует",
		SafeState: "affected route не публикуется",
		NextOwner: "Product Owner + UX/QA",
	},
	"AG-4": {
		Close:     "runtime-зависимости не подтверждены (адаптеры Novofon/Django/партнёра/B2B не реализованы; production credentials запрещены)",
		SafeState: "только локальная синтетическая реализация; без реальных credentials",
		NextOwner: "Technical Lead + Security",
	},
}

// montagioGateDictionaryIDs mirrors the six Montagio gate definitions the
// registry carries (the dictionary extension of spec 5-1) — the wire and
// OpenAPI enums pin it (the mirror discipline: routes 8 / gates 6 /
// stages 4).
var montagioGateDictionaryIDs = []string{"OQ-7A", "OQ-8", "OQ-9", "OQ-10", "OQ-13A", "OQ-18"}

// MontagioGateDictionaryIDs returns the six Montagio gate dictionary words
// in fixed order (a copy).
func MontagioGateDictionaryIDs() []string {
	return append([]string(nil), montagioGateDictionaryIDs...)
}

// ---------------------------------------------------------------------------
// The read-time derivation
// ---------------------------------------------------------------------------

// MontagioFacts are the live facts of the Montagio admission: the sealed
// pack pin, the derived capability states by catalog capability ID and the
// full gate instance history. The app layer assembles them from reused
// read ports (never secrets); the derivation is read-time only — a stored
// "computed" admission does not exist.
type MontagioFacts struct {
	PackID       string
	PackVersion  string
	Capabilities map[string]CapabilityFact
	Instances    []GateInstance
}

// MontagioRouteAdmission is the derived admission of one Montagio route:
// the stage, the honest stage word, the Route Matrix column words, the
// per-capability and per-gate requirement views, the blockers and the
// pinned cap/fence/fallback words.
type MontagioRouteAdmission struct {
	RouteID           string                `json:"route_id"`
	Title             string                `json:"title"`
	Stage             string                `json:"stage"`
	StageWord         string                `json:"stage_word"`
	ModalityWord      string                `json:"modality_word"`
	IntentsWord       string                `json:"intents_word"`
	AutonomyWord      string                `json:"autonomy_word"`
	HumanInboxWord    string                `json:"human_inbox_word"`
	FallbackRouteWord string                `json:"fallback_route_word"`
	CapWord           string                `json:"cap_word,omitempty"`
	Capabilities      []PilotCapabilityView `json:"capabilities,omitempty"`
	Gates             []PilotGateView       `json:"gates,omitempty"`
	LiveGates         []PilotGateView       `json:"live_gates,omitempty"`
	Blockers          []PilotBlocker        `json:"blockers,omitempty"`
	FenceWords        []string              `json:"fence_words,omitempty"`
	FallbackWords     []string              `json:"fallback_words,omitempty"`
}

// MontagioAdmissionReport is the whole read-time derivation: the pack pin
// state, the approved AC-11 wording dictionary, the six-word Montagio gate
// dictionary mirror and every route in dictionary order.
type MontagioAdmissionReport struct {
	PackPinned   bool                     `json:"pack_pinned"`
	PackWord     string                   `json:"pack_word"`
	WordingWords []string                 `json:"wording_words"`
	GateIDs      []string                 `json:"montagio_gate_ids"`
	Routes       []MontagioRouteAdmission `json:"routes"`
}

// DeriveMontagioAdmission is the pure read-time derivation of the Montagio
// admission: the pack pin, every route's capability ladder, controlled and
// live gate conjunctions, with the pinned honest words. Nothing is cached;
// a stale capability fact, a superseded PASS or a rotated secret reflects
// at the next read immediately. The function performs no writes and holds
// no state — an unchanged world always yields the byte-stable same answer.
// A blocking of one route never extends to another route (AC-13/AC-14).
func DeriveMontagioAdmission(facts MontagioFacts) MontagioAdmissionReport {
	report := MontagioAdmissionReport{
		Routes:       []MontagioRouteAdmission{},
		WordingWords: append([]string(nil), MontagioWordingWords...),
		GateIDs:      append([]string(nil), montagioGateDictionaryIDs...),
	}
	// (1) The pack pin: the fail-closed prerequisite of every route.
	packOK := facts.PackID == MontagioRequiredPackID && PackVersionAtLeast(facts.PackVersion, MontagioRequiredPackMinVersion)
	report.PackPinned = packOK
	if packOK {
		report.PackWord = fmt.Sprintf("Domain Pack «Услуги» прикреплён к запечатанному контуру: %s@%s", facts.PackID, facts.PackVersion)
	} else {
		report.PackWord = MontagioPackMissingWord
	}
	// (2–5) Every route of the fixed dictionary, independently.
	for _, route := range montagioRoutes {
		admission := MontagioRouteAdmission{
			RouteID: route.ID, Title: route.Title,
			ModalityWord: route.ModalityWord, IntentsWord: route.IntentsWord,
			AutonomyWord: route.AutonomyWord, HumanInboxWord: route.HumanInboxWord,
			FallbackRouteWord: route.FallbackRouteWord,
			CapWord:           route.CapWord, FenceWords: route.FenceWords, FallbackWords: route.FallbackWords,
		}
		capabilityOK := true
		if !packOK {
			admission.Blockers = append(admission.Blockers, PilotBlocker{
				Source: "pack", ID: MontagioRequiredPackID, Word: MontagioPackMissingWord,
				NextOwner: "Owner/Configurator — запечатать манифест с пином Domain Pack «Услуги» версии 1.0.0 или новее",
			})
		}
		// (2) The cap word: a capped route never rises above candidate this
		// slice — the honest cap is carried beside the stage, never hidden.
		capped := route.CapWord != ""
		if capped {
			admission.Blockers = append(admission.Blockers, PilotBlocker{
				Source: "cap", ID: route.ID, Word: route.CapWord,
				NextOwner: route.CapNextOwner,
			})
		}
		// (3) The capability ladder: every required capability must be proven
		// by live evidence (verified, no gap). Connected alone is never a
		// proof; an adapter not implemented caps the route at candidate with
		// the honest adapter word.
		for _, requirement := range route.RequiredCapabilities {
			view := PilotCapabilityView{ID: requirement.ID, ConnectorID: requirement.ConnectorID}
			fact, exists := facts.Capabilities[requirement.ID]
			verified := exists && fact.State == "verified" && !fact.Gap
			if verified {
				view.State = "verified"
				admission.Capabilities = append(admission.Capabilities, view)
				continue
			}
			capabilityOK = false
			view.State = "unproven"
			stateWord := montagioCapabilityMissingWord
			if exists {
				stateWord = montagioCapabilityStateWords[fact.State]
				if stateWord == "" {
					stateWord = "состояние неизвестно — успех не заявляется"
				}
				if fact.Reason != "" {
					stateWord = stateWord + " (" + fact.Reason + ")"
				}
			}
			view.Word = requirement.UnprovenWord + " — " + stateWord
			admission.Capabilities = append(admission.Capabilities, view)
			admission.Blockers = append(admission.Blockers, PilotBlocker{
				Source: "capability", ID: requirement.ID, Word: view.Word,
				NextOwner: montagioConnectorNextSteps[requirement.ConnectorID],
			})
		}
		// (4) The controlled gate conjunction: every required gate must carry
		// an acting PASS in the exact scope — tenant-level gates at tenant
		// level, the route-scoped OQ-11 per declared channel route (one honest
		// view per channel route, the precedent of the Oleg ladder). An open,
		// missing, in_test, fail or expired instance blocks with the exact gate
		// word, the safe state and the next owner.
		controlledOK := true
		for _, gateID := range route.RequiredGates {
			word, known := montagioGateWords[gateID]
			if !known {
				// An unworded gate is a dictionary integrity break — the
				// mirror test pins this away; the derivation refuses to
				// invent words.
				controlledOK = false
				admission.Gates = append(admission.Gates, PilotGateView{ID: gateID, State: "missing", Word: "слово гейта отсутствует в словаре Montagio — целостность словаря нарушена"})
				continue
			}
			if gateID == "OQ-11" {
				if len(route.RequiredOQ11Routes) == 0 {
					// The route names OQ-11 without a channel mapping: an honest
					// scope boundary — no applicable instance can exist this slice
					// (fail-closed, never a silent pass).
					controlledOK = false
					view := PilotGateView{
						ID: gateID, State: "missing",
						Word:      "OQ-11 missing: у маршрута нет канального scope — применимый инстанс невозможен этим срезом",
						SafeState: word.SafeState,
						NextOwner: word.NextOwner + " — расширение словаря каналов = новая версия спеки",
					}
					admission.Gates = append(admission.Gates, view)
					admission.Blockers = append(admission.Blockers, PilotBlocker{Source: "gate", ID: gateID, Word: view.Word, SafeState: view.SafeState, NextOwner: view.NextOwner})
					continue
				}
				for _, channelRoute := range route.RequiredOQ11Routes {
					view := PilotGateView{ID: gateID, SafeState: word.SafeState, NextOwner: word.NextOwner}
					if pass := ValidPassForRoute(gateID, facts.Instances, channelRoute); pass != nil {
						view.State = GateStatePass
					} else {
						state, reason := InstanceStateForRoute(gateID, facts.Instances, channelRoute)
						controlledOK = false
						view.State = state
						view.Word = fmt.Sprintf("OQ-11 %s (маршрут %s): %s", state, channelRoute, word.Close)
						admission.Blockers = append(admission.Blockers, PilotBlocker{
							Source: "gate", ID: gateID, Word: view.Word + " — " + reason,
							SafeState: view.SafeState, NextOwner: view.NextOwner,
						})
					}
					admission.Gates = append(admission.Gates, view)
				}
				continue
			}
			view := PilotGateView{ID: gateID, SafeState: word.SafeState, NextOwner: word.NextOwner}
			if pass := validPassForTenant(gateID, facts.Instances); pass != nil {
				view.State = GateStatePass
			} else {
				state, reason := instanceStateForTenant(gateID, facts.Instances)
				controlledOK = false
				view.State = state
				view.Word = gateID + " " + state + ": " + word.Close
				admission.Blockers = append(admission.Blockers, PilotBlocker{
					Source: "gate", ID: gateID, Word: view.Word + " — " + reason,
					SafeState: view.SafeState, NextOwner: view.NextOwner,
				})
			}
			admission.Gates = append(admission.Gates, view)
		}
		// (5) The live gate conjunction ({OQ-7B, OQ-14} — every route except
		// the eternal avito_candidate): tenant-level acting PASS only.
		liveOK := true
		for _, gateID := range route.RequiredLiveGates {
			word, known := montagioGateWords[gateID]
			if !known {
				// An unworded gate is a dictionary integrity break — the
				// mirror test pins this away; the derivation refuses to
				// invent words.
				liveOK = false
				admission.LiveGates = append(admission.LiveGates, PilotGateView{ID: gateID, State: "missing", Word: "слово гейта отсутствует в словаре Montagio — целостность словаря нарушена"})
				continue
			}
			view := PilotGateView{ID: gateID, SafeState: word.SafeState, NextOwner: word.NextOwner}
			if pass := validPassForTenant(gateID, facts.Instances); pass != nil {
				view.State = GateStatePass
			} else {
				state, reason := instanceStateForTenant(gateID, facts.Instances)
				liveOK = false
				view.State = state
				view.Word = gateID + " " + state + ": " + word.Close
				admission.LiveGates = append(admission.LiveGates, view)
				admission.Blockers = append(admission.Blockers, PilotBlocker{
					Source: "live-gate", ID: gateID, Word: view.Word + " — " + reason,
					SafeState: view.SafeState, NextOwner: view.NextOwner,
				})
				continue
			}
			admission.LiveGates = append(admission.LiveGates, view)
		}
		// (6) The stage: the minimum of the contributions — candidate while
		// the pack pin, the cap or any capability is unproven; verified when
		// every capability is proven but the controlled gates stay open;
		// controlled when the controlled conjunction holds; live only by the
		// full double conjunction.
		switch {
		case capped || !capabilityOK || !packOK:
			admission.Stage = MontagioStageCandidate
		case !controlledOK:
			admission.Stage = MontagioStageVerified
		case !liveOK:
			admission.Stage = MontagioStageControlled
		default:
			admission.Stage = MontagioStageLive
		}
		admission.StageWord = montagioStageWords[admission.Stage]
		report.Routes = append(report.Routes, admission)
	}
	return report
}

// MontagioCapabilityIDs lists the required capability IDs of one route word.
func MontagioCapabilityIDs(routeID string) []string {
	route, ok := MontagioRouteByID(routeID)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(route.RequiredCapabilities))
	for _, requirement := range route.RequiredCapabilities {
		ids = append(ids, requirement.ID)
	}
	return ids
}

// MontagioRouteGateIDs lists the required controlled gate IDs of one route word.
func MontagioRouteGateIDs(routeID string) []string {
	route, ok := MontagioRouteByID(routeID)
	if !ok {
		return nil
	}
	return append([]string(nil), route.RequiredGates...)
}

// MontagioRouteLiveGateIDs lists the required live gate IDs of one route word.
func MontagioRouteLiveGateIDs(routeID string) []string {
	route, ok := MontagioRouteByID(routeID)
	if !ok {
		return nil
	}
	return append([]string(nil), route.RequiredLiveGates...)
}
