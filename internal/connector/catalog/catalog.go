package catalog

import (
        "github.com/artemiosu/staffy/internal/connector/domain"
)

// definitions is the immutable binary catalog of published Connector definitions.
// Adding a provider means adding a definition (and an adapter) — never a Tenant branch.
// Older versions stay resolvable for pinned connections (Get pins an exact
// version); List only offers the newest version of every connector, so new
// Connections always pin the current contract.
var definitions = []domain.ConnectorDefinition{
        {
                ID:                  "staffy-synthetic-telephony",
                Version:             "1.1.0",
                Type:                domain.TypeSIP,
                Name:                "Staffy Synthetic Telephony",
                Purpose:             "Локальный синтетический SIP-контур для безопасной отладки подключений и проверки capabilities без реального провайдера.",
                SupportStatus:       domain.StatusSupported,
                AdapterImplemented:  true,
                AllowedEnvironments: []domain.Environment{domain.EnvSandbox, domain.EnvTest},
                RequiredAccountType: "synthetic",
                DocumentationURL:    "https://docs.bmad-method.org/",
                Fields: []domain.FieldSchema{
                        {Name: "endpoint", Label: "Endpoint", Type: domain.FieldURL, Required: true, Description: "Финальный адрес вида https://<имя>.sandbox.example/api (для сценария таймаута: https://timeout.<имя>.sandbox.example/api)"},
                        {Name: "trunk_label", Label: "Метка транка", Type: domain.FieldText, Required: false, Description: "Понятное имя тестового транка"},
                        {Name: "api_key", Label: "API-ключ (sandbox)", Type: domain.FieldText, Required: true, Secret: true, Description: "Формат sandbox-… Значение принимается один раз и больше не показывается."},
                },
                Capabilities: []domain.CapabilityDeclaration{
                        {
                                ID: "query.trunk-status", Title: "Статус транка", Description: "Чтение текущего статуса тестового транка (read-only).",
                                Direction: domain.DirectionOutbound, Operation: domain.OperationQuery,
                                TimeoutMS: 5000, Ordering: "unordered", RateLimit: "60 запросов/мин", SizeLimit: "4 КБ ответа",
                                Idempotent: false, Receipt: "query_result", ReceiptLevel: domain.DeliveryDelivered,
                                CancelCompensation: "не применимо (read-only)", Reconciliation: "не требуется (read-only)", ReconciliationOwner: "Technical Lead",
                                ProbeAllowed: true,
                        },
                        {
                                ID: "command.send-sms", Title: "Отправка SMS", Description: "Мутирующая операция отправки SMS; проверка заблокирована до Action Gateway.",
                                Direction: domain.DirectionOutbound, Operation: domain.OperationCommand,
                                TimeoutMS: 10000, Ordering: "fifo", RateLimit: "30 сообщений/мин", SizeLimit: "1 КБ полезной нагрузки",
                                Idempotent: true, Receipt: "accepted → delivered → committed", ReceiptLevel: domain.DeliveryCommitted,
                                CancelCompensation: "отмена недоступна после accepted; компенсация не поддерживается провайдером", Reconciliation: "query_reconcilable по GATE-REGISTRY: статус доставки сверяется повторным запросом", ReconciliationOwner: "Technical Lead",
                                ProbeAllowed: false,
                        },
                        {
                                ID: "event.inbound-call", Title: "Входящий звонок", Description: "Приём события входящего звонка; путь проверки не реализован в текущем срезе.",
                                Direction: domain.DirectionInbound, Operation: domain.OperationEvent,
                                TimeoutMS: 5000, Ordering: "fifo", RateLimit: "120 событий/мин", SizeLimit: "16 КБ события",
                                Idempotent: false, Receipt: "event_ack", ReceiptLevel: domain.DeliveryAccepted,
                                CancelCompensation: "не применимо (inbound)", Reconciliation: "дедупликация по event_id на стороне staffy", ReconciliationOwner: "Technical Lead",
                                ProbeAllowed: false,
                        },
                },
        },
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

// List returns a defensive copy of the newest published version of every
// connector. Pinned older versions stay resolvable through Get only, so the
// catalog cards always offer the current contract to new Connections.
func List() []domain.ConnectorDefinition {
        out := make([]domain.ConnectorDefinition, 0, len(definitions))
        seen := map[string]bool{}
        for _, d := range definitions {
                if seen[d.ID] {
                        continue
                }
                seen[d.ID] = true
                out = append(out, clone(d))
        }
        return out
}

// Get pins one definition by id and version. Older versions remain immutable
// and resolvable for existing connections; they are never offered as new.
func Get(id, version string) (domain.ConnectorDefinition, bool) {
        for _, d := range definitions {
                if d.ID == id && d.Version == version {
                        return clone(d), true
                }
        }
        return domain.ConnectorDefinition{}, false
}

// Latest returns the newest published version of a connector and reports
// whether the requested version is that current version. New Connections pin
// the latest contract; older pins are refused with an honest reason.
func Latest(id, requestedVersion string) (domain.ConnectorDefinition, bool, bool) {
        var newest domain.ConnectorDefinition
        found := false
        for _, d := range definitions {
                if d.ID != id {
                        continue
                }
                if !found || compareVersions(d.Version, newest.Version) > 0 {
                        newest = d
                        found = true
                }
        }
        if !found {
                return domain.ConnectorDefinition{}, false, false
        }
        return clone(newest), requestedVersion == newest.Version, true
}

// compareVersions orders simple dotted semver-ish strings ("1.10.0" > "1.9.0").
func compareVersions(a, b string) int {
        partsA, partsB := splitVersion(a), splitVersion(b)
        for i := 0; i < 3; i++ {
                if partsA[i] != partsB[i] {
                        if partsA[i] > partsB[i] {
                                return 1
                        }
                        return -1
                }
        }
        return 0
}

func splitVersion(v string) [3]int {
        var out [3]int
        current, index := 0, 0
        for _, r := range v {
                if r == '.' {
                        if index < 2 {
                                out[index] = current
                                index++
                                current = 0
                        }
                        continue
                }
                if r >= '0' && r <= '9' {
                        current = current*10 + int(r-'0')
                }
        }
        out[index] = current
        return out
}

func clone(d domain.ConnectorDefinition) domain.ConnectorDefinition {
        d.AllowedEnvironments = append([]domain.Environment(nil), d.AllowedEnvironments...)
        d.Fields = append([]domain.FieldSchema(nil), d.Fields...)
        d.Capabilities = append([]domain.CapabilityDeclaration(nil), d.Capabilities...)
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
