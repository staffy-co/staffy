package store

import (
        "context"
        "encoding/json"
        "errors"
        "fmt"
        "strings"
        "time"

        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
        "github.com/artemiosu/staffy/internal/connector/port"
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

const connectionColumns = "tenant_id,connection_id,connector_id,connector_version,label,environment,status,authentication_state,transport_state,config,secret_id,secret_hint,secret_active,owner_id,version,created_at,updated_at"

const evidenceColumns = "tenant_id,evidence_id,connection_id,secret_id,environment,contract_version,adapter_build,capability,probe_kind,actor_id,requested_at,deadline_at,completed_at,outcome,delivery_level,redacted_request,redacted_result,expires_at,idempotency_key,request_hash"

type connectionRow interface {
        Scan(dest ...any) error
}

func scanConnection(row connectionRow) (connectorDomain.Connection, error) {
        var c connectorDomain.Connection
        var config []byte
        var secretID, secretHint *string
        err := row.Scan(&c.TenantID, &c.ConnectionID, &c.ConnectorID, &c.ConnectorVer, &c.Label, &c.Environment, &c.Status, &c.AuthState, &c.Transport, &config, &secretID, &secretHint, &c.SecretActive, &c.OwnerID, &c.Version, &c.CreatedAt, &c.UpdatedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return connectorDomain.Connection{}, connectorDomain.ErrNotFound
        }
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        if secretID != nil {
                c.SecretID = id.UUID(*secretID)
        }
        if secretHint != nil {
                c.SecretHint = *secretHint
        }
        c.Config = map[string]string{}
        if len(config) > 0 {
                if err := json.Unmarshal(config, &c.Config); err != nil {
                        return connectorDomain.Connection{}, err
                }
        }
        return c, nil
}

func (s *Postgres) Create(ctx context.Context, p port.CreateParams) (connectorDomain.Connection, bool, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return connectorDomain.Connection{}, false, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        lock := p.Scope.TenantID().String() + ":" + p.Scope.PrincipalID().String() + ":connector.create.v1:" + p.Key
        if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", lock); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        var oldHash string
        var oldID id.UUID
        err = tx.QueryRow(ctx, "select request_hash,connection_id from connector.idempotency_records where tenant_id=$1 and actor_id=$2 and operation='connector.create.v1' and idempotency_key=$3", p.Scope.TenantID(), p.Scope.PrincipalID(), p.Key).Scan(&oldHash, &oldID)
        if err == nil {
                if oldHash != p.Hash {
                        var currentVersion int64
                        if err = tx.QueryRow(ctx, "select version from connector.connections where tenant_id=$1 and connection_id=$2", p.Scope.TenantID(), oldID).Scan(&currentVersion); err != nil {
                                return connectorDomain.Connection{}, false, err
                        }
                        if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,0,$5,'conflict',$6,$7)", p.Scope.TenantID(), p.AuditID, oldID, p.Scope.PrincipalID(), currentVersion, p.CorrelationID, p.Now); err != nil {
                                return connectorDomain.Connection{}, false, err
                        }
                        if err = tx.Commit(ctx); err != nil {
                                return connectorDomain.Connection{}, false, err
                        }
                        return connectorDomain.Connection{}, false, connectorDomain.ErrIdempotencyConflict
                }
                c, e := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2", p.Scope.TenantID(), oldID))
                if e != nil {
                        return connectorDomain.Connection{}, false, e
                }
                return c, true, tx.Commit(ctx)
        }
        if !errors.Is(err, pgx.ErrNoRows) {
                return connectorDomain.Connection{}, false, err
        }
        config, err := json.Marshal(p.Config)
        if err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.connections(tenant_id,connection_id,connector_id,connector_version,label,environment,status,authentication_state,transport_state,config,owner_id,version,created_at,updated_at) values($1,$2,$3,$4,$5,$6,'draft','pending','pending',$7,$8,1,$9,$9)", p.Scope.TenantID(), p.ConnectionID, p.ConnectorID, p.ConnectorVer, p.Label, p.Environment, config, p.Scope.PrincipalID(), p.Now); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.connection_versions(tenant_id,connection_id,version,connector_id,connector_version,label,environment,status,authentication_state,transport_state,config,actor_id,change_summary,changed_at) values($1,$2,1,$3,$4,$5,$6,'draft','pending','pending',$7,$8,$9,$10)", p.Scope.TenantID(), p.ConnectionID, p.ConnectorID, p.ConnectorVer, p.Label, p.Environment, config, p.Scope.PrincipalID(), "created", p.Now); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.idempotency_records(tenant_id,actor_id,operation,idempotency_key,request_hash,connection_id,created_at) values($1,$2,'connector.create.v1',$3,$4,$5,$6)", p.Scope.TenantID(), p.Scope.PrincipalID(), p.Key, p.Hash, p.ConnectionID, p.Now); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,0,1,'created',$5,$6)", p.Scope.TenantID(), p.AuditID, p.ConnectionID, p.Scope.PrincipalID(), p.CorrelationID, p.Now); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.outbox_events(tenant_id,event_id,connection_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,'connector.connection_created',1,1,$4,$5,$6,$7)", p.Scope.TenantID(), p.EventID, p.ConnectionID, p.Scope.PrincipalID(), p.CorrelationID, p.Now, []byte(fmt.Sprintf(`{"connection_id":%q,"connector_id":%q,"environment":%q,"version":1}`, p.ConnectionID.String(), p.ConnectorID, p.Environment))); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        c, e := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2", p.Scope.TenantID(), p.ConnectionID))
        if e != nil {
                return connectorDomain.Connection{}, false, e
        }
        return c, false, tx.Commit(ctx)
}

func (s *Postgres) List(ctx context.Context, scope tenancy.Context) ([]connectorDomain.Connection, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 order by updated_at desc,connection_id", scope.TenantID())
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []connectorDomain.Connection{}
        for rows.Next() {
                c, e := scanConnection(rows)
                if e != nil {
                        return nil, e
                }
                out = append(out, c)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        return out, tx.Commit(ctx)
}

func (s *Postgres) Read(ctx context.Context, scope tenancy.Context, connectionID id.UUID) (connectorDomain.Connection, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        c, err := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2", scope.TenantID(), connectionID))
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        return c, tx.Commit(ctx)
}

func (s *Postgres) ReadVersions(ctx context.Context, scope tenancy.Context, connectionID id.UUID) ([]connectorDomain.ConnectionVersion, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select v.tenant_id,v.connection_id,v.connector_id,v.connector_version,v.label,v.environment,v.status,v.authentication_state,v.transport_state,v.config,v.secret_id,v.secret_hint,v.secret_active,(select c2.owner_id from connector.connections c2 where c2.tenant_id=v.tenant_id and c2.connection_id=v.connection_id),v.version,v.changed_at,v.actor_id,v.change_summary from connector.connection_versions v where v.tenant_id=$1 and v.connection_id=$2 order by v.version desc", scope.TenantID(), connectionID)
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []connectorDomain.ConnectionVersion{}
        for rows.Next() {
                var v connectorDomain.ConnectionVersion
                var config []byte
                var secretID, secretHint *string
                if err = rows.Scan(&v.TenantID, &v.ConnectionID, &v.ConnectorID, &v.ConnectorVer, &v.Label, &v.Environment, &v.Status, &v.AuthState, &v.Transport, &config, &secretID, &secretHint, &v.SecretActive, &v.OwnerID, &v.Version, &v.CreatedAt, &v.ActorID, &v.ChangeSummary); err != nil {
                        return nil, err
                }
                if secretID != nil {
                        v.SecretID = id.UUID(*secretID)
                }
                if secretHint != nil {
                        v.SecretHint = *secretHint
                }
                v.Config = map[string]string{}
                if len(config) > 0 {
                        if err := json.Unmarshal(config, &v.Config); err != nil {
                                return nil, err
                        }
                }
                out = append(out, v)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        return out, tx.Commit(ctx)
}

// Save writes a new immutable version. Config changes reset the check states
// to pending and the status to draft: prior evidence no longer describes the
// stored values.
func (s *Postgres) Save(ctx context.Context, p port.SaveParams) (connectorDomain.Connection, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        var current connectorDomain.Connection
        current, err = scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2 for update", p.Scope.TenantID(), p.ConnectionID))
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        if current.Version != p.ExpectedVersion {
                if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'conflict',$7,$8)", p.Scope.TenantID(), p.AuditID, p.ConnectionID, p.Scope.PrincipalID(), p.ExpectedVersion, current.Version, p.CorrelationID, p.Now); err != nil {
                        return connectorDomain.Connection{}, err
                }
                if err = tx.Commit(ctx); err != nil {
                        return connectorDomain.Connection{}, err
                }
                return connectorDomain.Connection{}, connectorDomain.ErrConflict
        }
        nextStatus := current.Status
        if current.Status != connectorDomain.StatusRevoked {
                nextStatus = connectorDomain.StatusDraft
        }
        config, err := json.Marshal(p.Config)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        version := current.Version + 1
        if _, err = tx.Exec(ctx, "update connector.connections set label=$3,config=$4,status=$5,authentication_state='pending',transport_state='pending',version=$6,updated_at=$7 where tenant_id=$1 and connection_id=$2", p.Scope.TenantID(), p.ConnectionID, p.Label, config, nextStatus, version, p.Now); err != nil {
                return connectorDomain.Connection{}, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.connection_versions(tenant_id,connection_id,version,connector_id,connector_version,label,environment,status,authentication_state,transport_state,config,secret_id,secret_hint,secret_active,actor_id,change_summary,changed_at) values($1,$2,$3,$4,$5,$6,$7,'draft','pending','pending',$8,$9,$10,$11,$12,$13,$14)", p.Scope.TenantID(), p.ConnectionID, version, current.ConnectorID, current.ConnectorVer, p.Label, current.Environment, config, uuidArg(current.SecretID), current.SecretHint, current.SecretActive, p.Scope.PrincipalID(), p.Summary, p.Now); err != nil {
                return connectorDomain.Connection{}, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'saved',$7,$8)", p.Scope.TenantID(), p.AuditID, p.ConnectionID, p.Scope.PrincipalID(), p.ExpectedVersion, version, p.CorrelationID, p.Now); err != nil {
                return connectorDomain.Connection{}, err
        }
        payload, _ := json.Marshal(map[string]any{"connection_id": p.ConnectionID, "version": version, "label": p.Label})
        if _, err = tx.Exec(ctx, "insert into connector.outbox_events(tenant_id,event_id,connection_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,'connector.connection_saved',1,$4,$5,$6,$7,$8)", p.Scope.TenantID(), p.EventID, p.ConnectionID, version, p.Scope.PrincipalID(), p.CorrelationID, p.Now, payload); err != nil {
                return connectorDomain.Connection{}, err
        }
        c, e := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2", p.Scope.TenantID(), p.ConnectionID))
        if e != nil {
                return connectorDomain.Connection{}, e
        }
        return c, tx.Commit(ctx)
}

// Connect stores the sealed secret, records the redacted basic check result,
// and revokes the previous seal on rotation. Payload columns never contain
// secret material: only the opaque secret id and the short hint.
// The command is idempotent: a retry with the same key replays the stored
// outcome (the immutable version snapshot) instead of a false conflict.
func (s *Postgres) Connect(ctx context.Context, p port.ConnectParams) (connectorDomain.Connection, bool, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return connectorDomain.Connection{}, false, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        if p.IdempotencyKey != "" {
                replayed, conn, err := s.replayIfRecorded(ctx, tx, p.Scope, p.Operation, p.IdempotencyKey, p.RequestHash, p.ConnectionID, p.ExpectedVersion, p.AuditID, p.CorrelationID, p.Now)
                if err != nil || replayed {
                        return conn, replayed, err
                }
        }
        current, err := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2 for update", p.Scope.TenantID(), p.ConnectionID))
        if err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if current.Version != p.ExpectedVersion {
                if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'conflict',$7,$8)", p.Scope.TenantID(), p.AuditID, p.ConnectionID, p.Scope.PrincipalID(), p.ExpectedVersion, current.Version, p.CorrelationID, p.Now); err != nil {
                        return connectorDomain.Connection{}, false, err
                }
                if err = tx.Commit(ctx); err != nil {
                        return connectorDomain.Connection{}, false, err
                }
                return connectorDomain.Connection{}, false, connectorDomain.ErrConflict
        }
        // Single-secret rule under the row lock: with the expected version matched,
        // a plain connect over an accepted secret is an honest rule rejection; the
        // concurrent race is caught above by the CAS conflict instead.
        if !p.Rotate && current.SecretActive {
                return connectorDomain.Connection{}, false, connectorDomain.RuleError{Rule: "secret", Reason: "Секрет уже принят. Для замены используйте ротацию."}
        }
        if p.Rotate && current.SecretActive {
                if _, err = tx.Exec(ctx, "update connector.secret_seals set revoked_at=$4 where tenant_id=$1 and connection_id=$2 and secret_id=$3 and revoked_at is null", p.Scope.TenantID(), p.ConnectionID, current.SecretID, p.Now); err != nil {
                        return connectorDomain.Connection{}, false, err
                }
        }
        version := current.Version + 1
        nextStatus := connectorDomain.StatusFailed
        if p.Check.Authentication == connectorDomain.CheckVerified && p.Check.Transport == connectorDomain.CheckVerified {
                nextStatus = connectorDomain.StatusConnectedUnverified
        }
        if !connectorDomain.ValidTransition(current.Status, nextStatus) {
                return connectorDomain.Connection{}, false, connectorDomain.ErrConflict
        }
        secretID := current.SecretID
        hint := current.SecretHint
        secretActive := current.SecretActive
        if p.Seal {
                if _, err = tx.Exec(ctx, "insert into connector.secret_seals(tenant_id,connection_id,secret_id,sealed,seal_key,created_at) values($1,$2,$3,$4,$5,$6)", p.Scope.TenantID(), p.ConnectionID, p.SecretID, p.Sealed, p.Key, p.Now); err != nil {
                        return connectorDomain.Connection{}, false, err
                }
                secretID = p.SecretID
                hint = p.Hint
                secretActive = true
        } else if p.Rotate {
                // The previous seal was revoked above and no new secret was
                // accepted (invalid credentials are never stored): no secret
                // stays active, and a plain connect retry stays honest.
                secretActive = false
        }
        var secretIDArg any
        if secretID != "" {
                secretIDArg = secretID
        }
        if _, err = tx.Exec(ctx, "update connector.connections set status=$3,authentication_state=$4,transport_state=$5,secret_id=$6,secret_hint=$7,secret_active=$8,version=$9,updated_at=$10 where tenant_id=$1 and connection_id=$2", p.Scope.TenantID(), p.ConnectionID, nextStatus, p.Check.Authentication, p.Check.Transport, secretIDArg, hint, secretActive, version, p.Now); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        outcome := "checked"
        eventType := "connector.connection_checked"
        summary := checkSummary("", p.Check)
        if p.Rotate {
                outcome = "rotated"
                eventType = "connector.connection_rotated"
                summary = checkSummary("secret rotated; ", p.Check)
        }
        if _, err = tx.Exec(ctx, "insert into connector.connection_versions(tenant_id,connection_id,version,connector_id,connector_version,label,environment,status,authentication_state,transport_state,config,secret_id,secret_hint,secret_active,actor_id,change_summary,changed_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)", p.Scope.TenantID(), p.ConnectionID, version, current.ConnectorID, current.ConnectorVer, current.Label, current.Environment, nextStatus, p.Check.Authentication, p.Check.Transport, mustJSON(current.Config), uuidArg(secretID), hint, secretActive, p.Scope.PrincipalID(), summary, p.Now); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9)", p.Scope.TenantID(), p.AuditID, p.ConnectionID, p.Scope.PrincipalID(), p.ExpectedVersion, version, outcome, p.CorrelationID, p.Now); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        payloadFields := map[string]any{"connection_id": p.ConnectionID, "version": version, "authentication_state": p.Check.Authentication, "transport_state": p.Check.Transport, "status": nextStatus}
        // Reference a secret only when a seal row actually exists: a failed
        // check must not advertise an orphan id that nothing backs.
        if secretID != "" {
                payloadFields["secret_id"] = secretID
                payloadFields["secret_hint"] = hint
        }
        payload, _ := json.Marshal(payloadFields)
        if _, err = tx.Exec(ctx, "insert into connector.outbox_events(tenant_id,event_id,connection_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,$4,1,$5,$6,$7,$8,$9)", p.Scope.TenantID(), p.EventID, p.ConnectionID, eventType, version, p.Scope.PrincipalID(), p.CorrelationID, p.Now, payload); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if p.IdempotencyKey != "" {
                if _, err = tx.Exec(ctx, "insert into connector.idempotency_records(tenant_id,actor_id,operation,idempotency_key,request_hash,connection_id,created_at,resulting_version) values($1,$2,$3,$4,$5,$6,$7,$8)", p.Scope.TenantID(), p.Scope.PrincipalID(), p.Operation, p.IdempotencyKey, p.RequestHash, p.ConnectionID, p.Now, version); err != nil {
                        return connectorDomain.Connection{}, false, err
                }
        }
        c, e := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2", p.Scope.TenantID(), p.ConnectionID))
        if e != nil {
                return connectorDomain.Connection{}, false, e
        }
        return c, false, tx.Commit(ctx)
}

// Revoke is terminal: the seal is revoked and the status frozen; history
// stays. Like connect/rotate the command is idempotent by key.
func (s *Postgres) Revoke(ctx context.Context, p port.RevokeParams) (connectorDomain.Connection, bool, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return connectorDomain.Connection{}, false, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        if p.IdempotencyKey != "" {
                replayed, conn, err := s.replayIfRecorded(ctx, tx, p.Scope, p.Operation, p.IdempotencyKey, p.RequestHash, p.ConnectionID, p.ExpectedVersion, p.AuditID, p.CorrelationID, p.Now)
                if err != nil || replayed {
                        return conn, replayed, err
                }
        }
        current, err := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2 for update", p.Scope.TenantID(), p.ConnectionID))
        if err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if current.Version != p.ExpectedVersion {
                if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'conflict',$7,$8)", p.Scope.TenantID(), p.AuditID, p.ConnectionID, p.Scope.PrincipalID(), p.ExpectedVersion, current.Version, p.CorrelationID, p.Now); err != nil {
                        return connectorDomain.Connection{}, false, err
                }
                if err = tx.Commit(ctx); err != nil {
                        return connectorDomain.Connection{}, false, err
                }
                return connectorDomain.Connection{}, false, connectorDomain.ErrConflict
        }
        version := current.Version + 1
        if _, err = tx.Exec(ctx, "update connector.secret_seals set revoked_at=$3 where tenant_id=$1 and connection_id=$2 and revoked_at is null", p.Scope.TenantID(), p.ConnectionID, p.Now); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if _, err = tx.Exec(ctx, "update connector.connections set status='revoked',secret_active=false,version=$3,updated_at=$4 where tenant_id=$1 and connection_id=$2", p.Scope.TenantID(), p.ConnectionID, version, p.Now); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.connection_versions(tenant_id,connection_id,version,connector_id,connector_version,label,environment,status,authentication_state,transport_state,config,secret_id,secret_hint,secret_active,actor_id,change_summary,changed_at) values($1,$2,$3,$4,$5,$6,$7,'revoked',$8,$9,$10,$11,$12,false,$13,$14,$15)", p.Scope.TenantID(), p.ConnectionID, version, current.ConnectorID, current.ConnectorVer, current.Label, current.Environment, current.AuthState, current.Transport, mustJSON(current.Config), uuidArg(current.SecretID), current.SecretHint, p.Scope.PrincipalID(), "revoked: "+p.Reason, p.Now); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'revoked',$7,$8)", p.Scope.TenantID(), p.AuditID, p.ConnectionID, p.Scope.PrincipalID(), p.ExpectedVersion, version, p.CorrelationID, p.Now); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        payload, _ := json.Marshal(map[string]any{"connection_id": p.ConnectionID, "version": version, "reason": p.Reason})
        if _, err = tx.Exec(ctx, "insert into connector.outbox_events(tenant_id,event_id,connection_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,'connector.connection_revoked',1,$4,$5,$6,$7,$8)", p.Scope.TenantID(), p.EventID, p.ConnectionID, version, p.Scope.PrincipalID(), p.CorrelationID, p.Now, payload); err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if p.IdempotencyKey != "" {
                if _, err = tx.Exec(ctx, "insert into connector.idempotency_records(tenant_id,actor_id,operation,idempotency_key,request_hash,connection_id,created_at,resulting_version) values($1,$2,$3,$4,$5,$6,$7,$8)", p.Scope.TenantID(), p.Scope.PrincipalID(), p.Operation, p.IdempotencyKey, p.RequestHash, p.ConnectionID, p.Now, version); err != nil {
                        return connectorDomain.Connection{}, false, err
                }
        }
        c, e := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2", p.Scope.TenantID(), p.ConnectionID))
        if e != nil {
                return connectorDomain.Connection{}, false, e
        }
        return c, false, tx.Commit(ctx)
}


// LookupCommand answers the idempotency question without side effects: a
// recorded (operation, key) with the same hash replays the stored version
// snapshot; a different hash is a conflict; no record means "execute".
func (s *Postgres) LookupCommand(ctx context.Context, scope tenancy.Context, operation, key, hash string) (connectorDomain.Connection, bool, error) {
        if !scope.Valid() || key == "" {
                return connectorDomain.Connection{}, false, nil
        }
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return connectorDomain.Connection{}, false, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        var oldHash string
        var oldVersion *int64
        err = tx.QueryRow(ctx, "select request_hash,resulting_version from connector.idempotency_records where tenant_id=$1 and actor_id=$2 and operation=$3 and idempotency_key=$4", scope.TenantID(), scope.PrincipalID(), operation, key).Scan(&oldHash, &oldVersion)
        if errors.Is(err, pgx.ErrNoRows) {
                return connectorDomain.Connection{}, false, tx.Commit(ctx)
        }
        if err != nil {
                return connectorDomain.Connection{}, false, err
        }
        if oldHash != hash {
                return connectorDomain.Connection{}, false, connectorDomain.ErrIdempotencyConflict
        }
        if oldVersion != nil && *oldVersion >= 1 {
                var connectionID id.UUID
                if err = tx.QueryRow(ctx, "select connection_id from connector.idempotency_records where tenant_id=$1 and actor_id=$2 and operation=$3 and idempotency_key=$4", scope.TenantID(), scope.PrincipalID(), operation, key).Scan(&connectionID); err != nil {
                        return connectorDomain.Connection{}, false, err
                }
                snapshot, err := readVersionSnapshot(ctx, tx, scope.TenantID(), connectionID, *oldVersion)
                if err != nil {
                        return connectorDomain.Connection{}, false, err
                }
                return snapshot, true, tx.Commit(ctx)
        }
        return connectorDomain.Connection{}, false, tx.Commit(ctx)
}

// LookupEvidence answers the verify-command idempotency question without side
// effects: a stored (connection, key) with the same hash replays the evidence;
// a different hash is a conflict; nothing stored means "execute".
func (s *Postgres) LookupEvidence(ctx context.Context, scope tenancy.Context, connectionID id.UUID, key, hash string) (connectorDomain.CapabilityEvidence, bool, error) {
        if !scope.Valid() || key == "" {
                return connectorDomain.CapabilityEvidence{}, false, nil
        }
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return connectorDomain.CapabilityEvidence{}, false, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        existing, found, err := scanEvidenceOptional(tx.QueryRow(ctx, "select "+evidenceColumns+" from connector.capability_evidence where tenant_id=$1 and connection_id=$2 and idempotency_key=$3", scope.TenantID(), connectionID, key))
        if err != nil || !found {
                return connectorDomain.CapabilityEvidence{}, false, err
        }
        if existing.RequestHash != hash {
                return connectorDomain.CapabilityEvidence{}, false, connectorDomain.ErrIdempotencyConflict
        }
        return existing, true, tx.Commit(ctx)
}

// uuidArg converts an optional UUID into a nullable SQL argument.
func uuidArg(value id.UUID) any {
        if value == "" {
                return nil
        }
        return value
}

// replayIfRecorded implements the idempotent replay path of connect/rotate/
// revoke (R2 Gap 5). Under the per-key advisory lock it looks up the
// idempotency record of the operation: a matching request hash replays the
// immutable version snapshot that the command produced (never a false
// "changed by someone else"); a different hash is an honest conflict.
// Returns replayed=true when the caller must return the snapshot as-is.
func (s *Postgres) replayIfRecorded(ctx context.Context, tx pgx.Tx, scope tenancy.Context, operation, key, hash string, connectionID id.UUID, expectedVersion int64, auditID, correlationID id.UUID, now time.Time) (bool, connectorDomain.Connection, error) {
        lock := scope.TenantID().String() + ":" + scope.PrincipalID().String() + ":" + operation + ":" + key
        if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", lock); err != nil {
                return false, connectorDomain.Connection{}, err
        }
        var oldHash string
        var oldVersion *int64
        err := tx.QueryRow(ctx, "select request_hash,resulting_version from connector.idempotency_records where tenant_id=$1 and actor_id=$2 and operation=$3 and idempotency_key=$4", scope.TenantID(), scope.PrincipalID(), operation, key).Scan(&oldHash, &oldVersion)
        if errors.Is(err, pgx.ErrNoRows) {
                return false, connectorDomain.Connection{}, nil
        }
        if err != nil {
                return false, connectorDomain.Connection{}, err
        }
        if oldHash != hash {
                var currentVersion int64
                if err = tx.QueryRow(ctx, "select version from connector.connections where tenant_id=$1 and connection_id=$2", scope.TenantID(), connectionID).Scan(&currentVersion); err != nil {
                        return false, connectorDomain.Connection{}, err
                }
                if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'idempotency_conflict',$7,$8)", scope.TenantID(), auditID, connectionID, scope.PrincipalID(), expectedVersion, currentVersion, correlationID, now); err != nil {
                        return false, connectorDomain.Connection{}, err
                }
                if err = tx.Commit(ctx); err != nil {
                        return false, connectorDomain.Connection{}, err
                }
                return true, connectorDomain.Connection{}, connectorDomain.ErrIdempotencyConflict
        }
        if oldVersion != nil && *oldVersion >= 1 {
                snapshot, err := readVersionSnapshot(ctx, tx, scope.TenantID(), connectionID, *oldVersion)
                if err != nil {
                        return false, connectorDomain.Connection{}, err
                }
                return true, snapshot, tx.Commit(ctx)
        }
        // Legacy record without a resulting version: replay the current state.
        current, err := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2", scope.TenantID(), connectionID))
        if err != nil {
                return false, connectorDomain.Connection{}, err
        }
        return true, current, tx.Commit(ctx)
}

// readVersionSnapshot loads one immutable history version as a Connection.
func readVersionSnapshot(ctx context.Context, tx pgx.Tx, tenantID, connectionID id.UUID, version int64) (connectorDomain.Connection, error) {
        row := tx.QueryRow(ctx, "select v.tenant_id,v.connection_id,v.connector_id,v.connector_version,v.label,v.environment,v.status,v.authentication_state,v.transport_state,v.config,v.secret_id,v.secret_hint,v.secret_active,(select c2.owner_id from connector.connections c2 where c2.tenant_id=v.tenant_id and c2.connection_id=v.connection_id),v.version,v.changed_at,v.changed_at from connector.connection_versions v where v.tenant_id=$1 and v.connection_id=$2 and v.version=$3", tenantID, connectionID, version)
        return scanConnection(row)
}

// SaveEvidence appends one immutable capability evidence row. Idempotent by
// key: the same (connection, key) with the same request hash replays the
// stored row; a different hash is rejected as a conflict. Contradicting
// evidence for one key can therefore never exist.
func (s *Postgres) SaveEvidence(ctx context.Context, p port.SaveEvidenceParams) (connectorDomain.CapabilityEvidence, bool, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return connectorDomain.CapabilityEvidence{}, false, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        lock := p.Scope.TenantID().String() + ":" + p.Scope.PrincipalID().String() + ":connector.capability.verify.v1:" + p.IdempotencyKey
        if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", lock); err != nil {
                return connectorDomain.CapabilityEvidence{}, false, err
        }
        existing, found, err := scanEvidenceOptional(tx.QueryRow(ctx, "select "+evidenceColumns+" from connector.capability_evidence where tenant_id=$1 and connection_id=$2 and idempotency_key=$3", p.Scope.TenantID(), p.ConnectionID, p.IdempotencyKey))
        if err != nil {
                return connectorDomain.CapabilityEvidence{}, false, err
        }
        if found {
                if existing.RequestHash != p.RequestHash {
                        if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,0,null,'capability_conflict',$5,$6)", p.Scope.TenantID(), p.AuditID, p.ConnectionID, p.Scope.PrincipalID(), p.CorrelationID, p.CompletedAt); err != nil {
                                return connectorDomain.CapabilityEvidence{}, false, err
                        }
                        if err = tx.Commit(ctx); err != nil {
                                return connectorDomain.CapabilityEvidence{}, false, err
                        }
                        return connectorDomain.CapabilityEvidence{}, true, connectorDomain.ErrIdempotencyConflict
                }
                return existing, true, tx.Commit(ctx)
        }
        if _, err = tx.Exec(ctx, "insert into connector.capability_evidence("+evidenceColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)", p.Scope.TenantID(), p.EvidenceID, p.ConnectionID, p.SecretID, p.Environment, p.ContractVersion, p.AdapterBuild, p.Capability, p.ProbeKind, p.Scope.PrincipalID(), p.RequestedAt, p.DeadlineAt, p.CompletedAt, p.Outcome, p.DeliveryLevel, p.RedactedRequest, p.RedactedResult, p.ExpiresAt, p.IdempotencyKey, p.RequestHash); err != nil {
                return connectorDomain.CapabilityEvidence{}, false, err
        }
        auditOutcome := "capability_" + p.Outcome
        if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,0,null,$5,$6,$7)", p.Scope.TenantID(), p.AuditID, p.ConnectionID, p.Scope.PrincipalID(), auditOutcome, p.CorrelationID, p.CompletedAt); err != nil {
                return connectorDomain.CapabilityEvidence{}, false, err
        }
        payload, _ := json.Marshal(map[string]any{"connection_id": p.ConnectionID, "capability": p.Capability, "outcome": p.Outcome, "delivery_level": p.DeliveryLevel, "evidence_id": p.EvidenceID, "expires_at": p.ExpiresAt})
        if _, err = tx.Exec(ctx, "insert into connector.outbox_events(tenant_id,event_id,connection_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,'connector.capability_checked',1,0,$4,$5,$6,$7)", p.Scope.TenantID(), p.EventID, p.ConnectionID, p.Scope.PrincipalID(), p.CorrelationID, p.CompletedAt, payload); err != nil {
                return connectorDomain.CapabilityEvidence{}, false, err
        }
        saved, _, err := scanEvidenceOptional(tx.QueryRow(ctx, "select "+evidenceColumns+" from connector.capability_evidence where tenant_id=$1 and evidence_id=$2", p.Scope.TenantID(), p.EvidenceID))
        if err != nil {
                return connectorDomain.CapabilityEvidence{}, false, err
        }
        return saved, false, tx.Commit(ctx)
}

// ListEvidence returns every evidence row of one connection (newest first per
// capability) so the domain can derive states without a second writer.
func (s *Postgres) ListEvidence(ctx context.Context, scope tenancy.Context, connectionID id.UUID) ([]connectorDomain.CapabilityEvidence, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+evidenceColumns+" from connector.capability_evidence where tenant_id=$1 and connection_id=$2 order by capability, completed_at desc, evidence_id desc", scope.TenantID(), connectionID)
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []connectorDomain.CapabilityEvidence{}
        for rows.Next() {
                e, err := scanEvidenceRow(rows)
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

// ReadSeal reads the ACTIVE seal envelope of one connection secret (the
// narrow read of the MAC verification, Story 7-4): exactly the existing
// columns of the existing table — 0 migrations. The read runs inside the
// scoped transaction (the RLS tenant context of the platform), revoked
// seals never match and a missing row is the honest not-found.
func (s *Postgres) ReadSeal(ctx context.Context, scope tenancy.Context, connectionID, secretID id.UUID) (connectorDomain.Seal, bool, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return connectorDomain.Seal{}, false, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        var seal connectorDomain.Seal
        err = tx.QueryRow(ctx, "select sealed, seal_key, created_at from connector.secret_seals where tenant_id=$1 and connection_id=$2 and secret_id=$3 and revoked_at is null", scope.TenantID(), connectionID, secretID).Scan(&seal.Sealed, &seal.Key, &seal.CreatedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return connectorDomain.Seal{}, false, nil
        }
        if err != nil {
                return connectorDomain.Seal{}, false, err
        }
        if err = tx.Commit(ctx); err != nil {
                return connectorDomain.Seal{}, false, err
        }
        seal.TenantID = scope.TenantID()
        seal.ConnectionID = connectionID
        seal.SecretID = secretID
        return seal, true, nil
}

type evidenceRow interface {
        Scan(dest ...any) error
}

func scanEvidenceOptional(row evidenceRow) (connectorDomain.CapabilityEvidence, bool, error) {
        var e connectorDomain.CapabilityEvidence
        err := row.Scan(&e.TenantID, &e.EvidenceID, &e.ConnectionID, &e.SecretID, &e.Environment, &e.ContractVersion, &e.AdapterBuild, &e.Capability, &e.ProbeKind, &e.ActorID, &e.RequestedAt, &e.DeadlineAt, &e.CompletedAt, &e.Outcome, &e.DeliveryLevel, &e.RedactedRequest, &e.RedactedResult, &e.ExpiresAt, &e.IdempotencyKey, &e.RequestHash)
        if errors.Is(err, pgx.ErrNoRows) {
                return connectorDomain.CapabilityEvidence{}, false, nil
        }
        if err != nil {
                return connectorDomain.CapabilityEvidence{}, false, err
        }
        return e, true, nil
}

func scanEvidenceRow(row evidenceRow) (connectorDomain.CapabilityEvidence, error) {
        e, _, err := scanEvidenceOptional(row)
        return e, err
}


// checkSummary renders the redacted basic-check outcome for the immutable
// version history. The verifier reasons are fixed redacted strings, never the
// submitted values; the combined text is capped for the 300-char CHECK.
func checkSummary(prefix string, check connectorDomain.CheckResult) string {
        var b strings.Builder
        b.WriteString(prefix)
        b.WriteString("basic check: authentication=")
        b.WriteString(check.Authentication)
        if check.Authentication == connectorDomain.CheckFailed && check.AuthReason != "" {
                b.WriteString(" (")
                b.WriteString(check.AuthReason)
                b.WriteString(")")
        }
        b.WriteString(", transport=")
        b.WriteString(check.Transport)
        if check.Transport == connectorDomain.CheckFailed && check.TransportReason != "" {
                b.WriteString(" (")
                b.WriteString(check.TransportReason)
                b.WriteString(")")
        }
        summary := b.String()
        if runes := []rune(summary); len(runes) > 300 {
                return string(runes[:300])
        }
        return summary
}

func mustJSON(m map[string]string) []byte {
        if m == nil {
                return []byte("{}")
        }
        out, err := json.Marshal(m)
        if err != nil {
                return []byte("{}")
        }
        return out
}
