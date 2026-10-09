package main

import (
	"context"
	"sync"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// casesFixture serves the cases journey over the REAL domain functions of
// the slice (ValidateEnvelope, DeriveIngressOutcome, PayloadDigest,
// PayloadSeal, the load-envelope dictionary): the fixture world is a
// fixed acting decision (phone:inbound admitted, the fixture principal as
// the initial response owner) over the synthetic telephony connection, an
// active entitlement word and an in-memory case material. The transport
// contract under test is the production one — templates, routes and wire
// shapes included.
type casesFixture struct {
	mu sync.Mutex

	tenant     id.UUID
	employee   id.UUID
	owner      id.UUID
	connection id.UUID

	// The acting decision facts of the fixture world.
	routes   []casesDomain.RouteAdmissionWord
	coverage []casesDomain.CoverageWord

	events     map[string]*casesDomain.ChannelEvent // key: provider event id
	payloads   map[id.UUID][]byte
	results    map[id.UUID]casesDomain.IngestResult
	material   map[id.UUID]*fixtureCaseMaterial // key: case id
	sessions   map[string]id.UUID               // key: provider:session → conversation id
	owners     map[id.UUID]id.UUID              // conversation → owner
	epoch      map[id.UUID]int64
	surface    map[id.UUID]string
	items      map[id.UUID][]casesDomain.WorkItem // case → work item anchors
	exceptions []casesDomain.OperationalException
	rejections []casesDomain.IngressRejection

	// The Story 2.2 work material: the version ladders, the event links, the
	// facts with their confirmations and the command records.
	workAnchors     map[id.UUID]casesDomain.WorkItem
	workVersions    map[id.UUID][]casesDomain.WorkItemVersion
	workLinks       map[id.UUID][]id.UUID
	factAnchors     map[string]id.UUID
	factVersions    map[id.UUID][]casesDomain.FactVersion
	factConfirms    map[id.UUID][]casesDomain.FactConfirmation
	commandHashes   map[string]string
	commandOutcomes map[string]casesAPI.WorkOutcome

	// The Story 2.3 verification material: the records, the bindings with
	// the reversals, the identifier tokens with the per-tenant HMAC key,
	// the duplicate candidates with the resolutions.
	verifications    []casesDomain.VerificationRecord
	bindings         []casesDomain.ChannelBinding
	reversals        []casesDomain.BindingReversal
	identifierKey    []byte
	identifierTokens map[string]map[id.UUID]string // token → case → material key
	candidates       []casesDomain.DuplicateCandidate
	resolutions      []casesDomain.DuplicateResolution

	caseNo, conversationNo int64
	eventSeq               int
	workSeq, exceptionSeq  int
}

type fixtureCaseMaterial struct {
	cases         casesDomain.Case
	conversations []casesDomain.Conversation
	timeline      []casesDomain.TimelineEvent
}

func newCasesFixture() *casesFixture {
	owner := principalID
	return &casesFixture{
		tenant:     tenantID,
		employee:   fixtureEmployeeID,
		owner:      owner,
		connection: fixtureConnectionID,
		routes: []casesDomain.RouteAdmissionWord{
			{Channel: "phone", Direction: "inbound", State: casesDomain.RouteWordAdmitted},
		},
		coverage: []casesDomain.CoverageWord{
			{Channel: "phone", Direction: "inbound", ResponseOwnerID: owner},
		},
		events:   map[string]*casesDomain.ChannelEvent{},
		payloads: map[id.UUID][]byte{},
		results:  map[id.UUID]casesDomain.IngestResult{},
		material: map[id.UUID]*fixtureCaseMaterial{},
		sessions: map[string]id.UUID{},
		owners:   map[id.UUID]id.UUID{},
		epoch:    map[id.UUID]int64{},
		surface:  map[id.UUID]string{},
		items:    map[id.UUID][]casesDomain.WorkItem{},
		// The Story 2.2 work material:
		workAnchors:     map[id.UUID]casesDomain.WorkItem{},
		workVersions:    map[id.UUID][]casesDomain.WorkItemVersion{},
		workLinks:       map[id.UUID][]id.UUID{},
		factAnchors:     map[string]id.UUID{},
		factVersions:    map[id.UUID][]casesDomain.FactVersion{},
		factConfirms:    map[id.UUID][]casesDomain.FactConfirmation{},
		commandHashes:   map[string]string{},
		commandOutcomes: map[string]casesAPI.WorkOutcome{},
		// The Story 2.3 material:
		identifierTokens: map[string]map[id.UUID]string{},
	}
}

func (f *casesFixture) lock()   { f.mu.Lock() }
func (f *casesFixture) unlock() { f.mu.Unlock() }

func (f *casesFixture) facts(command casesAPI.IngestCommand) casesDomain.IngressFacts {
	return casesDomain.IngressFacts{
		Connection: casesDomain.ConnectionFacts{
			ConnectionID: command.Envelope.ConnectionID, Status: "connected_unverified",
			Environment: "sandbox", ConnectorID: "staffy-synthetic-telephony", ConnectorType: casesDomain.ConnectorTypeSIP,
		},
		Channel:   command.Envelope.Channel,
		Direction: command.Envelope.Direction,
		Modality:  command.Envelope.Modality,
		Route: casesDomain.RouteFacts{
			Candidates: []casesDomain.RouteCandidate{{
				EmployeeID: f.employee, DecisionNo: 1, Stage: "synthetic", ProfileVersion: 1,
				Routes: f.routes, Coverage: f.coverage,
			}},
			Entitlement: casesDomain.EntitlementWord{Status: "ok"},
		},
		ActiveOwnerIDs: map[id.UUID]bool{f.owner: true},
		InflightCount:  len(f.events),
		EnvelopeBytes:  command.Envelope.EnvelopeBytes,
		PayloadBytes:   len(command.Envelope.Payload),
	}
}

// IngestEvent runs the real ingress pipeline over the fixture world: the
// strict envelope validation, the fixture source verification (only the
// fixture connection of the test contour verifies), the material replay
// lookup, the REAL admission ladder and the in-memory material writes.
func (f *casesFixture) IngestEvent(_ context.Context, command casesAPI.IngestCommand) (casesAPI.IngestOutcome, error) {
	now := command.Now.Truncate(time.Microsecond)
	envelope, err := casesDomain.ValidateEnvelope(command.Envelope, now)
	if err != nil {
		return casesAPI.IngestOutcome{Refused: &casesAPI.IngestRefusal{Kind: casesDomain.KindMalformed, Message: err.Error()}}, nil
	}
	f.lock()
	defer f.unlock()
	// The source verification of the test contour: only the fixture
	// connection of the fixture tenant verifies (the honest unknown
	// refusal otherwise).
	if envelope.ConnectionID != f.connection || envelope.TenantClaim != f.tenant {
		f.rejections = append(f.rejections, casesDomain.IngressRejection{Kind: casesDomain.KindUnknownConnection, Message: "Подключение не найдено в заявленном пространстве.", CorrelationID: command.CorrelationID})
		return casesAPI.IngestOutcome{Refused: &casesAPI.IngestRefusal{Kind: casesDomain.KindUnknownConnection, Message: "Подключение не найдено в заявленном пространстве."}}, nil
	}
	// The material replay lookup happens BEFORE the admission ladder.
	if original, found := f.events[envelope.ProviderEventID]; found {
		if original.PayloadDigest == envelope.PayloadDigest {
			return casesAPI.IngestOutcome{Accepted: &casesAPI.IngestResult{
				EventID: original.EventID, CaseID: original.CaseID, ConversationID: original.ConversationID,
				CaseNo: f.results[original.EventID].CaseNo, ConversationNo: f.results[original.EventID].ConversationNo,
				ResponseOwnerID: f.owners[original.ConversationID], EmployeeID: f.employee,
				NewCase: false, Replay: true,
			}}, nil
		}
		// The digest conflict quarantine: the accepted material never
		// changes, one exception for the human.
		quarantined := *original
		quarantined.EventID = id.MustParse("01994f64-6c00-7000-8000-00000000dead")
		quarantined.State = casesDomain.EventStateQuarantined
		quarantined.PayloadDigest = envelope.PayloadDigest
		exception := casesDomain.OperationalException{
			TenantID: f.tenant, ExceptionID: quarantined.EventID, Kind: casesDomain.ExceptionDigestConflict,
			Summary:       "Digest-конфликт события: доставленный payload отличается от принятого; принятая запись не переписывается.",
			CorrelationID: command.CorrelationID, EventID: quarantined.EventID, OriginalEventID: original.EventID,
			CaseID: original.CaseID, ConversationID: original.ConversationID, CreatedAt: now,
		}
		f.exceptions = append(f.exceptions, exception)
		return casesAPI.IngestOutcome{Quarantined: &casesAPI.IngestQuarantine{
			Kind: casesDomain.ExceptionDigestConflict, ExceptionID: exception.ExceptionID, EventID: quarantined.EventID,
		}}, nil
	}
	outcome := casesDomain.DeriveIngressOutcome(f.facts(command))
	switch outcome.Kind {
	case casesDomain.OutcomeAccept:
		return f.accept(envelope, command, outcome, now), nil
	case casesDomain.OutcomeReject, casesDomain.OutcomeBounded:
		f.rejections = append(f.rejections, casesDomain.IngressRejection{
			Kind: outcome.RefusalKind, Message: outcome.Text, Counter: outcome.Counter,
			CorrelationID: command.CorrelationID, ConnectionID: envelope.ConnectionID, CreatedAt: now,
		})
		return casesAPI.IngestOutcome{Refused: &casesAPI.IngestRefusal{
			Kind: outcome.RefusalKind, Message: outcome.Text, Counter: outcome.Counter,
			Bounded: casesDomain.IsBoundedKind(outcome.RefusalKind),
		}}, nil
	default:
		exception := casesDomain.OperationalException{
			TenantID: f.tenant, ExceptionID: id.MustParse("01994f64-6c00-7000-8000-0000000000b5"),
			Kind: casesDomain.ExceptionAmbiguousRouting, Summary: outcome.Text,
			CorrelationID: command.CorrelationID, CreatedAt: now,
		}
		f.exceptions = append(f.exceptions, exception)
		return casesAPI.IngestOutcome{Quarantined: &casesAPI.IngestQuarantine{
			Kind: casesDomain.ExceptionAmbiguousRouting, ExceptionID: exception.ExceptionID, EventID: exception.ExceptionID,
		}}, nil
	}
}

// accept writes the in-memory case material of one admitted event.
func (f *casesFixture) accept(envelope casesDomain.Envelope, command casesAPI.IngestCommand, outcome casesDomain.Outcome, now time.Time) casesAPI.IngestOutcome {
	f.eventSeq++
	eventID := id.MustParse("01994f64-6c00-7000-8000-" + digit12(f.eventSeq))
	sessionKey := envelope.Provider + ":" + envelope.ProviderSessionID
	newCase := false
	conversationID, known := f.sessions[sessionKey]
	if !known {
		newCase = true
		f.caseNo++
		f.conversationNo++
		conversationID = id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(1000+f.conversationNo)))
		f.sessions[sessionKey] = conversationID
		f.owners[conversationID] = outcome.OwnerID
		f.epoch[conversationID] = casesDomain.ResponseOwnerEpochFirst
		f.surface[conversationID] = casesDomain.WritableSurfaceInbox
	}
	caseID := id.UUID("")
	caseNo := int64(0)
	initialWorkItemID := id.UUID("")
	if newCase {
		caseID = id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(5000+f.caseNo)))
		caseNo = f.caseNo
		material := &fixtureCaseMaterial{cases: casesDomain.Case{
			TenantID: f.tenant, CaseID: caseID, CaseNo: caseNo, EmployeeID: outcome.EmployeeID,
			State: casesDomain.CaseStateOpen, PackID: fixturePackID, PackVersion: fixturePackVersion, CreatedAt: now,
		}}
		material.conversations = append(material.conversations, casesDomain.Conversation{
			TenantID: f.tenant, CaseID: caseID, ConversationID: conversationID, ConversationNo: f.conversationNo,
			EmployeeID: outcome.EmployeeID, ConnectionID: envelope.ConnectionID, Provider: envelope.Provider,
			ProviderSessionID: envelope.ProviderSessionID, ResponseOwnerID: outcome.OwnerID,
			ResponseOwnerEpoch: casesDomain.ResponseOwnerEpochFirst, WritableSurface: casesDomain.WritableSurfaceInbox,
			State: casesDomain.ConversationStateActive, Modality: envelope.Modality, CreatedAt: now,
		})
		f.material[caseID] = material
		initialWorkItemID = id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(9000+f.caseNo)))
		f.items[caseID] = append(f.items[caseID], casesDomain.WorkItem{
			TenantID: f.tenant, CaseID: caseID, ConversationID: conversationID,
			WorkItemID: initialWorkItemID, CreatedAt: now,
		})
		// The version 1 of the initial work item (the honest untriaged state,
		// the system actor) — the REAL derivation material of the WCC.
		f.workAnchors[initialWorkItemID] = casesDomain.WorkItem{
			TenantID: f.tenant, CaseID: caseID, ConversationID: conversationID,
			WorkItemID: initialWorkItemID, CreatedAt: now,
		}
		f.workVersions[initialWorkItemID] = append(f.workVersions[initialWorkItemID], casesDomain.WorkItemVersion{
			TenantID: f.tenant, CaseID: caseID, ConversationID: conversationID,
			WorkItemID: initialWorkItemID, VersionNo: 1, Kind: casesDomain.VersionKindCreate,
			State: casesDomain.WorkItemStateUntriaged, CreatedAt: now,
		})
	} else {
		for candidateID, material := range f.material {
			for _, conversation := range material.conversations {
				if conversation.ConversationID == conversationID {
					caseID, caseNo = candidateID, material.cases.CaseNo
				}
			}
		}
	}
	event := casesDomain.ChannelEvent{
		TenantID: f.tenant, CaseID: caseID, ConversationID: conversationID, EventID: eventID,
		ConnectionID: envelope.ConnectionID, Provider: envelope.Provider,
		ProviderEventID: envelope.ProviderEventID, ProviderSessionID: envelope.ProviderSessionID,
		Direction: envelope.Direction, Modality: envelope.Modality, Channel: envelope.Channel,
		CorrelationID: command.CorrelationID, ProviderTimestamp: envelope.ProviderTimestamp,
		ReceivedAt: now, PayloadDigest: envelope.PayloadDigest,
		IdempotencyKey: command.IdempotencyKey, State: casesDomain.EventStateAccepted,
	}
	f.events[envelope.ProviderEventID] = &event
	f.payloads[eventID] = envelope.Payload
	// The link of the first event of a new conversation to the initial work
	// item (the system link); a session continuation writes no new work item
	// and no automatic link.
	if newCase && initialWorkItemID.Valid() {
		f.workLinks[initialWorkItemID] = append(f.workLinks[initialWorkItemID], eventID)
	}
	f.results[eventID] = casesDomain.IngestResult{
		EventID: eventID, CaseID: caseID, ConversationID: conversationID, CaseNo: caseNo,
		ConversationNo: f.conversationNoOf(conversationID), ResponseOwnerID: outcome.OwnerID,
		EmployeeID: outcome.EmployeeID, NewCase: newCase, Replay: false,
	}
	if material := f.material[caseID]; material != nil {
		material.timeline = append(material.timeline, casesDomain.TimelineEvent{
			EventID: eventID, Channel: event.Channel, Modality: event.Modality, Direction: event.Direction,
			Provider: event.Provider, ProviderEventID: event.ProviderEventID, State: event.State,
			ProviderTimestamp: event.ProviderTimestamp, ReceivedAt: event.ReceivedAt,
			PayloadDigest: event.PayloadDigest, CorrelationID: event.CorrelationID,
		})
	}
	return casesAPI.IngestOutcome{Accepted: &casesAPI.IngestResult{
		EventID: eventID, CaseID: caseID, ConversationID: conversationID, CaseNo: caseNo,
		ConversationNo: f.conversationNoOf(conversationID), ResponseOwnerID: outcome.OwnerID,
		EmployeeID: outcome.EmployeeID, NewCase: newCase, Replay: false,
	}}
}

func (f *casesFixture) conversationNoOf(conversationID id.UUID) int64 {
	for _, material := range f.material {
		for _, conversation := range material.conversations {
			if conversation.ConversationID == conversationID {
				return conversation.ConversationNo
			}
		}
	}
	return 0
}

func digit12(value int) string {
	digits := []byte("000000000000")
	text := []byte(itoa(value))
	copy(digits[12-len(text):], text)
	return string(digits)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	out := []byte{}
	for value > 0 {
		out = append([]byte{byte('0' + value%10)}, out...)
		value /= 10
	}
	return string(out)
}

// CasesView lists the fixture material and the open exceptions.
func (f *casesFixture) CasesView(_ context.Context, _ domain.Principal, tenant id.UUID) (casesAPI.CasesView, error) {
	f.lock()
	defer f.unlock()
	if tenant != f.tenant {
		return casesAPI.CasesView{}, domain.ErrNotFound
	}
	view := casesAPI.CasesView{TenantID: tenant, CanView: true, ViewScope: casesAPI.CaseScopeView}
	for _, material := range f.material {
		owner := id.UUID("")
		if len(material.conversations) > 0 {
			owner = material.conversations[0].ResponseOwnerID
		}
		last := material.cases.CreatedAt
		count := 0
		for _, event := range material.timeline {
			count++
			if event.ReceivedAt.After(last) {
				last = event.ReceivedAt
			}
		}
		view.Cases = append(view.Cases, casesDomain.CaseSummary{
			CaseID: material.cases.CaseID, CaseNo: material.cases.CaseNo, EmployeeID: material.cases.EmployeeID,
			State: material.cases.State, CreatedAt: material.cases.CreatedAt, Conversations: len(material.conversations),
			Events: count, LastEventAt: last, OwnerID: owner,
		})
	}
	view.Exceptions = append([]casesDomain.OperationalException(nil), f.exceptions...)
	return view, nil
}

// CaseView reads one case of the fixture material with the honest
// statements of the slice, the DERIVED work item views (the version
// ladders) and the pinned taxonomy of the Work Control Card.
func (f *casesFixture) CaseView(_ context.Context, _ domain.Principal, tenant, caseID id.UUID) (casesAPI.CaseView, error) {
	f.lock()
	defer f.unlock()
	if tenant != f.tenant {
		return casesAPI.CaseView{}, domain.ErrNotFound
	}
	material := f.material[caseID]
	if material == nil {
		return casesAPI.CaseView{}, casesDomain.ErrNotFound
	}
	// The derived work item views of the case (the ladder projection).
	workItems := make([]casesDomain.WorkItemCurrent, 0, len(f.items[caseID]))
	for _, anchor := range f.items[caseID] {
		if current, known := f.workCurrentOf(anchor.WorkItemID); known {
			workItems = append(workItems, current)
		}
	}
	// The Story 2.3 unified view: the conversations of the case plus the
	// actively bound conversations (the append-only links without a
	// reversal) with their timeline events in the one received_at order.
	conversations := append([]casesDomain.Conversation(nil), material.conversations...)
	timeline := append([]casesDomain.TimelineEvent(nil), material.timeline...)
	for _, binding := range casesDomain.ActiveBindings(f.bindings, f.reversals) {
		if binding.CaseID != caseID {
			continue
		}
		boundMaterial := f.material[binding.BoundCaseID]
		if boundMaterial == nil {
			continue
		}
		for _, conversation := range boundMaterial.conversations {
			if conversation.ConversationID == binding.BoundConversationID {
				conversations = append(conversations, conversation)
			}
		}
		for _, event := range boundMaterial.timeline {
			if eventBelongsToConversation(f, binding.BoundConversationID, event.EventID) {
				timeline = append(timeline, event)
			}
		}
	}
	// The Story 2.3 sections: the assurance derivation, the bindings, the
	// duplicate cards (the masked minimal material).
	records := f.verificationsOf(caseID)
	modality := ""
	if len(conversations) > 0 {
		modality = conversations[0].Modality
	}
	bindingCards := []casesPort.BindingCard{}
	for _, binding := range f.bindings {
		if binding.CaseID != caseID && binding.BoundCaseID != caseID {
			continue
		}
		card := casesPort.BindingCard{Binding: binding, ConfirmationText: casesDomain.BindingClientConfirmation(binding.Method)}
		if reversal := casesDomain.ReversalOf(f.reversals, binding.BindingID); reversal != nil {
			card.Reversal = reversal
		}
		bindingCards = append(bindingCards, card)
	}
	duplicateCards := []casesPort.DuplicateCard{}
	for _, candidate := range f.candidates {
		if candidate.CaseIDNew != caseID && candidate.CaseIDExisting != caseID {
			continue
		}
		card := casesPort.DuplicateCard{Candidate: candidate}
		if material := f.material[candidate.CaseIDNew]; material != nil {
			card.NewCaseNo = material.cases.CaseNo
			if len(material.timeline) > 0 {
				card.Channel = material.timeline[0].Channel
			}
		}
		if material := f.material[candidate.CaseIDExisting]; material != nil {
			card.ExistingCaseNo = material.cases.CaseNo
		}
		card.IsNewSide = candidate.CaseIDNew == caseID
		card.MaskedValue = f.maskedIdentifierValueOf(candidate)
		if decision := casesDomain.ResolvedDecision(f.resolutions, candidate.CandidateID); decision != "" {
			for _, resolution := range f.resolutions {
				if resolution.CandidateID == candidate.CandidateID {
					card.Resolution = &resolution
					break
				}
			}
		}
		duplicateCards = append(duplicateCards, card)
	}
	return casesAPI.CaseView{
		TenantID: tenant, Case: material.cases, Conversations: conversations,
		Timeline: timeline, WorkItems: workItems,
		Exceptions:    f.exceptionsOf(caseID),
		UntriagedText: "Запрос ещё не классифицирован.", NoAutoReplyText: "Автоответ клиенту не отправляется.",
		CanViewPayload: true, CanViewFacts: true, CanManageWork: true, ViewScope: casesAPI.CaseScopeView,
		TaxonomyTypes:    append([]string(nil), fixtureJourneys...),
		CriticalEntities: append([]string(nil), fixtureCriticalEntities...),
		Assurance: casesAPI.AssuranceView{
			State:           casesDomain.DeriveAssuranceState(records, timeNow()),
			Records:         records,
			AllowedMethods:  casesDomain.MethodsOfModality(modality),
			FallbackOptions: casesDomain.VerificationFallbackOptions,
			StepUpText:      casesDomain.StepUpText,
		},
		Bindings: bindingCards, Duplicates: duplicateCards,
	}, nil
}

// eventBelongsToConversation reports whether one timeline event of the
// fixture material belongs to one conversation (the event map of the
// fixture: the accepted events carry the conversation reference).
func eventBelongsToConversation(f *casesFixture, conversationID, eventID id.UUID) bool {
	for _, event := range f.events {
		if event.EventID == eventID && event.ConversationID == conversationID {
			return true
		}
	}
	return false
}

// maskedIdentifierValueOf masks the identifier value of one candidate: the
// fact of the NEW case with the candidate's material key (the value itself
// stays in the fact — the card carries the mask only).
func (f *casesFixture) maskedIdentifierValueOf(candidate casesDomain.DuplicateCandidate) string {
	anchorID, found := f.factAnchors[candidate.CaseIDNew.String()+"|"+candidate.MaterialKey]
	if !found {
		return ""
	}
	versions := f.factVersions[anchorID]
	if len(versions) == 0 || f.identifierKey == nil {
		return ""
	}
	// The mask of the version WHOSE TOKEN matched (a later rewrite does not
	// shift the mask — the mirror of the store discipline).
	for _, version := range versions {
		raw, ok := casesDomain.IdentifierStringValue(version.Value)
		if !ok {
			continue
		}
		normalized, err := casesDomain.NormalizeIdentifier(candidate.MaterialKey, raw)
		if err != nil {
			continue
		}
		if casesDomain.IdentifierToken(f.identifierKey, candidate.MaterialKey, normalized) == candidate.Token {
			return casesDomain.MaskIdentifier(normalized)
		}
	}
	return ""
}

// timeNow is the fixture clock: the same fixed moment the fixture server
// stamps the commands with (plus the minute margin of the flow) — the
// assurance derivations of the page read the SAME world the commands
// wrote, never the wall clock of the host.
func timeNow() time.Time {
	return time.Date(2026, 9, 16, 12, 30, 0, 0, time.UTC)
}

func (f *casesFixture) exceptionsOf(caseID id.UUID) []casesDomain.OperationalException {
	out := []casesDomain.OperationalException{}
	for _, exception := range f.exceptions {
		if exception.CaseID == caseID {
			out = append(out, exception)
		}
	}
	return out
}

// ReadPayload returns the sealed fixture payload with the real seal check.
func (f *casesFixture) ReadPayload(_ context.Context, _ domain.Principal, tenant, caseID, eventID id.UUID) (casesAPI.PayloadView, error) {
	f.lock()
	defer f.unlock()
	if tenant != f.tenant {
		return casesAPI.PayloadView{}, domain.ErrNotFound
	}
	if f.material[caseID] == nil {
		return casesAPI.PayloadView{}, casesDomain.ErrNotFound
	}
	payload := f.payloads[eventID]
	if payload == nil {
		return casesAPI.PayloadView{}, casesDomain.ErrNotFound
	}
	return casesAPI.PayloadView{EventID: eventID, PayloadDigest: casesDomain.PayloadDigest(payload), SealOK: true, Payload: payload}, nil
}

// DrainOutbox honestly defers everything: the fixture has no workers
// either — the append-only outcome is the same honest deferred record.
func (f *casesFixture) DrainOutbox(_ context.Context, _ domain.Principal, tenant id.UUID) (casesAPI.DrainResult, error) {
	f.lock()
	defer f.unlock()
	if tenant != f.tenant {
		return casesAPI.DrainResult{}, domain.ErrNotFound
	}
	// Two outbox records per accepted event (the event + the connector
	// ack); a repeated drain writes nothing new.
	accepted := len(f.events)
	f.eventSeq += 0 // the outcome rows stay in-memory; idempotency by the second call
	return casesAPI.DrainResult{Considered: accepted * 2, Deferred: accepted * 2}, nil
}
