package packs

import "github.com/artemiosu/staffy/internal/employee/domain"

// The map is private and every caller receives a copy, keeping published versions immutable.
var catalog = map[string]domain.PackManifest{
	"auto-parts@3.0.0":        {ID: "auto-parts", Version: "3.0.0", Name: "Автозапчасти", Purpose: "Настройка предпочтений и идентификации автомобиля для запросов об автозапчастях", SupportStatus: "reference_setup_only", Stage: "setup", ConnectorTypes: []string{"catalog", "inventory"}, Journeys: []string{"поиск детали", "уточнение аналога", "идентификация автомобиля"}, CriticalEntities: []string{"деталь", "артикул", "автомобиль"}, Policies: []string{"не обещать наличие и цену без проверки"}, Capabilities: []string{"setup_preferences"}, Gates: []string{"connector_check", "knowledge_eval", "simulation"}, Schema: map[string]domain.VariantField{"parts_interest": {Type: "string", Enum: []string{"original", "analogs", "both"}}, "matching_depth": {Type: "integer", Min: 1, Max: 3}}, VehicleIdentification: &domain.VehicleIdentificationSchema{MaterialKeys: []domain.VehicleMaterialKey{{Key: "vehicle.vin", Label: "VIN", Kind: "vin"}, {Key: "vehicle.make", Label: "Марка", Kind: "text"}, {Key: "vehicle.model", Label: "Модель", Kind: "text"}, {Key: "vehicle.year_of_manufacture", Label: "Год выпуска", Kind: "integer", Min: 1900, Max: 2100}, {Key: "vehicle.trim", Label: "Модификация", Kind: "text", Optional: true}, {Key: "vehicle.engine", Label: "Двигатель", Kind: "text", Optional: true}}, AlternativeSets: [][]string{{"vehicle.vin"}, {"vehicle.make", "vehicle.model", "vehicle.year_of_manufacture"}}}, PolicyRef: "auto-parts/policy/3.0.0", KnowledgeRef: "auto-parts/knowledge/3.0.0", EvalRef: "auto-parts/eval/3.0.0", UpgradeFrom: map[string]domain.PackImpact{"2.0.0": {Compatibility: "compatible", Changed: []string{"identification"}, ReviewSlots: []string{"connectors"}}}},
	"auto-parts@1.0.0":        {ID: "auto-parts", Version: "1.0.0", Name: "Автозапчасти", Purpose: "Настройка предпочтений для запросов об автозапчастях", SupportStatus: "reference_setup_only", Stage: "setup", ConnectorTypes: []string{"catalog", "inventory"}, Journeys: []string{"поиск детали", "уточнение аналога"}, CriticalEntities: []string{"деталь", "артикул", "автомобиль"}, Policies: []string{"не обещать наличие и цену без проверки"}, Capabilities: []string{"setup_preferences"}, Gates: []string{"connector_check", "knowledge_eval", "simulation"}, Schema: map[string]domain.VariantField{"parts_interest": {Type: "string", Enum: []string{"original", "analogs", "both"}}}, PolicyRef: "auto-parts/policy/1.0.0", KnowledgeRef: "auto-parts/knowledge/1.0.0", EvalRef: "auto-parts/eval/1.0.0", UpgradeFrom: map[string]domain.PackImpact{"2.0.0": {Compatibility: "incompatible", Changed: []string{"schema", "policy", "eval"}, ReviewSlots: []string{"connectors", "knowledge", "behavior"}}}},
	"auto-parts@2.0.0":        {ID: "auto-parts", Version: "2.0.0", Name: "Автозапчасти", Purpose: "Настройка предпочтений для запросов об автозапчастях", SupportStatus: "reference_setup_only", Stage: "setup", ConnectorTypes: []string{"catalog", "inventory"}, Journeys: []string{"поиск детали", "уточнение аналога"}, CriticalEntities: []string{"деталь", "артикул", "автомобиль"}, Policies: []string{"не обещать наличие и цену без проверки"}, Capabilities: []string{"setup_preferences"}, Gates: []string{"connector_check", "knowledge_eval", "simulation"}, Schema: map[string]domain.VariantField{"parts_interest": {Type: "string", Enum: []string{"original", "analogs", "both"}}, "matching_depth": {Type: "integer", Min: 1, Max: 3}}, PolicyRef: "auto-parts/policy/2.0.0", KnowledgeRef: "auto-parts/knowledge/2.0.0", EvalRef: "auto-parts/eval/2.0.0", UpgradeFrom: map[string]domain.PackImpact{"1.0.0": {Compatibility: "incompatible", Changed: []string{"schema", "policy", "eval"}, ReviewSlots: []string{"connectors", "knowledge", "behavior"}}}},
	"montagio-services@1.0.0": {ID: "montagio-services", Version: "1.0.0", Name: "Услуги Montagio", Purpose: "Настройка направлений инженерных услуг", SupportStatus: "reference_setup_only", Stage: "setup", ConnectorTypes: []string{"service_catalog", "scheduling"}, Journeys: []string{"уточнение услуги", "передача мастеру"}, CriticalEntities: []string{"услуга", "заявка"}, Policies: []string{"не обещать доступность и выполнение без проверки"}, Capabilities: []string{"setup_preferences"}, Gates: []string{"connector_check", "knowledge_eval", "simulation"}, Schema: map[string]domain.VariantField{"engineering_plumbing": {Type: "boolean"}, "heating": {Type: "boolean"}}, AtLeastOneOf: []string{"engineering_plumbing", "heating"}, PolicyRef: "montagio-services/policy/1.0.0", KnowledgeRef: "montagio-services/knowledge/1.0.0", EvalRef: "montagio-services/eval/1.0.0"},
}

var replacementImpact = domain.PackImpact{Compatibility: "incompatible", Changed: []string{"domain", "schema", "policy", "knowledge", "eval"}, ReviewSlots: []string{"connectors", "knowledge", "behavior"}}

func Get(id, version string) (domain.PackManifest, bool) {
	m, ok := catalog[id+"@"+version]
	return clone(m), ok
}
func List() []domain.PackManifest {
	out := make([]domain.PackManifest, 0, len(catalog))
	for _, key := range []string{"auto-parts@3.0.0", "auto-parts@2.0.0", "auto-parts@1.0.0", "montagio-services@1.0.0"} {
		out = append(out, clone(catalog[key]))
	}
	return out
}
func Impact(from domain.PackPin, to domain.PackManifest) (domain.PackImpact, bool) {
	if from.ID != to.ID {
		return to.ReplacementImpact, to.ReplacementImpact.Compatibility != ""
	}
	impact, ok := to.UpgradeFrom[from.Version]
	return impact, ok
}
func clone(m domain.PackManifest) domain.PackManifest {
	m.ReplacementImpact = domain.PackImpact{Compatibility: replacementImpact.Compatibility, Changed: append([]string(nil), replacementImpact.Changed...), ReviewSlots: append([]string(nil), replacementImpact.ReviewSlots...)}
	m.ConnectorTypes = append([]string(nil), m.ConnectorTypes...)
	m.Journeys = append([]string(nil), m.Journeys...)
	m.CriticalEntities = append([]string(nil), m.CriticalEntities...)
	m.Policies = append([]string(nil), m.Policies...)
	m.Capabilities = append([]string(nil), m.Capabilities...)
	m.Gates = append([]string(nil), m.Gates...)
	m.AtLeastOneOf = append([]string(nil), m.AtLeastOneOf...)
	if m.VehicleIdentification != nil {
		keys := make([]domain.VehicleMaterialKey, 0, len(m.VehicleIdentification.MaterialKeys))
		for _, key := range m.VehicleIdentification.MaterialKeys {
			keys = append(keys, key)
		}
		sets := make([][]string, 0, len(m.VehicleIdentification.AlternativeSets))
		for _, set := range m.VehicleIdentification.AlternativeSets {
			sets = append(sets, append([]string(nil), set...))
		}
		m.VehicleIdentification = &domain.VehicleIdentificationSchema{MaterialKeys: keys, AlternativeSets: sets}
	}
	schema := map[string]domain.VariantField{}
	for k, v := range m.Schema {
		switch k {
		case "parts_interest":
			v.Label = "Интерес к деталям"
		case "matching_depth":
			v.Label = "Глубина сопоставления"
		case "engineering_plumbing":
			v.Label = "Инженерная сантехника"
		case "heating":
			v.Label = "Отопление"
		}
		v.Enum = append([]string(nil), v.Enum...)
		schema[k] = v
	}
	m.Schema = schema
	upgrades := map[string]domain.PackImpact{}
	for k, v := range m.UpgradeFrom {
		v.Changed = append([]string(nil), v.Changed...)
		v.ReviewSlots = append([]string(nil), v.ReviewSlots...)
		upgrades[k] = v
	}
	m.UpgradeFrom = upgrades
	return m
}
