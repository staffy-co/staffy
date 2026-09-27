package httptransport

import (
	"bytes"
	"context"
	"errors"
	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func ingestMultipart(t *testing.T, path string, duplicate bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for key, value := range map[string]string{"type": "document", "title": "TXT", "owner": "Тест", "provenance": "Файл", "scope": "Тест", "priority": "50", "language": "ru"} {
		if err := w.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if duplicate {
		if err := w.WriteField("type", "website"); err != nil {
			t.Fatal(err)
		}
		if err := w.WriteField("fixture", "true"); err != nil {
			t.Fatal(err)
		}
		if err := w.WriteField("fixture", "false"); err != nil {
			t.Fatal(err)
		}
		if err := w.WriteField("url", "https://example.com/a"); err != nil {
			t.Fatal(err)
		}
		if err := w.WriteField("url", "https://example.com/b"); err != nil {
			t.Fatal(err)
		}
	}
	part, err := w.CreateFormFile("file", "sample.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("exact UTF-8 bytes: привет")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, path, &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.Header.Set("Origin", "http://example.com")
	return r
}

func TestKnowledgeIngestMultipartForwardsExactFileAndRejectsUnsafeMetadata(t *testing.T) {
	path := "/v1/tenants/" + testTenantID.String() + "/employees/" + testCorrelation.String() + "/knowledge-sources/ingest"
	t.Run("file", func(t *testing.T) {
		f := &knowledgeHTTPFake{}
		r := ingestMultipart(t, path, false)
		w := httptest.NewRecorder()
		knowledgeTestServer(t, f).Handler().ServeHTTP(w, r)
		if w.Code != http.StatusCreated || f.ingestCmd == nil || f.ingestCmd.FileName != "sample.txt" || !bytes.Equal(f.ingestCmd.File, []byte("exact UTF-8 bytes: привет")) {
			t.Fatalf("multipart forwarding: status=%d command=%+v", w.Code, f.ingestCmd)
		}
	})
	for _, tc := range []struct {
		name   string
		mutate func(*http.Request)
	}{
		{"cross_origin", func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") }},
		{"duplicates", func(r *http.Request) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &knowledgeHTTPFake{}
			r := ingestMultipart(t, path, tc.name == "duplicates")
			tc.mutate(r)
			w := httptest.NewRecorder()
			knowledgeTestServer(t, f).Handler().ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
				t.Fatalf("unsafe multipart accepted: %d %s", w.Code, w.Body.String())
			}
			if f.ingestCmd != nil {
				t.Fatal("unsafe multipart reached service")
			}
		})
	}
}

type knowledgeHTTPFake struct {
	fakeEmployee
	listErr, versionErr error
	createCalls         int
	processCmd          *employeeAPI.KnowledgeProcessCommand
	ingestCmd           *employeeAPI.KnowledgeIngestCommand
}

func (f *knowledgeHTTPFake) Read(context.Context, identityDomain.Principal, id.UUID, id.UUID) (employeeDomain.Draft, error) {
	return employeeDomain.Draft{TenantID: testTenantID, EmployeeID: testCorrelation, OwnerID: testPrincipalID, Version: 2, Pack: &employeeDomain.PackPin{ID: "auto-parts", Version: "1.0.0"}}, nil
}
func (f *knowledgeHTTPFake) CreateKnowledge(_ context.Context, _ employeeAPI.KnowledgeCreateCommand) (employeeDomain.KnowledgeSource, error) {
	f.createCalls++
	return employeeDomain.KnowledgeSource{}, nil
}
func (f *knowledgeHTTPFake) ListKnowledge(context.Context, identityDomain.Principal, id.UUID, id.UUID) ([]employeeDomain.KnowledgeSource, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return []employeeDomain.KnowledgeSource{{SourceID: testCorrelation, Title: "FAQ", Status: "ready", Version: 1}}, nil
}
func (f *knowledgeHTTPFake) ReadKnowledge(context.Context, identityDomain.Principal, id.UUID, id.UUID, id.UUID) (employeeDomain.KnowledgeSource, []employeeDomain.KnowledgeVersion, error) {
	if f.versionErr != nil {
		return employeeDomain.KnowledgeSource{}, nil, f.versionErr
	}
	return employeeDomain.KnowledgeSource{SourceID: testCorrelation, Title: "FAQ", Status: "ready", Version: 1}, nil, nil
}
func (f *knowledgeHTTPFake) ProcessKnowledge(_ context.Context, c employeeAPI.KnowledgeProcessCommand) (employeeDomain.KnowledgeSource, employeeDomain.KnowledgeVersion, bool, error) {
	f.processCmd = &c
	return employeeDomain.KnowledgeSource{SourceID: c.SourceID, Version: 2, Status: "ready"}, employeeDomain.KnowledgeVersion{SourceID: c.SourceID, Version: 1, Text: c.Text, Checksum: "stored-checksum"}, true, nil
}
func (f *knowledgeHTTPFake) IngestKnowledge(_ context.Context, c employeeAPI.KnowledgeIngestCommand) (employeeDomain.KnowledgeSource, employeeDomain.KnowledgeVersion, error) {
	f.ingestCmd = &c
	return employeeDomain.KnowledgeSource{SourceID: testCorrelation, Status: "processing"}, employeeDomain.KnowledgeVersion{}, nil
}
func (f *knowledgeHTTPFake) ReingestSyntheticKnowledge(_ context.Context, c employeeAPI.KnowledgeFixtureCommand) (employeeDomain.KnowledgeSource, employeeDomain.KnowledgeVersion, bool, error) {
	return employeeDomain.KnowledgeSource{SourceID: c.SourceID, Status: "preview"}, employeeDomain.KnowledgeVersion{Version: 1}, true, nil
}
func TestKnowledgeIngestJSONForwardsOnlyScopedFixture(t *testing.T) {
	f := &knowledgeHTTPFake{}
	s := knowledgeTestServer(t, f)
	path := "/v1/tenants/" + testTenantID.String() + "/employees/" + testCorrelation.String() + "/knowledge-sources/ingest"
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"type":"document","title":"Образец","owner":"Тест","provenance":"Фикстура","scope":"Тест","priority":50,"language":"ru","fixture":true}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusCreated || f.ingestCmd == nil || !f.ingestCmd.Fixture || f.ingestCmd.TenantID != testTenantID || f.ingestCmd.EmployeeID != testCorrelation || f.ingestCmd.Actor.ID != testPrincipalID {
		t.Fatalf("ingest response=%d command=%+v", w.Code, f.ingestCmd)
	}
	if !strings.Contains(w.Body.String(), `"publishable":false`) {
		t.Fatalf("ingest claimed publishable: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"status":"processing"`) {
		t.Fatalf("ingest skipped processing: %s", w.Body.String())
	}
}
func TestKnowledgeProcessJSONForwardsVersionAndText(t *testing.T) {
	f := &knowledgeHTTPFake{}
	s := knowledgeTestServer(t, f)
	path := "/v1/tenants/" + testTenantID.String() + "/employees/" + testCorrelation.String() + "/knowledge-sources/" + testCorrelation.String() + "/process"
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"version":7,"text":"Вопрос и ответ"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 || f.processCmd == nil {
		t.Fatalf("process response=%d %s command=%+v", w.Code, w.Body.String(), f.processCmd)
	}
	if f.processCmd.ExpectedVersion != 7 || f.processCmd.Text != "Вопрос и ответ" || f.processCmd.TenantID != testTenantID || f.processCmd.EmployeeID != testCorrelation || f.processCmd.SourceID != testCorrelation || f.processCmd.Actor.ID != testPrincipalID {
		t.Fatalf("process command=%+v", f.processCmd)
	}
	if !strings.Contains(w.Body.String(), `"replayed":true`) || !strings.Contains(w.Body.String(), `"checksum":"stored-checksum"`) || !strings.Contains(w.Body.String(), `"status":"ready"`) {
		t.Fatalf("process response=%d %s", w.Code, w.Body.String())
	}
}
func (f *knowledgeHTTPFake) ConflictKnowledge(context.Context, employeeAPI.KnowledgeConflictCommand) (employeeDomain.KnowledgeSource, error) {
	return employeeDomain.KnowledgeSource{}, nil
}
func (f *knowledgeHTTPFake) RetireKnowledge(context.Context, employeeAPI.KnowledgeRetireCommand) (employeeDomain.KnowledgeSource, error) {
	return employeeDomain.KnowledgeSource{}, nil
}
func knowledgeTestServer(t *testing.T, f *knowledgeHTTPFake) *Server {
	t.Helper()
	identity := fakeIdentity{create: func(identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
		return identityAPI.CreateTenantResult{}, nil
	}, workspace: func(id.UUID) (identityDomain.Workspace, error) {
		return identityDomain.Workspace{}, identityDomain.ErrNotFound
	}}
	s, err := New(identity, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	s.BindEmployee(f)
	return s
}
func TestKnowledgePageReportsListAndVersionErrors(t *testing.T) {
	path := "/app/tenants/" + testTenantID.String() + "/employees/" + testCorrelation.String()
	for _, tc := range []struct {
		name                string
		listErr, versionErr error
	}{{"list", errors.New("list failed"), nil}, {"version", nil, errors.New("version failed")}} {
		t.Run(tc.name, func(t *testing.T) {
			f := &knowledgeHTTPFake{listErr: tc.listErr, versionErr: tc.versionErr}
			s := knowledgeTestServer(t, f)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			if w.Code != 500 || strings.Contains(w.Body.String(), "FAQ") {
				t.Fatalf("knowledge error hidden: %d %s", w.Code, w.Body.String())
			}
		})
	}
}
func TestKnowledgeCreateRejectsURLAndAuthorityFields(t *testing.T) {
	f := &knowledgeHTTPFake{}
	s := knowledgeTestServer(t, f)
	path := "/v1/tenants/" + testTenantID.String() + "/employees/" + testCorrelation.String() + "/knowledge-sources"
	for _, body := range []string{`{"type":"faq","title":"FAQ","owner":"A","provenance":"B","scope":"C","priority":1,"language":"ru","url":"https://example.com"}`, `{"type":"faq","title":"FAQ","owner":"A","provenance":"B","scope":"C","priority":1,"language":"ru","role":"owner"}`} {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("unsafe field accepted: %d %s", w.Code, w.Body.String())
		}
	}
	if f.createCalls != 0 {
		t.Fatalf("created %d sources from rejected bodies", f.createCalls)
	}
}
