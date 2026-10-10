package domain

import (
	"fmt"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The admission ladder (Story 2.1): pure functions over facts
// ---------------------------------------------------------------------------

// ConnectionFacts is the state of the verified connection as read with the
// trusted scope (the step after the replay check).
type ConnectionFacts struct {
	ConnectionID  id.UUID
	Status        string
	Environment   string
	ConnectorID   string
	ConnectorType string
}

// RouteAdmissionWord is one per-route result of an acting decision.
type RouteAdmissionWord struct {
	Channel   string
	Direction string
	State     string // admitted | blocked
}

// Route returns the route string of the word.
func (w RouteAdmissionWord) Route() string { return w.Channel + ":" + w.Direction }

// CoverageWord is one human coverage row of the pinned profile version:
// the response owner, the backup owner and the acknowledgement deadline
// minutes (the Story 2.4 coverage inputs — re-read by the ownership
// commands; the deadline NEVER comes from a command parameter).
type CoverageWord struct {
	Channel            string
	Direction          string
	ResponseOwnerID    id.UUID
	BackupOwnerID      id.UUID
	AckDeadlineMinutes int64
}

// Route returns the route string of the coverage row.
func (w CoverageWord) Route() string { return w.Channel + ":" + w.Direction }

// RouteCandidate is one employee whose acting decision seals the
// connection: the per-route results and the human coverage of the exact
// profile version pinned by that decision. PackID/PackVersion carry the
// Domain Pack of the sealed configuration version (the taxonomy source of
// the case the ingress pins onto the case row).
type RouteCandidate struct {
	EmployeeID     id.UUID
	DecisionNo     int64
	Stage          string
	ProfileVersion int64
	PackID         string
	PackVersion    string
	Routes         []RouteAdmissionWord
	Coverage       []CoverageWord
}

// RouteFacts is the employee facade projection: the candidates whose
// sealed manifests contain the connection plus the relayed entitlement
// admission word. Zero, one or many candidates decide the ladder step.
type RouteFacts struct {
	Candidates  []RouteCandidate
	Entitlement EntitlementWord
}

// EntitlementWord is the relayed admission state of the tenant entitlement
// (hard limit → the bounded outcome of the ladder).
type EntitlementWord struct {
	Status string
	Reason string
}

// IngressFacts is everything the ladder knows about one new (non-replay)
// event: the verified connection, the route facts of the acting decisions,
// the active membership set, the load counters and the envelope sizes.
// Replay never reaches the ladder — a sender retry does not depend on the
// changed states.
type IngressFacts struct {
	Connection     ConnectionFacts
	Channel        string
	Direction      string
	Modality       string
	Route          RouteFacts
	ActiveOwnerIDs map[id.UUID]bool
	InflightCount  int
	EnvelopeBytes  int
	PayloadBytes   int
}

// OutcomeKind is the family of the ladder verdict.
type OutcomeKind string

const (
	// OutcomeAccept admits the event: one transaction creates or extends
	// the case material.
	OutcomeAccept OutcomeKind = "accept"
	// OutcomeReject is the honest fail-closed refusal (a rejection row).
	OutcomeReject OutcomeKind = "reject"
	// OutcomeBounded is the honest capacity refusal with a counter.
	OutcomeBounded OutcomeKind = "bounded"
	// OutcomeQuarantine holds the event for the human: the event row is
	// quarantined and an operational exception is raised; no case is
	// created and nothing is rewritten.
	OutcomeQuarantine OutcomeKind = "quarantine"
)

// Outcome is the ladder verdict with the initial owner of the accepted
// event, the counter of a bounded refusal and the Domain Pack pin of the
// admitting decision (the taxonomy the ingest writes onto the case).
type Outcome struct {
	Kind        OutcomeKind
	RefusalKind string
	Text        string
	Counter     int
	OwnerID     id.UUID
	EmployeeID  id.UUID
	PackID      string
	PackVersion string
}

// AdmittedRouteStates are the per-route result values of a decision.
const (
	RouteWordAdmitted = "admitted"
	RouteWordBlocked  = "blocked"
)

// DeriveIngressOutcome runs the admission ladder over the facts in the
// declared order: connection state → channel/connector dictionary →
// route resolution (0/1/many acting decisions, route-precise result,
// initial response owner from the pinned human coverage and the active
// membership set) → entitlement relay → load envelope. Every refusal names
// its kind and a safe text; the payload never appears.
func DeriveIngressOutcome(facts IngressFacts) Outcome {
	// Step: the connection must be live. A revoked connection refuses
	// before anything durable happens (the replay path already returned).
	if facts.Connection.Status == "revoked" {
		return Outcome{Kind: OutcomeReject, RefusalKind: KindRevokedConnection,
			Text: "Подключение отозвано: события этого источника не принимаются."}
	}
	// Step: the channel must be served by the connector type of the
	// connection — one admitted combination never admits the others.
	if required := RequiredConnectorType(facts.Channel); facts.Connection.ConnectorType != required {
		return Outcome{Kind: OutcomeReject, RefusalKind: KindChannelMismatch,
			Text: fmt.Sprintf("Канал %s обслуживается коннекторами типа %s; подключение — %s.", facts.Channel, required, facts.Connection.ConnectorType)}
	}
	// Step: the route resolution over the acting decisions that seal this
	// connection. Zero decisions — external traffic is not admitted; more
	// than one — the routing is ambiguous and the human decides.
	switch len(facts.Route.Candidates) {
	case 0:
		return Outcome{Kind: OutcomeReject, RefusalKind: KindNoActiveRelease,
			Text: "Внешний трафик не допущен: ни одно действующее решение не запечатывает это подключение."}
	case 1:
	default:
		return Outcome{Kind: OutcomeQuarantine, RefusalKind: KindAmbiguousEmployee,
			Text: fmt.Sprintf("Неоднозначная маршрутизация: %d сотрудников имеют действующие решения с этим подключением; событие карантинится, выбор за человеком.", len(facts.Route.Candidates))}
	}
	candidate := facts.Route.Candidates[0]
	// Step: the route-precise result — the event route must be admitted by
	// the acting decision (a decision never admits a route it did not
	// admit; the stage is the minimal boundary, per-route is exact).
	routeAdmitted := false
	for _, route := range candidate.Routes {
		if route.Channel == facts.Channel && route.Direction == facts.Direction {
			routeAdmitted = route.State == RouteWordAdmitted
			break
		}
	}
	if !routeAdmitted {
		return Outcome{Kind: OutcomeReject, RefusalKind: KindRouteNotAdmitted,
			Text: fmt.Sprintf("Маршрут %s:%s не допущен действующим решением №%d (стадия %s).", facts.Channel, facts.Direction, candidate.DecisionNo, candidate.Stage)}
	}
	// Step: the initial response owner — from the human coverage of the
	// exact profile version pinned by the decision, verified against the
	// active membership set. No admissible owner — the route is not
	// admitted for external traffic until a human unlocks it.
	var ownerID id.UUID
	ownerKnown := false
	for _, coverage := range candidate.Coverage {
		if coverage.Channel == facts.Channel && coverage.Direction == facts.Direction {
			ownerID, ownerKnown = coverage.ResponseOwnerID, true
			break
		}
	}
	if !ownerKnown || !facts.ActiveOwnerIDs[ownerID] {
		// The unlock hint stays honest: a missing coverage row points at
		// the employee whose pinned profile must gain one; an inactive
		// owner points at the owner identity that must be restored.
		hint := ownerID
		return Outcome{Kind: OutcomeReject, RefusalKind: KindNoResponseOwner,
			Text: noOwnerText(candidate.EmployeeID, hint)}
	}
	// Step: the entitlement relay — the hard limit bounds new conversations
	// honestly; existing commitments always continue.
	if facts.Route.Entitlement.Status == "blocked" {
		return Outcome{Kind: OutcomeBounded, RefusalKind: KindEntitlementBlocked,
			Text: "Новые обращения не принимаются: entitlement в hard limit (существующие обязательства продолжаются)."}
	}
	// Step: the load envelope — the fixed conservative dictionary bounds
	// the sizes and the in-flight counter with an honest bounded outcome.
	envelope := FixedLoadEnvelope
	if facts.EnvelopeBytes > envelope.MaxEnvelopeBytes {
		return Outcome{Kind: OutcomeBounded, RefusalKind: KindEnvelopeExceeded,
			Counter: facts.EnvelopeBytes,
			Text:    fmt.Sprintf("Envelope превышает границу словаря: %d байт из %d; событие не принято, молчаливой потери нет.", facts.EnvelopeBytes, envelope.MaxEnvelopeBytes)}
	}
	if facts.PayloadBytes > envelope.MaxPayloadBytes {
		return Outcome{Kind: OutcomeBounded, RefusalKind: KindEnvelopeExceeded,
			Counter: facts.PayloadBytes,
			Text:    fmt.Sprintf("Payload превышает границу словаря: %d байт из %d; событие не принято, молчаливой потери нет.", facts.PayloadBytes, envelope.MaxPayloadBytes)}
	}
	if facts.InflightCount >= envelope.MaxInflightPerConnection {
		return Outcome{Kind: OutcomeBounded, RefusalKind: KindEnvelopeExceeded,
			Counter: facts.InflightCount,
			Text:    fmt.Sprintf("In-flight предел подключения достигнут: %d событий в окне %s из %d; событие не принято, молчаливой потери нет.", facts.InflightCount, envelope.InflightWindow, envelope.MaxInflightPerConnection)}
	}
	return Outcome{Kind: OutcomeAccept, OwnerID: ownerID, EmployeeID: candidate.EmployeeID,
		PackID: candidate.PackID, PackVersion: candidate.PackVersion}
}

func noOwnerText(employeeID id.UUID, hint id.UUID) string {
	if hint.Valid() {
		return fmt.Sprintf("Маршрут не допущен до внешнего трафика: первоначальный владелец отсутствует или неактивен (сотрудник %s, разблокировать: %s).", employeeID, hint)
	}
	return fmt.Sprintf("Маршрут не допущен до внешнего трафика: в запечатленной версии профиля нет human coverage для маршрута (сотрудник %s).", employeeID)
}
