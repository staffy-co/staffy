package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.2 work command surface of the fixture: the REAL domain guards
// (GuardClassify, GuardTransition, GuardSplit, GuardMerge, GuardRecordFact,
// GuardConfirmFactVersion, GuardRescopeFact, DeriveFactStatus) over the
// in-memory version ladders — the transport contract under test is the
// production one.
// ---------------------------------------------------------------------------

// fixturePackPin is the Domain Pack pin of the fixture world (the same
// auto-parts dictionary the profile journeys pin): the taxonomy of the
// Work Control Card forms.
const (
	fixturePackID      = "auto-parts"
	fixturePackVersion = "1.0.0"
	// fixtureVehiclePackVersion is the Story 4.2 pin of the vehicle world
	// (the c2xx slot of the retro-3-3 register): the sessions carrying the
	// vehicle prefix pin auto-parts@3.0.0 — the schema-bearing pack the
	// identification panel derives over. Every other session keeps the
	// 1.0.0 world pin (the frozen words of the neighboring journeys stay
	// byte-stable — the rule 161).
	fixtureVehiclePackVersion = "3.0.0"
)

// fixtureVehicleSessionPrefix marks the ingress sessions of the vehicle
// world (the pin convention of the fixture — deterministic, no new
// toggles: the session id itself carries the world).
const fixtureVehicleSessionPrefix = "journey-vehicle-"

// fixtureVehicleSession answers whether one ingress session belongs to
// the vehicle world (the 3.0.0 pin).
func fixtureVehicleSession(sessionID string) bool {
	return strings.HasPrefix(sessionID, fixtureVehicleSessionPrefix)
}

// fixtureJourneys is the journeys dictionary the fixture taxonomy resolves
// (the real manifest of auto-parts@1.0.0 — «поиск детали», «уточнение аналога»).
var fixtureJourneys = []string{"поиск детали", "уточнение аналога"}

// fixtureVehicleJourneys is the journeys dictionary of the 3.0.0 pin (the
// real manifest of auto-parts@3.0.0 — the +1 identification journey).
var fixtureVehicleJourneys = []string{"поиск детали", "уточнение аналога", "идентификация автомобиля"}

// journeysOf resolves the journeys dictionary of one pinned pack version
// (the fixture taxonomy mirror of the newest catalog read).
func journeysOf(packVersion string) []string {
	if packVersion == fixtureVehiclePackVersion {
		return fixtureVehicleJourneys
	}
	return fixtureJourneys
}

// fixtureCriticalEntities is the material key dictionary of the same
// manifest (the fact form hint comes from the pinned pack, never a
// hardcoded list — «деталь, артикул, автомобиль» of auto-parts@1.0.0).
var fixtureCriticalEntities = []string{"деталь", "артикул", "автомобиль"}

// workCurrentOf derives the current view of one work item from the ladder
// (the same derivation the store executes: the maximal version carries the
// state; the type/title travel with the create/classify rows).
func (f *casesFixture) workCurrentOf(workItemID id.UUID) (casesDomain.WorkItemCurrent, bool) {
	ladder := f.workVersions[workItemID]
	anchor, known := f.workAnchors[workItemID]
	if !known || len(ladder) == 0 {
		return casesDomain.WorkItemCurrent{}, false
	}
	latest := ladder[len(ladder)-1]
	current := casesDomain.WorkItemCurrent{
		WorkItemID: workItemID, CaseID: anchor.CaseID, ConversationID: anchor.ConversationID,
		Version: latest.VersionNo, Kind: latest.Kind, State: latest.State, Reason: latest.Reason,
		ActorID: latest.ActorID, Ref: latest.Ref, CreatedAt: anchor.CreatedAt,
		Terminal: casesDomain.TerminalWorkItemStates[latest.State],
	}
	for index := len(ladder) - 1; index >= 0; index-- {
		if ladder[index].Type != "" {
			current.Type, current.Title = ladder[index].Type, ladder[index].Title
			break
		}
	}
	return current, true
}

// fixtureWorkSequence is the deterministic id generator of the fixture work
// material (the work items and the facts).
func (f *casesFixture) fixtureWorkSequence(kind string, seq int) id.UUID {
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(seq))
}

// workCommandFlow is the shared skeleton: the idempotency replay BEFORE the
// guards (the 1-9 precedent), the review scope check (the fixture operator
// is the review-scoped owner), the request hash of the material.
func (f *casesFixture) workCommandFlow(command string, key string, material any) (casesAPI.WorkOutcome, bool, error) {
	encoded, err := json.Marshal(material)
	if err != nil {
		return casesAPI.WorkOutcome{}, false, err
	}
	sum := sha256.Sum256(encoded)
	hash := hex.EncodeToString(sum[:])
	record := command + ":" + key
	if stored, found := f.commandOutcomes[record]; found {
		if f.commandHashes[record] != hash {
			return casesAPI.WorkOutcome{}, false, casesDomain.ErrCommandKeyConflict
		}
		stored.Replay = true
		return stored, false, nil
	}
	f.commandHashes[record] = hash
	return casesAPI.WorkOutcome{Command: command}, true, nil
}

// storeOutcome remembers the outcome of one executed command (the replay
// answer of the exact repeat).
func (f *casesFixture) storeOutcome(command, key string, outcome casesAPI.WorkOutcome) {
	outcome.Command = command
	f.commandOutcomes[command+":"+key] = outcome
}

// fixtureWorkRule wraps the domain guard failure into the honest rule error.
func fixtureWorkRule(command string, err error) error {
	if err == nil {
		return nil
	}
	return casesDomain.WorkRuleError{Rule: command, Reason: err.Error()}
}

// CreateWorkItem creates one typed and titled intention of the conversation.
func (f *casesFixture) CreateWorkItem(_ context.Context, command casesAPI.CreateWorkItemCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	outcome, execute, err := f.workCommandFlow("create_work_item", command.IdempotencyKey, struct {
		Case         string `json:"case"`
		Conversation string `json:"conversation"`
		Type         string `json:"type"`
		Title        string `json:"title"`
	}{command.CaseID.String(), command.ConversationID.String(), command.Type, command.Title})
	if err != nil || !execute {
		return outcome, err
	}
	if err := fixtureWorkRule("create_work_item", casesDomain.GuardWorkTitle(command.Title)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !f.fixtureTypeInTaxonomy(command.CaseID, command.Type) {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("create_work_item", fmt.Errorf("тип «%s» не входит в journeys закреплённого пака обращения", command.Type))
	}
	if _, known := f.sessionsOfCase(command.CaseID)[command.ConversationID]; !known {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	f.workSeq++
	workItemID := f.fixtureWorkSequence("wi", 60000+f.workSeq)
	f.workAnchors[workItemID] = casesDomain.WorkItem{
		TenantID: f.tenant, CaseID: command.CaseID, ConversationID: command.ConversationID,
		WorkItemID: workItemID, CreatedAt: command.Now,
	}
	f.items[command.CaseID] = append(f.items[command.CaseID], f.workAnchors[workItemID])
	f.workVersions[workItemID] = append(f.workVersions[workItemID], casesDomain.WorkItemVersion{
		TenantID: f.tenant, CaseID: command.CaseID, ConversationID: command.ConversationID,
		WorkItemID: workItemID, VersionNo: 1, Kind: casesDomain.VersionKindCreate,
		Type: command.Type, Title: command.Title, State: casesDomain.WorkItemStateInWork,
		Reason: command.Reason, ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "create_work_item", WorkItemID: workItemID, Version: 1, State: casesDomain.WorkItemStateInWork, Type: command.Type, Title: command.Title}
	f.storeOutcome("create_work_item", command.IdempotencyKey, outcome)
	return outcome, nil
}

// fixtureTypeInTaxonomy resolves the pinned dictionary of the case of
// the classified work item (the same manifest the real facade reads —
// the journeys travel with the pinned pack version of the case).
func (f *casesFixture) fixtureTypeInTaxonomy(caseID id.UUID, workType string) bool {
	packVersion := fixturePackVersion
	if material := f.material[caseID]; material != nil {
		packVersion = material.cases.PackVersion
	}
	for _, journey := range journeysOf(packVersion) {
		if journey == workType {
			return true
		}
	}
	return false
}

// ClassifyWorkItem writes the classification version through the REAL guard.
func (f *casesFixture) ClassifyWorkItem(_ context.Context, command casesAPI.ClassifyCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	outcome, execute, err := f.workCommandFlow("classify_work_item", command.IdempotencyKey, struct {
		Case     string `json:"case"`
		WorkItem string `json:"work_item"`
		Expected int64  `json:"expected_version"`
		Type     string `json:"type"`
		Title    string `json:"title"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.ExpectedVersion, command.Type, command.Title})
	if err != nil || !execute {
		return outcome, err
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if err := fixtureWorkRule("classify_work_item", casesDomain.GuardClassify(current, command.Type, command.Title)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !f.fixtureTypeInTaxonomy(command.CaseID, command.Type) {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("classify_work_item", fmt.Errorf("тип «%s» не входит в journeys закреплённого пака обращения", command.Type))
	}
	state := current.State
	if state == casesDomain.WorkItemStateUntriaged {
		state = casesDomain.WorkItemStateInWork
	}
	f.workVersions[command.WorkItemID] = append(f.workVersions[command.WorkItemID], casesDomain.WorkItemVersion{
		TenantID: f.tenant, CaseID: command.CaseID, ConversationID: current.ConversationID,
		WorkItemID: command.WorkItemID, VersionNo: current.Version + 1, Kind: casesDomain.VersionKindClassify,
		Type: command.Type, Title: command.Title, State: state,
		Reason: command.Reason, ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "classify_work_item", WorkItemID: command.WorkItemID, Version: current.Version + 1, State: state, Type: command.Type, Title: command.Title}
	f.storeOutcome("classify_work_item", command.IdempotencyKey, outcome)
	return outcome, nil
}

// TransitionWorkItem moves the state through the REAL ladder guard.
func (f *casesFixture) TransitionWorkItem(_ context.Context, command casesAPI.TransitionCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	outcome, execute, err := f.workCommandFlow("transition_work_item", command.IdempotencyKey, struct {
		Case     string `json:"case"`
		WorkItem string `json:"work_item"`
		Expected int64  `json:"expected_version"`
		ToState  string `json:"to_state"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.ExpectedVersion, command.ToState})
	if err != nil || !execute {
		return outcome, err
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if err := fixtureWorkRule("transition_work_item", casesDomain.GuardTransition(current, command.ToState)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	f.workVersions[command.WorkItemID] = append(f.workVersions[command.WorkItemID], casesDomain.WorkItemVersion{
		TenantID: f.tenant, CaseID: command.CaseID, ConversationID: current.ConversationID,
		WorkItemID: command.WorkItemID, VersionNo: current.Version + 1, Kind: casesDomain.VersionKindTransition,
		State: command.ToState, Reason: command.Reason, ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "transition_work_item", WorkItemID: command.WorkItemID, Version: current.Version + 1, State: command.ToState}
	f.storeOutcome("transition_work_item", command.IdempotencyKey, outcome)
	return outcome, nil
}

// SplitWorkItem writes the terminal split version and the two targets.
func (f *casesFixture) SplitWorkItem(_ context.Context, command casesAPI.SplitCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	outcome, execute, err := f.workCommandFlow("split_work_item", command.IdempotencyKey, struct {
		Case     string                    `json:"case"`
		WorkItem string                    `json:"work_item"`
		Expected int64                     `json:"expected_version"`
		Targets  []casesDomain.SplitTarget `json:"targets"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.ExpectedVersion, command.Targets})
	if err != nil || !execute {
		return outcome, err
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if err := fixtureWorkRule("split_work_item", casesDomain.GuardSplit(current, command.Targets)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	for _, target := range command.Targets {
		if !f.fixtureTypeInTaxonomy(command.CaseID, target.Type) {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("split_work_item", fmt.Errorf("тип «%s» не входит в journeys закреплённого пака обращения", target.Type))
		}
	}
	f.workVersions[command.WorkItemID] = append(f.workVersions[command.WorkItemID], casesDomain.WorkItemVersion{
		TenantID: f.tenant, CaseID: command.CaseID, ConversationID: current.ConversationID,
		WorkItemID: command.WorkItemID, VersionNo: current.Version + 1, Kind: casesDomain.VersionKindSplit,
		State: casesDomain.WorkItemStateSplit, Reason: command.Reason, ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	targetIDs := make([]id.UUID, 0, len(command.Targets))
	for index, target := range command.Targets {
		f.workSeq++
		targetID := f.fixtureWorkSequence("wi", 60000+f.workSeq)
		if index == 0 {
			// The backward reference of the source: the first target.
			f.workVersions[command.WorkItemID][len(f.workVersions[command.WorkItemID])-1].Ref = targetID
		}
		f.workAnchors[targetID] = casesDomain.WorkItem{
			TenantID: f.tenant, CaseID: command.CaseID, ConversationID: current.ConversationID,
			WorkItemID: targetID, CreatedAt: command.Now,
		}
		f.items[command.CaseID] = append(f.items[command.CaseID], f.workAnchors[targetID])
		f.workVersions[targetID] = append(f.workVersions[targetID], casesDomain.WorkItemVersion{
			TenantID: f.tenant, CaseID: command.CaseID, ConversationID: current.ConversationID,
			WorkItemID: targetID, VersionNo: 1, Kind: casesDomain.VersionKindCreate,
			Type: target.Type, Title: target.Title, State: casesDomain.WorkItemStateInWork,
			Reason: command.Reason, ActorID: command.Actor.ID, Ref: command.WorkItemID, CreatedAt: command.Now,
		})
		targetIDs = append(targetIDs, targetID)
	}
	outcome = casesAPI.WorkOutcome{Command: "split_work_item", WorkItemID: command.WorkItemID, Version: current.Version + 1, State: casesDomain.WorkItemStateSplit, TargetIDs: targetIDs}
	f.storeOutcome("split_work_item", command.IdempotencyKey, outcome)
	return outcome, nil
}

// MergeWorkItems merges the sources into the target: the terminal merged
// versions, the fact rescoping by new versions and the link move.
func (f *casesFixture) MergeWorkItems(_ context.Context, command casesAPI.MergeCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	outcome, execute, err := f.workCommandFlow("merge_work_items", command.IdempotencyKey, struct {
		Case     string                      `json:"case"`
		Target   string                      `json:"target"`
		Expected int64                       `json:"expected_version"`
		Sources  []casesAPI.MergeSourceInput `json:"sources"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.ExpectedVersion, command.Sources})
	if err != nil || !execute {
		return outcome, err
	}
	target, known := f.workCurrentOf(command.WorkItemID)
	if !known {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if target.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	sources := make([]casesDomain.WorkItemCurrent, 0, len(command.Sources))
	sourceIDs := make([]id.UUID, 0, len(command.Sources))
	for _, source := range command.Sources {
		current, known := f.workCurrentOf(source.WorkItemID)
		if !known {
			return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
		}
		if current.Version != source.ExpectedVersion {
			return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
		}
		sources = append(sources, current)
		sourceIDs = append(sourceIDs, source.WorkItemID)
	}
	if err := fixtureWorkRule("merge_work_items", casesDomain.GuardMerge(target, sources)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	sourceSet := map[id.UUID]bool{}
	for _, source := range sources {
		sourceSet[source.WorkItemID] = true
		f.workVersions[source.WorkItemID] = append(f.workVersions[source.WorkItemID], casesDomain.WorkItemVersion{
			TenantID: f.tenant, CaseID: command.CaseID, ConversationID: source.ConversationID,
			WorkItemID: source.WorkItemID, VersionNo: source.Version + 1, Kind: casesDomain.VersionKindMerge,
			State: casesDomain.WorkItemStateMerged, Reason: command.Reason, ActorID: command.Actor.ID,
			Ref: command.WorkItemID, CreatedAt: command.Now,
		})
	}
	// The fact rescope: new versions scoped to the target (no row duplication).
	for factID, versions := range f.factVersions {
		if len(versions) == 0 {
			continue
		}
		latest := versions[len(versions)-1]
		if !sourceSet[latest.WorkItemID] {
			continue
		}
		f.factVersions[factID] = append(versions, casesDomain.FactVersion{
			TenantID: f.tenant, CaseID: command.CaseID, FactID: factID, WorkItemID: command.WorkItemID,
			VersionNo: latest.VersionNo + 1, MaterialKey: latest.MaterialKey, Value: latest.Value,
			ValueDigest: latest.ValueDigest, SourceType: casesDomain.FactSourceOperator,
			SourceID: command.IdempotencyKey, Channel: "operator", ActorID: command.Actor.ID,
			ObservedAt: command.Now, ReceivedAt: command.Now, CreatedAt: command.Now,
		})
	}
	// The event links: the source events are appended onto the target.
	for _, sourceID := range sourceIDs {
		for _, eventID := range f.workLinks[sourceID] {
			linked := false
			for _, existing := range f.workLinks[command.WorkItemID] {
				if existing == eventID {
					linked = true
					break
				}
			}
			if !linked {
				f.workLinks[command.WorkItemID] = append(f.workLinks[command.WorkItemID], eventID)
			}
		}
	}
	outcome = casesAPI.WorkOutcome{Command: "merge_work_items", WorkItemID: command.WorkItemID, Version: target.Version, State: target.State, SourceIDs: sourceIDs, MergedInto: command.WorkItemID}
	f.storeOutcome("merge_work_items", command.IdempotencyKey, outcome)
	return outcome, nil
}

// LinkEvent binds one accepted event to one active work item.
func (f *casesFixture) LinkEvent(_ context.Context, command casesAPI.LinkEventCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	outcome, execute, err := f.workCommandFlow("link_event", command.IdempotencyKey, struct {
		Case     string `json:"case"`
		WorkItem string `json:"work_item"`
		Expected int64  `json:"expected_version"`
		Event    string `json:"event"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.ExpectedVersion, command.EventID.String()})
	if err != nil || !execute {
		return outcome, err
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	if err := fixtureWorkRule("link_event", casesDomain.GuardLinkEvent(current)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	event, accepted := f.eventByID(command.EventID)
	if !accepted || event.State != casesDomain.EventStateAccepted {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	for _, existing := range f.workLinks[command.WorkItemID] {
		if existing == command.EventID {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("link_event", fmt.Errorf("событие уже связано с этим рабочим элементом"))
		}
	}
	f.workLinks[command.WorkItemID] = append(f.workLinks[command.WorkItemID], command.EventID)
	outcome = casesAPI.WorkOutcome{Command: "link_event", WorkItemID: command.WorkItemID, Version: current.Version, State: current.State, EventID: command.EventID}
	f.storeOutcome("link_event", command.IdempotencyKey, outcome)
	return outcome, nil
}

// DisputeWorkItem raises the append-only exception for the human.
func (f *casesFixture) DisputeWorkItem(_ context.Context, command casesAPI.DisputeCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	outcome, execute, err := f.workCommandFlow("dispute_work_item", command.IdempotencyKey, struct {
		Case     string `json:"case"`
		WorkItem string `json:"work_item"`
		Expected int64  `json:"expected_version"`
		Note     string `json:"note"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.ExpectedVersion, command.Note})
	if err != nil || !execute {
		return outcome, err
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	f.exceptionSeq++
	exceptionID := f.fixtureWorkSequence("exc", 70000+f.exceptionSeq)
	summary := "Спор рабочего элемента (состояние " + current.State + ", версия " + fmt.Sprint(current.Version) + "): решение за человеком."
	if command.Note != "" {
		summary = "Спор рабочего элемента (состояние " + current.State + "): " + command.Note
	}
	f.exceptions = append(f.exceptions, casesDomain.OperationalException{
		TenantID: f.tenant, ExceptionID: exceptionID, Kind: casesDomain.ExceptionWorkItemDispute,
		Summary: summary, CorrelationID: exceptionID, CaseID: command.CaseID,
		ConversationID: current.ConversationID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{Command: "dispute_work_item", WorkItemID: command.WorkItemID, Version: current.Version, State: current.State, ExceptionID: exceptionID}
	f.storeOutcome("dispute_work_item", command.IdempotencyKey, outcome)
	return outcome, nil
}

// RecordFact writes the fact version with the full provenance through the
// REAL guards (the channel_event source inherits the event material).
func (f *casesFixture) RecordFact(_ context.Context, command casesAPI.RecordFactCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	materialKey, err := casesDomain.NormalizeMaterialKey(command.MaterialKey)
	if err != nil {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_fact", err)
	}
	value, valueDigest, err := casesDomain.FactValue(command.Value)
	if err != nil {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_fact", err)
	}
	outcome, execute, err := f.workCommandFlow("record_fact", command.IdempotencyKey, struct {
		Case        string `json:"case"`
		WorkItem    string `json:"work_item"`
		MaterialKey string `json:"material_key"`
		ValueDigest string `json:"value_digest"`
		SourceType  string `json:"source_type"`
		SourceID    string `json:"source_id"`
	}{command.CaseID.String(), command.WorkItemID.String(), materialKey, valueDigest, command.SourceType, command.SourceID})
	if err != nil || !execute {
		return outcome, err
	}
	sourceID := command.SourceID
	channel := ""
	observedAt, receivedAt := command.Now, command.Now
	if command.SourceType == casesDomain.FactSourceChannelEvent {
		event, accepted := f.eventByID(id.MustParse(sourceID))
		if !accepted || event.State != casesDomain.EventStateAccepted {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("record_fact", fmt.Errorf("событие-источник не найдено среди принятых событий этого обращения"))
		}
		channel = event.Channel
		observedAt, receivedAt = event.ProviderTimestamp, event.ReceivedAt
		sourceID = event.EventID.String()
	}
	channel, err = casesDomain.FactSourceChannel(command.SourceType, channel)
	if err != nil {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_fact", err)
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if err := fixtureWorkRule("record_fact", casesDomain.GuardRecordFact(current, command.SourceType, sourceID)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The material idempotency: the same source material replays.
	for factID, versions := range f.factVersions {
		for _, version := range versions {
			if version.SourceType == command.SourceType && version.SourceID == sourceID &&
				version.MaterialKey == materialKey && version.ValueDigest == valueDigest {
				confirmations := f.factConfirms[factID]
				status, _ := casesDomain.DeriveFactStatus(versions, confirmations)
				return casesAPI.WorkOutcome{
					Command: "record_fact", Replay: true, FactID: factID, FactVersion: version.VersionNo,
					FactStatus: status, WorkItemID: version.WorkItemID,
				}, nil
			}
		}
	}
	f.workSeq++
	factID := f.fixtureWorkSequence("fact", 80000+f.workSeq)
	anchorKey := command.CaseID.String() + "|" + materialKey
	if anchorID, found := f.factAnchors[anchorKey]; found {
		factID = anchorID
	} else {
		f.factAnchors[anchorKey] = factID
	}
	versions := f.factVersions[factID]
	f.factVersions[factID] = append(versions, casesDomain.FactVersion{
		TenantID: f.tenant, CaseID: command.CaseID, FactID: factID, WorkItemID: command.WorkItemID,
		VersionNo: int64(len(versions)) + 1, MaterialKey: materialKey, Value: value, ValueDigest: valueDigest,
		SourceType: command.SourceType, SourceID: sourceID, Channel: channel, ActorID: command.Actor.ID,
		ObservedAt: observedAt, ReceivedAt: receivedAt, CreatedAt: command.Now,
	})
	status, _ := casesDomain.DeriveFactStatus(f.factVersions[factID], f.factConfirms[factID])
	// The Story 2.3 tokenization: the identifier facts tokenize into the
	// keyed HMAC tokens with the duplicate candidates of the exact
	// matches (the same discipline as the store).
	if raw, isString := casesDomain.IdentifierStringValue(value); isString {
		f.recordIdentifierToken(command.CaseID, materialKey, raw, command.Now)
	}
	outcome = casesAPI.WorkOutcome{Command: "record_fact", WorkItemID: command.WorkItemID, FactID: factID, FactVersion: int64(len(versions)) + 1, FactStatus: status}
	f.storeOutcome("record_fact", command.IdempotencyKey, outcome)
	return outcome, nil
}

// ConfirmFactVersion binds the append-only confirmation to the exact version.
func (f *casesFixture) ConfirmFactVersion(_ context.Context, command casesAPI.ConfirmFactVersionCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	outcome, execute, err := f.workCommandFlow("confirm_fact_version", command.IdempotencyKey, struct {
		Case      string `json:"case"`
		Fact      string `json:"fact"`
		Version   int64  `json:"version_no"`
		Confirmer string `json:"confirmer_type"`
		Event     string `json:"evidence_event"`
		Note      string `json:"note"`
	}{command.CaseID.String(), command.FactID.String(), command.VersionNo, command.ConfirmerType, command.EvidenceEventID.String(), command.Note})
	if err != nil || !execute {
		return outcome, err
	}
	versions := f.factVersions[command.FactID]
	if len(versions) == 0 {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if err := fixtureWorkRule("confirm_fact_version", casesDomain.GuardConfirmFactVersion(len(versions), command.VersionNo, command.ConfirmerType, command.EvidenceEventID, command.Note)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if command.EvidenceEventID.Valid() {
		if _, accepted := f.eventByID(command.EvidenceEventID); !accepted {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("confirm_fact_version", fmt.Errorf("evidence-событие не найдено среди принятых событий этого обращения"))
		}
	}
	f.exceptionSeq++
	confirmationID := f.fixtureWorkSequence("conf", 90000+f.exceptionSeq)
	f.factConfirms[command.FactID] = append(f.factConfirms[command.FactID], casesDomain.FactConfirmation{
		TenantID: f.tenant, CaseID: command.CaseID, FactID: command.FactID,
		ConfirmationID: confirmationID, VersionNo: command.VersionNo,
		ConfirmerType: command.ConfirmerType, EvidenceEventID: command.EvidenceEventID,
		Note: command.Note, ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	status, _ := casesDomain.DeriveFactStatus(versions, f.factConfirms[command.FactID])
	outcome = casesAPI.WorkOutcome{Command: "confirm_fact_version", FactID: command.FactID, Version: command.VersionNo, FactVersion: command.VersionNo, FactStatus: status}
	f.storeOutcome("confirm_fact_version", command.IdempotencyKey, outcome)
	return outcome, nil
}

// RescopeFact writes the new version scoped to the target work item.
func (f *casesFixture) RescopeFact(_ context.Context, command casesAPI.RescopeFactCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	outcome, execute, err := f.workCommandFlow("rescope_fact", command.IdempotencyKey, struct {
		Case     string `json:"case"`
		Fact     string `json:"fact"`
		Target   string `json:"target"`
		Expected int64  `json:"expected_version"`
	}{command.CaseID.String(), command.FactID.String(), command.TargetWorkItem.String(), command.ExpectedVersion})
	if err != nil || !execute {
		return outcome, err
	}
	versions := f.factVersions[command.FactID]
	if int64(len(versions)) != command.ExpectedVersion || len(versions) == 0 {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	target, known := f.workCurrentOf(command.TargetWorkItem)
	if !known {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if err := fixtureWorkRule("rescope_fact", casesDomain.GuardRescopeFact(command.CaseID, command.CaseID, target)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	latest := versions[len(versions)-1]
	f.factVersions[command.FactID] = append(versions, casesDomain.FactVersion{
		TenantID: f.tenant, CaseID: command.CaseID, FactID: command.FactID, WorkItemID: command.TargetWorkItem,
		VersionNo: latest.VersionNo + 1, MaterialKey: latest.MaterialKey, Value: latest.Value,
		ValueDigest: latest.ValueDigest, SourceType: casesDomain.FactSourceOperator,
		SourceID: command.IdempotencyKey, Channel: "operator", ActorID: command.Actor.ID,
		ObservedAt: command.Now, ReceivedAt: command.Now, CreatedAt: command.Now,
	})
	status, _ := casesDomain.DeriveFactStatus(f.factVersions[command.FactID], f.factConfirms[command.FactID])
	outcome = casesAPI.WorkOutcome{Command: "rescope_fact", FactID: command.FactID, FactVersion: latest.VersionNo + 1, FactStatus: status, WorkItemID: command.TargetWorkItem}
	f.storeOutcome("rescope_fact", command.IdempotencyKey, outcome)
	return outcome, nil
}

// FactsView reads the fixture facts with the derivations.
func (f *casesFixture) FactsView(_ context.Context, _ domain.Principal, tenant, caseID id.UUID) (casesAPI.FactsView, error) {
	f.lock()
	defer f.unlock()
	if tenant != f.tenant {
		return casesAPI.FactsView{}, domain.ErrNotFound
	}
	view := casesAPI.FactsView{TenantID: tenant, CaseID: caseID, Facts: []casesAPI.FactView{}, CanView: true, ViewScope: casesAPI.CaseScopePayload}
	keys := make([]string, 0, len(f.factAnchors))
	for key := range f.factAnchors {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		factID := f.factAnchors[key]
		versions := f.factVersions[factID]
		if len(versions) == 0 || versions[0].CaseID != caseID {
			continue
		}
		confirmations := f.factConfirms[factID]
		status, _ := casesDomain.DeriveFactStatus(versions, confirmations)
		materialKey := key
		if bar := strings.LastIndex(key, "|"); bar >= 0 {
			materialKey = key[bar+1:]
		}
		view.Facts = append(view.Facts, casesAPI.FactView{
			FactID: factID, MaterialKey: materialKey, Status: status, NextAction: casesDomain.FactNextAction(status),
			Versions: versions, Confirmations: confirmations,
		})
	}
	return view, nil
}

// eventByID resolves one accepted fixture event by its id.
func (f *casesFixture) eventByID(eventID id.UUID) (casesDomain.ChannelEvent, bool) {
	for _, event := range f.events {
		if event != nil && event.EventID == eventID {
			return *event, true
		}
	}
	return casesDomain.ChannelEvent{}, false
}

var _ = strings.TrimSpace
var _ = time.Now

// sessionsOfCase resolves the conversations of one fixture case (the
// containment check of the create command).
func (f *casesFixture) sessionsOfCase(caseID id.UUID) map[id.UUID]bool {
	out := map[id.UUID]bool{}
	if material := f.material[caseID]; material != nil {
		for _, conversation := range material.conversations {
			out[conversation.ConversationID] = true
		}
	}
	return out
}
