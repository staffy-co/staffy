package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// Service is the application surface of Story 2.1. IngestEvent is the
// machine ingress: the tenant scope is built from the verified connection
// (trusted metadata), never from the envelope claim; the admission ladder
// runs before the case material; one transaction admits. The reads are the
// operator surface (review/configuration/audit; the sealed payload is
// review/audit only). DrainOutbox is the idempotent delivery command.
type Service struct {
	repository      casesPort.Repository
	identity        identityAPI.MembershipService
	verifier        casesPort.SourceVerifier
	routeFacts      casesPort.RouteFactsSource
	connectionFacts casesPort.ConnectionFactsSource
	members         casesPort.MembersSource
	clock           casesAPI.Clock
	ids             casesAPI.IDGenerator
}

// New wires the cases service. The verifier is mandatory (the ingress
// honestly refuses without a source verification contour); the fact ports
// are wired with the self-returning builders and honestly refuse when
// missing (the ConnectionsFacts precedent — an "ok" perimeter is never
// assumed).
func New(repository casesPort.Repository, identity identityAPI.MembershipService, verifier casesPort.SourceVerifier, clock casesAPI.Clock, ids casesAPI.IDGenerator) *Service {
	return &Service{repository: repository, identity: identity, verifier: verifier, clock: clock, ids: ids}
}

func (s *Service) WithRouteFacts(source casesPort.RouteFactsSource) *Service {
	s.routeFacts = source
	return s
}

func (s *Service) WithConnectionFacts(source casesPort.ConnectionFactsSource) *Service {
	s.connectionFacts = source
	return s
}

func (s *Service) WithMembers(source casesPort.MembersSource) *Service {
	s.members = source
	return s
}

// IngestEvent runs the whole ingress pipeline: strict decode → source
// verification → the trusted scope → the idempotent replay lookup BEFORE
// the admission ladder → the ladder over the gathered facts → the single
// admission transaction, the honest refusal row or the quarantine.
func (s *Service) IngestEvent(ctx context.Context, command casesAPI.IngestCommand) (casesAPI.IngestOutcome, error) {
	now := command.Now.Truncate(time.Microsecond)
	envelope, err := casesDomain.ValidateEnvelope(command.Envelope, now)
	if err != nil {
		s.recordRejection(ctx, tenancy.Context{}, command, casesDomain.KindMalformed, err.Error(), 0, envelope.ConnectionID, false)
		return casesAPI.IngestOutcome{Refused: &casesAPI.IngestRefusal{Kind: casesDomain.KindMalformed, Message: err.Error()}}, nil
	}
	// The source verification is the only durable thing that may precede
	// the scope — and it is not durable: nothing is written before the
	// verified connection exists.
	verification, err := s.verifier.VerifySource(ctx, envelope.TenantClaim, envelope.ConnectionID)
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	if !verification.Verified {
		// The refusal is committed under the CLAIMED tenant scope: the
		// rejection row is the honest trace of the refused delivery; RLS
		// keeps it inside the claimed tenant (the connection was never
		// found in it).
		scope, _ := tenancy.New(envelope.ConnectionID, envelope.TenantClaim)
		s.recordRejection(ctx, scope, command, verification.Kind, verification.Text, 0, envelope.ConnectionID, false)
		return casesAPI.IngestOutcome{Refused: &casesAPI.IngestRefusal{Kind: verification.Kind, Message: verification.Text}}, nil
	}
	// The trusted scope: principal slot carries the verified connection —
	// the ingress scope lives only inside this command and never becomes a
	// user session.
	scope, err := tenancy.New(verification.Snapshot.ConnectionID, verification.Snapshot.TenantID)
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	// The replay lookup happens BEFORE the admission ladder: a sender
	// retry never depends on the changed admission states (entitlement
	// pause after acceptance does not break the retry).
	lookup, err := s.repository.LookupEvent(ctx, scope, envelope.Provider, envelope.ProviderEventID, envelope.PayloadDigest, command.IdempotencyKey)
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	if lookup.KeyConflict {
		// The idempotency key is already held by an accepted event of
		// another provider event id: the delivery is quarantined for the
		// human — never an internal error.
		event, exception, err := s.quarantine(ctx, scope, command, envelope, now,
			casesDomain.ExceptionIdempotencyConflict,
			fmt.Sprintf("Идемпотентный ключ %s уже использован принятым событием %s/%s; доставка карантинится для человека.", command.IdempotencyKey, lookup.KeyOwnerEvent.Provider, lookup.KeyOwnerEvent.ProviderEventID),
			lookup.KeyOwnerEvent.EventID, lookup.KeyOwnerEvent.CaseID, lookup.KeyOwnerEvent.ConversationID)
		if err != nil {
			return casesAPI.IngestOutcome{}, err
		}
		return casesAPI.IngestOutcome{Quarantined: &casesAPI.IngestQuarantine{
			Kind: casesDomain.ExceptionIdempotencyConflict, ExceptionID: exception.ExceptionID, EventID: event.EventID}}, nil
	}
	if lookup.Found {
		if lookup.DigestMatch {
			return casesAPI.IngestOutcome{Accepted: acceptedView(lookup.Result, true)}, nil
		}
		// The same provider event id with another material payload: the
		// delivery is quarantined, the accepted material is never
		// rewritten, one operational exception for the human.
		event, exception, err := s.quarantine(ctx, scope, command, envelope, now,
			casesDomain.ExceptionDigestConflict,
			fmt.Sprintf("Digest-конфликт события %s/%s: доставленный payload отличается от принятого; принятая запись не переписывается.", envelope.Provider, envelope.ProviderEventID),
			lookup.OriginalEvent.EventID, lookup.Result.CaseID, lookup.Result.ConversationID)
		if err != nil {
			return casesAPI.IngestOutcome{}, err
		}
		return casesAPI.IngestOutcome{Quarantined: &casesAPI.IngestQuarantine{
			Kind: casesDomain.ExceptionDigestConflict, ExceptionID: exception.ExceptionID, EventID: event.EventID}}, nil
	}
	// The admission facts: the connection state (trusted-scope read), the
	// route facts (employee facade + the relayed entitlement word), the
	// active membership set and the load counters.
	facts, err := s.ingressFacts(ctx, scope, envelope)
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	outcome := casesDomain.DeriveIngressOutcome(facts)
	switch outcome.Kind {
	case casesDomain.OutcomeAccept:
		return s.accept(ctx, scope, command, envelope, outcome, now)
	case casesDomain.OutcomeReject, casesDomain.OutcomeBounded:
		s.recordRejection(ctx, scope, command, outcome.RefusalKind, outcome.Text, outcome.Counter, envelope.ConnectionID, true)
		return casesAPI.IngestOutcome{Refused: &casesAPI.IngestRefusal{
			Kind: outcome.RefusalKind, Message: outcome.Text, Counter: outcome.Counter,
			Bounded: casesDomain.IsBoundedKind(outcome.RefusalKind)}}, nil
	default: // quarantine (ambiguous routing)
		event, exception, err := s.quarantine(ctx, scope, command, envelope, now,
			casesDomain.ExceptionAmbiguousRouting, outcome.Text, id.UUID(""), id.UUID(""), id.UUID(""))
		if err != nil {
			return casesAPI.IngestOutcome{}, err
		}
		return casesAPI.IngestOutcome{Quarantined: &casesAPI.IngestQuarantine{
			Kind: casesDomain.ExceptionAmbiguousRouting, ExceptionID: exception.ExceptionID, EventID: event.EventID}}, nil
	}
}

// ingressFacts gathers the ladder facts with the trusted scope: the
// connection state read, the employee route facts (per candidates with the
// relayed entitlement word), the active membership set and the in-flight
// counter. Each missing port is an honest error, never an assumed state.
func (s *Service) ingressFacts(ctx context.Context, scope tenancy.Context, envelope casesDomain.Envelope) (casesDomain.IngressFacts, error) {
	if s.connectionFacts == nil {
		return casesDomain.IngressFacts{}, fmt.Errorf("факт-порт подключений не сконфигурирован: ingress честно отклоняется")
	}
	if s.routeFacts == nil {
		return casesDomain.IngressFacts{}, fmt.Errorf("факт-порт маршрутизации не сконфигурирован: ingress честно отклоняется")
	}
	if s.members == nil {
		return casesDomain.IngressFacts{}, fmt.Errorf("факт-порт участников не сконфигурирован: ingress честно отклоняется")
	}
	connection, err := s.connectionFacts.ConnectionFactsByID(ctx, scope, envelope.ConnectionID)
	if err != nil {
		return casesDomain.IngressFacts{}, err
	}
	routes, err := s.routeFacts.RouteFacts(ctx, scope, envelope.ConnectionID)
	if err != nil {
		return casesDomain.IngressFacts{}, err
	}
	// The principal slot carries the verified connection — the membership
	// read is the caller-authority projection (RLS isolates the tenant).
	memberships, err := s.members.MembersFacts(ctx, domain.Principal{ID: scope.PrincipalID()}, scope.TenantID())
	if err != nil {
		return casesDomain.IngressFacts{}, err
	}
	active := map[id.UUID]bool{}
	for _, membership := range memberships {
		if membership.Status == "active" && membership.PrincipalID != nil {
			active[*membership.PrincipalID] = true
		}
	}
	inflight, err := s.repository.CountInflight(ctx, scope, envelope.ConnectionID, casesDomain.FixedLoadEnvelope.InflightWindow, s.clock.Now().UTC())
	if err != nil {
		return casesDomain.IngressFacts{}, err
	}
	facts := casesDomain.IngressFacts{
		Connection: casesDomain.ConnectionFacts{
			ConnectionID: connection.ConnectionID, Status: connection.Status,
			Environment: connection.Environment, ConnectorID: connection.ConnectorID,
			ConnectorType: connection.ConnectorType,
		},
		Channel:        envelope.Channel,
		Direction:      envelope.Direction,
		Modality:       envelope.Modality,
		Route:          routeWords(routes),
		ActiveOwnerIDs: active,
		InflightCount:  inflight,
		EnvelopeBytes:  envelope.EnvelopeBytes,
		PayloadBytes:   len(envelope.Payload),
	}
	return facts, nil
}

// routeWords adapts the employee facade projection into the domain words
// of the ladder.
func routeWords(routes employeeAPI.RouteFacts) casesDomain.RouteFacts {
	candidates := make([]casesDomain.RouteCandidate, 0, len(routes.Candidates))
	for _, candidate := range routes.Candidates {
		words := make([]casesDomain.RouteAdmissionWord, 0, len(candidate.Routes))
		for _, route := range candidate.Routes {
			words = append(words, casesDomain.RouteAdmissionWord{Channel: route.Route.Channel, Direction: route.Route.Direction, State: route.State})
		}
		coverage := make([]casesDomain.CoverageWord, 0, len(candidate.Coverage))
		for _, row := range candidate.Coverage {
			coverage = append(coverage, casesDomain.CoverageWord{Channel: row.Route.Channel, Direction: row.Route.Direction, ResponseOwnerID: row.ResponseOwnerID})
		}
		candidates = append(candidates, casesDomain.RouteCandidate{
			EmployeeID: candidate.EmployeeID, DecisionNo: candidate.DecisionNo, Stage: candidate.Stage,
			ProfileVersion: candidate.ProfileVersion, Routes: words, Coverage: coverage,
		})
	}
	return casesDomain.RouteFacts{Candidates: candidates, Entitlement: casesDomain.EntitlementWord{
		Status: routes.Entitlement.Status, Reason: routes.Entitlement.Reason}}
}

// accept performs the single admission transaction: the ids, the seal key,
// the canonical outbox payloads and the one repository call.
func (s *Service) accept(ctx context.Context, scope tenancy.Context, command casesAPI.IngestCommand, envelope casesDomain.Envelope, outcome casesDomain.Outcome, now time.Time) (casesAPI.IngestOutcome, error) {
	eventID, err := s.ids.New(now)
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	caseID, err := s.ids.New(now)
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	conversationID, err := s.ids.New(now)
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	workItemID, err := s.ids.New(now)
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	sessionID, err := s.ids.New(now)
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	outboxEventID, err := s.ids.New(now)
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	ackEventID, err := s.ids.New(now)
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	sealKey, err := casesDomain.NewSealKey()
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(now); err != nil {
			return casesAPI.IngestOutcome{}, err
		}
	}
	event := casesDomain.ChannelEvent{
		TenantID: scope.TenantID(), CaseID: caseID, ConversationID: conversationID, EventID: eventID,
		ConnectionID: envelope.ConnectionID, Provider: envelope.Provider,
		ProviderEventID: envelope.ProviderEventID, ProviderSessionID: envelope.ProviderSessionID,
		Direction: envelope.Direction, Modality: envelope.Modality, Channel: envelope.Channel,
		CorrelationID: correlation, ProviderTimestamp: envelope.ProviderTimestamp.Truncate(time.Microsecond),
		ReceivedAt: now, PayloadDigest: envelope.PayloadDigest,
		IdempotencyKey: command.IdempotencyKey, State: casesDomain.EventStateAccepted,
	}
	acceptedPayload := casesDomain.AcceptedEventPayload{
		TenantID: event.TenantID, EventID: event.EventID, ConnectionID: event.ConnectionID,
		Provider: event.Provider, ProviderEventID: event.ProviderEventID, ProviderSessionID: event.ProviderSessionID,
		Channel: event.Channel, Modality: event.Modality, Direction: event.Direction,
		PayloadDigest: event.PayloadDigest,
	}
	ackPayload, err := casesDomain.ConnectorAckPayloadJSON(casesDomain.ConnectorAckPayload{
		TenantID: scope.TenantID(), EventID: eventID, ConnectionID: envelope.ConnectionID,
		Provider: envelope.Provider, ProviderEventID: envelope.ProviderEventID,
		AckContract: casesDomain.AckContractSynthetic, Deferred: true,
	})
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	result, err := s.repository.AcceptEvent(ctx, casesPort.AcceptParams{
		Scope: scope, Event: event, Payload: envelope.Payload, SealKey: sealKey,
		Seal:       casesDomain.PayloadSeal(sealKey, envelope.Payload),
		EmployeeID: outcome.EmployeeID, ResponseOwnerID: outcome.OwnerID,
		Session: casesDomain.PreCaseSession{
			TenantID: scope.TenantID(), SessionID: sessionID, ConnectionID: envelope.ConnectionID,
			Provider: envelope.Provider, ProviderSessionID: envelope.ProviderSessionID,
			Modality: envelope.Modality, CapturePolicy: casesDomain.CapturePolicyOfModality(envelope.Modality),
		},
		CaseID: caseID, ConversationID: conversationID, WorkItemID: workItemID, SessionID: sessionID,
		OutboxEventID: outboxEventID, AckEventID: ackEventID,
		AcceptedPayload: acceptedPayload, AckPayload: ackPayload, Now: now,
	})
	if err != nil {
		// The raced conflicts resolve honestly INLINE — never by a
		// recursive re-run: one fresh lookup decides deterministically
		// (the advisory locks guarantee the conflicting state is already
		// committed), and an unresolvable conflict stays a genuine
		// internal error instead of a stack overflow.
		if errors.Is(err, casesDomain.ErrDigestConflict) || errors.Is(err, casesDomain.ErrIdempotencyKeyConflict) || errors.Is(err, casesDomain.ErrEventRaced) {
			return s.resolveRacedConflict(ctx, scope, command, envelope, now)
		}
		return casesAPI.IngestOutcome{}, err
	}
	return casesAPI.IngestOutcome{Accepted: acceptedView(result, result.Replay)}, nil
}

// resolveRacedConflict turns a raced write conflict into the honest
// outcome with ONE fresh lookup: a material match replays, a digest
// mismatch quarantines, a key conflict quarantines; anything else is a
// genuine internal error (never a loop — no recursion exists).
func (s *Service) resolveRacedConflict(ctx context.Context, scope tenancy.Context, command casesAPI.IngestCommand, envelope casesDomain.Envelope, now time.Time) (casesAPI.IngestOutcome, error) {
	lookup, err := s.repository.LookupEvent(ctx, scope, envelope.Provider, envelope.ProviderEventID, envelope.PayloadDigest, command.IdempotencyKey)
	if err != nil {
		return casesAPI.IngestOutcome{}, err
	}
	if lookup.Found {
		if lookup.DigestMatch {
			return casesAPI.IngestOutcome{Accepted: acceptedView(lookup.Result, true)}, nil
		}
		event, exception, err := s.quarantine(ctx, scope, command, envelope, now,
			casesDomain.ExceptionDigestConflict,
			fmt.Sprintf("Digest-конфликт события %s/%s: доставленный payload отличается от принятого; принятая запись не переписывается.", envelope.Provider, envelope.ProviderEventID),
			lookup.OriginalEvent.EventID, lookup.Result.CaseID, lookup.Result.ConversationID)
		if err != nil {
			return casesAPI.IngestOutcome{}, err
		}
		// The quarantined row's id — the same semantics as the pre-ladder
		// branches of IngestEvent (the machine contract stays consistent
		// whether the conflict was detected before or after the write).
		return casesAPI.IngestOutcome{Quarantined: &casesAPI.IngestQuarantine{
			Kind: casesDomain.ExceptionDigestConflict, ExceptionID: exception.ExceptionID, EventID: event.EventID}}, nil
	}
	if lookup.KeyConflict {
		event, exception, err := s.quarantine(ctx, scope, command, envelope, now,
			casesDomain.ExceptionIdempotencyConflict,
			fmt.Sprintf("Идемпотентный ключ %s уже использован принятым событием %s/%s; доставка карантинится для человека.", command.IdempotencyKey, lookup.KeyOwnerEvent.Provider, lookup.KeyOwnerEvent.ProviderEventID),
			lookup.KeyOwnerEvent.EventID, lookup.KeyOwnerEvent.CaseID, lookup.KeyOwnerEvent.ConversationID)
		if err != nil {
			return casesAPI.IngestOutcome{}, err
		}
		return casesAPI.IngestOutcome{Quarantined: &casesAPI.IngestQuarantine{
			Kind: casesDomain.ExceptionIdempotencyConflict, ExceptionID: exception.ExceptionID, EventID: event.EventID}}, nil
	}
	return casesAPI.IngestOutcome{}, casesDomain.ErrEventRaced
}

// quarantine records the quarantined delivery with its operational
// exception; the accepted material is never rewritten.
func (s *Service) quarantine(ctx context.Context, scope tenancy.Context, command casesAPI.IngestCommand, envelope casesDomain.Envelope, now time.Time, kind, text string, originalEventID, caseID, conversationID id.UUID) (casesDomain.ChannelEvent, casesDomain.OperationalException, error) {
	eventID, err := s.ids.New(now)
	if err != nil {
		return casesDomain.ChannelEvent{}, casesDomain.OperationalException{}, err
	}
	exceptionID, err := s.ids.New(now)
	if err != nil {
		return casesDomain.ChannelEvent{}, casesDomain.OperationalException{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(now); err != nil {
			return casesDomain.ChannelEvent{}, casesDomain.OperationalException{}, err
		}
	}
	event := casesDomain.ChannelEvent{
		TenantID: scope.TenantID(), EventID: eventID, ConnectionID: envelope.ConnectionID,
		Provider: envelope.Provider, ProviderEventID: envelope.ProviderEventID,
		ProviderSessionID: envelope.ProviderSessionID, Direction: envelope.Direction,
		Modality: envelope.Modality, Channel: envelope.Channel, CorrelationID: correlation,
		ProviderTimestamp: envelope.ProviderTimestamp.Truncate(time.Microsecond), ReceivedAt: now,
		PayloadDigest: envelope.PayloadDigest, IdempotencyKey: command.IdempotencyKey,
		State: casesDomain.EventStateQuarantined,
	}
	return s.repository.QuarantineEvent(ctx, casesPort.QuarantineParams{
		Scope: scope, Event: event, ExceptionKind: kind, ExceptionText: text, ExceptionID: exceptionID,
		OriginalEventID: originalEventID, CaseID: caseID, ConversationID: conversationID, Now: now,
	})
}

// recordRejection commits one honest refusal row (its own transaction, no
// payload by construction). A storage failure never masks the refusal
// outcome: the rejection row is the trace, not the gate.
//
// TRUSTED-scope refusals (the admission ladder after the verified source)
// are written plainly. UNTRUSTED-scope refusals (the source verification
// of an unverifiable delivery — the endpoint is a machine contract without
// a principal) are written only when the CLAIMED tenant is real (the
// membership facts of the claim are visible under the RLS-scoped read)
// and the fixed dictionary bound of recorded refusals per (claimed
// tenant, kind) inside the window is not reached: the unauthenticated
// surface may never write unboundedly into someone's tenant — a
// frozen-clause tension documented in deferred-work.
func (s *Service) recordRejection(ctx context.Context, scope tenancy.Context, command casesAPI.IngestCommand, kind, text string, counter int, connectionID id.UUID, trusted bool) {
	if s.repository == nil {
		return
	}
	rejectionID, err := s.ids.New(s.clock.Now().UTC())
	if err != nil {
		return
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(s.clock.Now().UTC()); err != nil {
			return
		}
	}
	if !scope.Valid() {
		// A malformed envelope may carry no honest tenant claim: the
		// refusal is still returned to the sender, but there is no scope
		// to store the trace under — silently inventing one is worse than
		// the missing row.
		if claim := command.Envelope.TenantClaim; claim.Valid() && connectionID.Valid() {
			scope, _ = tenancy.New(connectionID, claim)
		} else {
			return
		}
	}
	if !trusted {
		if !s.claimedTenantIsReal(ctx, scope, connectionID) {
			return
		}
		count, err := s.repository.CountRejections(ctx, scope, kind, casesDomain.RejectionBoundWindow, s.clock.Now().UTC())
		if err != nil || count >= casesDomain.MaxClaimedRejectionsPerKey {
			return
		}
	}
	_, _ = s.repository.RecordRejection(ctx, casesPort.RejectionParams{
		Scope: scope, RejectionID: rejectionID, Kind: kind, Message: text,
		CorrelationID: correlation, ConnectionID: connectionID, Counter: counter, Now: s.clock.Now().UTC(),
	})
}

// claimedTenantIsReal resolves the membership facts under the claimed
// scope: the RLS tenant policy keeps the read inside the claim, and an
// empty answer means the claimed tenant does not exist (or holds no
// member) — no durable trace is written for it.
func (s *Service) claimedTenantIsReal(ctx context.Context, scope tenancy.Context, connectionID id.UUID) bool {
	if s.members == nil || !scope.Valid() {
		return false
	}
	memberships, err := s.members.MembersFacts(ctx, domain.Principal{ID: connectionID}, scope.TenantID())
	if err != nil || len(memberships) == 0 {
		return false
	}
	return true
}

func acceptedView(result casesDomain.IngestResult, replay bool) *casesAPI.IngestResult {
	return &casesAPI.IngestResult{
		EventID: result.EventID, CaseID: result.CaseID, ConversationID: result.ConversationID,
		CaseNo: result.CaseNo, ConversationNo: result.ConversationNo,
		ResponseOwnerID: result.ResponseOwnerID, EmployeeID: result.EmployeeID,
		NewCase: result.NewCase, Replay: replay,
	}
}

// ---------------------------------------------------------------------------
// Operator surface reads and the drain command
// ---------------------------------------------------------------------------

// viewScope authorizes the cases reads: review, configuration or audit see
// the surface; other members get an honest refusal; non-members a plain
// 404 (the tenant existence is never disclosed). The payload flag marks
// the stricter review/audit boundary.
func (s *Service) viewScope(ctx context.Context, actor domain.Principal, tenantID id.UUID) (tenancy.Context, bool, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, false, domain.ErrNotFound
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return tenancy.Context{}, false, err
	}
	payloadAllowed := false
	memberWithoutScope := false
	var infraFailure error
	for _, required := range []domain.Scope{domain.Review, domain.Configuration, domain.Audit} {
		err := s.identity.Authorize(ctx, actor, tenantID, required)
		if err == nil {
			if required != domain.Configuration {
				payloadAllowed = true
			}
			return scope, payloadAllowed, nil
		}
		if errors.Is(err, domain.ErrForbidden) {
			memberWithoutScope = true
			continue
		}
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		infraFailure = err
	}
	if infraFailure != nil {
		return tenancy.Context{}, false, infraFailure
	}
	if memberWithoutScope {
		return tenancy.Context{}, false, domain.ErrForbidden
	}
	return tenancy.Context{}, false, domain.ErrNotFound
}

// manageScope authorizes the drain command: the configuration scope.
func (s *Service) manageScope(ctx context.Context, actor domain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return tenancy.Context{}, err
	}
	if err := s.identity.Authorize(ctx, actor, tenantID, domain.Configuration); err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			return tenancy.Context{}, domain.ErrForbidden
		}
		if errors.Is(err, domain.ErrNotFound) {
			return tenancy.Context{}, domain.ErrNotFound
		}
		return tenancy.Context{}, err
	}
	return scope, nil
}

// CasesView lists the case material and the open operational exceptions.
func (s *Service) CasesView(ctx context.Context, actor domain.Principal, tenantID id.UUID) (casesAPI.CasesView, error) {
	scope, _, err := s.viewScope(ctx, actor, tenantID)
	if err != nil {
		return casesAPI.CasesView{}, err
	}
	summaries, err := s.repository.ListCases(ctx, scope)
	if err != nil {
		return casesAPI.CasesView{}, err
	}
	exceptions, err := s.repository.ListExceptions(ctx, scope)
	if err != nil {
		return casesAPI.CasesView{}, err
	}
	return casesAPI.CasesView{TenantID: tenantID, Cases: summaries, Exceptions: exceptions, CanView: true, ViewScope: casesAPI.CaseScopeView}, nil
}

// CaseView reads one case: the conversations, the timeline, the work items
// and the related exceptions. The two honest statements of the slice are
// part of the view contract.
func (s *Service) CaseView(ctx context.Context, actor domain.Principal, tenantID, caseID id.UUID) (casesAPI.CaseView, error) {
	scope, payloadAllowed, err := s.viewScope(ctx, actor, tenantID)
	if err != nil {
		return casesAPI.CaseView{}, err
	}
	detail, err := s.repository.ReadCase(ctx, scope, caseID)
	if err != nil {
		return casesAPI.CaseView{}, err
	}
	return casesAPI.CaseView{
		TenantID: tenantID, Case: detail.Case, Conversations: detail.Conversations,
		Timeline: detail.Timeline, WorkItems: detail.WorkItems, Exceptions: detail.Exceptions,
		UntriagedText:   "Запрос ещё не классифицирован.",
		NoAutoReplyText: "Автоответ клиенту не отправляется.",
		CanViewPayload:  payloadAllowed, ViewScope: casesAPI.CaseScopeView,
	}, nil
}

// ReadPayload reads the sealed payload of one event under the strict
// review/audit boundary: the digest and the HMAC seal are verified at the
// read boundary.
func (s *Service) ReadPayload(ctx context.Context, actor domain.Principal, tenantID, caseID, eventID id.UUID) (casesAPI.PayloadView, error) {
	scope, payloadAllowed, err := s.viewScope(ctx, actor, tenantID)
	if err != nil {
		return casesAPI.PayloadView{}, err
	}
	if !payloadAllowed {
		return casesAPI.PayloadView{}, domain.ErrForbidden
	}
	sealed, err := s.repository.ReadPayload(ctx, scope, caseID, eventID)
	if err != nil {
		return casesAPI.PayloadView{}, err
	}
	return casesAPI.PayloadView{EventID: sealed.EventID, PayloadDigest: sealed.Digest, SealOK: sealed.SealOK, Payload: sealed.Payload}, nil
}

// DrainOutbox idempotently drives the delivery outcomes of the pending
// outbox records: append-only outcome rows; a repeated run finds every
// record delivered and writes nothing (nothing is lost, nothing doubles).
// The delivery itself is honestly deferred — there are no background
// workers and no provider adapters in this slice (AG-4).
func (s *Service) DrainOutbox(ctx context.Context, actor domain.Principal, tenantID id.UUID) (casesAPI.DrainResult, error) {
	scope, err := s.manageScope(ctx, actor, tenantID)
	if err != nil {
		return casesAPI.DrainResult{}, err
	}
	pending, err := s.repository.PendingOutbox(ctx, scope, 500)
	if err != nil {
		return casesAPI.DrainResult{}, err
	}
	result := casesAPI.DrainResult{Considered: len(pending)}
	for _, event := range pending {
		deliveryID, err := s.ids.New(s.clock.Now().UTC())
		if err != nil {
			return casesAPI.DrainResult{}, err
		}
		if err := s.repository.RecordDelivery(ctx, scope, casesDomain.OutboxDelivery{
			TenantID: scope.TenantID(), DeliveryID: deliveryID, EventID: event.EventID,
			Attempt: 1, Outcome: casesDomain.DeliveryOutcomeDeferred, Details: casesDomain.DeliveryDeferReason,
			CreatedAt: s.clock.Now().UTC(),
		}); err != nil {
			return casesAPI.DrainResult{}, err
		}
		result.Deferred++
	}
	return result, nil
}
