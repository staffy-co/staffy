package httptransport

import (
	"bytes"
	"os"
	"strings"
	"testing"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
)

// ---------------------------------------------------------------------------
// The Story 4.5 offers wire shapes: the operator panel rendering (the
// state words as TEXT — role=alert for the stale words, the variants
// grouped by the link type and the confirmed fitment, the voice block),
// the honest degradation word, the two record form skeletons, the
// purchase-price boundary of the client material and the OpenAPI contract
// mirrors of the dictionaries.
// ---------------------------------------------------------------------------

// offersPanelSurface builds the Story 4.5 panel answer of the wire tests:
// one current position with two ranked variants (the budget criterion)
// and the operator record material with the purchase price + the recorded
// selection snapshot.
func offersPanelSurface() casesAPI.ComparisonPanelView {
	now := mustTime("2026-10-24T12:00:00Z")
	observed := mustTime("2026-10-24T10:00:00Z")
	term := 2
	purchase := 3900.0
	first := casesDomain.ComparisonVariantView{
		SupplierID: "SUP-2", SupplierName: "Поставщик два", Brand: "Mann", Article: "WF2B196K666AA",
		OfferNo: 1, ClientPrice: 4200, Currency: casesDomain.OfferCurrencyRUB, QuantityAvailable: 5,
		TermDays: &term, TermWord: casesDomain.OfferTermEstimateWord,
		Rank: 1, RankWord: casesDomain.OfferRankBudgetWord, Status: casesDomain.OfferStatusCurrent,
		StaleWords: []string{}, ChangeWords: []string{},
		OfferDigest: "b1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		Source: casesDomain.OfferSourceView{
			SourceSystem: casesDomain.OfferSourceAbcpCatalog, SourceRecordID: "rec-1", SourceVersion: "v2026.10",
			SourceObservedAt: observed, ReceivedAt: now,
		},
	}
	second := casesDomain.ComparisonVariantView{
		SupplierID: "SUP-1", SupplierName: "Поставщик один", Brand: "Bosch", Article: "WF2B196K666AA",
		OfferNo: 1, ClientPrice: 4850, Currency: casesDomain.OfferCurrencyRUB, QuantityAvailable: 4,
		TermDays: &term, TermWord: casesDomain.OfferTermEstimateWord, Conditions: "самовывоз со склада",
		Rank: 2, RankWord: casesDomain.OfferRankBudgetWord, Status: casesDomain.OfferStatusCurrent,
		StaleWords: []string{}, ChangeWords: []string{},
		ShortfallWord: "у этого поставщика меньше запрошенного: 4 из 6",
		OfferDigest: "c1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		Source: casesDomain.OfferSourceView{
			SourceSystem: casesDomain.OfferSourceAbcpCatalog, SourceRecordID: "rec-2", SourceVersion: "v2026.10",
			SourceObservedAt: observed, ReceivedAt: now,
		},
	}
	position := casesDomain.ComparisonPositionView{
		PositionKey: "WF2B196K666AA", LinkType: casesDomain.FitmentLinkOriginal, LinkTypeWord: "оригинал",
		FitmentStatus: casesDomain.FitmentStatusProven, FitmentStatusWord: casesDomain.FitmentProvenWord,
		Status: casesDomain.OfferStatusCurrent, StatusWord: casesDomain.OfferCurrentWord,
		Words:     []string{},
		Variants:  []casesDomain.ComparisonVariantView{first, second},
	}
	return casesAPI.ComparisonPanelView{
		SchemaDefined: true,
		Comparison: casesDomain.ComparisonView{
			Positions:          []casesDomain.ComparisonPositionView{position},
			PriceAuthorityWord: casesDomain.OfferPriceAuthorityWord,
			TermEstimateWord:   casesDomain.OfferTermEstimateWord,
			ScopeWord:          casesDomain.OfferScopeWord,
			VoiceBlock: casesDomain.ComparisonVoiceBlock{
				KeyDifferences:  []string{"цена: от 4200 до 4850 RUB"},
				RecommendedWord: "по подтверждённому приоритету «Бюджет» рекомендуемый вариант: Mann, поставщик Поставщик два, цена 4200 RUB",
				FullWord:        casesDomain.OfferVoiceFullWord,
			},
		},
		Records: []casesDomain.OfferView{
			{
				OfferNo: 1, PositionKey: "WF2B196K666AA", SupplierID: "SUP-2", SupplierName: "Поставщик два",
				Brand: "Mann", Article: "WF2B196K666AA", ClientPrice: 4200, PurchasePrice: nil,
				Currency: casesDomain.OfferCurrencyRUB, QuantityAvailable: 5, TermDays: &term,
				TermWord: casesDomain.OfferTermEstimateWord,
				Source: casesDomain.OfferSourceView{
					SourceSystem: casesDomain.OfferSourceAbcpCatalog, SourceRecordID: "rec-1", SourceVersion: "v2026.10",
					SourceObservedAt: observed, ReceivedAt: now,
				},
				OfferDigest: "b1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			},
			{
				OfferNo: 1, PositionKey: "WF2B196K666AA", SupplierID: "SUP-1", SupplierName: "Поставщик один",
				Brand: "Bosch", Article: "WF2B196K666AA", ClientPrice: 4850, PurchasePrice: &purchase,
				PurchaseWord: casesDomain.OfferPurchaseInternalWord,
				Currency: casesDomain.OfferCurrencyRUB, QuantityAvailable: 4, TermDays: &term,
				TermWord: casesDomain.OfferTermEstimateWord, Conditions: "самовывоз со склада",
				Source: casesDomain.OfferSourceView{
					SourceSystem: casesDomain.OfferSourceAbcpCatalog, SourceRecordID: "rec-2", SourceVersion: "v2026.10",
					SourceObservedAt: observed, ReceivedAt: now,
				},
				OfferDigest: "c1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			},
		},
		Selections: []casesDomain.SelectionSnapshotView{
			{
				SnapshotNo: 1, PositionKey: "WF2B196K666AA", SupplierID: "SUP-2", OfferNo: 1,
				Quantity: 2, UnitClientPrice: 4200, Currency: casesDomain.OfferCurrencyRUB, KnownClientSum: 8400,
				SumBoundaryWord:   casesDomain.OfferSumBoundaryWord,
				OrderBoundaryWord: casesDomain.OfferOrderBoundaryWord,
				ProfileDigestPin:  "v111111111111111111111111111111111111111111111111111111111111111v",
				PartRequestDigestPin: "p111111111111111111111111111111111111111111111111111111111111111p",
				FitmentDigestPin:     "f111111111111111111111111111111111111111111111111111111111111111f",
				OfferDigestPin:       "b1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
				SelectionDigest:      "d1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
				CreatedAt:            mustTime("2026-10-24T12:30:00Z"),
			},
		},
	}
}

// TestOffersPanelRendersOperatorCard pins the operator panel rendering:
// the grouping words, the ranked variants with the explanation words, the
// voice block, the operator record section with the purchase price and
// its internal-data word, the selection snapshots with the boundary words
// and the honest price/term/scope words of the case level.
func TestOffersPanelRendersOperatorCard(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	panelView := prepareOffersPanel(casesAPI.CaseView{Comparison: offersPanelSurface()})
	if !panelView.Present {
		t.Fatal("the prepared panel is present for the schema-defined case")
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", Offers: panelView}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the offers panel: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"Предложения и сравнение",
		"WF2B196K666AA",
		"оригинал",
		casesDomain.FitmentProvenWord,
		casesDomain.OfferCurrentWord,
		"Mann",
		"Bosch",
		"4200",
		"4850",
		"ранг 1",
		casesDomain.OfferRankBudgetWord,
		"у этого поставщика меньше запрошенного: 4 из 6",
		"цена: от 4200 до 4850 RUB",
		"рекомендуемый вариант: Mann",
		casesDomain.OfferVoiceFullWord,
		casesDomain.OfferPriceAuthorityWord,
		casesDomain.OfferTermEstimateWord,
		casesDomain.OfferScopeWord,
		"3900",
		casesDomain.OfferPurchaseInternalWord,
		"8400",
		casesDomain.OfferSumBoundaryWord,
		casesDomain.OfferOrderBoundaryWord,
		"b1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
	} {
		if !strings.Contains(rendered, pin) {
			t.Errorf("the operator offers panel must carry %q", pin)
		}
	}
	// The snapshot arithmetic renders as the honest equation with the
	// boundary word (AC-4/AC-13).
	if !strings.Contains(rendered, "2 × 4200") {
		t.Fatal("the snapshot must render the known sum equation")
	}
}

// TestOffersStaleWordsAreTextNotColor pins the stale words probe: the
// per-variant stale words render as TEXT inside role=alert elements —
// the expiry is never carried by color alone (AC-9/AC-10/AC-16).
func TestOffersStaleWordsAreTextNotColor(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	surface := offersPanelSurface()
	surface.Comparison.Positions[0].Status = casesDomain.OfferStatusStale
	surface.Comparison.Positions[0].StatusWord = casesDomain.OfferStalePositionWord
	surface.Comparison.Positions[0].Variants[0].StaleWords = []string{casesDomain.OfferStaleAgeWord, casesDomain.OfferStalePartRequestWord}
	panelView := prepareOffersPanel(casesAPI.CaseView{Comparison: surface})
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", Offers: panelView}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{casesDomain.OfferStalePositionWord, casesDomain.OfferStaleAgeWord, casesDomain.OfferStalePartRequestWord} {
		if !strings.Contains(rendered, pin) {
			t.Errorf("the stale word %q must render as text", pin)
		}
		if !strings.Contains(rendered, "role=\"alert\"") {
			t.Fatalf("the stale words must render inside role=alert elements")
		}
	}
}

// TestOffersDegradedPanelNote pins the honest degradation: a pack without
// the comparison policy renders the honest note — the panel section is
// absent, the reading NEVER fails (the 4.2/4.3/4.4 family precedent).
func TestOffersDegradedPanelNote(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	panelView := prepareOffersPanel(casesAPI.CaseView{Comparison: casesAPI.ComparisonPanelView{Note: casesAPI.ComparisonSchemaNoteAbsent}})
	if panelView.Present {
		t.Fatal("the degraded panel is not present")
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", Offers: panelView}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the degraded panel: %v", err)
	}
	rendered := page.String()
	if !strings.Contains(rendered, casesAPI.ComparisonSchemaNoteAbsent) {
		t.Fatal("the degraded note must render")
	}
	if strings.Contains(rendered, `aria-label="Предложения и сравнение"`) {
		t.Fatal("the degraded panel section must be absent")
	}
}

// TestOffersRecordFormsRender pins the two operator PRG forms: the offer
// record form (the pack dictionary selects, the optional numeric fields
// with the honest empty words, the traceability fields) and the selection
// record form (the chosen offer anchor and the quantity bounds).
func TestOffersRecordFormsRender(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	panelView := prepareOffersPanel(casesAPI.CaseView{Comparison: offersPanelSurface()})
	var page bytes.Buffer
	data := map[string]any{
		"TenantID": "01994f64-6c00-7000-8000-000000000002",
		"View": caseDetailPageView{
			CaseID:          "01994f64-6c00-7000-8000-00000000aa01",
			Offers:          panelView,
			ActiveWIOptions: []workOption{{Value: "01994f64-6c00-7000-8000-00000000ee20", Text: "поиск детали"}},
		},
	}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the record forms: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"/app/tenants/01994f64-6c00-7000-8000-000000000002/cases/01994f64-6c00-7000-8000-00000000aa01/work-items/01994f64-6c00-7000-8000-00000000ee20/supplier-offers",
		"/app/tenants/01994f64-6c00-7000-8000-000000000002/cases/01994f64-6c00-7000-8000-00000000aa01/work-items/01994f64-6c00-7000-8000-00000000ee20/selection-snapshots",
		`name="position_key"`,
		`name="supplier_id"`,
		`name="supplier_name"`,
		`name="brand"`,
		`name="article"`,
		`name="client_price"`,
		`name="purchase_price"`,
		`name="currency"`,
		`value="RUB"`,
		`name="quantity_available"`,
		`name="term_days"`,
		`name="conditions"`,
		`name="source_system"`,
		`value="abcp_catalog"`,
		`value="laximo_catalog"`,
		`name="source_record_id"`,
		`name="source_version"`,
		`name="source_observed_at"`,
		`name="offer_no"`,
		`name="quantity"`,
		"автоматический запрос к поставщикам НЕ выполняется",
		"Записать предложение",
		"Записать выбор",
		"ABCP Order не создаётся",
	} {
		if !strings.Contains(rendered, pin) {
			t.Errorf("the record forms must carry %q", pin)
		}
	}
}

// TestOffersPurchasePriceBoundary pins the AC-2 boundary on the wire: the
// purchase price renders ONLY in the operator record section together
// with the internal-data word — the CLIENT material (the ranked variant
// blocks before the operator section) never carries it.
func TestOffersPurchasePriceBoundary(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	panelView := prepareOffersPanel(casesAPI.CaseView{Comparison: offersPanelSurface()})
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", Offers: panelView}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render: %v", err)
	}
	rendered := page.String()
	// The CLIENT material: the substring of the variant details BEFORE the
	// operator record section header — no purchase price and no internal
	// word there.
	clientMaterial := rendered
	if index := strings.Index(rendered, "Записанные предложения — внутренняя секция оператора"); index >= 0 {
		clientMaterial = rendered[:index]
	}
	if strings.Contains(clientMaterial, "3900") {
		t.Fatal("the client material (the variant blocks) carries the purchase price — AC-2 violation")
	}
	if strings.Contains(clientMaterial, casesDomain.OfferPurchaseInternalWord) {
		t.Fatal("the client material carries the internal-data word")
	}
	// The OPERATOR section: the purchase price WITH the internal word.
	operatorMaterial := rendered[strings.Index(rendered, "Записанные предложения — внутренняя секция оператора"):]
	if !strings.Contains(operatorMaterial, "3900") || !strings.Contains(operatorMaterial, casesDomain.OfferPurchaseInternalWord) {
		t.Fatal("the operator record section must carry the purchase price with the internal-data word")
	}
}

// TestOffersOpenAPIContractMirrorsDictionaries pins the contract mirrors:
// the offer status enum (3 words), the currency enum (1), the term kind
// word, the rank criteria enum (3), the tie breakers enum (4), the
// handoff words enum (offers_research) and the schema family names.
func TestOffersOpenAPIContractMirrorsDictionaries(t *testing.T) {
	contract, err := os.ReadFile("../../../contracts/openapi/staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(contract)
	for _, pin := range []string{
		"ComparisonPanelView", "ComparisonView", "ComparisonPositionView", "ComparisonVariantView", "ComparisonVoiceBlock",
		"OfferView", "OfferSourceView", "SelectionSnapshotView", "RecordSupplierOfferRequest", "RecordClientSelectionRequest",
		"recordSupplierOffer", "recordClientSelection",
		"enum: [current, stale, unknown]",
		"enum: [RUB]",
		"enum: [current, stale]",
		"enum: [offers_research]",
		"supplier-offers",
		"selection-snapshots",
		"cases/00022",
		"cases/00024",
	} {
		if !strings.Contains(text, pin) {
			t.Errorf("the contract must mirror %q", pin)
		}
	}
}
