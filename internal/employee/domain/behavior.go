package domain

import (
	"strings"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// Behavior is declarative draft data. It is never interpolated into a system prompt.
type Behavior struct {
	Role             string         `json:"role"`
	Purpose          string         `json:"purpose"`
	AIDisclosure     bool           `json:"ai_disclosure"`
	Tone             string         `json:"tone"`
	Modes            []string       `json:"modes"`
	Claims           []AllowedClaim `json:"allowed_claims"`
	ForbiddenPhrases []string       `json:"forbidden_phrases"`
	Confirmation     string         `json:"confirmation"`
	Handoff          string         `json:"handoff"`
	Examples         []string       `json:"examples"`
}

type AllowedClaim struct {
	Text             string    `json:"text"`
	Owner            string    `json:"owner"`
	Scope            string    `json:"scope"`
	Channels         []string  `json:"channels"`
	Modes            []string  `json:"modes"`
	EvidenceSourceID id.UUID   `json:"evidence_source_id"`
	EvidenceVersion  int64     `json:"evidence_version"`
	ReviewAt         time.Time `json:"review_at"`
	Status           string    `json:"status,omitempty"`
	BlockReason      string    `json:"block_reason,omitempty"`
}

type BehaviorVersion struct {
	TenantID      id.UUID   `json:"tenant_id"`
	EmployeeID    id.UUID   `json:"employee_id"`
	Version       int64     `json:"version"`
	DraftVersion  int64     `json:"draft_version"`
	PackID        string    `json:"pack_id"`
	PackVersion   string    `json:"pack_version"`
	Config        Behavior  `json:"config"`
	ActorID       id.UUID   `json:"actor_id"`
	ChangedAt     time.Time `json:"changed_at"`
	ChangeSummary string    `json:"change_summary"`
}

type BehaviorRuleError struct {
	Rule   string
	Reason string
}

func (e BehaviorRuleError) Error() string { return e.Rule + ": " + e.Reason }
func (e BehaviorRuleError) Unwrap() error { return ErrInvalid }

type BehaviorPreview struct {
	Mode          string         `json:"mode"`
	Claims        []AllowedClaim `json:"claims"`
	Blockers      []string       `json:"blockers"`
	Handoff       string         `json:"handoff"`
	SyntheticOnly bool           `json:"synthetic_only"`
	Publishable   bool           `json:"publishable"`
}

var behaviorModes = map[string]bool{"answer": true, "intake": true, "sales": true, "handoff": true}

// ValidateBehavior enforces platform rules before any persistence or preview.
func ValidateBehavior(b Behavior) error {
	if !length(b.Role, 1, 160) || !length(b.Purpose, 1, 500) {
		return BehaviorRuleError{"role/purpose", "Укажите роль и назначение."}
	}
	for _, value := range []string{b.Role, b.Purpose} {
		if unsafeBehaviorText(strings.ToLower(value)) {
			return BehaviorRuleError{"safety/free text", "Роль и назначение не могут задавать инструкции поверх старших правил."}
		}
	}
	if !b.AIDisclosure {
		return BehaviorRuleError{"AI disclosure", "Раскрытие ИИ обязательно."}
	}
	if b.Tone != "neutral" && b.Tone != "warm" && b.Tone != "formal" {
		return BehaviorRuleError{"tone", "Неизвестный тон запрещён."}
	}
	if len(b.Modes) == 0 || len(b.Modes) > 4 {
		return BehaviorRuleError{"mode", "Выберите допустимый режим."}
	}
	seen := map[string]bool{}
	for _, m := range b.Modes {
		if !behaviorModes[m] || seen[m] {
			return BehaviorRuleError{"mode", "Неизвестный или повторный режим запрещён."}
		}
		seen[m] = true
	}
	if b.Confirmation != "required" {
		return BehaviorRuleError{"Confirmation", "Подтверждение клиента нельзя ослабить."}
	}
	if b.Handoff != "required" {
		return BehaviorRuleError{"human_requested", "Запрос человека должен вести к передаче человеку."}
	}
	if len(b.Claims) > 20 || len(b.ForbiddenPhrases) > 20 || len(b.Examples) > 8 {
		return BehaviorRuleError{"limits", "Превышен предел элементов."}
	}
	for _, claim := range b.Claims {
		if !length(claim.Text, 1, 300) || !length(claim.Owner, 1, 160) || !length(claim.Scope, 1, 160) || len(claim.Channels) == 0 || len(claim.Channels) > 12 || len(claim.Modes) == 0 || len(claim.Modes) > 4 {
			return BehaviorRuleError{"claim", "Укажите текст, владельца, scope, каналы и режимы."}
		}
		claimChannels := map[string]bool{}
		for _, channel := range claim.Channels {
			if !allowedChannels[channel] || claimChannels[channel] {
				return BehaviorRuleError{"claim/channel", "Неизвестный или повторный канал запрещён."}
			}
			claimChannels[channel] = true
		}
		claimModes := map[string]bool{}
		for _, mode := range claim.Modes {
			if !seen[mode] || claimModes[mode] {
				return BehaviorRuleError{"claim/mode", "Claim не может расширять или повторять режимы."}
			}
			claimModes[mode] = true
		}
		lower := strings.ToLower(claim.Text)
		if unsafeBehaviorText(lower) {
			return BehaviorRuleError{"safety/claim", "Claim противоречит старшему правилу."}
		}
		for _, word := range []string{"цена", "стоимость", "наличи", "статус", "доступност", "выполн", "price", "available", "status"} {
			if strings.Contains(lower, word) {
				return BehaviorRuleError{"Owning System", "Цена, наличие и статус требуют проверенной системы-владельца."}
			}
		}
	}
	for _, s := range append(append([]string{}, b.ForbiddenPhrases...), b.Examples...) {
		if !length(s, 1, 300) {
			return BehaviorRuleError{"free text", "Текст должен быть коротким."}
		}
	}
	for _, example := range b.Examples {
		if unsafeBehaviorText(strings.ToLower(example)) {
			return BehaviorRuleError{"safety/sales", "Пример противоречит старшему правилу."}
		}
	}
	return nil
}

func unsafeBehaviorText(lower string) bool {
	for _, phrase := range []string{"игнорируй", "обойди", "скрой, что", "не говори, что", "не сообщай, что", "скрывай ии", "не раскрывай ии", "без согласия", "без подтверждения", "после отказа", "без человека", "не передавай человеку", "давить", "давление", "ignore previous", "system prompt", "tool_call", "<script"} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}
