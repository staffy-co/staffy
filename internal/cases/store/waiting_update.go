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
)

// ---------------------------------------------------------------------------
// Story 2.16: the truthful waiting updates — the promise write and the
// consistent snapshot read of the waiting surface
// ---------------------------------------------------------------------------

// PromiseRoundRefusedText is the honest refusal of the round guard: the
// promise anchors to the OPEN waiting of the client (the fresh ask, the
// named round, the accepted case). The declined round consumed its
// assignment — the re-assignment decision belongs to the human (a
// promise over the consumed round would name nobody); the cancelled and
// returned rounds ended the waiting honestly (the promises of the closed
// rounds stay the readable history — the new promise has no live round
// to owe the update). The store mirror words answer with the derived
// state word of the moment.
const PromiseRoundRefusedText = "Обещание обновления якорится открытым ожиданием клиента: раунд в состоянии «%s» не принимает обещаний — переназначение и закрытие остаются решением человека."

// mapUpdatePromiseInsertError maps the raced insert conflicts of the
// promise surface onto the honest domain errors: the promise number
// UNIQUE is the raced derivation (the loser re-derives under the locks
// — the honest stale-retry signal) and the raced duplicate of the SAME
// promise row id maps the same way (the app replay never reaches here —
// the command record resolves before the exec).
func mapUpdatePromiseInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "handoff_update_promises_case_no":
			// The raced promise number: the loser re-derives under the
			// locks and retries (the honest stale-retry signal).
			return casesDomain.ErrStaleVersion
		case "handoff_update_promises_pkey":
			// The raced duplicate of the SAME promise row id (the honest
			// retry signal of a raced double-submit).
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

// scanUpdatePromiseRecord reads one append-only promise row (the
// promised time stays NULL for the honest unknown word — the material
// CHECK pair of the table owns the invariant).
func scanUpdatePromiseRecord(scan func(dest ...any) error, tenantID id.UUID) (casesDomain.UpdatePromiseRecord, error) {
	var record casesDomain.UpdatePromiseRecord
	var promisedAt *time.Time
	err := scan(&record.CaseID, &record.PromiseID, &record.PromiseNo, &record.RequestNo,
		&record.DeadlineWord, &promisedAt, &record.ActorID,
		&record.ObservedAt, &record.ReceivedAt, &record.CreatedAt)
	if err != nil {
		return record, err
	}
	record.TenantID = tenantID
	if promisedAt != nil {
		record.PromisedAt = promisedAt.UTC()
	}
	record.ObservedAt = record.ObservedAt.UTC()
	record.ReceivedAt = record.ReceivedAt.UTC()
	record.CreatedAt = record.CreatedAt.UTC()
	return record, nil
}

// readUpdatePromiseStateTx reads the promise rows of the case ordered by
// the monotonic promise numbers (the per-round LAST-promise derivations
// filter them by the request anchor read-time — the store never derives,
// the rows are the append-only truth).
func readUpdatePromiseStateTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.UpdatePromiseRecord, error) {
	rows, err := tx.Query(ctx, `
		select case_id, promise_id, promise_no, request_no, deadline_word, promised_at, actor_id,
		       observed_at, received_at, created_at
		from cases.handoff_update_promises
		where tenant_id=$1 and case_id=$2 order by promise_no`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []casesDomain.UpdatePromiseRecord{}
	for rows.Next() {
		record, err := scanUpdatePromiseRecord(rows.Scan, tenantID)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

// ReadUpdatePromiseState reads the ONE consistent snapshot of the
// waiting update surface: the handoff lifecycle material (the round
// derivation input — the same rows the operator card reads) together
// with the promise rows of the case (the promise state derivation
// input) — one read transaction, the derivations happen read-time in
// the domain over the consistent rows (the 2.15 snapshot precedent; the
// decision context of 2.14 is NOT touched — the case page reads both
// snapshots separately). The advisory locks are NOT taken: the
// consistent snapshot of one read transaction is the honest boundary of
// the GET.
func (s *Postgres) ReadUpdatePromiseState(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesPort.UpdatePromiseMaterial, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesPort.UpdatePromiseMaterial{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	requests, events, err := readHandoffsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return casesPort.UpdatePromiseMaterial{}, err
	}
	promises, err := readUpdatePromiseStateTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return casesPort.UpdatePromiseMaterial{}, err
	}
	return casesPort.UpdatePromiseMaterial{
		Handoff:  casesPort.HandoffMaterial{Requests: requests, Events: events},
		Promises: promises,
	}, tx.Commit(ctx)
}

// execPromiseUpdate writes one append-only promise row under the case +
// conversation locks (the sorted lock plan of the 2.5..2.15 precedent —
// the identical key set the 2.13 lifecycle writers hold, so a round
// closing and a promise write see each other's committed truth): the
// case must exist under the tenant scope and stay open (the terminal
// backstop — the read stays honest), the round guard re-validates the
// OPEN lifecycle of THIS case (the store mirror words refuse the
// declined, cancelled and returned rounds), the domain guards
// re-validate the deadline word and the 2.2 times under the locks, the
// promise number is the NEXT monotonic number of the case (the UNIQUE
// backstop catches the race the locks already serialize) and the
// promised time is the SERVER truth of the moment (the known words
// compute now + window; the honest unknown word is the NULL — the
// material CHECK pair owns the invariant, a fabricated timestamp NEVER
// accompanies the unknown word). The promise NEVER resolves the round
// and NEVER mutates a former row: the lifecycle transitions belong to
// the existing 2.13 commands, the supersede is the read-time
// derivation. The outbox envelope carries the identifiers and the
// deadline word ONLY — the client texts and any person material NEVER
// enter.
func (s *Postgres) execPromiseUpdate(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := handoffCaseOpen(ctx, tx, tenantID, p.CaseID, p.Command, now); err != nil {
		return err
	}
	requests, events, err := readHandoffsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	// The round anchor: the OPEN lifecycle, or the LAST closed one (the
	// guard then refuses with the honest state word — the raced twin of
	// the two-operator race learns the round is already closed).
	target := handoffOpenView(requests, events, now)
	if target == nil {
		target = handoffLastView(requests, events, now)
	}
	if target == nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.HandoffAbsentText}
	}
	// The round guard: the promise anchors to the open waiting — the
	// declined round consumed the assignment (the re-assignment decision
	// belongs to the human), the cancelled and returned rounds ended the
	// waiting (the store mirror words).
	if !casesDomain.UpdatePromiseRoundAccepts(target.State) {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: fmt.Sprintf(PromiseRoundRefusedText, target.State)}
	}
	// The domain guards under the locks: the deadline word of the fixed
	// dictionary (the honest unknown word included — a fabricated
	// deadline NEVER passes) and the 2.2 times.
	if err := casesDomain.GuardPromiseUpdate(p.PromiseDeadlineWord); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	if err := casesDomain.GuardUpdatePromiseTimes(p.ObservedAt, p.ReceivedAt); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The promise number: the NEXT monotonic number of the case under
	// the locks (the request never carries it; the UNIQUE backstop
	// catches the race).
	var promiseNo int64
	if err = tx.QueryRow(ctx, `
		select coalesce(max(promise_no),0)+1 from cases.handoff_update_promises
		where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&promiseNo); err != nil {
		return err
	}
	// The promised time: the SERVER truth of the moment under the locks
	// — the known words compute now + window; the honest unknown word is
	// the ZERO time, stored NULL (the material CHECK pair owns the
	// invariant).
	promisedAt := casesDomain.UpdatePromiseDeadline(p.PromiseDeadlineWord, now)
	// The append-only row: the row NEVER mutates — a new promise is a
	// new row, the former rows derive superseded read-time.
	if _, err = tx.Exec(ctx, `
		insert into cases.handoff_update_promises (tenant_id, case_id, promise_id, promise_no, request_no,
			deadline_word, promised_at, actor_id, observed_at, received_at, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.CaseID, p.PromiseID, promiseNo, target.Request.RequestNo,
		p.PromiseDeadlineWord, nilTime(promisedAt), p.Actor,
		p.ObservedAt.UTC().Truncate(time.Microsecond), p.ReceivedAt.UTC().Truncate(time.Microsecond),
		now.UTC().Truncate(time.Microsecond)); err != nil {
		return mapUpdatePromiseInsertError(err)
	}
	// The promise envelope: the identifiers and the deadline word ONLY —
	// the client texts and any person material NEVER enter (the
	// «кандидат не покидает строку» precedent of 2.6/2.13/2.15).
	payload, err := casesDomain.UpdatePromisedPayloadJSON(casesDomain.UpdatePromisedPayload{
		TenantID: tenantID, CaseID: p.CaseID, RequestNo: target.Request.RequestNo,
		PromiseID: p.PromiseID, PromiseNo: promiseNo,
		DeadlineWord: p.PromiseDeadlineWord, ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
		select connection_id from cases.conversations
		where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.PromiseOutboxEventID, casesDomain.OutboxEventUpdatePromised, casesDomain.OutboxEventUpdatePromisedV1,
		p.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyUpdatePromised(p.CaseID, target.Request.RequestNo, promiseNo), now); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
			// The raced envelope of the same promise number (the replay
			// never reaches here — the command record resolves before the
			// exec; the honest stale-retry signal).
			return casesDomain.ErrStaleVersion
		}
		return err
	}
	return nil
}
