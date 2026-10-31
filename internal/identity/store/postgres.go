package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/identity/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const createTenantOperation = "identity.create_tenant.v1"

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

type idempotentResponse struct {
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
	Created  string `json:"created_at"`
}

func (s *Postgres) ReplayTenant(ctx context.Context, principal domain.Principal, key, requestHash string) (domain.Tenant, bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.Tenant{}, false, fmt.Errorf("begin idempotency read: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := setLocal(ctx, tx, "staffy.principal_id", principal.ID.String()); err != nil {
		return domain.Tenant{}, false, err
	}
	var storedHash string
	var storedResponse []byte
	err = tx.QueryRow(ctx, `
		select request_hash, response_body
		from identity.idempotency_records
		where principal_id = $1 and operation = $2 and idempotency_key = $3`,
		principal.ID.String(), createTenantOperation, key,
	).Scan(&storedHash, &storedResponse)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Tenant{}, false, nil
	}
	if err != nil {
		return domain.Tenant{}, false, fmt.Errorf("read idempotency replay: %w", err)
	}
	if storedHash != requestHash {
		return domain.Tenant{}, false, domain.ErrIdempotencyConflict
	}
	tenant, err := decodeTenant(storedResponse)
	if err != nil {
		return domain.Tenant{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Tenant{}, false, fmt.Errorf("commit idempotency read: %w", err)
	}
	return tenant, true, nil
}

func (s *Postgres) CreateTenant(ctx context.Context, params port.CreateTenantParams) (domain.Tenant, bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Tenant{}, false, fmt.Errorf("begin create tenant: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := setLocal(ctx, tx, "staffy.principal_id", params.Principal.ID.String()); err != nil {
		return domain.Tenant{}, false, err
	}
	lockKey := fmt.Sprintf("%s:%d:%s:%d:%s", params.Principal.ID, len(createTenantOperation), createTenantOperation, len(params.IdempotencyKey), params.IdempotencyKey)
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1, 0))", lockKey); err != nil {
		return domain.Tenant{}, false, fmt.Errorf("lock idempotency key: %w", err)
	}

	var storedHash string
	var storedResponse []byte
	err = tx.QueryRow(ctx, `
		select request_hash, response_body
		from identity.idempotency_records
		where principal_id = $1 and operation = $2 and idempotency_key = $3`,
		params.Principal.ID.String(), createTenantOperation, params.IdempotencyKey,
	).Scan(&storedHash, &storedResponse)
	if err == nil {
		if storedHash != params.RequestHash {
			return domain.Tenant{}, false, domain.ErrIdempotencyConflict
		}
		tenant, decodeErr := decodeTenant(storedResponse)
		if decodeErr != nil {
			return domain.Tenant{}, false, decodeErr
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.Tenant{}, false, fmt.Errorf("commit idempotent replay: %w", err)
		}
		return tenant, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Tenant{}, false, fmt.Errorf("read idempotency record: %w", err)
	}

	if err := setLocal(ctx, tx, "staffy.tenant_id", params.TenantID.String()); err != nil {
		return domain.Tenant{}, false, err
	}
	if _, err := tx.Exec(ctx, `
		insert into identity.principals (principal_id, created_at)
		values ($1, $2) on conflict (principal_id) do nothing`, params.Principal.ID.String(), params.OccurredAt); err != nil {
		return domain.Tenant{}, false, fmt.Errorf("ensure principal: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into identity.tenants (tenant_id, name, created_at, created_by)
		values ($1, $2, $3, $4)`, params.TenantID.String(), params.Name, params.OccurredAt, params.Principal.ID.String()); err != nil {
		return domain.Tenant{}, false, fmt.Errorf("insert tenant: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into identity.memberships (tenant_id, principal_id, role, status, created_at)
		values ($1, $2, 'owner', 'active', $3)`, params.TenantID.String(), params.Principal.ID.String(), params.OccurredAt); err != nil {
		return domain.Tenant{}, false, fmt.Errorf("insert owner membership: %w", err)
	}
	payload, err := json.Marshal(map[string]string{
		"tenant_id": params.TenantID.String(), "actor_id": params.Principal.ID.String(), "role": "owner",
	})
	if err != nil {
		return domain.Tenant{}, false, fmt.Errorf("encode tenant event: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into identity.outbox_events
		(tenant_id, event_id, event_type, schema_version, aggregate_id, actor_id, correlation_id, occurred_at, payload)
		values ($1, $2, 'identity.tenant_created', 1, $1, $3, $4, $5, $6)`,
		params.TenantID.String(), params.EventID.String(), params.Principal.ID.String(), params.CorrelationID.String(), params.OccurredAt, payload); err != nil {
		return domain.Tenant{}, false, fmt.Errorf("insert tenant event: %w", err)
	}
	response, err := json.Marshal(idempotentResponse{
		TenantID: params.TenantID.String(), Name: params.Name, Created: params.OccurredAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
	})
	if err != nil {
		return domain.Tenant{}, false, fmt.Errorf("encode idempotency response: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into identity.idempotency_records
		(principal_id, operation, idempotency_key, request_hash, tenant_id, response_body, created_at)
		values ($1, $2, $3, $4, $5, $6, $7)`, params.Principal.ID.String(), createTenantOperation,
		params.IdempotencyKey, params.RequestHash, params.TenantID.String(), response, params.OccurredAt); err != nil {
		return domain.Tenant{}, false, fmt.Errorf("insert idempotency record: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Tenant{}, false, fmt.Errorf("commit create tenant: %w", err)
	}
	return domain.Tenant{ID: params.TenantID, Name: params.Name, CreatedAt: params.OccurredAt}, false, nil
}

// TenantName reads the display name of one tenant under the anonymous
// scope (the public continuation page header): the plain tenants read —
// the RLS tenant policy checks staffy.tenant_id only, no principal; a
// missing row is the honest ErrNotFound.
func (s *Postgres) TenantName(ctx context.Context, scope tenancy.Context) (string, error) {
	if !scope.Valid() {
		return "", tenancy.ErrMissingContext
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return "", fmt.Errorf("begin tenant name read: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := setLocal(ctx, tx, "staffy.principal_id", scope.PrincipalID().String()); err != nil {
		return "", err
	}
	if err := setLocal(ctx, tx, "staffy.tenant_id", scope.TenantID().String()); err != nil {
		return "", err
	}
	var name string
	err = tx.QueryRow(ctx, `select t.name from identity.tenants t where t.tenant_id = $1`,
		scope.TenantID().String(),
	).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read tenant name: %w", err)
	}
	return name, nil
}

func (s *Postgres) Workspace(ctx context.Context, scope tenancy.Context) (domain.Workspace, error) {
	if !scope.Valid() {
		return domain.Workspace{}, tenancy.ErrMissingContext
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("begin workspace read: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := setLocal(ctx, tx, "staffy.principal_id", scope.PrincipalID().String()); err != nil {
		return domain.Workspace{}, err
	}
	if err := setLocal(ctx, tx, "staffy.tenant_id", scope.TenantID().String()); err != nil {
		return domain.Workspace{}, err
	}
	var tenant domain.Tenant
	var role string
	err = tx.QueryRow(ctx, `
		select t.tenant_id, t.name, t.created_at, m.role
		from identity.tenants t
		join identity.memberships m on m.tenant_id = t.tenant_id
		where t.tenant_id = $1 and m.principal_id = $2 and m.status = 'active'`,
		scope.TenantID().String(), scope.PrincipalID().String(),
	).Scan(&tenant.ID, &tenant.Name, &tenant.CreatedAt, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Workspace{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("read workspace: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Workspace{}, fmt.Errorf("commit workspace read: %w", err)
	}
	return domain.Workspace{Tenant: tenant, Role: role}, nil
}

func setLocal(ctx context.Context, tx pgx.Tx, name, value string) error {
	if _, err := tx.Exec(ctx, "select set_config($1, $2, true)", name, value); err != nil {
		return fmt.Errorf("set transaction scope %s: %w", name, err)
	}
	return nil
}

func decodeTenant(data []byte) (domain.Tenant, error) {
	var response idempotentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return domain.Tenant{}, fmt.Errorf("decode idempotency response: %w", err)
	}
	tenantID, err := id.Parse(response.TenantID)
	if err != nil {
		return domain.Tenant{}, fmt.Errorf("decode tenant id: %w", err)
	}
	createdAt, err := time.Parse("2006-01-02T15:04:05.999999999Z07:00", response.Created)
	if err != nil {
		return domain.Tenant{}, fmt.Errorf("decode tenant creation time: %w", err)
	}
	return domain.Tenant{ID: tenantID, Name: response.Name, CreatedAt: createdAt}, nil
}
