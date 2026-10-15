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
// The Story 2.9 commands: the versioned commitment, the exact-version
// confirmation and the honest decline
// ---------------------------------------------------------------------------
//
// The three commands are idempotent by the tenant-scoped key with the stored
// outcome (the 2.2..2.8 precedent: the replay answer comes before the state
// guards); the request hash covers the operator material only — the resolved
// pins (the owner epoch, the lineage word, the observation number) are the
// server-side truth, never the command material. The gate order is frozen:
// the rights gate (review OR configuration — the M1 precedent) fires BEFORE
// every material guard; the dictionary guards refuse with the honest 400;
// the offered version with the KNOWN price passes the observation gate at
// creation (the current ok+fresh price observation of the Work Item with the
// byte-equal canonical value and without the conflict with the confirmed
// fact — AC-3); the confirm command demands the EXACT shown version (the
// presentation link of the sales reply), the verbatim span excerpts of the
// client consent (the byte-exact containment — AC-6), the freshness of the
// pinned observation (the pin must remain the CURRENT read — a re-read moved
// past it, so the version points at the old reading and a NEW version is
// the honest path), and the assurance ladder of 2.3 (the verified identity
// of the subject and — when the price is positive — the acting authority
// record); the decline mutates nothing (AC-11). The material conditions and
// the span excerpts never enter the outbox envelopes (the «кандидат не
// покидает строку» precedent).

// commitmentScopeText is the honest scope refusal of the commitment
// commands (the shared «review или configuration» vocabulary of the
// 2.5..2.8 rights gates): the app word travels inside the forbidden error
// only — the transport renders its own sentence for the wire and the form.
const commitmentScopeText = "Команды обязательств требуют полномочия review или configuration: участники только с audit не создают версии, не подтверждают и не фиксируют отказы."

// commitmentAssuranceGateText is the honest refusal of the identity half of
// the confirm authority gate: the step-up ladder words of 2.3 (the methods
// of the text modality — the confirm contour is text-only in this slice).
const commitmentAssuranceGateText = "Подтверждение требует проверенной личности субъекта (уровень verified): выполните step-up (knowledge_check, known_number_callback, known_address_reply) и повторите подтверждение."

// commitmentAuthorityGateText is the honest refusal of the authority half of
// the confirm gate: the acting mandate record of the subject (mandate_note —
// the authority method dictionary v1).
const commitmentAuthorityGateText = "Подтверждение с ценой требует действующей записи полномочий субъекта (mandate_note): задокументируйте основание полномочий и повторите подтверждение."

// commitmentFreshnessGateText is the honest refusal of the freshness gate:
// the pinned observation is no longer current, stale, failed or conflicting
// — a new version with a new reading is the honest path (never a silent
// merge of the old material).
const commitmentFreshnessGateText = "Наблюдение цены, на которое указывает версия, больше не является текущим: перечитайте поле (read_owning_system) и создайте новую версию обязательства."

// commitmentSubjectKind is the subject dictionary v1 of the confirmation
// (the client participant of the case).
const commitmentSubjectKindClient = "client"

// priceObservation resolves the CURRENT price observation of one Work Item
// with the conflict badge against the confirmed exact fact of the same
// material key (the 2.7 derivation inputs — the freshness gate basis).
func (s *Service) priceObservation(ctx context.Context, scope tenancy.Context, caseID, workItemID id.UUID) (*casesDomain.Observation, string, error) {
	observations, err := s.repository.ReadObservations(ctx, scope, caseID)
	if err != nil {
		return nil, "", err
	}
	observationsOfItem := []casesDomain.Observation{}
	for _, observation := range observations {
		if observation.WorkItemID == workItemID && observation.MaterialKey == casesDomain.MaterialFieldPrice {
			observationsOfItem = append(observationsOfItem, observation)
		}
	}
	current := casesDomain.CurrentObservation(observationsOfItem)
	if current == nil {
		return nil, "", nil
	}
	facts, err := s.repository.ReadFactsOfCase(ctx, scope, caseID)
	if err != nil {
		return nil, "", err
	}
	confirmedValues := map[string]*casesDomain.FactVersion{}
	for i := range facts {
		if confirmed := confirmedFactValueOf(facts[i]); confirmed != nil {
			confirmedValues[facts[i].Anchor.MaterialKey] = confirmed
		}
	}
	return current, casesDomain.ConflictBadgeOf(current.Value, confirmedValues[casesDomain.MaterialFieldPrice]), nil
}

// guardOfferedPrice is the creation observation gate: an offered version
// with the KNOWN price term demands the CURRENT price observation of the
// Work Item to be ok+fresh with the byte-equal canonical value and without
// the conflict with the confirmed fact (AC-3) — the passing gate returns the
// pinned observation number; an unknown price (the honest hole of the
// material_unknowns) pins zero and passes without a reading (the readiness
// derivation — not the creation — carries the honesty).
func (s *Service) guardOfferedPrice(ctx context.Context, scope tenancy.Context, command casesAPI.CreateCommitmentCommand) (int64, error) {
	if command.Status != casesDomain.CommitmentStatusOffered {
		return 0, nil
	}
	if casesDomain.MaterialUnknownTerm(command.MaterialUnknowns, casesDomain.CommitmentTermPrice) {
		return 0, nil
	}
	value, err := casesDomain.PriceValueJSON(command.Terms.Price)
	if err != nil {
		return 0, err
	}
	// The byte equality of the CANONICAL values: the observation value is
	// the canonical JSON of the projection (the sorted-key form of
	// CanonicalPayload); the price term renders its canonical form before
	// the comparison (the spec word — the byte equality of canonical
	// values, never the struct-field order).
	canonicalValue, err := casesDomain.CanonicalPayload(value)
	if err != nil {
		return 0, err
	}
	current, badge, err := s.priceObservation(ctx, scope, command.CaseID, command.WorkItemID)
	if err != nil {
		return 0, err
	}
	if current == nil {
		return 0, rule("create_commitment", fmt.Errorf("Предложение с ценой требует свежего наблюдения price системы-владельца: выполните read_owning_system и повторите создание (нечестно предполагать значение)."))
	}
	entry, _ := casesDomain.AuthorityEntryOf(casesDomain.MaterialFieldPrice)
	state := casesDomain.DeriveObservationState(current.Status, command.Now.Sub(current.ObservedAt), entry.FreshnessSeconds)
	if state != casesDomain.ObservationStateFresh {
		return 0, rule("create_commitment", fmt.Errorf("Наблюдение цены не свежее (статус: %s): перечитайте поле read_owning_system и повторите создание версии.", state))
	}
	// The observation value renders through the PG jsonb normalization (the
	// space-separated form); the comparison canonicalizes BOTH sides — the
	// byte equality of the canonical values, never the storage rendering.
	observedValue, err := casesDomain.CanonicalPayload(current.Value)
	if err != nil {
		return 0, err
	}
	if string(observedValue) != string(canonicalValue) {
		return 0, rule("create_commitment", fmt.Errorf("Цена версии не совпадает с текущим наблюдением price: сверьте значение с системой-владельцем — требуется новое чтение либо другая цена."))
	}
	if badge == casesDomain.ConflictBadgeDiffers {
		return 0, rule("create_commitment", fmt.Errorf("Наблюдение цены расходится с подтверждённым фактом (badge differs): разрешите расхождение подтверждением факта и повторите создание версии."))
	}
	return current.ObservationNo, nil
}

// CreateCommitment creates one version of a commitment: the operator review
// material (the typed terms, the action set, the validity window, the
// consumption limit, the honest unknown holes), the observation gate of the
// offered price (the pinned ok+fresh reading — AC-3), the relation pins (the
// owner epoch, the release lineage word, the current policy outcome number)
// and the content binding token of the EXACT material bytes. The version
// number is the per-commitment monotonic counter: the app resolves the
// expected number, the store re-validates it under the advisory locks (the
// raced counter refuses honestly — the 2.2 CAS discipline).
func (s *Service) CreateCommitment(ctx context.Context, command casesAPI.CreateCommitmentCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST: review or configuration (the M1 precedent);
	// an audit-only member reads the case surface but never creates the
	// commitment versions.
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, commitmentScopeText)
	}
	// The dictionary guard: the status word, the material terms, the action
	// set, the consumption limit, the unknown words and the future validity
	// (the honest 400 with the dictionary words in the text).
	if err := rule("create_commitment", casesDomain.GuardCreateCommitment(casesDomain.CreateCommitmentCommand{
		Status: command.Status, Terms: command.Terms, ActionSet: command.ActionSet,
		ValidUntil: command.ValidUntil, ConsumptionLimit: command.ConsumptionLimit,
		MaterialUnknowns: command.MaterialUnknowns,
	}, command.Now)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The canonical bytes of the version material (the deterministic
	// serialization — the content binding basis).
	conditionsJSON, err := casesDomain.MaterialTermsJSON(command.Terms)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	actionSetJSON, err := json.Marshal(command.ActionSet)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	unknowns := command.MaterialUnknowns
	if unknowns == nil {
		unknowns = []string{}
	}
	unknownsJSON, err := json.Marshal(unknowns)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the operator material (the anchors, the
	// status, the canonical terms, the action set, the limits and the
	// unknown holes): the replay of the exact repeat returns the stored
	// outcome; the same key with another material is the honest key
	// conflict.
	hash := requestHash(struct {
		Case     string `json:"case"`
		WorkItem string `json:"work_item"`
		Id       string `json:"commitment_id"`
		Status   string `json:"status"`
		Terms    []byte `json:"terms"`
		Actions  []byte `json:"actions"`
		Unknowns []byte `json:"unknowns"`
		Valid    int64  `json:"valid_until_unix"`
		Limit    int    `json:"consumption_limit"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.CommitmentID.String(),
		command.Status, conditionsJSON, actionSetJSON, unknownsJSON,
		command.ValidUntil.Unix(), command.ConsumptionLimit})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "create_commitment", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The Story 2.13 freeze gate (AFTER the dictionary guards and the
	// idempotency replay — an exact repeat returns the stored outcome;
	// BEFORE the material guards): the effective mode of the Work Item
	// (the stricter of the conversation ladder and the item's own
	// context) equal to human_requested, OR an active human handoff of
	// the case, freezes the creation of the new commitments — the
	// incompatible automatic actions stop until the acknowledged
	// handoff (AC-2). The handoff half is the honest pre-migration
	// absence of this step: the store revalidation of step-02
	// (commitmentHandoffGateOfTx under the advisory locks) brings the
	// live derivation of the 00013 rows. The control branch stays
	// byte-identical: without the mode and the handoff the 2.9 path
	// never differs.
	modeTerms, err := s.repository.ReadModeMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("create_commitment", casesDomain.GuardCommitmentFreeze(
		casesDomain.EffectiveWorkMode(
			casesDomain.ActiveConversationMode(modeTerms),
			casesDomain.WorkItemModeContext(modeTerms, command.WorkItemID)),
		false)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The Work Item of the command scope: it must exist in this case and be
	// alive (the write-command boundary of 2.2).
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, command.CaseID, []id.UUID{command.WorkItemID})
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current, ok := currents[command.WorkItemID]
	if !ok {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, rule("create_commitment", fmt.Errorf("%s", casesDomain.TerminalWorkItemCommitmentText))
	}
	// The observation gate of the offered price (the pinned ok+fresh
	// reading with the byte-equal value — AC-3; the unknown price pins
	// zero honestly).
	pinnedObservationNo, err := s.guardOfferedPrice(ctx, scope, command)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The relation pins: the owner epoch of the moment (the 2.4 term), the
	// release lineage word of the acting decision (the 2.4 reply-snapshot
	// precedent) and the current policy outcome number of the Work Item
	// (the 2.6 commercial ladder; zero — the honest absence).
	ownershipTerms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	owner := casesDomain.CurrentOwnership(ownershipTerms)
	if owner == nil {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	conversation, err := s.caseConnection(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	employee, decisionNo, profileVersion, err := s.lineageWord(ctx, scope, conversation)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	lineageWord := fmt.Sprintf("employee/%s/%d/%d", employee, decisionNo, profileVersion)
	policyOutcomeNo := int64(0)
	outcomes, err := s.repository.ReadPolicyOutcomes(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcomesOfItem := []casesDomain.PolicyOutcome{}
	for _, outcome := range outcomes {
		if outcome.WorkItemID == command.WorkItemID {
			outcomesOfItem = append(outcomesOfItem, outcome)
		}
	}
	if currentOutcome := casesDomain.CurrentPolicyOutcome(outcomesOfItem); currentOutcome != nil {
		policyOutcomeNo = currentOutcome.OutcomeNo
	}
	// The expected version number of the monotonic ladder (the store
	// re-validates under the advisory locks — the raced counter is the
	// honest 2.2 stale-version refusal) and the content binding token of
	// the EXACT material bytes (the per-tenant key v1, the ensure pattern
	// of the identifier keys).
	versions, err := s.repository.ReadCommitments(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	versionNo := int64(0)
	for _, version := range versions {
		if version.CommitmentID == command.CommitmentID && version.VersionNo > versionNo {
			versionNo = version.VersionNo
		}
	}
	versionNo++
	key, err := s.repository.ReadCommitmentKey(ctx, scope, command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	token := casesDomain.CommitmentToken(key, command.CommitmentID, versionNo, conditionsJSON)
	// The generated ids of the version row and the outbox event.
	commitmentID := command.CommitmentID
	if !commitmentID.Valid() {
		if commitmentID, err = s.ids.New(command.Now); err != nil {
			return casesAPI.WorkOutcome{}, err
		}
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
		Command: "create_commitment", CommitmentID: commitmentID,
		CommitmentVersion: versionNo, CommitmentStatus: command.Status,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "create_commitment", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		CommitmentID: commitmentID, VersionNo: versionNo,
		CommitmentStatus: command.Status, CommitmentConditions: conditionsJSON,
		CommitmentActionSet: actionSetJSON, CommitmentMaterialUnknowns: unknownsJSON,
		CommitmentValidUntil: command.ValidUntil, CommitmentConsumptionLimit: command.ConsumptionLimit,
		CommitmentOwnerKind: owner.OwnerKind, CommitmentOwnerRef: owner.OwnerRef,
		CommitmentOwnerEpoch: owner.Epoch, CommitmentLineageWord: lineageWord,
		CommitmentPolicyOutcomeNo:     policyOutcomeNo,
		CommitmentPinnedObservationNo: pinnedObservationNo,
		CommitmentContentToken:        token,
		CorrelationID:                 correlation, OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// ConfirmCommitment records the exact-version confirmation of the SHOWN
// material: the presentation link (a sales reply row carried the exact
// pair — AC-6 «показанную версию»), the verbatim span excerpts of the
// client consent of the accepted text turn (the byte-exact containment —
// an invented consent is impossible), the freshness of the pinned price
// observation (the pin must remain the CURRENT reading — a re-read moved
// past it, so a new version is the honest path, fail-closed), the assurance
// ladder of 2.3 (the verified identity of the subject and — when the price
// is positive — the acting authority record of the subject) and the Work
// Item transit to confirmed (the single machine step of the slice — the
// same transaction, the 2.2 CAS discipline of the version expectation).
func (s *Service) ConfirmCommitment(ctx context.Context, command casesAPI.ConfirmCommitmentCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST (the M1 precedent).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, commitmentScopeText)
	}
	// The subject dictionary v1: the client participant.
	if command.SubjectKind != commitmentSubjectKindClient {
		return casesAPI.WorkOutcome{}, rule("confirm_commitment", fmt.Errorf("Вид субъекта подтверждения «%s» не входит в словарь (client)", command.SubjectKind))
	}
	// The request hash covers the operator material (the anchors, the
	// exact version, the Work Item expectation, the subject, the turn and
	// the spans).
	hash := requestHash(struct {
		Case     string   `json:"case"`
		WorkItem string   `json:"work_item"`
		Id       string   `json:"commitment_id"`
		Version  int64    `json:"version_no"`
		Expected int64    `json:"expected_version"`
		Kind     string   `json:"subject_kind"`
		Ref      string   `json:"subject_ref"`
		Turn     string   `json:"turn_ref"`
		Spans    []string `json:"spans"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.CommitmentID.String(),
		command.VersionNo, command.ExpectedVersion, command.SubjectKind, command.SubjectRef,
		command.TurnRef.String(), command.Spans})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "confirm_commitment", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The Work Item: alive and on the confirm ladder (in_work or waiting —
	// the 2.2 boundary; the store re-validates under the locks with the
	// version expectation of the transit).
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, command.CaseID, []id.UUID{command.WorkItemID})
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current, ok := currents[command.WorkItemID]
	if !ok {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, rule("confirm_commitment", fmt.Errorf("%s", casesDomain.TerminalWorkItemCommitmentText))
	}
	if current.State != casesDomain.WorkItemStateInWork && current.State != casesDomain.WorkItemStateWaiting {
		return casesAPI.WorkOutcome{}, rule("confirm_commitment", fmt.Errorf("Подтверждение допустимо только из состояний in_work и waiting (лестница состояний 2-2): классифицируйте элемент работы."))
	}
	// The EXACT version: it must exist in this case, belong to this Work
	// Item and be offered; the readiness derivation guards the draft, the
	// superseded and the unknown-blocked versions (the read-time honesty —
	// AC-1/AC-2/AC-8).
	versions, err := s.repository.ReadCommitments(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	var version *casesDomain.CommitmentVersion
	maxVersionNo := int64(0)
	for i := range versions {
		if versions[i].CommitmentID == command.CommitmentID {
			if versions[i].VersionNo > maxVersionNo {
				maxVersionNo = versions[i].VersionNo
			}
			if versions[i].VersionNo == command.VersionNo && versions[i].WorkItemID == command.WorkItemID {
				version = &versions[i]
			}
		}
	}
	if version == nil {
		return casesAPI.WorkOutcome{}, rule("confirm_commitment", fmt.Errorf("Версия обязательства не найдена: подтверждается только точная существующая offered-версия этого элемента работы."))
	}
	readiness := casesDomain.DeriveCommitmentReadiness(*version, maxVersionNo)
	if err := rule("confirm_commitment", casesDomain.GuardConfirmReadiness(readiness, version.ValidUntil, command.Now)); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The presentation link: a sales reply row of this case carried the
	// exact pair (the shown version — AC-6; the temporal ordering is the
	// producer's mark, the link existence is the machine truth).
	links, err := s.repository.ReadPresentedCommitments(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	presented := false
	for _, link := range links {
		if link.CommitmentID == command.CommitmentID && link.VersionNo == command.VersionNo {
			presented = true
			break
		}
	}
	if !presented {
		return casesAPI.WorkOutcome{}, rule("confirm_commitment", fmt.Errorf("Версия не была предъявлена клиенту: подтверждается только показанная версия (связка сообщения-предъявления обязательна)."))
	}
	// The evidence: the turn source must be an ACCEPTED TEXT event of this
	// case (the voice contour refuses honestly — the modality boundary of
	// the slice); the spans are the byte-exact substrings of the sealed
	// payload (an invented consent is impossible — AC-6).
	event, err := s.repository.ReadEventOfCase(ctx, scope, command.CaseID, command.TurnRef)
	if err != nil {
		return casesAPI.WorkOutcome{}, rule("confirm_commitment", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
	}
	if event.State != casesDomain.EventStateAccepted {
		return casesAPI.WorkOutcome{}, rule("confirm_commitment", fmt.Errorf("Источник подтверждения не принят: разметка выдержек возможна только на принятых событиях кейса."))
	}
	if event.Modality != casesDomain.ModalityText {
		return casesAPI.WorkOutcome{}, rule("confirm_commitment", fmt.Errorf("Подтверждение голосовых сообщений — вне текущего срез: источник подтверждения должен быть текстовым событием."))
	}
	sealed, err := s.repository.ReadPayload(ctx, scope, command.CaseID, command.TurnRef)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("confirm_commitment", casesDomain.GuardConfirmEvidence(casesDomain.ConfirmCommitmentMaterial{
		SubjectKind: command.SubjectKind, SubjectRef: command.SubjectRef,
		TurnPayload: sealed.Payload, Spans: command.Spans,
	})); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The terms of the confirmed version (the material snapshot basis and
	// the freshness comparison).
	var terms casesDomain.MaterialTerms
	if err := json.Unmarshal(version.Conditions, &terms); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The freshness gate: the pinned price observation must remain the
	// CURRENT reading of the field — ok, fresh, byte-equal and without the
	// conflict with the confirmed fact (a re-read moved past the pin — the
	// version points at the old reading — a new version is the honest
	// path, fail-closed; AC-3).
	if !casesDomain.MaterialUnknownTerm(version.MaterialUnknowns, casesDomain.CommitmentTermPrice) && terms.Price.PriceKnown() {
		value, err := casesDomain.PriceValueJSON(terms.Price)
		if err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		canonicalValue, err := casesDomain.CanonicalPayload(value)
		if err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		observation, badge, err := s.priceObservation(ctx, scope, command.CaseID, command.WorkItemID)
		if err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		entry, _ := casesDomain.AuthorityEntryOf(casesDomain.MaterialFieldPrice)
		observedValue, err := casesDomain.CanonicalPayload(observation.Value)
		if err != nil {
			return casesAPI.WorkOutcome{}, err
		}
		if observation == nil || observation.ObservationNo != version.PinnedPriceObservationNo ||
			casesDomain.DeriveObservationState(observation.Status, command.Now.Sub(observation.ObservedAt), entry.FreshnessSeconds) != casesDomain.ObservationStateFresh ||
			string(observedValue) != string(canonicalValue) || badge == casesDomain.ConflictBadgeDiffers {
			return casesAPI.WorkOutcome{}, rule("confirm_commitment", fmt.Errorf("%s", commitmentFreshnessGateText))
		}
	}
	// The assurance gate of 2.3: the verified identity of the subject and —
	// when the price is positive (the rule table v1 trigger) — the acting
	// authority record of the subject (mandate_note). The refusal carries
	// the step-up ladder words (the honest path, never a silent pass).
	records, err := s.repository.ReadVerificationRecords(ctx, scope, command.CaseID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	assurance := casesDomain.DeriveAssuranceState(records, command.Now)
	if assurance.IdentityLevel != casesDomain.AssuranceVerified || assurance.IdentitySubject != command.SubjectRef {
		return casesAPI.WorkOutcome{}, rule("confirm_commitment", fmt.Errorf("%s", commitmentAssuranceGateText))
	}
	if terms.Price.PricePositive() && (assurance.AuthorityLevel == casesDomain.AssuranceNone || assurance.AuthoritySubject != command.SubjectRef) {
		return casesAPI.WorkOutcome{}, rule("confirm_commitment", fmt.Errorf("%s", commitmentAuthorityGateText))
	}
	// The material snapshot, the action set snapshot and the content
	// binding token of the confirmation (the canonical bytes of the EXACT
	// shown version — the row carries the material; the envelope never
	// does).
	key, err := s.repository.ReadCommitmentKey(ctx, scope, command.Now)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	token := casesDomain.CommitmentToken(key, command.CommitmentID, command.VersionNo, version.Conditions)
	actionSetJSON, err := json.Marshal(version.ActionSet)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	confirmationID, err := s.ids.New(command.Now)
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
	spansJSON, err := json.Marshal(command.Spans)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "confirm_commitment", CommitmentID: command.CommitmentID,
		CommitmentVersion: command.VersionNo, ConfirmationID: confirmationID,
		SubjectKind: command.SubjectKind, Version: command.ExpectedVersion + 1,
		State: casesDomain.WorkItemStateConfirmed,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "confirm_commitment", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		CommitmentID: command.CommitmentID, VersionNo: command.VersionNo,
		ConfirmationID: confirmationID, ExpectedVersion: command.ExpectedVersion,
		CommitmentSubjectKind: command.SubjectKind, CommitmentSubjectRef: command.SubjectRef,
		Channel: event.Channel, TurnRef: command.TurnRef,
		CommitmentExpiresAt: version.ValidUntil, CommitmentConsumptionLimit: version.ConsumptionLimit,
		CommitmentEvidenceSpans: spansJSON, CommitmentTurnPayload: sealed.Payload,
		CommitmentMaterialSnapshot: version.Conditions, CommitmentActionSet: actionSetJSON,
		CommitmentContentToken: token,
		CorrelationID:          correlation, OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// RecordCommitmentDecline records the honest refusal or the change request
// of the shown material: the outcome word of the dictionary and the
// verbatim span excerpts of the client words. The decline mutates nothing
// (AC-11); the operator answers the change request with a new version
// (create_commitment — version_no+1).
func (s *Service) RecordCommitmentDecline(ctx context.Context, command casesAPI.RecordCommitmentDeclineCommand) (casesAPI.WorkOutcome, error) {
	// The rights gate FIRST (the M1 precedent).
	scope, rights, err := s.ownershipScope(ctx, command.Actor, command.TenantID)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if !rights.Review && !rights.Configuration {
		return casesAPI.WorkOutcome{}, fmt.Errorf("%w: %s", domain.ErrForbidden, commitmentScopeText)
	}
	// The turn source and the span honesty: the accepted TEXT event of
	// this case carries the verbatim client words (the same discipline as
	// the confirmation evidence).
	event, err := s.repository.ReadEventOfCase(ctx, scope, command.CaseID, command.TurnRef)
	if err != nil {
		return casesAPI.WorkOutcome{}, rule("record_commitment_decline", fmt.Errorf("%s", casesDomain.TurnRefNotFoundText))
	}
	if event.State != casesDomain.EventStateAccepted {
		return casesAPI.WorkOutcome{}, rule("record_commitment_decline", fmt.Errorf("Источник отказа не принят: разметка выдержек возможна только на принятых событиях кейса."))
	}
	if event.Modality != casesDomain.ModalityText {
		return casesAPI.WorkOutcome{}, rule("record_commitment_decline", fmt.Errorf("Разметка выдержек голосовых сообщений — вне текущего срез: источник отказа должен быть текстовым событием."))
	}
	sealed, err := s.repository.ReadPayload(ctx, scope, command.CaseID, command.TurnRef)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if err := rule("record_commitment_decline", casesDomain.GuardDeclineCommitment(command.Outcome, casesDomain.ConfirmCommitmentMaterial{
		SubjectKind: commitmentSubjectKindClient, SubjectRef: "client",
		TurnPayload: sealed.Payload, Spans: command.Spans,
	})); err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	// The request hash covers the operator material (the anchors, the
	// exact version, the outcome word, the turn and the spans).
	hash := requestHash(struct {
		Case     string   `json:"case"`
		WorkItem string   `json:"work_item"`
		Id       string   `json:"commitment_id"`
		Version  int64    `json:"version_no"`
		Outcome  string   `json:"outcome"`
		Turn     string   `json:"turn_ref"`
		Spans    []string `json:"spans"`
	}{command.CaseID.String(), command.WorkItemID.String(), command.CommitmentID.String(),
		command.VersionNo, command.Outcome, command.TurnRef.String(), command.Spans})
	if outcome, replay, err := s.lookupCommand(ctx, scope, "record_commitment_decline", command.IdempotencyKey, hash); replay || err != nil {
		return outcome, err
	}
	// The Work Item: alive (the write-command boundary of 2.2).
	currents, err := s.repository.ReadWorkItemCurrents(ctx, scope, command.CaseID, []id.UUID{command.WorkItemID})
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	current, ok := currents[command.WorkItemID]
	if !ok {
		return casesAPI.WorkOutcome{}, casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesAPI.WorkOutcome{}, rule("record_commitment_decline", fmt.Errorf("%s", casesDomain.TerminalWorkItemCommitmentText))
	}
	declineID, err := s.ids.New(command.Now)
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
	spansJSON, err := json.Marshal(command.Spans)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	outcome := casesAPI.WorkOutcome{
		Command: "record_commitment_decline", CommitmentID: command.CommitmentID,
		CommitmentVersion: command.VersionNo, DeclineID: declineID, DeclineOutcome: command.Outcome,
	}
	outcomeJSON, err := json.Marshal(outcome)
	if err != nil {
		return casesAPI.WorkOutcome{}, err
	}
	if stored, replay, err := s.executeWork(ctx, scope, casesPort.WorkCommandParams{
		Scope: scope, Command: "record_commitment_decline", Key: command.IdempotencyKey, RequestHash: hash,
		Actor: command.Actor.ID, CaseID: command.CaseID, WorkItemID: command.WorkItemID,
		CommitmentID: command.CommitmentID, VersionNo: command.VersionNo,
		DeclineID: declineID, CommitmentDeclineOutcome: command.Outcome,
		TurnRef: command.TurnRef, CommitmentEvidenceSpans: spansJSON,
		CommitmentTurnPayload: sealed.Payload,
		CorrelationID:         correlation, OutboxEventID: outboxEventID, Outcome: outcomeJSON,
		Now: command.Now.UTC().Truncate(time.Microsecond),
	}); err != nil {
		return casesAPI.WorkOutcome{}, err
	} else if replay {
		return stored, nil
	}
	return outcome, nil
}

// commitmentView assembles the commitment section of the case view: the
// per-Work-Item sections with the version history (the read-time readiness
// derivation, the presentation links), the confirmations (the read-time
// validity derivation — expired/superseded/owner_stale/release_stale against
// the CURRENT owner epoch and the CURRENT release lineage), the declines
// and the dictionary words of the forms.
func (s *Service) commitmentView(ctx context.Context, scope tenancy.Context, detail casesDomain.CaseDetail) (casesAPI.CommitmentView, error) {
	versions, err := s.repository.ReadCommitments(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.CommitmentView{}, err
	}
	confirmations, err := s.repository.ReadConfirmations(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.CommitmentView{}, err
	}
	declines, err := s.repository.ReadCommitmentDeclines(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.CommitmentView{}, err
	}
	links, err := s.repository.ReadPresentedCommitments(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.CommitmentView{}, err
	}
	// The CURRENT owner epoch and the CURRENT release lineage word of the
	// validity derivation (the read-time honesty — the replay of an old
	// event never resurrects a dead confirmation).
	ownershipTerms, _, _, err := s.repository.ReadOwnershipMaterial(ctx, scope, detail.Case.CaseID)
	if err != nil {
		return casesAPI.CommitmentView{}, err
	}
	ownerEpoch := int64(0)
	if owner := casesDomain.CurrentOwnership(ownershipTerms); owner != nil {
		ownerEpoch = owner.Epoch
	}
	conversation, conversationErr := s.caseConnection(ctx, scope, detail.Case.CaseID)
	lineageCurrent := ""
	if conversationErr == nil {
		if employee, decisionNo, profileVersion, lineageErr := s.lineageWord(ctx, scope, conversation); lineageErr == nil {
			lineageCurrent = fmt.Sprintf("employee/%s/%d/%d", employee, decisionNo, profileVersion)
		}
	}
	now := s.clock.Now()
	view := casesAPI.CommitmentView{
		StatusWords:    append([]string(nil), casesDomain.CommitmentStatuses...),
		ActionWords:    append([]string(nil), casesDomain.CommitmentActions...),
		DeclineWords:   append([]string(nil), casesDomain.CommitmentDeclineOutcomes...),
		ValidityWords:  append([]string(nil), casesDomain.ConfirmationValidityWords...),
		ReadinessWords: append([]string(nil), casesDomain.CommitmentReadinessWords...),
		PanelText:      "Обязательство — версия с материальными условиями: новая версия = новая строка, прежние не меняются. Подтверждается только показанная версия с дословными выдержками согласия; отказ и изменение фиксируются отдельно, ничего не ломая.",
		Items:          make([]casesAPI.CommitmentWorkItemView, 0, len(detail.WorkItems)),
	}
	for _, item := range detail.WorkItems {
		section := casesAPI.CommitmentWorkItemView{
			WorkItemID:    item.WorkItemID,
			Versions:      []casesAPI.CommitmentVersionCard{},
			Confirmations: []casesAPI.ConfirmationCard{},
			Declines:      []casesPort.CommitmentDeclineView{},
			Presented:     []casesDomain.PresentedCommitment{},
		}
		maxVersionNo := map[id.UUID]int64{}
		for _, version := range versions {
			if version.WorkItemID == item.WorkItemID && version.VersionNo > maxVersionNo[version.CommitmentID] {
				maxVersionNo[version.CommitmentID] = version.VersionNo
			}
		}
		for _, version := range versions {
			if version.WorkItemID != item.WorkItemID {
				continue
			}
			card := casesAPI.CommitmentVersionCard{
				Version: version, Readiness: casesDomain.DeriveCommitmentReadiness(version, maxVersionNo[version.CommitmentID]),
				Current: version.VersionNo == maxVersionNo[version.CommitmentID],
			}
			for _, link := range links {
				if link.CommitmentID == version.CommitmentID && link.VersionNo == version.VersionNo {
					card.Presented = true
					break
				}
			}
			section.Versions = append(section.Versions, card)
		}
		for _, confirmation := range confirmations {
			if confirmation.WorkItemID != item.WorkItemID {
				continue
			}
			lineagePinned := ""
			for _, version := range versions {
				if version.CommitmentID == confirmation.CommitmentID && version.VersionNo == confirmation.VersionNo {
					lineagePinned = version.LineageWord
					break
				}
			}
			section.Confirmations = append(section.Confirmations, casesAPI.ConfirmationCard{
				Record: confirmation,
				Validity: casesDomain.DeriveConfirmationValidity(casesDomain.ConfirmationValidityInput{
					Now: now, ExpiresAt: confirmation.ExpiresAt,
					MaxVersionNo: maxVersionNo[confirmation.CommitmentID], ConfirmedVersion: confirmation.VersionNo,
					OwnerEpochPinned: ownerEpochOf(versions, confirmation.CommitmentID, confirmation.VersionNo), OwnerEpochCurrent: ownerEpoch,
					LineagePinned: lineagePinned, LineageCurrent: lineageCurrent,
				}),
			})
		}
		for _, decline := range declines {
			if decline.WorkItemID == item.WorkItemID {
				section.Declines = append(section.Declines, decline)
			}
		}
		for _, link := range links {
			belongs := false
			for _, version := range versions {
				if version.CommitmentID == link.CommitmentID && version.VersionNo == link.VersionNo && version.WorkItemID == item.WorkItemID {
					belongs = true
					break
				}
			}
			if belongs {
				section.Presented = append(section.Presented, link)
			}
		}
		view.Items = append(view.Items, section)
	}
	return view, nil
}

// ownerEpochOf resolves the pinned owner epoch of one exact version (the
// validity derivation input — the pin lives in the version row).
func ownerEpochOf(versions []casesDomain.CommitmentVersion, commitmentID id.UUID, versionNo int64) int64 {
	for _, version := range versions {
		if version.CommitmentID == commitmentID && version.VersionNo == versionNo {
			return version.OwnerEpoch
		}
	}
	return 0
}
