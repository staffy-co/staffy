package api

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	privacyDomain "github.com/artemiosu/staffy/internal/privacy/domain"
)

// Clock and IDGenerator follow the module conventions of identity/connector.
type Clock interface{ Now() time.Time }

type IDGenerator interface {
	New(time.Time) (id.UUID, error)
}

// MapEntryCommand publishes a new append-only version of the artifact data
// map record of one class. Unknown mandatory attributes are submitted empty
// and stored honestly as an incomplete entry.
type MapEntryCommand struct {
	Actor           domain.Principal
	TenantID        id.UUID
	Class           privacyDomain.ArtifactClass
	Purpose         string
	LegalBasis      string
	Controller      string
	Processor       string
	Recipients      []string
	SubProcessors   []string
	Locations       []string
	AccessibleRoles []string
	KnownCopies     []string
	PolicyRef       string
	DeletionMethod  string
	ExportMethod    string
	CorrelationID   id.UUID
}

// PolicyCommand publishes a new append-only retention policy version bound
// to exactly one artifact class.
type PolicyCommand struct {
	Actor             domain.Principal
	TenantID          id.UUID
	Class             privacyDomain.ArtifactClass
	Purpose           string
	LegalBasis        string
	Source            string
	TerminalEvent     string
	ActiveUseDays     int
	ArchiveDays       int
	DeletionDays      int
	BackupExpiryDays  int
	LegalHoldException string
	OwnerNote         string
	CorrelationID     id.UUID
}

// HoldCommand activates or releases a legal hold over a set of classes.
type HoldCommand struct {
	Actor          domain.Principal
	TenantID       id.UUID
	Classes        []privacyDomain.ArtifactClass
	Basis          string
	Approver       string
	ReviewDeadline time.Time
	CorrelationID  id.UUID
}

// DeletionCommand runs the idempotent data-governance deletion workflow over
// the requested class scope.
type DeletionCommand struct {
	Actor          domain.Principal
	TenantID       id.UUID
	Classes        []privacyDomain.ArtifactClass
	IdempotencyKey string
	CorrelationID  id.UUID
}

// DeletionResult is the receipt of one execution or replay.
type DeletionResult struct {
	Receipt  privacyDomain.Receipt
	Replayed bool
}

// ClassView is the derived per-class projection of the data map page.
type ClassView struct {
	Spec          privacyDomain.ClassSpec
	Entry         *privacyDomain.DataMapEntry
	Status        string // complete / incomplete / undocumented
	Missing       []string
	Policy        *privacyDomain.RetentionPolicy
	Hold          privacyDomain.HoldState
	ReferenceTime *time.Time
	DeletionDueAt *time.Time
	MandatoryDueAt *time.Time
}

// DataViewResult is the whole data map page payload.
type DataViewResult struct {
	Classes   []ClassView
	Receipts  []privacyDomain.Receipt
	HoldEvents []privacyDomain.LegalHoldEvent
}

// Service is the application surface consumed by the transport layer.
type Service interface {
	DataView(context.Context, domain.Principal, id.UUID) (DataViewResult, error)
	ReadReceipt(context.Context, domain.Principal, id.UUID, id.UUID) (DeletionResult, error)
	PublishMapEntry(context.Context, MapEntryCommand) (privacyDomain.DataMapEntry, error)
	PublishPolicy(context.Context, PolicyCommand) (privacyDomain.RetentionPolicy, error)
	ActivateHold(context.Context, HoldCommand) error
	ReleaseHold(context.Context, HoldCommand) error
	RunDeletion(context.Context, DeletionCommand) (DeletionResult, error)
}
