package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/action/api"
	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.5 step-02: the fake-repo unit matrix of the confirmation→
// authorization contour
// ---------------------------------------------------------------------------
//
// The counters pin the FROZEN gate order of the Action Gateway: the
// idempotency key gate fires BEFORE the pin resolve; THE REPLAY CHECK
// runs FIRST (a found operation answers the idempotent return whatever
// the consent's current state is); every refusal of the ladder happens
// BEFORE the ONE transaction and consumes NOTHING (the consumed counter
// never moves on a refusal — the by-name probes); the raced twins of one
// client key map to ONE operation (the honest replay return); the
// cross-case loser of the reservation reads the honest reference without
// consuming; the injected failure at the THIRD insert leaves NOTHING
// behind (the all-or-nothing transaction — AC-6); the transactional
// method takes ZERO ports (the signature probe).

var (
	authPreviewThreadID = id.MustParse("01994f64-6c00-7000-8000-0000000000c1")
	authForeignWorkID   = id.MustParse("01994f64-6c00-7000-8000-0000000000c9")
	authGoodKey         = "order-42.double-click"
	authSecondKey       = "order-42.webhook-repeat"
)

// ---------------------------------------------------------------------------
// The fake repositories (the store's semantics: the ladders re-derive,
// the raced twins answer the honest refusals, every method mutates the
// maps ONLY at the commit point)
// ---------------------------------------------------------------------------

// fakeConfirmationRepo is the in-memory consent ladder: the version-pin
// bijection (one confirmation row per preview version — the raced twins
// answer ErrStaleConfirmationVersion), the version ladder re-derivation
// and the all-or-nothing appends.
type fakeConfirmationRepo struct {
	reads         int
	appends       int
	eventAppends  int
	confirmations map[id.UUID][]domain.Confirmation
	events        []port.ConfirmationEventRow
	pinTaken      map[id.UUID]map[int64]bool
}

func newFakeConfirmationRepo() *fakeConfirmationRepo {
	return &fakeConfirmationRepo{
		confirmations: map[id.UUID][]domain.Confirmation{},
		pinTaken:      map[id.UUID]map[int64]bool{},
	}
}

func (f *fakeConfirmationRepo) versionsOf(thread id.UUID) []domain.Confirmation {
	return append([]domain.Confirmation(nil), f.confirmations[thread]...)
}

func (f *fakeConfirmationRepo) maxVersion(thread id.UUID) int64 {
	max := int64(0)
	for _, stored := range f.confirmations[thread] {
		if stored.VersionNo > max {
			max = stored.VersionNo
		}
	}
	return max
}

func (f *fakeConfirmationRepo) versionOf(thread id.UUID, versionNo int64) (domain.Confirmation, bool) {
	for _, stored := range f.confirmations[thread] {
		if stored.VersionNo == versionNo {
			return stored, true
		}
	}
	return domain.Confirmation{}, false
}

// consumedCount derives the consumption of one version read-time: the
// count of the consumed events (never a stored column).
func (f *fakeConfirmationRepo) consumedCount(thread id.UUID, versionNo int64) int {
	count := 0
	for _, row := range f.events {
		if row.ConfirmationID == thread && row.VersionNo == versionNo && row.Kind == domain.ConfirmationEventConsumed {
			count++
		}
	}
	return count
}

func (f *fakeConfirmationRepo) eventsOf(thread id.UUID, versionNo int64) []domain.ConfirmationEvent {
	out := []domain.ConfirmationEvent{}
	for _, row := range f.events {
		if row.ConfirmationID == thread && row.VersionNo == versionNo {
			out = append(out, domain.ConfirmationEvent{Kind: row.Kind, Reason: row.Reason, Details: row.Details, At: row.CreatedAt, ActorID: row.ActorID})
		}
	}
	return out
}

func (f *fakeConfirmationRepo) ReadPreviewConfirmations(_ context.Context, _ tenancy.Context, previewID id.UUID) ([]domain.Confirmation, error) {
	out := []domain.Confirmation{}
	for thread := range f.confirmations {
		for _, stored := range f.confirmations[thread] {
			if stored.PreviewID == previewID {
				out = append(out, stored)
			}
		}
	}
	return out, nil
}

func (f *fakeConfirmationRepo) ReadCaseConfirmations(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]domain.Confirmation, error) {
	out := []domain.Confirmation{}
	for thread := range f.confirmations {
		for _, stored := range f.confirmations[thread] {
			if stored.CaseID == caseID {
				out = append(out, stored)
			}
		}
	}
	return out, nil
}

func (f *fakeConfirmationRepo) ReadConfirmationVersion(_ context.Context, _ tenancy.Context, thread id.UUID, versionNo int64) (domain.Confirmation, error) {
	f.reads++
	stored, ok := f.versionOf(thread, versionNo)
	if !ok {
		return domain.Confirmation{}, domain.ErrNotFound
	}
	return stored, nil
}

func (f *fakeConfirmationRepo) ReadConfirmationVersionEvents(_ context.Context, _ tenancy.Context, thread id.UUID, versionNo int64) ([]domain.ConfirmationEvent, error) {
	return f.eventsOf(thread, versionNo), nil
}

func (f *fakeConfirmationRepo) ReadCaseConfirmationEvents(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]port.ConfirmationEventRow, error) {
	out := []port.ConfirmationEventRow{}
	for _, row := range f.events {
		if row.CaseID == caseID {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeConfirmationRepo) AppendConfirmationVersion(_ context.Context, p port.AppendConfirmationParams) error {
	thread := p.Confirmation.ConfirmationID
	// The version-pin bijection (the structural 1:1 stream mapping): no
	// preview version is ever confirmed twice — the raced twins of two
	// fresh threads AND the raced re-confirm twins of one thread answer
	// the honest structural refusal.
	taken, known := f.pinTaken[p.Confirmation.PreviewID]
	if !known {
		taken = map[int64]bool{}
		f.pinTaken[p.Confirmation.PreviewID] = taken
	}
	if taken[p.Confirmation.PreviewVersionNo] {
		return port.ErrStaleConfirmationVersion
	}
	// The version ladder re-derivation (the raced append refuses, never
	// merges).
	if f.maxVersion(thread)+1 != p.Confirmation.VersionNo {
		return port.ErrStaleConfirmationVersion
	}
	if (f.maxVersion(thread) == 0) != (p.PrevVersionNo == 0) {
		return port.ErrStaleConfirmationVersion
	}
	// ALL or NOTHING: nothing exists before the commit point.
	stagedEvents := []port.ConfirmationEventRow{{
		ConfirmationID: thread, VersionNo: p.Confirmation.VersionNo,
		CaseID: p.Confirmation.CaseID, EventID: p.ConfirmedEventID, EventNo: 1,
		Kind: domain.ConfirmationEventConfirmed, ActorID: p.Confirmation.ConfirmedBy, CreatedAt: p.Now,
	}}
	if p.PrevVersionNo > 0 {
		stagedEvents = append(stagedEvents, port.ConfirmationEventRow{
			ConfirmationID: thread, VersionNo: p.PrevVersionNo,
			CaseID: p.Confirmation.CaseID, EventID: p.SupersededEventID, EventNo: 1,
			Kind: domain.ConfirmationEventSuperseded, ActorID: p.Confirmation.ConfirmedBy, CreatedAt: p.Now,
		})
	}
	f.confirmations[thread] = append(f.confirmations[thread], p.Confirmation)
	taken[p.Confirmation.PreviewVersionNo] = true
	f.events = append(f.events, stagedEvents...)
	f.appends++
	return nil
}

func (f *fakeConfirmationRepo) AppendConfirmationEvent(_ context.Context, p port.AppendConfirmationEventParams) error {
	if _, ok := f.versionOf(p.ConfirmationID, p.VersionNo); !ok {
		return domain.ErrNotFound
	}
	f.events = append(f.events, port.ConfirmationEventRow{
		ConfirmationID: p.ConfirmationID, VersionNo: p.VersionNo,
		CaseID: p.CaseID, EventID: p.EventID, EventNo: int64(len(f.eventsOf(p.ConfirmationID, p.VersionNo)) + 1),
		Kind: p.Kind, Reason: p.Reason, Details: p.Details, ActorID: p.ActorID, CreatedAt: p.Now,
	})
	f.eventAppends++
	return nil
}

// fakeOperationRepo is the in-memory operation persistence: the pinned
// preview rows, the twin/reservation UNIQUE semantics of the two key
// spaces, the staged ALL-OR-NOTHING ReserveOperation (the injected
// failure at the Nth insert leaves NOTHING) and the raced consumption
// refusal under the "lock".
type fakeOperationRepo struct {
	reserveCalls   int
	findByKeyCalls int
	pinned         map[id.UUID]map[int64]domain.ActionPreview
	operations     []domain.Operation
	ledgers        map[id.UUID][]domain.OperationLedgerEntry
	outbox         map[id.UUID]port.OperationQueuedPayload
	reservations   map[string]id.UUID
	failAtInsert   int    // 1..5 — the injected failure BEFORE the Nth insert
	hideKey        string // the race-window simulation: the pre-tx find misses this key
	consents       *fakeConfirmationRepo
}

func newFakeOperationRepo(consents *fakeConfirmationRepo) *fakeOperationRepo {
	return &fakeOperationRepo{
		pinned:       map[id.UUID]map[int64]domain.ActionPreview{},
		ledgers:      map[id.UUID][]domain.OperationLedgerEntry{},
		outbox:       map[id.UUID]port.OperationQueuedPayload{},
		reservations: map[string]id.UUID{},
		consents:     consents,
	}
}

func (f *fakeOperationRepo) pinPreview(preview domain.ActionPreview) {
	versions, known := f.pinned[preview.PreviewID]
	if !known {
		versions = map[int64]domain.ActionPreview{}
		f.pinned[preview.PreviewID] = versions
	}
	versions[preview.VersionNo] = preview
}

func (f *fakeOperationRepo) operationCount() int { return len(f.operations) }

func (f *fakeOperationRepo) operationOfKey(key string) (domain.Operation, bool) {
	for _, op := range f.operations {
		if op.IdempotencyKey == key {
			return op, true
		}
	}
	return domain.Operation{}, false
}

func (f *fakeOperationRepo) ReadPinnedPreviewVersion(_ context.Context, _ tenancy.Context, previewID id.UUID, versionNo int64) (domain.ActionPreview, error) {
	if versions, known := f.pinned[previewID]; known {
		if row, ok := versions[versionNo]; ok {
			return row, nil
		}
	}
	return domain.ActionPreview{}, domain.ErrNotFound
}

func (f *fakeOperationRepo) FindOperationByIdempotencyKey(_ context.Context, _ tenancy.Context, key string) (domain.Operation, bool, error) {
	f.findByKeyCalls++
	if key == f.hideKey {
		return domain.Operation{}, false, nil
	}
	op, found := f.operationOfKey(key)
	return op, found, nil
}

func (f *fakeOperationRepo) FindOperationByIntent(_ context.Context, _ tenancy.Context, owningSystem, businessIntentKey string) (domain.Operation, bool, error) {
	operationID, ok := f.reservations[owningSystem+"\x1f"+businessIntentKey]
	if !ok {
		return domain.Operation{}, false, nil
	}
	for _, op := range f.operations {
		if op.OperationID == operationID {
			return op, true, nil
		}
	}
	return domain.Operation{}, false, nil
}

func (f *fakeOperationRepo) ReadCaseOperations(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]domain.Operation, error) {
	out := []domain.Operation{}
	for _, op := range f.operations {
		if op.CaseID == caseID {
			out = append(out, op)
		}
	}
	return out, nil
}

func (f *fakeOperationRepo) ReadOperationLedger(_ context.Context, _ tenancy.Context, operationID id.UUID) ([]domain.OperationLedgerEntry, error) {
	return append([]domain.OperationLedgerEntry(nil), f.ledgers[operationID]...), nil
}

func (f *fakeOperationRepo) ReadOutboxQueued(_ context.Context, _ tenancy.Context, operationID id.UUID) (bool, error) {
	_, queued := f.outbox[operationID]
	return queued, nil
}

// ReserveOperation mirrors the store's ONE transaction: the consumed
// re-selection under the lock, the allowance check, then the staged
// inserts in the frozen order (consumed → reservation → operation →
// ledger → outbox) — NOTHING exists before the commit point; the raced
// UNIQUE violations map onto the two internal signals; the injected
// failure at the Nth insert proves the all-or-nothing rollback.
func (f *fakeOperationRepo) ReserveOperation(_ context.Context, p port.ReserveOperationParams) error {
	f.reserveCalls++
	consumed := f.consents.consumedCount(p.Operation.ConfirmationID, p.Operation.ConfirmationVersionNo)
	allowance, err := domain.DeriveConsumptionAllowance(p.ConsumptionRule)
	if err != nil {
		return err
	}
	if allowance.Bounded && consumed >= allowance.Limit {
		return port.ErrConfirmationExhausted
	}
	reservationKey := p.OutboxPayload.OwningSystem + "\x1f" + p.Operation.BusinessIntentKey
	// INSERT 1/5 — the consumed event.
	if f.failAtInsert == 1 {
		return fmt.Errorf("инжект сбоя: вставка 1 (consumed)")
	}
	// INSERT 2/5 — the cross-case reservation.
	if _, exists := f.reservations[reservationKey]; exists {
		return port.ErrReservationConflict
	}
	if f.failAtInsert == 2 {
		return fmt.Errorf("инжект сбоя: вставка 2 (reservation)")
	}
	// INSERT 3/5 — the immutable operation (the twin of one client key).
	if _, twin := f.operationOfKey(p.Operation.IdempotencyKey); twin {
		return port.ErrIdempotencyTwin
	}
	if f.failAtInsert == 3 {
		return fmt.Errorf("инжект сбоя: вставка 3 (operation)")
	}
	// INSERT 4/5 — the ledger authorized entry.
	if f.failAtInsert == 4 {
		return fmt.Errorf("инжект сбоя: вставка 4 (ledger)")
	}
	// INSERT 5/5 — the outbox operation.queued record.
	if f.failAtInsert == 5 {
		return fmt.Errorf("инжект сбоя: вставка 5 (outbox)")
	}
	// COMMIT — nothing existed before this point.
	f.consents.events = append(f.consents.events, port.ConfirmationEventRow{
		ConfirmationID: p.Operation.ConfirmationID,
		VersionNo:      p.Operation.ConfirmationVersionNo, CaseID: p.Operation.CaseID,
		EventID: p.ConsumedEventID, EventNo: int64(len(f.consents.eventsOf(p.Operation.ConfirmationID, p.Operation.ConfirmationVersionNo)) + 1),
		Kind: domain.ConfirmationEventConsumed, ActorID: p.ActorID, CreatedAt: p.Now,
	})
	f.reservations[reservationKey] = p.Operation.OperationID
	f.operations = append(f.operations, p.Operation)
	f.ledgers[p.Operation.OperationID] = append(f.ledgers[p.Operation.OperationID], p.LedgerEntry)
	f.outbox[p.Operation.OperationID] = p.OutboxPayload
	return nil
}

// liveMembers serves the actor's membership set through the LIVE world
// pointer (the per-test role mutations propagate — the gate-order
// probes).
type liveMembers struct{ world *previewLiveFixture }

func (f liveMembers) MembersFacts(_ context.Context, _ identityDomain.Principal, _ id.UUID) ([]identityDomain.Membership, error) {
	principal := testActor.ID
	return []identityDomain.Membership{{
		TenantID: testTenant, PrincipalID: &principal, Email: "unit@staffy.test",
		Roles: f.world.roles, Scopes: []identityDomain.Scope{identityDomain.Review},
		Status: "active", Version: 1,
	}}, nil
}

// liveCaseMaterial serves the case material through a LIVE world pointer
// (the per-test mutations of the world propagate) and counts the reads
// (the gate-order pins).
type liveCaseMaterial struct {
	world      *previewLiveFixture
	caseCalls  int
	factsCalls int
}

func (f *liveCaseMaterial) CaseView(context.Context, identityDomain.Principal, id.UUID, id.UUID) (casesAPI.CaseView, error) {
	f.caseCalls++
	return f.world.caseView, nil
}

func (f *liveCaseMaterial) FactsView(context.Context, identityDomain.Principal, id.UUID, id.UUID) (casesAPI.FactsView, error) {
	f.factsCalls++
	return f.world.factsView, nil
}

// ---------------------------------------------------------------------------
// The authorization fixture
// ---------------------------------------------------------------------------

// authorizationFixture is the mutable world of one authorization test:
// the 3-4 live fixture (with the ownership terms the consent pins), the
// pinned preview version row (the digest computed with the fake key —
// the build imitation), the two fake repositories and the wired service.
type authorizationFixture struct {
	world    previewLiveFixture
	def      domain.ActionDefinition
	defs     []domain.ActionDefinition
	preview  domain.ActionPreview
	previews *fakePreviewRepo
	consents *fakeConfirmationRepo
	ops      *fakeOperationRepo
	material *liveCaseMaterial
	reader   *fakeReader
	service  *Service
	now      time.Time
}

// newAuthorizationFixture wires the full test composition root over the
// passing world (the confirm command resolves everything green).
func newAuthorizationFixture(t *testing.T) *authorizationFixture {
	t.Helper()
	return newAuthorizationFixtureWithDefs(t, nil)
}

// newAuthorizationFixtureWithDefs wires the root over a custom definition
// dictionary (the allowed_uncertain tolerance probe) — nil serves the
// standard fixture dictionary.
func newAuthorizationFixtureWithDefs(t *testing.T, defs []domain.ActionDefinition) *authorizationFixture {
	t.Helper()
	f := &authorizationFixture{
		world: newPreviewLiveFixture(),
		defs:  defs,
		now:   time.Now().UTC().Truncate(time.Microsecond),
	}
	if defs == nil {
		f.defs = fixtureDefinitions()
	}
	f.def = f.defs[0]
	// The ownership terms the consent pins (the response-owner triple of
	// the case at the confirmation moment).
	f.world.caseView.Ownership = casesAPI.OwnershipView{Terms: []casesDomain.OwnershipTerm{{
		TenantID: testTenant, CaseID: previewCaseID,
		TermID: id.MustParse("01994f64-6c00-7000-8000-0000000000c2"),
		Epoch:  1, OwnerKind: casesDomain.OwnerKindHuman, OwnerRef: testActor.ID,
	}}}
	f.previews = newFakePreviewRepo()
	f.consents = newFakeConfirmationRepo()
	f.ops = newFakeOperationRepo(f.consents)
	f.material = &liveCaseMaterial{world: &f.world}
	f.reader = f.world.reader
	f.preview = f.buildPreview()
	f.ops.pinPreview(f.preview)
	f.service = New(previewIdentity(), acceptAllPacks{}, acceptAllCaps{}, f.defs).
		WithPreviews(f.previews, f.world.control, f.material,
			liveMembers{world: &f.world}, fakeEntitlement{status: f.world.admission},
			fakeReadiness{stage: f.world.stage, instances: f.world.instances},
			fakeConnectors{state: f.world.capState, contractVersion: f.world.contract},
			fakeRouteFacts{profileVersion: f.world.profile}, &fakeIDs{}).
		WithPreviewReader(f.reader).
		WithAuthorization(f.consents, f.ops)
	return f
}

// pinPreviewVersion2 lands the EDIT of the pinned thread: preview version
// 2 with its own digest (the 2-9/3-4 append-only version pattern — the
// re-confirmation pins the NEW preview version, the old row's bytes never
// move).
func (f *authorizationFixture) pinPreviewVersion2() {
	edited := f.preview
	edited.VersionNo = 2
	edited.CreatedAt = previewNow.Add(time.Minute)
	edited.Digest = domain.PreviewDigest(f.previews.key, edited.PreviewID, edited.VersionNo, domain.PreviewCanonicalBytes(edited))
	f.ops.pinPreview(edited)
}

// confirmVersion2 re-confirms the thread over the edited preview version
// (the same thread identity, version+1, the new preview pin).
func (f *authorizationFixture) confirmVersion2(t *testing.T, thread id.UUID) int64 {
	t.Helper()
	f.pinPreviewVersion2()
	command := f.confirmCommand()
	command.PreviewVersionNo = 2
	outcome, err := f.service.ConfirmPreviewVersion(context.Background(), command)
	if err != nil {
		t.Fatalf("повторное подтверждение обязано создавать версию 2 того же потока: %v", err)
	}
	if outcome.ConfirmationID != thread || outcome.VersionNo != 2 {
		t.Fatalf("версия 2 обязана жить в том же потоке: %+v", outcome)
	}
	return outcome.VersionNo
}

// buildPreview imitates the build of the pinned preview version: the
// fixture's live pins (the freshness derivation sees NO reasons) with
// the keyed digest over the canonical bytes (the same per-tenant key the
// gateway re-reads).
func (f *authorizationFixture) buildPreview() domain.ActionPreview {
	preview := domain.ActionPreview{
		TenantID:               testTenant,
		PreviewID:              authPreviewThreadID,
		VersionNo:              1,
		DefinitionID:           f.def.ID,
		DefinitionVersion:      f.def.Version,
		CaseID:                 previewCaseID,
		WorkItemID:             previewWorkID,
		Target:                 domain.ProposalTarget{Kind: domain.TargetCatalogItem, Ref: previewTargetID},
		TypedInput:             map[string]string{"product_ref": "SKU-42"},
		SupportingFacts:        []domain.VersionPin{{Ref: previewFactID, Version: 3}},
		SupportingObservations: []domain.VersionPin{{Ref: previewObsID, Version: 2}},
		Snapshot: []domain.MaterialSnapshotRow{
			{MaterialField: "price", Status: domain.SnapshotFresh, ExternalID: "SKU-42", ExternalVersion: "7", ObservedAt: previewNow, EvidenceRef: "catalog://sku-42"},
			{MaterialField: "availability", Status: domain.SnapshotFresh, ExternalID: "SKU-42", ExternalVersion: "7", ObservedAt: previewNow, EvidenceRef: "catalog://sku-42"},
		},
		Effects:               domain.DerivePreviewEffects(f.def),
		Partition:             domain.ExecutionPartition{Partition: domain.PartitionStaffy},
		OwnerEpoch:            1,
		ControlGeneration:     3,
		ProfileVersion:        7,
		PolicyOutcomeNo:       1,
		CapabilityDeclVersion: "1.1.0",
		AuthorID:              testActor.ID,
		CreatedAt:             previewNow,
	}
	preview.Digest = domain.PreviewDigest(f.previews.key, preview.PreviewID, preview.VersionNo, domain.PreviewCanonicalBytes(preview))
	return preview
}

// confirmCommand is the passing consent of the pinned preview version by
// the step-up-verified human.
func (f *authorizationFixture) confirmCommand() api.ConfirmPreviewVersionCommand {
	return api.ConfirmPreviewVersionCommand{
		Actor:            stepUpActor(),
		TenantID:         testTenant,
		CaseID:           previewCaseID,
		PreviewID:        f.preview.PreviewID,
		PreviewVersionNo: f.preview.VersionNo,
		LifetimeSeconds:  600,
		Now:              f.now,
	}
}

// authorizeCommand is the machine authorization of the consent version by
// the given client key.
func (f *authorizationFixture) authorizeCommand(thread id.UUID, versionNo int64, key string) api.AuthorizeOperationCommand {
	return api.AuthorizeOperationCommand{
		Actor:                 testActor,
		TenantID:              testTenant,
		CaseID:                previewCaseID,
		ConfirmationID:        thread,
		ConfirmationVersionNo: versionNo,
		IdempotencyKey:        key,
		Now:                   f.now,
	}
}

// confirm runs the passing consent and answers the thread identity.
func (f *authorizationFixture) confirm(t *testing.T) (id.UUID, int64) {
	t.Helper()
	outcome, err := f.service.ConfirmPreviewVersion(context.Background(), f.confirmCommand())
	if err != nil {
		t.Fatalf("подтверждение обязано проходить на свежем готовом preview: %v", err)
	}
	if outcome.State != domain.ConfirmationValid {
		t.Fatalf("состояние свежего подтверждения обязано быть valid: %+v", outcome)
	}
	return outcome.ConfirmationID, outcome.VersionNo
}

// authorize runs the full authorization of the consent version.
func (f *authorizationFixture) authorize(t *testing.T, thread id.UUID, versionNo int64, key string) api.AuthorizationOutcome {
	t.Helper()
	outcome, err := f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, versionNo, key))
	if err != nil {
		t.Fatalf("авторизация обязана проходить на валидном согласии: %v", err)
	}
	return outcome
}

// stepUpActor is the step-up-verified session of the human acts.
func stepUpActor() identityDomain.Principal {
	return identityDomain.Principal{ID: testActor.ID, StepUpVerified: true}
}

// ---------------------------------------------------------------------------
// The confirm command
// ---------------------------------------------------------------------------

// TestConfirmPreviewVersionAppendsConsent — the happy path: the consent
// copies the LINKED MATERIAL of the pinned version (the digest, the
// snapshot, the effects — «согласие несёт ЧТО согласовано»), pins the
// owner triple and the release snapshot, derives the server-truth
// valid_until with the clamp and lands the confirmed event; NO operation
// side exists.
func TestConfirmPreviewVersionAppendsConsent(t *testing.T) {
	f := newAuthorizationFixture(t)
	outcome, err := f.service.ConfirmPreviewVersion(context.Background(), f.confirmCommand())
	if err != nil {
		t.Fatalf("прохождение согласия обязано создавать версию Confirmation: %v", err)
	}
	if outcome.VersionNo != 1 || !outcome.ConfirmationID.Valid() {
		t.Fatalf("исход обязан нести стабильную идентичность и версию 1: %+v", outcome)
	}
	if outcome.State != domain.ConfirmationValid || outcome.StateText == "" {
		t.Fatalf("состояние обязано быть valid с честным текстом: %+v", outcome)
	}
	wantUntil := domain.DeriveValidUntil(f.now, 600)
	if !outcome.ValidUntil.Equal(wantUntil) {
		t.Fatalf("valid_until обязана быть серверной истиной с клампом: %v ≠ %v", outcome.ValidUntil, wantUntil)
	}
	if outcome.ConsumptionText != "Потреблено 0 из 1." {
		t.Fatalf("слова потребления свежего single_use согласия обязаны быть честными: %q", outcome.ConsumptionText)
	}
	stored, ok := f.consents.versionOf(outcome.ConfirmationID, 1)
	if !ok {
		t.Fatal("версия согласия обязана быть сохранена")
	}
	if stored.PreviewDigest != f.preview.Digest {
		t.Fatalf("копия digest обязана совпадать байт-в-байт: %s ≠ %s", stored.PreviewDigest, f.preview.Digest)
	}
	if len(stored.Snapshot) != 2 || len(stored.Effects) != len(f.preview.Effects) {
		t.Fatalf("копии материала обязаны нести snapshot и конечный набор эффектов: %+v", stored)
	}
	if stored.OwnerKind != domain.OwnerHuman || stored.OwnerRef != testActor.ID || stored.OwnerEpoch != 1 {
		t.Fatalf("тройка владельца обязана быть снимком живого чтения: %+v", stored)
	}
	if stored.ReleaseDecisionNo != 1 || stored.ProfileVersion != 7 {
		t.Fatalf("снимок релиза обязан пинить решение и версию профиля: %+v", stored)
	}
	events := f.consents.eventsOf(outcome.ConfirmationID, 1)
	if len(events) != 1 || events[0].Kind != domain.ConfirmationEventConfirmed {
		t.Fatalf("история обязана нести ровно событие confirmed: %+v", events)
	}
	if f.ops.operationCount() != 0 || f.ops.reserveCalls != 0 {
		t.Fatal("согласие не создаёт операций и не вызывает транзакцию резервирования")
	}
}

// TestConfirmRequiresLiveStepUp — the consent is the HUMAN act: an
// unverified session never confirms (the Approve precedent of 3-3), the
// repository stays untouched.
func TestConfirmRequiresLiveStepUp(t *testing.T) {
	f := newAuthorizationFixture(t)
	command := f.confirmCommand()
	command.Actor = testActor // без StepUpVerified
	_, err := f.service.ConfirmPreviewVersion(context.Background(), command)
	if !errors.Is(err, identityDomain.ErrStepUpRequired) {
		t.Fatalf("сессия без step-up обязана получать ErrStepUpRequired: %v", err)
	}
	if f.consents.appends != 0 || f.consents.eventAppends != 0 {
		t.Fatal("отказ step-up не оставляет следов в репозитории")
	}
}

// TestConfirmRefusesNotRequiredDefinition — a not_required query
// definition honestly never enters the consent contour (its answers live
// in the observations of 2-7 — the honest boundary word).
func TestConfirmRefusesNotRequiredDefinition(t *testing.T) {
	f := newAuthorizationFixture(t)
	f.preview.DefinitionID = "catalog.check-availability"
	f.preview.DefinitionVersion = "1.0.0"
	f.preview.Digest = domain.PreviewDigest(f.previews.key, f.preview.PreviewID, f.preview.VersionNo, domain.PreviewCanonicalBytes(f.preview))
	f.ops.pinPreview(f.preview)
	_, err := f.service.ConfirmPreviewVersion(context.Background(), f.confirmCommand())
	var ruleErr domain.RuleError
	if !errors.As(err, &ruleErr) {
		t.Fatalf("not_required определение обязано получать RuleError: %v", err)
	}
	if !errors.Is(err, domain.ErrInvalid) || ruleErr.Reason != domain.ConfirmationNotRequiredText {
		t.Fatalf("причина обязана быть честным словом границы: %+v", ruleErr)
	}
	if f.consents.appends != 0 {
		t.Fatal("not_required определение не создаёт согласий")
	}
}

// TestConfirmRefusesStalePreview — the consent of dead material is
// meaningless: the freshness derivation reused from 3-4 refuses with the
// exact reason word, the repository stays untouched.
func TestConfirmRefusesStalePreview(t *testing.T) {
	f := newAuthorizationFixture(t)
	f.world.factsView.Facts[0].Versions[0].VersionNo = 4 // факт двинулся
	_, err := f.service.ConfirmPreviewVersion(context.Background(), f.confirmCommand())
	var ruleErr domain.RuleError
	if !errors.As(err, &ruleErr) {
		t.Fatalf("устаревший preview обязан получать RuleError честной причины: %v", err)
	}
	if ruleErr.Reason != domain.FreshnessReasonText(domain.FreshnessFactChanged) {
		t.Fatalf("причина обязана быть словом словаря 3-4: %+v", ruleErr)
	}
	if f.consents.appends != 0 {
		t.Fatal("устаревший материал не подтверждается")
	}
}

// TestConfirmRefusesBlockedMaterial — the blocked material is never
// consented: the readiness gate reads the pinned preview's OWN snapshot
// (readiness = истина снапшота сборки — урок 3-4); a blocked build refuses
// honestly, nothing is appended.
func TestConfirmRefusesBlockedMaterial(t *testing.T) {
	f := newAuthorizationFixture(t)
	blocked := f.preview
	blocked.Snapshot = []domain.MaterialSnapshotRow{ // availability не построена — blocked_material
		{MaterialField: "price", Status: domain.SnapshotFresh},
	}
	blocked.Digest = domain.PreviewDigest(f.previews.key, blocked.PreviewID, blocked.VersionNo, domain.PreviewCanonicalBytes(blocked))
	f.ops.pinPreview(blocked)
	_, err := f.service.ConfirmPreviewVersion(context.Background(), f.confirmCommand())
	var ruleErr domain.RuleError
	if !errors.As(err, &ruleErr) {
		t.Fatalf("заблокированный материал обязан получать RuleError: %v", err)
	}
	if ruleErr.Reason != domain.PreviewReadinessText(domain.PreviewBlockedMaterial) {
		t.Fatalf("причина обязана быть словом blocked_material: %+v", ruleErr)
	}
	if f.consents.appends != 0 {
		t.Fatal("заблокированный материал не подтверждается")
	}
}

// TestConfirmRacedTwinIsStructuralRefusal — the version-pin bijection is
// structural: a preview version is never confirmed twice (the raced twins
// of two fresh threads and the raced re-confirms of one thread both
// answer the honest stale refusal).
func TestConfirmRacedTwinIsStructuralRefusal(t *testing.T) {
	f := newAuthorizationFixture(t)
	racedScope, err := tenancy.New(testActor.ID, testTenant)
	if err != nil {
		t.Fatalf("область гонки обязана собираться: %v", err)
	}
	if err := f.consents.AppendConfirmationVersion(context.Background(), port.AppendConfirmationParams{
		Scope: racedScope,
		Confirmation: domain.Confirmation{
			TenantID: testTenant, ConfirmationID: id.MustParse("01994f64-6c00-7000-8000-0000000000c3"),
			VersionNo: 1, PreviewID: f.preview.PreviewID, PreviewVersionNo: f.preview.VersionNo,
			CaseID: previewCaseID, WorkItemID: previewWorkID, OwnerKind: domain.OwnerHuman, OwnerRef: testActor.ID,
			Snapshot: []domain.MaterialSnapshotRow{}, Effects: []domain.PreviewEffect{{}}, ValidUntil: f.now.Add(time.Hour),
		},
		PrevVersionNo:    0,
		ConfirmedEventID: id.MustParse("01994f64-6c00-7000-8000-0000000000c4"),
		Now:              f.now,
	}); err != nil {
		t.Fatalf("первое подтверждение обязано проходить: %v", err)
	}
	_, confirmErr := f.service.ConfirmPreviewVersion(context.Background(), f.confirmCommand())
	if !errors.Is(confirmErr, port.ErrStaleConfirmationVersion) {
		t.Fatalf("гонка близнецов одного пина preview обязана быть структурным отказом: %v", confirmErr)
	}
	if f.consents.appends != 1 {
		t.Fatalf("второй строки быть не обязано: %d", f.consents.appends)
	}
}

// ---------------------------------------------------------------------------
// The revoke command
// ---------------------------------------------------------------------------

// TestRevokeConfirmationAppendsEvent — the revocation is the human act
// with the step-up; the event lands on the exact version; the past
// consumption stays the audit fact (the note speaks it).
func TestRevokeConfirmationAppendsEvent(t *testing.T) {
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	outcome, err := f.service.RevokeConfirmation(context.Background(), api.RevokeConfirmationCommand{
		Actor: stepUpActor(), TenantID: testTenant, CaseID: previewCaseID,
		ConfirmationID: thread, VersionNo: version, Now: f.now,
	})
	if err != nil {
		t.Fatalf("отзыв валидного согласия обязан проходить: %v", err)
	}
	if outcome.State != domain.ConfirmationRevoked || outcome.RevocationNote == "" {
		t.Fatalf("исход отзыва обязан говорить revoked и заметку аудита: %+v", outcome)
	}
	events := f.consents.eventsOf(thread, version)
	foundRevoked := false
	for _, event := range events {
		if event.Kind == domain.ConfirmationEventRevoked {
			foundRevoked = true
		}
	}
	if !foundRevoked {
		t.Fatalf("событие revoked обязано попасть в историю: %+v", events)
	}
	// Отзыванное согласие больше не авторизуется (слово лестницы).
	_, err = f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, version, authGoodKey))
	var rejected port.ConfirmationRejectedError
	if !errors.As(err, &rejected) || rejected.State != domain.ConfirmationRevoked {
		t.Fatalf("отозванное согласие обязано отвергаться словом revoked: %v", err)
	}
}

// TestRevokeRequiresLiveStepUp — the revocation is the human act: an
// unverified session never revokes.
func TestRevokeRequiresLiveStepUp(t *testing.T) {
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	_, err := f.service.RevokeConfirmation(context.Background(), api.RevokeConfirmationCommand{
		Actor: testActor, TenantID: testTenant, CaseID: previewCaseID,
		ConfirmationID: thread, VersionNo: version, Now: f.now,
	})
	if !errors.Is(err, identityDomain.ErrStepUpRequired) {
		t.Fatalf("отзыв без step-up обязан получать ErrStepUpRequired: %v", err)
	}
	if f.consents.eventAppends != 0 {
		t.Fatal("отказ step-up не оставляет событий")
	}
}

// ---------------------------------------------------------------------------
// The authorize command: the ordered gateway
// ---------------------------------------------------------------------------

// TestAuthorizeOperationFullChain — the happy path: the ONE transaction
// lands consumed + reservation + operation + ledger authorized + outbox
// operation.queued; the card speaks authorized/queued with the honest
// boundary texts; the consumption is exactly +1; the replay flags stay
// false.
func TestAuthorizeOperationFullChain(t *testing.T) {
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	outcome := f.authorize(t, thread, version, authGoodKey)
	if outcome.Replay || outcome.ReservationReference {
		t.Fatalf("первая авторизация не является ни повтором, ни ссылкой: %+v", outcome)
	}
	card := outcome.Operation
	if card.State != "authorized/queued" {
		t.Fatalf("вид обязан говорить authorized/queued (никогда executed): %q", card.State)
	}
	if card.ExternalIDAbsentText != domain.OperationExternalIDAbsentText || card.DispatchAbsentText != domain.OperationDispatchAbsentText {
		t.Fatalf("карточка обязана нести честные тексты границ: %+v", card)
	}
	if len(card.Ledger) != 1 || card.Ledger[0].Kind != domain.LedgerEntryAuthorized || card.Ledger[0].EntryNo != 1 {
		t.Fatalf("ledger обязан нести ровно запись authorized №1: %+v", card.Ledger)
	}
	if card.IdempotencyKey != authGoodKey || len(card.BusinessIntentKey) != 64 || len(card.MaterialDigest) != 64 {
		t.Fatalf("карточка обязана нести оба ключа-отождествления: %+v", card)
	}
	if f.consents.consumedCount(thread, version) != 1 {
		t.Fatalf("потребление обязано быть ровно +1: %d", f.consents.consumedCount(thread, version))
	}
	if f.ops.operationCount() != 1 || f.ops.reserveCalls != 1 {
		t.Fatalf("обязана существовать ровно одна операция одного вызова транзакции: %d/%d", f.ops.operationCount(), f.ops.reserveCalls)
	}
	queuedScope, err := tenancy.New(testActor.ID, testTenant)
	if err != nil {
		t.Fatalf("область чтения обязана собираться: %v", err)
	}
	queued, _ := f.ops.ReadOutboxQueued(context.Background(), queuedScope, card.OperationID)
	if !queued {
		t.Fatal("запись operation.queued обязана существовать")
	}
	payload := f.ops.outbox[card.OperationID]
	if payload.EventType != "operation.queued" || payload.OwningSystem != "catalog" || payload.IdempotencyKey != authGoodKey {
		t.Fatalf("SAFE payload обязан нести ids/пины/слова/два ключа: %+v", payload)
	}
	if f.reader.calls != 1 {
		t.Fatalf("ридер обязан вызываться ровно один раз (последняя проверка материала): %d", f.reader.calls)
	}
}

// TestAuthorizeKeyGateFiresBeforeThePinResolve — the machine key gate
// (charset/length) refuses BEFORE anything is read: the pin resolve
// counter stays at zero.
func TestAuthorizeKeyGateFiresBeforeThePinResolve(t *testing.T) {
	f := newAuthorizationFixture(t)
	command := f.authorizeCommand(id.UUID(""), 1, "short")
	_, err := f.service.AuthorizeOperation(context.Background(), command)
	var ruleErr domain.RuleError
	if !errors.As(err, &ruleErr) {
		t.Fatalf("ключ вне словаря обязан получать RuleError: %v", err)
	}
	if f.consents.reads != 0 {
		t.Fatalf("ключ-гейт обязан срабатывать ДО резолвции пина: %d", f.consents.reads)
	}
	if f.ops.findByKeyCalls != 0 {
		t.Fatalf("ключ-гейт обязан срабатывать ДО replay-проверки: %d", f.ops.findByKeyCalls)
	}
}

// TestAuthorizeReplayReturnsOriginalOperation — the double-click (the
// same key, the same pins) answers the idempotent return of the ORIGINAL
// operation: the consumption NEVER repeats, the outbox stays ONE, the
// second operation does not exist, the material reader is NOT called
// again (the replay check runs FIRST — before the ladder, before the
// live reads).
func TestAuthorizeReplayReturnsOriginalOperation(t *testing.T) {
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	first := f.authorize(t, thread, version, authGoodKey)
	readerCallsAfterFirst := f.reader.calls
	outcome := f.authorize(t, thread, version, authGoodKey)
	if !outcome.Replay {
		t.Fatalf("повтор с тем же ключом и тем же материалом обязан быть replay-возвратом: %+v", outcome)
	}
	if outcome.OutcomeText != domain.IdempotentReplayText {
		t.Fatalf("текст повтора обязан быть фиксированной честной фразой: %q", outcome.OutcomeText)
	}
	if outcome.Operation.OperationID != first.Operation.OperationID {
		t.Fatalf("повтор обязан вернуть ПЕРВОНАЧАЛЬНУЮ операцию: %s ≠ %s", outcome.Operation.OperationID, first.Operation.OperationID)
	}
	if f.consents.consumedCount(thread, version) != 1 {
		t.Fatalf("повтор НЕ потребляет: %d", f.consents.consumedCount(thread, version))
	}
	if f.ops.operationCount() != 1 || f.ops.reserveCalls != 1 {
		t.Fatalf("вторая операции/транзакции не существует: %d/%d", f.ops.operationCount(), f.ops.reserveCalls)
	}
	if f.reader.calls != readerCallsAfterFirst {
		t.Fatalf("replay-проверка идёт ДО живых чтений — ридер не вызывается повторно: %d", f.reader.calls)
	}
}

// TestAuthorizeConflictIsVisibleException — the same client key with a
// DIFFERENT material (the re-confirmed version after the preview edit)
// answers the honest 409 conflict: the authorization_conflict event is
// visible, the ORIGINAL operation stays unchanged, the consumption stays
// at one.
func TestAuthorizeConflictIsVisibleException(t *testing.T) {
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	first := f.authorize(t, thread, version, authGoodKey)
	// Повторное подтверждение ПОСЛЕ РЕДАКТИРОВАНИЯ preview: версия 2 того
	// же потока пинит НОВУЮ версию preview (старая — superseded).
	f.confirmVersion2(t, thread)
	_, err := f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, 2, authGoodKey))
	if !errors.Is(err, port.ErrIdempotencyConflict) {
		t.Fatalf("тот же ключ с другим материалом обязан получать 409-конфликт: %v", err)
	}
	if !errors.Is(err, port.ErrIdempotencyConflict) || errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("конфликт идемпотентности — не словарная 400-семья: %v", err)
	}
	conflictEvents := 0
	for _, row := range f.consents.events {
		if row.Kind == domain.ConfirmationEventAuthorizationConflict {
			conflictEvents++
		}
	}
	if conflictEvents != 1 {
		t.Fatalf("видимое исключение конфликта обязано попасть в историю ровно один раз: %d", conflictEvents)
	}
	stored, _ := f.consents.versionOf(thread, 1)
	_ = stored
	if f.consents.consumedCount(thread, 1) != 1 || f.consents.consumedCount(thread, 2) != 0 {
		t.Fatalf("конфликт не потребляет: v1=%d v2=%d", f.consents.consumedCount(thread, 1), f.consents.consumedCount(thread, 2))
	}
	if f.ops.operationCount() != 1 {
		t.Fatalf("исходная Operation неизменна, второй не существует: %d", f.ops.operationCount())
	}
	if still, _ := f.ops.operationOfKey(authGoodKey); still.OperationID != first.Operation.OperationID {
		t.Fatal("операция того же ключа обязана остаться первоначальной")
	}
}

// TestAuthorizeRejectionLadderByName — every ladder word refuses BY NAME
// without consuming: superseded (a newer version exists), revoked (the
// human event), expired (the server clock passed valid_until), exhausted
// (the single_use consent consumed by the first authorization); every
// refusal appends the visible authorization_blocked event and NEVER
// calls the reserve transaction.
func TestAuthorizeRejectionLadderByName(t *testing.T) {
	blockedTotal := 0
	countBlocked := func(repo *fakeConfirmationRepo) int {
		count := 0
		for _, row := range repo.events {
			if row.Kind == domain.ConfirmationEventAuthorizationBlocked {
				count++
			}
		}
		return count
	}
	// superseded — новая версия потока гасит старую.
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	f.confirmVersion2(t, thread)
	_, err := f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, version, authGoodKey))
	var rejected port.ConfirmationRejectedError
	if !errors.As(err, &rejected) || rejected.State != domain.ConfirmationSuperseded {
		t.Fatalf("заменённое согласие обязано отвергаться словом superseded: %v", err)
	}
	if f.ops.reserveCalls != 0 || f.consents.consumedCount(thread, version) != 0 {
		t.Fatal("отвержение НЕ потребляет и НЕ вызывает транзакцию")
	}
	blockedTotal += countBlocked(f.consents)
	if countBlocked(f.consents) != 1 {
		t.Fatalf("superseded-отвержение обязано оставлять видимое событие: %d", countBlocked(f.consents))
	}

	// revoked — отзыв гасит будущие авторизации.
	f = newAuthorizationFixture(t)
	thread, version = f.confirm(t)
	if _, err := f.service.RevokeConfirmation(context.Background(), api.RevokeConfirmationCommand{
		Actor: stepUpActor(), TenantID: testTenant, CaseID: previewCaseID,
		ConfirmationID: thread, VersionNo: version, Now: f.now,
	}); err != nil {
		t.Fatalf("отзыв обязан проходить: %v", err)
	}
	_, err = f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, version, authGoodKey))
	if !errors.As(err, &rejected) || rejected.State != domain.ConfirmationRevoked {
		t.Fatalf("отозванное согласие обязано отвергаться словом revoked: %v", err)
	}
	blockedTotal += countBlocked(f.consents)
	if countBlocked(f.consents) != 1 {
		t.Fatalf("revoked-отвержение обязано оставлять видимое событие: %d", countBlocked(f.consents))
	}

	// expired — серверные часы прошли valid_until.
	f = newAuthorizationFixture(t)
	thread, version = f.confirm(t)
	late := f.authorizeCommand(thread, version, authGoodKey)
	late.Now = f.now.Add(2 * time.Hour)
	_, err = f.service.AuthorizeOperation(context.Background(), late)
	if !errors.As(err, &rejected) || rejected.State != domain.ConfirmationExpired {
		t.Fatalf("истёкшее согласие обязано отвергаться словом expired: %v", err)
	}
	blockedTotal += countBlocked(f.consents)
	if countBlocked(f.consents) != 1 {
		t.Fatalf("expired-отвержение обязано оставлять видимое событие: %d", countBlocked(f.consents))
	}

	// exhausted — single_use потреблена первой авторизацией.
	f = newAuthorizationFixture(t)
	thread, version = f.confirm(t)
	f.authorize(t, thread, version, authGoodKey)
	_, err = f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, version, authSecondKey))
	if !errors.As(err, &rejected) || rejected.State != domain.ConfirmationExhausted {
		t.Fatalf("исчерпанное согласие обязано отвергаться словом exhausted: %v", err)
	}
	if f.consents.consumedCount(thread, version) != 1 {
		t.Fatalf("исчерпание не добавляет потребления: %d", f.consents.consumedCount(thread, version))
	}
	if f.ops.operationCount() != 1 {
		t.Fatalf("вторая операция не существует: %d", f.ops.operationCount())
	}
	blockedTotal += countBlocked(f.consents)
	if blockedTotal != 4 {
		t.Fatalf("каждое отвержение лестницы обязано оставлять видимое событие authorization_blocked: %d", blockedTotal)
	}
}

// TestAuthorizeFreshnessRefusalConsumesNothing — the material condition
// moved between the preview and the authorization: the honest 409 with
// the 3-4 reason word, NO consumption, NO operation, the reader NOT
// called (the freshness gate precedes the material check).
func TestAuthorizeFreshnessRefusalConsumesNothing(t *testing.T) {
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	f.world.factsView.Facts[0].Versions[0].VersionNo = 9 // факт двинулся
	readerCallsBefore := f.reader.calls
	_, err := f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, version, authGoodKey))
	var stale port.AuthorizationStaleError
	if !errors.As(err, &stale) {
		t.Fatalf("устаревший материал обязан получать отказ свежести с причиной: %v", err)
	}
	if stale.Reason != domain.FreshnessReasonText(domain.FreshnessFactChanged) {
		t.Fatalf("причина обязана быть словом словаря 3-4: %q", stale.Reason)
	}
	if f.consents.consumedCount(thread, version) != 0 || f.ops.operationCount() != 0 {
		t.Fatal("отказ свежести НЕ потребляет и НЕ создаёт операций")
	}
	if f.reader.calls != readerCallsBefore {
		t.Fatalf("гейт свежести идёт ДО проверки материала — ридер не вызывается: %d", f.reader.calls)
	}
}

// TestAuthorizePolicyRefusalSpeaksReasonWord — the LIVE policy gate
// (GuardPreparePolicy reused) blocks with the exact reason word BEFORE
// the transaction; the consumption NEVER happens.
func TestAuthorizePolicyRefusalSpeaksReasonWord(t *testing.T) {
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	f.world.roles = []identityDomain.Role{identityDomain.ExpertQA} // роль вне AllowedRoles
	_, err := f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, version, authGoodKey))
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("policy-блок обязан быть ErrForbidden с точным словом: %v", err)
	}
	if f.consents.consumedCount(thread, version) != 0 || f.ops.operationCount() != 0 {
		t.Fatal("policy-блок НЕ потребляет и НЕ создаёт операций")
	}
}

// TestAuthorizeDeadWorkItemIsHonest404 — a dead work item reference
// answers the honest 404 BEFORE the material check (the AD-2 live-read
// discipline), nothing is consumed.
func TestAuthorizeDeadWorkItemIsHonest404(t *testing.T) {
	f := newAuthorizationFixture(t)
	f.world.caseView.WorkItems = nil // work item исчез из кейса
	thread, version := f.confirm(t)
	_, err := f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, version, authGoodKey))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("мёртвый work item обязан получать честный 404: %v", err)
	}
	if f.consents.consumedCount(thread, version) != 0 {
		t.Fatal("мёртвый work item не потребляет")
	}
}

// TestAuthorizeMaterialRefusalsFailClosed — the LAST material check: the
// nil reader fails closed (AG-4 — the production root honestly cannot
// authorize); a stale/missing required field refuses with the material
// condition word; the EXPLICITLY allowed uncertainty passes by the
// definition's declaration.
func TestAuthorizeMaterialRefusalsFailClosed(t *testing.T) {
	// nil-ридер — производство честно не авторизует.
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	f.service = New(previewIdentity(), acceptAllPacks{}, acceptAllCaps{}, f.defs).
		WithPreviews(f.previews, f.world.control, f.material,
			fakeMembers{roles: f.world.roles}, fakeEntitlement{status: f.world.admission},
							fakeReadiness{stage: f.world.stage, instances: f.world.instances},
							fakeConnectors{state: f.world.capState, contractVersion: f.world.contract},
							fakeRouteFacts{profileVersion: f.world.profile}, &fakeIDs{}).
		WithAuthorization(f.consents, f.ops) // ридер НЕ подключён
	_, err := f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, version, authGoodKey))
	if !errors.Is(err, port.ErrFreshReadsUnavailable) {
		t.Fatalf("nil-ридер обязан fail-closed отказывать честным словом: %v", err)
	}
	if f.consents.consumedCount(thread, version) != 0 {
		t.Fatal("недоступность свежих чтений не потребляет")
	}

	// Обязательное поле читается устаревшим — материальное условие изменилось.
	f = newAuthorizationFixture(t)
	thread, version = f.confirm(t)
	f.reader.rows = []domain.MaterialSnapshotRow{
		{MaterialField: "price", Status: domain.SnapshotStale},
		{MaterialField: "availability", Status: domain.SnapshotFresh},
	}
	_, err = f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, version, authGoodKey))
	if !errors.Is(err, port.ErrMaterialConditionChanged) {
		t.Fatalf("устаревшее обязательное поле обязано получать 409 материального условия: %v", err)
	}

	// Обязательное поле не ответило вовсе — fail-closed.
	f = newAuthorizationFixture(t)
	thread, version = f.confirm(t)
	f.reader.rows = []domain.MaterialSnapshotRow{
		{MaterialField: "price", Status: domain.SnapshotFresh},
	}
	_, err = f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, version, authGoodKey))
	if !errors.Is(err, port.ErrMaterialConditionChanged) {
		t.Fatalf("отсутствующее обязательное поле обязано отказывать fail-closed: %v", err)
	}

	// ЯВНО объявленная неопределённость проходит по объявлению
	// определения: словарь допуска 2-7 — объявленное слово ДОЛЖНО
	// совпасть со словом ридера (MapSnapshotStatus: reader stale +
	// declared stale → allowed_uncertain — неопределённость не выводится
	// из молчания).
	uncertainDefs := append([]domain.ActionDefinition(nil), fixtureDefinitions()...)
	changed := uncertainDefs[0]
	changed.ID = "quote.send-uncertain-quote"
	changed.Conditions.RequiredFacts = []domain.FactRequirement{
		{MaterialField: "price", Freshness: "fresh"},
		{MaterialField: "availability", Freshness: domain.SnapshotStale},
	}
	uncertainDefs = append(uncertainDefs, changed)
	g := newAuthorizationFixtureWithDefs(t, uncertainDefs)
	g.preview.DefinitionID = changed.ID
	g.preview.DefinitionVersion = changed.Version
	g.preview.Digest = domain.PreviewDigest(g.previews.key, g.preview.PreviewID, g.preview.VersionNo, domain.PreviewCanonicalBytes(g.preview))
	g.ops.pinPreview(g.preview)
	g.reader.rows = []domain.MaterialSnapshotRow{
		{MaterialField: "price", Status: domain.SnapshotFresh, ExternalID: "SKU-42", ExternalVersion: "7"},
		{MaterialField: "availability", Status: domain.SnapshotStale, ExternalID: "SKU-42", ExternalVersion: "7"},
	}
	confirmUncertain := g.confirmCommand()
	outcome, err := g.service.ConfirmPreviewVersion(context.Background(), confirmUncertain)
	if err != nil {
		t.Fatalf("согласие с явно разрешённой неопределённостью обязано проходить: %v", err)
	}
	authorized, err := g.service.AuthorizeOperation(context.Background(), g.authorizeCommand(outcome.ConfirmationID, outcome.VersionNo, authGoodKey))
	if err != nil {
		t.Fatalf("allowed_uncertain проходит ЯВНО по объявлению определения: %v", err)
	}
	if authorized.Replay || authorized.ReservationReference {
		t.Fatalf("это первая авторизация: %+v", authorized)
	}
}

// TestAuthorizeRollbackIsAllOrNothing — the injected failure at the THIRD
// insert (the operation row) rolls back the WHOLE chain: NO consumed
// event, NO reservation, NO operation, NO ledger, NO outbox («Confirmation
// остаётся непотреблённой» — AC-6 материальна); the retry of the same
// request passes cleanly.
func TestAuthorizeRollbackIsAllOrNothing(t *testing.T) {
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	f.ops.failAtInsert = 3
	_, err := f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, version, authGoodKey))
	if err == nil {
		t.Fatal("инжект сбоя обязан доходить ошибкой")
	}
	if f.consents.consumedCount(thread, version) != 0 {
		t.Fatalf("после сбоя подтверждение обязано остаться непотреблённым: %d", f.consents.consumedCount(thread, version))
	}
	if len(f.ops.reservations) != 0 || f.ops.operationCount() != 0 || len(f.ops.outbox) != 0 || len(f.ops.ledgers) != 0 {
		t.Fatalf("после сбоя не существует НИ reservation, НИ операции, НИ ledger, НИ outbox: %+v", f.ops)
	}
	// Повтор того же запроса проходит чисто (первой транзакции не существовало).
	f.ops.failAtInsert = 0
	outcome := f.authorize(t, thread, version, authGoodKey)
	if outcome.Replay {
		t.Fatalf("после отката повтор — первая успешная авторизация: %+v", outcome)
	}
	if f.consents.consumedCount(thread, version) != 1 {
		t.Fatalf("повтор обязан потребить ровно один раз: %d", f.consents.consumedCount(thread, version))
	}
}

// TestAuthorizeTwinsOfOneKeyAreOneOperation — the raced twins of one
// client key (both passed the pre-transaction replay check — the race
// window) map to ONE operation: the reservation constraint fires first
// (the same material = the same intent), the digest comparison answers
// the honest replay return, the consumption stays at one. The UNLIMITED
// consumption rule keeps the raced twin past the allowance re-check —
// the honest guard-valid shape of the race (the single_use race answers
// exhausted честным словом — the Implementation Notes semantics, pinned
// by the ladder test above).
func TestAuthorizeTwinsOfOneKeyAreOneOperation(t *testing.T) {
	unlimitedDefs := append([]domain.ActionDefinition(nil), fixtureDefinitions()...)
	unlimitedDefs[0].Conditions.ConsumptionRule = domain.ConsumptionUnlimited
	f := newAuthorizationFixtureWithDefs(t, unlimitedDefs)
	thread, version := f.confirm(t)
	f.authorize(t, thread, version, authGoodKey)
	// Окно гонки: обе попытки прошли replay-проверку до коммита победителя.
	f.ops.hideKey = authGoodKey
	outcome := f.authorize(t, thread, version, authGoodKey)
	f.ops.hideKey = ""
	if !outcome.Replay {
		t.Fatalf("близнец того же ключа обязан мапиться в replay-возврат: %+v", outcome)
	}
	if outcome.Operation.IdempotencyKey != authGoodKey {
		t.Fatalf("возврат обязан вернуть операцию того же ключа: %+v", outcome.Operation)
	}
	if f.ops.operationCount() != 1 {
		t.Fatalf("близнецы одного ключа = ровно ОДНА операция: %d", f.ops.operationCount())
	}
	if f.consents.consumedCount(thread, version) != 1 {
		t.Fatalf("потребление остаётся ровно одним: %d", f.consents.consumedCount(thread, version))
	}
	if len(f.ops.outbox) != 1 {
		t.Fatalf("вторая outbox запись не существует: %d", len(f.ops.outbox))
	}
}

// TestAuthorizeCrossCaseLoserReadsHonestReference — the equivalent
// business intent is ALREADY reserved (the parallel Case committed
// first): the loser answers the honest 200 reference to the existing
// tenant-visible operation, the loser's consumption NEVER happened (the
// whole transaction rolled back).
func TestAuthorizeCrossCaseLoserReadsHonestReference(t *testing.T) {
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	winner := f.authorize(t, thread, version, authGoodKey)
	// Параллельный кейс уже зарезервировал то же намерение (коммит
	// победителя — с ledger и outbox, существование операции честно).
	secondWorld := newAuthorizationFixture(t)
	secondThread, secondVersion := secondWorld.confirm(t)
	intentKey := domain.DeriveBusinessIntentKey(secondWorld.previews.key, domain.BusinessIntentInput{
		TenantID:     testTenant,
		OwningSystem: "catalog",
		DefinitionID: secondWorld.def.ID,
		Target:       secondWorld.preview.Target,
		TypedInput:   secondWorld.preview.TypedInput,
		Snapshot:     secondWorld.preview.Snapshot,
	})
	secondWorld.ops.reservations["catalog\x1f"+intentKey] = winner.Operation.OperationID
	winnerRow, _ := f.ops.operationOfKey(authGoodKey)
	secondWorld.ops.operations = append(secondWorld.ops.operations, winnerRow)
	secondWorld.ops.ledgers[winnerRow.OperationID] = append([]domain.OperationLedgerEntry(nil), f.ops.ledgers[winnerRow.OperationID]...)
	winnerPayload, hasPayload := f.ops.outbox[winnerRow.OperationID]
	if !hasPayload {
		t.Fatal("payload победителя обязана существовать")
	}
	secondWorld.ops.outbox[winnerRow.OperationID] = winnerPayload
	outcome, err := secondWorld.service.AuthorizeOperation(context.Background(), secondWorld.authorizeCommand(secondThread, secondVersion, authSecondKey))
	if err != nil {
		t.Fatalf("проигравший обязан получать 200-ссылку, не ошибку: %v", err)
	}
	if !outcome.ReservationReference || outcome.OutcomeText != domain.ReservationDuplicateText {
		t.Fatalf("исход проигравшего обязан быть ссылкой с фиксированным текстом: %+v", outcome)
	}
	if outcome.Operation.OperationID != winner.Operation.OperationID {
		t.Fatalf("ссылка обязана вести на существующую операцию: %+v", outcome.Operation)
	}
	if secondWorld.consents.consumedCount(secondThread, secondVersion) != 0 {
		t.Fatalf("потребление проигравшего НЕ произошло: %d", secondWorld.consents.consumedCount(secondThread, secondVersion))
	}
	if len(secondWorld.ops.reservations) != 1 {
		t.Fatalf("вторая резервация не существует: %d", len(secondWorld.ops.reservations))
	}
}

// TestReserveOperationParamsCarryNoPorts — the persist-before-emit
// discipline is the SIGNATURE itself: the ONE transactional method takes
// zero adapters/ports (the Enqueue precedent of 3-3) — the params struct
// carries data fields only.
func TestReserveOperationParamsCarryNoPorts(t *testing.T) {
	paramsType := reflect.TypeOf(port.ReserveOperationParams{})
	for i := 0; i < paramsType.NumField(); i++ {
		field := paramsType.Field(i)
		if field.Type.Kind() == reflect.Interface {
			t.Fatalf("поле %s не имеет права быть интерфейсом/портом — persist-before-emit структурен", field.Name)
		}
	}
	_, found := reflect.TypeOf(&Service{}).MethodByName("ReserveOperation")
	if found {
		t.Fatal("ReserveOperation — контракт репозитория, не метод сервиса")
	}
}

// TestAuthorizeGateOrderPinnedByCounters — the FROZEN order of the
// gateway: the replay check FIRST (the found operation answers without
// ANY live reads), the ladder refusals happen BEFORE the case material
// read, the material reader is the LAST check only.
func TestAuthorizeGateOrderPinnedByCounters(t *testing.T) {
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	f.authorize(t, thread, version, authGoodKey)
	caseCallsBefore := f.material.caseCalls
	// Replay-возврат: НИ одного живого чтения кейса/фактов.
	f.authorize(t, thread, version, authGoodKey)
	if f.material.caseCalls != caseCallsBefore {
		t.Fatalf("replay-возврат не читает живой материал кейса: %d", f.material.caseCalls)
	}
	// Отвержение лестницы (revoked): пин и события прочитаны, кейс НЕ читан.
	if _, err := f.service.RevokeConfirmation(context.Background(), api.RevokeConfirmationCommand{
		Actor: stepUpActor(), TenantID: testTenant, CaseID: previewCaseID,
		ConfirmationID: thread, VersionNo: version, Now: f.now,
	}); err != nil {
		t.Fatalf("отзыв обязан проходить: %v", err)
	}
	caseCallsBefore = f.material.caseCalls
	_, err := f.service.AuthorizeOperation(context.Background(), f.authorizeCommand(thread, version, authSecondKey))
	var rejected port.ConfirmationRejectedError
	if !errors.As(err, &rejected) || rejected.State != domain.ConfirmationRevoked {
		t.Fatalf("отозванное согласие обязано отвергаться словом revoked: %v", err)
	}
	if f.material.caseCalls != caseCallsBefore {
		t.Fatalf("отвержение лестницы идёт ДО чтения живого материала кейса: %d", f.material.caseCalls)
	}
}

// TestReadAuthorizationViewDerivesLiveWords — the view speaks the live
// derivations: the consent card shows the consumption words and the
// state, the operation card shows authorized/queued with the honest
// boundary texts, the events stay visible (confirmed + consumed).
func TestReadAuthorizationViewDerivesLiveWords(t *testing.T) {
	f := newAuthorizationFixture(t)
	thread, version := f.confirm(t)
	f.authorize(t, thread, version, authGoodKey)
	view, err := f.service.ReadAuthorizationView(context.Background(), testActor, testTenant, previewCaseID, f.now)
	if err != nil {
		t.Fatalf("чтение вида обязано проходить: %v", err)
	}
	if view.EmptyText != "" {
		t.Fatalf("пустого состояния здесь нет: %q", view.EmptyText)
	}
	if len(view.Confirmations) != 1 || len(view.Operations) != 1 {
		t.Fatalf("вид обязан нести одну карточку согласия и одну операцию: %+v", view)
	}
	consent := view.Confirmations[0]
	if consent.State != domain.ConfirmationExhausted {
		t.Fatalf("single_use потреблённая карточка обязана говорить exhausted: %q", consent.State)
	}
	if consent.ConsumptionText != "Потреблено 1 из 1." {
		t.Fatalf("слова потребления обязаны быть честными: %q", consent.ConsumptionText)
	}
	eventKinds := map[string]bool{}
	for _, event := range consent.Events {
		eventKinds[event.Kind] = true
	}
	if !eventKinds[domain.ConfirmationEventConfirmed] || !eventKinds[domain.ConfirmationEventConsumed] {
		t.Fatalf("история обязана показывать confirmed и consumed: %+v", consent.Events)
	}
	card := view.Operations[0]
	if card.State != "authorized/queued" || card.StateText == "" {
		t.Fatalf("карточка операции обязана говорить authorized/queued: %+v", card)
	}
}

// TestReadAuthorizationViewEmptyStateIsHonest — the empty state speaks
// the honest words (the surface never pretends).
func TestReadAuthorizationViewEmptyStateIsHonest(t *testing.T) {
	f := newAuthorizationFixture(t)
	view, err := f.service.ReadAuthorizationView(context.Background(), testActor, testTenant, previewCaseID, f.now)
	if err != nil {
		t.Fatalf("чтение пустого вида обязано проходить: %v", err)
	}
	if view.EmptyText == "" {
		t.Fatal("пустое состояние обязано говорить честные слова")
	}
	if len(view.Confirmations) != 0 || len(view.Operations) != 0 {
		t.Fatalf("пустой вид не несёт карточек: %+v", view)
	}
}
