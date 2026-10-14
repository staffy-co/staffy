package httptransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"io"
	"log/slog"
)

// ---------------------------------------------------------------------------
// The Story 2.10 wire shapes: the transport contract of the two operator
// commands and the public protected surface. The command semantics live in
// the integration suite; the fakes embed the full existing fake (the
// composite Service stays untouched — the narrow interface is the
// BindEmployee precedent).
// ---------------------------------------------------------------------------

// continuationCases is the Story 2.10 wire fake: the full existing surface
// plus the continuation methods under test.
type continuationCases struct {
	*fakeCases
	issue   func(context.Context, casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error)
	revoke  func(context.Context, casesAPI.RevokeContinuationLinkCommand) (casesAPI.WorkOutcome, error)
	surface func(context.Context, string, time.Time) (casesAPI.ContinuationSurface, error)
	correct func(context.Context, string, id.UUID, []byte, time.Time) (int64, error)
	human   func(context.Context, string, time.Time) error
	return_ func(context.Context, string, time.Time) error
}

func (f *continuationCases) IssueContinuationLink(ctx context.Context, command casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error) {
	return f.issue(ctx, command)
}

func (f *continuationCases) RevokeContinuationLink(ctx context.Context, command casesAPI.RevokeContinuationLinkCommand) (casesAPI.WorkOutcome, error) {
	return f.revoke(ctx, command)
}

func (f *continuationCases) ContinueSurface(ctx context.Context, token string, now time.Time) (casesAPI.ContinuationSurface, error) {
	return f.surface(ctx, token, now)
}

func (f *continuationCases) SubmitCorrection(ctx context.Context, token string, factID id.UUID, rawValue []byte, now time.Time) (int64, error) {
	return f.correct(ctx, token, factID, rawValue, now)
}

func (f *continuationCases) SubmitHuman(ctx context.Context, token string, now time.Time) error {
	return f.human(ctx, token, now)
}

func (f *continuationCases) SubmitReturn(ctx context.Context, token string, now time.Time) error {
	return f.return_(ctx, token, now)
}

// tenantNameIdentity is the identity fake of the public page header (the
// narrow TenantNameService of the Story 2.10).
type tenantNameIdentity struct {
	name string
}

func (f tenantNameIdentity) CreateTenant(context.Context, identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
	return identityAPI.CreateTenantResult{}, nil
}
func (f tenantNameIdentity) Workspace(context.Context, domain.Principal, id.UUID) (domain.Workspace, error) {
	return domain.Workspace{}, nil
}
func (f tenantNameIdentity) TenantName(_ context.Context, _ id.UUID) (string, error) {
	return f.name, nil
}

// newContinuationTestServer mounts the cases fake with the tenant-name
// identity fake.
func newContinuationTestServer(t *testing.T, fake casesAPI.Service, identity identityAPI.Service) *Server {
	t.Helper()
	server, err := New(identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.BindCases(fake)
	return server
}

const (
	issuePath  = "/v1/tenants/01994f64-6c00-7000-8000-000000000011/cases/01994f64-6c00-7000-8000-00000000aa01/work-items/01994f64-6c00-7000-8000-00000000bb02/continuation-links"
	revokePath = issuePath + "/01994f64-6c00-7000-8000-00000000cc04/revoke"
)

// TestContinuationIssueWireShapeIsHonest pins the machine contract of the
// issuance: the fresh 201 carries the one-time url/token with the
// no-store hygiene; the replay 200 answers the honest mask (the empty
// url — the preimage is never recoverable); the missing dictionary
// fields are the honest 400s naming the field.
func TestContinuationIssueWireShapeIsHonest(t *testing.T) {
	server := newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, issue: func(_ context.Context, command casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error) {
		if len(command.Purposes) != 1 || command.Purposes[0] != casesDomain.ContinuationPurposeFactCorrection {
			t.Fatalf("unexpected purposes: %v", command.Purposes)
		}
		if command.TTLMinutes != 30 || command.ConsumptionLimit != 1 {
			t.Fatalf("unexpected bounds: ttl=%d limit=%d", command.TTLMinutes, command.ConsumptionLimit)
		}
		return casesAPI.ContinuationIssuance{
			Outcome: casesAPI.WorkOutcome{Command: "issue_continuation_link", LinkID: id.MustParse("01994f64-6c00-7000-8000-00000000cc04"), ContinuationIssued: true},
			URL:     "/continue/testtoken", Token: "testtoken", Expires: time.Unix(1, 0).UTC(),
		}, nil
	}}, tenantNameIdentity{name: "Магазин Олега"})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, issuePath,
		`{"purposes":["fact_correction"],"question":"Верный VIN?","explanation":"Проверьте номер","participant_ref":"wa:7999","ttl_minutes":30}`))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"url":"/continue/testtoken"`) || !strings.Contains(body, `"token":"testtoken"`) {
		t.Fatalf("the fresh answer carries the one-time url/token, body = %s", body)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("the one-time answer carries the no-store/no-referrer hygiene")
	}
	// The replay mask: the empty url, the honest word — never a
	// reconstruction.
	server = newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, issue: func(context.Context, casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error) {
		return casesAPI.ContinuationIssuance{Outcome: casesAPI.WorkOutcome{Command: "issue_continuation_link", Replay: true, ContinuationIssued: true}}, nil
	}}, tenantNameIdentity{})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, issuePath,
		`{"purposes":["fact_correction"],"question":"В","explanation":"П","participant_ref":"wa:1","ttl_minutes":5}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("replay status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "testtoken") {
		t.Fatalf("the replay answer never reconstructs the token, body = %s", recorder.Body.String())
	}
	// The honest 400s name the missing fields.
	server = newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, issue: func(context.Context, casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error) {
		return casesAPI.ContinuationIssuance{}, nil
	}}, tenantNameIdentity{})
	payloads := map[string]string{
		"purposes":        `{"question":"В","explanation":"П","participant_ref":"wa:1","ttl_minutes":30}`,
		"question":        `{"purposes":["fact_correction"],"explanation":"П","participant_ref":"wa:1","ttl_minutes":30}`,
		"explanation":     `{"purposes":["fact_correction"],"question":"В","participant_ref":"wa:1","ttl_minutes":30}`,
		"participant_ref": `{"purposes":["fact_correction"],"question":"В","explanation":"П","ttl_minutes":30}`,
		"ttl_minutes":     `{"purposes":["fact_correction"],"question":"В","explanation":"П","participant_ref":"wa:1"}`,
	}
	for _, field := range []string{"purposes", "question", "explanation", "participant_ref", "ttl_minutes"} {
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, issuePath, payloads[field]))
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), field) {
			t.Fatalf("the missing %s names the field, status = %d, body = %s", field, recorder.Code, recorder.Body.String())
		}
	}
	// The scope gate: the 403 with the scope words BEFORE the guards.
	server = newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, issue: func(context.Context, casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error) {
		return casesAPI.ContinuationIssuance{}, domain.ErrForbidden
	}}, tenantNameIdentity{})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, issuePath,
		`{"purposes":["fact_correction"],"question":"В","explanation":"П","participant_ref":"wa:1","ttl_minutes":5}`))
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "review или configuration") {
		t.Fatalf("the audit-only member reads the scope words, status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	// The honest conflicts: the key conflict and the not-found.
	server = newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, issue: func(context.Context, casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error) {
		return casesAPI.ContinuationIssuance{}, casesDomain.ErrCommandKeyConflict
	}}, tenantNameIdentity{})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, issuePath,
		`{"purposes":["fact_correction"],"question":"В","explanation":"П","participant_ref":"wa:1","ttl_minutes":5}`))
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "command_key_conflict") {
		t.Fatalf("the key conflict is machine-readable, status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

// TestContinuationRevokeWireShapeIsHonest pins the revoke contract: the
// reason is required (the honest 400); the second revoke of the same link
// refuses honestly (the WorkRuleError of the store).
func TestContinuationRevokeWireShapeIsHonest(t *testing.T) {
	server := newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, revoke: func(_ context.Context, command casesAPI.RevokeContinuationLinkCommand) (casesAPI.WorkOutcome, error) {
		if command.Reason != "прекратить ожидание" {
			t.Fatalf("unexpected reason: %q", command.Reason)
		}
		return casesAPI.WorkOutcome{Command: "revoke_continuation_link", LinkID: command.LinkID}, nil
	}}, tenantNameIdentity{})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, revokePath, `{"reason":"прекратить ожидание"}`))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "revoke_continuation_link") {
		t.Fatalf("the outcome names the command, body = %s", recorder.Body.String())
	}
	// The missing reason is the honest 400.
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, revokePath, `{}`))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "reason") {
		t.Fatalf("the refusal names the reason field, status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	// The second revoke refuses honestly.
	server = newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, revoke: func(context.Context, casesAPI.RevokeContinuationLinkCommand) (casesAPI.WorkOutcome, error) {
		return casesAPI.WorkOutcome{}, casesDomain.WorkRuleError{Rule: "revoke_continuation_link", Reason: casesDomain.ContinuationAlreadyRevokedText}
	}}, tenantNameIdentity{})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost, revokePath, `{"reason":"ещё раз"}`))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "уже отозвана") {
		t.Fatalf("the second revoke is the honest refusal, status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

// continuationFormRequest builds one urlencoded form POST of the public
// actions (the browser shape: the content type header included).
func continuationFormRequest(method, target string, form url.Values) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

// TestContinuationIssueFormRendersOneTimeURL pins the operator form path:
// the issuance is NOT a redirect — the case page re-renders with the
// one-time URL card (the people invitation precedent: the material would
// be lost in a flash) under the no-store/no-referrer hygiene; the honest
// error path flashes and PRGs back to the case page.
func TestContinuationIssueFormRendersOneTimeURL(t *testing.T) {
	server := newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, issue: func(context.Context, casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error) {
		return casesAPI.ContinuationIssuance{
			Outcome: casesAPI.WorkOutcome{Command: "issue_continuation_link", ContinuationIssued: true},
			URL:     "/continue/testtoken", Token: "testtoken", Expires: time.Unix(1, 0).UTC(),
		}, nil
	}}, tenantNameIdentity{})
	form := url.Values{
		"purpose_fact_correction": {"on"},
		"question":                {"Верный VIN?"},
		"explanation":             {"Проверим вместе"},
		"participant_ref":         {"wa:7999"},
		"ttl_minutes":             {"30"},
		"consumption_limit":       {"1"},
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, continuationFormRequest(http.MethodPost,
		"/app/tenants/01994f64-6c00-7000-8000-000000000011/cases/01994f64-6c00-7000-8000-00000000aa01/work-items/01994f64-6c00-7000-8000-00000000bb02/continuation-links", form))
	if recorder.Code != http.StatusOK {
		t.Fatalf("the issuance re-renders the case page (no PRG — the one-time material), status = %d", recorder.Code)
	}
	page := recorder.Body.String()
	if !strings.Contains(page, "/continue/testtoken") || !strings.Contains(page, "Ссылка продолжения выдана") || !strings.Contains(page, "Скопируйте ссылку сейчас") {
		t.Fatalf("the re-render carries the one-time URL card, page = %s", page)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("the one-time card never caches and never leaks through a referrer")
	}
	// The replayed form issuance (the browser refresh of the same material
	// — the same derived key): the honest mask — the re-render WITHOUT the
	// one-time card (the token is never recoverable from the stored
	// outcome; the page honestly says nothing about the URL).
	server = newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, issue: func(context.Context, casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error) {
		return casesAPI.ContinuationIssuance{Outcome: casesAPI.WorkOutcome{Command: "issue_continuation_link", Replay: true, ContinuationIssued: true}}, nil
	}}, tenantNameIdentity{})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, continuationFormRequest(http.MethodPost,
		"/app/tenants/01994f64-6c00-7000-8000-000000000011/cases/01994f64-6c00-7000-8000-00000000aa01/work-items/01994f64-6c00-7000-8000-00000000bb02/continuation-links", form))
	if recorder.Code != http.StatusOK {
		t.Fatalf("the replayed issuance re-renders the case page honestly, status = %d", recorder.Code)
	}
	if replayPage := recorder.Body.String(); strings.Contains(replayPage, "continuation-issued") || strings.Contains(replayPage, "Ссылка продолжения выдана") {
		t.Fatalf("the replayed issuance must render NO one-time card (the honest mask), page = %s", replayPage)
	}
	// The honest error path: the flash + the PRG return to the case page.
	server = newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, issue: func(context.Context, casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error) {
		return casesAPI.ContinuationIssuance{}, domain.ErrForbidden
	}}, tenantNameIdentity{})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, continuationFormRequest(http.MethodPost,
		"/app/tenants/01994f64-6c00-7000-8000-000000000011/cases/01994f64-6c00-7000-8000-00000000aa01/work-items/01994f64-6c00-7000-8000-00000000bb02/continuation-links", form))
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("the error path is the PRG flash, status = %d", recorder.Code)
	}
	// The flash cookie carries the honest scope words.
	for _, cookie := range recorder.Result().Cookies() {
		if strings.Contains(url.QueryEscape(cookie.Value), "review") || strings.Contains(cookie.Value, "review") {
			return
		}
	}
	t.Fatalf("the flash cookie carries the honest scope refusal")
}

// continuationTokenOf renders one honest token of the tests (the real
// domain codec — the public page parses the tenant half for the header).
func continuationTokenOf(t *testing.T) string {
	t.Helper()
	random, err := casesDomain.RandomContinuationSecret()
	if err != nil {
		t.Fatal(err)
	}
	token, err := casesDomain.EncodeContinuationToken(testTenantID, random)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// TestContinuationPublicPageWireShape pins the public surface: the valid
// page renders the header (the tenant name of the identity service and
// the role constant), the question, the facts and the three actions; the
// invalid word renders the single generic refusal (404 — no cause
// disclosure); the POST actions are PRG; the validation error re-renders
// with the preserved input.
func TestContinuationPublicPageWireShape(t *testing.T) {
	token := continuationTokenOf(t)
	server := newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, surface: func(_ context.Context, gotToken string, _ time.Time) (casesAPI.ContinuationSurface, error) {
		if gotToken != token {
			t.Fatalf("unexpected token: %q", gotToken)
		}
		return casesAPI.ContinuationSurface{
			Kind: casesAPI.ContinuationSurfaceValid, Explanation: "Проверим вместе данные", Question: "Верный VIN?",
			Facts:     []casesAPI.ContinuationFactView{{FactID: id.MustParse("01994f64-6c00-7000-8000-00000000dd05"), MaterialKey: "vin", ValueText: "VIN0001"}},
			ExpiresAt: time.Unix(1, 0).UTC(), LimitLeft: 1,
		}, nil
	}}, tenantNameIdentity{name: "Магазин Олега"})
	request := httptest.NewRequest(http.MethodGet, "/continue/"+token, nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	page := recorder.Body.String()
	for _, word := range []string{"Магазин Олега", "ИИ-сотрудник", "Верный VIN?", "vin — VIN0001", "Отправить исправление", "Позвать человека", "Вернуться в разговор", "aria-live"} {
		if !strings.Contains(page, word) {
			t.Fatalf("the valid page carries %q, page = %s", word, page)
		}
	}
	if !strings.Contains(recorder.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("the protected page never caches")
	}
	// The consumed word: the terminal page (the double-GET stays stable).
	server = newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, surface: func(context.Context, string, time.Time) (casesAPI.ContinuationSurface, error) {
		return casesAPI.ContinuationSurface{Kind: casesAPI.ContinuationSurfaceConsumed}, nil
	}}, tenantNameIdentity{})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/continue/"+token, nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Материал отправлен") {
		t.Fatalf("the consumed word renders the terminal page, status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	// The invalid word: the single generic refusal — the expired, the
	// revoked, the unknown and the foreign tokens are ONE page (404, no
	// cause disclosure, the safe path words).
	server = newContinuationTestServer(t, &continuationCases{fakeCases: &fakeCases{}, surface: func(context.Context, string, time.Time) (casesAPI.ContinuationSurface, error) {
		return casesAPI.ContinuationSurface{Kind: casesAPI.ContinuationSurfaceInvalid}, nil
	}}, tenantNameIdentity{})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/continue/"+token, nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("the generic refusal renders 404, status = %d", recorder.Code)
	}
	refusal := recorder.Body.String()
	for _, word := range []string{"недействительна или истекла", "по каналу обращения"} {
		if !strings.Contains(refusal, word) {
			t.Fatalf("the generic refusal carries %q, page = %s", word, refusal)
		}
	}
	for _, cause := range []string{"отозвана", "expired", "revoked", "owner"} {
		if strings.Contains(refusal, cause) {
			t.Fatalf("the generic refusal never discloses the cause %q, page = %s", cause, refusal)
		}
	}
}

// TestContinuationPublicActionsWireShape pins the PRG contract of the
// three client actions: the success redirects with the landing flag; the
// dead-link refusal redirects to the plain GET (the derived page answers
// — the double-click of the limit is the terminal page); the validation
// error re-renders the form with the preserved input (the data never
// disappears).
func TestContinuationPublicActionsWireShape(t *testing.T) {
	token := continuationTokenOf(t)
	server := newContinuationTestServer(t, &continuationCases{
		fakeCases: &fakeCases{},
		surface: func(context.Context, string, time.Time) (casesAPI.ContinuationSurface, error) {
			return casesAPI.ContinuationSurface{Kind: casesAPI.ContinuationSurfaceValid, Explanation: "П", Question: "В",
				Facts: []casesAPI.ContinuationFactView{{FactID: id.MustParse("01994f64-6c00-7000-8000-00000000dd05"), MaterialKey: "vin", ValueText: "VIN0001"}}, LimitLeft: 1}, nil
		},
		correct: func(_ context.Context, gotToken string, factID id.UUID, raw []byte, _ time.Time) (int64, error) {
			if gotToken != token {
				t.Fatalf("unexpected token %q", gotToken)
			}
			if string(raw) != `"VIN0009"` {
				t.Fatalf("the text modality wraps the client value into one JSON string, got %s", raw)
			}
			return 2, nil
		},
		human:   func(context.Context, string, time.Time) error { return nil },
		return_: func(context.Context, string, time.Time) error { return nil },
	}, tenantNameIdentity{})
	form := url.Values{"fact_id": {"01994f64-6c00-7000-8000-00000000dd05"}, "value": {"VIN0009"}}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, continuationFormRequest(http.MethodPost, "/continue/"+token+"/correct", form))
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/continue/"+token+"?submitted=1" {
		t.Fatalf("the correction is PRG with the landing flag, status = %d, location = %s", recorder.Code, recorder.Header().Get("Location"))
	}
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, continuationFormRequest(http.MethodPost, "/continue/"+token+"/human", url.Values{}))
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/continue/"+token+"?human=1" {
		t.Fatalf("the human request is PRG with the landing flag, location = %s", recorder.Header().Get("Location"))
	}
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, continuationFormRequest(http.MethodPost, "/continue/"+token+"/return", url.Values{}))
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/continue/"+token+"?returned=1" {
		t.Fatalf("the return is PRG with the landing flag, location = %s", recorder.Header().Get("Location"))
	}
	// The landing announcement of the submitted flag.
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/continue/"+token+"?submitted=1", nil))
	if !strings.Contains(recorder.Body.String(), "оператор проверит") {
		t.Fatalf("the landing announces the honest submitted word, body = %s", recorder.Body.String())
	}
	// The dead-link correction: the redirect to the plain GET (the
	// derived page is the honest answer — the double-click of the limit).
	server = newContinuationTestServer(t, &continuationCases{
		fakeCases: &fakeCases{},
		correct: func(context.Context, string, id.UUID, []byte, time.Time) (int64, error) {
			return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: casesDomain.ContinuationNotUsableText}
		},
	}, tenantNameIdentity{})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, continuationFormRequest(http.MethodPost, "/continue/"+token+"/correct", form))
	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/continue/"+token {
		t.Fatalf("the dead-link action redirects to the derived page, status = %d, location = %s", recorder.Code, recorder.Header().Get("Location"))
	}
	// The validation error: the re-render with the preserved input (400 —
	// the data never disappears).
	server = newContinuationTestServer(t, &continuationCases{
		fakeCases: &fakeCases{},
		surface: func(context.Context, string, time.Time) (casesAPI.ContinuationSurface, error) {
			return casesAPI.ContinuationSurface{Kind: casesAPI.ContinuationSurfaceValid, Explanation: "П", Question: "В",
				Facts: []casesAPI.ContinuationFactView{{FactID: id.MustParse("01994f64-6c00-7000-8000-00000000dd05"), MaterialKey: "vin", ValueText: "VIN0001"}}, LimitLeft: 1}, nil
		},
		correct: func(context.Context, string, id.UUID, []byte, time.Time) (int64, error) {
			return 0, casesDomain.WorkRuleError{Rule: "continuation_correct", Reason: casesDomain.ContinuationFactNotCorrectableText}
		},
	}, tenantNameIdentity{})
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, continuationFormRequest(http.MethodPost, "/continue/"+token+"/correct", form))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("the validation refusal re-renders with 400, status = %d", recorder.Code)
	}
	page := recorder.Body.String()
	if !strings.Contains(page, "подтверждённый факт") || !strings.Contains(page, "VIN0009") {
		t.Fatalf("the re-render preserves the input and names the rule, page = %s", page)
	}
}

// TestContinuationReplyLinkBindingWireShape pins the delivery binding of
// the sales reply: the link_id travels into the command (the machine
// path); a malformed uuid is the honest transport 400.
func TestContinuationReplyLinkBindingWireShape(t *testing.T) {
	var captured id.UUID
	server := newContinuationTestServer(t, &continuationCases{
		fakeCases: &fakeCases{},
		issue: func(context.Context, casesAPI.IssueContinuationLinkCommand) (casesAPI.ContinuationIssuance, error) {
			return casesAPI.ContinuationIssuance{}, nil
		},
	}, tenantNameIdentity{})
	// The fakeCases stub answers SendClientReply without capture: bind a
	// dedicated fake through the command assertion.
	fake := &replyLinkCases{captured: &captured}
	server = newContinuationTestServer(t, fake, tenantNameIdentity{})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost,
		"/v1/tenants/01994f64-6c00-7000-8000-000000000011/cases/01994f64-6c00-7000-8000-00000000aa01/client-replies",
		`{"conversation_id":"01994f64-6c00-7000-8000-00000000ee06","expected_epoch":1,"reply_class":"sales","content":"Ответ со ссылкой","link_id":"01994f64-6c00-7000-8000-00000000cc04"}`))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if captured.String() != "01994f64-6c00-7000-8000-00000000cc04" {
		t.Fatalf("the link id travels into the command, got %s", captured)
	}
	// The malformed uuid: the honest transport 400.
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, casesJSONRequest(http.MethodPost,
		"/v1/tenants/01994f64-6c00-7000-8000-000000000011/cases/01994f64-6c00-7000-8000-00000000aa01/client-replies",
		`{"conversation_id":"01994f64-6c00-7000-8000-00000000ee06","expected_epoch":1,"reply_class":"sales","content":"Ответ","link_id":"not-a-uuid"}`))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "link_id") {
		t.Fatalf("the malformed link id is the honest 400, status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

// replyLinkCases captures the link id of the reply command (the full
// existing fake embedded).
type replyLinkCases struct {
	*fakeCases
	captured *id.UUID
}

func (f *replyLinkCases) SendClientReply(_ context.Context, command casesAPI.SendClientReplyCommand) (casesAPI.WorkOutcome, error) {
	*f.captured = command.LinkID
	return casesAPI.WorkOutcome{Command: "send_client_reply", ReplyClass: command.ReplyClass}, nil
}
