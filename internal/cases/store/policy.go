package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 2.6: the policy reads, the two command executions and the policy
// gate extension of send_client_reply
// ---------------------------------------------------------------------------

// mapPolicyInsertError maps the raced insert conflicts of the policy
// surface onto the honest domain errors: the outcome number UNIQUE is the
// monotonic backstop (the loser under the locks is impossible in the
// normal path — the backstop keeps the material truth anyway), the
// command record UNIQUE is the material idempotency conflict.
func mapPolicyInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "policy_outcomes_tenant_id_work_item_id_outcome_no_key":
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

// readAIProposalsTx reads every AI proposal of one case ordered by the
// creation time (the claims audit of the panel and the gate words).
func readAIProposalsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.AIProposal, error) {
	rows, err := tx.Query(ctx, `
                select proposal_id, case_id, conversation_id, work_item_id, turn_ref, schema_version,
                       intent, candidate_answer, missing_facts, extracted_facts, proposer, owner_epoch, mode_version, actor_id, created_at
                from cases.ai_proposals
                where tenant_id=$1 and case_id=$2 order by created_at, proposal_id`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	proposals := []casesDomain.AIProposal{}
	for rows.Next() {
		var proposal casesDomain.AIProposal
		var missingJSON, extractedJSON []byte
		if err := rows.Scan(&proposal.ProposalID, &proposal.CaseID, &proposal.ConversationID, &proposal.WorkItemID,
			&proposal.TurnRef, &proposal.SchemaVersion, &proposal.Intent, &proposal.CandidateAnswer,
			&missingJSON, &extractedJSON, &proposal.Proposer, &proposal.OwnerEpoch, &proposal.ModeVersion,
			&proposal.ActorID, &proposal.CreatedAt); err != nil {
			return nil, err
		}
		if len(missingJSON) > 0 {
			if err := json.Unmarshal(missingJSON, &proposal.MissingFacts); err != nil {
				return nil, err
			}
		}
		if len(extractedJSON) > 0 {
			if err := json.Unmarshal(extractedJSON, &proposal.ExtractedFacts); err != nil {
				return nil, err
			}
		}
		proposal.TenantID = tenantID
		proposal.CreatedAt = proposal.CreatedAt.UTC()
		proposals = append(proposals, proposal)
	}
	return proposals, rows.Err()
}

// ReadAIProposals reads every AI proposal of one case (the own read
// transaction of the view and the command pre-checks).
func (s *Postgres) ReadAIProposals(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.AIProposal, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	proposals, err := readAIProposalsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return proposals, tx.Commit(ctx)
}

// readPolicyOutcomesTx reads every Policy Outcome of one case ordered by
// the Work Item and the monotonic number (the derivation input of the
// CURRENT outcome the sales reply gate reads).
func readPolicyOutcomesTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.PolicyOutcome, error) {
	rows, err := tx.Query(ctx, `
                select outcome_id, case_id, conversation_id, work_item_id, proposal_id, outcome_no, outcome_class,
                       reason, rule_version, intent, mandatory_facts, next_steps, reused_facts, suppressed_questions,
                       active_alternatives, input_snapshot, decision_digest, actor_id, created_at
                from cases.policy_outcomes
                where tenant_id=$1 and case_id=$2 order by work_item_id, outcome_no`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	outcomes := []casesDomain.PolicyOutcome{}
	for rows.Next() {
		var outcome casesDomain.PolicyOutcome
		var mandatoryJSON, nextJSON, reusedJSON, suppressedJSON, snapshotJSON []byte
		var alternativesJSON []byte
		if err := rows.Scan(&outcome.OutcomeID, &outcome.CaseID, &outcome.ConversationID, &outcome.WorkItemID,
			&outcome.ProposalID, &outcome.OutcomeNo, &outcome.OutcomeClass, &outcome.Reason, &outcome.RuleVersion,
			&outcome.Intent, &mandatoryJSON, &nextJSON, &reusedJSON, &suppressedJSON, &alternativesJSON, &snapshotJSON,
			&outcome.DecisionDigest, &outcome.ActorID, &outcome.CreatedAt); err != nil {
			return nil, err
		}
		if len(alternativesJSON) > 0 {
			if err := json.Unmarshal(alternativesJSON, &outcome.ActiveAlternatives); err != nil {
				return nil, err
			}
		}
		if len(mandatoryJSON) > 0 {
			if err := json.Unmarshal(mandatoryJSON, &outcome.MandatoryFacts); err != nil {
				return nil, err
			}
		}
		if len(nextJSON) > 0 {
			if err := json.Unmarshal(nextJSON, &outcome.NextSteps); err != nil {
				return nil, err
			}
		}
		if len(reusedJSON) > 0 {
			if err := json.Unmarshal(reusedJSON, &outcome.ReusedFacts); err != nil {
				return nil, err
			}
		}
		if len(suppressedJSON) > 0 {
			if err := json.Unmarshal(suppressedJSON, &outcome.SuppressedQuestions); err != nil {
				return nil, err
			}
		}
		if err := json.Unmarshal(snapshotJSON, &outcome.InputSnapshot); err != nil {
			return nil, err
		}
		outcome.TenantID = tenantID
		outcome.CreatedAt = outcome.CreatedAt.UTC()
		outcomes = append(outcomes, outcome)
	}
	return outcomes, rows.Err()
}

// ReadPolicyOutcomes reads every Policy Outcome of one case (the own read
// transaction of the view and the command pre-checks).
func (s *Postgres) ReadPolicyOutcomes(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.PolicyOutcome, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	outcomes, err := readPolicyOutcomesTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return outcomes, tx.Commit(ctx)
}

// readFactsOfCaseTx reads the whole fact material of one case inside an
// existing transaction (the derivation input of the confirmed fact set of
// the policy evaluation — the anchor digest basis).
func readFactsOfCaseTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesPort.FactMaterial, error) {
	rows, err := tx.Query(ctx, `
                select fact_id, material_key, created_at from cases.facts
                where tenant_id=$1 and case_id=$2 order by material_key`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	anchors := []casesPort.FactMaterial{}
	for rows.Next() {
		var material casesPort.FactMaterial
		if err := rows.Scan(&material.Anchor.FactID, &material.Anchor.MaterialKey, &material.Anchor.CreatedAt); err != nil {
			return nil, err
		}
		material.Anchor.TenantID, material.Anchor.CaseID = tenantID, caseID
		anchors = append(anchors, material)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	materials := make([]casesPort.FactMaterial, 0, len(anchors))
	for _, anchor := range anchors {
		material, err := readFactMaterialTx(ctx, tx, tenantID, caseID, anchor.Anchor.FactID)
		if err != nil {
			return nil, err
		}
		materials = append(materials, material)
	}
	return materials, nil
}

// policyConfirmedFactsTx derives the confirmed exact fact set of the case
// inside the transaction (the 2-2 derivation — the reuse and the digest
// basis).
func policyConfirmedFactsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.ConfirmedFact, casesDomain.FactWorld, string, error) {
	materials, err := readFactsOfCaseTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return nil, nil, "", err
	}
	confirmed := []casesDomain.ConfirmedFact{}
	world := casesDomain.FactWorld{}
	for _, material := range materials {
		status, versionNo, digest := casesDomain.ConfirmedFactView(material.Versions, material.Confirmations)
		word := casesDomain.FactStatusWord{Status: status}
		if status == casesDomain.FactStatusConfirmed {
			fact := casesDomain.ConfirmedFact{
				FactID: material.Anchor.FactID, MaterialKey: material.Anchor.MaterialKey,
				VersionNo: versionNo, ValueDigest: digest,
			}
			confirmed = append(confirmed, fact)
			factCopy := fact
			word.Confirmed = &factCopy
		}
		world[material.Anchor.MaterialKey] = word
	}
	digest, _, err := casesDomain.ComputeFactsDigest(confirmed)
	if err != nil {
		return nil, nil, "", err
	}
	return confirmed, world, digest, nil
}

// policyProposalOfTx resolves one proposal of the case inside the
// transaction (the evaluation input — nil when the case has no such
// proposal).
func policyProposalOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, proposalID id.UUID) (*casesDomain.AIProposal, error) {
	proposals, err := readAIProposalsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	for i := range proposals {
		if proposals[i].ProposalID == proposalID {
			return &proposals[i], nil
		}
	}
	return nil, nil
}

// execSubmitAIProposal writes the typed proposal under the case +
// conversation locks: the conversation must be the case's own, the Work
// Item must be anchored in the case, the turn reference must be an
// accepted event of the case; the submission snapshot (the owner epoch
// and the mode version) is the lock-visible truth of the moment (the
// store re-reads — never a command claim). The outbox event carries the
// identifiers and the state words only (the candidate text never enters
// the envelope).
func (s *Postgres) execSubmitAIProposal(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := modeConversationOfTx(ctx, tx, tenantID, p.CaseID, p.ConversationID); err != nil {
		return err
	}
	if err := modeWorkItemOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	if err := modeTurnRefOfTx(ctx, tx, tenantID, p.CaseID, p.TurnRef); err != nil {
		return err
	}
	// The submission snapshot: the CURRENT owner epoch and mode version
	// under the locks (the honest re-read).
	terms, _, _, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	ownership := casesDomain.CurrentOwnership(terms)
	if ownership == nil {
		return casesDomain.ErrNotFound
	}
	modeTerms, err := readModeMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	modeVersion := casesDomain.ModeTermMaxVersion(modeTerms)
	if modeVersion < 1 {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "Лестница режимов пуста (до-миграционные данные): предложение требует действующей лестницы."}
	}
	missingJSON, err := json.Marshal(p.MissingFacts)
	if err != nil {
		return err
	}
	extractedJSON, err := json.Marshal(p.ExtractedFacts)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.ai_proposals (tenant_id, case_id, conversation_id, work_item_id, proposal_id, turn_ref,
                        schema_version, intent, candidate_answer, missing_facts, extracted_facts, proposer, owner_epoch, mode_version, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		tenantID, p.CaseID, p.ConversationID, p.WorkItemID, p.ProposalID, p.TurnRef,
		casesDomain.ProposalSchemaVersion, p.Intent, p.CandidateAnswer, missingJSON, extractedJSON,
		p.Proposer, ownership.Epoch, modeVersion, p.Actor, now); err != nil {
		return mapPolicyInsertError(err)
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	payload, err := casesDomain.AIProposalSubmittedPayloadJSON(casesDomain.AIProposalSubmittedPayload{
		TenantID: tenantID, CaseID: p.CaseID, ConversationID: p.ConversationID, WorkItemID: p.WorkItemID,
		ProposalID: p.ProposalID, TurnRef: p.TurnRef, SchemaVersion: casesDomain.ProposalSchemaVersion,
		Intent: p.Intent, Proposer: p.Proposer, OwnerEpoch: ownership.Epoch, ModeVersion: modeVersion,
		ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventAIProposalSubmitted, casesDomain.OutboxEventAIProposalSubmitV1,
		p.CaseID, sourceConnection, p.ProposalID, now, payload,
		casesDomain.DedupKeyAIProposalSubmitted(p.ProposalID), now); err != nil {
		return mapPolicyInsertError(err)
	}
	return nil
}

// execEvaluatePolicy writes the deterministic outcome under the case +
// conversation locks: the proposal must exist in the case, the Work Item
// must be alive (the terminal states refuse honestly), the four anchors
// must match the lock-visible versions (a moved anchor is the honest 409
// with the current words — the TOCTOU guard of the fact reads), the
// evaluation re-runs from the re-read material (the pure function — the
// store never trusts the app bytes), the outcome number is the
// per-Work-Item monotonic sequence under the lock, and the decision
// digest covers the anchored inputs + the decision (the same anchors
// produce the same digest).
func (s *Postgres) execEvaluatePolicy(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	proposal, err := policyProposalOfTx(ctx, tx, tenantID, p.CaseID, p.ProposalID)
	if err != nil {
		return err
	}
	if proposal == nil {
		return casesDomain.ErrNotFound
	}
	// The Work Item of the proposal must be anchored in the case and
	// alive: the terminal states refuse honestly (the merged item lives
	// on in its target).
	currents, err := readWorkItemCurrentsTx(ctx, tx, tenantID, p.CaseID, []id.UUID{proposal.WorkItemID})
	if err != nil {
		return err
	}
	current, ok := currents[proposal.WorkItemID]
	if !ok {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "Элемент работы не найден в обращении: предложение принадлежит элементу этого обращения."}
	}
	if current.Terminal {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.TerminalWorkItemText}
	}
	// The anchored input versions under the locks: the ownership epoch,
	// the mode ladder version, the confirmed fact digest and the Work
	// Item version. A moved anchor is the honest 409 with the current
	// words — the operator re-reads the state and re-anchors.
	terms, _, _, err := readOwnershipMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	ownership := casesDomain.CurrentOwnership(terms)
	if ownership == nil {
		return casesDomain.ErrNotFound
	}
	modeTerms, err := readModeMaterialTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	modeVersion := casesDomain.ModeTermMaxVersion(modeTerms)
	if modeVersion != p.ExpectedModeVersion || ownership.Epoch != p.ExpectedOwnerEpoch || current.Version != p.ExpectedWorkItemVersion {
		_, _, factsDigest, err := policyConfirmedFactsTx(ctx, tx, tenantID, p.CaseID)
		if err != nil {
			return err
		}
		return casesDomain.PolicyConflictError{
			CurrentModeVersion: modeVersion, CurrentOwnerEpoch: ownership.Epoch,
			CurrentFactsDigest: factsDigest, CurrentWorkItemVersion: current.Version,
		}
	}
	confirmed, world, factsDigest, err := policyConfirmedFactsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	if factsDigest != p.ExpectedFactsDigest {
		return casesDomain.PolicyConflictError{
			CurrentModeVersion: modeVersion, CurrentOwnerEpoch: ownership.Epoch,
			CurrentFactsDigest: factsDigest, CurrentWorkItemVersion: current.Version,
		}
	}
	// The effective mode of the Work Item: the stricter of the
	// conversation mode and the item context (the 2-5 derivation).
	active := casesDomain.ActiveConversationMode(modeTerms)
	context := casesDomain.WorkItemModeContext(modeTerms, proposal.WorkItemID)
	effective := casesDomain.EffectiveWorkModeWord(casesDomain.EffectiveWorkMode(active, context))
	// The deterministic evaluation over the re-read material (the pure
	// function — the store never trusts the app bytes).
	// The 2.11 fourth input is the preferences state: the step-01 wiring
	// keeps the control branch (nil — the decision stays byte-identical);
	// the real read under the same advisory locks arrives with step-03.
	decision := casesDomain.EvaluatePolicy(*proposal, world, effective, nil)
	_, digestMap, err := casesDomain.ComputeFactsDigest(confirmed)
	if err != nil {
		return err
	}
	snapshot := casesDomain.PolicyInputSnapshot{
		ProposalID: proposal.ProposalID, SchemaVersion: proposal.SchemaVersion,
		ModeVersion: modeVersion, OwnerEpoch: ownership.Epoch,
		FactsDigest: digestMap, FactsDigestHash: factsDigest,
		WorkItemVersion: current.Version, RuleVersion: casesDomain.PolicyRuleVersion,
		Preferences: casesDomain.AccommodationSnapshotOf(nil),
	}
	decisionDigest, err := casesDomain.DecisionDigest(snapshot, decision, proposal.Intent)
	if err != nil {
		return err
	}
	// The per-Work-Item monotonic outcome number under the lock.
	outcomes, err := readPolicyOutcomesTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	outcomeNo := int64(0)
	for _, outcome := range outcomes {
		if outcome.WorkItemID == proposal.WorkItemID && outcome.OutcomeNo > outcomeNo {
			outcomeNo = outcome.OutcomeNo
		}
	}
	outcomeNo++
	mandatoryJSON, err := json.Marshal(decision.MandatoryFacts)
	if err != nil {
		return err
	}
	nextJSON, err := json.Marshal(decision.NextSteps)
	if err != nil {
		return err
	}
	reusedJSON, err := json.Marshal(decision.ReusedFacts)
	if err != nil {
		return err
	}
	suppressedJSON, err := json.Marshal(decision.SuppressedQuestions)
	if err != nil {
		return err
	}
	// The nil slice marshals as the jsonb null — the column carries the
	// honest empty array of the control branch (the pre-migration rows read
	// as the empty array too).
	alternatives := decision.ActiveAlternatives
	if alternatives == nil {
		alternatives = []string{}
	}
	alternativesJSON, err := json.Marshal(alternatives)
	if err != nil {
		return err
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.policy_outcomes (tenant_id, case_id, conversation_id, work_item_id, proposal_id, outcome_id,
                        outcome_no, outcome_class, reason, rule_version, intent, mandatory_facts, next_steps, reused_facts,
                        suppressed_questions, active_alternatives, input_snapshot, decision_digest, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		tenantID, p.CaseID, proposal.ConversationID, proposal.WorkItemID, proposal.ProposalID, p.OutcomeID,
		outcomeNo, decision.OutcomeClass, decision.Reason, casesDomain.PolicyRuleVersion, proposal.Intent,
		mandatoryJSON, nextJSON, reusedJSON, suppressedJSON, alternativesJSON, snapshotJSON, decisionDigest, p.Actor, now); err != nil {
		return mapPolicyInsertError(err)
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	payload, err := casesDomain.PolicyEvaluatedPayloadJSON(casesDomain.PolicyEvaluatedPayload{
		TenantID: tenantID, CaseID: p.CaseID, ConversationID: proposal.ConversationID,
		WorkItemID: proposal.WorkItemID, ProposalID: proposal.ProposalID, OutcomeID: p.OutcomeID,
		OutcomeNo: outcomeNo, OutcomeClass: decision.OutcomeClass, Reason: decision.Reason,
		RuleVersion: casesDomain.PolicyRuleVersion, Intent: proposal.Intent,
		ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventPolicyEvaluated, casesDomain.OutboxEventPolicyEvaluatedV1,
		p.CaseID, sourceConnection, p.OutcomeID, now, payload,
		casesDomain.DedupKeyPolicyEvaluated(p.OutcomeID), now); err != nil {
		return mapPolicyInsertError(err)
	}
	return nil
}

// replyPolicyGateOfTx re-validates the policy gate of the sales reply
// under the locks: the Work Item anchor is required (the sales path
// belongs to a Work Item), the CURRENT outcome of that item (the maximal
// number) must be commercial_answer; the refusal names the current
// outcome words and the permitted path. The ownership, mode and
// disclosure gates fire EARLIER — their refusals never change. The
// return is the Policy Outcome id the reply record references (the zero
// id when the gate does not apply — the non-sales classes).
func replyPolicyGateOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID, p casesPort.WorkCommandParams) (id.UUID, error) {
	if p.ReplyClass != casesDomain.ReplyClassSales {
		return id.UUID(""), nil
	}
	if !p.WorkItemID.Valid() {
		return id.UUID(""), casesDomain.ReplyPolicyError{PermittedPath: casesDomain.PermittedPathAnchorWorkItem, Reason: casesDomain.ReplyPolicyNoAnchorText}
	}
	outcomes, err := readPolicyOutcomesTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return id.UUID(""), err
	}
	outcomesOfItem := []casesDomain.PolicyOutcome{}
	for _, outcome := range outcomes {
		if outcome.WorkItemID == p.WorkItemID {
			outcomesOfItem = append(outcomesOfItem, outcome)
		}
	}
	current := casesDomain.CurrentPolicyOutcome(outcomesOfItem)
	if current == nil {
		// No outcome exists: the permitted path is the evaluation — with
		// a proposal already at hand evaluate_policy, otherwise the
		// submission first.
		permitted := casesDomain.PermittedPathSubmitProposal
		proposals, err := readAIProposalsTx(ctx, tx, tenantID, caseID)
		if err != nil {
			return id.UUID(""), err
		}
		for _, proposal := range proposals {
			if proposal.WorkItemID == p.WorkItemID {
				permitted = casesDomain.PermittedPathEvaluatePolicy
				break
			}
		}
		return id.UUID(""), casesDomain.ReplyPolicyError{WorkItemID: p.WorkItemID, PermittedPath: permitted, Reason: casesDomain.ReplyPolicyOutcomeRequiredText}
	}
	if current.OutcomeClass != casesDomain.OutcomeCommercialAnswer {
		return id.UUID(""), casesDomain.ReplyPolicyError{
			WorkItemID: p.WorkItemID, CurrentClass: current.OutcomeClass, CurrentReason: current.Reason,
			RuleVersion: current.RuleVersion, OutcomeNo: current.OutcomeNo,
			PermittedPath: casesDomain.PermittedPathEvaluatePolicy, Reason: casesDomain.ReplyPolicyNotCommercialText,
		}
	}
	return current.OutcomeID, nil
}
