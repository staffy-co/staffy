package store

import (
        "context"
        "encoding/json"
        "errors"
        "fmt"
        "strings"

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
func (s *Postgres) Connect(ctx context.Context, p port.ConnectParams) (connectorDomain.Connection, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        current, err := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2 for update", p.Scope.TenantID(), p.ConnectionID))
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
        if p.Rotate && current.SecretActive {
                if _, err = tx.Exec(ctx, "update connector.secret_seals set revoked_at=$4 where tenant_id=$1 and connection_id=$2 and secret_id=$3 and revoked_at is null", p.Scope.TenantID(), p.ConnectionID, current.SecretID, p.Now); err != nil {
                        return connectorDomain.Connection{}, err
                }
        }
        version := current.Version + 1
        nextStatus := connectorDomain.StatusFailed
        if p.Check.Authentication == connectorDomain.CheckVerified && p.Check.Transport == connectorDomain.CheckVerified {
                nextStatus = connectorDomain.StatusConnectedUnverified
        }
        if !connectorDomain.ValidTransition(current.Status, nextStatus) {
                return connectorDomain.Connection{}, connectorDomain.ErrConflict
        }
        secretID := current.SecretID
        hint := current.SecretHint
        secretActive := current.SecretActive
        if p.Seal {
                if _, err = tx.Exec(ctx, "insert into connector.secret_seals(tenant_id,connection_id,secret_id,sealed,seal_key,created_at) values($1,$2,$3,$4,$5,$6)", p.Scope.TenantID(), p.ConnectionID, p.SecretID, p.Sealed, p.Key, p.Now); err != nil {
                        return connectorDomain.Connection{}, err
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
                return connectorDomain.Connection{}, err
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
                return connectorDomain.Connection{}, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9)", p.Scope.TenantID(), p.AuditID, p.ConnectionID, p.Scope.PrincipalID(), p.ExpectedVersion, version, outcome, p.CorrelationID, p.Now); err != nil {
                return connectorDomain.Connection{}, err
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
                return connectorDomain.Connection{}, err
        }
        c, e := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2", p.Scope.TenantID(), p.ConnectionID))
        if e != nil {
                return connectorDomain.Connection{}, e
        }
        return c, tx.Commit(ctx)
}

// Revoke is terminal: the seal is revoked and the status frozen; history stays.
func (s *Postgres) Revoke(ctx context.Context, p port.RevokeParams) (connectorDomain.Connection, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return connectorDomain.Connection{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        current, err := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2 for update", p.Scope.TenantID(), p.ConnectionID))
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
        version := current.Version + 1
        if _, err = tx.Exec(ctx, "update connector.secret_seals set revoked_at=$3 where tenant_id=$1 and connection_id=$2 and revoked_at is null", p.Scope.TenantID(), p.ConnectionID, p.Now); err != nil {
                return connectorDomain.Connection{}, err
        }
        if _, err = tx.Exec(ctx, "update connector.connections set status='revoked',secret_active=false,version=$3,updated_at=$4 where tenant_id=$1 and connection_id=$2", p.Scope.TenantID(), p.ConnectionID, version, p.Now); err != nil {
                return connectorDomain.Connection{}, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.connection_versions(tenant_id,connection_id,version,connector_id,connector_version,label,environment,status,authentication_state,transport_state,config,secret_id,secret_hint,secret_active,actor_id,change_summary,changed_at) values($1,$2,$3,$4,$5,$6,$7,'revoked',$8,$9,$10,$11,$12,false,$13,$14,$15)", p.Scope.TenantID(), p.ConnectionID, version, current.ConnectorID, current.ConnectorVer, current.Label, current.Environment, current.AuthState, current.Transport, mustJSON(current.Config), uuidArg(current.SecretID), current.SecretHint, p.Scope.PrincipalID(), "revoked: "+p.Reason, p.Now); err != nil {
                return connectorDomain.Connection{}, err
        }
        if _, err = tx.Exec(ctx, "insert into connector.audit(tenant_id,audit_id,connection_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'revoked',$7,$8)", p.Scope.TenantID(), p.AuditID, p.ConnectionID, p.Scope.PrincipalID(), p.ExpectedVersion, version, p.CorrelationID, p.Now); err != nil {
                return connectorDomain.Connection{}, err
        }
        payload, _ := json.Marshal(map[string]any{"connection_id": p.ConnectionID, "version": version, "reason": p.Reason})
        if _, err = tx.Exec(ctx, "insert into connector.outbox_events(tenant_id,event_id,connection_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,'connector.connection_revoked',1,$4,$5,$6,$7,$8)", p.Scope.TenantID(), p.EventID, p.ConnectionID, version, p.Scope.PrincipalID(), p.CorrelationID, p.Now, payload); err != nil {
                return connectorDomain.Connection{}, err
        }
        c, e := scanConnection(tx.QueryRow(ctx, "select "+connectionColumns+" from connector.connections where tenant_id=$1 and connection_id=$2", p.Scope.TenantID(), p.ConnectionID))
        if e != nil {
                return connectorDomain.Connection{}, e
        }
        return c, tx.Commit(ctx)
}


// uuidArg converts an optional UUID into a nullable SQL argument.
func uuidArg(value id.UUID) any {
        if value == "" {
                return nil
        }
        return value
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
