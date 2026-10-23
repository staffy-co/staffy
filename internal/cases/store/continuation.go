package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 2.10: the continuation link writes, the public surface submissions
// and the panel reads
// ---------------------------------------------------------------------------

// linkLockKey builds the advisory-lock key of one continuation link (the
// public submissions serialize per link: the consumption limit and the
// use rows are the lock-visible truth — a raced double-click derives the
// consumed state and refuses honestly, the PRG landing page stays the
// terminal «отправлено»).
func linkLockKey(tenantID id.UUID, linkID id.UUID) string {
	return "cases.link.v1:" + tenantID.String() + ":" + linkID.String()
}

// mapContinuationInsertError maps the raced insert conflicts of the
// continuation surface onto the honest domain errors: the command record
// UNIQUE is the material idempotency conflict, the revocation UNIQUE is
// the already-ended expectation (the append-only history says so), the
// fact material UNIQUE is the same-value resubmission through the same
// link (the material idempotency of 2.2 — the source may not write the
// same material twice), the fact version PK is the ladder backstop.
func mapContinuationInsertError(err error, command string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "continuation_link_revocations_tenant_id_link_id_key":
			return casesDomain.WorkRuleError{Rule: command, Reason: casesDomain.ContinuationAlreadyRevokedText}
		case "fact_versions_source_material":
			return casesDomain.WorkRuleError{Rule: command, Reason: "Это значение уже отправлено по этой ссылке: повторная отправка того же материала не создаёт новую версию."}
		case "fact_versions_pkey":
			return casesDomain.ErrStaleVersion
		case "continuation_links_pkey":
			// The raced identical issuance (the same link id under the same
			// command key): the loser of the advisory-lock race sees the
			// winner's committed row — the honest stale-retry signal (the
			// app retry replays the stored outcome through the command
			// record; the objection/commitment monotonic-backstop precedent).
			return casesDomain.ErrStaleVersion
		case "continuation_links_tenant_id_token_hash_key":
			// Cryptographically unreachable (the random 256 bits never
			// repeat); the honest mapping keeps the retry path explicit.
			return casesDomain.WorkRuleError{Rule: command, Reason: "Хеш токена ссылки уже хранится: перегенерируйте токен и повторите выдачу."}
		}
	}
	return err
}

// scanContinuationLink reads one link row of the surface.
func scanContinuationLink(scan func(dest ...any) error, tenantID id.UUID) (casesDomain.ContinuationLink, error) {
	var link casesDomain.ContinuationLink
	var purposes []byte
	err := scan(&link.LinkID, &link.CaseID, &link.WorkItemID, &link.ParticipantRef, &link.OwnerEpoch,
		&purposes, &link.Question, &link.Explanation, &link.TTLMinutes, &link.IssuedAt, &link.ExpiresAt,
		&link.ConsumptionLimit, &link.TokenHash, &link.ActorID, &link.CreatedAt)
	if err != nil {
		return link, err
	}
	if err := json.Unmarshal(purposes, &link.Purposes); err != nil {
		return link, err
	}
	link.TenantID = tenantID
	link.IssuedAt = link.IssuedAt.UTC()
	link.ExpiresAt = link.ExpiresAt.UTC()
	link.CreatedAt = link.CreatedAt.UTC()
	return link, nil
}

// readContinuationLinksTx reads every link of one case ordered by the
// issuance time (the panel history input).
func readContinuationLinksTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.ContinuationLink, error) {
	rows, err := tx.Query(ctx, `
                select link_id, case_id, work_item_id, participant_ref, owner_epoch, purposes, question, explanation,
                       ttl_minutes, issued_at, expires_at, consumption_limit, token_hash, actor_id, created_at
                from cases.continuation_links
                where tenant_id=$1 and case_id=$2 order by created_at, link_id`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := []casesDomain.ContinuationLink{}
	for rows.Next() {
		link, err := scanContinuationLink(rows.Scan, tenantID)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

// readContinuationUsesOfCaseTx reads every use row of one case ordered
// by the submission time (the event feed of the panel).
func readContinuationUsesOfCaseTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.ContinuationLinkUse, error) {
	rows, err := tx.Query(ctx, `
                select use_id, case_id, work_item_id, link_id, use_kind, fact_id, fact_version_no, submitted_at
                from cases.continuation_link_uses
                where tenant_id=$1 and case_id=$2 order by submitted_at, use_id`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	uses := []casesDomain.ContinuationLinkUse{}
	for rows.Next() {
		var use casesDomain.ContinuationLinkUse
		var factID *id.UUID
		var factVersionNo *int64
		if err := rows.Scan(&use.UseID, &use.CaseID, &use.WorkItemID, &use.LinkID, &use.UseKind,
			&factID, &factVersionNo, &use.SubmittedAt); err != nil {
			return nil, err
		}
		if factID != nil {
			use.FactID = *factID
			use.FactVersionNo = *factVersionNo
		}
		use.TenantID = tenantID
		use.SubmittedAt = use.SubmittedAt.UTC()
		uses = append(uses, use)
	}
	return uses, rows.Err()
}

// readContinuationUsesOfLinkTx reads every use row of one link ordered
// by the submission time (the derivation input of the consumption
// count).
func readContinuationUsesOfLinkTx(ctx context.Context, tx pgx.Tx, tenantID, linkID id.UUID) ([]casesDomain.ContinuationLinkUse, error) {
	rows, err := tx.Query(ctx, `
                select use_id, case_id, work_item_id, link_id, use_kind, fact_id, fact_version_no, submitted_at
                from cases.continuation_link_uses
                where tenant_id=$1 and link_id=$2 order by submitted_at, use_id`, tenantID, linkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	uses := []casesDomain.ContinuationLinkUse{}
	for rows.Next() {
		var use casesDomain.ContinuationLinkUse
		var factID *id.UUID
		var factVersionNo *int64
		if err := rows.Scan(&use.UseID, &use.CaseID, &use.WorkItemID, &use.LinkID, &use.UseKind,
			&factID, &factVersionNo, &use.SubmittedAt); err != nil {
			return nil, err
		}
		if factID != nil {
			use.FactID = *factID
			use.FactVersionNo = *factVersionNo
		}
		use.TenantID = tenantID
		use.SubmittedAt = use.SubmittedAt.UTC()
		uses = append(uses, use)
	}
	return uses, rows.Err()
}

// readContinuationRevocationsOfCaseTx reads every revocation of one
// case ordered by the creation time (the panel history input).
func readContinuationRevocationsOfCaseTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.ContinuationRevocation, error) {
	rows, err := tx.Query(ctx, `
                select revocation_id, case_id, work_item_id, link_id, reason, actor_id, created_at
                from cases.continuation_link_revocations
                where tenant_id=$1 and case_id=$2 order by created_at, revocation_id`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	revocations := []casesDomain.ContinuationRevocation{}
	for rows.Next() {
		var revocation casesDomain.ContinuationRevocation
		if err := rows.Scan(&revocation.RevocationID, &revocation.CaseID, &revocation.WorkItemID,
			&revocation.LinkID, &revocation.Reason, &revocation.ActorID, &revocation.CreatedAt); err != nil {
			return nil, err
		}
		revocation.TenantID = tenantID
		revocation.CreatedAt = revocation.CreatedAt.UTC()
		revocations = append(revocations, revocation)
	}
	return revocations, rows.Err()
}

// readContinuationDeliveredTx reads the delivered binding of the case:
// the link ids the outgoing reply rows carry (the FR-16 «the main
// channel shows whether the link was delivered» — the delivery is the
// sales reply that wove the URL into its text).
func readContinuationDeliveredTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) (map[id.UUID]bool, error) {
	rows, err := tx.Query(ctx, `
                select link_id from cases.client_replies
                where tenant_id=$1 and case_id=$2 and link_id is not null`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	delivered := map[id.UUID]bool{}
	for rows.Next() {
		var linkID id.UUID
		if err := rows.Scan(&linkID); err != nil {
			return nil, err
		}
		delivered[linkID] = true
	}
	return delivered, rows.Err()
}

// ReadContinuations reads the whole continuation material of one case
// (the own read transaction of the operator panel: the links, the event
// feed, the revocations and the delivered binding — the states are
// derived read-time, never stored).
func (s *Postgres) ReadContinuations(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesPort.ContinuationPanelMaterial, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesPort.ContinuationPanelMaterial{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	material := casesPort.ContinuationPanelMaterial{}
	if material.Links, err = readContinuationLinksTx(ctx, tx, scope.TenantID(), caseID); err != nil {
		return casesPort.ContinuationPanelMaterial{}, err
	}
	if material.Uses, err = readContinuationUsesOfCaseTx(ctx, tx, scope.TenantID(), caseID); err != nil {
		return casesPort.ContinuationPanelMaterial{}, err
	}
	if material.Revocations, err = readContinuationRevocationsOfCaseTx(ctx, tx, scope.TenantID(), caseID); err != nil {
		return casesPort.ContinuationPanelMaterial{}, err
	}
	if material.Delivered, err = readContinuationDeliveredTx(ctx, tx, scope.TenantID(), caseID); err != nil {
		return casesPort.ContinuationPanelMaterial{}, err
	}
	return material, tx.Commit(ctx)
}

// continuationLinkOfTx reads one link by its identifier under the locks
// (the revoke anchor re-validation: the link must exist and belong to
// this case and work item).
func continuationLinkOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, workItemID, linkID id.UUID) (casesDomain.ContinuationLink, error) {
	link, err := scanContinuationLink(tx.QueryRow(ctx, `
                select link_id, case_id, work_item_id, participant_ref, owner_epoch, purposes, question, explanation,
                       ttl_minutes, issued_at, expires_at, consumption_limit, token_hash, actor_id, created_at
                from cases.continuation_links
                where tenant_id=$1 and link_id=$2`, tenantID, linkID).Scan, tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ContinuationLink{}, casesDomain.ErrNotFound
	}
	if err != nil {
		return casesDomain.ContinuationLink{}, err
	}
	if link.CaseID != caseID || link.WorkItemID != workItemID {
		return casesDomain.ContinuationLink{}, casesDomain.ErrNotFound
	}
	return link, nil
}

// continuationLinkByHashTx reads one link by its token hash under the
// tenant scope (the AD-10 honesty: the hash is the ONLY lookup key —
// the scope candidate from the token verified against the stored hash
// BEFORE any case material is read; a wrong tenant simply finds no row
// under the RLS scope, the generic refusal without disclosure).
func continuationLinkByHashTx(ctx context.Context, tx pgx.Tx, tenantID id.UUID, tokenHash []byte) (casesDomain.ContinuationLink, error) {
	link, err := scanContinuationLink(tx.QueryRow(ctx, `
                select link_id, case_id, work_item_id, participant_ref, owner_epoch, purposes, question, explanation,
                       ttl_minutes, issued_at, expires_at, consumption_limit, token_hash, actor_id, created_at
                from cases.continuation_links
                where tenant_id=$1 and token_hash=$2`, tenantID, tokenHash).Scan, tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ContinuationLink{}, casesDomain.ErrNotFound
	}
	if err != nil {
		return casesDomain.ContinuationLink{}, err
	}
	return link, nil
}

// continuationLookupOfTx derives the read-time lookup answer of one
// link inside the transaction: the uses of the link, the revocation
// existence, the CURRENT owner epoch of the case and the derived state
// word in the spec order (the states are never stored — the derivation
// reads the CURRENT material, a dead link never resurrects).
func continuationLookupOfTx(ctx context.Context, tx pgx.Tx, tenantID id.UUID, link casesDomain.ContinuationLink, now time.Time) (casesDomain.ContinuationLinkLookup, error) {
	uses, err := readContinuationUsesOfLinkTx(ctx, tx, tenantID, link.LinkID)
	if err != nil {
		return casesDomain.ContinuationLinkLookup{}, err
	}
	revoked := false
	if err := tx.QueryRow(ctx, `
                select exists(select 1 from cases.continuation_link_revocations
                        where tenant_id=$1 and link_id=$2)`, tenantID, link.LinkID).Scan(&revoked); err != nil {
		return casesDomain.ContinuationLinkLookup{}, err
	}
	terms, _, _, err := readOwnershipMaterialTx(ctx, tx, tenantID, link.CaseID)
	if err != nil {
		return casesDomain.ContinuationLinkLookup{}, err
	}
	ownership := casesDomain.CurrentOwnership(terms)
	if ownership == nil {
		return casesDomain.ContinuationLinkLookup{}, casesDomain.ErrNotFound
	}
	accepted := 0
	for _, use := range uses {
		if casesDomain.ContinuationUseConsumesLimit(use.UseKind) {
			accepted++
		}
	}
	return casesDomain.ContinuationLinkLookup{
		Link: link, Uses: uses, Revoked: revoked, OwnerEpochCurrent: ownership.Epoch,
		State: casesDomain.DeriveContinuationLinkState(casesDomain.ContinuationLinkStateInput{
			Now: now, ExpiresAt: link.ExpiresAt, Revoked: revoked,
			AcceptedCorrections: accepted, ConsumptionLimit: link.ConsumptionLimit,
			OwnerEpochPinned: link.OwnerEpoch, OwnerEpochCurrent: ownership.Epoch,
		}),
	}, nil
}

// LookupContinuationByTokenHash resolves one link by its token hash
// under the tenant scope and derives the state of the given moment (the
// caller owns the clock — the app injects its request moment; the own
// read transaction of the public surface; ErrNotFound is the generic
// refusal material — the caller never distinguishes the causes to the
// client).
func (s *Postgres) LookupContinuationByTokenHash(ctx context.Context, scope tenancy.Context, tokenHash []byte, now time.Time) (casesDomain.ContinuationLinkLookup, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesDomain.ContinuationLinkLookup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	link, err := continuationLinkByHashTx(ctx, tx, scope.TenantID(), tokenHash)
	if err != nil {
		return casesDomain.ContinuationLinkLookup{}, err
	}
	lookup, err := continuationLookupOfTx(ctx, tx, scope.TenantID(), link, now.UTC())
	if err != nil {
		return casesDomain.ContinuationLinkLookup{}, err
	}
	return lookup, tx.Commit(ctx)
}

// execIssueContinuationLink writes one link under the case + work item
// locks: the Work Item must be anchored in the case and ALIVE (the
// terminal states refuse honestly — the continuation never extends a
// finished element), the domain guards re-validate under the locks (the
// dictionary words and the bounds are the table CHECK backstops), the
// owner epoch pin is the CURRENT server-side truth (the store reads it
// under the case lock — the request never carries it), the expiry is
// the issuance plus the ttl minutes and the outbox event carries the
// metadata only (the token, the question and the explanation never
// enter the envelope).
func (s *Postgres) execIssueContinuationLink(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
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
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.TerminalWorkItemContinuationText}
	}
	// The domain guards under the locks (the honest 400 backstopped by the
	// table CHECK words).
	var purposes []string
	if len(p.ContinuationPurposes) > 0 {
		if err := json.Unmarshal(p.ContinuationPurposes, &purposes); err != nil {
			return err
		}
	}
	if err := casesDomain.GuardContinuationPurposes(purposes); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	if err := casesDomain.GuardContinuationTTL(p.ContinuationTTLMinutes); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	if err := casesDomain.GuardContinuationConsumptionLimit(p.ContinuationLimit); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	if err := casesDomain.GuardContinuationTexts(p.ContinuationQuestion, p.ContinuationExplanation); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	if len(p.ContinuationParticipantRef) < 1 || len(p.ContinuationParticipantRef) > 128 {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "Адрес участника ссылки обязан быть 1..128 символами (канал клиента, которому вплетается URL)."}
	}
	if len(p.ContinuationTokenHash) != 32 {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "Хеш токена ссылки обязан быть SHA-256 (ровно 32 байта)."}
	}
	// The owner epoch pin: the CURRENT term under the case lock (the
	// server-side truth — the request never carries it; the ownership
	// writers serialize on the same case key).
	terms, _, _, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	ownership := casesDomain.CurrentOwnership(terms)
	if ownership == nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "Обращение без владельца не выдаёт ссылок продолжения: сначала закрепите Response Owner."}
	}
	expiresAt := now.Add(time.Duration(p.ContinuationTTLMinutes) * time.Minute)
	if _, err = tx.Exec(ctx, `
                insert into cases.continuation_links (tenant_id, case_id, work_item_id, link_id, participant_ref,
                        owner_epoch, purposes, question, explanation, ttl_minutes, issued_at, expires_at,
                        consumption_limit, token_hash, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		tenantID, p.CaseID, p.WorkItemID, p.LinkID, p.ContinuationParticipantRef,
		ownership.Epoch, p.ContinuationPurposes, p.ContinuationQuestion, p.ContinuationExplanation,
		p.ContinuationTTLMinutes, now, expiresAt, p.ContinuationLimit, p.ContinuationTokenHash, p.Actor, now); err != nil {
		return mapContinuationInsertError(err, p.Command)
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	// The outbox event: the metadata only — the TOKEN and the
	// question/explanation texts never enter the envelope (the token
	// exists only in the command answer and on the operator side).
	payload, err := casesDomain.ContinuationLinkIssuedPayloadJSON(casesDomain.ContinuationLinkIssuedPayload{
		TenantID: tenantID, CaseID: p.CaseID, WorkItemID: p.WorkItemID, LinkID: p.LinkID,
		Purposes: purposes, ExpiresAt: expiresAt, ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventContinuationLinkIssued, casesDomain.OutboxEventContinuationV1,
		p.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyContinuationLinkIssued(p.LinkID), now); err != nil {
		return mapContinuationInsertError(err, p.Command)
	}
	return nil
}

// execRevokeContinuationLink writes one revocation under the case + work
// item locks: the link must exist and belong to this case and work item,
// the reason is bounded (1..500 runes), the row mutates NOTHING (the
// read-time derivation reads it and blocks the surface — the revocation
// has NO outbox event: the state is never an envelope, the panel
// derives it read-time). The UNIQUE backstop keeps the second revoke of
// the same link the honest refusal (the expectation ended once).
func (s *Postgres) execRevokeContinuationLink(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := modeWorkItemOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	if _, err := continuationLinkOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.LinkID); err != nil {
		return err
	}
	if err := casesDomain.GuardReason(p.ContinuationReason); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	if _, err := tx.Exec(ctx, `
                insert into cases.continuation_link_revocations (tenant_id, case_id, work_item_id, link_id,
                        revocation_id, reason, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8)`,
		tenantID, p.CaseID, p.WorkItemID, p.LinkID, p.RevocationID, p.ContinuationReason, p.Actor, now); err != nil {
		return mapContinuationInsertError(err, p.Command)
	}
	return nil
}

// SubmitContinuationCorrection writes one public fact correction: the
// whole action is ONE transaction over the advisory locks of every
// anchor (the case, the work item of the link, the link itself, the fact
// key and the fact id — the sorted global order), the link is re-resolved
// by the token hash and re-derived under the locks (the state must be
// valid: the expired, revoked, consumed and owner-stale words all refuse
// with the same honest store reason — the page never distinguishes them
// to the client), the fact must be a CONFIRMED fact of the work item of
// the link, the value passes the domain fact guard (the same 2.2
// validation as the operator record branch), the NEW version lands in
// the unverified state (the operator confirms through the штатный 2.2
// command — the surface never confirms by itself) with the provenance
// channel protected_continuation and the source ref of the link, the use
// row consumes the limit and the outbox event carries the metadata only
// (the corrected VALUE never enters the envelope). The actor of every
// written row is the resolved link id — the capability IS the credential
// of the public surface (the ingress connection precedent: the verified
// actor of the no-principal path).
func (s *Postgres) SubmitContinuationCorrection(ctx context.Context, p casesPort.ContinuationCorrectionParams) (int64, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := p.Now.UTC().Truncate(time.Microsecond)
	tenantID := p.Scope.TenantID()
	// The lock plan pre-read (everything is re-validated under the locks):
	// the link by the hash, then the fact anchor for the material key.
	link, err := continuationLinkByHashTx(ctx, tx, tenantID, p.TokenHash)
	if err != nil {
		return 0, err
	}
	var anchorCase id.UUID
	var anchorKey string
	if err := tx.QueryRow(ctx, `
                select case_id, material_key from cases.facts where tenant_id=$1 and fact_id=$2`,
		tenantID, p.FactID).Scan(&anchorCase, &anchorKey); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, casesDomain.ErrNotFound
		}
		return 0, err
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, casesDomain.ErrNotFound
		}
		return 0, err
	}
	lockKeys := []string{
		caseLockKey(tenantID, link.CaseID),
		workLockKey(tenantID, link.WorkItemID),
		linkLockKey(tenantID, link.LinkID),
		factKeyLockKey(tenantID, link.CaseID, anchorKey),
		factIDLockKey(tenantID, p.FactID),
	}
	sort.Strings(lockKeys)
	for _, key := range lockKeys {
		if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", key); err != nil {
			return 0, err
		}
	}
	// The state derivation under the locks (the consumption limit is the
	// lock-visible truth — the raced double-click derives consumed).
	lookup, err := continuationLookupOfTx(ctx, tx, tenantID, link, now)
	if err != nil {
		return 0, err
	}
	if lookup.State != casesDomain.ContinuationStateValid {
		return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: casesDomain.ContinuationNotUsableText}
	}
	// The fact gate under the locks: the anchor belongs to the case of the
	// link, the CURRENT scope is the work item of the link, the fact is
	// CONFIRMED (the surface offers the confirmed facts only).
	if anchorCase != link.CaseID {
		return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: casesDomain.ContinuationFactNotCorrectableText}
	}
	material, err := readFactMaterialTx(ctx, tx, tenantID, link.CaseID, p.FactID)
	if err != nil {
		return 0, err
	}
	if len(material.Versions) == 0 || material.Versions[len(material.Versions)-1].WorkItemID != link.WorkItemID {
		return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: casesDomain.ContinuationFactNotCorrectableText}
	}
	status, _, _ := casesDomain.ConfirmedFactView(material.Versions, material.Confirmations)
	if status != casesDomain.FactStatusConfirmed {
		return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: casesDomain.ContinuationFactNotCorrectableText}
	}
	// The work item boundary of 2.2: a terminal scope refuses the fact
	// write (the same honest boundary as the operator record branch).
	currents, err := readWorkItemCurrentsTx(ctx, tx, tenantID, link.CaseID, []id.UUID{link.WorkItemID})
	if err != nil {
		return 0, err
	}
	current, ok := currents[link.WorkItemID]
	if !ok {
		return 0, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: casesDomain.TerminalWorkItemContinuationText}
	}
	// The value guard of 2.2 (the same validation as the operator record
	// branch: the canonical JSON, the 2 KiB bound, the stable numbers).
	value, valueDigest, err := casesDomain.FactValue(p.Value)
	if err != nil {
		return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: err.Error()}
	}
	versionNo := int64(len(material.Versions)) + 1
	// The new version: the unverified state (the confirmation is the
	// operator's штатный command, never the surface), the provenance
	// channel protected_continuation, the source ref of the link, the
	// capability as the actor.
	if err := insertFactVersion(ctx, tx, tenantID, link.CaseID, p.FactID, link.WorkItemID, versionNo,
		material.Anchor.MaterialKey, value, valueDigest,
		casesDomain.ContinuationCorrectionSourceType, link.LinkID.String(),
		casesDomain.FactChannelProtectedContinuation, link.LinkID, now, now, now); err != nil {
		return 0, mapContinuationInsertError(err, "continuation_correct")
	}
	// The use row consumes the limit (the accepted correction).
	if _, err = tx.Exec(ctx, `
                insert into cases.continuation_link_uses (tenant_id, case_id, work_item_id, link_id, use_id,
                        use_kind, fact_id, fact_version_no, submitted_at, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		tenantID, link.CaseID, link.WorkItemID, link.LinkID, p.UseID,
		casesDomain.ContinuationUseFactCorrectionSubmitted, p.FactID, versionNo, now, now); err != nil {
		return 0, mapContinuationInsertError(err, "continuation_correct")
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, link.CaseID).Scan(&sourceConnection); err != nil {
		return 0, err
	}
	// The outbox event: the metadata only — the corrected VALUE never
	// enters the envelope (the «кандидат не покидает строку» precedent).
	payload, err := casesDomain.ContinuationSubmittedPayloadJSON(casesDomain.ContinuationSubmittedPayload{
		TenantID: tenantID, CaseID: link.CaseID, WorkItemID: link.WorkItemID, LinkID: link.LinkID,
		UseID: p.UseID, FactID: p.FactID, FactVersionNo: versionNo,
		ActorID: link.LinkID, OccurredAt: now,
	})
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventContinuationSubmitted, casesDomain.OutboxEventContinuationV1,
		link.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyContinuationSubmitted(p.UseID), now); err != nil {
		return 0, mapContinuationInsertError(err, "continuation_correct")
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return versionNo, nil
}

// SubmitContinuationEvent writes one public human/return event: the
// transaction over the case + link locks (the event touches no work
// item material — the WI/факты/owner никогда не мутируются), the state
// must be valid under the locks, the use row carries no material and
// consumes NOTHING (the human request and the return are events, not
// data submissions) and the outbox event carries the identifiers and
// the use kind only.
func (s *Postgres) SubmitContinuationEvent(ctx context.Context, p casesPort.ContinuationEventParams) error {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := p.Now.UTC().Truncate(time.Microsecond)
	tenantID := p.Scope.TenantID()
	link, err := continuationLinkByHashTx(ctx, tx, tenantID, p.TokenHash)
	if err != nil {
		return err
	}
	if p.UseKind != casesDomain.ContinuationUseHumanRequested && p.UseKind != casesDomain.ContinuationUseReturnedToConversation {
		return casesDomain.WorkRuleError{Rule: "continuation_event", Reason: fmt.Sprintf("Вид события продолжения «%s» не входит в словарь публичных действий (%s, %s)",
			p.UseKind, casesDomain.ContinuationUseHumanRequested, casesDomain.ContinuationUseReturnedToConversation)}
	}
	lockKeys := []string{
		caseLockKey(tenantID, link.CaseID),
		linkLockKey(tenantID, link.LinkID),
	}
	sort.Strings(lockKeys)
	for _, key := range lockKeys {
		if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", key); err != nil {
			return err
		}
	}
	lookup, err := continuationLookupOfTx(ctx, tx, tenantID, link, now)
	if err != nil {
		return err
	}
	if lookup.State != casesDomain.ContinuationStateValid {
		return casesDomain.WorkRuleError{Rule: "continuation_event", Reason: casesDomain.ContinuationNotUsableText}
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.continuation_link_uses (tenant_id, case_id, work_item_id, link_id, use_id,
                        use_kind, fact_id, fact_version_no, submitted_at, created_at)
                values ($1,$2,$3,$4,$5,$6,null,null,$7,$8)`,
		tenantID, link.CaseID, link.WorkItemID, link.LinkID, p.UseID, p.UseKind, now, now); err != nil {
		return mapContinuationInsertError(err, "continuation_event")
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, link.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	eventType := casesDomain.OutboxEventContinuationHumanRequested
	if p.UseKind == casesDomain.ContinuationUseReturnedToConversation {
		eventType = casesDomain.OutboxEventContinuationReturned
	}
	payload, err := casesDomain.ContinuationEventPayloadJSON(casesDomain.ContinuationEventPayload{
		TenantID: tenantID, CaseID: link.CaseID, WorkItemID: link.WorkItemID, LinkID: link.LinkID,
		UseID: p.UseID, UseKind: p.UseKind, ActorID: link.LinkID, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, eventType, casesDomain.OutboxEventContinuationV1,
		link.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyContinuationEvent(p.UseID), now); err != nil {
		return mapContinuationInsertError(err, "continuation_event")
	}
	return tx.Commit(ctx)
}

// SubmitVehicleConfirmation writes the Story 4.2 client confirmation of
// the vehicle identification attributes in one transaction: the link by
// the hash, the case + work item + link + fact locks, the state
// re-derived under the locks (a dead link refuses; the raced
// double-click derives consumed — the same idempotency-by-consumption
// as the correction), every confirmation binds the EXACT version the
// client saw with confirmer=client (the first producer of the reserved
// 2.2 word) and the note carrying the link reference (the evidence
// discipline of 2.2: the note branch — the public page is no case
// event), the use row consumes the limit with the material pair of the
// primary fact, the metadata-only outbox event reuses the
// continuation_submitted vocabulary (no values in the envelope — the
// «кандидат не покидает строку» precedent). Zero new fact versions: a
// confirmation never rewrites a fact.
func (s *Postgres) SubmitVehicleConfirmation(ctx context.Context, p casesPort.SubmitVehicleConfirmationParams) error {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := p.Now.UTC().Truncate(time.Microsecond)
	tenantID := p.Scope.TenantID()
	if len(p.Confirmations) == 0 {
		return casesDomain.WorkRuleError{Rule: "vehicle_confirm", Reason: "Подтверждение пусто: отметьте хотя бы один признак автомобиля."}
	}
	link, err := continuationLinkByHashTx(ctx, tx, tenantID, p.TokenHash)
	if err != nil {
		return err
	}
	// The lock plan pre-read: the material keys of the confirmed anchors
	// (the fact-key locks sort with the case/work/link keys).
	lockKeys := []string{
		caseLockKey(tenantID, link.CaseID),
		workLockKey(tenantID, link.WorkItemID),
		linkLockKey(tenantID, link.LinkID),
	}
	anchors := make(map[id.UUID]string, len(p.Confirmations))
	for _, item := range p.Confirmations {
		var anchorCase id.UUID
		var anchorKey string
		if err := tx.QueryRow(ctx, `
                select case_id, material_key from cases.facts where tenant_id=$1 and fact_id=$2`,
			tenantID, item.FactID).Scan(&anchorCase, &anchorKey); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return casesDomain.ErrNotFound
			}
			return err
		}
		if anchorCase != link.CaseID {
			return casesDomain.WorkRuleError{Rule: "vehicle_confirm", Reason: casesDomain.ContinuationFactNotCorrectableText}
		}
		anchors[item.FactID] = anchorKey
		lockKeys = append(lockKeys,
			factKeyLockKey(tenantID, link.CaseID, anchorKey),
			factIDLockKey(tenantID, item.FactID))
	}
	sort.Strings(lockKeys)
	for _, key := range lockKeys {
		if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", key); err != nil {
			return err
		}
	}
	// The state derivation under the locks (the raced double-click derives
	// consumed — the second POST redirects to the terminal page).
	lookup, err := continuationLookupOfTx(ctx, tx, tenantID, link, now)
	if err != nil {
		return err
	}
	if lookup.State != casesDomain.ContinuationStateValid {
		return casesDomain.WorkRuleError{Rule: "vehicle_confirm", Reason: casesDomain.ContinuationNotUsableText}
	}
	// The per-fact gates under the locks: the CURRENT scope is the work
	// item of the link (the surface discipline of 2.10 — one link, one
	// work item material), the confirmed version exists, the client-path
	// guard bounds the note evidence.
	note := fmt.Sprintf("Клиент подтвердил по защищённой ссылке продолжения %s", link.LinkID)
	primary := casesPort.VehicleConfirmationItem{}
	for _, item := range p.Confirmations {
		material, err := readFactMaterialTx(ctx, tx, tenantID, link.CaseID, item.FactID)
		if err != nil {
			return err
		}
		if len(material.Versions) == 0 || material.Versions[len(material.Versions)-1].WorkItemID != link.WorkItemID {
			return casesDomain.WorkRuleError{Rule: "vehicle_confirm", Reason: casesDomain.ContinuationFactNotCorrectableText}
		}
		if err := casesDomain.GuardVehicleClientConfirmation(len(material.Versions), item.VersionNo, note); err != nil {
			return casesDomain.WorkRuleError{Rule: "vehicle_confirm", Reason: err.Error()}
		}
		if primary.FactID == "" {
			primary = item
		}
	}
	// The confirmation rows: append-only, bound to the EXACT versions,
	// confirmer=client (the first producer of the reserved dictionary
	// word — the SQL CHECK of 00002 admitted it always), the link as the
	// acting capability (the correction precedent).
	for _, item := range p.Confirmations {
		if _, err = tx.Exec(ctx, `
                insert into cases.fact_confirmations (tenant_id, fact_id, confirmation_id, version_no, confirmer_type, evidence_event_id, note, actor_id, created_at)
                values ($1,$2,$3,$4,'client',null,$5,$6,$7)`,
			tenantID, item.FactID, item.ConfirmationID, item.VersionNo, note, link.LinkID, now); err != nil {
			return mapContinuationInsertError(err, "vehicle_confirm")
		}
	}
	// The use row consumes the limit (a data submission) with the
	// material pair of the primary fact.
	if _, err = tx.Exec(ctx, `
                insert into cases.continuation_link_uses (tenant_id, case_id, work_item_id, link_id, use_id,
                        use_kind, fact_id, fact_version_no, submitted_at, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		tenantID, link.CaseID, link.WorkItemID, link.LinkID, p.UseID,
		casesDomain.ContinuationUseVehicleConfirmationSubmitted, primary.FactID, primary.VersionNo, now, now); err != nil {
		return mapContinuationInsertError(err, "vehicle_confirm")
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, link.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	// The metadata-only outbox event (the continuation_submitted
	// vocabulary reused — the bounded event CHECK of 00010 stays intact;
	// the use row carries the vehicle_confirmation_submitted word).
	payload, err := casesDomain.ContinuationSubmittedPayloadJSON(casesDomain.ContinuationSubmittedPayload{
		TenantID: tenantID, CaseID: link.CaseID, WorkItemID: link.WorkItemID, LinkID: link.LinkID,
		UseID: p.UseID, FactID: primary.FactID, FactVersionNo: primary.VersionNo,
		ActorID: link.LinkID, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventContinuationSubmitted, casesDomain.OutboxEventContinuationV1,
		link.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyContinuationSubmitted(p.UseID), now); err != nil {
		return mapContinuationInsertError(err, "vehicle_confirm")
	}
	return tx.Commit(ctx)
}
