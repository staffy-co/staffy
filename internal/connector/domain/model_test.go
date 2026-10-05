package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestEnvironmentProductionIsFailClosed(t *testing.T) {
	err := ValidateEnvironment(EnvProduct)
	if err == nil {
		t.Fatal("production must be blocked")
	}
	if !errors.Is(err, ErrProductionBlocked) {
		t.Fatalf("expected ErrProductionBlocked, got %v", err)
	}
	var blocked ProductionBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("expected ProductionBlockedError, got %T", err)
	}
	if !strings.Contains(blocked.Reason, "AG-4") || !strings.Contains(blocked.Reason, "Technical Lead") {
		t.Fatalf("gate evidence and owner missing from reason: %s", blocked.Reason)
	}
	if err = ValidateEnvironment(EnvSandbox); err != nil {
		t.Fatalf("sandbox rejected: %v", err)
	}
	if err = ValidateEnvironment(EnvTest); err != nil {
		t.Fatalf("test rejected: %v", err)
	}
	if err = ValidateEnvironment(Environment("prod")); err == nil {
		t.Fatal("unknown environment accepted")
	}
}

func TestEnvironmentAllowedNeverPassesProduction(t *testing.T) {
	def := ConnectorDefinition{ID: "x", Version: "1", AllowedEnvironments: []Environment{EnvSandbox, EnvTest, EnvProduct}, SupportStatus: StatusSupported, AdapterImplemented: true}
	for _, env := range []Environment{EnvProduct, EnvProduct} {
		if def.EnvironmentAllowed(env) {
			t.Fatal("definition policy must never allow production before AG-4")
		}
	}
	if !def.EnvironmentAllowed(EnvSandbox) {
		t.Fatal("sandbox should be allowed")
	}
}

func TestValidTransitionMatrix(t *testing.T) {
	cases := []struct {
		from, to string
		ok       bool
	}{
		{StatusDraft, StatusConnectedUnverified, true},
		{StatusDraft, StatusFailed, true},
		{StatusDraft, StatusRevoked, true},
		{StatusDraft, StatusDraft, false},
		{StatusConnectedUnverified, StatusDraft, true},
		{StatusConnectedUnverified, StatusFailed, true},
		{StatusConnectedUnverified, StatusRevoked, true},
		{StatusFailed, StatusConnectedUnverified, true},
		{StatusFailed, StatusDraft, true},
		{StatusFailed, StatusFailed, true},
		{StatusRevoked, StatusDraft, false},
		{StatusRevoked, StatusConnectedUnverified, false},
		{StatusRevoked, StatusRevoked, false},
	}
	for _, c := range cases {
		if got := ValidTransition(c.from, c.to); got != c.ok {
			t.Errorf("ValidTransition(%s -> %s) = %v, want %v", c.from, c.to, got, c.ok)
		}
	}
}

func TestValidateConfigRejectsUnknownAndInvalidFields(t *testing.T) {
	def := ConnectorDefinition{
		ID: "synthetic", Version: "1", SupportStatus: StatusSupported, AdapterImplemented: true,
		Fields: []FieldSchema{
			{Name: "endpoint", Label: "Endpoint", Type: FieldURL, Required: true},
			{Name: "api_key", Label: "API-ключ", Type: FieldText, Required: true, Secret: true},
		},
	}
	if err := def.ValidateConfig(map[string]string{"endpoint": "https://abc.sandbox.example"}); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if err := def.ValidateConfig(map[string]string{"endpoint": "https://abc.sandbox.example", "api_key": "sandbox-12345678"}); err == nil {
		t.Fatal("secret value must not be accepted in config")
	}
	if err := def.ValidateConfig(map[string]string{"endpoint": "https://abc.sandbox.example", "extra": "x"}); err == nil {
		t.Fatal("unknown field accepted")
	}
	if err := def.ValidateConfig(map[string]string{}); err == nil {
		t.Fatal("missing required field accepted")
	}
	if err := def.ValidateConfig(map[string]string{"endpoint": "notaurl"}); err == nil {
		t.Fatal("invalid url accepted")
	}
	var rule RuleError
	if err := def.ValidateConfig(map[string]string{"endpoint": "https://<x>.example"}); !errors.As(err, &rule) {
		t.Fatalf("forbidden characters accepted: %v", err)
	}
}

func TestValidateSecretsChecksPresenceAndHygiene(t *testing.T) {
	def := ConnectorDefinition{
		ID: "synthetic", Version: "1", SupportStatus: StatusSupported, AdapterImplemented: true,
		Fields: []FieldSchema{{Name: "api_key", Label: "API-ключ", Type: FieldText, Required: true, Secret: true}},
	}
	if err := def.ValidateSecrets(map[string]string{"api_key": "sandbox-12345678"}); err != nil {
		t.Fatalf("valid secret rejected: %v", err)
	}
	if err := def.ValidateSecrets(map[string]string{}); err == nil {
		t.Fatal("missing required secret accepted")
	}
	if err := def.ValidateSecrets(map[string]string{"api_key": "short"}); err == nil {
		t.Fatal("too-short secret accepted")
	}
	if err := def.ValidateSecrets(map[string]string{"api_key": strings.Repeat("a", 300)}); err == nil {
		t.Fatal("too-long secret accepted")
	}
	if err := def.ValidateSecrets(map[string]string{"unknown": "sandbox-12345678"}); err == nil {
		t.Fatal("undeclared secret field accepted")
	}
}

func TestConnectableRequiresImplementedAdapter(t *testing.T) {
	def := ConnectorDefinition{SupportStatus: StatusSupported, AdapterImplemented: false}
	if def.Connectable() {
		t.Fatal("definition without adapter must not be connectable")
	}
	def.AdapterImplemented = true
	if !def.Connectable() {
		t.Fatal("supported definition with adapter must be connectable")
	}
	def.SupportStatus = StatusCandidate
	if def.Connectable() {
		t.Fatal("candidate definition must not be connectable")
	}
}
