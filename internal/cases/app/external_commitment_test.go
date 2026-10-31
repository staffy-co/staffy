package app

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.3: the unit matrix of the external commitment commands
// ---------------------------------------------------------------------------
//
// The fake repository embeds the Repository interface (the unimplemented
// reads panic honestly — a unit test never touches them) and carries the
// COUNTERS: the order-of-gates proofs read the counters (a refused
// readiness never opens the enqueue transaction, a refused approval never
// writes). The fake pending implements the SAME selection semantics as the
// store query (the decided, the bound-exhausted and the guarantee-free
// unknowns are structurally unselectable) — the integration matrix of
// step-04 pins the SQL half against the live PG.

// externalFixture is the material of one unit test: the identifiers, the
// live case material (the work item, the ownership term, the policy
// outcomes), the envelope versions, the approvals, the dispatch history and
// the pending queue.
type externalFixture struct {
	tenant     id.UUID
	caseID     id.UUID
	workItemID id.UUID
	owner      id.UUID
	actor      domain.Principal
	extID      id.UUID
	pinCommit  id.UUID
	now        time.Time
	validUntil time.Time
	current    casesDomain.WorkItemCurrent
	terms      []casesDomain.OwnershipTerm
	versions   []casesDomain.ExternalCommitment
	commit29   []casesDomain.CommitmentVersion
	approvals  []casesDomain.ExternalApproval
	events     []casesDomain.ExternalDispatchEvent
	pending    []casesDomain.OutboxEvent
	ownerEpoch int64
	maxVersion int64
	hmacKey    []byte
	queued     map[string]bool
}

func newExternalFixture() *externalFixture {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	// The identifiers are MINTED as valid UUIDv7 — tenancy.New validates
	// the version and the variant bits of every scope anchor.
	tenant, _ := id.NewV7(now)
	caseID, _ := id.NewV7(now.Add(1 * time.Second))
	workItemID, _ := id.NewV7(now.Add(2 * time.Second))
	owner, _ := id.NewV7(now.Add(3 * time.Second))
	extID, _ := id.NewV7(now.Add(4 * time.Second))
	pinCommit, _ := id.NewV7(now.Add(5 * time.Second))
	f := &externalFixture{
		tenant: tenant, caseID: caseID, workItemID: workItemID, owner: owner,
		actor: domain.Principal{ID: owner, StepUpVerified: true},
		extID: extID, pinCommit: pinCommit,
		now: now, validUntil: now.Add(24 * time.Hour),
		current: casesDomain.WorkItemCurrent{WorkItemID: workItemID, CaseID: caseID, State: casesDomain.WorkItemStateInWork, Version: 1},
		terms: []casesDomain.OwnershipTerm{{
			TenantID: tenant, CaseID: caseID, Epoch: 1,
			OwnerKind: casesDomain.OwnerKindHuman, OwnerRef: owner,
		}},
		ownerEpoch: 1, maxVersion: 1, hmacKey: []byte("k"),
		queued: map[string]bool{},
	}
	return f
}

// version builds one envelope version of the fixture with the honest
// content token of the fixture key.
func (f *externalFixture) version(purpose string) casesDomain.ExternalCommitment {
	version := casesDomain.ExternalCommitment{
		TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1,
		Purpose: purpose, RecipientKind: casesDomain.ExternalRecipientClient,
		Channel: "phone", ContentText: "Материал конверта",
		ValidUntil: f.validUntil, ExpectedNextStep: "звонок клиента",
		AckRequirement: casesDomain.ExternalAckNotRequired,
		OwnerKind:      casesDomain.OwnerKindHuman, OwnerRef: f.owner, OwnerEpoch: f.ownerEpoch,
	}
	version.ContentToken = casesDomain.ExternalCommitmentContentToken(f.hmacKey, version)
	return version
}

// enqueue builds the queue evidence: the outbox record with the version pin
// dedup key.
func (f *externalFixture) enqueue() casesDomain.OutboxEvent {
	return casesDomain.OutboxEvent{
		TenantID: f.tenant, EventID: f.extID,
		EventType:     casesDomain.OutboxEventExternalCommitmentQueued,
		SchemaVersion: casesDomain.OutboxEventExternalCommitmentQueuedV1,
		AggregateID:   f.caseID, OccurredAt: f.now,
		Payload: []byte(`{"tenant_id":"` + f.tenant.String() + `","case_id":"` + f.caseID.String() +
			`","ext_commitment_id":"` + f.extID.String() + `","version_no":1,"purpose":"promise",` +
			`"recipient_kind":"client","channel":"phone","idempotency_key":"k","content_digest":"d"}`),
		DedupKey: casesDomain.DedupKeyExternalCommitmentQueued(f.extID, 1),
	}
}

// fakeExternalRepo embeds the Repository interface and implements the
// 3.3 surface with the counters (the gate-order proofs).
type fakeExternalRepo struct {
	casesPort.Repository
	f *externalFixture

	createCalls  int
	approveCalls int
	enqueueCalls int
	gasCalls     []string // the reason words of the gasps
	outcomeCalls int
	relayCalls   int
	pendingCalls int

	approveExists bool // the UNIQUE answer: the approval row already present
	outcomeAppend bool // the raced-twin answer of the outcome append
	enqueueFail   error
}

func (r *fakeExternalRepo) ReadCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesDomain.CaseDetail, error) {
	return casesDomain.CaseDetail{Case: casesDomain.Case{TenantID: r.f.tenant, CaseID: caseID},
		Conversations: []casesDomain.Conversation{{
			TenantID: r.f.tenant, CaseID: caseID, ConversationID: r.f.extID,
			ConnectionID: r.f.workItemID, Modality: casesDomain.ModalityText, State: "open",
		}},
		WorkItems: []casesDomain.WorkItemCurrent{r.f.current}}, nil
}

func (r *fakeExternalRepo) ReadWorkItemCurrents(ctx context.Context, scope tenancy.Context, caseID id.UUID, ids []id.UUID) (map[id.UUID]casesDomain.WorkItemCurrent, error) {
	out := map[id.UUID]casesDomain.WorkItemCurrent{}
	for _, wanted := range ids {
		if wanted == r.f.workItemID {
			out[wanted] = r.f.current
		}
	}
	return out, nil
}

func (r *fakeExternalRepo) ReadOwnershipMaterial(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.OwnershipTerm, []casesDomain.TransferRequest, []casesDomain.TransferResolution, error) {
	return r.f.terms, nil, nil, nil
}

func (r *fakeExternalRepo) ReadPolicyOutcomes(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.PolicyOutcome, error) {
	return nil, nil
}

func (r *fakeExternalRepo) ReadExternalCommitments(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ExternalCommitment, error) {
	return r.f.versions, nil
}

func (r *fakeExternalRepo) ReadExternalApprovals(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ExternalApproval, error) {
	return r.f.approvals, nil
}

func (r *fakeExternalRepo) ReadExternalDispatchEvents(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ExternalDispatchEvent, error) {
	return r.f.events, nil
}

// ReadExternalQueuedVersions mirrors the store read of the queue evidence:
// the dedup keys of the queued map ARE the version pins
// (external_commitment.queued:<ext>:<version> — the honest derivation of
// the fixture world).
func (r *fakeExternalRepo) ReadExternalQueuedVersions(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesPort.ExternalQueuedVersion, error) {
	pins := []casesPort.ExternalQueuedVersion{}
	for key := range r.f.queued {
		parts := strings.Split(key, ":")
		if len(parts) != 3 {
			continue
		}
		extID, err := id.Parse(parts[1])
		if err != nil {
			continue
		}
		versionNo, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			continue
		}
		pins = append(pins, casesPort.ExternalQueuedVersion{ExtCommitmentID: extID, VersionNo: versionNo})
	}
	return pins, nil
}

func (r *fakeExternalRepo) ReadCommitmentVersionsByID(ctx context.Context, scope tenancy.Context, commitmentID id.UUID) ([]casesDomain.CommitmentVersion, error) {
	return r.f.commit29, nil
}

func (r *fakeExternalRepo) ReadCommitmentKey(ctx context.Context, scope tenancy.Context, now time.Time) ([]byte, error) {
	return r.f.hmacKey, nil
}

func (r *fakeExternalRepo) CreateExternalCommitment(ctx context.Context, p casesPort.ExternalCommitmentCreateParams) error {
	r.createCalls++
	if r.f.versions == nil {
		r.f.versions = []casesDomain.ExternalCommitment{}
	}
	version := casesDomain.ExternalCommitment{
		TenantID: p.Scope.TenantID(), CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		ExtCommitmentID: p.ExtCommitmentID, VersionNo: p.VersionNo,
		Purpose: p.Purpose, RecipientKind: p.RecipientKind, Channel: p.Channel,
		ContentText: p.ContentText, ValidUntil: p.ValidUntil,
		ExpectedNextStep: p.ExpectedNextStep, AckRequirement: p.AckRequirement,
		Pin: p.Pin, OwnerKind: p.OwnerKind, OwnerRef: p.OwnerRef, OwnerEpoch: p.OwnerEpoch,
		DecisionNo: p.DecisionNo, ProfileVersion: p.ProfileVersion, PolicyOutcomeNo: p.PolicyOutcomeNo,
		ContentToken: p.ContentToken, ActorID: p.ActorID, CreatedAt: p.Now,
	}
	r.f.versions = append(r.f.versions, version)
	if p.VersionNo > r.f.maxVersion {
		r.f.maxVersion = p.VersionNo
	}
	return nil
}

func (r *fakeExternalRepo) ApproveExternalCommitment(ctx context.Context, p casesPort.ExternalApprovalParams) (bool, error) {
	r.approveCalls++
	if r.approveExists {
		return false, nil
	}
	r.approveExists = true
	r.f.approvals = append(r.f.approvals, casesDomain.ExternalApproval{
		TenantID: p.Scope.TenantID(), CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		ExtCommitmentID: p.ExtCommitmentID, VersionNo: p.VersionNo,
		ApprovalID: p.ApprovalID, ApproverID: p.ApproverID, StepUpVerified: p.StepUpVerified,
		CreatedAt: p.Now,
	})
	return true, nil
}

func (r *fakeExternalRepo) EnqueueExternalCommitment(ctx context.Context, p casesPort.ExternalEnqueueParams) error {
	r.enqueueCalls++
	if r.enqueueFail != nil {
		return r.enqueueFail
	}
	r.f.queued[casesDomain.DedupKeyExternalCommitmentQueued(p.ExtCommitmentID, p.VersionNo)] = true
	r.f.pending = append(r.f.pending, r.f.enqueue())
	return nil
}

// PendingExternalCommitments mirrors the store selection semantics: the
// decided (attempt_submitted, cancelled_stale), the bound-exhausted and
// the guarantee-free last unknowns are structurally unselectable.
func (r *fakeExternalRepo) PendingExternalCommitments(ctx context.Context, scope tenancy.Context, limit int) ([]casesPort.ExternalDispatchCandidate, error) {
	r.pendingCalls++
	candidates := []casesPort.ExternalDispatchCandidate{}
	for _, record := range r.f.pending {
		pending := true
		attemptUsed := int64(0)
		var last *casesDomain.ExternalDispatchEvent
		for i := range r.f.events {
			event := r.f.events[i]
			switch event.Kind {
			case casesDomain.ExternalDispatchAttemptSubmitted, casesDomain.ExternalDispatchCancelledStale:
				pending = false
			case casesDomain.ExternalDispatchAttemptFailed, casesDomain.ExternalDispatchAttemptUnknown:
				if event.AttemptNo > attemptUsed {
					attemptUsed = event.AttemptNo
				}
			}
			if last == nil || event.EventNo >= last.EventNo {
				last = &r.f.events[i]
			}
		}
		if attemptUsed >= casesDomain.MaxDispatchAttempts {
			pending = false
		}
		if last != nil && last.Kind == casesDomain.ExternalDispatchAttemptUnknown && !last.IdempotentGuarantee {
			pending = false
		}
		if !pending {
			continue
		}
		var queued casesDomain.ExternalCommitmentQueuedPayload
		if err := json.Unmarshal(record.Payload, &queued); err != nil {
			continue
		}
		var version *casesDomain.ExternalCommitment
		for i := range r.f.versions {
			if r.f.versions[i].ExtCommitmentID == queued.ExtCommitmentID && r.f.versions[i].VersionNo == queued.VersionNo {
				version = &r.f.versions[i]
				break
			}
		}
		if version == nil {
			continue
		}
		candidates = append(candidates, casesPort.ExternalDispatchCandidate{
			Record: record, Version: *version, Events: r.f.events,
			OwnerEpochCurrent: r.f.ownerEpoch, MaxVersionNo: r.f.maxVersion, HMACKey: r.f.hmacKey,
		})
	}
	return candidates, nil
}

func (r *fakeExternalRepo) GasExternalCommitmentQueue(ctx context.Context, p casesPort.ExternalGasParams) (bool, error) {
	r.gasCalls = append(r.gasCalls, p.Reason)
	for _, event := range r.f.events {
		if event.Kind != casesDomain.ExternalDispatchAttemptFailed {
			return false, nil
		}
	}
	r.f.events = append(r.f.events, casesDomain.ExternalDispatchEvent{
		TenantID: p.Scope.TenantID(), CaseID: p.CaseID, ExtCommitmentID: p.ExtCommitmentID,
		VersionNo: p.VersionNo, EventNo: int64(len(r.f.events) + 1),
		Kind: casesDomain.ExternalDispatchCancelledStale, Reason: p.Reason, ActorID: p.ActorID, CreatedAt: p.Now,
	})
	return true, nil
}

func (r *fakeExternalRepo) RecordExternalDispatchOutcome(ctx context.Context, p casesPort.ExternalDispatchOutcomeParams) (bool, error) {
	r.outcomeCalls++
	if !r.outcomeAppend {
		return false, nil
	}
	attemptUsed := int64(0)
	for _, event := range r.f.events {
		switch event.Kind {
		case casesDomain.ExternalDispatchAttemptSubmitted, casesDomain.ExternalDispatchAttemptFailed, casesDomain.ExternalDispatchAttemptUnknown:
			if event.AttemptNo > attemptUsed {
				attemptUsed = event.AttemptNo
			}
		}
	}
	for _, outcome := range p.Events {
		r.f.events = append(r.f.events, casesDomain.ExternalDispatchEvent{
			TenantID: p.Scope.TenantID(), CaseID: p.CaseID, ExtCommitmentID: p.ExtCommitmentID,
			VersionNo: p.VersionNo, EventID: outcome.EventID, EventNo: int64(len(r.f.events) + 1),
			Kind: outcome.Kind, AttemptNo: outcome.AttemptNo,
			ExternalCorrelationID: outcome.ExternalCorrelationID, IdempotentGuarantee: outcome.IdempotentGuarantee,
			Details: outcome.Details, ActorID: p.ActorID, CreatedAt: p.Now,
		})
	}
	return true, nil
}

func (r *fakeExternalRepo) RelayExternalReceipt(ctx context.Context, p casesPort.ExternalRelayReceiptParams) (bool, error) {
	r.relayCalls++
	if !r.f.queued[casesDomain.DedupKeyExternalCommitmentQueued(p.ExtCommitmentID, p.VersionNo)] {
		return false, casesDomain.WorkRuleError{Rule: "relay_external_receipt", Reason: "не состоит в очереди"}
	}
	r.f.events = append(r.f.events, casesDomain.ExternalDispatchEvent{
		TenantID: p.Scope.TenantID(), CaseID: p.CaseID, ExtCommitmentID: p.ExtCommitmentID,
		VersionNo: p.VersionNo, EventID: p.EventID, EventNo: int64(len(r.f.events) + 1),
		Kind: p.Kind, ActorID: p.ActorID, CreatedAt: p.Now,
	})
	return true, nil
}

// fakeExternalIdentity answers the scope gates: the allowed scopes pass,
// a member without any of them gets the honest forbidden word, a
// non-member the plain not-found (the membership blindness — the tenant
// existence is never disclosed).
type fakeExternalIdentity struct {
	allowed map[domain.Scope]bool
	member  bool
}

func (f fakeExternalIdentity) ListMembers(context.Context, domain.Principal, id.UUID) ([]domain.Membership, error) {
	return nil, domain.ErrNotFound
}

func (f fakeExternalIdentity) InviteMember(context.Context, identityAPI.MembershipCommand) (domain.Membership, string, error) {
	return domain.Membership{}, "", domain.ErrNotFound
}

func (f fakeExternalIdentity) AcceptInvitation(context.Context, domain.Principal, string, id.UUID) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrNotFound
}

func (f fakeExternalIdentity) ChangeMember(context.Context, identityAPI.MembershipCommand) (domain.Membership, error) {
	return domain.Membership{}, domain.ErrNotFound
}

func (f fakeExternalIdentity) RevokeMember(context.Context, identityAPI.MembershipCommand) error {
	return domain.ErrNotFound
}

func (f fakeExternalIdentity) Authorize(ctx context.Context, actor domain.Principal, tenantID id.UUID, scope domain.Scope) error {
	if f.allowed[scope] {
		return nil
	}
	if !f.member {
		return domain.ErrNotFound
	}
	return domain.ErrForbidden
}

// fakeExternalRoutes answers the lineage read (the employee facade shape).
type fakeExternalRoutes struct{}

func (fakeExternalRoutes) RouteFacts(ctx context.Context, scope tenancy.Context, connectionID id.UUID) (employeeAPI.RouteFacts, error) {
	employee, _ := id.Parse("77777777-7777-7777-7777-777777777777")
	return employeeAPI.RouteFacts{ConnectionID: connectionID, Candidates: []employeeAPI.RouteCandidate{{
		EmployeeID: employee, DecisionNo: 3, ProfileVersion: 2,
	}}}, nil
}

// fakeExternalDispatcher scripts the outcomes and captures the payloads
// (the idempotency-key equality proofs).
type fakeExternalDispatcher struct {
	outcomes []casesPort.ExternalDispatchResult
	err      error
	payloads []casesDomain.OutboundExternalCommitment
	appended []casesPort.ExternalDispatchResult
}

func (d *fakeExternalDispatcher) Dispatch(ctx context.Context, scope tenancy.Context, payload casesDomain.OutboundExternalCommitment) (casesPort.ExternalDispatchResult, error) {
	d.payloads = append(d.payloads, payload)
	if d.err != nil {
		return casesPort.ExternalDispatchResult{}, d.err
	}
	if len(d.outcomes) > 1 {
		result := d.outcomes[0]
		d.outcomes = d.outcomes[1:]
		d.appended = append(d.appended, result)
		return result, nil
	}
	if len(d.outcomes) == 1 {
		return d.outcomes[0], nil
	}
	return casesPort.ExternalDispatchResult{Outcome: casesDomain.DispatchOutcomeAccepted, ExternalCorrelationID: "corr-1", CapabilityID: "command.send-sms", IdempotentGuarantee: true}, nil
}

// fakeExternalControl scripts the effective control words.
type fakeExternalControl struct {
	mode string
	err  error
}

func (c fakeExternalControl) EffectiveFor(ctx context.Context, tenantID id.UUID, channel string) (casesPort.ExternalControlMode, error) {
	if c.err != nil {
		return casesPort.ExternalControlMode{}, c.err
	}
	return casesPort.ExternalControlMode{Mode: c.mode, Generation: 4, Reason: "причина контроля"}, nil
}

// clock and ids of the unit contour (the frozen now of the fixture).
type externalClock struct{ now time.Time }

func (c externalClock) Now() time.Time { return c.now }

type externalIDs struct{}

func (g *externalIDs) New(now time.Time) (id.UUID, error) { return id.NewV7(now) }

// externalService wires the unit service of the fixture.
func externalService(repo *fakeExternalRepo, identity fakeExternalIdentity, dispatcher casesPort.ExternalDispatcher, control casesPort.ControlEffectiveReader) *Service {
	f := repo.f
	service := New(repo, identity, nil, externalClock{f.now}, &externalIDs{})
	service.routeFacts = fakeExternalRoutes{}
	service.externalDispatcher = dispatcher
	service.controlEffective = control
	return service
}

// approvedFixture prepares the version + approval of the enqueue path.
func approvedFixture(t *testing.T, purpose string) (*fakeExternalRepo, *Service, *externalFixture) {
	t.Helper()
	f := newExternalFixture()
	f.versions = []casesDomain.ExternalCommitment{f.version(purpose)}
	f.approvals = []casesDomain.ExternalApproval{{
		TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, ApproverID: f.owner, StepUpVerified: true,
		CreatedAt: f.now,
	}}
	repo := &fakeExternalRepo{f: f, outcomeAppend: true}
	service := externalService(repo, fakeExternalIdentity{allowed: map[domain.Scope]bool{domain.Review: true, domain.Configuration: true}, member: true}, nil, nil)
	return repo, service, f
}

func errText(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("ожидалась ошибка, получен nil")
	}
	return err.Error()
}

// ---------------------------------------------------------------------------
// Create: the gates in order (the counters prove the refused paths never
// write)
// ---------------------------------------------------------------------------

func TestExternalCreateRefusesNonMemberWithNotFound(t *testing.T) {
	f := newExternalFixture()
	repo := &fakeExternalRepo{f: f}
	service := externalService(repo, fakeExternalIdentity{allowed: map[domain.Scope]bool{}}, nil, nil)
	_, err := service.CreateExternalCommitment(context.Background(), casesAPI.CreateExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		Purpose: casesDomain.ExternalPurposePromise, RecipientKind: casesDomain.ExternalRecipientClient,
		Channel: "phone", ContentText: "Обещание перезвонить", ValidUntil: f.validUntil,
		ExpectedNextStep: "звонок", AckRequirement: casesDomain.ExternalAckNotRequired, Now: f.now,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("не-участник обязан получить ErrNotFound, получено: %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("не-участник не пишет материал: createCalls=%d", repo.createCalls)
	}
}

func TestExternalCreateRefusesAuditOnlyMemberInWords(t *testing.T) {
	f := newExternalFixture()
	repo := &fakeExternalRepo{f: f}
	service := externalService(repo, fakeExternalIdentity{allowed: map[domain.Scope]bool{domain.Audit: true}, member: true}, nil, nil)
	_, err := service.CreateExternalCommitment(context.Background(), casesAPI.CreateExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		Purpose: casesDomain.ExternalPurposePromise, RecipientKind: casesDomain.ExternalRecipientClient,
		Channel: "phone", ContentText: "Обещание перезвонить", ValidUntil: f.validUntil,
		ExpectedNextStep: "звонок", AckRequirement: casesDomain.ExternalAckNotRequired, Now: f.now,
	})
	if !errors.Is(err, domain.ErrForbidden) || !strings.Contains(err.Error(), externalScopeText) {
		t.Fatalf("audit-only участник обязан получить словесный 403, получено: %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("audit-only участник не пишет материал: createCalls=%d", repo.createCalls)
	}
}

func TestExternalCreateRefusesForeignPurposeWord(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.createCalls = 0
	repo.f.versions = nil
	_, err := service.CreateExternalCommitment(context.Background(), casesAPI.CreateExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		Purpose: "pledge", RecipientKind: casesDomain.ExternalRecipientClient,
		Channel: "phone", ContentText: "Обещание перезвонить", ValidUntil: f.validUntil,
		ExpectedNextStep: "звонок", AckRequirement: casesDomain.ExternalAckNotRequired, Now: f.now,
	})
	var ruleErr casesDomain.WorkRuleError
	if !errors.As(err, &ruleErr) || !strings.Contains(err.Error(), "не входит в словарь") {
		t.Fatalf("чужое слово purpose обязано получить словарный отказ, получено: %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("словарный отказ не пишет материал: createCalls=%d", repo.createCalls)
	}
}

func TestExternalCreateRefusesOfferWithoutPin(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposeOffer)
	repo.f.versions = nil
	repo.createCalls = 0
	_, err := service.CreateExternalCommitment(context.Background(), casesAPI.CreateExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		Purpose: casesDomain.ExternalPurposeOffer, RecipientKind: casesDomain.ExternalRecipientClient,
		Channel: "phone", ContentText: "Предложение", ValidUntil: f.validUntil,
		ExpectedNextStep: "звонок", AckRequirement: casesDomain.ExternalAckNotRequired, Now: f.now,
	})
	var ruleErr casesDomain.WorkRuleError
	if !errors.As(err, &ruleErr) || !strings.Contains(err.Error(), "offer требует пин") {
		t.Fatalf("offer без пина обязан получить отказ пина, получено: %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("offer без пина не пишет материал: createCalls=%d", repo.createCalls)
	}
}

func TestExternalCreateRefusesPinMismatchAgainstLiveRow(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.versions = nil
	repo.createCalls = 0
	// The pin triple points at a 2.9 version whose STORED token differs.
	repo.f.commit29 = []casesDomain.CommitmentVersion{{
		TenantID: f.tenant, CommitmentID: f.pinCommit, VersionNo: 2,
		Status: casesDomain.CommitmentStatusOffered, ContentToken: strings.Repeat("a", 64),
	}}
	_, err := service.CreateExternalCommitment(context.Background(), casesAPI.CreateExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		Purpose: casesDomain.ExternalPurposePromise, RecipientKind: casesDomain.ExternalRecipientClient,
		Channel: "phone", ContentText: "Обещание", ValidUntil: f.validUntil,
		ExpectedNextStep: "звонок", AckRequirement: casesDomain.ExternalAckNotRequired,
		Pin: casesDomain.ExternalCommitmentPin{CommitmentID: f.pinCommit, VersionNo: 2, ContentToken: strings.Repeat("b", 64)},
		Now: f.now,
	})
	if !strings.Contains(err.Error(), externalPinMismatchText) {
		t.Fatalf("пин с чужим токеном обязан получить отказ живой строки, получено: %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("несовпавший пин не пишет материал: createCalls=%d", repo.createCalls)
	}
}

func TestExternalCreateRefusesTerminalWorkItem(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.versions = nil
	repo.createCalls = 0
	repo.f.current.Terminal = true
	_, err := service.CreateExternalCommitment(context.Background(), casesAPI.CreateExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		Purpose: casesDomain.ExternalPurposePromise, RecipientKind: casesDomain.ExternalRecipientClient,
		Channel: "phone", ContentText: "Обещание", ValidUntil: f.validUntil,
		ExpectedNextStep: "звонок", AckRequirement: casesDomain.ExternalAckNotRequired, Now: f.now,
	})
	if !strings.Contains(err.Error(), casesDomain.TerminalWorkItemCommitmentText) {
		t.Fatalf("терминальный элемент обязан получить честный отказ, получено: %v", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("терминальный элемент не пишет материал: createCalls=%d", repo.createCalls)
	}
}

func TestExternalCreateWritesFirstVersionWithOwnerSnapshot(t *testing.T) {
	f := newExternalFixture()
	repo := &fakeExternalRepo{f: f}
	service := externalService(repo, fakeExternalIdentity{allowed: map[domain.Scope]bool{domain.Configuration: true}, member: true}, nil, nil)
	outcome, err := service.CreateExternalCommitment(context.Background(), casesAPI.CreateExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		Purpose: casesDomain.ExternalPurposePromise, RecipientKind: casesDomain.ExternalRecipientClient,
		Channel: "phone", ContentText: "Обещание перезвонить через час", ValidUntil: f.validUntil,
		ExpectedNextStep: "звонок клиента", AckRequirement: casesDomain.ExternalAckNotRequired, Now: f.now,
	})
	if err != nil {
		t.Fatalf("создание обязано пройти, получено: %v", err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("создание пишет один раз, createCalls=%d", repo.createCalls)
	}
	if outcome.VersionNo != 1 {
		t.Fatalf("первая версия обязана быть 1, получено %d", outcome.VersionNo)
	}
	stored := repo.f.versions[0]
	if stored.OwnerKind != casesDomain.OwnerKindHuman || stored.OwnerRef != f.owner || stored.OwnerEpoch != 1 {
		t.Fatalf("owner-снимок обязан быть живым: %+v", stored)
	}
	if stored.DecisionNo != 3 || stored.ProfileVersion != 2 {
		t.Fatalf("lineage-числа обязаны быть серверными: %+v", stored)
	}
	if stored.ContentToken != outcome.ContentToken || outcome.ContentToken == "" {
		t.Fatalf("токен материала обязан совпасть с исходом: %q / %q", stored.ContentToken, outcome.ContentToken)
	}
	// The second version of the SAME envelope continues the ladder.
	repo.f.versions[0].VersionNo = 1
	second, err := service.CreateExternalCommitment(context.Background(), casesAPI.CreateExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: outcome.ExtCommitmentID, Purpose: casesDomain.ExternalPurposePromise,
		RecipientKind: casesDomain.ExternalRecipientClient, Channel: "phone",
		ContentText: "Изменённый текст — новая версия", ValidUntil: f.validUntil,
		ExpectedNextStep: "звонок клиента", AckRequirement: casesDomain.ExternalAckNotRequired, Now: f.now,
	})
	if err != nil || second.VersionNo != 2 {
		t.Fatalf("лестница версий обязана продолжиться: %v / %d", err, second.VersionNo)
	}
}

// ---------------------------------------------------------------------------
// Approve: the step-up, the current-owner gate, the silent idempotency
// ---------------------------------------------------------------------------

func TestExternalApproveRefusesWithoutStepUp(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	f.actor.StepUpVerified = false
	_, err := service.ApproveExternalCommitment(context.Background(), casesAPI.ApproveExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if !errors.Is(err, domain.ErrStepUpRequired) {
		t.Fatalf("без step-up утверждение обязано отказаться, получено: %v", err)
	}
	if repo.approveCalls != 0 {
		t.Fatalf("без step-up запись не пишется: approveCalls=%d", repo.approveCalls)
	}
}

func TestExternalApproveRefusesNonOwnerInWords(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	stranger, _ := id.NewV7(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	f.actor.ID = stranger
	_, err := service.ApproveExternalCommitment(context.Background(), casesAPI.ApproveExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if !strings.Contains(err.Error(), externalOwnerGateText) {
		t.Fatalf("не-владелец обязан получить словесный отказ, получено: %v", err)
	}
	if repo.approveCalls != 0 {
		t.Fatalf("не-владелец не пишет approval: approveCalls=%d", repo.approveCalls)
	}
}

func TestExternalApproveRefusesStaffyOwnerInWords(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.terms[0].OwnerKind = casesDomain.OwnerKindStaffy
	repo.f.terms[0].OwnerRef = f.workItemID
	_, err := service.ApproveExternalCommitment(context.Background(), casesAPI.ApproveExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if !strings.Contains(err.Error(), externalOwnerGateText) {
		t.Fatalf("staffy-владелец обязан получить словесный отказ (утверждает только человек), получено: %v", err)
	}
	if repo.approveCalls != 0 {
		t.Fatalf("staffy-владелец не пишет approval: approveCalls=%d", repo.approveCalls)
	}
}

func TestExternalApproveRecordsThenRepeatsSilently(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.approvals = nil
	first, err := service.ApproveExternalCommitment(context.Background(), casesAPI.ApproveExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if err != nil || !first.ApprovalRecorded {
		t.Fatalf("первое утверждение обязано записаться: %v / %+v", err, first)
	}
	second, err := service.ApproveExternalCommitment(context.Background(), casesAPI.ApproveExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if err != nil {
		t.Fatalf("повтор утверждения не ошибка: %v", err)
	}
	if second.ApprovalRecorded {
		t.Fatalf("повтор того же approver — молчаливая идемпотентность (UNIQUE), получено %+v", second)
	}
	if len(repo.f.approvals) != 1 {
		t.Fatalf("ровно одна строка approval: %d", len(repo.f.approvals))
	}
}

// ---------------------------------------------------------------------------
// Enqueue: the approval gate and the readiness refuse BEFORE the
// transaction (the enqueue counter is the proof)
// ---------------------------------------------------------------------------

func TestExternalEnqueueRefusesUnknownVersion(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.versions = nil
	_, err := service.EnqueueExternalCommitment(context.Background(), casesAPI.EnqueueExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if !errors.Is(err, casesDomain.ErrNotFound) {
		t.Fatalf("несуществующая версия — честный not-found, получено: %v", err)
	}
}

func TestExternalEnqueueRefusesWithoutApprovalInWords(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.approvals = nil
	_, err := service.EnqueueExternalCommitment(context.Background(), casesAPI.EnqueueExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if !strings.Contains(err.Error(), casesDomain.ExternalApprovalRequiredText(1)) {
		t.Fatalf("без approval — словесный отказ AC-9, получено: %v", err)
	}
	if repo.enqueueCalls != 0 {
		t.Fatalf("отказ approval не открывает транзакцию: enqueueCalls=%d", repo.enqueueCalls)
	}
}

func TestExternalEnqueueRefusesApprovalOfDepartedApprover(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	stranger, _ := id.NewV7(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	repo.f.approvals[0].ApproverID = stranger
	_, err := service.EnqueueExternalCommitment(context.Background(), casesAPI.EnqueueExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if !strings.Contains(err.Error(), casesDomain.ExternalApprovalRequiredText(1)) {
		t.Fatalf("approval чужого человека не авторизует отправку, получено: %v", err)
	}
	if repo.enqueueCalls != 0 {
		t.Fatalf("чужой approver не открывает транзакцию: enqueueCalls=%d", repo.enqueueCalls)
	}
}

func TestExternalEnqueueRefusesMovedOwnerEpochInWords(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.terms = append([]casesDomain.OwnershipTerm{{
		TenantID: f.tenant, CaseID: f.caseID, Epoch: 2,
		OwnerKind: casesDomain.OwnerKindHuman, OwnerRef: f.owner,
	}}, repo.f.terms...)
	repo.f.ownerEpoch = 2
	_, err := service.EnqueueExternalCommitment(context.Background(), casesAPI.EnqueueExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if !strings.Contains(err.Error(), casesDomain.ExternalApprovalRequiredText(1)) {
		t.Fatalf("двинувшийся epoch отказывает словами AC-9, получено: %v", err)
	}
	if repo.enqueueCalls != 0 {
		t.Fatalf("двинувшийся epoch не открывает транзакцию: enqueueCalls=%d", repo.enqueueCalls)
	}
}

func TestExternalEnqueueRefusesSupersededVersionInWords(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	newer := f.version(casesDomain.ExternalPurposePromise)
	newer.VersionNo = 2
	newer.ContentToken = casesDomain.ExternalCommitmentContentToken(f.hmacKey, newer)
	repo.f.versions = append(repo.f.versions, newer)
	repo.f.maxVersion = 2
	_, err := service.EnqueueExternalCommitment(context.Background(), casesAPI.EnqueueExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if !strings.Contains(err.Error(), casesDomain.ExternalReadinessRefusalTexts[casesDomain.ExternalReadinessSuperseded]) {
		t.Fatalf("суперседная версия отказывает честной причиной, получено: %v", err)
	}
	if repo.enqueueCalls != 0 {
		t.Fatalf("суперседная версия не открывает транзакцию: enqueueCalls=%d", repo.enqueueCalls)
	}
}

func TestExternalEnqueueRefusesExpiredWindowInWords(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.versions[0].ValidUntil = f.now
	_, err := service.EnqueueExternalCommitment(context.Background(), casesAPI.EnqueueExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if !strings.Contains(err.Error(), casesDomain.ExternalReadinessRefusalTexts[casesDomain.ExternalReadinessExpired]) {
		t.Fatalf("истёкшее окно отказывает честной причиной, получено: %v", err)
	}
	if repo.enqueueCalls != 0 {
		t.Fatalf("истёкшее окно не открывает транзакцию: enqueueCalls=%d", repo.enqueueCalls)
	}
}

func TestExternalEnqueueRefusesOfferWithBlockedPinReadiness(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposeOffer)
	// The offer pin points at a SUPERSEDED 2.9 version: the readiness is
	// the pin-blocked word, the refusal names the discipline of 2.9.
	token := strings.Repeat("c", 64)
	repo.f.versions[0].Pin = casesDomain.ExternalCommitmentPin{CommitmentID: f.pinCommit, VersionNo: 1, ContentToken: token}
	repo.f.versions[0].ContentToken = casesDomain.ExternalCommitmentContentToken(f.hmacKey, repo.f.versions[0])
	repo.f.commit29 = []casesDomain.CommitmentVersion{
		{TenantID: f.tenant, CommitmentID: f.pinCommit, VersionNo: 1, Status: casesDomain.CommitmentStatusOffered, ContentToken: token},
		{TenantID: f.tenant, CommitmentID: f.pinCommit, VersionNo: 2, Status: casesDomain.CommitmentStatusOffered, ContentToken: strings.Repeat("d", 64)},
	}
	_, err := service.EnqueueExternalCommitment(context.Background(), casesAPI.EnqueueExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if !strings.Contains(err.Error(), casesDomain.ExternalReadinessRefusalTexts[casesDomain.ExternalReadinessPinBlocked]) {
		t.Fatalf("пин не offered_confirmable отказывает честной причиной, получено: %v", err)
	}
	if repo.enqueueCalls != 0 {
		t.Fatalf("заблокированный пин не открывает транзакцию: enqueueCalls=%d", repo.enqueueCalls)
	}
}

func TestExternalEnqueueQueuesApprovedReadyVersion(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	outcome, err := service.EnqueueExternalCommitment(context.Background(), casesAPI.EnqueueExternalCommitmentCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Now: f.now,
	})
	if err != nil {
		t.Fatalf("валидный enqueue обязан пройти, получено: %v", err)
	}
	if repo.enqueueCalls != 1 {
		t.Fatalf("валидный enqueue открывает одну транзакцию: enqueueCalls=%d", repo.enqueueCalls)
	}
	if !outcome.Queued {
		t.Fatalf("исход обязан сказать queued: %+v", outcome)
	}
	if !repo.f.queued[casesDomain.DedupKeyExternalCommitmentQueued(f.extID, 1)] {
		t.Fatal("очередь обязана нести dedup-ключ пина версии")
	}
}

// ---------------------------------------------------------------------------
// Relay: the observation dictionary and the queue evidence
// ---------------------------------------------------------------------------

func TestExternalRelayRefusesForeignObservationWord(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	_, err := service.RelayExternalReceipt(context.Background(), casesAPI.RelayExternalReceiptCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Observation: "maybe_delivered", Now: f.now,
	})
	if !strings.Contains(err.Error(), casesDomain.RelayObservationForeignText[:20]) {
		t.Fatalf("чужое слово наблюдения обязано получить словарный отказ, получено: %v", err)
	}
	if repo.relayCalls != 0 {
		t.Fatalf("чужое слово не пишет событие: relayCalls=%d", repo.relayCalls)
	}
}

func TestExternalRelayRefusesUnknownVersion(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.versions = nil
	_, err := service.RelayExternalReceipt(context.Background(), casesAPI.RelayExternalReceiptCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Observation: casesDomain.RelayObservationDelivered, Now: f.now,
	})
	if !errors.Is(err, casesDomain.ErrNotFound) {
		t.Fatalf("несуществующая версия — честный not-found, получено: %v", err)
	}
}

func TestExternalRelayMapsObservationToReceiptKind(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.queued[casesDomain.DedupKeyExternalCommitmentQueued(f.extID, 1)] = true
	outcome, err := service.RelayExternalReceipt(context.Background(), casesAPI.RelayExternalReceiptCommand{
		Actor: f.actor, TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, Observation: casesDomain.RelayObservationAcknowledged, Now: f.now,
	})
	if err != nil {
		t.Fatalf("валидный relay обязан пройти, получено: %v", err)
	}
	if outcome.Kind != casesDomain.ExternalDispatchReceiptAcknowledged {
		t.Fatalf("наблюдение acknowledged маппится в receipt_acknowledged, получено: %s", outcome.Kind)
	}
	if repo.relayCalls != 1 {
		t.Fatalf("relay пишет одно событие: relayCalls=%d", repo.relayCalls)
	}
	// The derived state over the relayed history: the LAST event decides.
	state := casesDomain.DeriveExternalDeliveryState(true, repo.f.events)
	if state.State != casesDomain.ExternalDeliveryAcknowledged {
		t.Fatalf("состояние чтения после relay — acknowledged, получено: %s", state.State)
	}
}

// ---------------------------------------------------------------------------
// Drain: the AG-4 boundary, the fences in order, the control point, the
// dispatch policy and the dedup
// ---------------------------------------------------------------------------

func TestExternalDrainRefusesWithoutDispatcherAG4(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	_, err := service.DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if !strings.Contains(err.Error(), externalDrainRefusedText) {
		t.Fatalf("AG-4: без диспетчера дренер отказывает словами, получено: %v", err)
	}
	if repo.pendingCalls != 0 {
		t.Fatalf("AG-4: отказ до чтения очереди, pendingCalls=%d", repo.pendingCalls)
	}
}

func TestExternalDrainGasesNewerVersion(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	newer := f.version(casesDomain.ExternalPurposePromise)
	newer.VersionNo = 2
	repo.f.versions = append(repo.f.versions, newer)
	repo.f.maxVersion = 2
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{}
	drained, err := service.WithExternalDispatcher(dispatcher).DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if err != nil {
		t.Fatalf("дренер обязан пройти, получено: %v", err)
	}
	if drained.Gassed != 1 || len(repo.gasCalls) != 1 || repo.gasCalls[0] != casesDomain.ExternalStaleNewerVersion {
		t.Fatalf("новее версия гасит newer_version: %+v / %v", drained, repo.gasCalls)
	}
	if len(dispatcher.payloads) != 0 {
		t.Fatalf("гашение не вызывает диспетчер: %d", len(dispatcher.payloads))
	}
	if drained.Considered != 1 {
		t.Fatalf("рассмотрена одна запись: %+v", drained)
	}
}

func TestExternalDrainGasesExpiredWindow(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.versions[0].ValidUntil = f.now.Add(-time.Hour)
	// The mutation of the window changes the canonical bytes — the honest
	// test recomputes the content token of the mutated row (the drain
	// re-check basis).
	repo.f.versions[0].ContentToken = casesDomain.ExternalCommitmentContentToken(f.hmacKey, repo.f.versions[0])
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{}
	drained, _ := service.WithExternalDispatcher(dispatcher).DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.Gassed != 1 || repo.gasCalls[0] != casesDomain.ExternalStaleExpired {
		t.Fatalf("истёкшее окно гасит expired: %+v / %v", drained, repo.gasCalls)
	}
	if len(dispatcher.payloads) != 0 {
		t.Fatalf("гашение не вызывает диспетчер: %d", len(dispatcher.payloads))
	}
}

func TestExternalDrainGasesMovedOwnerEpoch(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.terms = []casesDomain.OwnershipTerm{{
		TenantID: f.tenant, CaseID: f.caseID, Epoch: 5,
		OwnerKind: casesDomain.OwnerKindHuman, OwnerRef: f.owner,
	}}
	repo.f.ownerEpoch = 5
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{}
	drained, _ := service.WithExternalDispatcher(dispatcher).DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.Gassed != 1 || repo.gasCalls[0] != casesDomain.ExternalStaleOwnerEpoch {
		t.Fatalf("двинувшийся epoch гасит owner_epoch_moved: %+v / %v", drained, repo.gasCalls)
	}
}

func TestExternalDrainGasesOnNilControlFailClosed(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{}
	drained, _ := service.WithExternalDispatcher(dispatcher).DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.Gassed != 1 || repo.gasCalls[0] != casesDomain.ExternalStaleControlMode {
		t.Fatalf("nil-контроль гасит fail-closed control_mode: %+v / %v", drained, repo.gasCalls)
	}
	if len(dispatcher.payloads) != 0 {
		t.Fatalf("гашение не вызывает диспетчер: %d", len(dispatcher.payloads))
	}
}

func TestExternalDrainGasesEveryNonAcceptControlMode(t *testing.T) {
	for _, mode := range []string{"paused", "human_only", "intake_only/no_promises"} {
		repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
		f.pending = []casesDomain.OutboxEvent{f.enqueue()}
		dispatcher := &fakeExternalDispatcher{}
		drained, _ := service.WithExternalDispatcher(dispatcher).
			WithControlEffective(fakeExternalControl{mode: mode}).
			DrainExternalCommitments(context.Background(), f.actor, f.tenant)
		if drained.Gassed != 1 || repo.gasCalls[0] != casesDomain.ExternalStaleControlMode {
			t.Fatalf("режим %s гасит control_mode (emergency brake): %+v / %v", mode, drained, repo.gasCalls)
		}
	}
}

func TestExternalDrainGasesOnControlResolutionError(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{}
	drained, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{err: errors.New("контроль недоступен")}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.Gassed != 1 || repo.gasCalls[0] != casesDomain.ExternalStaleControlMode {
		t.Fatalf("нерезолвция контроля гасит fail-closed: %+v / %v", drained, repo.gasCalls)
	}
}

func TestExternalDrainSkipsTokenAlarmWithoutSendOrGasp(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	// The corrupted row: the content token does not re-verify against the
	// row bytes (impossible through the guards — the probe stays honest).
	repo.f.versions[0].ContentToken = strings.Repeat("f", 64)
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{}
	drained, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.Skipped != 1 || drained.Gassed != 0 || len(dispatcher.payloads) != 0 {
		t.Fatalf("токен-тревога: skip без отправки и без гашения: %+v", drained)
	}
	if len(drained.Reports) != 1 || !strings.Contains(drained.Reports[0].Text, "reconciliation") {
		t.Fatalf("тревога зовёт reconciliation (3-7): %+v", drained.Reports)
	}
}

func TestExternalDrainDispatchesAcceptedOutcome(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{}
	drained, err := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if err != nil {
		t.Fatalf("дренер обязан пройти: %v", err)
	}
	if drained.Submitted != 1 || drained.Gassed != 0 || drained.Skipped != 0 {
		t.Fatalf("accepted → submitted: %+v", drained)
	}
	if len(dispatcher.payloads) != 1 {
		t.Fatalf("ровно одна отправка: %d", len(dispatcher.payloads))
	}
	// The payload is deterministic from the immutable version and carries
	// the idempotency key derivative of the version pin (opaque).
	payload := dispatcher.payloads[0]
	wantKey := casesDomain.ExternalCommitmentIdempotencyKey(f.tenant, f.extID, 1)
	if payload.IdempotencyKey != wantKey {
		t.Fatalf("ключ идемпотентности — производная пина версии: %s", payload.IdempotencyKey)
	}
	if payload.ContentText != "Материал конверта" {
		t.Fatalf("payload несёт материал доставки: %+v", payload)
	}
	// The appended history decides the state: submitted, never delivered.
	state := casesDomain.DeriveExternalDeliveryState(true, repo.f.events)
	if state.State != casesDomain.ExternalDeliverySubmitted {
		t.Fatalf("состояние после accepted — submitted: %s", state.State)
	}
	if state.LastExternalCorrelationID != "corr-1" {
		t.Fatalf("корреляция AC-8 сохранена в состоянии: %+v", state)
	}
}

func TestExternalDrainRecordsDeliveredOutcomeAsTwoEvents(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{outcomes: []casesPort.ExternalDispatchResult{{
		Outcome: casesDomain.DispatchOutcomeDelivered, ExternalCorrelationID: "corr-9",
		CapabilityID: "command.send-sms", IdempotentGuarantee: true,
	}}}
	drained, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.Delivered != 1 {
		t.Fatalf("delivered → delivered: %+v", drained)
	}
	if len(repo.f.events) != 2 {
		t.Fatalf("два события одной отправки (submitted + receipt): %d", len(repo.f.events))
	}
	if repo.f.events[0].Kind != casesDomain.ExternalDispatchAttemptSubmitted ||
		repo.f.events[1].Kind != casesDomain.ExternalDispatchReceiptDelivered {
		t.Fatalf("порядок событий: %s → %s", repo.f.events[0].Kind, repo.f.events[1].Kind)
	}
	state := casesDomain.DeriveExternalDeliveryState(true, repo.f.events)
	if state.State != casesDomain.ExternalDeliveryDelivered {
		t.Fatalf("состояние после delivered — delivered: %s", state.State)
	}
}

func TestExternalDrainRecordsFailedOutcomeRetryableWithinBound(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{outcomes: []casesPort.ExternalDispatchResult{{
		Outcome: casesDomain.DispatchOutcomeFailed, CapabilityID: "command.send-sms",
	}}}
	drained, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.FailedRetry != 1 {
		t.Fatalf("failed → повторяемый отказ: %+v", drained)
	}
	state := casesDomain.DeriveExternalDeliveryState(true, repo.f.events)
	if state.State != casesDomain.ExternalDeliveryFailed || state.LastAttemptNo != 1 {
		t.Fatalf("состояние failed, попытка 1: %+v", state)
	}
	// The second drain run retries the failed record with the SAME
	// idempotency key (the safe retry within the bound).
	dispatcher.payloads = nil
	drained2, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained2.FailedRetry != 1 || len(dispatcher.payloads) != 1 {
		t.Fatalf("повтор failed в пределах bound: %+v / %d", drained2, len(dispatcher.payloads))
	}
	wantKey := casesDomain.ExternalCommitmentIdempotencyKey(f.tenant, f.extID, 1)
	if dispatcher.payloads[0].IdempotencyKey != wantKey {
		t.Fatalf("повтор несёт ТОТ ЖЕ ключ: %s", dispatcher.payloads[0].IdempotencyKey)
	}
	state2 := casesDomain.DeriveExternalDeliveryState(true, repo.f.events)
	if state2.LastAttemptNo != 2 {
		t.Fatalf("вторая попытка = 2: %+v", state2)
	}
}

func TestExternalDrainParksGuaranteeFreeTimeoutWithoutBlindRetry(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{outcomes: []casesPort.ExternalDispatchResult{{
		Outcome: casesDomain.DispatchOutcomeTimeout, ExternalCorrelationID: "corr-t", IdempotentGuarantee: false,
	}}}
	drained, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.UnknownParked != 1 || len(repo.f.events) != 1 || repo.f.events[0].Kind != casesDomain.ExternalDispatchAttemptUnknown {
		t.Fatalf("timeout без гарантии — unknown-парковка: %+v / %d", drained, len(repo.f.events))
	}
	if repo.f.events[0].IdempotentGuarantee {
		t.Fatal("гарантия false обязана быть записана в событии")
	}
	// The next drain run NEVER selects the guarantee-free unknown — the
	// blind retry is structurally impossible (the fake mirrors the store).
	dispatcher.payloads = nil
	drained2, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if len(dispatcher.payloads) != 0 || drained2.Considered != 0 {
		t.Fatalf("слепого повтора НЕТ: payloads=%d considered=%d", len(dispatcher.payloads), drained2.Considered)
	}
	state := casesDomain.DeriveExternalDeliveryState(true, repo.f.events)
	if state.State != casesDomain.ExternalDeliveryUnknown {
		t.Fatalf("состояние unknown зовёт reconciliation: %s", state.State)
	}
	if !strings.Contains(casesDomain.ExternalDeliveryStateTexts[casesDomain.ExternalDeliveryUnknown], "reconciliation") {
		t.Fatal("текст unknown зовёт reconciliation (3-7)")
	}
}

func TestExternalDrainRetriesTimeoutUnderGuaranteeWithSameKey(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{outcomes: []casesPort.ExternalDispatchResult{{
		Outcome: casesDomain.DispatchOutcomeTimeout, IdempotentGuarantee: true,
	}}}
	drained, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.UnknownParked != 1 {
		t.Fatalf("timeout с гарантией — unknown записан: %+v", drained)
	}
	if !repo.f.events[0].IdempotentGuarantee {
		t.Fatal("гарантия true записана в событии")
	}
	// The next drain run retries the guaranteed unknown with the SAME key
	// (the safe retry within the guarantee).
	dispatcher.outcomes = []casesPort.ExternalDispatchResult{{
		Outcome: casesDomain.DispatchOutcomeAccepted, ExternalCorrelationID: "corr-ok",
	}}
	dispatcher.payloads = nil
	drained2, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained2.Submitted != 1 || len(dispatcher.payloads) != 1 {
		t.Fatalf("гарантийный unknown повторяется: %+v / %d", drained2, len(dispatcher.payloads))
	}
	wantKey := casesDomain.ExternalCommitmentIdempotencyKey(f.tenant, f.extID, 1)
	if dispatcher.payloads[0].IdempotencyKey != wantKey {
		t.Fatalf("повтор несёт ТОТ ЖЕ ключ (провайдер дедупит): %s", dispatcher.payloads[0].IdempotencyKey)
	}
}

func TestExternalDrainSkipsRacedTwinWithoutSecondIntent(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	repo.outcomeAppend = false // the raced twin decided first
	dispatcher := &fakeExternalDispatcher{}
	drained, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.Skipped != 1 || drained.Submitted != 0 {
		t.Fatalf("побеждённый близнец — честный skip без второго намерения: %+v", drained)
	}
	if len(repo.f.events) != 0 {
		t.Fatalf("второе намерение не пишется: %d", len(repo.f.events))
	}
	if len(drained.Reports) != 1 || !strings.Contains(drained.Reports[0].Text, "второго намерения нет") {
		t.Fatalf("skip назван честно: %+v", drained.Reports)
	}
}

func TestExternalDrainParksBoundExhaustedRecord(t *testing.T) {
	_, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	// Three attempt_failed events exhaust the bound — the record is not
	// selectable (the fake mirrors the store); a forced candidate parks.
	f.events = []casesDomain.ExternalDispatchEvent{
		{Kind: casesDomain.ExternalDispatchAttemptFailed, AttemptNo: 1, EventNo: 1},
		{Kind: casesDomain.ExternalDispatchAttemptFailed, AttemptNo: 2, EventNo: 2},
		{Kind: casesDomain.ExternalDispatchAttemptFailed, AttemptNo: 3, EventNo: 3},
	}
	dispatcher := &fakeExternalDispatcher{}
	drained, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.Considered != 0 || len(dispatcher.payloads) != 0 {
		t.Fatalf("исчерпанный bound структурно не выбирается: %+v / %d", drained, len(dispatcher.payloads))
	}
	state := casesDomain.DeriveExternalDeliveryState(true, f.events)
	if state.State != casesDomain.ExternalDeliveryFailed {
		t.Fatalf("состояние после bound — failed терминально: %s", state.State)
	}
}

func TestExternalDrainRecordsAdapterErrorAsUnknown(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{err: errors.New("адаптер недоступен")}
	drained, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.UnknownParked != 1 || repo.f.events[0].Kind != casesDomain.ExternalDispatchAttemptUnknown {
		t.Fatalf("ошибка адаптера — честный unknown: %+v", drained)
	}
	if repo.f.events[0].IdempotentGuarantee {
		t.Fatal("ошибка адаптера не заявляет гарантию")
	}
	if !strings.Contains(repo.f.events[0].Details, "диспетчер") {
		t.Fatalf("details несут безопасный текст: %q", repo.f.events[0].Details)
	}
}

func TestExternalDrainRecordsForeignOutcomeAsUnknown(t *testing.T) {
	repo, service, f := approvedFixture(t, casesDomain.ExternalPurposePromise)
	f.pending = []casesDomain.OutboxEvent{f.enqueue()}
	dispatcher := &fakeExternalDispatcher{outcomes: []casesPort.ExternalDispatchResult{{
		Outcome: "maybe", ExternalCorrelationID: "x",
	}}}
	drained, _ := service.WithExternalDispatcher(dispatcher).
		WithControlEffective(fakeExternalControl{mode: casesPort.ExternalControlAccept}).
		DrainExternalCommitments(context.Background(), f.actor, f.tenant)
	if drained.UnknownParked != 1 || repo.f.events[0].Kind != casesDomain.ExternalDispatchAttemptUnknown {
		t.Fatalf("чужой исход — честный unknown fail-closed: %+v", drained)
	}
	if !strings.Contains(repo.f.events[0].Details, "вне словаря") {
		t.Fatalf("details называют поломанный контракт: %q", repo.f.events[0].Details)
	}
}

// The outbox perimeter probe: the queued payload NEVER carries the content
// (the material never leaves the row, AC-10).
func TestExternalQueuedPayloadCarriesNoContentMaterial(t *testing.T) {
	payload, err := casesDomain.ExternalCommitmentQueuedPayloadJSON(casesDomain.ExternalCommitmentQueuedPayload{
		TenantID: newExternalFixture().tenant, ExtCommitmentID: newExternalFixture().extID,
		VersionNo: 1, Purpose: casesDomain.ExternalPurposePromise,
		RecipientKind: casesDomain.ExternalRecipientClient, Channel: "phone",
		IdempotencyKey: "k", ContentDigest: "d",
	})
	if err != nil {
		t.Fatalf("payload обязан кодироваться: %v", err)
	}
	if strings.Contains(string(payload), "content_text") || strings.Contains(string(payload), "Материал") {
		t.Fatalf("outbox конверт НЕ несёт содержание: %s", payload)
	}
	if !strings.Contains(string(payload), "content_digest") {
		t.Fatalf("outbox конверт несёт digest: %s", payload)
	}
}

// ---------------------------------------------------------------------------
// The Commitment Review view: the read-time derivations over the fixture
// material (the queue evidence, the seven delivery words, the readiness
// fences, the approval material and the honest empty)
// ---------------------------------------------------------------------------

// readFixture prepares the read view material: the version, the approval,
// the queue evidence and the dispatch history.
func readFixture(t *testing.T, purpose string) (*fakeExternalRepo, *Service, *externalFixture) {
	t.Helper()
	f := newExternalFixture()
	f.versions = []casesDomain.ExternalCommitment{f.version(purpose)}
	f.approvals = []casesDomain.ExternalApproval{{
		TenantID: f.tenant, CaseID: f.caseID, WorkItemID: f.workItemID,
		ExtCommitmentID: f.extID, VersionNo: 1, ApproverID: f.owner, StepUpVerified: true,
		CreatedAt: f.now,
	}}
	repo := &fakeExternalRepo{f: f}
	service := externalService(repo, fakeExternalIdentity{allowed: map[domain.Scope]bool{domain.Review: true, domain.Audit: true}, member: true}, nil, nil)
	return repo, service, f
}

func TestExternalReadViewRefusesNonMemberAndScopelessMember(t *testing.T) {
	f := newExternalFixture()
	nonMember := &fakeExternalRepo{f: f}
	service := externalService(nonMember, fakeExternalIdentity{allowed: map[domain.Scope]bool{}}, nil, nil)
	_, err := service.ReadExternalCommitmentView(context.Background(), f.actor, f.tenant, f.caseID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("не-участник — честный 404, получено: %v", err)
	}
	scopeless := &fakeExternalRepo{f: f}
	service = externalService(scopeless, fakeExternalIdentity{allowed: map[domain.Scope]bool{}, member: true}, nil, nil)
	_, err = service.ReadExternalCommitmentView(context.Background(), f.actor, f.tenant, f.caseID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("участник без скоупов — честный 403, получено: %v", err)
	}
}

func TestExternalReadViewServesSectionsAndDerivations(t *testing.T) {
	repo, service, f := readFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.queued[casesDomain.DedupKeyExternalCommitmentQueued(f.extID, 1)] = true
	repo.f.events = []casesDomain.ExternalDispatchEvent{{
		TenantID: f.tenant, CaseID: f.caseID, ExtCommitmentID: f.extID, VersionNo: 1,
		EventID: f.extID, EventNo: 1, Kind: casesDomain.ExternalDispatchAttemptSubmitted,
		AttemptNo: 1, ExternalCorrelationID: "prov-1", IdempotentGuarantee: true,
		Details: "исход: accepted", ActorID: f.owner, CreatedAt: f.now,
	}}
	view, err := service.ReadExternalCommitmentView(context.Background(), f.actor, f.tenant, f.caseID)
	if err != nil {
		t.Fatalf("чтение обязано пройти: %v", err)
	}
	if len(view.Items) != 1 || view.Items[0].WorkItemID != f.workItemID {
		t.Fatalf("секция — по одному на work item, получено: %+v", view.Items)
	}
	if len(view.Items[0].Versions) != 1 {
		t.Fatalf("карточка обязана быть одна, получено: %+v", view.Items[0].Versions)
	}
	card := view.Items[0].Versions[0]
	if !card.Enqueued {
		t.Fatalf("доказательство очереди — outbox запись, получено Enqueued=false")
	}
	if card.Delivery.State != casesDomain.ExternalDeliverySubmitted {
		t.Fatalf("последнее событие решает состояние: submitted, получено: %q", card.Delivery.State)
	}
	if card.Delivery.StateText != casesDomain.ExternalDeliveryStateTexts[casesDomain.ExternalDeliverySubmitted] {
		t.Fatalf("текст состояния — фиксированный доменный, получено: %q", card.Delivery.StateText)
	}
	if card.Delivery.LastExternalCorrelationID != "prov-1" || card.Delivery.LastEventNo != 1 {
		t.Fatalf("материал последнего события обязан приехать, получено: %+v", card.Delivery)
	}
	if card.Readiness != casesDomain.ExternalReadinessReady || card.ReadinessText != "" {
		t.Fatalf("живая версия — ready без текста отказа, получено: %q %q", card.Readiness, card.ReadinessText)
	}
	if !card.Current || !card.Approved || card.ApproverID != f.owner {
		t.Fatalf("current и approval материал обязаны быть честными, получено: %+v", card)
	}
	if len(card.Events) != 1 || card.Events[0].Kind != casesDomain.ExternalDispatchAttemptSubmitted ||
		card.Events[0].IdempotentGuarantee != true {
		t.Fatalf("append-only история обязана приехать на проводе, получено: %+v", card.Events)
	}
	if len(view.PurposeWords) != 4 || len(view.RelayWords) != 3 || len(view.StateWords) != 7 || len(view.ChannelWords) != 9 {
		t.Fatalf("словари обязаны приехать: purpose %d relay %d states %d channels %d",
			len(view.PurposeWords), len(view.RelayWords), len(view.StateWords), len(view.ChannelWords))
	}
	if view.PanelText == "" || view.EmptyText == "" {
		t.Fatalf("честные тексты панели и пустоты обязательны")
	}
}

func TestExternalReadViewDerivesSupersededAndCancelledHonesty(t *testing.T) {
	repo, service, f := readFixture(t, casesDomain.ExternalPurposePromise)
	// v1 — в очереди, загашена newer_version (слова не ушли); v2 — текущая persisted.
	v1 := f.version(casesDomain.ExternalPurposePromise)
	v2 := f.version(casesDomain.ExternalPurposePromise)
	v2.VersionNo = 2
	v2.ContentToken = casesDomain.ExternalCommitmentContentToken(f.hmacKey, v2)
	repo.f.versions = []casesDomain.ExternalCommitment{v1, v2}
	repo.f.queued[casesDomain.DedupKeyExternalCommitmentQueued(f.extID, 1)] = true
	repo.f.events = []casesDomain.ExternalDispatchEvent{{
		TenantID: f.tenant, CaseID: f.caseID, ExtCommitmentID: f.extID, VersionNo: 1,
		EventID: f.extID, EventNo: 1, Kind: casesDomain.ExternalDispatchCancelledStale,
		Reason: casesDomain.ExternalStaleNewerVersion, ActorID: f.owner, CreatedAt: f.now,
	}}
	view, err := service.ReadExternalCommitmentView(context.Background(), f.actor, f.tenant, f.caseID)
	if err != nil {
		t.Fatalf("чтение обязано пройти: %v", err)
	}
	cards := view.Items[0].Versions
	if len(cards) != 2 {
		t.Fatalf("обе версии обязаны быть в секции, получено: %d", len(cards))
	}
	v1Card, v2Card := cards[0], cards[1]
	if v1Card.Readiness != casesDomain.ExternalReadinessSuperseded ||
		v1Card.ReadinessText != casesDomain.ExternalReadinessRefusalTexts[casesDomain.ExternalReadinessSuperseded] {
		t.Fatalf("v1 — superseded с честным текстом, получено: %q %q", v1Card.Readiness, v1Card.ReadinessText)
	}
	if v1Card.Current {
		t.Fatalf("v1 не текущая — новое слово в тексте, не цветом")
	}
	if v1Card.Delivery.State != casesDomain.ExternalDeliveryPersisted {
		t.Fatalf("cancelled_stale возвращает persisted — слова не ушли, получено: %q", v1Card.Delivery.State)
	}
	if v1Card.Delivery.LastKind != casesDomain.ExternalDispatchCancelledStale ||
		v1Card.Events[0].ReasonText != casesDomain.ExternalStaleReasonTexts[casesDomain.ExternalStaleNewerVersion] {
		t.Fatalf("причина гашения — материал истории с доменным текстом, получено: %+v", v1Card.Events)
	}
	if !v1Card.Enqueued {
		t.Fatalf("доказательство очереди не исчезает от событий: outbox запись материальна")
	}
	if v2Card.Delivery.State != casesDomain.ExternalDeliveryPersisted || v2Card.Enqueued {
		t.Fatalf("v2 — persisted без очереди, получено: %q enqueued=%v", v2Card.Delivery.State, v2Card.Enqueued)
	}
	if !v2Card.Current {
		t.Fatalf("v2 — текущая версия конверта")
	}
}

func TestExternalReadViewOfferPinBlockedDegradesHonestly(t *testing.T) {
	repo, service, f := readFixture(t, casesDomain.ExternalPurposeOffer)
	version := f.version(casesDomain.ExternalPurposeOffer)
	version.Pin = casesDomain.ExternalCommitmentPin{CommitmentID: f.pinCommit, VersionNo: 3, ContentToken: "64hexpin"}
	version.ContentToken = casesDomain.ExternalCommitmentContentToken(f.hmacKey, version)
	repo.f.versions = []casesDomain.ExternalCommitment{version}
	repo.f.commit29 = nil // пиннутой строки 2.9 нет — слово деградирует fail-closed
	view, err := service.ReadExternalCommitmentView(context.Background(), f.actor, f.tenant, f.caseID)
	if err != nil {
		t.Fatalf("view НЕ падает на стале материале — слово несёт честность: %v", err)
	}
	card := view.Items[0].Versions[0]
	if card.Readiness != casesDomain.ExternalReadinessPinBlocked ||
		card.ReadinessText != casesDomain.ExternalReadinessRefusalTexts[casesDomain.ExternalReadinessPinBlocked] {
		t.Fatalf("отсутствующий пин — pin_blocked с честным текстом, получено: %q %q", card.Readiness, card.ReadinessText)
	}
}

func TestExternalReadViewOwnerStaleWhenEpochMoved(t *testing.T) {
	repo, service, f := readFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.terms = []casesDomain.OwnershipTerm{{
		TenantID: f.tenant, CaseID: f.caseID, Epoch: 2,
		OwnerKind: casesDomain.OwnerKindHuman, OwnerRef: f.owner,
	}}
	view, err := service.ReadExternalCommitmentView(context.Background(), f.actor, f.tenant, f.caseID)
	if err != nil {
		t.Fatalf("чтение обязано пройти: %v", err)
	}
	card := view.Items[0].Versions[0]
	if card.Readiness != casesDomain.ExternalReadinessOwnerStale {
		t.Fatalf("двинувшийся epoch — owner_stale, получено: %q", card.Readiness)
	}
}

func TestExternalReadViewHonestEmptySection(t *testing.T) {
	repo, service, f := readFixture(t, casesDomain.ExternalPurposePromise)
	repo.f.versions = nil
	repo.f.approvals = nil
	view, err := service.ReadExternalCommitmentView(context.Background(), f.actor, f.tenant, f.caseID)
	if err != nil {
		t.Fatalf("пустой кейс — честный ответ, не ошибка: %v", err)
	}
	if len(view.Items) != 1 {
		t.Fatalf("секция work item остаётся — пустые версии внутри, получено: %+v", view.Items)
	}
	if len(view.Items[0].Versions) != 0 {
		t.Fatalf("пустой список версий — честный ноль, получено: %+v", view.Items[0].Versions)
	}
}
