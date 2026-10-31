package ingresssyn

import (
	"context"
	"testing"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
	connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

type fakeConnections struct {
	facts connectorAPI.ConnectionFacts
	err   error
}

func (f fakeConnections) ConnectionFactsByID(context.Context, tenancy.Context, id.UUID) (connectorAPI.ConnectionFacts, error) {
	return f.facts, f.err
}

func testClaim() id.UUID      { return id.MustParse("01994f64-6c00-7000-8000-000000000011") }
func testConnection() id.UUID { return id.MustParse("01994f64-6c00-7000-8000-0000000000cc") }

func TestVerifierAcceptsSyntheticTelephonyInTestContours(t *testing.T) {
	for _, environment := range []connectorDomain.Environment{connectorDomain.EnvTest, connectorDomain.EnvSandbox} {
		verifier := New(fakeConnections{facts: connectorAPI.ConnectionFacts{
			ConnectionID: testConnection(), ConnectorID: VerifiableConnectorID,
			ConnectorType: "SIP/telephony", Environment: string(environment), Status: connectorDomain.StatusConnectedUnverified,
		}})
		verification, err := verifier.VerifySource(context.Background(), testClaim(), testConnection())
		if err != nil {
			t.Fatal(err)
		}
		if !verification.Verified {
			t.Fatalf("expected the synthetic telephony of the %s contour to verify, got %s", environment, verification.Kind)
		}
		if verification.Snapshot.TenantID != testClaim() || verification.Snapshot.ConnectorType != "SIP/telephony" {
			t.Fatalf("expected the trusted snapshot, got %+v", verification.Snapshot)
		}
	}
}

func TestVerifierRefusesUnknownConnectionHonestly(t *testing.T) {
	verifier := New(fakeConnections{err: connectorDomain.ErrNotFound})
	verification, err := verifier.VerifySource(context.Background(), testClaim(), testConnection())
	if err != nil {
		t.Fatal(err)
	}
	if verification.Verified || verification.Kind != casesDomain.KindUnknownConnection {
		t.Fatalf("expected the honest unknown_connection, got %+v", verification)
	}
}

func TestVerifierRefusesProductionEnvironmentFailClosed(t *testing.T) {
	verifier := New(fakeConnections{facts: connectorAPI.ConnectionFacts{
		ConnectionID: testConnection(), ConnectorID: VerifiableConnectorID,
		ConnectorType: "SIP/telephony", Environment: string(connectorDomain.EnvProduct), Status: connectorDomain.StatusConnectedUnverified,
	}})
	verification, err := verifier.VerifySource(context.Background(), testClaim(), testConnection())
	if err != nil {
		t.Fatal(err)
	}
	if verification.Verified || verification.Kind != casesDomain.KindProductionForbidden {
		t.Fatalf("expected the fail-closed production_forbidden, got %+v", verification)
	}
}

func TestVerifierRefusesConnectorWithoutVerifier(t *testing.T) {
	verifier := New(fakeConnections{facts: connectorAPI.ConnectionFacts{
		ConnectionID: testConnection(), ConnectorID: "jivo-chat",
		ConnectorType: "chat/inbox", Environment: string(connectorDomain.EnvSandbox), Status: connectorDomain.StatusConnectedUnverified,
	}})
	verification, err := verifier.VerifySource(context.Background(), testClaim(), testConnection())
	if err != nil {
		t.Fatal(err)
	}
	if verification.Verified || verification.Kind != casesDomain.KindProductionForbidden {
		t.Fatalf("expected the fail-closed refusal for a connector without a verifier, got %+v", verification)
	}
}

func TestVerifierRefusesMalformedClaims(t *testing.T) {
	verifier := New(fakeConnections{})
	verification, err := verifier.VerifySource(context.Background(), id.UUID(""), testConnection())
	if err != nil {
		t.Fatal(err)
	}
	if verification.Verified || verification.Kind != casesDomain.KindMalformed {
		t.Fatalf("expected the malformed refusal, got %+v", verification)
	}
}
