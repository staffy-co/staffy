package store

import (
        "context"
        "encoding/json"
        "errors"
        "time"

        privacyDomain "github.com/artemiosu/staffy/internal/privacy/domain"
        "github.com/artemiosu/staffy/internal/privacy/port"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
        "github.com/jackc/pgx/v5"
        "github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

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
        for _, pair := range [][2]string{{"staffy.principal_id", scope.PrincipalID().String()}, {"staffy.tenant_id", scope.TenantID().String()}} {
                if _, err := tx.Exec(ctx, "select set_config($1,$2,true)", pair[0], pair[1]); err != nil {
                        _ = tx.Rollback(ctx)
                        return nil, err
                }
        }
        return tx, nil
}

func (s *Postgres) lock(ctx context.Context, tx pgx.Tx, scope tenancy.Context, kind string, key string) error {
        lock := scope.TenantID().String() + ":" + kind + ":" + key
        _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", lock)
        return err
}

const mapColumns = "tenant_id,class,version,purpose,legal_basis,controller,processor,recipients,sub_processors,locations,accessible_roles,known_copies,retention_policy_ref,deletion_method,export_method,actor_id,created_at"

type mapRow interface{ Scan(dest ...any) error }

func scanMapEntry(row mapRow) (privacyDomain.DataMapEntry, error) {
        var e privacyDomain.DataMapEntry
        var class string
        var recipients, subProcessors, locations, roles, copies []byte
        err := row.Scan(&e.TenantID, &class, &e.Version, &e.Purpose, &e.LegalBasis, &e.Controller, &e.Processor,
                &recipients, &subProcessors, &locations, &roles, &copies, &e.RetentionPolicy,
                &e.DeletionMethod, &e.ExportMethod, &e.ActorID, &e.CreatedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return privacyDomain.DataMapEntry{}, privacyDomain.ErrNotFound
        }
        if err != nil {
                return privacyDomain.DataMapEntry{}, err
        }
        e.Class = privacyDomain.ArtifactClass(class)
        e.Recipients = stringsList(recipients)
        e.SubProcessors = stringsList(subProcessors)
        e.Locations = stringsList(locations)
        e.AccessibleRoles = stringsList(roles)
        e.KnownCopies = stringsList(copies)
        return e, nil
}

func stringsList(raw []byte) []string {
        out := []string{}
        if len(raw) > 0 {
                _ = json.Unmarshal(raw, &out)
        }
        return out
}

func jsonList(values []string) []byte {
        if values == nil {
                values = []string{}
        }
        out, err := json.Marshal(values)
        if err != nil {
                return []byte("[]")
        }
        return out
}

func classList(classes []privacyDomain.ArtifactClass) []byte {
        raw := make([]string, 0, len(classes))
        for _, class := range classes {
                raw = append(raw, string(class))
        }
        return jsonList(raw)
}

// SaveMapEntry appends the next version of the class entry under a per-class
// advisory lock. Attributes are stored as submitted: unknown mandatory
// attributes stay empty and the entry stays honestly incomplete.
func (s *Postgres) SaveMapEntry(ctx context.Context, p port.MapEntryParams) (privacyDomain.DataMapEntry, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return privacyDomain.DataMapEntry{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        if err := s.lock(ctx, tx, p.Scope, "privacy.map", string(p.Class)); err != nil {
                return privacyDomain.DataMapEntry{}, err
        }
        var version int64
        if err = tx.QueryRow(ctx, "select coalesce(max(version),0)+1 from privacy.data_map_entries where tenant_id=$1 and class=$2", p.Scope.TenantID(), string(p.Class)).Scan(&version); err != nil {
                return privacyDomain.DataMapEntry{}, err
        }
        entry := p.Entry
        entry.TenantID = p.Scope.TenantID().String()
        entry.Class = p.Class
        entry.Version = version
        entry.ActorID = p.Scope.PrincipalID().String()
        entry.CreatedAt = p.Now
        if _, err = tx.Exec(ctx, "insert into privacy.data_map_entries("+mapColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)",
                entry.TenantID, string(entry.Class), entry.Version, entry.Purpose, entry.LegalBasis, entry.Controller, entry.Processor,
                jsonList(entry.Recipients), jsonList(entry.SubProcessors), jsonList(entry.Locations), jsonList(entry.AccessibleRoles), jsonList(entry.KnownCopies),
                entry.RetentionPolicy, entry.DeletionMethod, entry.ExportMethod, entry.ActorID, entry.CreatedAt); err != nil {
                return privacyDomain.DataMapEntry{}, err
        }
        return entry, tx.Commit(ctx)
}

func (s *Postgres) ListMapEntries(ctx context.Context, scope tenancy.Context) ([]privacyDomain.DataMapEntry, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+mapColumns+" from privacy.data_map_entries where tenant_id=$1 order by class, version desc", scope.TenantID())
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []privacyDomain.DataMapEntry{}
        for rows.Next() {
                e, err := scanMapEntry(rows)
                if err != nil {
                        return nil, err
                }
                out = append(out, e)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        return out, tx.Commit(ctx)
}

const policyColumns = "tenant_id,class,version,purpose,legal_basis,source,terminal_event,active_use_days,archive_days,deletion_days,backup_expiry_days,legal_hold_exception,owner_note,actor_id,created_at"

func scanPolicy(row mapRow) (privacyDomain.RetentionPolicy, error) {
        var p privacyDomain.RetentionPolicy
        var class string
        err := row.Scan(&p.TenantID, &class, &p.Version, &p.Purpose, &p.LegalBasis, &p.Source, &p.TerminalEvent,
                &p.ActiveUseDays, &p.ArchiveDays, &p.DeletionDays, &p.BackupExpiryDays, &p.LegalHoldException, &p.OwnerNote, &p.ActorID, &p.CreatedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return privacyDomain.RetentionPolicy{}, privacyDomain.ErrNotFound
        }
        if err != nil {
                return privacyDomain.RetentionPolicy{}, err
        }
        p.Class = privacyDomain.ArtifactClass(class)
        return p, nil
}

// SavePolicy appends the next retention policy version of the class under a
// per-class advisory lock.
func (s *Postgres) SavePolicy(ctx context.Context, p port.PolicyParams) (privacyDomain.RetentionPolicy, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return privacyDomain.RetentionPolicy{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        if err := s.lock(ctx, tx, p.Scope, "privacy.policy", string(p.Class)); err != nil {
                return privacyDomain.RetentionPolicy{}, err
        }
        var version int64
        if err = tx.QueryRow(ctx, "select coalesce(max(version),0)+1 from privacy.retention_policies where tenant_id=$1 and class=$2", p.Scope.TenantID(), string(p.Class)).Scan(&version); err != nil {
                return privacyDomain.RetentionPolicy{}, err
        }
        policy := p.Policy
        policy.TenantID = p.Scope.TenantID().String()
        policy.Class = p.Class
        policy.Version = version
        policy.ActorID = p.Scope.PrincipalID().String()
        policy.CreatedAt = p.Now
        if _, err = tx.Exec(ctx, "insert into privacy.retention_policies("+policyColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)",
                policy.TenantID, string(policy.Class), policy.Version, policy.Purpose, policy.LegalBasis, policy.Source, policy.TerminalEvent,
                policy.ActiveUseDays, policy.ArchiveDays, policy.DeletionDays, policy.BackupExpiryDays, policy.LegalHoldException, policy.OwnerNote, policy.ActorID, policy.CreatedAt); err != nil {
                return privacyDomain.RetentionPolicy{}, err
        }
        return policy, tx.Commit(ctx)
}

func (s *Postgres) ListPolicies(ctx context.Context, scope tenancy.Context) ([]privacyDomain.RetentionPolicy, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+policyColumns+" from privacy.retention_policies where tenant_id=$1 order by class, version desc", scope.TenantID())
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []privacyDomain.RetentionPolicy{}
        for rows.Next() {
                p, err := scanPolicy(rows)
                if err != nil {
                        return nil, err
                }
                out = append(out, p)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        return out, tx.Commit(ctx)
}

const holdColumns = "tenant_id,event_id,event,classes,basis,approver,review_deadline,actor_id,created_at"

func scanHoldEvent(row mapRow) (privacyDomain.LegalHoldEvent, error) {
        var e privacyDomain.LegalHoldEvent
        var classes []byte
        err := row.Scan(&e.TenantID, &e.EventID, &e.Event, &classes, &e.Basis, &e.Approver, &e.ReviewDeadline, &e.ActorID, &e.CreatedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return privacyDomain.LegalHoldEvent{}, privacyDomain.ErrNotFound
        }
        if err != nil {
                return privacyDomain.LegalHoldEvent{}, err
        }
        for _, raw := range stringsList(classes) {
                e.Classes = append(e.Classes, privacyDomain.ArtifactClass(raw))
        }
        return e, nil
}

func (s *Postgres) SaveHoldEvent(ctx context.Context, p port.HoldEventParams) error {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        event := p.Event
        event.TenantID = p.Scope.TenantID().String()
        event.ActorID = p.Scope.PrincipalID().String()
        event.CreatedAt = p.Now
        if _, err = tx.Exec(ctx, "insert into privacy.legal_hold_events("+holdColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9)",
                event.TenantID, event.EventID, event.Event, classList(event.Classes), event.Basis, event.Approver, event.ReviewDeadline, event.ActorID, event.CreatedAt); err != nil {
                return err
        }
        return tx.Commit(ctx)
}

func (s *Postgres) ListHoldEvents(ctx context.Context, scope tenancy.Context) ([]privacyDomain.LegalHoldEvent, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+holdColumns+" from privacy.legal_hold_events where tenant_id=$1 order by created_at desc, event_id desc", scope.TenantID())
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []privacyDomain.LegalHoldEvent{}
        for rows.Next() {
                e, err := scanHoldEvent(rows)
                if err != nil {
                        return nil, err
                }
                out = append(out, e)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        return out, tx.Commit(ctx)
}

const requestColumns = "tenant_id,request_id,actor_id,idempotency_key,request_hash,scope,authority,requested_at"

func scanRequest(row mapRow) (port.DeletionRequestRecord, error) {
        var r port.DeletionRequestRecord
        var scope []byte
        var tenant, authority string
        err := row.Scan(&tenant, &r.RequestID, &r.ActorID, &r.IdempotencyKey, &r.RequestHash, &scope, &authority, &r.RequestedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return port.DeletionRequestRecord{}, privacyDomain.ErrNotFound
        }
        if err != nil {
                return port.DeletionRequestRecord{}, err
        }
        for _, raw := range stringsList(scope) {
                r.Scope = append(r.Scope, privacyDomain.ArtifactClass(raw))
        }
        return r, nil
}

const outcomeColumns = "tenant_id,request_id,class,outcome,affected_count,due_at,note,block_required,block_gate,block_owner,reconciliation_owner,reconciliation_deadline,created_at"

func scanOutcome(row mapRow) (privacyDomain.DeletionOutcome, error) {
        var o privacyDomain.DeletionOutcome
        var class, tenant, requestID string
        var blockRequired, blockGate, blockOwner, reconOwner *string
        var dueAt, reconDeadline *time.Time
        var createdAt time.Time
        err := row.Scan(&tenant, &requestID, &class, &o.Outcome, &o.AffectedCount, &dueAt, &o.Note,
                &blockRequired, &blockGate, &blockOwner, &reconOwner, &reconDeadline, &createdAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return privacyDomain.DeletionOutcome{}, privacyDomain.ErrNotFound
        }
        if err != nil {
                return privacyDomain.DeletionOutcome{}, err
        }
        o.Class = privacyDomain.ArtifactClass(class)
        if dueAt != nil {
                o.DueAt = dueAt
        }
        if blockRequired != nil && *blockRequired != "" {
                o.Block = &privacyDomain.ManagedBlock{Required: *blockRequired, Gate: deref(blockGate), Owner: deref(blockOwner)}
        }
        if reconOwner != nil && *reconOwner != "" {
                o.ReconciliationOwner = *reconOwner
                if reconDeadline != nil {
                        o.ReconciliationDeadline = reconDeadline
                }
        }
        return o, nil
}

func deref(value *string) string {
        if value == nil {
                return ""
        }
        return *value
}

const receiptColumns = "tenant_id,receipt_id,request_id,receipt_hash,authority,policy_versions,outcome_summary,completed_at,actor_id"

func scanReceipt(row mapRow) (privacyDomain.Receipt, error) {
        var r privacyDomain.Receipt
        var policyVersions, outcomeSummary []byte
        err := row.Scan(&r.TenantID, &r.ReceiptID, &r.RequestID, &r.ReceiptHash, &r.Authority, &policyVersions, &outcomeSummary, &r.CompletedAt, &r.ActorID)
        if errors.Is(err, pgx.ErrNoRows) {
                return privacyDomain.Receipt{}, privacyDomain.ErrNotFound
        }
        if err != nil {
                return privacyDomain.Receipt{}, err
        }
        r.PolicyVersions = []privacyDomain.PolicyPin{}
        if len(policyVersions) > 0 {
                _ = json.Unmarshal(policyVersions, &r.PolicyVersions)
        }
        r.Outcomes = []privacyDomain.DeletionOutcome{}
        return r, nil
}

// LookupDeletion answers the idempotency question without side effects.
func (s *Postgres) LookupDeletion(ctx context.Context, scope tenancy.Context, actorID id.UUID, key, hash string) (port.DeletionRequestRecord, []privacyDomain.DeletionOutcome, privacyDomain.Receipt, bool, error) {
        if !scope.Valid() || key == "" {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, nil
        }
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        request, outcomes, receipt, found, err := readDeletionBundle(ctx, tx, scope.TenantID(), actorID, key)
        if err != nil || !found {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
        }
        if request.RequestHash != hash {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, privacyDomain.ErrIdempotencyConflict
        }
        return request, outcomes, receipt, true, tx.Commit(ctx)
}

// readDeletionBundle loads the request row by idempotency key plus its
// outcomes and the latest receipt row.
func readDeletionBundle(ctx context.Context, tx pgx.Tx, tenantID, actorID interface{}, key string) (port.DeletionRequestRecord, []privacyDomain.DeletionOutcome, privacyDomain.Receipt, bool, error) {
        request, err := scanRequest(tx.QueryRow(ctx, "select "+requestColumns+" from privacy.deletion_requests where tenant_id=$1 and actor_id=$2 and idempotency_key=$3", tenantID, actorID, key))
        if errors.Is(err, privacyDomain.ErrNotFound) {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, nil
        }
        if err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
        }
        outcomes, err := readOutcomes(ctx, tx, tenantID, request.RequestID)
        if err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
        }
        receipt, err := readLatestReceipt(ctx, tx, tenantID, request.RequestID)
        if err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
        }
        return request, outcomes, receipt, true, nil
}

func readOutcomes(ctx context.Context, tx pgx.Tx, tenantID interface{}, requestID id.UUID) ([]privacyDomain.DeletionOutcome, error) {
        rows, err := tx.Query(ctx, "select "+outcomeColumns+" from privacy.deletion_outcomes where tenant_id=$1 and request_id=$2 order by class", tenantID, requestID)
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []privacyDomain.DeletionOutcome{}
        for rows.Next() {
                o, err := scanOutcome(rows)
                if err != nil {
                        return nil, err
                }
                out = append(out, o)
        }
        return out, rows.Err()
}

func readLatestReceipt(ctx context.Context, tx pgx.Tx, tenantID interface{}, requestID id.UUID) (privacyDomain.Receipt, error) {
        receipt, err := scanReceipt(tx.QueryRow(ctx, "select "+receiptColumns+" from privacy.lifecycle_receipts where tenant_id=$1 and request_id=$2 order by completed_at desc, receipt_id desc limit 1", tenantID, requestID))
        if errors.Is(err, privacyDomain.ErrNotFound) {
                return privacyDomain.Receipt{}, nil
        }
        return receipt, err
}

// SaveDeletion atomically persists the request bundle. The per-key advisory
// lock serializes concurrent duplicates: the same key with the same request
// hash replays the stored bundle; a different hash is an honest conflict.
func (s *Postgres) SaveDeletion(ctx context.Context, p port.DeletionRecordParams) (port.DeletionRequestRecord, []privacyDomain.DeletionOutcome, privacyDomain.Receipt, bool, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        lock := p.Scope.TenantID().String() + ":privacy.deletion.v1:" + p.Scope.PrincipalID().String() + ":" + p.Request.IdempotencyKey
        if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", lock); err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
        }
        request, outcomes, receipt, found, err := readDeletionBundle(ctx, tx, p.Scope.TenantID(), p.Scope.PrincipalID(), p.Request.IdempotencyKey)
        if err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
        }
        if found {
                if request.RequestHash != p.Request.RequestHash {
                        return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, privacyDomain.ErrIdempotencyConflict
                }
                return request, outcomes, receipt, true, tx.Commit(ctx)
        }
        if _, err = tx.Exec(ctx, "insert into privacy.deletion_requests("+requestColumns+") values($1,$2,$3,$4,$5,$6,'data_governance',$7)",
                p.Scope.TenantID(), p.Request.RequestID, p.Scope.PrincipalID(), p.Request.IdempotencyKey, p.Request.RequestHash, classList(p.Request.Scope), p.Request.RequestedAt); err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
        }
        for i := range p.Outcomes {
                o := p.Outcomes[i]
                var dueAt, reconDeadline interface{}
                if o.DueAt != nil {
                        dueAt = *o.DueAt
                }
                if o.ReconciliationDeadline != nil {
                        reconDeadline = *o.ReconciliationDeadline
                }
                var blockRequired, blockGate, blockOwner, reconOwner interface{}
                if o.Block != nil {
                        blockRequired, blockGate, blockOwner = o.Block.Required, o.Block.Gate, o.Block.Owner
                }
                if o.ReconciliationOwner != "" {
                        reconOwner = o.ReconciliationOwner
                }
                if _, err = tx.Exec(ctx, "insert into privacy.deletion_outcomes("+outcomeColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)",
                        p.Scope.TenantID(), p.Request.RequestID, string(o.Class), o.Outcome, o.AffectedCount, dueAt, o.Note,
                        blockRequired, blockGate, blockOwner, reconOwner, reconDeadline, p.Receipt.CompletedAt); err != nil {
                        return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
                }
        }
        summary := make([]privacyDomain.ReceiptLine, 0, len(p.Outcomes))
        for _, o := range p.Outcomes {
                summary = append(summary, privacyDomain.ReceiptLine{Class: o.Class, Outcome: o.Outcome, AffectedCount: o.AffectedCount})
        }
        pins, err := json.Marshal(p.Receipt.PolicyVersions)
        if err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
        }
        summaryJSON, err := json.Marshal(summary)
        if err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
        }
        if _, err = tx.Exec(ctx, "insert into privacy.lifecycle_receipts("+receiptColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9)",
                p.Scope.TenantID(), p.Receipt.ReceiptID, p.Request.RequestID, p.Receipt.ReceiptHash, p.Receipt.Authority, pins, summaryJSON, p.Receipt.CompletedAt, p.Scope.PrincipalID()); err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, err
        }
        return p.Request, p.Outcomes, p.Receipt, false, tx.Commit(ctx)
}

func (s *Postgres) ListReceipts(ctx context.Context, scope tenancy.Context) ([]privacyDomain.Receipt, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+receiptColumns+" from privacy.lifecycle_receipts r where tenant_id=$1 and completed_at = (select max(r2.completed_at) from privacy.lifecycle_receipts r2 where r2.tenant_id=r.tenant_id and r2.request_id=r.request_id) order by completed_at desc, receipt_id desc limit 50", scope.TenantID())
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []privacyDomain.Receipt{}
        for rows.Next() {
                r, err := scanReceipt(rows)
                if err != nil {
                        return nil, err
                }
                out = append(out, r)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        return out, tx.Commit(ctx)
}

func (s *Postgres) ReadReceipt(ctx context.Context, scope tenancy.Context, requestID id.UUID) (port.DeletionRequestRecord, []privacyDomain.DeletionOutcome, privacyDomain.Receipt, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        request, err := scanRequest(tx.QueryRow(ctx, "select "+requestColumns+" from privacy.deletion_requests where tenant_id=$1 and request_id=$2", scope.TenantID(), requestID))
        if err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, err
        }
        outcomes, err := readOutcomes(ctx, tx, scope.TenantID(), requestID)
        if err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, err
        }
        receipt, err := readLatestReceipt(ctx, tx, scope.TenantID(), requestID)
        if err != nil {
                return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, err
        }
        receipt.Outcomes = outcomes
        return request, outcomes, receipt, tx.Commit(ctx)
}

// classReferenceSQL maps every class to the SQL computing the last-change
// moment of its data. The queries run under the tenant RLS context, so rows
// of other tenants are invisible by construction.
var classReferenceSQL = map[privacyDomain.ArtifactClass]string{
        privacyDomain.ClassTenantConfiguration:    "select max(created_at) from identity.tenants where tenant_id=$1",
        privacyDomain.ClassMemberIdentity:         "select max(x) from (select max(p.created_at) as x from identity.principals p where p.principal_id in (select m.principal_id from identity.memberships m where m.tenant_id=$1) union all select max(m.updated_at) from identity.memberships m where m.tenant_id=$1 union all select max(i.updated_at) from identity.invitations i where i.tenant_id=$1) t",
        privacyDomain.ClassEmployeeDraft:          "select max(updated_at) from employee.drafts where tenant_id=$1",
        privacyDomain.ClassDomainPackPin:          "select max(updated_at) from employee.drafts where tenant_id=$1 and domain_pack is not null",
        privacyDomain.ClassKnowledgeSourcePayload: "select max(x) from (select max(s.updated_at) as x from employee.knowledge_sources s where s.tenant_id=$1 union all select max(v.created_at) from employee.knowledge_versions v where v.tenant_id=$1) t",
        privacyDomain.ClassBehaviorConfiguration:  "select max(changed_at) from employee.behavior_versions where tenant_id=$1",
        privacyDomain.ClassVoiceConfiguration:     "select max(changed_at) from employee.voice_versions where tenant_id=$1",
        privacyDomain.ClassVoicePreviewMedia:      "select max(created_at) from employee.voice_previews where tenant_id=$1",
        privacyDomain.ClassVoiceAssessment:        "select max(assessed_at) from employee.voice_assessments where tenant_id=$1",
        privacyDomain.ClassConnectionConfig:       "select max(x) from (select max(c.updated_at) as x from connector.connections c where c.tenant_id=$1 union all select max(v.changed_at) from connector.connection_versions v where v.tenant_id=$1) t",
        privacyDomain.ClassConnectionSecretSeal:   "select max(x) from (select max(created_at) as x from connector.secret_seals s where s.tenant_id=$1 union all select max(revoked_at) from connector.secret_seals s where s.tenant_id=$1) t",
        privacyDomain.ClassCapabilityEvidence:     "select max(completed_at) from connector.capability_evidence where tenant_id=$1",
        privacyDomain.ClassIdempotencyRecords:     "select max(x) from (select max(created_at) as x from identity.idempotency_records where tenant_id=$1 union all select max(created_at) from employee.idempotency_records where tenant_id=$1 union all select max(created_at) from connector.idempotency_records where tenant_id=$1) t",
        privacyDomain.ClassAuditTrail:             "select max(x) from (select max(occurred_at) as x from identity.membership_audit where tenant_id=$1 union all select max(occurred_at) from employee.audit where tenant_id=$1 union all select max(occurred_at) from employee.knowledge_audit where tenant_id=$1 union all select max(occurred_at) from connector.audit where tenant_id=$1) t",
        privacyDomain.ClassOutboxEvents:           "select max(x) from (select max(occurred_at) as x from identity.outbox_events where tenant_id=$1 union all select max(occurred_at) from employee.outbox_events where tenant_id=$1 union all select max(occurred_at) from employee.knowledge_outbox where tenant_id=$1 union all select max(occurred_at) from connector.outbox_events where tenant_id=$1) t",
}

// ClassReferenceTime computes the term anchor of one class: the last-change
// moment of its rows. No rows or no table for the class means no anchor; the
// caller then falls back to the policy publication time.
func (s *Postgres) ClassReferenceTime(ctx context.Context, scope tenancy.Context, class privacyDomain.ArtifactClass) (*time.Time, error) {
        query, ok := classReferenceSQL[class]
        if !ok {
                return nil, privacyDomain.ErrUnknownClass
        }
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        var anchor *time.Time
        if err = tx.QueryRow(ctx, query, scope.TenantID()).Scan(&anchor); err != nil {
                return nil, err
        }
        return anchor, tx.Commit(ctx)
}
