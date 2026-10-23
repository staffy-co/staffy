package api

import (
	"time"

	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 4.4: the fitment evidence record command
// ---------------------------------------------------------------------------

// RecordFitmentEvidenceCommand records ONE manager's catalog research
// result as the append-only fitment evidence (AC-2): the operator-manager
// with the review OR the configuration scope (the gate fires BEFORE the
// material guards — the M1 precedent), the work item of the case, the
// canonical position key (the normalized article rendering — the 4.3
// number discipline), the found/not-found catalog material (the node, the
// article, the link type, the whole cross chain, the catalog vehicle ids —
// empty is the honest not-found record) and the source traceability (the
// system word, the record identifier, the version, the observed time —
// the 2.7 discipline). The digest pins are NEVER an input: the app
// derives BOTH pins from the LIVE derivations of the vehicle profile
// (4-2) and the part request (4-3) BEFORE the write (AC-1/AC-11 — the
// record carries the exact input versions of the moment). The evidence
// number is the server truth under the locks; the supersede of one
// source's record is a NEW row with a NEW number — never an update.
type RecordFitmentEvidenceCommand struct {
	Actor         domain.Principal
	CorrelationID id.UUID
	TenantID      id.UUID
	CaseID        id.UUID
	WorkItemID    id.UUID
	PositionKey   string
	CatalogNode   string
	Article       string
	LinkType      string
	Chain         []string
	// CatalogVehicleIDs is the found catalog vehicle id list: empty is the
	// honest not-found record (the source was researched, the id was not
	// found — never «неприменимо», AC-10).
	CatalogVehicleIDs []string
	SourceSystem      string
	SourceRecordID    string
	SourceVersion     string
	SourceObservedAt  time.Time
	IdempotencyKey    string
	Now               time.Time
}
