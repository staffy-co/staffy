package httptransport

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/identity/api"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

type fakeMembers struct {
	fakeIdentity
	list   func(domain.Principal, id.UUID) ([]domain.Membership, error)
	invite func(api.MembershipCommand) (domain.Membership, string, error)
	accept func(domain.Principal, string, id.UUID) (domain.Membership, error)
	change func(api.MembershipCommand) (domain.Membership, error)
	revoke func(api.MembershipCommand) error
}

func TestMemberAPIAndFormsMapSuccessfulCommands(t *testing.T) {
	target := id.MustParse("01994f64-6c00-7000-8000-000000000007")
	principal := domain.Principal{ID: testPrincipalID, VerifiedEmail: "member@example.test", StepUpVerified: true}
	member := domain.Membership{TenantID: testTenantID, PrincipalID: &target, Roles: []domain.Role{domain.Operator}, Scopes: []domain.Scope{domain.Operations}, Status: "active", Version: 2}
	accepts, changes, revokes := 0, 0, 0
	identity := fakeMembers{
		list:   func(domain.Principal, id.UUID) ([]domain.Membership, error) { return []domain.Membership{member}, nil },
		invite: func(api.MembershipCommand) (domain.Membership, string, error) { return member, "new-token", nil },
		accept: func(p domain.Principal, token string, corr id.UUID) (domain.Membership, error) {
			accepts++
			if p.ID != principal.ID || token != "secret-token" || corr != testCorrelation {
				t.Fatalf("accept mapping: %v %s %v", p, token, corr)
			}
			return member, nil
		},
		change: func(c api.MembershipCommand) (domain.Membership, error) {
			changes++
			if c.Actor.ID != principal.ID || c.TenantID != testTenantID || c.TargetID != target || c.ExpectedVersion != 2 || c.Reason != domain.ReasonAccessAdjustment || len(c.Roles) != 1 || c.Roles[0] != domain.Configurator || len(c.Scopes) != 1 || c.Scopes[0] != domain.Configuration || c.CorrelationID != testCorrelation {
				t.Fatalf("change mapping: %#v", c)
			}
			return member, nil
		},
		revoke: func(c api.MembershipCommand) error {
			revokes++
			if c.Actor.ID != principal.ID || c.TenantID != testTenantID || c.TargetID != target || c.ExpectedVersion != 2 || c.Reason != domain.ReasonOffboarding || c.CorrelationID != testCorrelation {
				t.Fatalf("revoke mapping: %#v", c)
			}
			return nil
		},
	}
	handler := newTestServer(t, identity, fixedResolver{principal: principal}, &bytes.Buffer{})
	request := httptest.NewRequest(http.MethodPost, "/v1/invitations/accept", strings.NewReader(`{"token":"secret-token"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"status":"active"`) {
		t.Fatalf("accept API: %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodPatch, "/v1/tenants/"+testTenantID.String()+"/members/"+target.String(), strings.NewReader(`{"roles":["configurator"],"scopes":["configuration"],"version":2,"reason":"access_adjustment"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("change API: %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodDelete, "/v1/tenants/"+testTenantID.String()+"/members/"+target.String(), strings.NewReader(`{"version":2,"reason":"offboarding"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 204 {
		t.Fatalf("revoke API: %d %s", response.Code, response.Body.String())
	}
	acceptForm := url.Values{"token": {"secret-token"}}
	request = memberFormRequest(http.MethodPost, "/app/invitations/accept", acceptForm)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/app/tenants/"+testTenantID.String() {
		t.Fatalf("accept form: %d %s", response.Code, response.Body.String())
	}
	changeForm := url.Values{"principal_id": {target.String()}, "version": {"2"}, "roles": {"configurator"}, "scopes": {"configuration"}, "reason": {"access_adjustment"}}
	request = memberFormRequest(http.MethodPost, "/app/tenants/"+testTenantID.String()+"/people/change", changeForm)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "Доступ участника обновлён") {
		t.Fatalf("change form: %d %s", response.Code, response.Body.String())
	}
	revokeForm := url.Values{"principal_id": {target.String()}, "version": {"2"}, "reason": {"offboarding"}, "confirm_revoke": {"yes"}}
	request = memberFormRequest(http.MethodPost, "/app/tenants/"+testTenantID.String()+"/people/revoke", revokeForm)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("revoke form: %d %s", response.Code, response.Body.String())
	}
	if accepts != 2 || changes != 2 || revokes != 2 {
		t.Fatalf("mapped calls: accept=%d change=%d revoke=%d", accepts, changes, revokes)
	}
}

func TestPeopleFormsRequireMatchingOriginAndRevokeConfirmation(t *testing.T) {
	target := id.MustParse("01994f64-6c00-7000-8000-000000000008")
	called := 0
	member := domain.Membership{TenantID: testTenantID, PrincipalID: &target, Roles: []domain.Role{domain.Operator}, Scopes: []domain.Scope{domain.Operations}, Status: "active", Version: 2}
	identity := fakeMembers{list: func(domain.Principal, id.UUID) ([]domain.Membership, error) { return []domain.Membership{member}, nil }, invite: func(api.MembershipCommand) (domain.Membership, string, error) { called++; return member, "token", nil }, accept: func(domain.Principal, string, id.UUID) (domain.Membership, error) { called++; return member, nil }, change: func(api.MembershipCommand) (domain.Membership, error) { called++; return member, nil }, revoke: func(api.MembershipCommand) error { called++; return nil }}
	handler := newTestServer(t, identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID}}, &bytes.Buffer{})
	paths := []struct {
		path string
		form url.Values
	}{
		{"/app/tenants/" + testTenantID.String() + "/people", url.Values{"email": {"person@example.test"}}},
		{"/app/invitations/accept", url.Values{"token": {"secret-token"}}},
		{"/app/tenants/" + testTenantID.String() + "/people/change", url.Values{"principal_id": {target.String()}, "version": {"2"}, "reason": {"access_adjustment"}}},
		{"/app/tenants/" + testTenantID.String() + "/people/revoke", url.Values{"principal_id": {target.String()}, "version": {"2"}, "reason": {"offboarding"}, "confirm_revoke": {"yes"}}},
	}
	for _, tt := range paths {
		for _, origin := range []string{"", "https://other.example"} {
			request := memberFormRequest(http.MethodPost, tt.path, tt.form)
			request.Header.Set("Origin", origin)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Errorf("%s origin %q: status %d", tt.path, origin, response.Code)
			}
		}
	}
	if called != 0 {
		t.Fatalf("mutation called without valid origin: %d", called)
	}
	request := memberFormRequest(http.MethodPost, "/app/tenants/"+testTenantID.String()+"/people/revoke", url.Values{"principal_id": {target.String()}, "version": {"2"}, "reason": {"offboarding"}})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || called != 0 {
		t.Fatalf("revoke without confirmation: status=%d calls=%d", response.Code, called)
	}
	request = httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/people", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `value="operator" checked`) || !strings.Contains(response.Body.String(), `value="operations" checked`) || strings.Contains(response.Body.String(), `value="configurator" checked`) || !strings.Contains(response.Body.String(), `name="confirm_revoke"`) {
		t.Fatalf("editor grant/confirmation missing: %d %s", response.Code, response.Body.String())
	}
}

func memberFormRequest(method, path string, form url.Values) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.com")
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	return request
}

func (f fakeMembers) ListMembers(_ context.Context, p domain.Principal, t id.UUID) ([]domain.Membership, error) {
	return f.list(p, t)
}
func (f fakeMembers) InviteMember(_ context.Context, c api.MembershipCommand) (domain.Membership, string, error) {
	return f.invite(c)
}
func (f fakeMembers) AcceptInvitation(_ context.Context, principal domain.Principal, token string, correlation id.UUID) (domain.Membership, error) {
	if f.accept != nil {
		return f.accept(principal, token, correlation)
	}
	return domain.Membership{}, domain.ErrNotFound
}
func (f fakeMembers) ChangeMember(_ context.Context, command api.MembershipCommand) (domain.Membership, error) {
	if f.change != nil {
		return f.change(command)
	}
	return domain.Membership{}, domain.ErrStepUpRequired
}
func (f fakeMembers) RevokeMember(_ context.Context, command api.MembershipCommand) error {
	if f.revoke != nil {
		return f.revoke(command)
	}
	return domain.ErrStepUpRequired
}
func (fakeMembers) Authorize(context.Context, domain.Principal, id.UUID, domain.Scope) error {
	return domain.ErrForbidden
}

func TestMemberAPIScopesAndSafeDenial(t *testing.T) {
	called := false
	identity := fakeMembers{list: func(p domain.Principal, tenant id.UUID) ([]domain.Membership, error) {
		if tenant != testTenantID || p.ID != testPrincipalID {
			t.Fatal("untrusted tenant or actor")
		}
		return nil, domain.ErrNotFound
	}, invite: func(c api.MembershipCommand) (domain.Membership, string, error) {
		called = true
		if c.TenantID != testTenantID || c.Actor.ID != testPrincipalID || c.CorrelationID != testCorrelation {
			t.Fatalf("command = %#v", c)
		}
		return domain.Membership{}, "", domain.ErrStepUpRequired
	}}
	principal := domain.Principal{ID: testPrincipalID}
	var logs bytes.Buffer
	handler := newTestServer(t, identity, fixedResolver{principal: principal}, &logs)
	request := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/members", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), testTenantID.String()) {
		t.Fatalf("foreign list response = %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/v1/tenants/"+testTenantID.String()+"/invitations", strings.NewReader(`{"email":"person@example.test","roles":["owner"],"scopes":["configuration","integrations","operations","review","technical_expertise","claims","audit"]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	request.Header.Set("X-Tenant-ID", "01994f64-6c00-7000-8000-00000000dead")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !called || response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "step_up_required") {
		t.Fatalf("invite response = %d %s", response.Code, response.Body.String())
	}
	logText := logs.String()
	if !strings.Contains(logText, `"actor_id":"`+testPrincipalID.String()+`"`) || !strings.Contains(logText, `"correlation_id":"`+testCorrelation.String()+`"`) || !strings.Contains(logText, `"code":"step_up_required"`) {
		t.Fatalf("denial audit incomplete: %s", logText)
	}
	if strings.Contains(logText, "person@example.test") || strings.Contains(logText, "integrations") {
		t.Fatalf("denial audit leaked request data: %s", logText)
	}
}
