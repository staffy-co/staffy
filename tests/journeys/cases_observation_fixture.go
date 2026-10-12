package main

import (
	"context"
	"fmt"
	"time"

	"github.com/artemiosu/staffy/internal/adapters/observationsyn"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.7 fixture: the owning-system read command and the observation
// view over the REAL domain functions (GuardReadOwnCommand,
// GuardMaterialFieldInMatrix, GuardObservationMaterial, ObservationValue,
// DeriveObservationState, ConflictBadgeOf, CurrentObservation) — the
// synthetic reader of the fixture contour is the PRODUCTION adapter
// (observationsyn) with the scripted scenarios (the availability field
// answers with the provider age past its freshness window — the honest
// stale badge of the journey).

// newFixtureOwningReader wires the scripted synthetic reader of the fixture
// world: price and order_status answer ok (the fresh successful path of the
// journey), availability answers ok with the provider age of 10 minutes
// (the 60-second window of the matrix — the stale derivation with the
// re-read badge text).
func newFixtureOwningReader() *observationsyn.Reader {
	return observationsyn.New(map[string]observationsyn.Scenario{
		casesDomain.MaterialFieldAvailability: {Word: observationsyn.ScenarioOK, Age: 10 * time.Minute},
	})
}

// fixtureObservationID generates the next deterministic observation id.
func (f *casesFixture) fixtureObservationID() id.UUID {
	f.observationSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(80000+f.observationSeq)))
}

// ReadOwningSystem reads one material field of the Work Item from the
// synthetic owning system (the REAL guards; the matrix dictionary; the
// honest failure rows without a value; the monotonic observation number;
// the metadata-only command record — the projection value never enters the
// stored outcome).
func (f *casesFixture) ReadOwningSystem(_ context.Context, command casesAPI.ReadOwningSystemCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("read_owning_system", casesDomain.GuardReadOwnCommand(command.CaseID, command.WorkItemID, command.MaterialKey)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("read_owning_system", command.IdempotencyKey, struct {
		Case     string `json:"case"`
		WorkItem string `json:"work_item"`
		Field    string `json:"material_key"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.MaterialKey})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	// The matrix guard: the field must be declared (the honest unknown of
	// the class — the machine-readable field-not-in-matrix refusal).
	entry, declared := casesDomain.AuthorityEntryOf(command.MaterialKey)
	if !declared {
		return casesAPI.WorkOutcome{}, casesDomain.WorkRuleError{Rule: "field_not_in_matrix", Reason: casesDomain.GuardMaterialFieldInMatrix(command.MaterialKey).Error()}
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known || current.CaseID != command.CaseID {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("read_owning_system", fmt.Errorf("%s", casesDomain.TerminalWorkItemReadText))
	}
	// ONE read attempt of the synthetic owning system (the production
	// adapter of the test contour).
	now := command.Now.Truncate(time.Microsecond)
	reading, err := f.owningReader.Read(context.Background(), casesPort.OwningSystemReadRequest{
		MaterialKey: command.MaterialKey, OwningSystem: entry.OwningSystem, Projection: entry.Projection,
		ExternalReference: command.WorkItemID.String(), CorrelationID: command.CorrelationID, Now: now,
	})
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The REAL guards + the canonical value of the ok read.
	value := reading.Value
	if reading.Status == casesDomain.ObservationStatusOk {
		if value, err = casesDomain.ObservationValue(reading.Value); err != nil {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("read_owning_system", err)
		}
	}
	if err := fixtureWorkRule("read_owning_system", casesDomain.GuardObservationMaterial(reading.Status, reading.Reason, value, reading.ExternalID, reading.ExternalVersion, reading.ObservedAt)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The per-(work item, material field) monotonic observation number.
	observationNo := int64(0)
	for _, observation := range f.observations[command.CaseID] {
		if observation.WorkItemID == command.WorkItemID && observation.MaterialKey == command.MaterialKey && observation.ObservationNo > observationNo {
			observationNo = observation.ObservationNo
		}
	}
	observationNo++
	correlation := command.CorrelationID
	if !correlation.Valid() {
		correlation = f.fixtureObservationID()
	}
	observationID := f.fixtureObservationID()
	f.observations[command.CaseID] = append(f.observations[command.CaseID], casesDomain.Observation{
		TenantID: f.tenant, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ObservationID: observationID, ObservationNo: observationNo,
		MaterialKey: command.MaterialKey, OwningSystem: entry.OwningSystem, Projection: entry.Projection,
		Status: reading.Status, Reason: reading.Reason, Value: value,
		ExternalID: reading.ExternalID, ExternalVersion: reading.ExternalVersion,
		ObservedAt: reading.ObservedAt, ReceivedAt: now,
		ActorID: command.Actor.ID, CorrelationID: correlation, CreatedAt: now,
	})
	outcome = casesAPI.WorkOutcome{
		Command: "read_owning_system", ObservationID: observationID,
		ObservationStatus: reading.Status, ObservationReason: reading.Reason,
		ObservationExternalID: reading.ExternalID, ObservationVersion: reading.ExternalVersion,
	}
	f.storeOutcome("read_owning_system", command.IdempotencyKey, outcome)
	return outcome, nil
}

// fixtureConfirmedFactValueOf resolves the exact confirmed fact version of
// one material key of the case (nil when the case has no confirmed fact of
// the key — the REAL 2-2 derivation of the confirmed exact version).
func (f *casesFixture) fixtureConfirmedFactValueOf(caseID id.UUID, materialKey string) *casesDomain.FactVersion {
	factID, found := f.factAnchors[caseID.String()+"|"+materialKey]
	if !found {
		return nil
	}
	versions := f.factVersions[factID]
	if len(versions) == 0 {
		return nil
	}
	status, _, _ := casesDomain.ConfirmedFactView(versions, f.factConfirms[factID])
	if status != casesDomain.FactStatusConfirmed {
		return nil
	}
	confirmedNo := int64(-1)
	for _, confirmation := range f.factConfirms[factID] {
		if confirmation.VersionNo > confirmedNo {
			confirmedNo = confirmation.VersionNo
		}
	}
	for i := range versions {
		if versions[i].VersionNo == confirmedNo {
			return &versions[i]
		}
	}
	return nil
}

// fixtureObservationView assembles the owning-system section of the case
// view over the REAL derivations: the CURRENT observation per matrix field
// (the maximal number), the freshness state at the fixed view moment of the
// fixture world and the conflict badge against the confirmed exact fact of
// the same material key.
func (f *casesFixture) fixtureObservationView(caseID id.UUID) casesAPI.ObservationView {
	view := casesAPI.ObservationView{
		FieldWords:    append([]string(nil), casesDomain.MaterialFields...),
		MatrixVersion: casesDomain.AuthorityMatrixVersion,
		PanelText:     "Наблюдение — снимок чтения системы-владельца, не подтверждённый факт: значение, источник, внешний ID и время; устаревшее поле перечитайте перед материальным ответом; расхождение с фактом решает человек.",
	}
	now := clockFixture{}.Now()
	for _, anchor := range f.items[caseID] {
		section := casesAPI.ObservationWorkItemView{WorkItemID: anchor.WorkItemID}
		observationsOfItem := []casesDomain.Observation{}
		for _, observation := range f.observations[caseID] {
			if observation.WorkItemID == anchor.WorkItemID {
				observationsOfItem = append(observationsOfItem, observation)
			}
		}
		section.ObservationCount = len(observationsOfItem)
		for _, field := range casesDomain.MaterialFields {
			entry, _ := casesDomain.AuthorityEntryOf(field)
			card := casesAPI.ObservationCard{
				MaterialKey: field, OwningSystem: entry.OwningSystem, Projection: entry.Projection,
				FreshnessSeconds: entry.FreshnessSeconds,
			}
			observationsOfField := []casesDomain.Observation{}
			for _, observation := range observationsOfItem {
				if observation.MaterialKey == field {
					observationsOfField = append(observationsOfField, observation)
				}
			}
			card.Current = casesDomain.CurrentObservation(observationsOfField)
			if card.Current != nil {
				card.State = casesDomain.DeriveObservationState(card.Current.Status, now.Sub(card.Current.ObservedAt), entry.FreshnessSeconds)
				if card.Current.Status == casesDomain.ObservationStatusOk {
					card.ConflictBadge = casesDomain.ConflictBadgeOf(card.Current.Value, f.fixtureConfirmedFactValueOf(caseID, field))
				}
			}
			section.Fields = append(section.Fields, card)
		}
		view.Items = append(view.Items, section)
	}
	return view
}
