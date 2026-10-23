#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""step-01 surgical patch 2/4: internal/employee/packs/catalog.go
+auto-parts@5.0.0 (полный манифест 4.0.0 + Fitment) + List + clone (урок 4-3:
новые поля-слова обязаны проходить clone())."""
import io

P = 'internal/employee/packs/catalog.go'
s = io.open(P, encoding='utf-8').read()

# 1) Запись 5.0.0 — вставка ПЕРВОЙ строкой map (после "var catalog = map[...]{")
anchor = 'var catalog = map[string]domain.PackManifest{\n'
assert anchor in s, 'catalog.go: map anchor not found'

manifest_5 = '\t"auto-parts@5.0.0":        {ID: "auto-parts", Version: "5.0.0", Name: "Автозапчасти", Purpose: "Настройка предпочтений, идентификации автомобиля, материала запроса детали и доказательства применимости для запросов об автозапчастях", SupportStatus: "reference_setup_only", Stage: "setup", ConnectorTypes: []string{"catalog", "inventory"}, Journeys: []string{"поиск детали", "уточнение аналога", "идентификация автомобиля"}, CriticalEntities: []string{"деталь", "артикул", "автомобиль"}, Policies: []string{"не обещать наличие и цену без проверки"}, Capabilities: []string{"setup_preferences"}, Gates: []string{"connector_check", "knowledge_eval", "simulation"}, Schema: map[string]domain.VariantField{"parts_interest": {Type: "string", Enum: []string{"original", "analogs", "both"}}, "matching_depth": {Type: "integer", Min: 1, Max: 3}}, VehicleIdentification: &domain.VehicleIdentificationSchema{MaterialKeys: []domain.VehicleMaterialKey{{Key: "vehicle.vin", Label: "VIN", Kind: "vin"}, {Key: "vehicle.make", Label: "Марка", Kind: "text"}, {Key: "vehicle.model", Label: "Модель", Kind: "text"}, {Key: "vehicle.year_of_manufacture", Label: "Год выпуска", Kind: "integer", Min: 1900, Max: 2100}, {Key: "vehicle.trim", Label: "Модификация", Kind: "text", Optional: true}, {Key: "vehicle.engine", Label: "Двигатель", Kind: "text", Optional: true}}, AlternativeSets: [][]string{{"vehicle.vin"}, {"vehicle.make", "vehicle.model", "vehicle.year_of_manufacture"}}}, PartRequests: &domain.PartRequestSchema{MaterialKeys: []domain.PartMaterialKey{{Key: "part.kind", Label: "Деталь", Kind: "text", Role: "part_kind"}, {Key: "part.side", Label: "Сторона", Kind: "text", Optional: true}, {Key: "part.quantity", Label: "Количество", Kind: "integer", Min: 1, Max: 999}, {Key: "part.preference", Label: "Предпочтение", Kind: "enum", Values: []string{"original", "analogs", "both"}, Role: "preference"}, {Key: "part.oem_number", Label: "OEM-номер", Kind: "text", Role: "oem_number"}, {Key: "part.symptom", Label: "Симптом", Kind: "text", Role: "symptom"}, {Key: "part.budget", Label: "Бюджет", Kind: "text", Optional: true}, {Key: "part.deadline", Label: "Срок", Kind: "text", Optional: true}, {Key: "part.brands", Label: "Приемлемые бренды", Kind: "text", Optional: true}, {Key: "part.constraints", Label: "Ограничения", Kind: "text", Optional: true}}, Modes: []domain.PartModeSpec{{Mode: "diagnostic", Label: "Диагностика по симптому", Priority: 1, Requires: []string{"part.symptom"}}, {Mode: "exact_repeat", Label: "Точный повтор по номеру", Priority: 2, Requires: []string{"part.kind", "part.oem_number"}}, {Mode: "original_vs_analog", Label: "Выбор: оригинал или аналог", Priority: 3, Requires: []string{"part.kind", "part.preference"}}, {Mode: "named_part", Label: "Названная деталь", Priority: 4, Requires: []string{"part.kind", "part.quantity"}}}, SafetyMarkers: []string{"тормоз", "рул", "подушк", "ремн безопасн", "абс", "есп"}, PartSearchJourney: "поиск детали"}, Fitment: &domain.FitmentPolicySchema{LinkTypes: []string{"original", "cross"}, MaxCrossDepth: 3, AllowedSources: []string{"abcp_catalog", "laximo_catalog"}, MaxAgeSeconds: 604800}, PolicyRef: "auto-parts/policy/5.0.0", KnowledgeRef: "auto-parts/knowledge/5.0.0", EvalRef: "auto-parts/eval/5.0.0", UpgradeFrom: map[string]domain.PackImpact{"4.0.0": {Compatibility: "compatible", Changed: []string{"fitment"}, ReviewSlots: []string{"connectors"}}}},\n'

s = s.replace(anchor, anchor + manifest_5, 1)

# 2) List(): 5.0.0 первым в порядке
old_list = 'for _, key := range []string{"auto-parts@4.0.0", "auto-parts@3.0.0", "auto-parts@2.0.0", "auto-parts@1.0.0", "montagio-services@1.0.0"} {'
new_list = 'for _, key := range []string{"auto-parts@5.0.0", "auto-parts@4.0.0", "auto-parts@3.0.0", "auto-parts@2.0.0", "auto-parts@1.0.0", "montagio-services@1.0.0"} {'
assert old_list in s, 'catalog.go: list anchor not found'
s = s.replace(old_list, new_list, 1)

# 3) clone(): глубокая копия Fitment (урок 4-3 — поля-слова обязаны проходить clone)
old_clone = '''\tif m.PartRequests != nil {'''
new_clone = '''\tif m.Fitment != nil {
\t\tfit := *m.Fitment
\t\tfit.LinkTypes = append([]string(nil), m.Fitment.LinkTypes...)
\t\tfit.AllowedSources = append([]string(nil), m.Fitment.AllowedSources...)
\t\tm.Fitment = &fit
\t}
\tif m.PartRequests != nil {'''
assert old_clone in s, 'catalog.go: clone anchor not found'
s = s.replace(old_clone, new_clone, 1)

io.open(P, 'w', encoding='utf-8').write(s)
print('catalog.go: +5.0.0 manifest, +List, +clone(Fitment) OK')
