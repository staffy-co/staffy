package authdev

import (
	"context"
	"errors"

	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

var ErrDisabled = errors.New("development principal resolver is disabled")

type Fixed struct {
	principal domain.Principal
}

func New(environment string, principalID id.UUID) (*Fixed, error) {
	if environment != "development" || !principalID.Valid() {
		return nil, ErrDisabled
	}
	return &Fixed{principal: domain.Principal{ID: principalID, CanCreateTenant: true}}, nil
}

func (r *Fixed) Resolve(context.Context) (domain.Principal, error) { return r.principal, nil }
