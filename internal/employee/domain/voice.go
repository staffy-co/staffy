package domain

import (
	"strings"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// VoiceProfile is a provider-neutral, reviewed catalog entry.
type VoiceProfile struct {
	ID            string   `json:"id"`
	Language      string   `json:"language"`
	Styles        []string `json:"styles"`
	MinTempo      int      `json:"min_tempo"`
	MaxTempo      int      `json:"max_tempo"`
	PreviewStatus string   `json:"preview_status"`
	Restrictions  []string `json:"restrictions"`
}

var voiceCatalog = []VoiceProfile{
	{ID: "ru-neutral-1", Language: "ru", Styles: []string{"neutral"}, MinTempo: 80, MaxTempo: 120, PreviewStatus: "unavailable", Restrictions: []string{"synthetic_only", "development_only"}},
	{ID: "en-neutral-1", Language: "en", Styles: []string{"neutral"}, MinTempo: 80, MaxTempo: 120, PreviewStatus: "unavailable", Restrictions: []string{"synthetic_only", "development_only"}},
}

func VoiceCatalog() []VoiceProfile {
	out := make([]VoiceProfile, len(voiceCatalog))
	for i, p := range voiceCatalog {
		out[i] = p
		out[i].Styles = append([]string(nil), p.Styles...)
		out[i].Restrictions = append([]string(nil), p.Restrictions...)
	}
	return out
}

type Pronunciation struct {
	Written                string        `json:"written"`
	Language               string        `json:"language"`
	Scope                  string        `json:"scope"`
	Normalized             string        `json:"normalized"`
	TestPhrase             string        `json:"test_phrase"`
	SupportStatus          string        `json:"support_status"`
	CriticalKind           string        `json:"critical_kind,omitempty"`
	SyntheticDataConfirmed bool          `json:"synthetic_data_confirmed"`
	Readback               *ReadbackPlan `json:"readback,omitempty"`
	PhoneQualityApproved   bool          `json:"phone_quality_approved"`
}

// ReadbackPlan is a declarative exact-value confirmation, never an instruction prompt.
type ReadbackPlan struct {
	Segments         []string `json:"segments"`
	Confirmation     string   `json:"confirmation"`
	OnMismatch       string   `json:"on_mismatch"`
	PhoneQualityGate string   `json:"phone_quality_gate"`
}

type VoiceConfig struct {
	ProfileID  string          `json:"profile_id"`
	Language   string          `json:"language"`
	Style      string          `json:"style"`
	Tempo      int             `json:"tempo"`
	Dictionary []Pronunciation `json:"dictionary"`
}

type VoiceVersion struct {
	TenantID      id.UUID     `json:"tenant_id"`
	EmployeeID    id.UUID     `json:"employee_id"`
	Version       int64       `json:"version"`
	DraftVersion  int64       `json:"draft_version"`
	Config        VoiceConfig `json:"config"`
	ActorID       id.UUID     `json:"actor_id"`
	ChangedAt     time.Time   `json:"changed_at"`
	ChangeSummary string      `json:"change_summary"`
}

type VoicePreviewRecord struct {
	TenantID          id.UUID   `json:"tenant_id"`
	EmployeeID        id.UUID   `json:"employee_id"`
	PreviewID         id.UUID   `json:"preview_id"`
	VoiceVersion      int64     `json:"voice_version"`
	DictionaryVersion int64     `json:"dictionary_version"`
	DraftVersion      int64     `json:"draft_version"`
	Text              string    `json:"text"`
	SynthesisText     string    `json:"synthesis_text"`
	Route             string    `json:"route"`
	Slow              bool      `json:"slow"`
	ActorID           id.UUID   `json:"actor_id"`
	CreatedAt         time.Time `json:"created_at"`
	Stale             bool      `json:"stale"`
}

type VoiceAssessment struct {
	VoicePreviewRecord
	AssessmentID         id.UUID   `json:"assessment_id"`
	Verdict              string    `json:"verdict"`
	AssessedBy           id.UUID   `json:"assessed_by"`
	AssessedAt           time.Time `json:"assessed_at"`
	PhoneQualityApproved bool      `json:"phone_quality_approved"`
}

type VoiceRuleError struct{ Rule, Reason string }

func (e VoiceRuleError) Error() string { return e.Rule + ": " + e.Reason }
func (e VoiceRuleError) Unwrap() error { return ErrInvalid }

func ValidateVoice(b Brief, c VoiceConfig) error {
	phone := false
	language := false
	for _, v := range b.DesiredChannels {
		if v == "phone" {
			phone = true
		}
	}
	for _, v := range b.Languages {
		if v == c.Language {
			language = true
		}
	}
	if !phone || !language {
		return VoiceRuleError{"channel/language", "Нужен phone и совместимый язык Brief."}
	}
	var profile *VoiceProfile
	for i := range voiceCatalog {
		if voiceCatalog[i].ID == c.ProfileID {
			profile = &voiceCatalog[i]
			break
		}
	}
	if profile == nil || profile.Language != c.Language {
		return VoiceRuleError{"profile", "Профиль неизвестен или несовместим с языком."}
	}
	style := false
	for _, v := range profile.Styles {
		if v == c.Style {
			style = true
		}
	}
	if !style || c.Tempo < profile.MinTempo || c.Tempo > profile.MaxTempo {
		return VoiceRuleError{"style/tempo", "Стиль или темп не допускается профилем."}
	}
	if len(c.Dictionary) > 100 {
		return VoiceRuleError{"dictionary", "Слишком много записей."}
	}
	seen := map[string]bool{}
	seenPhrases := map[string]bool{}
	critical := map[string]bool{"vin": true, "oem": true, "article": true, "phone": true, "address": true, "date": true, "amount": true, "pack": true}
	for _, v := range c.Dictionary {
		if !length(v.Written, 1, 120) || !length(v.Normalized, 1, 240) || !length(v.TestPhrase, 1, 300) || v.Language != c.Language || v.Scope == "" {
			return VoiceRuleError{"dictionary", "Укажите написание, язык, scope, произношение и тестовую фразу."}
		}
		if strings.ContainsAny(v.Written+v.Normalized+v.TestPhrase, "<>\x00") {
			return VoiceRuleError{"dictionary", "Недопустимые символы."}
		}
		key := strings.ToLower(strings.TrimSpace(v.Written)) + "\x00" + strings.ToLower(strings.TrimSpace(v.Scope))
		if seen[key] {
			return VoiceRuleError{"dictionary", "Повторное или конфликтующее написание."}
		}
		seen[key] = true
		if v.SupportStatus != "unverified" && v.SupportStatus != "unsupported" {
			return VoiceRuleError{"support", "Поддержка произношения не подтверждена."}
		}
		if v.CriticalKind == "" || (v.CriticalKind != "none" && !critical[v.CriticalKind]) {
			return VoiceRuleError{"classification", "Явно выберите «не критическая» или вид критической сущности."}
		}
		if !v.SyntheticDataConfirmed {
			return VoiceRuleError{"synthetic data", "Подтвердите, что словарная запись содержит только вымышленные тестовые данные."}
		}
		if v.CriticalKind != "none" {
			if !critical[v.CriticalKind] || !validReadback(v.Written, v.Readback) || v.PhoneQualityApproved {
				return VoiceRuleError{"readback", "Критическая сущность требует структурированный readback и будущую проверку человеком на телефонном качестве."}
			}
		} else if v.Readback != nil || v.PhoneQualityApproved {
			return VoiceRuleError{"readback", "Readback допустим только для критической сущности."}
		}
		if strings.Count(v.TestPhrase, v.Written) != 1 {
			return VoiceRuleError{"test phrase", "Тестовая фраза должна содержать исходное написание ровно один раз."}
		}
		if seenPhrases[v.TestPhrase] {
			return VoiceRuleError{"test phrase", "Одна тестовая фраза не может относиться к двум записям."}
		}
		seenPhrases[v.TestPhrase] = true
	}
	return nil
}

func validReadback(written string, p *ReadbackPlan) bool {
	if p == nil || len(p.Segments) < 2 || len(p.Segments) > 60 || p.Confirmation != "ask_customer" || p.OnMismatch != "human_handoff" || p.PhoneQualityGate != "pending_human" {
		return false
	}
	var joined strings.Builder
	for _, segment := range p.Segments {
		if len([]rune(segment)) < 1 || len([]rune(segment)) > 32 || strings.ContainsAny(segment, "<>\x00\n\r") {
			return false
		}
		joined.WriteString(segment)
	}
	return foldEntity(written) != "" && strings.EqualFold(joined.String(), written)
}

func foldEntity(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= 'а' && r <= 'я' || r == 'ё' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ResolveSynthesisText applies only the one explicitly matching test phrase.
// A dictionary spelling outside its registered phrase is rejected to avoid a false pronunciation test.
func ResolveSynthesisText(c VoiceConfig, text string) (string, *int, error) {
	var match *int
	for i, entry := range c.Dictionary {
		if text == entry.TestPhrase {
			if match != nil {
				return "", nil, VoiceRuleError{"test phrase", "Несколько записей соответствуют одной тестовой фразе."}
			}
			j := i
			match = &j
		}
	}
	if match != nil {
		entry := c.Dictionary[*match]
		if entry.SupportStatus == "unsupported" {
			return "", nil, VoiceRuleError{"pronunciation", "Эта форма произношения не поддерживается preview."}
		}
		return strings.Replace(text, entry.Written, entry.Normalized, 1), match, nil
	}
	for _, entry := range c.Dictionary {
		if strings.Contains(text, entry.Written) {
			return "", nil, VoiceRuleError{"test phrase", "Используйте точную тестовую фразу сохранённой записи."}
		}
	}
	return text, nil, nil
}
