package httptransport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.15 transport: the machine JSON contract of the operator
// correction record and the PRG form «Исправить» of the fact row. The
// dictionary refusals (the version pin, the value discipline, the span
// bounds with the byte-exact source check, the mandatory reason) are the
// honest 400 rule words; the raced correction number and the idempotency
// key conflicts are the honest 409; a foreign tenant is a plain 404; an
// audit-only member is the 403 with the scope words BEFORE every material
// guard (the M1 precedent of 2.5..2.14). The OUTCOME is NEVER a request
// field — the strict decode refuses it (the domain matrix gate decides
// the word by the material key of the anchor); the correction number is
// the server truth under the locks. The PRG form lands on the
// #decision-context anchor (the section position survives, the exact
// scroll position does not — the no-JS boundary of the slice); the
// markers of the fact rows render only with the correction material
// (the control branch renders byte-identically).
// ---------------------------------------------------------------------------

// recordCorrectionInput is the request of the operator correction record:
// the pinned target version (1..versions of the fact of THIS case — the
// store guards the belonging), the summary link (0 = the event evidence;
// >=1 = the EXACT summary version of THIS case), the proposed value (any
// ONE JSON value — the domain value guard canonicalizes under the fact
// material discipline), the verbatim evidence spans (0..8, each 1..200
// runes, each byte-exact in the cited source) and the mandatory reason.
// The OUTCOME is NEVER a request field: the strict decode refuses it (the
// matrix gate of the domain decides — the request never carries the
// server truth). The correction number is NEVER a request field either.
type recordCorrectionInput struct {
	VersionNo *int64           `json:"version_no"`
	SummaryNo *int64           `json:"summary_no"`
	Value     *json.RawMessage `json:"value"`
	Spans     []string         `json:"spans"`
	Reason    *string          `json:"reason"`
}

// correctionFormError flashes the honest message and returns to the case
// page at the #decision-context anchor (the PRG target of the correction
// form — the section position survives, the scroll position does not).
func correctionFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	var rule casesDomain.WorkRuleError
	message := "Действие не выполнено. Повторите позднее."
	switch {
	case errors.As(err, &rule):
		message = rule.Reason
	case errors.Is(err, casesDomain.ErrStaleVersion):
		message = "Материал изменился: страница показывает актуальное состояние — сверьте версию и повторите."
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		message = "Ключ команды уже использован другим материалом: команда не выполнена."
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		message = "Обращение или факт не найдены."
	case errors.Is(err, domain.ErrForbidden):
		message = "Запись коррекции требует полномочия review или configuration."
	}
	setFlashError(w, tenant, message)
	http.Redirect(w, r, workFormTarget(tenant, caseID)+"#decision-context", http.StatusSeeOther)
}

// recordCorrectionAPI answers the machine POST of ONE append-only
// correction row: the fresh record is the honest 201, the exact repeat
// the 200 with the Idempotency-Replayed header, the dictionary refusals
// the 400 rule words, the raced conflicts the 409. The OUTCOME is never a
// request field (the strict decode refusal) — the word of the matrix gate
// travels in the answer only.
func (s *Server) recordCorrectionAPI(w http.ResponseWriter, r *http.Request) {
	if s.corrections == nil {
		s.problem(w, r, http.StatusServiceUnavailable, "cases_unavailable", "Контур обращений не сконфигурирован.", false, nil)
		return
	}
	actor, tenant, caseID, factID, ok := s.casesFactActor(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input recordCorrectionInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	if input.VersionNo == nil || *input.VersionNo < 1 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле version_no обязательно: точная версия факта (1..версий факта этого обращения).", false, nil)
		return
	}
	if input.Value == nil || len(*input.Value) == 0 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле value обязательно: предложенное значение (одно JSON-значение материальной дисциплины фактов).", false, nil)
		return
	}
	if input.Reason == nil || strings.TrimSpace(*input.Reason) == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле reason обязательно: причина коррекции (1..200 символов — акт осознанный).", false, nil)
		return
	}
	summaryNo := int64(0)
	if input.SummaryNo != nil && *input.SummaryNo < 0 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле summary_no — неотрицательный номер сводки (0 = выдержки из события; >=1 = точная версия сводки этого обращения).", false, nil)
		return
	} else if input.SummaryNo != nil {
		summaryNo = *input.SummaryNo
	}
	spans := input.Spans
	if spans == nil {
		spans = []string{}
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.corrections.RecordCorrection(r.Context(), casesAPI.RecordCorrectionCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, FactID: factID,
		VersionNo: *input.VersionNo, SummaryNo: summaryNo,
		Value: []byte(*input.Value), Spans: spans, Reason: *input.Reason,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		decisionContextProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// recordCorrectionForm answers the PRG form «Исправить» of the fact row:
// the version select (the ladder of the fact), the proposed value, one
// verbatim evidence span, the optional summary link and the mandatory
// reason. The double submit of the SAME material is the honest replay
// (the composed key covers the whole form material); the outcome lands
// on the #decision-context anchor (the marker appears at the section —
// the position survives, the scroll does not).
func (s *Server) recordCorrectionForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, factID, ok := s.casesFactActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		correctionFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	versionNo, err := formInt64(r, "version_no")
	if err != nil || versionNo < 1 {
		correctionFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Версия факта обязательна: выберите точную версию лестницы."})
		return
	}
	value := strings.TrimSpace(r.FormValue("value"))
	if value == "" {
		correctionFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Предложенное значение обязательно: одно JSON-значение материальной дисциплины фактов."})
		return
	}
	reason := r.FormValue("reason")
	summaryNo := int64(0)
	if raw := strings.TrimSpace(r.FormValue("summary_no")); raw != "" {
		summaryNo, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || summaryNo < 0 {
			correctionFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Номер сводки — неотрицательное число (0 = выдержки из события)."})
			return
		}
	}
	spans := []string{}
	if span := strings.TrimSpace(r.FormValue("span")); span != "" {
		spans = append(spans, span)
	}
	_, err = s.corrections.RecordCorrection(r.Context(), casesAPI.RecordCorrectionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, FactID: factID,
		VersionNo: versionNo, SummaryNo: summaryNo,
		Value: []byte(value), Spans: spans, Reason: reason,
		IdempotencyKey: formCommandKey("correction", actor.ID.String(), caseID.String(), factID.String(),
			strconv.FormatInt(versionNo, 10), value, strings.Join(spans, "\n"), reason, strconv.FormatInt(summaryNo, 10)),
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		correctionFormError(w, r, tenant, caseID, err)
		return
	}
	http.Redirect(w, r, workFormTarget(tenant, caseID)+"?correction_recorded=1#decision-context", http.StatusSeeOther)
}

// correctionSpanCountText renders the honest span counter of the panel
// line (the spans themselves render behind the progressive disclosure).
func correctionSpanCountText(spans []string) string {
	if len(spans) == 0 {
		return "без выдержек"
	}
	word := "выдержек"
	if len(spans)%10 == 1 && len(spans)%100 != 11 {
		word = "выдержка"
	} else if len(spans)%10 >= 2 && len(spans)%10 <= 4 && (len(spans)%100 < 10 || len(spans)%100 >= 20) {
		word = "выдержки"
	}
	return strconv.Itoa(len(spans)) + " " + word
}
