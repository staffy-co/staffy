package mcp

// The method handlers of the surface (Story 7.3). Step-01 carries the
// protocol carriers only: the three methods answer the honest
// method-not-found of the not-yet-wired dispatch — the machines (the
// discover answer, the gated list and the validated call path) land with
// step-02 of the story on this branch before any merge.

import (
	"context"

	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// serverDiscover answers the revision negotiation of the pinned
// protocol: the supported versions, the factual capabilities and the
// server identity (step-02).
func (s *Service) serverDiscover(ctx context.Context, envelope *Envelope) (Outcome, error) {
	response := methodNotFoundResponse(envelope.ID, envelope.Method)
	return Outcome{Status: httpStatusOK, Body: &response}, nil
}

// toolsList answers the gated tool dictionary of the tenant session
// (step-02).
func (s *Service) toolsList(ctx context.Context, envelope *Envelope, actor identityDomain.Principal, tenantID id.UUID) (Outcome, error) {
	response := methodNotFoundResponse(envelope.ID, envelope.Method)
	return Outcome{Status: httpStatusOK, Body: &response}, nil
}

// toolsCall runs the validated call path of one pinned contract
// (step-02).
func (s *Service) toolsCall(ctx context.Context, envelope *Envelope, actor identityDomain.Principal, tenantID id.UUID) (Outcome, error) {
	response := methodNotFoundResponse(envelope.ID, envelope.Method)
	return Outcome{Status: httpStatusOK, Body: &response}, nil
}
