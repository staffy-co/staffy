package store

import (
	"context"
	"encoding/json"
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
// Story 2.14: the summary record write and the decision context snapshot
// ---------------------------------------------------------------------------

// TerminalCaseSummaryText is the honest refusal of the summary write on
// a terminal case: the service is over — the derived representation has
// nothing to record (the v1 backstop word of the degradation and
// handoff precedents).
const TerminalCaseSummaryText = "Терминальное обращение не принимает записей сводки: обслуживание завершено — материал передан истории."

// SummaryNotOwnerText is the honest refusal of the summary record when
// the current response owner is NOT Staffy (the ownership gate words of
// 2.4): the summary is the ACT of the owner-Staffy of the moment —
// after the acknowledge the human owns the case, and the honest wait of
// the return precedes the new summary (a new owner writes a NEW row,
// never an edit of the old one).
const SummaryNotOwnerText = "Сводку пишет Staffy, пока владеет обращением: текущий владелец — человек; дождитесь возврата владения (return_to_staffy) — новая сводка будет новой записью."

// SummaryClaimRefForeignText is the honest refusal of the ref guard:
// every linked claim anchors a fact or an observation of THIS case — a
// foreign or missing ref is a fabrication the store refuses (the
// belonging discipline of the turn and conversation guards).
const SummaryClaimRefForeignText = "Утверждение сводки ссылается на материал другого обращения или несуществующий материал: ссылки утверждают факты и наблюдения этого обращения."

// mapSummaryInsertError maps the raced insert conflicts of the summary
// surface onto the honest domain errors: the summary number UNIQUE is
// the raced derivation (the loser of the advisory-lock race sees the
// winner's committed number — the honest stale-retry signal, the
// handoff precedent). The command record UNIQUE never surfaces here:
// the command row is inserted AFTER the exec by the work flow with its
// own mapping; a defensive pass-through keeps the fallback honest.
func mapSummaryInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		if pgErr.ConstraintName == "case_summaries_pkey" {
			// The raced summary number: the loser re-derives under the locks
			// and retries (the honest stale-retry signal).
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

// scanRecordedSummary reads one append-only summary row (the claims
// travel the canonical jsonb of the domain material — the snake_case
// keys of the SQL CHECK).
func scanRecordedSummary(scan func(dest ...any) error, tenantID id.UUID) (casesDomain.RecordedSummary, error) {
	var summary casesDomain.RecordedSummary
	var claims []byte
	err := scan(&summary.CaseID, &summary.SummaryNo, &summary.Kind,
		&summary.ActorID, &summary.Text, &claims,
		&summary.Watermark.EventCount, &summary.Watermark.FactVersionCount,
		&summary.Watermark.OwnerEpoch, &summary.Watermark.ModeVersion, &summary.CreatedAt)
	if err != nil {
		return summary, err
	}
	summary.TenantID = tenantID
	summary.CreatedAt = summary.CreatedAt.UTC()
	if err := json.Unmarshal(claims, &summary.Claims); err != nil {
		return summary, err
	}
	return summary, nil
}

// readLastSummaryTx reads the LAST summary row of the case (the maximal
// summary number — the append-only history is never rewritten; the
// earlier rows stay the readable history of the section).
func readLastSummaryTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) (*casesDomain.RecordedSummary, error) {
	rows, err := tx.Query(ctx, `
                select case_id, summary_no, kind, actor_id, text, claims,
                       event_count, fact_version_count, owner_epoch, mode_version, created_at
                from cases.case_summaries
                where tenant_id=$1 and case_id=$2 order by summary_no desc limit 1`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	summary, err := scanRecordedSummary(rows.Scan, tenantID)
	if err != nil {
		return nil, err
	}
	return &summary, rows.Err()
}

// summaryWatermarkTx derives the SERVER truth watermark of the case
// under the locks: the accepted-event count, the fact-version count,
// the current owner epoch and the current mode version. The request
// NEVER carries the watermark — this snapshot is the recorded truth of
// the moment (the staleness derivation compares it with the CURRENT
// counters read-time; the summary never mutates).
func summaryWatermarkTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) (casesDomain.SummaryWatermark, error) {
	var watermark casesDomain.SummaryWatermark
	err := tx.QueryRow(ctx, `
                select
                        (select count(*) from cases.channel_events
                         where tenant_id=$1 and case_id=$2 and state='accepted'),
                        (select count(*) from cases.fact_versions
                         where tenant_id=$1 and case_id=$2),
                        (select coalesce(max(epoch),0) from cases.ownership_terms
                         where tenant_id=$1 and case_id=$2),
                        (select coalesce(max(mode_version),0) from cases.mode_terms
                         where tenant_id=$1 and case_id=$2)`,
		tenantID, caseID).Scan(&watermark.EventCount, &watermark.FactVersionCount,
		&watermark.OwnerEpoch, &watermark.ModeVersion)
	return watermark, err
}

// summaryClaimGuard re-validates every LINKED claim ref under the locks:
// a fact_version ref must be a fact anchor of THIS case, an observation
// ref must be an observation row of THIS case (the belonging discipline
// — the store refuses a foreign or missing ref honestly; the disputed
// claims carry no ref and need no guard).
func summaryClaimGuard(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID, claims []casesDomain.SummaryClaim, command string) error {
	for _, claim := range claims {
		if !casesDomain.ClaimRefTypeInDictionary(claim.RefType) || !claim.RefID.Valid() {
			continue
		}
		switch claim.RefType {
		case casesDomain.ClaimRefFactVersion:
			var found int
			err := tx.QueryRow(ctx, `
                                select 1 from cases.facts where tenant_id=$1 and case_id=$2 and fact_id=$3`,
				tenantID, caseID, claim.RefID).Scan(&found)
			if errors.Is(err, pgx.ErrNoRows) {
				return casesDomain.WorkRuleError{Rule: command, Reason: SummaryClaimRefForeignText}
			}
			if err != nil {
				return err
			}
		case casesDomain.ClaimRefObservation:
			var found int
			err := tx.QueryRow(ctx, `
                                select 1 from cases.owning_system_observations where tenant_id=$1 and case_id=$2 and observation_id=$3`,
				tenantID, caseID, claim.RefID).Scan(&found)
			if errors.Is(err, pgx.ErrNoRows) {
				return casesDomain.WorkRuleError{Rule: command, Reason: SummaryClaimRefForeignText}
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// execRecordCaseSummary writes one append-only summary row under the
// case + conversation locks (the sorted lock plan of the 2.5..2.13
// precedent): the case must exist under the tenant scope (the honest
// 404 of the non-participant path) and stay open (the terminal
// backstop), the OWNERSHIP GATE runs BEFORE the material guards — the
// current response owner must be Staffy (the summary is the act of the
// owner-Staffy of the moment; a human owner gets the honest refusal
// words, never a material write), the domain guard re-validates the
// material under the locks (the kind word, the rune bounds, the claim
// discipline with the byte-exact spans), every LINKED claim ref is
// re-validated as the material of THIS case, the watermark is the
// SERVER truth snapshot under the locks, and the summary number is the
// NEXT monotonic number of the case (the request never carries it; the
// UNIQUE backstop catches the race the locks already serialize). The
// outbox envelope carries the identifiers, the kind word and the claim
// counters ONLY — the summary TEXT and the claims NEVER enter.
func (s *Postgres) execRecordCaseSummary(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
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
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: TerminalCaseSummaryText}
	}
	// The ownership gate BEFORE the material guards: the summary is the
	// act of the owner-Staffy (the 2.4 discipline — a human owner waits
	// for the return; the honest refusal carries no material words).
	ownershipTerms, _, _, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	current := casesDomain.CurrentOwnership(ownershipTerms)
	if current == nil {
		return casesDomain.ErrNotFound
	}
	if current.OwnerKind != casesDomain.OwnerKindStaffy {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: SummaryNotOwnerText}
	}
	// The domain guard re-validates the material under the locks (the
	// kind word, the bounds, the claim discipline with the byte-exact
	// spans — the machine checks the structure, never the truth).
	if err := casesDomain.GuardRecordCaseSummary(p.Kind, p.SummaryText, p.SummaryClaims); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The ref guards: every linked claim anchors the material of THIS
	// case (a foreign ref is a fabrication the store refuses).
	if err := summaryClaimGuard(ctx, tx, tenantID, p.CaseID, p.SummaryClaims, p.Command); err != nil {
		return err
	}
	// The watermark: the SERVER truth snapshot under the locks (the
	// request never carries it).
	watermark, err := summaryWatermarkTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	// The summary number: the NEXT monotonic number under the locks (the
	// request never carries it; the UNIQUE backstop catches the race).
	var summaryNo int64
	if err = tx.QueryRow(ctx, `
                select coalesce(max(summary_no),0)+1 from cases.case_summaries
                where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&summaryNo); err != nil {
		return err
	}
	claims, err := json.Marshal(p.SummaryClaims)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.case_summaries (tenant_id, case_id, summary_no, kind, actor_id, text, claims,
                        event_count, fact_version_count, owner_epoch, mode_version, created_at)
                values ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9,$10,$11,$12)`,
		tenantID, p.CaseID, summaryNo, p.Kind, p.Actor, p.SummaryText, claims,
		watermark.EventCount, watermark.FactVersionCount, watermark.OwnerEpoch, watermark.ModeVersion, now); err != nil {
		return mapSummaryInsertError(err)
	}
	// The summary envelope: the identifiers, the kind word and the claim
	// counters only — the summary text and the claims NEVER enter (the
	// perimeter of the outbox stays identifiers-and-words).
	linked, disputed := casesDomain.ClaimCountersOf(p.SummaryClaims)
	payload, err := casesDomain.SummaryRecordedPayloadJSON(casesDomain.SummaryRecordedPayload{
		TenantID: tenantID, CaseID: p.CaseID, SummaryNo: summaryNo, Kind: p.Kind,
		ClaimCount: len(p.SummaryClaims), LinkedClaims: linked, DisputedClaims: disputed,
		ActorID: p.Actor, OccurredAt: now,
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
		tenantID, p.SummaryOutboxEventID, casesDomain.OutboxEventSummaryRecorded, casesDomain.OutboxEventSummaryRecordedV1,
		p.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeySummaryRecorded(p.CaseID, summaryNo), now); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
			// The raced outbox envelope of the same summary number (the
			// replay never reaches here — the command record resolves
			// before the exec; the honest stale-retry signal).
			return casesDomain.ErrStaleVersion
		}
		return err
	}
	return nil
}

// readDecisionContextTx reads the ONE consistent material snapshot of
// every context block in ONE read transaction: the open handoff with
// the events, the work items, the mode ladder, the verifications, the
// ownership terms, the facts with the confirmations, the observations,
// the replies with the delivery events, the commitments with the
// confirmations, the accommodations, the policy outcomes, the last
// summary and the CURRENT watermark counters. The advisory locks are
// NOT taken (the consistent snapshot of one transaction is the honest
// boundary of the GET); the derivations — the states, the staleness,
// the words — are the read-time domain truth over this snapshot, never
// a stored status. The release lineage word stays empty here (the
// routing facts live outside the cases schema — the app layer resolves
// it for the confirmation validity derivation).
func readDecisionContextTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID, now time.Time) (casesDomain.DecisionContextInput, error) {
	input := casesDomain.DecisionContextInput{Now: now}
	var caseState string
	err := tx.QueryRow(ctx, `
                select state from cases.cases where tenant_id=$1 and case_id=$2`,
		tenantID, caseID).Scan(&caseState)
	if errors.Is(err, pgx.ErrNoRows) {
		return input, casesDomain.ErrNotFound
	}
	if err != nil {
		return input, err
	}

	// The open handoff with the whole lifecycle history (the states are
	// the read-time derivation — the defensive open scan of the handoff
	// store keeps the answer honest).
	requests, events, err := readHandoffsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}
	input.HandoffEvents = events
	if open := handoffOpenView(requests, events, now); open != nil {
		request := open.Request
		input.HandoffRequest = &request
	}

	// The work items with the current ladder views (the terminal
	// completions filter out at the derivation).
	items, err := tx.Query(ctx, "select "+currentViewColumns+" "+currentViewJoin+`
                where w.tenant_id=$1 and w.case_id=$2 order by w.created_at, w.work_item_id`, tenantID, caseID)
	if err != nil {
		return input, err
	}
	defer items.Close()
	input.WorkItems = []casesDomain.WorkItemCurrent{}
	for items.Next() {
		current, err := scanCurrent(items.Scan)
		if err != nil {
			return input, err
		}
		current.Terminal = casesDomain.TerminalWorkItemStates[current.State]
		input.WorkItems = append(input.WorkItems, current)
	}
	if err := items.Err(); err != nil {
		return input, err
	}

	input.ModeTerms, err = readModeMaterialTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}
	input.Verifications, err = readVerificationRecordsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}
	input.OwnershipTerms, _, _, err = readOwnershipMaterialTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}
	if owner := casesDomain.CurrentOwnership(input.OwnershipTerms); owner != nil {
		input.CurrentOwnerEpoch = owner.Epoch
	}

	// The facts with the confirmations (the flat material of the
	// derivation — the status words render read-time).
	factMaterials, err := readFactsOfCaseTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}
	input.FactVersions = []casesDomain.FactVersion{}
	input.FactConfirmations = []casesDomain.FactConfirmation{}
	for _, material := range factMaterials {
		input.FactVersions = append(input.FactVersions, material.Versions...)
		input.FactConfirmations = append(input.FactConfirmations, material.Confirmations...)
	}

	input.Observations, err = readObservationsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}

	// The replies with the delivery events (the acceptance, the delivery
	// and the business completion never mix — the state words render
	// read-time over the dispatch truth and the append-only evidence).
	// The dispatch truth is the 2.4 drain OUTCOME word — exactly the
	// truth the 2.12 surface reads (the LEFT JOIN of ReadClientReplies):
	// without the dispatched outcome the reply stays prepared (nothing
	// has left — a stale_needs_review reply never left either); the
	// acceptance by the machine is never fabricated here.
	replyRows, err := tx.Query(ctx, `
                select r.reply_id, o.kind
                from cases.client_replies r
                left join cases.client_reply_outcomes o on o.tenant_id = r.tenant_id and o.reply_id = r.reply_id
                where r.tenant_id=$1 and r.case_id=$2 order by r.reply_id`, tenantID, caseID)
	if err != nil {
		return input, err
	}
	defer replyRows.Close()
	type replyTruth struct {
		replyID    id.UUID
		dispatched bool
	}
	replies := []replyTruth{}
	for replyRows.Next() {
		var replyID id.UUID
		var outcomeKind *string
		if err := replyRows.Scan(&replyID, &outcomeKind); err != nil {
			return input, err
		}
		replies = append(replies, replyTruth{
			replyID:    replyID,
			dispatched: outcomeKind != nil && *outcomeKind == casesDomain.ReplyOutcomeDispatched,
		})
	}
	if err := replyRows.Err(); err != nil {
		return input, err
	}
	deliveryEvents, err := readDeliveryEventsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}
	input.Replies = make([]casesDomain.ReplyDeliveryMaterial, 0, len(replies))
	for _, reply := range replies {
		material := casesDomain.ReplyDeliveryMaterial{ReplyID: reply.replyID, Dispatched: reply.dispatched}
		for _, event := range deliveryEvents {
			if event.ReplyID == reply.replyID {
				material.Events = append(material.Events, event)
			}
		}
		input.Replies = append(input.Replies, material)
	}

	input.CommitmentVersions, err = readCommitmentsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}
	input.Confirmations, err = readConfirmationsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}
	input.Accommodations, err = readAccommodationsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}
	input.Outcomes, err = readPolicyOutcomesTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}

	// The last summary with the CURRENT watermark counters (the
	// staleness derivation compares them read-time; the summary never
	// mutates).
	input.Summary, err = readLastSummaryTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}
	input.Watermark, err = summaryWatermarkTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return input, err
	}
	return input, nil
}

// ReadDecisionContext reads the ONE consistent decision context
// snapshot of the case (the own read transaction of the operator
// section — the derivations are the read-time domain truth over this
// material, never stored statuses).
func (s *Postgres) ReadDecisionContext(ctx context.Context, scope tenancy.Context, caseID id.UUID, now time.Time) (casesDomain.DecisionContextInput, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesDomain.DecisionContextInput{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	input, err := readDecisionContextTx(ctx, tx, scope.TenantID(), caseID, now)
	if err != nil {
		return casesDomain.DecisionContextInput{}, err
	}
	return input, tx.Commit(ctx)
}
