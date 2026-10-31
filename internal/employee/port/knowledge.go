package port

import (
	"context"
	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"time"
)

type KnowledgeMutation struct {
	Scope                                                 tenancy.Context
	EmployeeID, SourceID, CorrelationID, AuditID, EventID id.UUID
	ExpectedVersion                                       int64
	Now                                                   time.Time
}
type KnowledgeRepository interface {
	CreateKnowledge(context.Context, KnowledgeMutation, domain.KnowledgeSource) (domain.KnowledgeSource, error)
	ListKnowledge(context.Context, tenancy.Context, id.UUID) ([]domain.KnowledgeSource, error)
	ReadKnowledge(context.Context, tenancy.Context, id.UUID, id.UUID) (domain.KnowledgeSource, []domain.KnowledgeVersion, error)
	ProcessKnowledge(context.Context, KnowledgeMutation, string) (domain.KnowledgeSource, domain.KnowledgeVersion, bool, error)
	ProcessSyntheticKnowledge(context.Context, KnowledgeMutation, string) (domain.KnowledgeSource, domain.KnowledgeVersion, bool, error)
	ConflictKnowledge(context.Context, KnowledgeMutation, string, string) (domain.KnowledgeSource, error)
	RetireKnowledge(context.Context, KnowledgeMutation) (domain.KnowledgeSource, error)
}
