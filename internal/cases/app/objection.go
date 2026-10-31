package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// The Story 2.8 command, the objection panel view and the sales reply gate
// ---------------------------------------------------------------------------
//
// The record command is idempotent by the tenant-scoped key with the stored
// outcome (the 2-2..2-7 precedent: the replay answer comes before the state
// guards); the request hash covers the Work Item anchor, the turn source,
// the dictionary words and the spans (the whole command material). The gate
// order is frozen: the rights gate (review OR configuration — the M1
// precedent of 2.5/2.7) fires BEFORE every material guard and refuses with
// the forbidden word (the honest 403 of the scope sentence); the dictionary
// guard refuses with the honest 400; the terminal Work Item refuses
// honestly (the write-command boundary of 2-2); the turn source must be an
// ACCEPTED TEXT event of the case (the voice turn refuses honestly — the
// excerpt markup of the voice contour belongs to the later slices); every
// span must be the byte-exact substring of the sealed payload (an invented
// excerpt is impossible — AC-4). The record lands append-only with the
// per-Work-Item monotonic number under the case + work item advisory
// locks; the outbox event carries the metadata only — the span text never
// enters the envelope (the client words already live in the sealed payload
// of 2-1). The record NEVER changes the mode: the raise stays the
// propose_mode command of 2.5 — the record only marks the trigger signal.

// objectionScopeText is the honest scope refusal of the record command (the
// shared «review или configuration» vocabulary of the 2.5..2.7 rights
// gates): the app word travels inside the forbidden error only — the
// transport renders its own sentence for the wire and the form.
const objectionScopeText = "Команды записи возражений требуют полномочия review или configuration: участники только с audit не размечают возражения и выдержки клиента."

// RecordObjection records one client objection of the Work Item: the
// operator with the review OR the configuration scope (the gate fires
// BEFORE the material guards and refuses with the 403 forbidden word — an
// audit-only member never writes), the dictionary words (the type, the
// signal), the accepted TEXT turn of the case as the span source and the
// verbatim spans (the byte-exact excerpts of the sealed payload). The
// record is append-only with the monotonic number; the persuasion state
// (the refusal budget, the stopped word) is the read-time derivation —
// nothing is ever mutated.
func (s *Service) RecordObjection(ctx context.Context, command casesAPI.RecordObjectionCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration (the M1 precedent
	// of 2.5/2.7); an audit-only member reads the case surface but never
	// records the objections. The refusal is the forbidden word of the
	// frozen matrix row (the honest 403 — the scope sentence, never the
	// 400 rule word). A non-member is the plain 404 (the scope
	// resolution itself).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, objectionScopeText)
	}
	// The dictionary guard: the type, the signal and the span discipline
	// (the honest 400 with the dictionary words in the text).
	if err := rule("record_objection", casesDomain.GuardObjectionRecord(command.ObjectionType, command.ClientSignal, command.Spans)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the whole command material (the Work Item
	// anchor, the turn source, the dictionary words and the spans): the
	// replay of the exact repeat returns the stored outcome; the same
	// key with another material is the honest key conflict.
	spansJSON, err := json.Marshal(command.Spans)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	hash := requestHash(struct {
		Case     string   `json:"case"`
		WorkItem string   `json:"work_item"`
		TurnRef  string   `json:"turn_ref"`
		Type     string   `json:"objection_type"`
		Signal   string   `json:"client_signal"`
		Spans    []string `json:"spans"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.TurnRef.String(),
		command.ObjectionType, command.ClientSignal, command.Spans})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_objection", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The Work Item of the record scope: it must exist in this case and
	// be alive — the terminal states refuse honestly (the write-command
	// boundary of 2-2; the merged item lives on in its target).
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, command.CaseID, []id.UUID{command.WorkItemID})
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current, ok := currents[command.WorkItemID]
	if !ok {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, rule("record_objection", fmt.Errorf("%s", casesDomain.TerminalWorkItemObjectionText))
	}
	// The turn source: an event of THIS case — accepted and TEXT (the
	// span carrier; the honest refusal words of the boundaries — a
	// missing event, a quarantined one or a voice turn never becomes a
	// span source).
	event, err := s.repository.ReadEventOfCase(ctx, scope, command.CaseID, command.TurnRef)
	if err != nil {
		return casesAPI.WorkOutcome{}, rule("record_objection", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
	}
	if event.State != casesDomain.EventStateAccepted {
		return casesAPI.WorkOutcome{}, rule("record_objection", fmt.Errorf("Источник возражения не принят: разметка выдержек возможна только на принятых событиях кейса."))
	}
	if event.Modality != casesDomain.ModalityText {
		return casesAPI.WorkOutcome{}, rule("record_objection", fmt.Errorf("Разметка выдержек голосовых сообщений — вне текущего срез: источник возражения должен быть текстовым событием."))
	}
	// The span honesty: every span is the byte-exact substring of the
	// sealed payload of the turn source (an invented excerpt is
	// impossible — AC-4). The payload read is the review/audit boundary
	// of the command scope.
	sealed, err := s.repository.ReadPayload(ctx, scope, command.CaseID, command.TurnRef)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	for _, span := range command.Spans {
		if !casesDomain.SpanInPayload(span, sealed.Payload) {
			return casesAPI.WorkOutcome{}, rule("record_objection", fmt.Errorf("Выдержка не входит в слова клиента: «%s» не является дословным фрагментом payload события-источника. Выдуманные выдержки невозможны.", span))
		}
	}
	// The generated ids of the record and the outbox event.
	objectionID, err := s.ids.New(command.Now)
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
		Command: "record_objection", ObjectionID: objectionID,
		ObjectionType: command.ObjectionType, ObjectionSignal: command.ClientSignal,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_objection", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		ObjectionID: objectionID, TurnRef: command.TurnRef,
		ObjectionType: command.ObjectionType, ClientSignal: command.ClientSignal,
		ObjectionSpans: spansJSON, CorrelationID: correlation,
		OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// salesObjectionGate is the app-side objection gate of the sales reply (the
// 2-6 replyPolicyGate precedent — the frozen ladder: ownership 2-4 → mode
// 2-5 → disclosure 2-5 → policy 2-6 → objection 2-8 → write; the SALES
// class only): the persuasion state of the Work Item derives from the
// append-only rows; the stopped persuasion refuses with the allowed
// endings; the CURRENT objection makes the playbook step mandatory and
// disciplined (the first linked answer — always acknowledge_concern). The
// passing gate resolves the objection id the reply record references (the
// request never carries it — the server-side truth).
func (s *Service) salesObjectionGate(ctx context.Context, scope tenancy.Context, caseID, workItemID id.UUID, step string) (id.UUID, error) {
	state, err := s.objectionStateOf(ctx, scope, caseID, workItemID)
	if err != nil {
		return id.UUID(""), err
	}
	if err := casesDomain.GuardSalesReplyAgainstObjections(state, step); err != nil {
		return id.UUID(""), err
	}
	if state.Current == nil {
		return id.UUID(""), nil
	}
	return state.Current.ObjectionID, nil
}

// objectionStateOf derives the persuasion state of one Work Item from the
// read-only rows (the pre-transaction honest read; the store re-validates
// the same derivation under the locks).
func (s *Service) objectionStateOf(ctx context.Context, scope tenancy.Context, caseID, workItemID id.UUID) (casesDomain.ObjectionState, error) {
	records, err := s.repository.ReadObjections(ctx, scope, caseID)
	if err != nil {
		return casesDomain.ObjectionState{}, err
	}
	recordsOfItem := []casesDomain.ObjectionRecord{}
	for _, record := range records {
		if record.WorkItemID == workItemID {
			recordsOfItem = append(recordsOfItem, record)
		}
	}
	links, err := s.repository.ReadLinkedReplies(ctx, scope, caseID)
	if err != nil {
		return casesDomain.ObjectionState{}, err
	}
	return casesDomain.DeriveObjectionState(recordsOfItem, links), nil
}

// objectionView assembles the objection section of the case view: the
// per-Work-Item sections with the record history (the CURRENT objection is
// the maximal number), the lifetime persuasion budget over the fixed
// threshold, the stopped word (with the allowed endings — the panel
// sentence), the acknowledged derivation of the current objection, the
// next-step words of the current playbook and the dictionary words of the
// forms. The reads degrade honestly — the same view scope.
func (s *Service) objectionView(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail) (casesAPI.ObjectionView, error) {
	records, err := s.repository.ReadObjections(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.ObjectionView{}, err
	}
	links, err := s.repository.ReadLinkedReplies(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.ObjectionView{}, err
	}
	view := casesAPI.ObjectionView{
		TypeWords:       append([]string(nil), casesDomain.ObjectionTypes...),
		SignalWords:     append([]string(nil), casesDomain.ClientSignals...),
		StepWords:       append([]string(nil), casesDomain.PlaybookSteps...),
		PlaybookVersion: casesDomain.ObjectionPlaybookVersion,
		Threshold:       casesDomain.PersuasionThresholdV1,
		PanelText:       "Возражение — слова клиента со ссылкой на принятый turn: тип, сигнал и дословные выдержки. Бюджет убеждения — монотонный счёт отказов; при остановке разрешены только вежливое завершение, запрос приватности и передача человеку.",
		Items:           make([]casesAPI.ObjectionWorkItemView, 0, len(detail.WorkItems)),
	}
	for _, item := range detail.WorkItems {
		section := casesAPI.ObjectionWorkItemView{
			WorkItemID: item.WorkItemID, Threshold: casesDomain.PersuasionThresholdV1,
			LinkedReplies: []casesDomain.ReplyLink{},
		}
		recordsOfItem := []casesDomain.ObjectionRecord{}
		for _, record := range records {
			if record.WorkItemID == item.WorkItemID {
				recordsOfItem = append(recordsOfItem, record)
			}
		}
		section.ObjectionCount = len(recordsOfItem)
		state := casesDomain.DeriveObjectionState(recordsOfItem, links)
		section.Refusals = state.Refusals
		section.Stopped = state.Stopped
		section.Acknowledged = state.Acknowledged
		if state.Current != nil {
			section.Current = &casesAPI.ObjectionCard{
				Record: state.Current, Refusals: state.Refusals, Acknowledged: state.Acknowledged,
			}
			// The next-step words of the current playbook (the
			// reply form hint: the allowed steps of the type).
			section.NextSteps = casesDomain.PlaybookStepsOf(state.Current.ObjectionType)
			for _, link := range links {
				if link.ObjectionID == state.Current.ObjectionID {
					section.LinkedReplies = append(section.LinkedReplies, link)
				}
			}
		}
		view.Items = append(view.Items, section)
	}
	return view, nil
}
