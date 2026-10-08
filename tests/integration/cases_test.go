//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/adapters/ingresssyn"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesApp "github.com/artemiosu/staffy/internal/cases/app"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesStore "github.com/artemiosu/staffy/internal/cases/store"
	connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	entitlementAPI "github.com/artemiosu/staffy/internal/entitlement/api"
	entitlementDomain "github.com/artemiosu/staffy/internal/entitlement/domain"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// casesWorld wraps the readiness world with the case contour service: the
// ingress over the synthetic verifier, the operator reads and the honest
// fact ports. The world carries an ACTIVE synthetic publication (the
// acting decision admits phone:inbound with the owner as the initial
// response owner), a live synthetic telephony connection and an active
// entitlement.
type casesWorld struct {
	*readinessWorld
	cases        *casesApp.Service
	employeeID   id.UUID
	connection   id.UUID
	manifestLink employeeDomain.ManifestLink
}

func newCasesWorld(t *testing.T, seed int) *casesWorld {
	t.Helper()
	w, employee, link := newReadinessWorld(t, seed)
	// The RouteFacts facade relays the scope-based entitlement admission
	// word (the ingress relay of Story 2-1).
	w.employee = w.employee.WithEntitlementAdmissionFacts(w.entitlement)
	ctx := context.Background()
	connections, err := w.connector.List(ctx, w.owner, w.tenant)
	if err != nil || len(connections) == 0 {
		t.Fatalf("expected the world connection: %v", err)
	}
	world := &casesWorld{readinessWorld: w, employeeID: employee, connection: connections[0].ConnectionID, manifestLink: link}
	world.publishSynthetic(t)
	world.cases = world.buildCasesService(newPrivacyIDs(seed + 900))
	return world
}

// buildCasesService wires the contour with the given id generator (the
// race test uses the thread-safe real generator).
func (w *casesWorld) buildCasesService(ids casesAPI.IDGenerator) *casesApp.Service {
	return casesApp.New(casesStore.NewPostgres(runtimePool), w.identity, ingresssyn.New(w.connector), releaseClock, ids).
		WithRouteFacts(w.employee).
		WithConnectionFacts(w.connector).
		WithMembers(w.identity)
}

func (w *casesWorld) publishSynthetic(t *testing.T) {
	t.Helper()
	_, err := w.employee.PublishAdmission(context.Background(), employeeAPI.PublishAdmissionCommand{
		Actor: w.readinessActor(true), TenantID: w.tenant, EmployeeID: w.employeeID,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000708"),
		Stage:         employeeDomain.StageSynthetic,
	})
	if err != nil {
		t.Fatal(err)
	}
}

var casesClock = fixedClock{time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}

// ingestCommand builds one canonical inbound phone-session event of the
// world connection.
func (w *casesWorld) ingestCommand(eventID, sessionID, payload string) casesAPI.IngestCommand {
	return casesAPI.IngestCommand{
		Envelope: casesDomain.Envelope{
			SchemaVersion: 1, TenantClaim: w.tenant, ConnectionID: w.connection,
			Provider: "staffy-synthetic-telephony", ProviderEventID: eventID, ProviderSessionID: sessionID,
			Direction: "inbound", Modality: "voice", Channel: "phone",
			ProviderTimestamp: casesClock.Now().Add(-time.Minute),
			Payload:           []byte(payload),
			PayloadDigest:     casesDomain.PayloadDigest([]byte(payload)),
			EnvelopeBytes:     256,
		},
		IdempotencyKey: "ingress-" + eventID,
		CorrelationID:  id.MustParse("01994f64-6c00-7000-8000-000000000710"),
		Now:            casesClock.Now(),
	}
}

func acceptedOutcome(t *testing.T, outcome casesAPI.IngestOutcome) casesAPI.IngestResult {
	t.Helper()
	if outcome.Refused != nil {
		t.Fatalf("expected acceptance, got refusal %s: %s", outcome.Refused.Kind, outcome.Refused.Message)
	}
	if outcome.Quarantined != nil {
		t.Fatalf("expected acceptance, got quarantine %s", outcome.Quarantined.Kind)
	}
	if outcome.Accepted == nil {
		t.Fatal("expected the accepted result")
	}
	return *outcome.Accepted
}

func refusedOutcome(t *testing.T, outcome casesAPI.IngestOutcome, kind string) {
	t.Helper()
	if outcome.Accepted != nil {
		t.Fatalf("expected the refusal %s, got acceptance %+v", kind, outcome.Accepted)
	}
	if outcome.Quarantined != nil {
		t.Fatalf("expected the refusal %s, got quarantine %s", kind, outcome.Quarantined.Kind)
	}
	if outcome.Refused == nil || outcome.Refused.Kind != kind {
		t.Fatalf("expected the refusal %s, got %+v", kind, outcome.Refused)
	}
}

// TestIngestAcceptCreatesCaseMaterialInOneTransaction covers the demo path
// of the slice: one inbound phone-session event on the admitted route
// creates the case, the conversation with the owner from human coverage,
// the untriaged work item, the sealed payload and the two outbox records;
// the operator sees the case and the timeline; the drain is idempotent.
func TestIngestAcceptCreatesCaseMaterialInOneTransaction(t *testing.T) {
	w := newCasesWorld(t, 310)
	ctx := context.Background()

	outcome, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-accept-1", "sess-accept-1", `{"call":"inbound-call","from":"+79000000001"}`))
	if err != nil {
		t.Fatal(err)
	}
	result := acceptedOutcome(t, outcome)
	if !result.NewCase || result.Replay {
		t.Fatalf("expected a fresh case, got %+v", result)
	}
	if result.CaseNo != 1 || result.ConversationNo != 1 {
		t.Fatalf("expected the first monotonic numbers, got %+v", result)
	}
	if result.ResponseOwnerID != w.owner.ID {
		t.Fatalf("expected the owner from human coverage, got %s", result.ResponseOwnerID)
	}
	if result.EmployeeID != w.employeeID {
		t.Fatalf("expected the acting employee, got %s", result.EmployeeID)
	}

	// The sealed payload read verifies the digest and the HMAC seal.
	payload, err := w.cases.ReadPayload(ctx, actorPrincipal{principalID: w.owner.ID, stepUp: true}.principal(), w.tenant, result.CaseID, result.EventID)
	if err != nil {
		t.Fatal(err)
	}
	if !payload.SealOK || payload.PayloadDigest != casesDomain.PayloadDigest(payload.Payload) {
		t.Fatalf("expected the sealed payload with a verified seal, got %+v", payload)
	}
	if string(payload.Payload) != `{"call":"inbound-call","from":"+79000000001"}` {
		t.Fatalf("expected the material payload bytes, got %s", payload.Payload)
	}

	// The operator surface: the case list and the detail with the
	// timeline, the honest statements and the work item.
	view, err := w.cases.CasesView(ctx, w.owner, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Cases) != 1 || view.Cases[0].CaseNo != 1 || view.Cases[0].Events != 1 {
		t.Fatalf("expected one case with one event, got %+v", view.Cases)
	}
	detail, err := w.cases.CaseView(ctx, w.owner, w.tenant, result.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Conversations) != 1 || detail.Conversations[0].ResponseOwnerEpoch != 1 || detail.Conversations[0].WritableSurface != "inbox" {
		t.Fatalf("expected the conversation with epoch 1 and the inbox surface, got %+v", detail.Conversations)
	}
	if len(detail.Timeline) != 1 || detail.Timeline[0].Channel != "phone" || detail.Timeline[0].State != "accepted" {
		t.Fatalf("expected the phone timeline event, got %+v", detail.Timeline)
	}
	if len(detail.WorkItems) != 1 || detail.WorkItems[0].State != "untriaged" {
		t.Fatalf("expected the untriaged work item, got %+v", detail.WorkItems)
	}
	if detail.UntriagedText == "" || detail.NoAutoReplyText == "" {
		t.Fatal("expected the honest statements of the slice")
	}

	// The outbox carries the accepted event and the connector-ack; the
	// drain writes the honest deferred outcomes and a repeated drain
	// writes nothing new.
	first, err := w.cases.DrainOutbox(ctx, w.owner, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if first.Deferred != 2 {
		t.Fatalf("expected two deferred outbox records, got %+v", first)
	}
	second, err := w.cases.DrainOutbox(ctx, w.owner, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if second.Deferred != 0 || second.Considered != 0 {
		t.Fatalf("a repeated drain must be a no-op, got %+v", second)
	}
}

// TestIngestReplayReturnsOriginalResultBeforeAdmission covers the material
// idempotency: an identical redelivery returns the original result with the
// replay marker, no second records appear, and the replay does not depend
// on the changed admission states (the entitlement pause after the
// acceptance does not break the sender retry).
func TestIngestReplayReturnsOriginalResultBeforeAdmission(t *testing.T) {
	w := newCasesWorld(t, 320)
	ctx := context.Background()
	command := w.ingestCommand("evt-replay-1", "sess-replay-1", `{"call":"inbound-call","from":"+79000000002"}`)

	first, err := w.cases.IngestEvent(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	original := acceptedOutcome(t, first)

	// The entitlement pause changes the admission word AFTER the
	// acceptance; the replay must still return the original result.
	if _, err := w.entitlement.RunStateTransition(ctx, entitlementAPI.StateCommand{
		Actor: w.owner, TenantID: w.tenant, Action: entitlementDomain.EventPause,
		Basis: "Пауза для теста реплея", CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000711"),
	}); err != nil {
		t.Fatal(err)
	}

	replayed, err := w.cases.IngestEvent(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	repeat := acceptedOutcome(t, replayed)
	if !repeat.Replay {
		t.Fatalf("expected the replay marker, got %+v", repeat)
	}
	if repeat.EventID != original.EventID || repeat.CaseID != original.CaseID ||
		repeat.ConversationID != original.ConversationID || repeat.CaseNo != original.CaseNo {
		t.Fatalf("expected the original identifiers, got %+v vs %+v", repeat, original)
	}
	view, err := w.cases.CasesView(ctx, w.owner, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Cases) != 1 || view.Cases[0].Events != 1 {
		t.Fatalf("a replay must not grow the counters, got %+v", view.Cases)
	}
}

// TestIngestDigestConflictQuarantinesTheDelivery covers the quarantine of
// the same provider event id with another material payload: the accepted
// material is never rewritten and one operational exception is raised.
func TestIngestDigestConflictQuarantinesTheDelivery(t *testing.T) {
	w := newCasesWorld(t, 330)
	ctx := context.Background()
	first, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-conflict-1", "sess-conflict-1", `{"call":"inbound-call","from":"+79000000003"}`))
	if err != nil {
		t.Fatal(err)
	}
	original := acceptedOutcome(t, first)

	conflicting := w.ingestCommand("evt-conflict-1", "sess-conflict-1", `{"call":"inbound-call","from":"+79000000099"}`)
	outcome, err := w.cases.IngestEvent(ctx, conflicting)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Quarantined == nil || outcome.Quarantined.Kind != casesDomain.ExceptionDigestConflict {
		t.Fatalf("expected the digest conflict quarantine, got %+v", outcome)
	}

	// The original case material is intact; the exception is visible to
	// the operator with the correlation id.
	detail, err := w.cases.CaseView(ctx, w.owner, w.tenant, original.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Timeline) != 2 {
		t.Fatalf("expected the accepted event plus the quarantined delivery in the timeline, got %+v", detail.Timeline)
	}
	acceptedStates := map[string]int{}
	for _, event := range detail.Timeline {
		acceptedStates[event.State]++
	}
	if acceptedStates["accepted"] != 1 || acceptedStates["quarantined"] != 1 {
		t.Fatalf("the accepted record stays and the conflicting delivery is quarantined, got %+v", detail.Timeline)
	}
	view, err := w.cases.CasesView(ctx, w.owner, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Exceptions) != 1 || view.Exceptions[0].Kind != casesDomain.ExceptionDigestConflict {
		t.Fatalf("expected one digest-conflict exception, got %+v", view.Exceptions)
	}
}

// TestIngestSessionContinuationExtendsTheCase covers the only proven link
// between events: the continuation of one channel session adds the event
// to the existing conversation and case without a new work item.
func TestIngestSessionContinuationExtendsTheCase(t *testing.T) {
	w := newCasesWorld(t, 340)
	ctx := context.Background()
	first := acceptedOutcome(t, mustIngest(t, w, w.ingestCommand("evt-cont-1", "sess-cont-1", `{"call":"inbound-call","seq":1}`)))
	second := acceptedOutcome(t, mustIngest(t, w, w.ingestCommand("evt-cont-2", "sess-cont-1", `{"call":"inbound-call","seq":2}`)))

	if second.CaseID != first.CaseID || second.ConversationID != first.ConversationID {
		t.Fatalf("expected the same case and conversation, got %+v vs %+v", second, first)
	}
	if second.NewCase {
		t.Fatalf("the continuation is not a new case, got %+v", second)
	}
	detail, err := w.cases.CaseView(ctx, w.owner, w.tenant, first.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Timeline) != 2 || len(detail.WorkItems) != 1 || len(detail.Conversations) != 1 {
		t.Fatalf("expected two events, one conversation and one work item, got %d/%d/%d",
			len(detail.Timeline), len(detail.Conversations), len(detail.WorkItems))
	}
}

func mustIngest(t *testing.T, w *casesWorld, command casesAPI.IngestCommand) casesAPI.IngestOutcome {
	t.Helper()
	outcome, err := w.cases.IngestEvent(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// TestIngestRefusesEveryLadderStep covers the honest refusals of the
// admission ladder with real facts: the unknown connection, the connector
// without a verifier, the channel/connector-type mismatch, the absence of
// an acting decision, the revoked connection and the hard limit of the
// entitlement relay.
func TestIngestRefusesEveryLadderStep(t *testing.T) {
	w := newCasesWorld(t, 350)
	ctx := context.Background()

	// unknown_connection: the claimed tenant does not hold the connection.
	foreign := w.ingestCommand("evt-unknown-1", "sess-unknown-1", `{"call":"x"}`)
	foreign.Envelope.ConnectionID = id.MustParse("01994f64-6c00-7000-8000-00000000deaf")
	foreign.Envelope.TenantClaim = w.tenant
	refusedOutcome(t, mustIngest(t, w, foreign), casesDomain.KindUnknownConnection)

	// production_forbidden and the connector-without-verifier branch are
	// covered by the ingresssyn unit test: the real catalog admits exactly
	// one creatable connector (the verifiable synthetic telephony), so the
	// fail-closed family is structurally unreachable over live data — the
	// honest split of the coverage.

	// channel_mismatch: a text channel on the SIP/telephony connection.
	mismatch := w.ingestCommand("evt-mismatch-1", "sess-mismatch-1", `{"message":"x"}`)
	mismatch.Envelope.Channel = "web_chat"
	mismatch.Envelope.Modality = "text"
	refusedOutcome(t, mustIngest(t, w, mismatch), casesDomain.KindChannelMismatch)

	// no_active_release: before any publication the external traffic is
	// not admitted — a fresh world without the synthetic decision.
	freshWorld := buildUnpublishedWorld(t, 351)
	freshOutcome, err := freshWorld.cases.IngestEvent(ctx, freshWorld.ingestCommand("evt-fresh-1", "sess-fresh-1", `{"call":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	refusedOutcome(t, freshOutcome, casesDomain.KindNoActiveRelease)

	// revoked_connection: the connection is revoked after the source is
	// verified — the refusal fires before the case material.
	current, err := w.connector.Read(ctx, w.owner, w.tenant, w.connection)
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := w.connector.Revoke(ctx, connectorAPI.RevokeCommand{
		Actor: w.owner, TenantID: w.tenant, ConnectionID: w.connection,
		ExpectedVersion: current.Version, Reason: "Отзыв для теста ingress",
		CorrelationID:  id.MustParse("01994f64-6c00-7000-8000-000000000713"),
		IdempotencyKey: "cases-revoke-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if revoked.Connection.Status != connectorDomain.StatusRevoked {
		t.Fatalf("expected the revoked connection, got %s", revoked.Connection.Status)
	}
	revokedEvent := w.ingestCommand("evt-revoked-1", "sess-revoked-1", `{"call":"x"}`)
	refusedOutcome(t, mustIngest(t, w, revokedEvent), casesDomain.KindRevokedConnection)

	// entitlement_blocked: the hard limit bounds new conversations; the
	// bounded refusal carries the honest reason.
	hardLimited := buildHardLimitWorld(t, 352)
	hardOutcome, err := hardLimited.cases.IngestEvent(ctx, hardLimited.ingestCommand("evt-hard-1", "sess-hard-1", `{"call":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if hardOutcome.Refused == nil || hardOutcome.Refused.Kind != casesDomain.KindEntitlementBlocked || !hardOutcome.Refused.Bounded {
		t.Fatalf("expected the bounded entitlement refusal, got %+v", hardOutcome.Refused)
	}
}

// buildUnpublishedWorld builds the full readiness candidate WITHOUT the
// publication: the ingress must refuse with no_active_release.
func buildUnpublishedWorld(t *testing.T, seed int) *casesWorld {
	t.Helper()
	w, employee, link := newReadinessWorld(t, seed)
	w.employee = w.employee.WithEntitlementAdmissionFacts(w.entitlement)
	ctx := context.Background()
	connections, err := w.connector.List(ctx, w.owner, w.tenant)
	if err != nil || len(connections) == 0 {
		t.Fatalf("expected the world connection: %v", err)
	}
	world := &casesWorld{readinessWorld: w, employeeID: employee, connection: connections[0].ConnectionID, manifestLink: link}
	world.cases = world.buildCasesService(newPrivacyIDs(seed + 900))
	return world
}

// buildHardLimitWorld publishes the candidate, then sets a hard limit and
// reaches it with a confirmed usage event: the relayed admission word
// becomes blocked for new conversations.
func buildHardLimitWorld(t *testing.T, seed int) *casesWorld {
	t.Helper()
	world := newCasesWorld(t, seed)
	ctx := context.Background()
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	if _, err := world.entitlement.RegisterSource(ctx, entitlementAPI.SourceCommand{
		Actor: world.owner, TenantID: world.tenant, Name: "manual",
		Categories:       []entitlementDomain.UsageCategory{entitlementDomain.CategoryCarrier},
		FreshnessMinutes: 60, Note: "Ручной источник для теста ingress",
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000718"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := world.entitlement.SetBudget(ctx, entitlementAPI.BudgetCommand{
		Actor: world.owner, TenantID: world.tenant, Amount: 1000, Currency: "RUB",
		PeriodStart: start, PeriodEnd: end, WarningPercent: 80,
		HardLimit: &[]float64{1000}[0], LimitPolicy: entitlementDomain.PolicyBlockNew,
		Note: "Хард-лимит для теста ingress", CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000714"),
	}); err != nil {
		t.Fatal(err)
	}
	amount := 2000.0
	if _, _, err := world.entitlement.IngestUsage(ctx, entitlementAPI.UsageEventCommand{
		Actor: world.owner, TenantID: world.tenant, EventKey: "cases-usage-1",
		Source: "manual", Category: entitlementDomain.CategoryCarrier, Quantity: 2000,
		Amount: &amount, Currency: "RUB", Estimate: false,
		PeriodStart: start, PeriodEnd: end, OccurredAt: casesClock.Now(), Provenance: "тест",
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000715"),
	}); err != nil {
		t.Fatal(err)
	}
	return world
}

// TestIngestNoResponseOwnerRefusesRoute covers the owner step: a route
// with an inactive response owner is not admitted for external traffic —
// the second profile version pins the coverage to a member that is then
// revoked.
func TestIngestNoResponseOwnerRefusesRoute(t *testing.T) {
	w := newCasesWorld(t, 360)
	ctx := context.Background()
	// A dedicated member becomes the pinned response owner.
	leaving := inviteMember(t, w.identity, w.owner, w.tenant, fmt.Sprintf("cases-leaving-%03d@example.org", 360),
		identityDomain.Operator, []identityDomain.Scope{identityDomain.Operations}, "000000000360", "000000000361")

	// The second profile version pins the coverage to the leaving member
	// over the EXACT manifest of the acting candidate (the world link).
	content := profileContent(leaving.ID, w.backup)
	actor := actorPrincipal{principalID: w.owner.ID, stepUp: true}
	if _, err := w.saveDraft(t, w.employeeID, w.manifestLink, content, 2, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := w.approve(t, w.employeeID, 3, actor); err != nil {
		t.Fatal(err)
	}
	// A new decision pins the new profile version: the new candidate starts
	// at its own synthetic stage (the ladder is per candidate).
	if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
		Actor: actor.principal(), TenantID: w.tenant, EmployeeID: w.employeeID,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000716"),
		Stage:         employeeDomain.StageSynthetic,
	}); err != nil {
		t.Fatal(err)
	}
	// The leaving member is revoked: the pinned owner is no longer active.
	members, err := w.identity.ListMembers(ctx, w.owner, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	for _, membership := range members {
		if membership.PrincipalID != nil && *membership.PrincipalID == leaving.ID {
			if err := w.identity.RevokeMember(ctx, identityAPI.MembershipCommand{
				Actor: actor.principal(), TenantID: w.tenant, TargetID: leaving.ID,
				ExpectedVersion: membership.Version, Reason: "offboarding",
				CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000717"),
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	outcome, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-owner-1", "sess-owner-1", `{"call":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	refusedOutcome(t, outcome, casesDomain.KindNoResponseOwner)
}

// TestIngestRaceWritesExactlyOneCase covers the material race: two
// concurrent identical deliveries resolve to exactly one case and one
// accepted event; the loser replays the winner's result.
func TestIngestRaceWritesExactlyOneCase(t *testing.T) {
	w := newCasesWorld(t, 370)
	// The race needs the thread-safe real generator.
	w.cases = w.buildCasesService(&employeeIDs{})
	ctx := context.Background()
	command := w.ingestCommand("evt-race-1", "sess-race-1", `{"call":"inbound-call","from":"+79000000004"}`)

	outcomes := make([]casesAPI.IngestOutcome, 2)
	errs := make([]error, 2)
	var wait sync.WaitGroup
	for index := range outcomes {
		wait.Add(1)
		go func(slot int) {
			defer wait.Done()
			outcomes[slot], errs[slot] = w.cases.IngestEvent(ctx, command)
		}(index)
	}
	wait.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	accepted := 0
	for _, outcome := range outcomes {
		if outcome.Accepted == nil {
			t.Fatalf("expected both deliveries to resolve, got %+v", outcome)
		}
		if !outcome.Accepted.Replay {
			accepted++
		}
	}
	if accepted != 1 {
		t.Fatalf("expected exactly one acceptance, got %d", accepted)
	}
	view, err := w.cases.CasesView(ctx, w.owner, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Cases) != 1 || view.Cases[0].Events != 1 {
		t.Fatalf("expected exactly one case with one event, got %+v", view.Cases)
	}
}

// TestCasesReadsRespectScopesAndTenancy covers the operator surface
// authorization: the review member sees the surface and the payload, the
// operations-only member is refused, a foreign principal gets a plain 404
// (the tenant existence is never disclosed).
func TestCasesReadsRespectScopesAndTenancy(t *testing.T) {
	w := newCasesWorld(t, 380)
	ctx := context.Background()
	result := acceptedOutcome(t, mustIngest(t, w, w.ingestCommand("evt-scopes-1", "sess-scopes-1", `{"call":"x"}`)))

	reviewer := inviteMember(t, w.identity, w.owner, w.tenant, "cases-reviewer@example.org",
		identityDomain.ExpertQA, []identityDomain.Scope{identityDomain.Review, identityDomain.Audit}, "000000000380", "000000000381")
	operator := inviteMember(t, w.identity, w.owner, w.tenant, "cases-operator@example.org",
		identityDomain.Operator, []identityDomain.Scope{identityDomain.Operations}, "000000000382", "000000000383")

	if _, err := w.cases.CasesView(ctx, reviewer, w.tenant); err != nil {
		t.Fatalf("the review member must see the cases: %v", err)
	}
	detail, err := w.cases.CaseView(ctx, reviewer, w.tenant, result.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.CanViewPayload {
		t.Fatal("the review member must be allowed the payload boundary")
	}
	payload, err := w.cases.ReadPayload(ctx, reviewer, w.tenant, result.CaseID, result.EventID)
	if err != nil {
		t.Fatalf("the review member must read the sealed payload: %v", err)
	}
	if !payload.SealOK {
		t.Fatal("expected the verified seal")
	}

	if _, err := w.cases.CasesView(ctx, operator, w.tenant); !errors.Is(err, identityDomain.ErrForbidden) {
		t.Fatalf("the operations-only member is honestly refused, got %v", err)
	}
	if _, err := w.cases.ReadPayload(ctx, operator, w.tenant, result.CaseID, result.EventID); !errors.Is(err, identityDomain.ErrForbidden) {
		t.Fatalf("the payload boundary refuses the operator, got %v", err)
	}

	foreign := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-00000000038f")}
	if _, err := w.cases.CasesView(ctx, foreign, w.tenant); !errors.Is(err, identityDomain.ErrNotFound) {
		t.Fatalf("a foreign principal gets a plain 404, got %v", err)
	}
}

// TestIngressCrossTenantClaimIsHonest covers the scope rule: an envelope
// claiming another tenant with this tenant's connection is an honest
// unknown_connection (the RLS-hiding of the verifier lookup).
func TestIngressCrossTenantClaimIsHonest(t *testing.T) {
	w := newCasesWorld(t, 390)
	command := w.ingestCommand("evt-cross-1", "sess-cross-1", `{"call":"x"}`)
	command.Envelope.TenantClaim = id.MustParse("01994f64-6c00-7000-8000-00000000039f")
	outcome, err := w.cases.IngestEvent(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	refusedOutcome(t, outcome, casesDomain.KindUnknownConnection)
}

// TestIngressEnvelopeBoundsBoundedOutcome covers the load envelope: a
// payload above the fixed dictionary limit is refused with the bounded
// envelope_exceeded outcome and a counter, never silently accepted.
func TestIngressEnvelopeBoundsBoundedOutcome(t *testing.T) {
	w := newCasesWorld(t, 400)
	command := w.ingestCommand("evt-envelope-1", "sess-envelope-1", `{"call":"x"}`)
	command.Envelope.Payload = []byte(`{"padding":"` + string(make([]byte, 0)) + `"}`)
	oversized := make([]byte, casesDomain.FixedLoadEnvelope.MaxPayloadBytes+1)
	for index := range oversized {
		oversized[index] = 'a'
	}
	command.Envelope.Payload = append([]byte(`{"padding":"`), oversized...)
	command.Envelope.Payload = append(command.Envelope.Payload, []byte(`"}`)...)
	command.Envelope.PayloadDigest = casesDomain.PayloadDigest(command.Envelope.Payload)
	command.Envelope.EnvelopeBytes = len(command.Envelope.Payload) + 64
	outcome, err := w.cases.IngestEvent(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Refused == nil || outcome.Refused.Kind != casesDomain.KindEnvelopeExceeded || !outcome.Refused.Bounded || outcome.Refused.Counter == 0 {
		t.Fatalf("expected the bounded envelope refusal with the counter, got %+v", outcome.Refused)
	}
}

// TestIngestSessionContinuationOutboxCarriesTrueMaterial asserts the R1
// fix: the accepted-event outbox record of a session continuation is
// marshalled inside the transaction with the RESOLVED material — the
// canonical JSON of an extending event references the existing case and
// conversation and carries new_case=false.
func TestIngestSessionContinuationOutboxCarriesTrueMaterial(t *testing.T) {
	w := newCasesWorld(t, 410)
	ctx := context.Background()
	first := acceptedOutcome(t, mustIngest(t, w, w.ingestCommand("evt-outbox-1", "sess-outbox-1", `{"call":"inbound-call","seq":1}`)))
	second := acceptedOutcome(t, mustIngest(t, w, w.ingestCommand("evt-outbox-2", "sess-outbox-1", `{"call":"inbound-call","seq":2}`)))
	if second.CaseID != first.CaseID {
		t.Fatal("expected the continuation of the same session")
	}
	store := casesStore.NewPostgres(runtimePool)
	scope, err := tenancy.New(w.connection, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := store.PendingOutbox(ctx, scope, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 4 {
		t.Fatalf("expected two outbox records per accepted event, got %d", len(pending))
	}
	acceptedRecords := map[string]casesDomain.AcceptedEventPayload{}
	for _, event := range pending {
		if event.EventType != casesDomain.OutboxEventAccepted {
			continue
		}
		var payload casesDomain.AcceptedEventPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		acceptedRecords[payload.ProviderEventID] = payload
	}
	if len(acceptedRecords) != 2 {
		t.Fatalf("expected one accepted-event record per event, got %d", len(acceptedRecords))
	}
	extension := acceptedRecords["evt-outbox-2"]
	if extension.CaseID != first.CaseID || extension.ConversationID != first.ConversationID {
		t.Fatalf("the continuation record must reference the resolved material, got %+v", extension)
	}
	if extension.NewCase {
		t.Fatalf("the continuation is not a new case, got %+v", extension)
	}
	// The referenced material exists — the record never carries fabricated
	// identifiers.
	detail, err := w.cases.CaseView(ctx, w.owner, w.tenant, extension.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Timeline) != 2 {
		t.Fatalf("expected both events of the session, got %d", len(detail.Timeline))
	}
}

// TestIngestIdempotencyKeyConflictQuarantines covers the R1 fix: a
// delivery reusing an idempotency key already held by an accepted event of
// another provider event id is quarantined with the honest exception —
// never an internal error.
func TestIngestIdempotencyKeyConflictQuarantines(t *testing.T) {
	w := newCasesWorld(t, 420)
	ctx := context.Background()
	acceptedOutcome(t, mustIngest(t, w, w.ingestCommand("evt-key-1", "sess-key-1", `{"call":"x"}`)))
	// The second event has another provider event id and session, but the
	// SAME idempotency key.
	conflicting := w.ingestCommand("evt-key-2", "sess-key-2", `{"call":"y"}`)
	original := w.ingestCommand("evt-key-1", "sess-key-1", `{"call":"x"}`)
	conflicting.IdempotencyKey = original.IdempotencyKey
	outcome, err := w.cases.IngestEvent(ctx, conflicting)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Quarantined == nil || outcome.Quarantined.Kind != casesDomain.ExceptionIdempotencyConflict {
		t.Fatalf("expected the idempotency-key conflict quarantine, got %+v", outcome)
	}
	view, err := w.cases.CasesView(ctx, w.owner, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, exception := range view.Exceptions {
		if exception.Kind == casesDomain.ExceptionIdempotencyConflict {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the idempotency conflict exception for the operator, got %+v", view.Exceptions)
	}
}

// TestIngestAmbiguousRoutingQuarantines covers the full write path of the
// ambiguous-routing matrix line: two employees with acting decisions that
// both seal the connection quarantine the delivery and raise one
// operational exception; no case is created.
func TestIngestAmbiguousRoutingQuarantines(t *testing.T) {
	w := newCasesWorld(t, 430)
	ctx := context.Background()
	// The second employee walks the same candidate path over the SAME
	// tenant connection: its manifest seals the connection too, so two
	// acting decisions resolve the route.
	employee2 := w.createEmployee(t, true)
	w.pinPack(t, employee2)
	w.saveBehavior(t, employee2, 3)
	w.addKnowledge(t, employee2)
	w.addVoice(t, employee2)
	draft := w.assemble(t, employee2, 0)
	version := w.createVersion(t, employee2, draft.Revision, "Кандидат второго сотрудника")
	manifest, err := w.seal(t, employee2, version.Version, draft.Revision+1)
	if err != nil {
		t.Fatal(err)
	}
	link2 := employeeDomain.ManifestLink{ManifestID: manifest.ManifestID, ConfigVersion: manifest.ConfigVersion, Digest: manifest.Digest}
	owner := actorPrincipal{principalID: w.owner.ID, stepUp: true}
	if _, err := w.saveDraft(t, employee2, link2, profileContent(w.owner.ID, w.backup), 0, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := w.approve(t, employee2, 1, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
		Actor: owner.principal(), TenantID: w.tenant, EmployeeID: employee2,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000719"),
		Stage:         employeeDomain.StageSynthetic,
	}); err != nil {
		t.Fatal(err)
	}
	outcome, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-ambiguous-1", "sess-ambiguous-1", `{"call":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Quarantined == nil || outcome.Quarantined.Kind != casesDomain.ExceptionAmbiguousRouting {
		t.Fatalf("expected the ambiguous-routing quarantine, got %+v", outcome)
	}
	view, err := w.cases.CasesView(ctx, w.owner, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Cases) != 0 {
		t.Fatalf("no case may be created on ambiguity, got %d", len(view.Cases))
	}
	if len(view.Exceptions) != 1 || view.Exceptions[0].Kind != casesDomain.ExceptionAmbiguousRouting {
		t.Fatalf("expected one ambiguous-routing exception, got %+v", view.Exceptions)
	}
}

// TestIngestSessionContinuationFailClosed covers the matrix line: an event
// of an EXISTING session is refused fail-closed when the admission has
// changed since the acceptance — the session is not extended.
func TestIngestSessionContinuationFailClosed(t *testing.T) {
	w := newCasesWorld(t, 440)
	ctx := context.Background()
	first := acceptedOutcome(t, mustIngest(t, w, w.ingestCommand("evt-failclosed-1", "sess-failclosed-1", `{"call":"x"}`)))
	// The connection is revoked after the acceptance: a NEW event of the
	// same session is refused (a replay of the already-accepted event
	// still returns the original result — replay goes before admission).
	current, err := w.connector.Read(ctx, w.owner, w.tenant, w.connection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.connector.Revoke(ctx, connectorAPI.RevokeCommand{
		Actor: w.owner, TenantID: w.tenant, ConnectionID: w.connection,
		ExpectedVersion: current.Version, Reason: "Отзыв для теста fail-closed",
		CorrelationID:  id.MustParse("01994f64-6c00-7000-8000-00000000071a"),
		IdempotencyKey: "cases-revoke-failclosed",
	}); err != nil {
		t.Fatal(err)
	}
	newEvent := w.ingestCommand("evt-failclosed-2", "sess-failclosed-1", `{"call":"y"}`)
	refusedOutcome(t, mustIngest(t, w, newEvent), casesDomain.KindRevokedConnection)
	// The session was not extended: exactly one accepted event.
	detail, err := w.cases.CaseView(ctx, w.owner, w.tenant, first.CaseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Timeline) != 1 {
		t.Fatalf("the revoked admission must not extend the session, got %d events", len(detail.Timeline))
	}
	// The identical redelivery of the accepted event still replays (the
	// retry does not depend on the changed states).
	replay := acceptedOutcome(t, mustIngest(t, w, w.ingestCommand("evt-failclosed-1", "sess-failclosed-1", `{"call":"x"}`)))
	if !replay.Replay || replay.EventID != first.EventID {
		t.Fatalf("expected the replay of the accepted event despite the revoked admission, got %+v", replay)
	}
}
