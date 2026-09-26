package domain

import "testing"

func TestGrantLimits(t *testing.T) {
	tests := []struct {
		name   string
		roles  []Role
		scopes []Scope
		want   bool
	}{
		{"operator claims", []Role{Operator}, []Scope{Operations, Claims}, true},
		{"operator audit denied", []Role{Operator}, []Scope{Audit}, false},
		{"unknown role denied", []Role{"unknown"}, []Scope{Operations}, false},
		{"unknown scope denied", []Role{Owner}, []Scope{"unknown"}, false},
		{"owner incomplete denied", []Role{Owner}, []Scope{Configuration}, false},
		{"owner all", []Role{Owner}, AllScopes(), true},
		{"integration admin integrations", []Role{IntegrationAdmin}, []Scope{Integrations}, true},
		{"integration admin configuration denied", []Role{IntegrationAdmin}, []Scope{Configuration}, false},
		{"configurator integrations denied", []Role{Configurator}, []Scope{Integrations}, false},
		{"duplicate role denied", []Role{Operator, Operator}, []Scope{Operations}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateGrant(tt.roles, tt.scopes); got != tt.want {
				t.Fatalf("ValidateGrant() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReasonCodesAreBounded(t *testing.T) {
	if !ValidChangeReason(ReasonAccessAdjustment) || !ValidRevokeReason(ReasonOffboarding) {
		t.Fatal("expected reason code rejected")
	}
	if ValidChangeReason("person@example.test") || ValidRevokeReason("free text with PII") {
		t.Fatal("free text accepted")
	}
}
