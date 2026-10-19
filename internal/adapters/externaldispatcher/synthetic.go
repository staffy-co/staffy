package externaldispatcher

import (
	"context"
	"sync"

	"github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/connector/catalog"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// The capability identifier the synthetic dispatcher serves the send
// under: the LIVE telephony declaration (command.send-sms of the 1-8
// catalog — Idempotent: true, ReceiptLevel: committed).
const sendSMSCapability = "command.send-sms"

// Synthetic is the TEST-CONTOUR dispatcher of the external commitment
// dispatch (AG-4: the production root wires NO dispatcher — no real
// providers exist; this package is imported ONLY by the fixture and the
// integration worlds, NEVER by cmd/staffy-core, so the production binary
// structurally excludes it). The outcomes are scriptable — the fixture
// toggles of the journeys and the scripted rows of the integration
// matrix walk every word of the four-word outcome dictionary; a scripted
// adapter error answers the possible-send ambiguity of a broken call (the
// drain maps it to the honest attempt_unknown without a guarantee).
//
// The idempotency guarantee answers from the LIVE capability declaration
// of command.send-sms (the catalog read live on every call — a future
// edition of the declaration reflects without a restart, the honest
// provenance instead of a test-side constant). The timeout scenario is
// the scripted timeout word (the drain consumes the word — the
// possible-send ambiguity); an optional delay can stretch the call for
// the latency rows of the matrix, the default stays zero.
type Synthetic struct {
	mu                  sync.Mutex
	outcome             string
	externalCorrelation string
	failure             error
	// guaranteeOverride scripts the recorded guarantee word OFF the
	// live declaration (the timeout-without-guarantee rows of the
	// matrix); nil answers the LIVE declaration of the served
	// capability.
	guaranteeOverride *bool
}

// New constructs the synthetic dispatcher with the default script: the
// channel accepts the request (the submitted evidence of AC-8).
func New() *Synthetic {
	return &Synthetic{outcome: domain.DispatchOutcomeAccepted}
}

// The structural pin: the synthetic satisfies the cases port contract.
var _ casesPort.ExternalDispatcher = (*Synthetic)(nil)

// Script sets the outcome word (the four-word domain dictionary), the
// scripted external correlation id (empty — minted from the idempotency
// key of the version) and the optional adapter failure. A foreign outcome
// word answers verbatim — the drain guard refuses it fail-closed (the
// machine boundary discipline; the synthetic never sanitises).
func (s *Synthetic) Script(outcome, externalCorrelation string, failure error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outcome = outcome
	s.externalCorrelation = externalCorrelation
	s.failure = failure
}

// ScriptGuarantee overrides the recorded guarantee word of the next
// attempts (the timeout-without-guarantee rows): the override lives
// until the next call — the default (nil) answers the LIVE capability
// declaration.
func (s *Synthetic) ScriptGuarantee(guaranteed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value := guaranteed
	s.guaranteeOverride = &value
}

// liveGuarantee resolves the idempotency guarantee from the LIVE
// declaration of the served capability (the stored word of the catalog —
// the recorded fact the drain policy reads, never a test-side guess).
func liveGuarantee() bool {
	for _, definition := range catalog.List() {
		for _, capability := range definition.Capabilities {
			if capability.ID == sendSMSCapability {
				return capability.Idempotent
			}
		}
	}
	return false
}

// Dispatch answers one scripted send attempt: the deterministic payload
// of the immutable version arrives with the idempotency key derivative
// of the version pin (the repeated attempt of the same version carries
// the SAME key — the synthetic honours it), the answer carries the
// outcome word, the external correlation (the minted derivative of the
// key when the script leaves it empty — the same version, the same
// correlation), the capability provenance and the LIVE guarantee word.
func (s *Synthetic) Dispatch(_ context.Context, _ tenancy.Context, payload domain.OutboundExternalCommitment) (casesPort.ExternalDispatchResult, error) {
	s.mu.Lock()
	outcome, correlation, failure, override := s.outcome, s.externalCorrelation, s.failure, s.guaranteeOverride
	s.mu.Unlock()
	if failure != nil {
		return casesPort.ExternalDispatchResult{}, failure
	}
	if correlation == "" && len(payload.IdempotencyKey) >= 16 {
		// The synthetic provider derives its correlation from the
		// idempotency key: the same version answers the same correlation
		// (the dedup of the provider, observed honestly).
		correlation = "syn-" + payload.IdempotencyKey[:16]
	}
	guarantee := liveGuarantee()
	if override != nil {
		// The scripted override of the guarantee word (the matrix rows
		// where the provider declares no idempotency).
		guarantee = *override
	}
	return casesPort.ExternalDispatchResult{
		Outcome:               outcome,
		ExternalCorrelationID: correlation,
		CapabilityID:          sendSMSCapability,
		IdempotentGuarantee:   guarantee,
		Details:               "синтетический диспетчер тестового контура: исход по сценарию",
	}, nil
}
