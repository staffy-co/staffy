package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ---------------------------------------------------------------------------
// Story 2.9: the commitment version writes, the exact-version confirmation,
// the decline and the per-tenant commitment keys
// ---------------------------------------------------------------------------

// mapCommitmentInsertError maps the raced insert conflicts of the
// commitment surface onto the honest domain errors: the confirmation
// subject UNIQUE is the fail-closed duplicate backstop (one subject per
// version — the second command key of the same subject refuses), the
// commitment version PK is the monotonic backstop (the loser under the
// locks is impossible in the normal path — the backstop keeps the
// material truth anyway), the command record UNIQUE is the material
// idempotency conflict.
func mapCommitmentInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "commitments_pkey":
			return casesDomain.ErrStaleVersion
		case "confirmations_tenant_id_commitment_id_version_no_subject_kind_subject_ref_key":
			return casesDomain.ErrCommitmentDuplicate
		}
	}
	return err
}

// readCommitmentsTx reads every commitment version of one case ordered by
// the commitment and the monotonic number (the derivation input of the
// CURRENT version, the readiness and the panel history).
func readCommitmentsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.CommitmentVersion, error) {
	rows, err := tx.Query(ctx, `
                select commitment_id, case_id, work_item_id, version_no, status, conditions, action_set,
                       valid_until, consumption_limit, material_unknowns, owner_kind, owner_ref, owner_epoch,
                       lineage_word, policy_outcome_no, pinned_price_observation_no, content_token, actor_id, created_at
                from cases.commitments
                where tenant_id=$1 and case_id=$2 order by commitment_id, version_no`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := []casesDomain.CommitmentVersion{}
	for rows.Next() {
		var version casesDomain.CommitmentVersion
		var conditions, actionSet, unknowns []byte
		if err := rows.Scan(&version.CommitmentID, &version.CaseID, &version.WorkItemID, &version.VersionNo,
			&version.Status, &conditions, &actionSet, &version.ValidUntil, &version.ConsumptionLimit,
			&unknowns, &version.OwnerKind, &version.OwnerRef, &version.OwnerEpoch, &version.LineageWord,
			&version.PolicyOutcomeNo, &version.PinnedPriceObservationNo, &version.ContentToken,
			&version.ActorID, &version.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(actionSet, &version.ActionSet); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(unknowns, &version.MaterialUnknowns); err != nil {
			return nil, err
		}
		version.TenantID = tenantID
		version.Conditions = conditions
		version.ValidUntil = version.ValidUntil.UTC()
		version.CreatedAt = version.CreatedAt.UTC()
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

// ReadCommitments reads every commitment version of one case (the own
// read transaction of the views and the command pre-checks).
func (s *Postgres) ReadCommitments(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.CommitmentVersion, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	versions, err := readCommitmentsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return versions, tx.Commit(ctx)
}

// readConfirmationsTx reads every confirmation of one case ordered by the
// confirmation time (the validity derivation input of the panel).
func readConfirmationsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.ConfirmationRecord, error) {
	rows, err := tx.Query(ctx, `
                select commitment_id, case_id, work_item_id, version_no, confirmation_id, subject_kind, subject_ref,
                       method, channel, expires_at, consumed_count, consumption_limit, turn_ref, actor_id, confirmed_at
                from cases.confirmations
                where tenant_id=$1 and case_id=$2 order by confirmed_at`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []casesDomain.ConfirmationRecord{}
	for rows.Next() {
		var record casesDomain.ConfirmationRecord
		if err := rows.Scan(&record.CommitmentID, &record.CaseID, &record.WorkItemID, &record.VersionNo,
			&record.ConfirmationID, &record.SubjectKind, &record.SubjectRef, &record.Method, &record.Channel,
			&record.ExpiresAt, &record.ConsumedCount, &record.ConsumptionLimit, &record.TurnRef,
			&record.ActorID, &record.ConfirmedAt); err != nil {
			return nil, err
		}
		record.TenantID = tenantID
		record.ExpiresAt = record.ExpiresAt.UTC()
		record.ConfirmedAt = record.ConfirmedAt.UTC()
		records = append(records, record)
	}
	return records, rows.Err()
}

// ReadConfirmations reads every confirmation of one case (the own read
// transaction of the derivation).
func (s *Postgres) ReadConfirmations(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.ConfirmationRecord, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	records, err := readConfirmationsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return records, tx.Commit(ctx)
}

// readCommitmentDeclinesTx reads every decline of one case ordered by the
// creation time (the panel history input).
func readCommitmentDeclinesTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesPort.CommitmentDeclineView, error) {
	rows, err := tx.Query(ctx, `
                select decline_id, work_item_id, commitment_id, version_no, outcome, turn_ref, actor_id, created_at
                from cases.commitment_declines
                where tenant_id=$1 and case_id=$2 order by created_at`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	declines := []casesPort.CommitmentDeclineView{}
	for rows.Next() {
		var decline casesPort.CommitmentDeclineView
		if err := rows.Scan(&decline.DeclineID, &decline.WorkItemID, &decline.CommitmentID, &decline.VersionNo,
			&decline.Outcome, &decline.TurnRef, &decline.ActorID, &decline.CreatedAt); err != nil {
			return nil, err
		}
		decline.TenantID = tenantID
		decline.CaseID = caseID
		decline.CreatedAt = decline.CreatedAt.UTC()
		declines = append(declines, decline)
	}
	return declines, rows.Err()
}

// ReadCommitmentDeclines reads every decline of one case (the own read
// transaction of the panel).
func (s *Postgres) ReadCommitmentDeclines(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesPort.CommitmentDeclineView, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	declines, err := readCommitmentDeclinesTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return declines, tx.Commit(ctx)
}

// readPresentedCommitmentsTx reads the presentation links of the outgoing
// sales replies of one case: the (commitment, version) pair of every reply
// row that carries the binding (the presentation gate input of the confirm
// command — the store re-validates the existence under the locks).
func readPresentedCommitmentsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.PresentedCommitment, error) {
	rows, err := tx.Query(ctx, `
                select commitment_id, commitment_version from cases.client_replies
                where tenant_id=$1 and case_id=$2 and commitment_id is not null
                order by created_at`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := []casesDomain.PresentedCommitment{}
	for rows.Next() {
		var link casesDomain.PresentedCommitment
		if err := rows.Scan(&link.CommitmentID, &link.VersionNo); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

// ReadPresentedCommitments reads the presentation links of the case (the
// own read transaction of the views).
func (s *Postgres) ReadPresentedCommitments(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesDomain.PresentedCommitment, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	links, err := readPresentedCommitmentsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return links, tx.Commit(ctx)
}

// readCommitmentKeyTx reads (or creates on the first use) the per-tenant
// commitment HMAC key inside a caller transaction (the ensure pattern of
// readIdentifierKeyTx 2-3: INSERT ON CONFLICT DO NOTHING, then SELECT —
// the INSERT-only discipline, the rotation honestly deferred).
func readCommitmentKeyTx(ctx context.Context, tx pgx.Tx, scope tenancy.Context, now time.Time) ([]byte, error) {
	key, err := casesDomain.NewCommitmentKey()
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.commitment_hmac_keys (tenant_id, key, created_at)
                values ($1,$2,$3) on conflict (tenant_id) do nothing`,
		scope.TenantID(), key, now); err != nil {
		return nil, err
	}
	var stored []byte
	if err = tx.QueryRow(ctx, `
                select key from cases.commitment_hmac_keys where tenant_id=$1`, scope.TenantID()).Scan(&stored); err != nil {
		return nil, err
	}
	return stored, nil
}

// ReadCommitmentKey reads (or creates on the first use) the per-tenant
// commitment key of the content binding (the own committed transaction —
// the app computes the token BEFORE the command transaction; the store
// re-validates the anchors under the locks).
func (s *Postgres) ReadCommitmentKey(ctx context.Context, scope tenancy.Context, now time.Time) ([]byte, error) {
	tx, err := s.begin(ctx, scope, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	key, err := readCommitmentKeyTx(ctx, tx, scope, now.UTC().Truncate(time.Microsecond))
	if err != nil {
		return nil, err
	}
	return key, tx.Commit(ctx)
}

// commitmentTurnSourceOfTx re-validates the turn source under the locks:
// an ACCEPTED TEXT event of the case (the span source anchor — the voice
// turn refuses honestly, the voice contour belongs to the later slices;
// the 2.8 objection precedent).
func commitmentTurnSourceOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, eventID id.UUID) error {
	var state, modality string
	err := tx.QueryRow(ctx, `
                select state, modality from cases.channel_events
                where tenant_id=$1 and event_id=$2 and case_id=$3`,
		tenantID, eventID, caseID).Scan(&state, &modality)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.WorkRuleError{Rule: "confirm_commitment", Reason: casesDomain.TurnRefNotFoundText}
	}
	if err != nil {
		return err
	}
	if state != casesDomain.EventStateAccepted {
		return casesDomain.WorkRuleError{Rule: "confirm_commitment", Reason: "Источник согласия не принят: разметка выдержек возможна только на принятых событиях кейса."}
	}
	if modality != casesDomain.ModalityText {
		return casesDomain.WorkRuleError{Rule: "confirm_commitment", Reason: "Голосовое подтверждение — вне текущего среза: источник согласия должен быть текстовым событием (readback/повтор — будущие срезы)."}
	}
	return nil
}

// commitmentVersionOfTx reads one EXACT version of one commitment under
// the locks (the confirm/decline anchor re-validation: the version must
// exist, be offered and belong to the case).
func commitmentVersionOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, workItemID, commitmentID id.UUID, versionNo int64) (casesDomain.CommitmentVersion, error) {
	var version casesDomain.CommitmentVersion
	var conditions, actionSet, unknowns []byte
	err := tx.QueryRow(ctx, `
                select commitment_id, case_id, work_item_id, version_no, status, conditions, action_set,
                       valid_until, consumption_limit, material_unknowns, owner_kind, owner_ref, owner_epoch,
                       lineage_word, policy_outcome_no, pinned_price_observation_no, content_token, actor_id, created_at
                from cases.commitments
                where tenant_id=$1 and case_id=$2 and work_item_id=$3 and commitment_id=$4 and version_no=$5`,
		tenantID, caseID, workItemID, commitmentID, versionNo).
		Scan(&version.CommitmentID, &version.CaseID, &version.WorkItemID, &version.VersionNo,
			&version.Status, &conditions, &actionSet, &version.ValidUntil, &version.ConsumptionLimit,
			&unknowns, &version.OwnerKind, &version.OwnerRef, &version.OwnerEpoch, &version.LineageWord,
			&version.PolicyOutcomeNo, &version.PinnedPriceObservationNo, &version.ContentToken,
			&version.ActorID, &version.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return version, casesDomain.ErrNotFound
	}
	if err != nil {
		return version, err
	}
	if err := json.Unmarshal(actionSet, &version.ActionSet); err != nil {
		return version, err
	}
	if err := json.Unmarshal(unknowns, &version.MaterialUnknowns); err != nil {
		return version, err
	}
	version.TenantID = tenantID
	version.Conditions = conditions
	version.ValidUntil = version.ValidUntil.UTC()
	version.CreatedAt = version.CreatedAt.UTC()
	return version, nil
}

// execCreateCommitment writes one commitment version under the case +
// work item locks: the Work Item must be anchored in the case and ALIVE
// (the terminal states refuse honestly — the write-command boundary of
// 2-2), the version number is the per-commitment monotonic sequence under
// the lock (max+1 — a new version is a NEW row, the old rows never
// mutate), the material passes the domain guard again (the CHECK words
// backstop the dictionaries) and the outbox event carries the metadata
// only (the material conditions never enter the envelope).
func (s *Postgres) execCreateCommitment(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := modeWorkItemOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	currents, err := readWorkItemCurrentsTx(ctx, tx, tenantID, p.CaseID, []id.UUID{p.WorkItemID})
	if err != nil {
		return err
	}
	current, ok := currents[p.WorkItemID]
	if !ok {
		return casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.TerminalWorkItemCommitmentText}
	}
	// The domain guard under the locks (the honest 400 backstopped by the
	// table CHECK words).
	var terms casesDomain.MaterialTerms
	if err := json.Unmarshal(p.CommitmentConditions, &terms); err != nil {
		return err
	}
	var actions, unknowns []string
	if err := json.Unmarshal(p.CommitmentActionSet, &actions); err != nil {
		return err
	}
	if len(p.CommitmentMaterialUnknowns) > 0 {
		if err := json.Unmarshal(p.CommitmentMaterialUnknowns, &unknowns); err != nil {
			return err
		}
	}
	if err := casesDomain.GuardCreateCommitment(casesDomain.CreateCommitmentCommand{
		Status: p.CommitmentStatus, Terms: terms, ActionSet: actions,
		ValidUntil: p.CommitmentValidUntil, ConsumptionLimit: p.CommitmentConsumptionLimit,
		MaterialUnknowns: unknowns,
	}, now); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The Story 2.13 freeze gate re-validation under the SAME advisory
	// locks (the store half of the app gate — the LIVE derivation of the
	// 00013 rows the app read could race): the effective mode of the Work
	// Item equal to human_requested OR an ACTIVE human handoff freezes the
	// creation with the honest mode word. The control branch stays
	// byte-identical: without the mode and without the handoff rows the
	// gate returns nil and the 2.9 path never differs.
	if err := commitmentHandoffGateOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.Command, now); err != nil {
		return err
	}
	// The per-commitment monotonic version number under the lock (the
	// max+1 of the append-only ladder — a new offer starts at 1; a
	// changed offer of the same commitment id continues the ladder).
	versions, err := readCommitmentsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	versionNo := int64(0)
	for _, version := range versions {
		if version.CommitmentID == p.CommitmentID && version.VersionNo > versionNo {
			versionNo = version.VersionNo
		}
	}
	versionNo++
	// The expected-number CAS of the monotonic ladder: the app resolved the
	// version number for the content token BEFORE the transaction; a raced
	// create moved the ladder — the honest 2.2 stale-version refusal (the
	// operator retries with the fresh material; the token never binds the
	// wrong version bytes).
	if versionNo != p.VersionNo {
		return casesDomain.ErrStaleVersion
	}
	// The observation currency under the locks (the 2.7 race guard the
	// store owns): the pinned price reading must remain the CURRENT
	// observation of the field — a concurrent read_owning_system landed a
	// newer reading between the app gate and this transaction, so the
	// version would pin the old observation (the store never trusts the
	// app's read of the ladder — the fail-closed discipline of the CAS).
	if p.CommitmentPinnedObservationNo > 0 {
		if err := commitmentPriceCurrencyOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.CommitmentPinnedObservationNo, false, p.Command, "повторите создание версии"); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.commitments (tenant_id, case_id, work_item_id, commitment_id, version_no, status,
                        conditions, action_set, valid_until, consumption_limit, material_unknowns,
                        owner_kind, owner_ref, owner_epoch, lineage_word, policy_outcome_no,
                        pinned_price_observation_no, content_token, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		tenantID, p.CaseID, p.WorkItemID, p.CommitmentID, versionNo, p.CommitmentStatus,
		p.CommitmentConditions, p.CommitmentActionSet, p.CommitmentValidUntil, p.CommitmentConsumptionLimit,
		p.CommitmentMaterialUnknowns, p.CommitmentOwnerKind, p.CommitmentOwnerRef, p.CommitmentOwnerEpoch,
		p.CommitmentLineageWord, p.CommitmentPolicyOutcomeNo, p.CommitmentPinnedObservationNo,
		p.CommitmentContentToken, p.Actor, now); err != nil {
		return mapCommitmentInsertError(err)
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	// The outbox event: the metadata only — the material conditions never
	// enter the envelope.
	payload, err := casesDomain.CommitmentVersionCreatedPayloadJSON(casesDomain.CommitmentVersionCreatedPayload{
		TenantID: tenantID, CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		CommitmentID: p.CommitmentID, VersionNo: versionNo, Status: p.CommitmentStatus,
		ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventCommitmentVersionCreated, casesDomain.OutboxEventCommitmentV1,
		p.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyCommitmentVersionCreated(p.CommitmentID, versionNo), now); err != nil {
		return mapCommitmentInsertError(err)
	}
	return nil
}

// commitmentPriceCurrencyOfTx re-validates the freshness pin under the
// locks (the 2.7 race guard): the pinned price observation of one version
// must remain the CURRENT ok reading of the price field of the Work Item —
// a concurrent read_owning_system landed a newer observation between the
// app gate and this transaction, so the version points at the old reading
// and a NEW version is the honest path (fail-closed; the observation row
// itself is immutable, so the app's value/freshness checks hold for its
// bytes). The confirmed exact fact conflict badge is re-derived from the
// lock-visible fact material when the caller demands it (the confirm
// command — the price fact key is locked by the plan, the badge is the
// commit-time truth).
func commitmentPriceCurrencyOfTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, workItemID id.UUID, pin int64, withBadge bool, command, pathText string) error {
	observations, err := readObservationsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return err
	}
	observationsOfField := []casesDomain.Observation{}
	for _, observation := range observations {
		if observation.WorkItemID == workItemID && observation.MaterialKey == casesDomain.MaterialFieldPrice {
			observationsOfField = append(observationsOfField, observation)
		}
	}
	current := casesDomain.CurrentObservation(observationsOfField)
	if current == nil || current.ObservationNo != pin || current.Status != casesDomain.ObservationStatusOk {
		return casesDomain.WorkRuleError{Rule: command, Reason: "Наблюдение цены, на которое указывает версия, больше не является текущим: перечитайте поле (read_owning_system) и " + pathText + "."}
	}
	if !withBadge {
		return nil
	}
	materials, err := readFactsOfCaseTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return err
	}
	for _, material := range materials {
		if material.Anchor.MaterialKey != casesDomain.MaterialFieldPrice {
			continue
		}
		status, versionNo, _ := casesDomain.ConfirmedFactView(material.Versions, material.Confirmations)
		if status != casesDomain.FactStatusConfirmed {
			return nil // no confirmed price fact — the honest no_fact badge
		}
		for i := range material.Versions {
			if material.Versions[i].VersionNo == versionNo {
				if casesDomain.ConflictBadgeOf(current.Value, &material.Versions[i]) == casesDomain.ConflictBadgeDiffers {
					return casesDomain.WorkRuleError{Rule: command, Reason: "Наблюдение цены расходится с подтверждённым фактом (badge differs): разрешите расхождение подтверждением факта и " + pathText + "."}
				}
				return nil
			}
		}
	}
	return nil
}

// execConfirmCommitment writes one confirmation under the case + work
// item locks: the Work Item must sit on the confirm ladder (in_work or
// waiting — the untriaged and the terminal states refuse honestly), the
// EXACT version must exist and be offered, the presentation link must
// exist (a sales reply row that carried the pair — the store re-validates
// the server-side truth), the turn source must be an accepted TEXT event,
// the evidence spans pass the domain guard under the locks (the app
// verified the byte-exact containment BEFORE the transaction), the
// material snapshot and the token land in the row, the Work Item
// transits to confirmed (the single machine step of the slice — the same
// transaction as the confirmation insert) and the outbox event carries
// the metadata only.
func (s *Postgres) execConfirmCommitment(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := modeWorkItemOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	// The optimistic-concurrency anchor of the transit (the 2-2 CAS
	// discipline of every version-writing command: the expected version
	// must match the current one, the stale command refuses honestly).
	current, err := currentForUpdate(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.ExpectedVersion)
	if err != nil {
		return err
	}
	if current.Terminal {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.TerminalWorkItemCommitmentText}
	}
	if current.State != casesDomain.WorkItemStateInWork && current.State != casesDomain.WorkItemStateWaiting {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "Подтверждение допустимо только из состояний in_work и waiting (лестница состояний 2-2): классифицируйте элемент работы."}
	}
	// The EXACT version anchor under the locks: it must exist, be offered
	// and belong to this work item of this case.
	version, err := commitmentVersionOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.CommitmentID, p.VersionNo)
	if err != nil {
		return err
	}
	if version.Status != casesDomain.CommitmentStatusOffered {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "Черновик не считается подтверждаемым обязательством: предъявите offered-версию."}
	}
	// The presentation link under the locks: a client reply row of THIS
	// case carried the exact pair (the shown version — AC-6).
	presented := false
	links, err := readPresentedCommitmentsTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	for _, link := range links {
		if link.CommitmentID == p.CommitmentID && link.VersionNo == p.VersionNo {
			presented = true
			break
		}
	}
	if !presented {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "Версия не была предъявлена клиенту: подтверждается только показанная версия (связка сообщения-предъявления обязательна)."}
	}
	// The turn source anchor under the locks (the accepted TEXT event).
	if err := commitmentTurnSourceOfTx(ctx, tx, tenantID, p.CaseID, p.TurnRef); err != nil {
		return err
	}
	// The evidence guard under the locks (the honest 400 of the invented
	// consent — the app verified the byte-exact containment against the
	// sealed payload BEFORE the transaction; the store re-validates the
	// shape).
	var spans []string
	if len(p.CommitmentEvidenceSpans) > 0 {
		if err := json.Unmarshal(p.CommitmentEvidenceSpans, &spans); err != nil {
			return err
		}
	}
	if err := casesDomain.GuardConfirmEvidence(casesDomain.ConfirmCommitmentMaterial{
		SubjectKind: p.CommitmentSubjectKind, SubjectRef: p.CommitmentSubjectRef,
		TurnPayload: p.CommitmentTurnPayload, Spans: spans,
	}); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The freshness re-validation under the locks: the pinned price
	// observation must remain the CURRENT ok reading of the field AND the
	// conflict badge against the confirmed exact fact is re-derived from
	// the lock-visible fact material (the price fact writers serialize on
	// the fact key this command locks — the badge is the commit-time
	// truth, not the app's read-time snapshot; a moved reading or a
	// differs badge refuses — the new version is the honest path).
	if !casesDomain.MaterialUnknownTerm(version.MaterialUnknowns, casesDomain.CommitmentTermPrice) {
		if err := commitmentPriceCurrencyOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, version.PinnedPriceObservationNo, true, p.Command, "создайте новую версию обязательства"); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.confirmations (tenant_id, case_id, work_item_id, commitment_id, version_no, confirmation_id,
                        subject_kind, subject_ref, method, channel, expires_at, consumed_count, consumption_limit, turn_ref,
                        evidence_spans, material_snapshot, action_set_snapshot, content_token, actor_id, confirmed_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		tenantID, p.CaseID, p.WorkItemID, p.CommitmentID, p.VersionNo, p.ConfirmationID,
		p.CommitmentSubjectKind, p.CommitmentSubjectRef, casesDomain.CommitmentMethodExplicitText,
		p.Channel, p.CommitmentExpiresAt, 0, p.CommitmentConsumptionLimit, p.TurnRef,
		p.CommitmentEvidenceSpans, p.CommitmentMaterialSnapshot, p.CommitmentActionSet,
		p.CommitmentContentToken, p.Actor, now); err != nil {
		return mapCommitmentInsertError(err)
	}
	// The Work Item transit to confirmed: the single machine step of the
	// slice (the same transaction — the material atomicity; the reason
	// word and the confirmation reference of the history row).
	if err := insertWorkItemVersion(ctx, tx, tenantID, p.CaseID, current.ConversationID, p.WorkItemID,
		p.ExpectedVersion+1, casesDomain.VersionKindTransition, "", "", casesDomain.WorkItemStateConfirmed,
		"commitment_confirmed", p.Actor, p.ConfirmationID, now); err != nil {
		return mapWorkInsertError(p.Command, err)
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	// The outbox event: the metadata only — the spans and the material
	// never enter the envelope.
	payload, err := casesDomain.CommitmentConfirmedPayloadJSON(casesDomain.CommitmentConfirmedPayload{
		TenantID: tenantID, CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		CommitmentID: p.CommitmentID, VersionNo: p.VersionNo, ConfirmationID: p.ConfirmationID,
		SubjectKind: p.CommitmentSubjectKind, SubjectRef: p.CommitmentSubjectRef,
		ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventCommitmentConfirmed, casesDomain.OutboxEventCommitmentV1,
		p.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyCommitmentConfirmed(p.ConfirmationID), now); err != nil {
		return mapCommitmentInsertError(err)
	}
	return nil
}

// execRecordCommitmentDecline writes one decline under the case + work
// item locks: the Work Item must be anchored in the case and ALIVE, the
// EXACT version must exist, the turn source must be an accepted TEXT
// event, the evidence spans pass the domain guard and the outbox event
// carries the metadata only. The decline mutates NOTHING (AC-11): no
// state, no version, no confirmation changes.
func (s *Postgres) execRecordCommitmentDecline(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if err := modeWorkItemOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	currents, err := readWorkItemCurrentsTx(ctx, tx, tenantID, p.CaseID, []id.UUID{p.WorkItemID})
	if err != nil {
		return err
	}
	current, ok := currents[p.WorkItemID]
	if !ok {
		return casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.TerminalWorkItemCommitmentText}
	}
	// The EXACT version anchor under the locks: it must exist and belong
	// to this work item of this case (the decline records the honest
	// lifecycle word of the SHOWN material).
	if _, err := commitmentVersionOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.CommitmentID, p.VersionNo); err != nil {
		return err
	}
	// The turn source anchor under the locks.
	if err := commitmentTurnSourceOfTx(ctx, tx, tenantID, p.CaseID, p.TurnRef); err != nil {
		return err
	}
	// The evidence guard under the locks.
	var spans []string
	if len(p.CommitmentEvidenceSpans) > 0 {
		if err := json.Unmarshal(p.CommitmentEvidenceSpans, &spans); err != nil {
			return err
		}
	}
	if err := casesDomain.GuardDeclineCommitment(p.CommitmentDeclineOutcome, casesDomain.ConfirmCommitmentMaterial{
		SubjectKind: "client", SubjectRef: p.CommitmentSubjectRef,
		TurnPayload: p.CommitmentTurnPayload, Spans: spans,
	}); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.commitment_declines (tenant_id, case_id, work_item_id, commitment_id, version_no,
                        decline_id, outcome, turn_ref, evidence_spans, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.CaseID, p.WorkItemID, p.CommitmentID, p.VersionNo,
		p.DeclineID, p.CommitmentDeclineOutcome, p.TurnRef, p.CommitmentEvidenceSpans, p.Actor, now); err != nil {
		return mapCommitmentInsertError(err)
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	// The outbox event: the metadata only — the spans never enter the
	// envelope.
	payload, err := casesDomain.CommitmentDeclinedPayloadJSON(casesDomain.CommitmentDeclinedPayload{
		TenantID: tenantID, CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		CommitmentID: p.CommitmentID, VersionNo: p.VersionNo, Outcome: p.CommitmentDeclineOutcome,
		ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.OutboxEventID, casesDomain.OutboxEventCommitmentDeclined, casesDomain.OutboxEventCommitmentV1,
		p.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyCommitmentDeclined(p.DeclineID), now); err != nil {
		return mapCommitmentInsertError(err)
	}
	return nil
}
