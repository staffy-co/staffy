package domain

import "fmt"

type VariantError struct{ Fields map[string]string }

func (e VariantError) Error() string { return "invalid domain pack variants" }
func (e VariantError) Unwrap() error { return ErrInvalid }

// PackManifest is declarative setup metadata. It never contains executable tenant policy.
type PackManifest struct {
	ID               string                  `json:"id"`
	Version          string                  `json:"version"`
	Name             string                  `json:"name"`
	Purpose          string                  `json:"purpose"`
	SupportStatus    string                  `json:"support_status"`
	Stage            string                  `json:"stage"`
	ConnectorTypes   []string                `json:"connector_types"`
	Journeys         []string                `json:"journeys"`
	CriticalEntities []string                `json:"critical_entities"`
	Policies         []string                `json:"policies"`
	Capabilities     []string                `json:"capabilities"`
	Gates            []string                `json:"gates"`
	Schema           map[string]VariantField `json:"schema"`
	// VehicleIdentification is the identification schema of Story 4.2:
	// nil for every pack without one (the 1.0.0/2.0.0 auto-parts and
	// montagio-services stay nil — the immutability of the published
	// versions is never violated).
	VehicleIdentification *VehicleIdentificationSchema `json:"vehicle_identification,omitempty"`
	// PartRequests is the part request schema of Story 4.3: nil for
	// every pack without one (the 1.0.0/2.0.0/3.0.0 auto-parts and
	// montagio-services stay nil — the immutability of the published
	// versions is never violated; the first schema arrives with
	// auto-parts@4.0.0).
	PartRequests      *PartRequestSchema    `json:"part_requests,omitempty"`
	AtLeastOneOf      []string              `json:"at_least_one_of,omitempty"`
	PolicyRef         string                `json:"policy_ref"`
	KnowledgeRef      string                `json:"knowledge_ref"`
	EvalRef           string                `json:"eval_ref"`
	UpgradeFrom       map[string]PackImpact `json:"upgrade_from,omitempty"`
	ReplacementImpact PackImpact            `json:"replacement_impact"`
}

// VehicleMaterialKey is one identification material key of the pack
// schema (Story 4.2): the fact form hint of the vehicle identification
// panel — the key, the human label, the kind (vin | text | integer), the
// integer bounds and the optional flag. The schema is declarative setup
// metadata: it never carries executable tenant policy.
type VehicleMaterialKey struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Min      int    `json:"min,omitempty"`
	Max      int    `json:"max,omitempty"`
	Optional bool   `json:"optional,omitempty"`
}

// VehicleIdentificationSchema is the versioned identification schema of
// one Domain Pack (Story 4.2): the material keys of the vehicle facts and
// the alternative sufficient sets (any ONE set satisfies the
// identification — «VIN либо допустимая комбинация»). Packs without a
// schema carry nil (montagio-services is untouched).
type VehicleIdentificationSchema struct {
	MaterialKeys    []VehicleMaterialKey `json:"material_keys"`
	AlternativeSets [][]string           `json:"alternative_sets"`
}

// PartMaterialKey is one request material key of the pack schema (Story
// 4.3): the fact form hint of the part request panel — the key, the human
// label, the kind (text | integer | enum), the enum values, the integer
// bounds, the optional flag and the ROLE of the key in the request
// classification (part_kind | symptom | oem_number | preference | пусто —
// the role drives the deterministic mode ladder, the mandatory-field
// dictionary comes from the pack, never from a store's code). The schema
// is declarative setup metadata: it never carries executable tenant
// policy.
type PartMaterialKey struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Kind     string   `json:"kind"` // text | integer | enum
	Values   []string `json:"values,omitempty"`
	Min      int      `json:"min,omitempty"`
	Max      int      `json:"max,omitempty"`
	Optional bool     `json:"optional,omitempty"`
	Role     string   `json:"role,omitempty"` // part_kind | symptom | oem_number | preference | пусто
}

// PartModeSpec is one request mode of the pack dictionary (Story 4.3):
// the mode word, the human label, the ladder priority (the LOWER number
// wins — symptom dominates always) and the mandatory key set of the mode
// (the completeness is derived against the SELECTED mode's set only).
type PartModeSpec struct {
	Mode     string   `json:"mode"`
	Label    string   `json:"label"`
	Priority int      `json:"priority"`
	Requires []string `json:"requires"`
}

// PartRequestSchema is the versioned part request schema of one Domain
// Pack (Story 4.3): the material keys with the roles, the mode dictionary
// with the priorities and the mandatory sets, and the safety markers (the
// declared heuristic of the safety-critical nodes — a substring match
// over the normalized part.kind, honestly a heuristic, never a proof).
// Packs without a schema carry nil.
type PartRequestSchema struct {
	MaterialKeys  []PartMaterialKey `json:"material_keys"`
	Modes         []PartModeSpec    `json:"modes"`
	SafetyMarkers []string          `json:"safety_markers"`
}

type VariantField struct {
	Label     string   `json:"label"`
	Type      string   `json:"type"`
	Enum      []string `json:"enum,omitempty"`
	Min       int      `json:"min,omitempty"`
	Max       int      `json:"max,omitempty"`
	Protected bool     `json:"protected,omitempty"`
}

type PackImpact struct {
	Compatibility string   `json:"compatibility"`
	Changed       []string `json:"changed"`
	ReviewSlots   []string `json:"review_slots"`
}

type PackPin struct {
	ID       string         `json:"id"`
	Version  string         `json:"version"`
	Variants map[string]any `json:"variants"`
}

type PackPreview struct {
	Current PackPin      `json:"current"`
	Target  PackManifest `json:"target"`
	Impact  PackImpact   `json:"impact"`
}

func ValidateVariants(m PackManifest, values map[string]any) map[string]string {
	errors := map[string]string{}
	for key, field := range m.Schema {
		if !field.Protected {
			if _, ok := values[key]; !ok {
				errors[key] = "Укажите значение."
			}
		}
	}
	for key, value := range values {
		field, ok := m.Schema[key]
		if !ok || field.Protected {
			errors[key] = "Поле нельзя изменять."
			continue
		}
		switch field.Type {
		case "string":
			v, ok := value.(string)
			if !ok {
				errors[key] = "Ожидается текст."
				continue
			}
			valid := false
			for _, candidate := range field.Enum {
				if v == candidate {
					valid = true
				}
			}
			if len(field.Enum) > 0 && !valid {
				errors[key] = "Недопустимое значение."
			}
		case "integer":
			v, ok := value.(float64)
			if !ok || v != float64(int(v)) || int(v) < field.Min || int(v) > field.Max {
				errors[key] = fmt.Sprintf("Число от %d до %d.", field.Min, field.Max)
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				errors[key] = "Ожидается да или нет."
			}
		default:
			errors[key] = "Неизвестный тип поля."
		}
	}
	if len(m.AtLeastOneOf) > 0 {
		selected := false
		for _, key := range m.AtLeastOneOf {
			if values[key] == true {
				selected = true
			}
		}
		if !selected {
			errors[m.AtLeastOneOf[0]] = "Выберите хотя бы одно направление."
		}
	}
	return errors
}
