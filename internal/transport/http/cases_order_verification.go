package httptransport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	actionAPI "github.com/artemiosu/staffy/internal/action/api"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Story 4.7 (step-03): the «Проверка заказа» panel of the case page — the
// manager verification surface over the Delegated Task of the committed
// receipt. The card carries the state word, the assignee and the backup
// by name, the deadline with its honest text, the overdue derivation and
// the assignment epoch; the context assembles the decision-ready material
// by READING the live panels of the case page (the evidence anchors of
// the vehicle/part-request/fitment/offers sections — the skip skeleton,
// never a duplicate); the technical line pins the operation, the
// idempotency key, the receipt and the external ABCP Order ID with the
// honest 1C-unknown word; the purchasing price rides the Commercial scope
// of the actor (the live grant matrix — the honest hiding word
// otherwise). The two decisions are TWO DIFFERENT forms («Подтвердить
// проверку» with the expected digest and epoch; «Нужна коррекция» with
// the positions and the reason select). NO supplier-send field, route or
// word exists (the OQ-3 structural fence).

// orderVerificationPanelView is the flattened panel of the case page.
type orderVerificationPanelView struct {
	// Wired reports the LIVE contour: the verification surface is bound
	// and readable — the section renders its honest boundary words and
	// the assignment affordance even before the first task exists (the
	// contour words are the interface truth, AC-13).
	Wired                bool
	Present              bool
	Cards                []orderVerificationCardView
	LivePresent          bool
	LivePositions        []orderVerificationPositionRow
	LiveDigest           string
	LiveSumText          string
	LiveRefusalWord      string
	DiffRows             []orderVerificationDiffRow
	VehicleAnchorText    string
	PartRequestText      string
	OneCWord             string
	ManualContourWord    string
	FenceWord            string
	DeliveryWord         string
	PurchasingHidden     bool
	PurchasingHiddenWord string
	AssignTarget         string
}

// orderVerificationCardView is one task card of the panel: the state word
// with its honest text, the round material (the assignee and the backup
// by id, the deadline word with its time), the overdue derivation, the
// assignment epoch (the CAS token of every form), the decision material
// (the approved digest with the evidence anchors; the blocked positions
// with the reason word), the read-time applicability of a recorded
// decision against the LIVE digest and the four command targets (the
// acknowledge / approve / block / reassign forms — the assign form lives
// on the panel level).
type orderVerificationCardView struct {
	VerificationNo      string
	State               string
	StateText           string
	Assignee            string
	BackupOwner         string
	DeadlineText        string
	DeadlineAt          string
	Overdue             bool
	OverdueWord         string
	AssignmentEpoch     string
	OperationID         string
	ReceiptVersionNo    string
	ExternalOrderID     string
	TechnicalLine       string
	DecisionDigest      string
	DecisionAnchors     []string
	DecisionPositions   []string
	DecisionReason      string
	DecisionReasonText  string
	ApplicabilityWord   string
	Applicable          bool
	AcknowledgeTarget   string
	ApproveTarget       string
	BlockTarget         string
	ReassignTarget      string
	ExpectedOrderDigest string
	ExpectedAssignment  string
}

// orderVerificationPositionRow is one LIVE position row of the
// verification context: the material the manager verifies (the brand,
// the article, the originality word, the quantity, the client price, the
// conditions) with the fitment/offer/selection digest pins and the
// purchasing price BY SCOPE (the Commercial gate of the live grant
// matrix — the honest hiding word otherwise, never a blank).
type orderVerificationPositionRow struct {
	PositionKey      string
	SupplierID       string
	Brand            string
	Article          string
	OriginalityText  string
	Quantity         string
	ClientPrice      string
	Currency         string
	Conditions       string
	FitmentDigestPin string
	OfferDigestPin   string
	SelectionDigest  string
	PurchasingText   string
}

// orderVerificationDiffRow is one EXPLICIT field difference of the stale
// composition (AC-9/AC-14): the pinned line (the render the operation
// confirmed) against the LIVE line (the fresh derivation) — по позиции,
// never a masked «changed».
type orderVerificationDiffRow struct {
	PositionKey string
	PinnedLine  string
	LiveLine    string
}

// orderVerificationPanelOf reads the panel material with the honest
// degradation of the case page precedent (the decision-context pattern):
// an unwired verification surface renders the zero panel — the page keeps
// the rest; a scope-forbidden member sees the panel without the task
// sections; the live-material derivation refusal is SPOKEN (the honest
// word, never an empty context); every other failure fails the page
// honestly.
func (s *Server) orderVerificationPanelOf(ctx context.Context, actor identityDomain.Principal, tenant, caseID id.UUID, view casesAPI.CaseView) (orderVerificationPanelView, error) {
	panel := orderVerificationPanelView{
		ManualContourWord:    actionDomain.OrderVerificationManualContourWord,
		FenceWord:            actionDomain.OrderVerificationFenceWord,
		DeliveryWord:         actionDomain.OrderVerificationDeliveryWord,
		PurchasingHiddenWord: actionDomain.OrderVerificationPurchasingHiddenWord,
		AssignTarget:         "/app/tenants/" + tenant.String() + "/cases/" + caseID.String() + "/order-verifications",
	}
	if s.orderVerifications == nil {
		// The honest degradation of the unwired surface: the zero panel,
		// the page keeps the rest.
		return panel, nil
	}
	// The task states of the case (the review-scope read gate: a member
	// without the scope sees the zero panel — the words-first 403 rides
	// the service ladder, the page keeps the rest).
	tasks, err := s.orderVerifications.ReadOrderVerificationCaseView(ctx, actor, tenant, caseID, s.clock.Now().UTC())
	if err != nil {
		if errors.Is(err, actionDomain.ErrForbidden) || isActionNotConfigured(err) {
			return panel, nil
		}
		return orderVerificationPanelView{}, err
	}
	panel.Wired = true
	// The purchasing visibility rides the LIVE Commercial scope of the
	// actor (the live grant matrix — the 4-5 optional internal data).
	panel.PurchasingHidden = true
	if members, ok := s.identity.(identityAPI2); ok {
		if err := members.Authorize(ctx, actor, tenant, identityDomain.Commercial); err == nil {
			panel.PurchasingHidden = false
		}
	}
	// The LIVE material of the verification context: the honest refusal
	// is SPOKEN (the composition the client confirmed is no longer
	// derivable — the screen speaks the word, the positions stay absent).
	liveMaterial, liveErr := s.orderVerifications.ReadOrderVerificationLiveMaterial(ctx, actor, tenant, caseID)
	if liveErr == nil {
		panel.LivePresent = true
		panel.LiveDigest = actionDomain.OrderDigestOf(liveMaterial)
		panel.LiveSumText = fmt.Sprintf("%.2f %s", liveMaterial.KnownClientSum, liveMaterial.Currency)
		panel.LivePositions = s.verificationPositionRows(liveMaterial, view, panel.PurchasingHidden)
	} else if actionDomain.IsOrderDerivationRefusal(liveErr) {
		panel.LiveRefusalWord = liveErr.Error()
	}
	// The technical line material: the operations of the case carry the
	// idempotency keys and the receipts (the same authorization read the
	// order panel rides).
	var operations []operationCardMaterial
	if s.authorization != nil {
		authView, err := s.authorization.ReadAuthorizationView(ctx, actor, tenant, caseID, s.clock.Now().UTC())
		if err == nil {
			for _, operation := range authView.Operations {
				if operation.DefinitionID == actionDomain.OrderActionID {
					operations = append(operations, operationCardMaterial{
						OperationID: operation.OperationID, IdempotencyKey: operation.IdempotencyKey,
						PreviewID: operation.PreviewID, PreviewVersionNo: operation.PreviewVersionNo,
						ExternalObjectID: orderReceiptExternalID(operation),
					})
				}
			}
		} else if !errors.Is(err, actionDomain.ErrForbidden) && !isActionNotConfigured(err) {
			return orderVerificationPanelView{}, err
		}
	}
	// The pinned render per operation (the explicit differences): the
	// preview cards of the order thread carry the typed input of every
	// version — the operation's (preview_id, version_no) resolves the
	// PINNED material.
	pinnedRenders := map[string]string{}
	if s.actionPreviews != nil {
		previewView, err := s.actionPreviews.ReadPreviewView(ctx, actor, tenant, caseID)
		if err == nil {
			for _, thread := range previewView.Threads {
				for _, card := range thread.Cards {
					if card.DefinitionID == actionDomain.OrderActionID && card.TypedInput["order_digest"] != "" {
						key := card.PreviewID.String() + "\x1f" + strconv.FormatInt(card.VersionNo, 10)
						pinnedRenders[key] = card.TypedInput["order_material"]
					}
				}
			}
		} else if !errors.Is(err, actionDomain.ErrForbidden) && !isActionNotConfigured(err) {
			return orderVerificationPanelView{}, err
		}
	}
	// The pinned render per OPERATION (the diff anchor): the operation's
	// (preview_id, version_no) resolves the render the client confirmed.
	operationRenders := map[string]string{}
	for _, operation := range operations {
		key := operation.PreviewID.String() + "\x1f" + strconv.FormatInt(operation.PreviewVersionNo, 10)
		if render, ok := pinnedRenders[key]; ok {
			operationRenders[operation.OperationID.String()] = render
		}
	}
	panel.Cards = s.orderVerificationCards(tasks, operations, pinnedRenders, tenant, caseID, liveMaterial, panel.LivePresent)
	if len(panel.Cards) > 0 {
		panel.Present = true
		panel.DiffRows = verificationDiffRows(operationRenders, liveMaterial, panel.LivePresent, tasks)
	}
	// The context anchors of the skip skeleton: the live panels of the
	// case page stay the evidence — the verification panel references
	// them (AC-3/5/6: the summary never replaces the original anchors).
	panel.VehicleAnchorText = "подтверждённый профиль и VIN — панель «Идентификация автомобиля» (якорь #vehicle)"
	panel.PartRequestText = "исходный запрос детали — панель «Запрос детали» (якорь #part-request); применимость — #fitment; предложения — #offers"
	panel.OneCWord = actionDomain.OrderOneCUnknownWord
	return panel, nil
}

// identityAPI2 is the narrow membership-scope read the purchasing gate
// uses (the transport type-assert over the composite identity service).
type identityAPI2 = interface {
	Authorize(context.Context, identityDomain.Principal, id.UUID, identityDomain.Scope) error
}

// operationCardMaterial is the technical-line material of one order
// operation (the idempotency key and the receipt external ID).
type operationCardMaterial struct {
	OperationID      id.UUID
	IdempotencyKey   string
	PreviewID        id.UUID
	PreviewVersionNo int64
	ExternalObjectID string
}

// orderReceiptExternalID resolves the external ABCP Order ID of the
// committed receipt of one operation card ("" — no committed receipt).
func orderReceiptExternalID(operation actionAPI.OperationCardView) string {
	for _, receipt := range operation.Receipts {
		if receipt.Outcome == actionDomain.ReceiptOutcomeCommitted && receipt.ExternalObjectID != "" {
			return receipt.ExternalObjectID
		}
	}
	return ""
}

// verificationPositionRows flattens the LIVE material positions with the
// purchasing price by scope (the live offer records of the case view
// carry the optional internal data — the Commercial gate decides).
func (s *Server) verificationPositionRows(material actionDomain.OrderCommitmentMaterial, view casesAPI.CaseView, purchasingHidden bool) []orderVerificationPositionRow {
	currentPurchasing := map[string]*float64{}
	for _, record := range view.Comparison.Records {
		key := record.PositionKey + "\x1f" + record.SupplierID
		if existing, ok := currentPurchasing[key]; !ok || record.OfferNo >= 0 && existing == nil {
			if existing == nil {
				currentPurchasing[key] = record.PurchasePrice
			}
		}
	}
	rows := make([]orderVerificationPositionRow, 0, len(material.Positions))
	for _, position := range material.Positions {
		purchasingText := ""
		if purchasingHidden {
			purchasingText = actionDomain.OrderVerificationPurchasingHiddenWord
		} else {
			if price := currentPurchasing[position.PositionKey+"\x1f"+position.SupplierID]; price != nil {
				purchasingText = fmt.Sprintf("%.2f %s (внутренние данные)", *price, position.Currency)
			} else {
				purchasingText = "закупочная цена не записана (опциональные внутренние данные 4-5)"
			}
		}
		rows = append(rows, orderVerificationPositionRow{
			PositionKey:      position.PositionKey,
			SupplierID:       position.SupplierID,
			Brand:            position.Brand,
			Article:          position.Article,
			OriginalityText:  actionDomain.OrderOriginalityRenderWord(position.Originality),
			Quantity:         strconv.Itoa(position.Quantity),
			ClientPrice:      fmt.Sprintf("%.2f", position.UnitClientPrice),
			Currency:         position.Currency,
			Conditions:       position.Conditions,
			FitmentDigestPin: position.FitmentDigestPin,
			OfferDigestPin:   position.OfferDigestPin,
			SelectionDigest:  position.SelectionDigest,
			PurchasingText:   purchasingText,
		})
	}
	return rows
}

// orderVerificationCards flattens the task state views into the panel
// cards with the technical line, the decision material, the read-time
// applicability and the four command targets.
func (s *Server) orderVerificationCards(tasks []actionDomain.OrderVerificationStateView, operations []operationCardMaterial, pinnedRenders map[string]string, tenant, caseID id.UUID, liveMaterial actionDomain.OrderCommitmentMaterial, livePresent bool) []orderVerificationCardView {
	cards := make([]orderVerificationCardView, 0, len(tasks))
	base := "/app/tenants/" + tenant.String() + "/cases/" + caseID.String() + "/order-verifications/"
	for _, task := range tasks {
		card := orderVerificationCardView{
			VerificationNo:     strconv.FormatInt(task.Verification.VerificationNo, 10),
			State:              task.State,
			StateText:          actionDomain.OrderVerificationStateText(task.State),
			Assignee:           task.Assignee.String(),
			BackupOwner:        task.BackupOwner.String(),
			DeadlineText:       actionDomain.VerifyDeadlineText(task.VerifyDeadline),
			DeadlineAt:         formatTime(task.DeadlineAt),
			Overdue:            task.State == actionDomain.OrderVerificationStateOverdue,
			OverdueWord:        actionDomain.OrderVerificationOverdueWord,
			AssignmentEpoch:    strconv.FormatInt(task.AssignmentEpoch, 10),
			OperationID:        task.Verification.OperationID.String(),
			ReceiptVersionNo:   strconv.FormatInt(task.Verification.ReceiptVersionNo, 10),
			ExternalOrderID:    task.Verification.ExternalOrderID,
			Applicable:         true,
			AcknowledgeTarget:  base + strconv.FormatInt(task.Verification.VerificationNo, 10) + "/acknowledgements",
			ApproveTarget:      base + strconv.FormatInt(task.Verification.VerificationNo, 10) + "/approvals",
			BlockTarget:        base + strconv.FormatInt(task.Verification.VerificationNo, 10) + "/blocks",
			ReassignTarget:     base + strconv.FormatInt(task.Verification.VerificationNo, 10) + "/reassignments",
			ExpectedAssignment: strconv.FormatInt(task.AssignmentEpoch, 10),
		}
		// The technical line: the operation identity, the idempotency key
		// and the external ABCP Order ID with the 1C-unknown word.
		technical := "операция " + task.Verification.OperationID.String()
		for _, operation := range operations {
			if operation.OperationID == task.Verification.OperationID {
				technical += "; idempotency key " + operation.IdempotencyKey
				if operation.ExternalObjectID != "" {
					technical += "; внешний ABCP Order ID " + operation.ExternalObjectID
				}
				// The pinned digest of the operation's preview version (the
				// expected digest of the approve form).
				if render, ok := pinnedRenders[operation.PreviewID.String()+"\x1f"+strconv.FormatInt(operation.PreviewVersionNo, 10)]; ok {
					_ = render
				}
				break
			}
		}
		technical += "; квитанция №" + card.ReceiptVersionNo
		card.TechnicalLine = technical
		// The decision material: approved carries the digest and the
		// evidence anchors; blocked carries the positions and the reason.
		if task.Decision != nil {
			card.DecisionDigest = task.Decision.OrderDigest
			card.DecisionAnchors = task.Decision.EvidenceAnchors
			card.DecisionPositions = task.Decision.Positions
			card.DecisionReason = task.Decision.Reason
			card.DecisionReasonText = actionDomain.VerifyBlockReasonText(task.Decision.Reason)
			// The read-time applicability of the recorded decision against
			// the LIVE digest (AC-14): a divergence marks the former
			// decision inapplicable with the honest word.
			if livePresent {
				applicability := actionDomain.DeriveOrderVerificationApplicability(task.Decision.OrderDigest, actionDomain.OrderDigestOf(liveMaterial))
				card.Applicable = applicability.Applicable
				if !applicability.Applicable {
					card.ApplicabilityWord = applicability.Word
				}
			}
		}
		cards = append(cards, card)
	}
	return cards
}

// verificationDiffRows renders the EXPLICIT field differences of the
// stale composition (AC-9): the pinned render lines against the LIVE
// lines — по позиции (the byte-stable canonical renders of 4-6), only
// when the material moved. The rows appear on the panel of a task whose
// operation's pin diverges from the live derivation.
func verificationDiffRows(operationRenders map[string]string, liveMaterial actionDomain.OrderCommitmentMaterial, livePresent bool, tasks []actionDomain.OrderVerificationStateView) []orderVerificationDiffRow {
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
	rows := []orderVerificationDiffRow{}
	seen := map[string]bool{}
	for _, task := range tasks {
		render := operationRenders[task.Verification.OperationID.String()]
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
				rows = append(rows, orderVerificationDiffRow{
					PositionKey: key,
					PinnedLine:  line,
					LiveLine:    liveLines[key],
				})
			}
		}
	}
	return rows
}

// verificationPositionKeyOfLine parses the position key of one canonical
// render line («позиция <key>: …» — the frozen 4-6 format).
func verificationPositionKeyOfLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "позиция ") {
		return "", false
	}
	rest := strings.TrimPrefix(line, "позиция ")
	if idx := strings.Index(rest, ":"); idx > 0 {
		return rest[:idx], true
	}
	return "", false
}

// BindOrderVerifications registers the Story 4.7 verification surface:
// the machine JSON routes and the page PRG forms of the five commands
// (the narrow OrderVerificationService type-assert over the action
// service — the OrderProposalService precedent).
func (s *Server) BindOrderVerifications(service actionAPI.OrderVerificationService) {
	s.orderVerifications, _ = service.(actionAPI.OrderVerificationService)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/order-verifications", s.assignOrderVerificationAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/order-verifications/{verification_no}/acknowledgements", s.acknowledgeOrderVerificationAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/order-verifications/{verification_no}/approvals", s.approveOrderVerificationAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/order-verifications/{verification_no}/blocks", s.blockOrderVerificationAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/order-verifications/{verification_no}/reassignments", s.reassignOrderVerificationAPI)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/order-verifications", s.assignOrderVerificationForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/order-verifications/{verification_no}/acknowledgements", s.acknowledgeOrderVerificationForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/order-verifications/{verification_no}/approvals", s.approveOrderVerificationForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/order-verifications/{verification_no}/blocks", s.blockOrderVerificationForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/order-verifications/{verification_no}/reassignments", s.reassignOrderVerificationForm)
}

// orderVerificationNo parses the verification number path value (the
// honest 404 of a non-number).
func orderVerificationNo(r *http.Request) (int64, bool) {
	value, err := strconv.ParseInt(r.PathValue("verification_no"), 10, 64)
	if err != nil || value < 1 {
		return 0, false
	}
	return value, true
}

// orderVerificationProblem maps the verification errors onto the honest
// transport ladder: the anchor word and the derivation refusals are the
// 409 family (the honest state), the active conflict and the raced CAS
// are the 409 with the current words, the guard words are the 400 family,
// the scope/404 words keep their ladder.
func (s *Server) orderVerificationProblem(w http.ResponseWriter, r *http.Request, err error) {
	var active actionDomain.OrderVerificationActiveError
	var raced actionDomain.OrderVerificationRacedError
	switch {
	case errors.As(err, &active):
		extras := map[string]string{}
		if active.State != "" {
			extras["state"] = active.State
			extras["state_text"] = active.StateText
		}
		s.problem(w, r, http.StatusConflict, "order_verification_active", active.Error(), false, extras)
	case errors.As(err, &raced):
		extras := map[string]string{}
		if raced.State != "" {
			extras["state"] = raced.State
			extras["state_text"] = raced.StateText
		}
		s.problem(w, r, http.StatusConflict, "order_verification_raced", raced.Error(), false, extras)
	case errors.Is(err, actionDomain.ErrOrderVerificationNoReceipt):
		s.problem(w, r, http.StatusConflict, "order_verification_no_receipt", err.Error(), false, nil)
	case errors.Is(err, actionDomain.ErrOrderVerificationCompositionChanged):
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
		s.problem(w, r, http.StatusNotFound, "not_found", "Кейс или задача проверки заказа не найдены.", false, nil)
	default:
		s.problem(w, r, http.StatusInternalServerError, "internal", "Внутренняя ошибка проверки заказа.", false, nil)
	}
}

// assignOrderVerificationAPI answers the machine assignment request: the
// strict decode of the closed body (the anchors, the members and the
// deadline word ONLY — no task material field exists to type).
func (s *Server) assignOrderVerificationAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.orderVerifications == nil {
		s.notFound(w, r, actor)
		return
	}
	command, err := decodeOrderVerificationAssign(w, r)
	if err != nil {
		s.orderVerificationProblem(w, r, err)
		return
	}
	command.Actor, command.TenantID, command.CaseID, command.Now = actor, tenant, caseID, s.clock.Now().UTC()
	outcome, err := s.orderVerifications.AssignOrderVerification(r.Context(), command)
	if err != nil {
		s.orderVerificationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"verification": orderVerificationOutcomeJSON(outcome)})
}

// decodeOrderVerificationAssign strictly decodes the assignment body: the
// closed shape with the anchors, the members and the deadline word (an
// unknown field is the honest 400 BEFORE the service is ever called).
func decodeOrderVerificationAssign(w http.ResponseWriter, r *http.Request) (actionAPI.AssignOrderVerificationCommand, error) {
	var input struct {
		WorkItemID  string `json:"work_item_id"`
		OperationID string `json:"operation_id"`
		Assignee    string `json:"assignee"`
		BackupOwner string `json:"backup_owner"`
		Deadline    string `json:"deadline"`
	}
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		return actionAPI.AssignOrderVerificationCommand{}, err
	}
	workItemID, err := id.Parse(input.WorkItemID)
	if err != nil {
		return actionAPI.AssignOrderVerificationCommand{}, fmt.Errorf("%w: поле work_item_id обязательно (UUID якорь элемента)", actionDomain.ErrInvalid)
	}
	operationID, err := id.Parse(input.OperationID)
	if err != nil {
		return actionAPI.AssignOrderVerificationCommand{}, fmt.Errorf("%w: поле operation_id обязательно (UUID якорь committed-операции)", actionDomain.ErrInvalid)
	}
	assignee, err := id.Parse(input.Assignee)
	if err != nil {
		return actionAPI.AssignOrderVerificationCommand{}, fmt.Errorf("%w: поле assignee обязательно (UUID участника с review)", actionDomain.ErrInvalid)
	}
	backup, err := id.Parse(input.BackupOwner)
	if err != nil {
		return actionAPI.AssignOrderVerificationCommand{}, fmt.Errorf("%w: поле backup_owner обязательно (UUID резервного участника)", actionDomain.ErrInvalid)
	}
	if input.Deadline == "" {
		return actionAPI.AssignOrderVerificationCommand{}, fmt.Errorf("%w: поле deadline обязательно (слово словаря {verify_4h, verify_1d})", actionDomain.ErrInvalid)
	}
	return actionAPI.AssignOrderVerificationCommand{
		WorkItemID: workItemID, OperationID: operationID,
		Assignee: assignee, BackupOwner: backup, Deadline: input.Deadline,
	}, nil
}

// acknowledgeOrderVerificationAPI answers the machine acceptance request
// (the strict decode of the closed body: the epoch expectation only).
func (s *Server) acknowledgeOrderVerificationAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	verificationNo, ok := orderVerificationNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderVerifications == nil {
		s.notFound(w, r, actor)
		return
	}
	var input struct {
		ExpectedAssignment int64 `json:"expected_assignment"`
	}
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.orderVerificationProblem(w, r, err)
		return
	}
	outcome, err := s.orderVerifications.AcknowledgeOrderVerification(r.Context(), actionAPI.AcknowledgeOrderVerificationCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, VerificationNo: verificationNo,
		ExpectedAssignment: input.ExpectedAssignment, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.orderVerificationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verification": orderVerificationOutcomeJSON(outcome)})
}

// approveOrderVerificationAPI answers the machine approval request (the
// strict decode of the closed body: the epoch expectation and the
// expected order digest — the decision fence reads BOTH).
func (s *Server) approveOrderVerificationAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	verificationNo, ok := orderVerificationNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderVerifications == nil {
		s.notFound(w, r, actor)
		return
	}
	var input struct {
		ExpectedAssignment  int64  `json:"expected_assignment"`
		ExpectedOrderDigest string `json:"expected_order_digest"`
	}
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.orderVerificationProblem(w, r, err)
		return
	}
	outcome, err := s.orderVerifications.ApproveOrderVerification(r.Context(), actionAPI.ApproveOrderVerificationCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, VerificationNo: verificationNo,
		ExpectedAssignment: input.ExpectedAssignment, ExpectedOrderDigest: input.ExpectedOrderDigest,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.orderVerificationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verification": orderVerificationOutcomeJSON(outcome)})
}

// blockOrderVerificationAPI answers the machine correction request (the
// strict decode of the closed body: the positions and the reason word).
func (s *Server) blockOrderVerificationAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	verificationNo, ok := orderVerificationNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderVerifications == nil {
		s.notFound(w, r, actor)
		return
	}
	var input struct {
		ExpectedAssignment int64    `json:"expected_assignment"`
		Positions          []string `json:"positions"`
		Reason             string   `json:"reason"`
	}
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.orderVerificationProblem(w, r, err)
		return
	}
	outcome, err := s.orderVerifications.BlockOrderVerification(r.Context(), actionAPI.BlockOrderVerificationCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, VerificationNo: verificationNo,
		ExpectedAssignment: input.ExpectedAssignment, Positions: input.Positions, Reason: input.Reason,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.orderVerificationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verification": orderVerificationOutcomeJSON(outcome)})
}

// reassignOrderVerificationAPI answers the machine reassignment request
// (the strict decode of the closed body: the new round material).
func (s *Server) reassignOrderVerificationAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	verificationNo, ok := orderVerificationNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderVerifications == nil {
		s.notFound(w, r, actor)
		return
	}
	var input struct {
		Assignee    string `json:"assignee"`
		BackupOwner string `json:"backup_owner"`
		Deadline    string `json:"deadline"`
	}
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.orderVerificationProblem(w, r, err)
		return
	}
	assignee, err := id.Parse(input.Assignee)
	if err != nil {
		s.orderVerificationProblem(w, r, fmt.Errorf("%w: поле assignee обязательно (UUID)", actionDomain.ErrInvalid))
		return
	}
	backup, err := id.Parse(input.BackupOwner)
	if err != nil {
		s.orderVerificationProblem(w, r, fmt.Errorf("%w: поле backup_owner обязательно (UUID)", actionDomain.ErrInvalid))
		return
	}
	outcome, err := s.orderVerifications.ReassignOrderVerification(r.Context(), actionAPI.ReassignOrderVerificationCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, VerificationNo: verificationNo,
		Assignee: assignee, BackupOwner: backup, Deadline: input.Deadline,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.orderVerificationProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verification": orderVerificationOutcomeJSON(outcome)})
}

// orderVerificationOutcomeJSON renders the honest JSON of the outcome
// (the derived words travel — the manual-contour word of the approved
// decision, the correction word of the blocked decision).
func orderVerificationOutcomeJSON(outcome actionAPI.OrderVerificationOutcome) map[string]any {
	payload := map[string]any{
		"verification_no": outcome.Task.VerificationNo,
		"state":           outcome.View.State,
		"state_text":      outcome.Word,
	}
	if outcome.StateText != "" {
		payload["boundary_text"] = outcome.StateText
	}
	if outcome.View.Decision != nil {
		payload["decision_digest"] = outcome.View.Decision.OrderDigest
	}
	return payload
}

// The PRG forms of the five commands (the origin gate, the strict parse,
// the honest flash word of every refusal — the order form precedent).

// assignOrderVerificationForm answers the page assignment form: the
// origin gate, the strict parse of the closed form (the anchors, the
// members and the deadline word) and the honest PRG landing.
func (s *Server) assignOrderVerificationForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.orderVerifications == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.orderVerificationProblem(w, r, fmt.Errorf("%w: форма проверки заказа доступна только из браузера приложения (origin).", actionDomain.ErrForbidden))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы проверки заказа.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	command, err := orderVerificationAssignForm(w, r, tenant, caseID)
	if err == nil {
		command.Actor, command.Now = actor, s.clock.Now().UTC()
		_, err = s.orderVerifications.AssignOrderVerification(r.Context(), command)
	}
	if err != nil {
		setFlashError(w, tenant, "Задача проверки не создана: "+err.Error())
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	workFormDone(w, r, tenant, caseID, "verification_assigned")
}

// orderVerificationAssignForm parses the assignment form fields (the
// strict closed set — no material field exists to type).
func orderVerificationAssignForm(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID) (actionAPI.AssignOrderVerificationCommand, error) {
	workItemID, err := id.Parse(strings.TrimSpace(r.PostFormValue("work_item_id")))
	if err != nil {
		return actionAPI.AssignOrderVerificationCommand{}, fmt.Errorf("%w: якорь элемента обязателен", actionDomain.ErrInvalid)
	}
	operationID, err := id.Parse(strings.TrimSpace(r.PostFormValue("operation_id")))
	if err != nil {
		return actionAPI.AssignOrderVerificationCommand{}, fmt.Errorf("%w: якорь операции обязателен", actionDomain.ErrInvalid)
	}
	assignee, err := id.Parse(strings.TrimSpace(r.PostFormValue("assignee")))
	if err != nil {
		return actionAPI.AssignOrderVerificationCommand{}, fmt.Errorf("%w: назначьте менеджера (UUID участника)", actionDomain.ErrInvalid)
	}
	backup, err := id.Parse(strings.TrimSpace(r.PostFormValue("backup_owner")))
	if err != nil {
		return actionAPI.AssignOrderVerificationCommand{}, fmt.Errorf("%w: укажите резервного участника", actionDomain.ErrInvalid)
	}
	deadline := strings.TrimSpace(r.PostFormValue("deadline"))
	if deadline == "" {
		return actionAPI.AssignOrderVerificationCommand{}, fmt.Errorf("%w: выберите слово дедлайна", actionDomain.ErrInvalid)
	}
	return actionAPI.AssignOrderVerificationCommand{
		TenantID: tenant, CaseID: caseID, WorkItemID: workItemID, OperationID: operationID,
		Assignee: assignee, BackupOwner: backup, Deadline: deadline,
	}, nil
}

// acknowledgeOrderVerificationForm answers the page acceptance form (the
// assignee's own act).
func (s *Server) acknowledgeOrderVerificationForm(w http.ResponseWriter, r *http.Request) {
	s.orderVerificationEpochForm(w, r, "acknowledge")
}

// approveOrderVerificationForm answers the page approval form («Подтвердить
// проверку» — the DIFFERENT action of «Нужна коррекция», AC-16).
func (s *Server) approveOrderVerificationForm(w http.ResponseWriter, r *http.Request) {
	s.orderVerificationEpochForm(w, r, "approve")
}

// orderVerificationEpochForm handles the two single-epoch forms (the
// acknowledge and the approve): the origin gate, the strict parse of the
// epoch expectation (the approve adds the expected digest — the pinned
// version the manager verified) and the honest PRG landing.
func (s *Server) orderVerificationEpochForm(w http.ResponseWriter, r *http.Request, kind string) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	verificationNo, ok := orderVerificationNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderVerifications == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.orderVerificationProblem(w, r, fmt.Errorf("%w: форма проверки заказа доступна только из браузера приложения (origin).", actionDomain.ErrForbidden))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы проверки заказа.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	epoch, err := strconv.ParseInt(strings.TrimSpace(r.PostFormValue("expected_assignment")), 10, 64)
	if err != nil || epoch < 1 {
		setFlashError(w, tenant, "Ожидаемая эпоха назначения обязательна: перечитайте задачу и повторите команду.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	var commandErr error
	if kind == "acknowledge" {
		_, commandErr = s.orderVerifications.AcknowledgeOrderVerification(r.Context(), actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: actor, TenantID: tenant, CaseID: caseID, VerificationNo: verificationNo,
			ExpectedAssignment: epoch, Now: s.clock.Now().UTC(),
		})
		if commandErr == nil {
			workFormDone(w, r, tenant, caseID, "verification_acknowledged")
			return
		}
	} else {
		digest := strings.TrimSpace(r.PostFormValue("expected_order_digest"))
		if digest == "" {
			setFlashError(w, tenant, "Пин точной версии заказа обязателен: перечитайте материал и повторите подтверждение.")
			http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
			return
		}
		var outcome actionAPI.OrderVerificationOutcome
		outcome, commandErr = s.orderVerifications.ApproveOrderVerification(r.Context(), actionAPI.ApproveOrderVerificationCommand{
			Actor: actor, TenantID: tenant, CaseID: caseID, VerificationNo: verificationNo,
			ExpectedAssignment: epoch, ExpectedOrderDigest: digest, Now: s.clock.Now().UTC(),
		})
		if commandErr == nil {
			// The MANDATORY manual-contour word rides the approved status
			// flag of the page (AC-10/AC-11): the formation and the send
			// stay the manager's manual acts — the word renders in the
			// verification section by name.
			workFormDone(w, r, tenant, caseID, "verification_approved")
			return
		}
		_ = outcome
	}
	if commandErr != nil {
		setFlashError(w, tenant, "Проверка не выполнена: "+commandErr.Error())
	}
	http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
}

// blockOrderVerificationForm answers the page correction form («Нужна
// коррекция» — the DIFFERENT action, AC-16): the positions and the
// reason word of the five-word dictionary.
func (s *Server) blockOrderVerificationForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	verificationNo, ok := orderVerificationNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderVerifications == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.orderVerificationProblem(w, r, fmt.Errorf("%w: форма проверки заказа доступна только из браузера приложения (origin).", actionDomain.ErrForbidden))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы проверки заказа.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	epoch, err := strconv.ParseInt(strings.TrimSpace(r.PostFormValue("expected_assignment")), 10, 64)
	if err != nil || epoch < 1 {
		setFlashError(w, tenant, "Ожидаемая эпоха назначения обязательна: перечитайте задачу и повторите команду.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	positions := r.PostForm["positions"]
	reason := strings.TrimSpace(r.PostFormValue("reason"))
	outcome, err := s.orderVerifications.BlockOrderVerification(r.Context(), actionAPI.BlockOrderVerificationCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, VerificationNo: verificationNo,
		ExpectedAssignment: epoch, Positions: positions, Reason: reason,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		setFlashError(w, tenant, "Коррекция не записана: "+err.Error())
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	workFormDone(w, r, tenant, caseID, "verification_blocked")
	_ = outcome
}

// reassignOrderVerificationForm answers the page reassignment form (the
// owner's explicit fallback of the overdue round).
func (s *Server) reassignOrderVerificationForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	verificationNo, ok := orderVerificationNo(r)
	if !ok {
		s.notFound(w, r, actor)
		return
	}
	if s.orderVerifications == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.orderVerificationProblem(w, r, fmt.Errorf("%w: форма проверки заказа доступна только из браузера приложения (origin).", actionDomain.ErrForbidden))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы проверки заказа.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	assignee, err := id.Parse(strings.TrimSpace(r.PostFormValue("assignee")))
	if err != nil {
		setFlashError(w, tenant, "Назначьте нового менеджера (UUID участника).")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	backup, err := id.Parse(strings.TrimSpace(r.PostFormValue("backup_owner")))
	if err != nil {
		setFlashError(w, tenant, "Укажите резервного участника раунда.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	deadline := strings.TrimSpace(r.PostFormValue("deadline"))
	_, err = s.orderVerifications.ReassignOrderVerification(r.Context(), actionAPI.ReassignOrderVerificationCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, VerificationNo: verificationNo,
		Assignee: assignee, BackupOwner: backup, Deadline: deadline,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		setFlashError(w, tenant, "Переназначение не выполнено: "+err.Error())
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	workFormDone(w, r, tenant, caseID, "verification_reassigned")
}
