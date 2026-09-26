package app

import (
	"context"
	"crypto/rand"
	"github.com/artemiosu/staffy/internal/employee/api"
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/employee/ingest"
	"github.com/artemiosu/staffy/internal/employee/port"
	identity "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"strings"
)

func (s *Service) knowledgeRepo() port.KnowledgeRepository {
	return s.repository.(port.KnowledgeRepository)
}
func (s *Service) knowledgeMutation(ctx context.Context, tenant, employee, source, correlation id.UUID, actor identity.Principal, expected int64) (port.KnowledgeMutation, error) {
	scope, err := s.scope(ctx, actor, tenant)
	if err != nil {
		return port.KnowledgeMutation{}, err
	}
	if !employee.Valid() || !correlation.Valid() || (expected > 0 && !source.Valid()) {
		return port.KnowledgeMutation{}, domain.ErrInvalid
	}
	now := s.clock.Now().UTC()
	audit, err := s.ids.New(now)
	if err != nil {
		return port.KnowledgeMutation{}, err
	}
	event, err := s.ids.New(now)
	if err != nil {
		return port.KnowledgeMutation{}, err
	}
	return port.KnowledgeMutation{Scope: scope, EmployeeID: employee, SourceID: source, CorrelationID: correlation, AuditID: audit, EventID: event, ExpectedVersion: expected, Now: now}, nil
}
func (s *Service) CreateKnowledge(ctx context.Context, c api.KnowledgeCreateCommand) (domain.KnowledgeSource, error) {
	p, err := s.knowledgeMutation(ctx, c.TenantID, c.EmployeeID, "", c.CorrelationID, c.Actor, 0)
	if err != nil {
		return domain.KnowledgeSource{}, err
	}
	if !c.Source.Valid() {
		return domain.KnowledgeSource{}, domain.ErrInvalid
	}
	if c.Source.Type == "website" || c.Source.Type == "document" {
		return domain.KnowledgeSource{}, domain.ErrInvalid
	}
	return s.createKnowledge(ctx, p, c.Source)
}
func (s *Service) createKnowledge(ctx context.Context, p port.KnowledgeMutation, sourceInput domain.KnowledgeSource) (domain.KnowledgeSource, error) {
	source, err := s.ids.New(p.Now)
	if err != nil {
		return domain.KnowledgeSource{}, err
	}
	p.SourceID = source
	sourceInput.TenantID = p.Scope.TenantID()
	sourceInput.EmployeeID = p.EmployeeID
	sourceInput.SourceID = source
	if sourceInput.Status == "" {
		sourceInput.Status = "processing"
	}
	sourceInput.ChecksumKey = make([]byte, 32)
	if _, err = rand.Read(sourceInput.ChecksumKey); err != nil {
		return domain.KnowledgeSource{}, err
	}
	return s.knowledgeRepo().CreateKnowledge(ctx, p, sourceInput)
}

func (s *Service) IngestKnowledge(ctx context.Context, c api.KnowledgeIngestCommand) (domain.KnowledgeSource, domain.KnowledgeVersion, error) {
	p, err := s.knowledgeMutation(ctx, c.TenantID, c.EmployeeID, "", c.CorrelationID, c.Actor, 0)
	if err != nil {
		return domain.KnowledgeSource{}, domain.KnowledgeVersion{}, err
	}
	if !c.Source.Valid() || (c.Source.Type != "website" && c.Source.Type != "document") {
		return domain.KnowledgeSource{}, domain.KnowledgeVersion{}, domain.ErrInvalid
	}
	if c.Fixture {
		if c.Source.Type != "document" {
			return domain.KnowledgeSource{}, domain.KnowledgeVersion{}, domain.ErrInvalid
		}
		if c.URL != "" || c.FileName != "" || len(c.File) != 0 {
			return domain.KnowledgeSource{}, domain.KnowledgeVersion{}, domain.ErrInvalid
		}
		c.Source.Origin = ingest.FixtureOrigin
		c.Source.Status = "processing"
	} else {
		c.Source.Status = "quarantined"
		c.Source.IngestReason = "Проверка безопасности недоступна; источник остаётся в карантине."
		if c.Source.Type == "website" {
			if c.FileName != "" || len(c.File) != 0 {
				return domain.KnowledgeSource{}, domain.KnowledgeVersion{}, domain.ErrInvalid
			}
			if _, err = ingest.ValidateURLSyntax(c.URL); err != nil {
				c.Source.Status = "failed"
				c.Source.IngestReason = "Адрес не прошёл проверку безопасности."
			} else {
				c.Source.Origin = c.URL
			}
		} else {
			if c.URL != "" {
				return domain.KnowledgeSource{}, domain.KnowledgeVersion{}, domain.ErrInvalid
			}
			if _, err = ingest.ValidateFile(c.FileName, c.File); err != nil {
				c.Source.Status = "failed"
				c.Source.IngestReason = "Файл не прошёл проверку типа или размера."
			} else {
				c.Source.Origin = c.FileName
				c.Source.Status = "failed"
				c.Source.IngestReason = "Файл не сохранён: проверка безопасности недоступна. Загрузите его повторно после подключения проверки."
			}
		}
	}
	source, err := s.createKnowledge(ctx, p, c.Source)
	return source, domain.KnowledgeVersion{}, err
}
func (s *Service) ReingestSyntheticKnowledge(ctx context.Context, c api.KnowledgeFixtureCommand) (domain.KnowledgeSource, domain.KnowledgeVersion, bool, error) {
	p, err := s.knowledgeMutation(ctx, c.TenantID, c.EmployeeID, c.SourceID, c.CorrelationID, c.Actor, c.ExpectedVersion)
	if err != nil {
		return domain.KnowledgeSource{}, domain.KnowledgeVersion{}, false, err
	}
	if c.ExpectedVersion < 1 {
		return domain.KnowledgeSource{}, domain.KnowledgeVersion{}, false, domain.ErrInvalid
	}
	text, err := ingest.SyntheticText(c.FixtureID)
	if err != nil {
		return domain.KnowledgeSource{}, domain.KnowledgeVersion{}, false, domain.ErrInvalid
	}
	return s.knowledgeRepo().ProcessSyntheticKnowledge(ctx, p, text)
}
func (s *Service) ListKnowledge(ctx context.Context, actor identity.Principal, tenant, employee id.UUID) ([]domain.KnowledgeSource, error) {
	scope, err := s.scope(ctx, actor, tenant)
	if err != nil {
		return nil, err
	}
	if !employee.Valid() {
		return nil, domain.ErrNotFound
	}
	return s.knowledgeRepo().ListKnowledge(ctx, scope, employee)
}
func (s *Service) ReadKnowledge(ctx context.Context, actor identity.Principal, tenant, employee, source id.UUID) (domain.KnowledgeSource, []domain.KnowledgeVersion, error) {
	scope, err := s.scope(ctx, actor, tenant)
	if err != nil {
		return domain.KnowledgeSource{}, nil, err
	}
	if !employee.Valid() || !source.Valid() {
		return domain.KnowledgeSource{}, nil, domain.ErrNotFound
	}
	return s.knowledgeRepo().ReadKnowledge(ctx, scope, employee, source)
}
func (s *Service) ProcessKnowledge(ctx context.Context, c api.KnowledgeProcessCommand) (domain.KnowledgeSource, domain.KnowledgeVersion, bool, error) {
	p, err := s.knowledgeMutation(ctx, c.TenantID, c.EmployeeID, c.SourceID, c.CorrelationID, c.Actor, c.ExpectedVersion)
	if err != nil {
		return domain.KnowledgeSource{}, domain.KnowledgeVersion{}, false, err
	}
	if c.ExpectedVersion < 1 || !domain.ValidKnowledgeText(c.Text) {
		return domain.KnowledgeSource{}, domain.KnowledgeVersion{}, false, domain.ErrInvalid
	}
	return s.knowledgeRepo().ProcessKnowledge(ctx, p, c.Text)
}
func (s *Service) ConflictKnowledge(ctx context.Context, c api.KnowledgeConflictCommand) (domain.KnowledgeSource, error) {
	p, err := s.knowledgeMutation(ctx, c.TenantID, c.EmployeeID, c.SourceID, c.CorrelationID, c.Actor, c.ExpectedVersion)
	if err != nil {
		return domain.KnowledgeSource{}, err
	}
	if c.ExpectedVersion < 1 || len([]rune(strings.TrimSpace(c.Note))) < 1 || len(c.Note) > 1000 || len(c.Provenance) > 500 || strings.TrimSpace(c.Provenance) == "" {
		return domain.KnowledgeSource{}, domain.ErrInvalid
	}
	return s.knowledgeRepo().ConflictKnowledge(ctx, p, c.Note, c.Provenance)
}
func (s *Service) RetireKnowledge(ctx context.Context, c api.KnowledgeRetireCommand) (domain.KnowledgeSource, error) {
	p, err := s.knowledgeMutation(ctx, c.TenantID, c.EmployeeID, c.SourceID, c.CorrelationID, c.Actor, c.ExpectedVersion)
	if err != nil {
		return domain.KnowledgeSource{}, err
	}
	if c.ExpectedVersion < 1 {
		return domain.KnowledgeSource{}, domain.ErrInvalid
	}
	return s.knowledgeRepo().RetireKnowledge(ctx, p)
}
