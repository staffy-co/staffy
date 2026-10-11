package main

import (
	"context"
	"fmt"
	"strconv"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.5 fixture: the four mode commands and the mode view over
// the REAL domain derivations (ActiveConversationMode, WorkItemModeContext,
// EffectiveWorkMode, EffectiveDisclosure, GuardProposeTransition,
// GuardExitTransition, ExitAuthorityAllowed, GuardReplyClassAgainstMode) —
// the fixture mirrors the store through the same domain functions, without
// convenient defaults (the R2 lesson of 2-3).

// fixtureModeID generates the next deterministic mode material id.
func (f *casesFixture) fixtureModeID() id.UUID {
	f.modeSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(30000+f.modeSeq)))
}

// fixtureDisclosureID generates the next deterministic disclosure id.
func (f *casesFixture) fixtureDisclosureID() id.UUID {
	f.disclosureSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(40000+f.disclosureSeq)))
}

// fixtureDeliveryEventID generates the next deterministic delivery event id.
func (f *casesFixture) fixtureDeliveryEventID() id.UUID {
	f.deliveryEventSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(50000+f.deliveryEventSeq)))
}

// fixtureModeConversation validates the conversation of a mode command: the
// case's OWN conversation (the ladder anchor).
func (f *casesFixture) fixtureModeConversation(caseID, conversationID id.UUID) (casesDomain.Conversation, error) {
	ownCase, known := f.caseOfConversation(conversationID)
	if !known || ownCase != caseID {
		return casesDomain.Conversation{}, fixtureWorkRule("mode", fmt.Errorf("режим управляется на собственном диалоге обращения: выберите диалог этого обращения"))
	}
	conversation, found := f.conversationOf(ownCase, conversationID)
	if !found {
		return casesDomain.Conversation{}, casesDomain.ErrNotFound
	}
	return conversation, nil
}

// fixtureModeWorkItem validates the Work Item anchor of a mode or reply
// command (the item of the case).
func (f *casesFixture) fixtureModeWorkItem(caseID, workItemID id.UUID) error {
	if _, known := f.workAnchors[workItemID]; !known {
		return fixtureWorkRule("mode", fmt.Errorf("элемент работы не найден в обращении: контекст режима привязан к элементу этого обращения"))
	}
	if f.workAnchors[workItemID].CaseID != caseID {
		return fixtureWorkRule("mode", fmt.Errorf("элемент работы не найден в обращении: контекст режима привязан к элементу этого обращения"))
	}
	return nil
}

// fixtureEventOfCase resolves the accepted event of the case of a turn
// reference (the honest not-found otherwise).
func (f *casesFixture) fixtureEventOfCase(caseID, eventID id.UUID) bool {
	for _, event := range f.events {
		if event.EventID == eventID && event.CaseID == caseID && event.State == casesDomain.EventStateAccepted {
			return true
		}
	}
	return false
}

// ProposeMode raises the mode of the conversation or of one Work Item
// context (the REAL ladder decision; the CAS word of the ladder).
func (f *casesFixture) ProposeMode(_ context.Context, command casesAPI.ProposeModeCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("propose_mode", casesDomain.GuardProposeModeMaterial(command.Scope, command.Mode, command.Reason, command.WorkItemID)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := fixtureWorkRule("propose_mode", casesDomain.GuardSignalSource(command.SignalSource, command.TurnRef)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := fixtureWorkRule("propose_mode", casesDomain.GuardExpectedModeVersion(command.ExpectedModeVersion)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("propose_mode", command.IdempotencyKey, struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		Mode         string `json:"mode"`
		Scope        string `json:"scope"`
		WorkItem     string `json:"work_item"`
		Expected     int64  `json:"expected_mode_version"`
		Reason       string `json:"reason"`
		SignalSource string `json:"signal_source"`
		TurnRef      string `json:"turn_ref"`
	}{command.CaseID.String(), command.ConversationID.String(), command.Mode, command.Scope, command.WorkItemID.String(), command.ExpectedModeVersion, command.Reason, command.SignalSource, command.TurnRef.String()})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if _, err := f.fixtureModeConversation(command.CaseID, command.ConversationID); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if command.SignalSource == casesDomain.ModeSignalTurnEvent && !f.fixtureEventOfCase(command.CaseID, command.TurnRef) {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("propose_mode", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
	}
	if command.Scope == casesDomain.ModeScopeWorkItem {
		if err := f.fixtureModeWorkItem(command.CaseID, command.WorkItemID); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	terms := f.modeTerms[command.CaseID]
	active := casesDomain.ActiveConversationMode(terms)
	context := active
	if command.Scope == casesDomain.ModeScopeWorkItem {
		context = casesDomain.WorkItemModeContext(terms, command.WorkItemID)
	}
	if err := fixtureWorkRule("propose_mode", casesDomain.GuardProposeTransition(command.Scope, command.Mode, active, context)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if current := casesDomain.ModeTermMaxVersion(terms); current != command.ExpectedModeVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ModeConflictError{
			ActiveMode: casesDomain.EffectiveWorkModeWord(active), Reason: active.Reason,
			SignalSource: active.SignalSource, RuleVersion: casesDomain.ModePolicyVersion, ModeVersion: current,
		}
	}
	priorMode := casesDomain.EffectiveWorkModeWord(casesDomain.EffectiveWorkMode(active, context))
	termID := f.fixtureModeID()
	f.modeTerms[command.CaseID] = append(f.modeTerms[command.CaseID], casesDomain.ModeTerm{
		TenantID: f.tenant, CaseID: command.CaseID, ConversationID: command.ConversationID, TermID: termID,
		ModeVersion: command.ExpectedModeVersion + 1, Mode: command.Mode, Scope: command.Scope,
		WorkItemID: command.WorkItemID, Origin: casesDomain.ModeOriginProposal, Reason: command.Reason,
		SignalSource: command.SignalSource, TurnRef: command.TurnRef, ActorID: command.Actor.ID,
		PriorMode: priorMode, RuleVersion: casesDomain.ModePolicyVersion, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "propose_mode", ModeTermID: termID, ModeVersion: command.ExpectedModeVersion + 1, ModeScope: command.Scope}
	f.storeOutcome("propose_mode", command.IdempotencyKey, outcome)
	return outcome, nil
}

// ExitMode lowers the conversation mode — the supervised path (the REAL
// authority ladder over the fixture ownership word; the safety mode is
// the supervision word only).
func (f *casesFixture) ExitMode(_ context.Context, command casesAPI.ExitModeCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("exit_mode", casesDomain.GuardExpectedModeVersion(command.ExpectedModeVersion)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := fixtureWorkRule("exit_mode", casesDomain.GuardExitModeMaterial(command.Mode, command.Reason, casesDomain.ModeAuthorityOperatorOwner)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("exit_mode", command.IdempotencyKey, struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		Mode         string `json:"mode"`
		Expected     int64  `json:"expected_mode_version"`
		Reason       string `json:"reason"`
	}{command.CaseID.String(), command.ConversationID.String(), command.Mode, command.ExpectedModeVersion, command.Reason})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if _, err := f.fixtureModeConversation(command.CaseID, command.ConversationID); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	terms := f.modeTerms[command.CaseID]
	active := casesDomain.ActiveConversationMode(terms)
	if err := fixtureWorkRule("exit_mode", casesDomain.GuardExitTransition(command.Mode, active)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if current := casesDomain.ModeTermMaxVersion(terms); current != command.ExpectedModeVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ModeConflictError{
			ActiveMode: casesDomain.EffectiveWorkModeWord(active), Reason: active.Reason,
			SignalSource: active.SignalSource, RuleVersion: casesDomain.ModePolicyVersion, ModeVersion: current,
		}
	}
	// The authority ladder: the fixture operator (the review-scoped owner)
	// is the current owner; the fixture supervisor flag names the
	// configuration word (the /fixture/operator switch).
	state := f.fixtureState(command.CaseID, command.Now)
	isOwner := state.Current != nil && state.Current.OwnerKind == casesDomain.OwnerKindHuman && state.Current.OwnerRef == command.Actor.ID
	supervision := f.supervisor
	if !casesDomain.ExitAuthorityAllowed(casesDomain.EffectiveWorkModeWord(active), isOwner, supervision) {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("exit_mode", fmt.Errorf("%s", casesDomain.ModeExitAuthorityText))
	}
	authority := casesDomain.ModeAuthorityOperatorOwner
	if supervision {
		authority = casesDomain.ModeAuthoritySupervisor
	}
	termID := f.fixtureModeID()
	f.modeTerms[command.CaseID] = append(f.modeTerms[command.CaseID], casesDomain.ModeTerm{
		TenantID: f.tenant, CaseID: command.CaseID, ConversationID: command.ConversationID, TermID: termID,
		ModeVersion: command.ExpectedModeVersion + 1, Mode: command.Mode, Scope: casesDomain.ModeScopeConversation,
		Origin: casesDomain.ModeOriginExitOverride, Reason: command.Reason, SignalSource: casesDomain.ModeSignalOperator,
		ActorID: command.Actor.ID, PriorMode: casesDomain.EffectiveWorkModeWord(active),
		ExitAuthority: authority, RuleVersion: casesDomain.ModePolicyVersion, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "exit_mode", ModeTermID: termID, ModeVersion: command.ExpectedModeVersion + 1, ModeScope: casesDomain.ModeScopeConversation}
	f.storeOutcome("exit_mode", command.IdempotencyKey, outcome)
	return outcome, nil
}

// RecordDisclosure writes one append-only disclosure record (the exact
// text, the channel, the lineage snapshot of the fixture route; the born
// generated event).
func (f *casesFixture) RecordDisclosure(_ context.Context, command casesAPI.RecordDisclosureCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("record_disclosure", casesDomain.GuardDisclosureChannel(command.Channel)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := fixtureWorkRule("record_disclosure", casesDomain.GuardDisclosureText(command.Text)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("record_disclosure", command.IdempotencyKey, struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		Channel      string `json:"channel"`
		Text         string `json:"text"`
		ParentRef    string `json:"parent_ref"`
	}{command.CaseID.String(), command.ConversationID.String(), command.Channel, command.Text, command.ParentRef.String()})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if _, err := f.fixtureModeConversation(command.CaseID, command.ConversationID); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if command.ParentRef.Valid() {
		found := false
		for _, record := range f.disclosures[command.CaseID] {
			if record.DisclosureID == command.ParentRef && record.ConversationID == command.ConversationID {
				found = true
				break
			}
		}
		if !found {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("record_disclosure", fmt.Errorf("родительская запись раскрытия не найдена в диалоге: повторное раскрытие ссылается на запись этого диалога"))
		}
	}
	disclosureID := f.fixtureDisclosureID()
	f.disclosures[command.CaseID] = append(f.disclosures[command.CaseID], casesDomain.DisclosureRecord{
		TenantID: f.tenant, CaseID: command.CaseID, ConversationID: command.ConversationID,
		DisclosureID: disclosureID, DisclosureNo: int64(len(f.disclosures[command.CaseID]) + 1),
		Channel: command.Channel, Text: command.Text, ParentRef: command.ParentRef,
		EmployeeID: f.employee, DecisionNo: 1, ProfileVersion: 1,
		ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	// The born delivery event (generated — the record does not act yet).
	f.deliveryEvents = append(f.deliveryEvents, casesDomain.DisclosureDeliveryEvent{
		TenantID: f.tenant, EventID: f.fixtureDeliveryEventID(), DisclosureID: disclosureID,
		EventNo: 1, Kind: casesDomain.DeliveryGenerated, ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "record_disclosure", DisclosureID: disclosureID, DeliveryKind: casesDomain.DeliveryGenerated}
	f.storeOutcome("record_disclosure", command.IdempotencyKey, outcome)
	return outcome, nil
}

// RecordDisclosureEvent appends one delivery observation (the last event
// is the record's status — the REAL derivation).
func (f *casesFixture) RecordDisclosureEvent(_ context.Context, command casesAPI.RecordDisclosureEventCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("record_disclosure_event", casesDomain.GuardDeliveryKind(command.Kind)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("record_disclosure_event", command.IdempotencyKey, struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		Disclosure   string `json:"disclosure"`
		Kind         string `json:"kind"`
	}{command.CaseID.String(), command.ConversationID.String(), command.DisclosureID.String(), command.Kind})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if _, err := f.fixtureModeConversation(command.CaseID, command.ConversationID); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	found := false
	for _, record := range f.disclosures[command.CaseID] {
		if record.DisclosureID == command.DisclosureID && record.ConversationID == command.ConversationID {
			found = true
			break
		}
	}
	if !found {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	eventNo := int64(1)
	for _, event := range f.deliveryEvents {
		if event.DisclosureID == command.DisclosureID && event.EventNo >= eventNo {
			eventNo = event.EventNo + 1
		}
	}
	f.deliveryEvents = append(f.deliveryEvents, casesDomain.DisclosureDeliveryEvent{
		TenantID: f.tenant, EventID: f.fixtureDeliveryEventID(), DisclosureID: command.DisclosureID,
		EventNo: eventNo, Kind: command.Kind, ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "record_disclosure_event", DisclosureID: command.DisclosureID, DeliveryKind: command.Kind}
	f.storeOutcome("record_disclosure_event", command.IdempotencyKey, outcome)
	return outcome, nil
}

// fixtureModeView assembles the mode section of the case view over the
// REAL domain derivations (the active mode, the item contexts, the
// disclosure cards with their statuses).
func (f *casesFixture) fixtureModeView(caseID id.UUID) casesAPI.ModeView {
	terms := f.modeTerms[caseID]
	view := casesAPI.ModeView{
		Active:      casesDomain.ActiveConversationMode(terms),
		Ladder:      append([]string(nil), casesDomain.ModeOrder...),
		Terms:       append([]casesDomain.ModeTerm(nil), terms...),
		NeutralRule: "Нейтральные ответы разрешены в любом режиме (явное правило mode-policy-v1: безопасная работа по независимой задаче).",
	}
	for _, anchor := range f.items[caseID] {
		if context := casesDomain.WorkItemModeContext(terms, anchor.WorkItemID); context != nil {
			view.ItemContexts = append(view.ItemContexts, casesAPI.ModeItemContextView{WorkItemID: anchor.WorkItemID, Term: *context})
		}
	}
	for _, record := range f.disclosures[caseID] {
		events := []casesDomain.DisclosureDeliveryEvent{}
		for _, event := range f.deliveryEvents {
			if event.DisclosureID == record.DisclosureID {
				events = append(events, event)
			}
		}
		view.Disclosures = append(view.Disclosures, casesPort.DisclosureCard{
			Record: record, Status: casesDomain.DisclosureStatus(events, record.DisclosureID), Events: events,
		})
	}
	view.ForbiddenText = modeForbiddenTextOf(view.Active)
	return view
}

// modeForbiddenTextOf renders the forbidden action words of the mode state
// (the store mirror of the app view word).
func modeForbiddenTextOf(active *casesDomain.ModeTerm) string {
	if active == nil {
		return "Режим не классифицирован (до-миграционные данные): продажные ответы недоступны (безопасный несбытовой путь)."
	}
	if active.Mode == casesDomain.ModeSales {
		return "Продажные ответы открыты: действующее AI-раскрытие канала обязательно до продажного и защищённого ответа."
	}
	return "Продажные ответы недоступны в текущем режиме (" + active.Mode + "): разрешённый путь — явный выход (exit_mode) с полномочием и причиной; нейтральная работа разрешена."
}

// fixtureModeGate applies the REAL mode gate of the client reply: the
// effective mode of the Work Item (or the conversation word) decides the
// sales class.
func (f *casesFixture) fixtureModeGate(command casesAPI.SendClientReplyCommand) error {
	if command.WorkItemID.Valid() {
		if err := f.fixtureModeWorkItem(command.CaseID, command.WorkItemID); err != nil {
			return err
		}
	}
	terms := f.modeTerms[command.CaseID]
	active := casesDomain.ActiveConversationMode(terms)
	context := active
	if command.WorkItemID.Valid() {
		context = casesDomain.WorkItemModeContext(terms, command.WorkItemID)
	}
	if err := casesDomain.GuardReplyClassAgainstMode(command.ReplyClass, casesDomain.EffectiveWorkMode(active, context)); err != nil {
		if gate, ok := err.(casesDomain.ReplyModeError); ok {
			gate.Reason = casesDomain.ReplyModeGateText
			return gate
		}
		return err
	}
	return nil
}

// fixtureDisclosureGate applies the REAL disclosure gate of the sales and
// the protected replies: the acting disclosure of the channel.
func (f *casesFixture) fixtureDisclosureGate(command casesAPI.SendClientReplyCommand, channel string) error {
	if command.ReplyClass != casesDomain.ReplyClassSales && command.ReplyClass != casesDomain.ReplyClassProtectedRequest {
		return nil
	}
	if casesDomain.EffectiveDisclosure(f.disclosures[command.CaseID], f.deliveryEvents, command.ConversationID, channel) == nil {
		return casesDomain.ReplyDisclosureError{Channel: channel, Reason: casesDomain.DisclosureGateText}
	}
	return nil
}

// fixtureReplyChannel resolves the channel word of the reply (the fixture
// mirror of the live route word).
func fixtureReplyChannel(conversation casesDomain.Conversation) string {
	if conversation.ConversationID != id.UUID("") && conversation.Modality != "" {
		return channelOfFixtureModality(conversation.Modality)
	}
	return "phone"
}

// supervisorWord renders the fixture supervisor flag word.
func supervisorWord(active bool) string {
	if active {
		return strconv.FormatBool(active)
	}
	return "false"
}
