package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

func workCurrent(state string, version int64) WorkItemCurrent {
	return WorkItemCurrent{WorkItemID: id.MustParse("01994f64-6c00-7000-8000-000000000101"), State: state, Version: version}
}

func TestTransitionLadderAllowsOnlyDictionarySteps(t *testing.T) {
	allowed := map[[2]string]bool{
		{WorkItemStateInWork, WorkItemStateWaiting}:     true,
		{WorkItemStateInWork, WorkItemStateResolved}:    true,
		{WorkItemStateInWork, WorkItemStateCancelled}:   true,
		{WorkItemStateWaiting, WorkItemStateInWork}:     true,
		{WorkItemStateWaiting, WorkItemStateResolved}:   true,
		{WorkItemStateWaiting, WorkItemStateCancelled}:  true,
		{WorkItemStateUntriaged, WorkItemStateInWork}:   false, // classification only
		{WorkItemStateUntriaged, WorkItemStateResolved}: false,
		{WorkItemStateResolved, WorkItemStateWaiting}:   false, // the matrix refusal
		{WorkItemStateCancelled, WorkItemStateInWork}:   false,
		{WorkItemStateSplit, WorkItemStateInWork}:       false,
		{WorkItemStateMerged, WorkItemStateResolved}:    false,
		{WorkItemStateInWork, WorkItemStateInWork}:      false,
		{WorkItemStateWaiting, WorkItemStateUntriaged}:  false,
		{WorkItemStateInWork, "nonexistent-state"}:      false,
	}
	for pair, want := range allowed {
		if got := TransitionAllowed(pair[0], pair[1]); got != want {
			t.Fatalf("TransitionAllowed(%s, %s) = %v, want %v", pair[0], pair[1], got, want)
		}
	}
}

func TestClassifyAllowedFromCoversActiveStatesOnly(t *testing.T) {
	for _, state := range []string{WorkItemStateUntriaged, WorkItemStateInWork, WorkItemStateWaiting} {
		if !ClassifyAllowedFrom(state) {
			t.Fatalf("classify must be allowed from %s", state)
		}
	}
	for _, state := range []string{WorkItemStateResolved, WorkItemStateCancelled, WorkItemStateSplit, WorkItemStateMerged} {
		if ClassifyAllowedFrom(state) {
			t.Fatalf("classify must be refused from %s", state)
		}
	}
}

func TestGuardClassifyRequiresTypeAndTitle(t *testing.T) {
	if err := GuardClassify(workCurrent(WorkItemStateUntriaged, 1), "поиск детали", "Подбор детали на Civic"); err != nil {
		t.Fatal(err)
	}
	if err := GuardClassify(workCurrent(WorkItemStateUntriaged, 1), "", "Название"); err == nil {
		t.Fatal("empty type must be refused")
	}
	if err := GuardClassify(workCurrent(WorkItemStateResolved, 3), "поиск детали", "Название"); err == nil {
		t.Fatal("terminal state must be refused")
	}
}

func TestGuardSplitRequiresTwoTypedTargets(t *testing.T) {
	current := workCurrent(WorkItemStateInWork, 2)
	if err := GuardSplit(current, []SplitTarget{{Type: "поиск детали", Title: "Деталь одна"}, {Type: "уточнение аналога", Title: "Аналог"}}); err != nil {
		t.Fatal(err)
	}
	if err := GuardSplit(current, []SplitTarget{{Type: "поиск детали", Title: "Одна цель"}}); err == nil {
		t.Fatal("split with a single target must be refused")
	}
	if err := GuardSplit(workCurrent(WorkItemStateSplit, 3), []SplitTarget{{Type: "a", Title: "b"}, {Type: "c", Title: "d"}}); err == nil {
		t.Fatal("split of a terminal item must be refused")
	}
	if err := GuardSplit(current, []SplitTarget{{Type: "", Title: "b"}, {Type: "c", Title: "d"}}); err == nil {
		t.Fatal("untyped target must be refused")
	}
}

func TestGuardMergeRejectsTerminalAndSelfMerge(t *testing.T) {
	target := WorkItemCurrent{WorkItemID: id.MustParse("01994f64-6c00-7000-8000-000000000001"), CaseID: id.MustParse("01994f64-6c00-7000-8000-0000000000aa"), State: WorkItemStateInWork}
	source := WorkItemCurrent{WorkItemID: id.MustParse("01994f64-6c00-7000-8000-000000000002"), CaseID: target.CaseID, State: WorkItemStateWaiting}
	if err := GuardMerge(target, []WorkItemCurrent{source}); err != nil {
		t.Fatal(err)
	}
	if err := GuardMerge(target, []WorkItemCurrent{target}); err == nil {
		t.Fatal("self merge must be refused")
	}
	resolved := source
	resolved.State = WorkItemStateResolved
	if err := GuardMerge(target, []WorkItemCurrent{resolved}); err == nil {
		t.Fatal("resolved source must be refused")
	}
	foreign := source
	foreign.CaseID = id.MustParse("01994f64-6c00-7000-8000-0000000000bb")
	if err := GuardMerge(target, []WorkItemCurrent{foreign}); err == nil {
		t.Fatal("cross-case source must be refused")
	}
	if err := GuardMerge(WorkItemCurrent{WorkItemID: target.WorkItemID, State: WorkItemStateMerged, CaseID: target.CaseID}, []WorkItemCurrent{source}); err == nil {
		t.Fatal("terminal target must be refused")
	}
}

func TestNormalizeMaterialKeyTrimsLowercasesAndBounds(t *testing.T) {
	key, err := NormalizeMaterialKey("  Автомобиль ")
	if err != nil || key != "автомобиль" {
		t.Fatalf("key = %q err = %v", key, err)
	}
	if _, err := NormalizeMaterialKey("   "); err == nil {
		t.Fatal("blank key must be refused")
	}
	if _, err := NormalizeMaterialKey("пробел внутри"); err == nil {
		t.Fatal("inner spaces must be refused")
	}
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := NormalizeMaterialKey(string(long)); err == nil {
		t.Fatal("overlong key must be refused")
	}
}

func TestFactValueCanonicalizesAndSeparatesNumbers(t *testing.T) {
	one, oneDigest, err := FactValue([]byte("1"))
	if err != nil {
		t.Fatal(err)
	}
	oneFloat, oneFloatDigest, err := FactValue([]byte("1.0"))
	if err != nil {
		t.Fatal(err)
	}
	if string(one) != "1" || string(oneFloat) != "1.0" {
		t.Fatalf("canonical bytes: %q %q", one, oneFloat)
	}
	if oneDigest == oneFloatDigest {
		t.Fatal("1 and 1.0 are materially different values (the 2-1 UseNumber discipline)")
	}
	precise, _, err := FactValue([]byte("9007199254740993"))
	if err != nil || string(precise) != "9007199254740993" {
		t.Fatalf("precision lost: %q err=%v", precise, err)
	}
	if _, _, err := FactValue([]byte("{\"broken\":")); err == nil {
		t.Fatal("malformed JSON must be refused")
	}
	big, _ := json.Marshal(map[string]any{"blob": string(make([]byte, 3000))})
	if _, _, err := FactValue(big); err == nil {
		t.Fatal("the 2 KiB dictionary bound must be enforced")
	}
}

func TestFactSourceChannelDictionary(t *testing.T) {
	if channel, err := FactSourceChannel(FactSourceChannelEvent, "phone"); err != nil || channel != "phone" {
		t.Fatalf("channel = %q err = %v", channel, err)
	}
	if channel, err := FactSourceChannel(FactSourceOperator, ""); err != nil || channel != "operator" {
		t.Fatalf("operator channel = %q err = %v", channel, err)
	}
	if _, err := FactSourceChannel("connector", ""); err == nil {
		t.Fatal("the reserved connector source has no producer and must be refused")
	}
	if _, err := FactSourceChannel("system", ""); err == nil {
		t.Fatal("the reserved system source has no producer and must be refused")
	}
	if _, err := FactSourceChannel(FactSourceChannelEvent, "carrier-pigeon"); err == nil {
		t.Fatal("an unknown event channel must be refused")
	}
}

func factVersion(versionNo int64, digest string) FactVersion {
	return FactVersion{VersionNo: versionNo, ValueDigest: digest, Value: []byte("{}")}
}

func confirmation(versionNo int64) FactConfirmation {
	return FactConfirmation{VersionNo: versionNo, ConfirmerType: ConfirmerOperatorRelay}
}

func TestDeriveFactStatusCoversTheMatrix(t *testing.T) {
	cases := []struct {
		name          string
		versions      []FactVersion
		confirmations []FactConfirmation
		want          string
	}{
		{"single unverified value", []FactVersion{factVersion(1, "a")}, nil, FactStatusUnverified},
		{"two values without confirmations conflict", []FactVersion{factVersion(1, "a"), factVersion(2, "b")}, nil, FactStatusConflicting},
		{"latest confirmed", []FactVersion{factVersion(1, "a"), factVersion(2, "b")}, []FactConfirmation{confirmation(2)}, FactStatusConfirmed},
		{"older confirmation with newer value conflicts", []FactVersion{factVersion(1, "a"), factVersion(2, "b")}, []FactConfirmation{confirmation(1)}, FactStatusConflicting},
		{"new value after a confirmed one conflicts again", []FactVersion{factVersion(1, "a"), factVersion(2, "a"), factVersion(3, "c")}, []FactConfirmation{confirmation(2)}, FactStatusConflicting},
		{"same digest re-recorded on top of a confirmed one stays confirmed", []FactVersion{factVersion(1, "a"), factVersion(2, "a")}, []FactConfirmation{confirmation(1)}, FactStatusConfirmed},
		{"no versions", nil, nil, FactStatusUnverified},
	}
	for _, testCase := range cases {
		if got, _ := DeriveFactStatus(testCase.versions, testCase.confirmations); got != testCase.want {
			t.Fatalf("%s: status = %s, want %s", testCase.name, got, testCase.want)
		}
	}
}

func TestGuardConfirmFactVersionEvidenceRules(t *testing.T) {
	eventID := id.MustParse("01994f64-6c00-7000-8000-0000000000e1")
	if err := GuardConfirmFactVersion(2, 1, ConfirmerOperatorRelay, eventID, ""); err != nil {
		t.Fatal(err)
	}
	if err := GuardConfirmFactVersion(2, 1, ConfirmerOperatorRelay, id.UUID(""), "Клиент назвал год в звонке"); err != nil {
		t.Fatal(err)
	}
	if err := GuardConfirmFactVersion(2, 1, "client", eventID, ""); err == nil {
		t.Fatal("the client confirmer has no producer in this slice")
	}
	if err := GuardConfirmFactVersion(2, 3, ConfirmerOperatorRelay, eventID, ""); err == nil {
		t.Fatal("a nonexistent version must be refused")
	}
	if err := GuardConfirmFactVersion(2, 1, ConfirmerOperatorRelay, id.UUID(""), ""); err == nil {
		t.Fatal("evidence is required")
	}
	if err := GuardConfirmFactVersion(2, 1, ConfirmerOperatorRelay, eventID, "и нота"); err == nil {
		t.Fatal("both evidence kinds at once must be refused")
	}
	long := string(make([]byte, 201))
	if err := GuardConfirmFactVersion(2, 1, ConfirmerOperatorRelay, id.UUID(""), long); err == nil {
		t.Fatal("the 200-rune note bound must be enforced")
	}
}

func TestRecommendedNextActionIsTheFixedDictionary(t *testing.T) {
	if RecommendedNextAction(WorkItemStateUntriaged) == "" || RecommendedNextAction(WorkItemStateInWork) == "" || RecommendedNextAction(WorkItemStateWaiting) == "" {
		t.Fatal("the active states carry the action text")
	}
	for _, state := range []string{WorkItemStateUntriaged, WorkItemStateInWork, WorkItemStateWaiting} {
		action := RecommendedNextAction(state)
		if !containsRussianAction(action) {
			t.Fatalf("action of %s must stay the honest Russian dictionary text, got %q", state, action)
		}
	}
}

func containsRussianAction(text string) bool {
	for _, r := range text {
		if r >= 0x400 && r <= 0x4FF {
			return true
		}
	}
	return false
}

func TestGuardReasonBoundsTheHistoryWhy(t *testing.T) {
	if err := GuardReason(""); err != nil {
		t.Fatal("an empty reason is allowed when the operator does not add one")
	}
	if err := GuardReason(string(make([]byte, 501))); err == nil {
		t.Fatal("the 500-rune bound must be enforced")
	}
}

var _ = time.Now

// TestFactValueRefusesExponentialNumbers covers the jsonb number
// discipline: exponential literals (1e10) are legal JSON but the jsonb
// storage expands them into the digit form — the canonical bytes and the
// stored rendering would diverge, and a dense exponential array would
// outrun the CHECK backstop. The digit form is the only accepted literal;
// the refusal is the honest rule error, never a 500 on the CHECK.
func TestFactValueRefusesExponentialNumbers(t *testing.T) {
	for _, raw := range []string{"1e10", "1E10", "1.5e-3", "[1e10,2]", `{"count":1e10}`} {
		if _, _, err := FactValue([]byte(raw)); err == nil {
			t.Fatalf("the exponential literal %q must be refused", raw)
		}
	}
	for _, raw := range []string{"10000000000", "-12.5", "[10000000000,2]", `{"count":10000000000}`, "0.000"} {
		if _, _, err := FactValue([]byte(raw)); err != nil {
			t.Fatalf("the digit form %q must be accepted: %v", raw, err)
		}
	}
	// The strings are never numbers: an 'e' inside a string value is
	// ordinary material.
	if value, _, err := FactValue([]byte(`{"model":"1e10"}`)); err != nil || string(value) != `{"model":"1e10"}` {
		t.Fatalf("the string material is untouched: %q err=%v", value, err)
	}
}
