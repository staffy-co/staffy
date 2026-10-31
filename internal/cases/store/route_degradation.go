package store

import (
	"context"
	"errors"
	"time"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 2.12: the route degradation writes, the delivery evidence writes
// and the case reads
// ---------------------------------------------------------------------------

// TerminalCaseDegradationText is the honest refusal of the record on a
// terminal case (the v1 backstop: the case-state dictionary carries only
// open today, the future closure words arrive with their own stories —
// the reserved-word precedent of mandatory_missing).
const TerminalCaseDegradationText = "Терминальное обращение не принимает записей деградации маршрута: обслуживание завершено — записывать нечего."

// DegradationRouteForeignText is the honest refusal of the route guard:
// the route (connection, channel) must exist in the conversations of the
// case — a degradation of a foreign route is impossible (the
// «независимые маршруты не объявляются недоступными без доказательства»
// structural backstop of AC-1).
const DegradationRouteForeignText = "Маршрут (подключение, канал) не существует в разговорах этого обращения: деградация чужого маршрута невозможна — проверьте подключение и канал."

// EvidenceReplyForeignText is the honest refusal of the reply guard of
// the delivery evidence: the reply must belong to the case under the
// locks (the operator relays the observation of THIS case's outgoing
// reply only).
const EvidenceReplyForeignText = "Ответ не принадлежит этому обращению: наблюдение доставки живёт на исходящем ответе этого обращения."

// mapDegradationInsertError maps the raced insert conflicts of the two
// degradation surfaces onto the honest domain errors: the command record
// UNIQUE is the material idempotency conflict; the revision and event
// number UNIQUEs are the raced derivations (the loser of the
// advisory-lock race sees the winner's committed number — the honest
// stale-retry signal, the case_accommodations precedent).
func mapDegradationInsertError(err error, command string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "route_degradations_pkey":
			// The raced identical record (the same record id under the same
			// command key): the honest stale-retry signal.
			return casesDomain.ErrStaleVersion
		case "route_degradations_tenant_id_case_id_connection_id_channel_revision_key":
			// The raced revision of the same route: the loser re-derives under
			// the locks and retries (the advisory locks serialize the normal
			// path — the UNIQUE is the defensive backstop).
			return casesDomain.ErrStaleVersion
		case "client_reply_delivery_events_pkey":
			return casesDomain.ErrStaleVersion
		case "client_reply_delivery_events_tenant_id_reply_id_event_no_key":
			// The raced event number of the same reply: the loser re-derives
			// under the locks and retries.
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

// scanRouteDegradationRecord reads one degradation revision row.
func scanRouteDegradationRecord(scan func(dest ...any) error, tenantID id.UUID) (casesDomain.RouteDegradationRecord, error) {
	var record casesDomain.RouteDegradationRecord
	err := scan(&record.RecordID, &record.CaseID, &record.Route.ConnectionID, &record.Route.Channel,
		&record.Cause, &record.Revision, &record.Cleared, &record.Note, &record.ActorID, &record.RecordedAt)
	if err != nil {
		return record, err
	}
	record.TenantID = tenantID
	record.RecordedAt = record.RecordedAt.UTC()
	return record, nil
}

// readDegradationsTx reads every degradation revision record of one case
// ordered by the route key and the monotonic revision (the derivation
// input of the CURRENT slots and the honest panel history). The read is
// the lock-visible truth under the SAME advisory locks of the evaluation
// (the readDegradationsTx mirror of readAccommodationsTx — the 2.12
// fifth input is never a raced snapshot).
func readDegradationsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.RouteDegradationRecord, error) {
	rows, err := tx.Query(ctx, `
                select record_id, case_id, connection_id, channel, cause, revision, cleared, note, actor_id, recorded_at
                from cases.route_degradations
                where tenant_id=$1 and case_id=$2 order by connection_id, channel, revision`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []casesDomain.RouteDegradationRecord{}
	for rows.Next() {
		record, err := scanRouteDegradationRecord(rows.Scan, tenantID)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// ReadDegradations reads every append-only route degradation record of
// one case (the own read transaction of the panel, the transfer view and
// the policy wiring input — the CURRENT slots are the read-time domain
// derivation, never a stored status).
func (s *Postgres) ReadDegradations(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.RouteDegradationRecord, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	records, err := readDegradationsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return records, tx.Commit(ctx)
}

// scanReplyDeliveryEvent reads one delivery evidence event row.
func scanReplyDeliveryEvent(scan func(dest ...any) error, tenantID id.UUID) (casesDomain.ReplyDeliveryEvent, error) {
	var event casesDomain.ReplyDeliveryEvent
	err := scan(&event.EventID, &event.CaseID, &event.ReplyID, &event.EventNo,
		&event.Kind, &event.ActorID, &event.CreatedAt)
	if err != nil {
		return event, err
	}
	event.TenantID = tenantID
	event.CreatedAt = event.CreatedAt.UTC()
	return event, nil
}

// readDeliveryEventsTx reads every delivery evidence event of the replies
// of one case ordered by the reply and the monotonic event number (the
// derivation input of the honest delivery states — the LAST event of
// every reply decides, the history is never rewritten).
func readDeliveryEventsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.ReplyDeliveryEvent, error) {
	rows, err := tx.Query(ctx, `
                select event_id, case_id, reply_id, event_no, kind, actor_id, created_at
                from cases.client_reply_delivery_events
                where tenant_id=$1 and case_id=$2 order by reply_id, event_no`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []casesDomain.ReplyDeliveryEvent{}
	for rows.Next() {
		event, err := scanReplyDeliveryEvent(rows.Scan, tenantID)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// ReadDeliveryEvents reads every append-only delivery evidence event of
// the replies of one case (the own read transaction of the reply panel —
// the delivery states are the read-time domain derivation, never a
// stored status).
func (s *Postgres) ReadDeliveryEvents(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ReplyDeliveryEvent, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	events, err := readDeliveryEventsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return events, tx.Commit(ctx)
}

// execRecordRouteDegradation writes one degradation revision record
// under the case + conversation locks: the case must exist under the
// tenant scope (the honest 404 of the non-participant path) and stay
// open (the terminal backstop), the domain guard re-validates under the
// locks (the channel of the 2.1 dictionary, the cause word, the note
// bound — the table CHECK words are the backstops), the ROUTE GUARD
// requires the (connection, channel) pair to exist in the conversations
// of the case (a degradation of a foreign route is impossible), the
// revision is the NEXT monotonic number of the route derived under the
// locks (the request never carries it), and the outbox event carries the
// metadata only (the route words, the cause, the revision and the
// clearing mark — the NOTE never enters the envelope).
func (s *Postgres) execRecordRouteDegradation(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	var caseState string
	err := tx.QueryRow(ctx, `
                select state from cases.cases where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&caseState)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if caseState != casesDomain.CaseStateOpen {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: TerminalCaseDegradationText}
	}
	route := casesDomain.RouteKey{ConnectionID: p.DegradationConnectionID, Channel: p.DegradationChannel}
	// The domain guard under the locks (the honest 400 backstopped by the
	// table CHECK words).
	if err := casesDomain.GuardRouteDegradationRecord(route, p.DegradationCause, p.DegradationNote); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The route guard: the (connection, channel) pair exists in the
	// conversations of the case — an accepted event of the case carries
	// the exact route (the conversations of the case own the connection;
	// the events of the conversations own the channel word). A route of
	// another case never passes; an invented channel never passes the
	// dictionary.
	var routeExists bool
	if err := tx.QueryRow(ctx, `
                select exists(
                        select 1 from cases.channel_events e
                        join cases.conversations c on c.tenant_id = e.tenant_id and c.conversation_id = e.conversation_id
                        where e.tenant_id=$1 and c.case_id=$2 and e.connection_id=$3 and e.channel=$4
                                and e.state='accepted')`,
		tenantID, p.CaseID, route.ConnectionID, route.Channel).Scan(&routeExists); err != nil {
		return err
	}
	if !routeExists {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: DegradationRouteForeignText}
	}
	// The revision: the next monotonic number of the route under the locks
	// (the request never carries it; the UNIQUE backstop catches the race
	// the advisory locks already serialize).
	var revision int64
	if err := tx.QueryRow(ctx, `
                select coalesce(max(revision),0)+1 from cases.route_degradations
                where tenant_id=$1 and case_id=$2 and connection_id=$3 and channel=$4`,
		tenantID, p.CaseID, route.ConnectionID, route.Channel).Scan(&revision); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.route_degradations (tenant_id, case_id, record_id, connection_id, channel, cause, revision, cleared, note, actor_id, recorded_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.CaseID, p.DegradationRecordID, route.ConnectionID, route.Channel,
		p.DegradationCause, revision, p.DegradationCleared, p.DegradationNote, p.Actor, now); err != nil {
		return mapDegradationInsertError(err, p.Command)
	}
	// The outbox event: the metadata only — the NOTE never enters the
	// envelope (the operational rows stay in the table).
	payload, err := casesDomain.RouteDegradationRecordedPayloadJSON(casesDomain.RouteDegradationRecordedPayload{
		TenantID: tenantID, CaseID: p.CaseID, RecordID: p.DegradationRecordID,
		ConnectionID: route.ConnectionID, Channel: route.Channel,
		Cause: p.DegradationCause, Revision: revision, Cleared: p.DegradationCleared,
		ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventRouteDegradationRecorded, casesDomain.OutboxEventRouteDegradationRecordedV1,
		p.CaseID, route.ConnectionID, p.CorrelationID, now, payload,
		casesDomain.DedupKeyRouteDegradationRecorded(p.DegradationRecordID), now); err != nil {
		return mapDegradationInsertError(err, p.Command)
	}
	return nil
}

// execRecordDeliveryEvidence writes one delivery evidence event under
// the case + conversation locks: the case must exist under the tenant
// scope and stay open, the domain guard re-validates the kind word under
// the locks (the table CHECK is the backstop), the REPLY GUARD requires
// the reply to belong to this case with the conversation of the lock
// plan (the honest 404 of a foreign reply), the event number is the NEXT
// monotonic number of the reply derived under the locks (the request
// never carries it), and the outbox event carries the metadata only
// (the reply reference, the event number and the kind word — the reply
// CONTENT never enters the envelope).
func (s *Postgres) execRecordDeliveryEvidence(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	var caseState string
	err := tx.QueryRow(ctx, `
                select state from cases.cases where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&caseState)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if caseState != casesDomain.CaseStateOpen {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: TerminalCaseDegradationText}
	}
	// The domain guard under the locks (the honest 400 backstopped by the
	// table CHECK words).
	if err := casesDomain.GuardDeliveryEvidence(p.DeliveryEvidenceKind); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The reply guard: the reply belongs to this case and its conversation
	// matches the lock-plan conversation (the honest 404/400 of the foreign
	// reply; the evidence of another case's reply is impossible).
	var replyConversation, replyConnection id.UUID
	err = tx.QueryRow(ctx, `
                select conversation_id, connection_id from cases.client_replies
                where tenant_id=$1 and case_id=$2 and reply_id=$3`,
		tenantID, p.CaseID, p.DeliveryEvidenceReplyID).Scan(&replyConversation, &replyConnection)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if replyConversation != p.ConversationID {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: EvidenceReplyForeignText}
	}
	// The event number: the next monotonic number of the reply under the
	// locks (the request never carries it; the UNIQUE backstop catches the
	// race the advisory locks already serialize).
	var eventNo int64
	if err := tx.QueryRow(ctx, `
                select coalesce(max(event_no),0)+1 from cases.client_reply_delivery_events
                where tenant_id=$1 and reply_id=$2`,
		tenantID, p.DeliveryEvidenceReplyID).Scan(&eventNo); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.client_reply_delivery_events (tenant_id, case_id, reply_id, event_id, event_no, kind, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8)`,
		tenantID, p.CaseID, p.DeliveryEvidenceReplyID, p.DeliveryEvidenceEventID, eventNo,
		p.DeliveryEvidenceKind, p.Actor, now); err != nil {
		return mapDegradationInsertError(err, p.Command)
	}
	// The outbox event: the metadata only — the reply CONTENT never enters
	// the envelope.
	payload, err := casesDomain.ClientReplyDeliveryEvidencePayloadJSON(casesDomain.ClientReplyDeliveryEvidencePayload{
		TenantID: tenantID, CaseID: p.CaseID, ReplyID: p.DeliveryEvidenceReplyID,
		EventID: p.DeliveryEvidenceEventID, EventNo: eventNo, Kind: p.DeliveryEvidenceKind,
		ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventClientReplyDeliveryEvidence, casesDomain.OutboxEventClientReplyDeliveryEvidenceV1,
		p.CaseID, replyConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyClientReplyDeliveryEvidence(p.DeliveryEvidenceEventID), now); err != nil {
		return mapDegradationInsertError(err, p.Command)
	}
	return nil
}

// dialogRouteOfTx resolves the route of the dialog of one conversation
// under the locks: the connection of the conversation and the channel of
// its latest accepted event (the honest live route word; the modality
// fallback only for a conversation without accepted events — the mirror
// of the app dialogRouteOf and the reply route snapshot resolution).
func dialogRouteOfTx(ctx context.Context, tx pgx.Tx, tenantID, conversationID id.UUID) (casesDomain.RouteKey, error) {
	var connection id.UUID
	if err := tx.QueryRow(ctx, `
		select connection_id from cases.conversations
		where tenant_id=$1 and conversation_id=$2`,
		tenantID, conversationID).Scan(&connection); err != nil {
		return casesDomain.RouteKey{}, err
	}
	channel := "web_chat"
	if err := tx.QueryRow(ctx, `
		select channel from cases.channel_events
		where tenant_id=$1 and conversation_id=$2 and state='accepted'
		order by received_at desc limit 1`,
		tenantID, conversationID).Scan(&channel); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return casesDomain.RouteKey{}, err
		}
		// No accepted event (the defensive corner — the ingest creates the
		// conversation WITH its first accepted event): the fallback mirrors
		// the reply route snapshot word of the app (channelOfModality of the
		// empty conversation word).
		channel = "web_chat"
	}
	return casesDomain.RouteKey{ConnectionID: connection, Channel: channel}, nil
}

// replyUpdatePromiseGateOfTx re-validates the honest timing gate of the
// reply under the locks (the Story 2.12 word of the frozen ladder —
// CLASSLESS): the reply route (the connection of the conversation and the
// resolved live channel) is degraded → the update promise word of the
// approved dictionary is REQUIRED; healthy → the word must stay empty
// (the app validated pre-transaction; the store re-validates the
// server-side truth under the case + conversation locks).
func replyUpdatePromiseGateOfTx(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, connection id.UUID, channel string) error {
	records, err := readDegradationsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	current := casesDomain.DeriveCurrentDegradations(records)
	route := casesDomain.RouteKey{ConnectionID: connection, Channel: channel}
	return casesDomain.GuardUpdatePromise(current.RouteDegraded(route), p.UpdatePromise)
}
