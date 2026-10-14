package domain

import (
	"strings"
	"testing"
	"time"
)

// The Story 2.12 unit matrix (the route degradation half): the cause /
// path / promise dictionaries, the read-time derivation of the current
// slots (latest-per-route with the honest recovery), the canonical
// snapshot map, the record guard, the classless honest timing gate and
// the degraded commercial path of the deterministic policy v3 (the
// question plan that stays, the control branches and the fail-closed
// stop).

func degradationRoute() RouteKey {
	return RouteKey{ConnectionID: testConnectionID(), Channel: "phone"}
}

func degradationRecordOf(route RouteKey, cause string, revision int64, cleared bool) RouteDegradationRecord {
	return RouteDegradationRecord{
		TenantID: newID(), CaseID: newID(), RecordID: newID(),
		Route: route, Cause: cause, Revision: revision, Cleared: cleared,
		Note: "операционное наблюдение", ActorID: newID(), RecordedAt: time.Now(),
	}
}

func degradedCurrent(revision int64, cleared bool) CurrentDegradations {
	route := degradationRoute()
	return CurrentDegradations{route.String(): DegradationState{
		Route: route, Cause: DegradationCauseTimeout, Revision: revision, Cleared: cleared,
	}}
}

func TestDegradationDictionariesClosed(t *testing.T) {
	if len(DegradationCauses) != 5 {
		t.Fatalf("the cause dictionary carries the five AC-1 words, got %d", len(DegradationCauses))
	}
	seen := map[string]bool{}
	for _, cause := range DegradationCauses {
		if seen[cause] {
			t.Fatalf("the cause word %q repeats", cause)
		}
		seen[cause] = true
		if !DegradationCauseInDictionary(cause) {
			t.Fatalf("the cause word %q is not in the dictionary", cause)
		}
		if DegradationCauseText(cause) == "" {
			t.Fatalf("every cause word carries its honest text, %q does not", cause)
		}
	}
	if DegradationCauseInDictionary("slow_channel") || DegradationCauseInDictionary("") {
		t.Fatalf("an unknown cause never enters the dictionary")
	}
	if DegradationCauseText("made_up_cause") != "" {
		t.Fatalf("an unknown cause renders no text")
	}
	if len(DegradedPermittedPathWords) != 5 {
		t.Fatalf("the permitted path dictionary carries the five AC-3 words, got %d", len(DegradedPermittedPathWords))
	}
	for _, word := range DegradedPermittedPathWords {
		if !DegradedPathInDictionary(word) {
			t.Fatalf("the path word %q is not in the dictionary", word)
		}
		if DegradedPathText(word) == "" {
			t.Fatalf("every path word carries its honest text, %q does not", word)
		}
	}
	if DegradedPathInDictionary("promise_a_price") {
		t.Fatalf("an invented path never enters the dictionary (the set never widens)")
	}
	if len(UpdatePromiseWords) != 4 {
		t.Fatalf("the promise dictionary carries the four policy words, got %d", len(UpdatePromiseWords))
	}
	for _, word := range UpdatePromiseWords {
		if !UpdatePromiseInDictionary(word) {
			t.Fatalf("the promise word %q is not in the dictionary", word)
		}
		if UpdatePromiseText(word) == "" {
			t.Fatalf("every promise word carries its honest text, %q does not", word)
		}
	}
	// The structural honesty of the promise timing: the exact-hours words
	// stay outside the dictionary — a promise never invents a precision
	// the policy did not approve.
	if UpdatePromiseInDictionary("within_2h") || UpdatePromiseInDictionary("at_15_00") {
		t.Fatalf("a foreign timing word never enters the approved dictionary")
	}
}

func TestDeriveCurrentDegradationsLatestPerRoute(t *testing.T) {
	route := degradationRoute()
	records := []RouteDegradationRecord{
		degradationRecordOf(route, DegradationCauseTimeout, 1, false),
		degradationRecordOf(route, DegradationCauseDisconnect, 2, false),
	}
	current := DeriveCurrentDegradations(records)
	state, known := current.RouteState(route)
	if !known || state.Revision != 2 || state.Cause != DegradationCauseDisconnect || state.Cleared {
		t.Fatalf("the latest revision of the route wins, got %+v (known=%v)", state, known)
	}
	if !current.RouteDegraded(route) {
		t.Fatalf("the route with a non-clearing latest revision is degraded")
	}
	// The clearing revision restores the route: the state stays in the
	// map (the honest revision truth), the route leaves the degraded set.
	restored := DeriveCurrentDegradations(append(records, degradationRecordOf(route, DegradationCauseDisconnect, 3, true)))
	state, known = restored.RouteState(route)
	if !known || state.Revision != 3 || !state.Cleared {
		t.Fatalf("the clearing revision is the latest truth, got %+v", state)
	}
	if restored.RouteDegraded(route) {
		t.Fatalf("the cleared route is not degraded")
	}
	if len(restored.DegradedRoutes()) != 0 {
		t.Fatalf("the degraded set is empty after the recovery, got %v", restored.DegradedRoutes())
	}
	// The independent route is never touched by another route's row.
	other := RouteKey{ConnectionID: testConnectionID(), Channel: "email"}
	if restored.RouteDegraded(other) {
		t.Fatalf("an independent route (another channel) is never degraded without its own row")
	}
	mixed := DeriveCurrentDegradations([]RouteDegradationRecord{
		degradationRecordOf(route, DegradationCauseTimeout, 1, false),
		degradationRecordOf(other, DegradationCauseRateLimited, 1, false),
	})
	if !mixed.RouteDegraded(route) || !mixed.RouteDegraded(other) {
		t.Fatalf("both routes degrade by their own rows")
	}
	if len(mixed.DegradedRoutes()) != 2 {
		t.Fatalf("the degraded set carries both routes, got %v", mixed.DegradedRoutes())
	}
	// The defensive tie: the LATER record of an equal revision wins (the
	// UNIQUE backstop makes the case unreachable — the derivation stays
	// deterministic).
	tie := DeriveCurrentDegradations([]RouteDegradationRecord{
		degradationRecordOf(route, DegradationCauseTimeout, 1, false),
		degradationRecordOf(route, DegradationCauseCapabilityRevoked, 1, false),
	})
	if state, _ := tie.RouteState(route); state.Cause != DegradationCauseCapabilityRevoked {
		t.Fatalf("the later record of an equal revision wins, got %+v", state)
	}
	// The empty history degrades nothing.
	if empty := DeriveCurrentDegradations(nil); empty.RouteDegraded(route) || len(empty) != 0 {
		t.Fatalf("no records — no degraded routes")
	}
}

func TestDegradationSnapshotOfCanonical(t *testing.T) {
	route := degradationRoute()
	snapshot := DegradationSnapshotOf(DegradationCurrentOf(route, 2, false))
	if len(snapshot) != 1 {
		t.Fatalf("the snapshot carries the route, got %v", snapshot)
	}
	entry, ok := snapshot[route.String()]
	if !ok || entry.Cause != DegradationCauseTimeout || entry.Revision != 2 || entry.Cleared {
		t.Fatalf("the snapshot entry mirrors the state, got %+v", entry)
	}
	// The clearing revision stays in the map — the honest supersede of the
	// recovered route (the digest changes on a recovery too).
	recovered := DegradationSnapshotOf(DegradationCurrentOf(route, 3, true))
	entry, ok = recovered[route.String()]
	if !ok || !entry.Cleared {
		t.Fatalf("the cleared route stays in the snapshot map, got %+v", entry)
	}
	// The empty state renders the initialized map — the canonical {} of the
	// digest determinism (never nil).
	if empty := DegradationSnapshotOf(nil); empty == nil || len(empty) != 0 {
		t.Fatalf("the empty state renders {}, got %v", empty)
	}
}

// DegradationCurrentOf builds one-route current state (the test helper).
func DegradationCurrentOf(route RouteKey, revision int64, cleared bool) CurrentDegradations {
	return CurrentDegradations{route.String(): DegradationState{
		Route: route, Cause: DegradationCauseTimeout, Revision: revision, Cleared: cleared,
	}}
}

func TestGuardRouteDegradationRecordRules(t *testing.T) {
	route := degradationRoute()
	if err := GuardRouteDegradationRecord(route, DegradationCauseTimeout, "заметка"); err != nil {
		t.Fatalf("the dictionary material passes, got %v", err)
	}
	// The empty note is honest material (the words carry the truth).
	if err := GuardRouteDegradationRecord(route, DegradationCauseDeliveryUnknown, ""); err != nil {
		t.Fatalf("the empty note passes, got %v", err)
	}
	if err := GuardRouteDegradationRecord(RouteKey{ConnectionID: route.ConnectionID, Channel: "sms"}, DegradationCauseTimeout, ""); err == nil {
		t.Fatalf("a channel outside the 2.1 dictionary is refused")
	}
	if err := GuardRouteDegradationRecord(RouteKey{Channel: "phone"}, DegradationCauseTimeout, ""); err == nil {
		t.Fatalf("an invalid connection id is refused")
	}
	if err := GuardRouteDegradationRecord(route, "provider_napping", ""); err == nil {
		t.Fatalf("an unknown cause word is refused")
	}
	long := strings.Repeat("з", MaxDegradationNoteRunes+1)
	if err := GuardRouteDegradationRecord(route, DegradationCauseTimeout, long); err == nil {
		t.Fatalf("a note over the rune bound is refused")
	}
}

func TestGuardUpdatePromiseTimingGate(t *testing.T) {
	// The degraded route: the promise word is REQUIRED and must belong to
	// the approved dictionary.
	missing := GuardUpdatePromise(true, "")
	if missing == nil {
		t.Fatalf("a reply on a degraded route without the promise is refused")
	}
	if workRule, ok := missing.(WorkRuleError); !ok || workRule.Rule != "update_promise_required" {
		t.Fatalf("the missing promise names the update_promise_required rule, got %v", missing)
	}
	if err := GuardUpdatePromise(true, UpdatePromiseWithin4h); err != nil {
		t.Fatalf("the dictionary promise word passes, got %v", err)
	}
	foreign := GuardUpdatePromise(true, "within_2h")
	if foreign == nil {
		t.Fatalf("a foreign promise word is refused on the degraded route")
	}
	if workRule, ok := foreign.(WorkRuleError); !ok || workRule.Rule != "update_promise_foreign" {
		t.Fatalf("the foreign word names the update_promise_foreign rule, got %v", foreign)
	}
	if !strings.Contains(foreign.Error(), "within_1h") {
		t.Fatalf("the refusal names the dictionary words, got %v", foreign)
	}
	// The healthy route: the field must stay EMPTY — the control branch is
	// byte-identical.
	if err := GuardUpdatePromise(false, ""); err != nil {
		t.Fatalf("the healthy route carries no promise, got %v", err)
	}
	healthy := GuardUpdatePromise(false, UpdatePromiseNoPromise)
	if healthy == nil {
		t.Fatalf("a promise on a healthy route is refused")
	}
	if workRule, ok := healthy.(WorkRuleError); !ok || workRule.Rule != "update_promise_foreign" {
		t.Fatalf("the healthy-route refusal names the foreign rule, got %v", healthy)
	}
}

func TestRouteDegradationOutboxPerimeter(t *testing.T) {
	payload, err := RouteDegradationRecordedPayloadJSON(RouteDegradationRecordedPayload{
		TenantID: newID(), CaseID: newID(), RecordID: newID(),
		ConnectionID: testConnectionID(), Channel: "phone",
		Cause: DegradationCauseTimeout, Revision: 2, Cleared: false,
		ActorID: newID(), OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "заметка") || strings.Contains(string(payload), "note") {
		t.Fatalf("the degradation envelope never carries the operator note")
	}
	for _, word := range []string{"cause", "revision", "cleared", "channel"} {
		if !strings.Contains(string(payload), word) {
			t.Fatalf("the envelope carries the identifiers and the dictionary words, %q missing", word)
		}
	}
	if key := DedupKeyRouteDegradationRecorded(newID()); !strings.Contains(key, OutboxEventRouteDegradationRecorded) {
		t.Fatalf("the dedup key names the event type, got %q", key)
	}
}

func TestEvaluatePolicyDegradedMode(t *testing.T) {
	// The degraded dialog route of a commercial intent: the class
	// degrades, the permitted continuation words travel, the question
	// plan STAYS (questions are not promises) and the commercial answer
	// never issues.
	proposal := AIProposal{Intent: IntentPriceInquiry, CandidateAnswer: "Есть вариант"}
	decision := EvaluatePolicy(proposal, confirmedWorld(), ModeSales, nil, degradedCurrent(1, false), degradationRoute())
	if decision.OutcomeClass != OutcomeDegradedMode || decision.Reason != ReasonDegradedRoute {
		t.Fatalf("the degraded route degrades the commercial path, got %+v", decision)
	}
	if len(decision.DegradedPermittedPaths) != 5 {
		t.Fatalf("the full pack dictionary of the permitted paths travels, got %v", decision.DegradedPermittedPaths)
	}
	if decision.OutcomeClass == OutcomeCommercialAnswer {
		t.Fatalf("the commercial answer never issues on a degraded route")
	}
	asked := map[string]bool{}
	for _, step := range decision.NextSteps {
		if step.Step == StepAskFact {
			asked[step.MaterialKey] = true
		} else {
			t.Fatalf("the degraded decision carries the questions only, got %+v", decision.NextSteps)
		}
	}
	if !asked["service_kind"] || !asked["urgency"] {
		t.Fatalf("the question plan stays (questions are not promises), got %+v", decision.NextSteps)
	}
	// The empty candidate of a commercial intent on a degraded route: the
	// SAME degraded decision (the clarify split never reaches — the class
	// degrades first).
	noCandidate := EvaluatePolicy(AIProposal{Intent: IntentPriceInquiry}, confirmedWorld(), ModeSales, nil, degradedCurrent(1, false), degradationRoute())
	if noCandidate.OutcomeClass != OutcomeDegradedMode || len(noCandidate.DegradedPermittedPaths) != 5 {
		t.Fatalf("the empty candidate degrades the same way, got %+v", noCandidate)
	}
}

func TestEvaluatePolicyDegradedControlBranches(t *testing.T) {
	proposal := AIProposal{Intent: IntentPriceInquiry, CandidateAnswer: "Есть вариант"}
	// The control branch: no degradations — the decision stays
	// byte-identical to the former v2 form (the class, the reason, the
	// plan and the empty paths).
	former := EvaluatePolicy(proposal, confirmedWorld(), ModeSales, nil, nil, RouteKey{})
	if former.OutcomeClass != OutcomeCommercialAnswer || former.Reason != ReasonOkCommercial || len(former.DegradedPermittedPaths) != 0 {
		t.Fatalf("no degradations — the former decision, got %+v", former)
	}
	// The route is not the dialog route: an independent route's row never
	// degrades the dialog (the derivation is per the exact key).
	otherRoute := RouteKey{ConnectionID: testConnectionID(), Channel: "email"}
	independent := EvaluatePolicy(proposal, confirmedWorld(), ModeSales, nil, DegradationCurrentOf(otherRoute, 1, false), degradationRoute())
	if independent.OutcomeClass != OutcomeCommercialAnswer {
		t.Fatalf("an independent route's degradation never touches the dialog route, got %+v", independent)
	}
	// The cleared (recovered) route is the control branch.
	recovered := EvaluatePolicy(proposal, confirmedWorld(), ModeSales, nil, degradedCurrent(2, true), degradationRoute())
	if recovered.OutcomeClass != OutcomeCommercialAnswer {
		t.Fatalf("the recovered route restores the commercial path, got %+v", recovered)
	}
	// The non-commercial intent on a degraded route: the informational
	// answer as before (a short failure message is not a promise).
	general := EvaluatePolicy(AIProposal{Intent: IntentGeneralQuestion, CandidateAnswer: "Часы: 9-18"}, confirmedWorld(), ModeSales, nil, degradedCurrent(1, false), degradationRoute())
	if general.OutcomeClass != OutcomeInformationalAnswer || general.Reason != ReasonOkInformational {
		t.Fatalf("the non-commercial intent stays informational on a degraded route, got %+v", general)
	}
	// The mode gate fires EARLIER: a non-sales mode with a degraded route
	// is the mode block (the load-bearing ladder order never changes).
	blocked := EvaluatePolicy(proposal, confirmedWorld(), ModeSafety, nil, degradedCurrent(1, false), degradationRoute())
	if blocked.OutcomeClass != OutcomeModeBlocked {
		t.Fatalf("the mode gate fires before the degraded branch, got %+v", blocked)
	}
	// The degraded branch does not weaken the 2.11 alternatives trigger:
	// the no_photo pair on a degraded route still attaches the media
	// alternatives alongside the degraded paths (both limitations are
	// honestly true).
	both := EvaluatePolicy(proposal, confirmedWorld(), ModeSales, noPhotoCurrent(1, false), degradedCurrent(1, false), degradationRoute())
	if both.OutcomeClass != OutcomeDegradedMode || len(both.DegradedPermittedPaths) != 5 {
		t.Fatalf("the degraded decision stays degraded with the preferences trigger, got %+v", both)
	}
	if len(both.ActiveAlternatives) != 4 {
		t.Fatalf("the 2.11 alternatives still attach on the degraded branch (the check never weakens), got %v", both.ActiveAlternatives)
	}
}

func TestEvaluatePolicyDegradedEmptyPathsFailClosed(t *testing.T) {
	// The empty pack dictionary of the degraded paths: the honest
	// fail-closed stop (the 2.11 mirror — the check never weakens, the
	// path hands off to a human). The fixed v0 dictionary always yields
	// the five words — the unit probe pins the branch for the future pack
	// forms.
	former := DegradedPermittedPathWords
	DegradedPermittedPathWords = nil
	defer func() { DegradedPermittedPathWords = former }()
	decision := EvaluatePolicy(AIProposal{Intent: IntentPriceInquiry, CandidateAnswer: "Есть вариант"}, confirmedWorld(), ModeSales, nil, degradedCurrent(1, false), degradationRoute())
	if decision.OutcomeClass != OutcomeFailClosed || decision.Reason != ReasonNoAlternativePath {
		t.Fatalf("the empty path dictionary fails closed, got %+v", decision)
	}
	if len(decision.NextSteps) != 1 || decision.NextSteps[0].Step != StepHumanHandoff {
		t.Fatalf("the fail-closed stop hands off to a human, got %+v", decision.NextSteps)
	}
	if len(decision.DegradedPermittedPaths) != 0 {
		t.Fatalf("the fail-closed stop offers no paths, got %v", decision.DegradedPermittedPaths)
	}
}

func TestDecisionDigestDegradationsHonestSupersede(t *testing.T) {
	base := PolicyInputSnapshot{
		ProposalID: newID(), SchemaVersion: ProposalSchemaVersion,
		ModeVersion: 2, OwnerEpoch: 1,
		FactsDigest:     FactsDigestMap{"service_kind": {VersionNo: 1, ValueDigest: "digest-service_kind"}},
		FactsDigestHash: "hash", WorkItemVersion: 1, RuleVersion: PolicyRuleVersion,
	}
	decision := PolicyDecision{OutcomeClass: OutcomeDegradedMode, Reason: ReasonDegradedRoute,
		DegradedPermittedPaths: append([]string(nil), DegradedPermittedPathWords...)}
	// The same degradations state produces the same digest (the map is
	// canonical — the empty state renders {}).
	first, second := base, base
	first.Degradations, second.Degradations = DegradationSnapshotOf(nil), DegradationSnapshotOf(nil)
	digestEmpty, err := DecisionDigest(first, decision, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	digestEmpty2, err := DecisionDigest(second, decision, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	if digestEmpty != digestEmpty2 {
		t.Fatalf("the same degradations state produces the same digest, got %q vs %q", digestEmpty, digestEmpty2)
	}
	// A degraded route changes the digest (the honest supersede).
	withRoute := base
	withRoute.Degradations = DegradationSnapshotOf(degradedCurrent(1, false))
	digestDegraded, err := DecisionDigest(withRoute, decision, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	if digestDegraded == digestEmpty {
		t.Fatalf("a degraded route must change the digest (the honest supersede)")
	}
	// The clearing revision changes the digest again (the state, not just
	// the presence, is the input).
	recovered := base
	recovered.Degradations = DegradationSnapshotOf(degradedCurrent(2, true))
	digestRecovered, err := DecisionDigest(recovered, decision, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	if digestRecovered == digestDegraded || digestRecovered == digestEmpty {
		t.Fatalf("the recovery is the honest supersede of the state, got %q", digestRecovered)
	}
	// The permitted paths are part of the decision digest.
	plain := PolicyDecision{OutcomeClass: OutcomeDegradedMode, Reason: ReasonDegradedRoute}
	digestPlain, err := DecisionDigest(withRoute, plain, IntentPriceInquiry)
	if err != nil {
		t.Fatal(err)
	}
	if digestPlain == digestDegraded {
		t.Fatalf("the permitted paths must change the decision digest")
	}
}

func TestPolicyRuleVersionV3Word(t *testing.T) {
	if PolicyRuleVersion != "intake-policy-v3" {
		t.Fatalf("the rule version word of the slice is intake-policy-v3, got %q", PolicyRuleVersion)
	}
	// The class and the reason dictionaries carry the 2.12 words.
	if !inDictionary(OutcomeClasses, OutcomeDegradedMode) {
		t.Fatalf("the outcome classes carry the degraded_mode word")
	}
	if !inDictionary(OutcomeReasons, ReasonDegradedRoute) {
		t.Fatalf("the outcome reasons carry the degraded_route word")
	}
}
