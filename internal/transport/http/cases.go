package httptransport

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// BindCases mounts the case contour surface: the machine ingress contract
// (no user principal — the scope is derived from the verified connection),
// the operator Cases/Timeline surfaces, the sealed payload read under
// review/audit, and the Story 2.2 work command surface (the machine JSON
// contract and the operator forms of the Work Control Card).
func (s *Server) BindCases(service casesAPI.Service) {
	s.cases = service
	s.mux.HandleFunc("POST /v1/ingress/channel-events", s.ingressChannelEventAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases", s.listCasesAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}", s.readCaseAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}/events/{event_id}/payload", s.readCasePayloadAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/cases", s.casesPage)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/cases/{case_id}", s.caseDetailPage)
	// The work commands (Story 2.2) — the machine JSON contract:
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items", s.createWorkItemAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/classify", s.classifyWorkItemAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/transition", s.transitionWorkItemAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/split", s.splitWorkItemAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/merge", s.mergeWorkItemAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/events", s.linkEventAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/dispute", s.disputeWorkItemAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/facts", s.recordFactAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/confirmations", s.confirmFactAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/rescope", s.rescopeFactAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}/facts", s.factsViewAPI)

	// The Story 2.3 surfaces: the verification commands (machine JSON +
	// operator forms, PRG).
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/verifications", s.startVerificationAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/bindings", s.bindChannelAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/bindings/{binding_id}/unlink", s.unlinkChannelAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/duplicates/{candidate_id}/resolution", s.resolveDuplicateAPI)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/verifications", s.startVerificationForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/bindings", s.bindChannelForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/bindings/{binding_id}/unlink", s.unlinkChannelForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/duplicates/{candidate_id}/resolution", s.resolveDuplicateForm)
	// The operator forms of the Work Control Card (the PRG pattern):
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items", s.createWorkItemForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/classify", s.classifyWorkItemForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/transition", s.transitionWorkItemForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/split", s.splitWorkItemForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/merge", s.mergeWorkItemForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/events", s.linkEventForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/dispute", s.disputeWorkItemForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/facts", s.recordFactForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/confirmations", s.confirmFactForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/facts/{fact_id}/rescope", s.rescopeFactForm)
}

// ingressEnvelopeInput is the versioned canonical machine contract
// (schema v1): strict decoding, dictionary-bounded values, the raw
// provider payload as one JSON value. The provider DTO never becomes the
// domain model — this contract is the only translator.
type ingressEnvelopeInput struct {
	SchemaVersion     *int             `json:"schema_version"`
	TenantID          *string          `json:"tenant_id"`
	ConnectionID      *string          `json:"connection_id"`
	Provider          *string          `json:"provider"`
	ProviderEventID   *string          `json:"provider_event_id"`
	ProviderSessionID *string          `json:"provider_session_id"`
	Direction         *string          `json:"direction"`
	Modality          *string          `json:"modality"`
	Channel           *string          `json:"channel"`
	ProviderTimestamp *string          `json:"provider_timestamp"`
	Payload           *json.RawMessage `json:"payload"`
}

// ingressBodyLimit is the hard transport DoS cap: four times the fixed
// envelope dictionary limit (1 MiB over a 256 KiB bound). The admission
// ladder bounds the honest sizes with counters (envelope_exceeded); the
// transport only refuses what it cannot safely read.
const ingressBodyLimit = 4 * 256 * 1024 // 4 × MaxEnvelopeBytes of the fixed domain dictionary

func (s *Server) ingressChannelEventAPI(w http.ResponseWriter, r *http.Request) {
	if s.cases == nil {
		s.problem(w, r, http.StatusServiceUnavailable, "cases_unavailable", "Контур обращений не сконфигурирован.", false, nil)
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		s.problem(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Используйте Content-Type application/json.", false, nil)
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key == "" || len(key) > 128 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Заголовок Idempotency-Key обязателен (до 128 символов).", false, nil)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, ingressBodyLimit))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Тело запроса не читается (превышен предел контура).", false, nil)
		return
	}
	decoder := json.NewDecoder(bytesReader(body))
	decoder.DisallowUnknownFields()
	var input ingressEnvelopeInput
	if err := decoder.Decode(&input); err != nil {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Проверьте поля envelope: строгий разбор (неизвестные поля запрещены).", false, nil)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Тело запроса должно содержать один JSON-объект.", false, nil)
		return
	}
	command, ok := s.ingressCommand(w, r, input, key, len(body))
	if !ok {
		return
	}
	outcome, err := s.cases.IngestEvent(r.Context(), command)
	if err != nil {
		correlationID, _ := correlationFromRequest(r)
		s.logger.ErrorContext(r.Context(), "ingress failed", "code", "ingress_failed", "correlation_id", correlationID.String(), "error", err.Error())
		casesProblem(w, r, s, err)
		return
	}
	if outcome.Accepted != nil {
		accepted := outcome.Accepted
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/v1/tenants/"+command.Envelope.TenantClaim.String()+"/cases/"+accepted.CaseID.String())
		if accepted.Replay {
			w.Header().Set("Idempotency-Replayed", "true")
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusCreated)
		}
		_ = json.NewEncoder(w).Encode(accepted)
		return
	}
	if outcome.Quarantined != nil {
		quarantined := outcome.Quarantined
		s.problem(w, r, http.StatusConflict, quarantined.Kind,
			"Доставка карантинится и видна оператору с correlation ID; принятый материал не переписывается.",
			false, map[string]string{"exception_id": quarantined.ExceptionID.String(), "event_id": quarantined.EventID.String()})
		return
	}
	if outcome.Refused != nil {
		refused := outcome.Refused
		status := refusalStatus(refused.Kind)
		fields := map[string]string(nil)
		if refused.Counter != 0 {
			fields = map[string]string{"counter": jsonInt(refused.Counter)}
		}
		s.problem(w, r, status, refused.Kind, refused.Message, refused.Bounded, fields)
		return
	}
	s.problem(w, r, http.StatusInternalServerError, "internal_error", "Ingress вернул пустой исход — это ошибка контура.", true, nil)
}

func refusalStatus(kind string) int {
	switch kind {
	case casesDomain.KindMalformed, casesDomain.KindChannelMismatch:
		return http.StatusBadRequest
	case casesDomain.KindUnknownConnection:
		return http.StatusNotFound
	case casesDomain.KindEntitlementBlocked, casesDomain.KindEnvelopeExceeded:
		return http.StatusTooManyRequests
	default:
		return http.StatusForbidden
	}
}

// ingressCommand assembles the app command from the decoded input: the
// canonical payload bytes and the measured envelope size.
func (s *Server) ingressCommand(w http.ResponseWriter, r *http.Request, input ingressEnvelopeInput, key string, envelopeBytes int) (casesAPI.IngestCommand, bool) {
	tenancyClaim, err := id.Parse(stringOrEmpty(input.TenantID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Поле tenant_id должно быть UUID.", false, nil)
		return casesAPI.IngestCommand{}, false
	}
	connectionID, err := id.Parse(stringOrEmpty(input.ConnectionID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Поле connection_id должно быть UUID.", false, nil)
		return casesAPI.IngestCommand{}, false
	}
	providerTimestamp, err := time.Parse(time.RFC3339, stringOrEmpty(input.ProviderTimestamp))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Поле provider_timestamp должно быть RFC3339.", false, nil)
		return casesAPI.IngestCommand{}, false
	}
	if input.Payload == nil || len(*input.Payload) == 0 {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Поле payload обязательно.", false, nil)
		return casesAPI.IngestCommand{}, false
	}
	payload, err := casesDomain.CanonicalPayload(*input.Payload)
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, casesDomain.KindMalformed, "Поле payload должно быть одним JSON-значением.", false, nil)
		return casesAPI.IngestCommand{}, false
	}
	correlationID, _ := correlationFromRequest(r)
	return casesAPI.IngestCommand{
		Envelope: casesDomain.Envelope{
			SchemaVersion:     intOrZero(input.SchemaVersion),
			TenantClaim:       tenancyClaim,
			ConnectionID:      connectionID,
			Provider:          stringOrEmpty(input.Provider),
			ProviderEventID:   stringOrEmpty(input.ProviderEventID),
			ProviderSessionID: stringOrEmpty(input.ProviderSessionID),
			Direction:         stringOrEmpty(input.Direction),
			Modality:          stringOrEmpty(input.Modality),
			Channel:           stringOrEmpty(input.Channel),
			ProviderTimestamp: providerTimestamp,
			Payload:           payload,
			PayloadDigest:     casesDomain.PayloadDigest(payload),
			EnvelopeBytes:     envelopeBytes,
		},
		IdempotencyKey: key,
		CorrelationID:  correlationID,
		Now:            s.clock.Now().UTC(),
	}, true
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func intOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func jsonInt(value int) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// casesProblem maps the service errors of the case surface onto the
// problem contract: a foreign tenant is a plain 404 (the tenant existence
// is never disclosed), a member without the scope a 403. Internal
// failures are logged with the correlation id — the honest text never
// leaks internals, the log carries them for the operator.
func casesProblem(w http.ResponseWriter, r *http.Request, s *Server, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, casesDomain.ErrNotFound):
		s.problem(w, r, http.StatusNotFound, "not_found", "Пространство или обращение не найдены.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		s.problem(w, r, http.StatusForbidden, "cases_forbidden", "Недостаточно полномочий для контура обращений.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		s.logger.ErrorContext(r.Context(), "cases surface failed", "code", "cases_surface_failed", "correlation_id", correlationID.String(), "error", err.Error())
		s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить запрос. Повторите позднее.", true, nil)
	}
}

func (s *Server) listCasesAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	view, err := s.cases.CasesView(r.Context(), actor, tenant)
	if err != nil {
		casesProblem(w, r, s, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) readCaseAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	view, err := s.cases.CaseView(r.Context(), actor, tenant, caseID)
	if err != nil {
		casesProblem(w, r, s, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) readCasePayloadAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	eventID, err := id.Parse(r.PathValue("event_id"))
	if err != nil {
		s.problem(w, r, http.StatusNotFound, "not_found", "Событие не найдено.", false, nil)
		return
	}
	view, err := s.cases.ReadPayload(r.Context(), actor, tenant, caseID, eventID)
	if err != nil {
		casesProblem(w, r, s, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		EventID       id.UUID         `json:"event_id"`
		PayloadDigest string          `json:"payload_digest"`
		SealOK        bool            `json:"seal_ok"`
		Payload       json.RawMessage `json:"payload"`
	}{EventID: view.EventID, PayloadDigest: view.PayloadDigest, SealOK: view.SealOK, Payload: json.RawMessage(view.Payload)})
}

func (s *Server) casesPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	view, err := s.cases.CasesView(r.Context(), actor, tenant)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	s.render(w, "cases", map[string]any{
		"TenantID": tenant,
		"View":     prepareCasesView(view),
	})
}

func (s *Server) caseDetailPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	view, err := s.cases.CaseView(r.Context(), actor, tenant, caseID)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	// The facts of the Work Control Card follow the review/audit boundary:
	// a configuration-scoped member sees the metadata surface without the
	// values (the honest note instead of a failed page).
	facts, err := s.cases.FactsView(r.Context(), actor, tenant, caseID)
	if err != nil && !errors.Is(err, domain.ErrForbidden) {
		s.casesPageProblem(w, r, err)
		return
	}
	flash := flashError(r, tenant)
	if flash != "" {
		http.SetCookie(w, clearFlashCookie(tenant))
	}
	s.render(w, "case", map[string]any{
		"TenantID": tenant,
		"View":     prepareCaseView(view, facts, flash),
	})
}

func (s *Server) casesPageProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, casesDomain.ErrNotFound):
		s.notFound(w, r, domain.Principal{})
	case errors.Is(err, domain.ErrForbidden):
		s.renderStatus(w, http.StatusForbidden, "cases", map[string]any{
			"View": casesPageView{ScopeText: casesScopeText(false), ForbiddenText: "Просмотр обращений требует полномочия review, configuration или audit."},
		})
	default:
		s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось открыть обращения. Повторите позднее.", true, nil)
	}
}

func (s *Server) casesActor(w http.ResponseWriter, r *http.Request) (domain.Principal, id.UUID, bool) {
	principal, ok := s.principal(w, r)
	if !ok {
		return domain.Principal{}, id.UUID(""), false
	}
	tenantID, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.notFound(w, r, principal)
		return domain.Principal{}, id.UUID(""), false
	}
	return principal, tenantID, true
}

func (s *Server) casesActorAndCase(w http.ResponseWriter, r *http.Request) (domain.Principal, id.UUID, id.UUID, bool) {
	actor, tenantID, ok := s.casesActor(w, r)
	if !ok {
		return domain.Principal{}, id.UUID(""), id.UUID(""), false
	}
	caseID, err := id.Parse(r.PathValue("case_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return domain.Principal{}, id.UUID(""), id.UUID(""), false
	}
	return actor, tenantID, caseID, true
}

// writeJSON renders one JSON answer of the case surface.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
