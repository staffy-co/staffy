package packs

import "github.com/artemiosu/staffy/internal/employee/domain"

// The map is private and every caller receives a copy, keeping published versions immutable.
var catalog = map[string]domain.PackManifest{
        "auto-parts@6.0.0":        {ID: "auto-parts", Version: "6.0.0", Name: "Автозапчасти", Purpose: "Настройка предпочтений, идентификации автомобиля, материала запроса детали, доказательства применимости и сравнения предложений для запросов об автозапчастях", SupportStatus: "reference_setup_only", Stage: "setup", ConnectorTypes: []string{"catalog", "inventory"}, Journeys: []string{"поиск детали", "уточнение аналога", "идентификация автомобиля"}, CriticalEntities: []string{"деталь", "артикул", "автомобиль"}, Policies: []string{"не обещать наличие и цену без проверки"}, Capabilities: []string{"setup_preferences"}, Gates: []string{"connector_check", "knowledge_eval", "simulation"}, Schema: map[string]domain.VariantField{"parts_interest": {Type: "string", Enum: []string{"original", "analogs", "both"}}, "matching_depth": {Type: "integer", Min: 1, Max: 3}}, VehicleIdentification: &domain.VehicleIdentificationSchema{MaterialKeys: []domain.VehicleMaterialKey{{Key: "vehicle.vin", Label: "VIN", Kind: "vin"}, {Key: "vehicle.make", Label: "Марка", Kind: "text"}, {Key: "vehicle.model", Label: "Модель", Kind: "text"}, {Key: "vehicle.year_of_manufacture", Label: "Год выпуска", Kind: "integer", Min: 1900, Max: 2100}, {Key: "vehicle.trim", Label: "Модификация", Kind: "text", Optional: true}, {Key: "vehicle.engine", Label: "Двигатель", Kind: "text", Optional: true}}, AlternativeSets: [][]string{{"vehicle.vin"}, {"vehicle.make", "vehicle.model", "vehicle.year_of_manufacture"}}}, PartRequests: &domain.PartRequestSchema{MaterialKeys: []domain.PartMaterialKey{{Key: "part.kind", Label: "Деталь", Kind: "text", Role: "part_kind"}, {Key: "part.side", Label: "Сторона", Kind: "text", Optional: true}, {Key: "part.quantity", Label: "Количество", Kind: "integer", Min: 1, Max: 999}, {Key: "part.preference", Label: "Предпочтение", Kind: "enum", Values: []string{"original", "analogs", "both"}, Role: "preference"}, {Key: "part.oem_number", Label: "OEM-номер", Kind: "text", Role: "oem_number"}, {Key: "part.symptom", Label: "Симптом", Kind: "text", Role: "symptom"}, {Key: "part.budget", Label: "Бюджет", Kind: "text", Optional: true}, {Key: "part.deadline", Label: "Срок", Kind: "text", Optional: true}, {Key: "part.brands", Label: "Приемлемые бренды", Kind: "text", Optional: true}, {Key: "part.constraints", Label: "Ограничения", Kind: "text", Optional: true}}, Modes: []domain.PartModeSpec{{Mode: "diagnostic", Label: "Диагностика по симптому", Priority: 1, Requires: []string{"part.symptom"}}, {Mode: "exact_repeat", Label: "Точный повтор по номеру", Priority: 2, Requires: []string{"part.kind", "part.oem_number"}}, {Mode: "original_vs_analog", Label: "Выбор: оригинал или аналог", Priority: 3, Requires: []string{"part.kind", "part.preference"}}, {Mode: "named_part", Label: "Названная деталь", Priority: 4, Requires: []string{"part.kind", "part.quantity"}}}, SafetyMarkers: []string{"тормоз", "рул", "подушк", "ремн безопасн", "абс", "есп"}, PartSearchJourney: "поиск детали"}, Fitment: &domain.FitmentPolicySchema{LinkTypes: []string{"original", "cross"}, MaxCrossDepth: 3, AllowedSources: []string{"abcp_catalog", "laximo_catalog"}, MaxAgeSeconds: 604800}, Comparison: &domain.ComparisonPolicySchema{Currencies: []string{"RUB"}, MaxOfferAgeSeconds: 86400, RankCriteria: []string{"budget", "brand", "term"}, RankCriteriaLabels: map[string]string{"budget": "Бюджет", "brand": "Бренд", "term": "Срок"}, TieBreakers: []string{"price", "term", "brand", "supplier"}, MaxVisibleVariants: 4}, PolicyRef: "auto-parts/policy/6.0.0", KnowledgeRef: "auto-parts/knowledge/6.0.0", EvalRef: "auto-parts/eval/6.0.0", UpgradeFrom: map[string]domain.PackImpact{"5.0.0": {Compatibility: "compatible", Changed: []string{"comparison"}, ReviewSlots: []string{"connectors"}}}},
        "auto-parts@5.0.0":        {ID: "auto-parts", Version: "5.0.0", Name: "Автозапчасти", Purpose: "Настройка предпочтений, идентификации автомобиля, материала запроса детали и доказательства применимости для запросов об автозапчастях", SupportStatus: "reference_setup_only", Stage: "setup", ConnectorTypes: []string{"catalog", "inventory"}, Journeys: []string{"поиск детали", "уточнение аналога", "идентификация автомобиля"}, CriticalEntities: []string{"деталь", "артикул", "автомобиль"}, Policies: []string{"не обещать наличие и цену без проверки"}, Capabilities: []string{"setup_preferences"}, Gates: []string{"connector_check", "knowledge_eval", "simulation"}, Schema: map[string]domain.VariantField{"parts_interest": {Type: "string", Enum: []string{"original", "analogs", "both"}}, "matching_depth": {Type: "integer", Min: 1, Max: 3}}, VehicleIdentification: &domain.VehicleIdentificationSchema{MaterialKeys: []domain.VehicleMaterialKey{{Key: "vehicle.vin", Label: "VIN", Kind: "vin"}, {Key: "vehicle.make", Label: "Марка", Kind: "text"}, {Key: "vehicle.model", Label: "Модель", Kind: "text"}, {Key: "vehicle.year_of_manufacture", Label: "Год выпуска", Kind: "integer", Min: 1900, Max: 2100}, {Key: "vehicle.trim", Label: "Модификация", Kind: "text", Optional: true}, {Key: "vehicle.engine", Label: "Двигатель", Kind: "text", Optional: true}}, AlternativeSets: [][]string{{"vehicle.vin"}, {"vehicle.make", "vehicle.model", "vehicle.year_of_manufacture"}}}, PartRequests: &domain.PartRequestSchema{MaterialKeys: []domain.PartMaterialKey{{Key: "part.kind", Label: "Деталь", Kind: "text", Role: "part_kind"}, {Key: "part.side", Label: "Сторона", Kind: "text", Optional: true}, {Key: "part.quantity", Label: "Количество", Kind: "integer", Min: 1, Max: 999}, {Key: "part.preference", Label: "Предпочтение", Kind: "enum", Values: []string{"original", "analogs", "both"}, Role: "preference"}, {Key: "part.oem_number", Label: "OEM-номер", Kind: "text", Role: "oem_number"}, {Key: "part.symptom", Label: "Симптом", Kind: "text", Role: "symptom"}, {Key: "part.budget", Label: "Бюджет", Kind: "text", Optional: true}, {Key: "part.deadline", Label: "Срок", Kind: "text", Optional: true}, {Key: "part.brands", Label: "Приемлемые бренды", Kind: "text", Optional: true}, {Key: "part.constraints", Label: "Ограничения", Kind: "text", Optional: true}}, Modes: []domain.PartModeSpec{{Mode: "diagnostic", Label: "Диагностика по симптому", Priority: 1, Requires: []string{"part.symptom"}}, {Mode: "exact_repeat", Label: "Точный повтор по номеру", Priority: 2, Requires: []string{"part.kind", "part.oem_number"}}, {Mode: "original_vs_analog", Label: "Выбор: оригинал или аналог", Priority: 3, Requires: []string{"part.kind", "part.preference"}}, {Mode: "named_part", Label: "Названная деталь", Priority: 4, Requires: []string{"part.kind", "part.quantity"}}}, SafetyMarkers: []string{"тормоз", "рул", "подушк", "ремн безопасн", "абс", "есп"}, PartSearchJourney: "поиск детали"}, Fitment: &domain.FitmentPolicySchema{LinkTypes: []string{"original", "cross"}, MaxCrossDepth: 3, AllowedSources: []string{"abcp_catalog", "laximo_catalog"}, MaxAgeSeconds: 604800}, PolicyRef: "auto-parts/policy/5.0.0", KnowledgeRef: "auto-parts/knowledge/5.0.0", EvalRef: "auto-parts/eval/5.0.0", UpgradeFrom: map[string]domain.PackImpact{"4.0.0": {Compatibility: "compatible", Changed: []string{"fitment"}, ReviewSlots: []string{"connectors"}}}},
        "auto-parts@4.0.0":        {ID: "auto-parts", Version: "4.0.0", Name: "Автозапчасти", Purpose: "Настройка предпочтений, идентификации автомобиля и материала запроса детали для запросов об автозапчастях", SupportStatus: "reference_setup_only", Stage: "setup", ConnectorTypes: []string{"catalog", "inventory"}, Journeys: []string{"поиск детали", "уточнение аналога", "идентификация автомобиля"}, CriticalEntities: []string{"деталь", "артикул", "автомобиль"}, Policies: []string{"не обещать наличие и цену без проверки"}, Capabilities: []string{"setup_preferences"}, Gates: []string{"connector_check", "knowledge_eval", "simulation"}, Schema: map[string]domain.VariantField{"parts_interest": {Type: "string", Enum: []string{"original", "analogs", "both"}}, "matching_depth": {Type: "integer", Min: 1, Max: 3}}, VehicleIdentification: &domain.VehicleIdentificationSchema{MaterialKeys: []domain.VehicleMaterialKey{{Key: "vehicle.vin", Label: "VIN", Kind: "vin"}, {Key: "vehicle.make", Label: "Марка", Kind: "text"}, {Key: "vehicle.model", Label: "Модель", Kind: "text"}, {Key: "vehicle.year_of_manufacture", Label: "Год выпуска", Kind: "integer", Min: 1900, Max: 2100}, {Key: "vehicle.trim", Label: "Модификация", Kind: "text", Optional: true}, {Key: "vehicle.engine", Label: "Двигатель", Kind: "text", Optional: true}}, AlternativeSets: [][]string{{"vehicle.vin"}, {"vehicle.make", "vehicle.model", "vehicle.year_of_manufacture"}}}, PartRequests: &domain.PartRequestSchema{MaterialKeys: []domain.PartMaterialKey{{Key: "part.kind", Label: "Деталь", Kind: "text", Role: "part_kind"}, {Key: "part.side", Label: "Сторона", Kind: "text", Optional: true}, {Key: "part.quantity", Label: "Количество", Kind: "integer", Min: 1, Max: 999}, {Key: "part.preference", Label: "Предпочтение", Kind: "enum", Values: []string{"original", "analogs", "both"}, Role: "preference"}, {Key: "part.oem_number", Label: "OEM-номер", Kind: "text", Role: "oem_number"}, {Key: "part.symptom", Label: "Симптом", Kind: "text", Role: "symptom"}, {Key: "part.budget", Label: "Бюджет", Kind: "text", Optional: true}, {Key: "part.deadline", Label: "Срок", Kind: "text", Optional: true}, {Key: "part.brands", Label: "Приемлемые бренды", Kind: "text", Optional: true}, {Key: "part.constraints", Label: "Ограничения", Kind: "text", Optional: true}}, Modes: []domain.PartModeSpec{{Mode: "diagnostic", Label: "Диагностика по симптому", Priority: 1, Requires: []string{"part.symptom"}}, {Mode: "exact_repeat", Label: "Точный повтор по номеру", Priority: 2, Requires: []string{"part.kind", "part.oem_number"}}, {Mode: "original_vs_analog", Label: "Выбор: оригинал или аналог", Priority: 3, Requires: []string{"part.kind", "part.preference"}}, {Mode: "named_part", Label: "Названная деталь", Priority: 4, Requires: []string{"part.kind", "part.quantity"}}}, SafetyMarkers: []string{"тормоз", "рул", "подушк", "ремн безопасн", "абс", "есп"}, PartSearchJourney: "поиск детали"}, PolicyRef: "auto-parts/policy/4.0.0", KnowledgeRef: "auto-parts/knowledge/4.0.0", EvalRef: "auto-parts/eval/4.0.0", UpgradeFrom: map[string]domain.PackImpact{"3.0.0": {Compatibility: "compatible", Changed: []string{"part_request"}, ReviewSlots: []string{"knowledge"}}}},
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
        for _, key := range []string{"auto-parts@6.0.0", "auto-parts@5.0.0", "auto-parts@4.0.0", "auto-parts@3.0.0", "auto-parts@2.0.0", "auto-parts@1.0.0", "montagio-services@1.0.0"} {
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
        if m.Fitment != nil {
                fit := *m.Fitment
                fit.LinkTypes = append([]string(nil), m.Fitment.LinkTypes...)
                fit.AllowedSources = append([]string(nil), m.Fitment.AllowedSources...)
                m.Fitment = &fit
        }
        if m.Comparison != nil {
                cmp := *m.Comparison
                cmp.Currencies = append([]string(nil), m.Comparison.Currencies...)
                cmp.RankCriteria = append([]string(nil), m.Comparison.RankCriteria...)
                cmp.RankCriteriaLabels = map[string]string{}
                for criterion, label := range m.Comparison.RankCriteriaLabels {
                        cmp.RankCriteriaLabels[criterion] = label
                }
                cmp.TieBreakers = append([]string(nil), m.Comparison.TieBreakers...)
                m.Comparison = &cmp
        }
        if m.PartRequests != nil {
                partKeys := make([]domain.PartMaterialKey, 0, len(m.PartRequests.MaterialKeys))
                for _, key := range m.PartRequests.MaterialKeys {
                        key.Values = append([]string(nil), key.Values...)
                        partKeys = append(partKeys, key)
                }
                modes := make([]domain.PartModeSpec, 0, len(m.PartRequests.Modes))
                for _, mode := range m.PartRequests.Modes {
                        mode.Requires = append([]string(nil), mode.Requires...)
                        modes = append(modes, mode)
                }
                m.PartRequests = &domain.PartRequestSchema{MaterialKeys: partKeys, Modes: modes, SafetyMarkers: append([]string(nil), m.PartRequests.SafetyMarkers...), PartSearchJourney: m.PartRequests.PartSearchJourney}
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
