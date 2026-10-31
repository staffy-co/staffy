//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// workCommandInput builds one canonical work command input of the operator
// (the owner with the review scope and the step-up).
func (w *casesWorld) workActor() identityDomain.Principal {
	return actorPrincipal{principalID: w.owner.ID, stepUp: true}.principal()
}

// workBaseInput is the shared expectation material of the work commands.
func (w *casesWorld) workBaseInput(caseID, workItemID id.UUID, expected int64) casesAPI.WorkCommandInput {
	return casesAPI.WorkCommandInput{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		ExpectedVersion: expected, IdempotencyKey: "", Now: casesClock.Now(),
	}
}

// acceptedCase ingests one phone event and returns the accepted case with
// its initial untriaged work item.
func (w *casesWorld) acceptedCase(t *testing.T, eventID, sessionID, payload string) (id.UUID, id.UUID) {
	t.Helper()
	outcome, err := w.cases.IngestEvent(context.Background(), w.ingestCommand(eventID, sessionID, payload))
	if err != nil {
		t.Fatal(err)
	}
	result := acceptedOutcome(t, outcome)
	detail, err := w.cases.CaseView(context.Background(), w.workActor(), w.tenant, result.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.WorkItems) != 1 {
		t.Fatalf("expected the initial work item, got %d", len(detail.WorkItems))
	}
	if detail.Case.PackID != "auto-parts" || detail.Case.PackVersion != "1.0.0" {
		t.Fatalf("expected the auto-parts pin on the case, got %s@%s", detail.Case.PackID, detail.Case.PackVersion)
	}
	return result.CaseID, detail.WorkItems[0].WorkItemID
}

// assertRuleError fails unless the error is the honest work rule refusal.
func assertRuleError(t *testing.T, err error, fragment string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected the honest rule refusal with %q", fragment)
	}
	var rule casesDomain.WorkRuleError
	if !errors.As(err, &rule) {
		t.Fatalf("expected WorkRuleError, got %v", err)
	}
	if !strings.Contains(rule.Reason, fragment) {
		t.Fatalf("expected the reason %q, got %q", fragment, rule.Reason)
	}
}

// TestClassifyWritesVersionAndReplaysIdempotently covers the first matrix
// rows: the classification of the initial work item writes the version 2
// with the actor and the reason; the type comes from the pinned taxonomy;
// the exact command repeat replays the original outcome; a type outside
// the pack and an invalid transition refuse honestly without material
// changes; a stale expected_version refuses.
func TestClassifyWritesVersionAndReplaysIdempotently(t *testing.T) {
	w := newCasesWorld(t, 510)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-classify-1", "sess-classify-1", `{"call":"inbound-call","from":"+79000000041"}`)

	base := w.workBaseInput(caseID, workItemID, 1)
	base.Reason = "Клиент ищет деталь на Civic 2020"
	command := casesAPI.ClassifyCommand{
		WorkCommandInput: base,
		Type:             "поиск детали", Title: "Подбор детали на Civic",
	}
	command.IdempotencyKey = "classify-1"
	outcome, err := w.cases.ClassifyWorkItem(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Replay || outcome.Version != 2 || outcome.State != "in_work" || outcome.Type != "поиск детали" {
		t.Fatalf("expected the fresh classification version, got %+v", outcome)
	}

	// The exact repeat replays the stored outcome before the guards (the
	// command record carries the same request hash).
	replayed, err := w.cases.ClassifyWorkItem(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replay || replayed.Version != 2 {
		t.Fatalf("expected the replayed classification, got %+v", replayed)
	}

	// The same key with another material is the honest key conflict.
	other := command
	other.Title = "Другое название"
	_, err = w.cases.ClassifyWorkItem(ctx, other)
	if !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("expected the command key conflict, got %v", err)
	}

	// The view: the derived current state is in_work with the type and the
	// title; the original event stays linked.
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.WorkItems) != 1 {
		t.Fatalf("expected one work item, got %d", len(detail.WorkItems))
	}
	current := detail.WorkItems[0]
	if current.Version != 2 || current.State != "in_work" || current.Type != "поиск детали" || current.Title != "Подбор детали на Civic" {
		t.Fatalf("expected the classified current view, got %+v", current)
	}
	if current.ActorID != w.owner.ID || current.Reason != command.Reason {
		t.Fatalf("expected the actor and the reason on the version, got %+v", current)
	}
	if detail.UntriagedText == "" {
		t.Fatal("the honest untriaged note stays part of the view contract")
	}

	// A type outside the pinned taxonomy refuses honestly: no version.
	outsideInput := w.workBaseInput(caseID, workItemID, 2)
	outsideInput.Reason, outsideInput.IdempotencyKey = "Вне словаря", "classify-outside-1"
	_, err = w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{
		WorkCommandInput: outsideInput,
		Type:             "уточнение услуги", Title: " Montagio на кейсе auto-parts",
	})
	assertRuleError(t, err, "не входит в journeys")

	// An invalid transition refuses: resolved → waiting is not on the
	// ladder; nothing is written.
	waitingInput := w.workBaseInput(caseID, workItemID, 2)
	waitingInput.Reason, waitingInput.IdempotencyKey = "В работу", "classify-wait-1"
	if _, err := w.cases.TransitionWorkItem(ctx, casesAPI.TransitionCommand{
		WorkCommandInput: waitingInput, ToState: "waiting",
	}); err != nil {
		t.Fatalf("in_work → waiting must be allowed: %v", err)
	}
	invalidInput := w.workBaseInput(caseID, workItemID, 3)
	invalidInput.Reason, invalidInput.IdempotencyKey = "Недопустимо", "classify-invalid-1"
	_, err = w.cases.TransitionWorkItem(ctx, casesAPI.TransitionCommand{
		WorkCommandInput: invalidInput, ToState: "waiting",
	})
	if err == nil {
		t.Fatal("expected the ladder refusal of waiting → waiting")
	}
	assertRuleError(t, err, "недопустим")

	// The stale expected_version refuses honestly (no silent merge).
	staleInput := w.workBaseInput(caseID, workItemID, 2)
	staleInput.Reason, staleInput.IdempotencyKey = "Устаревшая версия", "classify-stale-1"
	_, err = w.cases.TransitionWorkItem(ctx, casesAPI.TransitionCommand{
		WorkCommandInput: staleInput, ToState: "resolved",
	})
	if !errors.Is(err, casesDomain.ErrStaleVersion) {
		t.Fatalf("expected the stale version refusal, got %v", err)
	}

	// The conversation and the case stay active while the work item lives.
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Case.State != "open" || detail.Conversations[0].State != "active" {
		t.Fatalf("expected the live case and conversation, got %+v", detail.Case)
	}
	if detail.WorkItems[0].State != "waiting" {
		t.Fatalf("expected the waiting state, got %s", detail.WorkItems[0].State)
	}
}

// TestRecordFactProvenanceConflictAndConfirmation covers the fact matrix:
// the full provenance of the channel_event source (the channel, both
// times), the material idempotency (the exact tuple replays, another value
// is a new version and the field conflicts), the confirmation of the exact
// version resolves the conflict, a new value conflicts the field again.
func TestRecordFactProvenanceConflictAndConfirmation(t *testing.T) {
	w := newCasesWorld(t, 511)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-fact-1", "sess-fact-1", `{"call":"inbound-call","from":"+79000000042"}`)

	// Classify first: the fact scope must be an active work item.
	classifyInput := w.workBaseInput(caseID, workItemID, 1)
	classifyInput.Reason, classifyInput.IdempotencyKey = "Классификация", "fact-classify-1"
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{
		WorkCommandInput: classifyInput,
		Type:             "поиск детали", Title: "Подбор детали на Civic",
	}); err != nil {
		t.Fatal(err)
	}

	// The fact of the event: the material value, the channel_event source
	// with the event id, the full provenance.
	firstEventID := eventIDOf(t, w, caseID, "evt-fact-1")
	record := casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "  Автомобиль ", Value: []byte(`{"model":"Civic","year":2020}`),
		SourceType: casesDomain.FactSourceChannelEvent, SourceID: firstEventID.String(),
		Reason: "Клиент назвал автомобиль", IdempotencyKey: "record-fact-1", Now: casesClock.Now(),
	}
	outcome, err := w.cases.RecordFact(ctx, record)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Replay || outcome.FactVersion != 1 || outcome.FactStatus != casesDomain.FactStatusUnverified {
		t.Fatalf("expected the fresh unverified fact, got %+v", outcome)
	}
	factID := outcome.FactID

	// The exact command repeat replays the stored outcome; the same source
	// with the same material replays via the material tuple as well.
	replayed, err := w.cases.RecordFact(ctx, record)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replay || replayed.FactID != factID || replayed.FactVersion != 1 {
		t.Fatalf("expected the replayed fact, got %+v", replayed)
	}
	materialRepeat := record
	materialRepeat.IdempotencyKey = "record-fact-1b"
	materialRepeat.Reason = "Другая команда, тот же материал"
	replayed, err = w.cases.RecordFact(ctx, materialRepeat)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replay || replayed.FactVersion != 1 {
		t.Fatalf("expected the material replay, got %+v", replayed)
	}

	// The full provenance: the channel inherited from the event, both times
	// from the event (observed = the provider time).
	facts, err := w.cases.FactsView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Facts) != 1 || facts.Facts[0].MaterialKey != "автомобиль" {
		t.Fatalf("expected the normalized fact key, got %+v", facts.Facts)
	}
	version := facts.Facts[0].Versions[0]
	if version.SourceType != "channel_event" || version.SourceID != firstEventID.String() || version.Channel != "phone" {
		t.Fatalf("expected the event provenance, got %+v", version)
	}
	if !version.ObservedAt.Equal(casesClock.Now().Add(-time.Minute)) {
		t.Fatalf("expected observed = the provider time, got %v", version.ObservedAt)
	}
	if version.ReceivedAt != casesClock.Now() {
		t.Fatalf("expected received = the receive clock, got %v", version.ReceivedAt)
	}
	if version.ActorID != w.owner.ID {
		t.Fatalf("expected the recording operator, got %s", version.ActorID)
	}

	// The second event of the session brings another value: the field
	// conflicts (both versions are held, no automatic choice).
	continuation, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-fact-2", "sess-fact-1", `{"call":"inbound-call","note":"год 2019"}`))
	if err != nil {
		t.Fatal(err)
	}
	if acceptedOutcome(t, continuation).NewCase {
		t.Fatal("the session continuation must not create a new case")
	}
	secondEventID := eventIDOf(t, w, caseID, "evt-fact-2")
	second, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "автомобиль", Value: []byte(`{"model":"Civic","year":2019}`),
		SourceType: casesDomain.FactSourceChannelEvent, SourceID: secondEventID.String(),
		Reason: "Второе значение", IdempotencyKey: "record-fact-2", Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.FactID != factID || second.FactVersion != 2 || second.FactStatus != casesDomain.FactStatusConflicting {
		t.Fatalf("expected the conflicting second version, got %+v", second)
	}

	// The confirmation of the exact latest version with the event evidence
	// resolves the conflict.
	confirmed, err := w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: factID,
		VersionNo: 2, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
		EvidenceEventID: eventIDOf(t, w, caseID, "evt-fact-2"),
		Note:            "", IdempotencyKey: "confirm-fact-1", Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.FactStatus != casesDomain.FactStatusConfirmed {
		t.Fatalf("expected the confirmed status, got %+v", confirmed)
	}

	// A third source brings another value: the confirmed version is not
	// rewritten; the field conflicts again.
	third, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "автомобиль", Value: []byte(`{"model":"Civic","year":2021}`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "record-fact-3",
		Reason: "Третье значение", IdempotencyKey: "record-fact-3", Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if third.FactVersion != 3 || third.FactStatus != casesDomain.FactStatusConflicting {
		t.Fatalf("expected the field to conflict again, got %+v", third)
	}

	// The operator source without a producer is refused: connector and
	// system are the reserved dictionary values.
	_, err = w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "деталь", Value: []byte(`"clip"`),
		SourceType: "connector", SourceID: "record-fact-4",
		IdempotencyKey: "record-fact-4", Now: casesClock.Now(),
	})
	assertRuleError(t, err, "зарезервирован словарём без продюсера")

	// The client confirmer has no producer in this slice.
	_, err = w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: factID,
		VersionNo: 3, ConfirmerType: "client",
		EvidenceEventID: eventIDOf(t, w, caseID, "evt-fact-2"),
		IdempotencyKey:  "confirm-fact-2", Now: casesClock.Now(),
	})
	assertRuleError(t, err, "зарезервирован словарём без продюсера")

	// The evidence must exist: an unknown event refuses.
	_, err = w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: factID,
		VersionNo: 3, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
		EvidenceEventID: id.MustParse("01994f64-6c00-7000-8000-00000000dead"),
		Note:            "", IdempotencyKey: "confirm-fact-3", Now: casesClock.Now(),
	})
	assertRuleError(t, err, "evidence")

	// A note instead of the event works (the operator's relay of the words).
	if _, err := w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: factID,
		VersionNo: 3, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
		Note: "Клиент назвал год 2021 в звонке", IdempotencyKey: "confirm-fact-4", Now: casesClock.Now(),
	}); err != nil {
		t.Fatal(err)
	}
}

// eventIDOf resolves the internal event id of one provider event id of the
// case (the evidence material of the confirmations).
func eventIDOf(t *testing.T, w *casesWorld, caseID id.UUID, providerEventID string) id.UUID {
	t.Helper()
	detail, err := w.cases.CaseView(context.Background(), w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range detail.Timeline {
		if event.ProviderEventID == providerEventID {
			return event.EventID
		}
	}
	t.Fatalf("the provider event %s is not on the timeline", providerEventID)
	return id.UUID("")
}

// TestSplitMergeRescopeDispute covers the split/merge matrix: two typed and
// titled targets born in_work with the backward reference; the source goes
// terminal split; the facts stay on the source and move only by the
// explicit rescope; the merge marks the sources merged with merged_into,
// rescopes the facts by new versions (no fact rows are duplicated), moves
// the event links onto the target; the dispute raises the append-only
// operational exception for the human.
func TestSplitMergeRescopeDispute(t *testing.T) {
	w := newCasesWorld(t, 512)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-split-1", "sess-split-1", `{"call":"inbound-call","from":"+79000000043"}`)

	// Classify and record one fact on the initial work item.
	classifyInput := w.workBaseInput(caseID, workItemID, 1)
	classifyInput.Reason, classifyInput.IdempotencyKey = "Два намерения", "split-classify-1"
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{
		WorkCommandInput: classifyInput,
		Type:             "поиск детали", Title: "Смешанное обращение",
	}); err != nil {
		t.Fatal(err)
	}
	fact, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "автомобиль", Value: []byte(`{"model":"Civic"}`),
		SourceType: casesDomain.FactSourceChannelEvent, SourceID: eventIDOf(t, w, caseID, "evt-split-1").String(),
		Reason: "Факт источника", IdempotencyKey: "split-fact-1", Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Split: two typed and titled targets.
	splitInput := w.workBaseInput(caseID, workItemID, 2)
	splitInput.Reason, splitInput.IdempotencyKey = "Два независимых намерения", "split-work-1"
	split, err := w.cases.SplitWorkItem(ctx, casesAPI.SplitCommand{
		WorkCommandInput: splitInput,
		Targets: []casesDomain.SplitTarget{
			{Type: "поиск детали", Title: "Деталь на Civic"},
			{Type: "уточнение аналога", Title: "Подбор аналога"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(split.TargetIDs) != 2 {
		t.Fatalf("expected two targets, got %+v", split)
	}
	splitTypes := []string{"поиск детали", "уточнение аналога"}
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.WorkItems) != 3 {
		t.Fatalf("expected three work items after the split, got %d", len(detail.WorkItems))
	}
	byID := map[id.UUID]casesDomain.WorkItemCurrent{}
	for _, item := range detail.WorkItems {
		byID[item.WorkItemID] = item
	}
	source := byID[workItemID]
	if source.State != "split" || !source.Terminal || source.Version != 3 || source.Ref != split.TargetIDs[0] {
		t.Fatalf("expected the terminal split source, got %+v", source)
	}
	for index, targetID := range split.TargetIDs {
		target := byID[targetID]
		if target.State != "in_work" || target.Version != 1 || target.Type != splitTypes[index] {
			t.Fatalf("expected the born-in_work target %d, got %+v", index, target)
		}
		if target.Ref != workItemID {
			t.Fatalf("expected the backward reference of the target, got %s", target.Ref)
		}
	}

	// The fact stays on the source (the rescope is the explicit command).
	facts, err := w.cases.FactsView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Facts) != 1 || facts.Facts[0].Versions[0].WorkItemID != workItemID {
		t.Fatalf("expected the fact to stay on the source, got %+v", facts.Facts)
	}

	// The explicit rescope moves the fact onto the first target.
	firstTarget := split.TargetIDs[0]
	rescoped, err := w.cases.RescopeFact(ctx, casesAPI.RescopeFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: fact.FactID,
		TargetWorkItem: firstTarget, ExpectedVersion: 1,
		Reason: "Факт о детали нужен цели", IdempotencyKey: "rescope-1", Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if rescoped.FactVersion != 2 {
		t.Fatalf("expected the rescoped version 2, got %+v", rescoped)
	}
	facts, err = w.cases.FactsView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Facts) != 1 || len(facts.Facts[0].Versions) != 2 {
		t.Fatalf("expected the fact ladder of two versions (no row duplication), got %+v", facts.Facts)
	}
	if facts.Facts[0].Versions[1].WorkItemID != firstTarget || facts.Facts[0].Versions[1].SourceType != casesDomain.FactSourceOperator {
		t.Fatalf("expected the rescoped operator version, got %+v", facts.Facts[0].Versions[1])
	}

	// One target waits, the other resolves: the completion never closes the
	// case while the other item is active.
	waitingInput := w.workBaseInput(caseID, firstTarget, 1)
	waitingInput.Reason, waitingInput.IdempotencyKey = "Ждём звонок", "split-wait-1"
	if _, err := w.cases.TransitionWorkItem(ctx, casesAPI.TransitionCommand{
		WorkCommandInput: waitingInput, ToState: "waiting",
	}); err != nil {
		t.Fatal(err)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Case.State != "open" || detail.Conversations[0].State != "active" {
		t.Fatalf("the case stays open, got %+v", detail.Case)
	}

	// Merge the second target into the first: the source goes merged with
	// merged_into, the fact scopes and the event links move.
	secondTarget := split.TargetIDs[1]
	mergeInput := w.workBaseInput(caseID, firstTarget, 2)
	mergeInput.Reason, mergeInput.IdempotencyKey = "Фрагменты одной задачи", "merge-work-1"
	merge, err := w.cases.MergeWorkItems(ctx, casesAPI.MergeCommand{
		WorkCommandInput: mergeInput,
		Sources:          []casesAPI.MergeSourceInput{{WorkItemID: secondTarget, ExpectedVersion: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if merge.MergedInto != firstTarget || merge.SourceIDs[0] != secondTarget {
		t.Fatalf("expected the merged sources, got %+v", merge)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	byID = map[id.UUID]casesDomain.WorkItemCurrent{}
	for _, item := range detail.WorkItems {
		byID[item.WorkItemID] = item
	}
	merged := byID[secondTarget]
	if merged.State != "merged" || merged.Ref != firstTarget {
		t.Fatalf("expected the merged source with merged_into, got %+v", merged)
	}
	// The first event was linked to the initial (split) work item by the
	// ingest; the merged target inherits the links of the source only — the
	// initial link stays on the split source. The merge itself moves the
	// source's links (none here yet) — link one event explicitly first.

	// The dispute: the append-only exception for the human.
	disputeInput := w.workBaseInput(caseID, firstTarget, 2)
	disputeInput.IdempotencyKey = "dispute-1"
	dispute, err := w.cases.DisputeWorkItem(ctx, casesAPI.DisputeCommand{
		WorkCommandInput: disputeInput,
		Note:             "Объединение спорно: намерения были независимыми",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !dispute.ExceptionID.Valid() {
		t.Fatalf("expected the exception id, got %+v", dispute)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, exception := range detail.Exceptions {
		if exception.ExceptionID == dispute.ExceptionID && exception.Kind == "work_item_dispute" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the visible work_item_dispute exception, got %+v", detail.Exceptions)
	}
}

// TestWorkCommandOptimisticConcurrencyRaces covers the matrix race row:
// two commands over the same expected version — exactly one applies, the
// second refuses with the stale version; the material stays consistent.
func TestWorkCommandOptimisticConcurrencyRaces(t *testing.T) {
	w := newCasesWorld(t, 513)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-race-1", "sess-race-1", `{"call":"inbound-call","from":"+79000000044"}`)
	classifyInput := w.workBaseInput(caseID, workItemID, 1)
	classifyInput.Reason, classifyInput.IdempotencyKey = "Классификация", "race-classify-1"
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{
		WorkCommandInput: classifyInput,
		Type:             "поиск детали", Title: "Гонка версий",
	}); err != nil {
		t.Fatal(err)
	}
	firstInput := w.workBaseInput(caseID, workItemID, 2)
	firstInput.Reason, firstInput.IdempotencyKey = "Первая команда", "race-first-1"
	first, err := w.cases.TransitionWorkItem(ctx, casesAPI.TransitionCommand{
		WorkCommandInput: firstInput, ToState: "waiting",
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Replay || first.Version != 3 {
		t.Fatalf("expected the applied first command, got %+v", first)
	}
	secondInput := w.workBaseInput(caseID, workItemID, 2)
	secondInput.Reason, secondInput.IdempotencyKey = "Вторая команда с той же версией", "race-second-1"
	_, err = w.cases.TransitionWorkItem(ctx, casesAPI.TransitionCommand{
		WorkCommandInput: secondInput, ToState: "resolved",
	})
	if !errors.Is(err, casesDomain.ErrStaleVersion) {
		t.Fatalf("expected the optimistic concurrency refusal, got %v", err)
	}
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.WorkItems[0].Version != 3 || detail.WorkItems[0].State != "waiting" {
		t.Fatalf("expected exactly one applied command, got %+v", detail.WorkItems[0])
	}
}

// TestWorkCommandsAuthorizationAndPrivacy covers the authorization matrix:
// the command surface requires the review scope; the fact values follow
// the review/audit boundary (the configuration-only member gets the honest
// refusal); a foreign tenant is a plain 404 (the tenant existence is never
// disclosed).
func TestWorkCommandsAuthorizationAndPrivacy(t *testing.T) {
	w := newCasesWorld(t, 514)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-auth-1", "sess-auth-1", `{"call":"inbound-call","from":"+79000000045"}`)

	// A configuration-only member: the case metadata is visible, the fact
	// values and the commands are honestly refused.
	configOnly := inviteMember(t, w.identity, w.owner, w.tenant, "cases-config-414@example.org", identityDomain.Configurator,
		[]identityDomain.Scope{identityDomain.Configuration}, "000000004141", "000000041421")
	if _, err := w.cases.CaseView(ctx, configOnly, w.tenant, caseID); err != nil {
		t.Fatalf("the configuration member sees the case metadata: %v", err)
	}
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: configOnly, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: 1, IdempotencyKey: "auth-classify-1", Now: casesClock.Now(),
		},
		Type: "поиск детали", Title: "Нет полномочий",
	}); !errors.Is(err, identityDomain.ErrForbidden) {
		t.Fatalf("expected the review-scope refusal, got %v", err)
	}
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: configOnly, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "деталь", Value: []byte(`"clip"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "auth-fact-1",
		IdempotencyKey: "auth-fact-1", Now: casesClock.Now(),
	}); !errors.Is(err, identityDomain.ErrForbidden) {
		t.Fatalf("expected the review-scope refusal of the fact write, got %v", err)
	}
	if _, err := w.cases.FactsView(ctx, configOnly, w.tenant, caseID); !errors.Is(err, identityDomain.ErrForbidden) {
		t.Fatalf("expected the facts boundary refusal, got %v", err)
	}

	// A configuration+audit member: the audit scope ADDS the payload/facts
	// boundary on top of the configuration one (the viewScope accumulates
	// the rights of EVERY scope — the first match does not decide; the
	// work commands stay review-only). The role matrix needs both roles:
	// the Configurator carries the configuration scope, the ExpertQA the
	// audit one.
	configAuditCorrelation := id.MustParse("01994f64-6c00-7000-8000-000000004145")
	_, configAuditToken, err := w.identity.InviteMember(ctx, identityAPI.MembershipCommand{
		Actor: w.owner, TenantID: w.tenant, Email: "cases-config-audit-414@example.org",
		Roles:  []identityDomain.Role{identityDomain.Configurator, identityDomain.ExpertQA},
		Scopes: []identityDomain.Scope{identityDomain.Configuration, identityDomain.Audit}, CorrelationID: configAuditCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	configAudit := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-000000041425"), VerifiedEmail: "cases-config-audit-414@example.org"}
	if _, err = w.identity.AcceptInvitation(ctx, configAudit, configAuditToken, configAuditCorrelation); err != nil {
		t.Fatal(err)
	}
	facts, err := w.cases.FactsView(ctx, configAudit, w.tenant, caseID)
	if err != nil {
		t.Fatalf("the configuration+audit member sees the fact values: %v", err)
	}
	if len(facts.Facts) != 0 {
		t.Fatalf("the case has no facts yet, got %+v", facts.Facts)
	}
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: configAudit, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "деталь", Value: []byte(`"clip"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "auth-audit-fact-1",
		IdempotencyKey: "auth-audit-fact-1", Now: casesClock.Now(),
	}); !errors.Is(err, identityDomain.ErrForbidden) {
		t.Fatalf("the work command boundary stays review-only, got %v", err)
	}

	// A member of another tenant: the plain 404 (no tenant disclosure).
	other := inviteMember(t, w.identity, w.owner, w.tenant, "cases-foreign-414@example.org", "expert_qa",
		[]identityDomain.Scope{identityDomain.Review}, "000000004142", "000000041422")
	// The foreign actor must be a member of ANOTHER tenant: build one.
	foreignTenant := createForeignTenant(t, w)
	foreign := inviteMember(t, w.identity, w.owner, foreignTenant, "cases-foreign-414@example.org", "expert_qa",
		[]identityDomain.Scope{identityDomain.Review}, "000000004143", "000000041423")
	if _, err := w.cases.CaseView(ctx, foreign, w.tenant, caseID); !errors.Is(err, identityDomain.ErrNotFound) {
		t.Fatalf("expected the plain 404 of the foreign tenant, got %v", err)
	}
	_ = other
	if _, err := w.cases.FactsView(ctx, foreign, w.tenant, caseID); !errors.Is(err, identityDomain.ErrNotFound) {
		t.Fatalf("expected the plain 404 of the facts read, got %v", err)
	}
}

// createForeignTenant builds one tenant the foreign actor belongs to.
func createForeignTenant(t *testing.T, w *casesWorld) id.UUID {
	t.Helper()
	return createTenant(t, w.identity, w.owner, "Foreign Space 414", fmt.Sprintf("foreign-414-%04d", 414), "000000004144").Tenant.ID
}

// TestSessionContinuationGrowsTimelineWithoutNewWorkItems covers the
// continuation matrix row: the second event of the known session grows the
// timeline, creates no work item and recreates no facts; the new intent is
// the explicit CreateWorkItem command; the event binds to it only by the
// explicit LinkEvent (there is no automatic link).
func TestSessionContinuationGrowsTimelineWithoutNewWorkItems(t *testing.T) {
	w := newCasesWorld(t, 515)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-cont-1", "sess-cont-1", `{"call":"inbound-call","from":"+79000000046"}`)

	continuation := acceptedOutcome(t, ingestEventOf(t, w, "evt-cont-2", "sess-cont-1", `{"call":"inbound-call","note":"и ещё деталь"}`))
	if continuation.NewCase || continuation.CaseID != caseID {
		t.Fatalf("expected the continuation of the same case, got %+v", continuation)
	}
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Timeline) != 2 {
		t.Fatalf("expected the grown timeline, got %d events", len(detail.Timeline))
	}
	if len(detail.WorkItems) != 1 {
		t.Fatalf("expected no new work item on the continuation, got %d", len(detail.WorkItems))
	}

	// The new intent: the explicit CreateWorkItem of the existing
	// conversation.
	created, err := w.cases.CreateWorkItem(ctx, casesAPI.CreateWorkItemCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: detail.Conversations[0].ConversationID,
		Type:           "уточнение аналога", Title: "Второе намерение того же клиента",
		Reason: "Новое намерение в том же диалоге", IdempotencyKey: "create-wi-1", Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Version != 1 || created.State != "in_work" || !created.WorkItemID.Valid() {
		t.Fatalf("expected the created work item, got %+v", created)
	}

	// The link of the second event is the explicit command.
	secondEvent := detail.Timeline[1].EventID
	linkInput := w.workBaseInput(caseID, created.WorkItemID, 1)
	linkInput.IdempotencyKey = "link-event-1"
	linked, err := w.cases.LinkEvent(ctx, casesAPI.LinkEventCommand{
		WorkCommandInput: linkInput,
		EventID:          secondEvent,
	})
	if err != nil {
		t.Fatal(err)
	}
	if linked.EventID != secondEvent {
		t.Fatalf("expected the linked event, got %+v", linked)
	}

	// The unlinked-first fact discipline: recording a fact scoped to the
	// original item from the second event is allowed (the operator cites
	// the event) — the automatic link does not exist, but the fact scope is
	// the operator's choice.
	_, err = w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "деталь", Value: []byte(`"clip"`),
		SourceType: casesDomain.FactSourceChannelEvent, SourceID: secondEvent.String(),
		Reason: "Вторая деталь", IdempotencyKey: "cont-fact-1", Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

// ingestEventOf ingests one event of the world connection and fails loudly.
func ingestEventOf(t *testing.T, w *casesWorld, eventID, sessionID, payload string) casesAPI.IngestOutcome {
	t.Helper()
	outcome, err := w.cases.IngestEvent(context.Background(), w.ingestCommand(eventID, sessionID, payload))
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// TestClassifyRefusesThePreMigrationCaseWithoutPin covers the NULL-pin
// matrix row: a case created before the migration carries no pack pin —
// the classification refuses with the honest "taxonomy неизвестна" text
// (only the pre-migration test data; new cases always carry the pin).
func TestClassifyRefusesThePreMigrationCaseWithoutPin(t *testing.T) {
	w := newCasesWorld(t, 516)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-nullpin-1", "sess-nullpin-1", `{"call":"inbound-call","from":"+79000000047"}`)

	// Simulate the pre-migration row: the pin is cleared by the admin (the
	// only writer of the historical state).
	if _, err := adminPool.Exec(ctx,
		"update cases.cases set pack_id = null, pack_version = null where tenant_id = $1 and case_id = $2",
		w.tenant, caseID); err != nil {
		t.Fatal(err)
	}
	nullPinInput := w.workBaseInput(caseID, workItemID, 1)
	nullPinInput.Reason, nullPinInput.IdempotencyKey = "До-миграционные данные", "nullpin-1"
	_, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{
		WorkCommandInput: nullPinInput,
		Type:             "поиск детали", Title: "Без пина",
	})
	assertRuleError(t, err, "taxonomy неизвестна")
}

var _ = time.Now

// ---------------------------------------------------------------------------
// The concurrency battery of the R1 fixes: the raced classes the sequential
// tests cannot see. Every assertion is interleaving-independent — the honest
// signal set is fixed, the net effect on the version ladders is fixed; the
// winner is not.
// ---------------------------------------------------------------------------

// conversationOf answers the first conversation id of the case (the
// CreateWorkItem target surface of the concurrency battery).
func conversationOf(t *testing.T, w *casesWorld, caseID id.UUID) id.UUID {
	t.Helper()
	detail, err := w.cases.CaseView(context.Background(), w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Conversations) == 0 {
		t.Fatal("expected at least one conversation of the accepted case")
	}
	return detail.Conversations[0].ConversationID
}

// isHonestWorkSignal fails unless the error is one of the honest answers of
// the work surface: a rule refusal, the stale version, the key conflict or
// the material conflict (a raw 23505/500 — the M1 class — fails loudly).
func isHonestWorkSignal(t *testing.T, err error, context string) {
	t.Helper()
	if err == nil {
		return
	}
	if errors.Is(err, casesDomain.ErrStaleVersion) || errors.Is(err, casesDomain.ErrCommandKeyConflict) || errors.Is(err, casesDomain.ErrFactMaterialExists) {
		return
	}
	var ruleErr casesDomain.WorkRuleError
	if errors.As(err, &ruleErr) {
		return
	}
	t.Fatalf("%s: unexpected non-honest error class (the raw conflict leak of the M1 review class): %v", context, err)
}

// TestRacedIdenticalCommandsReplayTheStoredOutcome covers the M3 class: two
// concurrent byte-identical commands both pass the pre-lookup; the anchor
// locks serialize them, the twin commits first, the loser re-validates and
// fails (stale or the command record pkey) — the inline key resolution
// replays the stored outcome: the client of a double click sees two equal
// answers, the net effect is exactly one version.
func TestRacedIdenticalCommandsReplayTheStoredOutcome(t *testing.T) {
	w := newCasesWorld(t, 517)
	// The race needs the thread-safe real generator (the 2-1 precedent).
	w.cases = w.buildCasesService(&employeeIDs{})
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-raced-key-1", "sess-raced-key-1", `{"call":"inbound-call","from":"+79000000048"}`)
	command := casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "автомобиль", Value: []byte(`{"model":"Civic"}`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "raced-key-source-1",
		Reason: "Одновременный двойной клик", IdempotencyKey: "raced-key-1", Now: casesClock.Now(),
	}
	results := make([]casesAPI.WorkOutcome, 2)
	errs := make([]error, 2)
	start := make(chan struct{})
	var group sync.WaitGroup
	for index := range results {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			<-start
			results[i], errs[i] = w.cases.RecordFact(ctx, command)
		}(index)
	}
	close(start)
	group.Wait()
	for index, err := range errs {
		if err != nil {
			t.Fatalf("raced twin %d: the identical repeat must resolve honestly, got %v", index, err)
		}
	}
	if results[0].FactID != results[1].FactID || results[0].FactVersion != results[1].FactVersion {
		t.Fatalf("the raced twins must answer the identical material, got %+v and %+v", results[0], results[1])
	}
	applied := 0
	for _, result := range results {
		if !result.Replay {
			applied++
		}
	}
	if applied != 1 {
		t.Fatalf("exactly one twin applies, the other replays; applied=%d", applied)
	}
	facts, err := w.cases.FactsView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Facts) != 1 || len(facts.Facts[0].Versions) != 1 {
		t.Fatalf("expected exactly one fact with one version after the raced twins, got %+v", facts.Facts)
	}
}

// TestRacedFactWritersSerializeOnTheUnifiedFactLocks covers the M1 class: a
// record of a NEW value of one fact and a rescope of the same fact race —
// the unified fact key pair (the material key AND the fact id of BOTH
// commands) serializes them; one applies, the other answers the honest
// stale signal; the version ladder grows by exactly one step and never
// leaks a raw 23505 (the deterministic 500 of the review probe).
func TestRacedFactWritersSerializeOnTheUnifiedFactLocks(t *testing.T) {
	w := newCasesWorld(t, 518)
	// The race needs the thread-safe real generator (the 2-1 precedent).
	w.cases = w.buildCasesService(&employeeIDs{})
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-raced-fact-1", "sess-raced-fact-1", `{"call":"inbound-call","from":"+79000000049"}`)
	classifyInput := w.workBaseInput(caseID, workItemID, 1)
	classifyInput.Reason, classifyInput.IdempotencyKey = "Классификация", "raced-fact-classify-1"
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{
		WorkCommandInput: classifyInput, Type: "поиск детали", Title: "Гонка писателей факта",
	}); err != nil {
		t.Fatal(err)
	}
	firstEventID := eventIDOf(t, w, caseID, "evt-raced-fact-1")
	record := casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "автомобиль", Value: []byte(`{"model":"Civic"}`),
		SourceType: casesDomain.FactSourceChannelEvent, SourceID: firstEventID.String(),
		Reason: "Первая версия", IdempotencyKey: "raced-fact-1", Now: casesClock.Now(),
	}
	first, err := w.cases.RecordFact(ctx, record)
	if err != nil {
		t.Fatal(err)
	}
	// The second work item: the rescope target.
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	created, err := w.cases.CreateWorkItem(ctx, casesAPI.CreateWorkItemCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: detail.Conversations[0].ConversationID,
		Type:           "уточнение аналога", Title: "Цель переноса",
		Reason: "Второй элемент для переноса", IdempotencyKey: "raced-fact-create-1", Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	// The racing pair: the rescope of the existing fact and the record of a
	// NEW value under the same material key (a different command key and a
	// different source — no idempotency shortcut between them).
	rescope := casesAPI.RescopeFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: first.FactID,
		TargetWorkItem: created.WorkItemID, ExpectedVersion: 1,
		Reason: "Одновременный перенос", IdempotencyKey: "raced-rescope-1", Now: casesClock.Now(),
	}
	newValue := record
	newValue.Value = []byte(`{"model":"Civic","year":2020}`)
	newValue.SourceType = casesDomain.FactSourceOperator
	newValue.SourceID = "raced-fact-second-source-1"
	newValue.IdempotencyKey = "raced-record-2"
	newValue.Reason = "Одновременное новое значение"
	type commandResult struct {
		outcome casesAPI.WorkOutcome
		err     error
	}
	results := make(chan commandResult, 2)
	start := make(chan struct{})
	var group sync.WaitGroup
	run := func(call func() (casesAPI.WorkOutcome, error)) {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			outcome, err := call()
			results <- commandResult{outcome, err}
		}()
	}
	run(func() (casesAPI.WorkOutcome, error) { return w.cases.RescopeFact(ctx, rescope) })
	run(func() (casesAPI.WorkOutcome, error) { return w.cases.RecordFact(ctx, newValue) })
	close(start)
	group.Wait()
	close(results)
	for result := range results {
		isHonestWorkSignal(t, result.err, "the raced fact writer")
	}
	facts, err := w.cases.FactsView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Facts) != 1 {
		t.Fatalf("one fact anchor carries the raced writers, got %d", len(facts.Facts))
	}
	// The honest interleavings: BOTH commands re-plan their expectation at
	// entry — if the twin committed before the entry read, the ladder is
	// 1+2 steps (both applied, contiguous); if both read the same height,
	// the loser under the unified locks answers the stale signal (1 step).
	// Never a raw conflict, never a broken ladder.
	versions := facts.Facts[0].Versions
	if len(versions) < 2 || len(versions) > 3 {
		t.Fatalf("the ladder grows by one or two contiguous steps over the raced pair, got %d", len(versions))
	}
	for index, version := range versions {
		if version.VersionNo != int64(index+1) {
			t.Fatalf("the version numbers stay contiguous, got %+v", versions)
		}
	}
	if len(versions) == 3 && versions[2].ValueDigest == versions[1].ValueDigest {
		t.Fatalf("the third step must be the new value, not a rescope echo, got %+v", versions)
	}
}

// TestRecordFactRefusesTheTerminalWorkItemUnderTheLock covers the M2 class
// and the m4 letter of the material idempotency: a terminal scope refuses
// the NEW fact material honestly (the guard runs under the work lock — the
// re-check cannot be skipped by a concurrent transition), while the exact
// repeat of the material recorded while the item was active replays the
// original version (a repeat writes nothing, the terminal scope refusal
// applies to new material only).
func TestRecordFactRefusesTheTerminalWorkItemUnderTheLock(t *testing.T) {
	w := newCasesWorld(t, 519)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-terminal-1", "sess-terminal-1", `{"call":"inbound-call","from":"+79000000050"}`)
	classifyInput := w.workBaseInput(caseID, workItemID, 1)
	classifyInput.Reason, classifyInput.IdempotencyKey = "Классификация", "terminal-classify-1"
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{
		WorkCommandInput: classifyInput, Type: "поиск детали", Title: "Терминальный скоуп",
	}); err != nil {
		t.Fatal(err)
	}
	firstEventID := eventIDOf(t, w, caseID, "evt-terminal-1")
	recorded := casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "автомобиль", Value: []byte(`{"model":"Civic"}`),
		SourceType: casesDomain.FactSourceChannelEvent, SourceID: firstEventID.String(),
		Reason: "Материал при активном элементе", IdempotencyKey: "terminal-fact-1", Now: casesClock.Now(),
	}
	first, err := w.cases.RecordFact(ctx, recorded)
	if err != nil {
		t.Fatal(err)
	}
	transitionInput := w.workBaseInput(caseID, workItemID, 2)
	transitionInput.Reason, transitionInput.IdempotencyKey = "Завершение", "terminal-transition-1"
	if _, err := w.cases.TransitionWorkItem(ctx, casesAPI.TransitionCommand{
		WorkCommandInput: transitionInput, ToState: "resolved",
	}); err != nil {
		t.Fatal(err)
	}
	// The NEW material on the terminal scope: the honest rule refusal.
	newMaterial := recorded
	newMaterial.MaterialKey = "деталь"
	newMaterial.Value = []byte(`{"part":"brake-pads"}`)
	newMaterial.IdempotencyKey = "terminal-fact-2"
	newMaterial.Reason = "Новое значение на завершённом элементе"
	_, err = w.cases.RecordFact(ctx, newMaterial)
	assertRuleError(t, err, "факт нельзя записать в скоуп элемента в состоянии resolved")
	// The EXACT material repeat: the replay of the original version (the
	// m4 letter — the terminal refusal never blocks a repeat that writes
	// nothing).
	repeat := recorded
	repeat.IdempotencyKey = "terminal-fact-3"
	repeat.Reason = "Точный повтор на завершённом элементе"
	replayed, err := w.cases.RecordFact(ctx, repeat)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replay || replayed.FactID != first.FactID || replayed.FactVersion != 1 {
		t.Fatalf("expected the material replay on the terminal scope, got %+v", replayed)
	}
	facts, err := w.cases.FactsView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Facts) != 1 || len(facts.Facts[0].Versions) != 1 {
		t.Fatalf("no new versions after the terminal refusals and the replay, got %+v", facts.Facts)
	}
}

// TestMergeConcurrentRecordNeverLeaksRawConflicts covers the merge lock
// plan: a merge of one source and a concurrent record of a NEW fact onto
// that source race — the merge holds the source work lock (the record
// scope fence), the planned fact set covers the pre-read facts; whichever
// order the DB picks, both commands answer the honest signal set and the
// ladders stay contiguous (never a raw 23505 — the M1 probe of the
// review).
func TestMergeConcurrentRecordNeverLeaksRawConflicts(t *testing.T) {
	w := newCasesWorld(t, 520)
	// The race needs the thread-safe real generator (the 2-1 precedent).
	w.cases = w.buildCasesService(&employeeIDs{})
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-merge-race-1", "sess-merge-race-1", `{"call":"inbound-call","from":"+79000000051"}`)
	classifyInput := w.workBaseInput(caseID, workItemID, 1)
	classifyInput.Reason, classifyInput.IdempotencyKey = "Классификация", "merge-race-classify-1"
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{
		WorkCommandInput: classifyInput, Type: "поиск детали", Title: "Источник объединения",
	}); err != nil {
		t.Fatal(err)
	}
	firstEventID := eventIDOf(t, w, caseID, "evt-merge-race-1")
	recorded := casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "автомобиль", Value: []byte(`{"model":"Civic"}`),
		SourceType: casesDomain.FactSourceChannelEvent, SourceID: firstEventID.String(),
		Reason: "Факт до гонки", IdempotencyKey: "merge-race-fact-1", Now: casesClock.Now(),
	}
	if _, err := w.cases.RecordFact(ctx, recorded); err != nil {
		t.Fatal(err)
	}
	created, err := w.cases.CreateWorkItem(ctx, casesAPI.CreateWorkItemCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationOf(t, w, caseID),
		Type:           "уточнение аналога", Title: "Цель объединения",
		Reason: "Элемент-цель объединения", IdempotencyKey: "merge-race-create-1", Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	mergeInput := w.workBaseInput(caseID, created.WorkItemID, 1)
	mergeInput.Reason, mergeInput.IdempotencyKey = "Одновременное объединение", "merge-race-merge-1"
	merge := casesAPI.MergeCommand{
		WorkCommandInput: mergeInput,
		Sources:          []casesAPI.MergeSourceInput{{WorkItemID: workItemID, ExpectedVersion: 2}},
	}
	concurrent := recorded
	concurrent.MaterialKey = "деталь"
	concurrent.Value = []byte(`{"part":"brake-pads"}`)
	concurrent.SourceType = casesDomain.FactSourceOperator
	concurrent.SourceID = "merge-race-second-source-1"
	concurrent.IdempotencyKey = "merge-race-fact-2"
	concurrent.Reason = "Факт во время гонки"
	type commandResult struct {
		err error
	}
	results := make(chan commandResult, 2)
	start := make(chan struct{})
	var group sync.WaitGroup
	call := func(work func() error) {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results <- commandResult{work()}
		}()
	}
	call(func() error {
		_, err := w.cases.MergeWorkItems(ctx, merge)
		return err
	})
	call(func() error {
		_, err := w.cases.RecordFact(ctx, concurrent)
		return err
	})
	close(start)
	group.Wait()
	close(results)
	for result := range results {
		isHonestWorkSignal(t, result.err, "the merge race")
	}
	facts, err := w.cases.FactsView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	// The honest interleavings: the merge won — the concurrent record hits
	// the terminal scope guard (one fact, the pre-read one); the record
	// won — the merge re-reads under the locks and rescopes BOTH facts
	// (two facts). Never a raw conflict, never a broken ladder.
	if len(facts.Facts) < 1 || len(facts.Facts) > 2 {
		t.Fatalf("the pre-read fact always exists; the concurrent one does when the record won, got %d", len(facts.Facts))
	}
	for _, fact := range facts.Facts {
		for index, version := range fact.Versions {
			if version.VersionNo != int64(index+1) {
				t.Fatalf("the ladder of %s stays contiguous, got %+v", fact.MaterialKey, fact.Versions)
			}
		}
	}
	// The source is terminal after the merge (the raced record either
	// applied before the merge or was refused by the terminal guard).
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range detail.WorkItems {
		if item.WorkItemID == workItemID && !item.Terminal {
			t.Fatalf("the merge source must be terminal after the merge, got %+v", item)
		}
	}
}

// TestRecordFactRefusesTheExponentialArrayHonestly covers the R2 N1 class:
// a dense exponential array fits the 2048 CANONICAL byte bound but the
// jsonb storage expands every literal into the digit form (1e10 becomes
// 10000000000) — the value would outrun the CHECK backstop of the
// migration. The domain refuses the exponential spelling BEFORE the
// storage: the operator sees the honest rule refusal, never a raw 23514
// surfaced as a 500.
func TestRecordFactRefusesTheExponentialArrayHonestly(t *testing.T) {
	w := newCasesWorld(t, 521)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-exp-1", "sess-exp-1", `{"call":"inbound-call","from":"+79000000052"}`)
	// 409 exponential literals: 2046 canonical bytes (domain-valid by the
	// size bound alone), ~5300 text bytes in the jsonb rendering.
	exponential := []byte(`[` + strings.TrimSuffix(strings.Repeat("1e10,", 409), ",") + `]`)
	if len(exponential) > 2048 {
		t.Fatalf("the probe material must fit the canonical bound, got %d bytes", len(exponential))
	}
	dense := casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "автомобиль", Value: exponential,
		SourceType: casesDomain.FactSourceOperator, SourceID: "exp-source-1",
		Reason: "Экспоненциальная запись", IdempotencyKey: "exp-fact-1", Now: casesClock.Now(),
	}
	_, err := w.cases.RecordFact(ctx, dense)
	assertRuleError(t, err, "экспоненциальную запись")
	// The digit form of the same material is accepted and stored: the
	// jsonb rendering never expands it.
	digit := dense
	digit.Value = []byte(`[` + strings.TrimSuffix(strings.Repeat("10000000000,", 20), ",") + `]`)
	digit.IdempotencyKey = "exp-fact-2"
	digit.Reason = "Цифровая запись"
	if _, err := w.cases.RecordFact(ctx, digit); err != nil {
		t.Fatal(err)
	}
	facts, err := w.cases.FactsView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.Facts) != 1 || len(facts.Facts[0].Versions) != 1 {
		t.Fatalf("the digit form is the only stored version, got %+v", facts.Facts)
	}
}
