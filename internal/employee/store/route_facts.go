package store

import (
	"context"
	"errors"

	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
)

// ListTenantAdmissions reads the whole admission history of the tenant
// (every employee): the RouteFacts facade derives the acting decision per
// employee itself. The read boundary keeps the UTC canonicalization of
// scanAdmission (the R2/N1 lesson: the digests are recomputed identically
// on any host).
func (s *Postgres) ListTenantAdmissions(ctx context.Context, scope tenancy.Context) ([]domain.ReleaseAdmission, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, "select "+admissionColumns+" from employee.release_admissions where tenant_id=$1 order by employee_id, decision_no", scope.TenantID())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ReleaseAdmission{}
	for rows.Next() {
		admission, err := scanAdmission(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, admission)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

// ReadManifestContent reads the sealed manifest payload behind an exact
// manifest link: the strict schema read of ReadManifestPayload plus the
// link digest check — the pinned reference is never "the current state".
func (s *Postgres) ReadManifestContent(ctx context.Context, scope tenancy.Context, employeeID id.UUID, link domain.ManifestLink) (domain.ManifestContent, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return domain.ManifestContent{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var payload []byte
	var digest string
	err = tx.QueryRow(ctx, "select content, digest from employee.release_manifests where tenant_id=$1 and employee_id=$2 and manifest_id=$3 and config_version=$4", scope.TenantID(), employeeID, link.ManifestID, link.ConfigVersion).Scan(&payload, &digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ManifestContent{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ManifestContent{}, err
	}
	if digest != link.Digest {
		return domain.ManifestContent{}, domain.ProfileRuleError{Section: "route_facts", Field: "manifest", Reason: "Digest манифеста не совпадает со ссылкой решения: запечатленная версия не читается как текущая."}
	}
	read := domain.ReadManifestPayload(payload, digest)
	if read.Status != "" {
		return domain.ManifestContent{}, domain.ProfileRuleError{Section: "route_facts", Field: "manifest", Reason: "Манифест решения не читается: " + read.Reason}
	}
	return read.Content, tx.Commit(ctx)
}

// ReadApprovedProfileVersion reads one approved profile version by its
// number: the exact pinned content (the human coverage of the ingress
// owner check). The read boundary keeps the UTC canonicalization of
// scanProfileVersion.
func (s *Postgres) ReadApprovedProfileVersion(ctx context.Context, scope tenancy.Context, employeeID id.UUID, version int64) (domain.ProfileVersion, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return domain.ProfileVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row := tx.QueryRow(ctx, "select "+profileVersionColumns+" from employee.operating_profiles where tenant_id=$1 and employee_id=$2 and version=$3", scope.TenantID(), employeeID, version)
	profile, err := scanProfileVersion(row)
	if err != nil {
		return domain.ProfileVersion{}, err
	}
	return profile, tx.Commit(ctx)
}
