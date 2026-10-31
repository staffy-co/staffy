package toolsdependency

import (
	"context"
	"errors"
	"testing"
	"time"

	connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
	connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// factsFake scripts the live connection facts of one tenant.
type factsFake struct {
	rows []connectorAPI.ConnectionFacts
	err  error
}

func (f factsFake) ConnectionsFacts(context.Context, tenancy.Context) ([]connectorAPI.ConnectionFacts, error) {
	return f.rows, f.err
}

func gatePrincipal(t *testing.T) domain.Principal {
	t.Helper()
	principal, err := id.Parse("01994f64-6c00-7000-8000-0000000000c1")
	if err != nil {
		t.Fatalf("parse principal: %v", err)
	}
	return domain.Principal{ID: principal}
}

func gateTenant(t *testing.T) id.UUID {
	t.Helper()
	tenant, err := id.Parse("01994f64-6c00-7000-8000-0000000000a1")
	if err != nil {
		t.Fatalf("parse tenant: %v", err)
	}
	return tenant
}

func connectionOf(t *testing.T, connectorType, status string, states ...string) connectorAPI.ConnectionFacts {
	t.Helper()
	connectionID, err := id.Parse("01994f64-6c00-7000-8000-0000000000d1")
	if err != nil {
		t.Fatalf("parse connection: %v", err)
	}
	capabilities := make([]connectorDomain.CapabilityReport, 0, len(states))
	for _, state := range states {
		capabilities = append(capabilities, connectorDomain.CapabilityReport{State: state})
	}
	return connectorAPI.ConnectionFacts{
		ConnectionID:  connectionID,
		ConnectorType: connectorType,
		Status:        status,
		Capabilities:  capabilities,
	}
}

// TestGateDerivesTheVocabularyWords: the dependency fact is the honest
// derivation of the live connections of the type — the verified word of a
// proven capability, the failed/stale words of a refuted proof (the
// fail-closed order: a refutation closes the type before an unproven
// sibling could be read as hopeful), and the unverified word when nothing
// is proven.
func TestGateDerivesTheVocabularyWords(t *testing.T) {
	actor, tenant := gatePrincipal(t), gateTenant(t)
	connected := string(connectorDomain.StatusConnectedUnverified)

	// The live proof: verified exists, nothing refuted.
	gate := New(factsFake{rows: []connectorAPI.ConnectionFacts{
		connectionOf(t, "catalog", connected, connectorDomain.CapStateUnverified),
		connectionOf(t, "catalog", connected, connectorDomain.CapStateVerified),
	}})
	live, word, err := gate.Live(context.Background(), actor, tenant, "catalog")
	if err != nil || !live || word != "verified" {
		t.Fatalf("живое подключение с verified обязанo отвечать verified, получено live=%v word=%q err=%v", live, word, err)
	}

	// The stale rotation: one stale capability closes the type.
	gate = New(factsFake{rows: []connectorAPI.ConnectionFacts{
		connectionOf(t, "catalog", connected, connectorDomain.CapStateVerified),
		connectionOf(t, "catalog", connected, connectorDomain.CapStateStale),
	}})
	live, word, err = gate.Live(context.Background(), actor, tenant, "catalog")
	if err != nil || live || word != "stale" {
		t.Fatalf("ротация (stale) обязана закрывать гейт словом stale, получено live=%v word=%q err=%v", live, word, err)
	}

	// The failed proof: the fail-closed word of a refuted probe.
	gate = New(factsFake{rows: []connectorAPI.ConnectionFacts{
		connectionOf(t, "catalog", connected, connectorDomain.CapStateFailed),
	}})
	live, word, err = gate.Live(context.Background(), actor, tenant, "catalog")
	if err != nil || live || word != "failed" {
		t.Fatalf("провал (failed) обязан закрывать гейт словом failed, получено live=%v word=%q err=%v", live, word, err)
	}

	// Nothing proven yet: the honest unverified word.
	gate = New(factsFake{rows: []connectorAPI.ConnectionFacts{
		connectionOf(t, "catalog", connected, connectorDomain.CapStateUnverified),
	}})
	live, word, err = gate.Live(context.Background(), actor, tenant, "catalog")
	if err != nil || live || word != "unverified" {
		t.Fatalf("непроверенное подключение обязано отвечать unverified, получено live=%v word=%q err=%v", live, word, err)
	}

	// No live connection of the type: a foreign type or a dead connection
	// proves nothing.
	gate = New(factsFake{rows: []connectorAPI.ConnectionFacts{
		connectionOf(t, "telephony", connected, connectorDomain.CapStateVerified),
		connectionOf(t, "catalog", string(connectorDomain.StatusRevoked), connectorDomain.CapStateVerified),
	}})
	live, word, err = gate.Live(context.Background(), actor, tenant, "catalog")
	if err != nil || live || word != "no_live_connection" {
		t.Fatalf("отсутствие живого подключения типа обязано закрывать гейт, получено live=%v word=%q err=%v", live, word, err)
	}
}

// TestGateNeverGuesses: an infrastructure failure of the facts read is
// the honest error — the gate never reads a live or a dead word out of a
// failed read (fail-closed, never fail-open).
func TestGateNeverGuesses(t *testing.T) {
	gate := New(factsFake{err: errors.New("facts unavailable")})
	live, _, err := gate.Live(context.Background(), gatePrincipal(t), gateTenant(t), "catalog")
	if err == nil {
		t.Fatal("сбой чтения фактов обязан проходить ошибкой, а не догадкой")
	}
	if live {
		t.Fatal("сбой чтения фактов не обязан выглядеть живой зависимостью")
	}
}

// Compile-time pins of the fakes and the clock-free adapter.
var (
	_ ConnectionsFactsSource = factsFake{}
	_ time.Duration          = time.Second
)
