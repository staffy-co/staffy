//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionApp "github.com/staffy-co/staffy/internal/action/app"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	actionStore "github.com/staffy-co/staffy/internal/action/store"
	"github.com/staffy-co/staffy/internal/adapters/actiondispatcher"
	"github.com/staffy-co/staffy/internal/adapters/capabilityresolver"
	"github.com/staffy-co/staffy/internal/adapters/connectorrefresolver"
	"github.com/staffy-co/staffy/internal/adapters/controleffectiveresolver"
	"github.com/staffy-co/staffy/internal/adapters/owningsystem"
	"github.com/staffy-co/staffy/internal/adapters/packresolver"
	"github.com/staffy-co/staffy/internal/adapters/reconciliationsyn"
	controlAPI "github.com/staffy-co/staffy/internal/control/api"
	controlApp "github.com/staffy-co/staffy/internal/control/app"
	controlDomain "github.com/staffy-co/staffy/internal/control/domain"
	controlStore "github.com/staffy-co/staffy/internal/control/store"
	identityApp "github.com/staffy-co/staffy/internal/identity/app"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	identityStore "github.com/staffy-co/staffy/internal/identity/store"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// TestActionReconciliationMatrixRowsBySpec is the Story 3-7 step-04
// matrix: the honest rows of the reconciliation contour walked by name
// over the REAL app service (the light read-only fence ladder of
// ReconcileUnknownOperations — the worker the operator reconcile command
// calls), the REAL reconciliation store over the LIVE PG (the migration
// 00004 — RLS+FORCE on the two tables, the CHECK dictionaries, the
// append-only INSERT/SELECT grants, the advisory lock of the write-ahead
// lease and the fenced verdict transactions) and the synthetic READ-ONLY
// adapter of the test contour (scriptable verdict/correlation/evidence/
// failure — the journeys fixture precedent, imported ONLY by the test
// composition roots; the production binary wires none, AG-4).
//
// The rows pin the AC probes BY NAME: the full live path
// unknown→reconcile→committed with the receipt v2 append and the outbox
// receipt.created record carrying the version in its payload (v1
// byte-stable, the versions monotonic); the no_effect pair {ledger
// failed, receipt failed} under the DECLARED absence semantics and the
// retry-unlock drain (the new dispatch attempt of the SAME operation
// with the SAME external key and digest — no new intent, no new
// reservation); the no_effect claim WITHOUT the declared semantics read
// no_answer (the bare-404 discipline of AC-3 — nothing is written, the
// retry does NOT unlock); the in_progress intermediate (the attempt
// records its verdict, NO ledger word, NO receipt — unknown stays); the
// no_answer of the broken call (the right is spent, unknown stays); the
// terminal lock of the receipts (the idempotent confirmation skip, the
// conflicting-terminal alarm — never an auto-replacement); the fencing
// of the verdict recording (the stale holder NEVER writes history); the
// crash recovery over the expired lease (the next attempt within the
// bound of three); the material backoff window (next_eligible_at); the
// racing skip (the maximum ONE active chain); the decided-verify of the
// at-least-once command repeat; every non-querying fence by its exact
// reason word with the adapter NEVER called and the lease NEVER taken
// (the undeclared semantics, the paused tenant, the paused connector,
// the exhausted attempts, the AG-4 nil adapter); the SAFE probes of the
// stored rows; the database discipline of 00004 (RLS, the grants, the
// CHECK backstops, the append-only refusals); the digest divergence of
// the retry-unlock (the integrity alarm, no send); the scope gates of
// the command; the readback of the reconciliation attempts; and the
// import-graph probe (the production root never imports the synthetic).
// The transport rows of the spec matrix (the strict 400 of a free body
// field BEFORE the service, the page PRG discipline) live on the
// transport surface — the unit transport matrix and the z35 journey
// walk them; this matrix pins the app/store truth they stand on.
//
// The slot registry (the retro-2-3 discipline): the world seed is 357
// (the free slot of the 3xx decade — the ingest-ladder suite owns every
// x10 seed, the preview matrix owns 344, the operation reservation owns
// 355, the dispatch matrix owns 356), the identity ids live in the free
// b9xx suffix range (the dispatch matrix owns b801..b88x), the
// operation target refs own the 019a3f77 range (the 2.9 world owns
// 019a2f4c, the 3-3 world owns 019a3f33, the 3-4 world owns 019a3f44,
// the 3-5 world owns 019a3f55, the 3-6 world owns 019a3f66), the
// privacy pool lives at 13700 (the preview matrix owns 13400, the
// reservation 13500, the dispatch 13600), the service id pools live in
// the 3970/4070 ranges with the 512-value headroom and the seeded probe
// identities of the store-level rows mint in the 9xxxx decimal suffix
// range of the outbox event ids.
func TestActionReconciliationMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	w, svc, dispatch, reconcile, _, operator := actionReconcileWorld(t, 357)
	caseID, workItemID := w.acceptedCase(t, "evt-rec-m1", "sess-rec-m1", `{"call":"inbound-call","from":"+79000000701"}`)
	// The honest owner actor of every human command and the reconcile
	// command (the owner carries the operations scope; the policies read
	// its role).
	owner := w.ownershipActor()
	// The reconcile command of the matrix: the owner actor, the live
	// tenant, the monotone matrix clock (every row runs at its own later
	// moment so the lease windows and the backoff windows expire honestly
	// between the considerations).
	reconcileAt := func(minute, second int) actionApp.ReconcileReport {
		t.Helper()
		report, err := svc.ReconcileUnknownOperations(ctx, actionApp.ReconcileCommand{
			Actor: owner, TenantID: w.tenant,
			Now: actionReconcileNow.Add(time.Duration(minute)*time.Minute + time.Duration(second)*time.Second),
		})
		if err != nil {
			t.Fatalf("the reconcile command of the minute %d must answer its honest report: %v", minute, err)
		}
		return report
	}
	// drainAt drains the dispatch queue at its own later moment (the
	// unknown receipts of the rows are born here).
	drainAt := func(minute int) actionApp.DispatchDrainReport {
		t.Helper()
		report, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: owner, TenantID: w.tenant, Now: actionReconcileNow.Add(time.Duration(minute) * time.Minute),
		})
		if err != nil {
			t.Fatalf("the drain command of the minute %d must answer its honest report: %v", minute, err)
		}
		return report
	}
	// linesOf collects the spoken lines of one operation's consideration.
	linesOf := func(report actionApp.ReconcileReport, operationID id.UUID) string {
		lines := []string{}
		for _, line := range report.Reports {
			if line.OperationID == operationID {
				lines = append(lines, line.Text)
			}
		}
		return strings.Join(lines, " | ")
	}
	dispatchLinesOf := func(report actionApp.DispatchDrainReport, operationID id.UUID) string {
		lines := []string{}
		for _, line := range report.Reports {
			if line.OperationID == operationID {
				lines = append(lines, line.Text)
			}
		}
		return strings.Join(lines, " | ")
	}
	// unknownReceiptOperation walks the FULL live path to ONE pending
	// unknown target of the matrix: the typed-valid proposal, the human
	// confirm, the machine authorize, then the BROKEN dispatch call (the
	// honest unknown of 3-6 — the write-ahead lease, the ledger
	// submitted+unknown entries, the receipt v1 unknown, NO outcome row).
	// The returned Operation is the immutable row of the pending target.
	unknownReceiptOperation := func(row int64, key string, minute int) actionDomain.Operation {
		t.Helper()
		op := reconcileAuthorizedOperation(t, w, svc, owner, caseID, workItemID, row, authorizeMatrixUnlimited,
			map[string]string{"phone": fmt.Sprintf("+790000007%02d", row)}, nil, nil, key, 3600, actionReconcileNow)
		dispatch.inner.Script("", "", errors.New("адаптер недоступен: обрыв соединения до провайдера"))
		dispatch.reset()
		report := drainAt(minute)
		if report.Dispatched != 1 {
			t.Fatalf("the broken send of the row %d counts as dispatched (the lease and the call happened), got %+v", row, report)
		}
		if dispatch.total() != 1 {
			t.Fatalf("the broken dispatch adapter is called once, got %d", dispatch.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2 and outcome='unknown'`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the receipt v1 unknown of the row %d, got %d", row, n)
		}
		return op
	}

	// Row 1 — the FULL LIVE PATH of the reconciliation: the pending
	// unknown target (the receipt v1 unknown of the broken dispatch
	// call), the explicit reconcile command, the scriptable committed
	// verdict WITH the structural evidence triple — the write-ahead
	// lease + the ledger reconciling entry land BEFORE the ONE query,
	// the fenced verdict appends the receipt v2 {committed, the external
	// identifiers, the proof of the reconciliation} with the outbox
	// receipt.created record carrying the version in its payload; v1
	// stays byte-stable, the versions stay monotonic (UNIQUE).
	t.Run("row-1 the full live path: unknown, reconcile, committed receipt v2", func(t *testing.T) {
		op := unknownReceiptOperation(1, "rec-live-key-1", 1)
		// The v1 bytes of the append promise (AC-12): the references and
		// the details of the unknown receipt are captured BEFORE the
		// append and compared AFTER — v1 is never rewritten.
		var v1Outcome, v1References, v1Details string
		if err := adminPool.QueryRow(ctx, `select outcome, receipt_references::text, details from action.action_receipts where tenant_id=$1 and operation_id=$2 and receipt_version_no=1`,
			w.tenant, op.OperationID).Scan(&v1Outcome, &v1References, &v1Details); err != nil {
			t.Fatal(err)
		}
		reconcile.inner.Script(actionDomain.ReconciliationVerdictCommitted, "rec-corr-42", nil)
		reconcile.inner.ScriptEvidence("ext-rec-42", "7")
		reconcile.reset()
		report := reconcileAt(2, 0)
		if report.Queried != 1 || report.Decided != 1 {
			t.Fatalf("the live path queries exactly one target and lands one verdict, got %+v", report)
		}
		if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.ReconciliationCommittedText) {
			t.Fatalf("the report line speaks the committed verdict text, got %q", text)
		}
		if reconcile.total() != 1 {
			t.Fatalf("the read-only adapter is called EXACTLY once, got %d", reconcile.total())
		}
		// The write-ahead lease of attempt 1: the lease window is the
		// definition timeout (5000ms) plus the 30-second grace, the
		// backoff window is the base 30000ms plus the jitter <= 5000ms —
		// both material in the row (AC-1).
		var attemptNo int
		var leaseSeconds, backoffSeconds float64
		if err := adminPool.QueryRow(ctx, `select attempt_no,
                                extract(epoch from (lease_expires_at - created_at)), extract(epoch from (next_eligible_at - created_at))
                                from action.reconciliation_leases where tenant_id=$1 and operation_id=$2`,
			w.tenant, op.OperationID).Scan(&attemptNo, &leaseSeconds, &backoffSeconds); err != nil {
			t.Fatal(err)
		}
		if attemptNo != 1 || leaseSeconds != float64(5000+actionDomain.ReconcileLeaseGraceMS)/1000 {
			t.Fatalf("the lease of attempt 1 with the timeout+grace window (35s), got attempt=%d window=%v", attemptNo, leaseSeconds)
		}
		if backoffSeconds < float64(actionDomain.ReconcileBackoffBaseMS)/1000 ||
			backoffSeconds > float64(actionDomain.ReconcileBackoffBaseMS+actionDomain.ReconcileBackoffJitterMS)/1000 {
			t.Fatalf("the backoff window is base+jitter within [30s..35s], got %v", backoffSeconds)
		}
		// The ledger of the ONE verdict sequence: the reconciling entry
		// (NOT state-bearing — write-ahead) then the committed word.
		var ledgerOrder string
		if err := adminPool.QueryRow(ctx, `select string_agg(kind, ',' order by entry_no) from action.operation_ledger where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID).Scan(&ledgerOrder); err != nil {
			t.Fatal(err)
		}
		if ledgerOrder != "authorized,submitted,unknown,reconciling,committed" {
			t.Fatalf("the ONE verdict sequence: authorized → submitted → unknown (the broken call) → reconciling (write-ahead) → committed, got %s", ledgerOrder)
		}
		// The outcome row of the attempt: the normalized verdict word,
		// the correlation and the structural evidence triple.
		var verdict, correlation, externalID, externalVersion string
		if err := adminPool.QueryRow(ctx, `select verdict, correlation_id, external_object_id, external_object_version from action.reconciliation_outcomes where tenant_id=$1 and operation_id=$2`,
			w.tenant, op.OperationID).Scan(&verdict, &correlation, &externalID, &externalVersion); err != nil {
			t.Fatal(err)
		}
		if verdict != actionDomain.ReconciliationVerdictCommitted || correlation != "rec-corr-42" || externalID != "ext-rec-42" || externalVersion != "7" {
			t.Fatalf("the outcome row carries the guarded verdict with its evidence, got %q %q %q %q", verdict, correlation, externalID, externalVersion)
		}
		// The receipt chain: v1 unknown (byte-stable) + v2 committed with
		// the SAFE references (the external identifiers of the proof).
		type receiptRow struct {
			version  int
			outcome  string
			refBytes string
		}
		receipts := []receiptRow{}
		rows, err := adminPool.Query(ctx, `select receipt_version_no, outcome, receipt_references::text from action.action_receipts where tenant_id=$1 and operation_id=$2 order by receipt_version_no`, w.tenant, op.OperationID)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var row receiptRow
			if err := rows.Scan(&row.version, &row.outcome, &row.refBytes); err != nil {
				t.Fatal(err)
			}
			receipts = append(receipts, row)
		}
		rows.Close()
		if len(receipts) != 2 || receipts[0].version != 1 || receipts[1].version != 2 ||
			receipts[0].outcome != actionDomain.ReceiptOutcomeUnknown || receipts[1].outcome != actionDomain.ReceiptOutcomeCommitted {
			t.Fatalf("the receipt chain v1 unknown → v2 committed (monotonic, append-only), got %+v", receipts)
		}
		if receipts[0].refBytes != v1References || v1Outcome != actionDomain.ReceiptOutcomeUnknown || v1Details == "" {
			t.Fatalf("v1 stays byte-stable after the append, got %+v", receipts[0])
		}
		for _, required := range []string{caseID.String(), workItemID.String(), op.PreviewID.String(), op.ConfirmationID.String(), "ext-rec-42"} {
			if !strings.Contains(receipts[1].refBytes, required) {
				t.Fatalf("the v2 references carry the %s identity, got %s", required, receipts[1].refBytes)
			}
		}
		// The outbox receipt.created record of the append: the payload
		// carries the version 2 and the committed outcome word.
		var payload string
		if err := adminPool.QueryRow(ctx, `select payload::text from action.outbox_events where tenant_id=$1 and operation_id=$2 and event_type='receipt.created' order by created_at desc limit 1`, w.tenant, op.OperationID).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded["event_type"] != actionDomain.OutboxEventReceiptCreated || decoded["outcome"] != actionDomain.ReceiptOutcomeCommitted {
			t.Fatalf("the receipt.created payload speaks its event and outcome words, got %s", payload)
		}
		if version, ok := decoded["receipt_version_no"].(float64); !ok || int(version) != 2 {
			t.Fatalf("the receipt.created payload carries the appended version 2, got %s", payload)
		}
		// The operation card speaks the honest life words: the committed
		// state, the receipt versions visible, the reconciliation attempt
		// readback with the verdict word and the external object identity.
		view, err := svc.ReadAuthorizationView(ctx, owner, w.tenant, caseID, actionReconcileNow.Add(3*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		var committed *actionAPI.OperationCardView
		for i := range view.Operations {
			if view.Operations[i].OperationID == op.OperationID {
				committed = &view.Operations[i]
			}
		}
		if committed == nil || committed.State != actionDomain.OperationCommitted {
			t.Fatalf("the operation card speaks the committed life word, got %+v", committed)
		}
		if len(committed.Receipts) != 2 || committed.Receipts[1].ReceiptVersionNo != 2 || committed.Receipts[1].Outcome != actionDomain.ReceiptOutcomeCommitted {
			t.Fatalf("the receipt versions are visible in the card readback, got %+v", committed.Receipts)
		}
		if len(committed.ReconciliationAttempts) != 1 || committed.ReconciliationAttempts[0].Verdict != actionDomain.ReconciliationVerdictCommitted ||
			committed.ReconciliationAttempts[0].ExternalObjectID != "ext-rec-42" {
			t.Fatalf("the reconciliation attempt readback carries the verdict and the evidence identity, got %+v", committed.ReconciliationAttempts)
		}
	})

	// Row 2 — the no_effect pair under the DECLARED absence semantics and
	// the RETRY-UNLOCK: the verdict writes {ledger failed, receipt v2
	// failed} (the proven absence rides the receipt content, the state
	// dictionaries never grow), the report line carries the unlock word;
	// the NEXT dispatch drain re-runs the FULL 3-6 ladder — the new
	// dispatch attempt attempt+1 of the SAME operation with the SAME
	// external idempotency key and request digest (no new intent, no new
	// reservation), the scripted committed claim of the second attempt
	// lands the terminal success.
	t.Run("row-2 no_effect: the failed pair and the retry-unlock of the next drain", func(t *testing.T) {
		op := unknownReceiptOperation(2, "rec-live-key-2", 4)
		reconcile.inner.Script(actionDomain.ReconciliationVerdictNoEffect, "", nil)
		reconcile.reset()
		report := reconcileAt(5, 0)
		if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.ReconciliationNoEffectText) || !strings.Contains(text, actionDomain.DispatchNoEffectUnlockText) {
			t.Fatalf("the no_effect line speaks the verdict AND the unlock word, got %q", text)
		}
		if reconcile.total() != 1 {
			t.Fatalf("the no_effect verdict queried exactly once, got %d", reconcile.total())
		}
		var ledgerOrder string
		if err := adminPool.QueryRow(ctx, `select string_agg(kind, ',' order by entry_no) from action.operation_ledger where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID).Scan(&ledgerOrder); err != nil {
			t.Fatal(err)
		}
		if ledgerOrder != "authorized,submitted,unknown,reconciling,failed" {
			t.Fatalf("the no_effect pair: reconciling (write-ahead) then failed (the proven absence), got %s", ledgerOrder)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2 and receipt_version_no=2 and outcome='failed'`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the receipt v2 failed of the proven absence, got %d", n)
		}
		// The retry-unlock drain: the second dispatch attempt of the same
		// external key and digest, the scripted committed verdict.
		dispatch.inner.Script(actionDomain.MutationOutcomeCommitted, "", nil)
		dispatch.inner.ScriptEvidence("ext-rec-2", "3")
		dispatch.reset()
		retry := drainAt(6)
		if text := dispatchLinesOf(retry, op.OperationID); !strings.Contains(text, actionDomain.OperationCommittedText) {
			t.Fatalf("the unlocked retry lands the committed verdict of the second attempt, got %q", text)
		}
		if dispatch.total() != 1 {
			t.Fatalf("the unlocked retry sends exactly once, got %d", dispatch.total())
		}
		// The TWO dispatch attempts carry ONE external key and ONE digest
		// (the provider deduplicates the repeats — AC-6).
		var attempts, distinctKeys, distinctDigests int
		if err := adminPool.QueryRow(ctx, `select count(*), count(distinct external_idempotency_key), count(distinct request_digest) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`,
			w.tenant, op.OperationID).Scan(&attempts, &distinctKeys, &distinctDigests); err != nil {
			t.Fatal(err)
		}
		if attempts != 2 || distinctKeys != 1 || distinctDigests != 1 {
			t.Fatalf("the retry is the second attempt of the SAME key and digest, got attempts=%d keys=%d digests=%d", attempts, distinctKeys, distinctDigests)
		}
		// The terminal success of the second attempt: the operation
		// state commits and the receipt chain appends the new deciding
		// version of the NEW policy decision (the versions stay
		// monotonic).
		var stateLedger string
		if err := adminPool.QueryRow(ctx, `select string_agg(kind, ',' order by entry_no) from action.operation_ledger where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID).Scan(&stateLedger); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(stateLedger, ",committed") {
			t.Fatalf("the retry lands the committed ledger word, got %s", stateLedger)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 3 {
			t.Fatalf("the receipt chain of the full journey: v1 unknown, v2 failed, v3 committed — got %d", n)
		}
		var lastOutcome string
		var lastVersion int
		if err := adminPool.QueryRow(ctx, `select receipt_version_no, outcome from action.action_receipts where tenant_id=$1 and operation_id=$2 order by receipt_version_no desc limit 1`, w.tenant, op.OperationID).Scan(&lastVersion, &lastOutcome); err != nil {
			t.Fatal(err)
		}
		if lastVersion != 3 || lastOutcome != actionDomain.ReceiptOutcomeCommitted {
			t.Fatalf("the latest receipt version speaks the committed verdict of the retry, got v%d %q", lastVersion, lastOutcome)
		}
		// ONE reservation row of the business intent after the full
		// journey (the unlock creates NO new intent — AC-6/UNIQUE 3-5).
		if n := operationSQLCount(t, `select count(*) from action.reservations where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("ONE reservation of the business intent, got %d", n)
		}
	})

	// The store-level probe handle of the fencing rows: rows 7..9 and 11
	// speak to the REAL reconciliation store directly over the SAME live
	// PG and the SAME tenant scope of the world owner (the dispatch
	// matrix row-9 precedent — the fencing and the terminal lock are
	// store truths the live command only repeats honestly).
	reconcileStore := actionStore.NewPostgres(runtimePool)
	reconcileScope, scopeErr := tenancy.New(owner.ID, w.tenant)
	if scopeErr != nil {
		t.Fatal(scopeErr)
	}
	// seedReconcileLease seeds ONE write-ahead reconciliation lease of the
	// store-level rows (the honest seeding of the crashed-holder resting
	// states — the REAL AcquireReconciliationLease transaction with the
	// probe-chosen windows, the SAME external key and digest the dispatch
	// recorded, the monotonic ledger entry under the advisory lock): rows
	// 9 and 11 own the resting lease shapes the live command can never
	// leave behind (the live lease window always outlives the live
	// backoff), the dispatch matrix seeding precedent of the same suite.
	seedReconcileLease := func(op actionDomain.Operation, token string, now, expires, eligible time.Time) int {
		t.Helper()
		var extKey, digest string
		if err := adminPool.QueryRow(ctx, `select external_idempotency_key, request_digest from action.dispatch_leases where tenant_id=$1 and operation_id=$2 order by attempt_no asc limit 1`,
			w.tenant, op.OperationID).Scan(&extKey, &digest); err != nil {
			t.Fatal(err)
		}
		var entryNo int64
		if err := adminPool.QueryRow(ctx, `select coalesce(max(entry_no),0)+1 from action.operation_ledger where tenant_id=$1 and operation_id=$2`,
			w.tenant, op.OperationID).Scan(&entryNo); err != nil {
			t.Fatal(err)
		}
		attemptNo, err := reconcileStore.AcquireReconciliationLease(ctx, actionPort.AcquireReconciliationLeaseParams{
			Scope:                  reconcileScope,
			Operation:              op,
			DefinitionID:           op.DefinitionID,
			DefinitionVersion:      op.DefinitionVersion,
			ReconciliationWord:     "не требуется (read-only)",
			LeaseToken:             token,
			ExternalIDempotencyKey: extKey,
			RequestDigest:          digest,
			LeaseExpiresAt:         expires,
			NextEligibleAt:         eligible,
			LedgerEntry: actionDomain.OperationLedgerEntry{
				EntryNo: entryNo, Kind: actionDomain.LedgerEntryReconciling,
				Details: "the seeded resting lease of the store-level row",
			},
			Now: now,
		})
		if err != nil {
			t.Fatalf("the seeded reconciliation lease lands through the real store transaction: %v", err)
		}
		return attemptNo
	}
	// cleanupLeftoverAt is the matrix hygiene pass of the honest-unknown
	// rows: the row's probes are pinned FIRST, then the leftover target
	// resolves through the LIVE declared path (the scripted committed
	// verdict with the evidence) so every later row owns its own batch —
	// the fence truth the row pinned stays the truth of the row, the
	// resolved state only keeps the later batches clean.
	cleanupLeftoverAt := func(minute int) {
		t.Helper()
		reconcile.inner.Script(actionDomain.ReconciliationVerdictCommitted, "", nil)
		reconcile.inner.ScriptEvidence(fmt.Sprintf("ext-rec-clean-%d", minute), "1")
		reconcile.reset()
		if _, err := svc.ReconcileUnknownOperations(ctx, actionApp.ReconcileCommand{
			Actor: owner, TenantID: w.tenant,
			Now: actionReconcileNow.Add(time.Duration(minute) * time.Minute),
		}); err != nil {
			t.Fatalf("the hygiene pass of the minute %d must answer its honest report: %v", minute, err)
		}
	}

	// Row 3 — the UNDECLARED reconciliation fence: the target whose
	// connector carries NO live reconciliation word (the second local
	// service of the SAME world — the same stores, the same dictionary
	// minus the query_reconcilable shapes, the resolver that drops the
	// LIVE declaration word of the channel capability: the catalog keeps
	// its word, the live reading of the fence sees the connector that
	// stopped declaring it). The honest line speaks the undeclared word,
	// the adapter is NEVER called, the lease is NEVER taken, the ledger
	// gains NO reconciling entry, NO receipt version lands and the retry
	// stays locked — the next drain parks the target with the
	// blind-repeat-forbidden word of 3-6 (a bare 404 is never a proof of
	// absence — the target stays unknown for the human, AC-3/AC-9).
	t.Run("row-3 the undeclared connector: no lease, no query, the retry stays locked", func(t *testing.T) {
		op := unknownReceiptOperation(3, "rec-live-key-3", 10)
		undeclared := w.buildActionReconcileUndeclaredService(reconcile, actionPreviewIDs(4170, 64))
		reconcile.reset()
		report, err := undeclared.ReconcileUnknownOperations(ctx, actionApp.ReconcileCommand{
			Actor: owner, TenantID: w.tenant,
			Now: actionReconcileNow.Add(11 * time.Minute),
		})
		if err != nil {
			t.Fatalf("the undeclared command answers its honest report: %v", err)
		}
		if report.Queried != 0 || report.Decided != 0 {
			t.Fatalf("the undeclared fence queries nothing and records no verdict, got %+v", report)
		}
		if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.ReconciliationNotDeclaredText) {
			t.Fatalf("the undeclared line speaks the exact domain word, got %q", text)
		}
		if reconcile.total() != 0 {
			t.Fatalf("the adapter is never called on the undeclared target, got %d", reconcile.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("the undeclared fence never takes the lease, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operation_ledger where tenant_id=$1 and operation_id=$2 and kind='reconciling'`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("no write-ahead reconciling entry of the undeclared target, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the receipt chain stays the single unknown version, got %d", n)
		}
		// The retry stays locked: the next drain parks the target (no
		// no_effect verdict exists — the proven absence is the only
		// unlock) and the adapter is never called.
		dispatch.reset()
		retry := drainAt(12)
		if text := dispatchLinesOf(retry, op.OperationID); !strings.Contains(text, actionDomain.DispatchRetryText(actionDomain.DispatchRetryParkUnknown)) {
			t.Fatalf("the undeclared target parks in the drain — the retry does NOT unlock, got %q", text)
		}
		if dispatch.total() != 0 {
			t.Fatalf("the parked target never sends, got %d", dispatch.total())
		}
		cleanupLeftoverAt(13)
	})

	// Row 4 — the IN_PROGRESS intermediate: the declared owning system is
	// still processing — the attempt records its verdict (the outcome row
	// carries in_progress with the provider correlation), the write-ahead
	// lease and the reconciling entry land, but NO state-bearing word and
	// NO receipt version follow: the ledger stays exactly
	// authorized,submitted,unknown,reconciling, the receipt chain stays
	// the single unknown version, the operation stays unknown (the
	// unknown/reconciling slash of the epic — the reconciliation is an
	// activity in the history, never a state).
	t.Run("row-4 in_progress: the attempt records, the operation stays unknown", func(t *testing.T) {
		op := unknownReceiptOperation(4, "rec-live-key-4", 14)
		reconcile.inner.Script(actionDomain.ReconciliationVerdictInProgress, "rec-corr-4", nil)
		reconcile.reset()
		report := reconcileAt(15, 0)
		if report.Queried != 1 || report.Decided != 1 {
			t.Fatalf("the intermediate verdict queries once and records once, got %+v", report)
		}
		if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.ReconciliationInProgressText) {
			t.Fatalf("the in_progress line speaks its fixed honest text, got %q", text)
		}
		var verdict, correlation string
		if err := adminPool.QueryRow(ctx, `select verdict, correlation_id from action.reconciliation_outcomes where tenant_id=$1 and operation_id=$2`,
			w.tenant, op.OperationID).Scan(&verdict, &correlation); err != nil {
			t.Fatal(err)
		}
		if verdict != actionDomain.ReconciliationVerdictInProgress || correlation != "rec-corr-4" {
			t.Fatalf("the attempt records the declared intermediate with its correlation, got %q %q", verdict, correlation)
		}
		var ledgerOrder string
		if err := adminPool.QueryRow(ctx, `select string_agg(kind, ',' order by entry_no) from action.operation_ledger where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID).Scan(&ledgerOrder); err != nil {
			t.Fatal(err)
		}
		if ledgerOrder != "authorized,submitted,unknown,reconciling" {
			t.Fatalf("in_progress writes NO state-bearing ledger word, got %s", ledgerOrder)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("in_progress appends NO receipt version, got %d", n)
		}
		cleanupLeftoverAt(16)
	})

	// Row 5 — the NO_ANSWER of the broken call: the adapter error is the
	// honest absence of a verdict — the right is SPENT (the write-ahead
	// lease of attempt 1 with its windows), the outcome row records
	// no_answer, the ledger stays at the reconciling activity, the
	// receipt chain stays the single unknown version and the operation
	// stays unknown (never a masked success, never a masked absence).
	t.Run("row-5 no_answer of the broken call: the right is spent, unknown stays", func(t *testing.T) {
		op := unknownReceiptOperation(5, "rec-live-key-5", 17)
		reconcile.inner.Script("", "", errors.New("reconciliation adapter unreachable: connection reset before the owning system"))
		reconcile.reset()
		report := reconcileAt(18, 0)
		if report.Queried != 1 || report.Decided != 1 {
			t.Fatalf("the broken call still counts its query and its recorded truth, got %+v", report)
		}
		if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.ReconciliationNoAnswerText) {
			t.Fatalf("the broken call speaks the honest no_answer word, got %q", text)
		}
		var verdict string
		var attemptNo int
		if err := adminPool.QueryRow(ctx, `select verdict, attempt_no from action.reconciliation_outcomes where tenant_id=$1 and operation_id=$2`,
			w.tenant, op.OperationID).Scan(&verdict, &attemptNo); err != nil {
			t.Fatal(err)
		}
		if verdict != actionDomain.ReconciliationVerdictNoAnswer || attemptNo != 1 {
			t.Fatalf("the spent right records the honest no_answer of attempt 1, got %q attempt=%d", verdict, attemptNo)
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_leases where tenant_id=$1 and operation_id=$2 and attempt_no=1`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the write-ahead lease of the spent right, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("no_answer appends NO receipt version, got %d", n)
		}
		cleanupLeftoverAt(19)
	})

	// Row 6 — the COMMITTED CLAIM WITHOUT THE EVIDENCE TRIPLE: the owning
	// system claims the terminal success but names no external object —
	// the normalization refuses the claim (the terminal success is never
	// taken on word, AC-2): the attempt spends its right as no_answer,
	// NO committed word ever lands in the ledger, NO receipt v2 follows
	// (the chain stays the single unknown version).
	t.Run("row-6 the committed claim without evidence: refused to no_answer", func(t *testing.T) {
		op := unknownReceiptOperation(6, "rec-live-key-6", 20)
		reconcile.inner.Script(actionDomain.ReconciliationVerdictCommitted, "rec-corr-6", nil)
		reconcile.inner.ScriptEvidence("", "")
		reconcile.reset()
		report := reconcileAt(21, 0)
		if report.Queried != 1 || report.Decided != 1 {
			t.Fatalf("the refused claim still counts its query and its recorded truth, got %+v", report)
		}
		if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.ReconciliationNoAnswerText) {
			t.Fatalf("the evidence-less committed claim reads the honest no_answer, got %q", text)
		}
		var verdict string
		if err := adminPool.QueryRow(ctx, `select verdict from action.reconciliation_outcomes where tenant_id=$1 and operation_id=$2`,
			w.tenant, op.OperationID).Scan(&verdict); err != nil {
			t.Fatal(err)
		}
		if verdict != actionDomain.ReconciliationVerdictNoAnswer {
			t.Fatalf("the outcome row records the normalized refusal, got %q", verdict)
		}
		if n := operationSQLCount(t, `select count(*) from action.operation_ledger where tenant_id=$1 and operation_id=$2 and kind='committed'`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("the refused claim writes NO committed ledger word, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the refused claim appends NO receipt v2, got %d", n)
		}
		cleanupLeftoverAt(22)
	})

	// Row 7 — the TERMINAL LOCK, the confirmation half: the live path
	// lands the committed receipt v2 (the row-1 shape), then the
	// AT-LEAST-ONCE command repeat over the terminal answers the honest
	// decided skip — the classification cuts BEFORE the query (the
	// receipt is no longer unknown: the adapter is never called again, no
	// new version, no downgrade) — and the STORE-level same-terminal
	// claim (a second attempt whose verdict claims the SAME committed
	// outcome over the committed v2) skips silently: the idempotent
	// confirmation of the terminal lock, no new version, no outbox record
	// (AC-5).
	t.Run("row-7 the terminal lock confirmation: the repeat is the honest skip", func(t *testing.T) {
		op := unknownReceiptOperation(7, "rec-live-key-7", 23)
		reconcile.inner.Script(actionDomain.ReconciliationVerdictCommitted, "rec-corr-7", nil)
		reconcile.inner.ScriptEvidence("ext-rec-7", "4")
		reconcile.reset()
		first := reconcileAt(24, 0)
		if first.Queried != 1 || first.Decided != 1 {
			t.Fatalf("the live path of the row queries once and lands one verdict, got %+v", first)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 2 {
			t.Fatalf("the receipt chain v1 unknown → v2 committed, got %d", n)
		}
		// The at-least-once repeat: the decided-verify cut speaks FIRST —
		// no query, no new version (the repeat is the honest skip of the
		// I/O matrix; the different correlation of the script is never
		// consumed).
		reconcile.inner.Script(actionDomain.ReconciliationVerdictCommitted, "rec-corr-7-other", nil)
		reconcile.reset()
		repeat := reconcileAt(25, 0)
		if repeat.Queried != 0 {
			t.Fatalf("the decided operation is never queried again, got %+v", repeat)
		}
		if text := linesOf(repeat, op.OperationID); !strings.Contains(text, fmt.Sprintf(actionDomain.ReconciliationDecidedTextTemplate, actionDomain.ReceiptOutcomeCommitted)) {
			t.Fatalf("the repeat speaks the decided word with the committed outcome, got %q", text)
		}
		if reconcile.total() != 0 {
			t.Fatalf("the repeat never calls the adapter, got %d", reconcile.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 2 {
			t.Fatalf("the confirmation skip appends NO new version, got %d", n)
		}
		// The STORE-level same-terminal claim: the second attempt (its own
		// write-ahead lease after the first window expired) claims the
		// SAME committed outcome over the committed v2 — the guard answers
		// the silent skip, the verdict row of the attempt still records,
		// NO receipt version and NO outbox record follow.
		token := strings.Repeat("7", 64)
		claimAt := actionReconcileNow.Add(26 * time.Minute)
		if attempt := seedReconcileLease(op, token, claimAt, claimAt.Add(35*time.Second), claimAt.Add(time.Minute)); attempt != 2 {
			t.Fatalf("the store-level confirmation claim rides the second attempt, got %d", attempt)
		}
		if err := reconcileStore.RecordReconciliationVerdict(ctx, actionPort.RecordReconciliationVerdictParams{
			Scope:         reconcileScope,
			Operation:     op,
			AttemptNo:     2,
			LeaseToken:    token,
			Verdict:       actionDomain.ReconciliationVerdictCommitted,
			LedgerWord:    actionDomain.LedgerEntryCommitted,
			ReceiptWord:   actionDomain.ReceiptOutcomeCommitted,
			CorrelationID: "rec-corr-7-late",
			Evidence: actionDomain.MutationEvidence{
				ExternalObjectID: "ext-rec-7-late", ExternalObjectVersion: "2",
				ExternalObjectTime: claimAt,
			},
			ReceiptReferences: actionPort.ReceiptReferences{
				CaseID: op.CaseID, WorkItemID: op.WorkItemID,
				PreviewID: op.PreviewID, PreviewVersionNo: op.PreviewVersionNo,
				ConfirmationID: op.ConfirmationID, ConfirmationVersionNo: op.ConfirmationVersionNo,
				DefinitionID: op.DefinitionID, DefinitionVersion: op.DefinitionVersion,
				ExternalObjectID: "ext-rec-7-late", ExternalObjectVersion: "2",
			},
			ReceiptDetails: "the same-terminal confirmation probe of the terminal lock",
			OutboxEventID:  id.MustParse("01994f64-6c00-7000-8000-000000009701"),
			Now:            claimAt,
		}); err != nil {
			t.Fatalf("the same-terminal claim skips silently — never an error, never a new version: %v", err)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 2 {
			t.Fatalf("the idempotent confirmation keeps the two versions, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.outbox_events where tenant_id=$1 and operation_id=$2 and event_type='receipt.created'`, w.tenant, op.OperationID); n != 2 {
			t.Fatalf("the confirmation skip writes NO new outbox record, got %d", n)
		}
	})

	// Row 8 — the TERMINAL LOCK, the conflict half: after the live
	// committed v2 the CONFLICTING terminal claim (no_effect over
	// committed) refuses with the integrity alarm — the claim rides a
	// FRESH fenced attempt directly at the store (the live command itself
	// would cut at the decided-verify fence, the store guard is the belt
	// over braces; the first attempt already spent its recorded truth, so
	// the conflicting claim takes the second write-ahead lease):
	// ErrTerminalConflict, the whole verdict rolls back (NO
	// new outcome row, NO failed ledger word, NO receipt version) — the
	// human review owns the resolution, never an auto-replacement (AC-5).
	t.Run("row-8 the terminal lock conflict: the alarm never auto-replaces", func(t *testing.T) {
		op := unknownReceiptOperation(8, "rec-live-key-8", 27)
		reconcile.inner.Script(actionDomain.ReconciliationVerdictCommitted, "rec-corr-8", nil)
		reconcile.inner.ScriptEvidence("ext-rec-8", "5")
		reconcile.reset()
		first := reconcileAt(28, 0)
		if first.Queried != 1 || first.Decided != 1 {
			t.Fatalf("the live path of the row queries once and lands one verdict, got %+v", first)
		}
		// The conflicting claim rides a FRESH fenced attempt (the first
		// attempt already recorded its committed truth — the outcome row of
		// one attempt never lands twice): the second write-ahead lease after
		// the first window expired carries the no_effect claim straight into
		// the store guard over the committed v2.
		token := strings.Repeat("8", 64)
		conflictAt := actionReconcileNow.Add(29 * time.Minute)
		if attempt := seedReconcileLease(op, token, conflictAt, conflictAt.Add(35*time.Second), conflictAt.Add(time.Minute)); attempt != 2 {
			t.Fatalf("the conflicting claim rides the second attempt, got %d", attempt)
		}
		if err := reconcileStore.RecordReconciliationVerdict(ctx, actionPort.RecordReconciliationVerdictParams{
			Scope:          reconcileScope,
			Operation:      op,
			AttemptNo:      2,
			LeaseToken:     token,
			Verdict:        actionDomain.ReconciliationVerdictNoEffect,
			LedgerWord:     actionDomain.LedgerEntryFailed,
			ReceiptWord:    actionDomain.ReceiptOutcomeFailed,
			CorrelationID:  "rec-corr-8-conflict",
			ReceiptDetails: "the conflicting terminal claim probe of the terminal lock",
			OutboxEventID:  id.MustParse("01994f64-6c00-7000-8000-000000009702"),
			Now:            conflictAt,
		}); !errors.Is(err, actionPort.ErrTerminalConflict) {
			t.Fatalf("the conflicting terminal claim refuses with the integrity alarm, got %v", err)
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_outcomes where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the refused verdict rolls back its outcome row, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operation_ledger where tenant_id=$1 and operation_id=$2 and kind='failed'`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("the refused verdict writes NO failed ledger word, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 2 {
			t.Fatalf("the alarm never replaces the terminal version, got %d", n)
		}
	})

	// Row 9 — the FENCING of the verdict recording: the (attempt, token)
	// pair must own the CURRENT attempt — the older holder behind a
	// newer attempt and the foreign token of the current attempt both
	// refuse with ErrReconciliationLeaseStale, the history gains NOTHING
	// (the stale holder NEVER writes history; the dispatch row-9
	// precedent walked the same truth over the send contour).
	t.Run("row-9 fencing: the stale pair never writes history", func(t *testing.T) {
		op := unknownReceiptOperation(9, "rec-live-key-9", 30)
		token1 := strings.Repeat("9", 64)
		token2 := strings.Repeat("a", 64)
		seedAt := actionReconcileNow.Add(30*time.Minute + 30*time.Second)
		if attempt := seedReconcileLease(op, token1, seedAt, seedAt.Add(35*time.Second), seedAt.Add(40*time.Second)); attempt != 1 {
			t.Fatalf("the first seeded lease rides attempt 1, got %d", attempt)
		}
		if attempt := seedReconcileLease(op, token2, seedAt.Add(40*time.Second), seedAt.Add(75*time.Second), actionReconcileNow.Add(2*time.Hour)); attempt != 2 {
			t.Fatalf("the second seeded lease rides attempt 2 after the first window expired, got %d", attempt)
		}
		if err := reconcileStore.RecordReconciliationVerdict(ctx, actionPort.RecordReconciliationVerdictParams{
			Scope:         reconcileScope,
			Operation:     op,
			AttemptNo:     1,
			LeaseToken:    token1,
			Verdict:       actionDomain.ReconciliationVerdictCommitted,
			OutboxEventID: id.MustParse("01994f64-6c00-7000-8000-000000009703"),
			Now:           seedAt.Add(60 * time.Second),
		}); !errors.Is(err, actionPort.ErrReconciliationLeaseStale) {
			t.Fatalf("the older holder behind the newer attempt refuses, got %v", err)
		}
		if err := reconcileStore.RecordReconciliationVerdict(ctx, actionPort.RecordReconciliationVerdictParams{
			Scope:         reconcileScope,
			Operation:     op,
			AttemptNo:     2,
			LeaseToken:    strings.Repeat("0", 64),
			Verdict:       actionDomain.ReconciliationVerdictCommitted,
			OutboxEventID: id.MustParse("01994f64-6c00-7000-8000-000000009704"),
			Now:           seedAt.Add(60 * time.Second),
		}); !errors.Is(err, actionPort.ErrReconciliationLeaseStale) {
			t.Fatalf("the foreign token of the current attempt refuses, got %v", err)
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_outcomes where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("the stale holders write NO history, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the fenced rows touch NO receipt version, got %d", n)
		}
	})

	// Row 10 — the CRASH RECOVERY over the expired lease: the first
	// attempt breaks (the row-5 shape — the right spent as no_answer),
	// the monotone clock walks past the lease window AND the backoff
	// window, the next command takes the SECOND attempt within the bound
	// of three and lands the terminal success with the evidence — the
	// receipt v2 committed, the attempts read back 1..2 (the bounded
	// recovery, never a blind loop).
	t.Run("row-10 the crash recovery: the second attempt after the expired lease", func(t *testing.T) {
		op := unknownReceiptOperation(10, "rec-live-key-10", 33)
		reconcile.inner.Script("", "", errors.New("reconciliation adapter unreachable: the first attempt died mid-flight"))
		reconcile.reset()
		first := reconcileAt(34, 0)
		if first.Queried != 1 || first.Decided != 1 {
			t.Fatalf("the broken first attempt still counts its query and its truth, got %+v", first)
		}
		// The recovery moment: +2 minutes walks past the 35-second lease
		// window and the 30..35-second backoff window of attempt 1.
		reconcile.inner.Script(actionDomain.ReconciliationVerdictCommitted, "rec-corr-10", nil)
		reconcile.inner.ScriptEvidence("ext-rec-10", "6")
		reconcile.reset()
		second := reconcileAt(36, 0)
		if second.Queried != 1 || second.Decided != 1 {
			t.Fatalf("the recovered attempt queries once and lands its verdict, got %+v", second)
		}
		if text := linesOf(second, op.OperationID); !strings.Contains(text, actionDomain.ReconciliationCommittedText) {
			t.Fatalf("the recovered attempt speaks the committed verdict text, got %q", text)
		}
		if reconcile.total() != 1 {
			t.Fatalf("the recovery calls the adapter exactly once, got %d", reconcile.total())
		}
		var attempts, maxAttempt int
		if err := adminPool.QueryRow(ctx, `select count(*), max(attempt_no) from action.reconciliation_leases where tenant_id=$1 and operation_id=$2`,
			w.tenant, op.OperationID).Scan(&attempts, &maxAttempt); err != nil {
			t.Fatal(err)
		}
		if attempts != 2 || maxAttempt != 2 {
			t.Fatalf("the bounded recovery rides the second attempt within the bound of three, got attempts=%d max=%d", attempts, maxAttempt)
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_outcomes where tenant_id=$1 and operation_id=$2 and verdict='committed'`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the recovered attempt records its committed verdict, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2 and receipt_version_no=2 and outcome='committed'`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the recovery appends the receipt v2 committed, got %d", n)
		}
	})

	// Row 11 — the BACKOFF WINDOW: the resting target of a crashed holder
	// (the seeded store lease: the window expired, the recorded
	// next_eligible_at still in the future — the LIVE command can never
	// leave this shape behind because the live lease window always
	// outlives the live backoff, the seeded resting state IS the honest
	// crash shape) answers the honest «not ready until» word with the
	// exact material moment: NO new lease, NO query, the target waits.
	t.Run("row-11 the backoff window: not ready until the recorded moment", func(t *testing.T) {
		op := unknownReceiptOperation(11, "rec-live-key-11", 39)
		token := strings.Repeat("b", 64)
		seedAt := actionReconcileNow.Add(39*time.Minute + 30*time.Second)
		waitUntil := actionReconcileNow.Add(2 * time.Hour)
		if attempt := seedReconcileLease(op, token, seedAt, seedAt.Add(5*time.Second), waitUntil); attempt != 1 {
			t.Fatalf("the resting lease of the crashed holder rides attempt 1, got %d", attempt)
		}
		reconcile.reset()
		report := reconcileAt(40, 0)
		if report.Queried != 0 || report.Decided != 0 {
			t.Fatalf("the waiting target is never queried, got %+v", report)
		}
		if text := linesOf(report, op.OperationID); !strings.Contains(text, fmt.Sprintf(actionDomain.ReconciliationBackoffTextTemplate, waitUntil.Format(time.RFC3339))) {
			t.Fatalf("the waiting line speaks the exact material moment, got %q", text)
		}
		if reconcile.total() != 0 {
			t.Fatalf("the backoff window never calls the adapter, got %d", reconcile.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the waiting command takes NO new lease, got %d", n)
		}
	})

	// Row 12 — the RACING SKIP: the target with the ACTIVE un-expired
	// lease of another consideration (the broken first command holds the
	// right — seconds, not minutes: the second command runs INSIDE the
	// 35-second window) answers the honest single-chain word: NO second
	// lease, NO query, the parallel command owns nothing (the maximum ONE
	// active chain of the epic fault-block).
	t.Run("row-12 racing: the active lease keeps the single chain", func(t *testing.T) {
		op := unknownReceiptOperation(12, "rec-live-key-12", 42)
		reconcile.inner.Script("", "", errors.New("reconciliation adapter unreachable: the first chain holds the right"))
		reconcile.reset()
		first := reconcileAt(43, 0)
		if first.Queried != 1 || first.Decided != 1 {
			t.Fatalf("the first chain takes its right and records its truth, got %+v", first)
		}
		reconcile.reset()
		second := reconcileAt(43, 20)
		if second.Queried != 0 {
			t.Fatalf("the parallel command never queries, got %+v", second)
		}
		if text := linesOf(second, op.OperationID); !strings.Contains(text, actionDomain.ReconciliationRacingText) {
			t.Fatalf("the racing line speaks the single-chain word, got %q", text)
		}
		if reconcile.total() != 0 {
			t.Fatalf("the racing skip never calls the adapter, got %d", reconcile.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the parallel command takes NO second lease, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the racing skip writes NO receipt version, got %d", n)
		}
	})

	// The LIVE control service of the paused rows: the scoped controls of
	// the tenant point and the connector point land through the REAL 3-2
	// surface over the same live PG (the control Set/Resume demands the
	// operator manage surface — the operations-scoped operator member of
	// the world; the ids pool of the control mutations lives in the shared
	// b4xxx range, the dispatch matrix control precedent).
	identityLive := identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{time.Now().UTC()}, newPrivacyIDs(13700))
	controls := controlApp.New(controlStore.NewPostgres(runtimePool), identityLive, connectorrefresolver.New(), fixedClock{actionReconcileNow}, &sequenceIDs{values: controlMatrixIDs(8)})

	// Row 13 — the PAUSED TENANT: the scoped control of the tenant point
	// (the LIVE 3-2 surface — the control Set through the real control
	// service, the paused mode of the honest operator word) blocks the
	// query BEFORE the lease: the honest line speaks the exact paused word
	// with the reason of the winning point, the adapter is never called,
	// the write-ahead lease is never taken and the ledger gains NO
	// reconciling entry (the dispatch matrix paused-control precedent —
	// the fence reads the LIVE scoped controls at the query moment; the
	// lift heals the world for the later rows).
	t.Run("row-13 the paused tenant: no lease, no query, the exact paused word", func(t *testing.T) {
		op := unknownReceiptOperation(13, "rec-live-key-13", 45)
		if _, err := controls.Set(ctx, operator, w.tenant, controlAPI.SetRequest{
			Kind: controlDomain.ScopeKindTenant, Ref: w.tenant.String(), Mode: controlDomain.ControlModePaused,
			Reason:             "сверка приостановлена: инвентаризация исходящих операций",
			ExpectedGeneration: 0, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b911"),
		}); err != nil {
			t.Fatal(err)
		}
		reconcile.reset()
		report := reconcileAt(46, 0)
		if report.Queried != 0 {
			t.Fatalf("the paused tenant never queries, got %+v", report)
		}
		if text := linesOf(report, op.OperationID); !strings.Contains(text, fmt.Sprintf(actionDomain.ReconciliationPausedTextTemplate,
			actionDomain.ScopedControlPaused+": "+"сверка приостановлена: инвентаризация исходящих операций")) {
			t.Fatalf("the paused line speaks the exact domain word with the reason of the winning point, got %q", text)
		}
		if reconcile.total() != 0 {
			t.Fatalf("the paused fence never calls the adapter, got %d", reconcile.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("the paused fence never takes the lease, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operation_ledger where tenant_id=$1 and operation_id=$2 and kind='reconciling'`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("the paused fence writes NO write-ahead reconciling entry, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the paused target keeps its single unknown receipt, got %d", n)
		}
		// The lift heals the world (the paused tenant would park every later
		// preview, drain and reconcile — the fences re-run on every
		// consideration).
		if _, err := controls.Resume(ctx, operator, w.tenant, controlAPI.ResumeRequest{
			Kind: controlDomain.ScopeKindTenant, Ref: w.tenant.String(),
			Reason: "инвентаризация завершена, сверка возобновлена", ExpectedGeneration: 1,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b912"),
		}); err != nil {
			t.Fatal(err)
		}
		cleanupLeftoverAt(47)
	})

	// Row 14 — the PAUSED CONNECTOR: the scoped control of the connector
	// point of the definition's required capability (the tenant's live
	// telephony connector — the LIVE resolution of the control Set) blocks
	// the query the same honest way: the exact paused word with the
	// connector's reason rides the line, no lease, no query, no
	// reconciling entry (the recovery-scope permissions of a paused
	// connector are the 3-9 slice — this slice is conservative: paused
	// blocks); the lift heals the world.
	t.Run("row-14 the paused connector: the connector point blocks the query", func(t *testing.T) {
		op := unknownReceiptOperation(14, "rec-live-key-14", 48)
		if _, err := controls.Set(ctx, operator, w.tenant, controlAPI.SetRequest{
			Kind: controlDomain.ScopeKindConnector, Ref: "staffy-synthetic-telephony", Mode: controlDomain.ControlModePaused,
			Reason:             "коннектор на паузе: профилактика провайдера телефонии",
			ExpectedGeneration: 0, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b913"),
		}); err != nil {
			t.Fatal(err)
		}
		reconcile.reset()
		report := reconcileAt(49, 0)
		if report.Queried != 0 {
			t.Fatalf("the paused connector never queries, got %+v", report)
		}
		if text := linesOf(report, op.OperationID); !strings.Contains(text, fmt.Sprintf(actionDomain.ReconciliationPausedTextTemplate,
			actionDomain.ScopedControlPaused+": "+"коннектор на паузе: профилактика провайдера телефонии")) {
			t.Fatalf("the paused connector line speaks the exact domain word with its reason, got %q", text)
		}
		if reconcile.total() != 0 {
			t.Fatalf("the paused connector fence never calls the adapter, got %d", reconcile.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("the paused connector fence never takes the lease, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operation_ledger where tenant_id=$1 and operation_id=$2 and kind='reconciling'`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("the paused connector fence writes NO reconciling entry, got %d", n)
		}
		if _, err := controls.Resume(ctx, operator, w.tenant, controlAPI.ResumeRequest{
			Kind: controlDomain.ScopeKindConnector, Ref: "staffy-synthetic-telephony",
			Reason: "профилактика завершена, коннектор восстановлен", ExpectedGeneration: 1,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b914"),
		}); err != nil {
			t.Fatal(err)
		}
		cleanupLeftoverAt(50)
	})

	// Row 15 — the EXHAUSTED ATTEMPTS: the bounded recovery never loops —
	// three spent rights (three broken adapter calls at monotone moments,
	// each honestly recorded as no_answer with its write-ahead lease and
	// its outcome row) spend the bound of three; the FOURTH command
	// answers the honest human word BEFORE any lease: the adapter is
	// never called again, the lease count stays three, the receipt chain
	// stays the single unknown version (never a success by timer, never a
	// blind loop — the human evidence owns the resolution).
	t.Run("row-15 exhausted: the fourth command speaks the human word", func(t *testing.T) {
		op := unknownReceiptOperation(15, "rec-live-key-15", 51)
		reconcile.inner.Script("", "", errors.New("reconciliation adapter unreachable: the bounded attempts of the exhausted row"))
		reconcile.reset()
		for _, moment := range [][2]int{{52, 0}, {53, 30}, {55, 0}} {
			if report := reconcileAt(moment[0], moment[1]); report.Queried != 1 || report.Decided != 1 {
				t.Fatalf("the broken attempt of the moment %v spends its right honestly, got %+v", moment, report)
			}
		}
		if reconcile.total() != 3 {
			t.Fatalf("the three bounded attempts call the adapter exactly three times, got %d", reconcile.total())
		}
		reconcile.reset()
		fourth := reconcileAt(56, 30)
		if fourth.Queried != 0 || fourth.Decided != 0 {
			t.Fatalf("the exhausted bound never queries again, got %+v", fourth)
		}
		if text := linesOf(fourth, op.OperationID); !strings.Contains(text, actionDomain.ReconciliationExhaustedText) {
			t.Fatalf("the exhausted line speaks the honest human word, got %q", text)
		}
		if reconcile.total() != 0 {
			t.Fatalf("the fourth command never calls the adapter, got %d", reconcile.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 3 {
			t.Fatalf("the three spent rights keep exactly three leases, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_outcomes where tenant_id=$1 and operation_id=$2 and verdict='no_answer'`, w.tenant, op.OperationID); n != 3 {
			t.Fatalf("the three broken attempts recorded their honest no_answer verdicts, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the unknown stays with its single receipt version, got %d", n)
		}
	})

	// Row 16 — the AG-4 PRODUCTION REFUSAL: the composition root of the
	// production binary wires the reconciliation repository with the NIL
	// read-only adapter (cmd/staffy-core: WithReconcile(store, nil) — no
	// real owning systems exist, AG-4); the command refuses HONESTLY with
	// the typed signal and the exact domain word BEFORE the queue is even
	// read: the pending targets stay pending (no lease spent, no outcome
	// recorded, the unknown receipts untouched), the requests are zero.
	t.Run("row-16 the nil adapter: the honest AG-4 refusal before the queue", func(t *testing.T) {
		nilReconcileService := w.buildActionReconcileService(dispatch, nil, capabilityresolver.New(), owningsystem.New(), actionPreviewIDs(4180, 64))
		var leases, outcomes, unknowns int
		if err := adminPool.QueryRow(ctx, `select
                        (select count(*) from action.reconciliation_leases where tenant_id=$1),
                        (select count(*) from action.reconciliation_outcomes where tenant_id=$1),
                        (select count(*) from action.action_receipts where tenant_id=$1 and outcome='unknown')`,
			w.tenant).Scan(&leases, &outcomes, &unknowns); err != nil {
			t.Fatal(err)
		}
		if unknowns == 0 {
			t.Fatal("the world owns pending unknown targets the refusal must leave untouched")
		}
		_, err := nilReconcileService.ReconcileUnknownOperations(ctx, actionApp.ReconcileCommand{
			Actor: owner, TenantID: w.tenant, Now: actionReconcileNow.Add(57 * time.Minute),
		})
		if !errors.Is(err, actionPort.ErrReconcileRefusedAG4) {
			t.Fatalf("the nil adapter refuses with the AG-4 signal, got %v", err)
		}
		if !strings.Contains(err.Error(), actionDomain.ReconciliationNotWiredText) {
			t.Fatalf("the refusal speaks the exact domain word, got %s", err.Error())
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_leases where tenant_id=$1`, w.tenant); n != leases {
			t.Fatalf("the refusal spends NO lease of the pending queue, got %d want %d", n, leases)
		}
		if n := operationSQLCount(t, `select count(*) from action.reconciliation_outcomes where tenant_id=$1`, w.tenant); n != outcomes {
			t.Fatalf("the refusal records NO outcome of the pending queue, got %d want %d", n, outcomes)
		}
		if reconcile.total() != 0 {
			t.Fatalf("the refusal never queries through the world adapter, got %d", reconcile.total())
		}
	})

	// Row 17 — the SAFE probes of the stored rows: the four tables of the
	// contour (the reconciliation leases and outcomes, the action
	// receipts, the outbox) carry ONLY the SAFE shape — no column of the
	// material dictionary (phone/template/typed/secret/snapshot/material),
	// no stored value carries the material of the world rows (the phone
	// prefix, the typed field names, the secret template word); the
	// leases speak the keyed 64-hex identities alone (the dispatch SAFE
	// row precedent over the reconciliation surface — the raw payload
	// NEVER enters, AD-10/AC-5).
	t.Run("row-17 the SAFE probes: no material column, no material value", func(t *testing.T) {
		var named int
		if err := adminPool.QueryRow(ctx, `select count(*) from information_schema.columns
                        where table_schema='action' and table_name in ('reconciliation_leases','reconciliation_outcomes','action_receipts','outbox_events')
                        and (column_name like '%phone%' or column_name like '%template%' or column_name like '%typed%'
                                or column_name like '%secret%' or column_name like '%snapshot%' or column_name like '%material%')`).Scan(&named); err != nil {
			t.Fatal(err)
		}
		if named != 0 {
			t.Fatalf("the contour tables carry NO material column, got %d", named)
		}
		for _, probe := range []struct {
			name  string
			query string
		}{
			{"the reconciliation leases", `select count(*) from action.reconciliation_leases where tenant_id=$1 and concat_ws('|',
                                definition_id, definition_version, reconciliation_word, external_idempotency_key, request_digest, correlation_id, lease_token)
                                ~ '(phone|template|\+790000007)'`},
			{"the reconciliation outcomes", `select count(*) from action.reconciliation_outcomes where tenant_id=$1 and concat_ws('|',
                                verdict, correlation_id, external_object_id, external_object_version, details, lease_token)
                                ~ '(phone|template|\+790000007)'`},
			{"the action receipts", `select count(*) from action.action_receipts where tenant_id=$1 and concat_ws('|',
                                outcome, receipt_references::text, details)
                                ~ '(phone|template|\+790000007)'`},
			{"the outbox events", `select count(*) from action.outbox_events where tenant_id=$1 and concat_ws('|',
                                event_type, payload::text)
                                ~ '(phone|template|\+790000007)'`},
		} {
			if n := operationSQLCount(t, probe.query, w.tenant); n != 0 {
				t.Fatalf("%s never carry the material of the typed world, got %d", probe.name, n)
			}
		}
		// The keyed identities of the contour stay the honest 64-hex shape
		// (the lease tokens, the external keys and the request digests).
		if n := operationSQLCount(t, `select (select count(*) from action.reconciliation_leases where tenant_id=$1
                        and (lease_token !~ '^[0-9a-f]{64}$' or external_idempotency_key !~ '^[0-9a-f]{64}$' or request_digest !~ '^[0-9a-f]{64}$'))
                        + (select count(*) from action.reconciliation_outcomes where tenant_id=$1 and lease_token !~ '^[0-9a-f]{64}$')`,
			w.tenant); n != 0 {
			t.Fatalf("the keyed identities of the contour stay the 64-hex shape, got %d", n)
		}
	})

	// Row 18 — the APPEND-ONLY probes: the UPDATE and the DELETE of the
	// three append-only tables of the contour refuse on the runtime role
	// EVEN UNDER the honest tenant setting (the grants are absent — the
	// reconciliation history is never rewritten, the audit stays one row
	// per attempt, AC-12; the dispatch row-27 precedent over the 00004
	// surface).
	t.Run("row-18 the append-only probes: UPDATE and DELETE refuse", func(t *testing.T) {
		var liveOperation id.UUID
		if err := adminPool.QueryRow(ctx, `select operation_id from action.reconciliation_leases where tenant_id=$1 limit 1`,
			w.tenant).Scan(&liveOperation); err != nil {
			t.Fatal(err)
		}
		for _, probe := range []struct {
			query string
			args  []any
		}{
			{`update action.reconciliation_leases set lease_token=$2 where tenant_id=$1 and operation_id=$3`, []any{w.tenant, strings.Repeat("a", 64), liveOperation}},
			{`delete from action.reconciliation_leases where tenant_id=$1 and operation_id=$2`, []any{w.tenant, liveOperation}},
			{`update action.reconciliation_outcomes set details='rewritten' where tenant_id=$1 and operation_id=$2`, []any{w.tenant, liveOperation}},
			{`delete from action.reconciliation_outcomes where tenant_id=$1 and operation_id=$2`, []any{w.tenant, liveOperation}},
			{`update action.action_receipts set details='rewritten' where tenant_id=$1 and operation_id=$2`, []any{w.tenant, liveOperation}},
			{`delete from action.action_receipts where tenant_id=$1 and operation_id=$2`, []any{w.tenant, liveOperation}},
		} {
			if err := dispatchRuntimeRefuses(t, w.tenant, probe.query, probe.args...); err == nil {
				t.Fatalf("%s must refuse on the append-only surface (the grants are absent)", probe.query)
			}
		}
	})

	// Row 19 — the RLS isolation of the new tables: the runtime role
	// reads the reconciliation leases and outcomes ONLY under the honest
	// TenantContext — the blind read WITHOUT the tenant setting sees
	// NOTHING (the RLS mirror of the 404 gate) and the FOREIGN tenant
	// session sees nothing of the world rows either (the membership
	// blindness, AD-11; the dispatch row-26 blind-read precedent).
	t.Run("row-19 the RLS isolation: the blind read and the foreign scope see nothing", func(t *testing.T) {
		for _, table := range []string{"reconciliation_leases", "reconciliation_outcomes"} {
			var blind int
			if err := runtimePool.QueryRow(ctx, `select count(*) from action.`+table+` where tenant_id=$1`, w.tenant).Scan(&blind); err != nil {
				t.Fatal(err)
			}
			if blind != 0 {
				t.Fatalf("action.%s without the tenant setting sees NOTHING, got %d", table, blind)
			}
		}
		scoped := func(setting id.UUID, table string) int {
			t.Helper()
			tx, err := runtimePool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if _, err := tx.Exec(ctx, "select set_config('staffy.tenant_id', $1, true)", setting.String()); err != nil {
				t.Fatal(err)
			}
			var n int
			if err := tx.QueryRow(ctx, `select count(*) from action.`+table+` where tenant_id=$1`, w.tenant).Scan(&n); err != nil {
				t.Fatal(err)
			}
			return n
		}
		foreign := id.MustParse("01994f64-6c00-7000-8000-00000000b977")
		for _, table := range []string{"reconciliation_leases", "reconciliation_outcomes"} {
			if n := scoped(foreign, table); n != 0 {
				t.Fatalf("action.%s under the foreign tenant scope sees NOTHING of the world rows, got %d", table, n)
			}
			if n := scoped(w.tenant, table); n == 0 {
				t.Fatalf("action.%s under the own tenant scope sees the honest rows of the world", table)
			}
		}
	})

	// Row 20 — the CHECK backstops of the frozen verdict dictionary: the
	// out-of-dictionary verdict word ('bogus') refuses at the database
	// boundary from BOTH the admin probe and the runtime role under the
	// honest tenant setting (a tenant configuration can never widen the
	// four-word verdict dictionary — the loyal store backstop of the
	// domain guards; the dispatch row-26 CHECK precedent).
	t.Run("row-20 the CHECK backstops: the foreign verdict word refuses", func(t *testing.T) {
		var liveOperation id.UUID
		var liveToken string
		var liveAttempt int
		if err := adminPool.QueryRow(ctx, `select l.operation_id, l.lease_token, l.attempt_no from action.reconciliation_leases l
                        where l.tenant_id=$1 and not exists (
                                select 1 from action.reconciliation_outcomes o
                                where o.tenant_id=l.tenant_id and o.operation_id=l.operation_id and o.attempt_no=l.attempt_no)
                        limit 1`, w.tenant).Scan(&liveOperation, &liveToken, &liveAttempt); err != nil {
			t.Fatal(err)
		}
		insert := `insert into action.reconciliation_outcomes (tenant_id, operation_id, attempt_no, lease_token, verdict,
                        correlation_id, external_object_id, external_object_version, external_object_time,
                        details, created_at)
                        values ($1, $2, $3, $4, 'bogus', '', '', '', null, 'probe', $5)`
		if _, err := adminPool.Exec(ctx, insert, w.tenant, liveOperation, liveAttempt, liveToken, actionReconcileNow); err == nil {
			t.Fatal("the foreign verdict word refuses at the CHECK backstop (the admin probe)")
		}
		if err := dispatchRuntimeRefuses(t, w.tenant, insert, w.tenant, liveOperation, liveAttempt, liveToken, actionReconcileNow); err == nil {
			t.Fatal("the foreign verdict word refuses at the CHECK backstop (the runtime probe under the honest tenant setting)")
		}
	})

	// Row 21 — the DIGEST DIVERGENCE of the retry-unlock: the failed
	// target with the PROVEN no_effect (the row-2 shape) unlocks the
	// retry — but the RECORDED trace of the first attempt (the request
	// digest of the FIRST dispatch lease — seeded through the REAL store
	// transaction with a FOREIGN digest: the append-only surface forbids
	// any rewrite and the live ladder would only ever record the honest
	// digest, the store-API seeding is the honest way to leave the
	// diverged trace behind, the dispatch row-20 precedent) no longer
	// matches the rebuilt digest of the immutable rows: another payload
	// with the same external key is the integrity alarm — the corruption
	// is never masked, the send NEVER happens, no new lease lands (the
	// store-level birth of the unknown verdict through the REAL fenced
	// transaction, then the LIVE no_effect verdict, then the drain).
	t.Run("row-21 the retry-unlock digest conflict: the integrity alarm, no send", func(t *testing.T) {
		op := reconcileAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 21, authorizeMatrixUnlimited,
			map[string]string{"phone": "+79000000721"}, nil, nil, "rec-live-key-21", 3600, actionReconcileNow)
		// The seeded first lease of the previous worker: the honest external
		// key, the FOREIGN 64-hex digest — the recorded trace the unlocked
		// retry will compare against.
		seedAt := actionReconcileNow.Add(57*time.Minute + 30*time.Second)
		token := dispatchSeedLeaseWithDigest(t, w, owner, op, 2, seedAt, seedAt.Add(35*time.Second), strings.Repeat("ab", 32))
		// The seeded unknown verdict of the broken call (the honest shape of
		// the previous worker: the write-ahead lease, the unknown ledger
		// word, the receipt v1 unknown, NO outcome row) through the REAL
		// fenced verdict transaction.
		if err := actionStore.NewPostgres(runtimePool).RecordDispatchOutcome(ctx, actionPort.RecordDispatchOutcomeParams{
			Scope:       reconcileScope,
			Operation:   op,
			AttemptNo:   1,
			LeaseToken:  token,
			OutcomeWord: "",
			StateWord:   actionDomain.OperationUnknown,
			ReceiptWord: actionDomain.ReceiptOutcomeUnknown,
			LedgerEntry: actionDomain.OperationLedgerEntry{
				EntryNo: 3, Kind: actionDomain.OperationUnknown,
				Details: "the seeded unknown verdict of the digest-conflict row",
			},
			ReceiptVersionNo: 1,
			ReceiptReferences: actionPort.ReceiptReferences{
				CaseID: op.CaseID, WorkItemID: op.WorkItemID,
				PreviewID: op.PreviewID, PreviewVersionNo: op.PreviewVersionNo,
				ConfirmationID: op.ConfirmationID, ConfirmationVersionNo: op.ConfirmationVersionNo,
				DefinitionID: op.DefinitionID, DefinitionVersion: op.DefinitionVersion,
			},
			ReceiptDetails: "the seeded unknown receipt of the digest-conflict row",
			OutboxEventID:  id.MustParse("01994f64-6c00-7000-8000-000000009711"),
			OutboxPayload: actionPort.ReceiptCreatedPayload{
				EventType: actionDomain.OutboxEventReceiptCreated, TenantID: w.tenant,
				OperationID: op.OperationID, CaseID: op.CaseID, WorkItemID: op.WorkItemID,
				DefinitionID: op.DefinitionID, ReceiptVersionNo: 1,
				Outcome:   actionDomain.ReceiptOutcomeUnknown,
				CreatedAt: seedAt,
			},
			Now: seedAt.Add(15 * time.Second),
		}); err != nil {
			t.Fatalf("the seeded unknown verdict lands through the real fenced transaction: %v", err)
		}
		// The LIVE no_effect verdict of the proven absence: the failed pair
		// lands and the retry unlocks (the row-2 shape over the seeded
		// trace).
		reconcile.inner.Script(actionDomain.ReconciliationVerdictNoEffect, "", nil)
		reconcile.reset()
		unlock := reconcileAt(58, 30)
		if text := linesOf(unlock, op.OperationID); !strings.Contains(text, actionDomain.DispatchNoEffectUnlockText) {
			t.Fatalf("the no_effect verdict speaks the unlock word of the retry, got %q", text)
		}
		// The unlocked retry rebuilds the payload deterministically and
		// compares the digest with the RECORDED trace — the divergence is
		// the integrity alarm: no send, no new lease, nothing appended.
		dispatch.reset()
		alarm := drainAt(59)
		if text := dispatchLinesOf(alarm, op.OperationID); !strings.Contains(text, "разошедшимся digest") {
			t.Fatalf("the diverged retry alarms with its honest word, got %q", text)
		}
		if dispatch.total() != 0 {
			t.Fatalf("the replay alarm never sends, got %d", dispatch.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("no new lease of the alarmed retry, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 2 {
			t.Fatalf("the alarm appends NO receipt version (v1 unknown, v2 failed), got %d", n)
		}
	})

	// Row 22 — the IMPORT-GRAPH probe (AD-6/AG-4): the production
	// composition root (cmd/staffy-core) never imports the synthetic
	// read-only adapter — the product binary structurally excludes the
	// test-contour door to the provider world; the journeys fixture
	// imports it (the test contour composition root — the only importer
	// outside the integration suite).
	t.Run("row-22 the import graph: the production root excludes the synthetic", func(t *testing.T) {
		deps := func(target string) string {
			t.Helper()
			cmd := exec.Command("go", "list", "-deps", target)
			cmd.Dir = "../.."
			out, err := cmd.Output()
			if err != nil {
				var exit *exec.ExitError
				if errors.As(err, &exit) {
					t.Fatalf("the go list probe of %s must answer: %v: %s", target, err, exit.Stderr)
				}
				t.Fatalf("the go list probe of %s must run: %v", target, err)
			}
			return string(out)
		}
		if core := deps("./cmd/staffy-core"); strings.Contains(core, "internal/adapters/reconciliationsyn") {
			t.Fatal("the production root must never import the synthetic read-only adapter (AG-4)")
		}
		if journeys := deps("./tests/journeys"); !strings.Contains(journeys, "internal/adapters/reconciliationsyn") {
			t.Fatal("the journeys fixture imports the synthetic read-only adapter (the test contour)")
		}
	})

	// Row 23 — the READ-ONLY PORT by structure (epic AC-1): the
	// consumer-owned ReconciliationAdapter port carries NOT A SINGLE
	// mutation method name — the reconciliation never writes to the
	// provider world before the verdict; the two contract methods are
	// the honest surface (the unit-brother reflect probe of
	// internal/action/app/reconcile_test.go over the live matrix).
	t.Run("row-23 the read-only port: no mutation method by structure", func(t *testing.T) {
		adapterType := reflect.TypeOf((*actionPort.ReconciliationAdapter)(nil)).Elem()
		mutationNames := map[string]bool{
			"Set": true, "Write": true, "Create": true, "Update": true, "Delete": true,
			"Record": true, "Append": true, "Acquire": true, "Publish": true, "Send": true,
		}
		seen := map[string]bool{}
		for i := 0; i < adapterType.NumMethod(); i++ {
			method := adapterType.Method(i)
			if mutationNames[method.Name] {
				t.Fatalf("the mutation method name %s is forbidden in the READ-ONLY reconciliation port", method.Name)
			}
			seen[method.Name] = true
		}
		for _, want := range []string{"ServesCapability", "QueryOutcome"} {
			if !seen[want] {
				t.Fatalf("the contract method of the reconciliation port is missing: %s", want)
			}
		}
	})

}

// actionReconcileNow is the matrix clock of the Story 3-7 reconciliation
// matrix (the same hour after the operation clock as the dispatch matrix
// — a SEPARATE world with its own tenant, its own rows and its own id
// pools; the confirmations of the authorize path pin their windows
// around this moment, the drains and the reconciles walk the later
// minutes of the hour so the lease and backoff windows expire honestly
// between the considerations).
var actionReconcileNow = operationNow.Add(1 * time.Hour)

// reconcileTargetID mints the deterministic target reference of one
// matrix row (the 019a3f77 range of 3-7; the 2.9 world owns 019a2f4c,
// the 3-3 world owns 019a3f33, the 3-4 world owns 019a3f44, the 3-5
// world owns 019a3f55, the 3-6 world owns 019a3f66). Distinct target
// refs derive distinct proposal keys and distinct business intents —
// every row owns its fresh chain.
func reconcileTargetID(row int64) id.UUID {
	return id.MustParse(fmt.Sprintf("019a3f77-7000-7000-8000-%012d", row))
}

// reconcileAuthorizedOperation walks the FULL live path to ONE queued
// job of the matrix: the typed-valid proposal, the human confirmation
// (the step-up act, the requested window) and the machine authorization
// — the returned Operation is the immutable row of the queued intent
// (the dispatchAuthorizedOperation precedent with the target refs of
// the 3-7 range).
func reconcileAuthorizedOperation(t *testing.T, w *casesWorld, svc *actionApp.Service, actor identityDomain.Principal,
	caseID, workItemID id.UUID, row int64, def actionDomain.ActionDefinition,
	typed map[string]string, facts, observations []id.UUID, key string, lifetime int, now time.Time) actionDomain.Operation {
	t.Helper()
	built, err := svc.SubmitProposal(context.Background(), previewSubmitCommand(w, actor, caseID, workItemID, reconcileTargetID(row), def, typed, facts, observations, 1))
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := svc.ConfirmPreviewVersion(context.Background(), operationConfirmCommand(w, actor, caseID, built.PreviewID, 1, lifetime, now))
	if err != nil {
		t.Fatal(err)
	}
	authorized, err := svc.AuthorizeOperation(context.Background(), operationAuthorizeCommand(w, actor, caseID, confirmed.ConfirmationID, 1, key, now))
	if err != nil || authorized.Replay || authorized.ReservationReference {
		t.Fatalf("the fresh authorization of the queued intent, got %+v err=%v", authorized, err)
	}
	// The immutable domain row reads back through the REAL store by
	// its client key (the honest row of the queued intent — the card
	// view of the outcome is the flat projection, the store row keeps
	// the full pins for the seeded helpers).
	scope, err := tenancy.New(actor.ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	op, found, err := actionStore.NewPostgres(runtimePool).FindOperationByIdempotencyKey(context.Background(), scope, key)
	if err != nil || !found {
		t.Fatalf("the queued operation reads back by its client key, found=%v err=%v", found, err)
	}
	return op
}

// countingReconciliationAdapter wraps the synthetic READ-ONLY adapter of
// the test contour with the call counter of the fence probes (the
// adapter is NEVER called on a gas — the counter is the live pin of the
// ordered fences; the unit counters pinned the order, the live rows pin
// the words).
type countingReconciliationAdapter struct {
	inner *reconciliationsyn.Synthetic
	calls int64
}

func (c *countingReconciliationAdapter) ServesCapability() string { return c.inner.ServesCapability() }

func (c *countingReconciliationAdapter) QueryOutcome(ctx context.Context, scope tenancy.Context, request actionPort.ReconciliationQueryRequest) (actionPort.ReconciliationQueryResult, error) {
	atomic.AddInt64(&c.calls, 1)
	return c.inner.QueryOutcome(ctx, scope, request)
}

func (c *countingReconciliationAdapter) reset()       { atomic.StoreInt64(&c.calls, 0) }
func (c *countingReconciliationAdapter) total() int64 { return atomic.LoadInt64(&c.calls) }

// actionReconcileWorld builds the matrix world: the cases world of the
// suite plus the REAL reconciliation contour with the counting
// synthetic read-only adapter and the counting synthetic mutation
// adapter (the unknown receipts of the rows are born through the REAL
// dispatch contour — the full 3-6 ladder of the broken call) over the
// verified channel capability word.
func actionReconcileWorld(t *testing.T, seed int) (*casesWorld, *actionApp.Service, *countingMutationAdapter, *countingReconciliationAdapter, *owningsystem.Synthetic, identityDomain.Principal) {
	t.Helper()
	w := newCasesWorld(t, seed)
	// The 512-value id pools of the world (the case commands and the
	// action service share the free 39xx/40xx ranges with the headroom).
	w.cases = w.buildCasesServiceClock(actionPreviewIDs(seed*10+500, 512), fixedClock{operationNow})
	reader := owningsystem.New()
	dispatch := &countingMutationAdapter{inner: actiondispatcher.NewServing("query.trunk-status")}
	reconcile := &countingReconciliationAdapter{inner: reconciliationsyn.NewServing("query.trunk-status")}
	service := w.buildActionReconcileService(dispatch, reconcile, capabilityresolver.New(), reader, actionPreviewIDs(seed*10+400, 512))
	// The operations-scoped operator member of the scoped-control rows
	// (the control Set/Resume demands the operator manage surface).
	identityLive := identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{time.Now().UTC()}, newPrivacyIDs(13700))
	operator := inviteMember(t, identityLive, w.owner, w.tenant, "reconcile-ops@example.org", identityDomain.Operator, []identityDomain.Scope{identityDomain.Operations}, "00000000b901", "00000000b902")
	operator.StepUpVerified = true
	return w, service, dispatch, reconcile, reader, operator
}

// buildActionReconcileService wires the REAL app service of the Story
// 3-7 reconciliation contour over the shared cases world: the FULL
// 3-4/3-5/3-6 wiring (the preparation, the confirmation→authorization,
// the dispatch with its synthetic mutation adapter — the unknown
// receipts of the matrix rows are born here) plus the reconciliation
// wiring — the REAL reconciliation store over the LIVE PG (the
// migration 00004) and the read-only adapter of the test contour (nil =
// the honest AG-4 production refusal). The dictionary is the 3-5/3-6
// operation matrix dictionary (the single_use/unlimited/tolerant
// definitions over the verified channel capability).
func (w *casesWorld) buildActionReconcileService(dispatch actionPort.MutationAdapter, reconcile actionPort.ReconciliationAdapter,
	caps actionDomain.CapabilityRefResolver, reader actionPort.OwningSystemReader, ids *sequenceIDs) *actionApp.Service {
	packs := packresolver.New()
	defs := operationAuthorizationDictionary()
	// The admission walk at construction (the fixture discipline: a
	// dishonest dictionary panics before the first command).
	if err := actionDomain.CatalogIntegrity(defs); err != nil {
		panic("the reconcile matrix dictionary must pass the guard: " + err.Error())
	}
	for _, def := range defs {
		if err := actionDomain.ValidateDefinitionRefs(def, packs, caps); err != nil {
			panic("the reconcile matrix dictionary must resolve against the live catalogs: " + err.Error())
		}
	}
	service := actionApp.New(w.identity, packs, caps, defs).
		WithPreviews(actionStore.NewPostgres(runtimePool),
			controleffectiveresolver.NewRouteAdapter(controlStore.NewPostgres(runtimePool), fixedClock{operationNow}),
			w.cases, w.identity, w.entitlement, w.employee, w.connector, w.employee, ids).
		WithAuthorization(actionStore.NewPostgres(runtimePool), actionStore.NewPostgres(runtimePool)).
		WithDispatch(actionStore.NewPostgres(runtimePool), dispatch).
		WithReconcile(actionStore.NewPostgres(runtimePool), reconcile)
	if reader != nil {
		service = service.WithPreviewReader(reader)
	}
	return service
}

// undeclaredCapabilityResolver wraps the LIVE catalog resolver of the
// world with the DROPPED live reconciliation word: the connector catalog
// keeps its declaration (the binary dictionary of 1-8), the resolver of
// the undeclared service answers the facts WITHOUT the live word — the
// row-3 shape of the connector that stopped declaring the reconciliation
// semantics (the fence reads the LIVE declaration at the query moment;
// the declared query word of the definition alone is never a proof). The
// wrapper satisfies the consumer-owned resolver contract structurally
// (the capabilityresolver precedent).
type undeclaredCapabilityResolver struct {
	inner *capabilityresolver.Adapter
}

// Capability relays the catalog resolution with the blanked live
// reconciliation word (the resolution itself must still succeed — an
// unresolvable capability is a different refusal the admission walk
// owns).
func (r undeclaredCapabilityResolver) Capability(connectorID, connectorVersion, capabilityID string) (actionDomain.CapabilityFacts, bool) {
	facts, ok := r.inner.Capability(connectorID, connectorVersion, capabilityID)
	if ok {
		facts.Reconciliation = ""
	}
	return facts, ok
}

// buildActionReconcileUndeclaredService wires the SECOND local service of
// the row-3 world shape over the SAME live stores of the matrix world:
// the same preparation wiring, the same reconciliation repository and the
// SAME read-only adapter — only the capability resolver drops the live
// reconciliation word and the dictionary carries the definitions whose
// admission never demands it (the compensatable/unsafe shapes; the
// query_reconcilable class demands the declared live word, the guard of
// the admission walk). The operations of the matrix rows were born
// through the FIRST service — this service only ever answers the
// reconcile command of the undeclared fence row (the minimal world
// change of the row: a second local service configuration over the same
// stores, never a change of the production code). The id pool lives in
// the free 4170 range of the world (the first service owns 3970, the
// cases world owns 4070).
func (w *casesWorld) buildActionReconcileUndeclaredService(reconcile actionPort.ReconciliationAdapter, ids *sequenceIDs) *actionApp.Service {
	packs := packresolver.New()
	caps := undeclaredCapabilityResolver{inner: capabilityresolver.New()}
	defs := []actionDomain.ActionDefinition{}
	for _, def := range operationAuthorizationDictionary() {
		if def.DeliveryContract.CapabilityClass == actionDomain.CapabilityQueryReconcilable {
			continue
		}
		defs = append(defs, def)
	}
	// The admission walk at construction (the fixture discipline: a
	// dishonest dictionary panics before the first command).
	if err := actionDomain.CatalogIntegrity(defs); err != nil {
		panic("the undeclared reconcile dictionary must pass the guard: " + err.Error())
	}
	for _, def := range defs {
		if err := actionDomain.ValidateDefinitionRefs(def, packs, caps); err != nil {
			panic("the undeclared reconcile dictionary must resolve against the live catalogs: " + err.Error())
		}
	}
	return actionApp.New(w.identity, packs, caps, defs).
		WithPreviews(actionStore.NewPostgres(runtimePool),
			controleffectiveresolver.NewRouteAdapter(controlStore.NewPostgres(runtimePool), fixedClock{operationNow}),
			w.cases, w.identity, w.entitlement, w.employee, w.connector, w.employee, ids).
		WithReconcile(actionStore.NewPostgres(runtimePool), reconcile)
}
