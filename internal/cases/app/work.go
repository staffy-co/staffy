package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// The ten work commands (Story 2.2)
// ---------------------------------------------------------------------------
//
// Every command is idempotent by the tenant-scoped key with the stored
// outcome (the connector 1-9 precedent: the replay answer comes BEFORE the
// domain guards); every anchor mutation carries expected_version — the
// store re-validates it under the locks (optimistic concurrency); every
// write is append-only.

// commandScope authorizes the work commands: the review scope (the privacy
// class of the commands). A member without it gets an honest refusal, a
// non-member a plain 404.
func (s *Service) commandScope(ctx context.Context, actor domain.Principal, tenantID id.UUID) (tenancy.Context, error) {
	if !actor.ID.Valid() || !tenantID.Valid() {
		return tenancy.Context{}, domain.ErrNotFound
	}
	scope, err := tenancy.New(actor.ID, tenantID)
	if err != nil {
		return tenancy.Context{}, err
	}
	if err := s.identity.Authorize(ctx, actor, tenantID, domain.Review); err != nil {
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

// rule wraps a domain guard failure into the honest WorkRuleError of the
// command surface.
func rule(command string, err error) error {
	if err == nil {
		return nil
	}
	return casesDomain.WorkRuleError{Rule: command, Reason: err.Error()}
}

// requestHash digests the command material: the same key with another
// material is an honest key conflict, never a silent second execution.
func requestHash(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		// The command material is always marshalable (structs of strings and
		// numbers); a failure here is a genuine programming error.
		panic(fmt.Sprintf("запрос команды не кодируется: %v", err))
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// replayOutcome decodes the stored outcome of an idempotent redelivery.
func replayOutcome(stored []byte) (casesAPI.WorkOutcome, error) {
	var outcome casesAPI.WorkOutcome
	if err := json.Unmarshal(stored, &outcome); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome.Replay = true
	return outcome, nil
}

// lookupCommand answers the idempotency question BEFORE the guards: an
// exact repeat replays the stored outcome, the same key with another
// material is an honest conflict.
func (s *Service) lookupCommand(ctx context.Context, scope tenancy.Context, command, key, hash string) (casesAPI.WorkOutcome, bool, error) {
	lookup, err := s.repository.LookupCommand(ctx, scope, command, key)
	if err != nil {
		return casesAPI.WorkOutcome{}, false, err
	}
	if !lookup.Found {
		return casesAPI.WorkOutcome{}, false, nil
	}
	if lookup.RequestHash != hash {
		return casesAPI.WorkOutcome{}, false, casesDomain.ErrCommandKeyConflict
	}
	outcome, err := replayOutcome(lookup.Outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, false, err
	}
	return outcome, true, nil
}

// taxonomyOf resolves the journeys dictionary of the case's pinned pack.
// A NULL pin is the honest pre-migration refusal ("taxonomy неизвестна");
// an unpinnable pack is an honest error, never an assumed dictionary.
func (s *Service) taxonomyOf(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]string, error) {
	if s.taxonomy == nil {
		return nil, casesDomain.WorkRuleError{Rule: "taxonomy", Reason: "Факт-порт taxonomy пака не сконфигурирован: команды классификации честно отклоняются, словарь «ok» не предполагается."}
	}
	caseRow, err := s.repository.ReadCasePin(ctx, scope, caseID)
	if err != nil {
		return nil, err
	}
	if caseRow.PackID == "" || caseRow.PackVersion == "" {
		return nil, casesDomain.WorkRuleError{Rule: "taxonomy", Reason: "Пак не закреплён на обращении: taxonomy неизвестна (до-миграционные данные; новых таких обращений не бывает)."}
	}
	journeys, err := s.taxonomy.PackJourneys(caseRow.PackID, caseRow.PackVersion)
	if err != nil {
		return nil, casesDomain.WorkRuleError{Rule: "taxonomy", Reason: err.Error()}
	}
	return journeys, nil
}

// requireTypeInTaxonomy validates the work item type against the pinned
// journeys dictionary — the type of an open case never depends on the
// current admission (the pack pin is sealed by the decision that admitted
// the contact).
func requireTypeInTaxonomy(journeys []string, workType string) error {
	for _, journey := range journeys {
		if journey == workType {
			return nil
		}
	}
	return fmt.Errorf("тип «%s» не входит в journeys закреплённого пака обращения (%s)", workType, journeyList(journeys))
}

func journeyList(journeys []string) string {
	if len(journeys) == 0 {
		return "пустой словарь"
	}
	return fmt.Sprintf("доступны: %s", joinQuoted(journeys))
}

func joinQuoted(values []string) string {
	out := ""
	for index, value := range values {
		if index > 0 {
			out += ", "
		}
		out += "«" + value + "»"
	}
	return out
}

// currentOf reads the derived current view of one work item of the case.
func (s *Service) currentOf(ctx context.Context, scope tenancy.Context, caseID, workItemID id.UUID) (casesDomain.WorkItemCurrent, error) {
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, caseID, []id.UUID{workItemID})
	if err != nil {
		return casesDomain.WorkItemCurrent{}, err
	}
	current, ok := currents[workItemID]
	if !ok {
		return casesDomain.WorkItemCurrent{}, casesDomain.ErrNotFound
	}
	return current, nil
}

// CreateWorkItem creates one typed and titled work item for a new intention
// of an existing conversation. There is no automatic link of events — the
// operator binds them explicitly (LinkEvent).
func (s *Service) CreateWorkItem(ctx context.Context, command casesAPI.CreateWorkItemCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		Type         string `json:"type"`
		Title        string `json:"title"`
		Reason       string `json:"reason"`
	}{command.CaseID.String(), command.ConversationID.String(), command.Type, command.Title, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "create_work_item", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	if err := rule("create_work_item", casesDomain.GuardReason(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("create_work_item", casesDomain.GuardWorkTitle(command.Title)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	journeys, err := s.taxonomyOf(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("create_work_item", requireTypeInTaxonomy(journeys, command.Type)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	workItemID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "create_work_item", WorkItemID: workItemID, Version: 1,
		State: casesDomain.WorkItemStateInWork, Type: command.Type, Title: command.Title,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := s.repository.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "create_work_item", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: command.ConversationID,
		Type: command.Type, Title: command.Title, Reason: command.Reason,
		WorkItemID: workItemID, Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	return outcome, nil
}

// ClassifyWorkItem writes the classification version: the type from the
// pinned taxonomy, the title, the honest reason. The untriaged state leaves
// through this command; a re-classification is a new version (the history
// is never rewritten).
func (s *Service) ClassifyWorkItem(ctx context.Context, command casesAPI.ClassifyCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case     string `json:"case"`
		WorkItem string `json:"work_item"`
		Expected int64  `json:"expected_version"`
		Type     string `json:"type"`
		Title    string `json:"title"`
		Reason   string `json:"reason"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.ExpectedVersion, command.Type, command.Title, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "classify_work_item", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	current, err := s.currentOf(ctx, scope, command.CaseID, command.WorkItemID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if current.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if err := rule("classify_work_item", casesDomain.GuardReason(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("classify_work_item", casesDomain.GuardClassify(current, command.Type, command.Title)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	journeys, err := s.taxonomyOf(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("classify_work_item", requireTypeInTaxonomy(journeys, command.Type)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The classification state: untriaged becomes in_work; an active item
	// keeps its state (the type/title change is the new version).
	state := current.State
	if state == casesDomain.WorkItemStateUntriaged {
		state = casesDomain.WorkItemStateInWork
	}
	outcome := casesAPI.WorkOutcome{
		Command: "classify_work_item", WorkItemID: command.WorkItemID, Version: current.Version + 1,
		State: state, Type: command.Type, Title: command.Title,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := s.repository.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "classify_work_item", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ExpectedVersion: command.ExpectedVersion, Type: command.Type, Title: command.Title,
		Reason: command.Reason, ToState: state, Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	return outcome, nil
}

// TransitionWorkItem moves one work item along the state ladder: the
// completion never closes the conversation and the case while active work
// items remain.
func (s *Service) TransitionWorkItem(ctx context.Context, command casesAPI.TransitionCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case     string `json:"case"`
		WorkItem string `json:"work_item"`
		Expected int64  `json:"expected_version"`
		ToState  string `json:"to_state"`
		Reason   string `json:"reason"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.ExpectedVersion, command.ToState, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "transition_work_item", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	current, err := s.currentOf(ctx, scope, command.CaseID, command.WorkItemID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if current.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if err := rule("transition_work_item", casesDomain.GuardReason(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("transition_work_item", casesDomain.GuardTransition(current, command.ToState)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "transition_work_item", WorkItemID: command.WorkItemID, Version: current.Version + 1,
		State: command.ToState, Type: current.Type, Title: current.Title,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := s.repository.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "transition_work_item", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ExpectedVersion: command.ExpectedVersion, ToState: command.ToState, Reason: command.Reason,
		Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	return outcome, nil
}

// SplitWorkItem splits one work item into at least two typed and titled
// targets: the source goes terminal split, the targets are born in_work
// with the backward reference; the facts stay on the source (the rescope
// is the explicit new-version command).
func (s *Service) SplitWorkItem(ctx context.Context, command casesAPI.SplitCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case     string                    `json:"case"`
		WorkItem string                    `json:"work_item"`
		Expected int64                     `json:"expected_version"`
		Targets  []casesDomain.SplitTarget `json:"targets"`
		Reason   string                    `json:"reason"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.ExpectedVersion, command.Targets, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "split_work_item", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	current, err := s.currentOf(ctx, scope, command.CaseID, command.WorkItemID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if current.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if err := rule("split_work_item", casesDomain.GuardReason(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("split_work_item", casesDomain.GuardSplit(current, command.Targets)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	journeys, err := s.taxonomyOf(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	for _, target := range command.Targets {
		if err := rule("split_work_item", requireTypeInTaxonomy(journeys, target.Type)); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	targets := make([]casesPort.WorkTargetParams, 0, len(command.Targets))
	targetIDs := make([]id.UUID, 0, len(command.Targets))
	for _, target := range command.Targets {
		targetID, err := s.ids.New(command.Now)
		if err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		targets = append(targets, casesPort.WorkTargetParams{WorkItemID: targetID, Type: target.Type, Title: target.Title})
		targetIDs = append(targetIDs, targetID)
	}
	outcome := casesAPI.WorkOutcome{
		Command: "split_work_item", WorkItemID: command.WorkItemID, Version: current.Version + 1,
		State: casesDomain.WorkItemStateSplit, TargetIDs: targetIDs,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := s.repository.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "split_work_item", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ConversationID: current.ConversationID, ExpectedVersion: command.ExpectedVersion,
		Targets: targets, Reason: command.Reason, ToState: casesDomain.WorkItemStateSplit,
		Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	return outcome, nil
}

// MergeWorkItems merges the sources into the target: the sources go
// terminal merged with merged_into; the fact scopes move by NEW versions
// (no fact rows are duplicated); the event links are appended onto the
// target in the received_at order. The command is disputable — the dispute
// creates the operational exception for the human.
func (s *Service) MergeWorkItems(ctx context.Context, command casesAPI.MergeCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case     string                      `json:"case"`
		Target   string                      `json:"target"`
		Expected int64                       `json:"expected_version"`
		Sources  []casesAPI.MergeSourceInput `json:"sources"`
		Reason   string                      `json:"reason"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.ExpectedVersion, command.Sources, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "merge_work_items", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	ids := make([]id.UUID, 0, len(command.Sources)+1)
	ids = append(ids, command.WorkItemID)
	for _, source := range command.Sources {
		ids = append(ids, source.WorkItemID)
	}
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, command.CaseID, ids)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	target, ok := currents[command.WorkItemID]
	if !ok {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if target.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	sources := make([]casesDomain.WorkItemCurrent, 0, len(command.Sources))
	for _, source := range command.Sources {
		current, ok := currents[source.WorkItemID]
		if !ok {
			return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
		}
		if current.Version != source.ExpectedVersion {
			return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
		}
		sources = append(sources, current)
	}
	if err := rule("merge_work_items", casesDomain.GuardReason(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("merge_work_items", casesDomain.GuardMerge(target, sources)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The deterministic lock order of every anchor of the command (the
	// sources and the target) — the store re-validates under the locks.
	sourceIDs := make([]id.UUID, 0, len(sources))
	sourceParams := make([]casesPort.WorkSourceParams, 0, len(sources))
	for _, source := range sources {
		sourceIDs = append(sourceIDs, source.WorkItemID)
		sourceParams = append(sourceParams, casesPort.WorkSourceParams{WorkItemID: source.WorkItemID, ExpectedVersion: source.Version})
	}
	outcome := casesAPI.WorkOutcome{
		Command: "merge_work_items", WorkItemID: command.WorkItemID, Version: target.Version,
		State: target.State, SourceIDs: sourceIDs, MergedInto: command.WorkItemID,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := s.repository.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "merge_work_items", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ConversationID: target.ConversationID, ExpectedVersion: command.ExpectedVersion,
		Sources: sourceParams, Reason: command.Reason, ToState: casesDomain.WorkItemStateMerged,
		Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	return outcome, nil
}

// LinkEvent binds one accepted event of the case to one active work item —
// the explicit operator action (there is no automatic link as a class).
func (s *Service) LinkEvent(ctx context.Context, command casesAPI.LinkEventCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case     string `json:"case"`
		WorkItem string `json:"work_item"`
		Expected int64  `json:"expected_version"`
		Event    string `json:"event"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.ExpectedVersion, command.EventID.String()})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "link_event", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	current, err := s.currentOf(ctx, scope, command.CaseID, command.WorkItemID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if current.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if err := rule("link_event", casesDomain.GuardLinkEvent(current)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	event, err := s.repository.ReadEventOfCase(ctx, scope, command.CaseID, command.EventID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if event.State != casesDomain.EventStateAccepted {
		return casesAPI.WorkOutcome{}, rule("link_event", fmt.Errorf("событие %s не принято: связь возможна только с принятыми событиями", command.EventID))
	}
	outcome := casesAPI.WorkOutcome{
		Command: "link_event", WorkItemID: command.WorkItemID, Version: current.Version,
		State: current.State, EventID: command.EventID,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := s.repository.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "link_event", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ExpectedVersion: command.ExpectedVersion, EventID: command.EventID,
		Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	return outcome, nil
}

// DisputeWorkItem disputes one work command outcome: the dispute is the
// append-only operational exception for the human (the kind
// work_item_dispute) — the material is never rewritten by the dispute.
func (s *Service) DisputeWorkItem(ctx context.Context, command casesAPI.DisputeCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case     string `json:"case"`
		WorkItem string `json:"work_item"`
		Expected int64  `json:"expected_version"`
		Note     string `json:"note"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.ExpectedVersion, command.Note})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "dispute_work_item", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	current, err := s.currentOf(ctx, scope, command.CaseID, command.WorkItemID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if current.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if err := rule("dispute_work_item", casesDomain.GuardReason(command.Note)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	exceptionID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "dispute_work_item", WorkItemID: command.WorkItemID, Version: current.Version,
		State: current.State, ExceptionID: exceptionID,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := s.repository.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "dispute_work_item", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ExpectedVersion: command.ExpectedVersion, Note: command.Note, ExceptionID: exceptionID,
		Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	return outcome, nil
}

// RecordFact records one fact with the full provenance. The material
// idempotency tuple (source, source id, material key, value digest) replays
// the original version on an exact repeat; the same key with another value
// is a new version and a read-time conflict. The channel_event source
// cites an accepted event of this case (the channel, the actor and both
// times derive from it); the operator source carries the command key.
func (s *Service) RecordFact(ctx context.Context, command casesAPI.RecordFactCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	materialKey, err := casesDomain.NormalizeMaterialKey(command.MaterialKey)
	if err != nil {
		return casesAPI.WorkOutcome{}, rule("record_fact", err)
	}
	value, valueDigest, err := casesDomain.FactValue(command.Value)
	if err != nil {
		return casesAPI.WorkOutcome{}, rule("record_fact", err)
	}
	if err := rule("record_fact", casesDomain.GuardReason(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case        string `json:"case"`
		WorkItem    string `json:"work_item"`
		MaterialKey string `json:"material_key"`
		ValueDigest string `json:"value_digest"`
		SourceType  string `json:"source_type"`
		SourceID    string `json:"source_id"`
		Reason      string `json:"reason"`
	}{command.CaseID.String(), command.WorkItemID.String(), materialKey, valueDigest, command.SourceType, command.SourceID, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_fact", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The provenance derivation: a channel_event source inherits the channel
	// and both times of the cited accepted event; an operator source records
	// the operator's own words with the command key.
	sourceID := command.SourceID
	channel := ""
	observedAt, receivedAt := command.Now.UTC(), command.Now.UTC()
	if command.SourceType == casesDomain.FactSourceChannelEvent {
		eventID, err := id.Parse(command.SourceID)
		if err != nil {
			return casesAPI.WorkOutcome{}, rule("record_fact", fmt.Errorf("источник channel_event требует event_id принятого события этого обращения"))
		}
		event, err := s.repository.ReadEventOfCase(ctx, scope, command.CaseID, eventID)
		if err != nil {
			if errors.Is(err, casesDomain.ErrNotFound) {
				return casesAPI.WorkOutcome{}, rule("record_fact", fmt.Errorf("событие-источник не найдено среди принятых событий этого обращения"))
			}
			return casesAPI.WorkOutcome{}, err
		}
		if event.State != casesDomain.EventStateAccepted {
			return casesAPI.WorkOutcome{}, rule("record_fact", fmt.Errorf("событие-источник не принято: provenance факта требует принятого события"))
		}
		channel = event.Channel
		observedAt = event.ProviderTimestamp.UTC()
		receivedAt = event.ReceivedAt.UTC()
		sourceID = event.EventID.String()
	}
	channel, err = casesDomain.FactSourceChannel(command.SourceType, channel)
	if err != nil {
		return casesAPI.WorkOutcome{}, rule("record_fact", err)
	}
	current, err := s.currentOf(ctx, scope, command.CaseID, command.WorkItemID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("record_fact", casesDomain.GuardRecordFact(current, command.SourceType, sourceID)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The material idempotency tuple: an exact repeat replays the original
	// version; the same tuple written into another case is an honest
	// conflict (the command key/source reuse across cases).
	existing, found, err := s.repository.LookupFactVersionBySource(ctx, scope, command.SourceType, sourceID, materialKey, valueDigest)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if found {
		if existing.CaseID != command.CaseID {
			return casesAPI.WorkOutcome{}, rule("record_fact", fmt.Errorf("источник уже записал это значение в другом обращении: ключ источника не переиспользуется между обращениями"))
		}
		material, err := s.repository.ReadFactMaterial(ctx, scope, command.CaseID, existing.FactID)
		if err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		status, _ := casesDomain.DeriveFactStatus(material.Versions, material.Confirmations)
		return casesAPI.WorkOutcome{
			Command: "record_fact", Replay: true, FactID: existing.FactID, FactVersion: existing.VersionNo,
			FactStatus: status, WorkItemID: existing.WorkItemID,
		}, nil
	}
	// The fact anchor: find-or-create by the material key.
	factID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	expectedVersions := int64(0)
	material, err := s.repository.ReadFactMaterialByKey(ctx, scope, command.CaseID, materialKey)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if material.Anchor.FactID.Valid() {
		factID = material.Anchor.FactID
		expectedVersions = int64(len(material.Versions))
	}
	// The projected status of the new version: the derivation over the read
	// material plus the new version (the version numbers are deterministic
	// — the store re-validates the expectation under the locks).
	projected := append(append([]casesDomain.FactVersion(nil), material.Versions...), casesDomain.FactVersion{
		VersionNo: expectedVersions + 1, ValueDigest: valueDigest,
	})
	status, _ := casesDomain.DeriveFactStatus(projected, material.Confirmations)
	outcome := casesAPI.WorkOutcome{
		Command: "record_fact", WorkItemID: command.WorkItemID,
		FactID: factID, FactVersion: expectedVersions + 1, FactStatus: status,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	err = s.repository.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_fact", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ExpectedVersion: expectedVersions, FactID: factID, MaterialKey: materialKey,
		Value: value, ValueDigest: valueDigest, SourceType: command.SourceType, SourceID: sourceID,
		Channel: channel, ObservedAt: observedAt.UTC().Truncate(time.Microsecond),
		ReceivedAt: receivedAt.UTC().Truncate(time.Microsecond), Reason: command.Reason,
		Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	})
	if err != nil {
		// A raced material write resolves with ONE fresh lookup — never a
		// recursion: the same case replays the original version, another
		// case is the honest cross-case conflict.
		if errors.Is(err, casesDomain.ErrFactMaterialExists) {
			fresh, found, lookupErr := s.repository.LookupFactVersionBySource(ctx, scope, command.SourceType, sourceID, materialKey, valueDigest)
			if lookupErr != nil {
				return casesAPI.WorkOutcome{}, lookupErr
			}
			if found && fresh.CaseID == command.CaseID {
				return casesAPI.WorkOutcome{
					Command: "record_fact", Replay: true, FactID: fresh.FactID,
					FactVersion: fresh.VersionNo, FactStatus: casesDomain.FactStatusUnverified, WorkItemID: fresh.WorkItemID,
				}, nil
			}
			return casesAPI.WorkOutcome{}, rule("record_fact", fmt.Errorf("источник уже записал это значение (возможно, в другом обращении): ключ источника не переиспользуется"))
		}
		return casesAPI.WorkOutcome{}, err
	}
	return outcome, nil
}

// ConfirmFactVersion confirms the EXACT version of one fact: the confirmer
// of this slice is operator_relay, the evidence is an accepted event of the
// case or a note. The confirmation never rewrites the versions; a new
// value on top of a confirmed one is a new version and the field conflicts
// again.
func (s *Service) ConfirmFactVersion(ctx context.Context, command casesAPI.ConfirmFactVersionCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case      string `json:"case"`
		Fact      string `json:"fact"`
		Version   int64  `json:"version_no"`
		Confirmer string `json:"confirmer_type"`
		Event     string `json:"evidence_event"`
		Note      string `json:"note"`
	}{command.CaseID.String(), command.FactID.String(), command.VersionNo, command.ConfirmerType, command.EvidenceEventID.String(), command.Note})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "confirm_fact_version", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	material, err := s.repository.ReadFactMaterial(ctx, scope, command.CaseID, command.FactID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !material.Anchor.FactID.Valid() {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if err := rule("confirm_fact_version", casesDomain.GuardConfirmFactVersion(len(material.Versions), command.VersionNo, command.ConfirmerType, command.EvidenceEventID, command.Note)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The evidence event must be an accepted event of this case.
	if command.EvidenceEventID.Valid() {
		event, err := s.repository.ReadEventOfCase(ctx, scope, command.CaseID, command.EvidenceEventID)
		if err != nil {
			if errors.Is(err, casesDomain.ErrNotFound) {
				return casesAPI.WorkOutcome{}, rule("confirm_fact_version", fmt.Errorf("evidence-событие не найдено среди принятых событий этого обращения"))
			}
			return casesAPI.WorkOutcome{}, err
		}
		if event.State != casesDomain.EventStateAccepted {
			return casesAPI.WorkOutcome{}, rule("confirm_fact_version", fmt.Errorf("evidence-событие не принято"))
		}
	}
	confirmationID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The projected status: the derivation over the read material plus the
	// new confirmation.
	projected := append(append([]casesDomain.FactConfirmation(nil), material.Confirmations...), casesDomain.FactConfirmation{
		VersionNo: command.VersionNo,
	})
	status, _ := casesDomain.DeriveFactStatus(material.Versions, projected)
	outcome := casesAPI.WorkOutcome{
		Command: "confirm_fact_version", FactID: command.FactID, Version: command.VersionNo,
		FactVersion: command.VersionNo, FactStatus: status,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := s.repository.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "confirm_fact_version", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, FactID: command.FactID,
		ExpectedVersion: int64(len(material.Versions)), VersionNo: command.VersionNo,
		ConfirmerType: command.ConfirmerType, EvidenceEventID: command.EvidenceEventID, Note: command.Note,
		ExceptionID: confirmationID, Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	return outcome, nil
}

// RescopeFact moves the scope of one fact to another active work item of
// the same case by writing a NEW version — the fact anchor and the rows are
// never duplicated, the history carries the actor and the reason.
func (s *Service) RescopeFact(ctx context.Context, command casesAPI.RescopeFactCommand) (casesAPI.WorkOutcome, error) {
	scope, err := s.commandScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case     string `json:"case"`
		Fact     string `json:"fact"`
		Target   string `json:"target"`
		Expected int64  `json:"expected_version"`
		Reason   string `json:"reason"`
	}{command.CaseID.String(), command.FactID.String(), command.TargetWorkItem.String(), command.ExpectedVersion, command.Reason})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "rescope_fact", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	material, err := s.repository.ReadFactMaterial(ctx, scope, command.CaseID, command.FactID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !material.Anchor.FactID.Valid() {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if int64(len(material.Versions)) != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if err := rule("rescope_fact", casesDomain.GuardReason(command.Reason)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	target, err := s.currentOf(ctx, scope, command.CaseID, command.TargetWorkItem)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("rescope_fact", casesDomain.GuardRescopeFact(material.Anchor.CaseID, command.CaseID, target)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if len(material.Versions) == 0 {
		return casesAPI.WorkOutcome{}, rule("rescope_fact", fmt.Errorf("факт без версий не переносится"))
	}
	latest := material.Versions[len(material.Versions)-1]
	projected := append(append([]casesDomain.FactVersion(nil), material.Versions...), casesDomain.FactVersion{
		VersionNo: latest.VersionNo + 1, ValueDigest: latest.ValueDigest,
	})
	status, _ := casesDomain.DeriveFactStatus(projected, material.Confirmations)
	outcome := casesAPI.WorkOutcome{
		Command: "rescope_fact", FactID: command.FactID, FactVersion: latest.VersionNo + 1,
		FactStatus: status, WorkItemID: command.TargetWorkItem,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := s.repository.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "rescope_fact", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, FactID: command.FactID,
		TargetWorkItem: command.TargetWorkItem, ExpectedVersion: command.ExpectedVersion,
		MaterialKey: material.Anchor.MaterialKey, Value: latest.Value, ValueDigest: latest.ValueDigest,
		SourceType: casesDomain.FactSourceOperator, SourceID: command.IdempotencyKey,
		Channel: "operator", ObservedAt: command.Now.UTC(), ReceivedAt: command.Now.UTC(),
		Reason: command.Reason, Outcome: outcomeJSON, Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	return outcome, nil
}

// FactsView reads the facts of one case with the full version history and
// the confirmations (the Work Control Card material): the values are case
// content — the review/audit boundary of the payload applies. A
// configuration-scoped member sees the metadata refusal.
func (s *Service) FactsView(ctx context.Context, actor domain.Principal, tenantID, caseID id.UUID) (casesAPI.FactsView, error) {
	scope, rights, err := s.viewScope(ctx, actor, tenantID)
	if err != nil {
		return casesAPI.FactsView{}, err
	}
	if !rights.Payload {
		return casesAPI.FactsView{}, domain.ErrForbidden
	}
	materials, err := s.repository.ReadFactsOfCase(ctx, scope, caseID)
	if err != nil {
		return casesAPI.FactsView{}, err
	}
	view := casesAPI.FactsView{
		TenantID: tenantID, CaseID: caseID, Facts: []casesAPI.FactView{},
		CanView: true, ViewScope: casesAPI.CaseScopePayload,
	}
	for _, material := range materials {
		status, _ := casesDomain.DeriveFactStatus(material.Versions, material.Confirmations)
		view.Facts = append(view.Facts, casesAPI.FactView{
			FactID: material.Anchor.FactID, MaterialKey: material.Anchor.MaterialKey,
			Status: status, NextAction: casesDomain.FactNextAction(status),
			Versions: material.Versions, Confirmations: material.Confirmations,
		})
	}
	sort.Slice(view.Facts, func(i, j int) bool { return view.Facts[i].MaterialKey < view.Facts[j].MaterialKey })
	return view, nil
}
