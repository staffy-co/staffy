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
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	casesStore "github.com/staffy-co/staffy/internal/cases/store"
	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5/pgconn"
)

// The Story 4.2 integration matrix: the vehicle identification over the
// LIVE contour. The world of the matrix (the seed 720 — a free slot of the
// retro-3-3 register; the probe family 88xxx is the fresh c2xx family
// of Epic 4: the world builder caps the seed at three digits, the
// family slot lives in the probe ids) seals the auto-parts@3.0.0
// pin through the full live release chain (the pin 2.0.0 over 1.0.0,
// the compatible upgrade to 3.0.0, the assemble/version/seal, the fresh
// profile approval and the new acting admission decision) BEFORE the
// case ingestion — the case row pins the schema-bearing pack and every
// surface derives over it. The matrix walks the honest path first (the
// identification facts, the operator confirmations, the protected link,
// the LIVE client confirmation with confirmer=client) and pins every
// honest derivation word: the stale word of the former version (AC-8),
// the conflicting refusal to pick a variant (AC-7), the consumed
// double-click, the RLS blindness and the dictionary CHECK mirror of
// the migration 00018.

// vehicleSeedSeq keeps the seeded probe rows globally unique.
var vehicleSeedSeq int64

// nextVehicleProbeID mints one globally unique probe id of the matrix.
func nextVehicleProbeID() id.UUID {
	vehicleSeedSeq++
	return id.MustParse(fmt.Sprintf("019a2f4c-6000-7000-8000-%012d", 88000+vehicleSeedSeq))
}

// vehicleSealPackThree seals the auto-parts@3.0.0 pin through the live
// release chain: the incompatible 1.0.0→2.0.0 pin (confirmed), the
// compatible 2.0.0→3.0.0 upgrade (the additive identification schema),
// the assemble/version/seal of the new manifest, the fresh profile
// approval over the new link and the new acting admission decision.
func vehicleSealPackThree(t *testing.T, w *casesWorld, employee id.UUID) employeeDomain.ManifestLink {
	t.Helper()
	ctx := context.Background()
	// The two-step pin: 2.0.0 (confirmed incompatible) then 3.0.0
	// (compatible additive — the identification schema of the spec).
	current, err := w.employee.Read(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.Pin(ctx, employeeAPI.PinCommand{
		Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
		CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000c201"),
		ExpectedVersion: current.Version, PackID: "auto-parts", PackVersion: "2.0.0",
		Variants: map[string]any{"parts_interest": "both", "matching_depth": float64(1)}, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	current, err = w.employee.Read(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.Pin(ctx, employeeAPI.PinCommand{
		Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
		CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000c202"),
		ExpectedVersion: current.Version, PackID: "auto-parts", PackVersion: "3.0.0",
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
	version := w.createVersion(t, employee, draft.Revision, "Пин 3.0.0: схема идентификации")
	manifest, err := w.seal(t, employee, version.Version, draft.Revision+1)
	if err != nil {
		t.Fatal(err)
	}
	link := employeeDomain.ManifestLink{ManifestID: manifest.ManifestID, ConfigVersion: manifest.ConfigVersion, Digest: manifest.Digest}
	// The fresh profile approval over the new link (the acting decision
	// follows the approved profile): the TestIngestNoResponseOwnerRefusesRoute
	// precedent of the second approval round. The content extends the
	// coverage matrix with the new journey of the 3.0.0 pack («идентификация
	// автомобиля» — the honest covered cells; an empty cell is an explicit
	// gap the readiness validation refuses).
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
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000c203"),
		Stage:         employeeDomain.StageSynthetic,
	}); err != nil {
		t.Fatal(err)
	}
	return link
}

// vehicleRecordFact records one identification fact through the live
// operator command (the full provenance path of 2.2).
func vehicleRecordFact(t *testing.T, w *casesWorld, caseID, workItemID id.UUID, key string, value []byte, keyName string) casesAPI.WorkOutcome {
	t.Helper()
	outcome, err := w.cases.RecordFact(context.Background(), casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: key, Value: value, SourceType: casesDomain.FactSourceOperator, SourceID: keyName,
		Reason: "Идентификация автомобиля", IdempotencyKey: keyName, Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// vehicleAttributeOf finds one attribute of the live panel view.
func vehicleAttributeOf(t *testing.T, view casesAPI.CaseView, key string) casesAPI.VehicleAttributeView {
	t.Helper()
	for _, attribute := range view.Vehicle.Profile.Attributes {
		if attribute.MaterialKey == key {
			return attribute
		}
	}
	t.Fatalf("the vehicle panel must carry the %s attribute, got %+v", key, view.Vehicle.Profile.Attributes)
	return casesAPI.VehicleAttributeView{}
}

// TestVehicleIdentificationMatrixRowsBySpec walks the Story 4.2 matrix
// over the live contour of the world c200: the schema-bearing case pin,
// the honest flags of the VIN normalization, the operator
// confirmations, the protected link with the LIVE client confirmation
// (confirmer=client — the first producer of the reserved 2.2 word), the
// stale word of the new version after the confirmation, the conflicting
// refusal, the consumed double-click, the RLS blindness and the CHECK
// mirror of the migration 00018.
func TestVehicleIdentificationMatrixRowsBySpec(t *testing.T) {
	w := newCasesWorld(t, 720)
	ctx := context.Background()
	vehicleSealPackThree(t, w, w.employeeID)
	// The ingestion of the matrix case (the acceptedCase helper pins the
	// 1.0.0 world pin — this world pins 3.0.0, so the ingest runs inline).
	ingest, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-vehicle-1", "sess-vehicle-1", `{"call":"inbound-call","from":"+79000000666"}`))
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

	// row-1: the case pins the schema-bearing pack — the panel is
	// defined, the honest absence word never appears.
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Case.PackID != "auto-parts" || detail.Case.PackVersion != "3.0.0" {
		t.Fatalf("the case must pin auto-parts@3.0.0, got %s@%s", detail.Case.PackID, detail.Case.PackVersion)
	}
	if !detail.Vehicle.SchemaDefined || detail.Vehicle.Note != "" {
		t.Fatalf("the schema-defined panel must carry no absence note, got %+v", detail.Vehicle)
	}
	if len(detail.Vehicle.Profile.Attributes) != 0 {
		t.Fatalf("the empty world carries no attributes yet, got %+v", detail.Vehicle.Profile.Attributes)
	}

	// row-2: the identification facts land through the operator command;
	// the raw VIN (spaces, lower case) is preserved verbatim, the
	// normalized rendering carries the honest check-digit flag (the
	// spec-example VIN does not match the ISO 3779 digit — the flag, not
	// the rejection).
	vinOutcome := vehicleRecordFact(t, w, caseID, workItemID, "vehicle.vin", []byte(`" wf0xxxtt4s1234567 "`), "vehicle-fact-vin-1")
	if vinOutcome.FactStatus != casesDomain.FactStatusUnverified {
		t.Fatalf("the fresh vin fact is unverified, got %+v", vinOutcome)
	}
	vehicleRecordFact(t, w, caseID, workItemID, "vehicle.make", []byte(`"Ford"`), "vehicle-fact-make-1")
	vehicleRecordFact(t, w, caseID, workItemID, "vehicle.model", []byte(`"Focus"`), "vehicle-fact-model-1")
	vehicleRecordFact(t, w, caseID, workItemID, "vehicle.year_of_manufacture", []byte(`"2021"`), "vehicle-fact-year-1")

	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	vin := vehicleAttributeOf(t, detail, "vehicle.vin")
	if vin.RawValue != " wf0xxxtt4s1234567 " {
		t.Fatalf("the raw VIN must be preserved verbatim, got %q", vin.RawValue)
	}
	if vin.NormalizedValue == nil || vin.NormalizedValue.Normalized != "WF0XXXTT4S1234567" {
		t.Fatalf("the normalized rendering must trim/upper/drop separators, got %+v", vin.NormalizedValue)
	}
	joined := strings.Join(vin.NormalizedValue.Words, "; ")
	if !strings.Contains(joined, "контрольная цифра не совпала") || !strings.Contains(joined, "сверьте посимвольно") {
		t.Fatalf("the honest check-digit flag must render, got %q", joined)
	}
	if vin.Status != casesDomain.FactStatusUnverified {
		t.Fatalf("the vin attribute is unverified, got %s", vin.Status)
	}
	// The completeness: the alternative sets by presence, the missing
	// keys carry every non-confirmed key (AC-2 — the confirmed are never
	// re-asked).
	satisfied := strings.Join(detail.Vehicle.Profile.Completeness.SatisfiedBy, "; ")
	if !strings.Contains(satisfied, "vehicle.vin") || !strings.Contains(satisfied, "vehicle.make+vehicle.model+vehicle.year_of_manufacture") {
		t.Fatalf("both alternative sets are satisfied by presence, got %q", satisfied)
	}
	// The honest missing list: NO key is confirmed yet — the present but
	// unverified attributes stay missing too (AC-2 — only the confirmed
	// are never re-asked).
	if len(detail.Vehicle.Profile.Completeness.MissingKeys) != 6 {
		t.Fatalf("all six keys are honestly missing (none confirmed yet), got %v", detail.Vehicle.Profile.Completeness.MissingKeys)
	}

	// row-3: the operator confirmation of the exact versions (the
	// operator_relay producer); the confirmed keys leave the missing
	// list, the digest changes.
	if _, err := w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: vinOutcome.FactID,
		VersionNo: 1, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
		Note: "Клиент продиктовал VIN в разговоре", IdempotencyKey: "vehicle-confirm-vin-1", Now: casesClock.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	digestBefore := detail.Vehicle.Profile.ProfileDigest
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	vin = vehicleAttributeOf(t, detail, "vehicle.vin")
	if vin.Status != casesDomain.FactStatusConfirmed || vin.Confirmation == nil || vin.Confirmation.ConfirmerType != casesDomain.ConfirmerOperatorRelay {
		t.Fatalf("the operator confirmation derives confirmed, got %+v", vin)
	}
	for _, missing := range detail.Vehicle.Profile.Completeness.MissingKeys {
		if missing == "vehicle.vin" {
			t.Fatal("the confirmed vin never re-enters the missing list (AC-2)")
		}
	}
	if detail.Vehicle.Profile.ProfileDigest == digestBefore || len(detail.Vehicle.Profile.ProfileDigest) != 64 {
		t.Fatalf("the ProfileDigest must change over the confirmed set, got %q", detail.Vehicle.Profile.ProfileDigest)
	}

	// row-4: the protected link and the LIVE client confirmation — the
	// full VIN with the readback travels to the client surface (FR-55),
	// the confirmation binds the EXACT versions with confirmer=client
	// (the first producer of the reserved 2.2 word), the use row
	// consumes the limit, the double-click derives consumed.
	issuance, err := w.cases.IssueContinuationLink(ctx, casesAPI.IssueContinuationLinkCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		Purposes: []string{casesDomain.ContinuationPurposeFactCorrection},
		Question: "Подтвердите автомобиль", Explanation: "Сверьте распознанные признаки и подтвердите точные версии.",
		ParticipantRef: "client+79000000666", TTLMinutes: 30,
		IdempotencyKey: "vehicle-link-issue-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if issuance.Token == "" || !issuance.Outcome.ContinuationIssued {
		t.Fatalf("the fresh link must carry the capability, got %+v", issuance)
	}
	surface, err := w.cases.ContinueSurface(ctx, issuance.Token, ownershipNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if surface.Kind != casesAPI.ContinuationSurfaceValid {
		t.Fatalf("the fresh link opens the valid surface, got %s", surface.Kind)
	}
	if surface.Vehicle == nil {
		t.Fatal("the vehicle block must render for the schema-bearing case")
	}
	if !strings.Contains(fmt.Sprintf("%+v", surface.Vehicle.Profile.Attributes), " wf0xxxtt4s1234567 ") {
		t.Fatalf("the client surface shows the RAW value (the client corrects the exact material), got %+v", surface.Vehicle.Profile.Attributes)
	}
	if len(surface.Vehicle.Readback) != 5 {
		t.Fatalf("the readback blocks 4-4-4-4-1 must render, got %d", len(surface.Vehicle.Readback))
	}
	// The LIVE submission: confirm the exact versions of the present
	// attributes of the work item of the link.
	items := []casesAPI.VehicleConfirmationItem{}
	for _, attribute := range surface.Vehicle.Profile.Attributes {
		items = append(items, casesAPI.VehicleConfirmationItem{FactID: attribute.FactID, VersionNo: attribute.FactVersion})
	}
	if len(items) != 4 {
		t.Fatalf("the four present attributes are confirmable, got %d", len(items))
	}
	if err := w.cases.SubmitVehicleConfirmation(ctx, issuance.Token, items, ownershipNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	// The DB probe: the append-only client confirmations with the link
	// reference, the use row with the new word, the metadata-only outbox
	// event (the reused continuation_submitted vocabulary).
	confirmRows, err := adminPool.Query(ctx, `
                select confirmer_type, note from cases.fact_confirmations
                where tenant_id=$1 and fact_id=$2 and version_no=1 and confirmer_type='client'`,
		w.tenant, vinOutcome.FactID)
	if err != nil {
		t.Fatal(err)
	}
	defer confirmRows.Close()
	clientConfirmations := 0
	for confirmRows.Next() {
		var confirmer, note string
		if err = confirmRows.Scan(&confirmer, &note); err != nil {
			t.Fatal(err)
		}
		clientConfirmations++
		if !strings.Contains(note, "защищённой ссылке продолжения") || !strings.Contains(note, issuance.Outcome.LinkID.String()) {
			t.Fatalf("the client confirmation note carries the link reference, got %q", note)
		}
	}
	if err = confirmRows.Err(); err != nil {
		t.Fatal(err)
	}
	if clientConfirmations != 1 {
		t.Fatalf("exactly one client confirmation of the vin version 1, got %d", clientConfirmations)
	}
	useKind := ""
	if err := adminPool.QueryRow(ctx, `
                select use_kind from cases.continuation_link_uses
                where tenant_id=$1 and link_id=$2 and use_kind='vehicle_confirmation_submitted'`,
		w.tenant, issuance.Outcome.LinkID).Scan(&useKind); err != nil {
		t.Fatalf("the use row with the story 4.2 word must land: %v", err)
	}
	// The consumed double-click: the limit of one is spent, the second
	// submission derives the terminal consumed page.
	surface, err = w.cases.ContinueSurface(ctx, issuance.Token, ownershipNow.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if surface.Kind != casesAPI.ContinuationSurfaceConsumed {
		t.Fatalf("the spent link renders the consumed terminal page, got %s", surface.Kind)
	}
	if err := w.cases.SubmitVehicleConfirmation(ctx, issuance.Token, items, ownershipNow.Add(3*time.Minute)); err == nil {
		t.Fatal("the double-click must refuse with the not-usable word")
	} else {
		var rule casesDomain.WorkRuleError
		if !errors.As(err, &rule) || rule.Reason != casesDomain.ContinuationNotUsableText {
			t.Fatalf("the double-click refuses with the generic not-usable word, got %v", err)
		}
	}

	// row-5: a new version of the confirmed attribute makes it honestly
	// stale — the confirmation binds the former version (AC-8), the
	// stale word names it, the digest changes, the aggregate stale note
	// renders (the dependent future results must treat the profile as
	// stale).
	makeOutcome := vehicleRecordFact(t, w, caseID, workItemID, "vehicle.make", []byte(`"Mazda"`), "vehicle-fact-make-2")
	if makeOutcome.FactStatus != casesDomain.FactStatusConflicting {
		t.Fatalf("the new version after the client confirmation conflicts (the 2.2 canon), got %+v", makeOutcome)
	}
	digestClient := ""
	previousDigest := detail.Vehicle.Profile.ProfileDigest
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	makeAttribute := vehicleAttributeOf(t, detail, "vehicle.make")
	if makeAttribute.Status != casesDomain.FactStatusConflicting {
		t.Fatalf("the confirmed-then-changed attribute derives conflicting (canon), got %s", makeAttribute.Status)
	}
	if makeAttribute.Confirmation == nil || makeAttribute.Confirmation.StaleWord != "подтверждение относилось к прежней версии (v1)" {
		t.Fatalf("the stale word must name the former version, got %+v", makeAttribute.Confirmation)
	}
	if detail.Vehicle.Profile.StaleNote != casesDomain.VehicleProfileStaleNote {
		t.Fatalf("the aggregate stale note must render, got %q", detail.Vehicle.Profile.StaleNote)
	}
	if detail.Vehicle.Profile.ProfileDigest == previousDigest {
		t.Fatal("the ProfileDigest must change when a confirmed attribute becomes stale")
	}
	digestClient = detail.Vehicle.Profile.ProfileDigest

	// row-6: the conflicting sources — two values from different
	// sources conflict; Staffy never picks the variant (AC-7).
	modelOutcome := vehicleRecordFact(t, w, caseID, workItemID, "vehicle.model", []byte(`"Mazda 3"`), "vehicle-fact-model-2")
	if modelOutcome.FactStatus != casesDomain.FactStatusConflicting {
		t.Fatalf("the second value of the model conflicts, got %+v", modelOutcome)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	model := vehicleAttributeOf(t, detail, "vehicle.model")
	if model.Status != casesDomain.FactStatusConflicting || len(detail.Vehicle.Profile.Completeness.MissingKeys) == 0 {
		t.Fatalf("the conflicting model is honestly not confirmed: %+v / %v", model.Status, detail.Vehicle.Profile.Completeness.MissingKeys)
	}

	// row-7: the RLS blindness — the same token hash under the FOREIGN
	// tenant scope finds no row (the generic refusal without
	// disclosure); the foreign principal never reads the case view.
	_, random, err := casesDomain.ParseContinuationToken(issuance.Token)
	if err != nil {
		t.Fatal(err)
	}
	tokenHash, hashErr := casesDomain.ContinuationTokenHash(random)
	if hashErr != nil {
		t.Fatal(hashErr)
	}
	foreignScope, err := tenancy.New(nextVehicleProbeID(), continuationForeignTenant)
	if err != nil {
		t.Fatal(err)
	}
	store := casesStore.NewPostgres(runtimePool)
	probeItems := make([]casesPort.VehicleConfirmationItem, 0, len(items))
	for _, item := range items {
		probeItems = append(probeItems, casesPort.VehicleConfirmationItem{FactID: item.FactID, VersionNo: item.VersionNo})
	}
	if err := store.SubmitVehicleConfirmation(ctx, casesPort.SubmitVehicleConfirmationParams{
		Scope: foreignScope, TokenHash: tokenHash, Confirmations: probeItems,
		UseID: nextVehicleProbeID(), OutboxEventID: nextVehicleProbeID(), CorrelationID: nextVehicleProbeID(),
		Now: ownershipNow.Add(4 * time.Minute),
	}); !errors.Is(err, casesDomain.ErrNotFound) {
		t.Fatalf("the foreign-tenant submission must refuse with ErrNotFound (the RLS blindness), got %v", err)
	}
	foreignActor := identityDomain.Principal{ID: nextVehicleProbeID()}
	if _, err := w.cases.CaseView(ctx, foreignActor, continuationForeignTenant, caseID); err == nil ||
		!(errors.Is(err, casesDomain.ErrNotFound) || errors.Is(err, identityDomain.ErrNotFound)) {
		t.Fatalf("the foreign principal must never read the case view (the membership blindness), got %v", err)
	}

	// row-8: the CHECK mirror of the migration 00018 — the honest use
	// word lands (the row-4 probe pinned it), the out-of-dictionary word
	// refuses with the constraint violation (the SQL-зеркало discipline).
	if _, err := adminPool.Exec(ctx, `
                insert into cases.continuation_link_uses (tenant_id, case_id, work_item_id, link_id, use_id,
                        use_kind, fact_id, fact_version_no, submitted_at, created_at)
                values ($1,$2,$3,$4,$5,'supplier_form',null,null,$6,$6)`,
		w.tenant, caseID, workItemID, issuance.Outcome.LinkID, nextVehicleProbeID(), ownershipNow.Add(5*time.Minute)); err == nil {
		t.Fatal("the out-of-dictionary use kind must refuse (the CHECK mirror of 00018)")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Fatalf("the refusal must be the CHECK constraint violation, got %v", err)
		}
	}
	_ = digestClient
}
