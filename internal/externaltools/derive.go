package externaltools

// The derivation of the dictionary from the facts (Story 7-4 step-02): a
// PURE function — one section per connection of the class, the entries
// exactly while the query.tools-list report carries the APPLICABLE
// evidence (the report state of the connector machine already encodes the
// anchors: the rotation, the config save, the build change and the TTL
// read as stale — the entries disappear, the repeated probe returns
// them), and the projection decides the admission of every descriptor.
// There is no second record of states and no mutation: a dictionary read
// never writes anything.

import (
	"encoding/json"
	"fmt"
	"strings"

	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
)

// The capability row the dictionary consumes (the wiring-level pin of the
// adapter — the query.tools-list probe is the inventory source).
const toolsListCapability = "query.tools-list"

// Derive computes the sections of the tenant dictionary from the
// per-connection inventories. The order is the caller's facts order (the
// production store lists by the update recency); the entries of one
// section keep the inventory order of the external snapshot.
func Derive(inventories []ConnectionInventory) []Section {
	sections := make([]Section, 0, len(inventories))
	for _, inventory := range inventories {
		section := Section{
			ConnectionID:    inventory.ConnectionID,
			ConnectorID:     inventory.ConnectorID,
			ContractVersion: inventory.ContractVersion,
			Environment:     inventory.Environment,
			Status:          inventory.Status,
		}
		// The connection guard: the honest words of the connector machine —
		// a draft, failed or revoked connection of the class carries no
		// entries and never a false success.
		if inventory.Status != connectorDomain.StatusConnectedUnverified {
			section.Reason = sectionGuardReason(inventory.Status)
			sections = append(sections, section)
			continue
		}
		if inventory.Report.ID != toolsListCapability {
			// The adapter pins the row; a foreign report never feeds the
			// dictionary (the honest empty section of a wiring drift).
			section.Reason = "Отчёт capabilities словаря не получен для этого подключения."
			sections = append(sections, section)
			continue
		}
		switch inventory.Report.State {
		case connectorDomain.CapStateVerified:
			if inventory.Report.Evidence == nil {
				section.Reason = "Возможности не проверены: запустите проверку «Словарь инструментов сервера» на странице подключения."
				sections = append(sections, section)
				continue
			}
			snapshot, refusal := parseInventory(inventory.Report.Evidence.RedactedResult)
			if refusal != "" {
				section.Reason = refusal
				sections = append(sections, section)
				continue
			}
			section.HasEntries = true
			section.Entries = make([]Entry, 0, len(snapshot.Tools))
			for _, tool := range snapshot.Tools {
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
					entry.Provenance = nil
					entry.Input, entry.Output = nil, nil
				} else {
					entry.Input, entry.Output = input, output
				}
				section.Entries = append(section.Entries, entry)
			}
		case connectorDomain.CapStateStale:
			section.Reason = staleSectionReason(inventory.Report.StateReason)
		case connectorDomain.CapStateFailed:
			section.Reason = "Проверка словаря инструментов не прошла: возможности не подтверждены для этого аккаунта. Повторите проверку на странице подключения."
		case connectorDomain.CapStateUnknown:
			section.Reason = "Проверка словаря инструментов не завершилась до бюджета: результат неопределён. Повторите проверку осознанно на странице подключения."
		default:
			section.Reason = "Возможности не проверены: запустите проверку «Словарь инструментов сервера» на странице подключения."
		}
		sections = append(sections, section)
	}
	return sections
}

// parseInventory extracts the compact JSON snapshot from the redacted
// result of the verified evidence (the marker contract of the probe —
// pinned by the tests of both packages).
func parseInventory(redacted string) (projectedInventory, string) {
	marker := strings.LastIndex(redacted, inventoryJSONMarker)
	if marker < 0 {
		return projectedInventory{}, "Инвентарь в доказательстве отсутствует: повторите проверку словаря инструментов (query.tools-list) на странице подключения."
	}
	raw := strings.TrimSpace(redacted[marker+len(inventoryJSONMarker):])
	var snapshot projectedInventory
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return projectedInventory{}, "Инвентарь доказательства не читается: повторите проверку словаря инструментов (query.tools-list) на странице подключения."
	}
	if snapshot.Tools == nil {
		return projectedInventory{}, "Инвентарь доказательства не несёт массива инструментов: повторите проверку словаря инструментов (query.tools-list)."
	}
	return snapshot, ""
}

// inventoryJSONMarker mirrors the probe marker (the dictionary contract —
// a change of the form is a conscious step in BOTH packages, held by the
// tests).
const inventoryJSONMarker = "\nИнвентарь: "

// sectionGuardReason speaks the honest connection words.
func sectionGuardReason(status string) string {
	switch status {
	case connectorDomain.StatusDraft:
		return "Подключение в черновике: базовая проверка не выполнялась — словарь инструментов недоступен до подключения секрета."
	case connectorDomain.StatusFailed:
		return "Базовая проверка не прошла: словарь инструментов недоступен до успешного подключения секрета."
	case connectorDomain.StatusRevoked:
		return "Подключение отозвано: словарь инструментов недоступен; история доказательств сохранена на странице подключения."
	}
	return "Состояние подключения не допускает чтение словаря инструментов."
}

// staleSectionReason carries the stale anchor words of the connector
// machine (the rotation, the config save, the build change or the TTL —
// the entries disappeared, the repeated probe returns them).
func staleSectionReason(reportReason string) string {
	if strings.TrimSpace(reportReason) == "" {
		return "Доказательство устарело (ротация секрета, изменение конфигурации, смена сборки или истечение срока): записи словаря исчезли до повторной проверки."
	}
	return reportReason
}

// DictionaryAdmittedNames lists the admitted tool names of one section
// (the call machine pin material — the stable order).
func DictionaryAdmittedNames(section Section) []string {
	names := make([]string, 0, len(section.Entries))
	for _, entry := range section.Entries {
		if entry.Admitted() {
			names = append(names, entry.Name)
		}
	}
	return names
}

// sectionOf answers one section by the connection id.
func sectionOf(sections []Section, connectionID string) (Section, bool) {
	for _, section := range sections {
		if fmt.Sprint(section.ConnectionID) == connectionID {
			return section, true
		}
	}
	return Section{}, false
}
