package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.7 step-02: the fake-repo unit matrix of the reconcile worker
// ---------------------------------------------------------------------------
//
// The counters pin the FROZEN ladder order of the reconcile command: every
// fence is a NON-QUERYING block — the adapter is NEVER called (the call
// counter), the lease is NEVER taken (the lease counter) on any skip; the
// write-ahead lease lands BEFORE the ONE adapter call
// (persist-before-emit, AD-6 — the order log pins it); the verdict
// transaction is fenced — a stale pair NEVER writes history; the terminal
// lock of the receipts decides BEFORE the write (an append over the last
// unknown, the same-terminal confirmation skip, the conflicting-terminal
// alarm that never records); the reflect probe pins the READ-ONLY shape
// of the adapter port and the adapter-free signatures of the two store
// transactions.

// ---------------------------------------------------------------------------
// The fake read-only query adapter (the consumer-owned port of the test
// contour)
// ---------------------------------------------------------------------------

// fakeReconciliationAdapter is the scriptable adapter of the unit contour:
// the served capability word, the ONE call counter and the scripted answer
// (the call material is pinned for the key probes); the order hook lets
// the shared order log speak the write-ahead discipline.
type fakeReconciliationAdapter struct {
	capability string
	calls      int
	seen       []port.ReconciliationQueryRequest
	result     port.ReconciliationQueryResult
	err        error
	onQuery    func()
}

func (f *fakeReconciliationAdapter) ServesCapability() string { return f.capability }

func (f *fakeReconciliationAdapter) QueryOutcome(_ context.Context, _ tenancy.Context, request port.ReconciliationQueryRequest) (port.ReconciliationQueryResult, error) {
	f.calls++
	f.seen = append(f.seen, request)
	if f.onQuery != nil {
		f.onQuery()
	}
	if f.err != nil {
		return port.ReconciliationQueryResult{}, f.err
	}
	return f.result, nil
}

// noReconciliationCaps answers the LIVE catalog declaration WITHOUT the
// reconciliation word (the not-declared fence probe: the connector dropped
// the declaration — the no_effect claim would read no_answer, the target
// stays unknown for the human).
type noReconciliationCaps struct{}

func (noReconciliationCaps) Capability(string, string, string) (domain.CapabilityFacts, bool) {
	return domain.CapabilityFacts{Idempotent: true, Reconciliation: ""}, true
}

// ---------------------------------------------------------------------------
// The fake reconciliation repository (the store's semantics: the advisory
// fencing, the raced refusals, the receipt-version derivation)
// ---------------------------------------------------------------------------

// fakeReconcileLeaseRow is one lease of the fake: the attempt number, the
// holder token, the window and the backoff moment.
type fakeReconcileLeaseRow struct {
	attemptNo    int
	token        string
	expiresAt    time.Time
	nextEligible time.Time
	reconWord    string
}

// fakeReconciliationRepo mirrors the store's TWO transactions: the
// write-ahead lease (the racing refusal on an active lease, the monotonic
// attempt under the bound) and the fenced verdict (the (attempt, token)
// pair must own the CURRENT attempt — a stale holder NEVER writes
// history; the receipt version derives monotonically, the outbox payload
// patched by the store truth). The pending targets are the prepared
// fixtures (the classification is the store's derived fact). The order
// log speaks the write-ahead discipline: "lease" BEFORE "query" BEFORE
// "verdict".
type fakeReconciliationRepo struct {
	pendingCalls int
	leaseCalls   int
	verdictCalls int
	order        []string

	targets []port.PendingReconciliationTarget

	leaseParams  []port.AcquireReconciliationLeaseParams
	leases       map[id.UUID][]fakeReconcileLeaseRow
	outcomes     map[id.UUID]map[int]port.RecordReconciliationVerdictParams
	ledger       map[id.UUID][]domain.OperationLedgerEntry
	receipts     map[id.UUID][]port.RecordReconciliationVerdictParams
	receiptNext  map[id.UUID]int64
	outboxEvents []port.ReceiptCreatedPayload

	failPending    error
	failLease      error
	failVerdict    error
	verdictRefusal error
}

func newFakeReconciliationRepo() *fakeReconciliationRepo {
	return &fakeReconciliationRepo{
		leases:      map[id.UUID][]fakeReconcileLeaseRow{},
		outcomes:    map[id.UUID]map[int]port.RecordReconciliationVerdictParams{},
		ledger:      map[id.UUID][]domain.OperationLedgerEntry{},
		receipts:    map[id.UUID][]port.RecordReconciliationVerdictParams{},
		receiptNext: map[id.UUID]int64{},
	}
}

func (f *fakeReconciliationRepo) PendingReconciliationTargets(_ context.Context, _ tenancy.Context, _ time.Time, _ int) ([]port.PendingReconciliationTarget, error) {
	f.pendingCalls++
	if f.failPending != nil {
		return nil, f.failPending
	}
	return append([]port.PendingReconciliationTarget(nil), f.targets...), nil
}

func (f *fakeReconciliationRepo) lastReconcileLease(operationID id.UUID) (fakeReconcileLeaseRow, bool) {
	rows := f.leases[operationID]
	if len(rows) == 0 {
		return fakeReconcileLeaseRow{}, false
	}
	return rows[len(rows)-1], true
}

func (f *fakeReconciliationRepo) AcquireReconciliationLease(_ context.Context, p port.AcquireReconciliationLeaseParams) (int, error) {
	f.leaseCalls++
	if f.failLease != nil {
		return 0, f.failLease
	}
	operationID := p.Operation.OperationID
	if last, ok := f.lastReconcileLease(operationID); ok && last.expiresAt.After(p.Now) {
		return 0, port.ErrReconciliationLeaseRaced
	}
	attemptNo := 1
	if last, ok := f.lastReconcileLease(operationID); ok {
		attemptNo = last.attemptNo + 1
	}
	if attemptNo > domain.MaxReconciliationAttempts {
		return 0, port.ErrReconciliationLeaseStale
	}
	f.leaseParams = append(f.leaseParams, p)
	f.leases[operationID] = append(f.leases[operationID], fakeReconcileLeaseRow{
		attemptNo: attemptNo, token: p.LeaseToken, expiresAt: p.LeaseExpiresAt,
		nextEligible: p.NextEligibleAt, reconWord: p.ReconciliationWord,
	})
	f.ledger[operationID] = append(f.ledger[operationID], p.LedgerEntry)
	f.order = append(f.order, "lease")
	return attemptNo, nil
}

func (f *fakeReconciliationRepo) RecordReconciliationVerdict(_ context.Context, p port.RecordReconciliationVerdictParams) error {
	f.verdictCalls++
	if f.failVerdict != nil {
		return f.failVerdict
	}
	if f.verdictRefusal != nil {
		return f.verdictRefusal
	}
	operationID := p.Operation.OperationID
	lease, ok := fakeReconcileLeaseOf(f.leases[operationID], p.AttemptNo)
	if !ok || lease.token != p.LeaseToken {
		return port.ErrReconciliationLeaseStale
	}
	if f.outcomes[operationID] == nil {
		f.outcomes[operationID] = map[int]port.RecordReconciliationVerdictParams{}
	}
	f.outcomes[operationID][p.AttemptNo] = p
	if p.LedgerWord != "" {
		f.ledger[operationID] = append(f.ledger[operationID], domain.OperationLedgerEntry{
			EntryNo: int64(len(f.ledger[operationID]) + 1),
			Kind:    p.LedgerWord,
			Details: p.ReceiptDetails,
		})
	}
	if p.ReceiptWord != "" {
		f.receipts[operationID] = append(f.receipts[operationID], p)
		version := f.receiptNext[operationID] + 1
		f.receiptNext[operationID] = version
		payload := p.OutboxPayload
		payload.ReceiptVersionNo = version
		payload.Outcome = p.ReceiptWord
		f.outboxEvents = append(f.outboxEvents, payload)
	}
	f.order = append(f.order, "verdict")
	return nil
}

func fakeReconcileLeaseOf(rows []fakeReconcileLeaseRow, attemptNo int) (fakeReconcileLeaseRow, bool) {
	for _, row := range rows {
		if row.attemptNo == attemptNo {
			return row, true
		}
	}
	return fakeReconcileLeaseRow{}, false
}

func (f *fakeReconciliationRepo) ReadReconciliationAttempts(_ context.Context, _ tenancy.Context, operationID id.UUID) ([]port.ReconciliationAttemptRow, error) {
	rows := []port.ReconciliationAttemptRow{}
	for _, lease := range f.leases[operationID] {
		row := port.ReconciliationAttemptRow{AttemptNo: lease.attemptNo, LeaseToken: lease.token,
			LeaseExpiresAt: lease.expiresAt, NextEligibleAt: lease.nextEligible, ReconciliationWord: lease.reconWord}
		if outcome, ok := f.outcomes[operationID][lease.attemptNo]; ok {
			row.OutcomeVerdict = outcome.Verdict
			row.CorrelationID = outcome.CorrelationID
			row.ExternalObjectID = outcome.Evidence.ExternalObjectID
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (f *fakeReconciliationRepo) LastReconciliationNoEffect(_ context.Context, _ tenancy.Context, operationID id.UUID) (bool, error) {
	for _, outcome := range f.outcomes[operationID] {
		if outcome.Verdict == domain.ReconciliationVerdictNoEffect {
			return true, nil
		}
	}
	return false, nil
}

// verdictOf answers the recorded verdict of one attempt (the outcome row
// of the attempt's guarded truth).
func (f *fakeReconciliationRepo) verdictOf(operationID id.UUID, attemptNo int) (port.RecordReconciliationVerdictParams, bool) {
	verdict, ok := f.outcomes[operationID][attemptNo]
	return verdict, ok
}

// ---------------------------------------------------------------------------
// The reconcile fixture: the authorized operation of the 3-5 flow + the
// fake repository with the prepared fresh target + the scriptable adapter
// ---------------------------------------------------------------------------

// reconcileControl counts the reads and answers one configured effective
// reading (the order probes of the light ladder: the earlier fences answer
// BEFORE the control read).
type reconcileControl struct {
	mode  port.ControlEffective
	calls int
}

func (f *reconcileControl) EffectiveFor(_ context.Context, _ id.UUID, _ port.ControlRoute) (port.ControlEffective, error) {
	f.calls++
	return f.mode, nil
}

// reconcileFixture is the mutable world of one reconcile test: the FULL
// authorization fixture (the preview/consent/operation of the living
// confirm→authorize flow), the fake reconciliation repository over the
// unknown-receipt target and the scriptable read-only adapter.
type reconcileFixture struct {
	auth    *authorizationFixture
	op      domain.Operation
	repo    *fakeReconciliationRepo
	adapter *fakeReconciliationAdapter
	control *reconcileControl
}

// theDispatchFacts are the target's dispatch facts the query reuses (the
// FIRST lease key/digest, the last dispatch correlation — 64-hex shaped).
const (
	targetDigestKey   = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	targetExternalKey = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	targetCorrelation = "corr-dispatch-9"
)

// newReconcileFixture runs the LIVING 3-5 flow (confirm → authorize) and
// prepares the FRESH pending target over the immutable operation: the
// ledger ladder of a dispatch that ended unknown (authorized → submitted
// → unknown) with the receipt v1 {unknown} facts — the at-least-once
// anchor of the contour.
func newReconcileFixture(t *testing.T) *reconcileFixture {
	t.Helper()
	auth := newAuthorizationFixture(t)
	thread, version := auth.confirm(t)
	const key = "reconcile-unit-key-1"
	auth.authorize(t, thread, version, key)
	op, ok := auth.ops.operationOfKey(key)
	if !ok {
		t.Fatal("авторизация обязана оставить операцию в фейковом репозитории")
	}
	repo := newFakeReconciliationRepo()
	ledger := []domain.OperationLedgerEntry{
		{EntryNo: 1, Kind: domain.LedgerEntryAuthorized, Details: "авторизовано"},
		{EntryNo: 2, Kind: domain.LedgerEntrySubmitted, Details: "write-ahead"},
		{EntryNo: 3, Kind: domain.LedgerEntryUnknown, Details: "исход неизвестен"},
	}
	repo.targets = []port.PendingReconciliationTarget{{
		Operation:               op,
		Ledger:                  append([]domain.OperationLedgerEntry(nil), ledger...),
		StateWord:               domain.OperationUnknown,
		Classification:          port.ReconcileClassFresh,
		RequestDigest:           targetDigestKey,
		ExternalIDempotencyKey:  targetExternalKey,
		LastDispatchCorrelation: targetCorrelation,
		LastReceiptVersionNo:    1,
		LastReceiptOutcome:      domain.ReceiptOutcomeUnknown,
	}}
	repo.ledger[op.OperationID] = append([]domain.OperationLedgerEntry(nil), ledger...)
	repo.receiptNext[op.OperationID] = 1
	adapter := &fakeReconciliationAdapter{capability: auth.def.RequiredCapability.CapabilityID}
	control := &reconcileControl{mode: port.ControlEffective{Mode: domain.ScopedControlAccept, Generation: 3}}
	return &reconcileFixture{auth: auth, op: op, repo: repo, adapter: adapter, control: control}
}

// reconcileService wires the reconciliation contour over the SHARED 3-5
// fakes: the operations scope gate, the full preview contour (the
// fingerprint key, the control reader, the LIVE catalog resolver) and the
// reconcile wiring with the given adapter (nil = the AG-4 production
// probe) and the optional dictionary override (the empty-query probe).
func (f *reconcileFixture) reconcileService(adapter port.ReconciliationAdapter,
	caps domain.CapabilityRefResolver, defs []domain.ActionDefinition) *Service {
	if caps == nil {
		caps = acceptAllCaps{}
	}
	if defs == nil {
		defs = f.auth.defs
	}
	return New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Operations: true}},
		acceptAllPacks{}, caps, defs).
		WithPreviews(f.auth.previews, f.control, f.auth.material,
			liveMembers{world: &f.auth.world}, fakeEntitlement{status: f.auth.world.admission},
			fakeReadiness{stage: f.auth.world.stage, instances: f.auth.world.instances},
			fakeConnectors{state: f.auth.world.capState, contractVersion: f.auth.world.contract},
			fakeRouteFacts{profileVersion: f.auth.world.profile}, &fakeIDs{}).
		WithReconcile(f.repo, adapter)
}

// command is the passing reconcile command of the fixture moment.
func (f *reconcileFixture) command() ReconcileCommand {
	return ReconcileCommand{Actor: testActor, TenantID: testTenant, Now: f.auth.now}
}

// committedReconcileResult is the honest committed answer with the
// complete structural evidence triple.
func committedReconcileResult() port.ReconciliationQueryResult {
	return port.ReconciliationQueryResult{
		ClaimedVerdict:        domain.ReconciliationVerdictCommitted,
		CorrelationID:         "corr-recon-42",
		ExternalObjectID:      "ext-msg-42",
		ExternalObjectVersion: "3",
		ExternalObjectTime:    time.Date(2026, 10, 20, 12, 0, 30, 0, time.UTC),
		Details:               "ответ системы-владельца",
	}
}

// ---------------------------------------------------------------------------
// The AG-4 boundary
// ---------------------------------------------------------------------------

// TestReconcileNilAdapterRefusesAG4 — the nil read-only adapter is the
// HONEST production refusal FIRST: the reconcile command answers
// ErrReconcileRefusedAG4, the pending targets are never even read (the
// pending counter stays zero), no lease is taken, no query is made.
func TestReconcileNilAdapterRefusesAG4(t *testing.T) {
	f := newReconcileFixture(t)
	service := f.reconcileService(nil, nil, nil)
	report, err := service.ReconcileUnknownOperations(context.Background(), f.command())
	if !errors.Is(err, port.ErrReconcileRefusedAG4) {
		t.Fatalf("nil-адаптер обязан отказывать честным AG-4 отказом: %v", err)
	}
	if report.Considered != 0 || report.Queried != 0 || report.Decided != 0 {
		t.Fatalf("отказ AG-4 не рассматривает очередь сверки: %+v", report)
	}
	if f.repo.pendingCalls != 0 {
		t.Fatalf("очередь сверки не читается до отказа AG-4: pending=%d", f.repo.pendingCalls)
	}
	if f.repo.leaseCalls != 0 || f.adapter.calls != 0 {
		t.Fatalf("отказ AG-4 не берёт lease и не запрашивает: lease=%d adapter=%d", f.repo.leaseCalls, f.adapter.calls)
	}
}

// ---------------------------------------------------------------------------
// The light read-only ladder: every fence skips WITHOUT a lease and
// WITHOUT the adapter call (the counters); the order probes pin the ladder
// ---------------------------------------------------------------------------

// reconcileFenceProbe is one fence scenario: the mutation of the fixture
// world BEFORE the run, the expected spoken line and the control-read
// expectation (0 — the fence answers BEFORE the control read; 1 — the
// control was read before the refusal).
type reconcileFenceProbe struct {
	name        string
	caps        domain.CapabilityRefResolver
	defs        []domain.ActionDefinition
	mutate      func(f *reconcileFixture, command *ReconcileCommand)
	want        string
	controlRead int
}

// TestReconcileFencesSkipWithoutLeaseOrQuery — the frozen ladder matrix:
// every boundary is the NON-QUERYING skip — the adapter counter stays
// ZERO, the lease counter stays ZERO; the ladder order pins by the
// control-read counter (the classification, the fingerprint and the
// declared-semantics fences answer BEFORE the control read; the paused
// control and the foreign-capability gates answer AFTER it).
func TestReconcileFencesSkipWithoutLeaseOrQuery(t *testing.T) {
	probes := []reconcileFenceProbe{
		{
			name: "racing lease",
			mutate: func(f *reconcileFixture, _ *ReconcileCommand) {
				f.repo.targets[0].Classification = port.ReconcileClassRacing
				f.repo.targets[0].LastReconcileAttemptNo = 1
			},
			want: domain.ReconciliationRacingText,
		},
		{
			name: "exhausted attempts",
			mutate: func(f *reconcileFixture, _ *ReconcileCommand) {
				f.repo.targets[0].Classification = port.ReconcileClassExhausted
				f.repo.targets[0].LastReconcileAttemptNo = domain.MaxReconciliationAttempts
			},
			want: domain.ReconciliationExhaustedText,
		},
		{
			name: "decided target",
			mutate: func(f *reconcileFixture, _ *ReconcileCommand) {
				f.repo.targets[0].Classification = port.ReconcileClassDecided
				f.repo.targets[0].LastReconcileAttemptNo = 1
				f.repo.targets[0].LastReceiptOutcome = domain.ReceiptOutcomeCommitted
			},
			want: fmt.Sprintf(domain.ReconciliationDecidedTextTemplate, domain.ReceiptOutcomeCommitted),
		},
		{
			name: "backoff window",
			mutate: func(f *reconcileFixture, _ *ReconcileCommand) {
				f.repo.targets[0].Classification = port.ReconcileClassWaiting
				f.repo.targets[0].LastReconcileAttemptNo = 1
				f.repo.targets[0].NextEligibleAt = waitingBackoffUntil
			},
			want: fmt.Sprintf(domain.ReconciliationBackoffTextTemplate, waitingBackoffUntil.Format(time.RFC3339)),
		},
		{
			name: "not declared: live word empty",
			caps: noReconciliationCaps{},
			want: domain.ReconciliationNotDeclaredText,
		},
		{
			name: "not declared: query word empty",
			defs: emptyQueryDefinitions(),
			want: domain.ReconciliationNotDeclaredText,
		},
		{
			name: "scoped control paused",
			mutate: func(f *reconcileFixture, _ *ReconcileCommand) {
				f.control.mode = port.ControlEffective{Mode: domain.ScopedControlPaused, Reason: "коннектор на паузе нанимателем"}
			},
			want: fmt.Sprintf(domain.ReconciliationPausedTextTemplate,
				domain.ScopedControlPaused+": коннектор на паузе нанимателем"),
			controlRead: 1,
		},
		{
			name: "fingerprint alarm",
			mutate: func(f *reconcileFixture, _ *ReconcileCommand) {
				tampered := f.op
				tampered.MaterialDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
				f.repo.targets[0].Operation = tampered
			},
			want: dispatchFingerprintAlarmText,
		},
		{
			name: "foreign adapter capability",
			mutate: func(f *reconcileFixture, _ *ReconcileCommand) {
				f.adapter.capability = "command.send-email"
			},
			want:        reconcileCapabilityForeignText,
			controlRead: 1,
		},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			f := newReconcileFixture(t)
			command := f.command()
			if probe.mutate != nil {
				probe.mutate(f, &command)
			}
			service := f.reconcileService(f.adapter, probe.caps, probe.defs)
			report, err := service.ReconcileUnknownOperations(context.Background(), command)
			if err != nil {
				t.Fatalf("фенция сверки обязана говорить честный skip, а не ломать команду: %v", err)
			}
			if report.Considered != 1 || report.Queried != 0 || report.Decided != 0 || report.Skipped != 1 {
				t.Fatalf("таргет рассмотрен и честно пропущен: %+v", report)
			}
			if len(report.Reports) != 1 || report.Reports[0].Text != probe.want {
				t.Fatalf("строка отчёта обязана говорить точное слово: %+v (хотели %q)", report.Reports, probe.want)
			}
			if f.repo.leaseCalls != 0 {
				t.Fatalf("фенция НЕ берёт lease: lease=%d", f.repo.leaseCalls)
			}
			if f.adapter.calls != 0 {
				t.Fatalf("фенция НЕ запрашивает адаптер: calls=%d", f.adapter.calls)
			}
			if f.control.calls != probe.controlRead {
				t.Fatalf("порядок лестницы: чтений контроля %d (хотели %d)", f.control.calls, probe.controlRead)
			}
		})
	}
}

// waitingBackoffUntil is the deterministic backoff moment of the waiting
// probe (a fixed future moment — the formatted line pin of the fence).
var waitingBackoffUntil = time.Date(2030, 1, 15, 9, 30, 0, 0, time.UTC)

// emptyQueryDefinitions copies the fixture dictionary with the EMPTY
// ReconciliationQuery word of the pinned definition (the second half of
// the declared-semantics fence: a declared LIVE word without a query word
// is NOT declared).
func emptyQueryDefinitions() []domain.ActionDefinition {
	defs := fixtureDefinitions()
	defs[0].DeliveryContract.ReconciliationQuery = ""
	return defs
}

// ---------------------------------------------------------------------------
// The query: the write-ahead lease, the ONE adapter call, the fenced
// verdict, the receipt v2+ and the outbox record
// ---------------------------------------------------------------------------

// TestReconcileFreshCommittedAppendsReceiptV2 — the full living query:
// the lease lands BEFORE the adapter call (the order log: lease → query
// → verdict), the ledger reconciling entry (NOT state-bearing) rides the
// lease commit, the adapter is queried EXACTLY ONCE with the derived key
// identities, the fenced verdict writes the outcome row committed with
// the evidence triple, the ledger word committed, the receipt v2
// {committed} with the SAFE references and the outbox receipt.created
// record carrying the derived version — ONE verdict transaction.
func TestReconcileFreshCommittedAppendsReceiptV2(t *testing.T) {
	f := newReconcileFixture(t)
	f.adapter.result = committedReconcileResult()
	f.adapter.onQuery = func() { f.repo.order = append(f.repo.order, "query") }
	service := f.reconcileService(f.adapter, nil, nil)
	report, err := service.ReconcileUnknownOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("живой путь сверки обязан проходить: %v", err)
	}
	if report.Command != "reconcile_unknown_operations" {
		t.Fatalf("отчёт несёт слово команды: %q", report.Command)
	}
	if report.Considered != 1 || report.Queried != 1 || report.Decided != 1 || report.Skipped != 0 {
		t.Fatalf("отчёт обязан говорить об одном вердикте: %+v", report)
	}
	// The write-ahead discipline: the lease commit precedes the ONE query,
	// the verdict lands after it.
	if len(f.repo.order) != 3 || f.repo.order[0] != "lease" || f.repo.order[1] != "query" || f.repo.order[2] != "verdict" {
		t.Fatalf("порядок write-ahead: lease → query → verdict: %v", f.repo.order)
	}
	if f.repo.leaseCalls != 1 || f.adapter.calls != 1 || f.repo.verdictCalls != 1 {
		t.Fatalf("ровно одна lease, один запрос и одна запись вердикта: lease=%d query=%d verdict=%d",
			f.repo.leaseCalls, f.adapter.calls, f.repo.verdictCalls)
	}
	// The write-ahead lease params: the LIVE reconciliation word of the
	// declaration, the timeout+grace window and the material backoff
	// moment [base..base+jitter].
	leaseParams := f.repo.leaseParams[0]
	if leaseParams.ReconciliationWord != "объявлена" {
		t.Fatalf("lease несёт LIVE-слово декларации коннектора: %q", leaseParams.ReconciliationWord)
	}
	if got := leaseParams.LeaseExpiresAt.Sub(f.command().Now); got != 45*time.Second {
		t.Fatalf("окно lease = таймаут определения (15с) + grace (30с): %v", got)
	}
	backoff := leaseParams.NextEligibleAt.Sub(f.command().Now)
	if backoff < 30*time.Second || backoff > 35*time.Second {
		t.Fatalf("момент next_eligible_at в окне [30с..35с]: %v", backoff)
	}
	if leaseParams.ExternalIDempotencyKey != targetExternalKey || leaseParams.RequestDigest != targetDigestKey ||
		leaseParams.CorrelationID != targetCorrelation {
		t.Fatalf("lease несёт keyed-идентичности dispatch-фактов: %+v", leaseParams)
	}
	if leaseParams.LedgerEntry.Kind != domain.LedgerEntryReconciling {
		t.Fatalf("ledger-запись lease — reconciling (НЕ state-bearing): %+v", leaseParams.LedgerEntry)
	}
	if !strings.Contains(leaseParams.LedgerEntry.Details, "reconciliation attempt 1") ||
		!strings.Contains(leaseParams.LedgerEntry.Details, "query.trunk-status") {
		t.Fatalf("ledger-запись lease говорит попытку и слово запроса: %q", leaseParams.LedgerEntry.Details)
	}
	// The ONE query material: the SAME external key, the keyed digest, the
	// last dispatch correlation and the declared query word.
	seen := f.adapter.seen[0]
	if seen.AttemptNo != 1 || seen.OperationID != f.op.OperationID {
		t.Fatalf("запрос несёт идентичность операции и попытки: %+v", seen)
	}
	if seen.ExternalIDempotencyKey != targetExternalKey || seen.RequestDigest != targetDigestKey ||
		seen.CorrelationID != targetCorrelation {
		t.Fatalf("запрос несёт keyed-идентичности dispatch-фактов: %+v", seen)
	}
	if seen.ReconciliationQuery != "query.trunk-status" {
		t.Fatalf("запрос несёт объявленное слово запроса: %q", seen.ReconciliationQuery)
	}
	// The fenced verdict: the outcome row of the normalized committed
	// verdict with the structural evidence triple.
	verdict, ok := f.repo.verdictOf(f.op.OperationID, 1)
	if !ok {
		t.Fatal("вердикт попытки 1 обязан быть записан")
	}
	if verdict.Verdict != domain.ReconciliationVerdictCommitted {
		t.Fatalf("нормализованный вердикт — committed: %+v", verdict)
	}
	if verdict.LedgerWord != domain.LedgerEntryCommitted || verdict.ReceiptWord != domain.ReceiptOutcomeCommitted {
		t.Fatalf("committed пишет пару {ledger committed, receipt committed}: %+v", verdict)
	}
	if verdict.Evidence.ExternalObjectID != "ext-msg-42" || verdict.Evidence.ExternalObjectVersion != "3" ||
		verdict.Evidence.ExternalObjectTime.IsZero() {
		t.Fatalf("вердикт несёт структурное evidence: %+v", verdict.Evidence)
	}
	if verdict.CorrelationID != "corr-recon-42" {
		t.Fatalf("вердикт несёт корреляцию ответа: %q", verdict.CorrelationID)
	}
	if verdict.ReceiptDetails != domain.ReconciliationCommittedText {
		t.Fatalf("квитанция говорит честный текст вердикта: %q", verdict.ReceiptDetails)
	}
	if verdict.ReceiptReferences.CaseID != f.op.CaseID || verdict.ReceiptReferences.WorkItemID != f.op.WorkItemID ||
		verdict.ReceiptReferences.PreviewID != f.op.PreviewID || verdict.ReceiptReferences.ConfirmationID != f.op.ConfirmationID ||
		verdict.ReceiptReferences.ExternalObjectID != "ext-msg-42" {
		t.Fatalf("квитанция несёт SAFE-пинны решения: %+v", verdict.ReceiptReferences)
	}
	if verdict.OutboxPayload.ReceiptVersionNo != 0 {
		t.Fatalf("приложение передаёт версию 0 — хранилище выводит её под замком: %+v", verdict.OutboxPayload)
	}
	// The ledger ladder: authorized → submitted → unknown → reconciling →
	// committed (reconciling is NOT state-bearing — the verdict word is).
	ledger := f.repo.ledger[f.op.OperationID]
	if len(ledger) != 5 || ledger[3].Kind != domain.LedgerEntryReconciling || ledger[4].Kind != domain.LedgerEntryCommitted {
		t.Fatalf("ledger-лестница ... → reconciling → committed: %+v", ledger)
	}
	// The receipt v2 {committed} (the terminal lock passed: the append
	// over the last unknown v1) and the outbox record with the derived
	// version.
	if len(f.repo.receipts[f.op.OperationID]) != 1 {
		t.Fatalf("ровно одна квитанция v2: %+v", f.repo.receipts)
	}
	if len(f.repo.outboxEvents) != 1 {
		t.Fatalf("ровно одно событие receipt.created: %+v", f.repo.outboxEvents)
	}
	event := f.repo.outboxEvents[0]
	if event.EventType != domain.OutboxEventReceiptCreated || event.OperationID != f.op.OperationID ||
		event.ReceiptVersionNo != 2 || event.Outcome != domain.ReceiptOutcomeCommitted ||
		event.ExternalObjectID != "ext-msg-42" {
		t.Fatalf("событие receipt.created обязано зеркалить квитанцию v2: %+v", event)
	}
	// The spoken line: the fixed honest text of the committed verdict.
	if len(report.Reports) != 1 || report.Reports[0].Text != domain.ReconciliationCommittedText {
		t.Fatalf("строка отчёта обязана говорить committed: %+v", report.Reports)
	}
}

// TestReconcileCommittedClaimWithoutEvidenceReadsNoAnswer — the terminal
// success is NEVER taken on word: a committed claim without the complete
// external object triple normalizes to no_answer — the outcome row records
// the guarded truth, NEITHER a ledger word NOR a receipt version lands,
// the operation STAYS unknown.
func TestReconcileCommittedClaimWithoutEvidenceReadsNoAnswer(t *testing.T) {
	f := newReconcileFixture(t)
	f.adapter.result = port.ReconciliationQueryResult{
		ClaimedVerdict: domain.ReconciliationVerdictCommitted,
		CorrelationID:  "corr-recon-9",
	}
	service := f.reconcileService(f.adapter, nil, nil)
	report, err := service.ReconcileUnknownOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("отказ evidence — no_answer, не сбой команды: %v", err)
	}
	if report.Queried != 1 || report.Decided != 1 || report.Skipped != 0 {
		t.Fatalf("попытка записана guarded-вердиктом: %+v", report)
	}
	verdict, ok := f.repo.verdictOf(f.op.OperationID, 1)
	if !ok || verdict.Verdict != domain.ReconciliationVerdictNoAnswer {
		t.Fatalf("нормализованный вердикт — no_answer: %+v", verdict)
	}
	if verdict.LedgerWord != "" || verdict.ReceiptWord != "" {
		t.Fatalf("no_answer НЕ пишет ни ledger, ни квитанцию: %+v", verdict)
	}
	if len(f.repo.receipts[f.op.OperationID]) != 0 || len(f.repo.outboxEvents) != 0 {
		t.Fatalf("no_answer не создаёт квитанцию и событие: %+v / %+v", f.repo.receipts, f.repo.outboxEvents)
	}
	ledger := f.repo.ledger[f.op.OperationID]
	if len(ledger) != 4 || ledger[3].Kind != domain.LedgerEntryReconciling {
		t.Fatalf("ledger хранит только write-ahead факт сверки: %+v", ledger)
	}
	if len(report.Reports) != 1 || report.Reports[0].Text != domain.ReconciliationNoAnswerText {
		t.Fatalf("строка отчёта обязана говорить no_answer: %+v", report.Reports)
	}
}

// TestReconcileNoEffectUnlocksFailedPair — the PROVEN absence under the
// declared semantics: the verdict no_effect writes the pair {ledger
// failed, receipt failed} — the absence proof rides the receipt CONTENT,
// the spoken line carries the retry-unlock word of the dispatch policy.
func TestReconcileNoEffectUnlocksFailedPair(t *testing.T) {
	f := newReconcileFixture(t)
	f.adapter.result = port.ReconciliationQueryResult{
		ClaimedVerdict: domain.ReconciliationVerdictNoEffect,
		CorrelationID:  "corr-recon-7",
		Details:        "внешнего объекта нет по объявленной семантике",
	}
	service := f.reconcileService(f.adapter, nil, nil)
	report, err := service.ReconcileUnknownOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("no_effect — честный доказанный вердикт: %v", err)
	}
	if report.Decided != 1 {
		t.Fatalf("вердикт no_effect записан: %+v", report)
	}
	verdict, ok := f.repo.verdictOf(f.op.OperationID, 1)
	if !ok || verdict.Verdict != domain.ReconciliationVerdictNoEffect {
		t.Fatalf("нормализованный вердикт — no_effect: %+v", verdict)
	}
	if verdict.LedgerWord != domain.LedgerEntryFailed || verdict.ReceiptWord != domain.ReceiptOutcomeFailed {
		t.Fatalf("no_effect пишет пару {ledger failed, receipt failed}: %+v", verdict)
	}
	if len(f.repo.receipts[f.op.OperationID]) != 1 ||
		f.repo.receipts[f.op.OperationID][0].ReceiptWord != domain.ReceiptOutcomeFailed {
		t.Fatalf("квитанция v2 {failed} обязательна: %+v", f.repo.receipts)
	}
	if len(f.repo.outboxEvents) != 1 || f.repo.outboxEvents[0].Outcome != domain.ReceiptOutcomeFailed {
		t.Fatalf("событие receipt.created {failed} обязательно: %+v", f.repo.outboxEvents)
	}
	ledger := f.repo.ledger[f.op.OperationID]
	if len(ledger) != 5 || ledger[4].Kind != domain.LedgerEntryFailed {
		t.Fatalf("ledger завершается failed: %+v", ledger)
	}
	if len(report.Reports) != 1 {
		t.Fatalf("одна строка отчёта: %+v", report.Reports)
	}
	line := report.Reports[0].Text
	if !strings.Contains(line, domain.ReconciliationNoEffectText) || !strings.Contains(line, domain.DispatchNoEffectUnlockText) {
		t.Fatalf("строка no_effect обязана говорить и вердикт, и разблокировку повтора: %q", line)
	}
}

// TestReconcileInProgressWritesNeitherLedgerNorReceipt — the declared
// intermediate: the verdict records, the operation honestly STAYS unknown
// — NO ledger word, NO receipt version, NO outbox record.
func TestReconcileInProgressWritesNeitherLedgerNorReceipt(t *testing.T) {
	f := newReconcileFixture(t)
	f.adapter.result = port.ReconciliationQueryResult{
		ClaimedVerdict: domain.ReconciliationVerdictInProgress,
		CorrelationID:  "corr-recon-5",
	}
	service := f.reconcileService(f.adapter, nil, nil)
	report, err := service.ReconcileUnknownOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("in_progress — честный промежуточный итог: %v", err)
	}
	if report.Decided != 1 {
		t.Fatalf("вердикт in_progress записан: %+v", report)
	}
	verdict, ok := f.repo.verdictOf(f.op.OperationID, 1)
	if !ok || verdict.Verdict != domain.ReconciliationVerdictInProgress {
		t.Fatalf("нормализованный вердикт — in_progress: %+v", verdict)
	}
	if verdict.LedgerWord != "" || verdict.ReceiptWord != "" {
		t.Fatalf("in_progress НЕ пишет ни ledger, ни квитанцию: %+v", verdict)
	}
	if len(f.repo.receipts[f.op.OperationID]) != 0 || len(f.repo.outboxEvents) != 0 {
		t.Fatalf("in_progress не создаёт квитанцию и событие: %+v / %+v", f.repo.receipts, f.repo.outboxEvents)
	}
	if len(report.Reports) != 1 || report.Reports[0].Text != domain.ReconciliationInProgressText {
		t.Fatalf("строка отчёта обязана говорить in_progress: %+v", report.Reports)
	}
}

// TestReconcileAdapterErrorReadsNoAnswer — the call failure is the honest
// absence of a verdict: the outcome row records no_answer, NEITHER a
// ledger word NOR a receipt lands — never a masked success, never a
// masked absence.
func TestReconcileAdapterErrorReadsNoAnswer(t *testing.T) {
	f := newReconcileFixture(t)
	f.adapter.err = errors.New("система-владелец недоступна")
	service := f.reconcileService(f.adapter, nil, nil)
	report, err := service.ReconcileUnknownOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("ошибка вызова — честный no_answer, не сбой команды: %v", err)
	}
	if report.Queried != 1 || report.Decided != 1 {
		t.Fatalf("запрос состоялся и записан guarded-вердиктом: %+v", report)
	}
	verdict, ok := f.repo.verdictOf(f.op.OperationID, 1)
	if !ok || verdict.Verdict != domain.ReconciliationVerdictNoAnswer {
		t.Fatalf("нормализованный вердикт — no_answer: %+v", verdict)
	}
	if verdict.LedgerWord != "" || verdict.ReceiptWord != "" {
		t.Fatalf("no_answer НЕ пишет ни ledger, ни квитанцию: %+v", verdict)
	}
	if len(report.Reports) != 1 || report.Reports[0].Text != domain.ReconciliationNoAnswerText {
		t.Fatalf("строка отчёта обязана говорить no_answer: %+v", report.Reports)
	}
}

// ---------------------------------------------------------------------------
// The terminal lock of the receipts: the alarm never records, the
// same-terminal confirmation records without a new version
// ---------------------------------------------------------------------------

// TestReconcileTerminalLockAlarmAndConfirmation — the receipt terminal
// lock decides BEFORE the write: a CONFLICTING terminal claim (failed
// over the recorded committed) is the integrity alarm — the verdict is
// NOT recorded at all (the human review owns the resolution, never an
// auto-replacement); the SAME-terminal confirmation (committed over the
// recorded committed) is the idempotent skip — the outcome row records,
// NO new receipt version, NO outbox record.
func TestReconcileTerminalLockAlarmAndConfirmation(t *testing.T) {
	f := newReconcileFixture(t)
	f.repo.targets[0].LastReceiptOutcome = domain.ReceiptOutcomeCommitted
	f.adapter.result = committedReconcileResult()
	service := f.reconcileService(f.adapter, nil, nil)
	report, err := service.ReconcileUnknownOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("идемпотентное подтверждение — честная запись без новой версии: %v", err)
	}
	if report.Decided != 1 || report.Skipped != 0 {
		t.Fatalf("подтверждение записывает вердикт попытки: %+v", report)
	}
	verdict, ok := f.repo.verdictOf(f.op.OperationID, 1)
	if !ok || verdict.Verdict != domain.ReconciliationVerdictCommitted {
		t.Fatalf("outcome-строка попытки записана словом вердикта: %+v", verdict)
	}
	if verdict.ReceiptWord != "" {
		t.Fatalf("замок skip не передаёт ReceiptWord: %+v", verdict)
	}
	if len(f.repo.receipts[f.op.OperationID]) != 0 || len(f.repo.outboxEvents) != 0 {
		t.Fatalf("подтверждение НЕ создаёт квитанцию и событие: %+v / %+v", f.repo.receipts, f.repo.outboxEvents)
	}
	if len(report.Reports) != 1 || report.Reports[0].Text != domain.ReceiptAppendSkipText {
		t.Fatalf("строка отчёта обязана говорить идемпотентное подтверждение: %+v", report.Reports)
	}
	// The conflicting terminal: the no_effect claim over the recorded
	// committed — the integrity alarm, the verdict is NOT recorded at all.
	f2 := newReconcileFixture(t)
	f2.repo.targets[0].LastReceiptOutcome = domain.ReceiptOutcomeCommitted
	f2.adapter.result = port.ReconciliationQueryResult{
		ClaimedVerdict: domain.ReconciliationVerdictNoEffect,
		CorrelationID:  "corr-recon-late",
	}
	service2 := f2.reconcileService(f2.adapter, nil, nil)
	report2, err := service2.ReconcileUnknownOperations(context.Background(), f2.command())
	if err != nil {
		t.Fatalf("конфликт терминалов — integrity alarm, не сбой команды: %v", err)
	}
	if report2.Skipped != 1 || report2.Decided != 0 {
		t.Fatalf("конфликт — честный skip без записи вердикта: %+v", report2)
	}
	if f2.repo.verdictCalls != 0 {
		t.Fatalf("alarm НЕ записывает вердикт вовсе: verdict=%d", f2.repo.verdictCalls)
	}
	if len(f2.repo.receipts[f2.op.OperationID]) != 0 || len(f2.repo.outboxEvents) != 0 {
		t.Fatalf("alarm не создаёт квитанцию и событие: %+v / %+v", f2.repo.receipts, f2.repo.outboxEvents)
	}
	// The lease and the query DID happen (they are committed facts); the
	// HISTORY is what the terminal lock refuses.
	if f2.repo.leaseCalls != 1 || f2.adapter.calls != 1 {
		t.Fatalf("write-ahead lease и запрос — факты: lease=%d query=%d", f2.repo.leaseCalls, f2.adapter.calls)
	}
	if len(report2.Reports) != 1 || report2.Reports[0].Text != domain.ReceiptAppendAlarmText {
		t.Fatalf("строка отчёта обязана говорить alarm конфликта терминалов: %+v", report2.Reports)
	}
}

// ---------------------------------------------------------------------------
// The fenced refusal: a stale pair NEVER writes history
// ---------------------------------------------------------------------------

// TestReconcileFencedRefusalSpeaksHonestWord — the stale (attempt, token)
// pair refusal of the verdict recording and the terminal conflict of the
// store lock answer the honest spoken words, never a command failure.
func TestReconcileFencedRefusalSpeaksHonestWord(t *testing.T) {
	f := newReconcileFixture(t)
	f.adapter.result = committedReconcileResult()
	f.repo.verdictRefusal = port.ErrReconciliationLeaseStale
	service := f.reconcileService(f.adapter, nil, nil)
	report, err := service.ReconcileUnknownOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("fencing-отказ — честный skip, не сбой команды: %v", err)
	}
	if report.Skipped != 1 || report.Decided != 0 {
		t.Fatalf("устаревшая пара сверки получает честный skip: %+v", report)
	}
	if len(report.Reports) != 1 || report.Reports[0].Text != reconcileFencedRefusalText {
		t.Fatalf("строка отчёта обязана говорить fencing-отказ: %+v", report.Reports)
	}
	// The terminal conflict refusal of the store lock (the belt over
	// braces — the store re-runs the guard under the lock).
	f2 := newReconcileFixture(t)
	f2.adapter.result = committedReconcileResult()
	f2.repo.verdictRefusal = port.ErrTerminalConflict
	service2 := f2.reconcileService(f2.adapter, nil, nil)
	report2, err := service2.ReconcileUnknownOperations(context.Background(), f2.command())
	if err != nil {
		t.Fatalf("конфликт хранилища — честный skip, не сбой команды: %v", err)
	}
	if report2.Skipped != 1 || report2.Decided != 0 {
		t.Fatalf("конфликт хранилища — честный skip: %+v", report2)
	}
	if len(report2.Reports) != 1 || report2.Reports[0].Text != domain.ReceiptAppendAlarmText {
		t.Fatalf("строка отчёта обязана говорить alarm терминального замка: %+v", report2.Reports)
	}
}

// ---------------------------------------------------------------------------
// The backoff gate: the honest «not ready until» word
// ---------------------------------------------------------------------------

// TestReconcileBackoffGateSpeaksTimeWindow — the waiting target speaks
// the EXACT next_eligible_at moment of the material backoff: no lease, no
// query — the window governs.
func TestReconcileBackoffGateSpeaksTimeWindow(t *testing.T) {
	f := newReconcileFixture(t)
	until := f.command().Now.Add(90 * time.Second)
	f.repo.targets[0].Classification = port.ReconcileClassWaiting
	f.repo.targets[0].LastReconcileAttemptNo = 1
	f.repo.targets[0].NextEligibleAt = until
	service := f.reconcileService(f.adapter, nil, nil)
	report, err := service.ReconcileUnknownOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("окно backoff — честный skip, не сбой команды: %v", err)
	}
	if report.Skipped != 1 || report.Queried != 0 {
		t.Fatalf("окно backoff пропускает таргет без запроса: %+v", report)
	}
	if f.repo.leaseCalls != 0 || f.adapter.calls != 0 {
		t.Fatalf("окно backoff НЕ берёт lease и НЕ запрашивает: lease=%d query=%d", f.repo.leaseCalls, f.adapter.calls)
	}
	if len(report.Reports) != 1 {
		t.Fatalf("одна строка отчёта: %+v", report.Reports)
	}
	if want := fmt.Sprintf(domain.ReconciliationBackoffTextTemplate, until.Format(time.RFC3339)); report.Reports[0].Text != want {
		t.Fatalf("строка обязана говорить точный момент next_eligible_at: %q (хотели %q)", report.Reports[0].Text, want)
	}
	if !strings.Contains(report.Reports[0].Text, until.Format(time.RFC3339)) {
		t.Fatalf("строка содержит отформатированное время окна: %q", report.Reports[0].Text)
	}
}

// ---------------------------------------------------------------------------
// The signature probes: the READ-ONLY adapter and the adapter-free
// transactions
// ---------------------------------------------------------------------------

// TestReconciliationAdapterIsReadOnlyByStructure — the port is READ-ONLY
// BY STRUCTURE (epic AC-1): not a single mutation method name; the two
// transactional parameter structures of the repository carry NO interface
// field at all — a network call is impossible BY TYPE inside the store.
func TestReconciliationAdapterIsReadOnlyByStructure(t *testing.T) {
	adapterType := reflect.TypeOf((*port.ReconciliationAdapter)(nil)).Elem()
	mutationNames := map[string]bool{
		"Dispatch": true, "Mutate": true, "Send": true, "Update": true,
		"Delete": true, "Create": true, "Write": true, "Cancel": true, "Compensate": true,
	}
	seen := map[string]bool{}
	for i := 0; i < adapterType.NumMethod(); i++ {
		method := adapterType.Method(i)
		if mutationNames[method.Name] {
			t.Fatalf("мутационное имя метода %s в READ-ONLY порту сверки запрещено (AC-1)", method.Name)
		}
		seen[method.Name] = true
	}
	for _, want := range []string{"ServesCapability", "QueryOutcome"} {
		if !seen[want] {
			t.Fatalf("метод контракта сверки отсутствует: %s", want)
		}
	}
	// The transactional params carry ZERO adapters: the field types of the
	// write transactions hold NO interface at all (the data-only shape is
	// the persist-before-emit discipline itself).
	for _, params := range []reflect.Type{
		reflect.TypeOf(port.AcquireReconciliationLeaseParams{}),
		reflect.TypeOf(port.RecordReconciliationVerdictParams{}),
	} {
		assertNoInterfaceFields(t, params, params.Name(), 0)
	}
}

// assertNoInterfaceFields walks the type recursively and refuses ANY
// interface field — a smuggled adapter port would be one.
func assertNoInterfaceFields(t *testing.T, typ reflect.Type, owner string, depth int) {
	t.Helper()
	if depth > 4 {
		return
	}
	switch typ.Kind() {
	case reflect.Interface:
		t.Fatalf("параметр транзакции %s несёт интерфейсный тип %v — адаптер в транзакции запрещён", owner, typ)
	case reflect.Slice, reflect.Array, reflect.Pointer:
		assertNoInterfaceFields(t, typ.Elem(), owner, depth+1)
	case reflect.Map:
		assertNoInterfaceFields(t, typ.Key(), owner, depth+1)
		assertNoInterfaceFields(t, typ.Elem(), owner, depth+1)
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			assertNoInterfaceFields(t, typ.Field(i).Type, owner, depth+1)
		}
	}
}
