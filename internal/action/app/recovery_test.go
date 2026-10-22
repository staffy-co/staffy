package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/action/api"
	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.8 step-02: the fake-repo unit matrix of the recovery worker
// ---------------------------------------------------------------------------
//
// The counters pin the FROZEN ladder order of the creation command: the
// scope gate answers BEFORE the facts read (the non-member and the
// scope-less member never reach the operation), the eligibility ladder
// answers BEFORE the existing-case read and NEVER writes (the fake-repo
// counters pin the zero-write property on EVERY gas), the assignment
// reads (the case view, the route facts) happen ONLY on the creation
// path (the idempotent/evidence repeats never re-derive the assignment),
// and the ONE creation transaction lands the row with the created event.
// The reflect probe pins the LOCALITY of the path: not ONE provider
// adapter rides the command signature, the store params or the
// repository contract (the network stays in the dispatch/reconciliation
// contours — AG-4).

// ---------------------------------------------------------------------------
// The fakes of the unit contour
// ---------------------------------------------------------------------------

// recoveryCaseID is the case of the unit world.
var recoveryCaseID = id.MustParse("01994f64-6c00-7000-8000-0000000000c1")

// recoveryWorkID is the work item of the unit world.
var recoveryWorkID = id.MustParse("01994f64-6c00-7000-8000-0000000000c2")

// recoveryOperationID is the operation of the unit world.
var recoveryOperationID = id.MustParse("01994f64-6c00-7000-8000-0000000000c3")

// recoveryOwnerRef is the current owner of the original Case.
var recoveryOwnerRef = id.MustParse("01994f64-6c00-7000-8000-0000000000c5")

// recoveryBackupOwner is the backup owner of the coverage.
var recoveryBackupOwner = id.MustParse("01994f64-6c00-7000-8000-0000000000c6")

// recoveryNow is the moment of the unit commands.
var recoveryNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

// fakeRecoveryRepo mirrors the store semantics of the contour: the
// recorded facts read (scripted), the existing case read by the
// operation pin and the ONE creation transaction (the existing case
// under the "lock" decides the idempotent/evidence/create path — the
// domain comparator and the domain delta derivation are the SAME
// functions the store calls).
type fakeRecoveryRepo struct {
	factsCalls       int
	byOperationCalls int
	caseCalls        int
	casesCalls       int
	createCalls      int

	facts    port.RecoveryOperationFacts
	factsErr error

	rows map[id.UUID]*port.RecoveryCaseRow

	failByOperation error
	failCreate      error
	lastCreate      *port.CreateRecoveryCaseParams
}

func newFakeRecoveryRepo(facts port.RecoveryOperationFacts) *fakeRecoveryRepo {
	return &fakeRecoveryRepo{facts: facts, rows: map[id.UUID]*port.RecoveryCaseRow{}}
}

func (r *fakeRecoveryRepo) ReadRecoveryOperationFacts(context.Context, tenancy.Context, id.UUID) (port.RecoveryOperationFacts, error) {
	r.factsCalls++
	if r.factsErr != nil {
		return port.RecoveryOperationFacts{}, r.factsErr
	}
	return r.facts, nil
}

func (r *fakeRecoveryRepo) ReadRecoveryCaseByOperation(_ context.Context, _ tenancy.Context, operationID id.UUID) (port.RecoveryCaseRow, error) {
	r.byOperationCalls++
	if r.failByOperation != nil {
		return port.RecoveryCaseRow{}, r.failByOperation
	}
	if row, ok := r.rows[operationID]; ok {
		return *row, nil
	}
	return port.RecoveryCaseRow{}, port.ErrRecoveryNotFound
}

func (r *fakeRecoveryRepo) ReadRecoveryCase(_ context.Context, _ tenancy.Context, caseRowID id.UUID) (port.RecoveryCaseRow, error) {
	r.caseCalls++
	for _, row := range r.rows {
		if row.RecoveryCaseID == caseRowID {
			return *row, nil
		}
	}
	return port.RecoveryCaseRow{}, port.ErrRecoveryNotFound
}

func (r *fakeRecoveryRepo) ReadRecoveryCases(context.Context, tenancy.Context, int) ([]port.RecoveryCaseRow, error) {
	r.casesCalls++
	list := []port.RecoveryCaseRow{}
	for _, row := range r.rows {
		list = append(list, *row)
	}
	return list, nil
}

func (r *fakeRecoveryRepo) CreateRecoveryCase(_ context.Context, p port.CreateRecoveryCaseParams) (port.CreateRecoveryCaseResult, error) {
	r.createCalls++
	snapshot := p
	r.lastCreate = &snapshot
	if r.failCreate != nil {
		return port.CreateRecoveryCaseResult{}, r.failCreate
	}
	if existing, ok := r.rows[p.Operation.OperationID]; ok {
		decision, err := domain.DeriveRecoveryRepeatDecision(existing.BundleDigest, p.BundleDigest)
		if err != nil {
			return port.CreateRecoveryCaseResult{}, err
		}
		if decision == domain.RecoveryRepeatIdempotent {
			return port.CreateRecoveryCaseResult{Case: *existing, Decision: port.RecoveryDecisionIdempotent}, nil
		}
		eventNo := int64(len(existing.Events)) + 1
		existing.Events = append(existing.Events, domain.RecoveryEventRow{
			EventNo:   eventNo,
			Kind:      domain.RecoveryEventEvidenceAppended,
			Details:   domain.DeriveRecoveryEvidenceDelta(existing.Bundle, p.Bundle),
			ActorID:   p.ActorID,
			CreatedAt: p.Now,
		})
		return port.CreateRecoveryCaseResult{Case: *existing, Decision: port.RecoveryDecisionEvidence}, nil
	}
	row := port.RecoveryCaseRow{
		RecoveryCaseID: p.RecoveryCaseID,
		RecoveryCaseNo: int64(len(r.rows) + 1),
		RecoveryKey:    p.RecoveryKey,
		OperationID:    p.Operation.OperationID,
		Trigger:        p.Trigger,
		Bundle:         p.Bundle,
		BundleDigest:   p.BundleDigest,
		Assignment:     p.Assignment,
		DeadlineAt:     p.DeadlineAt,
		CreatedBy:      p.ActorID,
		CreatedAt:      p.Now,
		Events: []domain.RecoveryEventRow{{
			EventNo:   1,
			Kind:      domain.RecoveryEventCreated,
			Details:   "Recovery Case создан: триггер " + p.Trigger + ".",
			ActorID:   p.ActorID,
			CreatedAt: p.Now,
		}},
	}
	r.rows[p.Operation.OperationID] = &row
	return port.CreateRecoveryCaseResult{Case: row, Decision: port.RecoveryDecisionCreated}, nil
}

// recoveryCaseMaterial is the LIVE case view fake of the assignment
// path: the current ownership term, the original conversation and the
// configurable unreadability.
type recoveryCaseMaterial struct {
	view       casesAPI.CaseView
	viewErr    error
	caseCalls  int
	factsCalls int
}

func (f *recoveryCaseMaterial) CaseView(context.Context, identityDomain.Principal, id.UUID, id.UUID) (casesAPI.CaseView, error) {
	f.caseCalls++
	if f.viewErr != nil {
		return casesAPI.CaseView{}, f.viewErr
	}
	return f.view, nil
}

func (f *recoveryCaseMaterial) FactsView(context.Context, identityDomain.Principal, id.UUID, id.UUID) (casesAPI.FactsView, error) {
	f.factsCalls++
	return casesAPI.FactsView{}, nil
}

// recoveryRouteFacts is the LIVE coverage fake of the assignment path:
// the inbound candidate of the channel with the backup owner and the
// ack-deadline minutes, and the configurable unreadability.
type recoveryRouteFacts struct {
	routes     employeeAPI.RouteFacts
	routesErr  error
	routeCalls int
}

func (f *recoveryRouteFacts) RouteFacts(context.Context, tenancy.Context, id.UUID) (employeeAPI.RouteFacts, error) {
	f.routeCalls++
	if f.routesErr != nil {
		return employeeAPI.RouteFacts{}, f.routesErr
	}
	return f.routes, nil
}

// recoveryFixture is the unit world of the recovery contour.
type recoveryFixture struct {
	repo     *fakeRecoveryRepo
	material *recoveryCaseMaterial
	routes   *recoveryRouteFacts
	previews *fakePreviewRepo
	defs     []domain.ActionDefinition
	now      time.Time

	// The policy world of 3-9 (recovery_policy_test.go): the created
	// case pin and the wired service of the policy reads.
	policyCaseID  id.UUID
	policyService *Service
}

// recoveryFacts builds the RECORDED facts of the unit world: the
// unknown-exhausted baseline (the ledger [authorized, submitted,
// unknown], three reconciliation lease rows, one dispatch attempt, the
// unknown receipt chain).
func recoveryFacts(state string, reconcileAttempts, dispatchAttempts int) port.RecoveryOperationFacts {
	ledger := []domain.OperationLedgerEntry{
		{EntryNo: 1, Kind: domain.LedgerEntryAuthorized, Details: "авторизовано"},
		{EntryNo: 2, Kind: domain.LedgerEntrySubmitted, Details: "write-ahead"},
	}
	switch state {
	case domain.OperationUnknown:
		ledger = append(ledger, domain.OperationLedgerEntry{EntryNo: 3, Kind: domain.LedgerEntryUnknown, Details: "исход неизвестен"})
	case domain.OperationFailed:
		ledger = append(ledger, domain.OperationLedgerEntry{EntryNo: 3, Kind: domain.LedgerEntryFailed, Details: "отказ"})
	case domain.OperationCommitted:
		ledger = append(ledger, domain.OperationLedgerEntry{EntryNo: 3, Kind: domain.LedgerEntryCommitted, Details: "успех"})
	}
	return port.RecoveryOperationFacts{
		Operation: domain.Operation{
			TenantID:              testTenant,
			OperationID:           recoveryOperationID,
			DefinitionID:          "quote.send-price-quote",
			DefinitionVersion:     "1.2.0",
			PreviewID:             recoveryCaseID,
			PreviewVersionNo:      1,
			ConfirmationID:        recoveryWorkID,
			ConfirmationVersionNo: 2,
			CaseID:                recoveryCaseID,
			WorkItemID:            recoveryWorkID,
			Target:                domain.ProposalTarget{Kind: "catalog_item", Ref: recoveryWorkID},
			OwnerKind:             "human",
			OwnerRef:              recoveryOwnerRef,
			OwnerEpoch:            4,
			ReleaseDecisionNo:     9,
			ProfileVersion:        2,
			BusinessIntentKey:     "intent-key-0123456789abcdef0123456789abcdef",
			IdempotencyKey:        "client-key-word-1",
			MaterialDigest:        "material-0123456789abcdef0123456789abcdef",
			CreatedAt:             recoveryNow.Add(-time.Hour),
		},
		Ledger:                 ledger,
		StateWord:              state,
		ReconcileAttemptCount:  reconcileAttempts,
		DispatchAttemptCount:   dispatchAttempts,
		ReceiptVersionsCount:   1,
		LatestReceiptVersionNo: 1,
		LatestReceiptOutcome:   domain.ReceiptOutcomeUnknown,
		Observations:           []domain.VersionPin{{Ref: recoveryWorkID, Version: 3}},
	}
}

// newRecoveryFixture wires the unit world: the exhausted-unknown
// baseline facts, the LIVE assignment words (the ownership term of the
// maximal epoch, the inbound coverage of the web_chat channel) and the
// fixture dictionary of the catalog.
func newRecoveryFixture(t *testing.T) *recoveryFixture {
	t.Helper()
	facts := recoveryFacts(domain.OperationUnknown, domain.MaxReconciliationAttempts, 1)
	repo := newFakeRecoveryRepo(facts)
	material := &recoveryCaseMaterial{view: casesAPI.CaseView{
		TenantID: testTenant,
		Case:     casesDomain.Case{TenantID: testTenant, CaseID: recoveryCaseID, CaseNo: 7, EmployeeID: previewEmployee},
		Conversations: []casesDomain.Conversation{{
			TenantID: testTenant, CaseID: recoveryCaseID, ConversationID: previewConnID,
			ConversationNo: 1, EmployeeID: previewEmployee, ConnectionID: previewConnID,
			Modality: casesDomain.ModalityText,
		}},
		Ownership: casesAPI.OwnershipView{State: casesDomain.OwnershipState{
			Current: &casesDomain.OwnershipTerm{
				TenantID: testTenant, CaseID: recoveryCaseID, TermID: recoveryOwnerRef,
				Epoch: 4, OwnerKind: "human", OwnerRef: recoveryOwnerRef,
			},
		}},
	}}
	routes := &recoveryRouteFacts{routes: employeeAPI.RouteFacts{
		ConnectionID: previewConnID,
		Candidates: []employeeAPI.RouteCandidate{{
			EmployeeID: previewEmployee, DecisionNo: 1, ProfileVersion: 2,
			Coverage: []employeeDomain.HumanCoverage{{
				Route:              employeeDomain.Route{Channel: "web_chat", Direction: "inbound"},
				ResponseOwnerID:    recoveryOwnerRef,
				BackupOwnerID:      recoveryBackupOwner,
				AckDeadlineMinutes: 30,
			}},
		}},
	}}
	return &recoveryFixture{
		repo:     repo,
		material: material,
		routes:   routes,
		previews: newFakePreviewRepo(),
		defs:     fixtureDefinitions(),
		now:      recoveryNow,
	}
}

// recoveryService wires the recovery contour over the unit fakes: the
// operations scope gate, the preview key port (the HMAC key), the LIVE
// case material and route facts (the assignment words) and the recovery
// repository. The reconcile adapter stays nil — the AG-4 production fact
// (the eligibility ladder reads it as the composition truth).
func (f *recoveryFixture) recoveryService(member bool, scopes map[identityDomain.Scope]bool) *Service {
	if scopes == nil {
		scopes = map[identityDomain.Scope]bool{identityDomain.Operations: true}
	}
	return New(scopeIdentity{member: member, scopes: scopes}, acceptAllPacks{}, acceptAllCaps{}, f.defs).
		WithPreviews(f.previews, fakeControl{mode: port.ControlEffective{Mode: domain.ScopedControlAccept, Generation: 3}},
			f.material, liveMembers{world: &previewLiveFixture{}}, fakeEntitlement{status: "active"},
			fakeReadiness{stage: "admitted"}, fakeConnectors{state: "active"}, f.routes, &fakeIDs{}).
		WithRecovery(f.repo)
}

// command is the passing creation command of the fixture moment.
func (f *recoveryFixture) command() RecoveryCreateCommand {
	return RecoveryCreateCommand{Actor: testActor, TenantID: testTenant, OperationID: recoveryOperationID, Now: f.now}
}

// bundleOf rebuilds the SAME SAFE bundle the app derives from the
// fixture facts (the comparator input of the idempotent/evidence
// probes).
func bundleOf(facts port.RecoveryOperationFacts) domain.RecoveryBundle {
	return domain.RecoveryBundle{
		CaseID:                facts.Operation.CaseID,
		WorkItemID:            facts.Operation.WorkItemID,
		OperationID:           facts.Operation.OperationID,
		DefinitionID:          facts.Operation.DefinitionID,
		DefinitionVer:         facts.Operation.DefinitionVersion,
		PreviewID:             facts.Operation.PreviewID,
		PreviewVersionNo:      facts.Operation.PreviewVersionNo,
		ConfirmationID:        facts.Operation.ConfirmationID,
		ConfirmationVersionNo: facts.Operation.ConfirmationVersionNo,
		OwnerKind:             facts.Operation.OwnerKind,
		OwnerRef:              facts.Operation.OwnerRef,
		OwnerEpoch:            facts.Operation.OwnerEpoch,
		ReleaseDecisionNo:     facts.Operation.ReleaseDecisionNo,
		ProfileVersion:        facts.Operation.ProfileVersion,
		BusinessIntentKey:     facts.Operation.BusinessIntentKey,
		IdempotencyKey:        facts.Operation.IdempotencyKey,
		MaterialDigest:        facts.Operation.MaterialDigest,
		DispatchAttempts:      facts.DispatchAttemptCount,
		ReconcileAttempts:     facts.ReconcileAttemptCount,
		ReceiptVersionsCount:  facts.ReceiptVersionsCount,
		LatestReceiptVersion:  facts.LatestReceiptVersionNo,
		LatestReceiptOutcome:  facts.LatestReceiptOutcome,
		Observations:          facts.Observations,
		TargetKind:            facts.Operation.Target.Kind,
		TargetRef:             facts.Operation.Target.Ref,
		OwningSystem:          "catalog",
	}
}

// ---------------------------------------------------------------------------
// The ladder order: the counters pin the frozen discipline
// ---------------------------------------------------------------------------

// TestRecoveryCreateExhaustedUnknownCreatesCase — the happy creation
// path of the exhausted-unknown baseline: the trigger word, the
// assignment of the LIVE readings (the ownership term owner, the
// coverage backup and the deadline = now + 30 minutes), the created
// event and the honest created text.
func TestRecoveryCreateExhaustedUnknownCreatesCase(t *testing.T) {
	f := newRecoveryFixture(t)
	service := f.recoveryService(true, nil)
	result, err := service.createRecoveryCase(context.Background(), f.command())
	if err != nil {
		t.Fatalf("создание обязано проходить: %v", err)
	}
	if result.Decision != port.RecoveryDecisionCreated {
		t.Fatalf("решение: %q, ожидалось created", result.Decision)
	}
	if result.Text != domain.RecoveryCreatedText {
		t.Fatalf("honest-текст создания: %q", result.Text)
	}
	if result.Case.Trigger != domain.RecoveryTriggerReconciliationExhausted {
		t.Fatalf("триггер: %q, ожидался reconciliation_exhausted", result.Case.Trigger)
	}
	if result.Case.Assignment.OwnerRef != recoveryOwnerRef || result.Case.Assignment.OwnerEpoch != 4 {
		t.Fatalf("владелец назначения обязан быть термом исходного Case: %+v", result.Case.Assignment)
	}
	if result.Case.Assignment.BackupOwnerID != recoveryBackupOwner {
		t.Fatalf("backup обязан прийти из coverage: %+v", result.Case.Assignment)
	}
	wantDeadline := recoveryNow.Add(30 * time.Minute).UTC().Truncate(time.Microsecond)
	if !result.Case.DeadlineAt.Equal(wantDeadline) {
		t.Fatalf("дедлайн: %v, ожидался %v", result.Case.DeadlineAt, wantDeadline)
	}
	if len(result.Case.Events) != 1 || result.Case.Events[0].Kind != domain.RecoveryEventCreated {
		t.Fatalf("история обязана начинаться с created: %+v", result.Case.Events)
	}
	if f.repo.createCalls != 1 || f.material.caseCalls != 1 || f.routes.routeCalls != 1 {
		t.Fatalf("лестница: create=%d caseView=%d routeFacts=%d — создание читает назначение ОДИН раз",
			f.repo.createCalls, f.material.caseCalls, f.routes.routeCalls)
	}
}

// TestRecoveryCreateAG4Unavailable — the nil reconciliation adapter is
// the HONEST production fact (AG-4): an unknown operation below the
// attempt bound answers reconciliation_unavailable — the automatic
// contour does not exist, the human is mandatory, the case IS created.
func TestRecoveryCreateAG4Unavailable(t *testing.T) {
	f := newRecoveryFixture(t)
	f.repo.facts = recoveryFacts(domain.OperationUnknown, 0, 1)
	service := f.recoveryService(true, nil)
	result, err := service.createRecoveryCase(context.Background(), f.command())
	if err != nil {
		t.Fatalf("AG-4 путь обязан создавать кейс: %v", err)
	}
	if result.Case.Trigger != domain.RecoveryTriggerReconciliationUnavailable {
		t.Fatalf("триггер: %q, ожидался reconciliation_unavailable", result.Case.Trigger)
	}
	if result.Decision != port.RecoveryDecisionCreated {
		t.Fatalf("решение: %q, ожидалось created", result.Decision)
	}
}

// TestRecoveryCreateFailedNoRepeat — the failed operation without the
// retry right (no recorded no_effect, the attempts below the bound):
// the policy word is decided — the automation is spent, the case IS
// created with the operation_failed trigger.
func TestRecoveryCreateFailedNoRepeat(t *testing.T) {
	f := newRecoveryFixture(t)
	f.repo.facts = recoveryFacts(domain.OperationFailed, 0, 1)
	service := f.recoveryService(true, nil)
	result, err := service.createRecoveryCase(context.Background(), f.command())
	if err != nil {
		t.Fatalf("failed-без-повтора обязан создавать кейс: %v", err)
	}
	if result.Case.Trigger != domain.RecoveryTriggerOperationFailed {
		t.Fatalf("триггер: %q, ожидался operation_failed", result.Case.Trigger)
	}
}

// TestRecoveryCreateIneligibleGasesWriteNothing — every gas of the
// ladder writes NOTHING and reads no case: the committed refusal, the
// in-flight refusal, the not-exhausted refusal and the retry-available
// refusal answer by the honest word wrapped into ErrRecoveryIneligible
// (the fake-repo counters pin the zero-write property — the eligibility
// ladder NEVER writes on any gas).
func TestRecoveryCreateIneligibleGasesWriteNothing(t *testing.T) {
	cases := []struct {
		name        string
		facts       port.RecoveryOperationFacts
		wantRefusal string
	}{
		{"committed", recoveryFacts(domain.OperationCommitted, 0, 1), domain.RecoveryIneligibleCommittedText},
		{"в-исполнении-authorized", recoveryFacts(domain.OperationAuthorized, 0, 0), domain.RecoveryIneligibleInFlightText},
		{"в-исполнении-submitted", recoveryFacts(domain.OperationSubmitted, 0, 1), domain.RecoveryIneligibleInFlightText},
		{"в-исполнении-accepted", recoveryFacts(domain.OperationAccepted, 0, 1), domain.RecoveryIneligibleInFlightText},
		{"сверка-не-исчерпана", recoveryFacts(domain.OperationUnknown, 1, 1), domain.RecoveryIneligibleReconcileNotExhaustedText},
		{"повтор-доступен", recoveryFacts(domain.OperationFailed, 0, 1), domain.RecoveryIneligibleRetryAvailableText},
	}
	for _, tc := range cases {
		f := newRecoveryFixture(t)
		f.repo.facts = tc.facts
		if tc.name == "повтор-доступен" {
			// The recorded no_effect verdict unlocks the retry inside the
			// bound — the policy word is retry.
			f.repo.facts.NoEffectProven = true
		}
		service := f.recoveryService(true, nil)
		if tc.name == "сверка-не-исчерпана" {
			// The WIRED adapter makes the automatic contour alive: the
			// attempts below the bound refuse honestly (the right of the
			// automation is not taken away).
			service = service.WithReconcile(newFakeReconciliationRepo(), &fakeReconciliationAdapter{capability: "command.send-sms"})
		}
		_, err := service.createRecoveryCase(context.Background(), f.command())
		if !errors.Is(err, port.ErrRecoveryIneligible) {
			t.Fatalf("%s: ожидался ErrRecoveryIneligible, получено %v", tc.name, err)
		}
		if err == nil || !containsWord(err.Error(), tc.wantRefusal) {
			t.Fatalf("%s: слово отказа обязано быть %q: %v", tc.name, tc.wantRefusal, err)
		}
		if f.repo.createCalls != 0 {
			t.Fatalf("%s: газ НИЧЕГО не пишет — create=%d", tc.name, f.repo.createCalls)
		}
		// The existing-case read PRECEDES the gate by design (the repeat of
		// an existing case is a comparator question, never a new
		// eligibility question) — a gas WITHOUT a case reads it once and
		// finds nothing.
		if f.repo.byOperationCalls != 1 {
			t.Fatalf("%s: существующий кейс читается один раз до гейта — byOperation=%d", tc.name, f.repo.byOperationCalls)
		}
		if len(f.repo.rows) != 0 {
			t.Fatalf("%s: газ не оставляет строк — rows=%d", tc.name, len(f.repo.rows))
		}
		if f.material.caseCalls != 0 || f.routes.routeCalls != 0 {
			t.Fatalf("%s: газ не читает назначение — caseView=%d routeFacts=%d",
				tc.name, f.material.caseCalls, f.routes.routeCalls)
		}
	}
}

// containsWord reports the exact substring (the honest word probe).
func containsWord(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && (haystack == needle ||
		indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// TestRecoveryCreateScopeGateBeforeFacts — the membership FIRST (the
// non-member gets the 404 — no disclosure), then the operations scope
// word: BOTH answer before the facts read (the counters stay zero).
func TestRecoveryCreateScopeGateBeforeFacts(t *testing.T) {
	f := newRecoveryFixture(t)
	nonMember := f.recoveryService(false, nil)
	if _, err := nonMember.createRecoveryCase(context.Background(), f.command()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("не-участник обязан получать 404: %v", err)
	}
	if f.repo.factsCalls != 0 {
		t.Fatalf("гейт Membership стоит ДО чтения фактов: facts=%d", f.repo.factsCalls)
	}
	scopeless := f.recoveryService(true, map[identityDomain.Scope]bool{})
	_, err := scopeless.createRecoveryCase(context.Background(), f.command())
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("участник без полномочия обязан получать отказ: %v", err)
	}
	if !containsWord(err.Error(), recoveryScopeText) {
		t.Fatalf("отказ полномочия обязан говорить слово operations: %v", err)
	}
	if f.repo.factsCalls != 0 {
		t.Fatalf("гейт полномочия стоит ДО чтения фактов: facts=%d", f.repo.factsCalls)
	}
}

// TestRecoveryCreateUnknownOperation404 — an unknown operation (or the
// RLS blindness of a foreign tenant) answers the honest 404 wrapped with
// the word BEFORE any material.
func TestRecoveryCreateUnknownOperation404(t *testing.T) {
	f := newRecoveryFixture(t)
	f.repo.factsErr = port.ErrRecoveryNotFound
	service := f.recoveryService(true, nil)
	_, err := service.createRecoveryCase(context.Background(), f.command())
	if !errors.Is(err, port.ErrRecoveryNotFound) {
		t.Fatalf("неизвестная операция обязана отвечать 404: %v", err)
	}
	if !containsWord(err.Error(), recoveryOperationUnknownText) {
		t.Fatalf("404 обязан говорить честное слово: %v", err)
	}
}

// TestRecoveryCreateIdempotentRepeat — the repeat with the UNCHANGED
// bundle digest: the existing case returns (200, the honest word), NO
// second row, NO event — the assignment is NOT re-derived (the live
// reads never happen on the repeat path).
func TestRecoveryCreateIdempotentRepeat(t *testing.T) {
	f := newRecoveryFixture(t)
	service := f.recoveryService(true, nil)
	first, err := service.createRecoveryCase(context.Background(), f.command())
	if err != nil {
		t.Fatalf("первое создание обязано проходить: %v", err)
	}
	repeat, err := service.createRecoveryCase(context.Background(), f.command())
	if err != nil {
		t.Fatalf("повтор обязан проходить идемпотентно: %v", err)
	}
	if repeat.Decision != port.RecoveryDecisionIdempotent {
		t.Fatalf("решение повтора: %q, ожидалось idempotent", repeat.Decision)
	}
	if repeat.Text != domain.RecoveryIdempotentText {
		t.Fatalf("honest-текст повтора: %q", repeat.Text)
	}
	if repeat.Case.RecoveryCaseID != first.Case.RecoveryCaseID {
		t.Fatalf("повтор обязан вернуть СУЩЕСТВУЮЩИЙ кейс: %v vs %v", repeat.Case.RecoveryCaseID, first.Case.RecoveryCaseID)
	}
	if len(f.repo.rows) != 1 {
		t.Fatalf("вторая строка НЕ создаётся: rows=%d", len(f.repo.rows))
	}
	if len(repeat.Case.Events) != 1 {
		t.Fatalf("второго события НЕТ: events=%d", len(repeat.Case.Events))
	}
	if f.repo.createCalls != 1 {
		t.Fatalf("повтор НЕ входит в транзакцию создания (пре-чек решает): create=%d", f.repo.createCalls)
	}
	if f.material.caseCalls != 1 || f.routes.routeCalls != 1 {
		t.Fatalf("повтор НЕ читает назначение живьём: caseView=%d routeFacts=%d",
			f.material.caseCalls, f.routes.routeCalls)
	}
}

// TestRecoveryCreateEvidenceAppend — the repeat with the CHANGED digest
// (the later terminal evidence: the receipt chain grew to v2 with the
// committed outcome and the external object): ONE append-only
// evidence_appended row lands with the SAFE delta, the original bundle
// row is NEVER rewritten, the assignment is NOT re-derived.
func TestRecoveryCreateEvidenceAppend(t *testing.T) {
	f := newRecoveryFixture(t)
	service := f.recoveryService(true, nil)
	if _, err := service.createRecoveryCase(context.Background(), f.command()); err != nil {
		t.Fatalf("первое создание обязано проходить: %v", err)
	}
	// The later terminal evidence: the receipt v2 committed with the
	// external object, one more reconciliation attempt spent.
	later := recoveryFacts(domain.OperationUnknown, domain.MaxReconciliationAttempts, 1)
	later.ReceiptVersionsCount = 2
	later.LatestReceiptVersionNo = 2
	later.LatestReceiptOutcome = domain.ReceiptOutcomeCommitted
	later.LatestExternalObjectID = "ext-msg-42"
	later.LatestExternalObjectVersion = "3"
	later.LatestExternalObjectTime = recoveryNow.Add(-10 * time.Minute)
	f.repo.facts = later
	repeat, err := service.createRecoveryCase(context.Background(), f.command())
	if err != nil {
		t.Fatalf("дельта-повтор обязан проходить: %v", err)
	}
	if repeat.Decision != port.RecoveryDecisionEvidence {
		t.Fatalf("решение дельты: %q, ожидалось evidence", repeat.Decision)
	}
	if repeat.Text != domain.RecoveryEvidenceAppendedText {
		t.Fatalf("honest-текст дельты: %q", repeat.Text)
	}
	if len(f.repo.rows) != 1 {
		t.Fatalf("вторая строка НЕ создаётся: rows=%d", len(f.repo.rows))
	}
	if len(repeat.Case.Events) != 2 {
		t.Fatalf("дельта добавляет ОДНУ строку evidence_appended: events=%d", len(repeat.Case.Events))
	}
	if repeat.Case.Events[1].Kind != domain.RecoveryEventEvidenceAppended {
		t.Fatalf("kind дельты: %q", repeat.Case.Events[1].Kind)
	}
	if !containsWord(repeat.Case.Events[1].Details, "квант-квитанций 1→2") {
		t.Fatalf("SAFE-дельта обязана назвать изменившиеся пины: %q", repeat.Case.Events[1].Details)
	}
	// The original bundle row is never rewritten (the stored bundle keeps
	// the creation moment pins).
	stored := f.repo.rows[recoveryOperationID]
	if stored.Bundle.ReceiptVersionsCount != 1 || stored.Bundle.LatestReceiptVersion != 1 {
		t.Fatalf("исходный bundle НЕ переписывается: %+v", stored.Bundle)
	}
	if stored.Bundle.ExternalObjectID != "" {
		t.Fatalf("исходный внешний id остаётся пустым: %q", stored.Bundle.ExternalObjectID)
	}
	if f.material.caseCalls != 1 || f.routes.routeCalls != 1 {
		t.Fatalf("дельта НЕ читает назначение живьём: caseView=%d routeFacts=%d",
			f.material.caseCalls, f.routes.routeCalls)
	}
}

// TestRecoveryCreateCoverageFailClosed — the unreadable coverage (the
// route facts error, the unwired port, the coverage without the deadline
// word — minutes < 1): the honest fail-closed refusal writes NOTHING
// (the assignment is mandatory, the deadline and the backup are never
// invented).
func TestRecoveryCreateCoverageFailClosed(t *testing.T) {
	f := newRecoveryFixture(t)
	f.routes.routesErr = errors.New("route read failure")
	service := f.recoveryService(true, nil)
	_, err := service.createRecoveryCase(context.Background(), f.command())
	if !errors.Is(err, port.ErrRecoveryCoverageUnavailable) {
		t.Fatalf("нечитаемое покрытие обязано отказывать fail-closed: %v", err)
	}
	if f.repo.createCalls != 0 || len(f.repo.rows) != 0 {
		t.Fatalf("fail-closed пишет НИЧЕГО: create=%d rows=%d", f.repo.createCalls, len(f.repo.rows))
	}
	// The coverage without the deadline word (minutes < 1 — the
	// request-transfer precedent).
	noDeadline := newRecoveryFixture(t)
	noDeadline.routes.routes.Candidates[0].Coverage[0].AckDeadlineMinutes = 0
	_, err = noDeadline.recoveryService(true, nil).createRecoveryCase(context.Background(), noDeadline.command())
	if !errors.Is(err, port.ErrRecoveryCoverageUnavailable) {
		t.Fatalf("покрытие без слова дедлайна обязано отказывать: %v", err)
	}
	if noDeadline.repo.createCalls != 0 || len(noDeadline.repo.rows) != 0 {
		t.Fatalf("fail-closed пишет НИЧЕГО: create=%d rows=%d", noDeadline.repo.createCalls, len(noDeadline.repo.rows))
	}
}

// TestRecoveryCreateOwnerUnknownFailClosed — the original Case carries
// no current ownership term (the pre-migration honest state): the
// fail-closed refusal writes NOTHING.
func TestRecoveryCreateOwnerUnknownFailClosed(t *testing.T) {
	f := newRecoveryFixture(t)
	f.material.view.Ownership.State.Current = nil
	service := f.recoveryService(true, nil)
	_, err := service.createRecoveryCase(context.Background(), f.command())
	if !errors.Is(err, port.ErrRecoveryOwnerUnknown) {
		t.Fatalf("владелец-без-термов обязан отказывать fail-closed: %v", err)
	}
	if f.repo.createCalls != 0 || len(f.repo.rows) != 0 {
		t.Fatalf("fail-closed пишет НИЧЕГО: create=%d rows=%d", f.repo.createCalls, len(f.repo.rows))
	}
}

// TestRecoveryCreatePortsMissing — the unwired contour refuses honestly
// in words (the preview/dispatch/reconcile precedents).
func TestRecoveryCreatePortsMissing(t *testing.T) {
	f := newRecoveryFixture(t)
	service := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Operations: true}},
		acceptAllPacks{}, acceptAllCaps{}, f.defs)
	_, err := service.createRecoveryCase(context.Background(), f.command())
	if err == nil || !containsWord(err.Error(), recoveryPortsMissingText) {
		t.Fatalf("несконфигурированный контур обязан отказывать словом: %v", err)
	}
}

// TestRecoveryReadSurfaces — the card and the panel reads ride the
// scope gate and the repository (the operator surface of step-03).
func TestRecoveryReadSurfaces(t *testing.T) {
	f := newRecoveryFixture(t)
	service := f.recoveryService(true, nil)
	created, err := service.createRecoveryCase(context.Background(), f.command())
	if err != nil {
		t.Fatalf("создание обязано проходить: %v", err)
	}
	card, err := service.ReadRecoveryCase(context.Background(), testActor, testTenant, created.Case.RecoveryCaseID)
	if err != nil {
		t.Fatalf("карточка обязана читаться: %v", err)
	}
	if card.RecoveryCaseID != created.Case.RecoveryCaseID || card.Trigger != created.Case.Trigger {
		t.Fatalf("карточка обязана вернуть тот же кейс: %+v", card)
	}
	list, err := service.ReadRecoveryCases(context.Background(), testActor, testTenant)
	if err != nil || len(list) != 1 {
		t.Fatalf("список панели обязан вернуть один кейс: %v, %d", err, len(list))
	}
	if _, err := service.ReadRecoveryCase(context.Background(), testActor, testTenant, recoveryOperationID); !errors.Is(err, port.ErrRecoveryNotFound) {
		t.Fatalf("чужой идентификатор обязан отвечать 404: %v", err)
	}
	if _, err := service.ReadRecoveryCase(context.Background(), testActor, testTenant, created.Case.RecoveryCaseID); err != nil {
		t.Fatalf("не-участник гейтится ДО чтения: %v", err)
	}
}

// ---------------------------------------------------------------------------
// The locality probe: no provider adapter rides the path signatures
// ---------------------------------------------------------------------------

// TestRecoveryPathCarriesNoProviderAdapters pins the LOCALITY of the
// slice structurally (the reflect probe — the 3-7 read-only precedent):
// the command type, the store params and the repository contract carry
// NO provider adapter (the mutation adapter, the read-only query
// adapter, the owning-system reader, the dry runner) — the network
// stays in the dispatch/reconciliation contours, the creation command
// is local BY TYPE.
func TestRecoveryPathCarriesNoProviderAdapters(t *testing.T) {
	providerTypes := []reflect.Type{
		reflect.TypeOf((*port.MutationAdapter)(nil)).Elem(),
		reflect.TypeOf((*port.ReconciliationAdapter)(nil)).Elem(),
		reflect.TypeOf((*port.OwningSystemReader)(nil)).Elem(),
		reflect.TypeOf((*port.DryRunner)(nil)).Elem(),
	}
	check := func(name string, inspected reflect.Type) {
		for _, provider := range providerTypes {
			if inspected == provider || inspected.Implements(provider) {
				t.Fatalf("%s несёт адаптер-провайдер %s — путь обязан быть локальным", name, provider.String())
			}
			for i := 0; i < inspected.NumField(); i++ {
				field := inspected.Field(i).Type
				if field == provider || (field.Kind() == reflect.Interface && field.Implements(provider)) {
					t.Fatalf("%s.поле %s несёт адаптер-провайдер %s", name, inspected.Field(i).Name, provider.String())
				}
			}
		}
	}
	check("RecoveryCreateCommand", reflect.TypeOf(RecoveryCreateCommand{}))
	check("CreateRecoveryCaseParams", reflect.TypeOf(port.CreateRecoveryCaseParams{}))
	check("RecoveryOperationFacts", reflect.TypeOf(port.RecoveryOperationFacts{}))
	check("RecoveryCaseRow", reflect.TypeOf(port.RecoveryCaseRow{}))
	// The repository contract: no method signature accepts a provider.
	repoType := reflect.TypeOf((*port.RecoveryRepository)(nil)).Elem()
	for i := 0; i < repoType.NumMethod(); i++ {
		method := repoType.Method(i)
		for j := 0; j < method.Type.NumIn(); j++ {
			for _, provider := range providerTypes {
				if method.Type.In(j) == provider {
					t.Fatalf("метод репозитория %s принимает адаптер-провайдер %s", method.Name, provider.String())
				}
			}
		}
	}
	// The app command signature itself: the command is the ONLY input.
	serviceType := reflect.TypeOf(&Service{})
	method, ok := serviceType.MethodByName("CreateRecoveryCase")
	if !ok {
		t.Fatal("метод CreateRecoveryCase обязан существовать")
	}
	if method.Type.NumIn() != 3 || method.Type.In(2) != reflect.TypeOf(api.RecoveryCreateCommand{}) {
		t.Fatalf("сигнатура команды: %s — только команда, никаких адаптеров", method.Type.String())
	}
	// The api-typed command shape carries no provider adapter either (the
	// transport DTO is SAFE by type).
	check("api.RecoveryCreateCommand", reflect.TypeOf(api.RecoveryCreateCommand{}))
	check("api.RecoveryCaseView", reflect.TypeOf(api.RecoveryCaseView{}))
}
