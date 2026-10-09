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

	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// verificationNow is the command clock of the Story 2.3 tests (the release
// clock of the world plus one day — the same relation the 2-2 commands
// use).
var verificationNow = releaseClock.Now().Add(24 * time.Hour)

// verificationCommand builds one canonical verification command input.
func (w *casesWorld) verificationCommand(caseID, conversationID id.UUID, key string, now time.Time) casesAPI.StartVerificationCommand {
	return casesAPI.StartVerificationCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		IdempotencyKey: key, Now: now,
	}
}

// conversationFirst returns the first conversation of the case view.
func (w *casesWorld) conversationFirst(t *testing.T, caseID id.UUID) id.UUID {
	t.Helper()
	detail, err := w.cases.CaseView(context.Background(), w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Conversations) == 0 {
		t.Fatalf("expected a conversation of the case")
	}
	return detail.Conversations[0].ConversationID
}

// TestAssurancePanelAndBindingGate covers the matrix rows of AC-1/AC-2: no
// record — the honest none and the public path unblocked; the binding
// refusal lists the required level and the allowed methods of the channel;
// the knowledge check gives basic (the gate still refuses); the callback
// gives verified; the public path never blocks.
func TestAssurancePanelAndBindingGate(t *testing.T) {
	w := newCasesWorld(t, 560)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-verify-1", "sess-verify-1", `{"call":"inbound-call","from":"+79000000041"}`)
	conversationID := w.conversationFirst(t, caseID)

	// The honest none: the panel shows «не проверена», the public path is
	// unblocked (the work items are visible and manageable).
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Assurance.State.IdentityLevel != casesDomain.AssuranceNone {
		t.Fatalf("expected the honest none, got %s", detail.Assurance.State.IdentityLevel)
	}
	if len(detail.WorkItems) != 1 || detail.CanManageWork != true {
		t.Fatalf("the public path must stay unblocked")
	}

	// The binding gate without a record: the honest refusal names the
	// required level and the allowed methods of the conversation modality —
	// never the values that did not match.
	otherCase, _ := w.acceptedCase(t, "evt-verify-2", "sess-verify-2", `{"call":"inbound-call","from":"+79000000042"}`)
	otherConversation := w.conversationFirst(t, otherCase)
	bindCommand := casesAPI.BindChannelCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		BoundCaseID: otherCase, BoundConversationID: otherConversation,
		Method: casesDomain.BindingMethodConsentHandoff, Purpose: "клиент продолжает в мессенджере",
		SubjectHandle: "позвонивший", Note: "клиент согласился продолжить в WhatsApp",
		IdempotencyKey: "bind-gate-1", Now: verificationNow,
	}
	_, err = w.cases.BindChannel(ctx, bindCommand)
	assertRuleError(t, err, "verified")

	// The knowledge check passes: the panel shows basic, the binding gate
	// still refuses (the ladder is honest).
	knowledge := w.verificationCommand(caseID, conversationID, "verify-knowledge-1", verificationNow)
	knowledge.Kind, knowledge.Method, knowledge.ProviderCode = casesDomain.VerificationKindIdentity, casesDomain.MethodKnowledgeCheck, casesDomain.ProviderCodeOK
	knowledge.SubjectHandle, knowledge.Note = "позвонивший", "клиент назвал номер заказа из карточки"
	outcome, err := w.cases.StartVerification(ctx, knowledge)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Replay || outcome.VerificationOutcome != casesDomain.VerificationPassed || outcome.AssuranceLevel != casesDomain.AssuranceBasic {
		t.Fatalf("expected the passed basic record, got %+v", outcome)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Assurance.State.IdentityLevel != casesDomain.AssuranceBasic {
		t.Fatalf("expected the basic derivation, got %s", detail.Assurance.State.IdentityLevel)
	}
	_, err = w.cases.BindChannel(ctx, bindCommand)
	assertRuleError(t, err, "verified")

	// The record never stores a secret: the note carried the words, the
	// record carries the canonical outcome (the bare code refusal).
	codeNote := w.verificationCommand(caseID, conversationID, "verify-code-1", verificationNow)
	codeNote.Kind, codeNote.Method, codeNote.ProviderCode = casesDomain.VerificationKindIdentity, casesDomain.MethodKnowledgeCheck, casesDomain.ProviderCodeOK
	codeNote.SubjectHandle, codeNote.Note = "позвонивший", "4815162342"
	_, err = w.cases.StartVerification(ctx, codeNote)
	assertRuleError(t, err, "чистым кодом")

	// The callback passes: verified. The binding succeeds; the exact repeat
	// of the verification replays; the exact repeat of the binding replays.
	callback := w.verificationCommand(caseID, conversationID, "verify-callback-1", verificationNow)
	callback.Kind, callback.Method, callback.ProviderCode = casesDomain.VerificationKindIdentity, casesDomain.MethodKnownNumberCallback, casesDomain.ProviderCodeOK
	callback.SubjectHandle, callback.Note = "позвонивший", "перезвонили на номер из факта, абонент подтвердил детали"
	outcome, err = w.cases.StartVerification(ctx, callback)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.AssuranceLevel != casesDomain.AssuranceVerified {
		t.Fatalf("expected the verified record, got %+v", outcome)
	}
	replayed, err := w.cases.StartVerification(ctx, callback)
	if err != nil || !replayed.Replay {
		t.Fatalf("expected the idempotent replay, got %+v, %v", replayed, err)
	}
	bound, err := w.cases.BindChannel(ctx, bindCommand)
	if err != nil {
		t.Fatal(err)
	}
	if bound.Replay || !bound.BindingID.Valid() {
		t.Fatalf("expected the fresh binding, got %+v", bound)
	}
	if !strings.Contains(bound.ConfirmationText, "продолжается") {
		t.Fatalf("the client confirmation must be the deterministic text without internal identifiers: %q", bound.ConfirmationText)
	}
	rebound, err := w.cases.BindChannel(ctx, bindCommand)
	if err != nil || !rebound.Replay {
		t.Fatalf("expected the idempotent binding replay, got %+v, %v", rebound, err)
	}

	// The unified view: the bound conversation and its events join the case
	// view; the case of the bound conversation keeps the ownership; a new
	// event of the bound session attaches by the unchanged 2-1 rule without
	// a new case.
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Conversations) != 2 {
		t.Fatalf("expected the unified view of two conversations, got %d", len(detail.Conversations))
	}
	if len(detail.Timeline) != 2 {
		t.Fatalf("expected the unified timeline of two events, got %d", len(detail.Timeline))
	}
	if len(detail.Bindings) != 1 || detail.Bindings[0].Reversal != nil {
		t.Fatalf("expected one active binding, got %+v", detail.Bindings)
	}
	ingestOutcome, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-verify-3", "sess-verify-2", `{"call":"inbound-call","from":"+79000000042"}`))
	if err != nil {
		t.Fatal(err)
	}
	if accepted := acceptedOutcome(t, ingestOutcome); accepted.NewCase {
		t.Fatalf("the bound session continuation must not create a new case")
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Timeline) != 3 {
		t.Fatalf("expected the unified timeline of three events, got %d", len(detail.Timeline))
	}

	// The work items stay workable across the whole flow (the public path
	// never blocks on the assurance): the classification writes the version
	// 2 of the initial work item.
	classifyInput := w.workBaseInput(caseID, workItemID, 1)
	classifyInput.IdempotencyKey = "verify-classify-1"
	classified, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{
		WorkCommandInput: classifyInput,
		Type:             "поиск детали", Title: "Подбор детали на Civic",
	})
	if err != nil || classified.Version != 2 {
		t.Fatalf("the public path must stay workable: %+v, %v", classified, err)
	}
}

// TestVerificationFailureLadderAndStepUp covers AC-4/AC-10: the failed
// check keeps the assurance unchanged; the three failures inside one hour
// step up (the exception lands, the verifications and the bindings block,
// the public path survives); the window ages out by time alone.
func TestVerificationFailureLadderAndStepUp(t *testing.T) {
	w := newCasesWorld(t, 561)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-verify-10", "sess-verify-10", `{"call":"inbound-call","from":"+79000000010"}`)
	conversationID := w.conversationFirst(t, caseID)
	base := verificationNow

	failure := func(key string, now time.Time) casesAPI.StartVerificationCommand {
		command := w.verificationCommand(caseID, conversationID, key, now)
		command.Kind, command.Method, command.ProviderCode = casesDomain.VerificationKindIdentity, casesDomain.MethodKnowledgeCheck, casesDomain.ProviderCodeNoMatch
		command.SubjectHandle, command.Note = "позвонивший", "клиент не назвал детали заказа"
		return command
	}
	// Two failures: no step-up yet; the assurance stays none.
	for i, key := range []string{"fail-step-1", "fail-step-2"} {
		if _, err := w.cases.StartVerification(ctx, failure(key, base.Add(time.Duration(i)*time.Minute))); err != nil {
			t.Fatal(err)
		}
	}
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Assurance.State.IdentityLevel != casesDomain.AssuranceNone || detail.Assurance.State.StepUp {
		t.Fatalf("two failures must not step up: %+v", detail.Assurance.State)
	}
	// The third failure crosses: the exception lands, the state steps up.
	if _, err := w.cases.StartVerification(ctx, failure("fail-step-3", base.Add(2*time.Minute))); err != nil {
		t.Fatal(err)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, exception := range detail.Exceptions {
		if exception.Kind == casesDomain.ExceptionIdentityStepUp {
			found = true
			if strings.ContainsAny(exception.Summary, "0123456789+") && strings.Contains(exception.Summary, "+79") {
				t.Fatalf("the exception must not carry identifier values")
			}
		}
	}
	if !found {
		t.Fatalf("the threshold crossing must write the identity_step_up exception")
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.Assurance.State.StepUp {
		t.Fatalf("the third failure must step up")
	}
	// The step-up blocks the new verifications AND the bindings; the public
	// path stays alive.
	if _, err := w.cases.StartVerification(ctx, failure("fail-step-4", base.Add(3*time.Minute))); err == nil {
		t.Fatalf("the step-up must refuse the new verifications")
	} else {
		assertRuleError(t, err, "порог")
	}
	// The window ages out: the later clock (50+ minutes ahead of the first
	// failures) releases the attempts — nothing was ever updated.
	later := w.buildCasesServiceClock(newPrivacyIDs(5612), fixedClock{base.Add(70 * time.Minute)})
	command := failure("fail-step-5", base.Add(70*time.Minute))
	command.Actor = w.workActor()
	if _, err := later.StartVerification(ctx, command); err != nil {
		t.Fatalf("the window must age out by time alone: %v", err)
	}
}

// TestAuthorityGateOfOnBehalfBinding covers AC-5: the identity and the
// authority are checked SEPARATELY — a confirmed identity never grants the
// right to act for another person; the authority record carries the scope
// and the expiry.
func TestAuthorityGateOfOnBehalfBinding(t *testing.T) {
	w := newCasesWorld(t, 562)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-verify-20", "sess-verify-20", `{"call":"inbound-call","from":"+79000000020"}`)
	conversationID := w.conversationFirst(t, caseID)
	otherCase, _ := w.acceptedCase(t, "evt-verify-21", "sess-verify-21", `{"call":"inbound-call","from":"+79000000021"}`)
	otherConversation := w.conversationFirst(t, otherCase)

	identity := w.verificationCommand(caseID, conversationID, "verify-auth-1", verificationNow)
	identity.Kind, identity.Method, identity.ProviderCode = casesDomain.VerificationKindIdentity, casesDomain.MethodKnownNumberCallback, casesDomain.ProviderCodeOK
	identity.SubjectHandle, identity.Note = "представитель компании", "перезвонили на известный номер, абонент подтвердил"
	if _, err := w.cases.StartVerification(ctx, identity); err != nil {
		t.Fatal(err)
	}
	// Verified identity WITHOUT the authority record: the on-behalf
	// binding refuses honestly.
	bind := casesAPI.BindChannelCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		BoundCaseID: otherCase, BoundConversationID: otherConversation,
		Method: casesDomain.BindingMethodConsentHandoff, Purpose: "продолжение от имени компании",
		OnBehalf: true, SubjectHandle: "представитель компании", Note: "клиент согласился",
		IdempotencyKey: "bind-auth-1", Now: verificationNow,
	}
	_, err := w.cases.BindChannel(ctx, bind)
	assertRuleError(t, err, "полномочия")

	// The authority record with the scope passes the gate; the panel shows
	// the authority separately from the identity.
	authority := w.verificationCommand(caseID, conversationID, "verify-auth-2", verificationNow)
	authority.Kind, authority.Method, authority.ProviderCode = casesDomain.VerificationKindAuthority, casesDomain.MethodMandateNote, casesDomain.ProviderCodeOK
	authority.SubjectHandle, authority.AuthorityScope = "представитель компании", "заказы компании ACME"
	authority.Note = "письменная доверенность на бланке, скан приложен к письму"
	if _, err := w.cases.StartVerification(ctx, authority); err != nil {
		t.Fatal(err)
	}
	outcome, err := w.cases.BindChannel(ctx, bind)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Replay {
		t.Fatalf("expected the fresh on-behalf binding")
	}
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Assurance.State.AuthorityLevel != casesDomain.AssuranceVerified || detail.Assurance.State.AuthorityScope != "заказы компании ACME" {
		t.Fatalf("the authority derivation must show the scope: %+v", detail.Assurance.State)
	}
	if !detail.Bindings[0].Binding.OnBehalf {
		t.Fatalf("the binding must carry the on-behalf flag")
	}

	// The expired authority: a later-clock read refuses the binding gate
	// (the record stays in the history).
	later := w.buildCasesServiceClock(newPrivacyIDs(5622), fixedClock{verificationNow.Add(31 * 24 * time.Hour)})
	third, _ := w.acceptedCase(t, "evt-verify-22", "sess-verify-22", `{"call":"inbound-call","from":"+79000000022"}`)
	thirdConversation := w.conversationFirst(t, third)
	laterIdentity := identity
	laterIdentity.IdempotencyKey, laterIdentity.Now, laterIdentity.CaseID, laterIdentity.ConversationID = "verify-auth-3", verificationNow.Add(25*time.Hour), third, thirdConversation
	if _, err := later.StartVerification(ctx, laterIdentity); err != nil {
		t.Fatal(err)
	}
	laterBind := bind
	laterBind.IdempotencyKey, laterBind.Now = "bind-auth-2", verificationNow.Add(25*time.Hour)
	laterBind.CaseID, laterBind.ConversationID = third, thirdConversation
	laterBind.BoundCaseID, laterBind.BoundConversationID = otherCase, otherConversation
	_, err = later.BindChannel(ctx, laterBind)
	assertRuleError(t, err, "полномочия")
}

// TestDuplicateCandidatesAndResolution covers AC-6/AC-7: the identifier
// fact tokenizes into the keyed HMAC; the exact match across two open
// cases creates the candidate with the masked minimal material (no
// protected content); the same-client resolution binds atomically; the
// separate decision keeps the cases apart; the replay never duplicates.
func TestDuplicateCandidatesAndResolution(t *testing.T) {
	w := newCasesWorld(t, 563)
	ctx := context.Background()
	caseA, workItemA := w.acceptedCase(t, "evt-verify-30", "sess-verify-30", `{"call":"inbound-call","from":"+79000000030"}`)
	caseB, workItemB := w.acceptedCase(t, "evt-verify-31", "sess-verify-31", `{"call":"inbound-call","from":"+79000000030"}`)
	conversationA := w.conversationFirst(t, caseA)
	conversationB := w.conversationFirst(t, caseB)

	recordPhone := func(caseID, workItemID id.UUID, key string) casesAPI.WorkOutcome {
		outcome, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
			Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			MaterialKey: "phone", Value: []byte(`"+7 (912) 345-67-89"`),
			SourceType: casesDomain.FactSourceOperator, SourceID: key,
			IdempotencyKey: key, Now: verificationNow,
		})
		if err != nil {
			t.Fatal(err)
		}
		return outcome
	}
	recordPhone(caseA, workItemA, "fact-phone-a")
	// Before the second recording there is no candidate.
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseA)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Duplicates) != 0 {
		t.Fatalf("no candidate before the match")
	}
	recordPhone(caseB, workItemB, "fact-phone-b")

	// The candidate: the masked value, both case numbers, the reason; the
	// protected content (payload, facts) is NOT part of the card.
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseB)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Duplicates) != 1 {
		t.Fatalf("expected one duplicate candidate, got %d", len(detail.Duplicates))
	}
	card := detail.Duplicates[0]
	if card.Candidate.CaseIDNew != caseB || card.Candidate.CaseIDExisting != caseA {
		t.Fatalf("candidate sides broken")
	}
	if card.MaskedValue != "•••••••••89" {
		t.Fatalf("the card must carry the masked value only: %q", card.MaskedValue)
	}
	if card.Candidate.Token == "" || len(card.Candidate.Token) != 64 {
		t.Fatalf("the candidate must carry the HMAC token, not the value")
	}
	if strings.Contains(card.Candidate.Token, "79123456789") {
		t.Fatalf("the token must never contain the value")
	}

	// The exact repeat of the fact command replays (no second candidate).
	recordPhone(caseB, workItemB, "fact-phone-b")
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseB)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Duplicates) != 1 {
		t.Fatalf("the replay must not duplicate the candidate, got %d", len(detail.Duplicates))
	}

	// A non-string identifier value: the fact records without a token (the
	// dictionary boundary is honest, never a blocker).
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseB, WorkItemID: workItemB,
		MaterialKey: "email", Value: []byte(`{"box":"x@y.ru"}`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "fact-email-obj",
		IdempotencyKey: "fact-email-obj", Now: verificationNow,
	}); err != nil {
		t.Fatalf("a non-string identifier value must record as an ordinary fact: %v", err)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseB)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Duplicates) != 1 {
		t.Fatalf("the object value must not tokenize")
	}

	// The separate resolution: the cases stay apart; the candidate keeps
	// the decision in the history.
	separate, err := w.cases.ResolveDuplicate(ctx, casesAPI.ResolveDuplicateCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseB, CandidateID: card.Candidate.CandidateID,
		Decision: casesDomain.DuplicateDecisionSeparate, Note: "телефон общий, обращение о другой услуге",
		ConversationID: conversationA, BoundConversationID: conversationB,
		IdempotencyKey: "resolve-sep-1", Now: verificationNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if separate.BindingID.Valid() {
		t.Fatalf("the separate decision never binds")
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseB)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Duplicates) != 1 || detail.Duplicates[0].Resolution == nil || detail.Duplicates[0].Resolution.Decision != casesDomain.DuplicateDecisionSeparate {
		t.Fatalf("the resolution must close the review")
	}
	if len(detail.Conversations) != 1 {
		t.Fatalf("the cases stay separate: %d conversations", len(detail.Conversations))
	}

	// The same-client resolution of a second candidate: the resolution and
	// the binding land in ONE transaction.
	caseC, workItemC := w.acceptedCase(t, "evt-verify-32", "sess-verify-32", `{"call":"inbound-call","from":"+79000000032"}`)
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseC, WorkItemID: workItemC,
		MaterialKey: "phone", Value: []byte(`"+7 (912) 345-67-89"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "fact-phone-c",
		IdempotencyKey: "fact-phone-c", Now: verificationNow,
	}); err != nil {
		t.Fatal(err)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseC)
	if err != nil {
		t.Fatal(err)
	}
	// Two candidates: against A and against B (both hold the token).
	if len(detail.Duplicates) != 2 {
		t.Fatalf("expected two candidates, got %d", len(detail.Duplicates))
	}
	var candidateC casesDomain.DuplicateCandidate
	for _, row := range detail.Duplicates {
		if row.Candidate.CaseIDExisting == caseA {
			candidateC = row.Candidate
		}
	}
	conversationC := w.conversationFirst(t, caseC)
	same, err := w.cases.ResolveDuplicate(ctx, casesAPI.ResolveDuplicateCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseC, CandidateID: candidateC.CandidateID,
		Decision: casesDomain.DuplicateDecisionSameClient, Note: "тот же клиент по словам и истории",
		ConversationID: conversationA, BoundConversationID: conversationC, SubjectHandle: "клиент ACME",
		IdempotencyKey: "resolve-same-1", Now: verificationNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !same.BindingID.Valid() || !strings.Contains(same.ConfirmationText, "объединено") {
		t.Fatalf("the same-client resolution must create the binding: %+v", same)
	}
	// The unified view of A now includes the conversation C.
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseA)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Conversations) != 2 {
		t.Fatalf("the duplicate-review binding must join the views: %d", len(detail.Conversations))
	}
	foundReview := false
	for _, binding := range detail.Bindings {
		if binding.Binding.Method == casesDomain.BindingMethodDuplicateReview && binding.Binding.BoundConversationID == conversationC {
			foundReview = true
		}
	}
	if !foundReview {
		t.Fatalf("the duplicate-review binding must be visible")
	}
	// The second resolution of the same candidate is the honest conflict.
	_, err = w.cases.ResolveDuplicate(ctx, casesAPI.ResolveDuplicateCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseC, CandidateID: candidateC.CandidateID,
		Decision: casesDomain.DuplicateDecisionSeparate, Note: "повторное решение",
		ConversationID: conversationA, BoundConversationID: conversationC,
		IdempotencyKey: "resolve-same-2", Now: verificationNow,
	})
	if !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the second resolution must be the honest conflict, got %v", err)
	}
}

// TestUnlinkKeepsHistoryAndRecomputesViews covers AC-9: the unlink is
// append-only — the events and the access history stay; the unified view
// recomputes (the bound conversation drops out, the ownership never
// moves); the re-binding is a new row.
func TestUnlinkKeepsHistoryAndRecomputesViews(t *testing.T) {
	w := newCasesWorld(t, 564)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-verify-40", "sess-verify-40", `{"call":"inbound-call","from":"+79000000040"}`)
	conversationID := w.conversationFirst(t, caseID)
	otherCase, _ := w.acceptedCase(t, "evt-verify-41", "sess-verify-41", `{"call":"inbound-call","from":"+79000000041"}`)
	otherConversation := w.conversationFirst(t, otherCase)

	identity := w.verificationCommand(caseID, conversationID, "verify-unlink-1", verificationNow)
	identity.Kind, identity.Method, identity.ProviderCode = casesDomain.VerificationKindIdentity, casesDomain.MethodKnownNumberCallback, casesDomain.ProviderCodeOK
	identity.SubjectHandle, identity.Note = "позвонивший", "перезвонили, абонент подтвердил"
	if _, err := w.cases.StartVerification(ctx, identity); err != nil {
		t.Fatal(err)
	}
	bound, err := w.cases.BindChannel(ctx, casesAPI.BindChannelCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		BoundCaseID: otherCase, BoundConversationID: otherConversation,
		Method: casesDomain.BindingMethodConsentHandoff, Purpose: "клиент продолжает в WhatsApp",
		SubjectHandle: "позвонивший", Note: "клиент согласился",
		IdempotencyKey: "bind-unlink-1", Now: verificationNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The unlink: the honest reversal keeps everything.
	unlinked, err := w.cases.UnlinkChannel(ctx, casesAPI.UnlinkChannelCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, BindingID: bound.BindingID,
		Reason:         "связали не тот разговор по ошибке оператора",
		IdempotencyKey: "unlink-1", Now: verificationNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if unlinked.Replay {
		t.Fatalf("expected the fresh reversal")
	}
	replayed, err := w.cases.UnlinkChannel(ctx, casesAPI.UnlinkChannelCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, BindingID: bound.BindingID,
		Reason:         "связали не тот разговор по ошибке оператора",
		IdempotencyKey: "unlink-1", Now: verificationNow,
	})
	if err != nil || !replayed.Replay {
		t.Fatalf("expected the idempotent unlink replay: %+v %v", replayed, err)
	}
	// The views recompute: the case view loses the bound conversation; the
	// events stay in the owner case; the binding row keeps the reversal.
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Conversations) != 1 {
		t.Fatalf("the unlinked conversation must drop out of the unified view, got %d", len(detail.Conversations))
	}
	if len(detail.Bindings) != 1 || detail.Bindings[0].Reversal == nil {
		t.Fatalf("the binding history must keep the reversal")
	}
	ownerDetail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, otherCase)
	if err != nil {
		t.Fatal(err)
	}
	if len(ownerDetail.Timeline) != 1 {
		t.Fatalf("the events stay in the owner case, got %d", len(ownerDetail.Timeline))
	}
	// The second unlink of the same binding is the honest conflict.
	_, err = w.cases.UnlinkChannel(ctx, casesAPI.UnlinkChannelCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, BindingID: bound.BindingID,
		Reason: "повторная отмена", IdempotencyKey: "unlink-2", Now: verificationNow,
	})
	if !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the second reversal must conflict, got %v", err)
	}
	// The re-binding is a new row (the history stays).
	rebound, err := w.cases.BindChannel(ctx, casesAPI.BindChannelCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		BoundCaseID: otherCase, BoundConversationID: otherConversation,
		Method: casesDomain.BindingMethodConsentHandoff, Purpose: "клиент вернулся в WhatsApp",
		SubjectHandle: "позвонивший", Note: "повторное согласие",
		IdempotencyKey: "bind-unlink-2", Now: verificationNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Bindings) != 2 {
		t.Fatalf("the re-binding is a new row: %d", len(detail.Bindings))
	}
	if len(detail.Conversations) != 2 {
		t.Fatalf("the active binding joins again: %d", len(detail.Conversations))
	}
	if rebound.BindingID == bound.BindingID {
		t.Fatalf("the re-binding must be a new row")
	}
}

// TestVerificationCommandsRefuseCrossTenant covers AC-11: RLS and the
// server authorization block the foreign tenant; the answer never confirms
// the existence of the foreign case, conversation or identifier.
func TestVerificationCommandsRefuseCrossTenant(t *testing.T) {
	w := newCasesWorld(t, 565)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-verify-50", "sess-verify-50", `{"call":"inbound-call","from":"+79000000050"}`)
	conversationID := w.conversationFirst(t, caseID)
	foreignTenant := createForeignTenant(t, w)
	foreign := inviteMember(t, w.identity, w.owner, foreignTenant, "verify-foreign-414@example.org", "expert_qa",
		[]identityDomain.Scope{identityDomain.Review}, "000000041444", "000000041444")

	command := w.verificationCommand(caseID, conversationID, "verify-foreign-1", verificationNow)
	command.Kind, command.Method, command.ProviderCode = casesDomain.VerificationKindIdentity, casesDomain.MethodKnowledgeCheck, casesDomain.ProviderCodeOK
	command.SubjectHandle, command.Note = "кто-то", "нота"
	command.Actor = foreign
	// The foreign actor gets the honest 404 (the non-disclosure refusal —
	// the answer never confirms the existence of the foreign case).
	if _, err := w.cases.StartVerification(ctx, command); !errors.Is(err, identityDomain.ErrNotFound) {
		t.Fatalf("the foreign actor must be refused without disclosure, got %v", err)
	}
	bind := casesAPI.BindChannelCommand{
		Actor: foreign, TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		BoundCaseID: caseID, BoundConversationID: conversationID,
		Method: casesDomain.BindingMethodConsentHandoff, Purpose: "x",
		SubjectHandle: "кто-то", Note: "нота",
		IdempotencyKey: "bind-foreign-1", Now: verificationNow,
	}
	if _, err := w.cases.BindChannel(ctx, bind); !errors.Is(err, identityDomain.ErrNotFound) {
		t.Fatalf("the foreign binding must be refused without disclosure, got %v", err)
	}
	// The foreign read: the plain 404 without the tenant disclosure.
	if _, err := w.cases.CaseView(ctx, foreign, w.tenant, caseID); !errors.Is(err, identityDomain.ErrNotFound) {
		t.Fatalf("the foreign read must be the honest 404, got %v", err)
	}
}

// TestConcurrentBindingsSerialize covers the race matrix row: two
// concurrent bind commands of one conversation — exactly one applies, the
// loser answers the honest conflict; the replay never duplicates.
func TestConcurrentBindingsSerialize(t *testing.T) {
	w := newCasesWorld(t, 566)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-verify-60", "sess-verify-60", `{"call":"inbound-call","from":"+79000000060"}`)
	conversationID := w.conversationFirst(t, caseID)
	otherCase, _ := w.acceptedCase(t, "evt-verify-61", "sess-verify-61", `{"call":"inbound-call","from":"+79000000061"}`)
	otherConversation := w.conversationFirst(t, otherCase)

	identity := w.verificationCommand(caseID, conversationID, "verify-race-1", verificationNow)
	identity.Kind, identity.Method, identity.ProviderCode = casesDomain.VerificationKindIdentity, casesDomain.MethodKnownNumberCallback, casesDomain.ProviderCodeOK
	identity.SubjectHandle, identity.Note = "позвонивший", "перезвонили, подтвердил"
	if _, err := w.cases.StartVerification(ctx, identity); err != nil {
		t.Fatal(err)
	}

	// The thread-safe generator + the start-channel-synchronized pair (the
	// 2-2 race pattern).
	racing := w.buildCasesService(newPrivacyIDs(5670))
	results := make([]error, 2)
	start := make(chan struct{})
	var group sync.WaitGroup
	for i := range results {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			_, err := racing.BindChannel(ctx, casesAPI.BindChannelCommand{
				Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
				BoundCaseID: otherCase, BoundConversationID: otherConversation,
				Method: casesDomain.BindingMethodConsentHandoff, Purpose: "гонка связывания",
				SubjectHandle: "позвонивший", Note: "клиент согласился",
				IdempotencyKey: fmt.Sprintf("bind-race-%d", index), Now: verificationNow,
			})
			results[index] = err
		}(i)
	}
	close(start)
	group.Wait()
	failures := 0
	for _, err := range results {
		if err != nil {
			var rule casesDomain.WorkRuleError
			if errors.As(err, &rule) && strings.Contains(rule.Reason, "активная связь") {
				failures++
				continue
			}
			if errors.Is(err, casesDomain.ErrCommandKeyConflict) || errors.Is(err, casesDomain.ErrStaleVersion) {
				failures++
				continue
			}
			t.Fatalf("unexpected race error: %v", err)
		}
	}
	if failures != 1 {
		t.Fatalf("exactly one racing binding must win, %d refused", failures)
	}
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Bindings) != 1 || detail.Bindings[0].Reversal != nil {
		t.Fatalf("exactly one active binding after the race: %+v", detail.Bindings)
	}
}

// TestVerificationRecordsCarryNoSecrets covers the AC-3 storage discipline:
// every column of the verification record is the canonical material — the
// record count and the outcome shape prove no secret column exists (the
// domain has no field to carry one).
func TestVerificationRecordsCarryNoSecrets(t *testing.T) {
	w := newCasesWorld(t, 567)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-verify-70", "sess-verify-70", `{"call":"inbound-call","from":"+79000000070"}`)
	conversationID := w.conversationFirst(t, caseID)
	command := w.verificationCommand(caseID, conversationID, "verify-secret-1", verificationNow)
	command.Kind, command.Method, command.ProviderCode = casesDomain.VerificationKindIdentity, casesDomain.MethodKnowledgeCheck, casesDomain.ProviderCodeOK
	command.SubjectHandle, command.Note = "позвонивший", "клиент подтвердил детали заказа"
	if _, err := w.cases.StartVerification(ctx, command); err != nil {
		t.Fatal(err)
	}
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Assurance.Records) != 1 {
		t.Fatalf("one record expected, got %d", len(detail.Assurance.Records))
	}
	record := detail.Assurance.Records[0]
	if record.Outcome != casesDomain.VerificationPassed || record.Method != casesDomain.MethodKnowledgeCheck {
		t.Fatalf("the canonical outcome shape: %+v", record)
	}
	if record.Note != "клиент подтвердил детали заказа" {
		t.Fatalf("the note is the observed words, not a code: %q", record.Note)
	}
	// The provider code never lands in the record (the canonical outcome
	// replaced it): the record fields are the domain ones only.
	if record.Outcome == casesDomain.ProviderCodeOK {
		t.Fatalf("the provider spelling must never land in the record")
	}
}
