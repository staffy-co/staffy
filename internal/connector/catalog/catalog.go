package catalog

import (
	"github.com/artemiosu/staffy/internal/connector/domain"
)

// definitions is the immutable binary catalog of published Connector definitions.
// Adding a provider means adding a definition (and an adapter) — never a Tenant branch.
var definitions = []domain.ConnectorDefinition{
	{
		ID:                  "staffy-synthetic-telephony",
		Version:             "1.0.0",
		Type:                domain.TypeSIP,
		Name:                "Staffy Synthetic Telephony",
		Purpose:             "Локальный синтетический SIP-контур для безопасной отладки подключений без реального провайдера.",
		SupportStatus:       domain.StatusSupported,
		AdapterImplemented:  true,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "synthetic",
		DocumentationURL:    "https://docs.bmad-method.org/",
		Fields: []domain.FieldSchema{
			{Name: "endpoint", Label: "Endpoint", Type: domain.FieldURL, Required: true, Description: "Финальный адрес вида https://<имя>.sandbox.example/api"},
			{Name: "trunk_label", Label: "Метка транка", Type: domain.FieldText, Required: false, Description: "Понятное имя тестового транка"},
			{Name: "api_key", Label: "API-ключ (sandbox)", Type: domain.FieldText, Required: true, Secret: true, Description: "Формат sandbox-… Значение принимается один раз и больше не показывается."},
		},
	},
	{
		ID:                  "novofon-sip",
		Version:             "1.0.0",
		Type:                domain.TypeSIP,
		Name:                "Novofon",
		Purpose:             "Телефония для пилота Montagio. Адаптер ещё не реализован.",
		SupportStatus:       domain.StatusCandidate,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "novofon_business",
		DocumentationURL:    "https://novofon.com/",
		// PriorityNote is only rendered when AdapterImplemented is true.
		PriorityNote: "Приоритетный кандидат телефонии Montagio",
		Fields:       []domain.FieldSchema{},
	},
	{
		ID:                  "mango-office-sip",
		Version:             "1.0.0",
		Type:                domain.TypeSIP,
		Name:                "Mango Office",
		Purpose:             "Телефония для магазина Олега. Кандидат до проверки фактического аккаунта.",
		SupportStatus:       domain.StatusCandidate,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "mango_office",
		DocumentationURL:    "https://www.mango-office.ru/",
		Fields:              []domain.FieldSchema{},
	},
	{
		ID:                  "jivo-chat",
		Version:             "1.0.0",
		Type:                domain.TypeChat,
		Name:                "Jivo",
		Purpose:             "Чат-канал. Реализованные типы подключения отсутствуют.",
		SupportStatus:       domain.StatusDeferred,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "jivo_site",
		DocumentationURL:    "https://www.jivo.ru/",
		Fields:              []domain.FieldSchema{},
	},
	{
		ID:                  "abcp-parts",
		Version:             "1.0.0",
		Type:                domain.TypeBusiness,
		Name:                "ABCP / Laximo",
		Purpose:             "Бизнес-система автозапчастей для пилота магазина Олега. Реализованные типы подключения отсутствуют.",
		SupportStatus:       domain.StatusDeferred,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "abcp_partner",
		DocumentationURL:    "https://www.abcp.ru/",
		Fields:              []domain.FieldSchema{},
	},
	{
		ID:                  "yandex-speechkit",
		Version:             "1.0.0",
		Type:                domain.TypeAISpeech,
		Name:                "Yandex SpeechKit",
		Purpose:             "Речевые сервисы. Разрешённые типы подключения не реализованы.",
		SupportStatus:       domain.StatusDeferred,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
		RequiredAccountType: "yandex_cloud",
		DocumentationURL:    "https://yandex.cloud/ru/services/speechkit",
		Fields:              []domain.FieldSchema{},
	},
	{
		ID:                  "exolve-sip",
		Version:             "1.0.0",
		Type:                domain.TypeSIP,
		Name:                "Exolve",
		Purpose:             "Определение опубликовано, но подключение пока не предлагается.",
		SupportStatus:       domain.StatusUnavailable,
		AdapterImplemented:  false,
		AllowedEnvironments: []domain.Environment{domain.EnvSandbox},
		RequiredAccountType: "exolve_account",
		DocumentationURL:    "https://exolve.ru/",
		Fields:              []domain.FieldSchema{},
	},
}

// List returns a defensive copy of every published definition.
func List() []domain.ConnectorDefinition {
	out := make([]domain.ConnectorDefinition, len(definitions))
	for i, d := range definitions {
		out[i] = clone(d)
	}
	return out
}

// Get pins one definition by id and version.
func Get(id, version string) (domain.ConnectorDefinition, bool) {
	for _, d := range definitions {
		if d.ID == id && d.Version == version {
			return clone(d), true
		}
	}
	return domain.ConnectorDefinition{}, false
}

func clone(d domain.ConnectorDefinition) domain.ConnectorDefinition {
	d.AllowedEnvironments = append([]domain.Environment(nil), d.AllowedEnvironments...)
	d.Fields = append([]domain.FieldSchema(nil), d.Fields...)
	return d
}

// GroupByType preserves the fixed PRD type order.
func GroupByType(items []domain.ConnectorDefinition) map[domain.ConnectorType][]domain.ConnectorDefinition {
	order := []domain.ConnectorType{domain.TypeSIP, domain.TypeChat, domain.TypeBusiness, domain.TypeAISpeech, domain.TypeStorage}
	grouped := map[domain.ConnectorType][]domain.ConnectorDefinition{}
	for _, t := range order {
		grouped[t] = []domain.ConnectorDefinition{}
	}
	for _, item := range items {
		grouped[item.Type] = append(grouped[item.Type], item)
	}
	return grouped
}
