package store

import (
	"context"
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
// Story 2.2 reads (the Work Control Card material)
// ---------------------------------------------------------------------------

// LookupCommand answers the idempotency question of one work command key
// BEFORE the domain guards: the stored request hash with the stored
// outcome JSON (the connector 1-9 precedent, actor-scoped by the RLS
// policy).
func (s *Postgres) LookupCommand(ctx context.Context, scope tenancy.Context, command, key string) (casesPort.CommandLookup, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesPort.CommandLookup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var lookup casesPort.CommandLookup
	err = tx.QueryRow(ctx, `
                select request_hash, outcome from cases.command_records
                where tenant_id=$1 and actor_id=$2 and command=$3 and idempotency_key=$4`,
		scope.TenantID(), scope.PrincipalID(), command, key).Scan(&lookup.RequestHash, &lookup.Outcome)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesPort.CommandLookup{}, tx.Commit(ctx)
	}
	if err != nil {
		return casesPort.CommandLookup{}, err
	}
	lookup.Found = true
	return lookup, tx.Commit(ctx)
}

// ReadCasePin reads one case row: the pack pin and the existence.
func (s *Postgres) ReadCasePin(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesDomain.Case, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesDomain.Case{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	out, err := readCasePinTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return casesDomain.Case{}, err
	}
	return out, tx.Commit(ctx)
}

func readCasePinTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) (casesDomain.Case, error) {
	var row casesDomain.Case
	var packID, packVersion *string
	err := tx.QueryRow(ctx, `
                select case_id, case_no, employee_id, state, pack_id, pack_version, created_at
                from cases.cases where tenant_id=$1 and case_id=$2`, tenantID, caseID).
		Scan(&row.CaseID, &row.CaseNo, &row.EmployeeID, &row.State, &packID, &packVersion, &row.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.Case{}, casesDomain.ErrNotFound
	}
	if err != nil {
		return casesDomain.Case{}, err
	}
	row.TenantID = tenantID
	if packID != nil {
		row.PackID, row.PackVersion = *packID, *packVersion
	}
	row.CreatedAt = row.CreatedAt.UTC()
	return row, nil
}

// The derived current view of one work item: the maximal version with the
// material it set (the type/title travel with the create/classify rows).
const currentViewColumns = `
        w.work_item_id, w.case_id, w.conversation_id, w.created_at,
        v.version_no, v.kind, v.state, v.reason, v.actor_id, v.ref,
        coalesce(t.type, ''), coalesce(t.title, '')`

const currentViewJoin = `
        from cases.work_items w
        left join lateral (
                select version_no, kind, state, reason, actor_id, ref from cases.work_item_versions
                where tenant_id = w.tenant_id and work_item_id = w.work_item_id
                order by version_no desc limit 1
        ) v on true
        left join lateral (
                select type, title from cases.work_item_versions
                where tenant_id = w.tenant_id and work_item_id = w.work_item_id and type is not null
                order by version_no desc limit 1
        ) t on true`

func scanCurrent(scan func(dest ...any) error) (casesDomain.WorkItemCurrent, error) {
	var current casesDomain.WorkItemCurrent
	var versionNo *int64
	var kind, state *string
	var reason *string
	var actorID, ref *id.UUID
	var created time.Time
	err := scan(&current.WorkItemID, &current.CaseID, &current.ConversationID, &created,
		&versionNo, &kind, &state, &reason, &actorID, &ref, &current.Type, &current.Title)
	if err != nil {
		return casesDomain.WorkItemCurrent{}, err
	}
	current.CreatedAt = created.UTC()
	if versionNo != nil {
		current.Version, current.Kind, current.State = *versionNo, *kind, *state
		current.Terminal = casesDomain.TerminalWorkItemStates[*state]
	}
	if reason != nil {
		current.Reason = *reason
	}
	if actorID != nil {
		current.ActorID = *actorID
	}
	if ref != nil {
		current.Ref = *ref
	}
	return current, nil
}

// ReadWorkItemCurrents resolves the derived current views of the given
// work items (missing anchors are absent from the answer).
func (s *Postgres) ReadWorkItemCurrents(ctx context.Context, scope tenancy.Context, caseID id.UUID, ids []id.UUID) (map[id.UUID]casesDomain.WorkItemCurrent, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	currents, err := readWorkItemCurrentsTx(ctx, tx, scope.TenantID(), caseID, ids)
	if err != nil {
		return nil, err
	}
	return currents, tx.Commit(ctx)
}

func readWorkItemCurrentsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID, ids []id.UUID) (map[id.UUID]casesDomain.WorkItemCurrent, error) {
	out := make(map[id.UUID]casesDomain.WorkItemCurrent, len(ids))
	for _, workItemID := range ids {
		current, err := scanCurrent(tx.QueryRow(ctx,
			"select "+currentViewColumns+" "+currentViewJoin+" where w.tenant_id=$1 and w.case_id=$2 and w.work_item_id=$3",
			tenantID, caseID, workItemID).Scan)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[workItemID] = current
	}
	return out, nil
}

// ReadEventOfCase reads one event of the case (the link and the evidence
// validation input).
func (s *Postgres) ReadEventOfCase(ctx context.Context, scope tenancy.Context, caseID, eventID id.UUID) (casesDomain.ChannelEvent, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesDomain.ChannelEvent{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	event, err := scanEvent(tx.QueryRow(ctx,
		"select "+eventColumns+" from cases.channel_events where tenant_id=$1 and case_id=$2 and event_id=$3",
		scope.TenantID(), caseID, eventID))
	if err != nil {
		return casesDomain.ChannelEvent{}, err
	}
	return event, tx.Commit(ctx)
}

// ReadFactMaterial reads one fact with its versions and confirmations (an
// absent anchor returns the empty material).
func (s *Postgres) ReadFactMaterial(ctx context.Context, scope tenancy.Context, caseID, factID id.UUID) (casesPort.FactMaterial, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesPort.FactMaterial{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	material, err := readFactMaterialTx(ctx, tx, scope.TenantID(), caseID, factID)
	if err != nil {
		return casesPort.FactMaterial{}, err
	}
	return material, tx.Commit(ctx)
}

// ReadFactMaterialByKey resolves the fact anchor of one material key of the
// case (an absent anchor returns the empty material).
func (s *Postgres) ReadFactMaterialByKey(ctx context.Context, scope tenancy.Context, caseID id.UUID, materialKey string) (casesPort.FactMaterial, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesPort.FactMaterial{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var factID id.UUID
	err = tx.QueryRow(ctx, `
                select fact_id from cases.facts where tenant_id=$1 and case_id=$2 and material_key=$3`,
		scope.TenantID(), caseID, materialKey).Scan(&factID)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesPort.FactMaterial{}, tx.Commit(ctx)
	}
	if err != nil {
		return casesPort.FactMaterial{}, err
	}
	material, err := readFactMaterialTx(ctx, tx, scope.TenantID(), caseID, factID)
	if err != nil {
		return casesPort.FactMaterial{}, err
	}
	return material, tx.Commit(ctx)
}

// ReadFactsOfCase reads every fact of the case with its versions and
// confirmations (the Work Control Card read).
func (s *Postgres) ReadFactsOfCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesPort.FactMaterial, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
                select fact_id, material_key, created_at from cases.facts
                where tenant_id=$1 and case_id=$2 order by material_key`, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	materials := []casesPort.FactMaterial{}
	// Read all anchor rows first (the fact versions query below runs on the
	// same connection after this result set closes).
	anchors := []casesPort.FactMaterial{}
	for rows.Next() {
		var material casesPort.FactMaterial
		if err = rows.Scan(&material.Anchor.FactID, &material.Anchor.MaterialKey, &material.Anchor.CreatedAt); err != nil {
			return nil, err
		}
		material.Anchor.TenantID, material.Anchor.CaseID = scope.TenantID(), caseID
		material.Anchor.CreatedAt = material.Anchor.CreatedAt.UTC()
		anchors = append(anchors, material)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for _, anchor := range anchors {
		material, err := readFactMaterialTx(ctx, tx, scope.TenantID(), caseID, anchor.Anchor.FactID)
		if err != nil {
			return nil, err
		}
		materials = append(materials, material)
	}
	return materials, tx.Commit(ctx)
}

func readFactMaterialTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, factID id.UUID) (casesPort.FactMaterial, error) {
	material := casesPort.FactMaterial{}
	err := tx.QueryRow(ctx, `
                select fact_id, material_key, created_at from cases.facts
                where tenant_id=$1 and case_id=$2 and fact_id=$3`, tenantID, caseID, factID).
		Scan(&material.Anchor.FactID, &material.Anchor.MaterialKey, &material.Anchor.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesPort.FactMaterial{}, nil
	}
	if err != nil {
		return casesPort.FactMaterial{}, err
	}
	material.Anchor.TenantID, material.Anchor.CaseID = tenantID, caseID
	material.Anchor.CreatedAt = material.Anchor.CreatedAt.UTC()

	versionRows, err := tx.Query(ctx, `
                select fact_id, work_item_id, version_no, material_key, value, value_digest, source_type, source_id, channel, actor_id, observed_at, received_at, created_at
                from cases.fact_versions where tenant_id=$1 and fact_id=$2 order by version_no`, tenantID, factID)
	if err != nil {
		return casesPort.FactMaterial{}, err
	}
	defer versionRows.Close()
	material.Versions = []casesDomain.FactVersion{}
	for versionRows.Next() {
		var version casesDomain.FactVersion
		if err = versionRows.Scan(&version.FactID, &version.WorkItemID, &version.VersionNo, &version.MaterialKey,
			&version.Value, &version.ValueDigest, &version.SourceType, &version.SourceID, &version.Channel,
			&version.ActorID, &version.ObservedAt, &version.ReceivedAt, &version.CreatedAt); err != nil {
			return casesPort.FactMaterial{}, err
		}
		version.TenantID, version.CaseID = tenantID, caseID
		version.ObservedAt = version.ObservedAt.UTC()
		version.ReceivedAt = version.ReceivedAt.UTC()
		version.CreatedAt = version.CreatedAt.UTC()
		material.Versions = append(material.Versions, version)
	}
	if err = versionRows.Err(); err != nil {
		return casesPort.FactMaterial{}, err
	}

	confirmationRows, err := tx.Query(ctx, `
                select confirmation_id, version_no, confirmer_type, coalesce(evidence_event_id, '00000000-0000-0000-0000-000000000000'), coalesce(note, ''), actor_id, created_at
                from cases.fact_confirmations where tenant_id=$1 and fact_id=$2 order by created_at, confirmation_id`, tenantID, factID)
	if err != nil {
		return casesPort.FactMaterial{}, err
	}
	defer confirmationRows.Close()
	material.Confirmations = []casesDomain.FactConfirmation{}
	for confirmationRows.Next() {
		var confirmation casesDomain.FactConfirmation
		if err = confirmationRows.Scan(&confirmation.ConfirmationID, &confirmation.VersionNo, &confirmation.ConfirmerType,
			&confirmation.EvidenceEventID, &confirmation.Note, &confirmation.ActorID, &confirmation.CreatedAt); err != nil {
			return casesPort.FactMaterial{}, err
		}
		confirmation.TenantID, confirmation.CaseID, confirmation.FactID = tenantID, caseID, factID
		confirmation.CreatedAt = confirmation.CreatedAt.UTC()
		material.Confirmations = append(material.Confirmations, confirmation)
	}
	if err = confirmationRows.Err(); err != nil {
		return casesPort.FactMaterial{}, err
	}
	return material, nil
}

// LookupFactVersionBySource resolves the material idempotency tuple of a
// fact version (the source wrote the same material value already).
func (s *Postgres) LookupFactVersionBySource(ctx context.Context, scope tenancy.Context, sourceType, sourceID, materialKey, valueDigest string) (casesDomain.FactVersion, bool, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesDomain.FactVersion{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var version casesDomain.FactVersion
	err = tx.QueryRow(ctx, `
                select fact_id, case_id, work_item_id, version_no, material_key, value, value_digest, source_type, source_id, channel, actor_id, observed_at, received_at, created_at
                from cases.fact_versions
                where tenant_id=$1 and source_type=$2 and source_id=$3 and material_key=$4 and value_digest=$5`,
		scope.TenantID(), sourceType, sourceID, materialKey, valueDigest).
		Scan(&version.FactID, &version.CaseID, &version.WorkItemID, &version.VersionNo, &version.MaterialKey,
			&version.Value, &version.ValueDigest, &version.SourceType, &version.SourceID, &version.Channel,
			&version.ActorID, &version.ObservedAt, &version.ReceivedAt, &version.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.FactVersion{}, false, tx.Commit(ctx)
	}
	if err != nil {
		return casesDomain.FactVersion{}, false, err
	}
	version.TenantID = scope.TenantID()
	version.ObservedAt, version.ReceivedAt, version.CreatedAt = version.ObservedAt.UTC(), version.ReceivedAt.UTC(), version.CreatedAt.UTC()
	return version, true, tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// The transactional work command executor
// ---------------------------------------------------------------------------

// workLockKey builds the advisory-lock key of one work item anchor.
func workLockKey(tenantID id.UUID, workItemID id.UUID) string {
	return "cases.wi.v1:" + tenantID.String() + ":" + workItemID.String()
}

// factKeyLockKey builds the advisory-lock key of one fact identity (the
// case + material key — the find-or-create of the anchor and the version
// ladder serialize on it).
func factKeyLockKey(tenantID, caseID id.UUID, materialKey string) string {
	return "cases.fact.v1:" + tenantID.String() + ":" + caseID.String() + ":" + materialKey
}

func caseLockKey(tenantID, caseID id.UUID) string {
	return "case:" + tenantID.String() + ":" + caseID.String()
}

func conversationLockKey(tenantID, conversationID id.UUID) string {
	return "conversation:" + tenantID.String() + ":" + conversationID.String()
}

func bindingLockKey(tenantID, bindingID id.UUID) string {
	return "binding:" + tenantID.String() + ":" + bindingID.String()
}

func candidateLockKey(tenantID, candidateID id.UUID) string {
	return "candidate:" + tenantID.String() + ":" + candidateID.String()
}

func tokenLockKey(tenantID id.UUID, token string) string {
	return "token:" + tenantID.String() + ":" + token
}

func factIDLockKey(tenantID, factID id.UUID) string {
	return "cases.fact.v1:" + tenantID.String() + ":" + factID.String()
}

// ExecuteWorkCommand runs one transactional work command: the advisory
// locks of every involved anchor in the sorted order (the deadlock-free
// global order), the FOR UPDATE re-validation of the expected versions and
// the append-only inserts. The outcome JSON is the stored command record
// material — the app computed it from the same expectations the store
// re-validates.
func (s *Postgres) ExecuteWorkCommand(ctx context.Context, p casesPort.WorkCommandParams) error {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := p.Now.UTC().Truncate(time.Microsecond)
	tenantID := p.Scope.TenantID()
	// The lock plan: every anchor of the command, in the sorted order.
	// The fact anchors are locked by BOTH keys — the material key of the
	// case (the find-or-create identity) and the fact id (the version
	// ladder): the record path enters by the material key, the confirm
	// and rescope paths by the fact id, and the merge rescopes the planned
	// fact set — one unified key pair per fact serializes every writer of
	// the ladder (the record scope carries the work lock of its work
	// item, so a merge holding the source work locks fences every fact
	// that could enter its rescope set).
	lockKeys := []string{}
	switch p.Command {
	case "create_work_item":
		lockKeys = append(lockKeys, workLockKey(tenantID, p.WorkItemID))
	case "classify_work_item", "transition_work_item", "split_work_item", "link_event", "dispute_work_item":
		lockKeys = append(lockKeys, workLockKey(tenantID, p.WorkItemID))
	case "merge_work_items":
		lockKeys = append(lockKeys, workLockKey(tenantID, p.WorkItemID))
		for _, source := range p.Sources {
			lockKeys = append(lockKeys, workLockKey(tenantID, source.WorkItemID))
		}
		for _, factID := range p.FactLocks {
			lockKeys = append(lockKeys, factIDLockKey(tenantID, factID))
		}
	case "record_fact":
		lockKeys = append(lockKeys, workLockKey(tenantID, p.WorkItemID))
		lockKeys = append(lockKeys, factKeyLockKey(tenantID, p.CaseID, p.MaterialKey))
		if p.FactID.Valid() {
			lockKeys = append(lockKeys, factIDLockKey(tenantID, p.FactID))
		}
		if p.IdentifierToken != "" {
			// The identifier token lock: the match query and the candidate
			// inserts of the tokenization serialize per token — the match
			// set is the lock-visible truth (a concurrent writer of the
			// same token in another case either committed before this
			// query or creates the reverse candidate after it).
			lockKeys = append(lockKeys, tokenLockKey(tenantID, p.IdentifierToken))
		}
	case "start_verification":
		lockKeys = append(lockKeys, caseLockKey(tenantID, p.CaseID))
	case "bind_channel":
		lockKeys = append(lockKeys, caseLockKey(tenantID, p.CaseID), caseLockKey(tenantID, p.BoundCaseID),
			conversationLockKey(tenantID, p.ConversationID), conversationLockKey(tenantID, p.BoundConversationID))
	case "unlink_channel":
		lockKeys = append(lockKeys, bindingLockKey(tenantID, p.BindingID), caseLockKey(tenantID, p.CaseID))
	case "resolve_duplicate":
		lockKeys = append(lockKeys, candidateLockKey(tenantID, p.CandidateID), caseLockKey(tenantID, p.CaseID),
			conversationLockKey(tenantID, p.ConversationID), conversationLockKey(tenantID, p.BoundConversationID))
	case "confirm_fact_version", "rescope_fact":
		lockKeys = append(lockKeys, factIDLockKey(tenantID, p.FactID))
		if p.MaterialKey != "" {
			lockKeys = append(lockKeys, factKeyLockKey(tenantID, p.CaseID, p.MaterialKey))
		}
		if p.Command == "rescope_fact" {
			lockKeys = append(lockKeys, workLockKey(tenantID, p.TargetWorkItem))
		}
	case "take_ownership", "request_transfer", "resolve_transfer", "reassign_ownership":
		lockKeys = append(lockKeys, caseLockKey(tenantID, p.CaseID))
	case "send_client_reply":
		lockKeys = append(lockKeys, caseLockKey(tenantID, p.CaseID), conversationLockKey(tenantID, p.ConversationID))
	case "propose_mode", "exit_mode", "record_disclosure", "record_disclosure_event":
		lockKeys = append(lockKeys, caseLockKey(tenantID, p.CaseID), conversationLockKey(tenantID, p.ConversationID))
	case "submit_ai_proposal", "evaluate_policy":
		lockKeys = append(lockKeys, caseLockKey(tenantID, p.CaseID), conversationLockKey(tenantID, p.ConversationID))
	case "read_owning_system":
		lockKeys = append(lockKeys, caseLockKey(tenantID, p.CaseID), workLockKey(tenantID, p.WorkItemID))
	case "record_objection":
		lockKeys = append(lockKeys, caseLockKey(tenantID, p.CaseID), workLockKey(tenantID, p.WorkItemID))
	case "create_commitment", "record_commitment_decline":
		lockKeys = append(lockKeys, caseLockKey(tenantID, p.CaseID), workLockKey(tenantID, p.WorkItemID))
	case "issue_continuation_link", "revoke_continuation_link":
		// The continuation commands: the case key serializes the owner
		// epoch read of the issuance pin (the ownership writers hold the
		// same key), the work key fences the terminal-state boundary; the
		// link itself needs no lock here — the command-record idempotency
		// and the append-only UNIQUE backstops keep the raced paths honest.
		lockKeys = append(lockKeys, caseLockKey(tenantID, p.CaseID), workLockKey(tenantID, p.WorkItemID))
	case "confirm_commitment":
		// The price fact key joins the plan: the confirm re-derives the
		// conflict badge against the confirmed exact price fact under the
		// locks, so the fact writers (record_fact / confirm_fact_version of
		// the price key) serialize with the confirmation — the badge is the
		// commit-time truth, never a raced snapshot.
		lockKeys = append(lockKeys, caseLockKey(tenantID, p.CaseID), workLockKey(tenantID, p.WorkItemID),
			factKeyLockKey(tenantID, p.CaseID, casesDomain.MaterialFieldPrice))
	}
	sort.Strings(lockKeys)
	for _, key := range lockKeys {
		if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", key); err != nil {
			return err
		}
	}
	switch p.Command {
	case "create_work_item":
		err = s.execCreateWorkItem(ctx, tx, tenantID, p, now)
	case "classify_work_item":
		err = s.execClassify(ctx, tx, tenantID, p, now)
	case "transition_work_item":
		err = s.execTransition(ctx, tx, tenantID, p, now)
	case "split_work_item":
		err = s.execSplit(ctx, tx, tenantID, p, now)
	case "merge_work_items":
		err = s.execMerge(ctx, tx, tenantID, p, now)
	case "link_event":
		err = s.execLinkEvent(ctx, tx, tenantID, p, now)
	case "dispute_work_item":
		err = s.execDispute(ctx, tx, tenantID, p, now)
	case "record_fact":
		err = s.execRecordFact(ctx, tx, tenantID, p, now)
	case "confirm_fact_version":
		err = s.execConfirmFact(ctx, tx, tenantID, p, now)
	case "rescope_fact":
		err = s.execRescopeFact(ctx, tx, tenantID, p, now)
	case "start_verification":
		err = s.execStartVerification(ctx, tx, tenantID, p, now)
	case "bind_channel":
		err = s.execBindChannel(ctx, tx, tenantID, p, now)
	case "unlink_channel":
		err = s.execUnlinkChannel(ctx, tx, tenantID, p, now)
	case "resolve_duplicate":
		err = s.execResolveDuplicate(ctx, tx, tenantID, p, now)
	case "take_ownership":
		err = s.execTakeOwnership(ctx, tx, tenantID, p, now)
	case "request_transfer":
		err = s.execRequestTransfer(ctx, tx, tenantID, p, now)
	case "resolve_transfer":
		err = s.execResolveTransfer(ctx, tx, tenantID, p, now)
	case "reassign_ownership":
		err = s.execReassignOwnership(ctx, tx, tenantID, p, now)
	case "send_client_reply":
		err = s.execSendClientReply(ctx, tx, tenantID, p, now)
	case "propose_mode":
		err = s.execProposeMode(ctx, tx, tenantID, p, now)
	case "exit_mode":
		err = s.execExitMode(ctx, tx, tenantID, p, now)
	case "record_disclosure":
		err = s.execRecordDisclosure(ctx, tx, tenantID, p, now)
	case "record_disclosure_event":
		err = s.execRecordDisclosureEvent(ctx, tx, tenantID, p, now)
	case "submit_ai_proposal":
		err = s.execSubmitAIProposal(ctx, tx, tenantID, p, now)
	case "evaluate_policy":
		err = s.execEvaluatePolicy(ctx, tx, tenantID, p, now)
	case "read_owning_system":
		err = s.execReadOwningSystem(ctx, tx, tenantID, p, now)
	case "record_objection":
		err = s.execRecordObjection(ctx, tx, tenantID, p, now)
	case "create_commitment":
		err = s.execCreateCommitment(ctx, tx, tenantID, p, now)
	case "confirm_commitment":
		err = s.execConfirmCommitment(ctx, tx, tenantID, p, now)
	case "record_commitment_decline":
		err = s.execRecordCommitmentDecline(ctx, tx, tenantID, p, now)
	case "issue_continuation_link":
		err = s.execIssueContinuationLink(ctx, tx, tenantID, p, now)
	case "revoke_continuation_link":
		err = s.execRevokeContinuationLink(ctx, tx, tenantID, p, now)
	default:
		err = fmt.Errorf("неизвестная команда %s", p.Command)
	}
	if err != nil {
		return err
	}
	// The command record: the idempotent replay answer of the exact repeat.
	if _, err = tx.Exec(ctx, `
                insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                values ($1,$2,$3,$4,$5,$6,$7)`,
		tenantID, p.Actor, p.Command, p.Key, p.RequestHash, p.Outcome, now); err != nil {
		return mapWorkInsertError(p.Command, err)
	}
	return tx.Commit(ctx)
}

// currentForUpdate re-validates the expected version of the work item
// under the advisory lock (the derived current view). The anchor lock is
// the ADVISORY lock of ExecuteWorkCommand taken in the sorted order: a row
// FOR UPDATE requires the UPDATE privilege the append-only grants
// deliberately do not carry — the advisory lock serializes every command
// path with the identical material safety (the AcceptEvent precedent).
// The expectation -1 means "existence only" (the scope sanity of the fact
// commands).
func currentForUpdate(ctx context.Context, tx pgx.Tx, tenantID, caseID, workItemID id.UUID, expected int64) (casesDomain.WorkItemCurrent, error) {
	var conversationID id.UUID
	var created time.Time
	err := tx.QueryRow(ctx, `
                select conversation_id, created_at from cases.work_items
                where tenant_id=$1 and case_id=$2 and work_item_id=$3`,
		tenantID, caseID, workItemID).Scan(&conversationID, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.WorkItemCurrent{}, casesDomain.ErrNotFound
	}
	if err != nil {
		return casesDomain.WorkItemCurrent{}, err
	}
	current, err := scanCurrent(tx.QueryRow(ctx,
		"select "+currentViewColumns+" "+currentViewJoin+" where w.tenant_id=$1 and w.case_id=$2 and w.work_item_id=$3",
		tenantID, caseID, workItemID).Scan)
	if err != nil {
		return casesDomain.WorkItemCurrent{}, err
	}
	if expected >= 0 && current.Version != expected {
		return current, casesDomain.ErrStaleVersion
	}
	return current, nil
}

func insertWorkItemVersion(ctx context.Context, tx pgx.Tx, tenantID, caseID, conversationID, workItemID id.UUID, versionNo int64, kind, workType, title, state, reason string, actorID, ref id.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, `
                insert into cases.work_item_versions (tenant_id, case_id, conversation_id, work_item_id, version_no, kind, type, title, state, reason, actor_id, ref, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		tenantID, caseID, conversationID, workItemID, versionNo, kind,
		nilText(workType), nilText(title), state, nilText(reason), nilID(actorID), nilID(ref), now)
	return err
}

func insertWorkItemAnchor(ctx context.Context, tx pgx.Tx, tenantID, caseID, conversationID, workItemID id.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, `
                insert into cases.work_items (tenant_id, case_id, conversation_id, work_item_id, created_at)
                values ($1,$2,$3,$4,$5)`, tenantID, caseID, conversationID, workItemID, now)
	return err
}

// execCreateWorkItem creates the anchor with the version 1 (the typed and
// titled birth of a new intention inside an existing conversation).
func (s *Postgres) execCreateWorkItem(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	// The containment: the conversation must belong to the case.
	var conversationCase id.UUID
	err := tx.QueryRow(ctx, `
                select case_id from cases.conversations where tenant_id=$1 and conversation_id=$2 and case_id=$3`,
		tenantID, p.ConversationID, p.CaseID).Scan(&conversationCase)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := insertWorkItemAnchor(ctx, tx, tenantID, p.CaseID, p.ConversationID, p.WorkItemID, now); err != nil {
		return mapWorkInsertError(p.Command, err)
	}
	return insertWorkItemVersion(ctx, tx, tenantID, p.CaseID, p.ConversationID, p.WorkItemID, 1,
		casesDomain.VersionKindCreate, p.Type, p.Title, casesDomain.WorkItemStateInWork, p.Reason, p.Actor, id.UUID(""), now)
}

// execClassify writes the classification version (the state leaves
// untriaged here; a re-classification keeps the active state).
func (s *Postgres) execClassify(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	current, err := currentForUpdate(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.ExpectedVersion)
	if err != nil {
		return err
	}
	return insertWorkItemVersion(ctx, tx, tenantID, p.CaseID, current.ConversationID, p.WorkItemID, p.ExpectedVersion+1,
		casesDomain.VersionKindClassify, p.Type, p.Title, p.ToState, p.Reason, p.Actor, id.UUID(""), now)
}

// execTransition writes the state transition version.
func (s *Postgres) execTransition(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	current, err := currentForUpdate(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.ExpectedVersion)
	if err != nil {
		return err
	}
	return insertWorkItemVersion(ctx, tx, tenantID, p.CaseID, current.ConversationID, p.WorkItemID, p.ExpectedVersion+1,
		casesDomain.VersionKindTransition, "", "", p.ToState, p.Reason, p.Actor, id.UUID(""), now)
}

// execSplit writes the terminal split version of the source and creates
// the typed and titled targets with the backward reference. The target
// anchors are created BEFORE the version rows (the ref foreign keys of
// both directions must resolve inside the transaction).
func (s *Postgres) execSplit(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	current, err := currentForUpdate(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.ExpectedVersion)
	if err != nil {
		return err
	}
	firstTarget := id.UUID("")
	if len(p.Targets) > 0 {
		firstTarget = p.Targets[0].WorkItemID
	}
	for _, target := range p.Targets {
		if err := insertWorkItemAnchor(ctx, tx, tenantID, p.CaseID, current.ConversationID, target.WorkItemID, now); err != nil {
			return mapWorkInsertError(p.Command, err)
		}
	}
	if err := insertWorkItemVersion(ctx, tx, tenantID, p.CaseID, current.ConversationID, p.WorkItemID, p.ExpectedVersion+1,
		casesDomain.VersionKindSplit, "", "", casesDomain.WorkItemStateSplit, p.Reason, p.Actor, firstTarget, now); err != nil {
		return err
	}
	for _, target := range p.Targets {
		if err := insertWorkItemVersion(ctx, tx, tenantID, p.CaseID, current.ConversationID, target.WorkItemID, 1,
			casesDomain.VersionKindCreate, target.Type, target.Title, casesDomain.WorkItemStateInWork, p.Reason, p.Actor, p.WorkItemID, now); err != nil {
			return err
		}
	}
	return nil
}

// execMerge writes the terminal merged versions of the sources (with
// merged_into), rescopes the facts of the sources onto the target by NEW
// versions (no fact rows are duplicated) and appends the missing event
// links onto the target.
func (s *Postgres) execMerge(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	target, err := currentForUpdate(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.ExpectedVersion)
	if err != nil {
		return err
	}
	for _, source := range p.Sources {
		current, err := currentForUpdate(ctx, tx, tenantID, p.CaseID, source.WorkItemID, source.ExpectedVersion)
		if err != nil {
			return err
		}
		if err := insertWorkItemVersion(ctx, tx, tenantID, p.CaseID, current.ConversationID, source.WorkItemID, source.ExpectedVersion+1,
			casesDomain.VersionKindMerge, "", "", casesDomain.WorkItemStateMerged, p.Reason, p.Actor, p.WorkItemID, now); err != nil {
			return err
		}
	}
	// The fact rescope: every fact whose CURRENT scope is a source gets a
	// new version scoped to the target (the anchor and the rows stay).
	sourceSet := map[id.UUID]bool{}
	for _, source := range p.Sources {
		sourceSet[source.WorkItemID] = true
	}
	factRows, err := tx.Query(ctx, `
                select distinct f.fact_id from cases.facts f
                join lateral (
                        select work_item_id from cases.fact_versions
                        where tenant_id=f.tenant_id and fact_id=f.fact_id
                        order by version_no desc limit 1
                ) latest on true
                where f.tenant_id=$1 and f.case_id=$2`, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	var factIDs []id.UUID
	for factRows.Next() {
		var factID id.UUID
		if err = factRows.Scan(&factID); err != nil {
			factRows.Close()
			return err
		}
		factIDs = append(factIDs, factID)
	}
	factRows.Close()
	if err = factRows.Err(); err != nil {
		return err
	}
	for _, factID := range factIDs {
		material, err := readFactMaterialTx(ctx, tx, tenantID, p.CaseID, factID)
		if err != nil {
			return err
		}
		if len(material.Versions) == 0 {
			continue
		}
		latest := material.Versions[len(material.Versions)-1]
		if !sourceSet[latest.WorkItemID] {
			continue
		}
		if err := insertFactVersion(ctx, tx, tenantID, p.CaseID, factID, target.WorkItemID, latest.VersionNo+1,
			material.Anchor.MaterialKey, latest.Value, latest.ValueDigest,
			casesDomain.FactSourceOperator, p.Key, "operator", p.Actor, now, now, now); err != nil {
			return mapWorkInsertError(p.Command, err)
		}
	}
	// The event links: the events of the sources are appended onto the
	// target (the existing links stay in place; the order follows
	// received_at of the events on read).
	for _, source := range p.Sources {
		linkRows, err := tx.Query(ctx, `
                        select l.event_id from cases.work_item_event_links l
                        where l.tenant_id=$1 and l.work_item_id=$2`, tenantID, source.WorkItemID)
		if err != nil {
			return err
		}
		var eventIDs []id.UUID
		for linkRows.Next() {
			var eventID id.UUID
			if err = linkRows.Scan(&eventID); err != nil {
				linkRows.Close()
				return err
			}
			eventIDs = append(eventIDs, eventID)
		}
		linkRows.Close()
		if err = linkRows.Err(); err != nil {
			return err
		}
		for _, eventID := range eventIDs {
			// The explicit missing-link check (no ON CONFLICT shortcuts: the
			// insert-only surface stays honest and plain).
			var exists bool
			if err = tx.QueryRow(ctx, `
                                select exists (select 1 from cases.work_item_event_links
                                where tenant_id=$1 and work_item_id=$2 and event_id=$3)`,
				tenantID, p.WorkItemID, eventID).Scan(&exists); err != nil {
				return err
			}
			if exists {
				continue
			}
			if _, err = tx.Exec(ctx, `
                                insert into cases.work_item_event_links (tenant_id, work_item_id, event_id, actor_id, created_at)
                                values ($1,$2,$3,$4,$5)`, tenantID, p.WorkItemID, eventID, p.Actor, now); err != nil {
				return err
			}
		}
	}
	return nil
}

// execLinkEvent binds one accepted event of the case to the work item.
func (s *Postgres) execLinkEvent(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if _, err := currentForUpdate(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.ExpectedVersion); err != nil {
		return err
	}
	event, err := scanEvent(tx.QueryRow(ctx,
		"select "+eventColumns+" from cases.channel_events where tenant_id=$1 and case_id=$2 and event_id=$3 and state='accepted'",
		tenantID, p.CaseID, p.EventID))
	if errors.Is(err, casesDomain.ErrNotFound) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.work_item_event_links (tenant_id, work_item_id, event_id, actor_id, created_at)
                values ($1,$2,$3,$4,$5)`, tenantID, p.WorkItemID, event.EventID, p.Actor, now); err != nil {
		return mapWorkInsertError(p.Command, err)
	}
	return nil
}

// execDispute writes the append-only operational exception (the human
// queue of the contour); the material is never rewritten by the dispute.
func (s *Postgres) execDispute(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	current, err := currentForUpdate(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.ExpectedVersion)
	if err != nil {
		return err
	}
	summary := "Спор рабочего элемента (состояние " + current.State + ", версия " + itoa64(p.ExpectedVersion) + "): решение за человеком."
	if p.Note != "" {
		summary = "Спор рабочего элемента (состояние " + current.State + ", версия " + itoa64(p.ExpectedVersion) + "): " + p.Note
	}
	_, err = tx.Exec(ctx, `
                insert into cases.operational_exceptions (tenant_id, exception_id, kind, summary, correlation_id, event_id, original_event_id, case_id, conversation_id, created_at)
                values ($1,$2,$3,$4,$5,NULL,NULL,$6,$7,$8)`,
		tenantID, p.ExceptionID, casesDomain.ExceptionWorkItemDispute, summary,
		p.ExceptionID, p.CaseID, current.ConversationID, now)
	return err
}

// execRecordFact find-or-creates the fact anchor by the material key and
// writes the version with the full provenance (the expected version is the
// current ladder height of the fact; the scope work item is re-validated
// under the work lock — a terminal scope refuses the write honestly, the
// guard is not trusted outside the transaction).
func (s *Postgres) execRecordFact(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	current, err := currentForUpdate(ctx, tx, tenantID, p.CaseID, p.WorkItemID, -1)
	if err != nil {
		return err
	}
	if current.Terminal {
		return casesDomain.WorkRuleError{Rule: "record_fact", Reason: fmt.Sprintf("факт нельзя записать в скоуп элемента в состоянии %s", current.State)}
	}
	factID, versions, err := factAnchorForUpdate(ctx, tx, tenantID, p.CaseID, p.MaterialKey, p.FactID)
	if err != nil {
		return err
	}
	if versions != p.ExpectedVersion {
		// The fact ladder moved between the read and the transaction: the
		// honest stale refusal (the operator retries and reads the material).
		return casesDomain.ErrStaleVersion
	}
	if versions == 0 {
		if _, err = tx.Exec(ctx, `
                        insert into cases.facts (tenant_id, case_id, fact_id, material_key, created_at)
                        values ($1,$2,$3,$4,$5)`, tenantID, p.CaseID, factID, p.MaterialKey, now); err != nil {
			return mapWorkInsertError(p.Command, err)
		}
	}
	if err := insertFactVersion(ctx, tx, tenantID, p.CaseID, factID, p.WorkItemID, versions+1,
		p.MaterialKey, p.Value, p.ValueDigest, p.SourceType, p.SourceID, p.Channel, p.Actor,
		p.ObservedAt, p.ReceivedAt, now); err != nil {
		return mapWorkInsertError(p.Command, err)
	}
	// The Story 2.3 tokenization: the identifier token of the dictionary
	// keys lands with the fact (the same transaction), the exact matches
	// become the duplicate candidates of the human review.
	if p.IdentifierToken != "" {
		if err := execIdentifierToken(ctx, tx, tenantID, p, now, s); err != nil {
			return err
		}
	}
	return nil
}

// execConfirmFact writes the append-only confirmation bound to the exact
// version (the fact ladder height is re-validated under the lock).
func (s *Postgres) execConfirmFact(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	var anchorCase id.UUID
	err := tx.QueryRow(ctx, `
                select case_id from cases.facts where tenant_id=$1 and fact_id=$2`,
		tenantID, p.FactID).Scan(&anchorCase)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if anchorCase != p.CaseID {
		return casesDomain.ErrNotFound
	}
	var versions int64
	if err = tx.QueryRow(ctx, `
                select count(*) from cases.fact_versions where tenant_id=$1 and fact_id=$2`, tenantID, p.FactID).Scan(&versions); err != nil {
		return err
	}
	if versions != p.ExpectedVersion {
		return casesDomain.ErrStaleVersion
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.fact_confirmations (tenant_id, fact_id, confirmation_id, version_no, confirmer_type, evidence_event_id, note, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		tenantID, p.FactID, p.ExceptionID, p.VersionNo, p.ConfirmerType,
		nilID(p.EvidenceEventID), nilText(p.Note), p.Actor, now); err != nil {
		return mapWorkInsertError(p.Command, err)
	}
	return nil
}

// execRescopeFact writes the new version of the fact scoped to the target
// work item (the value and the digest repeat; the source is the operator
// command).
func (s *Postgres) execRescopeFact(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	target, err := currentForUpdate(ctx, tx, tenantID, p.CaseID, p.TargetWorkItem, -1)
	if err != nil {
		return err
	}
	if target.Terminal {
		return casesDomain.WorkRuleError{Rule: "rescope_fact", Reason: fmt.Sprintf("целевой элемент в состоянии %s не принимает скоуп факта", target.State)}
	}
	var anchorCase id.UUID
	err = tx.QueryRow(ctx, `
                select case_id from cases.facts where tenant_id=$1 and fact_id=$2`,
		tenantID, p.FactID).Scan(&anchorCase)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if anchorCase != p.CaseID {
		return casesDomain.ErrNotFound
	}
	var versions int64
	if err = tx.QueryRow(ctx, `
                select count(*) from cases.fact_versions where tenant_id=$1 and fact_id=$2`, tenantID, p.FactID).Scan(&versions); err != nil {
		return err
	}
	if versions != p.ExpectedVersion {
		return casesDomain.ErrStaleVersion
	}
	return mapWorkInsertError(p.Command, insertFactVersion(ctx, tx, tenantID, p.CaseID, p.FactID, p.TargetWorkItem, versions+1,
		p.MaterialKey, p.Value, p.ValueDigest, p.SourceType, p.SourceID, p.Channel, p.Actor,
		p.ObservedAt, p.ReceivedAt, now))
}

// factAnchorForUpdate resolves (and locks) the fact anchor by the material
// key; the new anchor id is used when the anchor does not exist yet.
func factAnchorForUpdate(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID, materialKey string, newFactID id.UUID) (id.UUID, int64, error) {
	var factID id.UUID
	err := tx.QueryRow(ctx, `
                select fact_id from cases.facts where tenant_id=$1 and case_id=$2 and material_key=$3`,
		tenantID, caseID, materialKey).Scan(&factID)
	if errors.Is(err, pgx.ErrNoRows) {
		return newFactID, 0, nil
	}
	if err != nil {
		return id.UUID(""), 0, err
	}
	var versions int64
	if err = tx.QueryRow(ctx, `
                select count(*) from cases.fact_versions where tenant_id=$1 and fact_id=$2`, tenantID, factID).Scan(&versions); err != nil {
		return id.UUID(""), 0, err
	}
	return factID, versions, nil
}

func insertFactVersion(ctx context.Context, tx pgx.Tx, tenantID, caseID, factID, workItemID id.UUID, versionNo int64, materialKey string, value []byte, valueDigest, sourceType, sourceID, channel string, actorID id.UUID, observedAt, receivedAt, createdAt time.Time) error {
	_, err := tx.Exec(ctx, `
                insert into cases.fact_versions (tenant_id, case_id, fact_id, version_no, work_item_id, material_key, value, value_digest, source_type, source_id, channel, actor_id, observed_at, received_at, created_at)
                values ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9,$10,$11,$12,$13,$14,$15)`,
		tenantID, caseID, factID, versionNo, workItemID, materialKey, value, valueDigest,
		sourceType, sourceID, channel, actorID,
		observedAt.UTC().Truncate(time.Microsecond), receivedAt.UTC().Truncate(time.Microsecond), createdAt.UTC().Truncate(time.Microsecond))
	return err
}

// mapWorkInsertError names the honest conflicts of a raced command insert:
// the fact version ladder (a raced version number), the fact material
// idempotency (a raced identical write), the link uniqueness (the event is
// already linked) and the command record key (a raced identical command).
// Every 23505 of the work surface maps to a named signal; an unmapped one
// surfaces as the error itself (and fails the tests, not the operators).
func mapWorkInsertError(command string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch {
		case pgErr.ConstraintName == "fact_versions_source_material":
			return casesDomain.ErrFactMaterialExists
		case pgErr.ConstraintName == "work_item_event_links_pkey":
			return casesDomain.WorkRuleError{Rule: command, Reason: "Событие уже связано с этим рабочим элементом: повторная связь не создаётся."}
		case pgErr.ConstraintName == "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case pgErr.ConstraintName == "fact_versions_pkey":
			// The version ladder moved under a writer the lock plan
			// could not cover — the honest raced signal (the operator
			// retries against the fresh material), never a raw error.
			return casesDomain.ErrStaleVersion
		case pgErr.ConstraintName == "facts_case_material":
			// The anchor exists (a raced find-or-create): the retry resolves
			// against the existing anchor — the honest stale signal.
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

func nilText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nilID(value id.UUID) any {
	if value.Valid() {
		return value
	}
	return nil
}

// nilInt64 renders the nullable integer column (zero — the honest absence
// of the Story 2.9 presentation pair half).
func nilInt64(value int64) any {
	if value != 0 {
		return value
	}
	return nil
}

func itoa64(value int64) string {
	if value == 0 {
		return "0"
	}
	out := []byte{}
	for value > 0 {
		out = append([]byte{byte('0' + value%10)}, out...)
		value /= 10
	}
	return string(out)
}
