package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

var releaseClock = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

func phoneBrief() Brief {
	return Brief{Purpose: "Помогать клиентам магазина", Name: "Аня", Languages: []string{"ru"}, TimeZone: "Europe/Moscow",
		WorkingHours:    WorkingHours{Days: []string{"mon", "tue"}, Start: "09:00", End: "18:00"},
		DesiredChannels: []string{"phone"}, PrimaryTasks: []string{"Консультации"}}
}

func textBrief() Brief {
	brief := phoneBrief()
	brief.DesiredChannels = []string{"web_chat"}
	return brief
}

func fullFacts() ConfigFacts {
	return ConfigFacts{
		Brief:        phoneBrief(),
		DraftVersion: 5,
		Pack:         &PackPin{ID: "auto-parts", Version: "2.0.0", Variants: map[string]any{"warehouse": "central"}},
		Behavior:     []BehaviorVersion{{Version: 3}, {Version: 1}},
		Knowledge:    []KnowledgeSource{{SourceID: "01994f64-6c00-7000-8000-0000000000a1", Status: "active", CurrentVersion: 2}},
		Voice:        []VoiceVersion{{Version: 2}, {Version: 1}},
	}
}

func fullRuntime() []RuntimeComponentState {
	state := make([]RuntimeComponentState, 0, len(runtimeDictionary))
	for _, spec := range runtimeDictionary {
		state = append(state, RuntimeComponentState{Component: spec.Component, Present: true, Ref: "runtime/" + spec.Component + "@1", Owner: spec.Owner, Responsible: spec.Responsible, Note: "present"})
	}
	return state
}

func verifiedConnection() ConnectionFact {
	return ConnectionFact{
		ConnectionID:    id.MustParse("01994f64-6c00-7000-8000-0000000000b1"),
		ConnectorID:     "abcp",
		ContractVersion: "1.4.0",
		AdapterBuild:    "abcp-adapter@7",
		Environment:     "sandbox",
		Status:          "connected_unverified",
		Capabilities: []CapabilityFact{{
			Capability: "parts_search", State: "verified", Reason: "probe ok",
			Evidence: &EvidenceFact{EvidenceID: id.MustParse("01994f64-6c00-7000-8000-0000000000c1"), Outcome: "verified", CheckedAt: releaseClock, ExpiresAt: releaseClock.Add(24 * time.Hour)},
			Gap:      false, Owner: "Technical Lead",
		}},
	}
}

func fullContent() ManifestContent {
	config := ConfigVersion{
		TenantID: id.MustParse("01994f64-6c00-7000-8000-000000000001"), EmployeeID: id.MustParse("01994f64-6c00-7000-8000-000000000002"),
		VersionID: id.MustParse("01994f64-6c00-7000-8000-000000000003"), Version: 1,
		Description: "Первая конфигурация пилота", Refs: AssembleConfigRefs(fullFacts()),
		ActorID: id.MustParse("01994f64-6c00-7000-8000-000000000004"), CreatedAt: releaseClock,
	}
	return BuildManifestContent(config, []ConnectionFact{verifiedConnection()}, fullRuntime())
}

// TestAssembleConfigRefsRecordsHonestAbsences: every missing component is a
// recorded absence with a reason, never an empty value; a text-only employee
// gets an honest not-applicable voice.
func TestAssembleConfigRefsRecordsHonestAbsences(t *testing.T) {
	refs := AssembleConfigRefs(ConfigFacts{Brief: phoneBrief(), DraftVersion: 2})
	if refs.Brief.Status != RefPresent || refs.Brief.DraftVersion != 2 {
		t.Fatalf("brief ref = %+v", refs.Brief)
	}
	if refs.Pack.Status != RefAbsent || refs.Pack.Note == "" {
		t.Fatalf("pack absence must be recorded with a reason: %+v", refs.Pack)
	}
	if refs.Behavior.Status != RefAbsent || refs.Behavior.Note == "" {
		t.Fatalf("behavior absence must be recorded: %+v", refs.Behavior)
	}
	if refs.Knowledge.Status != RefAbsent || len(refs.Knowledge.Sources) != 0 || refs.Knowledge.Note == "" {
		t.Fatalf("knowledge absence must be recorded: %+v", refs.Knowledge)
	}
	// Phone brief without voice: an honest applicable absence.
	if refs.Voice.Status != RefAbsent || refs.Voice.Note == "" {
		t.Fatalf("voice absence must be recorded: %+v", refs.Voice)
	}
	// Text-only employee: voice is not applicable, not missing.
	textRefs := AssembleConfigRefs(ConfigFacts{Brief: textBrief(), DraftVersion: 2})
	if textRefs.Voice.Status != RefNotApplicable || textRefs.Voice.Note == "" {
		t.Fatalf("text employee voice must be not_applicable: %+v", textRefs.Voice)
	}
	// Retired knowledge sources never enter the snapshot.
	refs = AssembleConfigRefs(ConfigFacts{Brief: textBrief(), DraftVersion: 2,
		Knowledge: []KnowledgeSource{
			{SourceID: "01994f64-6c00-7000-8000-0000000000a2", Status: "retired", CurrentVersion: 4},
			{SourceID: "01994f64-6c00-7000-8000-0000000000a1", Status: "active", CurrentVersion: 2},
		}})
	if len(refs.Knowledge.Sources) != 1 || refs.Knowledge.Sources[0].Version != 2 {
		t.Fatalf("retired source leaked into refs: %+v", refs.Knowledge)
	}
}

// TestDeriveManifestStatusCandidateOnCompleteSet: the candidate path is a
// pure function of a complete component set (precedent: admission_state of
// Story 1-11) — reachable today only in tests, but fixed in the domain.
func TestDeriveManifestStatusCandidateOnCompleteSet(t *testing.T) {
	if status := DeriveManifestStatus(fullContent()); status != ManifestStatusCandidate {
		t.Fatalf("complete set must derive candidate, got %q", status)
	}
}

// TestDeriveManifestStatusIncompleteOnAnyGap: any recorded gap — a missing
// component, an unproven capability, an absent runtime component — makes the
// manifest honestly incomplete.
func TestDeriveManifestStatusIncompleteOnAnyGap(t *testing.T) {
	cases := map[string]func(*ManifestContent){
		"missing pack":     func(c *ManifestContent) { c.Config.Refs.Pack.Status = RefAbsent },
		"missing behavior": func(c *ManifestContent) { c.Config.Refs.Behavior.Status = RefAbsent },
		"no knowledge":     func(c *ManifestContent) { c.Config.Refs.Knowledge.Status = RefAbsent },
		"voice absent":     func(c *ManifestContent) { c.Config.Refs.Voice.Status = RefAbsent },
		"capability gap": func(c *ManifestContent) {
			c.Connections[0].Capabilities[0].Gap = true
		},
		"capability unverified": func(c *ManifestContent) {
			c.Connections[0].Capabilities[0].State = "unverified"
		},
		"runtime absent": func(c *ManifestContent) {
			c.Runtime[3].Present = false
		},
	}
	for name, mutate := range cases {
		content := fullContent()
		mutate(&content)
		if status := DeriveManifestStatus(content); status != ManifestStatusIncomplete {
			t.Fatalf("%s must derive incomplete, got %q", name, status)
		}
	}
	// A not-applicable voice is not a gap for a text employee.
	content := fullContent()
	content.Config.Refs.Voice = VoiceRef{Status: RefNotApplicable, Note: "нет канала phone"}
	if status := DeriveManifestStatus(content); status != ManifestStatusCandidate {
		t.Fatalf("not_applicable voice must not make the manifest incomplete, got %q", status)
	}
}

// TestDeriveManifestStatusAlwaysIncompleteInThisSlice: the honest current
// state — no runtime surface exists, so a real seal is never candidate.
func TestDeriveManifestStatusAlwaysIncompleteInThisSlice(t *testing.T) {
	config := ConfigVersion{EmployeeID: id.MustParse("01994f64-6c00-7000-8000-000000000002"), Version: 1,
		VersionID: id.MustParse("01994f64-6c00-7000-8000-000000000003"), Description: "Пилот",
		Refs: AssembleConfigRefs(fullFacts()), CreatedAt: releaseClock}
	content := BuildManifestContent(config, []ConnectionFact{verifiedConnection()}, nil)
	if status := DeriveManifestStatus(content); status != ManifestStatusIncomplete {
		t.Fatalf("sealed manifest without runtime surfaces must be incomplete, got %q", status)
	}
	if len(content.Runtime) != 8 {
		t.Fatalf("runtime dictionary must list all 8 components, got %d", len(content.Runtime))
	}
	// Story 8-2: prompt_assembly and runtime_build are honestly present
	// (the versioned platform surfaces); the manifest still stays
	// incomplete — model_route, policy_bundle, eval_set and
	// autonomy_envelope remain honest gaps in this slice.
	present := map[string]bool{}
	for _, component := range content.Runtime {
		if component.Present {
			present[component.Component] = true
			if component.Ref == "" {
				t.Fatalf("runtime component %q claimed present without a ref", component.Component)
			}
		}
		if component.Owner == "" || component.Responsible == "" {
			t.Fatalf("runtime gap %q must name the owner and the responsible", component.Component)
		}
	}
	for _, required := range []string{"prompt_assembly", "runtime_build"} {
		if !present[required] {
			t.Fatalf("runtime component %q must be present since Story 8-2", required)
		}
	}
	for _, stillGap := range []string{"model_route", "policy_bundle", "eval_set", "autonomy_envelope"} {
		if present[stillGap] {
			t.Fatalf("runtime component %q must stay an honest gap in this slice", stillGap)
		}
	}
}

// TestManifestDigestIsCanonical: the digest depends on the canonical bytes —
// input array order does not matter, content changes do.
func TestManifestDigestIsCanonical(t *testing.T) {
	config := ConfigVersion{EmployeeID: id.MustParse("01994f64-6c00-7000-8000-000000000002"),
		VersionID: id.MustParse("01994f64-6c00-7000-8000-000000000003"), Version: 1,
		Description: "Пилот", Refs: AssembleConfigRefs(fullFacts()), CreatedAt: releaseClock}
	second := verifiedConnection()
	second.ConnectionID = id.MustParse("01994f64-6c00-7000-8000-0000000000b2")
	one := BuildManifestContent(config, []ConnectionFact{verifiedConnection(), second}, nil)
	two := BuildManifestContent(config, []ConnectionFact{second, verifiedConnection()}, nil)
	digestOne, err := ManifestDigest(one)
	if err != nil {
		t.Fatal(err)
	}
	digestTwo, err := ManifestDigest(two)
	if err != nil {
		t.Fatal(err)
	}
	if digestOne != digestTwo {
		t.Fatal("digest must not depend on input array order")
	}
	two.Config.Description = "Другое описание"
	changed, err := ManifestDigest(two)
	if err != nil {
		t.Fatal(err)
	}
	if changed == digestOne {
		t.Fatal("digest must change when the content changes")
	}
	if len(digestOne) != 64 {
		t.Fatalf("digest must be sha256 hex (64 chars), got %d", len(digestOne))
	}
}

// TestReadManifestPayloadRoundTrip: a payload of the current schema version
// reads back with an identical digest recomputation.
func TestReadManifestPayloadRoundTrip(t *testing.T) {
	content := fullContent()
	digest, err := ManifestDigest(content)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := CanonicalManifestJSON(content)
	if err != nil {
		t.Fatal(err)
	}
	read := ReadManifestPayload(raw, digest)
	if read.Status != QuarantineNone {
		t.Fatalf("current payload must read cleanly, got quarantine %q: %s", read.Status, read.Reason)
	}
	if read.Digest != digest || read.Content.Config.Version != content.Config.Version {
		t.Fatalf("round trip lost data: %+v", read)
	}
}

// TestReadManifestPayloadQuarantine: an unknown schema version, an unknown
// field and a digest mismatch each produce an explicit quarantine with the
// raw payload kept — nothing is silently discarded.
func TestReadManifestPayloadQuarantine(t *testing.T) {
	content := fullContent()
	digest, err := ManifestDigest(content)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := CanonicalManifestJSON(content)
	if err != nil {
		t.Fatal(err)
	}

	// Unknown schema version.
	var probe map[string]any
	if err = json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	future := string(raw)
	probe["schema_version"] = 99
	if futureBytes, err := json.Marshal(probe); err == nil {
		future = string(futureBytes)
	}
	read := ReadManifestPayload([]byte(future), digest)
	if read.Status != QuarantineSchemaVersion || read.Reason == "" {
		t.Fatalf("unknown schema version must quarantine: %+v", read)
	}
	if !bytes.Contains(read.Raw, []byte("schema_version")) {
		t.Fatal("quarantine must keep the raw payload")
	}

	// Unknown field inside the current schema version.
	probe["schema_version"] = ManifestSchemaCurrent
	probe["model_route_override"] = "silent-replacement"
	withUnknown, err := json.Marshal(probe)
	if err != nil {
		t.Fatal(err)
	}
	read = ReadManifestPayload(withUnknown, digest)
	if read.Status != QuarantineUnknownField || !strings.Contains(read.Reason, "не отбрасывается молча") {
		t.Fatalf("unknown field must quarantine honestly: %+v", read)
	}

	// Tampered content with the old digest.
	tampered := map[string]any{}
	if err = json.Unmarshal(raw, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered["schema_version"] = ManifestSchemaCurrent
	tampered["configuration"] = map[string]any{"version": 2, "description": "подмена"}
	tamperedBytes, err := json.Marshal(tampered)
	if err != nil {
		t.Fatal(err)
	}
	read = ReadManifestPayload(tamperedBytes, digest)
	if read.Status != QuarantineDigestMismatch || !strings.Contains(read.Reason, "целостность") {
		t.Fatalf("tampered payload must quarantine on digest: %+v", read)
	}
}

// TestRestoreCompatibilityReportsEveryDrift: the report names the concrete
// drift of every component against current facts — the restore is never
// blocked, but nothing is hidden either.
func TestRestoreCompatibilityReportsEveryDrift(t *testing.T) {
	refs := AssembleConfigRefs(fullFacts())
	sealed := BuildManifestContent(ConfigVersion{Refs: refs, CreatedAt: releaseClock}, []ConnectionFact{verifiedConnection()}, nil)
	current := RestoreFacts{
		DraftVersion:    7, // brief moved on
		PackAvailable:   false,
		BehaviorVersion: 4,
		VoiceVersion:    3,
		Knowledge:       map[id.UUID]int64{},
		Connections: []ConnectionFacts{{
			ConnectionID: verifiedConnection().ConnectionID,
			Status:       "revoked",
			Capabilities: []CapabilityFact{{Capability: "parts_search", State: "stale", Gap: true, Owner: "Technical Lead"}},
		}},
		Sealed: &sealed,
	}
	issues := RestoreCompatibility(refs, current)
	joined := map[string]bool{}
	for _, issue := range issues {
		joined[issue.Area] = true
		if issue.Subject == "" || issue.Problem == "" {
			t.Fatalf("issue must name the subject and the problem: %+v", issue)
		}
	}
	for _, area := range []string{"brief", "domain_pack", "behavior", "voice", "knowledge", "connection", "capability"} {
		if !joined[area] {
			t.Fatalf("drift of %q must be reported, issues: %+v", area, issues)
		}
	}

	// A fully aligned present keeps the report empty.
	aligned := RestoreFacts{
		DraftVersion: refs.Brief.DraftVersion, PackAvailable: true,
		BehaviorVersion: refs.Behavior.Version, VoiceVersion: refs.Voice.Version,
		Knowledge: map[id.UUID]int64{refs.Knowledge.Sources[0].SourceID: refs.Knowledge.Sources[0].Version},
	}
	if issues := RestoreCompatibility(refs, aligned); len(issues) != 0 {
		t.Fatalf("aligned facts must report no issues, got %+v", issues)
	}
}

// TestRuntimeDictionaryIsFixed: the dictionary order and the 8 component
// kinds are part of the contract; future surfaces fill them without a
// schema change.
func TestRuntimeDictionaryIsFixed(t *testing.T) {
	dictionary := RuntimeDictionary()
	want := []string{"model_route", "media_route", "prompt_assembly", "tool_schemas", "policy_bundle", "eval_set", "runtime_build", "autonomy_envelope"}
	if len(dictionary) != len(want) {
		t.Fatalf("dictionary size = %d", len(dictionary))
	}
	for i, spec := range dictionary {
		if spec.Component != want[i] {
			t.Fatalf("dictionary[%d] = %q, want %q", i, spec.Component, want[i])
		}
	}
}

// TestValidateReleaseDescription: the change description is bounded.
func TestValidateReleaseDescription(t *testing.T) {
	if err := ValidateReleaseDescription(""); err == nil {
		t.Fatal("empty description accepted")
	}
	if err := ValidateReleaseDescription(strings.Repeat("и", 301)); err == nil {
		t.Fatal("oversized description accepted")
	}
	if err := ValidateReleaseDescription("Пилотная конфигурация №1"); err != nil {
		t.Fatalf("valid description rejected: %v", err)
	}
}

// TestReleaseConflictErrorCarriesCurrentState: the honest conflict carries
// the expected and current revisions plus the current refs.
func TestReleaseConflictErrorCarriesCurrentState(t *testing.T) {
	refs := AssembleConfigRefs(fullFacts())
	conflict := ReleaseConflictError{Expected: 2, Current: 3, Refs: refs}
	if conflict.Error() == "" || conflict.Unwrap() != ErrConflict {
		t.Fatalf("conflict error must wrap ErrConflict: %v", conflict)
	}
	var target ReleaseConflictError
	if !errors.As(error(conflict), &target) || target.Current != 3 || target.Refs.Pack.Status != RefPresent {
		t.Fatalf("conflict details lost: %+v", target)
	}
}
