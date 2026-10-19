package controleffectiveresolver

import (
	"context"
	"time"

	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	controlDomain "github.com/artemiosu/staffy/internal/control/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// Clock is the read-time clock of the effective computation (the window
// filter reads the current moment).
type Clock interface{ Now() time.Time }

// controlStoreReader is the narrow read the adapter consumes (the
// control Postgres store satisfies it; the adapter never touches the
// writes or the operator-styled gates).
type controlStoreReader interface {
	LoadKeys(ctx context.Context, scope tenancy.Context, keys []controlDomain.ScopeKey) ([]controlDomain.ScopedControl, error)
}

// Adapter resolves the effective route control for the drain fence — the
// composition-root wiring of the promised 3.2 enforcement point (the
// packresolver/connectorrefresolver precedent of 3.1/3.2): the cases
// module consumes the consumer-owned port, this adapter imports the
// control store and the control domain in the root.
//
// The honest boundary: the read is a MACHINE read behind the drain's own
// configuration gate — the operator-styled gates of the control app
// service (the operations scope of the control UI) deliberately stay
// OUT: the drain operator already passed the drain gate, and an
// enforcement fence must never depend on the reader's UI authority (a
// configuration-only operator drains honestly; the fence reads the
// tenant rows through the RLS tenant isolation, the way the store reads
// every cases material). The route keys of the fence are the tenant point
// and the channel point of the delivery route; the corrupted stored
// values read as paused through SafeReadMode (fail-closed, the 3.2
// discipline — the allowance never expands on damage).
type Adapter struct {
	store controlStoreReader
	clock Clock
}

// New wires the adapter over the control store rows and the read clock.
func New(store controlStoreReader, clock Clock) *Adapter {
	return &Adapter{store: store, clock: clock}
}

// The structural pin: the adapter satisfies the cases port contract.
var _ casesPort.ControlEffectiveReader = (*Adapter)(nil)

// EffectiveFor resolves the most-restrictive effective control of the
// delivery route (the tenant + the channel of the version): the load of
// the route points, the domain intersection at the current moment, the
// accept word of an honestly unrestricted route. A storage failure is the
// loud error — the caller (the drain) gases fail-closed, never a guess.
func (a *Adapter) EffectiveFor(ctx context.Context, tenantID id.UUID, channel string) (casesPort.ExternalControlMode, error) {
	// The machine scope of the read: the tenant reads its own rows (the
	// RLS policy isolates by tenant_id; the principal slot carries the
	// tenant identifier — no operator identity participates in a machine
	// fence, and no UI gate can refuse the enforcement point).
	scope, err := tenancy.New(tenantID, tenantID)
	if err != nil {
		return casesPort.ExternalControlMode{}, err
	}
	controls, err := a.store.LoadKeys(ctx, scope, []controlDomain.ScopeKey{
		{Kind: controlDomain.ScopeKindTenant, Ref: tenantID.String()},
		{Kind: controlDomain.ScopeKindChannel, Ref: channel},
	})
	if err != nil {
		return casesPort.ExternalControlMode{}, err
	}
	effective := controlDomain.ComputeEffective(a.clock.Now(), controls)
	mode := casesPort.ExternalControlMode{Mode: effective.Mode, Reason: effective.Reason}
	// The generation of the winning rule: the most recent word of the
	// winning mode among the influencing points (0 — honestly nothing
	// restricts the route).
	for _, control := range effective.Influencing {
		if controlDomain.SafeReadMode(control.Mode) == effective.Mode && control.Generation > mode.Generation {
			mode.Generation = control.Generation
		}
	}
	return mode, nil
}
