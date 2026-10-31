package app

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/staffy-co/staffy/internal/action/api"
	actionCatalog "github.com/staffy-co/staffy/internal/action/catalog"
	"github.com/staffy-co/staffy/internal/action/domain"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// Story 4.6 — the Order Commitment builder. The operator NEVER composes
// the order material: this builder derives it LIVE from the case view (the
// recorded selections of 4-5 plus the current derivations of the panels)
// and submits the proposal through the EXISTING machine 3-4 with the typed
// input the definition pins (order_digest / order_material /
// known_client_sum / currency). The pins, the prices and the composition
// are never typed input of any human — the «the operator does not type
// digests» principle of 4-4/4-5.

// orderPositionMaterial is ONE derived position candidate: the LATEST
// selection snapshot of a position with its CURRENT offer record (the
// maximal offer number of the chosen supplier at the position — a
// re-recorded price, availability or term changes the CURRENT record and
// therefore the live composition, AC-5) and the live fitment answer.
type orderPositionMaterial struct {
	selection casesDomain.SelectionSnapshotView
	linkType  string
	offer     *casesDomain.OfferView
}

// DeriveOrderCommitmentMaterial derives the canonical order commitment
// material of one case view LIVE: the case-level input pins read from the
// LIVE panel derivations (the ProfileDigest of 4-2 and the
// PartRequestDigest of 4-3 — a new confirmed attribute changes the pin),
// the LATEST selection snapshot of every selected position (a
// re-selection supersedes by the snapshot number), every latest
// selection pinned to the CURRENT input versions (a selection over an
// older profile/request version is the honest mixed-versions refusal —
// the updated client selection is required), the proven fitment of every
// selected position with its LIVE FitmentDigest (a new evidence record
// changes the digest), the CURRENT offer record of the chosen supplier
// (the maximal offer number — the re-recorded price, availability, term
// or conditions change the OfferDigest and the sum arithmetic) and the
// originality word of the fitment link type. NO ABCP Order is created
// here and NOTHING is sent to any supplier — this is the material of the
// proposal ONLY (the OQ-3 fence).
func DeriveOrderCommitmentMaterial(view casesAPI.CaseView) (domain.OrderCommitmentMaterial, error) {
	selections := view.Comparison.Selections
	if len(selections) == 0 {
		return domain.OrderCommitmentMaterial{}, domain.ErrOrderNoSelection
	}
	// The LIVE case-level input pins: the panel derivations of 4-2/4-3 (a
	// new confirmed attribute changes the pin — the live truth, never the
	// selection's stored copy).
	profileDigest := view.Vehicle.Profile.ProfileDigest
	partRequestDigest := view.PartRequest.Part.PartRequestDigest
	if profileDigest == "" || partRequestDigest == "" {
		return domain.OrderCommitmentMaterial{}, domain.ErrOrderMixedInputVersions
	}
	// The LATEST selection of every position key (the view order is the
	// store order: by the work item, then the monotonic snapshot number —
	// the later entry IS the newer snapshot; a re-selection supersedes).
	latest := map[string]casesDomain.SelectionSnapshotView{}
	for _, snapshot := range selections {
		latest[snapshot.PositionKey] = snapshot
	}
	// The comparison positions: the live fitment answer per position (the
	// status word and the link type word).
	comparisonPositions := map[string]casesDomain.ComparisonPositionView{}
	for _, position := range view.Comparison.Comparison.Positions {
		comparisonPositions[position.PositionKey] = position
	}
	// The live fitment digests per position (the 4-4 panel — the LIVE pin;
	// a new evidence record changes it).
	fitmentDigests := map[string]string{}
	for _, position := range view.Fitment.Fitment.Positions {
		fitmentDigests[position.PositionKey] = position.FitmentDigest
	}
	// The CURRENT offer record of every (position, supplier) pair: the
	// maximal offer number (a re-record supersedes — the current conditions
	// of the chosen supplier; the pinned offer number of the selection
	// addresses the supplier, the CURRENT record addresses the conditions).
	currentOffers := map[string]casesDomain.OfferView{}
	for _, record := range view.Comparison.Records {
		key := record.PositionKey + "\x1f" + record.SupplierID
		if existing, ok := currentOffers[key]; !ok || record.OfferNo > existing.OfferNo {
			currentOffers[key] = record
		}
	}
	// The globally-latest selection carries the currency word (the pack
	// dictionary); the latest selections must agree.
	globallyLatest := selections[len(selections)-1]
	positionKeys := make([]string, 0, len(latest))
	for key := range latest {
		positionKeys = append(positionKeys, key)
	}
	sort.Strings(positionKeys)
	candidates := make([]orderPositionMaterial, 0, len(positionKeys))
	for _, key := range positionKeys {
		snapshot := latest[key]
		// The selection must be pinned to the CURRENT input versions: an
		// older profile/request version under the selection is the honest
		// mixed-versions refusal (the updated client selection is required).
		if snapshot.ProfileDigestPin != profileDigest || snapshot.PartRequestDigestPin != partRequestDigest {
			return domain.OrderCommitmentMaterial{}, domain.ErrOrderMixedInputVersions
		}
		if snapshot.Currency != globallyLatest.Currency {
			return domain.OrderCommitmentMaterial{}, domain.ErrOrderMixedCurrency
		}
		position, found := comparisonPositions[key]
		if !found || position.FitmentStatus != casesDomain.FitmentStatusProven {
			return domain.OrderCommitmentMaterial{}, domain.ErrOrderFitmentNotProven
		}
		liveFitmentDigest := fitmentDigests[key]
		if liveFitmentDigest == "" {
			return domain.OrderCommitmentMaterial{}, domain.ErrOrderFitmentNotProven
		}
		current := currentOffers[key+"\x1f"+snapshot.SupplierID]
		if current.OfferNo < 1 {
			return domain.OrderCommitmentMaterial{}, domain.ErrOrderOfferUnresolved
		}
		candidates = append(candidates, orderPositionMaterial{
			selection: snapshot,
			linkType:  position.LinkType,
			offer:     &current,
		})
	}
	// The composition: one position per selected position key (sorted —
	// the byte-stable order of the digest and the render alike), the sum
	// as the LIVE arithmetic of the quantities and the CURRENT prices.
	positions := make([]domain.OrderPosition, 0, len(candidates))
	knownSum := 0.0
	for _, candidate := range candidates {
		originality := orderOriginalityOf(candidate.linkType)
		if !domain.WordIn(domain.OrderOriginalityWords, originality) {
			return domain.OrderCommitmentMaterial{}, domain.ErrOrderFitmentNotProven
		}
		conditions := ""
		brand, article, currency := "", "", candidate.selection.Currency
		unitPrice := candidate.selection.UnitClientPrice
		offerDigest := candidate.selection.OfferDigestPin
		if candidate.offer != nil {
			conditions = candidate.offer.Conditions
			brand = candidate.offer.Brand
			article = candidate.offer.Article
			currency = candidate.offer.Currency
			unitPrice = candidate.offer.ClientPrice
			offerDigest = candidate.offer.OfferDigest
		}
		positions = append(positions, domain.OrderPosition{
			PositionKey:      candidate.selection.PositionKey,
			SupplierID:       candidate.selection.SupplierID,
			OfferNo:          candidate.offer.OfferNo,
			Brand:            brand,
			Article:          article,
			Originality:      originality,
			Quantity:         candidate.selection.Quantity,
			UnitClientPrice:  unitPrice,
			Currency:         currency,
			Conditions:       conditions,
			FitmentDigestPin: fitmentDigestsOf(candidate, view),
			OfferDigestPin:   offerDigest,
			SelectionDigest:  candidate.selection.SelectionDigest,
		})
		knownSum += float64(candidate.selection.Quantity) * unitPrice
	}
	material := domain.OrderCommitmentMaterial{
		ProfileDigestPin:     profileDigest,
		PartRequestDigestPin: partRequestDigest,
		Positions:            positions,
		KnownClientSum:       knownSum,
		Currency:             globallyLatest.Currency,
	}
	if err := domain.GuardOrderCommitmentMaterial(material); err != nil {
		return domain.OrderCommitmentMaterial{}, err
	}
	return material, nil
}

// fitmentDigestsOf resolves the LIVE fitment digest of the position of
// the candidate (the 4-4 panel answer — the live pin of the proof).
func fitmentDigestsOf(candidate orderPositionMaterial, view casesAPI.CaseView) string {
	for _, position := range view.Fitment.Fitment.Positions {
		if position.PositionKey == candidate.selection.PositionKey {
			return position.FitmentDigest
		}
	}
	return ""
}

// orderOriginalityOf maps the fitment link type word onto the order
// originality dictionary word (original — the OEM part; cross — the
// cross-number analog).
func orderOriginalityOf(linkType string) string {
	switch linkType {
	case casesDomain.FitmentLinkOriginal:
		return domain.OrderOriginalityOriginal
	case casesDomain.FitmentLinkCross:
		return domain.OrderOriginalityAnalog
	}
	return linkType
}

// PrepareOrderProposalCommand is the operator's ONE-word ask: build the
// order proposal of the case. The material is NEVER an input — no pins, no
// prices, no composition rides the command (the builder derives everything
// LIVE from the case view; the probe of the app tests pins it).
type PrepareOrderProposalCommand = api.PrepareOrderProposalCommand

// PrepareOrderProposal builds the order commitment proposal of the case
// through the EXISTING machine 3-4: the rights gate, the pinned definition
// resolve, the live case view, the material derivation, the guard, the
// canonical digest and render, and the submit with the typed input the
// definition pins. The honest refusals (no selection, a non-proven
// position, mixed input versions, an oversize render) speak BEFORE any
// proposal row is written.
func (s *Service) PrepareOrderProposal(ctx context.Context, command PrepareOrderProposalCommand) (api.ProposalOutcome, error) {
	if err := s.requirePreviewPorts(); err != nil {
		return api.ProposalOutcome{}, err
	}
	if _, err := s.authorizeProposal(ctx, command.Actor, command.TenantID); err != nil {
		return api.ProposalOutcome{}, err
	}
	// The pinned definition of the order contour resolves through the
	// injected dictionary (the production root serves the product catalog;
	// the test circuits register their fixtures ON TOP of it).
	if _, ok := actionCatalog.GetFrom(s.defs, domain.OrderActionID, domain.OrderActionVersion); !ok {
		return api.ProposalOutcome{}, domain.ErrNotFound
	}
	// The live case material: the case must exist and the work item must
	// anchor it (the AD-2 discipline of SubmitProposal).
	caseView, err := s.caseMaterial.CaseView(ctx, command.Actor, command.TenantID, command.CaseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return api.ProposalOutcome{}, domain.ErrNotFound
		}
		return api.ProposalOutcome{}, err
	}
	material := caseViewMaterial{view: caseView}
	if !material.workItemAnchored(command.WorkItemID) {
		return api.ProposalOutcome{}, domain.ErrNotFound
	}
	// The LIVE derivation of the order material (the honest refusals speak
	// BEFORE any proposal — no attempt row, no version, no typed material).
	orderMaterial, err := DeriveOrderCommitmentMaterial(caseView)
	if err != nil {
		return api.ProposalOutcome{}, err
	}
	digest := domain.OrderDigestOf(orderMaterial)
	render := domain.OrderMaterialCanonicalText(orderMaterial)
	if utf8.RuneCountInString(render) > domain.MaxTypedInputValueRunes {
		return api.ProposalOutcome{}, domain.ErrOrderMaterialOversize
	}
	// The typed input of the derived material (the schema fields only).
	typed := map[string]string{
		"order_digest":     digest,
		"order_material":   render,
		"known_client_sum": strconv.FormatFloat(orderMaterial.KnownClientSum, 'f', 2, 64),
		"currency":         orderMaterial.Currency,
	}
	// The thread discipline of the re-derivation (the AD-7 ladder of the
	// machine 3-4): the FIRST build of the case's order thread rides the
	// proposal submission; EVERY later re-derivation (the correction button
	// — the client re-selected, the inputs moved) rides the EDIT ladder of
	// the SAME thread: the new version supersedes the old atomically, the
	// operator never types the material — the builder re-derives it LIVE
	// both times (AC-16: the confirmation and the correction are DIFFERENT
	// actions of the SAME machine).
	scope, err := s.authorizeProposal(ctx, command.Actor, command.TenantID)
	if err != nil {
		return api.ProposalOutcome{}, err
	}
	versions, err := s.previews.ReadPreviewVersions(ctx, scope, command.CaseID)
	if err != nil {
		return api.ProposalOutcome{}, err
	}
	var threadPreviewID id.UUID
	var currentVersion int64
	for _, version := range versions {
		if version.DefinitionID == domain.OrderActionID && version.VersionNo > currentVersion {
			threadPreviewID, currentVersion = version.PreviewID, version.VersionNo
		}
	}
	if threadPreviewID.Valid() {
		edited, err := s.NewPreviewVersionFromEdit(ctx, api.NewPreviewVersionCommand{
			Actor:      command.Actor,
			TenantID:   command.TenantID,
			CaseID:     command.CaseID,
			PreviewID:  threadPreviewID,
			TypedInput: typed,
			Now:        command.Now,
		})
		if err != nil {
			return api.ProposalOutcome{}, err
		}
		return api.ProposalOutcome{
			Command:       "submit_proposal",
			Verdict:       domain.VerdictValid,
			AttemptNo:     1,
			PreviewID:     edited.PreviewID,
			VersionNo:     edited.VersionNo,
			PreviewDigest: edited.PreviewDigest,
		}, nil
	}
	// The FIRST build: the submit through the EXISTING machine — the
	// target of the order is the work item's part-search contour (the
	// operator never chooses the target — the builder pins it), the owner
	// epoch is the LIVE derivation of the view, the typed input is the
	// derived material ONLY.
	return s.SubmitProposal(ctx, api.SubmitProposalCommand{
		Actor:             command.Actor,
		TenantID:          command.TenantID,
		CaseID:            command.CaseID,
		WorkItemID:        command.WorkItemID,
		DefinitionID:      domain.OrderActionID,
		DefinitionVersion: domain.OrderActionVersion,
		Target:            domain.ProposalTarget{Kind: domain.TargetOrder, Ref: command.WorkItemID},
		TypedInput:        typed,
		OwnerEpoch:        material.ownerEpoch(),
		Now:               command.Now,
	})
}
