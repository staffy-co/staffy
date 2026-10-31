package httptransport

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	entitlementAPI "github.com/staffy-co/staffy/internal/entitlement/api"
	entitlementDomain "github.com/staffy-co/staffy/internal/entitlement/domain"
	"github.com/staffy-co/staffy/internal/identity/api"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// fakeEntitlement implements the entitlement service surface for the
// transport tests: it records the commands and answers with honest minimal
// views. The R1 regressions assert what the HTTP boundary claims.
type fakeEntitlement struct {
	composeCalls int
	raiseCalls   int
	composeErr   error
	view         entitlementAPI.EntitlementView
	meter        entitlementAPI.MeterView
	exceptions   entitlementAPI.ExceptionsView
}

func (f *fakeEntitlement) EntitlementView(context.Context, domain.Principal, id.UUID) (entitlementAPI.EntitlementView, error) {
	if f.view.Level == "" {
		f.view.Level = entitlementAPI.LevelFull
		f.view.State = entitlementDomain.StateDraft
	}
	return f.view, nil
}

func (f *fakeEntitlement) MeterView(context.Context, domain.Principal, id.UUID) (entitlementAPI.MeterView, error) {
	if f.meter.Level == "" {
		f.meter.Level = entitlementAPI.LevelFull
		f.meter.DataStatus = entitlementDomain.DataUnknown
	}
	return f.meter, nil
}

func (f *fakeEntitlement) AdmissionView(context.Context, domain.Principal, id.UUID) (entitlementDomain.AdmissionState, error) {
	return entitlementDomain.AdmissionState{Status: entitlementDomain.AdmissionOK}, nil
}

func (f *fakeEntitlement) ExceptionsView(context.Context, domain.Principal, id.UUID) (entitlementAPI.ExceptionsView, error) {
	if f.exceptions.Level == "" {
		f.exceptions.Level = entitlementAPI.LevelFull
	}
	return f.exceptions, nil
}

func (f *fakeEntitlement) ComposeTerms(_ context.Context, c entitlementAPI.TermsCommand) (entitlementDomain.TermsVersion, error) {
	f.composeCalls++
	if f.composeErr != nil {
		return entitlementDomain.TermsVersion{}, f.composeErr
	}
	return entitlementDomain.TermsVersion{TenantID: c.TenantID.String(), Version: 1, Currency: c.Currency, ActorID: c.Actor.ID.String()}, nil
}

func (f *fakeEntitlement) RunStateTransition(context.Context, entitlementAPI.StateCommand) (entitlementDomain.StateEvent, error) {
	return entitlementDomain.StateEvent{}, nil
}

func (f *fakeEntitlement) SetBudget(context.Context, entitlementAPI.BudgetCommand) (entitlementDomain.BudgetVersion, error) {
	return entitlementDomain.BudgetVersion{}, nil
}

func (f *fakeEntitlement) RegisterSource(context.Context, entitlementAPI.SourceCommand) (entitlementDomain.UsageSource, error) {
	return entitlementDomain.UsageSource{}, nil
}

func (f *fakeEntitlement) IngestUsage(context.Context, entitlementAPI.UsageEventCommand) (entitlementDomain.UsageEvent, bool, error) {
	return entitlementDomain.UsageEvent{}, false, nil
}

func (f *fakeEntitlement) RaiseException(context.Context, entitlementAPI.ManualExceptionCommand) (entitlementDomain.UsageException, error) {
	f.raiseCalls++
	return entitlementDomain.UsageException{ExceptionID: testCorrelation.String(), Cause: entitlementDomain.CauseManualEntry}, nil
}

func (f *fakeEntitlement) AcknowledgeException(context.Context, entitlementAPI.AckCommand) error {
	return nil
}

func newEntitlementTestServer(t *testing.T, fake *fakeEntitlement) *Server {
	t.Helper()
	identity := fakeIdentity{create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
		return api.CreateTenantResult{}, nil
	}, workspace: func(id.UUID) (domain.Workspace, error) {
		return domain.Workspace{}, nil
	}}
	server, err := New(identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.BindEntitlement(fake)
	return server
}

func entitlementJSONRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	return request
}

func entitlementFormRequest(method, path string, form url.Values) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.com")
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	return request
}

// R1: a decode failure must never masquerade as an invalid usage event key —
// the error reason names the actual problem (content type, malformed body,
// unknown fields).
func TestEntitlementAPIJSONErrorsAreHonest(t *testing.T) {
	server := newEntitlementTestServer(t, &fakeEntitlement{})
	path := "/v1/tenants/" + testTenantID.String() + "/entitlement/terms"
	cases := []struct {
		name   string
		build  func() *http.Request
		status int
		want   string
	}{
		{"wrong content type", func() *http.Request {
			request := entitlementJSONRequest(http.MethodPost, path, `{}`)
			request.Header.Set("Content-Type", "text/plain")
			return request
		}, http.StatusBadRequest, "Используйте Content-Type application/json."},
		{"malformed json", func() *http.Request {
			return entitlementJSONRequest(http.MethodPost, path, `{"currency":`)
		}, http.StatusBadRequest, "Проверьте JSON-запрос"},
		{"unknown field", func() *http.Request {
			return entitlementJSONRequest(http.MethodPost, path, `{"currency":"RUB","secret_field":"x"}`)
		}, http.StatusBadRequest, "Проверьте JSON-запрос"},
		{"trailing object", func() *http.Request {
			return entitlementJSONRequest(http.MethodPost, path, `{"currency":"RUB"}{"currency":"USD"}`)
		}, http.StatusBadRequest, "ровно один JSON-объект"},
	}
	for _, testCase := range cases {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, testCase.build())
		if response.Code != testCase.status {
			t.Fatalf("%s: status=%d body=%s", testCase.name, response.Code, response.Body.String())
		}
		body := response.Body.String()
		if !strings.Contains(body, testCase.want) {
			t.Fatalf("%s: body must carry the honest reason %q: %s", testCase.name, testCase.want, body)
		}
		if strings.Contains(body, "invalid_event_key") || strings.Contains(body, "Ключ события задан неверно") {
			t.Fatalf("%s: decode failure must not masquerade as an invalid event key: %s", testCase.name, body)
		}
	}
}

// R1: a malformed price is an honest validation error, never a silent drop
// that publishes the version as an open question while the owner believes
// the price was saved.
func TestEntitlementTermsFormMalformedPriceIsHonestError(t *testing.T) {
	fake := &fakeEntitlement{}
	server := newEntitlementTestServer(t, fake)
	path := "/app/tenants/" + testTenantID.String() + "/entitlement/terms"
	form := url.Values{
		"currency":     {"RUB"},
		"launch_price": {"49000руб"},
		"change_note":  {"Предложение №1"},
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, entitlementFormRequest(http.MethodPost, path, form))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var flash *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == flashCookieName {
			flash = cookie
		}
	}
	if flash == nil {
		t.Fatal("the malformed price must travel in the flash cookie")
	}
	message, err := url.QueryUnescape(flash.Value)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "Цена запуска задаётся числом") {
		t.Fatalf("flash message must name the price problem: %q", message)
	}
	if fake.composeCalls != 0 {
		t.Fatalf("a malformed price must not publish a terms version: %d calls", fake.composeCalls)
	}
}

// R1: raising a manual exception redirects with its own honest flag — never
// the usage-ingest flag with the "Usage-событие учтено" notice.
func TestEntitlementExceptionFormRedirectsWithOwnFlag(t *testing.T) {
	fake := &fakeEntitlement{}
	server := newEntitlementTestServer(t, fake)
	path := "/app/tenants/" + testTenantID.String() + "/budget/exception"
	form := url.Values{
		"cause":              {entitlementDomain.CauseManualEntry},
		"message":            {"Наблюдение оператора"},
		"recommended_action": {"Проверить маршрут"},
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, entitlementFormRequest(http.MethodPost, path, form))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	location := response.Header().Get("Location")
	if !strings.Contains(location, "exception_saved=1") {
		t.Fatalf("the redirect must carry the exception flag: %s", location)
	}
	if strings.Contains(location, "ingested=1") {
		t.Fatalf("the redirect must not claim a usage ingest: %s", location)
	}
	if fake.raiseCalls != 1 {
		t.Fatalf("the exception command must reach the service: %d calls", fake.raiseCalls)
	}

	// The budget page announces the recorded exception honestly.
	page := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/budget?exception_saved=1", nil)
	pageResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(pageResponse, page)
	if pageResponse.Code != http.StatusOK {
		t.Fatalf("budget page status=%d body=%s", pageResponse.Code, pageResponse.Body.String())
	}
	if !strings.Contains(pageResponse.Body.String(), "Операционное исключение зафиксировано") {
		t.Fatalf("the notice must name the recorded exception: %s", pageResponse.Body.String())
	}
}
