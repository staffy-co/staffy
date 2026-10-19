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
        _ "time/tzdata" // IANA zones without a system zone database (the profile window validator loads zones)

        "github.com/artemiosu/staffy/internal/adapters/authdev"
        "github.com/artemiosu/staffy/internal/adapters/capabilityresolver"
        "github.com/artemiosu/staffy/internal/adapters/connectorrefresolver"
        "github.com/artemiosu/staffy/internal/adapters/controleffectiveresolver"
        "github.com/artemiosu/staffy/internal/adapters/ingresssyn"
        "github.com/artemiosu/staffy/internal/adapters/packresolver"
        "github.com/artemiosu/staffy/internal/adapters/verificationsyn"
        actionApp "github.com/artemiosu/staffy/internal/action/app"
        actionCatalog "github.com/artemiosu/staffy/internal/action/catalog"
        casesApp "github.com/artemiosu/staffy/internal/cases/app"
        casesStore "github.com/artemiosu/staffy/internal/cases/store"
        connectorApp "github.com/artemiosu/staffy/internal/connector/app"
        connectorStore "github.com/artemiosu/staffy/internal/connector/store"
        connectorVerifier "github.com/artemiosu/staffy/internal/connector/verifier"
        controlApp "github.com/artemiosu/staffy/internal/control/app"
        controlStore "github.com/artemiosu/staffy/internal/control/store"
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
        entitlementService := entitlementApp.New(entitlementStore.NewPostgres(pool), identityService, clock, ids)
        // The release surface (Story 1-12) consumes connection facts through the
        // narrow port; without it sealing and restoring honestly refuse.
        employeeService = employeeService.WithConnectorFacts(connectorService)
        // The readiness surface (Story 1-14) consumes the derived entitlement
        // admission state through the narrow fact port; without it the
        // derivation honestly refuses instead of assuming "ok".
        employeeService = employeeService.WithEntitlementFacts(entitlementService)
        // The RouteFacts facade (Story 2-1) relays the scope-based
        // entitlement admission word for the ingress ladder.
        employeeService = employeeService.WithEntitlementAdmissionFacts(entitlementService)
        transport.BindEmployee(employeeService)
        transport.BindRelease(employeeService)
        transport.BindProfile(employeeService)
        transport.BindReadiness(employeeService)
        transport.BindConnector(connectorService)
        privacyExecutors := map[string]privacyPort.ClassDeletionExecutor{
                privacyDomain.ExecutorEmployee: employeeService.PrivacyDeletionExecutor(),
                privacyDomain.ExecutorIdentity: identityService.PrivacyDeletionExecutor(),
        }
        transport.BindPrivacy(privacyApp.New(privacyStore.NewPostgres(pool), identityService, privacyExecutors, clock, ids))
        transport.BindEntitlement(entitlementService)
        // The case contour (Story 2-1): the machine ingress over the
        // synthetic source verifier of the test contour (production stays
        // fail-closed — AG-4), the operator Cases/Timeline surface and the
        // sealed payload reads. The ingress scope is derived from the
        // verified connection, never from a user principal.
        casesService := casesApp.New(casesStore.NewPostgres(pool).WithIDs(ids), identityService, ingresssyn.New(connectorService), clock, ids).
                WithRouteFacts(employeeService).
                WithConnectionFacts(connectorService).
                WithMembers(identityService).
                WithPackTaxonomy(employeeService).
                WithIdentityVerifier(verificationsyn.New()).
                // The Story 3.3 enforcement point: the drain fence reads the LIVE
                // effective control of the delivery route (the tenant point + the
                // channel point) through the composition adapter over the control
                // rows (the DAG discipline: the case module consumes the
                // consumer-owned port only; the adapter reads the rows behind the
                // drain's own configuration gate — the control UI gates stay out
                // of a machine fence). The DISPATCHER is deliberately NOT wired
                // — AG-4: no real providers exist, the drain refuses in words and
                // the records stay queued; the synthetic dispatcher lives in the
                // test-contour adapter package this root never imports.
                WithControlEffective(controleffectiveresolver.New(controlStore.NewPostgres(pool), clock))
        transport.BindCases(casesService)
        // The action catalog (Story 3.1): the read-only surface of the authorized
        // action dictionary over the composition-root resolvers (the live pack
        // and connector catalogs through the ports) and the honestly empty
        // binary dictionary (AG-4 — the first entry appears only with a
        // published pack connector and a fully admitted definition).
        transport.BindActionCatalog(actionApp.New(identityService, packresolver.New(), capabilityresolver.New(), actionCatalog.All()))
        // The scoped controls surface (Story 3.2): the operator emergency
        // restriction module over its own PG store, the live identity gates
        // and the connector ref resolution through the consumer-owned port
        // (the composition-root adapter over the live 1-8 catalog — the
        // packresolver/capabilityresolver precedent of 3.1).
        controlService := controlApp.New(controlStore.NewPostgres(pool), identityService, connectorrefresolver.New(), clock, ids)
        transport.BindControls(controlService)
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
