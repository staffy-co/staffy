package port

import (
	"context"
	"time"

	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// The caller-authority entitlement fact port is defined in the app layer
// (the memberFactsSource precedent): the readiness surface verifies the
// actor's scope first, then reads the derived admission state through the
// structural interface that the real entitlement service satisfies.

// GateInstanceParams appends one immutable gate instance lifecycle record.
// The instance id, the actor and the time are stamped by the store; the
// spec has passed the domain validation (state machine, PASS discipline,
// acyclicity).
type GateInstanceParams struct {
	Scope                                                   tenancy.Context
	EmployeeID, InstanceID, EventID, AuditID, CorrelationID id.UUID
	Spec                                                    domain.GateTransitionSpec
	Now                                                     time.Time
}

// AdmissionParams appends one immutable release admission decision. The
// derivation re-runs inside the store transaction over the in-transaction
// in-module facts (approved profile, sealed manifest, gate instances,
// admission history) together with the cross-module facts carried here
// (the entitlement admission fact and the current connection facts read
// immediately before under the same verified actor) — TOCTOU stays closed
// for everything the employee module owns.
type AdmissionParams struct {
	Scope                                       tenancy.Context
	EmployeeID, EventID, AuditID, CorrelationID id.UUID
	Stage                                       string
	Entitlement                                 domain.AdmissionFact
	CurrentConnections                          []domain.ConnectionFacts
	PackJourneys                                []string
	ApproverRoles                               []string
	Now                                         time.Time
}

// ReadinessRepository is the persistence port of the test & release
// surface. gate_instances and release_admissions are append-only
// (INSERT/SELECT): a failed, expired or superseded PASS and every recorded
// decision are immutable history.
type ReadinessRepository interface {
	// SaveGateInstance appends one lifecycle record; a raced transition of
	// the same predecessor surfaces as GateConflictError (the partial UNIQUE
	// index on supersedes keeps exactly one successor).
	SaveGateInstance(context.Context, GateInstanceParams) (domain.GateInstance, error)
	// ListGateInstances returns every lifecycle record of the employee, the
	// derivation picks the chain heads.
	ListGateInstances(context.Context, tenancy.Context, id.UUID) ([]domain.GateInstance, error)
	// SaveReleaseAdmission locks the employee aggregate, re-runs the
	// derivation over the in-transaction facts, enforces the stage ladder
	// (the exact candidate digests) and appends the next monotonic decision;
	// a raced or repeated publication of the same candidate and stage
	// surfaces as AdmissionConflictError.
	SaveReleaseAdmission(context.Context, AdmissionParams) (domain.ReleaseAdmission, error)
	// ListReleaseAdmissions returns every decision, newest first.
	ListReleaseAdmissions(context.Context, tenancy.Context, id.UUID) ([]domain.ReleaseAdmission, error)
}
