package api

import (
	"context"
	"time"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 2.14: the decision context surface — the producer command of the
// AI summary record and the read-time context view of the assigned operator
// ---------------------------------------------------------------------------

// RecordCaseSummaryCommand is the producer command of ONE append-only AI
// summary row: the kind word of the single-word dictionary (ai_summary —
// operator summaries are an honest Never of the slice: the operator notes
// of 2.3 stay the operator surface), the text (1..1000 runes), the claims
// with the ref discipline (every claim either links a fact version or an
// observation row of THIS case or carries the honest disputed mark; the
// claim text is a byte-exact span of the summary text) and the idempotency
// key. The summary number and the watermark are the SERVER-side truth:
// the store derives the NEXT monotonic number and takes the watermark
// snapshot under the advisory locks — the request NEVER carries them (the
// request fields are the producer material only). The record is the act
// of the owner-Staffy of the moment: the store gate refuses a human
// owner with the honest NotOwner words (the summary waits for the return
// — a new owner writes a NEW row, never an edit).
type RecordCaseSummaryCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	Kind           string
	Text           string
	Claims         []casesDomain.SummaryClaim
	IdempotencyKey string
	Now            time.Time
}

// DecisionContextService is the narrow command-and-read surface of Story
// 2.14 (the HandoffService precedent: the composite Service is
// type-asserted, the test fakes of the other stories stay untouched).
// The read gate is review or configuration (the audit-only member gets
// the honest 403 of the scope words — the decision context is the
// operator working surface, not the audit material); the GET is the
// read-time derivation over ONE consistent snapshot, never a stored
// status.
type DecisionContextService interface {
	RecordCaseSummary(context.Context, RecordCaseSummaryCommand) (WorkOutcome, error)
	DecisionContext(ctx context.Context, actor domain.Principal, tenantID, caseID id.UUID) (DecisionContextView, error)
}

// ContextHandoffView is the open-handoff block of the context: the reason
// word with its honest text, the lifecycle state word with its text, the
// anchor turn of the ask and the deadline word of the live round.
type ContextHandoffView struct {
	Present         bool      `json:"present"`
	Reason          string    `json:"reason"`
	ReasonText      string    `json:"reason_text"`
	State           string    `json:"state"`
	StateText       string    `json:"state_text"`
	TurnRef         string    `json:"turn_ref"`
	AckDeadline     string    `json:"ack_deadline"`
	AckDeadlineText string    `json:"ack_deadline_text"`
	DeadlineAt      time.Time `json:"deadline_at"`
}

// ContextWorkItemRow is one active work item of the context: the ladder
// state word of the moment (the terminal completions never render into
// the active context — resolved, cancelled, split and merged are the
// history, not the working material).
type ContextWorkItemRow struct {
	WorkItemID string `json:"work_item_id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	State      string `json:"state"`
}

// ContextModeView is the conversation-mode block: the active ladder word
// with the version (the CAS truth the propose/exit commands consume).
type ContextModeView struct {
	Present     bool   `json:"present"`
	Mode        string `json:"mode"`
	ModeVersion int64  `json:"mode_version"`
}

// ContextAssuranceView is the identity/authority assurance block: the
// levels, the methods, the scope and the expiries of the 2.3 derivation
// (the honest none of the absence — it never becomes a confirmed
// identity).
type ContextAssuranceView struct {
	IdentityLevel      string    `json:"identity_level"`
	IdentityMethod     string    `json:"identity_method"`
	IdentityExpiresAt  time.Time `json:"identity_expires_at"`
	AuthorityLevel     string    `json:"authority_level"`
	AuthorityMethod    string    `json:"authority_method"`
	AuthorityScope     string    `json:"authority_scope"`
	AuthorityExpiresAt time.Time `json:"authority_expires_at"`
}

// ContextOwnerView is the response-owner block: the kind, the ref and the
// epoch of the current ownership term (the 2.4 truth of the moment).
type ContextOwnerView struct {
	Present   bool   `json:"present"`
	OwnerKind string `json:"owner_kind"`
	OwnerRef  string `json:"owner_ref"`
	Epoch     int64  `json:"epoch"`
}

// ContextFactRow is one fact of the context: the status word of the render
// dictionary with its honest text, the latest version number, the
// provenance (the source event, the channel) and the provider-observed
// time. The conflicting word is the render of the read-time derivation —
// never a stored status. The Story 2.15 fields carry the derived
// correction state of the fact (read-time, over the correction rows of
// the SAME consistent snapshot — never a stored status): the marker and
// the source line with the last correction number, the outcome word, the
// time, the byte-exact spans and the reason. No corrections is the zero
// state — the markers appear only with the material (the control branch
// renders byte-identically).
type ContextFactRow struct {
	FactID      string    `json:"fact_id"`
	MaterialKey string    `json:"material_key"`
	Status      string    `json:"status"`
	StatusText  string    `json:"status_text"`
	VersionNo   int64     `json:"version_no"`
	SourceType  string    `json:"source_type"`
	SourceID    string    `json:"source_id"`
	Channel     string    `json:"channel"`
	ObservedAt  time.Time `json:"observed_at"`
	// The Story 2.15 correction state of the fact row (read-time
	// derivation — the last correction of the fact speaks).
	HasCorrection        bool      `json:"has_correction"`
	CorrectionMarkerText string    `json:"correction_marker_text,omitempty"`
	CorrectionSourceText string    `json:"correction_source_text,omitempty"`
	CorrectionNo         int64     `json:"correction_no,omitempty"`
	CorrectionOutcome    string    `json:"correction_outcome,omitempty"`
	CorrectionAt         time.Time `json:"correction_at,omitempty"`
	CorrectionSpans      []string  `json:"correction_spans,omitempty"`
	CorrectionReason     string    `json:"correction_reason,omitempty"`
}

// ContextFactsView is the facts block: the three SEPARATE groups —
// confirmed, unconfirmed, conflicting (never one mixed list).
type ContextFactsView struct {
	Confirmed   []ContextFactRow `json:"confirmed"`
	Unconfirmed []ContextFactRow `json:"unconfirmed"`
	Conflicting []ContextFactRow `json:"conflicting"`
}

// ContextObservationRow is one current observation of the context: the
// owning-system word, the provider-observed time, the freshness word of
// the matrix window, the conflict-rule word, the evidence status and the
// provider technique (the render puts it behind the progressive
// disclosure).
type ContextObservationRow struct {
	WorkItemID      string    `json:"work_item_id"`
	MaterialKey     string    `json:"material_key"`
	OwningSystem    string    `json:"owning_system"`
	ObservedAt      time.Time `json:"observed_at"`
	FreshnessWord   string    `json:"freshness"`
	ConflictRule    string    `json:"conflict_rule"`
	EvidenceStatus  string    `json:"evidence_status"`
	Projection      string    `json:"projection"`
	ExternalID      string    `json:"external_id"`
	ExternalVersion string    `json:"external_version"`
}

// ContextDeliveryRow is one delivery line of the context: the derived
// delivery state word with the last evidence kind and its time — the
// acceptance, the delivery and the business completion never mix.
type ContextDeliveryRow struct {
	ReplyID     string    `json:"reply_id"`
	State       string    `json:"state"`
	LastKind    string    `json:"last_kind,omitempty"`
	LastEventAt time.Time `json:"last_event_at"`
}

// ContextCommitmentRow is one commitment of the context: the current
// version number, the render status word (draft, offered, confirmed —
// the confirmed word is the derivation of the valid exact-version
// confirmation), the validity time, the confirmation status word, the
// known and the honestly-unknown mandatory terms and the receipt line
// (the honest absent — the Action Gateway is Epic 3).
type ContextCommitmentRow struct {
	CommitmentID       string    `json:"commitment_id"`
	VersionNo          int64     `json:"version_no"`
	Status             string    `json:"status"`
	ValidUntil         time.Time `json:"valid_until"`
	ConfirmationStatus string    `json:"confirmation_status,omitempty"`
	TermsKnown         []string  `json:"terms_known"`
	TermsUnknown       []string  `json:"terms_unknown"`
	ReceiptText        string    `json:"receipt_text"`
}

// ContextPreferencesView is the preferences block: the active
// accommodation keys, the active alternative-path words with the degraded
// permitted paths of the last outcome, and the usual-modality words of
// the mandatory material keys. The words are visible BEFORE the first
// reply of the assignee (AC-4).
type ContextPreferencesView struct {
	AccommodationKeys []string             `json:"accommodation_keys"`
	AlternativePaths  []string             `json:"alternative_paths"`
	UsualModalities   []ContextModalityRow `json:"usual_modalities"`
}

// ContextModalityRow is one usual-modality word of a mandatory material
// key (how the key is usually established).
type ContextModalityRow struct {
	MaterialKey string `json:"material_key"`
	Word        string `json:"word"`
}

// ContextCheckRow is one mandatory check of the recommendation: the
// material key with its state word (what must be established or verified
// before the step).
type ContextCheckRow struct {
	MaterialKey string `json:"material_key"`
	State       string `json:"state"`
}

// ContextRecommendationView is the recommendation block: the plan step
// words of the last policy outcome with the reason word, the mandatory
// checks, the deadline of the open handoff round and the constraint words
// of the effective mode. The projection NEVER grants an authority — the
// honest boundary text renders beside it (AC-7).
type ContextRecommendationView struct {
	Present         bool              `json:"present"`
	Steps           []string          `json:"steps"`
	Reason          string            `json:"reason"`
	Checks          []ContextCheckRow `json:"checks"`
	AckDeadline     string            `json:"ack_deadline,omitempty"`
	AckDeadlineText string            `json:"ack_deadline_text,omitempty"`
	DeadlineAt      time.Time         `json:"deadline_at"`
	Mode            string            `json:"mode,omitempty"`
	ModeVersion     int64             `json:"mode_version,omitempty"`
	ProjectionText  string            `json:"projection_text"`
}

// ClaimView is one claim of the summary view: the ref-type word with the
// ref id, the disputed mark and the claim text (the byte-exact span of
// the summary — the view model the section renders as the link row).
type ClaimView struct {
	RefType  string `json:"ref_type"`
	RefID    string `json:"ref_id,omitempty"`
	Disputed bool   `json:"disputed"`
	Text     string `json:"text"`
}

// SummaryView is the summary block: the last recorded row with the claim
// views and the read-time staleness (the word with the honest counters —
// a stale summary renders with the sources beside it, never as the actual
// one). The marking text is the honest AI boundary of AD-7/AD-8.
type SummaryView struct {
	Present               bool        `json:"present"`
	SummaryNo             int64       `json:"summary_no"`
	Kind                  string      `json:"kind"`
	MarkingText           string      `json:"marking_text"`
	Text                  string      `json:"text"`
	Claims                []ClaimView `json:"claims"`
	Staleness             string      `json:"staleness"`
	StalenessText         string      `json:"staleness_text"`
	StalenessCountersText string      `json:"staleness_counters_text"`
}

// DecisionContextView is the compiled context of the case: the twelve
// blocks of the operator section. The view is read-time — nothing is ever
// mutated by the assembly (the derivations stay derivations); the section
// appears only when the material exists (the open handoff or the recorded
// summary — the honest absence of both renders nothing, byte-for-byte).
type DecisionContextView struct {
	Present        bool                      `json:"present"`
	PanelText      string                    `json:"panel_text"`
	Handoff        ContextHandoffView        `json:"handoff"`
	WorkItems      []ContextWorkItemRow      `json:"work_items"`
	Mode           ContextModeView           `json:"mode"`
	Assurance      ContextAssuranceView      `json:"assurance"`
	Owner          ContextOwnerView          `json:"owner"`
	Facts          ContextFactsView          `json:"facts"`
	Observations   []ContextObservationRow   `json:"observations"`
	Deliveries     []ContextDeliveryRow      `json:"deliveries"`
	Commitments    []ContextCommitmentRow    `json:"commitments"`
	Preferences    ContextPreferencesView    `json:"preferences"`
	Recommendation ContextRecommendationView `json:"recommendation"`
	Summary        SummaryView               `json:"summary"`
}
