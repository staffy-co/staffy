package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/action/api"
	actionCatalog "github.com/artemiosu/staffy/internal/action/catalog"
	"github.com/artemiosu/staffy/internal/action/domain"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
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
// selection snapshot of a position with its live panel material.
type orderPositionMaterial struct {
	selection casesDomain.SelectionSnapshotView
	linkType  string
	fitmentOK bool
	offer     *casesDomain.OfferView
}

// DeriveOrderCommitmentMaterial derives the canonical order commitment
// material of one case view LIVE: the LATEST selection snapshot of every
// selected position (a re-selection supersedes by the snapshot number),
// the case-level input pins of the globally-latest selection (a divergence
// between the selections' pins is the honest mixed-versions refusal), the
// proven fitment of every selected position (a non-proven selection is the
// honest refusal of the builder — the guard 4-5 refused the RECORD, this
// belt re-derives LIVE), the offer record material of the chosen
// (supplier, offer number) pins and the originality word of the fitment
// link type. NO ABCP Order is created here and NOTHING is sent to any
// supplier — this is the material of the proposal ONLY (the OQ-3 fence).
func DeriveOrderCommitmentMaterial(view casesAPI.CaseView) (domain.OrderCommitmentMaterial, error) {
	selections := view.Comparison.Selections
	if len(selections) == 0 {
		return domain.OrderCommitmentMaterial{}, domain.ErrOrderNoSelection
	}
	// The LATEST selection of every position key (the view order is the
	// store order: by the work item, then the monotonic snapshot number —
	// the later entry IS the newer snapshot; a re-selection supersedes).
	latest := map[string]casesDomain.SelectionSnapshotView{}
	for _, snapshot := range selections {
		latest[snapshot.PositionKey] = snapshot
	}
	// The comparison positions: the live fitment answer per position (the
	// status word, the link type word, the digest).
	comparisonPositions := map[string]casesDomain.ComparisonPositionView{}
	for _, position := range view.Comparison.Comparison.Positions {
		comparisonPositions[position.PositionKey] = position
	}
	// The offer records of the case: the append-only truth the selection
	// pins address (the (position, supplier, offer_no) pin pair).
	offers := map[string]casesDomain.OfferView{}
	for _, record := range view.Comparison.Records {
		offers[fmt.Sprintf("%s\x1f%s\x1f%d", record.PositionKey, record.SupplierID, record.OfferNo)] = record
	}
	// The globally-latest selection carries the case-level input pins: any
	// other latest selection pinned to a DIFFERENT profile/request version
	// is the honest mixed-versions refusal (the newer version arrived after
	// the older selection — the updated client selection is required).
	globallyLatest := selections[len(selections)-1]
	positionKeys := make([]string, 0, len(latest))
	for key := range latest {
		positionKeys = append(positionKeys, key)
	}
	sort.Strings(positionKeys)
	candidates := make([]orderPositionMaterial, 0, len(positionKeys))
	for _, key := range positionKeys {
		snapshot := latest[key]
		if snapshot.ProfileDigestPin != globallyLatest.ProfileDigestPin ||
			snapshot.PartRequestDigestPin != globallyLatest.PartRequestDigestPin {
			return domain.OrderCommitmentMaterial{}, domain.ErrOrderMixedInputVersions
		}
		if snapshot.Currency != globallyLatest.Currency {
			return domain.OrderCommitmentMaterial{}, domain.ErrOrderMixedCurrency
		}
		position, found := comparisonPositions[key]
		if !found || position.FitmentStatus != casesDomain.FitmentStatusProven {
			return domain.OrderCommitmentMaterial{}, domain.ErrOrderFitmentNotProven
		}
		offerKey := fmt.Sprintf("%s\x1f%s\x1f%d", snapshot.PositionKey, snapshot.SupplierID, snapshot.OfferNo)
		offer, found := offers[offerKey]
		if !found {
			return domain.OrderCommitmentMaterial{}, domain.ErrOrderOfferUnresolved
		}
		candidates = append(candidates, orderPositionMaterial{
			selection: snapshot,
			linkType:  position.LinkType,
			fitmentOK: true,
			offer:     &offer,
		})
	}
	// The composition: one position per selected position key (sorted —
	// the byte-stable order of the digest and the render alike).
	positions := make([]domain.OrderPosition, 0, len(candidates))
	knownSum := 0.0
	for _, candidate := range candidates {
		originality := orderOriginalityOf(candidate.linkType)
		if !domain.WordIn(domain.OrderOriginalityWords, originality) {
			return domain.OrderCommitmentMaterial{}, domain.ErrOrderFitmentNotProven
		}
		conditions := ""
		if candidate.offer != nil {
			conditions = candidate.offer.Conditions
		}
		positions = append(positions, domain.OrderPosition{
			PositionKey:      candidate.selection.PositionKey,
			SupplierID:       candidate.selection.SupplierID,
			OfferNo:          candidate.selection.OfferNo,
			Brand:            offerBrandOf(candidate.offer),
			Article:          offerArticleOf(candidate.offer),
			Originality:      originality,
			Quantity:         candidate.selection.Quantity,
			UnitClientPrice:  candidate.selection.UnitClientPrice,
			Currency:         candidate.selection.Currency,
			Conditions:       conditions,
			FitmentDigestPin: candidate.selection.FitmentDigestPin,
			OfferDigestPin:   candidate.selection.OfferDigestPin,
			SelectionDigest:  candidate.selection.SelectionDigest,
		})
		knownSum += candidate.selection.KnownClientSum
	}
	material := domain.OrderCommitmentMaterial{
		ProfileDigestPin:     globallyLatest.ProfileDigestPin,
		PartRequestDigestPin: globallyLatest.PartRequestDigestPin,
		Positions:            positions,
		KnownClientSum:       knownSum,
		Currency:             globallyLatest.Currency,
	}
	if err := domain.GuardOrderCommitmentMaterial(material); err != nil {
		return domain.OrderCommitmentMaterial{}, err
	}
	return material, nil
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

// offerBrandOf / offerArticleOf read the offer record material (an absent
// record is the honest unresolved refusal — never an invented word).
func offerBrandOf(offer *casesDomain.OfferView) string {
	if offer == nil {
		return ""
	}
	return offer.Brand
}

func offerArticleOf(offer *casesDomain.OfferView) string {
	if offer == nil {
		return ""
	}
	return offer.Article
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
	// The submit through the EXISTING machine: the target of the order is
	// the work item's part-search contour (the operator never chooses the
	// target — the builder pins it), the owner epoch is the LIVE derivation
	// of the view, the typed input is the derived material ONLY.
	typed := map[string]string{
		"order_digest":     digest,
		"order_material":   render,
		"known_client_sum": strconv.FormatFloat(orderMaterial.KnownClientSum, 'f', 2, 64),
		"currency":         orderMaterial.Currency,
	}
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
