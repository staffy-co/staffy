package owningsystem

import (
	"context"
	"sync"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
)

// ---------------------------------------------------------------------------
// The synthetic OwningSystemReader of the TEST contour (Story 3.4).
// ---------------------------------------------------------------------------
//
// The production composition root NEVER imports this package (AG-4: no
// real providers exist — the app's nil reader keeps the readiness
// honestly blocked_material). The journeys/fixture world wires this
// adapter and drives the material statuses through the toggles: one
// status word per material field, deterministic rows (the fresh evidence
// references and the external identities the freshness derivations
// re-check), and the fail-closed discipline of the reader contract (the
// four-word statuses only — the app re-maps every row through
// MapSnapshotStatus against the definition's declared tolerance, so this
// adapter can never smuggle the allowed_uncertain word in even if it
// tried; a foreign toggled word reads as unknown at the mapping).

// Synthetic is the scriptable material reader of the test contour.
type Synthetic struct {
	mu       sync.Mutex
	statuses map[string]string // material field → the four-word reader status
}

// New wires the synthetic reader with the honest default: every
// requested field answers fresh (the fixture world builds ready
// previews; the toggles degrade specific fields on demand).
func New() *Synthetic {
	return &Synthetic{statuses: map[string]string{}}
}

// The structural pin: the adapter satisfies the consumer-owned port.
var _ actionPort.OwningSystemReader = (*Synthetic)(nil)

// SetStatus toggles the reader status of one material field (the
// four-word contract dictionary; a foreign word reads as unknown at the
// app mapping — fail-closed, the adapter never widens the allowance).
func (s *Synthetic) SetStatus(materialField, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses[materialField] = status
}

// FreshReads answers one deterministic material row per requested field:
// the status from the toggles (fresh by default), the external identity
// words derived from the field and the work item anchor, the current
// observation time and the evidence reference the freshness derivations
// re-check. ONE attempt per command: no retry, no cache (the reader
// contract of the port).
func (s *Synthetic) FreshReads(_ context.Context, request actionPort.FreshReadRequest) ([]domain.MaterialSnapshotRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]domain.MaterialSnapshotRow, 0, len(request.MaterialFields))
	for _, field := range request.MaterialFields {
		status := domain.SnapshotFresh
		if toggled, ok := s.statuses[field]; ok {
			status = toggled
		}
		rows = append(rows, domain.MaterialSnapshotRow{
			MaterialField:   field,
			Status:          status,
			ExternalID:      "synthetic-" + field + "-" + request.WorkItemID.String(),
			ExternalVersion: "synthetic-1",
			ObservedAt:      time.Now().UTC(),
			EvidenceRef:     request.OwningSystem + "://" + field + "/" + request.WorkItemID.String(),
		})
	}
	return rows, nil
}
