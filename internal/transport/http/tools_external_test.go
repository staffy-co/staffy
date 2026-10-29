package httptransport

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/externaltools"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/tools"
)

// fakeExternalTools is the transport fake of the dictionary surface: it
// answers the canned sections and records the handed material (the
// transport tests pin the HTTP mapping and the ladder order — the
// derivation itself is the service's own unit truth).
type fakeExternalTools struct {
	sections   []externaltools.Section
	err        error
	lastActor  id.UUID
	lastTenant id.UUID
}

// The compile-time proof the fake satisfies the transport contract.
var _ externalToolsService = (*fakeExternalTools)(nil)

func (f *fakeExternalTools) Dictionary(_ context.Context, actor domain.Principal, tenantID id.UUID) ([]externaltools.Section, error) {
	f.lastActor = actor.ID
	f.lastTenant = tenantID
	if f.err != nil {
		return nil, f.err
	}
	if f.sections == nil {
		return []externaltools.Section{}, nil
	}
	return f.sections, nil
}

func newExternalToolsTestServer(t *testing.T, fake *fakeExternalTools, resolver fixedResolver) http.Handler {
	t.Helper()
	server, err := New(nil, resolver, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if fake != nil {
		server.BindExternalTools(fake)
	}
	return server.Handler()
}

func externalToolsTenant(t *testing.T) id.UUID {
	t.Helper()
	tenant, err := id.Parse("01994f64-6c00-7000-8000-0000000000a1")
	if err != nil {
		t.Fatalf("parse tenant: %v", err)
	}
	return tenant
}

func externalToolsPrincipal(t *testing.T) domain.Principal {
	t.Helper()
	principal, err := id.Parse("01994f64-6c00-7000-8000-0000000000c1")
	if err != nil {
		t.Fatalf("parse principal: %v", err)
	}
	return domain.Principal{ID: principal}
}

func externalToolsRequest(t *testing.T, handler http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// TestExternalToolsLadderBeforeMaterial: the resolver 401 and the
// malformed tenant 404 answer with the platform problem contract BEFORE
// the dictionary service (and any material) runs — the fake records the
// hand-off, and the refusal legs prove it never happened.
func TestExternalToolsLadderBeforeMaterial(t *testing.T) {
	fake := &fakeExternalTools{}
	tenant := externalToolsTenant(t).String()

	// The identity refusal comes first.
	handler := newExternalToolsTestServer(t, fake, fixedResolver{err: domain.ErrNotFound})
	recorder := externalToolsRequest(t, handler, http.MethodGet, "/v1/tenants/"+tenant+"/tools/external")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("резолвер: %d", recorder.Code)
	}
	if fake.lastTenant != "" {
		t.Fatal("материал не читается до резолвера")
	}

	// The malformed tenant path is the honest 404 (a valid identity, an
	// unparseable tenant — the resolver passes, the path refuses).
	handler = newExternalToolsTestServer(t, fake, fixedResolver{principal: externalToolsPrincipal(t)})
	recorder = externalToolsRequest(t, handler, http.MethodGet, "/v1/tenants/not-a-uuid/tools/external")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("малформед-путь: %d", recorder.Code)
	}
	if fake.lastTenant != "" {
		t.Fatal("материал не читается до разбора пути")
	}

	// The non-member: the module's honest 404 without any disclosure.
	handler = newExternalToolsTestServer(t, fake, fixedResolver{principal: externalToolsPrincipal(t)})
	fake.err = externaltools.ErrNotFound
	recorder = externalToolsRequest(t, handler, http.MethodGet, "/v1/tenants/"+tenant+"/tools/external")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("не-member: %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "sections") {
		t.Fatal("404 не раскрывает словарь")
	}

	// The scope refusal: the 403 whose message IS the scope words.
	fake.err = nil
	scopeWords := "Чтение словаря внешних инструментов требует полномочия integrations или configuration"
	fake.err = wrapExternalForbidden(scopeWords)
	recorder = externalToolsRequest(t, handler, http.MethodGet, "/v1/tenants/"+tenant+"/tools/external")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("скоупы: %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "integrations или configuration") {
		t.Fatalf("слова скоупов: %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "sections") {
		t.Fatal("403 не раскрывает материал")
	}

	// The infrastructure failure: the honest 500.
	fake.err = context.Canceled
	recorder = externalToolsRequest(t, handler, http.MethodGet, "/v1/tenants/"+tenant+"/tools/external")
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("инфраструктура: %d", recorder.Code)
	}

	// The unbound surface: the honest 404 of a missing contour.
	handler = newExternalToolsTestServer(t, nil, fixedResolver{principal: externalToolsPrincipal(t)})
	recorder = externalToolsRequest(t, handler, http.MethodGet, "/v1/tenants/"+tenant+"/tools/external")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("непривязанная поверхность: %d", recorder.Code)
	}
}

// wrapExternalForbidden builds the service wrap of the scope refusal (the
// fmt.Errorf("%w: %s") form the transport strips).
func wrapExternalForbidden(words string) error {
	return &externalForbiddenError{words: words}
}

type externalForbiddenError struct{ words string }

func (e *externalForbiddenError) Error() string {
	return externaltools.ErrForbidden.Error() + ": " + e.words
}

func (e *externalForbiddenError) Unwrap() error { return externaltools.ErrForbidden }

// TestExternalToolsAPIAnswersTheClosedSections: the machine list answers
// the closed section form — the honest empty array (never null) and the
// full entry material of the live derivation.
func TestExternalToolsAPIAnswersTheClosedSections(t *testing.T) {
	checked := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	tenant := externalToolsTenant(t).String()
	fake := &fakeExternalTools{sections: []externaltools.Section{{
		ConnectionID:    externalToolsTenant(t),
		ConnectorID:     "staffy-external-mcp-server",
		ContractVersion: "1.0.0",
		Environment:     "sandbox",
		Status:          "connected_unverified",
		HasEntries:      true,
		Entries: []externaltools.Entry{{
			Name:        "parts.lookup",
			Title:       "Поиск детали",
			Description: "Читает каталог",
			State:       externaltools.EntryAdmitted,
			Provenance: &externaltools.Provenance{
				ConnectorID:      "staffy-external-mcp-server",
				ConnectorVersion: "1.0.0",
				ServerName:       "fixture-external",
				ServerVersion:    "1.0.0",
				EvidenceID:       externalToolsTenant(t),
				CheckedAt:        checked,
				ExpiresAt:        checked.Add(24 * time.Hour),
			},
			Input: map[string]tools.ToolSchemaField{
				"query": {Label: "Запрос", Type: "string"},
			},
			Output: map[string]tools.ToolSchemaField{
				"found": {Label: "Найдено", Type: "boolean"},
			},
		}},
	}}}
	handler := newExternalToolsTestServer(t, fake, fixedResolver{principal: externalToolsPrincipal(t)})
	recorder := externalToolsRequest(t, handler, http.MethodGet, "/v1/tenants/"+tenant+"/tools/external")
	if recorder.Code != http.StatusOK {
		t.Fatalf("код: %d", recorder.Code)
	}
	body := recorder.Body.String()
	for _, word := range []string{
		`"sections":[{`,
		`"connector_id":"staffy-external-mcp-server"`,
		`"connector_version":"1.0.0"`,
		`"environment":"sandbox"`,
		`"status":"connected_unverified"`,
		`"has_entries":true`,
		`"name":"parts.lookup"`,
		`"state":"admitted"`,
		`"server_name":"fixture-external"`,
		`"input_schema":[{"name":"query","label":"Запрос","type":"string"`,
		`"output_schema":[{"name":"found","label":"Найдено","type":"boolean"`,
	} {
		if !strings.Contains(body, word) {
			t.Fatalf("закрытая форма ответа не несёт %q: %s", word, body)
		}
	}
	if fake.lastTenant.String() != tenant {
		t.Fatalf("изолированный скоуп: %v", fake.lastTenant)
	}

	// The honest empty dictionary: the empty array, never null.
	empty := &fakeExternalTools{}
	handler = newExternalToolsTestServer(t, empty, fixedResolver{principal: externalToolsPrincipal(t)})
	recorder = externalToolsRequest(t, handler, http.MethodGet, "/v1/tenants/"+tenant+"/tools/external")
	if recorder.Code != http.StatusOK {
		t.Fatalf("пустой словарь: %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"sections":[]`) {
		t.Fatalf("пустой словарь обязан быть пустым массивом: %s", recorder.Body.String())
	}
}

// TestExternalToolsPageIsReadOnlyAndHonest: the operator page renders the
// sections with the provenance of every entry, the honest reasons of the
// empty sections and the honest empty state of the tenant without
// connections of the class — and the surface carries no forms at all.
func TestExternalToolsPageIsReadOnlyAndHonest(t *testing.T) {
	tenant := externalToolsTenant(t).String()
	fake := &fakeExternalTools{sections: []externaltools.Section{{
		ConnectionID:    externalToolsTenant(t),
		ConnectorID:     "staffy-external-mcp-server",
		ContractVersion: "1.0.0",
		Environment:     "sandbox",
		Status:          "connected_unverified",
		HasEntries:      true,
		Entries: []externaltools.Entry{{
			Name:        "parts.mutate",
			Title:       "Мутация",
			Description: "Меняет каталог",
			State:       externaltools.EntryRejected,
			Reason:      "инструмент parts.mutate не заявляет read-only (annotations.readOnlyHint): без заявления сервера read-only инструмент не допускается (AR-30)",
		}},
	}}}
	handler := newExternalToolsTestServer(t, fake, fixedResolver{principal: externalToolsPrincipal(t)})
	recorder := externalToolsRequest(t, handler, http.MethodGet, "/app/tenants/"+tenant+"/tools/external")
	if recorder.Code != http.StatusOK {
		t.Fatalf("страница: %d", recorder.Code)
	}
	page := recorder.Body.String()
	if !strings.Contains(page, "Словарь внешних инструментов") {
		t.Fatalf("заголовок страницы: %s", page[:200])
	}
	if !strings.Contains(page, "отклонён") || !strings.Contains(page, "AR-30") {
		t.Fatalf("страница обязана нести честную причину отклонения: %s", page)
	}
	if strings.Contains(page, "<form") {
		t.Fatal("страница словаря строго read-only: форм не существует")
	}
	if !strings.Contains(page, "Planner") {
		t.Fatalf("страница обязана называть будущего потребителя: %s", page)
	}

	// The honest empty state of the tenant without connections of the class.
	empty := &fakeExternalTools{}
	handler = newExternalToolsTestServer(t, empty, fixedResolver{principal: externalToolsPrincipal(t)})
	recorder = externalToolsRequest(t, handler, http.MethodGet, "/app/tenants/"+tenant+"/tools/external")
	if recorder.Code != http.StatusOK {
		t.Fatalf("пустая страница: %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "Словарь внешних инструментов пуст") {
		t.Fatalf("честное пустое состояние: %s", recorder.Body.String()[:300])
	}
}
