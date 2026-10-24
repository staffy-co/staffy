package httptransport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/artemiosu/staffy/internal/action/api"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Story 4.6 (step-03): the «Заказ клиента» panel of the case page — the
// Order Commitment surface over the EXISTING machines (the builder
// proposal, the 3-4 confirmation with step-up, the receipt render with
// the derived words, the AG-4 recommendation of the parked operation).
// The panel carries NO supplier-send field anywhere (the OQ-3 fence) and
// the operator never types the order material: the build form posts the
// anchors ONLY (the builder derives everything live).

// orderPanelView is the flattened panel of the case page: the build
// affordance (the recorded selections exist), the current order preview
// card (the material lines, the digest, the sum, the mandatory
// manager-check word — AC-1) with the two DIFFERENT buttons of the two
// DIFFERENT actions (the confirmation through the existing 3-4 route
// with step-up; the correction through the builder re-derivation —
// AC-16), the receipts of the order operations with the derived state
// words and the honest 1C-unknown word, and the AG-4 recommendation of
// the parked order operation (the order_creation handoff reason word).
type orderPanelView struct {
	Buildable       bool
	NoSelectionWord string
	PreviewPresent  bool
	Preview         orderPreviewCardView
	HasReceipts     bool
	Receipts        []orderReceiptRowView
	AG4Recommended  bool
	AG4Word         string
	AG4Text         string
}

// orderPreviewCardView is the confirmation card of the CURRENT order
// preview version: the material lines of the canonical render, the
// digest pin, the known sum with its boundary word, the MANDATORY
// manager-check word (AC-1) and the two DIFFERENT buttons (the confirm
// target of the existing 3-4 form; the rebuild target of the builder —
// the correction is NEVER the typed-input edit of the material).
type orderPreviewCardView struct {
	PreviewID            string
	VersionNo            int64
	DefinitionPin        string
	OrderDigest          string
	MaterialLines        []string
	KnownClientSum       string
	Currency             string
	SumBoundaryWord      string
	ManagerCheckWord     string
	TimelineText         string
	ReadinessText        string
	FreshnessReasonText  string
	Current              bool
	ConfirmTarget        string
	ConfirmLifetimeField string
	RebuildTarget        string
}

// orderReceiptRowView is one receipt row of the panel: the external
// triple, the outcome word and the DERIVED order words of the committed
// state (the client words, the manager assignment, the honest
// 1C-unknown — AC-8/AC-13/AC-14).
type orderReceiptRowView struct {
	ReceiptVersionNo      string
	OutcomeText           string
	ExternalObjectID      string
	ExternalObjectVersion string
	CreatedAt             string
	OrderState            string
	OrderStateWord        string
	OrderBoundaryWord     string
	OrderOneCWord         string
	OrderManagerWord      string
}

// orderPanelOf reads the panel material with the honest degradation of
// the case page precedent (the decision-context pattern): an unwired
// action surface renders the zero panel — the page keeps the rest; a
// scope-forbidden member sees the panel without the action sections
// (the build note only); every other failure fails the page honestly.
func (s *Server) orderPanelOf(ctx context.Context, actor identityDomain.Principal, tenant, caseID id.UUID, view casesAPI.CaseView) (orderPanelView, error) {
	if s.actionPreviews == nil {
		// The honest degradation of the unwired action surface: the panel
		// renders the build note only (the decision-context precedent).
		return prepareOrderPanel(view, api.PreviewView{}, api.AuthorizationView{}, tenant, caseID), nil
	}
	previewView, err := s.actionPreviews.ReadPreviewView(ctx, actor, tenant, caseID)
	if err != nil {
		if errors.Is(err, actionDomain.ErrForbidden) {
			return prepareOrderPanel(view, api.PreviewView{}, api.AuthorizationView{}, tenant, caseID), nil
		}
		return orderPanelView{}, err
	}
	var authView api.AuthorizationView
	if s.authorization != nil {
		authView, err = s.authorization.ReadAuthorizationView(ctx, actor, tenant, caseID, s.clock.Now().UTC())
		if err != nil {
			if errors.Is(err, actionDomain.ErrForbidden) {
				authView = api.AuthorizationView{}
			} else {
				return orderPanelView{}, err
			}
		}
	}
	return prepareOrderPanel(view, previewView, authView, tenant, caseID), nil
}

// prepareOrderPanel flattens the action answers into the panel rows: the
// CURRENT preview card of the order definition (the newest version whose
// thread pins create_abcp_customer_order), every receipt of the order
// operations and the AG-4 recommendation of the order operation that has
// NOT reached a committed receipt (the production contour parks the
// dispatch honestly — the panel recommends the manager task, the
// operator decides).
func prepareOrderPanel(view casesAPI.CaseView, previews api.PreviewView, auth api.AuthorizationView, tenant, caseID id.UUID) orderPanelView {
	panel := orderPanelView{}
	// The build affordance rides the recorded selections (the builder
	// refuses without them — the same word the panel speaks).
	if !view.Comparison.SchemaDefined || len(view.Comparison.Selections) == 0 {
		panel.NoSelectionWord = actionDomain.OrderNoSelectionWord
		return panel
	}
	panel.Buildable = true
	// The CURRENT order preview card: the newest version of the order
	// thread (the current-version flag of the card; the superseded ones
	// stay readable on the Action Preview surface).
	for _, thread := range previews.Threads {
		for _, card := range thread.Cards {
			if card.DefinitionID != actionDomain.OrderActionID || !card.Current {
				continue
			}
			panel.PreviewPresent = true
			panel.Preview = orderPreviewCardView{
				PreviewID:            card.PreviewID.String(),
				VersionNo:            card.VersionNo,
				DefinitionPin:        card.DefinitionID + "@" + card.DefinitionVersion,
				OrderDigest:          card.TypedInput["order_digest"],
				MaterialLines:        orderMaterialLines(card.TypedInput["order_material"]),
				KnownClientSum:       card.TypedInput["known_client_sum"],
				Currency:             card.TypedInput["currency"],
				SumBoundaryWord:      actionDomain.OrderSumBoundaryWord,
				ManagerCheckWord:     actionDomain.OrderManagerCheckWord,
				TimelineText:         card.TimelineText,
				ReadinessText:        card.ReadinessText,
				FreshnessReasonText:  card.FreshnessReasonText,
				Current:              card.Current,
				ConfirmTarget:        "/app/tenants/" + tenant.String() + "/cases/" + caseID.String() + "/operations/confirmations",
				ConfirmLifetimeField: "lifetime_seconds",
				RebuildTarget:        "/app/tenants/" + tenant.String() + "/cases/" + caseID.String() + "/work-items/" + card.WorkItemID.String() + "/order-proposals",
			}
			break
		}
	}
	// The receipts of the order operations with the derived words; the
	// AG-4 recommendation rides on the order operation without a
	// committed receipt (the parked dispatch — the panel recommends the
	// order_creation manager task, never an automatic send).
	panel.Receipts = []orderReceiptRowView{}
	for _, operation := range auth.Operations {
		if operation.DefinitionID != actionDomain.OrderActionID {
			continue
		}
		committed := false
		for _, receipt := range operation.Receipts {
			if receipt.Outcome == actionDomain.ReceiptOutcomeCommitted {
				committed = true
			}
			panel.HasReceipts = true
			panel.Receipts = append(panel.Receipts, orderReceiptRowView{
				ReceiptVersionNo:      strconv.FormatInt(receipt.ReceiptVersionNo, 10),
				OutcomeText:           receipt.OutcomeText,
				ExternalObjectID:      receipt.ExternalObjectID,
				ExternalObjectVersion: receipt.ExternalObjectVersion,
				CreatedAt:             formatTime(receipt.CreatedAt),
				OrderState:            receipt.OrderState,
				OrderStateWord:        receipt.OrderStateWord,
				OrderBoundaryWord:     receipt.OrderBoundaryWord,
				OrderOneCWord:         receipt.OrderOneCWord,
				OrderManagerWord:      receipt.OrderManagerWord,
			})
		}
		if !committed {
			panel.AG4Recommended = true
			panel.AG4Word = casesDomain.HandoffReasonOrderCreation
			panel.AG4Text = casesDomain.HandoffReasonText(casesDomain.HandoffReasonOrderCreation)
		}
	}
	return panel
}

// orderMaterialLines splits the canonical order material render into the
// template lines (the render is line-oriented — the positions, the sum
// line and the manager-check word; empty lines never render).
func orderMaterialLines(render string) []string {
	if strings.TrimSpace(render) == "" {
		return nil
	}
	lines := strings.Split(render, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// BindOrderProposals registers the Story 4.6 order builder surface: the
// machine JSON route and the page PRG form of the order proposal (the
// ONE-word operator ask — the anchors only, the material is derived
// LIVE by the builder; the narrow OrderProposalService type-assert over
// the preview service — the ActionCatalogService precedent).
func (s *Server) BindOrderProposals(service api.PreviewService) {
	s.orderProposals, _ = service.(api.OrderProposalService)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/order-proposals", s.prepareOrderProposalAPI)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/order-proposals", s.prepareOrderProposalForm)
}

// prepareOrderProposalAPI answers the machine order build request: the
// strict decode of the EMPTY body (the order material is NEVER a request
// field — the builder derives it live; a free field is the honest 400
// BEFORE the service is ever called) and the honest ladder of the
// machine 3-4 submit (200 built / 400 violations / 403 policy block /
// 404 anchors / 409 the attempt ladder).
func (s *Server) prepareOrderProposalAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if s.orderProposals == nil {
		s.notFound(w, r, actor)
		return
	}
	var input struct{}
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.actionPreviewsProblem(w, r, err)
		return
	}
	outcome, err := s.orderProposals.PrepareOrderProposal(r.Context(), api.PrepareOrderProposalCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.orderProblem(w, r, err)
		return
	}
	status := http.StatusOK
	switch outcome.Verdict {
	case actionDomain.VerdictInvalid:
		status = http.StatusBadRequest
	case actionDomain.VerdictRejected:
		status = http.StatusConflict
	}
	if outcome.BlockedReason != "" && status == http.StatusOK {
		status = http.StatusForbidden
	}
	writeJSON(w, status, map[string]any{"proposal": outcome})
}

// prepareOrderProposalForm answers the page build form of the order
// panel: the origin gate (the action-form discipline), the strict parse
// of the EMPTY form (the anchors ride the route — no material field
// exists to type), the builder command and the honest PRG landing with
// the flash word of every refusal (the honest builder words of the
// pre-proposal refusals included).
func (s *Server) prepareOrderProposalForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if s.orderProposals == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.orderProblem(w, r, fmt.Errorf("%w: %s", actionDomain.ErrForbidden, "форма заказа доступна только из браузера приложения (origin)."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы заказа.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	outcome, err := s.orderProposals.PrepareOrderProposal(r.Context(), api.PrepareOrderProposalCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		setFlashError(w, tenant, orderFormMessage(err))
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	if outcome.Verdict != actionDomain.VerdictValid || outcome.BlockedReason != "" {
		word := actionDomain.PolicyReasonText(outcome.BlockedReason)
		if len(outcome.Violations) > 0 {
			word = strings.Join(outcome.Violations, "; ")
		}
		setFlashError(w, tenant, "Заказ не построен: "+word)
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
		return
	}
	workFormDone(w, r, tenant, caseID, "order_proposal_built")
}

// orderFormMessage renders the honest flash word of a builder refusal
// (the domain error word IS the message; the store/transport errors keep
// their own ladder).
func orderFormMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// orderProblem maps the order builder errors onto the honest transport
// ladder (the action previews precedent): the pre-proposal refusal words
// are the 409 conflict of the honest state (the selections, the fitment
// proof, the input versions), the scope/404 words keep their ladder.
func (s *Server) orderProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
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
		s.problem(w, r, http.StatusNotFound, "not_found", "Кейс, рабочий элемент или определение заказа не найдены.", false, nil)
	default:
		s.problem(w, r, http.StatusInternalServerError, "internal", "Внутренняя ошибка построения заказа.", false, nil)
	}
}
