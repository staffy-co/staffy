package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/artemiosu/staffy/internal/action/api"
	actionCatalog "github.com/artemiosu/staffy/internal/action/catalog"
	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.4: the preparation contour commands (proposal → preview)
// ---------------------------------------------------------------------------
//
// The chain order is FROZEN (the 2.9/3.3 discipline): the rights gate
// (review OR configuration) fires BEFORE every material; the pinned
// definition resolves or the honest 404 answers BEFORE the policy gates
// (the policy needs the resolved definition to run at all); the strict
// validation ladder speaks the exact violation words (one repair attempt,
// the terminal repeat — AD-7); the policy gates read the LIVE sources in
// the normative order (the typed proposal TEXT is never an input — the
// PreparePolicyInput struct of the step-01 domain is the whole input
// space, no field of it carries the submission values); the material
// snapshot is read ONLY after every gate passed (the fake-repo counters
// of the unit matrix pin the order — the reader sees ZERO calls on every
// block path); the build is ONE transaction over the append-only store.
//
// The dry-run evidence NEVER moves the Timeline words (AC-8 — the event
// kind is simply not in DerivePreviewState); the nil adapters are the
// HONEST production boundaries (AG-4): a nil reader keeps the readiness
// honestly blocked, a nil runner refuses the dry-run in words.

// previewScopeText is the honest rights refusal of the preparation
// commands and reads (the review-or-configuration vocabulary of the case
// mutations — the 3.3 precedent).
const previewScopeText = "Подготовка и проверка action preview требуют полномочия review или configuration: участники только с audit не подают proposal, не редактируют версии и не запускают dry-run."

// previewPortsMissingText is the honest refusal when the contour itself
// is not wired (the ConnectionsFacts precedent — an "ok" perimeter is
// never assumed).
const previewPortsMissingText = "Контур подготовки action preview не сконфигурирован: живые источники не подключены, команда честно отклоняется."

// The compile-time proof of the narrow interface registration: the
// transport type-asserts api.PreviewService (the BindCases 3-3
// precedent — the composite Service grows the surface, the test fakes
// stay untouched).
var _ api.PreviewService = (*Service)(nil)

// authorizeProposal is the rights gate of the preparation contour, in the
// honest order: membership FIRST (a non-member gets the 404 — no
// disclosure that the tenant prepares actions at all), then the review OR
// configuration scope words BEFORE any material (the member without
// either reads nothing).
func (s *Service) authorizeProposal(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return tenancy.Context{}, err
	}
	reviewErr := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Review)
	if reviewErr == nil {
		return scope, nil
	}
	if errors.Is(reviewErr, identityDomain.ErrForbidden) {
		// The member exists: configuration is the second honest way in.
		configurationErr := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Configuration)
		if configurationErr == nil {
			return scope, nil
		}
		if errors.Is(configurationErr, identityDomain.ErrForbidden) {
			return tenancy.Context{}, fmt.Errorf("%w: %s", domain.ErrForbidden, previewScopeText)
		}
		if errors.Is(configurationErr, identityDomain.ErrNotFound) {
			// The membership vanished between the two calls: the honest 404.
			return tenancy.Context{}, domain.ErrNotFound
		}
		return tenancy.Context{}, configurationErr
	}
	if errors.Is(reviewErr, identityDomain.ErrNotFound) {
		return tenancy.Context{}, domain.ErrNotFound
	}
	return tenancy.Context{}, reviewErr
}

// requirePreviewPorts refuses honestly when the contour is not wired: the
// repository, the LIVE fact sources and the ID generator are mandatory
// for every command and read of the contour (the optional AG-4 adapters —
// the reader and the dry-runner — are wired separately and may stay nil).
func (s *Service) requirePreviewPorts() error {
	if s.previews == nil || s.caseMaterial == nil || s.members == nil || s.entitlement == nil ||
		s.readiness == nil || s.connectors == nil || s.routeFacts == nil || s.ids == nil {
		return fmt.Errorf("%s", previewPortsMissingText)
	}
	return nil
}

// caseViewMaterial wraps the LIVE case view of one read: the typed
// anchors the preparation contour consumes (the mode word, the live owner
// epoch, the employee identity, the work item anchors, the current
// observations, the conversations, the policy outcome numbers). The view
// is the cases api projection — the action app reads ONLY this surface
// (AD-2: the direct typed reads of the case api; the domain packages are
// never imported for behavior, the range iteration below needs no type
// names).
type caseViewMaterial struct {
	view casesAPI.CaseView
}

// workItemAnchored reports whether the work item belongs to the case (the
// honest 404 of a dead reference BEFORE any material).
func (m caseViewMaterial) workItemAnchored(workItemID id.UUID) bool {
	for _, item := range m.view.WorkItems {
		if item.WorkItemID == workItemID {
			return true
		}
	}
	return false
}

// modeWord is the LIVE conversation mode of the case ("" when no term is
// active — the fail-closed reading of the policy gate).
func (m caseViewMaterial) modeWord() string {
	if m.view.Mode.Active == nil {
		return ""
	}
	return m.view.Mode.Active.Mode
}

// ownerEpoch is the LIVE response-owner epoch anchor of the case (the
// server truth of the 2.6 evaluation form — the policy gate and the
// freshness derivation re-read it at every pass).
func (m caseViewMaterial) ownerEpoch() int64 {
	return m.view.Policy.OwnerEpoch
}

// employeeID is the case's employee identity (the readiness reads scope
// to it).
func (m caseViewMaterial) employeeID() id.UUID {
	return m.view.Case.EmployeeID
}

// policyOutcomeNo is the CURRENT policy outcome number of the work item
// (0 — the honest absence).
func (m caseViewMaterial) policyOutcomeNo(workItemID id.UUID) int64 {
	for _, item := range m.view.Policy.Items {
		if item.WorkItemID == workItemID && item.Current != nil {
			return item.Current.OutcomeNo
		}
	}
	return 0
}

// routeConnectionID is the connection of the case's FIRST conversation
// (the lowest conversation number — the release lineage read anchors to
// it; the 3.3 enqueue precedent of the same scoping).
func (m caseViewMaterial) routeConnectionID() (id.UUID, bool) {
	var best id.UUID
	bestNo := int64(-1)
	for _, conversation := range m.view.Conversations {
		if bestNo < 0 || conversation.ConversationNo < bestNo {
			bestNo = conversation.ConversationNo
			best = conversation.ConnectionID
		}
	}
	return best, bestNo >= 0
}

// factPinsOf derives the version pins of the supporting fact references
// against the LIVE fact versions (the maximal version of each fact — the
// freshness derivation re-checks these pins at every read; a dead
// reference pins version 0 and the derivation names the change honestly).
func factPinsOf(facts casesAPI.FactsView, refs []id.UUID) []domain.VersionPin {
	liveVersions := map[id.UUID]int64{}
	for _, fact := range facts.Facts {
		maxVersion := int64(0)
		for _, version := range fact.Versions {
			if version.VersionNo > maxVersion {
				maxVersion = version.VersionNo
			}
		}
		if maxVersion > 0 {
			liveVersions[fact.FactID] = maxVersion
		}
	}
	pins := make([]domain.VersionPin, 0, len(refs))
	for _, ref := range refs {
		pins = append(pins, domain.VersionPin{Ref: ref, Version: liveVersions[ref]})
	}
	return pins
}

// observationPinsOf derives the version pins of the supporting
// observation references against the CURRENT observations of the case (a
// pinned observation must still BE the current one of its field; a dead
// or replaced reference pins version 0 and the freshness derivation
// names the observation change honestly).
func observationPinsOf(view casesAPI.CaseView, refs []id.UUID) []domain.VersionPin {
	currentNo := map[id.UUID]int64{}
	for _, item := range view.Observations.Items {
		for _, card := range item.Fields {
			if card.Current != nil {
				currentNo[card.Current.ObservationID] = card.Current.ObservationNo
			}
		}
	}
	pins := make([]domain.VersionPin, 0, len(refs))
	for _, ref := range refs {
		pins = append(pins, domain.VersionPin{Ref: ref, Version: currentNo[ref]})
	}
	return pins
}

// principalRoleWord derives the role word the policy gate reads: the
// actor's active roles, and among them the FIRST word of the definition's
// AllowedRoles (the any-role-passes semantics — the actor may hold
// several; the domain guard stays the single source of the decision, the
// app only selects which of the actor's OWN role words to present). When
// none matches, the first active role word is presented — the guard then
// answers with the honest role_not_allowed block.
func principalRoleWord(memberships []identityDomain.Membership, actorID id.UUID, allowed []string) string {
	roles := []string{}
	for _, membership := range memberships {
		if membership.PrincipalID != nil && *membership.PrincipalID == actorID && membership.Status == "active" {
			for _, role := range membership.Roles {
				roles = append(roles, string(role))
			}
		}
	}
	if len(roles) == 0 {
		return ""
	}
	// The dictionary order of the role words makes the pick deterministic.
	sort.Strings(roles)
	allowedSet := map[string]bool{}
	for _, word := range allowed {
		allowedSet[word] = true
	}
	for _, role := range roles {
		if allowedSet[role] {
			return role
		}
	}
	return roles[0]
}

// previewLiveReadings is the LIVE gather of one gate pass: the policy
// input (the step-01 struct — the typed text is structurally absent),
// the control generation (the freshness pin), and the capability material
// (the state word plus the declaration version of the connection). The
// release lineage profile version reads separately (the strict build
// and the tolerant freshness disagree on a vanished source).
type previewLiveReadings struct {
	policy         domain.PreparePolicyInput
	control        port.ControlEffective
	capState       string
	declVersion    string
	profileVersion int64
}

// gatherPreviewLive reads the LIVE sources of one gate pass: the actor's
// role word (identity member facts), the case mode and the live owner
// epoch (the case view anchors), the scoped control of the preparation
// route (the 3.2 port — the nil port or a foreign/unresolvable reading
// keeps the honest fail-closed control_mode block, the 3.3 gasp
// discipline), the entitlement admission word, the employee readiness
// stage with the live gate instances (a gate with no live instance stays
// absent — the honest documented boundary of the step-01 domain, never a
// silent pass invented here), and the connector capability state with the
// declaration version of the serving connection.
func (s *Service) gatherPreviewLive(ctx context.Context, actor identityDomain.Principal, scope tenancy.Context,
	def domain.ActionDefinition, material caseViewMaterial) (previewLiveReadings, error) {
	live := previewLiveReadings{}
	memberships, err := s.members.MembersFacts(ctx, actor, scope.TenantID())
	if err != nil {
		return live, err
	}
	live.policy.PrincipalRole = principalRoleWord(memberships, actor.ID, def.Conditions.AllowedRoles)
	live.policy.CaseMode = material.modeWord()
	live.policy.LiveOwnerEpoch = material.ownerEpoch()
	if s.previewControl != nil {
		effective, err := s.previewControl.EffectiveFor(ctx, scope.TenantID(), port.ControlRoute{
			ConnectorID:   def.RequiredCapability.ConnectorID,
			AutonomyClass: def.Conditions.AutonomyClass,
		})
		if err != nil {
			return live, err
		}
		live.control = effective
		live.policy.ControlMode = effective.Mode
	}
	admission, err := s.entitlement.AdmissionView(ctx, actor, scope.TenantID())
	if err != nil {
		return live, err
	}
	live.policy.AdmissionState = admission.Status
	// The CURRENT published stage word (nothing published = the synthetic
	// world — the 1-14 dictionary).
	history, err := s.readiness.AdmissionHistoryView(ctx, actor, scope.TenantID(), material.employeeID())
	if err != nil {
		return live, err
	}
	live.policy.Stage = "synthetic"
	if history.Active != nil {
		live.policy.Stage = history.Active.Stage
	}
	// The live gate instances of the required gates: the NEWEST instance
	// of each gate definition decides (the supersedes chain — the latest
	// recorded state IS the current state; expired/fail answer false).
	if len(def.Conditions.RequiredGates) > 0 {
		instances, err := s.readiness.GateInstancesView(ctx, actor, scope.TenantID(), material.employeeID())
		if err != nil {
			return live, err
		}
		newest := map[string]struct {
			state      string
			recordedAt time.Time
		}{}
		for _, instance := range instances.Instances {
			current, ok := newest[instance.DefinitionID]
			if !ok || instance.RecordedAt.After(current.recordedAt) {
				newest[instance.DefinitionID] = struct {
					state      string
					recordedAt time.Time
				}{state: instance.State, recordedAt: instance.RecordedAt}
			}
		}
		live.policy.GatePasses = map[string]bool{}
		for _, gate := range def.Conditions.RequiredGates {
			if instance, ok := newest[gate]; ok {
				live.policy.GatePasses[gate] = instance.state == "pass"
			}
		}
	}
	// The connector capability state: the tenant connections matching the
	// definition's connector pin carry the derived reports; the exact
	// capability must be declared on the connection, the state word must
	// be verified for the gate, and the connection contract version is
	// the declaration pin of the freshness derivation. An absent
	// connection or capability keeps the words empty — the fail-closed
	// capability_not_verified block of the domain guard.
	connections, err := s.connectors.ConnectionsFacts(ctx, scope)
	if err != nil {
		return live, err
	}
	for _, connection := range connections {
		if connection.ConnectorID != def.RequiredCapability.ConnectorID {
			continue
		}
		for _, capability := range connection.Capabilities {
			if capability.ID != def.RequiredCapability.CapabilityID {
				continue
			}
			live.capState = capability.State
			live.declVersion = connection.ContractVersion
			break
		}
		if live.capState != "" {
			break
		}
	}
	live.policy.CapState = live.capState
	return live, nil
}

// routeProfileVersion reads the release lineage profile version of the
// case's connection (the RouteFacts facade — the 3-3 lineageWord
// precedent). A case without a conversation or a route without
// candidates answers domain.ErrNotFound: the strict build refuses (the
// lineage is a required pin — the store CHECK refuses a negative
// number), the tolerant freshness derivation of the read treats the
// vanished source as the change it is.
func (s *Service) routeProfileVersion(ctx context.Context, scope tenancy.Context, material caseViewMaterial) (int64, error) {
	connectionID, ok := material.routeConnectionID()
	if !ok {
		return 0, domain.ErrNotFound
	}
	routes, err := s.routeFacts.RouteFacts(ctx, scope, connectionID)
	if err != nil {
		return 0, err
	}
	if len(routes.Candidates) == 0 {
		return 0, domain.ErrNotFound
	}
	return routes.Candidates[0].ProfileVersion, nil
}

// previewFreshnessLive reads the LIVE pins the freshness derivation
// re-checks at every read: the live fact versions, the live observation
// versions (the CURRENT observation of each pinned reference), the live
// owner epoch, the control generation of the definition's route, the
// release lineage profile version of the case's connection, the current
// policy outcome number of the work item, and the capability evidence
// words. A DISAPPEARED source reads as the mismatch (fail-closed — the
// derivation treats it as the change it is; the profile version reads -1
// when the route facts carry no candidate).
func (s *Service) previewFreshnessLive(ctx context.Context, actor identityDomain.Principal, scope tenancy.Context,
	preview domain.ActionPreview, material caseViewMaterial, live previewLiveReadings,
	facts casesAPI.FactsView) (domain.PreviewFreshnessLive, error) {
	fresh := domain.PreviewFreshnessLive{
		OwnerEpoch:            material.ownerEpoch(),
		ControlGeneration:     live.control.Generation,
		CapState:              live.capState,
		CapabilityDeclVersion: live.declVersion,
		PolicyOutcomeNo:       material.policyOutcomeNo(preview.WorkItemID),
		FactVersions:          map[id.UUID]int64{},
		ObservationVersions:   map[id.UUID]int64{},
	}
	for _, fact := range facts.Facts {
		maxVersion := int64(0)
		for _, version := range fact.Versions {
			if version.VersionNo > maxVersion {
				maxVersion = version.VersionNo
			}
		}
		if maxVersion > 0 {
			fresh.FactVersions[fact.FactID] = maxVersion
		}
	}
	for _, item := range material.view.Observations.Items {
		for _, card := range item.Fields {
			if card.Current != nil {
				fresh.ObservationVersions[card.Current.ObservationID] = card.Current.ObservationNo
			}
		}
	}
	fresh.ProfileVersion = int64(-1)
	if profileVersion, err := s.routeProfileVersion(ctx, scope, material); err == nil {
		fresh.ProfileVersion = profileVersion
	} else if !errors.Is(err, domain.ErrNotFound) {
		return fresh, err
	}
	return fresh, nil
}

// readPreviewMaterial reads the fresh material rows of the definition's
// required facts through the consumer-owned reader (AFTER every gate
// passed — the fake-repo counters pin the order). A NIL reader answers
// NO rows: the readiness derivation honestly blocks with the
// blocked_material word (the AG-4 production boundary — the readiness
// never pretends). Every row status is re-mapped through MapSnapshotStatus
// against the definition's DECLARED tolerance — a lying adapter can
// never smuggle the allowed_uncertain word in (fail-closed to unknown).
func (s *Service) readPreviewMaterial(ctx context.Context, scope tenancy.Context, def domain.ActionDefinition, preview domain.ActionPreview) []domain.MaterialSnapshotRow {
	if s.previewReader == nil {
		return nil
	}
	required := map[string]string{}
	for _, req := range def.Conditions.RequiredFacts {
		required[req.MaterialField] = req.Freshness
	}
	fields := make([]string, 0, len(required))
	for field := range required {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	rows, err := s.previewReader.FreshReads(ctx, port.FreshReadRequest{
		Scope:             scope,
		OwningSystem:      def.OwningSystem,
		CaseID:            preview.CaseID,
		WorkItemID:        preview.WorkItemID,
		DefinitionID:      def.ID,
		DefinitionVersion: def.Version,
		MaterialFields:    fields,
	})
	if err != nil {
		// The reader failure is the honest blocked_material material: NO
		// rows survive, the preview exists with the honest неготовность
		// (the pin_blocked degradation precedent 3-3 — the build never
		// pretends and never fails on a provider that is not wired to
		// exist).
		return nil
	}
	mapped := make([]domain.MaterialSnapshotRow, 0, len(rows))
	for _, row := range rows {
		row.Status = domain.MapSnapshotStatus(row.Status, required[row.MaterialField])
		mapped = append(mapped, row)
	}
	return mapped
}

// buildPreviewMaterial runs the post-policy chain of one version build
// (the shared material of the submit and the edit): the projected typed
// input (the STRUCTURAL projection of AC-2 — unknown fields and secret
// values cannot survive), the finite effects derived ONLY from the
// definition, the execution partition with the honest human-only markers
// (the unsafe class and the reconciliation resolution against the LIVE
// provider declaration — never the self-declaration), the fresh material
// snapshot, the supporting version pins, the lineage pins, and the
// keyed digest over the canonical bytes. The caller has ALREADY
// validated the submission and passed the policy gates.
func (s *Service) buildPreviewMaterial(ctx context.Context, actor identityDomain.Principal, scope tenancy.Context,
	def domain.ActionDefinition, proposal domain.Proposal, material caseViewMaterial, live previewLiveReadings,
	humanOnlyMark bool, facts casesAPI.FactsView, previewID id.UUID, versionNo int64, now time.Time) (domain.ActionPreview, error) {
	preview := domain.ActionPreview{
		TenantID:             scope.TenantID(),
		PreviewID:            previewID,
		VersionNo:            versionNo,
		DefinitionID:         def.ID,
		DefinitionVersion:    def.Version,
		CaseID:               proposal.CaseID,
		WorkItemID:           proposal.WorkItemID,
		Target:               proposal.Target,
		TypedInput:           domain.ProjectTypedInput(def, proposal.TypedInput),
		Effects:              domain.DerivePreviewEffects(def),
		HumanOnlyControlMark: humanOnlyMark,
		OwnerEpoch:           proposal.OwnerEpoch,
		ControlGeneration:    live.control.Generation,
		PolicyOutcomeNo:      material.policyOutcomeNo(proposal.WorkItemID),
		AuthorID:             proposal.AuthorID,
		CreatedAt:            now.UTC().Truncate(time.Microsecond),
	}
	// The reconciliation resolution against the LIVE provider declaration
	// (the 3-1 resolver — the capability facts of the pinned triple): a
	// declared reconciliation semantics is the proven path; anything else
	// keeps the action honestly human-only.
	reconciliationResolves := false
	if capabilityFacts, resolves := s.caps.Capability(def.RequiredCapability.ConnectorID, def.RequiredCapability.ConnectorVersion, def.RequiredCapability.CapabilityID); resolves {
		reconciliationResolves = capabilityFacts.Reconciliation != ""
	}
	preview.Partition = domain.DeriveExecutionPartition(def, reconciliationResolves)
	// The release lineage pin (the strict read — the 3-3 lineageWord
	// precedent: a build without a live lineage refuses honestly, the
	// store CHECK demands a non-negative number).
	profileVersion, err := s.routeProfileVersion(ctx, scope, material)
	if err != nil {
		return domain.ActionPreview{}, err
	}
	preview.ProfileVersion = profileVersion
	// The capability declaration version pin: the connection contract
	// version of the LIVE reading. The policy gate has ALREADY refused a
	// non-verified capability, so this pin only travels on passing
	// builds; an unreadable word keeps the honest "-" marker (the
	// freshness derivation re-checks it as the change it is).
	preview.CapabilityDeclVersion = live.declVersion
	if preview.CapabilityDeclVersion == "" {
		preview.CapabilityDeclVersion = "-"
	}
	// The fresh material snapshot (AFTER every gate — the reader sees
	// only the passing paths).
	preview.Snapshot = s.readPreviewMaterial(ctx, scope, def, preview)
	// The supporting version pins against the live fact and observation
	// versions (the freshness re-checks them at every read).
	preview.SupportingFacts = factPinsOf(facts, proposal.SupportingFacts)
	preview.SupportingObservations = observationPinsOf(material.view, proposal.SupportingObservations)
	// The digest: the keyed identity of the WHOLE shown row over the
	// canonical bytes (the store re-computes it under the locks — the
	// 3-3 token discipline).
	key, err := s.previews.ReadPreviewKey(ctx, scope, now)
	if err != nil {
		return domain.ActionPreview{}, err
	}
	preview.Digest = domain.PreviewDigest(key, previewID, versionNo, domain.PreviewCanonicalBytes(preview))
	return preview, nil
}

// SubmitProposal runs the preparation chain of one attempt (the frozen
// order): the rights gate → the definition resolution (the honest 404) →
// the live case material and the work item anchor (the honest 404 of a
// dead reference BEFORE any material) → the attempt ladder of the derived
// proposal key (the third attempt does not exist) → the strict validation
// with the AD-7 repair semantics → the LIVE policy gates in the normative
// order (the block path records the attempt and its event with the exact
// reason word, NO version is built, the material reader sees ZERO
// calls) → the material snapshot → the digest → the ONE build
// transaction.
func (s *Service) SubmitProposal(ctx context.Context, command api.SubmitProposalCommand) (api.ProposalOutcome, error) {
	if err := s.requirePreviewPorts(); err != nil {
		return api.ProposalOutcome{}, err
	}
	scope, err := s.authorizeProposal(ctx, command.Actor, command.TenantID)
	if err != nil {
		return api.ProposalOutcome{}, err
	}
	// The pinned definition resolves BEFORE the policy (the honest 404 —
	// the policy needs the resolved definition to run at all).
	def, ok := actionCatalog.GetFrom(s.defs, command.DefinitionID, command.DefinitionVersion)
	if !ok {
		return api.ProposalOutcome{}, domain.ErrNotFound
	}
	// The live case material: the case must exist and the work item must
	// anchor it (the AD-2 discipline — the case and the work item are
	// plain uuid pins in the store, so the LIVE read validates them
	// BEFORE the insert transaction; a dead reference is the honest 404).
	caseView, err := s.caseMaterial.CaseView(ctx, command.Actor, command.TenantID, command.CaseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return api.ProposalOutcome{}, domain.ErrNotFound
		}
		return api.ProposalOutcome{}, err
	}
	material := caseViewMaterial{view: caseView}
	if !material.workItemAnchored(command.WorkItemID) {
		return api.ProposalOutcome{}, domain.ErrNotFound
	}
	// The attempt ladder of the derived key (the key carries NO typed
	// input — a repair is the same key at attempt 2, AD-7).
	proposal := domain.Proposal{
		DefinitionID:           command.DefinitionID,
		DefinitionVersion:      command.DefinitionVersion,
		CaseID:                 command.CaseID,
		WorkItemID:             command.WorkItemID,
		Target:                 command.Target,
		TypedInput:             command.TypedInput,
		SupportingFacts:        command.SupportingFacts,
		SupportingObservations: command.SupportingObservations,
		OwnerEpoch:             command.OwnerEpoch,
		AttemptNo:              1,
		AuthorID:               command.Actor.ID,
	}
	proposalKey := domain.DeriveProposalKey(command.TenantID, proposal)
	attempts, err := s.previews.ReadProposalAttempts(ctx, scope, proposalKey)
	if err != nil {
		return api.ProposalOutcome{}, err
	}
	maxAttempt := 0
	previewID := id.UUID("")
	for _, stored := range attempts {
		if stored.AttemptNo > maxAttempt {
			maxAttempt = stored.AttemptNo
		}
		if stored.PreviewID.Valid() {
			previewID = stored.PreviewID
		}
	}
	if maxAttempt >= domain.MaxAttemptsProposal {
		// The terminal refusal: both attempts exist, the third does not
		// exist (the structural bound — never a discipline).
		return api.ProposalOutcome{}, fmt.Errorf("%w: обе попытки этого предложения исчерпаны — третьей не существует; отредактируйте построенный preview либо подайте предложение с другим пином", port.ErrAttemptExhausted)
	}
	attemptNo := maxAttempt + 1
	proposal.AttemptNo = attemptNo
	if !previewID.Valid() {
		if previewID, err = s.ids.New(command.Now); err != nil {
			return api.ProposalOutcome{}, err
		}
	}
	// The strict validation against the pinned definition's schema (the
	// unknown fields NEVER enter the preview and are never relayed).
	violations := domain.GuardValidateProposal(def, proposal)
	if len(violations) > 0 {
		verdict := domain.VerdictForInvalid(attemptNo)
		stageKind := domain.EventProposed
		if attemptNo > 1 {
			stageKind = domain.EventRepaired
		}
		stageEventID, err := s.ids.New(command.Now)
		if err != nil {
			return api.ProposalOutcome{}, err
		}
		if err := s.previews.RecordProposalAttempt(ctx, port.RecordAttemptParams{
			Scope: scope,
			Attempt: port.ProposalAttemptRow{
				ProposalKey:       proposalKey,
				AttemptNo:         attemptNo,
				PreviewID:         previewID,
				DefinitionID:      def.ID,
				DefinitionVersion: def.Version,
				CaseID:            command.CaseID,
				WorkItemID:        command.WorkItemID,
				TargetKind:        command.Target.Kind,
				TargetRef:         command.Target.Ref,
				Verdict:           verdict,
				InvalidReport:     domain.InvalidReportText(violations),
				ActorID:           command.Actor.ID,
				CreatedAt:         command.Now.UTC().Truncate(time.Microsecond),
			},
			StageEventKind: stageKind,
			StageEventID:   stageEventID,
			Now:            command.Now,
		}); err != nil {
			return api.ProposalOutcome{}, err
		}
		return api.ProposalOutcome{
			Command:    "submit_proposal",
			Verdict:    verdict,
			Violations: violations,
			AttemptNo:  attemptNo,
			PreviewID:  previewID,
		}, nil
	}
	// The LIVE policy gates in the normative order (the typed text is
	// never an input — the struct of the step-01 domain is the whole
	// input space; the unit matrix pins the probe).
	live, err := s.gatherPreviewLive(ctx, command.Actor, scope, def, material)
	if err != nil {
		return api.ProposalOutcome{}, err
	}
	policy := domain.GuardPreparePolicy(def, proposal.OwnerEpoch, live.policy)
	if policy.Outcome == domain.PolicyBlock {
		// The honest block BEFORE the snapshot: the attempt row and its
		// stage event carry the exact reason word, NO version is built,
		// the material reader sees ZERO calls (the unit counters pin it).
		stageKind := domain.EventProposed
		if attemptNo > 1 {
			stageKind = domain.EventRepaired
		}
		stageEventID, err := s.ids.New(command.Now)
		if err != nil {
			return api.ProposalOutcome{}, err
		}
		if err := s.previews.RecordProposalAttempt(ctx, port.RecordAttemptParams{
			Scope: scope,
			Attempt: port.ProposalAttemptRow{
				ProposalKey:       proposalKey,
				AttemptNo:         attemptNo,
				PreviewID:         previewID,
				DefinitionID:      def.ID,
				DefinitionVersion: def.Version,
				CaseID:            command.CaseID,
				WorkItemID:        command.WorkItemID,
				TargetKind:        command.Target.Kind,
				TargetRef:         command.Target.Ref,
				Verdict:           domain.VerdictValid,
				BlockedReason:     policy.Reason,
				ActorID:           command.Actor.ID,
				CreatedAt:         command.Now.UTC().Truncate(time.Microsecond),
			},
			StageEventKind:   stageKind,
			StageEventReason: policy.Reason,
			StageEventID:     stageEventID,
			Now:              command.Now,
		}); err != nil {
			return api.ProposalOutcome{}, err
		}
		return api.ProposalOutcome{
			Command:       "submit_proposal",
			Verdict:       domain.VerdictValid,
			AttemptNo:     attemptNo,
			PreviewID:     previewID,
			BlockedReason: policy.Reason,
		}, nil
	}
	// The live fact versions of the case (the supporting pins of the
	// build) — read once, BEFORE the material reader (the counters pin
	// the reader LAST).
	facts, err := s.caseMaterial.FactsView(ctx, command.Actor, command.TenantID, command.CaseID)
	if err != nil {
		return api.ProposalOutcome{}, err
	}
	// The build: the material chain and the ONE transaction (the store
	// re-derives the ladders and re-computes the digest under the
	// locks).
	preview, err := s.buildPreviewMaterial(ctx, command.Actor, scope, def, proposal, material, live, policy.HumanOnlyMark, facts, previewID, 1, command.Now)
	if err != nil {
		return api.ProposalOutcome{}, err
	}
	stageEventID, err := s.ids.New(command.Now)
	if err != nil {
		return api.ProposalOutcome{}, err
	}
	previewedEventID, err := s.ids.New(command.Now)
	if err != nil {
		return api.ProposalOutcome{}, err
	}
	attemptRow := port.ProposalAttemptRow{
		ProposalKey:       proposalKey,
		AttemptNo:         attemptNo,
		PreviewID:         previewID,
		DefinitionID:      def.ID,
		DefinitionVersion: def.Version,
		CaseID:            command.CaseID,
		WorkItemID:        command.WorkItemID,
		TargetKind:        command.Target.Kind,
		TargetRef:         command.Target.Ref,
		Verdict:           domain.VerdictValid,
		ActorID:           command.Actor.ID,
		CreatedAt:         command.Now.UTC().Truncate(time.Microsecond),
	}
	if err := s.previews.BuildPreviewVersion(ctx, port.BuildPreviewVersionParams{
		Scope:               scope,
		Preview:             preview,
		Attempt:             &attemptRow,
		AttemptStageEventID: stageEventID,
		PreviewedEventID:    previewedEventID,
		Now:                 command.Now,
	}); err != nil {
		return api.ProposalOutcome{}, err
	}
	return api.ProposalOutcome{
		Command:       "submit_proposal",
		Verdict:       domain.VerdictValid,
		AttemptNo:     attemptNo,
		PreviewID:     previewID,
		VersionNo:     preview.VersionNo,
		PreviewDigest: preview.Digest,
	}, nil
}

// ReadPreviewView renders the Preview Review surface of one case: every
// version card with the LIVE read-time derivations (the readiness, the
// freshness with the exact reason word, the Timeline word over the
// append-only events of the exact version), the honest attempt ladders,
// the dry-run evidence and the material diff of the last edit. The old
// versions stay readable for the audit forever; the authorization
// suitability dies through the freshness derivation, never through a
// deletion; the stored digest re-verifies at every read (the integrity
// alarm is spoken, never masked).
func (s *Service) ReadPreviewView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID) (api.PreviewView, error) {
	if err := s.requirePreviewPorts(); err != nil {
		return api.PreviewView{}, err
	}
	scope, err := s.authorizeProposal(ctx, actor, tenantID)
	if err != nil {
		return api.PreviewView{}, err
	}
	versions, err := s.previews.ReadPreviewVersions(ctx, scope, caseID)
	if err != nil {
		return api.PreviewView{}, err
	}
	events, err := s.previews.ReadPreviewEvents(ctx, scope, caseID)
	if err != nil {
		return api.PreviewView{}, err
	}
	attempts, err := s.previews.ReadCaseAttempts(ctx, scope, caseID)
	if err != nil {
		return api.PreviewView{}, err
	}
	// The LIVE material of the case: the freshness re-checks read the
	// live sources at EVERY read (the derived word is never a column); a
	// case that vanished mid-read answers the honest 404. The read runs
	// BEFORE the empty-state answer so the proposal form always carries
	// the anchored Work Items (an empty preview world is still a form
	// surface).
	caseView, err := s.caseMaterial.CaseView(ctx, actor, tenantID, caseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return api.PreviewView{}, domain.ErrNotFound
		}
		return api.PreviewView{}, err
	}
	material := caseViewMaterial{view: caseView}
	view := api.PreviewView{
		TenantID:  tenantID,
		CaseID:    caseID,
		PanelText: domain.PreviewPanelText,
		Threads:   []api.PreviewThreadView{},
		WorkItemIDs: func() []id.UUID {
			ids := make([]id.UUID, 0, len(caseView.WorkItems))
			for _, item := range caseView.WorkItems {
				ids = append(ids, item.WorkItemID)
			}
			return ids
		}(),
	}
	if len(versions) == 0 && len(attempts) == 0 {
		view.EmptyText = domain.PreviewEmptyText
		return view, nil
	}
	facts, err := s.caseMaterial.FactsView(ctx, actor, tenantID, caseID)
	if err != nil {
		return api.PreviewView{}, err
	}
	// The per-tenant key of the digest re-verification (the ensure
	// pattern is idempotent — the row exists, the read path inserts
	// nothing).
	key, err := s.previews.ReadPreviewKey(ctx, scope, time.Now().UTC())
	if err != nil {
		return api.PreviewView{}, err
	}
	// The per-thread grouping (the stable preview identity) and the
	// per-definition LIVE cache (one gate pass per distinct definition
	// pin of the case).
	threads := map[id.UUID]*api.PreviewThreadView{}
	maxVersion := map[id.UUID]int64{}
	for _, version := range versions {
		if version.VersionNo > maxVersion[version.PreviewID] {
			maxVersion[version.PreviewID] = version.VersionNo
		}
	}
	liveCache := map[string]previewLiveReadings{}
	for _, version := range versions {
		thread, ok := threads[version.PreviewID]
		if !ok {
			thread = &api.PreviewThreadView{PreviewID: version.PreviewID, Cards: []api.PreviewCardView{}, Attempts: []api.PreviewAttemptView{}, DryRunEvidence: []api.PreviewDryRunEvidenceView{}}
			threads[version.PreviewID] = thread
		}
		def, defOK := actionCatalog.GetFrom(s.defs, version.DefinitionID, version.DefinitionVersion)
		var fresh domain.PreviewFreshnessLive
		if defOK {
			cacheKey := version.DefinitionID + "@" + version.DefinitionVersion
			cached, seen := liveCache[cacheKey]
			if !seen {
				cached, err = s.gatherPreviewLive(ctx, actor, scope, def, material)
				if err != nil {
					return api.PreviewView{}, err
				}
				liveCache[cacheKey] = cached
			}
			fresh, err = s.previewFreshnessLive(ctx, actor, scope, version, material, cached, facts)
			if err != nil {
				return api.PreviewView{}, err
			}
		}
		card := previewCardOf(version, events, def, defOK, fresh, maxVersion[version.PreviewID] == version.VersionNo, key)
		thread.Cards = append(thread.Cards, card)
	}
	for _, row := range attempts {
		thread, ok := threads[row.PreviewID]
		if !ok {
			thread = &api.PreviewThreadView{PreviewID: row.PreviewID, Cards: []api.PreviewCardView{}, Attempts: []api.PreviewAttemptView{}, DryRunEvidence: []api.PreviewDryRunEvidenceView{}}
			threads[row.PreviewID] = thread
		}
		thread.Attempts = append(thread.Attempts, api.PreviewAttemptView{
			PreviewID:     row.PreviewID,
			AttemptNo:     row.AttemptNo,
			Verdict:       row.Verdict,
			BlockedReason: row.BlockedReason,
			InvalidReport: row.InvalidReport,
			CreatedAt:     row.CreatedAt,
		})
	}
	for _, event := range events {
		if event.Kind != domain.EventDryRunRecorded {
			continue
		}
		thread, ok := threads[event.PreviewID]
		if !ok {
			continue
		}
		thread.DryRunEvidence = append(thread.DryRunEvidence, api.PreviewDryRunEvidenceView{
			VersionNo: event.VersionNo,
			Result:    event.Reason,
			CreatedAt: event.CreatedAt,
		})
	}
	// The deterministic order of the threads: the creation time of the
	// first version, then the identity; the cards stay in the version
	// order; the diff of every edit lives on the card it produced (the
	// read-time derivation over the pair — NEVER stored).
	ordered := make([]api.PreviewThreadView, 0, len(threads))
	for _, thread := range threads {
		ordered = append(ordered, *thread)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if len(ordered[i].Cards) == 0 || len(ordered[j].Cards) == 0 {
			return ordered[i].PreviewID < ordered[j].PreviewID
		}
		if ordered[i].Cards[0].CreatedAt.Equal(ordered[j].Cards[0].CreatedAt) {
			return ordered[i].PreviewID < ordered[j].PreviewID
		}
		return ordered[i].Cards[0].CreatedAt.Before(ordered[j].Cards[0].CreatedAt)
	})
	for i := range ordered {
		cards := ordered[i].Cards
		for j := 1; j < len(cards); j++ {
			cards[j].DiffLines = derivePreviewDiff(cards[j-1], cards[j])
		}
	}
	view.Threads = ordered
	return view, nil
}

// previewCardOf maps one stored version row onto the wire card with the
// LIVE read-time derivations: the readiness over the snapshot rows
// against the definition's required facts (an unresolvable definition pin
// keeps the honest blocked word — the preview stays readable for the
// audit whatever happened to the dictionary), the freshness over the
// live pins (the exact reason word of the seven-word dictionary), the
// Timeline word over the append-only events of the EXACT version (the
// superseded priority; the dry-run and fresh-read kinds never move it),
// the digest integrity re-check (the alarm is spoken, never masked), and
// the fixed domain texts (one source — the app and the fixture share the
// words; the authorization-absence text of the primary action speaks the
// missing 3-5 contour honestly).
func previewCardOf(version domain.ActionPreview, events []port.PreviewEventRow, def domain.ActionDefinition, defResolved bool,
	fresh domain.PreviewFreshnessLive, current bool, key []byte) api.PreviewCardView {
	card := api.PreviewCardView{
		PreviewID:                version.PreviewID,
		VersionNo:                version.VersionNo,
		DefinitionID:             version.DefinitionID,
		DefinitionVersion:        version.DefinitionVersion,
		CaseID:                   version.CaseID,
		WorkItemID:               version.WorkItemID,
		TargetKind:               version.Target.Kind,
		TargetRef:                version.Target.Ref,
		TypedInput:               version.TypedInput,
		SupportingFacts:          version.SupportingFacts,
		SupportingObservations:   version.SupportingObservations,
		OwnerEpoch:               version.OwnerEpoch,
		ControlGeneration:        version.ControlGeneration,
		ProfileVersion:           version.ProfileVersion,
		PolicyOutcomeNo:          version.PolicyOutcomeNo,
		CapabilityDeclVersion:    version.CapabilityDeclVersion,
		Partition:                version.Partition.Partition,
		PartitionText:            domain.ExecutionPartitionText(version.Partition.Partition),
		RecoveryInstruction:      version.Partition.RecoveryInstruction,
		HumanOwnerRequired:       version.Partition.HumanOwnerRequired,
		UnsafeClass:              version.Partition.UnsafeClass,
		NoReconciliation:         version.Partition.NoProvenReconciliationPath,
		HumanOnlyControlMark:     version.HumanOnlyControlMark,
		TimelineText:             domain.TimelineText(domain.TimelineProposal),
		Current:                  current,
		Digest:                   version.Digest,
		AuthorizationAbsenceText: domain.AuthorizationAbsenceText,
		AuthorID:                 version.AuthorID,
		CreatedAt:                version.CreatedAt,
	}
	for _, row := range version.Snapshot {
		card.Snapshot = append(card.Snapshot, api.PreviewSnapshotRowView{
			MaterialField:   row.MaterialField,
			Status:          row.Status,
			ExternalID:      row.ExternalID,
			ExternalVersion: row.ExternalVersion,
			ObservedAt:      row.ObservedAt,
			EvidenceRef:     row.EvidenceRef,
		})
	}
	for _, effect := range version.Effects {
		card.Effects = append(card.Effects, api.PreviewEffectView{
			Order:            effect.Order,
			ActionRef:        effect.ActionRef,
			DependsOn:        effect.DependsOn,
			FailureSemantics: effect.FailureSemantics,
		})
	}
	// The keyed digest re-verification of the stored row (the integrity
	// alarm of the read: the stored digest is the identity of the exact
	// bytes the human saw — a mismatch is the spoken alarm, never a
	// masked silence; the store re-verified at the build, the view
	// re-verifies at every read).
	if len(key) > 0 {
		if digest := domain.PreviewDigest(key, version.PreviewID, version.VersionNo, domain.PreviewCanonicalBytes(version)); digest != version.Digest {
			card.DigestAlarm = true
		}
	}
	// The Timeline word over the events of the EXACT version.
	versionEvents := []domain.PreviewEvent{}
	for _, event := range events {
		if event.PreviewID == version.PreviewID && event.VersionNo == version.VersionNo {
			versionEvents = append(versionEvents, domain.PreviewEvent{Kind: event.Kind, Reason: event.Reason, Details: event.Details, At: event.CreatedAt, ActorID: event.ActorID})
		}
	}
	card.Timeline = domain.DerivePreviewState(versionEvents)
	card.TimelineText = domain.TimelineText(card.Timeline)
	// The live derivations: the readiness needs the definition's required
	// facts; the freshness needs the live pins. An unresolvable definition
	// pin answers the honest blocked/stale words — the audit readability
	// never depends on the dictionary staying alive.
	if defResolved {
		card.Readiness = domain.DerivePreviewReadiness(def, version.Snapshot)
		freshness := domain.DerivePreviewFreshness(version, fresh)
		card.FreshnessStale = freshness.Stale
		card.FreshnessReason = freshness.Reason
		if freshness.Stale {
			card.FreshnessReasonText = domain.FreshnessReasonText(freshness.Reason)
		}
	} else {
		card.Readiness = domain.PreviewBlockedMaterial
		card.FreshnessStale = true
		card.FreshnessReason = domain.FreshnessPolicyChanged
		card.FreshnessReasonText = domain.FreshnessReasonText(domain.FreshnessPolicyChanged)
	}
	card.ReadinessText = domain.PreviewReadinessText(card.Readiness)
	return card
}

// diffValueWord renders one side of a diff line with the honest bound
// (the long values are spoken-truncated, never silently cut).
func diffValueWord(value string) string {
	const bound = 120
	runes := []rune(value)
	if len(runes) <= bound {
		return value
	}
	return string(runes[:bound]) + "…"
}

// derivePreviewDiff derives the material diff of two adjacent versions of
// one thread — the read-time derivation over the pair (NEVER stored):
// the changed typed fields with the old→new values, the changed snapshot
// statuses, the partition changes and the lineage pin moves, in words.
func derivePreviewDiff(old, new api.PreviewCardView) []string {
	lines := []string{}
	seen := map[string]bool{}
	for name, value := range new.TypedInput {
		seen[name] = true
		oldValue, existed := old.TypedInput[name]
		if !existed {
			lines = append(lines, fmt.Sprintf("Поле «%s» добавлено: «%s».", name, diffValueWord(value)))
			continue
		}
		if oldValue != value {
			lines = append(lines, fmt.Sprintf("Поле «%s» изменено: «%s» → «%s».", name, diffValueWord(oldValue), diffValueWord(value)))
		}
	}
	for name := range old.TypedInput {
		if !seen[name] {
			lines = append(lines, fmt.Sprintf("Поле «%s» удалено (значение было: «%s»).", name, diffValueWord(old.TypedInput[name])))
		}
	}
	oldStatuses := map[string]string{}
	for _, row := range old.Snapshot {
		oldStatuses[row.MaterialField] = row.Status
	}
	newStatuses := map[string]string{}
	for _, row := range new.Snapshot {
		newStatuses[row.MaterialField] = row.Status
	}
	for field, status := range newStatuses {
		if oldStatuses[field] != status {
			lines = append(lines, fmt.Sprintf("Материальное поле «%s»: статус «%s» → «%s».", field, oldStatuses[field], status))
		}
	}
	for field, status := range oldStatuses {
		if _, ok := newStatuses[field]; !ok {
			lines = append(lines, fmt.Sprintf("Материальное поле «%s» исчезло из snapshot (статус был: «%s»).", field, status))
		}
	}
	if old.Partition != new.Partition {
		lines = append(lines, fmt.Sprintf("Разграничение изменилось: «%s» → «%s».", old.Partition, new.Partition))
	}
	if old.OwnerEpoch != new.OwnerEpoch {
		lines = append(lines, fmt.Sprintf("Эпоха владельца: %d → %d.", old.OwnerEpoch, new.OwnerEpoch))
	}
	return lines
}

// NewPreviewVersionFromEdit re-runs the FULL chain over the edited
// material and lands the next append-only version: the rights gate, the
// thread and its CURRENT version (the honest 404), the definition of the
// thread's pin, the live case material, the strict validation of the
// edited typed input (the honest 400 of the violation words — nothing is
// written, the old version stays fully current), the LIVE policy gates
// (the honest 403 of the reason word), the fresh snapshot, the digest of
// the new version — and the ONE transaction that lands the new version
// row with the superseded event of the old version (the atomic supersede:
// a failed build leaves no partial state). The edit consumes NO attempt
// of the AD-7 ladder (the ladder belongs to the proposal submission; the
// edit is a command on the existing preview).
func (s *Service) NewPreviewVersionFromEdit(ctx context.Context, command api.NewPreviewVersionCommand) (api.PreviewVersionOutcome, error) {
	if err := s.requirePreviewPorts(); err != nil {
		return api.PreviewVersionOutcome{}, err
	}
	scope, err := s.authorizeProposal(ctx, command.Actor, command.TenantID)
	if err != nil {
		return api.PreviewVersionOutcome{}, err
	}
	versions, err := s.previews.ReadPreviewVersions(ctx, scope, command.CaseID)
	if err != nil {
		return api.PreviewVersionOutcome{}, err
	}
	var current *domain.ActionPreview
	for i := range versions {
		if versions[i].PreviewID != command.PreviewID {
			continue
		}
		if current == nil || versions[i].VersionNo > current.VersionNo {
			current = &versions[i]
		}
	}
	if current == nil {
		return api.PreviewVersionOutcome{}, domain.ErrNotFound
	}
	def, ok := actionCatalog.GetFrom(s.defs, current.DefinitionID, current.DefinitionVersion)
	if !ok {
		return api.PreviewVersionOutcome{}, domain.ErrNotFound
	}
	caseView, err := s.caseMaterial.CaseView(ctx, command.Actor, command.TenantID, command.CaseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return api.PreviewVersionOutcome{}, domain.ErrNotFound
		}
		return api.PreviewVersionOutcome{}, err
	}
	material := caseViewMaterial{view: caseView}
	// The edited submission over the thread's OWN pins: the target, the
	// definition pin and the work item are the preview's identity — the
	// edit only rewrites the typed material and the supporting refs; the
	// owner epoch pin re-resolves LIVE (the edit re-runs the full chain,
	// the world is not obliged to be the same).
	proposal := domain.Proposal{
		DefinitionID:           current.DefinitionID,
		DefinitionVersion:      current.DefinitionVersion,
		CaseID:                 current.CaseID,
		WorkItemID:             current.WorkItemID,
		Target:                 current.Target,
		TypedInput:             command.TypedInput,
		SupportingFacts:        command.SupportingFacts,
		SupportingObservations: command.SupportingObservations,
		OwnerEpoch:             material.ownerEpoch(),
		AttemptNo:              1,
		AuthorID:               command.Actor.ID,
	}
	if violations := domain.GuardValidateProposal(def, proposal); len(violations) > 0 {
		return api.PreviewVersionOutcome{}, domain.RuleError{Rule: "edit_preview_version", Reason: domain.InvalidReportText(violations)}
	}
	live, err := s.gatherPreviewLive(ctx, command.Actor, scope, def, material)
	if err != nil {
		return api.PreviewVersionOutcome{}, err
	}
	policy := domain.GuardPreparePolicy(def, proposal.OwnerEpoch, live.policy)
	if policy.Outcome == domain.PolicyBlock {
		return api.PreviewVersionOutcome{}, domain.RuleError{Rule: "edit_preview_version", Reason: domain.PolicyReasonText(policy.Reason)}
	}
	facts, err := s.caseMaterial.FactsView(ctx, command.Actor, command.TenantID, command.CaseID)
	if err != nil {
		return api.PreviewVersionOutcome{}, err
	}
	preview, err := s.buildPreviewMaterial(ctx, command.Actor, scope, def, proposal, material, live, policy.HumanOnlyMark, facts, command.PreviewID, current.VersionNo+1, command.Now)
	if err != nil {
		return api.PreviewVersionOutcome{}, err
	}
	supersededEventID, err := s.ids.New(command.Now)
	if err != nil {
		return api.PreviewVersionOutcome{}, err
	}
	previewedEventID, err := s.ids.New(command.Now)
	if err != nil {
		return api.PreviewVersionOutcome{}, err
	}
	if err := s.previews.BuildPreviewVersion(ctx, port.BuildPreviewVersionParams{
		Scope:             scope,
		Preview:           preview,
		Attempt:           nil,
		SupersedesVersion: current.VersionNo,
		SupersededEventID: supersededEventID,
		PreviewedEventID:  previewedEventID,
		Now:               command.Now,
	}); err != nil {
		return api.PreviewVersionOutcome{}, err
	}
	return api.PreviewVersionOutcome{
		Command:       "edit_preview_version",
		PreviewID:     command.PreviewID,
		VersionNo:     preview.VersionNo,
		Supersedes:    current.VersionNo,
		PreviewDigest: preview.Digest,
	}, nil
}

// RecordDryRun appends the validation evidence of one EXACT version: the
// version must exist (the honest 404), the adapter must be wired (a nil
// runner is the honest AG-4 refusal in words — no evidence row is ever
// created), the result word must belong to the closed dictionary, and
// the evidence event lands append-only. The Timeline words NEVER move
// (the kind is simply not in DerivePreviewState — AC-8, the structural
// honesty of the step-01 domain; the unit matrix pins it).
func (s *Service) RecordDryRun(ctx context.Context, command api.DryRunCommand) (api.DryRunOutcome, error) {
	if err := s.requirePreviewPorts(); err != nil {
		return api.DryRunOutcome{}, err
	}
	scope, err := s.authorizeProposal(ctx, command.Actor, command.TenantID)
	if err != nil {
		return api.DryRunOutcome{}, err
	}
	if s.dryRunner == nil {
		// AG-4: the production composition root wires no adapter — the
		// surface honestly refuses in words, the evidence is NOT created.
		return api.DryRunOutcome{}, domain.RuleError{Rule: "record_dry_run", Reason: domain.DryRunUnsupportedText}
	}
	versions, err := s.previews.ReadPreviewVersions(ctx, scope, command.CaseID)
	if err != nil {
		return api.DryRunOutcome{}, err
	}
	var pinned *domain.ActionPreview
	for i := range versions {
		if versions[i].PreviewID == command.PreviewID && versions[i].VersionNo == command.VersionNo {
			pinned = &versions[i]
			break
		}
	}
	if pinned == nil {
		return api.DryRunOutcome{}, domain.ErrNotFound
	}
	outcome, err := s.dryRunner.Validate(ctx, port.DryRunRequest{
		Scope:             scope,
		PreviewID:         pinned.PreviewID,
		VersionNo:         pinned.VersionNo,
		DefinitionID:      pinned.DefinitionID,
		DefinitionVersion: pinned.DefinitionVersion,
		Target:            pinned.Target,
		TypedInput:        pinned.TypedInput,
	})
	if err != nil {
		return api.DryRunOutcome{}, err
	}
	if err := domain.GuardDryRunResult(outcome.Result); err != nil {
		return api.DryRunOutcome{}, domain.RuleError{Rule: "record_dry_run", Reason: err.Error()}
	}
	eventID, err := s.ids.New(command.Now)
	if err != nil {
		return api.DryRunOutcome{}, err
	}
	details := outcome.Details
	if len([]rune(details)) > domain.MaxEventDetailsRunes {
		// The loyal backstop of the store CHECK (the honest spoken
		// truncation of the step-01 report discipline).
		details = string([]rune(details)[:domain.MaxEventDetailsRunes]) + " … (перечень усечён границей события)"
	}
	if err := s.previews.RecordDryRunEvent(ctx, port.RecordDryRunParams{
		Scope:     scope,
		PreviewID: command.PreviewID,
		VersionNo: command.VersionNo,
		CaseID:    command.CaseID,
		EventID:   eventID,
		Result:    outcome.Result,
		Details:   details,
		ActorID:   command.Actor.ID,
		Now:       command.Now,
	}); err != nil {
		return api.DryRunOutcome{}, err
	}
	return api.DryRunOutcome{Command: "record_dry_run", Result: outcome.Result, Details: details}, nil
}
