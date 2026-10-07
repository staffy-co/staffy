package store

import (
        "context"
        "encoding/json"
        "errors"
        "time"

        "github.com/artemiosu/staffy/internal/employee/domain"
        "github.com/artemiosu/staffy/internal/employee/port"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
        "github.com/jackc/pgx/v5"
        "github.com/jackc/pgx/v5/pgconn"
)

const profileDraftColumns = "tenant_id,employee_id,revision,approved_revision,manifest_id,config_version,manifest_digest,content,actor_id,updated_at"

const profileVersionColumns = "tenant_id,employee_id,version,manifest_id,config_version,manifest_digest,digest,content,approver_id,approver_roles,step_up_verified,description,evidence_refs,approved_at"

const thresholdColumns = "tenant_id,employee_id,gate_id,threshold_version,value,actor_id,decision,evidence_refs,supersedes_version,decided_at"

// scanProfileDraft reads the mutable profile draft row.
func scanProfileDraft(r row) (domain.ProfileDraft, error) {
        var draft domain.ProfileDraft
        var content []byte
        err := r.Scan(&draft.TenantID, &draft.EmployeeID, &draft.Revision, &draft.ApprovedRevision,
                &draft.Manifest.ManifestID, &draft.Manifest.ConfigVersion, &draft.Manifest.Digest, &content, &draft.ActorID, &draft.UpdatedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return domain.ProfileDraft{}, domain.ErrNotFound
        }
        if err != nil {
                return domain.ProfileDraft{}, err
        }
        if err := json.Unmarshal(content, &draft.Content); err != nil {
                return domain.ProfileDraft{}, err
        }
        return draft, nil
}

// scanProfileVersion reads one immutable approved profile row.
func scanProfileVersion(r row) (domain.ProfileVersion, error) {
        var version domain.ProfileVersion
        var content, roles, refs []byte
        err := r.Scan(&version.TenantID, &version.EmployeeID, &version.Version,
                &version.Manifest.ManifestID, &version.Manifest.ConfigVersion, &version.Manifest.Digest,
                &version.Digest, &content, &version.ApproverID, &roles, &version.StepUpVerified,
                &version.Description, &refs, &version.ApprovedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return domain.ProfileVersion{}, domain.ErrNotFound
        }
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        if err := json.Unmarshal(content, &version.Content); err != nil {
                return domain.ProfileVersion{}, err
        }
        if err := json.Unmarshal(roles, &version.ApproverRoles); err != nil {
                return domain.ProfileVersion{}, err
        }
        if err := json.Unmarshal(refs, &version.EvidenceRefs); err != nil {
                return domain.ProfileVersion{}, err
        }
        return version, nil
}

// thresholdForScan reads one threshold decision row. The supersedes link's
// decided-at is reconstructed afterwards from the history itself.
func thresholdForScan(r row) (domain.GateThresholdVersion, error) {
        var decision domain.GateThresholdVersion
        var refs []byte
        var supersedes *int64
        err := r.Scan(&decision.TenantID, &decision.EmployeeID, &decision.GateID, &decision.ThresholdVersion,
                &decision.Value, &decision.ActorID, &decision.Decision, &refs, &supersedes, &decision.DecidedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return domain.GateThresholdVersion{}, domain.ErrNotFound
        }
        if err != nil {
                return domain.GateThresholdVersion{}, err
        }
        if err := json.Unmarshal(refs, &decision.EvidenceRefs); err != nil {
                return domain.GateThresholdVersion{}, err
        }
        if supersedes != nil {
                decision.Supersedes = &domain.ThresholdLink{ThresholdVersion: *supersedes}
        }
        return decision, nil
}

// profileConflict builds the honest conflict from the current draft state.
func profileConflict(ctx context.Context, tx pgx.Tx, scope tenancy.Context, employeeID id.UUID, expected int64) error {
        draft, err := scanProfileDraft(tx.QueryRow(ctx, "select "+profileDraftColumns+" from employee.operating_profile_drafts where tenant_id=$1 and employee_id=$2", scope.TenantID(), employeeID))
        if errors.Is(err, domain.ErrNotFound) {
                return domain.ProfileConflictError{Expected: expected, Current: 0}
        }
        if err != nil {
                return err
        }
        return domain.ProfileConflictError{Expected: expected, Current: draft.Revision, CurrentContent: &draft.Content}
}

// ReadProfileDraft returns the current mutable draft.
func (s *Postgres) ReadProfileDraft(ctx context.Context, scope tenancy.Context, employeeID id.UUID) (domain.ProfileDraft, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return domain.ProfileDraft{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        draft, err := scanProfileDraft(tx.QueryRow(ctx, "select "+profileDraftColumns+" from employee.operating_profile_drafts where tenant_id=$1 and employee_id=$2", scope.TenantID(), employeeID))
        if err != nil {
                return domain.ProfileDraft{}, err
        }
        return draft, tx.Commit(ctx)
}

// SaveProfileDraft creates the draft (ExpectedRevision 0) or updates it under
// the expected revision. A lost race returns ProfileConflictError with the
// current draft content for an honest comparison.
func (s *Postgres) SaveProfileDraft(ctx context.Context, p port.ProfileDraftParams) (domain.ProfileDraft, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return domain.ProfileDraft{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        contentJSON, err := json.Marshal(p.Content)
        if err != nil {
                return domain.ProfileDraft{}, err
        }
        var revision int64
        if p.ExpectedRevision == 0 {
                err = tx.QueryRow(ctx, "insert into employee.operating_profile_drafts(tenant_id,employee_id,revision,manifest_id,config_version,manifest_digest,content,actor_id,updated_at) values($1,$2,1,$3,$4,$5,$6,$7,$8) on conflict (tenant_id,employee_id) do nothing returning revision",
                        p.Scope.TenantID(), p.EmployeeID, p.Manifest.ManifestID, p.Manifest.ConfigVersion, p.Manifest.Digest, contentJSON, p.Scope.PrincipalID(), p.Now).Scan(&revision)
                if errors.Is(err, pgx.ErrNoRows) {
                        return domain.ProfileDraft{}, profileConflict(ctx, tx, p.Scope, p.EmployeeID, p.ExpectedRevision)
                }
        } else {
                err = tx.QueryRow(ctx, "update employee.operating_profile_drafts set revision=revision+1, manifest_id=$3, config_version=$4, manifest_digest=$5, content=$6, actor_id=$7, updated_at=$8 where tenant_id=$1 and employee_id=$2 and revision=$9 returning revision",
                        p.Scope.TenantID(), p.EmployeeID, p.Manifest.ManifestID, p.Manifest.ConfigVersion, p.Manifest.Digest, contentJSON, p.Scope.PrincipalID(), p.Now, p.ExpectedRevision).Scan(&revision)
                if errors.Is(err, pgx.ErrNoRows) {
                        return domain.ProfileDraft{}, profileConflict(ctx, tx, p.Scope, p.EmployeeID, p.ExpectedRevision)
                }
        }
        if err != nil {
                return domain.ProfileDraft{}, err
        }
        draft := domain.ProfileDraft{TenantID: p.Scope.TenantID(), EmployeeID: p.EmployeeID, Revision: revision, Manifest: p.Manifest, Content: p.Content, ActorID: p.Scope.PrincipalID(), UpdatedAt: p.Now}
        if err := releaseAudit(ctx, tx, p.Scope, p.EmployeeID, p.AuditID, p.CorrelationID, "profile_draft_saved", p.ExpectedRevision, revision, p.Now); err != nil {
                return domain.ProfileDraft{}, err
        }
        payload, _ := json.Marshal(map[string]any{"employee_id": p.EmployeeID, "profile_revision": revision, "manifest_digest": p.Manifest.Digest})
        if err := releaseOutbox(ctx, tx, p.Scope, p.EmployeeID, p.EventID, p.CorrelationID, "employee.profile_draft_saved", revision, payload, p.Now); err != nil {
                return domain.ProfileDraft{}, err
        }
        return draft, tx.Commit(ctx)
}

// ApproveProfile appends the immutable approved version: the draft row is
// locked FOR UPDATE, the expected revision is enforced (exactly one version
// wins a race) and the approved revision is stamped together with the bump —
// the very revision whose content was approved.
func (s *Postgres) ApproveProfile(ctx context.Context, p port.ProfileApproveParams) (domain.ProfileVersion, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        draft, err := scanProfileDraft(tx.QueryRow(ctx, "select "+profileDraftColumns+" from employee.operating_profile_drafts where tenant_id=$1 and employee_id=$2 for update", p.Scope.TenantID(), p.EmployeeID))
        if errors.Is(err, domain.ErrNotFound) {
                return domain.ProfileVersion{}, domain.ProfileRuleError{Section: "draft", Field: "draft", Reason: "Черновик профиля не существует: утверждение невозможно."}
        }
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        if draft.Revision != p.ExpectedRevision {
                return domain.ProfileVersion{}, domain.ProfileConflictError{Expected: p.ExpectedRevision, Current: draft.Revision, CurrentContent: &draft.Content}
        }
        var version int64
        if err = tx.QueryRow(ctx, "select coalesce(max(version),0)+1 from employee.operating_profiles where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), p.EmployeeID).Scan(&version); err != nil {
                return domain.ProfileVersion{}, err
        }
        contentJSON, err := json.Marshal(p.Content)
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        rolesJSON, err := json.Marshal(p.ApproverRoles)
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        refsJSON, err := json.Marshal(p.EvidenceRefs)
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        approved := domain.ProfileVersion{
                TenantID: p.Scope.TenantID(), EmployeeID: p.EmployeeID, Version: version,
                Manifest: p.Manifest, Content: p.Content, Digest: p.Digest,
                ApproverID: p.Scope.PrincipalID(), ApproverRoles: p.ApproverRoles, Description: p.Description,
                EvidenceRefs: p.EvidenceRefs, StepUpVerified: true, ApprovedAt: p.Now,
        }
        if _, err = tx.Exec(ctx, "insert into employee.operating_profiles("+profileVersionColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)",
                approved.TenantID, approved.EmployeeID, approved.Version, approved.Manifest.ManifestID, approved.Manifest.ConfigVersion, approved.Manifest.Digest,
                approved.Digest, contentJSON, approved.ApproverID, rolesJSON, approved.StepUpVerified, approved.Description, refsJSON, approved.ApprovedAt); err != nil {
                return domain.ProfileVersion{}, err
        }
        // The revision bump and the approval stamp are one transaction: the
        // content of the new revision is exactly the approved content, so
        // revision == approved_revision right after the approval.
        if _, err = tx.Exec(ctx, "update employee.operating_profile_drafts set revision=revision+1, approved_revision=revision+1, actor_id=$3, updated_at=$4 where tenant_id=$1 and employee_id=$2 and revision=$5",
                p.Scope.TenantID(), p.EmployeeID, p.Scope.PrincipalID(), p.Now, p.ExpectedRevision); err != nil {
                return domain.ProfileVersion{}, err
        }
        if err := releaseAudit(ctx, tx, p.Scope, p.EmployeeID, p.AuditID, p.CorrelationID, "profile_approved", p.ExpectedRevision, draft.Revision+1, p.Now); err != nil {
                return domain.ProfileVersion{}, err
        }
        payload, _ := json.Marshal(map[string]any{"employee_id": p.EmployeeID, "profile_version": version, "digest": p.Digest, "manifest_digest": p.Manifest.Digest})
        if err := releaseOutbox(ctx, tx, p.Scope, p.EmployeeID, p.EventID, p.CorrelationID, "employee.profile_approved", draft.Revision+1, payload, p.Now); err != nil {
                return domain.ProfileVersion{}, err
        }
        return approved, tx.Commit(ctx)
}

// ReadApprovedProfiles returns every approved profile version, newest first.
func (s *Postgres) ReadApprovedProfiles(ctx context.Context, scope tenancy.Context, employeeID id.UUID) ([]domain.ProfileVersion, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+profileVersionColumns+" from employee.operating_profiles where tenant_id=$1 and employee_id=$2 order by version desc", scope.TenantID(), employeeID)
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []domain.ProfileVersion{}
        for rows.Next() {
                version, err := scanProfileVersion(rows)
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

// SaveThresholdDecision appends one immutable threshold version. The UNIQUE
// (tenant, employee, gate, threshold_version) protects the history: a
// concurrent duplicate decision surfaces as an honest conflict.
func (s *Postgres) SaveThresholdDecision(ctx context.Context, p port.ThresholdDecisionParams) (domain.GateThresholdVersion, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return domain.GateThresholdVersion{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        refsJSON, err := json.Marshal(p.EvidenceRefs)
        if err != nil {
                return domain.GateThresholdVersion{}, err
        }
        var supersedes *int64
        if p.Supersedes != nil {
                version := p.Supersedes.ThresholdVersion
                supersedes = &version
        }
        decision := domain.GateThresholdVersion{
                TenantID: p.Scope.TenantID(), EmployeeID: p.EmployeeID, GateID: p.GateID,
                ThresholdVersion: p.ThresholdVersion, Value: p.Value, ActorID: p.Scope.PrincipalID(),
                Decision: p.Decision, EvidenceRefs: p.EvidenceRefs, Supersedes: p.Supersedes, DecidedAt: p.Now,
        }
        if _, err = tx.Exec(ctx, "insert into employee.gate_thresholds("+thresholdColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)",
                decision.TenantID, decision.EmployeeID, decision.GateID, decision.ThresholdVersion, decision.Value, decision.ActorID, decision.Decision, refsJSON, supersedes, decision.DecidedAt); err != nil {
                var pgErr *pgconn.PgError
                if errors.As(err, &pgErr) && pgErr.Code == "23505" {
                        return domain.GateThresholdVersion{}, domain.ProfileConflictError{Expected: p.ThresholdVersion, Current: p.ThresholdVersion}
                }
                return domain.GateThresholdVersion{}, err
        }
        if err := releaseAudit(ctx, tx, p.Scope, p.EmployeeID, p.AuditID, p.CorrelationID, "threshold_decided", p.ThresholdVersion-1, p.ThresholdVersion, p.Now); err != nil {
                return domain.GateThresholdVersion{}, err
        }
        payload, _ := json.Marshal(map[string]any{"employee_id": p.EmployeeID, "gate_id": p.GateID, "threshold_version": p.ThresholdVersion, "value": p.Value})
        if err := releaseOutbox(ctx, tx, p.Scope, p.EmployeeID, p.EventID, p.CorrelationID, "employee.threshold_decided", p.ThresholdVersion, payload, p.Now); err != nil {
                return domain.GateThresholdVersion{}, err
        }
        return decision, tx.Commit(ctx)
}

// ReadThresholdHistory returns every threshold decision of the employee
// (all gates); the supersedes links are reconstructed from the history
// itself so the decided-at of the superseded version is exact.
func (s *Postgres) ReadThresholdHistory(ctx context.Context, scope tenancy.Context, employeeID id.UUID) ([]domain.GateThresholdVersion, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+thresholdColumns+" from employee.gate_thresholds where tenant_id=$1 and employee_id=$2 order by decided_at desc, gate_id", scope.TenantID(), employeeID)
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []domain.GateThresholdVersion{}
        for rows.Next() {
                decision, err := thresholdForScan(rows)
                if err != nil {
                        return nil, err
                }
                out = append(out, decision)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        if err = tx.Commit(ctx); err != nil {
                return nil, err
        }
        relinkThresholdHistory(out)
        return out, nil
}

// relinkThresholdHistory fills the supersedes decided-at from the rows of the
// same gate: the stored link carries the version only.
func relinkThresholdHistory(history []domain.GateThresholdVersion) {
        decidedAt := map[string]time.Time{}
        for _, decision := range history {
                decidedAt[decision.GateID+"\x00"+itoa64(decision.ThresholdVersion)] = decision.DecidedAt
        }
        for index := range history {
                link := history[index].Supersedes
                if link == nil {
                        continue
                }
                if at, ok := decidedAt[history[index].GateID+"\x00"+itoa64(link.ThresholdVersion)]; ok {
                        link.DecidedAt = at
                }
        }
}

// itoa64 is the tiny local int64 formatter.
func itoa64(value int64) string {
        if value == 0 {
                return "0"
        }
        negative := value < 0
        if negative {
                value = -value
        }
        digits := []byte{}
        for value > 0 {
                digits = append([]byte{byte('0' + value%10)}, digits...)
                value /= 10
        }
        if negative {
                return "-" + string(digits)
        }
        return string(digits)
}
