package store

import (
	"context"
	"time"

	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
)

// Story 1.10: identity-owned deletion support. The purge of expired
// invitations runs through the SECURITY DEFINER function of migration
// identity/00003; the row counts stay tenant-scoped through RLS.

func (s *Postgres) scopedTx(ctx context.Context, scope tenancy.Context, readonly bool) (pgx.Tx, error) {
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

// PurgeExpiredInvitations removes invitations whose expiry is at or before
// the cutoff. Active memberships are never touched.
func (s *Postgres) PurgeExpiredInvitations(ctx context.Context, scope tenancy.Context, cutoff time.Time) (int64, error) {
	tx, err := s.scopedTx(ctx, scope, false)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var deleted int64
	if err = tx.QueryRow(ctx, "select identity.purge_expired_invitations($1,$2)", scope.TenantID(), cutoff).Scan(&deleted); err != nil {
		return 0, err
	}
	return deleted, tx.Commit(ctx)
}

// IdentityCensus counts active memberships and still-pending invitations of
// the tenant for the honest member-identity deletion outcome.
func (s *Postgres) IdentityCensus(ctx context.Context, scope tenancy.Context) (activeMemberships, pendingInvitations int64, err error) {
	tx, err := s.scopedTx(ctx, scope, true)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = tx.QueryRow(ctx, "select count(*) from identity.memberships where tenant_id=$1 and status='active'", scope.TenantID()).Scan(&activeMemberships); err != nil {
		return 0, 0, err
	}
	if err = tx.QueryRow(ctx, "select count(*) from identity.invitations where tenant_id=$1 and status='pending' and expires_at > clock_timestamp()", scope.TenantID()).Scan(&pendingInvitations); err != nil {
		return 0, 0, err
	}
	return activeMemberships, pendingInvitations, tx.Commit(ctx)
}

// EarliestPendingExpiry returns the earliest expiry moment of still-pending
// invitations (nil when none are pending).
func (s *Postgres) EarliestPendingExpiry(ctx context.Context, scope tenancy.Context) (*time.Time, error) {
	tx, err := s.scopedTx(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var expiry *time.Time
	if err = tx.QueryRow(ctx, "select min(expires_at) from identity.invitations where tenant_id=$1 and status='pending' and expires_at > clock_timestamp()", scope.TenantID()).Scan(&expiry); err != nil {
		return nil, err
	}
	return expiry, tx.Commit(ctx)
}
