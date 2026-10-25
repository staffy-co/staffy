package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/artemiosu/staffy/internal/action/api"
	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// Story 4.10 — the order servicing application contour: the ONE record
// command of the repeat-contact request (the operator with the review
// scope records WHAT THE CLIENT ASKED with the LIVE proof pins of the
// contact case assurance) and the Order Servicing View read (the panel
// feed with the LIVE derivations of the client words and the cancel
// feasibility). The Identity Proof Gate reads the assurance records of
// the contact case ONLY (FR-11/2-3 structural: the Caller ID, the chat
// profile name and the knowledge of the order number are NEVER a
// proof — no other input exists in this contour); the proof pins copy
// into the request at the moment of the record (the scope, the level
// and the time of the check are preserved — AC-4); the DISCLOSURE
// (including the very existence of the order) requires the LIVE
// verified trust — a denied request answers the gate word WITHOUT the
// lookup outcome (AC-1). The lookup reads the RECORDED material of the
// contour only (AG-4/OQ-18); the phone/email kinds do NOT exist
// (OQ-16). The command NEVER executes anything (the OQ-3 fence, the
// AG-4 manual contour — the cancel is SPOKEN in its four states, the
// change is the manager's handwork) and NEVER creates or changes an
// order operation (FR-62: the resolved_operation_id references the
// EXISTING operation; the write path is the ONE append).

// orderServicingScopeText is the honest scope refusal of the contour:
// the words BEFORE any material (the decision-context 2-14 precedent).
const orderServicingScopeText = "Запись обращения по заказу и чтение панели обслуживания требуют полномочия review: участник без полномочия review не записывает обращения и не читает панель обслуживания."

// orderServicingPortsMissingText is the honest refusal of the unwired
// contour (the preview precedent — never a nil panic).
const orderServicingPortsMissingText = "контур обслуживания заказа не подключён: сверьте конфигурацию поверхности"

type (
	RecordOrderServicingRequestCommand = api.RecordOrderServicingRequestCommand
	OrderServicingOutcome              = api.OrderServicingOutcome
)

// The compile-time proof of the narrow interface registration (the
// transport type-asserts api.OrderServicingService — the
// OrderStatusService precedent).
var _ api.OrderServicingService = (*Service)(nil)

// requireOrderServicingPorts refuses honestly when the contour is not
// wired: the servicing repository, the LIVE assurance source and the
// projection material reads (the operations, the receipts, the
// verifications, the corrections, the facts and the LIVE case material
// of the estimate) are mandatory.
func (s *Service) requireOrderServicingPorts() error {
	if s.orderServicing == nil || s.assurance == nil || s.operations == nil || s.dispatches == nil ||
		s.verifications == nil || s.corrections == nil || s.orderFacts == nil || s.caseMaterial == nil {
		return fmt.Errorf("%s", orderServicingPortsMissingText)
	}
	return nil
}

// authorizeOrderServicing is the read/record gate of the contour: the
// membership FIRST (a non-member is the honest 404), the review scope
// 403 BEFORE any servicing material (the words-first discipline).
func (s *Service) authorizeOrderServicing(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Review); err != nil {
		if errors.Is(err, identityDomain.ErrNotFound) {
			return tenancy.Context{}, domain.ErrNotFound
		}
		if errors.Is(err, identityDomain.ErrForbidden) {
			return tenancy.Context{}, fmt.Errorf("%w: %s", domain.ErrForbidden, orderServicingScopeText)
		}
		return tenancy.Context{}, err
	}
	return tenancy.New(actor.ID, tenantID)
}

// servicingAssurancePins is the LIVE proof-pin material of the contact
// case assurance (the 2-3 derivation read through the port): the proof
// state (none/basic/verified — the BY NAME mirror of the source
// dictionary; step_up — the threshold window word), the identity
// method, the issued/expiry times, the allowed methods of the
// conversation modality (the gate word render) and the fallback words
// (the honest options of the refusal).
type servicingAssurancePins struct {
	State          string
	Method         string
	IssuedAt       time.Time
	ExpiresAt      time.Time
	AllowedMethods []string
	FallbackWords  []string
}

// servicingProofPins derives the proof pins of ONE live assurance view
// (the record command input — AC-4): the step-up window governs the
// state word (the checks are suspended — the disclosure refuses); the
// identity level mirrors the source dictionary BY NAME; the issued
// time rides the LATEST passed unexpired identity record (the honest
// pin of the check time — the expired records stay in the history but
// never in the pins).
func servicingProofPins(view casesAPI.AssuranceView, now time.Time) servicingAssurancePins {
	pins := servicingAssurancePins{
		State:          domain.ProofStateNone,
		Method:         view.State.IdentityMethod,
		ExpiresAt:      view.State.IdentityExpiresAt,
		AllowedMethods: view.AllowedMethods,
		FallbackWords:  view.FallbackOptions,
	}
	if view.State.StepUp {
		pins.State = domain.ProofStateStepUp
	} else {
		switch view.State.IdentityLevel {
		case casesDomain.AssuranceBasic:
			pins.State = domain.ProofStateBasic
		case casesDomain.AssuranceVerified:
			pins.State = domain.ProofStateVerified
		default:
			pins.State = domain.ProofStateNone
		}
	}
	var issued time.Time
	for _, record := range view.Records {
		if record.Kind == casesDomain.VerificationKindIdentity &&
			record.Outcome == casesDomain.VerificationPassed &&
			record.ExpiresAt.After(now) && record.IssuedAt.After(issued) {
			issued = record.IssuedAt
		}
	}
	pins.IssuedAt = issued
	return pins
}

// RecordOrderServicingRequest records ONE repeat-contact request of
// the existing order (FR-62 — the RECORDED EVENT of the servicing):
// the rights gate (review) FIRST, the domain guard of the dictionary
// words (the intent, the kind — the phone/email kinds do NOT exist,
// OQ-16; the value bound), the LIVE assurance read of the contact case
// (the proof pins copy at the moment of the record — a refused read
// refuses the command, NEVER a masked «ok» pin), the lookup BY KEY
// over the RECORDED material of the tenant (the outcome —
// unique_match/not_found/ambiguous — is the recorded pin of the
// request: the enumeration of the orders does NOT exist), the REPLAY
// check BY KEY (the repeated command at the SAME trust state answers
// the SAVED request — never a second row; the changed trust state
// (none → verified) is a NEW request — the trust ladder stays
// visible) and the ONE append (the request_no ladder and the UNIQUE
// full request key under the per-case advisory lock). The command
// NEVER executes anything (the OQ-3 fence, AG-4) and NEVER creates or
// changes an order operation (the resolved anchor references the
// EXISTING operation only).
func (s *Service) RecordOrderServicingRequest(ctx context.Context, command RecordOrderServicingRequestCommand) (OrderServicingOutcome, error) {
	if err := s.requireOrderServicingPorts(); err != nil {
		return OrderServicingOutcome{}, err
	}
	scope, err := s.authorizeOrderServicing(ctx, command.Actor, command.TenantID)
	if err != nil {
		return OrderServicingOutcome{}, err
	}
	// The dictionary guards BEFORE any read (the honest work-rule
	// refusal of the out-of-dictionary words).
	value := strings.TrimSpace(command.IdentifierValue)
	if err := domain.GuardOrderServicingRequest(command.Intent, command.IdentifierKind, value); err != nil {
		return OrderServicingOutcome{}, err
	}
	// The LIVE assurance read of the contact case: the case anchor (the
	// honest 404 of a foreign case — the RLS blindness mirror) AND the
	// proof-pin source in ONE read. A refused read refuses the command —
	// never a masked «ok» proof pin (the nil-port honesty of the spec).
	assuranceView, err := s.assurance.AssuranceView(ctx, command.Actor, command.TenantID, command.CaseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return OrderServicingOutcome{}, domain.ErrOrderServicingNoCase
		}
		return OrderServicingOutcome{}, err
	}
	pins := servicingProofPins(assuranceView, command.Now)
	// The lookup BY KEY over the RECORDED material of the tenant (the
	// kind word is dictionary-clean — the store answers the honest empty
	// for a foreign word, never a query).
	candidates, err := s.orderServicing.LookupOrderByIDentifier(ctx, scope, command.IdentifierKind, value)
	if err != nil {
		return OrderServicingOutcome{}, err
	}
	outcome, resolved := domain.DeriveLookupOutcome(candidates)
	// The DISCLOSURE gate (AC-1): the proof state governs — verified
	// ONLY; the none/basic/step_up states answer the gate word WITHOUT
	// the lookup outcome in the client words (the request still records
	// — AC-6: the append fixes the ask, not the result).
	disclosure := domain.DisclosureDenied
	if domain.ProofStateAllowsDisclosure(pins.State) {
		disclosure = domain.DisclosureAllowed
	}
	// The REPLAY check BY KEY: the same command (the actor, the intent,
	// the kind, the value) at the SAME trust state — the SAVED request
	// answers (the recorded outcome is the pin, never a re-derivation).
	existing, err := s.orderServicing.ReadCaseServicingRequests(ctx, scope, command.CaseID)
	if err != nil {
		return OrderServicingOutcome{}, err
	}
	for _, request := range existing {
		if request.ActorID == command.Actor.ID && request.Intent == command.Intent &&
			request.IdentifierKind == command.IdentifierKind && request.IdentifierValue == value &&
			request.ProofState == pins.State {
			return OrderServicingOutcome{Request: request, Replay: true, SourceWord: domain.ServicingRequestRecordedWord}, nil
		}
	}
	request, err := s.orderServicing.AppendOrderServicingRequest(ctx, port.AppendOrderServicingRequestParams{
		Scope:               scope,
		CaseID:              command.CaseID,
		ConversationID:      command.ConversationID,
		Intent:              command.Intent,
		IdentifierKind:      command.IdentifierKind,
		IdentifierValue:     value,
		ResolvedOperationID: resolved,
		LookupOutcome:       outcome,
		ProofState:          pins.State,
		ProofMethod:         pins.Method,
		ProofIssuedAt:       pins.IssuedAt,
		ProofExpiresAt:      pins.ExpiresAt,
		Disclosure:          disclosure,
		ActorID:             command.Actor.ID,
		Now:                 command.Now,
	})
	if err != nil {
		if errors.Is(err, port.ErrOrderServicingRequestDuplicate) {
			// The raced twin of the same command landed first: the saved
			// request answers — the honest replay, never a masked write.
			saved, readErr := s.orderServicing.ReadCaseServicingRequests(ctx, scope, command.CaseID)
			if readErr == nil {
				for _, row := range saved {
					if row.ActorID == command.Actor.ID && row.Intent == command.Intent &&
						row.IdentifierKind == command.IdentifierKind && row.IdentifierValue == value &&
						row.ProofState == pins.State {
						return OrderServicingOutcome{Request: row, Replay: true, SourceWord: domain.ServicingRequestRecordedWord}, nil
					}
				}
			}
			return OrderServicingOutcome{}, err
		}
		return OrderServicingOutcome{}, err
	}
	return OrderServicingOutcome{Request: request, Replay: false, SourceWord: domain.ServicingRequestRecordedWord}, nil
}

// ReadOrderServicingView reads the Order Servicing View of one case
// (the panel feed): the recorded requests (the append-only history —
// the proof pins stay as they were recorded, AC-4) with the LIVE
// derivations — the client words re-derive over the LIVE projection of
// the resolved operation (the 4-9 material re-reads: the ledger, the
// receipts, the verifications, the corrections, the facts, the
// estimate), the cancel feasibility re-derives over the LIVE supplier
// facts, and the source pins of the resolved match re-derive over the
// CURRENT recorded material. ZERO writes — the computed words never
// become a stored status (AC-2 of 4-9, the same discipline).
func (s *Service) ReadOrderServicingView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) (domain.OrderServicingView, error) {
	if err := s.requireOrderServicingPorts(); err != nil {
		return domain.OrderServicingView{}, err
	}
	scope, err := s.authorizeOrderServicing(ctx, actor, tenantID)
	if err != nil {
		return domain.OrderServicingView{}, err
	}
	requests, err := s.orderServicing.ReadCaseServicingRequests(ctx, scope, caseID)
	if err != nil {
		return domain.OrderServicingView{}, err
	}
	// The LIVE assurance read of the contact case (the methods and the
	// fallback words of the gate word render — the same live source the
	// record command reads; the case anchor rides the same honest 404).
	assuranceView, err := s.assurance.AssuranceView(ctx, actor, tenantID, caseID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return domain.OrderServicingView{}, domain.ErrOrderServicingNoCase
		}
		return domain.OrderServicingView{}, err
	}
	view := domain.OrderServicingView{
		Requests:              []domain.OrderServicingRequestView{},
		PhoneBoundaryWord:     domain.ServicingPhoneSearchBoundaryWord,
		ConnectorBoundaryWord: domain.ServicingConnectorBoundaryWord,
		NoDeliveryWord:        domain.ServicingNoDeliveryWord,
		RequestRecordedWord:   domain.ServicingRequestRecordedWord,
	}
	projectionCache := map[id.UUID]*domain.OrderStatusProjection{}
	for _, request := range requests {
		requestView := domain.OrderServicingRequestView{
			Request:         request,
			IntentShortWord: domain.ServicingIntentShortWords[request.Intent],
			KindShortWord:   domain.LookupKindShortWords[request.IdentifierKind],
			KindSourceWord:  domain.LookupKindSourceWords[request.IdentifierKind],
			OutcomeWord:     domain.LookupOutcomeWords[request.LookupOutcome],
			Sources:         []string{},
		}
		var projection *domain.OrderStatusProjection
		if request.ResolvedOperationID != id.UUID("") {
			projection = s.servicingProjectionOf(ctx, scope, actor, request.ResolvedOperationID, projectionCache, now)
			// The CURRENT source pins of the resolved match (read-time —
			// the sources may have grown since the record; the honest
			// re-derivation, never a stored list).
			candidates, lookupErr := s.orderServicing.LookupOrderByIDentifier(ctx, scope, request.IdentifierKind, request.IdentifierValue)
			if lookupErr == nil {
				for _, candidate := range candidates {
					if candidate.OperationID == request.ResolvedOperationID {
						requestView.Sources = candidate.Sources
						break
					}
				}
			}
		}
		requestView.ClientWords = domain.DeriveServicingClientWords(request, projection, assuranceView.AllowedMethods, assuranceView.FallbackOptions)
		if request.Intent == domain.ServicingIntentCancel && projection != nil {
			feasibility := domain.DeriveCancelFeasibility(*projection)
			requestView.Cancel = &feasibility
		}
		view.Requests = append(view.Requests, requestView)
	}
	return view, nil
}

// servicingProjectionOf reads the LIVE Order Status Projection of ONE
// resolved operation (the disclosure material of the view — cached per
// operation within one view read): the honest nil of a lost anchor
// never fails the view — the client words of the request answer the
// intent word only (the anchor-less retell).
func (s *Service) servicingProjectionOf(ctx context.Context, scope tenancy.Context, actor identityDomain.Principal, operationID id.UUID, cache map[id.UUID]*domain.OrderStatusProjection, now time.Time) *domain.OrderStatusProjection {
	if cached, ok := cache[operationID]; ok {
		return cached
	}
	operation, found, err := s.orderServicing.ReadResolvedOperation(ctx, scope, operationID)
	if err != nil || !found {
		return nil
	}
	material, err := s.assembleServicingMaterial(ctx, scope, actor, operation)
	if err != nil {
		return nil
	}
	projection := domain.DeriveOrderStatusProjection(material, now)
	cache[operationID] = &projection
	return &projection
}

// assembleServicingMaterial reads the WHOLE projection material of ONE
// resolved operation by the EXISTING port reads (the pure read-time
// assembly of the disclosure view — the 4-9 discipline over the
// operation anchor of the servicing): the ledger state with the
// dispatch_blocked parking cause, the committed receipt with the
// external ABCP Order ID, the verification tasks with the events, the
// corrections with the events, the supplier facts and the honest
// estimate material of the operation's case. A derivation refusal of
// the estimate is the honest EMPTY word (never a masked failure of the
// whole view).
func (s *Service) assembleServicingMaterial(ctx context.Context, scope tenancy.Context, actor identityDomain.Principal, operation domain.Operation) (domain.OrderStatusMaterial, error) {
	material := domain.OrderStatusMaterial{
		Operation: domain.OrderStatusOperationMaterial{
			Found:           true,
			OperationID:     operation.OperationID,
			WorkItemID:      operation.WorkItemID,
			Receipt:         nil,
			DispatchBlocked: false,
		},
	}
	ledger, err := s.operations.ReadOperationLedger(ctx, scope, operation.OperationID)
	if err != nil {
		return material, err
	}
	for _, entry := range ledger {
		if entry.Kind == domain.LedgerEntryDispatchBlocked {
			material.Operation.DispatchBlocked = true
		}
	}
	receipts, err := s.dispatches.ReadActionReceipts(ctx, scope, operation.OperationID)
	if err != nil {
		return material, err
	}
	for _, receipt := range receipts {
		if receipt.Outcome == domain.ReceiptOutcomeCommitted && receipt.References.ExternalObjectID != "" {
			material.Operation.Receipt = &domain.OrderStatusReceiptPin{
				ReceiptVersionNo: receipt.ReceiptVersionNo,
				ExternalOrderID:  receipt.References.ExternalObjectID,
				CreatedAt:        receipt.CreatedAt,
			}
			break
		}
	}
	tasks, err := s.verifications.ReadOperationVerifications(ctx, scope, operation.OperationID)
	if err != nil {
		return material, err
	}
	for _, task := range tasks {
		events, err := s.verifications.ReadVerificationEvents(ctx, scope, task.CaseID, task.VerificationNo)
		if err != nil {
			return material, err
		}
		material.Verifications = append(material.Verifications, domain.OrderStatusVerificationMaterial{
			Task:   task,
			Events: events,
		})
	}
	corrections, err := s.corrections.ReadOperationCorrections(ctx, scope, operation.OperationID)
	if err != nil {
		return material, err
	}
	for _, correction := range corrections {
		events, err := s.corrections.ReadCorrectionEvents(ctx, scope, correction.CaseID, correction.CorrectionNo)
		if err != nil {
			return material, err
		}
		material.Corrections = append(material.Corrections, domain.OrderStatusCorrectionMaterial{
			Correction: correction,
			Events:     events,
		})
	}
	material.Facts, err = s.orderFacts.ReadOperationSupplierFacts(ctx, scope, operation.OperationID)
	if err != nil {
		return material, err
	}
	material.Operation.OfferEstimate = s.orderOfferEstimate(ctx, scope, actor, operation.CaseID)
	return material, nil
}
