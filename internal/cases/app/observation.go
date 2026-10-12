package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// The Story 2.7 command and the observation view
// ---------------------------------------------------------------------------
//
// The read command is idempotent by the tenant-scoped key with the stored
// outcome (the 2-2..2-6 precedent: the replay answer comes BEFORE the
// material guards); the request hash covers the work item anchor and the
// material field (the whole command material). The gate order is frozen:
// the rights gate (review OR configuration — the M1 precedent of 2-5) fires
// BEFORE every material guard; the field outside the matrix is the honest
// 400 field-not-in-matrix (the unknown class — no row is ever written); the
// terminal Work Item refuses honestly; the missing reader port refuses
// honestly BEFORE any record (the «коннектор без верификатора» precedent).
// The port answer materializes as the append-only row under the case + work
// item advisory locks with the per-(work item, material field) monotonic
// number; the outbox event carries the metadata only — the projection value
// never enters the envelope.

// WithOwningSystemReader wires the owning-system read port. Without it the
// read command honestly refuses — an "ok" owning-system contour is never
// assumed (the production wiring refuses honestly until the real providers
// arrive, AG-4).
func (s *Service) WithOwningSystemReader(reader casesPort.OwningSystemReader) *Service {
	s.owningReader = reader
	return s
}

// ReadOwningSystem reads the CURRENT value of one material field of the
// authority matrix from the owning system of the Work Item scope: the
// operator with the review OR the configuration scope (the gate fires
// BEFORE the material guards — an audit-only member never writes), the
// field must be declared in the matrix (otherwise the honest unknown of the
// class — the machine-readable field-not-in-matrix refusal), the Work Item
// must be alive, the reader answers ONCE (the fixed deadline word — no
// retry exists) and the observation lands append-only with the monotonic
// number. A failed or unknown read is an honest row WITHOUT a value — the
// assumed value is never written (AC-6).
func (s *Service) ReadOwningSystem(ctx context.Context, command casesAPI.ReadOwningSystemCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration (the frozen Boundaries
	// word — the M1 precedent of 2-5); an audit-only member reads the case
	// surface but never issues the owning-system reads.
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, rule("read_owning_system", fmt.Errorf("%s", ModeWriteScopeText))
	}
	if err := rule("read_owning_system", casesDomain.GuardReadOwnCommand(command.CaseID, command.WorkItemID, command.MaterialKey)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The matrix guard: the field must be declared — the honest unknown of
	// the class, the machine-readable 400 field-not-in-matrix with the
	// field dictionary in the text.
	entry, declared := casesDomain.AuthorityEntryOf(command.MaterialKey)
	if !declared {
		return casesAPI.WorkOutcome{}, casesDomain.WorkRuleError{Rule: "field_not_in_matrix", Reason: casesDomain.GuardMaterialFieldInMatrix(command.MaterialKey).Error()}
	}
	// The request hash covers the whole command material (the work item
	// anchor and the material field): the replay of the exact repeat returns
	// the stored outcome; the same key with another material is the honest
	// key conflict.
	hash := requestHash(struct {
		Case     string `json:"case"`
		WorkItem string `json:"work_item"`
		Field    string `json:"material_key"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.MaterialKey})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "read_owning_system", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The Work Item of the read scope: it must exist in this case and be
	// alive — the terminal states refuse honestly (the merged item lives on
	// in its target).
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, command.CaseID, []id.UUID{command.WorkItemID})
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current, ok := currents[command.WorkItemID]
	if !ok {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, rule("read_owning_system", fmt.Errorf("%s", casesDomain.TerminalWorkItemReadText))
	}
	// The reader port must be wired: the honest refusal fires BEFORE any
	// record (an "ok" contour is never assumed — the 2-1 precedent).
	if s.owningReader == nil {
		return casesAPI.WorkOutcome{}, rule("read_owning_system", fmt.Errorf("контур owning-system чтения не сконфигурирован: команда чтения честно отклоняется, значение «ok» не предполагается"))
	}
	// The correlation of the command trace (the transport correlation or
	// the fresh id of the command moment).
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	now := command.Now.UTC().Truncate(time.Microsecond)
	// ONE read attempt of the owning system (the fixed deadline word — no
	// retry, no circuit breaker, no rate limit).
	reading, err := s.owningReader.Read(ctx, casesPort.OwningSystemReadRequest{
		Scope: scope, MaterialKey: command.MaterialKey, OwningSystem: entry.OwningSystem, Projection: entry.Projection,
		ExternalReference: command.WorkItemID.String(), CorrelationID: correlation, Now: now,
	})
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The port answer guard: the honest adapter answers in the dictionary;
	// a failed/unknown read never carries a value (the fail-closed basis
	// against a misbehaving adapter). The ok value is canonicalized into the
	// deterministic bytes (the honest CHECK of the jsonb column).
	value := reading.Value
	if reading.Status == casesDomain.ObservationStatusOk {
		if value, err = casesDomain.ObservationValue(reading.Value); err != nil {
			return casesAPI.WorkOutcome{}, rule("read_owning_system", err)
		}
	}
	if err := rule("read_owning_system", casesDomain.GuardObservationMaterial(reading.Status, reading.Reason, value, reading.ExternalID, reading.ExternalVersion, reading.ObservedAt)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	observationID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "read_owning_system", ObservationID: observationID,
		ObservationStatus: reading.Status, ObservationReason: reading.Reason,
		ObservationExternalID: reading.ExternalID, ObservationVersion: reading.ExternalVersion,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "read_owning_system", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		MaterialKey: command.MaterialKey, ObservationID: observationID,
		OwningSystem: entry.OwningSystem, Projection: entry.Projection,
		ReadStatus: reading.Status, ReadReason: reading.Reason, ReadValue: value,
		ExternalID: reading.ExternalID, ExternalVersion: reading.ExternalVersion,
		ObservedAt: reading.ObservedAt, ReceivedAt: now, CorrelationID: correlation,
		OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: now,
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// observationView assembles the owning-system section of the case view: the
// cards of every matrix field per Work Item — the CURRENT observation (the
// maximal number) with the read-time freshness derivation (now = the clock
// of the view) and the conflict badge against the confirmed exact fact of
// the same material key (only the confirmed facts participate — the 2-2
// derivation). The per-field cards are isolated: the failure of one owning
// system never masks another field (AC-9).
func (s *Service) observationView(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail) (casesAPI.ObservationView, error) {
	observations, err := s.repository.ReadObservations(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.ObservationView{}, err
	}
	// The confirmed exact fact values of the case (the badge basis — only
	// the confirmed facts of the same material key participate).
	facts, err := s.repository.ReadFactsOfCase(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.ObservationView{}, err
	}
	confirmedValues := map[string]*casesDomain.FactVersion{}
	for i := range facts {
		if confirmed := confirmedFactValueOf(facts[i]); confirmed != nil {
			confirmedValues[facts[i].Anchor.MaterialKey] = confirmed
		}
	}
	now := s.clock.Now()
	view := casesAPI.ObservationView{
		FieldWords:    append([]string(nil), casesDomain.MaterialFields...),
		MatrixVersion: casesDomain.AuthorityMatrixVersion,
		PanelText:     "Наблюдение — снимок чтения системы-владельца, не подтверждённый факт: значение, источник, внешний ID и время; устаревшее поле перечитайте перед материальным ответом; расхождение с фактом решает человек.",
		Items:         make([]casesAPI.ObservationWorkItemView, 0, len(detail.WorkItems)),
	}
	for _, item := range detail.WorkItems {
		section := casesAPI.ObservationWorkItemView{WorkItemID: item.WorkItemID}
		observationsOfItem := []casesDomain.Observation{}
		for _, observation := range observations {
			if observation.WorkItemID == item.WorkItemID {
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
				// The read-time derivation: the age is measured from the
				// provider-observed time (never the receive time).
				age := now.Sub(card.Current.ObservedAt)
				card.State = casesDomain.DeriveObservationState(card.Current.Status, age, entry.FreshnessSeconds)
				if card.Current.Status == casesDomain.ObservationStatusOk {
					// The conflict badge: the byte comparison with the
					// confirmed exact fact value (no auto-resolution — the
					// word for the panel only).
					card.ConflictBadge = casesDomain.ConflictBadgeOf(card.Current.Value, confirmedValues[field])
				}
			}
			section.Fields = append(section.Fields, card)
		}
		view.Items = append(view.Items, section)
	}
	return view, nil
}

// confirmedFactValueOf resolves the exact confirmed fact version of one
// fact material (nil when the fact is not confirmed): the version of the
// LATEST confirmation (DeriveFactStatus confirms the value of that exact
// version) — the badge compares the bytes of THAT version.
func confirmedFactValueOf(material casesPort.FactMaterial) *casesDomain.FactVersion {
	status, _, _ := casesDomain.ConfirmedFactView(material.Versions, material.Confirmations)
	if status != casesDomain.FactStatusConfirmed {
		return nil
	}
	confirmedNo := int64(-1)
	for _, confirmation := range material.Confirmations {
		if confirmation.VersionNo > confirmedNo {
			confirmedNo = confirmation.VersionNo
		}
	}
	for i := range material.Versions {
		if material.Versions[i].VersionNo == confirmedNo {
			return &material.Versions[i]
		}
	}
	return nil
}
