package domain

import (
	"errors"
	"strings"
	"testing"
)

// mapPacks is the in-memory PackRefResolver of the refs matrix: the exact
// published pin pairs. The real adapter (composition root) answers from the
// employee pack catalog — the same contract.
type mapPacks struct{ published map[string]bool }

func (m mapPacks) Get(packID, packVersion string) bool {
	return m.published[packID+"@"+packVersion]
}

// mapCaps is the in-memory CapabilityRefResolver of the refs matrix: the
// resolved capability triples with their provider contract facts. The real
// adapter answers from the connector catalog.
type mapCaps struct{ entries map[string]CapabilityFacts }

func (m mapCaps) Capability(connectorID, connectorVersion, capabilityID string) (CapabilityFacts, bool) {
	facts, ok := m.entries[connectorID+"@"+connectorVersion+"/"+capabilityID]
	return facts, ok
}

// refsWorld is the control fixture of the refs matrix: the reference
// definition of the guard matrix resolves against the resolver pair (the
// published auto-parts@2.0.0 pack and the declared command.send-sms
// capability with Idempotent=true — the honest shape of the real catalogs).
func refsWorld() (ActionDefinition, mapPacks, mapCaps) {
	def := validDefinition()
	packs := mapPacks{published: map[string]bool{
		"auto-parts@2.0.0":        true,
		"auto-parts@1.0.0":        true,
		"montagio-services@1.0.0": true,
	}}
	caps := mapCaps{entries: map[string]CapabilityFacts{
		"staffy-synthetic-telephony@1.1.0/command.send-sms":   {Idempotent: true, Reconciliation: "query_reconcilable по GATE-REGISTRY"},
		"staffy-synthetic-telephony@1.1.0/query.trunk-status": {Idempotent: false, Reconciliation: "не требуется (read-only)"},
	}}
	return def, packs, caps
}

// TestRefsAcceptResolvableDefinition — the control branch: the reference
// definition with resolvable references and matching delivery words passes.
// Without this green branch every refusal below would be vacuous.
func TestRefsAcceptResolvableDefinition(t *testing.T) {
	def, packs, caps := refsWorld()
	if err := ValidateDefinitionRefs(def, packs, caps); err != nil {
		t.Fatalf("резолвящаяся ссылочная пара обязана проходить, получен отказ: %v", err)
	}
}

// TestRefsMatrixRefusesEveryUnresolvableReferenceByName — каждая ветка
// поимённо: одна мутированная ссылка → отказ с точным именем правила.
func TestRefsMatrixRefusesEveryUnresolvableReferenceByName(t *testing.T) {
	probes := []struct {
		name string
		rule string
		mut  func(*ActionDefinition, *mapPacks, *mapCaps)
	}{
		{"pack-не-опубликован", "pack_ref", func(d *ActionDefinition, p *mapPacks, c *mapCaps) {
			delete(p.published, "auto-parts@2.0.0")
		}},
		{"pack-версия-не-опубликована", "pack_ref", func(d *ActionDefinition, p *mapPacks, c *mapCaps) {
			d.PackRef.PackVersion = "9.9.9"
		}},
		{"pack-другой-ID", "pack_ref", func(d *ActionDefinition, p *mapPacks, c *mapCaps) {
			d.PackRef.PackID = "novofon-parts"
		}},
		{"capability-коннектор-не-существует", "required_capability", func(d *ActionDefinition, p *mapPacks, c *mapCaps) {
			d.RequiredCapability.ConnectorID = "unknown-provider"
		}},
		{"capability-версия-не-существует", "required_capability", func(d *ActionDefinition, p *mapPacks, c *mapCaps) {
			d.RequiredCapability.ConnectorVersion = "1.0.0"
		}},
		{"capability-ID-не-существует", "required_capability", func(d *ActionDefinition, p *mapPacks, c *mapCaps) {
			d.RequiredCapability.CapabilityID = "query.unknown"
		}},
		{"provider_idempotent-без-декларации-идемпотентности", "capability_class", func(d *ActionDefinition, p *mapPacks, c *mapCaps) {
			d.RequiredCapability.CapabilityID = "query.trunk-status"
		}},
		{"query_reconcilable-без-декларации-reconciliation", "capability_class", func(d *ActionDefinition, p *mapPacks, c *mapCaps) {
			d.DeliveryContract.CapabilityClass = CapabilityQueryReconcilable
			c.entries["staffy-synthetic-telephony@1.1.0/command.send-sms"] = CapabilityFacts{Idempotent: true, Reconciliation: ""}
		}},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			def, packs, caps := refsWorld()
			probe.mut(&def, &packs, &caps)
			err := ValidateDefinitionRefs(def, packs, caps)
			if err == nil {
				t.Fatalf("нерезолвящаяся ссылка «%s» обязана давать отказ", probe.name)
			}
			var ruleErr RuleError
			if !errors.As(err, &ruleErr) || ruleErr.Rule != probe.rule {
				t.Fatalf("ветка «%s»: ожидается RuleError правила %q, получено: %v", probe.name, probe.rule, err)
			}
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("ветка «%s»: отказ обязан распаковываться в ErrInvalid, получено: %v", probe.name, err)
			}
		})
	}
}

// TestRefsCrossCheckDeliveryWordsAgainstProviderContract — сверка
// delivery-слов живая (AC-3): класс действия обязан совпадать с контрактом
// провайдера, зелёные и красные ветки обеих сторон сверки.
func TestRefsCrossCheckDeliveryWordsAgainstProviderContract(t *testing.T) {
	def, packs, caps := refsWorld()

	// provider_idempotent с Idempotent=true (command.send-sms) — зелёная
	// ветка сверки: класс совпадает с декларацией провайдера.
	if err := ValidateDefinitionRefs(def, packs, caps); err != nil {
		t.Fatalf("provider_idempotent при декларации Idempotent=true обязан проходить сверку: %v", err)
	}

	// provider_idempotent при Idempotent=false (query.trunk-status) — отказ
	// с честной причиной «коннектор не объявляет идемпотентность».
	mutated := def
	mutated.RequiredCapability.CapabilityID = "query.trunk-status"
	err := ValidateDefinitionRefs(mutated, packs, caps)
	if err == nil || !strings.Contains(err.Error(), "идемпотентность") {
		t.Fatalf("provider_idempotent при Idempotent=false обязан отказывать причиной про идемпотентность, получено: %v", err)
	}

	// query_reconcilable с непустой декларацией Reconciliation — зелёная
	// ветка сверки.
	query := def
	query.DeliveryContract.CapabilityClass = CapabilityQueryReconcilable
	if err := ValidateDefinitionRefs(query, packs, caps); err != nil {
		t.Fatalf("query_reconcilable при непустой декларации reconciliation обязан проходить сверку: %v", err)
	}

	// compensatable и unsafe резолвятся ссылочно: их дисциплина — guard
	// (unsafe остаётся человеческим контуром), резолюция их не дискриминирует.
	compensatable := def
	compensatable.DeliveryContract.CapabilityClass = CapabilityCompensatable
	if err := ValidateDefinitionRefs(compensatable, packs, caps); err != nil {
		t.Fatalf("compensatable с резолвящейся ссылкой обязан проходить refs-ветку (дисциплина класса — guard): %v", err)
	}
	unsafe := def
	unsafe.DeliveryContract.CapabilityClass = CapabilityUnsafe
	if err := ValidateDefinitionRefs(unsafe, packs, caps); err != nil {
		t.Fatalf("unsafe с резолвящейся ссылкой обязан проходить refs-ветку (дисциплина класса — guard): %v", err)
	}
}

// TestRefsRefuseMissingResolvers — fail-closed: без живых каталогов
// определение не может быть допущено вовсе (резолверы обязательны).
func TestRefsRefuseMissingResolvers(t *testing.T) {
	def, packs, caps := refsWorld()
	if err := ValidateDefinitionRefs(def, nil, caps); err == nil {
		t.Fatal("отсутствие резолвера паков обязано давать отказ")
	}
	if err := ValidateDefinitionRefs(def, packs, nil); err == nil {
		t.Fatal("отсутствие резолвера capabilities обязано давать отказ")
	}
	if err := ValidateDefinitionRefs(def, nil, nil); err == nil {
		t.Fatal("отсутствие обоих резолверов обязано давать отказ")
	}
	var ruleErr RuleError
	if err := ValidateDefinitionRefs(def, nil, nil); !errors.As(err, &ruleErr) || ruleErr.Rule != "refs" {
		t.Fatalf("ожидается RuleError правила «refs», получено: %v", err)
	}
}
