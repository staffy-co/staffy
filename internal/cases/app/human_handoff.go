package app

import (
	"context"
	"encoding/json"
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
// The Story 2.13 commands: the unconditional human request and the
// lifecycle Handoff (the six commands, the gates and the panel view)
// ---------------------------------------------------------------------------
//
// All six commands are idempotent by the tenant-scoped key with the
// stored outcome (the 2.2..2.12 precedent: the replay answer comes
// BEFORE the state guards); the request hash covers the WHOLE command
// material INCLUDING the turn anchor, the assignee, the backup and the
// result word (the same key with another material is the honest key
// conflict). The rights gate (review OR configuration) fires BEFORE
// every material guard and refuses with the forbidden word (the honest
// 403 of the scope sentence); the dictionary guards refuse with the
// honest 400 (the domain truth before SQL); the terminal case refuses
// honestly; the store re-validates every guard under the case +
// conversation advisory locks of the sorted lock plan (the 2-13 lock
// rows of work.go — the serialization with the ownership, mode and
// commitment writers).

// handoffScopeText is the honest scope refusal of the six lifecycle
// commands (the shared «review или configuration» vocabulary of the
// 2.5..2.12 rights gates): the app word travels inside the forbidden
// error only — the transport renders its own sentence for the wire and
// the form.
const handoffScopeText = "Команды запроса человека и жизненного цикла передачи требуют полномочия review или configuration: участники только с audit не управляют передачей обращения."

// The compile-time proof of the narrow interface registration: the
// composite Service carries the six handoff commands (the transport
// type-asserts casesAPI.HandoffService — the DegradationService
// precedent; the test fakes of the other stories stay untouched).
var _ casesAPI.HandoffService = (*Service)(nil)

// casesStoreTerminalHandoffText mirrors the store refusal of the
// terminal case (the honest boundary of the write commands — the v1
// dictionary carries the open word only).
const casesStoreTerminalHandoffText = "Терминальное обращение не принимает команд жизненного цикла передачи: обслуживание завершено."

// handoffMaterialOf reads the whole lifecycle material of one case (the
// pre-transaction honest read; the store re-derives the same truth under
// the locks of every command).
func (s *Service) handoffMaterialOf(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesPort.HandoffMaterial, error) {
	return s.repository.ReadHandoffMaterial(ctx, scope, caseID)
}

// handoffViewsOf derives the read-time lifecycle state of every request
// of the case (the derivation happens on the material — never stored).
func handoffViewsOf(material casesPort.HandoffMaterial, now time.Time) []casesDomain.HandoffStateView {
	views := make([]casesDomain.HandoffStateView, 0, len(material.Requests))
	for _, request := range material.Requests {
		views = append(views, casesDomain.DeriveHandoffState(request, material.Events, now))
	}
	return views
}

// handoffOpenOf resolves the OPEN lifecycle view of the case (the
// one-competing-task truth of the repeated ask; nil when no open
// lifecycle lives).
func handoffOpenOf(material casesPort.HandoffMaterial, now time.Time) *casesDomain.HandoffStateView {
	views := handoffViewsOf(material, now)
	for i := range views {
		if views[i].Active() {
			return &views[i]
		}
	}
	return nil
}

// handoffLastOf resolves the LAST lifecycle view of the case (the
// maximal request number — the target of the closed-lifecycle guards:
// the raced twin of a closed lifecycle learns the honest state word of
// what already happened; nil when no lifecycle exists).
func handoffLastOf(material casesPort.HandoffMaterial, now time.Time) *casesDomain.HandoffStateView {
	views := handoffViewsOf(material, now)
	if len(views) == 0 {
		return nil
	}
	last := &views[0]
	for i := range views {
		if views[i].Request.RequestNo >= last.Request.RequestNo {
			last = &views[i]
		}
	}
	return last
}

// handoffRoleWord resolves the honest target-role word of the assignee
// from the membership truth (the server-side resolution — the operator
// NEVER inputs the role; the store re-validates the word against the
// request's target roles under the locks): the review scope carries the
// operator word, the configuration scope the supervision word. The
// resolution prefers the word the request addresses (the deterministic
// dictionary order — operator first); a member without any scope of the
// dictionary, a non-member or an inactive membership is the honest 400
// (the assignment addresses the members of the fixed dictionary).
func (s *Service) handoffRoleWord(ctx context.Context, scope tenancy.Context, targetRoles []string, assignee id.UUID) (string, error) {
	if s.members == nil {
		return "", fmt.Errorf("факт-порт участников не сконфигурирован: назначение честно отклоняется")
	}
	memberships, err := s.members.MembersFacts(ctx, domain.Principal{ID: scope.PrincipalID()}, scope.TenantID())
	if err != nil {
		return "", err
	}
	var membership *domain.Membership
	for i := range memberships {
		candidate := &memberships[i]
		if candidate.PrincipalID != nil && *candidate.PrincipalID == assignee && candidate.Status == "active" {
			membership = candidate
			break
		}
	}
	if membership == nil {
		return "", fmt.Errorf("назначаемый %s не активный участник тенента: назначение возможно только участникам словаря ролей", assignee.String())
	}
	review := false
	configuration := false
	for _, word := range membership.Scopes {
		if word == domain.Review {
			review = true
		}
		if word == domain.Configuration {
			configuration = true
		}
	}
	// The deterministic dictionary order (operator first): the resolved
	// word must belong to the fixed dictionary AND the request's target
	// roles — the membership truth carries the word, the request decides
	// who it addresses.
	for _, role := range []struct {
		word      string
		supported bool
	}{
		{casesDomain.HandoffTargetRoleOperator, review},
		{casesDomain.HandoffTargetRoleSupervision, configuration},
	} {
		if !role.supported || !inWords(targetRoles, role.word) {
			continue
		}
		return role.word, nil
	}
	return "", fmt.Errorf("роль назначаемого не входит в целевые роли запроса (%s): полномочия участника не несут слово целевых ролей", joinQuotedOf(targetRoles))
}

// handoffBackupMember proves the backup of the assignment round is an
// ACTIVE member of the tenant (the round's reserve — the same membership
// truth as the assignee; the honest 400 of a foreign or an inactive
// reference: the reserve is never a convenient id, AC-3).
func (s *Service) handoffBackupMember(ctx context.Context, scope tenancy.Context, backup id.UUID) error {
	if s.members == nil {
		return fmt.Errorf("факт-порт участников не сконфигурирован: назначение честно отклоняется")
	}
	memberships, err := s.members.MembersFacts(ctx, domain.Principal{ID: scope.PrincipalID()}, scope.TenantID())
	if err != nil {
		return err
	}
	for i := range memberships {
		candidate := &memberships[i]
		if candidate.PrincipalID != nil && *candidate.PrincipalID == backup && candidate.Status == "active" {
			return nil
		}
	}
	return fmt.Errorf("резервный участник %s не активный участник тенента: раунд назначается только участникам", backup.String())
}

// inWords reports whether the word belongs to the list (the honest
// dictionary membership of the target roles).
func inWords(words []string, word string) bool {
	for _, candidate := range words {
		if candidate == word {
			return true
		}
	}
	return false
}

// joinQuotedOf renders the honest dictionary list of the refusal.
func joinQuotedOf(words []string) string {
	quoted := make([]string, 0, len(words))
	for _, word := range words {
		quoted = append(quoted, "«"+word+"»")
	}
	return joinStrings(quoted)
}

// joinStrings joins the quoted words with the honest separator.
func joinStrings(words []string) string {
	result := ""
	for i, word := range words {
		if i > 0 {
			result += ", "
		}
		result += word
	}
	return result
}

// handoffIDs resolves the fresh identifiers of one lifecycle command
// (the number of the ids the command material needs; the same Now of the
// command — the deterministic generator discipline).
func (s *Service) handoffIDs(now time.Time, count int) ([]id.UUID, error) {
	ids := make([]id.UUID, 0, count)
	for i := 0; i < count; i++ {
		generated, err := s.ids.New(now)
		if err != nil {
			return nil, err
		}
		ids = append(ids, generated)
	}
	return ids, nil
}

// RequestHuman records the client's unconditional ask (AC-1): the
// rights gate FIRST (review or configuration — an audit-only member
// never writes the lifecycle), the dictionary guard of the ask material
// (the turn anchor, the CAS expectation, the target roles, the deadline
// word), the replay lookup with the whole-material hash, the case anchor
// (the open case), the conversation of the lock plan, the turn guard
// (the accepted client event of THIS case), the mode CAS (the raced twin
// of the ladder gets the honest 409 with the current mode words), the
// one-competing-task gate (the open lifecycle refuses the second ask
// with the honest words of what already lives — AC-4) and the single
// transactional write: the conditional ratchet raise through the
// EXISTING 2.5 machinery (a stricter mode is never touched — the ask is
// still recorded) plus the append-only request row with the
// human_requested envelope (the identifiers and the dictionary words
// only — the system reason text NEVER enters).
func (s *Service) RequestHuman(ctx context.Context, command casesAPI.RequestHumanCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration (the M1 precedent).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, handoffScopeText)
	}
	// The dictionary guard of the ask material (the honest 400 with the
	// dictionary words in the text — the domain truth before SQL; the
	// table CHECK words are the backstops).
	if err := rule("request_human", casesDomain.GuardRequestHuman(command.TurnRef, command.ExpectedModeVersion, command.TargetRoles, command.AckDeadline)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the WHOLE command material INCLUDING the
	// turn anchor, the CAS expectation, the target roles and the deadline
	// word (the replay of the exact repeat returns the stored outcome;
	// the same key with another material is the honest key conflict).
	hash := requestHash(struct {
		Case        string   `json:"case"`
		Turn        string   `json:"turn_ref"`
		Expected    int64    `json:"expected_mode_version"`
		TargetRoles []string `json:"target_roles"`
		AckDeadline string   `json:"ack_deadline"`
	}{command.CaseID.String(), command.TurnRef.String(), command.ExpectedModeVersion, command.TargetRoles, command.AckDeadline})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "request_human", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case anchor: the case must exist in the tenant scope and stay
	// open (the store re-validates under the locks).
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, rule("request_human", fmt.Errorf("%s", casesStoreTerminalHandoffText))
	}
	// The conversation of the lock plan: the case's own conversation (the
	// SAME sorted plan of the mode/policy/handoff commands — the mode CAS
	// and the lifecycle write serialize on the identical key set).
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The turn guard: the anchor is the ACCEPTED client event of THIS
	// case (the store guard re-validates under the locks; the honest 400
	// before SQL of the ask of a foreign turn).
	if _, err := s.repository.ReadEventOfCase(ctx, scope, command.CaseID, command.TurnRef); err != nil {
		return casesAPI.WorkOutcome{}, rule("request_human", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
	}
	// The mode CAS: the raced twin of the ladder gets the honest 409 with
	// the current mode words (the store re-validates the same order under
	// the lock); the conditional raise below decides the ratchet
	// direction.
	terms, err := s.repository.ReadModeMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	active := casesDomain.ActiveConversationMode(terms)
	if current := casesDomain.ModeTermMaxVersion(terms); current != command.ExpectedModeVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ModeConflictError{
			ActiveMode: casesDomain.EffectiveWorkModeWord(active), Reason: activeReason(active),
			SignalSource: activeSignal(active), RuleVersion: casesDomain.ModePolicyVersion, ModeVersion: current,
		}
	}
	// The one-competing-task gate: an open lifecycle refuses the second
	// ask with the honest words of what already lives (AC-4; the store
	// re-validates under the locks).
	material, err := s.handoffMaterialOf(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if open := handoffOpenOf(material, command.Now); open != nil {
		return casesAPI.WorkOutcome{}, casesDomain.HandoffActiveError{
			State:        open.State,
			ClientStatus: casesDomain.DeriveHandoffClientStatus(open.State),
		}
	}
	// The identifiers of the conditional machinery: the mode term id (the
	// ratchet writes it ONLY when the raise is honest — the store decides
	// under the locks), the mode_changed envelope id and the lifecycle
	// envelope id.
	ids, err := s.handoffIDs(command.Now, 3)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	// The conditional ratchet raise of the outcome word: the app derives
	// the honest prediction (a stricter or equal active mode leaves the
	// ladder untouched — the ask is still recorded; the store re-derives
	// the same decision under the locks).
	outcome := casesAPI.WorkOutcome{
		Command: "request_human", HandoffTurnRef: command.TurnRef,
		HandoffAckDeadline: command.AckDeadline,
	}
	if term, ok := casesDomain.HandoffModeTerm(active, scope.TenantID(), command.CaseID, conversation.ConversationID, command.TurnRef, command.Actor.ID, command.Now); ok {
		outcome.ModeTermID = ids[0]
		outcome.ModeVersion = term.ModeVersion
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "request_human", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: conversation.ConversationID,
		TurnRef: command.TurnRef, ExpectedModeVersion: command.ExpectedModeVersion,
		ModeTermID: ids[0], OutboxEventID: ids[1],
		HandoffTargetRoles: command.TargetRoles, HandoffAckDeadline: command.AckDeadline,
		HandoffOutboxEventID: ids[2], CorrelationID: correlation,
		Outcome: outcomeJSON,
		Now:     command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// AssignHumanHandoff records one assignment round (AC-3): the rights
// gate, the replay lookup, the case anchor, the conversation of the lock
// plan, the lifecycle target (the OPEN lifecycle — or the LAST closed
// one, so the guard refuses with the honest state word of what already
// happened), the assignee's role word resolved from the membership truth
// (the operator never inputs the role), the backup membership check and
// the domain guard of the round material (the state, the role against
// the request's target roles, the deadline word). The assignment NEVER
// changes the owner (назначение ≠ принятие); the re-assignment is a NEW
// event with a NEW deadline — the human re-decision.
func (s *Service) AssignHumanHandoff(ctx context.Context, command casesAPI.AssignHumanHandoffCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration.
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, handoffScopeText)
	}
	// The request hash covers the assignment material INCLUDING the
	// assignee and the backup references with the deadline word.
	hash := requestHash(struct {
		Case        string `json:"case"`
		Assignee    string `json:"assignee"`
		Backup      string `json:"backup"`
		AckDeadline string `json:"ack_deadline"`
	}{command.CaseID.String(), command.Assignee.String(), command.Backup.String(), command.AckDeadline})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "assign_human_handoff", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case anchor: the case must exist and stay open.
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, rule("assign_human_handoff", fmt.Errorf("%s", casesStoreTerminalHandoffText))
	}
	// The conversation of the lock plan.
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The lifecycle target: the OPEN lifecycle, or the LAST closed one
	// (the guard then refuses with the honest state word — the raced twin
	// of a closed lifecycle learns what already happened).
	material, err := s.handoffMaterialOf(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	target := handoffOpenOf(material, command.Now)
	if target == nil {
		target = handoffLastOf(material, command.Now)
	}
	if target == nil {
		return casesAPI.WorkOutcome{}, rule("assign_human_handoff", fmt.Errorf("%s", casesDomain.HandoffAbsentText))
	}
	// The assignee's role word: the SERVER-side resolution of the
	// membership truth (the store re-validates the word against the
	// request's target roles under the locks — the membership truth never
	// travels as an operator input).
	role, err := s.handoffRoleWord(ctx, scope, target.Request.TargetRoles, command.Assignee)
	if err != nil {
		return casesAPI.WorkOutcome{}, rule("assign_human_handoff", err)
	}
	// The backup membership check: the round's reserve is an active member
	// of the tenant (the same membership truth as the assignee — the
	// honest 400 of a foreign reference; the spec row of the matrix: the
	// backup is a member, never a convenient reference).
	if err := s.handoffBackupMember(ctx, scope, command.Backup); err != nil {
		return casesAPI.WorkOutcome{}, rule("assign_human_handoff", err)
	}
	// The domain guard of the round material (the honest 400 before SQL;
	// the table CHECK words are the backstops).
	if err := rule("assign_human_handoff", casesDomain.GuardAssignHandoff(target.State, command.Assignee, role, target.Request.TargetRoles, command.Backup, command.AckDeadline)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	ids, err := s.handoffIDs(command.Now, 1)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	outcome := casesAPI.WorkOutcome{
		Command: "assign_human_handoff", HandoffAssignee: command.Assignee,
		HandoffBackup: command.Backup, HandoffAckDeadline: command.AckDeadline,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "assign_human_handoff", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: conversation.ConversationID,
		HandoffAssignee: command.Assignee, HandoffBackup: command.Backup,
		HandoffAssigneeRole: role, HandoffAckDeadline: command.AckDeadline,
		OutboxEventID: ids[0], CorrelationID: correlation,
		Outcome: outcomeJSON,
		Now:     command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// AcknowledgeHumanHandoff writes the assignee's confirmed acceptance
// (AC-5): the rights gate, the replay lookup, the case anchor, the
// conversation, the lifecycle target (the OPEN round — or the LAST
// closed one, so the guard refuses with the honest state word of the
// two-operator race), the domain guard (the actor IS the assignee, the
// state is the open assigned round, the CAS expectation is present) and
// the ownership CAS against the read material (the raced twin loses on
// the locks first and on the UNIQUE + CAS backstops second). The
// acceptance is the ONE transition that moves the ownership — the
// exec-composition of the 2.4 transfer machinery in the SAME transaction
// (the store call, never a copy).
func (s *Service) AcknowledgeHumanHandoff(ctx context.Context, command casesAPI.AcknowledgeHumanHandoffCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration.
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, handoffScopeText)
	}
	// The request hash covers the CAS expectation of the ownership epoch.
	hash := requestHash(struct {
		Case     string `json:"case"`
		Expected int64  `json:"expected_epoch"`
	}{command.CaseID.String(), command.ExpectedEpoch})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "acknowledge_human_handoff", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case anchor: the case must exist and stay open.
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, rule("acknowledge_human_handoff", fmt.Errorf("%s", casesStoreTerminalHandoffText))
	}
	// The conversation of the lock plan.
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The lifecycle target: the OPEN round, or the LAST closed one.
	material, err := s.handoffMaterialOf(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	target := handoffOpenOf(material, command.Now)
	if target == nil {
		target = handoffLastOf(material, command.Now)
	}
	if target == nil {
		return casesAPI.WorkOutcome{}, rule("acknowledge_human_handoff", fmt.Errorf("%s", casesDomain.HandoffAbsentText))
	}
	// The domain guard: the state is the open assigned round, the actor
	// IS the assignee (the server truth — the assignment or the
	// notification is never an acknowledgement), the CAS expectation is
	// present.
	if err := rule("acknowledge_human_handoff", casesDomain.GuardHandoffAcknowledge(target.State, command.Actor.ID, target.Assignee, command.ExpectedEpoch)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The ownership CAS against the read material (the store re-validates
	// under the locks; the raced twin of the two-operator race gets the
	// honest stale signal).
	ownershipTerms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current := casesDomain.CurrentOwnership(ownershipTerms)
	if current == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Epoch != command.ExpectedEpoch {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	// The identifiers of the composed transaction: the transfer request,
	// the resolution, the ownership term, the ownership_changed envelope
	// and the lifecycle envelope.
	ids, err := s.handoffIDs(command.Now, 5)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	outcome := casesAPI.WorkOutcome{
		Command: "acknowledge_human_handoff", Epoch: current.Epoch + 1,
		TransferRequestID: ids[0], HandoffAssignee: command.Actor.ID,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "acknowledge_human_handoff", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: conversation.ConversationID,
		ExpectedVersion: command.ExpectedEpoch, TransferRequestID: ids[0], TransferResolutionID: ids[1],
		VerificationID: ids[2], OutboxEventID: ids[3], HandoffOutboxEventID: ids[4],
		CorrelationID: correlation, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// DeclineHumanHandoff writes the assignee's honest refusal of the
// assigned round (AC-6): the rights gate, the replay lookup, the case
// anchor, the conversation, the lifecycle target and the domain guard
// (the state is the open assigned round, the actor IS the assignee). The
// decline NEVER changes the owner (the prior safe owner stays); the
// request stays open for the human re-assignment.
func (s *Service) DeclineHumanHandoff(ctx context.Context, command casesAPI.DeclineHumanHandoffCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration.
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, handoffScopeText)
	}
	// The request hash: the refusal material is the case anchor only (the
	// actor IS the assignee of the round — the server truth).
	hash := requestHash(struct {
		Case string `json:"case"`
	}{command.CaseID.String()})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "decline_human_handoff", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case anchor: the case must exist and stay open.
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, rule("decline_human_handoff", fmt.Errorf("%s", casesStoreTerminalHandoffText))
	}
	// The conversation of the lock plan.
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The lifecycle target and the domain guard.
	material, err := s.handoffMaterialOf(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	target := handoffOpenOf(material, command.Now)
	if target == nil {
		target = handoffLastOf(material, command.Now)
	}
	if target == nil {
		return casesAPI.WorkOutcome{}, rule("decline_human_handoff", fmt.Errorf("%s", casesDomain.HandoffAbsentText))
	}
	if err := rule("decline_human_handoff", casesDomain.GuardHandoffDecline(target.State, command.Actor.ID, target.Assignee)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	ids, err := s.handoffIDs(command.Now, 1)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	outcome := casesAPI.WorkOutcome{Command: "decline_human_handoff", HandoffAssignee: command.Actor.ID}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "decline_human_handoff", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: conversation.ConversationID,
		OutboxEventID: ids[0], CorrelationID: correlation, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// CancelHumanRequest writes the client's cancellation of the open
// lifecycle (AC-10): the rights gate, the replay lookup, the case
// anchor, the conversation, the lifecycle target, the domain guard (the
// open state, the anchor turn) and the turn guard of the cancelling
// reply. The owner NEVER changes and the history is never deleted; the
// mode stays (the 2.5 ratchet — the exit is the authoritative
// exit_mode).
func (s *Service) CancelHumanRequest(ctx context.Context, command casesAPI.CancelHumanRequestCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration.
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, handoffScopeText)
	}
	// The request hash covers the WHOLE material INCLUDING the anchor
	// turn of the cancelling reply.
	hash := requestHash(struct {
		Case string `json:"case"`
		Turn string `json:"turn_ref"`
	}{command.CaseID.String(), command.TurnRef.String()})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "cancel_human_request", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case anchor: the case must exist and stay open.
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, rule("cancel_human_request", fmt.Errorf("%s", casesStoreTerminalHandoffText))
	}
	// The conversation of the lock plan.
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The turn guard: the anchor is the ACCEPTED client event of THIS
	// case (the store guard re-validates under the locks).
	if _, err := s.repository.ReadEventOfCase(ctx, scope, command.CaseID, command.TurnRef); err != nil {
		return casesAPI.WorkOutcome{}, rule("cancel_human_request", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
	}
	// The lifecycle target and the domain guard.
	material, err := s.handoffMaterialOf(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	target := handoffOpenOf(material, command.Now)
	if target == nil {
		target = handoffLastOf(material, command.Now)
	}
	if target == nil {
		return casesAPI.WorkOutcome{}, rule("cancel_human_request", fmt.Errorf("%s", casesDomain.HandoffAbsentText))
	}
	if err := rule("cancel_human_request", casesDomain.GuardCancelHumanRequest(target.State, command.TurnRef)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	ids, err := s.handoffIDs(command.Now, 1)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	outcome := casesAPI.WorkOutcome{Command: "cancel_human_request", HandoffTurnRef: command.TurnRef}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "cancel_human_request", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: conversation.ConversationID,
		TurnRef: command.TurnRef, OutboxEventID: ids[0], CorrelationID: correlation, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// ReturnToStaffy writes the human owner's explicit return of the
// acknowledged lifecycle (AC-9): the rights gate, the replay lookup, the
// case anchor, the conversation, the LAST lifecycle (the return answers
// the acknowledged acceptance), the domain guard (the state word, the
// result word of the fixed dictionary) and the ownership CAS with the
// human-owner check. The return composes the 2.4 transfer machinery in
// the SAME transaction (the owner=staffy, the external_inbox surface,
// the epoch+1); the mode is NEVER touched (the ladder keeps its honest
// word).
func (s *Service) ReturnToStaffy(ctx context.Context, command casesAPI.ReturnToStaffyCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration.
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, handoffScopeText)
	}
	// The request hash covers the result word and the CAS expectation.
	hash := requestHash(struct {
		Case     string `json:"case"`
		Result   string `json:"result"`
		Expected int64  `json:"expected_epoch"`
	}{command.CaseID.String(), command.Result, command.ExpectedEpoch})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "return_to_staffy", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case anchor: the case must exist and stay open.
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, rule("return_to_staffy", fmt.Errorf("%s", casesStoreTerminalHandoffText))
	}
	// The conversation of the lock plan.
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The LAST lifecycle (the return answers the acknowledged acceptance).
	material, err := s.handoffMaterialOf(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	last := handoffLastOf(material, command.Now)
	if last == nil {
		return casesAPI.WorkOutcome{}, rule("return_to_staffy", fmt.Errorf("%s", casesDomain.HandoffAbsentText))
	}
	if err := rule("return_to_staffy", casesDomain.GuardReturnToStaffy(last.State, command.Result)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The ownership CAS with the human-owner check (the store
	// re-validates under the locks).
	ownershipTerms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current := casesDomain.CurrentOwnership(ownershipTerms)
	if current == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Epoch != command.ExpectedEpoch {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if current.OwnerKind != casesDomain.OwnerKindHuman {
		return casesAPI.WorkOutcome{}, rule("return_to_staffy", fmt.Errorf("%s", casesDomain.StaffyOwnerText))
	}
	// The identifiers of the composed transaction: the transfer request,
	// the resolution, the ownership term, the ownership_changed envelope
	// and the lifecycle envelope.
	ids, err := s.handoffIDs(command.Now, 5)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	outcome := casesAPI.WorkOutcome{
		Command: "return_to_staffy", Epoch: current.Epoch + 1,
		TransferRequestID: ids[0], HandoffResult: command.Result,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "return_to_staffy", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: conversation.ConversationID,
		ExpectedVersion: command.ExpectedEpoch, TransferRequestID: ids[0], TransferResolutionID: ids[1],
		VerificationID: ids[2], OutboxEventID: ids[3], HandoffOutboxEventID: ids[4],
		HandoffResult: command.Result, CorrelationID: correlation, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// handoffView assembles the «Запрос человека» section of the case view:
// every append-only request row with its derived lifecycle (the state
// word, the honest text, the client status word, the current round
// material), the event history ordered by the monotonic numbers, the
// dictionaries of the forms (the fixed compile-time words) and the
// card flag of the acting principal (the assignee of the open assigned
// round — the card «Передача ждёт вашего подтверждения» is the
// assignee's surface). The reads degrade honestly — the same view scope.
func (s *Service) handoffView(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail, actor id.UUID) (casesAPI.HandoffView, error) {
	material, err := s.handoffMaterialOf(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.HandoffView{}, err
	}
	now := s.clock.Now()
	view := casesAPI.HandoffView{
		Requests:      make([]casesAPI.HandoffRequestView, 0, len(material.Requests)),
		RequestsCount: len(material.Requests),
		RoleWords:     append([]string(nil), casesDomain.HandoffTargetRoles...),
		DeadlineWords: append([]string(nil), casesDomain.HandoffAckDeadlines...),
		ResultWords:   append([]string(nil), casesDomain.HandoffResultWords...),
		PanelText:     "Запрос человека — запись жизненного цикла обращения, не факт и не доказательство: append-only строки запроса и событий; состояние и клиентский статус — производные чтения (просрочка — деривация дедлайна раунда); назначение ≠ принятие: владелец меняется только подтверждением специалиста либо возвратом.",
	}
	for _, request := range material.Requests {
		state := casesDomain.DeriveHandoffState(request, material.Events, now)
		requestView := casesAPI.HandoffRequestView{
			RequestNo:        request.RequestNo,
			Reason:           request.Reason,
			ReasonText:       casesDomain.HandoffReasonText(request.Reason),
			State:            state.State,
			StateText:        casesDomain.HandoffStateText(state.State),
			ClientStatus:     casesDomain.DeriveHandoffClientStatus(state.State),
			ClientStatusText: casesDomain.HandoffClientStatusText(casesDomain.DeriveHandoffClientStatus(state.State)),
			Active:           state.Active(),
			OwnerKind:        request.OwnerKind,
			OwnerRef:         request.OwnerRef,
			TargetRoles:      append([]string(nil), request.TargetRoles...),
			AckDeadline:      request.AckDeadline,
			AckDeadlineText:  casesDomain.HandoffAckDeadlineText(request.AckDeadline),
			TurnRef:          request.TurnRef,
			ActorID:          request.ActorID,
			CreatedAt:        request.CreatedAt,
			Assignee:         state.Assignee,
			BackupOwner:      state.BackupOwner,
			DeadlineAt:       state.DeadlineAt,
			Events:           make([]casesAPI.HandoffEventView, 0, len(material.Events)),
		}
		for _, event := range material.Events {
			if event.CaseID != request.CaseID || event.RequestNo != request.RequestNo {
				continue
			}
			requestView.Events = append(requestView.Events, casesAPI.HandoffEventView{
				RequestNo: event.RequestNo, EventNo: event.EventNo,
				Kind: event.Kind, KindText: casesAPI.HandoffEventKindText(event.Kind),
				Assignee: event.Assignee, BackupOwner: event.BackupOwner,
				AckDeadline: event.AckDeadline, DeadlineAt: event.DeadlineAt,
				Result: event.Result, ResultText: casesDomain.HandoffResultText(event.Result),
				TurnRef: event.TurnRef, ResultingEpoch: event.ResultingEpoch,
				ActorID: event.ActorID, CreatedAt: event.CreatedAt,
			})
		}
		if state.Active() {
			// The open round of the card: the assignee and the deadline of
			// the LATEST assigned event (the round the closing events
			// answer).
			view.RoundAssignee = state.Assignee
			view.RoundDeadlineAt = state.DeadlineAt
		}
		view.Requests = append(view.Requests, requestView)
	}
	// The honest client status of the LAST request (the interface truth
	// of CaseView — «назначен ≠ принял» are two words; no lifecycle, no
	// word: the honest absence, never an invented default).
	if last := handoffLastOf(material, now); last != nil {
		clientStatus := casesDomain.DeriveHandoffClientStatus(last.State)
		view.ClientStatus = clientStatus
		view.ClientStatusText = casesDomain.HandoffClientStatusText(clientStatus)
		// The card of the assignee: the acting principal IS the assignee
		// of the open assigned round (the assignment or the notification
		// is never an acknowledgement — the card carries the confirm and
		// the decline forms of the round).
		if last.State == casesDomain.HandoffStateAssigned && actor == last.Assignee {
			view.AcknowledgeForActor = true
		}
	}
	return view, nil
}
