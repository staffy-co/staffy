package reconciliationsyn

import (
	"context"
	"sync"
	"time"

	actionPort "github.com/artemiosu/staffy/internal/action/port"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// The capability identifier the synthetic read-only adapter serves the
// reconciliation query under by default: the LIVE telephony declaration
// (command.send-sms of the 1-8 catalog — the same word the synthetic
// mutation adapter of 3-6 defaults to; the test-contour composition roots
// override it with NewServing to the channel word the fixture
// definitions actually require).
const sendSMSCapability = "command.send-sms"

// Synthetic is the TEST-CONTOUR read-only query adapter of the action
// reconciliation (AG-4: the production root wires NO adapter — no real
// owning systems exist; this package is imported ONLY by the fixture and
// the integration composition roots, NEVER by cmd/staffy-core, so the
// production binary structurally excludes it — the import-graph probe of
// step-04 pins it). The claimed verdicts are scriptable — the fixture
// toggles of the journeys and the scripted rows of the integration
// matrix walk every word of the four-word verdict dictionary; a scripted
// adapter error answers the honest no_answer of a broken call (the
// attempt records its guarded truth, the operation stays unknown); the
// committed evidence triple (the external object id/version/time) is
// scriptable for the structural evidence rows — the domain guard fails
// a claimed committed WITHOUT the complete triple, exactly as a lying
// owning system would.
//
// The port carries NO secrets and NO mutation method: the request is the
// derived key identities alone (the SAME external idempotency key as the
// dispatch, the last dispatch correlation, the keyed request digest);
// the answer is the claimed verdict word, the provider correlation, the
// evidence triple and the safe details. The adapter NEVER writes to the
// provider world — the reflect probe of the unit matrix pins the shape
// (epic AC-1).
type Synthetic struct {
	mu                  sync.Mutex
	verdict             string
	externalCorrelation string
	// servedCapability is the declared destination word (the composition
	// root of the test contour declares it — NewServing below).
	servedCapability      string
	externalObjectID      string
	externalObjectVersion string
	failure               error
}

// New constructs the synthetic read-only adapter with the default
// script: the owning system is still processing (in_progress — the
// honest intermediate word: no terminal claim, no evidence, the
// operation stays unknown). The served capability is the LIVE
// declaration word (command.send-sms — the catalog word the fixture
// composition root may override).
func New() *Synthetic {
	return &Synthetic{verdict: "in_progress", servedCapability: sendSMSCapability}
}

// NewServing constructs the synthetic read-only adapter serving ONE
// declared capability word of the test contour (the channel word the
// fixture definitions require — the query.trunk-status pattern of the
// 3-6 dispatch world: the channel word is the connection probe, the
// reconciliation semantics ride the definition's delivery contract and
// the connector's LIVE declaration word). The composition root of the
// test contour wires the adapter over THAT word; the app allowlist gate
// cross-checks it BEFORE the lease and the query (FENCE 5 of the light
// ladder).
func NewServing(capability string) *Synthetic {
	if capability == "" {
		capability = sendSMSCapability
	}
	return &Synthetic{verdict: "in_progress", servedCapability: capability}
}

// The structural pin: the synthetic satisfies the action read-only port
// contract (NOT a single mutation method — the reflect probe of the unit
// matrix walks the type).
var _ actionPort.ReconciliationAdapter = (*Synthetic)(nil)

// Script sets the claimed verdict word (the four-word domain dictionary
// — a foreign word answers verbatim, the domain normalization fails it
// closed into the honest no_answer), the scripted provider correlation
// (empty — minted from the EXTERNAL idempotency key of the operation:
// the same operation answers the same correlation on every query, the
// synthetic provider recognizes its own trace) and the optional adapter
// failure.
func (s *Synthetic) Script(verdict, externalCorrelation string, failure error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verdict = verdict
	s.externalCorrelation = externalCorrelation
	s.failure = failure
}

// ScriptEvidence scripts the STRUCTURAL evidence triple of the committed
// rows (the external object id and version — the time is the call
// moment); an empty id removes the evidence (the claimed committed fails
// the evidence guard into the honest no_answer — the State/Authority
// discipline rows of the matrix).
func (s *Synthetic) ScriptEvidence(externalObjectID, externalObjectVersion string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.externalObjectID = externalObjectID
	s.externalObjectVersion = externalObjectVersion
}

// ServesCapability reports the destination allowlist word this adapter
// serves (the app cross-checks it against the definition's required
// capability BEFORE the lease and the query — a foreign capability is
// the honest refusal without a call, FENCE 5).
func (s *Synthetic) ServesCapability() string {
	return s.servedCapability
}

// QueryOutcome answers ONE scripted reconciliation query: the request
// carries the derived key identities alone (the SAME external
// idempotency key as the dispatch of this operation — the provider
// recognizes the operation; the correlation id of the last dispatch
// attempt — the provider's own trace; the keyed request digest of the
// expected material); the answer carries the claimed verdict word, the
// provider correlation (the minted derivative of the key when the script
// leaves it empty — the same operation answers the same correlation) and
// the structural evidence triple of the committed claims. NO secret ever
// travels on this port; NO write is ever performed.
func (s *Synthetic) QueryOutcome(_ context.Context, _ tenancy.Context, request actionPort.ReconciliationQueryRequest) (actionPort.ReconciliationQueryResult, error) {
	s.mu.Lock()
	verdict, correlation, objectID, objectVersion, failure := s.verdict, s.externalCorrelation, s.externalObjectID, s.externalObjectVersion, s.failure
	s.mu.Unlock()
	if failure != nil {
		return actionPort.ReconciliationQueryResult{}, failure
	}
	if correlation == "" && len(request.ExternalIDempotencyKey) >= 16 {
		// The synthetic provider derives its correlation from the
		// EXTERNAL idempotency key of the operation: the same operation
		// answers the same correlation on every query (the provider-side
		// dedup, observed honestly).
		correlation = "syn-" + request.ExternalIDempotencyKey[:16]
	}
	var evidenceTime time.Time
	if verdict == "committed" && objectID != "" {
		// The structural evidence of a terminal success: the complete
		// triple (id, version, time) — the domain guard accepts the
		// committed claim ONLY with it (State/Authority: the terminal
		// success is never taken on word, even from the reconciliation).
		evidenceTime = time.Now().UTC()
	}
	return actionPort.ReconciliationQueryResult{
		ClaimedVerdict:        verdict,
		CorrelationID:         correlation,
		ExternalObjectID:      objectID,
		ExternalObjectVersion: objectVersion,
		ExternalObjectTime:    evidenceTime,
		Details:               "синтетический read-only адаптер сверки тестового контура: вердикт по сценарию",
	}, nil
}
