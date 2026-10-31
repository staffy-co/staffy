package domain_test

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/action/domain"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
)

// The mirror contract of the action domain: the dictionaries that mirror
// the living sources are pinned here — drift turns one of these tests red
// (the action domain never imports the source packages; this test imports
// BOTH sides and compares). Pinning levels are honest and explicit:
//   - full-list equality (additions and renames caught): ModeOrder 2-5,
//     OwningSystems/MaterialFields 2-7, GateIDWords vs the machine registry;
//   - constant pins (renames and removals caught): identity roles, freshness
//     words — a NEW source word requires a conscious dictionary review here.

const registryRoot = "../../../_bmad-output/planning-artifacts/architecture/architecture-staffy-2026-09-13/"

// TestConversationModeWordsMirrorTheModeLadder pins the 7-word ladder of 2-5
// (exact order: strictest first).
func TestConversationModeWordsMirrorTheModeLadder(t *testing.T) {
	if got, want := domain.ConversationModeWords, casesDomain.ModeOrder; !reflect.DeepEqual(got, want) {
		t.Fatalf("зеркало режимов разошлось с лестницей 2-5:\nдействие: %v\nисточник: %v", got, want)
	}
}

// TestOwningSystemAndMaterialFieldWordsMirrorTheObservationMatrix pins the
// 2-7 owning-system and material-field dictionaries (exact lists).
func TestOwningSystemAndMaterialFieldWordsMirrorTheObservationMatrix(t *testing.T) {
	if got, want := domain.OwningSystemWords, casesDomain.OwningSystems; !reflect.DeepEqual(got, want) {
		t.Fatalf("зеркало владеющих систем разошлось с матрицей 2-7:\nдействие: %v\nисточник: %v", got, want)
	}
	if got, want := domain.MaterialFieldWords, casesDomain.MaterialFields; !reflect.DeepEqual(got, want) {
		t.Fatalf("зеркало материальных полей разошлось с матрицей 2-7:\nдействие: %v\nисточник: %v", got, want)
	}
}

// TestFreshnessWordsMirrorTheObservationDerivations pins the four honest
// derivation words of 2-7 (fresh never dresses failed up).
func TestFreshnessWordsMirrorTheObservationDerivations(t *testing.T) {
	want := []string{
		casesDomain.ObservationStateFresh,
		casesDomain.ObservationStateStale,
		casesDomain.ObservationStateFailed,
		casesDomain.ObservationStateUnknown,
	}
	if got := domain.FreshnessWords; !reflect.DeepEqual(got, want) {
		t.Fatalf("зеркало слов свежести разошлось с деривациями 2-7:\nдействие: %v\nисточник: %v", got, want)
	}
}

// TestRoleWordsMirrorIdentityRoles pins the five identity roles (constant
// pins: a rename or removal turns red; a NEW role demands the conscious
// dictionary review of this test — never a silent drift).
func TestRoleWordsMirrorIdentityRoles(t *testing.T) {
	want := []string{
		string(identityDomain.Owner),
		string(identityDomain.Configurator),
		string(identityDomain.Operator),
		string(identityDomain.ExpertQA),
		string(identityDomain.IntegrationAdmin),
	}
	if got := domain.RoleWords; !reflect.DeepEqual(got, want) {
		t.Fatalf("зеркало ролей разошлось с identity:\nдействие: %v\nисточник: %v", got, want)
	}
}

// TestGateIDWordsMirrorTheMachineRegistry pins the gate identifier
// dictionary against the authoritative machine registry
// (gate-definitions.json) — exact ordered equality.
func TestGateIDWordsMirrorTheMachineRegistry(t *testing.T) {
	raw, err := os.ReadFile(registryRoot + "gate-definitions.json")
	if err != nil {
		t.Fatalf("реестр гейтов недоступен: %v", err)
	}
	var file struct {
		Definitions []struct {
			DefinitionID string `json:"definition_id"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("реестр гейтов не читается: %v", err)
	}
	if len(file.Definitions) == 0 {
		t.Fatal("реестр гейтов пуст — сверка невозможна (fail-closed)")
	}
	want := make([]string, 0, len(file.Definitions))
	for _, definition := range file.Definitions {
		want = append(want, definition.DefinitionID)
	}
	if got := domain.GateIDWords; !reflect.DeepEqual(got, want) {
		t.Fatalf("зеркало гейтов разошлось с машинным реестром GATE-REGISTRY:\nдействие: %v\nреестр: %v", got, want)
	}
}

// TestAllowedStagesMirrorTheGateRegistryStages pins the five traffic stages
// against the «## Traffic stages» section of GATE-REGISTRY.md (exact bullet
// set).
func TestAllowedStagesMirrorTheGateRegistryStages(t *testing.T) {
	raw, err := os.ReadFile(registryRoot + "GATE-REGISTRY.md")
	if err != nil {
		t.Fatalf("GATE-REGISTRY недоступен: %v", err)
	}
	bullet := regexp.MustCompile("^- `([a-z_]+)`:")
	want := []string{}
	inSection := false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "## ") {
			inSection = strings.TrimSpace(strings.TrimPrefix(line, "##")) == "Traffic stages"
			continue
		}
		if !inSection {
			continue
		}
		if match := bullet.FindStringSubmatch(line); match != nil {
			want = append(want, match[1])
		}
	}
	if len(want) == 0 {
		t.Fatal("секция стадий GATE-REGISTRY не найдена — сверка невозможна (fail-closed)")
	}
	if got := domain.AllowedStages; !reflect.DeepEqual(got, want) {
		t.Fatalf("зеркало стадий разошлось с GATE-REGISTRY «Traffic stages»:\nдействие: %v\nисточник: %v", got, want)
	}
}

// TestProofStatesMirrorTheAssuranceDictionary pins the 4.10 proof-state
// dictionary against the living 2-3 assurance dictionary (the BY NAME
// mirror): the three level words ride EXACTLY (none, basic, verified —
// additions and renames caught here) and the step_up word mirrors the
// step-up window flag of the derivation. The action domain never
// imports the cases package; this test imports BOTH sides and compares.
func TestProofStatesMirrorTheAssuranceDictionary(t *testing.T) {
	want := append(append([]string{}, casesDomain.AssuranceLevels...), "step_up")
	if got := domain.ProofStates; !reflect.DeepEqual(got, want) {
		t.Fatalf("зеркало proof-состояний разошлось со словарём 2-3 + step_up:\nдействие: %v\nисточник: %v", got, want)
	}
	// The disclosure gate rides the verified word of the SOURCE dictionary.
	if domain.ProofStateAllowsDisclosure(casesDomain.AssuranceVerified) != true {
		t.Fatal("слово verified источника обязано открывать раскрытие")
	}
}
