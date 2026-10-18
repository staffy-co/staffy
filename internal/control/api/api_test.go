package api

import (
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/control/domain"
)

// The View mappers keep the wire honest: the safe mode read, the
// corruption marker, the defensive window copies, the verbatim
// provenance mapping.

func TestFromScopedControlMapsSafeModeAndCorruptionMarker(t *testing.T) {
	view := FromScopedControl(domain.ScopedControl{
		ScopeKind: "connector", ScopeRef: "montagio",
		Mode: "garbage", Reason: "повреждено", ActorID: "01994f64-6c00-7000-8000-000000000001",
		PrevMode: "human_only", Generation: 5, State: "active",
		UpdatedAt: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
	})
	if view.Mode != domain.ControlModePaused {
		t.Fatalf("a corrupted stored word must read as paused on the wire, got %q", view.Mode)
	}
	if !view.Corrupted {
		t.Fatalf("the corruption marker must travel to the wire")
	}
	if view.PrevMode != "human_only" || view.Generation != 5 || view.State != "active" {
		t.Fatalf("the verbatim material must survive the mapping, got %+v", view)
	}
}

func TestFromScopedControlDictionaryWordPassesUntouched(t *testing.T) {
	for _, mode := range domain.ControlModes {
		view := FromScopedControl(domain.ScopedControl{Mode: mode, State: "active"})
		if view.Mode != mode || view.Corrupted {
			t.Fatalf("the dictionary word %q must pass untouched and unmarked, got %+v", mode, view)
		}
	}
}

func TestFromScopedControlCopiesTheWindowDefensively(t *testing.T) {
	start := time.Date(2026, 10, 19, 13, 0, 0, 0, time.UTC)
	expires := start.Add(time.Hour)
	view := FromScopedControl(domain.ScopedControl{ScheduledStartAt: &start, ExpiresAt: &expires})
	if view.ScheduledStartAt == &start || view.ExpiresAt == &expires {
		t.Fatal("the window pointers must be defensive copies, never the caller's")
	}
	if !view.ScheduledStartAt.Equal(start) || !view.ExpiresAt.Equal(expires) {
		t.Fatalf("the window values must survive the copy, got %+v", view)
	}
	empty := FromScopedControl(domain.ScopedControl{})
	if empty.ScheduledStartAt != nil || empty.ExpiresAt != nil {
		t.Fatal("a nil window must stay nil on the wire (null, not a zero time)")
	}
}

func TestFromEffectiveControlMapsProvenanceAndIntersection(t *testing.T) {
	influencing := []domain.ScopedControl{
		{ScopeKind: "connector", ScopeRef: "montagio", Mode: domain.ControlModeHumanOnly, Reason: "победитель", State: "active", Generation: 2},
		{ScopeKind: "tenant", ScopeRef: "01994f64-6c00-7000-8000-000000000009", Mode: domain.ControlModeIntakeOnly, Reason: "проигравший", State: "active", Generation: 1},
	}
	view := FromEffectiveControl(domain.EffectiveControl{
		Mode:         domain.ControlModeHumanOnly,
		Influencing:  influencing,
		Reason:       "победитель",
		Intersection: []string{domain.ControlModeIntakeOnly, domain.ControlModeHumanOnly},
		Corrupted:    false,
	})
	if len(view.Influencing) != 2 || view.Influencing[0].ScopeRef != "montagio" {
		t.Fatalf("the provenance must map control by control, got %+v", view.Influencing)
	}
	if view.Mode != domain.ControlModeHumanOnly || view.Reason != "победитель" {
		t.Fatalf("the winner material must survive, got %+v", view)
	}
	if len(view.Intersection) != 2 || view.Intersection[0] != domain.ControlModeIntakeOnly {
		t.Fatalf("the intersection words must survive, got %v", view.Intersection)
	}

	// The mapped provenance is a fresh copy: mutating the view never
	// touches the caller's slice.
	view.Influencing[0].Reason = "mutated"
	if influencing[0].Reason != "победитель" {
		t.Fatal("the view provenance must be a defensive copy")
	}
}
