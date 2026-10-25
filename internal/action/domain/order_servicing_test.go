package domain_test

import (
        "reflect"
        "strings"
        "testing"
        "time"

        "github.com/artemiosu/staffy/internal/action/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
)

// Story 4.10 (step-01) — the order servicing domain tests: the record
// guards (the 8-intent dictionary, the 2-kind dictionary with the
// OQ-16 phone refusal, the typed value bound), the lookup outcome
// derivation BY KEY (the unique match, the honest not_found, the
// ambiguous without the list — the enumeration does not exist), the
// CancelFeasibility four states over the LIVE 4-9 projection (the
// supplier touch BY NAME), the client words derivation (the DENIED
// gate word WITHOUT the lookup outcome — the existence leak probe
// AC-1; the ALLOWED retell of the live projection words + the intent
// boundary word), the gate word render with the LIVE methods and the
// honest probes of the slice (the zero supplier-send words, the zero
// notification fields, the zero price fields, the OQ-16/OQ-18
// boundaries, the no-delivery word).

func servicingFixtureIDs() (id.UUID, id.UUID, id.UUID, id.UUID) {
        return id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4c001"),
                id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4c002"),
                id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4c003"),
                id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4c004")
}

func servicingNow() time.Time {
        return time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
}

func servicingRequest(intent, kind, value, proofState string) domain.OrderServicingRequest {
        tenantID, caseID, _, operationID := servicingFixtureIDs()
        return domain.OrderServicingRequest{
                TenantID:            tenantID,
                CaseID:              caseID,
                RequestNo:           1,
                Intent:              intent,
                IdentifierKind:      kind,
                IdentifierValue:     value,
                ResolvedOperationID: operationID,
                LookupOutcome:       domain.LookupOutcomeUnique,
                ProofState:          proofState,
                ProofMethod:         "known_number_callback",
                ProofIssuedAt:       servicingNow().Add(-10 * time.Minute),
                ProofExpiresAt:      servicingNow().Add(50 * time.Minute),
                Disclosure:          domain.DisclosureAllowed,
                ActorID:             id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4c005"),
                RecordedAt:          servicingNow(),
        }
}

// TestServicingDictionaries pins the compile-time dictionaries of the
// slice: the EIGHT intents (BY NAME — the order is the form order),
// the TWO identifier kinds (the ABCP client order ID and the supplier
// order ID — DIFFERENT TYPES of the external identifiers), the THREE
// lookup outcomes (the enumeration does not exist), the FOUR proof
// states (the 2-3 mirror plus the step-up window) and the TWO
// disclosure outcomes. Every word of every dictionary must exist (the
// honest render never falls back to a code).
func TestServicingDictionaries(t *testing.T) {
        wantIntents := []string{
                "order_status", "clarification", "change", "cancel",
                "return", "claim", "dispute", "human",
        }
        if got := domain.ServicingIntents; !reflect.DeepEqual(got, wantIntents) {
                t.Fatalf("словарь интентов: %v", got)
        }
        for _, intent := range domain.ServicingIntents {
                if word, ok := domain.ServicingIntentWords[intent]; !ok || word == "" {
                        t.Fatalf("интент %s без слова-границы", intent)
                }
                if short, ok := domain.ServicingIntentShortWords[intent]; !ok || short == "" {
                        t.Fatalf("интент %s без короткого слова формы", intent)
                }
        }
        wantKinds := []string{"abcp_client_order_id", "supplier_order_id"}
        if got := domain.LookupIdentifierKinds; !reflect.DeepEqual(got, wantKinds) {
                t.Fatalf("словарь типов признака: %v", got)
        }
        for _, kind := range domain.LookupIdentifierKinds {
                if word, ok := domain.LookupKindSourceWords[kind]; !ok || word == "" {
                        t.Fatalf("тип признака %s без слова источника", kind)
                }
        }
        wantOutcomes := []string{"unique_match", "not_found", "ambiguous"}
        if got := domain.LookupOutcomes; !reflect.DeepEqual(got, wantOutcomes) {
                t.Fatalf("словарь исходов поиска: %v", got)
        }
        for _, outcome := range domain.LookupOutcomes {
                if word, ok := domain.LookupOutcomeWords[outcome]; !ok || word == "" {
                        t.Fatalf("исход поиска %s без слова", outcome)
                }
        }
        wantProof := []string{"none", "basic", "verified", "step_up"}
        if got := domain.ProofStates; !reflect.DeepEqual(got, wantProof) {
                t.Fatalf("словарь proof-состояний: %v", got)
        }
        wantDisclosure := []string{"allowed", "denied"}
        if got := domain.DisclosureOutcomes; !reflect.DeepEqual(got, wantDisclosure) {
                t.Fatalf("словарь исходов раскрытия: %v", got)
        }
        // The disclosure gate: verified ONLY (AC-1).
        if !domain.ProofStateAllowsDisclosure(domain.ProofStateVerified) {
                t.Fatal("verified обязан открывать раскрытие")
        }
        for _, state := range []string{domain.ProofStateNone, domain.ProofStateBasic, domain.ProofStateStepUp} {
                if domain.ProofStateAllowsDisclosure(state) {
                        t.Fatalf("proof-состояние %s не открывает раскрытие (AC-1)", state)
                }
        }
}

// TestGuardOrderServicingRequestByName pins the honest refusals of the
// record command: the out-of-dictionary intent, the out-of-dictionary
// kind (the phone/email kinds do NOT exist — OQ-16), the empty and the
// overlong values. The guard NEVER reads the proof state — the request
// records at ANY trust level (AC-6).
func TestGuardOrderServicingRequestByName(t *testing.T) {
        cases := []struct {
                name              string
                intent, kind, val string
                rule              string
        }{
                {"интент вне словаря", "urgency", domain.LookupKindABCPClientOrderID, "ABCP-100", "order_servicing.intent"},
                {"тип вне словаря", domain.ServicingIntentOrderStatus, "phone_number", "+7-999-...", "order_servicing.identifier_kind"},
                {"тип email не существует", domain.ServicingIntentOrderStatus, "email", "a@b.c", "order_servicing.identifier_kind"},
                {"пустое значение", domain.ServicingIntentCancel, domain.LookupKindABCPClientOrderID, "   ", "order_servicing.identifier_value"},
                {"значение длиннее 200", domain.ServicingIntentHuman, domain.LookupKindSupplierOrderID, strings.Repeat("x", 201), "order_servicing.identifier_value"},
        }
        for _, testCase := range cases {
                t.Run(testCase.name, func(t *testing.T) {
                        err := domain.GuardOrderServicingRequest(testCase.intent, testCase.kind, testCase.val)
                        ruleErr, ok := err.(domain.RuleError)
                        if !ok {
                                t.Fatalf("ожидалась RuleError, получено %v", err)
                        }
                        if ruleErr.Rule != testCase.rule {
                                t.Fatalf("правило отказа %s, а не %s", ruleErr.Rule, testCase.rule)
                        }
                })
        }
        // The happy path of every intent and kind records at ANY trust
        // state (the guard carries no proof material — AC-6).
        for _, intent := range domain.ServicingIntents {
                for _, kind := range domain.LookupIdentifierKinds {
                        if err := domain.GuardOrderServicingRequest(intent, kind, "ABCP-ORDER-99"); err != nil {
                                t.Fatalf("легальная команда %s/%s отказана: %v", intent, kind, err)
                        }
                }
        }
}

// TestDeriveLookupOutcomeByName pins the three lookup outcomes BY KEY:
// the empty candidate list answers not_found (the generic word — never
// a disclosure of another client's orders); the ONE operation answers
// unique_match with the anchor; the TWO operations answer ambiguous
// with NO anchor (the enumeration does not exist). The SAME operation
// matched by SEVERAL sources stays the unique match — the sources are
// the pins, not the rows.
func TestDeriveLookupOutcomeByName(t *testing.T) {
        _, _, _, operationA := servicingFixtureIDs()
        operationB := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4c006")
        if outcome, resolved := domain.DeriveLookupOutcome(nil); outcome != domain.LookupOutcomeNone || resolved != id.UUID("") {
                t.Fatalf("пустой поиск: %s %v", outcome, resolved)
        }
        one := []domain.OrderLookupCandidate{{OperationID: operationA, Sources: []string{"system_receipt"}}}
        outcome, resolved := domain.DeriveLookupOutcome(one)
        if outcome != domain.LookupOutcomeUnique || resolved != operationA {
                t.Fatalf("единственный матч: %s %v", outcome, resolved)
        }
        twoSources := []domain.OrderLookupCandidate{
                {OperationID: operationA, Sources: []string{"system_receipt"}},
                {OperationID: operationA, Sources: []string{"manager_record"}},
        }
        outcome, resolved = domain.DeriveLookupOutcome(twoSources)
        if outcome != domain.LookupOutcomeUnique || resolved != operationA {
                t.Fatalf("одна операция с двумя источниками обязана остаться unique_match: %s %v", outcome, resolved)
        }
        two := append(twoSources, domain.OrderLookupCandidate{OperationID: operationB, Sources: []string{"manager_record"}})
        outcome, resolved = domain.DeriveLookupOutcome(two)
        if outcome != domain.LookupOutcomeMany || resolved != id.UUID("") {
                t.Fatalf("две операции: %s %v (якорь обязан быть пуст)", outcome, resolved)
        }
        // The outcome words: the not_found and ambiguous words never
        // enumerate; the unique word names the single match.
        if !strings.Contains(domain.LookupOutcomeWords[domain.LookupOutcomeNone], "не найден") {
                t.Fatalf("слово not_found: %s", domain.LookupOutcomeWords[domain.LookupOutcomeNone])
        }
        if !strings.Contains(domain.LookupOutcomeWords[domain.LookupOutcomeMany], "нескольким заказам") {
                t.Fatalf("слово ambiguous: %s", domain.LookupOutcomeWords[domain.LookupOutcomeMany])
        }
}

// TestDeriveCancelFeasibilityFourStates pins the FOUR read-time states
// of the cancel ask over the LIVE 4-9 projection (AC-7): the recorded
// request, the absent prepared action (the OQ-3 fence with the
// deferred-work pin), the absent confirmation (never promised) and the
// fourth state BY NAME — the supplier touch (the formed/sent facts of
// 4-9 → «затронул поставщика — человеку»; no facts → «не определена»).
func TestDeriveCancelFeasibilityFourStates(t *testing.T) {
        _, _, _, operationID := servicingFixtureIDs()
        base := domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{Found: true, OperationID: operationID, Receipt: &domain.OrderStatusReceiptPin{
                        ReceiptVersionNo: 1,
                        ExternalOrderID:  "ABCP-ORDER-99",
                        CreatedAt:        servicingNow().Add(-3 * time.Hour),
                }},
        }
        noFacts := domain.DeriveOrderStatusProjection(base, servicingNow())
        view := domain.DeriveCancelFeasibility(noFacts)
        if view.RequestRecordedWord != domain.ServicingCancelRequestRecordedWord {
                t.Fatalf("первое состояние: %s", view.RequestRecordedWord)
        }
        if !strings.Contains(view.PreparedActionWord, "cancel отсутствует в каталоге") || !strings.Contains(view.PreparedActionWord, "OQ-3") {
                t.Fatalf("второе состояние без пина каталога: %s", view.PreparedActionWord)
        }
        if !strings.Contains(view.ConfirmedWord, "квитанции отмены нет") || !strings.Contains(view.ConfirmedWord, "не обещается") {
                t.Fatalf("третье состояние без честного отрицания: %s", view.ConfirmedWord)
        }
        if view.SupplierTouched {
                t.Fatal("фактов поставщика нет — затронутость обязана быть false")
        }
        if !strings.Contains(view.NotEditableWord, "не определена") {
                t.Fatalf("четвёртое состояние без фактов: %s", view.NotEditableWord)
        }
        if !strings.Contains(view.ManualContourWord, "НЕ считается отменённым") {
                t.Fatalf("слово ручного контура: %s", view.ManualContourWord)
        }
        // The supplier-touch fourth state: the formed fact flips the word.
        withFacts := base
        withFacts.Facts = []domain.OrderSupplierFact{{
                TenantID:        id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4c001"),
                CaseID:          id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4c002"),
                WorkItemID:      id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4c003"),
                FactNo:          1,
                OperationID:     operationID,
                Kind:            domain.OrderSupplierFactFormed,
                SupplierOrderID: "SUP-77",
                HappenedAt:      servicingNow().Add(-time.Hour),
                ActorID:         id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4c005"),
                RecordedAt:      servicingNow().Add(-50 * time.Minute),
        }}
        touchedProjection := domain.DeriveOrderStatusProjection(withFacts, servicingNow())
        view = domain.DeriveCancelFeasibility(touchedProjection)
        if !view.SupplierTouched {
                t.Fatal("факт формирования записан — затронутость обязана быть true")
        }
        if !strings.Contains(view.NotEditableWord, "затронул поставщика") || !strings.Contains(view.NotEditableWord, "передаётся человеку") {
                t.Fatalf("четвёртое состояние с фактами: %s", view.NotEditableWord)
        }
}

// TestDeriveServicingClientWordsDeniedGate pins the AC-1 leak probe: a
// DENIED request answers the gate word with the LIVE allowed methods
// and the fallback words ONLY — the lookup outcome, the identifier
// value and ANY order material NEVER ride the denied client words (the
// very existence of the order is not disclosed).
func TestDeriveServicingClientWordsDeniedGate(t *testing.T) {
        _, _, _, operationID := servicingFixtureIDs()
        for _, state := range []string{domain.ProofStateNone, domain.ProofStateBasic, domain.ProofStateStepUp} {
                request := servicingRequest(domain.ServicingIntentOrderStatus, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-99", state)
                request.Disclosure = domain.DisclosureDenied
                request.LookupOutcome = domain.LookupOutcomeNone
                request.ResolvedOperationID = id.UUID("")
                projection := domain.DeriveOrderStatusProjection(domain.OrderStatusMaterial{
                        Operation: domain.OrderStatusOperationMaterial{Found: true, OperationID: operationID},
                }, servicingNow())
                fallback := []string{"Повторите проверку разрешённым способом.", "Продолжите обслуживание без защищённых данных.", "Передайте разговор человеку."}
                words := domain.DeriveServicingClientWords(request, &projection, []string{"перезвонок на известный номер", "ответ на письмо с известного адреса"}, fallback)
                if len(words) == 0 || !strings.Contains(words[0], "требует проверки личности") {
                        t.Fatalf("proof=%s: первое слово обязано быть словом гейта: %v", state, words)
                }
                if !strings.Contains(words[0], "перезвонок на известный номер") {
                        t.Fatalf("proof=%s: слово гейта без живых методов: %s", state, words[0])
                }
                joined := strings.Join(words, " ")
                if strings.Contains(joined, domain.LookupOutcomeWords[domain.LookupOutcomeNone]) ||
                        strings.Contains(joined, domain.LookupOutcomeWords[domain.LookupOutcomeUnique]) ||
                        strings.Contains(joined, domain.LookupOutcomeWords[domain.LookupOutcomeMany]) {
                        t.Fatalf("proof=%s: исход поиска утёк в клиентские слова отказа (AC-1): %v", state, words)
                }
                if strings.Contains(joined, "ABCP-ORDER-99") {
                        t.Fatalf("proof=%s: значение признака утекло в клиентские слова отказа: %v", state, words)
                }
                if strings.Contains(joined, "заказ клиента создан") {
                        t.Fatalf("proof=%s: материал заказа утёк в клиентские слова отказа: %v", state, words)
                }
        }
}

// TestDeriveServicingClientWordsAllowed pins the ALLOWED retell: the
// client words of the LIVE 4-9 projection of the resolved operation
// ride FIRST (the confirmed stages, the freshness, the boundary
// words), the intent boundary word rides LAST (AC-5 — only the
// confirmed material of the Order Status Projection).
func TestDeriveServicingClientWordsAllowed(t *testing.T) {
        _, _, _, operationID := servicingFixtureIDs()
        projection := domain.DeriveOrderStatusProjection(domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{Found: true, OperationID: operationID, Receipt: &domain.OrderStatusReceiptPin{
                        ReceiptVersionNo: 1,
                        ExternalOrderID:  "ABCP-ORDER-99",
                        CreatedAt:        servicingNow().Add(-3 * time.Hour),
                }},
        }, servicingNow())
        request := servicingRequest(domain.ServicingIntentOrderStatus, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-99", domain.ProofStateVerified)
        words := domain.DeriveServicingClientWords(request, &projection, nil, nil)
        if len(words) < len(projection.ClientWords)+1 {
                t.Fatalf("слова раскрытия короче живой проекции + слово интента: %v", words)
        }
        for i, projectionWord := range projection.ClientWords {
                if words[i] != projectionWord {
                        t.Fatalf("слово %d расходится с проекцией: %s ≠ %s", i, words[i], projectionWord)
                }
        }
        if words[len(words)-1] != domain.ServicingIntentWords[domain.ServicingIntentOrderStatus] {
                t.Fatalf("последнее слово обязано быть словом интента: %s", words[len(words)-1])
        }
        // The change intent rides its manual-contour word (AG-4 — never
        // a claim that the order changed).
        changeRequest := servicingRequest(domain.ServicingIntentChange, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-99", domain.ProofStateVerified)
        changeWords := domain.DeriveServicingClientWords(changeRequest, &projection, nil, nil)
        if !strings.Contains(changeWords[len(changeWords)-1], "не утверждает, что заказ изменён") {
                t.Fatalf("слово изменения: %s", changeWords[len(changeWords)-1])
        }
        // The nil projection (the anchor is not readable anymore) answers
        // the intent word only — the honest empty retell, never a panic.
        emptyWords := domain.DeriveServicingClientWords(request, nil, nil, nil)
        if len(emptyWords) != 1 || emptyWords[0] != domain.ServicingIntentWords[domain.ServicingIntentOrderStatus] {
                t.Fatalf("пустая проекция: %v", emptyWords)
        }
        // The ALLOWED not_found/ambiguous requests answer the GENERIC outcome
        // word + the intent word (safe by construction — never a disclosure
        // of another client's orders).
        for _, outcome := range []string{domain.LookupOutcomeNone, domain.LookupOutcomeMany} {
                missRequest := servicingRequest(domain.ServicingIntentOrderStatus, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-99", domain.ProofStateVerified)
                missRequest.LookupOutcome = outcome
                missRequest.ResolvedOperationID = id.UUID("")
                missWords := domain.DeriveServicingClientWords(missRequest, nil, nil, nil)
                if len(missWords) != 2 || missWords[0] != domain.LookupOutcomeWords[outcome] {
                        t.Fatalf("исход %s: %v", outcome, missWords)
                }
                if missWords[1] != domain.ServicingIntentWords[domain.ServicingIntentOrderStatus] {
                        t.Fatalf("исход %s: последнее слово обязано быть словом интента: %s", outcome, missWords[1])
                }
        }
}

// TestServicingGateWordRender pins the LIVE render of the gate word:
// the allowed methods of the contact case conversation ride the word
// (never a domain constant); the empty list answers the honest
// no-methods word (never an invented list, never «ok»).
func TestServicingGateWordRender(t *testing.T) {
        word := domain.ServicingGateWord([]string{"перезвонок на известный номер", "ответ на письмо с известного адреса"})
        if !strings.Contains(word, domain.ServicingGateWordPrefix) || !strings.Contains(word, "перезвонок") {
                t.Fatalf("слово гейта с методами: %s", word)
        }
        empty := domain.ServicingGateWord(nil)
        if empty != domain.ServicingGateNoMethodsWord {
                t.Fatalf("слово гейта без методов: %s", empty)
        }
}

// TestServicingHonestProbes pins the structural honesty of the slice:
// the dictionaries carry NO send words (the OQ-3 fence — the request
// records, it never performs), NO price material, NO notification
// material; the boundary words ride (the OQ-16 phone refusal, the
// AG-4/OQ-18 connector boundary, the OQ-1/OQ-2 no-delivery word); the
// record command word never claims a result (AC-6).
func TestServicingHonestProbes(t *testing.T) {
        for _, intent := range domain.ServicingIntents {
                if strings.Contains(intent, "send") || strings.Contains(intent, "ship") {
                        t.Fatalf("интент %s выглядит словом отправки — запросы записывают реальность, не выполняют её", intent)
                }
                if strings.Contains(intent, "notify") {
                        t.Fatalf("интент %s выглядит словом уведомления — доставки нет (OQ-1/OQ-2)", intent)
                }
        }
        for _, kind := range domain.LookupIdentifierKinds {
                if strings.Contains(kind, "phone") || strings.Contains(kind, "email") || strings.Contains(kind, "msisdn") {
                        t.Fatalf("тип признака %s не существует — OQ-16: сквозной профиль не строится", kind)
                }
        }
        // The lookup candidate carries the identifier match only — no
        // price, no send, no notification fields (the reflect probe).
        candidateType := reflect.TypeOf(domain.OrderLookupCandidate{})
        for i := 0; i < candidateType.NumField(); i++ {
                name := candidateType.Field(i).Name
                for _, forbidden := range []string{"price", "cost", "amount", "notify", "notification", "send"} {
                        if strings.Contains(strings.ToLower(name), forbidden) {
                                t.Fatalf("поле %s кандидата поиска — материал за границей среза", name)
                        }
                }
        }
        // The request carries the proof pins and the anchor only — no
        // price, no send, no notification fields.
        requestType := reflect.TypeOf(domain.OrderServicingRequest{})
        for i := 0; i < requestType.NumField(); i++ {
                name := requestType.Field(i).Name
                for _, forbidden := range []string{"price", "cost", "amount", "notify", "notification", "send"} {
                        if strings.Contains(strings.ToLower(name), forbidden) {
                                t.Fatalf("поле %s запроса — материал за границей среза", name)
                        }
                }
        }
        // The boundary words ride the dictionaries (the OQ-16/OQ-18
        // probes of the honest refusals).
        if !strings.Contains(domain.ServicingPhoneSearchBoundaryWord, "OQ-16") {
                t.Fatalf("слово границы поиска по телефону: %s", domain.ServicingPhoneSearchBoundaryWord)
        }
        if !strings.Contains(domain.ServicingConnectorBoundaryWord, "AG-4") {
                t.Fatalf("слово границы коннектора: %s", domain.ServicingConnectorBoundaryWord)
        }
        if !strings.Contains(domain.ServicingNoDeliveryWord, "не отправляются") {
                t.Fatalf("слово отсутствия доставки: %s", domain.ServicingNoDeliveryWord)
        }
        if !strings.Contains(domain.ServicingRequestRecordedWord, "не утверждает результат") {
                t.Fatalf("слово записи запроса: %s", domain.ServicingRequestRecordedWord)
        }
}

// TestServicingWordsByteStability pins the frozen words of the slice
// byte-exact (the drift of a word is a conscious dictionary review,
// never an accident).
func TestServicingWordsByteStability(t *testing.T) {
        pins := map[string]string{
                domain.ServicingLookupNotFoundWord:         "заказ по названному признаку не найден: проверьте номер с клиентом или передайте менеджеру",
                domain.ServicingLookupAmbiguousWord:        "признак соответствует нескольким заказам: уточнение у менеджера",
                domain.ServicingCancelNoPreparedActionWord: "подготовленное действие не существует: cancel отсутствует в каталоге действий (OQ-3 fence; декларация 4-1 — отмена подготовки отдельная команда до отправки, которой нет — deferred-work №215)",
                domain.ServicingCancelNoConfirmationWord:   "подтверждённая отмена не существует: квитанции отмены нет — не подтверждено; отмена не обещается до Action Receipt или человеческого подтверждения Owning System",
                domain.ServicingCancelSupplierTouchedWord:  "заказ затронул поставщика (записи формирования/отправки есть) — отмена НЕ выполняется как обычное редактирование: передаётся человеку",
                domain.ServicingCancelManualWord:           "отмену выполняет менеджер вручную (AG-4): до человеческого подтверждения заказ НЕ считается отменённым",
                domain.ServicingGateWordPrefix:             "раскрытие требует проверки личности (уровень verified); разрешённые методы канала: ",
                domain.ServicingIntentChangeWord:           "запрос зафиксирован; изменение заказа менеджер выполняет в ABCP вручную (AG-4); Staffy не утверждает, что заказ изменён, до человеческого подтверждения; коррекция подбора до закупки — контур 4-8",
        }
        for word, want := range pins {
                if word != want {
                        t.Fatalf("замороженное слово изменилось:\nполучено: %s\nожидалось: %s", word, want)
                }
        }
}
