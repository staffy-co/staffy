package httptransport

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
)

// ---------------------------------------------------------------------------
// The Story 4.4 fitment wire shapes: the operator panel rendering (the
// state words as TEXT — role=alert for the stale words, the whole cross
// chains, the source details without leaving the case), the honest
// degradation word, the record form skeleton and the OpenAPI contract
// mirrors of the dictionaries.
// ---------------------------------------------------------------------------

// fitmentPanelSurface builds the Story 4.4 panel answer of the wire
// tests: one proven original position with the safety word, one
// conflicted pair (the found against the not-found — BOTH records stay
// in the material) and the case-level unknown is absent (the positions
// exist).
func fitmentPanelSurface() casesAPI.FitmentPanelView {
	now := mustTime("2026-10-24T12:00:00Z")
	observed := mustTime("2026-10-24T10:00:00Z")
	proven := casesDomain.FitmentPositionView{
		PositionKey: "WF2B196K666AA",
		Status:      casesDomain.FitmentStatusProven,
		StatusWord:  casesDomain.FitmentProvenWord,
		Words:       []string{casesDomain.PartRequestSafetyNoteText},
		Evidences: []casesDomain.FitmentEvidenceView{
			{
				EvidenceNo: 1, SourceSystem: casesDomain.FitmentSourceAbcpCatalog, SourceRecordID: "rec-1", SourceVersion: "v2026.10",
				SourceObservedAt: observed, ReceivedAt: now, LinkType: casesDomain.FitmentLinkOriginal,
				Chain: []string{"Фильтр салона (оригинальная позиция)"}, CatalogVehicleIDs: []string{"ABCP-12345"},
				CatalogNode: "Фильтр салона", Article: "WF2B196K666AA",
				ProfileDigestPin:     "v111111111111111111111111111111111111111111111111111111111111111v",
				PartRequestDigestPin: "p111111111111111111111111111111111111111111111111111111111111111p",
				Found:                true,
			},
		},
		FitmentDigest: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
	}
	conflicted := casesDomain.FitmentPositionView{
		PositionKey: "C330PR",
		Status:      casesDomain.FitmentStatusConflicted,
		StatusWord:  casesDomain.FitmentConflictedSourcesWord,
		Evidences: []casesDomain.FitmentEvidenceView{
			{
				EvidenceNo: 1, SourceSystem: casesDomain.FitmentSourceAbcpCatalog, SourceRecordID: "rec-2", SourceVersion: "v2026.10",
				SourceObservedAt: observed, ReceivedAt: now, LinkType: casesDomain.FitmentLinkCross,
				Chain:             []string{"Фильтр салона (оригинальная позиция)", "кросс-запись источника", "аналог C330PR"},
				CatalogVehicleIDs: []string{"ABCP-12345"}, CatalogNode: "Фильтр салона", Article: "C330PR",
				ProfileDigestPin:     "v111111111111111111111111111111111111111111111111111111111111111v",
				PartRequestDigestPin: "p111111111111111111111111111111111111111111111111111111111111111p",
				Found:                true,
			},
			{
				EvidenceNo: 2, SourceSystem: casesDomain.FitmentSourceLaximoCatalog, SourceRecordID: "rec-3", SourceVersion: "v3.11",
				SourceObservedAt: observed, ReceivedAt: now, LinkType: casesDomain.FitmentLinkOriginal,
				Chain:                []string{"Фильтр салона (оригинальная позиция)"},
				ProfileDigestPin:     "v111111111111111111111111111111111111111111111111111111111111111v",
				PartRequestDigestPin: "p111111111111111111111111111111111111111111111111111111111111111p",
				Found:                false,
			},
		},
		HandoffWords: []string{casesDomain.HandoffReasonUnresolvedContradiction},
	}
	return casesAPI.FitmentPanelView{
		SchemaDefined: true,
		Fitment: casesDomain.FitmentView{
			Positions:       []casesDomain.FitmentPositionView{proven, conflicted},
			NumberMatchWord: casesDomain.PartNumberMatchIsNotFitmentText,
			PriceNote:       casesDomain.FitmentPriceNote,
			ScopeWord:       casesDomain.FitmentScopeWord,
		},
	}
}

// mustTime parses one RFC 3339 timestamp (the test fixture helper).
func mustTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

// TestFitmentPanelRendersOperatorCard pins the operator panel: the state
// words as TEXT, the whole cross chains, the source record identifiers
// with the versions (the details without leaving the case — AC-14), the
// proven digest, BOTH conflicted records, the constant word, the price
// note and the scope word.
func TestFitmentPanelRendersOperatorCard(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	panelView := prepareFitmentPanel(casesAPI.CaseView{Fitment: fitmentPanelSurface()})
	if !panelView.Present {
		t.Fatal("the prepared panel is present for the schema-defined case")
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", Fitment: panelView}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the fitment panel: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"Применимость",
		"WF2B196K666AA",
		casesDomain.FitmentProvenWord,
		"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		casesDomain.PartRequestSafetyNoteText,
		"rec-1",
		"v2026.10",
		"ABCP-12345",
		"C330PR",
		casesDomain.FitmentConflictedSourcesWord,
		"кросс-запись источника",
		"аналог C330PR",
		"источник исследован, доказательство не найдено",
		"unresolved_contradiction — " + casesDomain.HandoffReasonText(casesDomain.HandoffReasonUnresolvedContradiction),
		casesDomain.PartNumberMatchIsNotFitmentText,
		casesDomain.FitmentPriceNote,
		casesDomain.FitmentScopeWord,
	} {
		if !strings.Contains(rendered, pin) {
			t.Errorf("the operator fitment panel must carry %q", pin)
		}
	}
	// The whole cross chain renders as ONE arrow-separated path (AC-4).
	if !strings.Contains(rendered, "Фильтр салона (оригинальная позиция) → кросс-запись источника → аналог C330PR") {
		t.Fatal("the cross chain must render WHOLE as one arrow-separated path")
	}
}

// TestFitmentStaleWordsAreTextNotColor pins the stale words probe: the
// per-record stale words render as TEXT inside role=alert elements — the
// expiry is never carried by color alone (AC-13/AC-14).
func TestFitmentStaleWordsAreTextNotColor(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	surface := fitmentPanelSurface()
	surface.Fitment.Positions[0].Evidences[0].StaleWords = []string{casesDomain.FitmentStaleAgeWord, casesDomain.FitmentStalePartRequestWord}
	surface.Fitment.Positions[0].Status = casesDomain.FitmentStatusStale
	surface.Fitment.Positions[0].StatusWord = casesDomain.FitmentStaleAgeWord
	surface.Fitment.Positions[0].Words = []string{casesDomain.FitmentStaleRecheckWord}
	panelView := prepareFitmentPanel(casesAPI.CaseView{Fitment: surface})
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", Fitment: panelView}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		`role="alert"`,
		casesDomain.FitmentStaleAgeWord,
		casesDomain.FitmentStalePartRequestWord,
		casesDomain.FitmentStaleRecheckWord,
	} {
		if !strings.Contains(rendered, pin) {
			t.Errorf("the stale words must render as the alert TEXT %q", pin)
		}
	}
	if strings.Count(rendered, `role="alert"`) < 2 {
		t.Fatalf("every stale word row carries its own alert role, got %d", strings.Count(rendered, `role="alert"`))
	}
}

// TestFitmentPanelAbsenceWordRenders pins the honest degradation: a case
// whose pinned pack carries no fitment policy renders the absence word —
// never a hardcoded policy.
func TestFitmentPanelAbsenceWordRenders(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	panelView := prepareFitmentPanel(casesAPI.CaseView{Fitment: casesAPI.FitmentPanelView{Note: casesAPI.FitmentSchemaNoteAbsent}})
	if panelView.Present {
		t.Fatal("the panel is absent without the policy")
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", Fitment: panelView}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render: %v", err)
	}
	rendered := page.String()
	if !strings.Contains(rendered, casesAPI.FitmentSchemaNoteAbsent) {
		t.Fatal("the honest absence word must render")
	}
	if strings.Contains(rendered, `id="fitment"`) {
		t.Fatal("the panel section is absent without the policy — only the absence word renders")
	}
}

// TestFitmentUnknownWordRenders pins the case-level unknown: no evidence
// records at all — the honest unknown word with the research
// recommendation (AC-10: the absence is never a negative or a positive).
func TestFitmentUnknownWordRenders(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	surface := casesAPI.FitmentPanelView{SchemaDefined: true}
	surface.Fitment.CaseUnknownWord = casesDomain.FitmentUnknownWord
	surface.Fitment.CaseHandoffWords = []string{casesDomain.FitmentResearchWord}
	surface.Fitment.NumberMatchWord = casesDomain.PartNumberMatchIsNotFitmentText
	surface.Fitment.PriceNote = casesDomain.FitmentPriceNote
	surface.Fitment.ScopeWord = casesDomain.FitmentScopeWord
	panelView := prepareFitmentPanel(casesAPI.CaseView{Fitment: surface})
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", Fitment: panelView}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		casesDomain.FitmentUnknownWord,
		"fitment_research — " + casesDomain.HandoffReasonText(casesDomain.HandoffReasonFitmentResearch),
	} {
		if !strings.Contains(rendered, pin) {
			t.Errorf("the case-level unknown must carry %q", pin)
		}
	}
}

// TestFitmentRecordFormRenders pins the record form skeleton: the labels,
// the selects of the pack dictionaries (the link types, the sources), the
// chain textarea, the work-item anchored action and the producer word
// (the manager — never an automatic catalog request).
func TestFitmentRecordFormRenders(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	panelView := prepareFitmentPanel(casesAPI.CaseView{Fitment: fitmentPanelSurface()})
	var page bytes.Buffer
	data := map[string]any{
		"TenantID": "01994f64-6c00-7000-8000-000000000002",
		"View": caseDetailPageView{
			CaseID:          "01994f64-6c00-7000-8000-00000000aa01",
			Fitment:         panelView,
			ActiveWIOptions: []workOption{{Value: "01994f64-6c00-7000-8000-00000000ee20", Text: "поиск детали"}},
		},
	}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the record form: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"/app/tenants/01994f64-6c00-7000-8000-000000000002/cases/01994f64-6c00-7000-8000-00000000aa01/work-items/01994f64-6c00-7000-8000-00000000ee20/fitment-evidences",
		`name="position_key"`,
		`name="link_type"`,
		`value="original"`,
		`value="cross"`,
		`name="chain"`,
		`name="catalog_vehicle_ids"`,
		`name="source_system"`,
		`value="abcp_catalog"`,
		`value="laximo_catalog"`,
		`name="source_record_id"`,
		`name="source_version"`,
		`name="source_observed_at"`,
		"автоматический запрос к каталогу НЕ выполняется",
		"Записать доказательство",
	} {
		if !strings.Contains(rendered, pin) {
			t.Errorf("the record form must carry %q", pin)
		}
	}
}

// TestFitmentOpenAPIContractMirrorsDictionaries pins the contract
// mirrors: the fitment state enum (5 words), the link type enum (2), the
// source enum (2), the reason enum (4 operator words — the 00021 mirror)
// and the schema family names.
func TestFitmentOpenAPIContractMirrorsDictionaries(t *testing.T) {
	contract, err := os.ReadFile("../../../contracts/openapi/staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(contract)
	for _, pin := range []string{
		"FitmentPanelView", "FitmentView", "FitmentPositionView", "FitmentEvidenceView", "RecordFitmentEvidenceRequest",
		"enum: [proven, ambiguous, conflicted, unknown, stale]",
		"enum: [original, cross]",
		"enum: [abcp_catalog, laximo_catalog]",
		"enum: [diagnostic_uncertainty, safety_critical, unresolved_contradiction, fitment_research, offers_research]",
		"enum: [fitment_research, unresolved_contradiction]",
		"cases/00021",
	} {
		if !strings.Contains(text, pin) {
			t.Errorf("the contract must mirror %q", pin)
		}
	}
	// The link type and the source enums live in BOTH the request schema
	// and the evidence view (the drift of any one is red).
	if strings.Count(text, "enum: [original, cross]") < 2 {
		t.Fatalf("the link type enum mirror must live in the request and the view schemas, got %d", strings.Count(text, "enum: [original, cross]"))
	}
	if strings.Count(text, "enum: [abcp_catalog, laximo_catalog]") < 2 {
		t.Fatalf("the source enum mirror must live in the request and the view schemas, got %d", strings.Count(text, "enum: [abcp_catalog, laximo_catalog]"))
	}
}
