package api

import (
	"context"
	"time"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 2.7: the owning-system read command and the ObservationView
// projection
// ---------------------------------------------------------------------------

// ReadOwningSystemCommand reads the CURRENT value of one material field of
// the authority matrix from the owning system of the Work Item scope: the
// operator with the review OR the configuration scope (the gate fires
// BEFORE the material guards — the M1 precedent of 2-5), the case and the
// Work Item anchors, the material field word of the matrix dictionary and
// the idempotency key. The read is ONE attempt with the fixed deadline
// word — no retry exists in the slice. The observed value is the snapshot
// of the read moment: it is NOT a fact and never confirms one.
type ReadOwningSystemCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	WorkItemID     id.UUID
	MaterialKey    string
	IdempotencyKey string
	Now            time.Time
}

// ObservationCard is one material field card of the observation view: the
// matrix row words (the owning system, the projection, the freshness
// seconds), the CURRENT observation of the field of the Work Item (the
// maximal number; nil when the field was never read), the read-time state
// derivation word (fresh/stale/failed/unknown — never stored) and the
// conflict badge word against the confirmed exact fact of the same material
// key (equal/differs/no_fact — the resolution stays with the human).
type ObservationCard struct {
	MaterialKey      string                   `json:"material_key"`
	OwningSystem     string                   `json:"owning_system"`
	Projection       string                   `json:"projection"`
	FreshnessSeconds int                      `json:"freshness_seconds"`
	Current          *casesDomain.Observation `json:"current"`
	State            string                   `json:"state"`
	ConflictBadge    string                   `json:"conflict_badge"`
}

// ObservationWorkItemView is one Work Item section of the observation view:
// the cards of every material field of the matrix (the honest per-field
// states — the failure of one owning system never masks another field).
type ObservationWorkItemView struct {
	WorkItemID id.UUID           `json:"work_item_id"`
	Fields     []ObservationCard `json:"fields"`
	// ObservationCount is the number of the observation rows of the item
	// (the honest anchor of the read form: it moves with every landed
	// observation — the PRG double-submit protection material).
	ObservationCount int `json:"observation_count"`
}

// ObservationView is the owning-system section of the case view: the cards
// per Work Item with the fixed matrix words and the honest per-field
// derivations (the CURRENT observation, the freshness state, the conflict
// badge), the field dictionary of the panel and the fixed matrix version
// word.
type ObservationView struct {
	Items         []ObservationWorkItemView `json:"items"`
	FieldWords    []string                  `json:"field_words"`
	MatrixVersion string                    `json:"matrix_version"`
	PanelText     string                    `json:"panel_text"`
}

// ObservationService is the command surface of Story 2.7 (the read command
// is idempotent by the tenant-scoped key; the observations are append-only
// with the per-(Work Item, material field) monotonic number; the freshness
// is the read-time derivation of the matrix word — never stored).
type ObservationService interface {
	ReadOwningSystem(context.Context, ReadOwningSystemCommand) (WorkOutcome, error)
}
