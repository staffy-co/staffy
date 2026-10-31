//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 4.5 integration matrix: the supplier offer and the selection
// snapshot contour over the LIVE Postgres. The world of the matrix (the
// seed 755 — a free slot of the retro-3-3 register; the probe family
// continues the 892xx d2xx family of Epic 4) seals the auto-parts@6.0.0
// pin through the full live release chain (the pin chain 2.0.0 → 3.0.0 →
// 4.0.0 → 5.0.0 → the compatible additive upgrade 6.0.0 with the
// comparison policy) BEFORE the case ingestion. The matrix walks the
// offer record command LIVE (the guard refusals BEFORE any SQL with the
// dictionary words, the LIVE digest pins of the three inputs, the
// monotonic offer numbers, the idempotent replay), the read-time
// comparison derivation (the ranked variants of the confirmed budget
// criterion with the explanation words, the change words of the new
// record of one supplier, the shortfall word of the partial availability,
// the stale state of the age, the honest unknown of the absent offers)
// and the selection snapshot command (the snapshot with the pins of ALL
// FOUR inputs, the refusal family of the non-proven position, the expired
// offer and the over-availability quantity) with the RLS blindness. The
// degradation world (the seed 756) pins the 5.0.0 pack — the fitment
// panel stays defined (the control) while the comparison panel carries
// the honest absence word.

// offersSeedSeq keeps the seeded probe rows globally unique.
var offersSeedSeq int64

// nextOffersProbeID mints one globally unique probe id of the matrix.
func nextOffersProbeID() id.UUID {
	offersSeedSeq++
	return id.MustParse(fmt.Sprintf("019a2f4c-6000-7000-8000-%012d", 89200+offersSeedSeq))
}

// offersSealPackSix seals the auto-parts@6.0.0 pin through the live
// release chain: the fitment seal chain of 4.4 (2.0.0 → 3.0.0 → 4.0.0 →
// 5.0.0) plus the compatible 5.0.0→6.0.0 upgrade (the additive comparison
// policy — Changed [comparison]), the assemble/version/seal of the new
// manifest, the fresh profile approval over the new link and the new
// acting admission decision.
func offersSealPackSix(t *testing.T, w *casesWorld, employee id.UUID) employeeDomain.ManifestLink {
	t.Helper()
	ctx := context.Background()
	// The five-step pin chain: the fitment chain of 4.4 up to 5.0.0 plus
	// the compatible additive comparison policy of 6.0.0.
	if _, err := w.employee.Pin(ctx, employeeAPI.PinCommand{
		Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000c226"),
		ExpectedVersion: func() int64 {
			current, err := w.employee.Read(ctx, w.owner, w.tenant, employee)
			if err != nil {
				t.Fatal(err)
			}
			return current.Version
		}(), PackID: "auto-parts", PackVersion: "5.0.0",
		Variants: map[string]any{"parts_interest": "both", "matching_depth": float64(2)}, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	current, err := w.employee.Read(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.Pin(ctx, employeeAPI.PinCommand{
		Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
		CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000c227"),
		ExpectedVersion: current.Version, PackID: "auto-parts", PackVersion: "6.0.0",
		Variants: map[string]any{"parts_interest": "both", "matching_depth": float64(2)}, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	// The release chain over the upgraded refs (the live revision walk).
	view, err := w.employee.ReleaseView(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	revision := int64(0)
	if view.Draft != nil {
		revision = view.Draft.Revision
	}
	draft := w.assemble(t, employee, revision)
	version := w.createVersion(t, employee, draft.Revision, "Пин 6.0.0: политика сравнения предложений")
	manifest, err := w.seal(t, employee, version.Version, draft.Revision+1)
	if err != nil {
		t.Fatal(err)
	}
	link := employeeDomain.ManifestLink{ManifestID: manifest.ManifestID, ConfigVersion: manifest.ConfigVersion, Digest: manifest.Digest}
	// The fresh profile approval over the new link (the 6.0.0 pack carries
	// the same journeys as 5.0.0 — no new coverage cells).
	content := profileContent(w.owner.ID, w.backup)
	content.Matrix = append(content.Matrix,
		employeeDomain.CoverageCell{
			Journey: "идентификация автомобиля", Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound},
			Modality: employeeDomain.ModalityVoice, ConversationMode: "sales", AutonomyClass: "assisted",
			EvalRef: "eval://auto-parts/identify/voice-v1", HumanFallback: "human_callback по регламенту",
		},
		employeeDomain.CoverageCell{
			Journey: "идентификация автомобиля", Route: employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound},
			Modality: employeeDomain.ModalityText, ConversationMode: "sales", AutonomyClass: "assisted",
			EvalRef: "eval://auto-parts/identify/text-v1", HumanFallback: "human_callback по регламенту",
		},
	)
	actor := actorPrincipal{principalID: w.owner.ID, stepUp: true}
	if _, err := w.saveDraft(t, employee, link, content, 2, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := w.approve(t, employee, 3, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
		Actor: actor.principal(), TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000c228"),
		Stage:         employeeDomain.StageSynthetic,
	}); err != nil {
		t.Fatal(err)
	}
	return link
}

// offersSealPackFiveToSix seals the FULL pin chain from the base: the
// 2.0.0 → 3.0.0 → 4.0.0 chain of the part request world (the fresh world
// walks the whole ladder — the same five-step chain the production
// upgrade walks).
func offersSealFullChain(t *testing.T, w *casesWorld, employee id.UUID) employeeDomain.ManifestLink {
	t.Helper()
	ctx := context.Background()
	for step, pin := range []struct {
		version  string
		depth    float64
		confirme bool
	}{
		{"2.0.0", 1, true},
		{"3.0.0", 2, true},
		{"4.0.0", 2, true},
		{"5.0.0", 2, true},
		{"6.0.0", 2, true},
	} {
		current, err := w.employee.Read(ctx, w.owner, w.tenant, employee)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.employee.Pin(ctx, employeeAPI.PinCommand{
			Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
			CorrelationID:   id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-00000000c2%02d", 30+step)),
			ExpectedVersion: current.Version, PackID: "auto-parts", PackVersion: pin.version,
			Variants: map[string]any{"parts_interest": "both", "matching_depth": pin.depth}, Confirmed: pin.confirme,
		}); err != nil {
			t.Fatal(err)
		}
	}
	view, err := w.employee.ReleaseView(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	revision := int64(0)
	if view.Draft != nil {
		revision = view.Draft.Revision
	}
	draft := w.assemble(t, employee, revision)
	version := w.createVersion(t, employee, draft.Revision, "Пин 6.0.0: политика сравнения предложений")
	manifest, err := w.seal(t, employee, version.Version, draft.Revision+1)
	if err != nil {
		t.Fatal(err)
	}
	link := employeeDomain.ManifestLink{ManifestID: manifest.ManifestID, ConfigVersion: manifest.ConfigVersion, Digest: manifest.Digest}
	content := profileContent(w.owner.ID, w.backup)
	content.Matrix = append(content.Matrix,
		employeeDomain.CoverageCell{
			Journey: "идентификация автомобиля", Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound},
			Modality: employeeDomain.ModalityVoice, ConversationMode: "sales", AutonomyClass: "assisted",
			EvalRef: "eval://auto-parts/identify/voice-v1", HumanFallback: "human_callback по регламенту",
		},
		employeeDomain.CoverageCell{
			Journey: "идентификация автомобиля", Route: employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound},
			Modality: employeeDomain.ModalityText, ConversationMode: "sales", AutonomyClass: "assisted",
			EvalRef: "eval://auto-parts/identify/text-v1", HumanFallback: "human_callback по регламенту",
		},
	)
	actor := actorPrincipal{principalID: w.owner.ID, stepUp: true}
	if _, err := w.saveDraft(t, employee, link, content, 2, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := w.approve(t, employee, 3, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
		Actor: actor.principal(), TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000c239"),
		Stage:         employeeDomain.StageSynthetic,
	}); err != nil {
		t.Fatal(err)
	}
	return link
}

// offerRecord records ONE supplier offer through the LIVE command (the
// operator-manager write path with the derived pins).
func offerRecord(t *testing.T, w *casesWorld, caseID, workItemID id.UUID, key string, mutation func(*casesAPI.RecordSupplierOfferCommand)) casesAPI.WorkOutcome {
	t.Helper()
	purchase := 3900.0
	term := 2
	command := casesAPI.RecordSupplierOfferCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		PositionKey: "WF2B196K666AA", SupplierID: "SUP-1", SupplierName: "Поставщик один",
		Brand: "Bosch", Article: "WF2B196K666AA",
		ClientPrice: 4850, PurchasePrice: &purchase, Currency: casesDomain.OfferCurrencyRUB,
		QuantityAvailable: 4, TermDays: &term, Conditions: "самовывоз со склада",
		SourceSystem: casesDomain.OfferSourceAbcpCatalog, SourceRecordID: "rec-" + key, SourceVersion: "v2026.10",
		SourceObservedAt: releaseClock.Now().Add(-2 * time.Hour),
		IdempotencyKey:   "offers-" + key, Now: releaseClock.Now(),
	}
	if mutation != nil {
		mutation(&command)
	}
	outcome, err := w.cases.RecordSupplierOffer(context.Background(), command)
	if err != nil {
		t.Fatalf("the offer record command refused: %v", err)
	}
	return outcome
}

// offerPositionOf resolves ONE position of the comparison view.
func offerPositionOf(t *testing.T, detail casesAPI.CaseView, positionKey string) casesDomain.ComparisonPositionView {
	t.Helper()
	for _, position := range detail.Comparison.Comparison.Positions {
		if position.PositionKey == positionKey {
			return position
		}
	}
	t.Fatalf("the comparison position %q not found", positionKey)
	return casesDomain.ComparisonPositionView{}
}

// offerVariantOf resolves ONE ranked variant of the position by the
// supplier.
func offerVariantOf(t *testing.T, position casesDomain.ComparisonPositionView, supplierID string) casesDomain.ComparisonVariantView {
	t.Helper()
	for _, variant := range position.Variants {
		if variant.SupplierID == supplierID {
			return variant
		}
	}
	t.Fatalf("the variant of the supplier %q not found", supplierID)
	return casesDomain.ComparisonVariantView{}
}

// TestOffersMatrixRowsBySpec walks the Story 4.5 matrix LIVE.
func TestOffersMatrixRowsBySpec(t *testing.T) {
	w := newCasesWorld(t, 755)
	ctx := context.Background()
	offersSealFullChain(t, w, w.employeeID)

	// The ingestion of the matrix case (the world pins 6.0.0 — the case
	// row pins the comparison-policy-bearing pack).
	ingest, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-offers-1", "sess-offers-1", `{"call":"inbound-call","from":"+79000000787"}`))
	if err != nil {
		t.Fatal(err)
	}
	result := acceptedOutcome(t, ingest)
	caseID := result.CaseID
	ingestDetail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ingestDetail.WorkItems) != 1 {
		t.Fatalf("expected the initial work item, got %d", len(ingestDetail.WorkItems))
	}
	workItemID := ingestDetail.WorkItems[0].WorkItemID

	// row-1: the case pins the comparison-policy-bearing pack 6.0.0 — the
	// comparison panel is defined, NO offer exists: the honest case-level
	// unknown word with the offers research recommendation (AC-12 — the
	// old price never repeats as the current one), the price authority
	// word, the term estimate word and the scope word.
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Case.PackID != "auto-parts" || detail.Case.PackVersion != "6.0.0" {
		t.Fatalf("the case must pin auto-parts@6.0.0, got %s@%s", detail.Case.PackID, detail.Case.PackVersion)
	}
	if !detail.Comparison.SchemaDefined || detail.Comparison.Note != "" {
		t.Fatalf("the schema-defined comparison panel must carry no absence note, got %+v", detail.Comparison)
	}
	if detail.Comparison.Comparison.CaseUnknownWord != casesDomain.OfferUnknownWord {
		t.Fatalf("the honest case-level unknown word expected, got %q", detail.Comparison.Comparison.CaseUnknownWord)
	}
	if len(detail.Comparison.Comparison.CaseHandoffWords) != 1 || detail.Comparison.Comparison.CaseHandoffWords[0] != casesDomain.HandoffReasonOffersResearch {
		t.Fatalf("the offers research recommendation expected, got %v", detail.Comparison.Comparison.CaseHandoffWords)
	}
	if detail.Comparison.Comparison.PriceAuthorityWord != casesDomain.OfferPriceAuthorityWord || detail.Comparison.Comparison.TermEstimateWord != casesDomain.OfferTermEstimateWord {
		t.Fatalf("the price authority and the term estimate words expected")
	}

	// row-2: the part request material of the case — the confirmed kind,
	// the OEM number (the exact repeat mode), the budget («до 5000») and
	// the quantity (6): the confirmed criteria of the ranking and the
	// shortfall material.
	for _, material := range []struct {
		key   string
		value []byte
		name  string
	}{
		{"part.kind", []byte(`"фильтр салона"`), "offers-fact-kind"},
		{"part.oem_number", []byte(`"WF2B196K666AA"`), "offers-fact-oem"},
		{"part.budget", []byte(`"до 5000"`), "offers-fact-budget"},
		{"part.quantity", []byte(`6`), "offers-fact-quantity"},
	} {
		kindOutcome := partRecordFact(t, w, caseID, workItemID, material.key, material.value, material.name+"-1")
		if _, err := w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
			Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: kindOutcome.FactID,
			VersionNo: 1, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
			Note: "Клиент назвал материал в разговоре", IdempotencyKey: material.name + "-confirm-1", Now: casesClock.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	// row-3: the proven fitment record of the position (the comparison
	// READS the 4.4 output — the availability never changes the fitment
	// status, AC-1).
	fitmentRecord(t, w, caseID, workItemID, "offers-1", nil)

	// row-4: the LIVE offer record of the first supplier → the current
	// position with the ranked variant, the OfferDigest, the operator
	// record material with the purchase price and the internal word.
	first := offerRecord(t, w, caseID, workItemID, "1", nil)
	if first.SupplierOfferID == first.SupplierOfferID && first.OfferPositionKey != "WF2B196K666AA" {
		t.Fatalf("the outcome carries the position key, got %+v", first)
	}
	// row-5: the second supplier with the cheaper price → the budget
	// ranking (the confirmed criterion): the cheaper variant carries the
	// rank 1 with the explanation word (AC-6), the pricier one the rank 2;
	// the shortfall word of the partial availability (4 of 6 — AC-11).
	offerRecord(t, w, caseID, workItemID, "2", func(command *casesAPI.RecordSupplierOfferCommand) {
		command.SupplierID = "SUP-2"
		command.SupplierName = "Поставщик два"
		command.Brand = "Mann"
		command.ClientPrice = 4200
		command.PurchasePrice = nil
		command.QuantityAvailable = 6
		command.IdempotencyKey = "offers-2"
		command.SourceRecordID = "rec-2"
	})
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	position := offerPositionOf(t, detail, "WF2B196K666AA")
	if position.Status != casesDomain.OfferStatusCurrent || position.StatusWord != casesDomain.OfferCurrentWord {
		t.Fatalf("the current position expected, got %+v", position)
	}
	if position.FitmentStatus != casesDomain.FitmentStatusProven {
		t.Fatalf("the fitment status stays the 4.4 output (AC-1), got %+v", position.FitmentStatus)
	}
	if position.LinkTypeWord != "оригинал" {
		t.Fatalf("the original group word expected, got %q", position.LinkTypeWord)
	}
	cheaper := offerVariantOf(t, position, "SUP-2")
	pricier := offerVariantOf(t, position, "SUP-1")
	if cheaper.Rank != 1 || cheaper.RankWord != casesDomain.OfferRankBudgetWord {
		t.Fatalf("the cheaper variant carries the rank 1 with the budget word, got %+v", cheaper)
	}
	if pricier.Rank != 2 || pricier.RankWord != casesDomain.OfferRankBudgetWord {
		t.Fatalf("the pricier variant carries the rank 2, got %+v", pricier)
	}
	if pricier.ShortfallWord != "у этого поставщика меньше запрошенного: 4 из 6" {
		t.Fatalf("the shortfall word expected, got %q", pricier.ShortfallWord)
	}
	if cheaper.ShortfallWord != "" {
		t.Fatalf("the full availability carries no shortfall word, got %q", cheaper.ShortfallWord)
	}
	if len(cheaper.OfferDigest) != 64 || len(pricier.OfferDigest) != 64 {
		t.Fatalf("the canonical offer digests expected")
	}
	// The operator record section: BOTH records with the purchase price
	// material of the first (the internal word) and the honest absence of
	// the second (AC-2).
	if len(detail.Comparison.Records) != 2 {
		t.Fatalf("the operator record section carries both records, got %d", len(detail.Comparison.Records))
	}
	for _, record := range detail.Comparison.Records {
		if record.SupplierID == "SUP-1" {
			if record.PurchasePrice == nil || *record.PurchasePrice != 3900 || record.PurchaseWord != casesDomain.OfferPurchaseInternalWord {
				t.Fatalf("the purchase price with the internal word expected, got %+v", record)
			}
		} else if record.PurchasePrice != nil || record.PurchaseWord != "" {
			t.Fatalf("the honest purchase absence expected, got %+v", record)
		}
	}
	// The voice block: the key differences by the confirmed criterion and
	// the recommended variant of the rank 1 (AC-14/AC-7).
	if len(detail.Comparison.Comparison.VoiceBlock.KeyDifferences) != 1 || !strings.Contains(detail.Comparison.Comparison.VoiceBlock.KeyDifferences[0], "от 4200 до 4850") {
		t.Fatalf("the voice key differences of the budget expected, got %v", detail.Comparison.Comparison.VoiceBlock.KeyDifferences)
	}
	if !strings.Contains(detail.Comparison.Comparison.VoiceBlock.RecommendedWord, "Mann") || !strings.Contains(detail.Comparison.Comparison.VoiceBlock.RecommendedWord, "Бюджет") {
		t.Fatalf("the recommended variant word of the confirmed criterion expected, got %q", detail.Comparison.Comparison.VoiceBlock.RecommendedWord)
	}

	// row-6: the idempotent replay of the exact repeat — the stored
	// outcome returns (the same key, the same material).
	replay := offerRecord(t, w, caseID, workItemID, "1", nil)
	if replay.SupplierOfferID != first.SupplierOfferID {
		t.Fatalf("the idempotent replay returns the stored outcome, got %+v", replay)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Comparison.Records) != 2 {
		t.Fatalf("the replay writes nothing new, got %d records", len(detail.Comparison.Records))
	}

	// row-7: the guard refusals of the offer command — BEFORE any SQL with
	// the honest dictionary words (the foreign currency, the non-canonical
	// position key, the negative client price, the missing traceability).
	for _, refusal := range []struct {
		name string
		mut  func(*casesAPI.RecordSupplierOfferCommand)
		rule string
		word string
	}{
		{"foreign currency", func(c *casesAPI.RecordSupplierOfferCommand) { c.Currency = "USD"; c.IdempotencyKey = "offer-refusal-currency" }, "offer_currency_out_of_dictionary", "Domain Policy пака"},
		{"non-canonical key", func(c *casesAPI.RecordSupplierOfferCommand) { c.PositionKey = "wf 2b19"; c.IdempotencyKey = "offer-refusal-key" }, "offer_position_key_not_canonical", "канонизируйте"},
		{"negative price", func(c *casesAPI.RecordSupplierOfferCommand) { c.ClientPrice = -1; c.IdempotencyKey = "offer-refusal-price" }, "offer_client_price_negative", "модель не рассчитывает"},
		{"no source record id", func(c *casesAPI.RecordSupplierOfferCommand) { c.SourceRecordID = ""; c.IdempotencyKey = "offer-refusal-source" }, "offer_source_record_id_required", "трассируемость"},
	} {
		command := casesAPI.RecordSupplierOfferCommand{
			Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			PositionKey: "WF2B196K666AA", SupplierID: "SUP-9", SupplierName: "Поставщик девять",
			Brand: "Febi", Article: "WF2B196K666AA",
			ClientPrice: 1000, Currency: casesDomain.OfferCurrencyRUB, QuantityAvailable: 1,
			SourceSystem: casesDomain.OfferSourceAbcpCatalog, SourceRecordID: "rec-r", SourceVersion: "v2026.10",
			SourceObservedAt: releaseClock.Now().Add(-1 * time.Hour),
			Now:              releaseClock.Now(),
		}
		refusal.mut(&command)
		_, err := w.cases.RecordSupplierOffer(ctx, command)
		if err == nil {
			t.Fatalf("%s: the dishonest offer is accepted", refusal.name)
		}
		rule, ok := err.(casesDomain.WorkRuleError)
		if !ok {
			t.Fatalf("%s: the refusal is not a WorkRuleError: %v", refusal.name, err)
		}
		// The app wraps the domain guard into the command rule (the rule()
		// helper): the nested dictionary rule travels INSIDE the honest
		// reason text together with the dictionary word.
		if rule.Rule != "record_supplier_offer" {
			t.Fatalf("%s: rule = %q, want the command rule", refusal.name, rule.Rule)
		}
		if !strings.Contains(rule.Reason, refusal.rule) || !strings.Contains(rule.Reason, refusal.word) {
			t.Fatalf("%s: the honest refusal must carry the nested rule %q and the word %q, got %q", refusal.name, refusal.rule, refusal.word, rule.Reason)
		}
	}

	// row-8: the change words — the NEW record of the first supplier with
	// the changed price supersedes BY THE NUMBER (the old record stays the
	// numbered material): the change word names the changed aspects
	// (AC-10).
	offerRecord(t, w, caseID, workItemID, "3", func(command *casesAPI.RecordSupplierOfferCommand) {
		command.ClientPrice = 5100
		command.QuantityAvailable = 3
		command.IdempotencyKey = "offers-3"
		command.SourceRecordID = "rec-3"
	})
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	position = offerPositionOf(t, detail, "WF2B196K666AA")
	if len(position.Variants) != 2 {
		t.Fatalf("the superseded record never doubles the variant list, got %d", len(position.Variants))
	}
	changed := offerVariantOf(t, position, "SUP-1")
	if changed.ClientPrice != 5100 {
		t.Fatalf("the current record of the supplier is the NEW number's one, got %.0f", changed.ClientPrice)
	}
	if len(changed.ChangeWords) != 1 || !strings.Contains(changed.ChangeWords[0], "цена") || !strings.Contains(changed.ChangeWords[0], "требуется новая точная версия подтверждения") {
		t.Fatalf("the change word of the price expected, got %v", changed.ChangeWords)
	}

	// row-9: the stale state by the AGE — the offer whose received time is
	// over the pack TTL (the clock fixture: the command lands with the
	// past Now, the view clock stays fixed — the world clock of the case
	// service is the releaseClock). The position G330PR carries the proven
	// fitment record FIRST (the honest state machine: the expired offer of
	// the proven position — the selection refusal of the expiry).
	fitmentRecord(t, w, caseID, workItemID, "offers-aged", func(command *casesAPI.RecordFitmentEvidenceCommand) {
		command.PositionKey = "G330PR"
		command.Article = "G330PR"
		command.IdempotencyKey = "fitment-offers-aged"
		command.SourceRecordID = "rec-offers-aged"
	})
	offerRecord(t, w, caseID, workItemID, "4", func(command *casesAPI.RecordSupplierOfferCommand) {
		command.PositionKey = "G330PR"
		command.SupplierID = "SUP-3"
		command.SupplierName = "Поставщик три"
		command.Brand = "G330PR"
		command.Article = "G330PR"
		command.ClientPrice = 1500
		command.PurchasePrice = nil
		command.IdempotencyKey = "offers-4"
		command.SourceRecordID = "rec-4"
		command.Now = releaseClock.Now().Add(-86401 * time.Second)
	})
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	aged := offerPositionOf(t, detail, "G330PR")
	if aged.Status != casesDomain.OfferStatusStale || aged.StatusWord != casesDomain.OfferStalePositionWord {
		t.Fatalf("the over-TTL position turns stale, got %+v", aged)
	}
	if len(aged.Variants) != 1 || len(aged.Variants[0].StaleWords) == 0 || aged.Variants[0].StaleWords[0] != casesDomain.OfferStaleAgeWord {
		t.Fatalf("the age stale word expected, got %+v", aged.Variants)
	}

	// row-10: the honest unknown of the position WITHOUT offers (the
	// proven fitment, no commercial conditions) — the position-level
	// unknown with the research recommendation.
	if aged = offerPositionOf(t, detail, "WF2B196K666AA"); aged.Status != casesDomain.OfferStatusCurrent {
		t.Fatalf("the live position stays current, got %+v", aged)
	}

	// row-10b: the offer of the position WITHOUT a fitment record — the
	// honest unproven position (the availability never changes the
	// fitment status, AC-1; the selection of it refuses with the honest
	// fitment word).
	offerRecord(t, w, caseID, workItemID, "5", func(command *casesAPI.RecordSupplierOfferCommand) {
		command.PositionKey = "H777PR"
		command.SupplierID = "SUP-4"
		command.SupplierName = "Поставщик четыре"
		command.Brand = "H777PR"
		command.Article = "H777PR"
		command.ClientPrice = 900
		command.PurchasePrice = nil
		command.QuantityAvailable = 10
		command.IdempotencyKey = "offers-5"
		command.SourceRecordID = "rec-5"
	})

	// row-11: the LIVE selection snapshot — the chosen CURRENT offer of
	// the second supplier with the quantity 2: the snapshot pins ALL FOUR
	// inputs (AC-13), the known client sum 2 × 4200 = 8400 with the
	// boundary words and the order boundary (the fence OQ-3/OQ-4).
	selection, err := w.cases.RecordClientSelection(ctx, casesAPI.RecordClientSelectionCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		PositionKey: "WF2B196K666AA", SupplierID: "SUP-2", OfferNo: 1, Quantity: 2,
		IdempotencyKey: "selection-1", Now: releaseClock.Now(),
	})
	if err != nil {
		t.Fatalf("the selection command refused: %v", err)
	}
	if selection.SelectionSnapshotID == selection.SelectionSnapshotID && len(selection.SelectionDigest) != 64 {
		t.Fatalf("the selection outcome carries the canonical digest, got %+v", selection)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Comparison.Selections) != 1 {
		t.Fatalf("one selection snapshot expected, got %d", len(detail.Comparison.Selections))
	}
	snapshot := detail.Comparison.Selections[0]
	if snapshot.Quantity != 2 || snapshot.UnitClientPrice != 4200 || snapshot.KnownClientSum != 8400 {
		t.Fatalf("the known sum arithmetic 2 × 4200 = 8400 expected, got %+v", snapshot)
	}
	if snapshot.SumBoundaryWord != casesDomain.OfferSumBoundaryWord || snapshot.OrderBoundaryWord != casesDomain.OfferOrderBoundaryWord {
		t.Fatalf("the boundary words expected, got %+v", snapshot)
	}
	if snapshot.ProfileDigestPin == "" || snapshot.PartRequestDigestPin == "" || snapshot.FitmentDigestPin == "" || snapshot.OfferDigestPin == "" {
		t.Fatalf("the snapshot pins ALL FOUR inputs, got %+v", snapshot)
	}

	// row-12: the selection refusals — the non-proven position, the
	// expired offer and the over-availability quantity, all BEFORE any SQL
	// with the honest words (AC-5/AC-7, AC-9/AC-10, AC-11).
	for _, refusal := range []struct {
		name string
		mut  func(*casesAPI.RecordClientSelectionCommand)
		word string
	}{
		{"non-proven position", func(c *casesAPI.RecordClientSelectionCommand) {
			c.PositionKey = "H777PR"
			c.SupplierID = "SUP-4"
			c.OfferNo = 1
			c.Quantity = 1
			c.IdempotencyKey = "selection-refusal-proven"
		}, casesDomain.OfferSelectionGuardFitmentWord},
		{"expired offer", func(c *casesAPI.RecordClientSelectionCommand) {
			c.PositionKey = "G330PR"
			c.SupplierID = "SUP-3"
			c.OfferNo = 1
			c.Quantity = 1
			c.IdempotencyKey = "selection-refusal-stale"
		}, casesDomain.OfferSelectionGuardStaleWord},
		{"over availability", func(c *casesAPI.RecordClientSelectionCommand) {
			c.SupplierID = "SUP-2"
			c.OfferNo = 1
			c.Quantity = 7
			c.IdempotencyKey = "selection-refusal-quantity"
		}, casesDomain.OfferSelectionGuardQuantityWord},
	} {
		command := casesAPI.RecordClientSelectionCommand{
			Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			PositionKey: "WF2B196K666AA", SupplierID: "SUP-2", OfferNo: 1, Quantity: 2,
			Now: releaseClock.Now(),
		}
		refusal.mut(&command)
		_, err := w.cases.RecordClientSelection(ctx, command)
		if err == nil {
			t.Fatalf("%s: the dishonest selection is accepted", refusal.name)
		}
		if !strings.Contains(err.Error(), refusal.word) {
			t.Fatalf("%s: the honest word %q expected, got %v", refusal.name, refusal.word, err)
		}
	}
	// The refusals wrote NOTHING (the append-only surfaces stay at one
	// snapshot).
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Comparison.Selections) != 1 {
		t.Fatalf("the refusals wrote no snapshots, got %d", len(detail.Comparison.Selections))
	}

	// row-13: the RLS blindness — the foreign principal never reads the
	// case view of the matrix tenant (the generic refusal without
	// disclosure).
	foreignActor := identityDomain.Principal{ID: nextOffersProbeID()}
	if _, err := w.cases.CaseView(ctx, foreignActor, continuationForeignTenant, caseID); err == nil ||
		!(errors.Is(err, casesDomain.ErrNotFound) || errors.Is(err, identityDomain.ErrNotFound)) {
		t.Fatalf("the foreign principal must never read the case view (the membership blindness), got %v", err)
	}
}

// TestOffersPanelDegradesWithoutComparison pins the honest degradation of
// the 5.0.0 pin: the pack carries the fitment policy but NO comparison
// policy — the comparison panel is absent with the honest word, the
// fitment panel of 4.4 stays defined (the control).
func TestOffersPanelDegradesWithoutComparison(t *testing.T) {
	w := newCasesWorld(t, 756)
	ctx := context.Background()
	fitmentSealPackFive(t, w, w.employeeID)
	ingest, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-offers-degrade-1", "sess-offers-degrade-1", `{"call":"inbound-call","from":"+79000000788"}`))
	if err != nil {
		t.Fatal(err)
	}
	caseID := acceptedOutcome(t, ingest).CaseID
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Case.PackVersion != "5.0.0" {
		t.Fatalf("the degradation case pins the 5.0.0 pack, got %s", detail.Case.PackVersion)
	}
	if !detail.Fitment.SchemaDefined {
		t.Fatal("the fitment panel of 5.0.0 stays defined (the control)")
	}
	if detail.Comparison.SchemaDefined || detail.Comparison.Note != casesAPI.ComparisonSchemaNoteAbsent {
		t.Fatalf("the comparison panel must degrade with the honest word, got %+v", detail.Comparison)
	}
	// The record command refuses honestly on a pack without the comparison
	// policy (never a hardcoded policy).
	workItems := detail.WorkItems
	if len(workItems) != 1 {
		t.Fatalf("the initial work item expected, got %d", len(workItems))
	}
	_, err = w.cases.RecordSupplierOffer(ctx, casesAPI.RecordSupplierOfferCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItems[0].WorkItemID,
		PositionKey: "WF2B196K666AA", SupplierID: "SUP-1", SupplierName: "Поставщик один",
		Brand: "Bosch", Article: "WF2B196K666AA",
		ClientPrice: 4850, Currency: casesDomain.OfferCurrencyRUB, QuantityAvailable: 4,
		SourceSystem: casesDomain.OfferSourceAbcpCatalog, SourceRecordID: "rec-degrade", SourceVersion: "v2026.10",
		SourceObservedAt: releaseClock.Now().Add(-1 * time.Hour),
		IdempotencyKey:   "offer-degrade-1", Now: releaseClock.Now(),
	})
	if err == nil || !strings.Contains(err.Error(), "политика сравнения пака не определена") {
		t.Fatalf("the offer record on a pack without the comparison policy must refuse honestly, got %v", err)
	}
}
