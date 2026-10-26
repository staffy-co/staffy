package api

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Story 4.11: the pilot outcome surface — the READ-ONLY derivation of
// the ten-stage inquiry funnel of one case (the case outcome panel) and
// the aggregate pilot report of one period (the pilot report page).
// The derivation writes NOTHING: the computed representation never
// becomes a new source of truth (AC-13 read-time — a repeated read of
// the same world answers the same; a late supplier fact of 4.9 changes
// the reading WITHOUT rewriting the append-only history). The terminal
// stages stay unknown_pending always in this slice (OQ-4/OQ-3 — «не
// доказано И не опровергнуто», never zero and never a silent
// exclusion, AC-14).

// PilotOutcomeService is the narrow Story 4.11 surface the transport
// type-asserts over the action service (the OrderStatusService
// precedent): the composite app Service implements it; a composition
// root that binds the action surface WITHOUT the pilot contour answers
// the honest degradation word of the unwired surface.
type PilotOutcomeService interface {
	// ReadPilotCaseOutcome derives the outcome of ONE case over the
	// recorded material (the panel view of the case page): the
	// attribution of the first accepted inbound channel event, the
	// ten-stage funnel with the evidence words, the outcome chain words
	// and the recorded counters. ZERO writes — the pure read-time fold.
	ReadPilotCaseOutcome(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) (domain.PilotCaseOutcome, error)
	// ReadPilotReport derives the aggregate pilot report of one period
	// (the report page): the listing of the cases of the period is
	// re-read case by case (the AG-1 envelope — the pilot volume is
	// small), every case outcome is derived by the SAME fold and the
	// aggregate carries the sample size, the period word, the stage
	// counts, the conversions between the PROVEN neighbours, the
	// counters, the unknown share and the Measurement Dictionary v1.
	// ZERO writes.
	ReadPilotReport(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, period string, now time.Time) (domain.PilotReport, error)
}
