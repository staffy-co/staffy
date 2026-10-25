package httptransport

import (
        "context"
        "errors"
        "fmt"
        "net/http"
        "strings"
        "time"

        actionAPI "github.com/artemiosu/staffy/internal/action/api"
        actionDomain "github.com/artemiosu/staffy/internal/action/domain"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
)

// Story 4.9 (step-03): the «Статус заказа» panel of the case page —
// the Order Status Projection surface over the material of the three
// machines (the operation+receipt of 4-6, the verification events of
// 4-7, the correction events of 4-8) plus the Supplier Order Fact
// Receipt of this slice. The panel carries the five stages with the
// evidence (the time, the source BY NAME, the external ID, the owner)
// — the stages are WORDS, never colors only; the per-system blocks
// stay SEPARATE (ABCP / 1С / поставщик — AC-2) with the divergent
// external IDs BOTH shown (AC-8); the freshness speaks the live
// derivation and the no-schedule word (AC-10); the three deadline
// words stay SEPARATE (AC-11); the special state (the open correction)
// is never hidden behind «в работе» (AC-12); the notification and the
// dispute words ride the boundary blocks (OQ-1/OQ-2, AC-14); the
// client retell words close the panel (AC-16). The record form speaks
// the HUMAN receipt discipline: the kind select {сформирован,
// отправлен}, the supplier order ID and the named fact time — NO
// supplier-send action exists (the OQ-3 fence: the record IS the
// claim of what already happened, never a command to perform it).

// orderStatusPanelView is the flattened panel of the case page.
type orderStatusPanelView struct {
        // Wired reports the LIVE contour: the status surface is bound and
        // readable — the section renders its honest boundary words even for
        // a case WITHOUT an order operation (the contour words are the
        // interface truth).
        Wired      bool
        HasOrder   bool
        Stages     []orderStageRow
        Systems    []orderSystemRow
        Timeline   []orderTimelineRow
        Current    string
        CurrentTxt string
        Special    string
        SpecialOwn string
        SpecialDet string
        Next       string

        // The boundary blocks (the honest words of the slice).
        FreshnessLast  string
        FreshnessWord  string
        NoScheduleWord string
        DeadlineEst    string
        DeadlineConf   string
        DeadlineAct    string
        Notification   string
        Dispute        string
        Cancellation   string
        ClientWords    []string

        // The record form material (the anchor operation of the commands).
        FactFormTarget string
        OperationID    string
        FormNoteWord   string
        SourceWord     string
}

// orderStageRow is ONE lifecycle stage row of the panel: the word, the
// honest text, the confirmation state (TEXT, never a color only), the
// evidence pins (the time, the source BY NAME with its text, the
// external ID, the owner of the open round) and the per-stage detail.
type orderStageRow struct {
        Stage      string
        Word       string
        Confirmed  bool
        StateWord  string
        TimeText   string
        Source     string
        SourceText string
        ExternalID string
        OwnerID    string
        Detail     string
}

// orderSystemRow is ONE per-system block (AC-2): the system word, the
// honest word and the facts with the sources BY NAME (the divergent
// IDs flag speaks the BOTH-shown refusal — AC-8).
type orderSystemRow struct {
        System       string
        Title        string
        Word         string
        Facts        []orderFactRow
        DivergentIDs bool
}

// orderFactRow is ONE per-system fact of the panel (the render material
// of the per-system blocks and the timeline).
type orderFactRow struct {
        Kind         string
        KindText     string
        ExternalID   string
        HappenedText string
        RecordedText string
        Source       string
        SourceText   string
        ActorID      string
        Word         string
}

// orderTimelineRow is ONE transition of the ordered feed (AC-9): the
// time, the machine word, the kind word, the honest word, the source
// BY NAME and the external ID.
type orderTimelineRow struct {
        AtText     string
        Machine    string
        Kind       string
        Word       string
        Source     string
        SourceText string
        ExternalID string
}

// orderStatusSystemTitles is the honest system title dictionary of the
// panel (the render order of the projection dictionary).
var orderStatusSystemTitles = map[string]string{
        actionDomain.OrderStatusSystemABCP:     "ABCP (магазин)",
        actionDomain.OrderStatusSystemOneC:     "1С (бухгалтерия)",
        actionDomain.OrderStatusSystemSupplier: "Поставщик",
}

// orderStatusPanelOf reads the panel material with the honest
// degradation of the case page precedent: an unwired status surface
// renders the zero panel — the page keeps the rest; a scope-forbidden
// member sees the panel without the status sections; every other
// failure fails the page honestly. The projection NEVER fails on a
// case without an order — the honest «операции заказа в кейсе нет»
// rides the stages (AC-16: the absence is spoken).
func (s *Server) orderStatusPanelOf(ctx context.Context, actor identityDomain.Principal, tenant, caseID id.UUID) (orderStatusPanelView, error) {
        panel := orderStatusPanelView{}
        if s.orderStatus == nil {
                return panel, nil
        }
        projection, err := s.orderStatus.ReadOrderStatusView(ctx, actor, tenant, caseID, s.clock.Now().UTC())
        if err != nil {
                if errors.Is(err, actionDomain.ErrForbidden) || isActionNotConfigured(err) {
                        return panel, nil
                }
                return orderStatusPanelView{}, err
        }
        panel.Wired = true
        // The honest miss of a case WITHOUT an order operation: the panel
        // still renders (the contour words are the interface truth) — the
        // stages speak «операции заказа в кейсе нет» (AC-16: the absence is
        // spoken, never masked).
        panel.HasOrder = !strings.Contains(projection.Stages[0].Word, "операции заказа в кейсе нет")
        panel.OperationID = projection.OperationID.String()
        panel.Stages = orderStatusStageRows(projection.Stages)
        panel.Systems = orderStatusSystemRows(projection.Systems)
        panel.Timeline = orderStatusTimelineRows(projection.Timeline)
        panel.Current = projection.CurrentStage
        if projection.CurrentStage != "" {
                panel.CurrentTxt = actionDomain.OrderStageWords[projection.CurrentStage]
        }
        panel.Special = projection.SpecialState
        panel.SpecialOwn = projection.SpecialOwner.String()
        panel.SpecialDet = projection.SpecialDetail
        panel.Next = projection.NextStep
        panel.FreshnessLast = formatTime(projection.Freshness.LastConfirmedAt)
        panel.FreshnessWord = projection.Freshness.Word
        panel.NoScheduleWord = projection.Freshness.NoScheduleWord
        panel.DeadlineEst = projection.Deadline.EstimateWord
        panel.DeadlineConf = projection.Deadline.ConfirmedWord
        panel.DeadlineAct = projection.Deadline.ActualWord
        panel.Notification = projection.NotificationWord
        panel.Dispute = projection.DisputeWord
        panel.Cancellation = projection.CancellationWord
        panel.ClientWords = projection.ClientWords
        panel.FactFormTarget = "/app/tenants/" + tenant.String() + "/cases/" + caseID.String() + "/order-supplier-facts"
        panel.FormNoteWord = actionDomain.OrderStatusSupplierManagerSourceWord
        panel.SourceWord = actionDomain.OrderStatusSupplierManagerSourceWord
        return panel, nil
}

// orderStatusStageRows flattens the stage evidence into the panel rows
// (the confirmation state is TEXT — «подтверждено» / «не
// подтверждено», never a color only).
func orderStatusStageRows(stages []actionDomain.OrderStageEvidence) []orderStageRow {
        rows := make([]orderStageRow, 0, len(stages))
        for _, stage := range stages {
                row := orderStageRow{
                        Stage:      stage.Stage,
                        Word:       stage.Word,
                        Confirmed:  stage.Confirmed,
                        StateWord:  "не подтверждено",
                        ExternalID: stage.ExternalID,
                        OwnerID:    stage.OwnerID.String(),
                        Detail:     stage.Detail,
                }
                if stage.Confirmed {
                        row.StateWord = "подтверждено"
                        row.TimeText = formatTime(stage.Time)
                }
                if stage.Source != "" {
                        row.Source = stage.Source
                        row.SourceText = actionDomain.OrderFactSourceWords[stage.Source]
                }
                rows = append(rows, row)
        }
        return rows
}

// orderStatusSystemRows flattens the per-system blocks (AC-2: the
// systems stay SEPARATE — one word per system, the facts with the
// sources BY NAME).
func orderStatusSystemRows(systems []actionDomain.OrderStatusSystemView) []orderSystemRow {
        rows := make([]orderSystemRow, 0, len(systems))
        for _, system := range systems {
                row := orderSystemRow{
                        System:       system.System,
                        Title:        orderStatusSystemTitles[system.System],
                        Word:         system.Word,
                        DivergentIDs: system.DivergentIDs,
                }
                for _, fact := range system.Facts {
                        row.Facts = append(row.Facts, orderFactRow{
                                Kind:         fact.Kind,
                                KindText:     actionDomain.OrderSupplierFactKindWords[fact.Kind],
                                ExternalID:   fact.ExternalID,
                                HappenedText: formatTime(fact.HappenedAt),
                                RecordedText: formatTime(fact.RecordedAt),
                                Source:       fact.Source,
                                SourceText:   actionDomain.OrderFactSourceWords[fact.Source],
                                ActorID:      fact.ActorID.String(),
                                Word:         fact.Word,
                        })
                }
                rows = append(rows, row)
        }
        return rows
}

// orderStatusTimelineRows flattens the ordered transition feed (AC-9).
func orderStatusTimelineRows(timeline []actionDomain.OrderStatusTimelineEntry) []orderTimelineRow {
        rows := make([]orderTimelineRow, 0, len(timeline))
        for _, entry := range timeline {
                rows = append(rows, orderTimelineRow{
                        AtText:     formatTime(entry.At),
                        Machine:    entry.Machine,
                        Kind:       entry.Kind,
                        Word:       entry.Word,
                        Source:     entry.Source,
                        SourceText: actionDomain.OrderFactSourceWords[entry.Source],
                        ExternalID: entry.ExternalID,
                })
        }
        return rows
}

// BindOrderStatus registers the Story 4.9 status surface: the machine
// JSON routes of the two record commands and the page PRG form (the
// narrow OrderStatusService type-assert over the action service — the
// OrderCorrectionService precedent).
func (s *Server) BindOrderStatus(service actionAPI.OrderStatusService) {
        s.orderStatus, _ = service.(actionAPI.OrderStatusService)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/order-supplier-facts/formed", s.recordOrderSupplierFormedAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/order-supplier-facts/sent", s.recordOrderSupplierSentAPI)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/order-supplier-facts", s.recordOrderSupplierFactForm)
}

// orderStatusProblem maps the status errors onto the honest transport
// ladder: the guard words (the kind dictionary, the typed bounds, the
// named time, the sent-after-formed ladder) are the 400 family (the
// honest dictionary refusal), the anchor word is the 404, the scope
// words keep their ladder.
func (s *Server) orderStatusProblem(w http.ResponseWriter, r *http.Request, err error) {
        switch {
        case errors.Is(err, actionDomain.ErrInvalid),
                errors.Is(err, actionDomain.ErrOrderSupplierFactLadder):
                s.problem(w, r, http.StatusBadRequest, "order_supplier_fact_invalid", err.Error(), false, nil)
        case errors.Is(err, actionDomain.ErrOrderStatusNoOperation):
                s.problem(w, r, http.StatusNotFound, "order_operation_not_found", err.Error(), false, nil)
        case errors.Is(err, actionDomain.ErrForbidden):
                s.problem(w, r, http.StatusForbidden, "forbidden", err.Error(), false, nil)
        case errors.Is(err, actionDomain.ErrNotFound):
                s.problem(w, r, http.StatusNotFound, "not_found", "Кейс или операция заказа не найдены.", false, nil)
        default:
                s.problem(w, r, http.StatusInternalServerError, "internal", "Внутренняя ошибка статуса заказа.", false, nil)
        }
}

// orderSupplierFactOutcomeJSON renders the honest JSON of the record
// outcome: the fact pins (the kind word, the supplier order ID, the
// TWO times — never mixed), the replay flag (the idempotent repeat BY
// KEY, AC-9) and the source word (the human receipt — AG-4).
func orderSupplierFactOutcomeJSON(outcome actionAPI.OrderSupplierFactOutcome) map[string]any {
        payload := map[string]any{
                "kind":              outcome.Fact.Kind,
                "supplier_order_id": outcome.Fact.SupplierOrderID,
                "happened_at":       outcome.Fact.HappenedAt.UTC().Format(time.RFC3339),
                "recorded_at":       outcome.Fact.RecordedAt.UTC().Format(time.RFC3339),
                "fact_no":           outcome.Fact.FactNo,
                "replay":            outcome.Replay,
                "source":            actionDomain.OrderFactSourceManagerRecord,
                "source_word":       outcome.SourceWord,
        }
        return payload
}

// recordOrderSupplierFactInput is the strict decode target of the two
// machine routes (the closed body: the supplier order ID and the named
// happened_at ONLY — no send field exists to type, the OQ-3 fence).
type recordOrderSupplierFactInput struct {
        OperationID     string `json:"operation_id"`
        SupplierOrderID string `json:"supplier_order_id"`
        HappenedAt      string `json:"happened_at"`
}

// recordOrderSupplierFormedAPI answers the machine formed-record
// request: the strict decode of the closed body, the honest ladder of
// the refusals and the replay-aware 201/200.
func (s *Server) recordOrderSupplierFormedAPI(w http.ResponseWriter, r *http.Request) {
        s.recordOrderSupplierFactAPI(w, r, actionDomain.OrderSupplierFactFormed)
}

// recordOrderSupplierSentAPI answers the machine sent-record request
// (the sent-after-formed ladder rides the service guard).
func (s *Server) recordOrderSupplierSentAPI(w http.ResponseWriter, r *http.Request) {
        s.recordOrderSupplierFactAPI(w, r, actionDomain.OrderSupplierFactSent)
}

// recordOrderSupplierFactAPI is the shared implementation of the two
// machine record routes: the kind word rides the ROUTE (never the
// body — the closed form carries the fact material only).
func (s *Server) recordOrderSupplierFactAPI(w http.ResponseWriter, r *http.Request, kind string) {
        actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
        if !ok {
                return
        }
        if s.orderStatus == nil {
                s.notFound(w, r, actor)
                return
        }
        var input recordOrderSupplierFactInput
        if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
                s.orderStatusProblem(w, r, err)
                return
        }
        operationID, err := id.Parse(strings.TrimSpace(input.OperationID))
        if err != nil {
                s.orderStatusProblem(w, r, fmt.Errorf("%w: поле operation_id обязательно (UUID живой операции заказа)", actionDomain.ErrInvalid))
                return
        }
        happenedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(input.HappenedAt))
        if err != nil {
                s.orderStatusProblem(w, r, fmt.Errorf("%w: поле happened_at обязательно (RFC3339 — названное менеджером время события)", actionDomain.ErrInvalid))
                return
        }
        command := actionAPI.RecordOrderSupplierFactCommand{
                Actor: actor, TenantID: tenant, CaseID: caseID,
                OperationID: operationID, SupplierOrderID: strings.TrimSpace(input.SupplierOrderID),
                HappenedAt: happenedAt, Now: s.clock.Now().UTC(),
        }
        outcome, err := s.recordOrderSupplierFactByKind(r, kind, command)
        if err != nil {
                s.orderStatusProblem(w, r, err)
                return
        }
        status := http.StatusCreated
        if outcome.Replay {
                status = http.StatusOK
        }
        writeJSON(w, status, map[string]any{"fact": orderSupplierFactOutcomeJSON(outcome)})
}

// recordOrderSupplierFactByKind dispatches the kind word to the named
// service command.
func (s *Server) recordOrderSupplierFactByKind(r *http.Request, kind string, command actionAPI.RecordOrderSupplierFactCommand) (actionAPI.OrderSupplierFactOutcome, error) {
        if kind == actionDomain.OrderSupplierFactSent {
                return s.orderStatus.RecordOrderSupplierSent(r.Context(), command)
        }
        return s.orderStatus.RecordOrderSupplierFormed(r.Context(), command)
}

// recordOrderSupplierFactForm answers the page record form: the origin
// gate, the strict parse of the closed form (the kind select, the
// supplier order ID, the named fact time and the anchor operation) and
// the honest PRG landing (the flash word of every refusal — the order
// form precedent).
func (s *Server) recordOrderSupplierFactForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
        if !ok {
                return
        }
        if s.orderStatus == nil {
                s.notFound(w, r, actor)
                return
        }
        if !validFormOrigin(r) {
                s.orderStatusProblem(w, r, fmt.Errorf("%w: форма записи факта поставщика доступна только из браузера приложения (origin).", actionDomain.ErrForbidden))
                return
        }
        if !formParse(w, r) {
                setFlashError(w, tenant, "Проверьте данные формы записи факта поставщика.")
                http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
                return
        }
        kind := strings.TrimSpace(r.PostFormValue("kind"))
        if kind != actionDomain.OrderSupplierFactFormed && kind != actionDomain.OrderSupplierFactSent {
                setFlashError(w, tenant, "Вид факта — слово словаря {сформирован, отправлен}.")
                http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
                return
        }
        operationID, err := id.Parse(strings.TrimSpace(r.PostFormValue("operation_id")))
        if err != nil {
                setFlashError(w, tenant, "Якорь операции заказа обязателен (UUID живой операции create_abcp_customer_order).")
                http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
                return
        }
        happenedAt, err := time.Parse("2006-01-02T15:04", strings.TrimSpace(r.PostFormValue("happened_at")))
        if err != nil {
                setFlashError(w, tenant, "Время факта обязательно (менеджер называет фактическое время события — два времени никогда не смешиваются).")
                http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
                return
        }
        command := actionAPI.RecordOrderSupplierFactCommand{
                Actor: actor, TenantID: tenant, CaseID: caseID,
                OperationID: operationID, SupplierOrderID: strings.TrimSpace(r.PostFormValue("supplier_order_id")),
                HappenedAt: happenedAt.UTC(), Now: s.clock.Now().UTC(),
        }
        outcome, err := s.recordOrderSupplierFactByKind(r, kind, command)
        if err != nil {
                setFlashError(w, tenant, "Факт не записан: "+err.Error())
                http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases/"+caseID.String(), http.StatusSeeOther)
                return
        }
        flag := "supplier_fact_formed"
        if outcome.Fact.Kind == actionDomain.OrderSupplierFactSent {
                flag = "supplier_fact_sent"
        }
        if outcome.Replay {
                flag = "supplier_fact_replay"
        }
        workFormDone(w, r, tenant, caseID, flag)
}
