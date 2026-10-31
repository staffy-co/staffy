package httptransport

import "github.com/staffy-co/staffy/internal/employee/domain"

type knowledgeRow struct {
	Source          domain.KnowledgeSource
	EffectiveStatus string
	Versions        []domain.KnowledgeVersion
}
