package verifier

import (
        "context"
        "strings"
        "testing"

        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
)

func syntheticDefinition() connectorDomain.ConnectorDefinition {
        return connectorDomain.ConnectorDefinition{
                ID: "staffy-synthetic-telephony", Version: "1.0.0", Type: connectorDomain.TypeSIP,
                SupportStatus: connectorDomain.StatusSupported, AdapterImplemented: true,
                AllowedEnvironments: []connectorDomain.Environment{connectorDomain.EnvSandbox, connectorDomain.EnvTest},
                Fields: []connectorDomain.FieldSchema{
                        {Name: "endpoint", Label: "Endpoint", Type: connectorDomain.FieldURL, Required: true},
                        {Name: "api_key", Label: "API-ключ", Type: connectorDomain.FieldText, Required: true, Secret: true},
                },
        }
}

func TestSyntheticCheckVerifiedOnSandboxValues(t *testing.T) {
        v := Synthetic{}
        result := v.BasicCheck(context.Background(), syntheticDefinition(), connectorDomain.EnvSandbox,
                map[string]string{"endpoint": "https://demo.sandbox.example"},
                map[string]string{"api_key": "sandbox-demo-123"})
        if result.Authentication != connectorDomain.CheckVerified {
                t.Fatalf("authentication = %s (%s)", result.Authentication, result.AuthReason)
        }
        if result.Transport != connectorDomain.CheckVerified {
                t.Fatalf("transport = %s (%s)", result.Transport, result.TransportReason)
        }
}

func TestSyntheticCheckFailsOnNonSandboxSecret(t *testing.T) {
        v := Synthetic{}
        result := v.BasicCheck(context.Background(), syntheticDefinition(), connectorDomain.EnvSandbox,
                map[string]string{"endpoint": "https://demo.sandbox.example"},
                map[string]string{"api_key": "real-production-key"})
        if result.Authentication != connectorDomain.CheckFailed {
                t.Fatalf("authentication = %s", result.Authentication)
        }
        if !strings.Contains(result.AuthReason, "sandbox-") {
                t.Fatalf("reason must explain the sandbox format: %s", result.AuthReason)
        }
        if !strings.Contains(result.AuthReason, "Синтетическая") {
                t.Fatalf("reason must state the check is synthetic: %s", result.AuthReason)
        }
}

func TestSyntheticCheckFailsOnNonSandboxEndpoint(t *testing.T) {
        v := Synthetic{}
        result := v.BasicCheck(context.Background(), syntheticDefinition(), connectorDomain.EnvSandbox,
                map[string]string{"endpoint": "https://api.novofon.com"},
                map[string]string{"api_key": "sandbox-demo-123"})
        if result.Transport != connectorDomain.CheckFailed {
                t.Fatalf("transport = %s", result.Transport)
        }
        if !strings.Contains(result.TransportReason, "провайдер не опрашивается") {
                t.Fatalf("reason must be honest about no provider polling: %s", result.TransportReason)
        }
}

func TestSyntheticCheckTestEnvironmentUsesTestSuffix(t *testing.T) {
        v := Synthetic{}
        result := v.BasicCheck(context.Background(), syntheticDefinition(), connectorDomain.EnvTest,
                map[string]string{"endpoint": "https://demo.sandbox.example"},
                map[string]string{"api_key": "sandbox-demo-123"})
        if result.Transport != connectorDomain.CheckFailed {
                t.Fatalf("test environment must reject sandbox.example endpoint, got %s", result.Transport)
        }
        result = v.BasicCheck(context.Background(), syntheticDefinition(), connectorDomain.EnvTest,
                map[string]string{"endpoint": "https://demo.test.example"},
                map[string]string{"api_key": "sandbox-demo-123"})
        if result.Transport != connectorDomain.CheckVerified {
                t.Fatalf("test environment must accept test.example endpoint, got %s", result.Transport)
        }
}

func TestSyntheticCheckRejectsUnimplementedAdapter(t *testing.T) {
        v := Synthetic{}
        def := syntheticDefinition()
        def.AdapterImplemented = false
        result := v.BasicCheck(context.Background(), def, connectorDomain.EnvSandbox, map[string]string{}, map[string]string{})
        if result.Authentication != connectorDomain.CheckFailed || result.Transport != connectorDomain.CheckFailed {
                t.Fatalf("unimplemented adapter must fail both checks: %v", result)
        }
        if !strings.Contains(result.AuthReason, "Адаптер") {
                t.Fatalf("reason must mention the missing adapter: %s", result.AuthReason)
        }
}

func TestReasonsNeverEchoSecretValues(t *testing.T) {
        v := Synthetic{}
        secret := "sandbox-secret-value-xyz"
        result := v.BasicCheck(context.Background(), syntheticDefinition(), connectorDomain.EnvSandbox,
                map[string]string{"endpoint": "https://api.example.com"},
                map[string]string{"api_key": secret})
        combined := result.AuthReason + result.TransportReason
        if strings.Contains(combined, secret) {
                t.Fatalf("reason must never echo the secret value: %s", combined)
        }
}

func TestSyntheticCheckEndpointSuffixRequiresSeparatorDot(t *testing.T) {
        v := Synthetic{}
        def := syntheticDefinition()
        secret := map[string]string{"api_key": "sandbox-demo-123"}
        // Lookalike hosts that merely END with the suffix (no separating dot)
        // must not verify: the published rule is https://<имя>.sandbox.example.
        for _, host := range []string{"notsandbox.example", "mysandbox.example", "sandbox.example", "sandbox.example.com"} {
                result := v.BasicCheck(context.Background(), def, connectorDomain.EnvSandbox, map[string]string{"endpoint": "https://" + host}, secret)
                if result.Transport != connectorDomain.CheckFailed {
                        t.Fatalf("host %q must not verify transport, got %s", host, result.Transport)
                }
        }
        // The documented shape with the separating dot still verifies.
        result := v.BasicCheck(context.Background(), def, connectorDomain.EnvSandbox, map[string]string{"endpoint": "https://demo.sandbox.example"}, secret)
        if result.Transport != connectorDomain.CheckVerified {
                t.Fatalf("demo.sandbox.example must verify transport, got %s", result.Transport)
        }
}

func probeDeclaration() connectorDomain.CapabilityDeclaration {
	return connectorDomain.CapabilityDeclaration{
		ID: "query.trunk-status", Title: "Статус транка", Direction: connectorDomain.DirectionOutbound, Operation: connectorDomain.OperationQuery,
		TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60/мин", SizeLimit: "4 КБ", Receipt: "query_result",
		ReceiptLevel: connectorDomain.DeliveryDelivered, CancelCompensation: "не применимо", Reconciliation: "не требуется",
		ReconciliationOwner: "Technical Lead", ProbeAllowed: true,
	}
}

func probeDefinition() connectorDomain.ConnectorDefinition {
	return connectorDomain.ConnectorDefinition{
		ID: "staffy-synthetic-telephony", Version: "1.1.0", SupportStatus: connectorDomain.StatusSupported, AdapterImplemented: true,
		AllowedEnvironments: []connectorDomain.Environment{connectorDomain.EnvSandbox},
		Fields:              []connectorDomain.FieldSchema{{Name: "endpoint", Label: "Endpoint", Type: connectorDomain.FieldURL, Required: true}, {Name: "api_key", Label: "API-ключ", Type: connectorDomain.FieldText, Required: true, Secret: true}},
	}
}

func TestProbeVerifiesReadOnlyCapabilityAtDeliveredLevel(t *testing.T) {
	v := Synthetic{}
	result := v.Probe(context.Background(), probeDefinition(), probeDeclaration(), connectorDomain.EnvSandbox, map[string]string{"endpoint": "https://demo.sandbox.example", "api_key": "sandbox-secret-value"}, nil)
	if result.Outcome != connectorDomain.EvidenceVerified || result.DeliveryLevel != string(connectorDomain.DeliveryDelivered) {
		t.Fatalf("result=%+v", result)
	}
	// Redaction: only the host, never the path, the query or the secret.
	if strings.Contains(result.RedactedRequest, "sandbox-secret-value") || strings.Contains(result.RedactedRequest, "/api") {
		t.Fatalf("request must stay redacted: %s", result.RedactedRequest)
	}
	if strings.Contains(result.RedactedResult, "sandbox-") {
		t.Fatalf("result must stay redacted: %s", result.RedactedResult)
	}
	if !strings.Contains(result.RedactedRequest, AdapterBuild) {
		t.Fatalf("request must record the adapter build: %s", result.RedactedRequest)
	}
}

func TestProbeTimeoutHostYieldsHonestUnknown(t *testing.T) {
	v := Synthetic{}
	result := v.Probe(context.Background(), probeDefinition(), probeDeclaration(), connectorDomain.EnvSandbox, map[string]string{"endpoint": "https://timeout.demo.sandbox.example"}, nil)
	if result.Outcome != connectorDomain.EvidenceUnknown || result.DeliveryLevel != string(connectorDomain.DeliveryUnknown) {
		t.Fatalf("result=%+v", result)
	}
	if !strings.Contains(result.RedactedResult, "вслепую") || !strings.Contains(result.RedactedResult, "Technical Lead") {
		t.Fatalf("unknown must forbid blind retries and name the owner: %s", result.RedactedResult)
	}
}

func TestProbeExpiredContextYieldsUnknown(t *testing.T) {
	v := Synthetic{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := v.Probe(ctx, probeDefinition(), probeDeclaration(), connectorDomain.EnvSandbox, map[string]string{"endpoint": "https://demo.sandbox.example"}, nil)
	if result.Outcome != connectorDomain.EvidenceUnknown {
		t.Fatalf("expired deadline must yield unknown: %+v", result)
	}
}

func TestProbeFailHostYieldsFailed(t *testing.T) {
	v := Synthetic{}
	result := v.Probe(context.Background(), probeDefinition(), probeDeclaration(), connectorDomain.EnvSandbox, map[string]string{"endpoint": "https://fail.demo.sandbox.example"}, nil)
	if result.Outcome != connectorDomain.EvidenceFailed {
		t.Fatalf("fail host must yield failed: %+v", result)
	}
}

func TestProbeRefusesNonVerifiableCapabilities(t *testing.T) {
	v := Synthetic{}
	mutating := probeDeclaration()
	mutating.Operation = connectorDomain.OperationCommand
	result := v.Probe(context.Background(), probeDefinition(), mutating, connectorDomain.EnvSandbox, map[string]string{"endpoint": "https://demo.sandbox.example"}, nil)
	if result.Outcome != connectorDomain.EvidenceFailed || !strings.Contains(result.RedactedResult, "Action Gateway") {
		t.Fatalf("mutating capability probe must be refused: %+v", result)
	}
}

func TestAdapterBuildConstantPinsSyntheticVersion(t *testing.T) {
	if AdapterBuild != "synthetic/1.1.0" {
		t.Fatalf("adapter build = %s", AdapterBuild)
	}
	if v := (Synthetic{}); v.Build() != AdapterBuild {
		t.Fatal("Build() must report the pinned adapter build")
	}
}
