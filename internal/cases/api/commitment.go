package api

import (
	"context"
	"time"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.9 command surface: the versioned commitment, the exact-version
// confirmation and the honest decline
// ---------------------------------------------------------------------------
//
// The three commands are idempotent by the tenant-scoped key with the stored
// outcome (the 2.2..2.8 precedent: the replay answer comes before the state
// guards); the request hash covers the whole command material. The gate
// order is frozen: the rights gate (review OR configuration — the M1
// precedent) fires BEFORE every material guard; the dictionary guards refuse
// with the honest 400; the confirm command demands the EXACT shown version
// (the presentation link), the verbatim span excerpts of the client consent,
// the freshness of the pinned price observation and the assurance ladder of
// 2.3 (the verified identity and — when the price is positive — the acting
// authority record); the decline mutates nothing.

// CreateCommitmentCommand creates one version of a commitment (the operator
// review material: the typed terms, the action set, the validity window, the
// consumption limit and the honest unknown holes). The first version of one
// commitment id starts at 1; a repeated create with the same commitment id
// appends version_no+1 (the changed offer IS a new version — AC-8).
type CreateCommitmentCommand struct {
	Actor            domain.Principal
	CorrelationID    id.UUID
	TenantID         id.UUID
	CaseID           id.UUID
	WorkItemID       id.UUID
	CommitmentID     id.UUID
	Status           string
	Terms            casesDomain.MaterialTerms
	ActionSet        []string
	ValidUntil       time.Time
	ConsumptionLimit int
	MaterialUnknowns []string
	IdempotencyKey   string
	Now              time.Time
}

// ConfirmCommitmentCommand records the exact-version confirmation of the
// SHOWN material: the verbatim span excerpts of the client consent words of
// the accepted text turn, the confirmed subject (the kind+ref participant)
// and the Work Item version expectation (the CAS of the transit — the 2.2
// discipline).
type ConfirmCommitmentCommand struct {
	Actor           domain.Principal
	CorrelationID   id.UUID
	TenantID        id.UUID
	CaseID          id.UUID
	WorkItemID      id.UUID
	CommitmentID    id.UUID
	VersionNo       int64
	ExpectedVersion int64
	SubjectKind     string
	SubjectRef      string
	TurnRef         id.UUID
	Spans           []string
	IdempotencyKey  string
	Now             time.Time
}

// RecordCommitmentDeclineCommand records the honest refusal or the change
// request of the shown material: the outcome word of the dictionary and the
// verbatim span excerpts of the client words. The decline mutates nothing
// (AC-11); the change request is answered by the operator with a new version.
type RecordCommitmentDeclineCommand struct {
	Actor          domain.Principal
	CorrelationID  id.UUID
	TenantID       id.UUID
	CaseID         id.UUID
	WorkItemID     id.UUID
	CommitmentID   id.UUID
	VersionNo      int64
	Outcome        string
	TurnRef        id.UUID
	Spans          []string
	IdempotencyKey string
	Now            time.Time
}

// CommitmentVersionCard is one version row of the view: the status word, the
// material conditions (the review scope — the case content), the action set,
// the validity window, the consumption limit, the honest unknown holes, the
// relation pins, the content binding presence and the read-time readiness
// derivation (the CURRENT version of the commitment is the maximal number).
type CommitmentVersionCard struct {
	Version   casesDomain.CommitmentVersion `json:"version"`
	Readiness string                        `json:"readiness"`
	Presented bool                          `json:"presented"`
	Current   bool                          `json:"current"`
}

// ConfirmationCard is one confirmation row of the view with the read-time
// validity derivation (expired/superseded/owner_stale/release_stale — the
// states are never stored).
type ConfirmationCard struct {
	Record   casesDomain.ConfirmationRecord `json:"record"`
	Validity string                         `json:"validity"`
}

// CommitmentWorkItemView is one Work Item section of the commitment view:
// the version history with the readiness derivations, the confirmations with
// the validity derivations, the declines and the presentation links.
type CommitmentWorkItemView struct {
	WorkItemID    id.UUID                           `json:"work_item_id"`
	Versions      []CommitmentVersionCard           `json:"versions"`
	Confirmations []ConfirmationCard                `json:"confirmations"`
	Declines      []casesPort.CommitmentDeclineView `json:"declines"`
	Presented     []casesDomain.PresentedCommitment `json:"presented"`
}

// CommitmentView is the commitment section of the case view: the per-Work-
// Item sections with the dictionary words of the forms and the panel text.
type CommitmentView struct {
	Items          []CommitmentWorkItemView `json:"items"`
	StatusWords    []string                 `json:"status_words"`
	ActionWords    []string                 `json:"action_words"`
	DeclineWords   []string                 `json:"decline_words"`
	ValidityWords  []string                 `json:"validity_words"`
	ReadinessWords []string                 `json:"readiness_words"`
	PanelText      string                   `json:"panel_text"`
}

// CommitmentService is the command surface of Story 2.9 (the three commands
// are idempotent by the tenant-scoped key; the commitment rows are
// append-only with the per-commitment monotonic version; the readiness and
// the validity are the read-time derivations — never stored).
type CommitmentService interface {
	CreateCommitment(context.Context, CreateCommitmentCommand) (WorkOutcome, error)
	ConfirmCommitment(context.Context, ConfirmCommitmentCommand) (WorkOutcome, error)
	RecordCommitmentDecline(context.Context, RecordCommitmentDeclineCommand) (WorkOutcome, error)
}
