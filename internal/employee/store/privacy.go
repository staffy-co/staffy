package store

import (
	"context"

	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// Story 1.10: tenant-scoped purge methods backed by the SECURITY DEFINER
// functions of migration 00009. The application role never deletes table
// rows directly; each purge is executed by the module that owns the tables
// and returns counters for the Data Lifecycle Receipt.

// PurgeKnowledgePayload removes all knowledge sources, versions and chunks
// of the tenant plus the dependent knowledge audit and outbox rows.
func (s *Postgres) PurgeKnowledgePayload(ctx context.Context, scope tenancy.Context) (sources, dependent int64, err error) {
	tx, err := s.begin(ctx, scope, false)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = tx.QueryRow(ctx, "select * from employee.purge_knowledge_payload($1)", scope.TenantID()).Scan(&sources, &dependent); err != nil {
		return 0, 0, err
	}
	return sources, dependent, tx.Commit(ctx)
}

// PurgeVoicePreviewMedia removes all voice previews of the tenant and the
// assessments bound to them by the foreign key.
func (s *Postgres) PurgeVoicePreviewMedia(ctx context.Context, scope tenancy.Context) (previews, assessments int64, err error) {
	tx, err := s.begin(ctx, scope, false)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = tx.QueryRow(ctx, "select * from employee.purge_voice_preview_media($1)", scope.TenantID()).Scan(&previews, &assessments); err != nil {
		return 0, 0, err
	}
	return previews, assessments, tx.Commit(ctx)
}

// PurgeVoiceAssessments removes only the assessment rows of the tenant.
func (s *Postgres) PurgeVoiceAssessments(ctx context.Context, scope tenancy.Context) (assessments int64, err error) {
	tx, err := s.begin(ctx, scope, false)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = tx.QueryRow(ctx, "select employee.purge_voice_assessments($1)", scope.TenantID()).Scan(&assessments); err != nil {
		return 0, err
	}
	return assessments, tx.Commit(ctx)
}

// PurgeEmployeeDrafts removes the whole per-employee data aggregate of the
// tenant: drafts, knowledge, behavior, voice, previews, assessments and the
// dependent audit, outbox and idempotency rows (foreign keys bind them into
// one physical aggregate; every row group is counted).
func (s *Postgres) PurgeEmployeeDrafts(ctx context.Context, scope tenancy.Context) (drafts, dependent int64, err error) {
	tx, err := s.begin(ctx, scope, false)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = tx.QueryRow(ctx, "select * from employee.purge_employee_drafts($1)", scope.TenantID()).Scan(&drafts, &dependent); err != nil {
		return 0, 0, err
	}
	return drafts, dependent, tx.Commit(ctx)
}
