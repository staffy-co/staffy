package api

import (
        "time"

        "context"
        "github.com/artemiosu/staffy/internal/action/port"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 3.5: the confirmation→authorization surface (the DTO layer — the
// transport renders THESE shapes only; the Preview DTO precedent)
// ---------------------------------------------------------------------------

// ConfirmPreviewVersionCommand is the human consent request of one EXACT
// preview version: the consent is the human act with the LIVE step-up
// (the Approve precedent of 3-3), the requested lifetime clamps into the
// bounded server window (60..3600s — the promised_at precedent), and the
// pins come from the command — the app re-reads the LIVE preview row and
// copies the linked material (the consent carries WHAT was consented to).
type ConfirmPreviewVersionCommand struct {
        Actor            identityDomain.Principal
        TenantID         id.UUID
        CaseID           id.UUID
        PreviewID        id.UUID
        PreviewVersionNo int64
        LifetimeSeconds  int
        Now              time.Time
}

// ConfirmationVersionOutcome is the honest answer of one consent append:
// the confirmation identity with the new monotonic version, the derived
// state word with its fixed text, the server-truth valid_until and the
// consumption words of the fresh version.
type ConfirmationVersionOutcome struct {
        Command          string    `json:"command"`
        ConfirmationID   id.UUID   `json:"confirmation_id"`
        VersionNo        int64     `json:"version_no"`
        State            string    `json:"state"`
        StateText        string    `json:"state_text"`
        ValidUntil       time.Time `json:"valid_until"`
        ConsumptionText  string    `json:"consumption_text"`
        PreviewID        id.UUID   `json:"preview_id"`
        PreviewVersionNo int64     `json:"preview_version_no"`
}

// RevokeConfirmationCommand is the human revocation of one exact
// confirmation version: the revocation carries the LIVE step-up (the
// human act), kills the FUTURE authorizations of the consent and never
// un-consumes the past (the consumed events stay the audit fact).
type RevokeConfirmationCommand struct {
        Actor          identityDomain.Principal
        TenantID       id.UUID
        CaseID         id.UUID
        ConfirmationID id.UUID
        VersionNo      int64
        Now            time.Time
}

// ConfirmationRevocationOutcome is the honest answer of the revocation:
// the state word of the revoked version with the fixed text and the
// revocation note (the past consumption stays the audit fact).
type ConfirmationRevocationOutcome struct {
        Command        string  `json:"command"`
        ConfirmationID id.UUID `json:"confirmation_id"`
        VersionNo      int64   `json:"version_no"`
        State          string  `json:"state"`
        StateText      string  `json:"state_text"`
        RevocationNote string  `json:"revocation_note"`
}

// AuthorizeOperationCommand is the machine-readable authorization request
// (AG-4/AC-10: the AI Runtime does not exist — the input is pins and the
// machine idempotency word ONLY, the transport's strict decode refuses a
// free field; the model's confidence never substitutes a Confirmation).
// The double-click, the webhook repeat and the replay job carry the SAME
// idempotency key again — the replay comparison of the gateway honors
// it FIRST.
type AuthorizeOperationCommand struct {
        Actor                 identityDomain.Principal
        TenantID              id.UUID
        CaseID                id.UUID
        ConfirmationID        id.UUID
        ConfirmationVersionNo int64
        IdempotencyKey        string
        Now                   time.Time
}

// OperationLedgerEntryView is one append-only ledger record of the
// operation card (AD-13: the authorized entry of this slice; the honest
// history the card renders).
type OperationLedgerEntryView struct {
        EntryNo   int64     `json:"entry_no"`
        Kind      string    `json:"kind"`
        Details   string    `json:"details"`
        CreatedAt time.Time `json:"created_at"`
}

// OperationCardView is the full card of one IMMUTABLE operation: the
// pins (definition, preview, confirmation, case, work item, target, owner
// triple, release snapshot), the two key identities (the client
// idempotency reference and the server business intent), the derived
// life state word of the six-word dictionary (3-6 step-03: the queue is
// the structured FACT, the spoken pair-word of 3-5 matured into the life
// words), the append-only ledger, the dispatch view extension (the state
// moment, the Owning System, the next step, the visible block cause, the
// progressive attempts, the Action Receipt versions) and the fixed honest
// texts of the slice boundaries.
type OperationCardView struct {
        OperationID           id.UUID                    `json:"operation_id"`
        DefinitionID          string                     `json:"definition_id"`
        DefinitionVersion     string                     `json:"definition_version"`
        PreviewID             id.UUID                    `json:"preview_id"`
        PreviewVersionNo      int64                      `json:"preview_version_no"`
        ConfirmationID        id.UUID                    `json:"confirmation_id"`
        ConfirmationVersionNo int64                      `json:"confirmation_version_no"`
        CaseID                id.UUID                    `json:"case_id"`
        WorkItemID            id.UUID                    `json:"work_item_id"`
        TargetKind            string                     `json:"target_kind"`
        TargetRef             id.UUID                    `json:"target_ref"`
        OwnerKind             string                     `json:"owner_kind"`
        OwnerRef              id.UUID                    `json:"owner_ref"`
        OwnerEpoch            int64                      `json:"owner_epoch"`
        ReleaseDecisionNo     int64                      `json:"release_decision_no"`
        ProfileVersion        int64                      `json:"profile_version"`
        Partition             string                     `json:"partition"`
        IdempotencyKey        string                     `json:"idempotency_key"`
        BusinessIntentKey     string                     `json:"business_intent_key"`
        MaterialDigest        string                     `json:"material_digest"`
        State                 string                     `json:"state"`
        StateText             string                     `json:"state_text"`
        Queued                bool                       `json:"queued"`
        StateAt               time.Time                  `json:"state_at"`
        OwningSystem          string                     `json:"owning_system"`
        NextStepText          string                     `json:"next_step_text"`
        BlockReasonText       string                     `json:"block_reason_text"`
        ExternalIDAbsentText  string                     `json:"external_id_absent_text"`
        DispatchAbsentText    string                     `json:"dispatch_absent_text"`
        Ledger                []OperationLedgerEntryView `json:"ledger"`
        Attempts              []DispatchAttemptView      `json:"attempts"`
        Receipts              []ActionReceiptView        `json:"receipts"`
        CreatedAt             time.Time                  `json:"created_at"`
}

// ConfirmationEventView is one append-only consent history record of the
// confirmation card (the six-word kind; the conflict and the blocked
// words are the VISIBLE audit of the rejected authorization attempts).
type ConfirmationEventView struct {
        VersionNo int64     `json:"version_no"`
        EventNo   int64     `json:"event_no"`
        Kind      string    `json:"kind"`
        Reason    string    `json:"reason,omitempty"`
        Details   string    `json:"details,omitempty"`
        CreatedAt time.Time `json:"created_at"`
}

// ConfirmationCardView is the full card of one confirmation version: the
// pins, the derived state word with its fixed text, the server-truth
// valid_until, the consumption words («потреблено N из M» / unlimited)
// and the append-only event history.
type ConfirmationCardView struct {
        ConfirmationID    id.UUID                 `json:"confirmation_id"`
        VersionNo         int64                   `json:"version_no"`
        DefinitionID      string                  `json:"definition_id"`
        DefinitionVersion string                  `json:"definition_version"`
        CaseID            id.UUID                 `json:"case_id"`
        WorkItemID        id.UUID                 `json:"work_item_id"`
        TargetKind        string                  `json:"target_kind"`
        TargetRef         id.UUID                 `json:"target_ref"`
        OwnerKind         string                  `json:"owner_kind"`
        OwnerRef          id.UUID                 `json:"owner_ref"`
        PreviewID         id.UUID                 `json:"preview_id"`
        PreviewVersionNo  int64                   `json:"preview_version_no"`
        State             string                  `json:"state"`
        StateText         string                  `json:"state_text"`
        ValidUntil        time.Time               `json:"valid_until"`
        ConsumptionText   string                  `json:"consumption_text"`
        Events            []ConfirmationEventView `json:"events"`
        CreatedAt         time.Time               `json:"created_at"`
}

// AuthorizationOutcome is the honest answer of the authorize command:
// Replay=true speaks the idempotent return of the ORIGINAL operation
// (AC-7 — the consumption never repeats, the second outbox record never
// exists); ReservationReference=true speaks the cross-case loser return
// (AC-9 — the reference to the tenant-visible existing operation, the
// loser's consumption never happened); OutcomeText carries the fixed
// honest text of the return path; the operation card completes the
// answer.
type AuthorizationOutcome struct {
        Command              string            `json:"command"`
        Replay               bool              `json:"replay"`
        ReservationReference bool              `json:"reservation_reference"`
        OutcomeText          string            `json:"outcome_text"`
        Operation            OperationCardView `json:"operation"`
}

// AuthorizationView is the page/API projection of the Operation Review
// surface of one case: every confirmation version card with the live
// read-time derivations, every operation card with the queue fact, and
// the honest empty-state words (the transport renders the fixed domain
// texts — a private copy would drift).
type AuthorizationView struct {
        TenantID      id.UUID                `json:"tenant_id"`
        CaseID        id.UUID                `json:"case_id"`
        EmptyText     string                 `json:"empty_text,omitempty"`
        Confirmations []ConfirmationCardView `json:"confirmations"`
        Operations    []OperationCardView    `json:"operations"`
}

// AuthorizationService is the narrow confirmation→authorization surface
// the transport type-asserts (the PreviewService precedent): the
// composite app Service implements it, the fixture world binds it.
type AuthorizationService interface {
        // ConfirmPreviewVersion runs the consent chain of one EXACT preview
        // version in the frozen order: the rights gate (review|configuration)
        // → the LIVE step-up (the human act) → the LIVE preview row (the
        // honest 404 of a dead pin) → the definition resolve with the
        // required/allowance guards → the freshness gate (NO reasons — the
        // stale consent of dead material is meaningless) → the readiness gate
        // (ready only) → the append version with the linked material copies
        // (ONE transaction, the atomic supersede).
        ConfirmPreviewVersion(ctx context.Context, command ConfirmPreviewVersionCommand) (ConfirmationVersionOutcome, error)

        // RevokeConfirmation appends the human revocation of one exact
        // confirmation version (the LIVE step-up; the past consumption stays
        // the audit fact — the revocation kills the FUTURE authorizations
        // only).
        RevokeConfirmation(ctx context.Context, command RevokeConfirmationCommand) (ConfirmationRevocationOutcome, error)

        // AuthorizeOperation runs the FULL ordered gateway (the replay check
        // FIRST, the rejection ladder by name, the non-consuming refusals
        // BEFORE the ONE transaction): the double-click, the webhook repeat
        // and the replay job carry the SAME client key — the replay returns
        // the original operation; a different material under the same key is
        // the honest 409 conflict with the visible exception; the cross-case
        // loser of the reservation reads the honest reference; the Connector
        // call does not exist in this slice (AC-12 counter).
        AuthorizeOperation(ctx context.Context, command AuthorizeOperationCommand) (AuthorizationOutcome, error)

        // ReadAuthorizationView renders the case's Operation Review surface:
        // the confirmation cards with the live state/consumption derivations,
        // the operation cards with the ledger and the queue fact (executed
        // NEVER exists — the honest words of the slice boundaries). The now
        // parameter is the TRANSPORT clock — the same pair of hours the
        // confirm/authorize commands pin: the read-time expired derivation
        // stays coherent with the consent window instead of drifting over
        // the system wall (the production transport passes time.Now(), the
        // fixture worlds pass their single clock — the injected hours make
        // the derivation testable).
        ReadAuthorizationView(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) (AuthorizationView, error)
}

// The store-contract errors of the contour the transport maps (the
// consumer-owned vocabulary of the repository boundary re-exported for
// the render layer; the Preview precedent).
var (
        ErrStaleConfirmationVersion = port.ErrStaleConfirmationVersion
        ErrConfirmationExhausted    = port.ErrConfirmationExhausted
        ErrIdempotencyConflict      = port.ErrIdempotencyConflict
        ErrMaterialConditionChanged = port.ErrMaterialConditionChanged
        ErrFreshReadsUnavailable    = port.ErrFreshReadsUnavailable
        ErrPreviewDigestChanged     = port.ErrPreviewDigestChanged
        ErrPreviewBlockedMaterial   = port.ErrPreviewBlockedMaterial
        ErrOperationIntegrity       = port.ErrOperationIntegrity
)
