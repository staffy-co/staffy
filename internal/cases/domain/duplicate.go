package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Duplicate candidates over keyed identifier tokens (Story 2.3, first slice)
// ---------------------------------------------------------------------------
//
// The identifier facts of the fixed dictionary are tokenized into keyed
// HMAC tokens (the AD-10 rule: raw unsalted hashes of phone numbers, VINs,
// addresses or short customer text are FORBIDDEN — the token is
// HMAC-SHA256 under the per-tenant key). An exact token match across two
// OPEN cases creates one duplicate candidate — a task for the authorized
// human, never an automatic merge: the review card shows the minimal
// material (the masked value, the case numbers, the channel, the reason),
// never the protected content. The same-client resolution creates a
// duplicate-review binding atomically; until the decision (and after a
// separate decision) the two requests live apart.

// Identifier kinds: the fixed tokenization dictionary of the material keys
// of the facts. A fact with a key OUTSIDE this dictionary never tokenizes
// (the facts stay arbitrary by FR-10).
const (
	IdentifierPhone       = "phone"
	IdentifierEmail       = "email"
	IdentifierOrderNumber = "order_number"
)

// IdentifierKinds is the full dictionary.
var IdentifierKinds = []string{IdentifierPhone, IdentifierEmail, IdentifierOrderNumber}

// IdentifierKind normalizes the material key into the identifier kind: the
// material keys arrive normalized (trim+lower) — the identifier spelling
// must match the dictionary exactly.
func IdentifierKind(materialKey string) string {
	switch materialKey {
	case IdentifierPhone, IdentifierEmail, IdentifierOrderNumber:
		return materialKey
	}
	return ""
}

// NormalizeIdentifier normalizes one identifier value by its kind: the
// phone digits only (everything else strips), the email and the order
// number trim+lower. The rule is deterministic and unit-covered: the same
// spelling always produces the same normalized value.
func NormalizeIdentifier(kind, value string) (string, error) {
	if IdentifierKind(kind) == "" {
		return "", fmt.Errorf("вид идентификатора %s не входит в словарь токенизации", kind)
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("значение идентификатора пусто")
	}
	if utf8.RuneCountInString(trimmed) > MaxIdentifierValueRunes {
		return "", fmt.Errorf("значение идентификатора длиннее %d символов", MaxIdentifierValueRunes)
	}
	switch kind {
	case IdentifierPhone:
		digits := strings.Builder{}
		for _, r := range trimmed {
			if r >= '0' && r <= '9' {
				digits.WriteRune(r)
			}
		}
		if digits.Len() < 4 {
			return "", fmt.Errorf("номер телефона короче 4 цифр")
		}
		return digits.String(), nil
	default:
		return strings.ToLower(trimmed), nil
	}
}

// MaxIdentifierValueRunes bounds the raw identifier value (the dictionary
// of the slice).
const MaxIdentifierValueRunes = 64

// MaskIdentifier masks the normalized value for the review card: the LAST
// TWO runes survive, everything else is the bullet rune. A value of two
// runes or shorter masks completely — the minimal material is the reason of
// the match, never the value itself.
func MaskIdentifier(normalized string) string {
	runes := []rune(normalized)
	if len(runes) <= 2 {
		return strings.Repeat("•", len(runes))
	}
	return strings.Repeat("•", len(runes)-2) + string(runes[len(runes)-2:])
}

// IdentifierToken computes the keyed token of one normalized identifier:
// HMAC-SHA256(key, kind + ":" + normalized) in hex. The key is the
// per-tenant identifier key (32 bytes, INSERT-only, rotation honestly
// deferred); the token NEVER exposes the value — the same value under the
// same key matches, any other value does not.
func IdentifierToken(key []byte, kind, normalized string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(kind + ":" + normalized))
	return hex.EncodeToString(mac.Sum(nil))
}

// NewIdentifierKey generates the per-tenant key material (32 random bytes
// — the same size discipline as the payload seal keys).
func NewIdentifierKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// IdentifierStringValue extracts the string value of one canonical fact
// value: the identifier facts tokenize STRINGS only (an object or a number
// does not tokenize — the fact records as an ordinary 2-2 fact, without a
// token, honestly).
func IdentifierStringValue(canonicalValue []byte) (string, bool) {
	var decoded string
	if err := json.Unmarshal(canonicalValue, &decoded); err != nil {
		return "", false
	}
	return decoded, true
}

// The match reasons and the rule confidences of the dictionary: the exact
// token match is the only rule of the slice (high confidence, still a task
// for the human — the rule never merges anything).
const (
	MatchReasonExactIdentifier = "exact_identifier"
	RuleConfidenceHigh         = "high"
)

// MatchReasons is the full dictionary.
var MatchReasons = []string{MatchReasonExactIdentifier}

// RuleConfidences is the full dictionary.
var RuleConfidences = []string{RuleConfidenceHigh}

// Duplicate decisions: same_client (the reviewer binds the new conversation
// into the existing case through the duplicate-review method) and separate
// (the requests stay apart). The candidate never merges the cases itself.
const (
	DuplicateDecisionSameClient = "same_client"
	DuplicateDecisionSeparate   = "separate"
)

// DuplicateDecisions is the full dictionary.
var DuplicateDecisions = []string{DuplicateDecisionSameClient, DuplicateDecisionSeparate}

// DuplicateCandidate is one append-only task: the new case, the existing
// case, the matched token (the value NEVER lands here — the HMAC only), the
// material key, the match reason and the rule confidence.
type DuplicateCandidate struct {
	TenantID       id.UUID
	CandidateID    id.UUID
	CaseIDNew      id.UUID
	CaseIDExisting id.UUID
	Token          string
	MaterialKey    string
	MatchReason    string
	RuleConfidence string
	CreatedAt      time.Time
}

// DuplicateResolution is one append-only decision of the authorized human:
// which candidate, the decision, the evidence note (the basis the reviewer
// recorded) and the time.
type DuplicateResolution struct {
	TenantID     id.UUID
	ResolutionID id.UUID
	CandidateID  id.UUID
	Decision     string
	Note         string
	ActorID      id.UUID
	CreatedAt    time.Time
}

// ResolvedDecision resolves the decision of one candidate: empty while the
// review is open, otherwise the first resolution (the UNIQUE of the table
// makes it the only one).
func ResolvedDecision(resolutions []DuplicateResolution, candidateID id.UUID) string {
	for _, resolution := range resolutions {
		if resolution.CandidateID == candidateID {
			return resolution.Decision
		}
	}
	return ""
}

// GuardResolveDuplicate validates the resolution command: the decision
// dictionary and the evidence note (required — the reviewer records the
// basis, never a bare code).
func GuardResolveDuplicate(decision, note string) error {
	if !inDictionary(DuplicateDecisions, decision) {
		return fmt.Errorf("решение дубля %s не входит в словарь (%s)", decision, joinQuoted(DuplicateDecisions))
	}
	if strings.TrimSpace(note) == "" {
		return fmt.Errorf("решение дубля требует обоснование (нота ≤%d символов)", MaxNoteRunes)
	}
	if utf8.RuneCountInString(note) > MaxNoteRunes {
		return fmt.Errorf("нота решения дубля длиннее %d символов", MaxNoteRunes)
	}
	if noteIsBareCode(note) {
		return fmt.Errorf("нота решения не может быть чистым кодом: запишите обоснование словами")
	}
	return nil
}

// DuplicateReviewText is the honest guidance of the review card: the
// minimal material is the reason of the match; the protected content stays
// behind the review right of the ordinary surfaces.
const DuplicateReviewText = "Возможный дубликат: проверьте минимальные сведения и решите, тот ли это клиент. До решения обращения существуют раздельно."
