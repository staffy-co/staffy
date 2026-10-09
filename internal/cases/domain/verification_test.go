package domain

import (
	"testing"
	"time"
)

func TestVerificationDictionaries(t *testing.T) {
	if len(VerificationKinds) != 2 || VerificationKinds[0] != VerificationKindIdentity {
		t.Fatalf("kinds dictionary broken: %v", VerificationKinds)
	}
	for _, method := range VerificationMethods {
		if MethodKind(method) == "" {
			t.Fatalf("method %s has no kind", method)
		}
	}
	if MethodKind(MethodMandateNote) != VerificationKindAuthority {
		t.Fatalf("mandate must verify authority")
	}
	for _, method := range []string{MethodKnowledgeCheck, MethodKnownNumberCallback, MethodKnownAddressReply} {
		if MethodKind(method) != VerificationKindIdentity {
			t.Fatalf("method %s must verify identity", method)
		}
	}
}

func TestMethodModalityEligibility(t *testing.T) {
	cases := []struct {
		method, modality string
		allowed          bool
	}{
		{MethodKnowledgeCheck, ModalityVoice, true},
		{MethodKnowledgeCheck, ModalityText, true},
		{MethodKnownNumberCallback, ModalityVoice, true},
		{MethodKnownNumberCallback, ModalityText, false},
		{MethodKnownAddressReply, ModalityText, true},
		{MethodKnownAddressReply, ModalityVoice, false},
		{MethodMandateNote, ModalityVoice, true},
		{MethodMandateNote, ModalityText, true},
	}
	for _, row := range cases {
		if got := MethodAllowedForModality(row.method, row.modality); got != row.allowed {
			t.Fatalf("MethodAllowedForModality(%s, %s) = %v, want %v", row.method, row.modality, got, row.allowed)
		}
	}
	if len(MethodsOfModality(ModalityVoice)) != 3 {
		t.Fatalf("voice methods must list three entries: %v", MethodsOfModality(ModalityVoice))
	}
}

func TestMethodLevelsAndValidity(t *testing.T) {
	if MethodAssuranceLevel(MethodKnowledgeCheck) != AssuranceBasic {
		t.Fatalf("knowledge check must give basic")
	}
	for _, method := range []string{MethodKnownNumberCallback, MethodKnownAddressReply, MethodMandateNote} {
		if MethodAssuranceLevel(method) != AssuranceVerified {
			t.Fatalf("method %s must give verified", method)
		}
	}
	for _, method := range VerificationMethods {
		if MethodValidity(method) <= 0 {
			t.Fatalf("method %s has no validity", method)
		}
	}
	if MethodValidity(MethodKnownNumberCallback) != time.Hour {
		t.Fatalf("callback validity must be one hour")
	}
}

func TestCanonicalOutcomeMapping(t *testing.T) {
	pairs := map[string]string{
		ProviderCodeOK:        VerificationPassed,
		ProviderCodeNoMatch:   VerificationFailed,
		ProviderCodeExpired:   VerificationExpired,
		ProviderCodeCancelled: VerificationCancelled,
		ProviderCodeUnclear:   VerificationAmbiguous,
	}
	for code, want := range pairs {
		got, err := CanonicalOutcome(code)
		if err != nil || got != want {
			t.Fatalf("CanonicalOutcome(%s) = %v, %v; want %s", code, got, err, want)
		}
	}
	if _, err := CanonicalOutcome("definitely_ok"); err == nil {
		t.Fatalf("unknown provider code must refuse")
	}
}

func TestDeriveAssuranceState(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	// No records: the honest none, never a confirmed identity.
	if state := DeriveAssuranceState(nil, now); state.IdentityLevel != AssuranceNone || state.AuthorityLevel != AssuranceNone {
		t.Fatalf("empty state must be none/none")
	}
	// A passed callback gives verified identity for one hour.
	records := []VerificationRecord{{
		Kind: VerificationKindIdentity, Method: MethodKnownNumberCallback, Outcome: VerificationPassed,
		AssuranceLevel: AssuranceVerified, IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(59 * time.Minute),
	}}
	state := DeriveAssuranceState(records, now)
	if state.IdentityLevel != AssuranceVerified || state.IdentityMethod != MethodKnownNumberCallback {
		t.Fatalf("passed callback must derive verified: %+v", state)
	}
	if !state.BindingAllowed(false) {
		t.Fatalf("verified identity must allow the plain binding")
	}
	if state.BindingAllowed(true) {
		t.Fatalf("on-behalf binding requires the authority record")
	}
	// An expired record does not count.
	records[0].ExpiresAt = now.Add(-time.Second)
	if state := DeriveAssuranceState(records, now); state.IdentityLevel != AssuranceNone {
		t.Fatalf("expired record must not count")
	}
	// The knowledge check gives basic: the binding gate refuses.
	records[0].ExpiresAt = now.Add(59 * time.Minute)
	records[0].Method, records[0].AssuranceLevel = MethodKnowledgeCheck, AssuranceBasic
	if state := DeriveAssuranceState(records, now); state.BindingAllowed(false) {
		t.Fatalf("basic identity must not pass the binding gate")
	}
	// Authority with scope passes the on-behalf gate; identity alone never
	// implies authority (the identity record returns to the verified
	// method: the gate checks BOTH dimensions).
	records[0].Method, records[0].AssuranceLevel = MethodKnownNumberCallback, AssuranceVerified
	records = append(records, VerificationRecord{
		Kind: VerificationKindAuthority, Method: MethodMandateNote, Outcome: VerificationPassed,
		AssuranceLevel: AssuranceVerified, AuthorityScope: "orders of ACME", IssuedAt: now.Add(-time.Minute),
		ExpiresAt: now.Add(29 * 24 * time.Hour),
	})
	if state := DeriveAssuranceState(records, now); !state.BindingAllowed(true) {
		t.Fatalf("verified identity + authority with scope must allow the on-behalf binding: %+v", state)
	}
}

func TestDeriveAssuranceStepUpWindow(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	failed := func(issued time.Time) VerificationRecord {
		return VerificationRecord{Kind: VerificationKindIdentity, Outcome: VerificationFailed, IssuedAt: issued}
	}
	// Two failures: no step-up yet.
	if state := DeriveAssuranceState([]VerificationRecord{failed(now.Add(-10 * time.Minute)), failed(now.Add(-5 * time.Minute))}, now); state.StepUp {
		t.Fatalf("two failures must not step up")
	}
	// Three inside the window: the step-up is active.
	records := []VerificationRecord{failed(now.Add(-50 * time.Minute)), failed(now.Add(-10 * time.Minute)), failed(now.Add(-5 * time.Minute))}
	if state := DeriveAssuranceState(records, now); !state.StepUp {
		t.Fatalf("three window failures must step up")
	}
	// The oldest failure ages out: the window releases by time alone.
	if state := DeriveAssuranceState(records, now.Add(20*time.Minute)); state.StepUp {
		t.Fatalf("the window must age out: %d failures counted", state.StepUpFailures)
	}
	// Passed and ambiguous records: only failed and ambiguous count.
	passed := VerificationRecord{Kind: VerificationKindIdentity, Outcome: VerificationPassed, IssuedAt: now.Add(-5 * time.Minute), ExpiresAt: now.Add(time.Hour)}
	if state := DeriveAssuranceState([]VerificationRecord{failed(now.Add(-10 * time.Minute)), passed, failed(now.Add(-5 * time.Minute))}, now); state.StepUp {
		t.Fatalf("two failures + one pass must not step up")
	}
	ambiguous := VerificationRecord{Kind: VerificationKindIdentity, Outcome: VerificationAmbiguous, IssuedAt: now.Add(-2 * time.Minute)}
	if state := DeriveAssuranceState([]VerificationRecord{failed(now.Add(-10 * time.Minute)), failed(now.Add(-5 * time.Minute)), ambiguous}, now); !state.StepUp {
		t.Fatalf("ambiguous counts into the window")
	}
}

func TestGuardStartVerification(t *testing.T) {
	valid := GuardStartVerification(VerificationKindIdentity, MethodKnowledgeCheck, ProviderCodeOK, "позвонивший", "", "клиент назвал номер заказа")
	if valid != nil {
		t.Fatalf("valid command refused: %v", valid)
	}
	if err := GuardStartVerification("subscription", MethodKnowledgeCheck, ProviderCodeOK, "x", "", "нота"); err == nil {
		t.Fatalf("unknown kind must refuse")
	}
	if err := GuardStartVerification(VerificationKindIdentity, MethodMandateNote, ProviderCodeOK, "x", "", "нота"); err == nil {
		t.Fatalf("kind/method mismatch must refuse")
	}
	if err := GuardStartVerification(VerificationKindIdentity, MethodKnowledgeCheck, "maybe", "x", "", "нота"); err == nil {
		t.Fatalf("unknown provider code must refuse")
	}
	if err := GuardStartVerification(VerificationKindIdentity, MethodKnowledgeCheck, ProviderCodeOK, "", "", "нота"); err == nil {
		t.Fatalf("empty subject must refuse")
	}
	if err := GuardStartVerification(VerificationKindAuthority, MethodMandateNote, ProviderCodeOK, "x", "", "нота"); err == nil {
		t.Fatalf("authority without scope must refuse")
	}
	if err := GuardStartVerification(VerificationKindIdentity, MethodKnowledgeCheck, ProviderCodeOK, "x", "скоуп", "нота"); err == nil {
		t.Fatalf("identity with scope must refuse")
	}
	// The bare numeric code note: the one-time code never lands.
	if err := GuardStartVerification(VerificationKindIdentity, MethodKnowledgeCheck, ProviderCodeOK, "x", "", "4815162342"); err == nil {
		t.Fatalf("bare numeric note must refuse (the code is never stored)")
	}
	if err := GuardStartVerification(VerificationKindIdentity, MethodKnowledgeCheck, ProviderCodeOK, "x", "", "код 4815 — клиент назвал его словами нет"); err != nil {
		t.Fatalf("note with words must pass: %v", err)
	}
}
