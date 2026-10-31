package app

import (
	"context"
	"errors"

	connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
	"github.com/staffy-co/staffy/internal/employee/api"
	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/employee/packs"
	"github.com/staffy-co/staffy/internal/employee/port"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/staffy-co/staffy/internal/tools"
)

// WithConnectorFacts wires the narrow connector facts port (release
// manifest facts). Without it the release surface stays bound but sealing
// and restoring honestly refuse instead of gathering empty facts.
func (s *Service) WithConnectorFacts(connectors port.ConnectorFacts) *Service {
	s.connectors = connectors
	return s
}

// releaseRepository returns the release persistence when the repository
// supports it (the postgres store does).
func (s *Service) releaseRepository() port.ReleaseRepository {
	if repository, ok := s.repository.(port.ReleaseRepository); ok {
		return repository
	}
	return nil
}

// releaseTenancy validates the identifiers and builds the tenancy context.
func (s *Service) releaseTenancy(actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	return tenancy.New(actor.ID, tenantID)
}

// releaseManageScope authorizes configuration management. A member without
// the scope gets an honest refusal (403); a non-member a plain 404 so
// tenant existence is never disclosed.
func (s *Service) releaseManageScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	scope, err := s.releaseTenancy(actor, tenantID)
	if err != nil {
		return tenancy.Context{}, err
	}
	if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Configuration); err != nil {
		if errors.Is(err, identityDomain.ErrForbidden) {
			return tenancy.Context{}, domain.ErrReleaseForbidden
		}
		if errors.Is(err, identityDomain.ErrNotFound) {
			return tenancy.Context{}, domain.ErrNotFound
		}
		// An infrastructure failure is never masked as a 404.
		return tenancy.Context{}, err
	}
	return scope, nil
}

// releaseViewScope authorizes viewing: configuration (manage), review or
// audit (view only). Everyone else — member or not — is refused honestly.
func (s *Service) releaseViewScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, bool, error) {
	scope, err := s.releaseTenancy(actor, tenantID)
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
		// An infrastructure failure is never masked as a 404 or a 403.
		infraFailure = err
	}
	if infraFailure != nil {
		return tenancy.Context{}, false, infraFailure
	}
	if memberWithoutScope {
		return tenancy.Context{}, false, domain.ErrReleaseForbidden
	}
	return tenancy.Context{}, false, domain.ErrNotFound
}

// loadEmployeeFacts reads the current configuration facts of the employee.
func (s *Service) loadEmployeeFacts(ctx context.Context, scope tenancy.Context, employeeID id.UUID) (domain.Draft, []domain.BehaviorVersion, []domain.KnowledgeSource, []domain.VoiceVersion, error) {
	draft, err := s.repository.Read(ctx, scope, employeeID)
	if err != nil {
		return domain.Draft{}, nil, nil, nil, err
	}
	behavior, err := s.readBehaviorFacts(ctx, scope, employeeID)
	if err != nil {
		return domain.Draft{}, nil, nil, nil, err
	}
	knowledge, err := s.listKnowledgeFacts(ctx, scope, employeeID)
	if err != nil {
		return domain.Draft{}, nil, nil, nil, err
	}
	voice, err := s.readVoiceFacts(ctx, scope, employeeID)
	if err != nil {
		return domain.Draft{}, nil, nil, nil, err
	}
	return draft, behavior, knowledge, voice, nil
}

func (s *Service) readBehaviorFacts(ctx context.Context, scope tenancy.Context, employeeID id.UUID) ([]domain.BehaviorVersion, error) {
	if repository, ok := s.repository.(port.BehaviorRepository); ok {
		return repository.ReadBehavior(ctx, scope, employeeID)
	}
	return nil, nil
}

func (s *Service) listKnowledgeFacts(ctx context.Context, scope tenancy.Context, employeeID id.UUID) ([]domain.KnowledgeSource, error) {
	if repository, ok := s.repository.(port.KnowledgeRepository); ok {
		return repository.ListKnowledge(ctx, scope, employeeID)
	}
	return nil, nil
}

func (s *Service) readVoiceFacts(ctx context.Context, scope tenancy.Context, employeeID id.UUID) ([]domain.VoiceVersion, error) {
	if repository, ok := s.repository.(port.VoiceRepository); ok {
		return repository.ReadVoice(ctx, scope, employeeID)
	}
	return nil, nil
}

// connectorFacts gathers the safe connection facts of the tenant through
// the narrow caller-authority port (the release scope is already verified,
// so the read does not re-require the integrations scope; RLS isolates the
// tenant). A nil port is an honest refusal: sealing or restoring without
// the ability to see connection facts would silently produce a manifest
// that claims facts it never checked.
func (s *Service) connectorFacts(ctx context.Context, scope tenancy.Context) ([]domain.ConnectionFact, error) {
	if s.connectors == nil {
		return nil, domain.ReleaseRuleError{Rule: "connector_facts", Reason: "Порт фактов подключений не сконфигурирован: запечатывание и восстановление честно отклоняются, манифест не собирается вслепую."}
	}
	rows, err := s.connectors.ConnectionsFacts(ctx, scope)
	if err != nil {
		return nil, err
	}
	facts := make([]domain.ConnectionFact, 0, len(rows))
	for _, row := range rows {
		facts = append(facts, domain.ConnectionFact{
			ConnectionID:    row.ConnectionID,
			ConnectorID:     row.ConnectorID,
			ContractVersion: row.ContractVersion,
			AdapterBuild:    row.AdapterBuild,
			Environment:     row.Environment,
			Status:          row.Status,
			Capabilities:    capabilityFacts(row.Capabilities),
		})
	}
	return facts, nil
}

// capabilityFacts adapts the derived connector reports into the manifest
// facts. The states are consumed as derived — never re-derived here.
func capabilityFacts(reports []connectorDomain.CapabilityReport) []domain.CapabilityFact {
	out := make([]domain.CapabilityFact, 0, len(reports))
	for _, report := range reports {
		fact := domain.CapabilityFact{
			Capability: report.ID,
			State:      report.State,
			Reason:     report.StateReason,
			Owner:      report.ReconciliationOwner,
		}
		if fact.Owner == "" {
			fact.Owner = "Technical Lead"
		}
		if report.Evidence != nil {
			fact.Evidence = &domain.EvidenceFact{
				EvidenceID: report.Evidence.EvidenceID,
				Outcome:    report.Evidence.Outcome,
				CheckedAt:  report.Evidence.CheckedAt,
				ExpiresAt:  report.Evidence.ExpiresAt,
			}
		}
		fact.Gap = report.State != connectorDomain.CapStateVerified
		out = append(out, fact)
	}
	return out
}

// ReleaseView composes the release projection: the mutable draft, the
// append-only version history and the latest manifest read honestly
// (schema version, strict decode, digest re-computation).
func (s *Service) ReleaseView(ctx context.Context, actor identityDomain.Principal, tenantID, employeeID id.UUID) (api.ReleaseView, error) {
	scope, canManage, err := s.releaseViewScope(ctx, actor, tenantID)
	if err != nil {
		return api.ReleaseView{}, err
	}
	repository := s.releaseRepository()
	if repository == nil {
		return api.ReleaseView{}, domain.ErrNotFound
	}
	if _, err := s.repository.Read(ctx, scope, employeeID); err != nil {
		return api.ReleaseView{}, err
	}
	view := api.ReleaseView{EmployeeID: employeeID, CanManage: canManage, ViewScope: api.ReleaseScopeView}
	if canManage {
		view.ViewScope = api.ReleaseScopeManage
	}
	if draft, err := repository.ReadReleaseDraft(ctx, scope, employeeID); err == nil {
		view.Draft = &draft
	} else if !errors.Is(err, domain.ErrNotFound) {
		return api.ReleaseView{}, err
	}
	if view.Versions, err = repository.ListConfigVersions(ctx, scope, employeeID); err != nil {
		return api.ReleaseView{}, err
	}
	records, err := repository.ListManifests(ctx, scope, employeeID)
	if err != nil {
		return api.ReleaseView{}, err
	}
	view.Manifests = make([]api.ManifestSummary, 0, len(records))
	for _, record := range records {
		view.Manifests = append(view.Manifests, api.ManifestSummary{
			ConfigVersion: record.Manifest.ConfigVersion,
			ManifestID:    record.Manifest.ManifestID,
			Status:        record.Manifest.Status,
			Digest:        record.Manifest.Digest,
			SchemaVersion: record.Manifest.SchemaVersion,
			SealedAt:      record.Manifest.SealedAt,
		})
	}
	if len(records) > 0 {
		manifest := manifestView(records[0])
		view.Latest = &manifest
	}
	return view, nil
}

// manifestView converts one stored record into the honest read view.
func manifestView(record port.ManifestRecord) api.ManifestView {
	read := domain.ReadManifestPayload(record.Payload, record.Manifest.Digest)
	view := api.ManifestView{
		ConfigVersion:    record.Manifest.ConfigVersion,
		ManifestID:       record.Manifest.ManifestID,
		Status:           record.Manifest.Status,
		Digest:           record.Manifest.Digest,
		SchemaVersion:    read.SchemaVersion,
		SealedAt:         record.Manifest.SealedAt,
		ActorID:          record.Manifest.ActorID,
		Quarantine:       read.Status,
		QuarantineReason: read.Reason,
		Raw:              string(read.Raw),
	}
	if read.Status == domain.QuarantineNone {
		view.Content = &read.Content
	}
	return view
}

// ReadReleaseManifest reads one manifest by configuration version with the
// honest quarantine semantics.
func (s *Service) ReadReleaseManifest(ctx context.Context, actor identityDomain.Principal, tenantID, employeeID id.UUID, configVersion int64) (api.ManifestView, error) {
	scope, _, err := s.releaseViewScope(ctx, actor, tenantID)
	if err != nil {
		return api.ManifestView{}, err
	}
	repository := s.releaseRepository()
	if repository == nil {
		return api.ManifestView{}, domain.ErrNotFound
	}
	if _, err := s.repository.Read(ctx, scope, employeeID); err != nil {
		return api.ManifestView{}, err
	}
	records, err := repository.ListManifests(ctx, scope, employeeID)
	if err != nil {
		return api.ManifestView{}, err
	}
	for _, record := range records {
		if record.Manifest.ConfigVersion == configVersion {
			return manifestView(record), nil
		}
	}
	return api.ManifestView{}, domain.ErrNotFound
}

// AssembleDraft creates or refreshes the mutable release draft from the
// current component facts.
func (s *Service) AssembleDraft(ctx context.Context, c api.AssembleCommand) (domain.ReleaseDraft, error) {
	scope, err := s.releaseManageScope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return domain.ReleaseDraft{}, err
	}
	repository := s.releaseRepository()
	if repository == nil {
		return domain.ReleaseDraft{}, domain.ErrNotFound
	}
	draft, behavior, knowledge, voice, err := s.loadEmployeeFacts(ctx, scope, c.EmployeeID)
	if err != nil {
		return domain.ReleaseDraft{}, err
	}
	refs := domain.AssembleConfigRefs(domain.ConfigFacts{
		Brief: draft.Brief, DraftVersion: draft.Version, Pack: draft.Pack,
		Behavior: behavior, Knowledge: knowledge, Voice: voice,
	})
	now := s.clock.Now().UTC()
	eventID, err := s.ids.New(now)
	if err != nil {
		return domain.ReleaseDraft{}, err
	}
	auditID, err := s.ids.New(now)
	if err != nil {
		return domain.ReleaseDraft{}, err
	}
	return repository.SaveReleaseDraft(ctx, port.DraftParams{
		Scope: scope, EmployeeID: c.EmployeeID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID,
		Refs: refs, ExpectedRevision: c.ExpectedRevision, Now: now,
	})
}

// CreateConfigVersion appends one immutable configuration version from the
// locked draft refs.
func (s *Service) CreateConfigVersion(ctx context.Context, c api.VersionCommand) (domain.ConfigVersion, error) {
	scope, err := s.releaseManageScope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	repository := s.releaseRepository()
	if repository == nil {
		return domain.ConfigVersion{}, domain.ErrNotFound
	}
	if err := domain.ValidateReleaseDescription(c.Description); err != nil {
		return domain.ConfigVersion{}, err
	}
	if _, err := repository.ReadReleaseDraft(ctx, scope, c.EmployeeID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ConfigVersion{}, domain.ReleaseRuleError{Rule: "draft", Reason: "Черновик конфигурации ещё не собран: соберите его из текущих фактов, затем создавайте версию."}
		}
		return domain.ConfigVersion{}, err
	}
	now := s.clock.Now().UTC()
	versionID, err := s.ids.New(now)
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	eventID, err := s.ids.New(now)
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	auditID, err := s.ids.New(now)
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	return repository.SaveConfigVersion(ctx, port.VersionParams{
		Scope: scope, EmployeeID: c.EmployeeID, VersionID: versionID, EventID: eventID, AuditID: auditID,
		CorrelationID: c.CorrelationID, Description: c.Description, ExpectedRevision: c.ExpectedRevision, Now: now,
	})
}

// deriveToolSchemasState builds the derived tool_schemas component state
// from the pack pin of the sealed configuration (Story 7-2): the pin's
// pack manifest carries the tool binding, the tool registry carries the
// honest adapter statuses. The append-only registries make a present pin
// unresolvable IMPOSSIBLE (a pack version is never deleted; the loaders
// are fail-closed) — an unreachable state fails loudly instead of sealing
// a lie.
func (s *Service) deriveToolSchemasState(packRef domain.PackRef) domain.RuntimeComponentState {
	if packRef.Status != domain.RefPresent {
		return domain.DeriveToolSchemasState(packRef, nil)
	}
	manifest, ok := packs.Get(packRef.ID, packRef.Version)
	if !ok {
		panic("release tool_schemas: append-only invariant broken — pinned pack " + packRef.ID + "@" + packRef.Version + " отсутствует в реестре")
	}
	bindings := make([]domain.ToolBindingFact, 0, len(manifest.Tools))
	for _, ref := range manifest.Tools {
		contract, ok := tools.Get(ref.ID, ref.Version)
		if !ok {
			panic("release tool_schemas: append-only invariant broken — bound tool " + ref.ID + "@" + ref.Version + " отсутствует в реестре tools")
		}
		bindings = append(bindings, domain.ToolBindingFact{ID: ref.ID, Version: ref.Version, AdapterStatus: contract.AdapterStatus})
	}
	return domain.DeriveToolSchemasState(packRef, bindings)
}

// SealManifest seals the manifest of one configuration version: the content
// is built from the immutable version plus the connection facts at sealing
// time; the digest and the status are derived before persistence and never
// rewritten afterwards.
func (s *Service) SealManifest(ctx context.Context, c api.SealCommand) (domain.ReleaseManifest, error) {
	scope, err := s.releaseManageScope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	repository := s.releaseRepository()
	if repository == nil {
		return domain.ReleaseManifest{}, domain.ErrNotFound
	}
	if c.ConfigVersion < 1 {
		return domain.ReleaseManifest{}, domain.ReleaseRuleError{Rule: "config_version", Reason: "Укажите номер конфигурационной версии для запечатывания."}
	}
	if _, err := repository.ReadReleaseDraft(ctx, scope, c.EmployeeID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ReleaseManifest{}, domain.ReleaseRuleError{Rule: "draft", Reason: "Черновик конфигурации ещё не собран: версия и манифест создаются из черновика."}
		}
		return domain.ReleaseManifest{}, err
	}
	config, err := repository.ReadConfigVersion(ctx, scope, c.EmployeeID, c.ConfigVersion)
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	connections, err := s.connectorFacts(ctx, scope)
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	toolState := s.deriveToolSchemasState(config.Refs.Pack)
	content := domain.BuildManifestContent(config, connections, domain.BuildRuntimeState(toolState))
	status := domain.DeriveManifestStatus(content)
	digest, err := domain.ManifestDigest(content)
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	now := s.clock.Now().UTC()
	manifestID, err := s.ids.New(now)
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	eventID, err := s.ids.New(now)
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	auditID, err := s.ids.New(now)
	if err != nil {
		return domain.ReleaseManifest{}, err
	}
	return repository.SaveManifest(ctx, port.SealParams{
		Scope: scope, EmployeeID: c.EmployeeID, ManifestID: manifestID, EventID: eventID, AuditID: auditID,
		CorrelationID: c.CorrelationID, ConfigVersion: c.ConfigVersion, ExpectedRevision: c.ExpectedRevision,
		Status: status, Digest: digest, Content: content, Now: now,
	})
}

// Restore starts a new draft from the selected configuration version: the
// draft references the version as its source and carries the honest
// compatibility report against the current facts. The past itself never
// changes.
func (s *Service) Restore(ctx context.Context, c api.RestoreCommand) (api.RestoreResult, error) {
	scope, err := s.releaseManageScope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return api.RestoreResult{}, err
	}
	repository := s.releaseRepository()
	if repository == nil {
		return api.RestoreResult{}, domain.ErrNotFound
	}
	version, err := repository.ReadConfigVersion(ctx, scope, c.EmployeeID, c.ConfigVersion)
	if err != nil {
		return api.RestoreResult{}, err
	}
	draft, behavior, knowledge, voice, err := s.loadEmployeeFacts(ctx, scope, c.EmployeeID)
	if err != nil {
		return api.RestoreResult{}, err
	}
	packAvailable := true
	if version.Refs.Pack.Status == domain.RefPresent {
		_, packAvailable = packs.Get(version.Refs.Pack.ID, version.Refs.Pack.Version)
	}
	knowledgeVersions := map[id.UUID]int64{}
	for _, source := range knowledge {
		knowledgeVersions[source.SourceID] = source.CurrentVersion
	}
	connections, err := s.connectorFacts(ctx, scope)
	if err != nil {
		return api.RestoreResult{}, err
	}
	currentConnections := make([]domain.ConnectionFacts, 0, len(connections))
	for _, fact := range connections {
		currentConnections = append(currentConnections, domain.ConnectionFacts{
			ConnectionID: fact.ConnectionID, Status: fact.Status, Capabilities: fact.Capabilities,
		})
	}
	var sealed *domain.ManifestContent
	quarantinedManifest := ""
	if records, err := repository.ListManifests(ctx, scope, c.EmployeeID); err == nil {
		for _, record := range records {
			if record.Manifest.ConfigVersion == c.ConfigVersion {
				read := domain.ReadManifestPayload(record.Payload, record.Manifest.Digest)
				if read.Status == domain.QuarantineNone {
					content := read.Content
					sealed = &content
				} else {
					quarantinedManifest = read.Reason
				}
				break
			}
		}
	}
	latestBehavior, latestVoice := int64(0), int64(0)
	for _, v := range behavior {
		if v.Version > latestBehavior {
			latestBehavior = v.Version
		}
	}
	for _, v := range voice {
		if v.Version > latestVoice {
			latestVoice = v.Version
		}
	}
	issues := domain.RestoreCompatibility(version.Refs, domain.RestoreFacts{
		DraftVersion: draft.Version, PackAvailable: packAvailable,
		BehaviorVersion: latestBehavior, VoiceVersion: latestVoice,
		Knowledge: knowledgeVersions, Connections: currentConnections, Sealed: sealed,
	})
	// A quarantined manifest means the connection part of the report could
	// not be checked: the report says so instead of looking complete.
	if quarantinedManifest != "" {
		issues = append(issues, domain.CompatibilityIssue{Area: "manifest", Subject: "Release Manifest версии",
			Problem: "Запечатанный манифест версии в карантине (" + quarantinedManifest + "): сверка подключений и capabilities по манифесту не выполнена — отчёт неполный, а не пустой."})
	}
	now := s.clock.Now().UTC()
	eventID, err := s.ids.New(now)
	if err != nil {
		return api.RestoreResult{}, err
	}
	auditID, err := s.ids.New(now)
	if err != nil {
		return api.RestoreResult{}, err
	}
	link := domain.ConfigVersionLink{Version: version.Version, VersionID: version.VersionID, CreatedAt: version.CreatedAt}
	restored, err := repository.RestoreReleaseDraft(ctx, port.RestoreParams{
		Scope: scope, EmployeeID: c.EmployeeID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID,
		Refs: version.Refs, RestoredFrom: &link, ExpectedRevision: c.ExpectedRevision, Now: now,
	})
	if err != nil {
		return api.RestoreResult{}, err
	}
	return api.RestoreResult{Draft: restored, Issues: issues}, nil
}
