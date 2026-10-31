package domain

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 2.10 unit matrix: the link dictionaries, the issuance
// guards (the fixed bounds), the read-time state derivation with the
// spec priority order, the opaque capability-token codec (the
// round-trip, the strict refusals, the tenant part validation, the hash
// determinism) and the +1 fact channel word.

// continuationTestTenant builds a distinct deterministic v7 tenant id.
func continuationTestTenant() id.UUID {
	return id.MustParse("019a2f4c-2900-7000-8000-000000000010")
}

// continuationTestTime is the fixed derivation moment.
func continuationTestTime() time.Time {
	return time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
}

// validContinuationInput builds the valid baseline of the derivation.
func validContinuationInput() ContinuationLinkStateInput {
	return ContinuationLinkStateInput{
		Now:                 continuationTestTime(),
		ExpiresAt:           continuationTestTime().Add(30 * time.Minute),
		Revoked:             false,
		AcceptedCorrections: 0,
		ConsumptionLimit:    DefaultContinuationConsumptionLimit,
		OwnerEpochPinned:    2,
		OwnerEpochCurrent:   2,
	}
}

func TestContinuationDictionariesAreClosed(t *testing.T) {
	// No purpose, use-kind or state word appears outside its fixed
	// dictionary (the fail-closed basis of the slice).
	if len(ContinuationPurposes) != 1 || ContinuationPurposes[0] != ContinuationPurposeFactCorrection {
		t.Fatalf("the purpose dictionary carries one fact_correction word, got %v", ContinuationPurposes)
	}
	if len(ContinuationUseKinds) != 4 {
		t.Fatalf("the use-kind dictionary carries four words (the +1 vehicle_confirmation_submitted of story 4.2 — migrations/cases/00018 mirrors it), got %v", ContinuationUseKinds)
	}
	if !inDictionary(ContinuationUseKinds, ContinuationUseVehicleConfirmationSubmitted) {
		t.Fatalf("the story 4.2 word %q must be in the use-kind dictionary", ContinuationUseVehicleConfirmationSubmitted)
	}
	if len(ContinuationStateWords) != 6 {
		t.Fatalf("the state dictionary carries six words, got %v", ContinuationStateWords)
	}
	// The unknown word stays in the dictionary without a derivation
	// producer: it is the lookup-layer refusal of a token that finds
	// no row (the reserved-word precedent of the validity dictionary
	// of 2.9).
	if !inDictionary(ContinuationStateWords, ContinuationStateUnknown) {
		t.Fatalf("the lookup-layer word %q must stay in the state dictionary", ContinuationStateUnknown)
	}
	// The +1 fact channel word joins the provenance dictionary of 2.2.
	if !inDictionary(FactChannels, FactChannelProtectedContinuation) {
		t.Fatalf("the fact channel dictionary must carry the protected continuation word %q", FactChannelProtectedContinuation)
	}
	// The fixed bounds.
	if DefaultContinuationConsumptionLimit != 1 {
		t.Fatalf("the default consumption limit is one accepted submission, got %d", DefaultContinuationConsumptionLimit)
	}
}

func TestGuardContinuationPurposes(t *testing.T) {
	valid := []string{ContinuationPurposeFactCorrection}
	if err := GuardContinuationPurposes(valid); err != nil {
		t.Fatalf("the valid purpose set must pass, got %v", err)
	}
	empty := []string{}
	if err := GuardContinuationPurposes(empty); err == nil {
		t.Fatal("the empty purpose set must refuse")
	}
	unknown := []string{"read_commitment"}
	if err := GuardContinuationPurposes(unknown); err == nil {
		t.Fatal("a purpose word outside the dictionary must refuse")
	}
	duplicate := []string{ContinuationPurposeFactCorrection, ContinuationPurposeFactCorrection}
	if err := GuardContinuationPurposes(duplicate); err == nil {
		t.Fatal("a duplicated purpose word must refuse")
	}
}

func TestGuardContinuationTTL(t *testing.T) {
	for _, ttl := range []int{ContinuationTTLMinMinutes, 30, ContinuationTTLMaxMinutes} {
		if err := GuardContinuationTTL(ttl); err != nil {
			t.Fatalf("the ttl %d minutes must pass, got %v", ttl, err)
		}
	}
	for _, ttl := range []int{0, 4, ContinuationTTLMaxMinutes + 1, -1} {
		if err := GuardContinuationTTL(ttl); err == nil {
			t.Fatalf("the ttl %d minutes must refuse", ttl)
		}
	}
}

func TestGuardContinuationConsumptionLimit(t *testing.T) {
	for _, limit := range []int{ContinuationLimitMin, 5, ContinuationLimitMax} {
		if err := GuardContinuationConsumptionLimit(limit); err != nil {
			t.Fatalf("the limit %d must pass, got %v", limit, err)
		}
	}
	for _, limit := range []int{0, ContinuationLimitMax + 1, -1} {
		if err := GuardContinuationConsumptionLimit(limit); err == nil {
			t.Fatalf("the limit %d must refuse", limit)
		}
	}
}

func TestGuardContinuationTexts(t *testing.T) {
	question := "Какой пробег у автомобиля?"
	explanation := "Мы отправили вам ссылку, чтобы вы могли спокойно проверить данные обращения."
	if err := GuardContinuationTexts(question, explanation); err != nil {
		t.Fatalf("the valid texts must pass, got %v", err)
	}
	if err := GuardContinuationTexts("", explanation); err == nil {
		t.Fatal("the empty question must refuse")
	}
	if err := GuardContinuationTexts(question, ""); err == nil {
		t.Fatal("the empty explanation must refuse")
	}
	// The rune boundaries with multibyte text: 500/501 runes.
	boundaryQuestion := strings.Repeat("р", MaxContinuationQuestionRunes)
	if err := GuardContinuationTexts(boundaryQuestion, explanation); err != nil {
		t.Fatalf("the question of exactly %d runes must pass, got %v", MaxContinuationQuestionRunes, err)
	}
	overQuestion := strings.Repeat("р", MaxContinuationQuestionRunes+1)
	if err := GuardContinuationTexts(overQuestion, explanation); err == nil {
		t.Fatalf("the question of %d runes must refuse", MaxContinuationQuestionRunes+1)
	}
	boundaryExplanation := strings.Repeat("р", MaxContinuationExplanationRunes)
	if err := GuardContinuationTexts(question, boundaryExplanation); err != nil {
		t.Fatalf("the explanation of exactly %d runes must pass, got %v", MaxContinuationExplanationRunes, err)
	}
	if err := GuardContinuationTexts(question, strings.Repeat("р", MaxContinuationExplanationRunes+1)); err == nil {
		t.Fatalf("the explanation of %d runes must refuse", MaxContinuationExplanationRunes+1)
	}
}

func TestDeriveContinuationLinkState(t *testing.T) {
	// The baseline is valid.
	if got := DeriveContinuationLinkState(validContinuationInput()); got != ContinuationStateValid {
		t.Fatalf("the baseline must be valid, got %q", got)
	}

	// expired: the ttl passed — and the expiry DOMINATES every other
	// state of the spec order (the priority is the check order).
	expired := validContinuationInput()
	expired.Now = expired.ExpiresAt.Add(time.Second)
	for _, variant := range []ContinuationLinkStateInput{expired} {
		if got := DeriveContinuationLinkState(variant); got != ContinuationStateExpired {
			t.Fatalf("the passed expiry must derive expired, got %q", got)
		}
	}
	dominant := expired
	dominant.Revoked = true
	dominant.AcceptedCorrections = 5
	dominant.ConsumptionLimit = 1
	dominant.OwnerEpochCurrent = 7
	if got := DeriveContinuationLinkState(dominant); got != ContinuationStateExpired {
		t.Fatalf("the expired word dominates revoked/consumed/owner_stale, got %q", got)
	}

	// revoked: the revocation row exists — dominates consumed and
	// owner_stale.
	revoked := validContinuationInput()
	revoked.Revoked = true
	if got := DeriveContinuationLinkState(revoked); got != ContinuationStateRevoked {
		t.Fatalf("the revocation must derive revoked, got %q", got)
	}
	revokedConsumed := revoked
	revokedConsumed.AcceptedCorrections = 1
	revokedConsumed.ConsumptionLimit = 1
	if got := DeriveContinuationLinkState(revokedConsumed); got != ContinuationStateRevoked {
		t.Fatalf("revoked dominates consumed, got %q", got)
	}

	// consumed: the accepted corrections reached the limit — dominates
	// owner_stale.
	consumed := validContinuationInput()
	consumed.AcceptedCorrections = 1
	consumed.ConsumptionLimit = 1
	if got := DeriveContinuationLinkState(consumed); got != ContinuationStateConsumed {
		t.Fatalf("the exhausted limit must derive consumed, got %q", got)
	}
	consumedStale := consumed
	consumedStale.OwnerEpochCurrent = 3
	if got := DeriveContinuationLinkState(consumedStale); got != ContinuationStateConsumed {
		t.Fatalf("consumed dominates owner_stale, got %q", got)
	}
	// The boundary: the last allowed submission keeps the link valid.
	// At the exact expiry moment the link is still alive (Now.After is
	// strict).
	atExpiry := validContinuationInput()
	atExpiry.Now = atExpiry.ExpiresAt
	if got := DeriveContinuationLinkState(atExpiry); got != ContinuationStateValid {
		t.Fatalf("the exact expiry moment is still valid (strict After), got %q", got)
	}
	// The human/return events never consume: only the accepted
	// corrections count.
	withEvents := validContinuationInput()
	withEvents.AcceptedCorrections = 0
	withEvents.ConsumptionLimit = 1
	if got := DeriveContinuationLinkState(withEvents); got != ContinuationStateValid {
		t.Fatalf("the event-only usage keeps the link valid, got %q", got)
	}

	// owner_stale: the case owner epoch moved past the pin.
	stale := validContinuationInput()
	stale.OwnerEpochCurrent = stale.OwnerEpochPinned + 1
	if got := DeriveContinuationLinkState(stale); got != ContinuationStateOwnerStale {
		t.Fatalf("the moved owner epoch must derive owner_stale, got %q", got)
	}

	// unknown never comes from the derivation: it is the lookup-layer
	// word (asserted by the dictionary test above).
}

func TestContinuationTokenRoundTrip(t *testing.T) {
	tenant := continuationTestTenant()
	random := make([]byte, continuationTokenRandomBytes)
	for i := range random {
		random[i] = byte(i + 1)
	}
	token, err := EncodeContinuationToken(tenant, random)
	if err != nil {
		t.Fatalf("the token must encode, got %v", err)
	}
	if len(token) != 64 {
		t.Fatalf("the v1 token is 64 base64url characters, got %d (%q)", len(token), token)
	}
	if strings.ContainsAny(token, "+/=") {
		t.Fatalf("the token alphabet is base64url without padding, got %q", token)
	}
	parsedTenant, parsedRandom, err := ParseContinuationToken(token)
	if err != nil {
		t.Fatalf("the token must parse, got %v", err)
	}
	if parsedTenant != tenant {
		t.Fatalf("the round-trip tenant must match: %q != %q", parsedTenant, tenant)
	}
	if !bytes.Equal(parsedRandom, random) {
		t.Fatal("the round-trip random half must match byte-for-byte")
	}

	// The random halves differ, the tokens differ (the unguessability
	// basis).
	other := make([]byte, continuationTokenRandomBytes)
	other[0] = random[0] + 1
	otherToken, err := EncodeContinuationToken(tenant, other)
	if err != nil {
		t.Fatalf("the second token must encode, got %v", err)
	}
	if otherToken == token {
		t.Fatal("two random halves must produce two different tokens")
	}
}

func TestContinuationTokenIsOpaque(t *testing.T) {
	// The token carries the tenant and the random half ONLY: no case,
	// no work item, no participant material ever enters it.
	tenant := continuationTestTenant()
	random := make([]byte, continuationTokenRandomBytes)
	for i := range random {
		random[i] = byte(255 - i)
	}
	token, err := EncodeContinuationToken(tenant, random)
	if err != nil {
		t.Fatalf("the token must encode, got %v", err)
	}
	caseID := id.MustParse("019a2f4c-2900-7000-8000-000000000011")
	if strings.Contains(token, strings.ReplaceAll(caseID.String(), "-", "")) {
		t.Fatal("the token must not carry the case material")
	}
	if strings.Contains(token, "+7900") || strings.Contains(token, "клиент") {
		t.Fatal("the token must not carry the participant material")
	}
	// The decode result structurally carries two fields only.
	parsedTenant, parsedRandom, err := ParseContinuationToken(token)
	if err != nil {
		t.Fatalf("the token must parse, got %v", err)
	}
	if parsedTenant != tenant || len(parsedRandom) != continuationTokenRandomBytes {
		t.Fatal("the parse result carries the tenant and the random half only")
	}
}

func TestParseContinuationTokenRefuses(t *testing.T) {
	tenant := continuationTestTenant()
	random := make([]byte, continuationTokenRandomBytes)
	valid, err := EncodeContinuationToken(tenant, random)
	if err != nil {
		t.Fatalf("the token must encode, got %v", err)
	}

	// A non-base64url alphabet refuses.
	if _, _, err := ParseContinuationToken(valid + "!"); err == nil {
		t.Fatal("a token outside the base64url alphabet must refuse")
	}
	// A padded token refuses (the decoder is strict RawURL).
	if _, _, err := ParseContinuationToken(valid + "="); err == nil {
		t.Fatal("a padded token must refuse")
	}
	// The wrong total length refuses.
	short := base64.RawURLEncoding.EncodeToString(append([]byte{1, 2, 3}, random...))
	if _, _, err := ParseContinuationToken(short); err == nil {
		t.Fatal("a token of the wrong length must refuse")
	}
	// A tenant part that is not a valid UUIDv7 refuses (the random
	// half re-carrying the tenant layout: the v4-style variant bits).
	notV7 := make([]byte, continuationTokenTotalBytes)
	copy(notV7[:continuationTokenTenantBytes], []byte{0x01, 0x99, 0x4f, 0x64, 0x6c, 0x00, 0x40, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01})
	copy(notV7[continuationTokenTenantBytes:], random)
	if _, _, err := ParseContinuationToken(base64.RawURLEncoding.EncodeToString(notV7)); err == nil {
		t.Fatal("a tenant part outside the v7 layout must refuse")
	}
	// The all-zero tenant refuses.
	zeros := make([]byte, continuationTokenTotalBytes)
	copy(zeros[continuationTokenTenantBytes:], random)
	if _, _, err := ParseContinuationToken(base64.RawURLEncoding.EncodeToString(zeros)); err == nil {
		t.Fatal("the zero tenant must refuse")
	}
}

func TestContinuationTokenHash(t *testing.T) {
	first := make([]byte, continuationTokenRandomBytes)
	second := make([]byte, continuationTokenRandomBytes)
	second[31] = 1
	firstHash, err := ContinuationTokenHash(first)
	if err != nil {
		t.Fatalf("the hash must compute, got %v", err)
	}
	if len(firstHash) != 32 {
		t.Fatalf("the stored hash is 32 bytes, got %d", len(firstHash))
	}
	again, err := ContinuationTokenHash(first)
	if err != nil {
		t.Fatalf("the hash must recompute, got %v", err)
	}
	if !bytes.Equal(firstHash, again) {
		t.Fatal("the hash is deterministic")
	}
	secondHash, err := ContinuationTokenHash(second)
	if err != nil {
		t.Fatalf("the second hash must compute, got %v", err)
	}
	if bytes.Equal(firstHash, secondHash) {
		t.Fatal("two random halves must hash differently")
	}
	// The hash input validation.
	if _, err := ContinuationTokenHash(make([]byte, 31)); err == nil {
		t.Fatal("the hash of 31 bytes must refuse")
	}
	// The hash never equals the random half (SHA-256 is not identity).
	if bytes.Equal(firstHash, first) {
		t.Fatal("the hash must not leak the raw secret")
	}
}

func TestRandomContinuationSecret(t *testing.T) {
	first, err := RandomContinuationSecret()
	if err != nil {
		t.Fatalf("the secret must generate, got %v", err)
	}
	if len(first) != continuationTokenRandomBytes {
		t.Fatalf("the secret is 32 bytes, got %d", len(first))
	}
	second, err := RandomContinuationSecret()
	if err != nil {
		t.Fatalf("the second secret must generate, got %v", err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("two secrets must differ (the 256-bit entropy basis)")
	}
	// The encode refuses the wrong random length.
	if _, err := EncodeContinuationToken(continuationTestTenant(), first[:31]); err == nil {
		t.Fatal("the encode of 31 random bytes must refuse")
	}
}

// TestContinuationUseConsumptionWords pins the Story 4.2 limit
// accounting: the data submissions consume (the fact correction and the
// vehicle confirmation), the free events never do — every counting
// site reads the one dictionary word.
func TestContinuationUseConsumptionWords(t *testing.T) {
	if !ContinuationUseConsumesLimit(ContinuationUseFactCorrectionSubmitted) {
		t.Fatal("the fact correction consumes the limit")
	}
	if !ContinuationUseConsumesLimit(ContinuationUseVehicleConfirmationSubmitted) {
		t.Fatal("the vehicle confirmation of story 4.2 consumes the limit")
	}
	if ContinuationUseConsumesLimit(ContinuationUseHumanRequested) || ContinuationUseConsumesLimit(ContinuationUseReturnedToConversation) {
		t.Fatal("the free events never consume the limit")
	}
	if ContinuationUseConsumesLimit("supplier_form") {
		t.Fatal("an out-of-dictionary word never consumes anything (fail-closed)")
	}
}
