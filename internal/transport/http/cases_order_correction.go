package httptransport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// Story 4.8 (step-03): the «Коррекция заказа» panel of the case page —
// the Correction Work Item surface over the LAST CLOSED blocked decision
// of the manager order verification (4.7). The card carries the state
// word, the owner by id, the deadline with its honest text, the overdue
// derivation and the assignment epoch; the blocked positions and the
// reason word travel BY NAME from the opened event (the SQL-truth carry
// of the open); the recommended explanation assembles the EXPLICIT
// former → current differences по позиции (the byte-stable canonical
// renders of 4-6 — the pinned line against the LIVE line), the reason
// text, the new-confirmation word, the «прежде не отправлялся» boundary
// and the honest channel word; the three actions are THREE DIFFERENT
// forms («Записать перечит» with the external object ID and the pinned
// NEW confirmed version identity; «Прекратить коррекцию» with the reason
// select; «Переназначить» with the owner and the deadline word). The
// purchasing price rides the Commercial scope of the actor. NO
// supplier-send field, route or word and NO change-action material
// exists (the OQ-3/AG-4 structural fences).

// orderCorrectionPanelView is the flattened panel of the case page.
type orderCorrectionPanelView struct {
	// Wired reports the LIVE contour: the correction surface is bound
	// and readable — the section renders its honest boundary words and
	// the open affordance even before the first correction exists (the
	// contour words are the interface truth).
	Wired                bool
	Present              bool
	Cards                []orderCorrectionCardView
	LivePresent          bool
	LiveDigest           string
	LiveSumText          string
	LiveRefusalWord      string
	DiffRows             []orderCorrectionDiffRow
	RebuildLinkText      string
	ManualContourWord    string
	RereadWord           string
	NewConfirmationWord  string
	NotSentWord          string
	ChannelWord          string
	PendingWord          string
	NotStartedWord       string
	PurchasingHiddenWord string
	OpenTarget           string
}

// orderCorrectionCardView is one correction card of the panel: the state
// word with its honest text, the round material (the owner by id, the
// deadline word with its time), the overdue derivation, the assignment
// epoch (the CAS token of every form), the carried blocked material
// (the positions with the reason word — the SQL-truth carry), the
// closure material (the reread pins or the cessation reason) and the
// three command targets.
type orderCorrectionCardView struct {
	CorrectionNo        string
	State               string
	StateText           string
	Outcome             string
	Owner               string
	DeadlineText        string
	DeadlineAt          string
	Overdue             bool
	OverdueWord         string
	AssignmentEpoch     string
	OperationID         string
	VerificationNo      string
	ExternalOrderID     string
	TechnicalLine       string
	BlockedPositions    []string
	BlockedReason       string
	BlockedReasonText   string
	ClosureExternalID   string
	ClosureProposalID   string
	ClosureVersionNo    string
	ClosureConfirmID    string
	CessationReason     string
	CessationReasonText string
	CancellationWord    string
	RereadTarget        string
	CeaseTarget         string
	ReassignTarget      string
	ExpectedAssignment  string
}

// orderCorrectionDiffRow is one EXPLICIT per-position difference of the
// recommended explanation (AC-5/AC-6/AC-13): the former line (the render
// of the operation's confirmed version) against the current line (the
// LIVE derivation) — «прежний → текущий» is TEXT, never a color only.
type orderCorrectionDiffRow struct {
	PositionKey string
	PinnedLine  string
	LiveLine    string
}

// orderCorrectionPanelOf reads the panel material with the honest
// degradation of the case page precedent: an unwired correction surface
// renders the zero panel — the page keeps the rest; a scope-forbidden
// member sees the panel without the correction sections; the
// live-material derivation refusal is SPOKEN (the honest word, never an
// empty context); every other failure fails the page honestly.
func (s *Server) orderCorrectionPanelOf(ctx context.Context, actor identityDomain.Principal, tenant, caseID id.UUID, view casesAPI.CaseView) (orderCorrectionPanelView, error) {
	panel := orderCorrectionPanelView{
		ManualContourWord:    actionDomain.OrderCorrectionManualContourWord,
		RereadWord:           actionDomain.OrderCorrectionRereadWord,
		NewConfirmationWord:  actionDomain.OrderCorrectionNewConfirmationWord,
		NotSentWord:          actionDomain.OrderCorrectionNotSentWord,
		ChannelWord:          actionDomain.OrderCorrectionChannelWord,
		PendingWord:          actionDomain.OrderCorrectionPendingWord,
		NotStartedWord:       actionDomain.OrderCorrectionNotStartedWord,
		PurchasingHiddenWord: actionDomain.OrderCorrectionPurchasingHiddenWord,
		RebuildLinkText:      "пересборка заказа — панель «Заказ клиента» (якорь #order): новая версия предложения строится существующей EDIT-лестницей над пересобранными входами",
		OpenTarget:           "/app/tenants/" + tenant.String() + "/cases/" + caseID.String() + "/order-corrections",
	}
	if s.orderCorrections == nil {
		// The honest degradation of the unwired surface: the zero panel,
		// the page keeps the rest.
		return panel, nil
	}
	// The correction states of the case (the review-scope read gate: a
	// member without the scope sees the zero panel — the words-first 403
	// rides the service ladder, the page keeps the rest).
	states, err := s.orderCorrections.ReadOrderCorrectionCaseView(ctx, actor, tenant, caseID, s.clock.Now().UTC())
	if err != nil {
		if errors.Is(err, actionDomain.ErrForbidden) || isActionNotConfigured(err) {
			return panel, nil
		}
		return orderCorrectionPanelView{}, err
	}
	panel.Wired = true
	// The LIVE material of the recommended explanation: the honest
	// refusal is SPOKEN (the composition is no longer derivable — the
	// screen speaks the word, the diff rows stay absent).
	var liveMaterial actionDomain.OrderCommitmentMaterial
	if s.orderVerifications != nil {
		liveMaterial, err = s.orderVerifications.ReadOrderVerificationLiveMaterial(ctx, actor, tenant, caseID)
		if err == nil {
			panel.LivePresent = true
			panel.LiveDigest = actionDomain.OrderDigestOf(liveMaterial)
			panel.LiveSumText = fmt.Sprintf("%.2f %s", liveMaterial.KnownClientSum, liveMaterial.Currency)
		} else if actionDomain.IsOrderDerivationRefusal(err) {
			panel.LiveRefusalWord = err.Error()
		}
	}
	// The pinned render per OPERATION (the diff anchor): the operations
	// of the case carry the (preview_id, version_no) pairs; the preview
	// cards of the order thread resolve the PINNED material renders.
	operationRenders := map[string]string{}
	if s.authorization != nil && s.actionPreviews != nil {
		authView, authErr := s.authorization.ReadAuthorizationView(ctx, actor, tenant, caseID, s.clock.Now().UTC())
		if authErr == nil {
			pinnedRenders := map[string]string{}
			previewView, previewErr := s.actionPreviews.ReadPreviewView(ctx, actor, tenant, caseID)
			if previewErr == nil {
				for _, thread := range previewView.Threads {
					for _, card := range thread.Cards {
						if card.DefinitionID == actionDomain.OrderActionID && card.TypedInput["order_digest"] != "" {
							key := card.PreviewID.String() + "\x1f" + strconv.FormatInt(card.VersionNo, 10)
							pinnedRenders[key] = card.TypedInput["order_material"]
						}
					}
				}
				for _, operation := range authView.Operations {
					if operation.DefinitionID != actionDomain.OrderActionID {
						continue
					}
					key := operation.PreviewID.String() + "\x1f" + strconv.FormatInt(operation.PreviewVersionNo, 10)
					if render, ok := pinnedRenders[key]; ok {
						operationRenders[operation.OperationID.String()] = render
					}
				}
			} else if !errors.Is(previewErr, actionDomain.ErrForbidden) && !isActionNotConfigured(previewErr) {
				return orderCorrectionPanelView{}, previewErr
			}
		} else if !errors.Is(authErr, actionDomain.ErrForbidden) && !isActionNotConfigured(authErr) {
			return orderCorrectionPanelView{}, authErr
		}
	}
	panel.Cards = s.orderCorrectionCards(states, tenant, caseID)
	if len(panel.Cards) > 0 {
		panel.Present = true
		panel.DiffRows = correctionDiffRows(operationRenders, liveMaterial, panel.LivePresent, states)
	}
	return panel, nil
}

// orderCorrectionCards flattens the correction state views into the
// panel cards with the technical line, the carried blocked material, the
// closure material and the three command targets.
func (s *Server) orderCorrectionCards(states []actionDomain.OrderCorrectionStateView, tenant, caseID id.UUID) []orderCorrectionCardView {
	cards := make([]orderCorrectionCardView, 0, len(states))
	base := "/app/tenants/" + tenant.String() + "/cases/" + caseID.String() + "/order-corrections/"
	for _, state := range states {
		card := orderCorrectionCardView{
			CorrectionNo:       strconv.FormatInt(state.Correction.CorrectionNo, 10),
			State:              state.State,
			StateText:          actionDomain.OrderCorrectionStateText(state.State),
			Outcome:            state.Outcome(),
			Owner:              state.Owner.String(),
			DeadlineText:       actionDomain.CorrectionDeadlineText(state.CorrectDeadline),
			DeadlineAt:         formatTime(state.DeadlineAt),
			Overdue:            state.State == actionDomain.OrderCorrectionStateOverdue,
			OverdueWord:        actionDomain.OrderCorrectionOverdueWord,
			AssignmentEpoch:    strconv.FormatInt(state.AssignmentEpoch, 10),
			OperationID:        state.Correction.OperationID.String(),
			VerificationNo:     strconv.FormatInt(state.Correction.VerificationNo, 10),
			ExternalOrderID:    state.Correction.ExternalOrderID,
			RereadTarget:       base + strconv.FormatInt(state.Correction.CorrectionNo, 10) + "/rereads",
			CeaseTarget:        base + strconv.FormatInt(state.Correction.CorrectionNo, 10) + "/cessations",
			ReassignTarget:     base + strconv.FormatInt(state.Correction.CorrectionNo, 10) + "/reassignments",
			ExpectedAssignment: strconv.FormatInt(state.AssignmentEpoch, 10),
		}
		// The technical line: the correction identity, the anchored
		// operation, the blocked verification number and the external
		// ABCP Order ID of the receipt.
		technical := "операция " + state.Correction.OperationID.String() +
			"; заблокированная проверка №" + card.VerificationNo +
			"; исходный внешний ABCP Order ID " + state.Correction.ExternalOrderID
		card.TechnicalLine = technical
		// The carried blocked material: the positions and the reason word
		// of the opened event (the SQL-truth carry — the original blocked
		// event is READ, never rewritten).
		if state.LastEvent != nil && state.LastEvent.Kind == actionDomain.OrderCorrectionEventOpened {
			card.BlockedPositions = state.LastEvent.Positions
			card.BlockedReason = state.LastEvent.Reason
			card.BlockedReasonText = actionDomain.VerifyBlockReasonText(state.LastEvent.Reason)
		}
		// The closure material: reread carries the SAFE pin set (the
		// external object ID and the identity triple); ceased carries the
		// cessation reason word (client_refused rides the cancellation
		// recommendation word — AC-11).
		if state.Closure != nil {
			switch state.Closure.Kind {
			case actionDomain.OrderCorrectionEventReread:
				card.ClosureExternalID = state.Closure.ExternalObjectID
				card.ClosureProposalID = state.Closure.ProposalID.String()
				card.ClosureVersionNo = strconv.FormatInt(state.Closure.VersionNo, 10)
				card.ClosureConfirmID = state.Closure.ConfirmationID.String()
			case actionDomain.OrderCorrectionEventCeased:
				card.CessationReason = state.Closure.CessationReason
				card.CessationReasonText = actionDomain.CorrectionCessationReasonText(state.Closure.CessationReason)
				if state.Closure.CessationReason == actionDomain.CorrectionCessationClientRefused {
					card.CancellationWord = actionDomain.OrderCorrectionCancellationWord
				}
			}
		}
		cards = append(cards, card)
	}
	return cards
}

// correctionDiffRows renders the EXPLICIT per-position differences of
// the recommended explanation: the pinned render lines of the corrected
// operations against the LIVE lines — по позиции (the byte-stable
// canonical renders of 4-6, the same line diff the 4-7 panel rides),
// only when the material moved. The rows appear on the panel of an OPEN
// correction (the recommended explanation material) and of a closed one
// (the audit material of what changed).
func correctionDiffRows(operationRenders map[string]string, liveMaterial actionDomain.OrderCommitmentMaterial, livePresent bool, states []actionDomain.OrderCorrectionStateView) []orderCorrectionDiffRow {
	if !livePresent {
		return nil
	}
	liveLines := map[string]string{}
	liveRender := actionDomain.OrderMaterialCanonicalText(liveMaterial)
	for _, line := range strings.Split(liveRender, "\n") {
		if key, ok := verificationPositionKeyOfLine(line); ok {
			liveLines[key] = line
		}
	}
	rows := []orderCorrectionDiffRow{}
	seen := map[string]bool{}
	for _, state := range states {
		render := operationRenders[state.Correction.OperationID.String()]
		if render == "" || seen[render] {
			continue
		}
		seen[render] = true
		for _, line := range strings.Split(render, "\n") {
			key, ok := verificationPositionKeyOfLine(line)
			if !ok {
				continue
			}
			if liveLines[key] != line {
				rows = append(rows, orderCorrectionDiffRow{
					PositionKey: key,
					PinnedLine:  line,
					LiveLine:    liveLines[key],
				})
			}
		}
	}
	return rows
}

// BindOrderCorrections registers the Story 4.8 correction surface: the
// machine JSON routes and the page PRG forms of the four commands (the
// narrow OrderCorrectionService type-assert over the action service —
// the OrderVerificationService precedent).
func (s *Server) BindOrderCorrections(service actionAPI.OrderCorrectionService) {
	s.orderCorrections, _ = service.(actionAPI.OrderCorrectionService)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/order-corrections", s.openOrderCorrectionAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/order-corrections/{correction_no}/rereads", s.rereadOrderCorrectionAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/order-corrections/{correction_no}/cessations", s.ceaseOrderCorrectionAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/order-corrections/{correction_no}/reassignments", s.reassignOrderCorrectionAPI)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/order-corrections", s.openOrderCorrectionForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/order-corrections/{correction_no}/rereads", s.rereadOrderCorrectionForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/order-corrections/{correction_no}/cessations", s.ceaseOrderCorrectionForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/order-corrections/{correction_no}/reassignments", s.reassignOrderCorrectionForm)
}

// orderCorrectionNo parses the correction number path value (the honest
// 404 of a non-number).
func orderCorrectionNo(r *http.Request) (int64, bool) {
	value, err := strconv.ParseInt(r.PathValue("correction_no"), 10, 64)
	if err != nil || value < 1 {
		return 0, false
	}
	return value, true
}

// orderCorrectionProblem maps the correction errors onto the honest
// transport ladder: the anchor word is the 409 family (the honest
// state), the open-correction authorization gate and the active
// conflict and the raced CAS are the 409 with the current words, the
// assign gate is the 409 BY NAME (AC-2), the superseded confirmation and
// the composition fence are the 409 family (AC-15/AC-13), the guard
// words are the 400 family, the scope/404 words keep their ladder.
func (s *Server) orderCorrectionProblem(w http.ResponseWriter, r *http.Request, err error) {
	var active actionDomain.OrderCorrectionActiveError
	var raced actionDomain.OrderCorrectionRacedError
	var assignGate actionDomain.OrderCorrectionAssignGateError
	switch {
	case errors.As(err, &active):
		extras := map[string]string{}
		if active.State != "" {
			extras["state"] = active.State
			extras["state_text"] = active.StateText
		}
		s.problem(w, r, http.StatusConflict, "order_correction_active", active.Error(), false, extras)
	case errors.As(err, &raced):
		extras := map[string]string{}
		if raced.State != "" {
			extras["state"] = raced.State
			extras["state_text"] = raced.StateText
		}
		s.problem(w, r, http.StatusConflict, "order_correction_raced", raced.Error(), false, extras)
	case errors.As(err, &assignGate):
		s.problem(w, r, http.StatusConflict, "order_verification_correction_open", assignGate.Error(), false, nil)
	case errors.Is(err, actionDomain.ErrOrderCorrectionNoBlockedDecision):
		s.problem(w, r, http.StatusConflict, "order_correction_no_blocked_decision", err.Error(), false, nil)
	case errors.Is(err, actionDomain.ErrOrderCorrectionOpenGate):
		s.problem(w, r, http.StatusConflict, "order_correction_open_gate", err.Error(), false, nil)
	case errors.Is(err, actionDomain.ErrOrderCorrectionSupersededConfirmation):
		s.problem(w, r, http.StatusConflict, "order_confirmation_superseded", err.Error(), false, nil)
	case errors.Is(err, actionDomain.ErrOrderCorrectionCompositionChanged):
		s.problem(w, r, http.StatusConflict, "order_composition_changed", err.Error(), false, nil)
	case errors.Is(err, actionDomain.ErrOrderNoSelection),
		errors.Is(err, actionDomain.ErrOrderFitmentNotProven),
		errors.Is(err, actionDomain.ErrOrderMixedInputVersions),
		errors.Is(err, actionDomain.ErrOrderMixedCurrency),
		errors.Is(err, actionDomain.ErrOrderOfferUnresolved),
		errors.Is(err, actionDomain.ErrOrderMaterialOversize),
		errors.Is(err, actionDomain.ErrInvalid):
		s.problem(w, r, http.StatusConflict, "order_not_buildable", err.Error(), false, nil)
	case errors.Is(err, actionDomain.ErrForbidden):
		s.problem(w, r, http.StatusForbidden, "forbidden", err.Error(), false, nil)
	case errors.Is(err, actionDomain.ErrNotFound):
		s.problem(w, r, http.StatusNotFound, "not_found", "Кейс или коррекция заказа не найдены.", false, nil)
	default:
		s.problem(w, r, http.StatusInternalServerError, "internal", "Внутренняя ошибка коррекции заказа.", false, nil)
	}
}

// openOrderCorrectionAPI answers the machine open request: the strict
// decode of the closed body (the anchors, the owner and the deadline
// word ONLY — no blocked material field exists to type: the positions
// and the reason carry BY NAME inside the ONE creation transaction).
func (s *Server) openOrderCorrectionAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.orderCorrections == nil {
		s.notFound(w, r, actor)
		return
	}
	var input struct {
		OperationID string `json:"operation_id"`
		Owner       string `json:"owner"`
		Deadline    string `json:"deadline"`
	}
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.orderCorrectionProblem(w, r, err)
		return
	}
	operationID, err := id.Parse(input.OperationID)
	if err != nil {
		s.orderCorrectionProblem(w, r, fmt.Errorf("%w: поле operation_id обязательно (UUID якорь committed-операции с заблокированным решением)", actionDomain.ErrInvalid))
		return
	}
	owner, err := id.Parse(input.Owner)
	if err != nil {
		s.orderCorrectionProblem(w, r, fmt.Errorf("%w: поле owner обязательно (UUID участника с review)", actionDomain.ErrInvalid))
		return
	}
	if input.Deadline == "" {
		s.orderCorrectionProblem(w, r, fmt.Errorf("%w: поле deadline обязательно (слово словаря {correct_4h, correct_1d})", actionDomain.ErrInvalid))
		return
	}
	outcome, err := s.orderCorrections.OpenOrderCorrection(r.Context(), actionAPI.OpenOrderCorrectionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		OperationID: operationID, Owner: owner, Deadline: input.Deadline,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.orderCorrectionProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"correction": orderCorrectionOutcomeJSON(outcome)})
}

// rereadOrderCorrectionAPI answers the machine reread request (the
// strict decode of the closed body: the external object ID, the pinned
// identity triple and the CAS epoch).
func (s *Server) rereadOrderCorrectionAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	correctionNo, ok := orderCorrectionNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderCorrections == nil {
		s.notFound(w, r, actor)
		return
	}
	var input struct {
		ExpectedAssignment int64  `json:"expected_assignment"`
		ExternalObjectID   string `json:"external_object_id"`
		ProposalID         string `json:"proposal_id"`
		VersionNo          int64  `json:"version_no"`
		ConfirmationID     string `json:"confirmation_id"`
	}
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.orderCorrectionProblem(w, r, err)
		return
	}
	proposalID, err := id.Parse(input.ProposalID)
	if err != nil {
		s.orderCorrectionProblem(w, r, fmt.Errorf("%w: поле proposal_id обязательно (UUID пин новой версии)", actionDomain.ErrInvalid))
		return
	}
	confirmationID, err := id.Parse(input.ConfirmationID)
	if err != nil {
		s.orderCorrectionProblem(w, r, fmt.Errorf("%w: поле confirmation_id обязательно (UUID пин нового подтверждения)", actionDomain.ErrInvalid))
		return
	}
	outcome, err := s.orderCorrections.RecordOrderCorrectionReread(r.Context(), actionAPI.RecordOrderCorrectionRereadCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, CorrectionNo: correctionNo,
		ExpectedAssignment: input.ExpectedAssignment, ExternalObjectID: input.ExternalObjectID,
		ProposalID: proposalID, VersionNo: input.VersionNo, ConfirmationID: confirmationID,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.orderCorrectionProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"correction": orderCorrectionOutcomeJSON(outcome)})
}

// ceaseOrderCorrectionAPI answers the machine cessation request (the
// strict decode of the closed body: the reason word and the CAS epoch).
func (s *Server) ceaseOrderCorrectionAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	correctionNo, ok := orderCorrectionNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderCorrections == nil {
		s.notFound(w, r, actor)
		return
	}
	var input struct {
		ExpectedAssignment int64  `json:"expected_assignment"`
		Reason             string `json:"reason"`
	}
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.orderCorrectionProblem(w, r, err)
		return
	}
	outcome, err := s.orderCorrections.CeaseOrderCorrection(r.Context(), actionAPI.CeaseOrderCorrectionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, CorrectionNo: correctionNo,
		ExpectedAssignment: input.ExpectedAssignment, Reason: input.Reason,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.orderCorrectionProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"correction": orderCorrectionOutcomeJSON(outcome)})
}

// reassignOrderCorrectionAPI answers the machine reassignment request
// (the strict decode of the closed body: the new round material).
func (s *Server) reassignOrderCorrectionAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	correctionNo, ok := orderCorrectionNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderCorrections == nil {
		s.notFound(w, r, actor)
		return
	}
	var input struct {
		Owner    string `json:"owner"`
		Deadline string `json:"deadline"`
	}
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.orderCorrectionProblem(w, r, err)
		return
	}
	owner, err := id.Parse(input.Owner)
	if err != nil {
		s.orderCorrectionProblem(w, r, fmt.Errorf("%w: поле owner обязательно (UUID участника с review)", actionDomain.ErrInvalid))
		return
	}
	outcome, err := s.orderCorrections.ReassignOrderCorrection(r.Context(), actionAPI.ReassignOrderCorrectionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, CorrectionNo: correctionNo,
		Owner: owner, Deadline: input.Deadline,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.orderCorrectionProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"correction": orderCorrectionOutcomeJSON(outcome)})
}

// orderCorrectionOutcomeJSON renders the honest JSON of the outcome (the
// derived words travel — the reread word of the corrected closure, the
// cancellation recommendation word of the client_refused cessation).
func orderCorrectionOutcomeJSON(outcome actionAPI.OrderCorrectionOutcome) map[string]any {
	payload := map[string]any{
		"correction_no": outcome.Correction.CorrectionNo,
		"state":         outcome.View.State,
		"state_text":    outcome.Word,
	}
	if outcome.StateText != "" {
		payload["boundary_text"] = outcome.StateText
	}
	if outcome.View.Closure != nil {
		payload["outcome"] = outcome.View.Outcome()
	}
	return payload
}

// The PRG forms of the four commands (the origin gate, the strict parse,
// the honest flash word of every refusal — the order form precedent).

// openOrderCorrectionForm answers the page open form: the origin gate,
// the strict parse of the closed form (the anchors, the owner and the
// deadline word) and the honest PRG landing.
func (s *Server) openOrderCorrectionForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.orderCorrections == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.orderCorrectionProblem(w, r, fmt.Errorf("%w: форма коррекции заказа доступна только из браузера приложения (origin).", actionDomain.ErrForbidden))
		return
	}
	if !formParse(w, r) {
		setFlashError(w, tenant, "Проверьте данные формы коррекции заказа.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	operationID, err := id.Parse(strings.TrimSpace(r.PostFormValue("operation_id")))
	if err != nil {
		setFlashError(w, tenant, "Якорь операции с заблокированным решением обязателен (UUID).")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	owner, err := id.Parse(strings.TrimSpace(r.PostFormValue("owner")))
	if err != nil {
		setFlashError(w, tenant, "Назначьте владельца коррекции (UUID участника с review).")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	deadline := strings.TrimSpace(r.PostFormValue("deadline"))
	if _, err := s.orderCorrections.OpenOrderCorrection(r.Context(), actionAPI.OpenOrderCorrectionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		OperationID: operationID, Owner: owner, Deadline: deadline,
		Now: s.clock.Now().UTC(),
	}); err != nil {
		setFlashError(w, tenant, "Коррекция не открыта: "+err.Error())
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	workFormDone(w, r, tenant, caseID, "correction_opened")
}

// rereadOrderCorrectionForm answers the page reread form («Записать
// перечит» — the corrected closure): the external object ID and the
// pinned identity triple of the NEW confirmed version.
func (s *Server) rereadOrderCorrectionForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	correctionNo, ok := orderCorrectionNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderCorrections == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.orderCorrectionProblem(w, r, fmt.Errorf("%w: форма коррекции заказа доступна только из браузера приложения (origin).", actionDomain.ErrForbidden))
		return
	}
	if !formParse(w, r) {
		setFlashError(w, tenant, "Проверьте данные формы коррекции заказа.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	epoch, err := strconv.ParseInt(strings.TrimSpace(r.PostFormValue("expected_assignment")), 10, 64)
	if err != nil || epoch < 1 {
		setFlashError(w, tenant, "Ожидаемая эпоха назначения обязательна: перечитайте коррекцию и повторите команду.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	externalObjectID := strings.TrimSpace(r.PostFormValue("external_object_id"))
	proposalID, err := id.Parse(strings.TrimSpace(r.PostFormValue("proposal_id")))
	if err != nil {
		setFlashError(w, tenant, "Пин новой версии предложения обязателен (UUID).")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	versionNo, err := strconv.ParseInt(strings.TrimSpace(r.PostFormValue("version_no")), 10, 64)
	if err != nil || versionNo < 1 {
		setFlashError(w, tenant, "Номер подтверждённой версии предложения обязателен.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	confirmationID, err := id.Parse(strings.TrimSpace(r.PostFormValue("confirmation_id")))
	if err != nil {
		setFlashError(w, tenant, "Пин нового подтверждения клиента обязателен (UUID).")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	if _, err := s.orderCorrections.RecordOrderCorrectionReread(r.Context(), actionAPI.RecordOrderCorrectionRereadCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, CorrectionNo: correctionNo,
		ExpectedAssignment: epoch, ExternalObjectID: externalObjectID,
		ProposalID: proposalID, VersionNo: versionNo, ConfirmationID: confirmationID,
		Now: s.clock.Now().UTC(),
	}); err != nil {
		setFlashError(w, tenant, "Перечит не записан: "+err.Error())
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	workFormDone(w, r, tenant, caseID, "correction_reread")
}

// ceaseOrderCorrectionForm answers the page cessation form («Прекратить
// коррекцию» — the DIFFERENT action of «Записать перечит», AC-16): the
// reason word of the three-word dictionary.
func (s *Server) ceaseOrderCorrectionForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	correctionNo, ok := orderCorrectionNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderCorrections == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.orderCorrectionProblem(w, r, fmt.Errorf("%w: форма коррекции заказа доступна только из браузера приложения (origin).", actionDomain.ErrForbidden))
		return
	}
	if !formParse(w, r) {
		setFlashError(w, tenant, "Проверьте данные формы коррекции заказа.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	epoch, err := strconv.ParseInt(strings.TrimSpace(r.PostFormValue("expected_assignment")), 10, 64)
	if err != nil || epoch < 1 {
		setFlashError(w, tenant, "Ожидаемая эпоха назначения обязательна: перечитайте коррекцию и повторите команду.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	reason := strings.TrimSpace(r.PostFormValue("reason"))
	if _, err := s.orderCorrections.CeaseOrderCorrection(r.Context(), actionAPI.CeaseOrderCorrectionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, CorrectionNo: correctionNo,
		ExpectedAssignment: epoch, Reason: reason,
		Now: s.clock.Now().UTC(),
	}); err != nil {
		setFlashError(w, tenant, "Коррекция не прекращена: "+err.Error())
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	workFormDone(w, r, tenant, caseID, "correction_ceased")
}

// reassignOrderCorrectionForm answers the page reassignment form (the
// owner's explicit fallback of the overdue round).
func (s *Server) reassignOrderCorrectionForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	correctionNo, ok := orderCorrectionNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderCorrections == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.orderCorrectionProblem(w, r, fmt.Errorf("%w: форма коррекции заказа доступна только из браузера приложения (origin).", actionDomain.ErrForbidden))
		return
	}
	if !formParse(w, r) {
		setFlashError(w, tenant, "Проверьте данные формы коррекции заказа.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	owner, err := id.Parse(strings.TrimSpace(r.PostFormValue("owner")))
	if err != nil {
		setFlashError(w, tenant, "Назначьте нового владельца коррекции (UUID участника с review).")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	deadline := strings.TrimSpace(r.PostFormValue("deadline"))
	if _, err := s.orderCorrections.ReassignOrderCorrection(r.Context(), actionAPI.ReassignOrderCorrectionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, CorrectionNo: correctionNo,
		Owner: owner, Deadline: deadline,
		Now: s.clock.Now().UTC(),
	}); err != nil {
		setFlashError(w, tenant, "Переназначение не выполнено: "+err.Error())
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	workFormDone(w, r, tenant, caseID, "correction_reassigned")
}
