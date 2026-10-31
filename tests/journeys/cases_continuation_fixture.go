package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 2.10 fixture: the continuation commands and the protected
// surface over the REAL domain functions (GuardContinuationPurposes,
// GuardContinuationTTL, GuardContinuationConsumptionLimit,
// GuardContinuationTexts, DeriveContinuationLinkState, the token codec
// with its SHA-256 hash, ConfirmedFactView) — the fixture world stores
// the append-only link/use/revocation rows; the state of every link, the
// consumption accounting and the panel words derive at every read (the
// REAL derivations, never stored states). The capability token exists
// ONLY in the fresh issuance answer (the random half never enters the
// replay material — the honest mask of the store mirror); the lookup
// verifies the token-hash BEFORE any material is read and the tenant
// candidate against the row (the AD-10 honesty of the real RLS scope).

// fixtureContinuationID generates the next deterministic continuation
// id (the links, the uses and the revocations share the counter space —
// the ids never collide with the other fixture surfaces).
func (f *casesFixture) fixtureContinuationID() id.UUID {
	f.continuationSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(94000+f.continuationSeq)))
}

// IssueContinuationLink issues one purpose-bound link over the REAL
// domain guards: the purpose set, the lifetime bounds, the operator
// texts; the Work Item must be alive (the terminal refusal of 2.2). The
// random half is generated ONCE per execution and never enters the
// replay material: the exact repeat answers the stored outcome with the
// honest mask — the token is not recoverable (a lost URL means the
// revocation and a fresh link, exactly as the store answers).
func (f *casesFixture) IssueContinuationLink(_ context.Context, command casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error) {
	f.lock()
	defer f.unlock()
	if command.TenantID != f.tenant {
		return casesAPI.ContinuationIssuance{}, casesDomain.ErrNotFound
	}
	if err := fixtureWorkRule("issue_continuation_link", casesDomain.GuardContinuationPurposes(command.Purposes)); err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	if err := fixtureWorkRule("issue_continuation_link", casesDomain.GuardContinuationTTL(command.TTLMinutes)); err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	if command.ConsumptionLimit == 0 {
		command.ConsumptionLimit = casesDomain.DefaultContinuationConsumptionLimit
	}
	if err := fixtureWorkRule("issue_continuation_link", casesDomain.GuardContinuationConsumptionLimit(command.ConsumptionLimit)); err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	if err := fixtureWorkRule("issue_continuation_link", casesDomain.GuardContinuationTexts(command.Question, command.Explanation)); err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	if len(command.ParticipantRef) < 1 || len(command.ParticipantRef) > 128 {
		return casesAPI.ContinuationIssuance{}, fixtureWorkRule("issue_continuation_link", fmt.Errorf("адрес участника ссылки обязан быть 1..128 символами (канал клиента, которому вплетается URL)"))
	}
	// The request hash NEVER carries the random half (the replay of the
	// exact repeat answers the stored outcome; another material under
	// the same key is the honest key conflict).
	outcome, fresh, err := f.workCommandFlow("issue_continuation_link", command.IdempotencyKey, struct {
		Case        string   `json:"case"`
		WorkItem    string   `json:"work_item"`
		Purposes    []string `json:"purposes"`
		Question    string   `json:"question"`
		Explain     string   `json:"explanation"`
		Participant string   `json:"participant_ref"`
		TTL         int      `json:"ttl_minutes"`
		Limit       int      `json:"consumption_limit"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.Purposes, command.Question,
		command.Explanation, command.ParticipantRef, command.TTLMinutes, command.ConsumptionLimit})
	if err != nil || !fresh {
		return casesAPI.ContinuationIssuance{Outcome: outcome}, err
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known || current.CaseID != command.CaseID {
		return casesAPI.ContinuationIssuance{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.ContinuationIssuance{}, fixtureWorkRule("issue_continuation_link", fmt.Errorf("%s", casesDomain.TerminalWorkItemContinuationText))
	}
	linkID := command.LinkID
	if !linkID.Valid() {
		linkID = f.fixtureContinuationID()
	}
	random, err := casesDomain.RandomContinuationSecret()
	if err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	tokenHash, err := casesDomain.ContinuationTokenHash(random)
	if err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	token, err := casesDomain.EncodeContinuationToken(command.TenantID, random)
	if err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	ownerEpoch := int64(0)
	if state := f.fixtureState(command.CaseID, command.Now); state.Current != nil {
		ownerEpoch = state.Current.Epoch
	}
	expires := command.Now.UTC().Truncate(time.Microsecond).Add(time.Duration(command.TTLMinutes) * time.Minute)
	f.contLinks[linkID] = casesDomain.ContinuationLink{
		TenantID: f.tenant, CaseID: command.CaseID, WorkItemID: command.WorkItemID, LinkID: linkID,
		ParticipantRef: command.ParticipantRef, OwnerEpoch: ownerEpoch,
		Purposes: append([]string(nil), command.Purposes...),
		Question: command.Question, Explanation: command.Explanation,
		TTLMinutes: command.TTLMinutes, IssuedAt: command.Now.UTC(), ExpiresAt: expires,
		ConsumptionLimit: command.ConsumptionLimit, TokenHash: tokenHash,
		ActorID: command.Actor.ID, CreatedAt: command.Now.UTC(),
	}
	f.contByHash[string(tokenHash)] = linkID
	// The stored outcome: the link id and the honest mask word — the
	// token NEVER enters the outcome (the replay answer carries «выдан»).
	outcome = casesAPI.WorkOutcome{
		Command: "issue_continuation_link", LinkID: linkID,
		ContinuationIssued: true, ContinuationExpires: expires,
	}
	f.storeOutcome("issue_continuation_link", command.IdempotencyKey, outcome)
	return casesAPI.ContinuationIssuance{
		Outcome: outcome, URL: casesAPI.ContinuationURLPath(token), Token: token, Expires: expires,
	}, nil
}

// RevokeContinuationLink revokes one link: the append-only revocation
// row with the honest reason; the read-time derivation blocks the
// surface without disclosure; the second revoke of the same link
// refuses honestly (the one-revocation UNIQUE of the store mirror).
func (f *casesFixture) RevokeContinuationLink(_ context.Context, command casesAPI.RevokeContinuationLinkCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if command.TenantID != f.tenant {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if err := fixtureWorkRule("revoke_continuation_link", casesDomain.GuardReason(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("revoke_continuation_link", command.IdempotencyKey, struct {
		Case   string `json:"case"`
		Item   string `json:"work_item"`
		Link   string `json:"link"`
		Reason string `json:"reason"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.LinkID.String(), command.Reason})
	if err != nil || !fresh {
		return outcome, err
	}
	link, known := f.contLinks[command.LinkID]
	if !known || link.CaseID != command.CaseID || link.WorkItemID != command.WorkItemID {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if _, already := f.contRevokes[command.LinkID]; already {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("revoke_continuation_link", fmt.Errorf("%s", casesDomain.ContinuationAlreadyRevokedText))
	}
	f.contRevokes[command.LinkID] = casesDomain.ContinuationRevocation{
		TenantID: f.tenant, CaseID: command.CaseID, WorkItemID: command.WorkItemID, LinkID: command.LinkID,
		RevocationID: f.fixtureContinuationID(), Reason: command.Reason,
		ActorID: command.Actor.ID, CreatedAt: command.Now.UTC(),
	}
	outcome = casesAPI.WorkOutcome{Command: "revoke_continuation_link", LinkID: command.LinkID}
	f.storeOutcome("revoke_continuation_link", command.IdempotencyKey, outcome)
	return outcome, nil
}

// fixtureContinuationLookup resolves the opaque token the REAL codec
// way: the fail-closed parse, the SHA-256 hash lookup and the
// tenant-candidate verification BEFORE any material is read (the AD-10
// honesty of the store; a foreign tenant token answers the same unknown
// refusal). The derivation runs read-time over the CURRENT owner epoch,
// the revocation existence and the accepted corrections.
func (f *casesFixture) fixtureContinuationLookup(token string, now time.Time) (casesDomain.ContinuationLinkLookup, bool) {
	tenantID, random, err := casesDomain.ParseContinuationToken(token)
	if err != nil {
		return casesDomain.ContinuationLinkLookup{}, false
	}
	tokenHash, err := casesDomain.ContinuationTokenHash(random)
	if err != nil {
		return casesDomain.ContinuationLinkLookup{}, false
	}
	linkID, found := f.contByHash[string(tokenHash)]
	if !found {
		return casesDomain.ContinuationLinkLookup{}, false
	}
	link := f.contLinks[linkID]
	// The scope candidate verifies against the row BEFORE the material.
	if link.TenantID != tenantID {
		return casesDomain.ContinuationLinkLookup{}, false
	}
	uses := []casesDomain.ContinuationLinkUse{}
	for _, use := range f.contUses {
		if use.LinkID == linkID {
			uses = append(uses, use)
		}
	}
	_, revoked := f.contRevokes[linkID]
	ownerEpoch := int64(0)
	if state := f.fixtureState(link.CaseID, now); state.Current != nil {
		ownerEpoch = state.Current.Epoch
	}
	accepted := 0
	for _, use := range uses {
		if casesDomain.ContinuationUseConsumesLimit(use.UseKind) {
			accepted++
		}
	}
	return casesDomain.ContinuationLinkLookup{
		Link: link, Uses: uses, Revoked: revoked, OwnerEpochCurrent: ownerEpoch,
		State: casesDomain.DeriveContinuationLinkState(casesDomain.ContinuationLinkStateInput{
			Now: now, ExpiresAt: link.ExpiresAt, Revoked: revoked,
			AcceptedCorrections: accepted, ConsumptionLimit: link.ConsumptionLimit,
			OwnerEpochPinned: link.OwnerEpoch, OwnerEpochCurrent: ownerEpoch,
		}),
	}, true
}

// ContinueSurface reads the material of the protected page: the state of
// the moment (the valid word renders the form; the consumed word renders
// the terminal page; every other word — the expired, the revoked, the
// owner-stale and the unknown/parse refusals — renders the single
// generic refusal without disclosure), the confirmed facts of the Work
// Item of the link (the correctable select — only this Work Item) and
// the operator texts.
func (f *casesFixture) ContinueSurface(_ context.Context, token string, now time.Time) (casesAPI.ContinuationSurface, error) {
	f.lock()
	defer f.unlock()
	lookup, ok := f.fixtureContinuationLookup(token, now)
	if !ok {
		return casesAPI.ContinuationSurface{Kind: casesAPI.ContinuationSurfaceInvalid}, nil
	}
	switch lookup.State {
	case casesDomain.ContinuationStateValid:
	case casesDomain.ContinuationStateConsumed:
		return casesAPI.ContinuationSurface{Kind: casesAPI.ContinuationSurfaceConsumed}, nil
	default:
		return casesAPI.ContinuationSurface{Kind: casesAPI.ContinuationSurfaceInvalid}, nil
	}
	views := []casesAPI.ContinuationFactView{}
	accepted := 0
	for _, use := range lookup.Uses {
		if casesDomain.ContinuationUseConsumesLimit(use.UseKind) {
			accepted++
		}
	}
	for _, factID := range f.factAnchorsOfCase(lookup.Link.CaseID) {
		versions := f.factVersions[factID]
		if len(versions) == 0 || versions[len(versions)-1].WorkItemID != lookup.Link.WorkItemID {
			continue
		}
		status, versionNo, _ := casesDomain.ConfirmedFactView(versions, f.factConfirms[factID])
		if status != casesDomain.FactStatusConfirmed {
			continue
		}
		for _, version := range versions {
			if version.VersionNo == versionNo {
				views = append(views, casesAPI.ContinuationFactView{
					FactID: factID, MaterialKey: version.MaterialKey, ValueText: string(version.Value),
				})
				break
			}
		}
	}
	return casesAPI.ContinuationSurface{
		Kind:        casesAPI.ContinuationSurfaceValid,
		Explanation: lookup.Link.Explanation,
		Question:    lookup.Link.Question,
		Facts:       views,
		Vehicle:     f.fixtureVehicleSurface(lookup.Link),
		ExpiresAt:   lookup.Link.ExpiresAt,
		LimitLeft:   lookup.Link.ConsumptionLimit - accepted,
	}, nil
}

// SubmitCorrection writes one public fact correction through the link:
// the parse and the hash lookup refuse with the generic word; the value
// guard runs BEFORE the surface gates (the honest re-render with the
// preserved input); the state must be valid, the fact must be a
// CONFIRMED fact of the Work Item of the link and the Work Item must be
// alive; the NEW version lands in the unverified state with the
// protected-continuation provenance (the source ref of the link, the
// capability as the actor) and the use row consumes the limit.
func (f *casesFixture) SubmitCorrection(_ context.Context, token string, factID id.UUID, rawValue []byte, now time.Time) (int64, error) {
	f.lock()
	defer f.unlock()
	lookup, ok := f.fixtureContinuationLookup(token, now)
	if !ok {
		return 0, casesDomain.ErrNotFound
	}
	// The value guard BEFORE the state gates (the re-render keeps the
	// typed material of the client — the data never disappears).
	value, valueDigest, err := casesDomain.FactValue(rawValue)
	if err != nil {
		return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: err.Error()}
	}
	if lookup.State != casesDomain.ContinuationStateValid {
		return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: casesDomain.ContinuationNotUsableText}
	}
	versions := f.factVersions[factID]
	if len(versions) == 0 || versions[0].CaseID != lookup.Link.CaseID {
		return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: casesDomain.ContinuationFactNotCorrectableText}
	}
	if versions[len(versions)-1].WorkItemID != lookup.Link.WorkItemID {
		return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: casesDomain.ContinuationFactNotCorrectableText}
	}
	status, _, _ := casesDomain.ConfirmedFactView(versions, f.factConfirms[factID])
	if status != casesDomain.FactStatusConfirmed {
		return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: casesDomain.ContinuationFactNotCorrectableText}
	}
	current, known := f.workCurrentOf(lookup.Link.WorkItemID)
	if !known {
		return 0, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: casesDomain.TerminalWorkItemContinuationText}
	}
	versionNo := int64(len(versions)) + 1
	last := versions[len(versions)-1]
	f.factVersions[factID] = append(versions, casesDomain.FactVersion{
		TenantID: f.tenant, CaseID: lookup.Link.CaseID, FactID: factID, WorkItemID: lookup.Link.WorkItemID,
		VersionNo: versionNo, MaterialKey: last.MaterialKey, Value: value, ValueDigest: valueDigest,
		SourceType: casesDomain.ContinuationCorrectionSourceType, SourceID: lookup.Link.LinkID.String(),
		Channel: casesDomain.FactChannelProtectedContinuation, ActorID: lookup.Link.LinkID,
		ObservedAt: now.UTC(), ReceivedAt: now.UTC(), CreatedAt: now.UTC(),
	})
	f.contUses = append(f.contUses, casesDomain.ContinuationLinkUse{
		TenantID: f.tenant, CaseID: lookup.Link.CaseID, WorkItemID: lookup.Link.WorkItemID, LinkID: lookup.Link.LinkID,
		UseID: f.fixtureContinuationID(), UseKind: casesDomain.ContinuationUseFactCorrectionSubmitted,
		FactID: factID, FactVersionNo: versionNo, SubmittedAt: now.UTC(),
	})
	return versionNo, nil
}

// SubmitVehicleConfirmation writes the Story 4.2 client confirmation of
// the vehicle identification attributes through the link: the state
// must be valid (the raced double-click derives consumed — the terminal
// page honesty), every confirmed (fact, version) pair must be a fact of
// the case of the link scoped to the Work Item of the link (the one
// surface discipline); the append-only confirmations bind the EXACT
// versions with confirmer=client (the note carries the link reference)
// and the use row consumes the limit with the material pair of the
// primary fact. The confirmation never rewrites a fact version.
func (f *casesFixture) SubmitVehicleConfirmation(_ context.Context, token string, items []casesAPI.VehicleConfirmationItem, now time.Time) error {
	f.lock()
	defer f.unlock()
	lookup, ok := f.fixtureContinuationLookup(token, now)
	if !ok {
		return casesDomain.ErrNotFound
	}
	if lookup.State != casesDomain.ContinuationStateValid {
		return casesDomain.WorkRuleError{Rule: "vehicle_confirm", Reason: casesDomain.ContinuationNotUsableText}
	}
	if len(items) == 0 {
		return casesDomain.WorkRuleError{Rule: "vehicle_confirm", Reason: "Подтверждение пусто: отметьте хотя бы один признак автомобиля."}
	}
	note := "Клиент подтвердил по защищённой ссылке продолжения " + lookup.Link.LinkID.String()
	primary := items[0]
	for _, item := range items {
		versions := f.factVersions[item.FactID]
		if len(versions) == 0 || versions[0].CaseID != lookup.Link.CaseID {
			return casesDomain.WorkRuleError{Rule: "vehicle_confirm", Reason: casesDomain.ContinuationFactNotCorrectableText}
		}
		if versions[len(versions)-1].WorkItemID != lookup.Link.WorkItemID {
			return casesDomain.WorkRuleError{Rule: "vehicle_confirm", Reason: casesDomain.ContinuationFactNotCorrectableText}
		}
		if err := casesDomain.GuardVehicleClientConfirmation(len(versions), item.VersionNo, note); err != nil {
			return casesDomain.WorkRuleError{Rule: "vehicle_confirm", Reason: err.Error()}
		}
	}
	for _, item := range items {
		f.factConfirms[item.FactID] = append(f.factConfirms[item.FactID], casesDomain.FactConfirmation{
			TenantID: f.tenant, CaseID: lookup.Link.CaseID, FactID: item.FactID,
			VersionNo: item.VersionNo, ConfirmerType: "client", Note: note,
			ActorID: lookup.Link.LinkID, CreatedAt: now.UTC(),
		})
	}
	f.contUses = append(f.contUses, casesDomain.ContinuationLinkUse{
		TenantID: f.tenant, CaseID: lookup.Link.CaseID, WorkItemID: lookup.Link.WorkItemID, LinkID: lookup.Link.LinkID,
		UseID: f.fixtureContinuationID(), UseKind: casesDomain.ContinuationUseVehicleConfirmationSubmitted,
		FactID: primary.FactID, FactVersionNo: primary.VersionNo, SubmittedAt: now.UTC(),
	})
	return nil
}

// SubmitHuman records the «позвать человека» event: the use row without
// material and without consumption; nothing is ever mutated.
func (f *casesFixture) SubmitHuman(_ context.Context, token string, now time.Time) error {
	return f.fixtureContinuationEvent(token, casesDomain.ContinuationUseHumanRequested, now)
}

// SubmitReturn records the «вернуться в разговор» event: the use row
// without material and without consumption; the Work Item, the facts and
// the owner stay byte-identical.
func (f *casesFixture) SubmitReturn(_ context.Context, token string, now time.Time) error {
	return f.fixtureContinuationEvent(token, casesDomain.ContinuationUseReturnedToConversation, now)
}

// fixtureContinuationEvent writes one public human/return event: the
// dead-link refusals redirect to the derived page (the generic refusal
// or the terminal page — the causes are never distinguished).
func (f *casesFixture) fixtureContinuationEvent(token, useKind string, now time.Time) error {
	f.lock()
	defer f.unlock()
	lookup, ok := f.fixtureContinuationLookup(token, now)
	if !ok {
		return casesDomain.ErrNotFound
	}
	if lookup.State != casesDomain.ContinuationStateValid {
		return casesDomain.WorkRuleError{Rule: "continuation_event", Reason: casesDomain.ContinuationNotUsableText}
	}
	f.contUses = append(f.contUses, casesDomain.ContinuationLinkUse{
		TenantID: f.tenant, CaseID: lookup.Link.CaseID, WorkItemID: lookup.Link.WorkItemID, LinkID: lookup.Link.LinkID,
		UseID: f.fixtureContinuationID(), UseKind: useKind, SubmittedAt: now.UTC(),
	})
	return nil
}

// factAnchorsOfCase lists the fact anchors of one case in the
// deterministic material-key order (the stable page order of the
// surface select).
func (f *casesFixture) factAnchorsOfCase(caseID id.UUID) []id.UUID {
	prefix := caseID.String() + "|"
	keys := make([]string, 0, len(f.factAnchors))
	for key := range f.factAnchors {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	out := make([]id.UUID, 0, len(keys))
	for _, key := range keys {
		out = append(out, f.factAnchors[key])
	}
	return out
}

// fixtureContinuationView assembles the continuation panel of the case
// view (the mirror of the real app continuationView: the link states
// derive read-time from the CURRENT owner epoch, the delivery binding
// and the accepted uses; the token NEVER appears — the honest «выдан»
// mask word stays; the event feed carries the use kinds with the
// material pairs of the corrections).
func (f *casesFixture) fixtureContinuationView(caseID id.UUID) casesAPI.ContinuationView {
	ownerEpoch := int64(0)
	if state := f.fixtureState(caseID, timeNow()); state.Current != nil {
		ownerEpoch = state.Current.Epoch
	}
	now := journeyClock.Now()
	revoked := map[id.UUID]bool{}
	accepted := map[id.UUID]int{}
	links := []casesDomain.ContinuationLink{}
	for _, anchor := range f.items[caseID] {
		linkIDs := []id.UUID{}
		for linkID, link := range f.contLinks {
			if link.CaseID == caseID && link.WorkItemID == anchor.WorkItemID {
				linkIDs = append(linkIDs, linkID)
			}
		}
		sort.Slice(linkIDs, func(i, j int) bool { return linkIDs[i].String() < linkIDs[j].String() })
		for _, linkID := range linkIDs {
			links = append(links, f.contLinks[linkID])
		}
	}
	for _, use := range f.contUses {
		if use.CaseID != caseID {
			continue
		}
		if casesDomain.ContinuationUseConsumesLimit(use.UseKind) {
			accepted[use.LinkID]++
		}
	}
	for linkID := range f.contRevokes {
		revoked[linkID] = true
	}
	view := casesAPI.ContinuationView{
		PanelText: "Ссылка продолжения — одноразовая возможность клиента: токен существует только в ответе команды выдачи и повторно не показывается (потерянный URL = отзыв и новая ссылка). Состояния — производные чтения: действует, истёк срок, отозвана, отправка завершена, владение изменилось. Доставка — связка исходящего ответа; события клиента (исправление/позвать человека/вернуться) не расходуют лимит, кроме отправки исправления.",
		Links:     make([]casesAPI.ContinuationLinkView, 0, len(links)),
		Uses:      make([]casesAPI.ContinuationUseView, 0, len(f.contUses)),
	}
	for _, link := range links {
		state := casesDomain.DeriveContinuationLinkState(casesDomain.ContinuationLinkStateInput{
			Now: now, ExpiresAt: link.ExpiresAt, Revoked: revoked[link.LinkID],
			AcceptedCorrections: accepted[link.LinkID], ConsumptionLimit: link.ConsumptionLimit,
			OwnerEpochPinned: link.OwnerEpoch, OwnerEpochCurrent: ownerEpoch,
		})
		view.Links = append(view.Links, casesAPI.ContinuationLinkView{
			LinkID: link.LinkID, WorkItemID: link.WorkItemID,
			Purposes: append([]string(nil), link.Purposes...), State: state,
			Delivered: f.contDelivered[link.LinkID], AcceptedUses: accepted[link.LinkID],
			ConsumptionLimit: link.ConsumptionLimit, Question: link.Question,
			ParticipantRef: link.ParticipantRef, ExpiresAt: link.ExpiresAt,
		})
	}
	for _, use := range f.contUses {
		if use.CaseID != caseID {
			continue
		}
		view.Uses = append(view.Uses, casesAPI.ContinuationUseView{
			UseID: use.UseID, LinkID: use.LinkID, UseKind: use.UseKind,
			FactID: use.FactID, FactVersion: use.FactVersionNo, SubmittedAt: use.SubmittedAt,
		})
	}
	return view
}

// TenantName answers the display name of the fixture tenant (the
// principal-free narrow read of the Story 2.10 transport — the public
// continuation page header honestly shows whose conversation it is).
func (identityFixture) TenantName(_ context.Context, tenant id.UUID) (string, error) {
	if tenant != tenantID {
		return "", identityDomain.ErrNotFound
	}
	return "Магазин Олега", nil
}
