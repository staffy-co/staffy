//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/jackc/pgx/v5/pgconn"
)

// objectionSeedSeq keeps every seeded objection row globally unique across
// the whole suite (the tests run sequentially per package; the ids of the
// direct admin inserts never collide with the command-path ids).
var objectionSeedSeq int64

// The Story 2.8 matrix: the objection record guards (the dictionaries, the
// terminal Work Item, the turn-source boundaries — the voice turn refuses
// honestly: the excerpt markup of the voice contour belongs to the later
// slices, the text-turn happy path lives in the journeys contour where the
// chat fixture connection admits the web_chat text events; the integration
// world carries the only connectable connector — the synthetic telephony,
// AG-4), the rights gate BEFORE every material guard, the tenant
// isolation, the sales-reply objection gate over the seeded append-only
// rows (the persuasion stop with the allowed endings, the mandatory step
// discipline, the acknowledge-first rule, the per-WI isolation) and the
// append-only discipline probes of the database (RLS/grants, the CHECK
// backstops of the dictionaries and the spans shape).

// objectionView reads the objection section of the case view.
func (w *casesWorld) objectionView(t *testing.T, caseID id.UUID) casesAPI.ObjectionView {
	t.Helper()
	detail, err := w.cases.CaseView(context.Background(), w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	return detail.Objections
}

// objectionSectionOf resolves the Work Item section of the objection view.
func objectionSectionOf(t *testing.T, view casesAPI.ObjectionView, workItemID id.UUID) casesAPI.ObjectionWorkItemView {
	t.Helper()
	for _, section := range view.Items {
		if section.WorkItemID == workItemID {
			return section
		}
	}
	t.Fatalf("the objection section of the work item %s not found", workItemID)
	return casesAPI.ObjectionWorkItemView{}
}

// objectionRecord is the record shorthand of the tests (the honest error
// return — the refusals are the matrix rows).
func (w *casesWorld) objectionRecord(t *testing.T, command casesAPI.RecordObjectionCommand) (casesAPI.WorkOutcome, error) {
	t.Helper()
	return w.cases.RecordObjection(context.Background(), command)
}

// objectionRecordCommand builds the record command of the tests over the
// world owner actor.
func (w *casesWorld) objectionRecordCommand(caseID, workItemID, turnRef id.UUID, objectionType, clientSignal, key string) casesAPI.RecordObjectionCommand {
	return casesAPI.RecordObjectionCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		TurnRef: turnRef, ObjectionType: objectionType, ClientSignal: clientSignal,
		Spans: []string{"Мне кажется дорого"}, IdempotencyKey: key, Now: ownershipNow,
	}
}

// seedObjectionRow inserts one append-only objection row directly over the
// live case + work item anchors (the admin role behind the RLS curtain —
// the DB-probe precedent of 2.7). The AG-4 boundary note: the integration
// world's only connectable connector is the synthetic telephony (voice —
// the text turns the record command demands are impossible here), so the
// SALES-GATE matrix rows seed their rows directly — the gate reads the
// append-only rows regardless of the producer; the honest record path over
// the text turns lives in the journeys contour.
func (w *casesWorld) seedObjectionRow(t *testing.T, caseID, workItemID, turnRef id.UUID, no int64, objectionType, clientSignal string, spansJSON string) id.UUID {
	t.Helper()
	if adminPool == nil {
		t.Skip("no admin pool")
	}
	// The row id is globally unique per call (a monotonic counter): the
	// objection_no stays the per-WI number the caller pins, but two work
	// items of one case may both carry №1 — the id must never collide.
	objectionSeedSeq++
	objectionID := id.MustParse(fmt.Sprintf("019a2f4c-3000-7000-8000-%012d", 96000+objectionSeedSeq))
	_, err := adminPool.Exec(context.Background(), `
                insert into cases.objections (tenant_id, case_id, work_item_id, objection_id, objection_no,
                        turn_ref, objection_type, client_signal, spans, playbook_version, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11,$12)`,
		w.tenant, caseID, workItemID, objectionID, no, turnRef, objectionType, clientSignal, spansJSON,
		casesDomain.ObjectionPlaybookVersion, w.ownershipActor().ID, ownershipNow)
	if err != nil {
		t.Fatal(err)
	}
	return objectionID
}

// TestObjectionRecordDictionaryGuardsAndTerminal covers the honest refusals
// of the record command that never reach the turn source: the type or the
// signal outside the dictionary refuses with the full dictionary words
// (the guard fires BEFORE the material state — nothing is written); the
// terminal Work Item refuses with the work-rule text (the write-command
// boundary of 2-2).
func TestObjectionRecordDictionaryGuardsAndTerminal(t *testing.T) {
	w := newCasesWorld(t, 960)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-objection-1", "sess-objection-1", `{"call":"inbound-call","from":"+79000000130"}`)
	turn := w.modeTurnEvent(t, caseID, "sess-objection-1", "evt-objection-1-turn-1", `{"say":"Мне кажется дорого"}`)

	// The type outside the dictionary: the refusal lists the full type
	// dictionary (the machine-readable 400).
	_, err := w.objectionRecord(t, w.objectionRecordCommand(caseID, workItemID, turn, "magic_type", casesDomain.ClientSignalConcern, "objection-type-1"))
	var rule casesDomain.WorkRuleError
	if !errors.As(err, &rule) {
		t.Fatalf("expected the honest rule refusal, got %v", err)
	}
	for _, word := range casesDomain.ObjectionTypes {
		if !strings.Contains(rule.Reason, "«"+word+"»") {
			t.Fatalf("the refusal lists the type word %q, got %q", word, rule.Reason)
		}
	}
	// The signal outside the dictionary: the same honest shape.
	_, err = w.objectionRecord(t, w.objectionRecordCommand(caseID, workItemID, turn, casesDomain.ObjectionTypeTrust, "magic_signal", "objection-signal-1"))
	if !errors.As(err, &rule) {
		t.Fatalf("expected the honest rule refusal, got %v", err)
	}
	for _, word := range casesDomain.ClientSignals {
		if !strings.Contains(rule.Reason, "«"+word+"»") {
			t.Fatalf("the refusal lists the signal word %q, got %q", word, rule.Reason)
		}
	}
	// The span discipline: no spans, empty spans, duplicates.
	command := w.objectionRecordCommand(caseID, workItemID, turn, casesDomain.ObjectionTypeTrust, casesDomain.ClientSignalConcern, "objection-spans-1")
	command.Spans = nil
	assertRuleError(t, w.objectionRecordErr(t, command), "хотя бы одну дословную выдержку")
	command.Spans = []string{"дубль", "дубль"}
	assertRuleError(t, w.objectionRecordErr(t, command), "не повторяются")
	// Nothing was written: the refusals stay honest.
	if section := objectionSectionOf(t, w.objectionView(t, caseID), workItemID); section.ObjectionCount != 0 {
		t.Fatalf("the refused dictionaries never write, got %d rows", section.ObjectionCount)
	}

	// The terminal Work Item: classify, resolve, then the honest
	// refusal of the record (the resolved state is final).
	classify := w.workBaseInput(caseID, workItemID, 1)
	classify.Reason, classify.IdempotencyKey = "Классификация", "objection-classify-1"
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{WorkCommandInput: classify, Type: "поиск детали", Title: "Подбор детали"}); err != nil {
		t.Fatal(err)
	}
	resolve := w.workBaseInput(caseID, workItemID, 2)
	resolve.Reason, resolve.IdempotencyKey = "Решено", "objection-resolve-1"
	if _, err := w.cases.TransitionWorkItem(ctx, casesAPI.TransitionCommand{WorkCommandInput: resolve, ToState: "resolved"}); err != nil {
		t.Fatal(err)
	}
	_, err = w.objectionRecord(t, w.objectionRecordCommand(caseID, workItemID, turn, casesDomain.ObjectionTypeTrust, casesDomain.ClientSignalConcern, "objection-terminal-1"))
	assertRuleError(t, err, "Терминальный элемент работы не принимает возражений")
}

// objectionRecordErr returns the record error only (the refusal shorthand
// of the dictionary probes).
func (w *casesWorld) objectionRecordErr(t *testing.T, command casesAPI.RecordObjectionCommand) error {
	t.Helper()
	_, err := w.cases.RecordObjection(context.Background(), command)
	return err
}

// TestObjectionTurnSourceBoundaries covers the turn-source rows of the
// matrix: the voice turn refuses honestly (the excerpt markup of the voice
// contour belongs to the later slices — the honest 400 dictionary of the
// modality boundary); the missing turn of the case refuses with the
// turn-not-found rule word. The text-turn acceptance path (the span
// containment, the record landing) lives in the journeys contour of the
// chat fixture connection.
func TestObjectionTurnSourceBoundaries(t *testing.T) {
	w := newCasesWorld(t, 961)
	caseID, workItemID := w.acceptedCase(t, "evt-objection-2", "sess-objection-2", `{"call":"inbound-call","from":"+79000000131"}`)
	turn := w.modeTurnEvent(t, caseID, "sess-objection-2", "evt-objection-2-turn-1", `{"say":"у конкурента дешевле"}`)

	// The voice turn: the honest refusal of the voice excerpt markup.
	_, err := w.objectionRecord(t, w.objectionRecordCommand(caseID, workItemID, turn, casesDomain.ObjectionTypePriceComparison, casesDomain.ClientSignalConcern, "objection-voice-1"))
	assertRuleError(t, err, "Разметка выдержек голосовых сообщений")

	// The missing turn: the turn-not-found rule word.
	_, err = w.objectionRecord(t, w.objectionRecordCommand(caseID, workItemID, id.MustParse("01994f64-6c00-7000-8000-00000000dead"), casesDomain.ObjectionTypeTrust, casesDomain.ClientSignalConcern, "objection-missing-1"))
	assertRuleError(t, err, "Событие turn_ref не найдено")

	// Nothing was written.
	if section := objectionSectionOf(t, w.objectionView(t, caseID), workItemID); section.ObjectionCount != 0 {
		t.Fatalf("the refused turn sources never write, got %d rows", section.ObjectionCount)
	}
}

// TestObjectionCommandsRefuseAuditOnlyMember covers the rights gate of the
// record command (the M1 precedent of 2.5/2.7): an audit-only member reads
// the case view but never records the objections — the refusal is the
// honest 403 forbidden word with the scope text BEFORE every material
// guard. The composite probes pin the gate order live: neither the type
// outside the dictionary nor the terminal Work Item masks the scope
// refusal (the review R1 mutation basis of 2-7), nothing is ever written.
func TestObjectionCommandsRefuseAuditOnlyMember(t *testing.T) {
	w := newCasesWorld(t, 962)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-objection-3", "sess-objection-3", `{"call":"inbound-call","from":"+79000000132"}`)
	turn := w.modeTurnEvent(t, caseID, "sess-objection-3", "evt-objection-3-turn-1", `{"say":"дорого"}`)
	// The member ids avoid the world suffixes (the owner is seed*10+1,
	// the backup is seed*10+8): seed*10+6/9 stay free.
	auditor := inviteMember(t, w.identity, w.readinessActor(true), w.tenant, "objection-auditor-962@example.org", "expert_qa", []identityDomain.Scope{identityDomain.Audit}, "000000009626", "000000009629")
	auditOnly := actorPrincipal{principalID: auditor.ID, stepUp: true}.principal()

	record := func(objectionType string, turnRef id.UUID) error {
		_, err := w.cases.RecordObjection(ctx, casesAPI.RecordObjectionCommand{
			Actor: auditOnly, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			TurnRef: turnRef, ObjectionType: objectionType, ClientSignal: casesDomain.ClientSignalConcern,
			Spans: []string{"дорого"}, IdempotencyKey: "objection-audit-1", Now: ownershipNow,
		})
		return err
	}
	assertScopeRefusal := func(err error, probe string) {
		t.Helper()
		if !errors.Is(err, identityDomain.ErrForbidden) || !strings.Contains(err.Error(), "review или configuration") {
			t.Fatalf("%s: expected the honest 403 forbidden refusal with the scope words, got %v", probe, err)
		}
	}

	// The healthy dictionary of the alive Work Item: the plain scope
	// refusal.
	assertScopeRefusal(record(casesDomain.ObjectionTypeTrust, turn), "the healthy record")
	// The type outside the dictionary never masks the scope words: the
	// rights gate fires FIRST (before the dictionary guard).
	assertScopeRefusal(record("magic_type", turn), "the type outside the dictionary")

	// The terminal Work Item never masks the scope words either.
	classify := w.workBaseInput(caseID, workItemID, 1)
	classify.Reason, classify.IdempotencyKey = "Классификация", "objection-audit-classify-1"
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{WorkCommandInput: classify, Type: "поиск детали", Title: "Подбор детали"}); err != nil {
		t.Fatal(err)
	}
	resolve := w.workBaseInput(caseID, workItemID, 2)
	resolve.Reason, resolve.IdempotencyKey = "Решено", "objection-audit-resolve-1"
	if _, err := w.cases.TransitionWorkItem(ctx, casesAPI.TransitionCommand{WorkCommandInput: resolve, ToState: "resolved"}); err != nil {
		t.Fatal(err)
	}
	assertScopeRefusal(record(casesDomain.ObjectionTypeTrust, turn), "the terminal Work Item")

	// Nothing landed: the objection surface stays clean.
	if section := objectionSectionOf(t, w.objectionView(t, caseID), workItemID); section.ObjectionCount != 0 {
		t.Fatalf("the audit-only member writes nothing, got %d rows", section.ObjectionCount)
	}
}

// TestObjectionForeignTenantIsolation covers the tenant isolation: the
// record command of the foreign tenant sees the plain 404 (the existence
// is never disclosed — the RLS + the server-side authorization).
func TestObjectionForeignTenantIsolation(t *testing.T) {
	w := newCasesWorld(t, 963)
	caseID, workItemID := w.acceptedCase(t, "evt-objection-4", "sess-objection-4", `{"call":"inbound-call","from":"+79000000133"}`)
	turn := w.modeTurnEvent(t, caseID, "sess-objection-4", "evt-objection-4-turn-1", `{"say":"не хочу"}`)

	_, err := w.cases.RecordObjection(context.Background(), casesAPI.RecordObjectionCommand{
		Actor: w.ownershipActor(), TenantID: id.MustParse("01994f64-6c00-7000-8000-0000000000f1"),
		CaseID: caseID, WorkItemID: workItemID, TurnRef: turn,
		ObjectionType: casesDomain.ObjectionTypeTrust, ClientSignal: casesDomain.ClientSignalConcern,
		Spans: []string{"не хочу"}, IdempotencyKey: "objection-foreign-1", Now: ownershipNow,
	})
	if !errors.Is(err, casesDomain.ErrNotFound) && !errors.Is(err, identityDomain.ErrNotFound) {
		t.Fatalf("expected the honest not-found of the foreign tenant, got %v", err)
	}
}

// TestObjectionSalesGateDiscipline covers the sales-reply objection gate of
// Story 2.8 over the seeded append-only rows (the AG-4 note of
// seedObjectionRow): the frozen ladder order (ownership 2-4 → mode 2-5 →
// disclosure 2-5 → policy 2-6 → objection 2-8 → write — the prelude of 2-6
// opens the sales path); the first linked answer MUST acknowledge the
// concern (the refusal names the honest rule); the acknowledged state
// allows every step of the playbook of the type; the step outside the
// playbook of the type refuses; the objectionless Work Item keeps the
// byte-identical old path (the empty step passes).
func TestObjectionSalesGateDiscipline(t *testing.T) {
	w := newCasesWorld(t, 964)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-objection-5", "sess-objection-5", `{"call":"inbound-call","from":"+79000000134"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-objection-5", "evt-objection-5-turn-1", `{"say":"сколько стоит"}`)
	w.policySalesPrelude(t, caseID, conversationID, turn, "objection-sales-1")
	w.policyPrelude(t, caseID, conversationID, workItemID, "objection-policy-1")

	// One CURRENT objection (concern) on the Work Item.
	w.seedObjectionRow(t, caseID, workItemID, turn, 1, casesDomain.ObjectionTypePriceComparison, casesDomain.ClientSignalConcern, `["Мне кажется дорого"]`)

	reply := func(step string, key string) error {
		_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
			WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
			PlaybookStep: step, Content: "Ответ по возражению", IdempotencyKey: key, Now: ownershipNow,
		})
		return err
	}

	// The sales reply WITHOUT the step against a current objection: the
	// discipline is mandatory (AC-4).
	assertRuleError(t, reply("", "objection-nostep-1"), "дисциплина ответа не опциональна")
	// The FIRST linked answer with present_evidence: «сначала признаёт
	// смысл опасения» (AC-4).
	assertRuleError(t, reply(casesDomain.PlaybookStepPresentEvidence, "objection-evidence-1"), "Сначала отправьте acknowledge_concern")
	// The step outside the playbook of the type: the honest refusal
	// (price_comparison carries no clarify_question).
	assertRuleError(t, reply(casesDomain.PlaybookStepClarifyQuestion, "objection-clarify-1"), "последовательность типа возражения")
	// The step word outside the dictionary: the dictionary refusal.
	assertRuleError(t, reply("magic_step", "objection-magic-1"), "не входит в словарь")
	// The first honest acknowledge passes: the reply carries the link
	// (the server-side truth — the objection id).
	acknowledged, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		PlaybookStep: casesDomain.PlaybookStepAcknowledgeConcern,
		Content:      "Понимаю, что цена важна", IdempotencyKey: "objection-ack-1", Now: ownershipNow,
	})
	if err != nil || acknowledged.ReplyState != "prepared" {
		t.Fatalf("expected the acknowledging sales reply to pass, got %+v %v", acknowledged, err)
	}
	if !acknowledged.ObjectionID.Valid() || acknowledged.PlaybookStep != casesDomain.PlaybookStepAcknowledgeConcern {
		t.Fatalf("the reply carries the objection link and the step word, got %+v", acknowledged)
	}
	// The acknowledged state: every step of the playbook of the type
	// passes («затем доказательство ИЛИ честное отсутствие ответа»).
	for _, step := range casesDomain.PlaybookStepsOf(casesDomain.ObjectionTypePriceComparison) {
		if _, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
			WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
			PlaybookStep: step, Content: "Шаг playbook", IdempotencyKey: "objection-step-" + step, Now: ownershipNow,
		}); err != nil {
			t.Fatalf("the acknowledged state allows the playbook step %q, got %v", step, err)
		}
	}
	// The second Work Item of the case without objections: the empty
	// step keeps the byte-identical old path (the control branch).
	second := w.createWorkItemFor(t, caseID, conversationID, "objection-second-wi-1")
	w.policyPrelude(t, caseID, conversationID, second, "objection-second-policy-1")
	free, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: second, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Ответ без возражений", IdempotencyKey: "objection-free-1", Now: ownershipNow,
	})
	if err != nil || free.ReplyState != "prepared" || free.ObjectionID.Valid() {
		t.Fatalf("expected the objectionless sales reply to pass the old path, got %+v %v", free, err)
	}
}

// createWorkItemFor creates one extra Work Item of the case (the isolation
// probe of the per-WI budget).
func (w *casesWorld) createWorkItemFor(t *testing.T, caseID, conversationID id.UUID, key string) id.UUID {
	t.Helper()
	outcome, err := w.cases.CreateWorkItem(context.Background(), casesAPI.CreateWorkItemCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Type: "поиск детали", Title: "Второй элемент", Reason: "Изоляция возражений",
		IdempotencyKey: key, Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return outcome.WorkItemID
}

// TestObjectionSalesGateStopsPersuasion covers the persuasion stop of the
// sales reply (AC-7): the lifetime budget of two refusal records stops the
// persuasion for good — the refusal is the typed ObjectionStoppedError
// with the budget words and the allowed endings (the neutral completion,
// the protected request, the human handoff); the stop signal of the
// CURRENT objection stops immediately (the first record is enough); the
// neutral and the protected replies pass their own gate order (the
// byte-identical paths); the isolation — the objectionless Work Item of
// the same case still accepts the sales replies.
func TestObjectionSalesGateStopsPersuasion(t *testing.T) {
	w := newCasesWorld(t, 965)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-objection-6", "sess-objection-6", `{"call":"inbound-call","from":"+79000000135"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-objection-6", "evt-objection-6-turn-1", `{"say":"не буду брать"}`)
	w.policySalesPrelude(t, caseID, conversationID, turn, "objection-stop-1")
	w.policyPrelude(t, caseID, conversationID, workItemID, "objection-stop-policy-1")
	second := w.createWorkItemFor(t, caseID, conversationID, "objection-stop-second-1")
	w.policyPrelude(t, caseID, conversationID, second, "objection-stop-policy-2")

	// Two refusal records of the Work Item: the lifetime budget is
	// spent — the persuasion stops (AC-7).
	w.seedObjectionRow(t, caseID, workItemID, turn, 1, casesDomain.ObjectionTypeTrust, casesDomain.ClientSignalRefusal, `["Не буду брать"]`)
	w.seedObjectionRow(t, caseID, workItemID, turn, 2, casesDomain.ObjectionTypeTrust, casesDomain.ClientSignalRefusal, `["Не буду брать"]`)

	salesReply := func(workItem id.UUID, key string) error {
		_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
			WorkItemID: workItem, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
			PlaybookStep: casesDomain.PlaybookStepAcknowledgeConcern,
			Content:      "Понимаю", IdempotencyKey: key, Now: ownershipNow,
		})
		return err
	}

	// The stopped sales reply: the typed refusal with the budget words
	// and the allowed endings (the machine-readable 400 basis of the
	// transport).
	err := salesReply(workItemID, "objection-stopped-1")
	var stopped casesDomain.ObjectionStoppedError
	if !errors.As(err, &stopped) {
		t.Fatalf("expected the ObjectionStoppedError, got %v", err)
	}
	if stopped.Refusals != 2 || stopped.Threshold != casesDomain.PersuasionThresholdV1 {
		t.Fatalf("the refusal carries the budget words, got refusals=%d threshold=%d", stopped.Refusals, stopped.Threshold)
	}
	for _, ending := range []string{"вежливое завершение", "запрос приватности", "передача человеку"} {
		if !strings.Contains(stopped.Reason, ending) {
			t.Fatalf("the refusal names the allowed ending %q, got %q", ending, stopped.Reason)
		}
	}

	// The neutral reply of the same Work Item passes its own gate order
	// (the byte-identical path — the stop closes only the sales class).
	neutral, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
		Content: "Хорошо, завершаем", IdempotencyKey: "objection-neutral-1", Now: ownershipNow,
	})
	if err != nil || neutral.ReplyState != "prepared" {
		t.Fatalf("expected the neutral completion to pass, got %+v %v", neutral, err)
	}

	// The isolation (AC-10): the objectionless Work Item of the same
	// case accepts the sales replies (the budget is per-WI).
	isolated, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: second, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Ответ по другому элементу", IdempotencyKey: "objection-isolated-1", Now: ownershipNow,
	})
	if err != nil || isolated.ReplyState != "prepared" {
		t.Fatalf("expected the isolated sales reply to pass, got %+v %v", isolated, err)
	}

	// The view derivation: the stopped word with the budget numbers of
	// the panel (the read-time derivation — never stored).
	section := objectionSectionOf(t, w.objectionView(t, caseID), workItemID)
	if !section.Stopped || section.Refusals != 2 || section.Current == nil || section.Current.Record.ObjectionNo != 2 {
		t.Fatalf("the panel derivation shows the stopped budget, got %+v", section)
	}
	if isolatedSection := objectionSectionOf(t, w.objectionView(t, caseID), second); isolatedSection.Stopped || isolatedSection.ObjectionCount != 0 {
		t.Fatalf("the objectionless Work Item stays active, got %+v", isolatedSection)
	}

	// The immediate stop signal: stop_intent of the CURRENT objection
	// stops the FIRST record alone (the second Work Item path).
	stopTurn := w.modeTurnEvent(t, caseID, "sess-objection-6", "evt-objection-6-turn-2", `{"say":"не звоните больше"}`)
	w.seedObjectionRow(t, caseID, second, stopTurn, 1, casesDomain.ObjectionTypeGeneralDoubt, casesDomain.ClientSignalStopIntent, `["Не звоните больше"]`)
	err = salesReply(second, "objection-stop-intent-1")
	if !errors.As(err, &stopped) || stopped.CurrentSignal != casesDomain.ClientSignalStopIntent || stopped.Refusals != 0 {
		t.Fatalf("the stop signal stops immediately before the budget, got %+v", stopped)
	}
}

// TestObjectionTablesGrantsAndRLS asserts the append-only discipline of the
// objection surface at the database level: RLS + FORCE on the table,
// INSERT/SELECT for staffy_app and no UPDATE, DELETE or TRUNCATE (the
// observation precedent of 2.7).
func TestObjectionTablesGrantsAndRLS(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
                select relname, relrowsecurity, relforcerowsecurity from pg_class
                where relnamespace = 'cases'::regnamespace
                  and relname in ('objections')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := map[string][2]bool{}
	for rows.Next() {
		var name string
		var rls, force bool
		if err = rows.Scan(&name, &rls, &force); err != nil {
			t.Fatal(err)
		}
		seen[name] = [2]bool{rls, force}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if state, ok := seen["objections"]; !ok || !state[0] || !state[1] {
		t.Fatalf("objections must carry RLS + FORCE (the append-only discipline), got %+v", state)
	}
	for _, grant := range []string{"INSERT", "SELECT", "UPDATE", "DELETE", "TRUNCATE"} {
		var allowed bool
		if err := runtimePool.QueryRow(ctx, `select has_table_privilege('staffy_app', $1, $2)`, "cases.objections", grant).Scan(&allowed); err != nil {
			t.Fatal(err)
		}
		if (grant == "INSERT" || grant == "SELECT") != allowed {
			t.Fatalf("%s on cases.objections: allowed=%v, want %v", grant, allowed, grant == "INSERT" || grant == "SELECT")
		}
	}
}

// TestObjectionDBConstraintsRefuseDishonestRows pins the database
// backstops of the append-only row: the dictionary words of the row (the
// type, the signal) are the CHECK constraints; the spans stay a
// non-empty bounded array; the playbook step word of the client replies is
// the CHECK dictionary — the honest shape is a database guarantee, not
// only a domain guard (the review R1 mutation lesson of 2-7: the
// invariants are pinned by the live probes immediately). The direct
// INSERTs run as the admin role behind the RLS curtain.
func TestObjectionDBConstraintsRefuseDishonestRows(t *testing.T) {
	if adminPool == nil {
		t.Skip("no admin pool")
	}
	w := newCasesWorld(t, 966)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-objection-7", "sess-objection-7", `{"call":"inbound-call","from":"+79000000136"}`)
	turn := w.modeTurnEvent(t, caseID, "sess-objection-7", "evt-objection-7-turn-1", `{"say":"сомневаюсь"}`)

	// insert builds the direct INSERT of one objection row over the
	// real case + work item anchors.
	insert := func(no int64, objectionType, clientSignal, spans string) error {
		_, err := adminPool.Exec(ctx, `
                        insert into cases.objections (tenant_id, case_id, work_item_id, objection_id, objection_no,
                                turn_ref, objection_type, client_signal, spans, playbook_version, actor_id, created_at)
                        values ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11,$12)`,
			w.tenant, caseID, workItemID,
			fmt.Sprintf("019a2f4c-3000-7000-8000-%012d", 97000+no), no, turn,
			objectionType, clientSignal, spans,
			casesDomain.ObjectionPlaybookVersion, w.ownershipActor().ID, ownershipNow)
		return err
	}
	checkRefused := func(probe string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: the dishonest row must be refused, got accepted", probe)
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.SQLState() != "23514" {
			t.Fatalf("%s: expected the check_violation 23514, got %v", probe, err)
		}
	}

	// The honest row lands: the template is valid, so every refusal
	// below isolates exactly its dishonest part.
	if err := insert(1, casesDomain.ObjectionTypeTrust, casesDomain.ClientSignalConcern, `["Сомневаюсь"]`); err != nil {
		t.Fatalf("the honest objection row must land, got %v", err)
	}
	// The dictionary words of the row are the CHECK backstops.
	checkRefused("type outside the dictionary", insert(2, "magic_type", casesDomain.ClientSignalConcern, `["слова"]`))
	checkRefused("signal outside the dictionary", insert(3, casesDomain.ObjectionTypeTrust, "magic_signal", `["слова"]`))
	// The spans stay a non-empty array of at most eight elements.
	checkRefused("empty spans array", insert(4, casesDomain.ObjectionTypeTrust, casesDomain.ClientSignalConcern, `[]`))
	checkRefused("spans not an array", insert(5, casesDomain.ObjectionTypeTrust, casesDomain.ClientSignalConcern, `"слова"`))
	many := make([]string, 0)
	for i := 0; i < 9; i++ {
		many = append(many, fmt.Sprintf("выдержка %d", i))
	}
	manyJSON, marshalErr := json.Marshal(many)
	if marshalErr != nil {
		t.Fatalf("the nine-span probe must marshal: %v", marshalErr)
	}
	checkRefused("more than eight spans", insert(6, casesDomain.ObjectionTypeTrust, casesDomain.ClientSignalConcern, string(manyJSON)))
	// The playbook step word of the client replies is the CHECK
	// dictionary (the nullable column — only the sales rows carry
	// values; the honest NULL stays legal).
	insertReply := func(step any) error {
		// The reply ids stay unique per probe (the shared counter); the
		// channel is the literal phone word of the demo contour (the
		// conversations row carries no channel column — the real store
		// resolves the live channel; the probe pins the same demo word).
		objectionSeedSeq++
		_, err := adminPool.Exec(ctx, `
                        insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id, owner_kind, owner_ref, epoch, writable_surface, connection_id, provider, channel, direction, employee_id, decision_no, profile_version, content, actor_id, reply_class, work_item_id, policy_outcome_id, objection_id, playbook_step, created_at)
                        select $1,$2,conversation_id,$3,'human',$4,response_owner_epoch,writable_surface,connection_id,provider,'phone','outbound',employee_id,1,1,'Ответ',$5,'sales',$6,NULL,NULL,$7,$8
                        from cases.conversations where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
			w.tenant, caseID,
			fmt.Sprintf("00000000-0000-0000-0000-%012d", 97500+objectionSeedSeq), w.ownershipActor().ID,
			w.ownershipActor().ID, workItemID, step, ownershipNow)
		return err
	}
	if err := insertReply(nil); err != nil {
		t.Fatalf("the honest reply row without a step must land, got %v", err)
	}
	if err := insertReply(casesDomain.PlaybookStepAcknowledgeConcern); err != nil {
		t.Fatalf("the dictionary step word must land, got %v", err)
	}
	checkRefused("step outside the dictionary", insertReply("magic_step"))
}
