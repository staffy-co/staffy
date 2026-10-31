package planner_test

// The cross-field validation matrix of Story 8-1 step-01: the closed
// enums, the mandatory material and the cross-field rules of the
// proposal against the context — the mode ladder (a proposal never
// lowers the priority mode), the union reference of the proposed
// action, the reply and question budgets WITHOUT truncation, and the
// read requests against the inventory of the context (the dictionary
// discipline of 7-2/7-4 applied to the model's asks).

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/planner"
)

// decodedPair builds the decoded context and proposal of the matrix.
func decodedPair(t *testing.T) (planner.TurnContext, planner.TurnProposal) {
	t.Helper()
	context, refusal := decodeContext(t, validContextMaterial())
	if refusal != nil {
		t.Fatalf("контекст матрицы обязан быть валидным: %v", refusal)
	}
	proposal, refusal := decodeProposal(t, validProposalMaterial())
	if refusal != nil {
		t.Fatalf("предложение матрицы обязано быть валидным: %v", refusal)
	}
	return context, proposal
}

func TestValidProposalPassesTheWholeMatrix(t *testing.T) {
	context, proposal := decodedPair(t)
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal != nil {
		t.Fatalf("валидное предложение обязано проходить валидацию: %v", refusal)
	}
}

func TestDictionaryRuleMatrix(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(proposal *planner.TurnProposal)
		rule   string
	}{
		{
			name:   "намерение вне словаря",
			mutate: func(p *planner.TurnProposal) { p.Intent = "chitchat" },
			rule:   "proposal.intent",
		},
		{
			name:   "пустое намерение",
			mutate: func(p *planner.TurnProposal) { p.Intent = "" },
			rule:   "proposal.intent",
		},
		{
			name:   "режим вне словаря",
			mutate: func(p *planner.TurnProposal) { p.Mode = "urgent" },
			rule:   "proposal.mode",
		},
		{
			name:   "safety вне словаря",
			mutate: func(p *planner.TurnProposal) { p.Safety = "probably_fine" },
			rule:   "proposal.safety",
		},
		{
			name:   "вид действия вне union-словаря",
			mutate: func(p *planner.TurnProposal) { p.ProposedAction.Kind = "mutation" },
			rule:   "proposal.action_kind",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			context, proposal := decodedPair(t)
			testCase.mutate(&proposal)
			refusal := planner.ValidateTurnProposal(proposal, context)
			if refusal == nil {
				t.Fatalf("материал обязан быть отклонён по правилу %s", testCase.rule)
			}
			if refusal.Rule != testCase.rule {
				t.Fatalf("отказ обязан быть правилом %s, получено %q (%s)", testCase.rule, refusal.Rule, refusal.Detail)
			}
		})
	}
}

func TestModeLadderRuleNeverLowers(t *testing.T) {
	// The context is safety (the strictest word): a sales proposal is
	// a ladder DOWN — refused by name.
	context, proposal := decodedPair(t)
	proposal.Mode = planner.ModeSales
	refusal := planner.ValidateTurnProposal(proposal, context)
	if refusal == nil || refusal.Rule != "proposal.mode_ladder" {
		t.Fatalf("понижение safety→sales обязано давать отказ proposal.mode_ladder, получено %v", refusal)
	}
	// The SAME word passes (a turn keeps the mode of the conversation).
	proposal.Mode = planner.ModeSafety
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal != nil {
		t.Fatalf("равный режим обязан проходить: %v", refusal)
	}
	// A STRICTER mode passes too — raising is always allowed.
	proposal.Mode = planner.ModeSafety
	context.Mode = planner.ModeSales
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal != nil {
		t.Fatalf("повышение sales→safety обязано проходить: %v", refusal)
	}
	// The safety words of the proposal never remove the safety MODE of
	// the context — the ladder rule is the enforcement, so a «clear»
	// assessment under a safety mode still keeps the mode word strict.
	context.Mode = planner.ModeSafety
	proposal.Mode = planner.ModeSafety
	proposal.Safety = planner.SafetyClear
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal != nil {
		t.Fatalf("safety-слово предложения не снимает режим — режим остаётся safety и валиден: %v", refusal)
	}
}

func TestActionUnionReferenceRule(t *testing.T) {
	// A kind other than none REQUIRES the contract reference.
	_, proposal := decodedPair(t)
	proposal.ProposedAction.Kind = planner.ActionKindAction
	refusal := planner.ValidateTurnProposal(proposal, emptyContext(t))
	if refusal == nil || refusal.Rule != "proposal.action_ref" {
		t.Fatalf("вид action без контракт-референса обязан давать отказ proposal.action_ref, получено %v", refusal)
	}
	// A malformed reference is refused too.
	proposal.ProposedAction.ContractRef = "без-версии"
	if refusal := planner.ValidateTurnProposal(proposal, emptyContext(t)); refusal == nil || refusal.Rule != "proposal.action_ref" {
		t.Fatalf("референс без версии обязан давать отказ, получено %v", refusal)
	}
	// A well-formed reference passes; the proposal only NAMES.
	proposal.ProposedAction.ContractRef = "business-action@4.2.0"
	if refusal := planner.ValidateTurnProposal(proposal, emptyContext(t)); refusal != nil {
		t.Fatalf("вид action с референсом обязан проходить: %v", refusal)
	}
	// kind none with a reference is incoherent — refused.
	proposal.ProposedAction.Kind = planner.ActionKindNone
	proposal.ProposedAction.ContractRef = "business-action@4.2.0"
	if refusal := planner.ValidateTurnProposal(proposal, emptyContext(t)); refusal == nil || refusal.Rule != "proposal.action_ref" {
		t.Fatalf("вид none с референсом обязан давать отказ, получено %v", refusal)
	}
}

func TestReplyBudgetRuleRefusesWithoutTruncation(t *testing.T) {
	context, proposal := decodedPair(t)
	proposal.Reply.Text = strings.Repeat("р", planner.ReplyBudgetRunes)
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal != nil {
		t.Fatalf("реплика ровно в бюджете обязана проходить: %v", refusal)
	}
	proposal.Reply.Text = strings.Repeat("р", planner.ReplyBudgetRunes+1)
	refusal := planner.ValidateTurnProposal(proposal, context)
	if refusal == nil || refusal.Rule != "proposal.reply_budget" {
		t.Fatalf("реплика сверх бюджета обязана давать отказ proposal.reply_budget, получено %v", refusal)
	}
	if !strings.Contains(refusal.Detail, "усечения не существует") {
		t.Fatalf("отказ бюджета обязан честно называть отсутствие усечения, получено %q", refusal.Detail)
	}
	// NO truncation ever happens: the refused material keeps its full
	// length — the machine never shortens what it refuses.
	if got := len([]rune(proposal.Reply.Text)); got != planner.ReplyBudgetRunes+1 {
		t.Fatalf("отказ НЕ усекает материал: длина обязана остаться %d, получена %d", planner.ReplyBudgetRunes+1, got)
	}
	// An empty reply is not a reply — the turn must carry one.
	proposal.Reply.Text = ""
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal == nil || refusal.Rule != "proposal.reply_budget" {
		t.Fatalf("пустая реплика обязана давать отказ, получено %v", refusal)
	}
}

func TestQuestionBudgetRule(t *testing.T) {
	context, proposal := decodedPair(t)
	proposal.Questions = []planner.Question{{Text: strings.Repeat("в", planner.ReplyBudgetRunes+1)}}
	refusal := planner.ValidateTurnProposal(proposal, context)
	if refusal == nil || refusal.Rule != "proposal.question_budget" {
		t.Fatalf("вопрос сверх бюджета обязан давать отказ proposal.question_budget, получено %v", refusal)
	}
	proposal.Questions = []planner.Question{{Text: ""}}
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal == nil || refusal.Rule != "proposal.question_budget" {
		t.Fatalf("пустой вопрос обязан давать отказ, получено %v", refusal)
	}
	proposal.Questions = []planner.Question{{Text: "Какой объём двигателя?"}}
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal != nil {
		t.Fatalf("вопрос в бюджете обязан проходить: %v", refusal)
	}
}

func TestReadRequestAgainstInventoryMatrix(t *testing.T) {
	context, proposal := decodedPair(t)
	base := planner.ReadRequest{Tool: "catalog.lookup@1.0.0", Args: map[string]any{
		"query": "вентилятор радиатора",
	}}
	cases := []struct {
		name    string
		request planner.ReadRequest
		rule    string
	}{
		{
			name:    "инструмент вне инвентаря",
			request: planner.ReadRequest{Tool: "unknown.tool@1.0.0", Args: map[string]any{}},
			rule:    "proposal.read_tool",
		},
		{
			name:    "инструмент без формы имя@версия",
			request: planner.ReadRequest{Tool: "catalog.lookup", Args: map[string]any{}},
			rule:    "proposal.read_tool",
		},
		{
			name:    "обязательный аргумент отсутствует",
			request: planner.ReadRequest{Tool: base.Tool, Args: map[string]any{}},
			rule:    "proposal.read_arg",
		},
		{
			name:    "аргумент вне схемы инструмента",
			request: planner.ReadRequest{Tool: base.Tool, Args: map[string]any{"query": "x", "необъявленное": 1}},
			rule:    "proposal.read_arg",
		},
		{
			name:    "строка вместо целого",
			request: planner.ReadRequest{Tool: base.Tool, Args: map[string]any{"query": "x", "limit": "10"}},
			rule:    "proposal.read_arg",
		},
		{
			name:    "булево вместо целого",
			request: planner.ReadRequest{Tool: base.Tool, Args: map[string]any{"query": "x", "limit": true}},
			rule:    "proposal.read_arg",
		},
		{
			name:    "целое ниже границы",
			request: planner.ReadRequest{Tool: base.Tool, Args: map[string]any{"query": "x", "limit": json.Number("0")}},
			rule:    "proposal.read_arg",
		},
		{
			name:    "целое выше границы",
			request: planner.ReadRequest{Tool: base.Tool, Args: map[string]any{"query": "x", "limit": json.Number("51")}},
			rule:    "proposal.read_arg",
		},
		{
			name:    "целое вместо строки",
			request: planner.ReadRequest{Tool: base.Tool, Args: map[string]any{"query": 5}},
			rule:    "proposal.read_arg",
		},
		{
			name:    "строка вне enum-словаря схемы",
			request: planner.ReadRequest{Tool: "vehicle.identify@2.0.0", Args: map[string]any{"vin": "VIN9"}},
			rule:    "proposal.read_arg",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			proposal.ReadRequests = []planner.ReadRequest{testCase.request}
			refusal := planner.ValidateTurnProposal(proposal, context)
			if refusal == nil {
				t.Fatalf("read-запрос обязан быть отклонён по правилу %s", testCase.rule)
			}
			if refusal.Rule != testCase.rule {
				t.Fatalf("отказ обязан быть правилом %s, получено %q (%s)", testCase.rule, refusal.Rule, refusal.Detail)
			}
		})
	}
	// The enum word inside the schema passes; the integer bounds are
	// inclusive; the optional flag carries its absence honestly.
	proposal.ReadRequests = []planner.ReadRequest{
		{Tool: "vehicle.identify@2.0.0", Args: map[string]any{"vin": "VIN17"}},
		{Tool: base.Tool, Args: map[string]any{"query": "x", "limit": json.Number("50")}},
		{Tool: base.Tool, Args: map[string]any{"query": "x", "limit": json.Number("1")}},
	}
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal != nil {
		t.Fatalf("read-запросы внутри схем инвентаря обязаны проходить: %v", refusal)
	}
}

func TestProvenanceMandatoryEverywhere(t *testing.T) {
	context, proposal := decodedPair(t)
	proposal.FactProposals = []planner.FactProposal{{
		Key: "vehicle.confirmed_make", Value: "Renault",
		Provenance: planner.Provenance{Source: "", Version: "1"},
	}}
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal == nil || refusal.Rule != "proposal.fact_provenance" {
		t.Fatalf("fact-предложение без источника provenance обязано давать отказ, получено %v", refusal)
	}
	// Each probe starts from the clean pair — the mutations never leak
	// into the next refusal.
	_, proposal = decodedPair(t)
	proposal.Contradictions = []planner.Contradiction{{
		Key: "vehicle.year", Existing: 2019, Proposed: 2020,
		Provenance: planner.Provenance{Source: "catalog.lookup@1.0.0", Version: "1"},
	}}
	// The zero time of the provenance is a refusal too (the At field).
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal == nil || refusal.Rule != "proposal.contradictions" {
		t.Fatalf("противоречие без времени provenance обязано давать отказ, получено %v", refusal)
	}
	_, proposal = decodedPair(t)
	proposal.MemoryPatch = []planner.MemoryPatchEntry{{
		Key: "vehicle.make", Value: "Renault",
		Provenance: planner.Provenance{Source: "s", Version: "1"},
	}}
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal == nil || refusal.Rule != "proposal.memory_patch" {
		t.Fatalf("запись патча без полного provenance обязана давать отказ, получено %v", refusal)
	}
	// A platform value outside the typing dictionary is a refusal in
	// every fact-bearing list.
	proposal.MemoryPatch = []planner.MemoryPatchEntry{{
		Key: "vehicle.options", Value: []any{"lua"},
		Provenance: planner.Provenance{Source: "s", Version: "1"},
	}}
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal == nil || refusal.Rule != "proposal.memory_patch" {
		t.Fatalf("массив в значении патча обязан давать отказ типизации, получено %v", refusal)
	}
}

func TestEvidenceRefRule(t *testing.T) {
	context, proposal := decodedPair(t)
	proposal.EvidenceRefs = []planner.EvidenceRef{{Type: "", ID: "vehicle.make"}}
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal == nil || refusal.Rule != "proposal.evidence_refs" {
		t.Fatalf("evidence-ссылка без типа обязана давать отказ, получено %v", refusal)
	}
	proposal.EvidenceRefs = []planner.EvidenceRef{{Type: "fact", ID: ""}}
	if refusal := planner.ValidateTurnProposal(proposal, context); refusal == nil || refusal.Rule != "proposal.evidence_refs" {
		t.Fatalf("evidence-ссылка без идентификатора обязана давать отказ, получено %v", refusal)
	}
}

// emptyContext builds a minimal valid context for the validation
// probes that do not need the inventory (the action-union rules).
func emptyContext(t *testing.T) planner.TurnContext {
	t.Helper()
	context, refusal := decodeContext(t, validContextMaterial())
	if refusal != nil {
		t.Fatalf("контекст обязан быть валидным: %v", refusal)
	}
	return context
}
