package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// The Story 2.16 command and view: the operator update promise of the
// handoff round and the truthful waiting projection of the case page
// ---------------------------------------------------------------------------

// waitingUpdateScopeText is the honest scope refusal of the promise
// command (the shared «review или configuration» vocabulary of the
// 2.5..2.15 rights gates): the promise is the duty of communication —
// an audit-only member never writes it, ANY Response Owner works with
// it (the honest difference from the 2.14 producer record, the precedent
// of the 2.15 correction).
const waitingUpdateScopeText = "Обещание обновления требует полномочия review или configuration: участники только с audit не обещают обновлений клиенту."

// casesStoreTerminalPromiseText mirrors the store refusal of the terminal
// case (the honest boundary of the write command — the v1 backstop word
// of the summary and correction precedents: the app answers BEFORE the
// transaction, the store re-validates under the locks; the read stays
// honest — the terminal case READS its promise history).
const casesStoreTerminalPromiseText = "Терминальное обращение не принимает обещаний обновления: обслуживание завершено — материал читается историей."

// The compile-time proof of the narrow interface registration: the
// composite Service carries the promise command (the transport
// type-asserts casesAPI.UpdatePromiseService — the CorrectionService
// precedent; the test fakes of the other stories stay untouched).
var _ casesAPI.UpdatePromiseService = (*Service)(nil)

// PromiseUpdate writes ONE append-only update promise row of the open
// handoff round (AC-2): the rights gate FIRST (review or configuration —
// the promise is the duty of communication, NEVER gated by the
// conversation ownership — any Response Owner may promise the update,
// the precedent of the 2.15 correction), the domain guard of the ONLY
// request material (the deadline word of the fixed dictionary — the
// honest unknown word included; the promised time and the promise
// number are the SERVER truths, the strict decode of the transport
// refuses them), the replay lookup with the whole-material hash (the
// deadline word + the case anchor), the case anchor (open — the
// terminal refusal), the conversation of the lock plan and the single
// transactional write. The store re-validates every guard under the
// case + conversation advisory locks: the round anchor (the OPEN
// lifecycle of THIS case — the declined, cancelled and returned rounds
// refuse with the store mirror words), the domain guard, the NEXT
// monotonic promise number and the SERVER-computed promised time (the
// known words — now + window; the honest unknown word — NULL, the
// material CHECK pair of the table). The promise NEVER resolves the
// round and NEVER mutates a former row (the lifecycle transitions
// belong to the existing 2.13 commands; the supersede is the read-time
// derivation). The outbox envelope carries the identifiers and the
// deadline word ONLY — the client texts NEVER enter.
func (s *Service) PromiseUpdate(ctx context.Context, command casesAPI.PromiseUpdateCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration (the M1 precedent —
	// the audit-only member reads the audit contours, never writes the
	// operator act; the promise is available at ANY Response Owner).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, waitingUpdateScopeText)
	}
	// The domain guard of the ONLY request material (the honest 400 with
	// the dictionary words in the text — the domain truth before SQL; the
	// table CHECK words are the backstops): the deadline word of the fixed
	// dictionary, the honest unknown word included.
	if err := rule("promise_handoff_update", casesDomain.GuardPromiseUpdate(command.DeadlineWord)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the WHOLE command material — the deadline
	// word with the case anchor (the replay of the exact repeat returns
	// the stored outcome; the same key with another word is the honest
	// key conflict).
	hash := requestHash(struct {
		Case string `json:"case"`
		Word string `json:"deadline_word"`
	}{command.CaseID.String(), command.DeadlineWord})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "promise_handoff_update", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case anchor: the case must exist in the tenant scope and stay
	// open (the store re-validates under the locks; the read path stays
	// honest — the terminal case READS its promise history).
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, rule("promise_handoff_update", fmt.Errorf("%s", casesStoreTerminalPromiseText))
	}
	// The conversation of the lock plan: the case's own conversation (the
	// SAME sorted plan of the 2.13 lifecycle writers — the promise write
	// and the round closing see each other's committed truth).
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The identifiers of the promise row and its outbox envelope (the
	// promise number and the promised time are the SERVER truths — the
	// store derives them under the locks; the round anchor is NEVER a
	// request field — the store resolves the open round under the locks).
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
	// The stored outcome carries the identifiers and the dictionary words
	// only: the promise id with the deadline word (the promised time and
	// the promise number NEVER enter the stored outcome — the reader
	// learns them from the waiting view of the case page; the client
	// texts are the read-time render of the dictionary, never stored).
	outcome := casesAPI.WorkOutcome{
		Command: "promise_handoff_update", PromiseID: ids[0], PromiseDeadlineWord: command.DeadlineWord,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "promise_handoff_update", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: conversation.ConversationID,
		PromiseID: ids[0], PromiseDeadlineWord: command.DeadlineWord, PromiseOutboxEventID: ids[1],
		CorrelationID: correlation, Outcome: outcomeJSON,
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

// waitingView derives the read-time waiting projection of the case page
// (the CaseView extension — NO new read path): the ONE consistent
// snapshot of the handoff lifecycle rows together with the promise rows
// (the store joins both halves in one read transaction), the LAST round
// of the lifecycle (the same derivation truth the 2.13 client status
// reads — handoffLastOf), the client waiting status word in the THREE
// distinctions of AC-1 and the state of the LAST promise of THAT round
// (the append-only rows filtered by the round anchor — the promise rows
// of the former rounds stay the honest history, never the current
// waiting truth). The block appears ONLY with the promise material: a
// round without promises is the ZERO view — the page renders
// byte-for-byte unchanged (the control branch of the slice). The view
// carries the CLIENT-facing material only: the operator names, the
// epochs and the identifiers NEVER enter the waiting texts (the
// anti-disclosure of AC-1 — the responsible actor lives in the
// append-only row and the outbox event, never in the client
// projection).
func (s *Service) waitingView(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesAPI.WaitingUpdateView, error) {
	material, err := s.repository.ReadUpdatePromiseState(ctx, scope, caseID)
	if err != nil {
		return casesAPI.WaitingUpdateView{}, err
	}
	view := casesAPI.WaitingUpdateView{
		// The form dictionary renders regardless of the material (the
		// fixed compile-time words — never the invented options).
		DeadlineWords: append([]string(nil), casesDomain.UpdateDeadlines...),
	}
	now := s.clock.Now()
	// The round of the waiting truth: the LAST request of the lifecycle
	// (the same derivation truth the 2.13 panel reads; no lifecycle, no
	// waiting block — the honest absence, never an invented default).
	last := handoffLastOf(material.Handoff, now)
	if last == nil {
		return view, nil
	}
	// The promises of THIS round only (the round anchor filters the
	// append-only rows — the promise rows of the former rounds stay the
	// honest history; the store order by the monotonic promise numbers
	// holds).
	roundPromises := make([]casesDomain.UpdatePromiseRecord, 0, len(material.Promises))
	for _, promise := range material.Promises {
		if promise.RequestNo == last.Request.RequestNo {
			roundPromises = append(roundPromises, promise)
		}
	}
	state := casesDomain.DeriveUpdatePromiseState(roundPromises, last.State, now)
	if !state.HasPromise {
		// The control branch: the round without promises renders NOTHING
		// (the waiting view is the ZERO view — the client status word of
		// the 2.13 panel stays the honest surface of the round without
		// promises; the waiting projection exists only with the promise
		// material).
		return view, nil
	}
	// The client waiting status of the round (the dictionary word with
	// the honest text — three distinctions plus the honest ending words;
	// the derivation takes the STATE WORD only, the persona NEVER
	// enters).
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
	return view, nil
}
