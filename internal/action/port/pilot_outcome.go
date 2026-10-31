package port

import (
	"context"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// Story 4.11: the pilot outcome read contour. PilotCaseSource is the
// narrow case surface the pilot outcome reads through (the
// CaseMaterialSource precedent of the preparation contour): the case
// listing of the period (the pilot report sample), the full case view
// (the case anchor, the timeline with the accepted inbound channel
// events of the attribution, the vehicle/part/fitment panels of
// 4.2..4.4, the work items and the handoff requests) and the facts view
// (the live fact versions of the case). The concrete cases api Service
// satisfies it STRUCTURALLY (the composition-root adapter by the
// CaseMaterialSource/AssuranceSource precedent — no glue at all); the
// caller has ALREADY verified the actor's membership and the review
// scope. A NIL port is the HONEST zero degradation: the panel and the
// report page render their refusal word — NEVER an invented material
// and never a nil panic.
type PilotCaseSource interface {
	// CasesView answers the listing of the cases of the tenant (the
	// sample the pilot report re-reads per period — the AG-1 envelope:
	// the pilot volume is small, the listing read is the honest cost).
	CasesView(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (casesAPI.CasesView, error)
	// CaseView answers the full read-time view of one case (the panel
	// material: the timeline attribution events, the vehicle/part/
	// fitment panels, the work items, the handoff requests).
	CaseView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID) (casesAPI.CaseView, error)
	// FactsView answers the fact versions of one case (the review/audit
	// boundary of the values — the read carries NO write).
	FactsView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID) (casesAPI.FactsView, error)
}
