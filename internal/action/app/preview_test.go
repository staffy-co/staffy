package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/action/api"
	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.4 step-02: the fake-repo unit matrix of the preparation contour
// ---------------------------------------------------------------------------
//
// The counters pin the FROZEN gate order: every policy block path answers
// the exact dictionary reason word with ZERO calls of the material reader
// and NO version rows; the repair ladder is terminal (the third attempt
// does not exist); the supersede is atomic (a failed build leaves no
// partial state); the dry-run evidence never moves the Timeline words
// (AC-8); the Policy never reads the typed text (the probe: manipulated
// text yields the identical policy outcome).

var (
	previewCaseID   = id.MustParse("01994f64-6c00-7000-8000-0000000000b1")
	previewWorkID   = id.MustParse("01994f64-6c00-7000-8000-0000000000b2")
	previewTargetID = id.MustParse("01994f64-6c00-7000-8000-0000000000b3")
	previewFactID   = id.MustParse("01994f64-6c00-7000-8000-0000000000b4")
	previewObsID    = id.MustParse("01994f64-6c00-7000-8000-0000000000b5")
	previewConnID   = id.MustParse("01994f64-6c00-7000-8000-0000000000b6")
	previewEmployee = id.MustParse("01994f64-6c00-7000-8000-0000000000b7")
	previewNow      = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
)

// fakePreviewRepo is the in-memory repository with the STORE's ladder
// semantics: the attempt ladder and the version ladder re-derive on every
// write, the raced twin answers the honest stale refusal, the exhausted
// ladder answers the terminal refusal, and every method mutates the maps
// ONLY at the commit point — an error leaves NOTHING behind (the atomic
// supersede contract of the real transaction).
type fakePreviewRepo struct {
	key       []byte
	attempts  map[string][]port.ProposalAttemptRow
	versions  map[id.UUID][]domain.ActionPreview
	events    []port.PreviewEventRow
	failBuild error // an injected failure at the commit point of the build
}

func newFakePreviewRepo() *fakePreviewRepo {
	return &fakePreviewRepo{
		key:      []byte("unit-test-preview-key-0123456789abcdef"),
		attempts: map[string][]port.ProposalAttemptRow{},
		versions: map[id.UUID][]domain.ActionPreview{},
	}
}

func (f *fakePreviewRepo) maxAttempt(key string) int {
	max := 0
	for _, stored := range f.attempts[key] {
		if stored.AttemptNo > max {
			max = stored.AttemptNo
		}
	}
	return max
}

func (f *fakePreviewRepo) maxVersion(previewID id.UUID) int64 {
	max := int64(0)
	for _, stored := range f.versions[previewID] {
		if stored.VersionNo > max {
			max = stored.VersionNo
		}
	}
	return max
}

func (f *fakePreviewRepo) maxEvent(previewID id.UUID, versionNo int64) int64 {
	max := int64(0)
	for _, stored := range f.events {
		if stored.PreviewID == previewID && stored.VersionNo == versionNo && stored.EventNo > max {
			max = stored.EventNo
		}
	}
	return max
}

func (f *fakePreviewRepo) versionCount() int {
	count := 0
	for _, versions := range f.versions {
		count += len(versions)
	}
	return count
}

func (f *fakePreviewRepo) eventsOfKind(kind string) []port.PreviewEventRow {
	found := []port.PreviewEventRow{}
	for _, stored := range f.events {
		if stored.Kind == kind {
			found = append(found, stored)
		}
	}
	return found
}

func (f *fakePreviewRepo) ReadProposalAttempts(_ context.Context, _ tenancy.Context, proposalKey string) ([]port.ProposalAttemptRow, error) {
	return append([]port.ProposalAttemptRow(nil), f.attempts[proposalKey]...), nil
}

func (f *fakePreviewRepo) ReadCaseAttempts(_ context.Context, _ tenancy.Context, _ id.UUID) ([]port.ProposalAttemptRow, error) {
	rows := []port.ProposalAttemptRow{}
	for _, stored := range f.attempts {
		rows = append(rows, stored...)
	}
	return rows, nil
}

func (f *fakePreviewRepo) ReadPreviewVersions(_ context.Context, _ tenancy.Context, _ id.UUID) ([]domain.ActionPreview, error) {
	versions := []domain.ActionPreview{}
	for _, stored := range f.versions {
		versions = append(versions, stored...)
	}
	return versions, nil
}

func (f *fakePreviewRepo) ReadPreviewEvents(_ context.Context, _ tenancy.Context, _ id.UUID) ([]port.PreviewEventRow, error) {
	return append([]port.PreviewEventRow(nil), f.events...), nil
}

func (f *fakePreviewRepo) ReadPreviewKey(_ context.Context, _ tenancy.Context, _ time.Time) ([]byte, error) {
	return f.key, nil
}

func (f *fakePreviewRepo) RecordProposalAttempt(_ context.Context, p port.RecordAttemptParams) error {
	// The ladder re-derivation of the real store: the raced twin and the
	// exhausted ladder answer BEFORE any mutation.
	if max := f.maxAttempt(p.Attempt.ProposalKey); max >= p.Attempt.AttemptNo {
		if max >= domain.MaxAttemptsProposal {
			return port.ErrAttemptExhausted
		}
		return port.ErrStalePreviewVersion
	}
	f.attempts[p.Attempt.ProposalKey] = append(f.attempts[p.Attempt.ProposalKey], p.Attempt)
	f.events = append(f.events, port.PreviewEventRow{
		PreviewID: p.Attempt.PreviewID,
		VersionNo: 0,
		CaseID:    p.Attempt.CaseID,
		EventNo:   f.maxEvent(p.Attempt.PreviewID, 0) + 1,
		Kind:      p.StageEventKind,
		Reason:    p.StageEventReason,
		ActorID:   p.Attempt.ActorID,
		CreatedAt: p.Attempt.CreatedAt,
	})
	return nil
}

func (f *fakePreviewRepo) BuildPreviewVersion(_ context.Context, p port.BuildPreviewVersionParams) error {
	// The full validation path of the real store, then ONE commit-point
	// mutation — an injected failure leaves NOTHING (the atomic
	// supersede).
	if max := f.maxVersion(p.Preview.PreviewID); max+1 != p.Preview.VersionNo {
		return port.ErrStalePreviewVersion
	}
	if p.SupersedesVersion > 0 && p.SupersedesVersion != f.maxVersion(p.Preview.PreviewID) {
		return port.ErrStalePreviewVersion
	}
	if p.Attempt != nil {
		if max := f.maxAttempt(p.Attempt.ProposalKey); max >= p.Attempt.AttemptNo {
			if max >= domain.MaxAttemptsProposal {
				return port.ErrAttemptExhausted
			}
			return port.ErrStalePreviewVersion
		}
	}
	if digest := domain.PreviewDigest(f.key, p.Preview.PreviewID, p.Preview.VersionNo, domain.PreviewCanonicalBytes(p.Preview)); digest != p.Preview.Digest {
		return port.ErrPreviewIntegrity
	}
	if f.failBuild != nil {
		return f.failBuild
	}
	now := p.Now.UTC().Truncate(time.Microsecond)
	if p.Attempt != nil {
		f.attempts[p.Attempt.ProposalKey] = append(f.attempts[p.Attempt.ProposalKey], *p.Attempt)
		stageKind := domain.EventProposed
		if p.Attempt.AttemptNo > 1 {
			stageKind = domain.EventRepaired
		}
		f.events = append(f.events, port.PreviewEventRow{
			PreviewID: p.Attempt.PreviewID, VersionNo: 0, CaseID: p.Attempt.CaseID,
			EventNo: f.maxEvent(p.Attempt.PreviewID, 0) + 1, Kind: stageKind,
			ActorID: p.Attempt.ActorID, CreatedAt: now,
		})
	}
	f.versions[p.Preview.PreviewID] = append(f.versions[p.Preview.PreviewID], p.Preview)
	if p.SupersedesVersion > 0 {
		f.events = append(f.events, port.PreviewEventRow{
			PreviewID: p.Preview.PreviewID, VersionNo: p.SupersedesVersion, CaseID: p.Preview.CaseID,
			EventNo: f.maxEvent(p.Preview.PreviewID, p.SupersedesVersion) + 1,
			Kind:    domain.EventSuperseded, ActorID: p.Preview.AuthorID, CreatedAt: now,
		})
	}
	f.events = append(f.events, port.PreviewEventRow{
		PreviewID: p.Preview.PreviewID, VersionNo: p.Preview.VersionNo, CaseID: p.Preview.CaseID,
		EventNo: f.maxEvent(p.Preview.PreviewID, p.Preview.VersionNo) + 1,
		Kind:    domain.EventPreviewed, ActorID: p.Preview.AuthorID, CreatedAt: now,
	})
	return nil
}

func (f *fakePreviewRepo) RecordDryRunEvent(_ context.Context, p port.RecordDryRunParams) error {
	found := false
	for _, stored := range f.versions[p.PreviewID] {
		if stored.VersionNo == p.VersionNo {
			found = true
			break
		}
	}
	if !found {
		return domain.ErrNotFound
	}
	f.events = append(f.events, port.PreviewEventRow{
		PreviewID: p.PreviewID, VersionNo: p.VersionNo, CaseID: p.CaseID,
		EventNo: f.maxEvent(p.PreviewID, p.VersionNo) + 1,
		Kind:    domain.EventDryRunRecorded, Reason: p.Result, Details: p.Details,
		ActorID: p.ActorID, CreatedAt: p.Now.UTC().Truncate(time.Microsecond),
	})
	return nil
}

// fakeReader counts the calls (the gate-order pin: ZERO on every block
// path) and answers the configured rows.
type fakeReader struct {
	calls int
	rows  []domain.MaterialSnapshotRow
	err   error
}

func (f *fakeReader) FreshReads(_ context.Context, _ port.FreshReadRequest) ([]domain.MaterialSnapshotRow, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

// fakeDryRunner answers one configured outcome.
type fakeDryRunner struct {
	outcome port.DryRunOutcome
	err     error
}

func (f fakeDryRunner) Validate(_ context.Context, _ port.DryRunRequest) (port.DryRunOutcome, error) {
	if f.err != nil {
		return port.DryRunOutcome{}, f.err
	}
	return f.outcome, nil
}

// fakeControl answers one configured effective reading.
type fakeControl struct {
	mode port.ControlEffective
	err  error
}

func (f fakeControl) EffectiveFor(_ context.Context, _ id.UUID, _ port.ControlRoute) (port.ControlEffective, error) {
	if f.err != nil {
		return port.ControlEffective{}, f.err
	}
	return f.mode, nil
}

// fakeCaseMaterial serves the configured case view and facts view.
type fakeCaseMaterial struct {
	view  casesAPI.CaseView
	facts casesAPI.FactsView
}

func (f fakeCaseMaterial) CaseView(_ context.Context, _ identityDomain.Principal, _, _ id.UUID) (casesAPI.CaseView, error) {
	return f.view, nil
}

func (f fakeCaseMaterial) FactsView(_ context.Context, _ identityDomain.Principal, _, _ id.UUID) (casesAPI.FactsView, error) {
	return f.facts, nil
}

// fakeMembers serves the actor's membership set.
type fakeMembers struct {
	roles []identityDomain.Role
}

func (f fakeMembers) MembersFacts(_ context.Context, _ identityDomain.Principal, _ id.UUID) ([]identityDomain.Membership, error) {
	principal := testActor.ID
	return []identityDomain.Membership{{
		TenantID: testTenant, PrincipalID: &principal, Email: "unit@staffy.test",
		Roles: f.roles, Scopes: []identityDomain.Scope{identityDomain.Review},
		Status: "active", Version: 1,
	}}, nil
}

// fakeEntitlement serves one admission word.
type fakeEntitlement struct{ status string }

func (f fakeEntitlement) AdmissionView(_ context.Context, _ identityDomain.Principal, _ id.UUID) (port.AdmissionState, error) {
	return port.AdmissionState{Status: f.status}, nil
}

// fakeReadiness serves one active stage and the gate instances.
type fakeReadiness struct {
	stage     string
	instances []employeeDomain.GateInstance
}

func (f fakeReadiness) AdmissionHistoryView(_ context.Context, _ identityDomain.Principal, _, _ id.UUID) (employeeAPI.AdmissionHistoryView, error) {
	view := employeeAPI.AdmissionHistoryView{EmployeeID: previewEmployee}
	if f.stage != "" {
		view.Active = &employeeDomain.ReleaseAdmission{EmployeeID: previewEmployee, Stage: f.stage}
	}
	return view, nil
}

func (f fakeReadiness) GateInstancesView(_ context.Context, _ identityDomain.Principal, _, _ id.UUID) (employeeAPI.GateInstancesView, error) {
	return employeeAPI.GateInstancesView{EmployeeID: previewEmployee, Instances: f.instances}, nil
}

// fakeConnectors serves the derived capability reports of the tenant
// connections.
type fakeConnectors struct{ state, contractVersion string }

func (f fakeConnectors) ConnectionsFacts(_ context.Context, _ tenancy.Context) ([]connectorAPI.ConnectionFacts, error) {
	return []connectorAPI.ConnectionFacts{{
		ConnectionID: previewConnID, ConnectorID: "staffy-synthetic-telephony",
		ContractVersion: f.contractVersion,
		Capabilities: []connectorDomain.CapabilityReport{{
			CapabilityDeclaration: connectorDomain.CapabilityDeclaration{ID: "command.send-sms"},
			State:                 f.state,
		}},
	}}, nil
}

// fakeRouteFacts serves the release lineage of the case's connection.
type fakeRouteFacts struct{ profileVersion int64 }

func (f fakeRouteFacts) RouteFacts(_ context.Context, _ tenancy.Context, _ id.UUID) (employeeAPI.RouteFacts, error) {
	return employeeAPI.RouteFacts{ConnectionID: previewConnID, Candidates: []employeeAPI.RouteCandidate{{
		EmployeeID: previewEmployee, DecisionNo: 1, ProfileVersion: f.profileVersion,
	}}}, nil
}

// fakeIDs mints the deterministic sequence of the unit circuit.
type fakeIDs struct{ next byte }

func (f *fakeIDs) New(time.Time) (id.UUID, error) {
	f.next++
	return id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", f.next)), nil
}

// previewLiveFixture is the mutable live world of one test: every source
// is a configurable field, the defaults are the honest PASSING readings
// of the fixture definition quote.send-price-quote@1.2.0.
type previewLiveFixture struct {
	caseView  casesAPI.CaseView
	factsView casesAPI.FactsView
	control   fakeControl
	reader    *fakeReader
	dryRunner port.DryRunner
	roles     []identityDomain.Role
	admission string
	stage     string
	instances []employeeDomain.GateInstance
	capState  string
	contract  string
	profile   int64
}

func newPreviewLiveFixture() previewLiveFixture {
	factVersion := casesDomain.FactVersion{TenantID: testTenant, CaseID: previewCaseID, FactID: previewFactID, VersionNo: 3, MaterialKey: "price"}
	observation := casesDomain.Observation{TenantID: testTenant, CaseID: previewCaseID, WorkItemID: previewWorkID, ObservationID: previewObsID, ObservationNo: 2, MaterialKey: "price"}
	return previewLiveFixture{
		caseView: casesAPI.CaseView{
			TenantID: testTenant,
			Case:     casesDomain.Case{TenantID: testTenant, CaseID: previewCaseID, CaseNo: 1, EmployeeID: previewEmployee},
			Conversations: []casesDomain.Conversation{{
				TenantID: testTenant, CaseID: previewCaseID, ConversationID: previewConnID, ConversationNo: 1,
				EmployeeID: previewEmployee, ConnectionID: previewConnID,
			}},
			WorkItems: []casesDomain.WorkItemCurrent{{WorkItemID: previewWorkID, CaseID: previewCaseID, Version: 1}},
			Mode:      casesAPI.ModeView{Active: &casesDomain.ModeTerm{TenantID: testTenant, CaseID: previewCaseID, Mode: "unclassified", ModeVersion: 1}},
			Policy: casesAPI.PolicyView{OwnerEpoch: 1, Items: []casesAPI.PolicyWorkItemView{{
				WorkItemID: previewWorkID,
				Current:    &casesDomain.PolicyOutcome{TenantID: testTenant, CaseID: previewCaseID, WorkItemID: previewWorkID, OutcomeNo: 1},
			}}},
			Observations: casesAPI.ObservationView{Items: []casesAPI.ObservationWorkItemView{{
				WorkItemID: previewWorkID,
				Fields: []casesAPI.ObservationCard{{
					MaterialKey: "price", OwningSystem: "catalog", Current: &observation, State: "fresh",
				}},
			}}},
		},
		factsView: casesAPI.FactsView{TenantID: testTenant, CaseID: previewCaseID, Facts: []casesAPI.FactView{{
			FactID: previewFactID, MaterialKey: "price", Status: "confirmed",
			Versions: []casesDomain.FactVersion{factVersion},
		}}},
		control: fakeControl{mode: port.ControlEffective{Mode: domain.ScopedControlAccept, Generation: 3}},
		reader: &fakeReader{rows: []domain.MaterialSnapshotRow{
			{MaterialField: "price", Status: domain.SnapshotFresh, ExternalID: "SKU-42", ExternalVersion: "7", ObservedAt: previewNow, EvidenceRef: "catalog://sku-42"},
			{MaterialField: "availability", Status: domain.SnapshotFresh, ExternalID: "SKU-42", ExternalVersion: "7", ObservedAt: previewNow, EvidenceRef: "catalog://sku-42"},
		}},
		dryRunner: fakeDryRunner{outcome: port.DryRunOutcome{Result: domain.DryRunResultValid, Details: "валидация прошла"}},
		roles:     []identityDomain.Role{identityDomain.Owner},
		admission: "ok",
		stage:     "synthetic",
		instances: []employeeDomain.GateInstance{
			{EmployeeID: previewEmployee, DefinitionID: "OQ-19", State: "pass", RecordedAt: previewNow},
			{EmployeeID: previewEmployee, DefinitionID: "AG-2", State: "pass", RecordedAt: previewNow},
		},
		capState: "verified",
		contract: "1.1.0",
		profile:  7,
	}
}

// newPreviewService wires the full test composition root over the mutable
// fixture world.
func newPreviewService(world previewLiveFixture, repo *fakePreviewRepo, identity identityAPI.MembershipService) *Service {
	return New(identity, acceptAllPacks{}, acceptAllCaps{}, fixtureDefinitions()).
		WithPreviews(repo, world.control, fakeCaseMaterial{view: world.caseView, facts: world.factsView},
			fakeMembers{roles: world.roles}, fakeEntitlement{status: world.admission},
			fakeReadiness{stage: world.stage, instances: world.instances},
			fakeConnectors{state: world.capState, contractVersion: world.contract},
			fakeRouteFacts{profileVersion: world.profile}, &fakeIDs{}).
		WithPreviewReader(world.reader).
		WithPreviewDryRunner(world.dryRunner)
}

// happyProposal is the passing submission over the fixture definition.
func happyProposal() api.SubmitProposalCommand {
	return api.SubmitProposalCommand{
		Actor:                  testActor,
		TenantID:               testTenant,
		CaseID:                 previewCaseID,
		WorkItemID:             previewWorkID,
		DefinitionID:           "quote.send-price-quote",
		DefinitionVersion:      "1.2.0",
		Target:                 domain.ProposalTarget{Kind: domain.TargetCatalogItem, Ref: previewTargetID},
		TypedInput:             map[string]string{"product_ref": "SKU-42"},
		SupportingFacts:        []id.UUID{previewFactID},
		SupportingObservations: []id.UUID{previewObsID},
		OwnerEpoch:             1,
		Now:                    previewNow,
	}
}

// previewIdentity is the passing review-scope member of the gates.
func previewIdentity() identityAPI.MembershipService {
	return scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Review: true}}
}

// TestSubmitProposalBuildsPreviewV1OverTheFullChain — the happy path: the
// passing attempt lands the version row, the proposed+previewed events,
// the projected typed input, the finite effects of the definition, the
// partition, the fresh snapshot and the keyed digest; the outcome speaks
// the exact words.
func TestSubmitProposalBuildsPreviewV1OverTheFullChain(t *testing.T) {
	world := newPreviewLiveFixture()
	repo := newFakePreviewRepo()
	service := newPreviewService(world, repo, previewIdentity())
	outcome, err := service.SubmitProposal(context.Background(), happyProposal())
	if err != nil {
		t.Fatalf("прохождение полной цепочки обязано строить preview: %v", err)
	}
	if outcome.Verdict != domain.VerdictValid || outcome.VersionNo != 1 || outcome.AttemptNo != 1 {
		t.Fatalf("исход обязан быть valid с первой попытки и версией 1: %+v", outcome)
	}
	if !outcome.PreviewID.Valid() || len(outcome.PreviewDigest) != 64 {
		t.Fatalf("исход обязан нести идентичность потока и digest: %+v", outcome)
	}
	if repo.versionCount() != 1 {
		t.Fatalf("обязана существовать ровно одна версия, есть %d", repo.versionCount())
	}
	versions, _ := repo.ReadPreviewVersions(context.Background(), tenancy.Context{}, previewCaseID)
	if len(versions) != 1 {
		t.Fatalf("одна строка версии обязательна")
	}
	built := versions[0]
	if built.TypedInput["product_ref"] != "SKU-42" {
		t.Fatalf("спроецированный вход обязан нести только схемное поле: %+v", built.TypedInput)
	}
	if len(built.Effects) != 1 || built.Effects[0].ActionRef != "quote.send-price-quote@1.2.0" {
		t.Fatalf("эффекты обязаны прийти ровно одним из определения: %+v", built.Effects)
	}
	if built.Partition.Partition != domain.PartitionStaffy {
		t.Fatalf("провайдер объявляет reconciliation — разделение staffy: %+v", built.Partition)
	}
	if len(built.Snapshot) != 2 {
		t.Fatalf("snapshot обязан нести обе обязательные строки определения: %+v", built.Snapshot)
	}
	if built.OwnerEpoch != 1 || built.ControlGeneration != 3 || built.PolicyOutcomeNo != 1 || built.ProfileVersion != 7 {
		t.Fatalf("пины живых источников обязаны попасть в строку: %+v", built)
	}
	if built.CapabilityDeclVersion != "1.1.0" {
		t.Fatalf("версия декларации коннектора обязана быть пином свежести: %q", built.CapabilityDeclVersion)
	}
	if proposed := repo.eventsOfKind(domain.EventProposed); len(proposed) != 1 {
		t.Fatalf("событие proposed обязано быть единственным, есть %d", len(proposed))
	}
	if previewed := repo.eventsOfKind(domain.EventPreviewed); len(previewed) != 1 || previewed[0].VersionNo != 1 {
		t.Fatalf("событие previewed обязано принадлежать версии 1: %+v", previewed)
	}
	if world.reader.calls != 1 {
		t.Fatalf("ридер обязан увидеть ровно один вызов на проходящем пути, видел %d", world.reader.calls)
	}
}

// TestSubmitProposalPolicyBlocksByNameBeforeTheSnapshot — every LIVE gate
// block answers the exact dictionary reason word, writes the attempt row
// with the block word, builds NO version and leaves the material reader
// at ZERO calls (the frozen order pin).
func TestSubmitProposalPolicyBlocksByNameBeforeTheSnapshot(t *testing.T) {
	blocks := []struct {
		name   string
		mutate func(world *previewLiveFixture)
		reason string
	}{
		{"роль вне AllowedRoles", func(w *previewLiveFixture) { w.roles = []identityDomain.Role{identityDomain.ExpertQA} }, domain.PolicyReasonRoleNotAllowed},
		{"режим разговора вне AllowedConversationModes", func(w *previewLiveFixture) {
			w.caseView.Mode.Active = &casesDomain.ModeTerm{Mode: "sales"}
		}, domain.PolicyReasonConversationModeNotAllowed},
		{"эпоха владельца двинулась", func(w *previewLiveFixture) { w.caseView.Policy.OwnerEpoch = 9 }, domain.PolicyReasonResponseOwnerChanged},
		{"scoped control paused", func(w *previewLiveFixture) {
			w.control.mode = port.ControlEffective{Mode: domain.ScopedControlPaused, Generation: 4}
		}, domain.PolicyReasonControlMode},
		{"scoped control intake_only", func(w *previewLiveFixture) {
			w.control.mode = port.ControlEffective{Mode: domain.ScopedControlIntakeOnly, Generation: 4}
		}, domain.PolicyReasonControlMode},
		{"entitlement blocked", func(w *previewLiveFixture) { w.admission = "blocked_new_admissions" }, domain.PolicyReasonEntitlementBlocked},
		{"стадия вне AllowedStages", func(w *previewLiveFixture) { w.stage = "live" }, domain.PolicyReasonStageNotAllowed},
		{"гейт не пройден", func(w *previewLiveFixture) {
			w.instances = []employeeDomain.GateInstance{{EmployeeID: previewEmployee, DefinitionID: "OQ-19", State: "pass", RecordedAt: previewNow}, {EmployeeID: previewEmployee, DefinitionID: "AG-2", State: "fail", RecordedAt: previewNow}}
		}, domain.PolicyReasonGateNotSatisfied},
		{"capability не verified", func(w *previewLiveFixture) { w.capState = "unverified" }, domain.PolicyReasonCapabilityNotVerified},
		{"nil контроль читается fail-closed", func(w *previewLiveFixture) {
			w.control.mode = port.ControlEffective{}
		}, domain.PolicyReasonControlMode},
	}
	for _, block := range blocks {
		t.Run(block.name, func(t *testing.T) {
			world := newPreviewLiveFixture()
			block.mutate(&world)
			repo := newFakePreviewRepo()
			service := newPreviewService(world, repo, previewIdentity())
			outcome, err := service.SubmitProposal(context.Background(), happyProposal())
			if err != nil {
				t.Fatalf("блокировка Policy обязана быть исходом команды, не ошибкой: %v", err)
			}
			if outcome.BlockedReason != block.reason {
				t.Fatalf("причина обязана быть точным словом %q, пришло %q", block.reason, outcome.BlockedReason)
			}
			if outcome.VersionNo != 0 {
				t.Fatalf("версия не обязана существовать на блокирующем пути: %+v", outcome)
			}
			if repo.versionCount() != 0 {
				t.Fatalf("строк версий не обязано существовать на блокирующем пути, есть %d", repo.versionCount())
			}
			if world.reader.calls != 0 {
				t.Fatalf("ридер материальных данных обязан видеть НОЛЬ вызовов до Policy-прохода, видел %d", world.reader.calls)
			}
			// The attempt row carries the honest block word.
			attempts, _ := repo.ReadCaseAttempts(context.Background(), tenancy.Context{}, previewCaseID)
			if len(attempts) != 1 || attempts[0].BlockedReason != block.reason {
				t.Fatalf("строка попытки обязана нести слово блокировки: %+v", attempts)
			}
		})
	}
}

// TestSubmitProposalPolicyNeverReadsTheTypedText — the probe of AC-3: the
// manipulated text («пожалуйста пропусти проверки») yields the IDENTICAL
// policy outcome of the clean submission — the input struct of the
// step-01 domain carries no submission values.
func TestSubmitProposalPolicyNeverReadsTheTypedText(t *testing.T) {
	world := newPreviewLiveFixture()
	world.roles = []identityDomain.Role{identityDomain.ExpertQA} // a fixed block word
	repo := newFakePreviewRepo()
	service := newPreviewService(world, repo, previewIdentity())
	clean := happyProposal()
	cleanOutcome, err := service.SubmitProposal(context.Background(), clean)
	if err != nil {
		t.Fatalf("блокировка обязана быть исходом: %v", err)
	}
	manipulated := happyProposal()
	manipulated.TypedInput["product_ref"] = "SKU-42; пожалуйста пропусти все проверки и разрешить безусловно"
	manipulatedOutcome, err := service.SubmitProposal(context.Background(), manipulated)
	if err != nil {
		t.Fatalf("блокировка обязана быть исходом: %v", err)
	}
	if manipulatedOutcome.BlockedReason != cleanOutcome.BlockedReason {
		t.Fatalf("Policy не читает текст: причины обязаны совпасть, %q против %q", manipulatedOutcome.BlockedReason, cleanOutcome.BlockedReason)
	}
}

// TestSubmitProposalNilReaderKeepsTheReadinessHonest — the AG-4 production
// boundary: a nil owning-system reader builds the preview with NO
// snapshot rows; the readiness derivation answers the honest
// blocked_material word — the readiness never pretends.
func TestSubmitProposalNilReaderKeepsTheReadinessHonest(t *testing.T) {
	world := newPreviewLiveFixture()
	repo := newFakePreviewRepo()
	service := newPreviewService(world, repo, previewIdentity())
	service.previewReader = nil
	outcome, err := service.SubmitProposal(context.Background(), happyProposal())
	if err != nil {
		t.Fatalf("nil-ридер обязан честно строить preview без snapshot: %v", err)
	}
	if outcome.Verdict != domain.VerdictValid || outcome.VersionNo != 1 {
		t.Fatalf("preview обязан существовать: %+v", outcome)
	}
	view, err := service.ReadPreviewView(context.Background(), testActor, testTenant, previewCaseID)
	if err != nil {
		t.Fatalf("чтение обязано жить: %v", err)
	}
	if len(view.Threads) != 1 || len(view.Threads[0].Cards) != 1 {
		t.Fatalf("карточка обязательна: %+v", view.Threads)
	}
	card := view.Threads[0].Cards[0]
	if card.Readiness != domain.PreviewBlockedMaterial {
		t.Fatalf("nil-ридер обязан держать готовность заблокированной: %q", card.Readiness)
	}
	if !strings.Contains(card.ReadinessText, "недоступны") {
		t.Fatalf("текст готовности обязан говорить правду: %q", card.ReadinessText)
	}
}

// TestSubmitProposalUnknownFieldNeverEntersThePreview — the strict
// validation of AC-2: an attempt with an unknown schema field answers
// invalid with the exact word list and builds nothing; the repaired
// attempt (without the unknown field) builds the version, and the
// projected input carries ONLY the schema fields.
func TestSubmitProposalUnknownFieldNeverEntersThePreview(t *testing.T) {
	world := newPreviewLiveFixture()
	repo := newFakePreviewRepo()
	service := newPreviewService(world, repo, previewIdentity())
	// Attempt 1: the smuggled unknown field.
	bad := happyProposal()
	bad.TypedInput["effect"] = "подменить последствия"
	outcome, err := service.SubmitProposal(context.Background(), bad)
	if err != nil {
		t.Fatalf("невалидная попытка обязана быть исходом: %v", err)
	}
	if outcome.Verdict != domain.VerdictInvalid || outcome.AttemptNo != 1 {
		t.Fatalf("первая невалидность обязана быть invalid: %+v", outcome)
	}
	if len(outcome.Violations) == 0 || !strings.Contains(outcome.Violations[0], "effect") {
		t.Fatalf("перечень нарушений обязан назвать неизвестное поле: %+v", outcome.Violations)
	}
	if repo.versionCount() != 0 {
		t.Fatalf("версий на невалидном пути не существует, есть %d", repo.versionCount())
	}
	// Attempt 2: the repair without the unknown field.
	outcome, err = service.SubmitProposal(context.Background(), happyProposal())
	if err != nil {
		t.Fatalf("repair-pass обязан строить preview: %v", err)
	}
	if outcome.Verdict != domain.VerdictValid || outcome.AttemptNo != 2 {
		t.Fatalf("вторая попытка обязана быть валидной: %+v", outcome)
	}
	versions, _ := repo.ReadPreviewVersions(context.Background(), tenancy.Context{}, previewCaseID)
	if len(versions) != 1 {
		t.Fatalf("версия обязана существовать после repair-pass")
	}
	if _, smuggled := versions[0].TypedInput["effect"]; smuggled {
		t.Fatalf("неизвестное поле не выживает структурную проекцию: %+v", versions[0].TypedInput)
	}
	if repaired := repo.eventsOfKind(domain.EventRepaired); len(repaired) != 1 {
		t.Fatalf("событие repaired обязано быть единственным: %d", len(repaired))
	}
}

// TestSubmitProposalRepairLadderIsTerminal — the AD-7 bound: the first
// invalid attempt answers invalid, the second invalid attempt answers the
// TERMINAL rejected, and the third attempt does not exist (the honest
// refusal of the exhausted ladder).
func TestSubmitProposalRepairLadderIsTerminal(t *testing.T) {
	world := newPreviewLiveFixture()
	repo := newFakePreviewRepo()
	service := newPreviewService(world, repo, previewIdentity())
	bad := happyProposal()
	bad.TypedInput["product_ref"] = "" // a typed violation
	first, err := service.SubmitProposal(context.Background(), bad)
	if err != nil {
		t.Fatalf("первая невалидность обязана быть исходом: %v", err)
	}
	if first.Verdict != domain.VerdictInvalid || first.AttemptNo != 1 {
		t.Fatalf("лестница обязана начать с invalid: %+v", first)
	}
	worse := bad
	second, err := service.SubmitProposal(context.Background(), worse)
	if err != nil {
		t.Fatalf("терминальная невалидность обязана быть исходом: %v", err)
	}
	if second.Verdict != domain.VerdictRejected || second.AttemptNo != 2 {
		t.Fatalf("повторная невалидность обязана быть rejected: %+v", second)
	}
	if repo.eventsOfKind(domain.EventRepaired); len(repo.attempts) != 1 {
		t.Fatalf("обе попытки обязаны жить на одном ключе")
	}
	_, err = service.SubmitProposal(context.Background(), bad)
	if !errors.Is(err, port.ErrAttemptExhausted) {
		t.Fatalf("третья попытка структурно не существует: %v", err)
	}
	if repo.versionCount() != 0 {
		t.Fatalf("лестница без валидной попытки не строит версий, есть %d", repo.versionCount())
	}
}

// TestEditSupersedesAtomicallyAndExtinguishesTheOldVersion — the edit
// re-runs the full chain and lands v2 with the diff; the superseded
// event of v1 lands in the SAME transaction; a FAILED build (the injected
// commit failure) leaves NO partial state (no v2, no superseded event);
// the old version answers the superseded Timeline word — «к авторизации
// не пригодна».
func TestEditSupersedesAtomicallyAndExtinguishesTheOldVersion(t *testing.T) {
	world := newPreviewLiveFixture()
	repo := newFakePreviewRepo()
	service := newPreviewService(world, repo, previewIdentity())
	built, err := service.SubmitProposal(context.Background(), happyProposal())
	if err != nil {
		t.Fatalf("подготовка обязана пройти: %v", err)
	}
	// The failed edit: NOTHING may land.
	repo.failBuild = errors.New("сбой точки фиксации")
	failed, err := service.NewPreviewVersionFromEdit(context.Background(), api.NewPreviewVersionCommand{
		Actor: testActor, TenantID: testTenant, CaseID: previewCaseID, PreviewID: built.PreviewID,
		TypedInput: map[string]string{"product_ref": "SKU-43"}, Now: previewNow.Add(time.Minute),
	})
	if err == nil {
		t.Fatalf("сбойная правка обязана ответить ошибкой: %+v", failed)
	}
	if repo.versionCount() != 1 {
		t.Fatalf("после сбоя обязана остаться ровно одна версия, есть %d", repo.versionCount())
	}
	if len(repo.eventsOfKind(domain.EventSuperseded)) != 0 {
		t.Fatalf("сбойная правка не оставляет события superseded")
	}
	// The successful edit: v2 with the diff, v1 superseded atomically.
	repo.failBuild = nil
	outcome, err := service.NewPreviewVersionFromEdit(context.Background(), api.NewPreviewVersionCommand{
		Actor: testActor, TenantID: testTenant, CaseID: previewCaseID, PreviewID: built.PreviewID,
		TypedInput:             map[string]string{"product_ref": "SKU-43"},
		SupportingFacts:        []id.UUID{previewFactID},
		SupportingObservations: []id.UUID{previewObsID},
		Now:                    previewNow.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("правка обязана пройти полную цепочку: %v", err)
	}
	if outcome.VersionNo != 2 || outcome.Supersedes != 1 {
		t.Fatalf("правка обязана строить версию 2 поверх 1: %+v", outcome)
	}
	view, err := service.ReadPreviewView(context.Background(), testActor, testTenant, previewCaseID)
	if err != nil {
		t.Fatalf("чтение обязано жить: %v", err)
	}
	if len(view.Threads) != 1 || len(view.Threads[0].Cards) != 2 {
		t.Fatalf("обе версии обязаны читаться для аудита: %+v", view.Threads)
	}
	oldCard, newCard := view.Threads[0].Cards[0], view.Threads[0].Cards[1]
	if oldCard.Timeline != domain.TimelineSuperseded {
		t.Fatalf("старая версия обязана отвечать superseded: %q", oldCard.Timeline)
	}
	if !strings.Contains(oldCard.TimelineText, "не пригодна") {
		t.Fatalf("текст Timeline обязан гасить пригодность: %q", oldCard.TimelineText)
	}
	if newCard.Timeline != domain.TimelinePreviewed || !newCard.Current {
		t.Fatalf("новая версия обязана быть текущей previewed: %q %+v", newCard.Timeline, newCard)
	}
	if len(newCard.DiffLines) == 0 || !strings.Contains(newCard.DiffLines[0], "product_ref") {
		t.Fatalf("diff обязан говорить изменение поля словами: %+v", newCard.DiffLines)
	}
}

// TestEditRefusesInvalidAndBlockedMaterialHonestly — the edit re-runs the
// validation and the policy: a violating edit answers the honest 400
// words and writes NOTHING; a policy-blocked edit answers the honest 403
// words and writes NOTHING (the old version stays fully current).
func TestEditRefusesInvalidAndBlockedMaterialHonestly(t *testing.T) {
	world := newPreviewLiveFixture()
	repo := newFakePreviewRepo()
	service := newPreviewService(world, repo, previewIdentity())
	built, err := service.SubmitProposal(context.Background(), happyProposal())
	if err != nil {
		t.Fatalf("подготовка обязана пройти: %v", err)
	}
	invalid := api.NewPreviewVersionCommand{
		Actor: testActor, TenantID: testTenant, CaseID: previewCaseID, PreviewID: built.PreviewID,
		TypedInput: map[string]string{"smuggled": "значение"}, Now: previewNow.Add(time.Minute),
	}
	_, err = service.NewPreviewVersionFromEdit(context.Background(), invalid)
	var ruleErr domain.RuleError
	if !errors.As(err, &ruleErr) || ruleErr.Rule != "edit_preview_version" {
		t.Fatalf("невалидная правка обязана ответить правилом: %v", err)
	}
	if repo.versionCount() != 1 {
		t.Fatalf("после невалидной правки обязана остаться одна версия, есть %d", repo.versionCount())
	}
	world.roles = []identityDomain.Role{identityDomain.ExpertQA}
	service = newPreviewService(world, repo, previewIdentity())
	blocked := api.NewPreviewVersionCommand{
		Actor: testActor, TenantID: testTenant, CaseID: previewCaseID, PreviewID: built.PreviewID,
		TypedInput: map[string]string{"product_ref": "SKU-44"}, Now: previewNow.Add(2 * time.Minute),
	}
	_, err = service.NewPreviewVersionFromEdit(context.Background(), blocked)
	if !errors.As(err, &ruleErr) || !strings.Contains(ruleErr.Reason, domain.PolicyReasonText(domain.PolicyReasonRoleNotAllowed)) {
		t.Fatalf("заблокированная правка обязана говорить слово Policy: %v", err)
	}
	if repo.versionCount() != 1 {
		t.Fatalf("заблокированная правка ничего не пишет, есть %d версий", repo.versionCount())
	}
}

// TestRecordDryRunNeverMovesTheTimelineWords — the validation evidence of
// the wired adapter lands as the dry_run_recorded event and the Timeline
// word of the version stays previewed (AC-8); a NIL adapter is the honest
// AG-4 refusal in words and NO evidence row exists.
func TestRecordDryRunNeverMovesTheTimelineWords(t *testing.T) {
	world := newPreviewLiveFixture()
	repo := newFakePreviewRepo()
	service := newPreviewService(world, repo, previewIdentity())
	built, err := service.SubmitProposal(context.Background(), happyProposal())
	if err != nil {
		t.Fatalf("подготовка обязана пройти: %v", err)
	}
	outcome, err := service.RecordDryRun(context.Background(), api.DryRunCommand{
		Actor: testActor, TenantID: testTenant, CaseID: previewCaseID,
		PreviewID: built.PreviewID, VersionNo: 1, Now: previewNow.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("dry-run подключённого адаптера обязан пройти: %v", err)
	}
	if outcome.Result != domain.DryRunResultValid {
		t.Fatalf("исход dry-run обязан нести слово словаря: %+v", outcome)
	}
	if evidence := repo.eventsOfKind(domain.EventDryRunRecorded); len(evidence) != 1 {
		t.Fatalf("evidence-событие обязано быть единственным: %d", len(evidence))
	}
	view, err := service.ReadPreviewView(context.Background(), testActor, testTenant, previewCaseID)
	if err != nil {
		t.Fatalf("чтение обязано жить: %v", err)
	}
	if view.Threads[0].Cards[0].Timeline != domain.TimelinePreviewed {
		t.Fatalf("Timeline не смещается dry-run: %q", view.Threads[0].Cards[0].Timeline)
	}
	if len(view.Threads[0].DryRunEvidence) != 1 || view.Threads[0].DryRunEvidence[0].Result != domain.DryRunResultValid {
		t.Fatalf("evidence обязан читаться на карточке: %+v", view.Threads[0].DryRunEvidence)
	}
	// The nil adapter: the honest refusal, no evidence.
	nilWorld := newPreviewLiveFixture()
	nilService := newPreviewService(nilWorld, repo, previewIdentity())
	nilService.dryRunner = nil
	_, err = nilService.RecordDryRun(context.Background(), api.DryRunCommand{
		Actor: testActor, TenantID: testTenant, CaseID: previewCaseID,
		PreviewID: built.PreviewID, VersionNo: 1, Now: previewNow.Add(2 * time.Minute),
	})
	var ruleErr domain.RuleError
	if !errors.As(err, &ruleErr) || !strings.Contains(ruleErr.Reason, domain.DryRunUnsupportedText) {
		t.Fatalf("nil-адаптер обязан отказать словами: %v", err)
	}
	if evidence := repo.eventsOfKind(domain.EventDryRunRecorded); len(evidence) != 1 {
		t.Fatalf("nil-адаптер не создаёт evidence: %d", len(evidence))
	}
}

// TestReadPreviewViewNamesTheStaleReasonByWord — the freshness derivation
// reads the LIVE sources at EVERY read and answers the exact reason word
// of the seven-word dictionary: a moved fact, a moved owner epoch, a
// moved capability evidence.
func TestReadPreviewViewNamesTheStaleReasonByWord(t *testing.T) {
	world := newPreviewLiveFixture()
	repo := newFakePreviewRepo()
	service := newPreviewService(world, repo, previewIdentity())
	built, err := service.SubmitProposal(context.Background(), happyProposal())
	if err != nil {
		t.Fatalf("подготовка обязана пройти: %v", err)
	}
	// The fresh build reads fresh at first.
	view, err := service.ReadPreviewView(context.Background(), testActor, testTenant, previewCaseID)
	if err != nil {
		t.Fatalf("чтение обязано жить: %v", err)
	}
	if view.Threads[0].Cards[0].FreshnessStale {
		t.Fatalf("только что построенный preview обязан читаться свежим")
	}
	reasons := []struct {
		name   string
		mutate func(world *previewLiveFixture)
		reason string
	}{
		{"факт изменился", func(w *previewLiveFixture) {
			w.factsView.Facts[0].Versions = append(w.factsView.Facts[0].Versions, casesDomain.FactVersion{FactID: previewFactID, VersionNo: 4})
		}, domain.FreshnessFactChanged},
		{"владелец сменился", func(w *previewLiveFixture) { w.caseView.Policy.OwnerEpoch = 8 }, domain.FreshnessOwnerEpochMoved},
		{"capability понизилась", func(w *previewLiveFixture) { w.capState = "failed" }, domain.FreshnessCapabilityEvidenceMoved},
		{"наблюдение сменилось", func(w *previewLiveFixture) {
			w.caseView.Observations.Items[0].Fields[0].Current = &casesDomain.Observation{ObservationID: previewObsID, ObservationNo: 3, MaterialKey: "price"}
		}, domain.FreshnessObservationChanged},
	}
	for _, probe := range reasons {
		t.Run(probe.name, func(t *testing.T) {
			mutated := newPreviewLiveFixture()
			probe.mutate(&mutated)
			mutatedReader := mutated.service(repo)
			card, err := readFirstCard(mutatedReader, built.PreviewID)
			if err != nil {
				t.Fatalf("чтение обязано жить: %v", err)
			}
			if !card.FreshnessStale || card.FreshnessReason != probe.reason {
				t.Fatalf("причина обязана быть точным словом %q: stale=%v reason=%q", probe.reason, card.FreshnessStale, card.FreshnessReason)
			}
			if !strings.Contains(card.FreshnessReasonText, ".") {
				t.Fatalf("текст причины обязан быть честным предложением: %q", card.FreshnessReasonText)
			}
		})
	}
}

// readFirstCard reads the first card of the built thread over the mutated
// world.
func readFirstCard(service *Service, previewID id.UUID) (api.PreviewCardView, error) {
	view, err := service.ReadPreviewView(context.Background(), testActor, testTenant, previewCaseID)
	if err != nil {
		return api.PreviewCardView{}, err
	}
	for _, thread := range view.Threads {
		if thread.PreviewID != previewID {
			continue
		}
		return thread.Cards[0], nil
	}
	return api.PreviewCardView{}, errors.New("поток не найден")
}

// TestPreviewRightsGateRefusesAuditOnlyMemberAndNonMember — the honest
// order: a non-member answers 404 BEFORE any material; a member with only
// the audit scope answers the 403 words of the rights gate.
func TestPreviewRightsGateRefusesAuditOnlyMemberAndNonMember(t *testing.T) {
	world := newPreviewLiveFixture()
	repo := newFakePreviewRepo()
	nonMember := newPreviewService(world, repo, scopeIdentity{member: false})
	if _, err := nonMember.SubmitProposal(context.Background(), happyProposal()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("не-участник обязан получить 404: %v", err)
	}
	auditOnly := newPreviewService(world, repo, scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Audit: true}})
	_, err := auditOnly.SubmitProposal(context.Background(), happyProposal())
	if !errors.Is(err, domain.ErrForbidden) || !strings.Contains(err.Error(), "review или configuration") {
		t.Fatalf("участник только с audit обязан получить слова шлюза: %v", err)
	}
	if _, err := auditOnly.ReadPreviewView(context.Background(), testActor, testTenant, previewCaseID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("чтение держит тот же шлюз: %v", err)
	}
	if repo.versionCount() != 0 {
		t.Fatalf("шлюз не пропускает материалов: %d", repo.versionCount())
	}
}

// TestPreviewContourRefusesUnwiredHonestly — the unwired contour is the
// honest error, never an assumed "ok" perimeter.
func TestPreviewContourRefusesUnwiredHonestly(t *testing.T) {
	service := New(previewIdentity(), acceptAllPacks{}, acceptAllCaps{}, fixtureDefinitions())
	_, err := service.SubmitProposal(context.Background(), happyProposal())
	if err == nil || !strings.Contains(err.Error(), "не сконфигурирован") {
		t.Fatalf("неподключённый контур обязан отказать честно: %v", err)
	}
}

// service wires the mutated world over the shared repository (the
// freshness probe helper).
func (w previewLiveFixture) service(repo *fakePreviewRepo) *Service {
	return newPreviewService(w, repo, previewIdentity())
}
