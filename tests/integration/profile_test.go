//go:build integration

package integration_test

import (
        "context"
        "errors"
        "fmt"
        "strings"
        "sync"
        "testing"
        "time"

        employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
        employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
        identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
        "github.com/staffy-co/staffy/internal/platform/id"
)

// profileWorld extends the release world with a sealed manifest (the exact
// link target of the operating profile) and a backup owner member.
type profileWorld struct {
        *releaseWorld
        backup id.UUID
}

// newProfileWorld wires a full profile world: employee with phone+pack,
// sealed manifest v1, and an invited member (reviewer) as the backup owner.
func newProfileWorld(t *testing.T, seed int) (*profileWorld, id.UUID, employeeDomain.ManifestLink) {
        t.Helper()
        w := newReleaseWorld(t, seed)
        backup := inviteMember(t, w.identity, w.owner, w.tenant, fmt.Sprintf("profile-backup-%03d@example.org", seed), "expert_qa", []identityDomain.Scope{identityDomain.Review, identityDomain.Audit}, fmt.Sprintf("00000000%04d", seed*10+7), fmt.Sprintf("0000000%05d", seed*10+8))
        employee := w.createEmployee(t, true)
        w.pinPack(t, employee)
        w.saveBehavior(t, employee, 3)
        draft := w.assemble(t, employee, 0)
        version := w.createVersion(t, employee, draft.Revision, "Профильная конфигурация")
        manifest, err := w.seal(t, employee, version.Version, 2)
        if err != nil {
                t.Fatal(err)
        }
        link := employeeDomain.ManifestLink{ManifestID: manifest.ManifestID, ConfigVersion: manifest.ConfigVersion, Digest: manifest.Digest}
        return &profileWorld{releaseWorld: w, backup: backup.ID}, employee, link
}

// profileContent builds a complete valid content for the world: phone+web_chat
// routes, coverage by owner/backup, privacy assignments (recording in use with
// PASS), ops level, metric, gate and a complete matrix over the auto-parts
// journeys.
func profileContent(owner, backup id.UUID) employeeDomain.ProfileContent {
        return employeeDomain.ProfileContent{
                WorkScope: employeeDomain.WorkScope{
                        Routes: []employeeDomain.Route{{Channel: "phone", Direction: employeeDomain.DirectionInbound}, {Channel: "web_chat", Direction: employeeDomain.DirectionInbound}},
                        Forbidden: []employeeDomain.ForbiddenRoute{{Route: employeeDomain.Route{Channel: "telegram", Direction: employeeDomain.DirectionOutbound}, Reason: "Канал не подключён."}},
                        Languages: []string{"ru"},
                        Window:    employeeDomain.WorkWindow{Days: []string{"mon", "tue"}, Start: "09:00", End: "18:00", TimeZone: "Europe/Moscow"},
                        Limits: employeeDomain.WorkLimits{MaxConcurrentConversations: 4, MaxConcurrentCalls: 2, PerRoute: []employeeDomain.RouteLimit{
                                {Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound}, MaxConcurrentConversations: 2, MaxConcurrentCalls: 1},
                                {Route: employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound}, MaxConcurrentConversations: 2},
                        }},
                        StopConditions: []string{"Достижение hard limit бюджета.", "Перевод Entitlement в paused."},
                },
                Coverage: []employeeDomain.HumanCoverage{
                        {Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound}, ResponseOwnerID: owner, BackupOwnerID: backup,
                                Days: []string{"mon", "tue"}, Start: "09:00", End: "18:00", AckDeadlineMinutes: 15, Fallback: "voicemail", FallbackEvidence: "capability://jivo/voicemail/verified"},
                        {Route: employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound}, ResponseOwnerID: owner, BackupOwnerID: backup,
                                Days: []string{"mon", "tue"}, Start: "09:00", End: "18:00", AckDeadlineMinutes: 10, Fallback: "human_callback"},
                },
                Privacy: []employeeDomain.PrivacyAssignment{
                        {Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound}, Purpose: "recording", InUse: true,
                                LegalBasis: "Договор с клиентом", ConsentRef: "consent://tenant/recording", Disclosure: "Озвучивается в начале звонка",
                                Retention: "90 дней", Deletion: "Workflow Story 1-10", Withdrawal: "Остановка по запросу", OQ11State: employeeDomain.OQ11Pass, OQ11Evidence: "privacy://oq11/recording-pass"},
                        {Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound}, Purpose: "stt_model_processing", InUse: false, OQ11State: employeeDomain.OQ11Open},
                        {Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound}, Purpose: "media_processing", InUse: false, OQ11State: employeeDomain.OQ11Open},
                        {Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound}, Purpose: "transfer", InUse: false, OQ11State: employeeDomain.OQ11Open},
                        {Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound}, Purpose: "marketing_contact", InUse: false, OQ11State: employeeDomain.OQ11Open},
                        {Route: employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound}, Purpose: "recording", InUse: false, OQ11State: employeeDomain.OQ11Open},
                        {Route: employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound}, Purpose: "stt_model_processing", InUse: false, OQ11State: employeeDomain.OQ11Open},
                        {Route: employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound}, Purpose: "media_processing", InUse: false, OQ11State: employeeDomain.OQ11Open},
                        {Route: employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound}, Purpose: "transfer", InUse: false, OQ11State: employeeDomain.OQ11Open},
                        {Route: employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound}, Purpose: "marketing_contact", InUse: false, OQ11State: employeeDomain.OQ11Open},
                },
                OpsLevel: employeeDomain.OperationalLevel{Level: "best_effort"},
                Metrics: []employeeDomain.MetricDefinition{{
                        ID: "csat", Name: "CSAT после закрытия", BusinessMeaning: "Удовлетворённость клиента",
                        Numerator: "Положительные оценки", Denominator: "Все оценки с ответом", SourceOfTruth: "Опрос после закрытия case",
                        TerminalTiming: "Через 24 часа", Slices: []string{"journey", "modality", "conversation_mode", "release_version", "autonomy_class"},
                        OwnerID: owner, DisputePolicy: "Разногласия решает владелец метрики", RouteFailureVisibility: "Доля route-blocking failure показывается отдельной строкой",
                }},
                Gates: []employeeDomain.GateDeclaration{{
                        GateID: "knowledge_eval", EvidenceOwnerID: owner, DecisionDeadline: "До старта пилота",
                        FailClosedFallback: "block_route", AllowRoleCoincidence: false,
                }},
                Matrix: []employeeDomain.CoverageCell{
                        {Journey: "поиск детали", Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound}, Modality: employeeDomain.ModalityVoice, ConversationMode: "sales", AutonomyClass: "assisted",
                                EvalRef: "eval://auto-parts/search/voice-v1", Gates: []string{"knowledge_eval"}, HumanFallback: "human_callback по регламенту"},
                        {Journey: "поиск детали", Route: employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound}, Modality: employeeDomain.ModalityText, ConversationMode: "sales", AutonomyClass: "assisted",
                                EvalRef: "eval://auto-parts/search/text-v1", HumanFallback: "human_callback по регламенту"},
                        {Journey: "уточнение аналога", Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound}, Modality: employeeDomain.ModalityVoice, ConversationMode: "sales", AutonomyClass: "assisted",
                                NotAdmitted: true, NotAdmittedReason: "Голосовой сценарий аналогов требует доработки pack."},
                        {Journey: "уточнение аналога", Route: employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound}, Modality: employeeDomain.ModalityText, ConversationMode: "sales", AutonomyClass: "assisted",
                                EvalRef: "eval://auto-parts/analog/text-v1", HumanFallback: "human_callback по регламенту"},
                },
                Modes:           []string{"sales"},
                AutonomyClasses: []string{"assisted"},
        }
}

// saveDraft creates or CAS-updates the profile draft.
func (w *profileWorld) saveDraft(t *testing.T, employee id.UUID, link employeeDomain.ManifestLink, content employeeDomain.ProfileContent, expected int64, actor actorPrincipal) (employeeDomain.ProfileDraft, error) {
        t.Helper()
        return w.employee.UpdateDraft(context.Background(), employeeAPI.ProfileDraftCommand{
                Actor: actor.principal(), TenantID: w.tenant, EmployeeID: employee,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000a01"),
                Manifest: link, Content: content, ExpectedRevision: expected,
        })
}

// actorPrincipal abstracts owner/step-up owner so the helpers stay readable.
type actorPrincipal struct {
        principalID id.UUID
        stepUp      bool
}

func (a actorPrincipal) principal() identityDomain.Principal {
        return identityDomain.Principal{ID: a.principalID, StepUpVerified: a.stepUp}
}

// approve approves the draft under the expected revision.
func (w *profileWorld) approve(t *testing.T, employee id.UUID, expected int64, actor actorPrincipal) (employeeDomain.ProfileVersion, error) {
        t.Helper()
        return w.employee.ApproveProfile(context.Background(), employeeAPI.ProfileApproveCommand{
                Actor: actor.principal(), TenantID: w.tenant, EmployeeID: employee,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000a02"),
                Description: "Утверждение профиля v1", EvidenceRefs: []string{"manifest-digest://" + "reference", "threshold://v1"},
                ExpectedRevision: expected,
        })
}

// TestProfileDraftApprovalFlowImmutable: the honest walk — draft with the
// exact manifest link, step-up approval creating the immutable version with
// digest and approver roles, post-approval edits surfacing as unapproved
// changes while the approved version stays intact.
func TestProfileDraftApprovalFlowImmutable(t *testing.T) {
        w, employee, link := newProfileWorld(t, 41)
        owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
        draft, err := w.saveDraft(t, employee, link, profileContent(w.owner.ID, w.backup), 0, owner)
        if err != nil {
                t.Fatal(err)
        }
        if draft.Revision != 1 || !draft.Manifest.Equal(link) {
                t.Fatalf("draft must pin the exact manifest link at revision 1: %+v", draft)
        }
        view, err := w.employee.ProfileView(context.Background(), w.owner, w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        if len(view.Readiness) != 0 {
                t.Fatalf("complete content must be ready, got: %v", view.Readiness)
        }
        if len(view.RouteStates) != 2 {
                t.Fatalf("two routes must be derived, got %d", len(view.RouteStates))
        }
        for _, state := range view.RouteStates {
                if state.State != employeeDomain.RouteStateClear {
                        t.Fatalf("both routes must be clear in the complete content: %+v", state)
                }
        }
        if view.Applicability.State != employeeDomain.ProfileApplicabilityNone {
                t.Fatalf("no approved version yet: %+v", view.Applicability)
        }
        version, err := w.approve(t, employee, draft.Revision, owner)
        if err != nil {
                t.Fatal(err)
        }
        if version.Version != 1 || len(version.Digest) != 64 || !version.StepUpVerified {
                t.Fatalf("approval must create version 1 with a sha256 digest and the step-up fact: %+v", version)
        }
        if len(version.ApproverRoles) == 0 || version.ApproverRoles[0] != "owner" {
                t.Fatalf("approver roles must be recorded at approval time: %v", version.ApproverRoles)
        }
        digest, err := employeeDomain.ProfileDigest(version.Content)
        if err != nil {
                t.Fatal(err)
        }
        if digest != version.Digest {
                t.Fatalf("the digest must be re-computable from the approved content: %s vs %s", digest, version.Digest)
        }
        // Post-approval: the draft is editable again (the approval bumped the
        // revision to approved+1); the approved version never changes;
        // applicability surfaces unapproved changes.
        content := profileContent(w.owner.ID, w.backup)
        content.WorkScope.Languages = []string{"ru", "en"}
        if _, err := w.saveDraft(t, employee, link, content, draft.Revision+1, owner); err != nil {
                t.Fatal(err)
        }
        view, err = w.employee.ProfileView(context.Background(), w.owner, w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        if view.Applicability.State != employeeDomain.ProfileApplicabilityUnapproved {
                t.Fatalf("post-approval edits must surface as unapproved changes: %+v", view.Applicability)
        }
        reread, err := w.employee.ProfileView(context.Background(), w.owner, w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        if reread.Approved == nil || reread.Approved.Digest != version.Digest || reread.Approved.Version != 1 {
                t.Fatalf("the approved version must stay immutable: %+v", reread.Approved)
        }
}

// TestProfileReadinessAndStepUpRefusals: approval without step-up is an
// honest refusal; approval of an incomplete draft is refused with the list
// leading to concrete fields; the draft save itself is NOT rejected for
// missing pieces.
func TestProfileReadinessAndStepUpRefusals(t *testing.T) {
        w, employee, link := newProfileWorld(t, 42)
        noStepUp := actorPrincipal{principalID: w.owner.ID, stepUp: false}
        content := profileContent(w.owner.ID, w.backup)
        content.OpsLevel.Level = ""
        content.WorkScope.StopConditions = nil
        draft, err := w.saveDraft(t, employee, link, content, 0, noStepUp)
        if err != nil {
                t.Fatalf("a draft with missing mandatory pieces must save honestly: %v", err)
        }
        view, err := w.employee.ProfileView(context.Background(), w.owner, w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        found := map[string]bool{}
        for _, problem := range view.Readiness {
                found[problem.Section+"."+problem.Field] = true
                if problem.Kind != employeeDomain.ProfileProblemMissing && problem.Kind != employeeDomain.ProfileProblemAdvisory {
                        t.Fatalf("missing pieces must be readiness gaps, not save rejections: %+v", problem)
                }
        }
        if !found["operational_level.level"] || !found["work_scope.admission_stop_conditions"] {
                t.Fatalf("the readiness list must lead to the concrete unfilled fields: %v", found)
        }
        // Approval without step-up: honest refusal before anything else.
        if _, err := w.approve(t, employee, draft.Revision, noStepUp); !errors.Is(err, employeeDomain.ErrProfileStepUpRequired) {
                t.Fatalf("approval without step-up must refuse honestly, got %v", err)
        }
        // Approval without a single evidence reference: honest refusal (the
        // contract's minItems is enforced server-side, symmetric to thresholds).
        if _, err := w.employee.ApproveProfile(context.Background(), employeeAPI.ProfileApproveCommand{
                Actor: identityDomain.Principal{ID: w.owner.ID, StepUpVerified: true}, TenantID: w.tenant, EmployeeID: employee,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000a07"),
                Description: "Без ссылок", EvidenceRefs: nil, ExpectedRevision: draft.Revision,
        }); err == nil {
                t.Fatal("an approval without evidence references must be refused")
        } else {
                var rule employeeDomain.ProfileRuleError
                if !errors.As(err, &rule) || rule.Field != "evidence_refs" {
                        t.Fatalf("the refusal must lead to the evidence_refs field, got %v", err)
                }
        }
        // Approval with step-up but incomplete content: refused with the list
        // leading to concrete fields (the work_scope problems come first in the
        // validation flow, operational_level afterwards).
        stepUp := actorPrincipal{principalID: w.owner.ID, stepUp: true}
        _, err = w.approve(t, employee, draft.Revision, stepUp)
        var rule employeeDomain.ProfileRuleError
        if !errors.As(err, &rule) {
                t.Fatalf("incomplete content must refuse the approval with the rule list, got %v", err)
        }
        sections := map[string]bool{}
        for _, problem := range rule.Problems {
                sections[problem.Section+"."+problem.Field] = true
        }
        if !sections["work_scope.admission_stop_conditions"] || !sections["operational_level.level"] || len(rule.Problems) < 2 {
                t.Fatalf("the refusal must carry the list leading to the concrete fields: %+v", rule.Problems)
        }
}

// TestProfileInvalidValuesRejectTheSave: present but invalid values (route
// outside the dictionary, undeclared gate reference, journey outside the
// pack) reject the draft save — no defaults are substituted.
func TestProfileInvalidValuesRejectTheSave(t *testing.T) {
        w, employee, link := newProfileWorld(t, 43)
        owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
        content := profileContent(w.owner.ID, w.backup)
        content.WorkScope.Routes = append(content.WorkScope.Routes, employeeDomain.Route{Channel: "smoke_signal", Direction: employeeDomain.DirectionInbound})
        if _, err := w.saveDraft(t, employee, link, content, 0, owner); err == nil {
                t.Fatal("a route outside the dictionary must reject the save")
        }
        content = profileContent(w.owner.ID, w.backup)
        content.Matrix[0].Gates = []string{"undeclared_gate"}
        if _, err := w.saveDraft(t, employee, link, content, 0, owner); err == nil {
                t.Fatal("a cell referencing an undeclared gate must reject the save")
        }
        content = profileContent(w.owner.ID, w.backup)
        content.Matrix[0].Journey = "неизвестный journey"
        if _, err := w.saveDraft(t, employee, link, content, 0, owner); err == nil {
                t.Fatal("a journey outside the pack must reject the save")
        }
}

// TestProfileManifestLinkFailClosed: a draft referencing a missing manifest
// or a wrong digest is honestly refused — the draft is not saved.
func TestProfileManifestLinkFailClosed(t *testing.T) {
        w, employee, link := newProfileWorld(t, 44)
        owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
        missing := link
        missing.ManifestID = id.MustParse("01994f64-6c00-7000-8000-000000000abc")
        if _, err := w.saveDraft(t, employee, missing, profileContent(w.owner.ID, w.backup), 0, owner); err == nil {
                t.Fatal("a missing manifest must be an honest refusal")
        }
        wrongDigest := link
        wrongDigest.Digest = strings.Repeat("0", 64)
        if _, err := w.saveDraft(t, employee, wrongDigest, profileContent(w.owner.ID, w.backup), 0, owner); err == nil {
                t.Fatal("a digest mismatch must be an honest refusal")
        }
}

// TestProfileRouteBlockingDerivations: a missing coverage row, an unproven
// fallback and an open OQ-11 assignment block only the affected route — the
// other routes keep living; blocked routes never prevent the approval.
func TestProfileRouteBlockingDerivations(t *testing.T) {
        w, employee, link := newProfileWorld(t, 45)
        owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
        content := profileContent(w.owner.ID, w.backup)
        // phone: unproven fallback (advisory) — the route is blocked; web_chat
        // stays clear. The approval still succeeds (blocked routes are visible
        // honestly; admission is 1-14).
        content.Coverage[0].Fallback = "voicemail"
        content.Coverage[0].FallbackEvidence = ""
        draft, err := w.saveDraft(t, employee, link, content, 0, owner)
        if err != nil {
                t.Fatal(err)
        }
        view, err := w.employee.ProfileView(context.Background(), w.owner, w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        blocked := map[string]string{}
        for _, state := range view.RouteStates {
                if state.State == employeeDomain.RouteStateBlocked {
                        blocked[state.Route.String()] = state.Reason
                }
        }
        if len(blocked) != 1 {
                t.Fatalf("exactly the phone route must be blocked, got %v", blocked)
        }
        if !strings.Contains(blocked["phone:inbound"], "подтверждённым handoff") {
                t.Fatalf("the block reason must name the unproven fallback: %s", blocked["phone:inbound"])
        }
        if _, err := w.approve(t, employee, draft.Revision, owner); err != nil {
                t.Fatalf("a blocked route must never prevent the approval: %v", err)
        }
}

// TestProfileApprovalRaceProducesExactlyOneVersion: two concurrent approvals
// of the same revision — exactly one immutable version, the loser gets the
// honest conflict with the current state.
func TestProfileApprovalRaceProducesExactlyOneVersion(t *testing.T) {
        w, employee, link := newProfileWorld(t, 46)
        owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
        draft, err := w.saveDraft(t, employee, link, profileContent(w.owner.ID, w.backup), 0, owner)
        if err != nil {
                t.Fatal(err)
        }
        const attempts = 4
        results := make(chan error, attempts)
        start := make(chan struct{})
        var wg sync.WaitGroup
        for i := 0; i < attempts; i++ {
                wg.Add(1)
                go func() {
                        defer wg.Done()
                        <-start
                        _, err := w.employee.ApproveProfile(context.Background(), employeeAPI.ProfileApproveCommand{
                                Actor: owner.principal(), TenantID: w.tenant, EmployeeID: employee,
                                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000a03"),
                                Description: "Гонка утверждений", EvidenceRefs: []string{"manifest-digest://race"},
                                ExpectedRevision: draft.Revision,
                        })
                        results <- err
                }()
        }
        close(start)
        wg.Wait()
        close(results)
        approved, conflicts := 0, 0
        for err := range results {
                var conflict employeeDomain.ProfileConflictError
                switch {
                case err == nil:
                        approved++
                case errors.As(err, &conflict):
                        conflicts++
                default:
                        t.Fatalf("the loser must get an honest conflict, got %v", err)
                }
        }
        if approved != 1 || conflicts != attempts-1 {
                t.Fatalf("exactly one approval must win: approved=%d conflicts=%d", approved, conflicts)
        }
        view, err := w.employee.ProfileView(context.Background(), w.owner, w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        if view.Approved == nil || view.Approved.Version != 1 {
                t.Fatalf("exactly one version 1 must exist: %+v", view.Approved)
        }
}

// TestProfileStaleOnNewManifest: sealing a new manifest makes the approved
// profile stale for the new configuration version — an honest derivation,
// never an on-the-spot migration.
func TestProfileStaleOnNewManifest(t *testing.T) {
        w, employee, link := newProfileWorld(t, 47)
        owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
        draft, err := w.saveDraft(t, employee, link, profileContent(w.owner.ID, w.backup), 0, owner)
        if err != nil {
                t.Fatal(err)
        }
        if _, err := w.approve(t, employee, draft.Revision, owner); err != nil {
                t.Fatal(err)
        }
        view, err := w.employee.ProfileView(context.Background(), w.owner, w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        if view.Applicability.State != employeeDomain.ProfileApplicabilityCurrent {
                t.Fatalf("freshly approved profile must be current: %+v", view.Applicability)
        }
        // A component edit → new config version → new sealed manifest: the
        // approved profile becomes stale for the new version.
        w.saveBehavior(t, employee, 4)
        nextDraft := w.assemble(t, employee, 3)
        nextVersion := w.createVersion(t, employee, nextDraft.Revision, "Изменение поведения")
        newManifest, err := w.seal(t, employee, nextVersion.Version, nextDraft.Revision+1)
        if err != nil {
                t.Fatal(err)
        }
        view, err = w.employee.ProfileView(context.Background(), w.owner, w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        if view.Applicability.State != employeeDomain.ProfileApplicabilityStale {
                t.Fatalf("a new sealed manifest must make the profile stale: %+v", view.Applicability)
        }
        if view.LatestManifest == nil || view.LatestManifest.ConfigVersion != newManifest.ConfigVersion {
                t.Fatalf("the latest manifest must be visible for comparison: %+v", view.LatestManifest)
        }
}

// TestThresholdHistoryAndEvidenceOwnerOnly: the threshold starts honestly at
// pending baseline; only the declared evidence owner decides; the history is
// append-only with supersedes links.
func TestThresholdHistoryAndEvidenceOwnerOnly(t *testing.T) {
        w, employee, link := newProfileWorld(t, 48)
        owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
        draft, err := w.saveDraft(t, employee, link, profileContent(w.owner.ID, w.backup), 0, owner)
        if err != nil {
                t.Fatal(err)
        }
        if _, err := w.approve(t, employee, draft.Revision, owner); err != nil {
                t.Fatal(err)
        }
        view, err := w.employee.ProfileView(context.Background(), w.owner, w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        if len(view.Thresholds) != 1 || view.Thresholds[0].State != employeeDomain.ThresholdStatePendingBaseline {
                t.Fatalf("the threshold must start pending baseline, got %+v", view.Thresholds)
        }
        // A configurator with the configuration scope who is NOT the declared
        // evidence owner is honestly refused — the powers do not substitute
        // the declared human decision owner.
        configurator := inviteMember(t, w.identity, w.owner, w.tenant, fmt.Sprintf("profile-configurator-%03d@example.org", 48), "configurator", []identityDomain.Scope{identityDomain.Configuration}, "000000004807", "000000004808")
        if _, err := w.employee.DecideThreshold(context.Background(), employeeAPI.ThresholdCommand{
                Actor: identityDomain.Principal{ID: configurator.ID}, TenantID: w.tenant, EmployeeID: employee,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000a04"),
                GateID: "knowledge_eval", Value: "0.9", Decision: "Порог по baseline", EvidenceRefs: []string{"baseline://eval-1"},
        }); err == nil {
                t.Fatal("a non-evidence-owner must be honestly refused")
        } else {
                var rule employeeDomain.ProfileRuleError
                if !errors.As(err, &rule) || !strings.Contains(rule.Reason, "evidence owner") {
                        t.Fatalf("the refusal must name the evidence owner rule, got %v", err)
                }
        }
        // The declared evidence owner decides; the history grows append-only.
        first, err := w.employee.DecideThreshold(context.Background(), employeeAPI.ThresholdCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000a05"),
                GateID: "knowledge_eval", Value: "0.85", Decision: "Порог по контрольному baseline", EvidenceRefs: []string{"baseline://eval-1", "eval://auto-parts/report"},
        })
        if err != nil {
                t.Fatal(err)
        }
        if first.ThresholdVersion != 1 || first.Supersedes != nil {
                t.Fatalf("the first decision is version 1 without supersedes: %+v", first)
        }
        second, err := w.employee.DecideThreshold(context.Background(), employeeAPI.ThresholdCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000a06"),
                GateID: "knowledge_eval", Value: "0.90", Decision: "Уточнение порога по новому eval", EvidenceRefs: []string{"baseline://eval-2"},
        })
        if err != nil {
                t.Fatal(err)
        }
        if second.ThresholdVersion != 2 || second.Supersedes == nil || second.Supersedes.ThresholdVersion != 1 {
                t.Fatalf("the second decision must supersede version 1: %+v", second)
        }
        if second.Supersedes.DecidedAt.IsZero() {
                t.Fatal("the supersedes link must carry the exact decided-at of the superseded version")
        }
        history, err := w.employee.ThresholdHistory(context.Background(), w.owner, w.tenant, employee, "knowledge_eval")
        if err != nil {
                t.Fatal(err)
        }
        if len(history.Decisions) != 2 {
                t.Fatalf("the history must keep both decisions append-only, got %d", len(history.Decisions))
        }
        view, err = w.employee.ProfileView(context.Background(), w.owner, w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        if view.Thresholds[0].State != employeeDomain.ThresholdStateDecided || view.Thresholds[0].Value != "0.90" {
                t.Fatalf("the latest decision must win: %+v", view.Thresholds[0])
        }
}

// TestProfileScopeAndTenantBoundaries: viewing requires configuration,
// review or audit; managing requires configuration; a non-member gets a
// plain 404; another tenant stays invisible.
func TestProfileScopeAndTenantBoundaries(t *testing.T) {
        w, employee, link := newProfileWorld(t, 49)
        owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
        if _, err := w.saveDraft(t, employee, link, profileContent(w.owner.ID, w.backup), 0, owner); err != nil {
                t.Fatal(err)
        }
        reviewer := identityDomain.Principal{ID: w.backup}
        if _, err := w.employee.ProfileView(context.Background(), reviewer, w.tenant, employee); err != nil {
                t.Fatalf("a member with review must view the profile: %v", err)
        }
        // A member without any of the three scopes: honest 403.
        outsider := inviteMember(t, w.identity, w.owner, w.tenant, fmt.Sprintf("profile-outsider-%03d@example.org", 49), "operator", []identityDomain.Scope{identityDomain.Operations}, "000000004907", "000000004908")
        if _, err := w.employee.ProfileView(context.Background(), identityDomain.Principal{ID: outsider.ID}, w.tenant, employee); !errors.Is(err, employeeDomain.ErrProfileForbidden) {
                t.Fatalf("a member without the scopes must be honestly refused, got %v", err)
        }
        // A non-member: a plain 404 without tenant disclosure.
        stranger := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-00000000000f")}
        if _, err := w.employee.ProfileView(context.Background(), stranger, w.tenant, employee); !errors.Is(err, employeeDomain.ErrNotFound) {
                t.Fatalf("a non-member must get a plain 404, got %v", err)
        }
        // Cross-tenant: another tenant's employee stays invisible.
        other := newReleaseWorld(t, 50)
        if _, err := other.employee.ProfileView(context.Background(), other.owner, w.tenant, employee); !errors.Is(err, employeeDomain.ErrNotFound) {
                t.Fatalf("another tenant must not see the profile, got %v", err)
        }
}

// TestProfileWriteAuditAndGrants: every write lands in audit; the runtime
// role cannot mutate the append-only tables (UPDATE/DELETE are revoked).
func TestProfileWriteAuditAndGrants(t *testing.T) {
        w, employee, link := newProfileWorld(t, 51)
        owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
        draft, err := w.saveDraft(t, employee, link, profileContent(w.owner.ID, w.backup), 0, owner)
        if err != nil {
                t.Fatal(err)
        }
        if _, err := w.approve(t, employee, draft.Revision, owner); err != nil {
                t.Fatal(err)
        }
        ctx := context.Background()
        var outcomes []string
        rows, err := adminPool.Query(ctx, "select outcome from employee.audit where tenant_id=$1 and employee_id=$2 and outcome in ('profile_draft_saved','profile_approved','threshold_decided') order by occurred_at, resulting_version", w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        defer rows.Close()
        for rows.Next() {
                var outcome string
                if err := rows.Scan(&outcome); err != nil {
                        t.Fatal(err)
                }
                outcomes = append(outcomes, outcome)
        }
        if len(outcomes) < 2 || outcomes[0] != "profile_draft_saved" || outcomes[len(outcomes)-1] != "profile_approved" {
                t.Fatalf("the audit must record the honest outcomes in order: %v", outcomes)
        }
        // The append-only tables resist UPDATE and DELETE under the runtime role.
        for _, statement := range []string{
                "update employee.operating_profiles set description='подделка' where tenant_id=$1",
                "delete from employee.operating_profiles where tenant_id=$1",
                "update employee.gate_thresholds set value='0.1' where tenant_id=$1",
                "delete from employee.gate_thresholds where tenant_id=$1",
        } {
                if _, err := runtimePool.Exec(ctx, statement, w.tenant); err == nil {
                        t.Fatalf("the append-only table must resist the mutation: %s", statement)
                }
        }
        // Cross-tenant write is blocked by RLS: the transaction carries the real
        // tenant setting (transaction-scoped), and the policy rejects the foreign
        // tenant_id row — the cause is discriminated, not a constraint accident.
        rlsTx, err := runtimePool.Begin(ctx)
        if err != nil {
                t.Fatal(err)
        }
        defer rlsTx.Rollback(ctx)
        if _, err = rlsTx.Exec(ctx, "select set_config('staffy.tenant_id',$1,true)", w.tenant.String()); err != nil {
                t.Fatal(err)
        }
        foreign := id.MustParse("01994f64-6c00-7000-8000-0000000000ac")
        foreignManifestID, err := id.NewV7(time.Now().UTC())
        if err != nil {
                t.Fatal(err)
        }
        if _, err = rlsTx.Exec(ctx, "insert into employee.operating_profile_drafts(tenant_id,employee_id,revision,manifest_id,config_version,manifest_digest,content,actor_id,updated_at) values($1,$2,1,$3,$4,$5,'{}'::jsonb,$6,now())",
                foreign, employee, foreignManifestID, int64(1), strings.Repeat("a", 64), w.owner.ID); err == nil {
                t.Fatal("RLS must block cross-tenant inserts into the profile drafts")
        } else if !strings.Contains(err.Error(), "row-level security") {
                t.Fatalf("cross-tenant insert must fail on the RLS policy, got: %v", err)
        }
}

// TestProfileOpenPrivacyBlocksOnlyAffectedRoute: an OQ-11 open assignment in
// use blocks only the affected route fail-closed; the other route keeps
// living; the explicit not-used assignments never block anything.
func TestProfileOpenPrivacyBlocksOnlyAffectedRoute(t *testing.T) {
        w, employee, link := newProfileWorld(t, 52)
        owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
        content := profileContent(w.owner.ID, w.backup)
        // recording on phone is in use with an OQ-11 open state: phone blocks,
        // web_chat lives.
        content.Privacy[0].OQ11State = employeeDomain.OQ11Open
        content.Privacy[0].OQ11Evidence = ""
        if _, err := w.saveDraft(t, employee, link, content, 0, owner); err != nil {
                t.Fatal(err)
        }
        view, err := w.employee.ProfileView(context.Background(), w.owner, w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        blocked := map[string]string{}
        for _, state := range view.RouteStates {
                if state.State == employeeDomain.RouteStateBlocked {
                        blocked[state.Route.String()] = state.Reason
                }
        }
        if len(blocked) != 1 {
                t.Fatalf("only the affected route must be blocked, got %v", blocked)
        }
        if !strings.Contains(blocked["phone:inbound"], "OQ-11") {
                t.Fatalf("the block reason must name the open OQ-11 state: %s", blocked["phone:inbound"])
        }
}
