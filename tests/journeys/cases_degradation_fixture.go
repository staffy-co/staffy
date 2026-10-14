package main

import (
	"context"
	"fmt"
	"sort"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.12 fixture: the two record commands over the REAL domain
// guards (GuardRouteDegradationRecord — the channel of the 2.1 dictionary,
// the cause word, the note bound; GuardDeliveryEvidence — the kind word)
// and the REAL read-time derivations (DeriveCurrentDegradations — the
// latest revision of every route wins, a clearing revision restores the
// route; DeriveReplyDeliveryState — the last evidence event decides,
// dispatched without events is the honest unknown, never the delivered;
// DegradationSnapshotOf — the canonical route map of the policy input
// snapshot). The fixture world stores the append-only revision rows and
// the append-only evidence events; the CURRENT slots and the delivery
// states are never stored — every read derives them anew (the mirror of
// the store). The revision and the event numbers are the fixture-owned
// monotonic truths (the requests never carry them). The request hashes
// cover the WHOLE command materials INCLUDING the note and the kind word:
// the exact repeats answer the stored outcomes (the replays), the same
// keys with other materials are the honest conflicts.

// fixtureDegradationID generates the next deterministic degradation
// record id (the counter space of the Story 2.12 surface — the ids never
// collide with the other fixture surfaces).
func (f *casesFixture) fixtureDegradationID() id.UUID {
	f.degradationSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(99800+f.degradationSeq)))
}

// fixtureDegradationCurrent derives the read-time route state of one case
// over the REAL derivation (the latest revision per route; the cleared
// routes leave the degraded set but stay in the state with the honest
// mark — the snapshot truth of the policy input).
func (f *casesFixture) fixtureDegradationCurrent(caseID id.UUID) casesDomain.CurrentDegradations {
	return casesDomain.DeriveCurrentDegradations(f.degradations[caseID])
}

// fixtureDialogRoute resolves the route of the dialog of one
// conversation: the connection of the conversation and its channel word
// (the modality derivation of the fixture reply channel — the mirror of
// the store live route).
func (f *casesFixture) fixtureDialogRoute(conversationID id.UUID) casesDomain.RouteKey {
	caseID, known := f.caseOfConversation(conversationID)
	if !known {
		return casesDomain.RouteKey{}
	}
	conversation, _ := f.conversationOf(caseID, conversationID)
	return casesDomain.RouteKey{ConnectionID: conversation.ConnectionID, Channel: fixtureReplyChannel(conversation)}
}

// fixtureRouteExists guards the record: the route (the connection and the
// channel word) must exist in the conversations of the case — a
// degradation of a foreign route is impossible (the store route guard
// mirror over the fixture conversations).
func (f *casesFixture) fixtureRouteExists(caseID, connection id.UUID, channel string) bool {
	material := f.material[caseID]
	if material == nil {
		return false
	}
	for _, conversation := range material.conversations {
		if conversation.ConnectionID == connection && fixtureReplyChannel(conversation) == channel {
			return true
		}
	}
	return false
}

// RecordRouteDegradation records one revision of one route degradation of
// the case: the REAL dictionary guard before any write (the channel word,
// the cause word, the note bound — the domain truth, never a convenient
// default), the idempotent command flow with the hash of the WHOLE
// material (the note included), the honest refusal of an unknown or a
// terminal case, the ROUTE GUARD (the route exists in the conversations
// of the case) and the append-only landing of the NEXT monotonic revision
// of the route (the request never carries the number). The fixture actor
// is the review-scoped principal of the journey world — the scope gate of
// the production contour passes by construction; the audit-only leg lives
// in the integration matrix.
func (f *casesFixture) RecordRouteDegradation(_ context.Context, command casesAPI.RecordRouteDegradationCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if command.TenantID != f.tenant {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	route := casesDomain.RouteKey{ConnectionID: command.ConnectionID, Channel: command.Channel}
	if err := fixtureWorkRule("record_route_degradation", casesDomain.GuardRouteDegradationRecord(route, command.Cause, command.Note)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the WHOLE material INCLUDING the note.
	outcome, fresh, err := f.workCommandFlow("record_route_degradation", command.IdempotencyKey, struct {
		Case       string `json:"case"`
		Connection string `json:"connection_id"`
		Channel    string `json:"channel"`
		Cause      string `json:"cause"`
		Cleared    bool   `json:"cleared"`
		Note       string `json:"note"`
	}{command.CaseID.String(), command.ConnectionID.String(), command.Channel, command.Cause, command.Cleared, command.Note})
	if err != nil || !fresh {
		return outcome, err
	}
	material := f.material[command.CaseID]
	if material == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if material.cases.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_route_degradation", fmt.Errorf("Терминальное обращение не принимает записей деградации маршрута: обслуживание завершено — записывать нечего."))
	}
	// The route guard: the route exists in the conversations of the case.
	if !f.fixtureRouteExists(command.CaseID, command.ConnectionID, command.Channel) {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_route_degradation", fmt.Errorf("Маршрут (подключение, канал) не существует в разговорах этого обращения: деградация чужого маршрута невозможна — проверьте подключение и канал."))
	}
	// The NEXT monotonic revision of the route — the fixture-owned truth
	// under the world lock.
	revision := int64(0)
	for _, record := range f.degradations[command.CaseID] {
		if record.Route == route && record.Revision > revision {
			revision = record.Revision
		}
	}
	revision++
	recordID := f.fixtureDegradationID()
	f.degradations[command.CaseID] = append(f.degradations[command.CaseID], casesDomain.RouteDegradationRecord{
		TenantID: f.tenant, CaseID: command.CaseID, RecordID: recordID,
		Route: route, Cause: command.Cause, Revision: revision,
		Cleared: command.Cleared, Note: command.Note,
		ActorID: command.Actor.ID, RecordedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{
		Command: "record_route_degradation", DegradationRecordID: recordID,
		DegradationCause: command.Cause, DegradationCleared: command.Cleared,
	}
	f.storeOutcome("record_route_degradation", command.IdempotencyKey, outcome)
	return outcome, nil
}

// RecordDeliveryEvidence records one delivery evidence event of one
// outgoing reply of the case: the REAL dictionary guard (the kind word),
// the idempotent command flow with the hash of the reply reference and
// the kind word, the honest refusal of an unknown or a terminal case, the
// reply guard (the reply belongs to this case) and the append-only
// landing of the NEXT monotonic event number of the reply.
func (f *casesFixture) RecordDeliveryEvidence(_ context.Context, command casesAPI.RecordDeliveryEvidenceCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if command.TenantID != f.tenant {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if err := fixtureWorkRule("record_delivery_evidence", casesDomain.GuardDeliveryEvidence(command.Kind)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the reply reference and the kind word.
	outcome, fresh, err := f.workCommandFlow("record_delivery_evidence", command.IdempotencyKey, struct {
		Case  string `json:"case"`
		Reply string `json:"reply_id"`
		Kind  string `json:"kind"`
	}{command.CaseID.String(), command.ReplyID.String(), command.Kind})
	if err != nil || !fresh {
		return outcome, err
	}
	material := f.material[command.CaseID]
	if material == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if material.cases.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_delivery_evidence", fmt.Errorf("Терминальное обращение не принимает записей деградации маршрута: обслуживание завершено — записывать нечего."))
	}
	// The reply guard: the reply belongs to this case.
	replyKnown := false
	for _, reply := range f.replies[command.CaseID] {
		if reply.ReplyID == command.ReplyID {
			replyKnown = true
			break
		}
	}
	if !replyKnown {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	// The NEXT monotonic event number of the reply — the fixture-owned
	// truth under the world lock.
	eventNo := int64(0)
	for _, event := range f.replyDeliveryEvents[command.CaseID] {
		if event.ReplyID == command.ReplyID && event.EventNo > eventNo {
			eventNo = event.EventNo
		}
	}
	eventNo++
	eventID := f.fixtureDegradationID()
	f.replyDeliveryEvents[command.CaseID] = append(f.replyDeliveryEvents[command.CaseID], casesDomain.ReplyDeliveryEvent{
		TenantID: f.tenant, CaseID: command.CaseID, ReplyID: command.ReplyID,
		EventID: eventID, EventNo: eventNo, Kind: command.Kind,
		ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{
		Command: "record_delivery_evidence", DeliveryEvidenceID: eventID, DeliveryKindWord: command.Kind,
	}
	f.storeOutcome("record_delivery_evidence", command.IdempotencyKey, outcome)
	return outcome, nil
}

// fixtureTimingGate is the honest timing gate of the Story 2.12 (the
// app degradedTimingGate mirror — the frozen ladder: ... → commitment
// link 2-9 → honest timing 2-12 → write; CLASSLESS): the reply route is
// degraded → the promise word of the approved dictionary is REQUIRED;
// healthy → the word must stay empty (the field has exactly one
// meaning).
func (f *casesFixture) fixtureTimingGate(command casesAPI.SendClientReplyCommand) error {
	route := f.fixtureDialogRoute(command.ConversationID)
	return casesDomain.GuardUpdatePromise(f.fixtureDegradationCurrent(command.CaseID).RouteDegraded(route), command.UpdatePromise)
}

// fixtureDegradationView assembles the «Состояние маршрутов» section of
// the case view (the app routeDegradationView mirror): the CURRENT set
// derives read-time over the REAL derivation, the full append-only
// revision history in the canonical order, the cause dictionary of the
// record form, the channel dictionary of the route and the fixed panel
// words.
func (f *casesFixture) fixtureDegradationView(caseID id.UUID) casesAPI.RouteDegradationView {
	records := append([]casesDomain.RouteDegradationRecord(nil), f.degradations[caseID]...)
	sort.Slice(records, func(i, j int) bool {
		if records[i].Route.ConnectionID != records[j].Route.ConnectionID {
			return records[i].Route.ConnectionID < records[j].Route.ConnectionID
		}
		if records[i].Route.Channel != records[j].Route.Channel {
			return records[i].Route.Channel < records[j].Route.Channel
		}
		return records[i].Revision < records[j].Revision
	})
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
			Revision: state.Revision,
		})
	}
	for _, record := range records {
		view.History = append(view.History, casesAPI.RouteDegradationRevisionView{
			ConnectionID: record.Route.ConnectionID, Channel: record.Route.Channel,
			Cause: record.Cause, Revision: record.Revision, Cleared: record.Cleared,
			Note: record.Note, ActorID: record.ActorID, RecordedAt: record.RecordedAt,
		})
	}
	return view
}

// fixtureDeliveries derives the honest delivery states of the outgoing
// replies of the case (the app replyDeliveriesView mirror): the
// dispatched outcome word of every reply with the evidence events — the
// LAST event decides; dispatched without events is the UNKNOWN; without
// the dispatched outcome the state stays prepared.
func (f *casesFixture) fixtureDeliveries(caseID id.UUID) []casesAPI.ReplyDeliveryView {
	eventsOfReply := map[id.UUID][]casesDomain.ReplyDeliveryEvent{}
	for _, event := range f.replyDeliveryEvents[caseID] {
		eventsOfReply[event.ReplyID] = append(eventsOfReply[event.ReplyID], event)
	}
	deliveries := make([]casesAPI.ReplyDeliveryView, 0, len(f.replies[caseID]))
	for _, reply := range f.replies[caseID] {
		outcome, decided := f.replyOutcomes[reply.ReplyID]
		dispatched := decided && outcome.Kind == casesDomain.ReplyOutcomeDispatched
		state := casesDomain.DeriveReplyDeliveryState(dispatched, eventsOfReply[reply.ReplyID])
		deliveries = append(deliveries, casesAPI.ReplyDeliveryView{
			ReplyID: reply.ReplyID, State: state.State,
			LastEventNo: state.LastEventNo, LastKind: state.LastKind, LastEventAt: state.LastEventAt,
		})
	}
	return deliveries
}
