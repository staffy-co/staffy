package api

import (
	"context"
	"time"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 4.5: the supplier offer and the client selection record commands
// ---------------------------------------------------------------------------

// RecordSupplierOfferCommand records ONE manager's commercial conditions
// research result as the append-only supplier offer (AC-2): the
// operator-manager with the review OR the configuration scope (the gate
// fires BEFORE the material guards — the M1 precedent), the work item of
// the case, the canonical position key (the normalized article rendering —
// the 4.3 number discipline), the supplier material (the identifier, the
// name, the brand, the article), the commercial conditions (the client
// price by the ABCP permission — the RECORDED permitted price, never a
// computed one; the OPTIONAL purchase price of the internal data; the
// currency; the availability; the term estimate; the conditions text) and
// the source traceability (the system word, the record identifier, the
// version, the observed time — the 2.7 discipline). The digest pins are
// NEVER an input: the app derives the profile and the request pins from
// the LIVE derivations of 4.2/4.3 and the fitment pin from the LIVE 4.4
// derivation of the position (empty when the proof is absent or not
// proven — the honest offer without the proof, recorded but never
// recommended). The offer number and the canonical OfferDigest are the
// server truth under the locks; the supersede of one supplier's record is
// a NEW row with a NEW number — never an update.
type RecordSupplierOfferCommand struct {
	Actor            domain.Principal
	CorrelationID    id.UUID
	TenantID         id.UUID
	CaseID           id.UUID
	WorkItemID       id.UUID
	PositionKey      string
	SupplierID       string
	SupplierName     string
	Brand            string
	Article          string
	ClientPrice      float64
	PurchasePrice    *float64
	Currency         string
	QuantityAvailable int
	TermDays         *int
	Conditions       string
	SourceSystem     string
	SourceRecordID   string
	SourceVersion    string
	SourceObservedAt time.Time
	IdempotencyKey   string
	Now              time.Time
}

// RecordClientSelectionCommand records ONE client selection as the
// append-only selection snapshot (AC-13): the operator-manager with the
// review OR the configuration scope, the work item of the case, the
// canonical position key, the chosen CURRENT offer (the supplier and the
// offer number) and the chosen quantity. The pins, the prices and the
// known client sum are NEVER inputs: the app derives them from the LIVE
// derivations (the current offer record read, the 4.4 fitment answer of
// the position, the CURRENT profile and request digests) BEFORE the
// write; the store re-elects the current offer and re-runs the whole
// guard under the locks. NO ABCP Order is created and NOTHING is sent to
// the supplier (the fence OQ-3/OQ-4 — the order contour of 4.6).
type RecordClientSelectionCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	WorkItemID     id.UUID
	PositionKey    string
	SupplierID     string
	OfferNo        int64
	Quantity       int
	IdempotencyKey string
	Now            time.Time
}

// ---------------------------------------------------------------------------
// The Story 4.5 panel views and the service interface
// ---------------------------------------------------------------------------

// ComparisonView is the domain view alias (the ZERO-drift contract
// mirror: the panel JSON and the domain derivation carry the same shape —
// the contract test pins the enum dictionaries of the domain words).
type ComparisonView = casesDomain.ComparisonView

// ComparisonPositionView is the domain position view alias.
type ComparisonPositionView = casesDomain.ComparisonPositionView

// ComparisonVariantView is the domain variant view alias (the CLIENT
// material — the purchase price is structurally absent, AC-2).
type ComparisonVariantView = casesDomain.ComparisonVariantView

// ComparisonVoiceBlock is the domain phone block alias.
type ComparisonVoiceBlock = casesDomain.ComparisonVoiceBlock

// OfferSourceView is the domain traceability view alias.
type OfferSourceView = casesDomain.OfferSourceView

// OfferView is the domain operator record view alias (the internal
// section material WITH the purchase price and its internal-data word).
type OfferView = casesDomain.OfferView

// SelectionSnapshotView is the domain snapshot view alias.
type SelectionSnapshotView = casesDomain.SelectionSnapshotView

// ComparisonPanelView is the Story 4.5 section of the case view: the
// honest degradation note when the pinned pack carries no comparison
// policy (the 4.2/4.3/4.4 panel family precedent — the read NEVER fails),
// the comparison derivation, the operator record material and the
// recorded selection snapshots.
type ComparisonPanelView struct {
	SchemaDefined bool              `json:"schema_defined"`
	Note          string            `json:"note,omitempty"`
	Comparison    ComparisonView    `json:"comparison"`
	Records       []OfferView       `json:"records"`
	Selections    []SelectionSnapshotView `json:"selections"`
}

// ComparisonSchemaNoteAbsent is the honest degradation word of the panel
// when the pinned pack carries no comparison policy (the 4.2/4.3/4.4
// panel family precedent of the unreadable-pin degradation).
const ComparisonSchemaNoteAbsent = "Схема сравнения пака не определена: панель недоступна — политика сравнения (валюты, свежесть предложений, критерии ранжирования) приходит из закреплённого пака, без хардкода."

// OffersService is the write surface of the Story 4.5 records (the
// manager's commercial conditions research results and the client
// selections — the append-only rows; the read surface lives in the case
// view of the whole service).
type OffersService interface {
	RecordSupplierOffer(context.Context, RecordSupplierOfferCommand) (WorkOutcome, error)
	RecordClientSelection(context.Context, RecordClientSelectionCommand) (WorkOutcome, error)
}
