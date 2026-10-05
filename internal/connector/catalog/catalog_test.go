package catalog

import (
	"testing"

	"github.com/artemiosu/staffy/internal/connector/domain"
)

func TestCatalogListsEveryPublishedDefinition(t *testing.T) {
	defs := List()
	if len(defs) < 7 {
		t.Fatalf("expected at least 7 definitions, got %d", len(defs))
	}
	ids := map[string]bool{}
	for _, d := range defs {
		ids[d.ID] = true
	}
	for _, want := range []string{"staffy-synthetic-telephony", "novofon-sip", "mango-office-sip", "jivo-chat", "abcp-parts", "yandex-speechkit", "exolve-sip"} {
		if !ids[want] {
			t.Errorf("definition %s missing", want)
		}
	}
}

func TestCatalogOnlySyntheticConnectorIsConnectable(t *testing.T) {
	for _, d := range List() {
		if d.ID == "staffy-synthetic-telephony" {
			if !d.Connectable() {
				t.Fatal("synthetic telephony must be connectable")
			}
			continue
		}
		if d.Connectable() {
			t.Errorf("%s must not be connectable", d.ID)
		}
	}
}

func TestCatalogStatusesAreTextOnly(t *testing.T) {
	valid := map[domain.SupportStatus]bool{domain.StatusSupported: true, domain.StatusPilot: true, domain.StatusCandidate: true, domain.StatusDeferred: true, domain.StatusUnavailable: true}
	seen := map[domain.SupportStatus]bool{}
	for _, d := range List() {
		if !valid[d.SupportStatus] {
			t.Errorf("%s has invalid status %q", d.ID, d.SupportStatus)
		}
		seen[d.SupportStatus] = true
	}
	if !seen[domain.StatusSupported] || !seen[domain.StatusCandidate] || !seen[domain.StatusDeferred] || !seen[domain.StatusUnavailable] {
		t.Fatalf("catalog must demonstrate the status vocabulary, got %v", seen)
	}
}

func TestNovofonPriorityNoteHiddenWithoutAdapter(t *testing.T) {
	def, ok := Get("novofon-sip", "1.0.0")
	if !ok {
		t.Fatal("novofon definition missing")
	}
	if def.AdapterImplemented {
		t.Fatal("novofon adapter must not be implemented in this slice")
	}
	if def.Connectable() {
		t.Fatal("novofon must not be connectable")
	}
	if def.SupportStatus != domain.StatusCandidate {
		t.Fatalf("novofon status = %s", def.SupportStatus)
	}
}

func TestExolveIsUnavailableAndNeverConnectable(t *testing.T) {
	def, ok := Get("exolve-sip", "1.0.0")
	if !ok {
		t.Fatal("exolve definition missing")
	}
	if def.SupportStatus != domain.StatusUnavailable {
		t.Fatalf("exolve status = %s", def.SupportStatus)
	}
	if def.Connectable() {
		t.Fatal("unavailable definition must never be connectable")
	}
}

func TestGetPinsExactVersion(t *testing.T) {
	if _, ok := Get("staffy-synthetic-telephony", "1.0.0"); !ok {
		t.Fatal("pinned version missing")
	}
	if _, ok := Get("staffy-synthetic-telephony", "9.9.9"); ok {
		t.Fatal("unknown version must not resolve")
	}
}

func TestListReturnsDefensiveCopies(t *testing.T) {
	first := List()
	first[0].Fields[0].Name = "mutated"
	second := List()
	if second[0].Fields[0].Name == "mutated" {
		t.Fatal("List must return defensive copies")
	}
}

func TestGroupByTypeCoversFixedOrder(t *testing.T) {
	grouped := GroupByType(List())
	if _, ok := grouped[domain.TypeStorage]; !ok {
		t.Fatal("storage group missing")
	}
	if len(grouped[domain.TypeSIP]) < 3 {
		t.Fatalf("expected at least 3 SIP definitions, got %d", len(grouped[domain.TypeSIP]))
	}
}
