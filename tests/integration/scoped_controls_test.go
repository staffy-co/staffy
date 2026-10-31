//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/adapters/connectorrefresolver"
	controlAPI "github.com/staffy-co/staffy/internal/control/api"
	controlApp "github.com/staffy-co/staffy/internal/control/app"
	controlDomain "github.com/staffy-co/staffy/internal/control/domain"
	controlStore "github.com/staffy-co/staffy/internal/control/store"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityApp "github.com/staffy-co/staffy/internal/identity/app"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	identityStore "github.com/staffy-co/staffy/internal/identity/store"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// controlScopeWords mirrors the app wrap of the honest 403 (the words of
// the ONE scope that opens the control surfaces — the single source
// lives in the app layer; the mirror pins the text against silent
// drift, the transport-test precedent of step-03).
const controlScopeWords = "Поверхность аварийного ограничения работы требует полномочия operations: точечная остановка и возобновление маршрутов — операторский материал, участнику без operations он не открывается до вычисления итогового режима."

// TestScopedControlsMatrixRowsBySpec is the Story 3-2 step-04 matrix:
// the honest rows of the scoped control surface walked by name over the
// REAL app service with the REAL store, the REAL live connector
// registry adapter and the LIVE identity/PG membership (the step-03
// transport test pinned the transport mapping over the fake; this
// matrix walks the live gate order, the optimistic concurrency, the
// monotonic generations, the read-time windows, the append-only audit
// truth, the RLS boundary and the fail-closed corruption reading). The
// identity ids live in the free b2xx suffix range — no other test
// world derives from them (the membership collision lesson of a2xx).
func TestScopedControlsMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	identity, owner := newService("00000000b201", "00000000b202", "00000000b203")
	tenant := createTenant(t, identity, owner, "Аварийное ограничение", "control-matrix-create-1", "00000000b204").Tenant.ID
	// The membership-clock rebuild of the membership-test precedent: the
	// invitation UPDATE compares expires_at against the LIVE database
	// clock_timestamp(), so the service clock must sit at (near) the real
	// now — the frozen newService clock of 2026-09-16 would make every
	// invitation look expired at the acceptance UPDATE.
	identity = identityApp.New(identityStore.NewPostgres(runtimePool), fixedClock{time.Now().UTC().Truncate(time.Microsecond)}, &sequenceIDs{})
	owner.StepUpVerified = true
	invite := func(email, inviteSuffix, principalSuffix string, roles []identityDomain.Role, scopes []identityDomain.Scope) identityDomain.Principal {
		t.Helper()
		corr := id.MustParse("01994f64-6c00-7000-8000-" + inviteSuffix)
		_, token, err := identity.InviteMember(ctx, identityAPI.MembershipCommand{
			Actor: owner, TenantID: tenant, Email: email, Roles: roles, Scopes: scopes, CorrelationID: corr,
		})
		if err != nil {
			t.Fatalf("invite %s: %v", email, err)
		}
		member := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-" + principalSuffix), VerifiedEmail: email}
		if _, err := identity.AcceptInvitation(ctx, member, token, corr); err != nil {
			t.Fatalf("accept %s: %v", email, err)
		}
		return member
	}
	// Two confirmed operators (the live race of the concurrency row) and
	// one review member without the operations scope (the honest 403
	// words row). The foreign world: the owner of ANOTHER tenant reading
	// this tenant (the cross-tenant 404 row) and a principal with no
	// membership at all (the stranger row).
	operator1 := invite("control-op1@example.org", "00000000b205", "00000000b206", []identityDomain.Role{identityDomain.Operator}, []identityDomain.Scope{identityDomain.Operations})
	operator2 := invite("control-op2@example.org", "00000000b207", "00000000b208", []identityDomain.Role{identityDomain.Operator}, []identityDomain.Scope{identityDomain.Operations})
	reviewer := invite("control-review@example.org", "00000000b209", "00000000b20a", []identityDomain.Role{identityDomain.ExpertQA}, []identityDomain.Scope{identityDomain.Review, identityDomain.Audit})
	operator1.StepUpVerified = true
	operator2.StepUpVerified = true
	foreignIdentity, foreignOwner := newService("00000000b211", "00000000b212", "00000000b213")
	foreignTenant := createTenant(t, foreignIdentity, foreignOwner, "Чужой контроль", "control-matrix-foreign-1", "00000000b214").Tenant.ID
	stranger := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-00000000b216")}

	// The REAL service: the live PG store, the live identity, the REAL
	// composition-root connector adapter over the live 1-8 catalog and
	// the frozen window clock (the read-time window semantics of the
	// matrix walk their legs against the fixed now of 2026-09-16 12:00
	// UTC).
	frozen := fixedClock{time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)}
	controls := controlApp.New(controlStore.NewPostgres(runtimePool), identity, connectorrefresolver.New(), frozen, &sequenceIDs{values: controlMatrixIDs(64)})

	// corr mints a distinct correlation id of the b3xx range (the
	// correlation trail of every matrix mutation).
	corr := func(suffix string) id.UUID {
		return id.MustParse("01994f64-6c00-7000-8000-" + suffix)
	}
	set := func(actor identityDomain.Principal, kind, ref, mode, reason string, expected int64, start, expires *time.Time, correlation id.UUID) (controlDomain.ScopedControl, error) {
		return controls.Set(ctx, actor, tenant, controlAPI.SetRequest{
			Kind: kind, Ref: ref, Mode: mode, Reason: reason,
			ExpectedGeneration: expected, ScheduledStartAt: start, ExpiresAt: expires,
			CorrelationID: correlation,
		})
	}
	resume := func(actor identityDomain.Principal, kind, ref, reason string, expected int64, correlation id.UUID) (controlDomain.ScopedControl, error) {
		return controls.Resume(ctx, actor, tenant, controlAPI.ResumeRequest{
			Kind: kind, Ref: ref, Reason: reason, ExpectedGeneration: expected, CorrelationID: correlation,
		})
	}
	effective := func(actor identityDomain.Principal, keys ...controlDomain.ScopeKey) (controlDomain.EffectiveControl, error) {
		return controls.EffectiveFor(ctx, actor, tenant, keys)
	}
	// auditCount/outboxCount read the append-only truth over the admin
	// pool (the superuser bypasses RLS — the probe reads across tenants
	// by the pair, the honest counters of the atomicity rows).
	auditCount := func(kind, ref string) int {
		t.Helper()
		var n int
		if err := adminPool.QueryRow(ctx, "select count(*) from control.audit where tenant_id=$1 and scope_kind=$2 and scope_ref=$3", tenant, kind, ref).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	outboxCount := func(kind, ref string) int {
		t.Helper()
		var n int
		if err := adminPool.QueryRow(ctx, "select count(*) from control.outbox_events where tenant_id=$1 and scope_kind=$2 and scope_ref=$3", tenant, kind, ref).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	auditTotal := func() int {
		t.Helper()
		var n int
		if err := adminPool.QueryRow(ctx, "select count(*) from control.audit where tenant_id=$1", tenant).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Row 1 — the stranger (no membership anywhere) is the honest 404:
	// membership FIRST, no control material leaves the service.
	t.Run("row-1 stranger: membership first, the honest 404", func(t *testing.T) {
		_, err := controls.List(ctx, stranger, tenant)
		if !errors.Is(err, controlDomain.ErrNotFound) {
			t.Fatalf("the non-member is the honest 404, got %v", err)
		}
		if strings.Contains(err.Error(), "phone") || strings.Contains(err.Error(), "контроль") {
			t.Fatalf("the 404 never carries control material: %s", err.Error())
		}
	})

	// Row 2 — the foreign member (the owner of another tenant) is the
	// honest 404 on THIS tenant: the cross-tenant boundary is closed by
	// the live membership truth.
	t.Run("row-2 foreign member: the cross-tenant 404", func(t *testing.T) {
		if _, err := controls.List(ctx, foreignOwner, tenant); !errors.Is(err, controlDomain.ErrNotFound) {
			t.Fatalf("the foreign member reads nothing of this tenant, got %v", err)
		}
	})

	// Row 3 — the zero actor is the honest 404 before any membership
	// roundtrip: no disclosure that the tenant has control points.
	t.Run("row-3 zero actor: the honest 404", func(t *testing.T) {
		if _, err := controls.List(ctx, identityDomain.Principal{}, tenant); !errors.Is(err, controlDomain.ErrNotFound) {
			t.Fatalf("the unparseable actor is the honest 404, got %v", err)
		}
	})

	// Row 4 — the review member (review+audit, no operations) is the 403
	// WITH the scope words BEFORE any material: the words name the one
	// scope that opens the surface; the refusal is the exact app wrap.
	t.Run("row-4 review member: the 403 words before material", func(t *testing.T) {
		_, err := controls.List(ctx, reviewer, tenant)
		if !errors.Is(err, controlDomain.ErrForbidden) {
			t.Fatalf("the member without operations is the honest 403, got %v", err)
		}
		if err.Error() != controlDomain.ErrForbidden.Error()+": "+controlScopeWords {
			t.Fatalf("the wrap is the sentinel plus the exact scope words, got %s", err.Error())
		}
		if strings.Contains(err.Error(), "phone") {
			t.Fatalf("the 403 never carries control material: %s", err.Error())
		}
	})

	// Row 5 — the mutation step-up gate: an operator without the fresh
	// confirmation answers ErrStepUpRequired and writes NOTHING — the
	// gate ladder runs before the transaction, the audit stays empty
	// (the whole tenant audit counter is pinned at zero: this is the
	// first mutation attempt of the world).
	t.Run("row-5 unconfirmed operator: step-up first, no audit row", func(t *testing.T) {
		unconfirmed := operator1
		unconfirmed.StepUpVerified = false
		_, err := set(unconfirmed, "channel", "phone", controlDomain.ControlModePaused, "без step-up", 0, nil, nil, corr("00000000b301"))
		if !errors.Is(err, identityDomain.ErrStepUpRequired) {
			t.Fatalf("the unconfirmed mutation is the step-up refusal, got %v", err)
		}
		if n := auditTotal(); n != 0 {
			t.Fatalf("the refused mutation writes no audit row, got %d", n)
		}
	})

	// Row 6 — THE LIVE RACE of two operators: both confirm the first
	// installation (expected 0) of the same point; the advisory lock
	// serializes the material identity — exactly ONE applied, the OTHER
	// is the committed conflict with the current material; BOTH audit
	// rows stay (AC-11: the history of both attempts is material).
	t.Run("row-6 the live race: one applied, one conflict, both audit rows", func(t *testing.T) {
		type raceOutcome struct {
			control controlDomain.ScopedControl
			err     error
		}
		results := make(chan raceOutcome, 2)
		racers := []identityDomain.Principal{operator1, operator2}
		correlations := []id.UUID{corr("00000000b302"), corr("00000000b303")}
		for i, racer := range racers {
			go func(actor identityDomain.Principal, correlation id.UUID) {
				control, err := controls.Set(ctx, actor, tenant, controlAPI.SetRequest{
					Kind: "channel", Ref: "phone", Mode: controlDomain.ControlModePaused,
					Reason:        fmt.Sprintf("живая гонка двух операторов: актор %s", actor.ID),
					CorrelationID: correlation,
				})
				results <- raceOutcome{control, err}
			}(racer, correlations[i])
		}
		first, second := <-results, <-results
		var applied, conflicted raceOutcome
		if first.err == nil && second.err != nil {
			applied, conflicted = first, second
		} else if second.err == nil && first.err != nil {
			applied, conflicted = second, first
		} else {
			t.Fatalf("exactly one racer applies and one conflicts, got %v / %v", first.err, second.err)
		}
		if applied.control.Generation != 1 || applied.control.State != controlDomain.ControlStateActive {
			t.Fatalf("the winner installs the first generation, got %+v", applied.control)
		}
		var conflict controlDomain.ConflictError
		if !errors.As(conflicted.err, &conflict) {
			t.Fatalf("the loser is the typed conflict, got %v", conflicted.err)
		}
		if conflict.Current.Generation != 1 || conflict.Current.Mode != controlDomain.ControlModePaused {
			t.Fatalf("the conflict carries the CURRENT material of the winner, got %+v", conflict.Current)
		}
		if n := auditCount("channel", "phone"); n != 2 {
			t.Fatalf("both attempts stay material in the audit (applied + conflict), got %d rows", n)
		}
		var appliedRows, conflictRows int
		if err := adminPool.QueryRow(ctx, "select count(*) from control.audit where tenant_id=$1 and scope_kind='channel' and scope_ref='phone' and outcome='applied'", tenant).Scan(&appliedRows); err != nil {
			t.Fatal(err)
		}
		if err := adminPool.QueryRow(ctx, "select count(*) from control.audit where tenant_id=$1 and scope_kind='channel' and scope_ref='phone' and outcome='conflict'", tenant).Scan(&conflictRows); err != nil {
			t.Fatal(err)
		}
		if appliedRows != 1 || conflictRows != 1 {
			t.Fatalf("one applied row and one conflict row, got %d/%d", appliedRows, conflictRows)
		}
		var expected, resulting int64
		var action, requestedMode string
		if err := adminPool.QueryRow(ctx, "select action, requested_mode, expected_generation, resulting_generation from control.audit where tenant_id=$1 and scope_kind='channel' and scope_ref='phone' and outcome='conflict'", tenant).Scan(&action, &requestedMode, &expected, &resulting); err != nil {
			t.Fatal(err)
		}
		if action != "set" || requestedMode != controlDomain.ControlModePaused || expected != 0 || resulting != 1 {
			t.Fatalf("the conflict row speaks the lost race honestly (set/paused/0->1), got %s/%s/%d->%d", action, requestedMode, expected, resulting)
		}
		if n := outboxCount("channel", "phone"); n != 1 {
			t.Fatalf("only the applied state change emits the outbox event, got %d", n)
		}
	})

	// Row 7 — the re-set over the live generation: the confirmed
	// expectation 1 applies, the generation grows to 2, the prev_mode
	// carries the material mode of the first installation.
	t.Run("row-7 re-set: the monotonic generation and the prev mode", func(t *testing.T) {
		control, err := set(operator1, "channel", "phone", controlDomain.ControlModeHumanOnly, "усиление ограничения после разбора", 1, nil, nil, corr("00000000b304"))
		if err != nil {
			t.Fatal(err)
		}
		if control.Generation != 2 || control.PrevMode != controlDomain.ControlModePaused || control.State != controlDomain.ControlStateActive {
			t.Fatalf("the re-set grows the generation and carries the previous mode, got %+v", control)
		}
		if control.Mode != controlDomain.ControlModeHumanOnly || control.ActorID != operator1.ID.String() {
			t.Fatalf("the material of the change travels verbatim, got %+v", control)
		}
	})

	// Row 8 — the resume lifts the point: the row survives (state
	// removed — history, not absence), the generation grows to 3, the
	// audit speaks the resume action with the honest EMPTY requested
	// mode, the outbox aggregate version IS the new generation.
	t.Run("row-8 resume: the state removed, the generation grows, the audit words", func(t *testing.T) {
		control, err := resume(operator2, "channel", "phone", "инцидент закрыт, работа возобновляется", 2, corr("00000000b305"))
		if err != nil {
			t.Fatal(err)
		}
		if control.Generation != 3 || control.State != controlDomain.ControlStateRemoved {
			t.Fatalf("the lift keeps the row and grows the generation, got %+v", control)
		}
		if control.Mode != controlDomain.ControlModeHumanOnly {
			t.Fatalf("the lifted word stays as the honest history material, got %s", control.Mode)
		}
		var action, requestedMode, outcome string
		var resulting int64
		if err := adminPool.QueryRow(ctx, "select action, requested_mode, resulting_generation, outcome from control.audit where tenant_id=$1 and scope_kind='channel' and scope_ref='phone' and action='resume'", tenant).Scan(&action, &requestedMode, &resulting, &outcome); err != nil {
			t.Fatal(err)
		}
		if action != "resume" || requestedMode != "" || resulting != 3 || outcome != "applied" {
			t.Fatalf("the resume audit row (resume/empty requested/3/applied), got %s/%s/%d/%s", action, requestedMode, resulting, outcome)
		}
		var aggregate int64
		if err := adminPool.QueryRow(ctx, "select aggregate_version from control.outbox_events where tenant_id=$1 and scope_kind='channel' and scope_ref='phone' order by aggregate_version desc limit 1", tenant).Scan(&aggregate); err != nil {
			t.Fatal(err)
		}
		if aggregate != 3 {
			t.Fatalf("the newest outbox event carries the new generation as the aggregate version, got %d", aggregate)
		}
	})

	// Row 9 — the re-set of the REMOVED point continues the monotonic
	// series (generation 4, state active again): removal NEVER resets
	// the generation, the future stale detection is never deceived.
	t.Run("row-9 re-set of the removed point: the series continues", func(t *testing.T) {
		control, err := set(operator1, "channel", "phone", controlDomain.ControlModeIntakeOnly, "повторная установка после инцидента — приём без обещаний", 3, nil, nil, corr("00000000b306"))
		if err != nil {
			t.Fatal(err)
		}
		if control.Generation != 4 || control.State != controlDomain.ControlStateActive || control.PrevMode != controlDomain.ControlModeHumanOnly {
			t.Fatalf("the re-installation continues the series, got %+v", control)
		}
	})

	// Row 10 — the corruption probe: a DIRECT damage write of an
	// out-of-dictionary mode word (the CHECK constraint is dropped
	// first — the documented probe path of the migration header) reads
	// as paused — the most safe applicable, fail-closed by reading,
	// never an expanded allowance; the corruption marker stays honest.
	t.Run("row-10 corrupted mode: the paused reading, fail-closed", func(t *testing.T) {
		if _, err := adminPool.Exec(ctx, "alter table control.scoped_controls drop constraint scoped_controls_mode_check"); err != nil {
			t.Fatalf("drop the mode check for the damage probe: %v", err)
		}
		if _, err := adminPool.Exec(ctx, "update control.scoped_controls set mode='zzz_broken' where tenant_id=$1 and scope_kind='channel' and scope_ref='phone'", tenant); err != nil {
			t.Fatalf("write the damaged word: %v", err)
		}
		points, err := controls.List(ctx, operator1, tenant)
		if err != nil {
			t.Fatal(err)
		}
		var phone *controlDomain.ScopedControl
		for i := range points {
			if points[i].ScopeKind == "channel" && points[i].ScopeRef == "phone" {
				phone = &points[i]
			}
		}
		if phone == nil {
			t.Fatal("the damaged point stays readable (history, not absence)")
		}
		if controlDomain.SafeReadMode(phone.Mode) != controlDomain.ControlModePaused || controlDomain.WordIn(controlDomain.ControlModes, phone.Mode) {
			t.Fatalf("the damaged word reads paused, got %s", phone.Mode)
		}
		computed, err := effective(operator1, controlDomain.ScopeKey{Kind: "channel", Ref: "phone"})
		if err != nil {
			t.Fatal(err)
		}
		if computed.Mode != controlDomain.ControlModePaused || !computed.Corrupted || len(computed.Influencing) != 1 {
			t.Fatalf("the damaged word counts as paused with the honest marker, got %+v", computed)
		}
		if controlDomain.ControlModeRank("zzz_broken") != controlDomain.ControlModeRank(controlDomain.ControlModePaused) {
			t.Fatal("the out-of-dictionary rank IS the paused rank (fail-closed)")
		}
		// The repair of the probe: the dictionary word returns, the
		// constraint returns byte-identical (the second line of
		// defense stays armed for every later row).
		if _, err := adminPool.Exec(ctx, "update control.scoped_controls set mode=$2 where tenant_id=$1 and scope_kind='channel' and scope_ref='phone'", tenant, controlDomain.ControlModeIntakeOnly); err != nil {
			t.Fatalf("repair the damaged word: %v", err)
		}
		if _, err := adminPool.Exec(ctx, "alter table control.scoped_controls add constraint scoped_controls_mode_check check (mode in ('accept','intake_only/no_promises','human_only','paused'))"); err != nil {
			t.Fatalf("re-arm the mode check: %v", err)
		}
	})

	// Row 11 — the second lift and the honest no-op refusal: lifting an
	// already-removed point is a RuleError (repeating a removal is not
	// a state change) and writes NO audit row — the counter stays.
	t.Run("row-11 the already-removed lift: the no-op refusal without an audit row", func(t *testing.T) {
		before := auditCount("channel", "phone")
		control, err := resume(operator1, "channel", "phone", "финальное снятие точки", 4, corr("00000000b307"))
		if err != nil {
			t.Fatal(err)
		}
		if control.Generation != 5 || control.State != controlDomain.ControlStateRemoved {
			t.Fatalf("the second lift removes the re-installed point, got %+v", control)
		}
		var rule controlDomain.RuleError
		if _, err := resume(operator2, "channel", "phone", "повторное снятие уже снятого", 5, corr("00000000b308")); !errors.As(err, &rule) || rule.Rule != "state" {
			t.Fatalf("the repeat lift is the honest state refusal, got %v", err)
		}
		if after := auditCount("channel", "phone"); after != before+1 {
			t.Fatalf("the refused no-op writes no audit row, got %d -> %d", before, after)
		}
	})

	// Row 12 — the resume of a missing point is the honest 404 with NO
	// audit row: the FK of the audit table honestly refuses a history
	// row of a point that never existed.
	t.Run("row-12 the missing point: the honest 404 without an audit row", func(t *testing.T) {
		if _, err := resume(operator1, "channel", "avito", "снятие несуществующей точки", 0, corr("00000000b309")); !errors.Is(err, controlDomain.ErrNotFound) {
			t.Fatalf("the missing point is the honest 404, got %v", err)
		}
		if n := auditCount("channel", "avito"); n != 0 {
			t.Fatalf("no history of a point that never existed, got %d", n)
		}
	})

	// Row 13 — the growth truth of the whole series: the audit carries
	// BOTH attempts of the race plus every applied change, the outbox
	// carries exactly the APPLIED state changes — one event per applied
	// mutation, the conflict is audit-only.
	t.Run("row-13 the audit and outbox growth of the series", func(t *testing.T) {
		if n := auditCount("channel", "phone"); n != 6 {
			t.Fatalf("the audit of the pair: race applied + race conflict + re-set + resume + re-set + second lift = 6, got %d", n)
		}
		if n := outboxCount("channel", "phone"); n != 5 {
			t.Fatalf("the outbox of the pair: every APPLIED change emits exactly one event (the conflict is audit-only), got %d", n)
		}
		var eventTypes, schemaVersions int
		if err := adminPool.QueryRow(ctx, "select count(*) from control.outbox_events where tenant_id=$1 and scope_kind='channel' and scope_ref='phone' and event_type='scoped_control.changed' and schema_version=1", tenant).Scan(&eventTypes); err != nil {
			t.Fatal(err)
		}
		if eventTypes != 5 {
			t.Fatalf("every event is scoped_control.changed@1, got %d", eventTypes)
		}
		if err := adminPool.QueryRow(ctx, "select count(*) from control.outbox_events where tenant_id=$1 and scope_kind='channel' and scope_ref='phone' and aggregate_version in (1,2,3,4,5) and published_at is null", tenant).Scan(&schemaVersions); err != nil {
			t.Fatal(err)
		}
		if schemaVersions != 5 {
			t.Fatalf("the aggregate version IS the new generation and the rows wait unpublished, got %d", schemaVersions)
		}
	})

	// Row 14 — the most-restrictive intersection on LIVE rows: two
	// acting points (human_only and intake_only/no_promises) meet — the
	// rank maximum wins, the provenance keeps BOTH, the winning reason
	// travels, the crossing words join in the ladder order.
	t.Run("row-14 the rank-maximum intersection on live rows", func(t *testing.T) {
		if _, err := set(operator1, "channel", "telegram", controlDomain.ControlModeHumanOnly, "регламент обхода поставщика мессенджера", 0, nil, nil, corr("00000000b30a")); err != nil {
			t.Fatal(err)
		}
		if _, err := set(operator2, "channel", "web_chat", controlDomain.ControlModeIntakeOnly, "инцидент качества веб-чата", 0, nil, nil, corr("00000000b30b")); err != nil {
			t.Fatal(err)
		}
		computed, err := effective(operator1,
			controlDomain.ScopeKey{Kind: "channel", Ref: "telegram"},
			controlDomain.ScopeKey{Kind: "channel", Ref: "web_chat"},
		)
		if err != nil {
			t.Fatal(err)
		}
		if computed.Mode != controlDomain.ControlModeHumanOnly {
			t.Fatalf("the rank maximum wins the intersection, got %s", computed.Mode)
		}
		if len(computed.Influencing) != 2 {
			t.Fatalf("the provenance keeps every influencing control, got %d", len(computed.Influencing))
		}
		if computed.Reason != "регламент обхода поставщика мессенджера" {
			t.Fatalf("the winning reason travels, got %s", computed.Reason)
		}
		if strings.Join(computed.Intersection, ";") != "intake_only/no_promises;human_only" {
			t.Fatalf("the crossing words join in the ladder order, got %v", computed.Intersection)
		}
	})

	// Row 15 — the scheduled-future window: the point exists, the card
	// material stays readable, but the point does NOT act yet — the
	// read-time window of the frozen now (12:00) excludes the 13:00
	// start; the absent-acting route reads accept.
	t.Run("row-15 the scheduled-future window: not acting yet", func(t *testing.T) {
		start := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)
		control, err := set(operator1, "channel", "email", controlDomain.ControlModePaused, "запланированная остановка почтового канала на регламент", 0, &start, nil, corr("00000000b30c"))
		if err != nil {
			t.Fatal(err)
		}
		if control.ScheduledStartAt == nil || !control.ScheduledStartAt.Equal(start) {
			t.Fatalf("the window bound travels verbatim, got %+v", control.ScheduledStartAt)
		}
		computed, err := effective(operator1, controlDomain.ScopeKey{Kind: "channel", Ref: "email"})
		if err != nil {
			t.Fatal(err)
		}
		if computed.Mode != controlDomain.ControlModeAccept || len(computed.Influencing) != 0 {
			t.Fatalf("the scheduled-future point does not act yet, got %+v", computed)
		}
	})

	// Row 16 — the expired window: the point acts NOWHERE anymore and
	// the expiry is IDEMPOTENT by construction — two equal re-reads of
	// the same rows answer the equal effective truth, nothing
	// materializes at the boundary moment.
	t.Run("row-16 the expired window: not acting, idempotent re-reads", func(t *testing.T) {
		expires := time.Date(2026, 9, 16, 11, 0, 0, 0, time.UTC)
		if _, err := set(operator2, "channel", "max", controlDomain.ControlModePaused, "остановка канала max — уже истекла", 0, nil, &expires, corr("00000000b30d")); err != nil {
			t.Fatal(err)
		}
		first, err := effective(operator1, controlDomain.ScopeKey{Kind: "channel", Ref: "max"})
		if err != nil {
			t.Fatal(err)
		}
		if first.Mode != controlDomain.ControlModeAccept || len(first.Influencing) != 0 {
			t.Fatalf("the expired point does not act anymore, got %+v", first)
		}
		second, err := effective(operator2, controlDomain.ScopeKey{Kind: "channel", Ref: "max"})
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%+v", first) != fmt.Sprintf("%+v", second) {
			t.Fatal("the expiry is idempotent by construction — equal re-reads, equal truth")
		}
	})

	// Row 17 — the tenant-kind material equality: the own tenant object
	// installs green; a foreign tenant object is MATERIALLY unselectable
	// (AC-1) — the honest dictionary refusal BEFORE any transaction and
	// without an audit row.
	t.Run("row-17 the tenant scope: own green, foreign unselectable", func(t *testing.T) {
		control, err := set(operator1, "tenant", tenant.String(), controlDomain.ControlModeAccept, "точка уровня тенента — маршрут не ограничен", 0, nil, nil, corr("00000000b30e"))
		if err != nil {
			t.Fatal(err)
		}
		if control.Generation != 1 || control.ScopeRef != tenant.String() {
			t.Fatalf("the own tenant object installs, got %+v", control)
		}
		before := auditTotal()
		var rule controlDomain.RuleError
		if _, err := set(operator2, "tenant", foreignTenant.String(), controlDomain.ControlModePaused, "чужой тенент", 0, nil, nil, corr("00000000b30f")); !errors.As(err, &rule) || rule.Rule != "scope_ref" {
			t.Fatalf("the foreign tenant object is materially unselectable, got %v", err)
		}
		if after := auditTotal(); after != before {
			t.Fatalf("the refused guard writes no audit row, got %d -> %d", before, after)
		}
	})

	// Row 18 — the connector scope against the LIVE registry: the
	// published word resolves green; an unknown word is refused
	// fail-closed with the honest cause and without an audit row — the
	// ref of a foreign or unknown connector never becomes a control
	// point.
	t.Run("row-18 the connector scope: live green, unknown fail-closed", func(t *testing.T) {
		control, err := set(operator1, "connector", "staffy-synthetic-telephony", controlDomain.ControlModePaused, "регламентные работы телефонного поставщика", 0, nil, nil, corr("00000000b310"))
		if err != nil {
			t.Fatal(err)
		}
		if control.Generation != 1 || control.ScopeKind != "connector" {
			t.Fatalf("the published connector word installs, got %+v", control)
		}
		before := auditTotal()
		var rule controlDomain.RuleError
		if _, err := set(operator2, "connector", "staffy-ghost-connector", controlDomain.ControlModePaused, "неизвестный коннектор", 0, nil, nil, corr("00000000b311")); !errors.As(err, &rule) || rule.Rule != "scope_ref" {
			t.Fatalf("the unknown connector word is refused fail-closed, got %v", err)
		}
		if !strings.Contains(rule.Reason, "staffy-ghost-connector") {
			t.Fatalf("the refusal names the unresolvable word, got %s", rule.Reason)
		}
		if after := auditTotal(); after != before {
			t.Fatalf("the fail-closed refusal writes no audit row, got %d -> %d", before, after)
		}
	})

	// Row 19 — the RLS boundary of the live store: a runtime session
	// scoped to the own tenant reads the rows; scoped to the foreign
	// tenant (or scoped to nothing) reads ZERO rows — a control never
	// crosses tenants at the database level (the policies of the
	// migration, transaction-scoped like the store does).
	t.Run("row-19 RLS: the cross-tenant rows are invisible", func(t *testing.T) {
		connection, err := runtimePool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Release()
		readCount := func(tenantConfig string) int {
			t.Helper()
			if _, err := connection.Exec(ctx, "begin"); err != nil {
				t.Fatal(err)
			}
			if _, err := connection.Exec(ctx, "select set_config('staffy.tenant_id', $1, true)", tenantConfig); err != nil {
				t.Fatal(err)
			}
			var n int
			if err := connection.QueryRow(ctx, "select count(*) from control.scoped_controls").Scan(&n); err != nil {
				t.Fatal(err)
			}
			if _, err := connection.Exec(ctx, "commit"); err != nil {
				t.Fatal(err)
			}
			return n
		}
		if n := readCount(tenant.String()); n != 7 {
			t.Fatalf("the own tenant reads its own points (phone, telegram, web_chat, email, max, tenant, connector = 7), got %d", n)
		}
		if n := readCount(foreignTenant.String()); n != 0 {
			t.Fatalf("the foreign tenant reads nothing, got %d", n)
		}
		if n := readCount(""); n != 0 {
			t.Fatalf("an unscoped session reads nothing, got %d", n)
		}
	})

	// Row 20 — the append-only audit truth BY GRANTS: the runtime role
	// (staffy_app) cannot UPDATE or DELETE the audit rows — the grants
	// of the migration refuse it before any RLS consideration; the
	// history is materially immutable for the application.
	t.Run("row-20 the append-only audit: UPDATE and DELETE denied", func(t *testing.T) {
		if _, err := runtimePool.Exec(ctx, "update control.audit set reason='подделка' where tenant_id=$1", tenant); err == nil {
			t.Fatal("the audit UPDATE must be denied for the runtime role")
		}
		if _, err := runtimePool.Exec(ctx, "delete from control.audit where tenant_id=$1", tenant); err == nil {
			t.Fatal("the audit DELETE must be denied for the runtime role")
		}
		if _, err := runtimePool.Exec(ctx, "update control.outbox_events set payload='{}'::jsonb where tenant_id=$1", tenant); err == nil {
			t.Fatal("the outbox UPDATE must be denied for the runtime role")
		}
	})

	// Row 21 — the List history surface: BOTH states stay readable (the
	// removed phone point is history, not absence) in the deterministic
	// order (newest first; the frozen clock writes equal timestamps —
	// the (kind, ref) tie-break answers the deterministic order).
	t.Run("row-21 the list: both states, the deterministic order", func(t *testing.T) {
		points, err := controls.List(ctx, operator1, tenant)
		if err != nil {
			t.Fatal(err)
		}
		if len(points) != 7 {
			t.Fatalf("every point of the tenant stays readable (both states), got %d", len(points))
		}
		var removedSeen bool
		pairs := make([]string, 0, len(points))
		for _, point := range points {
			pairs = append(pairs, point.ScopeKind+":"+point.ScopeRef)
			if point.State == controlDomain.ControlStateRemoved {
				removedSeen = true
			}
		}
		if !removedSeen {
			t.Fatal("the removed point stays readable — the history surface")
		}
		expectedOrder := []string{
			"channel:email", "channel:max", "channel:phone", "channel:telegram", "channel:web_chat",
			"connector:staffy-synthetic-telephony", "tenant:" + tenant.String(),
		}
		if strings.Join(pairs, "|") != strings.Join(expectedOrder, "|") {
			t.Fatalf("the deterministic (kind, ref) order, got %v", pairs)
		}
	})
}

// controlMatrixIDs builds the fixed sequence of the matrix mutations
// (the goroutine-safe generator of the harness hands every UUID exactly
// once — the race row shares ONE sequence across two concurrent Sets).
func controlMatrixIDs(n int) []id.UUID {
	values := make([]id.UUID, 0, n)
	for i := 0; i < n; i++ {
		values = append(values, id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-b4000%07d", i+1)))
	}
	return values
}
