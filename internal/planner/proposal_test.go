package planner_test

// The proposal contract matrix of Story 8-1 step-01: the strict decode
// of the Turn Proposal — an undeclared key is a refusal, a foreign
// schema_version is a refusal (the wire-coherence discipline of
// 7-3/7-4), a wrong type is a refusal. The model output enters the
// machine as bytes and leaves the decode either as a closed form or
// as a named cause for the repair round.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/planner"
)

// validProposalMaterial builds the wire material of a well-formed
// proposal: the version, the dictionary words, one fact suggestion
// with provenance, one read request inside the inventory schema, one
// question, the none-kind action, the reply within the budget and one
// memory-patch entry. The raw bytes keep the decode honest.
func validProposalMaterial() map[string]any {
	return map[string]any{
		"schema_version": 1,
		"intent":         planner.IntentRespond,
		"mode":           planner.ModeSafety,
		"safety":         planner.SafetyCaution,
		"fact_proposals": []any{
			map[string]any{
				"key":   "vehicle.confirmed_make",
				"value": "Renault",
				"provenance": map[string]any{
					"source":  "catalog.lookup@1.0.0",
					"version": "1",
					"at":      "2026-10-31T10:05:00Z",
				},
			},
		},
		"contradictions": []any{},
		"read_requests": []any{
			map[string]any{
				"tool": "catalog.lookup@1.0.0",
				"args": map[string]any{
					"query":                "вентилятор радиатора",
					"limit":                10,
					"include_alternatives": true,
				},
			},
		},
		"questions": []any{
			map[string]any{"text": "Подтвердите, пожалуйста, объём двигателя."},
		},
		"proposed_action": map[string]any{"kind": planner.ActionKindNone},
		"reply":           map[string]any{"text": "Проверяю совместимость вентилятора радиатора для вашего автомобиля."},
		"evidence_refs": []any{
			map[string]any{"type": "fact", "id": "vehicle.make"},
		},
		"memory_patch": []any{},
	}
}

func decodeProposal(t *testing.T, material map[string]any) (planner.TurnProposal, *planner.RuleError) {
	t.Helper()
	raw, err := json.Marshal(material)
	if err != nil {
		t.Fatalf("маршализация материала предложения: %v", err)
	}
	return planner.DecodeTurnProposal(raw)
}

func TestProposalDecodesValidMaterial(t *testing.T) {
	proposal, refusal := decodeProposal(t, validProposalMaterial())
	if refusal != nil {
		t.Fatalf("валидное предложение отклонено: %v", refusal)
	}
	if proposal.SchemaVersion != planner.ProposalSchemaVersion {
		t.Fatalf("версия формы обязана быть %d, получена %d", planner.ProposalSchemaVersion, proposal.SchemaVersion)
	}
	if len(proposal.ReadRequests) != 1 || proposal.ReadRequests[0].Tool != "catalog.lookup@1.0.0" {
		t.Fatalf("read-запрос потерялся при decode: %+v", proposal.ReadRequests)
	}
	if proposal.ReadRequests[0].Args["limit"] != json.Number("10") {
		t.Fatalf("целочисленный аргумент обязан оставаться строгим числом, получено %T", proposal.ReadRequests[0].Args["limit"])
	}
}

func TestProposalRefusesUndeclaredKey(t *testing.T) {
	material := validProposalMaterial()
	material["chain_of_thought"] = "материал, которого не существует в форме"
	_, refusal := decodeProposal(t, material)
	if refusal == nil || refusal.Rule != "proposal.strict_decode" {
		t.Fatalf("undeclared-ключ предложения обязан давать отказ proposal.strict_decode, получено %v", refusal)
	}
}

func TestProposalRefusesForeignSchemaVersion(t *testing.T) {
	for _, version := range []any{0, 2, 99, -1} {
		material := validProposalMaterial()
		material["schema_version"] = version
		_, refusal := decodeProposal(t, material)
		if refusal == nil || refusal.Rule != "proposal.schema_version" {
			t.Fatalf("чужая версия формы %v обязана давать отказ proposal.schema_version, получено %v", version, refusal)
		}
		if !strings.Contains(refusal.Detail, "когерентность") {
			t.Fatalf("отказ чужой версии обязан называть когерентность версий, получено %q", refusal.Detail)
		}
	}
}

func TestProposalRefusesWrongFieldType(t *testing.T) {
	material := validProposalMaterial()
	material["reply"] = map[string]any{"text": 5}
	_, refusal := decodeProposal(t, material)
	if refusal == nil || refusal.Rule != "proposal.strict_decode" {
		t.Fatalf("неверный тип поля реплики обязан давать отказ decode, получено %v", refusal)
	}
}

func TestProposalRefusesTrailingMaterial(t *testing.T) {
	raw, err := json.Marshal(validProposalMaterial())
	if err != nil {
		t.Fatalf("маршализация: %v", err)
	}
	_, refusal := planner.DecodeTurnProposal(append(raw, []byte(" хвост")...))
	if refusal == nil || refusal.Rule != "proposal.strict_decode" {
		t.Fatalf("хвостовой материал обязан давать отказ decode, получено %v", refusal)
	}
}

func TestProposalRefusesNotAnObject(t *testing.T) {
	_, refusal := planner.DecodeTurnProposal([]byte(`[1,2,3]`))
	if refusal == nil || refusal.Rule != "proposal.strict_decode" {
		t.Fatalf("массив вместо объекта обязан давать отказ decode, получено %v", refusal)
	}
	_, refusal = planner.DecodeTurnProposal([]byte(``))
	if refusal == nil || refusal.Rule != "proposal.strict_decode" {
		t.Fatalf("пустой материал обязан давать отказ decode, получено %v", refusal)
	}
}
