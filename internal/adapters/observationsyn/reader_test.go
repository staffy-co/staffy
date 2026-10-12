package observationsyn

import (
	"context"
	"strings"
	"testing"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
)

// The synthetic contour matrix: the five deterministic scenarios of the
// Code Map (ok with the value/version/time, the new version by the counter,
// timeout/malformed/insufficient_scope) plus the honest read_error word of
// the dictionary — no network call, no provider DTO, the canonical
// projection bytes only.

func readRequest(materialKey string, now time.Time) casesPort.OwningSystemReadRequest {
	return casesPort.OwningSystemReadRequest{
		MaterialKey: materialKey, OwningSystem: casesDomain.OwningSystemCatalog,
		Projection: casesDomain.ProjectionPriceQuote, ExternalReference: "wi-1", Now: now,
	}
}

func TestReadOkScenarioCarriesTheValueVersionAndTime(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	reader := New(map[string]Scenario{})
	reading, err := reader.Read(context.Background(), readRequest(casesDomain.MaterialFieldPrice, now))
	if err != nil {
		t.Fatal(err)
	}
	if reading.Status != casesDomain.ObservationStatusOk || reading.Reason != "" {
		t.Fatalf("the default scenario is the honest ok, got %+v", reading)
	}
	if string(reading.Value) != `{"price_quote":"3000"}` {
		t.Fatalf("the default price value is the canonical projection bytes, got %s", reading.Value)
	}
	if reading.ExternalVersion != "v1" {
		t.Fatalf("the first successful read is v1, got %q", reading.ExternalVersion)
	}
	if reading.ExternalID != "catalog/price_quote/wi-1" {
		t.Fatalf("the external id is the stable carrier name, got %q", reading.ExternalID)
	}
	if !reading.ObservedAt.Equal(now) {
		t.Fatalf("a fresh ok observation carries the provider time of the read moment, got %v", reading.ObservedAt)
	}
	// The per-field defaults of the matrix v1.
	for key, want := range map[string]string{
		casesDomain.MaterialFieldAvailability: `{"stock_count":"4"}`,
		casesDomain.MaterialFieldOrderStatus:  `{"status_word":"in_progress"}`,
	} {
		field := readRequest(key, now)
		field.OwningSystem, field.Projection = casesDomain.OwningSystemErp, casesDomain.ProjectionStatusWord
		reading, err := reader.Read(context.Background(), field)
		if err != nil {
			t.Fatal(err)
		}
		if string(reading.Value) != want {
			t.Fatalf("the default value of %s is %s, got %s", key, want, reading.Value)
		}
	}
}

func TestReadSecondSuccessfulReadIsTheNewVersion(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	reader := New(map[string]Scenario{})
	first, err := reader.Read(context.Background(), readRequest(casesDomain.MaterialFieldPrice, now))
	if err != nil {
		t.Fatal(err)
	}
	second, err := reader.Read(context.Background(), readRequest(casesDomain.MaterialFieldPrice, now.Add(time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	if first.ExternalVersion != "v1" || second.ExternalVersion != "v2" {
		t.Fatalf("the successful read counter is monotonic: v1 then v2, got %q and %q", first.ExternalVersion, second.ExternalVersion)
	}
	if first.ExternalID != second.ExternalID {
		t.Fatalf("the external id of the same field anchor is stable, got %q and %q", first.ExternalID, second.ExternalID)
	}
	// A different external reference counts its own ladder (the per-field
	// isolation of the counters).
	other := readRequest(casesDomain.MaterialFieldPrice, now)
	other.ExternalReference = "wi-2"
	third, err := reader.Read(context.Background(), other)
	if err != nil {
		t.Fatal(err)
	}
	if third.ExternalVersion != "v1" {
		t.Fatalf("a different anchor counts its own ladder, got %q", third.ExternalVersion)
	}
}

func TestReadScenarioAgeShiftsTheProviderTime(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	reader := New(map[string]Scenario{
		casesDomain.MaterialFieldAvailability: {Word: ScenarioOK, Age: 10 * time.Minute},
	})
	reading, err := reader.Read(context.Background(), readRequest(casesDomain.MaterialFieldAvailability, now))
	if err != nil {
		t.Fatal(err)
	}
	if !reading.ObservedAt.Equal(now.Add(-10 * time.Minute)) {
		t.Fatalf("the scripted age shifts the provider-observed time back, got %v", reading.ObservedAt)
	}
	if string(reading.Value) != `{"stock_count":"4"}` {
		t.Fatalf("the scripted ok scenario keeps the per-field default value, got %s", reading.Value)
	}
}

func TestReadFailureScenariosCarryTheReasonAndNeverAValue(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	// The honest status mapping: the timeout, the malformed answer and the
	// read error are failed reads; the missing scope is the honest unknown.
	for word, wantStatus := range map[string]string{
		ScenarioProviderTimeout:   casesDomain.ObservationStatusFailed,
		ScenarioMalformedResponse: casesDomain.ObservationStatusFailed,
		ScenarioInsufficientScope: casesDomain.ObservationStatusUnknown,
		ScenarioReadError:         casesDomain.ObservationStatusFailed,
	} {
		reader := New(map[string]Scenario{casesDomain.MaterialFieldOrderStatus: {Word: word}})
		request := readRequest(casesDomain.MaterialFieldOrderStatus, now)
		request.OwningSystem, request.Projection = casesDomain.OwningSystemErp, casesDomain.ProjectionStatusWord
		reading, err := reader.Read(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if reading.Status != wantStatus || reading.Reason != word {
			t.Fatalf("the %s scenario is the %s read with its reason word, got %+v", word, wantStatus, reading)
		}
		if len(reading.Value) != 0 {
			t.Fatalf("the %s scenario never carries a value, got %s", word, reading.Value)
		}
		if reading.ExternalVersion != "" || reading.ExternalID != "" {
			t.Fatalf("the %s scenario never carries the carrier words, got %+v", word, reading)
		}
		if !reading.ObservedAt.IsZero() {
			t.Fatalf("the %s scenario never carries the provider time, got %v", word, reading.ObservedAt)
		}
		// A scripted value of a failure scenario stays unwritten (the
		// assumed value is not a class).
		scripted := New(map[string]Scenario{casesDomain.MaterialFieldOrderStatus: {Word: word, Value: []byte(`{"status_word":"guessed"}`)}})
		reading, err = scripted.Read(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if len(reading.Value) != 0 {
			t.Fatalf("the scripted value of the %s scenario never leaves the adapter, got %s", word, reading.Value)
		}
	}
	// The failure reads never count: the next successful read of the field
	// stays at the version of the successful ladder.
	reader := New(map[string]Scenario{casesDomain.MaterialFieldPrice: {Word: ScenarioProviderTimeout}})
	if _, err := reader.Read(context.Background(), readRequest(casesDomain.MaterialFieldPrice, now)); err != nil {
		t.Fatal(err)
	}
	scripted := New(map[string]Scenario{})
	// A fresh reader counts from v1 (the counters are per adapter instance —
	// the composition roots own the state).
	reading, err := scripted.Read(context.Background(), readRequest(casesDomain.MaterialFieldPrice, now))
	if err != nil {
		t.Fatal(err)
	}
	if reading.ExternalVersion != "v1" {
		t.Fatalf("the counters are the instance state of the wired adapter, got %q", reading.ExternalVersion)
	}
}

func TestNewRefusesTheUnknownScenarioWord(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("an unknown scenario word is a wiring programming error — the constructor refuses it")
		}
		if !strings.Contains(recovered.(string), "неизвестное слово сценария") {
			t.Fatalf("the refusal names the wiring error, got %v", recovered)
		}
	}()
	New(map[string]Scenario{"price": {Word: "magic_scenario"}})
}

func TestCarrierIDIsBounded(t *testing.T) {
	if rendered := carrierID("catalog", "price_quote", strings.Repeat("w", 100)); len([]rune(rendered)) != casesDomain.MaxExternalIDRunes {
		t.Fatalf("the carrier id is bounded by the dictionary limit, got %d runes", len([]rune(rendered)))
	}
}
