package main

import (
	"context"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.16 fixture: the operator update promise command over the
// REAL domain guard (GuardPromiseUpdate — the deadline word of the fixed
// dictionary, the honest unknown word included) and the REAL round
// anchor set (UpdatePromiseRoundAccepts — the open waiting of the
// client: requested, assigned, overdue, acknowledged; the declined and
// closed rounds refuse with the mirror words of the store). The fixture
// world stores the append-only promise rows; the promise state and the
// client waiting status are NEVER stored — every read derives them anew
// through the REAL DeriveUpdatePromiseState / DeriveClientWaitingStatus
// (the mirror of the app waitingView). The promise number is the
// fixture-owned monotonic truth (the requests never carry it); the
// promised time is the REAL server computation UpdatePromiseDeadline
// (now + window of the word — the unknown word stays the ZERO time, the
// material honesty pair of FR-41). The read derivations take the
// journeyClock truth of the moment (the shift-able fixture clock — the
// honest mirror of the transport clock the real app reads; the missed
// branch walks the shifted moment, the rows never mutate).

// fixturePromiseID generates the next deterministic promise material id
// (the counter space of the Story 2.16 surface — the ids never collide
// with the other fixture surfaces: 75000 the corrections, 77500 this
// one, 80000 the next family).
func (f *casesFixture) fixturePromiseID() id.UUID {
	f.promiseSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(77500+f.promiseSeq))
}

// fixturePromiseTerminalText mirrors the store refusal of the terminal
// case (the honest boundary of the write command — the read stays
// honest: the terminal case READS its promise history).
const fixturePromiseTerminalText = "Терминальное обращение не принимает обещаний обновления: обслуживание завершено — материал читается историей."

// fixturePromiseRoundText mirrors the store refusal of the round guard
// (the declined round consumed the assignment — the re-assignment
// decision belongs to the human; the closed rounds ended the waiting).
const fixturePromiseRoundText = "Обещание обновления якорится открытым ожиданием клиента: раунд в состоянии «%s» не принимает обещаний — переназначение и закрытие остаются решением человека."

// PromiseUpdate writes ONE append-only update promise row of the open
// handoff round (AC-2): the honest refusal of an unknown case, the
// terminal mirror, the round anchor (the OPEN lifecycle, else the LAST
// closed one — the guard refuses with the honest state word; no
// lifecycle at all — the absent mirror), the REAL domain guard of the
// ONLY request material (the deadline word of the fixed dictionary),
// the idempotent command flow with the hash of the WHOLE material (the
// deadline word with the case anchor), the single write: the append-only
// row with the NEXT monotonic promise number (the fixture-owned truth)
// and the REAL server-computed promised time (the known word — now +
// window; the honest unknown word — the ZERO time, the material CHECK
// pair of the table). The promise NEVER resolves the round and NEVER
// mutates a former row (the supersede is the read-time derivation of
// the LAST promise). The stored outcome carries the promise id with the
// deadline word ONLY — the promised time and the promise number are the
// server truths the reader learns from the waiting view of the page.
func (f *casesFixture) PromiseUpdate(_ context.Context, command casesAPI.PromiseUpdateCommand) (casesAPI.WorkOutcome, error) {
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
		return casesAPI.WorkOutcome{}, fixtureWorkRule("promise_handoff_update", fmt.Errorf("%s", fixturePromiseTerminalText))
	}
	// The round anchor (the store mirror): the OPEN lifecycle of this
	// case, else the LAST closed one (the guard then refuses with the
	// honest state word); no lifecycle at all — the absent mirror.
	target := f.fixtureHandoffOpen(command.CaseID, command.Now)
	if target == nil {
		target = f.fixtureHandoffLast(command.CaseID, command.Now)
	}
	if target == nil {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("promise_handoff_update", fmt.Errorf("%s", casesDomain.HandoffAbsentText))
	}
	if !casesDomain.UpdatePromiseRoundAccepts(target.State) {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("promise_handoff_update", fmt.Errorf(fixturePromiseRoundText, target.State))
	}
	// The REAL domain guard of the ONLY request material (the deadline
	// word of the fixed dictionary — the honest unknown word included; a
	// fabricated deadline NEVER passes).
	if err := fixtureWorkRule("promise_handoff_update", casesDomain.GuardPromiseUpdate(command.DeadlineWord)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the WHOLE command material — the deadline
	// word with the case anchor (the replay of the exact repeat returns
	// the stored outcome; the same key with another word is the honest
	// key conflict).
	outcome, fresh, err := f.workCommandFlow("promise_handoff_update", command.IdempotencyKey, struct {
		Case string `json:"case"`
		Word string `json:"deadline_word"`
	}{command.CaseID.String(), command.DeadlineWord})
	if err != nil || !fresh {
		return outcome, err
	}
	// The server truths: the NEXT monotonic promise number (the
	// fixture-owned truth, the request never carries it) and the REAL
	// server computation of the promised time (the known word — now +
	// window; the honest unknown word — the ZERO time).
	promiseNo := int64(0)
	for _, row := range f.promises[command.CaseID] {
		if row.PromiseNo > promiseNo {
			promiseNo = row.PromiseNo
		}
	}
	promiseNo++
	promiseID := f.fixturePromiseID()
	f.promises[command.CaseID] = append(f.promises[command.CaseID], casesDomain.UpdatePromiseRecord{
		TenantID: f.tenant, CaseID: command.CaseID,
		PromiseID: promiseID, PromiseNo: promiseNo,
		RequestNo: target.Request.RequestNo, DeadlineWord: command.DeadlineWord,
		PromisedAt: casesDomain.UpdatePromiseDeadline(command.DeadlineWord, command.Now),
		ActorID:    command.Actor.ID,
		ObservedAt: command.Now, ReceivedAt: command.Now, CreatedAt: command.Now,
	})
	// The stored outcome carries the identifiers and the dictionary word
	// only (the promise number and the computed time NEVER enter — the
	// reader learns them from the waiting view of the case page).
	outcome = casesAPI.WorkOutcome{
		Command: "promise_handoff_update", PromiseID: promiseID, PromiseDeadlineWord: command.DeadlineWord,
	}
	f.storeOutcome("promise_handoff_update", command.IdempotencyKey, outcome)
	return outcome, nil
}

// fixtureWaitingUpdateView derives the read-time waiting projection of
// the case page (the app waitingView mirror): the LAST round of the
// lifecycle (the same derivation truth the 2.13 panel reads), the
// promises of THAT round only (the round anchor filters the append-only
// rows — the promise rows of the former rounds stay the honest
// history), the state of the LAST promise through the REAL
// DeriveUpdatePromiseState (the missed word is the truth of the moment
// — never a stored status) and the client waiting status in the THREE
// distinctions of AC-1 through the REAL DeriveClientWaitingStatus. The
// block appears ONLY with the promise material: a round without
// promises is the ZERO view (the control branch renders
// byte-identically); the form dictionary renders regardless of the
// material. The view carries the CLIENT-facing material only — the
// persona NEVER enters the waiting texts (the anti-disclosure of AC-1).
func (f *casesFixture) fixtureWaitingUpdateView(caseID id.UUID, now time.Time) casesAPI.WaitingUpdateView {
	view := casesAPI.WaitingUpdateView{
		// The form dictionary renders regardless of the material (the
		// fixed compile-time words — never the invented options).
		DeadlineWords: append([]string(nil), casesDomain.UpdateDeadlines...),
	}
	// The round of the waiting truth: the LAST request of the lifecycle
	// (the same derivation truth the 2.13 panel reads; no lifecycle, no
	// waiting block — the honest absence, never an invented default).
	last := f.fixtureHandoffLast(caseID, now)
	if last == nil {
		return view
	}
	// The promises of THIS round only (the round anchor filters the
	// append-only rows; the promise numbers keep the store order).
	roundPromises := make([]casesDomain.UpdatePromiseRecord, 0, len(f.promises[caseID]))
	for _, promise := range f.promises[caseID] {
		if promise.RequestNo == last.Request.RequestNo {
			roundPromises = append(roundPromises, promise)
		}
	}
	state := casesDomain.DeriveUpdatePromiseState(roundPromises, last.State, now)
	if !state.HasPromise {
		// The control branch: the round without promises renders NOTHING
		// (the waiting view is the ZERO view — the client status word of
		// the 2.13 panel stays the honest surface of the round without
		// promises).
		return view
	}
	clientStatus := casesDomain.DeriveClientWaitingStatus(last.State)
	view.ClientStatus = clientStatus
	view.ClientStatusText = casesDomain.ClientWaitingStatusText(clientStatus)
	view.HasPromise = true
	view.PromiseNo = state.PromiseNo
	view.DeadlineWord = state.DeadlineWord
	view.DeadlineText = casesDomain.UpdateDeadlineText(state.DeadlineWord)
	view.PromisedAt = state.PromisedAt
	view.State = state.State
	view.StateText = casesDomain.UpdatePromiseStateText(state.State)
	view.PromiseLineText = state.ClientPromiseText()
	if state.State == casesDomain.UpdatePromiseMissed {
		view.MissedNextStepText = casesDomain.UpdatePromiseMissedNextStepText
	}
	return view
}
