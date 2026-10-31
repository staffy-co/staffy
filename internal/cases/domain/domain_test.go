package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

func testOwnerID() id.UUID { return id.MustParse("01994f64-6c00-7000-8000-0000000000aa") }
func testOwner2ID() id.UUID {
	return id.MustParse("01994f64-6c00-7000-8000-0000000000ab")
}
func testEmployeeID() id.UUID {
	return id.MustParse("01994f64-6c00-7000-8000-0000000000ba")
}
func testEmployee2ID() id.UUID {
	return id.MustParse("01994f64-6c00-7000-8000-0000000000bb")
}
func testConnectionID() id.UUID {
	return id.MustParse("01994f64-6c00-7000-8000-0000000000cc")
}

func phoneAdmittedFacts() IngressFacts {
	return IngressFacts{
		Connection: ConnectionFacts{ConnectionID: testConnectionID(), Status: "connected_unverified",
			Environment: "test", ConnectorID: "staffy-synthetic-telephony", ConnectorType: ConnectorTypeSIP},
		Channel:   "phone",
		Direction: DirectionInbound,
		Modality:  ModalityVoice,
		Route: RouteFacts{
			Candidates: []RouteCandidate{{
				EmployeeID: testEmployeeID(), DecisionNo: 1, Stage: "synthetic", ProfileVersion: 1,
				Routes:   []RouteAdmissionWord{{Channel: "phone", Direction: "inbound", State: RouteWordAdmitted}},
				Coverage: []CoverageWord{{Channel: "phone", Direction: "inbound", ResponseOwnerID: testOwnerID()}},
			}},
			Entitlement: EntitlementWord{Status: "ok"},
		},
		ActiveOwnerIDs: map[id.UUID]bool{testOwnerID(): true},
	}
}

func TestLadderAcceptsAdmittedRouteWithActiveOwner(t *testing.T) {
	outcome := DeriveIngressOutcome(phoneAdmittedFacts())
	if outcome.Kind != OutcomeAccept {
		t.Fatalf("expected accept, got %s (%s)", outcome.Kind, outcome.RefusalKind)
	}
	if outcome.OwnerID != testOwnerID() || outcome.EmployeeID != testEmployeeID() {
		t.Fatalf("expected the pinned owner and the acting employee, got %s / %s", outcome.OwnerID, outcome.EmployeeID)
	}
}

func TestLadderRefusesRevokedConnection(t *testing.T) {
	facts := phoneAdmittedFacts()
	facts.Connection.Status = "revoked"
	outcome := DeriveIngressOutcome(facts)
	if outcome.Kind != OutcomeReject || outcome.RefusalKind != KindRevokedConnection {
		t.Fatalf("expected reject revoked_connection, got %+v", outcome)
	}
}

func TestLadderRefusesChannelConnectorTypeMismatch(t *testing.T) {
	facts := phoneAdmittedFacts()
	facts.Channel = "web_chat"
	facts.Modality = ModalityText
	outcome := DeriveIngressOutcome(facts)
	if outcome.Kind != OutcomeReject || outcome.RefusalKind != KindChannelMismatch {
		t.Fatalf("expected reject channel_mismatch, got %+v", outcome)
	}
	if !strings.Contains(outcome.Text, "web_chat") || !strings.Contains(outcome.Text, ConnectorTypeChat) {
		t.Fatalf("expected the channel and type names in the safe text, got %q", outcome.Text)
	}
}

func TestLadderRefusesWhenNoDecisionSealsConnection(t *testing.T) {
	facts := phoneAdmittedFacts()
	facts.Route.Candidates = nil
	outcome := DeriveIngressOutcome(facts)
	if outcome.Kind != OutcomeReject || outcome.RefusalKind != KindNoActiveRelease {
		t.Fatalf("expected reject no_active_release, got %+v", outcome)
	}
}

func TestLadderQuarantinesAmbiguousRouting(t *testing.T) {
	facts := phoneAdmittedFacts()
	second := facts.Route.Candidates[0]
	second.EmployeeID = testEmployee2ID()
	facts.Route.Candidates = append(facts.Route.Candidates, second)
	outcome := DeriveIngressOutcome(facts)
	if outcome.Kind != OutcomeQuarantine || outcome.RefusalKind != KindAmbiguousEmployee {
		t.Fatalf("expected quarantine ambiguous_employee, got %+v", outcome)
	}
	if !strings.Contains(outcome.Text, "2") {
		t.Fatalf("expected the candidate count in the text, got %q", outcome.Text)
	}
}

func TestLadderRefusesRouteNotAdmittedByDecision(t *testing.T) {
	facts := phoneAdmittedFacts()
	facts.Route.Candidates[0].Routes = []RouteAdmissionWord{{Channel: "phone", Direction: "inbound", State: RouteWordBlocked}}
	outcome := DeriveIngressOutcome(facts)
	if outcome.Kind != OutcomeReject || outcome.RefusalKind != KindRouteNotAdmitted {
		t.Fatalf("expected reject route_not_admitted, got %+v", outcome)
	}
}

func TestLadderRefusesMissingRouteInDecision(t *testing.T) {
	facts := phoneAdmittedFacts()
	facts.Route.Candidates[0].Routes = nil
	outcome := DeriveIngressOutcome(facts)
	if outcome.Kind != OutcomeReject || outcome.RefusalKind != KindRouteNotAdmitted {
		t.Fatalf("expected reject route_not_admitted for a missing route row, got %+v", outcome)
	}
}

func TestLadderRefusesInactiveOrMissingResponseOwner(t *testing.T) {
	facts := phoneAdmittedFacts()
	facts.ActiveOwnerIDs = map[id.UUID]bool{}
	outcome := DeriveIngressOutcome(facts)
	if outcome.Kind != OutcomeReject || outcome.RefusalKind != KindNoResponseOwner {
		t.Fatalf("expected reject no_response_owner for an inactive owner, got %+v", outcome)
	}
	if !strings.Contains(outcome.Text, testOwnerID().String()) {
		t.Fatalf("expected the unlock owner id in the text, got %q", outcome.Text)
	}

	facts = phoneAdmittedFacts()
	facts.Route.Candidates[0].Coverage = nil
	outcome = DeriveIngressOutcome(facts)
	if outcome.Kind != OutcomeReject || outcome.RefusalKind != KindNoResponseOwner {
		t.Fatalf("expected reject no_response_owner for a missing coverage row, got %+v", outcome)
	}
}

func TestLadderBoundedWhenEntitlementHardLimit(t *testing.T) {
	facts := phoneAdmittedFacts()
	facts.Route.Entitlement = EntitlementWord{Status: "blocked", Reason: "hard limit"}
	outcome := DeriveIngressOutcome(facts)
	if outcome.Kind != OutcomeBounded || outcome.RefusalKind != KindEntitlementBlocked {
		t.Fatalf("expected bounded entitlement_blocked, got %+v", outcome)
	}
}

func TestLadderBoundedWhenEnvelopeSizesExceedDictionary(t *testing.T) {
	facts := phoneAdmittedFacts()
	facts.EnvelopeBytes = FixedLoadEnvelope.MaxEnvelopeBytes + 1
	if outcome := DeriveIngressOutcome(facts); outcome.Kind != OutcomeBounded || outcome.RefusalKind != KindEnvelopeExceeded {
		t.Fatalf("expected bounded envelope_exceeded (envelope), got %+v", outcome)
	} else if outcome.Counter != facts.EnvelopeBytes {
		t.Fatalf("expected the byte counter, got %d", outcome.Counter)
	}

	facts = phoneAdmittedFacts()
	facts.PayloadBytes = FixedLoadEnvelope.MaxPayloadBytes + 1
	if outcome := DeriveIngressOutcome(facts); outcome.Kind != OutcomeBounded || outcome.RefusalKind != KindEnvelopeExceeded {
		t.Fatalf("expected bounded envelope_exceeded (payload), got %+v", outcome)
	}

	facts = phoneAdmittedFacts()
	facts.InflightCount = FixedLoadEnvelope.MaxInflightPerConnection
	if outcome := DeriveIngressOutcome(facts); outcome.Kind != OutcomeBounded || outcome.RefusalKind != KindEnvelopeExceeded {
		t.Fatalf("expected bounded envelope_exceeded (inflight), got %+v", outcome)
	}
}

func TestLadderOrderConnectionBeforeRouteFacts(t *testing.T) {
	// A revoked connection refuses even when the routing is also broken:
	// the ladder order is part of the contract.
	facts := phoneAdmittedFacts()
	facts.Connection.Status = "revoked"
	facts.Route.Candidates = nil
	outcome := DeriveIngressOutcome(facts)
	if outcome.RefusalKind != KindRevokedConnection {
		t.Fatalf("expected revoked_connection to fire before no_active_release, got %+v", outcome)
	}
}

func TestValidateEnvelopeAcceptsCanonicalShape(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	envelope := Envelope{
		SchemaVersion: 1, TenantClaim: testTenantID(), ConnectionID: testConnectionID(),
		Provider: "staffy-synthetic-telephony", ProviderEventID: "evt-1", ProviderSessionID: "sess-1",
		Direction: DirectionInbound, Modality: ModalityVoice, Channel: "phone",
		ProviderTimestamp: now.Add(-time.Minute), Payload: []byte(`{"a":1}`),
	}
	validated, err := ValidateEnvelope(envelope, now)
	if err != nil {
		t.Fatalf("expected the canonical envelope to validate: %v", err)
	}
	if validated.PayloadDigest != PayloadDigest([]byte(`{"a":1}`)) {
		t.Fatalf("expected the payload digest to be derived")
	}
}

func TestValidateEnvelopeRefusesEveryStructuralBreak(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	base := Envelope{
		SchemaVersion: 1, TenantClaim: testTenantID(), ConnectionID: testConnectionID(),
		Provider: "p", ProviderEventID: "e", ProviderSessionID: "s",
		Direction: DirectionInbound, Modality: ModalityVoice, Channel: "phone",
		ProviderTimestamp: now.Add(-time.Minute), Payload: []byte(`{}`),
	}
	cases := map[string]func(Envelope) Envelope{
		"schema_version":   func(e Envelope) Envelope { e.SchemaVersion = 2; return e },
		"tenant_id":        func(e Envelope) Envelope { e.TenantClaim = id.UUID(""); return e },
		"connection_id":    func(e Envelope) Envelope { e.ConnectionID = id.UUID("nope"); return e },
		"provider":         func(e Envelope) Envelope { e.Provider = ""; return e },
		"provider_event":   func(e Envelope) Envelope { e.ProviderEventID = strings.Repeat("x", 129); return e },
		"session":          func(e Envelope) Envelope { e.ProviderSessionID = "has space"; return e },
		"direction":        func(e Envelope) Envelope { e.Direction = "outbound"; return e },
		"channel":          func(e Envelope) Envelope { e.Channel = "sms"; return e },
		"modality":         func(e Envelope) Envelope { e.Modality = ModalityText; return e },
		"timestamp_zero":   func(e Envelope) Envelope { e.ProviderTimestamp = time.Time{}; return e },
		"timestamp_future": func(e Envelope) Envelope { e.ProviderTimestamp = now.Add(72 * time.Hour); return e },
		"payload_empty":    func(e Envelope) Envelope { e.Payload = nil; return e },
	}
	for name, mutate := range cases {
		if _, err := ValidateEnvelope(mutate(base), now); err == nil {
			t.Errorf("%s: expected a structural refusal", name)
		}
	}
}

func TestCanonicalPayloadIsDeterministicRegardlessOfKeyOrder(t *testing.T) {
	first, err := CanonicalPayload(json.RawMessage(`{"b":2,"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalPayload(json.RawMessage(`{"a":1,"b":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("expected deterministic canonical bytes, got %q vs %q", first, second)
	}
	if PayloadDigest(first) != PayloadDigest(second) {
		t.Fatal("expected identical digests for identical material content")
	}
}

func TestPayloadSealVerifiesAndRefusesTampering(t *testing.T) {
	key, err := NewSealKey()
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"event":"inbound-call"}`)
	seal := PayloadSeal(key, payload)
	if seal == "" || len(seal) != 64 {
		t.Fatalf("expected a 64-hex seal, got %q", seal)
	}
	if PayloadSeal(key, append(payload, 'x')) == seal {
		t.Fatal("expected the seal to change when the payload changes")
	}
	if PayloadSeal([]byte("short"), payload) != "" {
		t.Fatal("expected a short key to refuse instead of faking a seal")
	}
}

func TestDictionariesAreFixed(t *testing.T) {
	if len(AllowedChannels) != 9 || !AllowedChannels["phone"] || !AllowedChannels["mobile_app_chat"] {
		t.Fatalf("expected the nine-channel profile dictionary, got %d", len(AllowedChannels))
	}
	if RequiredConnectorType("phone") != ConnectorTypeSIP || RequiredConnectorType("email") != ConnectorTypeChat {
		t.Fatal("expected phone→SIP and text→chat/inbox")
	}
	if ModalityOfChannel("phone") != ModalityVoice || ModalityOfChannel("email") != ModalityText {
		t.Fatal("expected the channel-honest modality rule")
	}
	if CapturePolicyOfModality(ModalityVoice) != CapturePolicyMetadataOnly {
		t.Fatal("expected voice sessions to declare metadata_only capture (no media edge)")
	}
	if len(RefusalKinds) != 11 || len(RejectionKinds) != 10 {
		t.Fatalf("expected 11 refusal kinds and 10 rejection kinds, got %d / %d", len(RefusalKinds), len(RejectionKinds))
	}
	for _, kind := range RejectionKinds {
		if kind == KindAmbiguousEmployee {
			t.Fatal("ambiguous_employee quarantines events, it never lands in ingress_rejections")
		}
	}
	if !IsBoundedKind(KindEntitlementBlocked) || !IsBoundedKind(KindEnvelopeExceeded) || IsBoundedKind(KindMalformed) {
		t.Fatal("expected exactly the two bounded kinds")
	}
}

func testTenantID() id.UUID { return id.MustParse("01994f64-6c00-7000-8000-000000000011") }

func TestCanonicalPayloadPreservesNumberPrecision(t *testing.T) {
	// An integer beyond 2^53 is material content: a float64 round-trip
	// would silently corrupt the sealed blob and its digest.
	big, err := CanonicalPayload(json.RawMessage(`{"id":9007199254740993}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(big) != `{"id":9007199254740993}` {
		t.Fatalf("expected the exact literal to survive canonicalization, got %s", big)
	}
	// 1 and 1.0 are materially different payloads: the digest must differ
	// (a collapsed digest would return a false replay instead of the
	// digest-conflict quarantine).
	one, _ := CanonicalPayload(json.RawMessage(`{"n":1}`))
	oneDot, _ := CanonicalPayload(json.RawMessage(`{"n":1.0}`))
	if PayloadDigest(one) == PayloadDigest(oneDot) {
		t.Fatal("expected 1 and 1.0 to keep distinct material identities")
	}
}
