package httptransport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	"github.com/staffy-co/staffy/internal/identity/api"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 3.4 transport tests: the fake serves the honest refusals of
// the gates and the scripted verdicts of the AD-7 ladder (the gate ORDER
// and the counters are the app's unit truth — these tests pin the MAPPING
// of every refusal onto its code and the wire/page shapes: the strict
// decode, the verbatim command capture, the verdict ladder of the status
// codes, the honest empty view, the thread 404, the AG-4 dry-run refusal,
// the primary-to-authorization transition of the page and the form
// origin/checkbox disciplines).
// ---------------------------------------------------------------------------

var (
	testPreviewCaseID = id.MustParse("01994f64-6c00-7000-8000-00000000f001")
	testPreviewWorkID = id.MustParse("01994f64-6c00-7000-8000-00000000f002")
	testPreviewID     = id.MustParse("01994f64-6c00-7000-8000-00000000f003")
)

// fakeActionPreviews serves the preview surface with the scripted answers
// and the captured commands.
type fakeActionPreviews struct {
	view      actionAPI.PreviewView
	viewErr   error
	submit    actionAPI.ProposalOutcome
	submitErr error
	edit      actionAPI.PreviewVersionOutcome
	editErr   error
	dryRun    actionAPI.DryRunOutcome
	dryRunErr error

	submitCalls int
	editCalls   int
	dryRunCalls int
	lastSubmit  *actionAPI.SubmitProposalCommand
	lastEdit    *actionAPI.NewPreviewVersionCommand
}

// The compile-time proof the fake satisfies the narrow transport contract.
var _ actionAPI.PreviewService = (*fakeActionPreviews)(nil)

func (f *fakeActionPreviews) SubmitProposal(_ context.Context, c actionAPI.SubmitProposalCommand) (actionAPI.ProposalOutcome, error) {
	f.submitCalls++
	commandCopy := c
	f.lastSubmit = &commandCopy
	if f.submitErr != nil {
		return actionAPI.ProposalOutcome{}, f.submitErr
	}
	return f.submit, nil
}

func (f *fakeActionPreviews) ReadPreviewView(context.Context, domain.Principal, id.UUID, id.UUID) (actionAPI.PreviewView, error) {
	if f.viewErr != nil {
		return actionAPI.PreviewView{}, f.viewErr
	}
	return f.view, nil
}

func (f *fakeActionPreviews) NewPreviewVersionFromEdit(_ context.Context, c actionAPI.NewPreviewVersionCommand) (actionAPI.PreviewVersionOutcome, error) {
	f.editCalls++
	commandCopy := c
	f.lastEdit = &commandCopy
	if f.editErr != nil {
		return actionAPI.PreviewVersionOutcome{}, f.editErr
	}
	return f.edit, nil
}

func (f *fakeActionPreviews) RecordDryRun(_ context.Context, c actionAPI.DryRunCommand) (actionAPI.DryRunOutcome, error) {
	f.dryRunCalls++
	if f.dryRunErr != nil {
		return actionAPI.DryRunOutcome{}, f.dryRunErr
	}
	return f.dryRun, nil
}

// newActionPreviewTestServer binds the fake through the production
// BindActionPreviews — the narrow interface is type-asserted exactly the
// way the real wiring does it (the composite cases fake carries the
// actor-and-case path anchors).
func newActionPreviewTestServer(t *testing.T, fake *fakeActionPreviews) *Server {
	t.Helper()
	identity := api.Service(fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
			return api.CreateTenantResult{}, nil
		},
		workspace: func(id.UUID) (domain.Workspace, error) {
			return domain.Workspace{}, nil
		},
	})
	server, err := New(identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID, StepUpVerified: true}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.BindCases(&fakeCases{})
	server.BindActionPreviews(fake)
	return server
}

// previewSubmitURL builds the machine submit path of the fixture anchors.
func previewSubmitURL() string {
	return "/v1/tenants/" + testTenantID.String() + "/cases/" + testPreviewCaseID.String() +
		"/work-items/" + testPreviewWorkID.String() + "/action-previews/proposals"
}

// previewSubmitBody builds the strict submit body of the fixture
// definition pin.
func previewSubmitBody() string {
	return `{"definition_id":"quote.send-price-quote","definition_version":"1.2.0",` +
		`"target_kind":"catalog_item","target_ref":"` + testPreviewID.String() + `",` +
		`"typed_input":{"product_ref":"SKU-42"},"owner_epoch":1}`
}

// TestPreviewReadServesTheViewAndTheHonestEmptyState — the machine read
// answers the view wrapper with the honest empty threads (never a null);
// the 404 of the missing case does not disclose the surface.
func TestPreviewReadServesTheViewAndTheHonestEmptyState(t *testing.T) {
	fake := &fakeActionPreviews{view: actionAPI.PreviewView{
		TenantID: testTenantID, CaseID: testPreviewCaseID,
		PanelText: "panel", Threads: []actionAPI.PreviewThreadView{},
		WorkItemIDs: []id.UUID{testPreviewWorkID},
	}}
	server := newActionPreviewTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/v1/tenants/"+testTenantID.String()+"/cases/"+testPreviewCaseID.String()+"/action-previews", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("чтение обязано отвечать 200, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"threads":[]`) {
		t.Fatalf("пустой threads обязан быть честным пустым списком, никогда null: %s", body)
	}
	if !strings.Contains(body, `"panel_text":"panel"`) {
		t.Fatalf("обёртка обязана нести вид: %s", body)
	}
}

// TestPreviewThreadReadAnswersTheHonest404OfAnUnknownThread — the thread
// read of a missing identity is the plain 404 (no disclosure).
func TestPreviewThreadReadAnswersTheHonest404OfAnUnknownThread(t *testing.T) {
	fake := &fakeActionPreviews{view: actionAPI.PreviewView{Threads: []actionAPI.PreviewThreadView{}}}
	server := newActionPreviewTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/v1/tenants/"+testTenantID.String()+"/cases/"+testPreviewCaseID.String()+"/action-previews/"+testPreviewID.String(), nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("неизвестный поток обязан отвечать 404, пришло %d", recorder.Code)
	}
}

// TestPreviewSubmitMapsTheVerdictLadderOntoTheStatusCodes — the AD-7
// ladder rides the status codes: 400 of the first typed-invalid attempt
// with the FULL word list, 409 of the terminal repeat, 403 of the policy
// block of a typed-valid attempt (the exact reason word), 200 of the
// passing build with the version pins.
func TestPreviewSubmitMapsTheVerdictLadderOntoTheStatusCodes(t *testing.T) {
	cases := []struct {
		name    string
		outcome actionAPI.ProposalOutcome
		status  int
	}{
		{"первая невалидность — 400 с перечнем", actionAPI.ProposalOutcome{
			Command: "submit_proposal", Verdict: actionDomain.VerdictInvalid, AttemptNo: 1,
			Violations: []string{"Поле «product_ref» обязано быть непустым."},
		}, http.StatusBadRequest},
		{"терминальный повтор — 409", actionAPI.ProposalOutcome{
			Command: "submit_proposal", Verdict: actionDomain.VerdictRejected, AttemptNo: 2,
			Violations: []string{"Поле «product_ref» обязано быть непустым."},
		}, http.StatusConflict},
		{"policy-блок валидной попытки — 403 со словом причины", actionAPI.ProposalOutcome{
			Command: "submit_proposal", Verdict: actionDomain.VerdictValid, AttemptNo: 1,
			BlockedReason: actionDomain.PolicyReasonControlMode,
		}, http.StatusForbidden},
		{"проходящая сборка — 200 с пинами версии", actionAPI.ProposalOutcome{
			Command: "submit_proposal", Verdict: actionDomain.VerdictValid, AttemptNo: 1,
			PreviewID: testPreviewID, VersionNo: 1, PreviewDigest: "64hex-digest-of-the-exact-material-bytes",
		}, http.StatusOK},
	}
	for _, probe := range cases {
		t.Run(probe.name, func(t *testing.T) {
			fake := &fakeActionPreviews{submit: probe.outcome}
			server := newActionPreviewTestServer(t, fake)
			request := httptest.NewRequest(http.MethodPost, previewSubmitURL(), strings.NewReader(previewSubmitBody()))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			server.Handler().ServeHTTP(recorder, request)
			if recorder.Code != probe.status {
				t.Fatalf("вердикт %q обязан отображаться в %d, пришло %d: %s", probe.outcome.Verdict, probe.status, recorder.Code, recorder.Body.String())
			}
			if fake.submitCalls != 1 {
				t.Fatalf("команда обязана дойти до сервиса один раз, дошла %d", fake.submitCalls)
			}
			if fake.lastSubmit == nil || fake.lastSubmit.DefinitionID != "quote.send-price-quote" || fake.lastSubmit.OwnerEpoch != 1 {
				t.Fatalf("команда обязана нести дословные пины: %+v", fake.lastSubmit)
			}
			if fake.lastSubmit.Target.Kind != actionDomain.TargetCatalogItem || fake.lastSubmit.Target.Ref != testPreviewID {
				t.Fatalf("цель обязана прийти дословно: %+v", fake.lastSubmit.Target)
			}
			if fake.lastSubmit.TypedInput["product_ref"] != "SKU-42" {
				t.Fatalf("типовой материал обязан прийти дословно: %+v", fake.lastSubmit.TypedInput)
			}
			if probe.outcome.Verdict == actionDomain.VerdictInvalid && !strings.Contains(recorder.Body.String(), "product_ref") {
				t.Fatalf("400 обязан нести полный перечень нарушений: %s", recorder.Body.String())
			}
		})
	}
}

// TestPreviewSubmitDecodeIsStrict — one JSON object, known fields only,
// the exact Content-Type; every deviation is the honest 400 BEFORE the
// service (the counter stays ZERO).
func TestPreviewSubmitDecodeIsStrict(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		body        string
	}{
		{"чужой Content-Type", "text/plain", previewSubmitBody()},
		{"данные после объекта", "application/json", previewSubmitBody() + `{"smuggled":1}`},
		{"неизвестное поле тела", "application/json", `{"definition_id":"a","definition_version":"1.2.0","target_kind":"catalog_item","target_ref":"` + testPreviewID.String() + `","typed_input":{},"owner_epoch":1,"smuggled":"значение"}`},
	}
	for _, probe := range cases {
		t.Run(probe.name, func(t *testing.T) {
			fake := &fakeActionPreviews{}
			server := newActionPreviewTestServer(t, fake)
			request := httptest.NewRequest(http.MethodPost, previewSubmitURL(), strings.NewReader(probe.body))
			request.Header.Set("Content-Type", probe.contentType)
			recorder := httptest.NewRecorder()
			server.Handler().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("строгий decode обязан отказать 400, пришло %d", recorder.Code)
			}
			if fake.submitCalls != 0 {
				t.Fatalf("отказ decode обязан происходить ДО сервиса, сервис видел %d вызовов", fake.submitCalls)
			}
		})
	}
}

// TestPreviewSubmitMapsTheHonestRefusals — the 404 of the missing case /
// definition, the 403 of the audit-only member with the app words, the
// 409 of the exhausted ladder and the raced versions, the 500 of the
// digest integrity alarm.
func TestPreviewSubmitMapsTheHonestRefusals(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{"не-участник — 404", domain.ErrNotFound, http.StatusNotFound, "не найдены"},
		{"audit-only — 403 словами скоупа", fmt.Errorf("%w: %s", domain.ErrForbidden, "требуют полномочия review или configuration"), http.StatusForbidden, "review или configuration"},
		{"исчерпанные попытки — 409", actionPort.ErrAttemptExhausted, http.StatusConflict, "исчерпаны"},
		{"двинутая лестница — 409", actionPort.ErrStalePreviewVersion, http.StatusConflict, "двинулась"},
	}
	for _, probe := range cases {
		t.Run(probe.name, func(t *testing.T) {
			fake := &fakeActionPreviews{submitErr: probe.err}
			server := newActionPreviewTestServer(t, fake)
			request := httptest.NewRequest(http.MethodPost, previewSubmitURL(), strings.NewReader(previewSubmitBody()))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			server.Handler().ServeHTTP(recorder, request)
			if recorder.Code != probe.status {
				t.Fatalf("отказ обязан отображаться в %d, пришло %d", probe.status, recorder.Code)
			}
			if !strings.Contains(recorder.Body.String(), probe.body) {
				t.Fatalf("тело отказа обязано нести честные слова «%s»: %s", probe.body, recorder.Body.String())
			}
		})
	}
	// The digest integrity alarm is the loud 500 (never masked).
	fake := &fakeActionPreviews{submitErr: actionPort.ErrPreviewIntegrity}
	server := newActionPreviewTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, previewSubmitURL(), strings.NewReader(previewSubmitBody()))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("digest-тревога обязана быть громкой 500, пришло %d", recorder.Code)
	}
}

// TestPreviewEditRunsTheFullChainBeforeAnyWrite — the edit command
// captures the typed pairs verbatim; the honest refusals answer the
// 400/403/409 words; the built version answers 200 with the pins.
func TestPreviewEditRunsTheFullChainBeforeAnyWrite(t *testing.T) {
	fake := &fakeActionPreviews{edit: actionAPI.PreviewVersionOutcome{
		Command: "edit_preview_version", PreviewID: testPreviewID, VersionNo: 2, Supersedes: 1, PreviewDigest: "64hex",
	}}
	server := newActionPreviewTestServer(t, fake)
	editURL := "/v1/tenants/" + testTenantID.String() + "/cases/" + testPreviewCaseID.String() +
		"/action-previews/" + testPreviewID.String() + "/versions"
	request := httptest.NewRequest(http.MethodPost, editURL, strings.NewReader(`{"typed_input":{"product_ref":"SKU-43"}}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("правка обязана отвечать 200, пришло %d: %s", recorder.Code, recorder.Body.String())
	}
	if fake.editCalls != 1 || fake.lastEdit == nil {
		t.Fatalf("команда правки обязана дойти до сервиса один раз: %d", fake.editCalls)
	}
	if fake.lastEdit.PreviewID != testPreviewID || fake.lastEdit.TypedInput["product_ref"] != "SKU-43" {
		t.Fatalf("правка обязана нести дословный материал: %+v", fake.lastEdit)
	}
}

// TestPreviewDryRunRefusesTheNilAdapterInWords — the AG-4 boundary of the
// production root rides the 400 with the honest words; the wired adapter
// answers the evidence.
func TestPreviewDryRunRefusesTheNilAdapterInWords(t *testing.T) {
	dryRunURL := "/v1/tenants/" + testTenantID.String() + "/cases/" + testPreviewCaseID.String() +
		"/action-previews/" + testPreviewID.String() + "/versions/1/dry-runs"
	fake := &fakeActionPreviews{dryRunErr: actionDomain.RuleError{Rule: "record_dry_run", Reason: actionDomain.DryRunUnsupportedText}}
	server := newActionPreviewTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, dryRunURL, nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("AG-4 отказ обязан быть 400, пришёл %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "не поддержан") {
		t.Fatalf("отказ обязан говорить честные слова адаптера: %s", recorder.Body.String())
	}
	fake = &fakeActionPreviews{dryRun: actionAPI.DryRunOutcome{Command: "record_dry_run", Result: actionDomain.DryRunResultValid}}
	server = newActionPreviewTestServer(t, fake)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, dryRunURL, nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "valid") {
		t.Fatalf("подключённый адаптер отвечает evidence: %d %s", recorder.Code, recorder.Body.String())
	}
}

// TestPreviewPageRendersThePrimaryTransitionAndTheAbsence — the page
// renders the card material in text, the PRIMARY action is the honest
// transition to the authorization words (the 3-5 contour absent — the
// interface never pretends), and NO dispatch verb exists anywhere on the
// page (the "no hidden dispatch" probe of AC-12).
func TestPreviewPageRendersThePrimaryTransitionAndTheAbsence(t *testing.T) {
	card := actionAPI.PreviewCardView{
		PreviewID: testPreviewID, VersionNo: 1,
		DefinitionID: "quote.send-price-quote", DefinitionVersion: "1.2.0",
		CaseID: testPreviewCaseID, WorkItemID: testPreviewWorkID,
		TargetKind: actionDomain.TargetCatalogItem, TargetRef: testPreviewID,
		TypedInput:               map[string]string{"product_ref": "SKU-42"},
		Partition:                actionDomain.PartitionStaffy,
		PartitionText:            actionDomain.ExecutionPartitionText(actionDomain.PartitionStaffy),
		Readiness:                actionDomain.PreviewReady,
		ReadinessText:            actionDomain.PreviewReadinessText(actionDomain.PreviewReady),
		Timeline:                 actionDomain.TimelinePreviewed,
		TimelineText:             actionDomain.TimelineText(actionDomain.TimelinePreviewed),
		Current:                  true,
		Digest:                   "64hex-digest",
		AuthorizationAbsenceText: actionDomain.AuthorizationAbsenceText,
		CreatedAt:                time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC),
	}
	fake := &fakeActionPreviews{view: actionAPI.PreviewView{
		TenantID: testTenantID, CaseID: testPreviewCaseID, PanelText: "панель",
		Threads:     []actionAPI.PreviewThreadView{{PreviewID: testPreviewID, Cards: []actionAPI.PreviewCardView{card}}},
		WorkItemIDs: []id.UUID{testPreviewWorkID},
	}}
	server := newActionPreviewTestServer(t, fake)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/app/tenants/"+testTenantID.String()+"/cases/"+testPreviewCaseID.String()+"/action-previews", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("страница обязана отвечать 200, пришло %d", recorder.Code)
	}
	page := recorder.Body.String()
	if !strings.Contains(page, "Перейти к подтверждению и авторизации") {
		t.Fatalf("primary-действие актуальной карточки — переход к confirm-форме контура 3-5: %s", page[:200])
	}
	if !strings.Contains(page, "/operations?confirm="+testPreviewID.String()+"@1") {
		t.Fatalf("primary-кнопка обязана вести в confirm-форму Operation Review с пином точной версии: %s", page[:200])
	}
	if !strings.Contains(page, actionDomain.AuthorizationAbsenceText) {
		t.Fatalf("страница обязана говорить слова контура 3-5: %s", page[:200])
	}
	for _, forbidden := range []string{"Отправить действие", "Выполнить действие", "dispatch"} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("скрытой отправки не существует — глагол «%s» не живёт на странице", forbidden)
		}
	}
	if !strings.Contains(page, "Timeline") || !strings.Contains(page, actionDomain.TimelinePreviewed) {
		t.Fatalf("страница обязана говорить Timeline словами: %s", page[:400])
	}
}

// TestPreviewFormRunsThePRGDiscipline — the proposal form demands the
// origin, parses the pairs honestly, lands PRG with the created flag; the
// invalid verdict re-renders through the flash with the honest words.
func TestPreviewFormRunsThePRGDiscipline(t *testing.T) {
	formURL := "/app/tenants/" + testTenantID.String() + "/cases/" + testPreviewCaseID.String() + "/action-previews/proposals"
	fake := &fakeActionPreviews{submit: actionAPI.ProposalOutcome{
		Command: "submit_proposal", Verdict: actionDomain.VerdictValid, AttemptNo: 1,
		PreviewID: testPreviewID, VersionNo: 1,
	}}
	server := newActionPreviewTestServer(t, fake)
	form := url.Values{}
	form.Set("work_item_id", testPreviewWorkID.String())
	form.Set("definition_pin", "quote.send-price-quote@1.2.0")
	form.Set("target_kind", actionDomain.TargetCatalogItem)
	form.Set("target_ref", testPreviewID.String())
	form.Set("typed_input", "product_ref=SKU-42")
	form.Set("owner_epoch", "1")
	request := httptest.NewRequest(http.MethodPost, formURL, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://"+request.Host)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("успешная форма обязана ответить PRG 303, пришло %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); !strings.Contains(location, "created=1") {
		t.Fatalf("PRG обязан нести флаг created: %s", location)
	}
	if fake.lastSubmit == nil || fake.lastSubmit.TypedInput["product_ref"] != "SKU-42" || fake.lastSubmit.OwnerEpoch != 1 {
		t.Fatalf("форма обязана разобрать пары и эпоху честно: %+v", fake.lastSubmit)
	}
	// The origin refusal: the cross-site form never reaches the service.
	fake = &fakeActionPreviews{}
	server = newActionPreviewTestServer(t, fake)
	request = httptest.NewRequest(http.MethodPost, formURL, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://evil.example")
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("чужой origin обязан отказать 403, пришло %d", recorder.Code)
	}
	if fake.submitCalls != 0 {
		t.Fatalf("origin-отказ происходит ДО сервиса, сервис видел %d", fake.submitCalls)
	}
	// The invalid verdict rides the flash with the honest words.
	fake = &fakeActionPreviews{submit: actionAPI.ProposalOutcome{
		Command: "submit_proposal", Verdict: actionDomain.VerdictInvalid, AttemptNo: 1,
		Violations: []string{"Поле «product_ref» обязано быть непустым."},
	}}
	server = newActionPreviewTestServer(t, fake)
	request = httptest.NewRequest(http.MethodPost, formURL, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://"+request.Host)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("невалидный вердикт rides PRG: %d", recorder.Code)
	}
	// The dry-run form demands the explicit confirmation (the UI
	// discipline the machine API does not carry).
	dryRunFormURL := "/app/tenants/" + testTenantID.String() + "/cases/" + testPreviewCaseID.String() +
		"/action-previews/" + testPreviewID.String() + "/versions/1/dry-runs"
	fake = &fakeActionPreviews{dryRun: actionAPI.DryRunOutcome{Command: "record_dry_run", Result: actionDomain.DryRunResultValid}}
	server = newActionPreviewTestServer(t, fake)
	request = httptest.NewRequest(http.MethodPost, dryRunFormURL, strings.NewReader(""))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://"+request.Host)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if fake.dryRunCalls != 0 {
		t.Fatalf("dry-run без чекбокса не доходит до сервиса, дошёл %d", fake.dryRunCalls)
	}
	request = httptest.NewRequest(http.MethodPost, dryRunFormURL, strings.NewReader("confirm_dry_run=yes"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://"+request.Host)
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther || !strings.Contains(recorder.Header().Get("Location"), "validated=1") {
		t.Fatalf("подтверждённый dry-run обязан ответить PRG validated: %d %s", recorder.Code, recorder.Header().Get("Location"))
	}
}

// TestPreviewUnboundSurfaceAnswersTheHonest404 — the unbound service
// keeps the surface absent (the 404, never a 500).
func TestPreviewUnboundSurfaceAnswersTheHonest404(t *testing.T) {
	identity := api.Service(fakeIdentity{
		create: func(api.CreateTenantCommand) (api.CreateTenantResult, error) {
			return api.CreateTenantResult{}, nil
		},
		workspace: func(id.UUID) (domain.Workspace, error) {
			return domain.Workspace{}, nil
		},
	})
	server, err := New(identity, fixedResolver{principal: domain.Principal{ID: testPrincipalID, StepUpVerified: true}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.BindCases(&fakeCases{})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/v1/tenants/"+testTenantID.String()+"/cases/"+testPreviewCaseID.String()+"/action-previews", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("непривязанная поверхность обязана отсутствовать (404), пришло %d", recorder.Code)
	}
}

// TestPreviewSubmitBodyIsBoundedAndJSON — the response of the passing
// build is the documented outcome wrapper (the wire contract of the
// OpenAPI test).
func TestPreviewSubmitBodyIsBoundedAndJSON(t *testing.T) {
	fake := &fakeActionPreviews{submit: actionAPI.ProposalOutcome{
		Command: "submit_proposal", Verdict: actionDomain.VerdictValid, AttemptNo: 1,
		PreviewID: testPreviewID, VersionNo: 1, PreviewDigest: "64hex",
	}}
	server := newActionPreviewTestServer(t, fake)
	request := httptest.NewRequest(http.MethodPost, previewSubmitURL(), strings.NewReader(previewSubmitBody()))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	var body struct {
		Proposal actionAPI.ProposalOutcome `json:"proposal"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("исход обязан быть документированной обёрткой proposal: %v (%s)", err, recorder.Body.String())
	}
	if body.Proposal.PreviewID != testPreviewID || body.Proposal.VersionNo != 1 {
		t.Fatalf("обёртка обязана нести пины построенной версии: %+v", body.Proposal)
	}
}
