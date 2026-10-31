package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 2.7 unit matrix: the authority matrix v1, the dictionaries, the
// guards (the field outside the matrix, the honest port answers), the
// read-time derivations (the freshness words) and the conflict badge over
// the canonical bytes.

func TestAuthorityMatrixV1CoversTheFixedFields(t *testing.T) {
	// The three fields of the Domain Pack v0 with the exact owning system
	// words, the projections, the conflict rule and the freshness seconds.
	expected := map[string]AuthorityEntry{
		"price": {
			MaterialKey: "price", OwningSystem: OwningSystemCatalog, ExternalIDCarrier: ExternalIDCarrierWorkItem,
			Projection: ProjectionPriceQuote, ConflictRule: ConflictRuleShowConflict, FreshnessSeconds: 300,
		},
		"availability": {
			MaterialKey: "availability", OwningSystem: OwningSystemCatalog, ExternalIDCarrier: ExternalIDCarrierWorkItem,
			Projection: ProjectionStockCount, ConflictRule: ConflictRuleShowConflict, FreshnessSeconds: 60,
		},
		"order_status": {
			MaterialKey: "order_status", OwningSystem: OwningSystemErp, ExternalIDCarrier: ExternalIDCarrierWorkItem,
			Projection: ProjectionStatusWord, ConflictRule: ConflictRuleShowConflict, FreshnessSeconds: 120,
		},
	}
	if len(authorityMatrixV1) != len(expected) {
		t.Fatalf("the matrix v1 carries exactly the three fields of the Domain Pack v0, got %d", len(authorityMatrixV1))
	}
	for key, want := range expected {
		entry, ok := AuthorityEntryOf(key)
		if !ok {
			t.Fatalf("the field %q must be declared in the matrix", key)
		}
		if entry != want {
			t.Fatalf("the matrix row of %q must be %+v, got %+v", key, want, entry)
		}
	}
}

func TestDictionariesAreClosed(t *testing.T) {
	// No status, reason, owning system, field or projection word appears
	// outside the fixed dictionary (the fail-closed basis of the slice).
	if len(ObservationStatuses) != 3 {
		t.Fatalf("the status dictionary carries three words, got %v", ObservationStatuses)
	}
	if len(ReadReasons) != 4 {
		t.Fatalf("the reason dictionary carries four words, got %v", ReadReasons)
	}
	if len(OwningSystems) != 2 {
		t.Fatalf("the owning system dictionary carries two words, got %v", OwningSystems)
	}
	if len(MaterialFields) != 3 {
		t.Fatalf("the material field dictionary carries three words, got %v", MaterialFields)
	}
	if len(MatrixProjections) != 3 {
		t.Fatalf("the projection dictionary carries three words, got %v", MatrixProjections)
	}
	for _, field := range MaterialFields {
		if !MaterialFieldInMatrix(field) {
			t.Fatalf("the dictionary field %q must be in the matrix", field)
		}
	}
	for _, status := range ObservationStatuses {
		if !inDictionary(ObservationStatuses, status) {
			t.Fatalf("the status %q must be in its own dictionary", status)
		}
	}
}

func TestGuardMaterialFieldInMatrixRefusesOutsideHonestly(t *testing.T) {
	// The field outside the matrix: the honest refusal names the field and
	// the full dictionary (the machine-readable 400 field-not-in-matrix).
	err := GuardMaterialFieldInMatrix("magic_field")
	if err == nil {
		t.Fatal("the field outside the matrix must refuse")
	}
	if !strings.Contains(err.Error(), "magic_field") {
		t.Fatalf("the refusal names the field, got %q", err.Error())
	}
	for _, field := range MaterialFields {
		if !strings.Contains(err.Error(), "«"+field+"»") {
			t.Fatalf("the refusal lists the dictionary word %q, got %q", field, err.Error())
		}
	}
	if !strings.Contains(err.Error(), "unknown как класс") {
		t.Fatalf("the refusal names the unknown class, got %q", err.Error())
	}
	for _, field := range MaterialFields {
		if err := GuardMaterialFieldInMatrix(field); err != nil {
			t.Fatalf("the declared field %q must pass, got %v", field, err)
		}
	}
}

func TestGuardReadOwnCommandRequiresTheAnchors(t *testing.T) {
	caseID, workItemID := newID(), newID()
	if err := GuardReadOwnCommand(caseID, workItemID, "price"); err != nil {
		t.Fatalf("the anchored command must pass, got %v", err)
	}
	if err := GuardReadOwnCommand(id.UUID(""), workItemID, "price"); err == nil {
		t.Fatal("the case anchor is required")
	}
	if err := GuardReadOwnCommand(caseID, id.UUID(""), "price"); err == nil {
		t.Fatal("the work item anchor is required")
	}
	if err := GuardReadOwnCommand(caseID, workItemID, ""); err == nil {
		t.Fatal("the material field is required")
	}
	if err := GuardReadOwnCommand(caseID, workItemID, strings.Repeat("x", MaxObservationMaterialRunes+1)); err == nil {
		t.Fatal("the material field is bounded")
	}
}

func TestGuardObservationMaterialOkRequiresTheValue(t *testing.T) {
	observed := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	if err := GuardObservationMaterial(ObservationStatusOk, "", []byte(`{"price_quote":"3000"}`), "catalog/price_quote/wi-1", "v1", observed); err != nil {
		t.Fatalf("the honest ok answer must pass, got %v", err)
	}
	if err := GuardObservationMaterial(ObservationStatusOk, ReadReasonProviderTimeout, []byte(`{}`), "id", "v1", observed); err == nil {
		t.Fatal("ok never carries a refusal reason")
	}
	if err := GuardObservationMaterial(ObservationStatusOk, "", nil, "id", "v1", observed); err == nil {
		t.Fatal("ok always carries the value")
	}
	if err := GuardObservationMaterial(ObservationStatusOk, "", []byte(`{}`), "", "v1", observed); err == nil {
		t.Fatal("ok always carries the external id")
	}
	if err := GuardObservationMaterial(ObservationStatusOk, "", []byte(`{}`), "id", "", observed); err == nil {
		t.Fatal("ok always carries the provider version")
	}
	if err := GuardObservationMaterial(ObservationStatusOk, "", []byte(`{}`), "id", "v1", time.Time{}); err == nil {
		t.Fatal("ok always carries the provider-observed time")
	}
	if err := GuardObservationMaterial(ObservationStatusOk, "", []byte(`{}`), strings.Repeat("i", MaxExternalIDRunes+1), "v1", observed); err == nil {
		t.Fatal("the external id is bounded")
	}
	if err := GuardObservationMaterial(ObservationStatusOk, "", []byte(`{}`), "id", strings.Repeat("v", MaxExternalVersionRunes+1), observed); err == nil {
		t.Fatal("the provider version is bounded")
	}
	if err := GuardObservationMaterial(ObservationStatusOk, "", make([]byte, MaxObservationValueBytes+1), "id", "v1", observed); err == nil {
		t.Fatal("the value is bounded by the dictionary bytes")
	}
}

func TestGuardObservationMaterialFailureNeverCarriesAValue(t *testing.T) {
	// failed/unknown: the reason word is the dictionary, the value is
	// ALWAYS empty — the assumed value is never written (AC-6).
	for _, status := range []string{ObservationStatusFailed, ObservationStatusUnknown} {
		for _, reason := range ReadReasons {
			if err := GuardObservationMaterial(status, reason, nil, "", "", time.Time{}); err != nil {
				t.Fatalf("the honest %s answer with the reason %q must pass, got %v", status, reason, err)
			}
		}
		if err := GuardObservationMaterial(status, "", nil, "", "", time.Time{}); err == nil {
			t.Fatalf("a %s answer always names its reason word", status)
		}
		if err := GuardObservationMaterial(status, "magic_reason", nil, "", "", time.Time{}); err == nil {
			t.Fatalf("a %s answer names the reason dictionary", status)
		}
		if err := GuardObservationMaterial(status, ReadReasonProviderTimeout, []byte(`{}`), "", "", time.Time{}); err == nil {
			t.Fatalf("a %s answer never carries a value", status)
		}
		if err := GuardObservationMaterial(status, ReadReasonProviderTimeout, nil, "", "v1", time.Time{}); err == nil {
			t.Fatalf("a %s answer never carries a provider version", status)
		}
	}
	if err := GuardObservationMaterial("maybe", ReadReasonProviderTimeout, nil, "", "", time.Time{}); err == nil {
		t.Fatal("the status word is the dictionary")
	}
}

func TestObservationValueCanonicalizesAndBounds(t *testing.T) {
	canonical, err := ObservationValue([]byte(`{"price_quote":   "3000"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != `{"price_quote":"3000"}` {
		t.Fatalf("the value is the canonical JSON, got %s", canonical)
	}
	if _, err := ObservationValue([]byte(`not json`)); err == nil {
		t.Fatal("the value is one JSON value")
	}
	if _, err := ObservationValue([]byte(`{} {}`)); err == nil {
		t.Fatal("the value is ONE JSON value")
	}
	if _, err := ObservationValue(make([]byte, MaxObservationValueBytes+1)); err == nil {
		t.Fatal("the value is bounded by the dictionary bytes")
	}
}

func TestDeriveObservationStateByAgeAndStatus(t *testing.T) {
	// ok inside the window → fresh; ok past the window → stale; failed and
	// unknown never dress up as fresh (the honest derivations, AC-3/6).
	if state := DeriveObservationState(ObservationStatusOk, 299*time.Second, 300); state != ObservationStateFresh {
		t.Fatalf("age 299 of the 300-second window is fresh, got %q", state)
	}
	if state := DeriveObservationState(ObservationStatusOk, 300*time.Second, 300); state != ObservationStateFresh {
		t.Fatalf("age exactly at the window edge is fresh, got %q", state)
	}
	if state := DeriveObservationState(ObservationStatusOk, 301*time.Second, 300); state != ObservationStateStale {
		t.Fatalf("age 301 of the 300-second window is stale, got %q", state)
	}
	if state := DeriveObservationState(ObservationStatusOk, 61*time.Second, 60); state != ObservationStateStale {
		t.Fatalf("the availability window is 60 seconds, got %q", state)
	}
	if state := DeriveObservationState(ObservationStatusOk, 121*time.Second, 120); state != ObservationStateStale {
		t.Fatalf("the order_status window is 120 seconds, got %q", state)
	}
	if state := DeriveObservationState(ObservationStatusFailed, 0, 300); state != ObservationStateFailed {
		t.Fatalf("failed stays failed, got %q", state)
	}
	if state := DeriveObservationState(ObservationStatusUnknown, time.Hour, 300); state != ObservationStateUnknown {
		t.Fatalf("unknown stays unknown, got %q", state)
	}
	if state := DeriveObservationState(ObservationStatusOk, -time.Minute, 300); state != ObservationStateFresh {
		t.Fatalf("a negative age (the provider clock ahead) stays fresh, got %q", state)
	}
}

func TestConflictBadgeOfComparesTheCanonicalBytes(t *testing.T) {
	// The badge is the byte comparison with the CONFIRMED exact fact
	// version: equal, differs, no_fact (AC-5).
	price := []byte(`{"price_quote":"3000"}`)
	same := FactVersion{VersionNo: 1, Value: []byte(`{"price_quote":"3000"}`)}
	other := FactVersion{VersionNo: 2, Value: []byte(`{"price_quote":"3500"}`)}
	if badge := ConflictBadgeOf(price, &same); badge != ConflictBadgeEqual {
		t.Fatalf("the equal bytes are equal, got %q", badge)
	}
	if badge := ConflictBadgeOf(price, &other); badge != ConflictBadgeDiffers {
		t.Fatalf("the different bytes differ, got %q", badge)
	}
	if badge := ConflictBadgeOf(price, nil); badge != ConflictBadgeNoFact {
		t.Fatalf("no confirmed fact is no_fact, got %q", badge)
	}
	// The canonical forms of the same material content are equal bytes.
	canonical, err := ObservationValue([]byte(`{ "price_quote" : "3000" }`))
	if err != nil {
		t.Fatal(err)
	}
	if badge := ConflictBadgeOf(canonical, &same); badge != ConflictBadgeEqual {
		t.Fatalf("the canonical form of the same content is equal, got %q", badge)
	}
}

func TestCurrentObservationIsTheMaximalNumber(t *testing.T) {
	workItem := newID()
	build := func(no int64) Observation {
		return Observation{ObservationID: newID(), WorkItemID: workItem, MaterialKey: "price", ObservationNo: no}
	}
	if current := CurrentObservation(nil); current != nil {
		t.Fatalf("no observations — no current, got %+v", current)
	}
	observations := []Observation{build(1), build(2), build(3)}
	current := CurrentObservation(observations)
	if current == nil || current.ObservationNo != 3 {
		t.Fatalf("the current observation is the maximal number, got %+v", current)
	}
	// The append-only discipline: the superseded rows stay readable (the
	// audit history of the panel).
	if len(observations) != 3 {
		t.Fatalf("the history is never rewritten, got %d rows", len(observations))
	}
}
