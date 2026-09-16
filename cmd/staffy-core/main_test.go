package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"
)

type verifierRow struct {
	role      string
	superuser bool
	bypassRLS bool
	err       error
}

func (r verifierRow) Scan(destinations ...any) error {
	if r.err != nil {
		return r.err
	}
	*destinations[0].(*string) = r.role
	*destinations[1].(*bool) = r.superuser
	*destinations[2].(*bool) = r.bypassRLS
	return nil
}

type verifierDatabase struct{ row pgx.Row }

func (d verifierDatabase) QueryRow(context.Context, string, ...any) pgx.Row { return d.row }

func TestVerifyRuntimeDatabaseRejectsRLSBypass(t *testing.T) {
	for name, row := range map[string]verifierRow{
		"superuser": {role: "postgres", superuser: true},
		"bypassrls": {role: "unsafe", bypassRLS: true},
	} {
		t.Run(name, func(t *testing.T) {
			if err := verifyRuntimeDatabase(context.Background(), verifierDatabase{row: row}); err == nil {
				t.Fatal("unsafe role accepted")
			}
		})
	}
	if err := verifyRuntimeDatabase(context.Background(), verifierDatabase{row: verifierRow{role: "staffy_app"}}); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsDevelopmentResolverOutsideLoopbackBeforeDatabaseAccess(t *testing.T) {
	t.Setenv("STAFFY_ENV", "development")
	t.Setenv("STAFFY_LISTEN_ADDR", "0.0.0.0:8080")
	t.Setenv("STAFFY_DEV_PRINCIPAL_ID", "01994f64-6c00-7000-8000-000000000001")
	t.Setenv("STAFFY_DATABASE_URL", "postgres://must-not-connect")
	err := run(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil || err.Error() != "development PrincipalResolver requires a loopback listen address" {
		t.Fatalf("error = %v", err)
	}
}

func TestLoopbackAddressVariants(t *testing.T) {
	for _, address := range []string{"127.0.0.1:8080", "[::1]:8080", "localhost:8080"} {
		if !loopbackAddress(address) {
			t.Errorf("%s rejected", address)
		}
	}
	for _, address := range []string{"0.0.0.0:8080", "[::]:8080", "example.com:8080", "invalid"} {
		if loopbackAddress(address) {
			t.Errorf("%s accepted", address)
		}
	}
}

func TestVerifyRuntimeDatabaseReportsConnectionFailure(t *testing.T) {
	want := errors.New("database unavailable")
	err := verifyRuntimeDatabase(context.Background(), verifierDatabase{row: verifierRow{err: want}})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}
