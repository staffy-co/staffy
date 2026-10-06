package httptransport

import (
        "encoding/json"
        "errors"
        "io"
        "mime"
        "net/http"
        "strconv"
        "time"

        "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/correlation"
        "github.com/artemiosu/staffy/internal/platform/id"
        privacyAPI "github.com/artemiosu/staffy/internal/privacy/api"
        privacyDomain "github.com/artemiosu/staffy/internal/privacy/domain"
)

func (s *Server) privacyService() privacyAPI.Service { return s.privacy }

func (s *Server) privacyScope(w http.ResponseWriter, r *http.Request) (domain.Principal, id.UUID, bool) {
        actor, ok := s.principal(w, r)
        if !ok {
                return actor, "", false
        }
        tenant, err := id.Parse(r.PathValue("tenant_id"))
        if err != nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return actor, "", false
        }
        return actor, tenant, true
}

func (s *Server) privacyProblem(w http.ResponseWriter, r *http.Request, err error) {
        status, code, message := 500, "internal_error", "Не удалось выполнить действие. Повторите позднее."
        var fields map[string]string
        var rule privacyDomain.RuleError
        if errors.As(err, &rule) {
                message = rule.Error()
                fields = map[string]string{rule.Rule: rule.Reason}
        }
        switch {
        case errors.Is(err, privacyDomain.ErrNotFound), errors.Is(err, privacyDomain.ErrForbidden):
                status, code, message = 404, "not_found", "Ресурс не найден."
        case errors.As(err, &rule):
                status, code = 400, "privacy_rule"
        case errors.Is(err, privacyDomain.ErrInvalidIdempotency):
                status, code, message = 400, "invalid_idempotency_key", "Обновите форму и повторите попытку."
        case errors.Is(err, privacyDomain.ErrIdempotencyConflict):
                status, code, message = 409, "idempotency_conflict", "Этот ключ уже использован для другого запроса."
        }
        s.problem(w, r, status, code, message, status >= 500, fields)
}

// BindPrivacy mounts the data perimeter surface: the JSON API and the
// keyboard-accessible data map pages.
func (s *Server) BindPrivacy(service privacyAPI.Service) {
        s.privacy = service
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/data-map", s.dataMapAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/data-map/entries", s.publishMapEntryAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/retention-policies", s.publishPolicyAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/legal-holds", s.activateHoldAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/legal-holds/release", s.releaseHoldAPI)
        s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/deletion-requests", s.runDeletionAPI)
        s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/deletion-requests/{request_id}/receipt", s.receiptAPI)
        s.mux.HandleFunc("GET /app/tenants/{tenant_id}/datamap", s.datamapPage)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/datamap/entry", s.mapEntryForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/datamap/policy", s.policyForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/datamap/hold", s.holdForm)
        s.mux.HandleFunc("POST /app/tenants/{tenant_id}/datamap/delete", s.deletionForm)
        s.mux.HandleFunc("GET /app/tenants/{tenant_id}/datamap/receipts/{request_id}", s.receiptPage)
}

func (s *Server) dataMapAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.privacyScope(w, r)
        if !ok {
                return
        }
        service := s.privacyService()
        if service == nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        view, err := service.DataView(r.Context(), actor, tenant)
        if err != nil {
                s.privacyProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{"classes": view.Classes, "receipts": view.Receipts, "hold_events": view.HoldEvents})
}

func (s *Server) publishMapEntryAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.privacyScope(w, r)
        if !ok {
                return
        }
        service := s.privacyService()
        if service == nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        var input struct {
                Class           string   `json:"class"`
                Purpose         string   `json:"purpose"`
                LegalBasis      string   `json:"legal_basis"`
                Controller      string   `json:"controller"`
                Processor       string   `json:"processor"`
                Recipients      []string `json:"recipients"`
                SubProcessors   []string `json:"sub_processors"`
                Locations       []string `json:"locations"`
                AccessibleRoles []string `json:"accessible_roles"`
                KnownCopies     []string `json:"known_copies"`
                PolicyRef       string   `json:"retention_policy_ref"`
                DeletionMethod  string   `json:"deletion_method"`
                ExportMethod    string   `json:"export_method"`
        }
        media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
        if err != nil || media != "application/json" {
                s.privacyProblem(w, r, privacyDomain.ErrInvalidIdempotency)
                return
        }
        decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
        decoder.DisallowUnknownFields()
        if err = decoder.Decode(&input); err != nil {
                s.privacyProblem(w, r, privacyDomain.ErrInvalidIdempotency)
                return
        }
        if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
                s.privacyProblem(w, r, privacyDomain.ErrInvalidIdempotency)
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        entry, err := service.PublishMapEntry(r.Context(), privacyAPI.MapEntryCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Class: privacyDomain.ArtifactClass(input.Class), Purpose: input.Purpose, LegalBasis: input.LegalBasis,
                Controller: input.Controller, Processor: input.Processor, Recipients: input.Recipients,
                SubProcessors: input.SubProcessors, Locations: input.Locations, AccessibleRoles: input.AccessibleRoles,
                KnownCopies: input.KnownCopies, PolicyRef: input.PolicyRef,
                DeletionMethod: input.DeletionMethod, ExportMethod: input.ExportMethod,
        })
        if err != nil {
                s.privacyProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(entry)
}

func (s *Server) publishPolicyAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.privacyScope(w, r)
        if !ok {
                return
        }
        service := s.privacyService()
        if service == nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        var input struct {
                Class              string `json:"class"`
                Purpose            string `json:"purpose"`
                LegalBasis         string `json:"legal_basis"`
                Source             string `json:"source"`
                TerminalEvent      string `json:"terminal_event"`
                ActiveUseDays      int    `json:"active_use_days"`
                ArchiveDays        int    `json:"archive_days"`
                DeletionDays       int    `json:"deletion_days"`
                BackupExpiryDays   int    `json:"backup_expiry_days"`
                LegalHoldException string `json:"legal_hold_exception"`
                OwnerNote          string `json:"owner_note"`
        }
        media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
        if err != nil || media != "application/json" {
                s.privacyProblem(w, r, privacyDomain.ErrInvalidIdempotency)
                return
        }
        decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
        decoder.DisallowUnknownFields()
        if err = decoder.Decode(&input); err != nil {
                s.privacyProblem(w, r, privacyDomain.ErrInvalidIdempotency)
                return
        }
        if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
                s.privacyProblem(w, r, privacyDomain.ErrInvalidIdempotency)
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        policy, err := service.PublishPolicy(r.Context(), privacyAPI.PolicyCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Class: privacyDomain.ArtifactClass(input.Class), Purpose: input.Purpose, LegalBasis: input.LegalBasis,
                Source: input.Source, TerminalEvent: input.TerminalEvent,
                ActiveUseDays: input.ActiveUseDays, ArchiveDays: input.ArchiveDays, DeletionDays: input.DeletionDays,
                BackupExpiryDays: input.BackupExpiryDays, LegalHoldException: input.LegalHoldException, OwnerNote: input.OwnerNote,
        })
        if err != nil {
                s.privacyProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(policy)
}

func (s *Server) activateHoldAPI(w http.ResponseWriter, r *http.Request) {
        s.holdAPI(w, r, "activated")
}

func (s *Server) releaseHoldAPI(w http.ResponseWriter, r *http.Request) {
        s.holdAPI(w, r, "released")
}

func (s *Server) holdAPI(w http.ResponseWriter, r *http.Request, kind string) {
        actor, tenant, ok := s.privacyScope(w, r)
        if !ok {
                return
        }
        service := s.privacyService()
        if service == nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        var input struct {
                Classes        []string `json:"classes"`
                Basis          string   `json:"basis"`
                Approver       string   `json:"approver"`
                ReviewDeadline string   `json:"review_deadline"`
        }
        media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
        if err != nil || media != "application/json" {
                s.privacyProblem(w, r, privacyDomain.ErrInvalidIdempotency)
                return
        }
        decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
        decoder.DisallowUnknownFields()
        if err = decoder.Decode(&input); err != nil {
                s.privacyProblem(w, r, privacyDomain.ErrInvalidIdempotency)
                return
        }
        classes := make([]privacyDomain.ArtifactClass, 0, len(input.Classes))
        for _, class := range input.Classes {
                classes = append(classes, privacyDomain.ArtifactClass(class))
        }
        // RFC3339 is the expected honest form for the review deadline.
        var reviewDeadline time.Time
        if input.ReviewDeadline != "" {
                parsed, err := http.ParseTime(input.ReviewDeadline)
                if err != nil {
                        s.privacyProblem(w, r, privacyDomain.RuleError{Rule: "review_deadline", Reason: "Дата пересмотра не распознана (ожидается RFC3339)."})
                        return
                }
                reviewDeadline = parsed
        }
        corr, _ := correlation.FromContext(r.Context())
        command := privacyAPI.HoldCommand{Actor: actor, TenantID: tenant, CorrelationID: corr, Classes: classes, Basis: input.Basis, Approver: input.Approver, ReviewDeadline: reviewDeadline}
        var holdErr error
        if kind == "released" {
                holdErr = service.ReleaseHold(r.Context(), command)
        } else {
                holdErr = service.ActivateHold(r.Context(), command)
        }
        if holdErr != nil {
                s.privacyProblem(w, r, holdErr)
                return
        }
        w.WriteHeader(http.StatusNoContent)
}

func (s *Server) runDeletionAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.privacyScope(w, r)
        if !ok {
                return
        }
        service := s.privacyService()
        if service == nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        var input struct {
                Classes        []string `json:"classes"`
                IdempotencyKey string   `json:"idempotency_key"`
        }
        media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
        if err != nil || media != "application/json" {
                s.privacyProblem(w, r, privacyDomain.ErrInvalidIdempotency)
                return
        }
        decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
        decoder.DisallowUnknownFields()
        if err = decoder.Decode(&input); err != nil {
                s.privacyProblem(w, r, privacyDomain.ErrInvalidIdempotency)
                return
        }
        if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
                s.privacyProblem(w, r, privacyDomain.ErrInvalidIdempotency)
                return
        }
        classes := make([]privacyDomain.ArtifactClass, 0, len(input.Classes))
        for _, class := range input.Classes {
                classes = append(classes, privacyDomain.ArtifactClass(class))
        }
        corr, _ := correlation.FromContext(r.Context())
        result, err := service.RunDeletion(r.Context(), privacyAPI.DeletionCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Classes: classes, IdempotencyKey: input.IdempotencyKey,
        })
        if err != nil {
                s.privacyProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        if result.Replayed {
                w.Header().Set("Idempotency-Replayed", "true")
        }
        w.WriteHeader(http.StatusCreated)
        _ = json.NewEncoder(w).Encode(result.Receipt)
}

func (s *Server) receiptAPI(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.privacyScope(w, r)
        if !ok {
                return
        }
        requestID, err := id.Parse(r.PathValue("request_id"))
        if err != nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        service := s.privacyService()
        if service == nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        result, err := service.ReadReceipt(r.Context(), actor, tenant, requestID)
        if err != nil {
                s.privacyProblem(w, r, err)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(result.Receipt)
}

// datamapPage renders the data map: classes, honest statuses, due positions,
// hold events and receipts, plus the management forms.
func (s *Server) datamapPage(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.privacyScope(w, r)
        if !ok {
                return
        }
        service := s.privacyService()
        if service == nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        view, err := service.DataView(r.Context(), actor, tenant)
        if err != nil {
                s.privacyProblem(w, r, err)
                return
        }
        data := map[string]any{
                "TenantID":    tenant,
                "Classes":     prepareDataMapViews(view),
                "Receipts":    prepareReceiptViews(view.Receipts),
                "HoldEvents":  prepareHoldEventViews(view.HoldEvents),
                "ClassOptions": classOptions(),
                "FormKey":     s.freshKey("datamap"),
                "DeletionKey": s.freshKey("deletion"),
        }
        if v := r.URL.Query().Get("saved"); v == "1" {
                data["Notice"] = "Запись карты данных опубликована."
        }
        if v := r.URL.Query().Get("policy_saved"); v == "1" {
                data["Notice"] = "Retention-политика опубликована."
        }
        if v := r.URL.Query().Get("hold_saved"); v == "1" {
                data["Notice"] = "Событие юридического удержания зафиксировано."
        }
        if message := flashError(r, tenant); message != "" {
                data["Error"] = message
                http.SetCookie(w, clearFlashCookie(tenant))
        }
        s.render(w, "datamap", data)
}

func classOptions() []map[string]any {
        options := make([]map[string]any, 0, len(privacyDomain.ClassSpecs()))
        for _, spec := range privacyDomain.ClassSpecs() {
                options = append(options, map[string]any{"ID": string(spec.ID), "Title": spec.Title})
        }
        return options
}

func (s *Server) mapEntryForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.privacyScope(w, r)
        if !ok {
                return
        }
        service := s.privacyService()
        if service == nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
        if err := r.ParseForm(); err != nil {
                s.privacyProblem(w, r, privacyDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."})
                return
        }
        corr, _ := correlation.FromContext(r.Context())
        _, err := service.PublishMapEntry(r.Context(), privacyAPI.MapEntryCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Class:           privacyDomain.ArtifactClass(r.FormValue("class")),
                Purpose:         r.FormValue("purpose"),
                LegalBasis:      r.FormValue("legal_basis"),
                Controller:      r.FormValue("controller"),
                Processor:       r.FormValue("processor"),
                Recipients:      splitLines(r.FormValue("recipients")),
                SubProcessors:   splitLines(r.FormValue("sub_processors")),
                Locations:       splitLines(r.FormValue("locations")),
                AccessibleRoles: r.Form["accessible_roles"],
                KnownCopies:     splitLines(r.FormValue("known_copies")),
                PolicyRef:       r.FormValue("retention_policy_ref"),
                DeletionMethod:  r.FormValue("deletion_method"),
                ExportMethod:    r.FormValue("export_method"),
        })
        if err != nil {
                redirectPrivacyError(w, r, tenant, err)
                return
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/datamap?saved=1", http.StatusSeeOther)
}

func (s *Server) policyForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.privacyScope(w, r)
        if !ok {
                return
        }
        service := s.privacyService()
        if service == nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err := r.ParseForm(); err != nil {
                s.privacyProblem(w, r, privacyDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."})
                return
        }
        active, _ := strconv.Atoi(r.FormValue("active_use_days"))
        archive, _ := strconv.Atoi(r.FormValue("archive_days"))
        deletion, _ := strconv.Atoi(r.FormValue("deletion_days"))
        backup, _ := strconv.Atoi(r.FormValue("backup_expiry_days"))
        corr, _ := correlation.FromContext(r.Context())
        _, err := service.PublishPolicy(r.Context(), privacyAPI.PolicyCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Class:              privacyDomain.ArtifactClass(r.FormValue("class")),
                Purpose:            r.FormValue("purpose"),
                LegalBasis:         r.FormValue("legal_basis"),
                Source:             r.FormValue("source"),
                TerminalEvent:      r.FormValue("terminal_event"),
                ActiveUseDays:      active,
                ArchiveDays:        archive,
                DeletionDays:       deletion,
                BackupExpiryDays:   backup,
                LegalHoldException: r.FormValue("legal_hold_exception"),
                OwnerNote:          r.FormValue("owner_note"),
        })
        if err != nil {
                redirectPrivacyError(w, r, tenant, err)
                return
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/datamap?policy_saved=1", http.StatusSeeOther)
}

func (s *Server) holdForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.privacyScope(w, r)
        if !ok {
                return
        }
        service := s.privacyService()
        if service == nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err := r.ParseForm(); err != nil {
                s.privacyProblem(w, r, privacyDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."})
                return
        }
        classes := make([]privacyDomain.ArtifactClass, 0, len(r.Form["classes"]))
        for _, class := range r.Form["classes"] {
                classes = append(classes, privacyDomain.ArtifactClass(class))
        }
        command := privacyAPI.HoldCommand{
                Actor: actor, TenantID: tenant, CorrelationID: correlationFrom(r), Classes: classes,
                Basis: r.FormValue("basis"), Approver: r.FormValue("approver"),
        }
        if deadline := r.FormValue("review_deadline"); deadline != "" {
                parsed, err := http.ParseTime(deadline + "T23:59:59Z")
                if err != nil {
                        redirectPrivacyError(w, r, tenant, privacyDomain.RuleError{Rule: "review_deadline", Reason: "Дата пересмотра не распознана."})
                        return
                }
                command.ReviewDeadline = parsed
        }
        var err error
        if r.FormValue("action") == "release" {
                err = service.ReleaseHold(r.Context(), command)
        } else {
                err = service.ActivateHold(r.Context(), command)
        }
        if err != nil {
                redirectPrivacyError(w, r, tenant, err)
                return
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/datamap?hold_saved=1", http.StatusSeeOther)
}

func (s *Server) deletionForm(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.privacyScope(w, r)
        if !ok {
                return
        }
        service := s.privacyService()
        if service == nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
        if err := r.ParseForm(); err != nil {
                s.privacyProblem(w, r, privacyDomain.RuleError{Rule: "form", Reason: "Проверьте данные формы."})
                return
        }
        // Accessible irreversible-action confirmation: an explicit checkbox with
        // a text label, not a color cue.
        if r.FormValue("confirm_deletion") != "yes" {
                redirectPrivacyError(w, r, tenant, privacyDomain.RuleError{Rule: "confirm", Reason: "Подтвердите необратимое удаление данных."})
                return
        }
        classes := make([]privacyDomain.ArtifactClass, 0, len(r.Form["classes"]))
        for _, class := range r.Form["classes"] {
                classes = append(classes, privacyDomain.ArtifactClass(class))
        }
        corr, _ := correlation.FromContext(r.Context())
        result, err := service.RunDeletion(r.Context(), privacyAPI.DeletionCommand{
                Actor: actor, TenantID: tenant, CorrelationID: corr,
                Classes: classes, IdempotencyKey: r.FormValue("idempotency_key"),
        })
        if err != nil {
                redirectPrivacyError(w, r, tenant, err)
                return
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/datamap/receipts/"+result.Receipt.RequestID, http.StatusSeeOther)
}

func (s *Server) receiptPage(w http.ResponseWriter, r *http.Request) {
        actor, tenant, ok := s.privacyScope(w, r)
        if !ok {
                return
        }
        requestID, err := id.Parse(r.PathValue("request_id"))
        if err != nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        service := s.privacyService()
        if service == nil {
                s.privacyProblem(w, r, privacyDomain.ErrNotFound)
                return
        }
        result, err := service.ReadReceipt(r.Context(), actor, tenant, requestID)
        if err != nil {
                s.privacyProblem(w, r, err)
                return
        }
        s.render(w, "receipt", map[string]any{
                "TenantID": tenant,
                "Receipt":  prepareReceiptDetail(result.Receipt),
        })
}

func redirectPrivacyError(w http.ResponseWriter, r *http.Request, tenant id.UUID, err error) {
        var rule privacyDomain.RuleError
        if errors.As(err, &rule) {
                setFlashError(w, tenant, rule.Error())
        } else {
                setFlashError(w, tenant, "Действие не выполнено. Повторите позднее.")
        }
        http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/datamap", http.StatusSeeOther)
}

func correlationFrom(r *http.Request) id.UUID {
        corr, _ := correlation.FromContext(r.Context())
        return corr
}
