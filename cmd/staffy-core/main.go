package main

import (
        "context"
        "errors"
        "fmt"
        "log/slog"
        "net"
        "net/http"
        "os"
        "os/signal"
        "syscall"
        "time"

        "github.com/artemiosu/staffy/internal/adapters/authdev"
        connectorApp "github.com/artemiosu/staffy/internal/connector/app"
        connectorStore "github.com/artemiosu/staffy/internal/connector/store"
        connectorVerifier "github.com/artemiosu/staffy/internal/connector/verifier"
        entitlementApp "github.com/artemiosu/staffy/internal/entitlement/app"
        entitlementStore "github.com/artemiosu/staffy/internal/entitlement/store"
        employeeApp "github.com/artemiosu/staffy/internal/employee/app"
        employeePort "github.com/artemiosu/staffy/internal/employee/port"
        employeeStore "github.com/artemiosu/staffy/internal/employee/store"
        "github.com/artemiosu/staffy/internal/identity/app"
        "github.com/artemiosu/staffy/internal/identity/store"
        privacyApp "github.com/artemiosu/staffy/internal/privacy/app"
        privacyDomain "github.com/artemiosu/staffy/internal/privacy/domain"
        privacyPort "github.com/artemiosu/staffy/internal/privacy/port"
        privacyStore "github.com/artemiosu/staffy/internal/privacy/store"
        "github.com/artemiosu/staffy/internal/platform/id"
        httptransport "github.com/artemiosu/staffy/internal/transport/http"
        "github.com/jackc/pgx/v5"
        "github.com/jackc/pgx/v5/pgxpool"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type uuidGenerator struct{}

func (uuidGenerator) New(now time.Time) (id.UUID, error) { return id.NewV7(now) }

func main() {
        logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
        if err := run(logger); err != nil {
                logger.Error("staffy-core stopped", "error", err)
                os.Exit(1)
        }
}

func run(logger *slog.Logger) error {
        environment := os.Getenv("STAFFY_ENV")
        listenAddress := envOr("STAFFY_LISTEN_ADDR", "127.0.0.1:8080")
        if environment != "development" {
                return errors.New("no production PrincipalResolver is configured")
        }
        if !loopbackAddress(listenAddress) {
                return errors.New("development PrincipalResolver requires a loopback listen address")
        }
        principalID, err := id.Parse(os.Getenv("STAFFY_DEV_PRINCIPAL_ID"))
        if err != nil {
                return errors.New("STAFFY_DEV_PRINCIPAL_ID must be a UUID")
        }
        resolver, err := authdev.New(environment, principalID)
        if err != nil {
                return err
        }
        databaseURL := os.Getenv("STAFFY_DATABASE_URL")
        if databaseURL == "" {
                return errors.New("STAFFY_DATABASE_URL is required")
        }
        pool, err := pgxpool.New(context.Background(), databaseURL)
        if err != nil {
                return err
        }
        defer pool.Close()
        if err := verifyRuntimeDatabase(context.Background(), pool); err != nil {
                return err
        }

        clock := systemClock{}
        ids := uuidGenerator{}
        identityService := app.New(store.NewPostgres(pool), clock, ids)
        transport, err := httptransport.New(identityService, resolver, ids, clock, logger)
        if err != nil {
                return err
        }
        var testTTS employeePort.TestTTS
        if binary := os.Getenv("STAFFY_DEV_ESPEAK_NG_BIN"); binary != "" {
                testTTS = &employeeApp.ESpeakNG{Binary: binary, DataParent: os.Getenv("STAFFY_DEV_ESPEAK_DATA_PATH"), LibraryPath: os.Getenv("STAFFY_DEV_ESPEAK_LIBRARY_PATH")}
        }
        employeeService := employeeApp.NewWithTestTTS(employeeStore.NewPostgres(pool), identityService, clock, ids, testTTS)
        connectorService := connectorApp.New(connectorStore.NewPostgres(pool), identityService, connectorVerifier.Synthetic{}, clock, ids)
        // The release surface (Story 1-12) consumes connection facts through the
        // narrow port; without it sealing and restoring honestly refuse.
        employeeService = employeeService.WithConnectorFacts(connectorService)
        transport.BindEmployee(employeeService)
        transport.BindConnector(connectorService)
        privacyExecutors := map[string]privacyPort.ClassDeletionExecutor{
                privacyDomain.ExecutorEmployee: employeeService.PrivacyDeletionExecutor(),
                privacyDomain.ExecutorIdentity: identityService.PrivacyDeletionExecutor(),
        }
        transport.BindPrivacy(privacyApp.New(privacyStore.NewPostgres(pool), identityService, privacyExecutors, clock, ids))
        transport.BindEntitlement(entitlementApp.New(entitlementStore.NewPostgres(pool), identityService, clock, ids))
        server := &http.Server{
                Addr:              listenAddress,
                Handler:           transport.Handler(),
                ReadHeaderTimeout: 5 * time.Second,
                ReadTimeout:       15 * time.Second,
                WriteTimeout:      30 * time.Second,
                IdleTimeout:       60 * time.Second,
                MaxHeaderBytes:    1 << 20,
        }

        ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
        defer stop()
        errCh := make(chan error, 1)
        go func() { errCh <- server.ListenAndServe() }()
        select {
        case err := <-errCh:
                if errors.Is(err, http.ErrServerClosed) {
                        return nil
                }
                return err
        case <-ctx.Done():
                shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
                defer cancel()
                return server.Shutdown(shutdownCtx)
        }
}

type databaseVerifier interface {
        QueryRow(context.Context, string, ...any) pgx.Row
}

func verifyRuntimeDatabase(ctx context.Context, database databaseVerifier) error {
        var role string
        var superuser, bypassRLS bool
        if err := database.QueryRow(ctx, `
                select current_user, rolsuper, rolbypassrls
                from pg_roles where rolname = current_user`).Scan(&role, &superuser, &bypassRLS); err != nil {
                return fmt.Errorf("verify runtime database role: %w", err)
        }
        if superuser || bypassRLS {
                return fmt.Errorf("runtime database role %q may bypass row-level security", role)
        }
        return nil
}

func envOr(key, fallback string) string {
        if value := os.Getenv(key); value != "" {
                return value
        }
        return fallback
}

func loopbackAddress(address string) bool {
        host, _, err := net.SplitHostPort(address)
        if err != nil {
                return false
        }
        if host == "localhost" {
                return true
        }
        ip := net.ParseIP(host)
        return ip != nil && ip.IsLoopback()
}
