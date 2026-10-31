package httptransport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 3.3 transport: the machine JSON contract of the external
// commitment dispatch (the first slice) and the operator Commitment
// Review page. The gate ladder lives in the app service and runs FIRST:
// a non-member of the tenant is the honest 404 (before any material), a
// member without the review-or-configuration scope is the 403 with the
// scope words BEFORE any material; the approval adds the step-up gate
// (ErrStepUpRequired — the ChangeMember identity precedent) and the
// CURRENT-human-owner gate (the app words); the enqueue adds the
// approval gate of the EXACT version and the readiness gate — every
// dictionary refusal answers the honest 400 rule words BEFORE the
// transaction. The decode is strict: one JSON object, known fields only,
// Content-Type application/json. The seven delivery states are spoken
// with the FIXED domain render texts — submitted never reads as a
// delivery, delivered never reads as an acceptance (AC-7/AC-11); the
// states and the readiness words are read-time derivations, never stored
// words. The page forms are the PRG family with the focus transfer onto
// the notice (the release/controls precedent); the explicit confirmation
// checkboxes are the UI discipline — the server-side protection is the
// step-up + the owner gate + the approval gate (the double discipline,
// documented honestly; the machine API carries no checkbox field).
// ---------------------------------------------------------------------------

// externalCommitmentsProblem maps the external commitment errors onto the
// problem contract: the module's ErrNotFound is the plain 404 (no
// disclosure that the surface exists); the identity forbidden sentinel is
// the 403 whose message IS the scope words the app attached at the wrap
// point (one source — the transport never invents its own copy, the
// sentinel is sliced off); the identity step-up sentinel is the 403 with
// the step-up words; the dictionary WorkRuleError is the 400 family with
// the rule name as the code and the honest reason (the cases-family
// precedent — internal details never leak as text); anything else is the
// honest 500 with the server-side log.
func (s *Server) externalCommitmentsProblem(w http.ResponseWriter, r *http.Request, err error) {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		s.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		s.problem(w, r, http.StatusNotFound, "not_found", "Обращение или версия конверта не найдены.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		message := err.Error()
		if prefix := domain.ErrForbidden.Error() + ": "; strings.HasPrefix(message, prefix) {
			message = strings.TrimPrefix(message, prefix)
		}
		s.problem(w, r, http.StatusForbidden, "cases_forbidden", message, false, nil)
	case errors.Is(err, domain.ErrStepUpRequired):
		s.problem(w, r, http.StatusForbidden, "step_up_required", "Для утверждения версии конверта требуется повторное подтверждение личности.", false, nil)
	default:
		s.logger.ErrorContext(r.Context(), "external commitment command failed", "code", "external_commitment_failed", "error", err.Error())
		s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить команду конверта. Повторите позднее.", true, nil)
	}
}

// externalCommitmentsFormMessage renders the honest flash text of the
// form errors (the commitmentFormMessage precedent: the rule words, the
// honest 404/403 texts, the step-up confirmation).
func externalCommitmentsFormMessage(err error) string {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		return rule.Reason
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		return "Обращение или версия конверта не найдены: перечитайте страницу и повторите."
	case errors.Is(err, domain.ErrForbidden):
		return "Команды внешних обязательств требуют полномочия review или configuration."
	case errors.Is(err, domain.ErrStepUpRequired):
		return "Для утверждения версии конверта требуется повторное подтверждение личности. Включите подтверждение личности и повторите."
	}
	return "Команда конверта не выполнена. Повторите позднее."
}

// externalReviewTarget answers the PRG target of the external commitment
// forms — the Commitment Review page of the case with the one success
// flag (the notice speaks the words, the focus moves onto it).
func externalReviewTarget(tenant, caseID id.UUID, flag string, value string) string {
	return "/app/tenants/" + tenant.String() + "/cases/" + caseID.String() + "/external-commitments?" + flag + "=" + value
}

// ---------------------------------------------------------------------------
// The machine JSON contract
// ---------------------------------------------------------------------------

// externalPinInput is the strict machine shape of the optional 2.9
// exact-version pin triple (all-or-nothing — the partial triple travels
// to the domain guard, the ONE refusal source).
type externalPinInput struct {
	CommitmentID string `json:"commitment_id"`
	VersionNo    int64  `json:"version_no"`
	ContentToken string `json:"content_token"`
}

// createExternalCommitmentInput is the strict machine shape of the
// envelope version creation. The Work Item anchor travels in the path
// (the 2.9 commitment family); the shape refusals (the uuid fields, the
// RFC3339 window) are the transport's honest 400; the dictionary words
// travel to the service — the domain guard is the ONE source of the
// refusal text.
type createExternalCommitmentInput struct {
	ExtCommitmentID  string            `json:"ext_commitment_id"`
	Purpose          string            `json:"purpose"`
	RecipientKind    string            `json:"recipient_kind"`
	Channel          string            `json:"channel"`
	ContentText      string            `json:"content_text"`
	ValidUntil       string            `json:"valid_until"`
	ExpectedNextStep string            `json:"expected_next_step"`
	AckRequirement   string            `json:"ack_requirement"`
	Pin              *externalPinInput `json:"pin"`
}

// relayExternalReceiptInput is the strict machine shape of the operator
// relay: the observation word of the channel world (the 2.12
// ConfirmerOperatorRelay precedent — the human passes the machine
// observation, the machine never invents it).
type relayExternalReceiptInput struct {
	Observation string `json:"observation"`
}

// decodeExternalCommitmentsJSON strictly decodes one JSON object: the
// exact Content-Type, the known fields only, nothing after the object
// (the decodeControlsJSON precedent spoken through the cases
// WorkRuleError so the 400 words have one shape).
func decodeExternalCommitmentsJSON(w http.ResponseWriter, r *http.Request, out any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return casesDomain.WorkRuleError{Rule: "content_type", Reason: "Используйте Content-Type application/json."}
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return casesDomain.WorkRuleError{Rule: "body", Reason: "Проверьте JSON-запрос: один объект с известными полями."}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return casesDomain.WorkRuleError{Rule: "body", Reason: "Ожидается ровно один JSON-объект без данных после него."}
	}
	return nil
}

// externalCommandVersionPin parses the pin triple of the create input
// into the domain triple (the all-or-nothing discipline stays with the
// domain guard; nil input answers the honest zero triple).
func externalCommandVersionPin(input *externalPinInput) casesDomain.ExternalCommitmentPin {
	if input == nil {
		return casesDomain.ExternalCommitmentPin{}
	}
	return casesDomain.ExternalCommitmentPin{
		CommitmentID: id.UUID(input.CommitmentID),
		VersionNo:    input.VersionNo,
		ContentToken: input.ContentToken,
	}
}

// externalVersionPinOfPath parses the envelope path pair: the stable
// identifier and the EXACT version number (the anchors of the approval,
// the enqueue and the relay commands).
func externalVersionPinOfPath(r *http.Request) (id.UUID, int64, error) {
	extID, err := id.Parse(r.PathValue("ext_commitment_id"))
	if err != nil {
		return id.UUID(""), 0, err
	}
	versionNo, err := strconv.ParseInt(r.PathValue("version_no"), 10, 64)
	if err != nil || versionNo < 1 {
		return id.UUID(""), 0, fmt.Errorf("версия — целое число начиная с 1")
	}
	return extID, versionNo, nil
}

// readExternalCommitmentsAPI answers the machine read of the Commitment
// Review surface: the per-Work-Item sections with the immutable version
// cards, the queue evidence, the append-only histories and the derived
// words (the honest empty items list of a case without envelopes — never
// a null).
func (s *Server) readExternalCommitmentsAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.externalCommitments == nil {
		s.notFound(w, r, actor)
		return
	}
	view, err := s.externalCommitments.ReadExternalCommitmentView(r.Context(), actor, tenant, caseID)
	if err != nil {
		s.externalCommitmentsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"external_commitments": view})
}

// createExternalCommitmentAPI answers the machine create: the Work Item
// anchor arrives in the path (the 2.9 commitment family), the version
// ladder, the server-side snapshots and the live pin gates run in the
// service; the honest refusal words answer BEFORE any row exists.
func (s *Server) createExternalCommitmentAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.externalCommitments == nil {
		s.notFound(w, r, actor)
		return
	}
	workItemID, err := id.Parse(r.PathValue("work_item_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	var input createExternalCommitmentInput
	if err := decodeExternalCommitmentsJSON(w, r, &input); err != nil {
		s.externalCommitmentsProblem(w, r, err)
		return
	}
	var extCommitmentID id.UUID
	if strings.TrimSpace(input.ExtCommitmentID) != "" {
		if extCommitmentID, err = id.Parse(input.ExtCommitmentID); err != nil {
			s.externalCommitmentsProblem(w, r, casesDomain.WorkRuleError{Rule: "ext_commitment_id", Reason: "Стабильный идентификатор конверта — действительный UUID (или пусто — сервер назначит новый)."})
			return
		}
	}
	validUntil, err := time.Parse(time.RFC3339, strings.TrimSpace(input.ValidUntil))
	if err != nil {
		s.externalCommitmentsProblem(w, r, casesDomain.WorkRuleError{Rule: "valid_until", Reason: "Срок действия конверта — время RFC3339, например 2026-10-26T12:00:00Z."})
		return
	}
	outcome, err := s.externalCommitments.CreateExternalCommitment(r.Context(), casesAPI.CreateExternalCommitmentCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		WorkItemID: workItemID, ExtCommitmentID: extCommitmentID,
		Purpose: input.Purpose, RecipientKind: input.RecipientKind, Channel: input.Channel,
		ContentText: input.ContentText, ValidUntil: validUntil,
		ExpectedNextStep: input.ExpectedNextStep, AckRequirement: input.AckRequirement,
		Pin: externalCommandVersionPin(input.Pin), Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.externalCommitmentsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"outcome": outcome})
}

// approveExternalCommitmentAPI answers the machine approval of the EXACT
// version by the CURRENT human owner (the step-up gate and the owner
// gate live in the service — the path pin is the only request material).
func (s *Server) approveExternalCommitmentAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.externalCommitments == nil {
		s.notFound(w, r, actor)
		return
	}
	extID, versionNo, err := externalVersionPinOfPath(r)
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	workItemID, err := id.Parse(r.PathValue("work_item_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	outcome, err := s.externalCommitments.ApproveExternalCommitment(r.Context(), casesAPI.ApproveExternalCommitmentCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		WorkItemID: workItemID, ExtCommitmentID: extID, VersionNo: versionNo, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.externalCommitmentsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"approval": outcome})
}

// enqueueExternalCommitmentAPI answers the machine enqueue: the approval
// gate of the EXACT version and the readiness gate refuse BEFORE the
// persist-before-emit transaction; the transaction's signature carries
// no adapter (AC-3 — the network call is physically impossible before
// the commit).
func (s *Server) enqueueExternalCommitmentAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.externalCommitments == nil {
		s.notFound(w, r, actor)
		return
	}
	extID, versionNo, err := externalVersionPinOfPath(r)
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	workItemID, err := id.Parse(r.PathValue("work_item_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	outcome, err := s.externalCommitments.EnqueueExternalCommitment(r.Context(), casesAPI.EnqueueExternalCommitmentCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		WorkItemID: workItemID, ExtCommitmentID: extID, VersionNo: versionNo, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.externalCommitmentsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enqueue": outcome})
}

// relayExternalReceiptAPI answers the machine operator relay: the
// observation word of the channel world lands in the append-only history
// of the version (the state follows from the history at read time, never
// from this answer).
func (s *Server) relayExternalReceiptAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.externalCommitments == nil {
		s.notFound(w, r, actor)
		return
	}
	extID, versionNo, err := externalVersionPinOfPath(r)
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	var input relayExternalReceiptInput
	if err := decodeExternalCommitmentsJSON(w, r, &input); err != nil {
		s.externalCommitmentsProblem(w, r, err)
		return
	}
	outcome, err := s.externalCommitments.RelayExternalReceipt(r.Context(), casesAPI.RelayExternalReceiptCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		ExtCommitmentID: extID, VersionNo: versionNo,
		Observation: input.Observation, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.externalCommitmentsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"relay": outcome})
}

// drainExternalCommitmentsAPI answers the machine drain: the bounded
// operator command (no daemons exist — AD-13). The AG-4 honest boundary
// answers the 400 rule words when the production root wires no
// dispatcher — the records stay queued, the state stays honest.
func (s *Server) drainExternalCommitmentsAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	if s.externalCommitments == nil {
		s.notFound(w, r, actor)
		return
	}
	result, err := s.externalCommitments.DrainExternalCommitments(r.Context(), actor, tenant)
	if err != nil {
		s.externalCommitmentsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"drain": result})
}

// ---------------------------------------------------------------------------
// The Commitment Review page
// ---------------------------------------------------------------------------

// externalWordOption is one dictionary option of the page forms: the
// word WITH the honest description beside it (the word alone is never
// enough — the 3.2 page discipline).
type externalWordOption struct {
	Word string
	Text string
}

// externalEventRow is one append-only history row of the page.
type externalEventRow struct {
	No            string
	Kind          string
	ReasonText    string
	AttemptText   string
	Correlation   string
	GuaranteeText string
	Details       string
	At            string
}

// externalVersionCardView is the page projection of one envelope version
// card: every state and derivation spoken in TEXT (never by the colour
// alone), the seven-word delivery state with the FIXED domain render
// text, the queue evidence, the readiness word with the refusal text,
// the approval material and the append-only history.
type externalVersionCardView struct {
	WorkItemID    string
	ExtID         string
	No            string
	PurposeText   string
	RecipientText string
	Channel       string
	Content       string
	ValidUntil    string
	NextStep      string
	AckText       string
	PinText       string
	OwnerText     string
	CreatedAt     string
	EnqueuedText  string
	StateWord     string
	StateText     string
	LastLine      string
	ReadinessWord string
	ReadinessText string
	CurrentText   string
	ApprovedText  string
	Events        []externalEventRow
}

// externalSectionView is one Work Item section of the page.
type externalSectionView struct {
	WorkItemID string
	Cards      []externalVersionCardView
}

// externalFormState carries the create form values for the honest
// re-render (the operator corrects one field, not the whole form).
type externalFormState struct {
	WorkItem      string
	ExtID         string
	Purpose       string
	Recipient     string
	Channel       string
	Content       string
	ValidUntil    string
	NextStep      string
	Ack           string
	PinCommitment string
	PinVersion    string
	PinToken      string
}

// externalCommitmentsPageView carries the page material: the tenant and
// the case, the sections, the honest empty state, the dictionaries of
// the forms with the render texts, the flash error and the PRG notice.
type externalCommitmentsPageView struct {
	TenantID         string
	CaseID           string
	StepUp           bool
	HasItems         bool
	Sections         []externalSectionView
	PanelText        string
	EmptyText        string
	Flash            string
	Notice           string
	HasWorkItems     bool
	PurposeOptions   []externalWordOption
	RecipientOptions []externalWordOption
	AckOptions       []externalWordOption
	ChannelOptions   []externalWordOption
	RelayOptions     []externalWordOption
	Form             externalFormState
}

// externalPurposeOptionTexts speaks the page descriptions of the relay
// observations — the PRESENTATION words of the page (the domain keeps
// the dictionary; the 2.12 relay discipline: the human passes the
// machine observation, the machine never invents it).
var externalRelayDescriptions = map[string]string{
	casesDomain.RelayObservationDelivered:    "канал сообщил доставку — записывается квитанцией receipt_delivered",
	casesDomain.RelayObservationAcknowledged: "получатель принял — записывается квитанцией receipt_acknowledged",
	casesDomain.RelayObservationFailed:       "канал сообщил отказ — записывается квитанцией receipt_failed",
}

// externalDictionaryOptions builds the form options from a dictionary
// with the domain render texts (the page never mirrors the words — the
// options read the live view dictionaries).
func externalDictionaryOptions(words []string, texts func(string) string) []externalWordOption {
	options := make([]externalWordOption, 0, len(words))
	for _, word := range words {
		options = append(options, externalWordOption{Word: word, Text: texts(word)})
	}
	return options
}

// externalPlainWordOptions builds the options of the words without
// domain render texts (the channel dictionary: the word IS the text).
func externalPlainWordOptions(words []string) []externalWordOption {
	options := make([]externalWordOption, 0, len(words))
	for _, word := range words {
		options = append(options, externalWordOption{Word: word, Text: word})
	}
	return options
}

// externalNotice speaks the PRG status of a successful form mutation: the
// flag carries the version number (or the drain counter), the words name
// the discipline of the step that landed.
func externalNotice(r *http.Request) string {
	if raw := r.URL.Query().Get("created"); raw != "" {
		return "Версия " + raw + " конверта сохранена (persisted): слова не уходят — требуется утверждение точной версии текущим человеком-владельцем, затем постановка в очередь. Фокус переведён на этот статус."
	}
	if raw := r.URL.Query().Get("approved"); raw != "" {
		return "Версия " + raw + " утверждена текущим человеком-владельцем: очередь этой точной версии открыта. Фокус переведён на этот статус."
	}
	if raw := r.URL.Query().Get("enqueued"); raw != "" {
		return "Версия " + raw + " поставлена в очередь (queued): слова сохранены до отправки — persist-before-emit, сетевого вызова при постановке нет. Фокус переведён на этот статус."
	}
	if raw := r.URL.Query().Get("relayed"); raw != "" {
		return "Наблюдение канала записано в историю версии " + raw + " — состояние чтения обновлено. Фокус переведён на этот статус."
	}
	if raw := r.URL.Query().Get("drain"); raw != "" {
		return "Диспетчеризация выполнена: рассмотрено " + raw + " записей — статусы и отчёты обновлены ниже. Фокус переведён на этот статус."
	}
	return ""
}

// externalLastEventLine renders the human line of the LAST event of the
// derivation (the empty string of the persisted/queued states — no
// events exist, the page never fabricates a timeline).
func externalLastEventLine(delivery casesAPI.ExternalDeliveryWire) string {
	if delivery.LastEventNo == 0 {
		return ""
	}
	line := "Последнее событие №" + strconv.FormatInt(delivery.LastEventNo, 10) + ": " + delivery.LastKind
	if delivery.LastReason != "" {
		line = line + " — " + casesDomain.ExternalStaleReasonText(delivery.LastReason)
	}
	if delivery.LastExternalCorrelationID != "" {
		line = line + "; корреляция: " + delivery.LastExternalCorrelationID
	}
	return line
}

// newExternalVersionCardView maps one api card onto the page projection.
func newExternalVersionCardView(card casesAPI.ExternalCommitmentVersionCard) externalVersionCardView {
	view := externalVersionCardView{
		ExtID:         card.Version.ExtCommitmentID.String(),
		No:            strconv.FormatInt(card.Version.VersionNo, 10),
		PurposeText:   casesDomain.ExternalPurposeTexts[card.Version.Purpose],
		RecipientText: casesDomain.ExternalRecipientKindTexts[card.Version.RecipientKind],
		Channel:       card.Version.Channel,
		Content:       card.Version.ContentText,
		ValidUntil:    card.Version.ValidUntil.UTC().Format(time.RFC3339),
		NextStep:      card.Version.ExpectedNextStep,
		AckText:       casesDomain.ExternalAckRequirementTexts[card.Version.AckRequirement],
		OwnerText:     card.Version.OwnerKind + "/" + card.Version.OwnerRef.String() + " (epoch " + strconv.FormatInt(card.Version.OwnerEpoch, 10) + ")",
		CreatedAt:     card.Version.CreatedAt.UTC().Format(time.RFC3339),
		EnqueuedText:  map[bool]string{true: "в очереди — outbox запись, слова не уходили", false: "не в очереди"}[card.Enqueued],
		StateWord:     card.Delivery.State,
		StateText:     card.Delivery.StateText,
		LastLine:      externalLastEventLine(card.Delivery),
		ReadinessWord: card.Readiness,
		ReadinessText: card.ReadinessText,
		CurrentText:   map[bool]string{true: "актуальная версия", false: "заменена новой версией"}[card.Current],
		ApprovedText:  map[bool]string{true: "утверждена: " + card.ApproverID.String(), false: "не утверждена"}[card.Approved],
	}
	if card.Version.Pin.CommitmentID != "" || card.Version.Pin.VersionNo != 0 || card.Version.Pin.ContentToken != "" {
		view.PinText = "пин точной версии 2.9: " + card.Version.Pin.CommitmentID.String() +
			" версия " + strconv.FormatInt(card.Version.Pin.VersionNo, 10) + ", токен " + card.Version.Pin.ContentToken
	}
	for _, event := range card.Events {
		row := externalEventRow{
			No:          strconv.FormatInt(event.EventNo, 10),
			Kind:        event.Kind,
			ReasonText:  event.ReasonText,
			Correlation: event.ExternalCorrelationID,
			Details:     event.Details,
			At:          event.CreatedAt.UTC().Format(time.RFC3339),
		}
		if event.AttemptNo > 0 {
			row.AttemptText = "попытка " + strconv.FormatInt(event.AttemptNo, 10)
		}
		if event.IdempotentGuarantee {
			row.GuaranteeText = "идемпотентная гарантия записана"
		}
		view.Events = append(view.Events, row)
	}
	return view
}

// externalCommitmentsPage renders the Commitment Review page of the
// case: the sections with the version cards (the state words with the
// fixed texts, the queue evidence, the histories), the create form with
// the dictionaries of the epic, the per-card approval/enqueue/relay
// forms behind the explicit confirmations and the drain command of the
// queue.
func (s *Server) externalCommitmentsPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.externalCommitments == nil {
		s.notFound(w, r, actor)
		return
	}
	view, err := s.externalCommitments.ReadExternalCommitmentView(r.Context(), actor, tenant, caseID)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	flash := flashError(r, tenant)
	if flash != "" {
		http.SetCookie(w, clearFlashCookie(tenant))
	}
	page := externalCommitmentsPageView{
		TenantID: tenant.String(), CaseID: caseID.String(), StepUp: actor.StepUpVerified,
		PanelText: view.PanelText, EmptyText: view.EmptyText, Flash: flash,
		Notice:           externalNotice(r),
		PurposeOptions:   externalDictionaryOptions(view.PurposeWords, func(word string) string { return casesDomain.ExternalPurposeTexts[word] }),
		RecipientOptions: externalDictionaryOptions(view.RecipientWords, func(word string) string { return casesDomain.ExternalRecipientKindTexts[word] }),
		AckOptions:       externalDictionaryOptions(view.AckWords, func(word string) string { return casesDomain.ExternalAckRequirementTexts[word] }),
		ChannelOptions:   externalPlainWordOptions(view.ChannelWords),
		RelayOptions:     externalDictionaryOptions(view.RelayWords, func(word string) string { return externalRelayDescriptions[word] }),
		Sections:         []externalSectionView{},
	}
	for _, section := range view.Items {
		page.HasWorkItems = true
		view := externalSectionView{WorkItemID: section.WorkItemID.String(), Cards: []externalVersionCardView{}}
		for _, card := range section.Versions {
			page.HasItems = true
			cardView := newExternalVersionCardView(card)
			cardView.WorkItemID = section.WorkItemID.String()
			view.Cards = append(view.Cards, cardView)
		}
		page.Sections = append(page.Sections, view)
	}
	s.render(w, "external_commitments", page)
}

// externalCreateFormInput parses the create form (the shared parse of
// the strict machine shape and the page form — the same refusal words).
type externalCreateFormInput struct {
	WorkItemID       string
	ExtCommitmentID  string
	Purpose          string
	RecipientKind    string
	Channel          string
	ContentText      string
	ValidUntil       string
	ExpectedNextStep string
	AckRequirement   string
	Pin              *externalPinInput
}

// parseExternalCreateForm reads the page form values into the shared
// input shape (the form carries the pin triple as three optional text
// fields — any non-empty field makes the triple present, the
// all-or-nothing discipline stays with the domain guard).
func parseExternalCreateForm(r *http.Request) externalCreateFormInput {
	input := externalCreateFormInput{
		WorkItemID:       r.FormValue("work_item_id"),
		ExtCommitmentID:  r.FormValue("ext_commitment_id"),
		Purpose:          r.FormValue("purpose"),
		RecipientKind:    r.FormValue("recipient_kind"),
		Channel:          r.FormValue("channel"),
		ContentText:      r.FormValue("content_text"),
		ValidUntil:       r.FormValue("valid_until"),
		ExpectedNextStep: r.FormValue("expected_next_step"),
		AckRequirement:   r.FormValue("ack_requirement"),
	}
	commitment := strings.TrimSpace(r.FormValue("pin_commitment_id"))
	version := strings.TrimSpace(r.FormValue("pin_version_no"))
	token := strings.TrimSpace(r.FormValue("pin_content_token"))
	if commitment != "" || version != "" || token != "" {
		pinVersion := int64(0)
		if parsed, err := strconv.ParseInt(version, 10, 64); err == nil {
			pinVersion = parsed
		}
		input.Pin = &externalPinInput{CommitmentID: commitment, VersionNo: pinVersion, ContentToken: token}
	}
	return input
}

// createExternalCommitmentForm answers the page create form: the origin
// gate, the strict parse and the service command; a success answers PRG
// with the created flag (the focus moves onto the notice); a refusal
// re-renders the page with the flash words and the form refilled.
func (s *Server) createExternalCommitmentForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.externalCommitments == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.externalCommitmentsProblem(w, r, fmt.Errorf("%w: %s", domain.ErrForbidden, "форма внешних обязательств доступна только из браузера приложения (origin)."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы.")
		http.Redirect(w, r, externalReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	input := parseExternalCreateForm(r)
	form := externalFormState{
		WorkItem: input.WorkItemID, ExtID: input.ExtCommitmentID,
		Purpose: input.Purpose, Recipient: input.RecipientKind, Channel: input.Channel,
		Content: input.ContentText, ValidUntil: input.ValidUntil, NextStep: input.ExpectedNextStep,
		Ack: input.AckRequirement,
	}
	if input.Pin != nil {
		form.PinCommitment = input.Pin.CommitmentID
		form.PinVersion = strconv.FormatInt(input.Pin.VersionNo, 10)
		form.PinToken = input.Pin.ContentToken
	}
	workItemID, err := id.Parse(input.WorkItemID)
	if err != nil {
		setFlashError(w, tenant, "Элемент работы — действительный UUID: выберите элемент из списка.")
		http.Redirect(w, r, externalReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	var extCommitmentID id.UUID
	if strings.TrimSpace(input.ExtCommitmentID) != "" {
		if extCommitmentID, err = id.Parse(input.ExtCommitmentID); err != nil {
			setFlashError(w, tenant, "Стабильный идентификатор конверта — действительный UUID (или пусто — сервер назначит новый).")
			http.Redirect(w, r, externalReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
			return
		}
	}
	validUntil, err := time.Parse(time.RFC3339, strings.TrimSpace(input.ValidUntil))
	if err != nil {
		setFlashError(w, tenant, "Срок действия конверта — время RFC3339, например 2026-10-26T12:00:00Z.")
		http.Redirect(w, r, externalReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	outcome, err := s.externalCommitments.CreateExternalCommitment(r.Context(), casesAPI.CreateExternalCommitmentCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		WorkItemID: workItemID, ExtCommitmentID: extCommitmentID,
		Purpose: input.Purpose, RecipientKind: input.RecipientKind, Channel: input.Channel,
		ContentText: input.ContentText, ValidUntil: validUntil,
		ExpectedNextStep: input.ExpectedNextStep, AckRequirement: input.AckRequirement,
		Pin: externalCommandVersionPin(input.Pin), Now: s.clock.Now().UTC(),
	})
	if err != nil {
		setFlashError(w, tenant, externalCommitmentsFormMessage(err))
		http.Redirect(w, r, externalReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, externalReviewTarget(tenant, caseID, "created", strconv.FormatInt(outcome.VersionNo, 10)), http.StatusSeeOther)
}

// externalVersionFormResult parses the per-card form path pin and runs
// the shared mutation result ladder: the origin gate, the Work Item
// anchor of the path, the UI-discipline confirmation (the server-side
// protection is the step-up/owner/approval gate — the double
// discipline documented honestly; the machine API carries no checkbox
// field), then the command with the PRG answer.
func (s *Server) externalVersionFormResult(w http.ResponseWriter, r *http.Request, refusalText func(r *http.Request) string, successFlag string, run func(actor domain.Principal, tenant, caseID, workItemID id.UUID, extID id.UUID, versionNo int64, r *http.Request) (string, error)) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.externalCommitments == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.externalCommitmentsProblem(w, r, fmt.Errorf("%w: %s", domain.ErrForbidden, "форма внешних обязательств доступна только из браузера приложения (origin)."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы.")
		http.Redirect(w, r, externalReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	extID, versionNo, err := externalVersionPinOfPath(r)
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	workItemID, err := id.Parse(r.PathValue("work_item_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	if text := refusalText(r); text != "" {
		setFlashError(w, tenant, text)
		http.Redirect(w, r, externalReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	value, err := run(actor, tenant, caseID, workItemID, extID, versionNo, r)
	if err != nil {
		setFlashError(w, tenant, externalCommitmentsFormMessage(err))
		http.Redirect(w, r, externalReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, externalReviewTarget(tenant, caseID, successFlag, value), http.StatusSeeOther)
}

// approveExternalCommitmentForm answers the page approval form of one
// version card: the explicit confirmation checkbox (the deliberate act of
// the current human owner), then the service command (the step-up gate
// and the owner gate live there).
func (s *Server) approveExternalCommitmentForm(w http.ResponseWriter, r *http.Request) {
	s.externalVersionFormResult(w, r, func(r *http.Request) string {
		if r.FormValue("confirm_approve") != "yes" {
			return "Утверждение версии требует явного подтверждения: отметьте чекбокс согласия."
		}
		return ""
	}, "approved", func(actor domain.Principal, tenant, caseID, workItemID, extID id.UUID, versionNo int64, _ *http.Request) (string, error) {
		outcome, err := s.externalCommitments.ApproveExternalCommitment(r.Context(), casesAPI.ApproveExternalCommitmentCommand{
			Actor: actor, TenantID: tenant, CaseID: caseID,
			WorkItemID: workItemID, ExtCommitmentID: extID, VersionNo: versionNo, Now: s.clock.Now().UTC(),
		})
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(outcome.VersionNo, 10), nil
	})
}

// enqueueExternalCommitmentForm answers the page enqueue form of one
// version card: the explicit confirmation checkbox (the send intent —
// the deliberate act), then the service command (the approval gate of
// the EXACT version and the readiness gate refuse before the
// persist-before-emit transaction).
func (s *Server) enqueueExternalCommitmentForm(w http.ResponseWriter, r *http.Request) {
	s.externalVersionFormResult(w, r, func(r *http.Request) string {
		if r.FormValue("confirm_enqueue") != "yes" {
			return "Постановка в очередь требует явного подтверждения: отметьте чекбокс согласия."
		}
		return ""
	}, "enqueued", func(actor domain.Principal, tenant, caseID, workItemID, extID id.UUID, versionNo int64, _ *http.Request) (string, error) {
		outcome, err := s.externalCommitments.EnqueueExternalCommitment(r.Context(), casesAPI.EnqueueExternalCommitmentCommand{
			Actor: actor, TenantID: tenant, CaseID: caseID,
			WorkItemID: workItemID, ExtCommitmentID: extID, VersionNo: versionNo, Now: s.clock.Now().UTC(),
		})
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(outcome.VersionNo, 10), nil
	})
}

// relayExternalReceiptForm answers the page relay form of one version
// card: the observation radios of the three-word channel dictionary
// (an unselected radio is the UI refusal; a foreign word travels to the
// service where the domain guard refuses it with the honest words),
// then the service command (the append-only history accepts the
// observation whatever came before — the state follows from the history
// at read time).
func (s *Server) relayExternalReceiptForm(w http.ResponseWriter, r *http.Request) {
	s.externalVersionFormResult(w, r, func(r *http.Request) string {
		if r.FormValue("observation") == "" {
			return "Передайте наблюдение канала: выберите одно слово — доставлено, принято или отказ."
		}
		return ""
	}, "relayed", func(actor domain.Principal, tenant, caseID, workItemID, extID id.UUID, versionNo int64, r *http.Request) (string, error) {
		outcome, err := s.externalCommitments.RelayExternalReceipt(r.Context(), casesAPI.RelayExternalReceiptCommand{
			Actor: actor, TenantID: tenant, CaseID: caseID,
			ExtCommitmentID: extID, VersionNo: versionNo,
			Observation: r.FormValue("observation"), Now: s.clock.Now().UTC(),
		})
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(outcome.VersionNo, 10), nil
	})
}

// drainExternalCommitmentsForm answers the page drain form: the bounded
// operator command over the pending queue (no daemons exist — AD-13).
// The AG-4 refusal words land on the page as the flash (the production
// root wires no dispatcher — the records stay queued, the state stays
// honest); the successful run answers PRG with the considered counter.
func (s *Server) drainExternalCommitmentsForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	if s.externalCommitments == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.externalCommitmentsProblem(w, r, fmt.Errorf("%w: %s", domain.ErrForbidden, "форма диспетчеризации доступна только из браузера приложения (origin)."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases", http.StatusSeeOther)
		return
	}
	caseID, err := id.Parse(r.FormValue("case_id"))
	if err != nil {
		setFlashError(w, tenant, "Не удалось вернуться на страницу конвертов: страница кейса не найдена.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases", http.StatusSeeOther)
		return
	}
	if r.FormValue("confirm_drain") != "yes" {
		setFlashError(w, tenant, "Диспетчеризация требует явного подтверждения: отметьте чекбокс согласия.")
		http.Redirect(w, r, externalReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	result, err := s.externalCommitments.DrainExternalCommitments(r.Context(), actor, tenant)
	if err != nil {
		setFlashError(w, tenant, externalCommitmentsFormMessage(err))
		http.Redirect(w, r, externalReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, externalReviewTarget(tenant, caseID, "drain", strconv.Itoa(result.Considered)), http.StatusSeeOther)
}
