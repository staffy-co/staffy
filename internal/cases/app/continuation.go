package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// The Story 2.10 commands, the public surface and the operator panel
// ---------------------------------------------------------------------------
//
// The two operator commands follow the frozen gate ladder of 2.5..2.9:
// the rights gate (review OR configuration) fires BEFORE every material
// guard and refuses with the forbidden word (the honest 403 of the scope
// sentence); the dictionary guards refuse with the honest 400; the
// terminal Work Item refuses honestly. The command material is
// idempotent by the tenant-scoped key with the stored outcome (the
// replay answer comes before the state guards) — the stored outcome
// NEVER carries the capability token (the mask word only: a replayed
// issuance returns the empty URL, the old link never resurrects, a lost
// URL means a revocation and a fresh issuance).
//
// The public surface carries NO session: the opaque token IS the
// credential (the playback-permit precedent — a request-carried
// credential). The parse is fail-closed (a wrong length, a wrong
// alphabet or a non-UUID tenant half is the same generic refusal as an
// unknown hash); the scope candidate from the token is verified by the
// stored hash under the RLS scope BEFORE any case material is read (the
// AD-10 honesty — a wrong tenant simply finds no row). The state of the
// moment is derived read-time: the consumed word renders the terminal
// «материал отправлен» page, every other non-valid word renders the
// single generic refusal — the causes are never distinguished to the
// client. The client actions write append-only rows (the correction
// consumes the limit; the human request and the return are events) and
// nothing is ever mutated: the Work Item, the facts, the owner and the
// mode stay byte-identical through every public action.

// continuationScopeText is the honest scope refusal of the continuation
// commands (the shared «review или configuration» vocabulary of the
// 2.5..2.9 rights gates): an audit-only member reads the case surface
// but never issues or revokes the links.
const continuationScopeText = "Команды ссылок продолжения требуют полномочия review или configuration: участники только с audit не выдают и не отзывают ссылки."

// IssueContinuationLink issues one purpose-bound continuation link of
// the Work Item: the operator with the review OR the configuration
// scope (the gate fires BEFORE the material guards), the domain guards
// (the purpose dictionary, the ttl/limit bounds, the operator texts),
// the alive Work Item (the terminal states refuse honestly — the
// continuation never extends a finished element). The capability is
// generated server-side (the crypto-random half never travels through
// the command); the database stores only its SHA-256. The fresh answer
// carries the URL token once (the operator weaves it into the reply);
// the replayed command returns the stored outcome with the honest mask —
// the token is never recoverable from the storage.
func (s *Service) IssueContinuationLink(ctx context.Context, command casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error) {
	// The rights gate FIRST (the M1 precedent).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.ContinuationIssuance{}, fmt.Errorf("%w: %s", domain.ErrForbidden, continuationScopeText)
	}
	// The domain guards: the purpose set, the lifetime bounds and the
	// operator texts (the honest 400 with the dictionary words).
	if err := rule("issue_continuation_link", casesDomain.GuardContinuationPurposes(command.Purposes)); err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	if err := rule("issue_continuation_link", casesDomain.GuardContinuationTTL(command.TTLMinutes)); err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	if command.ConsumptionLimit == 0 {
		// The default of the dictionary: one accepted submission — the
		// FR-16 «exactly one submission per request».
		command.ConsumptionLimit = casesDomain.DefaultContinuationConsumptionLimit
	}
	if err := rule("issue_continuation_link", casesDomain.GuardContinuationConsumptionLimit(command.ConsumptionLimit)); err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	if err := rule("issue_continuation_link", casesDomain.GuardContinuationTexts(command.Question, command.Explanation)); err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	if len(command.ParticipantRef) < 1 || len(command.ParticipantRef) > 128 {
		return casesAPI.ContinuationIssuance{}, rule("issue_continuation_link", fmt.Errorf("адрес участника ссылки обязан быть 1..128 символами (канал клиента, которому вплетается URL)"))
	}
	// The request hash covers the operator material (the anchors, the
	// purposes, the texts, the participant reference and the bounds).
	// The capability randomness is generated ONCE per command execution
	// and never enters the hash: the replay of the exact repeat returns
	// the stored outcome (the masked issuance answer); the same key with
	// another material is the honest key conflict.
	hash := requestHash(struct {
		Case        string   `json:"case"`
		WorkItem    string   `json:"work_item"`
		Purposes    []string `json:"purposes"`
		Question    string   `json:"question"`
		Explain     string   `json:"explanation"`
		Participant string   `json:"participant_ref"`
		TTL         int      `json:"ttl_minutes"`
		Limit       int      `json:"consumption_limit"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.Purposes,
		command.Question, command.Explanation, command.ParticipantRef,
		command.TTLMinutes, command.ConsumptionLimit})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "issue_continuation_link", command.IdempotencyKey, hash); replay || err != nil {
		// The replayed issuance honestly refuses to reconstruct the URL:
		// the token lives only in the first answer (the preimage is not
		// recoverable from the stored hash).
		if replay {
			return casesAPI.ContinuationIssuance{Outcome: outcome}, nil
		}
		return casesAPI.ContinuationIssuance{}, err
	}
	// The Work Item of the command scope: it must exist in this case and
	// be alive (the write-command boundary of 2.2).
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, command.CaseID, []id.UUID{command.WorkItemID})
	if err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	current, ok := currents[command.WorkItemID]
	if !ok {
		return casesAPI.ContinuationIssuance{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.ContinuationIssuance{}, rule("issue_continuation_link", fmt.Errorf("%s", casesDomain.TerminalWorkItemContinuationText))
	}
	// The generated ids and the capability material: the link id, the
	// crypto-random half with its SHA-256 (the only stored trace), the
	// outbox event and the correlation.
	linkID := command.LinkID
	if !linkID.Valid() {
		if linkID, err = s.ids.New(command.Now); err != nil {
			return casesAPI.ContinuationIssuance{}, err
		}
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
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.ContinuationIssuance{}, err
		}
	}
	purposesJSON, err := marshalPurposes(command.Purposes)
	if err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	expires := command.Now.UTC().Truncate(time.Microsecond).Add(time.Duration(command.TTLMinutes) * time.Minute)
	// The stored outcome: the link id, the honest mask word — the token
	// NEVER enters the command record (the replay answer shows «выдан»).
	outcome := casesAPI.WorkOutcome{
		Command: "issue_continuation_link", LinkID: linkID,
		ContinuationIssued: true, ContinuationExpires: expires,
	}
	outcomeJSON, err := marshalOutcome(outcome)
	if err != nil {
		return casesAPI.ContinuationIssuance{}, err
	}
	if _, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "issue_continuation_link", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		LinkID: linkID, ContinuationPurposes: purposesJSON,
		ContinuationQuestion: command.Question, ContinuationExplanation: command.Explanation,
		ContinuationParticipantRef: command.ParticipantRef,
		ContinuationTTLMinutes:     command.TTLMinutes, ContinuationLimit: command.ConsumptionLimit,
		ContinuationTokenHash: tokenHash, CorrelationID: correlation, OutboxEventID: outboxEventID,
		Outcome: outcomeJSON, Now: command.Now,
	}); err != nil {
		return casesAPI.ContinuationIssuance{}, err
	} else if replay {
		// The raced identical issuance answered by the concurrent
		// command: the honest replay (the token of THIS execution is
		// discarded — the stored row is the truth).
		return casesAPI.ContinuationIssuance{Outcome: outcome}, nil
	}
	// The fresh answer: the one-time capability material.
	return casesAPI.ContinuationIssuance{
		Outcome: outcome, URL: casesAPI.ContinuationURLPath(token), Token: token, Expires: expires,
	}, nil
}

// RevokeContinuationLink revokes one link: the append-only revocation
// row with the bounded reason; the read-time derivation blocks the
// surface without disclosure. Nothing is ever mutated; the second revoke
// of the same link refuses honestly (the expectation ended once).
func (s *Service) RevokeContinuationLink(ctx context.Context, command casesAPI.RevokeContinuationLinkCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST (the M1 precedent).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, continuationScopeText)
	}
	// The honest reason bounds (the dictionary-free text of the row).
	if err := rule("revoke_continuation_link", casesDomain.GuardReason(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the revocation material (the anchors, the
	// link and the reason).
	hash := requestHash(struct {
		Case   string `json:"case"`
		Item   string `json:"work_item"`
		Link   string `json:"link"`
		Reason string `json:"reason"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.LinkID.String(), command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "revoke_continuation_link", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	revocationID := command.RevocationID
	if !revocationID.Valid() {
		if revocationID, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	outcome := casesAPI.WorkOutcome{Command: "revoke_continuation_link", LinkID: command.LinkID}
	outcomeJSON, err := marshalOutcome(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "revoke_continuation_link", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		LinkID: command.LinkID, RevocationID: revocationID, ContinuationReason: command.Reason,
		CorrelationID: correlation, OutboxEventID: outboxEventID,
		Outcome: outcomeJSON, Now: command.Now,
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// continuationPublicScope builds the anonymous scope of the public
// surface: the tenant candidate comes from the verified token half (the
// parse is fail-closed), the principal is INERT (the reads and the
// public writes never gate on it — the RLS checks the tenant only; the
// write rows carry the resolved link id as their actor). The principal
// slot carries the tenant itself (a valid deterministic uuid — the
// honest placeholder of the no-session surface).
func continuationPublicScope(tenantID id.UUID) (tenancy.Context, error) {
	return tenancy.New(tenantID, tenantID)
}

// resolveContinuation parses the opaque token and resolves the link by
// the stored hash (the fail-closed parse and the ErrNotFound lookup are
// the SAME generic refusal for the caller — the causes are never
// distinguished).
func (s *Service) resolveContinuation(ctx context.Context, token string, now time.Time) (tenancy.Context, casesDomain.ContinuationLinkLookup, []byte, error) {
	tenantID, random, err := casesDomain.ParseContinuationToken(token)
	if err != nil {
		return tenancy.Context{}, casesDomain.ContinuationLinkLookup{}, nil, casesDomain.ErrNotFound
	}
	tokenHash, err := casesDomain.ContinuationTokenHash(random)
	if err != nil {
		return tenancy.Context{}, casesDomain.ContinuationLinkLookup{}, nil, casesDomain.ErrNotFound
	}
	scope, err := continuationPublicScope(tenantID)
	if err != nil {
		return tenancy.Context{}, casesDomain.ContinuationLinkLookup{}, nil, casesDomain.ErrNotFound
	}
	lookup, err := s.repository.LookupContinuationByTokenHash(ctx, scope, tokenHash, now)
	if err != nil {
		return tenancy.Context{}, casesDomain.ContinuationLinkLookup{}, nil, err
	}
	return scope, lookup, tokenHash, nil
}

// ContinueSurface reads the material of the protected page: the state of
// the moment (the valid word renders the form; the consumed word renders
// the terminal «материал отправлен» page; every other word — the expired,
// the revoked, the owner-stale and the unknown-hash/parse refusals —
// renders the single generic refusal without disclosure), the confirmed
// facts of the Work Item of the link (the correctable select) and the
// operator texts. The transport composes the header (the tenant name via
// the identity service, the role constant of the slice).
func (s *Service) ContinueSurface(ctx context.Context, token string, now time.Time) (casesAPI.ContinuationSurface, error) {
	scope, lookup, _, err := s.resolveContinuation(ctx, token, now)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			// The generic refusal: an unknown token, a foreign tenant, a
			// wrong shape — the same single page, the same honest word.
			return casesAPI.ContinuationSurface{Kind: casesAPI.ContinuationSurfaceInvalid}, nil
		}
		return casesAPI.ContinuationSurface{}, err
	}
	switch lookup.State {
	case casesDomain.ContinuationStateValid:
	case casesDomain.ContinuationStateConsumed:
		// The terminal page: the material was submitted, the link is
		// finished — the repeat of the URL stays honest and stable.
		return casesAPI.ContinuationSurface{Kind: casesAPI.ContinuationSurfaceConsumed}, nil
	default:
		// The expired, revoked and owner-stale words: the SAME generic
		// refusal as the unknown token (the client never sees the
		// cause; the operator panel carries the exact word).
		return casesAPI.ContinuationSurface{Kind: casesAPI.ContinuationSurfaceInvalid}, nil
	}
	// The confirmed facts of the Work Item of the link (the 2.2
	// derivation — the page offers the confirmed facts only; another
	// Work Item or another participant never appears). The read runs
	// under the public tenant scope the hash already verified.
	facts, err := s.repository.ReadFactsOfCase(ctx, scope, lookup.Link.CaseID)
	if err != nil {
		return casesAPI.ContinuationSurface{}, err
	}
	views := []casesAPI.ContinuationFactView{}
	accepted := 0
	for _, use := range lookup.Uses {
		if casesDomain.ContinuationUseConsumesLimit(use.UseKind) {
			accepted++
		}
	}
	for _, material := range facts {
		if len(material.Versions) == 0 || material.Versions[len(material.Versions)-1].WorkItemID != lookup.Link.WorkItemID {
			continue
		}
		status, versionNo, _ := casesDomain.ConfirmedFactView(material.Versions, material.Confirmations)
		if status != casesDomain.FactStatusConfirmed {
			continue
		}
		for _, version := range material.Versions {
			if version.VersionNo == versionNo {
				views = append(views, casesAPI.ContinuationFactView{
					FactID: material.Anchor.FactID, MaterialKey: material.Anchor.MaterialKey,
					ValueText: string(version.Value),
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
		ExpiresAt:   lookup.Link.ExpiresAt,
		LimitLeft:   lookup.Link.ConsumptionLimit - accepted,
		Vehicle:     s.continuationVehicle(ctx, scope, lookup, facts),
	}, nil
}

// continuationVehicle builds the Story 4.2 vehicle identification block
// of the public form: the schema of the pinned pack of the case of the
// link through the narrow taxonomy port, the facts of the WORK ITEM of
// the link only (the one-link-one-work-item discipline of 2.10 — the
// confirmable attributes are the material of this surface), the
// read-time profile and the readback words of the VIN (the blocks
// 4-4-4-4-1 and the focus re-ask of the disputed symbols). The block is
// absent when the pack carries no schema or the work item carries no
// identification facts (nothing to confirm — the honest absence, never
// an invented question). The FULL VIN renders on this surface and the
// operator work card only — every other path carries the mask.
func (s *Service) continuationVehicle(ctx context.Context, scope tenancy.Context, lookup casesDomain.ContinuationLinkLookup, facts []casesPort.FactMaterial) *casesAPI.ContinuationVehicleView {
	if s.taxonomy == nil {
		return nil
	}
	detail, err := s.repository.ReadCase(ctx, scope, lookup.Link.CaseID)
	if err != nil || detail.Case.PackID == "" {
		return nil
	}
	schemaPort, err := s.taxonomy.PackIdentificationSchema(detail.Case.PackID, detail.Case.PackVersion)
	if err != nil || len(schemaPort.MaterialKeys) == 0 {
		return nil
	}
	schema := casesDomain.VehicleSchema{
		MaterialKeys:    make([]casesDomain.VehicleKeySpec, 0, len(schemaPort.MaterialKeys)),
		AlternativeSets: schemaPort.AlternativeSets,
	}
	for _, key := range schemaPort.MaterialKeys {
		schema.MaterialKeys = append(schema.MaterialKeys, casesDomain.VehicleKeySpec{
			Key: key.Key, Label: key.Label, Kind: key.Kind, Min: key.Min, Max: key.Max, Optional: key.Optional,
		})
	}
	// The confirmable facts: the schema-keyed facts whose CURRENT scope is
	// the work item of the link (the correction gate twin — the store
	// re-validates the same boundary under the locks).
	inputs := make([]casesDomain.VehicleFactInput, 0, len(facts))
	for _, material := range facts {
		if len(material.Versions) == 0 || material.Versions[len(material.Versions)-1].WorkItemID != lookup.Link.WorkItemID {
			continue
		}
		inputs = append(inputs, casesDomain.VehicleFactInput{
			FactID: material.Anchor.FactID, MaterialKey: material.Anchor.MaterialKey,
			Versions: material.Versions, Confirmations: material.Confirmations,
		})
	}
	if len(inputs) == 0 {
		return nil
	}
	profile := casesDomain.DeriveVehicleProfile(schema, inputs)
	block := &casesAPI.ContinuationVehicleView{
		Profile:    profile,
		Readback:   []casesDomain.VINReadbackBlock{},
		FocusWords: []string{},
	}
	for _, attribute := range profile.Attributes {
		if attribute.Kind == "vin" && attribute.NormalizedValue != nil {
			block.Readback = casesDomain.ReadbackBlocks(attribute.NormalizedValue.Normalized)
			block.FocusWords = casesDomain.ReadbackFocus(attribute.NormalizedValue.Normalized, attribute.NormalizedValue.CyrillicAmbiguities)
			break
		}
	}
	return block
}

// SubmitCorrection writes one public fact correction through the link:
// the parse and the hash lookup refuse with the same generic word as
// every dead link; the store re-derives the state, re-validates the
// confirmed fact of the Work Item and the value guards under the locks,
// writes the append-only unverified version with the
// protected-continuation provenance, consumes the limit and emits the
// metadata-only event. The answer carries the new version number (the
// landing page announces «отправлено — оператор проверит»).
func (s *Service) SubmitCorrection(ctx context.Context, token string, factID id.UUID, rawValue []byte, now time.Time) (int64, error) {
	scope, _, tokenHash, err := s.resolveContinuation(ctx, token, now)
	if err != nil {
		return 0, err
	}
	// The value guard BEFORE the transaction (the honest re-render of
	// the form with the preserved input — the data never disappears on
	// a validation error).
	value, _, err := casesDomain.FactValue(rawValue)
	if err != nil {
		return 0, rule("continuation_correct", err)
	}
	useID, err := s.ids.New(now)
	if err != nil {
		return 0, err
	}
	outboxEventID, err := s.ids.New(now)
	if err != nil {
		return 0, err
	}
	correlation, err := s.ids.New(now)
	if err != nil {
		return 0, err
	}
	versionNo, err := s.repository.SubmitContinuationCorrection(ctx, casesPort.ContinuationCorrectionParams{
		Scope: scope, TokenHash: tokenHash, FactID: factID, Value: value,
		UseID: useID, OutboxEventID: outboxEventID, CorrelationID: correlation, Now: now,
	})
	if err != nil {
		return 0, err
	}
	return versionNo, nil
}

// SubmitHuman records the «позвать человека» event through the link: the
// use row without material and without consumption, the outbox event;
// nothing is ever mutated.
func (s *Service) SubmitHuman(ctx context.Context, token string, now time.Time) error {
	return s.submitContinuationEvent(ctx, token, casesDomain.ContinuationUseHumanRequested, now)
}

// SubmitVehicleConfirmation writes the Story 4.2 client confirmation
// of the vehicle identification attributes through the link: the parse
// and the hash lookup refuse with the same generic word as every dead
// link; the store re-derives the state, re-validates every (fact,
// version) pair under the locks (the confirmation binds the EXACT
// version the client saw), writes the append-only client confirmations
// (confirmer=client — the first producer of the reserved 2.2 word) and
// the limit-consuming use row in one transaction. The answer of a
// replayed double-click is the consumed terminal page (the same
// idempotency-by-consumption as the correction).
func (s *Service) SubmitVehicleConfirmation(ctx context.Context, token string, items []casesAPI.VehicleConfirmationItem, now time.Time) error {
	scope, _, tokenHash, err := s.resolveContinuation(ctx, token, now)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return rule("vehicle_confirm", fmt.Errorf("подтверждение пусто: отметьте хотя бы один признак автомобиля"))
	}
	useID, err := s.ids.New(now)
	if err != nil {
		return err
	}
	outboxEventID, err := s.ids.New(now)
	if err != nil {
		return err
	}
	correlation, err := s.ids.New(now)
	if err != nil {
		return err
	}
	confirmations := make([]casesPort.VehicleConfirmationItem, 0, len(items))
	for _, item := range items {
		confirmationID, err := s.ids.New(now)
		if err != nil {
			return err
		}
		confirmations = append(confirmations, casesPort.VehicleConfirmationItem{
			ConfirmationID: confirmationID, FactID: item.FactID, VersionNo: item.VersionNo,
		})
	}
	return s.repository.SubmitVehicleConfirmation(ctx, casesPort.SubmitVehicleConfirmationParams{
		Scope: scope, TokenHash: tokenHash, Confirmations: confirmations,
		UseID: useID, OutboxEventID: outboxEventID, CorrelationID: correlation, Now: now,
	})
}

// SubmitReturn records the «вернуться в разговор» event through the
// link: the use row without material and without consumption, the
// outbox event; the Work Item, the facts, the owner and the mode stay
// byte-identical (the machine never touches them).
func (s *Service) SubmitReturn(ctx context.Context, token string, now time.Time) error {
	return s.submitContinuationEvent(ctx, token, casesDomain.ContinuationUseReturnedToConversation, now)
}

// submitContinuationEvent is the shared event path (the use kind word
// is the only difference — the events are free, they consume nothing).
func (s *Service) submitContinuationEvent(ctx context.Context, token, useKind string, now time.Time) error {
	scope, _, tokenHash, err := s.resolveContinuation(ctx, token, now)
	if err != nil {
		return err
	}
	useID, err := s.ids.New(now)
	if err != nil {
		return err
	}
	outboxEventID, err := s.ids.New(now)
	if err != nil {
		return err
	}
	correlation, err := s.ids.New(now)
	if err != nil {
		return err
	}
	return s.repository.SubmitContinuationEvent(ctx, casesPort.ContinuationEventParams{
		Scope: scope, TokenHash: tokenHash, UseKind: useKind,
		UseID: useID, OutboxEventID: outboxEventID, CorrelationID: correlation, Now: now,
	})
}

// marshalPurposes renders the canonical purposes JSON of the link row.
func marshalPurposes(purposes []string) ([]byte, error) {
	if purposes == nil {
		purposes = []string{}
	}
	return json.Marshal(purposes)
}

// marshalOutcome renders the stored outcome JSON of the commands (the
// token NEVER enters it).
func marshalOutcome(outcome casesAPI.WorkOutcome) ([]byte, error) {
	return json.Marshal(outcome)
}

// continuationView assembles the operator panel section of the case view
// (the 2.9 commitmentView precedent): the links of the case with the
// read-time state derivations — the CURRENT owner epoch of the case feeds
// the owner_stale word (the replay of an old event never resurrects a
// dead link) — the delivered binding of the outgoing replies (the FR-16
// «the main channel shows whether the link was delivered»), the use
// accounting against the limit (the accepted corrections only — the
// human/return events never consume) and the event feed. The token never
// appears: the hash is the only stored trace, the panel shows the honest
// «токен выдан» material of the row (the question, the participant, the
// bounds).
func (s *Service) continuationView(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail) (casesAPI.ContinuationView, error) {
	material, err := s.repository.ReadContinuations(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.ContinuationView{}, err
	}
	// The CURRENT owner epoch of the owner_stale derivation (the
	// read-time truth — never a stored status word).
	ownershipTerms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.ContinuationView{}, err
	}
	ownerEpoch := int64(0)
	if owner := casesDomain.CurrentOwnership(ownershipTerms); owner != nil {
		ownerEpoch = owner.Epoch
	}
	now := s.clock.Now()
	revoked := map[id.UUID]bool{}
	for _, row := range material.Revocations {
		revoked[row.LinkID] = true
	}
	accepted := map[id.UUID]int{}
	for _, use := range material.Uses {
		if casesDomain.ContinuationUseConsumesLimit(use.UseKind) {
			accepted[use.LinkID]++
		}
	}
	view := casesAPI.ContinuationView{
		PanelText: "Ссылка продолжения — одноразовая возможность клиента: токен существует только в ответе команды выдачи и повторно не показывается (потерянный URL = отзыв и новая ссылка). Состояния — производные чтения: действует, истёк срок, отозвана, отправка завершена, владение изменилось. Доставка — связка исходящего ответа; события клиента (исправление/позвать человека/вернуться) не расходуют лимит, кроме отправки исправления.",
		Links:     make([]casesAPI.ContinuationLinkView, 0, len(material.Links)),
		Uses:      make([]casesAPI.ContinuationUseView, 0, len(material.Uses)),
	}
	for _, link := range material.Links {
		state := casesDomain.DeriveContinuationLinkState(casesDomain.ContinuationLinkStateInput{
			Now: now, ExpiresAt: link.ExpiresAt, Revoked: revoked[link.LinkID],
			AcceptedCorrections: accepted[link.LinkID], ConsumptionLimit: link.ConsumptionLimit,
			OwnerEpochPinned: link.OwnerEpoch, OwnerEpochCurrent: ownerEpoch,
		})
		view.Links = append(view.Links, casesAPI.ContinuationLinkView{
			LinkID: link.LinkID, WorkItemID: link.WorkItemID,
			Purposes: append([]string(nil), link.Purposes...), State: state,
			Delivered: material.Delivered[link.LinkID], AcceptedUses: accepted[link.LinkID],
			ConsumptionLimit: link.ConsumptionLimit, Question: link.Question,
			ParticipantRef: link.ParticipantRef, ExpiresAt: link.ExpiresAt,
		})
	}
	for _, use := range material.Uses {
		view.Uses = append(view.Uses, casesAPI.ContinuationUseView{
			UseID: use.UseID, LinkID: use.LinkID, UseKind: use.UseKind,
			FactID: use.FactID, FactVersion: use.FactVersionNo, SubmittedAt: use.SubmittedAt,
		})
	}
	return view, nil
}
