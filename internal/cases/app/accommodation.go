package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// The Story 2.11 command, the accuracy gate of the sales reply and the
// accommodation view
// ---------------------------------------------------------------------------
//
// The record command is idempotent by the tenant-scoped key with the
// stored outcome (the 2.2..2.10 precedent: the replay answer comes BEFORE
// the state guards); the request hash covers the WHOLE command material
// INCLUDING the value and the note (the same key with another material is
// the honest key conflict). The rights gate (review OR configuration —
// the M1 precedent of 2.5..2.8) fires BEFORE every material guard and
// refuses with the forbidden word (the honest 403 of the scope sentence);
// the dictionary guard refuses with the honest 400 (the key dictionary,
// the flag/valued shape, the channel word, the bounds and the clearing
// discipline — the domain truth before SQL); the terminal case refuses
// honestly (the v1 dictionary carries the open word only). The record
// lands append-only with the NEXT monotonic revision of the key derived
// under the case + conversation locks (the request never carries the
// number); the outbox event carries the metadata only — the VALUE (the
// channel word, the availability line) and the NOTE never enter the
// envelope (the operational rows stay in the table).

// accommodationScopeText is the honest scope refusal of the record
// command (the shared «review или configuration» vocabulary of the
// 2.5..2.8 rights gates): the app word travels inside the forbidden error
// only — the transport renders its own sentence for the wire and the
// form.
const accommodationScopeText = "Команды записи предпочтений связи требуют полномочия review или configuration: участники только с audit не размечают предпочтения клиента."

// RecordAccommodation records one revision of one communication preference
// of the case: the operator with the review OR the configuration scope
// (the gate fires BEFORE the material guards and refuses with the 403
// forbidden word — an audit-only member never writes), the case anchor,
// the preference key word, the valued material of the valued keys, the
// clearing mark and the operator note. The record is append-only with the
// next monotonic revision of the key (the server-side truth under the
// locks); the current set is the read-time derivation — nothing is ever
// mutated; the preferences are NEVER facts (no confirmation statuses, no
// fact versions, no facts digest).
func (s *Service) RecordAccommodation(ctx context.Context, command casesAPI.RecordAccommodationCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration (the M1 precedent of
	// 2.5..2.8); an audit-only member reads the case surface but never
	// records the preferences. The refusal is the forbidden word of the
	// frozen matrix row (the honest 403 — the scope sentence, never the
	// 400 rule word). A non-member is the plain 404 (the scope resolution
	// itself).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, accommodationScopeText)
	}
	// The dictionary guard: the key dictionary, the flag/valued shape, the
	// channel word, the bounds and the clearing discipline (the honest 400
	// with the dictionary words in the text — the domain truth before SQL;
	// the table CHECK words are the backstops).
	if err := rule("record_accommodation", casesDomain.GuardAccommodationRecord(command.AccommodationKey, command.AccommodationValue, command.Cleared, command.Note)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the WHOLE command material INCLUDING the
	// value and the note (the replay of the exact repeat returns the
	// stored outcome; the same key with another material is the honest key
	// conflict).
	hash := requestHash(struct {
		Case    string `json:"case"`
		Key     string `json:"accommodation_key"`
		Value   string `json:"value"`
		Cleared bool   `json:"cleared"`
		Note    string `json:"note"`
	}{command.CaseID.String(), command.AccommodationKey, command.AccommodationValue, command.Cleared, command.Note})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_accommodation", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The case anchor: the case must exist in the tenant scope (the honest
	// 404 of the non-participant path; the RLS curtain of the foreign
	// tenant) and stay open (the terminal backstop — the v1 dictionary
	// carries the open word only; the store re-validates under the locks).
	detail, err := s.repository.ReadCase(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if detail.Case.State != casesDomain.CaseStateOpen {
		return casesAPI.WorkOutcome{}, rule("record_accommodation", fmt.Errorf("%s", casesStoreTerminalCaseText))
	}
	// The conversation of the lock plan: the case's own conversation (the
	// unified dialog anchor — the SAME sorted plan of the mode/policy
	// commands; the preferences read of the evaluation and the record
	// write serialize on the identical key set).
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The generated ids of the record and the outbox event.
	recordID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outboxEventID, err := s.ids.New(command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	correlation := command.CorrelationID
	if !correlation.Valid() {
		if correlation, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
	}
	outcome := casesAPI.WorkOutcome{
		Command: "record_accommodation", AccommodationRecordID: recordID,
		AccommodationKey: command.AccommodationKey, AccommodationCleared: command.Cleared,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_accommodation", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, ConversationID: conversation.ConversationID,
		AccommodationRecordID: recordID, AccommodationKey: command.AccommodationKey,
		AccommodationValue: command.AccommodationValue, AccommodationCleared: command.Cleared,
		AccommodationNote: command.Note, CorrelationID: correlation,
		OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// casesStoreTerminalCaseText mirrors the store refusal of the terminal
// case (the honest boundary of the write commands — the v1 dictionary
// carries the open word only).
const casesStoreTerminalCaseText = "Терминальное обращение не принимает записей предпочтений связи: обслуживание завершено — записывать нечего."

// accommodationStateOf derives the CURRENT preferences state of one case
// from the read-only rows (the pre-transaction honest read; the store
// re-derives the same truth under the locks of the evaluation).
func (s *Service) accommodationStateOf(ctx context.Context, scope tenancy.Context, caseID id.UUID) (casesDomain.CurrentAccommodations, error) {
	records, err := s.repository.ReadAccommodations(ctx, scope, caseID)
	if err != nil {
		return nil, err
	}
	return casesDomain.DeriveCurrentAccommodations(records), nil
}

// salesAccuracyGate is the app-side accuracy gate of the sales reply (the
// salesObjectionGate precedent — the frozen ladder: ownership 2.4 → mode
// 2.5 → disclosure 2.5 → policy 2.6 → objection 2.8 → accuracy 2.11 →
// write; the SALES class only): the CURRENT (the maximal number) outcome
// of the Work Item decides which words are honest in this context — an
// outcome with active alternatives REQUIRES the qualifier word of its
// active set (the absence of media never becomes an invented exact
// result); an outcome without alternatives requires the EMPTY word (the
// field has exactly one meaning — the objection_step_foreign mirror of
// 2.8). The store re-validates under the locks.
func (s *Service) salesAccuracyGate(ctx context.Context, scope tenancy.Context, caseID, workItemID id.UUID, qualifier string) error {
	outcomes, err := s.repository.ReadPolicyOutcomes(ctx, scope, caseID)
	if err != nil {
		return err
	}
	outcomesOfItem := []casesDomain.PolicyOutcome{}
	for _, outcome := range outcomes {
		if outcome.WorkItemID == workItemID {
			outcomesOfItem = append(outcomesOfItem, outcome)
		}
	}
	current := casesDomain.CurrentPolicyOutcome(outcomesOfItem)
	if current == nil {
		// No outcome: the policy gate has already refused (its refusal
		// fires EARLIER — the frozen order never changes); the accuracy
		// gate of the empty outcome demands the empty word.
		return casesDomain.GuardSalesReplyAccuracy(nil, qualifier)
	}
	return casesDomain.GuardSalesReplyAccuracy(current.ActiveAlternatives, qualifier)
}

// accommodationView assembles the «Предпочтения связи» section of the
// case view: the CURRENT set (the read-time derivation — the cleared keys
// stay visible with the honest mark), the full append-only revision
// history, the key dictionary of the record form, the channel dictionary
// of the preferred_channel value and the fixed panel words. The reads
// degrade honestly — the same view scope.
func (s *Service) accommodationView(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail) (casesAPI.AccommodationView, error) {
	records, err := s.repository.ReadAccommodations(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.AccommodationView{}, err
	}
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
	// The honest history: every revision row including the cleared ones
	// (the operator surface renders the clearing truth; the append-only
	// audit).
	for _, record := range records {
		view.History = append(view.History, casesAPI.AccommodationRevisionView{
			Key: record.Key, Revision: record.Revision, Value: record.Value,
			Cleared: record.Cleared, Note: record.Note,
			ActorID: record.ActorID, RecordedAt: record.RecordedAt,
		})
	}
	return view, nil
}
