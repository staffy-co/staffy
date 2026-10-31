package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.10 (step-02): the order servicing request persistence — the
// case feed read, the lookup BY KEY over the recorded material of the
// tenant, the resolved-operation read and the ONE locked idempotent
// append
// ---------------------------------------------------------------------------

// The structural pin: the store satisfies the consumer-owned repository
// contract of the servicing contour (the OrderSupplierFactRepository
// precedent).
var _ actionPort.OrderServicingRepository = (*Postgres)(nil)

// orderServicingLockKey is the advisory lock key of one case's request
// ladder — its OWN key space (the "action.order_servicing:" prefix
// never mixes with the fact/correction/verification ones): the
// request_no ladder and the idempotency BY KEY serialize per case, the
// raced replay of the same request command reads the saved row under
// the lock and answers the honest duplicate signal — never a second
// row, never a masked write.
func orderServicingLockKey(tenantID, caseID id.UUID) string {
	return "action.order_servicing.v1:" + tenantID.String() + ":" + caseID.String()
}

// mapOrderServicingInsertError maps the raced insert conflicts onto the
// honest consumer-owned signals: the request PK is the per-case ladder
// backstop, the UNIQUE full request key is the IDEMPOTENCY backstop
// (the replay of the same command at the same trust state — the
// ladder-of-trust discipline of the slice). The store never masks a
// lost race as a write.
func mapOrderServicingInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		return actionPort.ErrOrderServicingRequestDuplicate
	}
	return err
}

// orderServicingRequestColumns is the canonical column list of the
// request row (the SELECT and the INSERT share it — the scan target
// list mirrors it).
const orderServicingRequestColumns = `tenant_id, case_id, conversation_id, request_no, intent,
 identifier_kind, identifier_value, resolved_operation_id, lookup_outcome,
 proof_state, proof_method, proof_issued_at, proof_expires_at, disclosure,
 actor_id, recorded_at`

// scanOrderServicingRequest scans ONE request row (the shared scan
// target of the feed read).
func scanOrderServicingRequest(scan func(dest ...any) error) (actionDomain.OrderServicingRequest, error) {
	var request actionDomain.OrderServicingRequest
	var conversationID *id.UUID
	var resolvedOperationID *id.UUID
	var proofIssuedAt, proofExpiresAt *time.Time
	if err := scan(&request.TenantID, &request.CaseID, &conversationID, &request.RequestNo,
		&request.Intent, &request.IdentifierKind, &request.IdentifierValue, &resolvedOperationID,
		&request.LookupOutcome, &request.ProofState, &request.ProofMethod, &proofIssuedAt,
		&proofExpiresAt, &request.Disclosure, &request.ActorID, &request.RecordedAt); err != nil {
		return actionDomain.OrderServicingRequest{}, err
	}
	if conversationID != nil {
		request.ConversationID = *conversationID
	}
	if resolvedOperationID != nil {
		request.ResolvedOperationID = *resolvedOperationID
	}
	if proofIssuedAt != nil {
		request.ProofIssuedAt = proofIssuedAt.UTC()
	}
	if proofExpiresAt != nil {
		request.ProofExpiresAt = proofExpiresAt.UTC()
	}
	request.RecordedAt = request.RecordedAt.UTC()
	return request, nil
}

// ReadCaseServicingRequests reads every request of one case ordered by
// the monotonic request number (the panel feed — the recorded history
// never rewrites).
func (s *Postgres) ReadCaseServicingRequests(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]actionDomain.OrderServicingRequest, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		select `+orderServicingRequestColumns+`
		from action.order_servicing_requests
		where tenant_id=$1 and case_id=$2 order by request_no`,
		scope.TenantID(), caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	requests := []actionDomain.OrderServicingRequest{}
	for rows.Next() {
		request, err := scanOrderServicingRequest(rows.Scan)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return requests, tx.Commit(ctx)
}

// LookupOrderByIDentifier finds the order operations of the TENANT whose
// RECORDED material carries the named external ID (the lookup BY KEY —
// NO enumeration of the orders exists: the caller derives the outcome
// over the candidates). The ABCP kind reads the committed receipts of
// the order operations (the system receipts of the dispatch machine
// 4-6) UNION the rereads of the corrections (the manager records of
// 4-8) — the sources ride the candidates BY NAME; the supplier kind
// reads the human facts of 4-9. The query crosses the CASES of the
// tenant (the identifier may live in another case's material — the
// ambiguous outcome of two DIFFERENT operations; the SAME operation
// matched by several sources stays ONE candidate with several source
// pins). NO adapter of the external systems is consulted (AG-4/OQ-18:
// the recorded material of the contour only — this function is the
// structural proof).
func (s *Postgres) LookupOrderByIDentifier(ctx context.Context, scope tenancy.Context, kind, value string) ([]actionDomain.OrderLookupCandidate, error) {
	if !scope.Valid() {
		return nil, tenancy.ErrMissingContext
	}
	if !actionDomain.LookupIdentifierKindInDictionary(kind) {
		// The phone/email kinds do NOT exist — the honest empty answer
		// (never a query with a foreign kind; the app guard refuses the
		// command BEFORE the lookup anyway).
		return []actionDomain.OrderLookupCandidate{}, nil
	}
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var query string
	switch kind {
	case actionDomain.LookupKindABCPClientOrderID:
		// The committed receipts of the ORDER operations ∪ the rereads of
		// the corrections (both sources of the ABCP external ID — the
		// DIVergent IDs of one operation still resolve to ONE operation).
		query = `
			select o.operation_id, o.case_id, 'system_receipt' as source
			from action.action_receipts r
			join action.operations o on o.tenant_id = r.tenant_id and o.operation_id = r.operation_id
			where r.tenant_id=$1 and o.definition_id='create_abcp_customer_order'
				and r.outcome='committed' and r.receipt_references->>'external_object_id'=$2
			union
			select c.operation_id, c.case_id, 'manager_record' as source
			from action.order_correction_events e
			join action.order_corrections c on c.tenant_id = e.tenant_id and c.case_id = e.case_id and c.correction_no = e.correction_no
			where e.tenant_id=$1 and e.kind='reread' and e.external_object_id=$2`
	default:
		// The supplier kind: the human facts of 4-9 (the append-only
		// {formed, sent} records with the supplier order IDs).
		query = `
			select f.operation_id, o.case_id, 'manager_record' as source
			from action.order_supplier_facts f
			join action.operations o on o.tenant_id = f.tenant_id and o.operation_id = f.operation_id
			where f.tenant_id=$1 and f.supplier_order_id=$2`
	}
	rows, err := tx.Query(ctx, query, scope.TenantID(), value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// The candidate grouping: the same operation matched by SEVERAL
	// sources stays ONE candidate with the source pins BY NAME (the
	// distinct operations are the rows of the outcome derivation).
	byOperation := map[id.UUID]*actionDomain.OrderLookupCandidate{}
	order := []id.UUID{}
	for rows.Next() {
		var operationID, caseID id.UUID
		var source string
		if err := rows.Scan(&operationID, &caseID, &source); err != nil {
			return nil, err
		}
		if candidate, ok := byOperation[operationID]; ok {
			if !actionDomain.WordIn(candidate.Sources, source) {
				candidate.Sources = append(candidate.Sources, source)
			}
			continue
		}
		candidate := &actionDomain.OrderLookupCandidate{OperationID: operationID, CaseID: caseID, Sources: []string{source}}
		byOperation[operationID] = candidate
		order = append(order, operationID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	candidates := make([]actionDomain.OrderLookupCandidate, 0, len(order))
	for _, operationID := range order {
		candidates = append(candidates, *byOperation[operationID])
	}
	return candidates, tx.Commit(ctx)
}

// ReadResolvedOperation reads the operation row of the resolved anchor
// (the LIVE material assembly of the disclosure view — the projection
// of the FOUND operation re-derives read-time). The honest miss (false)
// never fails the view: the client words of a lost anchor answer the
// intent word only.
func (s *Postgres) ReadResolvedOperation(ctx context.Context, scope tenancy.Context, operationID id.UUID) (actionDomain.Operation, bool, error) {
	if !scope.Valid() {
		return actionDomain.Operation{}, false, tenancy.ErrMissingContext
	}
	tx, err := s.begin(ctx, scope, true)
	if err != nil {
		return actionDomain.Operation{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		select `+operationSelectColumns+`
		from action.operations
		where tenant_id=$1 and operation_id=$2`, scope.TenantID(), operationID)
	if err != nil {
		return actionDomain.Operation{}, false, err
	}
	ops, err := scanOperationRows(ctx, rows)
	if err != nil {
		return actionDomain.Operation{}, false, err
	}
	if len(ops) == 0 {
		return actionDomain.Operation{}, false, tx.Commit(ctx)
	}
	return ops[0], true, tx.Commit(ctx)
}

// AppendOrderServicingRequest appends ONE request under the per-case
// advisory lock: the monotonic request_no derives under the lock (the
// ladder of the repeated records — the history is never rewritten),
// the UNIQUE full request key (tenant, case, actor, intent,
// identifier_kind, identifier_value, proof_state) backstops the
// IDEMPOTENCY BY KEY — the raced replay of the same command at the
// same trust state refuses with ErrOrderServicingRequestDuplicate and
// NEVER writes a second row (the app re-reads the saved request and
// answers the honest replay outcome; the CHANGED trust state is a
// different key — a new row lands, the trust ladder stays visible).
// The recorded_at is the SERVER time of the append; the proof pins are
// the LIVE copy the command carried (they never re-derive here — the
// scope, the level and the time of the check are preserved, AC-4). The
// append is a pure persistence boundary: nothing is emitted anywhere
// (the OQ-3 fence — the request records the ask, it never performs
// it), the order operation is NEVER created or changed (FR-62 — the
// resolved_operation_id references the EXISTING operation only).
func (s *Postgres) AppendOrderServicingRequest(ctx context.Context, p actionPort.AppendOrderServicingRequestParams) (actionDomain.OrderServicingRequest, error) {
	tx, err := s.begin(ctx, p.Scope, false)
	if err != nil {
		return actionDomain.OrderServicingRequest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tenantID := p.Scope.TenantID()
	if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))",
		orderServicingLockKey(tenantID, p.CaseID)); err != nil {
		return actionDomain.OrderServicingRequest{}, err
	}
	// The per-case ladder number UNDER the lock (the store truth — the
	// request never carries it).
	var requestNo int64
	if err := tx.QueryRow(ctx, `
		select coalesce(max(request_no), 0) + 1 from action.order_servicing_requests
		where tenant_id=$1 and case_id=$2`,
		tenantID, p.CaseID).Scan(&requestNo); err != nil {
		return actionDomain.OrderServicingRequest{}, err
	}
	var conversationID any
	if p.ConversationID != (id.UUID("")) {
		conversationID = p.ConversationID
	}
	var resolvedOperationID any
	if p.ResolvedOperationID != (id.UUID("")) {
		resolvedOperationID = p.ResolvedOperationID
	}
	var proofIssuedAt, proofExpiresAt any
	if !p.ProofIssuedAt.IsZero() {
		proofIssuedAt = p.ProofIssuedAt.UTC().Truncate(time.Microsecond)
	}
	if !p.ProofExpiresAt.IsZero() {
		proofExpiresAt = p.ProofExpiresAt.UTC().Truncate(time.Microsecond)
	}
	recordedAt := p.Now.UTC().Truncate(time.Microsecond)
	request := actionDomain.OrderServicingRequest{
		TenantID:            tenantID,
		CaseID:              p.CaseID,
		ConversationID:      p.ConversationID,
		RequestNo:           requestNo,
		Intent:              p.Intent,
		IdentifierKind:      p.IdentifierKind,
		IdentifierValue:     p.IdentifierValue,
		ResolvedOperationID: p.ResolvedOperationID,
		LookupOutcome:       p.LookupOutcome,
		ProofState:          p.ProofState,
		ProofMethod:         p.ProofMethod,
		ProofIssuedAt:       p.ProofIssuedAt,
		ProofExpiresAt:      p.ProofExpiresAt,
		Disclosure:          p.Disclosure,
		ActorID:             p.ActorID,
		RecordedAt:          recordedAt,
	}
	if _, err := tx.Exec(ctx, `
		insert into action.order_servicing_requests (tenant_id, case_id, conversation_id, request_no,
			intent, identifier_kind, identifier_value, resolved_operation_id, lookup_outcome,
			proof_state, proof_method, proof_issued_at, proof_expires_at, disclosure,
			actor_id, recorded_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		request.TenantID, request.CaseID, conversationID, request.RequestNo,
		request.Intent, request.IdentifierKind, request.IdentifierValue, resolvedOperationID,
		request.LookupOutcome, request.ProofState, request.ProofMethod, proofIssuedAt,
		proofExpiresAt, request.Disclosure, request.ActorID, request.RecordedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return actionDomain.OrderServicingRequest{}, err
		}
		return actionDomain.OrderServicingRequest{}, mapOrderServicingInsertError(err)
	}
	return request, tx.Commit(ctx)
}
