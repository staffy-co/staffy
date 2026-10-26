package httptransport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 4.11 outcome wire shapes: the case panel rendering (the
// ten-stage funnel with the state WORDS, the evidence pins, the
// attribution of the first accepted inbound channel event, the outcome
// chain words, the unknown pins of the terminal stages OQ-4/OQ-3), the
// report page rendering (the funnel with the proven counts — the
// terminal stages speak «не измеряется», NEVER a number; the conversions
// between the PROVEN neighbours; the counters; the unknown share; the
// non-measurable rows as the reason WORDS of the dictionary; the
// Measurement Dictionary v1; the OQ-7 boundary word; the sample size and
// the period word beside EVERY number) and the honest degradation probes
// (the unwired contour renders the zero panel / the zero page, the rest
// of the surface stays alive).
// ---------------------------------------------------------------------------

// The fixture identities of the outcome wire world.
var (
	pilotOutcomeTenant = id.MustParse("01994f64-6c00-7000-8000-000000000012")
	pilotOutcomeCase   = id.MustParse("01994f64-6c00-7000-8000-00000000cc12")
	pilotOutcomeCase2  = id.MustParse("01994f64-6c00-7000-8000-00000000cd12")
	pilotOutcomeOper   = id.MustParse("01994f64-6c00-7000-8000-00000000dd12")
	pilotOutcomeWork   = id.MustParse("01994f64-6c00-7000-8000-00000000ee12")
)

// fakePilotOutcomeSurface is the scriptable PilotOutcomeService of the
// wire tests: the two reads answer the fixture fields (the derivation
// itself is pinned by the domain and app tests).
type fakePilotOutcomeSurface struct {
	outcome        actionDomain.PilotCaseOutcome
	report         actionDomain.PilotReport
	readOutcomeErr error
	readReportErr  error
}

func (f *fakePilotOutcomeSurface) ReadPilotCaseOutcome(context.Context, identityDomain.Principal, id.UUID, id.UUID, time.Time) (actionDomain.PilotCaseOutcome, error) {
	return f.outcome, f.readOutcomeErr
}

func (f *fakePilotOutcomeSurface) ReadPilotReport(context.Context, identityDomain.Principal, id.UUID, string, time.Time) (actionDomain.PilotReport, error) {
	return f.report, f.readReportErr
}

// newPilotOutcomeWireServer wires the server of the wire tests over the
// scriptable outcome surface (the report page route rides the
// constructor — the route is mounted for EVERY server).
func newPilotOutcomeWireServer(t *testing.T, surface *fakePilotOutcomeSurface) *Server {
	t.Helper()
	identity := fakeCorrectionWireIdentity{fakeIdentity{
		create: func(identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
			return identityAPI.CreateTenantResult{}, nil
		}, workspace: func(id.UUID) (identityDomain.Workspace, error) {
			return identityDomain.Workspace{}, nil
		}}}
	server, err := New(identity, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.pilotOutcome = surface
	return server
}

// newPilotOutcomeUnwiredServer builds the server of the degradation
// probes WITHOUT any outcome surface (the honest zero render).
func newPilotOutcomeUnwiredServer(t *testing.T) *Server {
	t.Helper()
	identity := fakeCorrectionWireIdentity{fakeIdentity{
		create: func(identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
			return identityAPI.CreateTenantResult{}, nil
		}, workspace: func(id.UUID) (identityDomain.Workspace, error) {
			return identityDomain.Workspace{}, nil
		}}}
	server, err := New(identity, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return server
}

// pilotOutcomeFullMaterial builds the full funnel material of the wire
// fixture (the honest derivation input): the accepted inbound attribution
// event, the confirmed vehicle profile, the proven fitment position, the
// committed order receipt with the approved verification and the sent
// supplier fact, the handoff request, the corrections and the recovery
// case.
func pilotOutcomeFullMaterial() actionDomain.PilotCaseMaterial {
	at := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	return actionDomain.PilotCaseMaterial{
		CaseFound: true,
		CaseID:    pilotOutcomeCase,
		CaseNo:    42,
		CreatedAt: at,
		InboundEvents: []actionDomain.PilotInboundEvent{{
			Provider: "jivo", Channel: "chat", ProviderEventID: "evt-42", ReceivedAt: at,
		}},
		Vehicle: actionDomain.PilotVehicleMaterial{
			IdentificationSatisfied: true,
			ConfirmedKeys:           []string{"vin", "license_plate"},
			LastConfirmedAt:         at.Add(10 * time.Minute),
		},
		Fitment: actionDomain.PilotFitmentMaterial{Positions: []actionDomain.PilotFitmentPositionMaterial{{
			PositionKey: "front-left", Status: "proven", ProvenAt: at.Add(20 * time.Minute),
		}}},
		WorkItemsCount:       1,
		PartRequestConfirmed: true,
		HandoffRequests:      1,
		Corrections:          2,
		RecoveryCases:        1,
		Orders: []actionDomain.PilotOrderMaterial{{
			Operation: actionDomain.OrderStatusOperationMaterial{
				Found: true, OperationID: pilotOutcomeOper, WorkItemID: pilotOutcomeWork,
				Receipt: &actionDomain.OrderStatusReceiptPin{
					ReceiptVersionNo: 1, ExternalOrderID: "ABCP-ORDER-42", CreatedAt: at.Add(30 * time.Minute),
				},
			},
			Verifications: []actionDomain.OrderStatusVerificationMaterial{{
				Events: []actionDomain.OrderVerificationEvent{{
					VerificationNo: 1, EventNo: 2, Kind: actionDomain.OrderVerificationEventApproved, CreatedAt: at.Add(40 * time.Minute),
				}},
			}},
			Facts: []actionDomain.OrderSupplierFact{{
				FactNo: 1, Kind: actionDomain.OrderSupplierFactSent, SupplierOrderID: "SUP-42", HappenedAt: at.Add(50 * time.Minute),
			}},
		}},
	}
}

// pilotOutcomeWireReport builds the full report fixture over the honest
// aggregate fold: the full case plus one fresh case of the same period
// (the inquiry stage is proven for both, the rest of the ladder is the
// honest not_reached of the fresh case).
func pilotOutcomeWireReport() actionDomain.PilotReport {
	full := actionDomain.DeriveCaseOutcome(pilotOutcomeFullMaterial())
	fresh := actionDomain.DeriveCaseOutcome(actionDomain.PilotCaseMaterial{
		CaseFound: true,
		CaseID:    pilotOutcomeCase2,
		CaseNo:    43,
		CreatedAt: time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC),
	})
	return actionDomain.DerivePilotReport([]actionDomain.PilotCaseOutcome{full, fresh}, "период: 2026-10")
}

// TestPilotOutcomePanelRendersTheFunnel pins the panel rendering: the
// ten stage words BY NAME, the state words as TEXT (доказано /
// неизвестно-ожидает — never a color only, AC-18), the evidence pins
// with the times and the sources, the terminal unknown pins (OQ-4/OQ-3),
// the attribution with the provider and the channel, the unknown
// campaign/referrer words, the outcome chain words and the table caption
// of the a11y skeleton.
func TestPilotOutcomePanelRendersTheFunnel(t *testing.T) {
	surface := &fakePilotOutcomeSurface{outcome: actionDomain.DeriveCaseOutcome(pilotOutcomeFullMaterial())}
	server := newPilotOutcomeWireServer(t, surface)
	panel, err := server.pilotOutcomePanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, pilotOutcomeTenant, pilotOutcomeCase)
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	if !panel.Wired || len(panel.Stages) != 10 {
		t.Fatalf("the panel is wired with the ten stages: %+v", panel)
	}
	if panel.PortsMissingWord != "" {
		t.Fatalf("the wired panel carries no degradation word: %q", panel.PortsMissingWord)
	}
	if !strings.Contains(panel.AttributionWord, "jivo") || !strings.Contains(panel.AttributionWord, "chat") {
		t.Fatalf("the attribution word carries the provider and the channel: %q", panel.AttributionWord)
	}
	if len(panel.ChainWords) != 9 {
		t.Fatalf("the outcome chain words: %d", len(panel.ChainWords))
	}
	// The template render: the section words, the stage table with the
	// caption, the state words as text, the unknown pins and the
	// attribution words.
	templates := decisionContextRenderTemplates(t)
	var page bytes.Buffer
	data := map[string]any{"TenantID": pilotOutcomeTenant.String(),
		"View": caseDetailPageView{CaseID: pilotOutcomeCase.String(), PilotOutcome: panel}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the outcome panel: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"Исход обращения",
		"case-outcome",
		// The ten stage words BY NAME (AC-2/AC-18: the stages read as text).
		"Обращение принято",
		"Профиль автомобиля подтверждён",
		"Применимость доказана",
		"Заказ клиента создан",
		"Проверка менеджером подтверждена",
		"Отправка поставщику записана",
		"Исполнен",
		"Оплачен",
		"Отменён",
		"Возвращён",
		// The state words as TEXT (never a color only — AC-18).
		"доказано (proven)",
		"неизвестно/ожидает (unknown_pending)",
		// The terminal unknown pins BY NAME (OQ-4/OQ-3 — AC-14).
		"OQ-4",
		"OQ-3",
		// The attribution of the first accepted inbound event (AC-1).
		"источник обращения: провайдер jivo",
		"канал chat",
		"ID события провайдера evt-42",
		actionDomain.PilotAttributionUnknownWord,
		// The outcome chain words (the AC-2 graph).
		"источник:",
		"кейс: создан (номер 42)",
		"внешний ID заказа: ABCP-ORDER-42",
		// The evidence pins with the external IDs and the times.
		"ABCP-ORDER-42",
		"SUP-42",
		"26.10.2026 09:00:00 UTC",
		actionDomain.PilotSourceOperationReceiptWord,
		actionDomain.PilotSourceSupplierFactWord,
		// The a11y skeleton: the table caption of the stage funnel.
		"<caption>Десять стадий воронки обращения",
	} {
		if !strings.Contains(rendered, pin) {
			t.Fatalf("the rendered page misses %q", pin)
		}
	}
}

// TestPilotOutcomePanelDegradation pins the honest degradation: an
// unwired outcome surface renders the zero panel (the page keeps the
// rest); the ports-missing word of the app gate renders the zero panel
// with the honest word on the view; a scope-forbidden member sees the
// zero panel too.
func TestPilotOutcomePanelDegradation(t *testing.T) {
	server := newPilotOutcomeUnwiredServer(t)
	panel, err := server.pilotOutcomePanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, pilotOutcomeTenant, pilotOutcomeCase)
	if err != nil {
		t.Fatalf("the unwired panel read fails: %v", err)
	}
	if panel.Wired || len(panel.Stages) != 0 {
		t.Fatalf("the unwired panel is the zero panel: %+v", panel)
	}
	// The ports-missing word: the zero panel with the honest word — the
	// section renders NOTHING, the rest of the case page stays alive.
	missing := &fakePilotOutcomeSurface{readOutcomeErr: errors.New(pilotOutcomePortsMissingWord)}
	server.pilotOutcome = missing
	panel, err = server.pilotOutcomePanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, pilotOutcomeTenant, pilotOutcomeCase)
	if err != nil {
		t.Fatalf("the ports-missing panel read fails: %v", err)
	}
	if panel.Wired || len(panel.Stages) != 0 {
		t.Fatalf("the ports-missing panel is the zero panel: %+v", panel)
	}
	if panel.PortsMissingWord != pilotOutcomePortsMissingWord {
		t.Fatalf("the ports-missing word: %q", panel.PortsMissingWord)
	}
	templates := decisionContextRenderTemplates(t)
	var page bytes.Buffer
	data := map[string]any{"TenantID": pilotOutcomeTenant.String(),
		"View": caseDetailPageView{CaseID: pilotOutcomeCase.String(), PilotOutcome: panel}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render without the outcome panel: %v", err)
	}
	rendered := page.String()
	if strings.Contains(rendered, "case-outcome") || strings.Contains(rendered, "Исход обращения") {
		t.Fatal("the degraded outcome panel renders nothing (the zero panel)")
	}
	if !strings.Contains(rendered, "Диалоги канала") {
		t.Fatal("the rest of the case page stays alive (the honest degradation)")
	}
	// The scope-forbidden read: the zero panel, the page keeps the rest.
	forbidden := &fakePilotOutcomeSurface{readOutcomeErr: actionDomain.ErrForbidden}
	server.pilotOutcome = forbidden
	panel, err = server.pilotOutcomePanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, pilotOutcomeTenant, pilotOutcomeCase)
	if err != nil {
		t.Fatalf("the forbidden panel read fails: %v", err)
	}
	if panel.Wired || len(panel.Stages) != 0 {
		t.Fatalf("the forbidden panel is the zero panel: %+v", panel)
	}
}

// TestPilotOutcomeReportPageRendersTheAggregate pins the report page
// rendering through the LIVE route: the OQ-7 boundary word (the
// «baseline» gate — AC-12), the sample line with the period word
// (AC-11), the funnel rows of the ten stages (the terminal rows speak
// «не измеряется» — NEVER a number), the conversion table with the
// numerator/denominator columns, the counters with the sample and the
// period inline, the unknown share, the non-measurable rows as the
// reason WORDS of the dictionary (no numeric values — AC-4/AC-5/AC-7/
// AC-8) and the Measurement Dictionary v1 rows (AC-9).
func TestPilotOutcomeReportPageRendersTheAggregate(t *testing.T) {
	surface := &fakePilotOutcomeSurface{report: pilotOutcomeWireReport()}
	server := newPilotOutcomeWireServer(t, surface)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/app/tenants/"+pilotOutcomeTenant.String()+"/pilot-outcome?period=2026-10", nil)
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	server.Handler().ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("the report page answers 200, got %d: %s", response.Code, response.Body.String())
	}
	page := response.Body.String()
	for _, pin := range []string{
		"Отчёт пилота автозапчастей",
		// The OQ-7 boundary word (AC-12 — no approved baselines, no
		// improvement claim; the correlation is not the causation).
		"baseline",
		"корреляция — не причинность",
		// The sample line and the period word (AC-11).
		"Выборка:",
		"период: 2026-10",
		// The funnel rows of the ten stages BY NAME.
		"Обращение принято",
		"Профиль автомобиля подтверждён",
		"Применимость доказана",
		"Заказ клиента создан",
		"Проверка менеджером подтверждена",
		"Отправка поставщику записана",
		"Исполнен",
		"Оплачен",
		"Отменён",
		"Возвращён",
		// The honest words of the funnel states (TEXT — AC-18).
		"доказано (proven)",
		"неизвестно/ожидает (unknown_pending)",
		// The conversion table headers with the numerator/denominator.
		"Числитель",
		"Знаменатель",
		// The counters with the sample and the period inline (AC-11).
		"Коррекции (контур 4-8): 2 — выборка: 2; период: 2026-10",
		"Передачи человеку (контур 2-13): 1 — выборка: 2; период: 2026-10",
		"Recovery-кейсы (контур 3-8): 1 — выборка: 2; период: 2026-10",
		// The unknown share line (AC-14 — the influence is visible).
		"Доля неизвестных исходов: 2 из 2",
		// The dictionary rows (AC-9).
		"case_volume",
		"v1",
		// The a11y skeleton.
		"К основному содержанию",
		"<caption>",
		// The honest read-cost word of the footer.
		pilotOutcomeReadCostWord,
	} {
		if !strings.Contains(page, pin) {
			t.Fatalf("the report page misses %q", pin)
		}
	}
	// The terminal rows NEVER carry a number in the count column: the
	// honest word «не измеряется» rides the row (AC-14 — the owning
	// sources live outside the contour).
	for _, terminal := range []string{"Исполнен", "Оплачен", "Отменён", "Возвращён"} {
		row := terminal + " (" + map[string]string{
			"Исполнен":  actionDomain.PilotStageFulfilled,
			"Оплачен":   actionDomain.PilotStagePaid,
			"Отменён":   actionDomain.PilotStageCancelled,
			"Возвращён": actionDomain.PilotStageReturned,
		}[terminal] + ")</th><td>неизвестно/ожидает (unknown_pending)</td><td>не измеряется</td>"
		if !strings.Contains(page, row) {
			t.Fatalf("the terminal row %q must speak the honest not-measured word, not a number", terminal)
		}
	}
	// The non-measurable rows are the reason WORDS of the dictionary —
	// the exact reasons render, no numeric values replace them.
	for _, indicator := range actionDomain.PilotNotMeasurableIndicators {
		if !strings.Contains(page, indicator.Title) {
			t.Fatalf("the report page misses the non-measurable title %q", indicator.Title)
		}
		if !strings.Contains(page, indicator.Reason) {
			t.Fatalf("the report page misses the dictionary reason of %q", indicator.ID)
		}
	}
}

// TestPilotOutcomeReportPageDegradation pins the honest zero page: the
// ports-missing word of the app gate (and the unwired contour) render
// the single panel with the word — NO tables, NO funnel.
func TestPilotOutcomeReportPageDegradation(t *testing.T) {
	missing := &fakePilotOutcomeSurface{readReportErr: errors.New(pilotOutcomePortsMissingWord)}
	server := newPilotOutcomeWireServer(t, missing)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/app/tenants/"+pilotOutcomeTenant.String()+"/pilot-outcome", nil)
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	server.Handler().ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("the degraded report page answers 200, got %d: %s", response.Code, response.Body.String())
	}
	assertPilotOutcomeZeroPage(t, response.Body.String())
	// The unwired contour: the same honest zero page (the route is
	// mounted for EVERY server, the handler degrades inside).
	unwired := newPilotOutcomeUnwiredServer(t)
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/app/tenants/"+pilotOutcomeTenant.String()+"/pilot-outcome?period=2026-10", nil)
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	unwired.Handler().ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("the unwired report page answers 200, got %d: %s", response.Code, response.Body.String())
	}
	assertPilotOutcomeZeroPage(t, response.Body.String())
}

// assertPilotOutcomeZeroPage pins the honest zero page: the degradation
// word renders, NO table and NO funnel word rides the page.
func assertPilotOutcomeZeroPage(t *testing.T, page string) {
	t.Helper()
	if !strings.Contains(page, pilotOutcomePortsMissingWord) {
		t.Fatalf("the zero page must speak the honest degradation word")
	}
	if strings.Contains(page, "datamap-table") || strings.Contains(page, "Воронка") || strings.Contains(page, "case_volume") {
		t.Fatal("the zero page renders no tables (the honest zero note only)")
	}
	if !strings.Contains(page, "Отчёт пилота автозапчастей") {
		t.Fatal("the zero page keeps its title (the honest surface)")
	}
}

// TestPilotOutcomeA11yProbes pins the a11y skeleton of both surfaces:
// every table carries its caption element, the state words are TEXT
// (never a color only — AC-18) and the standalone page carries the
// skip-link of the shell.
func TestPilotOutcomeA11yProbes(t *testing.T) {
	// The panel render: the stage table caption + the state words as text.
	surface := &fakePilotOutcomeSurface{outcome: actionDomain.DeriveCaseOutcome(pilotOutcomeFullMaterial())}
	server := newPilotOutcomeWireServer(t, surface)
	panel, err := server.pilotOutcomePanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, pilotOutcomeTenant, pilotOutcomeCase)
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	templates := decisionContextRenderTemplates(t)
	var page bytes.Buffer
	data := map[string]any{"TenantID": pilotOutcomeTenant.String(),
		"View": caseDetailPageView{CaseID: pilotOutcomeCase.String(), PilotOutcome: panel}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the outcome panel: %v", err)
	}
	rendered := page.String()
	if !strings.Contains(rendered, "<caption>Десять стадий воронки обращения: состояние словом") {
		t.Fatal("the stage table carries its caption (the a11y skeleton)")
	}
	if !strings.Contains(rendered, ">доказано (proven)<") || !strings.Contains(rendered, ">неизвестно/ожидает (unknown_pending)<") {
		t.Fatal("the state words are text cells of the table (never a color only — AC-18)")
	}
	// The report page render: four captioned tables + the state words as
	// text + the skip-link.
	reportSurface := &fakePilotOutcomeSurface{report: pilotOutcomeWireReport()}
	reportServer := newPilotOutcomeWireServer(t, reportSurface)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/app/tenants/"+pilotOutcomeTenant.String()+"/pilot-outcome", nil)
	request.Header.Set("X-Correlation-ID", testCorrelation.String())
	reportServer.Handler().ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("the report page answers 200, got %d: %s", response.Code, response.Body.String())
	}
	report := response.Body.String()
	if got := strings.Count(report, "<caption>"); got < 4 {
		t.Fatalf("the report page carries the captions of every table (funnel, conversions, non-measurable, dictionary), got %d", got)
	}
	if !strings.Contains(report, ">доказано (proven)<") || !strings.Contains(report, ">неизвестно/ожидает (unknown_pending)<") {
		t.Fatal("the report state words are text cells of the table (never a color only — AC-18)")
	}
	if !strings.Contains(report, "К основному содержанию") {
		t.Fatal("the report page carries the skip-link of the a11y shell")
	}
}
