package app

import (
        "context"
        "errors"
        "strconv"

        "github.com/artemiosu/staffy/internal/employee/api"
        "github.com/artemiosu/staffy/internal/employee/domain"
        "github.com/artemiosu/staffy/internal/employee/packs"
        "github.com/artemiosu/staffy/internal/employee/port"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
)

// profileRepository returns the profile persistence when the repository
// supports it (the postgres store does).
func (s *Service) profileRepository() port.ProfileRepository {
        if repository, ok := s.repository.(port.ProfileRepository); ok {
                return repository
        }
        return nil
}

// profileManageScope authorizes configuration management. A member without
// the scope gets an honest refusal (403); a non-member a plain 404 so tenant
// existence is never disclosed.
func (s *Service) profileManageScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
        if !actor.ID.Valid() || !tenantID.Valid() {
                return tenancy.Context{}, domain.ErrNotFound
        }
        scope, err := tenancy.New(actor.ID, tenantID)
        if err != nil {
                return tenancy.Context{}, err
        }
        if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Configuration); err != nil {
                if errors.Is(err, identityDomain.ErrForbidden) {
                        return tenancy.Context{}, domain.ErrProfileForbidden
                }
                if errors.Is(err, identityDomain.ErrNotFound) {
                        return tenancy.Context{}, domain.ErrNotFound
                }
                // An infrastructure failure is never masked as a 404.
                return tenancy.Context{}, err
        }
        return scope, nil
}

// profileViewScope authorizes viewing: configuration (manage), review or
// audit (view only). Everyone else — member or not — is refused honestly.
func (s *Service) profileViewScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, bool, error) {
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
                // An infrastructure failure is never masked as a 404 or a 403.
                infraFailure = err
        }
        if infraFailure != nil {
                return tenancy.Context{}, false, infraFailure
        }
        if memberWithoutScope {
                return tenancy.Context{}, false, domain.ErrProfileForbidden
        }
        return tenancy.Context{}, false, domain.ErrNotFound
}

// memberFactsSource is the narrow caller-authority fact port to the identity
// module: the profile surface has already verified the actor's scope, so the
// projection does not re-require it (the ConnectionsFacts precedent of
// 1-12). The concrete identity service satisfies it structurally.
type memberFactsSource interface {
        MembersFacts(context.Context, identityDomain.Principal, id.UUID) ([]identityDomain.Membership, error)
}

// memberFacts gathers the caller-authority facts of the tenant members: the
// identifiers of active memberships (invitations carry no principal yet) and
// the roles per member. The read runs through the narrow caller-authority
// fact port: ListMembers itself is owner-gated and cannot serve the
// review/audit members who legitimately view the profile readiness.
func (s *Service) memberFacts(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (map[id.UUID]bool, map[id.UUID][]string, error) {
        source, ok := s.identity.(memberFactsSource)
        if !ok {
                return nil, nil, domain.ProfileRuleError{Section: "identity", Field: "member_facts", Reason: "Порт участников не сконфигурирован: проверка ссылок на участников честно отклоняется, а не пропускается."}
        }
        members, err := source.MembersFacts(ctx, actor, tenantID)
        if err != nil {
                if errors.Is(err, identityDomain.ErrForbidden) {
                        return nil, nil, domain.ErrProfileForbidden
                }
                if errors.Is(err, identityDomain.ErrNotFound) {
                        return nil, nil, domain.ErrNotFound
                }
                return nil, nil, err
        }
        ids := map[id.UUID]bool{}
        roles := map[id.UUID][]string{}
        for _, member := range members {
                if member.PrincipalID == nil || member.Status != "active" {
                        continue
                }
                ids[*member.PrincipalID] = true
                if _, seen := roles[*member.PrincipalID]; !seen {
                        roleNames := make([]string, 0, len(member.Roles))
                        for _, role := range member.Roles {
                                roleNames = append(roleNames, string(role))
                        }
                        roles[*member.PrincipalID] = roleNames
                }
        }
        return ids, roles, nil
}

// manifestRecord reads the exact sealed manifest row (manifest_id +
// config_version) with its stored payload. The link pins the exact version:
// the verification never falls back to "the current state".
func (s *Service) manifestRecord(ctx context.Context, scope tenancy.Context, employeeID id.UUID, link domain.ManifestLink) (port.ManifestRecord, error) {
        release := s.releaseRepository()
        if release == nil {
                return port.ManifestRecord{}, domain.ErrNotFound
        }
        records, err := release.ListManifests(ctx, scope, employeeID)
        if err != nil {
                return port.ManifestRecord{}, err
        }
        for _, record := range records {
                if record.Manifest.ManifestID == link.ManifestID && record.Manifest.ConfigVersion == link.ConfigVersion {
                        return record, nil
                }
        }
        return port.ManifestRecord{}, domain.ErrNotFound
}

// verifyManifestLink checks the exact reference: the manifest must exist as
// the stored row and carry the digest the caller pinned. A mismatched or
// missing manifest is an honest refusal — the draft is not saved.
func (s *Service) verifyManifestLink(ctx context.Context, scope tenancy.Context, employeeID id.UUID, link domain.ManifestLink) (port.ManifestRecord, error) {
        if !link.ManifestID.Valid() || link.ConfigVersion < 1 || len(link.Digest) != 64 {
                return port.ManifestRecord{}, domain.ProfileRuleError{Section: "manifest", Field: "link", Reason: "Ссылка на манифест неполна: укажите manifest_id, config_version и digest запечатанного манифеста."}
        }
        record, err := s.manifestRecord(ctx, scope, employeeID, link)
        if errors.Is(err, domain.ErrNotFound) {
                return port.ManifestRecord{}, domain.ProfileRuleError{Section: "manifest", Field: "link", Reason: "Манифест (manifest_id + config_version) не найден среди запечатанных: черновик профиля не сохраняется — привязка только к точной запечатанной версии."}
        }
        if err != nil {
                return port.ManifestRecord{}, err
        }
        if record.Manifest.Digest != link.Digest {
                return port.ManifestRecord{}, domain.ProfileRuleError{Section: "manifest", Field: "digest", Reason: "Digest ссылки не совпадает с digest запечатанного манифеста: ссылка фиксирует точную версию, «текущее состояние» не подставляется."}
        }
        return record, nil
}

// packJourneysOfManifest derives the journeys of the pack pinned by the
// manifest's configuration snapshot: the cartesian completeness of the
// coverage matrix is judged against this exact list.
func packJourneysOfManifest(record port.ManifestRecord) []string {
        pack := record.Manifest.Content.Config.Refs.Pack
        if pack.Status != domain.RefPresent {
                return nil
        }
        if manifest, ok := packs.Get(pack.ID, pack.Version); ok {
                return manifest.Journeys
        }
        return nil
}

// ProfileView composes the profile projection: the mutable draft with the
// honest readiness list, the approved version with its digest, the derived
// route states, the derived matrix coverage (gaps visible), the derived
// threshold states and the derived applicability. The profile is rules and
// boundaries of work, never a traffic admission.
func (s *Service) ProfileView(ctx context.Context, actor identityDomain.Principal, tenantID, employeeID id.UUID) (api.ProfileView, error) {
        scope, canManage, err := s.profileViewScope(ctx, actor, tenantID)
        if err != nil {
                return api.ProfileView{}, err
        }
        repository := s.profileRepository()
        release := s.releaseRepository()
        if repository == nil || release == nil {
                return api.ProfileView{}, domain.ErrNotFound
        }
        if _, err := s.repository.Read(ctx, scope, employeeID); err != nil {
                return api.ProfileView{}, err
        }
        view := api.ProfileView{EmployeeID: employeeID, CanManage: canManage, ViewScope: api.ProfileScopeView}
        if canManage {
                view.ViewScope = api.ProfileScopeManage
        }
        draft, draftErr := repository.ReadProfileDraft(ctx, scope, employeeID)
        if draftErr != nil && !errors.Is(draftErr, domain.ErrNotFound) {
                return api.ProfileView{}, draftErr
        }
        hasDraft := draftErr == nil
        if hasDraft {
                view.Draft = &draft
        }
        versions, err := repository.ReadApprovedProfiles(ctx, scope, employeeID)
        if err != nil {
                return api.ProfileView{}, err
        }
        if len(versions) > 0 {
                view.Approved = &versions[0]
        }
        thresholds, err := repository.ReadThresholdHistory(ctx, scope, employeeID)
        if err != nil {
                return api.ProfileView{}, err
        }
        manifests, err := release.ListManifests(ctx, scope, employeeID)
        if err != nil {
                return api.ProfileView{}, err
        }
        // The latest manifest is the highest configuration version — the
        // monotonic semantic ordering. The list order (sealed_at desc, id
        // desc) is not deterministic under a fixed clock: equal timestamps
        // fall back to random UUIDv7 bytes, and applicability must never be
        // a coin flip.
        var latest *port.ManifestRecord
        for index := range manifests {
                if latest == nil || manifests[index].Manifest.ConfigVersion > latest.Manifest.ConfigVersion {
                        latest = &manifests[index]
                }
        }
        if latest != nil {
                link := domain.ManifestLink{ManifestID: latest.Manifest.ManifestID, ConfigVersion: latest.Manifest.ConfigVersion, Digest: latest.Manifest.Digest}
                view.LatestManifest = &link
        }

        // The derivations reflect the current work-in-progress (the draft) when
        // one exists; otherwise the approved version; with neither, the states
        // stay empty and honest.
        var content *domain.ProfileContent
        var link domain.ManifestLink
        if hasDraft {
                content = &draft.Content
                link = draft.Manifest
        } else if view.Approved != nil {
                content = &view.Approved.Content
                link = view.Approved.Manifest
        }
        if content != nil {
                record, recordErr := s.manifestRecord(ctx, scope, employeeID, link)
                if recordErr != nil && !errors.Is(recordErr, domain.ErrNotFound) {
                        // An infrastructure failure is never masked as an empty matrix.
                        return api.ProfileView{}, recordErr
                }
                var journeys []string
                packReferenced := false
                if recordErr == nil {
                        journeys = packJourneysOfManifest(record)
                        packReferenced = record.Manifest.Content.Config.Refs.Pack.Status == domain.RefPresent
                }
                view.RouteStates = domain.DeriveRouteStates(*content)
                view.Matrix = domain.DeriveMatrixCoverage(*content, journeys)
                view.Thresholds = domain.DeriveThresholdStates(*content, thresholds)
                // The readiness list validates against the CURRENT facts — members
                // and pack journeys drift honestly even for an approved version.
                memberIDs, _, memberErr := s.memberFacts(ctx, actor, tenantID)
                if memberErr != nil {
                        return api.ProfileView{}, memberErr
                }
                view.Readiness = domain.ValidateProfileContent(*content, domain.ProfileFacts{MemberIDs: memberIDs, PackJourneys: journeys})
                // A manifest without a Pack reference leaves the cartesian
                // completeness vacuous (zero journeys → zero cells → zero
                // gaps): the honest refusal says so instead of silently
                // passing an empty matrix.
                if !packReferenced {
                        view.Readiness = append(view.Readiness, domain.ProfileRuleError{
                                Section: "coverage_matrix", Field: "pack_reference",
                                Reason: "Манифест профиля не содержит Pack-ссылку: полнота матрицы не может быть выведена (декартово произведение пусто) — утверждение честно блокируется до ссылки на манифест с закреплённым Pack.",
                                Kind:    domain.ProfileProblemMissing,
                        })
                }
        } else if view.Approved == nil && len(thresholds) > 0 {
                view.Thresholds = domain.DeriveThresholdStates(domain.ProfileContent{}, thresholds)
        }
        view.Applicability = domain.DeriveProfileApplicability(view.Approved, view.Draft, view.LatestManifest)
        return view, nil
}

// UpdateDraft creates or CAS-updates the mutable profile draft. The exact
// manifest link is re-verified against the stored row on every write; a
// present but invalid value rejects the save (defaults are never
// substituted); missing mandatory pieces save honestly and surface in the
// readiness derivation.
func (s *Service) UpdateDraft(ctx context.Context, c api.ProfileDraftCommand) (domain.ProfileDraft, error) {
        scope, err := s.profileManageScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return domain.ProfileDraft{}, err
        }
        repository := s.profileRepository()
        if repository == nil {
                return domain.ProfileDraft{}, domain.ErrNotFound
        }
        if _, err := s.repository.Read(ctx, scope, c.EmployeeID); err != nil {
                return domain.ProfileDraft{}, err
        }
        record, err := s.verifyManifestLink(ctx, scope, c.EmployeeID, c.Manifest)
        if err != nil {
                return domain.ProfileDraft{}, err
        }
        memberIDs, _, err := s.memberFacts(ctx, c.Actor, c.TenantID)
        if err != nil {
                return domain.ProfileDraft{}, err
        }
        problems := domain.ValidateProfileContent(c.Content, domain.ProfileFacts{MemberIDs: memberIDs, PackJourneys: packJourneysOfManifest(record)})
        if domain.HasInvalidProfileProblems(problems) {
                return domain.ProfileDraft{}, profileRejection(problems)
        }
        now := s.clock.Now().UTC()
        eventID, err := s.ids.New(now)
        if err != nil {
                return domain.ProfileDraft{}, err
        }
        auditID, err := s.ids.New(now)
        if err != nil {
                return domain.ProfileDraft{}, err
        }
        return repository.SaveProfileDraft(ctx, port.ProfileDraftParams{
                Scope: scope, EmployeeID: c.EmployeeID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID,
                Manifest: c.Manifest, Content: c.Content, ExpectedRevision: c.ExpectedRevision, Now: now,
        })
}

// ApproveProfile approves the current draft: step-up is required (the honest
// membership precedent — no provider is wired), all mandatory sections must
// be complete and valid (blocked routes never prevent the approval: they are
// honestly visible and enforced at admission in Story 1-14). The approval is
// atomic and append-only: exactly one version wins a revision race.
func (s *Service) ApproveProfile(ctx context.Context, c api.ProfileApproveCommand) (domain.ProfileVersion, error) {
        if !c.Actor.StepUpVerified {
                return domain.ProfileVersion{}, domain.ErrProfileStepUpRequired
        }
        scope, err := s.profileManageScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        repository := s.profileRepository()
        if repository == nil {
                return domain.ProfileVersion{}, domain.ErrNotFound
        }
        if _, err := s.repository.Read(ctx, scope, c.EmployeeID); err != nil {
                return domain.ProfileVersion{}, err
        }
        if err := domain.ValidateProfileDescription(c.Description); err != nil {
                return domain.ProfileVersion{}, err
        }
        draft, err := repository.ReadProfileDraft(ctx, scope, c.EmployeeID)
        if errors.Is(err, domain.ErrNotFound) {
                return domain.ProfileVersion{}, domain.ProfileRuleError{Section: "draft", Field: "draft", Reason: "Черновик профиля не существует: создайте и заполните его перед утверждением."}
        }
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        record, err := s.verifyManifestLink(ctx, scope, c.EmployeeID, draft.Manifest)
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        memberIDs, roles, err := s.memberFacts(ctx, c.Actor, c.TenantID)
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        problems := domain.ValidateProfileContent(draft.Content, domain.ProfileFacts{MemberIDs: memberIDs, PackJourneys: packJourneysOfManifest(record)})
        if domain.HasBlockingProfileProblems(problems) {
                return domain.ProfileVersion{}, profileRejection(problems)
        }
        if len(c.EvidenceRefs) == 0 {
                return domain.ProfileVersion{}, domain.ProfileRuleError{Section: "approval", Field: "evidence_refs", Reason: "Утверждение требует хотя бы одной evidence-ссылки (digest манифеста, использованные версии порогов) — как у порога, минимальный набор не опускается."}
        }
        for _, reference := range c.EvidenceRefs {
                if !profileRefLength(reference) {
                        return domain.ProfileVersion{}, domain.ProfileRuleError{Section: "approval", Field: "evidence_refs", Reason: "Каждая evidence-ссылка: 1–300 символов (digest манифеста, использованные версии порогов)."}
                }
        }
        digest, err := domain.ProfileDigest(draft.Content)
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        approverRoles := append([]string(nil), roles[c.Actor.ID]...)
        now := s.clock.Now().UTC()
        eventID, err := s.ids.New(now)
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        auditID, err := s.ids.New(now)
        if err != nil {
                return domain.ProfileVersion{}, err
        }
        return repository.ApproveProfile(ctx, port.ProfileApproveParams{
                Scope: scope, EmployeeID: c.EmployeeID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID,
                Manifest: draft.Manifest, Content: draft.Content, Digest: digest,
                Description: c.Description, EvidenceRefs: c.EvidenceRefs, ApproverRoles: approverRoles,
                ExpectedRevision: c.ExpectedRevision, Now: now,
        })
}

// DecideThreshold records one human threshold decision: only the participant
// declared as the evidence owner of the gate in the LAST APPROVED profile
// version may decide — everyone else is honestly refused. The decision is
// append-only with a supersedes link; the system never claims a machine
// verification against a controlled baseline.
func (s *Service) DecideThreshold(ctx context.Context, c api.ThresholdCommand) (domain.GateThresholdVersion, error) {
        scope, err := s.profileManageScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return domain.GateThresholdVersion{}, err
        }
        repository := s.profileRepository()
        if repository == nil {
                return domain.GateThresholdVersion{}, domain.ErrNotFound
        }
        if _, err := s.repository.Read(ctx, scope, c.EmployeeID); err != nil {
                return domain.GateThresholdVersion{}, err
        }
        if err := domain.ValidateThresholdDecision(c.Value, c.Decision, c.EvidenceRefs); err != nil {
                return domain.GateThresholdVersion{}, err
        }
        versions, err := repository.ReadApprovedProfiles(ctx, scope, c.EmployeeID)
        if err != nil {
                return domain.GateThresholdVersion{}, err
        }
        if len(versions) == 0 {
                return domain.GateThresholdVersion{}, domain.ProfileRuleError{Section: "gate_threshold", Field: "gate_id", Reason: "Утверждённой версии профиля ещё нет: evidence owner объявляется в утверждённом профиле, порог до этого не решается."}
        }
        approved := versions[0]
        var declared *domain.GateDeclaration
        for index := range approved.Content.Gates {
                if approved.Content.Gates[index].GateID == c.GateID {
                        declared = &approved.Content.Gates[index]
                        break
                }
        }
        if declared == nil {
                return domain.GateThresholdVersion{}, domain.ProfileRuleError{Section: "gate_threshold", Field: "gate_id", Reason: "Gate «" + c.GateID + "» не объявлен в последней утверждённой версии профиля: порог можно решать только по объявленному gate."}
        }
        if declared.EvidenceOwnerID != c.Actor.ID {
                return domain.GateThresholdVersion{}, domain.ProfileRuleError{Section: "gate_threshold", Field: "evidence_owner", Reason: "Порог решает только участник, объявленный evidence owner'ом gate «" + c.GateID + "» в последней утверждённой версии профиля: решение человеческое, подмена владельца не допускается."}
        }
        history, err := repository.ReadThresholdHistory(ctx, scope, c.EmployeeID)
        if err != nil {
                return domain.GateThresholdVersion{}, err
        }
        nextVersion := int64(1)
        var supersedes *domain.ThresholdLink
        for _, decision := range history {
                if decision.GateID != c.GateID {
                        continue
                }
                if decision.ThresholdVersion >= nextVersion {
                        nextVersion = decision.ThresholdVersion + 1
                }
                if supersedes == nil || decision.ThresholdVersion > supersedes.ThresholdVersion {
                        decidedAt := decision.DecidedAt
                        supersedes = &domain.ThresholdLink{ThresholdVersion: decision.ThresholdVersion, DecidedAt: decidedAt}
                }
        }
        now := s.clock.Now().UTC()
        eventID, err := s.ids.New(now)
        if err != nil {
                return domain.GateThresholdVersion{}, err
        }
        auditID, err := s.ids.New(now)
        if err != nil {
                return domain.GateThresholdVersion{}, err
        }
        return repository.SaveThresholdDecision(ctx, port.ThresholdDecisionParams{
                Scope: scope, EmployeeID: c.EmployeeID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID,
                GateID: c.GateID, ThresholdVersion: nextVersion, Value: c.Value, Decision: c.Decision,
                EvidenceRefs: c.EvidenceRefs, Supersedes: supersedes, Now: now,
        })
}

// ThresholdHistory returns the append-only history of one gate's decisions,
// newest first.
func (s *Service) ThresholdHistory(ctx context.Context, actor identityDomain.Principal, tenantID, employeeID id.UUID, gateID string) (api.ThresholdHistoryView, error) {
        scope, _, err := s.profileViewScope(ctx, actor, tenantID)
        if err != nil {
                return api.ThresholdHistoryView{}, err
        }
        repository := s.profileRepository()
        if repository == nil {
                return api.ThresholdHistoryView{}, domain.ErrNotFound
        }
        if _, err := s.repository.Read(ctx, scope, employeeID); err != nil {
                return api.ThresholdHistoryView{}, err
        }
        if gateID == "" {
                return api.ThresholdHistoryView{}, domain.ProfileRuleError{Section: "gate_threshold", Field: "gate_id", Reason: "Укажите идентификатор gate."}
        }
        history, err := repository.ReadThresholdHistory(ctx, scope, employeeID)
        if err != nil {
                return api.ThresholdHistoryView{}, err
        }
        view := api.ThresholdHistoryView{EmployeeID: employeeID, GateID: gateID, Decisions: []api.ThresholdDecisionView{}}
        for _, decision := range history {
                if decision.GateID != gateID {
                        continue
                }
                view.Decisions = append(view.Decisions, api.ThresholdDecisionView{
                        GateID: decision.GateID, ThresholdVersion: decision.ThresholdVersion, Value: decision.Value,
                        Decision: decision.Decision, ActorID: decision.ActorID, EvidenceRefs: decision.EvidenceRefs,
                        Supersedes: decision.Supersedes, DecidedAt: decision.DecidedAt,
                })
        }
        return view, nil
}

// profileRejection flattens the honest problem list into one rule rejection
// carrying every section.field with its reason.
func profileRejection(problems []domain.ProfileRuleError) domain.ProfileRuleError {
        if len(problems) == 0 {
                return domain.ProfileRuleError{Section: "profile", Field: "content", Reason: "Профиль не готов."}
        }
        first := problems[0]
        summary := first.Section + "." + first.Field + ": " + first.Reason
        if len(problems) > 1 {
                summary += " (и ещё " + strconv.Itoa(len(problems)-1) + ")"
        }
        return domain.ProfileRuleError{Section: first.Section, Field: first.Field, Reason: summary, Problems: problems}
}

// profileRefLength checks one evidence reference length.
func profileRefLength(reference string) bool {
        count := len([]rune(reference))
        return count >= 1 && count <= 300
}
