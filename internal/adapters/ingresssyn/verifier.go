// Package ingresssyn is the synthetic source verifier of the test contour
// (the authdev/verifier-synthetic precedent): it verifies that the
// connection exists in the CLAIMED tenant (RLS hides a foreign connection
// as an honest unknown_connection), that its environment is test and that
// its connector is the synthetic telephony connector this contour can
// verify. Production sources stay fail-closed: no provider signature
// schemes exist (AG-4) — a connector without a verifier is refused before
// any durable record.
package ingresssyn

import (
	"context"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// VerifiableConnectorID is the only connector the test contour can verify.
const VerifiableConnectorID = "staffy-synthetic-telephony"

// Verifier implements the cases SourceVerifier port over the connector
// facts read. The lookup runs under the CLAIMED tenant scope: the RLS
// tenant policy hides a connection of another tenant, so a wrong claim is
// an honest unknown_connection (the tenant existence is never disclosed).
type Verifier struct {
	connections Connections
}

// Connections is the narrow connector facts port the verifier consumes
// (satisfied structurally by the connector service).
type Connections interface {
	ConnectionFactsByID(ctx context.Context, scope tenancy.Context, connectionID id.UUID) (connectorAPI.ConnectionFacts, error)
}

// New wires the verifier over the connector facts port.
func New(connections Connections) *Verifier { return &Verifier{connections: connections} }

// VerifySource checks the origin of one ingress delivery. The failure
// kinds are the ladder dictionary kinds; the snapshot of a verified
// connection is the trusted metadata the ingress scope is built from.
func (v *Verifier) VerifySource(ctx context.Context, tenantClaim id.UUID, connectionID id.UUID) (casesDomain.SourceVerification, error) {
	if !tenantClaim.Valid() || !connectionID.Valid() {
		return casesDomain.SourceVerification{
			Verified: false,
			Kind:     casesDomain.KindMalformed,
			Text:     "Требуется UUID пространства и подключения.",
		}, nil
	}
	scope, err := tenancy.New(connectionID, tenantClaim)
	if err != nil {
		return casesDomain.SourceVerification{}, err
	}
	facts, err := v.connections.ConnectionFactsByID(ctx, scope, connectionID)
	if err != nil {
		if err == connectorDomain.ErrNotFound {
			return casesDomain.SourceVerification{
				Verified: false,
				Kind:     casesDomain.KindUnknownConnection,
				Text:     "Подключение не найдено в заявленном пространстве.",
			}, nil
		}
		return casesDomain.SourceVerification{}, err
	}
	switch facts.Environment {
	case string(connectorDomain.EnvTest), string(connectorDomain.EnvSandbox):
		// Both non-production contours of the catalog (AG-4 bans production
		// credentials at creation — the fail-closed ban is structural).
	default:
		return casesDomain.SourceVerification{
			Verified: false,
			Kind:     casesDomain.KindProductionForbidden,
			Text:     "Продакшн-источники закрыты: схемы подписи провайдеров не существуют (AG-4), верификация невозможна.",
		}, nil
	}
	if facts.ConnectorID != VerifiableConnectorID {
		return casesDomain.SourceVerification{
			Verified: false,
			Kind:     casesDomain.KindProductionForbidden,
			Text:     "Коннектор без верификатора: только staffy-synthetic-telephony проверяем в тестовом контуре (AG-4).",
		}, nil
	}
	return casesDomain.SourceVerification{
		Verified: true,
		Snapshot: casesDomain.ConnectionSnapshot{
			TenantID:      tenantClaim,
			ConnectionID:  connectionID,
			ConnectorID:   facts.ConnectorID,
			ConnectorType: facts.ConnectorType,
			Environment:   facts.Environment,
			Status:        facts.Status,
		},
	}, nil
}
