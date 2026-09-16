package domain

import (
	"errors"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

var (
	ErrForbidden           = errors.New("principal is not allowed to create a tenant")
	ErrInvalidName         = errors.New("tenant name is invalid")
	ErrInvalidIdempotency  = errors.New("idempotency key is invalid")
	ErrIdempotencyConflict = errors.New("idempotency key conflicts with another request")
	ErrNotFound            = errors.New("resource not found")
)

type Principal struct {
	ID              id.UUID
	CanCreateTenant bool
}

type Tenant struct {
	ID        id.UUID
	Name      string
	CreatedAt time.Time
}

type Workspace struct {
	Tenant Tenant
	Role   string
}
