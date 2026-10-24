package domain

import (
        "crypto/sha256"
        "encoding/hex"
        "encoding/json"
        "fmt"
        "sort"
        "strings"
        "time"
)

// ---------------------------------------------------------------------------
// Story 4.5: the supplier offer domain (the manager's recorded commercial
// conditions of the actually connected suppliers as the append-only
// material + the read-time comparison derivation with the digest pins and
// the honest words + the client selection snapshot)
// ---------------------------------------------------------------------------
//
// The honesty core of the slice: the supplier offer is RECORDED MATERIAL,
// never a derivation and never an automatic request (the parts_catalog_abcp
// route is not admitted by the 4.1 registry — OQ-5/AG-4 open; the producer
// is the operator-manager who reads the actual interface of the connected
// suppliers and records the conditions with the FULL traceability of 2.7:
// the source, the source record identifier, the version and the time). The
// client price is the RECORDED permitted ABCP price — the model never
// computes a discount, a markup or a total (AC-3); the only arithmetic of
// the slice is the known client sum = quantity × recorded price, always
// with the word that the final cost is NOT computed (AC-4). The purchase
// price is the internal data of the store: the record owns it, the CLIENT
// material never does (AC-2 — structurally: the client view family carries
// no purchase price field). The availability of an offer NEVER changes the
// catalog fitment status (AC-1 — DeriveFitment of 4.4 stays untouched, the
// comparison READS its output). The term is the supplier's ESTIMATE — never
// the store's promise and never a confirmed fulfillment fact (AC-8). The
// position is recommended ONLY with the proven fitment and the explained
// confirmed criterion (AC-6/AC-7); the «best» word without a defined basis
// is never spoken; the marketing claims of quality, resource or superiority
// are never created (AC-5). The order is never split between the suppliers
// and the quantity is never silently changed (AC-11). The Selection
// Snapshot pins the EXACT versions of ALL FOUR inputs (AC-13) and creates
// NO ABCP Order and sends NOTHING to the supplier (the fence OQ-3/OQ-4 —
// the order contour of 4.6). The state is the READ-TIME ladder over the
// append-only records: no records → unknown (the old price never repeats
// as the current one — AC-12); the record age over the pack TTL or the pin
// mismatch with the CURRENT digests of the vehicle profile (4.2), the part
// request (4.3) or the fitment proof (4.4) → stale (AC-9/AC-10 — a new
// version of ANY input expires EVERY dependent offer); the live record of
// every supplier is the one with the maximal offer number (the supersede
// is the NEW number, never an update). Nothing is silently fixed: the
// guards refuse with the honest words of the dictionary (WorkRuleError —
// the transport maps it onto the 400 problem).

// ---------------------------------------------------------------------------
// The dictionaries (the pack material words, mirrored by the narrow port —
// the domain never imports the port)
// ---------------------------------------------------------------------------

// The offer state words of the read-time ladder (the enum mirror for the
// panels and the contracts — the state is a DERIVATION, never a stored
// word).
const (
        OfferStatusCurrent = "current"
        OfferStatusStale   = "stale"
        OfferStatusUnknown = "unknown"
)

// The currency word of the auto-parts@6.0.0 dictionary (the enum mirror of
// the port answer — the word is pinned by the pack).
const OfferCurrencyRUB = "RUB"

// The term kind word of the dictionary: the SINGLE kind of this slice —
// the supplier's estimate (never the store's promise, never a confirmed
// fulfillment fact — AC-8).
const OfferTermKindSupplierEstimate = "supplier_estimate"

// The catalog source words of the offer records (the mirror of the 4.4
// fitment source dictionary — the same two admitted catalog systems).
const (
        OfferSourceAbcpCatalog   = FitmentSourceAbcpCatalog
        OfferSourceLaximoCatalog = FitmentSourceLaximoCatalog
)

// The rank criterion words of the auto-parts@6.0.0 dictionary (the enum
// mirror of the port answer — the words are pinned by the pack; the
// confirmed request material of 4.3 selects the primary criterion).
const (
        OfferRankBudget = "budget"
        OfferRankBrand  = "brand"
        OfferRankTerm   = "term"
)

// The tie breaker words of the auto-parts@6.0.0 dictionary (the enum mirror
// of the port answer — the deterministic total order after the primary
// criterion).
const (
        OfferTiePrice    = "price"
        OfferTieTerm     = "term"
        OfferTieBrand    = "brand"
        OfferTieSupplier = "supplier"
)

// OfferResearchWord is the manager task reason word of the offer research
// recommendation — the PUBLIC dictionary constant of 2.13 (the Story 4.5
// extension, the SQL mirror 00024 — the switch onto the public HandoffReason
// constant follows the 4.4 precedent of the fitment research word).
const OfferResearchWord = HandoffReasonOffersResearch

// The honest words (the text, never only color).
const (
        // OfferUnknownWord is the honest word of the absent offers (AC-12: the
        // old price never repeats as the current one).
        OfferUnknownWord = "актуальные коммерческие условия неизвестны: предложений нет; старая цена не повторяется как действующая"

        // OfferStalePositionWord is the honest word of the position whose every
        // current offer is expired (AC-9/AC-12: the re-request of the price,
        // the availability and the term; the stale records stay the numbered
        // material, never the current conditions).
        OfferStalePositionWord = "актуальные коммерческие условия неизвестны: предложения устарели — цена, наличие и срок повторно запрашиваются; старая цена не повторяется как действующая"

        // OfferStaleAgeWord is the honest word of the over-TTL offer (AC-9).
        OfferStaleAgeWord = "предложение устарело: возраст превышает допустимый"

        // OfferStaleProfileWord is the honest word of the profile pin mismatch
        // (AC-10: a new version of the vehicle profile expires the dependent
        // offer).
        OfferStaleProfileWord = "предложение относилось к прежней версии профиля автомобиля"

        // OfferStalePartRequestWord is the honest word of the request pin
        // mismatch (AC-10: a new version of the part request expires the
        // dependent offer).
        OfferStalePartRequestWord = "предложение относилось к прежней версии запроса"

        // OfferStaleFitmentWord is the honest word of the fitment pin mismatch
        // (AC-10: a new version of the fitment proof expires the dependent
        // offer).
        OfferStaleFitmentWord = "предложение относилось к прежней версии доказательства применимости"

        // OfferCurrentWord is the honest word of the live recorded offers.
        OfferCurrentWord = "актуальные предложения записаны: сравнение выведено по подтверждённому материалу"

        // OfferChangeWordFormat is the honest word of the changed conditions
        // between the consecutive records of one supplier (AC-10: the change
        // demands a new exact confirmation version; the aspects list is the
        // honest rendering of the actually changed fields).
        OfferChangeWordFormat = "условия изменились: %s обновлены — требуется новая точная версия подтверждения"

        // OfferPriceAuthorityWord is the honest price word (AC-3: the client
        // price is the recorded permitted ABCP price — the model computes
        // nothing).
        OfferPriceAuthorityWord = "цена по разрешению ABCP — рассчитана Owning System, НЕ моделью; скидка/наценка/итог системой не рассчитываются"

        // OfferSumBoundaryWord is the honest sum word (AC-4: the known sum is
        // the arithmetic of the quantity and the recorded price — the final
        // cost is NOT computed).
        OfferSumBoundaryWord = "без доставки и сбора — окончательная стоимость не рассчитана: известная стоимость отделена от ещё не рассчитанных составляющих"

        // OfferTermEstimateWord is the honest term word (AC-8: the estimate,
        // never the promise, never the fulfillment fact).
        OfferTermEstimateWord = "оценка поставщика — не обещание магазина и не подтверждённый факт исполнения"

        // OfferNoTermWord is the honest word of the absent term (the offer
        // without a term is never rendered as the fastest one).
        OfferNoTermWord = "срок не указан поставщиком"

        // OfferPurchaseInternalWord is the honest word of the purchase price
        // (AC-2: the internal data of the store — the client material never
        // carries it).
        OfferPurchaseInternalWord = "внутренние данные: клиенту не раскрывается"

        // OfferRankBudgetWord is the explanation word of the budget criterion
        // (AC-6: the variant corresponds to the stated priority).
        OfferRankBudgetWord = "вариант соответствует указанному приоритету: цена в пределах бюджета"

        // OfferRankBrandWord is the explanation word of the brand criterion
        // (AC-6).
        OfferRankBrandWord = "вариант соответствует указанному приоритету: бренд входит в указанные"

        // OfferRankTermWord is the explanation word of the term criterion
        // (AC-6).
        OfferRankTermWord = "вариант соответствует указанному приоритету: срок в пределах заявленного"

        // OfferNoCriteriaQuestion is the honest question of the absent
        // criterion (AC-7: the short clarifying question — never the «best»
        // word without a defined basis).
        OfferNoCriteriaQuestion = "укажите приоритет выбора: бюджет, бренд либо срок"

        // OfferLimitedSetWordFormat is the honest word of the limited variant
        // set (the pack MaxVisibleVariants bound of the no-criteria case —
        // never a silent truncation).
        OfferLimitedSetWordFormat = "показан ограниченный набор из %d вариантов (всего %d) — укажите приоритет либо запросите полное сравнение в разрешённом канале"

        // OfferShortfallWordFormat is the honest shortfall word (AC-11: the
        // supplier has less than requested — the split and the silent quantity
        // change never happen).
        OfferShortfallWordFormat = "у этого поставщика меньше запрошенного: %d из %d"

        // OfferOrderBoundaryWord is the honest order boundary word (AC-13 +
        // the fence OQ-3/OQ-4: the ABCP Order contour of 4.6).
        OfferOrderBoundaryWord = "ABCP Order не создаётся; поставщику не отправляется — заказ строится контуром 4-6 после менеджерской проверки"

        // OfferScopeWord is the honest scope word of the recorded conditions
        // (the record is scoped to ONE case with the pins of the exact input
        // versions — never a universal rule).
        OfferScopeWord = "коммерческие условия принадлежат одному обращению: записи привязаны к элементу работы кейса с пинами точных версий входов — правил для других обращений не создаётся"

        // OfferVoiceFullWord is the closing word of the phone block (AC-14: the
        // full comparison travels in the permitted channel).
        OfferVoiceFullWord = "полное сравнение отправим в разрешённый текстовый либо защищённый канал"

        // OfferVoiceRecommendedWordFormat is the recommended variant word of
        // the phone block (AC-14 + AC-7: the recommendation lives ONLY on the
        // confirmed criterion, never the «best» word).
        OfferVoiceRecommendedWordFormat = "по подтверждённому приоритету «%s» рекомендуемый вариант: %s, поставщик %s, цена %.0f %s"

        // OfferVoicePriceWordFormat is the key difference word of the price
        // (AC-14: the differences by the confirmed criteria only).
        OfferVoicePriceWordFormat = "цена: от %.0f до %.0f %s"

        // OfferVoiceBrandsWordFormat is the key difference word of the brands.
        OfferVoiceBrandsWordFormat = "бренды предложений: %s"

        // OfferVoiceTermWordFormat is the key difference word of the terms.
        OfferVoiceTermWordFormat = "срок: от %d до %d дней (оценка поставщика)"

        // OfferVoiceNoTermWord is the key difference word of the absent terms.
        OfferVoiceNoTermWord = "срок не указан поставщиками"

        // OfferSelectionGuardFitmentWord is the honest refusal of the
        // non-proven position selection (AC-5/AC-7).
        OfferSelectionGuardFitmentWord = "применимость не подтверждена: выбор подтверждённого варианта либо передача человеку"

        // OfferSelectionGuardStaleWord is the honest refusal of the expired
        // offer selection (AC-9/AC-10: the re-request happens BEFORE the
        // record).
        OfferSelectionGuardStaleWord = "предложение устарело: повторный запрос актуальных условий — выбор не записывается"

        // OfferSelectionGuardQuantityWord is the honest refusal of the
        // over-availability quantity (AC-11: the split and the silent change
        // never happen).
        OfferSelectionGuardQuantityWord = "количество превышает наличие поставщика: разбие́ние заказа между поставщиками и изменение количества не выполняются — выберите альтернативное предложение либо подтвердите иное количество"

        // OfferSelectionGuardQuantityMinWord is the honest refusal of the zero
        // quantity (AC-13: the snapshot fixes the chosen quantity).
        OfferSelectionGuardQuantityMinWord = "количество выбора не меньше единицы: снапшот фиксирует выбранные позиции и количество"
)

// OfferStatusWords is the honest word of every state word (the panel and
// the contract mirror the dictionary; the drift is the red contract test).
var OfferStatusWords = map[string]string{
        OfferStatusCurrent: OfferCurrentWord,
        OfferStatusStale:   OfferStalePositionWord,
        OfferStatusUnknown: OfferUnknownWord,
}

// ---------------------------------------------------------------------------
// The policy domain form (the pack material through the app mapping)
// ---------------------------------------------------------------------------

// ComparisonPolicyDomain is the domain form of the offer comparison policy
// of the pinned pack (the app maps the narrow port answer onto it — the
// domain never imports the port; the words are pinned by the pack).
type ComparisonPolicyDomain struct {
        Currencies         []string          `json:"currencies"`
        MaxOfferAgeSeconds int               `json:"max_offer_age_seconds"`
        RankCriteria       []string          `json:"rank_criteria"`
        RankCriteriaLabels map[string]string `json:"rank_criteria_labels"`
        TieBreakers        []string          `json:"tie_breakers"`
        MaxVisibleVariants int               `json:"max_visible_variants"`
}

// currencyAllowed answers whether the currency word belongs to the pack
// dictionary.
func (p ComparisonPolicyDomain) currencyAllowed(currency string) bool {
        return inDictionary(p.Currencies, currency)
}

// criterionAllowed answers whether the rank criterion word belongs to the
// pack dictionary.
func (p ComparisonPolicyDomain) criterionAllowed(criterion string) bool {
        return inDictionary(p.RankCriteria, criterion)
}

// ---------------------------------------------------------------------------
// The offer draft (the write command material) and the guard
// ---------------------------------------------------------------------------

// SupplierOfferDraft is the material of ONE supplier offer record the
// manager records (the write command of the app — the producer is the
// operator-manager; the automatic supplier request is never performed). The
// client price is the RECORDED permitted ABCP price (mandatory — AC-2/AC-3);
// the purchase price is the OPTIONAL internal data of the store (the client
// material never carries it — the nil is the honest «не видел / не записал»,
// never a zero); the term is the supplier's estimate in days (the single
// term kind of the dictionary); the traceability fields follow the 2.7
// discipline; the ProfileDigest and the PartRequestDigest pins are
// mandatory, the FitmentDigest pin is optional (the offer without the proof
// is recorded honestly but never recommended).
type SupplierOfferDraft struct {
        PositionKey          string
        SupplierID           string
        SupplierName         string
        Brand                string
        Article              string
        ClientPrice          float64
        PurchasePrice        *float64
        Currency             string
        QuantityAvailable    int
        TermDays             *int
        Conditions           string
        SourceSystem         string
        SourceRecordID       string
        SourceVersion        string
        SourceObservedAt     time.Time
        ProfileDigestPin     string
        PartRequestDigestPin string
        FitmentDigestPin     string
}

// GuardSupplierOffer refuses the dishonest record BEFORE any SQL with the
// honest dictionary words (the WorkRuleError family — the transport maps it
// onto the 400 problem; NOTHING is silently fixed). The guard walks the
// pack dictionaries first (the currencies and the sources — the Domain
// Policy of the pack, never the store code), then the canonical position
// key (the 4.3 number discipline), then the mandatory material fields of
// the found offer (the supplier, the brand, the article, the client price,
// the availability, the non-negative estimate term), then the
// traceability of 2.7 (the source record identifier, the version, the
// observed time) and BOTH mandatory digest pins (the optional fitment pin
// is never invented — AC-15: the offer without the source material is
// refused BEFORE any SQL).
func GuardSupplierOffer(policy ComparisonPolicyDomain, draft SupplierOfferDraft) error {
        if strings.TrimSpace(draft.PositionKey) == "" {
                return WorkRuleError{Rule: "offer_position_key_required", Reason: "ключ позиции обязателен: предложение привязывается к позиции материала кейса"}
        }
        if !policy.currencyAllowed(draft.Currency) {
                return WorkRuleError{Rule: "offer_currency_out_of_dictionary", Reason: "валюта вне словаря пака: разрешены " + joinQuoted(policy.Currencies) + " — Domain Policy пака"}
        }
        if !sourceAllowedForOffer(policy, draft.SourceSystem) {
                return WorkRuleError{Rule: "offer_source_out_of_dictionary", Reason: "источник вне словаря пака: разрешены " + joinQuoted(offerSourceDictionary(policy)) + " — Domain Policy пака"}
        }
        // The canonical position key: the normalized rendering of the article
        // (the 4.3 number discipline — the upper case, the dropped spaces, the
        // unified separators); the caller normalizes BEFORE the command, the
        // guard refuses the non-canonical key honestly (never a silent fix).
        if NormalizePartNumber(draft.PositionKey).Normalized != draft.PositionKey {
                return WorkRuleError{Rule: "offer_position_key_not_canonical", Reason: "канонизируйте ключ позиции: нормализованное представление артикула без пробелов, в верхнем регистре, с унифицированными разделителями"}
        }
        if strings.TrimSpace(draft.SupplierID) == "" {
                return WorkRuleError{Rule: "offer_supplier_required", Reason: "идентификатор поставщика обязателен: предложение несёт фактического поставщика"}
        }
        if strings.TrimSpace(draft.Brand) == "" {
                return WorkRuleError{Rule: "offer_brand_required", Reason: "бренд обязателен: сравнение показывает бренд как значимое различие"}
        }
        if strings.TrimSpace(draft.Article) == "" {
                return WorkRuleError{Rule: "offer_article_required", Reason: "артикул обязателен: предложение несёт артикул позиции"}
        }
        if draft.ClientPrice < 0 {
                return WorkRuleError{Rule: "offer_client_price_negative", Reason: "клиентская цена по разрешению ABCP не может быть отрицательной — модель не рассчитывает цену, цена записывается"}
        }
        if draft.PurchasePrice != nil && *draft.PurchasePrice < 0 {
                return WorkRuleError{Rule: "offer_purchase_price_negative", Reason: "закупочная цена (опциональные внутренние данные) не может быть отрицательной"}
        }
        if draft.QuantityAvailable < 0 {
                return WorkRuleError{Rule: "offer_quantity_negative", Reason: "количество у поставщика не может быть отрицательным"}
        }
        if draft.TermDays != nil && *draft.TermDays < 0 {
                return WorkRuleError{Rule: "offer_term_negative", Reason: "срок оценкой поставщика не может быть отрицательным"}
        }
        if strings.TrimSpace(draft.SourceRecordID) == "" {
                return WorkRuleError{Rule: "offer_source_record_id_required", Reason: "идентификатор записи источника обязателен — трассируемость до ответа Owning System (AC-15)"}
        }
        if strings.TrimSpace(draft.SourceVersion) == "" {
                return WorkRuleError{Rule: "offer_source_version_required", Reason: "версия источника обязательна — трассируемость до ответа Owning System"}
        }
        if draft.SourceObservedAt.IsZero() {
                return WorkRuleError{Rule: "offer_source_observed_at_required", Reason: "время получения материала источника обязательно — трассируемость до ответа Owning System"}
        }
        if draft.ProfileDigestPin == "" {
                return WorkRuleError{Rule: "offer_profile_digest_pin_required", Reason: "пин ProfileDigest обязателен: предложение обязано пиннить точную версию профиля автомобиля"}
        }
        if draft.PartRequestDigestPin == "" {
                return WorkRuleError{Rule: "offer_part_request_digest_pin_required", Reason: "пин PartRequestDigest обязателен: предложение обязано пиннить точную версию запроса детали"}
        }
        return nil
}

// offerSourceDictionary renders the admitted source words of the offer
// records: the same two catalog systems as the fitment sources of 4.4 (the
// honest mirror of the pack dictionary — the offer records never invent a
// third source system).
func offerSourceDictionary(policy ComparisonPolicyDomain) []string {
        return []string{FitmentSourceAbcpCatalog, FitmentSourceLaximoCatalog}
}

// sourceAllowedForOffer answers whether the source word belongs to the
// offer source dictionary (the 4.4 mirror).
func sourceAllowedForOffer(policy ComparisonPolicyDomain, source string) bool {
        return inDictionary(offerSourceDictionary(policy), source)
}

// ---------------------------------------------------------------------------
// The read-time comparison derivation (AC-5..AC-14)
// ---------------------------------------------------------------------------

// SupplierOfferInput is ONE append-only offer record handed to the
// derivation (the store material — the record of the manager's commercial
// conditions research with the traceability fields and the digest pins).
type SupplierOfferInput struct {
        OfferNo              int64
        PositionKey          string
        SupplierID           string
        SupplierName         string
        Brand                string
        Article              string
        ClientPrice          float64
        PurchasePrice        *float64
        Currency             string
        QuantityAvailable    int
        TermDays             *int
        Conditions           string
        SourceSystem         string
        SourceRecordID       string
        SourceVersion        string
        SourceObservedAt     time.Time
        ReceivedAt           time.Time
        ProfileDigestPin     string
        PartRequestDigestPin string
        FitmentDigestPin     string
}

// ComparisonCriteria is the CONFIRMED request material of the ranking
// (AC-6: the primary criterion comes from the confirmed facts of the part
// request of 4.3 — never from the unconfirmed ones; the app extracts the
// confirmed attributes, the domain never re-reads the facts). The brands
// list is the parsed confirmed brand set; the requested quantity is the
// confirmed quantity material of the request (0 = not confirmed — the
// shortfall word never appears on the unconfirmed material).
type ComparisonCriteria struct {
        BudgetConfirmed bool
        BrandsConfirmed bool
        Brands          []string
        TermConfirmed   bool
        RequestedQty    int
}

// ComparisonPositionInput is ONE position with its append-only offer
// records and the read-time fitment answer of 4.4 (the link type and the
// status of the current proof — the comparison READS the output of 4.4,
// never re-derives it; the empty link type is the position without a
// proof record).
type ComparisonPositionInput struct {
        PositionKey       string
        LinkType          string
        FitmentStatus     string
        FitmentStatusWord string
        FitmentDigest     string
        Offers            []SupplierOfferInput
}

// ComparisonDerivationInput is the whole material of the read-time
// comparison derivation of one case: the pack policy, the positions with
// their append-only offer records (ordered by the offer number), the
// CURRENT digests of the vehicle profile (4.2), the part request (4.3) and
// the per-position fitment proofs (4.4), the CONFIRMED criteria of the
// request material and the clock now.
type ComparisonDerivationInput struct {
        Policy             ComparisonPolicyDomain
        Positions          []ComparisonPositionInput
        ProfileDigest      string
        PartRequestDigest  string
        Criteria           ComparisonCriteria
        Now                time.Time
}

// OfferSourceView is the traceability material of ONE recorded offer (the
// 2.7 discipline — the source, the record identifier, the version and the
// times; the panel shows it as the details without leaving the case).
type OfferSourceView struct {
        SourceSystem     string    `json:"source_system"`
        SourceRecordID   string    `json:"source_record_id"`
        SourceVersion    string    `json:"source_version"`
        SourceObservedAt time.Time `json:"source_observed_at"`
        ReceivedAt       time.Time `json:"received_at"`
}

// ComparisonVariantView is the CLIENT material of ONE ranked variant: the
// brand, the client price, the availability, the term (with the estimate
// word), the conditions, the rank with the explanation word, the status
// with the honest words and the OfferDigest (the exact version of the
// conditions — the pin of the contours 4.6..4.9). The purchase price is
// structurally ABSENT (AC-2 — the client material never carries it; the
// operator panel renders the internal section from the record view).
type ComparisonVariantView struct {
        SupplierID        string           `json:"supplier_id"`
        SupplierName      string           `json:"supplier_name"`
        Brand             string           `json:"brand"`
        Article           string           `json:"article"`
        OfferNo           int64            `json:"offer_no"`
        ClientPrice       float64          `json:"client_price"`
        Currency          string           `json:"currency"`
        QuantityAvailable int              `json:"quantity_available"`
        TermDays          *int             `json:"term_days,omitempty"`
        TermWord          string           `json:"term_word"`
        Conditions        string           `json:"conditions,omitempty"`
        Rank              int              `json:"rank"`
        RankWord          string           `json:"rank_word,omitempty"`
        Status            string           `json:"status"`
        StaleWords        []string         `json:"stale_words"`
        ChangeWords       []string         `json:"change_words"`
        ShortfallWord     string           `json:"shortfall_word,omitempty"`
        OfferDigest       string           `json:"offer_digest"`
        Source            OfferSourceView  `json:"source"`
}

// ComparisonPositionView is the read-time comparison answer of ONE
// position: the link type group word (AC-5 — the grouping by the link type
// and the confirmed fitment), the fitment status with its honest word (the
// 4.4 output, re-shown — the availability NEVER changes it, AC-1), the
// offer state word, the honest words, the no-criteria question, the ranked
// variants and the handoff recommendation words (the 2.13 dictionary).
type ComparisonPositionView struct {
        PositionKey       string                  `json:"position_key"`
        LinkType          string                  `json:"link_type"`
        LinkTypeWord      string                  `json:"link_type_word"`
        FitmentStatus     string                  `json:"fitment_status"`
        FitmentStatusWord string                  `json:"fitment_status_word"`
        Status            string                  `json:"status"`
        StatusWord        string                  `json:"status_word"`
        Words             []string                `json:"words"`
        Question          string                  `json:"question,omitempty"`
        Variants          []ComparisonVariantView `json:"variants"`
        HandoffWords      []string                `json:"handoff_words"`
}

// ComparisonVoiceBlock is the short word of the phone (AC-14): the key
// differences (up to 3, by the CONFIRMED criteria only), the recommended
// variant word (ONLY on the confirmed criterion — never the «best» word
// without a basis, AC-7) and the full comparison word of the permitted
// channel.
type ComparisonVoiceBlock struct {
        KeyDifferences  []string `json:"key_differences"`
        RecommendedWord string   `json:"recommended_word,omitempty"`
        FullWord        string   `json:"full_word"`
}

// ComparisonView is the whole read-time comparison answer of the case: the
// positions with their variant groups, the case-level unknown word when NO
// offer exists at all (the honest «условия неизвестны» + the research
// recommendation — AC-12), the price authority word (AC-3), the term
// estimate word (AC-8), the scope word and the phone block (AC-14).
type ComparisonView struct {
        Positions        []ComparisonPositionView `json:"positions"`
        CaseUnknownWord  string                   `json:"case_unknown_word,omitempty"`
        CaseHandoffWords []string                 `json:"case_handoff_words,omitempty"`
        PriceAuthorityWord string                 `json:"price_authority_word"`
        TermEstimateWord string                   `json:"term_estimate_word"`
        ScopeWord        string                   `json:"scope_word"`
        VoiceBlock       ComparisonVoiceBlock     `json:"voice_block"`
}

// DeriveComparison derives the read-time comparison of the case over the
// append-only offer records (READ-TIME: zero writes, idempotent — the JSON
// of the same world is byte-stable). The ladder per position: the CURRENT
// record of every supplier is the one with the maximal offer number (the
// supersede is the NEW number, never an update); NO records at all → the
// case-level unknown with the research recommendation (AC-12); every
// current record is expired (the age over the pack TTL or the pin mismatch
// with the CURRENT digests of the profile/request/proof) → stale with the
// honest words (AC-9/AC-10); the live records → the ranked variants: the
// primary criterion from the CONFIRMED request material (AC-6), the
// deterministic tie breakers of the pack, the explained recommendation
// (never the «best» word without a basis — AC-7), the shortfall word of
// the partial availability (AC-11) and the change words of the consecutive
// records of one supplier (AC-10). The grouping follows the link type and
// the confirmed fitment of 4.4 (AC-5); the phone block carries the key
// differences and the recommended variant (AC-14).
func DeriveComparison(input ComparisonDerivationInput) ComparisonView {
        view := ComparisonView{
                Positions:          []ComparisonPositionView{},
                PriceAuthorityWord: OfferPriceAuthorityWord,
                TermEstimateWord:   OfferTermEstimateWord,
                ScopeWord:          OfferScopeWord,
                VoiceBlock:         ComparisonVoiceBlock{KeyDifferences: []string{}, FullWord: OfferVoiceFullWord},
        }
        offers := 0
        for _, position := range input.Positions {
                offers += len(position.Offers)
        }
        if offers == 0 {
                view.CaseUnknownWord = OfferUnknownWord
                view.CaseHandoffWords = []string{OfferResearchWord}
                return view
        }
        criterion := confirmedCriterion(input.Criteria)
        for _, position := range input.Positions {
                view.Positions = append(view.Positions, deriveComparisonPosition(input, position, criterion))
        }
        view.VoiceBlock = deriveComparisonVoice(input, criterion)
        return view
}

// confirmedCriterion resolves the primary rank criterion of the confirmed
// request material (AC-6: the FIRST confirmed one of the pack dictionary
// order; the empty word is the honest no-criterion case — the question and
// the limited set, never the «best» word).
func confirmedCriterion(criteria ComparisonCriteria) string {
        if criteria.BudgetConfirmed {
                return OfferRankBudget
        }
        if criteria.BrandsConfirmed {
                return OfferRankBrand
        }
        if criteria.TermConfirmed {
                return OfferRankTerm
        }
        return ""
}

// deriveComparisonPosition derives the comparison of ONE position.
func deriveComparisonPosition(input ComparisonDerivationInput, position ComparisonPositionInput, criterion string) ComparisonPositionView {
        out := ComparisonPositionView{
                PositionKey:       position.PositionKey,
                LinkType:          position.LinkType,
                LinkTypeWord:      offerLinkTypeWord(position.LinkType),
                FitmentStatus:     position.FitmentStatus,
                FitmentStatusWord: position.FitmentStatusWord,
                Words:             []string{},
                Variants:          []ComparisonVariantView{},
                HandoffWords:      []string{},
        }
        if len(position.Offers) == 0 {
                out.Status = OfferStatusUnknown
                out.StatusWord = OfferUnknownWord
                out.HandoffWords = append(out.HandoffWords, OfferResearchWord)
                return out
        }
        // The current record of every supplier: the maximal offer number of
        // that supplier (the supersede discipline — the NEW number, never an
        // update; the store reads them ordered, the derivation re-elects the
        // current ones honestly).
        current := map[string]SupplierOfferInput{}
        for _, offer := range position.Offers {
                existing, ok := current[offer.SupplierID]
                if !ok || offer.OfferNo > existing.OfferNo {
                        current[offer.SupplierID] = offer
                }
        }
        currentList := make([]SupplierOfferInput, 0, len(current))
        for _, offer := range current {
                currentList = append(currentList, offer)
        }
        sort.SliceStable(currentList, func(i, j int) bool {
                return currentList[i].SupplierID < currentList[j].SupplierID
        })
        // The live current records (the expired ones render their stale words
        // below — the numbered material stays, never the current conditions,
        // AC-12).
        live := make([]SupplierOfferInput, 0, len(currentList))
        liveViews := make([]ComparisonVariantView, 0, len(currentList))
        expiredViews := make([]ComparisonVariantView, 0, len(currentList))
        for _, offer := range currentList {
                if expired, _ := isExpiredOffer(input, position, offer); !expired {
                        live = append(live, offer)
                }
        }
        if len(live) == 0 {
                out.Status = OfferStatusStale
                out.StatusWord = OfferStalePositionWord
                for _, offer := range currentList {
                        variant := comparisonVariantOf(input, position, offer)
                        variant.Status = OfferStatusStale
                        _, variant.StaleWords = isExpiredOffer(input, position, offer)
                        expiredViews = append(expiredViews, variant)
                }
                out.Variants = expiredViews
                out.HandoffWords = append(out.HandoffWords, OfferResearchWord)
                return out
        }
        out.Status = OfferStatusCurrent
        out.StatusWord = OfferCurrentWord
        // The deterministic ranking over the live variants: the primary
        // criterion of the confirmed material (AC-6) with the pack tie
        // breakers; the no-criterion case renders the honest question and the
        // limited set (AC-7).
        sort.SliceStable(live, func(i, j int) bool {
                return compareVariants(input.Policy, criterion, input.Criteria, live[i], live[j]) < 0
        })
        for index, offer := range live {
                variant := comparisonVariantOf(input, position, offer)
                if criterion != "" {
                        variant.Rank = index + 1
                        variant.RankWord = rankWordOf(criterion, input.Criteria, offer)
                }
                variant.ChangeWords = offerChangeWords(position, offer)
                variant.ShortfallWord = offerShortfallWord(input, offer)
                liveViews = append(liveViews, variant)
        }
        if criterion == "" {
                out.Question = OfferNoCriteriaQuestion
                if len(liveViews) > input.Policy.MaxVisibleVariants {
                        out.Words = append(out.Words, fmt.Sprintf(OfferLimitedSetWordFormat, input.Policy.MaxVisibleVariants, len(liveViews)))
                        liveViews = liveViews[:input.Policy.MaxVisibleVariants]
                }
        }
        // The expired numbered material of the position (the details with the
        // honest stale words — never the current conditions, AC-12).
        for _, offer := range currentList {
                if expired, _ := isExpiredOffer(input, position, offer); expired {
                        variant := comparisonVariantOf(input, position, offer)
                        variant.Status = OfferStatusStale
                        _, variant.StaleWords = isExpiredOffer(input, position, offer)
                        expiredViews = append(expiredViews, variant)
                }
        }
        // The rendered order: the live variants in the deterministic order
        // first (the ranked ones of the confirmed criterion — AC-6), the
        // expired numbered material after.
        out.Variants = append(liveViews, expiredViews...)
        return out
}

// comparisonVariantOf renders the client material of ONE offer record
// (the purchase price is structurally absent — AC-2).
func comparisonVariantOf(input ComparisonDerivationInput, position ComparisonPositionInput, offer SupplierOfferInput) ComparisonVariantView {
        termWord := OfferNoTermWord
        if offer.TermDays != nil {
                termWord = OfferTermEstimateWord
        }
        return ComparisonVariantView{
                SupplierID:        offer.SupplierID,
                SupplierName:      offer.SupplierName,
                Brand:             offer.Brand,
                Article:           offer.Article,
                OfferNo:           offer.OfferNo,
                ClientPrice:       offer.ClientPrice,
                Currency:          offer.Currency,
                QuantityAvailable: offer.QuantityAvailable,
                TermDays:          offer.TermDays,
                TermWord:          termWord,
                Conditions:        offer.Conditions,
                Status:            OfferStatusCurrent,
                StaleWords:        []string{},
                ChangeWords:       []string{},
                OfferDigest:       OfferDigestOf(offer),
                Source: OfferSourceView{
                        SourceSystem: offer.SourceSystem, SourceRecordID: offer.SourceRecordID, SourceVersion: offer.SourceVersion,
                        SourceObservedAt: offer.SourceObservedAt, ReceivedAt: offer.ReceivedAt,
                },
        }
}

// isExpiredOffer answers whether ONE current offer record is expired by
// the age or the digest pins (the AC-9/AC-10 read-time expiry — a new
// version of ANY input expires EVERY dependent offer) and renders the
// honest stale words of the actual causes.
func isExpiredOffer(input ComparisonDerivationInput, position ComparisonPositionInput, offer SupplierOfferInput) (bool, []string) {
        words := []string{}
        expired := false
        if age := input.Now.Sub(offer.ReceivedAt); age > time.Duration(input.Policy.MaxOfferAgeSeconds)*time.Second {
                expired = true
                words = append(words, OfferStaleAgeWord)
        }
        if offer.ProfileDigestPin != input.ProfileDigest {
                expired = true
                words = append(words, OfferStaleProfileWord)
        }
        if offer.PartRequestDigestPin != input.PartRequestDigest {
                expired = true
                words = append(words, OfferStalePartRequestWord)
        }
        if offer.FitmentDigestPin != "" && offer.FitmentDigestPin != position.FitmentDigest {
                expired = true
                words = append(words, OfferStaleFitmentWord)
        }
        return expired, words
}

// offerChangeWords renders the change words of the consecutive records of
// ONE supplier (AC-10: the new record supersedes by the NEW number; the
// changed aspects are named honestly — the price, the availability, the
// term).
func offerChangeWords(position ComparisonPositionInput, offer SupplierOfferInput) []string {
        var previous *SupplierOfferInput
        for index := range position.Offers {
                candidate := position.Offers[index]
                if candidate.SupplierID == offer.SupplierID && candidate.OfferNo < offer.OfferNo {
                        if previous == nil || candidate.OfferNo > previous.OfferNo {
                                previous = &position.Offers[index]
                        }
                }
        }
        if previous == nil {
                return nil
        }
        aspects := []string{}
        if previous.ClientPrice != offer.ClientPrice {
                aspects = append(aspects, "цена")
        }
        if previous.QuantityAvailable != offer.QuantityAvailable {
                aspects = append(aspects, "наличие")
        }
        if termChanged(previous.TermDays, offer.TermDays) {
                aspects = append(aspects, "срок")
        }
        if len(aspects) == 0 {
                return nil
        }
        return []string{fmt.Sprintf(OfferChangeWordFormat, strings.Join(aspects, ", "))}
}

// termChanged answers whether the term estimate changed between the
// consecutive records (the nil and the value are honestly different
// states).
func termChanged(before, after *int) bool {
        if before == nil && after == nil {
                return false
        }
        if before == nil || after == nil {
                return true
        }
        return *before != *after
}

// offerShortfallWord renders the honest shortfall word of the partial
// availability (AC-11 — only on the CONFIRMED requested quantity; the
// split and the silent change never happen).
func offerShortfallWord(input ComparisonDerivationInput, offer SupplierOfferInput) string {
        requested := input.Criteria.RequestedQty
        if requested <= 0 || offer.QuantityAvailable >= requested {
                return ""
        }
        return fmt.Sprintf(OfferShortfallWordFormat, offer.QuantityAvailable, requested)
}

// rankWordOf renders the explanation word of the confirmed criterion
// (AC-6: «почему вариант соответствует указанным приоритетам»); the empty
// word is the honest no-word of the non-listed brand or the no-criterion
// case (never a silent «лучший»).
func rankWordOf(criterion string, criteria ComparisonCriteria, offer SupplierOfferInput) string {
        switch criterion {
        case OfferRankBudget:
                return OfferRankBudgetWord
        case OfferRankBrand:
                if brandListed(criteria, offer.Brand) {
                        return OfferRankBrandWord
                }
                return ""
        case OfferRankTerm:
                return OfferRankTermWord
        }
        return ""
}

// brandListed answers whether the variant brand belongs to the confirmed
// brand set (the case-insensitive comparison of the trimmed words).
func brandListed(criteria ComparisonCriteria, brand string) bool {
        normalized := strings.ToUpper(strings.TrimSpace(brand))
        for _, listed := range criteria.Brands {
                if strings.ToUpper(strings.TrimSpace(listed)) == normalized {
                        return true
                }
        }
        return false
}

// compareVariants renders the deterministic total order of the live
// variants: the primary criterion of the confirmed material first (AC-6),
// then the pack tie breakers in the dictionary order (the hidden supplier
// benefit is NEVER the order basis — the purchase price is not a key,
// AC-5/AC-6).
func compareVariants(policy ComparisonPolicyDomain, criterion string, criteria ComparisonCriteria, a, b SupplierOfferInput) int {
        if criterion != "" {
                if result := compareByCriterion(criterion, criteria, a, b); result != 0 {
                        return result
                }
        }
        for _, tie := range policy.TieBreakers {
                if result := compareByTie(tie, a, b); result != 0 {
                        return result
                }
        }
        return 0
}

// compareByCriterion compares the live variants by the primary criterion:
// the budget → the client price ascending; the brands → the listed brands
// above the unlisted ones; the term → the estimate ascending (the absent
// term is the LAST, never the fastest — the honest word).
func compareByCriterion(criterion string, criteria ComparisonCriteria, a, b SupplierOfferInput) int {
        switch criterion {
        case OfferRankBudget:
                if a.ClientPrice != b.ClientPrice {
                        if a.ClientPrice < b.ClientPrice {
                                return -1
                        }
                        return 1
                }
        case OfferRankBrand:
                listedA, listedB := brandListed(criteria, a.Brand), brandListed(criteria, b.Brand)
                if listedA != listedB {
                        if listedA {
                                return -1
                        }
                        return 1
                }
        case OfferRankTerm:
                if result := compareTerms(a.TermDays, b.TermDays); result != 0 {
                        return result
                }
        }
        return 0
}

// compareTerms compares the term estimates (the nil is the last).
func compareTerms(a, b *int) int {
        if a == nil && b == nil {
                return 0
        }
        if a == nil {
                return 1
        }
        if b == nil {
                return -1
        }
        if *a != *b {
                if *a < *b {
                        return -1
                }
                return 1
        }
        return 0
}

// compareByTie compares the live variants by ONE tie breaker word of the
// pack dictionary: the price, the term, the brand (alphabetical) or the
// supplier (alphabetical).
func compareByTie(tie string, a, b SupplierOfferInput) int {
        switch tie {
        case OfferTiePrice:
                if a.ClientPrice != b.ClientPrice {
                        if a.ClientPrice < b.ClientPrice {
                                return -1
                        }
                        return 1
                }
        case OfferTieTerm:
                if result := compareTerms(a.TermDays, b.TermDays); result != 0 {
                        return result
                }
        case OfferTieBrand:
                if a.Brand != b.Brand {
                        if a.Brand < b.Brand {
                                return -1
                        }
                        return 1
                }
        case OfferTieSupplier:
                if a.SupplierID != b.SupplierID {
                        if a.SupplierID < b.SupplierID {
                                return -1
                        }
                        return 1
                }
        }
        return 0
}

// offerLinkTypeWord renders the group word of the link type (AC-5: the
// grouping by the link type and the confirmed fitment — «оригинал» /
// «аналог по cross-цепочке»; the empty word is the position without a
// proof record — the fitment status word speaks).
func offerLinkTypeWord(linkType string) string {
        switch linkType {
        case FitmentLinkOriginal:
                return "оригинал"
        case FitmentLinkCross:
                return "аналог по cross-цепочке"
        }
        return ""
}

// deriveComparisonVoice renders the phone block of the case (AC-14): the
// key differences of the CONFIRMED criteria only (up to 3), the
// recommended variant word ONLY on the confirmed criterion (never the
// «best» word without a basis — AC-7) and the full comparison word of the
// permitted channel.
func deriveComparisonVoice(input ComparisonDerivationInput, criterion string) ComparisonVoiceBlock {
        block := ComparisonVoiceBlock{KeyDifferences: []string{}, FullWord: OfferVoiceFullWord}
        live := liveOffersOf(input)
        if len(live) == 0 || criterion == "" {
                return block
        }
        switch criterion {
        case OfferRankBudget:
                min, max := live[0].ClientPrice, live[0].ClientPrice
                for _, offer := range live {
                        if offer.ClientPrice < min {
                                min = offer.ClientPrice
                        }
                        if offer.ClientPrice > max {
                                max = offer.ClientPrice
                        }
                }
                block.KeyDifferences = append(block.KeyDifferences, fmt.Sprintf(OfferVoicePriceWordFormat, min, max, live[0].Currency))
        case OfferRankBrand:
                brands := map[string]struct{}{}
                for _, offer := range live {
                        brands[strings.TrimSpace(offer.Brand)] = struct{}{}
                }
                list := make([]string, 0, len(brands))
                for brand := range brands {
                        list = append(list, brand)
                }
                sort.Strings(list)
                block.KeyDifferences = append(block.KeyDifferences, fmt.Sprintf(OfferVoiceBrandsWordFormat, strings.Join(list, ", ")))
        case OfferRankTerm:
                min, max, seen := 0, 0, false
                for _, offer := range live {
                        if offer.TermDays == nil {
                                continue
                        }
                        if !seen {
                                min, max, seen = *offer.TermDays, *offer.TermDays, true
                                continue
                        }
                        if *offer.TermDays < min {
                                min = *offer.TermDays
                        }
                        if *offer.TermDays > max {
                                max = *offer.TermDays
                        }
                }
                if seen {
                        block.KeyDifferences = append(block.KeyDifferences, fmt.Sprintf(OfferVoiceTermWordFormat, min, max))
                } else {
                        block.KeyDifferences = append(block.KeyDifferences, OfferVoiceNoTermWord)
                }
        }
        recommended := live[0]
        for _, offer := range live[1:] {
                if compareVariants(input.Policy, criterion, input.Criteria, offer, recommended) < 0 {
                        recommended = offer
                }
        }
        label := input.Policy.RankCriteriaLabels[criterion]
        if label == "" {
                label = criterion
        }
        block.RecommendedWord = fmt.Sprintf(OfferVoiceRecommendedWordFormat, label, recommended.Brand, recommended.SupplierName, recommended.ClientPrice, recommended.Currency)
        return block
}

// liveOffersOf collects the live current offers of every position (the
// expired ones are the numbered material, never the current conditions).
func liveOffersOf(input ComparisonDerivationInput) []SupplierOfferInput {
        live := []SupplierOfferInput{}
        for _, position := range input.Positions {
                current := map[string]SupplierOfferInput{}
                for _, offer := range position.Offers {
                        existing, ok := current[offer.SupplierID]
                        if !ok || offer.OfferNo > existing.OfferNo {
                                current[offer.SupplierID] = offer
                        }
                }
                currentList := make([]SupplierOfferInput, 0, len(current))
                for _, offer := range current {
                        currentList = append(currentList, offer)
                }
                sort.SliceStable(currentList, func(i, j int) bool {
                        return currentList[i].SupplierID < currentList[j].SupplierID
                })
                for _, offer := range currentList {
                        if expired, _ := isExpiredOffer(input, position, offer); !expired {
                                live = append(live, offer)
                        }
                }
        }
        return live
}

// ---------------------------------------------------------------------------
// The client selection snapshot (AC-13)
// ---------------------------------------------------------------------------

// ClientSelectionDraft is the material of ONE client selection record the
// manager records: the chosen position, the chosen CURRENT offer, the
// chosen quantity and the LIVE derivation answers of the moment (the
// read-time fitment status and digest of 4.4, the CURRENT profile and
// request digests, the clock now). The guard refuses the dishonest
// selection BEFORE any SQL; the assembled snapshot pins the EXACT versions
// of ALL FOUR inputs (AC-13).
type ClientSelectionDraft struct {
        PositionKey         string
        Offer               SupplierOfferInput
        Quantity            int
        FitmentStatus       string
        FitmentDigest       string
        ProfileDigest       string
        PartRequestDigest   string
        Now                 time.Time
}

// GuardClientSelection refuses the dishonest selection BEFORE any SQL with
// the honest dictionary words: the non-proven position (AC-5/AC-7 — the
// selection of the confirmed variant or the human transfer), the expired
// offer (AC-9/AC-10 — the pins mismatch the CURRENT digests or the age is
// over the pack TTL; the re-request happens BEFORE the record) and the
// quantity out of the availability bounds (AC-11 — the order split and the
// silent quantity change never happen; the quantity is never below one,
// AC-13).
func GuardClientSelection(policy ComparisonPolicyDomain, draft ClientSelectionDraft) error {
        if strings.TrimSpace(draft.PositionKey) == "" {
                return WorkRuleError{Rule: "selection_position_key_required", Reason: "ключ позиции обязателен: выбор привязывается к позиции материала кейса"}
        }
        if NormalizePartNumber(draft.PositionKey).Normalized != draft.PositionKey {
                return WorkRuleError{Rule: "selection_position_key_not_canonical", Reason: "канонизируйте ключ позиции: нормализованное представление артикула без пробелов, в верхнем регистре, с унифицированными разделителями"}
        }
        if draft.FitmentStatus != FitmentStatusProven {
                return WorkRuleError{Rule: "selection_fitment_not_proven", Reason: OfferSelectionGuardFitmentWord}
        }
        if draft.Offer.FitmentDigestPin != "" && draft.Offer.FitmentDigestPin != draft.FitmentDigest {
                return WorkRuleError{Rule: "selection_offer_stale", Reason: OfferSelectionGuardStaleWord}
        }
        if draft.Offer.ProfileDigestPin != draft.ProfileDigest {
                return WorkRuleError{Rule: "selection_offer_stale", Reason: OfferSelectionGuardStaleWord}
        }
        if draft.Offer.PartRequestDigestPin != draft.PartRequestDigest {
                return WorkRuleError{Rule: "selection_offer_stale", Reason: OfferSelectionGuardStaleWord}
        }
        if age := draft.Now.Sub(draft.Offer.ReceivedAt); age > time.Duration(policy.MaxOfferAgeSeconds)*time.Second {
                return WorkRuleError{Rule: "selection_offer_stale", Reason: OfferSelectionGuardStaleWord}
        }
        if draft.Quantity < 1 {
                return WorkRuleError{Rule: "selection_quantity_min", Reason: OfferSelectionGuardQuantityMinWord}
        }
        if draft.Quantity > draft.Offer.QuantityAvailable {
                return WorkRuleError{Rule: "selection_quantity_over_availability", Reason: OfferSelectionGuardQuantityWord}
        }
        return nil
}

// SelectionSnapshotMaterial is the assembled material of ONE selection
// snapshot record: the position, the supplier and the offer number of the
// chosen CURRENT offer, the chosen quantity, the unit client price (the
// recorded permitted ABCP price — never a computed one), the known client
// sum (the ONLY arithmetic of the slice: the quantity × the recorded
// price, AC-4), the pins of the EXACT versions of ALL FOUR inputs (AC-13)
// and the canonical SelectionDigest.
type SelectionSnapshotMaterial struct {
        PositionKey          string  `json:"position_key"`
        SupplierID           string  `json:"supplier_id"`
        OfferNo              int64   `json:"offer_no"`
        Quantity             int     `json:"quantity"`
        UnitClientPrice      float64 `json:"unit_client_price"`
        Currency             string  `json:"currency"`
        KnownClientSum       float64 `json:"known_client_sum"`
        ProfileDigestPin     string  `json:"profile_digest_pin"`
        PartRequestDigestPin string  `json:"part_request_digest_pin"`
        FitmentDigestPin     string  `json:"fitment_digest_pin"`
        OfferDigestPin       string  `json:"offer_digest_pin"`
        SelectionDigest      string  `json:"selection_digest"`
}

// AssembleClientSelection guards the draft and assembles the snapshot
// material (the known client sum is the ONLY arithmetic of the slice — the
// quantity × the recorded client price, always rendered with the boundary
// word by the panel; the SelectionDigest pins the exact versions of ALL
// FOUR inputs, AC-13).
func AssembleClientSelection(policy ComparisonPolicyDomain, draft ClientSelectionDraft) (SelectionSnapshotMaterial, error) {
        if err := GuardClientSelection(policy, draft); err != nil {
                return SelectionSnapshotMaterial{}, err
        }
        material := SelectionSnapshotMaterial{
                PositionKey:          draft.PositionKey,
                SupplierID:           draft.Offer.SupplierID,
                OfferNo:              draft.Offer.OfferNo,
                Quantity:             draft.Quantity,
                UnitClientPrice:      draft.Offer.ClientPrice,
                Currency:             draft.Offer.Currency,
                KnownClientSum:       float64(draft.Quantity) * draft.Offer.ClientPrice,
                ProfileDigestPin:     draft.ProfileDigest,
                PartRequestDigestPin: draft.PartRequestDigest,
                FitmentDigestPin:     draft.FitmentDigest,
                OfferDigestPin:       OfferDigestOf(draft.Offer),
        }
        material.SelectionDigest = SelectionDigestOf(material)
        return material, nil
}

// ---------------------------------------------------------------------------
// The digests (the exact versions of the conditions and the selection)
// ---------------------------------------------------------------------------

// OfferDigestOf renders the SHA-256 of the canonical JSON of ONE offer
// record — {position_key, supplier_id, offer_no, brand, article,
// client_price, currency, quantity_available, term_days, conditions,
// source, source_version, pins} with the sorted keys (the byte-stable
// canonical form of «the exact version of the conditions», the pin of the
// contours 4.6..4.9). The purchase price is NOT the digest material (the
// optional internal data never changes the client-relevant version of the
// conditions).
func OfferDigestOf(offer SupplierOfferInput) string {
        canonical := map[string]any{
                "position_key":            offer.PositionKey,
                "supplier_id":             offer.SupplierID,
                "offer_no":                offer.OfferNo,
                "brand":                   offer.Brand,
                "article":                 offer.Article,
                "client_price":            offer.ClientPrice,
                "currency":                offer.Currency,
                "quantity_available":      offer.QuantityAvailable,
                "term_days":               nilIfZero(offer.TermDays),
                "conditions":              offer.Conditions,
                "source":                  offer.SourceSystem,
                "source_version":          offer.SourceVersion,
                "profile_digest_pin":      offer.ProfileDigestPin,
                "part_request_digest_pin": offer.PartRequestDigestPin,
                "fitment_digest_pin":      offer.FitmentDigestPin,
        }
        return sha256Canonical(canonical)
}

// SelectionDigestOf renders the SHA-256 of the canonical JSON of ONE
// selection snapshot (the byte-stable «exact version of the choice» — the
// pin of the contours 4.6..4.9).
func SelectionDigestOf(material SelectionSnapshotMaterial) string {
        canonical := map[string]any{
                "position_key":            material.PositionKey,
                "supplier_id":             material.SupplierID,
                "offer_no":                material.OfferNo,
                "quantity":                material.Quantity,
                "unit_client_price":       material.UnitClientPrice,
                "currency":                material.Currency,
                "known_client_sum":        material.KnownClientSum,
                "profile_digest_pin":      material.ProfileDigestPin,
                "part_request_digest_pin": material.PartRequestDigestPin,
                "fitment_digest_pin":      material.FitmentDigestPin,
                "offer_digest_pin":        material.OfferDigestPin,
        }
        return sha256Canonical(canonical)
}

// nilIfZero renders the optional integer as the canonical nil (the absent
// term is the honest nil, never a zero).
func nilIfZero(value *int) any {
        if value == nil {
                return nil
        }
        return *value
}

// OfferView is the OPERATOR material of ONE recorded offer (the panel's
// internal section of the case page): the full record material WITH the
// purchase price and its internal-data word (AC-2 — the operator section
// renders it; the CLIENT material family — the comparison variants and
// the voice block — never carries it, structurally).
type OfferView struct {
	OfferNo           int64           `json:"offer_no"`
	PositionKey       string          `json:"position_key"`
	SupplierID        string          `json:"supplier_id"`
	SupplierName      string          `json:"supplier_name"`
	Brand             string          `json:"brand"`
	Article           string          `json:"article"`
	ClientPrice       float64         `json:"client_price"`
	PurchasePrice     *float64        `json:"purchase_price,omitempty"`
	PurchaseWord      string          `json:"purchase_word,omitempty"`
	Currency          string          `json:"currency"`
	QuantityAvailable int             `json:"quantity_available"`
	TermDays          *int            `json:"term_days,omitempty"`
	TermWord          string          `json:"term_word"`
	Conditions        string          `json:"conditions,omitempty"`
	Source            OfferSourceView `json:"source"`
	OfferDigest       string          `json:"offer_digest"`
}

// OfferViewOf renders the operator view of ONE offer record (the purchase
// word appears only when the internal data exists — the honest absent
// price never renders a word).
func OfferViewOf(offer SupplierOfferInput) OfferView {
	termWord := OfferNoTermWord
	if offer.TermDays != nil {
		termWord = OfferTermEstimateWord
	}
	purchaseWord := ""
	if offer.PurchasePrice != nil {
		purchaseWord = OfferPurchaseInternalWord
	}
	return OfferView{
		OfferNo:           offer.OfferNo,
		PositionKey:       offer.PositionKey,
		SupplierID:        offer.SupplierID,
		SupplierName:      offer.SupplierName,
		Brand:             offer.Brand,
		Article:           offer.Article,
		ClientPrice:       offer.ClientPrice,
		PurchasePrice:     offer.PurchasePrice,
		PurchaseWord:      purchaseWord,
		Currency:          offer.Currency,
		QuantityAvailable: offer.QuantityAvailable,
		TermDays:          offer.TermDays,
		TermWord:          termWord,
		Conditions:        offer.Conditions,
		Source: OfferSourceView{
			SourceSystem: offer.SourceSystem, SourceRecordID: offer.SourceRecordID, SourceVersion: offer.SourceVersion,
			SourceObservedAt: offer.SourceObservedAt, ReceivedAt: offer.ReceivedAt,
		},
		OfferDigest: OfferDigestOf(offer),
	}
}

// SelectionSnapshotView is the rendered material of ONE recorded client
// choice: the position, the supplier and the offer number of the chosen
// CURRENT record, the quantity, the unit client price (the recorded
// permitted ABCP price), the known client sum with its boundary word
// (AC-4 — the ONLY arithmetic of the slice, never the final cost), the
// pins of the EXACT versions of ALL FOUR inputs (AC-13) and the order
// boundary word (the fence OQ-3/OQ-4 — the order contour of 4.6).
type SelectionSnapshotView struct {
	SnapshotNo           int64     `json:"snapshot_no"`
	PositionKey          string    `json:"position_key"`
	SupplierID           string    `json:"supplier_id"`
	OfferNo              int64     `json:"offer_no"`
	Quantity             int       `json:"quantity"`
	UnitClientPrice      float64   `json:"unit_client_price"`
	Currency             string    `json:"currency"`
	KnownClientSum       float64   `json:"known_client_sum"`
	SumBoundaryWord      string    `json:"sum_boundary_word"`
	OrderBoundaryWord    string    `json:"order_boundary_word"`
	ProfileDigestPin     string    `json:"profile_digest_pin"`
	PartRequestDigestPin string    `json:"part_request_digest_pin"`
	FitmentDigestPin     string    `json:"fitment_digest_pin"`
	OfferDigestPin       string    `json:"offer_digest_pin"`
	SelectionDigest      string    `json:"selection_digest"`
	CreatedAt            time.Time `json:"created_at"`
}

// SelectionSnapshotViewOf renders the view of ONE snapshot record.
func SelectionSnapshotViewOf(material SelectionSnapshotMaterial, snapshotNo int64, createdAt time.Time) SelectionSnapshotView {
	return SelectionSnapshotView{
		SnapshotNo:           snapshotNo,
		PositionKey:          material.PositionKey,
		SupplierID:           material.SupplierID,
		OfferNo:              material.OfferNo,
		Quantity:             material.Quantity,
		UnitClientPrice:      material.UnitClientPrice,
		Currency:             material.Currency,
		KnownClientSum:       material.KnownClientSum,
		SumBoundaryWord:      OfferSumBoundaryWord,
		OrderBoundaryWord:    OfferOrderBoundaryWord,
		ProfileDigestPin:     material.ProfileDigestPin,
		PartRequestDigestPin: material.PartRequestDigestPin,
		FitmentDigestPin:     material.FitmentDigestPin,
		OfferDigestPin:       material.OfferDigestPin,
		SelectionDigest:      material.SelectionDigest,
		CreatedAt:            createdAt,
	}
}

// sha256Canonical marshals the canonical map (the sorted keys of the Go
// map marshal — the byte-stable form) and renders the hex SHA-256.
func sha256Canonical(canonical map[string]any) string {
        encoded, err := json.Marshal(canonical)
        if err != nil {
                // The shape is fixed (the strings, the numbers and the nils) — the
                // marshal cannot fail; the honest fallback keeps the derivation
                // total.
                encoded = []byte("{}")
        }
        sum := sha256.Sum256(encoded)
        return hex.EncodeToString(sum[:])
}
