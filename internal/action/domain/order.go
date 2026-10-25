package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Module action, story 4.6 — the Order Commitment material domain: the
// canonical order composition over the pins of ALL FIVE inputs (Profile 4-2,
// PartRequest 4-3, Fitment 4-4, Offer + Selection 4-5), its guard, its
// ONE canonical digest and the honest words of the slice. The order is an
// ACTION, never a supplier send: the typed material carries no supplier-send
// field at all (the OQ-3 fence — supplier_form/send stays absent from the
// Action Catalog).
//
// The material is NEVER hand-written by the operator: the app builder
// derives it live from the CaseView (the recorded selections of 4-5 plus the
// current derivations) — the «the operator does not type digests» principle
// of 4-4/4-5. The guard below is the fail-closed admission of whatever the
// builder (or a test) assembles.

// ---------------------------------------------------------------------------
// The honest words of the slice (the text, never only color)
// ---------------------------------------------------------------------------

// OrderManagerCheckWord is the MANDATORY word of the order material (AC-1):
// the client-facing truth that the order is created in the store and is
// checked by a manager BEFORE anything is ever sent to a supplier. The word
// is a CONSTANT of the material — it cannot be absent, altered or opted out
// of; the digest derivation injects it, the guard demands the render carries
// it, the preview speaks it.
const OrderManagerCheckWord = "заказ сначала будет проверен менеджером и ещё не отправляется поставщику"

// OrderManagerAssignmentWord is the receipt-render word of the manager
// assignment (the derived READ-TIME word of the committed outcome — the
// delegated-task mechanics belong to the contour 4-7, this slice only
// speaks the assignment honestly).
const OrderManagerAssignmentWord = "заказ назначается проверяющему менеджеру согласно утверждённой политике (задача проверки — контур 4-7)"

// OrderClientCreatedWord is the client word of the created order (AC-14):
// ONLY this word is spoken — the order exists in the store and awaits the
// manager check.
const OrderClientCreatedWord = "заказ создан в магазине и ожидает проверки менеджера"

// OrderClientBoundaryWord is the client boundary word (AC-14): the
// never-spoken claims, spoken as the honest negation.
const OrderClientBoundaryWord = "не заказан у поставщика, не выкуплен, не отправлен, не гарантирован"

// OrderOneCUnknownWord is the honest 1C word (AC-13, OQ-4/OQ-19): the ABCP
// Order creation is NEVER declared a successful 1C synchronization — the
// projection stays unknown until proven.
const OrderOneCUnknownWord = "проекция в 1С неизвестна (unknown): синхронизация не доказана — создание ABCP Order не является успешной синхронизацией с 1С (OQ-4/OQ-19)"

// OrderSumBoundaryWord is the honest sum word of the order material (the
// 4-5 boundary carried into the commitment: the known sum without delivery
// and fees; the final cost is not computed).
const OrderSumBoundaryWord = "известная сумма без доставки и сбора — окончательная стоимость не рассчитана"

// OrderCompositionChangedWord is the dispatch-ladder word of the
// execute-time revalidation refusal (AC-5): the live re-derivation of the
// OrderDigest diverged from the pinned preview material — the dispatch is
// blocked and the client's updated confirmation is required. Never a false
// «failed externally» (FR-28), never a silent pass.
const OrderCompositionChangedWord = "состав заказа изменился: цена/наличие/срок/версия входа — требуется обновлённое подтверждение клиента"

// OrderVersionSourceMissingWord is the fail-closed word of the missing
// OrderVersionSource port (AG-4, the FreshReads precedent): no live source —
// no dispatch. The honest block, never an assumed «unchanged».
const OrderVersionSourceMissingWord = "источник версии заказа не подключён — dispatch заблокирован (fail-closed)"

// OrderPositionGuardWord is the honest refusal word of the position-count
// bound: one order carries one to eight positions (the case selections),
// the split into a second order never happens silently.
const OrderPositionGuardWord = "один заказ несёт от одной до восьми позиций: разбиение на второй заказ не выполняется"

// OrderNoSelectionWord is the builder's honest refusal word (the app layer
// speaks it BEFORE any proposal): no recorded client selection — no order.
const OrderNoSelectionWord = "нет подтверждённого выбора клиента: заказ строится из записанного выбора с точными версиями"

// OrderFitmentNotProvenWord is the builder's honest refusal word of a
// selected position whose live fitment derivation is not proven (the 4-5
// selection guard already refused the RECORD; the builder re-derives LIVE —
// the honest belt over braces).
const OrderFitmentNotProvenWord = "применимость не подтверждена: заказ строится только из подтверждённых вариантов — выберите подтверждённый вариант либо передайте человеку"

// OrderMixedInputVersionsWord is the builder's honest refusal word of the
// selections pinned to DIFFERENT case-level input versions (a newer profile
// or request version arrived after an older selection): the mixed versions
// never silently ride into one order — the updated client selection is
// required.
const OrderMixedInputVersionsWord = "версии входов расходятся между выборами: требуется обновлённый выбор клиента по текущим версиям входов"

// OrderMaterialOversizeWord is the builder's honest refusal word of the
// canonical render exceeding the typed-input field bound: the material is
// never silently truncated.
const OrderMaterialOversizeWord = "материал заказа превышает предел поля предложения — сократите известные условия позиций"

// OrderMixedCurrencyWord is the builder's honest refusal word of the
// selections in DIFFERENT currencies: one order carries ONE currency word
// of the pack dictionary — a mixed-currency composition never rides into
// one order silently.
const OrderMixedCurrencyWord = "валюты позиций расходятся: один заказ несёт одну валюту словаря пака — обновите выбор клиента"

// OrderOfferUnresolvedWord is the builder's honest refusal word of a
// selection whose pinned offer record does not resolve against the live
// records of the case (the append-only truth): the material is never
// invented — the integrity refusal, never a masked pass.
const OrderOfferUnresolvedWord = "записанное предложение выбора не резолвится против живых записей кейса — целостность материала нарушена"

// The action pin of the order contour (the first product entry of the
// catalog, story 4.6): the receipt render and the composition revalidation
// address THIS action BY NAME.
const (
	// OrderActionID is the catalog ID of the ABCP client order action.
	OrderActionID = "create_abcp_customer_order"
	// OrderActionVersion is the catalog version of the first slice.
	OrderActionVersion = "1.0.0"
)

// The builder's sentinel errors (the word IS the message; the transport
// maps them by name — the honest refusal BEFORE any proposal).
var (
	// ErrOrderNoSelection — no recorded selection, no order.
	ErrOrderNoSelection = errors.New(OrderNoSelectionWord)
	// ErrOrderFitmentNotProven — a selected position without the proven fitment.
	ErrOrderFitmentNotProven = errors.New(OrderFitmentNotProvenWord)
	// ErrOrderMixedInputVersions — selections pinned to different input versions.
	ErrOrderMixedInputVersions = errors.New(OrderMixedInputVersionsWord)
	// ErrOrderMaterialOversize — the canonical render exceeds the field bound.
	ErrOrderMaterialOversize = errors.New(OrderMaterialOversizeWord)
	// ErrOrderMixedCurrency — the selections carry different currency words.
	ErrOrderMixedCurrency = errors.New(OrderMixedCurrencyWord)
	// ErrOrderOfferUnresolved — a pinned offer record does not resolve.
	ErrOrderOfferUnresolved = errors.New(OrderOfferUnresolvedWord)
)

// IsOrderDerivationRefusal reports whether the error is one of the honest
// derivation refusals of the LIVE order material (the revalidation fence
// reads them as the CHANGED composition — the updated client confirmation
// is required; never a transport failure of the drain).
func IsOrderDerivationRefusal(err error) bool {
	return errors.Is(err, ErrOrderNoSelection) ||
		errors.Is(err, ErrOrderFitmentNotProven) ||
		errors.Is(err, ErrOrderMixedInputVersions) ||
		errors.Is(err, ErrOrderMixedCurrency) ||
		errors.Is(err, ErrOrderOfferUnresolved) ||
		errors.Is(err, ErrOrderMaterialOversize)
}

// OrderReceiptStatePendingManagerVerification is the DERIVED read-time
// state word of the committed outcome render (the stored receipt of 3-6
// stays three-worded {committed, failed, unknown} — this word is a
// projection of committed for THIS action, never a stored word).
const OrderReceiptStatePendingManagerVerification = "pending_manager_verification"

// OrderReceiptStateWords is the derived state dictionary of the order
// receipt render (the honest words of every state word).
var OrderReceiptStateWords = map[string]string{
	OrderReceiptStatePendingManagerVerification: OrderClientCreatedWord,
}

// ---------------------------------------------------------------------------
// The dictionaries of the material
// ---------------------------------------------------------------------------

// The originality words of an order position (the fitment link-type
// dictionary carried into the order material: original — the OEM part,
// analog — the cross-number replacement).
const (
	OrderOriginalityOriginal = "original"
	OrderOriginalityAnalog   = "analog"
)

// OrderOriginalityWords is the full dictionary.
var OrderOriginalityWords = []string{OrderOriginalityOriginal, OrderOriginalityAnalog}

// OrderOriginalityRenderWord renders the dictionary word into the honest
// position word (the render text of the material and the digest alike).
func OrderOriginalityRenderWord(word string) string {
	switch word {
	case OrderOriginalityOriginal:
		return "оригинал"
	case OrderOriginalityAnalog:
		return "аналог (кросс-номер)"
	}
	return word
}

// The position-count bounds of one order (the case selections of 4-5; the
// second order is never silently created).
const (
	OrderPositionsMin = 1
	OrderPositionsMax = 8
)

// The pin length of the digests (the 64-hex SHA-256 words of 4-2..4-5).
const orderPinHexLen = 64

// ---------------------------------------------------------------------------
// The material
// ---------------------------------------------------------------------------

// OrderPosition is ONE position of the order: the chosen variant of one
// part request position with the pins of ALL its inputs (Fitment 4-4,
// Offer 4-5, Selection 4-5) and the client-facing commercial material
// (brand, article, originality word, quantity, unit client price, the known
// conditions). NO supplier-send field exists here — structurally.
type OrderPosition struct {
	PositionKey      string  `json:"position_key"`
	SupplierID       string  `json:"supplier_id"`
	OfferNo          int64   `json:"offer_no"`
	Brand            string  `json:"brand"`
	Article          string  `json:"article"`
	Originality      string  `json:"originality"`
	Quantity         int     `json:"quantity"`
	UnitClientPrice  float64 `json:"unit_client_price"`
	Currency         string  `json:"currency"`
	Conditions       string  `json:"conditions,omitempty"`
	FitmentDigestPin string  `json:"fitment_digest_pin"`
	OfferDigestPin   string  `json:"offer_digest_pin"`
	SelectionDigest  string  `json:"selection_digest_pin"`
}

// OrderCommitmentMaterial is the canonical order composition: the case-level
// pins (Profile 4-2, PartRequest 4-3), one to eight positions, the known
// client sum and its currency. The manager-check word is a CONSTANT of the
// material — carried by the digest derivation and the canonical render, not
// an optional field (AC-1).
type OrderCommitmentMaterial struct {
	ProfileDigestPin     string          `json:"profile_digest_pin"`
	PartRequestDigestPin string          `json:"part_request_digest_pin"`
	Positions            []OrderPosition `json:"positions"`
	KnownClientSum       float64         `json:"known_client_sum"`
	Currency             string          `json:"currency"`
}

// GuardOrderCommitmentMaterial refuses a material that is not complete and
// fully in-dictionary (the fail-closed admission of the builder's output):
// the case-level pins are 64-hex words; one to eight positions without
// duplicate position keys; every position pins its three digests; the
// quantity is at least one; the unit price is non-negative; the currency is
// a non-empty word; the originality is a dictionary word. Every refusal
// names its rule honestly — no silent repair ever happens.
func GuardOrderCommitmentMaterial(m OrderCommitmentMaterial) error {
	if !orderPinHex(m.ProfileDigestPin) {
		return RuleError{"order.profile_digest_pin", "Пин профиля автомобиля (4-2) обязателен: 64-hex дайджест точной версии."}
	}
	if !orderPinHex(m.PartRequestDigestPin) {
		return RuleError{"order.part_request_digest_pin", "Пин запроса детали (4-3) обязателен: 64-hex дайджест точной версии."}
	}
	if len(m.Positions) < OrderPositionsMin || len(m.Positions) > OrderPositionsMax {
		return RuleError{"order.positions", fmt.Sprintf(
			"Один заказ несёт от %d до %d позиций (выборы кейса), получено %d — %s.",
			OrderPositionsMin, OrderPositionsMax, len(m.Positions), OrderPositionGuardWord)}
	}
	seen := map[string]bool{}
	for i, p := range m.Positions {
		rule := fmt.Sprintf("order.positions[%d]", i)
		if strings.TrimSpace(p.PositionKey) == "" {
			return RuleError{rule + ".position_key", "Канонический ключ позиции (4-3) обязателен."}
		}
		if seen[p.PositionKey] {
			return RuleError{rule + ".position_key", fmt.Sprintf(
				"Позиция «%s» повторяется: дубль позиции в одном заказе не допускается.", p.PositionKey)}
		}
		seen[p.PositionKey] = true
		if strings.TrimSpace(p.SupplierID) == "" {
			return RuleError{rule + ".supplier_id", "Идентификатор поставщика позиции обязателен (выбор 4-5)."}
		}
		if p.OfferNo < 1 {
			return RuleError{rule + ".offer_no", "Номер предложения (4-5) обязателен (монотонный номер записи)."}
		}
		if strings.TrimSpace(p.Brand) == "" || strings.TrimSpace(p.Article) == "" {
			return RuleError{rule + ".brand_article", "Бренд и артикул позиции обязательны (материал предложения 4-5)."}
		}
		if !WordIn(OrderOriginalityWords, p.Originality) {
			return RuleError{rule + ".originality", fmt.Sprintf(
				"Слово оригинальности позиции обязано быть словом словаря {%s}.",
				strings.Join(OrderOriginalityWords, ", "))}
		}
		if p.Quantity < 1 {
			return RuleError{rule + ".quantity", "Количество позиции не меньше единицы (снапшот выбора 4-5 фиксирует выбранное количество)."}
		}
		if p.UnitClientPrice < 0 {
			return RuleError{rule + ".unit_client_price", "Клиентская цена позиции не отрицательна (записанная разрешённая цена ABCP)."}
		}
		if strings.TrimSpace(p.Currency) == "" {
			return RuleError{rule + ".currency", "Валюта позиции обязательна (слово словаря пака auto-parts@6.0.0)."}
		}
		if !orderPinHex(p.FitmentDigestPin) {
			return RuleError{rule + ".fitment_digest_pin", "Пин доказательства применимости (4-4) обязателен: 64-hex дайджест."}
		}
		if !orderPinHex(p.OfferDigestPin) {
			return RuleError{rule + ".offer_digest_pin", "Пин предложения (4-5) обязателен: 64-hex дайджест."}
		}
		if !orderPinHex(p.SelectionDigest) {
			return RuleError{rule + ".selection_digest_pin", "Пин выбора клиента (4-5) обязателен: 64-hex дайджест."}
		}
	}
	if m.KnownClientSum < 0 {
		return RuleError{"order.known_client_sum", "Известная клиентская сумма не отрицательна (арифметика выбора 4-5)."}
	}
	if strings.TrimSpace(m.Currency) == "" {
		return RuleError{"order.currency", "Валюта суммы обязательна (слово словаря пака auto-parts@6.0.0)."}
	}
	return nil
}

// ---------------------------------------------------------------------------
// The ONE canonical digest
// ---------------------------------------------------------------------------

// OrderDigestOf renders the SHA-256 of the canonical JSON of the WHOLE
// order composition — the case-level pins, EVERY position with its three
// pins and its client material (sorted by the position key — the byte-stable
// form), the known sum, the currency and the MANDATORY manager-check word
// (the constant of the material, AC-1). One value carries the exact version
// of ALL FIVE inputs: any substantial change of any input changes the
// digest (AC-5 — the execute-time revalidation compares ONE value).
func OrderDigestOf(m OrderCommitmentMaterial) string {
	positions := make([]map[string]any, 0, len(m.Positions))
	for _, p := range m.Positions {
		positions = append(positions, map[string]any{
			"position_key":       p.PositionKey,
			"supplier_id":        p.SupplierID,
			"offer_no":           p.OfferNo,
			"brand":              p.Brand,
			"article":            p.Article,
			"originality":        p.Originality,
			"quantity":           p.Quantity,
			"unit_client_price":  p.UnitClientPrice,
			"currency":           p.Currency,
			"conditions":         p.Conditions,
			"fitment_digest_pin": p.FitmentDigestPin,
			"offer_digest_pin":   p.OfferDigestPin,
			"selection_digest":   p.SelectionDigest,
		})
	}
	sort.Slice(positions, func(i, j int) bool {
		return positions[i]["position_key"].(string) < positions[j]["position_key"].(string)
	})
	canonical := map[string]any{
		"profile_digest_pin":      m.ProfileDigestPin,
		"part_request_digest_pin": m.PartRequestDigestPin,
		"positions":               positions,
		"known_client_sum":        m.KnownClientSum,
		"currency":                m.Currency,
		"manager_check_word":      OrderManagerCheckWord,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		// The shape is fixed (strings, numbers, nils, maps) — the marshal
		// cannot fail; the honest fallback keeps the derivation total.
		encoded = []byte("{}")
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// The canonical render (the typed-input material of the proposal)
// ---------------------------------------------------------------------------

// OrderMaterialCanonicalText renders the canonical HUMAN-readable text of
// the material for the proposal's typed input (the machine 3-4 carries the
// render; the digest pins the bytes). Deterministic: the positions are
// sorted by the position key, every word comes from the material or the
// constants — the render is byte-stable for one material.
func OrderMaterialCanonicalText(m OrderCommitmentMaterial) string {
	positions := make([]OrderPosition, len(m.Positions))
	copy(positions, m.Positions)
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].PositionKey < positions[j].PositionKey
	})
	var b strings.Builder
	for _, p := range positions {
		line := fmt.Sprintf("позиция %s: %s / %s (%s); кол-во %d; цена %.2f %s",
			p.PositionKey, p.Brand, p.Article, OrderOriginalityRenderWord(p.Originality),
			p.Quantity, p.UnitClientPrice, p.Currency)
		if strings.TrimSpace(p.Conditions) != "" {
			line += "; условия: " + p.Conditions
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "итого: %.2f %s (%s)\n", m.KnownClientSum, m.Currency, OrderSumBoundaryWord)
	b.WriteString(OrderManagerCheckWord)
	return b.String()
}

// orderPinHex reports whether the value is the 64-hex lowercase digest word
// of the pinned inputs (4-2..4-5).
func orderPinHex(value string) bool {
	if len(value) != orderPinHexLen {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// OrderMaterialPositionKeys parses the position keys of the canonical
// render (the byte-stable «позиция <key>: …» lines of
// OrderMaterialCanonicalText — the frozen 4.6 format). The block-decision
// composition check of the verification contour (4.7) reads the keys of
// the composition the OPERATION pinned: an affected position outside the
// pinned composition is the honest guard refusal, never a silent clip.
// The parse is deterministic over the canonical format ONLY — the empty
// list of a non-render input (the caller refuses by its own words).
func OrderMaterialPositionKeys(render string) []string {
	keys := []string{}
	for _, line := range strings.Split(render, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "позиция ") {
			continue
		}
		rest := strings.TrimPrefix(line, "позиция ")
		if idx := strings.Index(rest, ":"); idx > 0 {
			keys = append(keys, rest[:idx])
		}
	}
	return keys
}
