package app

import (
        "context"
        "errors"
        "fmt"
        "strings"
        "testing"
        "time"

        identityAPI "github.com/artemiosu/staffy/internal/identity/api"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
        privacyAPI "github.com/artemiosu/staffy/internal/privacy/api"
        privacyDomain "github.com/artemiosu/staffy/internal/privacy/domain"
        "github.com/artemiosu/staffy/internal/privacy/port"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

// stubIdentity authorizes every action; other membership methods are not
// reachable through the privacy flows under test.
type stubIdentity struct{ identityAPI.MembershipService }

func (stubIdentity) Authorize(context.Context, identityDomain.Principal, id.UUID, identityDomain.Scope) error {
        return nil
}

// anchorFailingRepo serves complete map entries and policies but fails the
// class anchor query, exercising the honest unknown path of the term check.
type anchorFailingRepo struct {
        port.Repository
        entries  []privacyDomain.DataMapEntry
        policies []privacyDomain.RetentionPolicy
        saved    []privacyDomain.DeletionOutcome
}

func (r *anchorFailingRepo) LookupDeletion(context.Context, tenancy.Context, id.UUID, string, string) (port.DeletionRequestRecord, []privacyDomain.DeletionOutcome, privacyDomain.Receipt, bool, error) {
        return port.DeletionRequestRecord{}, nil, privacyDomain.Receipt{}, false, nil
}

func (r *anchorFailingRepo) ListMapEntries(context.Context, tenancy.Context) ([]privacyDomain.DataMapEntry, error) {
        return r.entries, nil
}

func (r *anchorFailingRepo) ListPolicies(context.Context, tenancy.Context) ([]privacyDomain.RetentionPolicy, error) {
        return r.policies, nil
}

func (r *anchorFailingRepo) ListHoldEvents(context.Context, tenancy.Context) ([]privacyDomain.LegalHoldEvent, error) {
        return nil, nil
}

func (r *anchorFailingRepo) ClassReferenceTime(context.Context, tenancy.Context, privacyDomain.ArtifactClass) (*time.Time, error) {
        return nil, errors.New("anchor query failed")
}

func (r *anchorFailingRepo) SaveDeletion(_ context.Context, p port.DeletionRecordParams) (port.DeletionRequestRecord, []privacyDomain.DeletionOutcome, privacyDomain.Receipt, bool, error) {
        r.saved = p.Outcomes
        return p.Request, p.Outcomes, p.Receipt, false, nil
}

type fixedIDs struct{ counter int }

func (f *fixedIDs) New(time.Time) (id.UUID, error) {
        f.counter++
        return id.MustParse(fmt.Sprintf("01994f64-6c00-7000-8000-%012d", f.counter)), nil
}

// TestRunDeletionAnchorFailureIsUnknown verifies the honest unknown outcome
// when the class anchor query fails: the term decision must never silently
// fall back to the policy publication time (review R1 finding).
func TestRunDeletionAnchorFailureIsUnknown(t *testing.T) {
        now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
        class := privacyDomain.ClassBehaviorConfiguration
        entry := privacyDomain.DataMapEntry{
                Class: class, Purpose: "Обработка обращений", LegalBasis: "Договор", Controller: "ООО Ромашка",
                Processor: "Staffy", Recipients: []string{"support"}, SubProcessors: []string{"hosting"},
                Locations: []string{"EU"}, AccessibleRoles: []string{"owner"},
                KnownCopies: []string{"postgres"}, RetentionPolicy: "ret-001",
                DeletionMethod: "purge", ExportMethod: "export",
        }
        policy := privacyDomain.RetentionPolicy{
                Class: class, Purpose: "Хранение", LegalBasis: "Договор", Source: "tenant_decision",
                TerminalEvent: "last_change", Version: 1, CreatedAt: now,
        }
        repo := &anchorFailingRepo{
                entries:  []privacyDomain.DataMapEntry{entry},
                policies: []privacyDomain.RetentionPolicy{policy},
        }
        service := New(repo, stubIdentity{}, nil, fixedClock{now}, &fixedIDs{})

        actor := identityDomain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-000000000911"), CanCreateTenant: true}
        tenant := id.MustParse("01994f64-6c00-7000-8000-000000000912")
        result, err := service.RunDeletion(context.Background(), privacyAPI.DeletionCommand{
                Actor: actor, TenantID: tenant, Classes: []privacyDomain.ArtifactClass{class},
                IdempotencyKey: "r1-anchor-failure", CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000913"),
        })
        if err != nil {
                t.Fatal(err)
        }
        if result.Replayed {
                t.Fatal("first execution must not be a replay")
        }
        if len(repo.saved) != 1 {
                t.Fatalf("saved outcomes = %d, want 1", len(repo.saved))
        }
        outcome := repo.saved[0]
        if outcome.Class != class || outcome.Outcome != privacyDomain.OutcomeUnknown {
                t.Fatalf("outcome = %#v, want unknown for %s", outcome, class)
        }
        if outcome.ReconciliationOwner == "" || outcome.ReconciliationDeadline == nil {
                t.Fatalf("reconciliation missing: %#v", outcome)
        }
        if !outcome.ReconciliationDeadline.Equal(now.Add(72 * time.Hour)) {
                t.Fatalf("reconciliation deadline = %v", outcome.ReconciliationDeadline)
        }
        wantNote := "точку отсчёта"
        if !strings.Contains(outcome.Note, wantNote) {
                t.Fatalf("note %q must mention the anchor failure", outcome.Note)
        }
}
