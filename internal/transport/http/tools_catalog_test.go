package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/identity/api"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/tools"
)

// fakeToolsCatalog is the transport fake of the read-only tool catalog
// surface: it serves the fixture contracts and the honest refusals of
// the read gate (the fake never touches the embedded registry — the
// loader's unit truth covers it).
type fakeToolsCatalog struct {
	contracts []tools.ToolContract
	listErr   error
	getErr    error
	lastGet   *toolGetCall
}

// The compile-time proof the fake satisfies the transport contract.
var _ toolsCatalogService = (*fakeToolsCatalog)(nil)

type toolGetCall struct {
	tenantID id.UUID
	toolID   string
	version  string
}

func (f *fakeToolsCatalog) List(context.Context, domain.Principal, id.UUID) ([]tools.ToolContract, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]tools.ToolContract{}, f.contracts...), nil
}

func (f *fakeToolsCatalog) Get(_ context.Context, _ domain.Principal, tenantID id.UUID, toolID, version string) (tools.ToolContract, error) {
	f.lastGet = &toolGetCall{tenantID: tenantID, toolID: toolID, version: version}
	if f.getErr != nil {
		return tools.ToolContract{}, f.getErr
	}
	for _, contract := range f.contracts {
		if contract.ID == toolID && contract.Version == version {
			return contract, nil
		}
	}
	return tools.ToolContract{}, tools.ErrNotFound
}

// toolScopeWords mirrors the service wrap of the honest 403: the words of
// the two scopes that DO open the surface, attached to the module error
// at the refusal point.
const toolScopeWords = "Чтение реестра инструментов требует полномочия integrations или configuration: словарь типизированных контрактов — материал настройки интеграций и конфигурации, оператору и аудиту он не открывается до вычисления доступности."

// toolFixtureContract builds the fixture of the slice: the fitment
// evidence contract with the catalog dependency and the honest
// not_implemented adapter state.
func toolFixtureContract(contractID, version string) tools.ToolContract {
	return tools.ToolContract{
		ID: contractID, Version: version,
		Name:             "Доказательство применимости",
		Purpose:          "Проверяет применимость детали к автомобилю по свежему доказательству из каталога",
		DeterminismClass: "deterministic_read",
		AdapterStatus:    "not_implemented",
		InputSchema: map[string]tools.ToolSchemaField{
			"part.oem_number": {Label: "OEM-номер", Type: "string"},
		},
		OutputSchema: map[string]tools.ToolSchemaField{
			"fitment.link_type": {Label: "Тип связи", Type: "string", Values: []string{"original", "cross"}},
		},
		Owner:       "auto-parts",
		EvidenceRef: "auto-parts/spec/4.4",
		Dependency:  &tools.ToolDependency{ConnectorType: "catalog"},
	}
}

func newToolsTestServer(t *testing.T, fake *fakeToolsCatalog, resolver fixedResolver) *Server {
	t.Helper()
	identity := fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
			return api.CreateTenantResult{}, nil
		},
		workspace: func(id.UUID) (domain.Workspace, error) {
			return domain.Workspace{}, nil
		},
	}
	server, err := New(identity, resolver, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if fake != nil {
		server.BindToolsCatalog(fake)
	}
	return server
}

func toolTenantID(t *testing.T) id.UUID {
	t.Helper()
	tenant, err := id.Parse("01994f64-6c00-7000-8000-0000000000a1")
	if err != nil {
		t.Fatalf("parse tenant: %v", err)
	}
	return tenant
}

// TestToolsCatalogListAPI: the machine list answers the newest versions
// with the full contract view — the dictionaries, the dependency, the
// schemas in the stable key order, the empty arrays (never null).
func TestToolsCatalogListAPI(t *testing.T) {
	resolver := fixedResolver{principal: domain.Principal{ID: testPrincipalID}}
	fake := &fakeToolsCatalog{contracts: []tools.ToolContract{toolFixtureContract("auto-parts.fitment-evidence", "1.0.0")}}
	server := newToolsTestServer(t, fake, resolver)
	tenant := toolTenantID(t)

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tenants/"+tenant.String()+"/tools", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("код = %d, тело: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(payload.Tools) != 1 {
		t.Fatalf("ожидался 1 контракт, получено %d", len(payload.Tools))
	}
	view := payload.Tools[0]
	if view["id"] != "auto-parts.fitment-evidence" || view["version"] != "1.0.0" {
		t.Fatalf("контракт: %v", view)
	}
	if view["determinism_class"] != "deterministic_read" {
		t.Fatalf("determinism_class: %v", view["determinism_class"])
	}
	if view["adapter_status"] != "not_implemented" {
		t.Fatalf("adapter_status: %v", view["adapter_status"])
	}
	if view["dependency"] == nil {
		t.Fatal("dependency отсутствует")
	}
	if view["dependency"].(map[string]any)["connector_type"] != "catalog" {
		t.Fatalf("dependency.connector_type: %v", view["dependency"])
	}
	input, ok := view["input_schema"].([]any)
	if !ok || len(input) == 0 {
		t.Fatalf("input_schema: %v", view["input_schema"])
	}
}

// TestToolsCatalogGetAPI: the pin read answers the exact version — the
// material for the future manifest/planner references.
func TestToolsCatalogGetAPI(t *testing.T) {
	resolver := fixedResolver{principal: domain.Principal{ID: testPrincipalID}}
	fake := &fakeToolsCatalog{contracts: []tools.ToolContract{toolFixtureContract("auto-parts.fitment-evidence", "1.0.0")}}
	server := newToolsTestServer(t, fake, resolver)
	tenant := toolTenantID(t)

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tenants/"+tenant.String()+"/tools/auto-parts.fitment-evidence/1.0.0", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("код = %d, тело: %s", rec.Code, rec.Body.String())
	}
	if fake.lastGet == nil || fake.lastGet.toolID != "auto-parts.fitment-evidence" || fake.lastGet.version != "1.0.0" {
		t.Fatalf("пин не дошёл до сервиса: %+v", fake.lastGet)
	}
	var payload struct {
		Tool map[string]any `json:"tool"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if payload.Tool["id"] != "auto-parts.fitment-evidence" {
		t.Fatalf("контракт: %v", payload.Tool)
	}
}

// TestToolsCatalogMalformedPinIs404: a malformed tool id (bad charset) or
// a non-triple version is the honest 404 of an invented pin — strict
// decode BEFORE the service (the service is never called).
func TestToolsCatalogMalformedPinIs404(t *testing.T) {
	resolver := fixedResolver{principal: domain.Principal{ID: testPrincipalID}}
	fake := &fakeToolsCatalog{contracts: []tools.ToolContract{toolFixtureContract("auto-parts.fitment-evidence", "1.0.0")}}
	server := newToolsTestServer(t, fake, resolver)
	tenant := toolTenantID(t)

	for _, probe := range []struct {
		name string
		path string
	}{
		{"плохой charset id", "/v1/tenants/" + tenant.String() + "/tools/Плохой!ID/1.0.0"},
		{"id с заглавными буквами", "/v1/tenants/" + tenant.String() + "/tools/Auto-Parts/1.0.0"},
		{"version двухкомпонентная", "/v1/tenants/" + tenant.String() + "/tools/auto-parts.fitment-evidence/1.0"},
		{"version с leading zero", "/v1/tenants/" + tenant.String() + "/tools/auto-parts.fitment-evidence/01.0.0"},
		{"version prerelease", "/v1/tenants/" + tenant.String() + "/tools/auto-parts.fitment-evidence/1.0.0-rc.1"},
		{"version не числовая", "/v1/tenants/" + tenant.String() + "/tools/auto-parts.fitment-evidence/abc"},
	} {
		t.Run(probe.name, func(t *testing.T) {
			fake.lastGet = nil
			rec := httptest.NewRecorder()
			server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, probe.path, nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("код = %d, ожидался 404; тело: %s", rec.Code, rec.Body.String())
			}
			if fake.lastGet != nil {
				t.Fatalf("малформед-пин дошёл до сервиса: %+v", fake.lastGet)
			}
		})
	}
}

// TestToolsCatalogUnknownPinIs404: a well-formed but unknown pin is the
// service's own honest 404 — the invented future does not exist.
func TestToolsCatalogUnknownPinIs404(t *testing.T) {
	resolver := fixedResolver{principal: domain.Principal{ID: testPrincipalID}}
	fake := &fakeToolsCatalog{}
	server := newToolsTestServer(t, fake, resolver)
	tenant := toolTenantID(t)

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tenants/"+tenant.String()+"/tools/auto-parts.surprise/1.0.0", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("код = %d, ожидался 404", rec.Code)
	}
}

// TestToolsCatalogLadder: the membership ladder — a non-member is the
// honest 404 (no disclosure the tenant has a tool surface), a member
// without the scopes is the 403 WITH the scope words BEFORE any material.
func TestToolsCatalogLadder(t *testing.T) {
	fake := &fakeToolsCatalog{contracts: []tools.ToolContract{toolFixtureContract("auto-parts.fitment-evidence", "1.0.0")}}
	tenant := toolTenantID(t)

	t.Run("не-member 404 (сервисный отказ лестницы)", func(t *testing.T) {
		// Сервис отвечает ErrNotFound не-member'а (лестница авторизации):
		// транспорт обязан отдать честный 404 без раскрытия поверхности.
		fakeNotFound := &fakeToolsCatalog{listErr: tools.ErrNotFound}
		resolver := fixedResolver{principal: domain.Principal{ID: testPrincipalID}}
		server := newToolsTestServer(t, fakeNotFound, resolver)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tenants/"+tenant.String()+"/tools", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("код = %d, ожидался 404", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "Реестр инструментов") {
			t.Fatal("404 не-member раскрыл существование поверхности")
		}
	})

	t.Run("малформед tenant id 404", func(t *testing.T) {
		resolver := fixedResolver{principal: domain.Principal{ID: testPrincipalID}}
		server := newToolsTestServer(t, fake, resolver)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tenants/not-a-uuid/tools", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("код = %d, ожидался 404", rec.Code)
		}
	})

	t.Run("member без скоупов 403 со словами скоупов", func(t *testing.T) {
		fakeForbidden := &fakeToolsCatalog{listErr: fmt.Errorf("%w: %s", tools.ErrForbidden, toolScopeWords)}
		resolver := fixedResolver{principal: domain.Principal{ID: testPrincipalID}}
		server := newToolsTestServer(t, fakeForbidden, resolver)
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tenants/"+tenant.String()+"/tools", nil))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("код = %d, ожидался 403; тело: %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "integrations или configuration") {
			t.Fatalf("403 без слов скоупов: %s", rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "Доказательство применимости") {
			t.Fatal("403 выдал материал каталога до отказа")
		}
	})
}

// TestToolsPageRenders: the page renders the cards with the dictionary
// words as TEXT (versions, statuses, classes, dependency, owner) and the
// progressive disclosure of the schemas.
func TestToolsPageRenders(t *testing.T) {
	resolver := fixedResolver{principal: domain.Principal{ID: testPrincipalID}}
	fake := &fakeToolsCatalog{contracts: []tools.ToolContract{toolFixtureContract("auto-parts.fitment-evidence", "1.0.0")}}
	server := newToolsTestServer(t, fake, resolver)
	tenant := toolTenantID(t)

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/tenants/"+tenant.String()+"/tools", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("код = %d, тело: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, word := range []string{
		"Реестр инструментов",
		"Доказательство применимости",
		"deterministic_read",
		"not_implemented",
		"auto-parts.fitment-evidence",
		"вызов невозможен",
		"каталог",
		"auto-parts/spec/4.4",
		"<details",
	} {
		if !strings.Contains(body, word) {
			t.Fatalf("страница без слова %q", word)
		}
	}
}

// TestToolsPageEmptyState: the honest empty registry renders the reason
// words — the empty state is a readable state, never a silent blank.
func TestToolsPageEmptyState(t *testing.T) {
	resolver := fixedResolver{principal: domain.Principal{ID: testPrincipalID}}
	server := newToolsTestServer(t, &fakeToolsCatalog{}, resolver)
	tenant := toolTenantID(t)

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/tenants/"+tenant.String()+"/tools", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("код = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Реестр инструментов пуст") {
		t.Fatal("пустое состояние без слов причины")
	}
	// html/template экранирует «+» как «&#43;» — проверяем слова без
	// спецсимвола (путь внешнего автора виден текстом).
	if !strings.Contains(body, "артефакт") || !strings.Contains(body, "индексная запись") {
		t.Fatal("пустое состояние без пути внешнего автора")
	}
}

// TestToolsCatalogInternalError: an unexpected service failure is the
// honest 500 with the server-side log — never a silent 200.
func TestToolsCatalogInternalError(t *testing.T) {
	resolver := fixedResolver{principal: domain.Principal{ID: testPrincipalID}}
	fake := &fakeToolsCatalog{listErr: errors.New("сбой")}
	server := newToolsTestServer(t, fake, resolver)
	tenant := toolTenantID(t)

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tenants/"+tenant.String()+"/tools", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("код = %d, ожидался 500", rec.Code)
	}
}

// TestToolsCatalogUnboundIs404: without the bound surface the reads are
// the honest 404 (the surface does not exist until the module wires it).
func TestToolsCatalogUnboundIs404(t *testing.T) {
	resolver := fixedResolver{principal: domain.Principal{ID: testPrincipalID}}
	server := newToolsTestServer(t, nil, resolver)
	tenant := toolTenantID(t)

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/tenants/"+tenant.String()+"/tools", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("код = %d, ожидался 404", rec.Code)
	}
}
