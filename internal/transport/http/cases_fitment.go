package httptransport

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 4.4: the applicability panel, the record API and the PRG form
// ---------------------------------------------------------------------------
//
// The panel is the read-time state ladder over the append-only evidence
// records: the states are TEXT (role=alert for the boundary words), the
// cross chains render WHOLE, the sources open as the details without
// leaving the case (AC-14), the boundaries are never only color. The
// record command is the operator-manager's write: the machine JSON route
// and the PRG form of the case page (the a11y skeleton of the 2.10/2.11
// precedent: the labels, the 320px layout, the role=alert failures). The
// producer is the manager — NO automatic catalog request is performed
// (OQ-5/AG-4); the honest refusal words of the domain guards travel as
// the 400 problem texts.

// recordFitmentEvidenceInput is the machine JSON material of the record
// command (the pins are NEVER an input — the app derives them from the
// live derivations of both inputs).
type recordFitmentEvidenceInput struct {
	PositionKey       *string  `json:"position_key"`
	CatalogNode       *string  `json:"catalog_node"`
	Article           *string  `json:"article"`
	LinkType          *string  `json:"link_type"`
	Chain             []string `json:"chain"`
	CatalogVehicleIDs []string `json:"catalog_vehicle_ids"`
	SourceSystem      *string  `json:"source_system"`
	SourceRecordID    *string  `json:"source_record_id"`
	SourceVersion     *string  `json:"source_version"`
	SourceObservedAt  *string  `json:"source_observed_at"`
}

// fitmentProblem maps the honest domain refusals of the record command
// onto the problem answers: the work rule words are the 400 problem
// texts (the dictionary words travel, the internal details never leak);
// the scope refusal is the 403; the unknown case or work item is the
// 404; the raced idempotency key with another material is the honest 409.
func fitmentProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Ключ идемпотентности занят другой командой с другим материалом — перечитайте обращение и повторите с новым ключом.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение или элемент работы не найден в этом пространстве.", false, nil)
	default:
		server.problem(w, r, http.StatusBadRequest, "invalid_request", err.Error(), false, nil)
	}
}

// fitmentFormError flashes the honest message and returns to the case
// page (the PRG discipline — the form material never re-renders in
// place).
func fitmentFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	setFlashError(w, tenant, fitmentFormMessage(err))
	http.Redirect(w, r, workFormTarget(tenant, caseID)+"#fitment", http.StatusSeeOther)
}

// fitmentFormMessage renders the honest flash message of one refusal.
func fitmentFormMessage(err error) string {
	var rule casesDomain.WorkRuleError
	if errors.As(err, &rule) {
		return rule.Reason
	}
	switch {
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		return "Запись не создана: ключ команды занят другим материалом — перечитайте обращение и повторите."
	case errors.Is(err, casesDomain.ErrNotFound):
		return "Запись не создана: обращение или элемент работы не найден."
	default:
		return "Запись доказательства не создана: проверьте материал записи."
	}
}

// recordFitmentEvidenceAPI records ONE manager's catalog research result
// (the machine JSON route of the operator contour).
func (s *Server) recordFitmentEvidenceAPI(w http.ResponseWriter, r *http.Request) {
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
	var input recordFitmentEvidenceInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	positionKey := stringOrEmpty(input.PositionKey)
	if positionKey == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле position_key обязательно: запись доказательства привязывается к позиции материала кейса.", false, nil)
		return
	}
	linkType := stringOrEmpty(input.LinkType)
	if linkType == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле link_type обязательно: слово словаря пака (original | cross).", false, nil)
		return
	}
	sourceSystem := stringOrEmpty(input.SourceSystem)
	if sourceSystem == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле source_system обязательно: слово словаря источников пака.", false, nil)
		return
	}
	sourceRecordID := stringOrEmpty(input.SourceRecordID)
	if sourceRecordID == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле source_record_id обязательно: трассируемость до ответа Owning System.", false, nil)
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
	outcome, err := s.fitments.RecordFitmentEvidence(r.Context(), casesAPI.RecordFitmentEvidenceCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		PositionKey: positionKey, CatalogNode: stringOrEmpty(input.CatalogNode), Article: stringOrEmpty(input.Article),
		LinkType: linkType, Chain: input.Chain, CatalogVehicleIDs: input.CatalogVehicleIDs,
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

// recordFitmentEvidenceForm records ONE manager's catalog research result
// through the operator PRG form of the case page (the a11y skeleton of
// the 2.10/2.11 precedent: the labels, the selects of the pack
// dictionaries, the textarea of the whole cross chain — one node per
// line, the textarea of the catalog vehicle ids — one id per line or the
// comma-separated list).
func (s *Server) recordFitmentEvidenceForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		fitmentFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы записи доказательства."})
		return
	}
	positionKey := strings.TrimSpace(r.FormValue("position_key"))
	if positionKey == "" {
		fitmentFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите ключ позиции — нормализованный артикул (без пробелов, верхний регистр)."})
		return
	}
	linkType := r.FormValue("link_type")
	if linkType == "" {
		fitmentFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите тип связи (словарь пака: original | cross)."})
		return
	}
	sourceSystem := r.FormValue("source_system")
	if sourceSystem == "" {
		fitmentFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите источник исследования (словарь пака)."})
		return
	}
	sourceRecordID := strings.TrimSpace(r.FormValue("source_record_id"))
	if sourceRecordID == "" {
		fitmentFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите идентификатор записи источника — трассируемость до ответа системы-владельца."})
		return
	}
	sourceVersion := strings.TrimSpace(r.FormValue("source_version"))
	if sourceVersion == "" {
		fitmentFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите версию источника — трассируемость до ответа системы-владельца."})
		return
	}
	observedRaw := strings.TrimSpace(r.FormValue("source_observed_at"))
	if observedRaw == "" {
		fitmentFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Укажите время получения материала источника."})
		return
	}
	observedAt, err := time.Parse(time.RFC3339, observedRaw)
	if err != nil {
		fitmentFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Время получения материала обязано быть RFC 3339 (например 2026-10-24T10:00:00Z)."})
		return
	}
	correlation, _ := correlationFromRequest(r)
	_, err = s.fitments.RecordFitmentEvidence(r.Context(), casesAPI.RecordFitmentEvidenceCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		PositionKey: positionKey, CatalogNode: strings.TrimSpace(r.FormValue("catalog_node")),
		Article:  strings.TrimSpace(r.FormValue("article")),
		LinkType: linkType, Chain: formLines(r.FormValue("chain")), CatalogVehicleIDs: formLines(r.FormValue("catalog_vehicle_ids")),
		SourceSystem: sourceSystem, SourceRecordID: sourceRecordID, SourceVersion: sourceVersion,
		SourceObservedAt: observedAt.UTC(),
		IdempotencyKey: formCommandKey("record-fitment", actor.ID.String(), caseID.String(), workItemID.String(),
			positionKey, linkType, sourceSystem, sourceRecordID, sourceVersion, observedRaw,
			strings.Join(formLines(r.FormValue("chain")), "|"), strings.Join(formLines(r.FormValue("catalog_vehicle_ids")), "|")),
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		fitmentFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "fitment_evidence_recorded")
}

// formLines splits one textarea value into the non-empty lines (the
// comma-separated words of one line are the additional nodes — the whole
// chain and the id list ride the same form field honestly).
func formLines(value string) []string {
	lines := []string{}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, word := range strings.Split(line, ",") {
			word = strings.TrimSpace(word)
			if word != "" {
				lines = append(lines, word)
			}
		}
	}
	return lines
}

// ---------------------------------------------------------------------------
// The panel flattening (the template rows)
// ---------------------------------------------------------------------------

// fitmentPositionRow is one position of the panel: the state word with
// the honest words, the evidence records material (BOTH under the
// contradiction — AC-9), the digest of the proven state and the question
// rows of the ambiguous state.
type fitmentPositionRow struct {
	PositionKey  string
	Status       string
	StatusWord   string
	Words        []string
	Evidences    []fitmentEvidenceRow
	Digest       string
	Questions    []fitmentQuestionRow
	HandoffWords []string
}

// fitmentEvidenceRow is one evidence record of the panel: the material
// with the traceability (the source, the record id, the version, both
// times) and the per-record stale words.
type fitmentEvidenceRow struct {
	EvidenceNo           int64
	SourceSystem         string
	SourceRecordID       string
	SourceVersion        string
	SourceObservedAt     string
	ReceivedAt           string
	LinkType             string
	Chain                []string
	CatalogVehicleIDs    []string
	CatalogNode          string
	Article              string
	ProfileDigestPin     string
	PartRequestDigestPin string
	StaleWords           []string
	Found                bool
}

// fitmentQuestionRow is one ambiguous question row.
type fitmentQuestionRow struct {
	Key      string
	Question string
}

// fitmentPanelView is the flattened Story 4.4 panel of the case page.
type fitmentPanelView struct {
	Present          bool
	Note             string
	CaseUnknownWord  string
	CaseHandoffWords []string
	NumberMatchWord  string
	PriceNote        string
	ScopeWord        string
	Positions        []fitmentPositionRow
}

// prepareFitmentPanel flattens the Story 4.4 fitment panel view of the
// case API answer into the template rows: the states are TEXT, the
// chains render WHOLE, the sources are the details material (the panel
// shows the traceability without leaving the case — AC-14), the
// boundaries are never only color (role=alert in the template).
func prepareFitmentPanel(view casesAPI.CaseView) fitmentPanelView {
	panel := fitmentPanelView{
		Note:             view.Fitment.Note,
		Positions:        []fitmentPositionRow{},
		CaseHandoffWords: []string{},
	}
	if !view.Fitment.SchemaDefined {
		return panel
	}
	panel.Present = true
	fitment := view.Fitment.Fitment
	panel.CaseUnknownWord = fitment.CaseUnknownWord
	for _, word := range fitment.CaseHandoffWords {
		panel.CaseHandoffWords = append(panel.CaseHandoffWords, word+" — "+casesDomain.HandoffReasonText(word))
	}
	panel.NumberMatchWord = fitment.NumberMatchWord
	panel.PriceNote = fitment.PriceNote
	panel.ScopeWord = fitment.ScopeWord
	for _, position := range fitment.Positions {
		row := fitmentPositionRow{
			PositionKey:  position.PositionKey,
			Status:       position.Status,
			StatusWord:   position.StatusWord,
			Words:        append([]string(nil), position.Words...),
			Evidences:    []fitmentEvidenceRow{},
			Digest:       position.FitmentDigest,
			Questions:    []fitmentQuestionRow{},
			HandoffWords: []string{},
		}
		for key, question := range position.Questions {
			row.Questions = append(row.Questions, fitmentQuestionRow{Key: key, Question: question})
		}
		for _, word := range position.HandoffWords {
			row.HandoffWords = append(row.HandoffWords, word+" — "+casesDomain.HandoffReasonText(word))
		}
		for _, evidence := range position.Evidences {
			row.Evidences = append(row.Evidences, fitmentEvidenceRow{
				EvidenceNo:           evidence.EvidenceNo,
				SourceSystem:         evidence.SourceSystem,
				SourceRecordID:       evidence.SourceRecordID,
				SourceVersion:        evidence.SourceVersion,
				SourceObservedAt:     formatTime(evidence.SourceObservedAt),
				ReceivedAt:           formatTime(evidence.ReceivedAt),
				LinkType:             evidence.LinkType,
				Chain:                append([]string(nil), evidence.Chain...),
				CatalogVehicleIDs:    append([]string(nil), evidence.CatalogVehicleIDs...),
				CatalogNode:          evidence.CatalogNode,
				Article:              evidence.Article,
				ProfileDigestPin:     evidence.ProfileDigestPin,
				PartRequestDigestPin: evidence.PartRequestDigestPin,
				StaleWords:           append([]string(nil), evidence.StaleWords...),
				Found:                evidence.Found,
			})
		}
		panel.Positions = append(panel.Positions, row)
	}
	return panel
}

// fitmentStatusKindText renders the honest status text of one position
// state (the template shows the word — never only color).
func fitmentStatusKindText(status string) string {
	switch status {
	case casesDomain.FitmentStatusProven:
		return "подтверждённо подходит"
	case casesDomain.FitmentStatusAmbiguous:
		return "неоднозначно: несколько модификаций"
	case casesDomain.FitmentStatusConflicted:
		return "источники противоречат"
	case casesDomain.FitmentStatusStale:
		return "доказательство устарело"
	default:
		return "не проверено"
	}
}

// fitmentEvidenceNoText renders the evidence number word of one record.
func fitmentEvidenceNoText(no int64) string {
	return "запись №" + strconv.FormatInt(no, 10)
}
