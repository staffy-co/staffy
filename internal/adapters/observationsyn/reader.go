// Package observationsyn is the synthetic Owning System of the test contour
// (the ingresssyn/verificationsyn precedent): it answers the typed read
// requests of the authority matrix deterministically over the fixed
// scenario dictionary — no network call exists, the provider DTO never
// leaves the adapter, the answers are the canonical projection bytes only.
// The successful reads count per (material field, external reference): the
// second read of the same field returns the version+1 (the AC-8 new-version
// probe). The scenario script is the constructor's map (the composition
// roots own the wiring — AD-2); production contours never wire this
// adapter, a nil reader refuses honestly before any record. The mutable
// counters live INSIDE the adapter instance of the test contour — no
// global mutable state exists.
package observationsyn

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
)

// The scenario words: the fixed dictionary of the synthetic contour — the
// ok word and the four reason words of the read-failure vocabulary.
const (
	ScenarioOK                = "ok"
	ScenarioProviderTimeout   = casesDomain.ReadReasonProviderTimeout
	ScenarioMalformedResponse = casesDomain.ReadReasonMalformedResponse
	ScenarioInsufficientScope = casesDomain.ReadReasonInsufficientScope
	ScenarioReadError         = casesDomain.ReadReasonReadError
)

// scenarioStatus maps every reason word onto the honest status word: a
// timeout, a malformed answer and a read error are FAILED reads (the answer
// is not a value); the missing scope is UNKNOWN (the value was never
// claimable — the field stays honestly unknown, never a guessed value).
var scenarioStatus = map[string]string{
	casesDomain.ReadReasonProviderTimeout:   casesDomain.ObservationStatusFailed,
	casesDomain.ReadReasonMalformedResponse: casesDomain.ObservationStatusFailed,
	casesDomain.ReadReasonInsufficientScope: casesDomain.ObservationStatusUnknown,
	casesDomain.ReadReasonReadError:         casesDomain.ObservationStatusFailed,
}

// defaultValues are the canonical projection bytes of the ok scenario per
// material field of the matrix v1 (the stable canonical form — the same
// content always yields the same bytes).
var defaultValues = map[string][]byte{
	casesDomain.MaterialFieldPrice:        []byte(`{"price_quote":"3000"}`),
	casesDomain.MaterialFieldAvailability: []byte(`{"stock_count":"4"}`),
	casesDomain.MaterialFieldOrderStatus:  []byte(`{"status_word":"in_progress"}`),
}

// Scenario is one scripted answer of the synthetic owning system: the
// scenario word (ok or a reason word), the canonical projection value bytes
// of the ok scenario (nil — the per-field default of the contour) and the
// provider age (how far back the provider observed the value — the
// staleness probe of the panel; 0 — the fresh read).
type Scenario struct {
	Word  string
	Value []byte
	Age   time.Duration
}

// Reader implements the cases OwningSystemReader port. The scenario script
// is keyed by the material field; a field without a script row answers ok
// (the honest default of the contour — the successful read path). The
// per-reference overrides (ScriptFor) win over the per-field script for ONE
// request anchor — the journeys contour scripts the price answer of one
// work item without touching the shared field default of the other chains.
type Reader struct {
	mu       sync.Mutex
	script   map[string]Scenario
	refs     map[string]Scenario
	counters map[string]int64
}

// New wires the synthetic reader with the scenario script (keyed by the
// material field). An unknown scenario word is a wiring programming error —
// the constructor refuses it immediately (never a mid-request failure).
func New(script map[string]Scenario) *Reader {
	copied := make(map[string]Scenario, len(script))
	for key, scenario := range script {
		switch scenario.Word {
		case ScenarioOK, ScenarioProviderTimeout, ScenarioMalformedResponse, ScenarioInsufficientScope, ScenarioReadError:
		default:
			panic(fmt.Sprintf("неизвестное слово сценария синтетического контура: %q", scenario.Word))
		}
		copied[key] = scenario
	}
	return &Reader{script: copied, refs: map[string]Scenario{}, counters: map[string]int64{}}
}

// ScriptFor scripts the answer of ONE request anchor (the material field +
// the external reference): the per-reference override wins over the per-field
// script and the default of that field for the reads of exactly this anchor.
// The fixture-only journeys toggle (the composition root owns the wiring —
// production contours never wire this adapter); an unknown scenario word is
// refused exactly like the constructor script.
func (r *Reader) ScriptFor(materialKey, externalReference string, scenario Scenario) {
	switch scenario.Word {
	case ScenarioOK, ScenarioProviderTimeout, ScenarioMalformedResponse, ScenarioInsufficientScope, ScenarioReadError:
	default:
		panic(fmt.Sprintf("неизвестное слово сценария синтетического контура: %q", scenario.Word))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refs[materialKey+"\x1f"+externalReference] = scenario
}

// Read answers one typed request deterministically: the ok scenario counts
// the successful reads per (material field, external reference) — the
// version is the counter, the external id is the stable carrier name of the
// request anchor, the observed time is the request moment minus the
// scripted age (the staleness probe); a failure scenario answers the status
// word with the reason word and NEVER a value (the assumed value is not a
// class). One attempt — no retry exists in the contour.
func (r *Reader) Read(ctx context.Context, request casesPort.OwningSystemReadRequest) (casesPort.OwningSystemReading, error) {
	scenario := Scenario{Word: ScenarioOK}
	r.mu.Lock()
	if scripted, ok := r.refs[request.MaterialKey+"\x1f"+request.ExternalReference]; ok {
		scenario = scripted
	} else if scripted, ok := r.script[request.MaterialKey]; ok {
		scenario = scripted
	}
	r.mu.Unlock()
	if scenario.Word != ScenarioOK {
		return casesPort.OwningSystemReading{
			Status: scenarioStatus[scenario.Word],
			Reason: scenario.Word,
		}, nil
	}
	value := scenario.Value
	if value == nil {
		value = defaultValues[request.MaterialKey]
	}
	r.mu.Lock()
	counterKey := request.MaterialKey + "\x1f" + request.ExternalReference
	r.counters[counterKey]++
	version := r.counters[counterKey]
	r.mu.Unlock()
	return casesPort.OwningSystemReading{
		Status:          casesDomain.ObservationStatusOk,
		Value:           value,
		ExternalID:      carrierID(request.OwningSystem, request.Projection, request.ExternalReference),
		ExternalVersion: fmt.Sprintf("v%d", version),
		ObservedAt:      request.Now.Add(-scenario.Age),
	}, nil
}

// carrierID renders the stable external id of the value carrier: the owning
// system word, the projection word and the request anchor (bounded by the
// dictionary limit — the honest truncation keeps the front words).
func carrierID(owningSystem, projection, externalReference string) string {
	rendered := owningSystem + "/" + projection + "/" + externalReference
	if utf8.RuneCountInString(rendered) > casesDomain.MaxExternalIDRunes {
		runes := []rune(rendered)
		rendered = string(runes[:casesDomain.MaxExternalIDRunes])
	}
	return rendered
}
