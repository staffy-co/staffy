package domain

import (
        "fmt"
)

// ---------------------------------------------------------------------------
// Pilot Admission Registry (spec 4-1, Epic 4)
//
// The compile-time dictionaries of the auto-parts pilot of the Oleg store:
// the fixed route vocabulary {text_jivo, voice_sip, parts_catalog_abcp,
// parts_order_abcp, abcp_1c_sync, human_fallback}, the ABCP→custom-1C
// State/Authority Matrix (OQ-19) and the READ-TIME admission derivation
// {candidate → verified → admitted} over live facts. Nothing here writes:
// admission is a derivation, not a record — a repeated read of an unchanged
// world returns the byte-stable same answer. The dictionaries are fixed in
// the domain (never tenant-configurable); extending them is a new spec
// version.
// ---------------------------------------------------------------------------

// Admission stages of a pilot route (AC-13): candidate (declared, the
// required capabilities are not proven by live evidence), verified (every
// required capability is proven, the admission gates are not closed) and
// admitted (verified and every required gate carries an acting PASS — the
// only conjunctive path). The stages are fixed words.
const (
        PilotStageCandidate = "candidate"
        PilotStageVerified  = "verified"
        PilotStageAdmitted  = "admitted"
)

// The required Domain Pack pin of every pilot route (AC-1): the versioned
// «Автозапчасти» pack of the shared core. A fork, a tenant module or a
// tenant branch of the core is structurally impossible (AD-1/AD-2/CI).
const (
        PilotRequiredPackID         = "auto-parts"
        PilotRequiredPackMinVersion = "2.0.0"
)

// PilotPackMissingWord is the pinned honest word of the missing pack pin:
// the whole pilot admission is fail-closed until the sealed manifest pins
// the required pack (row 1 of the spec's edge-case matrix).
const PilotPackMissingWord = "Domain Pack «Автозапчасти» не прикреплён к запечатанному контуру"

// pilotStageWords are the pinned honest stage words (AC-13: the stage reads
// as text, never as color or position alone).
var pilotStageWords = map[string]string{
        PilotStageCandidate: "маршрут заявлен; требуемые capabilities живыми доказательствами не подтверждены",
        PilotStageVerified:  "все требуемые capabilities подтверждены живой evidence; гейты допуска не закрыты",
        PilotStageAdmitted:  "все требуемые capabilities подтверждены живой evidence и все требуемые гейты имеют действующие PASS",
}

// ---------------------------------------------------------------------------
// The route dictionary (six fixed words)
// ---------------------------------------------------------------------------

// PilotCapabilityRequirement is one catalog capability a route must prove by
// live evidence. The IDs are the catalog dictionary words (spec 1-9); the
// UnprovenWord is the pinned honest word shown while no live evidence
// exists (the adapter honesty of AG-4: a product page or a documentation
// claim is never a proof).
type PilotCapabilityRequirement struct {
        ID          string `json:"id"`
        ConnectorID string `json:"connector_id"`
        UnprovenWord string `json:"unproven_word"`
}

// PilotRoute is one route word of the fixed pilot dictionary: what it is,
// which catalog capabilities it requires, which gates close its admission,
// and the pinned fence/fallback words of the surface.
type PilotRoute struct {
        ID    string `json:"id"`
        Title string `json:"title"`
        // RequiredCapabilities are the catalog capability IDs the route must
        // prove (empty means the route needs no external connector proof — the
        // honest vacuous case of the Staffy-own surface).
        RequiredCapabilities []PilotCapabilityRequirement `json:"required_capabilities,omitempty"`
        // RequiredGates are the dictionary gate IDs whose acting PASS the
        // admission requires (conjunction — a disjunction does not exist in the
        // registry model).
        RequiredGates []string `json:"required_gates,omitempty"`
        // RequiredOQ11Routes are the exact channel routes whose route-scoped
        // OQ-11 PASS the route consumes. An OQ-11 requirement with no channel
        // mapping is the honest scope boundary of a backend data route: the
        // channel dictionary carries no ABCP dimension, so no applicable
        // instance can exist this slice (fail-closed, never a silent pass).
        RequiredOQ11Routes []Route `json:"required_oq11_routes,omitempty"`
        // MatrixDependent marks the routes whose capability ladder is the
        // ABCP→1C matrix itself: their material fields' terminal evidence is
        // the proof, and until a terminal-evidence source exists the route
        // never rises above candidate (unknown stays unknown).
        MatrixDependent bool `json:"matrix_dependent,omitempty"`
        // FenceWords are the pinned order-fence words (AC-9).
        FenceWords []string `json:"fence_words,omitempty"`
        // FallbackWords are the pinned FR-83/OQ-17 words of the human surface.
        FallbackWords []string `json:"fallback_words,omitempty"`
}

// pilotRoutes is the fixed order of the six route words (the Exit rows of
// UJ-A1 slice 1/2 of the Capability→Architecture Map).
var pilotRoutes = []PilotRoute{
        {
                ID:    "text_jivo",
                Title: "текст: Jivo Human Inbox",
                RequiredCapabilities: []PilotCapabilityRequirement{
                        {ID: "event.inbound-message", ConnectorID: "jivo-chat", UnprovenWord: "Jivo-адаптер не реализован (AG-4): capabilities не проверяемы"},
                        {ID: "command.outbound-reply", ConnectorID: "jivo-chat", UnprovenWord: "Jivo-адаптер не реализован (AG-4): capabilities не проверяемы"},
                        {ID: "query.delivery-status", ConnectorID: "jivo-chat", UnprovenWord: "Jivo-адаптер не реализован (AG-4): capabilities не проверяемы"},
                        {ID: "command.assign-to-human", ConnectorID: "jivo-chat", UnprovenWord: "Jivo-адаптер не реализован (AG-4): capabilities не проверяемы"},
                        {ID: "event.acknowledgement", ConnectorID: "jivo-chat", UnprovenWord: "Jivo-адаптер не реализован (AG-4): capabilities не проверяемы"},
                        {ID: "query.one-writer-state", ConnectorID: "jivo-chat", UnprovenWord: "Jivo-адаптер не реализован (AG-4): capabilities не проверяемы"},
                        {ID: "query.media-upload-support", ConnectorID: "jivo-chat", UnprovenWord: "Jivo-адаптер не реализован (AG-4): capabilities не проверяемы"},
                        {ID: "query.fallback-state", ConnectorID: "jivo-chat", UnprovenWord: "Jivo-адаптер не реализован (AG-4): capabilities не проверяемы"},
                },
                RequiredGates:      []string{"OQ-2", "OQ-11", "AG-2", "AG-4"},
                RequiredOQ11Routes: []Route{{Channel: "jivo", Direction: DirectionInbound}, {Channel: "jivo", Direction: DirectionOutbound}},
        },
        {
                ID:    "voice_sip",
                Title: "голос: SIP-линия",
                RequiredCapabilities: []PilotCapabilityRequirement{
                        {ID: "query.trunk-status", ConnectorID: "staffy-synthetic-telephony", UnprovenWord: "capability не подтверждена живым синтетическим evidence"},
                        {ID: "query.transfer-bridge", ConnectorID: "staffy-synthetic-telephony", UnprovenWord: "capability не подтверждена живым синтетическим evidence"},
                        {ID: "query.callback-state", ConnectorID: "staffy-synthetic-telephony", UnprovenWord: "capability не подтверждена живым синтетическим evidence"},
                        {ID: "query.dtmf-support", ConnectorID: "staffy-synthetic-telephony", UnprovenWord: "capability не подтверждена живым синтетическим evidence"},
                        {ID: "query.playback-delivery", ConnectorID: "staffy-synthetic-telephony", UnprovenWord: "capability не подтверждена живым синтетическим evidence"},
                        {ID: "query.failure-states", ConnectorID: "staffy-synthetic-telephony", UnprovenWord: "capability не подтверждена живым синтетическим evidence"},
                },
                RequiredGates: []string{"OQ-1", "OQ-5", "OQ-11", "OQ-12", "OQ-13B", "AG-1", "AG-2", "AG-4"},
                RequiredOQ11Routes: []Route{{Channel: "phone", Direction: DirectionInbound}, {Channel: "phone", Direction: DirectionOutbound}},
        },
        {
                ID:    "parts_catalog_abcp",
                Title: "каталог ABCP/Laximo read-only",
                RequiredCapabilities: []PilotCapabilityRequirement{
                        {ID: "query.identify-vehicle", ConnectorID: "abcp-parts", UnprovenWord: "ABCP-адаптер не реализован (AG-4): capabilities не проверяемы"},
                        {ID: "query.catalog-navigation", ConnectorID: "abcp-parts", UnprovenWord: "ABCP-адаптер не реализован (AG-4): capabilities не проверяемы"},
                        {ID: "query.fitment-applicability", ConnectorID: "abcp-parts", UnprovenWord: "ABCP-адаптер не реализован (AG-4): capabilities не проверяемы"},
                        {ID: "query.oem-cross", ConnectorID: "abcp-parts", UnprovenWord: "ABCP-адаптер не реализован (AG-4): capabilities не проверяемы"},
                        {ID: "query.supplier-offers", ConnectorID: "abcp-parts", UnprovenWord: "ABCP-адаптер не реализован (AG-4): capabilities не проверяемы"},
                        {ID: "query.price-availability-term", ConnectorID: "abcp-parts", UnprovenWord: "ABCP-адаптер не реализован (AG-4): capabilities не проверяемы"},
                },
                RequiredGates: []string{"OQ-5", "OQ-11", "AG-4"},
        },
        {
                ID:    "parts_order_abcp",
                Title: "клиентский заказ ABCP (подготовка, без отправки поставщику)",
                RequiredCapabilities: []PilotCapabilityRequirement{
                        {ID: "command.prepare-client-order", ConnectorID: "abcp-parts", UnprovenWord: "ABCP-адаптер не реализован (AG-4): мутирующая capability не проверяема — probe запрещён (ProbeAllowed false, Action Gateway)"},
                },
                RequiredGates: []string{"OQ-3", "OQ-4", "OQ-5", "OQ-11", "OQ-19", "AG-1", "AG-2", "AG-4"},
                FenceWords:   []string{"автоматическая отправка поставщику не выполняется; заказ требует pending_manager_verification и явного одобрения менеджера — OQ-3/OQ-4"},
        },
        {
                ID:              "abcp_1c_sync",
                Title:           "обмен ABCP→кастомная 1С",
                RequiredGates:   []string{"OQ-4", "OQ-19"},
                MatrixDependent: true,
        },
        {
                ID:           "human_fallback",
                Title:        "минимальная операторская поверхность Staffy / fallback",
                RequiredGates: []string{"AG-2", "AG-4"},
                FallbackWords: []string{
                        "минимальная операторская поверхность Staffy (FR-83): человеческий путь всегда доступен",
                        "fallback не зависит от непроверенных внешних систем (OQ-17-связка: соответствующий Inbox обязателен для production)",
                },
        },
}

// PilotRoutes returns the fixed route dictionary in order (a copy).
func PilotRoutes() []PilotRoute {
        return append([]PilotRoute(nil), pilotRoutes...)
}

// PilotRouteByID resolves one route word of the dictionary.
func PilotRouteByID(routeID string) (PilotRoute, bool) {
        for _, route := range pilotRoutes {
                if route.ID == routeID {
                        return route, true
                }
        }
        return PilotRoute{}, false
}

// ---------------------------------------------------------------------------
// The ABCP→custom-1C State/Authority Matrix (OQ-19, AC-10)
// ---------------------------------------------------------------------------

// Owning-system words of the matrix.
const (
        MatrixOwnerABCP       = "ABCP"
        MatrixOwner1C         = "1C"
        MatrixOwnerStaffyCase = "Staffy-Case"
)

// MatrixField is one material field of the ABCP→custom-1C exchange: the
// declared owning system, the external identity discipline, the version and
// time discipline, the allowed Staffy projection, the conflict rule, the
// reconcile query and the terminal-evidence requirement. The LIVE state of
// every field is a read-time derivation — this slice has no
// terminal-evidence source, so every field stays unknown and the derivation
// never infers success (unknown stays unknown).
type MatrixField struct {
        Field                 string `json:"field"`
        Title                 string `json:"title"`
        OwningSystem          string `json:"owning_system"`
        ExternalIDText        string `json:"external_id_text"`
        VersionTimeText       string `json:"version_time_text"`
        AllowedProjectionText string `json:"allowed_projection_text"`
        ConflictRuleText      string `json:"conflict_rule_text"`
        ReconcileQueryText    string `json:"reconcile_query_text"`
        TerminalEvidenceText  string `json:"terminal_evidence_text"`
}

// MatrixUnknownWord is the pinned live-state word of every matrix field of
// this slice: no terminal-evidence source exists, the field stays unknown
// and no success is claimed (row 8 of the spec's edge-case matrix).
const MatrixUnknownWord = "unknown — терминальное доказательство отсутствует, успех не заявляется"

// MatrixFields returns the fixed ABCP→1C matrix in order (a copy).
func MatrixFields() []MatrixField {
        return append([]MatrixField(nil), abcpTo1CMatrix...)
}

// abcpTo1CMatrix is the compile-time State/Authority Matrix of the OQ-19
// canon: every material field of the exchange with its authority and
// disciplines (AC-10 verbatim dimensions).
var abcpTo1CMatrix = []MatrixField{
        {
                Field:        "client_order_state",
                Title:        "Состояние клиентского заказа",
                OwningSystem: MatrixOwnerABCP,
                ExternalIDText:        "идентификатор клиентского заказа в ABCP — внутренний ключ prepared-статуса",
                VersionTimeText:       "версия и время состояния заказа читаются из ABCP (reread по OQ-3)",
                AllowedProjectionText: "Staffy читает состояние prepared-заказа как проекцию; мутация возможна только через Action Gateway",
                ConflictRuleText:      "при расхождении состояний ABCP↔1С поле остаётся unknown до ручной сверки (OQ-4)",
                ReconcileQueryText:    "повторное чтение статуса ABCP и запрос состояния 1С",
                TerminalEvidenceText:  "терминальный успех подтверждается квитанцией закрытого заказа",
        },
        {
                Field:        "client_order_id",
                Title:        "Идентификатор клиентского заказа",
                OwningSystem: MatrixOwnerABCP,
                ExternalIDText:        "клиентский заказ создаётся в ABCP; внешний ID назначается стороной ABCP",
                VersionTimeText:       "ID неизменен; временем создания управляет ABCP",
                AllowedProjectionText: "Staffy хранит ссылку на внешний ID как корреляцию, не как собственный идентификатор",
                ConflictRuleText:      "дубликат внешнего ID = ручная сверка, автослияние запрещено",
                ReconcileQueryText:    "поиск заказа по внешнему ID в ABCP и 1С",
                TerminalEvidenceText:  "совпадение ID в обеих системах после сверки",
        },
        {
                Field:        "external_1c_id",
                Title:        "Внешний идентификатор заказа в 1С",
                OwningSystem: MatrixOwner1C,
                ExternalIDText:        "заказ в кастомной 1С несёт собственный внешний ID",
                VersionTimeText:       "время создания заказа в 1С читается стороной 1С",
                AllowedProjectionText: "Staffy читает связку ABCP-заказ ↔ 1С-заказ как проекцию соответствия",
                ConflictRuleText:      "отсутствие или расхождение связки = unknown, ручное сопоставление",
                ReconcileQueryText:    "запрос соответствующего заказа в 1С по корреляции",
                TerminalEvidenceText:  "подтверждённая связка IDs обеих систем",
        },
        {
                Field:        "state_version_time",
                Title:        "Версия и время состояния",
                OwningSystem: MatrixOwnerABCP,
                ExternalIDText:        "каждое состояние заказа в ABCP несёт версию/время записи",
                VersionTimeText:       "новейшая версия состояния определяется reread-ом, не локальным кэшем",
                AllowedProjectionText: "Staffy показывает время состояния как прочитанное, никогда не экстраполированное",
                ConflictRuleText:      "более старая версия никогда не затирает более новую (monotonic)",
                ReconcileQueryText:    "сравнение версий/времён ABCP и 1С при сверке",
                TerminalEvidenceText:  "терминальное состояние несёт финальные версию и время",
        },
        {
                Field:        "staffy_projection",
                Title:        "Разрешённая проекция Staffy",
                OwningSystem: MatrixOwnerStaffyCase,
                ExternalIDText:        "проекция ссылается на внешние IDs, собственных внешних IDs не создаёт",
                VersionTimeText:       "проекция читается в момент показа; сохранённое состояние = кейс-контур 2-4",
                AllowedProjectionText: "разрешены только read-only поля матрицы; отправка поставщику в пилоте не выполняется",
                ConflictRuleText:      "проекция не разрешает конфликт: она показывает unknown честным словом",
                ReconcileQueryText:    "проекция инициирует ручную сверку, автоматически не исправляет",
                TerminalEvidenceText:  "проекция заявляет успех только при терминальном доказательстве поля",
        },
        {
                Field:        "conflict_rule",
                Title:        "Правило конфликта",
                OwningSystem: MatrixOwnerStaffyCase,
                ExternalIDText:        "правило объявлено контуром кейсов Staffy (compile-time словарь матрицы)",
                VersionTimeText:       "изменение правила = новая версия спеки матрицы",
                AllowedProjectionText: "правило показывает обработку расхождения: unknown + ручная сверка",
                ConflictRuleText:      "расхождение ABCP↔1С никогда не разрешается автоматически",
                ReconcileQueryText:    "правило требует сверку обеих сторон до снятия unknown",
                TerminalEvidenceText:  "снятие конфликта подтверждается человеком (квитанция сверки)",
        },
        {
                Field:        "reconcile_query",
                Title:        "Reconcile-запрос",
                OwningSystem: MatrixOwnerStaffyCase,
                ExternalIDText:        "запрос сверки адресуется внешним системам по их IDs",
                VersionTimeText:       "каждая сверка несёт время исполнения запроса",
                AllowedProjectionText: "сверка читает обе стороны; результат — состояние поля, не мутация",
                ConflictRuleText:      "неоднозначный результат сверки оставляет поле unknown",
                ReconcileQueryText:    "reread статуса ABCP + запрос состояния 1С по корреляции",
                TerminalEvidenceText:  "сверка завершается терминальным доказательством либо honest-unknown",
        },
        {
                Field:        "terminal_evidence",
                Title:        "Терминальное доказательство",
                OwningSystem: MatrixOwner1C,
                ExternalIDText:        "терминальный успех заказа подтверждается стороной 1С (финальное состояние)",
                VersionTimeText:       "доказательство несёт версию/время финального состояния",
                AllowedProjectionText: "без терминального доказательства успех не отображается",
                ConflictRuleText:      "частичный/неоднозначный результат = не терминальный, поле остаётся unknown",
                ReconcileQueryText:    "запрос финального состояния заказа в 1С",
                TerminalEvidenceText:  "терминальная квитанция 1С — единственное основание заявить успех",
        },
}

// ---------------------------------------------------------------------------
// Pinned honest words of the derivation
// ---------------------------------------------------------------------------

// pilotCapabilityStateWords are the pinned per-state honest words of an
// unproven capability (the DERIVED states of spec 1-9).
var pilotCapabilityStateWords = map[string]string{
        "unverified":  "capability не проверена живым evidence",
        "verifying":   "проверка выполняется — доказательства ещё нет",
        "unsupported": "декларация неполна — capability unsupported (fail-closed)",
        "failed":      "проверка не прошла — capability failed",
        "unknown":     "состояние неизвестно — успех не заявляется",
        "stale":       "доказательство устарело — требуется повторная проверка",
}

// pilotCapabilityMissingWord is the pinned word when no live fact exists.
const pilotCapabilityMissingWord = "подключения с живым доказательством нет"

// pilotConnectorNextSteps are the pinned next-step words per connector.
var pilotConnectorNextSteps = map[string]string{
        "jivo-chat":                  "реализация адаптера после OQ-2 PASS",
        "abcp-parts":                 "реализация адаптера после OQ-2/OQ-5 PASS",
        "staffy-synthetic-telephony": "провести read-only probe синтетического контура (verifier 1-9)",
}

// pilotGateWords carries the pinned honest composition words of every gate
// referenced by a pilot route: the close phrase (what exactly is not
// closed), the Russian safe state (AC-9 words) and the next owner of the
// check (the accountable owner of the registry canon). The blocker word is
// "<ID> <state>: <close>"; the safe state is carried beside it.
var pilotGateWords = map[string]struct {
        Close     string
        SafeState string
        NextOwner string
}{
        "OQ-1": {
                Close:     "провайдер не подтверждён фактическими данными аккаунта — Mango Office остаётся кандидатом",
                SafeState: "awaiting_verification/limited; неподдерживаемые transfer/caller-заявления запрещены",
                NextOwner: "Олег + Technical Lead",
        },
        "OQ-2": {
                Close:     "семантика Jivo не подтверждена аккаунтными тестами",
                SafeState: "production shared inbox human-only; Staffy-only требует изолированного writer-exclusion и проверенного takeover",
                NextOwner: "Олег + Integration Lead",
        },
        "OQ-3": {
                Close:     "ABCP prepared order не закрыт",
                SafeState: "нейтральный pending_manager_verification; ручной reread и человеческая квитанция",
                NextOwner: "Олег + ABCP integrator",
        },
        "OQ-4": {
                Close:     "сквозной Order-тест ABCP→1С не закрыт",
                SafeState: "без terminal success; расхождение/unknown = ручная сверка",
                NextOwner: "Олег + 1С integrator",
        },
        "OQ-5": {
                Close:     "применимость не подтверждена записанным workflow",
                SafeState: "без inferred fitment/UI-scraping; человеческая задача либо лицензированный переиспользуемый каталог-коннектор",
                NextOwner: "Олег + эксперт по подбору + Integration Lead",
        },
        "OQ-11": {
                Close:     "data-маршрут не подтверждён purpose-specific consent",
                SafeState: "synthetic/non-PII only",
                NextOwner: "Security/Privacy + Technical Lead",
        },
        "OQ-12": {
                Close:     "маршрутные voice-метрики не измерены",
                SafeState: "без числовых обещаний immediacy",
                NextOwner: "Voice/Technical Lead",
        },
        "OQ-13B": {
                Close:     "восстановимый drill не записан",
                SafeState: "synthetic or closed-account test only",
                NextOwner: "Technical Lead + Operations",
        },
        "OQ-19": {
                Close:     "материальные поля обмена не имеют терминальных доказательств",
                SafeState: "поле = unknown; без мутации и terminal success",
                NextOwner: "Architecture + владельцы Owning Systems",
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
                Close:     "runtime-зависимости не подтверждены",
                SafeState: "только локальная синтетическая реализация; без реальных credentials",
                NextOwner: "Technical Lead + Security",
        },
}

// pilotOQ11ScopeWord is the pinned honest word of the backend data-route
// scope boundary: the ABCP data route has no channel dimension in the
// channel dictionary, so no applicable OQ-11 instance can exist this slice.
const pilotOQ11ScopeWord = "data-маршрут ABCP не имеет канального scope — ABCP является backend API-маршрутом данных, словарь каналов измерения ABCP не содержит"

// pilotMatrixWord is the pinned honest word of the matrix-dependent routes.
const pilotMatrixWord = "материальные поля обмена ABCP→1С остаются unknown — терминальные доказательства отсутствуют (OQ-19)"

// ---------------------------------------------------------------------------
// The read-time derivation
// ---------------------------------------------------------------------------

// PilotFacts are the live facts of the pilot admission: the sealed pack
// pin, the derived capability states by catalog capability ID and the full
// gate instance history. The app layer assembles them from reused read
// ports (never secrets); the derivation is read-time only — a stored
// "computed" admission does not exist.
type PilotFacts struct {
        PackID       string
        PackVersion  string
        Capabilities map[string]CapabilityFact
        Instances    []GateInstance
}

// PilotBlocker is one honest blocking of a pilot route: the source (pack,
// capability, matrix or gate), the exact word, the safe state and the next
// owner of the check. A blocked route never hides behind an aggregate.
type PilotBlocker struct {
        Source    string `json:"source"`
        ID        string `json:"id"`
        Word      string `json:"word"`
        SafeState string `json:"safe_state,omitempty"`
        NextOwner string `json:"next_owner"`
}

// PilotCapabilityView is one required capability with its derived state.
type PilotCapabilityView struct {
        ID          string `json:"id"`
        ConnectorID string `json:"connector_id"`
        State       string `json:"state"`
        Word        string `json:"word,omitempty"`
}

// PilotGateView is one required gate with its derived state and words.
type PilotGateView struct {
        ID        string `json:"id"`
        State     string `json:"state"`
        Word      string `json:"word,omitempty"`
        SafeState string `json:"safe_state,omitempty"`
        NextOwner string `json:"next_owner"`
}

// PilotRouteAdmission is the derived admission of one pilot route: the
// stage, the honest stage word, the per-capability and per-gate requirement
// views, the blockers and the pinned fence/fallback words.
type PilotRouteAdmission struct {
        RouteID       string                 `json:"route_id"`
        Title         string                 `json:"title"`
        Stage         string                 `json:"stage"`
        StageWord     string                 `json:"stage_word"`
        Capabilities  []PilotCapabilityView  `json:"capabilities,omitempty"`
        Gates         []PilotGateView        `json:"gates,omitempty"`
        Blockers      []PilotBlocker         `json:"blockers,omitempty"`
        FenceWords    []string               `json:"fence_words,omitempty"`
        FallbackWords []string               `json:"fallback_words,omitempty"`
}

// PilotMatrixFieldView is one matrix field with its live state word.
type PilotMatrixFieldView struct {
        MatrixField
        StateWord string `json:"state_word"`
}

// PilotAdmissionReport is the whole read-time derivation: the pack pin
// state, every route in dictionary order and the ABCP→1C matrix with its
// live field states.
type PilotAdmissionReport struct {
        PackPinned bool                   `json:"pack_pinned"`
        PackWord   string                 `json:"pack_word"`
        Routes     []PilotRouteAdmission  `json:"routes"`
        Matrix     []PilotMatrixFieldView `json:"matrix"`
}

// PackVersionAtLeast compares dotted numeric versions (major.minor.patch)
// — the exported form consumed by the app composition (the highest pinned
// pack version of the tenant's sealed manifests).
func PackVersionAtLeast(version, minimum string) bool {
        parse := func(value string) [3]int {
                var out [3]int
                part := 0
                index := 0
                for _, symbol := range value {
                        if symbol == '.' {
                                if index < 2 {
                                        index++
                                        part = 0
                                        continue
                                }
                                break
                        }
                        if symbol < '0' || symbol > '9' {
                                return [3]int{-1, -1, -1}
                        }
                        part = part*10 + int(symbol-'0')
                        out[index] = part
                }
                return out
        }
        have, want := parse(version), parse(minimum)
        for position := 0; position < 3; position++ {
                if have[position] != want[position] {
                        return have[position] > want[position]
                }
        }
        return true
}
// validPassForTenant resolves the acting PASS of a tenant-level definition:
// an unsuperseded record in the exact pass state carrying no route and no
// modality scope. Missing, open, in_test, fail, expired, superseded and
// not_applicable never are.
func validPassForTenant(definitionID string, instances []GateInstance) *GateInstance {
        superseded := SupersededSet(instances)
        for index := range instances {
                instance := instances[index]
                if instance.DefinitionID != definitionID || instance.State != GateStatePass {
                        continue
                }
                if superseded[instance.InstanceID] {
                        continue
                }
                if instance.Route != nil || instance.Modality != "" {
                        continue
                }
                return &instance
        }
        return nil
}

// instanceStateForTenant names the honest current state of a tenant-level
// definition: the newest unsuperseded tenant-level record's state, or
// "missing" when no such record exists.
func instanceStateForTenant(definitionID string, instances []GateInstance) (string, string) {
        superseded := SupersededSet(instances)
        var latest *GateInstance
        for index := range instances {
                instance := instances[index]
                if instance.DefinitionID != definitionID || superseded[instance.InstanceID] {
                        continue
                }
                if instance.Route != nil || instance.Modality != "" {
                        continue
                }
                if latest == nil || instance.RecordedAt.After(latest.RecordedAt) || (instance.RecordedAt == latest.RecordedAt && instance.InstanceID.String() > latest.InstanceID.String()) {
                        latest = &instances[index]
                }
        }
        if latest == nil {
                return "missing", "применимого инстанса нет"
        }
        return latest.State, fmt.Sprintf("инстанс %s (tenant, записан %s)", latest.InstanceID, latest.RecordedAt.Format("02.01.2006"))
}

// DerivePilotAdmission is the pure read-time derivation of the pilot
// admission: the pack pin, every route's capability ladder, matrix state
// and gate conjunction, with the pinned honest words. Nothing is cached; a
// stale capability fact, a superseded PASS or a rotated secret reflects at
// the next read immediately. The function performs no writes and holds no
// state — an unchanged world always yields the byte-stable same answer.
func DerivePilotAdmission(facts PilotFacts) PilotAdmissionReport {
        report := PilotAdmissionReport{Routes: []PilotRouteAdmission{}, Matrix: []PilotMatrixFieldView{}}
        // (1) The pack pin: the fail-closed prerequisite of every route.
        packOK := facts.PackID == PilotRequiredPackID && PackVersionAtLeast(facts.PackVersion, PilotRequiredPackMinVersion)
        report.PackPinned = packOK
        if packOK {
                report.PackWord = fmt.Sprintf("Domain Pack «Автозапчасти» прикреплён к запечатанному контуру: %s@%s", facts.PackID, facts.PackVersion)
        } else {
                report.PackWord = PilotPackMissingWord
        }
        // The matrix: every material field with its live state — this slice has
        // no terminal-evidence source, so every field honestly stays unknown.
        for _, field := range abcpTo1CMatrix {
                report.Matrix = append(report.Matrix, PilotMatrixFieldView{MatrixField: field, StateWord: MatrixUnknownWord})
        }
        // (2–4) Every route of the fixed dictionary, independently: a block of
        // one route never extends to another route (AC-12).
        for _, route := range pilotRoutes {
                admission := PilotRouteAdmission{
                        RouteID: route.ID, Title: route.Title,
                        FenceWords: route.FenceWords, FallbackWords: route.FallbackWords,
                }
                capabilityOK := true
                if !packOK {
                        admission.Blockers = append(admission.Blockers, PilotBlocker{
                                Source: "pack", ID: PilotRequiredPackID, Word: PilotPackMissingWord,
                                NextOwner: "Owner/Configurator — запечатать манифест с пином Domain Pack «Автозапчасти» версии 2.0.0 или новее",
                        })
                }
                // (2) The capability ladder: every required capability must be
                // proven by live evidence (verified, no gap). Connected alone is
                // never a proof; an adapter not implemented caps the route at
                // candidate with the honest adapter word.
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
                        stateWord := pilotCapabilityMissingWord
                        if exists {
                                stateWord = pilotCapabilityStateWords[fact.State]
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
                                NextOwner: pilotConnectorNextSteps[requirement.ConnectorID],
                        })
                }
                // The matrix ladder: a matrix-dependent route is proven by its
                // material fields' terminal evidence — which does not exist this
                // slice, so the route never rises above candidate (unknown stays
                // unknown; a vacuous capability list is never an admission).
                if route.MatrixDependent {
                        capabilityOK = false
                        admission.Blockers = append(admission.Blockers, PilotBlocker{
                                Source: "matrix", ID: "abcp_1c_matrix", Word: pilotMatrixWord,
                                SafeState: "поле = unknown; без мутации и terminal success",
                                NextOwner: "Architecture + владельцы Owning Systems (OQ-19)",
                        })
                }
                // (3) The gate conjunction: every required gate must carry an
                // acting PASS in the exact scope — tenant-level gates at tenant
                // level, the route-scoped OQ-11 per declared channel route. An
                // open, missing, in_test, fail or expired instance blocks with the
                // exact gate word, the safe state and the next owner.
                gatesAllPass := true
                for _, gateID := range route.RequiredGates {
                        word, known := pilotGateWords[gateID]
                        if !known {
                                // An unworded gate is a dictionary integrity break — the
                                // mirror test pins this away; the derivation refuses to
                                // invent words.
                                gatesAllPass = false
                                admission.Gates = append(admission.Gates, PilotGateView{ID: gateID, State: "missing", Word: "слово гейта отсутствует в словаре пилота — целостность словаря нарушена"})
                                continue
                        }
                        if gateID == "OQ-11" {
                                if len(route.RequiredOQ11Routes) == 0 {
                                        // The backend data route: no channel dimension exists,
                                        // no applicable instance can exist this slice.
                                        gatesAllPass = false
                                        view := PilotGateView{
                                                ID: gateID, State: "missing",
                                                Word:      "OQ-11 missing: " + pilotOQ11ScopeWord,
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
                                                gatesAllPass = false
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
                                gatesAllPass = false
                                view.State = state
                                view.Word = gateID + " " + state + ": " + word.Close
                                admission.Blockers = append(admission.Blockers, PilotBlocker{
                                        Source: "gate", ID: gateID, Word: view.Word + " — " + reason,
                                        SafeState: view.SafeState, NextOwner: word.NextOwner,
                                })
                        }
                        admission.Gates = append(admission.Gates, view)
                }
                // (4) The stage: the minimum of the contributions — candidate while
                // the pack pin, any capability or the matrix is unproven; verified
                // when every capability is proven but gates stay open; admitted
                // only by the full conjunction.
                switch {
                case capabilityOK && packOK && gatesAllPass:
                        admission.Stage = PilotStageAdmitted
                case capabilityOK && packOK:
                        admission.Stage = PilotStageVerified
                default:
                        admission.Stage = PilotStageCandidate
                }
                admission.StageWord = pilotStageWords[admission.Stage]
                report.Routes = append(report.Routes, admission)
        }
        return report
}

// PilotCapabilityIDs lists the required capability IDs of one route word.
func PilotCapabilityIDs(routeID string) []string {
        route, ok := PilotRouteByID(routeID)
        if !ok {
                return nil
        }
        ids := make([]string, 0, len(route.RequiredCapabilities))
        for _, requirement := range route.RequiredCapabilities {
                ids = append(ids, requirement.ID)
        }
        return ids
}

// PilotRouteGateIDs lists the required gate IDs of one route word.
func PilotRouteGateIDs(routeID string) []string {
        route, ok := PilotRouteByID(routeID)
        if !ok {
                return nil
        }
        return append([]string(nil), route.RequiredGates...)
}
