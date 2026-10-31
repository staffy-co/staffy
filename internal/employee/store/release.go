package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/employee/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// row scans both QueryRow results and Rows iterators.
type row interface{ Scan(dest ...any) error }

const releaseDraftColumns = "tenant_id,employee_id,revision,refs,restored_from,actor_id,updated_at"

const configVersionColumns = "tenant_id,employee_id,version_id,version,refs,description,actor_id,created_at"

const releaseManifestColumns = "tenant_id,employee_id,manifest_id,config_version,schema_version,status,digest,content,actor_id,sealed_at"

func scanReleaseDraft(r row) (domain.ReleaseDraft, error) {
	var draft domain.ReleaseDraft
	var refs, restored []byte
	err := r.Scan(&draft.TenantID, &draft.EmployeeID, &draft.Revision, &refs, &restored, &draft.ActorID, &draft.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ReleaseDraft{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ReleaseDraft{}, err
	}
	if err := json.Unmarshal(refs, &draft.Refs); err != nil {
		return domain.ReleaseDraft{}, err
	}
	if len(restored) > 0 {
		var link domain.ConfigVersionLink
		if err := json.Unmarshal(restored, &link); err != nil {
			return domain.ReleaseDraft{}, err
		}
		draft.RestoredFrom = &link
	}
	return draft, nil
}

func scanConfigVersion(r row) (domain.ConfigVersion, error) {
	var version domain.ConfigVersion
	var refs []byte
	err := r.Scan(&version.TenantID, &version.EmployeeID, &version.VersionID, &version.Version, &refs, &version.Description, &version.ActorID, &version.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ConfigVersion{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	if err := json.Unmarshal(refs, &version.Refs); err != nil {
		return domain.ConfigVersion{}, err
	}
	return version, nil
}

// releaseManifestForScan decodes the manifest metadata plus the exact
// stored payload bytes (the honest read path re-checks the digest against
// the stored bytes).
func releaseManifestForScan(r row) (port.ManifestRecord, error) {
	var manifest domain.ReleaseManifest
	var content []byte
	err := r.Scan(&manifest.TenantID, &manifest.EmployeeID, &manifest.ManifestID, &manifest.ConfigVersion,
		&manifest.SchemaVersion, &manifest.Status, &manifest.Digest, &content, &manifest.ActorID, &manifest.SealedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return port.ManifestRecord{}, domain.ErrNotFound
	}
	if err != nil {
		return port.ManifestRecord{}, err
	}
	if err := json.Unmarshal(content, &manifest.Content); err != nil {
		return port.ManifestRecord{}, err
	}
	return port.ManifestRecord{Manifest: manifest, Payload: content}, nil
}

// releaseConflict builds the honest conflict from the current draft state.
func releaseConflict(ctx context.Context, tx pgx.Tx, scope tenancy.Context, employeeID id.UUID, expected int64) error {
	draft, err := scanReleaseDraft(tx.QueryRow(ctx, "select "+releaseDraftColumns+" from employee.release_drafts where tenant_id=$1 and employee_id=$2", scope.TenantID(), employeeID))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ReleaseConflictError{Expected: expected, Current: 0}
	}
	if err != nil {
		return err
	}
	return domain.ReleaseConflictError{Expected: expected, Current: draft.Revision, Refs: draft.Refs}
}

func releaseAudit(ctx context.Context, tx pgx.Tx, scope tenancy.Context, employeeID, auditID, correlationID id.UUID, outcome string, attempted, resulting int64, now time.Time) error {
	_, err := tx.Exec(ctx, "insert into employee.audit(tenant_id,audit_id,employee_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9)",
		scope.TenantID(), auditID, employeeID, scope.PrincipalID(), attempted, resulting, outcome, correlationID, now)
	return err
}

func releaseOutbox(ctx context.Context, tx pgx.Tx, scope tenancy.Context, employeeID, eventID, correlationID id.UUID, eventType string, revision int64, payload []byte, now time.Time) error {
	_, err := tx.Exec(ctx, "insert into employee.outbox_events(tenant_id,event_id,employee_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,$4,1,$5,$6,$7,$8,$9)",
		scope.TenantID(), eventID, employeeID, eventType, revision, scope.PrincipalID(), correlationID, now, payload)
	return err
}

// ReadReleaseDraft returns the current mutable draft.
func (s *Postgres) ReadReleaseDraft(ctx context.Context, scope tenancy.Context, employeeID id.UUID) (domain.ReleaseDraft, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return domain.ReleaseDraft{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	draft, err := scanReleaseDraft(tx.QueryRow(ctx, "select "+releaseDraftColumns+" from employee.release_drafts where tenant_id=$1 and employee_id=$2", scope.TenantID(), employeeID))
	if err != nil {
		return domain.ReleaseDraft{}, err
	}
	return draft, tx.Commit(ctx)
}

// SaveReleaseDraft creates or CAS-updates the release draft.
func (s *Postgres) SaveReleaseDraft(ctx context.Context, p port.DraftParams) (domain.ReleaseDraft, error) {
	return s.saveReleaseDraftRow(ctx, p.Scope, p.EmployeeID, p.EventID, p.AuditID, p.CorrelationID, p.Refs, nil, p.ExpectedRevision, p.Now, "release_draft_saved", "employee.release_draft_saved")
}

// RestoreReleaseDraft CAS-updates the draft into a restored draft that
// references the source configuration version.
func (s *Postgres) RestoreReleaseDraft(ctx context.Context, p port.RestoreParams) (domain.ReleaseDraft, error) {
	return s.saveReleaseDraftRow(ctx, p.Scope, p.EmployeeID, p.EventID, p.AuditID, p.CorrelationID, p.Refs, p.RestoredFrom, p.ExpectedRevision, p.Now, "release_restored", "employee.release_restored")
}

func (s *Postgres) saveReleaseDraftRow(ctx context.Context, scope tenancy.Context, employeeID, eventID, auditID, correlationID id.UUID, refs domain.ConfigRefs, restoredFrom *domain.ConfigVersionLink, expected int64, now time.Time, outcome, eventType string) (domain.ReleaseDraft, error) {
	tx, err := s.begin(ctx, scope, false)
	if err != nil {
		return domain.ReleaseDraft{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	refsJSON, err := json.Marshal(refs)
	if err != nil {
		return domain.ReleaseDraft{}, err
	}
	var restoredJSON []byte
	if restoredFrom != nil {
		if restoredJSON, err = json.Marshal(restoredFrom); err != nil {
			return domain.ReleaseDraft{}, err
		}
	}
	var revision int64
	if expected == 0 {
		err = tx.QueryRow(ctx, "insert into employee.release_drafts(tenant_id,employee_id,revision,refs,restored_from,actor_id,updated_at) values($1,$2,1,$3,$4,$5,$6) on conflict (tenant_id,employee_id) do nothing returning revision",
			scope.TenantID(), employeeID, refsJSON, restoredJSON, scope.PrincipalID(), now).Scan(&revision)
		if errors.Is(err, pgx.ErrNoRows) {
			// releaseConflict always carries the current state; a plain
			// unreachable fallback would hide a real lookup failure.
			return domain.ReleaseDraft{}, releaseConflict(ctx, tx, scope, employeeID, expected)
		}
	} else {
		err = tx.QueryRow(ctx, "update employee.release_drafts set revision=revision+1, refs=$3, restored_from=$4, actor_id=$5, updated_at=$6 where tenant_id=$1 and employee_id=$2 and revision=$7 returning revision",
			scope.TenantID(), employeeID, refsJSON, restoredJSON, scope.PrincipalID(), now, expected).Scan(&revision)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ReleaseDraft{}, releaseConflict(ctx, tx, scope, employeeID, expected)
		}
	}
	if err != nil {
		return domain.ReleaseDraft{}, err
	}
	draft := domain.ReleaseDraft{TenantID: scope.TenantID(), EmployeeID: employeeID, Revision: revision, Refs: refs, ActorID: scope.PrincipalID(), UpdatedAt: now}
	if restoredFrom != nil {
		link := *restoredFrom
		draft.RestoredFrom = &link
	}
	if err := releaseAudit(ctx, tx, scope, employeeID, auditID, correlationID, outcome, expected, revision, now); err != nil {
		return domain.ReleaseDraft{}, err
	}
	payload, _ := json.Marshal(map[string]any{"employee_id": employeeID, "release_revision": revision})
	if err := releaseOutbox(ctx, tx, scope, employeeID, eventID, correlationID, eventType, revision, payload, now); err != nil {
		return domain.ReleaseDraft{}, err
	}
	return draft, tx.Commit(ctx)
}

// SaveConfigVersion appends the next monotonic configuration version under
// the locked draft row (MAX+1) and bumps the draft revision.
func (s *Postgres) SaveConfigVersion(ctx context.Context, p port.VersionParams) (domain.ConfigVersion, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	draft, err := scanReleaseDraft(tx.QueryRow(ctx, "select "+releaseDraftColumns+" from employee.release_drafts where tenant_id=$1 and employee_id=$2 for update", p.Scope.TenantID(), p.EmployeeID))
	if errors.Is(err, domain.ErrNotFound) {
		var exists int
		if empErr := tx.QueryRow(ctx, "select 1 from employee.drafts where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), p.EmployeeID).Scan(&exists); empErr != nil {
			return domain.ConfigVersion{}, domain.ErrNotFound
		}
		return domain.ConfigVersion{}, domain.ReleaseRuleError{Rule: "draft", Reason: "Черновик релиза не собран: создайте его перед версией конфигурации."}
	}
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	if draft.Revision != p.ExpectedRevision {
		return domain.ConfigVersion{}, domain.ReleaseConflictError{Expected: p.ExpectedRevision, Current: draft.Revision, Refs: draft.Refs}
	}
	var version int64
	if err = tx.QueryRow(ctx, "select coalesce(max(version),0)+1 from employee.config_versions where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), p.EmployeeID).Scan(&version); err != nil {
		return domain.ConfigVersion{}, err
	}
	refsJSON, err := json.Marshal(draft.Refs)
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	created := domain.ConfigVersion{
		TenantID: p.Scope.TenantID(), EmployeeID: p.EmployeeID, VersionID: p.VersionID, Version: version,
		Description: p.Description, Refs: draft.Refs, ActorID: p.Scope.PrincipalID(), CreatedAt: p.Now,
	}
	if _, err = tx.Exec(ctx, "insert into employee.config_versions("+configVersionColumns+") values($1,$2,$3,$4,$5,$6,$7,$8)",
		created.TenantID, created.EmployeeID, created.VersionID, created.Version, refsJSON, created.Description, created.ActorID, created.CreatedAt); err != nil {
		return domain.ConfigVersion{}, err
	}
	if _, err = tx.Exec(ctx, "update employee.release_drafts set revision=revision+1, actor_id=$3, updated_at=$4 where tenant_id=$1 and employee_id=$2 and revision=$5",
		p.Scope.TenantID(), p.EmployeeID, p.Scope.PrincipalID(), p.Now, p.ExpectedRevision); err != nil {
		return domain.ConfigVersion{}, err
	}
	if err := releaseAudit(ctx, tx, p.Scope, p.EmployeeID, p.AuditID, p.CorrelationID, "config_version_created", p.ExpectedRevision, draft.Revision+1, p.Now); err != nil {
		return domain.ConfigVersion{}, err
	}
	payload, _ := json.Marshal(map[string]any{"employee_id": p.EmployeeID, "config_version": version, "release_revision": draft.Revision + 1})
	if err := releaseOutbox(ctx, tx, p.Scope, p.EmployeeID, p.EventID, p.CorrelationID, "employee.config_version_created", draft.Revision+1, payload, p.Now); err != nil {
		return domain.ConfigVersion{}, err
	}
	return created, tx.Commit(ctx)
}

// SaveManifest appends the immutable manifest: the draft row is locked and
// the expected revision enforced (exactly one manifest wins a race), the
// UNIQUE constraint protects the one-manifest-per-version invariant.
func (s *Postgres) SaveManifest(ctx context.Context, p port.SealParams) (domain.ReleaseManifest, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	draft, err := scanReleaseDraft(tx.QueryRow(ctx, "select "+releaseDraftColumns+" from employee.release_drafts where tenant_id=$1 and employee_id=$2 for update", p.Scope.TenantID(), p.EmployeeID))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ReleaseManifest{}, domain.ReleaseRuleError{Rule: "draft", Reason: "Черновик релиза не собран: версия и манифест создаются из черновика."}
	}
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	if draft.Revision != p.ExpectedRevision {
		return domain.ReleaseManifest{}, domain.ReleaseConflictError{Expected: p.ExpectedRevision, Current: draft.Revision, Refs: draft.Refs}
	}
	config, err := scanConfigVersion(tx.QueryRow(ctx, "select "+configVersionColumns+" from employee.config_versions where tenant_id=$1 and employee_id=$2 and version=$3", p.Scope.TenantID(), p.EmployeeID, p.ConfigVersion))
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	_ = config // existence validated; the content was built by the app from this version
	contentJSON, err := json.Marshal(p.Content)
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	sealed := domain.ReleaseManifest{
		TenantID: p.Scope.TenantID(), EmployeeID: p.EmployeeID, ManifestID: p.ManifestID,
		ConfigVersion: p.ConfigVersion, SchemaVersion: p.Content.SchemaVersion, Status: p.Status,
		Digest: p.Digest, ActorID: p.Scope.PrincipalID(), SealedAt: p.Now, Content: p.Content,
	}
	if _, err = tx.Exec(ctx, "insert into employee.release_manifests("+releaseManifestColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)",
		sealed.TenantID, sealed.EmployeeID, sealed.ManifestID, sealed.ConfigVersion, sealed.SchemaVersion, sealed.Status, sealed.Digest, contentJSON, sealed.ActorID, sealed.SealedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ReleaseManifest{}, domain.ErrManifestExists
		}
		return domain.ReleaseManifest{}, err
	}
	if _, err = tx.Exec(ctx, "update employee.release_drafts set revision=revision+1, actor_id=$3, updated_at=$4 where tenant_id=$1 and employee_id=$2 and revision=$5",
		p.Scope.TenantID(), p.EmployeeID, p.Scope.PrincipalID(), p.Now, p.ExpectedRevision); err != nil {
		return domain.ReleaseManifest{}, err
	}
	if err := releaseAudit(ctx, tx, p.Scope, p.EmployeeID, p.AuditID, p.CorrelationID, "manifest_sealed", p.ExpectedRevision, draft.Revision+1, p.Now); err != nil {
		return domain.ReleaseManifest{}, err
	}
	payload, _ := json.Marshal(map[string]any{"employee_id": p.EmployeeID, "config_version": p.ConfigVersion, "digest": p.Digest, "status": p.Status, "release_revision": draft.Revision + 1})
	if err := releaseOutbox(ctx, tx, p.Scope, p.EmployeeID, p.EventID, p.CorrelationID, "employee.manifest_sealed", draft.Revision+1, payload, p.Now); err != nil {
		return domain.ReleaseManifest{}, err
	}
	return sealed, tx.Commit(ctx)
}

// ReadConfigVersion returns one immutable configuration version.
func (s *Postgres) ReadConfigVersion(ctx context.Context, scope tenancy.Context, employeeID id.UUID, version int64) (domain.ConfigVersion, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	found, err := scanConfigVersion(tx.QueryRow(ctx, "select "+configVersionColumns+" from employee.config_versions where tenant_id=$1 and employee_id=$2 and version=$3", scope.TenantID(), employeeID, version))
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	return found, tx.Commit(ctx)
}

// ListConfigVersions returns every configuration version, newest first.
func (s *Postgres) ListConfigVersions(ctx context.Context, scope tenancy.Context, employeeID id.UUID) ([]domain.ConfigVersion, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, "select "+configVersionColumns+" from employee.config_versions where tenant_id=$1 and employee_id=$2 order by version desc", scope.TenantID(), employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ConfigVersion{}
	for rows.Next() {
		version, err := scanConfigVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, version)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

// ListManifests returns every sealed manifest with the stored payload
// bytes, newest first.
func (s *Postgres) ListManifests(ctx context.Context, scope tenancy.Context, employeeID id.UUID) ([]port.ManifestRecord, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, "select "+releaseManifestColumns+" from employee.release_manifests where tenant_id=$1 and employee_id=$2 order by sealed_at desc, manifest_id desc", scope.TenantID(), employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []port.ManifestRecord{}
	for rows.Next() {
		record, err := releaseManifestForScan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}
