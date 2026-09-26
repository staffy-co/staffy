package api

import (
	"context"

	"github.com/artemiosu/staffy/internal/employee/domain"
	identity "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

type KnowledgeCreateCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	Source                              domain.KnowledgeSource
}
type KnowledgeProcessCommand struct {
	Actor                                         identity.Principal
	TenantID, EmployeeID, SourceID, CorrelationID id.UUID
	ExpectedVersion                               int64
	Text                                          string
}
type KnowledgeConflictCommand struct {
	Actor                                         identity.Principal
	TenantID, EmployeeID, SourceID, CorrelationID id.UUID
	ExpectedVersion                               int64
	Note, Provenance                              string
}
type KnowledgeRetireCommand struct {
	Actor                                         identity.Principal
	TenantID, EmployeeID, SourceID, CorrelationID id.UUID
	ExpectedVersion                               int64
}
type KnowledgeService interface {
	CreateKnowledge(context.Context, KnowledgeCreateCommand) (domain.KnowledgeSource, error)
	ListKnowledge(context.Context, identity.Principal, id.UUID, id.UUID) ([]domain.KnowledgeSource, error)
	ReadKnowledge(context.Context, identity.Principal, id.UUID, id.UUID, id.UUID) (domain.KnowledgeSource, []domain.KnowledgeVersion, error)
	ProcessKnowledge(context.Context, KnowledgeProcessCommand) (domain.KnowledgeSource, domain.KnowledgeVersion, bool, error)
	ConflictKnowledge(context.Context, KnowledgeConflictCommand) (domain.KnowledgeSource, error)
	RetireKnowledge(context.Context, KnowledgeRetireCommand) (domain.KnowledgeSource, error)
}
