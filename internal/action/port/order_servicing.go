package port

import (
	"context"
	"errors"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// Story 4.10: the order servicing ports. The request is the RECORDED
// EVENT of the repeat contact (FR-62): the ONE append of the slice
// over the LIVE contact case; the lookup reads the RECORDED material
// of the contour only (the receipts, the rereads, the facts — never a
// live external system, AG-4/OQ-18); the assurance rides the NEW
// narrow port over the cases API (the LIVE 2-3 derivation of the
// contact case — the composition-root adapter precedent of the
// CaseMaterialSource). NO adapter of the external systems, NO client
// delivery surface, NO automatic classification exists (OQ-1/OQ-2/
// AG-4) — the write path is the operator's record command only.

// The store-contract errors of the servicing contour (the honest
// vocabulary of the repository boundary — the transport maps them).
var (
	// ErrOrderServicingRequestDuplicate is the honest replay signal of
	// the idempotency BY KEY: the request of the SAME full key (the
	// actor, the intent, the identifier kind and value, the proof
	// state) already exists — the repeated command replays the SAVED
	// request, never a second row. The CHANGED trust state is a
	// DIFFERENT key: a new row lands (the trust ladder stays visible).
	ErrOrderServicingRequestDuplicate = errors.New("запрос обслуживания уже записан (идемпотентный повтор)")
)

// AppendOrderServicingRequestParams is the ONE append of the contour:
// the request row with the anchor pins (the LIVE contact case, the
// nullable conversation anchor), the intent word, the TYPED identifier
// the client named, the resolved operation anchor (zero when
// not_found/ambiguous — the enumeration does not exist), the lookup
// outcome, the LIVE proof pins copied at the moment of the record (the
// state, the method, the issued/expiry times — AC-4) and the actor
// identity. The store computes the monotonic request_no of the case
// ladder under the advisory lock; the recorded_at is the server time
// of the append. The signature carries ZERO adapters — the append is a
// pure persistence boundary (the request records the ask, it never
// performs it — the OQ-3 fence of the contour).
type AppendOrderServicingRequestParams struct {
	Scope               tenancy.Context
	CaseID              id.UUID
	ConversationID      id.UUID
	Intent              string
	IdentifierKind      string
	IdentifierValue     string
	ResolvedOperationID id.UUID
	LookupOutcome       string
	ProofState          string
	ProofMethod         string
	ProofIssuedAt       time.Time
	ProofExpiresAt      time.Time
	Disclosure          string
	ActorID             id.UUID
	Now                 time.Time
}

// OrderServicingRepository is the persistence contract of the order
// servicing requests: the case reads (the panel feed of the recorded
// requests), the lookup BY KEY over the RECORDED material of the
// tenant (the receipts of the order operations ∪ the rereads of the
// corrections for the ABCP kind; the supplier order facts for the
// supplier kind — the sources ride the candidates BY NAME), the
// resolved-operation read (the LIVE projection assembly of the
// disclosure view) and the ONE idempotent append. The store owns NO
// adapters — every transaction is a pure persistence boundary under
// the per-case advisory lock; the UNIQUE full request key backstops
// the idempotency the lock already serializes.
type OrderServicingRepository interface {
	// ReadCaseServicingRequests reads every request of one case ordered
	// by the monotonic request number (the panel feed — the recorded
	// history never rewrites).
	ReadCaseServicingRequests(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]domain.OrderServicingRequest, error)
	// LookupOrderByIDentifier finds the order operations of the TENANT
	// whose RECORDED material carries the named external ID: the
	// candidates carry the operation anchor, its case and the SOURCE
	// pins BY NAME (the receipt of the dispatch machine 4-6 and/or the
	// reread of the manager 4-8 for the ABCP kind; the human facts 4-9
	// for the supplier kind). NO enumeration of the orders exists —
	// the caller derives the outcome BY KEY (unique/not_found/
	// ambiguous). The kind word belongs to the fixed domain dictionary
	// (the phone/email kinds do NOT exist — OQ-16).
	LookupOrderByIDentifier(ctx context.Context, scope tenancy.Context, kind, value string) ([]domain.OrderLookupCandidate, error)
	// ReadResolvedOperation reads the operation row of the resolved
	// anchor (the LIVE material assembly of the disclosure view — the
	// projection of the FOUND operation re-derives read-time). The
	// honest miss (false) never fails the view: the client words of a
	// lost anchor answer the intent word only.
	ReadResolvedOperation(ctx context.Context, scope tenancy.Context, operationID id.UUID) (domain.Operation, bool, error)
	// AppendOrderServicingRequest appends ONE request under the per-case
	// advisory lock: the monotonic request_no derives under the lock
	// (the ladder of the repeated records); the UNIQUE full request key
	// (tenant, case, actor, intent, identifier_kind, identifier_value,
	// proof_state) backstops the IDEMPOTENCY BY KEY — the raced replay
	// of the same command at the same trust state refuses with
	// ErrOrderServicingRequestDuplicate and NEVER writes a second row
	// (the app re-reads the saved request and answers the honest replay
	// outcome; the changed trust state is a DIFFERENT key — a new row,
	// the ladder stays honest).
	AppendOrderServicingRequest(ctx context.Context, p AppendOrderServicingRequestParams) (domain.OrderServicingRequest, error)
}

// AssuranceSource is the narrow caller-authority assurance read of the
// contact case (the LIVE 2-3 derivation — the composition-root adapter
// over the cases API satisfies it, the CaseMaterialSource precedent):
// the derived state (the identity level/method/expiry with the step-up
// flag), the records with the canonical outcomes, the allowed methods
// of the conversation modality and the fixed fallback options. The
// Identity Proof Gate reads ONLY this surface — the Caller ID, the
// chat profile name and the knowledge of the order number are NEVER a
// proof (FR-11/2-3 structural: no other input exists). A NIL port is
// the HONEST unwired state: the record command refuses in words —
// never a masked «ok» proof pin.
type AssuranceSource interface {
	// AssuranceView answers the LIVE assurance derivation of one case
	// (the caller has ALREADY verified the actor's membership and
	// rights; the read is the gate input ONLY — the values never leak
	// into the client words).
	AssuranceView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID) (casesAPI.AssuranceView, error)
}
