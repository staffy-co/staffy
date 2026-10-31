package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/staffy-co/staffy/internal/action/api"
	actionCatalog "github.com/staffy-co/staffy/internal/action/catalog"
	"github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/action/port"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// Story 4.8 — the order correction application contour: the four
// lifecycle commands of the Correction Work Item over the LAST CLOSED
// blocked decision of the manager order verification (4.7). The
// correction NEVER moves the case ownership (the 2-4 terms stay
// untouched); the REREAD FENCE is the SAME live re-derivation code the
// dispatch ladder 4.5 and the 4.7 decision fence run (DeriveOrderCommitment
// Material over the fresh CaseView, OrderDigestOf, the pin comparison) —
// reused BY FUNCTION, never a new dispatch step (the 3-6 ladder is
// frozen); the CAS assignment-epoch serializes the parallel closures of
// two managers under the store locks; NOTHING is ever sent to any
// supplier and NOTHING is ever changed in ABCP (the OQ-3/AG-4 structural
// fences: the contour owns no adapter, no route, no send word, no change
// command).

// orderCorrectionScopeText is the honest scope refusal of the contour:
// the words BEFORE any material (the decision-context 2-14 precedent).
const orderCorrectionScopeText = "Открытие и переназначение коррекции требуют полномочия review: участник без полномочия review не открывает и не переназначает коррекцию."

// orderCorrectionPortsMissingText is the honest refusal of the unwired
// contour (the preview precedent — never a nil panic).
const orderCorrectionPortsMissingText = "контур коррекции заказа не подключён: сверьте конфигурацию поверхности"

type (
	OpenOrderCorrectionCommand         = api.OpenOrderCorrectionCommand
	ReassignOrderCorrectionCommand     = api.ReassignOrderCorrectionCommand
	RecordOrderCorrectionRereadCommand = api.RecordOrderCorrectionRereadCommand
	CeaseOrderCorrectionCommand        = api.CeaseOrderCorrectionCommand
	OrderCorrectionOutcome             = api.OrderCorrectionOutcome
)

// The compile-time proof of the narrow interface registration (the
// transport type-asserts api.OrderCorrectionService — the
// OrderVerificationService precedent).
var _ api.OrderCorrectionService = (*Service)(nil)

// requireOrderCorrectionPorts refuses honestly when the contour is not
// wired: the correction repository, the verification reads (the blocked
// anchor), the operation reads, the receipt reads and the LIVE case
// material are mandatory (the same ports the anchor reads and the reread
// fence re-derivation need).
func (s *Service) requireOrderCorrectionPorts() error {
	if s.corrections == nil || s.verifications == nil || s.operations == nil || s.dispatches == nil || s.caseMaterial == nil || s.members == nil || s.confirmations == nil {
		return fmt.Errorf("%s", orderCorrectionPortsMissingText)
	}
	return nil
}

// authorizeOrderCorrection is the read gate of the contour: the
// membership FIRST (a non-member is the honest 404), the review scope
// 403 BEFORE any correction material (the words-first discipline).
func (s *Service) authorizeOrderCorrection(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Review); err != nil {
		if errors.Is(err, identityDomain.ErrNotFound) {
			return tenancy.Context{}, domain.ErrNotFound
		}
		if errors.Is(err, identityDomain.ErrForbidden) {
			return tenancy.Context{}, fmt.Errorf("%w: %s", domain.ErrForbidden, orderCorrectionScopeText)
		}
		return tenancy.Context{}, err
	}
	return tenancy.New(actor.ID, tenantID)
}

// readOrderCorrectionView reads the correction of one case by the
// correction number and derives its state (the honest 404 of a missing
// correction — the RLS blindness mirror).
func (s *Service) readOrderCorrectionView(ctx context.Context, scope tenancy.Context, caseID id.UUID, correctionNo int64, now time.Time) (domain.OrderCorrection, domain.OrderCorrectionStateView, error) {
	corrections, err := s.corrections.ReadCaseCorrections(ctx, scope, caseID)
	if err != nil {
		return domain.OrderCorrection{}, domain.OrderCorrectionStateView{}, err
	}
	for _, correction := range corrections {
		if correction.CorrectionNo != correctionNo {
			continue
		}
		events, err := s.corrections.ReadCorrectionEvents(ctx, scope, caseID, correctionNo)
		if err != nil {
			return domain.OrderCorrection{}, domain.OrderCorrectionStateView{}, err
		}
		view := domain.DeriveOrderCorrectionState(correction, events, now)
		return correction, view, nil
	}
	return domain.OrderCorrection{}, domain.OrderCorrectionStateView{}, port.ErrCorrectionNotFound
}

// correctionMemberScopes resolves the LIVE scope set of one member by
// the member facts (the empty set of a non-member — the guards refuse).
func (s *Service) correctionMemberScopes(ctx context.Context, scope tenancy.Context, member id.UUID) map[identityDomain.Scope]bool {
	memberships, err := s.members.MembersFacts(ctx, identityDomain.Principal{ID: scope.PrincipalID()}, scope.TenantID())
	if err != nil {
		return nil
	}
	allowed := map[identityDomain.Scope]bool{}
	for _, membership := range memberships {
		if membership.PrincipalID == nil || *membership.PrincipalID != member || membership.Status != "active" {
			continue
		}
		scopes, ok := identityDomain.AllowedScopes(membership.Roles)
		if !ok {
			continue
		}
		for candidate := range scopes {
			allowed[candidate] = true
		}
	}
	return allowed
}

// orderBlockedAnchor reads the LIVE anchor of the open command: the
// operation must belong to THIS case, be the order action
// (create_abcp_customer_order), carry the derived state committed with
// the committed receipt (the SAME anchor discipline as the verification
// assign — the correction is built over the CREATED order only) AND the
// LAST CLOSED verification decision of that operation must be blocked
// (the positions and the reason word of the blocked event travel BY
// NAME — the SQL-truth carry of the one creation transaction; the
// original blocked event is READ, never rewritten). Every miss is the
// honest anchor refusal.
func (s *Service) orderBlockedAnchor(ctx context.Context, scope tenancy.Context, caseID, operationID id.UUID) (domain.Operation, port.ActionReceiptRow, domain.OrderVerification, domain.OrderVerificationEvent, error) {
	operation, receipt, err := s.orderCommittedAnchor(ctx, scope, caseID, operationID)
	if err != nil {
		return domain.Operation{}, port.ActionReceiptRow{}, domain.OrderVerification{}, domain.OrderVerificationEvent{}, err
	}
	tasks, err := s.verifications.ReadOperationVerifications(ctx, scope, operationID)
	if err != nil {
		return domain.Operation{}, port.ActionReceiptRow{}, domain.OrderVerification{}, domain.OrderVerificationEvent{}, err
	}
	// The LADDER filter: every correction of the operation addresses its
	// blocked decision BY the verification number pin — a closed
	// correction CONSUMED its decision (the history is the truth); the
	// NEW correction opens only by a NEWER blocked decision (the ladder
	// of the repeated correction, never a second correction of one
	// decision).
	corrections, err := s.corrections.ReadOperationCorrections(ctx, scope, operationID)
	if err != nil {
		return domain.Operation{}, port.ActionReceiptRow{}, domain.OrderVerification{}, domain.OrderVerificationEvent{}, err
	}
	addressed := int64(0)
	for _, correction := range corrections {
		if correction.VerificationNo > addressed {
			addressed = correction.VerificationNo
		}
	}
	// The LAST CLOSED blocked decision: the latest verification task of
	// the operation whose derived state is blocked AND whose number is
	// NEWER than every addressed decision (the ladder order — the
	// maximal verification number wins).
	var blockedTask *domain.OrderVerification
	var blockedEvent *domain.OrderVerificationEvent
	for i := range tasks {
		if tasks[i].VerificationNo <= addressed {
			continue
		}
		events, err := s.verifications.ReadVerificationEvents(ctx, scope, tasks[i].CaseID, tasks[i].VerificationNo)
		if err != nil {
			return domain.Operation{}, port.ActionReceiptRow{}, domain.OrderVerification{}, domain.OrderVerificationEvent{}, err
		}
		view := domain.DeriveOrderVerificationState(tasks[i], events, time.Time{})
		if view.State != domain.OrderVerificationStateBlocked {
			continue
		}
		if blockedTask == nil || tasks[i].VerificationNo > blockedTask.VerificationNo {
			taskCopy := tasks[i]
			blockedTask = &taskCopy
			if view.Decision != nil {
				decisionCopy := *view.Decision
				blockedEvent = &decisionCopy
			}
		}
	}
	if blockedTask == nil || blockedEvent == nil {
		return domain.Operation{}, port.ActionReceiptRow{}, domain.OrderVerification{}, domain.OrderVerificationEvent{}, domain.ErrOrderCorrectionNoBlockedDecision
	}
	return operation, receipt, *blockedTask, *blockedEvent, nil
}

// OpenOrderCorrection opens the correction of one committed order
// operation: the rights gate (review), the LIVE anchor read (the
// committed operation WITH the LAST CLOSED blocked verification
// decision — the positions and the reason word carry BY NAME), the
// owner validation against the LIVE member facts (the owner holds the
// review scope), the domain guard and the ONE creation transaction (the
// correction row + the opening opened event with the carried blocked
// material). The honest refusals speak BEFORE any row: the anchor
// refusal of a non-blocked contour, the 409 of the open correction, the
// guard words of the round material.
func (s *Service) OpenOrderCorrection(ctx context.Context, command OpenOrderCorrectionCommand) (OrderCorrectionOutcome, error) {
	if err := s.requireOrderCorrectionPorts(); err != nil {
		return OrderCorrectionOutcome{}, err
	}
	scope, err := s.authorizeOrderCorrection(ctx, command.Actor, command.TenantID)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	// The honest 409 of the OPEN correction FIRST (the I/O matrix order):
	// an open correction of the operation refuses the repeated open with
	// the CURRENT lifecycle words — the second competing correction is
	// never created (the anchor ladder of the closed corrections never
	// reaches this path).
	if corrections, correctionsErr := s.corrections.ReadOperationCorrections(ctx, scope, command.OperationID); correctionsErr == nil {
		for _, correction := range corrections {
			events, eventsErr := s.corrections.ReadCorrectionEvents(ctx, scope, correction.CaseID, correction.CorrectionNo)
			if eventsErr != nil {
				continue
			}
			if domain.DeriveOrderCorrectionState(correction, events, command.Now).Active() {
				return OrderCorrectionOutcome{}, s.orderCorrectionActiveConflict(ctx, scope, command.OperationID, command.Now)
			}
		}
	}
	// The LIVE anchor read: the committed operation WITH the LAST CLOSED
	// blocked decision (the SQL-truth carry of the blocked material).
	operation, receipt, blockedTask, blockedEvent, err := s.orderBlockedAnchor(ctx, scope, command.CaseID, command.OperationID)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	// The owner must be an ACTIVE member holding the review scope (the
	// LIVE truth of the identity — never a stored snapshot).
	if scopes := s.correctionMemberScopes(ctx, scope, command.Owner); !scopes[identityDomain.Review] {
		return OrderCorrectionOutcome{}, fmt.Errorf("owner не участник с полномочием review: назначьте владельца коррекции с полномочием review (живая грант-матрица identity)")
	}
	if err := domain.GuardOrderCorrectionOpen(command.Owner, command.Deadline); err != nil {
		return OrderCorrectionOutcome{}, err
	}
	deadlineAt := command.Now.Add(domain.CorrectionDeadlineWindow(command.Deadline))
	correction, err := s.corrections.AppendOrderCorrection(ctx, port.AppendOrderCorrectionParams{
		Scope:            scope,
		CaseID:           command.CaseID,
		WorkItemID:       operation.WorkItemID,
		OperationID:      operation.OperationID,
		VerificationNo:   blockedTask.VerificationNo,
		ReceiptVersionNo: receipt.ReceiptVersionNo,
		ExternalOrderID:  receipt.References.ExternalObjectID,
		CreatorID:        command.Actor.ID,
		Owner:            command.Owner,
		CorrectDeadline:  command.Deadline,
		DeadlineAt:       deadlineAt,
		Positions:        blockedEvent.Positions,
		Reason:           blockedEvent.Reason,
		Now:              command.Now,
	})
	if err != nil {
		if errors.Is(err, port.ErrCorrectionActive) {
			// The honest 409: the CURRENT lifecycle words travel with the
			// conflict (the open correction of the operation re-read under
			// the loser's eye).
			return OrderCorrectionOutcome{}, s.orderCorrectionActiveConflict(ctx, scope, command.OperationID, command.Now)
		}
		return OrderCorrectionOutcome{}, err
	}
	events, err := s.corrections.ReadCorrectionEvents(ctx, scope, command.CaseID, correction.CorrectionNo)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	view := domain.DeriveOrderCorrectionState(correction, events, command.Now)
	return OrderCorrectionOutcome{Correction: correction, View: view, Word: domain.OrderCorrectionStateText(view.State), StateText: domain.OrderCorrectionNotStartedWord}, nil
}

// orderCorrectionActiveConflict re-reads the OPEN correction of one
// operation and builds the honest 409 conflict with the CURRENT
// lifecycle words (the state word with its honest text).
func (s *Service) orderCorrectionActiveConflict(ctx context.Context, scope tenancy.Context, operationID id.UUID, now time.Time) error {
	corrections, err := s.corrections.ReadOperationCorrections(ctx, scope, operationID)
	if err != nil {
		return domain.OrderCorrectionActiveError{}
	}
	for _, correction := range corrections {
		events, err := s.corrections.ReadCorrectionEvents(ctx, scope, correction.CaseID, correction.CorrectionNo)
		if err != nil {
			continue
		}
		if view := domain.DeriveOrderCorrectionState(correction, events, now); view.Active() {
			return domain.OrderCorrectionActiveError{State: view.State, StateText: domain.OrderCorrectionStateText(view.State)}
		}
	}
	return domain.OrderCorrectionActiveError{}
}

// ReassignOrderCorrection records the owner's explicit reassignment (the
// overdue fallback — never an automatic takeover, AG-4/AD-14): the new
// round's owner and deadline word open a NEW epoch; the former owner's
// stale-epoch attempts answer the honest «коррекция переназначена». The
// history is never rewritten.
func (s *Service) ReassignOrderCorrection(ctx context.Context, command ReassignOrderCorrectionCommand) (OrderCorrectionOutcome, error) {
	if err := s.requireOrderCorrectionPorts(); err != nil {
		return OrderCorrectionOutcome{}, err
	}
	scope, err := s.authorizeOrderCorrection(ctx, command.Actor, command.TenantID)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	correction, view, err := s.readOrderCorrectionView(ctx, scope, command.CaseID, command.CorrectionNo, command.Now)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	if err := domain.GuardOrderCorrectionReassign(view.State, command.Owner, command.Deadline); err != nil {
		return OrderCorrectionOutcome{}, err
	}
	if scopes := s.correctionMemberScopes(ctx, scope, command.Owner); !scopes[identityDomain.Review] {
		return OrderCorrectionOutcome{}, fmt.Errorf("owner не участник с полномочием review: назначьте владельца коррекции с полномочием review (живая грант-матрица identity)")
	}
	deadlineAt := command.Now.Add(domain.CorrectionDeadlineWindow(command.Deadline))
	if _, err := s.corrections.AppendOrderCorrectionEvent(ctx, port.AppendOrderCorrectionEventParams{
		Scope:        scope,
		CaseID:       command.CaseID,
		CorrectionNo: command.CorrectionNo,
		Event: domain.OrderCorrectionEvent{
			Kind:            domain.OrderCorrectionEventReassigned,
			Owner:           command.Owner,
			CorrectDeadline: command.Deadline,
			DeadlineAt:      deadlineAt,
			ActorID:         command.Actor.ID,
		},
		// The reassignment is the owner's EXPLICIT decision over an OPEN
		// correction (any round): the CAS pins the app-read state and
		// epoch — a concurrent closure that closed the correction races
		// honestly; the owner's own epoch expectation never gates the
		// round change.
		ExpectedState: view.State,
		ExpectedEpoch: view.AssignmentEpoch,
		Now:           command.Now,
	}); err != nil {
		return OrderCorrectionOutcome{}, s.mapCorrectionRaced(ctx, scope, command.CaseID, command.CorrectionNo, command.Now, err)
	}
	return s.correctionOutcomeAfter(ctx, scope, correction, command.Now)
}

// RecordOrderCorrectionReread records the corrected closure — the
// manager's reread record: THE VERSION VALIDATIONS AND THE COMPOSITION
// FENCE run BEFORE the event is written (the SAME live re-derivation
// code the dispatch ladder 4.5 and the 4.7 decision fence run). The
// pinned NEW confirmed version {proposal, version, confirmation} must
// resolve to a LIVE confirmation version of THIS case belonging to the
// order action, its derived state must be CONFIRMED (a superseded
// version refuses BY NAME — AC-15), the pinned triple must match the
// confirmation's own preview pins and the LIVE composition re-derivation
// must equal the order digest of the pinned version (a divergence
// refuses with the EXPLICIT differences on the screen — AC-13: the
// hidden degradation is impossible STRUCTURALLY). The reread event
// carries the SAFE pin set only: the external object ID and the identity
// triple.
func (s *Service) RecordOrderCorrectionReread(ctx context.Context, command RecordOrderCorrectionRereadCommand) (OrderCorrectionOutcome, error) {
	if err := s.requireOrderCorrectionPorts(); err != nil {
		return OrderCorrectionOutcome{}, err
	}
	scope, err := s.authorizeOrderCorrection(ctx, command.Actor, command.TenantID)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	correction, view, err := s.readOrderCorrectionView(ctx, scope, command.CaseID, command.CorrectionNo, command.Now)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	if err := domain.GuardOrderCorrectionReread(view.State, command.Actor.ID, view.Owner, command.ExpectedAssignment,
		command.ExternalObjectID, command.ProposalID, command.VersionNo, command.ConfirmationID); err != nil {
		return OrderCorrectionOutcome{}, err
	}
	// THE VERSION VALIDATIONS: the pinned triple {proposal, version,
	// confirmation} resolves LIVE against the confirmation thread of the
	// proposal — the confirmation version that addressed EXACTLY the
	// pinned proposal version (the 3-4 thread machine: the version-pin
	// bijection); it must belong to THIS case and to the order action,
	// and its derived state must be CONFIRMED (the 3-4 derivation reused
	// — the same dictionary, the same words; a superseded/expired/
	// revoked/exhausted confirmation refuses the closure BY NAME).
	threads, err := s.confirmations.ReadPreviewConfirmations(ctx, scope, command.ProposalID)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	var pinned *domain.Confirmation
	for i := range threads {
		if threads[i].ConfirmationID == command.ConfirmationID && threads[i].PreviewVersionNo == command.VersionNo {
			copyOf := threads[i]
			pinned = &copyOf
			break
		}
	}
	if pinned == nil {
		return OrderCorrectionOutcome{}, domain.ErrNotFound
	}
	confirmation, err := s.confirmations.ReadConfirmationVersion(ctx, scope, pinned.ConfirmationID, pinned.VersionNo)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	if confirmation.CaseID != command.CaseID {
		return OrderCorrectionOutcome{}, domain.ErrNotFound
	}
	if confirmation.DefinitionID != domain.OrderActionID {
		return OrderCorrectionOutcome{}, fmt.Errorf("пин подтверждения не относится к действию заказа: коррекция закрывается подтверждением версии предложения заказа")
	}
	def, ok := actionCatalog.GetFrom(s.defs, confirmation.DefinitionID, confirmation.DefinitionVersion)
	if !ok {
		return OrderCorrectionOutcome{}, domain.ErrNotFound
	}
	confirmationEvents, err := s.confirmations.ReadConfirmationVersionEvents(ctx, scope, confirmation.ConfirmationID, confirmation.VersionNo)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	state, err := domain.DeriveConfirmationState(def.Conditions.ConsumptionRule, confirmation.ValidUntil, confirmationEvents, command.Now)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	if state == domain.ConfirmationSuperseded {
		// AC-15 BY NAME: the repeat use of the old Confirmation blocks
		// the route.
		return OrderCorrectionOutcome{}, domain.ErrOrderCorrectionSupersededConfirmation
	}
	if state != domain.ConfirmationValid {
		return OrderCorrectionOutcome{}, fmt.Errorf("подтверждение клиента не подтверждено (состояние %q): требуется подтверждённая версия для закрытия коррекции", state)
	}
	// THE COMPOSITION FENCE (the dispatch-ladder 4.5 code reused BY
	// FUNCTION): the pinned version's order digest reads from the LIVE
	// pinned preview version, the LIVE composition re-derives over the
	// fresh case view, and BOTH must equal. Any divergence — the honest
	// refusal BY NAME with the EXPLICIT differences on the screen
	// (AC-13); the event is NEVER written over an unconfirmed
	// composition.
	preview, err := s.operations.ReadPinnedPreviewVersion(ctx, scope, confirmation.PreviewID, confirmation.PreviewVersionNo)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	versionDigest, ok := preview.TypedInput["order_digest"]
	if !ok || versionDigest == "" {
		return OrderCorrectionOutcome{}, domain.ErrOrderCorrectionCompositionChanged
	}
	caseView, err := s.caseMaterial.CaseView(ctx, command.Actor, command.TenantID, command.CaseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return OrderCorrectionOutcome{}, domain.ErrNotFound
		}
		return OrderCorrectionOutcome{}, err
	}
	liveMaterial, err := DeriveOrderCommitmentMaterial(caseView)
	if err != nil {
		if domain.IsOrderDerivationRefusal(err) {
			// The LIVE derivation refuses the current composition (no
			// selection anymore, a non-proven position, mixed versions):
			// the SAME honest changed-composition word — never a masked
			// pass, never a transport failure.
			return OrderCorrectionOutcome{}, domain.ErrOrderCorrectionCompositionChanged
		}
		return OrderCorrectionOutcome{}, err
	}
	live := domain.OrderDigestOf(liveMaterial)
	if live != versionDigest {
		return OrderCorrectionOutcome{}, domain.ErrOrderCorrectionCompositionChanged
	}
	if _, err := s.corrections.AppendOrderCorrectionEvent(ctx, port.AppendOrderCorrectionEventParams{
		Scope:        scope,
		CaseID:       command.CaseID,
		CorrectionNo: command.CorrectionNo,
		Event: domain.OrderCorrectionEvent{
			Kind:             domain.OrderCorrectionEventReread,
			ExternalObjectID: command.ExternalObjectID,
			ProposalID:       command.ProposalID,
			VersionNo:        command.VersionNo,
			ConfirmationID:   command.ConfirmationID,
			ActorID:          command.Actor.ID,
		},
		// The CAS expectation rides the COMMAND's epoch (the round the
		// manager saw on the screen) — never the service's fresh read: a
		// reassignment that landed between the screen and the command is
		// the honest raced refusal, never a papered-over pass.
		ExpectedState: view.State,
		ExpectedEpoch: command.ExpectedAssignment,
		Now:           command.Now,
	}); err != nil {
		return OrderCorrectionOutcome{}, s.mapCorrectionRaced(ctx, scope, command.CaseID, command.CorrectionNo, command.Now, err)
	}
	outcome, err := s.correctionOutcomeAfter(ctx, scope, correction, command.Now)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	// The MANDATORY reread word rides the corrected outcome (AG-4): the
	// reread is the manager's words — Staffy never reads ABCP; the
	// composition was verified against the customer-confirmed version.
	outcome.StateText = domain.OrderCorrectionRereadWord
	return outcome, nil
}

// CeaseOrderCorrection records the ceased closure: the structured
// cessation reason word of the fixed three-word dictionary. The client
// refusal NEVER becomes a replacement — the replacement is the reread
// path only; the cancellation recommendation word rides the
// client_refused cessation outcome (AC-11).
func (s *Service) CeaseOrderCorrection(ctx context.Context, command CeaseOrderCorrectionCommand) (OrderCorrectionOutcome, error) {
	if err := s.requireOrderCorrectionPorts(); err != nil {
		return OrderCorrectionOutcome{}, err
	}
	scope, err := s.authorizeOrderCorrection(ctx, command.Actor, command.TenantID)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	correction, view, err := s.readOrderCorrectionView(ctx, scope, command.CaseID, command.CorrectionNo, command.Now)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	if err := domain.GuardOrderCorrectionCease(view.State, command.Actor.ID, view.Owner, command.ExpectedAssignment, command.Reason); err != nil {
		return OrderCorrectionOutcome{}, err
	}
	if _, err := s.corrections.AppendOrderCorrectionEvent(ctx, port.AppendOrderCorrectionEventParams{
		Scope:        scope,
		CaseID:       command.CaseID,
		CorrectionNo: command.CorrectionNo,
		Event: domain.OrderCorrectionEvent{
			Kind:            domain.OrderCorrectionEventCeased,
			CessationReason: command.Reason,
			ActorID:         command.Actor.ID,
		},
		// The CAS expectation rides the COMMAND's epoch (the round the
		// manager saw on the screen) — never the service's fresh read: a
		// reassignment that landed between the screen and the command is
		// the honest raced refusal, never a papered-over pass.
		ExpectedState: view.State,
		ExpectedEpoch: command.ExpectedAssignment,
		Now:           command.Now,
	}); err != nil {
		return OrderCorrectionOutcome{}, s.mapCorrectionRaced(ctx, scope, command.CaseID, command.CorrectionNo, command.Now, err)
	}
	outcome, err := s.correctionOutcomeAfter(ctx, scope, correction, command.Now)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	// The cancellation recommendation word rides the client_refused
	// cessation outcome (AC-11): the order cancellation is the contour
	// 4-10 — outside this slice; the absence of consent NEVER becomes a
	// replacement.
	if command.Reason == domain.CorrectionCessationClientRefused {
		outcome.StateText = domain.OrderCorrectionCancellationWord
	}
	return outcome, nil
}

// correctionOutcomeAfter re-reads the correction state after a
// successful write and builds the honest outcome (the derived words of
// the moment).
func (s *Service) correctionOutcomeAfter(ctx context.Context, scope tenancy.Context, correction domain.OrderCorrection, now time.Time) (OrderCorrectionOutcome, error) {
	events, err := s.corrections.ReadCorrectionEvents(ctx, scope, correction.CaseID, correction.CorrectionNo)
	if err != nil {
		return OrderCorrectionOutcome{}, err
	}
	view := domain.DeriveOrderCorrectionState(correction, events, now)
	return OrderCorrectionOutcome{Correction: correction, View: view, Word: domain.OrderCorrectionStateText(view.State)}, nil
}

// mapCorrectionRaced maps the raced CAS refusal onto the honest
// current-state words: the re-read speaks what lives NOW (the
// «коррекция переназначена» family — the parallel closure of the second
// manager lost honestly).
func (s *Service) mapCorrectionRaced(ctx context.Context, scope tenancy.Context, caseID id.UUID, correctionNo int64, now time.Time, err error) error {
	if !errors.Is(err, port.ErrCorrectionRaced) {
		return err
	}
	_, view, viewErr := s.readOrderCorrectionView(ctx, scope, caseID, correctionNo, now)
	if viewErr != nil {
		return domain.OrderCorrectionRacedError{}
	}
	return domain.OrderCorrectionRacedError{State: view.State, StateText: domain.OrderCorrectionStateText(view.State)}
}

// ReadOrderCorrectionCaseView reads the correction states of one case
// (the panel view material of step-03): every correction with its
// derived state view — the screen renders the CURRENT truth, never a
// stored status.
func (s *Service) ReadOrderCorrectionCaseView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) ([]domain.OrderCorrectionStateView, error) {
	if s.corrections == nil {
		return nil, fmt.Errorf("%s", orderCorrectionPortsMissingText)
	}
	scope, err := s.authorizeOrderCorrection(ctx, actor, tenantID)
	if err != nil {
		return nil, err
	}
	corrections, err := s.corrections.ReadCaseCorrections(ctx, scope, caseID)
	if err != nil {
		return nil, err
	}
	views := make([]domain.OrderCorrectionStateView, 0, len(corrections))
	for _, correction := range corrections {
		events, err := s.corrections.ReadCorrectionEvents(ctx, scope, caseID, correction.CorrectionNo)
		if err != nil {
			return nil, err
		}
		views = append(views, domain.DeriveOrderCorrectionState(correction, events, now))
	}
	return views, nil
}

// openOrderCorrectionOfCase resolves the OPEN correction of one case
// (the structural gates material): the derived view of the latest
// correction whose state is open — nil when no correction lives. The
// gates read the CURRENT truth (never a stored status); a NIL
// correction repository is the honest UNWIRED contour (no corrections
// exist — the gate holds nothing, the 4-7/4-8 composition roots wire
// the repository together).
func (s *Service) openOrderCorrectionOfCase(ctx context.Context, scope tenancy.Context, caseID id.UUID, now time.Time) *domain.OrderCorrectionStateView {
	if s.corrections == nil {
		return nil
	}
	corrections, err := s.corrections.ReadCaseCorrections(ctx, scope, caseID)
	if err != nil {
		return nil
	}
	for i := len(corrections) - 1; i >= 0; i-- {
		events, err := s.corrections.ReadCorrectionEvents(ctx, scope, caseID, corrections[i].CorrectionNo)
		if err != nil {
			continue
		}
		if view := domain.DeriveOrderCorrectionState(corrections[i], events, now); view.Active() {
			return &view
		}
	}
	return nil
}
