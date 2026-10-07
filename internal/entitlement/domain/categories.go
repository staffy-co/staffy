package domain

import "fmt"

// UsageCategory is one entry of the fixed dictionary of Budget Meter
// categories. The dictionary is fixed in the domain and mirrored by the SQL
// CHECK of the migration: an unregistered cost category cannot exist by
// construction, and a migration adding a category without a domain entry
// fails the dictionary integrity test.
type UsageCategory string

const (
	CategoryCarrier          UsageCategory = "carrier"
	CategoryAudio            UsageCategory = "audio"
	CategoryModelTokens      UsageCategory = "model_tokens"
	CategoryTools            UsageCategory = "tools"
	CategoryConnectorCharges UsageCategory = "connector_charges"
	CategoryInfrastructure   UsageCategory = "infrastructure"
	CategoryManualMinutes    UsageCategory = "manual_minutes"
)

// CategorySpec describes one usage category: its human title, the measurement
// unit of its quantity and whether the money amount belongs to the category
// (a money category without an amount is an honest validation error, because
// the charge is the point of the record).
type CategorySpec struct {
	ID    UsageCategory
	Title string
	Unit  string
	Money bool
}

var categoryRegistry = []CategorySpec{
	{ID: CategoryCarrier, Title: "Каналы и телефония (carrier)", Unit: "мин"},
	{ID: CategoryAudio, Title: "Аудио: STT/TTS", Unit: "мин"},
	{ID: CategoryModelTokens, Title: "Токены моделей ИИ", Unit: "токены"},
	{ID: CategoryTools, Title: "Вызовы инструментов (tools)", Unit: "вызовы"},
	{ID: CategoryConnectorCharges, Title: "Платные операции Connector", Unit: "операции", Money: true},
	{ID: CategoryInfrastructure, Title: "Инфраструктура", Unit: "услуги", Money: true},
	{ID: CategoryManualMinutes, Title: "Ручные минуты оператора", Unit: "мин"},
}

var categoryIndex = func() map[UsageCategory]CategorySpec {
	index := make(map[UsageCategory]CategorySpec, len(categoryRegistry))
	for _, spec := range categoryRegistry {
		index[spec.ID] = spec
	}
	return index
}()

// CategorySpecs returns the ordered dictionary itself.
func CategorySpecs() []CategorySpec { return append([]CategorySpec(nil), categoryRegistry...) }

// AllCategories returns the ordered category identifiers.
func AllCategories() []UsageCategory {
	out := make([]UsageCategory, 0, len(categoryRegistry))
	for _, spec := range categoryRegistry {
		out = append(out, spec.ID)
	}
	return out
}

// LookupCategory resolves one dictionary entry.
func LookupCategory(category UsageCategory) (CategorySpec, error) {
	spec, ok := categoryIndex[category]
	if !ok {
		return CategorySpec{}, fmt.Errorf("%w: %s", ErrUnknownCategory, category)
	}
	return spec, nil
}

// CategoryTitles maps every category to its human title (view helper).
func CategoryTitles() map[UsageCategory]string {
	titles := make(map[UsageCategory]string, len(categoryRegistry))
	for _, spec := range categoryRegistry {
		titles[spec.ID] = spec.Title
	}
	return titles
}
