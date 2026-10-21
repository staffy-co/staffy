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
	"github.com/artemiosu/staffy/internal/adapters/connectorrefresolver"
	"github.com/artemiosu/staffy/internal/adapters/controleffectiveresolver"
	"github.com/artemiosu/staffy/internal/adapters/owningsystem"
	"github.com/artemiosu/staffy/internal/adapters/packresolver"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
	controlAPI "github.com/artemiosu/staffy/internal/control/api"
	controlApp "github.com/artemiosu/staffy/internal/control/app"
	controlDomain "github.com/artemiosu/staffy/internal/control/domain"
	controlStore "github.com/artemiosu/staffy/internal/control/store"
	entitlementAPI "github.com/artemiosu/staffy/internal/entitlement/api"
	entitlementDomain "github.com/artemiosu/staffy/internal/entitlement/domain"
	identityApp "github.com/artemiosu/staffy/internal/identity/app"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	identityStore "github.com/artemiosu/staffy/internal/identity/store"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// TestOperationReservationMatrixRowsBySpec is the Story 3-5 step-04 matrix:
// the honest rows of the confirmation→authorization contour walked by
// name over the REAL app service (the ordered gateway of
// AuthorizeOperation), the REAL append-only consent store and the REAL
// immutable operation store over the LIVE PG (the migration 00002 of the
// module — RLS+FORCE on the six tables, the CHECK dictionaries, the
// append-only INSERT/SELECT grants, the advisory lock of the consent
// serialization), the REAL preparation contour of the same service (the
// SAME preview rows — the digest re-check compares the REAL built rows
// byte-exact) and the LIVE fact sources of the cases world. The rows pin
// the AC-12 probes BY NAME: the double-click and the webhook repeat of
// one client key are the replay of the ORIGINAL operation (the
// consumption never repeats, the second outbox record never exists), the
// crash-before-commit injection leaves NOTHING (no consumption, no
// reservation, no operation, no ledger, no outbox — and the repeat of
// the request is clean), the crash-after-commit repeat is the honest
// replay, the rejection ladder walks the live mutations of one state
// each WITHOUT consuming (the superseded edit, the revocation with the
// step-up, the expired window of the live clock, the exhausted
// single_use allowance), the cross-case loser of the equivalent
// business intent reads the honest reference while its consumption NEVER
// happened, the race of one client key lands exactly ONE operation
// through the reservation backstop, the seven freshness reasons answer
// their exact texts against the LIVE source moves, the nil reader fails
// CLOSED (AG-4) and the declared tolerance passes the stale read as the
// explicit allowed_uncertain word, the database discipline of 00002 is
// pinned (RLS, the grants, the CHECK backstops) and the outbox payload
// is SAFE (ids, pins, words and the two key identities — the material
// NEVER leaves the row).
//
// The slot registry (the retro-2-3 discipline): the world seed is 355
// (the free slot of the 35x decade — the honest correction of the 3-4
// comment that promised 350: the ingest-ladder suite OWNS every x10
// seed of the 3xx range — 310..390 all live in cases_test.go, the
// preview matrix owns 344), the identity
// ids live in the free b7xx suffix range, the operation target refs own
// the 019a3f55 range (the 2.9 world owns 019a2f4c, the 3-3 world owns
// 019a3f33, the 3-4 world owns 019a3f44), the service id pools live in
// the 3500 range with the 512-value headroom.
func TestOperationReservationMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	// The LIVE clock of the consent window: the confirm/authorize commands
	// pin the moments here so the read-time derivations of the view (the
	// state word of the confirmation cards, the valid_until audit) stay
	// coherent with the system clock the honest view derives from — the
	// window is the SERVER truth and the matrix moves it by the command
	// clock, never by the wall.
	liveNow := time.Now().UTC().Truncate(time.Microsecond)
	w, svc, reader, countingReader := operationReservationWorld(t, 355)
	caseID, workItemID := w.acceptedCase(t, "evt-op-m1", "sess-op-m1", `{"call":"inbound-call","from":"+79000000501"}`)
	// The invited members of the matrix: the operations-only operator (the
	// LIVE control actor of the policy rows) and the audit-only member (the
	// honest 403 of the rights gate).
	identityLive := identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{liveNow}, newPrivacyIDs(13500))
	operator := inviteMember(t, identityLive, w.owner, w.tenant, "operation-ops@example.org", identityDomain.Operator, []identityDomain.Scope{identityDomain.Operations}, "00000000b701", "00000000b702")
	operator.StepUpVerified = true
	auditor := inviteMember(t, identityLive, w.owner, w.tenant, "operation-audit@example.org", identityDomain.ExpertQA, []identityDomain.Scope{identityDomain.Audit}, "00000000b705", "00000000b706")
	// The LIVE control service of the policy rows (the 3-3 row-20
	// precedent: the real Set/Resume over the control store; the route of
	// the preparation is the tenant + connector + autonomy class points).
	controls := controlApp.New(controlStore.NewPostgres(runtimePool), identityLive, connectorrefresolver.New(), fixedClock{liveNow}, &sequenceIDs{values: controlMatrixIDs(21)})
	// The honest owner actor of every human command (the step-up owner of
	// the world; the confirm and the revoke are the human acts).
	owner := w.ownershipActor()

	// Row 1 — the stranger (no membership anywhere) is the honest 404:
	// membership FIRST on every path of the contour, no consent or
	// operation material leaves the service.
	t.Run("row-1 stranger: membership first, the honest 404", func(t *testing.T) {
		stranger := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-00000000b716")}
		_, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, stranger, caseID, id.MustParse("01994f64-6c00-7000-8000-00000000b721"), 1, "op-stranger-key-1", liveNow))
		if !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the non-member is the honest 404 on the authorize surface, got %v", err)
		}
		_, err = svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, stranger, caseID, id.MustParse("01994f64-6c00-7000-8000-00000000b722"), 1, 600, liveNow))
		if !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the non-member is the honest 404 on the confirm surface, got %v", err)
		}
		if strings.Contains(err.Error(), "preview") || strings.Contains(err.Error(), "confirmation") {
			t.Fatalf("the 404 never carries contour material: %s", err.Error())
		}
	})

	// Row 2 — the foreign member (the owner of another tenant) is the
	// honest 404 on THIS tenant: the cross-tenant boundary is closed by
	// the live membership truth (the RLS mirror of the 404 gate).
	t.Run("row-2 foreign member: the cross-tenant 404", func(t *testing.T) {
		foreignIdentity, foreignOwner := newService("00000000b711", "00000000b712", "00000000b713")
		createTenant(t, foreignIdentity, foreignOwner, "Чужие операции", "operation-matrix-foreign-1", "00000000b714")
		_, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, foreignOwner, caseID, operationTargetID(2), 1, 600, liveNow))
		if !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the foreign member confirms nothing of this tenant, got %v", err)
		}
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, foreignOwner, caseID, id.MustParse("01994f64-6c00-7000-8000-00000000b723"), 1, "op-foreign-key-1", liveNow))
		if !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the foreign member authorizes nothing of this tenant, got %v", err)
		}
	})

	// Row 3 — the audit-only member is the honest 403 with the scope words
	// BEFORE any material: the confirmation→authorization contour demands
	// review OR configuration, the audit member hears the exact two ways in.
	t.Run("row-3 audit-only member: the honest 403 with the scope words", func(t *testing.T) {
		_, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, auditor, caseID, operationTargetID(3), 1, 600, liveNow))
		if !errors.Is(err, actionDomain.ErrForbidden) {
			t.Fatalf("the audit-only member never confirms, got %v", err)
		}
		if !strings.Contains(err.Error(), "review или configuration") {
			t.Fatalf("the 403 carries the scope words, got %s", err.Error())
		}
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, auditor, caseID, id.MustParse("01994f64-6c00-7000-8000-00000000b724"), 1, "op-audit-key-1", liveNow))
		if !errors.Is(err, actionDomain.ErrForbidden) || !strings.Contains(err.Error(), "review или configuration") {
			t.Fatalf("the audit-only member never authorizes with the scope words, got %v", err)
		}
	})

	// Row 4 — the FULL LIVE PATH: the typed-valid proposal builds version
	// 1, the human confirm (the step-up act) pins the exact preview pair
	// with the server-truth window, the machine authorize (the first of
	// one client key) lands the ONE transaction — the consumed event, the
	// reservation, the immutable operation, the ledger authorized entry and
	// the outbox operation.queued record; the view speaks the honest words
	// (authorized/queued — executed NEVER exists), the live digest equals
	// the consent copy byte-exact (the double digest protection), the
	// Connector write count stays ZERO (the connection row never moves)
	// and the SQL audits of the ONE transaction count every table.
	t.Run("row-4 the full live path: confirm, authorize, the honest cards", func(t *testing.T) {
		submitted, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(4), authorizeMatrixSingleUse, map[string]string{"phone": "+79000000504"}, nil, nil, 1))
		if err != nil || submitted.VersionNo != 1 {
			t.Fatalf("the live path builds preview version 1, got %+v err=%v", submitted, err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, submitted.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		if confirmed.State != actionDomain.ConfirmationValid || confirmed.VersionNo != 1 || confirmed.PreviewID != submitted.PreviewID || confirmed.PreviewVersionNo != 1 {
			t.Fatalf("the fresh consent is version 1 valid over the exact pin, got %+v", confirmed)
		}
		if confirmed.ValidUntil != liveNow.Add(3600*time.Second) {
			t.Fatalf("the server truth pins the requested window, got %s", confirmed.ValidUntil)
		}
		// The view derivations BEFORE the consumption: the fresh consent
		// card reads valid with the zero-consumption words (the state is
		// the read-time derivation, never a stored column).
		preView, err := svc.ReadAuthorizationView(ctx, owner, w.tenant, caseID, liveNow)
		if err != nil {
			t.Fatal(err)
		}
		for _, confirmation := range preView.Confirmations {
			if confirmation.ConfirmationID == confirmed.ConfirmationID && confirmation.VersionNo == 1 {
				if confirmation.State != actionDomain.ConfirmationValid || !strings.Contains(confirmation.ConsumptionText, "Потреблено 0 из 1") {
					t.Fatalf("the fresh consent card reads valid with zero consumption, got state=%s consumption=%q", confirmation.State, confirmation.ConsumptionText)
				}
			}
		}
		connectionVersion := previewConnectionVersion(t, w)
		authorized, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-live-key-1", liveNow.Add(time.Minute)))
		if err != nil {
			t.Fatal(err)
		}
		if authorized.Replay || authorized.ReservationReference || authorized.Operation.State != actionDomain.OperationAuthorized || !authorized.Operation.Queued {
			t.Fatalf("the first authorization is the creating outcome with the life word and the queued fact, got %+v", authorized)
		}
		if authorized.Operation.ConfirmationID != confirmed.ConfirmationID || authorized.Operation.IdempotencyKey != "op-live-key-1" || len(authorized.Operation.Ledger) != 1 {
			t.Fatalf("the immutable operation pins the consent, the client key and the one ledger entry, got %+v", authorized.Operation)
		}
		if len(authorized.Operation.MaterialDigest) != 64 || len(authorized.Operation.BusinessIntentKey) != 64 {
			t.Fatalf("the two keyed identities are the 64-hex words of their own spaces, got digest=%s intent=%s",
				authorized.Operation.MaterialDigest, authorized.Operation.BusinessIntentKey)
		}
		if previewConnectionVersion(t, w) != connectionVersion {
			t.Fatal("the Connector write count stays ZERO — the connection row never moves on the authorization")
		}
		// The live digest of the built row stays readable for the audit
		// (the consent copy pinned the same 64-hex identity — the double
		// digest protection compared the REAL rows at the authorize).
		card := previewCardOf(t, svc, owner, w.tenant, caseID, submitted.PreviewID, 1)
		if len(card.Digest) != 64 || card.DigestAlarm {
			t.Fatalf("the live row keeps its 64-hex digest re-verifying, got %q alarm=%v", card.Digest, card.DigestAlarm)
		}
		// The view derivations AFTER the consumption: the single_use consent
		// honestly reads exhausted (the allowance is spent — the honest
		// read-time word), the consumption words speak the ONE consumed
		// unit, the operation card speaks the honest boundary texts and
		// executed NEVER exists anywhere.
		view, err := svc.ReadAuthorizationView(ctx, owner, w.tenant, caseID, liveNow)
		if err != nil {
			t.Fatal(err)
		}
		var consumptionText, stateText string
		for _, confirmation := range view.Confirmations {
			if confirmation.ConfirmationID == confirmed.ConfirmationID && confirmation.VersionNo == 1 {
				consumptionText = confirmation.ConsumptionText
				stateText = confirmation.State
			}
		}
		if stateText != actionDomain.ConfirmationExhausted || !strings.Contains(consumptionText, "Потреблено 1 из 1") {
			t.Fatalf("the consumed single_use consent honestly reads exhausted with the one consumed unit, got state=%s consumption=%q", stateText, consumptionText)
		}
		if len(view.Operations) != 1 || view.Operations[0].State != actionDomain.OperationAuthorized || !view.Operations[0].Queued {
			t.Fatalf("exactly one operation card speaks the authorized life word with the queued fact, got %+v", view.Operations)
		}
		if view.Operations[0].ExternalIDAbsentText != actionDomain.OperationExternalIDAbsentText ||
			view.Operations[0].DispatchAbsentText != actionDomain.OperationDispatchAbsentText {
			t.Fatalf("the honest absence texts of the slice boundaries, got %+v", view.Operations[0])
		}
		raw, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "executed") {
			t.Fatal("the view NEVER speaks executed — the word belongs to 3-6 and does not exist here")
		}
		// The material audits of the ONE transaction (the SQL probes over
		// the admin truth): exactly one consumed event, one operation of the
		// client key, one outbox record, one authorized ledger entry and one
		// reservation of the server intent.
		if n := operationConsumedCount(t, w.tenant, confirmed.ConfirmationID, 1); n != 1 {
			t.Fatalf("exactly one consumed event of the consent, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operations where tenant_id=$1 and idempotency_key=$2`, w.tenant, "op-live-key-1"); n != 1 {
			t.Fatalf("exactly one operation of the client key, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.outbox_events where tenant_id=$1 and operation_id=$2`, w.tenant, authorized.Operation.OperationID); n != 1 {
			t.Fatalf("exactly one outbox record of the operation, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operation_ledger where tenant_id=$1 and operation_id=$2 and kind='authorized'`, w.tenant, authorized.Operation.OperationID); n != 1 {
			t.Fatalf("exactly one authorized ledger entry, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.reservations where tenant_id=$1 and operation_id=$2`, w.tenant, authorized.Operation.OperationID); n != 1 {
			t.Fatalf("exactly one reservation of the operation, got %d", n)
		}
	})

	// Row 5 — the bounded clamp of the requested window: the lifetime below
	// the minimum clamps UP to 60 seconds, the lifetime above the maximum
	// clamps DOWN to 3600 — the server truth decides, never the client whim
	// (the promised_at precedent of 2-16). The second confirmation of the
	// SAME preview version refuses at the 1:1 stream bijection.
	t.Run("row-5 valid_until: the bounded server clamp of the window", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(5), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000505"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		under, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 5, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		if under.ValidUntil != liveNow.Add(60*time.Second) {
			t.Fatalf("the window below the minimum clamps up to 60s, got %s", under.ValidUntil)
		}
		if _, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 99999, liveNow)); !errors.Is(err, actionPort.ErrStaleConfirmationVersion) {
			t.Fatalf("the second confirmation of the same preview version refuses — the stream stays 1:1, got %v", err)
		}
		overBuilt, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(505), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000505"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		over, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, overBuilt.PreviewID, 1, 99999, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		if over.ValidUntil != liveNow.Add(3600*time.Second) {
			t.Fatalf("the window above the maximum clamps down to 3600s, got %s", over.ValidUntil)
		}
	})

	// Row 6 — the not_required definition honestly never enters this
	// contour: the query definitions answer through the observations of
	// 2-7, the confirm gate speaks the honest boundary word (AC-1).
	t.Run("row-6 not_required: the honest contour boundary word", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(6), actionMatrixPreviewReady, map[string]string{"phone": "+79000000506"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		_, err = svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 600, liveNow))
		var rule actionDomain.RuleError
		if !errors.As(err, &rule) || !strings.Contains(rule.Reason, actionDomain.ConfirmationNotRequiredText) {
			t.Fatalf("the not_required definition hears the honest boundary word, got %v", err)
		}
	})

	// Row 7 — the bounded consumption rule fails CLOSED at the confirm
	// gate: the numeric bound carrier does not exist in the 3-1
	// definitions, the honest reserved word is spoken, the consent is
	// never created.
	t.Run("row-7 bounded allowance: the fail-closed reserved word", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(7), authorizeMatrixBounded, map[string]string{"phone": "+79000000507"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		_, err = svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 600, liveNow))
		var rule actionDomain.RuleError
		if !errors.As(err, &rule) || !strings.Contains(rule.Reason, actionDomain.ConfirmationBoundedAllowanceText) {
			t.Fatalf("the bounded rule hears the fail-closed reserved word, got %v", err)
		}
	})

	// Row 8 — the step-up discipline: the confirm and the revoke are the
	// HUMAN acts (the LIVE step-up of the session), the authorize is the
	// MACHINE surface of the repeats — the webhook/retry/replay jobs
	// cannot repeat a step-up, the consent is already materialized in the
	// append-only row (the documented decision of the slice).
	t.Run("row-8 step-up: the human acts demand it, the machine surface does not", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(8), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000508"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		noStepUp := identityDomain.Principal{ID: w.owner.ID}
		if _, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, noStepUp, caseID, built.PreviewID, 1, 600, liveNow)); !errors.Is(err, identityDomain.ErrStepUpRequired) {
			t.Fatalf("the confirm without the step-up is the honest refusal, got %v", err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.RevokeConfirmation(ctx, operationRevokeCommand(w, noStepUp, caseID, confirmed.ConfirmationID, 1, liveNow)); !errors.Is(err, identityDomain.ErrStepUpRequired) {
			t.Fatalf("the revoke without the step-up is the honest refusal, got %v", err)
		}
		machine, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, noStepUp, caseID, confirmed.ConfirmationID, 1, "op-machine-key-8", liveNow))
		if err != nil || machine.Operation.ConfirmationID != confirmed.ConfirmationID {
			t.Fatalf("the authorize is the machine surface — the step-up flag never blocks the repeats, got %+v err=%v", machine, err)
		}
	})

	// Row 9 — the machine key gate: the foreign charset and the
	// out-of-bounds length refuse with the honest reason BEFORE
	// everything — the material reader never runs, nothing is consumed
	// (AC-2; the unit counters pinned the order — the live rows pin the
	// words).
	t.Run("row-9 key gate: the charset and the length bounds before everything", func(t *testing.T) {
		countingReader.reset()
		for _, key := range []string{"short", "кириллица-вне-закрытого-набора", strings.Repeat("k", 129)} {
			_, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, id.MustParse("01994f64-6c00-7000-8000-00000000b725"), 1, key, liveNow))
			var rule actionDomain.RuleError
			if !errors.As(err, &rule) {
				t.Fatalf("the key %q refuses with the rule words, got %v", key, err)
			}
			if !strings.Contains(strings.ToLower(rule.Reason), "idempotency key") {
				t.Fatalf("the honest reason names the key gate, got %s", rule.Reason)
			}
		}
		if countingReader.total() != 0 {
			t.Fatalf("the key gate refuses BEFORE the material reads, got %d reader calls", countingReader.total())
		}
	})

	// Row 10 — the replay contract of one client key: the double-click and
	// the webhook repeat carry the SAME key and the SAME pins — both answer
	// the idempotent return of the ORIGINAL operation with the fixed text;
	// the consumption never repeats (the consumed count stays ONE), the
	// second outbox record never exists, the second operation never exists
	// (AC-7/AC-12).
	t.Run("row-10 replay: the double-click and the webhook repeat of one key", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(10), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000510"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		first, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-replay-key-10", liveNow))
		if err != nil || first.Replay || first.ReservationReference {
			t.Fatalf("the first authorization creates the operation, got %+v err=%v", first, err)
		}
		doubleClick, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-replay-key-10", liveNow))
		if err != nil {
			t.Fatal(err)
		}
		if !doubleClick.Replay || doubleClick.OutcomeText != actionDomain.IdempotentReplayText || doubleClick.Operation.OperationID != first.Operation.OperationID {
			t.Fatalf("the double-click replays the ORIGINAL operation with the fixed text, got %+v", doubleClick)
		}
		webhook, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-replay-key-10", liveNow))
		if err != nil || !webhook.Replay || webhook.Operation.OperationID != first.Operation.OperationID {
			t.Fatalf("the webhook repeat replays the ORIGINAL operation, got %+v err=%v", webhook, err)
		}
		if n := operationConsumedCount(t, w.tenant, confirmed.ConfirmationID, 1); n != 1 {
			t.Fatalf("the consumption never repeats, got %d consumed events", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operations where tenant_id=$1 and idempotency_key=$2`, w.tenant, "op-replay-key-10"); n != 1 {
			t.Fatalf("exactly one operation of the repeated key, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.outbox_events where tenant_id=$1 and operation_id=$2`, w.tenant, first.Operation.OperationID); n != 1 {
			t.Fatalf("no second outbox record ever exists, got %d", n)
		}
	})

	// Row 11 — the crash AFTER the commit: the answer of the first request
	// is lost (the client never heard the 201), the replay job repeats the
	// SAME request — the honest replay of the committed operation, the
	// crash-after-commit is safe by the repeat (AC-12).
	t.Run("row-11 crash-after-commit: the lost answer, the honest replay", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(11), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000511"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		first, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-crash-key-11", liveNow))
		if err != nil {
			t.Fatal(err)
		}
		replayed, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-crash-key-11", liveNow))
		if err != nil || !replayed.Replay || replayed.Operation.OperationID != first.Operation.OperationID {
			t.Fatalf("the repeat after the lost answer replays the committed operation, got %+v err=%v", replayed, err)
		}
		if n := operationConsumedCount(t, w.tenant, confirmed.ConfirmationID, 1); n != 1 {
			t.Fatalf("the repeat consumes nothing, got %d consumed events", n)
		}
	})

	// Row 12 — the conflict of one client key with a DIFFERENT material:
	// the preview edit builds version 2, the re-confirmation pins version 2
	// of the SAME consent thread — the repeat of the OLD key with the NEW
	// pins diverges on the digest comparison: the honest 409 conflict with
	// the VISIBLE authorization_conflict exception, the original operation
	// stays unchanged (AC-8).
	t.Run("row-12 conflict: the same key, the new pins, the visible exception", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(12), authorizeMatrixSingleUse, map[string]string{"phone": "+79000000512"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		firstConfirm, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		first, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, firstConfirm.ConfirmationID, 1, "op-conflict-key-12", liveNow))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.NewPreviewVersionFromEdit(ctx, actionAPI.NewPreviewVersionCommand{
			Actor: owner, TenantID: w.tenant, CaseID: caseID, PreviewID: built.PreviewID,
			TypedInput: map[string]string{"phone": "+79000000513"}, Now: liveNow.Add(2 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		secondConfirm, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 2, 3600, liveNow.Add(3*time.Minute)))
		if err != nil || secondConfirm.VersionNo != 2 {
			t.Fatalf("the re-confirmation pins version 2 of the same thread, got %+v err=%v", secondConfirm, err)
		}
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, secondConfirm.ConfirmationID, 2, "op-conflict-key-12", liveNow.Add(3*time.Minute)))
		if !errors.Is(err, actionPort.ErrIdempotencyConflict) {
			t.Fatalf("the same key with the new pins is the honest conflict, got %v", err)
		}
		if !strings.Contains(err.Error(), actionDomain.IdempotencyConflictText) {
			t.Fatalf("the conflict speaks the visible exception text, got %s", err.Error())
		}
		// The VISIBLE exception lands on the consent history of version 2
		// (the authorized owner reads the honest audit of the rejection).
		events, err := svc.ReadAuthorizationView(ctx, owner, w.tenant, caseID, liveNow)
		if err != nil {
			t.Fatal(err)
		}
		var conflictSeen bool
		for _, confirmation := range events.Confirmations {
			if confirmation.ConfirmationID == secondConfirm.ConfirmationID {
				for _, event := range confirmation.Events {
					if event.VersionNo == 2 && event.Kind == actionDomain.ConfirmationEventAuthorizationConflict {
						conflictSeen = true
					}
				}
			}
		}
		if !conflictSeen {
			t.Fatal("the authorization_conflict exception is visible on the consent history")
		}
		// The original operation stays unchanged.
		if n := operationSQLCount(t, `select count(*) from action.operations where tenant_id=$1 and idempotency_key=$2 and material_digest=$3`,
			w.tenant, "op-conflict-key-12", first.Operation.MaterialDigest); n != 1 {
			t.Fatalf("the original operation stays the one committed row, got %d", n)
		}
		if n := operationConsumedCount(t, w.tenant, secondConfirm.ConfirmationID, 2); n != 0 {
			t.Fatalf("the conflicted request consumes nothing, got %d", n)
		}
	})

	// Row 13 — the ladder word superseded: the consent of the OLD preview
	// version answers the exact state word after the re-confirmation of the
	// edited thread — the consumption of the old version NEVER happens
	// (the unconsumed row stays unconsumed).
	t.Run("row-13 ladder superseded: the edit replaces the old consent", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(13), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000514"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.NewPreviewVersionFromEdit(ctx, actionAPI.NewPreviewVersionCommand{
			Actor: owner, TenantID: w.tenant, CaseID: caseID, PreviewID: built.PreviewID,
			TypedInput: map[string]string{"phone": "+79000000515"}, Now: liveNow.Add(2 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 2, 3600, liveNow.Add(3*time.Minute))); err != nil {
			t.Fatal(err)
		}
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-ladder-key-13", liveNow.Add(3*time.Minute)))
		var rejected actionPort.ConfirmationRejectedError
		if !errors.As(err, &rejected) || rejected.State != actionDomain.ConfirmationSuperseded {
			t.Fatalf("the superseded consent answers its exact word, got %v", err)
		}
		if !strings.Contains(actionDomain.ConfirmationStateText(rejected.State), "заменена новой версией") {
			t.Fatal("the ladder word speaks the fixed superseded text")
		}
		if n := operationConsumedCount(t, w.tenant, confirmed.ConfirmationID, 1); n != 0 {
			t.Fatalf("the superseded rejection consumes nothing, got %d", n)
		}
		if n := operationBlockedEventCount(t, w.tenant, confirmed.ConfirmationID, 1, actionDomain.ConfirmationSuperseded); n != 1 {
			t.Fatalf("the visible authorization_blocked audit of the rejection lands once, got %d", n)
		}
	})

	// Row 14 — the ladder word revoked: the human revocation (the step-up
	// act) kills the FUTURE authorizations; the consumption of the revoked
	// consent never happens, the past consumption stays the audit fact (the
	// honest revocation note).
	t.Run("row-14 ladder revoked: the human act kills the future", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(14), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000516"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		revoked, err := svc.RevokeConfirmation(ctx, operationRevokeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, liveNow))
		if err != nil || revoked.State != actionDomain.ConfirmationRevoked {
			t.Fatalf("the revocation speaks the revoked word with the audit note, got %+v err=%v", revoked, err)
		}
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-ladder-key-14", liveNow))
		var rejected actionPort.ConfirmationRejectedError
		if !errors.As(err, &rejected) || rejected.State != actionDomain.ConfirmationRevoked {
			t.Fatalf("the revoked consent answers its exact word, got %v", err)
		}
		if n := operationConsumedCount(t, w.tenant, confirmed.ConfirmationID, 1); n != 0 {
			t.Fatalf("the revoked rejection consumes nothing, got %d", n)
		}
	})

	// Row 15 — the ladder word expired: the valid_until window is the
	// SERVER truth over the LIVE clock — the authorize moment past the
	// window answers the exact expired word, the consumption never happens
	// (the window is a live derivation, never a stored state).
	t.Run("row-15 ladder expired: the live clock past the window", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(15), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000517"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 60, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		if confirmed.ValidUntil != liveNow.Add(60*time.Second) {
			t.Fatalf("the minimum window of the consent, got %s", confirmed.ValidUntil)
		}
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-ladder-key-15", liveNow.Add(2*time.Minute)))
		var rejected actionPort.ConfirmationRejectedError
		if !errors.As(err, &rejected) || rejected.State != actionDomain.ConfirmationExpired {
			t.Fatalf("the expired consent answers its exact word, got %v", err)
		}
		if n := operationConsumedCount(t, w.tenant, confirmed.ConfirmationID, 1); n != 0 {
			t.Fatalf("the expired rejection consumes nothing, got %d", n)
		}
	})

	// Row 16 — the ladder word exhausted: the single_use allowance is
	// consumed by the FIRST authorization (the consumed event of the ONE
	// transaction); the second request of a DIFFERENT key answers the
	// exact exhausted word — the raced second consumption never happens
	// (the count stays ONE).
	t.Run("row-16 ladder exhausted: the single_use allowance of one", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(16), authorizeMatrixSingleUse, map[string]string{"phone": "+79000000518"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-exhaust-key-16a", liveNow)); err != nil {
			t.Fatal(err)
		}
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-exhaust-key-16b", liveNow))
		var rejected actionPort.ConfirmationRejectedError
		if !errors.As(err, &rejected) || rejected.State != actionDomain.ConfirmationExhausted {
			t.Fatalf("the exhausted consent answers its exact word, got %v", err)
		}
		if n := operationConsumedCount(t, w.tenant, confirmed.ConfirmationID, 1); n != 1 {
			t.Fatalf("the second authorization consumes NOTHING — the count stays one, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operations where tenant_id=$1 and idempotency_key=$2`, w.tenant, "op-exhaust-key-16b"); n != 0 {
			t.Fatalf("the refused authorization never creates the operation, got %d", n)
		}
	})

	// Row 17 — the cross-case reservation (AC-9): two parallel cases carry
	// the EQUIVALENT business intent (the same tenant, the same owning
	// system, the same action type, the same target and typed intent) —
	// the unique tenant+Owning-System reservation admits exactly ONE
	// operation; the loser of the second case reads the honest reference
	// of the existing operation with the fixed text, the loser's
	// consumption NEVER happened (the whole transaction rolled back), and
	// no Connector write ever happened on either side.
	t.Run("row-17 cross-case: one reservation, the honest reference to the loser", func(t *testing.T) {
		caseB, workB := w.acceptedCase(t, "evt-op-m2", "sess-op-m2", `{"call":"inbound-call","from":"+79000000519"}`)
		sharedTarget := operationTargetID(17)
		sharedTyped := map[string]string{"phone": "+79000000520"}
		builtA, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, sharedTarget, authorizeMatrixUnlimited, sharedTyped, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		confirmedA, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, builtA.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		winner, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmedA.ConfirmationID, 1, "op-cross-key-17a", liveNow))
		if err != nil || winner.ReservationReference || winner.Replay {
			t.Fatalf("the first case reserves the intent, got %+v err=%v", winner, err)
		}
		builtB, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseB, workB, sharedTarget, authorizeMatrixUnlimited, sharedTyped, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		confirmedB, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseB, builtB.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		loser, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseB, confirmedB.ConfirmationID, 1, "op-cross-key-17b", liveNow))
		if err != nil || !loser.ReservationReference || loser.Replay {
			t.Fatalf("the loser reads the honest reference, got %+v err=%v", loser, err)
		}
		if loser.OutcomeText != actionDomain.ReservationDuplicateText || loser.Operation.OperationID != winner.Operation.OperationID {
			t.Fatalf("the reference names the existing operation with the fixed text, got %+v", loser)
		}
		if n := operationConsumedCount(t, w.tenant, confirmedB.ConfirmationID, 1); n != 0 {
			t.Fatalf("the loser's consumption NEVER happened, got %d consumed events", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operations where tenant_id=$1 and business_intent_key=$2`, w.tenant, winner.Operation.BusinessIntentKey); n != 1 {
			t.Fatalf("exactly one operation of the equivalent intent, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.outbox_events where tenant_id=$1 and operation_id=$2`, w.tenant, winner.Operation.OperationID); n != 1 {
			t.Fatalf("no second outbox record for the loser, got %d", n)
		}
	})

	// Row 18 — the race of ONE client key: the concurrent twins of the
	// same key run the full gateway in parallel — the reservation UNIQUE
	// fires BEFORE the operation twin (the frozen insert order), exactly
	// ONE operation lands, the loser's transaction rolls back (its
	// consumed event never survives) and the loser answers the honest
	// replay of the winner (AC-7/AC-12).
	t.Run("row-18 race of one key: exactly one operation through the reservation", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(18), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000521"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		var mu sync.Mutex
		var created, replayed, refused int
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				outcome, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-race-key-18", liveNow))
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					refused++
					return
				}
				if outcome.Replay {
					replayed++
					return
				}
				created++
			}()
		}
		wg.Wait()
		if created != 1 || replayed != 1 || refused != 0 {
			t.Fatalf("exactly one twin creates, the other replays the winner, got created=%d replayed=%d refused=%d", created, replayed, refused)
		}
		if n := operationConsumedCount(t, w.tenant, confirmed.ConfirmationID, 1); n != 1 {
			t.Fatalf("the rolled-back consumed event never survives — the count stays one, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operations where tenant_id=$1 and idempotency_key=$2`, w.tenant, "op-race-key-18"); n != 1 {
			t.Fatalf("exactly one operation of the raced key, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.outbox_events o join action.operations p on p.tenant_id=o.tenant_id and p.operation_id=o.operation_id where o.tenant_id=$1 and p.idempotency_key=$2`, w.tenant, "op-race-key-18"); n != 1 {
			t.Fatalf("exactly one outbox record of the raced key, got %d", n)
		}
	})

	// Row 19 — the crash BEFORE the commit (the injected probe): the
	// transactional chain of ReserveOperation runs over the LIVE PG with a
	// DOOMED third insert (the client key of row-4 is already committed by
	// another operation) — the consumed event and the reservation of THIS
	// attempt insert and ROLL BACK with the failed operation insert; the
	// SQL audits prove the NOTHING (no consumption, no reservation, no
	// operation, no ledger, no outbox), the original operation of row-4
	// stays unchanged, and the repeat of the same request through the app
	// gateway is CLEAN (the full honest path lands the 201 — the consent
	// was never consumed by the failed attempt) (AC-6/AC-12).
	t.Run("row-19 crash-before-commit: the injected failure rolls back everything", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(19), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000522"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		scope, err := tenancy.New(w.owner.ID, w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		probeOperationID := id.MustParse("01994f64-6c00-7000-8000-00000000b731")
		probeIntent := strings.Repeat("a", 64)
		store := actionStore.NewPostgres(runtimePool)
		err = store.ReserveOperation(ctx, actionPort.ReserveOperationParams{
			Scope:           scope,
			Operation:       operationProbeRow(w, caseID, workItemID, built.PreviewID, confirmed.ConfirmationID, probeOperationID, probeIntent, "op-live-key-1", strings.Repeat("b", 64), liveNow),
			ConsumptionRule: actionDomain.ConsumptionUnlimited,
			ActorID:         w.owner.ID,
			ConsumedEventID: id.MustParse("01994f64-6c00-7000-8000-00000000b732"),
			LedgerEntry: actionDomain.OperationLedgerEntry{
				EntryNo: 1, Kind: actionDomain.LedgerEntryAuthorized, Details: "injected probe",
			},
			OutboxEventID: id.MustParse("01994f64-6c00-7000-8000-00000000b733"),
			OutboxPayload: actionPort.OperationQueuedPayload{
				EventType: "operation.queued", TenantID: w.tenant, OperationID: probeOperationID,
				CaseID: caseID, WorkItemID: workItemID, DefinitionID: authorizeMatrixUnlimited.ID,
				DefinitionVersion: authorizeMatrixUnlimited.Version, OwningSystem: authorizeMatrixUnlimited.OwningSystem,
				PreviewID: built.PreviewID, PreviewVersionNo: 1, ConfirmationID: confirmed.ConfirmationID,
				ConfirmationVersionNo: 1, TargetKind: "order", TargetRef: operationTargetID(19),
				OwnerKind: "human", Partition: "staffy", BusinessIntentKey: probeIntent,
				IdempotencyKey: "op-live-key-1", MaterialDigest: strings.Repeat("b", 64), QueuedAt: liveNow,
			},
			Now: liveNow,
		})
		if !errors.Is(err, actionPort.ErrIdempotencyTwin) {
			t.Fatalf("the doomed third insert answers the twin signal of the doomed chain, got %v", err)
		}
		// The SQL audits of the NOTHING: the consent of the failed attempt
		// stays unconsumed, the reservation rolled back, the operation, the
		// ledger and the outbox of the probe identity never existed.
		if n := operationConsumedCount(t, w.tenant, confirmed.ConfirmationID, 1); n != 0 {
			t.Fatalf("the failed attempt never consumed the consent, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.reservations where tenant_id=$1 and business_intent_key=$2`, w.tenant, probeIntent); n != 0 {
			t.Fatalf("the reservation of the failed attempt rolled back, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operations where tenant_id=$1 and operation_id=$2`, w.tenant, probeOperationID); n != 0 {
			t.Fatalf("the operation of the failed attempt never existed, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.operation_ledger where tenant_id=$1 and operation_id=$2`, w.tenant, probeOperationID); n != 0 {
			t.Fatalf("the ledger of the failed attempt never existed, got %d", n)
		}
		if n := operationSQLCount(t, `select count(*) from action.outbox_events where tenant_id=$1 and operation_id=$2`, w.tenant, probeOperationID); n != 0 {
			t.Fatalf("the outbox record of the failed attempt never existed, got %d", n)
		}
		// The original operation of row-4 stays the unchanged one.
		if n := operationSQLCount(t, `select count(*) from action.operations where tenant_id=$1 and idempotency_key=$2`, w.tenant, "op-live-key-1"); n != 1 {
			t.Fatalf("the committed operation of row-4 stays the one row of the key, got %d", n)
		}
		// The repeat of the SAME request through the app gateway is CLEAN:
		// the consent was never consumed by the failed attempt, the full
		// honest path lands the operation of the NEW key.
		repaired, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-crash-key-19", liveNow))
		if err != nil || repaired.Replay || repaired.ReservationReference {
			t.Fatalf("the repeat of the same request is clean after the rollback, got %+v err=%v", repaired, err)
		}
		if n := operationConsumedCount(t, w.tenant, confirmed.ConfirmationID, 1); n != 1 {
			t.Fatalf("the clean repeat consumes exactly once, got %d", n)
		}
	})

	// Row 20 — SIX freshness reasons against the LIVE source moves (the 3-4
	// dictionary reused — «3-5 читает тот же вывод»): each move walks its
	// own case, and the authorize of the pinned consent answers the EXACT
	// reason text of the live derivation; the consumption NEVER happens on
	// a stale refusal (the seventh reason — the capability evidence —
	// walks LAST in row-26: the rotated secret honestly blocks every
	// LATER build of the world, so the walk stays last) (AC-2/AC-3).
	t.Run("row-20 the seven stale reasons against the live moves", func(t *testing.T) {
		staleReasonOf := func(err error) string {
			t.Helper()
			var stale actionPort.AuthorizationStaleError
			if errors.As(err, &stale) {
				return stale.Reason
			}
			if err != nil {
				t.Fatalf("expected the stale refusal, got %v", err)
			}
			return ""
		}
		pinAndConfirm := func(caseNo id.UUID, workNo id.UUID, target int64, facts, observations []id.UUID) id.UUID {
			t.Helper()
			built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseNo, workNo, operationTargetID(target), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000523"}, facts, observations, 1))
			if err != nil {
				t.Fatal(err)
			}
			confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseNo, built.PreviewID, 1, 3600, liveNow))
			if err != nil {
				t.Fatal(err)
			}
			return confirmed.ConfirmationID
		}

		factCase, factWork := w.acceptedCase(t, "evt-op-m3", "sess-op-m3", `{"call":"inbound-call","from":"+79000000523"}`)
		factID := previewConfirmedFact(t, w, factCase, factWork, "order_status", `"готов"`, "evt-op-m3", "op-fact-1")
		factConfirm := pinAndConfirm(factCase, factWork, 201, []id.UUID{factID}, nil)
		previewConfirmedFact(t, w, factCase, factWork, "order_status", `"отгружен"`, "evt-op-m3", "op-fact-2")
		_, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, factCase, factConfirm, 1, "op-stale-key-20a", liveNow))
		if reason := staleReasonOf(err); reason != actionDomain.FreshnessReasonText(actionDomain.FreshnessFactChanged) {
			t.Fatalf("the moved supporting fact answers its exact text, got %q", reason)
		}

		obsCase, obsWork := w.acceptedCase(t, "evt-op-m4", "sess-op-m4", `{"call":"inbound-call","from":"+79000000524"}`)
		firstRead := w.observationReadOK(t, w.observationReadCommand(obsCase, obsWork, casesDomain.MaterialFieldPrice, "op-observation-1"))
		obsConfirm := pinAndConfirm(obsCase, obsWork, 202, nil, []id.UUID{firstRead.ObservationID})
		w.observationReadOK(t, w.observationReadCommand(obsCase, obsWork, casesDomain.MaterialFieldPrice, "op-observation-2"))
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, obsCase, obsConfirm, 1, "op-stale-key-20b", liveNow))
		if reason := staleReasonOf(err); reason != actionDomain.FreshnessReasonText(actionDomain.FreshnessObservationChanged) {
			t.Fatalf("the newer observation answers its exact text, got %q", reason)
		}

		epochCase, epochWork := w.acceptedCase(t, "evt-op-m5", "sess-op-m5", `{"call":"inbound-call","from":"+79000000525"}`)
		epochConfirm := pinAndConfirm(epochCase, epochWork, 203, nil, nil)
		request, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: epochCase, ExpectedEpoch: 1,
			ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
			WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "передача владения на время отпуска",
			IdempotencyKey: "op-transfer-1", Now: operationNow,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
			Actor: w.backupActor(), TenantID: w.tenant, CaseID: epochCase, RequestID: request.TransferRequestID,
			Decision: casesDomain.TransferResolutionAcknowledged, Reason: "принимаю владение кейсом",
			IdempotencyKey: "op-transfer-2", Now: operationNow.Add(time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, epochCase, epochConfirm, 1, "op-stale-key-20c", liveNow))
		if reason := staleReasonOf(err); reason != actionDomain.FreshnessReasonText(actionDomain.FreshnessOwnerEpochMoved) {
			t.Fatalf("the moved ownership epoch answers its exact text, got %q", reason)
		}

		genCase, genWork := w.acceptedCase(t, "evt-op-m6", "sess-op-m6", `{"call":"inbound-call","from":"+79000000526"}`)
		genConfirm := pinAndConfirm(genCase, genWork, 204, nil, nil)
		if _, err := controls.Set(ctx, operator, w.tenant, controlAPI.SetRequest{
			Kind: controlDomain.ScopeKindAutonomyClass, Ref: "assisted", Mode: controlDomain.ControlModePaused,
			Reason:             "смена поколения контроля матрицы 3-5",
			ExpectedGeneration: 0, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b741"),
		}); err != nil {
			t.Fatal(err)
		}
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, genCase, genConfirm, 1, "op-stale-key-20d", liveNow))
		if reason := staleReasonOf(err); reason != actionDomain.FreshnessReasonText(actionDomain.FreshnessControlGenerationMoved) {
			t.Fatalf("the moved control generation answers its exact text, got %q", reason)
		}
		if _, err := controls.Resume(ctx, operator, w.tenant, controlAPI.ResumeRequest{
			Kind: controlDomain.ScopeKindAutonomyClass, Ref: "assisted",
			Reason: "контроль снят", ExpectedGeneration: 1,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b742"),
		}); err != nil {
			t.Fatal(err)
		}

		manifestCase, manifestWork := w.acceptedCase(t, "evt-op-m7", "sess-op-m7", `{"call":"inbound-call","from":"+79000000527"}`)
		manifestConfirm := pinAndConfirm(manifestCase, manifestWork, 205, nil, nil)
		previewPublishSecondProfile(t, w)
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, manifestCase, manifestConfirm, 1, "op-stale-key-20e", liveNow))
		if reason := staleReasonOf(err); reason != actionDomain.FreshnessReasonText(actionDomain.FreshnessReleaseManifestMoved) {
			t.Fatalf("the published profile answers its exact text, got %q", reason)
		}

		policyCase, policyWork := w.acceptedCase(t, "evt-op-m8", "sess-op-m8", `{"call":"inbound-call","from":"+79000000528"}`)
		policyConfirm := pinAndConfirm(policyCase, policyWork, 206, nil, nil)
		previewMovePolicyOutcome(t, w, policyCase, policyWork, "sess-op-m8", "evt-op-m8-turn-1")
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, policyCase, policyConfirm, 1, "op-stale-key-20f", liveNow))
		if reason := staleReasonOf(err); reason != actionDomain.FreshnessReasonText(actionDomain.FreshnessPolicyChanged) {
			t.Fatalf("the moved policy outcome answers its exact text, got %q", reason)
		}
	})

	// Row 21 — the LIVE entitlement gate of the authorize: the paused
	// commercial perimeter blocks BEFORE the transaction with its exact
	// reason word, the consumption never happens; the resumed admission
	// re-opens the clean path (the LIVE source, never a column). The
	// scoped-control walk of the authorize answers its generation-moved
	// freshness word (row-20d) — the control block of the PREPARATION
	// page answers control_mode, the authorization re-check names the
	// moved pin FIRST (the frozen order of the gateway: the freshness
	// gate runs before the policy gates).
	t.Run("row-21 the paused entitlement blocks the authorize live", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(21), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000530"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.entitlement.RunStateTransition(ctx, entitlementAPI.StateCommand{
			Actor: w.owner, TenantID: w.tenant, Action: entitlementDomain.EventPause,
			Basis: "сезонная приостановка на время инвентаризации операций", Confirm: true,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b761"),
		}); err != nil {
			t.Fatal(err)
		}
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-policy-key-21a", liveNow))
		if !errors.Is(err, actionDomain.ErrForbidden) || !strings.Contains(err.Error(), actionDomain.PolicyReasonText(actionDomain.PolicyReasonEntitlementBlocked)) {
			t.Fatalf("the paused entitlement blocks the authorize with its word, got %v", err)
		}
		if n := operationConsumedCount(t, w.tenant, confirmed.ConfirmationID, 1); n != 0 {
			t.Fatalf("the policy block consumes nothing, got %d", n)
		}
		if _, err := w.entitlement.RunStateTransition(ctx, entitlementAPI.StateCommand{
			Actor: w.owner, TenantID: w.tenant, Action: entitlementDomain.EventResume,
			Basis: "инвентаризация завершена, доступ возобновлён", Confirm: true,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b762"),
		}); err != nil {
			t.Fatal(err)
		}
		resumed, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-policy-key-21a", liveNow))
		if err != nil || resumed.Replay || resumed.ReservationReference {
			t.Fatalf("the resumed admission re-opens the clean path, got %+v err=%v", resumed, err)
		}
	})

	// Row 22 — the LAST material check (AC-3/AG-4): the nil reader of the
	// production root fails CLOSED with the honest 403 word (the
	// authorization honestly cannot happen without the fresh reads); the
	// stale status of the required field answers the honest 409 of the
	// changed material condition; the definition that DECLARED the stale
	// tolerance passes the stale read EXPLICITLY as the allowed_uncertain
	// word — the authorized uncertainty never blocks (AC-3).
	t.Run("row-22 material check: nil reader, stale status, the declared tolerance", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(22), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000531"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, built.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		nilReaderService := w.buildOperationService(nil, actionPreviewIDs(3560, 64))
		_, err = nilReaderService.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-material-key-22a", liveNow))
		if !errors.Is(err, actionPort.ErrFreshReadsUnavailable) || !strings.Contains(err.Error(), actionDomain.FreshReadsUnavailableText) {
			t.Fatalf("the nil reader fails closed with the honest word, got %v", err)
		}
		reader.SetStatus("order_status", actionDomain.SnapshotStale)
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, confirmed.ConfirmationID, 1, "op-material-key-22b", liveNow))
		if !errors.Is(err, actionPort.ErrMaterialConditionChanged) || !strings.Contains(err.Error(), actionDomain.MaterialConditionChangedText) {
			t.Fatalf("the stale status of the required field answers the changed material word, got %v", err)
		}
		if n := operationConsumedCount(t, w.tenant, confirmed.ConfirmationID, 1); n != 0 {
			t.Fatalf("the material refusals consume nothing, got %d", n)
		}
		// The DECLARED stale tolerance passes the stale read explicitly.
		tolerantBuilt, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(508), authorizeMatrixTolerant, map[string]string{"phone": "+79000000532"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		tolerantConfirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, caseID, tolerantBuilt.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		uncertain, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, caseID, tolerantConfirmed.ConfirmationID, 1, "op-material-key-22c", liveNow))
		if err != nil || uncertain.Replay || uncertain.ReservationReference {
			t.Fatalf("the declared tolerance passes the stale read EXPLICITLY, got %+v err=%v", uncertain, err)
		}
		reader.SetStatus("order_status", actionDomain.SnapshotFresh)
	})

	// Row 23 — the database discipline of 00002: RLS + FORCE on the six
	// tables, the append-only grants of the runtime role (INSERT/SELECT
	// only — no UPDATE, no DELETE, no TRUNCATE anywhere), the blind
	// cross-tenant read without the tenant setting (the RLS mirror of the
	// 404 gate), the admin probe that sees the honest rows, and the CHECK
	// backstops of the frozen vocabulary (the dishonest knobs refuse at
	// the database boundary).
	t.Run("row-23 database discipline: RLS, the grants, the blind read, the CHECKs", func(t *testing.T) {
		tables := []string{"confirmations", "confirmation_events", "operations", "operation_ledger", "reservations", "outbox_events"}
		for _, table := range tables {
			var rls, force bool
			if err := adminPool.QueryRow(ctx, `select relrowsecurity, relforcerowsecurity from pg_class
                                where relnamespace = 'action'::regnamespace and relname = $1`, table).Scan(&rls, &force); err != nil {
				t.Fatal(err)
			}
			if !rls || !force {
				t.Fatalf("action.%s must carry RLS + FORCE (the discipline of 00002), got rls=%v force=%v", table, rls, force)
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
		if err := runtimePool.QueryRow(ctx, `select count(*) from action.operations where tenant_id=$1`, w.tenant).Scan(&blind); err != nil {
			t.Fatal(err)
		}
		if blind != 0 {
			t.Fatal("the runtime role without the tenant setting sees NOTHING — the RLS mirror of the 404 gate")
		}
		var visible int
		if err := adminPool.QueryRow(ctx, `select count(*) from action.operations where tenant_id=$1`, w.tenant).Scan(&visible); err != nil {
			t.Fatal(err)
		}
		if visible == 0 {
			t.Fatal("the admin probe sees the honest rows of the tenant")
		}
		// The CHECK backstops: the frozen vocabularies refuse the foreign
		// words at the database boundary (over the REAL rows of the
		// earlier matrix material).
		var liveConfirm id.UUID
		if err := adminPool.QueryRow(ctx, `select confirmation_id from action.confirmations where tenant_id=$1 order by created_at limit 1`, w.tenant).Scan(&liveConfirm); err != nil {
			t.Fatal(err)
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.confirmation_events (tenant_id, confirmation_id, version_no, case_id, event_id, event_no, kind, reason, details, actor_id, created_at)
                        values ($1, $2, 1, $3, $4, 99, 'queued', '', '', $5, $6)`,
			w.tenant, liveConfirm, caseID, id.MustParse("01994f64-6c00-7000-8000-00000000b771"), w.owner.ID, liveNow); err == nil {
			t.Fatal("the foreign confirmation event kind refuses at the CHECK backstop")
		}
		var liveOperation id.UUID
		if err := adminPool.QueryRow(ctx, `select operation_id from action.operations where tenant_id=$1 limit 1`, w.tenant).Scan(&liveOperation); err != nil {
			t.Fatal(err)
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.operation_ledger (tenant_id, operation_id, entry_no, kind, details, created_at)
                        values ($1, $2, 99, 'dispatched', 'probe', $3)`,
			w.tenant, liveOperation, liveNow); err == nil {
			t.Fatal("the foreign ledger kind refuses at the CHECK backstop")
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.operations (tenant_id, operation_id, definition_id, definition_version,
                                preview_id, preview_version_no, confirmation_id, confirmation_version_no,
                                case_id, work_item_id, target_kind, target_ref,
                                owner_kind, owner_ref, owner_epoch, release_decision_no, profile_version,
                                partition, effects, business_intent_key, idempotency_key, material_digest, created_at)
                        values ($1, $2, 'x', '1.0.0', $3, 1, $4, 1, $5, $6, 'order', $7, 'human', $8, 1, 0, 0,
                                'external', '[]'::jsonb, $9, $10, $11, $12)`,
			w.tenant, id.MustParse("01994f64-6c00-7000-8000-00000000b772"),
			id.MustParse("01994f64-6c00-7000-8000-00000000b773"), liveConfirm, caseID, workItemID,
			operationTargetID(23), w.owner.ID, strings.Repeat("c", 64), "op-check-key-23", strings.Repeat("d", 64), liveNow); err == nil {
			t.Fatal("the foreign partition word refuses at the CHECK backstop")
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.operations (tenant_id, operation_id, definition_id, definition_version,
                                preview_id, preview_version_no, confirmation_id, confirmation_version_no,
                                case_id, work_item_id, target_kind, target_ref,
                                owner_kind, owner_ref, owner_epoch, release_decision_no, profile_version,
                                partition, effects, business_intent_key, idempotency_key, material_digest, created_at)
                        values ($1, $2, 'x', '1.0.0', $3, 1, $4, 1, $5, $6, 'order', $7, 'human', $8, 1, 0, 0,
                                'staffy', '[]'::jsonb, $9, 'cyrillic-ключ-зонда', $10, $11)`,
			w.tenant, id.MustParse("01994f64-6c00-7000-8000-00000000b774"),
			id.MustParse("01994f64-6c00-7000-8000-00000000b775"), liveConfirm, caseID, workItemID,
			operationTargetID(23), w.owner.ID, strings.Repeat("c", 64), strings.Repeat("d", 64), liveNow); err == nil {
			t.Fatal("the foreign idempotency charset refuses at the CHECK backstop")
		}
	})

	// Row 24 — the SAFE outbox payload probe (AC-5): the operation.queued
	// record carries ids, pins, words and the TWO key identities — the
	// material NEVER enters (no phone, no template word, no snapshot row,
	// no typed input value); the 3-6 worker will rebuild the material
	// payload from the immutable rows.
	t.Run("row-24 outbox payload: SAFE — ids, pins, words, the two keys", func(t *testing.T) {
		var payload string
		if err := adminPool.QueryRow(ctx, `select payload::text from action.outbox_events
                        where tenant_id=$1 order by created_at limit 1`, w.tenant).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"+79000000", "template", "snapshot", "typed_input", "order_status"} {
			if strings.Contains(payload, forbidden) {
				t.Fatalf("the material %q never enters the SAFE outbox payload, got %s", forbidden, payload)
			}
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			t.Fatal(err)
		}
		for _, required := range []string{"event_type", "operation_id", "case_id", "preview_id", "confirmation_id", "business_intent_key", "idempotency_key", "material_digest", "queued_at"} {
			if _, ok := decoded[required]; !ok {
				t.Fatalf("the SAFE payload carries the %s identity, got %s", required, payload)
			}
		}
		if decoded["event_type"] != "operation.queued" {
			t.Fatalf("the outbox speaks the operation.queued event word, got %v", decoded["event_type"])
		}
	})

	// Row 25 — the append-only digest and the honest empty view: the edit
	// builds version 2 while version 1 stays byte-identical (the digest
	// NEVER moves — the audit readability survives the edit), the fresh
	// case answers the honest empty words of the Operation Review (the
	// empty state never pretends), and executed NEVER exists on any card.
	t.Run("row-25 the digest immutability and the honest empty view", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, operationTargetID(25), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000533"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		before := previewCardOf(t, svc, owner, w.tenant, caseID, built.PreviewID, 1)
		if _, err := svc.NewPreviewVersionFromEdit(ctx, actionAPI.NewPreviewVersionCommand{
			Actor: owner, TenantID: w.tenant, CaseID: caseID, PreviewID: built.PreviewID,
			TypedInput: map[string]string{"phone": "+79000000534"}, Now: liveNow.Add(2 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		after := previewCardOf(t, svc, owner, w.tenant, caseID, built.PreviewID, 1)
		if after.Digest != before.Digest || after.Timeline != actionDomain.TimelineSuperseded {
			t.Fatalf("version 1 stays byte-identical and honestly superseded, got digest-eq=%v timeline=%s", after.Digest == before.Digest, after.Timeline)
		}
		emptyCase, emptyWork := w.acceptedCase(t, "evt-op-m10", "sess-op-m10", `{"call":"inbound-call","from":"+79000000535"}`)
		_ = emptyWork
		emptyView, err := svc.ReadAuthorizationView(ctx, owner, w.tenant, emptyCase, liveNow)
		if err != nil {
			t.Fatal(err)
		}
		if emptyView.EmptyText == "" || len(emptyView.Confirmations) != 0 || len(emptyView.Operations) != 0 {
			t.Fatalf("the honest empty view names the absence, got %+v", emptyView)
		}
		if !strings.Contains(emptyView.EmptyText, "Подтверждений и авторизованных операций нет") {
			t.Fatalf("the empty words speak the honest absence, got %q", emptyView.EmptyText)
		}
		raw, err := json.Marshal(emptyView)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "executed") || strings.Contains(string(raw), "failed") {
			t.Fatal("the empty view never invents an outcome word of 3-6")
		}
	})

	// Row 26 — the seventh stale reason LAST: the rotated connection
	// secret degrades the stored capability evidence (the secret id moved
	// — the old probe no longer proves anything about the live secret),
	// the freshness derivation names capability_evidence_moved; the walk
	// stays last because the degraded capability honestly blocks every
	// LATER build of the world (the same live word the policy gate would
	// answer).
	t.Run("row-26 the capability evidence stale LAST: the rotated secret degrades the proof", func(t *testing.T) {
		capCase, capWork := w.acceptedCase(t, "evt-op-m9", "sess-op-m9", `{"call":"inbound-call","from":"+79000000529"}`)
		capBuilt, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, capCase, capWork, operationTargetID(207), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000536"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		capConfirm, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, owner, capCase, capBuilt.PreviewID, 1, 3600, liveNow))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.connector.Rotate(ctx, connectorAPI.ConnectCommand{
			Actor: w.owner, TenantID: w.tenant, ConnectionID: w.connection,
			CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000b751"),
			IdempotencyKey:  "op-rotate-1",
			ExpectedVersion: previewConnectionVersion(t, w),
			Secrets:         map[string]string{"api_key": "sandbox-rotated-op-key"},
		}); err != nil {
			t.Fatal(err)
		}
		_, err = svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, owner, capCase, capConfirm.ConfirmationID, 1, "op-stale-key-26a", liveNow))
		var stale actionPort.AuthorizationStaleError
		if !errors.As(err, &stale) || stale.Reason != actionDomain.FreshnessReasonText(actionDomain.FreshnessCapabilityEvidenceMoved) {
			t.Fatalf("the rotated secret reads capability_evidence_moved, got %v", err)
		}
		if n := operationConsumedCount(t, w.tenant, capConfirm.ConfirmationID, 1); n != 0 {
			t.Fatalf("the stale refusal consumes nothing, got %d", n)
		}
		// The honest spillover: the degraded capability blocks the NEXT
		// build of the world with the policy word (the live source, never
		// a column).
		blocked, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, capCase, capWork, operationTargetID(208), authorizeMatrixUnlimited, map[string]string{"phone": "+79000000537"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if blocked.BlockedReason != actionDomain.PolicyReasonCapabilityNotVerified {
			t.Fatalf("the degraded capability blocks the next build with capability_not_verified, got %+v", blocked)
		}
	})
}

// operationNow is the case-command clock of the Story 3-5 matrix (one
// hour after the preview matrix — the transfer, the fact and the policy
// commands of the freshness rows pin this moment).
var operationNow = actionPreviewNow.Add(1 * time.Hour)

// operationTargetID mints the deterministic target reference of one
// matrix row (the 019a3f55 range of 3-5; the 2.9 world owns 019a2f4c,
// the 3-3 world owns 019a3f33, the 3-4 world owns 019a3f44). Distinct
// target refs derive distinct proposal keys and distinct business
// intents — every row owns its fresh chain.
func operationTargetID(row int64) id.UUID {
	return id.MustParse(fmt.Sprintf("019a3f55-7000-7000-8000-%012d", row))
}

// The dictionary of the Story 3-5 matrix: the permissive base of 3-4
// with the REQUIRED confirmation and the consumption rules of the
// vertical — the single_use rule of the loser probes, the unlimited rule
// of the replay/race rows, the bounded rule of the fail-closed reserved
// word and the DECLARED stale tolerance of the allowed_uncertain pass.
// The required-confirmation definitions carry the unsafe delivery class
// (the human act by hand — the mutating capability stays AG-4-guarded,
// the verified read-only trunk-status capability keeps the policy chain
// open) with the honest recovery instruction of the declared cancel
// semantics.
func operationAuthorizationDictionary() []actionDomain.ActionDefinition {
	// The unsafe single_use definition (the manualOnly precedent of the
	// journeys fixture): the human act by hand, the honest recovery
	// instruction, the unsafe class demands the single consumption.
	singleUse := actionPreviewBase()
	singleUse.ID = "notify.matrix-single"
	singleUse.Conditions.Confirmation = actionDomain.ConfirmationRequired
	singleUse.Conditions.ConsumptionRule = actionDomain.ConsumptionSingleUse
	singleUse.DeliveryContract.CapabilityClass = actionDomain.CapabilityUnsafe
	singleUse.DeliveryContract.CancelCompensationSem = "вручную: созвон с клиентом и отметка в журнале"
	// The compensatable mutating class carries the unlimited/bounded rules
	// (the unsafe discipline of the guard demands single_use only — the
	// honest dictionary keeps the delivery class per consumption rule).
	compensatable := func(defID string) actionDomain.ActionDefinition {
		def := actionPreviewBase()
		def.ID = defID
		def.Conditions.Confirmation = actionDomain.ConfirmationRequired
		def.DeliveryContract.CapabilityClass = actionDomain.CapabilityCompensatable
		def.DeliveryContract.CancelCompensationSem = "компенсация вручную: перезвон клиенту и запись в журнале изменений"
		return def
	}
	unlimited := compensatable("notify.matrix-unlimited")
	unlimited.Conditions.ConsumptionRule = actionDomain.ConsumptionUnlimited
	bounded := compensatable("notify.matrix-bounded")
	bounded.Conditions.ConsumptionRule = actionDomain.ConsumptionBounded
	tolerant := compensatable("notify.matrix-tolerant")
	tolerant.Conditions.ConsumptionRule = actionDomain.ConsumptionUnlimited
	tolerant.Conditions.RequiredFacts = []actionDomain.FactRequirement{{MaterialField: "order_status", Freshness: actionDomain.SnapshotStale}}
	// The not_required query definition of the honest boundary rows (the
	// 3-4 dictionary definition — the confirm gate of the contour speaks
	// its honest refusal word).
	return []actionDomain.ActionDefinition{singleUse, unlimited, bounded, tolerant, actionMatrixPreviewReady}
}

// The named definitions of the matrix dictionary (the frozen variants —
// every row names the gate it walks).
var (
	authorizeMatrixSingleUse = operationAuthorizationDictionary()[0]
	authorizeMatrixUnlimited = operationDictionaryOf(1)
	authorizeMatrixBounded   = operationDictionaryOf(2)
	authorizeMatrixTolerant  = operationDictionaryOf(3)
)

// operationDictionaryOf returns one dictionary entry by position (the
// honest shorthand of the frozen variants — the dictionary is admitted
// once and never re-derived per row).
func operationDictionaryOf(index int) actionDomain.ActionDefinition {
	return operationAuthorizationDictionary()[index]
}

// buildOperationService wires the REAL app service of the Story 3-5
// contour over the shared cases world: the REAL append-only stores over
// the LIVE PG (the migration 00002 — RLS, the CHECK backstops, the
// advisory locks), the REAL control adapter of the preparation route, the
// LIVE fact sources of the world and the AG-4 adapters of the test
// contour (the nil reader is the honest fail-closed authorization
// refusal — production wires no reader at all). The service carries BOTH
// contours: the preparation (WithPreviews) and the
// confirmation→authorization (WithAuthorization) — the digest re-checks
// compare the REAL built rows.
func (w *casesWorld) buildOperationService(reader actionPort.OwningSystemReader, ids *sequenceIDs) *actionApp.Service {
	packs, caps := packresolver.New(), capabilityresolver.New()
	defs := operationAuthorizationDictionary()
	// The admission walk at construction (the fixture discipline: a
	// dishonest dictionary panics before the first command).
	if err := actionDomain.CatalogIntegrity(defs); err != nil {
		panic("the operation matrix dictionary must pass the guard: " + err.Error())
	}
	for _, def := range defs {
		if err := actionDomain.ValidateDefinitionRefs(def, packs, caps); err != nil {
			panic("the operation matrix dictionary must resolve against the live catalogs: " + err.Error())
		}
	}
	service := actionApp.New(w.identity, packs, caps, defs).
		WithPreviews(actionStore.NewPostgres(runtimePool),
			controleffectiveresolver.NewRouteAdapter(controlStore.NewPostgres(runtimePool), fixedClock{operationNow}),
			w.cases, w.identity, w.entitlement, w.employee, w.connector, w.employee, ids).
		WithAuthorization(actionStore.NewPostgres(runtimePool), actionStore.NewPostgres(runtimePool))
	if reader != nil {
		service = service.WithPreviewReader(reader)
	}
	return service
}

// operationReservationWorld builds the matrix world: the cases world of
// the suite plus the REAL confirmation→authorization contour with the
// counting reader (the block-path probe of the key gate) over the
// synthetic material reader of the test contour.
func operationReservationWorld(t *testing.T, seed int) (*casesWorld, *actionApp.Service, *owningsystem.Synthetic, *countingReader) {
	t.Helper()
	w := newCasesWorld(t, seed)
	// The 512-value id pool of the case commands (the 3-4 precedent: the
	// ten cases of the freshness rows share ONE pool with the commands).
	w.cases = w.buildCasesServiceClock(actionPreviewIDs(seed*10+500, 512), fixedClock{operationNow})
	reader := owningsystem.New()
	counter := &countingReader{inner: reader}
	service := w.buildOperationService(counter, actionPreviewIDs(seed*10+400, 512))
	return w, service, reader, counter
}

// operationConfirmCommand builds the human consent command of one EXACT
// preview version (the step-up act of the matrix rows).
func operationConfirmCommand(w *casesWorld, actor identityDomain.Principal, caseID, previewID id.UUID, versionNo int64, lifetime int, now time.Time) actionAPI.ConfirmPreviewVersionCommand {
	return actionAPI.ConfirmPreviewVersionCommand{
		Actor:            actor,
		TenantID:         w.tenant,
		CaseID:           caseID,
		PreviewID:        previewID,
		PreviewVersionNo: versionNo,
		LifetimeSeconds:  lifetime,
		Now:              now,
	}
}

// operationRevokeCommand builds the human revocation command of one
// exact consent version.
func operationRevokeCommand(w *casesWorld, actor identityDomain.Principal, caseID, confirmationID id.UUID, versionNo int64, now time.Time) actionAPI.RevokeConfirmationCommand {
	return actionAPI.RevokeConfirmationCommand{
		Actor:          actor,
		TenantID:       w.tenant,
		CaseID:         caseID,
		ConfirmationID: confirmationID,
		VersionNo:      versionNo,
		Now:            now,
	}
}

// operationAuthorizeCommand builds the machine authorization command of
// one exact consent version (the client key word is the request
// identity — the double-click, the webhook repeat and the replay job
// carry it again).
func operationAuthorizeCommand(w *casesWorld, actor identityDomain.Principal, caseID, confirmationID id.UUID, versionNo int64, key string, now time.Time) actionAPI.AuthorizeOperationCommand {
	return actionAPI.AuthorizeOperationCommand{
		Actor:                 actor,
		TenantID:              w.tenant,
		CaseID:                caseID,
		ConfirmationID:        confirmationID,
		ConfirmationVersionNo: versionNo,
		IdempotencyKey:        key,
		Now:                   now,
	}
}

// operationConsumedCount counts the consumed events of one exact consent
// version over the admin truth (the read-time count is the material
// audit — the loser's consumed event never survives a rollback).
func operationConsumedCount(t *testing.T, tenant, confirmationID id.UUID, versionNo int64) int {
	t.Helper()
	var n int
	if err := adminPool.QueryRow(context.Background(),
		`select count(*) from action.confirmation_events where tenant_id=$1 and confirmation_id=$2 and version_no=$3 and kind='consumed'`,
		tenant, confirmationID, versionNo).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// operationBlockedEventCount counts the visible authorization_blocked
// audit events of one exact consent version with the exact reason word
// (the honest history of the rejected attempts).
func operationBlockedEventCount(t *testing.T, tenant, confirmationID id.UUID, versionNo int64, reason string) int {
	t.Helper()
	var n int
	if err := adminPool.QueryRow(context.Background(),
		`select count(*) from action.confirmation_events where tenant_id=$1 and confirmation_id=$2 and version_no=$3 and kind='authorization_blocked' and reason=$4`,
		tenant, confirmationID, versionNo, reason).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// operationSQLCount answers the admin count of one SQL probe (the honest
// shorthand of the row audits).
func operationSQLCount(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := adminPool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// operationProbeRow builds the doomed operation row of the
// crash-before-commit probe (the injected failure rolls back the WHOLE
// chain — the row itself never lands; the effects carry the honest
// one-effect default of the conservative abort semantics so the CHECK
// backstop of the table accepts the shape the doomed insert refuses for
// the UNIQUE reason alone).
func operationProbeRow(w *casesWorld, caseID, workItemID, previewID, confirmationID, operationID id.UUID, intentKey, idemKey, digest string, now time.Time) actionDomain.Operation {
	return actionDomain.Operation{
		TenantID:              w.tenant,
		OperationID:           operationID,
		DefinitionID:          authorizeMatrixUnlimited.ID,
		DefinitionVersion:     authorizeMatrixUnlimited.Version,
		PreviewID:             previewID,
		PreviewVersionNo:      1,
		ConfirmationID:        confirmationID,
		ConfirmationVersionNo: 1,
		CaseID:                caseID,
		WorkItemID:            workItemID,
		Target:                actionDomain.ProposalTarget{Kind: actionDomain.TargetOrder, Ref: operationTargetID(19)},
		OwnerKind:             actionDomain.OwnerHuman,
		OwnerRef:              w.owner.ID,
		OwnerEpoch:            1,
		ReleaseDecisionNo:     1,
		ProfileVersion:        1,
		Partition:             actionDomain.PartitionHumanOnly,
		Effects:               []actionDomain.PreviewEffect{{Order: 1, ActionRef: "probe", FailureSemantics: actionDomain.FailureAbortRemaining}},
		BusinessIntentKey:     intentKey,
		IdempotencyKey:        idemKey,
		MaterialDigest:        digest,
		CreatedAt:             now.UTC().Truncate(time.Microsecond),
	}
}
