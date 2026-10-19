package port

import (
	"context"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 3.3: the dispatch ports (consumer-owned, the DAG discipline)
// ---------------------------------------------------------------------------
//
// The cases module CONSUMES these narrow interfaces; the concrete adapters
// live in the composition root (the packresolver/connectorrefresolver
// precedent — the DAG never lets a case import a provider or the control
// module). The ExternalDispatcher is the machine boundary of the channel
// world; the ControlEffectiveReader is the promised 3.2 enforcement point
// (the drain reads the effective control of the route before the words
// leave). Both ports are optional in the wiring — and both absences are
// HONEST: a missing dispatcher keeps the records queued (the AG-4
// production boundary — the drain refuses in words), a missing or
// unresolvable control reader is the fail-closed gasp (the emergency
// brake: the words never leave on an unreadable control).

// ExternalDispatchResult is the honest answer of one dispatch call: the
// four-word outcome (accepted, delivered, failed, timeout — the domain
// dictionary), the external correlation id of the submitted/unknown
// evidence (AC-8), the capability id the adapter served the send under
// (the safe provenance of the events), the idempotency guarantee the
// capability DECLARED with this outcome (the safe-retry policy word — a
// recorded fact of the attempt, never a guess) and the bounded safe
// details (the machine words only, never the content).
type ExternalDispatchResult struct {
	Outcome               string
	ExternalCorrelationID string
	CapabilityID          string
	IdempotentGuarantee   bool
	Details               string
}

// ExternalDispatcher is the consumer-owned dispatch port: the drain hands
// the deterministic outbound payload (built from the immutable version
// row) and receives the outcome words. The payload carries the
// idempotency key derivative of the version pin — the repeated delivery
// of the same version presents the SAME key (the provider deduplicates),
// a changed version never reuses the old key.
type ExternalDispatcher interface {
	// Dispatch performs one send attempt. The scope carries the tenant
	// (a multi-tenant adapter routes by it); the principal slot is the
	// drain operator.
	Dispatch(ctx context.Context, scope tenancy.Context, payload casesDomain.OutboundExternalCommitment) (ExternalDispatchResult, error)
}

// ExternalControlAccept is the only passing control word of the drain
// fence (the 3.2 dictionary verbatim — the port owns the contract word;
// every other mode, including a corrupted value that READS as paused, is
// the fail-closed gasp).
const ExternalControlAccept = "accept"

// ExternalControlMode is the effective control word of one route: the
// four-word 3.2 dictionary READ SAFELY (a corrupted stored value reads as
// paused — the most safe applicable, fail-closed by reading), the
// generation of the winning control (0 — honestly nothing restricts) and
// the stated reason of the winner (the safe provenance material).
type ExternalControlMode struct {
	Mode       string
	Generation int64
	Reason     string
}

// ControlEffectiveReader is the consumer-owned read of the scoped control
// of one route (the tenant + the channel of the delivery): the promised
// 3.2 enforcement point. The composition adapter maps this onto the live
// control EffectiveFor with the route scope keys — the case module never
// imports the control module (the DAG); a nil port or a resolution error
// is the fail-closed gasp (the words never leave on an unreadable
// control).
type ControlEffectiveReader interface {
	// EffectiveFor resolves the most-restrictive effective control of
	// the route (tenant, channel) — accept is the only passing word.
	EffectiveFor(ctx context.Context, tenantID id.UUID, channel string) (ExternalControlMode, error)
}
