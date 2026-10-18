package capabilityresolver

import (
	"strings"
	"testing"
)

// TestCapabilityResolvesCompleteDeclarations — полные декларации живого
// каталога коннекторов резолвятся с фактами контракта провайдера
// (идемпотентность и reconciliation слова сверки AC-3).
func TestCapabilityResolvesCompleteDeclarations(t *testing.T) {
	adapter := New()
	facts, ok := adapter.Capability("staffy-synthetic-telephony", "1.1.0", "command.send-sms")
	if !ok {
		t.Fatal("полная декларация command.send-sms обязана резолвиться")
	}
	if !facts.Idempotent {
		t.Fatal("command.send-sms декларирует Idempotent=true — факт обязан это отражать")
	}
	if strings.TrimSpace(facts.Reconciliation) == "" {
		t.Fatal("command.send-sms декларирует reconciliation — факт обязан это нести")
	}
	facts, ok = adapter.Capability("staffy-synthetic-telephony", "1.1.0", "query.trunk-status")
	if !ok {
		t.Fatal("полная декларация query.trunk-status обязана резолвиться")
	}
	if facts.Idempotent {
		t.Fatal("query.trunk-status декларирует Idempotent=false — факт обязан это отражать (честная сверка delivery-слов)")
	}
}

// TestCapabilityRefusesUnresolvableTriples — неизвестный коннектор,
// неизвестная версия (1.0.0 без деклараций), неизвестная capability —
// честный отказ, никогда не догадка.
func TestCapabilityRefusesUnresolvableTriples(t *testing.T) {
	adapter := New()
	for _, probe := range []struct{ connector, version, capability string }{
		{"unknown-provider", "1.1.0", "command.send-sms"},
		{"staffy-synthetic-telephony", "9.9.9", "command.send-sms"},
		{"staffy-synthetic-telephony", "1.0.0", "command.send-sms"},
		{"staffy-synthetic-telephony", "1.1.0", "query.unknown"},
		{"", "1.1.0", "command.send-sms"},
		{"staffy-synthetic-telephony", "", "command.send-sms"},
		{"staffy-synthetic-telephony", "1.1.0", ""},
	} {
		if _, ok := adapter.Capability(probe.connector, probe.version, probe.capability); ok {
			t.Fatalf("тройка %q@%q/%q не обязана резолвиться", probe.connector, probe.version, probe.capability)
		}
	}
}
