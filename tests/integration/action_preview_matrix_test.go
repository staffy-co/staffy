//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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
	"github.com/artemiosu/staffy/internal/adapters/dryrunner"
	"github.com/artemiosu/staffy/internal/adapters/owningsystem"
	"github.com/artemiosu/staffy/internal/adapters/packresolver"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
	controlAPI "github.com/artemiosu/staffy/internal/control/api"
	controlApp "github.com/artemiosu/staffy/internal/control/app"
	controlDomain "github.com/artemiosu/staffy/internal/control/domain"
	controlStore "github.com/artemiosu/staffy/internal/control/store"
	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	entitlementAPI "github.com/artemiosu/staffy/internal/entitlement/api"
	entitlementDomain "github.com/artemiosu/staffy/internal/entitlement/domain"
	identityApp "github.com/artemiosu/staffy/internal/identity/app"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	identityStore "github.com/artemiosu/staffy/internal/identity/store"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// TestActionPreviewMatrixRowsBySpec is the Story 3-4 step-04 matrix: the
// honest rows of the proposal→preview preparation contour walked by name
// over the REAL app service, the REAL append-only store over the LIVE PG
// (the migration 00001 of the module — RLS+FORCE, the CHECK backstops,
// the INSERT/SELECT grants), the REAL composition control adapter of the
// preparation route over the LIVE 3.2 control store, the synthetic
// material reader and dry-runner of the test contour (AG-4: production
// wires neither) and the LIVE fact sources of the cases world (the mode
// and the owner epoch of the real case view, the LIVE facts and
// observations, the active entitlement, the published synthetic stage,
// the verified trunk-status capability of the world connection). The
// rows pin the frozen chain order (the rights gate, the definition
// resolution BEFORE the policy, the strict AD-7 validation ladder with
// the terminal repeat and the structurally absent third attempt), the
// policy blocks BY NAME with the LIVE control walk (paused and
// intake_only block, human_only passes with the honest mark), the
// snapshot status dictionary against the declared tolerance (the lying
// reader fails closed to unknown, the explicitly declared uncertainty
// passes), the seven freshness reasons against LIVE source moves, the
// atomic edit with the diff and the superseded Timeline, the dry-run
// evidence that never moves the Timeline, and the database discipline of
// 00001.
//
// The slot registry (the retro-2-3 discipline): the world seed is 344
// (the free slot: 340 belongs to the session-continuation test of the
// suite, 350 to the next world — the lesson of the seed collision), the
// identity ids live
// in the free b6xx suffix range, the preview target refs own the
// 019a3f44 range (the 2.9 world owns 019a2f4c, the 3-3 world owns
// 019a3f33), the service id pool is the 3400 range with the 512-value
// headroom.
func TestActionPreviewMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	w, svc, reader, runner, countingReader := actionPreviewWorld(t, 344)
	caseID, workItemID := w.acceptedCase(t, "evt-preview-m1", "sess-preview-m1", `{"call":"inbound-call","from":"+79000000341"}`)
	// The invited members of the matrix: the operations-only operator (the
	// LIVE control actor of the control rows — the membership-clock rebuild
	// precedent of 3-3: the invitation acceptance reads the live database
	// clock) and the audit-only member (the honest 403 of the rights gate).
	identityLive := identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{time.Now().UTC().Truncate(time.Microsecond)}, newPrivacyIDs(13400))
	operator := inviteMember(t, identityLive, w.owner, w.tenant, "preview-ops@example.org", identityDomain.Operator, []identityDomain.Scope{identityDomain.Operations}, "00000000b601", "00000000b602")
	operator.StepUpVerified = true
	auditor := inviteMember(t, identityLive, w.owner, w.tenant, "preview-audit@example.org", identityDomain.ExpertQA, []identityDomain.Scope{identityDomain.Audit}, "00000000b605", "00000000b606")
	// The LIVE control service of the control rows (the 3-3 row-20
	// precedent: the real Set/Resume over the control store; the route of
	// the preparation is the tenant + connector + autonomy class points).
	controls := controlApp.New(controlStore.NewPostgres(runtimePool), identityLive, connectorrefresolver.New(), fixedClock{actionPreviewNow}, &sequenceIDs{values: controlMatrixIDs(20)})
	// The honest owner actor of every submit (the step-up owner of the
	// world; the role word "owner" is the one the definitions allow).
	owner := w.ownershipActor()

	// Row 1 — the stranger (no membership anywhere) is the honest 404:
	// membership FIRST, no preview material leaves the service.
	t.Run("row-1 stranger: membership first, the honest 404", func(t *testing.T) {
		stranger := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-00000000b616")}
		_, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, stranger, caseID, workItemID, previewTargetID(101), actionMatrixPreviewReady, nil, nil, nil, 1))
		if !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the non-member is the honest 404, got %v", err)
		}
		if strings.Contains(err.Error(), "preview") || strings.Contains(err.Error(), "proposal") {
			t.Fatalf("the 404 never carries preview material: %s", err.Error())
		}
	})

	// Row 2 — the foreign member (the owner of another tenant) is the
	// honest 404 on THIS tenant: the cross-tenant boundary is closed by
	// the live membership truth (the RLS mirror of the 404 gate).
	t.Run("row-2 foreign member: the cross-tenant 404", func(t *testing.T) {
		foreignIdentity, foreignOwner := newService("00000000b611", "00000000b612", "00000000b613")
		createTenant(t, foreignIdentity, foreignOwner, "Чужие preview", "preview-matrix-foreign-1", "00000000b614")
		_, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, foreignOwner, caseID, workItemID, previewTargetID(102), actionMatrixPreviewReady, nil, nil, nil, 1))
		if !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the foreign member reads nothing of this tenant, got %v", err)
		}
	})

	// Row 3 — the audit-only member is the honest 403 with the scope
	// words BEFORE any material: the preparation contour demands review OR
	// configuration, the audit member hears the exact two ways in.
	t.Run("row-3 audit-only member: the honest 403 with the scope words", func(t *testing.T) {
		_, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, auditor, caseID, workItemID, previewTargetID(103), actionMatrixPreviewReady, nil, nil, nil, 1))
		if !errors.Is(err, actionDomain.ErrForbidden) {
			t.Fatalf("the audit-only member is the honest 403, got %v", err)
		}
		if !strings.Contains(err.Error(), "review или configuration") {
			t.Fatalf("the 403 carries the scope words, got %s", err.Error())
		}
	})

	// Row 4 — the unknown definition pin is the honest 404 BEFORE the case
	// material: the definition resolves BEFORE the policy gates (the
	// policy needs the resolved definition to run at all).
	t.Run("row-4 unknown definition pin: the honest 404 before the policy", func(t *testing.T) {
		command := previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(104), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1)
		command.DefinitionID = "preview.unknown"
		_, err := svc.SubmitProposal(ctx, command)
		if !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the unknown definition pin is the honest 404, got %v", err)
		}
	})

	// Row 5 — the dead work item reference is the honest 404 BEFORE the
	// attempt ladder (the AD-2 discipline: the case and the work item are
	// plain uuid pins, the LIVE read validates them before any material).
	t.Run("row-5 dead work item: the honest 404 before the ladder", func(t *testing.T) {
		dead := id.MustParse("019a3f44-7000-7000-8000-00000000dead")
		if _, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, dead, previewTargetID(105), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1)); !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the dead work item anchor is the honest 404, got %v", err)
		}
	})

	// Row 6 — the strict validation ladder: the unknown field and the
	// missing required field answer invalid attempt 1 with the FULL
	// violation word list; the attempt row is recorded with the honest
	// report, NO version exists yet, the material reader saw ZERO calls.
	t.Run("row-6 strict validation: invalid attempt 1 with the violation words", func(t *testing.T) {
		countingReader.reset()
		outcome, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(106), actionMatrixPreviewReady,
			map[string]string{"unknown_extra": "значение", "template_url": "https://example.org/t"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Verdict != actionDomain.VerdictInvalid || outcome.AttemptNo != 1 || outcome.VersionNo != 0 {
			t.Fatalf("the typed-invalid submission is invalid attempt 1 without a version, got %+v", outcome)
		}
		if len(outcome.Violations) == 0 || !previewViolationsSpeak(outcome.Violations, "неизвестно типовой схеме", "Обязательное поле «phone» отсутствует") {
			t.Fatalf("the violations speak the unknown field and the missing required word, got %+v", outcome.Violations)
		}
		if countingReader.total() != 0 {
			t.Fatalf("the material reader never runs on the invalid path, got %d calls", countingReader.total())
		}
		if threads := previewThreadsOf(t, svc, owner, w.tenant, caseID); len(threads) != 1 || len(threads[0].Cards) != 0 || len(threads[0].Attempts) != 1 {
			t.Fatalf("the attempt ladder row exists without a version, got %+v", threads)
		}
	})

	// Row 7 — the repair: the corrected submission of the SAME key is
	// attempt 2 and builds version №1 (the secret template_url never
	// enters the projected typed input — AC-2; the digest is the 64-hex
	// identity of the shown row).
	t.Run("row-7 repair: attempt 2 builds version 1, the secret never projects", func(t *testing.T) {
		outcome, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(106), actionMatrixPreviewReady,
			map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Verdict != actionDomain.VerdictValid || outcome.AttemptNo != 2 || outcome.VersionNo != 1 || !outcome.PreviewID.Valid() {
			t.Fatalf("the repaired submission builds version 1 at attempt 2, got %+v", outcome)
		}
		if len(outcome.PreviewDigest) != 64 {
			t.Fatalf("the digest is the 64-hex keyed identity, got %q", outcome.PreviewDigest)
		}
		card := previewCardOf(t, svc, owner, w.tenant, caseID, outcome.PreviewID, 1)
		if card.TypedInput["phone"] != "+79000000341" {
			t.Fatalf("the projected typed input carries the schema field, got %+v", card.TypedInput)
		}
		if _, secret := card.TypedInput["template_url"]; secret {
			t.Fatal("the secret field NEVER projects into the preview (AC-2)")
		}
		if _, unknown := card.TypedInput["unknown_extra"]; unknown {
			t.Fatal("the unknown field NEVER enters the preview (AC-2)")
		}
	})

	// Row 8 — the terminal repeat of the AD-7 ladder: the second
	// typed-invalid attempt of the same key answers rejected, the ladder
	// is exhausted, the third submission of the key is the structural
	// ErrAttemptExhausted (the store CHECK attempt_no {1,2} is the bound).
	t.Run("row-8 terminal repeat: rejected, then the third attempt does not exist", func(t *testing.T) {
		invalid := map[string]string{"unknown_extra": "again"}
		first, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(108), actionMatrixPreviewReady, invalid, nil, nil, 1))
		if err != nil || first.Verdict != actionDomain.VerdictInvalid || first.AttemptNo != 1 {
			t.Fatalf("the first invalid attempt of the new key, got %+v err=%v", first, err)
		}
		second, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(108), actionMatrixPreviewReady, invalid, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if second.Verdict != actionDomain.VerdictRejected || second.AttemptNo != 2 {
			t.Fatalf("the repeat invalid attempt is the TERMINAL rejected, got %+v", second)
		}
		_, err = svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(108), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if !errors.Is(err, actionPort.ErrAttemptExhausted) {
			t.Fatalf("the third attempt of the same key does not exist (the structural bound), got %v", err)
		}
	})

	// Row 9 — the policy blocks BY NAME on typed-valid material: the role,
	// the conversation mode, the stage and the live AG-1 gate instance
	// each answer their exact reason word; every block records the attempt
	// with the reason, builds NO version and the material reader sees ZERO
	// calls (the unit counters pinned the order — the live rows pin the
	// words).
	t.Run("row-9 policy blocks by name: role, mode, stage, gate", func(t *testing.T) {
		countingReader.reset()
		for _, row := range []struct {
			name   string
			def    actionDomain.ActionDefinition
			reason string
		}{
			{"role_not_allowed", actionMatrixPreviewRole, actionDomain.PolicyReasonRoleNotAllowed},
			{"conversation_mode_not_allowed", actionMatrixPreviewMode, actionDomain.PolicyReasonConversationModeNotAllowed},
			{"stage_not_allowed", actionMatrixPreviewStage, actionDomain.PolicyReasonStageNotAllowed},
		} {
			outcome, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(109), row.def, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Verdict != actionDomain.VerdictValid || outcome.BlockedReason != row.reason || outcome.VersionNo != 0 {
				t.Fatalf("%s: the typed-valid attempt carries the exact block word, got %+v", row.name, outcome)
			}
		}
		// The live gate instance: the OPEN AG-1 instance of the employee
		// is the honest non-pass source of the required gate.
		if _, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
			Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: w.employeeID,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b621"),
			DefinitionID:  "AG-1", TargetState: employeeDomain.GateStateOpen,
		}); err != nil {
			t.Fatal(err)
		}
		outcome, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(109), actionMatrixPreviewGate, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if outcome.BlockedReason != actionDomain.PolicyReasonGateNotSatisfied {
			t.Fatalf("the live open gate instance blocks with gate_not_satisfied, got %+v", outcome)
		}
		// The moved owner epoch answers its own word BEFORE the snapshot:
		// the pinned epoch is the author's stale observation.
		staleEpoch, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(109), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 999))
		if err != nil {
			t.Fatal(err)
		}
		if staleEpoch.BlockedReason != actionDomain.PolicyReasonResponseOwnerChanged {
			t.Fatalf("the moved owner epoch blocks with response_owner_changed, got %+v", staleEpoch)
		}
		if countingReader.total() != 0 {
			t.Fatalf("the material reader never runs on any block path, got %d calls", countingReader.total())
		}
		// The capability gate: the send-sms capability is AG-4-guarded and
		// can NEVER verify in the test contour — the honest word.
		outcome, err = svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(109), actionMatrixPreviewSMS, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if outcome.BlockedReason != actionDomain.PolicyReasonCapabilityNotVerified {
			t.Fatalf("the unverified mutating capability blocks with capability_not_verified, got %+v", outcome)
		}
		if threads := previewThreadsOf(t, svc, owner, w.tenant, caseID); len(threads) == 0 {
			t.Fatal("the blocked attempts still record their ladder rows for the page")
		}
	})

	// Row 10 — the LIVE scoped-control walk of the preparation route: the
	// paused and the intake_only points of the CONNECTOR scope block with
	// the control_mode word; the human_only point PASSES with the honest
	// human-only mark (the human acts by hand, the autonomous path is
	// never offered); the resumed accept opens the clean build again.
	t.Run("row-10 the LIVE control walk: paused and intake_only block, human_only passes with the mark", func(t *testing.T) {
		generation := int64(0)
		controlSet := func(mode string, correlation string) controlDomain.ScopedControl {
			t.Helper()
			point, err := controls.Set(ctx, operator, w.tenant, controlAPI.SetRequest{
				Kind: controlDomain.ScopeKindConnector, Ref: "staffy-synthetic-telephony", Mode: mode,
				Reason:             "регламентный режим подготовки действий матрицы 3-4",
				ExpectedGeneration: generation, CorrelationID: id.MustParse("01994f64-6c00-7000-8000-" + correlation),
			})
			if err != nil {
				t.Fatal(err)
			}
			generation = point.Generation
			return point
		}
		controlResume := func(correlation string) {
			t.Helper()
			point, err := controls.Resume(ctx, operator, w.tenant, controlAPI.ResumeRequest{
				Kind: controlDomain.ScopeKindConnector, Ref: "staffy-synthetic-telephony",
				Reason: "режим снят", ExpectedGeneration: generation,
				CorrelationID: id.MustParse("01994f64-6c00-7000-8000-" + correlation),
			})
			if err != nil {
				t.Fatal(err)
			}
			generation = point.Generation
		}
		controlSet(controlDomain.ControlModePaused, "00000000b631")
		paused, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(110), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if paused.BlockedReason != actionDomain.PolicyReasonControlMode {
			t.Fatalf("the LIVE paused connector point blocks with control_mode, got %+v", paused)
		}
		if _, err := controls.Resume(ctx, operator, w.tenant, controlAPI.ResumeRequest{
			Kind: controlDomain.ScopeKindConnector, Ref: "staffy-synthetic-telephony",
			Reason: "телефония восстановлена", ExpectedGeneration: 1,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b632"),
		}); err != nil {
			t.Fatal(err)
		}
		generation = 2
		controlSet(controlDomain.ControlModeIntakeOnly, "00000000b633")
		intake, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(310), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if intake.BlockedReason != actionDomain.PolicyReasonControlMode {
			t.Fatalf("the LIVE intake_only point blocks with control_mode, got %+v", intake)
		}
		if _, err := controls.Resume(ctx, operator, w.tenant, controlAPI.ResumeRequest{
			Kind: controlDomain.ScopeKindConnector, Ref: "staffy-synthetic-telephony",
			Reason: "приём восстановлен", ExpectedGeneration: 3,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b634"),
		}); err != nil {
			t.Fatal(err)
		}
		generation = 4
		controlSet(controlDomain.ControlModeHumanOnly, "00000000b635")
		humanOnly, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(510), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if humanOnly.BlockedReason != "" || humanOnly.VersionNo != 1 {
			t.Fatalf("the human_only point PASSES the preparation with the honest mark, got %+v", humanOnly)
		}
		card := previewCardOf(t, svc, owner, w.tenant, caseID, humanOnly.PreviewID, 1)
		if !card.HumanOnlyControlMark || card.Partition != actionDomain.PartitionStaffy {
			t.Fatalf("the human_only mark travels with the passing card, got mark=%v partition=%s", card.HumanOnlyControlMark, card.Partition)
		}
		// The lift returns the clean accept to the route (the generation moves
		// again — the next build pins the new generation).
		controlResume("00000000b636")
	})

	// Row 11 — the LIVE entitlement fence: the paused entitlement blocks
	// the preparation with the entitlement word; the resumed admission
	// re-opens the clean path (the LIVE source, never a column).
	t.Run("row-11 the LIVE entitlement: paused blocks, resumed passes", func(t *testing.T) {
		if _, err := w.entitlement.RunStateTransition(ctx, entitlementAPI.StateCommand{
			Actor: w.owner, TenantID: w.tenant, Action: entitlementDomain.EventPause,
			Basis: "сезонная приостановка на время инвентаризации", Confirm: true,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b641"),
		}); err != nil {
			t.Fatal(err)
		}
		blocked, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(111), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if blocked.BlockedReason != actionDomain.PolicyReasonEntitlementBlocked {
			t.Fatalf("the paused entitlement blocks with entitlement_blocked, got %+v", blocked)
		}
		if _, err := w.entitlement.RunStateTransition(ctx, entitlementAPI.StateCommand{
			Actor: w.owner, TenantID: w.tenant, Action: entitlementDomain.EventResume,
			Basis: "инвентаризация завершена, доступ возобновлён", Confirm: true,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b642"),
		}); err != nil {
			t.Fatal(err)
		}
		resumed, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(111), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil || resumed.VersionNo != 1 {
			t.Fatalf("the resumed admission re-opens the clean build, got %+v err=%v", resumed, err)
		}
	})

	// Row 12 — the FULL LIVE PATH: the typed-valid proposal over the
	// verified trunk-status capability builds version 1 with the finite
	// effect list derived ONLY from the definition, the fresh snapshot of
	// the synthetic reader (readiness ready), the staffy partition (the
	// declared reconciliation resolves), the Timeline word previewed and
	// the honest authorization-absence text of the missing 3-5 contour.
	// The view JSON never carries a dispatch verb — the primary action of
	// the page is the transition to the authorization, never a hidden
	// dispatch.
	t.Run("row-12 the full live path: the built card with the honest words", func(t *testing.T) {
		outcome, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(112), actionMatrixPreviewReady,
			map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Verdict != actionDomain.VerdictValid || outcome.VersionNo != 1 {
			t.Fatalf("the full live path builds version 1, got %+v", outcome)
		}
		card := previewCardOf(t, svc, owner, w.tenant, caseID, outcome.PreviewID, 1)
		if card.Readiness != actionDomain.PreviewReady || card.Timeline != actionDomain.TimelinePreviewed || !card.Current {
			t.Fatalf("the fresh card is ready, previewed and current, got %+v", card)
		}
		if card.Partition != actionDomain.PartitionStaffy || len(card.Effects) != 1 || card.Effects[0].FailureSemantics != actionDomain.FailureAbortRemaining {
			t.Fatalf("the staffy partition with the one conservative effect, got %+v", card)
		}
		if len(card.Snapshot) == 0 || card.Snapshot[0].MaterialField != "order_status" || card.Snapshot[0].Status != actionDomain.SnapshotFresh {
			t.Fatalf("the fresh order_status snapshot row, got %+v", card.Snapshot)
		}
		if card.AuthorizationAbsenceText != actionDomain.AuthorizationAbsenceText {
			t.Fatalf("the honest authorization-absence text of the missing 3-5 contour, got %q", card.AuthorizationAbsenceText)
		}
		if card.DigestAlarm {
			t.Fatal("the stored digest re-verifies at every read")
		}
		raw, err := json.Marshal(card)
		if err != nil {
			t.Fatal(err)
		}
		for _, verb := range []string{"dispatch", "authorized", "executed"} {
			if strings.Contains(string(raw), verb) {
				t.Fatalf("the card material never speaks the dispatch verb %q", verb)
			}
		}
	})

	// Row 13 — the unsafe delivery class marks the partition human_only
	// with the honest markers and the recovery instruction from the
	// declared cancel/compensation semantics (the preparation never
	// blocks on the unsafe class — unsafe is a legal human-only action).
	t.Run("row-13 the unsafe class: the human_only partition with the recovery instruction", func(t *testing.T) {
		outcome, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(113), actionMatrixPreviewUnsafe, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		card := previewCardOf(t, svc, owner, w.tenant, caseID, outcome.PreviewID, 1)
		if card.Partition != actionDomain.PartitionHumanOnly || !card.UnsafeClass || !card.HumanOwnerRequired {
			t.Fatalf("the unsafe class keeps the action human-only with the mandatory owner, got %+v", card)
		}
		if card.RecoveryInstruction == "" {
			t.Fatal("the recovery instruction speaks the declared cancel/compensation semantics")
		}
		if card.Readiness != actionDomain.PreviewReady {
			t.Fatalf("the unsafe class never blocks the preparation (readiness is the material truth), got %s", card.Readiness)
		}
	})

	// Row 14 — the snapshot status dictionary against the DECLARED
	// tolerance: fresh keeps ready; stale, conflicting and unknown block
	// the readiness with the honest blocked_material word (the preview
	// exists, the readiness does not pretend); the LYING reader word fails
	// closed to unknown at the mapping; the definition that DECLARED the
	// stale tolerance passes the stale read as the explicit
	// allowed_uncertain word (never derived from silence).
	t.Run("row-14 the snapshot dictionary: fresh, stale, conflicting, unknown, the lying word, the declared tolerance", func(t *testing.T) {
		readinessOf := func(def actionDomain.ActionDefinition, target int64) (string, actionAPI.PreviewCardView) {
			t.Helper()
			outcome, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(target), def, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
			if err != nil {
				t.Fatal(err)
			}
			card := previewCardOf(t, svc, owner, w.tenant, caseID, outcome.PreviewID, 1)
			return card.Readiness, card
		}
		if readiness, _ := readinessOf(actionMatrixPreviewReady, 114); readiness != actionDomain.PreviewReady {
			t.Fatalf("the fresh read keeps the readiness ready, got %s", readiness)
		}
		for i, row := range []struct {
			name   string
			status string
		}{
			{"stale", actionDomain.SnapshotStale},
			{"conflicting", actionDomain.SnapshotConflicting},
			{"unknown", actionDomain.SnapshotUnknown},
			{"the lying foreign word", "super-fresh"},
		} {
			reader.SetStatus("order_status", row.status)
			readiness, card := readinessOf(actionMatrixPreviewReady, int64(115+100*i))
			if readiness != actionDomain.PreviewBlockedMaterial {
				t.Fatalf("%s blocks the readiness honestly, got %s", row.name, readiness)
			}
			if row.status == "super-fresh" && card.Snapshot[0].Status != actionDomain.SnapshotUnknown {
				t.Fatalf("the lying reader word fails closed to unknown at the mapping, got %s", card.Snapshot[0].Status)
			}
		}
		reader.SetStatus("order_status", actionDomain.SnapshotStale)
		readiness, card := readinessOf(actionMatrixPreviewTolerant, 116)
		if readiness != actionDomain.PreviewReady || card.Snapshot[0].Status != actionDomain.SnapshotAllowedUncertain {
			t.Fatalf("the DECLARED stale tolerance passes as the explicit allowed_uncertain, got readiness=%s status=%s", readiness, card.Snapshot[0].Status)
		}
		reader.SetStatus("order_status", actionDomain.SnapshotFresh)
	})

	// Row 15 — the nil reader is the honest production boundary (AG-4):
	// the build still lands (the preview exists), the readiness honestly
	// blocks with the blocked_material word — the interface never pretends
	// a material it did not read.
	t.Run("row-15 the nil reader: the AG-4 production boundary", func(t *testing.T) {
		nilReaderService := w.buildActionPreviewService(nil, dryrunner.New(), actionPreviewIDs(3450, 64))
		outcome, err := nilReaderService.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(117), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		card := previewCardOf(t, nilReaderService, owner, w.tenant, caseID, outcome.PreviewID, 1)
		if card.Readiness != actionDomain.PreviewBlockedMaterial || len(card.Snapshot) != 0 {
			t.Fatalf("the nil reader keeps the readiness honestly blocked without rows, got %+v", card)
		}
	})

	// Row 16 — the dry-run evidence: the wired runner scripts the invalid
	// word of the exact version; the evidence lands append-only with the
	// result word, the Timeline words NEVER move (AC-8); the nil runner is
	// the honest AG-4 refusal in words and NO evidence row exists; the
	// lying foreign result word refuses at the guard.
	t.Run("row-16 dry-run: the evidence, the unmoved Timeline, the AG-4 refusal, the foreign word", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(118), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		runner.SetOutcome(built.PreviewID.String(), "1", actionDomain.DryRunResultInvalid, "синтетический прогон нашёл пустой номер")
		outcome, err := svc.RecordDryRun(ctx, actionAPI.DryRunCommand{
			Actor: owner, TenantID: w.tenant, CaseID: caseID,
			PreviewID: built.PreviewID, VersionNo: 1, Now: actionPreviewNow,
		})
		if err != nil || outcome.Result != actionDomain.DryRunResultInvalid {
			t.Fatalf("the wired runner records the evidence, got %+v err=%v", outcome, err)
		}
		card := previewCardOf(t, svc, owner, w.tenant, caseID, built.PreviewID, 1)
		if card.Timeline != actionDomain.TimelinePreviewed {
			t.Fatalf("the dry-run evidence NEVER moves the Timeline, got %s", card.Timeline)
		}
		for _, thread := range previewThreadsOf(t, svc, owner, w.tenant, caseID) {
			if thread.PreviewID == built.PreviewID {
				if len(thread.DryRunEvidence) == 0 || thread.DryRunEvidence[0].Result != actionDomain.DryRunResultInvalid {
					t.Fatalf("the thread carries the dry-run evidence row, got %+v", thread.DryRunEvidence)
				}
			}
		}
		// The nil runner: the honest refusal, no evidence row.
		nilRunnerService := w.buildActionPreviewService(owningsystem.New(), nil, actionPreviewIDs(3460, 64))
		_, err = nilRunnerService.RecordDryRun(ctx, actionAPI.DryRunCommand{
			Actor: owner, TenantID: w.tenant, CaseID: caseID,
			PreviewID: built.PreviewID, VersionNo: 1, Now: actionPreviewNow,
		})
		var rule actionDomain.RuleError
		if !errors.As(err, &rule) || !strings.Contains(rule.Reason, actionDomain.DryRunUnsupportedText) {
			t.Fatalf("the nil runner is the honest AG-4 refusal in words, got %v", err)
		}
		// The lying foreign result word refuses at the guard.
		lyingRunnerService := w.buildActionPreviewService(owningsystem.New(), &lyingDryRunner{}, actionPreviewIDs(3470, 64))
		_, err = lyingRunnerService.RecordDryRun(ctx, actionAPI.DryRunCommand{
			Actor: owner, TenantID: w.tenant, CaseID: caseID,
			PreviewID: built.PreviewID, VersionNo: 1, Now: actionPreviewNow,
		})
		if !errors.As(err, &rule) {
			t.Fatalf("the foreign result word refuses at the guard, got %v", err)
		}
	})

	// Row 17 — six of the seven freshness reasons against LIVE source
	// moves, each on its own case: the supporting fact version, the
	// current observation, the ownership epoch, the control generation,
	// the release lineage profile version and the policy outcome number
	// (the seventh — the capability evidence — walks LAST in row-21: the
	// degraded capability proof honestly blocks every later build of the
	// world). The FIRST non-matching pin is the exact reason word; the old
	// version stays readable for the audit forever (the digest never
	// moves, the Timeline never moves) and loses only the authorization
	// suitability through the derivation.
	t.Run("row-17 six stale reasons against the live moves (the seventh walks last)", func(t *testing.T) {
		freshnessReasonOf := func(caseID id.UUID, reason string) {
			t.Helper()
			for _, thread := range previewThreadsOf(t, svc, owner, w.tenant, caseID) {
				for _, card := range thread.Cards {
					if card.FreshnessStale && card.FreshnessReason != reason {
						t.Fatalf("the stale card carries the exact reason word %s, got %s (%s)", reason, card.FreshnessReason, card.FreshnessReasonText)
					}
				}
			}
		}
		factCase, factWork := w.acceptedCase(t, "evt-preview-m2", "sess-preview-m2", `{"call":"inbound-call","from":"+79000000342"}`)
		factID := previewConfirmedFact(t, w, factCase, factWork, "order_status", `"готов"`, "evt-preview-m2", "preview-fact-1")
		pinned, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, factCase, factWork, previewTargetID(119), actionMatrixPreviewReady, map[string]string{"phone": "+79000000342"}, []id.UUID{factID}, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		previewConfirmedFact(t, w, factCase, factWork, "order_status", `"отгружен"`, "evt-preview-m2", "preview-fact-2")
		previewReadStale(t, svc, owner, w.tenant, factCase, pinned.PreviewID, 1)
		freshnessReasonOf(factCase, actionDomain.FreshnessFactChanged)

		obsCase, obsWork := w.acceptedCase(t, "evt-preview-m3", "sess-preview-m3", `{"call":"inbound-call","from":"+79000000343"}`)
		firstRead := w.observationReadOK(t, w.observationReadCommand(obsCase, obsWork, casesDomain.MaterialFieldPrice, "preview-obs-1"))
		pinned, err = svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, obsCase, obsWork, previewTargetID(120), actionMatrixPreviewReady, map[string]string{"phone": "+79000000343"}, nil, []id.UUID{firstRead.ObservationID}, 1))
		if err != nil {
			t.Fatal(err)
		}
		w.observationReadOK(t, w.observationReadCommand(obsCase, obsWork, casesDomain.MaterialFieldPrice, "preview-obs-2"))
		previewReadStale(t, svc, owner, w.tenant, obsCase, pinned.PreviewID, 1)
		freshnessReasonOf(obsCase, actionDomain.FreshnessObservationChanged)

		epochCase, epochWork := w.acceptedCase(t, "evt-preview-m4", "sess-preview-m4", `{"call":"inbound-call","from":"+79000000344"}`)
		pinned, err = svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, epochCase, epochWork, previewTargetID(121), actionMatrixPreviewReady, map[string]string{"phone": "+79000000344"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		request, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: epochCase, ExpectedEpoch: 1,
			ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
			WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "передача владения на время отпуска",
			IdempotencyKey: "preview-matrix-tr-1", Now: actionPreviewNow,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
			Actor: w.backupActor(), TenantID: w.tenant, CaseID: epochCase, RequestID: request.TransferRequestID,
			Decision: casesDomain.TransferResolutionAcknowledged, Reason: "принимаю владение кейсом",
			IdempotencyKey: "preview-matrix-tr-2", Now: actionPreviewNow.Add(time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		previewReadStale(t, svc, owner, w.tenant, epochCase, pinned.PreviewID, 1)
		freshnessReasonOf(epochCase, actionDomain.FreshnessOwnerEpochMoved)

		genCase, genWork := w.acceptedCase(t, "evt-preview-m5", "sess-preview-m5", `{"call":"inbound-call","from":"+79000000345"}`)
		pinned, err = svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, genCase, genWork, previewTargetID(122), actionMatrixPreviewReady, map[string]string{"phone": "+79000000345"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := controls.Set(ctx, operator, w.tenant, controlAPI.SetRequest{
			Kind: controlDomain.ScopeKindAutonomyClass, Ref: "assisted", Mode: controlDomain.ControlModePaused,
			Reason: "смена поколения контроля матрицы 3-4", ExpectedGeneration: 0,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b651"),
		}); err != nil {
			t.Fatal(err)
		}
		previewReadStale(t, svc, owner, w.tenant, genCase, pinned.PreviewID, 1)
		freshnessReasonOf(genCase, actionDomain.FreshnessControlGenerationMoved)
		if _, err := controls.Resume(ctx, operator, w.tenant, controlAPI.ResumeRequest{
			Kind: controlDomain.ScopeKindAutonomyClass, Ref: "assisted",
			Reason: "контроль снят", ExpectedGeneration: 1,
			CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b652"),
		}); err != nil {
			t.Fatal(err)
		}

		manifestCase, manifestWork := w.acceptedCase(t, "evt-preview-m6", "sess-preview-m6", `{"call":"inbound-call","from":"+79000000346"}`)
		pinned, err = svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, manifestCase, manifestWork, previewTargetID(123), actionMatrixPreviewReady, map[string]string{"phone": "+79000000346"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		previewPublishSecondProfile(t, w)
		previewReadStale(t, svc, owner, w.tenant, manifestCase, pinned.PreviewID, 1)
		freshnessReasonOf(manifestCase, actionDomain.FreshnessReleaseManifestMoved)

		policyCase, policyWork := w.acceptedCase(t, "evt-preview-m7", "sess-preview-m7", `{"call":"inbound-call","from":"+79000000347"}`)
		pinned, err = svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, policyCase, policyWork, previewTargetID(124), actionMatrixPreviewReady, map[string]string{"phone": "+79000000347"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		previewMovePolicyOutcome(t, w, policyCase, policyWork, "sess-preview-m7", "evt-preview-m7-turn-1")
		previewReadStale(t, svc, owner, w.tenant, policyCase, pinned.PreviewID, 1)
		freshnessReasonOf(policyCase, actionDomain.FreshnessPolicyChanged)
	})

	// Row 18 — the atomic edit: the changed typed material re-runs the
	// FULL chain and lands version №2 in ONE transaction with the
	// superseded event of №1; the diff of the pair is the read-time
	// derivation (never stored), №1 stays byte-identical (the digest never
	// moves) and honestly superseded, №2 is current.
	t.Run("row-18 the atomic edit: v2 + diff + superseded v1", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(126), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		first := previewCardOf(t, svc, owner, w.tenant, caseID, built.PreviewID, 1)
		edited, err := svc.NewPreviewVersionFromEdit(ctx, actionAPI.NewPreviewVersionCommand{
			Actor: owner, TenantID: w.tenant, CaseID: caseID, PreviewID: built.PreviewID,
			TypedInput: map[string]string{"phone": "+79000000349"}, Now: actionPreviewNow.Add(2 * time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}
		if edited.VersionNo != 2 || edited.Supersedes != 1 || edited.PreviewDigest == first.Digest {
			t.Fatalf("the edit lands version 2 superseding 1 with the new digest, got %+v", edited)
		}
		old := previewCardOf(t, svc, owner, w.tenant, caseID, built.PreviewID, 1)
		fresh := previewCardOf(t, svc, owner, w.tenant, caseID, built.PreviewID, 2)
		if old.Digest != first.Digest || old.Timeline != actionDomain.TimelineSuperseded || old.Current {
			t.Fatalf("№1 stays byte-identical and honestly superseded, got digest-eq=%v timeline=%s current=%v", old.Digest == first.Digest, old.Timeline, old.Current)
		}
		if !fresh.Current || fresh.Timeline != actionDomain.TimelinePreviewed {
			t.Fatalf("№2 is the current previewed card, got %+v", fresh)
		}
		if len(fresh.DiffLines) == 0 || !strings.Contains(strings.Join(fresh.DiffLines, " | "), "Поле «phone» изменено") {
			t.Fatalf("the read-time diff speaks the changed field, got %+v", fresh.DiffLines)
		}
		// The typed-invalid edit answers BEFORE any write: the old version
		// stays fully current, the ladder does not move.
		_, err = svc.NewPreviewVersionFromEdit(ctx, actionAPI.NewPreviewVersionCommand{
			Actor: owner, TenantID: w.tenant, CaseID: caseID, PreviewID: built.PreviewID,
			TypedInput: map[string]string{"unknown_field": "значение"}, Now: actionPreviewNow.Add(3 * time.Minute),
		})
		var rule actionDomain.RuleError
		if !errors.As(err, &rule) {
			t.Fatalf("the invalid edit refuses with the rule words, got %v", err)
		}
		still := previewCardOf(t, svc, owner, w.tenant, caseID, built.PreviewID, 2)
		if !still.Current || still.TypedInput["phone"] != "+79000000349" {
			t.Fatalf("the refused edit never touches the current version, got %+v", still)
		}
	})

	// Row 19 — the database discipline of 00001: RLS + FORCE on the three
	// tables, the append-only grants of the runtime role (INSERT/SELECT
	// only), the blind cross-tenant read without the tenant setting (the
	// RLS mirror of the 404 gate) and the CHECK backstops of the frozen
	// vocabulary — the dishonest knobs refuse at the database boundary,
	// the honest template lands.
	t.Run("row-19 database discipline: RLS+FORCE, the append-only grants, the blind read, the CHECK backstops", func(t *testing.T) {
		rows, err := runtimePool.Query(ctx, `
                        select relname, relrowsecurity, relforcerowsecurity from pg_class
                        where relnamespace = 'action'::regnamespace
                          and relname in ('action_proposals','action_previews','action_preview_events')`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		seen := map[string][2]bool{}
		for rows.Next() {
			var name string
			var rls, force bool
			if err = rows.Scan(&name, &rls, &force); err != nil {
				t.Fatal(err)
			}
			seen[name] = [2]bool{rls, force}
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"action_proposals", "action_previews", "action_preview_events"} {
			if state, ok := seen[table]; !ok || !state[0] || !state[1] {
				t.Fatalf("%s must carry RLS + FORCE (the append-only discipline), got %+v", table, state)
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
		if err := runtimePool.QueryRow(ctx, `select count(*) from action.action_previews where tenant_id=$1`, w.tenant).Scan(&blind); err != nil {
			t.Fatal(err)
		}
		if blind != 0 {
			t.Fatal("the runtime role without the tenant setting sees NOTHING — the RLS mirror of the 404 gate")
		}
		var visible int
		if err := adminPool.QueryRow(ctx, `select count(*) from action.action_previews where tenant_id=$1`, w.tenant).Scan(&visible); err != nil {
			t.Fatal(err)
		}
		if visible == 0 {
			t.Fatal("the admin probe sees the honest rows of the tenant")
		}
		// The CHECK backstops over a real built row of the case.
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(190), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.action_proposals (tenant_id, proposal_key, attempt_no, preview_id, definition_id, definition_version,
                                case_id, work_item_id, target_kind, target_ref, verdict, blocked_reason, invalid_report, actor_id, created_at)
                        values ($1, $2, 3, $3, 'x', '1.0.0', $4, $5, 'order', $6, 'valid', '', '', $7, $8)`,
			w.tenant, strings.Repeat("a", 64), previewTargetID(191), caseID, workItemID, previewTargetID(191), w.owner.ID, actionPreviewNow); err == nil {
			t.Fatal("attempt_no 3 refuses at the CHECK backstop — the third attempt does not exist")
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.action_proposals (tenant_id, proposal_key, attempt_no, preview_id, definition_id, definition_version,
                                case_id, work_item_id, target_kind, target_ref, verdict, blocked_reason, invalid_report, actor_id, created_at)
                        values ($1, $2, 1, $3, 'x', '1.0.0', $4, $5, 'order', $6, 'maybe', '', '', $7, $8)`,
			w.tenant, strings.Repeat("b", 64), previewTargetID(192), caseID, workItemID, previewTargetID(192), w.owner.ID, actionPreviewNow); err == nil {
			t.Fatal("the foreign verdict word refuses at the CHECK backstop")
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into action.action_preview_events (tenant_id, preview_id, version_no, case_id, event_id, event_no, kind, reason, details, actor_id, created_at)
                        values ($1, $2, 1, $3, $4, 1, 'queued', '', '', $5, $6)`,
			w.tenant, built.PreviewID, caseID, previewTargetID(193), w.owner.ID, actionPreviewNow); err == nil {
			t.Fatal("the foreign event kind refuses at the CHECK backstop")
		}
		var sane int
		if err := adminPool.QueryRow(ctx, `select count(*) from action.action_previews where tenant_id=$1 and preview_id=$2 and version_no=1`,
			w.tenant, built.PreviewID).Scan(&sane); err != nil {
			t.Fatal(err)
		}
		if sane != 1 {
			t.Fatalf("the honest template of the row-19 build stays the only one, got %d", sane)
		}
	})

	// Row 20 — the append-only store probe: the AD-7 ladder of one key is
	// bound by the PRIMARY KEY (the raced twin answers the stale refusal)
	// and the version ladder of one thread is monotonic under the preview
	// lock — the concurrent edit twins land exactly ONE new version, the
	// raced twin hears the honest ladder-moved words.
	t.Run("row-20 the raced twins: exactly ONE new version of the thread", func(t *testing.T) {
		built, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, caseID, workItemID, previewTargetID(150), actionMatrixPreviewReady, map[string]string{"phone": "+79000000341"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		twin := w.buildActionPreviewService(owningsystem.New(), dryrunner.New(), actionPreviewIDs(3480, 64))
		var wg sync.WaitGroup
		var mu sync.Mutex
		var landed, raced int
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := twin.NewPreviewVersionFromEdit(ctx, actionAPI.NewPreviewVersionCommand{
					Actor: owner, TenantID: w.tenant, CaseID: caseID, PreviewID: built.PreviewID,
					TypedInput: map[string]string{"phone": "+79000000350"}, Now: actionPreviewNow.Add(4 * time.Minute),
				})
				mu.Lock()
				defer mu.Unlock()
				if err == nil {
					landed++
					return
				}
				if strings.Contains(err.Error(), "Лестница версий") || errors.Is(err, actionPort.ErrStalePreviewVersion) {
					raced++
					return
				}
			}()
		}
		wg.Wait()
		if landed != 1 || raced != 1 {
			t.Fatalf("exactly ONE twin lands the version, the raced twin hears the honest words, got landed=%d raced=%d", landed, raced)
		}
		var versions int
		if err := adminPool.QueryRow(ctx, `select count(*) from action.action_previews where tenant_id=$1 and preview_id=$2`,
			w.tenant, built.PreviewID).Scan(&versions); err != nil {
			t.Fatal(err)
		}
		if versions != 2 {
			t.Fatalf("the thread carries exactly two versions (the original plus the ONE landed twin edit), got %d", versions)
		}
	})

	// Row 21 — the seventh stale reason LAST: the rotated connection secret
	// degrades the stored capability evidence (the secret id moved — the old
	// probe no longer proves anything about the live secret), the freshness
	// derivation names capability_evidence_moved. The walk stays last: the
	// degraded capability honestly blocks every LATER build (the same live
	// word the policy gate would answer).
	t.Run("row-21 the capability evidence stale: the rotated secret degrades the proof", func(t *testing.T) {
		capCase, capWork := w.acceptedCase(t, "evt-preview-m8", "sess-preview-m8", `{"call":"inbound-call","from":"+79000000348"}`)
		pinned, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, capCase, capWork, previewTargetID(125), actionMatrixPreviewReady, map[string]string{"phone": "+79000000348"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.connector.Rotate(ctx, connectorAPI.ConnectCommand{
			Actor: w.owner, TenantID: w.tenant, ConnectionID: w.connection,
			CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000b671"),
			IdempotencyKey:  "preview-matrix-rotate-1",
			ExpectedVersion: previewConnectionVersion(t, w),
			Secrets:         map[string]string{"api_key": "sandbox-rotated-preview-key"},
		}); err != nil {
			t.Fatal(err)
		}
		previewReadStale(t, svc, owner, w.tenant, capCase, pinned.PreviewID, 1)
		for _, thread := range previewThreadsOf(t, svc, owner, w.tenant, capCase) {
			for _, card := range thread.Cards {
				if card.FreshnessStale && card.FreshnessReason != actionDomain.FreshnessCapabilityEvidenceMoved {
					t.Fatalf("the rotated secret reads capability_evidence_moved, got %s (%s)", card.FreshnessReason, card.FreshnessReasonText)
				}
			}
		}
		// The honest spillover: the degraded capability blocks the NEXT build
		// of the world with the policy word (the live source, never a column).
		blocked, err := svc.SubmitProposal(ctx, previewSubmitCommand(w, owner, capCase, capWork, previewTargetID(127), actionMatrixPreviewReady, map[string]string{"phone": "+79000000348"}, nil, nil, 1))
		if err != nil {
			t.Fatal(err)
		}
		if blocked.BlockedReason != actionDomain.PolicyReasonCapabilityNotVerified {
			t.Fatalf("the degraded capability blocks the next build with capability_not_verified, got %+v", blocked)
		}
	})
}

// actionPreviewNow is the command clock of the Story 3-4 matrix: one hour
// after the ownership world — every command of the matrix pins the same
// moment so the derived rows stay deterministic.
var actionPreviewNow = ownershipNow.Add(1 * time.Hour)

// previewTargetID mints the deterministic target reference of one matrix
// row (the 019a3f44 range of 3-4; the 2.9 world owns 019a2f4c, the 3-3
// world owns 019a3f33). Distinct target refs derive distinct proposal
// keys — every row owns its fresh AD-7 ladder.
func previewTargetID(row int64) id.UUID {
	return id.MustParse(fmt.Sprintf("019a3f44-7000-7000-8000-%012d", row))
}

// actionPreviewIDs mints the id pool of the matrix world (the 512-value
// headroom — the preview threads, the events and the attempts of one
// matrix share ONE pool).
func actionPreviewIDs(prefix, count int) *sequenceIDs {
	values := make([]id.UUID, 0, count)
	for i := 0; i < count; i++ {
		values = append(values, id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", prefix*1000+i)))
	}
	return &sequenceIDs{values: values}
}

// actionPreviewBase builds the permissive base of the matrix definitions:
// the LIVE trunk-status capability of the world connection (verified in
// the world construction — the only capability that CAN verify: the
// mutating send-sms is AG-4-blocked, its honest word is the row-9
// capability block), the permissive conditions over the honest live
// facts of the world (the owner role, the unclassified mode of the fresh
// case, the published synthetic stage) and the query_reconcilable
// delivery contract whose declared reconciliation resolves (the staffy
// partition). Every variant below mutates exactly one condition word —
// the row names the gate it walks.
func actionPreviewBase() actionDomain.ActionDefinition {
	return actionDomain.ActionDefinition{
		ID:      "preview.matrix-ready",
		Version: "1.0.0",
		Name:    "Матричная проверка подготовки",
		Purpose: "Пройти полную цепочку подготовки по живым источникам мира",
		TypedInputSchema: []actionDomain.FieldSchema{
			{Name: "phone", Label: "Телефон", Type: actionDomain.FieldText, Required: true, Description: "Номер в формате E.164"},
			{Name: "template_url", Label: "Ссылка на шаблон", Type: actionDomain.FieldURL, Secret: true, Description: "Секретное поле — не проецируется"},
		},
		TypedOutputSchema:  []actionDomain.FieldSchema{{Name: "status", Label: "Статус", Type: actionDomain.FieldText}},
		PackRef:            actionDomain.PackRef{PackID: "auto-parts", PackVersion: "2.0.0"},
		RequiredCapability: actionDomain.CapabilityRef{ConnectorID: "staffy-synthetic-telephony", ConnectorVersion: "1.1.0", CapabilityID: "query.trunk-status"},
		OwningSystem:       "erp",
		Conditions: actionDomain.Conditions{
			RiskClass:                "low",
			AutonomyClass:            "assisted",
			AllowedConversationModes: []string{"unclassified"},
			AllowedRoles:             []string{"owner"},
			RequiredFacts:            []actionDomain.FactRequirement{{MaterialField: "order_status", Freshness: "fresh"}},
			Confirmation:             "not_required",
			RequiredGates:            []string{"OQ-7B"},
			TimeoutMS:                5000,
			MaterialityClass:         "immaterial",
			ConsumptionRule:          "unlimited",
			AllowedStages:            []string{"synthetic"},
		},
		DeliveryContract: actionDomain.DeliveryContract{
			CapabilityClass:       actionDomain.CapabilityQueryReconcilable,
			IdempotencyScope:      "не требуется (read-only)",
			TerminalEvidence:      "query_result",
			ReconciliationQuery:   "query.trunk-status",
			CancelCompensationSem: "не применимо (read-only)",
		},
	}
}

// The named variants of the matrix definitions (each mutates exactly the
// word of the gate its row walks — the vocabulary of the live world).
var (
	actionMatrixPreviewReady = actionPreviewBase()
	// actionMatrixPreviewUnsafe: the unsafe delivery class — the
	// human_only partition with the honest markers (the preparation never
	// blocks on the unsafe class).
	actionMatrixPreviewUnsafe = func() actionDomain.ActionDefinition {
		def := actionPreviewBase()
		def.ID = "preview.matrix-unsafe"
		def.DeliveryContract.CapabilityClass = actionDomain.CapabilityUnsafe
		def.Conditions.Confirmation = "required"
		def.Conditions.ConsumptionRule = "single_use"
		def.DeliveryContract.CancelCompensationSem = "вручную: созвон с клиентом и отметка в журнале"
		return def
	}()
	// actionMatrixPreviewTolerant: the DECLARED stale tolerance of the
	// order_status field — the explicit allowed_uncertain word of the
	// row-14 mapping pass.
	actionMatrixPreviewTolerant = func() actionDomain.ActionDefinition {
		def := actionPreviewBase()
		def.ID = "preview.matrix-tolerant"
		def.Conditions.RequiredFacts = []actionDomain.FactRequirement{{MaterialField: "order_status", Freshness: "stale"}}
		return def
	}()
	// actionMatrixPreviewRole: the role gate — the owner is not allowed.
	actionMatrixPreviewRole = func() actionDomain.ActionDefinition {
		def := actionPreviewBase()
		def.ID = "preview.matrix-role"
		def.Conditions.AllowedRoles = []string{"operator"}
		return def
	}()
	// actionMatrixPreviewMode: the conversation-mode gate — the fresh
	// case stays unclassified, the definition demands claim.
	actionMatrixPreviewMode = func() actionDomain.ActionDefinition {
		def := actionPreviewBase()
		def.ID = "preview.matrix-mode"
		def.Conditions.AllowedConversationModes = []string{"claim"}
		return def
	}()
	// actionMatrixPreviewStage: the stage gate — the world employee
	// publishes synthetic, the definition demands the higher stage.
	actionMatrixPreviewStage = func() actionDomain.ActionDefinition {
		def := actionPreviewBase()
		def.ID = "preview.matrix-stage"
		def.Conditions.AllowedStages = []string{"closed_account_test"}
		return def
	}()
	// actionMatrixPreviewGate: the gate row — the live OPEN AG-1 instance
	// is the honest non-pass source of the required gate.
	actionMatrixPreviewGate = func() actionDomain.ActionDefinition {
		def := actionPreviewBase()
		def.ID = "preview.matrix-gate"
		def.Conditions.RequiredGates = []string{"AG-1"}
		return def
	}()
	// actionMatrixPreviewSMS: the capability gate — the mutating send-sms
	// capability is AG-4-guarded and never verifies in the test contour.
	actionMatrixPreviewSMS = func() actionDomain.ActionDefinition {
		def := actionPreviewBase()
		def.ID = "preview.matrix-sms"
		def.RequiredCapability.CapabilityID = "command.send-sms"
		def.Conditions.Confirmation = "required"
		def.Conditions.ConsumptionRule = "single_use"
		def.DeliveryContract.CapabilityClass = actionDomain.CapabilityProviderIdempotent
		def.DeliveryContract.IdempotencyScope = "connector-ack"
		def.DeliveryContract.TerminalEvidence = "delivery-receipt"
		def.DeliveryContract.ReconciliationQuery = "query.trunk-status"
		def.DeliveryContract.CancelCompensationSem = "не компенсируется — отказ до подтверждения"
		return def
	}()
)

// countingReader wraps the synthetic reader of the test contour with the
// call counter of the block-path probe (the reader sees ZERO calls on
// every policy-block path — the unit counters pinned the order, the
// live rows pin the words).
type countingReader struct {
	inner *owningsystem.Synthetic
	calls int64
}

func (c *countingReader) FreshReads(ctx context.Context, request actionPort.FreshReadRequest) ([]actionDomain.MaterialSnapshotRow, error) {
	atomic.AddInt64(&c.calls, 1)
	return c.inner.FreshReads(ctx, request)
}

func (c *countingReader) reset()       { atomic.StoreInt64(&c.calls, 0) }
func (c *countingReader) total() int64 { return atomic.LoadInt64(&c.calls) }

// lyingDryRunner is the dishonest adapter probe: the foreign result word
// refuses at the app guard — the adapter never widens the vocabulary.
type lyingDryRunner struct{}

func (lyingDryRunner) Validate(context.Context, actionPort.DryRunRequest) (actionPort.DryRunOutcome, error) {
	return actionPort.DryRunOutcome{Result: "super-valid", Details: "ложь адаптера"}, nil
}

// buildActionPreviewService wires the REAL app service of the Story 3-4
// contour over the shared cases world: the REAL append-only store over
// the LIVE PG (the migration 00001 — RLS, the CHECK backstops, the
// advisory locks), the REAL composition control adapter of the
// preparation route over the LIVE 3.2 control store, the LIVE fact
// sources of the world (the cases view and facts, the identity member
// facts, the active entitlement, the employee readiness and the release
// lineage, the connector capability facts) and the AG-4 adapters of the
// test contour (the nil reader is the honest blocked_material, the nil
// runner is the spoken refusal — production wires neither).
func (w *casesWorld) buildActionPreviewService(reader actionPort.OwningSystemReader, runner actionPort.DryRunner, ids *sequenceIDs) *actionApp.Service {
	packs, caps := packresolver.New(), capabilityresolver.New()
	defs := []actionDomain.ActionDefinition{
		actionMatrixPreviewReady, actionMatrixPreviewUnsafe, actionMatrixPreviewTolerant,
		actionMatrixPreviewRole, actionMatrixPreviewMode, actionMatrixPreviewStage,
		actionMatrixPreviewGate, actionMatrixPreviewSMS,
	}
	// The admission walk at construction (the fixture discipline: a
	// dishonest dictionary panics before the first command).
	if err := actionDomain.CatalogIntegrity(defs); err != nil {
		panic("the matrix dictionary must pass the guard: " + err.Error())
	}
	for _, def := range defs {
		if err := actionDomain.ValidateDefinitionRefs(def, packs, caps); err != nil {
			panic("the matrix dictionary must resolve against the live catalogs: " + err.Error())
		}
	}
	service := actionApp.New(w.identity, packs, caps, defs).
		WithPreviews(actionStore.NewPostgres(runtimePool),
			controleffectiveresolver.NewRouteAdapter(controlStore.NewPostgres(runtimePool), fixedClock{actionPreviewNow}),
			w.cases, w.identity, w.entitlement, w.employee, w.connector, w.employee, ids)
	if reader != nil {
		service = service.WithPreviewReader(reader)
	}
	if runner != nil {
		service = service.WithPreviewDryRunner(runner)
	}
	return service
}

// actionPreviewWorld builds the matrix world: the cases world of the
// suite plus the REAL action preparation contour with the counting
// reader (the block-path probe) and the scriptable dry-runner of the
// test contour.
func actionPreviewWorld(t *testing.T, seed int) (*casesWorld, *actionApp.Service, *owningsystem.Synthetic, *dryrunner.Synthetic, *countingReader) {
	t.Helper()
	w := newCasesWorld(t, seed)
	// The 512-value id pool of the matrix world (the 3-3 precedent: the
	// 64-id pool of the suite world dies at ~7 ingests — the eight cases
	// of the stale rows share ONE pool with the commands).
	w.cases = w.buildCasesServiceClock(actionPreviewIDs(seed*10+500, 512), fixedClock{actionPreviewNow})
	reader := owningsystem.New()
	runner := dryrunner.New()
	counter := &countingReader{inner: reader}
	service := w.buildActionPreviewService(counter, runner, actionPreviewIDs(seed*10+400, 512))
	return w, service, reader, runner, counter
}

// previewSubmitCommand builds the honest submit command of the matrix
// rows over the permissive base conditions of the given definition.
func previewSubmitCommand(w *casesWorld, actor identityDomain.Principal, caseID, workItemID, targetRef id.UUID,
	def actionDomain.ActionDefinition, typed map[string]string, facts, observations []id.UUID, ownerEpoch int64) actionAPI.SubmitProposalCommand {
	return actionAPI.SubmitProposalCommand{
		Actor:                  actor,
		TenantID:               w.tenant,
		CaseID:                 caseID,
		WorkItemID:             workItemID,
		DefinitionID:           def.ID,
		DefinitionVersion:      def.Version,
		Target:                 actionDomain.ProposalTarget{Kind: actionDomain.TargetOrder, Ref: targetRef},
		TypedInput:             typed,
		SupportingFacts:        facts,
		SupportingObservations: observations,
		OwnerEpoch:             ownerEpoch,
		Now:                    actionPreviewNow,
	}
}

// previewViolationsSpeak reports whether the violation list speaks every
// given word fragment (the honest word list of the typed violations).
func previewViolationsSpeak(violations []string, fragments ...string) bool {
	joined := strings.Join(violations, " | ")
	for _, fragment := range fragments {
		if !strings.Contains(joined, fragment) {
			return false
		}
	}
	return true
}

// previewThreadsOf reads the full Preview Review view of the case (the
// honest shorthand of the matrix rows).
func previewThreadsOf(t *testing.T, svc *actionApp.Service, actor identityDomain.Principal, tenant, caseID id.UUID) []actionAPI.PreviewThreadView {
	t.Helper()
	view, err := svc.ReadPreviewView(context.Background(), actor, tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	return view.Threads
}

// previewCardOf pins one exact version card of the thread (the honest
// shorthand of the matrix rows).
func previewCardOf(t *testing.T, svc *actionApp.Service, actor identityDomain.Principal, tenant, caseID, previewID id.UUID, versionNo int64) actionAPI.PreviewCardView {
	t.Helper()
	for _, thread := range previewThreadsOf(t, svc, actor, tenant, caseID) {
		if thread.PreviewID != previewID {
			continue
		}
		for _, card := range thread.Cards {
			if card.VersionNo == versionNo {
				return card
			}
		}
	}
	t.Fatalf("the pinned card %s version %d does not exist", previewID, versionNo)
	return actionAPI.PreviewCardView{}
}

// previewConfirmedFact records and confirms one fact of the case over the
// REAL cases service (the live fact version the freshness derivation
// re-checks) and returns the fact anchor for the supporting pins.
func previewConfirmedFact(t *testing.T, w *casesWorld, caseID, workItemID id.UUID, materialKey, value, evidenceEvent, keyNo string) id.UUID {
	t.Helper()
	recorded, err := w.cases.RecordFact(context.Background(), casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: materialKey, Value: []byte(value),
		SourceType: casesDomain.FactSourceChannelEvent, SourceID: eventIDOf(t, w, caseID, evidenceEvent).String(),
		Reason: "Клиент назвал значение", IdempotencyKey: "preview-fact-" + keyNo, Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := w.cases.ConfirmFactVersion(context.Background(), casesAPI.ConfirmFactVersionCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: recorded.FactID,
		VersionNo: recorded.FactVersion, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
		EvidenceEventID: eventIDOf(t, w, caseID, evidenceEvent),
		IdempotencyKey:  "preview-confirm-" + keyNo, Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.FactStatus != casesDomain.FactStatusConfirmed {
		t.Fatalf("expected the confirmed fact, got %+v", confirmed)
	}
	return recorded.FactID
}

// previewReadStale asserts the exact version reads stale on the live
// derivation while staying fully readable for the audit (the digest
// never moves, the Timeline never moves — the suitability dies through
// the derivation, never through a deletion).
func previewReadStale(t *testing.T, svc *actionApp.Service, actor identityDomain.Principal, tenant, caseID, previewID id.UUID, versionNo int64) {
	t.Helper()
	card := previewCardOf(t, svc, actor, tenant, caseID, previewID, versionNo)
	if !card.FreshnessStale {
		t.Fatalf("the moved live source reads stale honestly, got %+v", card.FreshnessReasonText)
	}
	if card.DigestAlarm {
		t.Fatal("the audit readability never depends on the live sources — the digest re-verifies")
	}
}

// previewPublishSecondProfile moves the release lineage of the world
// LIVE: the second approved profile version over the SAME sealed
// manifest link and the acting release decision that pins it (the
// profile version pin of the freshness derivation moves 1 → 2). The
// draft CAS: the first approval cycle left the profile draft at revision
// 2 (the approval bumped it to approved+1 — the profile test precedent).
func previewPublishSecondProfile(t *testing.T, w *casesWorld) {
	t.Helper()
	ctx := context.Background()
	owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
	content := profileContent(w.owner.ID, w.backup)
	content.WorkScope.Limits.MaxConcurrentConversations = 5
	draft, err := w.saveDraft(t, w.employeeID, w.manifestLink, content, 2, owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.approve(t, w.employeeID, draft.Revision, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
		Actor: owner.principal(), TenantID: w.tenant, EmployeeID: w.employeeID,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000b661"),
		Stage:         employeeDomain.StageSynthetic,
	}); err != nil {
		t.Fatal(err)
	}
}

// previewConnectionVersion reads the current version of the world
// connection (the CAS anchor of the rotate command).
func previewConnectionVersion(t *testing.T, w *casesWorld) int64 {
	t.Helper()
	connection, err := w.connector.Read(context.Background(), w.owner, w.tenant, w.connection)
	if err != nil {
		t.Fatal(err)
	}
	return connection.Version
}

// previewMovePolicyOutcome moves the CURRENT policy outcome number of the
// work item LIVE (the full 2.12 chain over the REAL cases service: one
// proposal and one evaluation with the current anchors).
func previewMovePolicyOutcome(t *testing.T, w *casesWorld, caseID, workItemID id.UUID, sessionID, eventID string) {
	t.Helper()
	// The turn event continues the SAME session of the case (the session
	// continuation word — a fresh session id would open a NEW case).
	turn := w.modeTurnEvent(t, caseID, sessionID, eventID, `{"say":"интересует стоимость работ"}`)
	conversationID := w.conversationOf(t, caseID)
	proposal := w.policySubmit(t, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Диагностика от 3000",
		MissingFacts: []casesDomain.MissingFact{
			{MaterialKey: "service_kind", Reason: casesDomain.MissingReasonMandatory},
		},
		ExtractedFacts: []casesDomain.ExtractedFact{
			{MaterialKey: "service_kind", ValueDigest: "preview-digest", SourceEventID: turn},
		},
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "preview-policy-1", Now: actionPreviewNow,
	})
	if proposal.Replay || !proposal.ProposalID.Valid() {
		t.Fatalf("expected the fresh policy proposal, got %+v", proposal)
	}
	modeVersion, ownerEpoch, workItemVersion, factsDigest := w.policyAnchors(t, caseID, workItemID)
	w.policyEvaluateOK(t, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ProposalID: proposal.ProposalID, ExpectedModeVersion: modeVersion, ExpectedOwnerEpoch: ownerEpoch,
		ExpectedFactsDigest: factsDigest, ExpectedWorkItemVersion: workItemVersion,
		IdempotencyKey: "preview-policy-eval-1", Now: actionPreviewNow,
	})
}

// conversationOf reads the first conversation id of the case (the anchor
// of the policy chain).
func (w *casesWorld) conversationOf(t *testing.T, caseID id.UUID) id.UUID {
	t.Helper()
	view, err := w.cases.CaseView(context.Background(), w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Conversations) == 0 {
		t.Fatal("the case carries no conversation for the policy chain")
	}
	return view.Conversations[0].ConversationID
}
