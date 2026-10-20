package dryrunner

import (
	"context"
	"sync"

	"github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
)

// ---------------------------------------------------------------------------
// The synthetic DryRunner of the TEST contour (Story 3.4).
// ---------------------------------------------------------------------------
//
// The production composition root NEVER imports this package (AG-4: no
// real providers exist — the app's nil runner refuses the dry-run in
// words and no evidence row is ever created). The adapter is the
// VALIDATION contract in its pure form: it carries NO write tool, it
// never touches the network, and its answer is the three-word result
// dictionary with the bounded safe details (the identifiers and words
// only — never a material value, never a secret). The journeys/fixture
// world wires the adapter and toggles the scripted outcomes per preview
// thread (the correlation the adapter may derive from the pinned
// identity — the same preview version always answers the same scripted
// result, the deterministic fixture discipline).

// Synthetic is the scriptable validation adapter of the test contour.
type Synthetic struct {
	mu       sync.Mutex
	outcomes map[string]actionPort.DryRunOutcome // preview_id:version → the scripted outcome
	fallback actionPort.DryRunOutcome
}

// New wires the synthetic adapter: the default script answers valid (the
// fixture world builds honest passing evidence; the toggles degrade
// specific versions on demand).
func New() *Synthetic {
	return &Synthetic{
		outcomes: map[string]actionPort.DryRunOutcome{},
		fallback: actionPort.DryRunOutcome{Result: domain.DryRunResultValid, Details: "синтетическая валидация прошла"},
	}
}

// The structural pin: the adapter satisfies the consumer-owned port.
var _ actionPort.DryRunner = (*Synthetic)(nil)

// SetOutcome scripts the answer of one EXACT preview version (the three
// word dictionary; a foreign word is refused by GuardDryRunResult at the
// app — the adapter never widens the vocabulary).
func (s *Synthetic) SetOutcome(previewID, versionNo, result, details string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outcomes[previewID+":"+versionNo] = actionPort.DryRunOutcome{Result: result, Details: details}
}

// Validate answers the scripted outcome of the pinned preview version
// (the fallback when no script exists). The call is the pure validation
// contract: no side effects exist at all — the evidence row is the APP's
// append-only event, never an adapter write.
func (s *Synthetic) Validate(_ context.Context, request actionPort.DryRunRequest) (actionPort.DryRunOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := request.PreviewID.String() + ":" + itoa(request.VersionNo)
	if scripted, ok := s.outcomes[key]; ok {
		return scripted, nil
	}
	return s.fallback, nil
}

// itoa renders the version number without the strconv import noise of a
// one-liner (the deterministic script key).
func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
