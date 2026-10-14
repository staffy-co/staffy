package main

import (
	"context"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.4 fixture: the five ownership commands and the reply drain
// over the REAL domain derivations (CurrentOwnership, PendingTransfer,
// DeriveOwnershipState, ExternalSurfaceEvidenced) — the fixture mirrors
// the store through the same domain functions, without convenient
// defaults (the R2 lesson of 2-3).

// fixtureMembers is the active member index of the fixture world.
func (f *casesFixture) fixtureMembers() map[id.UUID]casesPort.OwnerMember {
	return map[id.UUID]casesPort.OwnerMember{
		principalID:      {PrincipalID: principalID, Name: "owner@example.org"},
		secondOperatorID: {PrincipalID: secondOperatorID, Name: "backup@example.org"},
	}
}

// fixtureCoverage resolves the coverage word of the fixture route (the
// deadline comes from the coverage, never from a command).
func (f *casesFixture) fixtureCoverage() casesDomain.CoverageWord {
	for _, coverage := range f.coverage {
		if coverage.Direction == "inbound" {
			return coverage
		}
	}
	return casesDomain.CoverageWord{}
}

// fixtureState derives the ownership state of the case over the REAL
// domain function with the member activity word.
func (f *casesFixture) fixtureState(caseID id.UUID, now time.Time) casesDomain.OwnershipState {
	members := f.fixtureMembers()
	current := casesDomain.CurrentOwnership(f.ownershipTerms[caseID])
	ownerActive := current == nil || current.OwnerKind != casesDomain.OwnerKindHuman
	if current != nil && current.OwnerKind == casesDomain.OwnerKindHuman {
		_, ownerActive = members[current.OwnerRef]
	}
	return casesDomain.DeriveOwnershipState(f.ownershipTerms[caseID], f.transferRequests[caseID], f.transferResolutions, ownerActive, now)
}

// fixtureOwnershipID generates the next deterministic ownership id.
func (f *casesFixture) fixtureOwnershipID() id.UUID {
	f.ownershipSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(20000+f.ownershipSeq)))
}

// fixtureAppendTerm writes one transition term (the take, the
// acknowledged transfer, the reassignment).
func (f *casesFixture) fixtureAppendTerm(caseID id.UUID, term casesDomain.OwnershipTerm) {
	f.ownershipTerms[caseID] = append(f.ownershipTerms[caseID], term)
}

// TakeOwnership takes the ownership in a takeable state (the REAL
// derivation decides).
func (f *casesFixture) TakeOwnership(_ context.Context, command casesAPI.TakeOwnershipCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("take_ownership", casesDomain.GuardExpectedEpoch(command.ExpectedEpoch)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := fixtureWorkRule("take_ownership", casesDomain.GuardOwnershipReason(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("take_ownership", command.IdempotencyKey, struct {
		Case     string `json:"case"`
		Expected int64  `json:"expected_epoch"`
		Reason   string `json:"reason"`
	}{command.CaseID.String(), command.ExpectedEpoch, command.Reason})
	if err != nil || !fresh {
		return outcome, err
	}
	if _, ok := f.fixtureMembers()[command.Actor.ID]; !ok {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("take_ownership", fmt.Errorf("владельцем может быть только активный участник тенанта"))
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	state := f.fixtureState(command.CaseID, command.Now)
	if state.Current == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if state.Current.Epoch != command.ExpectedEpoch {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if !state.Takeable() {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("take_ownership", fmt.Errorf("%s", casesDomain.HealthyTakeText))
	}
	// The take voids the unresolved ask with the cancel resolution of the
	// same flow (the store mirror): after the transition the ownership is
	// healthy — the zombie expired ask never keeps the case takeable.
	if pending := casesDomain.PendingTransfer(f.ownershipTerms[command.CaseID], f.transferRequests[command.CaseID], f.transferResolutions, command.Now); pending != nil {
		resolutionID := f.fixtureOwnershipID()
		f.transferResolutions = append(f.transferResolutions, casesDomain.TransferResolution{
			TenantID: f.tenant, ResolutionID: resolutionID, RequestID: pending.RequestID,
			Kind: casesDomain.TransferResolutionCancelled, ActorID: command.Actor.ID,
			Reason: "погашено взятием владения (дедлайн истёк или владелец недоступен)", CreatedAt: command.Now,
		})
	}
	termID := f.fixtureOwnershipID()
	f.fixtureAppendTerm(command.CaseID, casesDomain.OwnershipTerm{
		TenantID: f.tenant, CaseID: command.CaseID, TermID: termID,
		Epoch: state.Current.Epoch + 1, OwnerKind: casesDomain.OwnerKindHuman, OwnerRef: command.Actor.ID,
		WritableSurface: casesDomain.WritableSurfaceInbox, Origin: casesDomain.OwnershipOriginTake,
		PreviousOwnerKind: state.Current.OwnerKind, PreviousOwnerRef: state.Current.OwnerRef,
		ActorID: command.Actor.ID, Reason: command.Reason, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "take_ownership", Epoch: state.Current.Epoch + 1}
	f.storeOutcome("take_ownership", command.IdempotencyKey, outcome)
	return outcome, nil
}

// RequestTransfer creates the assigned handoff ask: the deadline from the
// fixture coverage word (never from the command).
func (f *casesFixture) RequestTransfer(_ context.Context, command casesAPI.RequestTransferCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("request_transfer", casesDomain.GuardExpectedEpoch(command.ExpectedEpoch)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := fixtureWorkRule("request_transfer", casesDomain.GuardRequestTransfer(command.ToOwnerKind, command.WritableSurface, command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("request_transfer", command.IdempotencyKey, struct {
		Case     string `json:"case"`
		Expected int64  `json:"expected_epoch"`
		ToKind   string `json:"to_owner_kind"`
		ToRef    string `json:"to_owner_ref"`
		Surface  string `json:"writable_surface"`
		Reason   string `json:"reason"`
	}{command.CaseID.String(), command.ExpectedEpoch, command.ToOwnerKind, command.ToOwnerRef.String(), command.WritableSurface, command.Reason})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	state := f.fixtureState(command.CaseID, command.Now)
	if state.Current == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if state.Current.Epoch != command.ExpectedEpoch {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if !(state.Current.OwnerKind == casesDomain.OwnerKindHuman && state.Current.OwnerRef == command.Actor.ID) {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("request_transfer", fmt.Errorf("%s", casesDomain.TransferRequestScopeText))
	}
	if command.ToOwnerKind == casesDomain.OwnerKindHuman {
		if _, ok := f.fixtureMembers()[command.ToOwnerRef]; !ok {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("request_transfer", fmt.Errorf("владелец-цель должен быть активным участником тенанта (очередь или отдел владельцем быть не могут)"))
		}
	}
	coverage := f.fixtureCoverage()
	if coverage.AckDeadlineMinutes < 1 {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("request_transfer", fmt.Errorf("%s", casesDomain.CoverageUnavailableText))
	}
	if ask := casesDomain.ActiveTransferRequest(f.ownershipTerms[command.CaseID], f.transferRequests[command.CaseID], f.transferResolutions, command.Now); ask != nil {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("request_transfer", fmt.Errorf("%s", casesDomain.PendingTransferText))
	}
	requestID := f.fixtureOwnershipID()
	requests := f.transferRequests[command.CaseID]
	f.transferRequests[command.CaseID] = append(requests, casesDomain.TransferRequest{
		TenantID: f.tenant, CaseID: command.CaseID, RequestID: requestID, RequestNo: int64(len(requests) + 1),
		FromOwnerKind: state.Current.OwnerKind, FromOwnerRef: state.Current.OwnerRef,
		ToOwnerKind: command.ToOwnerKind, ToOwnerRef: command.ToOwnerRef,
		WritableSurface: command.WritableSurface, Reason: command.Reason, RequestedBy: command.Actor.ID,
		DeadlineAt: command.Now.Add(time.Duration(coverage.AckDeadlineMinutes) * time.Minute), CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "request_transfer", TransferRequestID: requestID, Epoch: command.ExpectedEpoch}
	f.storeOutcome("request_transfer", command.IdempotencyKey, outcome)
	return outcome, nil
}

// ResolveTransfer decides one request: the acknowledge — the atomic
// transition; the decline and the cancel — the resolution only.
func (f *casesFixture) ResolveTransfer(_ context.Context, command casesAPI.ResolveTransferCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("resolve_transfer", casesDomain.GuardResolveTransfer(command.Decision, command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("resolve_transfer", command.IdempotencyKey, struct {
		Case     string `json:"case"`
		Request  string `json:"request"`
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}{command.CaseID.String(), command.RequestID.String(), command.Decision, command.Reason})
	if err != nil || !fresh {
		return outcome, err
	}
	var request *casesDomain.TransferRequest
	for i := range f.transferRequests[command.CaseID] {
		if f.transferRequests[command.CaseID][i].RequestID == command.RequestID {
			request = &f.transferRequests[command.CaseID][i]
			break
		}
	}
	if request == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	for _, resolution := range f.transferResolutions {
		if resolution.RequestID == request.RequestID {
			return casesAPI.WorkOutcome{}, casesDomain.ErrCommandKeyConflict
		}
	}
	if command.Decision == casesDomain.TransferResolutionAcknowledged {
		if request.ToOwnerKind != casesDomain.OwnerKindHuman || request.ToOwnerRef != command.Actor.ID {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("resolve_transfer", fmt.Errorf("%s", casesDomain.OwnerNotTargetText))
		}
		if !command.Now.Before(request.DeadlineAt) {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("resolve_transfer", fmt.Errorf("%s", casesDomain.RequestExpiredText))
		}
		state := f.fixtureState(command.CaseID, command.Now)
		if state.Current == nil {
			return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
		}
		resolutionID := f.fixtureOwnershipID()
		f.transferResolutions = append(f.transferResolutions, casesDomain.TransferResolution{
			TenantID: f.tenant, ResolutionID: resolutionID, RequestID: request.RequestID,
			Kind: command.Decision, ActorID: command.Actor.ID, ResultingEpoch: state.Current.Epoch + 1,
			Reason: command.Reason, CreatedAt: command.Now,
		})
		termID := f.fixtureOwnershipID()
		f.fixtureAppendTerm(command.CaseID, casesDomain.OwnershipTerm{
			TenantID: f.tenant, CaseID: command.CaseID, TermID: termID,
			Epoch: state.Current.Epoch + 1, OwnerKind: request.ToOwnerKind, OwnerRef: request.ToOwnerRef,
			WritableSurface: request.WritableSurface, Origin: casesDomain.OwnershipOriginTransferAcknowledge,
			PreviousOwnerKind: state.Current.OwnerKind, PreviousOwnerRef: state.Current.OwnerRef,
			ActorID: command.Actor.ID, TransferRequestID: request.RequestID,
			RequestedAt: request.CreatedAt, AcknowledgedAt: command.Now, CreatedAt: command.Now,
		})
		outcome = casesAPI.WorkOutcome{Command: "resolve_transfer", TransferRequestID: request.RequestID, TransferDecision: command.Decision, Epoch: state.Current.Epoch + 1}
		f.storeOutcome("resolve_transfer", command.IdempotencyKey, outcome)
		return outcome, nil
	}
	if !(request.ToOwnerKind == casesDomain.OwnerKindHuman && request.ToOwnerRef == command.Actor.ID) && request.RequestedBy != command.Actor.ID {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("resolve_transfer", fmt.Errorf("%s", casesDomain.ResolveNotPermittedText))
	}
	resolutionID := f.fixtureOwnershipID()
	f.transferResolutions = append(f.transferResolutions, casesDomain.TransferResolution{
		TenantID: f.tenant, ResolutionID: resolutionID, RequestID: request.RequestID,
		Kind: command.Decision, ActorID: command.Actor.ID, Reason: command.Reason, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "resolve_transfer", TransferRequestID: request.RequestID, TransferDecision: command.Decision}
	f.storeOutcome("resolve_transfer", command.IdempotencyKey, outcome)
	return outcome, nil
}

// ReassignOwnership is the supervision path (the fixture owner carries
// every scope, including configuration).
func (f *casesFixture) ReassignOwnership(_ context.Context, command casesAPI.ReassignOwnershipCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("reassign_ownership", casesDomain.GuardExpectedEpoch(command.ExpectedEpoch)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := fixtureWorkRule("reassign_ownership", casesDomain.GuardRequestTransfer(command.ToOwnerKind, command.WritableSurface, command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("reassign_ownership", command.IdempotencyKey, struct {
		Case     string `json:"case"`
		Expected int64  `json:"expected_epoch"`
		ToKind   string `json:"to_owner_kind"`
		ToRef    string `json:"to_owner_ref"`
		Surface  string `json:"writable_surface"`
		Reason   string `json:"reason"`
	}{command.CaseID.String(), command.ExpectedEpoch, command.ToOwnerKind, command.ToOwnerRef.String(), command.WritableSurface, command.Reason})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	state := f.fixtureState(command.CaseID, command.Now)
	if state.Current == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if state.Current.Epoch != command.ExpectedEpoch {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if command.ToOwnerKind == casesDomain.OwnerKindHuman {
		if _, ok := f.fixtureMembers()[command.ToOwnerRef]; !ok {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("reassign_ownership", fmt.Errorf("владелец-цель должен быть активным участником тенанта"))
		}
	}
	// The pending request void (the cancel resolution of the same flow).
	if state.Pending != nil {
		resolutionID := f.fixtureOwnershipID()
		f.transferResolutions = append(f.transferResolutions, casesDomain.TransferResolution{
			TenantID: f.tenant, ResolutionID: resolutionID, RequestID: state.Pending.RequestID,
			Kind: casesDomain.TransferResolutionCancelled, ActorID: command.Actor.ID,
			Reason: "отменено переназначением надзора", CreatedAt: command.Now,
		})
	}
	termID := f.fixtureOwnershipID()
	f.fixtureAppendTerm(command.CaseID, casesDomain.OwnershipTerm{
		TenantID: f.tenant, CaseID: command.CaseID, TermID: termID,
		Epoch: state.Current.Epoch + 1, OwnerKind: command.ToOwnerKind, OwnerRef: command.ToOwnerRef,
		WritableSurface: command.WritableSurface, Origin: casesDomain.OwnershipOriginReassign,
		PreviousOwnerKind: state.Current.OwnerKind, PreviousOwnerRef: state.Current.OwnerRef,
		ActorID: command.Actor.ID, Reason: command.Reason, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "reassign_ownership", Epoch: state.Current.Epoch + 1}
	f.storeOutcome("reassign_ownership", command.IdempotencyKey, outcome)
	return outcome, nil
}

// SendClientReply is the only client-bound outgoing path: the gate over
// the REAL derivation, the full send snapshot, the content that never
// leaves the record.
func (f *casesFixture) SendClientReply(_ context.Context, command casesAPI.SendClientReplyCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	outcome, fresh, err := f.workCommandFlow("send_client_reply", command.IdempotencyKey, struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		WorkItem     string `json:"work_item"`
		Expected     int64  `json:"expected_epoch"`
		Class        string `json:"reply_class"`
		Step         string `json:"playbook_step"`
		Commitment   string `json:"commitment_id"`
		CVersion     int64  `json:"commitment_version"`
		Content      string `json:"content"`
		Link         string `json:"link_id"`
		Qualifier    string `json:"accuracy_qualifier"`
	}{command.CaseID.String(), command.ConversationID.String(), command.WorkItemID.String(), command.ExpectedEpoch, command.ReplyClass, command.PlaybookStep, command.CommitmentID.String(), command.CommitmentVersionNo, command.Content, command.LinkID.String(), command.AccuracyQualifier})
	if err != nil || !fresh {
		return outcome, err
	}
	if err := fixtureWorkRule("send_client_reply", casesDomain.GuardSendClientReply(command.ExpectedEpoch, command.Content)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The reply class is the producer's mark (the machine never classifies
	// the content — no silent default).
	if err := fixtureWorkRule("send_client_reply", casesDomain.GuardReplyClass(command.ReplyClass)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The governing case of the target conversation: the case it is
	// actively bound INTO (the REAL 2-3 derivation), or its own case.
	ownCase, known := f.caseOfConversation(command.ConversationID)
	conversation := casesDomain.Conversation{}
	if known {
		conversation, _ = f.conversationOf(ownCase, command.ConversationID)
	}
	governing := ownCase
	for _, binding := range casesDomain.ActiveBindings(f.bindings, f.reversals) {
		if binding.BoundConversationID == command.ConversationID {
			governing = binding.CaseID
			break
		}
	}
	if governing != command.CaseID || !known {
		if !known {
			return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
		}
		return casesAPI.WorkOutcome{}, f.fixtureGateRefusal(governing, casesDomain.GoverningCaseText)
	}
	state := f.fixtureState(command.CaseID, command.Now)
	if state.Current == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	members := f.fixtureMembers()
	ownerName := "цифровой сотрудник"
	if state.Current.OwnerKind == casesDomain.OwnerKindHuman {
		if member, ok := members[state.Current.OwnerRef]; ok && member.Name != "" {
			ownerName = member.Name
		} else {
			ownerName = state.Current.OwnerRef.String()
		}
	}
	if state.Current.OwnerKind == casesDomain.OwnerKindStaffy {
		return casesAPI.WorkOutcome{}, casesDomain.ReplyGateError{OwnerKind: state.Current.OwnerKind, OwnerName: ownerName, CurrentEpoch: state.Current.Epoch, Reason: casesDomain.StaffyOwnerText}
	}
	if state.ReplyBlocked() {
		return casesAPI.WorkOutcome{}, casesDomain.ReplyGateError{OwnerKind: state.Current.OwnerKind, OwnerName: ownerName, CurrentEpoch: state.Current.Epoch, Reason: casesDomain.ReassignmentText}
	}
	if state.Current.OwnerRef != command.Actor.ID {
		return casesAPI.WorkOutcome{}, casesDomain.ReplyGateError{OwnerKind: state.Current.OwnerKind, OwnerName: ownerName, CurrentEpoch: state.Current.Epoch, Reason: casesDomain.NotOwnerText}
	}
	if command.ExpectedEpoch != state.Current.Epoch {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	// The mode gate (Story 2.5, AFTER the ownership gate): the effective
	// mode of the Work Item the reply belongs to decides the sales class.
	if err := f.fixtureModeGate(command); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The disclosure gate (the sales and the protected classes): the
	// acting disclosure of the channel of the reply.
	channel := fixtureReplyChannel(conversation)
	if err := f.fixtureDisclosureGate(command, channel); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The policy gate (Story 2.6, AFTER the disclosure gate — the frozen
	// order: ownership 2-4 → mode 2-5 → disclosure 2-5 → policy 2-6 →
	// CAS): the sales reply needs the Work Item anchor and the CURRENT
	// outcome of that item to be commercial_answer.
	policyOutcomeID, err := f.fixturePolicyGate(command)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The objection gate (Story 2.8, AFTER the policy gate — the frozen
	// ladder: ownership 2-4 → mode 2-5 → disclosure 2-5 → policy 2-6 →
	// objection 2-8 → write; the SALES class only — the neutral and the
	// protected paths stay byte-identical): the stopped persuasion refuses
	// with the allowed endings; a CURRENT objection of the Work Item makes
	// the playbook step mandatory (the first linked answer — always
	// acknowledge_concern). The passing gate resolves the objection id the
	// reply link carries (the request never carries it).
	objectionID, err := f.fixtureObjectionGate(command)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The accuracy gate (Story 2.11, AFTER the objection gate — the frozen
	// ladder: ownership 2.4 → mode 2.5 → disclosure 2.5 → policy 2.6 →
	// objection 2.8 → accuracy 2.11 → write; the SALES class only — the
	// neutral and the protected paths stay byte-identical): the CURRENT
	// outcome of the Work Item decides which words are honest — an
	// outcome with active alternatives REQUIRES the qualifier word of
	// its active set (the absence of media never becomes an invented
	// exact result); an outcome without alternatives requires the EMPTY
	// word (the field has exactly one meaning).
	if command.ReplyClass == casesDomain.ReplyClassSales {
		if err := f.fixtureAccuracyGate(command); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	} else if command.AccuracyQualifier != "" {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("send_client_reply", fmt.Errorf("Слово ограничения точности несёт только коммерческий ответ (sales): нейтральные и защищённые сообщения не заявляют ограничение точности."))
	}
	if objectionID.Valid() && command.PlaybookStep != "" {
		f.replyLinks[command.CaseID] = append(f.replyLinks[command.CaseID], casesDomain.ReplyLink{
			ObjectionID: objectionID, Step: command.PlaybookStep,
		})
	}
	// The Story 2.9 presentation link: the nullable pair is both or
	// neither; ONLY the sales class carries the commercial material of
	// the EXACT shown offered version; the linked version must exist and
	// be offered — the draft never presents.
	if command.CommitmentID.Valid() || command.CommitmentVersionNo != 0 {
		if command.ReplyClass != casesDomain.ReplyClassSales {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("send_client_reply", fmt.Errorf("Связь обязательства несёт только коммерческий ответ (sales): нейтральные и защищённые сообщения не предъявляют материальные условия."))
		}
		linked := false
		for _, version := range f.commitments[command.CaseID] {
			if version.CommitmentID == command.CommitmentID && version.VersionNo == command.CommitmentVersionNo {
				linked = true
				if version.Status != casesDomain.CommitmentStatusOffered {
					return casesAPI.WorkOutcome{}, fixtureWorkRule("send_client_reply", fmt.Errorf("Черновик не считается предъявляемым обязательством: свяжите offered-версию (draft-not-presentable)."))
				}
				if version.WorkItemID != command.WorkItemID {
					return casesAPI.WorkOutcome{}, fixtureWorkRule("send_client_reply", fmt.Errorf("Версия обязательства принадлежит другому элементу работы: связь возможна только в пределах одного элемента."))
				}
				break
			}
		}
		if !linked {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("send_client_reply", fmt.Errorf("Версия обязательства не найдена: связь сообщения-предъявления требует существующей offered-версии этого элемента работы."))
		}
		f.presented[command.CaseID] = append(f.presented[command.CaseID], casesDomain.PresentedCommitment{
			CommitmentID: command.CommitmentID, VersionNo: command.CommitmentVersionNo,
		})
	}
	// The Story 2.10 continuation link binding gate (AFTER the
	// commitment gate — the frozen ladder: ownership 2.4 → mode 2.5
	// → disclosure 2.5 → policy 2.6 → objection 2.8 → commitment
	// link 2.9 → continuation link 2.10 → write): the nullable LinkID
	// is carried ONLY by the sales class (the delivered badge of the
	// FR-16); the linked row must exist and belong to THIS case. The
	// state never matters here: the binding is the honest delivery
	// record, a dead link may be bound after the fact (the panel
	// derives the state read-time).
	if command.LinkID.Valid() {
		if command.ReplyClass != casesDomain.ReplyClassSales {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("send_client_reply", fmt.Errorf("Связь ссылки продолжения несёт только коммерческий ответ (sales): нейтральные и защищённые сообщения не вплетают ссылку продолжения."))
		}
		link, known := f.contLinks[command.LinkID]
		if !known || link.CaseID != command.CaseID {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("send_client_reply", fmt.Errorf("Ссылка продолжения не найдена: связь возможна только со ссылкой этого обращения."))
		}
		f.contDelivered[command.LinkID] = true
	}
	replyID := f.fixtureOwnershipID()
	f.replies[command.CaseID] = append(f.replies[command.CaseID], casesDomain.ClientReply{
		TenantID: f.tenant, CaseID: command.CaseID, ConversationID: command.ConversationID, ReplyID: replyID,
		OwnerKind: state.Current.OwnerKind, OwnerRef: state.Current.OwnerRef, Epoch: state.Current.Epoch,
		WritableSurface: state.Current.WritableSurface, ConnectionID: conversation.ConnectionID,
		Provider: conversation.Provider, Channel: channel, Direction: "outbound",
		EmployeeID: f.employee, DecisionNo: 1, ProfileVersion: 1,
		Content: command.Content, ActorID: command.Actor.ID,
		ReplyClass: command.ReplyClass, WorkItemID: command.WorkItemID, PolicyOutcomeID: policyOutcomeID, CreatedAt: command.Now,
		AccuracyQualifier: command.AccuracyQualifier,
	})
	outcome = casesAPI.WorkOutcome{Command: "send_client_reply", ReplyID: replyID, ReplyState: "prepared", Epoch: state.Current.Epoch, ReplyClass: command.ReplyClass, PolicyOutcomeID: policyOutcomeID, ObjectionID: objectionID, PlaybookStep: command.PlaybookStep, AccuracyQualifier: command.AccuracyQualifier}
	f.storeOutcome("send_client_reply", command.IdempotencyKey, outcome)
	return outcome, nil
}

// channelOfFixtureModality resolves the fixture channel of a modality.
func channelOfFixtureModality(modality string) string {
	if modality == casesDomain.ModalityVoice {
		return "phone"
	}
	return "web_chat"
}

// fixtureGateRefusal resolves the machine-readable gate refusal of a
// governing case.
func (f *casesFixture) fixtureGateRefusal(caseID id.UUID, reason string) error {
	state := f.fixtureState(caseID, timeNow())
	if state.Current == nil {
		return casesDomain.ErrNotFound
	}
	ownerName := "цифровой сотрудник"
	if state.Current.OwnerKind == casesDomain.OwnerKindHuman {
		if member, ok := f.fixtureMembers()[state.Current.OwnerRef]; ok {
			ownerName = member.Name
		} else {
			ownerName = state.Current.OwnerRef.String()
		}
	}
	return casesDomain.ReplyGateError{OwnerKind: state.Current.OwnerKind, OwnerName: ownerName, CurrentEpoch: state.Current.Epoch, Reason: reason}
}

// DrainClientReplies drives the execute-time revalidation over the REAL
// derivation: fresh — dispatched; stale — stale_needs_review; the
// attention subjects raise the exception once.
func (f *casesFixture) DrainClientReplies(_ context.Context, _ domain.Principal, tenant id.UUID) (casesAPI.DrainRepliesResult, error) {
	f.lock()
	defer f.unlock()
	if tenant != f.tenant {
		return casesAPI.DrainRepliesResult{}, domain.ErrNotFound
	}
	result := casesAPI.DrainRepliesResult{}
	now := timeNow()
	for caseID, replies := range f.replies {
		for _, reply := range replies {
			if _, decided := f.replyOutcomes[reply.ReplyID]; decided {
				continue
			}
			result.Considered++
			state := f.fixtureState(caseID, now)
			material := f.material[caseID]
			caseOpen := material != nil && material.cases.State == casesDomain.CaseStateOpen
			ownerActive := true
			if state.Current != nil && state.Current.OwnerKind == casesDomain.OwnerKindHuman {
				_, ownerActive = f.fixtureMembers()[state.Current.OwnerRef]
			}
			fresh := state.Current != nil && caseOpen && ownerActive &&
				state.Current.OwnerKind == reply.OwnerKind && state.Current.OwnerRef == reply.OwnerRef &&
				state.Current.Epoch == reply.Epoch
			kind := casesDomain.ReplyOutcomeStaleNeedsReview
			if fresh {
				kind = casesDomain.ReplyOutcomeDispatched
			}
			f.ownershipSeq++
			f.replyOutcomes[reply.ReplyID] = casesDomain.ClientReplyOutcome{
				TenantID: f.tenant, OutcomeID: id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(20000+f.ownershipSeq))),
				ReplyID: reply.ReplyID, Kind: kind, CreatedAt: now,
			}
			if kind == casesDomain.ReplyOutcomeDispatched {
				result.Dispatched++
			} else {
				result.Stale++
			}
			// The attention subjects: the expired pending request and the
			// unavailable owner (once per subject — the in-memory UNIQUE).
			if state.Pending != nil && state.PendingTimedOut {
				f.raiseAttentionOnce(state.Pending.RequestID, caseID, reply.ConversationID, now)
			}
			if state.Health == casesDomain.OwnershipHealthReassignment && state.Current != nil {
				f.raiseAttentionOnce(state.Current.TermID, caseID, reply.ConversationID, now)
			}
		}
	}
	return result, nil
}

// raiseAttentionOnce appends the ownership_attention exception unless the
// subject already raised it (the material uniqueness of the store).
func (f *casesFixture) raiseAttentionOnce(subject id.UUID, caseID, conversationID id.UUID, now time.Time) {
	for _, exception := range f.exceptions {
		if exception.Kind == casesDomain.ExceptionOwnershipAttention && exception.CorrelationID == subject {
			return
		}
	}
	f.exceptionSeq++
	exceptionID := id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(30000+f.exceptionSeq)))
	f.exceptions = append(f.exceptions, casesDomain.OperationalException{
		TenantID: f.tenant, ExceptionID: exceptionID, Kind: casesDomain.ExceptionOwnershipAttention,
		Summary: casesDomain.OwnershipAttentionSummary, CorrelationID: subject,
		CaseID: caseID, ConversationID: conversationID, CreatedAt: now,
	})
}

// fixtureOwnershipView assembles the ownership section of the case view.
func (f *casesFixture) fixtureOwnershipView(caseID id.UUID) casesAPI.OwnershipView {
	members := f.fixtureMembers()
	state := f.fixtureState(caseID, timeNow())
	coverage := f.fixtureCoverage()
	state.BackupOwnerID = coverage.BackupOwnerID
	state.AckDeadlineMinutes = coverage.AckDeadlineMinutes
	view := casesAPI.OwnershipView{
		State: state,
		Terms: append([]casesDomain.OwnershipTerm(nil), f.ownershipTerms[caseID]...),
		Members: []casesPort.OwnerMember{
			{PrincipalID: principalID, Name: members[principalID].Name},
			{PrincipalID: secondOperatorID, Name: members[secondOperatorID].Name},
		},
	}
	if state.Current != nil {
		if state.Current.OwnerKind == casesDomain.OwnerKindStaffy {
			view.OwnerName = "цифровой сотрудник"
		} else if member, ok := members[state.Current.OwnerRef]; ok && member.Name != "" {
			view.OwnerName = member.Name
		} else {
			view.OwnerName = state.Current.OwnerRef.String()
		}
	}
	if state.Pending != nil {
		if state.Pending.ToOwnerKind == casesDomain.OwnerKindStaffy {
			view.PendingTargetName = "цифровой сотрудник"
		} else if member, ok := members[state.Pending.ToOwnerRef]; ok && member.Name != "" {
			view.PendingTargetName = member.Name
		} else {
			view.PendingTargetName = state.Pending.ToOwnerRef.String()
		}
	}
	for _, reply := range f.replies[caseID] {
		card := casesPort.ReplyCard{Reply: reply}
		if outcome, decided := f.replyOutcomes[reply.ReplyID]; decided {
			card.OutcomeKind = outcome.Kind
			card.OutcomeAt = outcome.CreatedAt
		}
		view.Replies = append(view.Replies, card)
	}
	return view
}
