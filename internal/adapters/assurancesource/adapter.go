package assurancesource

import (
	"context"

	actionPort "github.com/staffy-co/staffy/internal/action/port"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 4.10 composition-root adapter over the cases API: the
// Identity Proof Gate of the order servicing contour reads the LIVE
// 2-3 assurance derivation of the contact case through the narrow
// consumer-owned port. The adapter projects the assurance section of
// the case view onto the port (the CaseMaterialSource precedent — the
// cases api Service satisfies the reader interface structurally; the
// caller has ALREADY verified the actor's membership and rights).
//
// The honest boundary stays the 2-3 one: the adapter reads ONLY the
// assurance section (the derived state with the records, the allowed
// methods of the conversation modality and the fallback options) —
// the Identifier Proof Gate NEVER reads the Caller ID, the chat
// profile name or any claimed identifier (FR-11 structural: no other
// input exists on this path), and the values never leak into the
// client words of the servicing view.

// caseViewer is the narrow read the adapter needs (the cases api
// Service satisfies it structurally — the same CaseView the
// CaseMaterialSource precedent reads).
type caseViewer interface {
	CaseView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID) (casesAPI.CaseView, error)
}

// Adapter projects the case view's assurance section onto the action
// servicing port (the composition root wires it over the live cases
// service; the test contours wire it over their fixtures).
type Adapter struct {
	cases caseViewer
}

// New wires the adapter over the live cases service (or any case
// viewer of the test contour).
func New(cases caseViewer) *Adapter {
	return &Adapter{cases: cases}
}

// The structural pin: the adapter satisfies the consumer-owned port
// contract of the servicing contour.
var _ actionPort.AssuranceSource = (*Adapter)(nil)

// AssuranceView answers the LIVE assurance derivation of one case (the
// gate input ONLY): the derived state (the identity level/method/
// expiry with the step-up flag), the records with the canonical
// outcomes, the allowed methods of the conversation modality and the
// fixed fallback options. A case read refusal propagates honestly —
// the record command refuses, NEVER a masked «ok» proof pin.
func (a *Adapter) AssuranceView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID) (casesAPI.AssuranceView, error) {
	view, err := a.cases.CaseView(ctx, actor, tenantID, caseID)
	if err != nil {
		return casesAPI.AssuranceView{}, err
	}
	return view.Assurance, nil
}
