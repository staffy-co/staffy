//go:build integration

package integration_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.5 matrix: the mode ladder, the disclosure gate, the exit
// authority, the per-Work-Item contexts, the idempotency and the races —
// over the real world of the cases contour.

// modeView reads the mode section of the case view.
func (w *casesWorld) modeView(t *testing.T, caseID id.UUID) casesAPI.ModeView {
	t.Helper()
	detail, err := w.cases.CaseView(context.Background(), w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	return detail.Mode
}

// modeTurnEvent ingests one additional accepted event of the SAME
// conversation (the session continuation word) and returns its id — the
// turn reference of the turn signals.
func (w *casesWorld) modeTurnEvent(t *testing.T, caseID id.UUID, sessionID, eventID, payload string) id.UUID {
	t.Helper()
	before := len(w.modeView(t, caseID).Terms)
	_ = before
	outcome, err := w.cases.IngestEvent(context.Background(), w.ingestCommand(eventID, sessionID, payload))
	if err != nil {
		t.Fatal(err)
	}
	result := acceptedOutcome(t, outcome)
	if result.CaseID != caseID {
		t.Fatalf("the session continuation stays on the case, got %+v", result)
	}
	return result.EventID
}

// policyPrelude is the Story 2.6 mechanical extension of the passing
// sales replies of 2-5 (the honest Change Log entry of the spec): one
// proposal for the Work Item and one evaluation with the current anchors
// — the commercial outcome that opens the sales path of the reply (the
// gate needs the Work Item anchor AND the acting Policy Outcome; the
// 2-5 test bodies themselves are not changed — only the prelude).
func (w *casesWorld) policyPrelude(t *testing.T, caseID, conversationID, workItemID id.UUID, key string) casesAPI.WorkOutcome {
	t.Helper()
	detail, err := w.cases.CaseView(context.Background(), w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	turnRef := id.UUID("")
	for _, event := range detail.Timeline {
		if event.State == casesDomain.EventStateAccepted {
			turnRef = event.EventID
			break
		}
	}
	if !turnRef.Valid() {
		t.Fatalf("the case carries no accepted turn for the proposal")
	}
	proposal, err := w.cases.SubmitAIProposal(context.Background(), casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turnRef,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Предлагаю деталь",
		Proposer:       casesDomain.ProposerSyntheticPort,
		IdempotencyKey: key + "-proposal", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	version := int64(0)
	for _, item := range detail.WorkItems {
		if item.WorkItemID == workItemID {
			version = item.Version
		}
	}
	outcome, err := w.cases.EvaluatePolicy(context.Background(), casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: proposal.ProposalID,
		ExpectedModeVersion: detail.Policy.ModeVersion, ExpectedOwnerEpoch: detail.Policy.OwnerEpoch,
		ExpectedFactsDigest: detail.Policy.FactsDigest, ExpectedWorkItemVersion: version,
		IdempotencyKey: key + "-evaluate", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.OutcomeClass != casesDomain.OutcomeCommercialAnswer {
		t.Fatalf("expected the commercial outcome of the prelude, got %+v", outcome)
	}
	return outcome
}

// proposeMode is the command shorthand of the tests.
func (w *casesWorld) proposeMode(t *testing.T, command casesAPI.ProposeModeCommand) casesAPI.WorkOutcome {
	t.Helper()
	outcome, err := w.cases.ProposeMode(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// TestModeIngestWritesInitialTerm covers the admission matrix row: the
// SAME transaction writes the initial mode term — the version 1, the
// unclassified word (the 2-1 canon «не классифицирован», the safe
// non-sales path), the ingest origin, the rule version; the sales reply
// is impossible until a classification raises the mode (AC-5).
func TestModeIngestWritesInitialTerm(t *testing.T) {
	w := newCasesWorld(t, 730)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-mode-1", "sess-mode-1", `{"call":"inbound-call","from":"+79000000063"}`)
	conversationID := w.conversationFirst(t, caseID)

	view := w.modeView(t, caseID)
	if len(view.Terms) != 1 {
		t.Fatalf("expected exactly one initial mode term, got %d", len(view.Terms))
	}
	term := view.Terms[0]
	if term.ModeVersion != 1 || term.Mode != casesDomain.ModeUnclassified || term.Scope != casesDomain.ModeScopeConversation {
		t.Fatalf("expected the version 1 unclassified conversation term, got %+v", term)
	}
	if term.Origin != casesDomain.ModeOriginIngestInitial || term.RuleVersion != casesDomain.ModePolicyVersion {
		t.Fatalf("expected the ingest origin and the rule version, got %+v", term)
	}
	if view.Active == nil || view.Active.Mode != casesDomain.ModeUnclassified {
		t.Fatalf("expected the active unclassified derivation, got %+v", view.Active)
	}
	if !strings.Contains(view.ForbiddenText, "недоступны") {
		t.Fatalf("expected the honest forbidden word of the non-sales path, got %q", view.ForbiddenText)
	}

	// The sales reply under the unclassified mode: the machine-readable
	// mode gate refusal with the fail-closed non-sales word (AC-5).
	_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Продам всё", IdempotencyKey: "mode-sales-unclassified-1", Now: ownershipNow,
	})
	var modeGate casesDomain.ReplyModeError
	if !errors.As(err, &modeGate) || modeGate.EffectiveMode != casesDomain.ModeUnclassified {
		t.Fatalf("expected the fail-closed unclassified mode refusal, got %v", err)
	}

	// The neutral reply passes under any mode (the explicit v1 rule — the
	// safe independent work, AC-8).
	neutral, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
		Content: "Добрый день! Ваше обращение принято в работу.", IdempotencyKey: "mode-neutral-1", Now: ownershipNow,
	})
	if err != nil || neutral.ReplyState != "prepared" {
		t.Fatalf("expected the neutral reply to pass, got %+v %v", neutral, err)
	}

	// The reply class is the producer's mark: the empty word refuses
	// honestly — no silent default.
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 1, ReplyClass: "",
		Content: "без класса", IdempotencyKey: "mode-noclass-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "класс размечает продюсер")
}

// TestModeDemoPathClassificationSafetyExit covers the owner-approved demo
// chain: the classification signal (turn) raises to sales; the sales reply
// demands the acting disclosure first; the disclosure with its sent event
// opens the sales replies; the safety signal raises above sales; the
// sales reply refuses with the permitted exit path; the downgrade
// proposal refuses (no surface can ever lower); the repeated signal is
// not a transition; the turn without a signal writes no term; the
// supervised exit lowers back to sales (AC-1/2/4/6/7).
func TestModeDemoPathClassificationSafetyExit(t *testing.T) {
	w := newCasesWorld(t, 731)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-mode-2", "sess-mode-2", `{"call":"inbound-call","from":"+79000000064"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-mode-2", "evt-mode-2-turn-1", `{"say":"хочу заказать деталь"}`)

	// The classification signal: unclassified -> sales is a RAISE (the
	// admission floor; the proposal ladder of the owner decision).
	classification := w.proposeMode(t, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
		Reason: "клиент хочет заказать деталь", SignalSource: casesDomain.ModeSignalTurnEvent, TurnRef: turn,
		IdempotencyKey: "mode-propose-sales-1", Now: ownershipNow,
	})
	if classification.ModeVersion != 2 || classification.ModeScope != casesDomain.ModeScopeConversation {
		t.Fatalf("expected the version 2 conversation term, got %+v", classification)
	}
	view := w.modeView(t, caseID)
	if view.Active.Mode != casesDomain.ModeSales || len(view.Terms) != 2 {
		t.Fatalf("expected the active sales derivation, got %+v", view.Active)
	}
	if view.Terms[1].PriorMode != casesDomain.ModeUnclassified || view.Terms[1].TurnRef != turn {
		t.Fatalf("expected the prior mode and the turn snapshot, got %+v", view.Terms[1])
	}

	// The sales reply without the acting disclosure: the machine-readable
	// disclosure gate refusal with the permitted record path (AC-1).
	_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Предлагаю оригинальную деталь", IdempotencyKey: "mode-sales-nodisclosure-1", Now: ownershipNow,
	})
	var disclosureGate casesDomain.ReplyDisclosureError
	if !errors.As(err, &disclosureGate) || disclosureGate.Channel != "phone" {
		t.Fatalf("expected the disclosure gate refusal of the phone channel, got %v", err)
	}

	// The disclosure record: born generated — it does not act yet.
	disclosure, err := w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "phone", Text: "Я цифровой сотрудник компании «Магазин Олега». Человек подключится по вашему запросу.",
		IdempotencyKey: "mode-disclosure-1", Now: ownershipNow,
	})
	if err != nil || disclosure.DeliveryKind != casesDomain.DeliveryGenerated {
		t.Fatalf("expected the born generated disclosure, got %+v %v", disclosure, err)
	}
	cards := w.modeView(t, caseID).Disclosures
	if len(cards) != 1 || cards[0].Status != casesDomain.DeliveryGenerated || cards[0].Record.DecisionNo != 1 {
		t.Fatalf("expected the one generated record with the lineage snapshot, got %+v", cards)
	}

	// The sales reply still refuses: generated does not act (AC-2).
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Предлагаю оригинальную деталь", IdempotencyKey: "mode-sales-nodisclosure-2", Now: ownershipNow,
	})
	if !errors.As(err, &disclosureGate) {
		t.Fatalf("expected the disclosure gate refusal while generated, got %v", err)
	}

	// The send observation makes the disclosure act.
	sent, err := w.cases.RecordDisclosureEvent(ctx, casesAPI.RecordDisclosureEventCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: disclosure.DisclosureID, Kind: casesDomain.DeliverySent,
		IdempotencyKey: "mode-disclosure-sent-1", Now: ownershipNow,
	})
	if err != nil || sent.DeliveryKind != casesDomain.DeliverySent {
		t.Fatalf("expected the sent event, got %+v %v", sent, err)
	}

	// The sales reply passes now (AC-1: the disclosure BEFORE the sale).
	// The Story 2.6 mechanical extension: the sales path needs the Work
	// Item anchor and the acting commercial Policy Outcome — the prelude
	// submits and evaluates one proposal (the honest Change Log entry).
	policy := w.policyPrelude(t, caseID, conversationID, workItemID, "mode-sales-ok-1")
	if !policy.PolicyOutcomeID.Valid() && policy.OutcomeID.String() == "" {
		t.Fatalf("the prelude outcome carries its identifiers")
	}
	salesReply, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Предлагаю оригинальную деталь", IdempotencyKey: "mode-sales-ok-1", Now: ownershipNow,
	})
	if err != nil || salesReply.ReplyState != "prepared" {
		t.Fatalf("expected the sales reply to pass, got %+v %v", salesReply, err)
	}
	if salesReply.ReplyClass != casesDomain.ReplyClassSales {
		t.Fatalf("the outcome carries the producer's class, got %+v", salesReply)
	}
	if !salesReply.PolicyOutcomeID.Valid() {
		t.Fatalf("the passing sales reply carries the Policy Outcome reference, got %+v", salesReply)
	}

	// The safety signal: sales -> safety is a strict raise; the turn
	// reference answers a real client turn.
	safetyTurn := w.modeTurnEvent(t, caseID, "sess-mode-2", "evt-mode-2-turn-2", `{"say":"мне плохо, помогите"}`)
	safety := w.proposeMode(t, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSafety, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 2,
		Reason: "сигнал безопасности в реплике", SignalSource: casesDomain.ModeSignalTurnEvent, TurnRef: safetyTurn,
		IdempotencyKey: "mode-propose-safety-1", Now: ownershipNow,
	})
	if safety.ModeVersion != 3 {
		t.Fatalf("expected the version 3 safety term, got %+v", safety)
	}
	view = w.modeView(t, caseID)
	if view.Active.Mode != casesDomain.ModeSafety || view.Active.SignalSource != casesDomain.ModeSignalTurnEvent {
		t.Fatalf("expected the active safety word with the signal source, got %+v", view.Active)
	}

	// The sales reply refuses with the machine-readable mode words and the
	// permitted exit path (AC-6).
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Всё же предлагаю деталь", IdempotencyKey: "mode-sales-safety-1", Now: ownershipNow,
	})
	var modeGate casesDomain.ReplyModeError
	if !errors.As(err, &modeGate) || modeGate.EffectiveMode != casesDomain.ModeSafety {
		t.Fatalf("expected the safety mode refusal, got %v", err)
	}

	// The downgrade proposal: no surface can ever lower (AC-4/7).
	_, err = w.cases.ProposeMode(ctx, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 3,
		Reason: "понизить", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-downgrade-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "понизить")

	// The equal repeat: a repeated signal is not a transition.
	_, err = w.cases.ProposeMode(ctx, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSafety, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 3,
		Reason: "повтор сигнала", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-repeat-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "уже активен")

	// The turn without a signal: no term, no automatic return (AC-7).
	_ = w.modeTurnEvent(t, caseID, "sess-mode-2", "evt-mode-2-turn-3", `{"say":"спасибо"}`)
	if view := w.modeView(t, caseID); len(view.Terms) != 3 {
		t.Fatalf("the turn without a signal writes no term, got %d terms", len(view.Terms))
	}

	// The supervised exit (the owner carries the configuration word of the
	// world owner role): safety -> sales with the honest reason.
	exit, err := w.cases.ExitMode(ctx, casesAPI.ExitModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, ExpectedModeVersion: 3, Reason: "сигнал безопасности отработан человеком",
		IdempotencyKey: "mode-exit-1", Now: ownershipNow,
	})
	if err != nil || exit.ModeVersion != 4 {
		t.Fatalf("expected the version 4 exit term, got %+v %v", exit, err)
	}
	view = w.modeView(t, caseID)
	if view.Active.Mode != casesDomain.ModeSales {
		t.Fatalf("expected the active sales word after the exit, got %+v", view.Active)
	}
	exitTerm := view.Terms[3]
	if exitTerm.Origin != casesDomain.ModeOriginExitOverride || exitTerm.ExitAuthority != casesDomain.ModeAuthoritySupervisor {
		t.Fatalf("expected the exit override origin and the supervisor word, got %+v", exitTerm)
	}

	// The sales reply passes again after the exit (the disclosure still
	// acts — its last event is sent). The Story 2.6 mechanical extension:
	// the prelude outcome stays current (the honest Change Log entry).
	w.policyPrelude(t, caseID, conversationID, workItemID, "mode-sales-after-exit-1")
	afterExit, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Предлагаю аналог", IdempotencyKey: "mode-sales-after-exit-1", Now: ownershipNow,
	})
	if err != nil || afterExit.ReplyState != "prepared" {
		t.Fatalf("expected the sales reply after the exit, got %+v %v", afterExit, err)
	}
}

// TestModeExitAuthorityLadder covers the authority matrix: the current
// owner with the review scope exits the non-safety modes; the safety mode
// is the supervision word ONLY — the owner without the configuration
// scope never weakens the platform safety (AC-9).
func TestModeExitAuthorityLadder(t *testing.T) {
	w := newCasesWorld(t, 732)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-mode-3", "sess-mode-3", `{"call":"inbound-call","from":"+79000000065"}`)
	conversationID := w.conversationFirst(t, caseID)

	// The ownership moves to the backup member (the review+audit scopes —
	// never a configuration member): the transfer lifecycle of 2-4.
	request, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "передаю режимный сценарий",
		IdempotencyKey: "mode-auth-tr-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, RequestID: request.TransferRequestID,
		Decision: casesDomain.TransferResolutionAcknowledged, Reason: "принимаю",
		IdempotencyKey: "mode-auth-ack-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The classification and the raise: the backup (the current owner with
	// the review scope) drives the ladder.
	w.proposeMode(t, casesAPI.ProposeModeCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
		Reason: "классификация", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-auth-sales-1", Now: ownershipNow,
	})
	w.proposeMode(t, casesAPI.ProposeModeCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeClaim, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 2,
		Reason: "претензия клиента", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-auth-claim-1", Now: ownershipNow,
	})

	// The current owner (review, no configuration) exits the non-safety
	// mode: the operator_owner authority word.
	exit, err := w.cases.ExitMode(ctx, casesAPI.ExitModeCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, ExpectedModeVersion: 3, Reason: "претензия закрыта",
		IdempotencyKey: "mode-auth-exit-1", Now: ownershipNow,
	})
	if err != nil || exit.ModeVersion != 4 {
		t.Fatalf("expected the owner exit of the claim mode, got %+v %v", exit, err)
	}
	if term := w.modeView(t, caseID).Terms[3]; term.ExitAuthority != casesDomain.ModeAuthorityOperatorOwner {
		t.Fatalf("expected the operator_owner authority word, got %+v", term)
	}

	// The safety raise and the owner exit attempt: the platform safety is
	// the supervision word ONLY (AC-9).
	w.proposeMode(t, casesAPI.ProposeModeCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSafety, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 4,
		Reason: "сигнал безопасности", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-auth-safety-1", Now: ownershipNow,
	})
	_, err = w.cases.ExitMode(ctx, casesAPI.ExitModeCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, ExpectedModeVersion: 5, Reason: "хочу продавать",
		IdempotencyKey: "mode-auth-exit-2", Now: ownershipNow,
	})
	assertRuleError(t, err, "полномочия")

	// The configuration member (the world owner) exits the safety mode:
	// the supervisor word.
	supervised, err := w.cases.ExitMode(ctx, casesAPI.ExitModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, ExpectedModeVersion: 5, Reason: "безопасность отработана надзором",
		IdempotencyKey: "mode-auth-exit-3", Now: ownershipNow,
	})
	if err != nil || supervised.ModeVersion != 6 {
		t.Fatalf("expected the supervised exit, got %+v %v", supervised, err)
	}
}

// TestModeWorkItemContexts covers the per-Work-Item matrix: the item
// context term (scope=work_item) raises the effective mode of that item
// only — the strictest of the conversation word and the item word; the
// independent item keeps the sales path; the neutral replies pass under
// any mode (AC-8).
func TestModeWorkItemContexts(t *testing.T) {
	w := newCasesWorld(t, 733)
	ctx := context.Background()
	caseID, firstItem := w.acceptedCase(t, "evt-mode-4", "sess-mode-4", `{"call":"inbound-call","from":"+79000000066"}`)
	conversationID := w.conversationFirst(t, caseID)

	// The classification to sales and the acting disclosure: the sales
	// path opens.
	w.proposeMode(t, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
		Reason: "классификация", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-wi-sales-1", Now: ownershipNow,
	})
	disclosure, err := w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "phone", Text: "Я цифровой сотрудник компании.",
		IdempotencyKey: "mode-wi-disclosure-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.cases.RecordDisclosureEvent(ctx, casesAPI.RecordDisclosureEventCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: disclosure.DisclosureID, Kind: casesDomain.DeliverySent,
		IdempotencyKey: "mode-wi-sent-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}

	// The second work item of the case (the 2-2 command).
	secondItem, err := w.cases.CreateWorkItem(ctx, casesAPI.CreateWorkItemCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Type: "поиск детали", Title: "Подбор аналога", IdempotencyKey: "mode-wi-create-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The first item raises its own context: cancellation (the item scope
	// consumes a version of the ladder).
	w.proposeMode(t, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeCancellation, Scope: casesDomain.ModeScopeWorkItem, WorkItemID: firstItem,
		ExpectedModeVersion: 2, Reason: "клиент отменил эту задачу", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-wi-cancel-1", Now: ownershipNow,
	})
	view := w.modeView(t, caseID)
	if len(view.ItemContexts) != 1 || view.ItemContexts[0].WorkItemID != firstItem || view.ItemContexts[0].Term.Mode != casesDomain.ModeCancellation {
		t.Fatalf("expected the one cancellation item context, got %+v", view.ItemContexts)
	}

	// The sales reply of the first item: the effective mode is the
	// stricter cancellation — the refusal names it (AC-8).
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: firstItem, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Всё же предлагаю", IdempotencyKey: "mode-wi-sales-blocked-1", Now: ownershipNow,
	})
	var itemModeGate casesDomain.ReplyModeError
	if !errors.As(err, &itemModeGate) || itemModeGate.EffectiveMode != casesDomain.ModeCancellation {
		t.Fatalf("expected the cancellation effective refusal, got %v", err)
	}

	// The sales reply of the independent item: the conversation sales word
	// governs — allowed (the explicit v1 rule, AC-8). The Story 2.6
	// mechanical extension: the prelude opens the commercial outcome of
	// the independent item (the honest Change Log entry).
	w.policyPrelude(t, caseID, conversationID, secondItem.WorkItemID, "mode-wi-sales-ok-1")
	allowed, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: secondItem.WorkItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Предлагаю аналог", IdempotencyKey: "mode-wi-sales-ok-1", Now: ownershipNow,
	})
	if err != nil || allowed.ReplyState != "prepared" {
		t.Fatalf("expected the sales reply of the independent item, got %+v %v", allowed, err)
	}

	// The neutral reply of the restrictive item: the safe independent work
	// passes (AC-8 — the explicit v1 rule).
	neutral, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: firstItem, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
		Content: "Отмена принята, подтверждаю", IdempotencyKey: "mode-wi-neutral-1", Now: ownershipNow,
	})
	if err != nil || neutral.ReplyState != "prepared" {
		t.Fatalf("expected the neutral reply under the cancellation context, got %+v %v", neutral, err)
	}

	// The item context repeat and downgrade: the same ladder discipline of
	// the item scope.
	_, err = w.cases.ProposeMode(ctx, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeCancellation, Scope: casesDomain.ModeScopeWorkItem, WorkItemID: firstItem,
		ExpectedModeVersion: 3, Reason: "повтор", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-wi-repeat-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "уже активен")
	_, err = w.cases.ProposeMode(ctx, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, Scope: casesDomain.ModeScopeWorkItem, WorkItemID: firstItem,
		ExpectedModeVersion: 3, Reason: "понизить контекст", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-wi-downgrade-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "понизить")
}

// TestModeDisclosureInterruptionAndReplay covers the delivery lifecycle
// matrix: the interruption voids the acting disclosure (the sales reply
// refuses); the re-disclosure with the parent reference acts after its
// send event; the exact repeat replays; the same key with the other
// material is the honest conflict (AC-2/AC-10).
func TestModeDisclosureInterruptionAndReplay(t *testing.T) {
	w := newCasesWorld(t, 734)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-mode-5", "sess-mode-5", `{"call":"inbound-call","from":"+79000000067"}`)
	conversationID := w.conversationFirst(t, caseID)

	// The sales mode and the acting disclosure.
	w.proposeMode(t, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
		Reason: "классификация", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-disc-sales-1", Now: ownershipNow,
	})
	disclosure, err := w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "phone", Text: "Я цифровой сотрудник компании.",
		IdempotencyKey: "mode-disc-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.cases.RecordDisclosureEvent(ctx, casesAPI.RecordDisclosureEventCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: disclosure.DisclosureID, Kind: casesDomain.DeliverySent,
		IdempotencyKey: "mode-disc-sent-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}

	// The exact repeat of the disclosure command replays the outcome.
	replayed, err := w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "phone", Text: "Я цифровой сотрудник компании.",
		IdempotencyKey: "mode-disc-1", Now: ownershipNow,
	})
	if err != nil || !replayed.Replay || replayed.DisclosureID != disclosure.DisclosureID {
		t.Fatalf("expected the idempotent replay, got %+v %v", replayed, err)
	}
	// The same key with the other material: the honest conflict.
	_, err = w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "phone", Text: "Другой текст раскрытия",
		IdempotencyKey: "mode-disc-1", Now: ownershipNow,
	})
	if !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("expected the key conflict, got %v", err)
	}

	// The playback word acts (never a proof of hearing — the gate needs
	// the send word only, AC-2).
	if _, err = w.cases.RecordDisclosureEvent(ctx, casesAPI.RecordDisclosureEventCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: disclosure.DisclosureID, Kind: casesDomain.DeliveryPlaybackEstimated,
		IdempotencyKey: "mode-disc-playback-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	// The Story 2.6 mechanical extension: the prelude opens the commercial
	// outcome (the honest Change Log entry).
	policyItem := workItemID
	if !policyItem.Valid() {
		t.Fatalf("the prelude needs the Work Item anchor")
	}
	w.policyPrelude(t, caseID, conversationID, policyItem, "mode-disc-sales-1")
	sales, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: policyItem, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Предлагаю деталь", IdempotencyKey: "mode-disc-sales-1", Now: ownershipNow,
	})
	if err != nil || sales.ReplyState != "prepared" {
		t.Fatalf("expected the sales reply under the playback word, got %+v %v", sales, err)
	}

	// The interruption voids the acting disclosure: the sales reply
	// refuses until the re-disclosure (AC-2).
	if _, err = w.cases.RecordDisclosureEvent(ctx, casesAPI.RecordDisclosureEventCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: disclosure.DisclosureID, Kind: casesDomain.DeliveryInterrupted,
		IdempotencyKey: "mode-disc-interrupted-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	var disclosureGate casesDomain.ReplyDisclosureError
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Всё же предлагаю", IdempotencyKey: "mode-disc-sales-2", Now: ownershipNow,
	})
	if !errors.As(err, &disclosureGate) {
		t.Fatalf("expected the disclosure gate refusal after the interruption, got %v", err)
	}

	// The re-disclosure with the parent reference: the record carries the
	// parent snapshot; the send event makes it act again.
	reDisclosure, err := w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "phone", Text: "Я цифровой сотрудник компании (повторяю).", ParentRef: disclosure.DisclosureID,
		IdempotencyKey: "mode-disc-2", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	cards := w.modeView(t, caseID).Disclosures
	if len(cards) != 2 || !cards[1].Record.ParentRef.Valid() || cards[1].Record.ParentRef != disclosure.DisclosureID {
		t.Fatalf("expected the re-disclosure with the parent reference, got %+v", cards)
	}
	if _, err = w.cases.RecordDisclosureEvent(ctx, casesAPI.RecordDisclosureEventCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: reDisclosure.DisclosureID, Kind: casesDomain.DeliverySent,
		IdempotencyKey: "mode-disc-2-sent-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	// The Story 2.6 mechanical extension: the prelude outcome opens the
	// sales path again (the honest Change Log entry).
	w.policyPrelude(t, caseID, conversationID, workItemID, "mode-disc-sales-3")
	afterReDisclosure, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Предлагаю деталь (повторно)", IdempotencyKey: "mode-disc-sales-3", Now: ownershipNow,
	})
	if err != nil || afterReDisclosure.ReplyState != "prepared" {
		t.Fatalf("expected the sales reply after the re-disclosure, got %+v %v", afterReDisclosure, err)
	}

	// The protected request class needs the acting disclosure too (AC-1):
	// it passes under the acting re-disclosure.
	protected, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassProtectedRequest,
		Content: "Подтвердите, пожалуйста, номер карты для возврата", IdempotencyKey: "mode-protected-1", Now: ownershipNow,
	})
	if err != nil || protected.ReplyState != "prepared" {
		t.Fatalf("expected the protected reply under the acting disclosure, got %+v %v", protected, err)
	}
}

// TestModeProposeGuardsRacesAndIsolation covers the guard and the race
// matrix: the dictionary and the material refusals (the unknown mode, the
// scope discipline, the turn discipline, the unknown delivery kind, the
// unknown parent); the CAS race of the concurrent proposals — exactly one
// term, the honest conflict with the current mode words; the idempotent
// replay of the proposals; the cross-tenant 404 isolation (AC-10 + the
// regression row).
func TestModeProposeGuardsRacesAndIsolation(t *testing.T) {
	w := newCasesWorld(t, 735)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-mode-6", "sess-mode-6", `{"call":"inbound-call","from":"+79000000068"}`)
	conversationID := w.conversationFirst(t, caseID)

	base := casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeClaim, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
		Reason: "претензия", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-guard-claim-1", Now: ownershipNow,
	}

	// The dictionary refusals: the unknown mode and the scope discipline.
	_, err := w.cases.ProposeMode(ctx, func() casesAPI.ProposeModeCommand {
		command := base
		command.Mode = "marketing"
		command.IdempotencyKey = "mode-guard-unknown-1"
		return command
	}())
	assertRuleError(t, err, "словарь")
	_, err = w.cases.ProposeMode(ctx, func() casesAPI.ProposeModeCommand {
		command := base
		command.Scope = "case"
		command.IdempotencyKey = "mode-guard-scope-1"
		return command
	}())
	assertRuleError(t, err, "словарь")
	// The work item scope without the anchor.
	_, err = w.cases.ProposeMode(ctx, func() casesAPI.ProposeModeCommand {
		command := base
		command.Scope = casesDomain.ModeScopeWorkItem
		command.IdempotencyKey = "mode-guard-item-1"
		return command
	}())
	assertRuleError(t, err, "требует work_item_id")
	// The turn source without the reference; the operator word with one.
	_, err = w.cases.ProposeMode(ctx, func() casesAPI.ProposeModeCommand {
		command := base
		command.SignalSource = casesDomain.ModeSignalTurnEvent
		command.IdempotencyKey = "mode-guard-turn-1"
		return command
	}())
	assertRuleError(t, err, "turn_ref")
	_, err = w.cases.ProposeMode(ctx, func() casesAPI.ProposeModeCommand {
		command := base
		command.TurnRef = id.MustParse("01994f64-6c00-7000-8000-0000000000ff")
		command.IdempotencyKey = "mode-guard-turn-2"
		return command
	}())
	assertRuleError(t, err, "turn_ref допустим только")
	// The turn reference of ANOTHER case: the honest not-found.
	foreign := id.MustParse("01994f64-6c00-7000-8000-0000000000ee")
	_, err = w.cases.ProposeMode(ctx, func() casesAPI.ProposeModeCommand {
		command := base
		command.SignalSource = casesDomain.ModeSignalTurnEvent
		command.TurnRef = foreign
		command.IdempotencyKey = "mode-guard-turn-3"
		return command
	}())
	assertRuleError(t, err, "turn_ref")
	// The conversation of ANOTHER case: the ladder lives on the case's own
	// conversation.
	otherCase, _ := w.acceptedCase(t, "evt-mode-6b", "sess-mode-6b", `{"call":"inbound-call","from":"+79000000069"}`)
	otherConversation := w.conversationFirst(t, otherCase)
	_, err = w.cases.ProposeMode(ctx, func() casesAPI.ProposeModeCommand {
		command := base
		command.ConversationID = otherConversation
		command.IdempotencyKey = "mode-guard-conversation-1"
		return command
	}())
	assertRuleError(t, err, "собственном диалоге")
	// The unknown delivery kind and the unknown disclosure.
	_, err = w.cases.RecordDisclosureEvent(ctx, casesAPI.RecordDisclosureEventCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: id.MustParse("01994f64-6c00-7000-8000-0000000000e1"), Kind: "lost",
		IdempotencyKey: "mode-guard-kind-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "словарь")
	_, err = w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "no-such-channel", Text: "текст",
		IdempotencyKey: "mode-guard-channel-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "словарь каналов")
	_, err = w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "phone", Text: strings.Repeat("а", 2001),
		IdempotencyKey: "mode-guard-text-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "длиннее")
	// The parent reference of another conversation: the honest refusal.
	disclosure, err := w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: otherCase, ConversationID: otherConversation,
		Channel: "phone", Text: "Раскрытие другого кейса",
		IdempotencyKey: "mode-guard-parent-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "phone", Text: "Повтор с чужим родителем", ParentRef: disclosure.DisclosureID,
		IdempotencyKey: "mode-guard-parent-2", Now: ownershipNow,
	})
	assertRuleError(t, err, "родительская запись")

	// The CAS race: two concurrent proposals with the same expected version
	// produce exactly one term; the loser gets the honest conflict with the
	// current mode words (AC-10).
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := range 2 {
		wg.Add(1)
		go func(attempt int) {
			defer wg.Done()
			_, err := w.cases.ProposeMode(ctx, casesAPI.ProposeModeCommand{
				Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
				Mode: casesDomain.ModeClaim, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
				Reason: "конкурентная претензия", SignalSource: casesDomain.ModeSignalOperator,
				IdempotencyKey: "mode-race-" + string(rune('a'+attempt)) + "-1", Now: ownershipNow,
			})
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	var conflicts, succeeded int
	for err := range results {
		if err == nil {
			succeeded++
			continue
		}
		var conflict casesDomain.ModeConflictError
		if errors.As(err, &conflict) {
			if conflict.ActiveMode != casesDomain.ModeClaim || conflict.ModeVersion != 2 {
				t.Fatalf("the conflict carries the current mode words, got %+v", conflict)
			}
			conflicts++
		} else if !errors.Is(err, casesDomain.ErrStaleVersion) {
			t.Fatalf("unexpected race error: %v", err)
		} else {
			conflicts++
		}
	}
	if succeeded != 1 || conflicts != 1 {
		t.Fatalf("expected one winner and one honest conflict, got %d/%d", succeeded, conflicts)
	}
	if view := w.modeView(t, caseID); len(view.Terms) != 2 || view.Active.Mode != casesDomain.ModeClaim {
		t.Fatalf("expected exactly one claim term of the race, got %d terms %+v", len(view.Terms), view.Active)
	}

	// The idempotent replay of the winning proposal (the exact repeat
	// under the winner's key): re-run one with the same material and the
	// same key.
	replay, err := w.cases.ProposeMode(ctx, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeClaim, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
		Reason: "конкурентная претензия", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-race-a-1", Now: ownershipNow,
	})
	if err == nil && !replay.Replay {
		// The winner was the other goroutine — the honest conflict stands.
		t.Fatalf("expected the replay or the honest conflict, got %+v", replay)
	}

	// The cross-tenant isolation: the foreign tenant gets the plain 404
	// (the tenant existence is never disclosed).
	_, err = w.cases.ProposeMode(ctx, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: id.MustParse("01994f64-6c00-7000-8000-0000000000f1"),
		CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeClaim, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
		Reason: "чужой тенант", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-foreign-1", Now: ownershipNow,
	})
	if !errors.Is(err, casesDomain.ErrNotFound) && !errors.Is(err, identityDomain.ErrNotFound) {
		t.Fatalf("expected the honest not-found of the foreign tenant, got %v", err)
	}
}

// TestModeTablesRLSForced asserts the append-only discipline of the mode
// surface at the database level: RLS + FORCE on all three tables (the
// ownership precedent).
func TestModeTablesRLSForced(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
                select relname, relrowsecurity, relforcerowsecurity from pg_class
                where relnamespace = 'cases'::regnamespace
                  and relname in ('mode_terms','disclosure_records','disclosure_delivery_events')`)
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
	if len(seen) != 3 {
		t.Fatalf("expected the three mode tables, got %d", len(seen))
	}
	for name, flags := range seen {
		if !flags[0] || !flags[1] {
			t.Fatalf("the table %s must carry RLS + FORCE, got %v", name, flags)
		}
	}
}

// TestModeGrantsAppendOnly asserts the grant discipline: the mode tables
// carry INSERT/SELECT for staffy_app and no UPDATE or DELETE (the
// append-only ladder and disclosure history).
func TestModeGrantsAppendOnly(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	for _, table := range []string{"mode_terms", "disclosure_records", "disclosure_delivery_events"} {
		for _, grant := range []string{"INSERT", "SELECT", "UPDATE", "DELETE", "TRUNCATE"} {
			var allowed bool
			if err := runtimePool.QueryRow(ctx, `select has_table_privilege('staffy_app', $1, $2)`, "cases."+table, grant).Scan(&allowed); err != nil {
				t.Fatal(err)
			}
			if (grant == "INSERT" || grant == "SELECT") != allowed {
				t.Fatalf("%s on cases.%s: allowed=%v, want %v", grant, table, allowed, grant == "INSERT" || grant == "SELECT")
			}
		}
	}
}

// TestModeCommandsRefuseAuditOnlyMember covers the R1 Major fix: the mode
// write surface needs the review or the configuration scope — an
// audit-only member reads the case view but never writes the ladder, the
// disclosures or the delivery events (the frozen Boundaries word:
// «propose/record/event — участники с review»).
func TestModeCommandsRefuseAuditOnlyMember(t *testing.T) {
	w := newCasesWorld(t, 736)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-mode-audit-1", "sess-mode-audit-1", `{"call":"inbound-call","from":"+79000000070"}`)
	conversationID := w.conversationFirst(t, caseID)
	// The member ids avoid the world suffixes (the owner is seed*10+1, the
	// backup is seed*10+8): seed*10+6/9 stay free.
	auditor := inviteMember(t, w.identity, w.readinessActor(true), w.tenant, "mode-auditor-736@example.org", "expert_qa", []identityDomain.Scope{identityDomain.Audit}, "000000007366", "000000007369")
	auditOnly := actorPrincipal{principalID: auditor.ID, stepUp: true}.principal()

	_, err := w.cases.ProposeMode(ctx, casesAPI.ProposeModeCommand{
		Actor: auditOnly, TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
		Reason: "аудитор предлагает", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-audit-propose-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "review или configuration")

	_, err = w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: auditOnly, TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "phone", Text: "Я цифровой сотрудник компании.",
		IdempotencyKey: "mode-audit-disclosure-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "review или configuration")

	_, err = w.cases.RecordDisclosureEvent(ctx, casesAPI.RecordDisclosureEventCommand{
		Actor: auditOnly, TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: id.MustParse("01994f64-6c00-7000-8000-0000000000e1"), Kind: casesDomain.DeliverySent,
		IdempotencyKey: "mode-audit-event-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "review или configuration")

	// Nothing landed: the ladder and the disclosure surface stay clean.
	view := w.modeView(t, caseID)
	if len(view.Terms) != 1 || len(view.Disclosures) != 0 {
		t.Fatalf("the audit-only member writes nothing, got %d terms and %d disclosures", len(view.Terms), len(view.Disclosures))
	}
}

// TestModeBasisDisputeRaisesPrivacyNoContact covers the matrix row 14
// (FR-15): the client disputes the contact basis in a turn — the operator
// fixes the signal (the turn reference) — the deterministic policy raises
// privacy_no_contact — the sales replies stop immediately; the refusal
// text is never a marketing word.
func TestModeBasisDisputeRaisesPrivacyNoContact(t *testing.T) {
	w := newCasesWorld(t, 737)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-mode-privacy-1", "sess-mode-privacy-1", `{"call":"inbound-call","from":"+79000000071"}`)
	conversationID := w.conversationFirst(t, caseID)

	// The sales classification and the acting disclosure open the sales
	// path first (the dispute stops it afterwards — the honest «до» word).
	w.proposeMode(t, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
		Reason: "классификация", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-privacy-sales-1", Now: ownershipNow,
	})
	disclosure, err := w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "phone", Text: "Я цифровой сотрудник компании.",
		IdempotencyKey: "mode-privacy-disclosure-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.cases.RecordDisclosureEvent(ctx, casesAPI.RecordDisclosureEventCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: disclosure.DisclosureID, Kind: casesDomain.DeliverySent,
		IdempotencyKey: "mode-privacy-sent-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	// The Story 2.6 mechanical extension: the prelude opens the commercial
	// outcome (the honest Change Log entry).
	w.policyPrelude(t, caseID, conversationID, workItemID, "mode-privacy-open-1")
	open, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Предлагаю деталь", IdempotencyKey: "mode-privacy-open-1", Now: ownershipNow,
	})
	if err != nil || open.ReplyState != "prepared" {
		t.Fatalf("expected the sales reply to pass before the dispute, got %+v %v", open, err)
	}

	// The client disputes the contact basis in a turn: the operator fixes
	// the signal with the turn reference — the policy raises
	// privacy_no_contact (the sales path stops immediately, FR-15).
	disputeTurn := w.modeTurnEvent(t, caseID, "sess-mode-privacy-1", "evt-mode-privacy-turn-1", `{"say":"я не давал согласия на звонки"}`)
	raise := w.proposeMode(t, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModePrivacyNoContact, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 2,
		Reason: "клиент оспаривает основание контакта", SignalSource: casesDomain.ModeSignalTurnEvent, TurnRef: disputeTurn,
		IdempotencyKey: "mode-privacy-raise-1", Now: ownershipNow,
	})
	if raise.ModeVersion != 3 {
		t.Fatalf("expected the version 3 privacy term, got %+v", raise)
	}
	view := w.modeView(t, caseID)
	if view.Active.Mode != casesDomain.ModePrivacyNoContact || view.Active.TurnRef != disputeTurn {
		t.Fatalf("expected the privacy_no_contact active word with the turn snapshot, got %+v", view.Active)
	}

	// The sales reply stops immediately: the machine-readable mode refusal
	// with the privacy word (the refusal text is never a marketing word).
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		Content: "Всё же предлагаем вам", IdempotencyKey: "mode-privacy-blocked-1", Now: ownershipNow,
	})
	var modeGate casesDomain.ReplyModeError
	if !errors.As(err, &modeGate) || modeGate.EffectiveMode != casesDomain.ModePrivacyNoContact {
		t.Fatalf("expected the privacy_no_contact refusal, got %v", err)
	}
	if strings.Contains(modeGate.Reason, "предлагаем") || strings.Contains(modeGate.Reason, "скидк") {
		t.Fatalf("the refusal text is never a marketing word, got %q", modeGate.Reason)
	}

	// The protected request refusal branch of the disclosure gate (the
	// fresh case without any disclosure: the protected class demands the
	// acting disclosure BEFORE the request of the protected data — AC-1).
	otherCase, _ := w.acceptedCase(t, "evt-mode-privacy-2", "sess-mode-privacy-2", `{"call":"inbound-call","from":"+79000000072"}`)
	otherConversation := w.conversationFirst(t, otherCase)
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: otherCase, ConversationID: otherConversation,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassProtectedRequest,
		Content: "Подтвердите номер карты", IdempotencyKey: "mode-privacy-protected-1", Now: ownershipNow,
	})
	var disclosureGate casesDomain.ReplyDisclosureError
	if !errors.As(err, &disclosureGate) {
		t.Fatalf("expected the disclosure gate refusal of the protected class, got %v", err)
	}
}

// TestModeAuditOnlyOwnerCannotExit covers the R2 Minor fix: a supervision
// reassignment may hand the ownership to an audit-only member (the
// reassign target is any active member), but that owner never exits any
// mode — the frozen Boundaries letter requires the review scope on the
// owner side of the authority ladder.
func TestModeAuditOnlyOwnerCannotExit(t *testing.T) {
	w := newCasesWorld(t, 738)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-mode-audit-owner-1", "sess-mode-audit-owner-1", `{"call":"inbound-call","from":"+79000000073"}`)
	conversationID := w.conversationFirst(t, caseID)

	// The sales classification: a non-safety mode exists to exit.
	w.proposeMode(t, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeSales, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
		Reason: "классификация", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-audit-owner-sales-1", Now: ownershipNow,
	})

	// The supervision reassignment (the configuration path of 2-4) hands
	// the ownership to the audit-only member.
	auditor := inviteMember(t, w.identity, w.readinessActor(true), w.tenant, "mode-audit-owner-738@example.org", "expert_qa", []identityDomain.Scope{identityDomain.Audit}, "000000007386", "000000007389")
	auditOwner := actorPrincipal{principalID: auditor.ID, stepUp: true}.principal()
	_, err := w.cases.ReassignOwnership(ctx, casesAPI.ReassignOwnershipCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: auditor.ID,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "надзор передаёт аудитору",
		IdempotencyKey: "mode-audit-owner-reassign-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The audit-only owner never exits anything: the honest refusal names
	// the authority ladder (the review word of the owner side).
	_, err = w.cases.ExitMode(ctx, casesAPI.ExitModeCommand{
		Actor: auditOwner, TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeUnclassified, ExpectedModeVersion: 2, Reason: "аудитор-владелец хочет выйти",
		IdempotencyKey: "mode-audit-owner-exit-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "полномочия")

	// The mode mode commands stay refused for the audit-only owner too
	// (the write scope never follows the ownership alone).
	_, err = w.cases.ProposeMode(ctx, casesAPI.ProposeModeCommand{
		Actor: auditOwner, TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: casesDomain.ModeClaim, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 2,
		Reason: "аудитор-владелец предлагает", SignalSource: casesDomain.ModeSignalOperator,
		IdempotencyKey: "mode-audit-owner-propose-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "review или configuration")

	// Nothing landed: the ladder stays at the sales term.
	if view := w.modeView(t, caseID); len(view.Terms) != 2 {
		t.Fatalf("the audit-only owner writes nothing to the ladder, got %d terms", len(view.Terms))
	}
}
