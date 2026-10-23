package api

import (
	"context"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 2.10: the continuation commands and the surface views
// ---------------------------------------------------------------------------

// IssueContinuationLinkCommand issues one purpose-bound continuation
// link of the Work Item: the purpose set (the v1 dictionary subset), the
// open question the client answers on the protected surface, the
// transition explanation, the participant reference (the channel address
// of the client the operator weaves the URL to), the lifetime bounds
// (the ttl minutes and the consumption limit of the accepted fact
// corrections — the default 1 is the FR-16 «exactly one submission»).
// The capability token is generated server-side: the command NEVER
// carries it (the database stores only the SHA-256 of the random half).
type IssueContinuationLinkCommand struct {
	Actor            domain.Principal
	CorrelationID    id.UUID
	TenantID         id.UUID
	CaseID           id.UUID
	WorkItemID       id.UUID
	LinkID           id.UUID
	Purposes         []string
	Question         string
	Explanation      string
	ParticipantRef   string
	TTLMinutes       int
	ConsumptionLimit int
	IdempotencyKey   string
	Now              time.Time
}

// RevokeContinuationLinkCommand revokes one continuation link: the
// append-only revocation row with the honest reason («прекратить
// ожидание»); nothing is ever mutated — the read-time derivation blocks
// the surface without disclosure.
type RevokeContinuationLinkCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	WorkItemID     id.UUID
	LinkID         id.UUID
	RevocationID   id.UUID
	Reason         string
	IdempotencyKey string
	Now            time.Time
}

// ContinuationIssuance is the answer of the fresh issuance: the work
// outcome (the stored replay material — the token NEVER enters it) plus
// the one-time capability material of the protected surface: the URL
// token and its expiry. The token exists ONLY in this answer and on the
// operator side (the reply text); a replayed command returns the stored
// outcome with the empty URL and the honest mask word — the old link
// never resurrects, a lost URL means a new link and a revocation.
type ContinuationIssuance struct {
	Outcome WorkOutcome `json:"outcome"`
	URL     string      `json:"url,omitempty"`
	Token   string      `json:"token,omitempty"`
	Expires time.Time   `json:"expires_at,omitempty"`
}

// ContinuationSurface is the material of the protected continuation
// page: the header (the tenant display name of the identity service —
// the page honestly shows whose conversation it is; the role word of
// the slice is the «ИИ-сотрудник» constant), the explanation of the
// transition, the confirmed facts of the Work Item of the link (the
// correctable select — only this Work Item, never the other items or
// the other participants), the open question and the state of the
// moment (the consumed word renders the terminal page, the non-valid
// words render the single generic refusal — the causes are never
// distinguished to the client).
type ContinuationSurface struct {
	Kind        string // valid | consumed | invalid
	TenantName  string
	Explanation string
	Question    string
	Facts       []ContinuationFactView
	Vehicle     *ContinuationVehicleView
	ExpiresAt   time.Time
	LimitLeft   int
	Submitted   bool
}

// ContinuationFactView is one confirmed fact of the surface select: the
// material key with the human rendering of the confirmed value (the
// correction writes a NEW unverified version — the operator confirms
// through the штатный 2.2 command, the surface never confirms by
// itself).
type ContinuationFactView struct {
	FactID      id.UUID
	MaterialKey string
	ValueText   string
}

// ContinuationView is the operator panel section of the case view: the
// links with the derived states, the delivered binding, the use
// accounting and the event feed (the FR-16 statuses — «the main channel
// shows whether the link was delivered, the answer expected, the
// submission finished or the term expired» — are the read-time
// derivations, never stored words).
type ContinuationView struct {
	PanelText string                 `json:"panel_text"`
	Links     []ContinuationLinkView `json:"links"`
	Uses      []ContinuationUseView  `json:"uses"`
}

// ContinuationLinkView is one link of the operator panel: the
// identifiers, the purpose words, the derived state word, the delivered
// badge, the use accounting against the limit and the expiry moment.
// The token is never here — the panel shows the honest «токен выдан»
// mask of the issued capability.
type ContinuationLinkView struct {
	LinkID           id.UUID   `json:"link_id"`
	WorkItemID       id.UUID   `json:"work_item_id"`
	Purposes         []string  `json:"purposes"`
	State            string    `json:"state"`
	Delivered        bool      `json:"delivered"`
	AcceptedUses     int       `json:"accepted_uses"`
	ConsumptionLimit int       `json:"consumption_limit"`
	Question         string    `json:"question"`
	ParticipantRef   string    `json:"participant_ref"`
	ExpiresAt        time.Time `json:"expires_at"`
}

// ContinuationUseView is one client action of the event feed.
type ContinuationUseView struct {
	UseID       id.UUID   `json:"use_id"`
	LinkID      id.UUID   `json:"link_id"`
	UseKind     string    `json:"use_kind"`
	FactID      id.UUID   `json:"fact_id,omitempty"`
	FactVersion int64     `json:"fact_version_no,omitempty"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// ContinuationURLPath renders the URL path of one opaque token (the
// public surface route — the token is the only credential).
func ContinuationURLPath(token string) string {
	return "/continue/" + token
}

// ContinuationRoleWord is the honest role constant of the slice: the
// protected surface names the conversation party as the AI employee
// (the employee contour of the acting release is NOT read by the public
// page — the honest boundary; the per-employee naming arrives with the
// later slices).
const ContinuationRoleWord = "ИИ-сотрудник"

// The surface kind words (the page material discriminator; the invalid
// word covers every refusal cause — unknown token, foreign tenant,
// expired, revoked, owner-stale — the client never sees which).
const (
	ContinuationSurfaceValid    = "valid"
	ContinuationSurfaceConsumed = "consumed"
	ContinuationSurfaceInvalid  = "invalid"
)

// VehicleConfirmationItem is one attribute confirmation of the Story
// 4.2 submission (the machine form of the public POST): the fact anchor
// and the EXACT version the client confirmed on the protected form.
type VehicleConfirmationItem struct {
	FactID    id.UUID `json:"fact_id"`
	VersionNo int64   `json:"version_no"`
}

// ContinuationService is the command and surface interface of Story 2.10
// (the two operator commands are idempotent by the tenant-scoped key with
// the stored outcome — the token NEVER enters the stored material; the
// public surface reads and writes carry the opaque token as the only
// credential — no session, no principal; the transport type-asserts this
// narrow interface of the cases service — the BindEmployee precedent, the
// composite Service interface stays untouched).
type ContinuationService interface {
	IssueContinuationLink(context.Context, IssueContinuationLinkCommand) (ContinuationIssuance, error)
	RevokeContinuationLink(context.Context, RevokeContinuationLinkCommand) (WorkOutcome, error)
	ContinueSurface(context.Context, string, time.Time) (ContinuationSurface, error)
	SubmitCorrection(context.Context, string, id.UUID, []byte, time.Time) (int64, error)
	// SubmitVehicleConfirmation is the Story 4.2 client confirmation of
	// the vehicle identification attributes: the opaque token is the only
	// credential (the same public-surface discipline); the confirmation
	// binds the EXACT versions with confirmer=client and consumes the
	// link limit in one transaction.
	SubmitVehicleConfirmation(context.Context, string, []VehicleConfirmationItem, time.Time) error
	SubmitHuman(context.Context, string, time.Time) error
	SubmitReturn(context.Context, string, time.Time) error
}

// continuationStateWordsOf keeps the dictionary honest for the view
// rendering (the state words come from the domain derivation).
var _ = casesDomain.ContinuationStateWords
