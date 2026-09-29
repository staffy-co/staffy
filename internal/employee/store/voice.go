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

func (s *Postgres) ReadVoice(ctx context.Context, scope tenancy.Context, employeeID id.UUID) ([]domain.VoiceVersion, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, "select tenant_id,employee_id,version,draft_version,config,actor_id,changed_at,change_summary from employee.voice_versions where tenant_id=$1 and employee_id=$2 order by version desc", scope.TenantID(), employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.VoiceVersion{}
	for rows.Next() {
		var v domain.VoiceVersion
		var config []byte
		if err = rows.Scan(&v.TenantID, &v.EmployeeID, &v.Version, &v.DraftVersion, &config, &v.ActorID, &v.ChangedAt, &v.ChangeSummary); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(config, &v.Config); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func (s *Postgres) SaveVoice(ctx context.Context, p port.VoiceSaveParams) (domain.VoiceVersion, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.VoiceVersion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var draftVersion int64
	err = tx.QueryRow(ctx, "update employee.drafts set version=version+1,updated_at=$3 where tenant_id=$1 and employee_id=$2 and version=$4 returning version", p.Scope.TenantID(), p.EmployeeID, p.Now, p.ExpectedVersion).Scan(&draftVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		var current int64
		e := tx.QueryRow(ctx, "select version from employee.drafts where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), p.EmployeeID).Scan(&current)
		if errors.Is(e, pgx.ErrNoRows) {
			return domain.VoiceVersion{}, domain.ErrNotFound
		}
		if e != nil {
			return domain.VoiceVersion{}, e
		}
		_, e = tx.Exec(ctx, "insert into employee.audit(tenant_id,audit_id,employee_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'conflict',$7,$8)", p.Scope.TenantID(), p.AuditID, p.EmployeeID, p.Scope.PrincipalID(), p.ExpectedVersion, current, p.CorrelationID, p.Now)
		if e != nil {
			return domain.VoiceVersion{}, e
		}
		if e = tx.Commit(ctx); e != nil {
			return domain.VoiceVersion{}, e
		}
		return domain.VoiceVersion{}, domain.ErrConflict
	}
	if err != nil {
		return domain.VoiceVersion{}, err
	}
	var version int64
	if err = tx.QueryRow(ctx, "select coalesce(max(version),0)+1 from employee.voice_versions where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), p.EmployeeID).Scan(&version); err != nil {
		return domain.VoiceVersion{}, err
	}
	config, err := json.Marshal(p.Config)
	if err != nil {
		return domain.VoiceVersion{}, err
	}
	v := domain.VoiceVersion{TenantID: p.Scope.TenantID(), EmployeeID: p.EmployeeID, Version: version, DraftVersion: draftVersion, Config: p.Config, ActorID: p.Scope.PrincipalID(), ChangedAt: p.Now, ChangeSummary: p.Summary}
	_, err = tx.Exec(ctx, "insert into employee.voice_versions(tenant_id,employee_id,version,draft_version,config,actor_id,changed_at,change_summary) values($1,$2,$3,$4,$5,$6,$7,$8)", v.TenantID, v.EmployeeID, v.Version, v.DraftVersion, config, v.ActorID, v.ChangedAt, v.ChangeSummary)
	if err != nil {
		return domain.VoiceVersion{}, err
	}
	_, err = tx.Exec(ctx, "insert into employee.audit(tenant_id,audit_id,employee_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'voice_saved',$7,$8)", v.TenantID, p.AuditID, p.EmployeeID, v.ActorID, p.ExpectedVersion, draftVersion, p.CorrelationID, p.Now)
	if err != nil {
		return domain.VoiceVersion{}, err
	}
	payload, _ := json.Marshal(map[string]any{"employee_id": p.EmployeeID, "draft_version": draftVersion, "voice_version": version, "change_summary": p.Summary})
	_, err = tx.Exec(ctx, "insert into employee.outbox_events(tenant_id,event_id,employee_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,'employee.voice_saved',1,$4,$5,$6,$7,$8)", v.TenantID, p.EventID, p.EmployeeID, draftVersion, v.ActorID, p.CorrelationID, p.Now, payload)
	if err != nil {
		return domain.VoiceVersion{}, err
	}
	return v, tx.Commit(ctx)
}

func (s *Postgres) SaveVoicePreview(ctx context.Context, scope tenancy.Context, p domain.VoicePreviewRecord) error {
	tx, err := s.begin(ctx, scope, false)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var currentDraft int64
	err = tx.QueryRow(ctx, "select version from employee.drafts where tenant_id=$1 and employee_id=$2 for update", scope.TenantID(), p.EmployeeID).Scan(&currentDraft)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if currentDraft != p.DraftVersion {
		return domain.ErrConflict
	}
	var latest int64
	err = tx.QueryRow(ctx, "select coalesce(max(version),0) from employee.voice_versions where tenant_id=$1 and employee_id=$2", scope.TenantID(), p.EmployeeID).Scan(&latest)
	if err != nil {
		return err
	}
	if latest != p.VoiceVersion {
		return domain.ErrConflict
	}
	_, err = tx.Exec(ctx, "insert into employee.voice_previews(tenant_id,employee_id,preview_id,voice_version,dictionary_version,draft_version,source_text,synthesis_text,route,slow,actor_id,created_at) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,clock_timestamp())", scope.TenantID(), p.EmployeeID, p.PreviewID, p.VoiceVersion, p.DictionaryVersion, p.DraftVersion, p.Text, p.SynthesisText, p.Route, p.Slow, scope.PrincipalID())
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Postgres) ReadVoiceAssessments(ctx context.Context, scope tenancy.Context, employeeID id.UUID) ([]domain.VoiceAssessment, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `select p.tenant_id,p.employee_id,p.preview_id,p.voice_version,p.dictionary_version,p.draft_version,p.source_text,p.synthesis_text,p.route,p.slow,p.actor_id,p.created_at,
 a.assessment_id,a.verdict,a.actor_id,a.assessed_at,
 (p.voice_version <> (select max(version) from employee.voice_versions where tenant_id=p.tenant_id and employee_id=p.employee_id) or p.draft_version <> d.version) as stale
 from employee.voice_assessments a join employee.voice_previews p on p.tenant_id=a.tenant_id and p.preview_id=a.preview_id
 join employee.drafts d on d.tenant_id=p.tenant_id and d.employee_id=p.employee_id
 where p.tenant_id=$1 and p.employee_id=$2 and p.created_at > now() - interval '24 hours' and a.assessed_at > now() - interval '24 hours' order by a.assessed_at desc`, scope.TenantID(), employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.VoiceAssessment{}
	for rows.Next() {
		var a domain.VoiceAssessment
		err = rows.Scan(&a.TenantID, &a.EmployeeID, &a.PreviewID, &a.VoiceVersion, &a.DictionaryVersion, &a.DraftVersion, &a.Text, &a.SynthesisText, &a.Route, &a.Slow, &a.ActorID, &a.CreatedAt, &a.AssessmentID, &a.Verdict, &a.AssessedBy, &a.AssessedAt, &a.Stale)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func (s *Postgres) SaveVoiceAssessment(ctx context.Context, p port.VoiceAssessmentSaveParams) (domain.VoiceAssessment, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.VoiceAssessment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var receipt domain.VoicePreviewRecord
	var currentDraft, latestVoice int64
	var unexpired bool
	err = tx.QueryRow(ctx, `select v.tenant_id,v.employee_id,v.preview_id,v.voice_version,v.dictionary_version,v.draft_version,v.source_text,v.synthesis_text,v.route,v.slow,v.actor_id,v.created_at,d.version,
 (select max(version) from employee.voice_versions where tenant_id=v.tenant_id and employee_id=v.employee_id),
 v.created_at > clock_timestamp() - interval '24 hours'
 from employee.voice_previews v join employee.drafts d on d.tenant_id=v.tenant_id and d.employee_id=v.employee_id
	 where v.tenant_id=$1 and v.employee_id=$2 and v.preview_id=$3 for update of d`, p.Scope.TenantID(), p.EmployeeID, p.PreviewID).Scan(&receipt.TenantID, &receipt.EmployeeID, &receipt.PreviewID, &receipt.VoiceVersion, &receipt.DictionaryVersion, &receipt.DraftVersion, &receipt.Text, &receipt.SynthesisText, &receipt.Route, &receipt.Slow, &receipt.ActorID, &receipt.CreatedAt, &currentDraft, &latestVoice, &unexpired)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.VoiceAssessment{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.VoiceAssessment{}, err
	}
	if receipt.VoiceVersion != p.VoiceVersion || receipt.VoiceVersion != latestVoice || receipt.DraftVersion != currentDraft || !unexpired {
		return domain.VoiceAssessment{}, domain.ErrConflict
	}
	var prior int
	if err = tx.QueryRow(ctx, "select count(*) from employee.voice_assessments where tenant_id=$1 and preview_id=$2", p.Scope.TenantID(), p.PreviewID).Scan(&prior); err != nil {
		return domain.VoiceAssessment{}, err
	}
	if prior != 0 {
		return domain.VoiceAssessment{}, domain.ErrConflict
	}
	assessment := domain.VoiceAssessment{VoicePreviewRecord: receipt, AssessmentID: p.AssessmentID, Verdict: p.Verdict, AssessedBy: p.Scope.PrincipalID(), AssessedAt: p.Now, PhoneQualityApproved: false}
	commandTag, err := tx.Exec(ctx, "insert into employee.voice_assessments(tenant_id,employee_id,assessment_id,preview_id,verdict,actor_id,assessed_at) values($1,$2,$3,$4,$5,$6,$7) on conflict do nothing", p.Scope.TenantID(), p.EmployeeID, p.AssessmentID, p.PreviewID, p.Verdict, p.Scope.PrincipalID(), p.Now)
	if err != nil {
		return domain.VoiceAssessment{}, err
	}
	if commandTag.RowsAffected() != 1 {
		return domain.VoiceAssessment{}, domain.ErrConflict
	}
	_, err = tx.Exec(ctx, "insert into employee.audit(tenant_id,audit_id,employee_id,actor_id,attempted_version,resulting_version,outcome,correlation_id,occurred_at) values($1,$2,$3,$4,$5,$6,'voice_assessed',$7,$8)", p.Scope.TenantID(), p.AuditID, p.EmployeeID, p.Scope.PrincipalID(), currentDraft, currentDraft, p.CorrelationID, p.Now)
	if err != nil {
		return domain.VoiceAssessment{}, err
	}
	payload, _ := json.Marshal(map[string]any{"employee_id": p.EmployeeID, "preview_id": p.PreviewID, "voice_version": p.VoiceVersion, "dictionary_version": receipt.DictionaryVersion, "verdict": p.Verdict})
	_, err = tx.Exec(ctx, "insert into employee.outbox_events(tenant_id,event_id,employee_id,event_type,schema_version,aggregate_version,actor_id,correlation_id,occurred_at,payload) values($1,$2,$3,'employee.voice_assessed',1,$4,$5,$6,$7,$8)", p.Scope.TenantID(), p.EventID, p.EmployeeID, currentDraft, p.Scope.PrincipalID(), p.CorrelationID, p.Now, payload)
	if err != nil {
		return domain.VoiceAssessment{}, err
	}
	return assessment, tx.Commit(ctx)
}
