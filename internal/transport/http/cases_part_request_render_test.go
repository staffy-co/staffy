package httptransport

import (
	"bytes"
	"os"
	"strings"
	"testing"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 4.3 part request wire shapes: the operator panel rendering
// (the mode with the basis word, the attributes with the provenance, the
// number checks with the honest flag words, the missing-key questions,
// the boundaries as the role=alert TEXT — never only color), the honest
// degradation word and the OpenAPI contract mirrors of the dictionaries.
// ---------------------------------------------------------------------------

// partPanelSurface builds the Story 4.3 panel answer of the wire tests:
// the diagnostic mode under the symptom with the named safety part (the
// candidate word, the safety stop, the handoff recommendations) and the
// oem number with the homoglyph flag.
func partPanelSurface() casesAPI.PartPanelView {
	oem := casesDomain.NormalizePartNumber(" wf 2b19 6k666 aa ")
	part := casesDomain.PartRequestView{
		Mode:              casesDomain.PartModeDiagnostic,
		ModeLabel:         "Диагностика по симптому",
		ModeBasisWord:     "основание: записан симптом",
		NumberMatchWord:   casesDomain.PartNumberMatchIsNotFitmentText,
		PartRequestDigest: "b1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		ConfidenceWord:    "материал подтверждён частично: подтверждены 1 из 1 обязательных ключей выбранного режима",
		Attributes: []casesDomain.PartAttributeView{
			{
				FactID:      id.MustParse("01994f64-6c00-7000-8000-00000000ee10"),
				MaterialKey: "part.symptom", Label: "Симптом", Role: casesDomain.PartRoleSymptom, Kind: "text",
				RawValue: "грохот при торможении на кочках", FactVersion: 1, Status: casesDomain.FactStatusConfirmed,
				SourceType: casesDomain.FactSourceChannelEvent, Channel: "text",
				Confirmation: &casesDomain.PartConfirmationView{ConfirmerType: "client", VersionNo: 1},
			},
			{
				FactID:      id.MustParse("01994f64-6c00-7000-8000-00000000ee11"),
				MaterialKey: "part.kind", Label: "Деталь", Role: casesDomain.PartRolePartKind, Kind: "text",
				RawValue: "тормозные колодки", FactVersion: 2, Status: casesDomain.FactStatusUnverified,
				SourceType: casesDomain.FactSourceOperator, Channel: "operator",
				Confirmation: &casesDomain.PartConfirmationView{ConfirmerType: casesDomain.ConfirmerOperatorRelay, VersionNo: 1, StaleWord: "подтверждение относилось к прежней версии (v1)"},
			},
			{
				FactID:      id.MustParse("01994f64-6c00-7000-8000-00000000ee12"),
				MaterialKey: "part.oem_number", Label: "OEM-номер", Role: casesDomain.PartRoleOemNumber, Kind: "text",
				RawValue: " wf 2b19 6k666 aa ", FactVersion: 1, Status: casesDomain.FactStatusUnverified,
				SourceType: casesDomain.FactSourceChannelEvent, Channel: "text",
				NumberChecks: &oem,
			},
		},
		Completeness: casesDomain.PartCompleteness{
			CompleteWord: "материал запроса полон по обязательному набору выбранного режима",
		},
		Questions: map[string]string{},
		Boundaries: []casesDomain.PartBoundaryView{
			{Kind: "diagnostic", Text: casesDomain.PartRequestDiagnosticBoundaryText},
			{Kind: "diagnostic", Text: casesDomain.PartRequestCandidateUnderSymptomText},
			{Kind: "safety", Text: casesDomain.PartRequestSafetyNoteText + " (маркеры: тормоз)"},
			{Kind: "safety", Text: casesDomain.PartRequestSafetyHeuristicText},
			{Kind: "safety", Text: casesDomain.PartRequestSafetyStopText},
		},
		HandoffWords:     []string{casesDomain.HandoffReasonDiagnosticUncertainty, casesDomain.HandoffReasonSafetyCritical},
		SeveralPartsWord: casesDomain.PartRequestSeveralPartsText,
		StaleNote:        casesDomain.PartRequestStaleNote,
	}
	return casesAPI.PartPanelView{SchemaDefined: true, Part: part}
}

// TestPartRequestPanelRendersOperatorCard pins the operator panel: the
// mode with the basis word, the attributes with the versions and the
// statuses, the number checks with the honest flag words, the
// number-match constant word, the confidence word, the digest and the
// handoff recommendations with the dictionary texts.
func TestPartRequestPanelRendersOperatorCard(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	panelView := preparePartPanel(casesAPI.CaseView{PartRequest: partPanelSurface()})
	if !panelView.Present {
		t.Fatal("the prepared panel is present for the schema-defined case")
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", PartRequest: panelView}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the part request panel: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"Запрос детали",
		"Диагностика по симптому (diagnostic)",
		"основание: записан симптом",
		"грохот при торможении на кочках",
		"тормозные колодки",
		"подтверждение относилось к прежней версии (v1)",
		"WF2B196K666AA",
		casesDomain.PartNumberMatchIsNotFitmentText,
		"материал подтверждён частично",
		"b1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		"Рекомендация передачи человеку",
		"diagnostic_uncertainty — " + casesDomain.HandoffReasonText(casesDomain.HandoffReasonDiagnosticUncertainty),
		"safety_critical — " + casesDomain.HandoffReasonText(casesDomain.HandoffReasonSafetyCritical),
		casesDomain.PartRequestSeveralPartsText,
		casesDomain.PartRequestStaleNote,
	} {
		if !strings.Contains(rendered, pin) {
			t.Errorf("the operator part request panel must carry %q", pin)
		}
	}
}

// TestPartRequestBoundariesAreTextNotColor pins the AC-13 probe: every
// honesty boundary renders as TEXT inside a role=alert element — the
// diagnostic uncertainty is never carried by color alone.
func TestPartRequestBoundariesAreTextNotColor(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	panelView := preparePartPanel(casesAPI.CaseView{PartRequest: partPanelSurface()})
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", PartRequest: panelView}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		`role="alert"`,
		"Граница (diagnostic):",
		casesDomain.PartRequestDiagnosticBoundaryText,
		"Граница (safety):",
		casesDomain.PartRequestSafetyStopText,
	} {
		if !strings.Contains(rendered, pin) {
			t.Errorf("the boundaries must render as the alert TEXT %q", pin)
		}
	}
	// The boundary label and the boundary text live in the SAME alert
	// paragraph (the label is the strong head of the row).
	if !strings.Contains(rendered, "<strong>Граница (diagnostic):</strong> "+casesDomain.PartRequestDiagnosticBoundaryText) {
		t.Fatal("the diagnostic boundary label and text must render in one alert paragraph")
	}
	if strings.Count(rendered, `role="alert"`) < 5 {
		t.Fatalf("every boundary row carries its own alert role, got %d", strings.Count(rendered, `role="alert"`))
	}
}

// TestPartRequestPanelAbsenceWordRenders pins the honest degradation: a
// case whose pinned pack carries no part request schema renders the
// absence word — never a hardcoded key set.
func TestPartRequestPanelAbsenceWordRenders(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	panelView := preparePartPanel(casesAPI.CaseView{PartRequest: casesAPI.PartPanelView{Note: casesAPI.PartSchemaNoteAbsent}})
	if panelView.Present {
		t.Fatal("the panel is absent without the schema")
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", PartRequest: panelView}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render without the schema: %v", err)
	}
	rendered := page.String()
	if !strings.Contains(rendered, "Схема запроса детали пака не определена") {
		t.Fatal("the absence word must render honestly")
	}
	// The digest RENDER line (the material of the request) never appears
	// without the schema — the intro word of the section header is not
	// the panel material.
	if strings.Contains(rendered, "Точная версия запроса (PartRequestDigest):") || strings.Contains(rendered, "Режим запроса:") {
		t.Fatal("the absent schema renders no request material")
	}
}

// TestPartRequestUnstructuredDictionaryRenders pins the honest empty
// state: the unstructured word with the schema dictionary hint (the
// operator records the material by the pack dictionary — never a guess).
func TestPartRequestUnstructuredDictionaryRenders(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	dictionary := make([]casesDomain.PartMaterialKeyDomain, 0, 10)
	for _, key := range []casesDomain.PartMaterialKeyDomain{
		{Key: "part.kind", Label: "Деталь", Kind: "text", Role: casesDomain.PartRolePartKind},
		{Key: "part.side", Label: "Сторона", Kind: "text", Optional: true},
		{Key: "part.quantity", Label: "Количество", Kind: "integer"},
		{Key: "part.preference", Label: "Предпочтение", Kind: "enum", Values: []string{"original", "analogs", "both"}, Role: casesDomain.PartRolePreference},
		{Key: "part.oem_number", Label: "OEM-номер", Kind: "text", Role: casesDomain.PartRoleOemNumber},
		{Key: "part.symptom", Label: "Симптом", Kind: "text", Role: casesDomain.PartRoleSymptom},
		{Key: "part.budget", Label: "Бюджет", Kind: "text", Optional: true},
		{Key: "part.deadline", Label: "Срок", Kind: "text", Optional: true},
		{Key: "part.brands", Label: "Приемлемые бренды", Kind: "text", Optional: true},
		{Key: "part.constraints", Label: "Ограничения", Kind: "text", Optional: true},
	} {
		dictionary = append(dictionary, key)
	}
	part := casesDomain.PartRequestView{
		UnstructuredWord: casesDomain.PartRequestUnstructuredWord,
		SchemaDictionary: dictionary,
		ConfidenceWord:   "материал без подтверждения",
		Attributes:       []casesDomain.PartAttributeView{},
		Boundaries:       []casesDomain.PartBoundaryView{},
		HandoffWords:     []string{},
		Questions:        map[string]string{},
	}
	panelView := preparePartPanel(casesAPI.CaseView{PartRequest: casesAPI.PartPanelView{SchemaDefined: true, Part: part}})
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", PartRequest: panelView}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the empty material: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"запрос не структурирован",
		"Словарь материал-ключей схемы пака",
		"part.oem_number",
		"Приемлемые бренды",
		">симптом<",
		"необязательный",
	} {
		if !strings.Contains(rendered, pin) {
			t.Errorf("the empty material must carry the dictionary hint %q", pin)
		}
	}
}

// TestPartRequestOpenAPIContractMirrorsDictionaries pins the enum mirrors
// of the Story 4.3 OpenAPI contract: the handoff reason dictionary (the
// 00019 SQL mirror), the mode dictionary, the part material keys, the
// preference values, the boundary kinds and the roles (drift = red).
func TestPartRequestOpenAPIContractMirrorsDictionaries(t *testing.T) {
	contract, err := os.ReadFile("../../../contracts/openapi/staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(contract)
	for _, pin := range []string{
		"PartRequestPanelView", "PartRequestView", "PartAttributeView", "PartNumberNormalizationView", "PartBoundaryView", "PartMaterialKeyView", "PartCompletenessView",
		"enum: [diagnostic_uncertainty, safety_critical, unresolved_contradiction]",
		"enum: [diagnostic, exact_repeat, original_vs_analog, named_part]",
		"enum: [part.kind, part.side, part.quantity, part.preference, part.oem_number, part.symptom, part.budget, part.deadline, part.brands, part.constraints]",
		"enum: [original, analogs, both]",
		"enum: [part_kind, symptom, oem_number, preference]",
		"enum: [diagnostic, safety, contradiction]",
		"enum: [confirmed, unverified, conflicting]",
		"миграции cases/00019",
	} {
		if !strings.Contains(text, pin) {
			t.Errorf("the contract must mirror %q", pin)
		}
	}
	// The material key enum mirror must appear in EVERY key-bearing
	// schema (the attribute view, the completeness view and the
	// dictionary view — the drift of any one is red).
	if strings.Count(text, "enum: [part.kind, part.side, part.quantity, part.preference, part.oem_number, part.symptom, part.budget, part.deadline, part.brands, part.constraints]") != 3 {
		t.Fatalf("the part material key enum mirror must live in the three key-bearing schemas, got %d", strings.Count(text, "enum: [part.kind, part.side, part.quantity, part.preference, part.oem_number, part.symptom, part.budget, part.deadline, part.brands, part.constraints]"))
	}
}

// TestPartRequestFormReasonOptionsRender pins the Story 4.3 reason select
// of the existing request_human form: the three operator boundary words
// with their honest texts and the empty unconditional choice (the
// client_request word is NEVER an option — the system truth).
func TestPartRequestFormReasonOptionsRender(t *testing.T) {
	options := []workOption{}
	for _, reason := range casesDomain.HandoffReasons {
		if reason == casesDomain.HandoffReasonClientRequest {
			continue
		}
		options = append(options, workOption{Value: reason, Text: casesDomain.HandoffReasonText(reason)})
	}
	if len(options) != 3 {
		t.Fatalf("the three operator boundary words expected, got %d", len(options))
	}
	for _, option := range options {
		if option.Text == "" {
			t.Fatalf("the reason word %q must carry its honest text", option.Value)
		}
	}
	if strings.Contains(strings.Join([]string{options[0].Value, options[1].Value, options[2].Value}, ","), casesDomain.HandoffReasonClientRequest) {
		t.Fatal("the client_request word is never a form option — the system truth")
	}
}
