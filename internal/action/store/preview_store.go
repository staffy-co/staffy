package store

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// Story 3.4: the preview persistence of the action module
// ---------------------------------------------------------------------------
//
// Every table is tenant-scoped with RLS ENABLE+FORCE and append-only by
// grants (INSERT/SELECT only). The two advisory locks serialize the two
// ladders: the proposal lock (the AD-7 attempt ladder of one derived key)
// and the preview lock (the version ladder of one preparation thread),
// taken in the deterministic sorted order (the 2.2 discipline). The
// ladders are re-derived INSIDE the locks — the raced twin is the honest
// stale refusal, never a silent merge. The keyed digest is re-computed
// from the material under the locks with the per-tenant commitment key v1
// (the 2.9/3.3 reuse — the ensure pattern over cases.commitment_hmac_keys;
// the token space carries the action_preview:v1: prefix, materially
// distinct from commitment:v1: and external_commitment:v1:).
//
// The store owns NO adapters (the 3-3 persist-before-emit discipline): no
// transactional method can call the network because no adapter exists
// to call — the material snapshot is read by the app BEFORE the
// transaction, and the dry-run validation evidence is appended AFTER the
// adapter answered. The signatures stay pure persistence boundaries.

// Postgres is the action persistence (the 3-3 Postgres precedent: the
// pool, the RLS scope settings, the UTC canonicalization at the boundary).
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres wires the store over the shared runtime pool.
func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

// The structural pin: the store satisfies the consumer-owned repository
// contract of the port package.
var _ actionPort.PreviewRepository = (*Postgres)(nil)

// begin opens a tenant-scoped transaction: the RLS settings are applied
// before any query (the staffy.principal_id / staffy.tenant_id pair — the
// cases store precedent).
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

// The advisory lock keys of the two ladders (the deterministic sorted
// order is the caller's — lockPreviewThread takes them in the fixed
// alphabetical order below).
func proposalLockKey(tenantID id.UUID, proposalKey string) string {
	return "action.proposal.v1:" + tenantID.String() + ":" + proposalKey
}

func previewLockKey(tenantID id.UUID, previewID id.UUID) string {
	return "action.preview.v1:" + tenantID.String() + ":" + previewID.String()
}

// mapActionInsertError maps the raced insert conflicts onto the honest
// consumer-owned errors: the attempt PK is the AD-7 ladder backstop, the
// version PK is the monotonic ladder backstop, the event UNIQUE is the
// raced append twin.
func mapActionInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "action_proposals_pkey":
			return actionPort.ErrStalePreviewVersion
		case "action_previews_pkey":
			return actionPort.ErrStalePreviewVersion
		case "action_preview_events_tenant_id_preview_id_version_no_event_no_key":
			return actionPort.ErrStalePreviewVersion
		}
	}
	return err
}

// readPreviewKeyTx reads (or creates on the first use) the per-tenant
// commitment key inside a caller transaction (the 2.9/3.3 ensure pattern:
// INSERT ON CONFLICT DO NOTHING, then SELECT — the INSERT-only discipline,
// the rotation honestly deferred). The key is shared with the commitment
// and the external commitment token spaces; the action_preview:v1:
// prefix of PreviewDigest keeps the spaces apart.
func readPreviewKeyTx(ctx context.Context, tx pgx.Tx, scope tenancy.Context, now time.Time) ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate preview key entropy: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into cases.commitment_hmac_keys (tenant_id, key, created_at)
		values ($1,$2,$3) on conflict (tenant_id) do nothing`,
		scope.TenantID(), key, now); err != nil {
		return nil, err
	}
	var stored []byte
	if err := tx.QueryRow(ctx, `
		select key from cases.commitment_hmac_keys where tenant_id=$1`, scope.TenantID()).Scan(&stored); err != nil {
		return nil, err
	}
	return stored, nil
}

// ReadPreviewKey reads (or creates on the first use) the per-tenant key of
// the preview digest (the own committed transaction — the app computes
// the digest BEFORE the command transaction; the store re-computes it
// under the locks).
func (s *Postgres) ReadPreviewKey(ctx context.Context, scope tenancy.Context, now time.Time) ([]byte, error) {
	tx, err := s.begin(ctx, scope, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	key, err := readPreviewKeyTx(ctx, tx, scope, now.UTC().Truncate(time.Microsecond))
	if err != nil {
		return nil, err
	}
	return key, tx.Commit(ctx)
}

// readAttemptsTx reads the attempt ladder of one proposal key ordered by
// the attempt number (the AD-7 input).
func readAttemptsTx(ctx context.Context, tx pgx.Tx, tenantID id.UUID, proposalKey string) ([]actionPort.ProposalAttemptRow, error) {
	rows, err := tx.Query(ctx, `
		select proposal_key, attempt_no, preview_id, definition_id, definition_version, case_id, work_item_id,
		       target_kind, target_ref, verdict, blocked_reason, invalid_report, actor_id, created_at
		from action.action_proposals
		where tenant_id=$1 and proposal_key=$2 order by attempt_no`, tenantID, proposalKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attempts := []actionPort.ProposalAttemptRow{}
	for rows.Next() {
		var row actionPort.ProposalAttemptRow
		if err := rows.Scan(&row.ProposalKey, &row.AttemptNo, &row.PreviewID, &row.DefinitionID, &row.DefinitionVersion,
			&row.CaseID, &row.WorkItemID, &row.TargetKind, &row.TargetRef,
			&row.Verdict, &row.BlockedReason, &row.InvalidReport, &row.ActorID, &row.CreatedAt); err != nil {
			return nil, err
		}
		row.CreatedAt = row.CreatedAt.UTC()
		attempts = append(attempts, row)
	}
	return attempts, rows.Err()
}

// maxVersionOfTx re-derives the version ladder of one preview thread
// under the locks (the raced build is the honest stale refusal).
func maxVersionOfTx(ctx context.Context, tx pgx.Tx, tenantID id.UUID, previewID id.UUID) (int64, error) {
	var maxVersion int64
	err := tx.QueryRow(ctx, `
		select coalesce(max(version_no), 0) from action.action_previews
		where tenant_id=$1 and preview_id=$2`, tenantID, previewID).Scan(&maxVersion)
	if err != nil {
		return 0, err
	}
	return maxVersion, nil
}

// maxEventNoOfTx reads the monotonic event counter of one version scope
// (0 = the proposal stage) under the locks.
func maxEventNoOfTx(ctx context.Context, tx pgx.Tx, tenantID id.UUID, previewID id.UUID, versionNo int64) (int64, error) {
	var maxEvent int64
	err := tx.QueryRow(ctx, `
		select coalesce(max(event_no), 0) from action.action_preview_events
		where tenant_id=$1 and preview_id=$2 and version_no=$3`, tenantID, previewID, versionNo).Scan(&maxEvent)
	if err != nil {
		return 0, err
	}
	return maxEvent, nil
}

// marshalJSON renders the canonical jsonb material of one column (the
// domain renders the deterministic bytes; the CHECK is the loyal
// backstop).
func marshalJSON(material any) ([]byte, error) {
	if material == nil {
		return []byte("null"), nil
	}
	rendered, err := json.Marshal(material)
	if err != nil {
		return nil, err
	}
	return rendered, nil
}

// insertAttemptRow writes one append-only attempt row (the ladder
// backstop is the PK — the raced twin maps to the honest stale refusal).
func insertAttemptRow(ctx context.Context, tx pgx.Tx, tenantID id.UUID, row actionPort.ProposalAttemptRow, now time.Time) error {
	if _, err := tx.Exec(ctx, `
		insert into action.action_proposals (tenant_id, proposal_key, attempt_no, preview_id, definition_id, definition_version,
			case_id, work_item_id, target_kind, target_ref, verdict, blocked_reason, invalid_report, actor_id, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		tenantID, row.ProposalKey, row.AttemptNo, row.PreviewID, row.DefinitionID, row.DefinitionVersion,
		row.CaseID, row.WorkItemID, row.TargetKind, row.TargetRef,
		row.Verdict, row.BlockedReason, row.InvalidReport, row.ActorID, now); err != nil {
		return mapActionInsertError(err)
	}
	return nil
}

// insertEventRow writes one append-only event row with the re-derived
// monotonic event number of its version scope.
func insertEventRow(ctx context.Context, tx pgx.Tx, tenantID id.UUID, previewID id.UUID, versionNo int64, caseID id.UUID,
	eventID id.UUID, kind, reason, details string, actorID id.UUID, now time.Time) error {
	maxEvent, err := maxEventNoOfTx(ctx, tx, tenantID, previewID, versionNo)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		insert into action.action_preview_events (tenant_id, preview_id, version_no, case_id, event_id, event_no, kind, reason, details, actor_id, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, previewID, versionNo, caseID, eventID, maxEvent+1, kind, reason, details, actorID, now); err != nil {
		return mapActionInsertError(err)
	}
	return nil
}

// insertVersionRow writes one append-only version row (the PK is the
// ladder backstop).
func insertVersionRow(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p actionPort.BuildPreviewVersionParams) error {
	preview := p.Preview
	typedInput, err := marshalJSON(preview.TypedInput)
	if err != nil {
		return err
	}
	facts, err := marshalJSON(preview.SupportingFacts)
	if err != nil {
		return err
	}
	observations, err := marshalJSON(preview.SupportingObservations)
	if err != nil {
		return err
	}
	snapshot, err := marshalJSON(preview.Snapshot)
	if err != nil {
		return err
	}
	effects, err := marshalJSON(preview.Effects)
	if err != nil {
		return err
	}
	var supersedes any
	if p.SupersedesVersion > 0 {
		supersedes = p.SupersedesVersion
	}
	if _, err := tx.Exec(ctx, `
		insert into action.action_previews (tenant_id, preview_id, version_no, supersedes_version, definition_id, definition_version,
			case_id, work_item_id, target_kind, target_ref, typed_input, supporting_facts, supporting_observations, snapshot, effects,
			partition, recovery_instruction, human_owner_required, unsafe_class, no_reconciliation, human_only_control_mark,
			owner_epoch, control_generation, profile_version, policy_outcome_no, capability_decl_version, digest, author_id, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29)`,
		tenantID, preview.PreviewID, preview.VersionNo, supersedes, preview.DefinitionID, preview.DefinitionVersion,
		preview.CaseID, preview.WorkItemID, preview.Target.Kind, preview.Target.Ref,
		typedInput, facts, observations, snapshot, effects,
		preview.Partition.Partition, preview.Partition.RecoveryInstruction,
		preview.Partition.HumanOwnerRequired, preview.Partition.UnsafeClass,
		preview.Partition.NoProvenReconciliationPath, preview.HumanOnlyControlMark,
		preview.OwnerEpoch, preview.ControlGeneration, preview.ProfileVersion, preview.PolicyOutcomeNo,
		preview.CapabilityDeclVersion, preview.Digest, preview.AuthorID,
		preview.CreatedAt.UTC().Truncate(time.Microsecond)); err != nil {
		return mapActionInsertError(err)
	}
	return nil
}

// lockPreviewThread takes the preview advisory lock and — when the build
// carries an attempt row — the proposal advisory lock after it (the
// deterministic alphabetical order: action.preview… sorts before
// action.proposal…; every writer takes the same order, the 2.2
// discipline).
func lockPreviewThread(ctx context.Context, tx pgx.Tx, tenantID id.UUID, previewID id.UUID, proposalKey string) error {
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", previewLockKey(tenantID, previewID)); err != nil {
		return err
	}
	if proposalKey != "" {
		if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", proposalLockKey(tenantID, proposalKey)); err != nil {
			return err
		}
	}
	return nil
}

// ReadProposalAttempts reads the attempt ladder of one proposal key (the
// own read transaction of the submit chain).
func (s *Postgres) ReadProposalAttempts(ctx context.Context, scope tenancy.Context, proposalKey string) ([]actionPort.ProposalAttemptRow, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	attempts, err := readAttemptsTx(ctx, tx, scope.TenantID(), proposalKey)
	if err != nil {
		return nil, err
	}
	return attempts, tx.Commit(ctx)
}

// ReadCaseAttempts reads every attempt row of one case ordered by time.
func (s *Postgres) ReadCaseAttempts(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]actionPort.ProposalAttemptRow, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		select proposal_key, attempt_no, preview_id, definition_id, definition_version, case_id, work_item_id,
		       target_kind, target_ref, verdict, blocked_reason, invalid_report, actor_id, created_at
		from action.action_proposals
		where tenant_id=$1 and case_id=$2 order by created_at, attempt_no`, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attempts := []actionPort.ProposalAttemptRow{}
	for rows.Next() {
		var row actionPort.ProposalAttemptRow
		if err := rows.Scan(&row.ProposalKey, &row.AttemptNo, &row.PreviewID, &row.DefinitionID, &row.DefinitionVersion,
			&row.CaseID, &row.WorkItemID, &row.TargetKind, &row.TargetRef,
			&row.Verdict, &row.BlockedReason, &row.InvalidReport, &row.ActorID, &row.CreatedAt); err != nil {
			return nil, err
		}
		row.CreatedAt = row.CreatedAt.UTC()
		attempts = append(attempts, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attempts, tx.Commit(ctx)
}

// readVersionRows reads the version rows of the given WHERE anchors — the
// shared scanner of the case read.
func readVersionRows(ctx context.Context, tx pgx.Tx, query string, args ...any) ([]actionDomain.ActionPreview, error) {
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := []actionDomain.ActionPreview{}
	for rows.Next() {
		var preview actionDomain.ActionPreview
		var typedInput, facts, observations, snapshot, effects []byte
		if err := rows.Scan(&preview.TenantID, &preview.PreviewID, &preview.VersionNo, &preview.DefinitionID, &preview.DefinitionVersion,
			&preview.CaseID, &preview.WorkItemID, &preview.Target.Kind, &preview.Target.Ref,
			&typedInput, &facts, &observations, &snapshot, &effects,
			&preview.Partition.Partition, &preview.Partition.RecoveryInstruction,
			&preview.Partition.HumanOwnerRequired, &preview.Partition.UnsafeClass,
			&preview.Partition.NoProvenReconciliationPath, &preview.HumanOnlyControlMark,
			&preview.OwnerEpoch, &preview.ControlGeneration, &preview.ProfileVersion, &preview.PolicyOutcomeNo,
			&preview.CapabilityDeclVersion, &preview.Digest, &preview.AuthorID, &preview.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(typedInput, &preview.TypedInput); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(facts, &preview.SupportingFacts); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(observations, &preview.SupportingObservations); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(snapshot, &preview.Snapshot); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(effects, &preview.Effects); err != nil {
			return nil, err
		}
		preview.CreatedAt = preview.CreatedAt.UTC()
		versions = append(versions, preview)
	}
	return versions, rows.Err()
}

// ReadPreviewVersions reads every version row of one case ordered by the
// preview and the monotonic number (the view input).
func (s *Postgres) ReadPreviewVersions(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]actionDomain.ActionPreview, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	versions, err := readVersionRows(ctx, tx, `
		select tenant_id, preview_id, version_no, definition_id, definition_version, case_id, work_item_id, target_kind, target_ref,
		       typed_input, supporting_facts, supporting_observations, snapshot, effects,
		       partition, recovery_instruction, human_owner_required, unsafe_class, no_reconciliation, human_only_control_mark,
		       owner_epoch, control_generation, profile_version, policy_outcome_no, capability_decl_version, digest, author_id, created_at
		from action.action_previews
		where tenant_id=$1 and case_id=$2 order by preview_id, version_no`, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return versions, tx.Commit(ctx)
}

// ReadPreviewEvents reads every event of one case ordered by the preview,
// the version scope and the monotonic number.
func (s *Postgres) ReadPreviewEvents(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]actionPort.PreviewEventRow, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		select preview_id, version_no, case_id, event_no, kind, reason, details, actor_id, created_at
		from action.action_preview_events
		where tenant_id=$1 and case_id=$2 order by preview_id, version_no, event_no`,
		scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []actionPort.PreviewEventRow{}
	for rows.Next() {
		var row actionPort.PreviewEventRow
		if err := rows.Scan(&row.PreviewID, &row.VersionNo, &row.CaseID, &row.EventNo, &row.Kind, &row.Reason, &row.Details, &row.ActorID, &row.CreatedAt); err != nil {
			return nil, err
		}
		row.CreatedAt = row.CreatedAt.UTC()
		events = append(events, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, tx.Commit(ctx)
}

// RecordProposalAttempt appends one attempt row and its stage event under
// the proposal advisory lock: the ladder is re-derived INSIDE the lock —
// the raced twin answers the honest stale refusal, the attempt number
// derived earlier by the app must still be the next one. The blocked and
// the invalid paths record the ladder WITHOUT building any version (the
// event reason carries the policy word of the block; the invalid report
// lives on the attempt row).
func (s *Postgres) RecordProposalAttempt(ctx context.Context, p actionPort.RecordAttemptParams) error {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", proposalLockKey(tenantID, p.Attempt.ProposalKey)); err != nil {
		return err
	}
	// The ladder re-derivation under the lock: the expected attempt must
	// be exactly the next one of the stored rows.
	attempts, err := readAttemptsTx(ctx, tx, tenantID, p.Attempt.ProposalKey)
	if err != nil {
		return err
	}
	maxAttempt := 0
	for _, stored := range attempts {
		if stored.AttemptNo > maxAttempt {
			maxAttempt = stored.AttemptNo
		}
	}
	if maxAttempt >= p.Attempt.AttemptNo {
		// A raced twin already moved the ladder (or the key exhausted its
		// two attempts): the honest refusal, nothing is written.
		if maxAttempt >= actionDomain.MaxAttemptsProposal {
			return actionPort.ErrAttemptExhausted
		}
		return actionPort.ErrStalePreviewVersion
	}
	if err := insertAttemptRow(ctx, tx, tenantID, p.Attempt, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return err
	}
	if err := insertEventRow(ctx, tx, tenantID, p.Attempt.PreviewID, 0, p.Attempt.CaseID,
		p.StageEventID, p.StageEventKind, p.StageEventReason, "", p.Attempt.ActorID,
		p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// BuildPreviewVersion builds one version in ONE transaction under the
// preview advisory lock: the version ladder and (when the build carries a
// submission) the attempt ladder are re-derived under the locks, the keyed
// digest is re-computed from the material with the per-tenant key (the
// integrity of the exact bytes the human will see), the attempt row and
// the stage event land with the version row and the previewed event — and
// the edit path marks the superseded version with its event in the SAME
// transaction (the atomic supersede: a failed build leaves the old
// version fully current, no partial state exists).
func (s *Postgres) BuildPreviewVersion(ctx context.Context, p actionPort.BuildPreviewVersionParams) error {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	proposalKey := ""
	if p.Attempt != nil {
		proposalKey = p.Attempt.ProposalKey
	}
	if err := lockPreviewThread(ctx, tx, tenantID, p.Preview.PreviewID, proposalKey); err != nil {
		return err
	}
	// The version ladder re-derivation under the lock: the expected
	// version must be exactly the next one of the stored rows.
	maxVersion, err := maxVersionOfTx(ctx, tx, tenantID, p.Preview.PreviewID)
	if err != nil {
		return err
	}
	if maxVersion+1 != p.Preview.VersionNo {
		return actionPort.ErrStalePreviewVersion
	}
	if p.SupersedesVersion > 0 && p.SupersedesVersion != maxVersion {
		// The edit expected the CURRENT version to supersede; the ladder
		// moved — the honest refusal BEFORE any write.
		return actionPort.ErrStalePreviewVersion
	}
	// The attempt ladder re-derivation for the building submission.
	stageKind := ""
	if p.Attempt != nil {
		attempts, err := readAttemptsTx(ctx, tx, tenantID, p.Attempt.ProposalKey)
		if err != nil {
			return err
		}
		maxAttempt := 0
		for _, stored := range attempts {
			if stored.AttemptNo > maxAttempt {
				maxAttempt = stored.AttemptNo
			}
		}
		if maxAttempt >= p.Attempt.AttemptNo {
			if maxAttempt >= actionDomain.MaxAttemptsProposal {
				return actionPort.ErrAttemptExhausted
			}
			return actionPort.ErrStalePreviewVersion
		}
		if p.Attempt.AttemptNo == 1 {
			stageKind = actionDomain.EventProposed
		} else {
			stageKind = actionDomain.EventRepaired
		}
	}
	// The keyed digest re-computation from the material under the locks
	// (the 3-3 token discipline: the stored digest is the identity of the
	// exact bytes the human will see — a mismatch is the loud integrity
	// alarm, never a masked silence).
	key, err := readPreviewKeyTx(ctx, tx, p.Scope, p.Now)
	if err != nil {
		return err
	}
	if digest := actionDomain.PreviewDigest(key, p.Preview.PreviewID, p.Preview.VersionNo, actionDomain.PreviewCanonicalBytes(p.Preview)); digest != p.Preview.Digest {
		return actionPort.ErrPreviewIntegrity
	}
	// The attempt row (the building submission) and its stage event at
	// the proposal scope.
	if p.Attempt != nil {
		if err := insertAttemptRow(ctx, tx, tenantID, *p.Attempt, p.Now.UTC().Truncate(time.Microsecond)); err != nil {
			return err
		}
		if err := insertEventRow(ctx, tx, tenantID, p.Attempt.PreviewID, 0, p.Attempt.CaseID,
			p.AttemptStageEventID, stageKind, p.Attempt.BlockedReason, "", p.Attempt.ActorID,
			p.Now.UTC().Truncate(time.Microsecond)); err != nil {
			return err
		}
	}
	// The version row.
	if err := insertVersionRow(ctx, tx, tenantID, p); err != nil {
		return err
	}
	// The superseded event of the OLD version (the edit path) — the same
	// transaction, the atomic supersede.
	if p.SupersedesVersion > 0 {
		if err := insertEventRow(ctx, tx, tenantID, p.Preview.PreviewID, p.SupersedesVersion, p.Preview.CaseID,
			p.SupersededEventID, actionDomain.EventSuperseded, "", "", p.Preview.AuthorID,
			p.Now.UTC().Truncate(time.Microsecond)); err != nil {
			return err
		}
	}
	// The previewed event of the new version.
	if err := insertEventRow(ctx, tx, tenantID, p.Preview.PreviewID, p.Preview.VersionNo, p.Preview.CaseID,
		p.PreviewedEventID, actionDomain.EventPreviewed, "", "", p.Preview.AuthorID,
		p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RecordDryRunEvent appends the dry_run_recorded evidence event of one
// exact version under the preview advisory lock: the version must exist
// (the honest 404), the event number re-derives under the lock. The kind
// is simply not in DerivePreviewState — the Timeline words never move
// (AC-8, the structural honesty of the step-01 domain).
func (s *Postgres) RecordDryRunEvent(ctx context.Context, p actionPort.RecordDryRunParams) error {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if err := lockPreviewThread(ctx, tx, tenantID, p.PreviewID, ""); err != nil {
		return err
	}
	var found int
	err = tx.QueryRow(ctx, `
		select 1 from action.action_previews
		where tenant_id=$1 and preview_id=$2 and version_no=$3`,
		tenantID, p.PreviewID, p.VersionNo).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return actionDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := insertEventRow(ctx, tx, tenantID, p.PreviewID, p.VersionNo, p.CaseID,
		p.EventID, actionDomain.EventDryRunRecorded, p.Result, p.Details, p.ActorID,
		p.Now.UTC().Truncate(time.Microsecond)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
