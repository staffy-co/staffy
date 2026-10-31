package actiondispatcher

import (
	"context"
	"sync"
	"time"

	actionPort "github.com/staffy-co/staffy/internal/action/port"
	"github.com/staffy-co/staffy/internal/connector/catalog"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// The capability identifier the synthetic mutation adapter serves the
// dispatch under: the LIVE telephony declaration (command.send-sms of
// the 1-8 catalog — Idempotent: true, ReceiptLevel: committed).
const sendSMSCapability = "command.send-sms"

// Synthetic is the TEST-CONTOUR mutation adapter of the action dispatch
// (AG-4: the production root wires NO adapter — no real providers exist;
// this package is imported ONLY by the fixture and the integration
// composition roots, NEVER by cmd/staffy-core, so the production binary
// structurally excludes it — the import-graph probe of step-04 pins it).
// The outcomes are scriptable — the fixture toggles of the journeys and
// the scripted rows of the integration matrix walk every word of the
// four-word outcome dictionary; a scripted adapter error answers the
// possible-send ambiguity of a broken call (the worker records the
// honest unknown without a guarantee); the committed evidence triple
// (the external object id/version/time) is scriptable for the structural
// evidence rows — the guard of the domain fails a claimed committed
// WITHOUT the complete triple, exactly as a lying provider would.
//
// The idempotency guarantee answers from the LIVE capability declaration
// of command.send-sms (the catalog read live on every call — a future
// edition of the declaration reflects without a restart, the honest
// provenance instead of a test-side constant). The port carries NO
// secrets: the request is the rebuilt material plus the derived key
// identities; the answer is the claim word, the correlation, the
// evidence and the guarantee flag.
type Synthetic struct {
	mu                  sync.Mutex
	outcome             string
	externalCorrelation string
	// servedCapability is the declared destination word (the composition
	// root of the test contour declares it — NewServing above).
	servedCapability      string
	externalObjectID      string
	externalObjectVersion string
	failure               error
	// guaranteeOverride scripts the recorded guarantee word OFF the
	// live declaration (the timeout-without-guarantee rows of the
	// matrix); nil answers the LIVE declaration of the served
	// capability.
	guaranteeOverride *bool
}

// New constructs the synthetic mutation adapter with the default script:
// the provider accepts the request (the accepted reception of AC-6 —
// the non-terminal word without a receipt). The served capability is
// the LIVE mutating declaration (command.send-sms — the catalog word
// the guarantee read resolves).
func New() *Synthetic {
	return &Synthetic{outcome: "accepted", servedCapability: sendSMSCapability}
}

// NewServing constructs the synthetic mutation adapter serving ONE
// declared capability word of the test contour (the channel word the
// dispatchable definitions require). The honest boundary the step-04
// matrix caught LIVE: a MUTATING capability (command.send-sms) is never
// probe-verifiable before the Action Gateway exists (the frozen 3-4
// capability gate — capability_not_verified), so the operations that
// pass the authorize-time gates ride the VERIFIED read-only channel
// capability (the notify.manual-only pattern: the channel word is the
// connection probe, the mutation semantics ride the definition's
// delivery contract). The composition root of the test contour wires
// the adapter over THAT word; the LIVE guarantee read resolves the
// served word's catalog declaration (the guarantee rows without a live
// declaration ride the documented ScriptGuarantee override — the 3-3
// precedent).
func NewServing(capability string) *Synthetic {
	if capability == "" {
		capability = sendSMSCapability
	}
	return &Synthetic{outcome: "accepted", servedCapability: capability}
}

// The structural pin: the synthetic satisfies the action port contract.
var _ actionPort.MutationAdapter = (*Synthetic)(nil)

// Script sets the outcome word (the four-word domain dictionary — a
// foreign word answers verbatim, the domain guard refuses it fail-closed
// and records the honest unknown), the scripted provider correlation
// (empty — minted from the EXTERNAL idempotency key of the operation:
// every attempt of one operation carries the SAME key, the synthetic
// provider deduplicates it) and the optional adapter failure.
func (s *Synthetic) Script(outcome, externalCorrelation string, failure error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outcome = outcome
	s.externalCorrelation = externalCorrelation
	s.failure = failure
}

// ScriptEvidence scripts the STRUCTURAL evidence triple of the committed
// rows (the external object id and version — the time is the call
// moment); an empty id removes the evidence (the claimed committed fails
// the evidence guard into the honest unknown — the State/Authority
// discipline rows of the matrix).
func (s *Synthetic) ScriptEvidence(externalObjectID, externalObjectVersion string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.externalObjectID = externalObjectID
	s.externalObjectVersion = externalObjectVersion
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

// ServesCapability reports the destination allowlist word this adapter
// serves (the app cross-checks it against the definition's required
// capability BEFORE the lease and the call — a foreign capability is the
// honest refusal without a send, AC-4).
func (s *Synthetic) ServesCapability() string {
	return s.servedCapability
}

// liveGuarantee resolves the idempotency guarantee from the LIVE
// declaration of the served capability (the stored word of the catalog —
// the recorded fact the retry policy reads, never a test-side guess).
func liveGuarantee(capability string) bool {
	for _, definition := range catalog.List() {
		for _, candidate := range definition.Capabilities {
			if candidate.ID == capability {
				return candidate.Idempotent
			}
		}
	}
	return false
}

// Dispatch answers one scripted send attempt: the request carries the
// deterministically rebuilt payload of the immutable rows, the attempt
// number, the EXTERNAL idempotency key (ONE key per operation, stable
// across attempts — the provider deduplicates the repeats) and the keyed
// request digest; the answer carries the claim word, the provider
// correlation (the minted derivative of the key when the script leaves
// it empty — the same operation answers the same correlation), the
// structural evidence triple of the committed claims and the LIVE
// guarantee word. NO secret ever travels on this port.
func (s *Synthetic) Dispatch(_ context.Context, _ tenancy.Context, request actionPort.MutationRequest) (actionPort.MutationResult, error) {
	s.mu.Lock()
	outcome, correlation, objectID, objectVersion, failure, override := s.outcome, s.externalCorrelation, s.externalObjectID, s.externalObjectVersion, s.failure, s.guaranteeOverride
	s.mu.Unlock()
	if failure != nil {
		return actionPort.MutationResult{}, failure
	}
	if correlation == "" && len(request.ExternalIDempotencyKey) >= 16 {
		// The synthetic provider derives its correlation from the
		// EXTERNAL idempotency key of the operation: the same
		// operation answers the same correlation on every attempt
		// (the provider-side dedup, observed honestly).
		correlation = "syn-" + request.ExternalIDempotencyKey[:16]
	}
	guarantee := liveGuarantee(s.servedCapability)
	if override != nil {
		// The scripted override of the guarantee word (the matrix rows
		// where the provider declares no idempotency).
		guarantee = *override
	}
	var evidenceTime time.Time
	if outcome == "committed" && objectID != "" {
		// The structural evidence of a terminal success: the complete
		// triple (id, version, time) — the domain guard accepts the
		// committed claim ONLY with it.
		evidenceTime = time.Now().UTC()
	}
	return actionPort.MutationResult{
		Outcome:               outcome,
		CorrelationID:         correlation,
		ExternalObjectID:      objectID,
		ExternalObjectVersion: objectVersion,
		ExternalObjectTime:    evidenceTime,
		GuaranteeLive:         guarantee,
		Details:               "синтетический мутационный адаптер тестового контура: исход по сценарию",
	}, nil
}
