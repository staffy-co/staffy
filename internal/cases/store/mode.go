package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 2.5: the mode and disclosure reads, the four command executions
// and the reply class extension of send_client_reply
// ---------------------------------------------------------------------------

// mapModeInsertError maps the raced insert conflicts of the mode surface
// onto the honest domain errors: the mode version UNIQUE is the CAS race
// backstop (the loser gets the honest stale-version conflict), the
// command record UNIQUE is the material idempotency conflict.
func mapModeInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "mode_terms_tenant_id_conversation_id_mode_version_key":
			return casesDomain.ErrStaleVersion
		case "disclosure_records_tenant_id_conversation_id_disclosure_no_key":
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

// readModeMaterialTx reads the mode terms of one case ordered by the
// version (the derivation input of the active conversation mode, the Work
// Item contexts and the reply mode gate).
func readModeMaterialTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.ModeTerm, error) {
	rows, err := tx.Query(ctx, `
                select term_id, case_id, conversation_id, mode_version, mode, scope, work_item_id,
                       origin, reason, signal_source, turn_ref, actor_id, prior_mode, exit_authority, rule_version, created_at
                from cases.mode_terms
                where tenant_id=$1 and case_id=$2 order by mode_version`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	terms := []casesDomain.ModeTerm{}
	for rows.Next() {
		var term casesDomain.ModeTerm
		var workItemID, turnRef *id.UUID
		var reason, priorMode, authority *string
		if err := rows.Scan(&term.TermID, &term.CaseID, &term.ConversationID, &term.ModeVersion, &term.Mode,
			&term.Scope, &workItemID, &term.Origin, &reason, &term.SignalSource, &turnRef, &term.ActorID,
			&priorMode, &authority, &term.RuleVersion, &term.CreatedAt); err != nil {
			return nil, err
		}
		if workItemID != nil {
			term.WorkItemID = *workItemID
		}
		if turnRef != nil {
			term.TurnRef = *turnRef
		}
		if reason != nil {
			term.Reason = *reason
		}
		if priorMode != nil {
			term.PriorMode = *priorMode
		}
		if authority != nil {
			term.ExitAuthority = *authority
		}
		term.TenantID = tenantID
		terms = append(terms, term)
	}
	return terms, rows.Err()
}

// ReadModeMaterial reads the mode terms of one case (the own read
// transaction of the view and the command pre-checks).
func (s *Postgres) ReadModeMaterial(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ModeTerm, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	terms, err := readModeMaterialTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return terms, tx.Commit(ctx)
}

// readDisclosuresTx reads the disclosure records with their delivery
// events of one case ordered by the record number (the derivation input
// of the acting disclosure of a channel and the disclosure cards of the
// view).
func readDisclosuresTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesPort.DisclosureCard, error) {
	rows, err := tx.Query(ctx, `
                select disclosure_id, case_id, conversation_id, disclosure_no, channel, text, parent_ref,
                       employee_id, decision_no, profile_version, actor_id, created_at
                from cases.disclosure_records
                where tenant_id=$1 and case_id=$2 order by conversation_id, disclosure_no`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cards := []casesPort.DisclosureCard{}
	index := map[id.UUID]int{}
	for rows.Next() {
		var record casesDomain.DisclosureRecord
		var parentRef *id.UUID
		if err := rows.Scan(&record.DisclosureID, &record.CaseID, &record.ConversationID, &record.DisclosureNo,
			&record.Channel, &record.Text, &parentRef, &record.EmployeeID, &record.DecisionNo,
			&record.ProfileVersion, &record.ActorID, &record.CreatedAt); err != nil {
			return nil, err
		}
		if parentRef != nil {
			record.ParentRef = *parentRef
		}
		record.TenantID = tenantID
		cards = append(cards, casesPort.DisclosureCard{Record: record, Events: []casesDomain.DisclosureDeliveryEvent{}})
		index[record.DisclosureID] = len(cards) - 1
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	eventRows, err := tx.Query(ctx, `
                select event_id, disclosure_id, event_no, kind, actor_id, created_at
                from cases.disclosure_delivery_events
                where tenant_id=$1 order by disclosure_id, event_no`, tenantID)
	if err != nil {
		return nil, err
	}
	defer eventRows.Close()
	for eventRows.Next() {
		var event casesDomain.DisclosureDeliveryEvent
		if err := eventRows.Scan(&event.EventID, &event.DisclosureID, &event.EventNo, &event.Kind,
			&event.ActorID, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.TenantID = tenantID
		if at, ok := index[event.DisclosureID]; ok {
			cards[at].Events = append(cards[at].Events, event)
		}
	}
	if err := eventRows.Err(); err != nil {
		return nil, err
	}
	for i := range cards {
		cards[i].Status = casesDomain.DisclosureStatus(cards[i].Events, cards[i].Record.DisclosureID)
	}
	return cards, nil
}

// ReadDisclosures reads the disclosure records with their events of one
// case (the own read transaction of the view and the command pre-checks).
func (s *Postgres) ReadDisclosures(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesPort.DisclosureCard, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cards, err := readDisclosuresTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return cards, tx.Commit(ctx)
}

// ReadLiveConversationChannel reads the channel of the latest accepted
// event of one conversation (the live route word; false when the
// conversation has no accepted event — the honest degradation).
func (s *Postgres) ReadLiveConversationChannel(ctx context.Context, scope tenancy.Context, conversationID id.UUID) (string, bool, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var channel string
	err = tx.QueryRow(ctx, `
		select channel from cases.channel_events
		where tenant_id=$1 and conversation_id=$2 and state='accepted'
		order by received_at desc limit 1`, scope.TenantID(), conversationID).Scan(&channel)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, tx.Commit(ctx)
	}
	if err != nil {
		return "", false, err
	}
	return channel, true, tx.Commit(ctx)
}

// modeConversationOfTx re-validates the conversation of a mode command
// under the locks: it must be the case's OWN conversation (the ladder
// anchor of the dialog).
func modeConversationOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, conversationID id.UUID) error {
	var owner id.UUID
	err := tx.QueryRow(ctx, `
                select response_owner_id from cases.conversations
                where tenant_id=$1 and conversation_id=$2 and case_id=$3`,
		tenantID, conversationID, caseID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.WorkRuleError{Rule: "mode", Reason: "Режим управляется на собственном диалоге обращения: выберите диалог этого обращения."}
	}
	return err
}

// modeWorkItemOfTx re-validates the Work Item anchor under the locks: the
// item must be anchored in this case.
func modeWorkItemOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, workItemID id.UUID) error {
	var found int
	err := tx.QueryRow(ctx, `
                select 1 from cases.work_items
                where tenant_id=$1 and work_item_id=$2 and case_id=$3`,
		tenantID, workItemID, caseID).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.WorkRuleError{Rule: "mode", Reason: "Элемент работы не найден в обращении: контекст режима привязан к элементу этого обращения."}
	}
	return err
}

// modeTurnRefOfTx re-validates the turn reference under the locks: an
// accepted event of the case.
func modeTurnRefOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, eventID id.UUID) error {
	var found int
	err := tx.QueryRow(ctx, `
                select 1 from cases.channel_events
                where tenant_id=$1 and event_id=$2 and case_id=$3 and state='accepted'`,
		tenantID, eventID, caseID).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.WorkRuleError{Rule: "propose_mode", Reason: casesDomain.TurnRefNotFoundText}
	}
	return err
}

// modeConflictOf builds the machine-readable raced-transition refusal
// with the CURRENT mode words (the honest 409 — the loser re-reads the
// state).
func modeConflictOf(active *casesDomain.ModeTerm, version int64) error {
	word := casesDomain.EffectiveWorkModeWord(active)
	reason, signal := "", ""
	if active != nil {
		reason, signal = active.Reason, active.SignalSource
	}
	return casesDomain.ModeConflictError{ActiveMode: word, Reason: reason, SignalSource: signal, RuleVersion: casesDomain.ModePolicyVersion, ModeVersion: version}
}

// insertModeTerm writes one transition term and its mode_changed outbox
// event — the single atomic write of every mode transition. The source
// connection of the outbox record is the connection of the case's
// original conversation (the insertOwnershipTerm precedent).
func insertModeTerm(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, term casesDomain.ModeTerm, now time.Time) error {
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, term.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
                insert into cases.mode_terms (tenant_id, case_id, conversation_id, term_id, mode_version, mode, scope, work_item_id,
                        origin, reason, signal_source, turn_ref, actor_id, prior_mode, exit_authority, rule_version, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		tenantID, term.CaseID, term.ConversationID, term.TermID, term.ModeVersion, term.Mode, term.Scope,
		nilID(term.WorkItemID), term.Origin, nilText(term.Reason), term.SignalSource, nilID(term.TurnRef),
		term.ActorID, nilText(term.PriorMode), nilText(term.ExitAuthority), term.RuleVersion, now); err != nil {
		return mapModeInsertError(err)
	}
	payload, err := casesDomain.ModeChangedPayloadJSON(casesDomain.ModeChangedPayload{
		TenantID: tenantID, CaseID: term.CaseID, ConversationID: term.ConversationID, TermID: term.TermID,
		ModeVersion: term.ModeVersion, Mode: term.Mode, Scope: term.Scope,
		Reason: term.Reason, SignalSource: term.SignalSource, ActorID: term.ActorID, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventModeChanged, casesDomain.OutboxEventModeChangedV1,
		term.CaseID, sourceConnection, term.TermID, now, payload,
		casesDomain.DedupKeyModeChanged(term.TermID), now); err != nil {
		return mapModeInsertError(err)
	}
	return nil
}

// execProposeMode writes the raised term under the case + conversation
// locks: the conversation must be the case's own, the Work Item anchor
// (the item scope) must be anchored in the case, the turn reference must
// be an accepted event of the case, the CAS expectation must match the
// current maximal version, and the deterministic ladder decision
// re-validates — the raced twin maps onto the stale version conflict
// with the current mode words.
func (s *Postgres) execProposeMode(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := modeConversationOfTx(ctx, tx, tenantID, p.CaseID, p.ConversationID); err != nil {
		return err
	}
	if p.ModeScope == casesDomain.ModeScopeWorkItem {
		if err := modeWorkItemOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
			return err
		}
	}
	if p.SignalSource == casesDomain.ModeSignalTurnEvent {
		if err := modeTurnRefOfTx(ctx, tx, tenantID, p.CaseID, p.TurnRef); err != nil {
			return err
		}
	}
	terms, err := readModeMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	active := casesDomain.ActiveConversationMode(terms)
	// The CAS word comes FIRST: the raced twin of the transition gets the
	// honest 409 with the current mode words (the loser re-reads the
	// state); the ladder decision of the matching version follows.
	if current := casesDomain.ModeTermMaxVersion(terms); current != p.ExpectedModeVersion {
		return modeConflictOf(active, current)
	}
	context := active
	if p.ModeScope == casesDomain.ModeScopeWorkItem {
		context = casesDomain.WorkItemModeContext(terms, p.WorkItemID)
	}
	if err := casesDomain.GuardProposeTransition(p.ModeScope, p.Mode, active, context); err != nil {
		return err
	}
	prior := casesDomain.EffectiveWorkMode(active, context)
	priorMode := casesDomain.EffectiveWorkModeWord(prior)
	return insertModeTerm(ctx, tx, tenantID, p, casesDomain.ModeTerm{
		TenantID: tenantID, CaseID: p.CaseID, ConversationID: p.ConversationID, TermID: p.ModeTermID,
		ModeVersion: p.ExpectedModeVersion + 1, Mode: p.Mode, Scope: p.ModeScope, WorkItemID: p.WorkItemID,
		Origin: casesDomain.ModeOriginProposal, Reason: p.Reason, SignalSource: p.SignalSource,
		TurnRef: p.TurnRef, ActorID: p.Actor, PriorMode: priorMode, RuleVersion: casesDomain.ModePolicyVersion,
		CreatedAt: now,
	}, now)
}

// execExitMode writes the supervised lowering term under the case +
// conversation locks: the ladder decision re-validates (the exit lowers
// only), the CAS expectation must match, and the authority word of the
// app's server-side resolution lands in the term (operator_owner or
// supervisor — the safety mode is the supervision word only).
func (s *Postgres) execExitMode(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := modeConversationOfTx(ctx, tx, tenantID, p.CaseID, p.ConversationID); err != nil {
		return err
	}
	terms, err := readModeMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	active := casesDomain.ActiveConversationMode(terms)
	// The CAS word comes FIRST (the raced twin gets the honest 409).
	if current := casesDomain.ModeTermMaxVersion(terms); current != p.ExpectedModeVersion {
		return modeConflictOf(active, current)
	}
	if err := casesDomain.GuardExitTransition(p.Mode, active); err != nil {
		return err
	}
	return insertModeTerm(ctx, tx, tenantID, p, casesDomain.ModeTerm{
		TenantID: tenantID, CaseID: p.CaseID, ConversationID: p.ConversationID, TermID: p.ModeTermID,
		ModeVersion: p.ExpectedModeVersion + 1, Mode: p.Mode, Scope: casesDomain.ModeScopeConversation,
		Origin: casesDomain.ModeOriginExitOverride, Reason: p.Reason, SignalSource: casesDomain.ModeSignalOperator,
		ActorID: p.Actor, PriorMode: casesDomain.EffectiveWorkModeWord(active),
		ExitAuthority: p.ExitAuthority, RuleVersion: casesDomain.ModePolicyVersion, CreatedAt: now,
	}, now)
}

// execRecordDisclosure writes the disclosure record with its born
// generated delivery event and the disclosure_recorded outbox record
// under the case + conversation locks: the conversation must be the
// case's own, the parent reference (the re-disclosure) must be a record
// of the same conversation, the record number is the per-conversation
// monotonic sequence under the lock, and the lineage words are the
// command's honest re-read snapshot (the reply precedent).
func (s *Postgres) execRecordDisclosure(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := modeConversationOfTx(ctx, tx, tenantID, p.CaseID, p.ConversationID); err != nil {
		return err
	}
	if p.ParentRef.Valid() {
		var found int
		err := tx.QueryRow(ctx, `
                        select 1 from cases.disclosure_records
                        where tenant_id=$1 and disclosure_id=$2 and conversation_id=$3`,
			tenantID, p.ParentRef, p.ConversationID).Scan(&found)
		if errors.Is(err, pgx.ErrNoRows) {
			return casesDomain.WorkRuleError{Rule: "record_disclosure", Reason: "Родительская запись раскрытия не найдена в диалоге: повторное раскрытие ссылается на запись этого диалога."}
		}
		if err != nil {
			return err
		}
	}
	var disclosureNo int64
	if err := tx.QueryRow(ctx, `
                select coalesce(max(disclosure_no),0)+1 from cases.disclosure_records
                where tenant_id=$1 and conversation_id=$2`, tenantID, p.ConversationID).Scan(&disclosureNo); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
                insert into cases.disclosure_records (tenant_id, case_id, conversation_id, disclosure_id, disclosure_no, channel, text, parent_ref, employee_id, decision_no, profile_version, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		tenantID, p.CaseID, p.ConversationID, p.DisclosureID, disclosureNo, p.Channel, p.DisclosureText,
		nilID(p.ParentRef), p.LineageEmployeeID, p.LineageDecisionNo, p.LineageProfileVersion, p.Actor, now); err != nil {
		return mapModeInsertError(err)
	}
	// The born delivery event: the record does not act yet (generated).
	if _, err := tx.Exec(ctx, `
                insert into cases.disclosure_delivery_events (tenant_id, event_id, disclosure_id, event_no, kind, actor_id, created_at)
                values ($1,$2,$3,1,$4,$5,$6)`,
		tenantID, p.DeliveryEventID, p.DisclosureID, casesDomain.DeliveryGenerated, p.Actor, now); err != nil {
		return mapModeInsertError(err)
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	payload, err := casesDomain.DisclosureRecordedPayloadJSON(casesDomain.DisclosureRecordedPayload{
		TenantID: tenantID, CaseID: p.CaseID, ConversationID: p.ConversationID,
		DisclosureID: p.DisclosureID, DisclosureNo: disclosureNo, Channel: p.Channel,
		ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventDisclosureRecorded, casesDomain.OutboxEventDisclosureRecordedV1,
		p.CaseID, sourceConnection, p.DisclosureID, now, payload,
		casesDomain.DedupKeyDisclosureRecorded(p.DisclosureID), now); err != nil {
		return mapModeInsertError(err)
	}
	return nil
}

// execRecordDisclosureEvent appends one delivery observation under the
// case + conversation locks: the disclosure record must belong to this
// conversation, the event number is the per-record monotonic sequence
// under the lock (the last event is the record's status).
func (s *Postgres) execRecordDisclosureEvent(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := modeConversationOfTx(ctx, tx, tenantID, p.CaseID, p.ConversationID); err != nil {
		return err
	}
	var found int
	err := tx.QueryRow(ctx, `
                select 1 from cases.disclosure_records
                where tenant_id=$1 and disclosure_id=$2 and conversation_id=$3`,
		tenantID, p.DisclosureID, p.ConversationID).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	var eventNo int64
	if err := tx.QueryRow(ctx, `
                select coalesce(max(event_no),0)+1 from cases.disclosure_delivery_events
                where tenant_id=$1 and disclosure_id=$2`, tenantID, p.DisclosureID).Scan(&eventNo); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
                insert into cases.disclosure_delivery_events (tenant_id, event_id, disclosure_id, event_no, kind, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7)`,
		tenantID, p.DeliveryEventID, p.DisclosureID, eventNo, p.DeliveryKind, p.Actor, now)
	return mapModeInsertError(err)
}

// replyModeGateOfTx re-validates the mode gate of the client reply under
// the locks: the effective mode of the Work Item the reply belongs to
// (the stricter of the conversation mode and the item context) decides
// the sales class; the pre-migration absence is the fail-closed
// non-sales word.
func replyModeGateOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID, p casesPort.WorkCommandParams) error {
	if p.WorkItemID.Valid() {
		if err := modeWorkItemOfTx(ctx, tx, tenantID, caseID, p.WorkItemID); err != nil {
			return err
		}
	}
	terms, err := readModeMaterialTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return err
	}
	active := casesDomain.ActiveConversationMode(terms)
	context := active
	if p.WorkItemID.Valid() {
		context = casesDomain.WorkItemModeContext(terms, p.WorkItemID)
	}
	if err := casesDomain.GuardReplyClassAgainstMode(p.ReplyClass, casesDomain.EffectiveWorkMode(active, context)); err != nil {
		if gate, ok := err.(casesDomain.ReplyModeError); ok {
			gate.Reason = casesDomain.ReplyModeGateText
			return gate
		}
		return err
	}
	return nil
}

// replyDisclosureGateOfTx re-validates the disclosure gate of the sales
// and the protected replies under the locks: the acting disclosure of the
// channel of the reply (the live route word — the channel of the latest
// accepted event of the conversation, the honest degradation to the
// command word when no accepted event exists).
func replyDisclosureGateOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, conversationID id.UUID, p casesPort.WorkCommandParams) error {
	channel := p.RouteChannel
	if err := tx.QueryRow(ctx, `
                select channel from cases.channel_events
                where tenant_id=$1 and conversation_id=$2 and state='accepted'
                order by received_at desc limit 1`, tenantID, conversationID).Scan(&channel); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		channel = p.RouteChannel
	}
	cards, err := readDisclosuresTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return err
	}
	records := make([]casesDomain.DisclosureRecord, 0, len(cards))
	events := make([]casesDomain.DisclosureDeliveryEvent, 0, len(cards))
	for _, card := range cards {
		records = append(records, card.Record)
		events = append(events, card.Events...)
	}
	if casesDomain.EffectiveDisclosure(records, events, conversationID, channel) == nil {
		return casesDomain.ReplyDisclosureError{Channel: channel, Reason: casesDomain.DisclosureGateText}
	}
	return nil
}
