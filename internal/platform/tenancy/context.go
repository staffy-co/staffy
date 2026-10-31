package tenancy

import (
	"errors"

	"github.com/staffy-co/staffy/internal/platform/id"
)

var ErrMissingContext = errors.New("tenant context is required")

type Context struct {
	principalID id.UUID
	tenantID    id.UUID
}

func New(principalID, tenantID id.UUID) (Context, error) {
	if !principalID.Valid() || !tenantID.Valid() {
		return Context{}, ErrMissingContext
	}
	return Context{principalID: principalID, tenantID: tenantID}, nil
}

func (c Context) PrincipalID() id.UUID { return c.principalID }
func (c Context) TenantID() id.UUID    { return c.tenantID }
func (c Context) Valid() bool          { return c.principalID.Valid() && c.tenantID.Valid() }
