package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/identity/domain"
)

// ---------------------------------------------------------------------------
// The Story 2.15 command and view: the operator correction record and the
// correction state of the fact rows of the decision context
// ---------------------------------------------------------------------------

// casesStoreTerminalCorrectionText mirrors the store refusal of the
// terminal case (the honest boundary of the write command — the v1
// backstop word of the summary precedent: the app answers BEFORE the
// transaction, the store re-validates under the locks).
const casesStoreTerminalCorrectionText = "Терминальное обращение не принимает записей коррекции: обслуживание завершено — материал читается историей, исправление не записывается."

// correctionTargetForeignText mirrors the store refusal of the target
// guard (the honest 400 of the precision failure — the app pre-read maps
// the missing anchor onto the SAME words the store answers under the
// locks; the store re-validates the ladder and the belonging).
const correctionTargetForeignText = "Цель коррекции — точная версия факта этого обращения: факт не найден или принадлежит другому обращению, версия не существует у этого факта."

// The compile-time proof of the narrow interface registration: the
// composite Service carries the correction record command (the transport
// type-asserts casesAPI.CorrectionService — the HandoffService precedent;
// the test fakes of the other stories stay untouched).
var _ casesAPI.CorrectionService = (*Service)(nil)

// RecordCorrection writes ONE append-only correction row (AC-1/AC-8): the
// rights gate FIRST (review or configuration — an audit-only member never
// writes the operator act; the correction is the duty of correctness, it
// is NEVER gated by the conversation ownership — the honest difference
// from the 2.14 producer record of the owner-Staffy), the domain guard of
// the material (the fact value discipline, the mandatory reason — the
// honest 400 before SQL; the span bounds and the byte-exact source check
// live in the store under the locks with the LIVE source texts), the
// replay lookup with the whole-material hash (the proposed value and the
// spans INCLUDED — the same key with another material is the honest key
// conflict), the case anchor (open — the terminal refusal), the
// conversation of the lock plan, the fact pre-read (the material key of
// the anchor — the lock plan key of the pour — with the honest version
// pin), the matrix-gate outcome word of the stored outcome and the single
// transactional write. The store re-validates every guard under the
// case + conversation + fact locks and derives the NEXT monotonic
// correction number; the applied outcome POURS the new fact version
// through the existing record_fact discipline (source operator,
// correction:<id> source key — NEVER an automatic confirmation); the
// disputed outcome writes NO fact version (the Owning System value is
// never rewritten). The stored outcome carries the correction id, the
// outcome word and the span counter ONLY — the proposed value and the
// spans NEVER enter.
func (s *Service) RecordCorrection(ctx context.Context, command casesAPI.RecordCorrectionCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration (the M1 precedent —
	// the audit-only member reads the audit contours, never writes the
	// operator act; the correction is available at ANY Response Owner).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, decisionContextScopeText)
	}
	// The domain guard of the operator material (the honest 400 before
	// SQL; the table CHECK words are the backstops): the proposed value
	// under the fact material discipline (one canonical JSON value, the
	// 2048-byte bound, the digit-form numbers — the exponential literals
	// refuse) and the mandatory reason of the conscious act.
	value, valueDigest, err := casesDomain.FactValue(command.Value)
	if err != nil {
		return casesAPI.WorkOutcome{}, rule("record_correction", err)
	}
	if err := rule("record_correction", casesDomain.GuardCorrectionReason(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the WHOLE command material INCLUDING the
	// proposed value (the canonical bytes), the spans, the version pin and
	// the summary link (the replay of the exact repeat returns the stored
	// outcome; the same key with another material is the honest key
	// conflict).
	hash := requestHash(struct {
		Case      string   `json:"case"`
		Fact      string   `json:"fact"`
		VersionNo int64    `json:"version_no"`
		SummaryNo int64    `json:"summary_no"`
		Value     string   `json:"value"`
		Spans     []string `json:"spans"`
		Reason    string   `json:"reason"`
	}{command.CaseID.String(), command.FactID.String(), command.VersionNo, command.SummaryNo, string(value), command.Spans, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_correction", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case anchor: the case must exist in the tenant scope and stay
	// open (the store re-validates under the locks; the read path stays
	// honest — the terminal case READS its material).
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, rule("record_correction", fmt.Errorf("%s", casesStoreTerminalCorrectionText))
	}
	// The conversation of the lock plan: the case's own conversation (the
	// SAME sorted plan of the summary record — the correction write
	// serializes with the ownership, mode and summary writers; the store
	// adds the fact keys of the pour to the plan).
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The fact pre-read: the material key of the anchor (the lock plan key
	// of the pour) with the honest version pin (the store re-validates the
	// ladder under the locks — a raced new version between the read and
	// the write re-pins honestly by the store guards; a foreign or missing
	// anchor answers the SAME target-foreign words the store answers).
	material, err := s.repository.ReadFactMaterial(ctx, scope, command.CaseID, command.FactID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return casesAPI.WorkOutcome{}, rule("record_correction", fmt.Errorf("%s", correctionTargetForeignText))
		}
		return casesAPI.WorkOutcome{}, err
	}
	versions := int64(len(material.Versions))
	if versions == 0 || command.VersionNo < 1 || command.VersionNo > versions {
		return casesAPI.WorkOutcome{}, rule("record_correction", fmt.Errorf("%s", correctionTargetForeignText))
	}
	// The identifiers of the correction row and its outbox envelope (the
	// correction number and the outcome ROW word are the SERVER truth —
	// the store derives the number under the locks; the outcome word is
	// decided by the domain matrix gate over the immutable anchor key).
	ids, err := s.handoffIDs(command.Now, 2)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	// The matrix-gate outcome of the anchor key: the material key is the
	// immutable identity of the fact anchor — the app derivation equals
	// the store's lock-visible decision; the word travels to the stored
	// outcome (the request NEVER carries it; the pour truth stays the
	// store's under the locks).
	outcomeWord := casesDomain.DeriveCorrectionOutcome(material.Anchor.MaterialKey)
	// The stored outcome carries the identifiers and the dictionary words
	// only: the correction id, the outcome word of the matrix gate and the
	// span counter (the proposed value and the spans NEVER enter the
	// stored outcome; the correction number is the server truth the
	// reader learns from the context view).
	outcome := casesAPI.WorkOutcome{
		Command: "record_correction", CorrectionID: ids[0],
		CorrectionOutcome: outcomeWord, CorrectionSpans: len(command.Spans),
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_correction", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: conversation.ConversationID,
		FactID: command.FactID, VersionNo: command.VersionNo, MaterialKey: material.Anchor.MaterialKey,
		Value: value, ValueDigest: valueDigest, Reason: command.Reason,
		SummaryNo: command.SummaryNo, Spans: command.Spans,
		CorrectionID: ids[0], CorrectionOutboxEventID: ids[1], CorrelationID: correlation,
		Outcome:    outcomeJSON,
		ObservedAt: command.Now.UTC().Truncate(time.Microsecond),
		ReceivedAt: command.Now.UTC().Truncate(time.Microsecond),
		Now:        command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// attachContextCorrections derives the correction state of every fact row
// of the compiled context view over the correction rows of the SAME
// consistent snapshot (the store read joins the context blocks and the
// correction rows in ONE transaction — the markers never lag the
// material). The state is the read-time domain derivation (the LAST
// correction of the fact speaks; no corrections is the zero state — the
// rows stay untouched and the control branch renders byte-identically).
func attachContextCorrections(view *casesAPI.DecisionContextView, corrections []casesDomain.CorrectionRecord) {
	if len(corrections) == 0 || !view.Present {
		return
	}
	// The per-fact groups keep the store order (the correction number) —
	// the last row of each group speaks.
	groups := make(map[string][]casesDomain.CorrectionRecord)
	for _, record := range corrections {
		key := record.FactID.String()
		groups[key] = append(groups[key], record)
	}
	states := make(map[string]casesDomain.CorrectionStateView, len(groups))
	for key, rows := range groups {
		states[key] = casesDomain.DeriveCorrectionState(rows)
	}
	apply := func(rows []casesAPI.ContextFactRow) {
		for i := range rows {
			state, ok := states[rows[i].FactID]
			if !ok || !state.HasCorrection {
				continue
			}
			rows[i].HasCorrection = true
			rows[i].CorrectionMarkerText = state.CorrectionMarkerText()
			rows[i].CorrectionSourceText = state.CorrectionSourceText()
			rows[i].CorrectionNo = state.CorrectionNo
			rows[i].CorrectionOutcome = state.Outcome
			rows[i].CorrectionAt = state.CreatedAt
			rows[i].CorrectionSpans = state.Spans
			rows[i].CorrectionReason = state.Reason
		}
	}
	apply(view.Facts.Confirmed)
	apply(view.Facts.Unconfirmed)
	apply(view.Facts.Conflicting)
}
