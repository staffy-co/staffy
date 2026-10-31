package externaltoolsfacts

// The adapter tests (Story 7-4 step-02): the wiring-level pins — ONLY the
// connections of the class card enter the dictionary material, and the
// query.tools-list report rides every row (the toolsdependency precedent
// of 7-3: the pin lives in the composition, the consumer never knows it).

import (
	"context"
	"testing"

	connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
	connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

type fakeConnections struct {
	rows []connectorAPI.ConnectionFacts
}

func (f fakeConnections) ConnectionsFacts(context.Context, tenancy.Context) ([]connectorAPI.ConnectionFacts, error) {
	return f.rows, nil
}

func factsRow(connectorID, capabilityID, state string) connectorAPI.ConnectionFacts {
	report := connectorDomain.CapabilityReport{
		CapabilityDeclaration: connectorDomain.CapabilityDeclaration{ID: capabilityID},
		State:                 state,
	}
	return connectorAPI.ConnectionFacts{
		ConnectionID:    id.MustParse("01994f64-6c00-7000-8000-0000000000b1"),
		ConnectorID:     connectorID,
		ContractVersion: "1.0.0",
		Environment:     "sandbox",
		Status:          connectorDomain.StatusConnectedUnverified,
		Capabilities:    []connectorDomain.CapabilityReport{report},
	}
}

func TestSourcePinsTheClassCardAndTheToolsListRow(t *testing.T) {
	rows := []connectorAPI.ConnectionFacts{
		factsRow("staffy-synthetic-telephony", "query.trunk-status", connectorDomain.CapStateVerified),
		factsRow("staffy-external-mcp-server", "query.server-discover", connectorDomain.CapStateVerified),
		factsRow("staffy-external-mcp-server", "query.tools-list", connectorDomain.CapStateStale),
	}
	source := New(fakeConnections{rows: rows})
	inventories, err := source.InventoryFacts(context.Background(), tenancy.Context{})
	if err != nil {
		t.Fatal(err)
	}
	// The class pin: only the external-mcp connection enters the material.
	if len(inventories) != 1 {
		t.Fatalf("ожидалась ровно одна запись класса, получено %d", len(inventories))
	}
	// The capability pin: the row rides the query.tools-list report.
	row := inventories[0]
	if row.ConnectorID != "staffy-external-mcp-server" || row.Report.ID != "query.tools-list" {
		t.Fatalf("пин проводки: %+v", row)
	}
	if row.Report.State != connectorDomain.CapStateStale || row.Status != connectorDomain.StatusConnectedUnverified {
		t.Fatalf("факты строки: %+v", row)
	}
}

func TestSourceAnswersTheHonestEmptyArray(t *testing.T) {
	source := New(fakeConnections{rows: []connectorAPI.ConnectionFacts{factsRow("staffy-synthetic-telephony", "query.trunk-status", connectorDomain.CapStateVerified)}})
	inventories, err := source.InventoryFacts(context.Background(), tenancy.Context{})
	if err != nil {
		t.Fatal(err)
	}
	if inventories == nil || len(inventories) != 0 {
		t.Fatalf("чужой класс обязан давать честный пустой массив: %v", inventories)
	}
}
