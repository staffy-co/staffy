package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// Story 4.7 — the manager order verification application contour: the
// five lifecycle commands of the Delegated Task over the committed
// receipt of create_abcp_customer_order (4.6). The task NEVER moves the
// case ownership (the 2-4 terms stay untouched); the DECISION FENCE of
// the approve command is the SAME live re-derivation code the dispatch
// ladder 4.5 runs (DeriveOrderCommitmentMaterial over the fresh CaseView,
// OrderDigestOf, the pin comparison) — reused BY FUNCTION, never a new
// dispatch step (the 3-6 ladder is frozen); the CAS assignment-epoch
// serializes the parallel decisions of two managers under the store
// locks; NOTHING is ever sent to any supplier (the OQ-3 structural fence:
// the contour owns no adapter, no route, no send word).

// orderVerificationScopeText is the honest scope refusal of the contour:
// the words BEFORE any material (the decision-context 2-14 precedent).
const orderVerificationScopeText = "Назначение и переназначение проверки заказа требуют полномочия review: участник без полномочия review не назначает проверяющего менеджера."

// orderVerificationPortsMissingText is the honest refusal of the unwired
// contour (the preview precedent).
const orderVerificationPortsMissingText = "Контур проверки заказа не сконфигурирован: живые источники не подключены, команда честно отклоняется."

// AssignOrderVerificationCommand is the owner's assignment ask: the
// anchors (the case, the work item, the COMMITTED operation of the order
// action), the assignee and the backup (validated against the LIVE
// member facts with the Review scope) and the deadline word of the
// approved policy dictionary. The task material is NEVER an input — the
// receipt identity and the external ABCP Order ID pin from the LIVE
// operation read.
type AssignOrderVerificationCommand struct {
	Actor       identityDomain.Principal
	TenantID    id.UUID
	CaseID      id.UUID
	WorkItemID  id.UUID
	OperationID id.UUID
	Assignee    id.UUID
	BackupOwner id.UUID
	Deadline    string
	Now         time.Time
}

// AcknowledgeOrderVerificationCommand is the assignee's explicit
// acceptance: the task anchor, the CAS assignment-epoch expectation (the
// event_no of the assigned round the manager saw) — the assignment or a
// notification NEVER counts (the server truth: actor=assignee).
type AcknowledgeOrderVerificationCommand struct {
	Actor              identityDomain.Principal
	TenantID           id.UUID
	CaseID             id.UUID
	VerificationNo     int64
	ExpectedAssignment int64
	Now                time.Time
}

// ApproveOrderVerificationCommand is the «Подтвердить проверку» decision:
// the CAS epoch expectation AND the expected order digest — the 64-hex
// pin of the exact version the manager verified. The fence re-derives
// the LIVE composition and compares BOTH with the operation's pin: any
// divergence refuses the decision BY NAME (AC-9) — the event is never
// written over a stale version.
type ApproveOrderVerificationCommand struct {
	Actor               identityDomain.Principal
	TenantID            id.UUID
	CaseID              id.UUID
	VerificationNo      int64
	ExpectedAssignment  int64
	ExpectedOrderDigest string
	Now                 time.Time
}

// BlockOrderVerificationCommand is the «Нужна коррекция» decision: the
// affected position keys (validated against the composition of the
// OPERATION — the pinned canonical render) and the structured reason word
// of the five-word dictionary. The manager never types a free reason.
type BlockOrderVerificationCommand struct {
	Actor              identityDomain.Principal
	TenantID           id.UUID
	CaseID             id.UUID
	VerificationNo     int64
	ExpectedAssignment int64
	Positions          []string
	Reason             string
	Now                time.Time
}

// ReassignOrderVerificationCommand is the owner's explicit reassignment
// (the overdue fallback — never an automatic takeover): the new round's
// assignee, backup and deadline word. The new assigned event opens a NEW
// epoch — the former assignee's stale-epoch attempts answer the honest
// «задача переназначена».
type ReassignOrderVerificationCommand struct {
	Actor          identityDomain.Principal
	TenantID       id.UUID
	CaseID         id.UUID
	VerificationNo int64
	Assignee       id.UUID
	BackupOwner    id.UUID
	Deadline       string
	Now            time.Time
}

// OrderVerificationOutcome is the honest result of every command: the
// derived state view of the task AFTER the write (or the current state
// of a refusal when the transport wants it) and the derived words of the
// moment. The outcome NEVER claims a delivery (the notification is the
// surface's card — the 2-13 Never precedent).
type OrderVerificationOutcome struct {
	Task      domain.OrderVerification
	View      domain.OrderVerificationStateView
	Word      string
	StateText string
}

// requireOrderVerificationPorts refuses honestly when the contour is not
// wired: the verification repository, the operation reads, the receipt
// reads and the LIVE case material are mandatory (the same ports the
// anchor reads and the fence re-derivation need).
func (s *Service) requireOrderVerificationPorts() error {
	if s.verifications == nil || s.operations == nil || s.dispatches == nil || s.caseMaterial == nil || s.members == nil {
		return fmt.Errorf("%s", orderVerificationPortsMissingText)
	}
	return nil
}

// authorizeOrderVerification is the read gate of the contour: the
// membership FIRST (a non-member is the honest 404), the review scope
// 403 BEFORE any task material (the words-first discipline).
func (s *Service) authorizeOrderVerification(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Review); err != nil {
		if errors.Is(err, identityDomain.ErrNotFound) {
			return tenancy.Context{}, domain.ErrNotFound
		}
		if errors.Is(err, identityDomain.ErrForbidden) {
			return tenancy.Context{}, fmt.Errorf("%w: %s", domain.ErrForbidden, orderVerificationScopeText)
		}
		return tenancy.Context{}, err
	}
	return tenancy.New(actor.ID, tenantID)
}

// readOrderVerificationView reads the task of one case by the
// verification number and derives its state (the honest 404 of a missing
// task — the RLS blindness mirror).
func (s *Service) readOrderVerificationView(ctx context.Context, scope tenancy.Context, caseID id.UUID, verificationNo int64, now time.Time) (domain.OrderVerification, domain.OrderVerificationStateView, error) {
	tasks, err := s.verifications.ReadCaseVerifications(ctx, scope, caseID)
	if err != nil {
		return domain.OrderVerification{}, domain.OrderVerificationStateView{}, err
	}
	for _, task := range tasks {
		if task.VerificationNo != verificationNo {
			continue
		}
		events, err := s.verifications.ReadVerificationEvents(ctx, scope, caseID, verificationNo)
		if err != nil {
			return domain.OrderVerification{}, domain.OrderVerificationStateView{}, err
		}
		view := domain.DeriveOrderVerificationState(task, events, now)
		return task, view, nil
	}
	return domain.OrderVerification{}, domain.OrderVerificationStateView{}, port.ErrVerificationNotFound
}

// verificationMemberScopes resolves the LIVE scope set of one member by
// the member facts (the empty set of a non-member — the guards refuse).
func (s *Service) verificationMemberScopes(ctx context.Context, scope tenancy.Context, member id.UUID) map[identityDomain.Scope]bool {
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

// AssignOrderVerification creates the verification task of one committed
// order operation: the rights gate (review), the LIVE anchor read (the
// operation must be the order action of THIS case, its derived state
// must be committed, its committed receipt must carry the external ABCP
// Order ID), the assignee/backup validation against the LIVE member
// facts (the assignee holds the review scope; the backup is a member),
// the domain guard and the ONE creation transaction (the task row + the
// opening assigned event). The honest refusals speak BEFORE any row: the
// anchor refusal of a non-committed contour, the 409 of the active task,
// the guard words of the round material.
func (s *Service) AssignOrderVerification(ctx context.Context, command AssignOrderVerificationCommand) (OrderVerificationOutcome, error) {
	if err := s.requireOrderVerificationPorts(); err != nil {
		return OrderVerificationOutcome{}, err
	}
	scope, err := s.authorizeOrderVerification(ctx, command.Actor, command.TenantID)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	// The LIVE anchor read: the operation of the command must be the
	// order action of THIS case with the derived state committed and the
	// committed receipt carrying the external ABCP Order ID (the task is
	// built over the CREATED order only — the honest anchor refusal
	// otherwise; the unknown/failed contours belong to 3-7/3-8, the
	// parked AG-4 recommendation already lives on the 4-6 panel).
	operation, receipt, err := s.orderCommittedAnchor(ctx, scope, command.CaseID, command.OperationID)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	// The assignee must be an ACTIVE member holding the review scope; the
	// backup must be an active member (the LIVE truth of the identity —
	// never a stored snapshot).
	if scopes := s.verificationMemberScopes(ctx, scope, command.Assignee); !scopes[identityDomain.Review] {
		return OrderVerificationOutcome{}, fmt.Errorf("assignee не участник с полномочием review: назначьте проверяющего менеджера с полномочием review (живая грант-матрица identity)")
	}
	if scopes := s.verificationMemberScopes(ctx, scope, command.BackupOwner); len(scopes) == 0 {
		return OrderVerificationOutcome{}, fmt.Errorf("backup_owner не активный участник тенанта: укажите резервного участника раунда проверки")
	}
	if err := domain.GuardOrderVerificationAssignment(command.Assignee, command.BackupOwner, command.Deadline); err != nil {
		return OrderVerificationOutcome{}, err
	}
	deadlineAt := command.Now.Add(domain.VerifyDeadlineWindow(command.Deadline))
	task, err := s.verifications.AppendOrderVerification(ctx, port.AppendOrderVerificationParams{
		Scope:            scope,
		CaseID:           command.CaseID,
		WorkItemID:       operation.WorkItemID,
		OperationID:      operation.OperationID,
		ReceiptVersionNo: receipt.ReceiptVersionNo,
		ExternalOrderID:  receipt.References.ExternalObjectID,
		CreatorID:        command.Actor.ID,
		Assignee:         command.Assignee,
		BackupOwner:      command.BackupOwner,
		VerifyDeadline:   command.Deadline,
		DeadlineAt:       deadlineAt,
		Now:              command.Now,
	})
	if err != nil {
		if errors.Is(err, port.ErrVerificationActive) {
			// The honest 409: the CURRENT lifecycle words travel with the
			// conflict (the active task of the operation re-read under
			// the loser's eye).
			return OrderVerificationOutcome{}, s.orderVerificationActiveConflict(ctx, scope, command.OperationID, command.Now)
		}
		return OrderVerificationOutcome{}, err
	}
	events, err := s.verifications.ReadVerificationEvents(ctx, scope, command.CaseID, task.VerificationNo)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	view := domain.DeriveOrderVerificationState(task, events, command.Now)
	return OrderVerificationOutcome{Task: task, View: view, Word: domain.OrderVerificationStateText(view.State), StateText: domain.OrderVerificationDeliveryWord}, nil
}

// orderVerificationActiveConflict re-reads the ACTIVE task of one
// operation and builds the honest 409 conflict with the CURRENT
// lifecycle words (the state word with its honest text).
func (s *Service) orderVerificationActiveConflict(ctx context.Context, scope tenancy.Context, operationID id.UUID, now time.Time) error {
	tasks, err := s.verifications.ReadOperationVerifications(ctx, scope, operationID)
	if err != nil {
		return domain.OrderVerificationActiveError{}
	}
	for _, task := range tasks {
		events, err := s.verifications.ReadVerificationEvents(ctx, scope, task.CaseID, task.VerificationNo)
		if err != nil {
			continue
		}
		if view := domain.DeriveOrderVerificationState(task, events, now); view.Active() {
			return domain.OrderVerificationActiveError{State: view.State, StateText: domain.OrderVerificationStateText(view.State)}
		}
	}
	return domain.OrderVerificationActiveError{}
}

// orderCommittedAnchor reads the LIVE committed anchor of one operation:
// the operation must belong to THIS case, be the order action
// (create_abcp_customer_order) and carry the derived state committed;
// the committed receipt must exist with the non-empty external ABCP
// Order ID (the readback material of the screen and the pin of the
// Human Verification Receipt). Every miss is the honest anchor refusal.
func (s *Service) orderCommittedAnchor(ctx context.Context, scope tenancy.Context, caseID, operationID id.UUID) (domain.Operation, port.ActionReceiptRow, error) {
	operations, err := s.operations.ReadCaseOperations(ctx, scope, caseID)
	if err != nil {
		return domain.Operation{}, port.ActionReceiptRow{}, err
	}
	var found *domain.Operation
	for i := range operations {
		if operations[i].OperationID == operationID {
			found = &operations[i]
			break
		}
	}
	if found == nil || found.DefinitionID != domain.OrderActionID {
		return domain.Operation{}, port.ActionReceiptRow{}, domain.ErrOrderVerificationNoReceipt
	}
	ledger, err := s.operations.ReadOperationLedger(ctx, scope, operationID)
	if err != nil {
		return domain.Operation{}, port.ActionReceiptRow{}, err
	}
	state, err := domain.DeriveOperationState(ledger)
	if err != nil {
		return domain.Operation{}, port.ActionReceiptRow{}, err
	}
	if state != domain.OperationCommitted {
		return domain.Operation{}, port.ActionReceiptRow{}, domain.ErrOrderVerificationNoReceipt
	}
	receipts, err := s.dispatches.ReadActionReceipts(ctx, scope, operationID)
	if err != nil {
		return domain.Operation{}, port.ActionReceiptRow{}, err
	}
	for _, receipt := range receipts {
		if receipt.Outcome == domain.ReceiptOutcomeCommitted && receipt.References.ExternalObjectID != "" {
			return *found, receipt, nil
		}
	}
	return domain.Operation{}, port.ActionReceiptRow{}, domain.ErrOrderVerificationNoReceipt
}

// AcknowledgeOrderVerification records the assignee's explicit
// acceptance: the actor MUST be the assignee (the server truth), the
// CAS epoch must be current (a raced append never writes — the honest
// «задача переназначена» family with the CURRENT state words).
func (s *Service) AcknowledgeOrderVerification(ctx context.Context, command AcknowledgeOrderVerificationCommand) (OrderVerificationOutcome, error) {
	if err := s.requireOrderVerificationPorts(); err != nil {
		return OrderVerificationOutcome{}, err
	}
	scope, err := s.authorizeOrderVerification(ctx, command.Actor, command.TenantID)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	task, view, err := s.readOrderVerificationView(ctx, scope, command.CaseID, command.VerificationNo, command.Now)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	if err := domain.GuardOrderVerificationAcknowledge(view.State, command.Actor.ID, view.Assignee, command.ExpectedAssignment); err != nil {
		return OrderVerificationOutcome{}, err
	}
	if _, err := s.verifications.AppendOrderVerificationEvent(ctx, port.AppendOrderVerificationEventParams{
		Scope:          scope,
		CaseID:         command.CaseID,
		VerificationNo: command.VerificationNo,
		Event: domain.OrderVerificationEvent{
			Kind:    domain.OrderVerificationEventAcknowledged,
			ActorID: command.Actor.ID,
		},
		// The CAS expectation rides the COMMAND's epoch (the round the
		// manager saw on the screen) — never the service's fresh read: a
		// reassignment that landed between the screen and the command is
		// the honest raced refusal, never a papered-over pass.
		ExpectedState: view.State,
		ExpectedEpoch: command.ExpectedAssignment,
		Now:           command.Now,
	}); err != nil {
		return OrderVerificationOutcome{}, s.mapVerificationRaced(ctx, scope, command.CaseID, command.VerificationNo, command.Now, err)
	}
	return s.verificationOutcomeAfter(ctx, scope, task, command.Now)
}

// ApproveOrderVerification records the «Подтвердить проверку» decision —
// the Human Verification Receipt: THE DECISION FENCE runs BEFORE the
// event is written (the SAME live re-derivation code the dispatch ladder
// 4.5 runs — DeriveOrderCommitmentMaterial over the fresh CaseView,
// OrderDigestOf, the comparison with the operation's pin; the command's
// expected digest must equal the pin too — the manager confirmed THIS
// version). A divergence refuses the decision BY NAME (AC-9): the event
// is NEVER written over a stale version; the repeated full check is the
// honest path. The approved event carries the material pins: the order
// digest of the exact version, the receipt identity, the external ABCP
// Order ID and the evidence anchors (the SAFE pin set only).
func (s *Service) ApproveOrderVerification(ctx context.Context, command ApproveOrderVerificationCommand) (OrderVerificationOutcome, error) {
	if err := s.requireOrderVerificationPorts(); err != nil {
		return OrderVerificationOutcome{}, err
	}
	scope, err := s.authorizeOrderVerification(ctx, command.Actor, command.TenantID)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	task, view, err := s.readOrderVerificationView(ctx, scope, command.CaseID, command.VerificationNo, command.Now)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	if err := domain.GuardOrderVerificationApprove(view.State, command.Actor.ID, view.Assignee, command.ExpectedAssignment, command.ExpectedOrderDigest); err != nil {
		return OrderVerificationOutcome{}, err
	}
	// THE DECISION FENCE (the dispatch-ladder 4.5 code reused BY FUNCTION):
	// the operation's pin reads from the LIVE pinned preview version, the
	// LIVE composition re-derives over the fresh case view, and BOTH must
	// equal the command's expected digest. Any divergence — the honest
	// refusal BY NAME, the event is never written.
	operation, _, err := s.orderCommittedAnchor(ctx, scope, command.CaseID, task.OperationID)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	preview, err := s.operations.ReadPinnedPreviewVersion(ctx, scope, operation.PreviewID, operation.PreviewVersionNo)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	pinned, ok := preview.TypedInput["order_digest"]
	if !ok || pinned == "" {
		return OrderVerificationOutcome{}, domain.ErrOrderVerificationCompositionChanged
	}
	if command.ExpectedOrderDigest != pinned {
		return OrderVerificationOutcome{}, domain.ErrOrderVerificationCompositionChanged
	}
	caseView, err := s.caseMaterial.CaseView(ctx, command.Actor, command.TenantID, command.CaseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return OrderVerificationOutcome{}, domain.ErrNotFound
		}
		return OrderVerificationOutcome{}, err
	}
	liveMaterial, err := DeriveOrderCommitmentMaterial(caseView)
	if err != nil {
		if domain.IsOrderDerivationRefusal(err) {
			// The LIVE derivation refuses the current composition (no
			// selection anymore, a non-proven position, mixed versions):
			// the SAME honest changed-composition word — never a masked
			// pass, never a transport failure.
			return OrderVerificationOutcome{}, domain.ErrOrderVerificationCompositionChanged
		}
		return OrderVerificationOutcome{}, err
	}
	live := domain.OrderDigestOf(liveMaterial)
	if live != pinned {
		return OrderVerificationOutcome{}, domain.ErrOrderVerificationCompositionChanged
	}
	// The evidence anchors of the Human Verification Receipt (the SAFE
	// pin set only — identifiers, never a material value): the operation,
	// the work item, the receipt version and every position key of the
	// verified composition.
	anchors := []string{
		"operation:" + operation.OperationID.String(),
		"work_item:" + operation.WorkItemID.String(),
		"receipt:" + strconv.FormatInt(task.ReceiptVersionNo, 10),
	}
	for _, position := range liveMaterial.Positions {
		anchors = append(anchors, "position:"+position.PositionKey)
	}
	if _, err := s.verifications.AppendOrderVerificationEvent(ctx, port.AppendOrderVerificationEventParams{
		Scope:          scope,
		CaseID:         command.CaseID,
		VerificationNo: command.VerificationNo,
		Event: domain.OrderVerificationEvent{
			Kind:             domain.OrderVerificationEventApproved,
			OrderDigest:      pinned,
			ReceiptVersionNo: task.ReceiptVersionNo,
			ExternalOrderID:  task.ExternalOrderID,
			EvidenceAnchors:  anchors,
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
		return OrderVerificationOutcome{}, s.mapVerificationRaced(ctx, scope, command.CaseID, command.VerificationNo, command.Now, err)
	}
	outcome, err := s.verificationOutcomeAfter(ctx, scope, task, command.Now)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	// The MANDATORY manual-contour word rides the approved outcome (AC-10/
	// AC-11): Staffy sends NOTHING — the formation and the send stay the
	// manager's manual acts; the status is read separately.
	outcome.StateText = domain.OrderVerificationManualContourWord
	return outcome, nil
}

// BlockOrderVerification records the «Нужна коррекция» decision: the
// affected positions (validated against the composition of the OPERATION
// — the pinned canonical render's position keys) and the structured
// reason word. The correction flow itself is the contour 4-8 (outside
// this slice): the launch is the record plus the honest word — the
// original evidence is never changed.
func (s *Service) BlockOrderVerification(ctx context.Context, command BlockOrderVerificationCommand) (OrderVerificationOutcome, error) {
	if err := s.requireOrderVerificationPorts(); err != nil {
		return OrderVerificationOutcome{}, err
	}
	scope, err := s.authorizeOrderVerification(ctx, command.Actor, command.TenantID)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	task, view, err := s.readOrderVerificationView(ctx, scope, command.CaseID, command.VerificationNo, command.Now)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	if err := domain.GuardOrderVerificationBlock(view.State, command.Actor.ID, view.Assignee, command.ExpectedAssignment, command.Positions, command.Reason); err != nil {
		return OrderVerificationOutcome{}, err
	}
	// The affected positions must belong to the composition of the
	// OPERATION (the pinned canonical render — the byte-stable material
	// the client confirmed; a foreign position key is the honest guard
	// refusal, never a silent clip).
	operation, _, err := s.orderCommittedAnchor(ctx, scope, command.CaseID, task.OperationID)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	preview, err := s.operations.ReadPinnedPreviewVersion(ctx, scope, operation.PreviewID, operation.PreviewVersionNo)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	pinnedKeys := map[string]bool{}
	for _, key := range domain.OrderMaterialPositionKeys(preview.TypedInput["order_material"]) {
		pinnedKeys[key] = true
	}
	for _, position := range command.Positions {
		if !pinnedKeys[position] {
			return OrderVerificationOutcome{}, fmt.Errorf("позиция «%s» не входит в состав операции заказа: укажите затронутые позиции состава подтверждённого заказа", position)
		}
	}
	if _, err := s.verifications.AppendOrderVerificationEvent(ctx, port.AppendOrderVerificationEventParams{
		Scope:          scope,
		CaseID:         command.CaseID,
		VerificationNo: command.VerificationNo,
		Event: domain.OrderVerificationEvent{
			Kind:      domain.OrderVerificationEventBlocked,
			Positions: command.Positions,
			Reason:    command.Reason,
			ActorID:   command.Actor.ID,
		},
		// The CAS expectation rides the COMMAND's epoch (the round the
		// manager saw on the screen) — never the service's fresh read: a
		// reassignment that landed between the screen and the command is
		// the honest raced refusal, never a papered-over pass.
		ExpectedState: view.State,
		ExpectedEpoch: command.ExpectedAssignment,
		Now:           command.Now,
	}); err != nil {
		return OrderVerificationOutcome{}, s.mapVerificationRaced(ctx, scope, command.CaseID, command.VerificationNo, command.Now, err)
	}
	outcome, err := s.verificationOutcomeAfter(ctx, scope, task, command.Now)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	// The correction launch word rides the blocked outcome (AC-12): the
	// progression is blocked, the contour 4-8 is named, the original
	// evidence is never changed.
	outcome.StateText = domain.OrderVerificationCorrectionWord
	return outcome, nil
}

// ReassignOrderVerification records the owner's explicit reassignment
// (the overdue fallback — never an automatic takeover, AG-4/AD-14): the
// new round's assignee, backup and deadline word open a NEW epoch; the
// former assignee's stale-epoch attempts answer the honest «задача
// переназначена». The history is never rewritten.
func (s *Service) ReassignOrderVerification(ctx context.Context, command ReassignOrderVerificationCommand) (OrderVerificationOutcome, error) {
	if err := s.requireOrderVerificationPorts(); err != nil {
		return OrderVerificationOutcome{}, err
	}
	scope, err := s.authorizeOrderVerification(ctx, command.Actor, command.TenantID)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	task, view, err := s.readOrderVerificationView(ctx, scope, command.CaseID, command.VerificationNo, command.Now)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	if err := domain.GuardOrderVerificationReassign(view.State, command.Assignee, command.BackupOwner, command.Deadline); err != nil {
		return OrderVerificationOutcome{}, err
	}
	if scopes := s.verificationMemberScopes(ctx, scope, command.Assignee); !scopes[identityDomain.Review] {
		return OrderVerificationOutcome{}, fmt.Errorf("assignee не участник с полномочием review: назначьте проверяющего менеджера с полномочием review (живая грант-матрица identity)")
	}
	if scopes := s.verificationMemberScopes(ctx, scope, command.BackupOwner); len(scopes) == 0 {
		return OrderVerificationOutcome{}, fmt.Errorf("backup_owner не активный участник тенанта: укажите резервного участника раунда проверки")
	}
	deadlineAt := command.Now.Add(domain.VerifyDeadlineWindow(command.Deadline))
	if _, err := s.verifications.AppendOrderVerificationEvent(ctx, port.AppendOrderVerificationEventParams{
		Scope:          scope,
		CaseID:         command.CaseID,
		VerificationNo: command.VerificationNo,
		Event: domain.OrderVerificationEvent{
			Kind:           domain.OrderVerificationEventAssigned,
			Assignee:       command.Assignee,
			BackupOwner:    command.BackupOwner,
			VerifyDeadline: command.Deadline,
			DeadlineAt:     deadlineAt,
			ActorID:        command.Actor.ID,
		},
		// The reassignment is the owner's EXPLICIT decision over an OPEN
		// task (any round): the CAS pins the app-read state and epoch — a
		// concurrent decision that closed the task races honestly; the
		// owner's own epoch expectation never gates the round change.
		ExpectedState: view.State,
		ExpectedEpoch: view.AssignmentEpoch,
		Now:           command.Now,
	}); err != nil {
		return OrderVerificationOutcome{}, s.mapVerificationRaced(ctx, scope, command.CaseID, command.VerificationNo, command.Now, err)
	}
	return s.verificationOutcomeAfter(ctx, scope, task, command.Now)
}

// verificationOutcomeAfter re-reads the task state after a successful
// write and builds the honest outcome (the derived words of the moment).
func (s *Service) verificationOutcomeAfter(ctx context.Context, scope tenancy.Context, task domain.OrderVerification, now time.Time) (OrderVerificationOutcome, error) {
	events, err := s.verifications.ReadVerificationEvents(ctx, scope, task.CaseID, task.VerificationNo)
	if err != nil {
		return OrderVerificationOutcome{}, err
	}
	view := domain.DeriveOrderVerificationState(task, events, now)
	return OrderVerificationOutcome{Task: task, View: view, Word: domain.OrderVerificationStateText(view.State)}, nil
}

// mapVerificationRaced maps the raced CAS refusal onto the honest
// current-state words: the re-read speaks what lives NOW (the
// «задача переназначена» family — the parallel decision of the second
// manager lost honestly, AC-7).
func (s *Service) mapVerificationRaced(ctx context.Context, scope tenancy.Context, caseID id.UUID, verificationNo int64, now time.Time, err error) error {
	if !errors.Is(err, port.ErrVerificationRaced) {
		return err
	}
	_, view, viewErr := s.readOrderVerificationView(ctx, scope, caseID, verificationNo, now)
	if viewErr != nil {
		return domain.OrderVerificationRacedError{}
	}
	return domain.OrderVerificationRacedError{State: view.State, StateText: domain.OrderVerificationStateText(view.State)}
}

// ReadOrderVerificationCaseView reads the task states of one case (the
// panel view material of step-03): every task with its derived state
// view — the screen renders the CURRENT truth, never a stored status.
func (s *Service) ReadOrderVerificationCaseView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) ([]domain.OrderVerificationStateView, error) {
	if s.verifications == nil {
		return nil, fmt.Errorf("%s", orderVerificationPortsMissingText)
	}
	scope, err := s.authorizeOrderVerification(ctx, actor, tenantID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.verifications.ReadCaseVerifications(ctx, scope, caseID)
	if err != nil {
		return nil, err
	}
	views := make([]domain.OrderVerificationStateView, 0, len(tasks))
	for _, task := range tasks {
		events, err := s.verifications.ReadVerificationEvents(ctx, scope, caseID, task.VerificationNo)
		if err != nil {
			return nil, err
		}
		views = append(views, domain.DeriveOrderVerificationState(task, events, now))
	}
	return views, nil
}

// Suppress nothing: every import is live (tenancy rides the scope reads,
// id rides the anchors, casesDomain rides the honest 404 mapping).
