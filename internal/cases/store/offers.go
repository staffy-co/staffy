package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.5: the supplier offer and the selection snapshot reads and the
// record command executions
// ---------------------------------------------------------------------------

// mapOfferInsertError maps the raced insert conflicts of the offer
// surfaces onto the honest domain errors: the monotonic number UNIQUEs
// are the append-only backstops (the loser under the locks is impossible
// in the normal path — the backstops keep the material truth anyway), the
// command record UNIQUE is the material idempotency conflict.
func mapOfferInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		switch pgErr.ConstraintName {
		case "command_records_pkey":
			return casesDomain.ErrCommandKeyConflict
		case "supplier_offers_tenant_id_work_item_id_position_key_supplier_id_offer_no_key":
			return casesDomain.ErrStaleVersion
		case "selection_snapshots_tenant_id_work_item_id_snapshot_no_key":
			return casesDomain.ErrStaleVersion
		}
	}
	return err
}

// readSupplierOffersTx reads every supplier offer record of one case
// ordered by the work item, the position, the supplier and the monotonic
// number (the derivation input — the read-time comparison elects the
// current record of every supplier by the number).
func readSupplierOffersTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesPort.SupplierOfferRecord, error) {
	rows, err := tx.Query(ctx, `
		select work_item_id, offer_id, offer_no, position_key, supplier_id, supplier_name, brand, article,
		       client_price, purchase_price, currency, quantity_available, term_days, term_kind, conditions,
		       source_system, source_record_id, source_version, source_observed_at, received_at,
		       profile_digest_pin, part_request_digest_pin, fitment_digest_pin, offer_digest, created_at
		from cases.supplier_offers
		where tenant_id=$1 and case_id=$2 order by work_item_id, position_key, supplier_id, offer_no`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []casesPort.SupplierOfferRecord{}
	for rows.Next() {
		var record casesPort.SupplierOfferRecord
		var conditions, fitmentPin *string
		var termDays *int
		if err := rows.Scan(&record.WorkItemID, &record.OfferID, &record.OfferNo, &record.PositionKey,
			&record.SupplierID, &record.SupplierName, &record.Brand, &record.Article,
			&record.ClientPrice, &record.PurchasePrice, &record.Currency, &record.QuantityAvailable, &termDays, &record.TermKind, &conditions,
			&record.SourceSystem, &record.SourceRecordID, &record.SourceVersion, &record.SourceObservedAt, &record.ReceivedAt,
			&record.ProfileDigestPin, &record.PartRequestDigestPin, &fitmentPin, &record.OfferDigest, &record.CreatedAt); err != nil {
			return nil, err
		}
		record.TermDays = termDays
		if conditions != nil {
			record.Conditions = *conditions
		}
		if fitmentPin != nil {
			record.FitmentDigestPin = *fitmentPin
		}
		record.SourceObservedAt = record.SourceObservedAt.UTC()
		record.ReceivedAt = record.ReceivedAt.UTC()
		record.CreatedAt = record.CreatedAt.UTC()
		records = append(records, record)
	}
	return records, rows.Err()
}

// ReadSupplierOffersOfCase reads every supplier offer record of the case
// (the own read transaction of the view and the command pre-checks).
func (s *Postgres) ReadSupplierOffersOfCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesPort.SupplierOfferRecord, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	records, err := readSupplierOffersTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return records, tx.Commit(ctx)
}

// readSelectionSnapshotsTx reads every selection snapshot record of one
// case ordered by the work item and the monotonic number (the recorded
// client choices — the material of the order contour of 4.6).
func readSelectionSnapshotsTx(ctx context.Context, tx pgx.Tx, tenantID, caseID id.UUID) ([]casesPort.SelectionSnapshotRecord, error) {
	rows, err := tx.Query(ctx, `
		select work_item_id, snapshot_id, snapshot_no, position_key, supplier_id, offer_no, offer_digest_pin,
		       quantity, unit_client_price, currency, known_client_sum,
		       profile_digest_pin, part_request_digest_pin, fitment_digest_pin, selection_digest, created_at
		from cases.selection_snapshots
		where tenant_id=$1 and case_id=$2 order by work_item_id, snapshot_no`, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []casesPort.SelectionSnapshotRecord{}
	for rows.Next() {
		var record casesPort.SelectionSnapshotRecord
		if err := rows.Scan(&record.WorkItemID, &record.SnapshotID, &record.SnapshotNo, &record.PositionKey,
			&record.SupplierID, &record.OfferNo, &record.OfferDigestPin,
			&record.Quantity, &record.UnitClientPrice, &record.Currency, &record.KnownClientSum,
			&record.ProfileDigestPin, &record.PartRequestDigestPin, &record.FitmentDigestPin, &record.SelectionDigest, &record.CreatedAt); err != nil {
			return nil, err
		}
		record.CreatedAt = record.CreatedAt.UTC()
		records = append(records, record)
	}
	return records, rows.Err()
}

// ReadSelectionSnapshotsOfCase reads every selection snapshot record of
// the case (the own read transaction of the view).
func (s *Postgres) ReadSelectionSnapshotsOfCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]casesPort.SelectionSnapshotRecord, error) {
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	records, err := readSelectionSnapshotsTx(ctx, tx, scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	return records, tx.Commit(ctx)
}

// offerNextNoTx resolves the next monotonic offer number of one position
// of one supplier of the Work Item under the locks (the max+1 of the
// append-only ladder — the supersede is the NEW number, never an update).
func offerNextNoTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, workItemID id.UUID, positionKey, supplierID string) (int64, error) {
	records, err := readSupplierOffersTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return 0, err
	}
	nextNo := int64(0)
	for _, record := range records {
		if record.WorkItemID == workItemID && record.PositionKey == positionKey && record.SupplierID == supplierID && record.OfferNo > nextNo {
			nextNo = record.OfferNo
		}
	}
	return nextNo + 1, nil
}

// selectionNextNoTx resolves the next monotonic snapshot number of one
// Work Item under the locks (the max+1 of the append-only ladder — the
// changed mind is a NEW row with a NEW number).
func selectionNextNoTx(ctx context.Context, tx pgx.Tx, tenantID, caseID, workItemID id.UUID) (int64, error) {
	records, err := readSelectionSnapshotsTx(ctx, tx, tenantID, caseID)
	if err != nil {
		return 0, err
	}
	nextNo := int64(0)
	for _, record := range records {
		if record.WorkItemID == workItemID && record.SnapshotNo > nextNo {
			nextNo = record.SnapshotNo
		}
	}
	return nextNo + 1, nil
}

// execRecordSupplierOffer writes ONE append-only supplier offer record
// under the case + work item locks: the Work Item must be anchored in the
// case and ALIVE, the guard material re-validates under the locks (the
// store never trusts the app bytes — the SQL CHECK words are the
// backstops), the canonical OfferDigest re-derives under the locks (the
// same bytes — the server truth of the record material), the offer number
// is the per-(work item, position, supplier) monotonic sequence under the
// lock and the record materializes as the append-only row. NO outbox
// event: the commercial conditions stay inside the case perimeter (the
// append-only row is the audit; the envelope vocabulary is not extended by
// this story).
func (s *Postgres) execRecordSupplierOffer(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if p.Offer == nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "материал записи предложения отсутствует"}
	}
	if err := modeWorkItemOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	// The Work Item must be alive: the terminal states refuse honestly.
	currents, err := readWorkItemCurrentsTx(ctx, tx, tenantID, p.CaseID, []id.UUID{p.WorkItemID})
	if err != nil {
		return err
	}
	current, ok := currents[p.WorkItemID]
	if !ok {
		return casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.TerminalWorkItemReadText}
	}
	// The domain guard re-validation under the locks: the dictionaries and
	// the material shape of the record (the store never trusts the app
	// bytes; the SQL CHECK mirrors are the backstops).
	draft := casesDomain.SupplierOfferDraft{
		PositionKey:          p.Offer.PositionKey,
		SupplierID:           p.Offer.SupplierID,
		SupplierName:         p.Offer.SupplierName,
		Brand:                p.Offer.Brand,
		Article:              p.Offer.Article,
		ClientPrice:          p.Offer.ClientPrice,
		PurchasePrice:        p.Offer.PurchasePrice,
		Currency:             p.Offer.Currency,
		QuantityAvailable:    p.Offer.QuantityAvailable,
		TermDays:             p.Offer.TermDays,
		Conditions:           p.Offer.Conditions,
		SourceSystem:         p.Offer.SourceSystem,
		SourceRecordID:       p.Offer.SourceRecordID,
		SourceVersion:        p.Offer.SourceVersion,
		SourceObservedAt:     p.Offer.SourceObservedAt,
		ProfileDigestPin:     p.Offer.ProfileDigestPin,
		PartRequestDigestPin: p.Offer.PartRequestDigestPin,
		FitmentDigestPin:     p.Offer.FitmentDigestPin,
	}
	policy := casesDomain.ComparisonPolicyDomain{
		Currencies:         p.ComparisonPolicy.Currencies,
		MaxOfferAgeSeconds: p.ComparisonPolicy.MaxOfferAgeSeconds,
		RankCriteria:       p.ComparisonPolicy.RankCriteria,
		RankCriteriaLabels: p.ComparisonPolicy.RankCriteriaLabels,
		TieBreakers:        p.ComparisonPolicy.TieBreakers,
		MaxVisibleVariants: p.ComparisonPolicy.MaxVisibleVariants,
	}
	if err := casesDomain.GuardSupplierOffer(policy, draft); err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The monotonic offer number under the locks (the max+1 of the
	// per-(work item, position, supplier) ladder — the supersede is the NEW
	// number, never an update).
	offerNo, err := offerNextNoTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID, p.Offer.PositionKey, p.Offer.SupplierID)
	if err != nil {
		return err
	}
	// The canonical OfferDigest derives under the locks from the NUMBERED
	// record material — the SERVER truth (the digest pins the monotonic
	// number; the purchase price is NOT the digest material — the internal
	// data never changes the client-relevant version of the conditions).
	offer := casesDomain.SupplierOfferInput{
		OfferNo:              offerNo,
		PositionKey:          p.Offer.PositionKey,
		SupplierID:           p.Offer.SupplierID,
		SupplierName:         p.Offer.SupplierName,
		Brand:                p.Offer.Brand,
		Article:              p.Offer.Article,
		ClientPrice:          p.Offer.ClientPrice,
		PurchasePrice:        p.Offer.PurchasePrice,
		Currency:             p.Offer.Currency,
		QuantityAvailable:    p.Offer.QuantityAvailable,
		TermDays:             p.Offer.TermDays,
		Conditions:           p.Offer.Conditions,
		SourceSystem:         p.Offer.SourceSystem,
		SourceRecordID:       p.Offer.SourceRecordID,
		SourceVersion:        p.Offer.SourceVersion,
		SourceObservedAt:     p.Offer.SourceObservedAt,
		ReceivedAt:           p.ReceivedAt,
		ProfileDigestPin:     p.Offer.ProfileDigestPin,
		PartRequestDigestPin: p.Offer.PartRequestDigestPin,
		FitmentDigestPin:     p.Offer.FitmentDigestPin,
	}
	offerDigest := casesDomain.OfferDigestOf(offer)
	if _, err = tx.Exec(ctx, `
		insert into cases.supplier_offers (tenant_id, case_id, work_item_id, offer_id, offer_no,
			position_key, supplier_id, supplier_name, brand, article,
			client_price, purchase_price, currency, quantity_available, term_days, term_kind, conditions,
			source_system, source_record_id, source_version, source_observed_at, received_at,
			actor_id, correlation_id, profile_digest_pin, part_request_digest_pin, fitment_digest_pin, offer_digest, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29)`,
		tenantID, p.CaseID, p.WorkItemID, p.Offer.OfferID, offerNo,
		p.Offer.PositionKey, p.Offer.SupplierID, p.Offer.SupplierName, p.Offer.Brand, p.Offer.Article,
		p.Offer.ClientPrice, nilFloat(p.Offer.PurchasePrice), p.Offer.Currency, p.Offer.QuantityAvailable, nilInt(p.Offer.TermDays), casesDomain.OfferTermKindSupplierEstimate, nilText(p.Offer.Conditions),
		p.Offer.SourceSystem, p.Offer.SourceRecordID, p.Offer.SourceVersion, p.Offer.SourceObservedAt, p.ReceivedAt,
		p.Actor, p.CorrelationID, p.Offer.ProfileDigestPin, p.Offer.PartRequestDigestPin, nilText(p.Offer.FitmentDigestPin), offerDigest, now); err != nil {
		return mapOfferInsertError(err)
	}
	return nil
}

// execRecordClientSelection writes ONE append-only selection snapshot
// record under the case + work item locks: the Work Item must be anchored
// in the case and ALIVE, the guard material re-validates under the locks
// (the chosen CURRENT offer of the supplier — the store re-elects it by
// the maximal number and re-runs the whole selection guard over the
// server truth: the proven fitment pins come from the command material,
// the offer material never does), the canonical SelectionDigest
// re-derives under the locks and the record materializes as the
// append-only row. NO outbox event and NO ABCP Order: the choice stays
// inside the case perimeter (the order contour of 4.6 builds its own
// Business Action — the fence OQ-3/OQ-4).
func (s *Postgres) execRecordClientSelection(ctx context.Context, tx pgx.Tx, tenantID id.UUID, p casesPort.WorkCommandParams, now time.Time) error {
	if p.Selection == nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "материал записи выбора отсутствует"}
	}
	if err := modeWorkItemOfTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID); err != nil {
		return err
	}
	// The Work Item must be alive: the terminal states refuse honestly.
	currents, err := readWorkItemCurrentsTx(ctx, tx, tenantID, p.CaseID, []id.UUID{p.WorkItemID})
	if err != nil {
		return err
	}
	current, ok := currents[p.WorkItemID]
	if !ok {
		return casesDomain.ErrNotFound
	}
	if current.Terminal {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.TerminalWorkItemReadText}
	}
	// The chosen offer re-election under the locks: the CURRENT record of
	// the (work item, position, supplier) ladder by the maximal number —
	// the store never trusts the app's offer bytes, the whole selection
	// guard re-runs over the server truth.
	offers, err := readSupplierOffersTx(ctx, tx, tenantID, p.CaseID)
	if err != nil {
		return err
	}
	var chosen *casesPort.SupplierOfferRecord
	for index := range offers {
		offer := offers[index]
		if offer.WorkItemID != p.WorkItemID || offer.PositionKey != p.Selection.PositionKey || offer.SupplierID != p.Selection.SupplierID {
			continue
		}
		if chosen == nil || offer.OfferNo > chosen.OfferNo {
			chosen = &offers[index]
		}
	}
	if chosen == nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.OfferSelectionGuardStaleWord}
	}
	if chosen.OfferNo != p.Selection.OfferNo || chosen.OfferDigest != p.Selection.OfferDigestPin {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: casesDomain.OfferSelectionGuardStaleWord}
	}
	// The domain guard re-validation under the locks over the server truth
	// (the proven fitment material travels with the command — the app
	// derived it from the LIVE 4.4 derivation; the offer material is the
	// store's own re-elected record).
	policy := casesDomain.ComparisonPolicyDomain{
		Currencies:         p.ComparisonPolicy.Currencies,
		MaxOfferAgeSeconds: p.ComparisonPolicy.MaxOfferAgeSeconds,
		RankCriteria:       p.ComparisonPolicy.RankCriteria,
		RankCriteriaLabels: p.ComparisonPolicy.RankCriteriaLabels,
		TieBreakers:        p.ComparisonPolicy.TieBreakers,
		MaxVisibleVariants: p.ComparisonPolicy.MaxVisibleVariants,
	}
	selection := casesDomain.ClientSelectionDraft{
		PositionKey:       p.Selection.PositionKey,
		Offer:             supplierOfferInputOf(*chosen),
		Quantity:          p.Selection.Quantity,
		FitmentStatus:     casesDomain.FitmentStatusProven,
		FitmentDigest:     p.Selection.FitmentDigestPin,
		ProfileDigest:     p.Selection.ProfileDigestPin,
		PartRequestDigest: p.Selection.PartRequestDigestPin,
		Now:               now,
	}
	material, err := casesDomain.AssembleClientSelection(policy, selection)
	if err != nil {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: err.Error()}
	}
	// The canonical digest re-computation under the locks: the stored
	// SelectionDigest is the SERVER truth of the snapshot material.
	if material.SelectionDigest != p.Selection.SelectionDigest {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "канонический дайджест выбора не совпадает с материалом — пересчитайте выбор"}
	}
	if material.KnownClientSum != p.Selection.KnownClientSum {
		return casesDomain.WorkRuleError{Rule: p.Command, Reason: "известная клиентская сумма не совпадает с арифметикой количества и записанной цены — пересчитайте выбор"}
	}
	snapshotNo, err := selectionNextNoTx(ctx, tx, tenantID, p.CaseID, p.WorkItemID)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		insert into cases.selection_snapshots (tenant_id, case_id, work_item_id, snapshot_id, snapshot_no,
			position_key, supplier_id, offer_no, offer_digest_pin, quantity, unit_client_price, currency, known_client_sum,
			profile_digest_pin, part_request_digest_pin, fitment_digest_pin, selection_digest,
			actor_id, correlation_id, created_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		tenantID, p.CaseID, p.WorkItemID, p.Selection.SnapshotID, snapshotNo,
		p.Selection.PositionKey, p.Selection.SupplierID, p.Selection.OfferNo, p.Selection.OfferDigestPin, p.Selection.Quantity, p.Selection.UnitClientPrice, p.Selection.Currency, p.Selection.KnownClientSum,
		p.Selection.ProfileDigestPin, p.Selection.PartRequestDigestPin, p.Selection.FitmentDigestPin, p.Selection.SelectionDigest,
		p.Actor, p.CorrelationID, now); err != nil {
		return mapOfferInsertError(err)
	}
	return nil
}

// supplierOfferInputOf maps ONE stored offer record onto the domain
// derivation input (the store material — the same fields, the same
// honest shapes).
func supplierOfferInputOf(record casesPort.SupplierOfferRecord) casesDomain.SupplierOfferInput {
	return casesDomain.SupplierOfferInput{
		OfferNo:              record.OfferNo,
		PositionKey:          record.PositionKey,
		SupplierID:           record.SupplierID,
		SupplierName:         record.SupplierName,
		Brand:                record.Brand,
		Article:              record.Article,
		ClientPrice:          record.ClientPrice,
		PurchasePrice:        record.PurchasePrice,
		Currency:             record.Currency,
		QuantityAvailable:    record.QuantityAvailable,
		TermDays:             record.TermDays,
		Conditions:           record.Conditions,
		SourceSystem:         record.SourceSystem,
		SourceRecordID:       record.SourceRecordID,
		SourceVersion:        record.SourceVersion,
		SourceObservedAt:     record.SourceObservedAt,
		ReceivedAt:           record.ReceivedAt,
		ProfileDigestPin:     record.ProfileDigestPin,
		PartRequestDigestPin: record.PartRequestDigestPin,
		FitmentDigestPin:     record.FitmentDigestPin,
	}
}

// nilFloat renders the optional float as the SQL NULL (the honest absent
// purchase price — never a zero).
func nilFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

// nilInt renders the optional integer as the SQL NULL (the honest absent
// term — never a zero).
func nilInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
