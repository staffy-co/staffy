package httptransport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

type fakeEmployee struct {
	name       string
	calls      int
	saveErr    error
	readErr    error
	previewCmd *employeeAPI.PinCommand
}

func (f *fakeEmployee) Create(_ context.Context, c employeeAPI.CreateCommand) (employeeAPI.CreateResult, error) {
	f.calls++
	f.name = c.Name
	return employeeAPI.CreateResult{Draft: employeeDomain.Draft{TenantID: c.TenantID, EmployeeID: testCorrelation, OwnerID: c.Actor.ID, Status: "draft", Readiness: "incomplete", Version: 1, CurrentStep: "brief"}}, nil
}
func (f *fakeEmployee) List(context.Context, identityDomain.Principal, id.UUID) ([]employeeDomain.Draft, error) {
	return nil, nil
}
func (f *fakeEmployee) Read(context.Context, identityDomain.Principal, id.UUID, id.UUID) (employeeDomain.Draft, error) {
	if f.readErr != nil {
		return employeeDomain.Draft{}, f.readErr
	}
	return employeeDomain.Draft{}, employeeDomain.ErrNotFound
}
func (f *fakeEmployee) Save(context.Context, employeeAPI.SaveCommand) (employeeDomain.Draft, error) {
	if f.saveErr != nil {
		return employeeDomain.Draft{}, f.saveErr
	}
	return employeeDomain.Draft{}, employeeDomain.ErrConflict
}
func (f *fakeEmployee) Catalog(context.Context, identityDomain.Principal, id.UUID) ([]employeeDomain.PackManifest, error) {
	return nil, nil
}
func (f *fakeEmployee) Preview(_ context.Context, c employeeAPI.PinCommand) (employeeDomain.PackPreview, error) {
	f.previewCmd = &c
	return employeeDomain.PackPreview{Current: employeeDomain.PackPin{ID: "auto-parts", Version: "1.0.0"}, Target: employeeDomain.PackManifest{ID: "auto-parts", Version: "2.0.0"}, Impact: employeeDomain.PackImpact{Compatibility: "incompatible", Changed: []string{"schema"}, ReviewSlots: []string{"knowledge"}}}, nil
}
func (f *fakeEmployee) Pin(context.Context, employeeAPI.PinCommand) (employeeDomain.Draft, error) {
	return employeeDomain.Draft{}, employeeDomain.ErrInvalid
}

func TestDomainPackPreviewRouteReturnsImpact(t *testing.T) {
	identity := fakeIdentity{create: func(identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
		return identityAPI.CreateTenantResult{}, nil
	}, workspace: func(id.UUID) (identityDomain.Workspace, error) {
		return identityDomain.Workspace{}, identityDomain.ErrNotFound
	}}
	server, err := New(identity, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeEmployee{}
	server.BindEmployee(fake)
	path := "/v1/tenants/" + testTenantID.String() + "/employees/" + testCorrelation.String() + "/domain-pack/preview"
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"version":2,"pack_id":"auto-parts","pack_version":"2.0.0","variants":{"parts_interest":"both","matching_depth":2}}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"compatibility":"incompatible"`) || !strings.Contains(w.Body.String(), `"review_slots":["knowledge"]`) {
		t.Fatalf("preview response=%d %s", w.Code, w.Body.String())
	}
	if fake.previewCmd == nil || fake.previewCmd.TenantID != testTenantID || fake.previewCmd.EmployeeID != testCorrelation || fake.previewCmd.Actor.ID != testPrincipalID || fake.previewCmd.ExpectedVersion != 2 || fake.previewCmd.PackVersion != "2.0.0" {
		t.Fatalf("preview command=%+v", fake.previewCmd)
	}
}

func TestEmployeeAPIRejectsAuthorityPayloadAndExplainsConflict(t *testing.T) {
	identity := fakeIdentity{create: func(_ identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
		return identityAPI.CreateTenantResult{}, nil
	}, workspace: func(_ id.UUID) (identityDomain.Workspace, error) {
		return identityDomain.Workspace{}, identityDomain.ErrNotFound
	}}
	server, err := New(identity, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeEmployee{}
	server.BindEmployee(fake)
	base := "/v1/tenants/" + testTenantID.String() + "/employees"
	for _, tc := range []struct {
		body   string
		status int
	}{{`{"name":"Аня","role":"owner"}`, 400}, {`{"name":"Аня"}`, 201}} {
		r := httptest.NewRequest(http.MethodPost, base, strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", "test-create-key")
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
		}
	}
	if fake.calls != 1 || fake.name != "Аня" {
		t.Fatalf("accepted authority payload or lost name: %+v", fake)
	}
	r := httptest.NewRequest(http.MethodPatch, base+"/"+testCorrelation.String()+"/brief", strings.NewReader(`{"version":1,"brief":{"purpose":"Помощь","name":"Аня","languages":["ru"],"time_zone":"Europe/Moscow","working_hours":{"days":["mon"],"start":"09:00","end":"18:00"},"desired_channels":["phone"],"primary_tasks":["Запись"]}}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "version_conflict") || !strings.Contains(w.Body.String(), "Обновите страницу") {
		t.Fatalf("conflict response=%d %s", w.Code, w.Body.String())
	}
}

func TestEmployeeFormErrorsUseSafeStatusAndFailedRereadStopsComparison(t *testing.T) {
	identity := fakeIdentity{create: func(_ identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
		return identityAPI.CreateTenantResult{}, nil
	}, workspace: func(_ id.UUID) (identityDomain.Workspace, error) {
		return identityDomain.Workspace{}, identityDomain.ErrNotFound
	}}
	for _, tc := range []struct {
		name             string
		saveErr, readErr error
		status           int
	}{{"missing", employeeDomain.ErrNotFound, nil, 404}, {"database", errors.New("database unavailable"), nil, 500}, {"reread", employeeDomain.ErrConflict, errors.New("database unavailable"), 500}} {
		t.Run(tc.name, func(t *testing.T) {
			server, err := New(identity, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
			if err != nil {
				t.Fatal(err)
			}
			server.BindEmployee(&fakeEmployee{saveErr: tc.saveErr, readErr: tc.readErr})
			form := url.Values{"version": {"1"}, "purpose": {"Помощь"}, "name": {"Аня"}, "languages": {"ru"}, "time_zone": {"Europe/Moscow"}, "days": {"mon"}, "start": {"09:00"}, "end": {"18:00"}, "primary_tasks": {"Запись"}}
			r := httptest.NewRequest(http.MethodPost, "http://example.com/app/tenants/"+testTenantID.String()+"/employees/"+testCorrelation.String()+"/brief", strings.NewReader(form.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Origin", "http://example.com")
			w := httptest.NewRecorder()
			server.Handler().ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Header().Get("Content-Type"), "application/problem+json") || strings.Contains(w.Body.String(), "database unavailable") || strings.Contains(w.Body.String(), "Сейчас сохранено") {
				t.Fatalf("status=%d type=%q body=%s", w.Code, w.Header().Get("Content-Type"), w.Body.String())
			}
		})
	}
}
