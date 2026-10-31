package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Fact corrections (Story 2.15, first slice)
// ---------------------------------------------------------------------------
//
// A correction is one append-only operator act bound to the EXACT fact
// version: the operator pins the version being corrected (optimistic
// concurrency — a second correction of the same version is a second row
// and a second fact version, never a silent rewrite), proposes one value
// under the fact material discipline and cites the verbatim evidence (the
// byte-exact spans of the accepted event or the summary text). The
// applied outcome POURS the proposed value into the EXISTING fact
// version ladder through the record_fact discipline (the source is the
// operator, the source id is correction:<correction_id>) — the pour is
// NEVER an automatic confirmation: the new version stays
// conflicting/unverified by DeriveFactStatus, the resolution is the
// existing confirm_fact_version command of a human. A matrix material
// field (price/availability/order_status) never pours: the outcome is
// disputed, the proposed value survives as the dispute row, the Owning
// System value is never rewritten; the re-read is a separate operator act.
// The outcome is the SERVER decision of the matrix gate — the request
// never carries it.

// ---------------------------------------------------------------------------
// The dictionaries of the slice
// ---------------------------------------------------------------------------

// Correction outcomes: applied is the poured new fact version; disputed
// is the honest dispute of a matrix field (the system value is not
// rewritten). Refusal cases are the 400 guards — never outcome words.
const (
	CorrectionOutcomeApplied  = "applied"
	CorrectionOutcomeDisputed = "disputed"
)

// CorrectionOutcomes is the full dictionary.
var CorrectionOutcomes = []string{CorrectionOutcomeApplied, CorrectionOutcomeDisputed}

// CorrectionOutcomeInDictionary reports whether the outcome word belongs
// to the closed dictionary.
func CorrectionOutcomeInDictionary(word string) bool {
	return inDictionary(CorrectionOutcomes, word)
}

// The evidence source kinds of the panel line: the spans of a correction
// cite the accepted event text of the case or the summary text. The kind
// derives from the summary link (0 = the event evidence, >=1 = the
// summary evidence) — it is never a request field.
const (
	CorrectionSourceEvent   = "event"
	CorrectionSourceSummary = "summary"
)

// Correction material limits (the fixed dictionary): the proposed value
// follows the fact value discipline (2048 canonical bytes, the digit-form
// numbers); the spans follow the 2.8 span discipline (<=8 spans of
// <=200 runes, byte-exact in the source); the reason is the mandatory why
// of the conscious act (1..200 runes).
const (
	MinCorrectionReasonRunes = 1
	MaxCorrectionReasonRunes = 200
	MaxCorrectionSpans       = 8
	MinCorrectionSpanRunes   = 1
	MaxCorrectionSpanRunes   = 200
	MaxCorrectionValueBytes  = MaxFactValueBytes
)

// CorrectionSourceIDPrefix is the fact-source id prefix of the applied
// pour: the store composes correction:<correction_id> — the key of the
// 8..200 GuardRecordFact discipline, unique per correction row (a new
// correction is a new id, a new pour key — the material idempotency of
// the fact ladder replays the exact repeat and never silently rewrites).
const CorrectionSourceIDPrefix = "correction:"

// CorrectionFactSourceID composes the fact source id of one correction.
func CorrectionFactSourceID(correctionID id.UUID) string {
	return CorrectionSourceIDPrefix + correctionID.String()
}

// CorrectionSourceKindOf derives the evidence source kind of the summary
// link: 0 is the event evidence, >=1 is the summary evidence.
func CorrectionSourceKindOf(summaryNo int64) string {
	if summaryNo >= 1 {
		return CorrectionSourceSummary
	}
	return CorrectionSourceEvent
}

// ---------------------------------------------------------------------------
// The correction row (append-only, never updated)
// ---------------------------------------------------------------------------

// CorrectionRecord is one append-only correction row: the tenant and the
// case, the monotonic number (the store computes it under the locks —
// the requests never carry it), the pinned target (the fact and its
// EXACT version), the summary link (0 = no summary; >=1 = the summary of
// THIS case — the store guards the belonging), the proposed value (the
// canonical fact material), the byte-exact evidence spans, the outcome
// word (the server matrix-gate decision), the acting operator with the
// channel (the operator pseudo-channel of the fact provenance), the
// reason and the three times of the 2.2 discipline (observed is the
// provider time of the act, never the receive time; created is the
// server truth). The row never mutates; a second correction is a new
// row.
type CorrectionRecord struct {
	TenantID      id.UUID
	CaseID        id.UUID
	CorrectionID  id.UUID
	CorrectionNo  int64
	FactID        id.UUID
	VersionNo     int64  // the pinned target version (1..versions of this fact)
	SummaryNo     int64  // 0 = no summary; >=1 = the summary of THIS case
	ProposedValue []byte // canonical JSON bytes (the fact material discipline)
	Spans         []string
	Outcome       string
	ActorID       id.UUID
	Channel       string
	Reason        string
	ObservedAt    time.Time
	ReceivedAt    time.Time
	CreatedAt     time.Time
}

// ---------------------------------------------------------------------------
// The write guards (the 400 refusal dictionary — never outcome words)
// ---------------------------------------------------------------------------

// GuardCorrectionReason bounds the reason of the correction: the act is
// conscious, the why is mandatory (1..200 runes).
func GuardCorrectionReason(reason string) error {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return fmt.Errorf("причина коррекции обязательна: акт осознанный")
	}
	if utf8.RuneCountInString(trimmed) > MaxCorrectionReasonRunes {
		return fmt.Errorf("причина коррекции длиннее %d символов", MaxCorrectionReasonRunes)
	}
	return nil
}

// GuardCorrectionSpans bounds the evidence spans and probes them against
// the cited source texts: at most 8 spans, each 1..200 runes, each
// present in at least one source text byte-for-byte (the anti-
// hallucination probe of the 2.8 discipline — a paraphrase or a case
// difference is a fabrication and refuses). The spans are optional
// material (0 spans passes); a cited span with an empty source material
// refuses honestly.
func GuardCorrectionSpans(spans []string, sourceTexts []string) error {
	if len(spans) > MaxCorrectionSpans {
		return fmt.Errorf("выдержек коррекции больше %d", MaxCorrectionSpans)
	}
	for i, span := range spans {
		runes := utf8.RuneCountInString(span)
		if runes < MinCorrectionSpanRunes || runes > MaxCorrectionSpanRunes {
			return fmt.Errorf("выдержка №%d — от %d до %d рун, получено %d", i+1, MinCorrectionSpanRunes, MaxCorrectionSpanRunes, runes)
		}
		present := false
		for _, text := range sourceTexts {
			if strings.Contains(text, span) {
				present = true
				break
			}
		}
		if !present {
			return fmt.Errorf("выдержка №%d отсутствует в источнике байт-в-байт: выдержка обязана быть строкой события или сводки (анти-фантазийный зонд)", i+1)
		}
	}
	return nil
}

// GuardCorrectionTimes holds the 2.2 time discipline: the observed time
// is mandatory and never claims more than the 48h provider skew ahead of
// the receive time; the created time is the server truth elsewhere.
func GuardCorrectionTimes(observedAt, receivedAt time.Time) error {
	if observedAt.IsZero() {
		return fmt.Errorf("время наблюдения коррекции обязательно")
	}
	if receivedAt.IsZero() {
		return fmt.Errorf("время получения коррекции обязательно")
	}
	if observedAt.After(receivedAt.Add(maxProviderClockSkew)) {
		return fmt.Errorf("время наблюдения слишком далеко в будущем относительно времени получения")
	}
	return nil
}

// GuardRecordCorrection validates the correction write: the target pin
// (the exact version of THIS fact — the store guards the belonging of
// the fact and the summary to the case), the proposed value under the
// fact material discipline (one canonical JSON value, the 2048-byte
// bound, the digit-form numbers — the exponential literals refuse), the
// evidence spans against the cited source texts and the mandatory
// reason. The outcome is NOT guarded here: it is the server decision of
// the matrix gate (DeriveCorrectionOutcome) — a request carrying an
// outcome word refuses at the strict decode of the transport.
func GuardRecordCorrection(versionCount int, versionNo int64, summaryNo int64, proposedValue []byte, spans []string, sourceTexts []string, reason string) error {
	if versionCount == 0 || versionNo < 1 || versionNo > int64(versionCount) {
		return fmt.Errorf("версия %d не существует у этого факта", versionNo)
	}
	if summaryNo < 0 {
		return fmt.Errorf("сводка коррекции — неотрицательный номер (0 = без сводки)")
	}
	if _, _, err := FactValue(proposedValue); err != nil {
		return err
	}
	if err := GuardCorrectionSpans(spans, sourceTexts); err != nil {
		return err
	}
	return GuardCorrectionReason(reason)
}

// ---------------------------------------------------------------------------
// The matrix gate (the server outcome decision, never a request field)
// ---------------------------------------------------------------------------

// DeriveCorrectionOutcome decides the outcome of the correction by the
// material key of the target fact: a matrix field (price, availability,
// order_status — the State/Authority Matrix v1 of 2.7) NEVER pours — the
// Owning System owns the value, the correction survives as the honest
// dispute row and the recommended re-read is the separate operator act
// (read_owning_system, by the operator's decision — never an automatic
// re-read); every other fact key pours the new version through the
// existing record_fact discipline. The word travels to the row and to
// the outbox — the decision is the domain's, not the request's.
func DeriveCorrectionOutcome(materialKey string) string {
	if MaterialFieldInMatrix(materialKey) {
		return CorrectionOutcomeDisputed
	}
	return CorrectionOutcomeApplied
}

// ---------------------------------------------------------------------------
// The correction state derivation (read-time, never stored)
// ---------------------------------------------------------------------------

// CorrectionStateView is the derived correction state of one fact: the
// LAST correction (the store passes the rows ordered by the correction
// number) with its outcome, number, time, actor, evidence source kind,
// spans and reason — the material of the panel line. No corrections is
// the zero view (the markers appear only with the material — the control
// branch renders byte-identically).
type CorrectionStateView struct {
	HasCorrection bool
	Outcome       string
	CorrectionNo  int64
	CreatedAt     time.Time
	ActorID       id.UUID
	SourceKind    string
	Spans         []string
	Reason        string
}

// DeriveCorrectionState derives the read-time state of one fact from its
// correction rows (ordered by the correction number — the same store
// trust as the fact status derivation): the last correction speaks. The
// derivation never mutates the rows.
func DeriveCorrectionState(corrections []CorrectionRecord) CorrectionStateView {
	if len(corrections) == 0 {
		return CorrectionStateView{}
	}
	last := corrections[len(corrections)-1]
	return CorrectionStateView{
		HasCorrection: true,
		Outcome:       last.Outcome,
		CorrectionNo:  last.CorrectionNo,
		CreatedAt:     last.CreatedAt,
		ActorID:       last.ActorID,
		SourceKind:    CorrectionSourceKindOf(last.SummaryNo),
		Spans:         last.Spans,
		Reason:        last.Reason,
	}
}

// CorrectionMarkerText is the honest marker of the fact line: the
// applied correction is «исправлено» (the new version is poured — the
// confirmation is the separate human act); the disputed correction is
// «оспорено» with the honest boundary (the system value is not
// changed); no corrections is the empty string (the marker appears only
// with the material).
func (v CorrectionStateView) CorrectionMarkerText() string {
	if !v.HasCorrection {
		return ""
	}
	if v.Outcome == CorrectionOutcomeDisputed {
		return "оспорено (значение системы не изменено)"
	}
	return "исправлено"
}

// CorrectionSourceText renders the evidence source line of the panel:
// the source kind word with the honest bound (the spans cite the event
// text or the summary text — the operator follows to the source and
// decides). No corrections is the empty string.
func (v CorrectionStateView) CorrectionSourceText() string {
	if !v.HasCorrection {
		return ""
	}
	switch v.SourceKind {
	case CorrectionSourceSummary:
		return "источник исправления: сводка"
	default:
		return "источник исправления: событие"
	}
}

// ---------------------------------------------------------------------------
// The outbox event (the safe canonical JSON)
// ---------------------------------------------------------------------------

// Outbox event types of the correction surface (the bounded vocabulary
// of the slice).
const (
	OutboxEventCorrectionRecorded   = "case.correction_recorded"
	OutboxEventCorrectionRecordedV1 = 1
)

// CorrectionRecordedPayload is the safe canonical JSON of the correction
// record event: the identifiers (the tenant, the case, the correction,
// the fact with its pinned version, the summary link), the dictionary
// word (the outcome) and the span counter. The proposed value and the
// spans NEVER enter the envelope — the perimeter of the outbox stays
// identifiers-and-words (the «кандидат не покидает строку» precedent of
// 2.6/2.7/2.13/2.14).
type CorrectionRecordedPayload struct {
	TenantID     id.UUID   `json:"tenant_id"`
	CaseID       id.UUID   `json:"case_id"`
	CorrectionID id.UUID   `json:"correction_id"`
	CorrectionNo int64     `json:"correction_no"`
	FactID       id.UUID   `json:"fact_id"`
	VersionNo    int64     `json:"version_no"`
	SummaryNo    int64     `json:"summary_no"`
	Outcome      string    `json:"outcome"`
	SpanCount    int       `json:"span_count"`
	ActorID      id.UUID   `json:"actor_id"`
	OccurredAt   time.Time `json:"occurred_at"`
}

// DedupKeyCorrectionRecorded builds the dedup key of one correction
// record.
func DedupKeyCorrectionRecorded(caseID id.UUID, correctionNo int64) string {
	return fmt.Sprintf("case.case_correction:%s:%d", caseID.String(), correctionNo)
}

// CorrectionRecordedPayloadJSON renders the safe canonical JSON.
func CorrectionRecordedPayloadJSON(payload CorrectionRecordedPayload) ([]byte, error) {
	return json.Marshal(payload)
}
