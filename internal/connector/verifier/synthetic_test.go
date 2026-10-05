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
