package api

import (
	"context"

	"github.com/staffy-co/staffy/internal/employee/domain"
	identity "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
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

type KnowledgeIngestCommand struct {
	Actor                               identity.Principal
	TenantID, EmployeeID, CorrelationID id.UUID
	Source                              domain.KnowledgeSource
	URL                                 string
	FileName                            string
	File                                []byte
	Fixture                             bool
}

type KnowledgeIngestService interface {
	IngestKnowledge(context.Context, KnowledgeIngestCommand) (domain.KnowledgeSource, domain.KnowledgeVersion, error)
	ReingestSyntheticKnowledge(context.Context, KnowledgeFixtureCommand) (domain.KnowledgeSource, domain.KnowledgeVersion, bool, error)
}

type KnowledgeFixtureCommand struct {
	Actor                                         identity.Principal
	TenantID, EmployeeID, SourceID, CorrelationID id.UUID
	ExpectedVersion                               int64
	FixtureID                                     string
}
