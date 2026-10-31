package domain

import (
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

var profileOwner = id.MustParse("01994f64-6c00-7000-8000-0000000000aa")
var profileBackup = id.MustParse("01994f64-6c00-7000-8000-0000000000ab")

func profileFacts() ProfileFacts {
	return ProfileFacts{
		MemberIDs:      map[id.UUID]bool{profileOwner: true, profileBackup: true},
		PackJourneys:   []string{"поиск детали", "уточнение аналога"},
		PackReferenced: true,
	}
}

// validProfileContent is a complete, valid profile content: one voice route,
// full coverage, complete privacy assignments (one in use, others explicitly
// not used), ops level, metric, gate and a complete matrix.
func validProfileContent() ProfileContent {
	approvedRevision := int64(1)
	_ = approvedRevision
	return ProfileContent{
		WorkScope: WorkScope{
			Routes:    []Route{{Channel: "phone", Direction: DirectionInbound}, {Channel: "web_chat", Direction: DirectionInbound}},
			Forbidden: []ForbiddenRoute{{Route: Route{Channel: "telegram", Direction: DirectionOutbound}, Reason: "Канал не подключён и не оплачивается."}},
			Languages: []string{"ru"},
			Window:    WorkWindow{Days: []string{"mon", "tue"}, Start: "09:00", End: "18:00", TimeZone: "Europe/Moscow"},
			Limits: WorkLimits{MaxConcurrentConversations: 4, MaxConcurrentCalls: 2, PerRoute: []RouteLimit{
				{Route: Route{Channel: "phone", Direction: DirectionInbound}, MaxConcurrentConversations: 2, MaxConcurrentCalls: 1},
				{Route: Route{Channel: "web_chat", Direction: DirectionInbound}, MaxConcurrentConversations: 2},
			}},
			StopConditions: []string{"Достижение hard limit бюджета.", "Перевод Entitlement в paused."},
		},
		Coverage: []HumanCoverage{
			{Route: Route{Channel: "phone", Direction: DirectionInbound}, ResponseOwnerID: profileOwner, BackupOwnerID: profileBackup,
				Days: []string{"mon", "tue"}, Start: "09:00", End: "18:00", AckDeadlineMinutes: 15, Fallback: "voicemail", FallbackEvidence: "capability://jivo/voicemail/verified"},
			{Route: Route{Channel: "web_chat", Direction: DirectionInbound}, ResponseOwnerID: profileOwner, BackupOwnerID: profileBackup,
				Days: []string{"mon", "tue"}, Start: "09:00", End: "18:00", AckDeadlineMinutes: 10, Fallback: "human_callback"},
		},
		Privacy: []PrivacyAssignment{
			{Route: Route{Channel: "phone", Direction: DirectionInbound}, Purpose: "recording", InUse: true,
				LegalBasis: "Договор с клиентом", ConsentRef: "consent://tenant/recording", Disclosure: "Озвучивается в начале звонка",
				Retention: "90 дней", Deletion: "Workflow Story 1-10", Withdrawal: "Остановка по запросу клиента", OQ11State: OQ11Pass, OQ11Evidence: "privacy://oq11/recording-pass"},
			{Route: Route{Channel: "phone", Direction: DirectionInbound}, Purpose: "stt_model_processing", InUse: false, OQ11State: OQ11Open},
			{Route: Route{Channel: "phone", Direction: DirectionInbound}, Purpose: "media_processing", InUse: false, OQ11State: OQ11Open},
			{Route: Route{Channel: "phone", Direction: DirectionInbound}, Purpose: "transfer", InUse: false, OQ11State: OQ11Open},
			{Route: Route{Channel: "phone", Direction: DirectionInbound}, Purpose: "marketing_contact", InUse: false, OQ11State: OQ11Open},
			{Route: Route{Channel: "web_chat", Direction: DirectionInbound}, Purpose: "recording", InUse: false, OQ11State: OQ11Open},
			{Route: Route{Channel: "web_chat", Direction: DirectionInbound}, Purpose: "stt_model_processing", InUse: false, OQ11State: OQ11Open},
			{Route: Route{Channel: "web_chat", Direction: DirectionInbound}, Purpose: "media_processing", InUse: false, OQ11State: OQ11Open},
			{Route: Route{Channel: "web_chat", Direction: DirectionInbound}, Purpose: "transfer", InUse: false, OQ11State: OQ11Open},
			{Route: Route{Channel: "web_chat", Direction: DirectionInbound}, Purpose: "marketing_contact", InUse: false, OQ11State: OQ11Open},
		},
		OpsLevel: OperationalLevel{Level: "best_effort"},
		Metrics: []MetricDefinition{{
			ID: "csat", Name: "CSAT после закрытия", BusinessMeaning: "Удовлетворённость клиента обслуживанием",
			Numerator: "Положительные оценки", Denominator: "Все оценки с ответом", SourceOfTruth: "Опрос после закрытия case",
			TerminalTiming: "Через 24 часа после закрытия", Slices: []string{"journey", "modality", "conversation_mode", "release_version", "autonomy_class"},
			OwnerID: profileOwner, DisputePolicy: "Разногласия решает владелец метрики", RouteFailureVisibility: "Доля route-blocking failure показывается отдельной строкой, не средним",
		}},
		Gates: []GateDeclaration{{
			GateID: "knowledge_eval", EvidenceOwnerID: profileOwner, DecisionDeadline: "До старта пилота, но не позднее 2026-11-01",
			FailClosedFallback: "block_route", AllowRoleCoincidence: false,
		}},
		Matrix: []CoverageCell{
			{Journey: "поиск детали", Route: Route{Channel: "phone", Direction: DirectionInbound}, Modality: ModalityVoice, ConversationMode: "sales", AutonomyClass: "assisted",
				EvalRef: "eval://auto-parts/search/voice-v1", Gates: []string{"knowledge_eval"}, HumanFallback: "human_callback по регламенту"},
			{Journey: "поиск детали", Route: Route{Channel: "web_chat", Direction: DirectionInbound}, Modality: ModalityText, ConversationMode: "sales", AutonomyClass: "assisted",
				EvalRef: "eval://auto-parts/search/text-v1", HumanFallback: "human_callback по регламенту"},
			{Journey: "уточнение аналога", Route: Route{Channel: "phone", Direction: DirectionInbound}, Modality: ModalityVoice, ConversationMode: "sales", AutonomyClass: "assisted",
				NotAdmitted: true, NotAdmittedReason: "Голосовой сценарий аналогов требует доработки pack: явное «не допущено»."},
			{Journey: "уточнение аналога", Route: Route{Channel: "web_chat", Direction: DirectionInbound}, Modality: ModalityText, ConversationMode: "sales", AutonomyClass: "assisted",
				EvalRef: "eval://auto-parts/analog/text-v1", HumanFallback: "human_callback по регламенту"},
		},
		Modes:           []string{"sales"},
		AutonomyClasses: []string{"assisted"},
	}
}

func TestValidateProfileContentAcceptsCompleteContent(t *testing.T) {
	if problems := ValidateProfileContent(validProfileContent(), profileFacts()); len(problems) != 0 {
		t.Fatalf("complete content must validate cleanly, got: %v", problems)
	}
}

func TestValidateProfileContentLeadsToConcreteFields(t *testing.T) {
	content := validProfileContent()
	content.WorkScope.Limits.PerRoute = content.WorkScope.Limits.PerRoute[:1] // web_chat limit missing
	content.Coverage = content.Coverage[:1]                                   // web_chat coverage missing
	content.OpsLevel.Level = ""                                               // ops level undecided
	content.Metrics = nil                                                     // no metrics
	content.Gates = nil                                                       // no gates
	content.Modes = nil                                                       // no modes
	problems := ValidateProfileContent(content, profileFacts())
	field := func(section, prefix string) bool {
		for _, problem := range problems {
			if problem.Section == section && (prefix == "" || len(problem.Field) >= len(prefix)) {
				return true
			}
		}
		return false
	}
	if !field("work_scope", "routes") {
		t.Fatalf("routes problem missing: %v", problems)
	}
	if !field("work_scope", "limits") {
		t.Fatalf("per-route limits problem missing: %v", problems)
	}
	if !field("human_coverage", "web_chat:inbound") {
		t.Fatalf("missing coverage must name the concrete route: %v", problems)
	}
	if !field("operational_level", "level") {
		t.Fatalf("ops level problem missing: %v", problems)
	}
	if !field("measurement_dictionary", "metrics") {
		t.Fatalf("metrics problem missing: %v", problems)
	}
	if !field("gate_declarations", "gates") {
		t.Fatalf("gates problem missing: %v", problems)
	}
	// Missing modes also collapse the matrix completeness check.
	if !field("coverage_matrix", "") {
		t.Fatalf("matrix problem missing: %v", problems)
	}
	// Privacy assignments of removed routes are simply absent; declared routes
	// keep the mandatory per-purpose rows.
	for _, purpose := range processingPurposeDictionary {
		found := false
		for _, row := range content.Privacy {
			if row.Purpose == purpose {
				found = true
			}
		}
		if !found {
			t.Fatalf("privacy dictionary purpose %s disappeared", purpose)
		}
	}
}

func TestValidateProfileContentRejectsDictionaryViolations(t *testing.T) {
	content := validProfileContent()
	content.Modes = []string{"small_talk"} // not in the dictionary
	content.Gates[0].FailClosedFallback = "warn"
	content.Coverage[0].Fallback = "sms"
	content.OpsLevel.Level = "premium"
	problems := ValidateProfileContent(content, profileFacts())
	joined := ""
	for _, problem := range problems {
		joined += problem.Error() + "\n"
	}
	for _, expected := range []string{"small_talk", "fail_closed_fallback", "fallback", "level"} {
		if !contains(joined, expected) {
			t.Fatalf("expected %q among honest rejections: %s", expected, joined)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestValidateProfileContentRequiresFallbackEvidenceForNonHandoffFallbacks(t *testing.T) {
	content := validProfileContent()
	content.Coverage[0].FallbackEvidence = "" // voicemail without evidence
	problems := ValidateProfileContent(content, profileFacts())
	if len(problems) != 1 || problems[0].Section != "human_coverage" || problems[0].Field != "phone:inbound" {
		t.Fatalf("expected exactly the phone:inbound fallback evidence problem, got: %v", problems)
	}
}

func TestValidateProfileContentRequiresAllPurposesPerRoute(t *testing.T) {
	content := validProfileContent()
	content.Privacy = content.Privacy[:9] // drop one web_chat assignment
	problems := ValidateProfileContent(content, profileFacts())
	found := false
	for _, problem := range problems {
		if problem.Section == "privacy_assignments" && contains(problem.Field, "marketing_contact") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing route×purpose assignment must be named: %v", problems)
	}
}

func TestValidateProfileContentRejectsSharedConsentShortcut(t *testing.T) {
	content := validProfileContent()
	// The phone recording assignment is in use; wiping its separate consent
	// reference must be rejected (no single shared flag can cover it).
	content.Privacy[0].ConsentRef = ""
	problems := ValidateProfileContent(content, profileFacts())
	if len(problems) != 1 || !contains(problems[0].Reason, "consent_ref") {
		t.Fatalf("expected the consent_ref rejection, got: %v", problems)
	}
}

func TestValidateProfileContentCommercialLevelRequiresGoalsAndOwners(t *testing.T) {
	content := validProfileContent()
	content.OpsLevel = OperationalLevel{Level: "commercial"}
	problems := ValidateProfileContent(content, profileFacts())
	if len(problems) < 3 {
		t.Fatalf("commercial level without goals, owners and impact must list each: %v", problems)
	}
	content.OpsLevel = OperationalLevel{Level: "commercial", RTOGoal: "4 часа", RPOGoal: "1 час",
		ResponsibleIDs: []id.UUID{profileOwner}, ServiceImpact: "SLA пилота с компенсацией"}
	if problems := ValidateProfileContent(content, profileFacts()); len(problems) != 0 {
		t.Fatalf("complete commercial level must validate, got: %v", problems)
	}
}

func TestValidateProfileContentMetricSlicesAndRouteFailureVisibility(t *testing.T) {
	content := validProfileContent()
	content.Metrics[0].Slices = []string{"journey", "conversation_mode"} // missing slices
	content.Metrics[0].RouteFailureVisibility = ""                       // hidden route failure
	problems := ValidateProfileContent(content, profileFacts())
	slices, visibility := false, false
	for _, problem := range problems {
		if problem.Section == "measurement_dictionary" && problem.Field == "csat.slices" {
			slices = true
		}
		if problem.Section == "measurement_dictionary" && problem.Field == "csat.route_failure_visibility" {
			visibility = true
		}
	}
	if !slices || !visibility {
		t.Fatalf("expected slices and route failure visibility rejections, got: %v", problems)
	}
}

func TestValidateProfileContentMatrixReferencesDeclaredGatesOnly(t *testing.T) {
	content := validProfileContent()
	content.Matrix[1].Gates = []string{"undeclared_gate"}
	problems := ValidateProfileContent(content, profileFacts())
	found := false
	for _, problem := range problems {
		if problem.Section == "coverage_matrix" && contains(problem.Reason, "undeclared_gate") {
			found = true
		}
	}
	if !found {
		t.Fatalf("undeclared gate reference must be rejected: %v", problems)
	}
}

func TestValidateProfileContentMatrixJourneyMustBelongToPack(t *testing.T) {
	content := validProfileContent()
	content.Matrix[1].Journey = "неизвестный journey"
	problems := ValidateProfileContent(content, profileFacts())
	found := false
	for _, problem := range problems {
		if problem.Section == "coverage_matrix" && contains(problem.Reason, "неизвестный journey") {
			found = true
		}
	}
	if !found {
		t.Fatalf("journey outside the pack reference must be rejected: %v", problems)
	}
}

func TestValidateProfileContentMatrixCompletenessCountsGaps(t *testing.T) {
	content := validProfileContent()
	content.Matrix = content.Matrix[:3] // one cartesian cell removed → gap
	problems := ValidateProfileContent(content, profileFacts())
	found := false
	for _, problem := range problems {
		if problem.Section == "coverage_matrix" && contains(problem.Reason, "gap") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a removed matrix cell must surface as an explicit gap: %v", problems)
	}
}

func TestValidateProfileContentContradictoryForbiddenRoute(t *testing.T) {
	content := validProfileContent()
	content.WorkScope.Forbidden = append(content.WorkScope.Forbidden,
		ForbiddenRoute{Route: Route{Channel: "phone", Direction: DirectionInbound}, Reason: "Одновременно разрешён и запрещён."})
	problems := ValidateProfileContent(content, profileFacts())
	found := false
	for _, problem := range problems {
		if problem.Section == "work_scope" && contains(problem.Reason, "одновременно разрешён и запрещён") {
			found = true
		}
	}
	if !found {
		t.Fatalf("an allowed-and-forbidden route is a contradiction that must be named: %v", problems)
	}
}

func TestProfileDigestIsCanonicalAndSensitive(t *testing.T) {
	content := validProfileContent()
	first, err := ProfileDigest(content)
	if err != nil {
		t.Fatal(err)
	}
	// A reordered copy of the same content must produce the same digest.
	reordered := validProfileContent()
	reordered.WorkScope.Routes = []Route{{Channel: "web_chat", Direction: DirectionInbound}, {Channel: "phone", Direction: DirectionInbound}}
	reordered.Privacy = reversedPrivacy(reordered.Privacy)
	reordered.Matrix = []CoverageCell{reordered.Matrix[3], reordered.Matrix[2], reordered.Matrix[1], reordered.Matrix[0]}
	second, err := ProfileDigest(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("digest must be canonical: %s != %s", first, second)
	}
	// Any material change must change the digest.
	changed := validProfileContent()
	changed.WorkScope.Limits.MaxConcurrentConversations = 5
	third, err := ProfileDigest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatal("a material change must change the digest")
	}
}

func reversedPrivacy(rows []PrivacyAssignment) []PrivacyAssignment {
	out := make([]PrivacyAssignment, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, rows[i])
	}
	return out
}

func TestDeriveRouteStatesBlocksOnlyTheAffectedRoute(t *testing.T) {
	content := validProfileContent()
	// web_chat loses its coverage; phone keeps everything.
	content.Coverage = content.Coverage[:1]
	states := DeriveRouteStates(content)
	byRoute := map[string]RouteState{}
	for _, state := range states {
		byRoute[state.Route.String()] = state
	}
	if states := byRoute["web_chat:inbound"]; states.State != RouteStateBlocked || states.UnlockOwner == "" {
		t.Fatalf("web_chat must be blocked with an unlock owner: %+v", states)
	}
	if state := byRoute["phone:inbound"]; state.State != RouteStateClear {
		t.Fatalf("phone must keep living (route-level fail-closed): %+v", state)
	}
}

func TestDeriveRouteStatesBlocksUnprovenFallbackAndOpenPrivacy(t *testing.T) {
	content := validProfileContent()
	content.Coverage[0].FallbackEvidence = "" // voicemail without evidence
	content.Privacy[5].InUse = true           // web_chat recording in use
	content.Privacy[5].OQ11State = OQ11Open   // OQ-11 open
	content.Privacy[5].LegalBasis = "Согласие"
	content.Privacy[5].ConsentRef = "consent://tenant/chat"
	content.Privacy[5].Disclosure = "Баннер в чате"
	content.Privacy[5].Retention = "30 дней"
	content.Privacy[5].Deletion = "Workflow 1-10"
	content.Privacy[5].Withdrawal = "Кнопка отказа"
	states := DeriveRouteStates(content)
	byRoute := map[string]RouteState{}
	for _, state := range states {
		byRoute[state.Route.String()] = state
	}
	phone := byRoute["phone:inbound"]
	if phone.State != RouteStateBlocked || !contains(phone.Reason, "handoff") {
		t.Fatalf("unproven voicemail must block phone with the honest reason: %+v", phone)
	}
	chat := byRoute["web_chat:inbound"]
	if chat.State != RouteStateBlocked || !contains(chat.Reason, "OQ-11") {
		t.Fatalf("open privacy assignment must block web_chat: %+v", chat)
	}
}

func TestDeriveMatrixCoverageShowsGapsAndNotAdmitted(t *testing.T) {
	content := validProfileContent()
	content.Matrix = content.Matrix[:3] // the last cell becomes a gap
	cells := DeriveMatrixCoverage(content, profileFacts().PackJourneys)
	// journeys(2) × routes(2) × modes(1) × classes(1) = 4 cells
	if len(cells) != 4 {
		t.Fatalf("cartesian product size = %d", len(cells))
	}
	states := map[string]string{}
	for _, cell := range cells {
		states[cell.Journey+" · "+cell.Route.String()] = cell.State
	}
	if states["поиск детали · phone:inbound"] != MatrixCellCovered {
		t.Fatalf("declared cell must be covered: %s", states["поиск детали · phone:inbound"])
	}
	if states["уточнение аналога · phone:inbound"] != MatrixCellNotAdmitted {
		t.Fatalf("explicit refusal must stay visible: %s", states["уточнение аналога · phone:inbound"])
	}
	if states["уточнение аналога · web_chat:inbound"] != MatrixCellGap {
		t.Fatalf("missing cell must be an explicit gap: %s", states["уточнение аналога · web_chat:inbound"])
	}
}

func TestDeriveThresholdStatesPendingBaselineUntilDecided(t *testing.T) {
	content := validProfileContent()
	states := DeriveThresholdStates(content, nil)
	if len(states) != 1 || states[0].State != ThresholdStatePendingBaseline || states[0].Value != "" {
		t.Fatalf("an undecided gate must stay pending baseline without a value: %+v", states)
	}
	if !contains(states[0].Note, "pending baseline") {
		t.Fatalf("the honest note must name pending baseline: %s", states[0].Note)
	}
	decision := GateThresholdVersion{
		TenantID: id.MustParse("01994f64-6c00-7000-8000-0000000000cc"), EmployeeID: id.MustParse("01994f64-6c00-7000-8000-0000000000cd"),
		GateID: "knowledge_eval", ThresholdVersion: 1, Value: "≥ 0.9 доля корректных ответов",
		ActorID: profileOwner, Decision: "Утверждено по результатам ручной выборки",
		EvidenceRefs: []string{"eval://baseline/sample-2026-10"}, DecidedAt: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC),
	}
	states = DeriveThresholdStates(content, []GateThresholdVersion{decision})
	if states[0].State != ThresholdStateDecided || states[0].Value != decision.Value {
		t.Fatalf("the decision must surface with its value: %+v", states[0])
	}
	if !contains(states[0].Note, "controlled baseline") {
		t.Fatalf("the decision note must stay honest about the missing machine baseline: %s", states[0].Note)
	}
}

func TestDeriveThresholdStatesPrefersLatestVersion(t *testing.T) {
	content := validProfileContent()
	first := GateThresholdVersion{GateID: "knowledge_eval", ThresholdVersion: 1, Value: "0.8"}
	second := GateThresholdVersion{GateID: "knowledge_eval", ThresholdVersion: 2, Value: "0.9"}
	states := DeriveThresholdStates(content, []GateThresholdVersion{first, second})
	if states[0].Value != "0.9" {
		t.Fatalf("the latest threshold version wins: %+v", states[0])
	}
}

func TestDeriveProfileApplicabilityStates(t *testing.T) {
	manifest := ManifestLink{ManifestID: id.MustParse("01994f64-6c00-7000-8000-0000000000ee"), ConfigVersion: 1, Digest: "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"}
	if state := DeriveProfileApplicability(nil, nil, &manifest); state.State != ProfileApplicabilityNone {
		t.Fatalf("no approved version: %+v", state)
	}
	approved := &ProfileVersion{Manifest: manifest}
	if state := DeriveProfileApplicability(approved, nil, &manifest); state.State != ProfileApplicabilityCurrent {
		t.Fatalf("same manifest: %+v", state)
	}
	newer := ManifestLink{ManifestID: id.MustParse("01994f64-6c00-7000-8000-0000000000ef"), ConfigVersion: 2, Digest: "b1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"}
	if state := DeriveProfileApplicability(approved, nil, &newer); state.State != ProfileApplicabilityStale {
		t.Fatalf("new manifest must make the profile stale: %+v", state)
	}
	approvedRevision := int64(3)
	draft := &ProfileDraft{Manifest: manifest, Revision: 4, ApprovedRevision: &approvedRevision}
	if state := DeriveProfileApplicability(approved, draft, &manifest); state.State != ProfileApplicabilityUnapproved {
		t.Fatalf("post-approval edits must surface as unapproved changes: %+v", state)
	}
	draft.Revision = 3
	if state := DeriveProfileApplicability(approved, draft, &manifest); state.State != ProfileApplicabilityCurrent {
		t.Fatalf("no post-approval edits: %+v", state)
	}
}

func TestValidateThresholdDecisionAndDescription(t *testing.T) {
	if err := ValidateThresholdDecision("", "решение", []string{"ref"}); err == nil {
		t.Fatal("an empty value must be rejected")
	}
	if err := ValidateThresholdDecision("0.9", "", []string{"ref"}); err == nil {
		t.Fatal("an empty decision must be rejected")
	}
	if err := ValidateThresholdDecision("0.9", "решение", nil); err == nil {
		t.Fatal("a decision without evidence references must be rejected")
	}
	if err := ValidateThresholdDecision("0.9", "решение", []string{"eval://ref"}); err != nil {
		t.Fatalf("a complete decision must pass: %v", err)
	}
	if err := ValidateProfileDescription(""); err == nil {
		t.Fatal("an empty approval description must be rejected")
	}
	if err := ValidateProfileDescription("Утверждение профиля v1 по договору №77"); err != nil {
		t.Fatalf("a valid description must pass: %v", err)
	}
}

func TestValidateProfileContentRequiresPackReference(t *testing.T) {
        facts := profileFacts()
        facts.PackReferenced = false
        // An empty product cannot show gaps: without the pack reference the
        // completeness is vacuous and the approval must block honestly.
        problems := ValidateProfileContent(validProfileContent(), facts)
        found := false
        for _, problem := range problems {
                if problem.Section == "coverage_matrix" && problem.Field == "pack_reference" && problem.Kind == ProfileProblemMissing {
                        found = true
                }
        }
        if !found {
                t.Fatalf("a packless manifest must block the approval honestly, got: %v", problems)
        }
}

func TestValidateProfileContentRejectsModalityMismatch(t *testing.T) {
        content := validProfileContent()
        content.Matrix[0].Modality = ModalityText // phone route must carry voice
        problems := ValidateProfileContent(content, profileFacts())
        found := false
        for _, problem := range problems {
                if problem.Kind == ProfileProblemInvalid && contains(problem.Reason, "Modality") {
                        found = true
                }
        }
        if !found {
                t.Fatalf("a modality that disagrees with the route channel must reject the save, got: %v", problems)
        }
}
