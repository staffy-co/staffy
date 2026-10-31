//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
	controlAPI "github.com/staffy-co/staffy/internal/control/api"
	controlApp "github.com/staffy-co/staffy/internal/control/app"
	controlDomain "github.com/staffy-co/staffy/internal/control/domain"
	controlStore "github.com/staffy-co/staffy/internal/control/store"
	entitlementAPI "github.com/staffy-co/staffy/internal/entitlement/api"
	entitlementDomain "github.com/staffy-co/staffy/internal/entitlement/domain"
	identityApp "github.com/staffy-co/staffy/internal/identity/app"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	identityStore "github.com/staffy-co/staffy/internal/identity/store"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// TestActionDispatchMatrixRowsBySpec is the Story 3-6 step-04 matrix: the
// honest rows of the execution contour walked by name over the REAL app
// service (the ordered fences of DispatchQueuedOperations — the worker the
// transport drain command calls), the REAL dispatch store over the LIVE PG
// (the migration 00003 — RLS+FORCE on the three tables, the CHECK
// dictionaries, the append-only INSERT/SELECT grants, the advisory lock of
// the lease/verdict transactions) and the synthetic mutation adapter of the
// test contour (scriptable outcome/correlation/evidence/guarantee — the
// journeys fixture precedent, imported ONLY by the test composition roots).
// The rows pin the AC probes BY NAME: the full live path
// authorized→drain→accepted→committed with the receipt v1 row and the
// outbox receipt.created record; the SQL audits of the ONE verdict
// sequence (the lease write-ahead submitted BEFORE the adapter call, the
// outcome+ledger+receipt+outbox of the fenced verdict); every
// confirmation/freshness/policy/material fence by its exact reason word
// with the adapter NEVER called and the lease NEVER taken; the crash
// resting state and the honest unknown verdict on the crashed attempt; the
// bounded retry policy words (park_unknown / park_bound) with the ONE
// external idempotency key stable across the attempts; the stale-holder
// fencing refusal; the racing skip; the at-least-once duplicate redelivery
// verified by the keyed digest (no second send, no new intent); the
// integrity alarms (job schema, fingerprint, replay divergence); the
// evidence refusal (committed without the structural triple is unknown);
// the AG-4 boundaries (the nil adapter drain refusal, the nil reader
// fail-closed material block); the database discipline of 00003 (RLS, the
// grants, the CHECK backstops, the append-only refusals) and the
// import-graph probe (the production root never imports the synthetic).
//
// The honest structural boundary the matrix documents: the retry-SEND
// rows (a second attempt taken over a provider_idempotent definition)
// live only over command.send-sms — the ONLY catalog capability that
// declares Idempotent=true — and a MUTATING capability is never
// probe-verifiable before the Action Gateway exists (the frozen 3-4
// capability gate: capability_not_verified at the authorize contour, and
// capability_evidence_moved at the execute-time freshness fence). The
// live matrix therefore pins the policy words over the LIVE world
// (park_unknown without the guarantee or without the class, park_bound at
// the attempts limit) and the ONE-key-stability over the seeded attempts;
// the retry-send itself is the unit matrix's fake-caps row.
//
// The slot registry (the retro-2-3 discipline): the world seed is 356
// (the free slot of the 3xx decade — the ingest-ladder suite owns every
// x10 seed, the preview matrix owns 344, the operation reservation owns
// 355), the identity ids live in the free b8xx suffix range, the
// operation target refs own the 019a3f66 range (the 2.9 world owns
// 019a2f4c, the 3-3 world owns 019a3f33, the 3-4 world owns 019a3f44,
// the 3-5 world owns 019a3f55), the service id pools live in the 3600
// range with the 512-value headroom.
func TestActionDispatchMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	w, svc, adapter, reader, operator := actionDispatchWorld(t, 356)
	caseID, workItemID := w.acceptedCase(t, "evt-dis-m1", "sess-dis-m1", `{"call":"inbound-call","from":"+79000000601"}`)
	// The invited members of the matrix: the operations-only operator (the
	// honest 403 of the drain scope gate) over the LIVE identity service.
	identityLive := identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{time.Now().UTC()}, newPrivacyIDs(13600))
	auditor := inviteMember(t, identityLive, w.owner, w.tenant, "dispatch-audit@example.org", identityDomain.ExpertQA, []identityDomain.Scope{identityDomain.Audit}, "00000000b805", "00000000b806")
	// The LIVE control service of the freshness rows (the control
	// generation move of the scoped autonomy class route).
	controls := controlApp.New(controlStore.NewPostgres(runtimePool), identityLive, connectorrefresolver.New(), fixedClock{actionDispatchNow}, &sequenceIDs{values: controlMatrixIDs(22)})
	// The honest owner actor of every human command and the drain command
	// (the owner carries the operations scope; the policies read its role).
	owner := w.ownershipActor()
	stranger := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-00000000b816")}
	// The drain command of the matrix: the owner actor, the live tenant,
	// the monotone drain clock (every row drains at its own later moment
	// so the lease windows expire honestly between the considerations).
	drain := func(minute int) actionApp.DispatchDrainReport {
		t.Helper()
		report, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: owner, TenantID: w.tenant, Now: actionDispatchNow.Add(time.Duration(minute) * time.Minute),
		})
		if err != nil {
			t.Fatalf("the drain command of the minute %d must answer its honest report: %v", minute, err)
		}
		return report
	}
	// linesOf collects the spoken lines of one operation's consideration.
	linesOf := func(report actionApp.DispatchDrainReport, operationID id.UUID) string {
		lines := []string{}
		for _, line := range report.Reports {
			if line.OperationID == operationID {
				lines = append(lines, line.Text)
			}
		}
		return strings.Join(lines, " | ")
	}

	// Row 1 — the FULL LIVE PATH: the typed-valid proposal, the human
	// confirm, the machine authorize of a single_use consent (consumed=1 —
	// the documented NO-exhausted decision: the consumption after a
	// successful authorization is the EXPECTED state, the queued intent
	// dispatches freely), then the drain: the write-ahead lease, the ONE
	// adapter call, the fenced committed verdict — the outcome row, the
	// ledger submitted+committed entries, the receipt v1 row with the SAFE
	// references and the outbox receipt.created record; the SQL audits
	// count every table of the ONE verdict sequence.
	t.Run("row-1 the full live path: authorize, drain, committed receipt", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 1, authorizeMatrixSingleUse, map[string]string{"phone": "+79000000604"}, nil, nil, "dis-live-key-1", 3600, actionDispatchNow)
		adapter.inner.Script(actionDomain.MutationOutcomeCommitted, "", nil)
		adapter.inner.ScriptEvidence("ext-dis-42", "7")
		adapter.reset()
		report := drain(1)
		if report.Dispatched != 1 {
			t.Fatalf("the live path dispatches exactly one job, got %+v", report)
		}
		if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.OperationCommittedText) {
			t.Fatalf("the report line speaks the committed life text, got %q", text)
		}
		if adapter.total() != 1 {
			t.Fatalf("the adapter is called EXACTLY once, got %d", adapter.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("exactly one lease row (attempt 1), got %d", n)
		}
		var attemptNo int
		if err := adminPool.QueryRow(ctx, `select attempt_no from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID).Scan(&attemptNo); err != nil || attemptNo != 1 {
			t.Fatalf("the monotonic fencing token of the first attempt is 1, got %d err=%v", attemptNo, err)
		}
		var ledgerOrder string
		if err := adminPool.QueryRow(ctx, `select string_agg(kind, ',' order by entry_no) from action.operation_ledger where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID).Scan(&ledgerOrder); err != nil {
			t.Fatal(err)
		}
		if ledgerOrder != "authorized,submitted,committed" {
			t.Fatalf("the ONE verdict sequence: authorized → submitted (write-ahead) → committed, got %s", ledgerOrder)
		}
		var outcome, externalID string
		var guarantee bool
		if err := adminPool.QueryRow(ctx, `select outcome, external_object_id, guarantee_recorded from action.dispatch_outcomes where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID).Scan(&outcome, &externalID, &guarantee); err != nil {
			t.Fatal(err)
		}
		if outcome != actionDomain.MutationOutcomeCommitted || externalID != "ext-dis-42" || guarantee {
			t.Fatalf("the committed claim with the structural evidence and no guarantee word, got %q %q %v", outcome, externalID, guarantee)
		}
		var receiptOutcome, references string
		if err := adminPool.QueryRow(ctx, `select outcome, receipt_references::text from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID).Scan(&receiptOutcome, &references); err != nil {
			t.Fatal(err)
		}
		if receiptOutcome != actionDomain.ReceiptOutcomeCommitted {
			t.Fatalf("the receipt v1 speaks the deciding committed word, got %q", receiptOutcome)
		}
		for _, forbidden := range []string{"+79000000604", "phone", "template"} {
			if strings.Contains(references, forbidden) {
				t.Fatalf("the SAFE receipt references never carry the material %q, got %s", forbidden, references)
			}
		}
		for _, required := range []string{caseID.String(), workItemID.String(), op.PreviewID.String(), op.ConfirmationID.String(), "ext-dis-42"} {
			if !strings.Contains(references, required) {
				t.Fatalf("the SAFE receipt references carry the %s identity, got %s", required, references)
			}
		}
		var payload string
		if err := adminPool.QueryRow(ctx, `select payload::text from action.outbox_events where tenant_id=$1 and operation_id=$2 and event_type='receipt.created'`, w.tenant, op.OperationID).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"+790000006", "phone", "template", "typed_input"} {
			if strings.Contains(payload, forbidden) {
				t.Fatalf("the material %q never enters the SAFE receipt.created payload, got %s", forbidden, payload)
			}
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded["event_type"] != actionDomain.OutboxEventReceiptCreated || decoded["outcome"] != actionDomain.ReceiptOutcomeCommitted {
			t.Fatalf("the receipt.created payload speaks its event and outcome words, got %s", payload)
		}
		// The ONE external idempotency key of the operation (AC-10): the
		// keyed derivation over {tenant, operation} — deterministic, 64-hex.
		store := actionStore.NewPostgres(runtimePool)
		scope, err := tenancy.New(owner.ID, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		hmacKey, err := store.ReadPreviewKey(ctx, scope, actionDispatchNow)
		if err != nil {
			t.Fatal(err)
		}
		var externalKey string
		if err := adminPool.QueryRow(ctx, `select external_idempotency_key from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID).Scan(&externalKey); err != nil {
			t.Fatal(err)
		}
		if externalKey != actionDomain.DeriveExternalIdempotencyKey(hmacKey, w.tenant, op.OperationID) || len(externalKey) != 64 {
			t.Fatalf("the external key is the keyed derivation over {tenant, operation}, got %s", externalKey)
		}
		// The consumption of the single_use consent is the audit fact —
		// the queued intent dispatched anyway (NO exhausted fence).
		if n := operationConsumedCount(t, w.tenant, op.ConfirmationID, 1); n != 1 {
			t.Fatalf("the single_use consent consumed exactly once, got %d", n)
		}
		// The view card speaks the honest life words: the committed state,
		// the attempt facts and the receipt readback (FR-28/FR-31).
		view, err := svc.ReadAuthorizationView(ctx, owner, w.tenant, caseID, actionDispatchNow.Add(2*time.Minute))
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
		if len(committed.Attempts) != 1 || committed.Attempts[0].Outcome != actionDomain.MutationOutcomeCommitted || !committed.Attempts[0].OutcomeRecorded {
			t.Fatalf("the attempt view carries the committed claim word, got %+v", committed.Attempts)
		}
		if len(committed.Receipts) != 1 || committed.Receipts[0].Outcome != actionDomain.ReceiptOutcomeCommitted || committed.Receipts[0].ExternalObjectID != "ext-dis-42" {
			t.Fatalf("the receipt readback carries the external object identity, got %+v", committed.Receipts)
		}
	})

	// Row 2 — the accepted verdict is NON-TERMINAL: the provider confirmed
	// the request reception — no receipt, no receipt.created record (the
	// business result is not confirmed yet), the ledger kind accepted; the
	// SECOND drain of the decided job is the decided-verify skip (AC-10):
	// the digest matches — no second send, no new intent.
	t.Run("row-2 accepted: non-terminal, the decided repeat skips", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 2, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000605"}, nil, nil, "dis-live-key-2", 3600, actionDispatchNow)
		adapter.inner.Script(actionDomain.MutationOutcomeAccepted, "", nil)
		adapter.reset()
		first := drain(2)
		if first.Dispatched != 1 || !strings.Contains(linesOf(first, op.OperationID), actionDomain.OperationAcceptedText) {
			t.Fatalf("the accepted verdict dispatches with its honest text, got %+v", first)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("accepted carries NO receipt (the business result is not confirmed), got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.outbox_events where tenant_id=$1 and operation_id=$2 and event_type='receipt.created'`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("accepted carries NO receipt.created record, got %d", n)
		}
		adapter.reset()
		second := drain(3)
		if text := linesOf(second, op.OperationID); !strings.Contains(text, actionDomain.DispatchRedeliveryDecidedText) {
			t.Fatalf("the decided repeat answers the honest skip word, got %q", text)
		}
		if adapter.total() != 0 {
			t.Fatalf("the decided repeat NEVER re-sends, got %d adapter calls", adapter.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("no second lease of the decided operation, got %d", n)
		}
	})

	// Row 3 — the adapter CALL FAILURE is the honest unknown WITHOUT a
	// guarantee: the write-ahead lease and the ledger submitted entry are
	// the committed facts that stay (persist-before-emit — the right and
	// the moment of the send were material BEFORE the failed call), NO
	// outcome row (no honest claim), the receipt v1 unknown and the outbox
	// receipt.created record; the repeat drain parks unknown (no recorded
	// guarantee — the blind repeat is forbidden).
	t.Run("row-3 the call failure: unknown without a guarantee", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 3, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000606"}, nil, nil, "dis-live-key-3", 3600, actionDispatchNow)
		adapter.inner.Script("", "", errors.New("адаптер недоступен: обрыв соединения"))
		adapter.reset()
		first := drain(4)
		if first.Dispatched != 1 {
			t.Fatalf("the attempted send counts as dispatched (the lease and the call happened), got %+v", first)
		}
		if adapter.total() != 1 {
			t.Fatalf("the broken adapter was called once, got %d", adapter.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the write-ahead lease survives the failed call, got %d", n)
		}
		var ledgerOrder string
		if err := adminPool.QueryRow(ctx, `select string_agg(kind, ',' order by entry_no) from action.operation_ledger where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID).Scan(&ledgerOrder); err != nil {
			t.Fatal(err)
		}
		if ledgerOrder != "authorized,submitted,unknown" {
			t.Fatalf("the failed call leaves submitted (write-ahead) then unknown, got %s", ledgerOrder)
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_outcomes where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("a broken call records NO outcome row, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2 and outcome='unknown'`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the receipt v1 unknown of the failed call, got %d", n)
		}
		adapter.reset()
		second := drain(5)
		if text := linesOf(second, op.OperationID); !strings.Contains(text, actionDomain.DispatchRetryText(actionDomain.DispatchRetryParkUnknown)) {
			t.Fatalf("the repeat without the recorded guarantee parks unknown, got %q", text)
		}
		if adapter.total() != 0 || operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID) != 1 {
			t.Fatalf("the blind repeat is forbidden: adapter=%d leases=%d", adapter.total(), operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID))
		}
	})

	// Row 4 — the EVIDENCE REFUSAL (State/Authority): the provider claims
	// committed WITHOUT the structural external object triple — the guard
	// refuses the terminal success, the honest unknown lands instead (no
	// outcome row of the refused claim, the receipt v1 unknown, the ledger
	// entry carries the evidence-incomplete word).
	t.Run("row-4 the committed claim without evidence: unknown", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 4, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000607"}, nil, nil, "dis-live-key-4", 3600, actionDispatchNow)
		adapter.inner.Script(actionDomain.MutationOutcomeCommitted, "", nil)
		adapter.inner.ScriptEvidence("", "")
		adapter.reset()
		report := drain(6)
		if report.Dispatched != 1 {
			t.Fatalf("the attempted send counts, got %+v", report)
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_outcomes where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("the refused claim records NO outcome row, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2 and outcome='unknown'`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the receipt v1 unknown of the refused claim, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operation_ledger where tenant_id=$1 and operation_id=$2 and kind='unknown' and details like '%' || $3 || '%'`, w.tenant, op.OperationID, "Evidence неполные"); n != 1 {
			t.Fatalf("the ledger entry speaks the evidence-incomplete word, got %d", n)
		}
	})

	// Row 5 — failed is TERMINAL: the provider refused before any effect —
	// the outcome row failed, the receipt v1 failed, and the repeat drain
	// is the decided skip (NO auto-retry of the same intention — a new
	// launch is a NEW confirmation of the 3-5 contour, AC-8).
	t.Run("row-5 failed: terminal, no auto-retry", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 5, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000608"}, nil, nil, "dis-live-key-5", 3600, actionDispatchNow)
		adapter.inner.Script(actionDomain.MutationOutcomeFailed, "", nil)
		adapter.reset()
		first := drain(7)
		if first.Dispatched != 1 || !strings.Contains(linesOf(first, op.OperationID), actionDomain.OperationFailedText) {
			t.Fatalf("the failed verdict dispatches with its honest text, got %+v", first)
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_outcomes where tenant_id=$1 and operation_id=$2 and outcome='failed'`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the failed outcome row, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2 and outcome='failed'`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the receipt v1 failed, got %d", n)
		}
		adapter.reset()
		second := drain(8)
		if text := linesOf(second, op.OperationID); !strings.Contains(text, actionDomain.DispatchRedeliveryDecidedText) {
			t.Fatalf("the failed repeat is the decided skip — never an auto-retry, got %q", text)
		}
		if adapter.total() != 0 {
			t.Fatalf("failed never re-sends, got %d adapter calls", adapter.total())
		}
	})

	// Row 6 — the TIMEOUT claim records the LIVE guarantee word (the
	// scripted override of the adapter — the honest timeout-with-guarantee
	// audit fact): the outcome row carries guarantee_recorded=true and the
	// receipt v1 unknown; the repeat drain reads the RECORDED word — but
	// the class decides: compensatable is not provider_idempotent, the
	// repeat parks unknown (the recorded guarantee alone never retries).
	t.Run("row-6 the timeout with the recorded guarantee: the class decides", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 6, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000609"}, nil, nil, "dis-live-key-6", 3600, actionDispatchNow)
		adapter.inner.Script(actionDomain.MutationOutcomeTimeout, "", nil)
		adapter.inner.ScriptGuarantee(true)
		adapter.reset()
		first := drain(9)
		if first.Dispatched != 1 {
			t.Fatalf("the timeout attempt counts as dispatched, got %+v", first)
		}
		var guarantee bool
		var outcome string
		if err := adminPool.QueryRow(ctx, `select outcome, guarantee_recorded from action.dispatch_outcomes where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID).Scan(&outcome, &guarantee); err != nil {
			t.Fatal(err)
		}
		if outcome != actionDomain.MutationOutcomeTimeout || !guarantee {
			t.Fatalf("the timeout claim records its guarantee word, got %q %v", outcome, guarantee)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2 and outcome='unknown'`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the receipt v1 unknown of the timeout, got %d", n)
		}
		adapter.reset()
		second := drain(10)
		if text := linesOf(second, op.OperationID); !strings.Contains(text, actionDomain.DispatchRetryText(actionDomain.DispatchRetryParkUnknown)) {
			t.Fatalf("the recorded guarantee without the provider_idempotent class still parks, got %q", text)
		}
		if adapter.total() != 0 {
			t.Fatalf("no retry send of the non-idempotent class, got %d adapter calls", adapter.total())
		}
	})

	// Row 7 — the CRASH RESTING STATE (recoverable): the write-ahead lease
	// of a dead worker (the seeded store transaction: the lease row and
	// the ledger submitted entry, the window expired, NO outcome); the
	// drain reads the LIVE catalog guarantee of the channel capability
	// (query.trunk-status declares Idempotent=false) — the blind repeat is
	// forbidden, the honest unknown verdict lands ON THE CRASHED attempt:
	// the receipt v1 unknown and the outbox receipt.created record, NO
	// outcome row, NO new attempt, the adapter NEVER called (AC-9/AC-11).
	t.Run("row-7 the crash resting state: unknown on the crashed attempt", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 7, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000610"}, nil, nil, "dis-live-key-7", 3600, actionDispatchNow)
		dispatchSeedLease(t, w, owner, op, 2, actionDispatchNow, actionDispatchNow.Add(35*time.Second))
		adapter.reset()
		report := drain(11)
		if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.DispatchBlindRepeatForbiddenText) {
			t.Fatalf("the crashed attempt parks with the blind-repeat-forbidden word, got %q", text)
		}
		if adapter.total() != 0 {
			t.Fatalf("the crashed operation is never re-sent blind, got %d adapter calls", adapter.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("no second attempt of the crashed operation, got %d leases", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_outcomes where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("the crash records NO outcome row (no honest claim), got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.action_receipts where tenant_id=$1 and operation_id=$2 and outcome='unknown'`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the receipt v1 unknown on the crashed attempt, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.outbox_events where tenant_id=$1 and operation_id=$2 and event_type='receipt.created'`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the receipt.created record of the crashed attempt, got %d", n)
		}
	})

	// Row 8 — the BOUNDED retry limit (MaxDispatchAttempts=3): the seeded
	// history of a previous worker — three expired attempts with the
	// timeout outcome and the RECORDED guarantee at attempt 3 — the repeat
	// drain answers park_bound (the bound is checked FIRST, before the
	// class and the guarantee); NO fourth lease is ever taken and the
	// adapter is never called; the three attempts carry ONE external
	// idempotency key (AC-10: one key per operation, stable across the
	// attempts — the provider deduplicates the repeats).
	t.Run("row-8 the attempts bound: park_bound, ONE external key", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 8, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000611"}, nil, nil, "dis-live-key-8", 3600, actionDispatchNow)
		// The seeded attempts of the "previous worker" walk their own
		// honest windows: every next lease lands after the previous
		// one expired (the store refuses an active twin with the
		// racing signal — the seeding never fights the fence).
		token1 := dispatchSeedLease(t, w, owner, op, 2, actionDispatchNow, actionDispatchNow.Add(35*time.Second))
		token2 := dispatchSeedLease(t, w, owner, op, 3, actionDispatchNow.Add(36*time.Second), actionDispatchNow.Add(71*time.Second))
		token3 := dispatchSeedLease(t, w, owner, op, 4, actionDispatchNow.Add(72*time.Second), actionDispatchNow.Add(107*time.Second))
		dispatchSeedOutcome(t, w, owner, op, 3, token3, actionDomain.MutationOutcomeTimeout, actionDomain.OperationUnknown, true, 5, actionDispatchNow.Add(108*time.Second))
		_ = token1
		_ = token2
		adapter.reset()
		report := drain(12)
		if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.DispatchAttemptsBoundText) {
			t.Fatalf("the bound answers the honest park word, got %q", text)
		}
		if adapter.total() != 0 {
			t.Fatalf("the bound never sends a fourth attempt, got %d adapter calls", adapter.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 3 {
			t.Fatalf("exactly three attempts, never a fourth, got %d", n)
		}
		var distinctKeys int
		if err := adminPool.QueryRow(ctx, `select count(distinct external_idempotency_key) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID).Scan(&distinctKeys); err != nil {
			t.Fatal(err)
		}
		if distinctKeys != 1 {
			t.Fatalf("ONE external idempotency key across the attempts, got %d", distinctKeys)
		}
	})

	// Row 9 — the FENCING of the verdict recording (the store's live
	// transactions): the (attempt_no, lease_token) pair must own the
	// CURRENT attempt — a newer attempt refuses the older holder with
	// ErrLeaseStale, a foreign token refuses the same way, and the
	// duplicate outcome of one attempt refuses with ErrOutcomeDuplicate
	// (the append-only audit stays one row per attempt, AC-1).
	t.Run("row-9 fencing: the stale holder never writes history", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 9, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000612"}, nil, nil, "dis-live-key-9", 3600, actionDispatchNow)
		// The second seeded attempt lands after the first window
		// expired (the honest seeding of the newer holder).
		token1 := dispatchSeedLease(t, w, owner, op, 2, actionDispatchNow, actionDispatchNow.Add(35*time.Second))
		token2 := dispatchSeedLease(t, w, owner, op, 3, actionDispatchNow.Add(36*time.Second), actionDispatchNow.Add(71*time.Second))
		store := actionStore.NewPostgres(runtimePool)
		scope, err := tenancy.New(owner.ID, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.RecordDispatchOutcome(ctx, actionPort.RecordDispatchOutcomeParams{
			Scope: scope, Operation: op, AttemptNo: 1, LeaseToken: token1,
			OutcomeWord: actionDomain.MutationOutcomeAccepted, StateWord: actionDomain.OperationAccepted,
			LedgerEntry:      actionDomain.OperationLedgerEntry{EntryNo: 4, Kind: actionDomain.OperationAccepted, Details: "fencing probe"},
			ReceiptVersionNo: 1, OutboxEventID: id.MustParse("01994f64-6c00-7000-8000-00000000b831"), Now: actionDispatchNow,
		}); !errors.Is(err, actionPort.ErrLeaseStale) {
			t.Fatalf("the older holder behind a newer attempt refuses, got %v", err)
		}
		if err := store.RecordDispatchOutcome(ctx, actionPort.RecordDispatchOutcomeParams{
			Scope: scope, Operation: op, AttemptNo: 2, LeaseToken: strings.Repeat("f", 64),
			OutcomeWord: actionDomain.MutationOutcomeAccepted, StateWord: actionDomain.OperationAccepted,
			LedgerEntry:      actionDomain.OperationLedgerEntry{EntryNo: 4, Kind: actionDomain.OperationAccepted, Details: "fencing probe"},
			ReceiptVersionNo: 1, OutboxEventID: id.MustParse("01994f64-6c00-7000-8000-00000000b832"), Now: actionDispatchNow,
		}); !errors.Is(err, actionPort.ErrLeaseStale) {
			t.Fatalf("the foreign token refuses, got %v", err)
		}
		if err := store.RecordDispatchOutcome(ctx, actionPort.RecordDispatchOutcomeParams{
			Scope: scope, Operation: op, AttemptNo: 2, LeaseToken: token2,
			OutcomeWord: actionDomain.MutationOutcomeAccepted, StateWord: actionDomain.OperationAccepted,
			LedgerEntry:      actionDomain.OperationLedgerEntry{EntryNo: 4, Kind: actionDomain.OperationAccepted, Details: "fencing probe"},
			ReceiptVersionNo: 1, OutboxEventID: id.MustParse("01994f64-6c00-7000-8000-00000000b833"), Now: actionDispatchNow,
		}); err != nil {
			t.Fatalf("the current holder records its verdict: %v", err)
		}
		if err := store.RecordDispatchOutcome(ctx, actionPort.RecordDispatchOutcomeParams{
			Scope: scope, Operation: op, AttemptNo: 2, LeaseToken: token2,
			OutcomeWord: actionDomain.MutationOutcomeAccepted, StateWord: actionDomain.OperationAccepted,
			LedgerEntry:      actionDomain.OperationLedgerEntry{EntryNo: 5, Kind: actionDomain.OperationAccepted, Details: "fencing probe"},
			ReceiptVersionNo: 1, OutboxEventID: id.MustParse("01994f64-6c00-7000-8000-00000000b834"), Now: actionDispatchNow,
		}); !errors.Is(err, actionPort.ErrOutcomeDuplicate) {
			t.Fatalf("the verdict of one attempt never lands twice, got %v", err)
		}
	})

	// Row 10 — the RACING skip: an ACTIVE un-expired lease of another
	// consideration owns the operation (the seeded store transaction) —
	// the drain answers the honest skip word, takes NO second right to
	// send, writes NOTHING (no ledger append of the racing skip).
	t.Run("row-10 racing: the active lease of another consideration", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 10, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000613"}, nil, nil, "dis-live-key-10", 3600, actionDispatchNow)
		dispatchSeedLease(t, w, owner, op, 2, actionDispatchNow, actionDispatchNow.Add(2*time.Hour))
		adapter.reset()
		report := drain(13)
		if text := linesOf(report, op.OperationID); !strings.Contains(text, "Активная lease другого рассмотрения") {
			t.Fatalf("the racing skip speaks its honest word, got %q", text)
		}
		if adapter.total() != 0 {
			t.Fatalf("the racing consideration never calls the adapter, got %d", adapter.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("no second right to send, got %d leases", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operation_ledger where tenant_id=$1 and operation_id=$2 and kind='dispatch_blocked'`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("the racing skip writes NO block entry, got %d", n)
		}
	})

	// Row 11 — the CONFIRMATION fences by name (FR-28): the superseded
	// edit, the human revocation and the expired window each kill the
	// queued intent BEFORE the send — the dispatch_blocked entry with the
	// exact fence word, the adapter never called, the lease never taken,
	// and the operation stays AUTHORIZED (parked with a visible cause,
	// never a false failed externally, AC-2).
	t.Run("row-11 the confirmation fences: superseded, revoked, expired", func(t *testing.T) {
		// superseded: the preview edit builds version 2 and the
		// re-confirmation pins it — the pinned version 1 consent of the
		// queued operation is stale.
		superseded := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 11, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000614"}, nil, nil, "dis-live-key-11a", 3600, actionDispatchNow)
		if _, err := svc.NewPreviewVersionFromEdit(ctx, actionAPI.NewPreviewVersionCommand{
			Actor: owner, TenantID: w.tenant, CaseID: caseID, PreviewID: superseded.PreviewID,
			TypedInput: map[string]string{"phone": "+79000000615"}, Now: actionDispatchNow.Add(2 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, superseded.PreviewID, 2, 3600, actionDispatchNow.Add(3*time.Minute))); err != nil {
			t.Fatal(err)
		}
		adapter.reset()
		report := drain(14)
		if text := linesOf(report, superseded.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText(actionDomain.DispatchFenceSuperseded)) {
			t.Fatalf("the superseded fence speaks its exact word, got %q", text)
		}
		// revoked: the human revocation (the step-up act) kills the queued
		// intent of the still-valid consent.
		revoked := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 12, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000616"}, nil, nil, "dis-live-key-11b", 3600, actionDispatchNow)
		if _, err := svc.RevokeConfirmation(ctx, operationRevokeCommand(w, owner, caseID, revoked.ConfirmationID, 1, actionDispatchNow.Add(time.Minute))); err != nil {
			t.Fatal(err)
		}
		report = drain(15)
		if text := linesOf(report, revoked.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText(actionDomain.DispatchFenceRevoked)) {
			t.Fatalf("the revoked fence speaks its exact word, got %q", text)
		}
		// expired: the 60-second consent window is past at the drain
		// moment — a new send demands a new confirmation.
		expired := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 13, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000617"}, nil, nil, "dis-live-key-11c", 60, actionDispatchNow)
		report = drain(16)
		if text := linesOf(report, expired.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText(actionDomain.DispatchFenceExpired)) {
			t.Fatalf("the expired fence speaks its exact word, got %q", text)
		}
		if adapter.total() != 0 {
			t.Fatalf("no fence ever calls the adapter, got %d calls", adapter.total())
		}
		for _, op := range []actionDomain.Operation{superseded, revoked, expired} {
			if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 0 {
				t.Fatalf("no fence ever takes a lease, got %d", n)
			}
			if n := operationSQLCount(t, `select count(*) from action.operation_ledger where tenant_id=$1 and operation_id=$2 and kind='dispatch_blocked'`, w.tenant, op.OperationID); n < 1 {
				t.Fatalf("the visible dispatch_blocked entry of the parked operation, got %d", n)
			}
		}
		// The parked operation stays AUTHORIZED — never a false failed
		// externally (AC-2): the view card of the superseded operation
		// speaks the authorized life word with the visible block cause.
		view, err := svc.ReadAuthorizationView(ctx, owner, w.tenant, caseID, actionDispatchNow.Add(17*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		for i := range view.Operations {
			if view.Operations[i].OperationID == superseded.OperationID {
				if view.Operations[i].State != actionDomain.OperationAuthorized {
					t.Fatalf("the parked operation stays authorized, got %s", view.Operations[i].State)
				}
				if view.Operations[i].BlockReasonText == "" {
					t.Fatal("the parked operation shows its visible block cause")
				}
			}
		}
	})

	// Row 12 — the CASE/WORK-ITEM liveness fence: the probe operations
	// (the honest store transaction ReserveOperation over the LIVE PG —
	// the dead-case simulation of the world moving on) — a dead case pin
	// and a dead work-item pin each block with the case_closed word
	// BEFORE any send (the queued intent of a finished world dies, FR-28).
	t.Run("row-12 the liveness fence: the dead case, the dead work item", func(t *testing.T) {
		// The base scaffolding operations of the probes settle their
		// own honest live path FIRST (their dispatch leaves the
		// queue — the probes alone remain for the fence row).
		baseA := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 14, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000640"}, nil, nil, "dis-live-key-12a-base", 3600, actionDispatchNow)
		baseB := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 16, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000641"}, nil, nil, "dis-live-key-12b-base", 3600, actionDispatchNow)
		adapter.inner.Script(actionDomain.MutationOutcomeCommitted, "", nil)
		adapter.inner.ScriptEvidence("ext-dis-12a", "1")
		drain(16)
		adapter.reset()
		deadCase := dispatchProbeOperation(t, w, owner, baseA, 15, id.MustParse("01994f64-6c00-7000-8000-00000000b821"), workItemID, "dis-live-key-12a", actionDispatchNow)
		deadWork := dispatchProbeOperation(t, w, owner, baseB, 17, caseID, id.MustParse("01994f64-6c00-7000-8000-00000000b822"), "dis-live-key-12b", actionDispatchNow)
		report := drain(17)
		for _, op := range []actionDomain.Operation{deadCase, deadWork} {
			if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText("case_closed")) {
				t.Fatalf("the liveness fence speaks the case_closed word, got %q", text)
			}
			if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 0 {
				t.Fatalf("the dead world never takes a lease, got %d", n)
			}
		}
		if adapter.total() != 0 {
			t.Fatalf("the dead world never calls the adapter, got %d", adapter.total())
		}
	})

	// Row 13 — the FRESHNESS re-check: the seven-reason dictionary of 3-4
	// reused at the execute time — six live moves (the seventh — the
	// capability evidence — walks LAST in row-22: the rotated secret
	// honestly blocks every later build of the world) each park the queued
	// intent with the EXACT reason word, the adapter never called, the
	// lease never taken.
	t.Run("row-13 the freshness fences: six reasons by name", func(t *testing.T) {
		// fact_changed: the supporting fact version moved after the
		// authorization.
		factCase, factWork := w.acceptedCase(t, "evt-dis-m2", "sess-dis-m2", `{"call":"inbound-call","from":"+79000000618"}`)
		factID := previewConfirmedFact(t, w, factCase, factWork, "order_status", `"готов"`, "evt-dis-m2", "dis-fact-1")
		factOp := dispatchAuthorizedOperation(t, w, svc, owner, factCase, factWork, 20, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000619"}, []id.UUID{factID}, nil, "dis-live-key-13a", 3600, actionDispatchNow)
		previewConfirmedFact(t, w, factCase, factWork, "order_status", `"отгружен"`, "evt-dis-m2", "dis-fact-2")
		adapter.reset()
		if text := linesOf(drain(18), factOp.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText(actionDomain.FreshnessFactChanged)) {
			t.Fatalf("the moved fact answers its exact word, got %q", text)
		}
		// observation_changed: the newer owning-system observation.
		obsCase, obsWork := w.acceptedCase(t, "evt-dis-m3", "sess-dis-m3", `{"call":"inbound-call","from":"+79000000620"}`)
		firstRead := w.observationReadOK(t, w.observationReadCommand(obsCase, obsWork, casesDomain.MaterialFieldPrice, "dis-observation-1"))
		obsOp := dispatchAuthorizedOperation(t, w, svc, owner, obsCase, obsWork, 21, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000621"}, nil, []id.UUID{firstRead.ObservationID}, "dis-live-key-13b", 3600, actionDispatchNow)
		w.observationReadOK(t, w.observationReadCommand(obsCase, obsWork, casesDomain.MaterialFieldPrice, "dis-observation-2"))
		if text := linesOf(drain(19), obsOp.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText(actionDomain.FreshnessObservationChanged)) {
			t.Fatalf("the newer observation answers its exact word, got %q", text)
		}
		// owner_epoch_moved: the ownership transfer resolved to the backup
		// owner moves the live epoch.
		epochCase, epochWork := w.acceptedCase(t, "evt-dis-m4", "sess-dis-m4", `{"call":"inbound-call","from":"+79000000622"}`)
		epochOp := dispatchAuthorizedOperation(t, w, svc, owner, epochCase, epochWork, 22, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000623"}, nil, nil, "dis-live-key-13c", 3600, actionDispatchNow)
		request, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: epochCase, ExpectedEpoch: 1,
			ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
			WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "передача владения на время отпуска",
			IdempotencyKey: "dis-transfer-1", Now: actionDispatchNow,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
			Actor: w.backupActor(), TenantID: w.tenant, CaseID: epochCase, RequestID: request.TransferRequestID,
			Decision: casesDomain.TransferResolutionAcknowledged, Reason: "принимаю владение кейсом",
			IdempotencyKey: "dis-transfer-2", Now: actionDispatchNow.Add(time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		if text := linesOf(drain(20), epochOp.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText(actionDomain.FreshnessOwnerEpochMoved)) {
			t.Fatalf("the moved epoch answers its exact word, got %q", text)
		}
		// control_generation_moved: the scoped control of the autonomy
		// class route moved (the freshness fence precedes the policy
		// fence — the moved pin is named first).
		genCase, genWork := w.acceptedCase(t, "evt-dis-m5", "sess-dis-m5", `{"call":"inbound-call","from":"+79000000624"}`)
		genOp := dispatchAuthorizedOperation(t, w, svc, owner, genCase, genWork, 23, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000625"}, nil, nil, "dis-live-key-13d", 3600, actionDispatchNow)
		if _, err := controls.Set(ctx, operator, w.tenant, controlAPI.SetRequest{
			Kind: controlDomain.ScopeKindAutonomyClass, Ref: "assisted", Mode: controlDomain.ControlModePaused,
			Reason:             "смена поколения контроля матрицы 3-6",
			ExpectedGeneration: 0, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b841"),
		}); err != nil {
			t.Fatal(err)
		}
		if text := linesOf(drain(21), genOp.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText(actionDomain.FreshnessControlGenerationMoved)) {
			t.Fatalf("the moved control generation answers its exact word, got %q", text)
		}
		if _, err := controls.Resume(ctx, operator, w.tenant, controlAPI.ResumeRequest{
			Kind: controlDomain.ScopeKindAutonomyClass, Ref: "assisted",
			Reason: "контроль снят", ExpectedGeneration: 1,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b842"),
		}); err != nil {
			t.Fatal(err)
		}
		// release_manifest_moved: the second approved profile version of
		// the world moves the release lineage pin.
		manifestCase, manifestWork := w.acceptedCase(t, "evt-dis-m6", "sess-dis-m6", `{"call":"inbound-call","from":"+79000000626"}`)
		manifestOp := dispatchAuthorizedOperation(t, w, svc, owner, manifestCase, manifestWork, 24, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000627"}, nil, nil, "dis-live-key-13e", 3600, actionDispatchNow)
		previewPublishSecondProfile(t, w)
		if text := linesOf(drain(22), manifestOp.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText(actionDomain.FreshnessReleaseManifestMoved)) {
			t.Fatalf("the published profile answers its exact word, got %q", text)
		}
		// policy_changed: the current policy outcome number moved (the
		// full 2.12 chain over the REAL cases service).
		policyCase, policyWork := w.acceptedCase(t, "evt-dis-m7", "sess-dis-m7", `{"call":"inbound-call","from":"+79000000628"}`)
		policyOp := dispatchAuthorizedOperation(t, w, svc, owner, policyCase, policyWork, 25, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000629"}, nil, nil, "dis-live-key-13f", 3600, actionDispatchNow)
		previewMovePolicyOutcome(t, w, policyCase, policyWork, "sess-dis-m7", "evt-dis-m7-turn-1")
		if text := linesOf(drain(23), policyOp.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText(actionDomain.FreshnessPolicyChanged)) {
			t.Fatalf("the moved policy outcome answers its exact word, got %q", text)
		}
		if adapter.total() != 0 {
			t.Fatalf("no freshness fence ever calls the adapter, got %d calls", adapter.total())
		}
		for _, op := range []actionDomain.Operation{factOp, obsOp, epochOp, genOp, manifestOp, policyOp} {
			if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 0 {
				t.Fatalf("no freshness fence ever takes a lease, got %d", n)
			}
		}
	})

	// Row 14 — the POLICY fence (GuardPreparePolicy reused): the paused
	// commercial perimeter blocks the queued intent with its exact reason
	// word BEFORE the send; the resumed admission heals the world (the
	// parked cause may heal — the fences re-run on every consideration).
	t.Run("row-14 the paused entitlement parks the queued intent", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 26, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000630"}, nil, nil, "dis-live-key-14", 3600, actionDispatchNow)
		if _, err := w.entitlement.RunStateTransition(ctx, entitlementAPI.StateCommand{
			Actor: w.owner, TenantID: w.tenant, Action: entitlementDomain.EventPause,
			Basis: "сезонная приостановка на время инвентаризации отправок", Confirm: true,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b861"),
		}); err != nil {
			t.Fatal(err)
		}
		adapter.reset()
		report := drain(24)
		if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText(actionDomain.PolicyReasonEntitlementBlocked)) {
			t.Fatalf("the paused entitlement speaks its exact word, got %q", text)
		}
		if adapter.total() != 0 || operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID) != 0 {
			t.Fatalf("the policy block never sends: adapter=%d", adapter.total())
		}
		if _, err := w.entitlement.RunStateTransition(ctx, entitlementAPI.StateCommand{
			Actor: w.owner, TenantID: w.tenant, Action: entitlementDomain.EventResume,
			Basis: "инвентаризация завершена, доступ возобновлён", Confirm: true,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b862"),
		}); err != nil {
			t.Fatal(err)
		}
		// The parked cause healed — the fences re-run and the queued intent
		// dispatches on the next consideration (the honest retry-of-blocked
		// semantics); the human revocation then parks it for good (the
		// world stays honest for the later rows).
		adapter.inner.Script(actionDomain.MutationOutcomeCommitted, "", nil)
		adapter.inner.ScriptEvidence("ext-dis-14", "2")
		adapter.reset()
		healed := drain(25)
		if text := linesOf(healed, op.OperationID); !strings.Contains(text, actionDomain.OperationCommittedText) {
			t.Fatalf("the healed cause re-opens the dispatch, got %q", text)
		}
		if adapter.total() != 1 {
			t.Fatalf("the healed operation dispatches once, got %d", adapter.total())
		}
		if _, err := svc.RevokeConfirmation(ctx, operationRevokeCommand(w, owner, caseID, op.ConfirmationID, 1, actionDispatchNow.Add(26*time.Minute))); err != nil {
			t.Fatal(err)
		}
	})

	// Row 15 — the LAST material check (FreshReads, AC-3/AG-4): the stale
	// status of the required field parks the queued intent with the
	// material_condition_changed word; the DECLARED tolerance of the
	// definition passes the stale read EXPLICITLY as allowed_uncertain —
	// the authorized uncertainty dispatches (the explicit pass, never a
	// silent one).
	t.Run("row-15 the material fences: stale blocks, tolerance dispatches", func(t *testing.T) {
		staleOp := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 27, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000631"}, nil, nil, "dis-live-key-15a", 3600, actionDispatchNow)
		tolerantOp := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 28, authorizeMatrixTolerant, map[string]string{"phone": "+79000000632"}, nil, nil, "dis-live-key-15b", 3600, actionDispatchNow)
		reader.SetStatus("order_status", actionDomain.SnapshotStale)
		adapter.inner.Script(actionDomain.MutationOutcomeCommitted, "", nil)
		adapter.inner.ScriptEvidence("ext-dis-15", "3")
		adapter.reset()
		report := drain(26)
		if text := linesOf(report, staleOp.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText("material_condition_changed")) {
			t.Fatalf("the stale material answers its exact word, got %q", text)
		}
		if text := linesOf(report, tolerantOp.OperationID); !strings.Contains(text, actionDomain.OperationCommittedText) {
			t.Fatalf("the declared tolerance dispatches the allowed uncertainty, got %q", text)
		}
		if adapter.total() != 1 {
			t.Fatalf("only the tolerant operation dispatches, got %d calls", adapter.total())
		}
		reader.SetStatus("order_status", actionDomain.SnapshotFresh)
		// The stale-parked operation stays honestly parked for the later
		// rows (the human revocation of its consent — the world stays
		// honest after the reader heals).
		if _, err := svc.RevokeConfirmation(ctx, operationRevokeCommand(w, owner, caseID, staleOp.ConfirmationID, 1, actionDispatchNow.Add(27*time.Minute))); err != nil {
			t.Fatal(err)
		}
	})

	// Row 16 — the DESTINATION ALLOWLIST (AC-4): the adapter serving a
	// foreign capability word refuses BEFORE the lease and the call — the
	// honest skip word, nothing written; the main-adapter consideration
	// then dispatches the operation (the world heals).
	t.Run("row-16 the allowlist: the foreign-serving adapter never sends", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 29, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000633"}, nil, nil, "dis-live-key-16", 3600, actionDispatchNow)
		foreignAdapter := &countingMutationAdapter{inner: actiondispatcher.NewServing("command.send-sms")}
		foreignService := w.buildActionDispatchService(foreignAdapter, reader, actionPreviewIDs(3650, 64))
		report, err := foreignService.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: owner, TenantID: w.tenant, Now: actionDispatchNow.Add(28 * time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}
		if text := linesOf(report, op.OperationID); !strings.Contains(text, "Адаптер обслуживает другую capability") {
			t.Fatalf("the allowlist refusal speaks its honest word, got %q", text)
		}
		if foreignAdapter.total() != 0 {
			t.Fatalf("the foreign-serving adapter is never called, got %d", foreignAdapter.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 0 {
			t.Fatalf("the allowlist refusal takes no lease, got %d", n)
		}
		// The world heals: the main-adapter drain dispatches the operation.
		adapter.inner.Script(actionDomain.MutationOutcomeCommitted, "", nil)
		adapter.inner.ScriptEvidence("ext-dis-16", "4")
		adapter.reset()
		if text := linesOf(drain(29), op.OperationID); !strings.Contains(text, actionDomain.OperationCommittedText) {
			t.Fatalf("the main adapter dispatches the operation, got %q", text)
		}
	})

	// Row 17 — the JOB-SCHEMA integrity alarm: the corrupt queue row (the
	// malformed pin form of the at-least-once delivery) is the alarm skip
	// — never a gas, never a send, never a ledger append (a broken
	// machine never touches the world); the record stays for the human.
	t.Run("row-17 the job-schema alarm: the corrupt queue row", func(t *testing.T) {
		var operationID id.UUID
		if err := adminPool.QueryRow(ctx, `select operation_id from action.operations where tenant_id=$1 order by created_at limit 1`, w.tenant).Scan(&operationID); err != nil {
			t.Fatal(err)
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.outbox_events (tenant_id, event_id, event_type, case_id, operation_id, payload, created_at)
                        values ($1, $2, 'operation.queued', $3, $4, '{"event_type":"operation.queued","operation_id":"01994f64-6c00-7000-8000-00000000b861"}'::jsonb, $5)`,
			w.tenant, id.MustParse("01994f64-6c00-7000-8000-00000000b871"), caseID, operationID, actionDispatchNow.Add(30*time.Minute)); err != nil {
			t.Fatal(err)
		}
		adapter.reset()
		report := drain(30)
		found := false
		for _, line := range report.Reports {
			if line.OperationID == operationID && strings.Contains(line.Text, "Схема задания не распознана") {
				found = true
			}
		}
		if !found {
			t.Fatalf("the corrupt job alarms with its honest word, got %+v", report.Reports)
		}
		if adapter.total() != 0 {
			t.Fatalf("the alarm never calls the adapter, got %d", adapter.total())
		}
		// The corrupt row is removed (the queue's ack of the poison
		// message — the admin truth of the test contour).
		if _, err := adminPool.Exec(ctx, `delete from action.outbox_events where tenant_id=$1 and event_id=$2`, w.tenant, id.MustParse("01994f64-6c00-7000-8000-00000000b871")); err != nil {
			t.Fatal(err)
		}
	})

	// Row 18 — the AT-LEAST-ONCE duplicate redelivery (AC-10): the queue
	// redelivers the job of the DECIDED operation (the duplicate outbox
	// row of the same payload — the second delivery unit is LEGAL); the
	// decided-verify rebuilds the payload deterministically and compares
	// the keyed digest with the RECORDED trace of the first attempt — the
	// match is the honest skip (no new send, no new intent, the link to
	// the existing attempt).
	t.Run("row-18 the duplicate redelivery: the decided-verify skip", func(t *testing.T) {
		var operationID id.UUID
		if err := adminPool.QueryRow(ctx, `select operation_id from action.operations where tenant_id=$1 and idempotency_key='dis-live-key-1'`, w.tenant).Scan(&operationID); err != nil {
			t.Fatal(err)
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.outbox_events (tenant_id, event_id, event_type, case_id, operation_id, payload, created_at)
                        select tenant_id, $2, event_type, case_id, operation_id, payload, created_at
                        from action.outbox_events where tenant_id=$1 and operation_id=$3 and event_type='operation.queued' limit 1`,
			w.tenant, id.MustParse("01994f64-6c00-7000-8000-00000000b872"), operationID); err != nil {
			t.Fatal(err)
		}
		adapter.reset()
		report := drain(31)
		matches := 0
		for _, line := range report.Reports {
			if line.OperationID == operationID && strings.Contains(line.Text, actionDomain.DispatchRedeliveryDecidedText) {
				matches++
			}
		}
		if matches < 2 {
			t.Fatalf("both delivery units of the decided operation answer the honest skip, got %d lines: %+v", matches, report.Reports)
		}
		if adapter.total() != 0 {
			t.Fatalf("the duplicate redelivery NEVER re-sends, got %d", adapter.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, operationID); n != 1 {
			t.Fatalf("no second lease of the redelivered job, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_outcomes where tenant_id=$1 and operation_id=$2`, w.tenant, operationID); n != 1 {
			t.Fatalf("no second outcome of the redelivered job, got %d", n)
		}
	})

	// Row 19 — the FINGERPRINT integrity alarm: the admin-tampered
	// material digest of the immutable operation row (the corruption
	// probe) diverges from the re-derived pins fingerprint — the alarm
	// skip, nothing written; the heal restores the row.
	t.Run("row-19 the fingerprint alarm: the tampered digest", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 30, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000634"}, nil, nil, "dis-live-key-19", 3600, actionDispatchNow)
		if _, err := adminPool.Exec(ctx, `update action.operations set material_digest=$2 where tenant_id=$1 and operation_id=$3`,
			w.tenant, strings.Repeat("c", 64), op.OperationID); err != nil {
			t.Fatal(err)
		}
		adapter.reset()
		report := drain(32)
		if text := linesOf(report, op.OperationID); !strings.Contains(text, "Целостность операции не подтверждена") {
			t.Fatalf("the tampered digest alarms with its honest word, got %q", text)
		}
		if adapter.total() != 0 {
			t.Fatalf("the alarm never calls the adapter, got %d", adapter.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.operation_ledger where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("the alarm appends NOTHING (a broken machine never touches the world), got %d entries", n)
		}
		if _, err := adminPool.Exec(ctx, `update action.operations set material_digest=$2 where tenant_id=$1 and operation_id=$3`,
			w.tenant, op.MaterialDigest, op.OperationID); err != nil {
			t.Fatal(err)
		}
		// The heal restores the row: the next consideration of the
		// honest job dispatches it through the full live path (the
		// alarm was the skip of ONE pass, never a permanent park).
		adapter.inner.Script(actionDomain.MutationOutcomeCommitted, "", nil)
		adapter.inner.ScriptEvidence("ext-dis-19", "9")
		adapter.reset()
		healed := drain(32)
		if text := linesOf(healed, op.OperationID); !strings.Contains(text, actionDomain.OperationCommittedText) {
			t.Fatalf("the healed operation dispatches on the next consideration, got %q", text)
		}
		if adapter.total() != 1 {
			t.Fatalf("the healed operation sends exactly once, got %d", adapter.total())
		}
	})

	// Row 20 — the decided-verify DIGEST DIVERGENCE: the recorded trace of
	// the first attempt (the seeded foreign request digest of the previous
	// worker's lease) no longer matches the rebuilt digest — the replay
	// alarm (the corruption is never masked, no send).
	t.Run("row-20 the replay divergence: the foreign recorded digest", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 31, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000635"}, nil, nil, "dis-live-key-20", 3600, actionDispatchNow)
		token := dispatchSeedLeaseWithDigest(t, w, owner, op, 2, actionDispatchNow, actionDispatchNow.Add(35*time.Second), strings.Repeat("ab", 32))
		dispatchSeedOutcome(t, w, owner, op, 1, token, actionDomain.MutationOutcomeAccepted, actionDomain.OperationAccepted, false, 3, actionDispatchNow)
		adapter.reset()
		report := drain(33)
		if text := linesOf(report, op.OperationID); !strings.Contains(text, "разошедшимся digest") {
			t.Fatalf("the diverged replay alarms with its honest word, got %q", text)
		}
		if adapter.total() != 0 {
			t.Fatalf("the replay alarm never sends, got %d", adapter.total())
		}
		if n := operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID); n != 1 {
			t.Fatalf("no new lease of the alarmed replay, got %d", n)
		}
	})

	// Row 21 — the LEASE WINDOW arithmetic: the lease outlives the
	// possible in-flight adapter call — the window is the definition
	// timeout (5000ms) plus the 30-second grace (DeriveDispatchLeaseExpiresAt);
	// the SQL probe reads the recorded window of the live rows (no real
	// waiting — the arithmetic IS the contract).
	t.Run("row-21 the lease window: timeout plus grace", func(t *testing.T) {
		var seconds float64
		if err := adminPool.QueryRow(ctx, `select extract(epoch from (lease_expires_at - created_at)) from action.dispatch_leases where tenant_id=$1 order by created_at limit 1`, w.tenant).Scan(&seconds); err != nil {
			t.Fatal(err)
		}
		if seconds != float64(5000+actionDomain.DispatchLeaseGraceMS)/1000 {
			t.Fatalf("the lease window is TimeoutMS+grace (35s), got %v", seconds)
		}
	})

	// Row 22 — the NIL READER of the production root fails CLOSED (AG-4,
	// AC-3): the fresh reads of the owning system are unavailable — the
	// queued intent parks with the fresh_reads_unavailable word; the
	// adapter is never called. The revocation tail parks the probe
	// operation for good (the world stays honest for the later rows).
	t.Run("row-22 the nil reader: the fail-closed material gate", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 32, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000636"}, nil, nil, "dis-live-key-22", 3600, actionDispatchNow)
		nilReaderService := w.buildActionDispatchService(adapter, nil, actionPreviewIDs(3660, 64))
		adapter.reset()
		report, err := nilReaderService.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: owner, TenantID: w.tenant, Now: actionDispatchNow.Add(34 * time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}
		if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText("fresh_reads_unavailable")) {
			t.Fatalf("the nil reader parks with its exact word, got %q", text)
		}
		if adapter.total() != 0 {
			t.Fatalf("the nil reader never dispatches, got %d", adapter.total())
		}
		if _, err := svc.RevokeConfirmation(ctx, operationRevokeCommand(w, owner, caseID, op.ConfirmationID, 1, actionDispatchNow.Add(35*time.Minute))); err != nil {
			t.Fatal(err)
		}
	})

	// Row 23 — the CAPABILITY EVIDENCE freshness (the seventh reason,
	// LAST): the rotated connection secret degrades the stored capability
	// evidence — the proof no longer proves anything about the live
	// secret; the queued intent parks with capability_evidence_moved and
	// the honest spillover blocks every LATER build of the world (the
	// last walk of the matrix: the world does not build previews after
	// the rotation).
	t.Run("row-23 the capability evidence: the rotated secret", func(t *testing.T) {
		op := dispatchAuthorizedOperation(t, w, svc, owner, caseID, workItemID, 33, authorizeMatrixUnlimited, map[string]string{"phone": "+79000000637"}, nil, nil, "dis-live-key-23", 3600, actionDispatchNow)
		if _, err := w.connector.Rotate(ctx, connectorAPI.ConnectCommand{
			Actor: w.owner, TenantID: w.tenant, ConnectionID: w.connection,
			CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000b851"),
			IdempotencyKey:  "dis-rotate-1",
			ExpectedVersion: previewConnectionVersion(t, w),
			Secrets:         map[string]string{"api_key": "sandbox-rotated-dis-key"},
		}); err != nil {
			t.Fatal(err)
		}
		adapter.reset()
		report := drain(36)
		if text := linesOf(report, op.OperationID); !strings.Contains(text, actionDomain.DispatchBlockedText(actionDomain.FreshnessCapabilityEvidenceMoved)) {
			t.Fatalf("the rotated secret answers its exact word, got %q", text)
		}
		if adapter.total() != 0 || operationSQLCount(t, `select count(*) from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, w.tenant, op.OperationID) != 0 {
			t.Fatalf("the degraded evidence never dispatches: adapter=%d", adapter.total())
		}
	})

	// Row 24 — the AG-4 production refusal: the nil mutation adapter of
	// the production root (cmd/staffy-core wires WithDispatch(store, nil))
	// refuses the drain command HONESTLY — the queue is never even read
	// (the records stay queued, zero sends).
	t.Run("row-24 the nil adapter: the honest AG-4 refusal", func(t *testing.T) {
		nilAdapterService := w.buildActionDispatchService(nil, reader, actionPreviewIDs(3670, 64))
		_, err := nilAdapterService.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: owner, TenantID: w.tenant, Now: actionDispatchNow.Add(37 * time.Minute),
		})
		if !errors.Is(err, actionPort.ErrDispatchRefusedAG4) {
			t.Fatalf("the nil adapter refuses with the AG-4 signal, got %v", err)
		}
		if !strings.Contains(err.Error(), "Исполнение не подключено") {
			t.Fatalf("the refusal speaks its honest word, got %s", err.Error())
		}
	})

	// Row 25 — the scope gates of the drain command: the stranger (no
	// membership anywhere) is the honest 404 BEFORE the queue is read;
	// the audit-only member is the honest 403 with the operations scope
	// words (the drain is the operator surface).
	t.Run("row-25 the scope gates: the stranger 404, the auditor 403", func(t *testing.T) {
		_, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: stranger, TenantID: w.tenant, Now: actionDispatchNow.Add(38 * time.Minute),
		})
		if !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the non-member is the honest 404, got %v", err)
		}
		_, err = svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
			Actor: auditor, TenantID: w.tenant, Now: actionDispatchNow.Add(39 * time.Minute),
		})
		if !errors.Is(err, actionDomain.ErrForbidden) || !strings.Contains(err.Error(), "полномочия operations") {
			t.Fatalf("the audit-only member hears the operations scope words, got %v", err)
		}
	})

	// Row 26 — the database discipline of 00003: RLS + FORCE on the three
	// tables, the append-only grants of the runtime role (INSERT/SELECT
	// only — no UPDATE, no DELETE, no TRUNCATE), the blind cross-tenant
	// read without the tenant setting (the RLS mirror of the 404 gate) and
	// the CHECK backstops of the frozen vocabularies (the dishonest knobs
	// refuse at the database boundary).
	t.Run("row-26 the database discipline: RLS, grants, blind read, CHECKs", func(t *testing.T) {
		tables := []string{"dispatch_leases", "dispatch_outcomes", "action_receipts"}
		for _, table := range tables {
			var rls, force bool
			if err := adminPool.QueryRow(ctx, `select relrowsecurity, relforcerowsecurity from pg_class
                                where relnamespace = 'action'::regnamespace and relname = $1`, table).Scan(&rls, &force); err != nil {
				t.Fatal(err)
			}
			if !rls || !force {
				t.Fatalf("action.%s must carry RLS + FORCE (the discipline of 00003), got rls=%v force=%v", table, rls, force)
			}
			for _, grant := range []string{"INSERT", "SELECT", "UPDATE", "DELETE", "TRUNCATE"} {
				var allowed bool
				if err := runtimePool.QueryRow(ctx, `select has_table_privilege('staffy_app', $1, $2)`, "action."+table, grant).Scan(&allowed); err != nil {
					t.Fatal(err)
				}
				if (grant == "INSERT" || grant == "SELECT") != allowed {
					t.Fatalf("%s on action.%s: allowed=%v — the append-only grants", grant, table, allowed)
				}
			}
		}
		var blind int
		if err := runtimePool.QueryRow(ctx, `select count(*) from action.dispatch_leases where tenant_id=$1`, w.tenant).Scan(&blind); err != nil {
			t.Fatal(err)
		}
		if blind != 0 {
			t.Fatal("the runtime role without the tenant setting sees NOTHING — the RLS mirror of the 404 gate")
		}
		var visible int
		if err := adminPool.QueryRow(ctx, `select count(*) from action.dispatch_leases where tenant_id=$1`, w.tenant).Scan(&visible); err != nil {
			t.Fatal(err)
		}
		if visible == 0 {
			t.Fatal("the admin probe sees the honest rows of the tenant")
		}
		// The CHECK backstops over the REAL rows of the matrix material:
		// the foreign outcome word, the foreign receipt outcome, the
		// out-of-bound attempt number and the foreign outbox event word.
		var liveOperation id.UUID
		if err := adminPool.QueryRow(ctx, `select operation_id from action.dispatch_leases where tenant_id=$1 limit 1`, w.tenant).Scan(&liveOperation); err != nil {
			t.Fatal(err)
		}
		var liveToken string
		if err := adminPool.QueryRow(ctx, `select lease_token from action.dispatch_leases where tenant_id=$1 limit 1`, w.tenant).Scan(&liveToken); err != nil {
			t.Fatal(err)
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.dispatch_outcomes (tenant_id, operation_id, attempt_no, lease_token, outcome,
                                correlation_id, external_object_id, external_object_version, external_object_time,
                                guarantee_recorded, details, created_at)
                        values ($1, $2, 3, $3, 'delivered', '', '', '', null, false, 'probe', $4)`,
			w.tenant, liveOperation, liveToken, actionDispatchNow); err == nil {
			t.Fatal("the foreign outcome word refuses at the CHECK backstop")
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.action_receipts (tenant_id, operation_id, receipt_version_no, outcome, receipt_references, details, created_at)
                        values ($1, $2, 1, 'reconciling', '{}'::jsonb, 'probe', $3)`,
			w.tenant, liveOperation, actionDispatchNow); err == nil {
			t.Fatal("the foreign receipt outcome refuses at the CHECK backstop")
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.dispatch_leases (tenant_id, operation_id, attempt_no, lease_token,
                                definition_id, definition_version, external_idempotency_key, request_digest, lease_expires_at, created_at)
                        values ($1, $2, 4, $3, 'x', '1.0.0', $4, $5, $6, $6)`,
			w.tenant, liveOperation, liveToken, strings.Repeat("e", 64), strings.Repeat("f", 64), actionDispatchNow); err == nil {
			t.Fatal("the out-of-bound attempt number refuses at the CHECK backstop")
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.outbox_events (tenant_id, event_id, event_type, case_id, operation_id, payload, created_at)
                        values ($1, $2, 'receipt.updated', $3, $4, '{}'::jsonb, $5)`,
			w.tenant, id.MustParse("01994f64-6c00-7000-8000-00000000b881"), caseID, liveOperation, actionDispatchNow); err == nil {
			t.Fatal("the foreign outbox event word refuses at the CHECK backstop")
		}
	})

	// Row 27 — the APPEND-ONLY probes: the UPDATE and the DELETE of the
	// three tables refuse on the runtime role EVEN UNDER the honest tenant
	// setting (the grants are absent — the rows are never rewritten, the
	// audit stays append-only; AC-12: v1 is never rewritten).
	t.Run("row-27 the append-only probes: UPDATE and DELETE refuse", func(t *testing.T) {
		var liveOperation id.UUID
		if err := adminPool.QueryRow(ctx, `select operation_id from action.dispatch_leases where tenant_id=$1 limit 1`, w.tenant).Scan(&liveOperation); err != nil {
			t.Fatal(err)
		}
		for _, probe := range []struct {
			query string
			args  []any
		}{
			{`update action.dispatch_leases set lease_token=$2 where tenant_id=$1 and operation_id=$3`, []any{w.tenant, strings.Repeat("a", 64), liveOperation}},
			{`delete from action.dispatch_leases where tenant_id=$1 and operation_id=$2`, []any{w.tenant, liveOperation}},
			{`update action.dispatch_outcomes set details='rewritten' where tenant_id=$1 and operation_id=$2`, []any{w.tenant, liveOperation}},
			{`delete from action.dispatch_outcomes where tenant_id=$1 and operation_id=$2`, []any{w.tenant, liveOperation}},
			{`update action.action_receipts set details='rewritten' where tenant_id=$1 and operation_id=$2`, []any{w.tenant, liveOperation}},
			{`delete from action.action_receipts where tenant_id=$1 and operation_id=$2`, []any{w.tenant, liveOperation}},
		} {
			if err := dispatchRuntimeRefuses(t, w.tenant, probe.query, probe.args...); err == nil {
				t.Fatalf("%s must refuse on the append-only surface (the grants are absent)", probe.query)
			}
		}
	})

	// Row 28 — the IMPORT-GRAPH probe (AD-6/AG-4): the production
	// composition root (cmd/staffy-core) never imports the synthetic
	// mutation adapter — the product binary structurally excludes the
	// test-contour door to the provider world; the drain in production
	// honestly refuses (row-24) with the queue pending.
	t.Run("row-28 the import graph: the production root excludes the synthetic", func(t *testing.T) {
		source, err := os.ReadFile("../../cmd/staffy-core/main.go")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(source), "actiondispatcher") {
			t.Fatal("the production root must never import the synthetic mutation adapter (AG-4)")
		}
	})
}

// actionDispatchNow is the drain-command clock of the Story 3-6 matrix
// (one hour after the operation reservation matrix — the confirmations of
// the authorize path pin their windows around this moment, the drains
// walk the later minutes of the hour so the lease windows expire
// honestly between the considerations).
var actionDispatchNow = operationNow.Add(1 * time.Hour)

// dispatchTargetID mints the deterministic target reference of one matrix
// row (the 019a3f66 range of 3-6; the 2.9 world owns 019a2f4c, the 3-3
// world owns 019a3f33, the 3-4 world owns 019a3f44, the 3-5 world owns
// 019a3f55). Distinct target refs derive distinct proposal keys and
// distinct business intents — every row owns its fresh chain.
func dispatchTargetID(row int64) id.UUID {
	return id.MustParse(fmt.Sprintf("019a3f66-7000-7000-8000-%012d", row))
}

// countingMutationAdapter wraps the synthetic mutation adapter of the
// test contour with the call counter of the fence probes (the adapter is
// NEVER called on a gas — the counter is the live pin of the ordered
// fences; the unit counters pinned the order, the live rows pin the
// words).
type countingMutationAdapter struct {
	inner *actiondispatcher.Synthetic
	calls int64
}

func (c *countingMutationAdapter) ServesCapability() string { return c.inner.ServesCapability() }

func (c *countingMutationAdapter) Dispatch(ctx context.Context, scope tenancy.Context, request actionPort.MutationRequest) (actionPort.MutationResult, error) {
	atomic.AddInt64(&c.calls, 1)
	return c.inner.Dispatch(ctx, scope, request)
}

func (c *countingMutationAdapter) reset()       { atomic.StoreInt64(&c.calls, 0) }
func (c *countingMutationAdapter) total() int64 { return atomic.LoadInt64(&c.calls) }

// actionDispatchWorld builds the matrix world: the cases world of the
// suite plus the REAL execution contour with the counting synthetic
// mutation adapter over the verified channel capability word.
func actionDispatchWorld(t *testing.T, seed int) (*casesWorld, *actionApp.Service, *countingMutationAdapter, *owningsystem.Synthetic, identityDomain.Principal) {
	t.Helper()
	w := newCasesWorld(t, seed)
	// The 512-value id pools of the world (the case commands and the
	// action service share the free 36xx range with the headroom).
	w.cases = w.buildCasesServiceClock(actionPreviewIDs(seed*10+500, 512), fixedClock{operationNow})
	reader := owningsystem.New()
	adapter := &countingMutationAdapter{inner: actiondispatcher.NewServing("query.trunk-status")}
	service := w.buildActionDispatchService(adapter, reader, actionPreviewIDs(seed*10+400, 512))
	// The operations-scoped operator member of the scoped-control rows
	// (the control Set/Resume demands the operator manage surface).
	identityLive := identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{time.Now().UTC()}, newPrivacyIDs(13600))
	operator := inviteMember(t, identityLive, w.owner, w.tenant, "dispatch-ops@example.org", identityDomain.Operator, []identityDomain.Scope{identityDomain.Operations}, "00000000b801", "00000000b802")
	operator.StepUpVerified = true
	return w, service, adapter, reader, operator
}

// buildActionDispatchService wires the REAL app service of the Story 3-6
// execution contour over the shared cases world: the FULL 3-4/3-5 wiring
// (the preparation, the confirmation→authorization) plus the dispatch
// wiring — the REAL dispatch store over the LIVE PG (the migration 00003)
// and the mutation adapter of the test contour (nil = the honest AG-4
// production refusal). The dictionary is the 3-5 operation matrix
// dictionary (the single_use/unlimited/tolerant definitions over the
// verified channel capability — the notify.manual-only pattern: the
// channel word is the connection probe, the mutation semantics ride the
// delivery contract).
func (w *casesWorld) buildActionDispatchService(adapter actionPort.MutationAdapter, reader actionPort.OwningSystemReader, ids *sequenceIDs) *actionApp.Service {
	packs, caps := packresolver.New(), capabilityresolver.New()
	defs := operationAuthorizationDictionary()
	// The admission walk at construction (the fixture discipline: a
	// dishonest dictionary panics before the first command).
	if err := actionDomain.CatalogIntegrity(defs); err != nil {
		panic("the dispatch matrix dictionary must pass the guard: " + err.Error())
	}
	for _, def := range defs {
		if err := actionDomain.ValidateDefinitionRefs(def, packs, caps); err != nil {
			panic("the dispatch matrix dictionary must resolve against the live catalogs: " + err.Error())
		}
	}
	service := actionApp.New(w.identity, packs, caps, defs).
		WithPreviews(actionStore.NewPostgres(runtimePool),
			controleffectiveresolver.NewRouteAdapter(controlStore.NewPostgres(runtimePool), fixedClock{operationNow}),
			w.cases, w.identity, w.entitlement, w.employee, w.connector, w.employee, ids).
		WithAuthorization(actionStore.NewPostgres(runtimePool), actionStore.NewPostgres(runtimePool)).
		WithDispatch(actionStore.NewPostgres(runtimePool), adapter)
	if reader != nil {
		service = service.WithPreviewReader(reader)
	}
	return service
}

// dispatchAuthorizedOperation walks the FULL live path to ONE queued job
// of the matrix: the typed-valid proposal, the human confirmation (the
// step-up act, the requested window) and the machine authorization —
// the returned Operation is the immutable row of the queued intent.
func dispatchAuthorizedOperation(t *testing.T, w *casesWorld, svc *actionApp.Service, actor identityDomain.Principal,
	caseID, workItemID id.UUID, row int64, def actionDomain.ActionDefinition,
	typed map[string]string, facts, observations []id.UUID, key string, lifetime int, now time.Time) actionDomain.Operation {
	t.Helper()
	built, err := svc.SubmitProposal(context.Background(), previewSubmitCommand(w, actor, caseID, workItemID, dispatchTargetID(row), def, typed, facts, observations, 1))
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

// dispatchSeedLease writes the WRITE-AHEAD lease of a "previous worker"
// through the REAL store transaction over the LIVE PG (the crash probe:
// the dead worker's right and moment of the send — the honest derived
// keys and digest of the operation, the given expiry window). The
// returned token identifies the seeded holder.
func dispatchSeedLease(t *testing.T, w *casesWorld, actor identityDomain.Principal, op actionDomain.Operation, entryNo int64, now, expires time.Time) string {
	t.Helper()
	return dispatchSeedLeaseWithDigest(t, w, actor, op, entryNo, now, expires, "")
}

// dispatchSeedLeaseWithDigest seeds the lease with the GIVEN request
// digest (the empty word derives the honest digest of the rebuilt
// payload; the foreign 64-hex word is the divergence probe of the
// decided-verify row).
func dispatchSeedLeaseWithDigest(t *testing.T, w *casesWorld, actor identityDomain.Principal, op actionDomain.Operation, entryNo int64, now, expires time.Time, digest string) string {
	t.Helper()
	ctx := context.Background()
	store := actionStore.NewPostgres(runtimePool)
	scope, err := tenancy.New(actor.ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	hmacKey, err := store.ReadPreviewKey(ctx, scope, now)
	if err != nil {
		t.Fatal(err)
	}
	if digest == "" {
		digest = dispatchRebuiltDigest(t, store, scope, op, hmacKey)
	}
	token := fmt.Sprintf("%064d", dispatchSeedCounter.mint())
	if _, err := store.AcquireDispatchLease(ctx, actionPort.AcquireDispatchLeaseParams{
		Scope:                  scope,
		Operation:              op,
		DefinitionID:           op.DefinitionID,
		DefinitionVersion:      op.DefinitionVersion,
		LeaseToken:             token,
		ExternalIDempotencyKey: actionDomain.DeriveExternalIdempotencyKey(hmacKey, w.tenant, op.OperationID),
		RequestDigest:          digest,
		LeaseExpiresAt:         expires,
		LedgerEntry: actionDomain.OperationLedgerEntry{
			EntryNo: entryNo, Kind: actionDomain.LedgerEntrySubmitted, Details: "seeded write-ahead of the probe",
		},
		Now: now,
	}); err != nil {
		t.Fatalf("the seeded lease must land through the real store transaction: %v", err)
	}
	return token
}

// dispatchSeedOutcome records the verdict of a "previous worker" through
// the REAL fenced store transaction over the LIVE PG (the timeout claim
// with the recorded guarantee of the bound row; the accepted claim of
// the divergence row).
func dispatchSeedOutcome(t *testing.T, w *casesWorld, actor identityDomain.Principal, op actionDomain.Operation,
	attemptNo int, token, claim, stateWord string, guarantee bool, entryNo int64, now time.Time) {
	t.Helper()
	store := actionStore.NewPostgres(runtimePool)
	scope, err := tenancy.New(actor.ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordDispatchOutcome(context.Background(), actionPort.RecordDispatchOutcomeParams{
		Scope:             scope,
		Operation:         op,
		AttemptNo:         attemptNo,
		LeaseToken:        token,
		OutcomeWord:       claim,
		StateWord:         stateWord,
		GuaranteeRecorded: guarantee,
		LedgerEntry: actionDomain.OperationLedgerEntry{
			EntryNo: entryNo, Kind: stateWord, Details: "seeded verdict of the probe",
		},
		ReceiptVersionNo: 1,
		OutboxEventID:    id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", dispatchSeedCounter.mint())),
		Now:              now,
	}); err != nil {
		t.Fatalf("the seeded verdict must land through the real fenced transaction: %v", err)
	}
}

// dispatchSeedCounter mints the deterministic probe identities of the
// seeded leases/verdicts (the free b83x suffix range — the 64-decimal
// lease tokens and the outbox event ids of the probe rows).
var dispatchSeedCounter = &dispatchSeedIDs{next: 1}

type dispatchSeedIDs struct{ next int }

func (s *dispatchSeedIDs) mint() int {
	s.next++
	return s.next * 1001
}

// dispatchRebuiltDigest derives the honest keyed digest of the rebuilt
// payload of one operation (the same derivation the worker runs — the
// preview row read live, the definition resolved from the dictionary).
func dispatchRebuiltDigest(t *testing.T, store *actionStore.Postgres, scope tenancy.Context, op actionDomain.Operation, hmacKey []byte) string {
	t.Helper()
	preview, err := store.ReadPinnedPreviewVersion(context.Background(), scope, op.PreviewID, op.PreviewVersionNo)
	if err != nil {
		t.Fatal(err)
	}
	def := dispatchDefinitionOf(op.DefinitionID)
	payload, err := actionDomain.BuildAuthorizedActionPayload(op, preview, def)
	if err != nil {
		t.Fatalf("the seeded digest derives over the honest rebuild: %v", err)
	}
	return actionDomain.DeriveDispatchRequestDigest(hmacKey, payload)
}

// dispatchDefinitionOf resolves one definition of the matrix dictionary
// by its id (the compile-time catalog read of the test contour).
func dispatchDefinitionOf(definitionID string) actionDomain.ActionDefinition {
	for _, def := range operationAuthorizationDictionary() {
		if def.ID == definitionID {
			return def
		}
	}
	panic("the dispatch matrix dictionary must resolve the pinned definition: " + definitionID)
}

// dispatchProbeOperation builds the PROBE operation row of the liveness
// fence (the honest ReserveOperation store transaction over the LIVE PG —
// the row-19 precedent of the 3-5 matrix): the GIVEN already-authorized
// base operation pins the probe copy to the dead case/work-item (the
// dead-case and the dead-work-item simulations of the world moving on)
// with the honest derived identities.
func dispatchProbeOperation(t *testing.T, w *casesWorld, actor identityDomain.Principal, base actionDomain.Operation,
	probeRow int64, probeCaseID, probeWorkItemID id.UUID, key string, now time.Time) actionDomain.Operation {
	t.Helper()
	ctx := context.Background()
	store := actionStore.NewPostgres(runtimePool)
	scope, err := tenancy.New(actor.ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	hmacKey, err := store.ReadPreviewKey(ctx, scope, now)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := store.ReadPinnedPreviewVersion(ctx, scope, base.PreviewID, base.PreviewVersionNo)
	if err != nil {
		t.Fatal(err)
	}
	def := dispatchDefinitionOf(base.DefinitionID)
	probeTarget := dispatchTargetID(probeRow)
	intentKey := actionDomain.DeriveBusinessIntentKey(hmacKey, actionDomain.BusinessIntentInput{
		TenantID: base.TenantID, OwningSystem: def.OwningSystem, DefinitionID: def.ID,
		Target:     actionDomain.ProposalTarget{Kind: base.Target.Kind, Ref: probeTarget},
		TypedInput: preview.TypedInput, Snapshot: preview.Snapshot,
	})
	probe := base
	probe.OperationID = id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", dispatchSeedCounter.mint()))
	probe.CaseID = probeCaseID
	probe.WorkItemID = probeWorkItemID
	probe.Target = actionDomain.ProposalTarget{Kind: base.Target.Kind, Ref: probeTarget}
	probe.BusinessIntentKey = intentKey
	probe.IdempotencyKey = key
	if err := store.ReserveOperation(ctx, actionPort.ReserveOperationParams{
		Scope:           scope,
		Operation:       probe,
		ConsumptionRule: actionDomain.ConsumptionUnlimited,
		ActorID:         actor.ID,
		ConsumedEventID: id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", dispatchSeedCounter.mint())),
		LedgerEntry: actionDomain.OperationLedgerEntry{
			EntryNo: 1, Kind: actionDomain.LedgerEntryAuthorized, Details: "probe row of the liveness fence",
		},
		OutboxEventID: id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", dispatchSeedCounter.mint())),
		OutboxPayload: actionPort.OperationQueuedPayload{
			EventType: actionDomain.OutboxEventOperationQueued, TenantID: w.tenant,
			OperationID: probe.OperationID, CaseID: probeCaseID, WorkItemID: probeWorkItemID,
			DefinitionID: probe.DefinitionID, DefinitionVersion: probe.DefinitionVersion,
			OwningSystem: def.OwningSystem, PreviewID: probe.PreviewID, PreviewVersionNo: probe.PreviewVersionNo,
			ConfirmationID: probe.ConfirmationID, ConfirmationVersionNo: probe.ConfirmationVersionNo,
			TargetKind: probe.Target.Kind, TargetRef: probeTarget,
			OwnerKind: probe.OwnerKind, OwnerRef: probe.OwnerRef, Partition: probe.Partition,
			BusinessIntentKey: intentKey, IdempotencyKey: key, MaterialDigest: probe.MaterialDigest,
			QueuedAt: now.UTC().Truncate(time.Microsecond),
		},
		Now: now,
	}); err != nil {
		t.Fatalf("the probe operation must land through the real store transaction: %v", err)
	}
	return probe
}

// dispatchRuntimeRefuses probes ONE query against the append-only
// surface under the honest tenant setting of the runtime role: the
// UPDATE/DELETE grants are absent, so the query must refuse (the error
// return is the pass; a nil error fails the probe).
func dispatchRuntimeRefuses(t *testing.T, tenant id.UUID, query string, args ...any) error {
	t.Helper()
	ctx := context.Background()
	tx, err := runtimePool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "select set_config('staffy.tenant_id', $1, true)", tenant.String()); err != nil {
		t.Fatal(err)
	}
	_, execErr := tx.Exec(ctx, query, args...)
	return execErr
}
