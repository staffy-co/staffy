package app

import (
	"context"
	"github.com/artemiosu/staffy/internal/employee/api"
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/employee/packs"
	"github.com/artemiosu/staffy/internal/employee/port"
	identity "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"strings"
)

func (s *Service) behaviorRepository() port.BehaviorRepository {
	return s.repository.(port.BehaviorRepository)
}

func (s *Service) ReadBehavior(ctx context.Context, actor identity.Principal, tenantID, employeeID id.UUID) ([]domain.BehaviorVersion, error) {
	scope, err := s.scope(ctx, actor, tenantID)
	if err != nil {
		return nil, err
	}
	if _, err = s.repository.Read(ctx, scope, employeeID); err != nil {
		return nil, err
	}
	versions, err := s.behaviorRepository().ReadBehavior(ctx, scope, employeeID)
	if err != nil {
		return nil, err
	}
	for vi := range versions {
		for ci := range versions[vi].Config.Claims {
			claim := &versions[vi].Config.Claims[ci]
			claim.Status = "blocked"
			claim.BlockReason = "claim: проверяемая привязка утверждения к доказательству отсутствует"
		}
	}
	return versions, nil
}

func (s *Service) prepareBehavior(ctx context.Context, c api.BehaviorCommand) (domain.BehaviorPreview, error) {
	scope, err := s.scope(ctx, c.Actor, c.TenantID)
	if err != nil {
		return domain.BehaviorPreview{}, err
	}
	if !c.EmployeeID.Valid() || c.ExpectedVersion < 1 || !c.CorrelationID.Valid() {
		return domain.BehaviorPreview{}, domain.ErrInvalid
	}
	d, err := s.repository.Read(ctx, scope, c.EmployeeID)
	if err != nil {
		return domain.BehaviorPreview{}, err
	}
	if d.Version != c.ExpectedVersion {
		return domain.BehaviorPreview{}, domain.ErrConflict
	}
	if d.Pack == nil {
		return domain.BehaviorPreview{}, domain.BehaviorRuleError{Rule: "Domain Pack", Reason: "Сначала закрепите пакет."}
	}
	manifest, ok := packs.Get(d.Pack.ID, d.Pack.Version)
	if !ok || manifest.PolicyRef == "" || len(manifest.Policies) == 0 {
		return domain.BehaviorPreview{}, domain.BehaviorRuleError{Rule: "Domain Pack", Reason: "Политика закреплённого пакета неизвестна."}
	}
	for _, policy := range manifest.Policies {
		if policy != "не обещать наличие и цену без проверки" && policy != "не обещать доступность и выполнение без проверки" {
			return domain.BehaviorPreview{}, domain.BehaviorRuleError{Rule: "Domain Pack", Reason: "Неизвестное старшее правило пакета запрещает настройку."}
		}
	}
	versions, err := s.behaviorRepository().ReadBehavior(ctx, scope, c.EmployeeID)
	if err != nil {
		return domain.BehaviorPreview{}, err
	}
	if len(versions) > 0 && (versions[0].PackID != d.Pack.ID || versions[0].PackVersion != d.Pack.Version) && c.PackID == "" {
		return domain.BehaviorPreview{}, domain.BehaviorRuleError{Rule: "Domain Pack", Reason: "Пакет изменился; начните настройку поведения заново."}
	}
	if c.PackID != "" && (c.PackID != d.Pack.ID || c.PackVersion != d.Pack.Version) {
		return domain.BehaviorPreview{}, domain.BehaviorRuleError{Rule: "Domain Pack", Reason: "Настройка относится к другой версии пакета."}
	}
	if err = domain.ValidateBehavior(c.Config); err != nil {
		return domain.BehaviorPreview{}, err
	}
	result := domain.BehaviorPreview{Mode: "answer", Claims: make([]domain.AllowedClaim, len(c.Config.Claims)), SyntheticOnly: true, Publishable: false, Handoff: "blocked: человек и маршрут не подтверждены"}
	copy(result.Claims, c.Config.Claims)
	if len(c.Config.Modes) > 0 {
		result.Mode = c.Config.Modes[0]
	}
	result.Blockers = append(result.Blockers, "Публикация и реальные действия недоступны в черновике.", "human_requested: человек и маршрут не подтверждены")
	switch c.Scenario {
	case "", "general":
	case "refusal":
		result.Mode = "blocked"
		result.Blockers = append(result.Blockers, "cancellation/sales: после отказа убеждение прекращается")
	case "no_contact":
		result.Mode = "blocked"
		result.Blockers = append(result.Blockers, "privacy/no_contact: дальнейший контакт запрещён")
	case "safety":
		result.Mode = "blocked"
		result.Blockers = append(result.Blockers, "safety: передать человеку; продажи прекращены")
	case "human_requested":
		result.Mode = "blocked"
		result.Blockers = append(result.Blockers, "human_requested: без подтверждённого человека handoff заблокирован")
	case "business_action":
		result.Mode = "blocked"
		result.Blockers = append(result.Blockers, "Confirmation: черновик не разрешает Business Action")
	default:
		return domain.BehaviorPreview{}, domain.BehaviorRuleError{Rule: "scenario", Reason: "Неизвестный сценарий запрещён."}
	}
	for i := range result.Claims {
		claim := &result.Claims[i]
		claim.Status = "blocked"
		claim.BlockReason = "claim: проверяемая привязка утверждения к доказательству отсутствует"
		result.Blockers = append(result.Blockers, claim.BlockReason)
	}
	if result.Mode == "sales" || result.Mode == "handoff" || len(result.Claims) > 0 {
		result.Mode = "blocked"
	}
	return result, nil
}

func (s *Service) PreviewBehavior(ctx context.Context, c api.BehaviorCommand) (domain.BehaviorPreview, error) {
	return s.prepareBehavior(ctx, c)
}

func (s *Service) SaveBehavior(ctx context.Context, c api.BehaviorCommand) (domain.BehaviorVersion, error) {
	preview, err := s.prepareBehavior(ctx, c)
	if err != nil {
		return domain.BehaviorVersion{}, err
	}
	if !lengthSummary(c.ChangeSummary) {
		return domain.BehaviorVersion{}, domain.BehaviorRuleError{Rule: "change summary", Reason: "Кратко опишите изменение."}
	}
	for i := range c.Config.Claims {
		c.Config.Claims[i].Status = preview.Claims[i].Status
		c.Config.Claims[i].BlockReason = preview.Claims[i].BlockReason
	}
	scope, _ := s.scope(ctx, c.Actor, c.TenantID)
	now := s.clock.Now().UTC()
	eventID, err := s.ids.New(now)
	if err != nil {
		return domain.BehaviorVersion{}, err
	}
	auditID, err := s.ids.New(now)
	if err != nil {
		return domain.BehaviorVersion{}, err
	}
	return s.behaviorRepository().SaveBehavior(ctx, port.BehaviorSaveParams{Scope: scope, EmployeeID: c.EmployeeID, CorrelationID: c.CorrelationID, EventID: eventID, AuditID: auditID, ExpectedVersion: c.ExpectedVersion, Config: c.Config, Summary: strings.TrimSpace(c.ChangeSummary), Now: now})
}
func lengthSummary(s string) bool { n := len([]rune(strings.TrimSpace(s))); return n >= 1 && n <= 300 }
