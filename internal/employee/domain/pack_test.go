package domain_test

import (
	"testing"

	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/employee/packs"
)

func TestReferencePackVariantsAndImmutableCatalog(t *testing.T) {
	auto, ok := packs.Get("auto-parts", "1.0.0")
	if !ok {
		t.Fatal("missing auto parts pack")
	}
	if errors := domain.ValidateVariants(auto, map[string]any{"parts_interest": "both"}); len(errors) != 0 {
		t.Fatalf("valid variants: %v", errors)
	}
	for _, bad := range []map[string]any{{"unknown": true}, {"parts_interest": "pirated"}, {"parts_interest": 1}, {"parts_interest": "both", "policy_ref": "unsafe"}} {
		if errors := domain.ValidateVariants(auto, bad); len(errors) == 0 {
			t.Fatalf("accepted invalid variants: %v", bad)
		}
	}
	auto.Schema["parts_interest"] = domain.VariantField{Type: "boolean"}
	other, _ := packs.Get("auto-parts", "1.0.0")
	if other.Schema["parts_interest"].Type != "string" {
		t.Fatal("catalog mutated")
	}
	montagio, _ := packs.Get("montagio-services", "1.0.0")
	if len(domain.ValidateVariants(montagio, map[string]any{"engineering_plumbing": false, "heating": false})) == 0 {
		t.Fatal("accepted empty service selection")
	}
	if len(domain.ValidateVariants(montagio, map[string]any{"engineering_plumbing": true, "heating": false})) != 0 {
		t.Fatal("rejected valid service selection")
	}
	v2, _ := packs.Get("auto-parts", "2.0.0")
	if v2.UpgradeFrom["1.0.0"].Compatibility != "incompatible" {
		t.Fatal("missing declared impact")
	}
	reverse, ok := packs.Impact(domain.PackPin{ID: "auto-parts", Version: "2.0.0"}, other)
	if !ok || reverse.Compatibility != "incompatible" || len(reverse.ReviewSlots) == 0 {
		t.Fatalf("missing reverse impact: %+v %t", reverse, ok)
	}
	if len(domain.ValidateVariants(v2, map[string]any{"parts_interest": "original", "matching_depth": float64(4)})) == 0 {
		t.Fatal("accepted out-of-range integer")
	}
}
