package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/artemiosu/staffy/internal/adapters/observationsyn"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.9 fixture: the three commitment commands over the REAL domain
// functions (GuardCreateCommitment, GuardConfirmEvidence,
// GuardConfirmReadiness, GuardDeclineCommitment, DeriveCommitmentReadiness,
// DeriveConfirmationValidity, CommitmentToken) — the fixture world stores
// the append-only version rows, the confirmations, the declines and the
// presentation links; the readiness, the validity and the price freshness
// are computed at every read (the REAL derivations, never stored states).

// fixtureCommitmentKey is the deterministic 32-byte key of the content
// binding of the fixture contour (the journeys assert the token PRESENCE —
// the bytes never travel).
var fixtureCommitmentKey = repeatByte(0x2f, 32)

func repeatByte(fill byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = fill
	}
	return out
}

// fixtureCommitmentID generates the next deterministic commitment-surface
// id (the version row, the confirmation and the decline share the counter
// space — the ids never collide).
func (f *casesFixture) fixtureCommitmentID() id.UUID {
	f.commitmentSeq++
	return id.MustParse("01994f64-6c00-7000-8000-" + digit12(int(92000+f.commitmentSeq)))
}

// ScriptPriceQuote scripts the synthetic owning system's price answer of
// ONE work item reference: the per-reference scenario of the production
// adapter (observationsyn) carries the canonical price bytes of the amount
// — the journeys commitment chain walks the honest price gate (the fresh
// observation with the byte-equal canonical value) without touching the
// shared field default of the observation chain (the fixture-only toggle,
// the /fixture/step-up precedent).
func (f *casesFixture) ScriptPriceQuote(workItem string, amountCents int64) error {
	workItemID, err := id.Parse(workItem)
	if err != nil {
		return err
	}
	if amountCents <= 0 {
		return fmt.Errorf("цена — положительные копейки")
	}
	value, err := casesDomain.PriceValueJSON(casesDomain.CommitmentPrice{
		Kind: casesDomain.CommitmentPriceKindAmount, AmountCents: amountCents,
	})
	if err != nil {
		return err
	}
	f.owningReader.ScriptFor(casesDomain.MaterialFieldPrice, workItemID.String(), observationsyn.Scenario{
		Word: observationsyn.ScenarioOK, Value: value,
	})
	return nil
}

// fixturePriceObservation resolves the CURRENT price observation of one
// Work Item of the case (the freshness gate basis — the REAL derivation
// inputs of 2.7).
func (f *casesFixture) fixturePriceObservation(caseID, workItemID id.UUID) *casesDomain.Observation {
	observationsOfField := []casesDomain.Observation{}
	for _, observation := range f.observations[caseID] {
		if observation.WorkItemID == workItemID && observation.MaterialKey == casesDomain.MaterialFieldPrice {
			observationsOfField = append(observationsOfField, observation)
		}
	}
	return casesDomain.CurrentObservation(observationsOfField)
}

// CreateCommitment creates one version of a commitment over the REAL
// guards: the dictionary words, the material terms, the action set, the
// limits, the future validity; the offered version with the KNOWN price
// passes the observation gate (the CURRENT ok+fresh reading of the field
// with the byte-equal canonical value — AC-3); the version number is the
// monotonic ladder; the content token binds the EXACT material bytes.
func (f *casesFixture) CreateCommitment(_ context.Context, command casesAPI.CreateCommitmentCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if err := fixtureWorkRule("create_commitment", casesDomain.GuardCreateCommitment(casesDomain.CreateCommitmentCommand{
		Status: command.Status, Terms: command.Terms, ActionSet: command.ActionSet,
		ValidUntil: command.ValidUntil, ConsumptionLimit: command.ConsumptionLimit,
		MaterialUnknowns: command.MaterialUnknowns,
	}, command.Now)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	conditions, err := casesDomain.MaterialTermsJSON(command.Terms)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("create_commitment", command.IdempotencyKey, struct {
		Case     string `json:"case"`
		WorkItem string `json:"work_item"`
		Id       string `json:"commitment_id"`
		Status   string `json:"status"`
		Terms    []byte `json:"terms"`
		Valid    int64  `json:"valid_until_unix"`
		Limit    int    `json:"consumption_limit"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.CommitmentID.String(), command.Status, conditions, command.ValidUntil.Unix(), command.ConsumptionLimit})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	// The Story 2.13 freeze gate (AFTER the dictionary guards and the
	// idempotency replay — an exact repeat returns the stored outcome;
	// BEFORE the material guards): the effective mode of the Work Item
	// (the stricter of the conversation ladder and the item's own
	// context) equal to human_requested, OR an active human handoff of
	// the case, freezes the creation of the new commitments (AC-2). The
	// control branch stays byte-identical: without the mode and without
	// the handoff the 2.9 path never differs.
	if err := fixtureWorkRule("create_commitment", casesDomain.GuardCommitmentFreeze(
		casesDomain.EffectiveWorkMode(
			casesDomain.ActiveConversationMode(f.modeTerms[command.CaseID]),
			casesDomain.WorkItemModeContext(f.modeTerms[command.CaseID], command.WorkItemID)),
		f.fixtureHandoffActive(command.CaseID))); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known || current.CaseID != command.CaseID {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("create_commitment", fmt.Errorf("%s", casesDomain.TerminalWorkItemCommitmentText))
	}
	// The observation gate of the offered known price (the pinned ok+fresh
	// reading with the byte-equal value — AC-3; the unknown price pins
	// zero honestly).
	pinned := int64(0)
	if command.Status == casesDomain.CommitmentStatusOffered && !casesDomain.MaterialUnknownTerm(command.MaterialUnknowns, casesDomain.CommitmentTermPrice) {
		value, valueErr := casesDomain.PriceValueJSON(command.Terms.Price)
		if valueErr != nil {
			return casesAPI.WorkOutcome{}, valueErr
		}
		observation := f.fixturePriceObservation(command.CaseID, command.WorkItemID)
		entry, _ := casesDomain.AuthorityEntryOf(casesDomain.MaterialFieldPrice)
		if observation == nil {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("create_commitment", fmt.Errorf("Предложение с ценой требует свежего наблюдения price системы-владельца: выполните read_owning_system и повторите создание (нечестно предполагать значение)."))
		}
		if casesDomain.DeriveObservationState(observation.Status, command.Now.Sub(observation.ObservedAt), entry.FreshnessSeconds) != casesDomain.ObservationStateFresh {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("create_commitment", fmt.Errorf("Наблюдение цены не свежее: перечитайте поле read_owning_system и повторите создание версии."))
		}
		// The byte equality of the CANONICAL values (the app mirror:
		// the stored observation value is the canonical form, the
		// price term renders its canonical form before the
		// comparison — never the struct-field order).
		observedValue, canonicalErr := casesDomain.CanonicalPayload(observation.Value)
		termValue, termErr := casesDomain.CanonicalPayload(value)
		if canonicalErr != nil || termErr != nil || string(observedValue) != string(termValue) {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("create_commitment", fmt.Errorf("Цена версии не совпадает с текущим наблюдением price: сверьте значение с системой-владельцем — требуется новое чтение либо другая цена."))
		}
		// The conflict badge against the confirmed exact fact (the
		// app mirror: a differs badge refuses the creation).
		confirmed := f.fixtureConfirmedFactValueOf(command.CaseID, casesDomain.MaterialFieldPrice)
		if casesDomain.ConflictBadgeOf(observation.Value, confirmed) == casesDomain.ConflictBadgeDiffers {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("create_commitment", fmt.Errorf("Наблюдение цены расходится с подтверждённым фактом (badge differs): разрешите расхождение подтверждением факта и повторите создание версии."))
		}
		pinned = observation.ObservationNo
	}
	// The relation pins (the owner epoch, the lineage word and the policy
	// outcome number of the fixture world).
	ownerEpoch := int64(1)
	ownerKind, ownerRef := casesDomain.OwnerKindHuman, f.owner
	if state := f.fixtureState(command.CaseID, command.Now); state.Current != nil {
		ownerKind, ownerRef, ownerEpoch = state.Current.OwnerKind, state.Current.OwnerRef, state.Current.Epoch
	}
	policyOutcomeNo := int64(0)
	for _, outcome := range f.policyOutcomes[command.CaseID] {
		if outcome.WorkItemID == command.WorkItemID && outcome.OutcomeNo > policyOutcomeNo {
			policyOutcomeNo = outcome.OutcomeNo
		}
	}
	// The monotonic ladder and the content token of the EXACT bytes.
	commitmentID := command.CommitmentID
	if !commitmentID.Valid() {
		commitmentID = f.fixtureCommitmentID()
	}
	versionNo := int64(0)
	for _, version := range f.commitments[command.CaseID] {
		if version.CommitmentID == commitmentID && version.VersionNo > versionNo {
			versionNo = version.VersionNo
		}
	}
	versionNo++
	token := casesDomain.CommitmentToken(fixtureCommitmentKey, commitmentID, versionNo, conditions)
	f.commitments[command.CaseID] = append(f.commitments[command.CaseID], casesDomain.CommitmentVersion{
		TenantID: f.tenant, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		CommitmentID: commitmentID, VersionNo: versionNo, Status: command.Status,
		Conditions: conditions, ActionSet: append([]string(nil), command.ActionSet...),
		ValidUntil: command.ValidUntil, ConsumptionLimit: command.ConsumptionLimit,
		MaterialUnknowns: append([]string(nil), command.MaterialUnknowns...),
		OwnerKind:        ownerKind, OwnerRef: ownerRef, OwnerEpoch: ownerEpoch,
		LineageWord:     fmt.Sprintf("employee/%s/1/1", f.employee),
		PolicyOutcomeNo: policyOutcomeNo, PinnedPriceObservationNo: pinned,
		ContentToken: token, ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{
		Command: "create_commitment", CommitmentID: commitmentID,
		CommitmentVersion: versionNo, CommitmentStatus: command.Status,
	}
	f.storeOutcome("create_commitment", command.IdempotencyKey, outcome)
	return outcome, nil
}

// ConfirmCommitment records the exact-version confirmation over the REAL
// guards: the presentation link (the sales reply carried the exact pair),
// the verbatim span excerpts of the accepted TEXT turn, the freshness of
// the pinned price observation, the assurance ladder of 2.3 and the Work
// Item transit to confirmed (the single machine step of the slice).
func (f *casesFixture) ConfirmCommitment(_ context.Context, command casesAPI.ConfirmCommitmentCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	if command.SubjectKind != "client" {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("confirm_commitment", fmt.Errorf("Вид субъекта подтверждения «%s» не входит в словарь (client)", command.SubjectKind))
	}
	outcome, fresh, err := f.workCommandFlow("confirm_commitment", command.IdempotencyKey, struct {
		Case     string   `json:"case"`
		WorkItem string   `json:"work_item"`
		Id       string   `json:"commitment_id"`
		Version  int64    `json:"version_no"`
		Expected int64    `json:"expected_version"`
		Kind     string   `json:"subject_kind"`
		Ref      string   `json:"subject_ref"`
		Turn     string   `json:"turn_ref"`
		Spans    []string `json:"spans"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.CommitmentID.String(), command.VersionNo, command.ExpectedVersion, command.SubjectKind, command.SubjectRef, command.TurnRef.String(), command.Spans})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known || current.CaseID != command.CaseID {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("confirm_commitment", fmt.Errorf("%s", casesDomain.TerminalWorkItemCommitmentText))
	}
	if current.State != casesDomain.WorkItemStateInWork && current.State != casesDomain.WorkItemStateWaiting {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("confirm_commitment", fmt.Errorf("Подтверждение допустимо только из состояний in_work и waiting (лестница состояний 2-2): классифицируйте элемент работы."))
	}
	if current.Version != command.ExpectedVersion {
		return casesAPI.WorkOutcome{}, casesDomain.ErrStaleVersion
	}
	// The EXACT version: it must exist, belong to this Work Item and be
	// offered; the readiness derivation guards the draft, the superseded
	// and the unknown-blocked versions.
	var version *casesDomain.CommitmentVersion
	maxVersionNo := int64(0)
	for i := range f.commitments[command.CaseID] {
		candidate := &f.commitments[command.CaseID][i]
		if candidate.CommitmentID != command.CommitmentID {
			continue
		}
		if candidate.VersionNo > maxVersionNo {
			maxVersionNo = candidate.VersionNo
		}
		if candidate.VersionNo == command.VersionNo && candidate.WorkItemID == command.WorkItemID {
			version = candidate
		}
	}
	if version == nil {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("confirm_commitment", fmt.Errorf("Версия обязательства не найдена: подтверждается только точная существующая offered-версия этого элемента работы."))
	}
	if err := fixtureWorkRule("confirm_commitment", casesDomain.GuardConfirmReadiness(casesDomain.DeriveCommitmentReadiness(*version, maxVersionNo), version.ValidUntil, command.Now)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The presentation link: a sales reply row carried the exact pair.
	presented := false
	for _, link := range f.presented[command.CaseID] {
		if link.CommitmentID == command.CommitmentID && link.VersionNo == command.VersionNo {
			presented = true
			break
		}
	}
	if !presented {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("confirm_commitment", fmt.Errorf("Версия не была предъявлена клиенту: подтверждается только показанная версия (связка сообщения-предъявления обязательна)."))
	}
	// The evidence: the accepted TEXT turn with the byte-exact spans.
	event, found := f.fixtureEvent(command.TurnRef)
	if !found || event.CaseID != command.CaseID || event.State != casesDomain.EventStateAccepted {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("confirm_commitment", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
	}
	if event.Modality != casesDomain.ModalityText {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("confirm_commitment", fmt.Errorf("Подтверждение голосовых сообщений — вне текущего срез: источник подтверждения должен быть текстовым событием."))
	}
	payload := f.payloads[command.TurnRef]
	if err := fixtureWorkRule("confirm_commitment", casesDomain.GuardConfirmEvidence(casesDomain.ConfirmCommitmentMaterial{
		SubjectKind: command.SubjectKind, SubjectRef: command.SubjectRef,
		TurnPayload: payload, Spans: command.Spans,
	})); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The freshness gate: the pinned observation must remain the CURRENT
	// ok+fresh reading with the byte-equal canonical value and without
	// the conflict with the confirmed fact (the app mirror).
	if !casesDomain.MaterialUnknownTerm(version.MaterialUnknowns, casesDomain.CommitmentTermPrice) {
		observation := f.fixturePriceObservation(command.CaseID, command.WorkItemID)
		entry, _ := casesDomain.AuthorityEntryOf(casesDomain.MaterialFieldPrice)
		freshMismatch := observation == nil || observation.ObservationNo != version.PinnedPriceObservationNo ||
			casesDomain.DeriveObservationState(observation.Status, command.Now.Sub(observation.ObservedAt), entry.FreshnessSeconds) != casesDomain.ObservationStateFresh
		if !freshMismatch {
			// The byte equality of the CANONICAL values and the
			// conflict badge (the nil observation short-circuits
			// above — the value reads are safe here).
			observedValue, observedErr := casesDomain.CanonicalPayload(observation.Value)
			termValue, termErr := casesDomain.CanonicalPayload(versionPriceValue(version.Conditions))
			confirmed := f.fixtureConfirmedFactValueOf(command.CaseID, casesDomain.MaterialFieldPrice)
			freshMismatch = observedErr != nil || termErr != nil || string(observedValue) != string(termValue) ||
				casesDomain.ConflictBadgeOf(observation.Value, confirmed) == casesDomain.ConflictBadgeDiffers
		}
		if freshMismatch {
			return casesAPI.WorkOutcome{}, fixtureWorkRule("confirm_commitment", fmt.Errorf("Наблюдение цены, на которое указывает версия, больше не является текущим: перечитайте поле (read_owning_system) и создайте новую версию обязательства."))
		}
	}
	// The assurance ladder of 2.3: the verified identity of the subject
	// and — when the price is positive — the acting authority record.
	records := f.verificationsOf(command.CaseID)
	assurance := casesDomain.DeriveAssuranceState(records, command.Now)
	if assurance.IdentityLevel != casesDomain.AssuranceVerified || assurance.IdentitySubject != command.SubjectRef {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("confirm_commitment", fmt.Errorf("Подтверждение требует проверенной личности субъекта (уровень verified): выполните step-up (knowledge_check, known_number_callback, known_address_reply) и повторите подтверждение."))
	}
	if versionPricePositive(version.Conditions) && (assurance.AuthorityLevel == casesDomain.AssuranceNone || assurance.AuthoritySubject != command.SubjectRef) {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("confirm_commitment", fmt.Errorf("Подтверждение с ценой требует действующей записи полномочий субъекта (mandate_note): задокументируйте основание полномочий и повторите подтверждение."))
	}
	// The duplicate subject of the same version: the fail-closed 409.
	for _, confirmation := range f.confirmations[command.CaseID] {
		if confirmation.CommitmentID == command.CommitmentID && confirmation.VersionNo == command.VersionNo &&
			confirmation.SubjectKind == command.SubjectKind && confirmation.SubjectRef == command.SubjectRef {
			return casesAPI.WorkOutcome{}, casesDomain.ErrCommitmentDuplicate
		}
	}
	confirmationID := f.fixtureCommitmentID()
	f.confirmations[command.CaseID] = append(f.confirmations[command.CaseID], casesDomain.ConfirmationRecord{
		TenantID: f.tenant, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		CommitmentID: command.CommitmentID, VersionNo: command.VersionNo, ConfirmationID: confirmationID,
		SubjectKind: command.SubjectKind, SubjectRef: command.SubjectRef,
		Method: casesDomain.CommitmentMethodExplicitText, Channel: "web_chat",
		ExpiresAt: version.ValidUntil, ConsumedCount: 0, ConsumptionLimit: version.ConsumptionLimit,
		TurnRef: command.TurnRef, ActorID: command.Actor.ID, ConfirmedAt: command.Now,
	})
	// The Work Item transit to confirmed (the single machine step — the
	// 2.2 ladder over the REAL guard).
	f.workVersions[command.WorkItemID] = append(f.workVersions[command.WorkItemID], casesDomain.WorkItemVersion{
		TenantID: f.tenant, CaseID: command.CaseID, ConversationID: current.ConversationID,
		WorkItemID: command.WorkItemID, VersionNo: current.Version + 1, Kind: casesDomain.VersionKindTransition,
		State: casesDomain.WorkItemStateConfirmed, Reason: "commitment_confirmed",
		ActorID: command.Actor.ID, Ref: confirmationID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{
		Command: "confirm_commitment", CommitmentID: command.CommitmentID,
		CommitmentVersion: command.VersionNo, ConfirmationID: confirmationID,
		SubjectKind: command.SubjectKind, Version: current.Version + 1, State: casesDomain.WorkItemStateConfirmed,
	}
	f.storeOutcome("confirm_commitment", command.IdempotencyKey, outcome)
	return outcome, nil
}

// RecordCommitmentDecline records the honest refusal or the change request
// over the REAL guards: the outcome word and the verbatim spans of the
// accepted TEXT turn. The decline mutates nothing (AC-11).
func (f *casesFixture) RecordCommitmentDecline(_ context.Context, command casesAPI.RecordCommitmentDeclineCommand) (casesAPI.WorkOutcome, error) {
	f.lock()
	defer f.unlock()
	event, found := f.fixtureEvent(command.TurnRef)
	if !found || event.CaseID != command.CaseID || event.State != casesDomain.EventStateAccepted {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_commitment_decline", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
	}
	if event.Modality != casesDomain.ModalityText {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_commitment_decline", fmt.Errorf("Разметка выдержек голосовых сообщений — вне текущего срез: источник отказа должен быть текстовым событием."))
	}
	if err := fixtureWorkRule("record_commitment_decline", casesDomain.GuardDeclineCommitment(command.Outcome, casesDomain.ConfirmCommitmentMaterial{
		SubjectKind: "client", SubjectRef: "client",
		TurnPayload: f.payloads[command.TurnRef], Spans: command.Spans,
	})); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome, fresh, err := f.workCommandFlow("record_commitment_decline", command.IdempotencyKey, struct {
		Case     string   `json:"case"`
		WorkItem string   `json:"work_item"`
		Id       string   `json:"commitment_id"`
		Version  int64    `json:"version_no"`
		Outcome  string   `json:"outcome"`
		Turn     string   `json:"turn_ref"`
		Spans    []string `json:"spans"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.CommitmentID.String(), command.VersionNo, command.Outcome, command.TurnRef.String(), command.Spans})
	if err != nil || !fresh {
		return outcome, err
	}
	if f.material[command.CaseID] == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	current, known := f.workCurrentOf(command.WorkItemID)
	if !known || current.CaseID != command.CaseID {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, fixtureWorkRule("record_commitment_decline", fmt.Errorf("%s", casesDomain.TerminalWorkItemCommitmentText))
	}
	declineID := f.fixtureCommitmentID()
	f.commitmentDeclines[command.CaseID] = append(f.commitmentDeclines[command.CaseID], casesPort.CommitmentDeclineView{
		TenantID: f.tenant, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		DeclineID: declineID, CommitmentID: command.CommitmentID, VersionNo: command.VersionNo,
		Outcome: command.Outcome, TurnRef: command.TurnRef,
		ActorID: command.Actor.ID, CreatedAt: command.Now,
	})
	outcome = casesAPI.WorkOutcome{
		Command: "record_commitment_decline", CommitmentID: command.CommitmentID,
		CommitmentVersion: command.VersionNo, DeclineID: declineID, DeclineOutcome: command.Outcome,
	}
	f.storeOutcome("record_commitment_decline", command.IdempotencyKey, outcome)
	return outcome, nil
}

// fixtureCommitmentView assembles the commitment section of the case view
// over the REAL derivations: the readiness per version, the validity per
// confirmation (expired/superseded/owner_stale/release_stale — the states
// are never stored) and the presentation links.
func (f *casesFixture) fixtureCommitmentView(caseID id.UUID) casesAPI.CommitmentView {
	view := casesAPI.CommitmentView{
		StatusWords:    append([]string(nil), casesDomain.CommitmentStatuses...),
		ActionWords:    append([]string(nil), casesDomain.CommitmentActions...),
		DeclineWords:   append([]string(nil), casesDomain.CommitmentDeclineOutcomes...),
		ValidityWords:  append([]string(nil), casesDomain.ConfirmationValidityWords...),
		ReadinessWords: append([]string(nil), casesDomain.CommitmentReadinessWords...),
		PanelText:      "Обязательство — версия с материальными условиями: новая версия = новая строка, прежние не меняются. Подтверждается только показанная версия с дословными выдержками согласия; отказ и изменение фиксируются отдельно, ничего не ломая.",
	}
	ownerEpoch := int64(1)
	if state := f.fixtureState(caseID, journeyClock.Now()); state.Current != nil {
		ownerEpoch = state.Current.Epoch
	}
	lineageCurrent := fmt.Sprintf("employee/%s/1/1", f.employee)
	for _, anchor := range f.items[caseID] {
		section := casesAPI.CommitmentWorkItemView{WorkItemID: anchor.WorkItemID}
		maxVersionNo := map[id.UUID]int64{}
		for _, version := range f.commitments[caseID] {
			if version.WorkItemID == anchor.WorkItemID && version.VersionNo > maxVersionNo[version.CommitmentID] {
				maxVersionNo[version.CommitmentID] = version.VersionNo
			}
		}
		for _, version := range f.commitments[caseID] {
			if version.WorkItemID != anchor.WorkItemID {
				continue
			}
			card := casesAPI.CommitmentVersionCard{
				Version: version, Current: version.VersionNo == maxVersionNo[version.CommitmentID],
				Readiness: casesDomain.DeriveCommitmentReadiness(version, maxVersionNo[version.CommitmentID]),
			}
			for _, link := range f.presented[caseID] {
				if link.CommitmentID == version.CommitmentID && link.VersionNo == version.VersionNo {
					card.Presented = true
					break
				}
			}
			section.Versions = append(section.Versions, card)
		}
		for _, confirmation := range f.confirmations[caseID] {
			if confirmation.WorkItemID != anchor.WorkItemID {
				continue
			}
			lineagePinned, ownerPinned := lineageOfVersion(f.commitments[caseID], confirmation.CommitmentID, confirmation.VersionNo)
			section.Confirmations = append(section.Confirmations, casesAPI.ConfirmationCard{
				Record: confirmation,
				Validity: casesDomain.DeriveConfirmationValidity(casesDomain.ConfirmationValidityInput{
					Now: journeyClock.Now(), ExpiresAt: confirmation.ExpiresAt,
					MaxVersionNo: maxVersionNo[confirmation.CommitmentID], ConfirmedVersion: confirmation.VersionNo,
					OwnerEpochPinned: ownerPinned, OwnerEpochCurrent: ownerEpoch,
					LineagePinned: lineagePinned, LineageCurrent: lineageCurrent,
				}),
			})
		}
		for _, decline := range f.commitmentDeclines[caseID] {
			if decline.WorkItemID == anchor.WorkItemID {
				section.Declines = append(section.Declines, decline)
			}
		}
		for _, link := range f.presented[caseID] {
			for _, version := range f.commitments[caseID] {
				if version.CommitmentID == link.CommitmentID && version.VersionNo == link.VersionNo && version.WorkItemID == anchor.WorkItemID {
					section.Presented = append(section.Presented, link)
					break
				}
			}
		}
		view.Items = append(view.Items, section)
	}
	return view
}

// lineageOfVersion resolves the pinned lineage word and owner epoch of one
// exact version (the validity derivation inputs).
func lineageOfVersion(versions []casesDomain.CommitmentVersion, commitmentID id.UUID, versionNo int64) (string, int64) {
	for _, version := range versions {
		if version.CommitmentID == commitmentID && version.VersionNo == versionNo {
			return version.LineageWord, version.OwnerEpoch
		}
	}
	return "", 0
}

// versionPriceValue resolves the canonical price bytes of the version
// conditions (the freshness comparison basis).
func versionPriceValue(conditions []byte) []byte {
	var terms casesDomain.MaterialTerms
	if err := json.Unmarshal(conditions, &terms); err != nil {
		return nil
	}
	value, err := casesDomain.PriceValueJSON(terms.Price)
	if err != nil {
		return nil
	}
	return value
}

// versionPricePositive resolves the authority trigger of the version
// conditions (the price > 0 — the rule table v1).
func versionPricePositive(conditions []byte) bool {
	var terms casesDomain.MaterialTerms
	if err := json.Unmarshal(conditions, &terms); err != nil {
		return false
	}
	return terms.Price.PricePositive()
}
