package main

import (
	"context"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.15 fixture: the operator correction record command over
// the REAL domain guard (GuardRecordCorrection — the pinned exact
// version of the fact of THIS case, the value material discipline, the
// byte-exact spans against the cited source, the mandatory reason) and
// the REAL matrix gate (DeriveCorrectionOutcome by the material key of
// the anchor — the outcome is NEVER a request field). The fixture world
// stores the append-only correction rows; the correction state of the
// fact rows is never stored — every read derives it anew through the
// REAL DeriveCorrectionState (the last correction of the fact speaks,
// the mirror of the app attach). The correction number is the
// fixture-owned monotonic truth (the requests never carry it). The
// applied pour goes through the fixture fact ladder with the operator
// source and the correction:<id> source key (the record_fact
// discipline mirror); the disputed outcome writes NO fact version (the
// Owning System value is never rewritten). The request hashes cover the
// WHOLE command materials INCLUDING the value, the spans and the
// summary link: the exact repeats answer the stored outcomes (the
// replays), the same keys with other materials are the honest
// conflicts.

// fixtureCorrectionID generates the next deterministic correction
// material id (the counter space of the Story 2.15 surface — the ids
// never collide with the other fixture surfaces).
func (f *casesFixture) fixtureCorrectionID() id.UUID {
	f.correctionSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(75000+f.correctionSeq)))
}

// fixtureCorrectionTargetText mirrors the store refusal of the target
// guard (the honest boundary of the write command).
const fixtureCorrectionTargetText = "Цель коррекции — точная версия факта этого обращения: факт не найден или принадлежит другому обращению, версия не существует у этого факта."

// fixtureCorrectionSummaryText mirrors the store refusal of the
// summary link guard (the link pins the EXACT summary version of THIS
// case).
const fixtureCorrectionSummaryText = "Сводка коррекции не найдена или принадлежит другому обращению: связь утверждает точную версию сводки этого обращения."

// fixtureCorrectionTerminalText mirrors the store refusal of the
// terminal case (the honest boundary of the write command).
const fixtureCorrectionTerminalText = "Терминальное обращение не принимает записей коррекции: обслуживание завершено — материал передан истории."

// fixtureCorrectionSourceTexts resolves the evidence source texts of
// the correction: the summary link >=1 pins the EXACT summary version
// of THIS case (its text is the cited source); the link 0 cites the
// accepted event payloads of the case (the sealed provider bytes — the
// same span discipline of 2.8).
func (f *casesFixture) fixtureCorrectionSourceTexts(caseID id.UUID, summaryNo int64, summaryText string) []string {
	if summaryNo >= 1 {
		if summaryText == "" {
			return nil
		}
		return []string{summaryText}
	}
	texts := []string{}
	if material := f.material[caseID]; material != nil {
		for _, event := range material.timeline {
			if event.State != casesDomain.EventStateAccepted {
				continue
			}
			if payload, known := f.payloads[event.EventID]; known {
				texts = append(texts, string(payload))
			}
		}
	}
	return texts
}

// RecordCorrection writes ONE append-only correction row (AC-1): the
// REAL domain guard of the operator material (the version pin, the
// value discipline, the byte-exact spans against the cited source, the
// mandatory reason), the idempotent command flow with the hash of the
// WHOLE material INCLUDING the value, the spans and the summary link,
// the honest refusal of an unknown case, the target guards (the fact
// and the pinned version belong to THIS case; the summary link pins the
// summary of THIS case), the REAL matrix gate (the server outcome
// decision by the material key of the anchor — the request never
// carries it) and the single write: the append-only row with the NEXT
// monotonic correction number (the fixture-owned truth) and the applied
// pour through the fixture fact ladder (the operator source, the
// correction:<id> source key — NEVER an automatic confirmation; the
// disputed outcome writes NO fact version).
func (f *casesFixture) RecordCorrection(_ context.Context, command casesAPI.RecordCorrectionCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if command.TenantID != f.tenant {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	material := f.material[command.CaseID]
	if material == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if material.cases.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_correction", fmt.Errorf("%s", fixtureCorrectionTerminalText))
	}
	// The target guards: the fact and the pinned version belong to THIS
	// case; the summary link pins the summary of THIS case.
	versions := f.factVersions[command.FactID]
	if len(versions) == 0 || versions[0].CaseID != command.CaseID {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_correction", fmt.Errorf("%s", fixtureCorrectionTargetText))
	}
	summaryText := ""
	if command.SummaryNo >= 1 {
		found := false
		for _, summary := range f.summaries[command.CaseID] {
			if summary.SummaryNo == command.SummaryNo {
				summaryText, found = summary.Text, true
			}
		}
		if !found {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("record_correction", fmt.Errorf("%s", fixtureCorrectionSummaryText))
		}
	}
	// The REAL domain guard of the operator material (the version pin,
	// the value discipline, the byte-exact spans, the reason).
	sourceTexts := f.fixtureCorrectionSourceTexts(command.CaseID, command.SummaryNo, summaryText)
	if err := fixtureWorkRule("record_correction", casesDomain.GuardRecordCorrection(len(versions), command.VersionNo, command.SummaryNo, command.Value, command.Spans, sourceTexts, command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the WHOLE material (the value, the spans,
	// the summary link, the version pin and the reason).
	outcome, fresh, err := f.workCommandFlow("record_correction", command.IdempotencyKey, struct {
		Case    string   `json:"case"`
		Fact    string   `json:"fact"`
		Version int64    `json:"version_no"`
		Summary int64    `json:"summary_no"`
		Value   string   `json:"value"`
		Spans   []string `json:"spans"`
		Reason  string   `json:"reason"`
	}{command.CaseID.String(), command.FactID.String(), command.VersionNo, command.SummaryNo,
		string(command.Value), command.Spans, command.Reason})
	if err != nil || !fresh {
		return outcome, err
	}
	// The REAL matrix gate: the server outcome decision by the material
	// key of the anchor (a matrix field NEVER pours — the honest
	// dispute row; the request never carries the word).
	outcomeWord := casesDomain.DeriveCorrectionOutcome(versions[0].MaterialKey)
	value, valueDigest, err := casesDomain.FactValue(command.Value)
	if err != nil {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_correction", err)
	}
	// The append-only row: the NEXT monotonic correction number (the
	// fixture-owned truth, the request never carries it).
	correctionNo := int64(0)
	for _, row := range f.corrections[command.CaseID] {
		if row.CorrectionNo > correctionNo {
			correctionNo = row.CorrectionNo
		}
	}
	correctionNo++
	correctionID := f.fixtureCorrectionID()
	f.corrections[command.CaseID] = append(f.corrections[command.CaseID], casesDomain.CorrectionRecord{
		TenantID: f.tenant, CaseID: command.CaseID,
		CorrectionID: correctionID, CorrectionNo: correctionNo,
		FactID: command.FactID, VersionNo: command.VersionNo, SummaryNo: command.SummaryNo,
		ProposedValue: value, Spans: append([]string{}, command.Spans...),
		Outcome: outcomeWord, ActorID: command.Actor.ID, Channel: "operator",
		Reason:     command.Reason,
		ObservedAt: command.Now, ReceivedAt: command.Now, CreatedAt: command.Now,
	})
	// The applied pour through the fixture fact ladder (the operator
	// source, the correction:<id> source key — the record_fact
	// discipline mirror; NEVER an automatic confirmation); the disputed
	// outcome writes NO fact version (the system value is never
	// rewritten).
	pouredVersion := int64(0)
	if outcomeWord == casesDomain.CorrectionOutcomeApplied {
		scope := versions[len(versions)-1]
		f.factVersions[command.FactID] = append(versions, casesDomain.FactVersion{
			TenantID: f.tenant, CaseID: command.CaseID, FactID: command.FactID,
			WorkItemID: scope.WorkItemID,
			VersionNo:  int64(len(versions)) + 1, MaterialKey: scope.MaterialKey,
			Value: value, ValueDigest: valueDigest,
			SourceType: casesDomain.FactSourceOperator, SourceID: casesDomain.CorrectionFactSourceID(correctionID),
			Channel: "operator", ActorID: command.Actor.ID,
			ObservedAt: command.Now, ReceivedAt: command.Now, CreatedAt: command.Now,
		})
		pouredVersion = int64(len(versions)) + 1
	}
	status, _ := casesDomain.DeriveFactStatus(f.factVersions[command.FactID], f.factConfirms[command.FactID])
	outcome = casesAPI.WorkOutcome{
		Command: "record_correction", CorrectionID: correctionID, CorrectionOutcome: outcomeWord,
		CorrectionSpans: len(command.Spans), FactID: command.FactID, FactVersion: pouredVersion, FactStatus: status,
	}
	f.storeOutcome("record_correction", command.IdempotencyKey, outcome)
	return outcome, nil
}

// fixtureAttachCorrections maps the read-time correction state onto the
// fact rows of the API view (the app attachContextCorrections mirror):
// the per-fact groups keep the store order (the correction number), the
// LAST row of each group speaks through the REAL DeriveCorrectionState;
// no corrections is the zero view (the markers appear only with the
// material — the control branch renders byte-identically).
func fixtureAttachCorrections(view *casesAPI.DecisionContextView, corrections []casesDomain.CorrectionRecord, now time.Time) {
	_ = now
	if len(corrections) == 0 || !view.Present {
		return
	}
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
