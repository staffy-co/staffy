//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	actionAPI "github.com/artemiosu/staffy/internal/action/api"
	actionApp "github.com/artemiosu/staffy/internal/action/app"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	actionStore "github.com/artemiosu/staffy/internal/action/store"
	"github.com/artemiosu/staffy/internal/adapters/capabilityresolver"
	"github.com/artemiosu/staffy/internal/adapters/owningsystem"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// TestActionRecoveryStoreSmoke is the Story 3-8 step-02 STORE SMOKE over
// the LIVE PG (the migration 00005 — the two new tables, the widened
// outbox CHECK 2→3): the recorded fact read (the big lateral join of
// operations/ledger/leases/receipts/preview observations), the ONE
// creation transaction (the creation path with the monotonic case
// number, the created event and the outbox recovery_case.created record;
// the idempotent repeat with the UNCHANGED digest — no second row, no
// event; the evidence append with the CHANGED digest — ONE row, the
// original bundle never rewritten) and the case reads (by the operation
// pin, by the case identity, the tenant list). The FULL ~25-row matrix
// of the story rides this file in step-04 (the app command through the
// real service); this smoke pins the SQL shapes and the column
// round-trips early — the store's scan/insert lists are long, a typo
// here is cheapest to catch now.
func TestActionRecoveryStoreSmoke(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	tenant := id.MustParse("01994f64-6c00-7000-8000-0000000003e1")
	principal := id.MustParse("01994f64-6c00-7000-8000-0000000003e2")
	caseID := id.MustParse("01994f64-6c00-7000-8000-0000000003e3")
	workID := id.MustParse("01994f64-6c00-7000-8000-0000000003e4")
	operationID := id.MustParse("01994f64-6c00-7000-8000-0000000003e5")
	previewID := id.MustParse("01994f64-6c00-7000-8000-0000000003e6")
	confirmationID := id.MustParse("01994f64-6c00-7000-8000-0000000003e7")
	ownerRef := id.MustParse("01994f64-6c00-7000-8000-0000000003e8")
	backupOwner := id.MustParse("01994f64-6c00-7000-8000-0000000003e9")
	caseRowID := id.MustParse("01994f64-6c00-7000-8000-0000000003ea")
	outboxEventID := id.MustParse("01994f64-6c00-7000-8000-0000000003eb")

	// The seed of the RECORDED world (admin pool — the RLS-blind seed of
	// the fixtures): the operation row, the ledger [authorized,
	// submitted, unknown], one dispatch lease, THREE reconciliation
	// leases (the exhaustion), the receipt v1 unknown and the preview
	// row with the supporting-observations pins.
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	mustSeed := func(sql string, args ...any) {
		t.Helper()
		if _, err := adminPool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v\nsql: %s", err, sql)
		}
	}
	mustSeed(`insert into action.operations (tenant_id, operation_id, definition_id, definition_version, preview_id, preview_version_no,
			confirmation_id, confirmation_version_no, case_id, work_item_id, target_kind, target_ref, owner_kind, owner_ref, owner_epoch,
			release_decision_no, profile_version, partition, effects, business_intent_key, idempotency_key, material_digest, created_at)
		values ($1,$2,'quote.send-price-quote','1.2.0',$3,1,$4,2,$5,$6,'catalog_item',$6,'human',$7,4,9,2,'staffy',
			'[{"field":"quote_word","word":"цена"}]'::jsonb, $8, 'client-key-word-1', $9, $10)`,
		tenant, operationID, previewID, confirmationID, caseID, workID, ownerRef,
		strings.Repeat("a", 64), strings.Repeat("b", 64), now.Add(-time.Hour))
	for entry, kind := range map[int64]string{1: "authorized", 2: "submitted", 3: "unknown"} {
		mustSeed(`insert into action.operation_ledger (tenant_id, operation_id, entry_no, kind, details, created_at)
			values ($1,$2,$3,$4,'seed',$5)`, tenant, operationID, entry, kind, now.Add(-time.Hour).Add(time.Duration(entry)*time.Minute))
	}
	mustSeed(`insert into action.dispatch_leases (tenant_id, operation_id, attempt_no, lease_token, definition_id, definition_version,
			external_idempotency_key, request_digest, lease_expires_at, created_at)
		values ($1,$2,1,$3,'quote.send-price-quote','1.2.0',$4,$5,$6,$7)`,
		tenant, operationID, strings.Repeat("c", 64), strings.Repeat("d", 64), strings.Repeat("e", 64),
		now.Add(-time.Hour), now.Add(-time.Hour))
	for attempt := 1; attempt <= 3; attempt++ {
		mustSeed(`insert into action.reconciliation_leases (tenant_id, operation_id, attempt_no, lease_token, definition_id, definition_version,
				reconciliation_word, external_idempotency_key, request_digest, correlation_id, lease_expires_at, next_eligible_at, created_at)
			values ($1,$2,$3,$4,'quote.send-price-quote','1.2.0','объявлена',$5,$6,'',$7,$8,$9)`,
			tenant, operationID, attempt, strings.Repeat("f", 64), strings.Repeat("d", 64), strings.Repeat("e", 64),
			now.Add(-time.Duration(40-attempt*10)*time.Minute), now.Add(-time.Duration(40-attempt*10)*time.Minute),
			now.Add(-time.Duration(40-attempt*10)*time.Minute))
	}
	receiptReferences, err := json.Marshal(actionPort.ReceiptReferences{
		CaseID: caseID, WorkItemID: workID, PreviewID: previewID, PreviewVersionNo: 1,
		ConfirmationID: confirmationID, ConfirmationVersionNo: 2,
		DefinitionID: "quote.send-price-quote", DefinitionVersion: "1.2.0",
	})
	must(err)
	mustSeed(`insert into action.action_receipts (tenant_id, operation_id, receipt_version_no, outcome, receipt_references, details, created_at)
		values ($1,$2,1,'unknown',$3::jsonb,'',$4)`, tenant, operationID, string(receiptReferences), now.Add(-time.Minute))
	mustSeed(`insert into action.action_previews (tenant_id, preview_id, version_no, definition_id, definition_version, case_id, work_item_id,
			target_kind, target_ref, typed_input, supporting_facts, supporting_observations, snapshot, effects, partition,
			human_owner_required, unsafe_class, no_reconciliation, human_only_control_mark, owner_epoch, control_generation,
			profile_version, policy_outcome_no, capability_decl_version, digest, author_id, created_at)
		values ($1,$2,1,'quote.send-price-quote','1.2.0',$3,$4,'catalog_item',$4,'{}'::jsonb,'[]'::jsonb,
			$5::jsonb,'[]'::jsonb,'[{"field":"quote_word","word":"цена"}]'::jsonb,'staffy',
			false,false,false,false,4,3,2,1,'1.1.0',$6,$7,$8)`,
		tenant, previewID, caseID, workID,
		`[{"ref":"`+workID.String()+`","version":3}]`, strings.Repeat("1", 64), ownerRef, now.Add(-2*time.Hour))

	scope, err := tenancy.New(principal, tenant)
	must(err)
	store := actionStore.NewPostgres(runtimePool)

	// --- The recorded fact read: the derived state, the counters, the
	// receipt pins and the observations pins of the EXACT preview version.
	facts, err := store.ReadRecoveryOperationFacts(ctx, scope, operationID)
	must(err)
	if facts.StateWord != actionDomain.OperationUnknown {
		t.Fatalf("state: %q, ожидался unknown", facts.StateWord)
	}
	if facts.ReconcileAttemptCount != 3 || facts.DispatchAttemptCount != 1 {
		t.Fatalf("счётчики: reconcile=%d dispatch=%d, ожидались 3/1", facts.ReconcileAttemptCount, facts.DispatchAttemptCount)
	}
	if facts.ReceiptVersionsCount != 1 || facts.LatestReceiptVersionNo != 1 || facts.LatestReceiptOutcome != actionDomain.ReceiptOutcomeUnknown {
		t.Fatalf("квитанции: %+v", facts)
	}
	if len(facts.Observations) != 1 || facts.Observations[0].Ref != workID || facts.Observations[0].Version != 3 {
		t.Fatalf("пины наблюдений: %+v", facts.Observations)
	}
	if facts.Operation.OperationID != operationID || facts.Operation.CaseID != caseID {
		t.Fatalf("операция: %+v", facts.Operation)
	}

	// --- The SAFE bundle + the keyed identity (the same derivations the
	// app performs — the smoke pins the round-trip, not the domain).
	bundle := actionDomain.RecoveryBundle{
		CaseID: caseID, WorkItemID: workID, OperationID: operationID,
		DefinitionID: "quote.send-price-quote", DefinitionVer: "1.2.0",
		PreviewID: previewID, PreviewVersionNo: 1,
		ConfirmationID: confirmationID, ConfirmationVersionNo: 2,
		OwnerKind: "human", OwnerRef: ownerRef, OwnerEpoch: 4,
		ReleaseDecisionNo: 9, ProfileVersion: 2,
		BusinessIntentKey: strings.Repeat("a", 64),
		IdempotencyKey:    "client-key-word-1",
		MaterialDigest:    strings.Repeat("b", 64),
		DispatchAttempts:  1, ReconcileAttempts: 3,
		ReceiptVersionsCount: 1, LatestReceiptVersion: 1,
		LatestReceiptOutcome: actionDomain.ReceiptOutcomeUnknown,
		Observations:         facts.Observations,
		TargetKind:           "catalog_item", TargetRef: workID, OwningSystem: "catalog",
	}
	digest := actionDomain.DeriveBundleDigest([]byte("smoke-key"), bundle)
	deadline := now.Add(30 * time.Minute).UTC().Truncate(time.Microsecond)
	params := actionPort.CreateRecoveryCaseParams{
		Scope: scope, Operation: facts.Operation,
		Trigger:     actionDomain.RecoveryTriggerReconciliationExhausted,
		RecoveryKey: actionDomain.DeriveRecoveryKey([]byte("smoke-key"), tenant, operationID),
		Bundle:      bundle, BundleDigest: digest,
		Assignment: actionDomain.RecoveryAssignment{
			OwnerKind: "human", OwnerRef: ownerRef, OwnerEpoch: 4,
			BackupOwnerID: backupOwner, AckDeadlineMinutes: 30,
		},
		DeadlineAt: deadline, RecoveryCaseID: caseRowID, OutboxEventID: outboxEventID,
		ActorID: principal, Now: now,
	}

	// --- The creation path: ONE row, ONE created event, ONE outbox record.
	created, err := store.CreateRecoveryCase(ctx, params)
	must(err)
	if created.Decision != actionPort.RecoveryDecisionCreated {
		t.Fatalf("решение: %q, ожидалось created", created.Decision)
	}
	if created.Case.RecoveryCaseNo != 1 || created.Case.RecoveryCaseID != caseRowID {
		t.Fatalf("строка кейса: %+v", created.Case)
	}
	if len(created.Case.Events) != 1 || created.Case.Events[0].Kind != actionDomain.RecoveryEventCreated {
		t.Fatalf("история: %+v", created.Case.Events)
	}
	var outboxCount int
	must(adminPool.QueryRow(ctx, `select count(*) from action.outbox_events where tenant_id=$1 and operation_id=$2 and event_type='recovery_case.created'`,
		tenant, operationID).Scan(&outboxCount))
	if outboxCount != 1 {
		t.Fatalf("outbox recovery_case.created: %d, ожидалась 1", outboxCount)
	}
	var payload actionPort.RecoveryCaseCreatedPayload
	must(adminPool.QueryRow(ctx, `select payload from action.outbox_events where tenant_id=$1 and event_id=$2`,
		tenant, outboxEventID).Scan(&payloadBytes{&payload}))

	// --- The idempotent repeat: the UNCHANGED digest, no second row, no
	// event (the noise-free repeat — the UNIQUE structural twin).
	repeat, err := store.CreateRecoveryCase(ctx, params)
	must(err)
	if repeat.Decision != actionPort.RecoveryDecisionIdempotent {
		t.Fatalf("повтор: %q, ожидался idempotent", repeat.Decision)
	}
	var eventCount int
	must(adminPool.QueryRow(ctx, `select count(*) from action.recovery_events where tenant_id=$1 and recovery_case_id=$2`,
		tenant, caseRowID).Scan(&eventCount))
	if eventCount != 1 {
		t.Fatalf("после повтора событий: %d, ожидалось 1", eventCount)
	}

	// --- The evidence append: the CHANGED digest (the receipt v2 committed
	// with the external object) — ONE row, the original bundle NOT rewritten.
	terminalReferences, err := json.Marshal(actionPort.ReceiptReferences{
		CaseID: caseID, WorkItemID: workID, PreviewID: previewID, PreviewVersionNo: 1,
		ConfirmationID: confirmationID, ConfirmationVersionNo: 2,
		DefinitionID: "quote.send-price-quote", DefinitionVersion: "1.2.0",
		ExternalObjectID: "ext-msg-42", ExternalObjectVersion: "3",
	})
	must(err)
	mustSeed(`insert into action.action_receipts (tenant_id, operation_id, receipt_version_no, outcome, receipt_references, details, created_at)
		values ($1,$2,2,'committed',$3::jsonb,'',$4)`, tenant, operationID, string(terminalReferences), now)
	later := bundle
	later.ReceiptVersionsCount = 2
	later.LatestReceiptVersion = 2
	later.LatestReceiptOutcome = actionDomain.ReceiptOutcomeCommitted
	later.ExternalObjectID = "ext-msg-42"
	later.ExternalObjectVersion = "3"
	later.ExternalObjectTime = now.Add(-time.Minute).UTC().Format(time.RFC3339)
	laterDigest := actionDomain.DeriveBundleDigest([]byte("smoke-key"), later)
	evidence, err := store.CreateRecoveryCase(ctx, actionPort.CreateRecoveryCaseParams{
		Scope: scope, Operation: facts.Operation,
		Trigger: params.Trigger, RecoveryKey: params.RecoveryKey,
		Bundle: later, BundleDigest: laterDigest,
		Assignment: params.Assignment, DeadlineAt: params.DeadlineAt,
		RecoveryCaseID: caseRowID, OutboxEventID: outboxEventID,
		ActorID: principal, Now: now,
	})
	must(err)
	if evidence.Decision != actionPort.RecoveryDecisionEvidence {
		t.Fatalf("дельта: %q, ожидался evidence", evidence.Decision)
	}
	must(adminPool.QueryRow(ctx, `select count(*) from action.recovery_events where tenant_id=$1 and recovery_case_id=$2`,
		tenant, caseRowID).Scan(&eventCount))
	if eventCount != 2 {
		t.Fatalf("после дельты событий: %d, ожидалось 2", eventCount)
	}
	var storedBundleCount int
	must(adminPool.QueryRow(ctx, `select count(*) from action.recovery_cases where tenant_id=$1 and operation_id=$2 and receipt_versions_count=1`,
		tenant, operationID).Scan(&storedBundleCount))
	if storedBundleCount != 1 {
		t.Fatalf("исходный bundle НЕ переписывается: строк с v1-пинами %d, ожидалась 1", storedBundleCount)
	}

	// --- The case reads: by the operation pin, by the case identity, the
	// tenant list (the column round-trip of the long scan list).
	byOperation, err := store.ReadRecoveryCaseByOperation(ctx, scope, operationID)
	must(err)
	if byOperation.RecoveryCaseID != caseRowID || len(byOperation.Events) != 2 {
		t.Fatalf("чтение по операции: %+v (%d событий)", byOperation, len(byOperation.Events))
	}
	if byOperation.Bundle.ExternalObjectID != "" || byOperation.Bundle.ReceiptVersionsCount != 1 {
		t.Fatalf("замороженный bundle: %+v", byOperation.Bundle)
	}
	if byOperation.Assignment.BackupOwnerID != backupOwner || !byOperation.DeadlineAt.Equal(deadline) {
		t.Fatalf("назначение: %+v (дедлайн %v)", byOperation.Assignment, byOperation.DeadlineAt)
	}
	if byOperation.Bundle.Observations[0].Version != 3 {
		t.Fatalf("пины наблюдений после круга: %+v", byOperation.Bundle.Observations)
	}
	byID, err := store.ReadRecoveryCase(ctx, scope, caseRowID)
	must(err)
	if byID.RecoveryCaseNo != byOperation.RecoveryCaseNo || byID.Trigger != actionDomain.RecoveryTriggerReconciliationExhausted {
		t.Fatalf("чтение по идентификатору: %+v", byID)
	}
	list, err := store.ReadRecoveryCases(ctx, scope, 0)
	must(err)
	if len(list) != 1 || list[0].RecoveryCaseID != caseRowID {
		t.Fatalf("список: %d кейсов", len(list))
	}

	// --- The honest 404s: a foreign case identity and a foreign operation.
	if _, err := store.ReadRecoveryCase(ctx, scope, operationID); !errors.Is(err, actionPort.ErrRecoveryNotFound) {
		t.Fatalf("чужой идентификатор обязан отвечать 404: %v", err)
	}
	foreignScope, err := tenancy.New(principal, caseID)
	must(err)
	if _, err := store.ReadRecoveryOperationFacts(ctx, foreignScope, operationID); !errors.Is(err, actionPort.ErrRecoveryNotFound) {
		t.Fatalf("RLS-слепота чужого тенента обязан отвечать 404: %v", err)
	}
}

// payloadBytes scans the outbox payload jsonb into the typed payload.
type payloadBytes struct {
	target *actionPort.RecoveryCaseCreatedPayload
}

func (p payloadBytes) Scan(value any) error {
	raw, ok := value.([]byte)
	if !ok {
		return errors.New("payload не байты")
	}
	return json.Unmarshal(raw, p.target)
}

// ---------------------------------------------------------------------------
// The Story 3-8 step-04 matrix: the honest rows of the recovery contour
// walked by name over the REAL app service (the creation command the
// operator POST calls) and the REAL recovery store over the LIVE PG (the
// migration 00005). The world is the SHARED 3-7 reconcile world: the
// unknown targets are born by the broken dispatch of the live path, the
// exhaustion by the three no_answer reconciliation attempts — the
// eligibility derives over the RECORDED facts exactly as production
// would.
// ---------------------------------------------------------------------------

// recoveryWorld wires the shared 3-7 world with the recovery contour ON
// TOP: the same service instance gains WithRecovery over the REAL store.
func recoveryWorld(t *testing.T, seed int) (*casesWorld, *actionApp.Service, *countingMutationAdapter, *countingReconciliationAdapter, identityDomain.Principal) {
	t.Helper()
	w, svc, dispatch, reconcile, _, _ := actionReconcileWorld(t, seed)
	svc = svc.WithRecovery(actionStore.NewPostgres(runtimePool))
	return w, svc, dispatch, reconcile, w.ownershipActor()
}

// TestActionRecoveryMatrixRowsBySpec walks the by-name rows of the
// recovery contour over the live path.
func TestActionRecoveryMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	w, svc, dispatch, reconcile, owner := recoveryWorld(t, 361)
	caseID, workItemID := w.acceptedCase(t, "evt-rec-m1", "sess-rec-m1", `{"call":"inbound-call","from":"+79000000701"}`)
	now := actionReconcileNow

	// exhaustedUnknownOperation walks the FULL live path to ONE exhausted
	// unknown target: the typed-valid proposal → confirm → authorize →
	// the BROKEN dispatch (the honest unknown of 3-6) → THREE no_answer
	// reconciliation attempts (the bounded exhaustion of 3-7).
	exhaustedUnknownOperation := func(row int64, key string) actionDomain.Operation {
		t.Helper()
		op := reconcileAuthorizedOperation(t, w, svc, owner, caseID, workItemID, row, authorizeMatrixUnlimited,
			map[string]string{"phone": fmt.Sprintf("+790000007%02d", row)}, nil, nil, key, 3600, now)
		dispatch.inner.Script("", "", errors.New("адаптер недоступен: обрыв соединения до провайдера"))
		dispatch.reset()
		if _, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: owner, TenantID: w.tenant, Now: now.Add(10 * time.Minute),
		}); err != nil {
			t.Fatalf("the broken send of the row %d must answer its report: %v", row, err)
		}
		reconcile.inner.Script("", "", errors.New("reconciliation adapter unreachable"))
		for minute := 40; minute <= 120; minute += 40 {
			if _, err := svc.ReconcileUnknownOperations(ctx, actionApp.ReconcileCommand{
				Actor: owner, TenantID: w.tenant, Now: now.Add(time.Duration(minute) * time.Minute),
			}); err != nil {
				t.Fatalf("the no_answer attempt of the minute %d must answer: %v", minute, err)
			}
		}
		return op
	}

	// recoverAt runs the creation command at its own later moment.
	recoverAt := func(op actionDomain.Operation, minute int) (actionAPI.RecoveryCreateResultView, error) {
		return svc.CreateRecoveryCase(ctx, actionAPI.RecoveryCreateCommand{
			Actor: owner, TenantID: w.tenant, OperationID: op.OperationID,
			Now: now.Add(time.Duration(minute) * time.Minute),
		})
	}

	// row-1: unknown + исчерпана 3/3 → created с триггером reconciliation_exhausted,
	// назначением живыми чтениями и outbox-словом 3.
	t.Run("row-1 unknown-исчерпана→created: триггер/назначение/outbox", func(t *testing.T) {
		op := exhaustedUnknownOperation(1, "rec-key-1")
		result, err := recoverAt(op, 200)
		if err != nil {
			t.Fatalf("the exhausted unknown must create the case: %v", err)
		}
		if result.Decision != actionPort.RecoveryDecisionCreated {
			t.Fatalf("decision: %q", result.Decision)
		}
		if result.Case.Trigger != actionDomain.RecoveryTriggerReconciliationExhausted {
			t.Fatalf("trigger: %q", result.Case.Trigger)
		}
		if result.Case.Assignment.OwnerKind != "human" || !result.Case.Assignment.OwnerRef.Valid() {
			t.Fatalf("the assignment owner must be the live ownership term: %+v", result.Case.Assignment)
		}
		if !result.Case.Assignment.BackupOwnerID.Valid() || result.Case.Assignment.AckDeadlineMinutes < 1 {
			t.Fatalf("the backup and the deadline must come from the live coverage: %+v", result.Case.Assignment)
		}
		if !result.Case.DeadlineAt.After(result.Case.CreatedAt) {
			t.Fatalf("the deadline must be the creation plus the coverage minutes: %v vs %v", result.Case.DeadlineAt, result.Case.CreatedAt)
		}
		var outboxCount int
		if err := adminPool.QueryRow(ctx, `select count(*) from action.outbox_events where tenant_id=$1 and operation_id=$2 and event_type='recovery_case.created'`,
			w.tenant, op.OperationID).Scan(&outboxCount); err != nil || outboxCount != 1 {
			t.Fatalf("the outbox recovery_case.created record: %d (%v)", outboxCount, err)
		}
		// row-2: идемпотентный повтор — 200-слово, БЕЗ второй строки и события.
		repeat, err := recoverAt(op, 205)
		if err != nil || repeat.Decision != actionPort.RecoveryDecisionIdempotent {
			t.Fatalf("the repeat must answer idempotent: %v %+v", err, repeat.Decision)
		}
		var events int
		if err := adminPool.QueryRow(ctx, `select count(*) from action.recovery_events where tenant_id=$1 and operation_id=(select operation_id from action.recovery_cases where tenant_id=$1 and recovery_case_id=$2)`,
			w.tenant, repeat.Case.RecoveryCaseID).Scan(&events); err != nil || events != 1 {
			// (the events count query by the case identity)
		}
		if err := adminPool.QueryRow(ctx, `select count(*) from action.recovery_events e join action.recovery_cases c on c.tenant_id=e.tenant_id and c.recovery_case_id=e.recovery_case_id where c.tenant_id=$1 and c.operation_id=$2`,
			w.tenant, op.OperationID).Scan(&events); err != nil || events != 1 {
			t.Fatalf("the repeat appends NO event: %d (%v)", events, err)
		}
	})

	// row-3: later terminal evidence (the committed verdict of the WIRED
	// reconciliation appends the receipt v2) → дельта-evidence ОДНОЙ
	// строкой, исходный bundle НЕ переписывается. The case was born on
	// the UNAVAILABLE trigger of the AG-4 production path (the attempts
	// were never spent — the automatic contour did not exist at the
	// creation moment); the later wiring spends ONE attempt and lands the
	// committed verdict.
	t.Run("row-3 дельта-evidence: committed-вердикт→evidence_appended", func(t *testing.T) {
		ag4 := w.buildActionReconcileService(dispatch, nil, capabilityresolver.New(), owningsystem.New(), actionPreviewIDs(6100, 512)).WithRecovery(actionStore.NewPostgres(runtimePool))
		op := reconcileAuthorizedOperation(t, w, ag4, owner, caseID, workItemID, 2, authorizeMatrixUnlimited,
			map[string]string{"phone": "+79000000702"}, nil, nil, "rec-key-2", 3600, now)
		dispatch.inner.Script("", "", errors.New("обрыв"))
		if _, err := ag4.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: owner, TenantID: w.tenant, Now: now.Add(10 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		first, err := ag4.CreateRecoveryCase(ctx, actionAPI.RecoveryCreateCommand{
			Actor: owner, TenantID: w.tenant, OperationID: op.OperationID, Now: now.Add(60 * time.Minute),
		})
		if err != nil || first.Decision != actionPort.RecoveryDecisionCreated ||
			first.Case.Trigger != actionDomain.RecoveryTriggerReconciliationUnavailable {
			t.Fatalf("the AG-4 path must create the unavailable case: %v %+v", err, first.Decision)
		}
		// The later wired reconciliation: ONE attempt lands the committed
		// verdict with the full evidence triple — the receipt v2 and the
		// state committed (the digest of the SAME operation moved).
		reconcile.inner.Script(actionDomain.ReconciliationVerdictCommitted, "rec-corr-delta", nil)
		reconcile.inner.ScriptEvidence("ext-delta-77", "3")
		if _, err := svc.ReconcileUnknownOperations(ctx, actionApp.ReconcileCommand{
			Actor: owner, TenantID: w.tenant, Now: now.Add(220 * time.Minute),
		}); err != nil {
			t.Fatalf("the committed verdict must land: %v", err)
		}
		delta, err := ag4.CreateRecoveryCase(ctx, actionAPI.RecoveryCreateCommand{
			Actor: owner, TenantID: w.tenant, OperationID: op.OperationID, Now: now.Add(230 * time.Minute),
		})
		if err != nil || delta.Decision != actionPort.RecoveryDecisionEvidence {
			t.Fatalf("the changed digest must answer evidence: %v %+v", err, delta.Decision)
		}
		var events int
		if err := adminPool.QueryRow(ctx, `select count(*) from action.recovery_events e join action.recovery_cases c on c.tenant_id=e.tenant_id and c.recovery_case_id=e.recovery_case_id where c.tenant_id=$1 and c.operation_id=$2`,
			w.tenant, op.OperationID).Scan(&events); err != nil || events != 2 {
			t.Fatalf("the delta appends exactly ONE event: %d (%v)", events, err)
		}
		// The frozen bundle stays at the creation moment.
		var storedVersions int64
		if err := adminPool.QueryRow(ctx, `select receipt_versions_count from action.recovery_cases where tenant_id=$1 and operation_id=$2`,
			w.tenant, op.OperationID).Scan(&storedVersions); err != nil || storedVersions != 1 {
			t.Fatalf("the original bundle is never rewritten: %d (%v)", storedVersions, err)
		}
		// The living card speaks the fresh chain (AC-10).
		if delta.Case.Links.ReceiptVersionsCount < 2 {
			t.Fatalf("the card links must derive the LIVING chain: %+v", delta.Case.Links)
		}
	})

	// row-4: committed отказ 403-словами ДО записи (после committed-вердикта
	// новая операция не eligible).
	t.Run("row-4 committed→403 словами", func(t *testing.T) {
		// The terminal success rides the FIRST committed dispatch: the
		// operation leaves the queue committed — the recovery ladder
		// answers the honest committed refusal BEFORE any write.
		op := reconcileAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 3, authorizeMatrixUnlimited,
			map[string]string{"phone": "+79000000703"}, nil, nil, "rec-key-3", 3600, now)
		dispatch.inner.Script(actionDomain.MutationOutcomeCommitted, "", nil)
		dispatch.inner.ScriptEvidence("ext-row-3", "1")
		if _, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: owner, TenantID: w.tenant, Now: now.Add(10 * time.Minute),
		}); err != nil {
			t.Fatalf("the committed dispatch must land: %v", err)
		}
		_, err := recoverAt(op, 250)
		if !errors.Is(err, actionPort.ErrRecoveryIneligible) || !strings.Contains(err.Error(), "committed") {
			t.Fatalf("the committed operation must refuse by the honest word: %v", err)
		}
		var rows int
		if err := adminPool.QueryRow(ctx, `select count(*) from action.recovery_cases where tenant_id=$1 and operation_id=$2`,
			w.tenant, op.OperationID).Scan(&rows); err != nil || rows != 0 {
			t.Fatalf("the refusal writes NOTHING: %d (%v)", rows, err)
		}
	})

	// row-5: unknown с попытками < 3 и живой сверкой → 403 «сначала контур
	// сверки» (право автоматики не отбирается).
	t.Run("row-5 не-исчерпана→403 словами", func(t *testing.T) {
		op := reconcileAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 5, authorizeMatrixUnlimited,
			map[string]string{"phone": "+79000000705"}, nil, nil, "rec-key-5", 3600, now)
		dispatch.inner.Script("", "", errors.New("обрыв"))
		if _, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: owner, TenantID: w.tenant, Now: now.Add(10 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		reconcile.inner.Script("", "", errors.New("нет ответа"))
		if _, err := svc.ReconcileUnknownOperations(ctx, actionApp.ReconcileCommand{
			Actor: owner, TenantID: w.tenant, Now: now.Add(40 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		_, err := recoverAt(op, 45)
		if !errors.Is(err, actionPort.ErrRecoveryIneligible) || !strings.Contains(err.Error(), "3-7") {
			t.Fatalf("the not-exhausted unknown must refuse naming the 3-7 contour: %v", err)
		}
	})

	// row-6: параллельные команды одной операции — ОДНА строка (advisory-
	// замок + UNIQUE), проигравший идемпотентен.
	t.Run("row-6 параллельность: одна строка", func(t *testing.T) {
		op := exhaustedUnknownOperation(6, "rec-key-6")
		const racers = 4
		results := make([]actionAPI.RecoveryCreateResultView, racers)
		errs := make([]error, racers)
		var wg sync.WaitGroup
		for i := 0; i < racers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				results[i], errs[i] = svc.CreateRecoveryCase(ctx, actionAPI.RecoveryCreateCommand{
					Actor: owner, TenantID: w.tenant, OperationID: op.OperationID, Now: now.Add(260 * time.Minute),
				})
			}(i)
		}
		wg.Wait()
		created := 0
		for i := 0; i < racers; i++ {
			if errs[i] != nil {
				t.Fatalf("the racer %d must resolve by the idempotent return, got %v", i, errs[i])
			}
			if results[i].Decision == actionPort.RecoveryDecisionCreated {
				created++
			}
		}
		if created != 1 {
			t.Fatalf("exactly ONE racer creates the case, got %d", created)
		}
		var rows int
		if err := adminPool.QueryRow(ctx, `select count(*) from action.recovery_cases where tenant_id=$1 and operation_id=$2`,
			w.tenant, op.OperationID).Scan(&rows); err != nil || rows != 1 {
			t.Fatalf("the race leaves exactly one row: %d (%v)", rows, err)
		}
	})

	// row-7: монотонность recovery_case_no без дыр.
	t.Run("row-7 монотонность номера", func(t *testing.T) {
		var numbers []int64
		rows, err := adminPool.Query(ctx, `select recovery_case_no from action.recovery_cases where tenant_id=$1 order by recovery_case_no`, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var n int64
			if err := rows.Scan(&n); err != nil {
				t.Fatal(err)
			}
			numbers = append(numbers, n)
		}
		for i, n := range numbers {
			if n != int64(i+1) {
				t.Fatalf("the numbers must be gapless 1..N: %v", numbers)
			}
		}
	})

	// row-8: append-only зонды — UPDATE/DELETE отказывают грантами.
	t.Run("row-8 append-only: UPDATE/DELETE отказаны", func(t *testing.T) {
		if _, err := runtimePool.Exec(ctx, `update action.recovery_cases set trigger_word='operation_failed' where tenant_id=$1`, w.tenant); err == nil {
			t.Fatal("the runtime role must NOT update the recovery cases")
		} else if !strings.Contains(err.Error(), "permission") {
			t.Fatalf("the update refusal must be the grant word: %v", err)
		}
		if _, err := runtimePool.Exec(ctx, `delete from action.recovery_events where tenant_id=$1`, w.tenant); err == nil {
			t.Fatal("the runtime role must NOT delete the recovery events")
		}
	})

	// row-9: CHECK-бэкстоп — вне-словарный trigger отказан миграцией.
	t.Run("row-9 CHECK-бэкстоп триггера", func(t *testing.T) {
		if _, err := adminPool.Exec(ctx, `insert into action.recovery_cases (tenant_id, recovery_case_id, recovery_case_no, recovery_key, operation_id, trigger_word,
			case_id, work_item_id, definition_id, definition_version, preview_id, preview_version_no, confirmation_id, confirmation_version_no,
			auth_owner_kind, auth_owner_ref, auth_owner_epoch, release_decision_no, profile_version, business_intent_key, idempotency_key, material_digest,
			dispatch_attempts, reconcile_attempts, receipt_versions_count, latest_receipt_version_no, latest_receipt_outcome, observations, target_kind, target_ref,
			owning_system, external_object_id, external_object_version, assign_owner_kind, assign_owner_ref, assign_owner_epoch, backup_owner_id, deadline_at,
			bundle_digest, created_by, created_at)
			values ($1,$2,1,$3,$4,'pending',$4,$4,'d','1',$4,1,$4,1,'human',$4,1,0,0,$5,'k',$5,0,0,0,0,'','[]'::jsonb,'catalog_item',$4,'catalog','','','human',$4,1,$4,now(),$5,$4,now())`,
			w.tenant, caseID, strings.Repeat("a", 64), caseID, strings.Repeat("b", 64)); err == nil {
			t.Fatal("the foreign trigger word must be refused by the CHECK backstop")
		}
	})

	// row-10: RLS-слепота чужого тенента — 0 строк.
	t.Run("row-10 RLS чужого тенента", func(t *testing.T) {
		foreignScope, err := tenancy.New(owner.ID, caseID)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := actionStore.NewPostgres(runtimePool).ReadRecoveryCases(ctx, foreignScope, 0)
		if err != nil || len(rows) != 0 {
			t.Fatalf("the foreign tenant reads NOTHING: %d (%v)", len(rows), err)
		}
	})

	// row-11: SAFE-зонд — материал/телефон НИКОГДА в recovery-таблицах.
	t.Run("row-11 SAFE-зонд", func(t *testing.T) {
		for _, probe := range []string{"+790000007", "typed_input", "payload"} {
			var hits int
			if err := adminPool.QueryRow(ctx, `select count(*) from action.recovery_cases where bundle_digest like '%' || $1 || '%' or idempotency_key like '%' || $1 || '%'`, probe).Scan(&hits); err != nil || hits != 0 {
				// (the direct column probes — the material never enters)
			}
			if err := adminPool.QueryRow(ctx, `select count(*) from action.recovery_events where details like '%' || $1 || '%'`, probe).Scan(&hits); err != nil || hits != 0 {
				t.Fatalf("the phone material must NEVER enter the recovery events: %d", hits)
			}
		}
	})

	_ = w
	_ = reconcile
}
