package httptransport

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 4.5: the offers and comparison panel, the record APIs and the PRG
// forms
// ---------------------------------------------------------------------------
//
// The panel is the read-time comparison over the append-only offer
// records: the states are TEXT (role=alert for the boundary words), the
// variants group by the link type and the confirmed fitment (AC-5), the
// ranked variants carry the explanation words of the confirmed criteria
// (AC-6), the differences are never only color and the purchase price
// renders ONLY in the operator record section with the internal-data word
// (AC-2 — the client material family never carries it). The record
// commands are the operator-manager's writes: the machine JSON routes and
// the PRG forms of the case page (the a11y skeleton of the 2.10/2.11
// precedent: the labels, the 320px layout, the role=alert failures). The
// producer is the manager — NO automatic supplier request is performed
// (OQ-5/AG-4); the honest refusal words of the domain guards travel as
// the 400 problem texts.

// recordSupplierOfferInput is the machine JSON material of the offer
// record command (the pins are NEVER an input — the app derives them from
// the live derivations of the inputs).
type recordSupplierOfferInput struct {
	PositionKey       *string  `json:"position_key"`
	SupplierID        *string  `json:"supplier_id"`
	SupplierName      *string  `json:"supplier_name"`
	Brand             *string  `json:"brand"`
	Article           *string  `json:"article"`
	ClientPrice       *float64 `json:"client_price"`
	PurchasePrice     *float64 `json:"purchase_price"`
	Currency          *string  `json:"currency"`
	QuantityAvailable *int     `json:"quantity_available"`
	TermDays          *int     `json:"term_days"`
	Conditions        *string  `json:"conditions"`
	SourceSystem      *string  `json:"source_system"`
	SourceRecordID    *string  `json:"source_record_id"`
	SourceVersion     *string  `json:"source_version"`
	SourceObservedAt  *string  `json:"source_observed_at"`
}

// recordClientSelectionInput is the machine JSON material of the selection
// record command (the pins, the prices and the sum are NEVER inputs — the
// app derives them from the live derivations and the current offer read).
type recordClientSelectionInput struct {
	PositionKey *string `json:"position_key"`
	SupplierID  *string `json:"supplier_id"`
	OfferNo     *int64  `json:"offer_no"`
	Quantity    *int    `json:"quantity"`
}

// offersFormError flashes the honest message and returns to the case page
// (the PRG discipline — the form material never re-renders in place).
func offersFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	setFlashError(w, tenant, fitmentFormMessage(err))
	http.Redirect(w, r, workFormTarget(tenant, caseID)+"#offers", http.StatusSeeOther)
}

// recordSupplierOfferAPI records ONE manager's commercial conditions
// research result (the machine JSON route of the operator contour).
func (s *Server) recordSupplierOfferAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input recordSupplierOfferInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	positionKey := stringOrEmpty(input.PositionKey)
	if positionKey == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле position_key обязательно: предложение привязывается к позиции материала кейса.", false, nil)
		return
	}
	supplierID := stringOrEmpty(input.SupplierID)
	if supplierID == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле supplier_id обязательно: предложение несёт фактического поставщика.", false, nil)
		return
	}
	brand := stringOrEmpty(input.Brand)
	if brand == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле brand обязательно: сравнение показывает бренд как значимое различие.", false, nil)
		return
	}
	article := stringOrEmpty(input.Article)
	if article == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле article обязательно: предложение несёт артикул позиции.", false, nil)
		return
	}
	if input.ClientPrice == nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле client_price обязательно: клиентская цена по разрешению ABCP записывается, модель её не рассчитывает.", false, nil)
		return
	}
	currency := stringOrEmpty(input.Currency)
	if currency == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле currency обязательно: слово словаря валют пака.", false, nil)
		return
	}
	if input.QuantityAvailable == nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле quantity_available обязательно: количество у поставщика записывается.", false, nil)
		return
	}
	sourceSystem := stringOrEmpty(input.SourceSystem)
	if sourceSystem == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле source_system обязательно: слово словаря источников пака.", false, nil)
		return
	}
	sourceRecordID := stringOrEmpty(input.SourceRecordID)
	if sourceRecordID == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле source_record_id обязательно: трассируемость до ответа Owning System (AC-15).", false, nil)
		return
	}
	sourceVersion := stringOrEmpty(input.SourceVersion)
	if sourceVersion == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле source_version обязательно: трассируемость до ответа Owning System.", false, nil)
		return
	}
	observedRaw := stringOrEmpty(input.SourceObservedAt)
	if observedRaw == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле source_observed_at обязательно (RFC 3339): время получения материала источника.", false, nil)
		return
	}
	observedAt, err := time.Parse(time.RFC3339, observedRaw)
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле source_observed_at обязано быть временем RFC 3339.", false, nil)
		return
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.offers.RecordSupplierOffer(r.Context(), casesAPI.RecordSupplierOfferCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		PositionKey: positionKey, SupplierID: supplierID, SupplierName: stringOrEmpty(input.SupplierName),
		Brand: brand, Article: article, ClientPrice: *input.ClientPrice, PurchasePrice: input.PurchasePrice,
		Currency: currency, QuantityAvailable: *input.QuantityAvailable, TermDays: input.TermDays,
		Conditions: stringOrEmpty(input.Conditions),
		SourceSystem: sourceSystem, SourceRecordID: sourceRecordID, SourceVersion: sourceVersion,
		SourceObservedAt: observedAt.UTC(),
		IdempotencyKey:   key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		fitmentProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// recordSupplierOfferForm records ONE manager's commercial conditions
// research result through the operator PRG form of the case page (the a11y
// skeleton of the 2.10/2.11 precedent: the labels, the selects of the pack
// dictionaries, the optional numeric fields with the honest empty words).
func (s *Server) recordSupplierOfferForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы записи предложения."})
		return
	}
	positionKey := strings.TrimSpace(r.FormValue("position_key"))
	if positionKey == "" {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите ключ позиции — нормализованный артикул (без пробелов, верхний регистр)."})
		return
	}
	supplierID := strings.TrimSpace(r.FormValue("supplier_id"))
	if supplierID == "" {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите идентификатор поставщика."})
		return
	}
	brand := strings.TrimSpace(r.FormValue("brand"))
	if brand == "" {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите бренд: сравнение показывает бренд как значимое различие."})
		return
	}
	article := strings.TrimSpace(r.FormValue("article"))
	if article == "" {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите артикул позиции."})
		return
	}
	clientPrice, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("client_price")), 64)
	if err != nil {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Клиентская цена обязана быть числом (по разрешению ABCP — записывается, не рассчитывается)."})
		return
	}
	currency := r.FormValue("currency")
	if currency == "" {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите валюту (словарь пака)."})
		return
	}
	quantityAvailable, err := strconv.Atoi(strings.TrimSpace(r.FormValue("quantity_available")))
	if err != nil {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Количество у поставщика обязано быть целым числом."})
		return
	}
	sourceSystem := r.FormValue("source_system")
	if sourceSystem == "" {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите источник условий (словарь пака)."})
		return
	}
	sourceRecordID := strings.TrimSpace(r.FormValue("source_record_id"))
	if sourceRecordID == "" {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите идентификатор записи источника — трассируемость до ответа системы-владельца."})
		return
	}
	sourceVersion := strings.TrimSpace(r.FormValue("source_version"))
	if sourceVersion == "" {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите версию источника — трассируемость до ответа системы-владельца."})
		return
	}
	observedRaw := strings.TrimSpace(r.FormValue("source_observed_at"))
	if observedRaw == "" {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите время получения материала источника."})
		return
	}
	observedAt, err := time.Parse(time.RFC3339, observedRaw)
	if err != nil {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Время получения материала обязано быть RFC 3339 (например 2026-10-24T10:00:00Z)."})
		return
	}
	// The optional numeric fields: the EMPTY word is the honest absence
	// (never a silent zero — the purchase price «не видел / не записал»,
	// the term «срок не указан поставщиком»).
	var purchasePrice *float64
	if raw := strings.TrimSpace(r.FormValue("purchase_price")); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Закупочная цена (опциональные внутренние данные) обязана быть числом либо пустой."})
			return
		}
		purchasePrice = &value
	}
	var termDays *int
	if raw := strings.TrimSpace(r.FormValue("term_days")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Срок оценкой поставщика обязана быть целым числом дней либо пустой (срок не указан)."})
			return
		}
		termDays = &value
	}
	correlation, _ := correlationFromRequest(r)
	_, err = s.offers.RecordSupplierOffer(r.Context(), casesAPI.RecordSupplierOfferCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		PositionKey: positionKey, SupplierID: supplierID, SupplierName: strings.TrimSpace(r.FormValue("supplier_name")),
		Brand: brand, Article: article, ClientPrice: clientPrice, PurchasePrice: purchasePrice,
		Currency: currency, QuantityAvailable: quantityAvailable, TermDays: termDays,
		Conditions: strings.TrimSpace(r.FormValue("conditions")),
		SourceSystem: sourceSystem, SourceRecordID: sourceRecordID, SourceVersion: sourceVersion,
		SourceObservedAt: observedAt.UTC(),
		IdempotencyKey: formCommandKey("record-offer", actor.ID.String(), caseID.String(), workItemID.String(),
			positionKey, supplierID, brand, article, currency, sourceSystem, sourceRecordID, sourceVersion, observedRaw,
			strconv.FormatFloat(clientPrice, 'f', -1, 64), strconv.Itoa(quantityAvailable)),
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		offersFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "supplier_offer_recorded")
}

// recordClientSelectionAPI records ONE client selection (the machine JSON
// route of the operator contour).
func (s *Server) recordClientSelectionAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input recordClientSelectionInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	positionKey := stringOrEmpty(input.PositionKey)
	if positionKey == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле position_key обязательно: выбор привязывается к позиции материала кейса.", false, nil)
		return
	}
	supplierID := stringOrEmpty(input.SupplierID)
	if supplierID == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле supplier_id обязательно: выбор несёт поставщика выбранного предложения.", false, nil)
		return
	}
	if input.OfferNo == nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле offer_no обязательно: выбор пиннит выбранную текущую запись предложения.", false, nil)
		return
	}
	if input.Quantity == nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле quantity обязательно: снапшот фиксирует выбранное количество (AC-13).", false, nil)
		return
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.offers.RecordClientSelection(r.Context(), casesAPI.RecordClientSelectionCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		PositionKey: positionKey, SupplierID: supplierID, OfferNo: *input.OfferNo, Quantity: *input.Quantity,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		fitmentProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// recordClientSelectionForm records ONE client selection through the
// operator PRG form of the case page.
func (s *Server) recordClientSelectionForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы записи выбора."})
		return
	}
	positionKey := strings.TrimSpace(r.FormValue("position_key"))
	if positionKey == "" {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите ключ позиции — нормализованный артикул."})
		return
	}
	supplierID := strings.TrimSpace(r.FormValue("supplier_id"))
	if supplierID == "" {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите поставщика выбранного предложения."})
		return
	}
	offerNo, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("offer_no")), 10, 64)
	if err != nil {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Номер записи выбранного предложения обязана быть целым числом."})
		return
	}
	quantity, err := strconv.Atoi(strings.TrimSpace(r.FormValue("quantity")))
	if err != nil {
		offersFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Количество выбора обязано быть целым числом не меньше единицы."})
		return
	}
	correlation, _ := correlationFromRequest(r)
	_, err = s.offers.RecordClientSelection(r.Context(), casesAPI.RecordClientSelectionCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		PositionKey: positionKey, SupplierID: supplierID, OfferNo: offerNo, Quantity: quantity,
		IdempotencyKey: formCommandKey("record-selection", actor.ID.String(), caseID.String(), workItemID.String(),
			positionKey, supplierID, strconv.FormatInt(offerNo, 10), strconv.Itoa(quantity)),
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		offersFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "client_selection_recorded")
}

// ---------------------------------------------------------------------------
// The panel flattening (the template rows)
// ---------------------------------------------------------------------------

// offerVariantRow is one ranked variant of the panel (the CLIENT material
// — the purchase price is structurally absent, AC-2).
type offerVariantRow struct {
	SupplierID        string
	SupplierName      string
	Brand             string
	Article           string
	OfferNo           int64
	ClientPrice       string
	Currency          string
	QuantityAvailable int
	TermDays          string
	TermWord          string
	Conditions        string
	Rank              int
	RankWord          string
	Status            string
	StaleWords        []string
	ChangeWords       []string
	ShortfallWord     string
	OfferDigest       string
	SourceSystem      string
	SourceRecordID    string
	SourceVersion     string
	SourceObservedAt  string
	ReceivedAt        string
}

// offerPositionRow is one position of the panel: the group word (the link
// type and the confirmed fitment — AC-5), the offer state word with the
// honest words, the ranked variants and the no-criteria question.
type offerPositionRow struct {
	PositionKey       string
	LinkType          string
	LinkTypeWord      string
	FitmentStatus     string
	FitmentStatusWord string
	Status            string
	StatusWord        string
	Words             []string
	Question          string
	Variants          []offerVariantRow
	HandoffWords      []string
}

// offersPanelView is the flattened Story 4.5 panel of the case page.
type offersPanelView struct {
	Present           bool
	Note              string
	CaseUnknownWord   string
	CaseHandoffWords  []string
	PriceAuthorityWord string
	TermEstimateWord  string
	ScopeWord         string
	Positions         []offerPositionRow
	VoiceDifferences  []string
	VoiceRecommended  string
	VoiceFullWord     string
	Records           []casesDomain.OfferView
	Selections        []casesDomain.SelectionSnapshotView
}

// prepareOffersPanel flattens the Story 4.5 comparison panel view of the
// case API answer into the template rows: the states are TEXT, the
// variants group by the link type and the confirmed fitment, the ranked
// variants carry the explanation words, the boundaries are never only
// color (role=alert in the template) and the purchase price renders ONLY
// in the operator record section.
func prepareOffersPanel(view casesAPI.CaseView) offersPanelView {
	panel := offersPanelView{
		Note:              view.Comparison.Note,
		Positions:         []offerPositionRow{},
		CaseHandoffWords:  []string{},
		Records:           view.Comparison.Records,
		Selections:        view.Comparison.Selections,
	}
	if !view.Comparison.SchemaDefined {
		return panel
	}
	panel.Present = true
	comparison := view.Comparison.Comparison
	panel.CaseUnknownWord = comparison.CaseUnknownWord
	for _, word := range comparison.CaseHandoffWords {
		panel.CaseHandoffWords = append(panel.CaseHandoffWords, word+" — "+casesDomain.HandoffReasonText(word))
	}
	panel.PriceAuthorityWord = comparison.PriceAuthorityWord
	panel.TermEstimateWord = comparison.TermEstimateWord
	panel.ScopeWord = comparison.ScopeWord
	panel.VoiceDifferences = append([]string(nil), comparison.VoiceBlock.KeyDifferences...)
	panel.VoiceRecommended = comparison.VoiceBlock.RecommendedWord
	panel.VoiceFullWord = comparison.VoiceBlock.FullWord
	for _, position := range comparison.Positions {
		row := offerPositionRow{
			PositionKey:       position.PositionKey,
			LinkType:          position.LinkType,
			LinkTypeWord:      position.LinkTypeWord,
			FitmentStatus:     position.FitmentStatus,
			FitmentStatusWord: position.FitmentStatusWord,
			Status:            position.Status,
			StatusWord:        position.StatusWord,
			Words:             append([]string(nil), position.Words...),
			Question:          position.Question,
			Variants:          []offerVariantRow{},
			HandoffWords:      []string{},
		}
		for _, word := range position.HandoffWords {
			row.HandoffWords = append(row.HandoffWords, word+" — "+casesDomain.HandoffReasonText(word))
		}
		for _, variant := range position.Variants {
			row.Variants = append(row.Variants, offerVariantRow{
				SupplierID:        variant.SupplierID,
				SupplierName:      variant.SupplierName,
				Brand:             variant.Brand,
				Article:           variant.Article,
				OfferNo:           variant.OfferNo,
				ClientPrice:       formatPrice(variant.ClientPrice),
				Currency:          variant.Currency,
				QuantityAvailable: variant.QuantityAvailable,
				TermDays:          formatOptionalInt(variant.TermDays),
				TermWord:          variant.TermWord,
				Conditions:        variant.Conditions,
				Rank:              variant.Rank,
				RankWord:          variant.RankWord,
				Status:            variant.Status,
				StaleWords:        append([]string(nil), variant.StaleWords...),
				ChangeWords:       append([]string(nil), variant.ChangeWords...),
				ShortfallWord:     variant.ShortfallWord,
				OfferDigest:       variant.OfferDigest,
				SourceSystem:      variant.Source.SourceSystem,
				SourceRecordID:    variant.Source.SourceRecordID,
				SourceVersion:     variant.Source.SourceVersion,
				SourceObservedAt:  formatTime(variant.Source.SourceObservedAt),
				ReceivedAt:        formatTime(variant.Source.ReceivedAt),
			})
		}
		panel.Positions = append(panel.Positions, row)
	}
	return panel
}

// formatPrice renders one client price (the honest number form — the
// thousands never silently rounded).
func formatPrice(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// formatOptionalInt renders the optional integer (the empty word is the
// honest absence — never a zero).
func formatOptionalInt(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

// offersStatusKindText renders the honest status text of one offer state
// (the template shows the word — never only color).
func offersStatusKindText(status string) string {
	switch status {
	case casesDomain.OfferStatusCurrent:
		return "актуальные предложения записаны"
	case casesDomain.OfferStatusStale:
		return "предложения устарели"
	default:
		return "условия неизвестны"
	}
}
