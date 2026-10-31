package controleffectiveresolver

import (
	"context"

	actionPort "github.com/staffy-co/staffy/internal/action/port"
	controlDomain "github.com/staffy-co/staffy/internal/control/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// RouteAdapter adapts the control store rows onto the action preparation
// port of Story 3.4 — the SAME composition package as the 3.3 drain fence
// (the "reused adapter" of the spec: one package, two consumer-owned
// ports). The preparation route of an action is the tenant point + the
// connector point of the definition's required capability + the
// autonomy-class point of the definition's conditions — the 3.2 scope
// kinds that NAME an action; the delivery-channel point of the 3.3 drain
// belongs to the delivery contour, not to the preparation.
//
// The honest boundary is the 3.3 one: the read is a MACHINE read behind
// the app's own rights gate (review|configuration) — the operator-styled
// gates of the control UI deliberately stay OUT (an enforcement fence of
// the preparation must never depend on the reader's UI authority), the
// rows read through the RLS tenant isolation, and the corrupted stored
// values read as paused through SafeReadMode (fail-closed — the
// allowance never expands on damage).
type RouteAdapter struct {
	store controlStoreReader
	clock Clock
}

// NewRouteAdapter wires the preparation-route adapter over the control
// store rows and the read clock.
func NewRouteAdapter(store controlStoreReader, clock Clock) *RouteAdapter {
	return &RouteAdapter{store: store, clock: clock}
}

// The structural pin: the adapter satisfies the action port contract.
var _ actionPort.ControlEffectiveReader = (*RouteAdapter)(nil)

// EffectiveFor resolves the most-restrictive effective control of the
// preparation route (the tenant + the connector of the definition's
// capability + the autonomy class of its conditions): the load of the
// route points, the domain intersection at the current moment, the safe
// four-word mode with the generation of the winning point. A storage
// failure is the loud error — the caller (the app policy gate) blocks
// fail-closed with the control_mode reason, never a guess.
func (a *RouteAdapter) EffectiveFor(ctx context.Context, tenantID id.UUID, route actionPort.ControlRoute) (actionPort.ControlEffective, error) {
	// The machine scope of the read: the tenant reads its own rows (the
	// RLS policy isolates by tenant_id; the principal slot carries the
	// tenant identifier — no operator identity participates in a
	// machine fence, and no UI gate can refuse the preparation).
	scope, err := tenancy.New(tenantID, tenantID)
	if err != nil {
		return actionPort.ControlEffective{}, err
	}
	controls, err := a.store.LoadKeys(ctx, scope, []controlDomain.ScopeKey{
		{Kind: controlDomain.ScopeKindTenant, Ref: tenantID.String()},
		{Kind: controlDomain.ScopeKindConnector, Ref: route.ConnectorID},
		{Kind: controlDomain.ScopeKindAutonomyClass, Ref: route.AutonomyClass},
	})
	if err != nil {
		return actionPort.ControlEffective{}, err
	}
	effective := controlDomain.ComputeEffective(a.clock.Now(), controls)
	reading := actionPort.ControlEffective{Mode: effective.Mode, Reason: effective.Reason}
	// The generation of the winning rule: the most recent word of the
	// winning mode among the influencing points (0 — honestly nothing
	// restricts the route).
	for _, control := range effective.Influencing {
		if controlDomain.SafeReadMode(control.Mode) == effective.Mode && control.Generation > reading.Generation {
			reading.Generation = control.Generation
		}
	}
	return reading, nil
}
