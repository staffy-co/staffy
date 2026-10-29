package httptransport

import (
	"strconv"
	"strings"

	connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// fieldView is one rendered form field with a stable, unique element id.
type fieldView struct {
	Name        string
	Label       string
	Description string
	Value       string
	Required    bool
	ElementID   string
}

// connectorCard is a catalog card prepared for the integrations page.
type connectorCard struct {
	ID                  string
	Version             string
	Name                string
	Purpose             string
	SupportStatus       string
	PriorityNote        string
	AllowedEnvironments string
	RequiredAccountType string
	DocumentationURL    string
	Connectable         bool
	ConfigFields        []fieldView
	SecretFields        []fieldView
}

// connectionView is one tenant connection prepared for the page.
type connectionView struct {
	ConnectionID  id.UUID
	Label         string
	ConnectorID   string
	ConnectorVer  string
	Environment   string
	Status        string
	AuthState     string
	Transport     string
	SecretHint    string
	SecretActive  bool
	Version       int64
	ConfigFields  []fieldView
	SecretFields  []fieldView
	HasSecretForm bool
	// ProbeSecret marks the external MCP class (Story 7-4): its verify
	// forms carry the probe key — the MAC-checked material of the live
	// rounds, never stored again.
	ProbeSecret  bool
	Capabilities []capabilityView
	FormKey      string
}

// capabilityView is one derived capability report prepared for the card.
type capabilityView struct {
	ID            string
	Title         string
	State         string
	StateText     string
	Reason        string
	Meta          string
	CanVerify     bool
	VerifyKey     string
	BlockRequired string
	BlockGate     string
	BlockOwner    string
	Evidence      *evidenceView
}

type evidenceView struct {
	CheckedAtText   string
	ExpiresAtText   string
	DeliveryLevel   string
	Outcome         string
	RedactedRequest string
	RedactedResult  string
}

// prepareCapabilityViews projects the derived reports into template values.
// The verify button appears only for probe-verifiable capabilities.
func prepareCapabilityViews(state connectorAPI.CapabilityStateResult, verifyKey string) []capabilityView {
	out := make([]capabilityView, 0, len(state.Capabilities))
	for _, report := range state.Capabilities {
		view := capabilityView{
			ID: report.ID, Title: report.Title,
			State: report.State, StateText: capabilityStateText(report.State),
			Reason:    report.StateReason,
			Meta:      capabilityMetaText(report),
			CanVerify: report.CanVerify, VerifyKey: verifyKey,
		}
		if report.Block != nil {
			view.BlockRequired = report.Block.Required
			view.BlockGate = report.Block.Gate
			view.BlockOwner = report.Block.Owner
		}
		if report.Evidence != nil {
			view.Evidence = &evidenceView{
				CheckedAtText:   report.Evidence.CheckedAt.Format("02.01.2006 15:04 UTC"),
				ExpiresAtText:   report.Evidence.ExpiresAt.Format("02.01.2006 15:04 UTC"),
				DeliveryLevel:   report.Evidence.DeliveryLevel,
				Outcome:         report.Evidence.Outcome,
				RedactedRequest: report.Evidence.RedactedRequest,
				RedactedResult:  report.Evidence.RedactedResult,
			}
		}
		out = append(out, view)
	}
	return out
}

// capabilityStateText renders the machine state as readable Russian so a
// state never reads as success when it is not.
func capabilityStateText(state string) string {
	switch state {
	case connectorDomain.CapStateUnverified:
		return "не проверено"
	case connectorDomain.CapStateVerifying:
		return "проверяется"
	case connectorDomain.CapStateVerified:
		return "проверено"
	case connectorDomain.CapStateUnsupported:
		return "не поддерживается"
	case connectorDomain.CapStateFailed:
		return "проверка не прошла"
	case connectorDomain.CapStateUnknown:
		return "результат неопределён"
	case connectorDomain.CapStateStale:
		return "устарело"
	}
	return state
}

// capabilityMetaText renders the declared operational semantics compactly.
func capabilityMetaText(r connectorDomain.CapabilityReport) string {
	direction := "исходящая"
	if r.Direction == connectorDomain.DirectionInbound {
		direction = "входящая"
	}
	operation := string(r.Operation)
	return direction + " · " + operation + " · таймаут " + strconv.Itoa(r.TimeoutMS) + " мс · лимиты: " + r.RateLimit + ", " + r.SizeLimit + " · receipt: " + r.Receipt
}

type connectorTypeGroupView struct {
	Title string
	Cards []connectorCard
}

var connectorTypeTitles = map[connectorDomain.ConnectorType]string{
	connectorDomain.TypeSIP:      "Телефония (SIP)",
	connectorDomain.TypeChat:     "Чат и входящие",
	connectorDomain.TypeBusiness: "Бизнес-системы",
	connectorDomain.TypeAISpeech: "ИИ и речь",
	connectorDomain.TypeStorage:  "Хранилища и вспомогательные сервисы",
}

func prepareConnectorGroups(defs []connectorDomain.ConnectorDefinition) []connectorTypeGroupView {
	order := []connectorDomain.ConnectorType{connectorDomain.TypeSIP, connectorDomain.TypeChat, connectorDomain.TypeBusiness, connectorDomain.TypeAISpeech, connectorDomain.TypeStorage}
	groups := []connectorTypeGroupView{}
	for _, t := range order {
		cards := []connectorCard{}
		for _, def := range defs {
			if def.Type != t {
				continue
			}
			card := connectorCard{
				ID: def.ID, Version: def.Version, Name: def.Name, Purpose: def.Purpose,
				SupportStatus: string(def.SupportStatus), RequiredAccountType: def.RequiredAccountType,
				DocumentationURL: def.DocumentationURL, Connectable: def.Connectable(),
			}
			if def.AdapterImplemented && def.PriorityNote != "" {
				card.PriorityNote = def.PriorityNote
			}
			card.AllowedEnvironments = strings.Join(environmentsText(def.AllowedEnvironments), ", ")
			card.ConfigFields = fieldViews("c-"+def.ID, def.ConfigSchema(), nil)
			card.SecretFields = fieldViews("s-"+def.ID, def.SecretSchema(), nil)
			cards = append(cards, card)
		}
		if len(cards) > 0 {
			groups = append(groups, connectorTypeGroupView{Title: connectorTypeTitles[t], Cards: cards})
		}
	}
	return groups
}

func prepareConnectionViews(connections []connectorDomain.Connection, defs []connectorDomain.ConnectorDefinition) []connectionView {
	byID := map[string]connectorDomain.ConnectorDefinition{}
	for _, def := range defs {
		byID[def.ID+"\x00"+def.Version] = def
	}
	views := make([]connectionView, 0, len(connections))
	for _, c := range connections {
		view := connectionView{
			ConnectionID: c.ConnectionID, Label: c.Label, ConnectorID: c.ConnectorID, ConnectorVer: c.ConnectorVer,
			Environment: string(c.Environment), Status: c.Status, AuthState: c.AuthState, Transport: c.Transport,
			SecretHint: c.SecretHint, SecretActive: c.SecretActive, Version: c.Version,
		}
		prefix := "conn-" + c.ConnectionID.String()
		def, ok := byID[c.ConnectorID+"\x00"+c.ConnectorVer]
		if ok {
			values := map[string]string{}
			for k, v := range c.Config {
				values[k] = v
			}
			view.ConfigFields = fieldViews(prefix, def.ConfigSchema(), values)
			view.SecretFields = fieldViews(prefix, def.SecretSchema(), nil)
			view.HasSecretForm = len(view.SecretFields) > 0
			view.ProbeSecret = def.RequiredAccountType == "external-mcp"
		}
		views = append(views, view)
	}
	return views
}

func fieldViews(prefix string, schema []connectorDomain.FieldSchema, values map[string]string) []fieldView {
	out := make([]fieldView, 0, len(schema))
	for _, f := range schema {
		view := fieldView{Name: f.Name, Label: f.Label, Description: f.Description, Required: f.Required, ElementID: prefix + "-" + f.Name}
		if values != nil {
			if v, ok := values[f.Name]; ok {
				view.Value = v
			}
		}
		out = append(out, view)
	}
	return out
}

func environmentsText(envs []connectorDomain.Environment) []string {
	out := make([]string, 0, len(envs))
	for _, e := range envs {
		out = append(out, string(e))
	}
	return out
}

// connectionStatusText renders the honest machine status as readable text with
// an explanation so states never read as success when they are not. A failed
// check carries the concrete safe next step (published sandbox formats), not a
// generic apology.
func connectionStatusText(status, auth, transport, environment string) string {
	switch status {
	case connectorDomain.StatusDraft:
		return "Черновик: секрет ещё не проверялся"
	case connectorDomain.StatusConnectedUnverified:
		return "Подключено, но возможности не проверены (connected/unverified): успешная проверка аутентификации и транспорта не подтверждает отправку, приём и бизнес-операции"
	case connectorDomain.StatusFailed:
		return "Проверка не прошла: " + failedCheckText(auth, transport, environment)
	case connectorDomain.StatusRevoked:
		return "Отозвано"
	}
	return status
}

// failedCheckText explains each failed part with the published synthetic rule
// so the user sees the next step without echoing any submitted value.
func failedCheckText(auth, transport, environment string) string {
	suffix := "sandbox.example"
	if environment == string(connectorDomain.EnvTest) {
		suffix = "test.example"
	}
	authPart := checkText(auth, "аутентификация")
	if auth == connectorDomain.CheckFailed {
		authPart = "аутентификация не пройдена — используйте тестовый ключ формата sandbox-<8-64 символа>"
	}
	transportPart := checkText(transport, "транспорт")
	if transport == connectorDomain.CheckFailed {
		transportPart = "транспорт не пройден — endpoint должен быть вида https://<имя>." + suffix
	}
	return authPart + ", " + transportPart
}

func checkText(state, label string) string {
	switch state {
	case connectorDomain.CheckVerified:
		return label + " пройдена"
	case connectorDomain.CheckFailed:
		return label + " не пройдена"
	default:
		return label + " ожидает"
	}
}
