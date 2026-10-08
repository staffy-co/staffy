package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is the cases persistence: every table is tenant-scoped with
// RLS; the admission write is one transaction over the advisory lock of
// the material identity; the reads canonicalize timestamps to UTC at the
// boundary (the R2/N1 lesson of 1-14).
type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

type row interface{ Scan(dest ...any) error }

func (s *Postgres) begin(ctx context.Context, scope tenancy.Context, readonly bool) (pgx.Tx, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	opts := pgx.TxOptions{}
	if readonly {
		opts.AccessMode = pgx.ReadOnly
	}
	tx, err := s.pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	for _, pair := range [][2]string{
		{"staffy.principal_id", scope.PrincipalID().String()},
		{"staffy.tenant_id", scope.TenantID().String()},
	} {
		if _, err := tx.Exec(ctx, "select set_config($1,$2,true)", pair[0], pair[1]); err != nil {
			_ = tx.Rollback(ctx)
			return nil, err
		}
	}
	return tx, nil
}

const eventColumns = "tenant_id,event_id,case_id,conversation_id,connection_id,provider,provider_event_id,provider_session_id,direction,modality,channel,correlation_id,idempotency_key,provider_timestamp,received_at,payload_digest,state,created_at"

func scanEvent(r row) (casesDomain.ChannelEvent, error) {
	var e casesDomain.ChannelEvent
	err := r.Scan(&e.TenantID, &e.EventID, &e.CaseID, &e.ConversationID, &e.ConnectionID, &e.Provider,
		&e.ProviderEventID, &e.ProviderSessionID, &e.Direction, &e.Modality, &e.Channel, &e.CorrelationID,
		&e.IdempotencyKey, &e.ProviderTimestamp, &e.ReceivedAt, &e.PayloadDigest, &e.State, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ChannelEvent{}, casesDomain.ErrNotFound
	}
	if err != nil {
		return casesDomain.ChannelEvent{}, err
	}
	// The read boundary canonicalizes to UTC: the timestamps are a
	// property of the record, never of the reading process's zone.
	e.ProviderTimestamp = e.ProviderTimestamp.UTC()
	e.ReceivedAt = e.ReceivedAt.UTC()
	e.CreatedAt = e.CreatedAt.UTC()
	return e, nil
}

// materialLockKey builds the advisory-lock key of the material identity:
// the same provider event id serializes its deliveries.
func materialLockKey(tenantID id.UUID, provider, providerEventID string) string {
	return "cases.ingest.v1:" + tenantID.String() + ":" + provider + ":" + providerEventID
}

func tenantNumberLockKey(kind string, tenantID id.UUID) string {
	return "cases." + kind + ".v1:" + tenantID.String()
}

// sessionLockKey builds the advisory-lock key of one channel session.
func sessionLockKey(tenantID, connectionID id.UUID, provider, providerSessionID string) string {
	return "cases.session.v1:" + tenantID.String() + ":" + connectionID.String() + ":" + provider + ":" + providerSessionID
}

// LookupEvent resolves the material idempotency BEFORE the admission
// ladder: an accepted event with the same payload digest replays the
// original result; another digest marks the delivery a digest conflict
// (the caller quarantines it).
func (s *Postgres) LookupEvent(ctx context.Context, scope tenancy.Context, provider, providerEventID, payloadDigest, idempotencyKey string) (casesPort.ReplayLookup, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesPort.ReplayLookup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	lookup, err := lookupEventTx(ctx, tx, scope.TenantID(), provider, providerEventID, payloadDigest)
	if err != nil {
		return casesPort.ReplayLookup{}, err
	}
	if idempotencyKey != "" {
		owner, err := scanEvent(tx.QueryRow(ctx, "select "+eventColumns+" from cases.channel_events where tenant_id=$1 and idempotency_key=$2 and state='accepted'", scope.TenantID(), idempotencyKey))
		if err != nil && !errors.Is(err, casesDomain.ErrNotFound) {
			return casesPort.ReplayLookup{}, err
		}
		if err == nil && (owner.Provider != provider || owner.ProviderEventID != providerEventID) {
			// The key is held by an accepted event of another MATERIAL
			// identity: the provider namespace is part of it — the same
			// event id under another provider is a different event (the
			// idempotency index is provider-independent, so the conflict
			// must be flagged by the full pair, never by the event id
			// alone — a partial comparison would loop the resolution).
			lookup.KeyConflict = true
			lookup.KeyOwnerEvent = owner
		}
	}
	return lookup, tx.Commit(ctx)
}

func lookupEventTx(ctx context.Context, tx pgx.Tx, tenantID id.UUID, provider, providerEventID, payloadDigest string) (casesPort.ReplayLookup, error) {
	event, err := scanEvent(tx.QueryRow(ctx, "select "+eventColumns+" from cases.channel_events where tenant_id=$1 and provider=$2 and provider_event_id=$3 and state='accepted'", tenantID, provider, providerEventID))
	if errors.Is(err, casesDomain.ErrNotFound) {
		return casesPort.ReplayLookup{}, nil
	}
	if err != nil {
		return casesPort.ReplayLookup{}, err
	}
	lookup := casesPort.ReplayLookup{Found: true, OriginalEvent: event, DigestMatch: event.PayloadDigest == payloadDigest}
	var caseNo, conversationNo int64
	var ownerID, employeeID id.UUID
	err = tx.QueryRow(ctx, `
                select c.case_no, v.conversation_no, v.response_owner_id, c.employee_id
                from cases.conversations v join cases.cases c on c.tenant_id = v.tenant_id and c.case_id = v.case_id
                where v.tenant_id=$1 and v.conversation_id=$2`,
		tenantID, event.ConversationID).Scan(&caseNo, &conversationNo, &ownerID, &employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesPort.ReplayLookup{}, fmt.Errorf("принятое событие %s без материала кейса: данные повреждены", event.EventID)
	}
	if err != nil {
		return casesPort.ReplayLookup{}, err
	}
	lookup.Result = casesDomain.IngestResult{
		EventID: event.EventID, CaseID: event.CaseID, ConversationID: event.ConversationID,
		CaseNo: caseNo, ConversationNo: conversationNo, ResponseOwnerID: ownerID, EmployeeID: employeeID,
		NewCase: false, Replay: true,
	}
	return lookup, nil
}

// AcceptEvent is the ONE admission transaction: the advisory lock on the
// material identity, the in-transaction re-check (a raced identical
// delivery returns the original result), the find-or-create of the
// conversation by the channel session, the monotonic case and
// conversation numbers under the tenant locks, and the inserts of the
// event, the sealed payload and the two outbox records. Any failure rolls
// everything back — a half-admitted case does not exist.
func (s *Postgres) AcceptEvent(ctx context.Context, p casesPort.AcceptParams) (casesDomain.IngestResult, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return casesDomain.IngestResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := p.Now.UTC().Truncate(time.Microsecond)
	tenantID := p.Scope.TenantID()
	// The material lock serializes the deliveries of one provider event id.
	if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", materialLockKey(tenantID, p.Event.Provider, p.Event.ProviderEventID)); err != nil {
		return casesDomain.IngestResult{}, err
	}
	// The re-check under the lock: the race resolves to the original
	// result (exactly one case, exactly one event).
	lookup, err := lookupEventTx(ctx, tx, tenantID, p.Event.Provider, p.Event.ProviderEventID, p.Event.PayloadDigest)
	if err != nil {
		return casesDomain.IngestResult{}, err
	}
	if lookup.Found {
		if lookup.DigestMatch {
			if err = tx.Commit(ctx); err != nil {
				return casesDomain.IngestResult{}, err
			}
			return lookup.Result, nil
		}
		return casesDomain.IngestResult{}, casesDomain.ErrDigestConflict
	}
	// The session lock serializes the deliveries of one channel session:
	// two different events of a brand-new session cannot race the pre-case
	// session or conversation insert — the loser waits and finds the
	// material (lock order: material → session → numbers, consistent).
	if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		sessionLockKey(tenantID, p.Event.ConnectionID, p.Event.Provider, p.Event.ProviderSessionID)); err != nil {
		return casesDomain.IngestResult{}, err
	}
	// The find-or-create of the conversation by the channel session: the
	// session continuation is the only proven link between events.
	newCase := false
	employeeID := p.EmployeeID
	ownerID := p.ResponseOwnerID
	conversationID, caseID, caseNo, conversationNo := p.ConversationID, p.CaseID, int64(0), int64(0)
	err = tx.QueryRow(ctx, `
                select v.conversation_id, v.case_id, c.case_no, v.conversation_no, c.employee_id, v.response_owner_id
                from cases.conversations v join cases.cases c on c.tenant_id = v.tenant_id and c.case_id = v.case_id
                where v.tenant_id=$1 and v.connection_id=$2 and v.provider=$3 and v.provider_session_id=$4`,
		tenantID, p.Event.ConnectionID, p.Event.Provider, p.Event.ProviderSessionID).Scan(&conversationID, &caseID, &caseNo, &conversationNo, &employeeID, &ownerID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		newCase = true
		// The pre-case session record (the declared capture policy).
		if _, err = tx.Exec(ctx, `
                        insert into cases.pre_case_sessions (tenant_id,session_id,connection_id,provider,provider_session_id,modality,capture_policy,created_at)
                        values ($1,$2,$3,$4,$5,$6,$7,$8)`,
			tenantID, p.SessionID, p.Event.ConnectionID, p.Event.Provider, p.Event.ProviderSessionID,
			p.Event.Modality, p.Session.CapturePolicy, now); err != nil {
			return casesDomain.IngestResult{}, err
		}
		// The monotonic case number under the tenant lock (the decision_no
		// pattern of 1-14).
		if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", tenantNumberLockKey("case_no", tenantID)); err != nil {
			return casesDomain.IngestResult{}, err
		}
		if err = tx.QueryRow(ctx, "select coalesce(max(case_no),0)+1 from cases.cases where tenant_id=$1", tenantID).Scan(&caseNo); err != nil {
			return casesDomain.IngestResult{}, err
		}
		if _, err = tx.Exec(ctx, `
                        insert into cases.cases (tenant_id,case_id,case_no,employee_id,state,created_at)
                        values ($1,$2,$3,$4,$5,$6)`,
			tenantID, p.CaseID, caseNo, employeeID, casesDomain.CaseStateOpen, now); err != nil {
			return casesDomain.IngestResult{}, err
		}
		if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", tenantNumberLockKey("conversation_no", tenantID)); err != nil {
			return casesDomain.IngestResult{}, err
		}
		if err = tx.QueryRow(ctx, "select coalesce(max(conversation_no),0)+1 from cases.conversations where tenant_id=$1", tenantID).Scan(&conversationNo); err != nil {
			return casesDomain.IngestResult{}, err
		}
		if _, err = tx.Exec(ctx, `
                        insert into cases.conversations (tenant_id,case_id,conversation_id,conversation_no,employee_id,connection_id,provider,provider_session_id,response_owner_id,response_owner_epoch,writable_surface,state,created_at)
                        values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			tenantID, p.CaseID, p.ConversationID, conversationNo, employeeID,
			p.Event.ConnectionID, p.Event.Provider, p.Event.ProviderSessionID,
			ownerID, casesDomain.ResponseOwnerEpochFirst, casesDomain.WritableSurfaceInbox,
			casesDomain.ConversationStateActive, now); err != nil {
			return casesDomain.IngestResult{}, err
		}
		// The initial work item is created only with a new conversation.
		if _, err = tx.Exec(ctx, `
                        insert into cases.work_items (tenant_id,case_id,conversation_id,work_item_id,state,created_at)
                        values ($1,$2,$3,$4,$5,$6)`,
			tenantID, p.CaseID, p.ConversationID, p.WorkItemID, casesDomain.WorkItemStateUntriaged, now); err != nil {
			return casesDomain.IngestResult{}, err
		}
	case err != nil:
		return casesDomain.IngestResult{}, err
	}
	// The channel event (accepted) and the sealed payload.
	if _, err = tx.Exec(ctx, `
                insert into cases.channel_events (tenant_id,event_id,case_id,conversation_id,connection_id,provider,provider_event_id,provider_session_id,direction,modality,channel,correlation_id,idempotency_key,provider_timestamp,received_at,payload_digest,state,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		tenantID, p.Event.EventID, caseID, conversationID, p.Event.ConnectionID, p.Event.Provider,
		p.Event.ProviderEventID, p.Event.ProviderSessionID, p.Event.Direction, p.Event.Modality,
		p.Event.Channel, p.Event.CorrelationID, p.Event.IdempotencyKey, p.Event.ProviderTimestamp.UTC().Truncate(time.Microsecond),
		now, p.Event.PayloadDigest, casesDomain.EventStateAccepted, now); err != nil {
		return casesDomain.IngestResult{}, mapInsertError(err)
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.channel_event_payloads (tenant_id,event_id,seal_key,payload,payload_digest,seal,created_at)
                values ($1,$2,$3,$4,$5,$6,$7)`,
		tenantID, p.Event.EventID, p.SealKey, p.Payload, p.Event.PayloadDigest, p.Seal, now); err != nil {
		return casesDomain.IngestResult{}, err
	}
	// The outbox record of the accepted event is marshalled INSIDE the
	// transaction with the resolved material: the canonical JSON never
	// carries fabricated identifiers (a pre-encoded payload of a session
	// continuation would otherwise reference a case that was never
	// created).
	acceptedPayload := p.AcceptedPayload
	acceptedPayload.CaseID = caseID
	acceptedPayload.ConversationID = conversationID
	acceptedPayload.NewCase = newCase
	acceptedPayload.Replay = false
	acceptedJSON, err := casesDomain.AcceptedEventPayloadJSON(acceptedPayload)
	if err != nil {
		return casesDomain.IngestResult{}, err
	}
	// The outbox records: the accepted event and the connector-ack.
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventAccepted, casesDomain.OutboxEventAcceptedV1,
		caseID, p.Event.ConnectionID, p.Event.CorrelationID, now, acceptedJSON,
		casesDomain.DedupKeyAccepted(p.Event.EventID), now); err != nil {
		return casesDomain.IngestResult{}, err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.AckEventID, casesDomain.OutboxEventConnectorAck, casesDomain.OutboxEventConnectorAckV1,
		caseID, p.Event.ConnectionID, p.Event.CorrelationID, now, p.AckPayload,
		casesDomain.DedupKeyConnectorAck(p.Event.EventID), now); err != nil {
		return casesDomain.IngestResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return casesDomain.IngestResult{}, err
	}
	return casesDomain.IngestResult{
		EventID: p.Event.EventID, CaseID: caseID, ConversationID: conversationID,
		CaseNo: caseNo, ConversationNo: conversationNo,
		ResponseOwnerID: ownerID, EmployeeID: employeeID,
		NewCase: newCase, Replay: false,
	}, nil
}

// mapInsertError names the honest conflict of a raced insert by the
// violated constraint: the idempotency-key uniqueness is a key conflict
// (the resolution quarantines), the material and session uniquenesses are
// the defensive backstops of the advisory locks (the resolution replays
// or extends). Raw 23505s never surface as internal errors.
func mapInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "channel_events_idempotency":
			return casesDomain.ErrIdempotencyKeyConflict
		default:
			return casesDomain.ErrEventRaced
		}
	}
	return err
}

// QuarantineEvent records the quarantined delivery and the operational
// exception: the accepted material is never rewritten, the quarantined
// row references the original case when the conflict has one.
func (s *Postgres) QuarantineEvent(ctx context.Context, p casesPort.QuarantineParams) (casesDomain.ChannelEvent, casesDomain.OperationalException, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return casesDomain.ChannelEvent{}, casesDomain.OperationalException{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := p.Now.UTC().Truncate(time.Microsecond)
	tenantID := p.Scope.TenantID()
	if _, err = tx.Exec(ctx, `
                insert into cases.channel_events (tenant_id,event_id,case_id,conversation_id,connection_id,provider,provider_event_id,provider_session_id,direction,modality,channel,correlation_id,idempotency_key,provider_timestamp,received_at,payload_digest,state,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		tenantID, p.Event.EventID, nilCaseID(p.CaseID), nilCaseID(p.ConversationID), p.Event.ConnectionID, p.Event.Provider,
		p.Event.ProviderEventID, p.Event.ProviderSessionID, p.Event.Direction, p.Event.Modality,
		p.Event.Channel, p.Event.CorrelationID, p.Event.IdempotencyKey,
		p.Event.ProviderTimestamp.UTC().Truncate(time.Microsecond), now, p.Event.PayloadDigest,
		casesDomain.EventStateQuarantined, now); err != nil {
		return casesDomain.ChannelEvent{}, casesDomain.OperationalException{}, err
	}
	exception := casesDomain.OperationalException{
		TenantID: tenantID, ExceptionID: p.ExceptionID, Kind: p.ExceptionKind, Summary: p.ExceptionText,
		CorrelationID: p.Event.CorrelationID, EventID: p.Event.EventID, OriginalEventID: p.OriginalEventID,
		CaseID: p.CaseID, ConversationID: p.ConversationID, CreatedAt: now,
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.operational_exceptions (tenant_id,exception_id,kind,summary,correlation_id,event_id,original_event_id,case_id,conversation_id,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		tenantID, p.ExceptionID, p.ExceptionKind, p.ExceptionText, p.Event.CorrelationID, p.Event.EventID,
		nilEventID(p.OriginalEventID), nilCaseID(p.CaseID), nilCaseID(p.ConversationID), now); err != nil {
		return casesDomain.ChannelEvent{}, casesDomain.OperationalException{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return casesDomain.ChannelEvent{}, casesDomain.OperationalException{}, err
	}
	event := p.Event
	event.State = casesDomain.EventStateQuarantined
	event.ReceivedAt = now
	return event, exception, nil
}

// RecordRejection commits one honest refusal row in its own transaction —
// the payload never travels with a refusal.
func (s *Postgres) RecordRejection(ctx context.Context, p casesPort.RejectionParams) (casesDomain.IngressRejection, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return casesDomain.IngressRejection{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := p.Now.UTC().Truncate(time.Microsecond)
	connectionID := nilCaseID(p.ConnectionID)
	if _, err = tx.Exec(ctx, `
                insert into cases.ingress_rejections (tenant_id,rejection_id,kind,message,correlation_id,connection_id,counter,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8)`,
		p.Scope.TenantID(), p.RejectionID, p.Kind, p.Message, p.CorrelationID, connectionID, p.Counter, now); err != nil {
		return casesDomain.IngressRejection{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return casesDomain.IngressRejection{}, err
	}
	return casesDomain.IngressRejection{
		TenantID: p.Scope.TenantID(), RejectionID: p.RejectionID, Kind: p.Kind, Message: p.Message,
		CorrelationID: p.CorrelationID, ConnectionID: p.ConnectionID, Counter: p.Counter, CreatedAt: now,
	}, nil
}

// CountInflight counts the accepted events of the connection inside the
// load-envelope window.
func (s *Postgres) CountInflight(ctx context.Context, scope tenancy.Context, connectionID id.UUID, window time.Duration, now time.Time) (int, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var count int
	if err = tx.QueryRow(ctx, `
                select count(*) from cases.channel_events
                where tenant_id=$1 and connection_id=$2 and state='accepted' and received_at > $3`,
		scope.TenantID(), connectionID, now.UTC().Add(-window)).Scan(&count); err != nil {
		return 0, err
	}
	return count, tx.Commit(ctx)
}

// CountRejections counts the recorded refusals of one KIND inside the
// window — the bound of the pre-verification rejection rows, keyed by
// (tenant, kind): the connection id of an unverifiable delivery is
// attacker-chosen and must never multiply the budget.
func (s *Postgres) CountRejections(ctx context.Context, scope tenancy.Context, kind string, window time.Duration, now time.Time) (int, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var count int
	if err = tx.QueryRow(ctx, `
		select count(*) from cases.ingress_rejections
		where tenant_id=$1 and kind=$2 and created_at > $3`,
		scope.TenantID(), kind, now.UTC().Add(-window)).Scan(&count); err != nil {
		return 0, err
	}
	return count, tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Operator surface reads
// ---------------------------------------------------------------------------

// ListCases summarizes the tenant cases: the counts and the last event
// time per case.
func (s *Postgres) ListCases(ctx context.Context, scope tenancy.Context) ([]casesDomain.CaseSummary, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select c.case_id, c.case_no, c.employee_id, c.state, c.created_at,
                       (select count(*) from cases.conversations v where v.tenant_id=c.tenant_id and v.case_id=c.case_id) as conversations,
                       (select count(*) from cases.channel_events e where e.tenant_id=c.tenant_id and e.case_id=c.case_id and e.state='accepted') as events,
                       coalesce((select max(e.received_at) from cases.channel_events e where e.tenant_id=c.tenant_id and e.case_id=c.case_id and e.state='accepted'), c.created_at) as last_event_at,
                       (select v.response_owner_id from cases.conversations v where v.tenant_id=c.tenant_id and v.case_id=c.case_id order by v.conversation_no limit 1) as owner_id
                from cases.cases c
                where c.tenant_id=$1
                order by c.case_no desc`, scope.TenantID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []casesDomain.CaseSummary{}
	for rows.Next() {
		var summary casesDomain.CaseSummary
		var lastEventAt time.Time
		var ownerID *id.UUID
		if err = rows.Scan(&summary.CaseID, &summary.CaseNo, &summary.EmployeeID, &summary.State, &summary.CreatedAt,
			&summary.Conversations, &summary.Events, &lastEventAt, &ownerID); err != nil {
			return nil, err
		}
		summary.CreatedAt = summary.CreatedAt.UTC()
		summary.LastEventAt = lastEventAt.UTC()
		if ownerID != nil {
			summary.OwnerID = *ownerID
		}
		out = append(out, summary)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

// ReadCase reads one case with its conversations, the accepted timeline,
// the work items and the related exceptions.
func (s *Postgres) ReadCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesDomain.CaseDetail, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesDomain.CaseDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	detail, err := readCaseTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return casesDomain.CaseDetail{}, err
	}
	return detail, tx.Commit(ctx)
}

func readCaseTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) (casesDomain.CaseDetail, error) {
	detail := casesDomain.CaseDetail{}
	err := tx.QueryRow(ctx, `
                select case_id, case_no, employee_id, state, created_at
                from cases.cases where tenant_id=$1 and case_id=$2`, tenantID, caseID).
		Scan(&detail.Case.CaseID, &detail.Case.CaseNo, &detail.Case.EmployeeID, &detail.Case.State, &detail.Case.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.CaseDetail{}, casesDomain.ErrNotFound
	}
	if err != nil {
		return casesDomain.CaseDetail{}, err
	}
	detail.Case.TenantID = tenantID
	detail.Case.CreatedAt = detail.Case.CreatedAt.UTC()

	conversations, err := tx.Query(ctx, `
                select conversation_id, case_id, conversation_no, employee_id, connection_id, provider, provider_session_id, response_owner_id, response_owner_epoch, writable_surface, state, created_at
                from cases.conversations where tenant_id=$1 and case_id=$2 order by conversation_no`, tenantID, caseID)
	if err != nil {
		return casesDomain.CaseDetail{}, err
	}
	defer conversations.Close()
	detail.Conversations = []casesDomain.Conversation{}
	for conversations.Next() {
		var conversation casesDomain.Conversation
		if err = conversations.Scan(&conversation.ConversationID, &conversation.CaseID, &conversation.ConversationNo,
			&conversation.EmployeeID, &conversation.ConnectionID, &conversation.Provider, &conversation.ProviderSessionID,
			&conversation.ResponseOwnerID, &conversation.ResponseOwnerEpoch, &conversation.WritableSurface,
			&conversation.State, &conversation.CreatedAt); err != nil {
			return casesDomain.CaseDetail{}, err
		}
		conversation.TenantID = tenantID
		conversation.CreatedAt = conversation.CreatedAt.UTC()
		detail.Conversations = append(detail.Conversations, conversation)
	}
	if err = conversations.Err(); err != nil {
		return casesDomain.CaseDetail{}, err
	}

	events, err := tx.Query(ctx, `
                select event_id, channel, modality, direction, provider, provider_event_id, state, provider_timestamp, received_at, payload_digest, correlation_id
                from cases.channel_events where tenant_id=$1 and case_id=$2 order by received_at, event_id`, tenantID, caseID)
	if err != nil {
		return casesDomain.CaseDetail{}, err
	}
	defer events.Close()
	detail.Timeline = []casesDomain.TimelineEvent{}
	for events.Next() {
		var event casesDomain.TimelineEvent
		if err = events.Scan(&event.EventID, &event.Channel, &event.Modality, &event.Direction, &event.Provider,
			&event.ProviderEventID, &event.State, &event.ProviderTimestamp, &event.ReceivedAt,
			&event.PayloadDigest, &event.CorrelationID); err != nil {
			return casesDomain.CaseDetail{}, err
		}
		event.ProviderTimestamp = event.ProviderTimestamp.UTC()
		event.ReceivedAt = event.ReceivedAt.UTC()
		detail.Timeline = append(detail.Timeline, event)
	}
	if err = events.Err(); err != nil {
		return casesDomain.CaseDetail{}, err
	}

	items, err := tx.Query(ctx, `
                select work_item_id, case_id, conversation_id, state, created_at
                from cases.work_items where tenant_id=$1 and case_id=$2 order by created_at, work_item_id`, tenantID, caseID)
	if err != nil {
		return casesDomain.CaseDetail{}, err
	}
	defer items.Close()
	detail.WorkItems = []casesDomain.WorkItem{}
	for items.Next() {
		var item casesDomain.WorkItem
		if err = items.Scan(&item.WorkItemID, &item.CaseID, &item.ConversationID, &item.State, &item.CreatedAt); err != nil {
			return casesDomain.CaseDetail{}, err
		}
		item.TenantID = tenantID
		item.CreatedAt = item.CreatedAt.UTC()
		detail.WorkItems = append(detail.WorkItems, item)
	}
	if err = items.Err(); err != nil {
		return casesDomain.CaseDetail{}, err
	}

	exceptions, err := exceptionRows(ctx, tx, tenantID, "case_id=$1 and tenant_id=$2", caseID, tenantID)
	if err != nil {
		return casesDomain.CaseDetail{}, err
	}
	detail.Exceptions = exceptions
	return detail, nil
}

// ReadPayload reads the sealed payload of one event and verifies the
// digest and the HMAC seal at the read boundary.
func (s *Postgres) ReadPayload(ctx context.Context, scope tenancy.Context, caseID, eventID id.UUID) (casesDomain.SealedPayload, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesDomain.SealedPayload{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var payload []byte
	var sealKey []byte
	var digest, seal string
	err = tx.QueryRow(ctx, `
                select p.payload, p.seal_key, p.payload_digest, p.seal
                from cases.channel_event_payloads p
                join cases.channel_events e on e.tenant_id=p.tenant_id and e.event_id=p.event_id
                where p.tenant_id=$1 and p.event_id=$2 and e.state='accepted' and e.case_id=$3`, scope.TenantID(), eventID, caseID).
		Scan(&payload, &sealKey, &digest, &seal)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.SealedPayload{}, casesDomain.ErrNotFound
	}
	if err != nil {
		return casesDomain.SealedPayload{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return casesDomain.SealedPayload{}, err
	}
	sealed := casesDomain.SealedPayload{EventID: eventID, Digest: digest, Payload: payload}
	sealed.SealOK = digest == casesDomain.PayloadDigest(payload) && seal == casesDomain.PayloadSeal(sealKey, payload)
	return sealed, nil
}

// ListExceptions lists the operational exceptions of the tenant, newest
// first.
func (s *Postgres) ListExceptions(ctx context.Context, scope tenancy.Context) ([]casesDomain.OperationalException, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	out, err := exceptionRows(ctx, tx, scope.TenantID(), "tenant_id=$1", scope.TenantID())
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func exceptionRows(ctx context.Context, tx pgx.Tx, tenantID id.UUID, where string, args ...any) ([]casesDomain.OperationalException, error) {
	rows, err := tx.Query(ctx, `
                select exception_id, kind, summary, correlation_id, event_id, coalesce(original_event_id, '00000000-0000-0000-0000-000000000000'), coalesce(case_id, '00000000-0000-0000-0000-000000000000'), coalesce(conversation_id, '00000000-0000-0000-0000-000000000000'), created_at
                from cases.operational_exceptions where `+where+` order by created_at desc`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []casesDomain.OperationalException{}
	for rows.Next() {
		var exception casesDomain.OperationalException
		var originalEventID, caseID, conversationID id.UUID
		if err = rows.Scan(&exception.ExceptionID, &exception.Kind, &exception.Summary, &exception.CorrelationID,
			&exception.EventID, &originalEventID, &caseID, &conversationID, &exception.CreatedAt); err != nil {
			return nil, err
		}
		exception.TenantID = tenantID
		exception.OriginalEventID = originalEventID
		exception.CaseID = caseID
		exception.ConversationID = conversationID
		exception.CreatedAt = exception.CreatedAt.UTC()
		out = append(out, exception)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Outbox drain
// ---------------------------------------------------------------------------

// PendingOutbox returns the outbox events without any append-only delivery
// outcome: the idempotent drain input (a delivered record never appears
// again).
func (s *Postgres) PendingOutbox(ctx context.Context, scope tenancy.Context, limit int) ([]casesDomain.OutboxEvent, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select o.tenant_id, o.event_id, o.event_type, o.schema_version, o.aggregate_id, o.correlation_id, o.occurred_at, o.payload, o.dedup_key
                from cases.outbox_events o
                where o.tenant_id=$1 and not exists (
                        select 1 from cases.outbox_deliveries d where d.tenant_id=o.tenant_id and d.event_id=o.event_id)
                order by o.created_at, o.event_id
                limit $2`, scope.TenantID(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []casesDomain.OutboxEvent{}
	for rows.Next() {
		var event casesDomain.OutboxEvent
		if err = rows.Scan(&event.TenantID, &event.EventID, &event.EventType, &event.SchemaVersion,
			&event.AggregateID, &event.CorrelationID, &event.OccurredAt, &event.Payload, &event.DedupKey); err != nil {
			return nil, err
		}
		event.OccurredAt = event.OccurredAt.UTC()
		out = append(out, event)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

// RecordDelivery appends one delivery outcome.
func (s *Postgres) RecordDelivery(ctx context.Context, scope tenancy.Context, delivery casesDomain.OutboxDelivery) error {
	tx, err := s.begin(ctx, scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_deliveries (tenant_id,delivery_id,event_id,attempt,outcome,details,created_at)
                values ($1,$2,$3,$4,$5,$6,$7)`,
		scope.TenantID(), delivery.DeliveryID, delivery.EventID, delivery.Attempt,
		delivery.Outcome, delivery.Details, delivery.CreatedAt.UTC().Truncate(time.Microsecond)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func nilCaseID(value id.UUID) any {
	if value.Valid() {
		return value
	}
	return nil
}

func nilEventID(value id.UUID) any {
	if value.Valid() {
		return value
	}
	return nil
}
