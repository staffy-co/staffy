package app

import (
	"context"
	"errors"

	"github.com/staffy-co/staffy/internal/employee/api"
	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/employee/port"
	entitlementDomain "github.com/staffy-co/staffy/internal/entitlement/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// EntitlementFactsSource is the narrow caller-authority fact port to the
// entitlement module (the ConnectionsFacts/MembersFacts precedent): the
// readiness surface has already verified the actor's scope, so the
// admission read does not re-require it. The concrete entitlement service
// satisfies it structurally.
type EntitlementFactsSource interface {
	AdmissionView(context.Context, identityDomain.Principal, id.UUID) (entitlementDomain.AdmissionState, error)
}

// WithEntitlementFacts wires the narrow entitlement admission fact port
// (readiness derivation facts). Without it the readiness view and the
// publication honestly refuse instead of assuming an "ok" perimeter.
func (s *Service) WithEntitlementFacts(entitlement EntitlementFactsSource) *Service {
	s.entitlement = entitlement
	return s
}

// readinessRepository returns the readiness persistence when the repository
// supports it (the postgres store does).
func (s *Service) readinessRepository() port.ReadinessRepository {
	if repository, ok := s.repository.(port.ReadinessRepository); ok {
		return repository
	}
	return nil
}

// readinessManageScope authorizes the commands of the surface: gate
// transitions and the publication require the configuration scope. A
// member without the scope gets an honest refusal (403); a non-member a
// plain 404 so tenant existence is never disclosed.
func (s *Service) readinessManageScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return tenancy.Context{}, err
	}
	if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Configuration); err != nil {
		if errors.Is(err, identityDomain.ErrForbidden) {
			return tenancy.Context{}, domain.ErrReadinessForbidden
		}
		if errors.Is(err, identityDomain.ErrNotFound) {
			return tenancy.Context{}, domain.ErrNotFound
		}
		return tenancy.Context{}, err
	}
	return scope, nil
}

// readinessViewScope authorizes viewing: configuration (manage), review or
// audit (view only). Everyone else — member or not — is refused honestly.
func (s *Service) readinessViewScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, bool, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, false, domain.ErrNotFound
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return tenancy.Context{}, false, err
	}
	memberWithoutScope := false
	infraFailure := error(nil)
	for _, candidate := range []identityDomain.Scope{identityDomain.Configuration, identityDomain.Review, identityDomain.Audit} {
		err := s.identity.Authorize(ctx, actor, tenantID, candidate)
		if err == nil {
			return scope, candidate == identityDomain.Configuration, nil
		}
		if errors.Is(err, identityDomain.ErrForbidden) {
			memberWithoutScope = true
			continue
		}
		if errors.Is(err, identityDomain.ErrNotFound) {
			continue
		}
		infraFailure = err
	}
	if infraFailure != nil {
		return tenancy.Context{}, false, infraFailure
	}
	if memberWithoutScope {
		return tenancy.Context{}, false, domain.ErrReadinessForbidden
	}
	return tenancy.Context{}, false, domain.ErrNotFound
}

// entitlementAdmissionFact reads the derived entitlement admission state
// through the narrow fact port under the already verified scope. A missing
// port is an honest refusal — the derivation never assumes "ok".
func (s *Service) entitlementAdmissionFact(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (domain.AdmissionFact, error) {
	if s.entitlement == nil {
		return domain.AdmissionFact{}, domain.ProfileRuleError{Section: "entitlement", Field: "admission_fact", Reason: "Факт-порт entitlement не сконфигурирован: readiness честно отклоняется, состояние «ok» не предполагается."}
	}
	state, err := s.entitlement.AdmissionView(ctx, actor, tenantID)
	if err != nil {
		if errors.Is(err, entitlementDomain.ErrForbidden) {
			return domain.AdmissionFact{}, domain.ErrReadinessForbidden
		}
		if errors.Is(err, entitlementDomain.ErrNotFound) {
			return domain.AdmissionFact{}, domain.ErrNotFound
		}
		return domain.AdmissionFact{}, err
	}
	return domain.AdmissionFact{Status: state.Status, Reason: state.Reason}, nil
}

// currentConnectionFacts adapts the current connection facts for the
// readiness derivation (drift detection against the sealed snapshot): the
// identifiers, states and the derived capability facts — never secrets.
func (s *Service) currentConnectionFacts(ctx context.Context, scope tenancy.Context) ([]domain.ConnectionFacts, error) {
	facts, err := s.connectorFacts(ctx, scope)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ConnectionFacts, 0, len(facts))
	for _, fact := range facts {
		out = append(out, domain.ConnectionFacts{ConnectionID: fact.ConnectionID, Status: fact.Status, Capabilities: fact.Capabilities})
	}
	return out, nil
}

// readinessFacts gathers the live facts of the derivation: the approved
// profile, the sealed manifest it pins (with the honest payload read), the
// latest sealed manifest, the gate instance history, the entitlement
// admission fact and the current connection facts. Nothing is cached —
// every read re-derives.
func (s *Service) readinessFacts(ctx context.Context, scope tenancy.Context, actor identityDomain.Principal, tenantID, employeeID id.UUID) (domain.ReadinessFacts, error) {
	profile := s.profileRepository()
	release := s.releaseRepository()
	readiness := s.readinessRepository()
	if profile == nil || release == nil || readiness == nil {
		return domain.ReadinessFacts{}, domain.ErrNotFound
	}
	versions, err := profile.ReadApprovedProfiles(ctx, scope, employeeID)
	if err != nil {
		return domain.ReadinessFacts{}, err
	}
	facts := domain.ReadinessFacts{}
	if len(versions) > 0 {
		approved := versions[0]
		facts.Approved = &approved
	}
	manifests, err := release.ListManifests(ctx, scope, employeeID)
	if err != nil {
		return domain.ReadinessFacts{}, err
	}
	var latest *port.ManifestRecord
	for index := range manifests {
		if latest == nil || manifests[index].Manifest.ConfigVersion > latest.Manifest.ConfigVersion {
			latest = &manifests[index]
		}
	}
	if latest != nil {
		link := domain.ManifestLink{ManifestID: latest.Manifest.ManifestID, ConfigVersion: latest.Manifest.ConfigVersion, Digest: latest.Manifest.Digest}
		facts.LatestManifest = &link
	}
	if facts.Approved != nil {
		for index := range manifests {
			record := manifests[index]
			if record.Manifest.ManifestID == facts.Approved.Manifest.ManifestID && record.Manifest.ConfigVersion == facts.Approved.Manifest.ConfigVersion {
				read := domain.ReadManifestPayload(record.Payload, record.Manifest.Digest)
				facts.ManifestRead = &read
				facts.PackJourneys = packJourneysOfManifest(record)
				break
			}
		}
	}
	facts.Instances, err = readiness.ListGateInstances(ctx, scope, employeeID)
	if err != nil {
		return domain.ReadinessFacts{}, err
	}
	facts.Entitlement, err = s.entitlementAdmissionFact(ctx, actor, tenantID)
	if err != nil {
		return domain.ReadinessFacts{}, err
	}
	facts.CurrentConnections, err = s.currentConnectionFacts(ctx, scope)
	if err != nil {
		return domain.ReadinessFacts{}, err
	}
	return facts, nil
}

// ReadinessView composes the dry-run projection for the requested stage
// (empty stage defaults to the only available next step of the ladder).
// The view never records anything — it is the StagePreview of the spec.
func (s *Service) ReadinessView(ctx context.Context, actor identityDomain.Principal, tenantID, employeeID id.UUID, stage string) (api.ReadinessView, error) {
	scope, canManage, err := s.readinessViewScope(ctx, actor, tenantID)
	if err != nil {
		return api.ReadinessView{}, err
	}
	readiness := s.readinessRepository()
	if readiness == nil {
		return api.ReadinessView{}, domain.ErrNotFound
	}
	if _, err := s.repository.Read(ctx, scope, employeeID); err != nil {
		return api.ReadinessView{}, err
	}
	admissions, err := readiness.ListReleaseAdmissions(ctx, scope, employeeID)
	if err != nil {
		return api.ReadinessView{}, err
	}
	view := api.ReadinessView{EmployeeID: employeeID, CanManage: canManage, ViewScope: api.ReadinessScopeView, History: admissions}
	if canManage {
		view.ViewScope = api.ReadinessScopeManage
	}
	if stage == "" {
		stage = api.ReadinessDefaultStage(domain.DeriveActiveRelease(admissions))
	}
	facts, err := s.readinessFacts(ctx, scope, actor, tenantID, employeeID)
	if err != nil {
		return api.ReadinessView{}, err
	}
	view.Stage = stage
	view.Report = domain.DeriveReadiness(facts, stage)
	view.Active = domain.DeriveActiveRelease(admissions)
	if facts.Approved != nil {
		view.Applicability = domain.DeriveProfileApplicability(facts.Approved, nil, facts.LatestManifest)
	}
	return view, nil
}

// GateInstancesView lists the fixed platform dictionary and every lifecycle
// record of the employee with the supersedes history visible.
func (s *Service) GateInstancesView(ctx context.Context, actor identityDomain.Principal, tenantID, employeeID id.UUID) (api.GateInstancesView, error) {
	scope, canManage, err := s.readinessViewScope(ctx, actor, tenantID)
	if err != nil {
		return api.GateInstancesView{}, err
	}
	readiness := s.readinessRepository()
	if readiness == nil {
		return api.GateInstancesView{}, domain.ErrNotFound
	}
	if _, err := s.repository.Read(ctx, scope, employeeID); err != nil {
		return api.GateInstancesView{}, err
	}
	instances, err := readiness.ListGateInstances(ctx, scope, employeeID)
	if err != nil {
		return api.GateInstancesView{}, err
	}
	view := api.GateInstancesView{EmployeeID: employeeID, Definitions: domain.GateDictionary(), Instances: instances, CanManage: canManage, ViewScope: api.ReadinessScopeView}
	if canManage {
		view.ViewScope = api.ReadinessScopeManage
	}
	return view, nil
}

// RecordGateTransition appends one gate lifecycle record: step-up is
// required (the single rule for every state change), the PASS discipline
// is validated against the dictionary and the participants must be active
// members of the tenant (the org truth of the role labels stays a human
// governance fact).
func (s *Service) RecordGateTransition(ctx context.Context, c api.GateTransitionCommand) (domain.GateInstance, error) {
	if !c.Actor.StepUpVerified {
		return domain.GateInstance{}, domain.ErrReadinessStepUpRequired
	}
	scope, err := s.readinessManageScope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return domain.GateInstance{}, err
	}
	readiness := s.readinessRepository()
	if readiness == nil {
		return domain.GateInstance{}, domain.ErrNotFound
	}
	if _, err := s.repository.Read(ctx, scope, c.EmployeeID); err != nil {
		return domain.GateInstance{}, err
	}
	history, err := readiness.ListGateInstances(ctx, scope, c.EmployeeID)
	if err != nil {
		return domain.GateInstance{}, err
	}
	spec, err := domain.ValidateGateTransition(domain.GateTransitionSpec{
		DefinitionID: c.DefinitionID, Route: c.Route, Modality: c.Modality, TargetState: c.TargetState,
		Supersedes: c.Supersedes, ApprovedBy: c.ApprovedBy, EvidenceRefs: c.EvidenceRefs, EvidenceHash: c.EvidenceHash,
		NotApplicableReason: c.NotApplicableReason, ExpiryTrigger: c.ExpiryTrigger,
	}, history)
	if err != nil {
		return domain.GateInstance{}, err
	}
	// The named approvers must exist as active members of the tenant — the
	// structural check the system owns; who really holds the registry role
	// label is the human governance fact (the honest note).
	if len(spec.ApprovedBy) > 0 {
		memberIDs, _, err := s.memberFacts(ctx, c.Actor, c.TenantID)
		if err != nil {
			return domain.GateInstance{}, err
		}
		for _, approval := range spec.ApprovedBy {
			if !memberIDs[approval.ParticipantID] {
				return domain.GateInstance{}, domain.ProfileRuleError{Section: "gate_instance", Field: "approved_by", Reason: "Approver не является активным участником Tenant: система проверяет существование участников и полноту покрытия ролей, орг-истина роли — governance-факт человека.", Kind: domain.ProfileProblemInvalid}
			}
		}
	}
	now := s.clock.Now().UTC()
	instanceID, err := s.ids.New(now)
	if err != nil {
		return domain.GateInstance{}, err
	}
	eventID, err := s.ids.New(now)
	if err != nil {
		return domain.GateInstance{}, err
	}
	auditID, err := s.ids.New(now)
	if err != nil {
		return domain.GateInstance{}, err
	}
	return readiness.SaveGateInstance(ctx, port.GateInstanceParams{
		Scope: scope, EmployeeID: c.EmployeeID, InstanceID: instanceID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID,
		Spec: spec, Now: now,
	})
}

// PublishAdmission records the controlled publication decision: step-up is
// required, the derivation re-runs atomically in the store transaction
// over the in-transaction facts together with the cross-module facts read
// here (entitlement admission, current connections), the stage ladder is
// enforced against the exact candidate digests and the per-route results
// are sealed into the append-only record. A publication with zero admitted
// routes never exists.
func (s *Service) PublishAdmission(ctx context.Context, c api.PublishAdmissionCommand) (domain.ReleaseAdmission, error) {
	if !c.Actor.StepUpVerified {
		return domain.ReleaseAdmission{}, domain.ErrReadinessStepUpRequired
	}
	scope, err := s.readinessManageScope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	readiness := s.readinessRepository()
	if readiness == nil {
		return domain.ReleaseAdmission{}, domain.ErrNotFound
	}
	if _, err := s.repository.Read(ctx, scope, c.EmployeeID); err != nil {
		return domain.ReleaseAdmission{}, err
	}
	// The cross-module facts travel with the command: the entitlement
	// admission state and the current connection facts read immediately
	// before under the same verified actor (the derivation re-runs over
	// them inside the transaction together with the in-module facts).
	entitlementFact, err := s.entitlementAdmissionFact(ctx, c.Actor, c.TenantID)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	currentConnections, err := s.currentConnectionFacts(ctx, scope)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	_, roles, err := s.memberFacts(ctx, c.Actor, c.TenantID)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	approverRoles := append([]string(nil), roles[c.Actor.ID]...)
	// The pack journeys of the approved manifest's pack reference — derived
	// here for the in-transaction derivation (the manifest row is re-read
	// and re-verified inside the transaction).
	profile := s.profileRepository()
	if profile == nil {
		return domain.ReleaseAdmission{}, domain.ErrNotFound
	}
	versions, err := profile.ReadApprovedProfiles(ctx, scope, c.EmployeeID)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	packJourneys := []string(nil)
	if len(versions) > 0 {
		record, err := s.manifestRecord(ctx, scope, c.EmployeeID, versions[0].Manifest)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return domain.ReleaseAdmission{}, err
		}
		if err == nil {
			packJourneys = packJourneysOfManifest(record)
		}
	}
	now := s.clock.Now().UTC()
	eventID, err := s.ids.New(now)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	auditID, err := s.ids.New(now)
	if err != nil {
		return domain.ReleaseAdmission{}, err
	}
	return readiness.SaveReleaseAdmission(ctx, port.AdmissionParams{
		Scope: scope, EmployeeID: c.EmployeeID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID,
		Stage: c.Stage, Entitlement: entitlementFact, CurrentConnections: currentConnections,
		PackJourneys: packJourneys, ApproverRoles: approverRoles, Now: now,
	})
}

// AdmissionHistoryView returns the append-only publication history with the
// acting decision derived (the highest decision number — there is no
// rollback, the pointer grows monotonically).
func (s *Service) AdmissionHistoryView(ctx context.Context, actor identityDomain.Principal, tenantID, employeeID id.UUID) (api.AdmissionHistoryView, error) {
	scope, _, err := s.readinessViewScope(ctx, actor, tenantID)
	if err != nil {
		return api.AdmissionHistoryView{}, err
	}
	readiness := s.readinessRepository()
	if readiness == nil {
		return api.AdmissionHistoryView{}, domain.ErrNotFound
	}
	if _, err := s.repository.Read(ctx, scope, employeeID); err != nil {
		return api.AdmissionHistoryView{}, err
	}
	admissions, err := readiness.ListReleaseAdmissions(ctx, scope, employeeID)
	if err != nil {
		return api.AdmissionHistoryView{}, err
	}
	return api.AdmissionHistoryView{EmployeeID: employeeID, Active: domain.DeriveActiveRelease(admissions), Admissions: admissions}, nil
}
