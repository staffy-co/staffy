package domain

import (
        "errors"
        "time"

        "github.com/artemiosu/staffy/internal/platform/id"
)

var (
        ErrForbidden           = errors.New("principal is not allowed to create a tenant")
        ErrInvalidName         = errors.New("tenant name is invalid")
        ErrInvalidIdempotency  = errors.New("idempotency key is invalid")
        ErrIdempotencyConflict = errors.New("idempotency key conflicts with another request")
        ErrNotFound            = errors.New("resource not found")
        ErrInvalidMembership   = errors.New("membership is invalid")
        ErrConflict            = errors.New("membership version conflict")
        ErrStepUpRequired      = errors.New("step-up confirmation required")
        ErrLastOwner           = errors.New("last owner cannot be removed")
)

type Principal struct {
        ID              id.UUID
        CanCreateTenant bool
        VerifiedEmail   string
        StepUpVerified  bool
}

type Role string
type Scope string

const (
        Owner              Role  = "owner"
        Configurator       Role  = "configurator"
        Operator           Role  = "operator"
        ExpertQA           Role  = "expert_qa"
        IntegrationAdmin   Role  = "integration_admin"
        Configuration      Scope = "configuration"
        Integrations       Scope = "integrations"
        Operations         Scope = "operations"
        Review             Scope = "review"
        TechnicalExpertise Scope = "technical_expertise"
        Claims             Scope = "claims"
        Audit              Scope = "audit"
        DataGovernance     Scope = "data_governance"
)

var allScopes = []Scope{Configuration, Integrations, Operations, Review, TechnicalExpertise, Claims, Audit, DataGovernance}

func AllowedScopes(roles []Role) (map[Scope]bool, bool) {
        allowed := map[Scope]bool{}
        if len(roles) == 0 {
                return nil, false
        }
        for _, role := range roles {
                switch role {
                case Owner:
                        for _, scope := range allScopes {
                                allowed[scope] = true
                        }
                case Configurator:
                        // data_governance is assignable to a Configurator, but
                        // only explicitly: a membership lists every scope it
                        // grants, so the scope is never implicit.
                        allowed[Configuration] = true
                        allowed[DataGovernance] = true
                case IntegrationAdmin:
                        allowed[Integrations] = true
                case Operator:
                        allowed[Operations], allowed[Claims] = true, true
                case ExpertQA:
                        allowed[Review], allowed[TechnicalExpertise], allowed[Audit] = true, true, true
                default:
                        return nil, false
                }
        }
        return allowed, true
}

func ValidateGrant(roles []Role, scopes []Scope) bool {
        allowed, ok := AllowedScopes(roles)
        if !ok {
                return false
        }
        seen := map[Role]bool{}
        for _, role := range roles {
                if seen[role] {
                        return false
                }
                seen[role] = true
        }
        if seen[Owner] {
                return len(roles) == 1 && len(scopes) == len(allScopes) && containsAllScopes(scopes)
        }
        if len(scopes) == 0 {
                return false
        }
        selected := map[Scope]bool{}
        for _, scope := range scopes {
                if !allowed[scope] || selected[scope] {
                        return false
                }
                selected[scope] = true
        }
        return true
}

func containsAllScopes(scopes []Scope) bool {
        seen := map[Scope]bool{}
        for _, scope := range scopes {
                seen[scope] = true
        }
        for _, scope := range allScopes {
                if !seen[scope] {
                        return false
                }
        }
        return true
}

func AllScopes() []Scope { return append([]Scope(nil), allScopes...) }

type ReasonCode string

const (
        ReasonInvitationCreated  ReasonCode = "invitation_created"
        ReasonInvitationAccepted ReasonCode = "invitation_accepted"
        ReasonAccessAdjustment   ReasonCode = "access_adjustment"
        ReasonOffboarding        ReasonCode = "offboarding"
        ReasonSecurityResponse   ReasonCode = "security_response"
)

func ValidChangeReason(reason ReasonCode) bool {
        return reason == ReasonAccessAdjustment || reason == ReasonSecurityResponse
}

func ValidRevokeReason(reason ReasonCode) bool {
        return reason == ReasonOffboarding || reason == ReasonSecurityResponse
}

type Membership struct {
        TenantID    id.UUID    `json:"tenant_id"`
        PrincipalID *id.UUID   `json:"principal_id,omitempty"`
        Email       string     `json:"email"`
        Roles       []Role     `json:"roles"`
        Scopes      []Scope    `json:"scopes"`
        Status      string     `json:"status"`
        Version     int64      `json:"version"`
        ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type Tenant struct {
        ID        id.UUID
        Name      string
        CreatedAt time.Time
}

type Workspace struct {
        Tenant Tenant
        Role   string
}
