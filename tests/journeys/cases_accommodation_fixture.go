package main

import (
	"context"
	"fmt"
	"sort"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.11 fixture: the accommodation record command over the REAL
// domain guard (GuardAccommodationRecord — the key dictionary, the
// flag/valued shape, the channel word of preferred_channel, the bounds
// and the clearing discipline) and the REAL read-time derivations
// (DeriveCurrentAccommodations — the latest revision of every key wins,
// a clearing revision removes the key from the active set;
// AccommodationSnapshotOf — the canonical preferences map of the policy
// input snapshot). The fixture world stores the append-only revision
// rows; the CURRENT set is never stored — every read derives it anew
// (the mirror of the store). The revision number is the fixture-owned
// monotonic truth of the key (the request never carries it — the store
// derives the next number under the locks). The request hash covers the
// WHOLE command material INCLUDING the value and the note: the exact
// repeat answers the stored outcome (the replay), the same key with
// another material is the honest key conflict.

// fixtureAccommodationID generates the next deterministic accommodation
// record id (the counter space of the Story 2.11 surface — the ids never
// collide with the other fixture surfaces).
func (f *casesFixture) fixtureAccommodationID() id.UUID {
	f.accommodationSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(96000+f.accommodationSeq)))
}

// fixtureAccommodationRecords lists the append-only revision rows of one
// case in the canonical store order (the key word, then the monotonic
// revision number — the stable panel history order).
func (f *casesFixture) fixtureAccommodationRecords(caseID id.UUID) []casesDomain.AccommodationRecord {
	records := append([]casesDomain.AccommodationRecord(nil), f.accommodations[caseID]...)
	sort.Slice(records, func(i, j int) bool {
		if records[i].Key != records[j].Key {
			return records[i].Key < records[j].Key
		}
		return records[i].Revision < records[j].Revision
	})
	return records
}

// fixtureAccommodationCurrent derives the read-time preferences state of
// one case over the REAL derivation (the latest revision per key; the
// cleared keys leave the active set but stay in the state with the
// honest mark — the snapshot truth of the policy input).
func (f *casesFixture) fixtureAccommodationCurrent(caseID id.UUID) casesDomain.CurrentAccommodations {
	return casesDomain.DeriveCurrentAccommodations(f.accommodations[caseID])
}

// RecordAccommodation records one revision of one communication
// preference of the case: the REAL dictionary guard before any write
// (the key dictionary, the flag/valued shape, the channel word, the
// bounds, the clearing discipline — the domain truth, never a convenient
// default), the idempotent command flow with the hash of the WHOLE
// material (the value and the note included), the honest refusal of an
// unknown or a terminal case and the append-only landing of the NEXT
// monotonic revision of the key (the request never carries the number).
// The fixture actor is the review-scoped principal of the journey world
// — the scope gate of the production contour passes by construction;
// the audit-only leg lives in the integration matrix.
func (f *casesFixture) RecordAccommodation(_ context.Context, command casesAPI.RecordAccommodationCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if command.TenantID != f.tenant {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if err := fixtureWorkRule("record_accommodation", casesDomain.GuardAccommodationRecord(command.AccommodationKey, command.AccommodationValue, command.Cleared, command.Note)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the WHOLE material INCLUDING the value and
	// the note (the replay of the exact repeat answers the stored
	// outcome; the same key with another material is the honest conflict).
	outcome, fresh, err := f.workCommandFlow("record_accommodation", command.IdempotencyKey, struct {
		Case  string `json:"case"`
		Key   string `json:"accommodation_key"`
		Value string `json:"value"`
		Clear bool   `json:"cleared"`
		Note  string `json:"note"`
	}{command.CaseID.String(), command.AccommodationKey, command.AccommodationValue, command.Cleared, command.Note})
	if err != nil || !fresh {
		return outcome, err
	}
	material := f.material[command.CaseID]
	if material == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if material.cases.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_accommodation", fmt.Errorf("Терминальное обращение не принимает записей предпочтений связи: обслуживание завершено — записывать нечего."))
	}
	// The NEXT monotonic revision of the key — the fixture-owned truth
	// under the world lock (the request never carries the number; the
	// append-only history never rewrites).
	revision := int64(0)
	for _, record := range f.accommodations[command.CaseID] {
		if record.Key == command.AccommodationKey && record.Revision > revision {
			revision = record.Revision
		}
	}
	revision++
	recordID := f.fixtureAccommodationID()
	f.accommodations[command.CaseID] = append(f.accommodations[command.CaseID], casesDomain.AccommodationRecord{
		TenantID: f.tenant, CaseID: command.CaseID, RecordID: recordID,
		Key: command.AccommodationKey, Revision: revision,
		Value: command.AccommodationValue, Cleared: command.Cleared, Note: command.Note,
		ActorID: command.Actor.ID, RecordedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{
		Command: "record_accommodation", AccommodationRecordID: recordID,
		AccommodationKey: command.AccommodationKey, AccommodationCleared: command.Cleared,
	}
	f.storeOutcome("record_accommodation", command.IdempotencyKey, outcome)
	return outcome, nil
}

// fixtureAccuracyGate is the reply accuracy gate of the Story 2.11
// (the app salesAccuracyGate mirror — the frozen ladder: ownership 2.4
// → mode 2.5 → disclosure 2.5 → policy 2.6 → objection 2.8 → accuracy
// 2.11 → write; the SALES class only): the CURRENT (the maximal number)
// outcome of the Work Item decides which words are honest — an outcome
// with active alternatives REQUIRES the qualifier word of its active
// set; an outcome without alternatives requires the EMPTY word (the
// field has exactly one meaning — the objection_step mirror of 2.8).
func (f *casesFixture) fixtureAccuracyGate(command casesAPI.SendClientReplyCommand) error {
	outcomesOfItem := []casesDomain.PolicyOutcome{}
	for _, outcome := range f.policyOutcomes[command.CaseID] {
		if outcome.WorkItemID == command.WorkItemID {
			outcomesOfItem = append(outcomesOfItem, outcome)
		}
	}
	current := casesDomain.CurrentPolicyOutcome(outcomesOfItem)
	if current == nil {
		// No outcome: the policy gate has already refused (its refusal
		// fires EARLIER — the frozen order never changes); the accuracy
		// gate of the empty outcome demands the empty word.
		return casesDomain.GuardSalesReplyAccuracy(nil, command.AccuracyQualifier)
	}
	return casesDomain.GuardSalesReplyAccuracy(current.ActiveAlternatives, command.AccuracyQualifier)
}

// fixtureAccommodationView assembles the «Предпочтения связи» section of
// the case view (the app accommodationView mirror): the CURRENT set
// derives read-time over the REAL derivation (the cleared keys stay
// visible with the honest mark), the full append-only revision history
// in the canonical order, the key dictionary of the record form, the
// channel dictionary of the preferred_channel value and the fixed panel
// words. The preferences are an OPERATIONAL setting: never facts, never
// a diagnosis, never an identity proof.
func (f *casesFixture) fixtureAccommodationView(caseID id.UUID) casesAPI.AccommodationView {
	records := f.fixtureAccommodationRecords(caseID)
	current := casesDomain.DeriveCurrentAccommodations(records)
	view := casesAPI.AccommodationView{
		Current:      make([]casesAPI.AccommodationStateView, 0, len(current)),
		History:      make([]casesAPI.AccommodationRevisionView, 0, len(records)),
		KeyWords:     append([]string(nil), casesDomain.AccommodationKeys...),
		ChannelWords: append([]string(nil), casesDomain.ChannelOrder...),
		RecordsCount: len(records),
		PanelText:    "Предпочтения связи — операционная настройка обслуживания, не факт и не доказательство: append-only ревизии, текущий набор — производная чтения; значения (канал, время) живут только в строках таблицы и никогда не покидают её в outbox.",
	}
	for _, key := range current.ActiveKeys() {
		state := current[key]
		view.Current = append(view.Current, casesAPI.AccommodationStateView{
			Key: state.Key, Revision: state.Revision, Cleared: state.Cleared, Value: state.Value,
		})
	}
	for _, record := range records {
		view.History = append(view.History, casesAPI.AccommodationRevisionView{
			Key: record.Key, Revision: record.Revision, Value: record.Value,
			Cleared: record.Cleared, Note: record.Note,
			ActorID: record.ActorID, RecordedAt: record.RecordedAt,
		})
	}
	return view
}
