package app

import (
	"context"
	"github.com/staffy-co/staffy/internal/employee/api"
	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/employee/port"
	identity "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"strings"
)

func (s *Service) voiceRepository() port.VoiceRepository { return s.repository.(port.VoiceRepository) }

func (s *Service) VoiceCatalog(ctx context.Context, actor identity.Principal, tenantID id.UUID) ([]domain.VoiceProfile, error) {
	if _, err := s.scope(ctx, actor, tenantID); err != nil {
		return nil, err
	}
	profiles := domain.VoiceCatalog()
	if s.testTTS != nil {
		for i := range profiles {
			candidate := domain.VoiceConfig{ProfileID: profiles[i].ID, Language: profiles[i].Language, Style: "neutral", Tempo: 100}
			if s.testTTS.Capability(ctx, candidate) == nil {
				profiles[i].PreviewStatus = "available"
			}
		}
	}
	return profiles, nil
}

func (s *Service) ReadVoice(ctx context.Context, actor identity.Principal, tenantID, employeeID id.UUID) ([]domain.VoiceVersion, error) {
	scope, err := s.scope(ctx, actor, tenantID)
	if err != nil {
		return nil, err
	}
	if _, err = s.repository.Read(ctx, scope, employeeID); err != nil {
		return nil, err
	}
	return s.voiceRepository().ReadVoice(ctx, scope, employeeID)
}

func (s *Service) SaveVoice(ctx context.Context, c api.VoiceCommand) (domain.VoiceVersion, error) {
	scope, err := s.scope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return domain.VoiceVersion{}, err
	}
	if !c.EmployeeID.Valid() || !c.CorrelationID.Valid() || c.ExpectedVersion < 1 || !lengthSummary(c.ChangeSummary) {
		return domain.VoiceVersion{}, domain.ErrInvalid
	}
	d, err := s.repository.Read(ctx, scope, c.EmployeeID)
	if err != nil {
		return domain.VoiceVersion{}, err
	}
	if d.Version != c.ExpectedVersion {
		return domain.VoiceVersion{}, domain.ErrConflict
	}
	if err = domain.ValidateVoice(d.Brief, c.Config); err != nil {
		return domain.VoiceVersion{}, err
	}
	now := s.clock.Now().UTC()
	eventID, err := s.ids.New(now)
	if err != nil {
		return domain.VoiceVersion{}, err
	}
	auditID, err := s.ids.New(now)
	if err != nil {
		return domain.VoiceVersion{}, err
	}
	return s.voiceRepository().SaveVoice(ctx, port.VoiceSaveParams{Scope: scope, EmployeeID: c.EmployeeID, CorrelationID: c.CorrelationID, EventID: eventID, AuditID: auditID, ExpectedVersion: c.ExpectedVersion, Config: c.Config, Summary: strings.TrimSpace(c.ChangeSummary), Now: now})
}

func (s *Service) PreviewVoice(ctx context.Context, c api.VoicePreviewRequest) (api.VoicePreview, error) {
	if !c.SyntheticDataConfirmed {
		return api.VoicePreview{}, domain.VoiceRuleError{Rule: "synthetic data", Reason: "Подтвердите, что preview содержит только вымышленные тестовые данные."}
	}
	scope, err := s.scope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return api.VoicePreview{}, err
	}
	d, err := s.repository.Read(ctx, scope, c.EmployeeID)
	if err != nil {
		return api.VoicePreview{}, err
	}
	versions, err := s.voiceRepository().ReadVoice(ctx, scope, c.EmployeeID)
	if err != nil {
		return api.VoicePreview{}, err
	}
	if len(versions) == 0 || c.VoiceVersion != versions[0].Version {
		return api.VoicePreview{}, domain.ErrConflict
	}
	if !safePreviewText(c.Text) {
		return api.VoicePreview{}, domain.VoiceRuleError{Rule: "preview text", Reason: "Введите короткий синтетический пример без недопустимых символов."}
	}
	v := versions[0]
	if v.DraftVersion != d.Version {
		return api.VoicePreview{}, domain.ErrConflict
	}
	synthesisText, _, err := domain.ResolveSynthesisText(v.Config, c.Text)
	if err != nil {
		return api.VoicePreview{}, err
	}
	if len([]rune(synthesisText)) > 500 {
		return api.VoicePreview{}, domain.VoiceRuleError{Rule: "preview text", Reason: "После применения произношения тестовый текст превышает 500 символов."}
	}
	result := api.VoicePreview{Text: c.Text, SynthesisText: synthesisText, VoiceVersion: v.Version, DictionaryVersion: v.Version, DraftVersion: v.DraftVersion, Route: "offline-test-tts", Capability: "unavailable", Stale: false, PhoneQualityApproved: false}
	if s.testTTS == nil || s.testTTS.Capability(ctx, v.Config) != nil {
		return result, domain.VoiceRuleError{Rule: "preview capability", Reason: "Офлайн test-TTS недоступен; черновик сохранён. Проверка телефонного качества остаётся открытой."}
	}
	audio, mime, err := s.testTTS.Synthesize(ctx, v.Config, synthesisText, c.Slow)
	if err != nil {
		return result, domain.VoiceRuleError{Rule: "preview capability", Reason: "Офлайн test-TTS не создал аудио; черновик сохранён. Повторите попытку или обратитесь к человеку."}
	}
	result.Capability = "available"
	result.Audio = audio
	result.AudioType = mime
	previewID, err := s.ids.New(s.clock.Now().UTC())
	if err != nil {
		return api.VoicePreview{}, err
	}
	result.PreviewID = previewID
	receipt := domain.VoicePreviewRecord{TenantID: scope.TenantID(), EmployeeID: c.EmployeeID, PreviewID: previewID, VoiceVersion: v.Version, DictionaryVersion: v.Version, DraftVersion: v.DraftVersion, Text: c.Text, SynthesisText: synthesisText, Route: result.Route, Slow: c.Slow, ActorID: scope.PrincipalID(), CreatedAt: s.clock.Now().UTC()}
	if err = s.voiceRepository().SaveVoicePreview(ctx, scope, receipt); err != nil {
		return api.VoicePreview{}, err
	}
	return result, nil
}

func (s *Service) ReadVoiceAssessments(ctx context.Context, actor identity.Principal, tenantID, employeeID id.UUID) ([]domain.VoiceAssessment, error) {
	scope, err := s.scope(ctx, actor, tenantID)
	if err != nil {
		return nil, err
	}
	if _, err = s.repository.Read(ctx, scope, employeeID); err != nil {
		return nil, err
	}
	return s.voiceRepository().ReadVoiceAssessments(ctx, scope, employeeID)
}

func (s *Service) AssessVoice(ctx context.Context, c api.VoiceAssessmentCommand) (domain.VoiceAssessment, error) {
	scope, err := s.scope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return domain.VoiceAssessment{}, err
	}
	if !c.EmployeeID.Valid() || !c.PreviewID.Valid() || !c.CorrelationID.Valid() || c.VoiceVersion < 1 || (c.Verdict != "approved" && c.Verdict != "needs_revision") {
		return domain.VoiceAssessment{}, domain.ErrInvalid
	}
	now := s.clock.Now().UTC()
	assessmentID, err := s.ids.New(now)
	if err != nil {
		return domain.VoiceAssessment{}, err
	}
	eventID, err := s.ids.New(now)
	if err != nil {
		return domain.VoiceAssessment{}, err
	}
	auditID, err := s.ids.New(now)
	if err != nil {
		return domain.VoiceAssessment{}, err
	}
	return s.voiceRepository().SaveVoiceAssessment(ctx, port.VoiceAssessmentSaveParams{Scope: scope, EmployeeID: c.EmployeeID, PreviewID: c.PreviewID, AssessmentID: assessmentID, EventID: eventID, AuditID: auditID, CorrelationID: c.CorrelationID, VoiceVersion: c.VoiceVersion, Verdict: c.Verdict, Now: now})
}

func safePreviewText(s string) bool {
	if len([]rune(s)) < 1 || len([]rune(s)) > 300 || strings.ContainsAny(s, "<>\x00") {
		return false
	}
	return true
}
