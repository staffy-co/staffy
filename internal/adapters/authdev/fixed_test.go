package authdev

import (
	"context"
	"errors"
	"testing"

	"github.com/staffy-co/staffy/internal/platform/id"
)

func TestFixedResolverIsDevelopmentOnly(t *testing.T) {
	principalID := id.MustParse("01994f64-6c00-7000-8000-000000000001")
	if _, err := New("production", principalID); !errors.Is(err, ErrDisabled) {
		t.Fatalf("production resolver error = %v", err)
	}
	resolver, err := New("development", principalID)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := resolver.Resolve(context.Background())
	if err != nil || !principal.CanCreateTenant {
		t.Fatalf("principal = %#v, error = %v", principal, err)
	}
}
