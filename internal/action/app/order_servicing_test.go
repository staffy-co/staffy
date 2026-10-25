package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.10 step-02: the order servicing application contour (the ONE
// record command with the LIVE proof pins, the idempotent replay BY
// KEY at the same trust state, the lookup outcomes over the fake
// candidate material, the gate refusals and the view read with the
// LIVE derivations)
// ---------------------------------------------------------------------------

// fakeOrderServicingRepo is the in-memory OrderServicingRepository with
// the store semantics: the per-case request_no ladder, the UNIQUE full
// request key at the same trust state (the duplicate append refuses
// with the honest duplicate signal) and the raceHook (the
// parallel-winner simulation of the raced replay tests). The lookup
// candidates and the resolved-operation reads are the CONFIGURED
// material of the test world (the real store reads the recorded
// receipts/rereads/facts — the integration matrix pins the SQL).
type fakeOrderServicingRepo struct {
	requests    []domain.OrderServicingRequest
	nextNumber  map[id.UUID]int64
	candidates  []domain.OrderLookupCandidate
	operations  map[id.UUID]domain.Operation
	raceHook    func(repo *fakeOrderServicingRepo)
	lookupCalls int
}

func newFakeOrderServicingRepo() *fakeOrderServicingRepo {
	return &fakeOrderServicingRepo{nextNumber: map[id.UUID]int64{}, operations: map[id.UUID]domain.Operation{}}
}

func (f *fakeOrderServicingRepo) requestsOf(caseID id.UUID) []domain.OrderServicingRequest {
	out := []domain.OrderServicingRequest{}
	for _, request := range f.requests {
		if request.CaseID == caseID {
			out = append(out, request)
		}
	}
	return out
}

func (f *fakeOrderServicingRepo) ReadCaseServicingRequests(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]domain.OrderServicingRequest, error) {
	return f.requestsOf(caseID), nil
}

func (f *fakeOrderServicingRepo) LookupOrderByIDentifier(_ context.Context, _ tenancy.Context, kind, value string) ([]domain.OrderLookupCandidate, error) {
	f.lookupCalls++
	if !domain.LookupIdentifierKindInDictionary(kind) {
		return []domain.OrderLookupCandidate{}, nil
	}
	out := []domain.OrderLookupCandidate{}
	for _, candidate := range f.candidates {
		out = append(out, candidate)
	}
	return out, nil
}

func (f *fakeOrderServicingRepo) ReadResolvedOperation(_ context.Context, _ tenancy.Context, operationID id.UUID) (domain.Operation, bool, error) {
	operation, ok := f.operations[operationID]
	return operation, ok, nil
}

func (f *fakeOrderServicingRepo) AppendOrderServicingRequest(_ context.Context, p port.AppendOrderServicingRequestParams) (domain.OrderServicingRequest, error) {
	if f.raceHook != nil {
		hook := f.raceHook
		f.raceHook = nil
		hook(f)
	}
	for _, request := range f.requests {
		if request.CaseID == p.CaseID && request.ActorID == p.ActorID && request.Intent == p.Intent &&
			request.IdentifierKind == p.IdentifierKind && request.IdentifierValue == p.IdentifierValue &&
			request.ProofState == p.ProofState {
			return domain.OrderServicingRequest{}, port.ErrOrderServicingRequestDuplicate
		}
	}
	requestNo := f.nextNumber[p.CaseID] + 1
	f.nextNumber[p.CaseID] = requestNo
	request := domain.OrderServicingRequest{
		TenantID: p.Scope.TenantID(), CaseID: p.CaseID, ConversationID: p.ConversationID,
		RequestNo: requestNo, Intent: p.Intent, IdentifierKind: p.IdentifierKind,
		IdentifierValue: p.IdentifierValue, ResolvedOperationID: p.ResolvedOperationID,
		LookupOutcome: p.LookupOutcome, ProofState: p.ProofState, ProofMethod: p.ProofMethod,
		ProofIssuedAt: p.ProofIssuedAt, ProofExpiresAt: p.ProofExpiresAt,
		Disclosure: p.Disclosure, ActorID: p.ActorID, RecordedAt: p.Now,
	}
	f.requests = append(f.requests, request)
	return request, nil
}

// fakeAssuranceSource is the in-memory AssuranceSource: the LIVE 2-3
// derivation view of the contact case (the configurable state, records,
// allowed methods and fallback words). The honest refusal of a foreign
// case answers the casesDomain.ErrNotFound (the adapter mirrors the
// real composition root over the cases API).
type fakeAssuranceSource struct {
	view      casesAPI.AssuranceView
	refuseAll bool
}

func (f fakeAssuranceSource) AssuranceView(_ context.Context, _ identityDomain.Principal, _, _ id.UUID) (casesAPI.AssuranceView, error) {
	if f.refuseAll {
		return casesAPI.AssuranceView{}, casesDomain.ErrNotFound
	}
	return f.view, nil
}

// servicingFixture is the full world of the servicing tests: the
// committed order operation with the APPROVED verification decision
// (the resolved anchor of the lookup), the wired servicing repository
// with the candidate material, the assurance source and the service.
type servicingFixture struct {
	verification *verificationFixture
	servicing    *fakeOrderServicingRepo
	assurance    *fakeAssuranceSource
	facts        *fakeOrderFactRepo
	service      *Service
	operation    domain.Operation
}

// newServicingFixture wires the service over the approved-verification
// world (the unit composition root of the servicing tests): the
// verification fixture first (the committed operation with the receipt
// of ABCP-ORDER-77), then the servicing wiring with the assurance view
// of the chosen level.
func newServicingFixture(t *testing.T, level string) *servicingFixture {
	t.Helper()
	verification := newVerificationFixture(t)
	if _, err := verification.assign(); err != nil {
		t.Fatalf("назначение проверки обязано пройти: %v", err)
	}
	if _, err := verification.acknowledge(1, 1); err != nil {
		t.Fatalf("принятие проверки обязано пройти: %v", err)
	}
	if _, err := verification.approve(1, 1); err != nil {
		t.Fatalf("подтверждение проверки обязано пройти: %v", err)
	}
	servicing := newFakeOrderServicingRepo()
	facts := newFakeOrderFactRepo()
	// The candidate material of the test world: the committed receipt of
	// the fixture operation carries the external ABCP Order ID (the same
	// material the REAL store finds over action.action_receipts).
	servicing.candidates = []domain.OrderLookupCandidate{{
		OperationID: verification.operation.OperationID,
		CaseID:      previewCaseID,
		Sources:     []string{domain.OrderFactSourceSystemReceipt},
	}}
	servicing.operations[verification.operation.OperationID] = verification.operation
	issued := previewNow.Add(-10 * time.Minute)
	expires := previewNow.Add(3 * time.Hour)
	assurance := &fakeAssuranceSource{view: casesAPI.AssuranceView{
		State: casesDomain.AssuranceState{
			IdentityLevel:     level,
			IdentityMethod:    "known_number_callback",
			IdentityExpiresAt: expires,
		},
		Records: []casesDomain.VerificationRecord{{
			Kind: casesDomain.VerificationKindIdentity, Method: "known_number_callback",
			Outcome: casesDomain.VerificationPassed, AssuranceLevel: level,
			IssuedAt: issued, ExpiresAt: expires, CreatedAt: issued,
		}},
		AllowedMethods:  []string{"перезвонок на известный номер", "ответ на письмо с известного адреса"},
		FallbackOptions: []string{"Повторите проверку разрешённым способом.", "Продолжите обслуживание без защищённых данных.", "Передайте разговор человеку."},
	}}
	service := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Review: true}},
		acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs()).
		WithPreviews(newFakePreviewRepo(), verification.world.control, verification.material, verificationMembers{},
			fakeEntitlement{status: verification.world.admission}, fakeReadiness{stage: verification.world.stage, instances: verification.world.instances},
			orderConnectors{state: verification.world.capState, contractVersion: verification.world.contract},
			fakeRouteFacts{profileVersion: verification.world.profile}, &fakeIDs{}).
		WithAuthorization(newFakeConfirmationRepo(), verification.ops).
		WithDispatch(verification.dispatch, nil).
		WithOrderVerifications(verification.repo).
		WithOrderCorrections(newFakeCorrectionRepo()).
		WithOrderSupplierFacts(facts).
		WithOrderServicing(servicing, assurance)
	return &servicingFixture{verification: verification, servicing: servicing, assurance: assurance, facts: facts, service: service, operation: verification.operation}
}

// record builds the passing record command of the fixture world.
func (f *servicingFixture) record(intent, kind, value string) (OrderServicingOutcome, error) {
	return f.service.RecordOrderServicingRequest(context.Background(), RecordOrderServicingRequestCommand{
		Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
		CaseID: previewCaseID, Intent: intent, IdentifierKind: kind,
		IdentifierValue: value, Now: previewNow.Add(time.Hour),
	})
}

// view reads the servicing view of the fixture world.
func (f *servicingFixture) view() (domain.OrderServicingView, error) {
	return f.service.ReadOrderServicingView(context.Background(),
		identityDomain.Principal{ID: verificationOwner}, testTenant, previewCaseID, previewNow.Add(2*time.Hour))
}

// TestRecordOrderServicingRequestUniqueMatch pins the happy verified
// path BY NAME: the lookup BY KEY over the recorded receipt material
// answers unique_match with the resolved anchor, the LIVE proof pins
// copy into the request (the state, the method, the issued/expiry
// times — AC-4), the disclosure opens (verified ONLY) and the record
// command never claims a result (AC-6).
func TestRecordOrderServicingRequestUniqueMatch(t *testing.T) {
	fixture := newServicingFixture(t, casesDomain.AssuranceVerified)
	outcome, err := fixture.record(domain.ServicingIntentOrderStatus, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-77")
	if err != nil {
		t.Fatalf("запись обязана пройти: %v", err)
	}
	request := outcome.Request
	if request.LookupOutcome != domain.LookupOutcomeUnique {
		t.Fatalf("исход поиска: %s", request.LookupOutcome)
	}
	if request.ResolvedOperationID != fixture.operation.OperationID {
		t.Fatalf("якорь операции: %v", request.ResolvedOperationID)
	}
	if request.ProofState != domain.ProofStateVerified || request.ProofMethod != "known_number_callback" {
		t.Fatalf("proof-пины: %s %s", request.ProofState, request.ProofMethod)
	}
	if request.ProofIssuedAt.IsZero() || request.ProofExpiresAt.IsZero() {
		t.Fatalf("proof-пины времён: %v %v", request.ProofIssuedAt, request.ProofExpiresAt)
	}
	if request.Disclosure != domain.DisclosureAllowed {
		t.Fatalf("раскрытие: %s", request.Disclosure)
	}
	if request.RequestNo != 1 || request.ActorID != verificationManager {
		t.Fatalf("лестница/актёр: %d %v", request.RequestNo, request.ActorID)
	}
	if outcome.Replay {
		t.Fatal("первая запись — не реплей")
	}
	if !strings.Contains(outcome.SourceWord, "не утверждает результат") {
		t.Fatalf("слово записи: %s", outcome.SourceWord)
	}
	// The FR-62 structural pin: the order operation count NEVER grows
	// (the write path is the ONE request append).
	operationsScope, _ := tenancy.New(verificationManager, testTenant)
	operations, err := fixture.verification.ops.ReadCaseOperations(context.Background(), operationsScope, previewCaseID)
	if err != nil {
		t.Fatalf("чтение операций: %v", err)
	}
	if len(operations) != 1 {
		t.Fatalf("операций заказа обязана остаться одна: %d", len(operations))
	}
}

// TestRecordOrderServicingRequestGateDenied pins the AC-1/AC-6 gate: a
// NOT-verified assurance (none/basic/step_up) still records the request
// (the append fixes the ask) with the honest proof pins and the denied
// disclosure — the lookup outcome stays in the ROW (the operator's
// review right) but NEVER rides the client words.
func TestRecordOrderServicingRequestGateDenied(t *testing.T) {
	for _, level := range []string{casesDomain.AssuranceNone, casesDomain.AssuranceBasic} {
		fixture := newServicingFixture(t, level)
		outcome, err := fixture.record(domain.ServicingIntentOrderStatus, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-77")
		if err != nil {
			t.Fatalf("уровень %s: запись обязана пройти (AC-6 — фиксирует запрос): %v", level, err)
		}
		request := outcome.Request
		if request.Disclosure != domain.DisclosureDenied {
			t.Fatalf("уровень %s: раскрытие обязано быть denied: %s", level, request.Disclosure)
		}
		wantState := domain.ProofStateNone
		if level == casesDomain.AssuranceBasic {
			wantState = domain.ProofStateBasic
		}
		if request.ProofState != wantState {
			t.Fatalf("уровень %s: proof-состояние %s, а не %s", level, request.ProofState, wantState)
		}
		// The row still carries the resolved anchor and the outcome (the
		// operator's panel material — the review right of the operator).
		if request.LookupOutcome != domain.LookupOutcomeUnique {
			t.Fatalf("уровень %s: исход поиска в строке: %s", level, request.LookupOutcome)
		}
		// The client words of the denied view carry the gate word ONLY.
		view, err := fixture.view()
		if err != nil {
			t.Fatalf("уровень %s: чтение панели: %v", level, err)
		}
		if len(view.Requests) != 1 {
			t.Fatalf("уровень %s: запросов %d", level, len(view.Requests))
		}
		joined := strings.Join(view.Requests[0].ClientWords, " ")
		if !strings.Contains(joined, "требует проверки личности") {
			t.Fatalf("уровень %s: слово гейта отсутствует: %v", level, view.Requests[0].ClientWords)
		}
		if strings.Contains(joined, domain.LookupOutcomeWords[domain.LookupOutcomeUnique]) ||
			strings.Contains(joined, "заказ клиента создан") {
			t.Fatalf("уровень %s: материал заказа утёк в отказ (AC-1): %v", level, view.Requests[0].ClientWords)
		}
	}
}

// TestRecordOrderServicingRequestStepUp pins the step_up window word:
// the threshold flag of the live derivation answers the step_up proof
// state (the checks are suspended — the disclosure refuses with the
// gate word).
func TestRecordOrderServicingRequestStepUp(t *testing.T) {
	fixture := newServicingFixture(t, casesDomain.AssuranceVerified)
	view := fixture.assurance.view
	view.State.StepUp = true
	view.State.StepUpFailures = 3
	fixture.assurance.view = view
	outcome, err := fixture.record(domain.ServicingIntentCancel, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-77")
	if err != nil {
		t.Fatalf("запись обязана пройти: %v", err)
	}
	if outcome.Request.ProofState != domain.ProofStateStepUp {
		t.Fatalf("proof-состояние: %s", outcome.Request.ProofState)
	}
	if outcome.Request.Disclosure != domain.DisclosureDenied {
		t.Fatalf("раскрытие: %s", outcome.Request.Disclosure)
	}
}

// TestRecordOrderServicingRequestReplayAndTrustLadder pins the
// idempotency BY KEY and the trust ladder: the repeated command at the
// SAME trust state replays the SAVED request (one row); the CHANGED
// trust state (none → verified) is a NEW request (two rows — the
// ladder stays visible).
func TestRecordOrderServicingRequestReplayAndTrustLadder(t *testing.T) {
	fixture := newServicingFixture(t, casesDomain.AssuranceNone)
	first, err := fixture.record(domain.ServicingIntentOrderStatus, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-77")
	if err != nil {
		t.Fatalf("первая запись: %v", err)
	}
	repeat, err := fixture.record(domain.ServicingIntentOrderStatus, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-77")
	if err != nil {
		t.Fatalf("повтор: %v", err)
	}
	if !repeat.Replay || repeat.Request.RequestNo != first.Request.RequestNo {
		t.Fatalf("повтор обязан реплеить первую запись: %+v", repeat.Request)
	}
	if len(fixture.servicing.requests) != 1 {
		t.Fatalf("строк обязана быть одна: %d", len(fixture.servicing.requests))
	}
	// The trust ladder: the verified assurance CHANGES the key — a NEW
	// request lands, BOTH rows live in the history.
	fixture.assurance.view.State.IdentityLevel = casesDomain.AssuranceVerified
	verified, err := fixture.record(domain.ServicingIntentOrderStatus, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-77")
	if err != nil {
		t.Fatalf("запись после проверки: %v", err)
	}
	if verified.Replay || verified.Request.RequestNo != 2 {
		t.Fatalf("изменившееся доверие = НОВАЯ запись: %+v", verified.Request)
	}
	if verified.Request.Disclosure != domain.DisclosureAllowed {
		t.Fatalf("раскрытие после проверки: %s", verified.Request.Disclosure)
	}
	if len(fixture.servicing.requests) != 2 {
		t.Fatalf("строк обязано быть две (лестница доверия видна): %d", len(fixture.servicing.requests))
	}
}

// TestRecordOrderServicingRequestRacedReplay pins the raced twin of
// the same command: the parallel winner lands first, the loser's
// duplicate append refuses with the honest duplicate signal and the
// re-read answers the SAVED request — never a masked write, never a
// second row.
func TestRecordOrderServicingRequestRacedReplay(t *testing.T) {
	fixture := newServicingFixture(t, casesDomain.AssuranceVerified)
	fixture.servicing.raceHook = func(repo *fakeOrderServicingRepo) {
		requestNo := int64(1)
		repo.requests = append(repo.requests, domain.OrderServicingRequest{
			TenantID: testTenant, CaseID: previewCaseID, RequestNo: requestNo,
			Intent: domain.ServicingIntentOrderStatus, IdentifierKind: domain.LookupKindABCPClientOrderID,
			IdentifierValue: "ABCP-ORDER-77", ResolvedOperationID: fixture.operation.OperationID,
			LookupOutcome: domain.LookupOutcomeUnique, ProofState: domain.ProofStateVerified,
			ProofMethod: "known_number_callback", Disclosure: domain.DisclosureAllowed,
			ActorID: verificationManager, RecordedAt: previewNow.Add(time.Hour),
		})
		repo.nextNumber[previewCaseID] = requestNo
	}
	outcome, err := fixture.record(domain.ServicingIntentOrderStatus, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-77")
	if err != nil {
		t.Fatalf("проигравший гонку обязан ответить реплеем: %v", err)
	}
	if !outcome.Replay || outcome.Request.RequestNo != 1 {
		t.Fatalf("реплей гонки: %+v", outcome.Request)
	}
	if len(fixture.servicing.requests) != 1 {
		t.Fatalf("строк обязана быть одна: %d", len(fixture.servicing.requests))
	}
}

// TestRecordOrderServicingRequestLookupOutcomes pins the three lookup
// outcomes BY NAME over the fake candidate material: the empty list
// answers not_found WITHOUT an anchor (the generic word — the client
// words of the verified miss ride it), the two DIFFERENT operations
// answer ambiguous WITHOUT an anchor and WITHOUT a list.
func TestRecordOrderServicingRequestLookupOutcomes(t *testing.T) {
	fixture := newServicingFixture(t, casesDomain.AssuranceVerified)
	fixture.servicing.candidates = nil
	miss, err := fixture.record(domain.ServicingIntentOrderStatus, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-404")
	if err != nil {
		t.Fatalf("промах обязан записаться: %v", err)
	}
	if miss.Request.LookupOutcome != domain.LookupOutcomeNone || miss.Request.ResolvedOperationID != id.UUID("") {
		t.Fatalf("промах: %s %v", miss.Request.LookupOutcome, miss.Request.ResolvedOperationID)
	}
	if miss.Request.Disclosure != domain.DisclosureAllowed {
		t.Fatalf("промах: раскрытие %s (гейт — про доверие, не про матч)", miss.Request.Disclosure)
	}
	// The client words of the verified miss: the generic outcome word +
	// the intent word (never a disclosure of another client's orders).
	view, err := fixture.view()
	if err != nil {
		t.Fatalf("панель: %v", err)
	}
	if len(view.Requests) != 1 || len(view.Requests[0].ClientWords) != 2 {
		t.Fatalf("слова промаха: %v", view.Requests[0].ClientWords)
	}
	if view.Requests[0].ClientWords[0] != domain.ServicingLookupNotFoundWord {
		t.Fatalf("первое слово промаха: %s", view.Requests[0].ClientWords[0])
	}
	// The ambiguous outcome: TWO different operations, NO anchor, NO list.
	other := id.MustParse("01994f64-6c00-7000-8000-0000000000e1")
	fixture.servicing.candidates = []domain.OrderLookupCandidate{
		{OperationID: fixture.operation.OperationID, CaseID: previewCaseID, Sources: []string{domain.OrderFactSourceSystemReceipt}},
		{OperationID: other, CaseID: previewCaseID, Sources: []string{domain.OrderFactSourceManagerRecord}},
	}
	ambiguous, err := fixture.record(domain.ServicingIntentClarification, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-DUP")
	if err != nil {
		t.Fatalf("неоднозначность обязана записаться: %v", err)
	}
	if ambiguous.Request.LookupOutcome != domain.LookupOutcomeMany || ambiguous.Request.ResolvedOperationID != id.UUID("") {
		t.Fatalf("неоднозначность: %s %v", ambiguous.Request.LookupOutcome, ambiguous.Request.ResolvedOperationID)
	}
	view, err = fixture.view()
	if err != nil {
		t.Fatalf("панель: %v", err)
	}
	// The ambiguous request words carry the generic ambiguous word —
	// NEVER the list of the orders.
	var ambiguousWords []string
	for _, requestView := range view.Requests {
		if requestView.Request.RequestNo == ambiguous.Request.RequestNo {
			ambiguousWords = requestView.ClientWords
		}
	}
	joined := strings.Join(ambiguousWords, " ")
	if !strings.Contains(joined, domain.ServicingLookupAmbiguousWord) || strings.Contains(joined, "ABCP-ORDER-77") {
		t.Fatalf("слова неоднозначности: %v", ambiguousWords)
	}
}

// TestRecordOrderServicingRequestSupplierKind pins the supplier kind
// lookup: the supplier order ID of the recorded facts anchors the
// operation (the source pin BY NAME — the human facts of 4-9; the ABCP
// ID and the supplier ID are DIFFERENT TYPES).
func TestRecordOrderServicingRequestSupplierKind(t *testing.T) {
	fixture := newServicingFixture(t, casesDomain.AssuranceVerified)
	fixture.servicing.candidates = []domain.OrderLookupCandidate{{
		OperationID: fixture.operation.OperationID,
		CaseID:      previewCaseID,
		Sources:     []string{domain.OrderFactSourceManagerRecord},
	}}
	outcome, err := fixture.record(domain.ServicingIntentCancel, domain.LookupKindSupplierOrderID, "SUP-77")
	if err != nil {
		t.Fatalf("запись: %v", err)
	}
	if outcome.Request.LookupOutcome != domain.LookupOutcomeUnique || outcome.Request.ResolvedOperationID != fixture.operation.OperationID {
		t.Fatalf("поставщик: %s %v", outcome.Request.LookupOutcome, outcome.Request.ResolvedOperationID)
	}
	view, err := fixture.view()
	if err != nil {
		t.Fatalf("панель: %v", err)
	}
	// The source pins of the resolved match ride the view BY NAME.
	found := false
	for _, requestView := range view.Requests {
		if requestView.Request.RequestNo == outcome.Request.RequestNo {
			found = true
			if len(requestView.Sources) != 1 || requestView.Sources[0] != domain.OrderFactSourceManagerRecord {
				t.Fatalf("источники матча: %v", requestView.Sources)
			}
			// The cancel intent derives the LIVE four-state view over the
			// projection of the resolved operation (the fixture carries NO
			// supplier facts — the fourth state speaks «не определена»).
			if requestView.Cancel == nil {
				t.Fatal("клиентские слова отмены без четырёх состояний")
			}
			if requestView.Cancel.SupplierTouched || !strings.Contains(requestView.Cancel.NotEditableWord, "не определена") {
				t.Fatalf("четвёртое состояние: %+v", requestView.Cancel)
			}
		}
	}
	if !found {
		t.Fatal("запись поставщика не найдена в панели")
	}
}

// TestRecordOrderServicingRequestRefusals pins the honest refusals BY
// NAME: the out-of-dictionary intent and kind (OQ-16 — the phone kind
// does NOT exist), the empty value, the missing review scope (403 with
// the scope words), the foreign case (404 — the RLS blindness mirror)
// and the unwired contour (the honest words, never a nil panic).
func TestRecordOrderServicingRequestRefusals(t *testing.T) {
	fixture := newServicingFixture(t, casesDomain.AssuranceVerified)
	cases := []struct {
		name        string
		intent      string
		kind        string
		value       string
		wantContain string
	}{
		{"интент вне словаря", "urgency", domain.LookupKindABCPClientOrderID, "ABCP-ORDER-77", "словаря"},
		{"тип телефона не существует", domain.ServicingIntentOrderStatus, "phone_number", "+7-999", "OQ-16"},
		{"пустое значение", domain.ServicingIntentHuman, domain.LookupKindSupplierOrderID, "  ", "обязательно"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := fixture.record(testCase.intent, testCase.kind, testCase.value)
			if err == nil || !strings.Contains(err.Error(), testCase.wantContain) {
				t.Fatalf("ожидался отказ со словом %q: %v", testCase.wantContain, err)
			}
		})
	}
	// The scope refusal: an actor WITHOUT the review scope reads the 403
	// words BEFORE any material.
	noScope := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{}},
		acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs()).
		WithPreviews(newFakePreviewRepo(), fixture.verification.world.control, fixture.verification.material, verificationMembers{},
			fakeEntitlement{status: fixture.verification.world.admission}, fakeReadiness{stage: fixture.verification.world.stage, instances: fixture.verification.world.instances},
			orderConnectors{state: fixture.verification.world.capState, contractVersion: fixture.verification.world.contract},
			fakeRouteFacts{profileVersion: fixture.verification.world.profile}, &fakeIDs{}).
		WithAuthorization(newFakeConfirmationRepo(), fixture.verification.ops).
		WithDispatch(fixture.verification.dispatch, nil).
		WithOrderVerifications(fixture.verification.repo).
		WithOrderCorrections(newFakeCorrectionRepo()).
		WithOrderSupplierFacts(fixture.facts).
		WithOrderServicing(fixture.servicing, fixture.assurance)
	_, err := noScope.RecordOrderServicingRequest(context.Background(), RecordOrderServicingRequestCommand{
		Actor: identityDomain.Principal{ID: verificationNoScope}, TenantID: testTenant,
		CaseID: previewCaseID, Intent: domain.ServicingIntentOrderStatus,
		IdentifierKind: domain.LookupKindABCPClientOrderID, IdentifierValue: "ABCP-ORDER-77", Now: previewNow,
	})
	if err == nil || !strings.Contains(err.Error(), "полномочия review") {
		t.Fatalf("отказ прав: %v", err)
	}
	// The foreign case refusal: the assurance read answers the honest
	// 404 word (the RLS blindness mirror — never a case enumeration).
	foreign := newServicingFixture(t, casesDomain.AssuranceVerified)
	foreign.assurance.refuseAll = true
	_, err = foreign.record(domain.ServicingIntentOrderStatus, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-77")
	if !errors.Is(err, domain.ErrOrderServicingNoCase) {
		t.Fatalf("чужой кейс: %v", err)
	}
	// The unwired contour refusal: the honest words, never a nil panic.
	unwired := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Review: true}},
		acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs())
	_, err = unwired.RecordOrderServicingRequest(context.Background(), RecordOrderServicingRequestCommand{
		Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
		CaseID: previewCaseID, Intent: domain.ServicingIntentOrderStatus,
		IdentifierKind: domain.LookupKindABCPClientOrderID, IdentifierValue: "ABCP-ORDER-77", Now: previewNow,
	})
	if err == nil || !strings.Contains(err.Error(), "не подключён") {
		t.Fatalf("неподключённый контур: %v", err)
	}
}

// TestReadOrderServicingViewAllowed pins the LIVE derivations of the
// view: the verified unique-match request carries the client words of
// the LIVE 4-9 projection (the fixture world: the committed receipt —
// «заказ клиента создан») plus the intent boundary word; the boundary
// words of the contour ride the view (OQ-16/AG-4/OQ-1/OQ-2/AC-6).
func TestReadOrderServicingViewAllowed(t *testing.T) {
	fixture := newServicingFixture(t, casesDomain.AssuranceVerified)
	if _, err := fixture.record(domain.ServicingIntentOrderStatus, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-77"); err != nil {
		t.Fatalf("запись: %v", err)
	}
	view, err := fixture.view()
	if err != nil {
		t.Fatalf("панель: %v", err)
	}
	if len(view.Requests) != 1 {
		t.Fatalf("запросов: %d", len(view.Requests))
	}
	requestView := view.Requests[0]
	// The LIVE consistency pin: the servicing client words EQUAL the LIVE
	// 4-9 projection words of the same service + the intent boundary word
	// (the re-derivation at read time — never a stored status).
	projection, err := fixture.service.ReadOrderStatusView(context.Background(),
		identityDomain.Principal{ID: verificationOwner}, testTenant, previewCaseID, previewNow.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("живая проекция: %v", err)
	}
	want := append(append([]string{}, projection.ClientWords...), domain.ServicingIntentWords[domain.ServicingIntentOrderStatus])
	if len(requestView.ClientWords) != len(want) {
		t.Fatalf("слова живут не проекцией+интентом: %v ≠ %v", requestView.ClientWords, want)
	}
	for i := range want {
		if requestView.ClientWords[i] != want[i] {
			t.Fatalf("слово %d: %s ≠ %s", i, requestView.ClientWords[i], want[i])
		}
	}
	if requestView.IntentShortWord != "статус заказа" || requestView.KindShortWord != "номер заказа клиента в ABCP" {
		t.Fatalf("короткие слова формы: %s / %s", requestView.IntentShortWord, requestView.KindShortWord)
	}
	if !strings.Contains(requestView.KindSourceWord, "раздельные источники") {
		t.Fatalf("слово источника типа: %s", requestView.KindSourceWord)
	}
	if !strings.Contains(view.PhoneBoundaryWord, "OQ-16") || !strings.Contains(view.ConnectorBoundaryWord, "AG-4") {
		t.Fatalf("слова границ: %s / %s", view.PhoneBoundaryWord, view.ConnectorBoundaryWord)
	}
	if !strings.Contains(view.NoDeliveryWord, "не отправляются") || !strings.Contains(view.RequestRecordedWord, "не утверждает результат") {
		t.Fatalf("слова контура: %s / %s", view.NoDeliveryWord, view.RequestRecordedWord)
	}
}

// TestReadOrderServicingViewCancelFourStates pins the LIVE cancel
// feasibility derivation of the view: the supplier facts of the
// resolved operation flip the fourth state («затронул поставщика —
// человеку»); the derivation rides the cancel intent ONLY.
func TestReadOrderServicingViewCancelFourStates(t *testing.T) {
	fixture := newServicingFixture(t, casesDomain.AssuranceVerified)
	if _, err := fixture.record(domain.ServicingIntentCancel, domain.LookupKindABCPClientOrderID, "ABCP-ORDER-77"); err != nil {
		t.Fatalf("запись отмены: %v", err)
	}
	// A supplier fact over the resolved operation flips the touch (the
	// human receipt of the 4-9 contour — the append-only fact ladder).
	factScope, _ := tenancy.New(verificationManager, testTenant)
	if _, err := fixture.facts.AppendOrderSupplierFact(context.Background(), port.AppendOrderSupplierFactParams{
		Scope: factScope, CaseID: previewCaseID,
		WorkItemID: fixture.operation.WorkItemID, OperationID: fixture.operation.OperationID,
		Kind: domain.OrderSupplierFactFormed, SupplierOrderID: "SUP-77",
		HappenedAt: previewNow.Add(30 * time.Minute), ActorID: verificationManager,
		Now: previewNow.Add(31 * time.Minute),
	}); err != nil {
		t.Fatalf("факт поставщика: %v", err)
	}
	view, err := fixture.view()
	if err != nil {
		t.Fatalf("панель: %v", err)
	}
	if len(view.Requests) != 1 || view.Requests[0].Cancel == nil {
		t.Fatalf("слово отмены без деривации: %+v", view.Requests)
	}
	cancel := view.Requests[0].Cancel
	if !cancel.SupplierTouched || !strings.Contains(cancel.NotEditableWord, "затронул поставщика") {
		t.Fatalf("четвёртое состояние с фактом: %+v", cancel)
	}
	if !strings.Contains(cancel.PreparedActionWord, "OQ-3") || !strings.Contains(cancel.ConfirmedWord, "не обещается") {
		t.Fatalf("второе/третье состояния: %+v", cancel)
	}
	if !strings.Contains(cancel.ManualContourWord, "НЕ считается отменённым") {
		t.Fatalf("слово ручного контура: %s", cancel.ManualContourWord)
	}
}
