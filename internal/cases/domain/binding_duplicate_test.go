package domain

import (
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

func TestBindingGuards(t *testing.T) {
	if err := GuardBindChannel(BindingMethodConsentHandoff, "клиент продолжит в WhatsApp", "Иван"); err != nil {
		t.Fatalf("valid consent binding refused: %v", err)
	}
	if err := GuardBindChannel(BindingMethodConsentHandoff, "", "Иван"); err == nil {
		t.Fatalf("consent without purpose must refuse")
	}
	if err := GuardBindChannel("auto", "цель", "Иван"); err == nil {
		t.Fatalf("unknown method must refuse")
	}
	if err := GuardBindChannel(BindingMethodConsentHandoff, "цель", ""); err == nil {
		t.Fatalf("empty subject must refuse")
	}
	if err := GuardUnlinkChannel(""); err == nil {
		t.Fatalf("empty reversal reason must refuse")
	}
	if err := GuardUnlinkChannel("ошибочное связывание по неверно записанному телефону"); err != nil {
		t.Fatalf("valid reason refused: %v", err)
	}
}

func TestBindingActivityDerivation(t *testing.T) {
	bindingID := id.MustParse("01994f64-6c00-7000-8000-000000000001")
	otherID := id.MustParse("01994f64-6c00-7000-8000-000000000002")
	bindings := []ChannelBinding{
		{BindingID: bindingID, Method: BindingMethodConsentHandoff},
		{BindingID: otherID, Method: BindingMethodDuplicateReview},
	}
	if active := ActiveBindings(bindings, nil); len(active) != 2 {
		t.Fatalf("no reversals: both active")
	}
	reversals := []BindingReversal{{BindingID: bindingID, Reason: "ошибка"}}
	if active := ActiveBindings(bindings, reversals); len(active) != 1 || active[0].BindingID != otherID {
		t.Fatalf("the reversed binding must drop out: %+v", active)
	}
	if ReversalOf(reversals, otherID) != nil {
		t.Fatalf("no reversal of the second binding")
	}
}

func TestBindingClientConfirmation(t *testing.T) {
	for _, method := range BindingMethods {
		text := BindingClientConfirmation(method)
		if text == "" {
			t.Fatalf("method %s has no confirmation text", method)
		}
		for _, leak := range []string{bindingIDText, "conversation", "session"} {
			if containsWord(text, leak) {
				t.Fatalf("confirmation of %s leaks the internal identifier %q", method, leak)
			}
		}
	}
}

const bindingIDText = "01994f64"

func containsWord(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestDuplicateDictionaryAndNormalization(t *testing.T) {
	if IdentifierKind("phone") != IdentifierPhone || IdentifierKind("автомобиль") != "" {
		t.Fatalf("identifier dictionary broken")
	}
	phone, err := NormalizeIdentifier(IdentifierPhone, "+7 (912) 345-67-89")
	if err != nil || phone != "79123456789" {
		t.Fatalf("phone normalization = %q, %v", phone, err)
	}
	if _, err := NormalizeIdentifier(IdentifierPhone, "123"); err == nil {
		t.Fatalf("short phone must refuse")
	}
	email, err := NormalizeIdentifier(IdentifierEmail, "  Ivan@Mail.RU ")
	if err != nil || email != "ivan@mail.ru" {
		t.Fatalf("email normalization = %q, %v", email, err)
	}
	order, err := NormalizeIdentifier(IdentifierOrderNumber, " ABC-123 ")
	if err != nil || order != "abc-123" {
		t.Fatalf("order normalization = %q, %v", order, err)
	}
}

func TestMaskIdentifier(t *testing.T) {
	if mask := MaskIdentifier("79123456789"); mask != "•••••••••89" {
		t.Fatalf("mask = %q", mask)
	}
	if mask := MaskIdentifier("ab"); mask != "••" {
		t.Fatalf("short value masks completely: %q", mask)
	}
}

func TestIdentifierTokenKeyed(t *testing.T) {
	keyA := []byte("0123456789abcdef0123456789abcdef")
	tokenA := IdentifierToken(keyA, IdentifierPhone, "79123456789")
	if len(tokenA) != 64 {
		t.Fatalf("token must be 64 hex chars")
	}
	// The same key + value matches deterministically.
	if IdentifierToken(keyA, IdentifierPhone, "79123456789") != tokenA {
		t.Fatalf("token not deterministic")
	}
	// Another key does not match (the keyed discipline of AD-10).
	keyB := []byte("fedcba9876543210fedcba9876543210")
	if IdentifierToken(keyB, IdentifierPhone, "79123456789") == tokenA {
		t.Fatalf("token must depend on the key")
	}
	// Another value does not match.
	if IdentifierToken(keyA, IdentifierPhone, "79123456790") == tokenA {
		t.Fatalf("token must depend on the value")
	}
	// Another kind does not match.
	if IdentifierToken(keyA, IdentifierEmail, "79123456789") == tokenA {
		t.Fatalf("token must depend on the kind")
	}
}

func TestIdentifierStringValue(t *testing.T) {
	if raw, ok := IdentifierStringValue([]byte(`"79123456789"`)); !ok || raw != "79123456789" {
		t.Fatalf("string value must extract")
	}
	if _, ok := IdentifierStringValue([]byte(`{"value":"79123456789"}`)); ok {
		t.Fatalf("object value must not extract")
	}
	if _, ok := IdentifierStringValue([]byte(`12345`)); ok {
		t.Fatalf("number value must not extract")
	}
}

func TestDuplicateResolutionDerivation(t *testing.T) {
	candidate := id.MustParse("01994f64-6c00-7000-8000-00000000000a")
	if decision := ResolvedDecision(nil, candidate); decision != "" {
		t.Fatalf("no resolutions: empty decision")
	}
	resolutions := []DuplicateResolution{{CandidateID: candidate, Decision: DuplicateDecisionSameClient}}
	if decision := ResolvedDecision(resolutions, candidate); decision != DuplicateDecisionSameClient {
		t.Fatalf("resolved decision = %q", decision)
	}
}

func TestGuardResolveDuplicate(t *testing.T) {
	if err := GuardResolveDuplicate(DuplicateDecisionSeparate, "телефоны разных операторов"); err != nil {
		t.Fatalf("valid resolution refused: %v", err)
	}
	if err := GuardResolveDuplicate("merge", "нота"); err == nil {
		t.Fatalf("unknown decision must refuse (auto-merge is not a class)")
	}
	if err := GuardResolveDuplicate(DuplicateDecisionSameClient, ""); err == nil {
		t.Fatalf("resolution without the basis note must refuse")
	}
	if err := GuardResolveDuplicate(DuplicateDecisionSameClient, "4815162342"); err == nil {
		t.Fatalf("bare numeric note must refuse")
	}
}

func TestNewIdentifierKey(t *testing.T) {
	key, err := NewIdentifierKey()
	if err != nil || len(key) != 32 {
		t.Fatalf("identifier key must be 32 bytes: %d, %v", len(key), err)
	}
	other, _ := NewIdentifierKey()
	if string(key) == string(other) {
		t.Fatalf("identifier keys must be random")
	}
}

var _ = time.Now
