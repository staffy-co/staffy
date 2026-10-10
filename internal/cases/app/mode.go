package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// The four mode commands and the mode view (Story 2.5)
// ---------------------------------------------------------------------------
//
// Every command is idempotent by the tenant-scoped key with the stored
// outcome (the 2-2/2-3/2-4 precedent: the replay answer comes BEFORE the
// domain guards); every transition carries the expected mode version —
// the store re-validates it under the case + conversation advisory locks
// (the CAS discipline); every write is append-only. The proposal raises
// only (the deterministic policy refuses the downgrade and the equal
// repeat); the exit is the supervised lowering path; the disclosure gate
// of the sales and the protected replies is the server-side truth.

// modeConversation resolves and validates the conversation of a mode
// command: it must be the case's OWN conversation (the conversation born
// with the case — the unified dialog anchor; the bound conversations of
// other cases are governed by their target case, never by this one).
func (s *Service) modeConversation(ctx context.Context, scope tenancy.Context, commandCase, conversationID id.UUID) (casesDomain.Conversation, error) {
	conversation, err := s.repository.ReadConversationByID(ctx, scope, conversationID)
	if err != nil {
		return casesDomain.Conversation{}, err
	}
	if conversation.CaseID != commandCase {
		return casesDomain.Conversation{}, rule("mode", fmt.Errorf("режим управляется на собственном диалоге обращения: выберите диалог этого обращения"))
	}
	return conversation, nil
}

// modeWorkItem validates the Work Item anchor of a mode or reply command:
// the item must be an anchored work item of the case.
func (s *Service) modeWorkItem(ctx context.Context, scope tenancy.Context, caseID, workItemID id.UUID) error {
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, caseID, []id.UUID{workItemID})
	if err != nil {
		return err
	}
	if _, ok := currents[workItemID]; !ok {
		return rule("mode", fmt.Errorf("элемент работы не найден в обращении: контекст режима привязан к элементу этого обращения"))
	}
	return nil
}

// ProposeMode raises the mode of the conversation or of one Work Item
// context: the deterministic policy decides (the downgrade and the equal
// repeat refuse honestly); the term of the next version lands with the
// prior mode snapshot and the mode_changed outbox event of the same
// transaction. The signal of a turn event answers an accepted event of
// the case; the expected mode version is the CAS word of the ladder.
func (s *Service) ProposeMode(ctx context.Context, command casesAPI.ProposeModeCommand) (casesAPI.WorkOutcome, error) {
	scope, _, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("propose_mode", casesDomain.GuardProposeModeMaterial(command.Scope, command.Mode, command.Reason, command.WorkItemID)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("propose_mode", casesDomain.GuardSignalSource(command.SignalSource, command.TurnRef)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("propose_mode", casesDomain.GuardExpectedModeVersion(command.ExpectedModeVersion)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
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
	if outcome, replay, err := s.lookupCommand(ctx, scope, "propose_mode", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The conversation of the ladder: the case's own conversation.
	if _, err := s.modeConversation(ctx, scope, command.CaseID, command.ConversationID); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The turn reference of a turn signal: an accepted event of the case.
	if command.SignalSource == casesDomain.ModeSignalTurnEvent {
		if _, err := s.repository.ReadEventOfCase(ctx, scope, command.CaseID, command.TurnRef); err != nil {
			return casesAPI.WorkOutcome{}, rule("propose_mode", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
		}
	}
	// The Work Item anchor of the item scope.
	if command.Scope == casesDomain.ModeScopeWorkItem {
		if err := s.modeWorkItem(ctx, scope, command.CaseID, command.WorkItemID); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	terms, err := s.repository.ReadModeMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	active := casesDomain.ActiveConversationMode(terms)
	// The CAS word comes FIRST: the raced twin of the transition gets the
	// honest 409 with the current mode words (the store re-validates the
	// same order under the lock); the ladder decision of the matching
	// version follows.
	if current := casesDomain.ModeTermMaxVersion(terms); current != command.ExpectedModeVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ModeConflictError{
			ActiveMode: casesDomain.EffectiveWorkModeWord(active), Reason: activeReason(active),
			SignalSource: activeSignal(active), RuleVersion: casesDomain.ModePolicyVersion, ModeVersion: current,
		}
	}
	context := active
	if command.Scope == casesDomain.ModeScopeWorkItem {
		context = casesDomain.WorkItemModeContext(terms, command.WorkItemID)
	}
	if err := rule("propose_mode", casesDomain.GuardProposeTransition(command.Scope, command.Mode, active, context)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	termID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "propose_mode", ModeTermID: termID,
		ModeVersion: command.ExpectedModeVersion + 1, ModeScope: command.Scope,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "propose_mode", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: command.ConversationID,
		WorkItemID: command.WorkItemID, Mode: command.Mode, ModeScope: command.Scope,
		ModeTermID: termID, ExpectedModeVersion: command.ExpectedModeVersion,
		Reason: command.Reason, SignalSource: command.SignalSource, TurnRef: command.TurnRef,
		OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// ExitMode lowers the conversation mode — the ONLY lowering path: the
// authority ladder is the server-side word (the current owner with review
// or the configuration supervision; the safety mode is exit-able by the
// supervision only — the platform safety is never weakened by an operator
// override). The exit term carries the authority word, the honest reason
// and the prior mode snapshot.
func (s *Service) ExitMode(ctx context.Context, command casesAPI.ExitModeCommand) (casesAPI.WorkOutcome, error) {
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("exit_mode", casesDomain.GuardExpectedModeVersion(command.ExpectedModeVersion)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The material guard of the target word and the mandatory honest
	// reason (the authority word itself is the server-side resolution
	// below — never a command claim).
	if err := rule("exit_mode", casesDomain.GuardExitModeMaterial(command.Mode, command.Reason, casesDomain.ModeAuthorityOperatorOwner)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		Mode         string `json:"mode"`
		Expected     int64  `json:"expected_mode_version"`
		Reason       string `json:"reason"`
	}{command.CaseID.String(), command.ConversationID.String(), command.Mode, command.ExpectedModeVersion, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "exit_mode", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	if _, err := s.modeConversation(ctx, scope, command.CaseID, command.ConversationID); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	terms, err := s.repository.ReadModeMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	active := casesDomain.ActiveConversationMode(terms)
	// The CAS word comes FIRST (the raced twin gets the honest 409; the
	// store re-validates the same order under the lock).
	if current := casesDomain.ModeTermMaxVersion(terms); current != command.ExpectedModeVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ModeConflictError{
			ActiveMode: casesDomain.EffectiveWorkModeWord(active), Reason: activeReason(active),
			SignalSource: activeSignal(active), RuleVersion: casesDomain.ModePolicyVersion, ModeVersion: current,
		}
	}
	if err := rule("exit_mode", casesDomain.GuardExitTransition(command.Mode, active)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The authority ladder: the current ownership word decides the owner
	// side; the configuration scope decides the supervision side. The
	// safety mode is the supervision word ONLY (AC-9).
	ownershipTerms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current := casesDomain.CurrentOwnership(ownershipTerms)
	isOwner := current != nil && current.OwnerKind == casesDomain.OwnerKindHuman && current.OwnerRef == command.Actor.ID
	if !casesDomain.ExitAuthorityAllowed(casesDomain.EffectiveWorkModeWord(active), isOwner, rights.Configuration) {
		return casesAPI.WorkOutcome{}, rule("exit_mode", fmt.Errorf("%s", casesDomain.ModeExitAuthorityText))
	}
	authority := casesDomain.ModeAuthorityOperatorOwner
	if rights.Configuration {
		authority = casesDomain.ModeAuthoritySupervisor
	}
	termID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "exit_mode", ModeTermID: termID,
		ModeVersion: command.ExpectedModeVersion + 1, ModeScope: casesDomain.ModeScopeConversation,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "exit_mode", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: command.ConversationID,
		Mode: command.Mode, ModeScope: casesDomain.ModeScopeConversation, ModeTermID: termID,
		ExpectedModeVersion: command.ExpectedModeVersion, ExitAuthority: authority,
		Reason: command.Reason, SignalSource: casesDomain.ModeSignalOperator,
		OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// RecordDisclosure writes one append-only AI disclosure record: the exact
// bounded text, the channel, the release lineage snapshot of the acting
// decision (the reply precedent) and — for the re-disclosure — the parent
// reference of the voided record. The record is born with its generated
// delivery event and the disclosure_recorded outbox record of the same
// transaction; the send observation makes it act for the channel.
func (s *Service) RecordDisclosure(ctx context.Context, command casesAPI.RecordDisclosureCommand) (casesAPI.WorkOutcome, error) {
	scope, _, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("record_disclosure", casesDomain.GuardDisclosureChannel(command.Channel)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("record_disclosure", casesDomain.GuardDisclosureText(command.Text)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		Channel      string `json:"channel"`
		Text         string `json:"text"`
		ParentRef    string `json:"parent_ref"`
	}{command.CaseID.String(), command.ConversationID.String(), command.Channel, command.Text, command.ParentRef.String()})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_disclosure", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	conversation, err := s.modeConversation(ctx, scope, command.CaseID, command.ConversationID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The parent reference of the re-disclosure: a record of the same
	// conversation (the voided word the operator repeats).
	if command.ParentRef.Valid() {
		cards, err := s.repository.ReadDisclosures(ctx, scope, command.CaseID)
		if err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		found := false
		for _, card := range cards {
			if card.Record.DisclosureID == command.ParentRef && card.Record.ConversationID == command.ConversationID {
				found = true
				break
			}
		}
		if !found {
			return casesAPI.WorkOutcome{}, rule("record_disclosure", fmt.Errorf("родительская запись раскрытия не найдена в диалоге: повторное раскрытие ссылается на запись этого диалога"))
		}
	}
	// The release lineage snapshot of the acting decision (the reply
	// precedent — the honest re-read, never a command claim).
	employee, decisionNo, profileVersion, err := s.lineageWord(ctx, scope, conversation)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	disclosureID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	eventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "record_disclosure", DisclosureID: disclosureID, DeliveryKind: casesDomain.DeliveryGenerated,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_disclosure", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: command.ConversationID,
		DisclosureID: disclosureID, ParentRef: command.ParentRef, DisclosureText: command.Text,
		Channel: command.Channel, LineageEmployeeID: employee, LineageDecisionNo: decisionNo,
		LineageProfileVersion: profileVersion, DeliveryKind: casesDomain.DeliveryGenerated,
		DeliveryEventID: eventID, OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// RecordDisclosureEvent appends one delivery observation of a disclosure
// record: sent, a playback word, the customer confirmation or the
// interruption that voids the acting disclosure. One event — one kind;
// the last event (the maximal per-record number) is the record's status.
func (s *Service) RecordDisclosureEvent(ctx context.Context, command casesAPI.RecordDisclosureEventCommand) (casesAPI.WorkOutcome, error) {
	scope, _, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("record_disclosure_event", casesDomain.GuardDeliveryKind(command.Kind)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		Disclosure   string `json:"disclosure"`
		Kind         string `json:"kind"`
	}{command.CaseID.String(), command.ConversationID.String(), command.DisclosureID.String(), command.Kind})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_disclosure_event", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	if _, err := s.modeConversation(ctx, scope, command.CaseID, command.ConversationID); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The disclosure record must exist and belong to this conversation.
	cards, err := s.repository.ReadDisclosures(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	found := false
	for _, card := range cards {
		if card.Record.DisclosureID == command.DisclosureID && card.Record.ConversationID == command.ConversationID {
			found = true
			break
		}
	}
	if !found {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	eventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "record_disclosure_event", DisclosureID: command.DisclosureID, DeliveryKind: command.Kind,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_disclosure_event", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: command.ConversationID,
		DisclosureID: command.DisclosureID, DeliveryKind: command.Kind, DeliveryEventID: eventID,
		Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// activeReason resolves the honest reason word of the active mode term
// (the empty word of the pre-migration absence).
func activeReason(active *casesDomain.ModeTerm) string {
	if active == nil {
		return ""
	}
	return active.Reason
}

// activeSignal resolves the signal source word of the active term.
func activeSignal(active *casesDomain.ModeTerm) string {
	if active == nil {
		return ""
	}
	return active.SignalSource
}

// modeView assembles the mode section of the case view: the active
// conversation mode with its honest words, the ladder, the term history,
// the Work Item contexts, the forbidden action words of the current state
// and the disclosure records with their delivery statuses. The reads
// degrade honestly — an unreadable section never fails the case view.
func (s *Service) modeView(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail) (casesAPI.ModeView, error) {
	terms, err := s.repository.ReadModeMaterial(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.ModeView{}, err
	}
	disclosures, err := s.repository.ReadDisclosures(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.ModeView{}, err
	}
	view := casesAPI.ModeView{
		Active: casesDomain.ActiveConversationMode(terms),
		Ladder: append([]string(nil), casesDomain.ModeOrder...),
		Terms:  terms, Disclosures: disclosures,
		NeutralRule: "Нейтральные ответы разрешены в любом режиме (явное правило mode-policy-v1: безопасная работа по независимой задаче).",
	}
	// The Work Item contexts of the case's items (the items without a
	// context stay absent — the honest word).
	for _, item := range detail.WorkItems {
		if context := casesDomain.WorkItemModeContext(terms, item.WorkItemID); context != nil {
			view.ItemContexts = append(view.ItemContexts, casesAPI.ModeItemContextView{WorkItemID: item.WorkItemID, Term: *context})
		}
	}
	view.ForbiddenText = modeForbiddenText(view.Active)
	return view, nil
}

// modeForbiddenText renders the forbidden action words of the current
// mode state (the honest text, never a color).
func modeForbiddenText(active *casesDomain.ModeTerm) string {
	if active == nil {
		return "Режим не классифицирован (до-миграционные данные): продажные ответы недоступны (безопасный несбытовой путь)."
	}
	if active.Mode == casesDomain.ModeSales {
		return "Продажные ответы открыты: действующее AI-раскрытие канала обязательно до продажного и защищённого ответа."
	}
	return "Продажные ответы недоступны в текущем режиме (" + active.Mode + "): разрешённый путь — явный выход (exit_mode) с полномочием и причиной; нейтральная работа разрешена."
}
