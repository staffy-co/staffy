package domain

import "testing"

func TestVoiceMatrixValidation(t *testing.T) {
	brief := Brief{Languages: []string{"ru"}, DesiredChannels: []string{"phone"}}
	config := VoiceConfig{ProfileID: "ru-neutral-1", Language: "ru", Style: "neutral", Tempo: 100, Dictionary: []Pronunciation{{Written: "АБ-12", Language: "ru", Scope: "Тест", Normalized: "а бэ двенадцать", TestPhrase: "Артикул АБ-12", SupportStatus: "unverified", CriticalKind: "article", SyntheticDataConfirmed: true, Readback: &ReadbackPlan{Segments: []string{"А", "Б", "-", "1", "2"}, Confirmation: "ask_customer", OnMismatch: "human_handoff", PhoneQualityGate: "pending_human"}}}}
	if err := ValidateVoice(brief, config); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Brief, *VoiceConfig)
	}{
		{"без телефона", func(b *Brief, c *VoiceConfig) { b.DesiredChannels = nil }},
		{"чужой язык", func(b *Brief, c *VoiceConfig) { c.Language = "en" }},
		{"неизвестный профиль", func(b *Brief, c *VoiceConfig) { c.ProfileID = "unknown" }},
		{"неверный темп", func(b *Brief, c *VoiceConfig) { c.Tempo = 200 }},
		{"дубликат", func(b *Brief, c *VoiceConfig) { c.Dictionary = append(c.Dictionary, c.Dictionary[0]) }},
		{"без классификации", func(b *Brief, c *VoiceConfig) { c.Dictionary[0].CriticalKind = "" }},
		{"без подтверждения вымышленных данных", func(b *Brief, c *VoiceConfig) { c.Dictionary[0].SyntheticDataConfirmed = false }},
		{"потерян разделитель", func(b *Brief, c *VoiceConfig) {
			p := *c.Dictionary[0].Readback
			p.Segments = []string{"А", "Б", "1", "2"}
			c.Dictionary[0].Readback = &p
		}},
		{"без readback", func(b *Brief, c *VoiceConfig) { c.Dictionary[0].Readback = nil }},
		{"неверные сегменты", func(b *Brief, c *VoiceConfig) {
			c.Dictionary[0].Readback = &ReadbackPlan{Segments: []string{"А", "Б", "1", "3"}, Confirmation: "ask_customer", OnMismatch: "human_handoff", PhoneQualityGate: "pending_human"}
		}},
		{"нет подтверждения", func(b *Brief, c *VoiceConfig) {
			p := *c.Dictionary[0].Readback
			p.Confirmation = ""
			c.Dictionary[0].Readback = &p
		}},
		{"пустое нормализованное значение", func(b *Brief, c *VoiceConfig) {
			c.Dictionary[0].Written = "!!!"
			c.Dictionary[0].TestPhrase = "Артикул !!!"
			c.Dictionary[0].Readback = &ReadbackPlan{Segments: []string{"!", "!!"}, Confirmation: "ask_customer", OnMismatch: "human_handoff", PhoneQualityGate: "pending_human"}
		}},
		{"недостоверная тестовая фраза", func(b *Brief, c *VoiceConfig) { c.Dictionary[0].TestPhrase = "Другой текст" }},
		{"ложная проверка", func(b *Brief, c *VoiceConfig) { c.Dictionary[0].PhoneQualityApproved = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := brief
			c := config
			c.Dictionary = append([]Pronunciation(nil), config.Dictionary...)
			tc.change(&b, &c)
			if err := ValidateVoice(b, c); err == nil {
				t.Fatal("ожидался отказ")
			}
		})
	}
}

func TestResolveSynthesisTextExactPhrase(t *testing.T) {
	c := VoiceConfig{Dictionary: []Pronunciation{{Written: "VIN", Normalized: "ви ай эн", TestPhrase: "Проверьте VIN", SupportStatus: "unverified"}}}
	text, index, err := ResolveSynthesisText(c, "Проверьте VIN")
	if err != nil || text != "Проверьте ви ай эн" || index == nil || *index != 0 {
		t.Fatalf("resolved=%q index=%v err=%v", text, index, err)
	}
	if _, _, err = ResolveSynthesisText(c, "VIN другой текст"); err == nil {
		t.Fatal("неверная тестовая фраза принята")
	}
	c.Dictionary[0].SupportStatus = "unsupported"
	if _, _, err = ResolveSynthesisText(c, "Проверьте VIN"); err == nil {
		t.Fatal("неподдерживаемая форма принята")
	}
}
