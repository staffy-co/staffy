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

	actionAPI "github.com/artemiosu/staffy/internal/action/api"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 3.4 transport: the machine JSON contract of the action
// preview preparation contour and the operator Preview Review page. The
// gate ladder lives in the app service and runs FIRST: a non-member of
// the tenant is the honest 404 (before any material), a member without
// the review-or-configuration scope is the 403 with the scope words
// BEFORE any material. The decode is strict: one JSON object, known
// fields only, Content-Type application/json (the 3.3 precedent).
//
// The AD-7 verdict ladder maps onto the status codes honestly: the FIRST
// typed-invalid attempt answers 400 with the full violation word list
// (the author reads the exact fields back), the TERMINAL repeat answers
// 409 (the third attempt does not exist — the structural bound), the
// policy-blocked typed-valid attempt answers 403 with the exact reason
// word of the ten-word dictionary, and only the passing attempt answers
// 200 with the built version pins and the digest. The edit answers the
// 400/403/409 rule words BEFORE any write (the old version stays fully
// current); the dry-run of the nil adapter is the honest AG-4 refusal
// (400 in words, no evidence row). The Timeline words are the read-time
// derivations of the append-only events — dry-run evidence NEVER moves
// them (AC-8) and the page says so in the fixed domain words.
//
// The page forms are the PRG family with the focus transfer onto the
// notice (the 3.3 precedent); the explicit confirmation checkboxes are
// the UI discipline of the dry-run form (the server-side protection is
// the rights gate — the double discipline, documented honestly; the
// machine API carries no checkbox field). The PRIMARY action of every
// current card is the transition to the authorization — the 3-5 contour
// does not exist yet, the transition explains itself in the fixed
// domain words (the «диспетчер не сконфигурирован» precedent 3-3 — the
// interface never pretends; a hidden dispatch does not exist).
// ---------------------------------------------------------------------------

// actionPreviewsProblem maps the preparation contour errors onto the
// problem contract: the action module's RuleError is the 400 family with
// the rule name as the code and the honest reason (the dictionary words —
// internal details never leak as text); the module's ErrNotFound and the
// identity ErrNotFound are the plain 404 (no disclosure); the identity
// forbidden sentinel is the 403 whose message IS the scope words the app
// attached at the wrap point (the transport never invents its own copy);
// the exhausted attempt ladder and the raced version ladder answer 409;
// the digest integrity alarm is the loud 500 with the server-side log.
func (s *Server) actionPreviewsProblem(w http.ResponseWriter, r *http.Request, err error) {
	var rule actionDomain.RuleError
	switch {
	case errors.As(err, &rule):
		s.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, actionDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		s.problem(w, r, http.StatusNotFound, "not_found", "Обращение, версия preview или определение действия не найдены.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		message := err.Error()
		if prefix := domain.ErrForbidden.Error() + ": "; strings.HasPrefix(message, prefix) {
			message = strings.TrimPrefix(message, prefix)
		}
		s.problem(w, r, http.StatusForbidden, "action_previews_forbidden", message, false, nil)
	case errors.Is(err, actionPort.ErrAttemptExhausted):
		s.problem(w, r, http.StatusConflict, "proposal_attempts_exhausted", "Обе попытки этого предложения исчерпаны — третьей не существует: отредактируйте построенный preview либо подайте предложение с другим пином.", false, nil)
	case errors.Is(err, actionPort.ErrStalePreviewVersion):
		s.problem(w, r, http.StatusConflict, "stale_preview_version", "Лестница версий двинулась параллельным прогоном: перечитайте страницу и повторите команду.", false, nil)
	case errors.Is(err, actionPort.ErrPreviewIntegrity):
		s.logger.ErrorContext(r.Context(), "preview digest integrity alarm", "code", "preview_integrity_alarm", "error", err.Error())
		s.problem(w, r, http.StatusInternalServerError, "preview_integrity_alarm", "Digest версии не перепровернился: строка не совпадает с построенным материалом — передайте в проверку целостности.", false, nil)
	default:
		s.logger.ErrorContext(r.Context(), "action preview command failed", "code", "action_preview_failed", "error", err.Error())
		s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить команду preview. Повторите позднее.", true, nil)
	}
}

// actionPreviewsFormMessage renders the honest flash text of the form
// errors (the 3.3 precedent: the rule words, the 404/403 texts, the
// ladder conflicts).
func actionPreviewsFormMessage(err error) string {
	var rule actionDomain.RuleError
	switch {
	case errors.As(err, &rule):
		return rule.Reason
	case errors.Is(err, actionDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		return "Обращение, версия preview или определение действия не найдены: перечитайте страницу и повторите."
	case errors.Is(err, domain.ErrForbidden):
		return "Подготовка action preview требует полномочия review или configuration."
	case errors.Is(err, actionPort.ErrAttemptExhausted):
		return "Обе попытки этого предложения исчерпаны — третьей не существует: отредактируйте построенный preview либо подайте предложение с другим пином."
	case errors.Is(err, actionPort.ErrStalePreviewVersion):
		return "Лестница версий двинулась параллельным прогоном: перечитайте страницу и повторите."
	}
	return "Команда preview не выполнена. Повторите позднее."
}

// previewReviewTarget answers the PRG target of the page forms — the
// Preview Review page of the case with the one success flag (the notice
// speaks the words, the focus moves onto it).
func previewReviewTarget(tenant, caseID id.UUID, flag string, value string) string {
	return "/app/tenants/" + tenant.String() + "/cases/" + caseID.String() + "/action-previews?" + flag + "=" + value
}

// ---------------------------------------------------------------------------
// The machine JSON contract
// ---------------------------------------------------------------------------

// submitProposalInput is the strict machine shape of the proposal
// submission. The Work Item anchor travels in the path; the definition
// pin and the typed pins travel in the body; the shape refusals (the
// uuid fields, the epoch) are the transport's honest 400; the dictionary
// and the schema words travel to the service — the domain guard is the
// ONE source of the refusal text.
type submitProposalInput struct {
	DefinitionID           string            `json:"definition_id"`
	DefinitionVersion      string            `json:"definition_version"`
	TargetKind             string            `json:"target_kind"`
	TargetRef              string            `json:"target_ref"`
	TypedInput             map[string]string `json:"typed_input"`
	SupportingFacts        []string          `json:"supporting_facts"`
	SupportingObservations []string          `json:"supporting_observations"`
	OwnerEpoch             int64             `json:"owner_epoch"`
}

// newPreviewVersionInput is the strict machine shape of the edit: the
// typed material and the supporting references rewrite; the pins of the
// thread (the definition, the case, the Work Item, the target) are the
// preview's own identity — structurally immutable per thread.
type newPreviewVersionInput struct {
	TypedInput             map[string]string `json:"typed_input"`
	SupportingFacts        []string          `json:"supporting_facts"`
	SupportingObservations []string          `json:"supporting_observations"`
}

// typedInputFromJSON pairs the typed-input map rows (the strict machine
// shape of the submit).
func typedInputFromJSON(input map[string]string) map[string]string {
	// The machine shape IS the map: an unknown field inside it refuses at
	// the domain guard with the exact field name (the guard is the ONE
	// source; the transport relays the map without touching the values).
	return input
}

// typedInputFromPairs parses the "field=value" text pairs of the page
// form into the typed-input map (the honest page shape of a schema-free
// form: one pair per line, the = separator; a malformed pair answers
// the honest parse refusal, an unknown field refuses at the domain
// guard by name).
func typedInputFromPairs(raw string) (map[string]string, error) {
	input := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("строка «%s» не является парой поле=значение", line)
		}
		input[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}
	return input, nil
}

// typedInputToPairs renders the projected typed input of a card back
// into the form pairs (the honest refill of the edit form; the
// deterministic order goes through the package-wide sortStrings).
func typedInputToPairs(input map[string]string) string {
	names := make([]string, 0, len(input))
	for name := range input {
		names = append(names, name)
	}
	sortStrings(names)
	pairs := make([]string, 0, len(names))
	for _, name := range names {
		pairs = append(pairs, name+"="+input[name])
	}
	return strings.Join(pairs, "\n")
}

// refListFromJSON parses the machine uuid list.
func refListFromJSON(values []string) ([]id.UUID, error) {
	refs := make([]id.UUID, 0, len(values))
	for _, raw := range values {
		ref, err := id.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("ссылка «%s» — действительный UUID", raw)
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// refListFromCSV parses the comma-separated uuid list of the page form.
func refListFromCSV(raw string) ([]id.UUID, error) {
	values := strings.Split(raw, ",")
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed2 := strings.TrimSpace(value); trimmed2 != "" {
			trimmed = append(trimmed, trimmed2)
		}
	}
	if len(trimmed) == 0 {
		return nil, nil
	}
	return refListFromJSON(trimmed)
}

// decodeActionPreviewsJSON strictly decodes one JSON object (the 3.3
// precedent spoken through the action RuleError so the 400 words have
// one shape).
func decodeActionPreviewsJSON(w http.ResponseWriter, r *http.Request, out any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return actionDomain.RuleError{Rule: "content_type", Reason: "Используйте Content-Type application/json."}
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return actionDomain.RuleError{Rule: "body", Reason: "Проверьте JSON-запрос: один объект с известными полями."}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return actionDomain.RuleError{Rule: "body", Reason: "Ожидается ровно один JSON-объект без данных после него."}
	}
	return nil
}

// previewVersionPinOfPath parses the preview path pin: the stable thread
// identity and the EXACT version number (the anchors of the dry-run
// command; the edit addresses the thread and reads the current version
// itself).
func previewVersionPinOfPath(r *http.Request) (id.UUID, int64, error) {
	previewID, err := id.Parse(r.PathValue("preview_id"))
	if err != nil {
		return id.UUID(""), 0, err
	}
	versionNo, err := strconv.ParseInt(r.PathValue("version_no"), 10, 64)
	if err != nil || versionNo < 1 {
		return id.UUID(""), 0, fmt.Errorf("версия — целое число начиная с 1")
	}
	return previewID, versionNo, nil
}

// submitProposalStatus maps the AD-7 outcome onto the honest status: the
// typed-invalid attempt is the 400 of the word list, the terminal repeat
// is the 409, the policy block of a typed-valid attempt is the 403 of
// the reason word, the passing build is the 200 (the body always carries
// the SAME outcome shape — the status is the semantics, the shape is the
// contract).
func submitProposalStatus(outcome actionAPI.ProposalOutcome) int {
	switch {
	case outcome.Verdict == actionDomain.VerdictInvalid:
		return http.StatusBadRequest
	case outcome.Verdict == actionDomain.VerdictRejected:
		return http.StatusConflict
	case outcome.BlockedReason != "":
		return http.StatusForbidden
	default:
		return http.StatusOK
	}
}

// readPreviewViewAPI answers the machine read of the Preview Review
// surface: the threads with the version cards (the live read-time
// derivations — the readiness, the freshness with the exact reason word,
// the Timeline words), the attempt ladders, the dry-run evidence and the
// material diffs (the honest empty threads list of a case without
// previews — never a null).
func (s *Server) readPreviewViewAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.actionPreviews == nil {
		s.notFound(w, r, actor)
		return
	}
	view, err := s.actionPreviews.ReadPreviewView(r.Context(), actor, tenant, caseID)
	if err != nil {
		s.actionPreviewsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"previews": view})
}

// readPreviewThreadAPI answers the machine read of ONE preparation
// thread: the stable preview identity with its version cards, the
// attempt ladder and the validation evidence — the material of the
// future Confirmation references of 3-5 (the (preview_id, version_no)
// pins).
func (s *Server) readPreviewThreadAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.actionPreviews == nil {
		s.notFound(w, r, actor)
		return
	}
	previewID, err := id.Parse(r.PathValue("preview_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	view, err := s.actionPreviews.ReadPreviewView(r.Context(), actor, tenant, caseID)
	if err != nil {
		s.actionPreviewsProblem(w, r, err)
		return
	}
	for _, thread := range view.Threads {
		if thread.PreviewID == previewID {
			writeJSON(w, http.StatusOK, map[string]any{"preview": thread})
			return
		}
	}
	s.notFound(w, r, actor)
}

// submitProposalAPI answers the machine proposal submission: the strict
// decode, the pinned definition, the frozen gate order in the service,
// and the honest verdict ladder of the status codes.
func (s *Server) submitProposalAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.actionPreviews == nil {
		s.notFound(w, r, actor)
		return
	}
	workItemID, err := id.Parse(r.PathValue("work_item_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	var input submitProposalInput
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.actionPreviewsProblem(w, r, err)
		return
	}
	targetRef, err := id.Parse(input.TargetRef)
	if err != nil {
		s.actionPreviewsProblem(w, r, actionDomain.RuleError{Rule: "target_ref", Reason: "Ссылка цели — действительный UUID."})
		return
	}
	supportingFacts, err := refListFromJSON(input.SupportingFacts)
	if err != nil {
		s.actionPreviewsProblem(w, r, actionDomain.RuleError{Rule: "supporting_facts", Reason: err.Error()})
		return
	}
	supportingObservations, err := refListFromJSON(input.SupportingObservations)
	if err != nil {
		s.actionPreviewsProblem(w, r, actionDomain.RuleError{Rule: "supporting_observations", Reason: err.Error()})
		return
	}
	outcome, err := s.actionPreviews.SubmitProposal(r.Context(), actionAPI.SubmitProposalCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		DefinitionID: input.DefinitionID, DefinitionVersion: input.DefinitionVersion,
		Target:                 actionDomain.ProposalTarget{Kind: input.TargetKind, Ref: targetRef},
		TypedInput:             typedInputFromJSON(input.TypedInput),
		SupportingFacts:        supportingFacts,
		SupportingObservations: supportingObservations,
		OwnerEpoch:             input.OwnerEpoch,
		Now:                    s.clock.Now().UTC(),
	})
	if err != nil {
		s.actionPreviewsProblem(w, r, err)
		return
	}
	writeJSON(w, submitProposalStatus(outcome), map[string]any{"proposal": outcome})
}

// newPreviewVersionAPI answers the machine edit of one thread: the
// edited typed material and supporting references re-run the FULL chain
// (the validation, the policy, the fresh snapshot) and land as the NEXT
// append-only version with the atomic supersede of the old one; the
// refusals answer BEFORE any write (the old version stays fully
// current).
func (s *Server) newPreviewVersionAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.actionPreviews == nil {
		s.notFound(w, r, actor)
		return
	}
	previewID, err := id.Parse(r.PathValue("preview_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	var input newPreviewVersionInput
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.actionPreviewsProblem(w, r, err)
		return
	}
	supportingFacts, err := refListFromJSON(input.SupportingFacts)
	if err != nil {
		s.actionPreviewsProblem(w, r, actionDomain.RuleError{Rule: "supporting_facts", Reason: err.Error()})
		return
	}
	supportingObservations, err := refListFromJSON(input.SupportingObservations)
	if err != nil {
		s.actionPreviewsProblem(w, r, actionDomain.RuleError{Rule: "supporting_observations", Reason: err.Error()})
		return
	}
	outcome, err := s.actionPreviews.NewPreviewVersionFromEdit(r.Context(), actionAPI.NewPreviewVersionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, PreviewID: previewID,
		TypedInput:             input.TypedInput,
		SupportingFacts:        supportingFacts,
		SupportingObservations: supportingObservations,
		Now:                    s.clock.Now().UTC(),
	})
	if err != nil {
		s.actionPreviewsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"edit": outcome})
}

// recordDryRunAPI answers the machine dry-run of one EXACT version: the
// validation evidence lands append-only; the Timeline words NEVER move
// (AC-8); the production nil adapter is the honest 400 in words (AG-4 —
// no evidence row is ever created).
func (s *Server) recordDryRunAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.actionPreviews == nil {
		s.notFound(w, r, actor)
		return
	}
	previewID, versionNo, err := previewVersionPinOfPath(r)
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	outcome, err := s.actionPreviews.RecordDryRun(r.Context(), actionAPI.DryRunCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		PreviewID: previewID, VersionNo: versionNo, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.actionPreviewsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dry_run": outcome})
}

// ---------------------------------------------------------------------------
// The Preview Review page
// ---------------------------------------------------------------------------

// previewWordOption is one dictionary option of the page forms.
type previewWordOption struct {
	Word string
	Text string
}

// previewTargetKinds is the closed target-kind dictionary of the form
// (the step-01 domain words with the honest descriptions).
var previewTargetKinds = []previewWordOption{
	{Word: actionDomain.TargetCatalogItem, Text: "карточка каталога — товар владеющей системы catalog"},
	{Word: actionDomain.TargetClient, Text: "клиент кейса"},
	{Word: actionDomain.TargetOrder, Text: "заказ ERP"},
}

// previewSnapshotRowView is one material snapshot row of the card.
type previewSnapshotRowView struct {
	MaterialField   string
	Status          string
	ExternalID      string
	ExternalVersion string
	ObservedAt      string
	EvidenceRef     string
}

// previewEffectRowView is one effect row of the card.
type previewEffectRowView struct {
	Order            string
	ActionRef        string
	DependsOn        string
	FailureSemantics string
}

// previewAttemptRowView is one attempt ladder row of the thread.
type previewAttemptRowView struct {
	No            string
	Verdict       string
	BlockedText   string
	InvalidReport string
	At            string
}

// previewEvidenceRowView is one dry-run evidence row of the thread.
type previewEvidenceRowView struct {
	Version string
	Result  string
	At      string
}

// previewCardRowView is the page projection of one version card: every
// state spoken in TEXT (never by the colour alone), the diff lines of
// the edit that produced it, and the authorization-absence words of the
// primary action.
type previewCardRowView struct {
	PreviewID         string
	No                string
	Supersedes        string
	DefinitionText    string
	TargetText        string
	TypedInputPairs   string
	FactsText         string
	ObservationsText  string
	SnapshotRows      []previewSnapshotRowView
	EffectRows        []previewEffectRowView
	PartitionText     string
	RecoveryText      string
	HumanOwnerText    string
	UnsafeText        string
	NoReconText       string
	ControlMarkText   string
	ReadinessWord     string
	ReadinessText     string
	FreshnessText     string
	TimelineWord      string
	TimelineText      string
	CurrentText       string
	Digest            string
	DigestAlarmText   string
	AuthorizationText string
	DiffLines         []string
	Attempts          []previewAttemptRowView
	Evidence          []previewEvidenceRowView
	HasSnapshot       bool
	HasEffects        bool
	Current           bool
}

// previewThreadRowView is one thread section of the page.
type previewThreadRowView struct {
	PreviewID string
	Cards     []previewCardRowView
}

// previewDefinitionOption is one catalog option of the proposal form.
type previewDefinitionOption struct {
	Pin  string
	Name string
}

// previewFormState carries the proposal form values for the honest
// re-render (the operator corrects one field, not the whole form).
type previewFormState struct {
	WorkItem     string
	Definition   string
	TargetKind   string
	TargetRef    string
	TypedInput   string
	Facts        string
	Observations string
	OwnerEpoch   string
}

// previewReviewPageView carries the page material: the tenant and the
// case, the threads, the honest empty state, the form dictionaries, the
// flash error and the PRG notice.
type previewReviewPageView struct {
	TenantID          string
	CaseID            string
	HasThreads        bool
	Threads           []previewThreadRowView
	PanelText         string
	EmptyText         string
	Flash             string
	Notice            string
	HasWorkItems      bool
	WorkItemIDs       []string
	HasDefinitions    bool
	DefinitionOptions []previewDefinitionOption
	TargetKindOptions []previewWordOption
	Form              previewFormState
}

// previewNotice speaks the PRG status of a successful form mutation: the
// flag carries the built version (or the validation result), the words
// name the discipline of the step that landed (the focus moves onto the
// notice).
func previewNotice(r *http.Request) string {
	if raw := r.URL.Query().Get("created"); raw != "" {
		return "Preview версия 1 построена из предложения: строгая типовая валидация прошла, Policy-гейты живых источников зелёные, материальный snapshot записан. Timeline — previewed; скрытой отправки не существует. Фокус переведён на этот статус."
	}
	if raw := r.URL.Query().Get("submitted"); raw != "" {
		return "Предложение зафиксировано (попытка " + raw + "): смотрите вердикт и слова violations на карточке ниже. Фокус переведён на этот статус."
	}
	if raw := r.URL.Query().Get("edited"); raw != "" {
		return "Версия " + raw + " построена полной цепочкой (валидация + Policy + свежий snapshot); прежняя версия заменена — читаема для аудита, к авторизации не пригодна. Фокус переведён на этот статус."
	}
	if raw := r.URL.Query().Get("validated"); raw != "" {
		return "Dry-run версии " + raw + " записан как validation evidence: это не выполненное действие и не гарантия будущего успеха; слова Timeline не смещаются. Фокус переведён на этот статус."
	}
	return ""
}

// newPreviewCardRowView maps one api card onto the page projection: the
// words with the texts, the diff lines, the attempts, the evidence, and
// the honest marker texts of the human-only partition.
func newPreviewCardRowView(card actionAPI.PreviewCardView) previewCardRowView {
	row := previewCardRowView{
		PreviewID:         card.PreviewID.String(),
		No:                strconv.FormatInt(card.VersionNo, 10),
		DefinitionText:    card.DefinitionID + "@" + card.DefinitionVersion,
		TargetText:        card.TargetKind + " " + card.TargetRef.String(),
		TypedInputPairs:   typedInputToPairs(card.TypedInput),
		PartitionText:     card.PartitionText,
		ReadinessWord:     card.Readiness,
		ReadinessText:     card.ReadinessText,
		TimelineWord:      card.Timeline,
		TimelineText:      card.TimelineText,
		Current:           card.Current,
		CurrentText:       map[bool]string{true: "актуальная версия", false: "заменена новой версией"}[card.Current],
		Digest:            card.Digest,
		AuthorizationText: card.AuthorizationAbsenceText,
		DiffLines:         card.DiffLines,
	}
	if card.SupersedesVersion > 0 {
		row.Supersedes = strconv.FormatInt(card.SupersedesVersion, 10)
	}
	if card.FreshnessStale {
		row.FreshnessText = "устарела: " + card.FreshnessReasonText
	} else {
		row.FreshnessText = "свежая — живые источники совпадают с пинами"
	}
	if card.DigestAlarm {
		row.DigestAlarmText = "ВНИМАНИЕ: digest версии не перепровернился — материал строки не совпадает с построенным; передайте в проверку целостности."
	}
	if card.RecoveryInstruction != "" {
		row.RecoveryText = "Инструкция восстановления: " + card.RecoveryInstruction
	}
	if card.HumanOwnerRequired {
		row.HumanOwnerText = "обязательный человек-владелец: действие исполняет только человек"
	}
	if card.UnsafeClass {
		row.UnsafeText = "класс доставки unsafe — действие остаётся в человеческом контуре"
	}
	if card.NoReconciliation {
		row.NoReconText = "доказанного reconciliation path нет — действие human-only"
	}
	if card.HumanOnlyControlMark {
		row.ControlMarkText = "scoped control: human_only — подготовка разрешена под человеком"
	}
	factPins := make([]string, 0, len(card.SupportingFacts))
	for _, pin := range card.SupportingFacts {
		factPins = append(factPins, pin.Ref.String()+":"+strconv.FormatInt(pin.Version, 10))
	}
	row.FactsText = strings.Join(factPins, ", ")
	observationPins := make([]string, 0, len(card.SupportingObservations))
	for _, pin := range card.SupportingObservations {
		observationPins = append(observationPins, pin.Ref.String()+":"+strconv.FormatInt(pin.Version, 10))
	}
	row.ObservationsText = strings.Join(observationPins, ", ")
	row.HasSnapshot = len(card.Snapshot) > 0
	for _, snapshot := range card.Snapshot {
		row.SnapshotRows = append(row.SnapshotRows, previewSnapshotRowView{
			MaterialField: snapshot.MaterialField, Status: snapshot.Status,
			ExternalID: snapshot.ExternalID, ExternalVersion: snapshot.ExternalVersion,
			ObservedAt: snapshot.ObservedAt.UTC().Format("2006-01-02T15:04:05Z07:00"), EvidenceRef: snapshot.EvidenceRef,
		})
	}
	row.HasEffects = len(card.Effects) > 0
	for _, effect := range card.Effects {
		row.EffectRows = append(row.EffectRows, previewEffectRowView{
			Order: strconv.Itoa(effect.Order), ActionRef: effect.ActionRef,
			DependsOn: strings.Join(intsToStrings(effect.DependsOn), ","), FailureSemantics: effect.FailureSemantics,
		})
	}
	return row
}

// intsToStrings renders the depends-on orders.
func intsToStrings(values []int) []string {
	rendered := make([]string, 0, len(values))
	for _, value := range values {
		rendered = append(rendered, strconv.Itoa(value))
	}
	return rendered
}

// previewReviewPage renders the Preview Review page of the case: the
// threads with the version cards (the live derivations spoken in text,
// the diffs, the attempt ladders, the validation evidence), the proposal
// form with the catalog options, and the per-card edit/dry-run forms.
// The PRIMARY action of every current card is the honest transition to
// the authorization words (the 3-5 contour does not exist — the
// interface never pretends).
func (s *Server) previewReviewPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.actionPreviews == nil {
		s.notFound(w, r, actor)
		return
	}
	view, err := s.actionPreviews.ReadPreviewView(r.Context(), actor, tenant, caseID)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	flash := flashError(r, tenant)
	if flash != "" {
		http.SetCookie(w, clearFlashCookie(tenant))
	}
	page := previewReviewPageView{
		TenantID: tenant.String(), CaseID: caseID.String(),
		PanelText: view.PanelText, EmptyText: view.EmptyText, Flash: flash,
		Notice:            previewNotice(r),
		Threads:           []previewThreadRowView{},
		TargetKindOptions: previewTargetKinds,
	}
	for _, workItemID := range view.WorkItemIDs {
		page.HasWorkItems = true
		page.WorkItemIDs = append(page.WorkItemIDs, workItemID.String())
	}
	// The catalog options of the proposal form (the 3-1 read surface —
	// the honest empty select when the catalog carries no definitions).
	if s.actionCatalog != nil {
		if defs, err := s.actionCatalog.List(r.Context(), actor, tenant); err == nil {
			for _, def := range defs {
				page.HasDefinitions = true
				page.DefinitionOptions = append(page.DefinitionOptions, previewDefinitionOption{
					Pin: def.ID + "@" + def.Version, Name: def.Name,
				})
			}
		}
	}
	attemptsByThread := map[id.UUID][]previewAttemptRowView{}
	evidenceByThread := map[id.UUID][]previewEvidenceRowView{}
	for _, thread := range view.Threads {
		for _, attempt := range thread.Attempts {
			attemptsByThread[thread.PreviewID] = append(attemptsByThread[thread.PreviewID], previewAttemptRowView{
				No: strconv.Itoa(attempt.AttemptNo), Verdict: attempt.Verdict,
				BlockedText: attempt.BlockedReason, InvalidReport: attempt.InvalidReport,
				At: attempt.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			})
		}
		for _, evidence := range thread.DryRunEvidence {
			evidenceByThread[thread.PreviewID] = append(evidenceByThread[thread.PreviewID], previewEvidenceRowView{
				Version: strconv.FormatInt(evidence.VersionNo, 10), Result: evidence.Result,
				At: evidence.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			})
		}
	}
	for _, thread := range view.Threads {
		page.HasThreads = true
		threadRow := previewThreadRowView{PreviewID: thread.PreviewID.String(), Cards: []previewCardRowView{}}
		for _, card := range thread.Cards {
			cardRow := newPreviewCardRowView(card)
			cardRow.Attempts = attemptsByThread[thread.PreviewID]
			cardRow.Evidence = evidenceByThread[thread.PreviewID]
			threadRow.Cards = append(threadRow.Cards, cardRow)
		}
		page.Threads = append(page.Threads, threadRow)
	}
	s.render(w, "action_previews", page)
}

// splitDefinitionPin cuts the id@version select value into the pin pair.
func splitDefinitionPin(pin string) (string, string) {
	idPart, version, found := strings.Cut(pin, "@")
	if !found {
		return idPart, ""
	}
	return idPart, version
}

// submitProposalForm answers the page proposal form: the origin gate, the
// strict parse, the service command; every typed-invalid verdict
// re-renders the page through the PRG flash with the honest violation
// words and the refilled form (the operator corrects one field); the
// passing build lands PRG with the created flag.
func (s *Server) submitProposalForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.actionPreviews == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.actionPreviewsProblem(w, r, fmt.Errorf("%w: %s", domain.ErrForbidden, "форма proposal доступна только из браузера приложения (origin)."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы.")
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	workItemID, err := id.Parse(r.FormValue("work_item_id"))
	if err != nil {
		setFlashError(w, tenant, "Элемент работы — действительный UUID: выберите элемент из списка.")
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	definitionID, definitionVersion := splitDefinitionPin(r.FormValue("definition_pin"))
	targetRef, err := id.Parse(strings.TrimSpace(r.FormValue("target_ref")))
	if err != nil {
		setFlashError(w, tenant, "Ссылка цели — действительный UUID.")
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	typedInput, err := typedInputFromPairs(r.FormValue("typed_input"))
	if err != nil {
		setFlashError(w, tenant, "Типовой материал — пары поле=значение, по одной в строке: "+err.Error())
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	supportingFacts, err := refListFromCSV(r.FormValue("supporting_facts"))
	if err != nil {
		setFlashError(w, tenant, "Поддерживающие факты — список UUID через запятую: "+err.Error())
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	supportingObservations, err := refListFromCSV(r.FormValue("supporting_observations"))
	if err != nil {
		setFlashError(w, tenant, "Поддерживающие наблюдения — список UUID через запятую: "+err.Error())
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	ownerEpoch, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("owner_epoch")), 10, 64)
	if err != nil {
		setFlashError(w, tenant, "Эпоха владельца ответа — живое наблюдение (положительное целое число).")
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	outcome, err := s.actionPreviews.SubmitProposal(r.Context(), actionAPI.SubmitProposalCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		DefinitionID: definitionID, DefinitionVersion: definitionVersion,
		Target:                 actionDomain.ProposalTarget{Kind: r.FormValue("target_kind"), Ref: targetRef},
		TypedInput:             typedInput,
		SupportingFacts:        supportingFacts,
		SupportingObservations: supportingObservations,
		OwnerEpoch:             ownerEpoch,
		Now:                    s.clock.Now().UTC(),
	})
	if err != nil {
		setFlashError(w, tenant, actionPreviewsFormMessage(err))
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	switch {
	case outcome.Verdict == actionDomain.VerdictInvalid || outcome.Verdict == actionDomain.VerdictRejected:
		// The honest violation words of the typed ladder land as the
		// flash; the attempt counter shows on the page ladder.
		verdictText := "Невалидный материал (попытка " + strconv.Itoa(outcome.AttemptNo) + ")"
		if outcome.Verdict == actionDomain.VerdictRejected {
			verdictText = "Повторно невалидный материал — предложение терминально отклонено (попытки исчерпаны)"
		}
		setFlashError(w, tenant, verdictText+": "+strings.Join(outcome.Violations, "; "))
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
	case outcome.BlockedReason != "":
		setFlashError(w, tenant, "Policy-гейт заблокировал подготовку ("+outcome.BlockedReason+"): "+actionDomain.PolicyReasonText(outcome.BlockedReason))
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
	default:
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "created", strconv.FormatInt(outcome.VersionNo, 10)), http.StatusSeeOther)
	}
}

// newPreviewVersionForm answers the page edit form of one thread: the
// edited typed pairs and references re-run the full chain; the honest
// refusal words land as the flash, the success lands PRG with the new
// version number.
func (s *Server) newPreviewVersionForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.actionPreviews == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.actionPreviewsProblem(w, r, fmt.Errorf("%w: %s", domain.ErrForbidden, "форма правки доступна только из браузера приложения (origin)."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы.")
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	previewID, err := id.Parse(r.PathValue("preview_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	typedInput, err := typedInputFromPairs(r.FormValue("typed_input"))
	if err != nil {
		setFlashError(w, tenant, "Типовой материал — пары поле=значение: "+err.Error())
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	supportingFacts, err := refListFromCSV(r.FormValue("supporting_facts"))
	if err != nil {
		setFlashError(w, tenant, "Поддерживающие факты — список UUID через запятую: "+err.Error())
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	supportingObservations, err := refListFromCSV(r.FormValue("supporting_observations"))
	if err != nil {
		setFlashError(w, tenant, "Поддерживающие наблюдения — список UUID через запятую: "+err.Error())
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	outcome, err := s.actionPreviews.NewPreviewVersionFromEdit(r.Context(), actionAPI.NewPreviewVersionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, PreviewID: previewID,
		TypedInput:             typedInput,
		SupportingFacts:        supportingFacts,
		SupportingObservations: supportingObservations,
		Now:                    s.clock.Now().UTC(),
	})
	if err != nil {
		setFlashError(w, tenant, actionPreviewsFormMessage(err))
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, previewReviewTarget(tenant, caseID, "edited", strconv.FormatInt(outcome.VersionNo, 10)), http.StatusSeeOther)
}

// recordDryRunForm answers the page dry-run form of one version card: the
// explicit confirmation checkbox (the deliberate act — the UI discipline
// the machine API does not carry), then the service command (the nil
// adapter of the production root refuses in words, no evidence row).
func (s *Server) recordDryRunForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.actionPreviews == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.actionPreviewsProblem(w, r, fmt.Errorf("%w: %s", domain.ErrForbidden, "форма dry-run доступна только из браузера приложения (origin)."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы.")
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	previewID, versionNo, err := previewVersionPinOfPath(r)
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	if r.FormValue("confirm_dry_run") != "yes" {
		setFlashError(w, tenant, "Dry-run требует явного подтверждения: отметьте чекбокс согласия.")
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	outcome, err := s.actionPreviews.RecordDryRun(r.Context(), actionAPI.DryRunCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		PreviewID: previewID, VersionNo: versionNo, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		setFlashError(w, tenant, actionPreviewsFormMessage(err))
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	if outcome.Unsupported {
		// The honest AG-4 boundary reached the form path: the refusal is
		// spoken by the service words already; the redirect stays on the
		// x flag (no evidence exists).
		http.Redirect(w, r, previewReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, previewReviewTarget(tenant, caseID, "validated", strconv.FormatInt(versionNo, 10)), http.StatusSeeOther)
}

// BindActionPreviews registers the preparation contour surface: the five
// machine paths (the case view, the thread read, the proposal submission,
// the edit, the dry-run) and the operator Preview Review page with the
// PRG forms. The service is the narrow api.PreviewService (the
// ExternalCommitmentService type-assert precedent — the app Service
// carries the compile-time proof).
func (s *Server) BindActionPreviews(service actionAPI.PreviewService) {
	s.actionPreviews = service
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}/action-previews", s.readPreviewViewAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}/action-previews/{preview_id}", s.readPreviewThreadAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/work-items/{work_item_id}/action-previews/proposals", s.submitProposalAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/action-previews/{preview_id}/versions", s.newPreviewVersionAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/action-previews/{preview_id}/versions/{version_no}/dry-runs", s.recordDryRunAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/cases/{case_id}/action-previews", s.previewReviewPage)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/action-previews/proposals", s.submitProposalForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/action-previews/{preview_id}/versions", s.newPreviewVersionForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/action-previews/{preview_id}/versions/{version_no}/dry-runs", s.recordDryRunForm)
}
