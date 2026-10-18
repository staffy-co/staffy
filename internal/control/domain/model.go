package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Module control — the AD-12/13/16 record core of Scoped Controls.
// This file owns the compile-time dictionaries of story 3.2 slice 1: the
// control mode ladder, the scope kind dictionary with the honest ref
// validation, the read-time window and the most-restrictive intersection
// with provenance. The words are frozen in code; runtime or tenant-config
// extension is NEVER allowed.
//
// Dictionary sources (mirrors below are pinned by mirrors_test against the
// living sources — drift turns the test red; the control domain does NOT
// import cases/action/identity domain packages):
//   - Control modes — ARCHITECTURE-SPINE AD-12 (exact words, the slash of
//     intake_only/no_promises is part of the word)
//   - Channels — cases profile dictionary (AllowedChannels/ChannelOrder)
//   - Policy outcome classes — cases 2-6 (with the degraded_mode of 2-12)
//   - Autonomy classes — PRD FR-42 (pinned against the action domain 3-1)
//
// Honesty decisions of this file:
//   - The rank of a word OUTSIDE the mode dictionary is the rank of paused:
//     a corrupted or unknown stored value NEVER expands the allowance —
//     reading is fail-closed (SafeReadMode / ControlModeRank).
//   - The intersection is the rank MAXIMUM: a narrower permission never
//     cancels a wider blocking (AD-12 most restrictive result).
//   - The window is read-time: a scheduled-future control does not act
//     yet, an expired control does not act anymore; expiry is idempotent
//     by construction — the effective mode is recomputed over the
//     REMAINING controls, never a stored snapshot.
//   - Absent controls mean accept: a route without controls is not
//     restricted (the honest empty state, not an error).
//   - Generations are monotonic on EVERY state change (set AND resume)
//     and never reset: a removal+re-installation cannot pretend to be the
//     old generation, so the future stale detection is never deceived.

var (
	// ErrInvalid is the umbrella for every rejected scoped control.
	ErrInvalid = errors.New("scoped control is invalid")
	// ErrGenerationConflict is the optimistic concurrency signal of the
	// Set/Resume mutations: the expected generation did not match. The
	// app layer attaches the actual state material (ConflictError) so the
	// second operator re-confirms over the CURRENT truth — the history
	// of BOTH attempts is preserved in the append-only audit (AC-11).
	ErrGenerationConflict = errors.New("scoped control generation conflict")
)

// RuleError keeps a precise, user-safe reason for a rejected control
// (precedent: connector domain 1-8, action domain 3-1). Every guard
// branch fails closed with one of these — dictionary refusals happen
// BEFORE the transaction, so a rejected mutation writes no audit row.
type RuleError struct{ Rule, Reason string }

func (e RuleError) Error() string { return e.Rule + ": " + e.Reason }
func (e RuleError) Unwrap() error { return ErrInvalid }

// ConflictError is the typed optimistic-concurrency refusal: the caller
// expected one generation, the stored row carries another. It embeds the
// CURRENT control material — the operator sees the actual state and
// re-confirms; nothing is silently overwritten (AC-11).
type ConflictError struct {
	ScopeKind string
	ScopeRef  string
	Expected  int64
	Current   ScopedControl
}

func (e ConflictError) Error() string {
	return "scoped control generation conflict: expected " + itoa(e.Expected) +
		", stored " + itoa(e.Current.Generation)
}

func (e ConflictError) Unwrap() error { return ErrGenerationConflict }

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	negative := v < 0
	if negative {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---------------------------------------------------------------------------
// The control mode dictionary (AD-12, exact words)
// ---------------------------------------------------------------------------

// Control mode words — exactly the AD-12 vocabulary. The slash of
// intake_only/no_promises is part of the word (the FR-32 wording is
// verbatim); renaming it to intake_only is forbidden by the mirror
// discipline — the contract test pins the exact string.
const (
	ControlModeAccept     = "accept"
	ControlModeIntakeOnly = "intake_only/no_promises"
	ControlModeHumanOnly  = "human_only"
	ControlModePaused     = "paused"
)

// ControlModes is the full dictionary. The order is the normative
// restrictiveness ladder: accept is the least restrictive, paused is the
// most safe. The intersection of several controls is the rank maximum —
// the most restrictive result (AD-12).
var ControlModes = []string{ControlModeAccept, ControlModeIntakeOnly, ControlModeHumanOnly, ControlModePaused}

// ControlModeRank returns the restrictiveness rank of a mode value:
// 0 = accept … 3 = paused. A value OUTSIDE the dictionary returns the
// rank of paused — an unknown or corrupted stored value never expands
// the allowance (fail-closed by reading). Callers that need to know
// whether the value was corrupted compare with WordIn separately.
func ControlModeRank(mode string) int {
	switch mode {
	case ControlModeAccept:
		return 0
	case ControlModeIntakeOnly:
		return 1
	case ControlModeHumanOnly:
		return 2
	default:
		// paused AND every out-of-dictionary value: the most safe rank.
		return 3
	}
}

// SafeReadMode returns the dictionary word a stored mode value READS as:
// an out-of-dictionary (corrupted) value reads as paused — the most safe
// applicable mode. The store keeps the raw value; the reading surfaces
// mark the corruption separately (the honest damage marker).
func SafeReadMode(mode string) string {
	if WordIn(ControlModes, mode) {
		return mode
	}
	return ControlModePaused
}

// ---------------------------------------------------------------------------
// The state dictionary
// ---------------------------------------------------------------------------

// Control state words. resume sets removed — the row is NEVER deleted:
// the history of the point stays readable, the audit stays complete, a
// re-installation continues the generation series.
const (
	ControlStateActive  = "active"
	ControlStateRemoved = "removed"
)

// ControlStates is the full dictionary.
var ControlStates = []string{ControlStateActive, ControlStateRemoved}

// ---------------------------------------------------------------------------
// The scope kind dictionary (seven words)
// ---------------------------------------------------------------------------

// Scope kind words — the closed dictionary of the addressable scopes.
const (
	ScopeKindTenant        = "tenant"
	ScopeKindConnector     = "connector"
	ScopeKindChannel       = "channel"
	ScopeKindService       = "service"
	ScopeKindPolicyOutcome = "policy_outcome"
	ScopeKindAutonomyClass = "autonomy_class"
	ScopeKindCampaign      = "campaign"
)

// ScopeKinds is the full dictionary. The order is also the deterministic
// tie-break order of equal-rank controls (kind order, then ref).
var ScopeKinds = []string{
	ScopeKindTenant,
	ScopeKindConnector,
	ScopeKindChannel,
	ScopeKindService,
	ScopeKindPolicyOutcome,
	ScopeKindAutonomyClass,
	ScopeKindCampaign,
}

// scopeKindRank returns the dictionary position of the kind (an
// out-of-dictionary kind sorts last — defensive only: rows are written
// through the guard, so a live kind is always a dictionary word).
func scopeKindRank(kind string) int {
	for i, word := range ScopeKinds {
		if word == kind {
			return i
		}
	}
	return len(ScopeKinds)
}

// ---------------------------------------------------------------------------
// The mirrors (pinned by mirrors_test against the source modules)
// ---------------------------------------------------------------------------

// ChannelWords mirrors the nine profile channels of the cases dictionary
// (ChannelOrder is the canonical order). Mirror only: the control domain
// never imports the cases domain; mirrors_test pins the slice.
var ChannelWords = []string{
	"phone",
	"web_chat",
	"telegram",
	"whatsapp",
	"email",
	"max",
	"jivo",
	"avito",
	"mobile_app_chat",
}

// PolicyOutcomeWords mirrors the policy outcome classes of 2-6 — the
// LIVING dictionary carries six words: degraded_mode joined in 2-12
// (the mirror follows the source, the pin catches every drift).
var PolicyOutcomeWords = []string{
	"commercial_answer",
	"informational_answer",
	"clarify",
	"mode_blocked",
	"degraded_mode",
	"fail_closed",
}

// AutonomyClassWords mirrors the FR-42 five words (pinned against the
// action domain 3-1 — the same vocabulary the action catalog guards).
var AutonomyClassWords = []string{
	"shadow",
	"copilot",
	"assisted",
	"auto",
	"exception",
}

// ---------------------------------------------------------------------------
// Scope ref validation (the honest boundaries)
// ---------------------------------------------------------------------------

const (
	// ScopeRefMinRunes is the lower bound of every scope ref shape.
	ScopeRefMinRunes = 1
	// ScopeRefMaxRunes is the upper bound of every scope ref shape.
	ScopeRefMaxRunes = 120
)

// validScopeRefShape reports whether ref fits the lexical shape
// [a-z0-9._-]{1,120} (rune count; the shape is pure ASCII so a non-ASCII
// ref fails the character loop).
func validScopeRefShape(ref string) bool {
	n := utf8.RuneCountInString(ref)
	if n < ScopeRefMinRunes || n > ScopeRefMaxRunes {
		return false
	}
	for _, r := range ref {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

// ValidScopeRef validates the scope reference for a kind — the LEXICAL
// half of the honesty; the LIVE half lives in the app layer:
//
//   - tenant: the ref must EQUAL the caller's own tenant id — an object
//     of a foreign tenant is materially unselectable (AC-1);
//   - connector: the shape only here; the app resolves the ref against
//     the LIVE registry of the tenant's connectors 1-8 through the
//     ConnectorRefResolver port — a non-resolving ref is refused with an
//     honest cause (fail-closed, step-02);
//   - channel: a word of the nine-channel dictionary (mirror + pin);
//   - policy_outcome: a word of the 2-6 outcome classes (mirror + pin);
//   - autonomy_class: a word of the FR-42 five (mirror + pin);
//   - service/campaign: the shape only — an honest boundary WITHOUT a
//     registry: the dictionaries arrive with the future modules, and the
//     impossibility of resolution NEVER expands the allowance. The
//     boundary is documented here and on the operator page.
func ValidScopeRef(kind, ref, tenantID string) error {
	if !WordIn(ScopeKinds, kind) {
		return RuleError{Rule: "scope_kind", Reason: "неизвестный вид скоупа: " + kind}
	}
	switch kind {
	case ScopeKindTenant:
		if tenantID == "" {
			return RuleError{Rule: "scope_ref", Reason: "скоуп tenant требует личность тенента"}
		}
		if ref != tenantID {
			// Material equality: the own tenant object only — a foreign
			// tenant is unselectable, not merely forbidden by policy.
			return RuleError{Rule: "scope_ref", Reason: "скоуп tenant допускает только собственный тенент"}
		}
		return nil
	case ScopeKindConnector, ScopeKindService, ScopeKindCampaign:
		if !validScopeRefShape(ref) {
			return RuleError{Rule: "scope_ref", Reason: "референс скоупа вне формы [a-z0-9._-], 1..120"}
		}
		return nil
	case ScopeKindChannel:
		if !WordIn(ChannelWords, ref) {
			return RuleError{Rule: "scope_ref", Reason: "неизвестный канал: " + ref}
		}
		return nil
	case ScopeKindPolicyOutcome:
		if !WordIn(PolicyOutcomeWords, ref) {
			return RuleError{Rule: "scope_ref", Reason: "неизвестный класс исхода: " + ref}
		}
		return nil
	case ScopeKindAutonomyClass:
		if !WordIn(AutonomyClassWords, ref) {
			return RuleError{Rule: "scope_ref", Reason: "неизвестный класс автономии: " + ref}
		}
		return nil
	}
	return RuleError{Rule: "scope_kind", Reason: "неизвестный вид скоупа: " + kind}
}

// ---------------------------------------------------------------------------
// The reason validation
// ---------------------------------------------------------------------------

const (
	// MaxReasonRunes is the upper bound of the mutation reason (the
	// operational-reason precedent of the cases commands: 500 runes).
	MaxReasonRunes = 500
	// MinReasonRunes is the lower bound after trimming: the reason is
	// MANDATORY on every mutation (set AND resume) — an empty reason is
	// a refusal, never a default.
	MinReasonRunes = 1
)

// ValidReason validates the mandatory mutation reason: trimmed of the
// surrounding space, 1..500 runes. The reason is the operator's stated
// cause — the audit and the provenance carry it verbatim.
func ValidReason(reason string) error {
	trimmed := strings.TrimSpace(reason)
	n := utf8.RuneCountInString(trimmed)
	if n < MinReasonRunes {
		return RuleError{Rule: "reason", Reason: "причина обязательна: пустая причина отклонена"}
	}
	if n > MaxReasonRunes {
		return RuleError{Rule: "reason", Reason: "причина длиннее 500 символов"}
	}
	return nil
}

// ---------------------------------------------------------------------------
// ScopedControl — the record
// ---------------------------------------------------------------------------

// ScopedControl is one control point: a scope pair, the mode, the stated
// reason, the acting operator, the previous mode, the monotonic
// generation, the read-time window and the state. The struct is a value:
// the store layer returns fresh copies (defensive copies on read).
type ScopedControl struct {
	ScopeKind string
	ScopeRef  string
	Mode      string
	Reason    string
	ActorID   string
	// PrevMode is the mode before the CURRENT change ("" on the first
	// set of a point; a dictionary word otherwise). The audit row carries
	// it — the ladder history of the point stays readable.
	PrevMode string
	// Generation is monotonic on EVERY state change (set AND resume) and
	// NEVER resets: a removal plus re-installation continues the series,
	// so the future stale detection is never deceived (AD-16).
	Generation int64
	// ScheduledStartAt opens the window (nil = active from the first
	// moment); ExpiresAt closes it (nil = no expiry). The window is
	// read-time semantics: [start, expires).
	ScheduledStartAt *time.Time
	ExpiresAt        *time.Time
	// State is active or removed. removed is a LIVE state of the record
	// (the history stays), not a deletion.
	State     string
	UpdatedAt time.Time
}

// Influencing reports whether the control acts at `now`: the state is
// active AND the read-time window covers now — scheduled_start <= now
// AND now < expires. A scheduled-future control does not act yet; an
// expired control does not act anymore; the expiry is idempotent by
// construction — nothing is materialized at the boundary moment.
func (c ScopedControl) Influencing(now time.Time) bool {
	if c.State != ControlStateActive {
		return false
	}
	if c.ScheduledStartAt != nil && now.Before(*c.ScheduledStartAt) {
		return false
	}
	if c.ExpiresAt != nil && !now.Before(*c.ExpiresAt) {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// The most-restrictive intersection with provenance
// ---------------------------------------------------------------------------

// EffectiveControl is the read result of the effective computation: the
// mode after the most-restrictive intersection, every influencing
// control (the provenance), the reason of the winning rule, the mode
// words that took part (the intersection, ladder order, deduplicated)
// and the honest corruption marker.
type EffectiveControl struct {
	Mode        string
	Influencing []ScopedControl
	// Reason is the stated reason of the MOST RESTRICTIVE control (the
	// winning rule). Empty when nothing restricts the route.
	Reason string
	// Intersection carries the mode words of every influencing control
	// (ladder order, deduplicated) — the words of the crossing of AC-3:
	// the reader sees WHAT met, not only the winner.
	Intersection []string
	// Corrupted marks that at least one influencing control stored an
	// out-of-dictionary mode value: such a value READS as paused (the
	// most safe applicable) — damage never expands the allowance, and
	// the marker keeps the reading honest about it.
	Corrupted bool
}

// ComputeEffective computes the most-restrictive intersection of the
// route-matching controls at `now`:
//
//   - the window filter first: only influencing controls take part
//     (state active AND the window covers now);
//   - the rank maximum wins — a narrower permission never cancels a
//     wider blocking; a corrupted mode value counts as paused;
//   - the provenance keeps EVERY influencing control, not only the
//     winner;
//   - absent controls mean accept: an empty or fully filtered-out list
//     returns Mode=accept with no provenance (the route is not
//     restricted — the honest empty state);
//   - the result is deterministic regardless of the input order: the
//     winner is picked by (rank desc, kind order asc, ref asc) — equal
//     re-reads of the same rows produce the same effective answer.
//
// The caller passes the controls matched to the route scope keys (the
// app layer query); ComputeEffective never re-queries. The returned
// Influencing slice is a fresh copy — mutating it never touches the
// caller's rows.
func ComputeEffective(now time.Time, controls []ScopedControl) EffectiveControl {
	influencing := make([]ScopedControl, 0, len(controls))
	for _, control := range controls {
		if control.Influencing(now) {
			influencing = append(influencing, control)
		}
	}
	if len(influencing) == 0 {
		return EffectiveControl{Mode: ControlModeAccept}
	}

	corrupted := false
	seen := make(map[string]bool)
	for _, control := range influencing {
		if !WordIn(ControlModes, control.Mode) {
			corrupted = true
		}
		seen[SafeReadMode(control.Mode)] = true
	}

	// The winner: rank maximum, then kind order, then ref — the total
	// order keeps equal-rank ties deterministic.
	winner := influencing[0]
	for _, control := range influencing[1:] {
		if beats(control, winner) {
			winner = control
		}
	}

	intersection := make([]string, 0, len(seen))
	for _, word := range ControlModes {
		if seen[word] {
			intersection = append(intersection, word)
		}
	}

	return EffectiveControl{
		Mode:         SafeReadMode(winner.Mode),
		Influencing:  influencing,
		Reason:       winner.Reason,
		Intersection: intersection,
		Corrupted:    corrupted,
	}
}

// beats reports whether control a wins over control b under the total
// winner order: a higher rank first (the most restrictive), then the
// smaller scope kind order, then the smaller ref.
func beats(a, b ScopedControl) bool {
	ra, rb := ControlModeRank(a.Mode), ControlModeRank(b.Mode)
	if ra != rb {
		return ra > rb
	}
	ka, kb := scopeKindRank(a.ScopeKind), scopeKindRank(b.ScopeKind)
	if ka != kb {
		return ka < kb
	}
	return a.ScopeRef < b.ScopeRef
}

// ---------------------------------------------------------------------------
// The mutation guard (dictionary refusals BEFORE the transaction)
// ---------------------------------------------------------------------------

// GuardValidateSet validates the Set mutation BEFORE any store call:
// every dictionary refusal happens here, so a rejected mutation writes
// no audit row and opens no transaction (the honest order: validation
// first, concurrency second).
func GuardValidateSet(kind, ref, tenantID, mode, reason string, scheduledStart, expires *time.Time) error {
	if err := ValidScopeRef(kind, ref, tenantID); err != nil {
		return err
	}
	if !WordIn(ControlModes, mode) {
		return RuleError{Rule: "mode", Reason: "неизвестный режим контроля: " + mode}
	}
	if err := ValidReason(reason); err != nil {
		return err
	}
	if scheduledStart != nil && expires != nil && !scheduledStart.Before(*expires) {
		// [start, expires) must be a non-empty forward window: an empty
		// or inverted window is refused — it would never influence and
		// would only corrupt the operator's mental model.
		return RuleError{Rule: "window", Reason: "окно действия пусто или перевёрнуто: scheduled_start обязан быть раньше expires"}
	}
	return nil
}

// WordIn reports whether w belongs to the closed dictionary.
func WordIn(dictionary []string, w string) bool {
	for _, word := range dictionary {
		if w == word {
			return true
		}
	}
	return false
}
