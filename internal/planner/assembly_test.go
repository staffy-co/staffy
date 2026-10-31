package planner_test

// The assembly matrix of Story 8-2 step-01: the round material is the
// versioned, deterministic, closed prompt form — byte-identical for
// the same context and repair cause (the verifiability of the form),
// complete over the context material (facts with full provenance,
// mode + ladder, policy pins as references, the read-tool inventory,
// the §8.1 pin, the fallback policy, the reply budget, the closed
// answer-form guide, the named repair cause) and closed at the
// top-level key set (no secret slot, no chain-of-thought slot, no
// retrieval slot — the form has no place for any of them).

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/planner"
)

// decodedContext marshals the valid material and decodes it through
// the strict contract surface (the assembly input is a VALIDATED
// context, always).
func decodedContext(t *testing.T) planner.TurnContext {
	t.Helper()
	raw, err := json.Marshal(validContextMaterial())
	if err != nil {
		t.Fatalf("маршализация контекста: %v", err)
	}
	context, refusal := planner.DecodeTurnContext(raw)
	if refusal != nil {
		t.Fatalf("валидный контекст: %v", refusal)
	}
	return context
}

func TestAssemblyIsByteDeterministic(t *testing.T) {
	context := decodedContext(t)
	first, err := planner.AssembleRoundMaterial(context, "").Bytes()
	if err != nil {
		t.Fatalf("сборка материала: %v", err)
	}
	second, err := planner.AssembleRoundMaterial(context, "").Bytes()
	if err != nil {
		t.Fatalf("повторная сборка материала: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("сборка обязана быть байтово-детерминированной:\n%s\n≠\n%s", first, second)
	}

	// The repair cause changes the material honestly: the repair round
	// carries the named cause, and the bytes differ from round 1.
	withCause, err := planner.AssembleRoundMaterial(context, "proposal.strict_decode: причина").Bytes()
	if err != nil {
		t.Fatalf("сборка repair-материала: %v", err)
	}
	if string(withCause) == string(first) {
		t.Fatal("repair-материал обязан отличаться от материала первого раунда")
	}
	againWithCause, err := planner.AssembleRoundMaterial(context, "proposal.strict_decode: причина").Bytes()
	if err != nil {
		t.Fatalf("повторная сборка repair-материала: %v", err)
	}
	if string(withCause) != string(againWithCause) {
		t.Fatal("repair-сборка обязана быть байтово-детерминированной")
	}
}

func TestAssemblyCarriesTheWholeContextMaterial(t *testing.T) {
	context := decodedContext(t)
	material := planner.AssembleRoundMaterial(context, "proposal.mode_ladder: причина")

	if material.AssemblyVersion != planner.AssemblyVersion {
		t.Fatalf("материал обязан нести слово версии сборки, получено %q", material.AssemblyVersion)
	}
	if material.Refs.TurnID != context.TurnID || material.Refs.TenantID != context.TenantID ||
		material.Refs.CaseID != context.CaseID || material.Refs.ConversationID != context.ConversationID ||
		material.Refs.EmployeeID != context.EmployeeID || material.Refs.WorkItemID != nil ||
		material.Refs.ReleaseManifestRef != context.ReleaseManifestRef || material.Refs.PackRef != context.PackRef {
		t.Fatalf("референсы материала обязаны зеркалить контекст: %+v", material.Refs)
	}

	// The facts ride with their FULL provenance — authority/source/
	// version/time (AD-8): the model reasons over evidence.
	if len(material.Facts) != 2 {
		t.Fatalf("материал обязан нести оба факта, получено %d", len(material.Facts))
	}
	vehicleMake := material.Facts[0]
	if vehicleMake.Key != "vehicle.make" || vehicleMake.Value != "Renault" {
		t.Fatalf("факт обязан приезжать как есть: %+v", vehicleMake)
	}
	if vehicleMake.Provenance.Source != "dialogue.intake" || vehicleMake.Provenance.Version != "1" {
		t.Fatalf("provenance факта обязана быть полной: %+v", vehicleMake.Provenance)
	}
	if vehicleMake.Provenance.At.IsZero() {
		t.Fatal("время наблюдения provenance обязано доезжать")
	}

	if material.Mode != context.Mode {
		t.Fatalf("режим материала обязан зеркалить контекст: %q ≠ %q", material.Mode, context.Mode)
	}
	if strings.Join(material.Ladder, ",") != strings.Join(planner.ModeLadder, ",") {
		t.Fatalf("лестница материала обязана быть зеркалом платформенной: %v", material.Ladder)
	}
	if material.PolicyPins != context.PolicyPins {
		t.Fatalf("пины политики обязаны ехать референсами: %+v", material.PolicyPins)
	}
	if len(material.Tools) != len(context.Tools) {
		t.Fatalf("инвентарь инструментов обязан доезжать целиком: %d ≠ %d", len(material.Tools), len(context.Tools))
	}
	if material.Tools[0].Name != "catalog.lookup@1.0.0" || len(material.Tools[0].InputSchema) != 3 {
		t.Fatalf("инструмент обязан нести имя@версия + схему входа: %+v", material.Tools[0])
	}

	// The §8.1 pin travels INTO the material: the model is required to
	// see the disclosure requirement.
	if !material.AIDisclosureRequired {
		t.Fatal("пин раскрытия ИИ §8.1 обязан доезжать в материал (контекст требует его)")
	}
	if material.FallbackPolicy != planner.FallbackHonestStop {
		t.Fatalf("fallback-политика хода обязана доезжать: %q", material.FallbackPolicy)
	}
	if material.ReplyBudgetRunes != planner.ReplyBudgetRunes {
		t.Fatalf("бюджет реплики обязан доезжать: %d", material.ReplyBudgetRunes)
	}

	// The closed answer-form guide: the model answers in the contract.
	if material.ProposalForm.SchemaVersion != planner.ProposalSchemaVersion {
		t.Fatalf("гид формы обязан нести версию схемы предложения: %d", material.ProposalForm.SchemaVersion)
	}
	if strings.Join(material.ProposalForm.IntentDictionary, ",") != strings.Join(planner.IntentDictionary, ",") {
		t.Fatalf("словарь намерений обязан доезжать: %v", material.ProposalForm.IntentDictionary)
	}
	if strings.Join(material.ProposalForm.SafetyDictionary, ",") != strings.Join(planner.SafetyDictionary, ",") {
		t.Fatalf("словарь safety обязан доезжать: %v", material.ProposalForm.SafetyDictionary)
	}
	if strings.Join(material.ProposalForm.ActionKindDictionary, ",") != strings.Join(planner.ActionKindDictionary, ",") {
		t.Fatalf("словарь видов действия обязан доезжать: %v", material.ProposalForm.ActionKindDictionary)
	}

	// The named repair cause travels on the repair round.
	if material.Repair != "proposal.mode_ladder: причина" {
		t.Fatalf("именованная причина отказа обязана доезжать в repair-материал: %q", material.Repair)
	}
}

func TestAssemblyFormIsClosedAtTheTopLevelKeySet(t *testing.T) {
	context := decodedContext(t)
	raw, err := planner.AssembleRoundMaterial(context, "").Bytes()
	if err != nil {
		t.Fatalf("сборка материала: %v", err)
	}
	var material map[string]any
	if err := json.Unmarshal(raw, &material); err != nil {
		t.Fatalf("материал обязан быть JSON: %v", err)
	}
	// The closed key set of the assembly form: a slot for secrets, a
	// slot for a hidden chain of thought or a slot for retrieved
	// material DOES NOT EXIST in the form — the honest proof is the
	// exact key set of the marshaled material.
	declared := map[string]bool{
		"assembly_version": true, "refs": true, "facts": true, "mode": true,
		"ladder": true, "policy_pins": true, "tools": true,
		"ai_disclosure_required": true, "fallback_policy": true,
		"reply_budget_runes": true, "proposal_form": true,
	}
	if len(material) != len(declared) {
		t.Fatalf("материал первого раунда обязан нести ровно %d ключей верхнего уровня, получено %d: %v", len(declared), len(material), material)
	}
	for key := range material {
		if !declared[key] {
			t.Fatalf("ключ %q вне закрытой формы материала — слота для него не существует", key)
		}
	}
	// Round 1 carries no repair key at all (omitempty — the honest
	// absence, not an empty string).
	if _, exists := material["repair"]; exists {
		t.Fatal("материал первого раунда не несёт ключа repair (omitempty — честное отсутствие)")
	}

	withRepair, err := planner.AssembleRoundMaterial(context, "cause").Bytes()
	if err != nil {
		t.Fatalf("сборка repair-материала: %v", err)
	}
	var repairMaterial map[string]any
	if err := json.Unmarshal(withRepair, &repairMaterial); err != nil {
		t.Fatalf("repair-материал обязан быть JSON: %v", err)
	}
	if _, exists := repairMaterial["repair"]; !exists {
		t.Fatal("repair-материал обязан нести ключ repair с поимённой причиной")
	}
}

func TestAssemblyVersionWordIsPlatformClean(t *testing.T) {
	if !strings.Contains(planner.AssemblyVersion, "assembly/") {
		t.Fatalf("слово версии сборки обязано быть платформенной формой: %q", planner.AssemblyVersion)
	}
	if !strings.HasPrefix(planner.RuntimeBuildRef, planner.Build+"+") {
		t.Fatalf("составное слово рантайма обязано строиться из билда машины и версии сборки: %q", planner.RuntimeBuildRef)
	}
	if !strings.HasSuffix(planner.RuntimeBuildRef, "+"+planner.AssemblyVersion) {
		t.Fatalf("составное слово рантайма обязано нести версию сборки: %q", planner.RuntimeBuildRef)
	}
}
