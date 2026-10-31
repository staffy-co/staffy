package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/identity/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

func scopedTx(ctx context.Context, tx pgx.Tx, actor, tenant id.UUID) error {
	if !actor.Valid() || !tenant.Valid() {
		return tenancy.ErrMissingContext
	}
	if err := setLocal(ctx, tx, "staffy.principal_id", actor.String()); err != nil {
		return err
	}
	return setLocal(ctx, tx, "staffy.tenant_id", tenant.String())
}

func isOwner(ctx context.Context, tx pgx.Tx, actor, tenant id.UUID) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `select exists(select 1 from identity.memberships where tenant_id=$1 and principal_id=$2 and status='active' and 'owner'=any(roles))`, tenant.String(), actor.String()).Scan(&ok)
	return ok, err
}

func ownerLock(ctx context.Context, tx pgx.Tx, actor, tenant id.UUID) error {
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1, 0))`, "membership:"+tenant.String()); err != nil {
		return err
	}
	var t string
	if err := tx.QueryRow(ctx, `select tenant_id from identity.tenants where tenant_id=$1`, tenant.String()).Scan(&t); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	}
	ok, err := isOwner(ctx, tx, actor, tenant)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrNotFound
	}
	return nil
}

func roleStrings(roles []domain.Role) []string {
	out := make([]string, len(roles))
	for i, v := range roles {
		out[i] = string(v)
	}
	return out
}
func scopeStrings(scopes []domain.Scope) []string {
	out := make([]string, len(scopes))
	for i, v := range scopes {
		out[i] = string(v)
	}
	return out
}
func rolesFrom(in []string) []domain.Role {
	out := make([]domain.Role, len(in))
	for i, v := range in {
		out[i] = domain.Role(v)
	}
	return out
}
func scopesFrom(in []string) []domain.Scope {
	out := make([]domain.Scope, len(in))
	for i, v := range in {
		out[i] = domain.Scope(v)
	}
	return out
}

func auditChange(ctx context.Context, tx pgx.Tx, tenant, actor id.UUID, subject *id.UUID, reason domain.ReasonCode, correlation id.UUID, before, after any, now time.Time) error {
	auditID, err := id.NewV7(now)
	if err != nil {
		return err
	}
	oldJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	newJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `insert into identity.membership_audit (tenant_id,audit_id,actor_id,subject_id,old_state,new_state,reason,correlation_id,occurred_at) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, tenant.String(), auditID.String(), actor.String(), subject, oldJSON, newJSON, string(reason), correlation.String(), now)
	return err
}

// MembersFacts is the caller-authority projection: active memberships only,
// no owner gate — the caller surface has already verified the actor's scope
// (the fact-port precedent; the tenancy GUC still isolates the tenant).
func (s *Postgres) MembersFacts(ctx context.Context, scope tenancy.Context) ([]domain.Membership, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = scopedTx(ctx, tx, scope.PrincipalID(), scope.TenantID()); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `select principal_id,coalesce(email,''),roles,scopes,status,version from identity.memberships where tenant_id=$1 and status='active' order by 2`, scope.TenantID().String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Membership{}
	for rows.Next() {
		var principal *id.UUID
		var email string
		var roles, scopes []string
		var status string
		var version int64
		if err := rows.Scan(&principal, &email, &roles, &scopes, &status, &version); err != nil {
			return nil, err
		}
		m := domain.Membership{TenantID: scope.TenantID(), Email: email, Roles: rolesFrom(roles), Scopes: scopesFrom(scopes), Status: status, Version: version}
		if principal != nil {
			m.PrincipalID = principal
		}
		out = append(out, m)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func (s *Postgres) ListMembers(ctx context.Context, scope tenancy.Context) ([]domain.Membership, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = scopedTx(ctx, tx, scope.PrincipalID(), scope.TenantID()); err != nil {
		return nil, err
	}
	ok, err := isOwner(ctx, tx, scope.PrincipalID(), scope.TenantID())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrNotFound
	}
	rows, err := tx.Query(ctx, `select principal_id,coalesce(email,''),roles,scopes,status,version from identity.memberships where tenant_id=$1 union all select null::uuid,email,roles,scopes,case when expires_at<=now() then 'expired' else status end,version from identity.invitations where tenant_id=$1 and status='pending' order by 2`, scope.TenantID().String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.Membership{}
	for rows.Next() {
		var principal *string
		var email string
		var roles, scopes []string
		var status string
		var version int64
		if err := rows.Scan(&principal, &email, &roles, &scopes, &status, &version); err != nil {
			return nil, err
		}
		m := domain.Membership{TenantID: scope.TenantID(), Email: email, Roles: rolesFrom(roles), Scopes: scopesFrom(scopes), Status: status, Version: version}
		if principal != nil {
			p := id.UUID(*principal)
			m.PrincipalID = &p
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, tx.Commit(ctx)
}

func (s *Postgres) Authorize(ctx context.Context, scope tenancy.Context, required domain.Scope) error {
	if !scope.Valid() {
		return tenancy.ErrMissingContext
	}
	valid := false
	for _, s := range domain.AllScopes() {
		if s == required {
			valid = true
		}
	}
	if !valid {
		return domain.ErrForbidden
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = scopedTx(ctx, tx, scope.PrincipalID(), scope.TenantID()); err != nil {
		return err
	}
	var scopes []string
	err = tx.QueryRow(ctx, `select scopes from identity.memberships where tenant_id=$1 and principal_id=$2 and status='active'`, scope.TenantID().String(), scope.PrincipalID().String()).Scan(&scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	allowed := false
	for _, v := range scopes {
		if v == string(required) {
			allowed = true
		}
	}
	if !allowed {
		return domain.ErrForbidden
	}
	return tx.Commit(ctx)
}

func (s *Postgres) InviteMember(ctx context.Context, p port.MembershipMutation) (domain.Membership, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Membership{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = scopedTx(ctx, tx, p.Actor.ID, p.TenantID); err != nil {
		return domain.Membership{}, err
	}
	if err = ownerLock(ctx, tx, p.Actor.ID, p.TenantID); err != nil {
		return domain.Membership{}, err
	}
	var exists bool
	err = tx.QueryRow(ctx, `select exists(select 1 from identity.memberships where tenant_id=$1 and lower(email)=lower($2) and status='active')`, p.TenantID.String(), p.Email).Scan(&exists)
	if err != nil {
		return domain.Membership{}, err
	}
	if exists {
		return domain.Membership{}, domain.ErrConflict
	}
	var previous any
	var oldRoles, oldScopes []string
	var oldStatus string
	var oldVersion int64
	var oldInvitationID string
	err = tx.QueryRow(ctx, `select invitation_id,roles,scopes,status,version from identity.invitations where tenant_id=$1 and email=$2`, p.TenantID.String(), p.Email).Scan(&oldInvitationID, &oldRoles, &oldScopes, &oldStatus, &oldVersion)
	if err == nil {
		previous = map[string]any{"invitation_id": oldInvitationID, "status": oldStatus, "roles": oldRoles, "scopes": oldScopes, "version": oldVersion}
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, err
	}
	invID, err := id.NewV7(p.Now)
	if err != nil {
		return domain.Membership{}, err
	}
	var version int64
	var persistedInvitationID string
	err = tx.QueryRow(ctx, `insert into identity.invitations(tenant_id,invitation_id,email,token_digest,roles,scopes,status,expires_at,created_at,updated_at) values($1,$2,$3,$4,$5,$6,'pending',$7,$8,$8) on conflict(tenant_id,email) do update set token_digest=excluded.token_digest,roles=excluded.roles,scopes=excluded.scopes,status='pending',expires_at=excluded.expires_at,updated_at=excluded.updated_at,version=identity.invitations.version+1 returning invitation_id,version`, p.TenantID.String(), invID.String(), p.Email, p.TokenDigest, roleStrings(p.Roles), scopeStrings(p.Scopes), p.ExpiresAt, p.Now).Scan(&persistedInvitationID, &version)
	if err != nil {
		return domain.Membership{}, err
	}
	state := map[string]any{"invitation_id": persistedInvitationID, "status": "pending", "roles": p.Roles, "scopes": p.Scopes, "version": version}
	if err = auditChange(ctx, tx, p.TenantID, p.Actor.ID, nil, p.Reason, p.CorrelationID, previous, state, p.Now); err != nil {
		return domain.Membership{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Membership{}, err
	}
	return domain.Membership{TenantID: p.TenantID, Email: p.Email, Roles: p.Roles, Scopes: p.Scopes, Status: "pending", Version: version, ExpiresAt: &p.ExpiresAt}, nil
}

func (s *Postgres) AcceptInvitation(ctx context.Context, actor domain.Principal, digest string, correlation id.UUID, now time.Time) (domain.Membership, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Membership{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = setLocal(ctx, tx, "staffy.principal_id", actor.ID.String()); err != nil {
		return domain.Membership{}, err
	}
	if err = setLocal(ctx, tx, "staffy.invitation_digest", digest); err != nil {
		return domain.Membership{}, err
	}
	var tenant, invitation, email, status string
	var roles, scopes []string
	var expiry time.Time
	err = tx.QueryRow(ctx, `select tenant_id,invitation_id,email,roles,scopes,status,expires_at from identity.invitations where token_digest=$1`, digest).Scan(&tenant, &invitation, &email, &roles, &scopes, &status, &expiry)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Membership{}, err
	}
	if status != "pending" || !now.Before(expiry) || !strings.EqualFold(email, actor.VerifiedEmail) {
		return domain.Membership{}, domain.ErrNotFound
	}
	tenantID, err := id.Parse(tenant)
	if err != nil {
		return domain.Membership{}, err
	}
	if err = setLocal(ctx, tx, "staffy.tenant_id", tenant); err != nil {
		return domain.Membership{}, err
	}
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1, 0))`, "membership:"+tenant); err != nil {
		return domain.Membership{}, err
	}
	var locked string
	err = tx.QueryRow(ctx, `select tenant_id from identity.tenants where tenant_id=$1`, tenant).Scan(&locked)
	if err != nil {
		return domain.Membership{}, err
	}
	var changed int64
	err = tx.QueryRow(ctx, `update identity.invitations set status='accepted',updated_at=$3,version=version+1 where tenant_id=$1 and invitation_id=$2 and token_digest=$4 and status='pending' and expires_at>clock_timestamp() returning version`, tenant, invitation, now, digest).Scan(&changed)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Membership{}, err
	}
	_, err = tx.Exec(ctx, `insert into identity.principals(principal_id,created_at) values($1,$2) on conflict do nothing`, actor.ID.String(), now)
	if err != nil {
		return domain.Membership{}, err
	}
	var existing string
	var previousRoles, previousScopes []string
	var previousVersion int64
	err = tx.QueryRow(ctx, `select status,roles,scopes,version from identity.memberships where tenant_id=$1 and principal_id=$2 for update`, tenant, actor.ID.String()).Scan(&existing, &previousRoles, &previousScopes, &previousVersion)
	var membershipVersion int64
	previous := map[string]any{"invitation_id": invitation, "status": "pending", "roles": roles, "scopes": scopes, "version": changed - 1}
	if err == nil {
		if existing != "revoked" {
			return domain.Membership{}, domain.ErrConflict
		}
		previous = map[string]any{"invitation_id": invitation, "status": "revoked", "roles": previousRoles, "scopes": previousScopes, "version": previousVersion}
		membershipVersion = previousVersion + 1
		_, err = tx.Exec(ctx, `update identity.memberships set email=$3,role=$4,roles=$5,scopes=$6,status='active',version=version+1,updated_at=$7 where tenant_id=$1 and principal_id=$2`, tenant, actor.ID.String(), email, roles[0], roles, scopes, now)
	} else if errors.Is(err, pgx.ErrNoRows) {
		membershipVersion = 1
		_, err = tx.Exec(ctx, `insert into identity.memberships(tenant_id,principal_id,email,role,roles,scopes,status,created_at,updated_at) values($1,$2,$3,$4,$5,$6,'active',$7,$7)`, tenant, actor.ID.String(), email, roles[0], roles, scopes, now)
	}
	if err != nil {
		return domain.Membership{}, err
	}
	state := map[string]any{"invitation_id": invitation, "status": "active", "roles": roles, "scopes": scopes, "version": membershipVersion}
	if err = auditChange(ctx, tx, tenantID, actor.ID, &actor.ID, domain.ReasonInvitationAccepted, correlation, previous, state, now); err != nil {
		return domain.Membership{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Membership{}, err
	}
	return domain.Membership{TenantID: tenantID, PrincipalID: &actor.ID, Email: email, Roles: rolesFrom(roles), Scopes: scopesFrom(scopes), Status: "active", Version: membershipVersion}, nil
}

func (s *Postgres) ChangeMember(ctx context.Context, p port.MembershipMutation) (domain.Membership, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Membership{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = scopedTx(ctx, tx, p.Actor.ID, p.TenantID); err != nil {
		return domain.Membership{}, err
	}
	if err = ownerLock(ctx, tx, p.Actor.ID, p.TenantID); err != nil {
		return domain.Membership{}, err
	}
	var email, status string
	var oldRoles, oldScopes []string
	var version int64
	err = tx.QueryRow(ctx, `select coalesce(email,''),roles,scopes,status,version from identity.memberships where tenant_id=$1 and principal_id=$2 for update`, p.TenantID.String(), p.TargetID.String()).Scan(&email, &oldRoles, &oldScopes, &status, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Membership{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Membership{}, err
	}
	if status != "active" {
		return domain.Membership{}, domain.ErrNotFound
	}
	if version != p.ExpectedVersion {
		return domain.Membership{}, domain.ErrConflict
	}
	if contains(oldRoles, "owner") && !contains(roleStrings(p.Roles), "owner") {
		ok, err := otherOwner(ctx, tx, p.TenantID, p.TargetID)
		if err != nil {
			return domain.Membership{}, err
		}
		if !ok {
			return domain.Membership{}, domain.ErrLastOwner
		}
	}
	_, err = tx.Exec(ctx, `update identity.memberships set role=$3,roles=$4,scopes=$5,version=version+1,updated_at=$6 where tenant_id=$1 and principal_id=$2`, p.TenantID.String(), p.TargetID.String(), string(p.Roles[0]), roleStrings(p.Roles), scopeStrings(p.Scopes), p.Now)
	if err != nil {
		return domain.Membership{}, err
	}
	old := map[string]any{"status": status, "roles": oldRoles, "scopes": oldScopes, "version": version}
	next := map[string]any{"status": "active", "roles": p.Roles, "scopes": p.Scopes, "version": version + 1}
	if err = auditChange(ctx, tx, p.TenantID, p.Actor.ID, &p.TargetID, p.Reason, p.CorrelationID, old, next, p.Now); err != nil {
		return domain.Membership{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Membership{}, err
	}
	return domain.Membership{TenantID: p.TenantID, PrincipalID: &p.TargetID, Email: email, Roles: p.Roles, Scopes: p.Scopes, Status: "active", Version: version + 1}, nil
}

func (s *Postgres) RevokeMember(ctx context.Context, p port.MembershipMutation) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = scopedTx(ctx, tx, p.Actor.ID, p.TenantID); err != nil {
		return err
	}
	if err = ownerLock(ctx, tx, p.Actor.ID, p.TenantID); err != nil {
		return err
	}
	var email, status string
	var roles, scopes []string
	var version int64
	err = tx.QueryRow(ctx, `select coalesce(email,''),roles,scopes,status,version from identity.memberships where tenant_id=$1 and principal_id=$2 for update`, p.TenantID.String(), p.TargetID.String()).Scan(&email, &roles, &scopes, &status, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "active" {
		return domain.ErrNotFound
	}
	if version != p.ExpectedVersion {
		return domain.ErrConflict
	}
	if contains(roles, "owner") {
		ok, err := otherOwner(ctx, tx, p.TenantID, p.TargetID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrLastOwner
		}
	}
	_, err = tx.Exec(ctx, `update identity.memberships set status='revoked',version=version+1,updated_at=$3 where tenant_id=$1 and principal_id=$2`, p.TenantID.String(), p.TargetID.String(), p.Now)
	if err != nil {
		return err
	}
	old := map[string]any{"status": "active", "roles": roles, "scopes": scopes, "version": version}
	next := map[string]any{"status": "revoked", "roles": roles, "scopes": scopes, "version": version + 1}
	if err = auditChange(ctx, tx, p.TenantID, p.Actor.ID, &p.TargetID, p.Reason, p.CorrelationID, old, next, p.Now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func otherOwner(ctx context.Context, tx pgx.Tx, tenant, target id.UUID) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `select exists(select 1 from identity.memberships where tenant_id=$1 and principal_id<>$2 and status='active' and 'owner'=any(roles))`, tenant.String(), target.String()).Scan(&ok)
	return ok, err
}

var _ port.MembershipRepository = (*Postgres)(nil)
