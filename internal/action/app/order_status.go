package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/staffy-co/staffy/internal/action/api"
	"github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/action/port"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// Story 4.9 — the order status application contour: the Supplier Order
// Fact Receipt (the TWO record commands of the human facts {formed,
// sent} over the LIVE order operation) and the Order Status Projection
// read (the PURE read-time assembly of the material of the three
// machines plus the facts). The record commands NEVER send or form
// anything anywhere (the OQ-3 structural fence: no adapter, no route,
// no send word — the fact IS the human claim of what already happened
// in ABCP, recorded BY NAME with the two times); the projection writes
// NOTHING (AC-2 — the computed representation never becomes a new
// source of truth; the material reads ride the EXISTING repositories
// of the action module: the operations, the receipts, the
// verifications, the corrections and the facts).

// orderStatusScopeText is the honest scope refusal of the contour: the
// words BEFORE any material (the decision-context 2-14 precedent).
const orderStatusScopeText = "Запись фактов поставщика и чтение статуса заказа требуют полномочия review: участник без полномочия review не записывает факты и не читает статусную панель."

// orderStatusPortsMissingText is the honest refusal of the unwired
// contour (the preview precedent — never a nil panic).
const orderStatusPortsMissingText = "контур статуса заказа не подключён: сверьте конфигурацию поверхности"

type (
	RecordOrderSupplierFactCommand = api.RecordOrderSupplierFactCommand
	OrderSupplierFactOutcome       = api.OrderSupplierFactOutcome
)

// The compile-time proof of the narrow interface registration (the
// transport type-asserts api.OrderStatusService — the
// OrderCorrectionService precedent).
var _ api.OrderStatusService = (*Service)(nil)

// requireOrderStatusPorts refuses honestly when the contour is not
// wired: the fact repository, the operation reads, the receipt reads,
// the verification reads, the correction reads and the LIVE case
// material are mandatory (the same ports the anchor reads and the
// projection assembly need).
func (s *Service) requireOrderStatusPorts() error {
	if s.orderFacts == nil || s.operations == nil || s.dispatches == nil || s.verifications == nil || s.corrections == nil || s.caseMaterial == nil || s.members == nil {
		return fmt.Errorf("%s", orderStatusPortsMissingText)
	}
	return nil
}

// authorizeOrderStatus is the read/record gate of the contour: the
// membership FIRST (a non-member is the honest 404), the review scope
// 403 BEFORE any status material (the words-first discipline).
func (s *Service) authorizeOrderStatus(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Review); err != nil {
		if errors.Is(err, identityDomain.ErrNotFound) {
			return tenancy.Context{}, domain.ErrNotFound
		}
		if errors.Is(err, identityDomain.ErrForbidden) {
			return tenancy.Context{}, fmt.Errorf("%w: %s", domain.ErrForbidden, orderStatusScopeText)
		}
		return tenancy.Context{}, err
	}
	return tenancy.New(actor.ID, tenantID)
}

// orderOperationAnchor reads the LIVE order operation of one case: the
// operation must belong to THIS case and be the order action
// (create_abcp_customer_order). The anchor is HONEST over the ledger
// state — the reality is recorded as it is: a dispatch_blocked or a
// failed operation still anchors the human facts (the guard NEVER
// blocks a «check bypass», AC-12 spirit — the panel shows BOTH facts
// and speaks both states). Every miss is the honest anchor refusal.
func (s *Service) orderOperationAnchor(ctx context.Context, scope tenancy.Context, caseID, operationID id.UUID) (domain.Operation, error) {
	operations, err := s.operations.ReadCaseOperations(ctx, scope, caseID)
	if err != nil {
		return domain.Operation{}, err
	}
	for _, operation := range operations {
		if operation.OperationID == operationID && operation.DefinitionID == domain.OrderActionID {
			return operation, nil
		}
	}
	return domain.Operation{}, domain.ErrOrderStatusNoOperation
}

// recordOrderSupplierFact is the ONE shared implementation of the two
// record commands: the rights gate (review), the LIVE operation anchor
// (the honest 404 of a foreign case — the RLS blindness mirror), the
// REPLAY check BY KEY (the idempotent repeat answers the SAVED fact —
// never a second row, AC-9), the domain guard (the kind word, the
// typed supplier order ID, the named happened_at, the
// sent-after-formed ladder of the SAME supplier order ID) and the ONE
// append (the fact_no ladder and the UNIQUE full fact key under the
// per-case advisory lock). The happened_at is the manager's NAMED
// time; the recorded_at is the server time of the append — the two
// times never mix (the provider-observed precedent of 2.7). NOTHING
// is sent or formed by this command anywhere (the OQ-3 fence).
func (s *Service) recordOrderSupplierFact(ctx context.Context, command RecordOrderSupplierFactCommand) (OrderSupplierFactOutcome, error) {
	if err := s.requireOrderStatusPorts(); err != nil {
		return OrderSupplierFactOutcome{}, err
	}
	scope, err := s.authorizeOrderStatus(ctx, command.Actor, command.TenantID)
	if err != nil {
		return OrderSupplierFactOutcome{}, err
	}
	operation, err := s.orderOperationAnchor(ctx, scope, command.CaseID, command.OperationID)
	if err != nil {
		return OrderSupplierFactOutcome{}, err
	}
	existing, err := s.orderFacts.ReadOperationSupplierFacts(ctx, scope, operation.OperationID)
	if err != nil {
		return OrderSupplierFactOutcome{}, err
	}
	// The REPLAY check BY KEY (AC-9): the fact of the same full key
	// already exists — the SAVED fact answers (the first write wins,
	// the history is never rewritten; the honest replay word rides the
	// outcome).
	for _, fact := range existing {
		if fact.Kind == command.Kind && fact.SupplierOrderID == command.SupplierOrderID {
			return OrderSupplierFactOutcome{Fact: fact, Replay: true, SourceWord: domain.OrderStatusSupplierManagerSourceWord}, nil
		}
	}
	if err := domain.GuardOrderSupplierFact(command.Kind, command.SupplierOrderID, command.HappenedAt, existing); err != nil {
		return OrderSupplierFactOutcome{}, err
	}
	fact, err := s.orderFacts.AppendOrderSupplierFact(ctx, port.AppendOrderSupplierFactParams{
		Scope:           scope,
		CaseID:          command.CaseID,
		WorkItemID:      operation.WorkItemID,
		OperationID:     operation.OperationID,
		Kind:            command.Kind,
		SupplierOrderID: command.SupplierOrderID,
		HappenedAt:      command.HappenedAt,
		ActorID:         command.Actor.ID,
		Now:             command.Now,
	})
	if err != nil {
		if errors.Is(err, port.ErrOrderSupplierFactDuplicate) {
			// The raced twin of the same command landed first: the saved
			// fact answers — the honest replay, never a masked write.
			saved, readErr := s.orderFacts.ReadOperationSupplierFacts(ctx, scope, operation.OperationID)
			if readErr == nil {
				for _, row := range saved {
					if row.Kind == command.Kind && row.SupplierOrderID == command.SupplierOrderID {
						return OrderSupplierFactOutcome{Fact: row, Replay: true, SourceWord: domain.OrderStatusSupplierManagerSourceWord}, nil
					}
				}
			}
			return OrderSupplierFactOutcome{}, err
		}
		return OrderSupplierFactOutcome{}, err
	}
	return OrderSupplierFactOutcome{Fact: fact, Replay: false, SourceWord: domain.OrderStatusSupplierManagerSourceWord}, nil
}

// RecordOrderSupplierFormed records the formed fact — the manager's
// human receipt that the order was formed at the supplier in ABCP
// (with the supplier order ID). NOTHING is formed by this command:
// the contour owns no adapter and no send path (AG-4/OQ-3).
func (s *Service) RecordOrderSupplierFormed(ctx context.Context, command RecordOrderSupplierFactCommand) (OrderSupplierFactOutcome, error) {
	command.Kind = domain.OrderSupplierFactFormed
	return s.recordOrderSupplierFact(ctx, command)
}

// RecordOrderSupplierSent records the sent fact — the manager's human
// receipt that the order was sent to the supplier. The
// sent-after-formed ladder of the SAME supplier order ID runs inside
// the shared implementation BEFORE the append (the discipline of the
// full history — the times are named as they happened, never invented).
func (s *Service) RecordOrderSupplierSent(ctx context.Context, command RecordOrderSupplierFactCommand) (OrderSupplierFactOutcome, error) {
	command.Kind = domain.OrderSupplierFactSent
	return s.recordOrderSupplierFact(ctx, command)
}

// orderOfferEstimate derives the honest AC-11 estimate material of the
// order composition: the conditions of the LIVE order material
// positions (the terms the offer of 4-5/4-6 carried). A derivation
// refusal (no selection anymore, a foreign case) is the honest EMPTY
// estimate — the projection speaks «не записана», never an invented
// term and never a masked failure of the WHOLE view.
func (s *Service) orderOfferEstimate(ctx context.Context, scope tenancy.Context, actor identityDomain.Principal, caseID id.UUID) string {
	view, err := s.caseMaterial.CaseView(ctx, actor, scope.TenantID(), caseID)
	if err != nil {
		return ""
	}
	material, err := DeriveOrderCommitmentMaterial(view)
	if err != nil {
		return ""
	}
	seen := map[string]bool{}
	conditions := []string{}
	for _, position := range material.Positions {
		term := firstTermWord(position.Conditions)
		if term != "" && !seen[term] {
			seen[term] = true
			conditions = append(conditions, term)
		}
	}
	if len(conditions) == 0 {
		return ""
	}
	out := conditions[0]
	for _, term := range conditions[1:] {
		out += "; " + term
	}
	return out
}

// firstTermWord extracts the FIRST term word of one position's
// conditions line (the estimate material of the offer — the honest
// projection never invents a term; the empty line stays empty).
func firstTermWord(conditions string) string {
	for _, part := range splitSemicolons(conditions) {
		if trimmed := trimSpace(part); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// splitSemicolons splits the conditions line by the semicolon
// separators (the canonical render format of 4-6).
func splitSemicolons(line string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(line); i++ {
		if line[i] == ';' {
			out = append(out, line[start:i])
			start = i + 1
		}
	}
	out = append(out, line[start:])
	return out
}

// trimSpace is the local ASCII space trim (the conditions material of
// the canonical render — the honest boundary of the term word).
func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// ReadOrderStatusView reads the Order Status Projection of one case
// (the panel view material): the LATEST order operation of the case
// anchors the view (the current order — the honest 404-free miss of a
// case without an order speaks «операции заказа в кейсе нет»), the
// material assembles by the EXISTING port reads (the ledger state with
// the dispatch_blocked parking cause, the committed receipt with the
// external ABCP Order ID, the verification tasks with their events,
// the corrections with their events, the supplier facts) and the PURE
// domain derivation renders the projection (ZERO writes — the
// computed representation never becomes a new source of truth, AC-2).
func (s *Service) ReadOrderStatusView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) (domain.OrderStatusProjection, error) {
	if err := s.requireOrderStatusPorts(); err != nil {
		return domain.OrderStatusProjection{}, err
	}
	scope, err := s.authorizeOrderStatus(ctx, actor, tenantID)
	if err != nil {
		return domain.OrderStatusProjection{}, err
	}
	material, err := s.assembleOrderStatusMaterial(ctx, scope, actor, caseID)
	if err != nil {
		return domain.OrderStatusProjection{}, err
	}
	return domain.DeriveOrderStatusProjection(material, now), nil
}

// assembleOrderStatusMaterial reads the WHOLE projection material of
// one case by the EXISTING port reads (the pure read-time assembly):
// the LATEST order operation (the current order of the case), its
// ledger state with the dispatch_blocked parking cause, its committed
// receipt with the external ABCP Order ID, its verification tasks with
// the events, its corrections with the events, its supplier facts and
// the honest AC-11 estimate material of the LIVE order composition.
func (s *Service) assembleOrderStatusMaterial(ctx context.Context, scope tenancy.Context, actor identityDomain.Principal, caseID id.UUID) (domain.OrderStatusMaterial, error) {
	material := domain.OrderStatusMaterial{}
	operations, err := s.operations.ReadCaseOperations(ctx, scope, caseID)
	if err != nil {
		return material, err
	}
	// The LATEST order operation of the case (the current order): the
	// operations arrive ordered by time — the LAST order-action row
	// wins (a case may carry an older superseded order; the projection
	// speaks the CURRENT one).
	var anchor *domain.Operation
	for i := range operations {
		if operations[i].DefinitionID == domain.OrderActionID {
			anchor = &operations[i]
		}
	}
	if anchor == nil {
		return material, nil
	}
	material.Operation = domain.OrderStatusOperationMaterial{
		Found:           true,
		OperationID:     anchor.OperationID,
		WorkItemID:      anchor.WorkItemID,
		Receipt:         nil,
		DispatchBlocked: false,
	}
	// The ledger state and the dispatch_blocked parking cause (the
	// AG-4 production contour word of 4-6 — the honest manual contour).
	ledger, err := s.operations.ReadOperationLedger(ctx, scope, anchor.OperationID)
	if err != nil {
		return material, err
	}
	for _, entry := range ledger {
		if entry.Kind == domain.LedgerEntryDispatchBlocked {
			material.Operation.DispatchBlocked = true
		}
	}
	// The committed receipt with the external ABCP Order ID (the
	// system receipt of the dispatch machine — the evidence of the
	// client_order_created stage).
	receipts, err := s.dispatches.ReadActionReceipts(ctx, scope, anchor.OperationID)
	if err != nil {
		return material, err
	}
	for _, receipt := range receipts {
		if receipt.Outcome == domain.ReceiptOutcomeCommitted && receipt.References.ExternalObjectID != "" {
			material.Operation.Receipt = &domain.OrderStatusReceiptPin{
				ReceiptVersionNo: receipt.ReceiptVersionNo,
				ExternalOrderID:  receipt.References.ExternalObjectID,
				CreatedAt:        receipt.CreatedAt,
			}
			break
		}
	}
	// The verification tasks of the operation with their events (the
	// 4-7 material of the fold).
	tasks, err := s.verifications.ReadOperationVerifications(ctx, scope, anchor.OperationID)
	if err != nil {
		return material, err
	}
	for _, task := range tasks {
		events, err := s.verifications.ReadVerificationEvents(ctx, scope, task.CaseID, task.VerificationNo)
		if err != nil {
			return material, err
		}
		material.Verifications = append(material.Verifications, domain.OrderStatusVerificationMaterial{
			Task:   task,
			Events: events,
		})
	}
	// The corrections of the operation with their events (the 4-8
	// material of the fold).
	corrections, err := s.corrections.ReadOperationCorrections(ctx, scope, anchor.OperationID)
	if err != nil {
		return material, err
	}
	for _, correction := range corrections {
		events, err := s.corrections.ReadCorrectionEvents(ctx, scope, correction.CaseID, correction.CorrectionNo)
		if err != nil {
			return material, err
		}
		material.Corrections = append(material.Corrections, domain.OrderStatusCorrectionMaterial{
			Correction: correction,
			Events:     events,
		})
	}
	// The supplier facts of the operation (the human receipts of this
	// slice).
	material.Facts, err = s.orderFacts.ReadOperationSupplierFacts(ctx, scope, anchor.OperationID)
	if err != nil {
		return material, err
	}
	// The honest AC-11 estimate material (the LIVE order composition —
	// the terms the offer carried; a refusal is the honest empty word).
	material.Operation.OfferEstimate = s.orderOfferEstimate(ctx, scope, actor, caseID)
	return material, nil
}
