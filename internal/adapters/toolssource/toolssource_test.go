package toolssource

import (
	"testing"

	"github.com/staffy-co/staffy/internal/tools"
)

// TestSourceServesTheEmbeddedRegistry: the production adapter serves the
// LIVE embedded registry of 7-2 — the canonical list and the immutable
// pin read of a published version (the clone discipline: the caller
// receives a copy, the registry stays the single source).
func TestSourceServesTheEmbeddedRegistry(t *testing.T) {
	source := New()
	contracts := source.List()
	if len(contracts) == 0 {
		t.Fatal("живой реестр 7-2 обязан нести контракты — адаптер обязан их отдавать")
	}
	published := contracts[0]
	pinned, ok := source.Get(published.ID, published.Version)
	if !ok {
		t.Fatalf("пин опубликованной версии %s@%s обязан резолвиться", published.ID, published.Version)
	}
	if pinned.ID != published.ID || pinned.Version != published.Version {
		t.Fatalf("пин обязан возвращать точную пару, получено %s@%s", pinned.ID, pinned.Version)
	}
	// The invented future does not exist.
	if _, ok := source.Get(published.ID, "999.999.999"); ok {
		t.Fatal("изобретённая версия не обязана резолвиться")
	}
	// The dictionary of the live registry stays the platform safety
	// dictionary (the write class is structurally impossible).
	for _, contract := range contracts {
		if contract.DeterminismClass != "deterministic_read" {
			t.Fatalf("словарь determinism_class обязан нести единственное значение, обнаружено %q", contract.DeterminismClass)
		}
	}
}

// Compile-time pin of the platform typing dictionary mirror.
var _ = tools.ToolFieldTypes
