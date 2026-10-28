package domain

// Story 7-2 step-03: the derived tool_schemas state — four honest
// branches of DeriveToolSchemasState and the explicit BuildRuntimeState
// wiring. The static note («the surface does not exist») is REPLACED by
// the derived note for every new seal; the sealed history itself is never
// recomputed (AD-10) — the derivation runs at sealing time only.

import (
	"strings"
	"testing"
)

// TestDeriveToolSchemasStateFourBranches: pack not pinned, pinned pack
// without a binding, a binding with all adapters not_implemented, and a
// fixture binding with at least one implemented adapter.
func TestDeriveToolSchemasStateFourBranches(t *testing.T) {
	spec := RuntimeComponentSpec{}
	for _, candidate := range runtimeDictionary {
		if candidate.Component == "tool_schemas" {
			spec = candidate
		}
	}
	if spec.Component == "" {
		t.Fatal("tool_schemas отсутствует в словаре")
	}

	// Ветвь 1: пак не закреплён — привязки нет.
	state := DeriveToolSchemasState(PackRef{Status: RefAbsent}, nil)
	if state.Component != "tool_schemas" || state.Present {
		t.Fatalf("ветвь «пак не закреплён»: %+v", state)
	}
	if !strings.Contains(state.Note, "Domain Pack не закреплён") {
		t.Fatalf("записка ветви 1: %q", state.Note)
	}

	// Ветвь 2: пак закреплён, привязки нет — производная записка, НЕ
	// статичная «поверхность не существует».
	present := PackRef{Status: RefPresent, ID: "auto-parts", Version: "6.0.0"}
	state = DeriveToolSchemasState(present, nil)
	if state.Present {
		t.Fatal("ветвь «без привязки»: Present обязан быть false")
	}
	if !strings.Contains(state.Note, "auto-parts@6.0.0 не привязывает tool-контракты") {
		t.Fatalf("записка ветви 2: %q", state.Note)
	}

	// Ветвь 3: привязка с честными not_implemented — видима, но вызов
	// невозможен, Present остаётся false.
	bindings := []ToolBindingFact{
		{ID: "auto-parts.vehicle-identification", Version: "1.0.0", AdapterStatus: "not_implemented"},
		{ID: "auto-parts.part-request-classification", Version: "1.0.0", AdapterStatus: "not_implemented"},
		{ID: "auto-parts.fitment-evidence", Version: "1.0.0", AdapterStatus: "not_implemented"},
	}
	state = DeriveToolSchemasState(present, bindings)
	if state.Present {
		t.Fatal("ветвь «not_implemented»: Present обязан быть false")
	}
	if !strings.Contains(state.Note, "привязывает 3 tool-контракт(ов), адаптеры не реализованы — вызов невозможен") {
		t.Fatalf("записка ветви 3: %q", state.Note)
	}

	// Ветвь 4 (fixture): хотя бы один реализованный адаптер — Present=true
	// с пином пака (живых implemented-контрактов в реестре нет — машина
	// покрыта фикстурой, честная граница среза).
	fixtureBindings := []ToolBindingFact{
		{ID: "auto-parts.vehicle-identification", Version: "1.0.0", AdapterStatus: "not_implemented"},
		{ID: "auto-parts.part-request-classification", Version: "1.0.0", AdapterStatus: "implemented"},
	}
	state = DeriveToolSchemasState(present, fixtureBindings)
	if !state.Present {
		t.Fatal("ветвь «implemented»: Present обязан быть true")
	}
	if state.Ref != "auto-parts@6.0.0: 2 tool-контракт(ов)" {
		t.Fatalf("Ref ветви 4: %q", state.Ref)
	}
	if !strings.Contains(state.Note, "реализовано адаптеров: 1") {
		t.Fatalf("записка ветви 4: %q", state.Note)
	}
}

// TestBuildRuntimeStateUsesDerivedToolSchemas: the dictionary keeps the
// fixed order and the eight components; the tool_schemas slot carries the
// DERIVED state, every other component keeps the static gap note.
func TestBuildRuntimeStateUsesDerivedToolSchemas(t *testing.T) {
	derived := DeriveToolSchemasState(PackRef{Status: RefPresent, ID: "auto-parts", Version: "6.1.0"}, []ToolBindingFact{
		{ID: "auto-parts.vehicle-identification", Version: "1.0.0", AdapterStatus: "not_implemented"},
	})
	state := BuildRuntimeState(derived)
	if len(state) != len(runtimeDictionary) {
		t.Fatalf("размер словаря = %d, ожидался %d", len(state), len(runtimeDictionary))
	}
	found := false
	for _, component := range state {
		if component.Component == "tool_schemas" {
			found = true
			if component.Note != derived.Note || component.Present != derived.Present {
				t.Fatalf("tool_schemas не подставлен: %+v", component)
			}
			continue
		}
		if !strings.Contains(component.Note, "Поверхность компонента ещё не существует") {
			t.Fatalf("прочий компонент обязан сохранить статичную записку gap: %+v", component)
		}
	}
	if !found {
		t.Fatal("tool_schemas отсутствует в словаре")
	}
}

// TestBuildManifestContentNilFallbackDerives: the nil runtime branch (the
// non-seal readers) derives the tool_schemas note from the config's pack
// ref instead of the static gap — the static note is not a default path.
func TestBuildManifestContentNilFallbackDerives(t *testing.T) {
	config := ConfigVersion{Refs: AssembleConfigRefs(ConfigFacts{Brief: Brief{}, DraftVersion: 1})}
	content := BuildManifestContent(config, nil, nil)
	for _, component := range content.Runtime {
		if component.Component == "tool_schemas" {
			if !strings.Contains(component.Note, "Domain Pack не закреплён — tool-контракты не привязаны") {
				t.Fatalf("nil-ветка обязана давать производную записку: %q", component.Note)
			}
			return
		}
	}
	t.Fatal("tool_schemas отсутствует")
}
