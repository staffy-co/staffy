package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/employee/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
)

const knowledgeColumns = "tenant_id,employee_id,source_id,source_type,title,owner_name,provenance,origin_ref,ingest_reason,source_scope,priority,language,status,version,current_version,conflict_note,conflict_provenance,fresh_until,created_at,updated_at"

func scanKnowledge(row pgx.Row) (domain.KnowledgeSource, error) {
	var s domain.KnowledgeSource
	err := row.Scan(&s.TenantID, &s.EmployeeID, &s.SourceID, &s.Type, &s.Title, &s.Owner, &s.Provenance, &s.Origin, &s.IngestReason, &s.Scope, &s.Priority, &s.Language, &s.Status, &s.Version, &s.CurrentVersion, &s.ConflictNote, &s.ConflictProvenance, &s.FreshUntil, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, domain.ErrNotFound
	}
	return s, err
}
func (s *Postgres) CreateKnowledge(ctx context.Context, p port.KnowledgeMutation, k domain.KnowledgeSource) (domain.KnowledgeSource, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return k, err
	}
	defer tx.Rollback(ctx)
	var pack []byte
	err = tx.QueryRow(ctx, "select domain_pack from employee.drafts where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), p.EmployeeID).Scan(&pack)
	if errors.Is(err, pgx.ErrNoRows) {
		return k, domain.ErrNotFound
	}
	if err != nil {
		return k, err
	}
	if len(pack) == 0 {
		return k, domain.ErrInvalid
	}
	k, err = scanKnowledge(tx.QueryRow(ctx, "insert into employee.knowledge_sources(tenant_id,employee_id,source_id,source_type,title,owner_name,provenance,origin_ref,ingest_reason,source_scope,priority,language,checksum_key,fresh_until,created_at,updated_at,status) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$15,$16) returning "+knowledgeColumns, p.Scope.TenantID(), p.EmployeeID, p.SourceID, k.Type, k.Title, k.Owner, k.Provenance, k.Origin, k.IngestReason, k.Scope, k.Priority, k.Language, k.ChecksumKey, k.FreshUntil, p.Now, k.Status))
	if err != nil {
		return k, err
	}
	if err = knowledgeEvent(ctx, tx, p, k, "created"); err != nil {
		return k, err
	}
	return k, tx.Commit(ctx)
}
func (s *Postgres) ListKnowledge(ctx context.Context, scope tenancy.Context, employee id.UUID) ([]domain.KnowledgeSource, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "select "+knowledgeColumns+" from employee.knowledge_sources where tenant_id=$1 and employee_id=$2 order by created_at,source_id", scope.TenantID(), employee)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.KnowledgeSource{}
	for rows.Next() {
		k, e := scanKnowledge(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, k)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return result, tx.Commit(ctx)
}
func (s *Postgres) ReadKnowledge(ctx context.Context, scope tenancy.Context, employee, source id.UUID) (domain.KnowledgeSource, []domain.KnowledgeVersion, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return domain.KnowledgeSource{}, nil, err
	}
	defer tx.Rollback(ctx)
	k, err := scanKnowledge(tx.QueryRow(ctx, "select "+knowledgeColumns+" from employee.knowledge_sources where tenant_id=$1 and employee_id=$2 and source_id=$3", scope.TenantID(), employee, source))
	if err != nil {
		return k, nil, err
	}
	rows, err := tx.Query(ctx, "select version,checksum,source_text,created_at,origin_ref from employee.knowledge_versions where tenant_id=$1 and employee_id=$2 and source_id=$3 order by version desc", scope.TenantID(), employee, source)
	if err != nil {
		return k, nil, err
	}
	versions := []domain.KnowledgeVersion{}
	for rows.Next() {
		v := domain.KnowledgeVersion{SourceID: source}
		if err = rows.Scan(&v.Version, &v.Checksum, &v.Text, &v.CreatedAt, &v.Origin); err != nil {
			rows.Close()
			return k, nil, err
		}
		versions = append(versions, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return k, nil, err
	}
	for i := range versions {
		cr, e := tx.Query(ctx, "select position,chunk_text,checksum from employee.knowledge_chunks where tenant_id=$1 and employee_id=$2 and source_id=$3 and version=$4 order by position", scope.TenantID(), employee, source, versions[i].Version)
		if e != nil {
			return k, nil, e
		}
		versions[i].Chunks = []domain.KnowledgeChunk{}
		for cr.Next() {
			var c domain.KnowledgeChunk
			if e = cr.Scan(&c.Position, &c.Text, &c.Checksum); e != nil {
				cr.Close()
				return k, nil, e
			}
			c.SourceID = source
			c.Version = versions[i].Version
			c.Origin = versions[i].Origin
			c.CapturedAt = versions[i].CreatedAt
			versions[i].Chunks = append(versions[i].Chunks, c)
		}
		e = cr.Err()
		cr.Close()
		if e != nil {
			return k, nil, e
		}
	}
	for i := 0; i+1 < len(versions); i++ {
		versions[i].RemovedFromPrevious, versions[i].AddedFromPrevious = domain.KnowledgeDiff(versions[i+1].Text, versions[i].Text)
	}
	return k, versions, tx.Commit(ctx)
}
func knowledgeEvent(ctx context.Context, tx pgx.Tx, p port.KnowledgeMutation, k domain.KnowledgeSource, outcome string) error {
	_, err := tx.Exec(ctx, "insert into employee.knowledge_audit(tenant_id,audit_id,employee_id,source_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)", p.Scope.TenantID(), p.AuditID, p.EmployeeID, p.SourceID, p.Scope.PrincipalID(), p.ExpectedVersion, k.Version, outcome, p.CorrelationID, p.Now)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"source_id": p.SourceID, "employee_id": p.EmployeeID, "version": k.Version, "current_version": k.CurrentVersion, "status": k.Status})
	_, err = tx.Exec(ctx, "insert into employee.knowledge_outbox(tenant_id,event_id,employee_id,source_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,$4,$5,1,$6,$7,$8,$9,$10)", p.Scope.TenantID(), p.EventID, p.EmployeeID, p.SourceID, "employee.knowledge_"+outcome, k.Version, p.Scope.PrincipalID(), p.CorrelationID, p.Now, payload)
	return err
}
func knowledgeLocked(ctx context.Context, tx pgx.Tx, p port.KnowledgeMutation) (domain.KnowledgeSource, error) {
	return scanKnowledge(tx.QueryRow(ctx, "select "+knowledgeColumns+" from employee.knowledge_sources where tenant_id=$1 and employee_id=$2 and source_id=$3 for update", p.Scope.TenantID(), p.EmployeeID, p.SourceID))
}
func (s *Postgres) ProcessKnowledge(ctx context.Context, p port.KnowledgeMutation, value string) (domain.KnowledgeSource, domain.KnowledgeVersion, bool, error) {
	return s.processKnowledge(ctx, p, value, false)
}
func (s *Postgres) ProcessSyntheticKnowledge(ctx context.Context, p port.KnowledgeMutation, value string) (domain.KnowledgeSource, domain.KnowledgeVersion, bool, error) {
	return s.processKnowledge(ctx, p, value, true)
}
func (s *Postgres) processKnowledge(ctx context.Context, p port.KnowledgeMutation, value string, synthetic bool) (domain.KnowledgeSource, domain.KnowledgeVersion, bool, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.KnowledgeSource{}, domain.KnowledgeVersion{}, false, err
	}
	defer tx.Rollback(ctx)
	k, err := knowledgeLocked(ctx, tx, p)
	if err != nil {
		return k, domain.KnowledgeVersion{}, false, err
	}
	if k.Status == "retired" || (synthetic && (k.Type != "website" && k.Type != "document" || k.Origin != "fixture:staffy-synthetic-v1")) || (!synthetic && (k.Type == "website" || k.Type == "document")) {
		return k, domain.KnowledgeVersion{}, false, domain.ErrInvalid
	}
	var checksumKey []byte
	if err = tx.QueryRow(ctx, "select checksum_key from employee.knowledge_sources where tenant_id=$1 and employee_id=$2 and source_id=$3", p.Scope.TenantID(), p.EmployeeID, p.SourceID).Scan(&checksumKey); err != nil {
		return k, domain.KnowledgeVersion{}, false, err
	}
	if len(checksumKey) != 32 {
		return k, domain.KnowledgeVersion{}, false, domain.ErrInvalid
	}
	checksum := domain.KnowledgeChecksum(checksumKey, value)
	v := domain.KnowledgeVersion{SourceID: p.SourceID, Checksum: checksum, Text: value, CreatedAt: p.Now, Origin: k.Origin, Chunks: domain.KnowledgeChunks(checksumKey, value)}
	var currentChecksum string
	if k.CurrentVersion > 0 {
		err = tx.QueryRow(ctx, "select checksum from employee.knowledge_versions where tenant_id=$1 and employee_id=$2 and source_id=$3 and version=$4", p.Scope.TenantID(), p.EmployeeID, p.SourceID, k.CurrentVersion).Scan(&currentChecksum)
		if err != nil {
			return k, v, false, err
		}
	}
	if checksum == currentChecksum {
		if p.ExpectedVersion > k.Version {
			return k, v, false, domain.ErrConflict
		}
		v = domain.KnowledgeVersion{SourceID: p.SourceID, Version: k.CurrentVersion}
		if err = tx.QueryRow(ctx, "select checksum,source_text,created_at,origin_ref from employee.knowledge_versions where tenant_id=$1 and employee_id=$2 and source_id=$3 and version=$4", p.Scope.TenantID(), p.EmployeeID, p.SourceID, v.Version).Scan(&v.Checksum, &v.Text, &v.CreatedAt, &v.Origin); err != nil {
			return k, v, false, err
		}
		rows, queryErr := tx.Query(ctx, "select position,chunk_text,checksum from employee.knowledge_chunks where tenant_id=$1 and employee_id=$2 and source_id=$3 and version=$4 order by position", p.Scope.TenantID(), p.EmployeeID, p.SourceID, v.Version)
		if queryErr != nil {
			return k, v, false, queryErr
		}
		v.Chunks = []domain.KnowledgeChunk{}
		for rows.Next() {
			chunk := domain.KnowledgeChunk{SourceID: p.SourceID, Version: v.Version, Origin: v.Origin, CapturedAt: v.CreatedAt}
			if err = rows.Scan(&chunk.Position, &chunk.Text, &chunk.Checksum); err != nil {
				rows.Close()
				return k, v, false, err
			}
			v.Chunks = append(v.Chunks, chunk)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return k, v, false, err
		}
		return k, v, true, tx.Commit(ctx)
	}
	if k.Version != p.ExpectedVersion {
		return k, v, false, domain.ErrConflict
	}
	v.Version = k.CurrentVersion + 1
	for i := range v.Chunks {
		v.Chunks[i].SourceID = p.SourceID
		v.Chunks[i].Version = v.Version
		v.Chunks[i].Origin = k.Origin
		v.Chunks[i].CapturedAt = p.Now
	}
	_, err = tx.Exec(ctx, "insert into employee.knowledge_versions(tenant_id,employee_id,source_id,version,checksum,source_text,created_at,origin_ref) values($1,$2,$3,$4,$5,$6,$7,$8)", p.Scope.TenantID(), p.EmployeeID, p.SourceID, v.Version, v.Checksum, v.Text, p.Now, v.Origin)
	if err != nil {
		return k, v, false, err
	}
	for _, c := range v.Chunks {
		_, err = tx.Exec(ctx, "insert into employee.knowledge_chunks(tenant_id,employee_id,source_id,version,position,chunk_text,checksum) values($1,$2,$3,$4,$5,$6,$7)", p.Scope.TenantID(), p.EmployeeID, p.SourceID, v.Version, c.Position, c.Text, c.Checksum)
		if err != nil {
			return k, v, false, err
		}
	}
	status := "ready"
	if synthetic {
		status = "preview"
	}
	k, err = scanKnowledge(tx.QueryRow(ctx, "update employee.knowledge_sources set status=$6,current_version=$4,version=version+1,updated_at=$5 where tenant_id=$1 and employee_id=$2 and source_id=$3 returning "+knowledgeColumns, p.Scope.TenantID(), p.EmployeeID, p.SourceID, v.Version, p.Now, status))
	if err != nil {
		return k, v, false, err
	}
	if err = knowledgeEvent(ctx, tx, p, k, "processed"); err != nil {
		return k, v, false, err
	}
	return k, v, false, tx.Commit(ctx)
}
func (s *Postgres) ConflictKnowledge(ctx context.Context, p port.KnowledgeMutation, note, provenance string) (domain.KnowledgeSource, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.KnowledgeSource{}, err
	}
	defer tx.Rollback(ctx)
	k, err := knowledgeLocked(ctx, tx, p)
	if err != nil {
		return k, err
	}
	if k.Version != p.ExpectedVersion {
		return k, domain.ErrConflict
	}
	if k.Status == "retired" {
		return k, domain.ErrInvalid
	}
	k, err = scanKnowledge(tx.QueryRow(ctx, "update employee.knowledge_sources set conflict_note=$4,conflict_provenance=$5,version=version+1,updated_at=$6 where tenant_id=$1 and employee_id=$2 and source_id=$3 returning "+knowledgeColumns, p.Scope.TenantID(), p.EmployeeID, p.SourceID, note, provenance, p.Now))
	if err != nil {
		return k, err
	}
	if err = knowledgeEvent(ctx, tx, p, k, "conflict_marked"); err != nil {
		return k, err
	}
	return k, tx.Commit(ctx)
}
func (s *Postgres) RetireKnowledge(ctx context.Context, p port.KnowledgeMutation) (domain.KnowledgeSource, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.KnowledgeSource{}, err
	}
	defer tx.Rollback(ctx)
	k, err := knowledgeLocked(ctx, tx, p)
	if err != nil {
		return k, err
	}
	if k.Version != p.ExpectedVersion {
		return k, domain.ErrConflict
	}
	if k.Status == "retired" {
		return k, nil
	}
	k, err = scanKnowledge(tx.QueryRow(ctx, "update employee.knowledge_sources set status='retired',version=version+1,updated_at=$4 where tenant_id=$1 and employee_id=$2 and source_id=$3 returning "+knowledgeColumns, p.Scope.TenantID(), p.EmployeeID, p.SourceID, p.Now))
	if err != nil {
		return k, err
	}
	if err = knowledgeEvent(ctx, tx, p, k, "retired"); err != nil {
		return k, err
	}
	return k, tx.Commit(ctx)
}
