//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	actionAPI "github.com/artemiosu/staffy/internal/action/api"
	actionApp "github.com/artemiosu/staffy/internal/action/app"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 4.10 (step-04): the order servicing eval matrix — the AC-14 rows
// LIVE over the committed-order world of 4-6/4-7 plus the verification
// decisions and the supplier facts of 4-9 (the Order Servicing Request
// over the recorded material of the contour with the Identity Proof Gate
// over the LIVE 2-3 assurance derivation of the contact case). Every row
// owns its own case: the per-row 4-2..4-5 chain, the builder proposal,
// the confirmation, the authorization, the committed dispatch, the
// approved verification decision — then the assurance state and the
// servicing requests walk the row's honest paths.
// ---------------------------------------------------------------------------

// servicingWorld wires the order world with the servicing contour (the
// same service of the 4-6..4-9 matrices — the WithOrderServicing wiring
// of buildOrderService).
func servicingWorld(t *testing.T, seed int) (*casesWorld, *actionApp.Service, *countingMutationAdapter, identityDomain.Principal, identityDomain.Principal) {
	t.Helper()
	w, svc, adapter, _, _, operator := orderWorld(t, seed)
	backup := identityDomain.Principal{ID: w.backup}
	return w, svc, adapter, operator, backup
}

// servicingRecord runs the passing record command of one row (the honest
// refusal variant answers the error — the caller pins the word).
func servicingRecord(t *testing.T, svc *actionApp.Service, operator identityDomain.Principal, w *casesWorld, caseID id.UUID, intent, kind, value string, now time.Time) actionAPI.OrderServicingOutcome {
	t.Helper()
	outcome, err := svc.RecordOrderServicingRequest(context.Background(), actionAPI.RecordOrderServicingRequestCommand{
		Actor: operator, TenantID: w.tenant, CaseID: caseID,
		Intent: intent, IdentifierKind: kind, IdentifierValue: value, Now: now,
	})
	if err != nil {
		t.Fatalf("the record of the %s request %q refused: %v", intent, value, err)
	}
	return outcome
}

// servicingView reads the servicing view of one row's case.
func servicingView(t *testing.T, svc *actionApp.Service, operator identityDomain.Principal, w *casesWorld, caseID id.UUID) actionDomain.OrderServicingView {
	t.Helper()
	view, err := svc.ReadOrderServicingView(context.Background(), operator, w.tenant, caseID, orderNow.Add(80*time.Minute))
	if err != nil {
		t.Fatalf("the servicing view read of the case %s refused: %v", caseID, err)
	}
	return view
}

// servicingVerify runs ONE identity verification of the row's case (the
// passed method of the voice channel — the LIVE 2-3 derivation input).
func servicingVerify(t *testing.T, w *casesWorld, caseID id.UUID, key string, method, providerCode string, now time.Time) {
	t.Helper()
	conversationID := w.conversationFirst(t, caseID)
	command := w.verificationCommand(caseID, conversationID, key, now)
	command.Kind, command.Method, command.ProviderCode = casesDomain.VerificationKindIdentity, method, providerCode
	command.SubjectHandle, command.Note = "позвонивший", "проверка личности для обслуживания заказа"
	if _, err := w.cases.StartVerification(context.Background(), command); err != nil {
		t.Fatalf("the verification %s refused: %v", key, err)
	}
}

// servicingRequestRows reads the request rows of one case over the admin
// truth (the append-only rows — the SQL backstop of every ladder
// assertion).
func servicingRequestRows(t *testing.T, tenant, caseID id.UUID) []struct {
	RequestNo   int64
	Intent      string
	Kind        string
	Value       string
	Outcome     string
	ProofState  string
	Disclosure  string
	OperationID *id.UUID
} {
	t.Helper()
	rows, err := adminPool.Query(context.Background(),
		`select request_no, intent, identifier_kind, identifier_value, lookup_outcome, proof_state, disclosure, resolved_operation_id
                 from action.order_servicing_requests
                 where tenant_id=$1 and case_id=$2 order by request_no`,
		tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	requests := []struct {
		RequestNo   int64
		Intent      string
		Kind        string
		Value       string
		Outcome     string
		ProofState  string
		Disclosure  string
		OperationID *id.UUID
	}{}
	for rows.Next() {
		var request struct {
			RequestNo   int64
			Intent      string
			Kind        string
			Value       string
			Outcome     string
			ProofState  string
			Disclosure  string
			OperationID *id.UUID
		}
		if err := rows.Scan(&request.RequestNo, &request.Intent, &request.Kind, &request.Value,
			&request.Outcome, &request.ProofState, &request.Disclosure, &request.OperationID); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, request)
	}
	return requests
}

// servicingOperationCount counts the order operations of the case over
// the admin truth (the FR-62 structural pin: the repeat contact NEVER
// creates a new order operation).
func servicingOperationCount(t *testing.T, tenant, caseID id.UUID) int {
	t.Helper()
	var count int
	if err := adminPool.QueryRow(context.Background(),
		`select count(*) from action.operations where tenant_id=$1 and case_id=$2 and definition_id='create_abcp_customer_order'`,
		tenant, caseID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// TestOrderServicingMatrixRowsBySpec walks the Story 4.10 eval matrix
// LIVE: the wrong number (the generic not_found that never discloses
// another client's orders — AC-2/AC-12), the phone kind refusal (OQ-16),
// the spoofed Caller ID (the gate reads ONLY the verification records —
// FR-11 structural), the insufficient basic level (the honest denied +
// the LIVE methods word), the verified status ask (the disclosure of
// the LIVE projection client words — AC-5), the cancel after the sent
// fact (the four states with the supplier touch — AC-7), the
// claim/dispute/human intents (the words of the EXISTING contours
// 3-8/2-13 — AC-9), the trust ladder (the replay at the same state +
// the NEW row after the verification), the FR-62 count pin, the RLS
// blindness and the structural probes (no price column, no send path,
// the append-only surface).
func TestOrderServicingMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	// The world seed 772: the free range of the order family (757, 770 and
	// 771 are taken — the UNIQUE seed keeps the shared-DB worlds
	// collision-free, the seed register of the family).
	w, svc, adapter, operator, backup := servicingWorld(t, 772)

	// Row 1 — the WRONG NUMBER (AC-2/AC-12): the verified case asks for a
	// number that matches NOTHING in the recorded material of the tenant —
	// the outcome not_found with the GENERIC word (never a confirmation or
	// a denial of another client's orders), the request records (AC-6).
	t.Run("row-1 wrong number: the generic not_found word, never a disclosure", func(t *testing.T) {
		caseID, _, _ := statusApprovedOrder(t, w, svc, adapter, operator, backup, "v1", fmtOrderTarget(941))
		servicingVerify(t, w, caseID, "servicing-verify-1", casesDomain.MethodKnownNumberCallback, casesDomain.ProviderCodeOK, orderNow.Add(50*time.Minute))
		outcome := servicingRecord(t, svc, operator, w, caseID, actionDomain.ServicingIntentOrderStatus, actionDomain.LookupKindABCPClientOrderID, "ABCP-ORDER-NOT-EXIST", orderNow.Add(60*time.Minute))
		if outcome.Request.LookupOutcome != actionDomain.LookupOutcomeNone || outcome.Request.ResolvedOperationID != id.UUID("") {
			t.Fatalf("the wrong number outcome: %s %v", outcome.Request.LookupOutcome, outcome.Request.ResolvedOperationID)
		}
		if outcome.Request.Disclosure != actionDomain.DisclosureAllowed || outcome.Request.ProofState != actionDomain.ProofStateVerified {
			t.Fatalf("the gate state of the miss: %s %s", outcome.Request.Disclosure, outcome.Request.ProofState)
		}
		// The client words of the verified miss: the generic outcome word +
		// the intent word — never a disclosure of another client's orders.
		view := servicingView(t, svc, operator, w, caseID)
		if len(view.Requests) != 1 || len(view.Requests[0].ClientWords) != 2 {
			t.Fatalf("the miss words: %+v", view.Requests)
		}
		if view.Requests[0].ClientWords[0] != actionDomain.ServicingLookupNotFoundWord {
			t.Fatalf("the first miss word: %q", view.Requests[0].ClientWords[0])
		}
		if !strings.Contains(view.Requests[0].ClientWords[0], "не найден") {
			t.Fatalf("the generic word: %q", view.Requests[0].ClientWords[0])
		}
	})

	// Row 2 — the PHONE KIND refusal (OQ-16): the phone/email kinds do NOT
	// exist — the guard refuses with the dictionary word; the request
	// NEVER lands (the out-of-dictionary kind is the honest 400).
	t.Run("row-2 phone kind: the search by phone does not exist (OQ-16)", func(t *testing.T) {
		caseID, _, _ := statusApprovedOrder(t, w, svc, adapter, operator, backup, "v2", fmtOrderTarget(942))
		_, err := svc.RecordOrderServicingRequest(context.Background(), actionAPI.RecordOrderServicingRequestCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID,
			Intent: actionDomain.ServicingIntentOrderStatus, IdentifierKind: "phone_number",
			IdentifierValue: "+79000000797", Now: orderNow.Add(60 * time.Minute),
		})
		if err == nil || !strings.Contains(err.Error(), "OQ-16") {
			t.Fatalf("the phone kind refusal: %v", err)
		}
		if got := servicingRequestRows(t, w.tenant, caseID); len(got) != 0 {
			t.Fatalf("the phone kind request landed: %+v", got)
		}
	})

	// Row 3 — the SPOOFED CALLER ID (FR-11 structural): the ingest event
	// carries the caller phone (the from field) but NO verification record
	// exists — the gate reads ONLY the verification records; the request
	// records (AC-6) with the honest denied proof pins and the client
	// words NEVER carry the lookup outcome (the very existence of the
	// order is not disclosed — AC-1).
	t.Run("row-3 spoofed caller id: the gate reads only the verification records", func(t *testing.T) {
		caseID, _, _ := statusApprovedOrder(t, w, svc, adapter, operator, backup, "v3", fmtOrderTarget(943))
		outcome := servicingRecord(t, svc, operator, w, caseID, actionDomain.ServicingIntentOrderStatus, actionDomain.LookupKindABCPClientOrderID, "ABCP-ORDER-v3", orderNow.Add(60*time.Minute))
		if outcome.Request.ProofState != actionDomain.ProofStateNone || outcome.Request.Disclosure != actionDomain.DisclosureDenied {
			t.Fatalf("the unverified gate: %s %s", outcome.Request.ProofState, outcome.Request.Disclosure)
		}
		// The ROW still carries the resolved anchor (the operator's review
		// right) — the disclosure gate governs the CLIENT WORDS only.
		if outcome.Request.LookupOutcome != actionDomain.LookupOutcomeUnique || outcome.Request.ResolvedOperationID == id.UUID("") {
			t.Fatalf("the row outcome of the unverified ask: %+v", outcome.Request)
		}
		// The AC-1 leak probe: the client words carry the gate word + the
		// fallback words ONLY — the lookup outcome and ANY order material
		// NEVER ride them.
		view := servicingView(t, svc, operator, w, caseID)
		if len(view.Requests) != 1 {
			t.Fatalf("the requests of the unverified ask: %d", len(view.Requests))
		}
		joined := strings.Join(view.Requests[0].ClientWords, " ")
		if !strings.Contains(joined, actionDomain.ServicingGateWordPrefix) {
			t.Fatalf("the gate word: %q", joined)
		}
		// The LIVE methods ride the gate word with the dictionary codes —
		// the SAME render the 2-3 assurance panel uses (the method codes of
		// the voice channel: the callback gives verified).
		if !strings.Contains(joined, "known_number_callback") {
			t.Fatalf("the live methods word: %q", joined)
		}
		for _, leak := range []string{
			actionDomain.ServicingLookupNotFoundWord,
			actionDomain.ServicingLookupUniqueWord,
			actionDomain.ServicingLookupAmbiguousWord,
			"заказ клиента создан",
			"ABCP-ORDER-v3",
		} {
			if strings.Contains(joined, leak) {
				t.Fatalf("the denied client words leak %q (AC-1): %q", leak, joined)
			}
		}
	})

	// Row 4 — the INSUFFICIENT BASIC level (AC-1): the passed knowledge
	// check gives basic — the disclosure still refuses; the client words
	// carry the gate word with the LIVE methods of the channel.
	t.Run("row-4 basic level: the honest denied with the live methods word", func(t *testing.T) {
		caseID, _, _ := statusApprovedOrder(t, w, svc, adapter, operator, backup, "v4", fmtOrderTarget(944))
		servicingVerify(t, w, caseID, "servicing-verify-4", casesDomain.MethodKnowledgeCheck, casesDomain.ProviderCodeOK, orderNow.Add(50*time.Minute))
		outcome := servicingRecord(t, svc, operator, w, caseID, actionDomain.ServicingIntentOrderStatus, actionDomain.LookupKindABCPClientOrderID, "ABCP-ORDER-v4", orderNow.Add(60*time.Minute))
		if outcome.Request.ProofState != actionDomain.ProofStateBasic || outcome.Request.Disclosure != actionDomain.DisclosureDenied {
			t.Fatalf("the basic gate: %s %s", outcome.Request.ProofState, outcome.Request.Disclosure)
		}
		view := servicingView(t, svc, operator, w, caseID)
		joined := strings.Join(view.Requests[0].ClientWords, " ")
		if !strings.Contains(joined, "требует проверки личности (уровень verified)") {
			t.Fatalf("the gate word of the basic ask: %q", joined)
		}
	})

	// Row 5 — the VERIFIED STATUS ASK (AC-5): the callback gives the
	// verified level — the disclosure opens; the client words carry the
	// LIVE projection words of the resolved operation (the confirmed
	// stages, the freshness, the boundary words) + the intent boundary
	// word; the request anchors the operation of the receipt.
	t.Run("row-5 verified status: the live projection client words", func(t *testing.T) {
		caseID, _, operation := statusApprovedOrder(t, w, svc, adapter, operator, backup, "v5", fmtOrderTarget(945))
		servicingVerify(t, w, caseID, "servicing-verify-5", casesDomain.MethodKnownNumberCallback, casesDomain.ProviderCodeOK, orderNow.Add(50*time.Minute))
		outcome := servicingRecord(t, svc, operator, w, caseID, actionDomain.ServicingIntentOrderStatus, actionDomain.LookupKindABCPClientOrderID, "ABCP-ORDER-v5", orderNow.Add(60*time.Minute))
		if outcome.Request.LookupOutcome != actionDomain.LookupOutcomeUnique || outcome.Request.ResolvedOperationID != operation.OperationID {
			t.Fatalf("the unique match of the receipt ID: %+v", outcome.Request)
		}
		if outcome.Request.Disclosure != actionDomain.DisclosureAllowed || outcome.Request.ProofState != actionDomain.ProofStateVerified {
			t.Fatalf("the verified gate: %s %s", outcome.Request.Disclosure, outcome.Request.ProofState)
		}
		if outcome.Request.ProofMethod != casesDomain.MethodKnownNumberCallback || outcome.Request.ProofIssuedAt.IsZero() || outcome.Request.ProofExpiresAt.IsZero() {
			t.Fatalf("the proof pins: %s %v %v", outcome.Request.ProofMethod, outcome.Request.ProofIssuedAt, outcome.Request.ProofExpiresAt)
		}
		view := servicingView(t, svc, operator, w, caseID)
		if len(view.Requests) != 1 {
			t.Fatalf("the requests: %d", len(view.Requests))
		}
		joined := strings.Join(view.Requests[0].ClientWords, " ")
		if !strings.Contains(joined, "проверка менеджером подтверждена") {
			t.Fatalf("the live projection words: %q", joined)
		}
		if !strings.Contains(joined, "не заказан у поставщика") {
			t.Fatalf("the live boundary word: %q", joined)
		}
		if !strings.Contains(joined, "только подтверждённые сведения") {
			t.Fatalf("the intent word: %q", joined)
		}
		// The sources of the match ride the view BY NAME (the system
		// receipt of the dispatch machine 4-6).
		if len(view.Requests[0].Sources) != 1 || view.Requests[0].Sources[0] != actionDomain.OrderFactSourceSystemReceipt {
			t.Fatalf("the match sources: %v", view.Requests[0].Sources)
		}
	})

	// Row 6 — the CANCEL AFTER THE SENT FACT (AC-7): the supplier facts
	// {formed, sent} of 4-9 flip the fourth state — «затронул поставщика —
	// человеку»; the four states ride the words; the cancel NEVER executes
	// (the count pin of the operations) and NEVER promises.
	t.Run("row-6 cancel after the sent fact: the four states, the human word", func(t *testing.T) {
		caseID, _, operation := statusApprovedOrder(t, w, svc, adapter, operator, backup, "v6", fmtOrderTarget(946))
		servicingVerify(t, w, caseID, "servicing-verify-6", casesDomain.MethodKnownNumberCallback, casesDomain.ProviderCodeOK, orderNow.Add(50*time.Minute))
		// The supplier facts: formed then sent of the same supplier order ID.
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactFormed, "SUP-46",
			orderNow.Add(30*time.Minute), orderNow.Add(31*time.Minute))
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactSent, "SUP-46",
			orderNow.Add(40*time.Minute), orderNow.Add(41*time.Minute))
		outcome := servicingRecord(t, svc, operator, w, caseID, actionDomain.ServicingIntentCancel, actionDomain.LookupKindSupplierOrderID, "SUP-46", orderNow.Add(60*time.Minute))
		if outcome.Request.LookupOutcome != actionDomain.LookupOutcomeUnique || outcome.Request.ResolvedOperationID != operation.OperationID {
			t.Fatalf("the supplier kind match: %+v", outcome.Request)
		}
		view := servicingView(t, svc, operator, w, caseID)
		if len(view.Requests) != 1 || view.Requests[0].Cancel == nil {
			t.Fatalf("the cancel view: %+v", view.Requests)
		}
		cancel := view.Requests[0].Cancel
		if !cancel.SupplierTouched || !strings.Contains(cancel.NotEditableWord, "затронул поставщика") || !strings.Contains(cancel.NotEditableWord, "передаётся человеку") {
			t.Fatalf("the fourth state with the facts: %+v", cancel)
		}
		if !strings.Contains(cancel.PreparedActionWord, "cancel отсутствует в каталоге") || !strings.Contains(cancel.ConfirmedWord, "не обещается") {
			t.Fatalf("the second/third states: %+v", cancel)
		}
		if !strings.Contains(cancel.ManualContourWord, "НЕ считается отменённым") {
			t.Fatalf("the manual contour word: %q", cancel.ManualContourWord)
		}
		// The FR-62 count pin: the repeat contact NEVER creates a new order
		// operation — the operation count stays ONE.
		if got := servicingOperationCount(t, w.tenant, caseID); got != 1 {
			t.Fatalf("the operation count after the cancel ask: %d", got)
		}
	})

	// Row 7 — the CLAIM/RECOVERY words (AC-9): the claim intent routes to
	// the EXISTING contours 3-8/2-13 BY WORDS — no recovery case is
	// created automatically (the autonomous claim decision is impossible:
	// the machines live in the manager's hands).
	t.Run("row-7 claim: the words of the existing contours, no autonomous machine", func(t *testing.T) {
		caseID, _, _ := statusApprovedOrder(t, w, svc, adapter, operator, backup, "v7", fmtOrderTarget(947))
		servicingVerify(t, w, caseID, "servicing-verify-7", casesDomain.MethodKnownNumberCallback, casesDomain.ProviderCodeOK, orderNow.Add(50*time.Minute))
		outcome := servicingRecord(t, svc, operator, w, caseID, actionDomain.ServicingIntentClaim, actionDomain.LookupKindABCPClientOrderID, "ABCP-ORDER-v7", orderNow.Add(60*time.Minute))
		if outcome.Request.Intent != actionDomain.ServicingIntentClaim {
			t.Fatalf("the claim intent: %s", outcome.Request.Intent)
		}
		view := servicingView(t, svc, operator, w, caseID)
		joined := strings.Join(view.Requests[0].ClientWords, " ")
		for _, pin := range []string{"claim/recovery", "контур 3-8", "контур 2-13", "юридическая ответственность не признаётся", "компенсация не обещается"} {
			if !strings.Contains(joined, pin) {
				t.Fatalf("the claim words miss %q: %q", pin, joined)
			}
		}
		// The autonomous machine probe: NO recovery case was created by the
		// request (the recovery contour lives in the manager's commands).
		var recoveryCount int
		if err := adminPool.QueryRow(context.Background(),
			`select count(*) from action.recovery_cases where tenant_id=$1 and case_id=$2`,
			w.tenant, caseID).Scan(&recoveryCount); err != nil {
			t.Fatal(err)
		}
		if recoveryCount != 0 {
			t.Fatalf("the claim request created a recovery case: %d", recoveryCount)
		}
	})

	// Row 8 — the DISPUTE words (AC-9/AC-14 of 4-9): the dispute intent
	// speaks the append-only history word — the history never rewrites.
	t.Run("row-8 dispute: the append-only history word", func(t *testing.T) {
		caseID, _, _ := statusApprovedOrder(t, w, svc, adapter, operator, backup, "v8", fmtOrderTarget(948))
		servicingVerify(t, w, caseID, "servicing-verify-8", casesDomain.MethodKnownNumberCallback, casesDomain.ProviderCodeOK, orderNow.Add(50*time.Minute))
		servicingRecord(t, svc, operator, w, caseID, actionDomain.ServicingIntentDispute, actionDomain.LookupKindABCPClientOrderID, "ABCP-ORDER-v8", orderNow.Add(60*time.Minute))
		view := servicingView(t, svc, operator, w, caseID)
		joined := strings.Join(view.Requests[0].ClientWords, " ")
		if !strings.Contains(joined, "история не переписывается") {
			t.Fatalf("the dispute words: %q", joined)
		}
	})

	// Row 9 — the HUMAN HANDOFF words (AC-9): the human intent routes to
	// the contour 2-13 with the order anchor word — the owner and the
	// next-update word ride the machine of 2-13 (this slice speaks the
	// routing word only).
	t.Run("row-9 human: the handoff contour words with the order anchor", func(t *testing.T) {
		caseID, _, _ := statusApprovedOrder(t, w, svc, adapter, operator, backup, "v9", fmtOrderTarget(949))
		servicingVerify(t, w, caseID, "servicing-verify-9", casesDomain.MethodKnownNumberCallback, casesDomain.ProviderCodeOK, orderNow.Add(50*time.Minute))
		servicingRecord(t, svc, operator, w, caseID, actionDomain.ServicingIntentHuman, actionDomain.LookupKindABCPClientOrderID, "ABCP-ORDER-v9", orderNow.Add(60*time.Minute))
		view := servicingView(t, svc, operator, w, caseID)
		joined := strings.Join(view.Requests[0].ClientWords, " ")
		for _, pin := range []string{"передача человеку", "контур 2-13", "operation_id", "owner и ожидаемое следующее обновление"} {
			if !strings.Contains(joined, pin) {
				t.Fatalf("the human words miss %q: %q", pin, joined)
			}
		}
	})

	// Row 10 — the TRUST LADDER (AC-4/AC-6): the repeated command at the
	// SAME trust state replays the SAVED request (one row); the
	// verification CHANGES the state — the NEW row lands (two rows, the
	// ladder stays visible); the proof pins of the second row carry the
	// verified method and times.
	t.Run("row-10 trust ladder: the replay at the same state, the new row after the verification", func(t *testing.T) {
		caseID, _, _ := statusApprovedOrder(t, w, svc, adapter, operator, backup, "v10", fmtOrderTarget(950))
		first := servicingRecord(t, svc, operator, w, caseID, actionDomain.ServicingIntentOrderStatus, actionDomain.LookupKindABCPClientOrderID, "ABCP-ORDER-v10", orderNow.Add(60*time.Minute))
		if first.Request.ProofState != actionDomain.ProofStateNone || first.Request.Disclosure != actionDomain.DisclosureDenied {
			t.Fatalf("the first ask: %s %s", first.Request.ProofState, first.Request.Disclosure)
		}
		replay := servicingRecord(t, svc, operator, w, caseID, actionDomain.ServicingIntentOrderStatus, actionDomain.LookupKindABCPClientOrderID, "ABCP-ORDER-v10", orderNow.Add(61*time.Minute))
		if !replay.Replay || replay.Request.RequestNo != first.Request.RequestNo {
			t.Fatalf("the replay: %+v", replay.Request)
		}
		if got := servicingRequestRows(t, w.tenant, caseID); len(got) != 1 {
			t.Fatalf("the rows after the repeat: %+v", got)
		}
		// The verification: the callback gives verified — the SAME command
		// becomes a NEW request (the changed key), the disclosure opens.
		servicingVerify(t, w, caseID, "servicing-verify-10", casesDomain.MethodKnownNumberCallback, casesDomain.ProviderCodeOK, orderNow.Add(62*time.Minute))
		verified := servicingRecord(t, svc, operator, w, caseID, actionDomain.ServicingIntentOrderStatus, actionDomain.LookupKindABCPClientOrderID, "ABCP-ORDER-v10", orderNow.Add(63*time.Minute))
		if verified.Replay || verified.Request.RequestNo != 2 {
			t.Fatalf("the verified ask after the check: %+v", verified.Request)
		}
		if verified.Request.Disclosure != actionDomain.DisclosureAllowed || verified.Request.ProofState != actionDomain.ProofStateVerified {
			t.Fatalf("the verified ask gate: %s %s", verified.Request.Disclosure, verified.Request.ProofState)
		}
		rows := servicingRequestRows(t, w.tenant, caseID)
		if len(rows) != 2 || rows[0].ProofState != actionDomain.ProofStateNone || rows[1].ProofState != actionDomain.ProofStateVerified {
			t.Fatalf("the trust ladder rows: %+v", rows)
		}
		// The view feed carries BOTH rows — the ladder stays visible.
		view := servicingView(t, svc, operator, w, caseID)
		if len(view.Requests) != 2 {
			t.Fatalf("the view ladder: %d", len(view.Requests))
		}
		if strings.Join(view.Requests[0].ClientWords, " ") == strings.Join(view.Requests[1].ClientWords, " ") {
			t.Fatal("the denied and the allowed words are the same — the gate does not live")
		}
	})

	// The STRUCTURAL probes: the RLS blindness (the foreign case answers
	// the honest 404 word), the append-only surface (the UPDATE/DELETE
	// paths are closed for the runtime role) and the price column absence
	// (the purchasing material never enters the servicing table).
	t.Run("probes: rls blindness, append-only surface, no price columns", func(t *testing.T) {
		// The RLS blindness: a case of ANOTHER tenant answers the honest
		// not-found word (the foreign case never enumerates).
		foreign := id.MustParse("01994f64-6c00-7000-8000-00000000f0f1")
		_, err := svc.RecordOrderServicingRequest(context.Background(), actionAPI.RecordOrderServicingRequestCommand{
			Actor: operator, TenantID: w.tenant, CaseID: foreign,
			Intent: actionDomain.ServicingIntentOrderStatus, IdentifierKind: actionDomain.LookupKindABCPClientOrderID,
			IdentifierValue: "ABCP-ORDER-x", Now: orderNow.Add(60 * time.Minute),
		})
		if err == nil || !strings.Contains(err.Error(), "кейс обращения не найден") {
			t.Fatalf("the foreign case refusal: %v", err)
		}
		// The append-only surface: the runtime role can NOT update or
		// delete the request rows (the grants pin).
		ctx := context.Background()
		caseID, _, _ := statusApprovedOrder(t, w, svc, adapter, operator, backup, "vp", fmtOrderTarget(951))
		servicingRecord(t, svc, operator, w, caseID, actionDomain.ServicingIntentOrderStatus, actionDomain.LookupKindABCPClientOrderID, "ABCP-ORDER-vp", orderNow.Add(60*time.Minute))
		rows := servicingRequestRows(t, w.tenant, caseID)
		if len(rows) != 1 {
			t.Fatalf("the probe row: %+v", rows)
		}
		scope := fmt.Sprintf("set local staffy.tenant_id='%s'", w.tenant)
		tx, err := runtimePool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, scope); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `update action.order_servicing_requests set disclosure='allowed' where tenant_id=$1`, w.tenant); err == nil {
			t.Fatal("the update path is OPEN for the runtime role")
		}
		if _, err := tx.Exec(ctx, `delete from action.order_servicing_requests where tenant_id=$1`, w.tenant); err == nil {
			t.Fatal("the delete path is OPEN for the runtime role")
		}
		_ = tx.Rollback(ctx)
		// The price column absence: the servicing table carries NO price
		// material (the purchasing data never enters the contour).
		columns, err := adminPool.Query(ctx,
			`select column_name from information_schema.columns where table_schema='action' and table_name='order_servicing_requests'`)
		if err != nil {
			t.Fatal(err)
		}
		defer columns.Close()
		for columns.Next() {
			var name string
			if err := columns.Scan(&name); err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"price", "cost", "amount", "sum"} {
				if strings.Contains(strings.ToLower(name), forbidden) {
					t.Fatalf("the servicing table carries the price column %q", name)
				}
			}
		}
	})
}
