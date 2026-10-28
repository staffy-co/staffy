package externaltools

// The credential- and target-agnostic call machine (Story 7-4 step-02):
// the FORM under the future Planner 8.3 (the 7-3 call-path precedent).
// The entry pin comes from the CURRENT derivation (the honest TOCTOU —
// a rotation, a config save, a build change or the TTL expiry makes the
// pin vanish before the round), the arguments are validated strictly
// against the projected input dictionary (every refusal names the field),
// the round runs through the injected transport with the caller's
// explicit endpoint and Authorization material (the machine never stores,
// logs or echoes them), and the answer is validated fail-closed against
// the projected output dictionary — an undeclared key of a misbehaving
// external server never travels to the caller (AD-11). No HTTP surface of
// the call exists in this slice: the machine is proven by the Go contour
// over the REAL wire (httptest) and the fixture world.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/artemiosu/staffy/internal/tools"
)

// The honest refusal identities of the call machine.
var (
	// ErrEntryNotFound: the connection or the tool name does not resolve
	// against the CURRENT derivation.
	ErrEntryNotFound = errors.New("externaltools: запись словаря не найдена")
	// ErrEntryNotAdmitted: the row exists but the projection refused it —
	// the named reason travels.
	ErrEntryNotAdmitted = errors.New("externaltools: запись словаря не допущена")
	// ErrArguments: the strict validation of the arguments refused — the
	// named field refusals travel (SEP-1303: the model self-corrects).
	ErrArguments = errors.New("externaltools: аргументы вызова не прошли валидацию")
	// ErrOutputRefused: the fail-closed output — the answer of the external
	// server carries undeclared or mistyped material; the internal material
	// never travels to the caller (AD-11).
	ErrOutputRefused = errors.New("externaltools: выход вызова отклонён")
)

// Calls is the call machine over the live facts and the round transport.
type Calls struct {
	facts     InventoryFactsSource
	transport RoundTransport
}

// NewCalls wires the facts source and the round transport (the composition
// root injects the mcpclient-backed transport of the production wire or
// the scripted transport of the test contour).
func NewCalls(facts InventoryFactsSource, transport RoundTransport) *Calls {
	return &Calls{facts: facts, transport: transport}
}

// Call executes one admitted tool of one live connection of the class.
// The scope is the caller's own verified authority; the endpoint and the
// Authorization material are the caller's explicit parameters.
func (c *Calls) Call(ctx context.Context, scope tenancy.Context, connectionID id.UUID, toolName string, arguments map[string]any, endpoint, authorization string) (map[string]any, error) {
	// The pin from the CURRENT derivation: the honest TOCTOU — the
	// derivation is recomputed, a vanished row refuses by name.
	entry, refusal := c.pinEntry(ctx, scope, connectionID, toolName)
	if refusal != "" {
		return nil, fmt.Errorf("%w: %s", ErrEntryNotFound, refusal)
	}
	if !entry.Admitted() {
		return nil, fmt.Errorf("%w: %s", ErrEntryNotAdmitted, entry.Reason)
	}
	// The strict validation of the arguments against the projected input
	// dictionary: an unknown key, a mistyped value, a value outside the
	// enum or the bounds, a missing required field — every refusal names
	// the field, and the round never happens.
	if refusals := validateValues("аргумент", entry.Input, arguments); len(refusals) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrArguments, strings.Join(refusals, "; "))
	}
	// The authenticated round with the caller's material: the machine
	// passes the parameters through and never interprets them.
	answer, err := c.transport.CallTool(ctx, endpoint, authorization, toolName, arguments)
	if err != nil {
		return nil, err
	}
	// The fail-closed validation of the output: the answer must carry
	// exactly the declared keys with the declared types — an undeclared key
	// never travels to the caller (AD-11).
	if refusals := validateValues("выход", entry.Output, answer); len(refusals) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrOutputRefused, strings.Join(refusals, "; ")+"; выход не переслан")
	}
	return answer, nil
}

// pinEntry recomputes the CURRENT derivation and answers one admitted
// candidate row (the honest TOCTOU: the rotation, the config save, the
// build change and the TTL expiry make the row vanish before the round).
func (c *Calls) pinEntry(ctx context.Context, scope tenancy.Context, connectionID id.UUID, toolName string) (Entry, string) {
	if !scope.Valid() || !connectionID.Valid() || strings.TrimSpace(toolName) == "" {
		return Entry{}, "идентификатор вызова неполон"
	}
	inventories, err := c.facts.InventoryFacts(ctx, scope)
	if err != nil {
		return Entry{}, "факты словаря не прочитаны"
	}
	for _, inventory := range inventories {
		if inventory.ConnectionID != connectionID {
			continue
		}
		if inventory.Status != connectorDomain.StatusConnectedUnverified || inventory.Report.State != connectorDomain.CapStateVerified || inventory.Report.Evidence == nil {
			return Entry{}, "подключение не несёт применимого доказательства словаря: записи исчезли до вызова (ротация, конфигурация, сборка или TTL) — повторите проверку"
		}
		snapshot, refusal := parseInventory(inventory.Report.Evidence.RedactedResult)
		if refusal != "" {
			return Entry{}, refusal
		}
		for _, tool := range snapshot.Tools {
			if tool.Name != toolName {
				continue
			}
			entry := Entry{
				Name:        tool.Name,
				Title:       tool.Title,
				Description: tool.Description,
				State:       EntryAdmitted,
				Provenance: &Provenance{
					ConnectorID:      inventory.ConnectorID,
					ConnectorVersion: inventory.ContractVersion,
					ServerName:       snapshot.Server.Name,
					ServerVersion:    snapshot.Server.Version,
					EvidenceID:       inventory.Report.Evidence.EvidenceID,
					CheckedAt:        inventory.Report.Evidence.CheckedAt,
					ExpiresAt:        inventory.Report.Evidence.ExpiresAt,
				},
			}
			input, output, projectRefusal := projectTool(tool)
			if projectRefusal != "" {
				entry.State = EntryRejected
				entry.Reason = projectRefusal
				return entry, ""
			}
			entry.Input, entry.Output = input, output
			return entry, ""
		}
		return Entry{}, fmt.Sprintf("инструмент %s отсутствует в текущем снимке словаря подключения", toolName)
	}
	return Entry{}, "подключение класса не найдено в фактах тенента"
}

// validateValues checks one value map against the projected dictionary:
// every key must be declared, every value must carry the declared type,
// the enum values and the integer bounds must hold and every required
// field must be present. Every refusal NAMES the field.
func validateValues(side string, schema map[string]tools.ToolSchemaField, values map[string]any) []string {
	var refusals []string
	for name := range values {
		if _, ok := schema[name]; !ok {
			refusals = append(refusals, side+": ключ "+name+" не объявлен словарём записи")
		}
	}
	for _, name := range sortedNames(schema) {
		field := schema[name]
		value, ok := values[name]
		if !ok {
			if !field.Optional {
				refusals = append(refusals, side+": обязательное поле "+name+" отсутствует")
			}
			continue
		}
		switch field.Type {
		case "string":
			if _, ok := value.(string); !ok {
				refusals = append(refusals, side+": поле "+name+" обязано быть строкой")
				continue
			}
			if len(field.Values) > 0 && !containsValue(field.Values, value.(string)) {
				refusals = append(refusals, side+": поле "+name+" вне перечисления ("+strings.Join(field.Values, "; ")+")")
			}
		case "integer":
			number, ok := value.(float64)
			if !ok || number != float64(int(number)) {
				refusals = append(refusals, side+": поле "+name+" обязано быть целым числом")
				continue
			}
			whole := int(number)
			if field.Min != 0 && whole < field.Min {
				refusals = append(refusals, fmt.Sprintf("%s: поле %s ниже границы %d", side, name, field.Min))
			}
			if field.Max != 0 && whole > field.Max {
				refusals = append(refusals, fmt.Sprintf("%s: поле %s выше границы %d", side, name, field.Max))
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				refusals = append(refusals, side+": поле "+name+" обязано быть булевым значением")
			}
		}
	}
	return refusals
}

func containsValue(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
