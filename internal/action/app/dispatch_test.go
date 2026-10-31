package app

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/action/port"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.6 step-02: the fake-repo unit matrix of the dispatch worker
// ---------------------------------------------------------------------------
//
// The counters pin the FROZEN fence order of the drain: every fence is a
// NON-SENDING block — the adapter is NEVER called (the adapter counter),
// the lease is NEVER taken (the lease counter) on any gas; the integrity
// alarms (job schema, fingerprint) skip WITHOUT a ledger append (a broken
// machine never touches the world); every other boundary parks the
// operation with the EXACT reason word of the dispatch_blocked entry
// (never a false failed externally, AC-2). The write-ahead lease lands
// BEFORE the ONE adapter call (persist-before-emit, AD-6); the verdict
// transaction is fenced — a stale (attempt, token) pair NEVER writes
// history (AC-1); the decided redelivery verifies by the keyed digest
// (AC-10); the injected failures prove the all-or-nothing verdict
// rollback; the transactional methods of the repository take ZERO ports
// (the reflect probe of the signatures).

// ---------------------------------------------------------------------------
// The fake mutation adapter (the consumer-owned port of the test contour)
// ---------------------------------------------------------------------------

// fakeMutationAdapter is the scriptable adapter of the unit contour: the
// served capability word, the ONE call counter and the scripted answer
// (the synthetic adapter precedent of 3-3 — the call material is pinned
// for the payload/key probes).
type fakeMutationAdapter struct {
	capability string
	calls      int
	seen       []port.MutationRequest
	result     port.MutationResult
	err        error
}

func (f *fakeMutationAdapter) ServesCapability() string { return f.capability }

func (f *fakeMutationAdapter) Dispatch(_ context.Context, _ tenancy.Context, request port.MutationRequest) (port.MutationResult, error) {
	f.calls++
	f.seen = append(f.seen, request)
	if f.err != nil {
		return port.MutationResult{}, f.err
	}
	return f.result, nil
}

// idempotentNoCaps answers the LIVE catalog declaration WITHOUT the
// provider idempotency guarantee (the recoverable crash probe: no
// guarantee — no blind repeat, the honest unknown parks the operation).
type idempotentNoCaps struct{}

func (idempotentNoCaps) Capability(string, string, string) (domain.CapabilityFacts, bool) {
	return domain.CapabilityFacts{Idempotent: false, Reconciliation: "не объявлена"}, true
}

// ---------------------------------------------------------------------------
// The fake dispatch repository (the store's semantics: the advisory-lock
// fencing, the raced refusals, the all-or-nothing commits)
// ---------------------------------------------------------------------------

// fakeLeaseRow is one lease of the fake: the attempt number, the holder
// token, the window and the keyed identities (the raw payload NEVER
// enters — the fake pins the AD-10 discipline by shape).
type fakeLeaseRow struct {
	attemptNo   int
	token       string
	expiresAt   time.Time
	digest      string
	externalKey string
}

// fakeDispatchRepo mirrors the store's THREE transactions: the
// write-ahead lease (the racing refusal on an active lease, the monotonic
// attempt under the bound), the fenced verdict (the (attempt, token)
// pair must own the CURRENT attempt — a stale holder NEVER writes
// history; ANY injected failure records NOTHING) and the raced-guarded
// parking append. The pending jobs are the prepared fixtures (the
// classification is the store's derived fact).
type fakeDispatchRepo struct {
	pendingCalls int
	leaseCalls   int
	outcomeCalls int
	blockedCalls int
	outboxCalls  int

	jobs []port.PendingDispatchJob

	leases        map[id.UUID][]fakeLeaseRow
	outcomes      map[id.UUID]map[int]port.RecordDispatchOutcomeParams
	receipts      map[id.UUID][]port.RecordDispatchOutcomeParams
	receiptEvents []port.ReceiptCreatedPayload
	ledger        map[id.UUID][]domain.OperationLedgerEntry
	blocked       []port.AppendDispatchBlockedParams

	failPending error
	failLease   error
	failOutcome error
	failBlocked error

	// forceNewerAttempt pre-seeds a newer attempt of the operation (the
	// fencing probe: the verdict of the older holder refuses honestly).
	forceNewerAttempt map[id.UUID]int
}

func newFakeDispatchRepo() *fakeDispatchRepo {
	return &fakeDispatchRepo{
		leases:            map[id.UUID][]fakeLeaseRow{},
		outcomes:          map[id.UUID]map[int]port.RecordDispatchOutcomeParams{},
		receipts:          map[id.UUID][]port.RecordDispatchOutcomeParams{},
		ledger:            map[id.UUID][]domain.OperationLedgerEntry{},
		forceNewerAttempt: map[id.UUID]int{},
	}
}

func (f *fakeDispatchRepo) PendingDispatchJobs(_ context.Context, _ tenancy.Context, _ time.Time, _ int) ([]port.PendingDispatchJob, error) {
	f.pendingCalls++
	if f.failPending != nil {
		return nil, f.failPending
	}
	return append([]port.PendingDispatchJob(nil), f.jobs...), nil
}

func (f *fakeDispatchRepo) lastLease(operationID id.UUID) (fakeLeaseRow, bool) {
	rows := f.leases[operationID]
	if len(rows) == 0 {
		return fakeLeaseRow{}, false
	}
	return rows[len(rows)-1], true
}

func (f *fakeDispatchRepo) AcquireDispatchLease(_ context.Context, p port.AcquireDispatchLeaseParams) (int, error) {
	f.leaseCalls++
	if f.failLease != nil {
		return 0, f.failLease
	}
	operationID := p.Operation.OperationID
	if last, ok := f.lastLease(operationID); ok && last.expiresAt.After(p.Now) {
		return 0, port.ErrLeaseRaced
	}
	attemptNo := 1
	if last, ok := f.lastLease(operationID); ok {
		attemptNo = last.attemptNo + 1
	}
	if attemptNo > domain.MaxDispatchAttempts {
		return 0, port.ErrLeaseStale
	}
	f.leases[operationID] = append(f.leases[operationID], fakeLeaseRow{
		attemptNo: attemptNo, token: p.LeaseToken, expiresAt: p.LeaseExpiresAt,
		digest: p.RequestDigest, externalKey: p.ExternalIDempotencyKey,
	})
	f.ledger[operationID] = append(f.ledger[operationID], p.LedgerEntry)
	return attemptNo, nil
}

func (f *fakeDispatchRepo) RecordDispatchOutcome(_ context.Context, p port.RecordDispatchOutcomeParams) error {
	f.outcomeCalls++
	if f.failOutcome != nil {
		// The injected failure: the WHOLE verdict rolls back — nothing of
		// the outcome, the ledger entry, the receipt or the outbox record
		// exists (the all-or-nothing commit of the store).
		return f.failOutcome
	}
	operationID := p.Operation.OperationID
	lease, ok := fakeLeaseOf(f.leases[operationID], p.AttemptNo)
	if !ok || lease.token != p.LeaseToken {
		return port.ErrLeaseStale
	}
	if newer := f.forceNewerAttempt[operationID]; newer > p.AttemptNo {
		return port.ErrLeaseStale
	}
	for attempt := range f.outcomes[operationID] {
		if attempt == p.AttemptNo {
			return port.ErrOutcomeDuplicate
		}
	}
	if p.OutcomeWord != "" {
		if f.outcomes[operationID] == nil {
			f.outcomes[operationID] = map[int]port.RecordDispatchOutcomeParams{}
		}
		f.outcomes[operationID][p.AttemptNo] = p
	}
	f.ledger[operationID] = append(f.ledger[operationID], p.LedgerEntry)
	if p.ReceiptWord != "" {
		f.receipts[operationID] = append(f.receipts[operationID], p)
		f.receiptEvents = append(f.receiptEvents, p.OutboxPayload)
	}
	return nil
}

func fakeLeaseOf(rows []fakeLeaseRow, attemptNo int) (fakeLeaseRow, bool) {
	for _, row := range rows {
		if row.attemptNo == attemptNo {
			return row, true
		}
	}
	return fakeLeaseRow{}, false
}

func (f *fakeDispatchRepo) AppendDispatchBlocked(_ context.Context, p port.AppendDispatchBlockedParams) error {
	f.blockedCalls++
	if f.failBlocked != nil {
		return f.failBlocked
	}
	if last, ok := f.lastLease(p.OperationID); ok && last.expiresAt.After(p.Now) {
		return port.ErrLeaseRaced
	}
	f.ledger[p.OperationID] = append(f.ledger[p.OperationID], p.LedgerEntry)
	f.blocked = append(f.blocked, p)
	return nil
}

func (f *fakeDispatchRepo) AppendOutboxReceiptCreated(_ context.Context, p port.AppendOutboxReceiptCreatedParams) error {
	f.outboxCalls++
	f.receiptEvents = append(f.receiptEvents, p.Payload)
	return nil
}

func (f *fakeDispatchRepo) ReadDispatchAttempts(_ context.Context, _ tenancy.Context, operationID id.UUID) ([]port.DispatchAttemptRow, error) {
	rows := []port.DispatchAttemptRow{}
	for _, lease := range f.leases[operationID] {
		row := port.DispatchAttemptRow{AttemptNo: lease.attemptNo, LeaseToken: lease.token,
			LeaseExpiresAt: lease.expiresAt, RequestDigest: lease.digest}
		if outcome, ok := f.outcomes[operationID][lease.attemptNo]; ok {
			row.OutcomeWord = outcome.OutcomeWord
			row.CorrelationID = outcome.CorrelationID
			row.GuaranteeRecorded = outcome.GuaranteeRecorded
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (f *fakeDispatchRepo) ReadActionReceipts(_ context.Context, _ tenancy.Context, operationID id.UUID) ([]port.ActionReceiptRow, error) {
	rows := []port.ActionReceiptRow{}
	for _, verdict := range f.receipts[operationID] {
		rows = append(rows, port.ActionReceiptRow{ReceiptVersionNo: verdict.ReceiptVersionNo,
			Outcome: verdict.ReceiptWord, References: verdict.ReceiptReferences, Details: verdict.ReceiptDetails})
	}
	return rows, nil
}

// outcomeOf answers the recorded verdict of one attempt (nil when the
// attempt carries no outcome row — the refused or absent claims).
func (f *fakeDispatchRepo) outcomeOf(operationID id.UUID, attemptNo int) (port.RecordDispatchOutcomeParams, bool) {
	outcome, ok := f.outcomes[operationID][attemptNo]
	return outcome, ok
}

// ---------------------------------------------------------------------------
// The dispatch fixture: the authorized operation of the 3-5 flow + the
// fake repository with the prepared fresh job + the scriptable adapter
// ---------------------------------------------------------------------------

// dispatchFixture is the mutable world of one dispatch test: the FULL
// authorization fixture (the preview/consent/operation of the living
// confirm→authorize flow), the fake dispatch repository over the queued
// job and the scriptable mutation adapter.
type dispatchFixture struct {
	auth    *authorizationFixture
	thread  id.UUID
	version int64
	key     string
	op      domain.Operation
	repo    *fakeDispatchRepo
	adapter *fakeMutationAdapter
}

// newDispatchFixture runs the LIVING 3-5 flow (confirm → authorize) and
// prepares the fresh queued job over the immutable operation (the outbox
// payload marshals the SAFE canonical form — the at-least-once delivery
// unit of the drain).
func newDispatchFixture(t *testing.T) *dispatchFixture {
	t.Helper()
	auth := newAuthorizationFixture(t)
	thread, version := auth.confirm(t)
	const key = "dispatch-unit-key-1"
	auth.authorize(t, thread, version, key)
	op, ok := auth.ops.operationOfKey(key)
	if !ok {
		t.Fatal("авторизация обязана оставить операцию в фейковом репозитории")
	}
	rawPayload, err := json.Marshal(auth.ops.outbox[op.OperationID])
	if err != nil {
		t.Fatalf("маршалинг SAFE-полезной нагрузки обязан проходить: %v", err)
	}
	repo := newFakeDispatchRepo()
	repo.jobs = []port.PendingDispatchJob{{
		EventID:        id.MustParse("01994f64-6c00-7000-8000-0000000000d1"),
		RawPayload:     rawPayload,
		Operation:      op,
		Ledger:         append([]domain.OperationLedgerEntry(nil), auth.ops.ledgers[op.OperationID]...),
		StateWord:      domain.OperationAuthorized,
		Classification: port.PendingClassFresh,
	}}
	repo.ledger[op.OperationID] = append([]domain.OperationLedgerEntry(nil), auth.ops.ledgers[op.OperationID]...)
	adapter := &fakeMutationAdapter{capability: auth.def.RequiredCapability.CapabilityID}
	// The gate-order counters start CLEAN at the drain moment (the
	// confirm/authorize flow of the fixture build consumed its reads).
	auth.material.caseCalls = 0
	auth.material.factsCalls = 0
	return &dispatchFixture{auth: auth, thread: thread, version: version, key: key, op: op, repo: repo, adapter: adapter}
}

// drainService wires the execution contour over the SHARED 3-5 fakes
// (the same repositories the authorization wrote into): the operations
// scope gate, the full preview contour and the dispatch wiring with the
// given adapter (nil = the AG-4 production probe) and the given LIVE
// catalog resolver (the recoverable guarantee probe).
func (f *dispatchFixture) drainService(adapter port.MutationAdapter, caps domain.CapabilityRefResolver, reader port.OwningSystemReader) *Service {
	if caps == nil {
		caps = acceptAllCaps{}
	}
	return New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Operations: true}},
		acceptAllPacks{}, caps, f.auth.defs).
		WithPreviews(f.auth.previews, f.auth.world.control, f.auth.material,
			liveMembers{world: &f.auth.world}, fakeEntitlement{status: f.auth.world.admission},
			fakeReadiness{stage: f.auth.world.stage, instances: f.auth.world.instances},
			fakeConnectors{state: f.auth.world.capState, contractVersion: f.auth.world.contract},
			fakeRouteFacts{profileVersion: f.auth.world.profile}, &fakeIDs{}).
		WithPreviewReader(reader).
		WithAuthorization(f.auth.consents, f.auth.ops).
		WithDispatch(f.repo, adapter)
}

// command is the passing drain command of the fixture moment.
func (f *dispatchFixture) command() DispatchDrainCommand {
	return DispatchDrainCommand{Actor: testActor, TenantID: testTenant, Now: f.auth.now}
}

// committedResult is the honest committed answer with the complete
// structural evidence triple.
func committedResult() port.MutationResult {
	return port.MutationResult{
		Outcome:               domain.MutationOutcomeCommitted,
		CorrelationID:         "corr-42",
		ExternalObjectID:      "ext-msg-42",
		ExternalObjectVersion: "3",
		ExternalObjectTime:    time.Date(2026, 10, 20, 12, 0, 30, 0, time.UTC),
		Details:               "квитанция провайдера",
	}
}

// ---------------------------------------------------------------------------
// The AG-4 boundary and the gate order
// ---------------------------------------------------------------------------

// TestDispatchNilAdapterRefusesAG4 — the nil mutation adapter is the
// HONEST production refusal FIRST: the drain command answers
// ErrDispatchRefusedAG4, the queue is never even read (the pending
// counter stays zero — the records stay queued, zero sends).
func TestDispatchNilAdapterRefusesAG4(t *testing.T) {
	f := newDispatchFixture(t)
	service := f.drainService(nil, nil, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if !errors.Is(err, port.ErrDispatchRefusedAG4) {
		t.Fatalf("nil-адаптер обязан отказывать честным AG-4 отказом: %v", err)
	}
	if report.Considered != 0 || report.Dispatched != 0 {
		t.Fatalf("отказ AG-4 не рассматривает очередь: %+v", report)
	}
	if f.repo.pendingCalls != 0 {
		t.Fatalf("очередь не читается до отказа AG-4: pending=%d", f.repo.pendingCalls)
	}
	if f.repo.leaseCalls != 0 || f.adapter.calls != 0 {
		t.Fatalf("отказ AG-4 не берёт lease и не вызывает адаптер: lease=%d adapter=%d", f.repo.leaseCalls, f.adapter.calls)
	}
}

// TestDispatchScopeRefusesWithoutOperationsScope — the drain is the
// operator surface: a member without the operations scope gets the honest
// 403 word BEFORE the queue is read.
func TestDispatchScopeRefusesWithoutOperationsScope(t *testing.T) {
	f := newDispatchFixture(t)
	service := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Review: true}},
		acceptAllPacks{}, acceptAllCaps{}, f.auth.defs).
		WithPreviews(f.auth.previews, f.auth.world.control, f.auth.material,
			liveMembers{world: &f.auth.world}, fakeEntitlement{status: f.auth.world.admission},
			fakeReadiness{stage: f.auth.world.stage, instances: f.auth.world.instances},
			fakeConnectors{state: f.auth.world.capState, contractVersion: f.auth.world.contract},
			fakeRouteFacts{profileVersion: f.auth.world.profile}, &fakeIDs{}).
		WithAuthorization(f.auth.consents, f.auth.ops).
		WithDispatch(f.repo, f.adapter)
	_, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("участник без полномочия operations получает честный 403: %v", err)
	}
	if f.repo.pendingCalls != 0 {
		t.Fatalf("очередь не читается до гейта полномочия: pending=%d", f.repo.pendingCalls)
	}
}

// ---------------------------------------------------------------------------
// The ordered fences: every gas blocks WITHOUT a lease and WITHOUT the
// adapter call (the counters); the integrity alarms skip WITHOUT the
// ledger append
// ---------------------------------------------------------------------------

// fenceProbe is one fence scenario: the mutation of the fixture world
// BEFORE the drain, the expected report shape and the exact reason word
// (empty for the alarm skips — the broken machine never touches the
// ledger).
type fenceProbe struct {
	name       string
	mutate     func(f *dispatchFixture, command *DispatchDrainCommand)
	alarm      bool // the integrity alarm: skip, NO dispatch_blocked append
	reason     string
	want       string // the spoken line of the report
	beforeCase bool   // pin: the fence fires BEFORE the case liveness read
}

// TestDispatchFencesBlockWithoutLeaseOrAdapter — the frozen fence matrix:
// every boundary is the NON-SENDING block — the adapter counter stays
// ZERO, the lease counter stays ZERO, the parking append carries the
// EXACT reason word; the alarms (job schema, fingerprint) skip without
// ANY ledger append. The order counters pin the ladder: the confirmation
// fence fires BEFORE the case read; the material fences never read the
// owning-system rows when an earlier fence answers.
func TestDispatchFencesBlockWithoutLeaseOrAdapter(t *testing.T) {
	probes := []fenceProbe{
		{
			name:  "job-schema alarm",
			alarm: true,
			mutate: func(f *dispatchFixture, _ *DispatchDrainCommand) {
				f.repo.jobs[0].RawPayload = []byte(`{"event_type":"foreign.thing"}`)
			},
			want: dispatchJobSchemaAlarmText,
		},
		{
			name:  "fingerprint alarm",
			alarm: true,
			mutate: func(f *dispatchFixture, _ *DispatchDrainCommand) {
				tampered := f.op
				tampered.MaterialDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
				f.repo.jobs[0].Operation = tampered
			},
			want: dispatchFingerprintAlarmText,
		},
		{
			name: "confirmation superseded",
			mutate: func(f *dispatchFixture, _ *DispatchDrainCommand) {
				f.auth.consents.events = append(f.auth.consents.events, port.ConfirmationEventRow{
					ConfirmationID: f.thread, VersionNo: f.version, Kind: domain.ConfirmationEventSuperseded,
					CreatedAt: f.auth.now,
				})
			},
			reason:     domain.DispatchFenceSuperseded,
			want:       domain.DispatchBlockedText(domain.DispatchFenceSuperseded),
			beforeCase: true,
		},
		{
			name: "confirmation revoked",
			mutate: func(f *dispatchFixture, _ *DispatchDrainCommand) {
				f.auth.consents.events = append(f.auth.consents.events, port.ConfirmationEventRow{
					ConfirmationID: f.thread, VersionNo: f.version, Kind: domain.ConfirmationEventRevoked,
					CreatedAt: f.auth.now,
				})
			},
			reason:     domain.DispatchFenceRevoked,
			want:       domain.DispatchBlockedText(domain.DispatchFenceRevoked),
			beforeCase: true,
		},
		{
			name: "confirmation expired",
			mutate: func(f *dispatchFixture, command *DispatchDrainCommand) {
				command.Now = f.auth.now.Add(2 * time.Hour)
			},
			reason:     domain.DispatchFenceExpired,
			want:       domain.DispatchBlockedText(domain.DispatchFenceExpired),
			beforeCase: true,
		},
		{
			name: "case work item dead",
			mutate: func(f *dispatchFixture, _ *DispatchDrainCommand) {
				f.auth.world.caseView.WorkItems = nil
			},
			reason: dispatchCaseClosedReason,
			want:   domain.DispatchBlockedText(dispatchCaseClosedReason),
		},
		{
			name: "freshness owner epoch moved",
			mutate: func(f *dispatchFixture, _ *DispatchDrainCommand) {
				f.auth.world.caseView.Policy.OwnerEpoch = 2
			},
			reason: domain.FreshnessOwnerEpochMoved,
			want:   domain.DispatchBlockedText(domain.FreshnessOwnerEpochMoved),
		},
		{
			name: "readiness blocked material",
			mutate: func(f *dispatchFixture, _ *DispatchDrainCommand) {
				pinned := f.auth.preview
				pinned.Snapshot = []domain.MaterialSnapshotRow{{
					MaterialField: "price", Status: domain.SnapshotStale,
					ExternalID: "SKU-42", ExternalVersion: "7", ObservedAt: previewNow, EvidenceRef: "catalog://sku-42",
				}}
				f.auth.ops.pinPreview(pinned)
			},
			reason: domain.PreviewBlockedMaterial,
			want:   domain.DispatchBlockedText(domain.PreviewBlockedMaterial),
		},
		{
			name: "policy conversation mode not allowed",
			mutate: func(f *dispatchFixture, _ *DispatchDrainCommand) {
				f.auth.world.caseView.Mode.Active.Mode = "foreign_mode"
			},
			reason: domain.PolicyReasonConversationModeNotAllowed,
			want:   domain.DispatchBlockedText(domain.PolicyReasonConversationModeNotAllowed),
		},
		{
			name: "fresh reads unavailable (nil reader, AG-4)",
			mutate: func(f *dispatchFixture, _ *DispatchDrainCommand) {
				f.auth.world.reader = nil
			},
			reason: dispatchFreshReadsReason,
			want:   domain.DispatchBlockedText(dispatchFreshReadsReason),
		},
		{
			name: "material condition changed (stale row)",
			mutate: func(f *dispatchFixture, _ *DispatchDrainCommand) {
				f.auth.world.reader = &fakeReader{rows: []domain.MaterialSnapshotRow{
					{MaterialField: "price", Status: domain.SnapshotFresh, ExternalID: "SKU-42", ExternalVersion: "7", ObservedAt: previewNow, EvidenceRef: "catalog://sku-42"},
					{MaterialField: "availability", Status: domain.SnapshotStale, ExternalID: "SKU-42", ExternalVersion: "6", ObservedAt: previewNow, EvidenceRef: "catalog://sku-42"},
				}}
			},
			reason: dispatchMaterialChangedReason,
			want:   domain.DispatchBlockedText(dispatchMaterialChangedReason),
		},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			f := newDispatchFixture(t)
			command := f.command()
			probe.mutate(f, &command)
			reader := port.OwningSystemReader(nil)
			if f.auth.world.reader != nil {
				reader = f.auth.world.reader
			}
			service := f.drainService(f.adapter, nil, reader)
			report, err := service.DispatchQueuedOperations(context.Background(), command)
			if err != nil {
				t.Fatalf("фенция обязана паркеровать операцию, а не ломать drain: %v", err)
			}
			if report.Considered != 1 || report.Dispatched != 0 {
				t.Fatalf("задание рассмотрено и НЕ отправлено: %+v", report)
			}
			if probe.alarm {
				if report.Skipped != 1 || report.Blocked != 0 {
					t.Fatalf("alarm обязан быть честным skip без блок-записи: %+v", report)
				}
				if len(f.repo.blocked) != 0 {
					t.Fatalf("alarm неappendит блок-запись: %+v", f.repo.blocked)
				}
			} else {
				if report.Blocked != 1 || report.Skipped != 0 {
					t.Fatalf("фенция обязана паркеровать ровно одной блок-записью: %+v", report)
				}
				if len(f.repo.blocked) != 1 || f.repo.blocked[0].Reason != probe.reason {
					t.Fatalf("блок-запись обязана нести точное слово причины «%s»: %+v", probe.reason, f.repo.blocked)
				}
				if f.repo.blocked[0].LedgerEntry.Kind != domain.LedgerEntryDispatchBlocked {
					t.Fatalf("блок-запись ledger обязана быть dispatch_blocked: %+v", f.repo.blocked[0].LedgerEntry)
				}
			}
			if f.repo.leaseCalls != 0 {
				t.Fatalf("фенция НЕ берёт lease: lease=%d", f.repo.leaseCalls)
			}
			if f.adapter.calls != 0 {
				t.Fatalf("фенция НЕ вызывает адаптер: calls=%d", f.adapter.calls)
			}
			if len(report.Reports) != 1 || report.Reports[0].Text != probe.want {
				t.Fatalf("строка отчёта обязана говорить точное слово: %+v (хотели %q)", report.Reports, probe.want)
			}
			// The order counters: the confirmation fence answers BEFORE the
			// case liveness read; the case answers before the owning-system
			// material read.
			if probe.beforeCase && f.auth.material.caseCalls != 0 {
				t.Fatalf("фенция подтверждения срабатывает ДО чтения кейса: caseCalls=%d", f.auth.material.caseCalls)
			}
			if reader != nil && f.auth.world.reader != nil {
				if calls := f.auth.world.reader.calls; calls != 0 && f.adapter.calls != 0 {
					t.Fatalf("материал читается только на живом пути отправки: calls=%d", calls)
				}
			}
			// The parked operation stays authorized (dispatch_blocked is NOT
			// state-bearing): the ledger of the fake carries the block entry
			// appended AFTER the authorized entry.
			ledger := f.repo.ledger[f.op.OperationID]
			if probe.alarm {
				if len(ledger) != 1 || ledger[0].Kind != domain.LedgerEntryAuthorized {
					t.Fatalf("alarm не трогает ledger: %+v", ledger)
				}
			} else if len(ledger) != 2 || ledger[0].Kind != domain.LedgerEntryAuthorized || ledger[1].Kind != domain.LedgerEntryDispatchBlocked {
				t.Fatalf("ledger обязан расти authorized → dispatch_blocked: %+v", ledger)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The send: the write-ahead lease, the ONE adapter call, the fenced
// verdict, the receipt v1 and the outbox record
// ---------------------------------------------------------------------------

// TestDispatchHappyPathCommittedWithReceipt — the full living send: the
// lease lands BEFORE the adapter call (the write-ahead counter order),
// the adapter is called EXACTLY ONCE with the ONE external idempotency
// key and the keyed request digest, the fenced verdict writes the outcome
// row of the honest claim, the ledger ladder authorized → submitted →
// committed, the receipt v1 {committed} and the outbox receipt.created
// record — ONE verdict transaction.
func TestDispatchHappyPathCommittedWithReceipt(t *testing.T) {
	f := newDispatchFixture(t)
	f.adapter.result = committedResult()
	service := f.drainService(f.adapter, nil, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("живой путь обязан проходить: %v", err)
	}
	if report.Considered != 1 || report.Dispatched != 1 || report.Blocked != 0 || report.Skipped != 0 {
		t.Fatalf("отчёт обязан говорить об одной отправке: %+v", report)
	}
	if f.repo.leaseCalls != 1 || f.adapter.calls != 1 {
		t.Fatalf("ровно одна lease и ровно один вызов адаптера: lease=%d calls=%d", f.repo.leaseCalls, f.adapter.calls)
	}
	// The write-ahead lease: attempt 1 with the derived identities.
	lease, ok := fakeLeaseOf(f.repo.leases[f.op.OperationID], 1)
	if !ok {
		t.Fatal("lease попытки 1 обязана существовать")
	}
	if len(lease.token) != 64 || lease.digest == "" || lease.externalKey == "" {
		t.Fatalf("lease обязана нести 64-hex токен и keyed-идентичности: %+v", lease)
	}
	// The ONE adapter call material: the ONE external key of the
	// operation (stable across attempts) and the keyed digest.
	if len(f.adapter.seen) != 1 {
		t.Fatalf("адаптер вызван ровно один раз: %d", len(f.adapter.seen))
	}
	seen := f.adapter.seen[0]
	if seen.AttemptNo != 1 || seen.OperationID != f.op.OperationID {
		t.Fatalf("вызов адаптера несёт идентичность операции и попытки: %+v", seen)
	}
	if seen.ExternalIDempotencyKey != lease.externalKey {
		t.Fatalf("внешний ключ попытки обязан совпадать с lease: %s ≠ %s", seen.ExternalIDempotencyKey, lease.externalKey)
	}
	if seen.RequestDigest != lease.digest {
		t.Fatalf("digest запроса обязан совпадать с lease: %s ≠ %s", seen.RequestDigest, lease.digest)
	}
	if seen.Payload.DefinitionID != f.op.DefinitionID {
		t.Fatalf("пересобранный payload несёт пинны определения: %+v", seen.Payload)
	}
	// The verdict: the outcome row of the honest committed claim.
	outcome, ok := f.repo.outcomeOf(f.op.OperationID, 1)
	if !ok {
		t.Fatal("исход попытки 1 обязан быть записан")
	}
	if outcome.OutcomeWord != domain.MutationOutcomeCommitted {
		t.Fatalf("записанное слово исхода — committed: %+v", outcome)
	}
	if outcome.CorrelationID != "corr-42" || outcome.Evidence.ExternalObjectID != "ext-msg-42" ||
		outcome.Evidence.ExternalObjectVersion != "3" || outcome.Evidence.ExternalObjectTime.IsZero() {
		t.Fatalf("исход обязан нести структурное evidence и корреляцию: %+v", outcome)
	}
	// The ledger ladder: authorized → submitted → committed.
	ledger := f.repo.ledger[f.op.OperationID]
	if len(ledger) != 3 || ledger[0].Kind != domain.LedgerEntryAuthorized ||
		ledger[1].Kind != domain.LedgerEntrySubmitted || ledger[2].Kind != domain.LedgerEntryCommitted {
		t.Fatalf("ledger-лестница authorized → submitted → committed: %+v", ledger)
	}
	// The receipt v1 {committed} with the SAFE references.
	if len(f.repo.receipts[f.op.OperationID]) != 1 {
		t.Fatalf("ровно одна квитанция v1: %+v", f.repo.receipts)
	}
	receipt := f.repo.receipts[f.op.OperationID][0]
	if receipt.ReceiptWord != domain.ReceiptOutcomeCommitted || receipt.ReceiptVersionNo != 1 {
		t.Fatalf("квитанция v1 обязана говорить committed: %+v", receipt)
	}
	if receipt.ReceiptReferences.CaseID != f.op.CaseID || receipt.ReceiptReferences.WorkItemID != f.op.WorkItemID ||
		receipt.ReceiptReferences.PreviewID != f.op.PreviewID || receipt.ReceiptReferences.ConfirmationID != f.op.ConfirmationID ||
		receipt.ReceiptReferences.ExternalObjectID != "ext-msg-42" {
		t.Fatalf("квитанция несёт SAFE-пинны решения: %+v", receipt.ReceiptReferences)
	}
	// The outbox receipt.created record (AD-2: the link rides THIS
	// module's own outbox).
	if len(f.repo.receiptEvents) != 1 {
		t.Fatalf("ровно одно событие receipt.created: %+v", f.repo.receiptEvents)
	}
	event := f.repo.receiptEvents[0]
	if event.EventType != domain.OutboxEventReceiptCreated || event.OperationID != f.op.OperationID ||
		event.ReceiptVersionNo != 1 || event.Outcome != domain.ReceiptOutcomeCommitted {
		t.Fatalf("событие receipt.created обязано зеркалить квитанцию: %+v", event)
	}
}

// TestDispatchAcceptedCarriesNoReceipt — accepted is the NON-terminal
// reception: the state moves to accepted, NO receipt, NO receipt.created
// record (the business result is not confirmed yet).
func TestDispatchAcceptedCarriesNoReceipt(t *testing.T) {
	f := newDispatchFixture(t)
	f.adapter.result = port.MutationResult{Outcome: domain.MutationOutcomeAccepted, CorrelationID: "corr-7"}
	service := f.drainService(f.adapter, nil, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("accepted — честный не-терминальный исход: %v", err)
	}
	if report.Dispatched != 1 {
		t.Fatalf("accepted считается отправкой: %+v", report)
	}
	outcome, ok := f.repo.outcomeOf(f.op.OperationID, 1)
	if !ok || outcome.OutcomeWord != domain.MutationOutcomeAccepted {
		t.Fatalf("исход accepted обязан быть записан: %+v", outcome)
	}
	if len(f.repo.receipts[f.op.OperationID]) != 0 || len(f.repo.receiptEvents) != 0 {
		t.Fatalf("accepted НЕ несёт квитанции и события: %+v / %+v", f.repo.receipts, f.repo.receiptEvents)
	}
	ledger := f.repo.ledger[f.op.OperationID]
	if len(ledger) != 3 || ledger[2].Kind != domain.LedgerEntryAccepted {
		t.Fatalf("ledger завершается accepted: %+v", ledger)
	}
}

// TestDispatchFailedIsTerminalWithReceipt — failed is the terminal
// refusal: the receipt v1 {failed} lands, no auto-repeat.
func TestDispatchFailedIsTerminalWithReceipt(t *testing.T) {
	f := newDispatchFixture(t)
	f.adapter.result = port.MutationResult{Outcome: domain.MutationOutcomeFailed, Details: "провайдер отказал"}
	service := f.drainService(f.adapter, nil, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("failed — честный терминальный исход: %v", err)
	}
	if report.Dispatched != 1 {
		t.Fatalf("попытка состоялась и записана: %+v", report)
	}
	if len(f.repo.receipts[f.op.OperationID]) != 1 ||
		f.repo.receipts[f.op.OperationID][0].ReceiptWord != domain.ReceiptOutcomeFailed {
		t.Fatalf("квитанция v1 {failed} обязательна: %+v", f.repo.receipts)
	}
	if len(f.repo.receiptEvents) != 1 || f.repo.receiptEvents[0].Outcome != domain.ReceiptOutcomeFailed {
		t.Fatalf("событие receipt.created {failed} обязательно: %+v", f.repo.receiptEvents)
	}
}

// TestDispatchCallErrorRecordsUnknownWithoutGuarantee — the adapter call
// error is the honest unknown WITHOUT a guarantee: NO outcome row (there
// is no honest claim), the guarded ledger kind unknown, the receipt v1
// {unknown} and the outbox record.
func TestDispatchCallErrorRecordsUnknownWithoutGuarantee(t *testing.T) {
	f := newDispatchFixture(t)
	f.adapter.err = errors.New("провайдер недоступен")
	service := f.drainService(f.adapter, nil, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("ошибка вызова — честный unknown, а не сбой drain: %v", err)
	}
	if report.Dispatched != 1 {
		t.Fatalf("попытка состояла и записана: %+v", report)
	}
	if _, ok := f.repo.outcomeOf(f.op.OperationID, 1); ok {
		t.Fatalf("ошибка вызова НЕ записывает исход: %+v", f.repo.outcomes)
	}
	if len(f.repo.receipts[f.op.OperationID]) != 1 ||
		f.repo.receipts[f.op.OperationID][0].ReceiptWord != domain.ReceiptOutcomeUnknown {
		t.Fatalf("квитанция v1 {unknown} обязательна: %+v", f.repo.receipts)
	}
	verdict := f.repo.receipts[f.op.OperationID][0]
	if verdict.GuaranteeRecorded {
		t.Fatalf("без честного timeout-заявления гарантия НЕ записывается: %+v", verdict)
	}
	ledger := f.repo.ledger[f.op.OperationID]
	if len(ledger) != 3 || ledger[2].Kind != domain.LedgerEntryUnknown {
		t.Fatalf("ledger завершается unknown: %+v", ledger)
	}
}

// TestDispatchTimeoutRecordsGuaranteeForRetry — the honest timeout claim
// records the LIVE guarantee word (the audit fact of the NEXT retry
// policy): the outcome row timeout with guarantee_recorded=true.
func TestDispatchTimeoutRecordsGuaranteeForRetry(t *testing.T) {
	f := newDispatchFixture(t)
	f.adapter.result = port.MutationResult{Outcome: domain.MutationOutcomeTimeout, GuaranteeLive: true}
	service := f.drainService(f.adapter, nil, f.auth.world.reader)
	if _, err := service.DispatchQueuedOperations(context.Background(), f.command()); err != nil {
		t.Fatalf("timeout — честный unknown с гарантией: %v", err)
	}
	outcome, ok := f.repo.outcomeOf(f.op.OperationID, 1)
	if !ok || outcome.OutcomeWord != domain.MutationOutcomeTimeout {
		t.Fatalf("исход timeout обязан быть записан: %+v", outcome)
	}
	if !outcome.GuaranteeRecorded {
		t.Fatalf("LIVE-гарантия адаптера записывается для retry-политики: %+v", outcome)
	}
	if len(f.repo.receipts[f.op.OperationID]) != 1 ||
		f.repo.receipts[f.op.OperationID][0].ReceiptWord != domain.ReceiptOutcomeUnknown {
		t.Fatalf("timeout несёт квитанцию v1 {unknown}: %+v", f.repo.receipts)
	}
}

// TestDispatchCommittedClaimWithoutEvidenceRefusedToUnknown — the
// terminal success is NEVER taken on word: a committed claim without the
// complete external object triple records NO outcome row; the ledger and
// the receipt speak the guarded unknown.
func TestDispatchCommittedClaimWithoutEvidenceRefusedToUnknown(t *testing.T) {
	f := newDispatchFixture(t)
	f.adapter.result = port.MutationResult{Outcome: domain.MutationOutcomeCommitted, CorrelationID: "corr-9"}
	service := f.drainService(f.adapter, nil, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("отказ evidence — guarded unknown, не сбой drain: %v", err)
	}
	if report.Dispatched != 1 {
		t.Fatalf("попытка записана guarded-исходом: %+v", report)
	}
	if _, ok := f.repo.outcomeOf(f.op.OperationID, 1); ok {
		t.Fatalf("заявление без evidence НЕ записывает исход: %+v", f.repo.outcomes)
	}
	if len(f.repo.receipts[f.op.OperationID]) != 1 ||
		f.repo.receipts[f.op.OperationID][0].ReceiptWord != domain.ReceiptOutcomeUnknown {
		t.Fatalf("квитанция v1 {unknown} — отказ evidence: %+v", f.repo.receipts)
	}
	ledger := f.repo.ledger[f.op.OperationID]
	if len(ledger) != 3 || ledger[2].Kind != domain.LedgerEntryUnknown {
		t.Fatalf("ledger завершается guarded unknown: %+v", ledger)
	}
}

// ---------------------------------------------------------------------------
// The fencing: a stale lease NEVER writes history
// ---------------------------------------------------------------------------

// TestDispatchStaleLeaseNeverWritesHistory — the verdict of an older
// holder refuses honestly: a newer attempt of the operation makes the
// (attempt, token) pair stale — NO outcome, NO receipt, NO outbox record
// (the history stays the winner's); the report speaks the fenced refusal.
func TestDispatchStaleLeaseNeverWritesHistory(t *testing.T) {
	f := newDispatchFixture(t)
	f.adapter.result = committedResult()
	f.repo.forceNewerAttempt[f.op.OperationID] = 2
	service := f.drainService(f.adapter, nil, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("fencing-отказ — честный skip, не сбой drain: %v", err)
	}
	if report.Skipped != 1 || report.Dispatched != 0 {
		t.Fatalf("устаревшая lease получает честный skip: %+v", report)
	}
	if len(report.Reports) != 1 || report.Reports[0].Text != dispatchFencedRefusalText {
		t.Fatalf("строка отчёта обязана говорить fencing-отказ: %+v", report.Reports)
	}
	if _, ok := f.repo.outcomeOf(f.op.OperationID, 1); ok {
		t.Fatalf("устаревшая lease НЕ пишет исход: %+v", f.repo.outcomes)
	}
	if len(f.repo.receipts[f.op.OperationID]) != 0 || len(f.repo.receiptEvents) != 0 {
		t.Fatalf("устаревшая lease НЕ пишет квитанцию и событие: %+v / %+v", f.repo.receipts, f.repo.receiptEvents)
	}
	// The write-ahead lease and the adapter call DID happen (they are
	// committed facts); the HISTORY is what the fencing refuses.
	if f.repo.leaseCalls != 1 || f.adapter.calls != 1 {
		t.Fatalf("write-ahead lease и вызов адаптера — факты: lease=%d calls=%d", f.repo.leaseCalls, f.adapter.calls)
	}
	ledger := f.repo.ledger[f.op.OperationID]
	if len(ledger) != 2 || ledger[1].Kind != domain.LedgerEntrySubmitted {
		t.Fatalf("ledger хранит только write-ahead факт: %+v", ledger)
	}
}

// ---------------------------------------------------------------------------
// The redelivery: the decided verification by the keyed digest (AC-10)
// ---------------------------------------------------------------------------

// TestDispatchDecidedReplayVerifiedByDigest — the at-least-once
// redelivery of a DECIDED operation: the rebuilt digest MATCHES the
// recorded trace — the honest skip (no new send, no new intent); a
// DIVERGENCE — the integrity alarm (the corruption is never masked). The
// adapter is never called, the lease is never taken.
func TestDispatchDecidedReplayVerifiedByDigest(t *testing.T) {
	f := newDispatchFixture(t)
	// The decided job: the ledger ladder authorized → submitted →
	// committed, the recorded digest of the first attempt.
	payload, err := domain.BuildAuthorizedActionPayload(f.op, f.auth.preview, f.auth.def)
	if err != nil {
		t.Fatalf("пересборка payload обязана проходить: %v", err)
	}
	recordedDigest := domain.DeriveDispatchRequestDigest(f.auth.previews.key, payload)
	decidedLedger := []domain.OperationLedgerEntry{
		{EntryNo: 1, Kind: domain.LedgerEntryAuthorized, Details: "авторизовано"},
		{EntryNo: 2, Kind: domain.LedgerEntrySubmitted, Details: "write-ahead"},
		{EntryNo: 3, Kind: domain.LedgerEntryCommitted, Details: "исход записан"},
	}
	f.repo.jobs[0].Ledger = decidedLedger
	f.repo.jobs[0].StateWord = domain.OperationCommitted
	f.repo.jobs[0].Classification = port.PendingClassDecided
	f.repo.jobs[0].LastAttemptNo = 1
	f.repo.jobs[0].RequestDigest = recordedDigest
	f.repo.ledger[f.op.OperationID] = append([]domain.OperationLedgerEntry(nil), decidedLedger...)
	service := f.drainService(f.adapter, nil, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("повтор решённой операции — честный skip: %v", err)
	}
	if report.Skipped != 1 || report.Dispatched != 0 {
		t.Fatalf("совпавший digest — честный skip повтора: %+v", report)
	}
	if len(report.Reports) != 1 || report.Reports[0].Text != domain.DispatchRedeliveryDecidedText {
		t.Fatalf("строка отчёта обязана говорить повтор решённой операции: %+v", report.Reports)
	}
	if f.repo.leaseCalls != 0 || f.adapter.calls != 0 {
		t.Fatalf("повтор НЕ берёт lease и НЕ вызывает адаптер: lease=%d calls=%d", f.repo.leaseCalls, f.adapter.calls)
	}
	// The divergence: the recorded trace no longer matches the rebuild —
	// the integrity alarm, never a masked send.
	f2 := newDispatchFixture(t)
	payload2, err := domain.BuildAuthorizedActionPayload(f2.op, f2.auth.preview, f2.auth.def)
	if err != nil {
		t.Fatalf("пересборка payload обязана проходить: %v", err)
	}
	f2.repo.jobs[0].Ledger = append([]domain.OperationLedgerEntry(nil), decidedLedger...)
	f2.repo.jobs[0].StateWord = domain.OperationCommitted
	f2.repo.jobs[0].Classification = port.PendingClassDecided
	f2.repo.jobs[0].LastAttemptNo = 1
	f2.repo.jobs[0].RequestDigest = domain.DeriveDispatchRequestDigest([]byte("другой-ключ"), payload2)
	service2 := f2.drainService(f2.adapter, nil, f2.auth.world.reader)
	report2, err := service2.DispatchQueuedOperations(context.Background(), f2.command())
	if err != nil {
		t.Fatalf("расхождение digest — alarm skip, не сбой drain: %v", err)
	}
	if report2.Skipped != 1 || report2.Dispatched != 0 {
		t.Fatalf("расхождение digest — alarm skip: %+v", report2)
	}
	if len(report2.Reports) != 1 || report2.Reports[0].Text != dispatchReplayAlarmText {
		t.Fatalf("строка отчёта обязана говорить alarm расхождения: %+v", report2.Reports)
	}
	if f2.repo.leaseCalls != 0 || f2.adapter.calls != 0 {
		t.Fatalf("alarm НЕ берёт lease и НЕ вызывает адаптер: lease=%d calls=%d", f2.repo.leaseCalls, f2.adapter.calls)
	}
}

// ---------------------------------------------------------------------------
// The retry policy: the RECORDED guarantee decides (belt over braces)
// ---------------------------------------------------------------------------

// TestDispatchRetryableReadsRecordedGuarantee — the recorded unknown
// verdict retries ONLY with the recorded guarantee word in the bounds:
// without the guarantee the park skip answers; with the guarantee the
// fences re-run and the SECOND attempt takes the lease (the same
// external key — the provider deduplicates).
func TestDispatchRetryableReadsRecordedGuarantee(t *testing.T) {
	f := newDispatchFixture(t)
	retryableLedger := []domain.OperationLedgerEntry{
		{EntryNo: 1, Kind: domain.LedgerEntryAuthorized, Details: "авторизовано"},
		{EntryNo: 2, Kind: domain.LedgerEntrySubmitted, Details: "write-ahead"},
		{EntryNo: 3, Kind: domain.LedgerEntryUnknown, Details: "исход неизвестен"},
	}
	f.repo.jobs[0].Ledger = retryableLedger
	f.repo.jobs[0].StateWord = domain.OperationUnknown
	f.repo.jobs[0].Classification = port.PendingClassRetryable
	f.repo.jobs[0].LastAttemptNo = 1
	f.repo.jobs[0].LastOutcomeGuarantee = false
	f.repo.ledger[f.op.OperationID] = append([]domain.OperationLedgerEntry(nil), retryableLedger...)
	service := f.drainService(f.adapter, nil, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("retryable без гарантии — park skip: %v", err)
	}
	if report.Skipped != 1 || report.Dispatched != 0 {
		t.Fatalf("без записанной гарантии повтора нет: %+v", report)
	}
	if f.repo.leaseCalls != 0 || f.adapter.calls != 0 {
		t.Fatalf("слепой повтор запрещён: lease=%d calls=%d", f.repo.leaseCalls, f.adapter.calls)
	}
	// With the recorded guarantee: the retry re-runs the fences and takes
	// the SECOND attempt of the SAME operation.
	f2 := newDispatchFixture(t)
	f2.repo.jobs[0].Ledger = append([]domain.OperationLedgerEntry(nil), retryableLedger...)
	f2.repo.jobs[0].StateWord = domain.OperationUnknown
	f2.repo.jobs[0].Classification = port.PendingClassRetryable
	f2.repo.jobs[0].LastAttemptNo = 1
	f2.repo.jobs[0].LastOutcomeGuarantee = true
	f2.repo.ledger[f2.op.OperationID] = append([]domain.OperationLedgerEntry(nil), retryableLedger...)
	// The prior attempt's lease (expired) with the recorded key.
	priorKey := domain.DeriveExternalIdempotencyKey(f2.auth.previews.key, testTenant, f2.op.OperationID)
	f2.repo.leases[f2.op.OperationID] = []fakeLeaseRow{{
		attemptNo: 1, token: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		expiresAt: f2.auth.now.Add(-time.Minute), digest: "digest-1", externalKey: priorKey,
	}}
	f2.adapter.result = committedResult()
	service2 := f2.drainService(f2.adapter, nil, f2.auth.world.reader)
	report2, err := service2.DispatchQueuedOperations(context.Background(), f2.command())
	if err != nil {
		t.Fatalf("retry с записанной гарантией обязан проходить фенции: %v", err)
	}
	if report2.Dispatched != 1 {
		t.Fatalf("вторая попытка отправлена: %+v", report2)
	}
	if len(f2.repo.leases[f2.op.OperationID]) != 2 {
		t.Fatalf("вторая lease обязана существовать: %+v", f2.repo.leases)
	}
	second := f2.repo.leases[f2.op.OperationID][1]
	if second.attemptNo != 2 || second.externalKey != priorKey {
		t.Fatalf("вторая попытка несёт ТОТ ЖЕ внешний ключ (провайдер дедупит): %+v", second)
	}
}

// TestDispatchRecoverableCrashParksUnknownWithoutLiveGuarantee — the
// crash resting state (submitted, the expired lease, no outcome): the
// LIVE catalog declaration decides — WITHOUT the idempotency guarantee
// the honest unknown verdict lands ON THE CRASHED attempt (no outcome
// row, no new attempt — «возможен предыдущий вызов — слепой повтор
// запрещён»), the receipt v1 {unknown} speaks the boundary.
func TestDispatchRecoverableCrashParksUnknownWithoutLiveGuarantee(t *testing.T) {
	f := newDispatchFixture(t)
	recoverableLedger := []domain.OperationLedgerEntry{
		{EntryNo: 1, Kind: domain.LedgerEntryAuthorized, Details: "авторизовано"},
		{EntryNo: 2, Kind: domain.LedgerEntrySubmitted, Details: "write-ahead"},
	}
	f.repo.jobs[0].Ledger = recoverableLedger
	f.repo.jobs[0].StateWord = domain.OperationSubmitted
	f.repo.jobs[0].Classification = port.PendingClassRecoverable
	f.repo.jobs[0].LastAttemptNo = 1
	f.repo.jobs[0].LastLeaseToken = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	f.repo.jobs[0].LastLeaseExpiresAt = f.auth.now.Add(-time.Minute)
	f.repo.jobs[0].RequestDigest = "digest-1"
	f.repo.ledger[f.op.OperationID] = append([]domain.OperationLedgerEntry(nil), recoverableLedger...)
	f.repo.leases[f.op.OperationID] = []fakeLeaseRow{{
		attemptNo: 1, token: f.repo.jobs[0].LastLeaseToken,
		expiresAt: f.auth.now.Add(-time.Minute), digest: "digest-1", externalKey: "key-1",
	}}
	// The LIVE catalog declaration WITHOUT the guarantee.
	service := f.drainService(f.adapter, idempotentNoCaps{}, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("крах без гарантии — честная парковка unknown: %v", err)
	}
	if report.Blocked != 1 || report.Dispatched != 0 {
		t.Fatalf("операция припаркована unknown: %+v", report)
	}
	if len(report.Reports) != 1 || report.Reports[0].Text != domain.DispatchBlindRepeatForbiddenText {
		t.Fatalf("строка отчёта обязана говорить запрет слепого повтора: %+v", report.Reports)
	}
	if f.adapter.calls != 0 {
		t.Fatalf("слепой повтор запрещён — адаптер НЕ вызван: calls=%d", f.adapter.calls)
	}
	if len(f.repo.leases[f.op.OperationID]) != 1 {
		t.Fatalf("новая попытка НЕ создаётся: %+v", f.repo.leases)
	}
	if _, ok := f.repo.outcomeOf(f.op.OperationID, 1); ok {
		t.Fatalf("без honest-заявления исход НЕ записывается: %+v", f.repo.outcomes)
	}
	if len(f.repo.receipts[f.op.OperationID]) != 1 ||
		f.repo.receipts[f.op.OperationID][0].ReceiptWord != domain.ReceiptOutcomeUnknown {
		t.Fatalf("квитанция v1 {unknown} на крахнутой попытке: %+v", f.repo.receipts)
	}
	if len(f.repo.receiptEvents) != 1 {
		t.Fatalf("событие receipt.created {unknown} обязательно: %+v", f.repo.receiptEvents)
	}
}

// ---------------------------------------------------------------------------
// The all-or-nothing verdict: the injected failure records NOTHING
// ---------------------------------------------------------------------------

// TestDispatchVerdictRollbackProbe — the injected failure of the verdict
// transaction aborts the drain honestly: NO outcome, NO receipt, NO
// outbox record exist after the failure (the fake mirrors the store's
// all-or-nothing commit); the write-ahead lease and the adapter call are
// the committed facts that stay.
func TestDispatchVerdictRollbackProbe(t *testing.T) {
	f := newDispatchFixture(t)
	f.adapter.result = committedResult()
	f.repo.failOutcome = errors.New("инжект сбоя: verdict-транзакция")
	service := f.drainService(f.adapter, nil, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err == nil {
		t.Fatal("инжект сбоя обязан прервать drain честной ошибкой")
	}
	if report.Dispatched != 0 {
		t.Fatalf("отчёт не считает незафиксированный вердикт: %+v", report)
	}
	if f.repo.leaseCalls != 1 || f.adapter.calls != 1 {
		t.Fatalf("write-ahead факт и вызов адаптера состоялись: lease=%d calls=%d", f.repo.leaseCalls, f.adapter.calls)
	}
	if len(f.repo.outcomes[f.op.OperationID]) != 0 {
		t.Fatalf("откат вердикта не оставляет исхода: %+v", f.repo.outcomes)
	}
	if len(f.repo.receipts[f.op.OperationID]) != 0 || len(f.repo.receiptEvents) != 0 {
		t.Fatalf("откат вердикта не оставляет квитанции и события: %+v / %+v", f.repo.receipts, f.repo.receiptEvents)
	}
}

// TestDispatchBlockedAppendFailureAbortsDrain — the injected failure of
// the parking append aborts the drain honestly (a fence append is a
// write too — the report never counts an unwritten block).
func TestDispatchBlockedAppendFailureAbortsDrain(t *testing.T) {
	f := newDispatchFixture(t)
	f.auth.consents.events = append(f.auth.consents.events, port.ConfirmationEventRow{
		ConfirmationID: f.thread, VersionNo: f.version, Kind: domain.ConfirmationEventRevoked,
		CreatedAt: f.auth.now,
	})
	f.repo.failBlocked = errors.New("инжект сбоя: блок-запись")
	service := f.drainService(f.adapter, nil, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err == nil {
		t.Fatal("инжект сбоя блок-записи обязан прервать drain")
	}
	if report.Blocked != 0 {
		t.Fatalf("незаписанная блок-запись не считается: %+v", report)
	}
	if f.adapter.calls != 0 || f.repo.leaseCalls != 0 {
		t.Fatalf("адаптер и lease не тронуты: calls=%d lease=%d", f.adapter.calls, f.repo.leaseCalls)
	}
}

// ---------------------------------------------------------------------------
// The signature probe: the transactional methods take ZERO ports
// ---------------------------------------------------------------------------

// TestDispatchRepositoryTransactionsTakeNoAdapterPorts — the
// persist-before-emit discipline is the SIGNATURE itself: the four
// transactional methods of the dispatch repository (the write-ahead
// lease, the fenced verdict, the raced-guarded parking, the standalone
// outbox append) accept NO port parameter — a network call is impossible
// BY TYPE inside the store (the Enqueue/ReserveOperation precedents).
func TestDispatchRepositoryTransactionsTakeNoAdapterPorts(t *testing.T) {
	repoType := reflect.TypeOf((*port.DispatchRepository)(nil)).Elem()
	transactional := map[string]bool{
		"AcquireDispatchLease":       true,
		"RecordDispatchOutcome":      true,
		"AppendDispatchBlocked":      true,
		"AppendOutboxReceiptCreated": true,
		"PendingDispatchJobs":        false,
		"ReadDispatchAttempts":       false,
		"ReadActionReceipts":         false,
	}
	seen := map[string]bool{}
	for i := 0; i < repoType.NumMethod(); i++ {
		method := repoType.Method(i)
		transactional, known := transactional[method.Name]
		if !known {
			t.Fatalf("неизвестный метод контракта DispatchRepository: %s", method.Name)
		}
		seen[method.Name] = true
		if !transactional {
			continue
		}
		methodType := method.Type
		for j := 0; j < methodType.NumIn(); j++ {
			param := methodType.In(j)
			// The ambient context is the ONLY interface a transaction may
			// take — every OTHER interface parameter would be a smuggled
			// port (the adapter above all).
			if param.Kind() == reflect.Interface && param != reflect.TypeOf((*context.Context)(nil)).Elem() {
				t.Fatalf("транзакционный метод %s принимает интерфейсный параметр %v — порт в транзакции запрещён",
					method.Name, param)
			}
			if param == reflect.TypeOf((*port.MutationAdapter)(nil)) {
				t.Fatalf("транзакционный метод %s принимает адаптер — запрещено", method.Name)
			}
		}
	}
	for name := range transactional {
		if !seen[name] {
			t.Fatalf("метод контракта отсутствует: %s", name)
		}
	}
}

// TestDispatchReportSpeaksHonestWords — the drain report carries the
// command word and the spoken line of every job (FR-28: the operator
// sees the stage and the cause).
func TestDispatchReportSpeaksHonestWords(t *testing.T) {
	f := newDispatchFixture(t)
	f.adapter.result = committedResult()
	service := f.drainService(f.adapter, nil, f.auth.world.reader)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("живой путь обязан проходить: %v", err)
	}
	if report.Command != "dispatch_queued_operations" {
		t.Fatalf("отчёт несёт слово команды: %q", report.Command)
	}
	if len(report.Reports) != 1 || report.Reports[0].OperationID != f.op.OperationID || report.Reports[0].AttemptNo != 1 {
		t.Fatalf("строка отчёта несёт операцию и попытку: %+v", report.Reports)
	}
	if report.Reports[0].Text == "" {
		t.Fatalf("строка отчёта обязана говорить честное слово: %+v", report.Reports)
	}
}
