package main

import (
	"context"
	"encoding/json"
	"time"

	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/employee/packs"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// releaseFixture is the in-memory release surface of the journey server:
// the same honest domain functions over fixture state.
type releaseFixture struct {
	mu         chan struct{}
	employee   *employeeFixture
	connectors *connectorFixture
	draft      *employeeDomain.ReleaseDraft
	versions   []employeeDomain.ConfigVersion
	manifests  []employeeDomain.ReleaseManifest
	raw        map[int64][]byte
}

func newReleaseFixture(employee *employeeFixture, connectors *connectorFixture) *releaseFixture {
	lock := make(chan struct{}, 1)
	lock <- struct{}{}
	return &releaseFixture{mu: lock, employee: employee, connectors: connectors, raw: map[int64][]byte{}}
}

func (f *releaseFixture) lock()   { <-f.mu }
func (f *releaseFixture) unlock() { f.mu <- struct{}{} }

// knowledgeSources returns the fixture knowledge list (nil-safe).
func (f *releaseFixture) knowledgeSources() []employeeDomain.KnowledgeSource {
	if f.employee.knowledge == nil {
		return nil
	}
	return []employeeDomain.KnowledgeSource{*f.employee.knowledge}
}

// ReleaseView composes the page projection with honest reads.
func (f *releaseFixture) ReleaseView(_ context.Context, _ domain.Principal, _, employee id.UUID) (employeeAPI.ReleaseView, error) {
	f.lock()
	defer f.unlock()
	if employee != fixtureEmployeeID {
		return employeeAPI.ReleaseView{}, employeeDomain.ErrNotFound
	}
	view := employeeAPI.ReleaseView{EmployeeID: employee, CanManage: true, ViewScope: employeeAPI.ReleaseScopeManage, Versions: []employeeDomain.ConfigVersion{}, Manifests: []employeeAPI.ManifestSummary{}}
	if f.draft != nil {
		draft := *f.draft
		view.Draft = &draft
	}
	view.Versions = append([]employeeDomain.ConfigVersion(nil), f.versions...)
	for _, manifest := range f.manifests {
		view.Manifests = append(view.Manifests, employeeAPI.ManifestSummary{
			ConfigVersion: manifest.ConfigVersion, ManifestID: manifest.ManifestID, Status: manifest.Status,
			Digest: manifest.Digest, SchemaVersion: manifest.SchemaVersion, SealedAt: manifest.SealedAt,
		})
	}
	if len(f.manifests) > 0 {
		manifest := fixtureManifestView(f.manifests[0], f.raw[f.manifests[0].ConfigVersion])
		view.Latest = &manifest
	}
	return view, nil
}

func fixtureManifestView(manifest employeeDomain.ReleaseManifest, payload []byte) employeeAPI.ManifestView {
	read := employeeDomain.ReadManifestPayload(payload, manifest.Digest)
	view := employeeAPI.ManifestView{
		ConfigVersion: manifest.ConfigVersion, ManifestID: manifest.ManifestID, Status: manifest.Status,
		Digest: manifest.Digest, SchemaVersion: read.SchemaVersion, SealedAt: manifest.SealedAt,
		ActorID: manifest.ActorID, Quarantine: read.Status, QuarantineReason: read.Reason, Raw: string(read.Raw),
	}
	if read.Status == employeeDomain.QuarantineNone {
		view.Content = &read.Content
	}
	return view
}

// ReadReleaseManifest reads one manifest by configuration version.
func (f *releaseFixture) ReadReleaseManifest(_ context.Context, _ domain.Principal, _, employee id.UUID, configVersion int64) (employeeAPI.ManifestView, error) {
	f.lock()
	defer f.unlock()
	if employee != fixtureEmployeeID {
		return employeeAPI.ManifestView{}, employeeDomain.ErrNotFound
	}
	for _, manifest := range f.manifests {
		if manifest.ConfigVersion == configVersion {
			return fixtureManifestView(manifest, f.raw[configVersion]), nil
		}
	}
	return employeeAPI.ManifestView{}, employeeDomain.ErrNotFound
}

// AssembleDraft builds the refs from the current fixture facts.
func (f *releaseFixture) AssembleDraft(_ context.Context, c employeeAPI.AssembleCommand) (employeeDomain.ReleaseDraft, error) {
	f.lock()
	defer f.unlock()
	if c.EmployeeID != fixtureEmployeeID || f.employee.draft == nil {
		return employeeDomain.ReleaseDraft{}, employeeDomain.ErrNotFound
	}
	refs := employeeDomain.AssembleConfigRefs(employeeDomain.ConfigFacts{
		Brief: f.employee.draft.Brief, DraftVersion: f.employee.draft.Version, Pack: f.employee.draft.Pack,
		Behavior: f.employee.behavior, Knowledge: f.knowledgeSources(), Voice: f.employee.voice,
	})
	draft, err := f.saveDraft(refs, nil, c.ExpectedRevision, c.Actor.ID)
	if err != nil {
		return employeeDomain.ReleaseDraft{}, err
	}
	return draft, nil
}

// saveDraft CAS-upserts the release draft.
func (f *releaseFixture) saveDraft(refs employeeDomain.ConfigRefs, restoredFrom *employeeDomain.ConfigVersionLink, expected int64, actor id.UUID) (employeeDomain.ReleaseDraft, error) {
	now := time.Now().UTC()
	switch {
	case f.draft == nil && expected == 0:
		f.draft = &employeeDomain.ReleaseDraft{TenantID: tenantID, EmployeeID: fixtureEmployeeID, Revision: 1, Refs: refs, RestoredFrom: restoredFrom, ActorID: actor, UpdatedAt: now}
	case f.draft == nil:
		return employeeDomain.ReleaseDraft{}, employeeDomain.ReleaseConflictError{Expected: expected, Current: 0}
	case expected != f.draft.Revision:
		return employeeDomain.ReleaseDraft{}, employeeDomain.ReleaseConflictError{Expected: expected, Current: f.draft.Revision, Refs: f.draft.Refs}
	default:
		f.draft = &employeeDomain.ReleaseDraft{TenantID: tenantID, EmployeeID: fixtureEmployeeID, Revision: f.draft.Revision + 1, Refs: refs, RestoredFrom: restoredFrom, ActorID: actor, UpdatedAt: now}
	}
	return *f.draft, nil
}

// CreateConfigVersion appends the next monotonic version from the draft refs.
func (f *releaseFixture) CreateConfigVersion(_ context.Context, c employeeAPI.VersionCommand) (employeeDomain.ConfigVersion, error) {
	f.lock()
	defer f.unlock()
	if c.EmployeeID != fixtureEmployeeID || f.draft == nil {
		return employeeDomain.ConfigVersion{}, employeeDomain.ErrNotFound
	}
	if err := employeeDomain.ValidateReleaseDescription(c.Description); err != nil {
		return employeeDomain.ConfigVersion{}, err
	}
	if c.ExpectedRevision != f.draft.Revision {
		return employeeDomain.ConfigVersion{}, employeeDomain.ReleaseConflictError{Expected: c.ExpectedRevision, Current: f.draft.Revision, Refs: f.draft.Refs}
	}
	versionID, err := id.NewV7(time.Now().UTC())
	if err != nil {
		return employeeDomain.ConfigVersion{}, err
	}
	version := employeeDomain.ConfigVersion{
		TenantID: tenantID, EmployeeID: fixtureEmployeeID, VersionID: versionID,
		Version: int64(len(f.versions) + 1), Description: c.Description, Refs: f.draft.Refs,
		ActorID: c.Actor.ID, CreatedAt: time.Now().UTC(),
	}
	f.versions = append(f.versions, version)
	if _, err := f.saveDraft(f.draft.Refs, f.draft.RestoredFrom, f.draft.Revision, c.Actor.ID); err != nil {
		return employeeDomain.ConfigVersion{}, err
	}
	return version, nil
}

// SealManifest seals the manifest from the version plus connection facts.
func (f *releaseFixture) SealManifest(_ context.Context, c employeeAPI.SealCommand) (employeeDomain.ReleaseManifest, error) {
	f.lock()
	defer f.unlock()
	if c.EmployeeID != fixtureEmployeeID || f.draft == nil {
		return employeeDomain.ReleaseManifest{}, employeeDomain.ErrNotFound
	}
	if c.ExpectedRevision != f.draft.Revision {
		return employeeDomain.ReleaseManifest{}, employeeDomain.ReleaseConflictError{Expected: c.ExpectedRevision, Current: f.draft.Revision, Refs: f.draft.Refs}
	}
	var config employeeDomain.ConfigVersion
	found := false
	for _, version := range f.versions {
		if version.Version == c.ConfigVersion {
			config, found = version, true
		}
	}
	if !found {
		return employeeDomain.ReleaseManifest{}, employeeDomain.ErrNotFound
	}
	for _, manifest := range f.manifests {
		if manifest.ConfigVersion == c.ConfigVersion {
			return employeeDomain.ReleaseManifest{}, employeeDomain.ErrManifestExists
		}
	}
	connections, err := f.connectorFacts(c.Actor)
	if err != nil {
		return employeeDomain.ReleaseManifest{}, err
	}
	content := employeeDomain.BuildManifestContent(config, connections, nil)
	status := employeeDomain.DeriveManifestStatus(content)
	digest, err := employeeDomain.ManifestDigest(content)
	if err != nil {
		return employeeDomain.ReleaseManifest{}, err
	}
	manifestID, err := id.NewV7(time.Now().UTC())
	if err != nil {
		return employeeDomain.ReleaseManifest{}, err
	}
	raw, err := json.Marshal(content)
	if err != nil {
		return employeeDomain.ReleaseManifest{}, err
	}
	manifest := employeeDomain.ReleaseManifest{
		TenantID: tenantID, EmployeeID: fixtureEmployeeID, ManifestID: manifestID,
		ConfigVersion: c.ConfigVersion, SchemaVersion: content.SchemaVersion, Status: status,
		Digest: digest, ActorID: c.Actor.ID, SealedAt: time.Now().UTC(), Content: content,
	}
	f.manifests = append([]employeeDomain.ReleaseManifest{manifest}, f.manifests...)
	f.raw[c.ConfigVersion] = raw
	if _, err := f.saveDraft(f.draft.Refs, f.draft.RestoredFrom, f.draft.Revision, c.Actor.ID); err != nil {
		return employeeDomain.ReleaseManifest{}, err
	}
	return manifest, nil
}

// connectorFacts gathers the fixture connection facts through the connector
// fixture (the derived states come from the connector domain — one truth).
func (f *releaseFixture) connectorFacts(actor domain.Principal) ([]employeeDomain.ConnectionFact, error) {
	connections, err := f.connectors.List(context.Background(), actor, tenantID)
	if err != nil {
		return nil, err
	}
	facts := make([]employeeDomain.ConnectionFact, 0, len(connections))
	for _, conn := range connections {
		state, err := f.connectors.CapabilityState(context.Background(), actor, tenantID, conn.ConnectionID)
		if err != nil {
			return nil, err
		}
		capabilities := make([]employeeDomain.CapabilityFact, 0, len(state.Capabilities))
		for _, report := range state.Capabilities {
			fact := employeeDomain.CapabilityFact{
				Capability: report.ID, State: report.State, Reason: report.StateReason, Owner: report.ReconciliationOwner,
			}
			if fact.Owner == "" {
				fact.Owner = "Technical Lead"
			}
			if report.Evidence != nil {
				fact.Evidence = &employeeDomain.EvidenceFact{
					EvidenceID: report.Evidence.EvidenceID, Outcome: report.Evidence.Outcome,
					CheckedAt: report.Evidence.CheckedAt, ExpiresAt: report.Evidence.ExpiresAt,
				}
			}
			fact.Gap = report.State != connectorDomain.CapStateVerified
			capabilities = append(capabilities, fact)
		}
		facts = append(facts, employeeDomain.ConnectionFact{
			ConnectionID: conn.ConnectionID, ConnectorID: conn.ConnectorID, ContractVersion: conn.ConnectorVer,
			AdapterBuild: state.AdapterBuild, Environment: string(conn.Environment), Status: conn.Status,
			Capabilities: capabilities,
		})
	}
	return facts, nil
}

// Restore creates a new draft from the selected version with the honest
// compatibility report.
func (f *releaseFixture) Restore(_ context.Context, c employeeAPI.RestoreCommand) (employeeAPI.RestoreResult, error) {
	f.lock()
	defer f.unlock()
	if c.EmployeeID != fixtureEmployeeID {
		return employeeAPI.RestoreResult{}, employeeDomain.ErrNotFound
	}
	var version employeeDomain.ConfigVersion
	found := false
	for _, candidate := range f.versions {
		if candidate.Version == c.ConfigVersion {
			version, found = candidate, true
		}
	}
	if !found {
		return employeeAPI.RestoreResult{}, employeeDomain.ErrNotFound
	}
	var sealed *employeeDomain.ManifestContent
	if raw, ok := f.raw[c.ConfigVersion]; ok {
		read := employeeDomain.ReadManifestPayload(raw, f.digestOf(c.ConfigVersion))
		if read.Status == employeeDomain.QuarantineNone {
			content := read.Content
			sealed = &content
		}
	}
	connections, err := f.connectorFacts(c.Actor)
	if err != nil {
		return employeeAPI.RestoreResult{}, err
	}
	current := make([]employeeDomain.ConnectionFacts, 0, len(connections))
	for _, fact := range connections {
		current = append(current, employeeDomain.ConnectionFacts{ConnectionID: fact.ConnectionID, Status: fact.Status, Capabilities: fact.Capabilities})
	}
	packAvailable := true
	if version.Refs.Pack.Status == employeeDomain.RefPresent {
		_, packAvailable = packs.Get(version.Refs.Pack.ID, version.Refs.Pack.Version)
	}
	knowledge := map[id.UUID]int64{}
	for _, source := range f.knowledgeSources() {
		knowledge[source.SourceID] = source.CurrentVersion
	}
	var behaviorVersion, voiceVersion int64
	for _, v := range f.employee.behavior {
		if v.Version > behaviorVersion {
			behaviorVersion = v.Version
		}
	}
	for _, v := range f.employee.voice {
		if v.Version > voiceVersion {
			voiceVersion = v.Version
		}
	}
	draftVersion := int64(0)
	if f.employee.draft != nil {
		draftVersion = f.employee.draft.Version
	}
	issues := employeeDomain.RestoreCompatibility(version.Refs, employeeDomain.RestoreFacts{
		DraftVersion: draftVersion, PackAvailable: packAvailable,
		BehaviorVersion: behaviorVersion, VoiceVersion: voiceVersion,
		Knowledge: knowledge, Connections: current, Sealed: sealed,
	})
	link := employeeDomain.ConfigVersionLink{Version: version.Version, VersionID: version.VersionID, CreatedAt: version.CreatedAt}
	draft, err := f.saveDraft(version.Refs, &link, c.ExpectedRevision, c.Actor.ID)
	if err != nil {
		return employeeAPI.RestoreResult{}, err
	}
	return employeeAPI.RestoreResult{Draft: draft, Issues: issues}, nil
}

func (f *releaseFixture) digestOf(configVersion int64) string {
	for _, manifest := range f.manifests {
		if manifest.ConfigVersion == configVersion {
			return manifest.Digest
		}
	}
	return ""
}
