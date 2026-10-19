package app

import (
	"context"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.3: the external commitment commands (the first slice)
// ---------------------------------------------------------------------------
//
// The gate order is frozen per command (the 2.9 discipline): the rights
// gate (review OR configuration) fires BEFORE every material guard; the
// dictionary guards refuse with the honest words; the approval gate of the
// enqueue demands the EXACT version approved by the CURRENT human owner
// (AC-9 — an earlier approval of another version structurally never
// authorises the changed text); the readiness refuses BEFORE the
// transaction (the fake-repo counters of the unit matrix prove the order —
// no enqueue transaction opens on a refused path); the enqueue itself is
// the persist-before-emit transaction whose signature carries NO adapter
// (AC-3 — the network call is physically impossible before the commit).
// The drain is an operator command (no daemons exist — AD-13): the
// execute-time stale fences run IN ORDER (a newer version, the expired
// window, the moved owner epoch, the re-checked content token and the
// live 3.2 control of the route — the promised enforcement point), every
// fence gases with the honest «слова не ушли» reason, and the dispatch
// policy is decided by the RECORDED capability guarantee (a guarantee-free
// timeout never retries — the blind retry is structurally unselectable,
// AC-5/AC-8).

// externalScopeText is the honest scope refusal of the external
// commitment commands (the shared review-или-configuration vocabulary of
// the case mutations).
const externalScopeText = "Команды внешних обязательств требуют полномочия review или configuration: участники только с audit не создают конверты, не утверждают, не ставят в очередь и не передают наблюдения канала."

// externalOwnerGateText is the honest refusal of the approval command when
// the actor is not the CURRENT human owner of the case.
const externalOwnerGateText = "Утверждать версию конверта может только текущий человек-владелец кейса (response owner): подтвердите передачу владения или создайте версию под текущим владельцем."

// externalDrainRefusedText is the AG-4 honest boundary: the production
// root wires no dispatcher (no real providers exist), the drain refuses in
// words and the records stay queued — the state stays honest.
const externalDrainRefusedText = "Диспетчер внешних отправок не сконфигурирован: записи остаются в очереди (производственный контур не подключает провайдеров — AG-4); слова не уходят."

// externalTokenAlarmText is the integrity alarm of the drain: the content
// token of the immutable row did not re-verify — the row is not the row
// that was approved. Structurally impossible through the guards; the
// honest response is the skip with the alarm, never a gasp with an
// invented reason and never a send.
const externalTokenAlarmText = "Токен версии не перепровернулся: запись оставлена в очереди, отправки нет; передайте в reconciliation (3-7)."

// externalUnknownParkedText names the reconciliation path of the
// guarantee-free unknown (AC-5): the next attempt is structurally
// impossible, the words are possibly out — the human owns the truth.
const externalUnknownParkedText = "Итог неопределён (timeout без идемпотентной гарантии): слепого повтора нет — передаётся в reconciliation (3-7)."

// externalBoundParkedText is the honest parking of the exhausted bound
// (AC-5/AC-8): the attempts never exceed the bound, the honest text names
// the path instead of a blind loop.
const externalBoundParkedText = "Попытки отправки исчерпаны (не более трёх): запись припаркована — создайте новую версию либо передайте в reconciliation (3-7)."

// externalRacedSkipText is the honest skip of the raced twin: the events
// are the first dedup line, the second Staffy intent never exists.
const externalRacedSkipText = "Запись уже обработана параллельным прогоном: второго намерения нет."

// externalPinMismatchText is the live pin refusal of the create and the
// enqueue gates: the pinned version is absent tenant-wide or its STORED
// token differs from the pin token (the exact-version discipline of 2.9).
const externalPinMismatchText = "Пин обязательства не совпал с живой строкой 2.9 (нет версии с таким номером либо токен иной): сверьте пин точной версии и повторите."

// CreateExternalCommitment creates one envelope version: the operator
// material (the dictionary words, the content, the window, the next step),
// the relation pins resolved SERVER-SIDE (the response-owner snapshot with
// the epoch, the release lineage numbers, the current policy outcome
// number), the live 2.9 pin match (ANY carried pin is verified against the
// stored row token; the offer demands the pin — the domain guard), the
// expected monotonic number and the content binding token of the EXACT
// material bytes. The version lands persisted — the enqueue is a separate
// command behind the approval gate.
func (s *Service) CreateExternalCommitment(ctx context.Context, command casesAPI.CreateExternalCommitmentCommand) (casesAPI.ExternalCommitmentOutcome, error) {
	// The rights gate FIRST (review or configuration — the M1 precedent).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.ExternalCommitmentOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.ExternalCommitmentOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, externalScopeText)
	}
	// The stable envelope identifier resolves FIRST: the caller may pass
	// one (the re-creation of a known envelope) or none — the app mints a
	// fresh UUIDv7; the dictionary guard below demands the valid word.
	extCommitmentID := command.ExtCommitmentID
	if !extCommitmentID.Valid() {
		if extCommitmentID, err = s.ids.New(command.Now); err != nil {
			return casesAPI.ExternalCommitmentOutcome{}, err
		}
	}
	// The expected monotonic number of the envelope ladder resolves BEFORE
	// the guard (the domain command models the STORE material — the guard
	// demands the honest version number; the store re-validates it under
	// the advisory locks, the raced ladder is the honest stale-version
	// refusal).
	versions, err := s.repository.ReadExternalCommitments(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.ExternalCommitmentOutcome{}, err
	}
	versionNo := int64(0)
	for _, version := range versions {
		if version.ExtCommitmentID == extCommitmentID && version.VersionNo > versionNo {
			versionNo = version.VersionNo
		}
	}
	versionNo++
	// The response-owner snapshot with the epoch resolves BEFORE the guard
	// too (the domain command models the STORE material — the owner words
	// are the server-side truth, the 2.4 ClientReply precedent).
	ownershipTerms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.ExternalCommitmentOutcome{}, err
	}
	owner := casesDomain.CurrentOwnership(ownershipTerms)
	if owner == nil {
		return casesAPI.ExternalCommitmentOutcome{}, casesDomain.ErrNotFound
	}
	// The dictionary guard (the honest 400 with the dictionary words).
	if err := rule("create_external_commitment", casesDomain.GuardCreateExternalCommitment(casesDomain.CreateExternalCommitmentCommand{
		TenantID: command.TenantID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ExtCommitmentID: extCommitmentID, VersionNo: versionNo, Purpose: command.Purpose,
		RecipientKind: command.RecipientKind, Channel: command.Channel,
		ContentText: command.ContentText, ValidUntil: command.ValidUntil,
		ExpectedNextStep: command.ExpectedNextStep, AckRequirement: command.AckRequirement,
		Pin: command.Pin, OwnerKind: owner.OwnerKind, OwnerRef: owner.OwnerRef, OwnerEpoch: owner.Epoch,
	}, command.Now)); err != nil {
		return casesAPI.ExternalCommitmentOutcome{}, err
	}
	// The work item must exist and be alive (the write-command boundary).
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, command.CaseID, []id.UUID{command.WorkItemID})
	if err != nil {
		return casesAPI.ExternalCommitmentOutcome{}, err
	}
	current, ok := currents[command.WorkItemID]
	if !ok {
		return casesAPI.ExternalCommitmentOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.ExternalCommitmentOutcome{}, rule("create_external_commitment", fmt.Errorf("%s", casesDomain.TerminalWorkItemCommitmentText))
	}
	// The release lineage numbers (the 2.9 precedent) and the current
	// policy outcome number of the Work Item (zero — the honest absence).
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.ExternalCommitmentOutcome{}, err
	}
	_, decisionNo, profileVersion, err := s.lineageWord(ctx, scope, conversation)
	if err != nil {
		return casesAPI.ExternalCommitmentOutcome{}, err
	}
	policyOutcomeNo := int64(0)
	outcomes, err := s.repository.ReadPolicyOutcomes(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.ExternalCommitmentOutcome{}, err
	}
	outcomesOfItem := []casesDomain.PolicyOutcome{}
	for _, outcome := range outcomes {
		if outcome.WorkItemID == command.WorkItemID {
			outcomesOfItem = append(outcomesOfItem, outcome)
		}
	}
	if currentOutcome := casesDomain.CurrentPolicyOutcome(outcomesOfItem); currentOutcome != nil {
		policyOutcomeNo = currentOutcome.OutcomeNo
	}
	// The live 2.9 pin match of ANY carried pin (the offer demands one —
	// the domain guard refused above): the pinned version must exist
	// tenant-wide and its STORED content token must equal the pin token
	// (the stored word is the truth — never a re-computation).
	if command.Pin.Present() {
		if _, err := s.livePinReadiness(ctx, scope, command.Pin); err != nil {
			return casesAPI.ExternalCommitmentOutcome{}, err
		}
	}
	// The content binding token of the EXACT material bytes (the per-tenant
	// commitment key v1 — the 2.9 reuse; the store re-computes under the
	// locks).
	key, err := s.repository.ReadCommitmentKey(ctx, scope, command.Now)
	if err != nil {
		return casesAPI.ExternalCommitmentOutcome{}, err
	}
	token := casesDomain.ExternalCommitmentContentToken(key, casesDomain.ExternalCommitment{
		TenantID: command.TenantID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ExtCommitmentID: extCommitmentID, VersionNo: versionNo,
		Purpose: command.Purpose, RecipientKind: command.RecipientKind, Channel: command.Channel,
		ContentText: command.ContentText, ValidUntil: command.ValidUntil,
		ExpectedNextStep: command.ExpectedNextStep, AckRequirement: command.AckRequirement,
		Pin: command.Pin, OwnerKind: owner.OwnerKind, OwnerRef: owner.OwnerRef, OwnerEpoch: owner.Epoch,
		DecisionNo: decisionNo, ProfileVersion: profileVersion, PolicyOutcomeNo: policyOutcomeNo,
	})
	if err := s.repository.CreateExternalCommitment(ctx, casesPort.ExternalCommitmentCreateParams{
		Scope: scope, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ExtCommitmentID: extCommitmentID, VersionNo: versionNo,
		Purpose: command.Purpose, RecipientKind: command.RecipientKind, Channel: command.Channel,
		ContentText: command.ContentText, ValidUntil: command.ValidUntil,
		ExpectedNextStep: command.ExpectedNextStep, AckRequirement: command.AckRequirement,
		Pin: command.Pin, OwnerKind: owner.OwnerKind, OwnerRef: owner.OwnerRef, OwnerEpoch: owner.Epoch,
		DecisionNo: decisionNo, ProfileVersion: profileVersion, PolicyOutcomeNo: policyOutcomeNo,
		ContentToken: token, ActorID: command.Actor.ID,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.ExternalCommitmentOutcome{}, err
	}
	return casesAPI.ExternalCommitmentOutcome{
		Command: "create_external_commitment", ExtCommitmentID: extCommitmentID,
		VersionNo: versionNo, ContentToken: token,
	}, nil
}

// ApproveExternalCommitment records the approval of the EXACT version by
// the CURRENT human owner: the step-up evidence, the owner gate and the
// append-only row (the repeated approval of the same approver is the
// silent idempotency of the UNIQUE). The approval creates NO outbox record
// (AC-9) — it only unlocks the enqueue of that exact version.
func (s *Service) ApproveExternalCommitment(ctx context.Context, command casesAPI.ApproveExternalCommitmentCommand) (casesAPI.ExternalApprovalOutcome, error) {
	// The rights gate FIRST.
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.ExternalApprovalOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.ExternalApprovalOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, externalScopeText)
	}
	// The step-up gate: the approval is the strongest human confirmation
	// of the slice — an unverified session never approves.
	if !command.Actor.StepUpVerified {
		return casesAPI.ExternalApprovalOutcome{}, domain.ErrStepUpRequired
	}
	// The current-owner gate: the actor must BE the current human owner
	// (the store re-validates under the locks — the app read is never the
	// authorisation truth).
	ownershipTerms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.ExternalApprovalOutcome{}, err
	}
	owner := casesDomain.CurrentOwnership(ownershipTerms)
	if owner == nil {
		return casesAPI.ExternalApprovalOutcome{}, casesDomain.ErrNotFound
	}
	if owner.OwnerKind != casesDomain.OwnerKindHuman || owner.OwnerRef != command.Actor.ID {
		return casesAPI.ExternalApprovalOutcome{}, rule("approve_external_commitment", fmt.Errorf("%s", externalOwnerGateText))
	}
	approvalID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.ExternalApprovalOutcome{}, err
	}
	recorded, err := s.repository.ApproveExternalCommitment(ctx, casesPort.ExternalApprovalParams{
		Scope: scope, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ExtCommitmentID: command.ExtCommitmentID, VersionNo: command.VersionNo,
		ApprovalID: approvalID, ApproverID: command.Actor.ID, StepUpVerified: true,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	})
	if err != nil {
		return casesAPI.ExternalApprovalOutcome{}, err
	}
	return casesAPI.ExternalApprovalOutcome{
		ExtCommitmentID: command.ExtCommitmentID, VersionNo: command.VersionNo,
		ApprovalRecorded: recorded,
	}, nil
}

// EnqueueExternalCommitment puts the EXACT version into the delivery
// queue: the approval gate (the CURRENT human owner approved THIS version
// — the live ownership read), the readiness gate (the stale fences IN
// ORDER: a newer version, the expired window, the moved owner epoch, the
// offer pin readiness of the LIVE 2.9 row) and the persist-before-emit
// transaction (the signature carries NO adapter — AC-3). Every refusal
// happens BEFORE the transaction with the honest words; the unit matrix
// proves the order with the fake-repo counters.
func (s *Service) EnqueueExternalCommitment(ctx context.Context, command casesAPI.EnqueueExternalCommitmentCommand) (casesAPI.ExternalEnqueueOutcome, error) {
	// The rights gate FIRST.
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.ExternalEnqueueOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.ExternalEnqueueOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, externalScopeText)
	}
	// The EXACT version must exist in this case (the anchor of every gate).
	versions, err := s.repository.ReadExternalCommitments(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.ExternalEnqueueOutcome{}, err
	}
	var version *casesDomain.ExternalCommitment
	maxVersionNo := int64(0)
	for i := range versions {
		if versions[i].ExtCommitmentID == command.ExtCommitmentID {
			if versions[i].VersionNo > maxVersionNo {
				maxVersionNo = versions[i].VersionNo
			}
			if versions[i].VersionNo == command.VersionNo {
				version = &versions[i]
			}
		}
	}
	if version == nil {
		return casesAPI.ExternalEnqueueOutcome{}, casesDomain.ErrNotFound
	}
	// The approval gate (AC-9): the LIVE owner must be the human approver
	// of THIS EXACT version — an absent approval, a staffy owner, a
	// departed approver or a moved epoch all refuse with the same honest
	// words, BEFORE the transaction.
	ownershipTerms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.ExternalEnqueueOutcome{}, err
	}
	owner := casesDomain.CurrentOwnership(ownershipTerms)
	approvals, err := s.repository.ReadExternalApprovals(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.ExternalEnqueueOutcome{}, err
	}
	approved := false
	if owner != nil && owner.OwnerKind == casesDomain.OwnerKindHuman && owner.Epoch == version.OwnerEpoch {
		if approval := casesDomain.ApproverOf(approvals, command.ExtCommitmentID, command.VersionNo); approval != nil &&
			approval.ApproverID == owner.OwnerRef && approval.StepUpVerified {
			approved = true
		}
	}
	if !approved {
		return casesAPI.ExternalEnqueueOutcome{}, rule("enqueue_external_commitment", fmt.Errorf("%s", casesDomain.ExternalApprovalRequiredText(command.VersionNo)))
	}
	// The readiness gate BEFORE the transaction (the stale fences in
	// order; the offer consumes the LIVE readiness of the pinned 2.9
	// version). A refused readiness never opens the transaction — the
	// fake-repo counter of the unit matrix is the proof.
	pinReadiness := casesDomain.CommitmentReadinessConfirmable
	if version.Purpose == casesDomain.ExternalPurposeOffer {
		if pinReadiness, err = s.livePinReadiness(ctx, scope, version.Pin); err != nil {
			return casesAPI.ExternalEnqueueOutcome{}, err
		}
	}
	ownerEpoch := int64(0)
	if owner != nil {
		ownerEpoch = owner.Epoch
	}
	readiness := casesDomain.DeriveExternalReadiness(*version, casesDomain.ExternalReadinessInput{
		Now: command.Now, MaxVersionNo: maxVersionNo, OwnerEpochCurrent: ownerEpoch, PinReadiness: pinReadiness,
	})
	if err := rule("enqueue_external_commitment", casesDomain.GuardEnqueueExternalCommitment(readiness)); err != nil {
		return casesAPI.ExternalEnqueueOutcome{}, err
	}
	// The persist-before-emit transaction: the signature carries NO
	// adapter — the network call is physically impossible before the
	// commit (AC-3); the store re-validates the approval and the readiness
	// under the advisory locks.
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.ExternalEnqueueOutcome{}, err
	}
	correlation, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.ExternalEnqueueOutcome{}, err
	}
	if err := s.repository.EnqueueExternalCommitment(ctx, casesPort.ExternalEnqueueParams{
		Scope: scope, CaseID: command.CaseID, WorkItemID: version.WorkItemID,
		ExtCommitmentID: command.ExtCommitmentID, VersionNo: command.VersionNo,
		OutboxEventID: outboxEventID, CorrelationID: correlation, ActorID: command.Actor.ID,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.ExternalEnqueueOutcome{}, err
	}
	return casesAPI.ExternalEnqueueOutcome{
		ExtCommitmentID: command.ExtCommitmentID, VersionNo: command.VersionNo, Queued: true,
	}, nil
}

// RelayExternalReceipt records the operator observation of the channel
// world (the 2.12 ConfirmerOperatorRelay precedent — the human passes the
// machine observation, the machine never invents it): the observation word
// is the three-word dictionary, the version must exist, and the store
// re-validates the queue evidence under the locks. The append-only history
// accepts the observation whatever came before — the read-time derivation
// takes the LAST event.
func (s *Service) RelayExternalReceipt(ctx context.Context, command casesAPI.RelayExternalReceiptCommand) (casesAPI.ExternalRelayOutcome, error) {
	// The rights gate FIRST.
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.ExternalRelayOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.ExternalRelayOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, externalScopeText)
	}
	// The observation dictionary (the honest 400 with the three words).
	if err := rule("relay_external_receipt", casesDomain.GuardRelayObservation(command.Observation)); err != nil {
		return casesAPI.ExternalRelayOutcome{}, err
	}
	// The EXACT version must exist in this case.
	versions, err := s.repository.ReadExternalCommitments(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.ExternalRelayOutcome{}, err
	}
	found := false
	workItemID := id.UUID("")
	for _, version := range versions {
		if version.ExtCommitmentID == command.ExtCommitmentID && version.VersionNo == command.VersionNo {
			found = true
			workItemID = version.WorkItemID
			break
		}
	}
	if !found {
		return casesAPI.ExternalRelayOutcome{}, casesDomain.ErrNotFound
	}
	eventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.ExternalRelayOutcome{}, err
	}
	kind := casesDomain.RelayObservationDispatchKind(command.Observation)
	appended, err := s.repository.RelayExternalReceipt(ctx, casesPort.ExternalRelayReceiptParams{
		Scope: scope, CaseID: command.CaseID, WorkItemID: workItemID,
		ExtCommitmentID: command.ExtCommitmentID, VersionNo: command.VersionNo,
		EventID: eventID, Kind: kind, ActorID: command.Actor.ID,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	})
	if err != nil {
		return casesAPI.ExternalRelayOutcome{}, err
	}
	if !appended {
		return casesAPI.ExternalRelayOutcome{}, rule("relay_external_receipt", fmt.Errorf("%s", externalRacedSkipText))
	}
	return casesAPI.ExternalRelayOutcome{
		ExtCommitmentID: command.ExtCommitmentID, VersionNo: command.VersionNo, Kind: kind,
	}, nil
}

// DrainExternalCommitments is the operator drain command (no daemons
// exist — AD-13; the pages never pretend to be automation): the bounded
// pending batch, the execute-time stale fences IN ORDER (the re-checked
// content token first — the integrity probe is an alarm, never a gasp;
// then the newer version, the expired window, the moved owner epoch and
// the live 3.2 control of the route — the promised enforcement point,
// fail-closed on an unreadable control), the dispatch policy by the
// RECORDED capability guarantee (a guarantee-free timeout never retries —
// the pending query makes the blind retry structurally unselectable) and
// the bounded attempts (the exhaustion parks with the honest text). The
// words leave ONLY through the dispatcher — and the production root wires
// none (AG-4): the command refuses in words, the records stay queued.
func (s *Service) DrainExternalCommitments(ctx context.Context, actor domain.Principal, tenantID id.UUID) (casesAPI.ExternalDrainResult, error) {
	// The configuration scope (the DrainOutbox precedent).
	scope, err := s.manageScope(ctx, actor, tenantID)
	if err != nil {
		return casesAPI.ExternalDrainResult{}, err
	}
	// The AG-4 honest boundary: no dispatcher, no sends — the records stay
	// queued, the state stays honest.
	if s.externalDispatcher == nil {
		return casesAPI.ExternalDrainResult{}, rule("drain_external_commitments", fmt.Errorf("%s", externalDrainRefusedText))
	}
	pending, err := s.repository.PendingExternalCommitments(ctx, scope, casesDomain.ExternalCommitmentDrainLimit)
	if err != nil {
		return casesAPI.ExternalDrainResult{}, err
	}
	result := casesAPI.ExternalDrainResult{Considered: len(pending), Reports: []casesAPI.ExternalDrainReport{}}
	report := func(extID id.UUID, versionNo int64, text string) {
		result.Reports = append(result.Reports, casesAPI.ExternalDrainReport{ExtCommitmentID: extID, VersionNo: versionNo, Text: text})
	}
	now := s.clock.Now().UTC()
	for _, candidate := range pending {
		version := candidate.Version
		// The integrity probe FIRST (never a gasp, never a send): the
		// content token of the immutable row re-computed from the row
		// bytes with the per-tenant key. A mismatch is the corruption
		// alarm — the record stays queued for the human (3-7).
		if casesDomain.ExternalCommitmentContentToken(candidate.HMACKey, version) != version.ContentToken {
			result.Skipped++
			report(version.ExtCommitmentID, version.VersionNo, externalTokenAlarmText)
			continue
		}
		// The execute-time stale fences IN ORDER — every fence gases with
		// the honest reason (AC-6: «слова не ушли»), the raced gasp is
		// the honest skip.
		if reason, stale := externalStaleFence(candidate, now); stale {
			eventID, err := s.ids.New(now)
			if err != nil {
				return casesAPI.ExternalDrainResult{}, err
			}
			gassed, err := s.repository.GasExternalCommitmentQueue(ctx, casesPort.ExternalGasParams{
				Scope: scope, CaseID: version.CaseID, WorkItemID: version.WorkItemID,
				ExtCommitmentID: version.ExtCommitmentID, VersionNo: version.VersionNo,
				EventID: eventID, Reason: reason, ActorID: actor.ID, Now: now,
			})
			if err != nil {
				return casesAPI.ExternalDrainResult{}, err
			}
			if !gassed {
				result.Skipped++
				report(version.ExtCommitmentID, version.VersionNo, externalRacedSkipText)
				continue
			}
			result.Gassed++
			continue
		}
		// The control fence is LIVE (3.2 — the promised enforcement
		// point): a nil port or an unresolvable control is the fail-closed
		// gasp (the emergency brake — the words never leave on an
		// unreadable control); every non-accept mode gases ANY purpose.
		if reason, refuse := s.externalControlFence(ctx, tenantID, version); refuse {
			eventID, err := s.ids.New(now)
			if err != nil {
				return casesAPI.ExternalDrainResult{}, err
			}
			gassed, err := s.repository.GasExternalCommitmentQueue(ctx, casesPort.ExternalGasParams{
				Scope: scope, CaseID: version.CaseID, WorkItemID: version.WorkItemID,
				ExtCommitmentID: version.ExtCommitmentID, VersionNo: version.VersionNo,
				EventID: eventID, Reason: reason, ActorID: actor.ID, Now: now,
			})
			if err != nil {
				return casesAPI.ExternalDrainResult{}, err
			}
			if !gassed {
				result.Skipped++
				report(version.ExtCommitmentID, version.VersionNo, externalRacedSkipText)
				continue
			}
			result.Gassed++
			continue
		}
		// The dispatch policy by the derived state and the RECORDED
		// guarantee (the pending query already excluded the decided, the
		// bound-exhausted and the guarantee-free unknowns — this is the
		// belt over the braces, honestly).
		state := casesDomain.DeriveExternalDeliveryState(true, candidate.Events)
		attemptUsed := int64(0)
		var lastEvent *casesDomain.ExternalDispatchEvent
		for i := range candidate.Events {
			event := candidate.Events[i]
			switch event.Kind {
			case casesDomain.ExternalDispatchAttemptSubmitted, casesDomain.ExternalDispatchAttemptFailed, casesDomain.ExternalDispatchAttemptUnknown:
				if event.AttemptNo > attemptUsed {
					attemptUsed = event.AttemptNo
				}
			}
			if lastEvent == nil || event.EventNo >= lastEvent.EventNo {
				lastEvent = &candidate.Events[i]
			}
		}
		attemptNo := attemptUsed + 1
		switch state.State {
		case casesDomain.ExternalDeliveryFailed:
			// The pre-send refusal retries within the bound — the same
			// idempotency key, no blind loop.
			if attemptUsed >= casesDomain.MaxDispatchAttempts {
				result.Skipped++
				report(version.ExtCommitmentID, version.VersionNo, externalBoundParkedText)
				continue
			}
		case casesDomain.ExternalDeliveryUnknown:
			// The possible-send ambiguity retries ONLY by the RECORDED
			// capability guarantee of the last attempt — a guarantee-free
			// unknown never retries (the blind retry is structurally
			// impossible, AC-5).
			if lastEvent == nil || !lastEvent.IdempotentGuarantee {
				result.Skipped++
				report(version.ExtCommitmentID, version.VersionNo, externalUnknownParkedText)
				continue
			}
			if attemptUsed >= casesDomain.MaxDispatchAttempts {
				result.Skipped++
				report(version.ExtCommitmentID, version.VersionNo, externalBoundParkedText)
				continue
			}
		}
		// The dispatch (the ONLY network call of the slice — AFTER the
		// enqueue commit, never inside a transaction): the deterministic
		// payload of the immutable version with the idempotency key
		// derivative of the version pin (the repeated attempt of the
		// same version carries the SAME key — the provider deduplicates).
		idempotencyKey := casesDomain.ExternalCommitmentIdempotencyKey(tenantID, version.ExtCommitmentID, version.VersionNo)
		outcome := casesPort.ExternalDispatchResult{Outcome: casesDomain.DispatchOutcomeTimeout, IdempotentGuarantee: false}
		dispatched, dispatchErr := s.externalDispatcher.Dispatch(ctx, scope, casesDomain.BuildOutboundExternalPayload(version, idempotencyKey))
		switch {
		case dispatchErr != nil:
			// The adapter call itself failed — the words' fate is the
			// possible-send ambiguity: the honest unknown, no guarantee
			// word, no blind retry (the fail-closed discipline).
			outcome = casesPort.ExternalDispatchResult{Outcome: casesDomain.DispatchOutcomeTimeout, IdempotentGuarantee: false,
				Details: "ошибка вызова диспетчера"}
		case casesDomain.GuardDispatchOutcome(dispatched.Outcome) != nil:
			// A foreign outcome word is the broken machine contract: the
			// honest unknown, no guarantee, no retry.
			outcome = casesPort.ExternalDispatchResult{Outcome: casesDomain.DispatchOutcomeTimeout, IdempotentGuarantee: false,
				Details: casesDomain.TruncateExternalDetails("исход вне словаря: " + dispatched.Outcome)}
		default:
			outcome = dispatched
		}
		events := []casesPort.ExternalDispatchEventAppend{}
		attemptEventID, err := s.ids.New(now)
		if err != nil {
			return casesAPI.ExternalDrainResult{}, err
		}
		details := casesDomain.ExternalDispatchText(outcome.Outcome, outcome.CapabilityID, outcome.ExternalCorrelationID)
		if outcome.Details != "" {
			details = casesDomain.TruncateExternalDetails(outcome.Details)
		}
		switch outcome.Outcome {
		case casesDomain.DispatchOutcomeAccepted:
			events = append(events, casesPort.ExternalDispatchEventAppend{
				EventID: attemptEventID, Kind: casesDomain.ExternalDispatchAttemptSubmitted, AttemptNo: attemptNo,
				ExternalCorrelationID: outcome.ExternalCorrelationID, IdempotentGuarantee: outcome.IdempotentGuarantee,
				Details: details,
			})
		case casesDomain.DispatchOutcomeDelivered:
			// The delivered outcome is the two-event truth of ONE call:
			// the words left (attempt_submitted with the correlation) and
			// the channel reported the delivery (receipt_delivered) —
			// the derivation takes the last, the state is delivered.
			receiptEventID, err := s.ids.New(now)
			if err != nil {
				return casesAPI.ExternalDrainResult{}, err
			}
			events = append(events,
				casesPort.ExternalDispatchEventAppend{
					EventID: attemptEventID, Kind: casesDomain.ExternalDispatchAttemptSubmitted, AttemptNo: attemptNo,
					ExternalCorrelationID: outcome.ExternalCorrelationID, IdempotentGuarantee: outcome.IdempotentGuarantee,
					Details: details,
				},
				casesPort.ExternalDispatchEventAppend{
					EventID: receiptEventID, Kind: casesDomain.ExternalDispatchReceiptDelivered,
					Details: "квитанция канала: доставка",
				})
		case casesDomain.DispatchOutcomeFailed:
			events = append(events, casesPort.ExternalDispatchEventAppend{
				EventID: attemptEventID, Kind: casesDomain.ExternalDispatchAttemptFailed, AttemptNo: attemptNo,
				IdempotentGuarantee: outcome.IdempotentGuarantee, Details: details,
			})
		default: // timeout — the possible-send ambiguity
			events = append(events, casesPort.ExternalDispatchEventAppend{
				EventID: attemptEventID, Kind: casesDomain.ExternalDispatchAttemptUnknown, AttemptNo: attemptNo,
				ExternalCorrelationID: outcome.ExternalCorrelationID,
				IdempotentGuarantee:   outcome.IdempotentGuarantee, Details: details,
			})
		}
		appended, err := s.repository.RecordExternalDispatchOutcome(ctx, casesPort.ExternalDispatchOutcomeParams{
			Scope: scope, CaseID: version.CaseID, WorkItemID: version.WorkItemID,
			ExtCommitmentID: version.ExtCommitmentID, VersionNo: version.VersionNo,
			Events: events, ActorID: actor.ID, Now: now,
		})
		if err != nil {
			return casesAPI.ExternalDrainResult{}, err
		}
		if !appended {
			result.Skipped++
			report(version.ExtCommitmentID, version.VersionNo, externalRacedSkipText)
			continue
		}
		switch outcome.Outcome {
		case casesDomain.DispatchOutcomeAccepted:
			result.Submitted++
		case casesDomain.DispatchOutcomeDelivered:
			result.Delivered++
		case casesDomain.DispatchOutcomeFailed:
			result.FailedRetry++
		default:
			result.UnknownParked++
			report(version.ExtCommitmentID, version.VersionNo, externalUnknownParkedText)
		}
	}
	return result, nil
}

// externalStaleFence resolves the execute-time stale fence of one
// candidate IN ORDER (a newer version, the expired window, the moved
// owner epoch): the first fired fence wins, its reason is the gasp word
// (AC-6). The content token probe ran earlier (the alarm, never a gasp);
// the control fence is the LIVE 3.2 read — it runs separately.
func externalStaleFence(candidate casesPort.ExternalDispatchCandidate, now time.Time) (string, bool) {
	version := candidate.Version
	switch {
	case candidate.MaxVersionNo > version.VersionNo:
		return casesDomain.ExternalStaleNewerVersion, true
	case !version.ValidUntil.After(now):
		return casesDomain.ExternalStaleExpired, true
	case candidate.OwnerEpochCurrent != version.OwnerEpoch:
		return casesDomain.ExternalStaleOwnerEpoch, true
	}
	return "", false
}

// externalControlFence is the LIVE 3.2 enforcement point of the drain: a
// nil port or an unresolvable control is the fail-closed gasp (the
// emergency brake — the words never leave on an unreadable control), and
// every non-accept mode gases ANY purpose (the per-mode admission
// subtlety belongs to the later 3.2 slice, honestly).
func (s *Service) externalControlFence(ctx context.Context, tenantID id.UUID, version casesDomain.ExternalCommitment) (string, bool) {
	if s.controlEffective == nil {
		return casesDomain.ExternalStaleControlMode, true
	}
	control, err := s.controlEffective.EffectiveFor(ctx, tenantID, version.Channel)
	if err != nil || control.Mode != casesPort.ExternalControlAccept {
		return casesDomain.ExternalStaleControlMode, true
	}
	return "", false
}

// livePinReadiness resolves the LIVE readiness word of the pinned 2.9
// version (the create and the enqueue gate): the pinned version must
// exist tenant-wide, its STORED content token must equal the pin token
// (the exact-version discipline — the stored word is the truth), and the
// DeriveCommitmentReadiness of the exact version resolves the word
// (offered_confirmable is the only passing readiness of the offer pin).
func (s *Service) livePinReadiness(ctx context.Context, scope tenancy.Context, pin casesDomain.ExternalCommitmentPin) (string, error) {
	versions, err := s.repository.ReadCommitmentVersionsByID(ctx, scope, pin.CommitmentID)
	if err != nil {
		return "", err
	}
	var pinned *casesDomain.CommitmentVersion
	maxVersionNo := int64(0)
	for i := range versions {
		if versions[i].VersionNo == pin.VersionNo {
			pinned = &versions[i]
		}
		if versions[i].VersionNo > maxVersionNo {
			maxVersionNo = versions[i].VersionNo
		}
	}
	if pinned == nil || pinned.ContentToken != pin.ContentToken {
		return "", rule("create_external_commitment", fmt.Errorf("%s", externalPinMismatchText))
	}
	return casesDomain.DeriveCommitmentReadiness(*pinned, maxVersionNo), nil
}
