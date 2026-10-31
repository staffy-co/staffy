package planner_test

// The contract mirrors of Story 8-1 step-01: the planner dictionaries
// are MIRRORS of the platform sources — the mode ladder mirrors the
// cases ModeOrder and the read-argument typing mirrors the tools
// dictionary. The mirrors are pinned element by element: a change on
// either side turns this test red BEFORE any code that trusts the
// words ships. Extending a dictionary is a conscious core change with
// a spec revision, never drift.

import (
	"testing"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/planner"
	"github.com/staffy-co/staffy/internal/tools"
)

func TestModeLadderMirrorsCasesModeOrder(t *testing.T) {
	ladder := planner.ModeLadder
	order := casesDomain.ModeOrder
	if len(ladder) != len(order) {
		t.Fatalf("лестница планировщика (%d слов) отличается по длине от ModeOrder случаев (%d) — зеркало сломано", len(ladder), len(order))
	}
	for index := range order {
		if ladder[index] != order[index] {
			t.Fatalf("позиция %d лестницы планировщика %q ≠ ModeOrder случаев %q — зеркало сломано (расширение словаря = осознанное core-изменение со спекой, не дрейф)", index, ladder[index], order[index])
		}
	}
	// The rank semantics mirror too: the smaller rank is the stricter
	// word, safety at the head, sales at the tail.
	if planner.ModeRank(planner.ModeSafety) != 0 {
		t.Fatalf("safety обязана быть строгим словом позиции 0, получена %d", planner.ModeRank(planner.ModeSafety))
	}
	if planner.ModeRank(planner.ModeSales) != len(order)-1 {
		t.Fatalf("sales обязана быть последним словом лестницы, получена %d", planner.ModeRank(planner.ModeSales))
	}
	for _, word := range order {
		if planner.ModeRank(word) != casesDomain.ModeRank(word) {
			t.Fatalf("ранг слова %q расходится с рангом случаев — зеркало сломано", word)
		}
	}
	if planner.ModeRank("не-слово-словаря") != -1 {
		t.Fatal("чужое слово обязано честно давать -1 — словарная проверка decode/валидации идёт раньше любого сравнения рангов")
	}
}

func TestReadArgTypesMirrorToolsFieldTypes(t *testing.T) {
	ours := planner.ReadArgTypes
	source := tools.ToolFieldTypes()
	if len(ours) != len(source) {
		t.Fatalf("словарь типизации read-аргументов (%d слов) отличается по длине от tools.ToolFieldTypes (%d) — зеркало сломано", len(ours), len(source))
	}
	for index := range source {
		if ours[index] != source[index] {
			t.Fatalf("позиция %d словаря типизации %q ≠ tools.ToolFieldTypes %q — реюз без расширения нарушен", index, ours[index], source[index])
		}
	}
}

func TestDictionariesAreClosedNonEmptyAndPlatformClean(t *testing.T) {
	dictionaries := []struct {
		name    string
		words   []string
		atLeast int
	}{
		{"намерений", planner.IntentDictionary, 4},
		{"лестницы режимов", planner.ModeLadder, 7},
		{"safety-состояний", planner.SafetyDictionary, 3},
		{"fallback-форм", planner.FallbackDictionary, 3},
		{"видов предлагаемого действия", planner.ActionKindDictionary, 4},
	}
	for _, dictionary := range dictionaries {
		if len(dictionary.words) < dictionary.atLeast {
			t.Fatalf("словарь %s несёт %d слов — закрытая форма среза не может быть пустой", dictionary.name, len(dictionary.words))
		}
		seen := map[string]bool{}
		for _, word := range dictionary.words {
			if word == "" {
				t.Fatalf("словарь %s несёт пустое слово", dictionary.name)
			}
			if seen[word] {
				t.Fatalf("словарь %s несёт дубликат %q", dictionary.name, word)
			}
			seen[word] = true
		}
	}
	// The outcome dictionary of the journal.
	if planner.OutcomeValidated != "validated" || planner.OutcomeRepaired != "repaired" || planner.OutcomeFailed != "failed" {
		t.Fatal("исходы журнала обязаны быть тремя закрытыми словами validated | repaired | failed")
	}
	// The build constant and the schema version travel with every
	// record — the honest staleness semantics of the journal.
	if planner.Build != "planner/1.0.0" {
		t.Fatalf("Build-константа машины обязана быть planner/1.0.0, получена %q", planner.Build)
	}
	if planner.ProposalSchemaVersion != 1 {
		t.Fatalf("версия формы предложения первого среза обязана быть 1, получена %d", planner.ProposalSchemaVersion)
	}
	// The reply budget is the platform text-material budget.
	if planner.ReplyBudgetRunes <= 0 {
		t.Fatalf("бюджет реплики обязан быть положительной платформенной константой, получено %d", planner.ReplyBudgetRunes)
	}
}
