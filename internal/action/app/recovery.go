package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.8 (step-02): the recovery-case worker — CreateRecoveryCase,
// the eligibility ladder and the LIVE-reading assignment
// ---------------------------------------------------------------------------
//
// The creation is an EXPLICIT operator command (NO daemon, NO ticker —
// the drain precedent of 3-3/3-6/3-7). The contour is LOCAL: not ONE
// provider adapter rides the command path — the network stays in the
// dispatch contour of 3-6 and the reconciliation contour of 3-7 (the
// reflect probe of the unit matrix pins the absence of the provider
// ports in the signatures). The command ladder is the spec order:
//
//   - the operator scope gate (the membership FIRST — a non-member gets
//     the 404, never a disclosure; then the operations manage word);
//   - the RECORDED facts read (read-time, NO locks, NO writes — the
//     eligibility ladder NEVER writes on any gas, the fake-repo counters
//     pin it);
//   - the eligibility ladder (the domain derivation over the recorded
//     state + the declaredness halves of the definition catalog + the
//     composition fact of the wired reconciliation adapter — the AG-4
//     production path answers reconciliation_unavailable, the human is
//     mandatory): every gas is the honest refusal word wrapped into
//     ErrRecoveryIneligible — NOTHING is written;
//   - the idempotent pre-check (the existing case read by the operation
//     pin): the digest comparator answers idempotent/evidence — the
//     store transaction re-runs it UNDER the lock (the belt over braces);
//   - the assignment of the LIVE readings ONLY (the creation path): the
//     CURRENT ownership term of the original Case (CaseView.Ownership —
//     the maximal epoch) and the human coverage of the original
//     conversation route (RouteFactsSource — the coverageWord canon:
//     the channel by the modality, the inbound candidate; the words
//     NEVER come from a command parameter) — both fail-closed refusals
//     write NOTHING;
//   - the ONE creation transaction (the store re-reads the existing case
//     under the recovery advisory lock — a raced twin resolves by the
//     idempotent/evidence decision; the creation lands the case row, the
//     created event and the outbox recovery_case.created record in ONE
//     commit, persist-before-emit AD-6).
//
// The material NEVER travels: the bundle carries the SAFE pins only
// (AD-10/AC-9); the recovery key and the bundle digest ride the
// per-tenant commitment key v1 (the INSERT-only ensure pattern of the
// preview store).

// recoveryScopeText is the honest scope refusal of the creation command:
// the transport pins the operator manage surface (step-03); the app gate
// speaks the operations scope word of the acting member (the
// dispatchScope/reconcileScope precedent — the recovery is the same
// operator surface).
const recoveryScopeText = "Команда создания Recovery Case требует полномочия operations: восстановление — операторная команда, участники без операторного полномочия кейсы восстановления не создают."

// recoveryPortsMissingText is the honest refusal of the unwired contour
// (the preview/dispatch/reconcile precedents): the recovery repository
// and the preview key are mandatory; the case material and the route
// facts read through the ports wired since 3-4 (the assignment needs the
// LIVE words — an unwired port is the fail-closed coverage refusal, not
// a crash).
const recoveryPortsMissingText = "Контур восстановления не сконфигурирован: репозиторий recovery не подключён, команда честно отклоняется."

// recoveryOperationUnknownText speaks the 404 of an unknown operation
// (the membership blindness BEFORE any material — the 3-6/3-7
// precedent).
const recoveryOperationUnknownText = "Операция неизвестна в этом тененте: Recovery Case не создаётся (404 до материала)."

// RecoveryCreateCommand is the explicit creation command: the acting
// operator principal, the tenant, the operation pin and the moment. The
// body is EMPTY by the transport contract (the strict decode pins it) —
// the words of the assignment NEVER come from the command parameters.
type RecoveryCreateCommand struct {
	Actor       identityDomain.Principal
	TenantID    id.UUID
	OperationID id.UUID
	Now         time.Time
}

// RecoveryCreateResult is the honest outcome of the creation command:
// the case view (the existing one on the idempotent/evidence paths), the
// decision word (created/idempotent/evidence) and the fixed honest text
// of the outcome (the surface speaks the SAME words the panel speaks).
type RecoveryCreateResult struct {
	Decision string
	Text     string
	Case     port.RecoveryCaseRow
}

// recoveryScope is the gate of the creation command: the membership
// FIRST (a non-member gets the 404 — no disclosure that the tenant
// recovers actions at all), then the operations scope word of the acting
// member (the dispatchScope/reconcileScope precedent).
func (s *Service) recoveryScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return tenancy.Context{}, err
	}
	if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Operations); err != nil {
		if errors.Is(err, identityDomain.ErrForbidden) {
			return tenancy.Context{}, fmt.Errorf("%w: %s", domain.ErrForbidden, recoveryScopeText)
		}
		if errors.Is(err, identityDomain.ErrNotFound) {
			return tenancy.Context{}, domain.ErrNotFound
		}
		return tenancy.Context{}, err
	}
	return scope, nil
}

// requireRecoveryPorts refuses honestly when the recovery contour is not
// wired: the recovery repository is mandatory (the fact read, the case
// reads and the ONE creation transaction); the preview key is mandatory
// (the recovery key and the bundle digest ride it — the requireReconcile
// precedent).
func (s *Service) requireRecoveryPorts() error {
	if s.recoveries == nil {
		return fmt.Errorf("%s", recoveryPortsMissingText)
	}
	if err := s.requirePreviewPorts(); err != nil {
		return err
	}
	return nil
}

// recoveryAssignmentInput reads the LIVE assignment words of the
// ORIGINAL Case: the current ownership term (the CaseView of the
// caseMaterial port — the term of the maximal epoch; NO terms = the
// honest owner-unknown refusal) and the human coverage of the original
// conversation route (the RouteFactsSource read — the coverageWord canon
// of the cases contour: the channel by the modality, the inbound
// candidate of the route; an unwired port or an unreadable route = the
// honest coverage-unavailable refusal). The words NEVER come from the
// command parameters — the derivation is material (AC-1: «assigns»
// never happens without the assignment).
func (s *Service) recoveryAssignmentInput(ctx context.Context, scope tenancy.Context, caseID id.UUID) (domain.RecoveryAssignmentInput, error) {
	in := domain.RecoveryAssignmentInput{}
	if s.caseMaterial == nil {
		return in, fmt.Errorf("%w: %s", port.ErrRecoveryCoverageUnavailable, domain.RecoveryCoverageUnavailableText)
	}
	view, err := s.caseMaterial.CaseView(ctx, identityDomain.Principal{ID: scope.PrincipalID()}, scope.TenantID(), caseID)
	if err != nil {
		return in, fmt.Errorf("%w: %s", port.ErrRecoveryCoverageUnavailable, domain.RecoveryCoverageUnavailableText)
	}
	if term := view.Ownership.State.Current; term != nil {
		in.HasOwnerTerm = true
		in.OwnerKind = term.OwnerKind
		in.OwnerRef = term.OwnerRef
		in.OwnerEpoch = term.Epoch
	}
	// The original conversation of the case (the one born with it — the
	// caseConnection canon of the cases app): the route facts of ITS
	// connection carry the human coverage of the channel.
	var original casesDomain.Conversation
	found := false
	for _, conversation := range view.Conversations {
		if conversation.CaseID == caseID {
			original = conversation
			found = true
			break
		}
	}
	if !found {
		return in, fmt.Errorf("%w: %s", port.ErrRecoveryCoverageUnavailable, domain.RecoveryCoverageUnavailableText)
	}
	if s.routeFacts == nil {
		return in, fmt.Errorf("%w: %s", port.ErrRecoveryCoverageUnavailable, domain.RecoveryCoverageUnavailableText)
	}
	routes, err := s.routeFacts.RouteFacts(ctx, scope, original.ConnectionID)
	if err != nil {
		return in, fmt.Errorf("%w: %s", port.ErrRecoveryCoverageUnavailable, domain.RecoveryCoverageUnavailableText)
	}
	channel := recoveryChannelOfModality(original.Modality)
	for _, candidate := range routes.Candidates {
		for _, coverage := range candidate.Coverage {
			if coverage.Route.Channel == channel && coverage.Route.Direction == "inbound" {
				in.CoverageReadable = true
				in.CoverageBackupOwnerID = coverage.BackupOwnerID
				in.CoverageAckDeadlineMinutes = coverage.AckDeadlineMinutes
				return in, nil
			}
		}
	}
	return in, fmt.Errorf("%w: %s", port.ErrRecoveryCoverageUnavailable, domain.RecoveryCoverageUnavailableText)
}

// recoveryChannelOfModality resolves the honest default channel of a
// modality (the channelOfModality canon of the cases app — the coverage
// lookup word of the conversation).
func recoveryChannelOfModality(modality string) string {
	if modality == "voice" {
		return "phone"
	}
	return "web_chat"
}

// buildRecoveryBundle builds the SAFE link-bundle of the frozen creation
// moment from the recorded facts and the pinned definition: pins,
// versions, counters, words and 64-hex digests ONLY (the material NEVER
// enters, AD-10/AC-9 — the sensitive payload stays in its own immutable
// preview rows).
func buildRecoveryBundle(facts port.RecoveryOperationFacts, owningSystem string) domain.RecoveryBundle {
	externalTime := ""
	if !facts.LatestExternalObjectTime.IsZero() {
		externalTime = facts.LatestExternalObjectTime.UTC().Format(time.RFC3339)
	}
	return domain.RecoveryBundle{
		CaseID:                facts.Operation.CaseID,
		WorkItemID:            facts.Operation.WorkItemID,
		OperationID:           facts.Operation.OperationID,
		DefinitionID:          facts.Operation.DefinitionID,
		DefinitionVer:         facts.Operation.DefinitionVersion,
		PreviewID:             facts.Operation.PreviewID,
		PreviewVersionNo:      facts.Operation.PreviewVersionNo,
		ConfirmationID:        facts.Operation.ConfirmationID,
		ConfirmationVersionNo: facts.Operation.ConfirmationVersionNo,
		OwnerKind:             facts.Operation.OwnerKind,
		OwnerRef:              facts.Operation.OwnerRef,
		OwnerEpoch:            facts.Operation.OwnerEpoch,
		ReleaseDecisionNo:     facts.Operation.ReleaseDecisionNo,
		ProfileVersion:        facts.Operation.ProfileVersion,
		BusinessIntentKey:     facts.Operation.BusinessIntentKey,
		IdempotencyKey:        facts.Operation.IdempotencyKey,
		MaterialDigest:        facts.Operation.MaterialDigest,
		DispatchAttempts:      facts.DispatchAttemptCount,
		ReconcileAttempts:     facts.ReconcileAttemptCount,
		ReceiptVersionsCount:  facts.ReceiptVersionsCount,
		LatestReceiptVersion:  facts.LatestReceiptVersionNo,
		LatestReceiptOutcome:  facts.LatestReceiptOutcome,
		Observations:          facts.Observations,
		TargetKind:            facts.Operation.Target.Kind,
		TargetRef:             facts.Operation.Target.Ref,
		OwningSystem:          owningSystem,
		ExternalObjectID:      facts.LatestExternalObjectID,
		ExternalObjectVersion: facts.LatestExternalObjectVersion,
		ExternalObjectTime:    externalTime,
	}
}

// CreateRecoveryCase is the EXPLICIT creation command of the recovery
// contour (NO daemon — the drain precedent): the eligibility ladder
// answers FIRST (the recorded facts — every gas is the honest refusal
// word, NOTHING is written), the idempotent pre-check follows (the
// existing case with the UNCHANGED digest returns — NO second row, NO
// event), the assignment of the LIVE readings gates the creation path
// (both fail-closed refusals write NOTHING), and the ONE creation
// transaction lands the case row, the created event and the outbox
// record. The raced twin resolves UNDER the recovery advisory lock by
// the idempotent/evidence decision — the loser's derivation is simply
// not needed.
func (s *Service) CreateRecoveryCase(ctx context.Context, command RecoveryCreateCommand) (RecoveryCreateResult, error) {
	if err := s.requireRecoveryPorts(); err != nil {
		return RecoveryCreateResult{}, err
	}
	scope, err := s.recoveryScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return RecoveryCreateResult{}, err
	}
	// The RECORDED facts (read-time, NO locks, NO writes): an unknown
	// operation (or the RLS blindness of a foreign tenant) answers the
	// honest 404 BEFORE any material.
	facts, err := s.recoveries.ReadRecoveryOperationFacts(ctx, scope, command.OperationID)
	if err != nil {
		if errors.Is(err, port.ErrRecoveryNotFound) {
			return RecoveryCreateResult{}, fmt.Errorf("%w: %s", port.ErrRecoveryNotFound, recoveryOperationUnknownText)
		}
		return RecoveryCreateResult{}, err
	}
	// The definition resolve of the operation pins (the honest refusal of
	// an unknown pin — the catalog is compile-time): the declaredness
	// halves and the delivery class word come from the pinned definition.
	def, ok := catalogDefinition(s, facts.Operation.DefinitionID, facts.Operation.DefinitionVersion)
	if !ok {
		return RecoveryCreateResult{}, domain.ErrNotFound
	}
	// The eligibility ladder over the recorded facts (the domain
	// derivation — the spec order): the LIVE declaration word read at the
	// command moment (the caps resolver — the composition root wires it),
	// the definition's query word, the wired fact of the reconciliation
	// adapter (AG-4 nil = the honest production boundary — the
	// reconciliation_unavailable path, the human is mandatory).
	liveWord := ""
	if s.caps != nil {
		if capabilityFacts, resolves := s.caps.Capability(def.RequiredCapability.ConnectorID,
			def.RequiredCapability.ConnectorVersion, def.RequiredCapability.CapabilityID); resolves {
			liveWord = capabilityFacts.Reconciliation
		}
	}
	eligibility, err := domain.DeriveRecoveryEligibility(domain.RecoveryEligibilityFacts{
		StateWord:               facts.StateWord,
		ReconcileAttemptCount:   facts.ReconcileAttemptCount,
		ReconciliationQueryWord: def.DeliveryContract.ReconciliationQuery,
		ReconciliationLiveWord:  liveWord,
		ReconciliationWired:     s.reconcileAdapter != nil,
		DispatchAttemptCount:    facts.DispatchAttemptCount,
		CapabilityClass:         def.DeliveryContract.CapabilityClass,
		GuaranteeRecorded:       facts.LastOutcomeGuarantee,
		NoEffectProven:          facts.NoEffectProven,
	})
	if err != nil {
		return RecoveryCreateResult{}, err
	}
	if !eligibility.Eligible() {
		// The honest refusal word of the gas — NOTHING is written (the
		// fake-repo counters of the unit matrix pin the zero-write
		// property on EVERY gas).
		return RecoveryCreateResult{}, fmt.Errorf("%w: %s", port.ErrRecoveryIneligible, eligibility.RefusalText)
	}
	// The HMAC key of the tenant (the per-tenant commitment key — the
	// recovery key and the bundle digest ride it, the ensure pattern).
	hmacKey, err := s.previews.ReadPreviewKey(ctx, scope, command.Now)
	if err != nil {
		return RecoveryCreateResult{}, err
	}
	recoveryKey := domain.DeriveRecoveryKey(hmacKey, scope.TenantID(), facts.Operation.OperationID)
	bundle := buildRecoveryBundle(facts, def.OwningSystem)
	bundleDigest := domain.DeriveBundleDigest(hmacKey, bundle)
	// The idempotent pre-check (the existing case read): the digest
	// comparator answers the repeat decision — the store re-runs it UNDER
	// the lock (the belt over braces; a raced twin resolves there).
	existing, err := s.recoveries.ReadRecoveryCaseByOperation(ctx, scope, command.OperationID)
	if err == nil {
		decision, cmpErr := domain.DeriveRecoveryRepeatDecision(existing.BundleDigest, bundleDigest)
		if cmpErr != nil {
			return RecoveryCreateResult{}, cmpErr
		}
		if decision == domain.RecoveryRepeatIdempotent {
			return RecoveryCreateResult{
				Decision: port.RecoveryDecisionIdempotent,
				Text:     domain.RecoveryIdempotentText,
				Case:     existing,
			}, nil
		}
		result, txErr := s.recoveries.CreateRecoveryCase(ctx, port.CreateRecoveryCaseParams{
			Scope:        scope,
			Operation:    facts.Operation,
			Trigger:      eligibility.Trigger,
			RecoveryKey:  recoveryKey,
			Bundle:       bundle,
			BundleDigest: bundleDigest,
			Now:          command.Now,
			ActorID:      command.Actor.ID,
		})
		if txErr != nil {
			return RecoveryCreateResult{}, txErr
		}
		return RecoveryCreateResult{Decision: result.Decision, Text: domain.RecoveryEvidenceAppendedText, Case: result.Case}, nil
	}
	if !errors.Is(err, port.ErrRecoveryNotFound) {
		return RecoveryCreateResult{}, err
	}
	// The creation path: the assignment of the LIVE readings ONLY (both
	// fail-closed refusals write NOTHING — the owner-unknown and the
	// coverage-unavailable words).
	assignment, err := s.recoveryAssignmentInput(ctx, scope, facts.Operation.CaseID)
	if err != nil {
		return RecoveryCreateResult{}, err
	}
	derived, err := domain.DeriveRecoveryAssignment(assignment)
	if err != nil {
		if strings.Contains(err.Error(), domain.RecoveryOwnerUnknownText) {
			return RecoveryCreateResult{}, fmt.Errorf("%w: %s", port.ErrRecoveryOwnerUnknown, domain.RecoveryOwnerUnknownText)
		}
		return RecoveryCreateResult{}, fmt.Errorf("%w: %s", port.ErrRecoveryCoverageUnavailable, domain.RecoveryCoverageUnavailableText)
	}
	deadlineAt := domain.DeriveRecoveryDeadlineAt(command.Now, derived.AckDeadlineMinutes)
	// The pre-minted identities of the case row and the outbox record.
	caseID, err := s.ids.New(command.Now)
	if err != nil {
		return RecoveryCreateResult{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return RecoveryCreateResult{}, err
	}
	result, err := s.recoveries.CreateRecoveryCase(ctx, port.CreateRecoveryCaseParams{
		Scope:          scope,
		Operation:      facts.Operation,
		Trigger:        eligibility.Trigger,
		RecoveryKey:    recoveryKey,
		Bundle:         bundle,
		BundleDigest:   bundleDigest,
		Assignment:     derived,
		DeadlineAt:     deadlineAt,
		RecoveryCaseID: caseID,
		OutboxEventID:  outboxEventID,
		ActorID:        command.Actor.ID,
		Now:            command.Now,
	})
	if err != nil {
		return RecoveryCreateResult{}, err
	}
	switch result.Decision {
	case port.RecoveryDecisionIdempotent:
		return RecoveryCreateResult{Decision: result.Decision, Text: domain.RecoveryIdempotentText, Case: result.Case}, nil
	case port.RecoveryDecisionEvidence:
		return RecoveryCreateResult{Decision: result.Decision, Text: domain.RecoveryEvidenceAppendedText, Case: result.Case}, nil
	default:
		return RecoveryCreateResult{Decision: result.Decision, Text: domain.RecoveryCreatedText, Case: result.Case}, nil
	}
}

// ReadRecoveryCase reads one Recovery Case for the card view (the
// operator surface of step-03): the case row with its append-only event
// history — the LIVING receipt chain and attempt facts derive at the
// read from the dispatch/reconciliation tables (AC-10: the card is
// always fresh, the frozen bundle never rewrites).
func (s *Service) ReadRecoveryCase(ctx context.Context, actor identityDomain.Principal, tenantID, recoveryCaseID id.UUID) (port.RecoveryCaseRow, error) {
	if s.recoveries == nil {
		return port.RecoveryCaseRow{}, fmt.Errorf("%s", recoveryPortsMissingText)
	}
	scope, err := s.recoveryScope(ctx, actor, tenantID)
	if err != nil {
		return port.RecoveryCaseRow{}, err
	}
	return s.recoveries.ReadRecoveryCase(ctx, scope, recoveryCaseID)
}

// ReadRecoveryCases reads the tenant's cases for the panel list (the
// operator surface of step-03), bounded by the panel limit.
func (s *Service) ReadRecoveryCases(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]port.RecoveryCaseRow, error) {
	if s.recoveries == nil {
		return nil, fmt.Errorf("%s", recoveryPortsMissingText)
	}
	scope, err := s.recoveryScope(ctx, actor, tenantID)
	if err != nil {
		return nil, err
	}
	return s.recoveries.ReadRecoveryCases(ctx, scope, port.RecoveryPanelLimit)
}
