//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/app"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/identity/store"
	"github.com/artemiosu/staffy/internal/platform/id"
)

func TestMembershipInvitationAcceptanceAndLastOwner(t *testing.T) {
	ctx := context.Background()
	service, owner := newService("000000000701", "000000000702", "000000000703")
	tenant := createTenant(t, service, owner, "Membership tenant", "membership-create-01", "000000000704").Tenant.ID
	membershipNow := time.Now().UTC().Truncate(time.Microsecond)
	service = app.New(store.NewPostgres(runtimePool), fixedClock{membershipNow}, &sequenceIDs{})
	corr := id.MustParse("01994f64-6c00-7000-8000-000000000705")
	owner.StepUpVerified = true
	command := api.MembershipCommand{Actor: owner, TenantID: tenant, Email: "colleague@example.test", Roles: []domain.Role{domain.Operator}, Scopes: []domain.Scope{domain.Operations}, CorrelationID: corr}
	invitation, oldToken, err := service.InviteMember(ctx, command)
	if err != nil || invitation.Status != "pending" {
		t.Fatalf("invite = %#v, %v", invitation, err)
	}
	_, newToken, err := service.InviteMember(ctx, command)
	if err != nil || oldToken == newToken {
		t.Fatalf("resend = %v", err)
	}
	member := domain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-000000000706"), VerifiedEmail: "colleague@example.test"}
	if _, err := service.AcceptInvitation(ctx, member, oldToken, corr); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("old token = %v", err)
	}
	wrong := member
	wrong.VerifiedEmail = "other@example.test"
	if _, err := service.AcceptInvitation(ctx, wrong, newToken, corr); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("wrong identity = %v", err)
	}
	active, err := service.AcceptInvitation(ctx, member, newToken, corr)
	if err != nil || active.Status != "active" {
		t.Fatalf("accept = %#v, %v", active, err)
	}
	if _, err := service.AcceptInvitation(ctx, member, newToken, corr); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("replay = %v", err)
	}
	if err := service.Authorize(ctx, member, tenant, domain.Operations); err != nil {
		t.Fatalf("operations denied: %v", err)
	}
	if err := service.Authorize(ctx, member, tenant, domain.Audit); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("audit = %v", err)
	}
	if _, err := service.ListMembers(ctx, member, tenant); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("member listing = %v", err)
	}
	if _, err := service.ChangeMember(ctx, api.MembershipCommand{Actor: owner, TenantID: tenant, TargetID: member.ID, Roles: []domain.Role{domain.Configurator}, Scopes: []domain.Scope{domain.Configuration}, ExpectedVersion: 9, Reason: domain.ReasonAccessAdjustment, CorrelationID: corr}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale version = %v", err)
	}
	changed, err := service.ChangeMember(ctx, api.MembershipCommand{Actor: owner, TenantID: tenant, TargetID: member.ID, Roles: []domain.Role{domain.Configurator}, Scopes: []domain.Scope{domain.Configuration}, ExpectedVersion: 1, Reason: domain.ReasonAccessAdjustment, CorrelationID: corr})
	if err != nil || changed.Version != 2 {
		t.Fatalf("change = %#v, %v", changed, err)
	}
	if err := service.Authorize(ctx, member, tenant, domain.Operations); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("old permission = %v", err)
	}
	if err := service.RevokeMember(ctx, api.MembershipCommand{Actor: owner, TenantID: tenant, TargetID: owner.ID, ExpectedVersion: 1, Reason: domain.ReasonOffboarding, CorrelationID: corr}); !errors.Is(err, domain.ErrLastOwner) {
		t.Fatalf("last owner = %v", err)
	}
	if err := service.RevokeMember(ctx, api.MembershipCommand{Actor: owner, TenantID: tenant, TargetID: member.ID, ExpectedVersion: 2, Reason: domain.ReasonOffboarding, CorrelationID: corr}); err != nil {
		t.Fatalf("revoke = %v", err)
	}
	if err := service.Authorize(ctx, member, tenant, domain.Configuration); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("revoked permission = %v", err)
	}
	var privateDigestColumns, unsafeReasons int
	if err := adminPool.QueryRow(ctx, `select count(*) from information_schema.columns where table_schema='identity' and table_name='membership_audit' and column_name='subject_email_digest'`).Scan(&privateDigestColumns); err != nil {
		t.Fatal(err)
	}
	if err := adminPool.QueryRow(ctx, `select count(*) from identity.membership_audit where tenant_id=$1 and reason not in ('invitation_created','invitation_accepted','access_adjustment','offboarding','security_response')`, tenant.String()).Scan(&unsafeReasons); err != nil {
		t.Fatal(err)
	}
	if privateDigestColumns != 0 || unsafeReasons != 0 {
		t.Fatalf("unsafe audit schema/data: digest columns=%d reasons=%d", privateDigestColumns, unsafeReasons)
	}
	rows, err := adminPool.Query(ctx, `select actor_id::text, subject_id::text, old_state, new_state, reason, correlation_id::text, occurred_at from identity.membership_audit where tenant_id=$1`, tenant.String())
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var actorID, reason, correlationID string
		var subjectID *string
		var oldRaw, nextRaw []byte
		var occurred time.Time
		if err := rows.Scan(&actorID, &subjectID, &oldRaw, &nextRaw, &reason, &correlationID, &occurred); err != nil {
			t.Fatal(err)
		}
		var oldState, nextState map[string]any
		if err := json.Unmarshal(oldRaw, &oldState); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(nextRaw, &nextState); err != nil {
			t.Fatal(err)
		}
		if correlationID != corr.String() || !occurred.Equal(membershipNow) {
			t.Fatalf("audit metadata: correlation=%s time=%s", correlationID, occurred)
		}
		if reason == string(domain.ReasonInvitationCreated) {
			if actorID != owner.ID.String() || subjectID != nil || nextState["status"] != "pending" || nextState["invitation_id"] == nil {
				t.Fatalf("invitation audit: actor=%s subject=%v old=%v next=%v", actorID, subjectID, oldState, nextState)
			}
			if nextState["version"] == float64(1) && oldState != nil {
				t.Fatalf("first invitation old state=%v", oldState)
			}
			if nextState["version"] == float64(2) && (oldState["status"] != "pending" || oldState["invitation_id"] != nextState["invitation_id"]) {
				t.Fatalf("resend transition: old=%v next=%v", oldState, nextState)
			}
		} else {
			if subjectID == nil || *subjectID != member.ID.String() {
				t.Fatalf("audit subject=%v", subjectID)
			}
			switch reason {
			case string(domain.ReasonInvitationAccepted):
				if actorID != member.ID.String() || oldState["status"] != "pending" || nextState["status"] != "active" {
					t.Fatalf("accept audit: actor=%s old=%v next=%v", actorID, oldState, nextState)
				}
			case string(domain.ReasonAccessAdjustment):
				if actorID != owner.ID.String() || oldState["status"] != "active" || nextState["status"] != "active" || oldState["version"] != float64(1) || nextState["version"] != float64(2) {
					t.Fatalf("change audit: actor=%s old=%v next=%v", actorID, oldState, nextState)
				}
			case string(domain.ReasonOffboarding):
				if actorID != owner.ID.String() || oldState["status"] != "active" || nextState["status"] != "revoked" || oldState["version"] != float64(2) || nextState["version"] != float64(3) {
					t.Fatalf("revoke audit: actor=%s old=%v next=%v", actorID, oldState, nextState)
				}
			default:
				t.Fatalf("unexpected audit reason %s", reason)
			}
		}
		counts[reason]++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if counts[string(domain.ReasonInvitationCreated)] != 2 || counts[string(domain.ReasonInvitationAccepted)] != 1 || counts[string(domain.ReasonAccessAdjustment)] != 1 || counts[string(domain.ReasonOffboarding)] != 1 {
		t.Fatalf("audit transitions=%v", counts)
	}
}

func TestExpiredInvitationNeverActivates(t *testing.T) {
	ctx := context.Background()
	service, owner := newService("000000000721", "000000000722", "000000000723")
	tenant := createTenant(t, service, owner, "Expiring tenant", "membership-create-03", "000000000724").Tenant.ID
	service = app.New(store.NewPostgres(runtimePool), fixedClock{time.Now().UTC()}, &sequenceIDs{})
	correlation := id.MustParse("01994f64-6c00-7000-8000-000000000725")
	_, token, err := service.InviteMember(ctx, api.MembershipCommand{Actor: owner, TenantID: tenant, Email: "expired@example.test", Roles: []domain.Role{domain.Operator}, Scopes: []domain.Scope{domain.Operations}, CorrelationID: correlation})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adminPool.Exec(ctx, `update identity.invitations set expires_at=clock_timestamp()-interval '1 second' where tenant_id=$1`, tenant.String()); err != nil {
		t.Fatal(err)
	}
	later := app.New(store.NewPostgres(runtimePool), fixedClock{time.Now().UTC()}, &sequenceIDs{})
	invitee := domain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-000000000726"), VerifiedEmail: "expired@example.test"}
	if _, err := later.AcceptInvitation(ctx, invitee, token, correlation); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expired acceptance = %v", err)
	}
	if _, err := service.Workspace(ctx, invitee, tenant); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expired membership = %v", err)
	}
}

func TestIntegrationAdminCannotConfigure(t *testing.T) {
	ctx := context.Background()
	service, owner := newService("000000000731", "000000000732", "000000000733")
	tenant := createTenant(t, service, owner, "Integration tenant", "membership-create-04", "000000000734").Tenant.ID
	service = app.New(store.NewPostgres(runtimePool), fixedClock{time.Now().UTC()}, &sequenceIDs{})
	correlation := id.MustParse("01994f64-6c00-7000-8000-000000000735")
	command := api.MembershipCommand{Actor: owner, TenantID: tenant, Email: "integrator@example.test", Roles: []domain.Role{domain.IntegrationAdmin}, Scopes: []domain.Scope{domain.Integrations}, CorrelationID: correlation}
	if _, _, err := service.InviteMember(ctx, command); !errors.Is(err, domain.ErrStepUpRequired) {
		t.Fatalf("missing step-up = %v", err)
	}
	owner.StepUpVerified = true
	command.Actor = owner
	_, token, err := service.InviteMember(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	member := domain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-000000000736"), VerifiedEmail: "integrator@example.test"}
	if _, err = service.AcceptInvitation(ctx, member, token, correlation); err != nil {
		t.Fatal(err)
	}
	if err = service.Authorize(ctx, member, tenant, domain.Integrations); err != nil {
		t.Fatalf("integrations denied: %v", err)
	}
	if err = service.Authorize(ctx, member, tenant, domain.Configuration); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("configuration access = %v", err)
	}
}

func TestRevokedMemberRejoinsOnlyThroughFreshInvitation(t *testing.T) {
	ctx := context.Background()
	service, owner := newService("000000000741", "000000000742", "000000000743")
	tenant := createTenant(t, service, owner, "Rejoin tenant", "membership-create-05", "000000000744").Tenant.ID
	service = app.New(store.NewPostgres(runtimePool), fixedClock{time.Now().UTC()}, &sequenceIDs{})
	correlation := id.MustParse("01994f64-6c00-7000-8000-000000000745")
	owner.StepUpVerified = true
	invite := api.MembershipCommand{Actor: owner, TenantID: tenant, Email: "rejoin@example.test", Roles: []domain.Role{domain.Operator}, Scopes: []domain.Scope{domain.Operations}, CorrelationID: correlation}
	_, oldToken, err := service.InviteMember(ctx, invite)
	if err != nil {
		t.Fatal(err)
	}
	member := domain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-000000000746"), VerifiedEmail: "rejoin@example.test"}
	if _, err = service.AcceptInvitation(ctx, member, oldToken, correlation); err != nil {
		t.Fatal(err)
	}
	if err = service.RevokeMember(ctx, api.MembershipCommand{Actor: owner, TenantID: tenant, TargetID: member.ID, ExpectedVersion: 1, Reason: domain.ReasonOffboarding, CorrelationID: correlation}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AcceptInvitation(ctx, member, oldToken, correlation); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("old token replay=%v", err)
	}
	invite.Roles = []domain.Role{domain.Configurator}
	invite.Scopes = []domain.Scope{domain.Configuration}
	_, newToken, err := service.InviteMember(ctx, invite)
	if err != nil {
		t.Fatal(err)
	}
	active, err := service.AcceptInvitation(ctx, member, newToken, correlation)
	if err != nil {
		t.Fatal(err)
	}
	if active.Version != 3 || active.Status != "active" {
		t.Fatalf("reactivated membership=%#v", active)
	}
	if err = service.Authorize(ctx, member, tenant, domain.Configuration); err != nil {
		t.Fatalf("new grant denied: %v", err)
	}
	if err = service.Authorize(ctx, member, tenant, domain.Operations); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("old grant persisted: %v", err)
	}
}

func TestConcurrentOwnerRevocationLeavesOneOwner(t *testing.T) {
	ctx := context.Background()
	service, first := newService("000000000711", "000000000712", "000000000713")
	tenant := createTenant(t, service, first, "Two owners", "membership-create-02", "000000000714").Tenant.ID
	service = app.New(store.NewPostgres(runtimePool), fixedClock{time.Now().UTC()}, &sequenceIDs{})
	correlation := id.MustParse("01994f64-6c00-7000-8000-000000000715")
	first.StepUpVerified = true
	_, token, err := service.InviteMember(ctx, api.MembershipCommand{Actor: first, TenantID: tenant, Email: "owner2@example.test", Roles: []domain.Role{domain.Owner}, Scopes: domain.AllScopes(), CorrelationID: correlation})
	if err != nil {
		t.Fatal(err)
	}
	second := domain.Principal{ID: id.MustParse("01994f64-6c00-7000-8000-000000000716"), VerifiedEmail: "owner2@example.test", StepUpVerified: true}
	if _, err := service.AcceptInvitation(ctx, second, token, correlation); err != nil {
		t.Fatal(err)
	}
	commands := []api.MembershipCommand{
		{Actor: first, TenantID: tenant, TargetID: second.ID, ExpectedVersion: 1, Reason: domain.ReasonOffboarding, CorrelationID: correlation},
		{Actor: second, TenantID: tenant, TargetID: first.ID, ExpectedVersion: 1, Reason: domain.ReasonOffboarding, CorrelationID: correlation},
	}
	results := make([]error, 2)
	var wg sync.WaitGroup
	for i := range commands {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i] = service.RevokeMember(ctx, commands[i]) }(i)
	}
	wg.Wait()
	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, domain.ErrLastOwner) && !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful revocations = %d; errors = %v", successes, results)
	}
	var remaining int
	if err := adminPool.QueryRow(ctx, `select count(*) from identity.memberships where tenant_id=$1 and status='active' and 'owner'=any(roles)`, tenant.String()).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("remaining owners = %d", remaining)
	}
}
