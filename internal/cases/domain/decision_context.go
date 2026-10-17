package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The decision-ready context and the operator surface (Story 2.14, first
// slice): the dictionaries, the read-time block assembly, the staleness
// derivation and the record guards of the AI summary
// ---------------------------------------------------------------------------
//
// The context is a READ-ONLY assembly of the existing derivations: no
// ladder, gate or dictionary of 2.1..2.13 is extended, and the single new
// record of the slice is the append-only AI summary. The summary kind
// dictionary carries the ONE word (ai_summary): operator summaries are
// deliberately absent — the operator writes the existing notes of 2.3, the
// summary is the producer-side record the machine gates (the AI Runtime
// itself is outside the contour — AD-7; the tests feed the material
// directly, the 2.6/2.8 precedent).
//
// The claim discipline is the anti-hallucination probe of the slice (the
// span honesty precedent of 2.8): the machine NEVER checks the truth of a
// claim — the machine checks the STRUCTURE. Every claim either carries a
// reference into the case material (a fact_version or an observation row)
// or is honestly marked disputed, and the claim text must be present in
// the summary text BYTE-FOR-BYTE (an assertion that is not a line of the
// summary is a fabrication the guard refuses — the claim is a span, never
// a free paraphrase). The refs are guarded by the store (the rows must
// belong to THIS case — step-02); the watermark is the server truth taken
// under the locks (the request never carries it — the spec pins it).
//
// The staleness is the read-time comparison of the summary watermark with
// the CURRENT material counters: the word actual|stale plus the honest
// counters («новых событий: N; новых версий фактов: N; владелец сменился;
// режим сменился»). A stale summary never mutates — the sources render
// beside it, the badge tells the truth (AC-6, AD-8: the summary does not
// replace the sources).
//
// The recommendation is a PROJECTION, never an authority: the plan words
// of the last policy outcome with the reason word, the mandatory checks
// (the gate dictionary words of the plan), the deadline of the open
// handoff round and the constraint words of the effective mode. Executing
// any other permitted command is the honest deviation — every command
// walks its own ladder and writes its own record (AC-7).

// ---------------------------------------------------------------------------
// The summary kind dictionary (the single word of the slice)
// ---------------------------------------------------------------------------

// SummaryKindAI is the ONLY kind word of the slice: the producer-side AI
// summary. Operator summaries are an honest Never (the notes of 2.3 are
// the operator surface; PRG forms do not exist — the record path is the
// machine JSON command).
const SummaryKindAI = "ai_summary"

// SummaryKinds is the full dictionary.
var SummaryKinds = []string{SummaryKindAI}

// SummaryKindInDictionary reports whether the kind word belongs to the
// fixed dictionary (the fail-closed basis of the record guard).
func SummaryKindInDictionary(kind string) bool {
	return inDictionary(SummaryKinds, kind)
}

// ---------------------------------------------------------------------------
// The context staleness dictionary (the honest words of the watermark
// comparison)
// ---------------------------------------------------------------------------

// The context staleness words: actual is the honest derivation of an
// unchanged material; stale is the honest derivation of the changed one
// (the counters tell WHAT moved — never a silent badge).
const (
	ContextStalenessActual = "actual"
	ContextStalenessStale  = "stale"
)

// ContextStalenessWords is the full dictionary.
var ContextStalenessWords = []string{ContextStalenessActual, ContextStalenessStale}

// ContextStalenessInDictionary reports whether the staleness word belongs
// to the fixed dictionary.
func ContextStalenessInDictionary(word string) bool {
	return inDictionary(ContextStalenessWords, word)
}

// ---------------------------------------------------------------------------
// The claim ref-type dictionary (the material the claims link to)
// ---------------------------------------------------------------------------

// The claim ref types: the words of the case material a claim may anchor
// to — the exact fact version (2.2) or the observation row (2.7). A claim
// without a link must be marked disputed (the honest boundary of AD-7:
// every material assertion is either sourced or honestly disputed).
const (
	ClaimRefFactVersion = "fact_version"
	ClaimRefObservation = "observation"
)

// ClaimRefTypes is the full dictionary.
var ClaimRefTypes = []string{ClaimRefFactVersion, ClaimRefObservation}

// ClaimRefTypeInDictionary reports whether the ref-type word belongs to
// the fixed dictionary (the closed write basis).
func ClaimRefTypeInDictionary(word string) bool {
	return inDictionary(ClaimRefTypes, word)
}

// ---------------------------------------------------------------------------
// The context fact-status dictionary (the render mirror of the work.go
// read-time derivations)
// ---------------------------------------------------------------------------

// The context fact statuses: the RENDER dictionary of the fact block —
// the mirror of the 2.2 read-time derivations (DeriveFactStatus): the
// unverified derivation renders as the unconfirmed word of this
// dictionary (the honest mirror mapping — the stored material never
// changes, only the context view names its own word); conflicting is the
// RENDER of the read-time conflict derivation (never a stored status).
const (
	ContextFactConfirmed   = "confirmed"
	ContextFactUnconfirmed = "unconfirmed"
	ContextFactConflicting = "conflicting"
)

// ContextFactStatuses is the full dictionary.
var ContextFactStatuses = []string{ContextFactConfirmed, ContextFactUnconfirmed, ContextFactConflicting}

// ContextFactStatusTexts is the honest text of every context fact-status
// word (the render material of the panel; the texts never enter the
// database or the outbox).
var ContextFactStatusTexts = map[string]string{
	ContextFactConfirmed:   "Подтверждено: точная версия подтверждена со ссылкой на источник.",
	ContextFactUnconfirmed: "Не подтверждено: значение без подтверждения — сверьте с источником.",
	ContextFactConflicting: "Противоречие: значения расходятся — разрешите конфликт подтверждением точной версии.",
}

// ContextFactStatusText resolves the honest text of one context fact
// status word (the empty string of an unknown word — the render layer
// only).
func ContextFactStatusText(word string) string {
	return ContextFactStatusTexts[word]
}

// ---------------------------------------------------------------------------
// The context commitment-status dictionary (the render words of the
// commitment block)
// ---------------------------------------------------------------------------

// The context commitment statuses: the RENDER dictionary of the
// commitment block. draft and offered mirror the version statuses of
// 2.9; confirmed is the derived render word — the current version
// carries a VALID exact-version confirmation (the 2.9 binding that still
// holds). The words travel, the derivation is read-time.
const (
	ContextCommitmentDraft     = "draft"
	ContextCommitmentOffered   = "offered"
	ContextCommitmentConfirmed = "confirmed"
)

// ContextCommitmentStatuses is the full dictionary.
var ContextCommitmentStatuses = []string{ContextCommitmentDraft, ContextCommitmentOffered, ContextCommitmentConfirmed}

// ---------------------------------------------------------------------------
// The honest marking texts (the render material of the section)
// ---------------------------------------------------------------------------

// The honest marking texts of the operator surface: the AI marking of
// the summary (AD-7/AD-8 — the summary never replaces the sources), the
// stale marking of the changed material (AC-6) and the honest absent
// receipt line of the commitments (the Action Gateway is Epic 3 — the
// line is the truth of this build, never a guess).
const (
	AIMarkingText     = "AI-производное представление — сверяйтесь с источниками ниже."
	ContextStaleText  = "Источники изменились после сводки."
	ReceiptAbsentText = "Квитанции нет — действие не выполнено."
)

// RecommendationProjectionText is the honest boundary text of the
// recommendation block: the projection never grants an authority (AC-7 —
// the deviation is any permitted command with its own record).
const RecommendationProjectionText = "Рекомендация — проекция слов плана: не команда и не полномочие; исполнение — только существующими командами, каждая со своей записью."

// ---------------------------------------------------------------------------
// The material limits (the fixed dictionary of the record discipline)
// ---------------------------------------------------------------------------

// The summary material limits: the text is 1..1000 runes, the claims are
// 1..32, the claim text is 1..200 runes and must be a byte-exact span of
// the summary text. The limits are the fixed dictionary — a runtime or
// tenant configuration never breeds them.
const (
	MinSummaryTextRunes = 1
	MaxSummaryTextRunes = 1000
	MinSummaryClaims    = 1
	MaxSummaryClaims    = 32
	MinClaimTextRunes   = 1
	MaxClaimTextRunes   = 200
)

// ---------------------------------------------------------------------------
// The claim, the watermark and the summary row
// ---------------------------------------------------------------------------

// SummaryClaim is one material assertion of the summary: the ref-type
// word with the ref id (the fact version or the observation row of THIS
// case — the store guards the belonging), the disputed mark (the honest
// boundary of an unsourced assertion) and the claim text — the span that
// must be present in the summary text byte-for-byte.
type SummaryClaim struct {
	RefType  string
	RefID    id.UUID
	Disputed bool
	Text     string
}

// SummaryWatermark is the server-truth snapshot of the case material at
// the moment the summary was recorded: the accepted-event count, the
// fact-version count, the owner epoch and the mode version. The request
// never carries it — the store takes it under the locks (the spec pins
// the server truth); the staleness derivation compares it with the
// CURRENT counters read-time.
type SummaryWatermark struct {
	EventCount       int64
	FactVersionCount int64
	OwnerEpoch       int64
	ModeVersion      int64
}

// RecordedSummary is one append-only summary row: the monotonic number (the
// store computes it under the locks — the requests never carry it), the
// kind word, the actor (the owner-Staffy of the moment — the NotOwner
// gate of the store), the text with the claims and the watermark. The
// row never mutates: a correction or a dispute is Story 2.15, a new
// summary of a new owner is a NEW row (the append-only versioning by
// number).
type RecordedSummary struct {
	TenantID  id.UUID
	CaseID    id.UUID
	SummaryNo int64
	Kind      string
	ActorID   id.UUID
	Text      string
	Claims    []SummaryClaim
	Watermark SummaryWatermark
	CreatedAt time.Time
}

// ---------------------------------------------------------------------------
// The staleness derivation (read-time, never a stored status)
// ---------------------------------------------------------------------------

// ContextStalenessView is the derived staleness of the last summary: the
// word (actual|stale) and the honest counters — what moved since the
// watermark (the new accepted events, the new fact versions, the owner
// change, the mode change). The derivation never mutates the summary.
type ContextStalenessView struct {
	Word            string
	NewEvents       int64
	NewFactVersions int64
	OwnerChanged    bool
	ModeChanged     bool
}

// DeriveContextStaleness derives the read-time staleness of the summary
// against the CURRENT watermark counters: every dimension equal → the
// honest actual; any dimension moved → the honest stale with the
// counters. A negative counter (the defensive backstop of a recycled
// material) clamps to zero — the derivation stays deterministic on any
// input order. No summary → the zero view (the section renders the
// material without the summary block).
func DeriveContextStaleness(summary *RecordedSummary, current SummaryWatermark) ContextStalenessView {
	if summary == nil {
		return ContextStalenessView{}
	}
	view := ContextStalenessView{
		NewEvents:       current.EventCount - summary.Watermark.EventCount,
		NewFactVersions: current.FactVersionCount - summary.Watermark.FactVersionCount,
		OwnerChanged:    current.OwnerEpoch != summary.Watermark.OwnerEpoch,
		ModeChanged:     current.ModeVersion != summary.Watermark.ModeVersion,
	}
	if view.NewEvents < 0 {
		view.NewEvents = 0
	}
	if view.NewFactVersions < 0 {
		view.NewFactVersions = 0
	}
	if view.NewEvents == 0 && view.NewFactVersions == 0 && !view.OwnerChanged && !view.ModeChanged {
		view.Word = ContextStalenessActual
		return view
	}
	view.Word = ContextStalenessStale
	return view
}

// StalenessCountersText renders the honest counters line of the stale
// badge: only the moved dimensions speak, never an empty part (the empty
// string of the actual word — the badge carries the word alone).
func (v ContextStalenessView) StalenessCountersText() string {
	if v.Word != ContextStalenessStale {
		return ""
	}
	parts := make([]string, 0, 4)
	if v.NewEvents > 0 {
		parts = append(parts, fmt.Sprintf("новых событий: %d", v.NewEvents))
	}
	if v.NewFactVersions > 0 {
		parts = append(parts, fmt.Sprintf("новых версий фактов: %d", v.NewFactVersions))
	}
	if v.OwnerChanged {
		parts = append(parts, "владелец сменился")
	}
	if v.ModeChanged {
		parts = append(parts, "режим сменился")
	}
	return strings.Join(parts, "; ")
}

// ---------------------------------------------------------------------------
// The record guard (the material bounds of the producer command)
// ---------------------------------------------------------------------------

// GuardRecordCaseSummary validates the record command material: the kind
// word belongs to the closed dictionary (the single word of the slice);
// the text and the claims are within the fixed limits; every claim is
// either linked (the ref-type word of the dictionary WITH the ref id) or
// honestly disputed; the claim text is a byte-exact span of the summary
// text (the anti-hallucination probe — an assertion that is not a line
// of the summary is a fabrication the guard refuses). The truth of a
// claim is NEVER checked here — the machine checks the structure, the
// truth is the producer's and the operator's responsibility (the honest
// boundary of AD-7/AD-8).
func GuardRecordCaseSummary(kind, text string, claims []SummaryClaim) error {
	if !SummaryKindInDictionary(kind) {
		return fmt.Errorf("вид сводки «%s» не входит в словарь (%s): операторские сводки не вводятся — слово продюсера одно", kind, joinQuoted(SummaryKinds))
	}
	runes := utf8.RuneCountInString(text)
	if runes < MinSummaryTextRunes || runes > MaxSummaryTextRunes {
		return fmt.Errorf("текст сводки — от %d до %d рун, получено %d", MinSummaryTextRunes, MaxSummaryTextRunes, runes)
	}
	if len(claims) < MinSummaryClaims || len(claims) > MaxSummaryClaims {
		return fmt.Errorf("число утверждений сводки — от %d до %d, получено %d", MinSummaryClaims, MaxSummaryClaims, len(claims))
	}
	for i, claim := range claims {
		claimRunes := utf8.RuneCountInString(claim.Text)
		if claimRunes < MinClaimTextRunes || claimRunes > MaxClaimTextRunes {
			return fmt.Errorf("текст утверждения №%d — от %d до %d рун, получено %d", i+1, MinClaimTextRunes, MaxClaimTextRunes, claimRunes)
		}
		if claim.RefType != "" && !ClaimRefTypeInDictionary(claim.RefType) {
			return fmt.Errorf("тип ссылки утверждения №%d «%s» не входит в словарь (%s)", i+1, claim.RefType, joinQuoted(ClaimRefTypes))
		}
		linked := ClaimRefTypeInDictionary(claim.RefType) && claim.RefID.Valid()
		if claim.RefID.Valid() && !linked {
			return fmt.Errorf("утверждение №%d несёт идентификатор без слова словаря: укажите тип ссылки (%s)", i+1, joinQuoted(ClaimRefTypes))
		}
		if !linked && !claim.Disputed {
			if ClaimRefTypeInDictionary(claim.RefType) {
				return fmt.Errorf("утверждение №%d несёт слово ссылки «%s» без идентификатора: ссылка несёт тип и id либо пометку disputed", i+1, claim.RefType)
			}
			return fmt.Errorf("утверждение №%d не связано и не помечено спорным: каждое утверждение несёт ссылку (%s) либо disputed=true", i+1, joinQuoted(ClaimRefTypes))
		}
		if !strings.Contains(text, claim.Text) {
			return fmt.Errorf("текст утверждения №%d отсутствует в тексте сводки байт-в-байт: утверждение обязано быть строкой сводки (анти-фантазийный зонд)", i+1)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// The context input material (ONE consistent snapshot of the store read)
// ---------------------------------------------------------------------------

// ReplyDeliveryMaterial is the delivery material of one client reply: the
// dispatch truth (the reply left — the machine acceptance) and the
// append-only evidence events (the delivery observations of the channel
// world). The acceptance, the delivery and the business completion are
// NEVER mixed: the state word is the delivery truth, the business
// completion has no word in this build (the Action Gateway is Epic 3 —
// the honest boundary of the line).
type ReplyDeliveryMaterial struct {
	ReplyID    id.UUID
	Dispatched bool
	Events     []ReplyDeliveryEvent
}

// DecisionContextInput is the full material of ONE consistent snapshot:
// the store reads every block in a single transaction (the advisory
// locks are NOT taken for the read — the consistent snapshot of one
// transaction is the honest boundary of the GET; the watermark counters
// of the staleness are computed by the store read). Every field is the
// RAW material of the existing stories; the derivation below resolves
// the current rows and the words — read-time, never a stored status.
type DecisionContextInput struct {
	Now                time.Time
	HandoffRequest     *HandoffRequest
	HandoffEvents      []HandoffEvent
	WorkItems          []WorkItemCurrent
	ModeTerms          []ModeTerm
	Verifications      []VerificationRecord
	OwnershipTerms     []OwnershipTerm
	FactVersions       []FactVersion
	FactConfirmations  []FactConfirmation
	Observations       []Observation
	Replies            []ReplyDeliveryMaterial
	CommitmentVersions []CommitmentVersion
	Confirmations      []ConfirmationRecord
	CurrentOwnerEpoch  int64
	CurrentLineage     string
	Accommodations     []AccommodationRecord
	Outcomes           []PolicyOutcome
	Summary            *RecordedSummary
	Watermark          SummaryWatermark
}

// ---------------------------------------------------------------------------
// The context view (the block model of the operator section)
// ---------------------------------------------------------------------------

// HandoffContextView is the open-handoff block: the reason word, the
// lifecycle state word, the anchor turn of the ask and the deadline of
// the live round (the assignment round when assigned — the request's own
// deadline before the assignment). The closed lifecycles never render
// here (the section appears with the open ask or the summary only).
type HandoffContextView struct {
	Present     bool
	Reason      string
	State       string
	TurnRef     id.UUID
	AckDeadline string
	DeadlineAt  time.Time
}

// WorkItemContextRow is one active work item of the context: the ladder
// state word of the moment (the terminal completions never render into
// the active context — resolved, cancelled, split and merged are the
// history, not the working material).
type WorkItemContextRow struct {
	WorkItemID id.UUID
	Title      string
	Type       string
	State      string
}

// ModeContextView is the conversation-mode block: the active ladder word
// with the version (the CAS truth the propose/exit commands consume).
type ModeContextView struct {
	Present     bool
	Mode        string
	ModeVersion int64
}

// AssuranceContextView is the identity/authority assurance block: the
// levels, the methods, the scope and the expiries of the existing 2.3
// derivation (the honest none of the absence — it never becomes a
// confirmed identity).
type AssuranceContextView struct {
	IdentityLevel      string
	IdentityMethod     string
	IdentityExpiresAt  time.Time
	AuthorityLevel     string
	AuthorityMethod    string
	AuthorityScope     string
	AuthorityExpiresAt time.Time
}

// OwnerContextView is the response-owner block: the kind, the ref and
// the epoch of the current ownership term (the 2.4 truth of the moment).
type OwnerContextView struct {
	Present   bool
	OwnerKind string
	OwnerRef  id.UUID
	Epoch     int64
}

// FactContextRow is one fact of the context: the status word of the
// render dictionary, the latest version number, the provenance (the
// source event, the channel) and the provider-observed time.
type FactContextRow struct {
	FactID      id.UUID
	MaterialKey string
	Status      string
	VersionNo   int64
	SourceType  string
	SourceID    string
	Channel     string
	ObservedAt  time.Time
}

// FactsContextView is the facts block: the three SEPARATE groups —
// confirmed, unconfirmed, conflicting (never one mixed list; the
// conflicting word is the render of the read-time derivation).
type FactsContextView struct {
	Confirmed   []FactContextRow
	Unconfirmed []FactContextRow
	Conflicting []FactContextRow
}

// ObservationContextRow is one current observation of the context: the
// owning-system word, the provider-observed time, the freshness word of
// the matrix window, the conflict-rule word, the evidence status and the
// provider technique (the projection, the external id and the external
// version — the render puts them behind the progressive disclosure).
type ObservationContextRow struct {
	WorkItemID      id.UUID
	MaterialKey     string
	OwningSystem    string
	ObservedAt      time.Time
	FreshnessWord   string
	ConflictRule    string
	EvidenceStatus  string
	Projection      string
	ExternalID      string
	ExternalVersion string
}

// DeliveryContextRow is one delivery line of the context: the derived
// delivery state word with the last evidence kind and its time — the
// acceptance, the delivery and the business completion never mix.
type DeliveryContextRow struct {
	ReplyID     id.UUID
	State       string
	LastKind    string
	LastEventAt time.Time
}

// CommitmentContextRow is one commitment of the context: the current
// version number, the render status word (draft, offered, confirmed —
// the confirmed word is the derivation of the valid exact-version
// confirmation), the validity time, the confirmation status word, the
// known and the honestly-unknown mandatory terms and the receipt line
// (the honest absent — the Action Gateway is Epic 3).
type CommitmentContextRow struct {
	CommitmentID       id.UUID
	VersionNo          int64
	Status             string
	ValidUntil         time.Time
	ConfirmationStatus string
	TermsKnown         []string
	TermsUnknown       []string
	ReceiptText        string
}

// UsualModalityRow is one usual-modality word of the mandatory material
// key (the pack data v0 of 2.11 — how the key is usually established).
type UsualModalityRow struct {
	MaterialKey string
	Word        string
}

// PreferencesContextView is the preferences block: the active
// accommodation keys (the read-time revision truth of 2.11), the active
// alternative-path words with the degraded permitted paths of the last
// outcome, and the usual-modality words of the mandatory keys. The words
// are visible BEFORE the first reply of the assignee (AC-4).
type PreferencesContextView struct {
	AccommodationKeys []string
	AlternativePaths  []string
	UsualModalities   []UsualModalityRow
}

// CheckContextRow is one mandatory check of the recommendation: the
// material key with its state word (the ask/reused words of the plan —
// what must be established or verified before the step).
type CheckContextRow struct {
	MaterialKey string
	State       string
}

// RecommendationContextView is the recommendation block: the plan step
// words of the last outcome with the reason word, the mandatory checks,
// the deadline of the open handoff round and the constraint words of the
// effective mode. The projection NEVER grants an authority — the honest
// boundary text renders beside it (AC-7).
type RecommendationContextView struct {
	Present     bool
	Steps       []string
	Reason      string
	Checks      []CheckContextRow
	AckDeadline string
	DeadlineAt  time.Time
	Mode        string
	ModeVersion int64
}

// ClaimView is one claim of the summary view: the ref-type word with the
// ref id, the disputed mark and the claim text (the byte-exact span of
// the summary — the view model the section renders as the link rows).
type ClaimView struct {
	RefType  string
	RefID    id.UUID
	Disputed bool
	Text     string
}

// SummaryContextView is the summary block: the last recorded row with
// the claim views and the read-time staleness (the word with the honest
// counters — a stale summary renders with the sources beside it, never
// as the actual one).
type SummaryContextView struct {
	Present   bool
	SummaryNo int64
	Kind      string
	Text      string
	Claims    []ClaimView
	Staleness ContextStalenessView
}

// DecisionContextView is the compiled context of the case: the twelve
// blocks of the operator section. The view is read-time — nothing is
// ever mutated by the assembly (the derivations stay derivations).
type DecisionContextView struct {
	Handoff        HandoffContextView
	WorkItems      []WorkItemContextRow
	Mode           ModeContextView
	Assurance      AssuranceContextView
	Owner          OwnerContextView
	Facts          FactsContextView
	Observations   []ObservationContextRow
	Deliveries     []DeliveryContextRow
	Commitments    []CommitmentContextRow
	Preferences    PreferencesContextView
	Recommendation RecommendationContextView
	Summary        SummaryContextView
}

// ---------------------------------------------------------------------------
// The read-time block assembly (the derivation of the whole view)
// ---------------------------------------------------------------------------

// DeriveDecisionContext derives the whole context view of ONE consistent
// snapshot: the open handoff block, the active work items, the
// conversation mode, the assurance, the owner, the three fact groups,
// the current observations, the delivery lines, the commitments, the
// preferences, the recommendation projection and the summary with its
// staleness. Every block reuses the EXISTING read-time derivations of
// 2.2..2.13 — the assembly never extends a ladder or a gate; the words
// of every block belong to their fixed dictionaries.
func DeriveDecisionContext(input DecisionContextInput) DecisionContextView {
	view := DecisionContextView{}
	now := input.Now

	// The open handoff block: the reason word, the anchor turn and the
	// deadline of the live round.
	var handoffState *HandoffStateView
	if input.HandoffRequest != nil {
		state := DeriveHandoffState(*input.HandoffRequest, input.HandoffEvents, now)
		if state.Active() {
			handoffState = &state
			view.Handoff = HandoffContextView{
				Present:     true,
				Reason:      state.Request.Reason,
				State:       state.State,
				TurnRef:     state.Request.TurnRef,
				AckDeadline: state.Request.AckDeadline,
				DeadlineAt:  handoffDeadlineOf(state),
			}
		}
	}

	// The active work items: the non-terminal ladder words.
	for _, item := range input.WorkItems {
		if TerminalWorkItemStates[item.State] {
			continue
		}
		view.WorkItems = append(view.WorkItems, WorkItemContextRow{
			WorkItemID: item.WorkItemID, Title: item.Title, Type: item.Type, State: item.State,
		})
	}
	sort.Slice(view.WorkItems, func(i, j int) bool {
		return view.WorkItems[i].WorkItemID < view.WorkItems[j].WorkItemID
	})

	// The conversation mode: the active term of the ladder.
	if active := ActiveConversationMode(input.ModeTerms); active != nil {
		view.Mode = ModeContextView{Present: true, Mode: active.Mode, ModeVersion: active.ModeVersion}
	}

	// The identity/authority assurance: the existing 2.3 derivation.
	assurance := DeriveAssuranceState(input.Verifications, now)
	view.Assurance = AssuranceContextView{
		IdentityLevel:      assurance.IdentityLevel,
		IdentityMethod:     assurance.IdentityMethod,
		IdentityExpiresAt:  assurance.IdentityExpiresAt,
		AuthorityLevel:     assurance.AuthorityLevel,
		AuthorityMethod:    assurance.AuthorityMethod,
		AuthorityScope:     assurance.AuthorityScope,
		AuthorityExpiresAt: assurance.AuthorityExpiresAt,
	}

	// The response owner: the term of the maximal epoch.
	if owner := CurrentOwnership(input.OwnershipTerms); owner != nil {
		view.Owner = OwnerContextView{
			Present: true, OwnerKind: owner.OwnerKind, OwnerRef: owner.OwnerRef, Epoch: owner.Epoch,
		}
	}

	view.Facts = deriveContextFacts(input)
	view.Observations = deriveContextObservations(input, now)
	view.Deliveries = deriveContextDeliveries(input)
	view.Commitments = deriveContextCommitments(input, now)

	// The last policy outcome of the case: the plan words of the
	// recommendation and the preference extensions.
	lastOutcome := lastPolicyOutcome(input.Outcomes)

	// The preferences: the active accommodation keys, the alternative
	// words of the last outcome, the usual-modality words of the
	// mandatory keys.
	preferences := DeriveCurrentAccommodations(input.Accommodations)
	view.Preferences.AccommodationKeys = preferences.ActiveKeys()
	if lastOutcome != nil {
		view.Preferences.AlternativePaths = append(append([]string{}, lastOutcome.ActiveAlternatives...), lastOutcome.DegradedPermittedPaths...)
		for _, mandatory := range lastOutcome.MandatoryFacts {
			view.Preferences.UsualModalities = append(view.Preferences.UsualModalities, UsualModalityRow{
				MaterialKey: mandatory.MaterialKey,
				Word:        UsualModalityOfMaterialKey(mandatory.MaterialKey),
			})
		}
	}

	// The recommendation: the projection of the plan words (never an
	// authority — the deviation is any permitted command).
	if lastOutcome != nil {
		view.Recommendation.Present = true
		view.Recommendation.Reason = lastOutcome.Reason
		for _, step := range lastOutcome.NextSteps {
			view.Recommendation.Steps = append(view.Recommendation.Steps, step.Step)
		}
		for _, mandatory := range lastOutcome.MandatoryFacts {
			view.Recommendation.Checks = append(view.Recommendation.Checks, CheckContextRow{
				MaterialKey: mandatory.MaterialKey, State: mandatory.State,
			})
		}
		if handoffState != nil {
			view.Recommendation.AckDeadline = handoffState.Request.AckDeadline
			view.Recommendation.DeadlineAt = handoffDeadlineOf(*handoffState)
		}
		view.Recommendation.Mode = view.Mode.Mode
		view.Recommendation.ModeVersion = view.Mode.ModeVersion
	}

	// The summary block: the last row with the claim views and the
	// read-time staleness against the current watermark counters.
	if input.Summary != nil {
		view.Summary = SummaryContextView{
			Present:   true,
			SummaryNo: input.Summary.SummaryNo,
			Kind:      input.Summary.Kind,
			Text:      input.Summary.Text,
			Staleness: DeriveContextStaleness(input.Summary, input.Watermark),
		}
		for _, claim := range input.Summary.Claims {
			view.Summary.Claims = append(view.Summary.Claims, ClaimView{
				RefType: claim.RefType, RefID: claim.RefID, Disputed: claim.Disputed, Text: claim.Text,
			})
		}
	}
	return view
}

// handoffDeadlineOf resolves the honest deadline of the live round: the
// assignment round deadline when a round exists, the request's own
// deadline before the assignment (the zero time of neither — the ask
// always carries its window).
func handoffDeadlineOf(state HandoffStateView) time.Time {
	if !state.DeadlineAt.IsZero() {
		return state.DeadlineAt
	}
	return state.Request.AckDeadlineAt
}

// contextFactStatusOf maps the work.go read-time derivation onto the
// context render word: the unverified derivation renders as the
// unconfirmed word of this dictionary (the honest mirror — the stored
// material never changes, only the view names its word).
func contextFactStatusOf(status string) string {
	switch status {
	case FactStatusConfirmed:
		return ContextFactConfirmed
	case FactStatusConflicting:
		return ContextFactConflicting
	default:
		return ContextFactUnconfirmed
	}
}

// deriveContextFacts derives the three fact groups: the fact material
// grouped by the fact id, the versions ordered by the version number
// (the DeriveFactStatus precondition), the status mapped onto the render
// word, the rows ordered by the material key (the canonical panel
// order).
func deriveContextFacts(input DecisionContextInput) FactsContextView {
	var view FactsContextView
	type factGroup struct {
		versions      []FactVersion
		confirmations []FactConfirmation
	}
	groups := map[id.UUID]*factGroup{}
	order := []id.UUID{}
	for _, version := range input.FactVersions {
		group := groups[version.FactID]
		if group == nil {
			group = &factGroup{}
			groups[version.FactID] = group
			order = append(order, version.FactID)
		}
		group.versions = append(group.versions, version)
	}
	for _, confirmation := range input.FactConfirmations {
		if group := groups[confirmation.FactID]; group != nil {
			group.confirmations = append(group.confirmations, confirmation)
		}
	}
	rows := make([]FactContextRow, 0, len(order))
	for _, factID := range order {
		group := groups[factID]
		sort.Slice(group.versions, func(i, j int) bool {
			return group.versions[i].VersionNo < group.versions[j].VersionNo
		})
		status, _ := DeriveFactStatus(group.versions, group.confirmations)
		latest := group.versions[len(group.versions)-1]
		rows = append(rows, FactContextRow{
			FactID:      factID,
			MaterialKey: latest.MaterialKey,
			Status:      contextFactStatusOf(status),
			VersionNo:   latest.VersionNo,
			SourceType:  latest.SourceType,
			SourceID:    latest.SourceID,
			Channel:     latest.Channel,
			ObservedAt:  latest.ObservedAt,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].MaterialKey < rows[j].MaterialKey })
	for _, row := range rows {
		switch row.Status {
		case ContextFactConfirmed:
			view.Confirmed = append(view.Confirmed, row)
		case ContextFactConflicting:
			view.Conflicting = append(view.Conflicting, row)
		default:
			view.Unconfirmed = append(view.Unconfirmed, row)
		}
	}
	return view
}

// deriveContextObservations derives the current observation rows: the
// material grouped by the work item and the material field (the current
// row is the maximal observation number — the 2.7 read-time truth), the
// freshness word from the matrix window (the age measured from the
// provider-observed time — never the receive time), the conflict-rule
// word of the matrix row, the evidence status and the provider
// technique.
func deriveContextObservations(input DecisionContextInput, now time.Time) []ObservationContextRow {
	type obsKey struct {
		WorkItemID  id.UUID
		MaterialKey string
	}
	current := map[obsKey]*Observation{}
	keys := []obsKey{}
	for i := range input.Observations {
		observation := &input.Observations[i]
		key := obsKey{WorkItemID: observation.WorkItemID, MaterialKey: observation.MaterialKey}
		existing := current[key]
		if existing == nil {
			current[key] = observation
			keys = append(keys, key)
			continue
		}
		if observation.ObservationNo > existing.ObservationNo {
			current[key] = observation
		}
	}
	rows := make([]ObservationContextRow, 0, len(keys))
	for _, key := range keys {
		observation := current[key]
		row := ObservationContextRow{
			WorkItemID:      observation.WorkItemID,
			MaterialKey:     observation.MaterialKey,
			OwningSystem:    observation.OwningSystem,
			ObservedAt:      observation.ObservedAt,
			EvidenceStatus:  observation.Status,
			Projection:      observation.Projection,
			ExternalID:      observation.ExternalID,
			ExternalVersion: observation.ExternalVersion,
		}
		if entry, ok := AuthorityEntryOf(observation.MaterialKey); ok {
			row.FreshnessWord = DeriveObservationState(observation.Status, now.Sub(observation.ObservedAt), entry.FreshnessSeconds)
			row.ConflictRule = entry.ConflictRule
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].MaterialKey != rows[j].MaterialKey {
			return rows[i].MaterialKey < rows[j].MaterialKey
		}
		return rows[i].WorkItemID < rows[j].WorkItemID
	})
	return rows
}

// deriveContextDeliveries derives the delivery lines: the existing 2.12
// derivation of every reply material (the state word with the last
// evidence kind — the acceptance, the delivery and the business
// completion never mix).
func deriveContextDeliveries(input DecisionContextInput) []DeliveryContextRow {
	rows := make([]DeliveryContextRow, 0, len(input.Replies))
	for _, reply := range input.Replies {
		state := DeriveReplyDeliveryState(reply.Dispatched, reply.Events)
		rows = append(rows, DeliveryContextRow{
			ReplyID:     reply.ReplyID,
			State:       state.State,
			LastKind:    state.LastKind,
			LastEventAt: state.LastEventAt,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ReplyID < rows[j].ReplyID })
	return rows
}

// deriveContextCommitments derives the commitment rows: the current
// version of every commitment (the maximal version number), the render
// status word (draft, offered, confirmed — the confirmed word is the
// derivation of the valid exact-version confirmation), the confirmation
// status word of the latest confirmation of the current version (the
// pins come from the version the confirmation binds; the currents from
// the input snapshot), the known and the honestly-unknown mandatory
// terms, and the honest absent receipt line.
func deriveContextCommitments(input DecisionContextInput, now time.Time) []CommitmentContextRow {
	type commitmentGroup struct {
		versions      []CommitmentVersion
		confirmations []ConfirmationRecord
	}
	groups := map[id.UUID]*commitmentGroup{}
	order := []id.UUID{}
	for _, version := range input.CommitmentVersions {
		group := groups[version.CommitmentID]
		if group == nil {
			group = &commitmentGroup{}
			groups[version.CommitmentID] = group
			order = append(order, version.CommitmentID)
		}
		group.versions = append(group.versions, version)
	}
	for _, confirmation := range input.Confirmations {
		if group := groups[confirmation.CommitmentID]; group != nil {
			group.confirmations = append(group.confirmations, confirmation)
		}
	}
	rows := make([]CommitmentContextRow, 0, len(order))
	for _, commitmentID := range order {
		group := groups[commitmentID]
		current := CurrentCommitmentVersion(group.versions)
		if current == nil {
			continue
		}
		row := CommitmentContextRow{
			CommitmentID: commitmentID,
			VersionNo:    current.VersionNo,
			Status:       ContextCommitmentDraft,
			ValidUntil:   current.ValidUntil,
			ReceiptText:  ReceiptAbsentText,
		}
		if current.Status == CommitmentStatusOffered {
			row.Status = ContextCommitmentOffered
		}
		row.TermsKnown, row.TermsUnknown = commitmentTermsOf(current)
		var latest *ConfirmationRecord
		for i := range group.confirmations {
			confirmation := &group.confirmations[i]
			if confirmation.VersionNo != current.VersionNo {
				continue
			}
			if latest == nil || confirmation.ConfirmedAt.After(latest.ConfirmedAt) {
				latest = confirmation
			}
		}
		if latest != nil {
			row.ConfirmationStatus = DeriveConfirmationValidity(ConfirmationValidityInput{
				Now: now, ExpiresAt: latest.ExpiresAt,
				MaxVersionNo: current.VersionNo, ConfirmedVersion: latest.VersionNo,
				OwnerEpochPinned: commitmentOwnerEpochOf(group.versions, latest.VersionNo), OwnerEpochCurrent: input.CurrentOwnerEpoch,
				LineagePinned: commitmentLineageOf(group.versions, latest.VersionNo), LineageCurrent: input.CurrentLineage,
			})
			if row.ConfirmationStatus == ConfirmationValidityValid {
				row.Status = ContextCommitmentConfirmed
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CommitmentID < rows[j].CommitmentID })
	return rows
}

// commitmentTermsOf splits the mandatory term words into the known and
// the honestly-unknown halves: the MaterialUnknowns of the version carry
// the holes (an unknown MANDATORY term blocks the readiness — the honest
// hole of 2.9), the known half is the dictionary order.
func commitmentTermsOf(version *CommitmentVersion) (known, unknown []string) {
	holes := map[string]bool{}
	for _, term := range version.MaterialUnknowns {
		holes[term] = true
	}
	for _, term := range CommitmentMandatoryTerms {
		if holes[term] {
			continue
		}
		known = append(known, term)
	}
	unknown = append([]string{}, version.MaterialUnknowns...)
	sort.Strings(unknown)
	return known, unknown
}

// commitmentOwnerEpochOf resolves the pinned owner epoch of the version
// the confirmation binds (the 2.9 pin — the current epoch moves past it
// honestly).
func commitmentOwnerEpochOf(versions []CommitmentVersion, versionNo int64) int64 {
	for _, version := range versions {
		if version.VersionNo == versionNo {
			return version.OwnerEpoch
		}
	}
	return 0
}

// commitmentLineageOf resolves the pinned release lineage word of the
// version the confirmation binds.
func commitmentLineageOf(versions []CommitmentVersion, versionNo int64) string {
	for _, version := range versions {
		if version.VersionNo == versionNo {
			return version.LineageWord
		}
	}
	return ""
}

// lastPolicyOutcome resolves the LAST outcome of the case material: the
// maximal creation time (the defensive tie-break by the outcome number —
// the deterministic order of the equal timestamps of the fixtures).
func lastPolicyOutcome(outcomes []PolicyOutcome) *PolicyOutcome {
	var last *PolicyOutcome
	for i := range outcomes {
		candidate := &outcomes[i]
		if last == nil || candidate.CreatedAt.After(last.CreatedAt) ||
			(candidate.CreatedAt.Equal(last.CreatedAt) && candidate.OutcomeNo > last.OutcomeNo) {
			last = candidate
		}
	}
	return last
}

// ---------------------------------------------------------------------------
// The outbox event (the safe canonical JSON)
// ---------------------------------------------------------------------------

// Outbox event types of the summary surface (the bounded vocabulary of
// the slice).
const (
	OutboxEventSummaryRecorded   = "case.summary_recorded"
	OutboxEventSummaryRecordedV1 = 1
)

// SummaryRecordedPayload is the safe canonical JSON of the summary
// record event: the identifiers (the tenant, the case, the summary
// number, the actor), the dictionary word (the kind) and the claim
// counters (the total, the linked, the disputed). The summary TEXT and
// the claims NEVER enter the envelope — the perimeter of the outbox
// stays identifiers-and-words (the «кандидат не покидает строку»
// precedent of 2.6/2.7/2.13).
type SummaryRecordedPayload struct {
	TenantID       id.UUID   `json:"tenant_id"`
	CaseID         id.UUID   `json:"case_id"`
	SummaryNo      int64     `json:"summary_no"`
	Kind           string    `json:"kind"`
	ClaimCount     int       `json:"claim_count"`
	LinkedClaims   int       `json:"linked_claims"`
	DisputedClaims int       `json:"disputed_claims"`
	ActorID        id.UUID   `json:"actor_id"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// ClaimCountersOf counts the linked and the disputed claims of the
// summary material (the outbox counters — the words travel, the text
// never).
func ClaimCountersOf(claims []SummaryClaim) (linked, disputed int) {
	for _, claim := range claims {
		if ClaimRefTypeInDictionary(claim.RefType) && claim.RefID.Valid() {
			linked++
		}
		if claim.Disputed {
			disputed++
		}
	}
	return linked, disputed
}

// DedupKeySummaryRecorded builds the dedup key of one summary record.
func DedupKeySummaryRecorded(caseID id.UUID, summaryNo int64) string {
	return fmt.Sprintf("case.case_summary:%s:%d", caseID.String(), summaryNo)
}

// SummaryRecordedPayloadJSON renders the safe canonical JSON.
func SummaryRecordedPayloadJSON(payload SummaryRecordedPayload) ([]byte, error) {
	return json.Marshal(payload)
}
