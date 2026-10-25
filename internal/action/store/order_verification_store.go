package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 4.7 (step-02): the manager order verification persistence — the
// task reads, the event history read and the TWO locked appends
// ---------------------------------------------------------------------------

// The structural pin: the store satisfies the consumer-owned repository
// contract of the verification contour (the RecoveryRepository precedent).
var _ actionPort.OrderVerificationRepository = (*Postgres)(nil)

// orderVerificationLockKey is the advisory lock key of one case's
// verification ladder — its OWN key space (the "action.order_verification:"
// prefix never mixes with the proposal/preview/dispatch/recovery ones):
// the creation race (the verification_no ladder) and every lifecycle
// append (the CAS epoch re-validation) serialize per case, the raced
// loser reads the winner's rows under the lock and answers the honest
// current-state refusal — never a masked write.
func orderVerificationLockKey(tenantID, caseID id.UUID) string {
	return "action.order_verification.v1:" + tenantID.String() + ":" + caseID.String()
}

// mapOrderVerificationInsertError maps the raced insert conflicts and the
// CHECK backstops onto the honest consumer-owned signals (the
// mapRecoveryInsertError precedent): the task PK is the per-case ladder
// backstop, the event PK is the monotonic append twin, the material CHECK
// is the loyal domain-mirror refusal — the store never masks a lost race
// as a write.
func mapOrderVerificationInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		return actionPort.ErrVerificationRaced
	}
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23514" {
		return actionPort.ErrVerificationRaced
	}
	return err
}

// orderVerificationColumns is the canonical column list of the task row
// (the SELECT and the INSERT share it — the scan target list mirrors it).
const orderVerificationColumns = `tenant_id, case_id, work_item_id, verification_no, operation_id,
 receipt_version_no, external_order_id, creator_id, created_at`

// scanOrderVerification scans one task row (the shared target list).
func scanOrderVerification(row pgx.Row, tenantID id.UUID) (actionDomain.OrderVerification, error) {
	var task actionDomain.OrderVerification
	if err := row.Scan(&task.TenantID, &task.CaseID, &task.WorkItemID, &task.VerificationNo,
		&task.OperationID, &task.ReceiptVersionNo, &task.ExternalOrderID, &task.CreatorID, &task.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return actionDomain.OrderVerification{}, actionPort.ErrVerificationNotFound
		}
		return actionDomain.OrderVerification{}, err
	}
	if task.TenantID != tenantID {
		return actionDomain.OrderVerification{}, actionPort.ErrVerificationNotFound
	}
	return task, nil
}

// readOrderVerifications reads the task rows of one anchor (the case or
// the operation) ordered by the verification number (the ladder view).
func readOrderVerifications(ctx context.Context, tx pgx.Tx, tenantID id.UUID, where string, anchor id.UUID) ([]actionDomain.OrderVerification, error) {
	rows, err := tx.Query(ctx, `select `+orderVerificationColumns+`
                from action.order_verifications where tenant_id=$1 and `+where+` order by verification_no`,
		tenantID, anchor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []actionDomain.OrderVerification{}
	for rows.Next() {
		var task actionDomain.OrderVerification
		if err := rows.Scan(&task.TenantID, &task.CaseID, &task.WorkItemID, &task.VerificationNo,
			&task.OperationID, &task.ReceiptVersionNo, &task.ExternalOrderID, &task.CreatorID, &task.CreatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// readOrderVerificationEvents reads the append-only events of one task
// ordered by the monotonic event number (the state derivation input).
// The per-kind material columns are NULL for the kinds that carry other
// material — the scan goes through the pointer intermediates (the honest
// zero of an absent material, never a scan error).
func readOrderVerificationEvents(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID, verificationNo int64) ([]actionDomain.OrderVerificationEvent, error) {
	rows, err := tx.Query(ctx, `select tenant_id, case_id, verification_no, event_no, kind,
                assignee, backup_owner, verify_deadline, deadline_at,
                order_digest, receipt_version_no, external_order_id, evidence_anchors,
                positions, reason, actor_id, created_at
                from action.order_verification_events
                where tenant_id=$1 and case_id=$2 and verification_no=$3 order by event_no`,
		tenantID, caseID, verificationNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []actionDomain.OrderVerificationEvent{}
	for rows.Next() {
		var event actionDomain.OrderVerificationEvent
		var assignee, backupOwner, verifyDeadline, orderDigest, externalOrderID, reason *string
		var receiptVersionNo *int64
		var deadlineAt, createdAt *time.Time
		var evidenceAnchors, positions []byte
		if err := rows.Scan(&event.TenantID, &event.CaseID, &event.VerificationNo, &event.EventNo, &event.Kind,
			&assignee, &backupOwner, &verifyDeadline, &deadlineAt,
			&orderDigest, &receiptVersionNo, &externalOrderID, &evidenceAnchors,
			&positions, &reason, &event.ActorID, &createdAt); err != nil {
			return nil, err
		}
		if assignee != nil {
			event.Assignee = id.UUID(*assignee)
		}
		if backupOwner != nil {
			event.BackupOwner = id.UUID(*backupOwner)
		}
		if verifyDeadline != nil {
			event.VerifyDeadline = *verifyDeadline
		}
		if deadlineAt != nil {
			event.DeadlineAt = *deadlineAt
		}
		if orderDigest != nil {
			event.OrderDigest = *orderDigest
		}
		if receiptVersionNo != nil {
			event.ReceiptVersionNo = *receiptVersionNo
		}
		if externalOrderID != nil {
			event.ExternalOrderID = *externalOrderID
		}
		if reason != nil {
			event.Reason = *reason
		}
		if createdAt != nil {
			event.CreatedAt = *createdAt
		}
		if evidenceAnchors != nil {
			decoded, err := decodeStringArray(evidenceAnchors)
			if err != nil {
				return nil, err
			}
			event.EvidenceAnchors = decoded
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

// ReadOperationVerifications reads every task anchored to one operation
// ordered by the verification number (the active-task conflict check and
// the ladder view).
func (s *Postgres) ReadOperationVerifications(ctx context.Context, scope tenancy.Context, operationID id.UUID) ([]actionDomain.OrderVerification, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	tenantID := scope.TenantID()
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tasks, err := readOrderVerifications(ctx, tx, tenantID, "operation_id=$2", operationID)
	if err != nil {
		return nil, err
	}
	return tasks, tx.Commit(ctx)
}

// ReadCaseVerifications reads every task of one case ordered by the
// verification number (the view material of the panel).
func (s *Postgres) ReadCaseVerifications(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]actionDomain.OrderVerification, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	tenantID := scope.TenantID()
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tasks, err := readOrderVerifications(ctx, tx, tenantID, "case_id=$2", caseID)
	if err != nil {
		return nil, err
	}
	return tasks, tx.Commit(ctx)
}

// ReadVerificationEvents reads the append-only events of one task ordered
// by the monotonic event number (the state derivation input).
func (s *Postgres) ReadVerificationEvents(ctx context.Context, scope tenancy.Context, caseID id.UUID, verificationNo int64) ([]actionDomain.OrderVerificationEvent, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	tenantID := scope.TenantID()
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	events, err := readOrderVerificationEvents(ctx, tx, tenantID, caseID, verificationNo)
	if err != nil {
		return nil, err
	}
	return events, tx.Commit(ctx)
}

// AppendOrderVerification creates the task row AND the opening assigned
// event in ONE transaction under the per-case advisory lock: the monotonic
// verification_no derives under the lock (the ladder of the repeated
// verification — the history is never rewritten), the one-active-task rule
// re-validates UNDER the lock (the raced twin of the same operation
// refuses with ErrVerificationActive — the honest 409, never a second
// competing task).
func (s *Postgres) AppendOrderVerification(ctx context.Context, p actionPort.AppendOrderVerificationParams) (actionDomain.OrderVerification, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return actionDomain.OrderVerification{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		orderVerificationLockKey(tenantID, p.CaseID)); err != nil {
		return actionDomain.OrderVerification{}, err
	}
	// The one-active-task rule UNDER the lock: every task of the operation
	// with its events derives its state — an OPEN task refuses the second
	// competing task honestly (the app speaks the current state words).
	anchored, err := readOrderVerifications(ctx, tx, tenantID, "operation_id=$2", p.OperationID)
	if err != nil {
		return actionDomain.OrderVerification{}, err
	}
	for _, task := range anchored {
		events, err := readOrderVerificationEvents(ctx, tx, tenantID, task.CaseID, task.VerificationNo)
		if err != nil {
			return actionDomain.OrderVerification{}, err
		}
		if actionDomain.DeriveOrderVerificationState(task, events, p.Now).Active() {
			return actionDomain.OrderVerification{}, actionPort.ErrVerificationActive
		}
	}
	// The per-case ladder number UNDER the lock (the store truth — the
	// task never carries it).
	var verificationNo int64
	if err := tx.QueryRow(ctx, `
                select coalesce(max(verification_no), 0) + 1 from action.order_verifications
                where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&verificationNo); err != nil {
		return actionDomain.OrderVerification{}, err
	}
	createdAt := p.Now.UTC().Truncate(time.Microsecond)
	task := actionDomain.OrderVerification{
		TenantID:         tenantID,
		CaseID:           p.CaseID,
		WorkItemID:       p.WorkItemID,
		VerificationNo:   verificationNo,
		OperationID:      p.OperationID,
		ReceiptVersionNo: p.ReceiptVersionNo,
		ExternalOrderID:  p.ExternalOrderID,
		CreatorID:        p.CreatorID,
		CreatedAt:        createdAt,
	}
	if _, err := tx.Exec(ctx, `
                insert into action.order_verifications (tenant_id, case_id, work_item_id, verification_no,
                        operation_id, receipt_version_no, external_order_id, creator_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		task.TenantID, task.CaseID, task.WorkItemID, task.VerificationNo,
		task.OperationID, task.ReceiptVersionNo, task.ExternalOrderID, task.CreatorID, task.CreatedAt); err != nil {
		return actionDomain.OrderVerification{}, mapOrderVerificationInsertError(err)
	}
	if _, err := tx.Exec(ctx, `
                insert into action.order_verification_events (tenant_id, case_id, verification_no, event_no, kind,
                        assignee, backup_owner, verify_deadline, deadline_at,
                        order_digest, receipt_version_no, external_order_id, evidence_anchors,
                        positions, reason, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,NULL,NULL,NULL,NULL,NULL,NULL,$10,$11)`,
		tenantID, p.CaseID, verificationNo, int64(1), actionDomain.OrderVerificationEventAssigned,
		p.Assignee, p.BackupOwner, p.VerifyDeadline, p.DeadlineAt.UTC().Truncate(time.Microsecond),
		p.CreatorID, createdAt); err != nil {
		return actionDomain.OrderVerification{}, mapOrderVerificationInsertError(err)
	}
	return task, tx.Commit(ctx)
}

// AppendOrderVerificationEvent appends ONE lifecycle event under the
// per-case advisory lock with the CAS re-validation: the derived state
// and the assignment epoch must EQUAL the command's expectation — a raced
// append (the parallel decision of the second manager, the reassignment,
// the expiry word move) refuses with ErrVerificationRaced and NEVER
// writes. The monotonic event_no derives under the lock.
func (s *Postgres) AppendOrderVerificationEvent(ctx context.Context, p actionPort.AppendOrderVerificationEventParams) (actionDomain.OrderVerificationEvent, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return actionDomain.OrderVerificationEvent{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		orderVerificationLockKey(tenantID, p.CaseID)); err != nil {
		return actionDomain.OrderVerificationEvent{}, err
	}
	// The task re-read UNDER the lock (the anchor must exist — the RLS
	// blindness of a foreign tenant answers the honest 404).
	row := tx.QueryRow(ctx, `select `+orderVerificationColumns+`
                from action.order_verifications where tenant_id=$1 and case_id=$2 and verification_no=$3`,
		tenantID, p.CaseID, p.VerificationNo)
	task, err := scanOrderVerification(row, tenantID)
	if err != nil {
		return actionDomain.OrderVerificationEvent{}, err
	}
	// The state re-derivation UNDER the lock: the CAS expectation of the
	// command must equal the CURRENT derived truth — the raced append
	// never writes (the parallel decision loses honestly, AC-7).
	events, err := readOrderVerificationEvents(ctx, tx, tenantID, task.CaseID, task.VerificationNo)
	if err != nil {
		return actionDomain.OrderVerificationEvent{}, err
	}
	view := actionDomain.DeriveOrderVerificationState(task, events, p.Now)
	if view.State != p.ExpectedState || view.AssignmentEpoch != p.ExpectedEpoch {
		return actionDomain.OrderVerificationEvent{}, actionPort.ErrVerificationRaced
	}
	var eventNo int64
	if err := tx.QueryRow(ctx, `
                select coalesce(max(event_no), 0) + 1 from action.order_verification_events
                where tenant_id=$1 and case_id=$2 and verification_no=$3`,
		tenantID, p.CaseID, p.VerificationNo).Scan(&eventNo); err != nil {
		return actionDomain.OrderVerificationEvent{}, err
	}
	event := p.Event
	event.TenantID = tenantID
	event.CaseID = task.CaseID
	event.VerificationNo = task.VerificationNo
	event.EventNo = eventNo
	event.CreatedAt = p.Now.UTC().Truncate(time.Microsecond)
	var evidenceAnchors, positions any
	if event.EvidenceAnchors != nil {
		evidenceAnchors = encodeStringArray(event.EvidenceAnchors)
	}
	if event.Positions != nil {
		positions = encodeStringArray(event.Positions)
	}
	if _, err := tx.Exec(ctx, `
                insert into action.order_verification_events (tenant_id, case_id, verification_no, event_no, kind,
                        assignee, backup_owner, verify_deadline, deadline_at,
                        order_digest, receipt_version_no, external_order_id, evidence_anchors,
                        positions, reason, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		event.TenantID, event.CaseID, event.VerificationNo, event.EventNo, event.Kind,
		nilIfZero(event.Assignee), nilIfZero(event.BackupOwner),
		nilIfEmpty(event.VerifyDeadline), nilIfZeroTime(event.DeadlineAt),
		nilIfEmpty(event.OrderDigest), nilIfZeroInt(event.ReceiptVersionNo), nilIfEmpty(event.ExternalOrderID),
		evidenceAnchors, positions, nilIfEmpty(event.Reason), event.ActorID, event.CreatedAt); err != nil {
		return actionDomain.OrderVerificationEvent{}, mapOrderVerificationInsertError(err)
	}
	return event, tx.Commit(ctx)
}

// decodeStringArray decodes the stored jsonb string array (the evidence
// anchors / the affected positions — the SAFE pin sets only).
func decodeStringArray(raw []byte) ([]string, error) {
	var values []string
	if len(raw) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

// encodeStringArray renders the jsonb string array of the SAFE pin sets.
func encodeStringArray(values []string) []byte {
	if len(values) == 0 {
		return []byte("[]")
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return []byte("[]")
	}
	return encoded
}

// nilIfZero maps a zero UUID onto the SQL NULL of the per-kind material.
func nilIfZero(value id.UUID) any {
	if !value.Valid() {
		return nil
	}
	return value
}

// nilIfEmpty maps an empty word onto the SQL NULL of the per-kind material.
func nilIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// nilIfZeroTime maps a zero time onto the SQL NULL of the per-kind material.
func nilIfZeroTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

// nilIfZeroInt maps a zero number onto the SQL NULL of the per-kind material.
func nilIfZeroInt(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
