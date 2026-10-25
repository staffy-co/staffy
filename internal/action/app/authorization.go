package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/artemiosu/staffy/internal/action/api"
	actionCatalog "github.com/artemiosu/staffy/internal/action/catalog"
	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.5 (step-02): the confirmation→authorization application contour
// ---------------------------------------------------------------------------
//
// The three commands and the view run the FROZEN gate order of the
// Action Gateway. The confirm command is the HUMAN act (the LIVE
// step-up — the Approve precedent of 3-3); the authorize command is the
// MACHINE surface of the repeats (the webhook/retry/replay jobs cannot
// repeat a step-up — the consent is ALREADY materialized in the
// confirmation row and checked by the pin, the documented decision): the
// replay check runs FIRST — an already-authorized operation answers the
// idempotent return whatever the confirmation's current state is (the
// first authorization consumed the consent honestly), and EVERY refusal
// of the ordered ladder happens BEFORE the ONE transaction and consumes
// NOTHING (the unit counters pin the order).
//
// The single_use/unlimited consumption vertical: the bounded rule fails
// closed (the numeric carrier does not exist in the 3-1 definitions —
// the honest reserved word); the raced second authorization of one
// single_use consent sees the first one's consumed event under the
// advisory lock and refuses honestly WITHOUT consuming.

// authorizationScopeText is the honest scope refusal of the contour: the
// same two scopes of the preparation (the 3-4 discipline — the words
// BEFORE any material).
const authorizationScopeText = "Подтверждение и авторизация действия требуют полномочия review или configuration: участник только с audit не подтверждает, не отзывает и не авторизует действия."

// authorizationPortsMissingText is the honest refusal of the unwired
// contour (the preview precedent).
const authorizationPortsMissingText = "Контур подтверждения и авторизации не сконфигурирован: живые источники не подключены, команда честно отклоняется."

// The compile-time proof of the narrow interface registration (the
// transport type-asserts api.AuthorizationService — the PreviewService
// precedent).
var _ api.AuthorizationService = (*Service)(nil)

// requireAuthorizationPorts refuses honestly when the contour is not
// wired: the two repositories of the contour are mandatory besides the
// LIVE fact sources of the preparation (the same WithPreviews set — the
// freshness/policy re-checks read them).
func (s *Service) requireAuthorizationPorts() error {
	if s.confirmations == nil || s.operations == nil {
		return fmt.Errorf("%s", authorizationPortsMissingText)
	}
	if err := s.requirePreviewPorts(); err != nil {
		return err
	}
	return nil
}

// currentOwnerTriple reads the LIVE response-owner triple of the case
// from the ownership terms of the case view (the max-epoch term — the
// CurrentOwnership semantics through the api projection, the 2-4
// dictionary): the consent snapshots the owner the case HAD at the
// confirmation moment. The freshness derivation re-checks the epoch
// against the live sources at every authorization. A case without a
// readable owner term fails CLOSED (the store CHECK demands a real
// triple — the honest refusal happens here, never at the database).
func currentOwnerTriple(view casesAPI.CaseView) (kind string, ref id.UUID, epoch int64, err error) {
	var bestEpoch int64
	for _, term := range view.Ownership.Terms {
		if term.Epoch >= bestEpoch {
			bestEpoch = term.Epoch
			kind = term.OwnerKind
			ref = term.OwnerRef
		}
	}
	if kind == "" || !ref.Valid() || bestEpoch < 1 || !domain.WordIn(domain.OwnerKindWords, kind) {
		return "", id.UUID(""), 0, domain.RuleError{
			Rule:   "currentOwnerTriple",
			Reason: "Владелец ответа кейса не читается живым чтением — согласие не может запинить владельца (fail-closed).",
		}
	}
	return kind, ref, bestEpoch, nil
}

// routeReleaseSnapshot reads the release lineage snapshot of the case's
// connection (the RouteFacts facade — the 3-3 lineageWord precedent): the
// acting release decision number and the sealed profile version are the
// moment pins of the consent. A case without a conversation or a route
// without candidates answers domain.ErrNotFound — the strict consent
// refuses honestly (the release snapshot is a required pin, the store
// CHECK demands non-negative numbers).
func (s *Service) routeReleaseSnapshot(ctx context.Context, scope tenancy.Context, material caseViewMaterial) (decisionNo, profileVersion int64, err error) {
	connectionID, ok := material.routeConnectionID()
	if !ok {
		return 0, 0, domain.ErrNotFound
	}
	routes, err := s.routeFacts.RouteFacts(ctx, scope, connectionID)
	if err != nil {
		return 0, 0, err
	}
	if len(routes.Candidates) == 0 {
		return 0, 0, domain.ErrNotFound
	}
	return routes.Candidates[0].DecisionNo, routes.Candidates[0].ProfileVersion, nil
}

// consumptionWords speaks the consumption of one confirmation version
// honestly: single_use — «потреблено N из 1»; unlimited — the word of the
// unbounded allowance; the bounded rule never reaches here (the confirm
// guard refused it). The count is the read-time derivation of the
// consumed events.
func consumptionWords(rule string, consumed int) string {
	allowance, err := domain.DeriveConsumptionAllowance(rule)
	if err != nil || !allowance.Bounded {
		return fmt.Sprintf("Правило потребления unlimited: потреблено %d — исчерпание невозможно.", consumed)
	}
	return fmt.Sprintf("Потреблено %d из %d.", consumed, allowance.Limit)
}

// ConfirmPreviewVersion runs the consent chain of one EXACT preview
// version (the frozen order): the ports → the rights gate → the LIVE
// step-up (the human act — the Approve precedent) → the LIVE preview row
// (the honest 404) → the definition resolve (the required/allowance
// guards — a not_required query definition honestly never enters this
// contour) → the LIVE case material → the freshness gate (NO reasons —
// the consent of dead material is meaningless) → the readiness gate
// (ready only — the blocked material is never consented) → the ONE
// append transaction (the version with the linked material copies, the
// confirmed event of the new version, the superseded event of the old
// one — atomic).
func (s *Service) ConfirmPreviewVersion(ctx context.Context, command api.ConfirmPreviewVersionCommand) (api.ConfirmationVersionOutcome, error) {
	if err := s.requireAuthorizationPorts(); err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	scope, err := s.authorizeProposal(ctx, command.Actor, command.TenantID)
	if err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	if !command.Actor.StepUpVerified {
		// The human act of the consent: the LIVE step-up of the session.
		return api.ConfirmationVersionOutcome{}, identityDomain.ErrStepUpRequired
	}
	// The LIVE preview row of the exact pin (the honest 404 of a dead pin
	// BEFORE any material).
	preview, err := s.operations.ReadPinnedPreviewVersion(ctx, scope, command.PreviewID, command.PreviewVersionNo)
	if err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	if preview.CaseID != command.CaseID {
		// The pin does not anchor to the named case: the honest 404.
		return api.ConfirmationVersionOutcome{}, domain.ErrNotFound
	}
	def, ok := actionCatalog.GetFrom(s.defs, preview.DefinitionID, preview.DefinitionVersion)
	if !ok {
		return api.ConfirmationVersionOutcome{}, domain.ErrNotFound
	}
	if err := domain.GuardConfirm(def); err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	// The LIVE case material: the case must exist (the AD-2 discipline —
	// the honest 404 BEFORE the append transaction).
	caseView, err := s.caseMaterial.CaseView(ctx, command.Actor, command.TenantID, command.CaseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return api.ConfirmationVersionOutcome{}, domain.ErrNotFound
		}
		return api.ConfirmationVersionOutcome{}, err
	}
	material := caseViewMaterial{view: caseView}
	// The freshness gate: NO reasons (the 3-4 derivation reused — «3-5
	// читает тот же вывод», the promised single dictionary).
	live, err := s.gatherPreviewLive(ctx, command.Actor, scope, def, material)
	if err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	facts, err := s.caseMaterial.FactsView(ctx, command.Actor, command.TenantID, command.CaseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return api.ConfirmationVersionOutcome{}, domain.ErrNotFound
		}
		return api.ConfirmationVersionOutcome{}, err
	}
	fresh, err := s.previewFreshnessLive(ctx, command.Actor, scope, preview, material, live, facts)
	if err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	if derivation := domain.DerivePreviewFreshness(preview, fresh); derivation.Stale {
		return api.ConfirmationVersionOutcome{}, domain.RuleError{
			Rule:   "ConfirmPreviewVersion",
			Reason: domain.FreshnessReasonText(derivation.Reason),
		}
	}
	// The readiness gate: ready only (the nil reader of the production
	// root honestly blocks — AG-4).
	if readiness := domain.DerivePreviewReadiness(def, preview.Snapshot); readiness != domain.PreviewReady {
		return api.ConfirmationVersionOutcome{}, domain.RuleError{
			Rule:   "ConfirmPreviewVersion",
			Reason: domain.PreviewReadinessText(readiness),
		}
	}
	// The owner triple and the release snapshot of the consent moment.
	ownerKind, ownerRef, ownerEpoch, err := currentOwnerTriple(caseView)
	if err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	decisionNo, profileVersion, err := s.routeReleaseSnapshot(ctx, scope, material)
	if err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	// The thread resolve: the newest confirmation version of the preview
	// thread — the re-confirmation after a preview edit is version+1 of
	// the SAME thread (the 2-9/3-4 append-only pattern); the first
	// consent mints the stable identity.
	thread, err := s.confirmations.ReadPreviewConfirmations(ctx, scope, command.PreviewID)
	if err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	confirmationID := id.UUID("")
	prevVersion := int64(0)
	for _, stored := range thread {
		if stored.VersionNo > prevVersion {
			prevVersion = stored.VersionNo
			confirmationID = stored.ConfirmationID
		}
	}
	if !confirmationID.Valid() {
		if confirmationID, err = s.ids.New(command.Now); err != nil {
			return api.ConfirmationVersionOutcome{}, err
		}
	}
	confirmedEventID, err := s.ids.New(command.Now)
	if err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	supersededEventID := id.UUID("")
	if prevVersion > 0 {
		if supersededEventID, err = s.ids.New(command.Now); err != nil {
			return api.ConfirmationVersionOutcome{}, err
		}
	}
	confirmation := domain.Confirmation{
		TenantID:          scope.TenantID(),
		ConfirmationID:    confirmationID,
		VersionNo:         prevVersion + 1,
		DefinitionID:      preview.DefinitionID,
		DefinitionVersion: preview.DefinitionVersion,
		CaseID:            preview.CaseID,
		WorkItemID:        preview.WorkItemID,
		Target:            preview.Target,
		OwnerKind:         ownerKind,
		OwnerRef:          ownerRef,
		OwnerEpoch:        ownerEpoch,
		ReleaseDecisionNo: decisionNo,
		ProfileVersion:    profileVersion,
		PreviewID:         preview.PreviewID,
		PreviewVersionNo:  preview.VersionNo,
		PreviewDigest:     preview.Digest,
		Snapshot:          preview.Snapshot,
		Effects:           preview.Effects,
		ValidUntil:        domain.DeriveValidUntil(command.Now, command.LifetimeSeconds),
		ConfirmedBy:       command.Actor.ID,
		CreatedAt:         command.Now.UTC().Truncate(time.Microsecond),
	}
	if err := s.confirmations.AppendConfirmationVersion(ctx, port.AppendConfirmationParams{
		Scope:             scope,
		Confirmation:      confirmation,
		PrevVersionNo:     prevVersion,
		ConfirmedEventID:  confirmedEventID,
		SupersededEventID: supersededEventID,
		Now:               command.Now,
	}); err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	events, err := s.confirmations.ReadConfirmationVersionEvents(ctx, scope, confirmationID, confirmation.VersionNo)
	if err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	state, err := domain.DeriveConfirmationState(def.Conditions.ConsumptionRule, confirmation.ValidUntil, events, command.Now)
	if err != nil {
		return api.ConfirmationVersionOutcome{}, err
	}
	return api.ConfirmationVersionOutcome{
		Command:          "confirm_preview_version",
		ConfirmationID:   confirmationID,
		VersionNo:        confirmation.VersionNo,
		State:            state,
		StateText:        domain.ConfirmationStateText(state),
		ValidUntil:       confirmation.ValidUntil,
		ConsumptionText:  consumptionWords(def.Conditions.ConsumptionRule, domain.DeriveConsumedCount(events)),
		PreviewID:        preview.PreviewID,
		PreviewVersionNo: preview.VersionNo,
	}, nil
}

// RevokeConfirmation appends the human revocation of one exact
// confirmation version: the LIVE step-up (the human act), the pin resolve
// (the honest 404), the definition gate (a not_required definition has
// no consent to revoke) and the ONE event append. The revocation kills
// the FUTURE authorizations of the consent; the PAST consumption stays
// the audit fact — nothing is ever un-consumed.
func (s *Service) RevokeConfirmation(ctx context.Context, command api.RevokeConfirmationCommand) (api.ConfirmationRevocationOutcome, error) {
	if err := s.requireAuthorizationPorts(); err != nil {
		return api.ConfirmationRevocationOutcome{}, err
	}
	scope, err := s.authorizeProposal(ctx, command.Actor, command.TenantID)
	if err != nil {
		return api.ConfirmationRevocationOutcome{}, err
	}
	if !command.Actor.StepUpVerified {
		return api.ConfirmationRevocationOutcome{}, identityDomain.ErrStepUpRequired
	}
	confirmation, err := s.confirmations.ReadConfirmationVersion(ctx, scope, command.ConfirmationID, command.VersionNo)
	if err != nil {
		return api.ConfirmationRevocationOutcome{}, err
	}
	if confirmation.CaseID != command.CaseID {
		return api.ConfirmationRevocationOutcome{}, domain.ErrNotFound
	}
	def, ok := actionCatalog.GetFrom(s.defs, confirmation.DefinitionID, confirmation.DefinitionVersion)
	if !ok {
		return api.ConfirmationRevocationOutcome{}, domain.ErrNotFound
	}
	if err := domain.GuardRevoke(def); err != nil {
		return api.ConfirmationRevocationOutcome{}, err
	}
	eventID, err := s.ids.New(command.Now)
	if err != nil {
		return api.ConfirmationRevocationOutcome{}, err
	}
	if err := s.confirmations.AppendConfirmationEvent(ctx, port.AppendConfirmationEventParams{
		Scope:          scope,
		ConfirmationID: command.ConfirmationID,
		VersionNo:      command.VersionNo,
		CaseID:         command.CaseID,
		EventID:        eventID,
		Kind:           domain.ConfirmationEventRevoked,
		Reason:         domain.ConfirmationEventRevoked,
		Details:        "отзыв человеком с step-up",
		ActorID:        command.Actor.ID,
		Now:            command.Now,
	}); err != nil {
		return api.ConfirmationRevocationOutcome{}, err
	}
	return api.ConfirmationRevocationOutcome{
		Command:        "revoke_confirmation",
		ConfirmationID: command.ConfirmationID,
		VersionNo:      command.VersionNo,
		State:          domain.ConfirmationRevoked,
		StateText:      domain.ConfirmationStateText(domain.ConfirmationRevoked),
		RevocationNote: "Прошлое потребление остаётся фактом аудита: отзыв гасит будущие авторизации и никогда не отменяет потреблённое.",
	}, nil
}

// appendAuthorizationConflict appends the VISIBLE conflict exception of
// the idempotency comparison (AC-8): the same client key arrived with a
// different material — the exception is visible to the authorized owner.
// The details are the SAFE words only (the client key and the honest
// mismatch word).
func (s *Service) appendAuthorizationConflict(ctx context.Context, scope tenancy.Context, command api.AuthorizeOperationCommand, keyMismatch string) error {
	eventID, err := s.ids.New(command.Now)
	if err != nil {
		return err
	}
	return s.confirmations.AppendConfirmationEvent(ctx, port.AppendConfirmationEventParams{
		Scope:          scope,
		ConfirmationID: command.ConfirmationID,
		VersionNo:      command.ConfirmationVersionNo,
		CaseID:         command.CaseID,
		EventID:        eventID,
		Kind:           domain.ConfirmationEventAuthorizationConflict,
		Reason:         domain.ConfirmationEventAuthorizationConflict,
		Details:        "idempotency key " + command.IdempotencyKey + ": " + keyMismatch,
		ActorID:        command.Actor.ID,
		Now:            command.Now,
	})
}

// appendAuthorizationBlocked appends the VISIBLE audit word of a rejected
// authorization attempt of the rejection ladder (the consent-scoped
// visible exception — the reason carries the exact state word). The
// consumption NEVER happens on a rejection.
func (s *Service) appendAuthorizationBlocked(ctx context.Context, scope tenancy.Context, command api.AuthorizeOperationCommand, stateWord string) error {
	eventID, err := s.ids.New(command.Now)
	if err != nil {
		return err
	}
	return s.confirmations.AppendConfirmationEvent(ctx, port.AppendConfirmationEventParams{
		Scope:          scope,
		ConfirmationID: command.ConfirmationID,
		VersionNo:      command.ConfirmationVersionNo,
		CaseID:         command.CaseID,
		EventID:        eventID,
		Kind:           domain.ConfirmationEventAuthorizationBlocked,
		Reason:         stateWord,
		Details:        "авторизация отвергнута: " + stateWord,
		ActorID:        command.Actor.ID,
		Now:            command.Now,
	})
}

// AuthorizeOperation runs the FULL ordered gateway (the Action Gateway —
// the order is part of the contract, the unit counters pin it): the
// ports → the rights gate → the machine key gate (charset/length BEFORE
// everything) → the consent pin resolve (the honest 404) → THE REPLAY
// CHECK FIRST (the operation of the client key: the digest comparison
// answers the idempotent return of the ORIGINAL operation or the honest
// 409 conflict with the visible exception — whatever the consent's
// current state is) → the rejection ladder BY NAME (no consumption) →
// the live preview digest re-check (the double digest protection) → the
// freshness re-check (the 3-4 dictionary) → the readiness re-check → the
// LIVE policy gates (GuardPreparePolicy reused) → the work item anchor →
// the LAST material check (the owning-system fresh reads; the nil reader
// of the production root fails closed — AG-4) → the key derivations →
// THE ONE ReserveOperation transaction.
func (s *Service) AuthorizeOperation(ctx context.Context, command api.AuthorizeOperationCommand) (api.AuthorizationOutcome, error) {
	if err := s.requireAuthorizationPorts(); err != nil {
		return api.AuthorizationOutcome{}, err
	}
	scope, err := s.authorizeProposal(ctx, command.Actor, command.TenantID)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	// The machine key gate: charset/length BEFORE everything (AC-2).
	if err := domain.GuardAuthorize(command.IdempotencyKey); err != nil {
		return api.AuthorizationOutcome{}, err
	}
	// The consent pin resolve (the honest 404 BEFORE the replay
	// comparison — the material digest derives over the pinned rows).
	confirmation, err := s.confirmations.ReadConfirmationVersion(ctx, scope, command.ConfirmationID, command.ConfirmationVersionNo)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	if confirmation.CaseID != command.CaseID {
		return api.AuthorizationOutcome{}, domain.ErrNotFound
	}
	// The keyed material digest of the request pins (the same per-tenant
	// commitment key of the preparation — the ensure pattern).
	hmacKey, err := s.previews.ReadPreviewKey(ctx, scope, command.Now)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	requestDigest := domain.DeriveIdempotencyFingerprint(hmacKey, scope.TenantID(), domain.OperationRequestPins{
		PreviewID:             confirmation.PreviewID,
		PreviewVersionNo:      confirmation.PreviewVersionNo,
		ConfirmationID:        confirmation.ConfirmationID,
		ConfirmationVersionNo: confirmation.VersionNo,
	})
	// THE REPLAY CHECK FIRST: an operation of this client key exists —
	// the first authorization already succeeded (whatever the consent's
	// state is NOW — the expired consent of an authorized operation is
	// the honest replay, never the 409-exhausted of a double-click).
	existing, found, err := s.operations.FindOperationByIdempotencyKey(ctx, scope, command.IdempotencyKey)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	if found {
		if existing.MaterialDigest == requestDigest {
			// The idempotent return of the ORIGINAL operation (AC-7): NO
			// consumption, NO second outbox record, NO new operation.
			card, err := s.operationCard(ctx, scope, existing)
			if err != nil {
				return api.AuthorizationOutcome{}, err
			}
			return api.AuthorizationOutcome{
				Command:     "authorize_operation",
				Replay:      true,
				OutcomeText: domain.IdempotentReplayText,
				Operation:   card,
			}, nil
		}
		// The honest 409 conflict (AC-8): the same key, a different
		// material — the visible exception lands, the original operation
		// stays unchanged.
		if err := s.appendAuthorizationConflict(ctx, scope, command, "материал разошёлся с первоначальной Operation"); err != nil {
			return api.AuthorizationOutcome{}, err
		}
		return api.AuthorizationOutcome{}, fmt.Errorf("%w: %s", port.ErrIdempotencyConflict, domain.IdempotencyConflictText)
	}
	// The definition resolve of the consent pins (the honest 404).
	def, ok := actionCatalog.GetFrom(s.defs, confirmation.DefinitionID, confirmation.DefinitionVersion)
	if !ok {
		return api.AuthorizationOutcome{}, domain.ErrNotFound
	}
	// THE STRUCTURAL GATE of story 4.8 (AC-9/AC-10: «новый дублирующий
	// заказ не создаётся»): an OPEN order correction of the case refuses
	// the authorization of the NEW create_abcp_customer_order BEFORE the
	// reservation transaction — NOTHING is consumed (the replay
	// idempotency above stays untouched: the ORIGINAL operation's replay
	// answers honestly; the 3-4 superseded ladder below stays untouched:
	// the gate precedes the rejection ladder and the consumption). The
	// single_use consumption is per confirmation, never per case — the
	// gate is the STRUCTURAL close of the duplicate-order hole. A NIL
	// correction repository is the honest unwired contour — no
	// corrections exist, the gate holds nothing.
	if confirmation.DefinitionID == domain.OrderActionID && s.corrections != nil {
		if open := s.openOrderCorrectionOfCase(ctx, scope, command.CaseID, command.Now); open != nil {
			return api.AuthorizationOutcome{}, domain.ErrOrderCorrectionOpenGate
		}
	}
	// The rejection ladder BY NAME (AC-1): superseded beats revoked beats
	// expired beats exhausted — every rejection speaks the exact word,
	// the consumption NEVER happens (the counter probes).
	events, err := s.confirmations.ReadConfirmationVersionEvents(ctx, scope, command.ConfirmationID, command.ConfirmationVersionNo)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	state, err := domain.DeriveConfirmationState(def.Conditions.ConsumptionRule, confirmation.ValidUntil, events, command.Now)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	if state != domain.ConfirmationValid {
		if err := s.appendAuthorizationBlocked(ctx, scope, command, state); err != nil {
			return api.AuthorizationOutcome{}, err
		}
		return api.AuthorizationOutcome{}, port.ConfirmationRejectedError{State: state}
	}
	// The double digest protection: the LIVE preview row against the
	// linked copy of the consent (an edited preview is a NEW version —
	// the old consent is superseded by the ladder; the digest is the
	// second structural layer).
	livePreview, err := s.operations.ReadPinnedPreviewVersion(ctx, scope, confirmation.PreviewID, confirmation.PreviewVersionNo)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	if livePreview.Digest != confirmation.PreviewDigest {
		return api.AuthorizationOutcome{}, fmt.Errorf("%w: %s", port.ErrPreviewDigestChanged, domain.ConfirmationPreviewChangedText)
	}
	// The LIVE case material (the freshness/policy gates read it; the
	// work item anchor check follows the policy per the frozen order).
	caseView, err := s.caseMaterial.CaseView(ctx, command.Actor, command.TenantID, command.CaseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return api.AuthorizationOutcome{}, domain.ErrNotFound
		}
		return api.AuthorizationOutcome{}, err
	}
	material := caseViewMaterial{view: caseView}
	// The freshness re-check (the 3-4 derivation reused — the same
	// dictionary, the same words; the consumption NEVER happens on a
	// refusal).
	live, err := s.gatherPreviewLive(ctx, command.Actor, scope, def, material)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	facts, err := s.caseMaterial.FactsView(ctx, command.Actor, command.TenantID, command.CaseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return api.AuthorizationOutcome{}, domain.ErrNotFound
		}
		return api.AuthorizationOutcome{}, err
	}
	fresh, err := s.previewFreshnessLive(ctx, command.Actor, scope, livePreview, material, live, facts)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	if derivation := domain.DerivePreviewFreshness(livePreview, fresh); derivation.Stale {
		return api.AuthorizationOutcome{}, port.AuthorizationStaleError{Reason: domain.FreshnessReasonText(derivation.Reason)}
	}
	// The readiness re-check: the blocked material is never authorized.
	if readiness := domain.DerivePreviewReadiness(def, livePreview.Snapshot); readiness != domain.PreviewReady {
		return api.AuthorizationOutcome{}, fmt.Errorf("%w: %s", port.ErrPreviewBlockedMaterial, domain.PreviewReadinessText(readiness))
	}
	// The LIVE policy gates (GuardPreparePolicy reused — the pinned
	// owner epoch of the CONSENT against the live sources; the block
	// speaks the exact reason word, the consumption NEVER happens).
	policy := domain.GuardPreparePolicy(def, confirmation.OwnerEpoch, live.policy)
	if policy.Outcome == domain.PolicyBlock {
		return api.AuthorizationOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, domain.PolicyReasonText(policy.Reason))
	}
	// The work item anchor (the honest 404 of a dead reference BEFORE
	// the material check).
	if !material.workItemAnchored(confirmation.WorkItemID) {
		return api.AuthorizationOutcome{}, domain.ErrNotFound
	}
	// The LAST material check (AC-3/AG-4): the definition's required
	// facts read through the consumer-owned reader; the statuses
	// re-map against the DECLARED tolerance. The NIL reader of the
	// production root fails CLOSED — the authorization honestly cannot
	// happen (the fixture world wires the synthetic reader).
	if len(def.Conditions.RequiredFacts) > 0 {
		if s.previewReader == nil {
			return api.AuthorizationOutcome{}, fmt.Errorf("%w: %s", port.ErrFreshReadsUnavailable, domain.FreshReadsUnavailableText)
		}
		rows := s.readPreviewMaterial(ctx, scope, def, livePreview)
		byField := map[string]domain.MaterialSnapshotRow{}
		for _, row := range rows {
			byField[row.MaterialField] = row
		}
		for _, required := range def.Conditions.RequiredFacts {
			row, present := byField[required.MaterialField]
			if !present {
				// The required field never answered: the honest 409 of the
				// unverifiable condition (fail-closed).
				return api.AuthorizationOutcome{}, fmt.Errorf("%w: %s", port.ErrMaterialConditionChanged, domain.MaterialConditionChangedText)
			}
			switch row.Status {
			case domain.SnapshotFresh, domain.SnapshotAllowedUncertain:
				// The fresh field passes; the ALLOWED uncertainty passes
				// EXPLICITLY by the definition's declaration.
			case domain.SnapshotStale, domain.SnapshotConflicting, domain.SnapshotUnknown:
				return api.AuthorizationOutcome{}, fmt.Errorf("%w: %s", port.ErrMaterialConditionChanged, domain.MaterialConditionChangedText)
			default:
				// A foreign status word fails closed (a lying adapter can never
				// smuggle a pass in).
				return api.AuthorizationOutcome{}, fmt.Errorf("%w: %s", port.ErrMaterialConditionChanged, domain.MaterialConditionChangedText)
			}
		}
	}
	// The two key derivations (FR-29 — the identities are NOT
	// interchangeable): the server business intent over the normalized
	// material (the definition version NEVER enters) and the client
	// request digest over the pins (already derived above).
	intentKey := domain.DeriveBusinessIntentKey(hmacKey, domain.BusinessIntentInput{
		TenantID:     scope.TenantID(),
		OwningSystem: def.OwningSystem,
		DefinitionID: def.ID,
		Target:       livePreview.Target,
		TypedInput:   livePreview.TypedInput,
		Snapshot:     livePreview.Snapshot,
	})
	operationID, err := s.ids.New(command.Now)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	consumedEventID, err := s.ids.New(command.Now)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	operation := domain.Operation{
		TenantID:              scope.TenantID(),
		OperationID:           operationID,
		DefinitionID:          confirmation.DefinitionID,
		DefinitionVersion:     confirmation.DefinitionVersion,
		PreviewID:             confirmation.PreviewID,
		PreviewVersionNo:      confirmation.PreviewVersionNo,
		ConfirmationID:        confirmation.ConfirmationID,
		ConfirmationVersionNo: confirmation.VersionNo,
		CaseID:                confirmation.CaseID,
		WorkItemID:            confirmation.WorkItemID,
		Target:                confirmation.Target,
		OwnerKind:             confirmation.OwnerKind,
		OwnerRef:              confirmation.OwnerRef,
		OwnerEpoch:            confirmation.OwnerEpoch,
		ReleaseDecisionNo:     confirmation.ReleaseDecisionNo,
		ProfileVersion:        confirmation.ProfileVersion,
		Partition:             livePreview.Partition.Partition,
		Effects:               confirmation.Effects,
		BusinessIntentKey:     intentKey,
		IdempotencyKey:        command.IdempotencyKey,
		MaterialDigest:        requestDigest,
		CreatedAt:             command.Now.UTC().Truncate(time.Microsecond),
	}
	// THE ONE TRANSACTION (persist-before-emit — the signature carries
	// ZERO adapters): consumed → reservation → operation → ledger →
	// outbox, ANY failure rolls back everything.
	err = s.operations.ReserveOperation(ctx, port.ReserveOperationParams{
		Scope:           scope,
		Operation:       operation,
		ConsumptionRule: def.Conditions.ConsumptionRule,
		ActorID:         command.Actor.ID,
		ConsumedEventID: consumedEventID,
		LedgerEntry: domain.OperationLedgerEntry{
			EntryNo: 1,
			Kind:    domain.LedgerEntryAuthorized,
			Details: "авторизация закреплена транзакцией ReserveOperation",
		},
		OutboxEventID: outboxEventID,
		OutboxPayload: port.OperationQueuedPayload{
			EventType:             "operation.queued",
			TenantID:              scope.TenantID(),
			OperationID:           operationID,
			CaseID:                confirmation.CaseID,
			WorkItemID:            confirmation.WorkItemID,
			DefinitionID:          confirmation.DefinitionID,
			DefinitionVersion:     confirmation.DefinitionVersion,
			OwningSystem:          def.OwningSystem,
			PreviewID:             confirmation.PreviewID,
			PreviewVersionNo:      confirmation.PreviewVersionNo,
			ConfirmationID:        confirmation.ConfirmationID,
			ConfirmationVersionNo: confirmation.VersionNo,
			TargetKind:            confirmation.Target.Kind,
			TargetRef:             confirmation.Target.Ref,
			OwnerKind:             confirmation.OwnerKind,
			OwnerRef:              confirmation.OwnerRef,
			Partition:             livePreview.Partition.Partition,
			BusinessIntentKey:     intentKey,
			IdempotencyKey:        command.IdempotencyKey,
			MaterialDigest:        requestDigest,
			QueuedAt:              command.Now.UTC().Truncate(time.Microsecond),
		},
		Now: command.Now,
	})
	if err != nil {
		switch {
		case errors.Is(err, port.ErrIdempotencyTwin):
			// The raced twin of one client key: the winner committed
			// first — the honest replay comparison decides (the twin's
			// transaction rolled back: nothing consumed, nothing queued).
			winner, found, readErr := s.operations.FindOperationByIdempotencyKey(ctx, scope, command.IdempotencyKey)
			if readErr != nil {
				return api.AuthorizationOutcome{}, readErr
			}
			if !found {
				return api.AuthorizationOutcome{}, port.ErrOperationIntegrity
			}
			if winner.MaterialDigest != requestDigest {
				if conflictErr := s.appendAuthorizationConflict(ctx, scope, command, "материал разошёлся с выигравшей близнецом транзакцией"); conflictErr != nil {
					return api.AuthorizationOutcome{}, conflictErr
				}
				return api.AuthorizationOutcome{}, fmt.Errorf("%w: %s", port.ErrIdempotencyConflict, domain.IdempotencyConflictText)
			}
			card, cardErr := s.operationCard(ctx, scope, winner)
			if cardErr != nil {
				return api.AuthorizationOutcome{}, cardErr
			}
			return api.AuthorizationOutcome{
				Command:     "authorize_operation",
				Replay:      true,
				OutcomeText: domain.IdempotentReplayText,
				Operation:   card,
			}, nil
		case errors.Is(err, port.ErrReservationConflict):
			// The reservation race: EITHER the cross-case loser (a
			// DIFFERENT request, an equivalent business intent — AC-9)
			// OR the same-material twin of one client key (AC-7 — the
			// reservation constraint fires before the operation insert,
			// so the raced same-material twin arrives here first). The
			// digest comparison separates the two identities honestly:
			// the SAME key with the SAME material is the replay of the
			// ORIGINAL request; a different key (or a different
			// material) reads the honest reference. Either way the
			// loser's consumption NEVER happened (the whole transaction
			// rolled back).
			existingByIntent, foundByIntent, readErr := s.operations.FindOperationByIntent(ctx, scope, def.OwningSystem, intentKey)
			if readErr != nil {
				return api.AuthorizationOutcome{}, readErr
			}
			if !foundByIntent {
				return api.AuthorizationOutcome{}, port.ErrOperationIntegrity
			}
			card, cardErr := s.operationCard(ctx, scope, existingByIntent)
			if cardErr != nil {
				return api.AuthorizationOutcome{}, cardErr
			}
			if existingByIntent.IdempotencyKey == command.IdempotencyKey && existingByIntent.MaterialDigest == requestDigest {
				return api.AuthorizationOutcome{
					Command:     "authorize_operation",
					Replay:      true,
					OutcomeText: domain.IdempotentReplayText,
					Operation:   card,
				}, nil
			}
			return api.AuthorizationOutcome{
				Command:              "authorize_operation",
				ReservationReference: true,
				OutcomeText:          domain.ReservationDuplicateText,
				Operation:            card,
			}, nil
		case errors.Is(err, port.ErrConfirmationExhausted):
			// The raced consumption under the lock: the honest exhausted
			// refusal WITHOUT consuming (the transaction rolled back).
			return api.AuthorizationOutcome{}, port.ConfirmationRejectedError{State: domain.ConfirmationExhausted}
		default:
			return api.AuthorizationOutcome{}, err
		}
	}
	card, err := s.operationCard(ctx, scope, operation)
	if err != nil {
		return api.AuthorizationOutcome{}, err
	}
	return api.AuthorizationOutcome{
		Command:     "authorize_operation",
		OutcomeText: domain.OperationAuthorizedText + " " + domain.OperationQueuedText,
		Operation:   card,
	}, nil
}

// operationCard renders the full card of one immutable operation: the
// live life-state derivation over the append-only ledger (the six-word
// dictionary — 3-6 step-03: the queue is the structured FACT, the spoken
// pair-word of 3-5 matured into the life words), the state moment (the
// time of the last state-bearing entry), the Owning System word of the
// pinned definition, the honest next-step text, the visible block cause
// (the latest dispatch_blocked entry — the operation stays authorized,
// parked with a visible cause) and the dispatch view extension (the
// progressive attempts and the Action Receipt versions — SAFE reads of
// the execution contour's own append-only tables; the unwired contour
// renders the honest empty lists).
func (s *Service) operationCard(ctx context.Context, scope tenancy.Context, op domain.Operation) (api.OperationCardView, error) {
	ledger, err := s.operations.ReadOperationLedger(ctx, scope, op.OperationID)
	if err != nil {
		return api.OperationCardView{}, err
	}
	queued, err := s.operations.ReadOutboxQueued(ctx, scope, op.OperationID)
	if err != nil {
		return api.OperationCardView{}, err
	}
	state, err := domain.DeriveOperationState(ledger)
	if err != nil {
		return api.OperationCardView{}, err
	}
	// The state moment (AC-13): the time of the LAST state-bearing
	// entry — when the life word landed; the dispatch_blocked entries
	// park visibly WITHOUT moving the state or its moment.
	stateAt := op.CreatedAt
	blockReason := ""
	for _, entry := range ledger {
		if entry.Kind == domain.LedgerEntryDispatchBlocked {
			blockReason = entry.Details
			continue
		}
		stateAt = entry.At
	}
	// The Owning System of the execution (AC-13): the 2-7 dictionary
	// word of the pinned definition — the system that owns the mutation;
	// an unresolvable pin keeps the honest empty word (the definition
	// pins stay on the card for the operator).
	owningSystem := ""
	if def, ok := catalogDefinition(s, op.DefinitionID, op.DefinitionVersion); ok {
		owningSystem = def.OwningSystem
	}
	attempts := []api.DispatchAttemptView{}
	receipts := []api.ActionReceiptView{}
	if s.dispatches != nil {
		attemptRows, err := s.dispatches.ReadDispatchAttempts(ctx, scope, op.OperationID)
		if err != nil {
			return api.OperationCardView{}, err
		}
		attempts = dispatchAttemptViews(attemptRows)
		receiptRows, err := s.dispatches.ReadActionReceipts(ctx, scope, op.OperationID)
		if err != nil {
			return api.OperationCardView{}, err
		}
		receipts = actionReceiptViews(receiptRows)
	}
	// The reconciliation view extension (3-7 step-03): the attempts of
	// the reconcile contour ride the SAME card read — the unwired contour
	// renders the honest empty list (the dispatch reads precedent); the
	// receipt versions grow with the terminal-locked appends of the verdict
	// transactions through the SAME receipts list of the dispatch view.
	reconciliationAttempts := []api.ReconciliationAttemptView{}
	if s.reconciliations != nil {
		reconcileRows, err := s.reconciliations.ReadReconciliationAttempts(ctx, scope, op.OperationID)
		if err != nil {
			return api.OperationCardView{}, err
		}
		reconciliationAttempts = reconciliationAttemptViews(reconcileRows)
	}
	card := api.OperationCardView{
		OperationID:            op.OperationID,
		DefinitionID:           op.DefinitionID,
		DefinitionVersion:      op.DefinitionVersion,
		PreviewID:              op.PreviewID,
		PreviewVersionNo:       op.PreviewVersionNo,
		ConfirmationID:         op.ConfirmationID,
		ConfirmationVersionNo:  op.ConfirmationVersionNo,
		CaseID:                 op.CaseID,
		WorkItemID:             op.WorkItemID,
		TargetKind:             op.Target.Kind,
		TargetRef:              op.Target.Ref,
		OwnerKind:              op.OwnerKind,
		OwnerRef:               op.OwnerRef,
		OwnerEpoch:             op.OwnerEpoch,
		ReleaseDecisionNo:      op.ReleaseDecisionNo,
		ProfileVersion:         op.ProfileVersion,
		Partition:              op.Partition,
		IdempotencyKey:         op.IdempotencyKey,
		BusinessIntentKey:      op.BusinessIntentKey,
		MaterialDigest:         op.MaterialDigest,
		State:                  state,
		StateText:              domain.OperationAuthorizedText,
		Queued:                 queued,
		StateAt:                stateAt,
		OwningSystem:           owningSystem,
		NextStepText:           domain.OperationNextStepText(state),
		BlockReasonText:        blockReason,
		ExternalIDAbsentText:   domain.OperationExternalIDAbsentText,
		DispatchAbsentText:     domain.OperationDispatchAbsentText,
		Ledger:                 []api.OperationLedgerEntryView{},
		Attempts:               attempts,
		Receipts:               receipts,
		ReconcileAbsentText:    domain.OperationReconcileAbsentText,
		ReconciliationAttempts: reconciliationAttempts,
		CreatedAt:              op.CreatedAt,
	}
	if queued && state == domain.OperationAuthorized {
		card.StateText = domain.OperationAuthorizedText + " " + domain.OperationQueuedText
	}
	switch state {
	case domain.OperationSubmitted:
		card.StateText = domain.OperationSubmittedText
	case domain.OperationAccepted:
		card.StateText = domain.OperationAcceptedText
	case domain.OperationCommitted:
		card.StateText = domain.OperationCommittedText
	case domain.OperationFailed:
		card.StateText = domain.OperationFailedText
	case domain.OperationUnknown:
		card.StateText = domain.OperationUnknownText
	}
	// The Story 3.8 recovery extension (the 3-7 card precedent): the
	// read-time eligibility trigger word over the recorded facts, the
	// existing case pin and the next-step word override of the ELIGIBLE
	// card (DeriveOperationNextStepText — the frozen inherited texts
	// never change; the word change is registered for journeys z36, rule
	// 161). The unwired contour renders the honest empty extension.
	recoveryExtension := s.recoveryOperationCardExtension(ctx, scope, op)
	card.RecoveryTrigger = recoveryExtension.Trigger
	if recoveryExtension.Trigger != "" {
		card.RecoveryTriggerText = domain.RecoveryTriggerText(recoveryExtension.Trigger)
	}
	card.RecoveryCaseID = recoveryExtension.RecoveryCaseID
	card.RecoveryCaseNo = recoveryExtension.RecoveryCaseNo
	card.NextStepText = domain.DeriveOperationNextStepText(state, recoveryExtension.Trigger)
	for _, entry := range ledger {
		card.Ledger = append(card.Ledger, api.OperationLedgerEntryView{
			EntryNo:   entry.EntryNo,
			Kind:      entry.Kind,
			Details:   entry.Details,
			CreatedAt: entry.At,
		})
	}
	return card, nil
}

// ReadAuthorizationView renders the case's Operation Review surface: the
// confirmation cards with the live state/consumption derivations (the
// events speak the visible conflict/blocked exceptions of the rejected
// attempts) and the operation cards with the ledger and the queue fact.
// executed NEVER exists here (AC-11 — the honest words speak the
// boundary). The now parameter is the transport clock — the SAME pair of
// hours the consent commands pin: the expired derivation reads the
// injected window coherently (the production transport passes
// time.Now()).
func (s *Service) ReadAuthorizationView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) (api.AuthorizationView, error) {
	if err := s.requireAuthorizationPorts(); err != nil {
		return api.AuthorizationView{}, err
	}
	scope, err := s.authorizeProposal(ctx, actor, tenantID)
	if err != nil {
		return api.AuthorizationView{}, err
	}
	confirmations, err := s.confirmations.ReadCaseConfirmations(ctx, scope, caseID)
	if err != nil {
		return api.AuthorizationView{}, err
	}
	eventRows, err := s.confirmations.ReadCaseConfirmationEvents(ctx, scope, caseID)
	if err != nil {
		return api.AuthorizationView{}, err
	}
	operations, err := s.operations.ReadCaseOperations(ctx, scope, caseID)
	if err != nil {
		return api.AuthorizationView{}, err
	}
	defsOf := func(definitionID, definitionVersion string) (domain.ActionDefinition, bool) {
		return actionCatalog.GetFrom(s.defs, definitionID, definitionVersion)
	}
	view := api.AuthorizationView{
		TenantID:      tenantID,
		CaseID:        caseID,
		Confirmations: []api.ConfirmationCardView{},
		Operations:    []api.OperationCardView{},
	}
	if len(confirmations) == 0 && len(operations) == 0 {
		view.EmptyText = "Подтверждений и авторизованных операций нет: подготовьте action preview и подтвердите точную версию, чтобы авторизовать действие."
	}
	for _, confirmation := range confirmations {
		card := api.ConfirmationCardView{
			ConfirmationID:    confirmation.ConfirmationID,
			VersionNo:         confirmation.VersionNo,
			DefinitionID:      confirmation.DefinitionID,
			DefinitionVersion: confirmation.DefinitionVersion,
			CaseID:            confirmation.CaseID,
			WorkItemID:        confirmation.WorkItemID,
			TargetKind:        confirmation.Target.Kind,
			TargetRef:         confirmation.Target.Ref,
			OwnerKind:         confirmation.OwnerKind,
			OwnerRef:          confirmation.OwnerRef,
			PreviewID:         confirmation.PreviewID,
			PreviewVersionNo:  confirmation.PreviewVersionNo,
			ValidUntil:        confirmation.ValidUntil,
			Events:            []api.ConfirmationEventView{},
			CreatedAt:         confirmation.CreatedAt,
		}
		versionEvents := []domain.ConfirmationEvent{}
		for _, row := range eventRows {
			if row.ConfirmationID == confirmation.ConfirmationID && row.VersionNo == confirmation.VersionNo {
				versionEvents = append(versionEvents, domain.ConfirmationEvent{
					Kind: row.Kind, Reason: row.Reason, Details: row.Details, At: row.CreatedAt, ActorID: row.ActorID,
				})
			}
			if row.ConfirmationID == confirmation.ConfirmationID {
				card.Events = append(card.Events, api.ConfirmationEventView{
					VersionNo: row.VersionNo,
					EventNo:   row.EventNo,
					Kind:      row.Kind,
					Reason:    row.Reason,
					Details:   row.Details,
					CreatedAt: row.CreatedAt,
				})
			}
		}
		state := domain.ConfirmationValid
		consumed := 0
		if def, ok := defsOf(confirmation.DefinitionID, confirmation.DefinitionVersion); ok {
			if derived, deriveErr := domain.DeriveConfirmationState(def.Conditions.ConsumptionRule, confirmation.ValidUntil, versionEvents, now); deriveErr == nil {
				state = derived
			}
			consumed = domain.DeriveConsumedCount(versionEvents)
			card.ConsumptionText = consumptionWords(def.Conditions.ConsumptionRule, consumed)
		} else {
			card.ConsumptionText = "Правило потребления неизвестно: определение не резолвится."
		}
		card.State = state
		card.StateText = domain.ConfirmationStateText(state)
		view.Confirmations = append(view.Confirmations, card)
	}
	for _, op := range operations {
		card, err := s.operationCard(ctx, scope, op)
		if err != nil {
			return api.AuthorizationView{}, err
		}
		view.Operations = append(view.Operations, card)
	}
	return view, nil
}
