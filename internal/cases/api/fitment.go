package api

import (
	"context"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
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

// ---------------------------------------------------------------------------
// The Story 4.4 panel view and the service interface
// ---------------------------------------------------------------------------

// FitmentView is the domain view alias (the ZERO-drift contract mirror:
// the panel JSON and the domain derivation carry the same shape — the
// contract test pins the enum dictionaries of the domain words).
type FitmentView = casesDomain.FitmentView

// FitmentPositionView is the domain position view alias.
type FitmentPositionView = casesDomain.FitmentPositionView

// FitmentEvidenceView is the domain evidence view alias.
type FitmentEvidenceView = casesDomain.FitmentEvidenceView

// FitmentPanelView is the Story 4.4 section of the case view: the honest
// degradation note when the pinned pack carries no fitment policy (the
// 4.2/4.3 panel family precedent — the read NEVER fails).
type FitmentPanelView struct {
	SchemaDefined bool        `json:"schema_defined"`
	Note          string      `json:"note,omitempty"`
	Fitment       FitmentView `json:"fitment"`
}

// FitmentSchemaNoteAbsent is the honest degradation word of the panel when
// the pinned pack carries no fitment policy (the 4.2/4.3 panel family
// precedent of the unreadable-pin degradation).
const FitmentSchemaNoteAbsent = "Схема применимости пака не определена: панель недоступна — политика доказательства (типы связей, глубина цепочки, источники, свежесть) приходит из закреплённого пака, без хардкода."

// FitmentService is the write surface of the Story 4.4 evidence records
// (the manager's catalog research results — the append-only proof rows;
// the read surface lives in the case view of the whole service).
type FitmentService interface {
	RecordFitmentEvidence(context.Context, RecordFitmentEvidenceCommand) (WorkOutcome, error)
}
