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
// The five ownership commands and the reply drain (Story 2.4)
// ---------------------------------------------------------------------------
//
// Every command is idempotent by the tenant-scoped key with the stored
// outcome (the 2-2 precedent: the replay answer comes BEFORE the domain
// guards); every transition carries the expected epoch — the store
// re-validates it under the case advisory lock (the CAS discipline); every
// write is append-only. The reply gate is the server-side truth: the actor
// identity, the current term and the health word decide — the UI button is
// only UX.

// ownRights is the accumulated scope word of the ownership surface: the
// review scope of the commands, the configuration scope of the transfer
// requests and the supervision reassignments.
type ownRights struct {
	Review        bool
	Configuration bool
}

// ownershipScope authorizes the ownership surface: a member with the
// review or the configuration scope enters; the command-specific role
// checks (the current owner word, the configuration path) run later, on
// the state. A member without any scope gets an honest refusal, a
// non-member a plain 404.
func (s *Service) ownershipScope(ctx context.Context, actor domain.Principal, tenantID id.UUID) (tenancy.Context, ownRights, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, ownRights{}, domain.ErrNotFound
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return tenancy.Context{}, ownRights{}, err
	}
	rights := ownRights{}
	memberWithoutScope := false
	anyScope := false
	var infraFailure error
	for _, required := range []domain.Scope{domain.Review, domain.Configuration, domain.Audit} {
		err := s.identity.Authorize(ctx, actor, tenantID, required)
		if err == nil {
			anyScope = true
			if required == domain.Review {
				rights.Review = true
			}
			if required == domain.Configuration {
				rights.Configuration = true
			}
			continue
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
	if anyScope {
		return scope, rights, nil
	}
	if infraFailure != nil {
		return tenancy.Context{}, ownRights{}, infraFailure
	}
	if memberWithoutScope {
		return tenancy.Context{}, ownRights{}, domain.ErrForbidden
	}
	return tenancy.Context{}, ownRights{}, domain.ErrNotFound
}

// memberIndex reads the member facts of the tenant: the active word and
// the honest name (the email) per principal. The read is the caller-
// authority projection of the command scope.
func (s *Service) memberIndex(ctx context.Context, scope tenancy.Context) (map[id.UUID]casesPort.OwnerMember, error) {
	if s.members == nil {
		return nil, fmt.Errorf("факт-порт участников не сконфигурирован: команды владения честно отклоняются")
	}
	memberships, err := s.members.MembersFacts(ctx, domain.Principal{ID: scope.PrincipalID()}, scope.TenantID())
	if err != nil {
		return nil, err
	}
	index := make(map[id.UUID]casesPort.OwnerMember, len(memberships))
	for _, membership := range memberships {
		if membership.PrincipalID == nil || membership.Status != "active" {
			continue
		}
		index[*membership.PrincipalID] = casesPort.OwnerMember{PrincipalID: *membership.PrincipalID, Name: membership.Email}
	}
	return index, nil
}

// caseConnection resolves the original conversation of the case (the one
// born with it — the bound conversations belong to other cases): the
// connection of the acting decision that admitted the case.
func (s *Service) caseConnection(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesDomain.Conversation, error) {
	detail, err := s.repository.ReadCase(ctx, scope, caseID)
	if err != nil {
		return casesDomain.Conversation{}, err
	}
	for _, conversation := range detail.Conversations {
		if conversation.CaseID == caseID {
			return conversation, nil
		}
	}
	return casesDomain.Conversation{}, casesDomain.ErrNotFound
}

// coverageWord re-reads the human coverage of the acting route decision:
// the backup owner and the ack deadline of the case's channel. The word
// NEVER comes from a command parameter — an unreadable coverage is the
// honest fail-closed refusal (AC-5/AC-9 inputs).
func (s *Service) coverageWord(ctx context.Context, scope tenancy.Context, conversation casesDomain.Conversation) (casesDomain.CoverageWord, error) {
	if s.routeFacts == nil {
		return casesDomain.CoverageWord{}, rule("request_transfer", fmt.Errorf("%s", casesDomain.CoverageUnavailableText))
	}
	routes, err := s.routeFacts.RouteFacts(ctx, scope, conversation.ConnectionID)
	if err != nil {
		return casesDomain.CoverageWord{}, err
	}
	channel := channelOfModality(conversation.Modality)
	for _, candidate := range routes.Candidates {
		for _, coverage := range candidate.Coverage {
			if coverage.Route.Channel == channel && coverage.Route.Direction == "inbound" {
				return casesDomain.CoverageWord{
					Channel: coverage.Route.Channel, Direction: coverage.Route.Direction,
					ResponseOwnerID: coverage.ResponseOwnerID, BackupOwnerID: coverage.BackupOwnerID,
					AckDeadlineMinutes: coverage.AckDeadlineMinutes,
				}, nil
			}
		}
	}
	return casesDomain.CoverageWord{}, rule("request_transfer", fmt.Errorf("%s", casesDomain.CoverageUnavailableText))
}

// channelOfModality resolves the honest default channel of a modality
// (the coverage lookup word of the ownership commands — the case channel
// of the conversation).
func channelOfModality(modality string) string {
	if modality == casesDomain.ModalityVoice {
		return "phone"
	}
	return "web_chat"
}

// lineageWord re-reads the acting release decision of the case
// connection: the employee, the decision number and the profile version —
// the lineage snapshot of the client reply (AC-2).
func (s *Service) lineageWord(ctx context.Context, scope tenancy.Context, conversation casesDomain.Conversation) (employee id.UUID, decisionNo, profileVersion int64, err error) {
	if s.routeFacts == nil {
		return id.UUID(""), 0, 0, fmt.Errorf("факт-порт маршрутизации не сконфигурирован: lineage ответа недоступен, команда честно отклоняется")
	}
	routes, err := s.routeFacts.RouteFacts(ctx, scope, conversation.ConnectionID)
	if err != nil {
		return id.UUID(""), 0, 0, err
	}
	if len(routes.Candidates) == 0 {
		return id.UUID(""), 0, 0, casesDomain.ErrNotFound
	}
	candidate := routes.Candidates[0]
	return candidate.EmployeeID, candidate.DecisionNo, candidate.ProfileVersion, nil
}

// TakeOwnership takes the ownership of a case in a takeable state: the
// acknowledgement deadline of the assigned request expired without a
// decision, or the current owner is unavailable. A healthy ownership is
// never takeable — the honest refusal names the transfer path. The store
// re-derives the takeable state under the case lock.
func (s *Service) TakeOwnership(ctx context.Context, command casesAPI.TakeOwnershipCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("take_ownership", casesDomain.GuardExpectedEpoch(command.ExpectedEpoch)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("take_ownership", casesDomain.GuardOwnershipReason(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case     string `json:"case"`
		Expected int64  `json:"expected_epoch"`
		Reason   string `json:"reason"`
	}{command.CaseID.String(), command.ExpectedEpoch, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "take_ownership", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The member word: the taker must resolve to an active member
	// (the domain AC-1 discipline — the owner is never a queue or an
	// unconfirmed candidate).
	members, err := s.memberIndex(ctx, scope)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if _, ok := members[command.Actor.ID]; !ok {
		return casesAPI.WorkOutcome{}, rule("take_ownership", fmt.Errorf("владельцем может быть только активный участник тенанта"))
	}
	// The current owner activity word drives the takeable state (the
	// honest snapshot; the store re-derives the current term under the
	// case lock).
	terms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	ownerActive := ownerActivityWord(casesDomain.CurrentOwnership(terms), members)
	termID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	resolutionID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "take_ownership", Epoch: command.ExpectedEpoch + 1,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "take_ownership", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ExpectedVersion: command.ExpectedEpoch,
		OwnerKind: casesDomain.OwnerKindHuman, OwnerRef: command.Actor.ID,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: command.Reason,
		VerificationID: termID, TransferResolutionID: resolutionID,
		OutboxEventID: outboxEventID, OwnerActiveWord: ownerActive,
		Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// RequestTransfer creates the assigned handoff ask: the current owner (or
// a configuration member) names the target — an active member or the
// Staffy employee of the case — with the surface and the honest reason.
// The deadline comes from the human coverage word of the acting route
// decision (the RouteFacts re-read) — never from the command. The store
// re-derives the no-pending discipline under the case lock (the honest
// 409 of the concurrent second request).
func (s *Service) RequestTransfer(ctx context.Context, command casesAPI.RequestTransferCommand) (casesAPI.WorkOutcome, error) {
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("request_transfer", casesDomain.GuardExpectedEpoch(command.ExpectedEpoch)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("request_transfer", casesDomain.GuardRequestTransfer(command.ToOwnerKind, command.WritableSurface, command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case     string `json:"case"`
		Expected int64  `json:"expected_epoch"`
		ToKind   string `json:"to_owner_kind"`
		ToRef    string `json:"to_owner_ref"`
		Surface  string `json:"writable_surface"`
		Reason   string `json:"reason"`
	}{command.CaseID.String(), command.ExpectedEpoch, command.ToOwnerKind, command.ToOwnerRef.String(), command.WritableSurface, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "request_transfer", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case material: the original conversation drives the coverage
	// re-read; the terms drive the current owner word; the requests with
	// the resolutions drive the live-ask discipline (an EXPIRED
	// unresolved request is voided by the derivation — it never blocks
	// the new ask; the store re-validates under the case lock).
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	terms, requests, resolutions, err := s.repository.ReadOwnershipMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if ask := casesDomain.ActiveTransferRequest(terms, requests, resolutions, command.Now); ask != nil {
		return casesAPI.WorkOutcome{}, rule("request_transfer", fmt.Errorf("%s", casesDomain.PendingTransferText))
	}
	members, err := s.memberIndex(ctx, scope)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current := casesDomain.CurrentOwnership(terms)
	if current == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	// The role word: the current owner or a configuration member.
	if !(current.OwnerKind == casesDomain.OwnerKindHuman && current.OwnerRef == command.Actor.ID) && !rights.Configuration {
		return casesAPI.WorkOutcome{}, rule("request_transfer", fmt.Errorf("%s", casesDomain.TransferRequestScopeText))
	}
	// The target word: an active member or the Staffy employee of the
	// case (AC-1 — a queue or a department is never an owner).
	if command.ToOwnerKind == casesDomain.OwnerKindHuman {
		if _, ok := members[command.ToOwnerRef]; !ok {
			return casesAPI.WorkOutcome{}, rule("request_transfer", fmt.Errorf("владелец-цель должен быть активным участником тенанта (очередь или отдел владельцем быть не могут)"))
		}
		if command.ToOwnerRef == current.OwnerRef && current.OwnerKind == casesDomain.OwnerKindHuman {
			return casesAPI.WorkOutcome{}, rule("request_transfer", fmt.Errorf("цель передачи совпадает с текущим владельцем"))
		}
	} else {
		// The Staffy target: the employee of this case.
		detail, err := s.repository.ReadCasePin(ctx, scope, command.CaseID)
		if err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		if command.ToOwnerRef != detail.EmployeeID {
			return casesAPI.WorkOutcome{}, rule("request_transfer", fmt.Errorf("цифровой сотрудник-цель обязан быть сотрудником этого обращения"))
		}
	}
	// The surface evidence gate (AC-8): the external Inbox needs the
	// acting account-scoped write capability word — a product name is
	// never a proof; in the contour the branch is unreachable (AG-4)
	// and refuses honestly.
	if command.WritableSurface == casesDomain.WritableSurfaceExternalInbox {
		if s.connectionFacts == nil {
			return casesAPI.WorkOutcome{}, rule("request_transfer", fmt.Errorf("%s", casesDomain.SurfaceNotEvidencedText))
		}
		connection, err := s.connectionFacts.ConnectionFactsByID(ctx, scope, conversation.ConnectionID)
		if err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		capabilities := make([]casesDomain.SurfaceCapability, 0, len(connection.Capabilities))
		for _, report := range connection.Capabilities {
			capabilities = append(capabilities, casesDomain.SurfaceCapability{
				Direction: string(report.Direction), Operation: string(report.Operation),
				State: report.State, ExpiresAt: derefTime(report.ExpiresAt), HasExpiry: report.ExpiresAt != nil,
			})
		}
		if !casesDomain.ExternalSurfaceEvidenced(capabilities, command.Now) {
			return casesAPI.WorkOutcome{}, rule("request_transfer", fmt.Errorf("%s", casesDomain.SurfaceNotEvidencedText))
		}
	}
	// The coverage word: the deadline of the human coverage of the
	// acting decision (fail-closed when unreadable).
	coverage, err := s.coverageWord(ctx, scope, conversation)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if coverage.AckDeadlineMinutes < 1 {
		return casesAPI.WorkOutcome{}, rule("request_transfer", fmt.Errorf("%s", casesDomain.CoverageUnavailableText))
	}
	requestID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "request_transfer", TransferRequestID: requestID, Epoch: command.ExpectedEpoch,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "request_transfer", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ExpectedVersion: command.ExpectedEpoch,
		OwnerKind: command.ToOwnerKind, OwnerRef: command.ToOwnerRef, WritableSurface: command.WritableSurface,
		Reason: command.Reason, TransferRequestID: requestID,
		DeadlineAt: command.Now.Add(time.Duration(coverage.AckDeadlineMinutes) * time.Minute).UTC().Truncate(time.Microsecond),
		Outcome:    outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// ResolveTransfer decides one request: the acknowledge is the target-only
// atomic transition (the resolution, the term of the new epoch and the
// outbox event in one transaction); the decline and the cancel keep the
// ownership. The late acknowledge past the deadline gets the honest
// request_expired refusal — the derivation voided the ask.
func (s *Service) ResolveTransfer(ctx context.Context, command casesAPI.ResolveTransferCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("resolve_transfer", casesDomain.GuardResolveTransfer(command.Decision, command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case     string `json:"case"`
		Request  string `json:"request"`
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}{command.CaseID.String(), command.RequestID.String(), command.Decision, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "resolve_transfer", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	terms, requests, resolutions, err := s.repository.ReadOwnershipMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	var request *casesDomain.TransferRequest
	for i := range requests {
		if requests[i].RequestID == command.RequestID {
			request = &requests[i]
			break
		}
	}
	if request == nil || request.CaseID != command.CaseID {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if resolved := resolutionOfTransfer(resolutions, request.RequestID); resolved != nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrCommandKeyConflict
	}
	current := casesDomain.CurrentOwnership(terms)
	// The role words: the acknowledge is target-only; the decline and
	// the cancel — the target or the author.
	switch command.Decision {
	case casesDomain.TransferResolutionAcknowledged:
		if request.ToOwnerKind != casesDomain.OwnerKindHuman || request.ToOwnerRef != command.Actor.ID {
			return casesAPI.WorkOutcome{}, rule("resolve_transfer", fmt.Errorf("%s", casesDomain.OwnerNotTargetText))
		}
		// The late acknowledge: the deadline derivation voided the
		// ask — the honest refusal, never a zombie handoff.
		if !command.Now.Before(request.DeadlineAt) {
			return casesAPI.WorkOutcome{}, rule("resolve_transfer", fmt.Errorf("%s", casesDomain.RequestExpiredText))
		}
	case casesDomain.TransferResolutionDeclined, casesDomain.TransferResolutionCancelled:
		if !(request.ToOwnerKind == casesDomain.OwnerKindHuman && request.ToOwnerRef == command.Actor.ID) && request.RequestedBy != command.Actor.ID {
			return casesAPI.WorkOutcome{}, rule("resolve_transfer", fmt.Errorf("%s", casesDomain.ResolveNotPermittedText))
		}
	}
	resolutionID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	termID := id.UUID("")
	outboxEventID := id.UUID("")
	epoch := int64(0)
	if command.Decision == casesDomain.TransferResolutionAcknowledged {
		if current == nil {
			return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
		}
		epoch = current.Epoch + 1
		if termID, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		if outboxEventID, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	outcome := casesAPI.WorkOutcome{
		Command: "resolve_transfer", TransferRequestID: command.RequestID,
		TransferDecision: command.Decision, Epoch: epoch,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "resolve_transfer", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, TransferRequestID: command.RequestID,
		TransferResolutionID: resolutionID, Decision: command.Decision, Reason: command.Reason,
		OwnerKind: request.ToOwnerKind, OwnerRef: request.ToOwnerRef, WritableSurface: request.WritableSurface,
		VerificationID: termID, OutboxEventID: outboxEventID, ExpectedVersion: epoch,
		Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// ReassignOwnership is the supervision path of a configuration member:
// the direct term of the new epoch with the honest reason; the pending
// request is voided by the cancel resolution of the SAME transaction.
func (s *Service) ReassignOwnership(ctx context.Context, command casesAPI.ReassignOwnershipCommand) (casesAPI.WorkOutcome, error) {
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Configuration {
		return casesAPI.WorkOutcome{}, rule("reassign_ownership", fmt.Errorf("%s", casesDomain.ReassignScopeText))
	}
	if err := rule("reassign_ownership", casesDomain.GuardExpectedEpoch(command.ExpectedEpoch)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("reassign_ownership", casesDomain.GuardRequestTransfer(command.ToOwnerKind, command.WritableSurface, command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case     string `json:"case"`
		Expected int64  `json:"expected_epoch"`
		ToKind   string `json:"to_owner_kind"`
		ToRef    string `json:"to_owner_ref"`
		Surface  string `json:"writable_surface"`
		Reason   string `json:"reason"`
	}{command.CaseID.String(), command.ExpectedEpoch, command.ToOwnerKind, command.ToOwnerRef.String(), command.WritableSurface, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "reassign_ownership", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The target word: an active member or the Staffy employee of the
	// case (the same AC-1 discipline).
	if command.ToOwnerKind == casesDomain.OwnerKindHuman {
		members, err := s.memberIndex(ctx, scope)
		if err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		if _, ok := members[command.ToOwnerRef]; !ok {
			return casesAPI.WorkOutcome{}, rule("reassign_ownership", fmt.Errorf("владелец-цель должен быть активным участником тенанта"))
		}
	} else {
		detail, err := s.repository.ReadCasePin(ctx, scope, command.CaseID)
		if err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		if command.ToOwnerRef != detail.EmployeeID {
			return casesAPI.WorkOutcome{}, rule("reassign_ownership", fmt.Errorf("цифровой сотрудник-цель обязан быть сотрудником этого обращения"))
		}
	}
	termID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	resolutionID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "reassign_ownership", Epoch: command.ExpectedEpoch + 1,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "reassign_ownership", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ExpectedVersion: command.ExpectedEpoch,
		OwnerKind: command.ToOwnerKind, OwnerRef: command.ToOwnerRef, WritableSurface: command.WritableSurface,
		Reason: command.Reason, VerificationID: termID, TransferResolutionID: resolutionID,
		OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// SendClientReply is the ONLY client-bound outgoing path of the slice:
// the actor must be the current human owner, the target conversation must
// belong to the case or be actively bound into it (the governing case),
// and the health word must not be reassignment_required. The record is
// the full send snapshot with the route and the release lineage; the
// content never leaves the reply row.
func (s *Service) SendClientReply(ctx context.Context, command casesAPI.SendClientReplyCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		Content      string `json:"content"`
	}{command.CaseID.String(), command.ConversationID.String(), command.Content})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "send_client_reply", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	if err := rule("send_client_reply", casesDomain.GuardSendClientReply(command.ExpectedEpoch, command.Content)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The governing case of the target conversation: the case it is
	// actively bound INTO (the 2-3 derivation — the binding target), or
	// its own case when no active binding exists. The command's case must
	// BE the governing case — the refusal of the B-owner writing through
	// the bound conversation Y names the owner of the unified dialog
	// (AC-7: the channel change never creates a second owner).
	conversation, err := s.repository.ReadConversationByID(ctx, scope, command.ConversationID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	governing := conversation.CaseID
	bindings, reversals, err := s.repository.ReadBindingsTouchingCase(ctx, scope, conversation.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	for _, binding := range casesDomain.ActiveBindings(bindings, reversals) {
		if binding.BoundConversationID == command.ConversationID {
			governing = binding.CaseID
			break
		}
	}
	if governing != command.CaseID {
		return casesAPI.WorkOutcome{}, s.replyGateRefusal(ctx, scope, governing, casesDomain.GoverningCaseText)
	}
	// The gate: the state words of the governing case + the member
	// word (the owner activity) — the store re-validates under the
	// case lock.
	terms, requests, resolutions, err := s.repository.ReadOwnershipMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	members, err := s.memberIndex(ctx, scope)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current := casesDomain.CurrentOwnership(terms)
	if current == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	ownerActive := ownerActivityWord(current, members)
	state := casesDomain.DeriveOwnershipState(terms, requests, resolutions, ownerActive, command.Now)
	ownerName := ownerDisplayName(current, members)
	if current.OwnerKind == casesDomain.OwnerKindStaffy {
		return casesAPI.WorkOutcome{}, casesDomain.ReplyGateError{
			OwnerKind: current.OwnerKind, OwnerName: ownerName, CurrentEpoch: current.Epoch,
			Reason: casesDomain.StaffyOwnerText,
		}
	}
	if state.ReplyBlocked() {
		return casesAPI.WorkOutcome{}, casesDomain.ReplyGateError{
			OwnerKind: current.OwnerKind, OwnerName: ownerName, CurrentEpoch: current.Epoch,
			Reason: casesDomain.ReassignmentText,
		}
	}
	if current.OwnerRef != command.Actor.ID {
		return casesAPI.WorkOutcome{}, casesDomain.ReplyGateError{
			OwnerKind: current.OwnerKind, OwnerName: ownerName, CurrentEpoch: current.Epoch,
			Reason: casesDomain.NotOwnerText,
		}
	}
	// The CAS expectation of the reply — AFTER the gate words: the
	// non-owner always learns the CURRENT owner first (AC-3/AC-10); the
	// owner with the stale expectation gets the honest version conflict
	// (the store re-validates under the case lock).
	if command.ExpectedEpoch != current.Epoch {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	// The route snapshot and the lineage word of the acting decision.
	conversationOfCase, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	employee, decisionNo, profileVersion, err := s.lineageWord(ctx, scope, conversationOfCase)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	replyID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "send_client_reply", ReplyID: replyID,
		ReplyState: "prepared", Epoch: current.Epoch,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "send_client_reply", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: command.ConversationID,
		OwnerKind: current.OwnerKind, OwnerRef: current.OwnerRef, ExpectedVersion: current.Epoch,
		WritableSurface: current.WritableSurface, ReplyID: replyID, ReplyContent: command.Content,
		OwnerActiveWord:   ownerActive,
		RouteConnectionID: conversation.ConnectionID, RouteChannel: channelOfModality(conversation.Modality),
		RouteDirection: "outbound", LineageEmployeeID: employee, LineageDecisionNo: decisionNo,
		LineageProfileVersion: profileVersion, OutboxEventID: outboxEventID,
		Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// replyGateRefusal resolves the machine-readable gate refusal of a
// governing case: the current owner word with the honest name.
func (s *Service) replyGateRefusal(ctx context.Context, scope tenancy.Context, caseID id.UUID, reason string) error {
	terms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, caseID)
	if err != nil {
		return err
	}
	members, err := s.memberIndex(ctx, scope)
	if err != nil {
		return err
	}
	current := casesDomain.CurrentOwnership(terms)
	if current == nil {
		return casesDomain.ErrNotFound
	}
	return casesDomain.ReplyGateError{
		OwnerKind: current.OwnerKind, OwnerName: ownerDisplayName(current, members),
		CurrentEpoch: current.Epoch, Reason: reason,
	}
}

// ownerDisplayName resolves the honest owner word: the member email or
// the staffy label (never an invented name).
func ownerDisplayName(current *casesDomain.OwnershipTerm, members map[id.UUID]casesPort.OwnerMember) string {
	if current.OwnerKind == casesDomain.OwnerKindStaffy {
		return "цифровой сотрудник"
	}
	if member, ok := members[current.OwnerRef]; ok && member.Name != "" {
		return member.Name
	}
	return current.OwnerRef.String()
}

// resolutionOfTransfer resolves the resolution of one request (nil when
// the request is still open).
func resolutionOfTransfer(resolutions []casesDomain.TransferResolution, requestID id.UUID) *casesDomain.TransferResolution {
	for i := range resolutions {
		if resolutions[i].RequestID == requestID {
			return &resolutions[i]
		}
	}
	return nil
}

// ownershipView assembles the ownership section of the case view: the
// derived state (the current owner with the honest name, the pending
// transfer with the target name, the health, the backup owner and the
// deadline words of the coverage), the term history, the outgoing replies
// with their states and the active member options of the transfer forms.
// The coverage read degrades honestly (an unreadable word stays empty —
// the view never fails on it).
func (s *Service) ownershipView(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail) (casesAPI.OwnershipView, error) {
	terms, requests, resolutions, err := s.repository.ReadOwnershipMaterial(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.OwnershipView{}, err
	}
	replies, err := s.repository.ReadClientReplies(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.OwnershipView{}, err
	}
	members := map[id.UUID]casesPort.OwnerMember{}
	if index, err := s.memberIndex(ctx, scope); err == nil {
		members = index
	}
	now := s.clock.Now()
	current := casesDomain.CurrentOwnership(terms)
	ownerActive := true
	if current != nil && current.OwnerKind == casesDomain.OwnerKindHuman {
		if _, ok := members[current.OwnerRef]; !ok {
			ownerActive = false
		}
	}
	state := casesDomain.DeriveOwnershipState(terms, requests, resolutions, ownerActive, now)
	// The coverage words: the backup owner and the ack deadline of the
	// human coverage of the acting decision (the original conversation of
	// the case). An unreadable coverage degrades to the empty word — the
	// view shows the honest absence, the commands refuse fail-closed.
	if conversation, err := s.caseConnection(ctx, scope, detail.Case.CaseID); err == nil {
		if coverage, err := s.coverageWordRead(ctx, scope, conversation); err == nil {
			state.BackupOwnerID = coverage.BackupOwnerID
			state.AckDeadlineMinutes = coverage.AckDeadlineMinutes
		}
	}
	view := casesAPI.OwnershipView{
		State: state, Terms: terms, Replies: replies,
		Members: make([]casesPort.OwnerMember, 0, len(members)),
	}
	if current != nil {
		view.OwnerName = ownerDisplayName(current, members)
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
	for _, member := range members {
		view.Members = append(view.Members, member)
	}
	return view, nil
}

// coverageWordRead re-reads the human coverage without the command rule
// wrapper (the view degrades honestly, the commands refuse fail-closed).
func (s *Service) coverageWordRead(ctx context.Context, scope tenancy.Context, conversation casesDomain.Conversation) (casesDomain.CoverageWord, error) {
	if s.routeFacts == nil {
		return casesDomain.CoverageWord{}, fmt.Errorf("факт-порт маршрутизации не сконфигурирован")
	}
	routes, err := s.routeFacts.RouteFacts(ctx, scope, conversation.ConnectionID)
	if err != nil {
		return casesDomain.CoverageWord{}, err
	}
	channel := channelOfModality(conversation.Modality)
	for _, candidate := range routes.Candidates {
		for _, coverage := range candidate.Coverage {
			if coverage.Route.Channel == channel && coverage.Route.Direction == "inbound" {
				return casesDomain.CoverageWord{
					Channel: coverage.Route.Channel, Direction: coverage.Route.Direction,
					ResponseOwnerID: coverage.ResponseOwnerID, BackupOwnerID: coverage.BackupOwnerID,
					AckDeadlineMinutes: coverage.AckDeadlineMinutes,
				}, nil
			}
		}
	}
	return casesDomain.CoverageWord{}, fmt.Errorf("human coverage маршрута не найдена")
}

// DrainClientReplies idempotently drives the execute-time revalidation of
// the prepared replies: the fresh ones get the dispatched outcome with
// the outbox record (the synthetic contour delivery — the real channel
// delivery is the deferred outgoing story), the stale ones get
// stale_needs_review (the payload is never relayed to the new owner
// automatically), and the observed attention subjects (the expired
// pending request, the unavailable owner) raise the exception once. The
// material UNIQUE (tenant, reply) keeps the repeats void — nothing is
// lost, nothing doubles.
func (s *Service) DrainClientReplies(ctx context.Context, actor domain.Principal, tenantID id.UUID) (casesAPI.DrainRepliesResult, error) {
	scope, err := s.manageScope(ctx, actor, tenantID)
	if err != nil {
		return casesAPI.DrainRepliesResult{}, err
	}
	pending, err := s.repository.PendingClientReplies(ctx, scope, casesDomain.ClientReplyDrainLimit)
	if err != nil {
		return casesAPI.DrainRepliesResult{}, err
	}
	result := casesAPI.DrainRepliesResult{Considered: len(pending)}
	members, memberErr := s.memberIndex(ctx, scope)
	currentTerms := map[id.UUID]*casesDomain.OwnershipTerm{}
	for _, reply := range pending {
		// The app-side fact words of this reply: the activity of the
		// CURRENT owner of the reply's governing case (the member facts —
		// the word of the current term owner, NOT the reply snapshot: the
		// stale reply of a departed owner still names the CURRENT owner
		// in the attention derivation) and the route admission (the route
		// facts of the reply's connection) — honest snapshots at the
		// drain start; the ownership state itself re-validates under the
		// case lock in the store.
		current := currentTerms[reply.CaseID]
		if current == nil {
			if terms, _, _, materialErr := s.repository.ReadOwnershipMaterial(ctx, scope, reply.CaseID); materialErr == nil {
				current = casesDomain.CurrentOwnership(terms)
				currentTerms[reply.CaseID] = current
			}
		}
		ownerActive := false
		if memberErr == nil && current != nil {
			if current.OwnerKind != casesDomain.OwnerKindHuman {
				ownerActive = true
			} else if _, ok := members[current.OwnerRef]; ok {
				ownerActive = true
			}
		}
		routeAdmitted := false
		if s.routeFacts != nil {
			if routes, err := s.routeFacts.RouteFacts(ctx, scope, reply.ConnectionID); err == nil {
				for _, candidate := range routes.Candidates {
					for _, route := range candidate.Routes {
						if route.Route.Channel == reply.Channel && route.Route.Direction == "inbound" && route.State == "admitted" {
							routeAdmitted = true
						}
					}
				}
			}
		}
		outcomeID, err := s.ids.New(s.clock.Now().UTC())
		if err != nil {
			return casesAPI.DrainRepliesResult{}, err
		}
		outboxEventID, err := s.ids.New(s.clock.Now().UTC())
		if err != nil {
			return casesAPI.DrainRepliesResult{}, err
		}
		exceptionID, err := s.ids.New(s.clock.Now().UTC())
		if err != nil {
			return casesAPI.DrainRepliesResult{}, err
		}
		kind, drained, err := s.repository.ResolveReplyDrain(ctx, casesPort.ReplyDrainParams{
			Scope: scope, Reply: reply, OutcomeID: outcomeID, OutboxEventID: outboxEventID,
			ExceptionID: exceptionID, OwnerActive: ownerActive, RouteAdmitted: routeAdmitted,
			Now: s.clock.Now().UTC(),
		})
		if err != nil {
			return casesAPI.DrainRepliesResult{}, err
		}
		if !drained {
			continue
		}
		if kind == casesDomain.ReplyOutcomeDispatched {
			result.Dispatched++
		} else {
			result.Stale++
		}
	}
	return result, nil
}

// derefTime is the nil-safe expiry word of the capability reports.
func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

// ownerActivityWord resolves the honest activity word of the current
// owner: the staffy owner is never "unavailable" by the member word; the
// human owner must resolve to an active member.
func ownerActivityWord(current *casesDomain.OwnershipTerm, members map[id.UUID]casesPort.OwnerMember) bool {
	if current == nil || current.OwnerKind != casesDomain.OwnerKindHuman {
		return true
	}
	_, ok := members[current.OwnerRef]
	return ok
}
