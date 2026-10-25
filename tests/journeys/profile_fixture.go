package main

import (
	"context"
	"time"

	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/employee/packs"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// profileFixture is the in-memory operating profile surface of the journey
// server: the same honest domain functions over fixture state. The manifest
// link verification reads the sealed rows of the release fixture.
type profileFixture struct {
	mu         chan struct{}
	release    *releaseFixture
	draft      *employeeDomain.ProfileDraft
	approved   []employeeDomain.ProfileVersion
	thresholds []employeeDomain.GateThresholdVersion
}

func newProfileFixture(release *releaseFixture) *profileFixture {
	lock := make(chan struct{}, 1)
	lock <- struct{}{}
	return &profileFixture{mu: lock, release: release}
}

func (f *profileFixture) lock()   { <-f.mu }
func (f *profileFixture) unlock() { f.mu <- struct{}{} }

// manifestLink finds the exact sealed manifest row the link points at.
func (f *profileFixture) manifestLink(link employeeDomain.ManifestLink) (employeeDomain.ManifestLink, employeeDomain.ManifestContent, bool) {
	for _, manifest := range f.release.manifests {
		if manifest.ManifestID == link.ManifestID && manifest.ConfigVersion == link.ConfigVersion {
			return employeeDomain.ManifestLink{ManifestID: manifest.ManifestID, ConfigVersion: manifest.ConfigVersion, Digest: manifest.Digest}, manifest.Content, true
		}
	}
	return employeeDomain.ManifestLink{}, employeeDomain.ManifestContent{}, false
}

// packJourneys derives the journeys from the manifest's pack reference.
func (f *profileFixture) packJourneys(content employeeDomain.ManifestContent) []string {
	pack := content.Config.Refs.Pack
	if pack.Status != employeeDomain.RefPresent {
		return nil
	}
	if manifest, ok := packs.Get(pack.ID, pack.Version); ok {
		return manifest.Journeys
	}
	return nil
}

// backupMemberID is the second active member of the fixture tenant (the
// production-world precedent: the readiness backup). The two-role PASS
// gates of the registry (AG-2, AG-4 — distinct principals, no role
// coincidence) need a second member to walk the full approver discipline.
var backupMemberID = id.MustParse("01994f64-6c00-7000-8000-00000000000c")

// memberIDs mirrors the fixture identity: the owner and the backup member.
func profileMemberIDs() map[id.UUID]bool {
	return map[id.UUID]bool{principalID: true, backupMemberID: true}
}

// ProfileView composes the page projection with the same derivations.
func (f *profileFixture) ProfileView(_ context.Context, _ domain.Principal, _, employee id.UUID) (employeeAPI.ProfileView, error) {
	f.lock()
	defer f.unlock()
	if employee != fixtureEmployeeID {
		return employeeAPI.ProfileView{}, employeeDomain.ErrNotFound
	}
	view := employeeAPI.ProfileView{EmployeeID: employee, CanManage: true, ViewScope: employeeAPI.ProfileScopeManage,
		Readiness: []employeeDomain.ProfileRuleError{}, RouteStates: []employeeDomain.RouteState{},
		Matrix: []employeeDomain.MatrixCellState{}, Thresholds: []employeeDomain.ThresholdState{}}
	if f.draft != nil {
		draft := *f.draft
		view.Draft = &draft
	}
	if len(f.approved) > 0 {
		approved := f.approved[0]
		view.Approved = &approved
	}
	if len(f.release.manifests) > 0 {
		latest := f.release.manifests[0]
		link := employeeDomain.ManifestLink{ManifestID: latest.ManifestID, ConfigVersion: latest.ConfigVersion, Digest: latest.Digest}
		view.LatestManifest = &link
	}
	var content *employeeDomain.ProfileContent
	var link employeeDomain.ManifestLink
	if view.Draft != nil {
		content = &view.Draft.Content
		link = view.Draft.Manifest
	} else if view.Approved != nil {
		content = &view.Approved.Content
		link = view.Approved.Manifest
	}
	if content != nil {
		journeys := []string{}
		packReferenced := false
		if _, manifestContent, ok := f.manifestLink(link); ok {
			journeys = f.packJourneys(manifestContent)
			packReferenced = manifestContent.Config.Refs.Pack.Status == employeeDomain.RefPresent
		}
		view.RouteStates = employeeDomain.DeriveRouteStates(*content)
		view.Matrix = employeeDomain.DeriveMatrixCoverage(*content, journeys)
		view.Thresholds = employeeDomain.DeriveThresholdStates(*content, f.thresholds)
		view.Readiness = employeeDomain.ValidateProfileContent(*content, employeeDomain.ProfileFacts{MemberIDs: profileMemberIDs(), PackJourneys: journeys, PackReferenced: packReferenced})
	}
	view.Applicability = employeeDomain.DeriveProfileApplicability(view.Approved, view.Draft, view.LatestManifest)
	return view, nil
}

// UpdateDraft verifies the exact manifest link and saves the draft: invalid
// present values reject the save, missing pieces save honestly.
func (f *profileFixture) UpdateDraft(_ context.Context, c employeeAPI.ProfileDraftCommand) (employeeDomain.ProfileDraft, error) {
	f.lock()
	defer f.unlock()
	if c.EmployeeID != fixtureEmployeeID {
		return employeeDomain.ProfileDraft{}, employeeDomain.ErrNotFound
	}
	stored, manifestContent, ok := f.manifestLink(c.Manifest)
	if !ok {
		return employeeDomain.ProfileDraft{}, employeeDomain.ProfileRuleError{Section: "manifest", Field: "link", Reason: "Манифест (manifest_id + config_version) не найден среди запечатанных: черновик профиля не сохраняется — привязка только к точной запечатанной версии.", Kind: employeeDomain.ProfileProblemInvalid}
	}
	if stored.Digest != c.Manifest.Digest {
		return employeeDomain.ProfileDraft{}, employeeDomain.ProfileRuleError{Section: "manifest", Field: "digest", Reason: "Digest ссылки не совпадает с digest запечатанного манифеста: ссылка фиксирует точную версию, «текущее состояние» не подставляется.", Kind: employeeDomain.ProfileProblemInvalid}
	}
	_ = manifestContent
	problems := employeeDomain.ValidateProfileContent(c.Content, employeeDomain.ProfileFacts{MemberIDs: profileMemberIDs(), PackJourneys: f.packJourneysOfLink(c.Manifest), PackReferenced: f.linkPackReferenced(c.Manifest)})
	if employeeDomain.HasInvalidProfileProblems(problems) {
		return employeeDomain.ProfileDraft{}, profileFixtureRejection(problems)
	}
	now := time.Now().UTC()
	switch {
	case f.draft == nil && c.ExpectedRevision == 0:
		f.draft = &employeeDomain.ProfileDraft{TenantID: tenantID, EmployeeID: fixtureEmployeeID, Revision: 1, Manifest: c.Manifest, Content: c.Content, ActorID: c.Actor.ID, UpdatedAt: now}
	case f.draft == nil:
		return employeeDomain.ProfileDraft{}, employeeDomain.ProfileConflictError{Expected: c.ExpectedRevision, Current: 0}
	case c.ExpectedRevision != f.draft.Revision:
		content := f.draft.Content
		return employeeDomain.ProfileDraft{}, employeeDomain.ProfileConflictError{Expected: c.ExpectedRevision, Current: f.draft.Revision, CurrentContent: &content}
	default:
		approved := f.draft.ApprovedRevision
		f.draft = &employeeDomain.ProfileDraft{TenantID: tenantID, EmployeeID: fixtureEmployeeID, Revision: f.draft.Revision + 1, ApprovedRevision: approved, Manifest: c.Manifest, Content: c.Content, ActorID: c.Actor.ID, UpdatedAt: now}
	}
	return *f.draft, nil
}

// linkPackReferenced reports whether the manifest of the link carries a
// pack reference (the vacuous-completeness guard needs the exact fact).
func (f *profileFixture) linkPackReferenced(link employeeDomain.ManifestLink) bool {
	if _, content, ok := f.manifestLink(link); ok {
		return content.Config.Refs.Pack.Status == employeeDomain.RefPresent
	}
	return false
}

// packJourneysOfLink resolves the journeys for one manifest link.
func (f *profileFixture) packJourneysOfLink(link employeeDomain.ManifestLink) []string {
	if _, content, ok := f.manifestLink(link); ok {
		return f.packJourneys(content)
	}
	return nil
}

// ApproveProfile: step-up is required (the resolver fixture controls the
// flag); the readiness list must be free of invalid/missing problems.
func (f *profileFixture) ApproveProfile(_ context.Context, c employeeAPI.ProfileApproveCommand) (employeeDomain.ProfileVersion, error) {
	if !c.Actor.StepUpVerified {
		return employeeDomain.ProfileVersion{}, employeeDomain.ErrProfileStepUpRequired
	}
	f.lock()
	defer f.unlock()
	if c.EmployeeID != fixtureEmployeeID || f.draft == nil {
		return employeeDomain.ProfileVersion{}, employeeDomain.ErrNotFound
	}
	if err := employeeDomain.ValidateProfileDescription(c.Description); err != nil {
		return employeeDomain.ProfileVersion{}, err
	}
	if c.ExpectedRevision != f.draft.Revision {
		content := f.draft.Content
		return employeeDomain.ProfileVersion{}, employeeDomain.ProfileConflictError{Expected: c.ExpectedRevision, Current: f.draft.Revision, CurrentContent: &content}
	}
	_, manifestContent, ok := f.manifestLink(f.draft.Manifest)
	if !ok {
		return employeeDomain.ProfileVersion{}, employeeDomain.ProfileRuleError{Section: "manifest", Field: "link", Reason: "Манифест черновика больше не читается: утверждение невозможно.", Kind: employeeDomain.ProfileProblemInvalid}
	}
	problems := employeeDomain.ValidateProfileContent(f.draft.Content, employeeDomain.ProfileFacts{MemberIDs: profileMemberIDs(), PackJourneys: f.packJourneys(manifestContent), PackReferenced: manifestContent.Config.Refs.Pack.Status == employeeDomain.RefPresent})
	if employeeDomain.HasBlockingProfileProblems(problems) {
		return employeeDomain.ProfileVersion{}, profileFixtureRejection(problems)
	}
	digest, err := employeeDomain.ProfileDigest(f.draft.Content)
	if err != nil {
		return employeeDomain.ProfileVersion{}, err
	}
	version := employeeDomain.ProfileVersion{
		TenantID: tenantID, EmployeeID: fixtureEmployeeID, Version: int64(len(f.approved) + 1),
		Manifest: f.draft.Manifest, Content: f.draft.Content, Digest: digest,
		ApproverID: c.Actor.ID, ApproverRoles: []string{"owner"}, Description: c.Description,
		EvidenceRefs: c.EvidenceRefs, StepUpVerified: true, ApprovedAt: time.Now().UTC(),
	}
	f.approved = append([]employeeDomain.ProfileVersion{version}, f.approved...)
	// The approval bumps the revision together with the approved stamp: the
	// content of the new revision is exactly the approved content.
	revision := f.draft.Revision + 1
	f.draft = &employeeDomain.ProfileDraft{TenantID: tenantID, EmployeeID: fixtureEmployeeID, Revision: revision, ApprovedRevision: &revision, Manifest: f.draft.Manifest, Content: f.draft.Content, ActorID: c.Actor.ID, UpdatedAt: time.Now().UTC()}
	return version, nil
}

// DecideThreshold: only the declared evidence owner of the gate in the last
// approved version decides; the history grows append-only.
func (f *profileFixture) DecideThreshold(_ context.Context, c employeeAPI.ThresholdCommand) (employeeDomain.GateThresholdVersion, error) {
	f.lock()
	defer f.unlock()
	if c.EmployeeID != fixtureEmployeeID || len(f.approved) == 0 {
		return employeeDomain.GateThresholdVersion{}, employeeDomain.ErrNotFound
	}
	if err := employeeDomain.ValidateThresholdDecision(c.Value, c.Decision, c.EvidenceRefs); err != nil {
		return employeeDomain.GateThresholdVersion{}, err
	}
	approved := f.approved[0]
	var declared *employeeDomain.GateDeclaration
	for index := range approved.Content.Gates {
		if approved.Content.Gates[index].GateID == c.GateID {
			declared = &approved.Content.Gates[index]
			break
		}
	}
	if declared == nil {
		return employeeDomain.GateThresholdVersion{}, employeeDomain.ProfileRuleError{Section: "gate_threshold", Field: "gate_id", Reason: "Gate не объявлен в последней утверждённой версии профиля: порог можно решать только по объявленному gate.", Kind: employeeDomain.ProfileProblemInvalid}
	}
	if declared.EvidenceOwnerID != c.Actor.ID {
		return employeeDomain.GateThresholdVersion{}, employeeDomain.ProfileRuleError{Section: "gate_threshold", Field: "evidence_owner", Reason: "Порог решает только участник, объявленный evidence owner'ом gate в последней утверждённой версии профиля: решение человеческое, подмена владельца не допускается.", Kind: employeeDomain.ProfileProblemInvalid}
	}
	next := int64(1)
	var supersedes *employeeDomain.ThresholdLink
	for _, decision := range f.thresholds {
		if decision.GateID != c.GateID {
			continue
		}
		if decision.ThresholdVersion >= next {
			next = decision.ThresholdVersion + 1
		}
		if supersedes == nil || decision.ThresholdVersion > supersedes.ThresholdVersion {
			decidedAt := decision.DecidedAt
			supersedes = &employeeDomain.ThresholdLink{ThresholdVersion: decision.ThresholdVersion, DecidedAt: decidedAt}
		}
	}
	decision := employeeDomain.GateThresholdVersion{
		TenantID: tenantID, EmployeeID: fixtureEmployeeID, GateID: c.GateID, ThresholdVersion: next,
		Value: c.Value, ActorID: c.Actor.ID, Decision: c.Decision, EvidenceRefs: c.EvidenceRefs,
		Supersedes: supersedes, DecidedAt: time.Now().UTC(),
	}
	f.thresholds = append(f.thresholds, decision)
	return decision, nil
}

// ThresholdHistory returns the decisions of one gate, newest first.
func (f *profileFixture) ThresholdHistory(_ context.Context, _ domain.Principal, _, employee id.UUID, gateID string) (employeeAPI.ThresholdHistoryView, error) {
	f.lock()
	defer f.unlock()
	if employee != fixtureEmployeeID {
		return employeeAPI.ThresholdHistoryView{}, employeeDomain.ErrNotFound
	}
	view := employeeAPI.ThresholdHistoryView{EmployeeID: employee, GateID: gateID, Decisions: []employeeAPI.ThresholdDecisionView{}}
	for index := len(f.thresholds) - 1; index >= 0; index-- {
		decision := f.thresholds[index]
		if decision.GateID != gateID {
			continue
		}
		view.Decisions = append(view.Decisions, employeeAPI.ThresholdDecisionView{
			GateID: decision.GateID, ThresholdVersion: decision.ThresholdVersion, Value: decision.Value,
			Decision: decision.Decision, ActorID: decision.ActorID, EvidenceRefs: decision.EvidenceRefs,
			Supersedes: decision.Supersedes, DecidedAt: decision.DecidedAt,
		})
	}
	return view, nil
}

// profileFixtureRejection flattens the honest problem list.
func profileFixtureRejection(problems []employeeDomain.ProfileRuleError) employeeDomain.ProfileRuleError {
	if len(problems) == 0 {
		return employeeDomain.ProfileRuleError{Section: "profile", Field: "content", Reason: "Профиль не готов."}
	}
	first := problems[0]
	return employeeDomain.ProfileRuleError{Section: first.Section, Field: first.Field, Reason: first.Reason, Kind: first.Kind, Problems: problems}
}
