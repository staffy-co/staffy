package store

import (
        "context"
        "encoding/json"
        "errors"

        entitlementDomain "github.com/artemiosu/staffy/internal/entitlement/domain"
        "github.com/artemiosu/staffy/internal/entitlement/port"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
        "github.com/jackc/pgx/v5"
        "github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (s *Postgres) begin(ctx context.Context, scope tenancy.Context, readonly bool) (pgx.Tx, error) {
        if !scope.Valid() {
                return nil, tenancy.ErrMissingContext
        }
        opts := pgx.TxOptions{}
        if readonly {
                opts.AccessMode = pgx.ReadOnly
        }
        tx, err := s.pool.BeginTx(ctx, opts)
        if err != nil {
                return nil, err
        }
        for _, pair := range [][2]string{{"staffy.principal_id", scope.PrincipalID().String()}, {"staffy.tenant_id", scope.TenantID().String()}} {
                if _, err := tx.Exec(ctx, "select set_config($1,$2,true)", pair[0], pair[1]); err != nil {
                        _ = tx.Rollback(ctx)
                        return nil, err
                }
        }
        return tx, nil
}

func (s *Postgres) lock(ctx context.Context, tx pgx.Tx, scope tenancy.Context, kind, key string) error {
        lock := scope.TenantID().String() + ":" + kind + ":" + key
        _, err := tx.Exec(ctx, "select pg_advisory_xact_lock(hashtextextended($1,0))", lock)
        return err
}

func stringsList(raw []byte) []string {
        out := []string{}
        if len(raw) > 0 {
                _ = json.Unmarshal(raw, &out)
        }
        return out
}

func jsonList(values []string) []byte {
        if values == nil {
                values = []string{}
        }
        out, err := json.Marshal(values)
        if err != nil {
                return []byte("[]")
        }
        return out
}

func categoriesList(categories []entitlementDomain.UsageCategory) []byte {
        raw := make([]string, 0, len(categories))
        for _, category := range categories {
                raw = append(raw, string(category))
        }
        return jsonList(raw)
}

func limitsList(limits []entitlementDomain.CategoryLimit) []byte {
        if limits == nil {
                limits = []entitlementDomain.CategoryLimit{}
        }
        out, err := json.Marshal(limits)
        if err != nil {
                return []byte("[]")
        }
        return out
}

func anomalyList(thresholds []entitlementDomain.AnomalyThreshold) []byte {
        if thresholds == nil {
                thresholds = []entitlementDomain.AnomalyThreshold{}
        }
        out, err := json.Marshal(thresholds)
        if err != nil {
                return []byte("[]")
        }
        return out
}

type row interface{ Scan(dest ...any) error }

// ---------------------------------------------------------------------------
// Terms versions
// ---------------------------------------------------------------------------

const termsColumns = "tenant_id,version,currency,tax_regime,period_start,period_end,included_employees,included_channels,included_connectors,limits,launch_price,subscription_price,change_note,actor_id,created_at"

func scanTerms(r row) (entitlementDomain.TermsVersion, error) {
        var t entitlementDomain.TermsVersion
        var employees, channels, connectors, limits []byte
        err := r.Scan(&t.TenantID, &t.Version, &t.Currency, &t.TaxRegime, &t.PeriodStart, &t.PeriodEnd,
                &employees, &channels, &connectors, &limits, &t.LaunchPrice, &t.SubscriptionPrice,
                &t.ChangeNote, &t.ActorID, &t.CreatedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return entitlementDomain.TermsVersion{}, entitlementDomain.ErrNotFound
        }
        if err != nil {
                return entitlementDomain.TermsVersion{}, err
        }
        t.IncludedEmployees = stringsList(employees)
        t.IncludedChannels = stringsList(channels)
        t.IncludedConnectors = stringsList(connectors)
        t.Limits = []entitlementDomain.CategoryLimit{}
        if len(limits) > 0 {
                _ = json.Unmarshal(limits, &t.Limits)
        }
        return t, nil
}

// SaveTerms appends the next terms version under an advisory lock.
func (s *Postgres) SaveTerms(ctx context.Context, p port.TermsParams) (entitlementDomain.TermsVersion, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return entitlementDomain.TermsVersion{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        if err := s.lock(ctx, tx, p.Scope, "entitlement.terms", "v"); err != nil {
                return entitlementDomain.TermsVersion{}, err
        }
        var version int64
        if err = tx.QueryRow(ctx, "select coalesce(max(version),0)+1 from entitlement.terms_versions where tenant_id=$1", p.Scope.TenantID()).Scan(&version); err != nil {
                return entitlementDomain.TermsVersion{}, err
        }
        terms := p.Terms
        terms.TenantID = p.Scope.TenantID().String()
        terms.Version = version
        terms.ActorID = p.Scope.PrincipalID().String()
        terms.CreatedAt = p.Now
        if _, err = tx.Exec(ctx, "insert into entitlement.terms_versions("+termsColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)",
                terms.TenantID, terms.Version, terms.Currency, terms.TaxRegime, terms.PeriodStart, terms.PeriodEnd,
                jsonList(terms.IncludedEmployees), jsonList(terms.IncludedChannels), jsonList(terms.IncludedConnectors),
                limitsList(terms.Limits), terms.LaunchPrice, terms.SubscriptionPrice, terms.ChangeNote, terms.ActorID, terms.CreatedAt); err != nil {
                return entitlementDomain.TermsVersion{}, err
        }
        return terms, tx.Commit(ctx)
}

// ListTerms returns every stored terms version.
func (s *Postgres) ListTerms(ctx context.Context, scope tenancy.Context) ([]entitlementDomain.TermsVersion, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+termsColumns+" from entitlement.terms_versions where tenant_id=$1 order by version desc", scope.TenantID())
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []entitlementDomain.TermsVersion{}
        for rows.Next() {
                t, err := scanTerms(rows)
                if err != nil {
                        return nil, err
                }
                out = append(out, t)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        return out, tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// State events
// ---------------------------------------------------------------------------

const stateEventColumns = "tenant_id,event_id,event,terms_version,basis,limits_check,actor_id,created_at"

func scanStateEvent(r row) (entitlementDomain.StateEvent, error) {
        var e entitlementDomain.StateEvent
        var limitsCheck *string
        err := r.Scan(&e.TenantID, &e.EventID, &e.Event, &e.TermsVersion, &e.Basis, &limitsCheck, &e.ActorID, &e.CreatedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return entitlementDomain.StateEvent{}, entitlementDomain.ErrNotFound
        }
        if err != nil {
                return entitlementDomain.StateEvent{}, err
        }
        if limitsCheck != nil {
                e.LimitsCheck = *limitsCheck
        }
        return e, nil
}

// SaveStateEvent appends one lifecycle event.
func (s *Postgres) SaveStateEvent(ctx context.Context, p port.StateEventParams) error {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        event := p.Event
        event.TenantID = p.Scope.TenantID().String()
        event.ActorID = p.Scope.PrincipalID().String()
        event.CreatedAt = p.Now
        var limitsCheck interface{}
        if event.LimitsCheck != "" {
                limitsCheck = event.LimitsCheck
        }
        if _, err = tx.Exec(ctx, "insert into entitlement.state_events("+stateEventColumns+") values($1,$2,$3,$4,$5,$6,$7,$8)",
                event.TenantID, event.EventID, event.Event, event.TermsVersion, event.Basis, limitsCheck, event.ActorID, event.CreatedAt); err != nil {
                return err
        }
        return tx.Commit(ctx)
}

// ListStateEvents returns every stored lifecycle event.
func (s *Postgres) ListStateEvents(ctx context.Context, scope tenancy.Context) ([]entitlementDomain.StateEvent, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+stateEventColumns+" from entitlement.state_events where tenant_id=$1 order by created_at desc, event_id desc", scope.TenantID())
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []entitlementDomain.StateEvent{}
        for rows.Next() {
                e, err := scanStateEvent(rows)
                if err != nil {
                        return nil, err
                }
                out = append(out, e)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        return out, tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Usage events (their own idempotency records)
// ---------------------------------------------------------------------------

const usageColumns = "tenant_id,event_id,event_key,payload_hash,source,category,quantity,unit,amount,currency,is_estimate,period_start,period_end,occurred_at,ingested_at,employee_id,channel,release_version,provenance,agg_rule_version,actor_id"

func scanUsageEvent(r row) (entitlementDomain.UsageEvent, error) {
        var e entitlementDomain.UsageEvent
        var category string
        var employee, channel, release *string
        err := r.Scan(&e.TenantID, &e.EventID, &e.EventKey, &e.PayloadHash, &e.Source, &category, &e.Quantity, &e.Unit,
                &e.Amount, &e.Currency, &e.Estimate, &e.PeriodStart, &e.PeriodEnd, &e.OccurredAt, &e.IngestedAt,
                &employee, &channel, &release, &e.Provenance, &e.AggRuleVersion, &e.ActorID)
        if errors.Is(err, pgx.ErrNoRows) {
                return entitlementDomain.UsageEvent{}, entitlementDomain.ErrNotFound
        }
        if err != nil {
                return entitlementDomain.UsageEvent{}, err
        }
        e.Category = entitlementDomain.UsageCategory(category)
        e.EmployeeID = derefString(employee)
        e.Channel = derefString(channel)
        e.ReleaseVersion = derefString(release)
        return e, nil
}

func derefString(value *string) string {
        if value == nil {
                return ""
        }
        return *value
}

// IngestUsage appends one usage event atomically under a per-key advisory
// lock: the same key with the same payload hash replays the stored event;
// a different hash is an honest conflict. The event is the idempotency
// record — there is no separate key table.
func (s *Postgres) IngestUsage(ctx context.Context, p port.IngestParams) (entitlementDomain.UsageEvent, bool, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return entitlementDomain.UsageEvent{}, false, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        if err := s.lock(ctx, tx, p.Scope, "entitlement.usage.v1", p.Event.EventKey); err != nil {
                return entitlementDomain.UsageEvent{}, false, err
        }
        existing, err := scanUsageEvent(tx.QueryRow(ctx, "select "+usageColumns+" from entitlement.usage_events where tenant_id=$1 and event_key=$2", p.Scope.TenantID(), p.Event.EventKey))
        if err == nil {
                if existing.PayloadHash != p.Event.PayloadHash {
                        return entitlementDomain.UsageEvent{}, false, entitlementDomain.ErrEventKeyConflict
                }
                return existing, true, tx.Commit(ctx)
        }
        if !errors.Is(err, entitlementDomain.ErrNotFound) {
                return entitlementDomain.UsageEvent{}, false, err
        }
        event := p.Event
        event.TenantID = p.Scope.TenantID().String()
        event.ActorID = p.Scope.PrincipalID().String()
        event.IngestedAt = p.Now
        if _, err = tx.Exec(ctx, "insert into entitlement.usage_events("+usageColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)",
                event.TenantID, event.EventID, event.EventKey, event.PayloadHash, event.Source, string(event.Category), event.Quantity, event.Unit,
                event.Amount, event.Currency, event.Estimate, event.PeriodStart, event.PeriodEnd, event.OccurredAt, event.IngestedAt,
                event.EmployeeID, event.Channel, event.ReleaseVersion, event.Provenance, event.AggRuleVersion, event.ActorID); err != nil {
                return entitlementDomain.UsageEvent{}, false, err
        }
        return event, false, tx.Commit(ctx)
}

// ListUsageEvents returns every stored usage event of the tenant. No LIMIT:
// the meter aggregates ALL facts on read — a truncated list would silently
// undercount the sums while the data status claims completeness (R1).
func (s *Postgres) ListUsageEvents(ctx context.Context, scope tenancy.Context) ([]entitlementDomain.UsageEvent, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+usageColumns+" from entitlement.usage_events where tenant_id=$1 order by ingested_at desc, event_id desc", scope.TenantID())
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []entitlementDomain.UsageEvent{}
        for rows.Next() {
                e, err := scanUsageEvent(rows)
                if err != nil {
                        return nil, err
                }
                out = append(out, e)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        return out, tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Usage sources
// ---------------------------------------------------------------------------

const sourceColumns = "tenant_id,registration_id,name,categories,freshness_minutes,retired,note,actor_id,created_at"

func scanSource(r row) (entitlementDomain.UsageSource, error) {
        var source entitlementDomain.UsageSource
        var categories []byte
        err := r.Scan(&source.TenantID, &source.RegistrationID, &source.Name, &categories, &source.FreshnessMinutes,
                &source.Retired, &source.Note, &source.ActorID, &source.CreatedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return entitlementDomain.UsageSource{}, entitlementDomain.ErrNotFound
        }
        if err != nil {
                return entitlementDomain.UsageSource{}, err
        }
        source.Categories = []entitlementDomain.UsageCategory{}
        for _, raw := range stringsList(categories) {
                source.Categories = append(source.Categories, entitlementDomain.UsageCategory(raw))
        }
        return source, nil
}

// SaveSource appends one source registration.
func (s *Postgres) SaveSource(ctx context.Context, p port.SourceParams) (entitlementDomain.UsageSource, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return entitlementDomain.UsageSource{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        source := p.Source
        source.TenantID = p.Scope.TenantID().String()
        source.ActorID = p.Scope.PrincipalID().String()
        source.CreatedAt = p.Now
        if _, err = tx.Exec(ctx, "insert into entitlement.usage_sources("+sourceColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9)",
                source.TenantID, source.RegistrationID, source.Name, categoriesList(source.Categories), source.FreshnessMinutes,
                source.Retired, source.Note, source.ActorID, source.CreatedAt); err != nil {
                return entitlementDomain.UsageSource{}, err
        }
        return source, tx.Commit(ctx)
}

// ListSources returns every stored source registration.
func (s *Postgres) ListSources(ctx context.Context, scope tenancy.Context) ([]entitlementDomain.UsageSource, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+sourceColumns+" from entitlement.usage_sources where tenant_id=$1 order by created_at desc, registration_id desc", scope.TenantID())
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []entitlementDomain.UsageSource{}
        for rows.Next() {
                source, err := scanSource(rows)
                if err != nil {
                        return nil, err
                }
                out = append(out, source)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        return out, tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Budget versions
// ---------------------------------------------------------------------------

const budgetColumns = "tenant_id,version,amount,currency,period_start,period_end,warning_threshold_percent,hard_limit,limit_policy,anomaly_thresholds,note,actor_id,created_at"

func scanBudget(r row) (entitlementDomain.BudgetVersion, error) {
        var b entitlementDomain.BudgetVersion
        var thresholds []byte
        err := r.Scan(&b.TenantID, &b.Version, &b.Amount, &b.Currency, &b.PeriodStart, &b.PeriodEnd,
                &b.WarningPercent, &b.HardLimit, &b.LimitPolicy, &thresholds, &b.Note, &b.ActorID, &b.CreatedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return entitlementDomain.BudgetVersion{}, entitlementDomain.ErrNotFound
        }
        if err != nil {
                return entitlementDomain.BudgetVersion{}, err
        }
        b.AnomalyThresholds = []entitlementDomain.AnomalyThreshold{}
        if len(thresholds) > 0 {
                _ = json.Unmarshal(thresholds, &b.AnomalyThresholds)
        }
        return b, nil
}

// SaveBudget appends the next budget version under an advisory lock.
func (s *Postgres) SaveBudget(ctx context.Context, p port.BudgetParams) (entitlementDomain.BudgetVersion, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return entitlementDomain.BudgetVersion{}, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        if err := s.lock(ctx, tx, p.Scope, "entitlement.budget", "v"); err != nil {
                return entitlementDomain.BudgetVersion{}, err
        }
        var version int64
        if err = tx.QueryRow(ctx, "select coalesce(max(version),0)+1 from entitlement.budget_versions where tenant_id=$1", p.Scope.TenantID()).Scan(&version); err != nil {
                return entitlementDomain.BudgetVersion{}, err
        }
        budget := p.Budget
        budget.TenantID = p.Scope.TenantID().String()
        budget.Version = version
        budget.ActorID = p.Scope.PrincipalID().String()
        budget.CreatedAt = p.Now
        if _, err = tx.Exec(ctx, "insert into entitlement.budget_versions("+budgetColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)",
                budget.TenantID, budget.Version, budget.Amount, budget.Currency, budget.PeriodStart, budget.PeriodEnd,
                budget.WarningPercent, budget.HardLimit, budget.LimitPolicy, anomalyList(budget.AnomalyThresholds),
                budget.Note, budget.ActorID, budget.CreatedAt); err != nil {
                return entitlementDomain.BudgetVersion{}, err
        }
        return budget, tx.Commit(ctx)
}

// ListBudgets returns every stored budget version.
func (s *Postgres) ListBudgets(ctx context.Context, scope tenancy.Context) ([]entitlementDomain.BudgetVersion, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+budgetColumns+" from entitlement.budget_versions where tenant_id=$1 order by version desc", scope.TenantID())
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []entitlementDomain.BudgetVersion{}
        for rows.Next() {
                b, err := scanBudget(rows)
                if err != nil {
                        return nil, err
                }
                out = append(out, b)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        return out, tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Exceptions and acknowledgements
// ---------------------------------------------------------------------------

const exceptionColumns = "tenant_id,exception_id,dedupe_key,cause,message,data_status,affected_employee,affected_route,recommended_action,agg_rule_version,actor_id,created_at"

func scanException(r row) (entitlementDomain.UsageException, error) {
        var e entitlementDomain.UsageException
        var employee, route *string
        err := r.Scan(&e.TenantID, &e.ExceptionID, &e.DedupeKey, &e.Cause, &e.Message, &e.DataStatus,
                &employee, &route, &e.RecommendedAction, &e.AggRuleVersion, &e.ActorID, &e.CreatedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return entitlementDomain.UsageException{}, entitlementDomain.ErrNotFound
        }
        if err != nil {
                return entitlementDomain.UsageException{}, err
        }
        e.AffectedEmployee = derefString(employee)
        e.AffectedRoute = derefString(route)
        return e, nil
}

// SaveException appends one exception; a duplicate dedupe key is a silent
// no-op returning created=false (the exceptions are deduplicated by key).
func (s *Postgres) SaveException(ctx context.Context, p port.ExceptionParams) (entitlementDomain.UsageException, bool, error) {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return entitlementDomain.UsageException{}, false, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        exception := p.Exception
        exception.TenantID = p.Scope.TenantID().String()
        exception.ActorID = p.Scope.PrincipalID().String()
        exception.CreatedAt = p.Now
        inserted, err := tx.Exec(ctx, "insert into entitlement.usage_exceptions("+exceptionColumns+") values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) on conflict (tenant_id, dedupe_key) do nothing",
                exception.TenantID, exception.ExceptionID, exception.DedupeKey, exception.Cause, exception.Message, exception.DataStatus,
                exception.AffectedEmployee, exception.AffectedRoute, exception.RecommendedAction, exception.AggRuleVersion, exception.ActorID, exception.CreatedAt)
        if err != nil {
                return entitlementDomain.UsageException{}, false, err
        }
        if inserted.RowsAffected() == 0 {
                return exception, false, tx.Commit(ctx)
        }
        return exception, true, tx.Commit(ctx)
}

// ExceptionExists answers whether a dedupe key is already stored.
func (s *Postgres) ExceptionExists(ctx context.Context, scope tenancy.Context, dedupeKey string) (bool, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return false, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        var exists bool
        if err = tx.QueryRow(ctx, "select exists(select 1 from entitlement.usage_exceptions where tenant_id=$1 and dedupe_key=$2)", scope.TenantID(), dedupeKey).Scan(&exists); err != nil {
                return false, err
        }
        return exists, tx.Commit(ctx)
}

// ListExceptions returns the stored exceptions with their derived
// acknowledgements (the latest ack event per exception).
func (s *Postgres) ListExceptions(ctx context.Context, scope tenancy.Context) ([]entitlementDomain.UsageException, error) {
        tx, err := s.begin(ctx, scope, true)
        if err != nil {
                return nil, err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        rows, err := tx.Query(ctx, "select "+exceptionColumns+" from entitlement.usage_exceptions where tenant_id=$1 order by created_at desc, exception_id desc limit 200", scope.TenantID())
        if err != nil {
                return nil, err
        }
        defer rows.Close()
        out := []entitlementDomain.UsageException{}
        for rows.Next() {
                e, err := scanException(rows)
                if err != nil {
                        return nil, err
                }
                out = append(out, e)
        }
        if err = rows.Err(); err != nil {
                return nil, err
        }
        for i := range out {
                ack, err := scanAck(tx.QueryRow(ctx, "select tenant_id,ack_id,exception_id,actor_id,note,created_at from entitlement.exception_acknowledgements where tenant_id=$1 and exception_id=$2 order by created_at desc, ack_id desc limit 1", scope.TenantID(), out[i].ExceptionID))
                if err == nil {
                        out[i].Acknowledged = &ack
                } else if !errors.Is(err, entitlementDomain.ErrNotFound) {
                        return nil, err
                }
        }
        return out, tx.Commit(ctx)
}

func scanAck(r row) (entitlementDomain.ExceptionAck, error) {
        var a entitlementDomain.ExceptionAck
        err := r.Scan(&a.TenantID, &a.AckID, &a.ExceptionID, &a.ActorID, &a.Note, &a.CreatedAt)
        if errors.Is(err, pgx.ErrNoRows) {
                return entitlementDomain.ExceptionAck{}, entitlementDomain.ErrNotFound
        }
        if err != nil {
                return entitlementDomain.ExceptionAck{}, err
        }
        return a, nil
}

// SaveAck appends one exception acknowledgement.
func (s *Postgres) SaveAck(ctx context.Context, p port.AckParams) error {
        tx, err := s.begin(ctx, p.Scope, false)
        if err != nil {
                return err
        }
        defer func() { _ = tx.Rollback(ctx) }()
        ack := p.Ack
        ack.TenantID = p.Scope.TenantID().String()
        ack.ActorID = p.Scope.PrincipalID().String()
        ack.CreatedAt = p.Now
        if _, err = tx.Exec(ctx, "insert into entitlement.exception_acknowledgements(tenant_id,ack_id,exception_id,actor_id,note,created_at) values($1,$2,$3,$4,$5,$6)",
                ack.TenantID, ack.AckID, ack.ExceptionID, ack.ActorID, ack.Note, ack.CreatedAt); err != nil {
                return err
        }
        return tx.Commit(ctx)
}

var _ port.Repository = (*Postgres)(nil)
