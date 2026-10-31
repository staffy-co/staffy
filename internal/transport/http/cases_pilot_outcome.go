package httptransport

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// Story 4.11 (step-03): the «Исход обращения» panel of the case page and
// the standalone «Отчёт пилота» page — the READ-ONLY transport surface
// over the pure derivation of the pilot outcome contour. The panel
// carries the ten-stage funnel with the state WORDS (доказано /
// не достигнуто / неизвестно-ожидает — the text, never a color only,
// AC-18), the evidence words BY NAME with the times and the sources, the
// unknown pins of the terminal stages (OQ-4/OQ-3 — «не доказано и не
// опровергнуто», never zero and never a silent exclusion, AC-14), the
// attribution of the first accepted inbound channel event (the unknown
// campaign/referrer words — never restored by a guess, AC-1) and the
// outcome chain words (the AC-2 graph). The report page carries the
// funnel with the proven counts, the conversions between the PROVEN
// neighbours only (the unknown_pending stages never enter a denominator),
// the counters, the unknown share, the non-measurable indicators as
// WORDS of reasons (never numbers — AC-4/AC-5/AC-7/AC-8), the
// Measurement Dictionary v1 (AC-9) and the OQ-7 boundary word (AC-12);
// EVERY number rides the sample size and the period word (AC-11). The
// honest degradation: an unwired contour (a nil service or the
// ports-missing word of the app gate) renders the zero panel / the zero
// page — the rest of the surface stays alive (the 4.9/4.10 precedent,
// never a nil panic); a scope-forbidden member sees the zero panel the
// same honest way. NOTHING is written anywhere (AC-13 read-time).

// ---------------------------------------------------------------------------
// The case panel «Исход обращения» (the order status panel precedent)
// ---------------------------------------------------------------------------

// PilotOutcomePanelView is the flattened outcome panel of the case page:
// the ten-stage funnel rows (render-ready strings), the attribution
// words, the outcome chain words and the honest degradation word of the
// unwired contour.
type PilotOutcomePanelView struct {
	// Wired reports the LIVE contour: the outcome surface is bound and
	// readable — the section renders its honest boundary words even for a
	// case WITHOUT any order material (the not_reached ladder is the
	// interface truth).
	Wired bool
	// Stages are the rows of the ten-stage funnel (render-ready).
	Stages []PilotOutcomeStageRow
	// The attribution render-ready words (the first accepted inbound
	// channel event; the campaign/referrer unknown words).
	AttributionWord string
	CampaignWord    string
	ReferrerWord    string
	// ChainWords are the outcome chain words (the AC-2 graph).
	ChainWords []string
	// PortsMissingWord is the honest degradation word of the unwired
	// contour (the zero panel rides it — the page keeps the rest).
	PortsMissingWord string
}

// PilotOutcomeStageRow is ONE stage row of the funnel: the honest Russian
// word of the stage id, the state word (TEXT, never a color only), the
// evidence word, the time, the source word and the unknown pin word of
// the terminal stages.
type PilotOutcomeStageRow struct {
	Stage      string
	StageWord  string
	State      string
	StateWord  string
	Evidence   string
	TimeText   string
	SourceWord string
	PinWord    string
}

// pilotOutcomeStageWords is the honest Russian word of every stage id
// (the compile-time dictionary of the render — the stages read as text,
// never as bare ids only).
var pilotOutcomeStageWords = map[string]string{
	actionDomain.PilotStageInquiryAccepted:         "Обращение принято",
	actionDomain.PilotStageVehicleProfileConfirmed: "Профиль автомобиля подтверждён",
	actionDomain.PilotStageApplicabilityProven:     "Применимость доказана",
	actionDomain.PilotStageClientOrderCreated:      "Заказ клиента создан",
	actionDomain.PilotStageManagerVerified:         "Проверка менеджером подтверждена",
	actionDomain.PilotStageSupplierSent:            "Отправка поставщику записана",
	actionDomain.PilotStageFulfilled:               "Исполнен",
	actionDomain.PilotStagePaid:                    "Оплачен",
	actionDomain.PilotStageCancelled:               "Отменён",
	actionDomain.PilotStageReturned:                "Возвращён",
}

// pilotOutcomePortsMissingWord is the transport mirror of the honest
// refusal word of the unwired contour (the app gate speaks the same word
// — the zero page renders it, the zero panel carries it).
const pilotOutcomePortsMissingWord = "контур исхода обращения не подключён: сверьте конфигурацию поверхности"

// pilotOutcomeScopeWord is the honest scope word of the page (the review
// read gate — the words-first discipline of the operator surface family).
const pilotOutcomeScopeWord = "review — чтение исхода обращения и отчёта пилота; участнику без полномочия review контур отвечает отказом"

// pilotOutcomeReadCostWord is the honest read-cost word of the report
// footer (the listing re-read with the per-case reads — the AG-1
// envelope; batch reads are a future optimization, spoken honestly).
const pilotOutcomeReadCostWord = "Отчёт перечитывает листинг кейсов периода с по-кейсовыми чтениями (объём пилота мал — конверт AG-1); батч-чтения — будущая оптимизация."

// pilotOutcomeNotMeasuredWord is the honest word of the count column of
// the terminal stages — NEVER a number (the owning sources live outside
// the contour; a number would claim a measurement that does not exist).
const pilotOutcomeNotMeasuredWord = "не измеряется"

// isPilotOutcomePortsMissing reports the honest ports-missing refusal of
// the unwired contour (the app gate speaks the plain word — the word
// match mirrors it, never a typed sentinel).
func isPilotOutcomePortsMissing(err error) bool {
	return err != nil && strings.Contains(err.Error(), "контур исхода обращения не подключён")
}

// pilotOutcomeService resolves the Story 4.11 read surface over the LIVE
// bound action contour: an explicit BindPilotOutcome pin wins; otherwise
// the order status binding carries the SAME composite app Service (the
// composite implements the narrow pilot interface beside the status
// interface — the BindEmployee knowledge type-assert precedent), so the
// surface rides the existing production wiring of the action contour
// WITHOUT a new root call; a server without the action contour resolves
// NO service — the honest degradation answers (the zero panel, the zero
// page word).
func (s *Server) pilotOutcomeService() actionAPI.PilotOutcomeService {
	if s.pilotOutcome != nil {
		return s.pilotOutcome
	}
	if service, ok := s.orderStatus.(actionAPI.PilotOutcomeService); ok {
		return service
	}
	return nil
}

// BindPilotOutcome registers the Story 4.11 outcome surface: the narrow
// PilotOutcomeService type-assert over the action service (the
// OrderStatusService precedent). The case panel bind reads the resolved
// surface; the report page route is mounted by the constructor for EVERY
// server (the honest degradation of an unwired contour renders the zero
// page word inside the handler).
func (s *Server) BindPilotOutcome(service actionAPI.PilotOutcomeService) {
	s.pilotOutcome, _ = service.(actionAPI.PilotOutcomeService)
}

// pilotOutcomePanelOf reads the panel material with the honest
// degradation of the case page precedent: an unwired outcome surface (or
// the ports-missing word of the app gate) renders the zero panel — the
// page keeps the rest; a scope-forbidden member sees the zero panel too;
// every other failure fails the page honestly. The derivation NEVER
// fails on a case without order material — the funnel speaks the honest
// not_reached ladder (the absence is spoken, never masked).
func (s *Server) pilotOutcomePanelOf(ctx context.Context, actor identityDomain.Principal, tenant, caseID id.UUID) (PilotOutcomePanelView, error) {
	panel := PilotOutcomePanelView{}
	service := s.pilotOutcomeService()
	if service == nil {
		return panel, nil
	}
	outcome, err := service.ReadPilotCaseOutcome(ctx, actor, tenant, caseID, s.clock.Now().UTC())
	if err != nil {
		if isPilotOutcomePortsMissing(err) {
			panel.PortsMissingWord = pilotOutcomePortsMissingWord
			return panel, nil
		}
		if errors.Is(err, actionDomain.ErrForbidden) || isActionNotConfigured(err) {
			return panel, nil
		}
		return PilotOutcomePanelView{}, err
	}
	panel.Wired = true
	panel.Stages = pilotOutcomeStageRows(outcome.Stages)
	panel.AttributionWord = outcome.Attribution.Word
	panel.CampaignWord = outcome.Attribution.CampaignWord
	panel.ReferrerWord = outcome.Attribution.ReferrerWord
	panel.ChainWords = outcome.ChainWords
	return panel, nil
}

// pilotOutcomeStageRows flattens the stage evidence into the panel rows
// (the state word is TEXT — «доказано (proven)» / «не достигнуто
// (not_reached)» / «неизвестно/ожидает (unknown_pending)», never a color
// only; the proven times format through the shared helper, the zero time
// stays the honest empty word).
func pilotOutcomeStageRows(stages []actionDomain.PilotStageEvidence) []PilotOutcomeStageRow {
	rows := make([]PilotOutcomeStageRow, 0, len(stages))
	for _, stage := range stages {
		rows = append(rows, PilotOutcomeStageRow{
			Stage:      stage.Stage,
			StageWord:  pilotOutcomeStageWords[stage.Stage],
			State:      stage.State,
			StateWord:  stage.StateWord,
			Evidence:   stage.Evidence,
			TimeText:   pilotOutcomeTime(stage.Time),
			SourceWord: stage.SourceWord,
			PinWord:    stage.PinWord,
		})
	}
	return rows
}

// pilotOutcomeTime renders ONE stage evidence time (the zero time stays
// empty — the honest absence, never the 01.01.0001 artifact).
func pilotOutcomeTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return formatTime(value)
}

// ---------------------------------------------------------------------------
// The standalone page «Отчёт пилота» (the pilot.html 4-1 precedent)
// ---------------------------------------------------------------------------

// pilotOutcomePage renders the «Отчёт пилота» page: the principal rides
// the session resolver (the pilot.go 4-1 precedent), the tenant rides the
// path, the period rides the query (the absence is the whole observation
// period). The errors map onto the honest problem ladder (401 by the
// resolver, 404 for the bad tenant path, the 403 scope word, the honest
// 500); the ports-missing word of the unwired contour renders the honest
// zero page (a single panel with the word, NO tables).
func (s *Server) pilotOutcomePage(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	tenant, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.pilotOutcomeProblem(w, r, actionDomain.ErrNotFound)
		return
	}
	service := s.pilotOutcomeService()
	if service == nil {
		s.render(w, "pilot-outcome", map[string]any{
			"TenantID": tenant,
			"View":     pilotOutcomeDegradedPageView(),
		})
		return
	}
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	report, err := service.ReadPilotReport(r.Context(), actor, tenant, period, s.clock.Now().UTC())
	if err != nil {
		if isPilotOutcomePortsMissing(err) {
			s.render(w, "pilot-outcome", map[string]any{
				"TenantID": tenant,
				"View":     pilotOutcomeDegradedPageView(),
			})
			return
		}
		s.pilotOutcomeProblem(w, r, err)
		return
	}
	s.render(w, "pilot-outcome", map[string]any{
		"TenantID": tenant,
		"View":     preparePilotOutcomePageView(report),
	})
}

// pilotOutcomeProblem maps the page errors onto the honest problem
// ladder (the pilot.go 4-1 handler precedent: the bad tenant path is the
// honest 404; the scope refusal keeps its 403 word — the honest scope
// text rides the app error; every other failure is the honest 500).
func (s *Server) pilotOutcomeProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, actionDomain.ErrForbidden):
		s.problem(w, r, http.StatusForbidden, "forbidden", err.Error(), false, nil)
	case errors.Is(err, actionDomain.ErrNotFound):
		s.problem(w, r, http.StatusNotFound, "not_found", "Пространство не найдено.", false, nil)
	default:
		s.problem(w, r, http.StatusInternalServerError, "internal", "Не удалось открыть отчёт пилота. Повторите позднее.", false, nil)
	}
}

// pilotOutcomeDegradedPageView builds the honest zero page of the
// unwired contour (a single panel with the word, NO tables — the preview
// precedent).
func pilotOutcomeDegradedPageView() pilotOutcomePageView {
	return pilotOutcomePageView{
		Degraded:        true,
		DegradationWord: pilotOutcomePortsMissingWord,
		ScopeText:       pilotOutcomeScopeWord,
	}
}

// pilotOutcomePageView is the flattened projection of the report page:
// the render-ready strings of every number (the sample size and the
// period word ride EVERY number — AC-11), the funnel rows, the
// conversion rows, the counter texts, the unknown share, the
// non-measurable rows, the dictionary rows and the degradation word.
type pilotOutcomePageView struct {
	// Degraded reports the unwired contour: the honest zero note — one
	// panel with the word, NO tables.
	Degraded        bool
	DegradationWord string
	// The sample size and the period word ride EVERY number (AC-11).
	SampleSizeText     string
	PeriodWord         string
	EmptySampleWord    string
	ScopeText          string
	BoundaryWord       string
	Funnel             []pilotOutcomeFunnelRow
	Conversions        []pilotOutcomeConversionRow
	Counters           []string
	UnknownNumerator   string
	UnknownDenominator string
	UnknownShareWord   string
	NotMeasurable      []pilotOutcomeNotMeasurableRow
	Dictionary         []pilotOutcomeDictionaryRow
	ReadCostWord       string
}

// pilotOutcomeFunnelRow is ONE stage row of the report funnel: the honest
// Russian word of the stage, the dictionary state word and the count
// text — a NUMBER for the measurable stages (the proven counts), the
// honest word «не измеряется» for the terminal stages (NEVER a number).
type pilotOutcomeFunnelRow struct {
	Stage     string
	StageWord string
	StateWord string
	CountText string
}

// pilotOutcomeConversionRow is ONE conversion between the PROVEN
// neighbours: the stages BY NAME with the numerator and the denominator
// (render-ready) and the honest conversion word.
type pilotOutcomeConversionRow struct {
	From        string
	To          string
	Numerator   string
	Denominator string
	Word        string
}

// pilotOutcomeNotMeasurableRow is ONE non-measurable indicator of the
// report: the title and the reason WORD (the row carries NO numeric
// value — the missing component is never replaced by a zero).
type pilotOutcomeNotMeasurableRow struct {
	Title  string
	Reason string
}

// pilotOutcomeDictionaryRow is ONE indicator definition of the
// Measurement Dictionary v1 (AC-9): the id BY NAME, the title, the
// formula word, the grain, the numerator, the denominator, the version
// and the owner word.
type pilotOutcomeDictionaryRow struct {
	ID          string
	Title       string
	Formula     string
	Grain       string
	Numerator   string
	Denominator string
	Version     string
	Owner       string
}

// preparePilotOutcomePageView converts the derived report into the page
// projection: every number becomes the render-ready string that carries
// the sample size and the period word (AC-11 — the captions repeat them
// beside every table, the counters carry them inline); the terminal
// stages speak the honest «не измеряется» word; the non-measurable
// indicators stay the reason WORDS of the dictionary; the OQ-7 boundary
// word and the Measurement Dictionary v1 ride the page.
func preparePilotOutcomePageView(report actionDomain.PilotReport) pilotOutcomePageView {
	view := pilotOutcomePageView{
		SampleSizeText:  strconv.Itoa(report.SampleSize),
		PeriodWord:      report.PeriodWord,
		EmptySampleWord: report.EmptySampleWord,
		ScopeText:       pilotOutcomeScopeWord,
		BoundaryWord:    report.BoundaryWord,
		ReadCostWord:    pilotOutcomeReadCostWord,
		Funnel:          make([]pilotOutcomeFunnelRow, 0, len(actionDomain.PilotOutcomeStages)),
		Conversions:     make([]pilotOutcomeConversionRow, 0, len(report.Conversions)),
		NotMeasurable:   make([]pilotOutcomeNotMeasurableRow, 0, len(report.NotMeasurable)),
		Dictionary:      make([]pilotOutcomeDictionaryRow, 0, len(report.Dictionary)),
	}
	for _, stage := range actionDomain.PilotOutcomeStages {
		row := pilotOutcomeFunnelRow{
			Stage:     stage,
			StageWord: pilotOutcomeStageWords[stage],
			StateWord: actionDomain.PilotStageStateWords[actionDomain.PilotStageStateUnknownPending],
			CountText: pilotOutcomeNotMeasuredWord,
		}
		if actionDomain.WordIn(actionDomain.PilotMeasurableStages, stage) {
			row.StateWord = actionDomain.PilotStageStateWords[actionDomain.PilotStageStateProven]
			row.CountText = strconv.Itoa(report.StageCounts[stage])
		}
		view.Funnel = append(view.Funnel, row)
	}
	for _, conversion := range report.Conversions {
		view.Conversions = append(view.Conversions, pilotOutcomeConversionRow{
			From:        conversion.From,
			To:          conversion.To,
			Numerator:   strconv.Itoa(conversion.Numerator),
			Denominator: strconv.Itoa(conversion.Denominator),
			Word:        conversion.Word,
		})
	}
	view.Counters = []string{
		"Коррекции (контур 4-8): " + strconv.Itoa(report.CorrectionsCount) + " — выборка: " + view.SampleSizeText + "; " + report.PeriodWord,
		"Передачи человеку (контур 2-13): " + strconv.Itoa(report.HandoffCount) + " — выборка: " + view.SampleSizeText + "; " + report.PeriodWord,
		"Recovery-кейсы (контур 3-8): " + strconv.Itoa(report.RecoveryCount) + " — выборка: " + view.SampleSizeText + "; " + report.PeriodWord,
	}
	view.UnknownNumerator = strconv.Itoa(report.UnknownShare.Numerator)
	view.UnknownDenominator = strconv.Itoa(report.UnknownShare.Denominator)
	view.UnknownShareWord = report.UnknownShare.Word
	for _, indicator := range report.NotMeasurable {
		view.NotMeasurable = append(view.NotMeasurable, pilotOutcomeNotMeasurableRow{
			Title:  indicator.Title,
			Reason: indicator.Reason,
		})
	}
	for _, indicator := range report.Dictionary {
		view.Dictionary = append(view.Dictionary, pilotOutcomeDictionaryRow{
			ID:          indicator.ID,
			Title:       indicator.Title,
			Formula:     indicator.Formula,
			Grain:       indicator.Grain,
			Numerator:   indicator.Numerator,
			Denominator: indicator.Denominator,
			Version:     indicator.Version,
			Owner:       indicator.Owner,
		})
	}
	return view
}
