package main

// readiness_fixture.go — the Story 1-14 fixture of the journey server: the
// test & release surface over fixture state with the same honest domain
// functions as production (DeriveReadiness, ValidateGateTransition,
// ValidateStageProgression, AdmissionDigest). The approved profile comes from
// the profile fixture, the sealed manifests and the current connection facts
// from the release fixture, the entitlement admission fact from the
// entitlement fixture; the gate instances and the publication decisions live
// here, append-only.

import (
	"context"
	"fmt"
	"time"

	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

type readinessFixture struct {
	mu          chan struct{}
	profile     *profileFixture
	release     *releaseFixture
	entitlement *entitlementFixture
	instances   []employeeDomain.GateInstance
	admissions  []employeeDomain.ReleaseAdmission
	nextID      int
}

func newReadinessFixture(profile *profileFixture, release *releaseFixture, entitlement *entitlementFixture) *readinessFixture {
	lock := make(chan struct{}, 1)
	lock <- struct{}{}
	return &readinessFixture{mu: lock, profile: profile, release: release, entitlement: entitlement, nextID: 200}
}

func (f *readinessFixture) lock()   { <-f.mu }
func (f *readinessFixture) unlock() { f.mu <- struct{}{} }

// newFixtureUUID derives a deterministic unique fixture identifier.
func (f *readinessFixture) newFixtureUUID() id.UUID {
	f.nextID++
	return id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", f.nextID))
}

// currentConnectionFacts adapts the fixture connection facts for the
// derivation (the same projection as production: identifiers, states and
// derived capabilities — never secrets).
func (f *readinessFixture) currentConnectionFacts() ([]employeeDomain.ConnectionFacts, error) {
	facts, err := f.release.connectorFacts()
	if err != nil {
		return nil, err
	}
	out := make([]employeeDomain.ConnectionFacts, 0, len(facts))
	for _, fact := range facts {
		out = append(out, employeeDomain.ConnectionFacts{ConnectionID: fact.ConnectionID, Status: fact.Status, Capabilities: fact.Capabilities})
	}
	return out, nil
}

// facts gathers the live derivation facts of the fixture world.
func (f *readinessFixture) facts() (employeeDomain.ReadinessFacts, error) {
	facts := employeeDomain.ReadinessFacts{}
	f.profile.lock()
	if len(f.profile.approved) > 0 {
		approved := f.profile.approved[0]
		facts.Approved = &approved
	}
	f.profile.unlock()
	if len(f.release.manifests) > 0 {
		latest := f.release.manifests[0]
		link := employeeDomain.ManifestLink{ManifestID: latest.ManifestID, ConfigVersion: latest.ConfigVersion, Digest: latest.Digest}
		facts.LatestManifest = &link
	}
	if facts.Approved != nil {
		if raw, ok := f.release.raw[facts.Approved.Manifest.ConfigVersion]; ok {
			digest := f.release.digestOf(facts.Approved.Manifest.ConfigVersion)
			read := employeeDomain.ReadManifestPayload(raw, digest)
			facts.ManifestRead = &read
			facts.PackJourneys = f.profile.packJourneysOfLink(facts.Approved.Manifest)
		}
	}
	facts.Instances = append([]employeeDomain.GateInstance(nil), f.instances...)
	var err error
	facts.CurrentConnections, err = f.currentConnectionFacts()
	if err != nil {
		return employeeDomain.ReadinessFacts{}, err
	}
	state, err := f.entitlement.AdmissionView(context.Background(), fixturePrincipal(), tenantID)
	if err != nil {
		return employeeDomain.ReadinessFacts{}, err
	}
	facts.Entitlement = employeeDomain.AdmissionFact{Status: state.Status, Reason: state.Reason}
	return facts, nil
}

// ReadinessView composes the dry-run projection for the requested stage.
func (f *readinessFixture) ReadinessView(_ context.Context, _ domain.Principal, _, employee id.UUID, stage string) (employeeAPI.ReadinessView, error) {
	f.lock()
	defer f.unlock()
	if employee != fixtureEmployeeID {
		return employeeAPI.ReadinessView{}, employeeDomain.ErrNotFound
	}
	view := employeeAPI.ReadinessView{EmployeeID: employee, CanManage: true, ViewScope: employeeAPI.ReadinessScopeManage,
		History: append([]employeeDomain.ReleaseAdmission(nil), f.admissions...)}
	if stage == "" {
		stage = employeeAPI.ReadinessDefaultStage(employeeDomain.DeriveActiveRelease(f.admissions))
	}
	facts, err := f.facts()
	if err != nil {
		return employeeAPI.ReadinessView{}, err
	}
	view.Stage = stage
	view.Report = employeeDomain.DeriveReadiness(facts, stage)
	view.Active = employeeDomain.DeriveActiveRelease(f.admissions)
	if facts.Approved != nil {
		view.Applicability = employeeDomain.DeriveProfileApplicability(facts.Approved, nil, facts.LatestManifest)
	}
	return view, nil
}

// GateInstancesView lists the fixed dictionary and every lifecycle record.
func (f *readinessFixture) GateInstancesView(_ context.Context, _ domain.Principal, _, employee id.UUID) (employeeAPI.GateInstancesView, error) {
	f.lock()
	defer f.unlock()
	if employee != fixtureEmployeeID {
		return employeeAPI.GateInstancesView{}, employeeDomain.ErrNotFound
	}
	return employeeAPI.GateInstancesView{EmployeeID: employee, Definitions: employeeDomain.GateDictionary(),
		Instances: append([]employeeDomain.GateInstance(nil), f.instances...), CanManage: true, ViewScope: employeeAPI.ReadinessScopeManage}, nil
}

// RecordGateTransition validates the transition against the real domain
// state machine and appends the record.
func (f *readinessFixture) RecordGateTransition(_ context.Context, c employeeAPI.GateTransitionCommand) (employeeDomain.GateInstance, error) {
	if !c.Actor.StepUpVerified {
		return employeeDomain.GateInstance{}, employeeDomain.ErrReadinessStepUpRequired
	}
	f.lock()
	defer f.unlock()
	if c.EmployeeID != fixtureEmployeeID {
		return employeeDomain.GateInstance{}, employeeDomain.ErrNotFound
	}
	spec, err := employeeDomain.ValidateGateTransition(employeeDomain.GateTransitionSpec{
		DefinitionID: c.DefinitionID, Route: c.Route, Modality: c.Modality, TargetState: c.TargetState,
		Supersedes: c.Supersedes, ApprovedBy: c.ApprovedBy, EvidenceRefs: c.EvidenceRefs, EvidenceHash: c.EvidenceHash,
		NotApplicableReason: c.NotApplicableReason, ExpiryTrigger: c.ExpiryTrigger,
	}, f.instances)
	if err != nil {
		return employeeDomain.GateInstance{}, err
	}
	if len(spec.ApprovedBy) > 0 {
		memberIDs := profileMemberIDs()
		for _, approval := range spec.ApprovedBy {
			if !memberIDs[approval.ParticipantID] {
				return employeeDomain.GateInstance{}, employeeDomain.ProfileRuleError{Section: "gate_instance", Field: "approved_by", Reason: "Approver не является активным участником Tenant: система проверяет существование участников и полноту покрытия ролей, орг-истина роли — governance-факт человека.", Kind: employeeDomain.ProfileProblemInvalid}
			}
		}
	}
	instance := employeeDomain.GateInstance{
		TenantID: tenantID, EmployeeID: fixtureEmployeeID, InstanceID: f.newFixtureUUID(),
		DefinitionID: spec.DefinitionID, Route: spec.Route, Modality: spec.Modality, State: spec.TargetState,
		ApprovedBy: spec.ApprovedBy, EvidenceRefs: spec.EvidenceRefs, EvidenceHash: spec.EvidenceHash,
		NotApplicableReason: spec.NotApplicableReason, ExpiryTrigger: spec.ExpiryTrigger,
		SupersedesInstanceID: spec.Supersedes, ActorID: c.Actor.ID,
		// The monotonic record time: every appended instance is one
		// second later than the previous (the fixed clock of the fixture
		// world would stamp every supersedes chain link identically —
		// the newest-instance derivations read strictly-later times, so
		// the chain head must be strictly newest; the Story 4.6 order
		// contour exposed the ambiguity: its policy gate read the first
		// link of the pilot's AG-4 chain as the current state).
		RecordedAt: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC).Add(time.Duration(len(f.instances)) * time.Second),
	}
	if spec.TargetState == employeeDomain.GateStatePass {
		tested := time.Date(2026, 10, 8, 11, 0, 0, 0, time.UTC)
		instance.TestedAt = &tested
	}
	f.instances = append(f.instances, instance)
	return instance, nil
}

// PublishAdmission re-derives over the live facts, enforces the stage ladder
// and appends the decision with the canonical digest.
func (f *readinessFixture) PublishAdmission(_ context.Context, c employeeAPI.PublishAdmissionCommand) (employeeDomain.ReleaseAdmission, error) {
	if !c.Actor.StepUpVerified {
		return employeeDomain.ReleaseAdmission{}, employeeDomain.ErrReadinessStepUpRequired
	}
	f.lock()
	defer f.unlock()
	if c.EmployeeID != fixtureEmployeeID {
		return employeeDomain.ReleaseAdmission{}, employeeDomain.ErrNotFound
	}
	facts, err := f.facts()
	if err != nil {
		return employeeDomain.ReleaseAdmission{}, err
	}
	report := employeeDomain.DeriveReadiness(facts, c.Stage)
	if report.Status == employeeDomain.ReadinessIncomplete {
		return employeeDomain.ReleaseAdmission{}, admissionFixtureRejection(report, "обязательные элементы не заполнены — публикация невозможна")
	}
	if facts.Approved == nil {
		return employeeDomain.ReleaseAdmission{}, employeeDomain.ProfileRuleError{Section: "publication", Field: "profile", Reason: "Утверждённой версии Operating Profile нет: публикация невозможна.", Kind: employeeDomain.ProfileProblemInvalid}
	}
	candidate := employeeDomain.CandidateKey{Manifest: facts.Approved.Manifest, ProfileVersion: facts.Approved.Version, ProfileDigest: facts.Approved.Digest}
	if err := employeeDomain.ValidateStageProgression(c.Stage, candidate, f.admissions); err != nil {
		return employeeDomain.ReleaseAdmission{}, err
	}
	admitted := 0
	for _, route := range report.Routes {
		if route.State == employeeDomain.RouteAdmitted {
			admitted++
		}
	}
	if admitted == 0 {
		return employeeDomain.ReleaseAdmission{}, admissionFixtureRejection(report, "ни один маршрут не допущен: пустых публикаций не существует")
	}
	decisionNo := int64(1)
	if active := employeeDomain.DeriveActiveRelease(f.admissions); active != nil {
		decisionNo = active.DecisionNo + 1
	}
	admission := employeeDomain.ReleaseAdmission{
		TenantID: tenantID, EmployeeID: fixtureEmployeeID, DecisionNo: decisionNo, Stage: c.Stage,
		Manifest: facts.Approved.Manifest, ProfileVersion: facts.Approved.Version, ProfileDigest: facts.Approved.Digest,
		Routes:  append([]employeeDomain.RouteAdmission(nil), report.Routes...),
		ActorID: c.Actor.ID, ApproverRoles: []string{"owner"}, StepUpVerified: true,
		DecidedAt: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC),
	}
	digest, err := employeeDomain.AdmissionDigest(admission)
	if err != nil {
		return employeeDomain.ReleaseAdmission{}, err
	}
	admission.Digest = digest
	f.admissions = append(f.admissions, admission)
	return admission, nil
}

// AdmissionHistoryView returns the append-only history with the acting
// decision derived.
func (f *readinessFixture) AdmissionHistoryView(_ context.Context, _ domain.Principal, _, employee id.UUID) (employeeAPI.AdmissionHistoryView, error) {
	f.lock()
	defer f.unlock()
	if employee != fixtureEmployeeID {
		return employeeAPI.AdmissionHistoryView{}, employeeDomain.ErrNotFound
	}
	return employeeAPI.AdmissionHistoryView{EmployeeID: employee,
		Active:     employeeDomain.DeriveActiveRelease(f.admissions),
		Admissions: append([]employeeDomain.ReleaseAdmission(nil), f.admissions...)}, nil
}

// PilotAdmission (spec 4-1) composes the tenant-level read-time derivation
// over the SAME fixture fact sources as production: the pack pin read from
// the sealed manifest payloads, the derived capability states of the
// current connections (never secrets) and the gate instance history — the
// same domain function DerivePilotAdmission, no writes.
func (f *readinessFixture) PilotAdmission(_ context.Context, _ domain.Principal, tenant id.UUID) (employeeAPI.PilotAdmissionView, error) {
	f.lock()
	defer f.unlock()
	if tenant != tenantID {
		return employeeAPI.PilotAdmissionView{}, employeeDomain.ErrNotFound
	}
	// (1) The pack pin: the newest sealed manifest payload read honestly
	// (the same ReadManifestPayload as the readiness derivation).
	packID, packVersion := "", ""
	f.release.lock()
	if len(f.release.manifests) > 0 {
		newest := f.release.manifests[0]
		if raw, ok := f.release.raw[newest.ConfigVersion]; ok {
			read := employeeDomain.ReadManifestPayload(raw, f.release.digestOf(newest.ConfigVersion))
			if read.Status == "" && read.Content.Config.Refs.Pack.Status == employeeDomain.RefPresent {
				packID = read.Content.Config.Refs.Pack.ID
				packVersion = read.Content.Config.Refs.Pack.Version
			}
		}
	}
	f.release.unlock()
	// (2) The capability facts: the derived states of the current
	// connections (the same projection as the readiness facts — never
	// secrets).
	capabilities := map[string]employeeDomain.CapabilityFact{}
	connectionFacts, err := f.currentConnectionFacts()
	if err != nil {
		return employeeAPI.PilotAdmissionView{}, err
	}
	for _, connection := range connectionFacts {
		for _, fact := range connection.Capabilities {
			if fact.Capability == "" {
				continue
			}
			current, known := capabilities[fact.Capability]
			if !known || (current.State != "verified" && fact.State == "verified") {
				capabilities[fact.Capability] = fact
			}
		}
	}
	// (3) The gate instance history of the fixture employee.
	report := employeeDomain.DerivePilotAdmission(employeeDomain.PilotFacts{
		PackID: packID, PackVersion: packVersion,
		Capabilities: capabilities,
		Instances:    append([]employeeDomain.GateInstance(nil), f.instances...),
	})
	return employeeAPI.PilotAdmissionView{TenantID: tenant, Report: report, CanManage: true, ViewScope: employeeAPI.ReadinessScopeManage}, nil
}

// admissionFixtureRejection flattens the honest derivation into one rule
// rejection carrying the per-route reasons (the production shape).
func admissionFixtureRejection(report employeeDomain.ReadinessReport, summary string) employeeDomain.ProfileRuleError {
	problems := []employeeDomain.ProfileRuleError{}
	for _, element := range report.Elements {
		if element.State == employeeDomain.ElementMissing || element.State == employeeDomain.ElementBlocked {
			problems = append(problems, employeeDomain.ProfileRuleError{Section: "readiness." + element.Key, Field: "state", Reason: element.Title + ": " + element.Reason, Kind: employeeDomain.ProfileProblemMissing})
		}
	}
	for _, route := range report.Routes {
		if route.State != employeeDomain.RouteAdmitted {
			for _, block := range route.Reasons {
				problems = append(problems, employeeDomain.ProfileRuleError{Section: "readiness.route", Field: block.Cause, Reason: route.Route.String() + ": " + block.Reason, Kind: employeeDomain.ProfileProblemMissing})
			}
		}
	}
	return employeeDomain.ProfileRuleError{Section: "readiness", Field: "readiness", Reason: summary, Kind: employeeDomain.ProfileProblemInvalid, Problems: problems}
}

// fixturePrincipal mirrors the fixture resolver identity for the fact ports.
func fixturePrincipal() domain.Principal {
	return domain.Principal{ID: principalID, CanCreateTenant: true}
}

var _ employeeAPI.ReadinessService = (*readinessFixture)(nil)
