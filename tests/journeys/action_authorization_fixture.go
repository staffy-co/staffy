package main

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// The Story 3.5 fixture: the confirmation→authorization contour over the
// REAL app service (the ordered gateway of WithAuthorization runs the
// REAL domain derivations and the REAL gate ladder), the in-memory
// repositories of the fixture world (the "мир пути, не продуктовая
// персистентность" of the spec — the append-only consent ladder, the
// atomic supersede, the monotonic event numbers, the immutable operation
// rows, the cross-case reservation and the append-only ledger with the
// outbox queue fact over the memory rows) and the SAME AG-4 test adapters
// the preparation contour uses (the synthetic OwningSystemReader with its
// /fixture/preview-material toggle — production wires no reader at all:
// the authorize of a definition with required material facts honestly
// answers the fail-closed 403 there).
//
// The honest race semantics of the PG store are ported EXACTLY: the
// version re-derivation under the lock (the raced twin answers the honest
// stale refusal, never a merge), the UNIQUE (tenant, preview_id,
// preview_version_no) bijection backstop (a raced confirm twin onto one
// preview version answers the honest stale refusal), the consumed
// re-selection UNDER the lock with the allowance re-derivation over the
// definition's consumption rule word (the raced second authorization of
// one single_use consent refuses honestly WITHOUT consuming), and the
// frozen INSERT order of the ONE transaction — the consumed event, the
// reservation, the operation, the ledger authorized entry and the outbox
// operation.queued record; ANY refusal rolls back EVERYTHING (the mutex
// makes the chain atomic by construction — the checks run BEFORE the
// first mutation, the failure paths leave no partial state).

// authorizationMemoryRepository is the in-memory ConfirmationRepository
// AND OperationRepository of the fixture world (the two contracts of the
// port over one locked state — the advisory-lock serialization of the PG
// store is the single mutex here). The pinned preview version reads
// consult the SAME preview repository of the fixture world (the live
// action_previews rows of the journeys — the digest re-check compares the
// REAL built rows byte-exact).
type authorizationMemoryRepository struct {
	mu sync.Mutex

	previews *previewMemoryRepository

	confirmations map[string][]actionDomain.Confirmation       // tenant|confirmationID → versions asc
	events        map[string][]actionPort.ConfirmationEventRow // tenant|confirmationID|versionNo → events by EventNo
	caseEvents    map[string][]actionPort.ConfirmationEventRow // tenant|caseID → events by time
	caseConfIndex map[string][]id.UUID                         // tenant → the confirmation ids of the case reads

	operations map[string]actionDomain.Operation              // tenant|operationID
	byKey      map[string]actionDomain.Operation              // tenant|idempotencyKey
	byIntent   map[string]actionDomain.Operation              // tenant|owningSystem|businessIntentKey
	caseOps    map[string][]actionDomain.Operation            // tenant|caseID → operations by time
	ledger     map[string][]actionDomain.OperationLedgerEntry // tenant|operationID → entries by EntryNo
	outbox     map[string]bool                                // tenant|operationID → the queue fact

	// Story 3.6 (step-03): the queued-row material of the fixture drain —
	// the at-least-once delivery units (the duplicate rows of one
	// operation are LEGAL — the classification governs the repeat). The
	// dispatch memory repository reads these rows under the SAME mutex.
	outboxRows map[string][]authorizationOutboxRow // tenant|operationID → rows by time
}

// newAuthorizationMemoryRepository constructs the repositories of the
// fixture world over the live preview rows.
func newAuthorizationMemoryRepository(previews *previewMemoryRepository) *authorizationMemoryRepository {
	return &authorizationMemoryRepository{
		previews:      previews,
		confirmations: map[string][]actionDomain.Confirmation{},
		events:        map[string][]actionPort.ConfirmationEventRow{},
		caseEvents:    map[string][]actionPort.ConfirmationEventRow{},
		caseConfIndex: map[string][]id.UUID{},
		operations:    map[string]actionDomain.Operation{},
		byKey:         map[string]actionDomain.Operation{},
		byIntent:      map[string]actionDomain.Operation{},
		caseOps:       map[string][]actionDomain.Operation{},
		ledger:        map[string][]actionDomain.OperationLedgerEntry{},
		outbox:        map[string]bool{},
		outboxRows:    map[string][]authorizationOutboxRow{},
	}
}

// The compile-time proofs the fixture repositories satisfy BOTH consumer
// contracts of the contour (the narrow interfaces the app type-asserts).
var (
	_ actionPort.ConfirmationRepository = (*authorizationMemoryRepository)(nil)
	_ actionPort.OperationRepository    = (*authorizationMemoryRepository)(nil)
)

// authorizationConfirmationKey builds the consent identity key of the
// tenant scope.
func authorizationConfirmationKey(scope tenancy.Context, confirmationID id.UUID) string {
	return scope.TenantID().String() + "|" + confirmationID.String()
}

// authorizationEventKey builds the version-scope key of the consent
// history (the itoa probe of the cases fixture world).
func authorizationEventKey(scope tenancy.Context, confirmationID id.UUID, versionNo int64) string {
	return scope.TenantID().String() + "|" + confirmationID.String() + "|" + itoa(int(versionNo))
}

// appendConfirmationEventLocked appends ONE history event with the
// re-derived monotonic number of its version scope (the caller holds the
// lock — the PG insertConfirmationEventRow port).
func (r *authorizationMemoryRepository) appendConfirmationEventLocked(scope tenancy.Context,
	confirmationID id.UUID, versionNo int64, caseID, eventID id.UUID,
	kind, reason, details string, actorID id.UUID, now time.Time) {
	key := authorizationEventKey(scope, confirmationID, versionNo)
	var maxEvent int64
	if rows := r.events[key]; len(rows) > 0 {
		maxEvent = rows[len(rows)-1].EventNo
	}
	row := actionPort.ConfirmationEventRow{
		ConfirmationID: confirmationID, VersionNo: versionNo, CaseID: caseID,
		EventID: eventID, EventNo: maxEvent + 1, Kind: kind,
		Reason: reason, Details: details, ActorID: actorID, CreatedAt: now.UTC(),
	}
	r.events[key] = append(r.events[key], row)
	caseKey := scope.TenantID().String() + "|" + caseID.String()
	r.caseEvents[caseKey] = append(r.caseEvents[caseKey], row)
}

// maxConfirmationVersionLocked reads the version head of one consent
// identity (the caller holds the lock).
func (r *authorizationMemoryRepository) maxConfirmationVersionLocked(scope tenancy.Context, confirmationID id.UUID) int64 {
	versions := r.confirmations[authorizationConfirmationKey(scope, confirmationID)]
	if len(versions) == 0 {
		return 0
	}
	return versions[len(versions)-1].VersionNo
}

// ReadPreviewConfirmations reads every confirmation version of one
// preview thread ordered by the monotonic version.
func (r *authorizationMemoryRepository) ReadPreviewConfirmations(_ context.Context, scope tenancy.Context, previewID id.UUID) ([]actionDomain.Confirmation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionDomain.Confirmation{}
	for _, versions := range r.confirmations {
		for _, version := range versions {
			if version.TenantID == scope.TenantID() && version.PreviewID == previewID {
				out = append(out, version)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VersionNo < out[j].VersionNo })
	return out, nil
}

// ReadCaseConfirmations reads every confirmation version of one case
// ordered by the identity and the version.
func (r *authorizationMemoryRepository) ReadCaseConfirmations(_ context.Context, scope tenancy.Context, caseID id.UUID) ([]actionDomain.Confirmation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionDomain.Confirmation{}
	for _, versions := range r.confirmations {
		for _, version := range versions {
			if version.TenantID == scope.TenantID() && version.CaseID == caseID {
				out = append(out, version)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ConfirmationID != out[j].ConfirmationID {
			return out[i].ConfirmationID.String() < out[j].ConfirmationID.String()
		}
		return out[i].VersionNo < out[j].VersionNo
	})
	return out, nil
}

// ReadConfirmationVersion reads one exact confirmation version; the
// honest ErrNotFound answers a dead pin BEFORE any material.
func (r *authorizationMemoryRepository) ReadConfirmationVersion(_ context.Context, scope tenancy.Context, confirmationID id.UUID, versionNo int64) (actionDomain.Confirmation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, version := range r.confirmations[authorizationConfirmationKey(scope, confirmationID)] {
		if version.VersionNo == versionNo {
			return version, nil
		}
	}
	return actionDomain.Confirmation{}, actionDomain.ErrNotFound
}

// ReadConfirmationVersionEvents reads the events of one exact version
// ordered by the monotonic number (the state derivation input).
func (r *authorizationMemoryRepository) ReadConfirmationVersionEvents(_ context.Context, scope tenancy.Context, confirmationID id.UUID, versionNo int64) ([]actionDomain.ConfirmationEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionDomain.ConfirmationEvent{}
	for _, row := range r.events[authorizationEventKey(scope, confirmationID, versionNo)] {
		out = append(out, actionDomain.ConfirmationEvent{
			Kind: row.Kind, Reason: row.Reason, Details: row.Details,
			At: row.CreatedAt.UTC(), ActorID: row.ActorID,
		})
	}
	return out, nil
}

// ReadCaseConfirmationEvents reads every confirmation event of one case
// ordered by time (the view material with the visible exceptions).
func (r *authorizationMemoryRepository) ReadCaseConfirmationEvents(_ context.Context, scope tenancy.Context, caseID id.UUID) ([]actionPort.ConfirmationEventRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionPort.ConfirmationEventRow{}
	out = append(out, r.caseEvents[scope.TenantID().String()+"|"+caseID.String()]...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

// AppendConfirmationVersion appends one consent version under the lock —
// ONE atomic chain: the version row, the confirmed event of the new
// version and (when this is not the first version) the superseded event
// of the old version. The version head re-derives (the raced twin answers
// the honest stale refusal); the UNIQUE (tenant, preview_id,
// preview_version_no) bijection backstop holds structurally.
func (r *authorizationMemoryRepository) AppendConfirmationVersion(_ context.Context, p actionPort.AppendConfirmationParams) error {
	if !p.Scope.Valid() {
		return tenancy.ErrMissingContext
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	maxVersion := r.maxConfirmationVersionLocked(p.Scope, p.Confirmation.ConfirmationID)
	if maxVersion+1 != p.Confirmation.VersionNo {
		// The raced ladder: the twin appended first — the honest stale
		// refusal, never a merge.
		return actionPort.ErrStaleConfirmationVersion
	}
	if (maxVersion == 0) != (p.PrevVersionNo == 0) {
		// The app's thread resolve and the store's head disagree about the
		// first/existing shape — the honest stale refusal.
		return actionPort.ErrStaleConfirmationVersion
	}
	for _, versions := range r.confirmations {
		for _, version := range versions {
			if version.TenantID == p.Scope.TenantID() &&
				version.PreviewID == p.Confirmation.PreviewID &&
				version.PreviewVersionNo == p.Confirmation.PreviewVersionNo {
				// The structural bijection backstop of the raced confirm
				// twins (two fresh threads of one preview version, or two
				// re-confirms onto one preview version) — the honest
				// stale refusal, nothing merges.
				return actionPort.ErrStaleConfirmationVersion
			}
		}
	}
	r.confirmations[authorizationConfirmationKey(p.Scope, p.Confirmation.ConfirmationID)] =
		append(r.confirmations[authorizationConfirmationKey(p.Scope, p.Confirmation.ConfirmationID)], p.Confirmation)
	r.appendConfirmationEventLocked(p.Scope, p.Confirmation.ConfirmationID,
		p.Confirmation.VersionNo, p.Confirmation.CaseID, p.ConfirmedEventID,
		actionDomain.ConfirmationEventConfirmed, "", "", p.Confirmation.ConfirmedBy, p.Now)
	if p.PrevVersionNo > 0 {
		r.appendConfirmationEventLocked(p.Scope, p.Confirmation.ConfirmationID,
			p.PrevVersionNo, p.Confirmation.CaseID, p.SupersededEventID,
			actionDomain.ConfirmationEventSuperseded, "", "", p.Confirmation.ConfirmedBy, p.Now)
	}
	return nil
}

// AppendConfirmationEvent appends ONE history event of one exact version
// under the lock (the version must exist — the honest 404 of a dead pin
// before any material; the history is never rewritten).
func (r *authorizationMemoryRepository) AppendConfirmationEvent(_ context.Context, p actionPort.AppendConfirmationEventParams) error {
	if !p.Scope.Valid() {
		return tenancy.ErrMissingContext
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	found := false
	for _, version := range r.confirmations[authorizationConfirmationKey(p.Scope, p.ConfirmationID)] {
		if version.VersionNo == p.VersionNo {
			found = true
			break
		}
	}
	if !found {
		return actionDomain.ErrNotFound
	}
	r.appendConfirmationEventLocked(p.Scope, p.ConfirmationID, p.VersionNo,
		p.CaseID, p.EventID, p.Kind, p.Reason, p.Details, p.ActorID, p.Now)
	return nil
}

// ---------------------------------------------------------------------------
// The operation repository of the fixture world
// ---------------------------------------------------------------------------

// authorizationOperationKey builds the operation identity key of the
// tenant scope.
func authorizationOperationKey(scope tenancy.Context, operationID id.UUID) string {
	return scope.TenantID().String() + "|" + operationID.String()
}

// ReadPinnedPreviewVersion reads one exact LIVE preview version row of
// the fixture world (the same repository the preparation contour built
// its threads in — the digest re-check compares the REAL built row
// byte-exact); the honest ErrNotFound answers a dead pin BEFORE any
// material.
func (r *authorizationMemoryRepository) ReadPinnedPreviewVersion(_ context.Context, _ tenancy.Context, previewID id.UUID, versionNo int64) (actionDomain.ActionPreview, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, version := range r.previews.versions[previewID.String()] {
		if version.VersionNo == versionNo {
			return version, nil
		}
	}
	return actionDomain.ActionPreview{}, actionDomain.ErrNotFound
}

// FindOperationByIdempotencyKey reads the operation of one client key
// (the FIRST check of the gateway — the replay comparison).
func (r *authorizationMemoryRepository) FindOperationByIdempotencyKey(_ context.Context, scope tenancy.Context, idempotencyKey string) (actionDomain.Operation, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	operation, found := r.byKey[scope.TenantID().String()+"|"+idempotencyKey]
	return operation, found, nil
}

// FindOperationByIntent reads the operation reserved under one business
// intent (the loser reference of the cross-case race).
func (r *authorizationMemoryRepository) FindOperationByIntent(_ context.Context, scope tenancy.Context, owningSystem, businessIntentKey string) (actionDomain.Operation, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	operation, found := r.byIntent[scope.TenantID().String()+"|"+owningSystem+"|"+businessIntentKey]
	return operation, found, nil
}

// ReadCaseOperations reads every operation of one case ordered by time
// (the view material).
func (r *authorizationMemoryRepository) ReadCaseOperations(_ context.Context, scope tenancy.Context, caseID id.UUID) ([]actionDomain.Operation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionDomain.Operation{}
	out = append(out, r.caseOps[scope.TenantID().String()+"|"+caseID.String()]...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].OperationID.String() < out[j].OperationID.String()
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// ReadOperationLedger reads the append-only ledger of one operation
// ordered by the monotonic entry number (the state derivation input).
func (r *authorizationMemoryRepository) ReadOperationLedger(_ context.Context, scope tenancy.Context, operationID id.UUID) ([]actionDomain.OperationLedgerEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []actionDomain.OperationLedgerEntry{}
	out = append(out, r.ledger[authorizationOperationKey(scope, operationID)]...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].EntryNo < out[j].EntryNo })
	return out, nil
}

// ReadOutboxQueued reports the queue FACT of one operation (the presence
// of the operation.queued record — the spoken word derives from it,
// nothing is stored).
func (r *authorizationMemoryRepository) ReadOutboxQueued(_ context.Context, scope tenancy.Context, operationID id.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.outbox[authorizationOperationKey(scope, operationID)], nil
}

// ReserveOperation is the ONE transaction of the authorization (AC-5/AC-6):
// the consumed events of the pinned version re-select, the allowance
// re-derives over the definition's consumption rule word, then the frozen
// INSERT order — the consumed event, the reservation, the operation, the
// ledger authorized entry and the outbox operation.queued record. The
// signature carries ZERO adapters — persist-before-emit is STRUCTURAL;
// ANY refusal rolls back the WHOLE chain: the mutex serializes the chain
// (the advisory-lock serialization of the PG store), the checks run
// BEFORE the first mutation, the failure paths leave no partial state
// (the consent stays unconsumed, the reservation/operation/ledger/outbox
// never existed).
func (r *authorizationMemoryRepository) ReserveOperation(_ context.Context, p actionPort.ReserveOperationParams) error {
	if !p.Scope.Valid() {
		return tenancy.ErrMissingContext
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// The consumed re-selection under the lock: the count of the consumed
	// events of the PINNED version (read-time derivation discipline — the
	// count is never a stored column).
	var consumed int
	for _, row := range r.events[authorizationEventKey(p.Scope, p.Operation.ConfirmationID, p.Operation.ConfirmationVersionNo)] {
		if row.Kind == actionDomain.ConfirmationEventConsumed {
			consumed++
		}
	}
	allowance, err := actionDomain.DeriveConsumptionAllowance(p.ConsumptionRule)
	if err != nil {
		return err
	}
	if allowance.Bounded && consumed >= allowance.Limit {
		// The raced consumption: the honest refusal UNDER the lock — the
		// whole transaction rolls back, this attempt consumed NOTHING.
		return actionPort.ErrConfirmationExhausted
	}
	// CHECK 2/5 — the cross-case reservation (the loser of the UNIQUE is
	// the honest reference, never a masked replay; the reservation fires
	// BEFORE the operation twin — the frozen order of the PG inserts).
	intentKey := p.Scope.TenantID().String() + "|" + p.OutboxPayload.OwningSystem + "|" + p.Operation.BusinessIntentKey
	if _, exists := r.byIntent[intentKey]; exists {
		return actionPort.ErrReservationConflict
	}
	// CHECK 3/5 — the immutable operation row: the twin of one client key
	// is the replay signal, a minted identity collision is the loud
	// integrity alarm.
	idempotencyKey := p.Scope.TenantID().String() + "|" + p.Operation.IdempotencyKey
	if _, exists := r.byKey[idempotencyKey]; exists {
		return actionPort.ErrIdempotencyTwin
	}
	if _, exists := r.operations[authorizationOperationKey(p.Scope, p.Operation.OperationID)]; exists {
		return actionPort.ErrOperationIntegrity
	}
	// All checks passed — the frozen chain applies atomically under the
	// lock (INSERT 1/5 — the consumed event of the pinned version).
	r.appendConfirmationEventLocked(p.Scope, p.Operation.ConfirmationID,
		p.Operation.ConfirmationVersionNo, p.Operation.CaseID, p.ConsumedEventID,
		actionDomain.ConfirmationEventConsumed, "", "", p.ActorID, p.Now)
	// INSERT 2/5 — the cross-case reservation.
	r.byIntent[intentKey] = p.Operation
	// INSERT 3/5 — the immutable operation row.
	r.operations[authorizationOperationKey(p.Scope, p.Operation.OperationID)] = p.Operation
	r.byKey[idempotencyKey] = p.Operation
	caseKey := p.Scope.TenantID().String() + "|" + p.Operation.CaseID.String()
	r.caseOps[caseKey] = append(r.caseOps[caseKey], p.Operation)
	// INSERT 4/5 — the ledger authorized entry (AD-13): the entry
	// moment lands the SAME server truth the PG store stamps (the
	// fixture never serves a zero time — the card state moment of
	// 3-6 derives from the entries).
	authorizedEntry := p.LedgerEntry
	authorizedEntry.At = p.Now.UTC()
	r.ledger[authorizationOperationKey(p.Scope, p.Operation.OperationID)] = append(
		r.ledger[authorizationOperationKey(p.Scope, p.Operation.OperationID)], authorizedEntry)
	// INSERT 5/5 — the outbox operation.queued record (the SAFE payload —
	// the material NEVER enters; the fixture drain of 3-6 reads the SAME
	// rows through the dispatch memory repository).
	operationKey := authorizationOperationKey(p.Scope, p.Operation.OperationID)
	r.outbox[operationKey] = true
	payloadBytes, err := json.Marshal(p.OutboxPayload)
	if err != nil {
		return err
	}
	r.outboxRows[operationKey] = append(r.outboxRows[operationKey], authorizationOutboxRow{
		EventID: p.OutboxEventID, Payload: payloadBytes, CreatedAt: p.Now.UTC(),
	})
	return nil
}

// ---------------------------------------------------------------------------
// The Story 3.5 delegation of the actions fixture (the previewService
// precedent): the confirmation→authorization contour lives on the SAME
// Story 3-4 world service (WithAuthorization extends the one service —
// both surfaces answer through the REAL ordered gateway); every other
// contour answers the honest not-configured refusal (the vertical never
// fakes the contour).
// ---------------------------------------------------------------------------

// The compile-time proof the fixture serves the narrow authorization
// surface the transport type-asserts.
var _ actionAPI.AuthorizationService = (*actionsFixture)(nil)

// authorizationNotConfiguredText is the honest refusal of the contours
// that carry no authorization repositories (the 3-1 pair and the empty
// production dictionary — the same discipline the preparation surface
// speaks; the vertical never fakes the contour).
const authorizationNotConfiguredText = "Контур авторизации (подтверждение и Operation) не сконфигурирован: репозитории согласия и операции не подключены, команда честно отклоняется."

// authorizationService picks the confirmation→authorization contour
// under the lock: the Story 3-4/3-5 world when the toggle chose it (the
// Story 3.8 AG-4 composition while its flag is on — the SAME
// repositories, the nil reconciliation adapter; the composition root IS
// the world).
func (f *actionsFixture) authorizationService() actionAPI.AuthorizationService {
	if f.mode == "preview" {
		// The Story 4.6 order world takes the precedence while its toggle
		// is on (the previewService precedent — the confirm/authorize chain
		// reads the SAME fresh repositories of the order contour).
		if f.orderEnabled && f.order != nil {
			return f.order
		}
		if f.ag4Enabled && f.ag4 != nil {
			return f.ag4
		}
		if f.preview != nil {
			return f.preview
		}
	}
	return nil
}

// ConfirmPreviewVersion delegates to the active authorization contour;
// the contours without the repositories answer the honest refusal.
func (f *actionsFixture) ConfirmPreviewVersion(ctx context.Context, command actionAPI.ConfirmPreviewVersionCommand) (actionAPI.ConfirmationVersionOutcome, error) {
	f.mu.Lock()
	service := f.authorizationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.ConfirmationVersionOutcome{}, actionDomain.RuleError{Rule: "confirm_preview_version", Reason: authorizationNotConfiguredText}
	}
	return service.ConfirmPreviewVersion(ctx, command)
}

// RevokeConfirmation delegates to the active authorization contour.
func (f *actionsFixture) RevokeConfirmation(ctx context.Context, command actionAPI.RevokeConfirmationCommand) (actionAPI.ConfirmationRevocationOutcome, error) {
	f.mu.Lock()
	service := f.authorizationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.ConfirmationRevocationOutcome{}, actionDomain.RuleError{Rule: "revoke_confirmation", Reason: authorizationNotConfiguredText}
	}
	return service.RevokeConfirmation(ctx, command)
}

// AuthorizeOperation delegates to the active authorization contour (the
// REAL ordered gateway runs there — the replay check first, the ladder by
// name, the ONE transaction of the in-memory repositories).
func (f *actionsFixture) AuthorizeOperation(ctx context.Context, command actionAPI.AuthorizeOperationCommand) (actionAPI.AuthorizationOutcome, error) {
	f.mu.Lock()
	service := f.authorizationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.AuthorizationOutcome{}, actionDomain.RuleError{Rule: "authorize_operation", Reason: authorizationNotConfiguredText}
	}
	return service.AuthorizeOperation(ctx, command)
}

// ReadAuthorizationView delegates to the active authorization contour
// (the honest empty words of the domain).
func (f *actionsFixture) ReadAuthorizationView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) (actionAPI.AuthorizationView, error) {
	f.mu.Lock()
	service := f.authorizationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.AuthorizationView{}, actionDomain.RuleError{Rule: "read_authorization_view", Reason: authorizationNotConfiguredText}
	}
	return service.ReadAuthorizationView(ctx, actor, tenantID, caseID, journeyClock.Now())
}

// ---------------------------------------------------------------------------
// The Story 3.6 delegation of the actions fixture: the execution contour
// lives on the SAME world service (WithDispatch extends the one service —
// the ordered fences of the REAL app answer through the fixture path).
// ---------------------------------------------------------------------------

// The compile-time proof the fixture serves the narrow dispatch surface
// the transport type-asserts.
var _ actionAPI.DispatchService = (*actionsFixture)(nil)

// DrainDispatchQueue delegates to the active authorization contour (the
// REAL ordered fences run there — the write-ahead lease, the fenced
// verdict, the honest report of every considered job); the contours
// without the repositories answer the honest refusal.
func (f *actionsFixture) DrainDispatchQueue(ctx context.Context, command actionAPI.DrainDispatchCommand) (actionAPI.DispatchDrainResult, error) {
	f.mu.Lock()
	service := f.authorizationService()
	f.mu.Unlock()
	if service == nil {
		return actionAPI.DispatchDrainResult{}, actionDomain.RuleError{Rule: "drain_dispatch_queue", Reason: authorizationNotConfiguredText}
	}
	dispatch, ok := service.(actionAPI.DispatchService)
	if !ok {
		return actionAPI.DispatchDrainResult{}, actionDomain.RuleError{Rule: "drain_dispatch_queue", Reason: authorizationNotConfiguredText}
	}
	return dispatch.DrainDispatchQueue(ctx, actionAPI.DrainDispatchCommand{
		Actor: command.Actor, TenantID: command.TenantID, Now: journeyClock.Now(),
	})
}
