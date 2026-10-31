package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

var deriveNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func completeQueryDeclaration() CapabilityDeclaration {
	return CapabilityDeclaration{
		ID: "query.trunk-status", Title: "Статус транка", Direction: DirectionOutbound, Operation: OperationQuery,
		TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60/мин", SizeLimit: "4 КБ", Idempotent: false,
		Receipt: "query_result", ReceiptLevel: DeliveryDelivered, CancelCompensation: "не применимо",
		Reconciliation: "не требуется", ReconciliationOwner: "Technical Lead", ProbeAllowed: true,
	}
}

func deriveConnection() Connection {
	return Connection{
		TenantID: "t", ConnectionID: "c1", ConnectorID: "staffy-synthetic-telephony", ConnectorVer: "1.1.0",
		Environment: EnvSandbox, Status: StatusConnectedUnverified, SecretID: "s1", SecretActive: true,
		UpdatedAt: deriveNow.Add(-time.Hour),
	}
}

func applicableEvidence() CapabilityEvidence {
	return CapabilityEvidence{
		TenantID: "t", EvidenceID: "e1", ConnectionID: "c1", SecretID: "s1", Environment: EnvSandbox,
		ContractVersion: "1.1.0", AdapterBuild: "synthetic/1.1.0", Capability: "query.trunk-status",
		ProbeKind: EvidenceProbeKind, RequestedAt: deriveNow.Add(-30 * time.Minute), CompletedAt: deriveNow.Add(-30 * time.Minute),
		Outcome: EvidenceVerified, DeliveryLevel: "delivered", RedactedRequest: "r", RedactedResult: "r",
		ExpiresAt: deriveNow.Add(23 * time.Hour),
	}
}

func find(reports []CapabilityReport, capability string) CapabilityReport {
	for _, r := range reports {
		if r.ID == capability {
			return r
		}
	}
	return CapabilityReport{}
}

func TestDeriveUnverifiedWithoutEvidence(t *testing.T) {
	def := ConnectorDefinition{Capabilities: []CapabilityDeclaration{completeQueryDeclaration()}}
	reports := DeriveCapabilityStates(def, deriveConnection(), nil, "synthetic/1.1.0", deriveNow)
	report := find(reports, "query.trunk-status")
	if report.State != CapStateUnverified || !report.CanVerify || report.Evidence != nil {
		t.Fatalf("no evidence must derive unverified: %+v", report)
	}
}

func TestDeriveVerifiedFromApplicableEvidence(t *testing.T) {
	def := ConnectorDefinition{Capabilities: []CapabilityDeclaration{completeQueryDeclaration()}}
	reports := DeriveCapabilityStates(def, deriveConnection(), []CapabilityEvidence{applicableEvidence()}, "synthetic/1.1.0", deriveNow)
	report := find(reports, "query.trunk-status")
	if report.State != CapStateVerified || report.Evidence == nil || !report.CanVerify {
		t.Fatalf("applicable evidence must derive verified: %+v", report)
	}
	if report.CheckedAt == nil || report.ExpiresAt == nil {
		t.Fatal("checked/expires times must be disclosed")
	}
}

func TestDeriveStaleReasonsNameExactFacts(t *testing.T) {
	base := applicableEvidence()
	cases := []struct {
		name   string
		mutate func(*Connection, *CapabilityEvidence, *string)
		want   string
	}{
		{"secret rotated", func(c *Connection, e *CapabilityEvidence, b *string) { e.SecretID = "s2" }, "ротирован"},
		{"secret revoked", func(c *Connection, e *CapabilityEvidence, b *string) { c.SecretActive = false }, "ротирован или отозван"},
		{"config changed after evidence", func(c *Connection, e *CapabilityEvidence, b *string) { c.UpdatedAt = deriveNow }, "изменились"},
		{"contract version changed", func(c *Connection, e *CapabilityEvidence, b *string) { c.ConnectorVer = "1.2.0" }, "версия контракта"},
		{"adapter build changed", func(c *Connection, e *CapabilityEvidence, b *string) { *b = "synthetic/1.2.0" }, "сборка адаптера"},
		{"ttl expired", func(c *Connection, e *CapabilityEvidence, b *string) { e.ExpiresAt = deriveNow.Add(-time.Minute) }, "истёк"},
	}
	def := ConnectorDefinition{Capabilities: []CapabilityDeclaration{completeQueryDeclaration()}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := deriveConnection()
			evidence := base
			build := "synthetic/1.1.0"
			tc.mutate(&conn, &evidence, &build)
			reports := DeriveCapabilityStates(def, conn, []CapabilityEvidence{evidence}, build, deriveNow)
			report := find(reports, "query.trunk-status")
			if report.State != CapStateStale {
				t.Fatalf("state = %s, want stale", report.State)
			}
			if !strings.Contains(report.StateReason, tc.want) {
				t.Fatalf("reason must name %q: %s", tc.want, report.StateReason)
			}
		})
	}
}

func TestDeriveForeignEvidenceNeverApplies(t *testing.T) {
	def := ConnectorDefinition{Capabilities: []CapabilityDeclaration{completeQueryDeclaration()}}
	foreign := applicableEvidence()
	foreign.ConnectionID = "other-connection"
	foreign.SecretID = "other-secret"
	foreign.Environment = EnvTest
	reports := DeriveCapabilityStates(def, deriveConnection(), []CapabilityEvidence{foreign}, "synthetic/1.1.0", deriveNow)
	if report := find(reports, "query.trunk-status"); report.State != CapStateStale {
		t.Fatalf("foreign evidence must be stale, got %s", report.State)
	}
}

func TestDeriveGuardedWhenConnectionNotConnected(t *testing.T) {
	def := ConnectorDefinition{Capabilities: []CapabilityDeclaration{completeQueryDeclaration()}}
	for _, status := range []string{StatusDraft, StatusFailed, StatusRevoked} {
		conn := deriveConnection()
		conn.Status = status
		reports := DeriveCapabilityStates(def, conn, []CapabilityEvidence{applicableEvidence()}, "synthetic/1.1.0", deriveNow)
		report := find(reports, "query.trunk-status")
		if report.State != CapStateUnverified || report.Evidence != nil {
			t.Fatalf("status %s must guard capabilities: %+v", status, report)
		}
		if report.CanVerify {
			t.Fatalf("status %s must not offer verification", status)
		}
	}
}

func TestDeriveIncompleteDeclarationIsUnsupported(t *testing.T) {
	incomplete := completeQueryDeclaration()
	incomplete.TimeoutMS = 0 // mandatory semantics missing
	def := ConnectorDefinition{Capabilities: []CapabilityDeclaration{incomplete}}
	reports := DeriveCapabilityStates(def, deriveConnection(), []CapabilityEvidence{applicableEvidence()}, "synthetic/1.1.0", deriveNow)
	report := find(reports, "query.trunk-status")
	if report.State != CapStateUnsupported || report.CanVerify {
		t.Fatalf("missing semantics must derive unsupported: %+v", report)
	}
	if !strings.Contains(report.StateReason, "unsupported") {
		t.Fatalf("reason must say unsupported: %s", report.StateReason)
	}
}

func TestDeriveMutatingCapabilityCarriesManagedBlock(t *testing.T) {
	mutating := completeQueryDeclaration()
	mutating.ID = "command.send-sms"
	mutating.Operation = OperationCommand
	mutating.ProbeAllowed = false
	def := ConnectorDefinition{Capabilities: []CapabilityDeclaration{mutating}}
	reports := DeriveCapabilityStates(def, deriveConnection(), nil, "synthetic/1.1.0", deriveNow)
	report := find(reports, "command.send-sms")
	if report.State != CapStateUnverified || report.Block == nil {
		t.Fatalf("mutating capability must carry the block: %+v", report)
	}
	if report.Block.Required != "Action Gateway (Epic 3)" || report.Block.Gate != "AG-4" || report.Block.Owner != "Technical Lead + Security" {
		t.Fatalf("block must name the requirement, gate and owner: %+v", report.Block)
	}
	if !strings.Contains(report.StateReason, "Action Gateway") {
		t.Fatalf("reason must explain the block: %s", report.StateReason)
	}
	// The block does not stop an independently proven read-only capability.
	query := completeQueryDeclaration()
	def.Capabilities = append(def.Capabilities, query)
	reports = DeriveCapabilityStates(def, deriveConnection(), nil, "synthetic/1.1.0", deriveNow)
	if r := find(reports, "query.trunk-status"); !r.CanVerify {
		t.Fatal("the block on send-sms must not block an independent read-only capability")
	}
}

func TestDeriveInboundEventStaysHonestUnverified(t *testing.T) {
	event := completeQueryDeclaration()
	event.ID = "event.inbound-call"
	event.Operation = OperationEvent
	event.Direction = DirectionInbound
	event.ProbeAllowed = false
	def := ConnectorDefinition{Capabilities: []CapabilityDeclaration{event}}
	reports := DeriveCapabilityStates(def, deriveConnection(), nil, "synthetic/1.1.0", deriveNow)
	report := find(reports, "event.inbound-call")
	if report.State != CapStateUnverified || report.CanVerify || report.Block != nil {
		t.Fatalf("inbound event must stay honestly unverified: %+v", report)
	}
	if !strings.Contains(report.StateReason, "не реализован") {
		t.Fatalf("reason must state the honest gap: %s", report.StateReason)
	}
}

func TestDeriveUnknownOutcomeNamesReconciliationOwner(t *testing.T) {
	evidence := applicableEvidence()
	evidence.Outcome = EvidenceUnknown
	evidence.DeliveryLevel = "unknown"
	def := ConnectorDefinition{Capabilities: []CapabilityDeclaration{completeQueryDeclaration()}}
	reports := DeriveCapabilityStates(def, deriveConnection(), []CapabilityEvidence{evidence}, "synthetic/1.1.0", deriveNow)
	report := find(reports, "query.trunk-status")
	if report.State != CapStateUnknown {
		t.Fatalf("state = %s, want unknown", report.State)
	}
	if !strings.Contains(report.StateReason, "Technical Lead") || !strings.Contains(report.StateReason, "вслепую") {
		t.Fatalf("unknown must name the owner and forbid blind retries: %s", report.StateReason)
	}
}

func TestDerivePicksLatestEvidencePerCapability(t *testing.T) {
	older := applicableEvidence()
	older.Outcome = EvidenceFailed
	older.CompletedAt = deriveNow.Add(-2 * time.Hour)
	older.EvidenceID = id.UUID("older")
	newer := applicableEvidence()
	newer.EvidenceID = id.UUID("newer")
	def := ConnectorDefinition{Capabilities: []CapabilityDeclaration{completeQueryDeclaration()}}
	reports := DeriveCapabilityStates(def, deriveConnection(), []CapabilityEvidence{older, newer}, "synthetic/1.1.0", deriveNow)
	if report := find(reports, "query.trunk-status"); report.State != CapStateVerified {
		t.Fatalf("latest evidence must win: %+v", report)
	}
}

func TestEvidenceSummaryNeverCarriesInternalFields(t *testing.T) {
	evidence := applicableEvidence()
	evidence.IdempotencyKey = "secretish-key"
	evidence.RequestHash = "hash-value"
	summary := evidence.Summary()
	if summary.RedactedRequest == "" || summary.Outcome == "" {
		t.Fatal("summary must carry the published facts")
	}
	// The struct has no fields for key/hash: compilation-level guarantee plus
	// a sanity check that the summary stays fully publishable.
	if summary.EvidenceID != evidence.EvidenceID {
		t.Fatal("summary must identify the evidence row")
	}
}
