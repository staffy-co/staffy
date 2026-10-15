package main

import (
        "context"
        "fmt"
        "sort"
        "time"

        casesAPI "github.com/artemiosu/staffy/internal/cases/api"
        casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.13 fixture: the six handoff lifecycle commands over the REAL
// domain guards (GuardRequestHuman — the turn anchor, the CAS expectation,
// the target roles subset, the deadline word; GuardAssignHandoff — the
// open state, the assignee role against the request's target roles, the
// backup and the deadline; GuardHandoffAcknowledge / GuardHandoffDecline —
// the actor IS the assignee of the open assigned round; GuardCancelHuman-
// Request — the open lifecycle and the anchor turn; GuardReturnToStaffy —
// the acknowledged state and the result word) and the REAL read-time
// derivations (DeriveHandoffState — the last event decides, the overdue is
// the honest time derivation; DeriveHandoffClientStatus — «назначен ≠
// принял» are two words). The fixture world stores the append-only request
// rows and the append-only lifecycle events; the states and the client
// statuses are never stored — every read derives them anew (the mirror of
// the store). The request and the event numbers are the fixture-owned
// monotonic truths (the requests never carry them). The mode raise of the
// ask is the conditional ratchet term through the REAL HandoffModeTerm (a
// stricter mode is never touched — the ask is still recorded); the
// acknowledge and the return compose the 2.4 transfer machinery of the
// fixture world in the SAME flow (the request, the resolution and the term
// — the exec-composition mirror of the store transaction). The request
// hashes cover the WHOLE command materials INCLUDING the turn anchors, the
// assignee, the backup and the result word: the exact repeats answer the
// stored outcomes (the replays), the same keys with other materials are
// the honest conflicts.

// fixtureHandoffID generates the next deterministic handoff material id
// (the counter space of the Story 2.13 surface — the ids never collide
// with the other fixture surfaces).
func (f *casesFixture) fixtureHandoffID() id.UUID {
        f.handoffSeq++
        return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(60000+f.handoffSeq)))
}

// fixtureHandoffTerminalText mirrors the store refusal of the terminal
// case (the honest boundary of the write commands).
const fixtureHandoffTerminalText = "Терминальное обращение не принимает команд жизненного цикла передачи: обслуживание завершено."

// fixtureHandoffMemberScopes is the membership scope truth of the fixture
// world (the role resolution input — the app handoffRoleWord mirror: the
// owner carries review AND configuration, the second operator carries
// review only).
var fixtureHandoffMemberScopes = map[id.UUID]struct {
        review        bool
        configuration bool
}{
        principalID:      {review: true, configuration: true},
        secondOperatorID: {review: true, configuration: false},
}

// fixtureHandoffRoleWord resolves the assignee's target-role word from
// the membership truth (the app handoffRoleWord mirror — the operator
// NEVER inputs the role): the review scope carries the operator word, the
// configuration scope the supervision word; the deterministic dictionary
// order (operator first) prefers the word the request addresses. A
// non-member is the honest refusal; a member whose scopes carry no word
// of the request's target roles is the honest refusal (the request
// decides who it addresses).
func (f *casesFixture) fixtureHandoffRoleWord(targetRoles []string, assignee id.UUID) (string, error) {
        scopes, member := fixtureHandoffMemberScopes[assignee]
        if !member {
                return "", fmt.Errorf("назначаемый %s не активный участник тенента: назначение возможно только участникам словаря ролей", assignee.String())
        }
        for _, role := range []struct {
                word      string
                supported bool
        }{
                {casesDomain.HandoffTargetRoleOperator, scopes.review},
                {casesDomain.HandoffTargetRoleSupervision, scopes.configuration},
        } {
                if !role.supported || !containsWord(targetRoles, role.word) {
                        continue
                }
                return role.word, nil
        }
        return "", fmt.Errorf("роль назначаемого не входит в целевые роли запроса (%s): полномочия участника не несут слово целевых ролей", quotedWords(targetRoles))
}

// containsWord reports whether the word belongs to the list.
func containsWord(words []string, word string) bool {
        for _, candidate := range words {
                if candidate == word {
                        return true
                }
        }
        return false
}

// quotedWords renders the honest «word» list of the refusals.
func quotedWords(words []string) string {
        result := ""
        for i, word := range words {
                if i > 0 {
                        result += ", "
                }
                result += "«" + word + "»"
        }
        return result
}

// fixtureHandoffState derives the read-time lifecycle view of every
// request of the case over the REAL domain function.
func (f *casesFixture) fixtureHandoffStates(caseID id.UUID, now time.Time) []casesDomain.HandoffStateView {
        requests := f.handoffs[caseID]
        events := f.handoffEvents[caseID]
        views := make([]casesDomain.HandoffStateView, 0, len(requests))
        for _, request := range requests {
                views = append(views, casesDomain.DeriveHandoffState(request, events, now))
        }
        return views
}

// fixtureHandoffOpen resolves the OPEN lifecycle view of the case (the
// one-competing-task truth; nil when no open lifecycle lives).
func (f *casesFixture) fixtureHandoffOpen(caseID id.UUID, now time.Time) *casesDomain.HandoffStateView {
        views := f.fixtureHandoffStates(caseID, now)
        for i := range views {
                if views[i].Active() {
                        return &views[i]
                }
        }
        return nil
}

// fixtureHandoffLast resolves the LAST lifecycle view of the case (the
// maximal request number — the target of the closed-lifecycle guards).
func (f *casesFixture) fixtureHandoffLast(caseID id.UUID, now time.Time) *casesDomain.HandoffStateView {
        views := f.fixtureHandoffStates(caseID, now)
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

// fixtureHandoffActive reports whether the case carries an OPEN lifecycle
// (the freeze truth of the fixture commitment gate).
func (f *casesFixture) fixtureHandoffActive(caseID id.UUID) bool {
        return f.fixtureHandoffOpen(caseID, timeNow()) != nil
}

// fixtureHandoffEventNo derives the next monotonic event number of one
// request (the fixture-owned truth under the world lock).
func (f *casesFixture) fixtureHandoffEventNo(caseID id.UUID, requestNo int64) int64 {
        eventNo := int64(0)
        for _, event := range f.handoffEvents[caseID] {
                if event.RequestNo == requestNo && event.EventNo > eventNo {
                        eventNo = event.EventNo
                }
        }
        return eventNo + 1
}

// fixtureHandoffTerminal guards the terminal boundary of the case.
func (f *casesFixture) fixtureHandoffTerminal(commandCase id.UUID) error {
        material := f.material[commandCase]
        if material == nil {
                return casesDomain.ErrNotFound
        }
        if material.cases.State != casesDomain.CaseStateOpen {
                return fixtureWorkRule("handoff", fmt.Errorf("%s", fixtureHandoffTerminalText))
        }
        return nil
}

// RequestHuman records the client's unconditional ask (AC-1): the REAL
// dictionary guard before any write (the turn anchor, the CAS
// expectation, the target roles subset, the deadline word — the domain
// truth, never a convenient default), the idempotent command flow with
// the hash of the WHOLE material, the honest refusal of an unknown or a
// terminal case, the turn anchor guard (the accepted client event of
// THIS case), the mode CAS of the ladder, the one-competing-task gate
// (the open lifecycle refuses the second ask with the CURRENT words) and
// the single write: the conditional ratchet raise through the REAL
// HandoffModeTerm (a stricter mode is never touched — the ask is still
// recorded) plus the append-only request row with the owner snapshot of
// the server truth. The request number is the fixture-owned monotonic
// truth (the request never carries it).
func (f *casesFixture) RequestHuman(_ context.Context, command casesAPI.RequestHumanCommand) (casesAPI.WorkOutcome, error) {
        f.lock()
        defer f.unlock()
        if command.TenantID != f.tenant {
                return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
        }
        if err := fixtureWorkRule("request_human", casesDomain.GuardRequestHuman(command.TurnRef, command.ExpectedModeVersion, command.TargetRoles, command.AckDeadline)); err != nil {
                return casesAPI.WorkOutcome{}, err
        }
        // The request hash covers the WHOLE material INCLUDING the turn
        // anchor, the CAS expectation, the target roles and the deadline word.
        outcome, fresh, err := f.workCommandFlow("request_human", command.IdempotencyKey, struct {
                Case        string   `json:"case"`
                Turn        string   `json:"turn_ref"`
                Expected    int64    `json:"expected_mode_version"`
                TargetRoles []string `json:"target_roles"`
                AckDeadline string   `json:"ack_deadline"`
        }{command.CaseID.String(), command.TurnRef.String(), command.ExpectedModeVersion, command.TargetRoles, command.AckDeadline})
        if err != nil || !fresh {
                return outcome, err
        }
        if err := f.fixtureHandoffTerminal(command.CaseID); err != nil {
                return casesAPI.WorkOutcome{}, err
        }
        // The turn anchor guard: the anchor is the ACCEPTED client event of
        // THIS case (the anchor of the unconditional ask).
        if !f.fixtureEventOfCase(command.CaseID, command.TurnRef) {
                return casesAPI.WorkOutcome{}, fixtureWorkRule("request_human", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
        }
        // The mode CAS: the raced twin of the ladder gets the honest conflict
        // with the current mode words.
        terms := f.modeTerms[command.CaseID]
        active := casesDomain.ActiveConversationMode(terms)
        if current := casesDomain.ModeTermMaxVersion(terms); current != command.ExpectedModeVersion {
                return casesAPI.WorkOutcome{}, casesDomain.ModeConflictError{
                        ActiveMode: casesDomain.EffectiveWorkModeWord(active), Reason: activeReasonOf(active),
                        SignalSource: activeSignalOf(active), RuleVersion: casesDomain.ModePolicyVersion, ModeVersion: current,
                }
        }
        // The one-competing-task gate: the open lifecycle refuses the second
        // ask with the CURRENT lifecycle words (AC-4).
        if open := f.fixtureHandoffOpen(command.CaseID, command.Now); open != nil {
                return casesAPI.WorkOutcome{}, casesDomain.HandoffActiveError{
                        State:        open.State,
                        ClientStatus: casesDomain.DeriveHandoffClientStatus(open.State),
                }
        }
        // The conditional ratchet raise through the REAL domain term (the
        // conversation of the case — the ladder anchor).
        conversation := casesDomain.Conversation{}
        if material := f.material[command.CaseID]; material != nil && len(material.conversations) > 0 {
                conversation = material.conversations[0]
        }
        outcome = casesAPI.WorkOutcome{
                Command: "request_human", HandoffTurnRef: command.TurnRef,
                HandoffAckDeadline: command.AckDeadline,
        }
        if term, ok := casesDomain.HandoffModeTerm(active, f.tenant, command.CaseID, conversation.ConversationID, command.TurnRef, command.Actor.ID, command.Now); ok {
                term.TermID = f.fixtureHandoffID()
                f.modeTerms[command.CaseID] = append(f.modeTerms[command.CaseID], *term)
                outcome.ModeTermID = term.TermID
                outcome.ModeVersion = term.ModeVersion
        }
        // The append-only request row: the NEXT monotonic request number (the
        // fixture-owned truth), the owner snapshot of the server truth.
        requestNo := int64(0)
        for _, request := range f.handoffs[command.CaseID] {
                if request.RequestNo > requestNo {
                        requestNo = request.RequestNo
                }
        }
        requestNo++
        ownerKind, ownerRef, ownerEpoch := casesDomain.OwnerKindHuman, f.owner, int64(1)
        if state := f.fixtureState(command.CaseID, command.Now); state.Current != nil {
                ownerKind, ownerRef, ownerEpoch = state.Current.OwnerKind, state.Current.OwnerRef, state.Current.Epoch
        }
        f.handoffs[command.CaseID] = append(f.handoffs[command.CaseID], casesDomain.HandoffRequest{
                TenantID: f.tenant, CaseID: command.CaseID, RequestNo: requestNo,
                Reason: casesDomain.HandoffReasonClientRequest,
                OwnerKind: ownerKind, OwnerRef: ownerRef, OwnerEpoch: ownerEpoch,
                TargetRoles: append([]string(nil), command.TargetRoles...),
                AckDeadline: command.AckDeadline,
                AckDeadlineAt: command.Now.Add(casesDomain.HandoffAckDeadlineWindow(command.AckDeadline)),
                TurnRef:      command.TurnRef, ActorID: command.Actor.ID, CreatedAt: command.Now,
        })
        f.storeOutcome("request_human", command.IdempotencyKey, outcome)
        return outcome, nil
}

// AssignHumanHandoff records one assignment round of the open lifecycle
// (AC-3): the idempotent command flow with the hash of the assignment
// material, the honest refusals, the lifecycle target (the OPEN lifecycle
// — or the LAST closed one, so the guard refuses with the honest state
// word), the assignee's role word resolved from the membership truth (the
// operator never inputs the role) and the REAL domain guard of the round
// material. The assignment NEVER changes the owner (назначение ≠
// принятие); the re-assignment is a NEW event with a NEW deadline — the
// human re-decision. The event number is the fixture-owned monotonic
// truth.
func (f *casesFixture) AssignHumanHandoff(_ context.Context, command casesAPI.AssignHumanHandoffCommand) (casesAPI.WorkOutcome, error) {
        f.lock()
        defer f.unlock()
        if command.TenantID != f.tenant {
                return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
        }
        // The request hash covers the assignment material INCLUDING the
        // assignee and the backup references with the deadline word.
        outcome, fresh, err := f.workCommandFlow("assign_human_handoff", command.IdempotencyKey, struct {
                Case        string `json:"case"`
                Assignee    string `json:"assignee"`
                Backup      string `json:"backup"`
                AckDeadline string `json:"ack_deadline"`
        }{command.CaseID.String(), command.Assignee.String(), command.Backup.String(), command.AckDeadline})
        if err != nil || !fresh {
                return outcome, err
        }
        if err := f.fixtureHandoffTerminal(command.CaseID); err != nil {
                return casesAPI.WorkOutcome{}, err
        }
        // The lifecycle target: the OPEN lifecycle, or the LAST closed one
        // (the guard then refuses with the honest state word).
        target := f.fixtureHandoffOpen(command.CaseID, command.Now)
        if target == nil {
                target = f.fixtureHandoffLast(command.CaseID, command.Now)
        }
        if target == nil {
                return casesAPI.WorkOutcome{}, fixtureWorkRule("assign_human_handoff", fmt.Errorf("%s", casesDomain.HandoffAbsentText))
        }
        // The assignee's role word: the SERVER-side resolution of the
        // membership truth (the operator never inputs the role).
        role, err := f.fixtureHandoffRoleWord(target.Request.TargetRoles, command.Assignee)
        if err != nil {
                return casesAPI.WorkOutcome{}, fixtureWorkRule("assign_human_handoff", err)
        }
        // The backup membership check: the round's reserve is an active member
        // of the fixture world (the app handoffBackupMember mirror — the
        // honest 400 of a foreign reference).
        if _, member := fixtureHandoffMemberScopes[command.Backup]; !member {
                return casesAPI.WorkOutcome{}, fixtureWorkRule("assign_human_handoff", fmt.Errorf(
                        "резервный участник %s не активный участник тенента: раунд назначается только участникам", command.Backup.String()))
        }
        // The REAL domain guard of the round material (the honest 400 before
        // the write; the table CHECK words are the store backstops).
        if err := fixtureWorkRule("assign_human_handoff", casesDomain.GuardAssignHandoff(target.State, command.Assignee, role, target.Request.TargetRoles, command.Backup, command.AckDeadline)); err != nil {
                return casesAPI.WorkOutcome{}, err
        }
        // The append-only assigned event: the NEXT monotonic event number of
        // the request with the round's own deadline material.
        eventNo := f.fixtureHandoffEventNo(command.CaseID, target.Request.RequestNo)
        f.handoffEvents[command.CaseID] = append(f.handoffEvents[command.CaseID], casesDomain.HandoffEvent{
                TenantID: f.tenant, CaseID: command.CaseID,
                RequestNo: target.Request.RequestNo, EventNo: eventNo,
                Kind:        casesDomain.HandoffEventAssigned,
                Assignee:    command.Assignee, BackupOwner: command.Backup,
                AckDeadline: command.AckDeadline,
                DeadlineAt:  command.Now.Add(casesDomain.HandoffAckDeadlineWindow(command.AckDeadline)),
                ActorID:     command.Actor.ID, CreatedAt: command.Now,
        })
        outcome = casesAPI.WorkOutcome{
                Command: "assign_human_handoff", HandoffAssignee: command.Assignee,
                HandoffBackup: command.Backup, HandoffAckDeadline: command.AckDeadline,
        }
        f.storeOutcome("assign_human_handoff", command.IdempotencyKey, outcome)
        return outcome, nil
}

// AcknowledgeHumanHandoff writes the assignee's confirmed acceptance
// (AC-5): the idempotent command flow with the CAS hash, the lifecycle
// target, the REAL domain guard (the actor IS the assignee — the server
// truth; the assignment or the notification is never an acknowledgement),
// the ownership CAS against the derived state and the composed write of
// the 2.4 transfer machinery in the SAME flow: the acknowledged event
// with the resulting epoch, the transfer request with the honest reason
// constant, the acknowledged resolution and the ownership term (the epoch
// +1, the assignee owner, the inbox surface) — the exec-composition
// mirror of the store transaction.
func (f *casesFixture) AcknowledgeHumanHandoff(_ context.Context, command casesAPI.AcknowledgeHumanHandoffCommand) (casesAPI.WorkOutcome, error) {
        f.lock()
        defer f.unlock()
        if command.TenantID != f.tenant {
                return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
        }
        // The request hash covers the CAS expectation of the ownership epoch.
        outcome, fresh, err := f.workCommandFlow("acknowledge_human_handoff", command.IdempotencyKey, struct {
                Case     string `json:"case"`
                Expected int64  `json:"expected_epoch"`
        }{command.CaseID.String(), command.ExpectedEpoch})
        if err != nil || !fresh {
                return outcome, err
        }
        if err := f.fixtureHandoffTerminal(command.CaseID); err != nil {
                return casesAPI.WorkOutcome{}, err
        }
        // The lifecycle target: the OPEN round, or the LAST closed one.
        target := f.fixtureHandoffOpen(command.CaseID, command.Now)
        if target == nil {
                target = f.fixtureHandoffLast(command.CaseID, command.Now)
        }
        if target == nil {
                return casesAPI.WorkOutcome{}, fixtureWorkRule("acknowledge_human_handoff", fmt.Errorf("%s", casesDomain.HandoffAbsentText))
        }
        // The REAL domain guard: the state is the open assigned round, the
        // actor IS the assignee, the CAS expectation is present.
        if err := fixtureWorkRule("acknowledge_human_handoff", casesDomain.GuardHandoffAcknowledge(target.State, command.Actor.ID, target.Assignee, command.ExpectedEpoch)); err != nil {
                return casesAPI.WorkOutcome{}, err
        }
        // The ownership CAS against the derived state (the raced twin of the
        // two-operator race gets the honest stale signal).
        state := f.fixtureState(command.CaseID, command.Now)
        if state.Current == nil {
                return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
        }
        if state.Current.Epoch != command.ExpectedEpoch {
                return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
        }
        // The composed write (the ONE flow of the store transaction): the
        // acknowledged event, the transfer request with the reason constant,
        // the acknowledged resolution and the ownership term (the epoch +1,
        // the assignee owner, the inbox surface, the origin of the confirmed
        // transfer).
        eventNo := f.fixtureHandoffEventNo(command.CaseID, target.Request.RequestNo)
        f.handoffEvents[command.CaseID] = append(f.handoffEvents[command.CaseID], casesDomain.HandoffEvent{
                TenantID: f.tenant, CaseID: command.CaseID,
                RequestNo: target.Request.RequestNo, EventNo: eventNo,
                Kind: casesDomain.HandoffEventAcknowledged,
                ResultingEpoch: state.Current.Epoch + 1,
                ActorID:        command.Actor.ID, CreatedAt: command.Now,
        })
        requestID := f.fixtureOwnershipID()
        requests := f.transferRequests[command.CaseID]
        f.transferRequests[command.CaseID] = append(requests, casesDomain.TransferRequest{
                TenantID: f.tenant, CaseID: command.CaseID, RequestID: requestID, RequestNo: int64(len(requests) + 1),
                FromOwnerKind: state.Current.OwnerKind, FromOwnerRef: state.Current.OwnerRef,
                ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: command.Actor.ID,
                WritableSurface: casesDomain.WritableSurfaceInbox,
                Reason: casesDomain.HandoffReasonText(casesDomain.HandoffReasonClientRequest),
                RequestedBy: command.Actor.ID,
                DeadlineAt:  command.Now.Add(casesDomain.HandoffAckDeadlineWindow(target.Request.AckDeadline)),
                CreatedAt:   command.Now,
        })
        resolutionID := f.fixtureOwnershipID()
        f.transferResolutions = append(f.transferResolutions, casesDomain.TransferResolution{
                TenantID: f.tenant, ResolutionID: resolutionID, RequestID: requestID,
                Kind: casesDomain.TransferResolutionAcknowledged, ActorID: command.Actor.ID,
                ResultingEpoch: state.Current.Epoch + 1,
                Reason:         casesDomain.HandoffReasonText(casesDomain.HandoffReasonClientRequest),
                CreatedAt:      command.Now,
        })
        termID := f.fixtureOwnershipID()
        f.fixtureAppendTerm(command.CaseID, casesDomain.OwnershipTerm{
                TenantID: f.tenant, CaseID: command.CaseID, TermID: termID,
                Epoch: state.Current.Epoch + 1, OwnerKind: casesDomain.OwnerKindHuman, OwnerRef: command.Actor.ID,
                WritableSurface: casesDomain.WritableSurfaceInbox, Origin: casesDomain.OwnershipOriginTransferAcknowledge,
                PreviousOwnerKind: state.Current.OwnerKind, PreviousOwnerRef: state.Current.OwnerRef,
                ActorID: command.Actor.ID, TransferRequestID: requestID,
                RequestedAt: command.Now, AcknowledgedAt: command.Now, CreatedAt: command.Now,
        })
        outcome = casesAPI.WorkOutcome{
                Command: "acknowledge_human_handoff", Epoch: state.Current.Epoch + 1,
                TransferRequestID: requestID, HandoffAssignee: command.Actor.ID,
        }
        f.storeOutcome("acknowledge_human_handoff", command.IdempotencyKey, outcome)
        return outcome, nil
}

// DeclineHumanHandoff writes the assignee's honest refusal of the
// assigned round (AC-6): the idempotent command flow (the refusal
// material is the case anchor only — the actor IS the assignee of the
// round), the lifecycle target and the REAL domain guard. The decline
// NEVER changes the owner (the prior safe owner stays); the request stays
// open for the human re-assignment.
func (f *casesFixture) DeclineHumanHandoff(_ context.Context, command casesAPI.DeclineHumanHandoffCommand) (casesAPI.WorkOutcome, error) {
        f.lock()
        defer f.unlock()
        if command.TenantID != f.tenant {
                return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
        }
        // The request hash: the refusal material is the case anchor only (the
        // actor IS the assignee of the round — the server truth).
        outcome, fresh, err := f.workCommandFlow("decline_human_handoff", command.IdempotencyKey, struct {
                Case string `json:"case"`
        }{command.CaseID.String()})
        if err != nil || !fresh {
                return outcome, err
        }
        if err := f.fixtureHandoffTerminal(command.CaseID); err != nil {
                return casesAPI.WorkOutcome{}, err
        }
        target := f.fixtureHandoffOpen(command.CaseID, command.Now)
        if target == nil {
                target = f.fixtureHandoffLast(command.CaseID, command.Now)
        }
        if target == nil {
                return casesAPI.WorkOutcome{}, fixtureWorkRule("decline_human_handoff", fmt.Errorf("%s", casesDomain.HandoffAbsentText))
        }
        if err := fixtureWorkRule("decline_human_handoff", casesDomain.GuardHandoffDecline(target.State, command.Actor.ID, target.Assignee)); err != nil {
                return casesAPI.WorkOutcome{}, err
        }
        eventNo := f.fixtureHandoffEventNo(command.CaseID, target.Request.RequestNo)
        f.handoffEvents[command.CaseID] = append(f.handoffEvents[command.CaseID], casesDomain.HandoffEvent{
                TenantID: f.tenant, CaseID: command.CaseID,
                RequestNo: target.Request.RequestNo, EventNo: eventNo,
                Kind: casesDomain.HandoffEventDeclined,
                ActorID: command.Actor.ID, CreatedAt: command.Now,
        })
        outcome = casesAPI.WorkOutcome{Command: "decline_human_handoff", HandoffAssignee: command.Actor.ID}
        f.storeOutcome("decline_human_handoff", command.IdempotencyKey, outcome)
        return outcome, nil
}

// CancelHumanRequest writes the client's cancellation of the open
// lifecycle (AC-10): the idempotent command flow with the anchor turn
// hash, the honest refusals, the turn anchor guard of the cancelling
// reply, the lifecycle target and the REAL domain guard. The owner NEVER
// changes and the history is never deleted; the mode stays (the ratchet —
// the exit is the authoritative exit_mode).
func (f *casesFixture) CancelHumanRequest(_ context.Context, command casesAPI.CancelHumanRequestCommand) (casesAPI.WorkOutcome, error) {
        f.lock()
        defer f.unlock()
        if command.TenantID != f.tenant {
                return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
        }
        // The request hash covers the WHOLE material INCLUDING the anchor
        // turn of the cancelling reply.
        outcome, fresh, err := f.workCommandFlow("cancel_human_request", command.IdempotencyKey, struct {
                Case string `json:"case"`
                Turn string `json:"turn_ref"`
        }{command.CaseID.String(), command.TurnRef.String()})
        if err != nil || !fresh {
                return outcome, err
        }
        if err := f.fixtureHandoffTerminal(command.CaseID); err != nil {
                return casesAPI.WorkOutcome{}, err
        }
        // The turn anchor guard: the anchor is the ACCEPTED client event of
        // THIS case.
        if !f.fixtureEventOfCase(command.CaseID, command.TurnRef) {
                return casesAPI.WorkOutcome{}, fixtureWorkRule("cancel_human_request", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
        }
        target := f.fixtureHandoffOpen(command.CaseID, command.Now)
        if target == nil {
                target = f.fixtureHandoffLast(command.CaseID, command.Now)
        }
        if target == nil {
                return casesAPI.WorkOutcome{}, fixtureWorkRule("cancel_human_request", fmt.Errorf("%s", casesDomain.HandoffAbsentText))
        }
        if err := fixtureWorkRule("cancel_human_request", casesDomain.GuardCancelHumanRequest(target.State, command.TurnRef)); err != nil {
                return casesAPI.WorkOutcome{}, err
        }
        eventNo := f.fixtureHandoffEventNo(command.CaseID, target.Request.RequestNo)
        f.handoffEvents[command.CaseID] = append(f.handoffEvents[command.CaseID], casesDomain.HandoffEvent{
                TenantID: f.tenant, CaseID: command.CaseID,
                RequestNo: target.Request.RequestNo, EventNo: eventNo,
                Kind: casesDomain.HandoffEventCancelled,
                TurnRef:  command.TurnRef,
                ActorID:  command.Actor.ID, CreatedAt: command.Now,
        })
        outcome = casesAPI.WorkOutcome{Command: "cancel_human_request", HandoffTurnRef: command.TurnRef}
        f.storeOutcome("cancel_human_request", command.IdempotencyKey, outcome)
        return outcome, nil
}

// ReturnToStaffy writes the human owner's explicit return of the
// acknowledged lifecycle (AC-9): the idempotent command flow with the
// result word and the CAS hash, the LAST lifecycle (the return answers
// the acknowledged acceptance), the REAL domain guard, the ownership CAS
// with the human-owner check and the composed write of the 2.4 transfer
// machinery in the SAME flow: the returned event with the result word,
// the transfer request to Staffy (the external_inbox surface, the honest
// reason constant of the result word), the acknowledged resolution and
// the ownership term (the owner = staffy, the epoch +1). The mode is
// NEVER touched (the ladder keeps its honest word — the sales path never
// resumes under the stricter mode).
func (f *casesFixture) ReturnToStaffy(_ context.Context, command casesAPI.ReturnToStaffyCommand) (casesAPI.WorkOutcome, error) {
        f.lock()
        defer f.unlock()
        if command.TenantID != f.tenant {
                return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
        }
        // The request hash covers the result word and the CAS expectation.
        outcome, fresh, err := f.workCommandFlow("return_to_staffy", command.IdempotencyKey, struct {
                Case     string `json:"case"`
                Result   string `json:"result"`
                Expected int64  `json:"expected_epoch"`
        }{command.CaseID.String(), command.Result, command.ExpectedEpoch})
        if err != nil || !fresh {
                return outcome, err
        }
        if err := f.fixtureHandoffTerminal(command.CaseID); err != nil {
                return casesAPI.WorkOutcome{}, err
        }
        // The LAST lifecycle (the return answers the acknowledged acceptance).
        last := f.fixtureHandoffLast(command.CaseID, command.Now)
        if last == nil {
                return casesAPI.WorkOutcome{}, fixtureWorkRule("return_to_staffy", fmt.Errorf("%s", casesDomain.HandoffAbsentText))
        }
        if err := fixtureWorkRule("return_to_staffy", casesDomain.GuardReturnToStaffy(last.State, command.Result)); err != nil {
                return casesAPI.WorkOutcome{}, err
        }
        // The ownership CAS with the human-owner check (the return answers
        // the confirmed human owner).
        state := f.fixtureState(command.CaseID, command.Now)
        if state.Current == nil {
                return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
        }
        if state.Current.Epoch != command.ExpectedEpoch {
                return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
        }
        if state.Current.OwnerKind != casesDomain.OwnerKindHuman {
                return casesAPI.WorkOutcome{}, fixtureWorkRule("return_to_staffy", fmt.Errorf("%s", casesDomain.StaffyOwnerText))
        }
        // The composed write (the ONE flow of the store transaction): the
        // returned event, the transfer request to Staffy with the result-word
        // reason constant, the acknowledged resolution and the ownership term
        // (the owner = staffy, the external_inbox surface, the epoch +1).
        eventNo := f.fixtureHandoffEventNo(command.CaseID, last.Request.RequestNo)
        f.handoffEvents[command.CaseID] = append(f.handoffEvents[command.CaseID], casesDomain.HandoffEvent{
                TenantID: f.tenant, CaseID: command.CaseID,
                RequestNo: last.Request.RequestNo, EventNo: eventNo,
                Kind: casesDomain.HandoffEventReturned, Result: command.Result,
                ActorID: command.Actor.ID, CreatedAt: command.Now,
        })
        requestID := f.fixtureOwnershipID()
        requests := f.transferRequests[command.CaseID]
        f.transferRequests[command.CaseID] = append(requests, casesDomain.TransferRequest{
                TenantID: f.tenant, CaseID: command.CaseID, RequestID: requestID, RequestNo: int64(len(requests) + 1),
                FromOwnerKind: state.Current.OwnerKind, FromOwnerRef: state.Current.OwnerRef,
                ToOwnerKind: casesDomain.OwnerKindStaffy, ToOwnerRef: f.employee,
                WritableSurface: casesDomain.WritableSurfaceExternalInbox,
                Reason:          casesDomain.HandoffResultText(command.Result),
                RequestedBy:     command.Actor.ID,
                DeadlineAt:      command.Now.Add(casesDomain.HandoffAckDeadlineWindow(last.Request.AckDeadline)),
                CreatedAt:       command.Now,
        })
        resolutionID := f.fixtureOwnershipID()
        f.transferResolutions = append(f.transferResolutions, casesDomain.TransferResolution{
                TenantID: f.tenant, ResolutionID: resolutionID, RequestID: requestID,
                Kind: casesDomain.TransferResolutionAcknowledged, ActorID: command.Actor.ID,
                ResultingEpoch: state.Current.Epoch + 1,
                Reason:         casesDomain.HandoffResultText(command.Result),
                CreatedAt:      command.Now,
        })
        termID := f.fixtureOwnershipID()
        f.fixtureAppendTerm(command.CaseID, casesDomain.OwnershipTerm{
                TenantID: f.tenant, CaseID: command.CaseID, TermID: termID,
                Epoch: state.Current.Epoch + 1, OwnerKind: casesDomain.OwnerKindStaffy, OwnerRef: f.employee,
                WritableSurface: casesDomain.WritableSurfaceExternalInbox, Origin: casesDomain.OwnershipOriginTransferAcknowledge,
                PreviousOwnerKind: state.Current.OwnerKind, PreviousOwnerRef: state.Current.OwnerRef,
                ActorID: command.Actor.ID, TransferRequestID: requestID,
                RequestedAt: command.Now, AcknowledgedAt: command.Now, CreatedAt: command.Now,
        })
        outcome = casesAPI.WorkOutcome{
                Command: "return_to_staffy", Epoch: state.Current.Epoch + 1,
                TransferRequestID: requestID, HandoffResult: command.Result,
        }
        f.storeOutcome("return_to_staffy", command.IdempotencyKey, outcome)
        return outcome, nil
}

// fixtureHandoffView assembles the «Запрос человека» section of the case
// view (the app handoffView mirror): every append-only request row with
// its derived lifecycle (the state word, the honest text, the client
// status word «назначение ≠ принятие», the current round material), the
// event history ordered by the monotonic numbers, the dictionaries of the
// forms (the fixed compile-time words), the honest client status of the
// LAST request and the card flag of the acting principal (the assignee of
// the open assigned round — the card «Передача ждёт вашего подтверждения»
// is the assignee's surface).
func (f *casesFixture) fixtureHandoffView(caseID id.UUID, actor id.UUID) casesAPI.HandoffView {
        now := timeNow()
        requests := append([]casesDomain.HandoffRequest(nil), f.handoffs[caseID]...)
        sort.Slice(requests, func(i, j int) bool { return requests[i].RequestNo < requests[j].RequestNo })
        events := append([]casesDomain.HandoffEvent(nil), f.handoffEvents[caseID]...)
        sort.Slice(events, func(i, j int) bool {
                if events[i].RequestNo != events[j].RequestNo {
                        return events[i].RequestNo < events[j].RequestNo
                }
                return events[i].EventNo < events[j].EventNo
        })
        view := casesAPI.HandoffView{
                Requests:      make([]casesAPI.HandoffRequestView, 0, len(requests)),
                RequestsCount: len(requests),
                RoleWords:     append([]string(nil), casesDomain.HandoffTargetRoles...),
                DeadlineWords: append([]string(nil), casesDomain.HandoffAckDeadlines...),
                ResultWords:   append([]string(nil), casesDomain.HandoffResultWords...),
                PanelText:     "Запрос человека — запись жизненного цикла обращения, не факт и не доказательство: append-only строки запроса и событий; состояние и клиентский статус — производные чтения (просрочка — деривация дедлайна раунда); назначение ≠ принятие: владелец меняется только подтверждением специалиста либо возвратом.",
        }
        for _, request := range requests {
                state := casesDomain.DeriveHandoffState(request, events, now)
                clientStatus := casesDomain.DeriveHandoffClientStatus(state.State)
                requestView := casesAPI.HandoffRequestView{
                        RequestNo:        request.RequestNo,
                        Reason:           request.Reason,
                        ReasonText:       casesDomain.HandoffReasonText(request.Reason),
                        State:            state.State,
                        StateText:        casesDomain.HandoffStateText(state.State),
                        ClientStatus:     clientStatus,
                        ClientStatusText: casesDomain.HandoffClientStatusText(clientStatus),
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
                        Events:           make([]casesAPI.HandoffEventView, 0, len(events)),
                }
                for _, event := range events {
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
                        view.RoundAssignee = state.Assignee
                        view.RoundDeadlineAt = state.DeadlineAt
                }
                view.Requests = append(view.Requests, requestView)
        }
        // The honest client status of the LAST request (the interface truth
        // of CaseView — «назначен ≠ принял» are two words; no lifecycle, no
        // word: the honest absence, never an invented default).
        if last := f.fixtureHandoffLast(caseID, now); last != nil {
                clientStatus := casesDomain.DeriveHandoffClientStatus(last.State)
                view.ClientStatus = clientStatus
                view.ClientStatusText = casesDomain.HandoffClientStatusText(clientStatus)
                // The card of the assignee: the acting principal IS the assignee of
                // the open assigned round (the assignment or the notification is
                // never an acknowledgement — AC-3/AC-5).
                if last.State == casesDomain.HandoffStateAssigned && actor == last.Assignee {
                        view.AcknowledgeForActor = true
                }
        }
        return view
}

// activeReasonOf resolves the honest reason word of the active mode term
// (the nil ladder carries no reason — the unclassified floor).
func activeReasonOf(active *casesDomain.ModeTerm) string {
        if active != nil {
                return active.Reason
        }
        return ""
}

// activeSignalOf resolves the honest signal source word of the active
// mode term (the nil ladder carries no signal).
func activeSignalOf(active *casesDomain.ModeTerm) string {
        if active != nil {
                return active.SignalSource
        }
        return ""
}
