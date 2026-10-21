package app

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/artemiosu/staffy/internal/action/api"
	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.7 (step-02): the reconcile worker — ReconcileUnknownOperations,
// the LIGHT read-only fence ladder and the terminal-locked verdict
// ---------------------------------------------------------------------------
//
// The reconcile is an EXPLICIT command (NO daemon, NO ticker — the honest
// boundary of the dispatch precedent): every run is a bounded pass over
// the pending targets of the tenant (the operations with an unknown
// receipt — the at-least-once anchor). The reconciliation READS, it never
// sends and never consumes a consent (AD-6 «reconcile before retry»):
// the FULL confirmation/freshness/readiness/policy ladder of the dispatch
// contour is deliberately NOT repeated here — the light ladder below
// checks the classification first (racing/exhausted/decided/waiting speak
// their honest words WITHOUT a lease and WITHOUT a query), then the
// fingerprint integrity, the DECLARED reconciliation semantics (BOTH
// halves — the LIVE connector word and the definition query; a bare 404
// without a declared semantics is no_answer, never a proof), the scoped
// control (paused blocks — the conservative word of this slice) and the
// adapter allowlist. EVERY fence is a NON-QUERYING block: the adapter is
// NEVER called, the lease is NEVER taken, the honest word rides the
// report alone (the fences are report-only — no ledger word is written
// for a transient block).
//
// THE QUERY — write-ahead, fenced, adapter-free transactions: the lease
// and the ledger reconciling entry (NOT state-bearing — the operation
// STAYS unknown) land in ONE commit BEFORE the owning-system call
// (persist-before-emit); the adapter is called EXACTLY ONCE, OUTSIDE
// every transaction, under the definition timeout plus a grace stock;
// the NORMALIZED verdict (the domain matrix refuses the unjustified
// claims — a committed claim without the evidence triple and a no_effect
// claim without the declared semantics both read no_answer) lands in ONE
// fenced commit with the terminal writes the verdict earned: the ledger
// word (committed/no_effect only), the receipt v2+ and the outbox
// receipt.created record — APPENDED only over the last unknown version
// (the terminal lock; the same-terminal confirmation is the idempotent
// skip, a conflicting terminal is the integrity alarm that never
// auto-replaces, epic AC-5). The raw material never travels: the query
// carries the SAME external idempotency key as the dispatch, the last
// dispatch correlation and the keyed request digest (AD-10).

// reconcileScopeText is the honest scope refusal of the reconcile
// command: the transport pins the operator manage surface (step-03); the
// app gate speaks the operations scope word of the acting member (the
// dispatchScope precedent — the reconcile is the same operator surface).
const reconcileScopeText = "Команда reconcile требует полномочия operations: сверка unknown-операций — операторная команда, участники без операторного полномочия очередь сверки не трогают."

// reconcilePortsMissingText is the honest refusal of the unwired contour
// (the preview/dispatch precedents).
const reconcilePortsMissingText = "Контур сверки не сконфигурирован: репозиторий reconciliation не подключён, команда честно отклоняется."

// reconcileCapabilityForeignText is the honest allowlist refusal: the
// read-only adapter serves a foreign capability — no lease, no query.
const reconcileCapabilityForeignText = "Адаптер сверки обслуживает другую capability — запрос не выполняется: честный отказ без lease (allowlist-сверка до вызова)."

// reconcileFencedRefusalText is the honest fenced refusal of the verdict
// recording: the stale (attempt, token) pair never writes history.
const reconcileFencedRefusalText = "Устаревшая пара сверки — вердикт не записан: fencing-отказ, история остаётся владельцу права."

// reconcilePausedWord renders the exact paused word of the scoped control
// with its reason (the honest line carries BOTH — the mode and the
// winning point's reason).
func reconcilePausedWord(mode, reason string) string {
	if strings.TrimSpace(reason) == "" {
		return mode
	}
	return mode + ": " + reason
}

// ReconcileCommand is the explicit reconcile command of the
// reconciliation contour: the acting operator principal, the tenant and
// the moment. The batch is bounded by the domain drain limit
// (ReconcileDrainLimit=100) — the rest stay honestly for the next run.
type ReconcileCommand struct {
	Actor    identityDomain.Principal
	TenantID id.UUID
	Now      time.Time
}

// ReconcileReportLine is ONE spoken line of the reconcile report: the
// operation, the attempt number of the line and the honest word (every
// boundary names the exact cause and the allowed next step — FR-30).
type ReconcileReportLine struct {
	OperationID id.UUID
	AttemptNo   int
	Text        string
}

// ReconcileReport is the honest result of the reconcile command: how many
// targets were considered, how many were queried (the owning-system
// calls), how many landed a recorded verdict (the attempt's guarded truth
// — whatever the terminal writes decided), how many skipped honestly
// (fences, races, alarms) — with the spoken line of every target.
type ReconcileReport struct {
	Command    string
	Considered int
	Queried    int
	Decided    int
	Skipped    int
	Reports    []ReconcileReportLine
}

// reconcileRun is the per-target working set of one reconcile pass: the
// report accumulators as closures over the shared report (the
// dispatchDrain precedent).
type reconcileRun struct {
	report *ReconcileReport
}

// line appends ONE spoken line of the report.
func (r *reconcileRun) line(operationID id.UUID, attemptNo int, text string) {
	r.report.Reports = append(r.report.Reports, ReconcileReportLine{
		OperationID: operationID, AttemptNo: attemptNo, Text: text,
	})
}

// skip counts one honest skip with its spoken line (a fence, a race, an
// alarm — the target stays what it was, the word rides the report).
func (r *reconcileRun) skip(operationID id.UUID, attemptNo int, text string) {
	r.report.Skipped++
	r.line(operationID, attemptNo, text)
}

// decided counts one recorded verdict with its spoken line: the outcome
// row of the attempt landed (the guarded truth), whatever the terminal
// writes of the verdict decided (the receipt-append skip speaks its own
// confirmation word).
func (r *reconcileRun) decided(operationID id.UUID, attemptNo int, text string) {
	r.report.Decided++
	r.line(operationID, attemptNo, text)
}

// requireReconcilePorts refuses honestly when the reconciliation contour
// is not wired: the reconciliation repository is mandatory (the pending
// read, the write-ahead lease and the fenced verdict transactions); the
// fingerprint fence reads the preview key — the preview contour is
// mandatory too (the requireDispatchPorts precedent).
func (s *Service) requireReconcilePorts() error {
	if s.reconciliations == nil {
		return fmt.Errorf("%s", reconcilePortsMissingText)
	}
	if err := s.requirePreviewPorts(); err != nil {
		return err
	}
	return nil
}

// reconcileScope is the gate of the reconcile command: the membership
// FIRST (a non-member gets the 404 — no disclosure that the tenant
// reconciles actions at all), then the operations scope word of the
// acting member (the dispatchScope precedent — the copied ladder of the
// same operator surface with the reconcile word).
func (s *Service) reconcileScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return tenancy.Context{}, err
	}
	if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Operations); err != nil {
		if errors.Is(err, identityDomain.ErrForbidden) {
			return tenancy.Context{}, fmt.Errorf("%w: %s", domain.ErrForbidden, reconcileScopeText)
		}
		if errors.Is(err, identityDomain.ErrNotFound) {
			return tenancy.Context{}, domain.ErrNotFound
		}
		return tenancy.Context{}, err
	}
	return scope, nil
}

// ReconcileUnknownOperations is the EXPLICIT reconcile command of the
// reconciliation contour (NO daemon, NO ticker — the honest boundary of
// the dispatch precedent): the nil read-only adapter is the honest
// production refusal FIRST (AG-4 — no real owning systems exist, the
// pending targets honestly stay), then the bounded batch of the pending
// targets processes ONE BY ONE through the light read-only fence ladder —
// every target is considered, not every target is queried. A storage
// error of a target aborts the run honestly (the recorded verdicts stay
// recorded — each target's writes are independent transactions).
func (s *Service) ReconcileUnknownOperations(ctx context.Context, command ReconcileCommand) (ReconcileReport, error) {
	if err := s.requireReconcilePorts(); err != nil {
		return ReconcileReport{}, err
	}
	// The AG-4 honest boundary FIRST: no adapter, no queries — the pending
	// targets stay, the state stays honest.
	if s.reconcileAdapter == nil {
		return ReconcileReport{}, fmt.Errorf("%w: %s", port.ErrReconcileRefusedAG4, domain.ReconciliationNotWiredText)
	}
	scope, err := s.reconcileScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return ReconcileReport{}, err
	}
	targets, err := s.reconciliations.PendingReconciliationTargets(ctx, scope, command.Now, domain.ReconcileDrainLimit)
	if err != nil {
		return ReconcileReport{}, err
	}
	r := &reconcileRun{
		report: &ReconcileReport{
			Command:    "reconcile_unknown_operations",
			Considered: len(targets),
			Reports:    []ReconcileReportLine{},
		},
	}
	for i := range targets {
		if err := s.reconcileOneTarget(ctx, scope, command, r, &targets[i]); err != nil {
			return *r.report, err
		}
	}
	return *r.report, nil
}

// reconcileOneTarget processes ONE pending target through the LIGHT
// read-only fence ladder. The classification words answer FIRST (no
// lease, no query — the honest structural words); the fresh path checks
// the fingerprint integrity, the DECLARED reconciliation semantics, the
// scoped control and the adapter allowlist BEFORE the write-ahead lease;
// the ONE query runs OUTSIDE every transaction under the definition
// timeout plus a grace stock; the NORMALIZED verdict lands in ONE fenced
// commit with the terminal writes the verdict earned.
func (s *Service) reconcileOneTarget(ctx context.Context, scope tenancy.Context, command ReconcileCommand,
	r *reconcileRun, target *port.PendingReconciliationTarget) error {
	op := target.Operation
	// FENCE 1 — the structural classification: the ONE reconciler of
	// decisions (every target is considered, not every target is queried).
	switch target.Classification {
	case port.ReconcileClassRacing:
		r.skip(op.OperationID, target.LastReconcileAttemptNo, domain.ReconciliationRacingText)
		return nil
	case port.ReconcileClassExhausted:
		r.skip(op.OperationID, target.LastReconcileAttemptNo, domain.ReconciliationExhaustedText)
		return nil
	case port.ReconcileClassDecided:
		r.skip(op.OperationID, target.LastReconcileAttemptNo,
			fmt.Sprintf(domain.ReconciliationDecidedTextTemplate, target.LastReceiptOutcome))
		return nil
	case port.ReconcileClassWaiting:
		r.skip(op.OperationID, target.LastReconcileAttemptNo,
			fmt.Sprintf(domain.ReconciliationBackoffTextTemplate, target.NextEligibleAt.Format(time.RFC3339)))
		return nil
	}
	// The HMAC key of the tenant (the same per-tenant commitment key the
	// authorization derived the fingerprints with — the ensure pattern).
	hmacKey, err := s.previews.ReadPreviewKey(ctx, scope, command.Now)
	if err != nil {
		return err
	}
	// FENCE 2 — the fingerprint integrity alarm (never a lease, never a
	// query): the material digest of the immutable row re-derived from the
	// pins — a mismatch is the corruption alarm (an admin-SQL tampering).
	fingerprint := domain.DeriveIdempotencyFingerprint(hmacKey, scope.TenantID(), domain.OperationRequestPins{
		PreviewID:             op.PreviewID,
		PreviewVersionNo:      op.PreviewVersionNo,
		ConfirmationID:        op.ConfirmationID,
		ConfirmationVersionNo: op.ConfirmationVersionNo,
	})
	if fingerprint != op.MaterialDigest {
		r.skip(op.OperationID, target.LastReconcileAttemptNo, dispatchFingerprintAlarmText)
		return nil
	}
	// The definition resolve of the operation pins (the honest refusal of
	// an unknown pin — the catalog is compile-time).
	def, ok := catalogDefinition(s, op.DefinitionID, op.DefinitionVersion)
	if !ok {
		return domain.ErrNotFound
	}
	// FENCE 3 — the DECLARED reconciliation semantics (BOTH halves — the
	// LIVE word of the connector declaration read at the query moment and
	// the definition's ReconciliationQuery word): a bare 404 without a
	// declared semantics is no_answer, never a proof — the undeclared
	// target stays unknown for the human (no lease, no query).
	liveWord := ""
	if s.caps != nil {
		if facts, resolves := s.caps.Capability(def.RequiredCapability.ConnectorID,
			def.RequiredCapability.ConnectorVersion, def.RequiredCapability.CapabilityID); resolves {
			liveWord = facts.Reconciliation
		}
	}
	declared := domain.DeclaredReconciliationSemantics{
		LiveWord: liveWord,
		Query:    def.DeliveryContract.ReconciliationQuery,
	}
	if !declared.Declared() {
		r.skip(op.OperationID, target.LastReconcileAttemptNo, domain.ReconciliationNotDeclaredText)
		return nil
	}
	// FENCE 4 — the scoped control by the LIVE reading of the 3-2 surface:
	// the paused word blocks the query (the conservative word of this
	// slice — the recovery-scope permissions of a paused connector are the
	// 3-9 slice); the exact mode and reason ride the honest line.
	control, err := s.previewControl.EffectiveFor(ctx, scope.TenantID(), port.ControlRoute{
		ConnectorID:   def.RequiredCapability.ConnectorID,
		AutonomyClass: def.Conditions.AutonomyClass,
	})
	if err != nil {
		return err
	}
	if control.Mode == domain.ScopedControlPaused {
		r.skip(op.OperationID, target.LastReconcileAttemptNo,
			fmt.Sprintf(domain.ReconciliationPausedTextTemplate, reconcilePausedWord(control.Mode, control.Reason)))
		return nil
	}
	// FENCE 5 — the destination allowlist BEFORE the lease and the query:
	// the read-only adapter must serve the definition's required
	// capability (a foreign capability is the honest refusal without a
	// call).
	if s.reconcileAdapter.ServesCapability() != def.RequiredCapability.CapabilityID {
		r.skip(op.OperationID, target.LastReconcileAttemptNo, reconcileCapabilityForeignText)
		return nil
	}
	// THE QUERY — the write-ahead lease, the ONE adapter call outside
	// every transaction, the fenced verdict.
	leaseToken, err := dispatchMintLeaseToken()
	if err != nil {
		return err
	}
	leaseExpires, err := domain.DeriveReconciliationLeaseExpiresAt(command.Now, def.Conditions.TimeoutMS)
	if err != nil {
		return err
	}
	backoffJitter := rand.Intn(domain.ReconcileBackoffJitterMS + 1)
	nextEligible, err := domain.DeriveReconciliationBackoffUntil(command.Now, backoffJitter)
	if err != nil {
		return err
	}
	entryNo := int64(1)
	for _, entry := range target.Ledger {
		if entry.EntryNo >= entryNo {
			entryNo = entry.EntryNo + 1
		}
	}
	// The write-ahead lease transaction (persist-before-emit — the
	// signature carries ZERO adapters): the right and the moment of the
	// query are material BEFORE the network call; the ledger reconciling
	// entry is NOT state-bearing — the operation STAYS unknown.
	attemptNo, err := s.reconciliations.AcquireReconciliationLease(ctx, port.AcquireReconciliationLeaseParams{
		Scope:                  scope,
		Operation:              op,
		DefinitionID:           op.DefinitionID,
		DefinitionVersion:      op.DefinitionVersion,
		ReconciliationWord:     declared.LiveWord,
		LeaseToken:             leaseToken,
		ExternalIDempotencyKey: target.ExternalIDempotencyKey,
		RequestDigest:          target.RequestDigest,
		CorrelationID:          target.LastDispatchCorrelation,
		LeaseExpiresAt:         leaseExpires,
		NextEligibleAt:         nextEligible,
		LedgerEntry: domain.OperationLedgerEntry{
			EntryNo: entryNo,
			Kind:    domain.LedgerEntryReconciling,
			Details: fmt.Sprintf("reconciliation attempt %d; query %s", target.ReconcileAttemptCount+1, declared.Query),
		},
		Now: command.Now,
	})
	if err != nil {
		if errors.Is(err, port.ErrReconciliationLeaseRaced) {
			r.skip(op.OperationID, target.LastReconcileAttemptNo, domain.ReconciliationRacingText)
			return nil
		}
		if errors.Is(err, port.ErrReconciliationLeaseStale) {
			r.skip(op.OperationID, target.LastReconcileAttemptNo, domain.ReconciliationExhaustedText)
			return nil
		}
		return err
	}
	// The ONE adapter call — OUTSIDE every store transaction, under the
	// definition timeout plus a grace stock (the lease window outlives it
	// by the domain grace).
	queryCtx, cancel := context.WithTimeout(ctx,
		time.Duration(def.Conditions.TimeoutMS)*time.Millisecond+5*time.Second)
	result, callErr := s.reconcileAdapter.QueryOutcome(queryCtx, scope, port.ReconciliationQueryRequest{
		TenantID:               scope.TenantID(),
		OperationID:            op.OperationID,
		AttemptNo:              attemptNo,
		ExternalIDempotencyKey: target.ExternalIDempotencyKey,
		CorrelationID:          target.LastDispatchCorrelation,
		RequestDigest:          target.RequestDigest,
		DefinitionID:           op.DefinitionID,
		DefinitionVersion:      op.DefinitionVersion,
		ReconciliationQuery:    declared.Query,
	})
	cancel()
	r.report.Queried++
	// The verdict normalization (fail-closed everywhere): a call error, a
	// foreign word, a committed claim without the structural evidence
	// triple and a no_effect claim without the declared semantics all read
	// no_answer — never a masked success, never a masked absence.
	claimed := ""
	evidence := domain.MutationEvidence{}
	correlation := target.LastDispatchCorrelation
	details := ""
	if callErr == nil {
		claimed = result.ClaimedVerdict
		evidence = domain.MutationEvidence{
			ExternalObjectID:      result.ExternalObjectID,
			ExternalObjectVersion: result.ExternalObjectVersion,
			ExternalObjectTime:    result.ExternalObjectTime,
		}
		if result.CorrelationID != "" {
			correlation = result.CorrelationID
		}
		details = result.Details
	}
	verdict := domain.DeriveReconciliationVerdict(claimed, evidence, declared)
	effects, err := domain.DeriveReconciliationVerdictEffects(verdict)
	if err != nil {
		return err
	}
	// The terminal lock BEFORE the write (the app pre-check with the
	// target facts — the store re-runs the guard under the lock, the belt
	// over braces): an append over the last unknown passes; the
	// same-terminal confirmation is the idempotent skip (the outcome row
	// still records — the receipt does NOT); a conflicting terminal is the
	// integrity alarm — the verdict is NOT recorded at all, the human
	// review owns the resolution (never an auto-replacement).
	receiptWord := effects.ReceiptWord
	receiptSkip := false
	if receiptWord != "" {
		decision, guardErr := domain.GuardReceiptAppend(target.LastReceiptOutcome, receiptWord)
		if guardErr != nil {
			return guardErr
		}
		switch decision {
		case domain.ReceiptAppendAlarm:
			r.skip(op.OperationID, attemptNo, domain.ReceiptAppendAlarmText)
			return nil
		case domain.ReceiptAppendSkip:
			receiptWord = ""
			receiptSkip = true
		}
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
			ReceiptVersionNo: 0, // the store derives the monotonic version under the lock
			Outcome:          receiptWord,
			ExternalObjectID: evidence.ExternalObjectID,
			CreatedAt:        command.Now.UTC().Truncate(time.Microsecond),
		}
	}
	// The fenced verdict transaction (the signature carries ZERO
	// adapters): the outcome row of the NORMALIZED verdict (the attempt
	// ALWAYS records its guarded truth), the ledger word of the
	// state-bearing verdicts, the receipt v2+ with the outbox record (the
	// terminal lock inside the commit) — ONE commit; a stale pair NEVER
	// writes history.
	err = s.reconciliations.RecordReconciliationVerdict(ctx, port.RecordReconciliationVerdictParams{
		Scope:         scope,
		Operation:     op,
		AttemptNo:     attemptNo,
		LeaseToken:    leaseToken,
		Verdict:       verdict,
		LedgerWord:    effects.LedgerWord,
		ReceiptWord:   receiptWord,
		CorrelationID: correlation,
		Evidence:      evidence,
		Details:       boundDetails(details),
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
		ReceiptDetails: domain.ReconciliationVerdictText(verdict),
		OutboxEventID:  outboxEventID,
		OutboxPayload:  outboxPayload,
		Now:            command.Now,
	})
	if err != nil {
		if errors.Is(err, port.ErrReconciliationLeaseStale) {
			r.skip(op.OperationID, attemptNo, reconcileFencedRefusalText)
			return nil
		}
		if errors.Is(err, port.ErrTerminalConflict) {
			r.skip(op.OperationID, attemptNo, domain.ReceiptAppendAlarmText)
			return nil
		}
		return err
	}
	// The spoken line of the recorded verdict (FR-30 — every line names
	// its truth and the allowed next step): the receipt-append skip speaks
	// its idempotent confirmation; no_effect carries the retry-unlock word
	// of the dispatch policy; every other verdict speaks its fixed honest
	// text.
	if receiptSkip {
		r.decided(op.OperationID, attemptNo, domain.ReceiptAppendSkipText)
		return nil
	}
	if verdict == domain.ReconciliationVerdictNoEffect {
		r.decided(op.OperationID, attemptNo,
			domain.ReconciliationNoEffectText+" "+domain.DispatchNoEffectUnlockText)
		return nil
	}
	r.decided(op.OperationID, attemptNo, domain.ReconciliationVerdictText(verdict))
	return nil
}

// ---------------------------------------------------------------------------
// Story 3.7 (step-03): the api projection of the contour (the transport
// renders the DTO layer only — a private copy would drift; the
// DrainDispatchQueue precedent of 3-6)
// ---------------------------------------------------------------------------

// The compile-time proof of the narrow transport registration.
var _ api.ReconcileService = (*Service)(nil)

// ReconcileOperations is the api projection of the EXPLICIT reconcile
// command: the honest report of the bounded run, every considered target
// spoken by its line (FR-30 — never a masked silence).
func (s *Service) ReconcileOperations(ctx context.Context, command api.ReconcileOperationsCommand) (api.ReconcileResultView, error) {
	report, err := s.ReconcileUnknownOperations(ctx, ReconcileCommand{
		Actor: command.Actor, TenantID: command.TenantID, Now: command.Now,
	})
	if err != nil {
		return api.ReconcileResultView{}, err
	}
	result := api.ReconcileResultView{
		Command:    report.Command,
		Considered: report.Considered,
		Queried:    report.Queried,
		Decided:    report.Decided,
		Skipped:    report.Skipped,
		Reports:    []api.ReconcileReportLineView{},
	}
	for _, line := range report.Reports {
		result.Reports = append(result.Reports, api.ReconcileReportLineView{
			OperationID: line.OperationID, AttemptNo: line.AttemptNo, Text: line.Text,
		})
	}
	return result, nil
}

// reconcileVerdictKnown answers whether the stored verdict word belongs
// to the four-word dictionary (the honest integrity check of a stored
// row — a foreign word reads as the resting absence, never a masked
// state; the attemptOutcomeText precedent of 3-6).
func reconcileVerdictKnown(word string) bool {
	for _, candidate := range domain.ReconciliationVerdictWords {
		if candidate == word {
			return true
		}
	}
	return false
}

// reconcileVerdictText speaks the honest text of one attempt's verdict
// word (the verdict texts of the domain reused — never a private copy);
// the un-recorded attempt speaks the write-ahead resting word by name.
func reconcileVerdictText(word string) string {
	if word == "" || !reconcileVerdictKnown(word) {
		return domain.ReconciliationAttemptRestingText
	}
	return domain.ReconciliationVerdictText(word)
}

// reconciliationAttemptViews maps the attempt rows onto the SAFE view
// material: the write-ahead lease facts (the right and the moment of the
// query landed BEFORE the network call), the material backoff moment, the
// honest verdict word WHEN recorded (the four-word dictionary), the
// provider correlation and the external object identity of the committed
// evidence — identifiers and words only, the material NEVER enters
// (AC-5/AD-10).
func reconciliationAttemptViews(rows []port.ReconciliationAttemptRow) []api.ReconciliationAttemptView {
	views := make([]api.ReconciliationAttemptView, 0, len(rows))
	for _, row := range rows {
		views = append(views, api.ReconciliationAttemptView{
			AttemptNo:        row.AttemptNo,
			LeaseCreatedAt:   row.LeaseCreatedAt,
			LeaseExpiresAt:   row.LeaseExpiresAt,
			NextEligibleAt:   row.NextEligibleAt,
			Verdict:          row.OutcomeVerdict,
			VerdictRecorded:  row.OutcomeVerdict != "",
			VerdictText:      reconcileVerdictText(row.OutcomeVerdict),
			CorrelationID:    row.CorrelationID,
			ExternalObjectID: row.ExternalObjectID,
			VerdictCreatedAt: row.OutcomeCreatedAt,
		})
	}
	return views
}
