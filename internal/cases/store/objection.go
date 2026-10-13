package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 2.8: the objection record writes and the sales reply objection gate
// ---------------------------------------------------------------------------

// mapObjectionInsertError maps the raced insert conflicts of the objection
// surface onto the honest domain errors: the objection number UNIQUE is the
// monotonic backstop (the loser under the locks is impossible in the normal
// path — the backstop keeps the material truth anyway), the command record
// UNIQUE is the material idempotency conflict.
func mapObjectionInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "objections_tenant_id_work_item_id_objection_no_key":
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

// readObjectionsTx reads every objection record of one case ordered by the
// Work Item and the monotonic number (the derivation input of the CURRENT
// objection, the refusal budget and the panel history).
func readObjectionsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.ObjectionRecord, error) {
	rows, err := tx.Query(ctx, `
                select objection_id, case_id, work_item_id, objection_no, turn_ref, objection_type,
                       client_signal, spans, playbook_version, actor_id, created_at
                from cases.objections
                where tenant_id=$1 and case_id=$2 order by work_item_id, objection_no`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []casesDomain.ObjectionRecord{}
	for rows.Next() {
		var record casesDomain.ObjectionRecord
		var spans []byte
		if err := rows.Scan(&record.ObjectionID, &record.CaseID, &record.WorkItemID,
			&record.ObjectionNo, &record.TurnRef, &record.ObjectionType,
			&record.ClientSignal, &spans, &record.PlaybookVersion, &record.ActorID, &record.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(spans, &record.Spans); err != nil {
			return nil, err
		}
		record.TenantID = tenantID
		record.CreatedAt = record.CreatedAt.UTC()
		records = append(records, record)
	}
	return records, rows.Err()
}

// ReadObjections reads every objection record of one case (the own read
// transaction of the view and the command pre-checks).
func (s *Postgres) ReadObjections(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ObjectionRecord, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	records, err := readObjectionsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return records, tx.Commit(ctx)
}

// readLinkedRepliesTx reads the objection links of the outgoing replies of
// one case: the objection id with the playbook step word of every reply
// row that carries the link (the acknowledged derivation input).
func readLinkedRepliesTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.ReplyLink, error) {
	rows, err := tx.Query(ctx, `
                select objection_id, playbook_step from cases.client_replies
                where tenant_id=$1 and case_id=$2 and objection_id is not null
                order by created_at`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := []casesDomain.ReplyLink{}
	for rows.Next() {
		var link casesDomain.ReplyLink
		if err := rows.Scan(&link.ObjectionID, &link.Step); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

// ReadLinkedReplies reads the objection links of the outgoing replies of
// one case (the own read transaction of the derivation).
func (s *Postgres) ReadLinkedReplies(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ReplyLink, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	links, err := readLinkedRepliesTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return links, tx.Commit(ctx)
}

// objectionTurnRefOfTx re-validates the turn source under the locks: an
// ACCEPTED TEXT event of the case (the span source anchor — the voice turn
// refuses honestly, the excerpt markup of the voice contour belongs to the
// later slices).
func objectionTurnRefOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, eventID id.UUID) error {
	var state, modality string
	err := tx.QueryRow(ctx, `
                select state, modality from cases.channel_events
                where tenant_id=$1 and event_id=$2 and case_id=$3`,
		tenantID, eventID, caseID).Scan(&state, &modality)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.WorkRuleError{Rule: "record_objection", Reason: casesDomain.TurnRefNotFoundText}
	}
	if err != nil {
		return err
	}
	if state != casesDomain.EventStateAccepted {
		return casesDomain.WorkRuleError{Rule: "record_objection", Reason: "Источник возражения не принят: разметка выдержек возможна только на принятых событиях кейса."}
	}
	if modality != casesDomain.ModalityText {
		return casesDomain.WorkRuleError{Rule: "record_objection", Reason: "Разметка выдержек голосовых сообщений — вне текущего срез: источник возражения должен быть текстовым событием."}
	}
	return nil
}

// execRecordObjection writes one objection record under the case + work
// item locks: the Work Item must be anchored in the case and ALIVE (the
// terminal states refuse honestly — the write-command boundary of 2-2),
// the dictionary words are the CHECK backstops, the objection number is
// the per-Work-Item monotonic sequence under the lock, the spans land as
// the canonical JSON array (the app verified the byte-exact containment
// against the sealed payload BEFORE the transaction; the store re-validates
// the anchors and the turn source) and the outbox event carries the
// metadata only (the span text never enters the envelope).
func (s *Postgres) execRecordObjection(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := modeWorkItemOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	currents, err := readWorkItemCurrentsTx(ctx, tx, tenantID, p.CaseID, []id.UUID{p.WorkItemID})
	if err != nil {
		return err
	}
	current, ok := currents[p.WorkItemID]
	if !ok {
		return casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.TerminalWorkItemObjectionText}
	}
	// The turn source anchor under the locks: the accepted TEXT event of
	// the case (the honest refusal words of the boundaries).
	if err := objectionTurnRefOfTx(ctx, tx, tenantID, p.CaseID, p.TurnRef); err != nil {
		return err
	}
	// The dictionary guard under the locks (the honest 400 backstopped by
	// the table CHECK words).
	var spans []string
	if len(p.ObjectionSpans) > 0 {
		if err := json.Unmarshal(p.ObjectionSpans, &spans); err != nil {
			return err
		}
	}
	if err := casesDomain.GuardObjectionRecord(p.ObjectionType, p.ClientSignal, spans); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The per-Work-Item monotonic objection number under the lock (the
	// max+1 of the append-only ladder).
	records, err := readObjectionsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	objectionNo := int64(0)
	for _, record := range records {
		if record.WorkItemID == p.WorkItemID && record.ObjectionNo > objectionNo {
			objectionNo = record.ObjectionNo
		}
	}
	objectionNo++
	if _, err = tx.Exec(ctx, `
                insert into cases.objections (tenant_id, case_id, work_item_id, objection_id, objection_no,
                        turn_ref, objection_type, client_signal, spans, playbook_version, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		tenantID, p.CaseID, p.WorkItemID, p.ObjectionID, objectionNo,
		p.TurnRef, p.ObjectionType, p.ClientSignal, p.ObjectionSpans,
		casesDomain.ObjectionPlaybookVersion, p.Actor, now); err != nil {
		return mapObjectionInsertError(err)
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	// The outbox event: the metadata only — the span text never enters
	// the envelope (the client words live in the sealed payload of 2-1).
	payload, err := casesDomain.ObjectionRecordedPayloadJSON(casesDomain.ObjectionRecordedPayload{
		TenantID: tenantID, CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		ObjectionID: p.ObjectionID, ObjectionNo: objectionNo, TurnRef: p.TurnRef,
		ObjectionType: p.ObjectionType, ClientSignal: p.ClientSignal,
		PlaybookVersion: casesDomain.ObjectionPlaybookVersion,
		ActorID:         p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventObjectionRecorded, casesDomain.OutboxEventObjectionRecordedV1,
		p.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyObjectionRecorded(p.ObjectionID), now); err != nil {
		return mapObjectionInsertError(err)
	}
	return nil
}

// replyObjectionGateOfTx re-validates the objection gate of the sales reply
// under the locks (the 2-6 replyPolicyGateOfTx precedent — the frozen
// order: ownership 2-4 → mode 2-5 → disclosure 2-5 → policy 2-6 → objection
// 2-8 → write): the stopped persuasion refuses with the allowed endings;
// no objection of the Work Item → the step must be empty (the
// byte-identical old path); a CURRENT objection → the step is required,
// dictionary-bounded, inside the playbook of the type and the first linked
// answer is ALWAYS acknowledge_concern. The return is the objection id the
// reply record references (the zero id when the gate does not apply — the
// non-sales classes).
func replyObjectionGateOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID, p casesPort.WorkCommandParams) (id.UUID, error) {
	if p.ReplyClass != casesDomain.ReplyClassSales {
		return id.UUID(""), nil
	}
	records, err := readObjectionsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return id.UUID(""), err
	}
	recordsOfItem := []casesDomain.ObjectionRecord{}
	for _, record := range records {
		if record.WorkItemID == p.WorkItemID {
			recordsOfItem = append(recordsOfItem, record)
		}
	}
	links, err := readLinkedRepliesTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return id.UUID(""), err
	}
	state := casesDomain.DeriveObjectionState(recordsOfItem, links)
	if err := casesDomain.GuardSalesReplyAgainstObjections(state, p.PlaybookStep); err != nil {
		return id.UUID(""), err
	}
	if state.Current == nil {
		return id.UUID(""), nil
	}
	return state.Current.ObjectionID, nil
}
