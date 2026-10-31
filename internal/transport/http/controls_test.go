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

	"github.com/staffy-co/staffy/internal/control/api"
	controlDomain "github.com/staffy-co/staffy/internal/control/domain"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// fakeControlService is the transport fake of the control surface: it
// serves the fixture points and the honest refusals of the gates without
// running the app ladder (the ladder is the app's unit truth — the
// transport test pins the MAPPING of every refusal onto its code).
type fakeControlService struct {
	controls     []controlDomain.ScopedControl
	listErr      error
	setErr       error
	resumeErr    error
	effectiveErr error
	lastSet      *api.SetRequest
	lastResume   *api.ResumeRequest
	lastKeys     []controlDomain.ScopeKey
	setCalls     int
	resumeCalls  int
}

// The compile-time proof the fake satisfies the transport contract.
var _ api.ControlService = (*fakeControlService)(nil)

func (f *fakeControlService) List(context.Context, identityDomain.Principal, id.UUID) ([]controlDomain.ScopedControl, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]controlDomain.ScopedControl(nil), f.controls...), nil
}

func (f *fakeControlService) Set(_ context.Context, _ identityDomain.Principal, _ id.UUID, request api.SetRequest) (controlDomain.ScopedControl, error) {
	f.setCalls++
	requestCopy := request
	f.lastSet = &requestCopy
	if f.setErr != nil {
		return controlDomain.ScopedControl{}, f.setErr
	}
	return controlDomain.ScopedControl{
		ScopeKind: request.Kind, ScopeRef: request.Ref, Mode: request.Mode,
		Reason: request.Reason, ActorID: testPrincipalID.String(),
		Generation: 1, State: controlDomain.ControlStateActive,
		UpdatedAt: time.Unix(1, 0).UTC(),
	}, nil
}

func (f *fakeControlService) Resume(_ context.Context, _ identityDomain.Principal, _ id.UUID, request api.ResumeRequest) (controlDomain.ScopedControl, error) {
	f.resumeCalls++
	requestCopy := request
	f.lastResume = &requestCopy
	if f.resumeErr != nil {
		return controlDomain.ScopedControl{}, f.resumeErr
	}
	return controlDomain.ScopedControl{
		ScopeKind: request.Kind, ScopeRef: request.Ref,
		Reason: request.Reason, ActorID: testPrincipalID.String(),
		Generation: 2, State: controlDomain.ControlStateRemoved,
		UpdatedAt: time.Unix(1, 0).UTC(),
	}, nil
}

func (f *fakeControlService) EffectiveFor(_ context.Context, _ identityDomain.Principal, _ id.UUID, keys []controlDomain.ScopeKey) (controlDomain.EffectiveControl, error) {
	f.lastKeys = append([]controlDomain.ScopeKey(nil), keys...)
	if f.effectiveErr != nil {
		return controlDomain.EffectiveControl{}, f.effectiveErr
	}
	// The fake mirrors the app semantic: the intersection is computed over
	// the loaded rows at the read moment (fixedClock's Unix(1,0) — the
	// fixture windows are unbounded, so the moment does not filter them).
	return controlDomain.ComputeEffective(time.Unix(1, 0).UTC(), f.controls), nil
}

// fixtureControl builds one guard-valid control point of the fake world.
func fixtureControl(kind, ref, mode string, generation int64, state string) controlDomain.ScopedControl {
	return controlDomain.ScopedControl{
		ScopeKind:  kind,
		ScopeRef:   ref,
		Mode:       mode,
		Reason:     "фикстура: причина оператора",
		ActorID:    testPrincipalID.String(),
		PrevMode:   controlDomain.ControlModeAccept,
		Generation: generation,
		State:      state,
		UpdatedAt:  time.Unix(1, 0).UTC(),
	}
}

// controlsScopeWords mirrors the app layer wrap of the honest 403 (the
// byte-exact words of the ONE scope that opens the surface).
const controlsScopeWords = "Поверхность аварийного ограничения работы требует полномочия operations: точечная остановка и возобновление маршрутов — операторский материал, участнику без operations он не открывается до вычисления итогового режима."

// controlsForbiddenWrap mirrors the app refusal wrap.
func controlsForbiddenWrap() error {
	return fmt.Errorf("%w: %s", controlDomain.ErrForbidden, controlsScopeWords)
}

// newControlsTestServer binds the fake control service to a test server.
func newControlsTestServer(t *testing.T, fake *fakeControlService, resolver fixedResolver) *Server {
	t.Helper()
	identity := identityAPI.Service(fakeIdentity{
		create: func(identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
			return identityAPI.CreateTenantResult{}, nil
		},
		workspace: func(id.UUID) (identityDomain.Workspace, error) {
			return identityDomain.Workspace{}, nil
		},
	})
	server, err := New(identity, resolver, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.BindControls(fake)
	return server
}

// controlJSONRequest builds a machine JSON request with the correlation
// header of the fixture identity.
func controlJSONRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestControlsListAPIServesBothStatesWithViews(t *testing.T) {
	fake := &fakeControlService{controls: []controlDomain.ScopedControl{
		fixtureControl("connector", "montagio", controlDomain.ControlModePaused, 3, controlDomain.ControlStateActive),
		fixtureControl("channel", "email", controlDomain.ControlModeIntakeOnly, 2, controlDomain.ControlStateRemoved),
	}}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/controls", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Controls []struct {
			ScopeKind  string `json:"scope_kind"`
			ScopeRef   string `json:"scope_ref"`
			Mode       string `json:"mode"`
			Corrupted  bool   `json:"corrupted"`
			Generation int64  `json:"generation"`
			State      string `json:"state"`
		} `json:"controls"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Controls) != 2 {
		t.Fatalf("обе стороны истории обязаны приехать (active + removed), got %d", len(body.Controls))
	}
	first := body.Controls[0]
	if first.Mode != "paused" || first.Corrupted || first.State != "active" || first.Generation != 3 {
		t.Fatalf("карточка обязана нести слово режима/поколение/состояние: %+v", first)
	}
}

func TestControlsListAPIHonestEmptyArray(t *testing.T) {
	server := newControlsTestServer(t, &fakeControlService{}, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/controls", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("честная пустота = 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"controls":[]`) {
		t.Fatalf("пустой список обязан быть пустым массивом, не null: %s", w.Body.String())
	}
}

func TestControlsListAPIRequiresIdentity(t *testing.T) {
	server := newControlsTestServer(t, &fakeControlService{}, fixedResolver{})
	r := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/controls", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("не подтверждённая личность = 401, got %d", w.Code)
	}
}

func TestControlsNonMemberIsPlain404BeforeMaterial(t *testing.T) {
	fake := &fakeControlService{
		controls: []controlDomain.ScopedControl{fixtureControl("connector", "montagio", controlDomain.ControlModePaused, 1, controlDomain.ControlStateActive)},
		listErr:  controlDomain.ErrNotFound,
	}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/controls", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("не-участник = честный 404, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "montagio") {
		t.Fatalf("материал контроля не покидает сервис при 404: %s", w.Body.String())
	}
}

func TestControlsScopeRefusalIs403WithWordsBeforeMaterial(t *testing.T) {
	fake := &fakeControlService{
		controls:     []controlDomain.ScopedControl{fixtureControl("connector", "montagio", controlDomain.ControlModePaused, 1, controlDomain.ControlStateActive)},
		listErr:      controlsForbiddenWrap(),
		setErr:       controlsForbiddenWrap(),
		resumeErr:    controlsForbiddenWrap(),
		effectiveErr: controlsForbiddenWrap(),
	}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	for _, path := range []string{
		"/v1/tenants/" + testTenantID.String() + "/controls",
		"/v1/tenants/" + testTenantID.String() + "/controls/effective?scope=connector:montagio",
		"/app/tenants/" + testTenantID.String() + "/controls",
	} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s: участник без скоупа = 403, got %d body=%s", path, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "operations") {
			t.Fatalf("%s: 403 обязан говорить слово скоупа ДО материала: %s", path, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "montagio") {
			t.Fatalf("%s: материал контроля не покидает сервис при 403: %s", path, w.Body.String())
		}
	}
	r := controlJSONRequest(http.MethodPost, "/v1/tenants/"+testTenantID.String()+"/controls", `{"scope_kind":"connector","scope_ref":"montagio","mode":"paused","reason":"причина","expected_generation":0}`)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("мутация без скоупа = 403, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "operations") {
		t.Fatalf("403 мутации обязан говорить слово скоупа: %s", w.Body.String())
	}
}

func TestControlsSetAPIStrictDecode(t *testing.T) {
	server := newControlsTestServer(t, &fakeControlService{}, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	base := "/v1/tenants/" + testTenantID.String() + "/controls"
	probes := []struct {
		name        string
		contentType string
		body        string
	}{
		{"wrong content type", "text/plain", `{"scope_kind":"connector"}`},
		{"unknown field", "application/json", `{"scope_kind":"connector","unexpected":1}`},
		{"trailing data", "application/json", `{"scope_kind":"connector"} {}`},
		{"not an object", "application/json", `["connector"]`},
	}
	for _, probe := range probes {
		r := httptest.NewRequest(http.MethodPost, base, strings.NewReader(probe.body))
		r.Header.Set("Content-Type", probe.contentType)
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: строгий decode = 400, got %d body=%s", probe.name, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "control_rule") {
			t.Fatalf("%s: код отказа словарной семьи, got %s", probe.name, w.Body.String())
		}
	}
}

func TestControlsSetAPIRecordsTheRequestVerbatim(t *testing.T) {
	fake := &fakeControlService{}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	body := `{"scope_kind":"connector","scope_ref":"montagio","mode":"paused","reason":"остановка инцидента","expected_generation":5,"scheduled_start_at":"2026-10-19T12:00:00Z","expires_at":"2026-10-20T12:00:00Z"}`
	r := controlJSONRequest(http.MethodPost, "/v1/tenants/"+testTenantID.String()+"/controls", body)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if fake.setCalls != 1 || fake.lastSet == nil {
		t.Fatalf("сервис обязан получить ровно один запрос: %+v", fake.lastSet)
	}
	request := fake.lastSet
	if request.Kind != "connector" || request.Ref != "montagio" || request.Mode != "paused" || request.Reason != "остановка инцидента" || request.ExpectedGeneration != 5 {
		t.Fatalf("материал запроса обязан приехать дословно: %+v", request)
	}
	if request.ScheduledStartAt == nil || request.ScheduledStartAt.UTC().Format(time.RFC3339) != "2026-10-19T12:00:00Z" {
		t.Fatalf("начало окна обязано распарситься RFC3339: %+v", request.ScheduledStartAt)
	}
	if request.ExpiresAt == nil || request.ExpiresAt.UTC().Format(time.RFC3339) != "2026-10-20T12:00:00Z" {
		t.Fatalf("окончание окна обязано распарситься RFC3339: %+v", request.ExpiresAt)
	}
	if request.CorrelationID != testCorrelation {
		t.Fatalf("корреляция запроса выигрывает (контекст платформы): %+v", request.CorrelationID)
	}
	var answer struct {
		Control struct {
			Mode       string `json:"mode"`
			Generation int64  `json:"generation"`
			State      string `json:"state"`
		} `json:"control"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Control.Mode != "paused" || answer.Control.Generation != 1 || answer.Control.State != "active" {
		t.Fatalf("ответ обязан нести материальную точку: %+v", answer.Control)
	}
}

func TestControlsSetAPIConflictIs409WithHonestWords(t *testing.T) {
	current := fixtureControl("connector", "montagio", controlDomain.ControlModeHumanOnly, 7, controlDomain.ControlStateActive)
	fake := &fakeControlService{setErr: controlDomain.ConflictError{ScopeKind: "connector", ScopeRef: "montagio", Expected: 0, Current: current}}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	r := controlJSONRequest(http.MethodPost, "/v1/tenants/"+testTenantID.String()+"/controls", `{"scope_kind":"connector","scope_ref":"montagio","mode":"paused","reason":"причина","expected_generation":0}`)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusConflict {
		t.Fatalf("конфликт поколений = 409, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "generation_conflict") {
		t.Fatalf("код конфликта поколений, got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "подтвердите") {
		t.Fatalf("слова повторного подтверждения AC-11: %s", w.Body.String())
	}
}

func TestControlsSetAPIStepUpRequiredIs403(t *testing.T) {
	fake := &fakeControlService{setErr: identityDomain.ErrStepUpRequired}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	r := controlJSONRequest(http.MethodPost, "/v1/tenants/"+testTenantID.String()+"/controls", `{"scope_kind":"connector","scope_ref":"montagio","mode":"paused","reason":"причина","expected_generation":0}`)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("step-up = 403, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "step_up_required") {
		t.Fatalf("код step-up отказа, got %s", w.Body.String())
	}
}

func TestControlsSetAPIDictionaryRefusalIs400WithRuleWords(t *testing.T) {
	fake := &fakeControlService{setErr: controlDomain.RuleError{Rule: "mode", Reason: "неизвестный режим контроля: freeze"}}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	r := controlJSONRequest(http.MethodPost, "/v1/tenants/"+testTenantID.String()+"/controls", `{"scope_kind":"connector","scope_ref":"montagio","mode":"freeze","reason":"причина","expected_generation":0}`)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("словарный отказ = 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "control_rule") {
		t.Fatalf("код словарной семьи, got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "неизвестный режим контроля") {
		t.Fatalf("слова честной причины: %s", w.Body.String())
	}
}

func TestControlsResumeAPIStrictDecodeAndRecord(t *testing.T) {
	fake := &fakeControlService{}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	base := "/v1/tenants/" + testTenantID.String() + "/controls/resume"
	r := httptest.NewRequest(http.MethodPost, base, strings.NewReader(`{"scope_kind":"connector","scope_ref":"montagio","unexpected":true}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("строгий decode resume = 400, got %d", w.Code)
	}
	r = controlJSONRequest(http.MethodPost, base, `{"scope_kind":"connector","scope_ref":"montagio","reason":"инцидент закрыт","expected_generation":7}`)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if fake.lastResume == nil || fake.lastResume.Kind != "connector" || fake.lastResume.Ref != "montagio" || fake.lastResume.Reason != "инцидент закрыт" || fake.lastResume.ExpectedGeneration != 7 {
		t.Fatalf("resume обязан получить материал дословно: %+v", fake.lastResume)
	}
	var answer struct {
		Control struct {
			State string `json:"state"`
		} `json:"control"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Control.State != "removed" {
		t.Fatalf("снятие = state removed (история, не удаление): %+v", answer.Control)
	}
}

func TestControlsEffectiveAPIParsesRepeatedScopeParams(t *testing.T) {
	fake := &fakeControlService{controls: []controlDomain.ScopedControl{
		fixtureControl("connector", "montagio", controlDomain.ControlModePaused, 3, controlDomain.ControlStateActive),
		fixtureControl("tenant", testTenantID.String(), controlDomain.ControlModeHumanOnly, 2, controlDomain.ControlStateActive),
	}}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/controls/effective?scope=connector:montagio&scope=tenant:"+testTenantID.String(), nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(fake.lastKeys) != 2 {
		t.Fatalf("повторяемый параметр обязан стать двумя ключами: %+v", fake.lastKeys)
	}
	if fake.lastKeys[0].Kind != "connector" || fake.lastKeys[0].Ref != "montagio" || fake.lastKeys[1].Kind != "tenant" || fake.lastKeys[1].Ref != testTenantID.String() {
		t.Fatalf("ключи обязаны распарситься kind:ref: %+v", fake.lastKeys)
	}
	var answer struct {
		Effective struct {
			Mode         string   `json:"mode"`
			Reason       string   `json:"reason"`
			Intersection []string `json:"intersection"`
			Influencing  []struct {
				ScopeKind string `json:"scope_kind"`
				ScopeRef  string `json:"scope_ref"`
			} `json:"influencing"`
		} `json:"effective"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Effective.Mode != "paused" {
		t.Fatalf("ранг-максимум: paused бьёт human_only, got %s", answer.Effective.Mode)
	}
	if len(answer.Effective.Influencing) != 2 {
		t.Fatalf("провенанс ВСЕХ повлиявших (не только победителя): %+v", answer.Effective.Influencing)
	}
	if len(answer.Effective.Intersection) != 2 || answer.Effective.Intersection[0] != "human_only" || answer.Effective.Intersection[1] != "paused" {
		t.Fatalf("слова пересечения в порядке лестницы: %+v", answer.Effective.Intersection)
	}
}

func TestControlsEffectiveAPIRefusesMalformedAndMissingScope(t *testing.T) {
	server := newControlsTestServer(t, &fakeControlService{}, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	for _, query := range []string{"", "scope=connectormontagio", "scope="} {
		r := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+testTenantID.String()+"/controls/effective?"+query, nil)
		w := httptest.NewRecorder()
		server.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("query %q: бессмысленный скоуп = честный 400, got %d", query, w.Code)
		}
	}
}

func TestControlsPageRequiresIdentity(t *testing.T) {
	server := newControlsTestServer(t, &fakeControlService{}, fixedResolver{})
	r := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/controls", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("не подтверждённая личность = 401, got %d", w.Code)
	}
}

// controlFormRequest builds a browser form request (the Origin discipline
// of the membership forms).
func controlFormRequest(method, path string, form url.Values) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.com")
	return request
}

func TestControlsPageRendersCardsWithModeWordsAndWindow(t *testing.T) {
	fake := &fakeControlService{controls: []controlDomain.ScopedControl{
		fixtureControl("connector", "montagio", controlDomain.ControlModePaused, 3, controlDomain.ControlStateActive),
		fixtureControl("channel", "email", controlDomain.ControlModeIntakeOnly, 2, controlDomain.ControlStateRemoved),
	}}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/controls", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"Аварийное ограничение работы",
		"Контрольные точки",
		"Режим: paused",
		"полная остановка маршрута",
		"Скоуп: connector:montagio",
		"Владелец изменения: " + testPrincipalID.String(),
		"Поколение: 3",
		"Состояние: действует",
		"Причина: фикстура: причина оператора",
		"Окно действия: с без ограничения до бессрочно",
		"intake_only/no_promises",
		"приём данных без новых Обещаний",
		"Состояние: снято — история и аудит сохранены",
		"<details><summary>Снять контрольную точку</summary>",
		"name=\"confirm_resume\" value=\"yes\"",
		"Установить контрольную точку",
		"Вычислитель итогового режима",
		"name=\"confirm_dangerous\" value=\"yes\"",
		"идентификатор из живого реестра коннекторов",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("страница обязана нести %q: %s", want, body)
		}
	}
	if strings.Contains(body, "Контрольных точек нет") {
		t.Fatalf("карточки есть — пустое состояние не должно рендериться")
	}
}

func TestControlsPageEmptyStateExplainsReasonByteExact(t *testing.T) {
	server := newControlsTestServer(t, &fakeControlService{}, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/controls", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "Контрольных точек нет") {
		t.Fatalf("пустая страница обязана назвать отсутствие: %s", body)
	}
	if !strings.Contains(body, controlsEmptyText) {
		t.Fatalf("пустое состояние обязано объяснять причину словами байт-в-байт: %s", body)
	}
}

func TestControlsPageCorruptedModeReadsPausedWithMarker(t *testing.T) {
	corrupted := fixtureControl("connector", "montagio", "freeze", 3, controlDomain.ControlStateActive)
	fake := &fakeControlService{controls: []controlDomain.ScopedControl{corrupted}}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/controls", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "Режим: paused") {
		t.Fatalf("испорченное значение обязано читаться paused (fail-closed): %s", body)
	}
	if !strings.Contains(body, "Хранимое значение режима повреждено") {
		t.Fatalf("повреждение обязано быть названо честно: %s", body)
	}
}

func TestControlsPageCalculatorComputesIntersection(t *testing.T) {
	fake := &fakeControlService{controls: []controlDomain.ScopedControl{
		fixtureControl("connector", "montagio", controlDomain.ControlModePaused, 3, controlDomain.ControlStateActive),
		fixtureControl("tenant", testTenantID.String(), controlDomain.ControlModeHumanOnly, 2, controlDomain.ControlStateActive),
	}}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	query := url.Values{"keys": []string{"connector:montagio\ntenant:" + testTenantID.String()}}
	r := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/controls?"+query.Encode(), nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"Итоговый режим: paused",
		"connector:montagio",
		"tenant:" + testTenantID.String(),
		"Слова пересечения (порядок лестницы): human_only; paused",
		"Причина победившего правила: фикстура: причина оператора",
		"connector:montagio",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("вычислитель обязан нести %q: %s", want, body)
		}
	}
	if len(fake.lastKeys) != 2 {
		t.Fatalf("вычислитель обязан получить оба ключа: %+v", fake.lastKeys)
	}
}

func TestControlsSetFormRedirectsPRGToApplied(t *testing.T) {
	fake := &fakeControlService{}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID, StepUpVerified: true}})
	form := url.Values{
		"scope_kind":          []string{"connector"},
		"scope_ref":           []string{"montagio"},
		"mode":                []string{"paused"},
		"reason":              []string{"остановка инцидента"},
		"expected_generation": []string{"0"},
		"confirm_dangerous":   []string{"yes"},
	}
	r := controlFormRequest(http.MethodPost, "/app/tenants/"+testTenantID.String()+"/controls", form)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("успех формы = PRG 303, got %d body=%s", w.Code, w.Body.String())
	}
	if location := w.Header().Get("Location"); location != "/app/tenants/"+testTenantID.String()+"/controls?applied=1" {
		t.Fatalf("PRG-цель с флагом поколения: %q", location)
	}
	if fake.lastSet == nil || fake.lastSet.Mode != "paused" || fake.lastSet.Reason != "остановка инцидента" || fake.lastSet.ExpectedGeneration != 0 {
		t.Fatalf("сервис обязан получить форму дословно: %+v", fake.lastSet)
	}
}

func TestControlsSetFormNoticeCarriesGenerationAndFocusTarget(t *testing.T) {
	server := newControlsTestServer(t, &fakeControlService{}, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}})
	r := httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/controls?applied=4", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "поколение 4") {
		t.Fatalf("статус обязан нести поколение: %s", body)
	}
	if !strings.Contains(body, `tabindex="-1" id="page-notice"`) {
		t.Fatalf("фокус переводится на новый статус (AC-12): %s", body)
	}
	if !strings.Contains(body, "getElementById('page-notice').focus()") {
		t.Fatalf("скрипт фокуса статуса: %s", body)
	}
	r = httptest.NewRequest(http.MethodGet, "/app/tenants/"+testTenantID.String()+"/controls?resumed=5", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), "история и аудит сохранены") {
		t.Fatalf("статус снятия обязан говорить сохранение истории: %s", w.Body.String())
	}
}

func TestControlsSetFormDangerousModeRequiresCheckbox(t *testing.T) {
	fake := &fakeControlService{}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID, StepUpVerified: true}})
	form := url.Values{
		"scope_kind":          []string{"connector"},
		"scope_ref":           []string{"montagio"},
		"mode":                []string{"paused"},
		"reason":              []string{"остановка"},
		"expected_generation": []string{"0"},
	}
	r := controlFormRequest(http.MethodPost, "/app/tenants/"+testTenantID.String()+"/controls", form)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("опасный режим без чекбокса = 400 (дисциплина UI AC-12), got %d", w.Code)
	}
	if fake.setCalls != 0 {
		t.Fatalf("мутация НЕ доходит до сервиса без чекбокса: %d вызовов", fake.setCalls)
	}
	if !strings.Contains(w.Body.String(), "Опасный режим требует явного подтверждения") {
		t.Fatalf("слова честного отказа чекбокса: %s", w.Body.String())
	}
	// Мягкий режим (accept) чекбокса не требует.
	form.Set("mode", "accept")
	r = controlFormRequest(http.MethodPost, "/app/tenants/"+testTenantID.String()+"/controls", form)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("accept без чекбокса = успех, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestControlsSetFormConflictRendersCurrentMaterialForReconfirmation(t *testing.T) {
	current := fixtureControl("connector", "montagio", controlDomain.ControlModeHumanOnly, 7, controlDomain.ControlStateActive)
	fake := &fakeControlService{
		controls: []controlDomain.ScopedControl{current},
		setErr:   controlDomain.ConflictError{ScopeKind: "connector", ScopeRef: "montagio", Expected: 0, Current: current},
	}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID, StepUpVerified: true}})
	form := url.Values{
		"scope_kind":          []string{"connector"},
		"scope_ref":           []string{"montagio"},
		"mode":                []string{"paused"},
		"reason":              []string{"остановка"},
		"expected_generation": []string{"0"},
		"confirm_dangerous":   []string{"yes"},
	}
	r := controlFormRequest(http.MethodPost, "/app/tenants/"+testTenantID.String()+"/controls", form)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusConflict {
		t.Fatalf("конфликт формы = 409 рендер с материалом, got %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		"Конкурентное изменение",
		"Актуальный режим: human_only",
		"Поколение: 7",
		"value=\"7\"", // форма перезаполнена актуальным поколением — повторное подтверждение
		"Поколение изменилось",
		"Причина последнего изменения: фикстура: причина оператора",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("повторное подтверждение AC-11 обязано нести %q: %s", want, body)
		}
	}
}

func TestControlsSetFormStepUpWordsOnPage(t *testing.T) {
	fake := &fakeControlService{setErr: identityDomain.ErrStepUpRequired}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID, StepUpVerified: false}})
	form := url.Values{
		"scope_kind":          []string{"connector"},
		"scope_ref":           []string{"montagio"},
		"mode":                []string{"accept"},
		"reason":              []string{"причина"},
		"expected_generation": []string{"0"},
	}
	r := controlFormRequest(http.MethodPost, "/app/tenants/"+testTenantID.String()+"/controls", form)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("step-up = 403 рендер страницы, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "повторное подтверждение личности") {
		t.Fatalf("слова step-up на странице: %s", w.Body.String())
	}
}

func TestControlsResumeFormRequiresConfirmAndRedirectsPRG(t *testing.T) {
	fake := &fakeControlService{}
	server := newControlsTestServer(t, fake, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID, StepUpVerified: true}})
	base := "/app/tenants/" + testTenantID.String() + "/controls/resume"
	form := url.Values{
		"scope_kind":          []string{"connector"},
		"scope_ref":           []string{"montagio"},
		"reason":              []string{"инцидент закрыт"},
		"expected_generation": []string{"7"},
	}
	r := controlFormRequest(http.MethodPost, base, form)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("снятие без чекбокса = 400, got %d", w.Code)
	}
	if fake.resumeCalls != 0 {
		t.Fatalf("мутация НЕ доходит до сервиса без чекбокса: %d вызовов", fake.resumeCalls)
	}
	form.Set("confirm_resume", "yes")
	r = controlFormRequest(http.MethodPost, base, form)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("успех снятия = PRG 303, got %d body=%s", w.Code, w.Body.String())
	}
	if location := w.Header().Get("Location"); location != "/app/tenants/"+testTenantID.String()+"/controls?resumed=2" {
		t.Fatalf("PRG-цель снятия: %q", location)
	}
	if fake.lastResume == nil || fake.lastResume.ExpectedGeneration != 7 || fake.lastResume.Reason != "инцидент закрыт" {
		t.Fatalf("resume обязан получить форму дословно: %+v", fake.lastResume)
	}
}

func TestControlsSetFormRefusesForeignOrigin(t *testing.T) {
	server := newControlsTestServer(t, &fakeControlService{}, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID, StepUpVerified: true}})
	form := url.Values{
		"scope_kind": []string{"connector"},
		"scope_ref":  []string{"montagio"},
		"mode":       []string{"accept"},
		"reason":     []string{"причина"},
	}
	r := httptest.NewRequest(http.MethodPost, "/app/tenants/"+testTenantID.String()+"/controls", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "http://evil.example")
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("чужой origin = честный 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "origin") {
		t.Fatalf("слова происхождения формы: %s", w.Body.String())
	}
}
