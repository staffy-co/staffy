package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// The Story 2.12 commands, the honest timing gate of the reply and the
// degradation view
// ---------------------------------------------------------------------------
//
// Both record commands are idempotent by the tenant-scoped key with the
// stored outcome (the 2.2..2.11 precedent: the replay answer comes BEFORE
// the state guards); the request hash covers the WHOLE command material
// INCLUDING the note and the kind word (the same key with another material
// is the honest key conflict). The rights gate (review OR configuration —
// the M1 precedent of 2.5..2.11) fires BEFORE every material guard and
// refuses with the forbidden word (the honest 403 of the scope sentence);
// the dictionary guards refuse with the honest 400 (the cause word, the
// channel word, the kind word, the note bound — the domain truth before
// SQL); the terminal case refuses honestly. The degradation record is
// append-only with the NEXT monotonic revision of the route under the case
// + conversation locks (the request never carries the number); the store
// route guard requires the route to exist in the conversations of the
// case. The evidence event is append-only with the NEXT monotonic event
// number of the reply under the same locks.

// degradationScopeText is the honest scope refusal of the two record
// commands (the shared «review или configuration» vocabulary of the
// 2.5..2.11 rights gates): the app word travels inside the forbidden
// error only — the transport renders its own sentence for the wire and
// the form.
const degradationScopeText = "Команды записи деградации маршрута и наблюдения доставки требуют полномочия review или configuration: участники только с audit не записывают операционное состояние маршрутов."

// RecordRouteDegradation records one revision of one route degradation of
// the case: the operator with the review OR the configuration scope (the
// gate fires BEFORE the material guards and refuses with the 403 forbidden
// word — an audit-only member never writes), the case anchor, the route
// (the connection id and the channel word of the 2.1 dictionary), the
// cause word, the clearing mark and the operator note. The record is
// append-only with the next monotonic revision of the route (the
// server-side truth under the locks); the store route guard requires the
// route to exist in the conversations of the case — a degradation of a
// foreign route is impossible; the degradations are NEVER facts (no
// confirmation statuses, no fact versions, no facts digest).
func (s *Service) RecordRouteDegradation(ctx context.Context, command casesAPI.RecordRouteDegradationCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration (the M1 precedent); an
	// audit-only member reads the case surface but never records the route
	// state. A non-member is the plain 404 (the scope resolution itself).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, degradationScopeText)
	}
	// The dictionary guard: the channel word of the 2.1 dictionary, the
	// cause word, the note bound (the honest 400 with the dictionary words
	// in the text — the domain truth before SQL; the table CHECK words are
	// the backstops).
	route := casesDomain.RouteKey{ConnectionID: command.ConnectionID, Channel: command.Channel}
	if err := rule("record_route_degradation", casesDomain.GuardRouteDegradationRecord(route, command.Cause, command.Note)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the WHOLE command material INCLUDING the note
	// (the replay of the exact repeat returns the stored outcome; the same
	// key with another material is the honest key conflict).
	hash := requestHash(struct {
		Case       string `json:"case"`
		Connection string `json:"connection_id"`
		Channel    string `json:"channel"`
		Cause      string `json:"cause"`
		Cleared    bool   `json:"cleared"`
		Note       string `json:"note"`
	}{command.CaseID.String(), command.ConnectionID.String(), command.Channel, command.Cause, command.Cleared, command.Note})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_route_degradation", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case anchor: the case must exist in the tenant scope and stay
	// open (the store re-validates under the locks).
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, rule("record_route_degradation", fmt.Errorf("%s", casesStoreTerminalDegradationText))
	}
	// The conversation of the lock plan: the case's own conversation (the
	// SAME sorted plan of the mode/policy commands — the readDegradationsTx
	// read of the evaluation and the record write serialize on the
	// identical key set).
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	recordID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	outcome := casesAPI.WorkOutcome{
		Command: "record_route_degradation", DegradationRecordID: recordID,
		DegradationCause: command.Cause, DegradationCleared: command.Cleared,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_route_degradation", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: conversation.ConversationID,
		DegradationRecordID: recordID, DegradationConnectionID: command.ConnectionID,
		DegradationChannel: command.Channel, DegradationCause: command.Cause,
		DegradationCleared: command.Cleared, DegradationNote: command.Note, CorrelationID: correlation,
		OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// RecordDeliveryEvidence records one delivery evidence event of one
// outgoing reply of the case: the operator with the review OR the
// configuration scope relays the connector observation (the kind word of
// the fixed dictionary — the machine never invents the observation), the
// reply must belong to this case (the store re-validates the reply and
// its conversation under the locks). The event is append-only with the
// next monotonic event number of the reply (the server-side truth under
// the locks); the derived delivery state is the read-time derivation of
// the LAST event — dispatched without events stays the honest unknown.
func (s *Service) RecordDeliveryEvidence(ctx context.Context, command casesAPI.RecordDeliveryEvidenceCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration.
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, degradationScopeText)
	}
	// The dictionary guard: the kind word (the honest 400 with the
	// dictionary words in the text — the domain truth before SQL; the table
	// CHECK words are the backstops).
	if err := rule("record_delivery_evidence", casesDomain.GuardDeliveryEvidence(command.Kind)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the reply reference and the kind word (the
	// same key with another material is the honest key conflict).
	hash := requestHash(struct {
		Case  string `json:"case"`
		Reply string `json:"reply_id"`
		Kind  string `json:"kind"`
	}{command.CaseID.String(), command.ReplyID.String(), command.Kind})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_delivery_evidence", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case anchor: the case must exist and stay open; the reply must
	// belong to this case (the honest 404 of the foreign reply; the
	// conversation of the lock plan comes from the reply row — the store
	// re-validates the match under the locks).
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, rule("record_delivery_evidence", fmt.Errorf("%s", casesStoreTerminalDegradationText))
	}
	replyConversation := id.UUID("")
	replies, err := s.repository.ReadClientReplies(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	for _, card := range replies {
		if card.Reply.ReplyID == command.ReplyID {
			replyConversation = card.Reply.ConversationID
			break
		}
	}
	if !replyConversation.Valid() {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	eventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	outcome := casesAPI.WorkOutcome{
		Command: "record_delivery_evidence", DeliveryEvidenceID: eventID, DeliveryKindWord: command.Kind,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_delivery_evidence", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: replyConversation,
		DeliveryEvidenceEventID: eventID, DeliveryEvidenceReplyID: command.ReplyID,
		DeliveryEvidenceKind: command.Kind, CorrelationID: correlation,
		OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// casesStoreTerminalDegradationText mirrors the store refusal of the
// terminal case (the honest boundary of the write commands — the v1
// dictionary carries the open word only).
const casesStoreTerminalDegradationText = "Терминальное обращение не принимает записей деградации маршрута: обслуживание завершено — записывать нечего."

// degradationStateOf derives the CURRENT route state of one case from the
// read-only rows (the pre-transaction honest read; the store re-derives
// the same truth under the locks of the evaluation and the reply gate).
func (s *Service) degradationStateOf(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesDomain.CurrentDegradations, error) {
	records, err := s.repository.ReadDegradations(ctx, scope, caseID)
	if err != nil {
		return nil, err
	}
	return casesDomain.DeriveCurrentDegradations(records), nil
}

// dialogRouteOf resolves the route of the dialog of one conversation: the
// connection of the conversation and the channel of its latest accepted
// event (the honest live route word; the modality fallback only for a
// conversation without accepted events — the store mirrors the same
// resolution under the locks).
func (s *Service) dialogRouteOf(ctx context.Context, scope tenancy.Context, conversationID id.UUID) (casesDomain.RouteKey, error) {
	conversation, err := s.repository.ReadConversationByID(ctx, scope, conversationID)
	if err != nil {
		return casesDomain.RouteKey{}, err
	}
	channel := channelOfModality(conversation.Modality)
	if live, found, liveErr := s.repository.ReadLiveConversationChannel(ctx, scope, conversationID); liveErr != nil {
		return casesDomain.RouteKey{}, liveErr
	} else if found {
		channel = live
	}
	return casesDomain.RouteKey{ConnectionID: conversation.ConnectionID, Channel: channel}, nil
}

// degradedTimingGate is the app-side honest timing gate of the reply (the
// Story 2.12 word of the frozen ladder: ownership 2.4 → mode 2.5 →
// disclosure 2.5 → policy 2.6 → objection 2.8 → accuracy 2.11 →
// commitment link 2.9 → honest timing 2.12 → CAS; CLASSLESS — the sales,
// the neutral and the protected replies all check: a short failure
// message can be any class): the reply route is degraded → the update
// promise word of the approved dictionary is REQUIRED; the route is
// healthy → the word must stay EMPTY (the byte-identical old path). The
// store re-validates under the locks.
func (s *Service) degradedTimingGate(ctx context.Context, scope tenancy.Context, caseID, conversationID id.UUID, promise string) error {
	degradations, err := s.degradationStateOf(ctx, scope, caseID)
	if err != nil {
		return err
	}
	route, err := s.dialogRouteOf(ctx, scope, conversationID)
	if err != nil {
		return err
	}
	return casesDomain.GuardUpdatePromise(degradations.RouteDegraded(route), promise)
}

// routeDegradationView assembles the «Состояние маршрутов» section of the
// case view: the CURRENT degraded set (the read-time derivation — the
// recovered routes leave the set), the full append-only revision history
// including the recovery rows, the cause dictionary of the record form,
// the channel dictionary of the route and the fixed panel words. The
// reads degrade honestly — the same view scope.
func (s *Service) routeDegradationView(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail) (casesAPI.RouteDegradationView, error) {
	records, err := s.repository.ReadDegradations(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.RouteDegradationView{}, err
	}
	current := casesDomain.DeriveCurrentDegradations(records)
	view := casesAPI.RouteDegradationView{
		Current:      make([]casesAPI.RouteDegradationStateView, 0, len(current)),
		History:      make([]casesAPI.RouteDegradationRevisionView, 0, len(records)),
		CauseWords:   append([]string(nil), casesDomain.DegradationCauses...),
		ChannelWords: append([]string(nil), casesDomain.ChannelOrder...),
		RecordsCount: len(records),
		PanelText:    "Состояние маршрутов — операционная запись сервиса, не факт и не доказательство: append-only ревизии по маршруту (подключение + канал), текущий слот — производная чтения; восстановление — своей ревизией; независимый маршрут не объявляется недоступным без своей строки.",
	}
	for _, key := range current.DegradedRoutes() {
		state := current[key]
		view.Current = append(view.Current, casesAPI.RouteDegradationStateView{
			ConnectionID: state.Route.ConnectionID, Channel: state.Route.Channel,
			Cause: state.Cause, CauseText: casesDomain.DegradationCauseText(state.Cause),
			Revision: state.Revision, RecordedAt: state.RecordedAt,
		})
	}
	// The honest history: every revision row including the recovery ones
	// (the operator surface renders the clearing truth; the append-only
	// audit). The recorded-at time travels from the row.
	for _, record := range records {
		view.History = append(view.History, casesAPI.RouteDegradationRevisionView{
			ConnectionID: record.Route.ConnectionID, Channel: record.Route.Channel,
			Cause: record.Cause, Revision: record.Revision, Cleared: record.Cleared,
			Note: record.Note, ActorID: record.ActorID, RecordedAt: record.RecordedAt,
		})
	}
	return view, nil
}

// replyDeliveriesView derives the honest delivery state of every outgoing
// reply of the case: the dispatched outcome word of the reply (the 2-4
// truth) with the evidence events of the case (the LAST event of every
// reply decides; dispatched without events is the UNKNOWN — never the
// delivered; without the dispatched outcome the state stays prepared).
func (s *Service) replyDeliveriesView(ctx context.Context, scope tenancy.Context, caseID id.UUID, replies []casesPort.ReplyCard) ([]casesAPI.ReplyDeliveryView, error) {
	events, err := s.repository.ReadDeliveryEvents(ctx, scope, caseID)
	if err != nil {
		return nil, err
	}
	byReply := map[id.UUID][]casesDomain.ReplyDeliveryEvent{}
	for _, event := range events {
		byReply[event.ReplyID] = append(byReply[event.ReplyID], event)
	}
	deliveries := make([]casesAPI.ReplyDeliveryView, 0, len(replies))
	for _, card := range replies {
		dispatched := card.OutcomeKind == casesDomain.ReplyOutcomeDispatched
		state := casesDomain.DeriveReplyDeliveryState(dispatched, byReply[card.Reply.ReplyID])
		deliveries = append(deliveries, casesAPI.ReplyDeliveryView{
			ReplyID: card.Reply.ReplyID, State: state.State,
			LastEventNo: state.LastEventNo, LastKind: state.LastKind, LastEventAt: state.LastEventAt,
		})
	}
	return deliveries, nil
}
