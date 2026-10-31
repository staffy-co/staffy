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

        connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
        connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
        employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
        employeeApp "github.com/staffy-co/staffy/internal/employee/app"
        employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
        employeeStore "github.com/staffy-co/staffy/internal/employee/store"
        entitlementAPI "github.com/staffy-co/staffy/internal/entitlement/api"
        entitlementApp "github.com/staffy-co/staffy/internal/entitlement/app"
        entitlementDomain "github.com/staffy-co/staffy/internal/entitlement/domain"
        entitlementStore "github.com/staffy-co/staffy/internal/entitlement/store"
        identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
        "github.com/staffy-co/staffy/internal/platform/id"
)

// readinessWorld extends the profile world with everything the candidate
// needs to be honestly complete: an ACTIVE entitlement (the admission fact),
// a knowledge source, a saved voice version, a connected synthetic telephony
// connection (contract 1.0.0 declares no capabilities — the honest state of
// the only creatable connector), a second sealed manifest carrying all of it
// and an approved operating profile over that exact manifest.
type readinessWorld struct {
        *profileWorld
        entitlement *entitlementApp.Service
}

func newReadinessWorld(t *testing.T, seed int) (*readinessWorld, id.UUID, employeeDomain.ManifestLink) {
        t.Helper()
        w, employee, _ := newProfileWorld(t, seed)
        entitlement := entitlementApp.New(entitlementStore.NewPostgres(runtimePool), w.identity, releaseClock, newPrivacyIDs(seed+700))
        w.employee = w.employee.WithEntitlementFacts(entitlement)
        world := &readinessWorld{profileWorld: w, entitlement: entitlement}
        world.activateEntitlement(t)
        world.addKnowledge(t, employee)
        world.addVoice(t, employee)
        world.connectSyntheticTelephony(t)
        // The second manifest carries the knowledge, the voice version and the
        // connection; the profile is approved over exactly this one.
        draft := w.assemble(t, employee, 3)
        version := w.createVersion(t, employee, draft.Revision, "Кандидат публикации")
        manifest, err := w.seal(t, employee, version.Version, draft.Revision+1)
        if err != nil {
                t.Fatal(err)
        }
        link := employeeDomain.ManifestLink{ManifestID: manifest.ManifestID, ConfigVersion: manifest.ConfigVersion, Digest: manifest.Digest}
        owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
        if _, err := w.saveDraft(t, employee, link, profileContent(w.owner.ID, w.backup), 0, owner); err != nil {
                t.Fatal(err)
        }
        if _, err := w.approve(t, employee, 1, owner); err != nil {
                t.Fatal(err)
        }
        return world, employee, link
}

// activateEntitlement composes confirmed terms and activates the commercial
// perimeter so the admission fact is honestly "ok".
func (w *readinessWorld) activateEntitlement(t *testing.T) {
        t.Helper()
        ctx := context.Background()
        start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
        end := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
        launch, subscription := 49000.0, 15000.0
        if _, err := w.entitlement.ComposeTerms(ctx, entitlementAPI.TermsCommand{
                Actor: w.owner, TenantID: w.tenant,
                Currency: "RUB", TaxRegime: "УСН", PeriodStart: &start, PeriodEnd: &end,
                IncludedEmployees:  []string{"consultant-01"},
                IncludedChannels:   []string{"jivo"},
                IncludedConnectors: []string{"abcp"},
                Limits:             []entitlementDomain.CategoryLimit{{Category: entitlementDomain.CategoryCarrier, Cap: 1000}},
                LaunchPrice:        &launch, SubscriptionPrice: &subscription,
                ChangeNote:    "Коммерческое предложение для публикации",
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000701"),
        }); err != nil {
                t.Fatal(err)
        }
        if _, err := w.entitlement.RunStateTransition(ctx, entitlementAPI.StateCommand{
                Actor: w.owner, TenantID: w.tenant, Action: entitlementDomain.EventActivate,
                Basis: "Договор №77 от 01.10.2026, подтвердил Олег", Confirm: true,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000702"),
        }); err != nil {
                t.Fatal(err)
        }
}

// addKnowledge ingests the synthetic fixture document to the preview state
// (the manifest ref counts non-retired sources).
func (w *readinessWorld) addKnowledge(t *testing.T, employee id.UUID) {
        t.Helper()
        ctx := context.Background()
        corr := id.MustParse("01994f64-6c00-7000-8000-000000000703")
        source, _, err := w.employee.IngestKnowledge(ctx, employeeAPI.KnowledgeIngestCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr,
                Source:  employeeDomain.KnowledgeSource{Type: "document", Title: "Синтетический документ", Owner: "Тест", Provenance: "Фикстура", Scope: "Тест", Priority: 50, Language: "ru"},
                Fixture: true,
        })
        if err != nil {
                t.Fatal(err)
        }
        if _, _, _, err := w.employee.ReingestSyntheticKnowledge(ctx, employeeAPI.KnowledgeFixtureCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr,
                SourceID: source.SourceID, ExpectedVersion: source.Version, FixtureID: "sample-v1",
        }); err != nil {
                t.Fatal(err)
        }
}

// addVoice saves one voice version so the manifest's voice ref is present
// for the phone route of the profile.
func (w *readinessWorld) addVoice(t *testing.T, employee id.UUID) {
        t.Helper()
        ctx := context.Background()
        corr := id.MustParse("01994f64-6c00-7000-8000-000000000704")
        if _, err := w.employee.SaveVoice(ctx, employeeAPI.VoiceCommand{
                Actor: w.owner, TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr,
                ExpectedVersion: 4, ChangeSummary: "Голос кандидата",
                Config: employeeDomain.VoiceConfig{ProfileID: "ru-neutral-1", Language: "ru", Style: "neutral", Tempo: 100,
                        Dictionary: []employeeDomain.Pronunciation{{Written: "VIN", Language: "ru", Scope: "Тест", Normalized: "вин", TestPhrase: "Прочтите VIN", SupportStatus: "unverified", CriticalKind: "vin", SyntheticDataConfirmed: true,
                                Readback: &employeeDomain.ReadbackPlan{Segments: []string{"V", "I", "N"}, Confirmation: "ask_customer", OnMismatch: "human_handoff", PhoneQualityGate: "pending_human"}}},
                }}); err != nil {
                t.Fatal(err)
        }
}

// connectSyntheticTelephony creates and connects the only creatable
// connector at contract 1.0.0: the version that declares no capabilities, so
// the connection facts carry no unverified capability (the 1.1.0 contract
// keeps send-sms behind AG-4 and inbound-call without a verification path —
// an honest eternal capability block of the slice).
func (w *readinessWorld) connectSyntheticTelephony(t *testing.T) id.UUID {
        t.Helper()
        ctx := context.Background()
        created, err := w.connector.Create(ctx, connectorAPI.CreateCommand{
                Actor: w.owner, TenantID: w.tenant, IdempotencyKey: fmt.Sprintf("readiness-connection-%d", time.Now().UnixNano()),
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000705"),
                ConnectorID:   "staffy-synthetic-telephony", ConnectorVer: "1.2.0",
                Label: "Телефония кандидата", Environment: connectorDomain.EnvSandbox,
                Config: map[string]string{"endpoint": "https://readiness.sandbox.example/api"},
        })
        if err != nil {
                t.Fatal(err)
        }
        connected, err := w.connector.Connect(ctx, connectorAPI.ConnectCommand{
                Actor: w.owner, TenantID: w.tenant, ConnectionID: created.Connection.ConnectionID,
                CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-000000000706"),
                IdempotencyKey:  fmt.Sprintf("readiness-connect-%d", time.Now().UnixNano()),
                ExpectedVersion: created.Connection.Version, Secrets: map[string]string{"api_key": "sandbox-readiness-key"},
        })
        if err != nil {
                t.Fatal(err)
        }
        // The acting capability: the read-only probe verified with live
        // evidence. send-sms (AG-4-guarded) and inbound-call (no probe path)
        // stay honest notes of the element; their discipline is the AG-4 gate
        // of the stages from controlled_real_customer up, not this element.
        if _, err := w.connector.VerifyCapability(ctx, connectorAPI.VerifyCommand{
                Actor: w.owner, TenantID: w.tenant, ConnectionID: created.Connection.ConnectionID,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000707"),
                Capability:    "query.trunk-status", IdempotencyKey: fmt.Sprintf("readiness-verify-%d", time.Now().UnixNano()),
        }); err != nil {
                t.Fatal(err)
        }
        return connected.Connection.ConnectionID
}

// readinessActor builds a step-up owner principal for commands.
func (w *readinessWorld) readinessActor(stepUp bool) identityDomain.Principal {
        return identityDomain.Principal{ID: w.owner.ID, StepUpVerified: stepUp}
}

// TestReadinessDerivationAndPublication walks the honest demo path of the
// spec: the full candidate derives READY for synthetic, the ladder refuses
// skips and repeats honestly, synthetic and closed_account_test publish
// end-to-end (step-up, atomicity, digests), and the higher stages stay
// blocked by the eternal gates/eval facts — with the viewer and the
// non-member boundaries on the way.
func TestReadinessDerivationAndPublication(t *testing.T) {
        w, employee, link := newReadinessWorld(t, 61)
        ctx := context.Background()
        corr := id.MustParse("01994f64-6c00-7000-8000-000000000710")

        // The dry-run derivation for synthetic: every element pass, both routes
        // admitted, the overall status ready. Nothing was recorded yet.
        view, err := w.employee.ReadinessView(ctx, w.readinessActor(true), w.tenant, employee, employeeDomain.StageSynthetic)
        if err != nil {
                t.Fatal(err)
        }
        if view.Stage != employeeDomain.StageSynthetic || view.Report.Status != employeeDomain.ReadinessReady {
                t.Fatalf("synthetic view: stage=%s status=%s text=%q", view.Stage, view.Report.Status, view.Report.StatusText)
        }
        states := map[string]string{}
        for _, element := range view.Report.Elements {
                states[element.Key] = element.State
        }
        for _, key := range []string{"operating_profile", "release_manifest", "brief", "domain_pack", "knowledge", "behavior", "voice", "connections", "capabilities", "entitlement", "human_coverage", "consent"} {
                if states[key] != employeeDomain.ElementPass {
                        t.Fatalf("element %s = %s (%q), want pass", key, states[key], states[key])
                }
        }
        if states["gates"] != employeeDomain.ElementNotApplicable {
                t.Fatalf("synthetic gates element = %s, want not_applicable", states["gates"])
        }
        if states["eval"] != employeeDomain.ElementLinkedNotExecuted {
                t.Fatalf("eval element = %s, want linked_not_executed (no eval surfaces exist)", states["eval"])
        }
        for _, route := range view.Report.Routes {
                if route.State != employeeDomain.RouteAdmitted || len(route.Reasons) != 0 {
                        t.Fatalf("route %s = %s with %d blocks, want admitted", route.Route, route.State, len(route.Reasons))
                }
        }
        if view.Active != nil || len(view.History) != 0 {
                t.Fatalf("the view is a dry-run: nothing may be recorded, active=%v history=%d", view.Active != nil, len(view.History))
        }

        // The viewer boundary: the review-scope member reads, cannot manage.
        backup := identityDomain.Principal{ID: w.backup, StepUpVerified: true}
        viewerView, err := w.employee.ReadinessView(ctx, backup, w.tenant, employee, employeeDomain.StageSynthetic)
        if err != nil || viewerView.CanManage || viewerView.ViewScope != employeeAPI.ReadinessScopeView {
                t.Fatalf("viewer view: err=%v can_manage=%v scope=%s", err, viewerView.CanManage, viewerView.ViewScope)
        }
        if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                Actor: backup, TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr, Stage: employeeDomain.StageSynthetic,
        }); !errors.Is(err, employeeDomain.ErrReadinessForbidden) {
                t.Fatalf("viewer publish must be honestly forbidden, got %v", err)
        }

        // The non-member boundary: a foreign principal gets a plain 404 (the
        // tenant existence is never disclosed).
        foreign := newReleaseWorld(t, 62)
        if _, err := foreign.employee.ReadinessView(ctx, foreign.owner, w.tenant, employee, employeeDomain.StageSynthetic); !errors.Is(err, employeeDomain.ErrNotFound) {
                t.Fatalf("foreign view must be not_found, got %v", err)
        }

        // Step-up is required for every command of the surface.
        if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                Actor: w.readinessActor(false), TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr, Stage: employeeDomain.StageSynthetic,
        }); !errors.Is(err, employeeDomain.ErrReadinessStepUpRequired) {
                t.Fatalf("publication without step-up must be refused, got %v", err)
        }

        // The ladder: a closed stage without the previous decision is a skip.
        if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr, Stage: employeeDomain.StageClosedAccountTest,
        }); err == nil {
                t.Fatal("a stage skip must be refused")
        } else {
                var rule employeeDomain.ProfileRuleError
                if !errors.As(err, &rule) {
                        t.Fatalf("stage skip must be a rule refusal, got %v", err)
                }
                if !strings.Contains(err.Error(), "synthetic") {
                        t.Fatalf("the skip refusal must name the first stage: %v", err)
                }
        }

        // The synthetic publication: an append-only decision with the exact
        // candidate digests and the per-route results.
        synthetic, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr, Stage: employeeDomain.StageSynthetic,
        })
        if err != nil {
                t.Fatal(err)
        }
        if synthetic.DecisionNo != 1 || synthetic.Stage != employeeDomain.StageSynthetic || !synthetic.StepUpVerified {
                t.Fatalf("synthetic decision = %+v", synthetic)
        }
        if !synthetic.Manifest.Equal(link) {
                t.Fatalf("the decision must pin the exact manifest: %v", synthetic.Manifest)
        }
        if employeeDomain.AdmittedRouteCount(synthetic) != len(synthetic.Routes) {
                t.Fatalf("every route must be admitted: %+v", synthetic.Routes)
        }
        if digest, err := employeeDomain.AdmissionDigest(synthetic); err != nil || digest != synthetic.Digest {
                t.Fatalf("the canonical digest must recompute: %v %s vs %s", err, digest, synthetic.Digest)
        }
        if len(synthetic.Digest) != 64 {
                t.Fatal("digest must be 64 hex chars")
        }

        // A repeat of the same stage is an honest conflict, never a second record.
        if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr, Stage: employeeDomain.StageSynthetic,
        }); err == nil {
                t.Fatal("a repeated publication of the same stage must conflict")
        } else {
                var conflict employeeDomain.AdmissionConflictError
                if !errors.As(err, &conflict) || conflict.Stage != employeeDomain.StageSynthetic || conflict.Current == nil || conflict.Current.DecisionNo != 1 {
                        t.Fatalf("the conflict must carry the current decision, got %v", err)
                }
        }

        // The entitlement admission fact is honest: a paused perimeter blocks
        // closed_account_test but never the already recorded history.
        if _, err := w.entitlement.RunStateTransition(ctx, entitlementAPI.StateCommand{
                Actor: w.owner, TenantID: w.tenant, Action: entitlementDomain.EventPause,
                Basis: "Пауза для проверки блокировки", Confirm: true,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000711"),
        }); err != nil {
                t.Fatal(err)
        }
        pausedView, err := w.employee.ReadinessView(ctx, w.readinessActor(true), w.tenant, employee, employeeDomain.StageClosedAccountTest)
        if err != nil {
                t.Fatal(err)
        }
        if states := elementStateMap(pausedView.Report); states["entitlement"] != employeeDomain.ElementBlocked {
                t.Fatalf("paused entitlement must block the element, got %s", states["entitlement"])
        }
        blockedByEntitlement := false
        for _, route := range pausedView.Report.Routes {
                for _, block := range route.Reasons {
                        if block.Cause == "entitlement" {
                                blockedByEntitlement = true
                        }
                }
                if route.State != employeeDomain.RouteBlocked {
                        t.Fatalf("paused entitlement must block every route at closed_account_test: %s", route.Route)
                }
        }
        if !blockedByEntitlement || pausedView.Report.Status != employeeDomain.ReadinessBlocked {
                t.Fatalf("paused closed_account_test view must be blocked, got %s", pausedView.Report.Status)
        }
        if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr, Stage: employeeDomain.StageClosedAccountTest,
        }); err == nil {
                t.Fatal("closed_account_test with a paused entitlement must refuse (zero admitted routes)")
        }
        if _, err := w.entitlement.RunStateTransition(ctx, entitlementAPI.StateCommand{
                Actor: w.owner, TenantID: w.tenant, Action: entitlementDomain.EventResume,
                Basis: "Возобновление после проверки", Confirm: true,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000712"),
        }); err != nil {
                t.Fatal(err)
        }

        // closed_account_test publishes end-to-end over the resumed perimeter.
        closed, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr, Stage: employeeDomain.StageClosedAccountTest,
        })
        if err != nil {
                t.Fatal(err)
        }
        if closed.DecisionNo != 2 || closed.ProfileVersion != synthetic.ProfileVersion || !closed.Manifest.Equal(synthetic.Manifest) {
                t.Fatalf("closed decision = %+v", closed)
        }

        // The higher stages stay honestly blocked: no gate PASS exists and eval
        // is linked, not executed — the eternal blocks of the slice.
        controlled, err := w.employee.ReadinessView(ctx, w.readinessActor(true), w.tenant, employee, employeeDomain.StageControlledRealCustomer)
        if err != nil {
                t.Fatal(err)
        }
        controlledStates := elementStateMap(controlled.Report)
        if controlledStates["gates"] != employeeDomain.ElementBlocked || controlledStates["eval"] != employeeDomain.ElementLinkedNotExecuted {
                t.Fatalf("controlled elements: gates=%s eval=%s, want blocked/linked_not_executed", controlledStates["gates"], controlledStates["eval"])
        }
        for _, route := range controlled.Report.Routes {
                if route.State != employeeDomain.RouteBlocked {
                        t.Fatalf("controlled route %s must be blocked", route.Route)
                }
        }
        if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr, Stage: employeeDomain.StageControlledRealCustomer,
        }); err == nil {
                t.Fatal("controlled_real_customer must refuse: zero admitted routes (empty publications do not exist)")
        } else {
                var rule employeeDomain.ProfileRuleError
                if !errors.As(err, &rule) {
                        t.Fatalf("the refusal must be a rule error with the per-route reasons, got %v", err)
                }
        }
        // live is a stage skip on top: the ladder requires controlled first.
        if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr, Stage: employeeDomain.StageLive,
        }); err == nil || !strings.Contains(err.Error(), "controlled_real_customer") {
                t.Fatalf("live must be refused as a stage skip naming the previous stage, got %v", err)
        }

        // The history: append-only, the acting pointer is the highest number.
        history, err := w.employee.AdmissionHistoryView(ctx, w.readinessActor(true), w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        if len(history.Admissions) != 2 || history.Active == nil || history.Active.DecisionNo != 2 {
                t.Fatalf("history = %d records, active №%d", len(history.Admissions), history.Active.DecisionNo)
        }

        // The gate registry view lists the fixed dictionary and the empty history.
        // Spec 4-1 extends the dictionary: 8 platform + 6 Oleg pilot definitions;
        // spec 5-1 adds the 6 Montagio definitions: 8 + 6 + 6 = 20.
        gates, err := w.employee.GateInstancesView(ctx, w.readinessActor(true), w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        if len(gates.Definitions) != 20 || len(gates.Instances) != 0 || !gates.CanManage {
                t.Fatalf("gate view: definitions=%d instances=%d can_manage=%v", len(gates.Definitions), len(gates.Instances), gates.CanManage)
        }
}

// TestPublicationRaceWritesExactlyOneRecord: two concurrent publications of
// the same candidate and stage serialize on the employee aggregate lock —
// exactly one append-only record wins, the competitor receives the honest
// conflict with the current decision.
func TestPublicationRaceWritesExactlyOneRecord(t *testing.T) {
        w, employee, _ := newReadinessWorld(t, 63)
        ctx := context.Background()
        const racers = 2
        results := make(chan error, racers)
        var start sync.WaitGroup
        start.Add(1)
        for index := 0; index < racers; index++ {
                go func() {
                        start.Wait()
                        _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                                Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee,
                                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000720"), Stage: employeeDomain.StageSynthetic,
                        })
                        results <- err
                }()
        }
        start.Done()
        wins, conflicts := 0, 0
        for index := 0; index < racers; index++ {
                if err := <-results; err == nil {
                        wins++
                } else {
                        var conflict employeeDomain.AdmissionConflictError
                        if errors.As(err, &conflict) {
                                conflicts++
                        } else {
                                t.Fatalf("the losing racer must surface an admission conflict, got %v", err)
                        }
                }
        }
        if wins != 1 || conflicts != racers-1 {
                t.Fatalf("race outcome: wins=%d conflicts=%d, want exactly one record", wins, conflicts)
        }
        history, err := w.employee.AdmissionHistoryView(ctx, w.readinessActor(true), w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        if len(history.Admissions) != 1 {
                t.Fatalf("exactly one record may exist, got %d", len(history.Admissions))
        }
}

// TestGateTransitionDisciplineAndHistory: the lifecycle walks the registry
// state machine (open → in_test → pass), the PASS discipline (role coverage
// by distinct active members, evidence references, the proof hash), the
// append-only chain with exactly one successor and the honest reopening of a
// settled record.
func TestGateTransitionDisciplineAndHistory(t *testing.T) {
        w, employee, _ := newReadinessWorld(t, 64)
        ctx := context.Background()
        corr := id.MustParse("01994f64-6c00-7000-8000-000000000730")
        owner := w.readinessActor(true)
        hash := strings.Repeat("a", 64)
        // The PASS payload travels only with the pass target of an in_test
        // predecessor (the registry state machine).
        passCommand := func(mutate func(*employeeAPI.GateTransitionCommand)) employeeAPI.GateTransitionCommand {
                base := employeeAPI.GateTransitionCommand{
                        Actor: owner, TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr,
                        DefinitionID: "AG-1", TargetState: employeeDomain.GateStatePass,
                        ApprovedBy: []employeeDomain.GateApproval{
                                {ParticipantID: w.owner.ID, Role: employeeDomain.GateRoleTechnicalLead},
                                {ParticipantID: w.backup, Role: employeeDomain.GateRoleOperations},
                        },
                        EvidenceRefs: []string{"evidence://ag-1/load-envelope"},
                        EvidenceHash: hash,
                }
                if mutate != nil {
                        mutate(&base)
                }
                return base
        }

        // Step-up is required for every state change.
        openCommand := employeeAPI.GateTransitionCommand{
                Actor: owner, TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr,
                DefinitionID: "AG-1", TargetState: employeeDomain.GateStateOpen,
        }
        if _, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
                Actor: w.readinessActor(false), TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr,
                DefinitionID: "AG-1", TargetState: employeeDomain.GateStateOpen,
        }); !errors.Is(err, employeeDomain.ErrReadinessStepUpRequired) {
                t.Fatalf("a transition without step-up must be refused, got %v", err)
        }
        // A route scope on a tenant-level definition is refused (exact scope) —
        // the scope discipline fires before any state is created.
        if _, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
                Actor: owner, TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr,
                DefinitionID: "AG-1", TargetState: employeeDomain.GateStateOpen,
                Route: &employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound},
        }); err == nil {
                t.Fatal("a route scope on AG-1 must be refused")
        }
        // OQ-11 requires the exact route scope.
        if _, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
                Actor: owner, TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr,
                DefinitionID: "OQ-11", TargetState: employeeDomain.GateStateOpen,
        }); err == nil {
                t.Fatal("OQ-11 without the route scope must be refused")
        }
        // A PASS cannot appear out of thin air: the new-instance states are open
        // and in_test only (the registry state machine).
        if _, err := w.employee.RecordGateTransition(ctx, passCommand(nil)); err == nil {
                t.Fatal("a new instance in state pass must be refused")
        }

        // The lifecycle: open → in_test.
        opened, err := w.employee.RecordGateTransition(ctx, openCommand)
        if err != nil {
                t.Fatal(err)
        }
        if opened.State != employeeDomain.GateStateOpen || opened.DefinitionID != "AG-1" || opened.SupersedesInstanceID != nil {
                t.Fatalf("opened instance = %+v", opened)
        }
        inTest, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
                Actor: owner, TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr,
                DefinitionID: "AG-1", TargetState: employeeDomain.GateStateInTest, Supersedes: &opened.InstanceID,
        })
        if err != nil {
                t.Fatal(err)
        }
        if inTest.State != employeeDomain.GateStateInTest || *inTest.SupersedesInstanceID != opened.InstanceID {
                t.Fatalf("in_test instance = %+v", inTest)
        }

        // The PASS discipline against the in_test predecessor.
        passAgainst := func(mutate func(*employeeAPI.GateTransitionCommand)) employeeAPI.GateTransitionCommand {
                command := passCommand(mutate)
                if command.Supersedes == nil {
                        command.Supersedes = &inTest.InstanceID
                }
                return command
        }
        // Coinciding principals without allow_role_coincidence are refused.
        if _, err := w.employee.RecordGateTransition(ctx, passAgainst(func(c *employeeAPI.GateTransitionCommand) {
                c.ApprovedBy = []employeeDomain.GateApproval{
                        {ParticipantID: w.owner.ID, Role: employeeDomain.GateRoleTechnicalLead},
                        {ParticipantID: w.owner.ID, Role: employeeDomain.GateRoleOperations},
                }
        })); err == nil {
                t.Fatal("coinciding principals must be refused")
        }
        // A missing role is named honestly.
        if _, err := w.employee.RecordGateTransition(ctx, passAgainst(func(c *employeeAPI.GateTransitionCommand) {
                c.ApprovedBy = c.ApprovedBy[:1]
        })); err == nil || !strings.Contains(err.Error(), employeeDomain.GateRoleOperations) {
                t.Fatalf("the missing role must be named, got %v", err)
        }
        // An approver outside the tenant membership is refused (MembersFacts).
        if _, err := w.employee.RecordGateTransition(ctx, passAgainst(func(c *employeeAPI.GateTransitionCommand) {
                c.ApprovedBy = []employeeDomain.GateApproval{
                        {ParticipantID: w.owner.ID, Role: employeeDomain.GateRoleTechnicalLead},
                        {ParticipantID: id.MustParse("01994f64-6c00-7000-8000-0000000007f1"), Role: employeeDomain.GateRoleOperations},
                }
        })); err == nil || !strings.Contains(err.Error(), "участником") {
                t.Fatalf("a foreign approver must be refused honestly, got %v", err)
        }
        // Empty evidence and a malformed hash are refused.
        if _, err := w.employee.RecordGateTransition(ctx, passAgainst(func(c *employeeAPI.GateTransitionCommand) {
                c.EvidenceRefs = nil
        })); err == nil {
                t.Fatal("a PASS without evidence must be refused")
        }
        if _, err := w.employee.RecordGateTransition(ctx, passAgainst(func(c *employeeAPI.GateTransitionCommand) {
                c.EvidenceHash = "zz"
        })); err == nil {
                t.Fatal("a non-hex hash must be refused")
        }

        // The full discipline records the PASS.
        first, err := w.employee.RecordGateTransition(ctx, passAgainst(nil))
        if err != nil {
                t.Fatal(err)
        }
        if first.State != employeeDomain.GateStatePass || first.DefinitionID != "AG-1" || *first.SupersedesInstanceID != inTest.InstanceID {
                t.Fatalf("PASS instance = %+v", first)
        }
        if len(first.ApprovedBy) != 2 || len(first.EvidenceRefs) != 1 || first.EvidenceHash != hash {
                t.Fatalf("the PASS payload must be sealed: %+v", first)
        }

        // The lifecycle: pass → expired (the first successor of the PASS).
        expired, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
                Actor: owner, TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr,
                DefinitionID: "AG-1", TargetState: employeeDomain.GateStateExpired, Supersedes: &first.InstanceID,
        })
        if err != nil {
                t.Fatal(err)
        }
        // A raced second successor of the same record is an honest gate conflict:
        // exactly one successor exists, the loser re-reads the current state.
        if _, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
                Actor: owner, TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr,
                DefinitionID: "AG-1", TargetState: employeeDomain.GateStateNotApplicable, Supersedes: &first.InstanceID,
                NotApplicableReason: "обход гонки",
        }); err == nil {
                t.Fatal("a second successor of the same record must conflict")
        } else {
                var conflict employeeDomain.GateConflictError
                if !errors.As(err, &conflict) || conflict.InstanceID != first.InstanceID {
                        t.Fatalf("the gate conflict must carry the instance, got %v", err)
                }
        }
        if expired.State != employeeDomain.GateStateExpired || expired.SupersedesInstanceID == nil || *expired.SupersedesInstanceID != first.InstanceID {
                t.Fatalf("expired instance = %+v", expired)
        }
        reopened, err := w.employee.RecordGateTransition(ctx, employeeAPI.GateTransitionCommand{
                Actor: owner, TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr,
                DefinitionID: "AG-1", TargetState: employeeDomain.GateStateInTest, Supersedes: &expired.InstanceID,
        })
        if err != nil {
                t.Fatal(err)
        }
        if reopened.State != employeeDomain.GateStateInTest || *reopened.SupersedesInstanceID != expired.InstanceID {
                t.Fatalf("reopened instance = %+v", reopened)
        }

        // The append-only history: every record is visible, the expired PASS is
        // never the acting permission.
        gates, err := w.employee.GateInstancesView(ctx, w.readinessActor(true), w.tenant, employee)
        if err != nil {
                t.Fatal(err)
        }
        if len(gates.Instances) != 5 {
                t.Fatalf("the chain must keep every record (open, in_test, pass, expired, reopened), got %d", len(gates.Instances))
        }
        if pass := employeeDomain.ValidPassForRoute("AG-1", gates.Instances, employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound}); pass != nil {
                t.Fatalf("an expired PASS is never a permission: %+v", pass)
        }
        // RLS: the foreign tenant sees nothing.
        foreign := newReleaseWorld(t, 65)
        if _, err := foreign.employee.GateInstancesView(ctx, foreign.owner, w.tenant, employee); !errors.Is(err, employeeDomain.ErrNotFound) {
                t.Fatalf("foreign gate view must be not_found, got %v", err)
        }
}

// TestReadinessReflectsStaleFactsImmediately: a newer sealed manifest makes
// the approved profile inapplicable at the next read — nothing is cached.
func TestReadinessReflectsStaleFactsImmediately(t *testing.T) {
        w, employee, _ := newReadinessWorld(t, 66)
        ctx := context.Background()
        before, err := w.employee.ReadinessView(ctx, w.readinessActor(true), w.tenant, employee, employeeDomain.StageSynthetic)
        if err != nil {
                t.Fatal(err)
        }
        if before.Report.Status != employeeDomain.ReadinessReady {
                t.Fatalf("before = %s, want ready", before.Report.Status)
        }
        // A second manifest exists: the profile pins the previous one.
        draft := w.assemble(t, employee, 6)
        version := w.createVersion(t, employee, draft.Revision, "Новый манифест — кандидат устарел")
        if _, err := w.seal(t, employee, version.Version, draft.Revision+1); err != nil {
                t.Fatal(err)
        }
        after, err := w.employee.ReadinessView(ctx, w.readinessActor(true), w.tenant, employee, employeeDomain.StageSynthetic)
        if err != nil {
                t.Fatal(err)
        }
        states := elementStateMap(after.Report)
        if states["operating_profile"] != employeeDomain.ElementBlocked {
                t.Fatalf("the stale profile must block the element immediately, got %s", states["operating_profile"])
        }
        if !strings.Contains(strings.ToLower(after.Applicability.State), "stale") {
                t.Fatalf("applicability must be stale, got %s", after.Applicability.State)
        }
        // R1/M3: a blocked element blocks the whole candidate — the stale
        // profile never publishes behind a ready-looking route table.
        if after.Report.Status != employeeDomain.ReadinessBlocked {
                t.Fatalf("the stale candidate must be blocked overall, got %s (%s)", after.Report.Status, after.Report.StatusText)
        }
        if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee,
                CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000741"), Stage: employeeDomain.StageSynthetic,
        }); err == nil {
                t.Fatal("a stale candidate must refuse the publication (a blocked element is not a ready state)")
        }
}

// TestPublicationDigestSurvivesProductionClock: the production clock carries
// nanoseconds while Postgres keeps microseconds — the decision time is
// truncated before the digest and the insert, so every read path (the
// history view, the readiness view and the in-transaction re-read of the
// next publication) keeps working after the first publication (R1/M1).
func TestPublicationDigestSurvivesProductionClock(t *testing.T) {
        w, employee, _ := newReadinessWorld(t, 67)
        ctx := context.Background()
        corr := id.MustParse("01994f64-6c00-7000-8000-000000000740")
        nanoClock := fixedClock{time.Now().UTC()}
        nano := employeeApp.New(employeeStore.NewPostgres(runtimePool), w.identity, nanoClock, &employeeIDs{}).
                WithConnectorFacts(w.connector).WithEntitlementFacts(w.entitlement)
        synthetic, err := nano.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr, Stage: employeeDomain.StageSynthetic,
        })
        if err != nil {
                t.Fatal(err)
        }
        if digest, err := employeeDomain.AdmissionDigest(synthetic); err != nil || digest != synthetic.Digest {
                t.Fatalf("the returned digest must recompute: %v %s vs %s", err, digest, synthetic.Digest)
        }
        // Every read path must stay honest after a nanosecond publication.
        if _, err := w.employee.AdmissionHistoryView(ctx, w.owner, w.tenant, employee); err != nil {
                t.Fatalf("the history read must not break on the digest guard: %v", err)
        }
        if _, err := w.employee.ReadinessView(ctx, w.owner, w.tenant, employee, employeeDomain.StageClosedAccountTest); err != nil {
                t.Fatalf("the readiness view must not break on the digest guard: %v", err)
        }
        // The next publication re-reads the history inside its transaction.
        if _, err := nano.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
                Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: employee, CorrelationID: corr, Stage: employeeDomain.StageClosedAccountTest,
        }); err != nil {
                t.Fatalf("the next publication must re-read the history without a digest failure: %v", err)
        }
}

// elementStateMap flattens the element list for assertions.
func elementStateMap(report employeeDomain.ReadinessReport) map[string]string {
        states := map[string]string{}
        for _, element := range report.Elements {
                states[element.Key] = element.State
        }
        return states
}
