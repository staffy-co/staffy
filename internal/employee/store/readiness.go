package store

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/employee/packs"
	"github.com/staffy-co/staffy/internal/employee/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const gateInstanceColumns = "tenant_id,employee_id,instance_id,definition_id,route_channel,route_direction,modality,state,approved_by,evidence_refs,evidence_hash,not_applicable_reason,tested_at,expiry_trigger,supersedes_instance_id,actor_id,recorded_at"

const admissionColumns = "tenant_id,employee_id,decision_no,stage,manifest_id,config_version,manifest_digest,profile_version,profile_digest,routes,actor_id,approver_roles,step_up_verified,decided_at,digest"

// scanGateInstance reads one lifecycle record. The route scope columns are
// both null or both present (the migration CHECK); the supersedes link
// carries the instance id only.
func scanGateInstance(r row) (domain.GateInstance, error) {
	instance := domain.GateInstance{}
	var approvedBy, evidenceRefs []byte
	var routeChannel, routeDirection, modality, evidenceHash, notApplicableReason, expiryTrigger *string
	var testedAt *time.Time
	var supersedes *id.UUID
	err := r.Scan(&instance.TenantID, &instance.EmployeeID, &instance.InstanceID, &instance.DefinitionID,
		&routeChannel, &routeDirection, &modality, &instance.State,
		&approvedBy, &evidenceRefs, &evidenceHash, &notApplicableReason, &testedAt, &expiryTrigger,
		&supersedes, &instance.ActorID, &instance.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GateInstance{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.GateInstance{}, err
	}
	if routeChannel != nil && routeDirection != nil {
		instance.Route = &domain.Route{Channel: *routeChannel, Direction: *routeDirection}
	}
	if modality != nil {
		instance.Modality = *modality
	}
	if evidenceHash != nil {
		instance.EvidenceHash = *evidenceHash
	}
	if notApplicableReason != nil {
		instance.NotApplicableReason = *notApplicableReason
	}
	if expiryTrigger != nil {
		instance.ExpiryTrigger = *expiryTrigger
	}
	// The same UTC canonicalization for the chain records (a non-UTC host
	// keeps identical ordering and identical serializations).
	instance.RecordedAt = instance.RecordedAt.UTC()
	if instance.TestedAt != nil {
		utc := instance.TestedAt.UTC()
		instance.TestedAt = &utc
	}

	instance.TestedAt = testedAt
	instance.SupersedesInstanceID = supersedes
	if len(approvedBy) > 0 {
		if err := json.Unmarshal(approvedBy, &instance.ApprovedBy); err != nil {
			return domain.GateInstance{}, err
		}
	}
	if len(evidenceRefs) > 0 {
		if err := json.Unmarshal(evidenceRefs, &instance.EvidenceRefs); err != nil {
			return domain.GateInstance{}, err
		}
	}
	if instance.ApprovedBy == nil {
		instance.ApprovedBy = []domain.GateApproval{}
	}
	if instance.EvidenceRefs == nil {
		instance.EvidenceRefs = []string{}
	}
	return instance, nil
}

// scanAdmission reads one publication decision and re-verifies the
// canonical digest against the stored bytes — a mismatch is an honest
// refusal, never a silent trust.
func scanAdmission(r row) (domain.ReleaseAdmission, error) {
	admission := domain.ReleaseAdmission{}
	var routes, roles []byte
	err := r.Scan(&admission.TenantID, &admission.EmployeeID, &admission.DecisionNo, &admission.Stage,
		&admission.Manifest.ManifestID, &admission.Manifest.ConfigVersion, &admission.Manifest.Digest,
		&admission.ProfileVersion, &admission.ProfileDigest, &routes, &admission.ActorID, &roles,
		&admission.StepUpVerified, &admission.DecidedAt, &admission.Digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ReleaseAdmission{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	if err := json.Unmarshal(routes, &admission.Routes); err != nil {
		return domain.ReleaseAdmission{}, err
	}
	if err := json.Unmarshal(roles, &admission.ApproverRoles); err != nil {
		return domain.ReleaseAdmission{}, err
	}
	if admission.Routes == nil { // The read boundary canonicalizes the timestamp to UTC: pgx decodes
		// timestamptz into the process-local zone, while the canonical digest
		// bytes were rendered in UTC at the write — a non-UTC host must recompute
		// the same digest (R2/N1).
		admission.DecidedAt = admission.DecidedAt.UTC()

		admission.Routes = []domain.RouteAdmission{}
	}
	if admission.ApproverRoles == nil {
		admission.ApproverRoles = []string{}
	}
	recomputed, err := domain.AdmissionDigest(admission)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	if recomputed != admission.Digest {
		return domain.ReleaseAdmission{}, domain.ProfileRuleError{Section: "publication", Field: "digest", Reason: "Digest решения №" + strconv.FormatInt(admission.DecisionNo, 10) + " не совпал с пересчитанным значением: целостность записи нарушена, молчаливое доверие исключено."}
	}
	return admission, nil
}

// SaveGateInstance appends one lifecycle record. The partial UNIQUE index on
// supersedes keeps exactly one successor per record: a raced transition of
// the same predecessor surfaces as an honest GateConflictError.
func (s *Postgres) SaveGateInstance(ctx context.Context, p port.GateInstanceParams) (domain.GateInstance, error) {
	// The same microsecond truncation keeps the recorded_at ordering of the
	// append-only chain stable between the write and every later read.
	p.Now = p.Now.Truncate(time.Microsecond)
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.GateInstance{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	approvedBy, err := json.Marshal(p.Spec.ApprovedBy)
	if err != nil {
		return domain.GateInstance{}, err
	}
	evidenceRefs, err := json.Marshal(p.Spec.EvidenceRefs)
	if err != nil {
		return domain.GateInstance{}, err
	}
	var routeChannel, routeDirection *string
	if p.Spec.Route != nil {
		channel, direction := p.Spec.Route.Channel, p.Spec.Route.Direction
		routeChannel, routeDirection = &channel, &direction
	}
	var evidenceHash, notApplicableReason, expiryTrigger *string
	if p.Spec.EvidenceHash != "" {
		hash := p.Spec.EvidenceHash
		evidenceHash = &hash
	}
	if p.Spec.NotApplicableReason != "" {
		reason := p.Spec.NotApplicableReason
		notApplicableReason = &reason
	}
	if p.Spec.ExpiryTrigger != "" {
		trigger := p.Spec.ExpiryTrigger
		expiryTrigger = &trigger
	}
	var testedAt *time.Time
	if p.Spec.TargetState == domain.GateStatePass {
		testedAt = &p.Now
	}
	var modality *string
	if p.Spec.Modality != "" {
		value := p.Spec.Modality
		modality = &value
	}
	instance := domain.GateInstance{
		TenantID: p.Scope.TenantID(), EmployeeID: p.EmployeeID, InstanceID: p.InstanceID,
		DefinitionID: p.Spec.DefinitionID, Route: p.Spec.Route, Modality: p.Spec.Modality,
		State: p.Spec.TargetState, ApprovedBy: p.Spec.ApprovedBy, EvidenceRefs: p.Spec.EvidenceRefs,
		NotApplicableReason: p.Spec.NotApplicableReason, TestedAt: testedAt,
		ExpiryTrigger: p.Spec.ExpiryTrigger, SupersedesInstanceID: p.Spec.Supersedes,
		ActorID: p.Scope.PrincipalID(), RecordedAt: p.Now,
	}
	if p.Spec.EvidenceHash != "" {
		instance.EvidenceHash = p.Spec.EvidenceHash
	}
	if _, err = tx.Exec(ctx, "insert into employee.gate_instances("+gateInstanceColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)",
		instance.TenantID, instance.EmployeeID, instance.InstanceID, instance.DefinitionID,
		routeChannel, routeDirection, modality, instance.State,
		approvedBy, evidenceRefs, evidenceHash, notApplicableReason, testedAt, expiryTrigger,
		instance.SupersedesInstanceID, instance.ActorID, instance.RecordedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			conflictID := instance.InstanceID
			if instance.SupersedesInstanceID != nil {
				conflictID = *instance.SupersedesInstanceID
			}
			return domain.GateInstance{}, domain.GateConflictError{InstanceID: conflictID,
				Reason: "переход этого инстанса уже применён конкурентной командой: ровно один преемник, перечитайте состояние."}
		}
		return domain.GateInstance{}, err
	}
	if err := releaseAudit(ctx, tx, p.Scope, p.EmployeeID, p.AuditID, p.CorrelationID, "gate_recorded", 0, 1, p.Now); err != nil {
		return domain.GateInstance{}, err
	}
	payload, _ := json.Marshal(map[string]any{"employee_id": p.EmployeeID, "instance_id": p.InstanceID, "definition_id": instance.DefinitionID, "state": instance.State})
	if err := releaseOutbox(ctx, tx, p.Scope, p.EmployeeID, p.EventID, p.CorrelationID, "employee.gate_recorded", 1, payload, p.Now); err != nil {
		return domain.GateInstance{}, err
	}
	return instance, tx.Commit(ctx)
}

// ListGateInstances returns every lifecycle record, newest first — the
// derivations pick the chain heads and the supersedes history stays visible.
func (s *Postgres) ListGateInstances(ctx context.Context, scope tenancy.Context, employeeID id.UUID) ([]domain.GateInstance, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, "select "+gateInstanceColumns+" from employee.gate_instances where tenant_id=$1 and employee_id=$2 order by recorded_at desc, instance_id desc", scope.TenantID(), employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.GateInstance{}
	for rows.Next() {
		instance, err := scanGateInstance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, instance)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

// ListReleaseAdmissions returns every publication decision, newest first;
// the active pointer is derived (the highest decision number).
func (s *Postgres) ListReleaseAdmissions(ctx context.Context, scope tenancy.Context, employeeID id.UUID) ([]domain.ReleaseAdmission, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, "select "+admissionColumns+" from employee.release_admissions where tenant_id=$1 and employee_id=$2 order by decision_no desc", scope.TenantID(), employeeID)
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

// SaveReleaseAdmission records the controlled publication atomically: the
// employee aggregate row is locked, the in-module facts (approved profile,
// sealed manifest, gate instances, admission history) are re-read IN the
// transaction, the derivation re-runs over them together with the
// cross-module facts carried by the params (entitlement admission, current
// connections, pack journeys), the stage ladder is enforced against the
// exact candidate digests and the decision is appended with the monotonic
// number. A raced or repeated publication of the same candidate and stage
// surfaces as an honest AdmissionConflictError — exactly one record.
func (s *Postgres) SaveReleaseAdmission(ctx context.Context, p port.AdmissionParams) (domain.ReleaseAdmission, error) {
	// Postgres timestamptz keeps microseconds: the decision time is truncated
	// BEFORE the canonical digest is computed and BEFORE the insert, so the
	// digest of the stored row (recomputed on every read as an integrity
	// guard) always matches the digest computed at the write — a
	// nanosecond-precision production clock never breaks the read path.
	p.Now = p.Now.Truncate(time.Microsecond)
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// The aggregate lock: concurrent publications serialize here; the
	// second command re-derives after the first commits and honestly
	// conflicts instead of writing a second record.
	if _, err = tx.Exec(ctx, "select 1 from employee.drafts where tenant_id=$1 and employee_id=$2 for update", p.Scope.TenantID(), p.EmployeeID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ReleaseAdmission{}, domain.ErrNotFound
		}
		return domain.ReleaseAdmission{}, err
	}
	// In-transaction facts: the latest approved profile.
	approved, err := scanProfileVersion(tx.QueryRow(ctx, "select "+profileVersionColumns+" from employee.operating_profiles where tenant_id=$1 and employee_id=$2 order by version desc limit 1", p.Scope.TenantID(), p.EmployeeID))
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ReleaseAdmission{}, domain.ProfileRuleError{Section: "publication", Field: "profile", Reason: "Утверждённой версии Operating Profile нет: публикация невозможна (обязательный элемент отсутствует)."}
	}
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	// The sealed manifest the approved profile pins, with the honest
	// payload read (schema check, strict decode, digest re-computation).
	var payload []byte
	var storedDigest string
	manifest := domain.ReleaseManifest{}
	err = tx.QueryRow(ctx, "select tenant_id,employee_id,manifest_id,config_version,schema_version,status,digest,actor_id,sealed_at,content from employee.release_manifests where tenant_id=$1 and employee_id=$2 and config_version=$3",
		p.Scope.TenantID(), p.EmployeeID, approved.Manifest.ConfigVersion).
		Scan(&manifest.TenantID, &manifest.EmployeeID, &manifest.ManifestID, &manifest.ConfigVersion, &manifest.SchemaVersion, &manifest.Status, &storedDigest, &manifest.ActorID, &manifest.SealedAt, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ReleaseAdmission{}, domain.ProfileRuleError{Section: "publication", Field: "manifest", Reason: "Запечатанный манифест утверждённого профиля не найден: привязка только к точной запечатанной версии."}
	}
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	manifest.Digest = storedDigest
	read := domain.ReadManifestPayload(payload, storedDigest)
	// The latest sealed manifest (the applicability derivation).
	var latestVersion int64
	var latestManifestID id.UUID
	var latestDigest string
	latestLink := (*domain.ManifestLink)(nil)
	if err := tx.QueryRow(ctx, "select config_version, manifest_id, digest from employee.release_manifests where tenant_id=$1 and employee_id=$2 order by config_version desc limit 1",
		p.Scope.TenantID(), p.EmployeeID).Scan(&latestVersion, &latestManifestID, &latestDigest); err == nil {
		latestLink = &domain.ManifestLink{ManifestID: latestManifestID, ConfigVersion: latestVersion, Digest: latestDigest}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return domain.ReleaseAdmission{}, err
	}
	// The gate instance history.
	instances := []domain.GateInstance{}
	instanceRows, err := tx.Query(ctx, "select "+gateInstanceColumns+" from employee.gate_instances where tenant_id=$1 and employee_id=$2", p.Scope.TenantID(), p.EmployeeID)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	for instanceRows.Next() {
		instance, err := scanGateInstance(instanceRows)
		if err != nil {
			instanceRows.Close()
			return domain.ReleaseAdmission{}, err
		}
		instances = append(instances, instance)
	}
	if err = instanceRows.Err(); err != nil {
		instanceRows.Close()
		return domain.ReleaseAdmission{}, err
	}
	instanceRows.Close()
	// The admission history (the acting pointer).
	admissions := []domain.ReleaseAdmission{}
	admissionRows, err := tx.Query(ctx, "select "+admissionColumns+" from employee.release_admissions where tenant_id=$1 and employee_id=$2 order by decision_no desc", p.Scope.TenantID(), p.EmployeeID)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	for admissionRows.Next() {
		admission, err := scanAdmission(admissionRows)
		if err != nil {
			admissionRows.Close()
			return domain.ReleaseAdmission{}, err
		}
		admissions = append(admissions, admission)
	}
	if err = admissionRows.Err(); err != nil {
		admissionRows.Close()
		return domain.ReleaseAdmission{}, err
	}
	admissionRows.Close()
	// The derivation re-runs over the in-transaction facts.
	packJourneys := p.PackJourneys
	if read.Status == domain.QuarantineNone && read.Content.Config.Refs.Pack.Status == domain.RefPresent {
		if manifestPack, ok := packs.Get(read.Content.Config.Refs.Pack.ID, read.Content.Config.Refs.Pack.Version); ok {
			packJourneys = manifestPack.Journeys
		}
	}
	facts := domain.ReadinessFacts{
		Approved:           &approved,
		ManifestRead:       &read,
		LatestManifest:     latestLink,
		CurrentConnections: p.CurrentConnections,
		Entitlement:        p.Entitlement,
		PackJourneys:       packJourneys,
		Instances:          instances,
	}
	report := domain.DeriveReadiness(facts, p.Stage)
	candidate := domain.CandidateKey{Manifest: approved.Manifest, ProfileVersion: approved.Version, ProfileDigest: approved.Digest}
	// The full history travels into the ladder check: the candidate's own
	// decisions stay acting for the ladder regardless of interleaved
	// candidates (the list is ordered by decision_no desc, the acting
	// pointer is the first row).
	if err := domain.ValidateStageProgression(p.Stage, candidate, admissions); err != nil {
		return domain.ReleaseAdmission{}, err
	}
	// The publication exists only in the ready state: a missing element, a
	// blocked element (the entitlement-at-synthetic exception stays inside
	// the derivation) or zero admitted routes each refuse honestly with the
	// derivation's own status text — never behind an aggregate.
	if report.Status != domain.ReadinessReady {
		return domain.ReleaseAdmission{}, admissionRejection(report, report.StatusText)
	}
	// The monotonic number under the aggregate lock: the globally highest
	// decision of the employee (the list is ordered by decision_no desc).
	decisionNo := int64(1)
	if len(admissions) > 0 {
		decisionNo = admissions[0].DecisionNo + 1
	}
	acting := (*domain.ReleaseAdmission)(nil)
	if len(admissions) > 0 {
		acting = &admissions[0]
	}
	admission := domain.ReleaseAdmission{
		TenantID: p.Scope.TenantID(), EmployeeID: p.EmployeeID, DecisionNo: decisionNo, Stage: p.Stage,
		Manifest: approved.Manifest, ProfileVersion: approved.Version, ProfileDigest: approved.Digest,
		Routes:  append([]domain.RouteAdmission(nil), report.Routes...),
		ActorID: p.Scope.PrincipalID(), ApproverRoles: p.ApproverRoles, StepUpVerified: true, DecidedAt: p.Now,
	}
	sort.SliceStable(admission.Routes, func(i, j int) bool { return admission.Routes[i].Route.String() < admission.Routes[j].Route.String() })
	digest, err := domain.AdmissionDigest(admission)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	admission.Digest = digest
	routesJSON, err := json.Marshal(admission.Routes)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	rolesJSON, err := json.Marshal(admission.ApproverRoles)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	if _, err = tx.Exec(ctx, "insert into employee.release_admissions("+admissionColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)",
		admission.TenantID, admission.EmployeeID, admission.DecisionNo, admission.Stage,
		admission.Manifest.ManifestID, admission.Manifest.ConfigVersion, admission.Manifest.Digest,
		admission.ProfileVersion, admission.ProfileDigest, routesJSON, admission.ActorID, rolesJSON,
		admission.StepUpVerified, admission.DecidedAt, admission.Digest); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ReleaseAdmission{}, domain.AdmissionConflictError{Stage: p.Stage, Current: acting}
		}
		return domain.ReleaseAdmission{}, err
	}
	if err := releaseAudit(ctx, tx, p.Scope, p.EmployeeID, p.AuditID, p.CorrelationID, "release_published", decisionNo-1, decisionNo, p.Now); err != nil {
		return domain.ReleaseAdmission{}, err
	}
	payloadOut, _ := json.Marshal(map[string]any{"employee_id": p.EmployeeID, "decision_no": decisionNo, "stage": p.Stage, "digest": digest, "manifest_digest": admission.Manifest.Digest, "profile_digest": admission.ProfileDigest})
	if err := releaseOutbox(ctx, tx, p.Scope, p.EmployeeID, p.EventID, p.CorrelationID, "employee.release_published", decisionNo, payloadOut, p.Now); err != nil {
		return domain.ReleaseAdmission{}, err
	}
	return admission, tx.Commit(ctx)
}

// admissionRejection flattens the honest derivation into one rule rejection
// carrying the per-route reasons — the refusal never hides behind an
// aggregate status.
func admissionRejection(report domain.ReadinessReport, summary string) domain.ProfileRuleError {
	problems := []domain.ProfileRuleError{}
	for _, element := range report.Elements {
		if element.State == domain.ElementMissing || element.State == domain.ElementBlocked {
			problems = append(problems, domain.ProfileRuleError{Section: "readiness." + element.Key, Field: "state", Reason: element.Title + ": " + element.Reason, Kind: domain.ProfileProblemMissing})
		}
	}
	for _, route := range report.Routes {
		for _, block := range route.Reasons {
			problems = append(problems, domain.ProfileRuleError{Section: "route." + route.Route.String(), Field: block.Cause, Reason: block.Reason, Kind: domain.ProfileProblemAdvisory})
		}
	}
	reason := summary + " (" + report.StatusText + ")"
	if len(problems) > 0 {
		reason = summary + ": " + problems[0].Reason
	}
	return domain.ProfileRuleError{Section: "publication", Field: "readiness", Reason: reason, Problems: problems}
}
