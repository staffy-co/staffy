package correlation

import (
	"context"

	"github.com/staffy-co/staffy/internal/platform/id"
)

type contextKey struct{}

func WithID(ctx context.Context, correlationID id.UUID) context.Context {
	return context.WithValue(ctx, contextKey{}, correlationID)
}

func FromContext(ctx context.Context) (id.UUID, bool) {
	value, ok := ctx.Value(contextKey{}).(id.UUID)
	return value, ok && value.Valid()
}
