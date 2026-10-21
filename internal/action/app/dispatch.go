package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.6 (step-02): the dispatch worker — DispatchQueuedOperations, the
// ordered execute-time fences and the verdict classification
// ---------------------------------------------------------------------------
//
// The drain is an EXPLICIT command (the DrainExternalCommitments precedent
// of 3-3): NO background daemon, NO ticker — the scheduler is a future
// infrastructure (the honest boundary). The worker reads ONLY under
// TenantContext (the RLS blindness is structural, AC-1).
//
// THE ORDERED FENCES — the order IS part of the contract (the unit
// counters pin it): job-schema alarm → fingerprint integrity alarm →
// lease/fencing classification → confirmation fence {superseded, revoked,
// expired} by name (NO exhausted — the consumption after a successful
// authorization is the expected state, the documented decision) → the
// case/work_item liveness → the freshness re-check (the 3-4 dictionary,
// seven reasons by name) → the readiness re-check → the LIVE policy gates
// (GuardPreparePolicy reused) → the LAST material check (FreshReads; the
// nil reader of the production root fails CLOSED — AG-4). EVERY fence is
// a NON-SENDING block: the adapter is NEVER called (the counter), the
// lease is NEVER taken, the operation parks authorized with the EXACT
// reason word in a dispatch_blocked ledger entry (never a false failed
// externally, AC-2). The integrity alarms (job schema, fingerprint) skip
// the job WITHOUT a ledger append — a broken machine never touches the
// world; the visible word rides the drain report for the human.
//
// THE SEND — write-ahead, fenced, adapter-free transactions: the lease
// and the ledger submitted entry land in ONE commit BEFORE the provider
// call (persist-before-emit, AD-6); the adapter is called EXACTLY ONCE,
// OUTSIDE every transaction, under the definition timeout; the verdict
// (the outcome row of the honest claim, the guarded ledger kind, the
// receipt v1 on the first deciding verdict and the outbox
// receipt.created record) lands in ONE fenced commit — a stale lease
// NEVER writes history (AC-1). The raw payload is never stored (AD-10):
// the lease and the outcome carry the keyed digest alone; the repeat
// verification rebuilds the payload and compares the keyed traces.

// dispatchScopeText is the honest scope refusal of the drain command: the
// transport pins the operator manage surface (step-03); the app gate
// speaks the operations scope word of the acting member.
const dispatchScopeText = "Команда drain требует полномочия operations: запуск отправок очереди — операторная команда, участники без операторного полномочия очередь не трогают."

// dispatchPortsMissingText is the honest refusal of the unwired contour
// (the preview/authorization precedents).
const dispatchPortsMissingText = "Контур диспетчерства не сконфигурирован: репозиторий dispatch не подключён, команда честно отклоняется."

// dispatchRefusedAG4Text is the honest production refusal of the nil
// mutation adapter (AG-4 — the DrainExternalCommitments precedent of
// 3-3): the queue stays untouched, zero sends.
const dispatchRefusedAG4Text = "Исполнение не подключено — реальных провайдеров нет: очередь не тронута, отправок ноль (AG-4)."

// dispatchJobSchemaAlarmText is the visible alarm word of an unrecognized
// job schema (a foreign event_type or a corrupt payload form): no gas, no
// lease, no send — the broken machine never touches the world.
const dispatchJobSchemaAlarmText = "Схема задания не распознана: пропуск с видимым словом — ни lease, ни отправки; запись остаётся человеку (alarm)."

// dispatchFingerprintAlarmText is the visible alarm word of the operation
// fingerprint integrity probe (the material digest of the immutable row
// re-derived from the pins): a mismatch is the corruption alarm.
const dispatchFingerprintAlarmText = "Целостность операции не подтверждена: digest строки расходится с пиннами — отправки нет, запись остаётся человеку (alarm)."

// dispatchReplayAlarmText is the visible alarm word of the decided-verify
// digest divergence (the at-least-once redelivery of a decided operation
// whose rebuilt digest no longer matches the recorded trace).
const dispatchReplayAlarmText = "Повтор задания с разошедшимся digest: коррупция не маскируется — отправки нет, запись остаётся человеку (alarm)."

// dispatchRacingSkipText is the honest racing skip (an active lease of
// another consideration owns the operation right now — the second drain
// gets NO second right to send).
const dispatchRacingSkipText = "Активная lease другого рассмотрения: честный skip — второго права отправки нет."

// dispatchCapabilityForeignText is the honest allowlist refusal: the
// adapter serves a foreign capability — no lease, no send (AC-4).
const dispatchCapabilityForeignText = "Адаптер обслуживает другую capability: честный отказ без отправки (allowlist-сверка до вызова)."

// dispatchFencedRefusalText is the honest fenced refusal of the outcome
// recording: the stale (attempt, token) pair never writes history.
const dispatchFencedRefusalText = "Устаревшая lease записи не получает: fencing-отказ — исход НЕ попал в историю."

// dispatchCaseClosedReason is the exact block reason word of the
// case/work_item liveness fence (the case or the work item is dead by the
// live read).
const dispatchCaseClosedReason = "case_closed"

// dispatchCaseClosedText speaks the liveness fence honestly.
const dispatchCaseClosedText = "Кейс/рабочий элемент завершён: queued intent гасится до отправки — требуется новое Preview и Confirmation контуром 3-5."

// dispatchFreshReadsReason is the exact block reason word of the nil
// owning-system reader (the fail-closed material gate, AG-4).
const dispatchFreshReadsReason = "fresh_reads_unavailable"

// dispatchMaterialChangedReason is the exact block reason word of the
// stale/conflicting/unknown material statuses (the changed material
// condition requires the new version pair).
const dispatchMaterialChangedReason = "material_condition_changed"

// dispatchMintLeaseToken mints the 64-hex holder token of one attempt
// (crypto/rand — the holder identity at the verdict recording; the
// fencing token is the monotonic attempt_no itself).
func dispatchMintLeaseToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// boundDetails bounds the safe details to the store CHECK limit (the
// domain truncation backstop of the confirmation precedent).
func boundDetails(text string) string {
	runes := []rune(text)
	if len(runes) > 500 {
		return string(runes[:500])
	}
	return text
}

// DispatchDrainCommand is the explicit drain command of the execution
// contour: the acting operator principal, the tenant and the moment. The
// batch is bounded by the domain drain limit (DispatchDrainLimit=100) —
// the rest stay queued for the next run, honestly.
type DispatchDrainCommand struct {
	Actor    identityDomain.Principal
	TenantID id.UUID
	Now      time.Time
}

// DispatchReportLine is ONE spoken line of the drain report: the
// operation, the attempt number of the line and the honest word (every
// boundary names the exact cause and the allowed next step — FR-28).
type DispatchReportLine struct {
	OperationID id.UUID
	AttemptNo   int
	Text        string
}

// DispatchDrainReport is the honest result of the drain command: how many
// jobs were considered, how many were dispatched, how many parked on a
// fence and how many skipped (races, replays, alarms) — with the spoken
// line of every job.
type DispatchDrainReport struct {
	Command    string
	Considered int
	Dispatched int
	Blocked    int
	Skipped    int
	Reports    []DispatchReportLine
}

// dispatchDrain is the per-job working set of one drain pass: the report
// accumulators as closures over the shared report, and the adapter call
// counter the unit fences pin (the adapter is NEVER called on a fence).
type dispatchDrain struct {
	report      *DispatchDrainReport
	adapterHits *int
}

// line appends ONE spoken line of the report.
func (d *dispatchDrain) line(operationID id.UUID, attemptNo int, text string) {
	d.report.Reports = append(d.report.Reports, DispatchReportLine{
		OperationID: operationID, AttemptNo: attemptNo, Text: text,
	})
}

// skip counts one honest skip with its spoken line.
func (d *dispatchDrain) skip(operationID id.UUID, attemptNo int, text string) {
	d.report.Skipped++
	d.line(operationID, attemptNo, text)
}

// block parks the operation on a fence: the dispatch_blocked ledger
// entry with the EXACT reason word — the adapter was never called, the
// lease was never taken, the operation stays authorized (AC-2). A raced
// append (an active lease appeared) is the honest skip (the
// GasExternalCommitmentQueue precedent). The explicit (ctx, scope, now)
// triple carries the per-job pass state — the drain is strictly
// sequential, one job at a time; a storage error of the append aborts
// the drain honestly (the committed jobs stay committed — each job's
// writes are independent transactions).
func (s *Service) block(ctx context.Context, scope tenancy.Context, now time.Time,
	d *dispatchDrain, job *port.PendingDispatchJob, attemptNo int, reason, details string) error {
	entryNo := int64(1)
	for _, entry := range job.Ledger {
		if entry.EntryNo >= entryNo {
			entryNo = entry.EntryNo + 1
		}
	}
	err := s.dispatches.AppendDispatchBlocked(ctx, port.AppendDispatchBlockedParams{
		Scope:       scope,
		OperationID: job.Operation.OperationID,
		Reason:      reason,
		Details:     boundDetails(details),
		LedgerEntry: domain.OperationLedgerEntry{
			EntryNo: entryNo,
			Kind:    domain.LedgerEntryDispatchBlocked,
			Details: boundDetails(details),
		},
		Now: now,
	})
	if err != nil {
		if errors.Is(err, port.ErrLeaseRaced) {
			d.skip(job.Operation.OperationID, attemptNo, dispatchRacingSkipText)
			return nil
		}
		return err
	}
	d.report.Blocked++
	d.line(job.Operation.OperationID, attemptNo, domain.DispatchBlockedText(reason))
	return nil
}

// dispatchOneJob processes ONE queued job through the ordered fences.
// The pass state (ctx/scope/now) rides the explicit parameters (the
// drain is strictly sequential, one job at a time); a transport/storage
// error of any append aborts the drain honestly (the committed jobs stay
// committed — each job's writes are independent transactions).
func (s *Service) dispatchOneJob(ctx context.Context, scope tenancy.Context, command DispatchDrainCommand,
	d *dispatchDrain, job *port.PendingDispatchJob) error {
	op := job.Operation
	// FENCE 1 — the job-schema alarm (never a gas, never a send): the
	// outbox record must decode into the SAFE canonical payload of the
	// queued job with the exact identity pins.
	var payload port.OperationQueuedPayload
	if err := json.Unmarshal(job.RawPayload, &payload); err != nil ||
		payload.EventType != domain.OutboxEventOperationQueued ||
		payload.TenantID != scope.TenantID() ||
		payload.OperationID != op.OperationID ||
		payload.CaseID != op.CaseID ||
		payload.WorkItemID != op.WorkItemID {
		d.skip(op.OperationID, job.LastAttemptNo, dispatchJobSchemaAlarmText)
		return nil
	}
	// The HMAC key of the tenant (the same per-tenant commitment key the
	// authorization derived the fingerprints with — the ensure pattern).
	hmacKey, err := s.previews.ReadPreviewKey(ctx, scope, command.Now)
	if err != nil {
		return err
	}
	// FENCE 2 — the fingerprint integrity alarm (never a gas, never a
	// send): the material digest of the immutable row re-derived from the
	// pins — a mismatch is the corruption alarm (an admin-SQL tampering).
	fingerprint := domain.DeriveIdempotencyFingerprint(hmacKey, scope.TenantID(), domain.OperationRequestPins{
		PreviewID:             op.PreviewID,
		PreviewVersionNo:      op.PreviewVersionNo,
		ConfirmationID:        op.ConfirmationID,
		ConfirmationVersionNo: op.ConfirmationVersionNo,
	})
	if fingerprint != op.MaterialDigest {
		d.skip(op.OperationID, job.LastAttemptNo, dispatchFingerprintAlarmText)
		return nil
	}
	// The definition resolve of the operation pins (the honest refusal of
	// an unknown pin — the catalog is compile-time).
	def, ok := catalogDefinition(s, op.DefinitionID, op.DefinitionVersion)
	if !ok {
		return domain.ErrNotFound
	}
	// FENCE 3 — the lease/fencing classification: the ONE dispatcher of
	// decisions (every job is considered, not every job is sent).
	switch job.Classification {
	case port.PendingClassRacing:
		d.skip(op.OperationID, job.LastAttemptNo, dispatchRacingSkipText)
		return nil
	case port.PendingClassDecided:
		return s.decidedVerify(ctx, scope, command, d, job, hmacKey, def)
	case port.PendingClassRetryable:
		// The recorded unknown verdict: the retry policy reads the
		// RECORDED guarantee word of the attempt (the audit fact, never a
		// re-read of the live declaration — the 3-3 «belt over braces»).
		policy, err := domain.DeriveDispatchRetryPolicy(job.StateWord,
			def.DeliveryContract.CapabilityClass, job.LastOutcomeGuarantee, job.LastAttemptNo)
		if err != nil {
			return err
		}
		if policy != domain.DispatchRetryRetry {
			d.skip(op.OperationID, job.LastAttemptNo, domain.DispatchRetryText(policy))
			return nil
		}
	case port.PendingClassRecoverable:
		// The crash resting state (submitted, the lease expired, no
		// outcome): the LIVE guarantee of the connector catalog declaration
		// decides the recovery — a retry is safe ONLY with
		// provider_idempotent AND the live guarantee; otherwise the honest
		// unknown verdict lands on the crashed attempt («возможен
		// предыдущий вызов — слепой повтор запрещён») and the operation
		// parks (AC-9/AC-11).
		guarantee := false
		if s.caps != nil {
			if facts, resolves := s.caps.Capability(def.RequiredCapability.ConnectorID,
				def.RequiredCapability.ConnectorVersion, def.RequiredCapability.CapabilityID); resolves {
				guarantee = facts.Idempotent
			}
		}
		policy, err := domain.DeriveDispatchRetryPolicy(job.StateWord,
			def.DeliveryContract.CapabilityClass, guarantee, job.LastAttemptNo)
		if err != nil {
			return err
		}
		if policy != domain.DispatchRetryRetry {
			return s.recordCrashUnknown(ctx, scope, command, d, job)
		}
	case port.PendingClassFresh, port.PendingClassBlocked:
		// The fences re-run (a parked cause may have healed): the ordered
		// chain below decides.
	}
	// FENCE 4 — the confirmation dispatch fence BY NAME (no exhausted:
	// the consumption after a successful authorization is the expected
	// state — the documented decision; the 3-5 ladder governs NEW
	// authorizations, not the dispatch of the created Operation).
	confirmation, err := s.confirmations.ReadConfirmationVersion(ctx, scope, op.ConfirmationID, op.ConfirmationVersionNo)
	if err != nil {
		return err
	}
	consentEvents, err := s.confirmations.ReadConfirmationVersionEvents(ctx, scope, op.ConfirmationID, op.ConfirmationVersionNo)
	if err != nil {
		return err
	}
	consentState, err := domain.DeriveConfirmationState(def.Conditions.ConsumptionRule, confirmation.ValidUntil, consentEvents, command.Now)
	if err != nil {
		return err
	}
	fence, err := domain.DeriveConfirmationDispatchFence(consentState)
	if err != nil {
		return err
	}
	if fence != "" {
		return s.block(ctx, scope, command.Now, d, job, job.LastAttemptNo, fence, domain.DispatchFenceText(fence))
	}
	// FENCE 5 — the case/work_item liveness (the live read of the case
	// api; a dead case or an unanchored work item kills the queued intent
	// before the send).
	caseView, err := s.caseMaterial.CaseView(ctx, command.Actor, command.TenantID, op.CaseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return s.block(ctx, scope, command.Now, d, job, job.LastAttemptNo, dispatchCaseClosedReason, dispatchCaseClosedText)
		}
		return err
	}
	material := caseViewMaterial{view: caseView}
	if !material.workItemAnchored(op.WorkItemID) {
		return s.block(ctx, scope, command.Now, d, job, job.LastAttemptNo, dispatchCaseClosedReason, dispatchCaseClosedText)
	}
	// The pinned preview version (the linked material of every
	// re-check — the live row of the exact pin).
	preview, err := s.operations.ReadPinnedPreviewVersion(ctx, scope, op.PreviewID, op.PreviewVersionNo)
	if err != nil {
		return err
	}
	// FENCE 6 — the freshness re-check (the 3-4 derivation reused — the
	// same dictionary, the seven reasons BY NAME).
	live, err := s.gatherPreviewLive(ctx, command.Actor, scope, def, material)
	if err != nil {
		return err
	}
	facts, err := s.caseMaterial.FactsView(ctx, command.Actor, command.TenantID, op.CaseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return s.block(ctx, scope, command.Now, d, job, job.LastAttemptNo, dispatchCaseClosedReason, dispatchCaseClosedText)
		}
		return err
	}
	fresh, err := s.previewFreshnessLive(ctx, command.Actor, scope, preview, material, live, facts)
	if err != nil {
		return err
	}
	if derivation := domain.DerivePreviewFreshness(preview, fresh); derivation.Stale {
		return s.block(ctx, scope, command.Now, d, job, job.LastAttemptNo, derivation.Reason, domain.FreshnessReasonText(derivation.Reason))
	}
	// FENCE 7 — the readiness re-check (the blocked material is never
	// sent).
	if readiness := domain.DerivePreviewReadiness(def, preview.Snapshot); readiness != domain.PreviewReady {
		return s.block(ctx, scope, command.Now, d, job, job.LastAttemptNo, readiness, domain.PreviewReadinessText(readiness))
	}
	// FENCE 8 — the LIVE policy gates (GuardPreparePolicy reused — the
	// pinned owner epoch of the OPERATION against the live sources).
	policy := domain.GuardPreparePolicy(def, op.OwnerEpoch, live.policy)
	if policy.Outcome == domain.PolicyBlock {
		return s.block(ctx, scope, command.Now, d, job, job.LastAttemptNo, policy.Reason, domain.PolicyReasonText(policy.Reason))
	}
	// FENCE 9 — the LAST material check (AC-3/AG-4): the required facts of
	// the definition read through the consumer-owned reader; the NIL
	// reader of the production root fails CLOSED; the allowed uncertainty
	// passes EXPLICITLY by the declaration.
	if len(def.Conditions.RequiredFacts) > 0 {
		if s.previewReader == nil {
			return s.block(ctx, scope, command.Now, d, job, job.LastAttemptNo, dispatchFreshReadsReason, domain.FreshReadsUnavailableText)
		}
		rows := s.readPreviewMaterial(ctx, scope, def, preview)
		byField := map[string]domain.MaterialSnapshotRow{}
		for _, row := range rows {
			byField[row.MaterialField] = row
		}
		for _, required := range def.Conditions.RequiredFacts {
			row, present := byField[required.MaterialField]
			if !present {
				return s.block(ctx, scope, command.Now, d, job, job.LastAttemptNo, dispatchMaterialChangedReason, domain.MaterialConditionChangedText)
			}
			switch row.Status {
			case domain.SnapshotFresh, domain.SnapshotAllowedUncertain:
				// The fresh field passes; the allowed uncertainty passes
				// EXPLICITLY by the definition's declaration.
			case domain.SnapshotStale, domain.SnapshotConflicting, domain.SnapshotUnknown:
				return s.block(ctx, scope, command.Now, d, job, job.LastAttemptNo, dispatchMaterialChangedReason, domain.MaterialConditionChangedText)
			default:
				// A foreign status word fails closed (a lying adapter can
				// never smuggle a pass in).
				return s.block(ctx, scope, command.Now, d, job, job.LastAttemptNo, dispatchMaterialChangedReason, domain.MaterialConditionChangedText)
			}
		}
	}
	// THE SEND — the deterministic rebuild, the derived keys, the
	// write-ahead lease, the ONE adapter call outside every transaction,
	// the fenced verdict.
	payloadRebuilt, err := domain.BuildAuthorizedActionPayload(op, preview, def)
	if err != nil {
		// The integrity refusal of the rebuild: the alarm skip (never a
		// gas, never a send — the mixed rows never dispatch).
		d.skip(op.OperationID, job.LastAttemptNo, dispatchFingerprintAlarmText)
		return nil
	}
	externalKey := domain.DeriveExternalIdempotencyKey(hmacKey, scope.TenantID(), op.OperationID)
	requestDigest := domain.DeriveDispatchRequestDigest(hmacKey, payloadRebuilt)
	// The destination allowlist BEFORE the lease and the call: the adapter
	// must serve the definition's required capability (AC-4).
	if s.dispatchAdapter.ServesCapability() != def.RequiredCapability.CapabilityID {
		d.skip(op.OperationID, job.LastAttemptNo, dispatchCapabilityForeignText)
		return nil
	}
	leaseToken, err := dispatchMintLeaseToken()
	if err != nil {
		return err
	}
	leaseExpires, err := domain.DeriveDispatchLeaseExpiresAt(command.Now, def.Conditions.TimeoutMS)
	if err != nil {
		return err
	}
	entryNo := int64(1)
	for _, entry := range job.Ledger {
		if entry.EntryNo >= entryNo {
			entryNo = entry.EntryNo + 1
		}
	}
	// The write-ahead lease transaction (persist-before-emit — the
	// signature carries ZERO adapters): the right and the moment of the
	// send are material BEFORE the network call.
	attemptNo, err := s.dispatches.AcquireDispatchLease(ctx, port.AcquireDispatchLeaseParams{
		Scope:                  scope,
		Operation:              op,
		DefinitionID:           op.DefinitionID,
		DefinitionVersion:      op.DefinitionVersion,
		LeaseToken:             leaseToken,
		ExternalIDempotencyKey: externalKey,
		RequestDigest:          requestDigest,
		LeaseExpiresAt:         leaseExpires,
		LedgerEntry: domain.OperationLedgerEntry{
			EntryNo: entryNo,
			Kind:    domain.LedgerEntrySubmitted,
			Details: "write-ahead: право и момент отправки закреплены до сетевого вызова",
		},
		Now: command.Now,
	})
	if err != nil {
		if errors.Is(err, port.ErrLeaseRaced) {
			d.skip(op.OperationID, job.LastAttemptNo, dispatchRacingSkipText)
			return nil
		}
		if errors.Is(err, port.ErrLeaseStale) {
			d.skip(op.OperationID, job.LastAttemptNo, domain.DispatchAttemptsBoundText)
			return nil
		}
		return err
	}
	// The ONE adapter call — OUTSIDE every store transaction, under the
	// definition timeout (the lease window outlives it by the grace).
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(def.Conditions.TimeoutMS)*time.Millisecond)
	result, callErr := s.dispatchAdapter.Dispatch(timeoutCtx, scope, port.MutationRequest{
		TenantID:               scope.TenantID(),
		OperationID:            op.OperationID,
		AttemptNo:              attemptNo,
		ExternalIDempotencyKey: externalKey,
		RequestDigest:          requestDigest,
		DefinitionID:           op.DefinitionID,
		DefinitionVersion:      op.DefinitionVersion,
		Payload:                payloadRebuilt,
	})
	cancel()
	*d.adapterHits++
	// The verdict classification (fail-closed everywhere): a call error or
	// a foreign word is the honest unknown WITHOUT a guarantee; a
	// committed claim without the structural evidence is REFUSED to
	// unknown; the guarantee word is recorded ONLY for the honest timeout
	// claim (the LIVE declaration read by the adapter at the call moment).
	claim := ""
	honest := domain.OperationUnknown
	guarantee := false
	evidence := domain.MutationEvidence{}
	correlation := ""
	details := ""
	if callErr == nil {
		correlation = result.CorrelationID
		details = result.Details
		evidence = domain.MutationEvidence{
			ExternalObjectID:      result.ExternalObjectID,
			ExternalObjectVersion: result.ExternalObjectVersion,
			ExternalObjectTime:    result.ExternalObjectTime,
		}
		if domain.GuardMutationOutcome(result.Outcome) == nil {
			guarded, guardErr := domain.GuardCommittedEvidence(result.Outcome, evidence)
			if guardErr == nil {
				if guarded == domain.MutationOutcomeCommitted {
					// The honest committed claim with the complete triple.
					claim = domain.MutationOutcomeCommitted
					honest = domain.OperationCommitted
				} else {
					// The refused terminal claim (no structural evidence):
					// NO outcome row — the ledger and the receipt speak the
					// guarded unknown (the terminal success is never taken
					// on word, AC-6/AC-7).
					honest = domain.OperationUnknown
					details = domain.DispatchEvidenceIncompleteText
				}
				if result.Outcome == domain.MutationOutcomeAccepted {
					claim = domain.MutationOutcomeAccepted
					honest = domain.OperationAccepted
				}
				if result.Outcome == domain.MutationOutcomeFailed {
					claim = domain.MutationOutcomeFailed
					honest = domain.OperationFailed
				}
				if result.Outcome == domain.MutationOutcomeTimeout {
					claim = domain.MutationOutcomeTimeout
					honest = domain.OperationUnknown
					guarantee = result.GuaranteeLive
				}
			}
		}
	}
	stateWord, err := domain.DeriveMutationStateWord(honest)
	if err != nil {
		return err
	}
	receiptWord, err := domain.DeriveReceiptOutcomeWord(honest)
	if err != nil {
		return err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return err
	}
	var outboxPayload port.ReceiptCreatedPayload
	if receiptWord != "" {
		outboxPayload = port.ReceiptCreatedPayload{
			EventType:        domain.OutboxEventReceiptCreated,
			TenantID:         scope.TenantID(),
			OperationID:      op.OperationID,
			CaseID:           op.CaseID,
			WorkItemID:       op.WorkItemID,
			DefinitionID:     op.DefinitionID,
			ReceiptVersionNo: 1,
			Outcome:          receiptWord,
			ExternalObjectID: evidence.ExternalObjectID,
			CreatedAt:        command.Now.UTC().Truncate(time.Microsecond),
		}
	}
	// The fenced verdict transaction (the signature carries ZERO
	// adapters): the outcome row of the honest claim, the guarded ledger
	// kind, the receipt v1 on the first deciding verdict and the outbox
	// receipt.created record — ONE commit; a stale lease NEVER writes
	// history.
	err = s.dispatches.RecordDispatchOutcome(ctx, port.RecordDispatchOutcomeParams{
		Scope:             scope,
		Operation:         op,
		AttemptNo:         attemptNo,
		LeaseToken:        leaseToken,
		OutcomeWord:       claim,
		StateWord:         stateWord,
		ReceiptWord:       receiptWord,
		CorrelationID:     correlation,
		Evidence:          evidence,
		GuaranteeRecorded: guarantee,
		Details:           boundDetails(details),
		LedgerEntry: domain.OperationLedgerEntry{
			EntryNo: entryNo + 1,
			Kind:    stateWord,
			Details: boundDetails(details),
		},
		ReceiptVersionNo: 1,
		ReceiptReferences: port.ReceiptReferences{
			CaseID:                op.CaseID,
			WorkItemID:            op.WorkItemID,
			PreviewID:             op.PreviewID,
			PreviewVersionNo:      op.PreviewVersionNo,
			ConfirmationID:        op.ConfirmationID,
			ConfirmationVersionNo: op.ConfirmationVersionNo,
			DefinitionID:          op.DefinitionID,
			DefinitionVersion:     op.DefinitionVersion,
			ExternalObjectID:      evidence.ExternalObjectID,
			ExternalObjectVersion: evidence.ExternalObjectVersion,
		},
		ReceiptDetails: boundDetails(verdictDetails(honest, guarantee)),
		OutboxEventID:  outboxEventID,
		OutboxPayload:  outboxPayload,
		Now:            command.Now,
	})
	if err != nil {
		if errors.Is(err, port.ErrLeaseStale) || errors.Is(err, port.ErrOutcomeDuplicate) {
			d.skip(op.OperationID, attemptNo, dispatchFencedRefusalText)
			return nil
		}
		return err
	}
	d.report.Dispatched++
	d.line(op.OperationID, attemptNo, verdictDetails(honest, guarantee))
	return nil
}

// verdictDetails speaks the honest verdict word of one attempt (the
// honest texts of the domain — every outcome names its truth and the
// allowed next step).
func verdictDetails(honest string, guarantee bool) string {
	switch honest {
	case domain.OperationAccepted:
		return domain.OperationAcceptedText
	case domain.OperationCommitted:
		return domain.OperationCommittedText
	case domain.OperationFailed:
		return domain.OperationFailedText
	}
	if guarantee {
		return domain.OperationUnknownText + " Гарантия провайдера записана: повтор разрешён политикой в пределах 3."
	}
	return domain.OperationUnknownText
}

// catalogDefinition resolves one definition pin against the injected
// dictionary (the compile-time catalog read of the module).
func catalogDefinition(s *Service, definitionID, version string) (domain.ActionDefinition, bool) {
	for _, def := range s.defs {
		if def.ID == definitionID && def.Version == version {
			return def, true
		}
	}
	return domain.ActionDefinition{}, false
}

// decidedVerify is the at-least-once redelivery handling of a DECIDED
// operation (AC-10): the payload is rebuilt deterministically and the
// keyed digest is compared with the RECORDED trace of the first attempt —
// a match is the honest skip («повтор задания: операция уже решена», no
// new send, no new intent), a divergence is the integrity alarm (the
// corruption is never masked).
func (s *Service) decidedVerify(ctx context.Context, scope tenancy.Context, command DispatchDrainCommand,
	d *dispatchDrain, job *port.PendingDispatchJob, hmacKey []byte, def domain.ActionDefinition) error {
	op := job.Operation
	preview, err := s.operations.ReadPinnedPreviewVersion(ctx, scope, op.PreviewID, op.PreviewVersionNo)
	if err != nil {
		return err
	}
	payload, err := domain.BuildAuthorizedActionPayload(op, preview, def)
	if err != nil {
		d.skip(op.OperationID, job.LastAttemptNo, dispatchReplayAlarmText)
		return nil
	}
	digest := domain.DeriveDispatchRequestDigest(hmacKey, payload)
	if job.RequestDigest == "" || digest != job.RequestDigest {
		d.skip(op.OperationID, job.LastAttemptNo, dispatchReplayAlarmText)
		return nil
	}
	d.skip(op.OperationID, job.LastAttemptNo, domain.DispatchRedeliveryDecidedText)
	return nil
}

// recordCrashUnknown records the honest unknown verdict ON THE CRASHED
// attempt (the recoverable park without the guarantee — AC-9/AC-11): the
// fenced verdict transaction writes the guarded ledger kind, the receipt
// v1 {unknown} and the outbox receipt.created record; NO outcome row
// (there is no honest claim), NO new attempt («возможен предыдущий
// вызов — слепой повтор запрещён»), the Operation stays the single one.
func (s *Service) recordCrashUnknown(ctx context.Context, scope tenancy.Context, command DispatchDrainCommand,
	d *dispatchDrain, job *port.PendingDispatchJob) error {
	op := job.Operation
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return err
	}
	entryNo := int64(1)
	for _, entry := range job.Ledger {
		if entry.EntryNo >= entryNo {
			entryNo = entry.EntryNo + 1
		}
	}
	err = s.dispatches.RecordDispatchOutcome(ctx, port.RecordDispatchOutcomeParams{
		Scope:       scope,
		Operation:   op,
		AttemptNo:   job.LastAttemptNo,
		LeaseToken:  job.LastLeaseToken,
		OutcomeWord: "",
		StateWord:   domain.OperationUnknown,
		ReceiptWord: domain.ReceiptOutcomeUnknown,
		LedgerEntry: domain.OperationLedgerEntry{
			EntryNo: entryNo,
			Kind:    domain.OperationUnknown,
			Details: domain.DispatchBlindRepeatForbiddenText,
		},
		ReceiptVersionNo: 1,
		ReceiptReferences: port.ReceiptReferences{
			CaseID:                op.CaseID,
			WorkItemID:            op.WorkItemID,
			PreviewID:             op.PreviewID,
			PreviewVersionNo:      op.PreviewVersionNo,
			ConfirmationID:        op.ConfirmationID,
			ConfirmationVersionNo: op.ConfirmationVersionNo,
			DefinitionID:          op.DefinitionID,
			DefinitionVersion:     op.DefinitionVersion,
		},
		ReceiptDetails: domain.DispatchBlindRepeatForbiddenText,
		OutboxEventID:  outboxEventID,
		OutboxPayload: port.ReceiptCreatedPayload{
			EventType:        domain.OutboxEventReceiptCreated,
			TenantID:         scope.TenantID(),
			OperationID:      op.OperationID,
			CaseID:           op.CaseID,
			WorkItemID:       op.WorkItemID,
			DefinitionID:     op.DefinitionID,
			ReceiptVersionNo: 1,
			Outcome:          domain.ReceiptOutcomeUnknown,
			CreatedAt:        command.Now.UTC().Truncate(time.Microsecond),
		},
		Now: command.Now,
	})
	if err != nil {
		if errors.Is(err, port.ErrLeaseStale) || errors.Is(err, port.ErrOutcomeDuplicate) {
			d.skip(op.OperationID, job.LastAttemptNo, dispatchFencedRefusalText)
			return nil
		}
		return err
	}
	d.report.Blocked++
	d.line(op.OperationID, job.LastAttemptNo, domain.DispatchBlindRepeatForbiddenText)
	return nil
}

// requireDispatchPorts refuses honestly when the execution contour is not
// wired: the dispatch repository and the authorization repositories are
// mandatory (the fences read the consent and the operations live).
func (s *Service) requireDispatchPorts() error {
	if s.dispatches == nil || s.confirmations == nil || s.operations == nil {
		return fmt.Errorf("%s", dispatchPortsMissingText)
	}
	if err := s.requirePreviewPorts(); err != nil {
		return err
	}
	return nil
}

// dispatchScope is the gate of the drain command: the membership FIRST
// (a non-member gets the 404 — no disclosure that the tenant dispatches
// actions at all), then the operations scope word of the acting member
// (the transport pins the operator manage surface in step-03).
func (s *Service) dispatchScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return tenancy.Context{}, err
	}
	if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Operations); err != nil {
		if errors.Is(err, identityDomain.ErrForbidden) {
			return tenancy.Context{}, fmt.Errorf("%w: %s", domain.ErrForbidden, dispatchScopeText)
		}
		if errors.Is(err, identityDomain.ErrNotFound) {
			return tenancy.Context{}, domain.ErrNotFound
		}
		return tenancy.Context{}, err
	}
	return scope, nil
}

// DispatchQueuedOperations is the EXPLICIT drain command of the execution
// contour (NO daemon, NO ticker — the scheduler is a future
// infrastructure, the honest boundary): the nil mutation adapter is the
// honest production refusal FIRST (AG-4 — the queue stays untouched,
// zero sends; the DrainExternalCommitments precedent), then the bounded
// batch of the pending jobs processes ONE BY ONE through the ordered
// fences — every job is considered, not every job is sent. A storage
// error of a job aborts the drain honestly (the committed jobs stay
// committed — each job's writes are independent transactions).
func (s *Service) DispatchQueuedOperations(ctx context.Context, command DispatchDrainCommand) (DispatchDrainReport, error) {
	if err := s.requireDispatchPorts(); err != nil {
		return DispatchDrainReport{}, err
	}
	// The AG-4 honest boundary FIRST: no adapter, no sends — the records
	// stay queued, the state stays honest.
	if s.dispatchAdapter == nil {
		return DispatchDrainReport{}, fmt.Errorf("%w: %s", port.ErrDispatchRefusedAG4, dispatchRefusedAG4Text)
	}
	scope, err := s.dispatchScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return DispatchDrainReport{}, err
	}
	jobs, err := s.dispatches.PendingDispatchJobs(ctx, scope, command.Now, domain.DispatchDrainLimit)
	if err != nil {
		return DispatchDrainReport{}, err
	}
	adapterHits := 0
	d := &dispatchDrain{
		report: &DispatchDrainReport{
			Command:    "dispatch_queued_operations",
			Considered: len(jobs),
			Reports:    []DispatchReportLine{},
		},
		adapterHits: &adapterHits,
	}
	for i := range jobs {
		if err := s.dispatchOneJob(ctx, scope, command, d, &jobs[i]); err != nil {
			return *d.report, err
		}
	}
	return *d.report, nil
}
