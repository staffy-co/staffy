package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.8 (step-02): the order correction persistence — the correction
// reads, the event history read and the TWO locked appends
// ---------------------------------------------------------------------------

// The structural pin: the store satisfies the consumer-owned repository
// contract of the correction contour (the OrderVerificationRepository
// precedent).
var _ actionPort.OrderCorrectionRepository = (*Postgres)(nil)

// orderCorrectionLockKey is the advisory lock key of one case's
// correction ladder — its OWN key space (the "action.order_correction:"
// prefix never mixes with the verification/proposal/dispatch ones): the
// creation race (the correction_no ladder) and every lifecycle append
// (the CAS epoch re-validation) serialize per case, the raced loser
// reads the winner's rows under the lock and answers the honest
// current-state refusal — never a masked write.
func orderCorrectionLockKey(tenantID, caseID id.UUID) string {
	return "action.order_correction.v1:" + tenantID.String() + ":" + caseID.String()
}

// mapOrderCorrectionInsertError maps the raced insert conflicts and the
// CHECK backstops onto the honest consumer-owned signals (the
// mapOrderVerificationInsertError precedent): the correction PK is the
// per-case ladder backstop, the event PK is the monotonic append twin,
// the one-closure partial UNIQUE is the structural closure backstop,
// the material CHECK is the loyal domain-mirror refusal — the store
// never masks a lost race as a write.
func mapOrderCorrectionInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		return actionPort.ErrCorrectionRaced
	}
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23514" {
		return actionPort.ErrCorrectionRaced
	}
	return err
}

// orderCorrectionColumns is the canonical column list of the correction
// row (the SELECT and the INSERT share it — the scan target list
// mirrors it).
const orderCorrectionColumns = `tenant_id, case_id, work_item_id, correction_no, operation_id,
 verification_no, receipt_version_no, external_order_id, creator_id, created_at`

// scanOrderCorrection scans one correction row (the shared target list).
func scanOrderCorrection(row pgx.Row, tenantID id.UUID) (actionDomain.OrderCorrection, error) {
	var correction actionDomain.OrderCorrection
	if err := row.Scan(&correction.TenantID, &correction.CaseID, &correction.WorkItemID, &correction.CorrectionNo,
		&correction.OperationID, &correction.VerificationNo, &correction.ReceiptVersionNo,
		&correction.ExternalOrderID, &correction.CreatorID, &correction.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return actionDomain.OrderCorrection{}, actionPort.ErrCorrectionNotFound
		}
		return actionDomain.OrderCorrection{}, err
	}
	if correction.TenantID != tenantID {
		return actionDomain.OrderCorrection{}, actionPort.ErrCorrectionNotFound
	}
	return correction, nil
}

// readOrderCorrections reads the correction rows of one anchor (the case
// or the operation) ordered by the correction number (the ladder view).
func readOrderCorrections(ctx context.Context, tx pgx.Tx, tenantID id.UUID, where string, anchor id.UUID) ([]actionDomain.OrderCorrection, error) {
	rows, err := tx.Query(ctx, `select `+orderCorrectionColumns+`
                from action.order_corrections where tenant_id=$1 and `+where+` order by correction_no`,
		tenantID, anchor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	corrections := []actionDomain.OrderCorrection{}
	for rows.Next() {
		var correction actionDomain.OrderCorrection
		if err := rows.Scan(&correction.TenantID, &correction.CaseID, &correction.WorkItemID, &correction.CorrectionNo,
			&correction.OperationID, &correction.VerificationNo, &correction.ReceiptVersionNo,
			&correction.ExternalOrderID, &correction.CreatorID, &correction.CreatedAt); err != nil {
			return nil, err
		}
		corrections = append(corrections, correction)
	}
	return corrections, rows.Err()
}

// readOrderCorrectionEvents reads the append-only events of one
// correction ordered by the monotonic event number (the state
// derivation input). The per-kind material columns are NULL for the
// kinds that carry other material — the scan goes through the pointer
// intermediates (the honest zero of an absent material, never a scan
// error — the 4-7 step-04 lesson carried by design).
func readOrderCorrectionEvents(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID, correctionNo int64) ([]actionDomain.OrderCorrectionEvent, error) {
	rows, err := tx.Query(ctx, `select tenant_id, case_id, correction_no, event_no, kind,
                owner, correct_deadline, deadline_at,
                external_object_id, proposal_id, version_no, confirmation_id,
                positions, reason, cessation_reason, actor_id, created_at
                from action.order_correction_events
                where tenant_id=$1 and case_id=$2 and correction_no=$3 order by event_no`,
		tenantID, caseID, correctionNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []actionDomain.OrderCorrectionEvent{}
	for rows.Next() {
		var event actionDomain.OrderCorrectionEvent
		var owner, correctDeadline, externalObjectID, reason, cessationReason *string
		var proposalID, confirmationID *string
		var versionNo *int64
		var deadlineAt, createdAt *time.Time
		var positions []byte
		if err := rows.Scan(&event.TenantID, &event.CaseID, &event.CorrectionNo, &event.EventNo, &event.Kind,
			&owner, &correctDeadline, &deadlineAt,
			&externalObjectID, &proposalID, &versionNo, &confirmationID,
			&positions, &reason, &cessationReason, &event.ActorID, &createdAt); err != nil {
			return nil, err
		}
		if owner != nil {
			event.Owner = id.UUID(*owner)
		}
		if correctDeadline != nil {
			event.CorrectDeadline = *correctDeadline
		}
		if deadlineAt != nil {
			event.DeadlineAt = *deadlineAt
		}
		if externalObjectID != nil {
			event.ExternalObjectID = *externalObjectID
		}
		if proposalID != nil {
			event.ProposalID = id.UUID(*proposalID)
		}
		if versionNo != nil {
			event.VersionNo = *versionNo
		}
		if confirmationID != nil {
			event.ConfirmationID = id.UUID(*confirmationID)
		}
		if reason != nil {
			event.Reason = *reason
		}
		if cessationReason != nil {
			event.CessationReason = *cessationReason
		}
		if createdAt != nil {
			event.CreatedAt = *createdAt
		}
		if positions != nil {
			decoded, err := decodeStringArray(positions)
			if err != nil {
				return nil, err
			}
			event.Positions = decoded
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// ReadOperationCorrections reads every correction anchored to one
// operation ordered by the correction number (the open-correction
// conflict check and the ladder view).
func (s *Postgres) ReadOperationCorrections(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]actionDomain.OrderCorrection, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	tenantID := scope.TenantID()
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	corrections, err := readOrderCorrections(ctx, tx, tenantID, "operation_id=$2", operationID)
	if err != nil {
		return nil, err
	}
	return corrections, tx.Commit(ctx)
}

// ReadCaseCorrections reads every correction of one case ordered by the
// correction number (the view material of the panel).
func (s *Postgres) ReadCaseCorrections(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]actionDomain.OrderCorrection, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	tenantID := scope.TenantID()
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	corrections, err := readOrderCorrections(ctx, tx, tenantID, "case_id=$2", caseID)
	if err != nil {
		return nil, err
	}
	return corrections, tx.Commit(ctx)
}

// ReadCorrectionEvents reads the append-only events of one correction
// ordered by the monotonic event number (the state derivation input).
func (s *Postgres) ReadCorrectionEvents(ctx context.Context, scope tenancy.Context, caseID id.UUID, correctionNo int64) ([]actionDomain.OrderCorrectionEvent, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	tenantID := scope.TenantID()
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	events, err := readOrderCorrectionEvents(ctx, tx, tenantID, caseID, correctionNo)
	if err != nil {
		return nil, err
	}
	return events, tx.Commit(ctx)
}

// AppendOrderCorrection creates the correction row AND the opening
// opened event in ONE transaction under the per-case advisory lock: the
// monotonic correction_no derives under the lock (the ladder of the
// repeated correction — the history is never rewritten), the
// one-open-correction rule re-validates UNDER the lock (the raced twin
// of the same operation refuses with ErrCorrectionActive — the honest
// 409, never a second competing correction). The blocked positions and
// the reason word travel BY NAME inside this transaction — the original
// blocked event of the verification task was READ by the app before the
// transaction, its row is NEVER rewritten.
func (s *Postgres) AppendOrderCorrection(ctx context.Context, p actionPort.AppendOrderCorrectionParams) (actionDomain.OrderCorrection, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return actionDomain.OrderCorrection{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		orderCorrectionLockKey(tenantID, p.CaseID)); err != nil {
		return actionDomain.OrderCorrection{}, err
	}
	// The one-open-correction rule UNDER the lock: every correction of
	// the operation with its events derives its state — an OPEN
	// correction refuses the second competing correction honestly (the
	// app speaks the current state words).
	anchored, err := readOrderCorrections(ctx, tx, tenantID, "operation_id=$2", p.OperationID)
	if err != nil {
		return actionDomain.OrderCorrection{}, err
	}
	for _, correction := range anchored {
		events, err := readOrderCorrectionEvents(ctx, tx, tenantID, correction.CaseID, correction.CorrectionNo)
		if err != nil {
			return actionDomain.OrderCorrection{}, err
		}
		if actionDomain.DeriveOrderCorrectionState(correction, events, p.Now).Active() {
			return actionDomain.OrderCorrection{}, actionPort.ErrCorrectionActive
		}
	}
	// The per-case ladder number UNDER the lock (the store truth — the
	// correction never carries it).
	var correctionNo int64
	if err := tx.QueryRow(ctx, `
                select coalesce(max(correction_no), 0) + 1 from action.order_corrections
                where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&correctionNo); err != nil {
		return actionDomain.OrderCorrection{}, err
	}
	createdAt := p.Now.UTC().Truncate(time.Microsecond)
	correction := actionDomain.OrderCorrection{
		TenantID:         tenantID,
		CaseID:           p.CaseID,
		WorkItemID:       p.WorkItemID,
		CorrectionNo:     correctionNo,
		OperationID:      p.OperationID,
		VerificationNo:   p.VerificationNo,
		ReceiptVersionNo: p.ReceiptVersionNo,
		ExternalOrderID:  p.ExternalOrderID,
		CreatorID:        p.CreatorID,
		CreatedAt:        createdAt,
	}
	if _, err := tx.Exec(ctx, `
                insert into action.order_corrections (tenant_id, case_id, work_item_id, correction_no,
                        operation_id, verification_no, receipt_version_no, external_order_id, creator_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		correction.TenantID, correction.CaseID, correction.WorkItemID, correction.CorrectionNo,
		correction.OperationID, correction.VerificationNo, correction.ReceiptVersionNo,
		correction.ExternalOrderID, correction.CreatorID, correction.CreatedAt); err != nil {
		return actionDomain.OrderCorrection{}, mapOrderCorrectionInsertError(err)
	}
	var positions any
	if p.Positions != nil {
		positions = encodeStringArray(p.Positions)
	}
	if _, err := tx.Exec(ctx, `
                insert into action.order_correction_events (tenant_id, case_id, correction_no, event_no, kind,
                        owner, correct_deadline, deadline_at,
                        external_object_id, proposal_id, version_no, confirmation_id,
                        positions, reason, cessation_reason, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,NULL,NULL,NULL,NULL,$9,$10,NULL,$11,$12)`,
		tenantID, p.CaseID, correctionNo, int64(1), actionDomain.OrderCorrectionEventOpened,
		p.Owner, p.CorrectDeadline, p.DeadlineAt.UTC().Truncate(time.Microsecond),
		positions, p.Reason, p.CreatorID, createdAt); err != nil {
		return actionDomain.OrderCorrection{}, mapOrderCorrectionInsertError(err)
	}
	return correction, tx.Commit(ctx)
}

// AppendOrderCorrectionEvent appends ONE lifecycle event under the
// per-case advisory lock with the CAS re-validation: the derived state
// and the assignment epoch must EQUAL the command's expectation — a
// raced append (the parallel closure of the second manager, the
// reassignment, the expiry word move) refuses with ErrCorrectionRaced
// and NEVER writes. The monotonic event_no derives under the lock. The
// one-closure partial UNIQUE of the migration backstops the invariant
// the lock already holds (a correction never closes twice).
func (s *Postgres) AppendOrderCorrectionEvent(ctx context.Context, p actionPort.AppendOrderCorrectionEventParams) (actionDomain.OrderCorrectionEvent, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return actionDomain.OrderCorrectionEvent{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		orderCorrectionLockKey(tenantID, p.CaseID)); err != nil {
		return actionDomain.OrderCorrectionEvent{}, err
	}
	// The correction re-read UNDER the lock (the anchor must exist — the
	// RLS blindness of a foreign tenant answers the honest 404).
	row := tx.QueryRow(ctx, `select `+orderCorrectionColumns+`
                from action.order_corrections where tenant_id=$1 and case_id=$2 and correction_no=$3`,
		tenantID, p.CaseID, p.CorrectionNo)
	correction, err := scanOrderCorrection(row, tenantID)
	if err != nil {
		return actionDomain.OrderCorrectionEvent{}, err
	}
	// The state re-derivation UNDER the lock: the CAS expectation of the
	// command must equal the CURRENT derived truth — the raced append
	// never writes (the parallel closure loses honestly).
	events, err := readOrderCorrectionEvents(ctx, tx, tenantID, correction.CaseID, correction.CorrectionNo)
	if err != nil {
		return actionDomain.OrderCorrectionEvent{}, err
	}
	view := actionDomain.DeriveOrderCorrectionState(correction, events, p.Now)
	if view.State != p.ExpectedState || view.AssignmentEpoch != p.ExpectedEpoch {
		return actionDomain.OrderCorrectionEvent{}, actionPort.ErrCorrectionRaced
	}
	var eventNo int64
	if err := tx.QueryRow(ctx, `
                select coalesce(max(event_no), 0) + 1 from action.order_correction_events
                where tenant_id=$1 and case_id=$2 and correction_no=$3`,
		tenantID, p.CaseID, p.CorrectionNo).Scan(&eventNo); err != nil {
		return actionDomain.OrderCorrectionEvent{}, err
	}
	event := p.Event
	event.TenantID = tenantID
	event.CaseID = correction.CaseID
	event.CorrectionNo = correction.CorrectionNo
	event.EventNo = eventNo
	event.CreatedAt = p.Now.UTC().Truncate(time.Microsecond)
	var positions any
	if event.Positions != nil {
		positions = encodeStringArray(event.Positions)
	}
	if _, err := tx.Exec(ctx, `
                insert into action.order_correction_events (tenant_id, case_id, correction_no, event_no, kind,
                        owner, correct_deadline, deadline_at,
                        external_object_id, proposal_id, version_no, confirmation_id,
                        positions, reason, cessation_reason, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		event.TenantID, event.CaseID, event.CorrectionNo, event.EventNo, event.Kind,
		nilIfZero(event.Owner), nilIfEmpty(event.CorrectDeadline), nilIfZeroTime(event.DeadlineAt),
		nilIfEmpty(event.ExternalObjectID), nilIfZero(event.ProposalID), nilIfZeroInt(event.VersionNo),
		nilIfZero(event.ConfirmationID), positions, nilIfEmpty(event.Reason),
		nilIfEmpty(event.CessationReason), event.ActorID, event.CreatedAt); err != nil {
		return actionDomain.OrderCorrectionEvent{}, mapOrderCorrectionInsertError(err)
	}
	return event, tx.Commit(ctx)
}
