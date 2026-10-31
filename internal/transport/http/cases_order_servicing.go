package httptransport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// Story 4.10 (step-03): the «Обслуживание заказа» panel of the case
// page — the Order Servicing Request surface over the repeat-contact
// material of the store contour. The panel carries the recorded
// requests of the case (the append-only feed with the proof pins —
// the state, the method, the issued/expiry times stay AS RECORDED,
// AC-4) with the LIVE derivations (the client words re-derive over the
// live projection of the resolved operation, the cancel feasibility
// re-derives over the live supplier facts); the intent and the outcome
// are WORDS, never colors only (AC-16); the Identity Proof Gate speaks
// its word with the LIVE allowed methods of the conversation (the
// assurance panel of 2-3 rides the SAME page — the equal text
// alternative); the lookup outcomes never enumerate (not_found — the
// generic word that never discloses another client's orders, ambiguous
// — the word without the list); the cancel speaks its FOUR states
// (AC-7 — the cancel is never promised, never executed); the record
// form speaks the operator discipline: the intent select of the 8
// words, the TYPED identifier select of the 2 kinds with the value —
// NO phone/email kind exists (OQ-16), NO cancel execution exists (the
// OQ-3 fence), NO lookup adapter of the external systems exists
// (AG-4/OQ-18 — the lookup reads the recorded material only).

// orderServicingPanelView is the flattened panel of the case page.
type orderServicingPanelView struct {
	// Wired reports the LIVE contour: the servicing surface is bound and
	// readable — the section renders its honest boundary words even for a
	// case WITHOUT any recorded request (the contour words are the
	// interface truth).
	Wired             bool
	Requests          []orderServicingRow
	IntentOptions     []orderServicingOption
	KindOptions       []orderServicingOption
	FormTarget        string
	PhoneBoundary     string
	ConnectorBoundary string
	NoDelivery        string
	RequestRecorded   string
	// The stable word-links of the human contours (AC-16): «Позвать
	// человека» (the handoff contour 2-13 of the same page) and
	// «Вернуться в разговор» (the conversation mode surface).
	HumanLinkWord  string
	HumanLinkHref  string
	ReturnLinkWord string
	ReturnLinkHref string
}

// orderServicingRow is ONE recorded request of the panel feed: the
// ladder number, the intent word with its short form and boundary
// word, the TYPED identifier with the kind words, the lookup outcome
// word, the source pins BY NAME, the proof pins (AS RECORDED), the
// disclosure state (TEXT, never a color only), the LIVE client words
// and the LIVE cancel feasibility of the cancel intent.
type orderServicingRow struct {
	RequestNo      int64
	Intent         string
	IntentShort    string
	IntentWord     string
	Kind           string
	KindShort      string
	KindSource     string
	Value          string
	Outcome        string
	OutcomeWord    string
	Sources        string
	ProofState     string
	ProofMethod    string
	ProofIssued    string
	ProofExpires   string
	Disclosure     string
	DisclosureWord string
	RecordedAt     string
	ActorID        string
	ClientWords    []string
	Cancel         *orderServicingCancelRow
}

// orderServicingCancelRow is the LIVE four-state view of the cancel
// intent (AC-7 — the words, never only color): the recorded request,
// the absent prepared action (the OQ-3 fence with the deferred-work
// pin), the absent confirmation (never promised), the fourth state
// (the supplier touch — «затронул поставщика — человеку») and the
// AG-4 manual contour word.
type orderServicingCancelRow struct {
	RequestRecorded string
	PreparedAction  string
	Confirmed       string
	NotEditable     string
	TouchWord       string
	ManualContour   string
}

// orderServicingOption is ONE select option of the record form (the
// dictionary word with its readable label — a11y: the text, never a
// color only).
type orderServicingOption struct {
	Value string
	Text  string
	Note  string
}

// orderServicingDisclosureWords is the disclosure state dictionary of
// the panel rows (TEXT, never a color only — AC-16).
var orderServicingDisclosureWords = map[string]string{
	actionDomain.DisclosureAllowed: "раскрытие разрешено (уровень verified на момент записи)",
	actionDomain.DisclosureDenied:  "раскрытие закрыто до проверки личности (уровень verified)",
}

// orderServicingProofStateWords is the proof-state word dictionary of
// the panel rows (the BY NAME mirror of the 2-3 levels plus the step-up
// window word).
var orderServicingProofStateWords = map[string]string{
	actionDomain.ProofStateNone:     "нет записи проверки",
	actionDomain.ProofStateBasic:    "базовая проверка",
	actionDomain.ProofStateVerified: "личность подтверждена (verified)",
	actionDomain.ProofStateStepUp:   "окно step-up: проверки приостановлены",
}

// orderServicingPanelOf reads the panel material with the honest
// degradation of the case page precedent: an unwired servicing surface
// renders the zero panel — the page keeps the rest; a scope-forbidden
// member sees the panel without the servicing sections; every other
// failure fails the page honestly. The panel NEVER fails on a case
// without recorded requests — the honest «обращений по заказу не
// записано» rides the feed (AC-16: the absence is spoken).
func (s *Server) orderServicingPanelOf(ctx context.Context, actor identityDomain.Principal, tenant, caseID id.UUID) (orderServicingPanelView, error) {
	panel := orderServicingPanelView{
		IntentOptions:     orderServicingIntentOptions(),
		KindOptions:       orderServicingKindOptions(),
		PhoneBoundary:     actionDomain.ServicingPhoneSearchBoundaryWord,
		ConnectorBoundary: actionDomain.ServicingConnectorBoundaryWord,
		NoDelivery:        actionDomain.ServicingNoDeliveryWord,
		RequestRecorded:   actionDomain.ServicingRequestRecordedWord,
		HumanLinkWord:     "Позвать человека",
		HumanLinkHref:     "#handoff",
		ReturnLinkWord:    "Вернуться в разговор",
		ReturnLinkHref:    "#mode-active",
	}
	if s.orderServicing == nil {
		return panel, nil
	}
	view, err := s.orderServicing.ReadOrderServicingView(ctx, actor, tenant, caseID, s.clock.Now().UTC())
	if err != nil {
		if errors.Is(err, actionDomain.ErrForbidden) || isActionNotConfigured(err) {
			return panel, nil
		}
		return orderServicingPanelView{}, err
	}
	panel.Wired = true
	panel.Requests = make([]orderServicingRow, 0, len(view.Requests))
	for _, requestView := range view.Requests {
		row := orderServicingRow{
			RequestNo:      requestView.Request.RequestNo,
			Intent:         requestView.Request.Intent,
			IntentShort:    requestView.IntentShortWord,
			IntentWord:     actionDomain.ServicingIntentWords[requestView.Request.Intent],
			Kind:           requestView.Request.IdentifierKind,
			KindShort:      requestView.KindShortWord,
			KindSource:     requestView.KindSourceWord,
			Value:          requestView.Request.IdentifierValue,
			Outcome:        requestView.Request.LookupOutcome,
			OutcomeWord:    requestView.OutcomeWord,
			ProofState:     requestView.Request.ProofState,
			ProofMethod:    requestView.Request.ProofMethod,
			ProofIssued:    orderServicingTime(requestView.Request.ProofIssuedAt),
			ProofExpires:   orderServicingTime(requestView.Request.ProofExpiresAt),
			Disclosure:     requestView.Request.Disclosure,
			DisclosureWord: orderServicingDisclosureWords[requestView.Request.Disclosure],
			RecordedAt:     formatTime(requestView.Request.RecordedAt),
			ActorID:        requestView.Request.ActorID.String(),
			ClientWords:    requestView.ClientWords,
		}
		if requestView.Request.ProofState != "" {
			if word, ok := orderServicingProofStateWords[requestView.Request.ProofState]; ok {
				row.ProofState = requestView.Request.ProofState + " (" + word + ")"
			}
		}
		row.Sources = orderServicingSourcesWord(requestView.Sources)
		if requestView.Cancel != nil {
			row.Cancel = &orderServicingCancelRow{
				RequestRecorded: requestView.Cancel.RequestRecordedWord,
				PreparedAction:  requestView.Cancel.PreparedActionWord,
				Confirmed:       requestView.Cancel.ConfirmedWord,
				NotEditable:     requestView.Cancel.NotEditableWord,
				ManualContour:   requestView.Cancel.ManualContourWord,
			}
			if requestView.Cancel.SupplierTouched {
				row.Cancel.TouchWord = "заказ затронул поставщика: записи формирования/отправки есть — человеку"
			} else {
				row.Cancel.TouchWord = "фактов поставщика нет: impossibility-as-routine-editing не определена"
			}
		}
		panel.Requests = append(panel.Requests, row)
	}
	panel.FormTarget = "/app/tenants/" + tenant.String() + "/cases/" + caseID.String() + "/order-servicing-requests"
	return panel, nil
}

// orderServicingTime renders ONE proof-pin time (the zero time stays
// empty — the honest absence, never the 01.01.0001 artifact).
func orderServicingTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return formatTime(value)
}

// orderServicingSourcesWord renders the BY NAME source pins of the
// resolved match (the words, never the bare codes — AC-2).
func orderServicingSourcesWord(sources []string) string {
	if len(sources) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		if word, ok := actionDomain.OrderFactSourceWords[source]; ok {
			parts = append(parts, source+" ("+word+")")
			continue
		}
		parts = append(parts, source)
	}
	return strings.Join(parts, "; ")
}

// orderServicingIntentOptions builds the intent select of the record
// form (the 8 dictionary words with the readable labels — the frozen
// render order of the domain dictionary).
func orderServicingIntentOptions() []orderServicingOption {
	options := make([]orderServicingOption, 0, len(actionDomain.ServicingIntents))
	for _, intent := range actionDomain.ServicingIntents {
		options = append(options, orderServicingOption{
			Value: intent,
			Text:  actionDomain.ServicingIntentShortWords[intent],
			Note:  actionDomain.ServicingIntentWords[intent],
		})
	}
	return options
}

// orderServicingKindOptions builds the identifier kind select of the
// record form (the 2 dictionary words with the readable labels — the
// phone/email kinds do NOT exist, OQ-16).
func orderServicingKindOptions() []orderServicingOption {
	options := make([]orderServicingOption, 0, len(actionDomain.LookupIdentifierKinds))
	for _, kind := range actionDomain.LookupIdentifierKinds {
		options = append(options, orderServicingOption{
			Value: kind,
			Text:  actionDomain.LookupKindShortWords[kind],
			Note:  actionDomain.LookupKindSourceWords[kind],
		})
	}
	return options
}

// BindOrderServicing registers the Story 4.10 servicing surface: the
// machine JSON route of the ONE record command and the page PRG form
// (the narrow OrderServicingService type-assert over the action
// service — the OrderStatusService precedent).
func (s *Server) BindOrderServicing(service actionAPI.OrderServicingService) {
	s.orderServicing, _ = service.(actionAPI.OrderServicingService)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/order-servicing-requests", s.recordOrderServicingRequestAPI)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/order-servicing-requests", s.recordOrderServicingRequestForm)
}

// orderServicingProblem maps the servicing errors onto the honest
// transport ladder: the guard words (the intent/kind dictionaries, the
// typed value bound — the OQ-16 phone refusal rides them) are the 400
// family (the honest dictionary refusal), the case word is the 404,
// the scope words keep their ladder.
func (s *Server) orderServicingProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, actionDomain.ErrInvalid):
		s.problem(w, r, http.StatusBadRequest, "order_servicing_request_invalid", err.Error(), false, nil)
	case errors.Is(err, actionDomain.ErrOrderServicingNoCase):
		s.problem(w, r, http.StatusNotFound, "order_servicing_case_not_found", err.Error(), false, nil)
	case errors.Is(err, actionDomain.ErrForbidden):
		s.problem(w, r, http.StatusForbidden, "forbidden", err.Error(), false, nil)
	case errors.Is(err, actionDomain.ErrNotFound):
		s.problem(w, r, http.StatusNotFound, "not_found", "Кейс обращения не найден.", false, nil)
	default:
		s.problem(w, r, http.StatusInternalServerError, "internal", "Внутренняя ошибка обслуживания заказа.", false, nil)
	}
}

// orderServicingOutcomeJSON renders the honest JSON of the record
// outcome: the request pins (the intent word, the TYPED identifier, the
// resolved anchor, the lookup outcome, the proof pins — the state, the
// method, the two times, the disclosure), the replay flag (the
// idempotent repeat BY KEY at the same trust state) and the recorded
// word (the request fixes the ask, it never claims the result — AC-6).
func orderServicingOutcomeJSON(outcome actionAPI.OrderServicingOutcome) map[string]any {
	request := outcome.Request
	payload := map[string]any{
		"request_no":       request.RequestNo,
		"intent":           request.Intent,
		"identifier_kind":  request.IdentifierKind,
		"identifier_value": request.IdentifierValue,
		"lookup_outcome":   request.LookupOutcome,
		"proof_state":      request.ProofState,
		"proof_method":     request.ProofMethod,
		"disclosure":       request.Disclosure,
		"actor_id":         request.ActorID.String(),
		"recorded_at":      request.RecordedAt.UTC().Format(time.RFC3339),
		"replay":           outcome.Replay,
		"recorded_word":    outcome.SourceWord,
	}
	if request.ResolvedOperationID != id.UUID("") {
		payload["resolved_operation_id"] = request.ResolvedOperationID.String()
	}
	if !request.ProofIssuedAt.IsZero() {
		payload["proof_issued_at"] = request.ProofIssuedAt.UTC().Format(time.RFC3339)
	}
	if !request.ProofExpiresAt.IsZero() {
		payload["proof_expires_at"] = request.ProofExpiresAt.UTC().Format(time.RFC3339)
	}
	return payload
}

// recordOrderServicingRequestInput is the strict decode target of the
// machine route (the closed body: the intent word, the TYPED identifier
// kind and the value ONLY — no phone/email kind exists to type, the
// OQ-16 fence; no cancel execution field exists, the OQ-3 fence; no
// adapter material exists, the AG-4/OQ-18 fence).
type recordOrderServicingRequestInput struct {
	Intent          string `json:"intent"`
	IdentifierKind  string `json:"identifier_kind"`
	IdentifierValue string `json:"identifier_value"`
}

// recordOrderServicingRequestAPI answers the machine record request:
// the strict decode of the closed body, the honest ladder of the
// refusals and the replay-aware 201/200.
func (s *Server) recordOrderServicingRequestAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.orderServicing == nil {
		s.notFound(w, r, actor)
		return
	}
	var input recordOrderServicingRequestInput
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.orderServicingProblem(w, r, err)
		return
	}
	command := actionAPI.RecordOrderServicingRequestCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		Intent:          strings.TrimSpace(input.Intent),
		IdentifierKind:  strings.TrimSpace(input.IdentifierKind),
		IdentifierValue: strings.TrimSpace(input.IdentifierValue),
		Now:             s.clock.Now().UTC(),
	}
	outcome, err := s.orderServicing.RecordOrderServicingRequest(r.Context(), command)
	if err != nil {
		s.orderServicingProblem(w, r, err)
		return
	}
	status := http.StatusCreated
	if outcome.Replay {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"request": orderServicingOutcomeJSON(outcome)})
}

// recordOrderServicingRequestForm answers the page record form: the
// origin gate, the strict parse of the closed form (the intent select
// of the 8 words, the TYPED identifier select of the 2 kinds and the
// value) and the honest PRG landing (the flash word of every refusal —
// the order form precedent).
func (s *Server) recordOrderServicingRequestForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.orderServicing == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.orderServicingProblem(w, r, fmt.Errorf("%w: форма записи обращения по заказу доступна только из браузера приложения (origin).", actionDomain.ErrForbidden))
		return
	}
	if !formParse(w, r) {
		setFlashError(w, tenant, "Проверьте данные формы записи обращения по заказу.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	command := actionAPI.RecordOrderServicingRequestCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		Intent:          strings.TrimSpace(r.PostFormValue("intent")),
		IdentifierKind:  strings.TrimSpace(r.PostFormValue("identifier_kind")),
		IdentifierValue: strings.TrimSpace(r.PostFormValue("identifier_value")),
		Now:             s.clock.Now().UTC(),
	}
	outcome, err := s.orderServicing.RecordOrderServicingRequest(r.Context(), command)
	if err != nil {
		setFlashError(w, tenant, "Обращение не записано: "+err.Error())
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	flag := "servicing_request_recorded"
	if outcome.Replay {
		flag = "servicing_request_replay"
	}
	workFormDone(w, r, tenant, caseID, flag)
}
