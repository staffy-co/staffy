package api

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/control/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Clock is the app clock contract (the identity/employee api precedent):
// every mutation reads Now() once and canonicalizes to UTC at the store
// boundary; the tests inject a frozen clock.
type Clock interface {
	Now() time.Time
}

// IDGenerator is the app id contract (the identity/employee api
// precedent): the audit, outbox and correlation identifiers are minted
// BEFORE the transaction so the store stays a pure persistence boundary.
type IDGenerator interface {
	New(time.Time) (id.UUID, error)
}

// ControlService is the surface of the control module the transport
// type-asserts (the HandoffService/ActionCatalogService narrow-interface
// precedent): the four operator paths of the slice — list, effective,
// set, resume. The DTOs of this package are the ONLY shapes the transport
// renders; the domain records never leak to the wire directly.
type ControlService interface {
	// Set installs or re-installs a control point (one atomic transaction:
	// the row, the append-only audit row, the outbox event). The gates run
	// IN ORDER before any material: membership (404), the operations scope
	// (403 in words), step-up, then the dictionary guard. A generation
	// mismatch answers domain.ErrGenerationConflict carrying the CURRENT
	// material (AC-11).
	Set(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, request SetRequest) (domain.ScopedControl, error)

	// Resume lifts a control point (state removed, the generation grows,
	// the history stays). The same mutation gates as Set.
	Resume(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, request ResumeRequest) (domain.ScopedControl, error)

	// List returns every control point of the tenant (both states — the
	// history surface). The read gates: membership (404) then the
	// operations scope (403 in words); step-up is a MUTATION gate only.
	List(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]domain.ScopedControl, error)

	// EffectiveFor computes the most-restrictive intersection of the
	// controls matching the route scope keys, with the provenance of every
	// influencing control and the winning reason (AC-3). The read gates of
	// List; the keys are validated with the same dictionary discipline.
	EffectiveFor(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, keys []domain.ScopeKey) (domain.EffectiveControl, error)
}

// SetRequest is the mutation shape of the set path. CorrelationID carries
// the request correlation (the transport takes it from the platform
// correlation context); ExpectedGeneration 0 addresses a point that does
// not exist yet (the first installation).
type SetRequest struct {
	Kind, Ref, Mode, Reason string
	ExpectedGeneration      int64
	// ScheduledStartAt/ExpiresAt open and close the read-time window
	// [start, expires); nil means unbounded on that side.
	ScheduledStartAt, ExpiresAt *time.Time
	CorrelationID               id.UUID
}

// ResumeRequest is the mutation shape of the resume (lift) path.
type ResumeRequest struct {
	Kind, Ref, Reason  string
	ExpectedGeneration int64
	CorrelationID      id.UUID
}

// ScopedControlView is the wire shape of one control point: the mode
// spoken as a word, the raw stored word kept honest (the corruption
// marker below), the window and the generation. From maps the domain
// record; nothing is invented, nothing is dropped.
type ScopedControlView struct {
	// ScopeKind/ScopeRef — the addressable scope pair of the point.
	ScopeKind string `json:"scope_kind"`
	ScopeRef  string `json:"scope_ref"`
	// Mode is the SAFE read of the stored word: a corrupted value reads as
	// paused — the most safe applicable (AC-2, fail-closed reading).
	Mode string `json:"mode"`
	// Corrupted marks an out-of-dictionary stored word: the surface stays
	// honest about the damage it answered with paused.
	Corrupted bool `json:"corrupted"`
	// Reason — the operator's stated cause of the last change, verbatim.
	Reason string `json:"reason"`
	// ActorID — who made the last change.
	ActorID string `json:"actor_id"`
	// PrevMode — the mode before the last change ("" on the first set).
	PrevMode string `json:"prev_mode"`
	// Generation — the monotonic state version (never resets, grows on set
	// AND resume).
	Generation int64 `json:"generation"`
	// ScheduledStartAt/ExpiresAt — the read-time window [start, expires);
	// null means unbounded on that side.
	ScheduledStartAt *time.Time `json:"scheduled_start_at"`
	ExpiresAt        *time.Time `json:"expires_at"`
	// State — active or removed (a removed point is history, not absence).
	State string `json:"state"`
	// UpdatedAt — the time of the last change (RFC3339, UTC).
	UpdatedAt time.Time `json:"updated_at"`
}

// FromScopedControl maps the domain record onto the view: the safe mode
// read, the honest corruption marker, the defensive window copies.
func FromScopedControl(c domain.ScopedControl) ScopedControlView {
	return ScopedControlView{
		ScopeKind:        c.ScopeKind,
		ScopeRef:         c.ScopeRef,
		Mode:             domain.SafeReadMode(c.Mode),
		Corrupted:        !domain.WordIn(domain.ControlModes, c.Mode),
		Reason:           c.Reason,
		ActorID:          c.ActorID,
		PrevMode:         c.PrevMode,
		Generation:       c.Generation,
		ScheduledStartAt: copyWindow(c.ScheduledStartAt),
		ExpiresAt:        copyWindow(c.ExpiresAt),
		State:            c.State,
		UpdatedAt:        c.UpdatedAt,
	}
}

func copyWindow(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	copy := *t
	return &copy
}

// EffectiveControlView is the wire shape of the effective computation:
// the intersection mode, the influencing provenance, the winning reason,
// the words that met (ladder order) and the corruption marker.
type EffectiveControlView struct {
	Mode string `json:"mode"`
	// Influencing — every control that acts on the route at the read
	// moment (the full provenance, not only the winner).
	Influencing []ScopedControlView `json:"influencing"`
	// Reason — the stated reason of the MOST restrictive control (empty
	// when nothing restricts the route).
	Reason string `json:"reason"`
	// Intersection — the mode words that met, ladder order, deduplicated.
	Intersection []string `json:"intersection"`
	// Corrupted — at least one influencing control stored an
	// out-of-dictionary word (read as paused, never expanding allowance).
	Corrupted bool `json:"corrupted"`
}

// FromEffectiveControl maps the domain computation onto the view.
func FromEffectiveControl(e domain.EffectiveControl) EffectiveControlView {
	influencing := make([]ScopedControlView, 0, len(e.Influencing))
	for _, control := range e.Influencing {
		influencing = append(influencing, FromScopedControl(control))
	}
	intersection := append([]string(nil), e.Intersection...)
	return EffectiveControlView{
		Mode:         e.Mode,
		Influencing:  influencing,
		Reason:       e.Reason,
		Intersection: intersection,
		Corrupted:    e.Corrupted,
	}
}
