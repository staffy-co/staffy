package main

import (
	"context"
	"fmt"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.8 fixture: the objection record command and the persuasion
// derivations over the REAL domain functions (GuardObjectionRecord,
// SpanInPayload, DeriveObjectionState, GuardSalesReplyAgainstObjections,
// PlaybookStepsOf) — the fixture world stores the append-only rows and the
// reply links; the CURRENT objection, the refusal budget, the stopped word
// and the acknowledged derivation are computed at every read.

// fixtureObjectionID generates the next deterministic objection id.
func (f *casesFixture) fixtureObjectionID() id.UUID {
	f.objectionSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(90000+f.objectionSeq)))
}

// RecordObjection records one client objection of the Work Item: the REAL
// guards (the dictionary words, the span discipline), the turn source
// anchor (an accepted TEXT event of the case), the byte-exact span
// containment against the payload of the turn, the monotonic objection
// number and the metadata-only command record (the span text never enters
// the stored outcome).
func (f *casesFixture) RecordObjection(_ context.Context, command casesAPI.RecordObjectionCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("record_objection", casesDomain.GuardObjectionRecord(command.ObjectionType, command.ClientSignal, command.Spans)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("record_objection", command.IdempotencyKey, struct {
		Case     string   `json:"case"`
		WorkItem string   `json:"work_item"`
		TurnRef  string   `json:"turn_ref"`
		Type     string   `json:"objection_type"`
		Signal   string   `json:"client_signal"`
		Spans    []string `json:"spans"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.TurnRef.String(), command.ObjectionType, command.ClientSignal, command.Spans})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known || current.CaseID != command.CaseID {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_objection", fmt.Errorf("%s", casesDomain.TerminalWorkItemObjectionText))
	}
	// The turn source: an ACCEPTED TEXT event of the case (the span
	// carrier — the voice turn refuses honestly).
	event, found := f.fixtureEvent(command.TurnRef)
	if !found || event.CaseID != command.CaseID || event.State != casesDomain.EventStateAccepted {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_objection", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
	}
	if event.Modality != casesDomain.ModalityText {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_objection", fmt.Errorf("Разметка выдержек голосовых сообщений — вне текущего срез: источник возражения должен быть текстовым событием."))
	}
	// The span honesty: every span is the byte-exact substring of the
	// sealed payload of the turn source.
	payload := f.payloads[command.TurnRef]
	for _, span := range command.Spans {
		if !casesDomain.SpanInPayload(span, payload) {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("record_objection", fmt.Errorf("Выдержка не входит в слова клиента: «%s» не является дословным фрагментом payload события-источника. Выдуманные выдержки невозможны.", span))
		}
	}
	// The per-Work-Item monotonic objection number.
	objectionNo := int64(0)
	for _, record := range f.objections[command.CaseID] {
		if record.WorkItemID == command.WorkItemID && record.ObjectionNo > objectionNo {
			objectionNo = record.ObjectionNo
		}
	}
	objectionNo++
	objectionID := f.fixtureObjectionID()
	f.objections[command.CaseID] = append(f.objections[command.CaseID], casesDomain.ObjectionRecord{
		TenantID: f.tenant, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ObjectionID: objectionID, ObjectionNo: objectionNo, TurnRef: command.TurnRef,
		ObjectionType: command.ObjectionType, ClientSignal: command.ClientSignal,
		Spans:           append([]string(nil), command.Spans...),
		PlaybookVersion: casesDomain.ObjectionPlaybookVersion,
		ActorID:         command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{
		Command: "record_objection", ObjectionID: objectionID, ObjectionNo: objectionNo,
		ObjectionType: command.ObjectionType, ObjectionSignal: command.ClientSignal,
	}
	f.storeOutcome("record_objection", command.IdempotencyKey, outcome)
	return outcome, nil
}

// fixtureEvent resolves one event of the fixture world by its id.
func (f *casesFixture) fixtureEvent(eventID id.UUID) (*casesDomain.ChannelEvent, bool) {
	for _, event := range f.events {
		if event.EventID == eventID {
			return event, true
		}
	}
	return nil, false
}

// fixtureObjectionGate applies the REAL objection gate of the sales reply
// (the frozen ladder: ownership 2-4 → mode 2-5 → disclosure 2-5 → policy
// 2-6 → objection 2-8 → write; the SALES class only): the persuasion state
// of the Work Item derives from the append-only rows and the linked
// replies; the passing gate resolves the objection id the reply link
// carries (the server-side truth).
func (f *casesFixture) fixtureObjectionGate(command casesAPI.SendClientReplyCommand) (id.UUID, error) {
	if command.ReplyClass != casesDomain.ReplyClassSales {
		return id.UUID(""), nil
	}
	state := f.fixtureObjectionStateOf(command.CaseID, command.WorkItemID)
	if err := casesDomain.GuardSalesReplyAgainstObjections(state, command.PlaybookStep); err != nil {
		return id.UUID(""), err
	}
	if state.Current == nil {
		return id.UUID(""), nil
	}
	return state.Current.ObjectionID, nil
}

// fixtureObjectionStateOf derives the persuasion state of one Work Item
// over the REAL domain function (the records of the item with the linked
// replies of the case).
func (f *casesFixture) fixtureObjectionStateOf(caseID, workItemID id.UUID) casesDomain.ObjectionState {
	recordsOfItem := []casesDomain.ObjectionRecord{}
	for _, record := range f.objections[caseID] {
		if record.WorkItemID == workItemID {
			recordsOfItem = append(recordsOfItem, record)
		}
	}
	return casesDomain.DeriveObjectionState(recordsOfItem, f.replyLinks[caseID])
}

// fixtureObjectionView assembles the objection section of the case view
// over the REAL derivations: the per-Work-Item sections with the CURRENT
// objection (the maximal number), the lifetime refusal budget, the stopped
// word, the acknowledged derivation and the next-step words of the current
// playbook.
func (f *casesFixture) fixtureObjectionView(caseID id.UUID) casesAPI.ObjectionView {
	view := casesAPI.ObjectionView{
		TypeWords:       append([]string(nil), casesDomain.ObjectionTypes...),
		SignalWords:     append([]string(nil), casesDomain.ClientSignals...),
		StepWords:       append([]string(nil), casesDomain.PlaybookSteps...),
		PlaybookVersion: casesDomain.ObjectionPlaybookVersion,
		Threshold:       casesDomain.PersuasionThresholdV1,
		PanelText:       "Возражение — слова клиента со ссылкой на принятый turn: тип, сигнал и дословные выдержки. Бюджет убеждения — монотонный счёт отказов; при остановке разрешены только вежливое завершение, запрос приватности и передача человеку.",
	}
	for _, anchor := range f.items[caseID] {
		section := casesAPI.ObjectionWorkItemView{
			WorkItemID: anchor.WorkItemID, Threshold: casesDomain.PersuasionThresholdV1,
			LinkedReplies: []casesDomain.ReplyLink{},
		}
		recordsOfItem := []casesDomain.ObjectionRecord{}
		for _, record := range f.objections[caseID] {
			if record.WorkItemID == anchor.WorkItemID {
				recordsOfItem = append(recordsOfItem, record)
			}
		}
		section.ObjectionCount = len(recordsOfItem)
		state := casesDomain.DeriveObjectionState(recordsOfItem, f.replyLinks[caseID])
		section.Refusals = state.Refusals
		section.Stopped = state.Stopped
		section.Acknowledged = state.Acknowledged
		if state.Current != nil {
			section.Current = &casesAPI.ObjectionCard{
				Record: state.Current, Refusals: state.Refusals, Acknowledged: state.Acknowledged,
			}
			section.NextSteps = casesDomain.PlaybookStepsOf(state.Current.ObjectionType)
			for _, link := range f.replyLinks[caseID] {
				if link.ObjectionID == state.Current.ObjectionID {
					section.LinkedReplies = append(section.LinkedReplies, link)
				}
			}
		}
		view.Items = append(view.Items, section)
	}
	return view
}
