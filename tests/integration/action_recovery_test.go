//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	actionStore "github.com/artemiosu/staffy/internal/action/store"
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
