package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Identity verification and safe channel binding (Story 2.3, first slice)
// ---------------------------------------------------------------------------
//
// The verification records are append-only rows: the type of the check, the
// method, the CANONICAL outcome (the provider-specific code is mapped by the
// domain, never stored), the assurance level, the subject handle (a declared
// string — never a proof), the issuer, both times and the evidence. There is
// NO field for a one-time code, a password or a secret anywhere in the
// command or the record: the code-based methods require an event evidence,
// and a note that IS a bare numeric code is refused (the operator rewrites
// it as words). The assurance state of a case is a read-time derivation —
// the latest unexpired passed record per kind; nothing is ever updated.

// ---------------------------------------------------------------------------
// Kinds, methods and the channel eligibility dictionary
// ---------------------------------------------------------------------------

// Verification kinds: identity (who the participant is) and authority (the
// basis the participant may act for someone else). They are checked
// SEPARATELY: a confirmed identity never implies the right to see or change
// an object of another person.
const (
	VerificationKindIdentity  = "identity"
	VerificationKindAuthority = "authority"
)

// VerificationKinds is the full dictionary.
var VerificationKinds = []string{VerificationKindIdentity, VerificationKindAuthority}

// Verification methods (the fixed dictionary of the slice). The real
// verifiers (AG-4) will arrive with their own stories; the tenant-level
// configuration of the method lists is honestly deferred — the dictionary
// is the channel-modality one.
const (
	MethodKnowledgeCheck      = "knowledge_check"       // knowledge of the case details
	MethodKnownNumberCallback = "known_number_callback" // a callback to the number on file (voice)
	MethodKnownAddressReply   = "known_address_reply"   // a reply from the address on file (text)
	MethodMandateNote         = "mandate_note"          // the recorded basis of authority
)

// VerificationMethods is the full dictionary.
var VerificationMethods = []string{
	MethodKnowledgeCheck, MethodKnownNumberCallback, MethodKnownAddressReply, MethodMandateNote,
}

// MethodKind resolves the kind one method verifies: the first three are
// identity methods, the mandate note is the authority method.
func MethodKind(method string) string {
	switch method {
	case MethodKnowledgeCheck, MethodKnownNumberCallback, MethodKnownAddressReply:
		return VerificationKindIdentity
	case MethodMandateNote:
		return VerificationKindAuthority
	}
	return ""
}

// MethodAllowedForModality reports the channel eligibility: the callback
// method lives on the voice channels, the address reply on the text ones,
// the knowledge check and the mandate note are channel-agnostic.
func MethodAllowedForModality(method, modality string) bool {
	switch method {
	case MethodKnowledgeCheck, MethodMandateNote:
		return modality == ModalityVoice || modality == ModalityText
	case MethodKnownNumberCallback:
		return modality == ModalityVoice
	case MethodKnownAddressReply:
		return modality == ModalityText
	}
	return false
}

// MethodsOfModality lists the allowed methods of one modality (the honest
// step-up list of the gate refusal — the operator sees WHICH methods may
// raise the level of this conversation).
func MethodsOfModality(modality string) []string {
	methods := []string{}
	for _, method := range VerificationMethods {
		if MethodAllowedForModality(method, modality) {
			methods = append(methods, method)
		}
	}
	return methods
}

// Assurance levels: none (no passed record), basic (the knowledge check),
// verified (the callback, the address reply and the mandate). The binding
// gate requires verified identity.
const (
	AssuranceNone     = "none"
	AssuranceBasic    = "basic"
	AssuranceVerified = "verified"
)

// AssuranceLevels is the full dictionary.
var AssuranceLevels = []string{AssuranceNone, AssuranceBasic, AssuranceVerified}

// MethodAssuranceLevel resolves the level one PASSED method gives; an
// unknown method is none.
func MethodAssuranceLevel(method string) string {
	switch method {
	case MethodKnownNumberCallback, MethodKnownAddressReply, MethodMandateNote:
		return AssuranceVerified
	case MethodKnowledgeCheck:
		return AssuranceBasic
	}
	return AssuranceNone
}

// MethodValidity is the fixed validity dictionary: the binding-coupled
// verified methods live one hour, the knowledge check one day, the mandate
// thirty days.
func MethodValidity(method string) time.Duration {
	switch method {
	case MethodKnownNumberCallback, MethodKnownAddressReply:
		return time.Hour
	case MethodKnowledgeCheck:
		return 24 * time.Hour
	case MethodMandateNote:
		return 30 * 24 * time.Hour
	}
	return 0
}

// ---------------------------------------------------------------------------
// The provider vocabulary and the canonical outcome mapping
// ---------------------------------------------------------------------------

// Provider codes: the observed vocabulary of the synthetic verifier contour
// (the real verifiers of AG-4 will supply their own codes through the port —
// the mapping stays the domain's). ok means the check passed, no_match the
// subject did not match, expired the provider-side material had expired,
// cancelled the check was aborted, unclear an ambiguous result.
const (
	ProviderCodeOK        = "ok"
	ProviderCodeNoMatch   = "no_match"
	ProviderCodeExpired   = "expired"
	ProviderCodeCancelled = "cancelled"
	ProviderCodeUnclear   = "unclear"
)

// ProviderCodes is the full dictionary.
var ProviderCodes = []string{
	ProviderCodeOK, ProviderCodeNoMatch, ProviderCodeExpired, ProviderCodeCancelled, ProviderCodeUnclear,
}

// The canonical verification outcomes (what the system stores).
const (
	VerificationPassed    = "passed"
	VerificationFailed    = "failed"
	VerificationExpired   = "expired"
	VerificationCancelled = "cancelled"
	VerificationAmbiguous = "ambiguous"
)

// VerificationOutcomes is the full dictionary.
var VerificationOutcomes = []string{
	VerificationPassed, VerificationFailed, VerificationExpired, VerificationCancelled, VerificationAmbiguous,
}

// CanonicalOutcome maps one provider code to the canonical outcome: the
// provider-specific result NEVER lands in the record as-is.
func CanonicalOutcome(providerCode string) (string, error) {
	switch providerCode {
	case ProviderCodeOK:
		return VerificationPassed, nil
	case ProviderCodeNoMatch:
		return VerificationFailed, nil
	case ProviderCodeExpired:
		return VerificationExpired, nil
	case ProviderCodeCancelled:
		return VerificationCancelled, nil
	case ProviderCodeUnclear:
		return VerificationAmbiguous, nil
	}
	return "", fmt.Errorf("код верификатора %s не входит в словарь контура (%s)", providerCode, joinQuoted(ProviderCodes))
}

// The fixed material limits of the verification dictionary.
const (
	MaxSubjectHandleRunes  = 64
	MaxAuthorityScopeRunes = 200
	MaxPurposeRunes        = 200
	MaxBindingReasonRunes  = 200
)

// The step-up threshold (the fixed conservative dictionary, the load
// envelope precedent of 2-1): at least three failed or ambiguous outcomes
// of one case inside one hour block the verifications and the bindings; the
// window ages out by time alone — nothing is ever updated.
const (
	VerificationThresholdWindow = time.Hour
	VerificationThresholdLimit  = 3
)

// ---------------------------------------------------------------------------
// The verification record and the assurance derivation
// ---------------------------------------------------------------------------

// VerificationRecord is one append-only check: the kind, the method, the
// CANONICAL outcome, the assurance level (the method level when passed,
// none otherwise), the declared subject handle, the issuing operator, the
// evidence (an accepted event of the case or a note — exactly one), the
// authority scope (the authority kind only), both times and the computed
// expiry. The secret material of the check never enters this struct: there
// is no field for it.
type VerificationRecord struct {
	TenantID        id.UUID
	CaseID          id.UUID
	ConversationID  id.UUID
	VerificationID  id.UUID
	Kind            string
	Method          string
	Outcome         string
	AssuranceLevel  string
	SubjectHandle   string
	IssuerID        id.UUID
	EvidenceEventID id.UUID // zero when the note carries the words
	Note            string
	AuthorityScope  string // empty for the identity kind
	IssuedAt        time.Time
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

// AssuranceState is the read-time derivation of one case: the latest
// unexpired passed identity and authority records (the level, the method,
// the subject handle, the scope and the expiry of each), and the step-up
// flag of the failure window. The absence of a record is the honest none —
// it never becomes a confirmed identity.
type AssuranceState struct {
	IdentityLevel      string
	IdentityMethod     string
	IdentitySubject    string
	IdentityExpiresAt  time.Time
	AuthorityLevel     string
	AuthorityMethod    string
	AuthoritySubject   string
	AuthorityScope     string
	AuthorityExpiresAt time.Time
	StepUp             bool
	StepUpFailures     int
}

// DeriveAssuranceState derives the assurance state of one case from its
// records ordered by issued_at (the latest passed per kind wins; an expired
// record does not count but stays in the history) and the step-up window
// (the failed and ambiguous outcomes of the last hour).
func DeriveAssuranceState(records []VerificationRecord, now time.Time) AssuranceState {
	state := AssuranceState{IdentityLevel: AssuranceNone, AuthorityLevel: AssuranceNone}
	var identityLatest, authorityLatest *VerificationRecord
	for i := range records {
		record := &records[i]
		if record.Outcome != VerificationPassed {
			if record.IssuedAt.After(now.Add(-VerificationThresholdWindow)) {
				if record.Outcome == VerificationFailed || record.Outcome == VerificationAmbiguous {
					state.StepUpFailures++
				}
			}
			continue
		}
		if !record.ExpiresAt.After(now) {
			continue // expired: the history stays, the assurance does not
		}
		switch record.Kind {
		case VerificationKindIdentity:
			if identityLatest == nil || !record.IssuedAt.Before(identityLatest.IssuedAt) {
				identityLatest = record
			}
		case VerificationKindAuthority:
			if authorityLatest == nil || !record.IssuedAt.Before(authorityLatest.IssuedAt) {
				authorityLatest = record
			}
		}
	}
	if identityLatest != nil {
		state.IdentityLevel = identityLatest.AssuranceLevel
		state.IdentityMethod = identityLatest.Method
		state.IdentitySubject = identityLatest.SubjectHandle
		state.IdentityExpiresAt = identityLatest.ExpiresAt
	}
	if authorityLatest != nil {
		state.AuthorityLevel = authorityLatest.AssuranceLevel
		state.AuthorityMethod = authorityLatest.Method
		state.AuthoritySubject = authorityLatest.SubjectHandle
		state.AuthorityScope = authorityLatest.AuthorityScope
		state.AuthorityExpiresAt = authorityLatest.ExpiresAt
	}
	state.StepUp = state.StepUpFailures >= VerificationThresholdLimit
	return state
}

// BindingAllowed reports whether the assurance state passes the binding
// gate: the verified identity is required, and the on-behalf binding also
// requires an unexpired authority record with its scope. A confirmed
// identity alone never grants the right to act for another person.
func (s AssuranceState) BindingAllowed(onBehalf bool) bool {
	if s.IdentityLevel != AssuranceVerified {
		return false
	}
	if onBehalf {
		return s.AuthorityLevel == AssuranceVerified && s.AuthorityScope != ""
	}
	return true
}

// BindingGateText is the honest gate refusal: WHICH level is required and
// WHICH methods of the conversation may give it — never which identifier
// failed (the values never leak).
func BindingGateText(onBehalf bool) string {
	if onBehalf {
		return "Связывание от имени другого лица требует подтверждённую личность (verified) и действующие полномочия со скоупом."
	}
	return "Связывание каналов требует подтверждённую личность (verified): доступные методы повышают уровень."
}

// VerificationFallbackOptions is the fixed guidance dictionary of a failed
// or insufficient check (AC-4): the three safe options, never the values
// that did not match.
var VerificationFallbackOptions = []string{
	"Повторите проверку разрешённым способом.",
	"Продолжите обслуживание без защищённых данных.",
	"Передайте разговор человеку.",
}

// StepUpText is the honest step-up refusal of the threshold window.
const StepUpText = "Серия неудачных проверок: шаги проверки и связывания приостановлены на окно порога. Обратитесь к человеку или повторите позже."

// ---------------------------------------------------------------------------
// Command guards
// ---------------------------------------------------------------------------

// noteIsBareCode reports a note that IS a bare code: no letters at all
// (digits, punctuation, spaces) of length three or more. The operator must
// record the observed WORDS, never the one-time code itself. Letters of
// any alphabet count as words (the Cyrillic notes are the normal case).
func noteIsBareCode(note string) bool {
	if len(note) < 3 {
		return false
	}
	for _, r := range note {
		if unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// GuardStartVerification validates the command material: the kind, the
// method (of the kind), the provider code, the subject handle, the scope
// (the authority kind requires it) and the note (never a bare code).
func GuardStartVerification(kind, method, providerCode, subjectHandle, scope, note string) error {
	if !inDictionary(VerificationKinds, kind) {
		return fmt.Errorf("вид проверки %s не входит в словарь (%s)", kind, joinQuoted(VerificationKinds))
	}
	if !inDictionary(VerificationMethods, method) {
		return fmt.Errorf("метод проверки %s не входит в словарь (%s)", method, joinQuoted(VerificationMethods))
	}
	if MethodKind(method) != kind {
		return fmt.Errorf("метод %s проверяет %s, а не %s", method, MethodKind(method), kind)
	}
	if !inDictionary(ProviderCodes, providerCode) {
		return fmt.Errorf("код верификатора %s не входит в словарь контура", providerCode)
	}
	handle := strings.TrimSpace(subjectHandle)
	if handle == "" {
		return fmt.Errorf("объявленное имя участника обязательно")
	}
	if utf8.RuneCountInString(handle) > MaxSubjectHandleRunes {
		return fmt.Errorf("объявленное имя участника длиннее %d символов", MaxSubjectHandleRunes)
	}
	if utf8.RuneCountInString(note) > MaxNoteRunes {
		return fmt.Errorf("нота проверки длиннее %d символов", MaxNoteRunes)
	}
	if noteIsBareCode(note) {
		return fmt.Errorf("нота проверки не может быть чистым кодом: запишите наблюдение словами (одноразовый код не хранится)")
	}
	if kind == VerificationKindAuthority {
		if strings.TrimSpace(scope) == "" {
			return fmt.Errorf("проверка полномочий требует скоуп (на что распространяется полномочие)")
		}
		if utf8.RuneCountInString(scope) > MaxAuthorityScopeRunes {
			return fmt.Errorf("скоуп полномочия длиннее %d символов", MaxAuthorityScopeRunes)
		}
	} else if strings.TrimSpace(scope) != "" {
		return fmt.Errorf("скоуп относится к проверке полномочий, а не личности")
	}
	return nil
}

// joinQuoted renders the dictionary for the honest refusal texts.
func joinQuoted(values []string) string {
	out := ""
	for i, value := range values {
		if i > 0 {
			out += ", "
		}
		out += "«" + value + "»"
	}
	return out
}

// inDictionary reports the dictionary membership.
func inDictionary(dictionary []string, value string) bool {
	for _, entry := range dictionary {
		if entry == value {
			return true
		}
	}
	return false
}
