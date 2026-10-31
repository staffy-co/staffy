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
// Story 2.15: the fact correction write and the correction state read
// ---------------------------------------------------------------------------

// TerminalCaseCorrectionText is the honest refusal of the correction write
// on a terminal case: the service is over — the history of the facts is
// not rewritten, the material stays readable (the read path returns the
// material honestly; the privacy deletion contour is a future slice).
const TerminalCaseCorrectionText = "Терминальное обращение не принимает записей коррекции: обслуживание завершено — материал читается историей, исправление не записывается."

// CorrectionTargetForeignText is the honest refusal of the target guard:
// the pinned fact (with its EXACT version) belongs to THIS case — a
// foreign or missing target is the precision failure the store refuses
// (AC-8: the correction pins the exact version of the fact of this
// case; the ambiguity never arises).
const CorrectionTargetForeignText = "Цель коррекции — точная версия факта этого обращения: факт не найден или принадлежит другому обращению, версия не существует у этого факта."

// CorrectionSummaryForeignText is the honest refusal of the summary link
// guard: the link >=1 pins the EXACT summary version of THIS case (the
// deferred-work closure of 2.14 — the correction asserts the statement
// of that summary version).
const CorrectionSummaryForeignText = "Сводка коррекции не найдена или принадлежит другому обращению: связь утверждает точную версию сводки этого обращения."

// mapCorrectionInsertError maps the raced insert conflicts of the
// correction surface onto the honest domain errors: the correction
// number UNIQUE is the raced derivation (the loser re-derives under the
// locks — the honest stale-retry signal); the command record and the
// pour conflicts map through mapWorkInsertError (the command row is
// inserted AFTER the exec by the work flow; a defensive pass-through
// keeps the fallback honest).
func mapCorrectionInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "case_corrections_case_no":
			// The raced correction number: the loser re-derives under the
			// locks and retries (the honest stale-retry signal).
			return casesDomain.ErrStaleVersion
		case "case_corrections_pkey":
			// The raced duplicate of the SAME correction row id (the app
			// replay never reaches here — the command record resolves
			// first; the honest retry signal of a raced double-submit).
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

// scanCorrectionRecord reads one append-only correction row (the spans
// travel the canonical jsonb array of the domain material).
func scanCorrectionRecord(scan func(dest ...any) error, tenantID id.UUID) (casesDomain.CorrectionRecord, error) {
	var record casesDomain.CorrectionRecord
	var spans []byte
	var summaryNo *int64
	err := scan(&record.CaseID, &record.CorrectionID, &record.CorrectionNo,
		&record.FactID, &record.VersionNo, &summaryNo, &record.ProposedValue, &spans,
		&record.Outcome, &record.ActorID, &record.Channel, &record.Reason,
		&record.ObservedAt, &record.ReceivedAt, &record.CreatedAt)
	if err != nil {
		return record, err
	}
	record.TenantID = tenantID
	if summaryNo != nil {
		record.SummaryNo = *summaryNo
	}
	record.ObservedAt = record.ObservedAt.UTC()
	record.ReceivedAt = record.ReceivedAt.UTC()
	record.CreatedAt = record.CreatedAt.UTC()
	if len(spans) > 0 {
		if err := json.Unmarshal(spans, &record.Spans); err != nil {
			return record, err
		}
	}
	if record.Spans == nil {
		record.Spans = []string{}
	}
	return record, nil
}

// readCorrectionStateTx reads the correction rows of the case ordered by
// the monotonic correction numbers (the per-fact LAST-correction
// derivations group them read-time — the store never derives, the rows
// are the append-only truth).
func readCorrectionStateTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesDomain.CorrectionRecord, error) {
	rows, err := tx.Query(ctx, `
                select case_id, correction_id, correction_no, fact_id, version_no, summary_no,
                       proposed_value, spans, outcome, actor_id, channel, reason,
                       observed_at, received_at, created_at
                from cases.case_corrections
                where tenant_id=$1 and case_id=$2 order by correction_no`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []casesDomain.CorrectionRecord{}
	for rows.Next() {
		record, err := scanCorrectionRecord(rows.Scan, tenantID)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

// ReadDecisionContextWithCorrections reads the ONE consistent snapshot of
// the decision context material TOGETHER with the correction rows of the
// case (the same read transaction — the fact lines of the view derive
// the correction markers over the consistent material; the advisory
// locks are NOT taken, the consistent snapshot of one transaction is the
// honest boundary of the GET).
func (s *Postgres) ReadDecisionContextWithCorrections(ctx context.Context, scope tenancy.Context, caseID id.UUID, now time.Time) (casesPort.DecisionContextMaterial, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return casesPort.DecisionContextMaterial{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	input, err := readDecisionContextTx(ctx, tx, scope.TenantID(), caseID, now)
	if err != nil {
		return casesPort.DecisionContextMaterial{}, err
	}
	corrections, err := readCorrectionStateTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return casesPort.DecisionContextMaterial{}, err
	}
	return casesPort.DecisionContextMaterial{Input: input, Corrections: corrections}, tx.Commit(ctx)
}

// correctionSourceTextsTx resolves the evidence source texts of the
// correction under the locks: the summary link >=1 pins the EXACT
// summary version of THIS case (its text is the cited source); the link
// 0 cites the accepted event payloads of the case (the sealed provider
// bytes — the same span discipline of 2.8; the read re-validates the
// byte-exact presence the app checked before the transaction).
func correctionSourceTextsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID, summaryNo int64, summaryText string) []string {
	if summaryNo >= 1 {
		if summaryText == "" {
			return nil
		}
		return []string{summaryText}
	}
	rows, err := tx.Query(ctx, `
                select p.payload from cases.channel_event_payloads p
                join cases.channel_events e on e.tenant_id=p.tenant_id and e.event_id=p.event_id
                where e.tenant_id=$1 and e.case_id=$2 and e.state='accepted'`, tenantID, caseID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	texts := []string{}
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			continue
		}
		texts = append(texts, string(payload))
	}
	return texts
}

// execRecordCorrection writes one append-only correction row under the
// case + conversation + fact locks (the sorted lock plan of the
// 2.5..2.14 precedent with the fact keys of the pour — the
// confirm_commitment precedent: the pour through the EXISTING fact
// contour serializes with every fact writer of the same anchor): the
// case must exist under the tenant scope and stay open (the terminal
// backstop — the read stays honest), the target guards re-validate the
// pinned fact version of THIS case (the anchor, the version pin, the
// summary link), the domain guard re-validates the material under the
// locks (the fact value discipline, the byte-exact spans against the
// cited source, the mandatory reason), the matrix gate decides the
// outcome (the server truth — the request never carries it) and the
// correction number is the NEXT monotonic number of the case (the
// UNIQUE backstop catches the race the locks already serialize). The
// applied outcome POURS the proposed value as the new fact version
// through the existing record_fact discipline (the operator source, the
// correction:<id> source key — the material idempotency of the ladder;
// NEVER an automatic confirmation); the disputed outcome writes NO fact
// version (the system value of the Owning System is never rewritten).
// The outbox envelope carries the identifiers, the outcome word and the
// span counter ONLY — the proposed value and the spans NEVER enter.
func (s *Postgres) execRecordCorrection(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	var caseState string
	err := tx.QueryRow(ctx, `
                select state from cases.cases where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&caseState)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if caseState != casesDomain.CaseStateOpen {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: TerminalCaseCorrectionText}
	}
	// The target guards: the fact anchor belongs to THIS case (the
	// material key re-read under the locks — the app's pre-read key may
	// lag the anchor truth; the pour uses the anchor key of the locks).
	var anchorCase id.UUID
	var materialKey string
	err = tx.QueryRow(ctx, `
                select case_id, material_key from cases.facts where tenant_id=$1 and fact_id=$2`,
		tenantID, p.FactID).Scan(&anchorCase, &materialKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: CorrectionTargetForeignText}
	}
	if err != nil {
		return err
	}
	if anchorCase != p.CaseID {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: CorrectionTargetForeignText}
	}
	// The version pin: the fact ladder with the confirmations is the
	// lock-visible truth (the pour scope is the CURRENT scope of the
	// latest version — the rescope may have moved it since the pinned
	// version; the rescope discipline of 2.2 keeps the same-case scope).
	material, err := readFactMaterialTx(ctx, tx, tenantID, p.CaseID, p.FactID)
	if err != nil {
		if errors.Is(err, casesDomain.ErrNotFound) {
			return casesDomain.WorkRuleError{Rule: p.Command, Reason: CorrectionTargetForeignText}
		}
		return err
	}
	versions := int64(len(material.Versions))
	versionExists := false
	var scopeWorkItem id.UUID
	for _, version := range material.Versions {
		if version.VersionNo == p.VersionNo {
			versionExists = true
		}
	}
	if versions == 0 || p.VersionNo < 1 || p.VersionNo > versions || !versionExists {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: CorrectionTargetForeignText}
	}
	scopeWorkItem = material.Versions[versions-1].WorkItemID
	// The work item boundary of the pour: the record_fact discipline
	// refuses the fact write into a terminal scope (the same honest
	// boundary as the operator record branch).
	scope, err := currentForUpdate(ctx, tx, tenantID, p.CaseID, scopeWorkItem, -1)
	if err != nil {
		return err
	}
	if scope.Terminal {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "факт нельзя записать в скоуп элемента в состоянии " + scope.State}
	}
	// The summary link guard: the link >=1 pins the summary of THIS case;
	// its text is the cited evidence source of the spans.
	summaryText := ""
	if p.SummaryNo >= 1 {
		err = tx.QueryRow(ctx, `
                        select text from cases.case_summaries where tenant_id=$1 and case_id=$2 and summary_no=$3`,
			tenantID, p.CaseID, p.SummaryNo).Scan(&summaryText)
		if errors.Is(err, pgx.ErrNoRows) {
			return casesDomain.WorkRuleError{Rule: p.Command, Reason: CorrectionSummaryForeignText}
		}
		if err != nil {
			return err
		}
	}
	// The domain guards under the locks: the version pin, the summary
	// link, the fact value discipline, the byte-exact spans against the
	// cited source and the mandatory reason; the 2.2 times.
	sourceTexts := correctionSourceTextsTx(ctx, tx, tenantID, p.CaseID, p.SummaryNo, summaryText)
	if err := casesDomain.GuardRecordCorrection(int(versions), p.VersionNo, p.SummaryNo, p.Value, p.Spans, sourceTexts, p.Reason); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	if err := casesDomain.GuardCorrectionTimes(p.ObservedAt, p.ReceivedAt); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The matrix gate: the SERVER decision of the outcome (the request
	// never carries it — a matrix material field disputes, every other
	// key pours).
	outcome := casesDomain.DeriveCorrectionOutcome(materialKey)
	// The correction number: the NEXT monotonic number under the locks
	// (the request never carries it; the UNIQUE backstop catches the race).
	var correctionNo int64
	if err = tx.QueryRow(ctx, `
                select coalesce(max(correction_no),0)+1 from cases.case_corrections
                where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&correctionNo); err != nil {
		return err
	}
	spans, err := json.Marshal(p.Spans)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.case_corrections (tenant_id, case_id, correction_id, correction_no, fact_id, version_no,
                        summary_no, proposed_value, spans, outcome, actor_id, channel, reason, observed_at, received_at, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9::jsonb,$10,$11,$12,$13,$14,$15,$16)`,
		tenantID, p.CaseID, p.CorrectionID, correctionNo, p.FactID, p.VersionNo,
		nilInt64(p.SummaryNo), p.Value, spans, outcome, p.Actor, "operator", p.Reason,
		p.ObservedAt.UTC().Truncate(time.Microsecond), p.ReceivedAt.UTC().Truncate(time.Microsecond),
		now.UTC().Truncate(time.Microsecond)); err != nil {
		return mapCorrectionInsertError(err)
	}
	// The applied pour: the new fact version through the EXISTING record
	// discipline (the operator source, the correction:<id> source key —
	// the material idempotency of the ladder; NEVER a confirmation: the
	// new version stays conflicting/unverified by DeriveFactStatus, the
	// resolution is the existing confirm_fact_version of a human). The
	// disputed outcome writes NO fact version (the system value of the
	// Owning System is never rewritten — the matrix gate closed it).
	if outcome == casesDomain.CorrectionOutcomeApplied {
		if err := insertFactVersion(ctx, tx, tenantID, p.CaseID, p.FactID, scopeWorkItem, versions+1,
			materialKey, p.Value, p.ValueDigest, casesDomain.FactSourceOperator,
			casesDomain.CorrectionFactSourceID(p.CorrectionID), "operator", p.Actor,
			p.ObservedAt, p.ReceivedAt, now); err != nil {
			return mapWorkInsertError(p.Command, err)
		}
	}
	// The correction envelope: the identifiers, the outcome word and the
	// span counter only — the proposed value and the spans NEVER enter.
	payload, err := casesDomain.CorrectionRecordedPayloadJSON(casesDomain.CorrectionRecordedPayload{
		TenantID: tenantID, CaseID: p.CaseID, CorrectionID: p.CorrectionID, CorrectionNo: correctionNo,
		FactID: p.FactID, VersionNo: p.VersionNo, SummaryNo: p.SummaryNo, Outcome: outcome,
		SpanCount: len(p.Spans), ActorID: p.Actor, OccurredAt: now,
	})
	if err != nil {
		return err
	}
	var sourceConnection id.UUID
	if err := tx.QueryRow(ctx, `
                select connection_id from cases.conversations
                where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
		tenantID, p.CaseID).Scan(&sourceConnection); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tenantID, p.CorrectionOutboxEventID, casesDomain.OutboxEventCorrectionRecorded, casesDomain.OutboxEventCorrectionRecordedV1,
		p.CaseID, sourceConnection, p.CorrelationID, now, payload,
		casesDomain.DedupKeyCorrectionRecorded(p.CaseID, correctionNo), now); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
			// The raced outbox envelope of the same correction number (the
			// replay never reaches here — the command record resolves before
			// the exec; the honest stale-retry signal).
			return casesDomain.ErrStaleVersion
		}
		return err
	}
	return nil
}
