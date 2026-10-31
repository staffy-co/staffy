package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

var readinessTenant = id.MustParse("01994f64-6c00-7000-8000-0000000000c0")
var readinessEmployee = id.MustParse("01994f64-6c00-7000-8000-0000000000c1")
var readinessApprover = id.MustParse("01994f64-6c00-7000-8000-0000000000c2")

// readinessFacts assembles a complete, valid candidate: the approved profile
// over a sealed manifest with all refs present, verified current connections
// and an "ok" entitlement admission.
func readinessFacts() ReadinessFacts {
	content := validProfileContent()
	digest, err := ProfileDigest(content)
	if err != nil {
		panic(err)
	}
	refs := ConfigRefs{
		Brief:    BriefRef{Status: RefPresent, DraftVersion: 3},
		Pack:     PackRef{Status: RefPresent, ID: "auto-parts", Version: "2.0.0"},
		Behavior: BehaviorRef{Status: RefPresent, Version: 2},
		Knowledge: KnowledgeRef{Status: RefPresent, Sources: []KnowledgeSourceRef{
			{SourceID: profileOwner, Version: 4},
		}},
		Voice: VoiceRef{Status: RefPresent, Version: 1},
	}
	connections := []ConnectionFact{{
		ConnectionID: id.MustParse("01994f64-6c00-7000-8000-0000000000c3"), ConnectorID: "jivo",
		ContractVersion: "1", AdapterBuild: "synthetic", Environment: "test", Status: "connected",
		Capabilities: []CapabilityFact{{Capability: "text_chat", State: "verified", Reason: "read_only_probe passed", Gap: false, Owner: "Technical Lead"}},
	}}
	manifestContent := BuildManifestContent(ConfigVersion{
		TenantID: readinessTenant, EmployeeID: readinessEmployee, VersionID: profileBackup, Version: 1,
		Description: "readiness fixture", ActorID: profileOwner, CreatedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), Refs: refs,
	}, connections, nil)
	manifestDigest, err := ManifestDigest(manifestContent)
	if err != nil {
		panic(err)
	}
	raw, err := CanonicalManifestJSON(manifestContent)
	if err != nil {
		panic(err)
	}
	read := ReadManifestPayload(raw, manifestDigest)
	approved := &ProfileVersion{
		TenantID: readinessTenant, EmployeeID: readinessEmployee, Version: 1,
		Manifest: ManifestLink{ManifestID: profileBackup, ConfigVersion: 1, Digest: manifestDigest},
		Content:  content, Digest: digest, ApproverID: profileOwner, ApproverRoles: []string{"owner"},
		StepUpVerified: true, ApprovedAt: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC),
	}
	link := approved.Manifest
	return ReadinessFacts{
		Approved: approved, ManifestRead: &read, LatestManifest: &link,
		CurrentConnections: []ConnectionFacts{{ConnectionID: connections[0].ConnectionID, Status: "connected", Capabilities: connections[0].Capabilities}},
		Entitlement:        AdmissionFact{Status: AdmissionOKEntitlement, Reason: "ok"},
		PackJourneys:       []string{"поиск детали", "уточнение аналога"},
		Instances:          []GateInstance{},
	}
}

// AdmissionOKEntitlement mirrors the entitlement ok state label.
const AdmissionOKEntitlement = "ok"

// passInstance builds an applicable PASS record of a definition.
func passInstance(definitionID string, route *Route, modality string) GateInstance {
	participantA, participantB := profileOwner, profileBackup
	definition, _ := GateDefinitionByID(definitionID)
	approvedBy := []GateApproval{}
	for index, role := range definition.RequiredApproverRoles {
		participant := participantA
		if index > 0 && !definition.AllowRoleCoincidence {
			participant = participantB
		}
		approvedBy = append(approvedBy, GateApproval{ParticipantID: participant, Role: role})
	}
	tested := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	return GateInstance{
		TenantID: readinessTenant, EmployeeID: readinessEmployee, InstanceID: readinessApprover,
		DefinitionID: definitionID, Route: route, Modality: modality, State: GateStatePass,
		ApprovedBy: approvedBy, EvidenceRefs: []string{"evidence://" + strings.ToLower(definitionID) + "/pass"},
		EvidenceHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		TestedAt:     &tested, ActorID: profileOwner, RecordedAt: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC),
	}
}

func TestGateDictionaryMatchesRegistry(t *testing.T) {
	dictionary := GateDictionary()
	// Spec 4-1 (Pilot Admission Registry) extends the dictionary: 8 platform
	// definitions + 6 pilot definitions {OQ-1..OQ-5, OQ-19}; spec 5-1
	// (Montagio Admission Registry) extends it again: + 6 Montagio
	// definitions {OQ-7A, OQ-8, OQ-9, OQ-10, OQ-13A, OQ-18} — the AG-4
	// of both pilot sets is the platform definition already present.
	if len(dictionary) != 20 {
		t.Fatalf("the registry dictionary carries 8 platform + 6 Oleg (spec 4-1) + 6 Montagio (spec 5-1) definitions, got %d", len(dictionary))
	}
	for _, definition := range dictionary {
		if len(definition.RequiredApproverRoles) == 0 {
			t.Fatalf("%s must name at least one approver role", definition.ID)
		}
		if definition.EvidenceRequirement == "" || definition.SafeState == "" {
			t.Fatalf("%s must carry the evidence requirement and the safe state", definition.ID)
		}
	}
	oq11, ok := GateDefinitionByID("OQ-11")
	if !ok || !oq11.RouteScoped || oq11.AllowRoleCoincidence {
		t.Fatalf("OQ-11 is route-scoped without role coincidence, got %+v", oq11)
	}
	oq12, _ := GateDefinitionByID("OQ-12")
	if !oq12.AllowRoleCoincidence {
		t.Fatalf("OQ-12 allows role coincidence per the registry")
	}
}

func TestStageMapFollowsExecutionOrder(t *testing.T) {
	text := Route{Channel: "web_chat", Direction: DirectionInbound}
	voice := Route{Channel: "phone", Direction: DirectionInbound}
	if gates := RequiredGateIDs(StageSynthetic, voice); len(gates) != 0 {
		t.Fatalf("synthetic requires no gates, got %v", gates)
	}
	if gates := RequiredGateIDs(StageClosedAccountTest, voice); len(gates) != 0 {
		t.Fatalf("closed_account_test requires no gates, got %v", gates)
	}
	if gates := RequiredGateIDs(StageControlledRealCustomer, text); strings.Join(gates, ",") != "OQ-11,AG-1,AG-2,AG-4" {
		t.Fatalf("controlled text route: OQ-11,AG-1,AG-2,AG-4, got %v", gates)
	}
	if gates := RequiredGateIDs(StageControlledRealCustomer, voice); strings.Join(gates, ",") != "OQ-11,AG-1,AG-2,AG-4,OQ-12,OQ-13B" {
		t.Fatalf("controlled voice route adds OQ-12,OQ-13B, got %v", gates)
	}
	if gates := RequiredGateIDs(StageLive, text); strings.Join(gates, ",") != "OQ-11,AG-1,AG-2,AG-4,OQ-14,OQ-7B" {
		t.Fatalf("live adds OQ-14,OQ-7B, got %v", gates)
	}
	if !StageRequiresEval(StageControlledRealCustomer) || StageRequiresEval(StageClosedAccountTest) {
		t.Fatalf("eval blocks controlled and up only")
	}
	if !StageRequiresEntitlementAdmission(StageClosedAccountTest) || StageRequiresEntitlementAdmission(StageSynthetic) {
		t.Fatalf("entitlement blocks closed_account_test and up, synthetic is the fixture exception")
	}
}

func TestValidateGateTransitionScopeDiscipline(t *testing.T) {
	// OQ-11 requires the exact route scope.
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "OQ-11", TargetState: GateStateOpen}, nil); err == nil {
		t.Fatal("OQ-11 without route scope must be rejected")
	}
	// A tenant-level definition rejects the route scope.
	route := Route{Channel: "web_chat", Direction: DirectionInbound}
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "AG-1", Route: &route, TargetState: GateStateOpen}, nil); err == nil {
		t.Fatal("AG-1 is tenant-level: a route scope must be rejected")
	}
	// Unknown definitions never enter the product surface. OQ-1..OQ-5 and
	// OQ-19 are dictionary words since spec 4-1; OQ-6..OQ-10 and AG-3 stay
	// project-planning artifacts.
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "OQ-6", TargetState: GateStateOpen}, nil); err == nil {
		t.Fatal("definitions outside the registry dictionary must be rejected")
	}
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "AG-3", TargetState: GateStateOpen}, nil); err == nil {
		t.Fatal("AG-3 stays a project-planning artifact — not carried into the product surface")
	}
	// The carried pilot gates are writable dictionary words now.
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "OQ-1", TargetState: GateStateOpen}, nil); err != nil {
		t.Fatalf("OQ-1 is a dictionary word of spec 4-1: %v", err)
	}
	// A new chain starts at open or in_test only.
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "AG-1", TargetState: GateStatePass, ApprovedBy: []GateApproval{{ParticipantID: profileOwner, Role: "TechnicalLead"}, {ParticipantID: profileBackup, Role: "Operations"}}, EvidenceRefs: []string{"evidence://ag-1"}, EvidenceHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, nil); err == nil {
		t.Fatal("creation directly in pass must be rejected (open → in_test → pass)")
	}
}

func TestValidateGateTransitionStateMachine(t *testing.T) {
	open := GateInstance{TenantID: readinessTenant, EmployeeID: readinessEmployee, InstanceID: readinessApprover, DefinitionID: "AG-1", State: GateStateOpen, ActorID: profileOwner, RecordedAt: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
	inTest := open
	inTest.InstanceID = profileOwner
	inTest.State = GateStateInTest
	inTest.SupersedesInstanceID = &readinessApprover
	// open → in_test is legal.
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "AG-1", TargetState: GateStateInTest, Supersedes: &readinessApprover}, []GateInstance{open}); err != nil {
		t.Fatalf("open → in_test must pass: %v", err)
	}
	// open → pass is illegal.
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "AG-1", TargetState: GateStatePass, Supersedes: &readinessApprover}, []GateInstance{open}); err == nil {
		t.Fatal("open → pass must be rejected")
	}
	// in_test → fail is legal without the PASS payload.
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "AG-1", TargetState: GateStateFail, Supersedes: &profileOwner}, []GateInstance{open, inTest}); err != nil {
		t.Fatalf("in_test → fail must pass: %v", err)
	}
	// pass → not_applicable requires a reason.
	passed := inTest
	passed.InstanceID = profileBackup
	passed.State = GateStatePass
	passed.SupersedesInstanceID = &profileOwner
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "AG-1", TargetState: GateStateNotApplicable, Supersedes: &profileBackup}, []GateInstance{open, inTest, passed}); err == nil {
		t.Fatal("not_applicable without a scoped reason must be rejected")
	}
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "AG-1", TargetState: GateStateNotApplicable, Supersedes: &profileBackup, NotApplicableReason: "Действие не затрагивает управляемые поверхности этого scope."}, []GateInstance{open, inTest, passed}); err != nil {
		t.Fatalf("not_applicable with a reason must pass: %v", err)
	}
	// A settled state never changes: reopening creates a fresh chain.
	failed := passed
	failed.InstanceID = readinessApprover
	failed.State = GateStateFail
	failed.SupersedesInstanceID = &profileBackup
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "AG-1", TargetState: GateStatePass, Supersedes: &readinessApprover}, []GateInstance{failed}); err == nil {
		t.Fatal("fail → pass rewrites the settled record: must be rejected")
	}
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "AG-1", TargetState: GateStateInTest, Supersedes: &readinessApprover}, []GateInstance{failed}); err != nil {
		t.Fatalf("reopening after fail (new chain at in_test with supersedes) must pass: %v", err)
	}
	// A transition of a scope mismatch is refused.
	webChat := Route{Channel: "web_chat", Direction: DirectionInbound}
	phone := Route{Channel: "phone", Direction: DirectionInbound}
	oq11 := passInstance("OQ-11", &webChat, "")
	if _, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "OQ-11", Route: &phone, TargetState: GateStateExpired, Supersedes: &oq11.InstanceID}, []GateInstance{oq11}); err == nil {
		t.Fatal("a transition against a different scope must be rejected")
	}
}

func TestValidateGateTransitionPassDiscipline(t *testing.T) {
	inTest := GateInstance{TenantID: readinessTenant, EmployeeID: readinessEmployee, InstanceID: readinessApprover, DefinitionID: "OQ-11", Route: &[]Route{{Channel: "phone", Direction: DirectionInbound}}[0], State: GateStateInTest, ActorID: profileOwner, RecordedAt: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
	spec := func(approvedBy []GateApproval, refs []string, hash string) GateTransitionSpec {
		return GateTransitionSpec{DefinitionID: "OQ-11", Route: inTest.Route, TargetState: GateStatePass, Supersedes: &readinessApprover, ApprovedBy: approvedBy, EvidenceRefs: refs, EvidenceHash: hash}
	}
	// One participant covering both roles without allow_role_coincidence is refused.
	samePrincipal := []GateApproval{{ParticipantID: profileOwner, Role: "SecurityPrivacy"}, {ParticipantID: profileOwner, Role: "TechnicalLead"}}
	if _, err := ValidateGateTransition(spec(samePrincipal, []string{"evidence://oq11"}, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), []GateInstance{inTest}); err == nil {
		t.Fatal("coinciding principals without allow_role_coincidence must be rejected")
	}
	// A missing role is named honestly.
	missingRole := []GateApproval{{ParticipantID: profileOwner, Role: "SecurityPrivacy"}}
	if _, err := ValidateGateTransition(spec(missingRole, []string{"evidence://oq11"}, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), []GateInstance{inTest}); err == nil || !strings.Contains(err.Error(), "TechnicalLead") {
		t.Fatalf("the missing role must be named, got %v", err)
	}
	// Empty evidence references are refused.
	if _, err := ValidateGateTransition(spec(nil, nil, ""), []GateInstance{inTest}); err == nil {
		t.Fatal("a PASS without roles, evidence and hash must be rejected")
	}
	// A malformed hash is refused.
	if _, err := ValidateGateTransition(spec(nil, []string{"evidence://oq11"}, "zz"), []GateInstance{inTest}); err == nil {
		t.Fatal("a non-hex hash must be rejected")
	}
	// A role outside the definition is refused.
	outside := []GateApproval{{ParticipantID: profileOwner, Role: "VoiceTechnicalLead"}, {ParticipantID: profileBackup, Role: "TechnicalLead"}}
	if _, err := ValidateGateTransition(spec(outside, []string{"evidence://oq11"}, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), []GateInstance{inTest}); err == nil {
		t.Fatal("a role outside the definition must be rejected")
	}
	// The full discipline passes.
	proper := []GateApproval{{ParticipantID: profileOwner, Role: "SecurityPrivacy"}, {ParticipantID: profileBackup, Role: "TechnicalLead"}}
	if _, err := ValidateGateTransition(spec(proper, []string{"evidence://oq11/perimeter"}, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), []GateInstance{inTest}); err != nil {
		t.Fatalf("the full PASS discipline must pass: %v", err)
	}
}

func TestValidateGateTransitionRaceAndCycle(t *testing.T) {
	inTest := passInstance("AG-1", nil, "")
	inTest.State = GateStateInTest
	// A transition of an already-superseded record is an honest conflict.
	raced := passInstance("AG-1", nil, "")
	raced.SupersedesInstanceID = &inTest.InstanceID
	_, err := ValidateGateTransition(GateTransitionSpec{DefinitionID: "AG-1", TargetState: GateStatePass, Supersedes: &inTest.InstanceID,
		ApprovedBy:   []GateApproval{{ParticipantID: profileOwner, Role: "TechnicalLead"}, {ParticipantID: profileBackup, Role: "Operations"}},
		EvidenceRefs: []string{"evidence://ag-1"}, EvidenceHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, []GateInstance{inTest, raced})
	var conflict GateConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("a raced transition must surface as a gate conflict, got %v", err)
	}
	// A cyclic supersedes chain is refused at the write (a fabricated
	// corrupt history: P is unsuperseded, its backward chain loops A↔B).
	head := passInstance("AG-1", nil, "")
	head.InstanceID = id.MustParse("01994f64-6c00-7000-8000-0000000000e1")
	loopA := passInstance("AG-1", nil, "")
	loopA.InstanceID = id.MustParse("01994f64-6c00-7000-8000-0000000000e2")
	loopB := passInstance("AG-1", nil, "")
	loopB.InstanceID = id.MustParse("01994f64-6c00-7000-8000-0000000000e3")
	head.SupersedesInstanceID = &loopA.InstanceID
	loopA.SupersedesInstanceID = &loopB.InstanceID
	loopB.SupersedesInstanceID = &loopA.InstanceID
	_, err = ValidateGateTransition(GateTransitionSpec{DefinitionID: "AG-1", TargetState: GateStatePass, Supersedes: &head.InstanceID,
		ApprovedBy:   []GateApproval{{ParticipantID: profileOwner, Role: "TechnicalLead"}, {ParticipantID: profileBackup, Role: "Operations"}},
		EvidenceRefs: []string{"evidence://ag-1"}, EvidenceHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, []GateInstance{head, loopA, loopB})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "цикл") {
		t.Fatalf("a cyclic chain must be rejected, got %v", err)
	}
}

func TestValidPassForRouteExactScope(t *testing.T) {
	voice := Route{Channel: "phone", Direction: DirectionInbound}
	text := Route{Channel: "web_chat", Direction: DirectionInbound}
	scoped := passInstance("OQ-11", &voice, "")
	if ValidPassForRoute("OQ-11", []GateInstance{scoped}, text) != nil {
		t.Fatal("a route-scoped PASS never applies to another route")
	}
	if ValidPassForRoute("OQ-11", []GateInstance{scoped}, voice) == nil {
		t.Fatal("the exact route scope must apply")
	}
	// A superseded PASS is not a permission.
	superseder := passInstance("OQ-11", &voice, "")
	superseder.InstanceID = profileOwner
	superseder.SupersedesInstanceID = &scoped.InstanceID
	superseder.State = GateStateFail
	if ValidPassForRoute("OQ-11", []GateInstance{scoped, superseder}, voice) != nil {
		t.Fatal("a superseded PASS is never the current permission")
	}
	// A modality scope narrows the application.
	modalityScoped := passInstance("AG-2", nil, ModalityVoice)
	if ValidPassForRoute("AG-2", []GateInstance{modalityScoped}, text) != nil {
		t.Fatal("a voice-modality PASS never covers a text route")
	}
	if ValidPassForRoute("AG-2", []GateInstance{modalityScoped}, voice) == nil {
		t.Fatal("a voice-modality PASS covers the voice route")
	}
}

func TestDeriveReadinessStatuses(t *testing.T) {
	facts := readinessFacts()
	// A complete candidate is ready for synthetic and closed_account_test.
	for _, stage := range []string{StageSynthetic, StageClosedAccountTest} {
		report := DeriveReadiness(facts, stage)
		if report.Status != ReadinessReady {
			t.Fatalf("stage %s: a complete candidate must be ready, got %s (%s)", stage, report.Status, report.StatusText)
		}
		for _, route := range report.Routes {
			if route.State != RouteAdmitted {
				t.Fatalf("stage %s: route %s must be admitted, blocked by %v", stage, route.Route, route.Reasons)
			}
		}
	}
	// The eval element stays honest: linked, not executed.
	report := DeriveReadiness(facts, StageSynthetic)
	evalState := ""
	for _, element := range report.Elements {
		if element.Key == "eval" {
			evalState = element.State
		}
	}
	if evalState != ElementLinkedNotExecuted {
		t.Fatalf("eval must stay linked_not_executed, got %s", evalState)
	}
	// Controlled is blocked: gates without PASS and eval not executed.
	report = DeriveReadiness(facts, StageControlledRealCustomer)
	if report.Status != ReadinessBlocked {
		t.Fatalf("controlled must be blocked today (no gate PASS, eval not executed), got %s", report.Status)
	}
	gateBlocks, evalBlocks := 0, 0
	for _, route := range report.Routes {
		for _, block := range route.Reasons {
			if strings.HasPrefix(block.Cause, "gate:") {
				gateBlocks++
			}
			if block.Cause == "eval" {
				evalBlocks++
			}
		}
	}
	if gateBlocks == 0 || evalBlocks == 0 {
		t.Fatalf("controlled routes must carry gate and eval blocks, got gates=%d eval=%d", gateBlocks, evalBlocks)
	}
}

func TestDeriveReadinessIncompleteWithoutProfile(t *testing.T) {
	report := DeriveReadiness(ReadinessFacts{Entitlement: AdmissionFact{Status: "ok"}}, StageSynthetic)
	if report.Status != ReadinessIncomplete {
		t.Fatalf("without an approved profile the candidate is incomplete, got %s", report.Status)
	}
}

func TestDeriveReadinessEntitlementBlocksByStage(t *testing.T) {
	facts := readinessFacts()
	facts.Entitlement = AdmissionFact{Status: "blocked_new_admissions", Reason: "Entitlement приостановлен (paused): новые admission и публикации блокируются политикой."}
	// Synthetic is the documented fixture exception: shown honestly, not blocking.
	report := DeriveReadiness(facts, StageSynthetic)
	if report.Status != ReadinessReady {
		t.Fatalf("synthetic stays ready under a blocked entitlement (fixtures), got %s", report.Status)
	}
	entitlementBlocked := false
	for _, element := range report.Elements {
		if element.Key == "entitlement" && element.State == ElementBlocked {
			entitlementBlocked = true
		}
	}
	if !entitlementBlocked {
		t.Fatal("the entitlement element must show the honest blocked state even for synthetic")
	}
	// closed_account_test and up: the publication is blocked.
	report = DeriveReadiness(facts, StageClosedAccountTest)
	if report.Status != ReadinessBlocked {
		t.Fatalf("closed_account_test must be blocked by the entitlement admission state, got %s", report.Status)
	}
}

func TestDeriveReadinessCapabilityDriftBlocksRoutes(t *testing.T) {
	facts := readinessFacts()
	facts.CurrentConnections = []ConnectionFacts{{
		ConnectionID: facts.CurrentConnections[0].ConnectionID, Status: "connected",
		Capabilities: []CapabilityFact{{Capability: "text_chat", State: "stale", Reason: "credential rotation reset the evidence", Gap: false}},
	}}
	report := DeriveReadiness(facts, StageSynthetic)
	if report.Status != ReadinessBlocked {
		t.Fatalf("stale current capabilities must block the routes (fail-closed), got %s", report.Status)
	}
	found := false
	for _, route := range report.Routes {
		for _, block := range route.Reasons {
			if block.Cause == "capability" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("the routes must carry the capability block with the reason")
	}
}

func TestDeriveReadinessVoiceNotApplicableWithoutVoiceRoutes(t *testing.T) {
	facts := readinessFacts()
	content := validProfileContent()
	content.WorkScope.Routes = []Route{{Channel: "web_chat", Direction: DirectionInbound}}
	content.Coverage = content.Coverage[1:]
	content.Privacy = content.Privacy[5:]
	digest, _ := ProfileDigest(content)
	facts.Approved.Content = content
	facts.Approved.Digest = digest
	facts.ManifestRead.Content.Config.Refs.Voice = VoiceRef{Status: RefNotApplicable, Note: "phone не входит в Brief"}
	report := DeriveReadiness(facts, StageSynthetic)
	voiceState := ""
	for _, element := range report.Elements {
		if element.Key == "voice" {
			voiceState = element.State
		}
	}
	if voiceState != ElementNotApplicable {
		t.Fatalf("voice must be not_applicable without voice routes, got %s", voiceState)
	}
}

func TestDeriveReadinessGatePassUnblocksControlledRoutes(t *testing.T) {
	facts := readinessFacts()
	// All the gates of the controlled stage for the text route PASS — the
	// routes stay blocked by eval (the honest eternal block of the slice).
	instances := []GateInstance{
		passInstance("OQ-11", &Route{Channel: "web_chat", Direction: DirectionInbound}, ""),
		passInstance("OQ-11", &Route{Channel: "phone", Direction: DirectionInbound}, ""),
		passInstance("AG-1", nil, ""),
		passInstance("AG-2", nil, ""),
		passInstance("AG-4", nil, ""),
		passInstance("OQ-12", nil, ModalityVoice),
		passInstance("OQ-13B", nil, ModalityVoice),
	}
	instanceIDs := []string{
		"01994f64-6c00-7000-8000-0000000000d1",
		"01994f64-6c00-7000-8000-0000000000d2",
		"01994f64-6c00-7000-8000-0000000000d3",
		"01994f64-6c00-7000-8000-0000000000d4",
		"01994f64-6c00-7000-8000-0000000000d5",
		"01994f64-6c00-7000-8000-0000000000d6",
		"01994f64-6c00-7000-8000-0000000000d7",
	}
	for index := range instances {
		instances[index].InstanceID = id.MustParse(instanceIDs[index])
	}
	facts.Instances = instances
	report := DeriveReadiness(facts, StageControlledRealCustomer)
	if report.Status != ReadinessBlocked {
		t.Fatalf("controlled stays blocked by eval even with every gate PASS, got %s", report.Status)
	}
	gateBlocks := 0
	for _, route := range report.Routes {
		for _, block := range route.Reasons {
			if strings.HasPrefix(block.Cause, "gate:") {
				gateBlocks++
			}
		}
	}
	if gateBlocks != 0 {
		t.Fatalf("gate blocks must be resolved by the PASS instances, got %d", gateBlocks)
	}
}

func TestValidateStageProgressionLadder(t *testing.T) {
	facts := readinessFacts()
	candidate := CandidateKey{Manifest: facts.Approved.Manifest, ProfileVersion: facts.Approved.Version, ProfileDigest: facts.Approved.Digest}
	// Nothing acts yet: synthetic is allowed, every closed stage is refused.
	if err := ValidateStageProgression(StageSynthetic, candidate, nil); err != nil {
		t.Fatalf("synthetic without a previous decision must pass: %v", err)
	}
	if err := ValidateStageProgression(StageClosedAccountTest, candidate, nil); err == nil {
		t.Fatal("a stage jump without a previous decision must be refused")
	}
	synthetic := ReleaseAdmission{TenantID: readinessTenant, EmployeeID: readinessEmployee, DecisionNo: 1, Stage: StageSynthetic,
		Manifest: facts.Approved.Manifest, ProfileVersion: 1, ProfileDigest: facts.Approved.Digest}
	// The next step of the ladder is allowed.
	if err := ValidateStageProgression(StageClosedAccountTest, candidate, []ReleaseAdmission{synthetic}); err != nil {
		t.Fatalf("closed_account_test after synthetic must pass: %v", err)
	}
	// The acting stage of the same candidate is an honest conflict.
	err := ValidateStageProgression(StageSynthetic, candidate, []ReleaseAdmission{synthetic})
	var conflict AdmissionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("a repeated publication of the acting stage must conflict, got %v", err)
	}
	// A jump over a stage is refused naming the required stage.
	if err := ValidateStageProgression(StageControlledRealCustomer, candidate, []ReleaseAdmission{synthetic}); err == nil || !strings.Contains(err.Error(), StageClosedAccountTest) {
		t.Fatalf("a jump must name the required previous stage, got %v", err)
	}
	// A downgrade would be a rollback (deferred).
	closed := synthetic
	closed.DecisionNo = 2
	closed.Stage = StageClosedAccountTest
	if err := ValidateStageProgression(StageSynthetic, candidate, []ReleaseAdmission{closed}); err == nil {
		t.Fatal("a downgrade must be refused (rollback is deferred)")
	}
	// A different candidate restarts from synthetic.
	other := closed
	other.Manifest = ManifestLink{ManifestID: profileOwner, ConfigVersion: 2, Digest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	other.ProfileVersion = 2
	// The interleaved fixtures: the other candidate's own two decisions
	// (synthetic №2, closed №3) recorded after this candidate's synthetic №1.
	otherCandidate := CandidateKey{Manifest: other.Manifest, ProfileVersion: other.ProfileVersion, ProfileDigest: other.ProfileDigest}
	otherSynthetic := other
	otherSynthetic.DecisionNo = 2
	otherSynthetic.Stage = StageSynthetic
	otherClosed := other
	otherClosed.DecisionNo = 3
	otherClosed.Stage = StageClosedAccountTest
	if err := ValidateStageProgression(StageSynthetic, candidate, []ReleaseAdmission{other}); err != nil {
		t.Fatalf("a new candidate starts from synthetic: %v", err)
	}
	if err := ValidateStageProgression(StageClosedAccountTest, candidate, []ReleaseAdmission{other}); err == nil {
		t.Fatal("a new candidate cannot skip synthetic")
	}
	// Interleaved candidates (R2/N2): the globally latest decisions belong to
	// another candidate, but this candidate's own synthetic decision stays
	// acting for the ladder — the closed stage of THIS candidate is allowed
	// even though a newer decision of ANOTHER candidate is the active
	// pointer, and a newer decision of another candidate at a higher stage
	// does not look like this candidate's rollback.
	interleaved := []ReleaseAdmission{otherClosed, otherSynthetic, synthetic}
	if err := ValidateStageProgression(StageClosedAccountTest, candidate, interleaved); err != nil {
		t.Fatalf("an interleaved candidate must not break this candidate's ladder: %v", err)
	}
	// The repeat of this candidate's stage conflicts with ITS record.
	err = ValidateStageProgression(StageSynthetic, candidate, interleaved)
	var interleavedConflict AdmissionConflictError
	if !errors.As(err, &interleavedConflict) || interleavedConflict.Current == nil || interleavedConflict.Current.DecisionNo != synthetic.DecisionNo {
		t.Fatalf("the repeat of this candidate's stage must conflict with its own record, got %v", err)
	}
	// The other candidate's own ladder is unaffected by this candidate.
	if err := ValidateStageProgression(StageControlledRealCustomer, otherCandidate, interleaved); err != nil {
		t.Fatalf("the other candidate may step to controlled after its closed decision: %v", err)
	}
}

func TestAdmissionDigestIsCanonical(t *testing.T) {
	facts := readinessFacts()
	block := RouteBlock{Cause: "eval", Reason: "linked, not executed", UnlockOwner: "Product Owner", RequiredEvidence: "runs", NextStep: "wait"}
	first := ReleaseAdmission{TenantID: readinessTenant, EmployeeID: readinessEmployee, DecisionNo: 1, Stage: StageSynthetic,
		Manifest: facts.Approved.Manifest, ProfileVersion: 1, ProfileDigest: facts.Approved.Digest,
		Routes: []RouteAdmission{
			{Route: Route{Channel: "phone", Direction: DirectionInbound}, Modality: ModalityVoice, AutonomyClasses: []string{"assisted"}, State: RouteAdmitted},
			{Route: Route{Channel: "web_chat", Direction: DirectionInbound}, Modality: ModalityText, AutonomyClasses: []string{"assisted"}, State: RouteBlocked, Reasons: []RouteBlock{block}},
		},
		ActorID: profileOwner, ApproverRoles: []string{"owner", "configurator"}, StepUpVerified: true,
		DecidedAt: time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)}
	digest, err := AdmissionDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	// The route order and the approver role order do not change the digest.
	reordered := first
	reordered.Routes = []RouteAdmission{first.Routes[1], first.Routes[0]}
	reordered.ApproverRoles = []string{"configurator", "owner"}
	other, err := AdmissionDigest(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if digest != other {
		t.Fatalf("the digest must be canonical (order-independent), got %s vs %s", digest, other)
	}
	// A different decision number changes the digest.
	bumped := first
	bumped.DecisionNo = 2
	third, _ := AdmissionDigest(bumped)
	if digest == third {
		t.Fatal("the decision number must be part of the digest")
	}
	if AdmittedRouteCount(first) != 1 {
		t.Fatal("the admitted route count must be honest")
	}
}

func TestDeriveActiveReleasePicksHighestDecision(t *testing.T) {
	facts := readinessFacts()
	first := ReleaseAdmission{DecisionNo: 1, Stage: StageSynthetic, Manifest: facts.Approved.Manifest}
	second := ReleaseAdmission{DecisionNo: 2, Stage: StageClosedAccountTest, Manifest: facts.Approved.Manifest}
	active := DeriveActiveRelease([]ReleaseAdmission{first, second})
	if active == nil || active.DecisionNo != 2 {
		t.Fatalf("the active pointer is the highest decision number, got %+v", active)
	}
	if DeriveActiveRelease(nil) != nil {
		t.Fatal("no decisions means no active pointer")
	}
}

// TestDeriveReadinessMixedConnectionsAdmitRoutes: one connection with an
// acting verified capability is enough for the element and the routes; the
// failed and revoked siblings are honest notes, never hidden by the pass —
// their full discipline is the AG-4 gate of the higher stages.
func TestDeriveReadinessMixedConnectionsAdmitRoutes(t *testing.T) {
	facts := readinessFacts()
	base := facts.CurrentConnections[0]
	facts.CurrentConnections = []ConnectionFacts{
		{ConnectionID: id.MustParse("01994f64-6c00-7000-8000-0000000000e1"), Status: "failed",
			Capabilities: []CapabilityFact{{Capability: "query.trunk-status", State: "unverified", Reason: "check failed", Gap: true}}},
		base,
		{ConnectionID: id.MustParse("01994f64-6c00-7000-8000-0000000000e3"), Status: "revoked",
			Capabilities: []CapabilityFact{{Capability: "query.trunk-status", State: "unverified", Reason: "revoked", Gap: true}}},
	}
	report := DeriveReadiness(facts, StageSynthetic)
	states := map[string]string{}
	for _, element := range report.Elements {
		states[element.Key] = element.State
	}
	if states["capabilities"] != ElementPass {
		t.Fatalf("the acting capability of one connection must satisfy the element, got %s", states["capabilities"])
	}
	for _, element := range report.Elements {
		if element.Key == "capabilities" && !strings.Contains(element.Reason, "Непроверенные остатки") {
			t.Fatalf("the pass must keep the honest notes about the unproven siblings: %s", element.Reason)
		}
	}
	for _, route := range report.Routes {
		if route.State != RouteAdmitted {
			t.Fatalf("route %s must be admitted (one acting connection suffices), blocks=%v", route.Route, route.Reasons)
		}
	}
	if report.Status != ReadinessReady {
		t.Fatalf("status = %s, want ready", report.Status)
	}
	// Every capability gone stale or unproven keeps the routes blocked.
	facts.CurrentConnections = []ConnectionFacts{facts.CurrentConnections[0], facts.CurrentConnections[1]}
	facts.CurrentConnections[0].Capabilities = []CapabilityFact{{Capability: "text_chat", State: "stale", Reason: "rotation reset the evidence", Gap: false}}
	facts.CurrentConnections[1].Capabilities = []CapabilityFact{{Capability: "text_chat", State: "unverified", Gap: true}}
	report = DeriveReadiness(facts, StageSynthetic)
	if report.Status != ReadinessBlocked {
		t.Fatalf("no acting capability anywhere must block the routes, got %s", report.Status)
	}
}
