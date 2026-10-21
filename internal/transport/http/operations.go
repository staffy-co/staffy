package httptransport

import (
	"errors"
	"fmt"
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
// The Story 3.5 transport: the machine JSON contract of the
// confirmation→authorization contour and the operator Operation Review
// page. The gate ladder lives in the app service and runs FIRST: a
// non-member of the tenant is the honest 404 (before any material), a
// member without the review-or-configuration scope is the 403 with the
// scope words BEFORE any material. The decode is strict: one JSON
// object, known fields only — a free «confidence»/«assurance» field is
// the honest 400 (AG-4: the model's confidence never substitutes a
// Confirmation, the interface never accepts it).
//
// The honest status ladder of the contour: the FIRST authorization
// answers 201 with the immutable Operation; the replay of the SAME
// client key with the SAME material answers 200 (the idempotent return
// of the ORIGINAL operation — the consumption never repeats, the second
// outbox record never exists); the cross-case reservation loser answers
// 200 with the honest reference; the same key with a DIFFERENT material
// answers the 409 conflict with the visible exception; the rejection
// ladder answers 409 with the EXACT state word (superseded / revoked /
// expired / exhausted — never a masked silence); the freshness and the
// material condition answer 409 with the 3-4 dictionary reason words;
// the nil reader of the production root answers the fail-closed 403
// (AG-4 — the authorization honestly cannot happen without the fresh
// reads); the consent and the revocation are the HUMAN acts — the live
// step-up refusal is the 403 with the step-up words.
//
// The page forms are the PRG family with the focus transfer onto the
// notice (the 3.3/3.4 precedent); the explicit confirmation checkbox of
// the revocation is the UI discipline of the human act (the server-side
// protection is the LIVE step-up — the double discipline, documented
// honestly; the machine API carries no checkbox field). The authorize
// form carries the machine idempotency word ONLY — the double-click, the
// webhook repeat and the replay job carry the SAME key again (FR-29).
// ---------------------------------------------------------------------------

// operationsWrappedReason extracts the honest domain words the app
// attached at the wrap point of a store-contract sentinel (the
// «transport never invents its own copy» discipline — the wrapped text
// IS the message; the raw sentinel never leaks).
func operationsWrappedReason(err error, sentinel error) string {
	message := err.Error()
	if prefix := sentinel.Error() + ": "; strings.HasPrefix(message, prefix) {
		return strings.TrimPrefix(message, prefix)
	}
	return ""
}

// operationsProblem maps the confirmation→authorization errors onto the
// problem contract: the action module's RuleError is the 400 family
// with the rule name as the code and the honest reason (the key gate,
// the required/allowance guards, the freshness/readiness refusals of the
// consent — the dictionary words, internal details never leak as text);
// the rejection ladder and every honest race/conflict answer 409 with
// the exact words; the fail-closed fresh-reads refusal and the policy
// blocks answer 403; the module's ErrNotFound and the identity ErrNotFound
// are the plain 404 (no disclosure); the identity step-up sentinel is
// the 403 with the step-up words; the structural integrity alarm is the
// loud 500 with the server-side log. The dispatch contour (3-6 step-03)
// adds the AG-4 honest refusal of the drain command — the nil mutation
// adapter of the production root answers 403 with the honest words (the
// queue stays pending, zero sends — never a masked success).
func (s *Server) operationsProblem(w http.ResponseWriter, r *http.Request, err error) {
	var rule actionDomain.RuleError
	var rejected actionPort.ConfirmationRejectedError
	var stale actionPort.AuthorizationStaleError
	switch {
	case errors.As(err, &rule):
		s.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, actionAPI.ErrDispatchRefusedAG4):
		// The honest production refusal of the drain (AG-4): no real
		// providers exist — the queue honestly stays pending; the honest
		// words the app attached at the wrap point ARE the message.
		reason := operationsWrappedReason(err, actionAPI.ErrDispatchRefusedAG4)
		if reason == "" {
			reason = "Исполнение не подключено — реальных провайдеров нет: очередь не тронута, отправок ноль (AG-4)."
		}
		s.problem(w, r, http.StatusForbidden, "mutation_execution_not_wired", reason, false, nil)
	case errors.As(err, &rejected):
		// The rejection ladder BY NAME (AC-1): the code carries the exact
		// state word, the message is the fixed domain text of the word.
		s.problem(w, r, http.StatusConflict, "confirmation_"+rejected.State, actionDomain.ConfirmationStateText(rejected.State), false, nil)
	case errors.As(err, &stale):
		s.problem(w, r, http.StatusConflict, "authorization_stale", stale.Reason, false, nil)
	case errors.Is(err, actionPort.ErrIdempotencyConflict):
		reason := operationsWrappedReason(err, actionPort.ErrIdempotencyConflict)
		if reason == "" {
			reason = actionDomain.IdempotencyConflictText
		}
		s.problem(w, r, http.StatusConflict, "idempotency_conflict", reason, false, nil)
	case errors.Is(err, actionPort.ErrPreviewDigestChanged):
		reason := operationsWrappedReason(err, actionPort.ErrPreviewDigestChanged)
		if reason == "" {
			reason = actionDomain.ConfirmationPreviewChangedText
		}
		s.problem(w, r, http.StatusConflict, "confirmation_preview_changed", reason, false, nil)
	case errors.Is(err, actionPort.ErrPreviewBlockedMaterial):
		reason := operationsWrappedReason(err, actionPort.ErrPreviewBlockedMaterial)
		if reason == "" {
			reason = "Материал preview заблокирован — авторизация невозможна."
		}
		s.problem(w, r, http.StatusConflict, "preview_blocked_material", reason, false, nil)
	case errors.Is(err, actionPort.ErrMaterialConditionChanged):
		reason := operationsWrappedReason(err, actionPort.ErrMaterialConditionChanged)
		if reason == "" {
			reason = actionDomain.MaterialConditionChangedText
		}
		s.problem(w, r, http.StatusConflict, "material_condition_changed", reason, false, nil)
	case errors.Is(err, actionPort.ErrStaleConfirmationVersion):
		s.problem(w, r, http.StatusConflict, "stale_confirmation_version", "Лестница версий Confirmation двинулась параллельным прогоном: перечитайте страницу и повторите команду.", false, nil)
	case errors.Is(err, actionPort.ErrFreshReadsUnavailable):
		// The fail-closed refusal of the nil OwningSystemReader (production,
		// AG-4): the authorization honestly cannot happen.
		reason := operationsWrappedReason(err, actionPort.ErrFreshReadsUnavailable)
		if reason == "" {
			reason = actionDomain.FreshReadsUnavailableText
		}
		s.problem(w, r, http.StatusForbidden, "fresh_reads_unavailable", reason, false, nil)
	case errors.Is(err, actionPort.ErrOperationIntegrity):
		s.logger.ErrorContext(r.Context(), "operation integrity alarm", "code", "operation_integrity_alarm", "error", err.Error())
		s.problem(w, r, http.StatusInternalServerError, "operation_integrity_alarm", "Структурная целостность операции нарушена — передайте в проверку целостности.", false, nil)
	case errors.Is(err, actionDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		s.problem(w, r, http.StatusNotFound, "not_found", "Обращение, версия preview, Confirmation или Operation не найдены.", false, nil)
	case errors.Is(err, actionDomain.ErrForbidden), errors.Is(err, domain.ErrForbidden):
		// The module's own 403 sentinel carries the honest words the app
		// attached at the wrap point (the scope words of the rights gate or
		// the exact policy reason word); the transport never invents its own
		// copy; the identity sentinel keeps the legacy branch of the
		// transport-level fakes.
		message := err.Error()
		for _, prefix := range []string{actionDomain.ErrForbidden.Error() + ": ", domain.ErrForbidden.Error() + ": "} {
			if strings.HasPrefix(message, prefix) {
				message = strings.TrimPrefix(message, prefix)
				break
			}
		}
		s.problem(w, r, http.StatusForbidden, "operations_forbidden", message, false, nil)
	case errors.Is(err, domain.ErrStepUpRequired):
		s.problem(w, r, http.StatusForbidden, "step_up_required", "Для подтверждения или отзыва требуется повторное подтверждение личности.", false, nil)
	default:
		s.logger.ErrorContext(r.Context(), "operation command failed", "code", "operation_command_failed", "error", err.Error())
		s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить команду контура. Повторите позднее.", true, nil)
	}
}

// operationsPageProblem maps the contour errors of the PAGE read: the
// module's own 404 (a non-member or a foreign tenant never learn the
// case authorizes actions) and the module's own 403 (the scope words of
// the honest refusal) — the unmapped failures answer the honest 500.
func (s *Server) operationsPageProblem(w http.ResponseWriter, r *http.Request, actor domain.Principal, err error) {
	var rule actionDomain.RuleError
	switch {
	case errors.As(err, &rule):
		s.renderStatus(w, http.StatusForbidden, "cases", map[string]any{
			"View": casesPageView{ScopeText: casesScopeText(false), ForbiddenText: rule.Reason},
		})
	case errors.Is(err, actionDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		s.notFound(w, r, actor)
	case errors.Is(err, actionDomain.ErrForbidden), errors.Is(err, domain.ErrForbidden):
		message := operationsFormMessage(err)
		s.renderStatus(w, http.StatusForbidden, "cases", map[string]any{
			"View": casesPageView{ScopeText: casesScopeText(false), ForbiddenText: message},
		})
	default:
		s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось открыть Operation Review. Повторите позднее.", true, nil)
	}
}

// operationsFormMessage renders the honest flash text of the page form
// errors (the 3.3/3.4 precedent: the rule words, the ladder words, the
// honest 404/403 texts, the step-up confirmation).
func operationsFormMessage(err error) string {
	var rule actionDomain.RuleError
	var rejected actionPort.ConfirmationRejectedError
	var stale actionPort.AuthorizationStaleError
	switch {
	case errors.As(err, &rule):
		return rule.Reason
	case errors.As(err, &rejected):
		return actionDomain.ConfirmationStateText(rejected.State)
	case errors.As(err, &stale):
		return "Материал устарел: " + stale.Reason + " Требуется новая версия Preview и Confirmation."
	case errors.Is(err, actionPort.ErrIdempotencyConflict):
		return actionDomain.IdempotencyConflictText
	case errors.Is(err, actionPort.ErrPreviewDigestChanged):
		return actionDomain.ConfirmationPreviewChangedText
	case errors.Is(err, actionPort.ErrPreviewBlockedMaterial):
		return "Материал preview заблокирован — авторизация невозможна; требуется новая версия Preview."
	case errors.Is(err, actionPort.ErrMaterialConditionChanged):
		return actionDomain.MaterialConditionChangedText
	case errors.Is(err, actionPort.ErrStaleConfirmationVersion):
		return "Лестница версий Confirmation двинулась параллельным прогоном: перечитайте страницу и повторите команду."
	case errors.Is(err, actionPort.ErrFreshReadsUnavailable):
		return actionDomain.FreshReadsUnavailableText
	case errors.Is(err, actionPort.ErrOperationIntegrity):
		return "Структурная целостность операции нарушена — передайте в проверку целостности."
	case errors.Is(err, actionDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		return "Обращение, версия preview, Confirmation или Operation не найдены: перечитайте страницу и повторите."
	case errors.Is(err, actionDomain.ErrForbidden), errors.Is(err, domain.ErrForbidden):
		return "Подтверждение и авторизация действия требуют полномочия review или configuration."
	case errors.Is(err, actionAPI.ErrDispatchRefusedAG4):
		return operationsFormMessageAG4(err)
	case errors.Is(err, domain.ErrStepUpRequired):
		return "Для подтверждения или отзыва требуется повторное подтверждение личности. Включите подтверждение личности и повторите."
	}
	return "Команда контура не выполнена. Повторите позднее."
}

// operationsFormMessageAG4 renders the honest flash text of the drain
// refusal at the nil mutation adapter (AG-4): the words the app attached
// at the wrap point ARE the flash; the queue stays pending, the operator
// reads why.
func operationsFormMessageAG4(err error) string {
	if reason := operationsWrappedReason(err, actionAPI.ErrDispatchRefusedAG4); reason != "" {
		return reason
	}
	return "Исполнение не подключено — реальных провайдеров нет: очередь не тронута, отправок ноль (AG-4)."
}

// operationsReviewTarget answers the PRG target of the page forms — the
// Operation Review page of the case with the one success flag (the
// notice speaks the words, the focus moves onto it).
func operationsReviewTarget(tenant, caseID id.UUID, flag string, value string) string {
	return "/app/tenants/" + tenant.String() + "/cases/" + caseID.String() + "/operations?" + flag + "=" + value
}

// confirmationPinOfPath parses the confirmation path pin: the stable
// consent identity and the EXACT version number (the anchors of the
// revocation and the authorization commands).
func confirmationPinOfPath(r *http.Request) (id.UUID, int64, error) {
	confirmationID, err := id.Parse(r.PathValue("confirmation_id"))
	if err != nil {
		return id.UUID(""), 0, err
	}
	versionNo, err := strconv.ParseInt(r.PathValue("version_no"), 10, 64)
	if err != nil || versionNo < 1 {
		return id.UUID(""), 0, fmt.Errorf("версия — целое число начиная с 1")
	}
	return confirmationID, versionNo, nil
}

// ---------------------------------------------------------------------------
// The machine JSON contract
// ---------------------------------------------------------------------------

// confirmPreviewInput is the strict machine shape of the consent
// request: the requested lifetime clamps into the bounded server window
// (the domain's server truth); the preview pin travels in the path; NO
// free field exists (a «confidence» field is the honest 400 — the
// consent is the human act over the pinned material, never a model
// belief).
type confirmPreviewInput struct {
	LifetimeSeconds int `json:"lifetime_seconds"`
}

// revokeConfirmationInput is the strict machine shape of the revocation
// request: the body carries NOTHING (the pin travels in the path; the
// live step-up travels in the session).
type revokeConfirmationInput struct{}

// authorizeOperationInput is the strict machine shape of the machine
// authorization request: the CLIENT idempotency word ONLY (the
// double-click, the webhook repeat and the replay job carry the SAME
// key again — FR-29; the charset/length bounds are the domain gate's
// honest words BEFORE everything; a free «confidence»/«assurance» field
// is the honest 400 — the model's confidence never substitutes a
// Confirmation, AG-4/AC-10).
type authorizeOperationInput struct {
	IdempotencyKey string `json:"idempotency_key"`
}

// authorizeOutcomeStatus maps the authorization outcome onto the honest
// status: the replay of the same client key and the cross-case
// reservation reference answer 200 (the idempotent return and the honest
// loser reference — AC-7/AC-9); the FIRST authorization answers 201
// (the immutable Operation exists now — the consumption, the ledger and
// the outbox record landed in ONE transaction).
func authorizeOutcomeStatus(outcome actionAPI.AuthorizationOutcome) int {
	if outcome.Replay || outcome.ReservationReference {
		return http.StatusOK
	}
	return http.StatusCreated
}

// readAuthorizationViewAPI answers the machine read of the Operation
// Review surface: the confirmation version cards with the live
// read-time derivations (the state word, the consumption words, the
// visible conflict/blocked exceptions) and the operation cards with the
// ledger and the queue fact (the honest empty view of a case without
// consents — never a null).
func (s *Server) readAuthorizationViewAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.authorization == nil {
		s.notFound(w, r, actor)
		return
	}
	view, err := s.authorization.ReadAuthorizationView(r.Context(), actor, tenant, caseID, s.clock.Now().UTC())
	if err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authorizations": view})
}

// readConfirmationThreadAPI answers the machine read of ONE consent
// thread: every confirmation version card of the stable identity (the
// append-only ladder — the newest version is the thread head).
func (s *Server) readConfirmationThreadAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.authorization == nil {
		s.notFound(w, r, actor)
		return
	}
	confirmationID, err := id.Parse(r.PathValue("confirmation_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	view, err := s.authorization.ReadAuthorizationView(r.Context(), actor, tenant, caseID, s.clock.Now().UTC())
	if err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	versions := []actionAPI.ConfirmationCardView{}
	for _, card := range view.Confirmations {
		if card.ConfirmationID == confirmationID {
			versions = append(versions, card)
		}
	}
	if len(versions) == 0 {
		s.notFound(w, r, actor)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"confirmations": versions})
}

// readOperationAPI answers the machine read of ONE immutable operation:
// the full card with the pins, the two key identities, the derived
// state word (authorized/queued — executed NEVER exists here, AC-11),
// the append-only ledger and the fixed honest texts of the slice
// boundaries.
func (s *Server) readOperationAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.authorization == nil {
		s.notFound(w, r, actor)
		return
	}
	operationID, err := id.Parse(r.PathValue("operation_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	view, err := s.authorization.ReadAuthorizationView(r.Context(), actor, tenant, caseID, s.clock.Now().UTC())
	if err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	for _, card := range view.Operations {
		if card.OperationID == operationID {
			writeJSON(w, http.StatusOK, map[string]any{"operation": card})
			return
		}
	}
	s.notFound(w, r, actor)
}

// confirmPreviewVersionAPI answers the machine consent request of one
// EXACT preview version: the strict decode, the LIVE step-up gate and
// the frozen chain in the service; the honest 201 speaks the new version
// with the server-truth valid_until.
func (s *Server) confirmPreviewVersionAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.authorization == nil {
		s.notFound(w, r, actor)
		return
	}
	previewID, versionNo, err := previewVersionPinOfPath(r)
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	var input confirmPreviewInput
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	outcome, err := s.authorization.ConfirmPreviewVersion(r.Context(), actionAPI.ConfirmPreviewVersionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		PreviewID: previewID, PreviewVersionNo: versionNo,
		LifetimeSeconds: input.LifetimeSeconds,
		Now:             s.clock.Now().UTC(),
	})
	if err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"confirmation": outcome})
}

// revokeConfirmationAPI answers the machine revocation of one exact
// confirmation version: the LIVE step-up gate and the one event append
// in the service; the honest 200 speaks the revoked state words (the
// past consumption stays the audit fact).
func (s *Server) revokeConfirmationAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.authorization == nil {
		s.notFound(w, r, actor)
		return
	}
	confirmationID, versionNo, err := confirmationPinOfPath(r)
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	var input revokeConfirmationInput
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	outcome, err := s.authorization.RevokeConfirmation(r.Context(), actionAPI.RevokeConfirmationCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		ConfirmationID: confirmationID, VersionNo: versionNo,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revocation": outcome})
}

// authorizeOperationAPI answers the machine authorization request: the
// strict decode (the idempotency word only — a free «confidence» field
// refuses), the ordered gateway with the replay check FIRST in the
// service, and the honest status ladder (201 first / 200 replay and
// reference / 409 the exact conflict and ladder words).
func (s *Server) authorizeOperationAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.authorization == nil {
		s.notFound(w, r, actor)
		return
	}
	confirmationID, versionNo, err := confirmationPinOfPath(r)
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	var input authorizeOperationInput
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	outcome, err := s.authorization.AuthorizeOperation(r.Context(), actionAPI.AuthorizeOperationCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		ConfirmationID: confirmationID, ConfirmationVersionNo: versionNo,
		IdempotencyKey: input.IdempotencyKey,
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	writeJSON(w, authorizeOutcomeStatus(outcome), map[string]any{"authorization": outcome})
}

// ---------------------------------------------------------------------------
// The Operation Review page
// ---------------------------------------------------------------------------

// confirmationEventRowView is one append-only consent history record of
// the page card (the visible conflict/blocked exceptions of the rejected
// authorization attempts speak their words — AC-8/AC-1).
type confirmationEventRowView struct {
	VersionNo string
	EventNo   string
	Kind      string
	Reason    string
	Details   string
	At        string
}

// confirmationCardRowView is the page projection of one confirmation
// version card: every state spoken in TEXT (never by the colour alone),
// the server-truth valid_until, the consumption words and the visible
// event history.
type confirmationCardRowView struct {
	ConfirmationID   string
	No               string
	DefinitionText   string
	TargetText       string
	OwnerText        string
	PreviewPinText   string
	State            string
	StateText        string
	ConsumptionText  string
	ValidUntilText   string
	Events           []confirmationEventRowView
	HasEvents        bool
	CreatedAt        string
	RevocationFormID string
}

// operationLedgerRowView is one append-only ledger record of the page
// card (AD-13 — the authorized entry of this slice).
type operationLedgerRowView struct {
	No      string
	Kind    string
	Details string
	At      string
}

// dispatchAttemptRowView is one progressive attempt of the page card
// (AC-13: the attempts render in CLOSED details — the card stays legible
// at 320px without scrolling through the audit): the write-ahead lease
// facts, the keyed digest trace, the honest claim word with its text,
// the correlation and the recorded guarantee word.
type dispatchAttemptRowView struct {
	No              string
	LeaseAt         string
	LeaseExpires    string
	RequestDigest   string
	Outcome         string
	OutcomeText     string
	CorrelationText string
	GuaranteeText   string
	OutcomeAt       string
}

// actionReceiptRowView is one Action Receipt version of the page card
// (FR-31: the identifier readback — the decision reconstructs WITHOUT
// the dialog; the sensitive material never enters).
type actionReceiptRowView struct {
	VersionNo           string
	Outcome             string
	OutcomeText         string
	CaseID              string
	WorkItemID          string
	PreviewPinText      string
	ConfirmationPinText string
	DefinitionText      string
	ExternalObjectText  string
	Details             string
	CreatedAt           string
}

// operationCardRowView is the page projection of one immutable operation
// card: the pins, the two key identities, the derived life state word of
// the six-word dictionary with the queue fact, the state moment, the
// Owning System, the next step, the visible block cause, the progressive
// attempts, the Action Receipt versions and the fixed honest texts of
// the slice boundaries.
type operationCardRowView struct {
	OperationID         string
	DefinitionText      string
	PreviewPinText      string
	ConfirmationPinText string
	TargetText          string
	OwnerText           string
	PartitionText       string
	IdempotencyKey      string
	BusinessIntentKey   string
	MaterialDigest      string
	State               string
	StateText           string
	QueuedText          string
	StateAtText         string
	OwningSystemText    string
	NextStepText        string
	BlockReasonText     string
	ExternalIDText      string
	DispatchText        string
	Ledger              []operationLedgerRowView
	HasLedger           bool
	Attempts            []dispatchAttemptRowView
	HasAttempts         bool
	Receipts            []actionReceiptRowView
	HasReceipts         bool
	CreatedAt           string
}

// operationsPageView carries the page material: the tenant and the case,
// the confirmation and operation cards, the honest empty state, the
// confirm form with the prefill pin of the 3-4 primary transition, the
// operator drain form of the execution contour (3-6 step-03 — shown
// when the dispatch surface is bound), the flash error and the PRG
// notice.
type operationsPageView struct {
	TenantID         string
	CaseID           string
	HasConfirmations bool
	Confirmations    []confirmationCardRowView
	HasOperations    bool
	Operations       []operationCardRowView
	EmptyText        string
	Flash            string
	Notice           string
	ConfirmPreview   string
	ConfirmVersion   string
	ConfirmLifetime  string
	HasDrain         bool
}

// operationNotice speaks the PRG status of a successful form mutation:
// the flag carries the identity of the landed step (the version number,
// the operation id), the words name the discipline of the step (the
// focus moves onto the notice; the critical identifiers read back).
func operationNotice(r *http.Request) string {
	if raw := r.URL.Query().Get("confirmed"); raw != "" {
		return "Confirmation версии " + raw + " построена: согласие запинило точную версию preview (digest-копия байт-в-байт, snapshot и эффекты связаны), окно valid_until назначено серверной истиной. Фокус переведён на этот статус."
	}
	if raw := r.URL.Query().Get("revoked"); raw != "" {
		return "Confirmation версии " + raw + " отозвана: будущие авторизации невозможны; прошлое потребление остаётся фактом аудита. Фокус переведён на этот статус."
	}
	if raw := r.URL.Query().Get("authorized"); raw != "" {
		return "Operation " + raw + " авторизована и в очереди на будущий dispatch (operation.queued): потребление Confirmation, резервация, операция, запись ledger и outbox прошли одной транзакцией; внешнего исполнения в этом срезе не существует. Фокус переведён на этот статус."
	}
	if raw := r.URL.Query().Get("replayed"); raw != "" {
		return "Повтор авторизации: возвращена первоначальная Operation " + raw + " и её состояние — Confirmation повторно не потребляется, вторая outbox запись не создаётся. Фокус переведён на этот статус."
	}
	if raw := r.URL.Query().Get("referenced"); raw != "" {
		return "Эквивалентное бизнес-намерение уже зарезервировано: возвращена ссылка на существующую Operation " + raw + "; потребление Confirmation проигравшего не произошло. Фокус переведён на этот статус."
	}
	if raw := r.URL.Query().Get("drained"); raw != "" {
		return "Очередь отправок дренирована: рассмотрено заданий — " + raw + " (ограничение батча — доменный предел); каждое рассмотренное задание говорило своей строкой отчёта — отправленные, припаркованные с точной причиной и честные пропуски. Фокус переведён на этот статус."
	}
	return ""
}

// newConfirmationCardRowView maps one api card onto the page projection:
// the state word with its fixed text, the server-truth valid_until, the
// consumption words and the visible event history.
func newConfirmationCardRowView(card actionAPI.ConfirmationCardView) confirmationCardRowView {
	row := confirmationCardRowView{
		ConfirmationID:  card.ConfirmationID.String(),
		No:              strconv.FormatInt(card.VersionNo, 10),
		DefinitionText:  card.DefinitionID + "@" + card.DefinitionVersion,
		TargetText:      card.TargetKind + " " + card.TargetRef.String(),
		OwnerText:       card.OwnerKind + " " + card.OwnerRef.String(),
		PreviewPinText:  card.PreviewID.String() + ":" + strconv.FormatInt(card.PreviewVersionNo, 10),
		State:           card.State,
		StateText:       card.StateText,
		ConsumptionText: card.ConsumptionText,
		Events:          []confirmationEventRowView{},
	}
	if !card.ValidUntil.IsZero() {
		row.ValidUntilText = card.ValidUntil.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if !card.CreatedAt.IsZero() {
		row.CreatedAt = card.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	row.HasEvents = len(card.Events) > 0
	for _, event := range card.Events {
		row.Events = append(row.Events, confirmationEventRowView{
			VersionNo: strconv.FormatInt(event.VersionNo, 10),
			EventNo:   strconv.FormatInt(event.EventNo, 10),
			Kind:      event.Kind,
			Reason:    event.Reason,
			Details:   event.Details,
			At:        event.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	row.RevocationFormID = "revoke-" + card.ConfirmationID.String() + "-" + row.No
	return row
}

// newOperationCardRowView maps one api card onto the page projection:
// the derived life state word of the six-word dictionary with the queue
// fact, the state moment, the Owning System, the next step, the visible
// block cause, the pins, the two key identities, the progressive
// attempts, the Action Receipt versions and the honest boundary texts.
func newOperationCardRowView(card actionAPI.OperationCardView) operationCardRowView {
	row := operationCardRowView{
		OperationID:         card.OperationID.String(),
		DefinitionText:      card.DefinitionID + "@" + card.DefinitionVersion,
		PreviewPinText:      card.PreviewID.String() + ":" + strconv.FormatInt(card.PreviewVersionNo, 10),
		ConfirmationPinText: card.ConfirmationID.String() + ":" + strconv.FormatInt(card.ConfirmationVersionNo, 10),
		TargetText:          card.TargetKind + " " + card.TargetRef.String(),
		OwnerText:           card.OwnerKind + " " + card.OwnerRef.String() + " (эпоха " + strconv.FormatInt(card.OwnerEpoch, 10) + ")",
		PartitionText:       actionDomain.ExecutionPartitionText(card.Partition),
		IdempotencyKey:      card.IdempotencyKey,
		BusinessIntentKey:   card.BusinessIntentKey,
		MaterialDigest:      card.MaterialDigest,
		State:               card.State,
		StateText:           card.StateText,
		QueuedText:          operationQueuedWords(card.Queued),
		OwningSystemText:    operationOwningSystemWords(card.OwningSystem),
		NextStepText:        card.NextStepText,
		BlockReasonText:     card.BlockReasonText,
		ExternalIDText:      card.ExternalIDAbsentText,
		DispatchText:        card.DispatchAbsentText,
		Ledger:              []operationLedgerRowView{},
		Attempts:            []dispatchAttemptRowView{},
		Receipts:            []actionReceiptRowView{},
	}
	if !card.StateAt.IsZero() {
		row.StateAtText = card.StateAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if !card.CreatedAt.IsZero() {
		row.CreatedAt = card.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	row.HasLedger = len(card.Ledger) > 0
	for _, entry := range card.Ledger {
		row.Ledger = append(row.Ledger, operationLedgerRowView{
			No:      strconv.FormatInt(entry.EntryNo, 10),
			Kind:    entry.Kind,
			Details: entry.Details,
			At:      entry.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	row.HasAttempts = len(card.Attempts) > 0
	for _, attempt := range card.Attempts {
		row.Attempts = append(row.Attempts, dispatchAttemptRowView{
			No:              strconv.Itoa(attempt.AttemptNo),
			LeaseAt:         attempt.LeaseCreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			LeaseExpires:    attempt.LeaseExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			RequestDigest:   attempt.RequestDigest,
			Outcome:         attempt.Outcome,
			OutcomeText:     attempt.OutcomeText,
			CorrelationText: attemptCorrelationWords(attempt.CorrelationID),
			GuaranteeText:   attemptGuaranteeWords(attempt.GuaranteeRecorded),
			OutcomeAt:       attempt.OutcomeCreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	row.HasReceipts = len(card.Receipts) > 0
	for _, receipt := range card.Receipts {
		row.Receipts = append(row.Receipts, actionReceiptRowView{
			VersionNo:           strconv.FormatInt(receipt.ReceiptVersionNo, 10),
			Outcome:             receipt.Outcome,
			OutcomeText:         receipt.OutcomeText,
			CaseID:              receipt.CaseID.String(),
			WorkItemID:          receipt.WorkItemID.String(),
			PreviewPinText:      receipt.PreviewID.String() + ":" + strconv.FormatInt(receipt.PreviewVersionNo, 10),
			ConfirmationPinText: receipt.ConfirmationID.String() + ":" + strconv.FormatInt(receipt.ConfirmationVersionNo, 10),
			DefinitionText:      receipt.DefinitionID + "@" + receipt.DefinitionVersion,
			ExternalObjectText:  receiptExternalObjectWords(receipt.ExternalObjectID, receipt.ExternalObjectVersion),
			Details:             receipt.Details,
			CreatedAt:           receipt.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return row
}

// operationQueuedWords speaks the queue fact of the card honestly: the
// outbox record presence is a FACT, never a stored state.
func operationQueuedWords(queued bool) string {
	if queued {
		return "В очереди на dispatch: да — запись operation.queued существует в outbox (факт, не хранимое состояние)."
	}
	return "В очереди на dispatch: нет — записи operation.queued в outbox не существует."
}

// operationOwningSystemWords speaks the Owning System word of the
// pinned definition (the unresolvable pin keeps the honest refusal).
func operationOwningSystemWords(word string) string {
	if word != "" {
		return "Система-владелец исполнения (Owning System): " + word + " — слово словаря 2-7 пиннутого определения."
	}
	return "Система-владелец исполнения (Owning System): определение не резолвится против каталога — слово отсутствует."
}

// attemptCorrelationWords speaks the provider correlation of one attempt
// (empty — the provider did not issue one).
func attemptCorrelationWords(correlation string) string {
	if correlation != "" {
		return "Корреляция провайдера: " + correlation
	}
	return "Корреляция провайдера: не выдана."
}

// attemptGuaranteeWords speaks the recorded guarantee word (the retry
// policy input — the audit fact, never a live re-read).
func attemptGuaranteeWords(recorded bool) string {
	if recorded {
		return "Гарантия провайдера записана: повтор разрешён политикой в пределах 3."
	}
	return "Гарантия провайдера не записана: слепой повтор запрещён."
}

// receiptExternalObjectWords speaks the external object identity readback
// (the structural evidence of a terminal success — only committed
// carries the complete triple).
func receiptExternalObjectWords(objectID, objectVersion string) string {
	if objectID != "" {
		return "Внешний объект: " + objectID + " (версия " + objectVersion + ") — структурное evidence терминального успеха."
	}
	return "Внешний объект не выдан: evidence появляется записью dispatch-исхода — слово не выдумывается."
}

// splitPreviewConfirmPin cuts the ?confirm= prefill value of the 3-4
// primary transition into the preview pin pair (preview_id@version_no —
// the splitDefinitionPin precedent).
func splitPreviewConfirmPin(pin string) (string, string) {
	idPart, version, found := strings.Cut(pin, "@")
	if !found {
		return idPart, ""
	}
	return idPart, version
}

// operationReviewPage renders the Operation Review page of the case:
// the confirmation cards with the live derivations and the visible
// event history, the operation cards with the ledger and the queue
// fact, the confirm form (prefilled by the 3-4 primary transition pin),
// the per-card revocation and authorization forms.
func (s *Server) operationReviewPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.authorization == nil {
		s.notFound(w, r, actor)
		return
	}
	view, err := s.authorization.ReadAuthorizationView(r.Context(), actor, tenant, caseID, s.clock.Now().UTC())
	if err != nil {
		s.operationsPageProblem(w, r, actor, err)
		return
	}
	flash := flashError(r, tenant)
	if flash != "" {
		http.SetCookie(w, clearFlashCookie(tenant))
	}
	page := operationsPageView{
		TenantID: tenant.String(), CaseID: caseID.String(),
		EmptyText: view.EmptyText, Flash: flash,
		Notice:          operationNotice(r),
		Confirmations:   []confirmationCardRowView{},
		Operations:      []operationCardRowView{},
		ConfirmLifetime: "3600",
		HasDrain:        s.dispatch != nil,
	}
	if raw := r.URL.Query().Get("confirm"); raw != "" {
		page.ConfirmPreview, page.ConfirmVersion = splitPreviewConfirmPin(raw)
	}
	for _, card := range view.Confirmations {
		page.HasConfirmations = true
		page.Confirmations = append(page.Confirmations, newConfirmationCardRowView(card))
	}
	for _, card := range view.Operations {
		page.HasOperations = true
		page.Operations = append(page.Operations, newOperationCardRowView(card))
	}
	s.render(w, "operations", page)
}

// confirmPreviewVersionForm answers the page consent form: the origin
// gate, the strict parse, the pinned version and the requested lifetime
// (the server clamps — the promised_at precedent); the honest refusal
// words land as the flash, the success lands PRG with the new version
// number. The LIVE step-up is the session's flag — the refusal words say
// it honestly.
func (s *Server) confirmPreviewVersionForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.authorization == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.operationsProblem(w, r, fmt.Errorf("%w: %s", domain.ErrForbidden, "форма подтверждения доступна только из браузера приложения (origin)."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы.")
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	previewID, err := id.Parse(strings.TrimSpace(r.FormValue("preview_id")))
	if err != nil {
		setFlashError(w, tenant, "Preview — действительный UUID: перейдите по primary-кнопке карточки на странице Action Preview либо введите пин вручную.")
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	versionNo, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("version_no")), 10, 64)
	if err != nil || versionNo < 1 {
		setFlashError(w, tenant, "Версия preview — целое число начиная с 1.")
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	lifetime, err := strconv.Atoi(strings.TrimSpace(r.FormValue("lifetime_seconds")))
	if err != nil || lifetime < 0 {
		setFlashError(w, tenant, "Запрошенное окно — положительное целое число секунд (сервер ограничивает в границы 60..3600).")
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	outcome, err := s.authorization.ConfirmPreviewVersion(r.Context(), actionAPI.ConfirmPreviewVersionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		PreviewID: previewID, PreviewVersionNo: versionNo,
		LifetimeSeconds: lifetime,
		Now:             s.clock.Now().UTC(),
	})
	if err != nil {
		setFlashError(w, tenant, operationsFormMessage(err))
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "confirmed", strconv.FormatInt(outcome.VersionNo, 10)), http.StatusSeeOther)
}

// revokeConfirmationForm answers the page revocation form of one exact
// confirmation version: the explicit confirmation checkbox (the
// deliberate human act — the UI discipline the machine API does not
// carry; the server-side protection is the LIVE step-up — the double
// discipline, documented honestly), then the service command.
func (s *Server) revokeConfirmationForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.authorization == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.operationsProblem(w, r, fmt.Errorf("%w: %s", domain.ErrForbidden, "форма отзыва доступна только из браузера приложения (origin)."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы.")
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	confirmationID, versionNo, err := confirmationPinOfPath(r)
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	if r.FormValue("confirm_revocation") != "yes" {
		setFlashError(w, tenant, "Отзыв требует явного подтверждения: отметьте чекбокс согласия.")
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	outcome, err := s.authorization.RevokeConfirmation(r.Context(), actionAPI.RevokeConfirmationCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		ConfirmationID: confirmationID, VersionNo: versionNo,
		Now: s.clock.Now().UTC(),
	})
	if err != nil {
		setFlashError(w, tenant, operationsFormMessage(err))
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "revoked", strconv.FormatInt(outcome.VersionNo, 10)), http.StatusSeeOther)
}

// authorizeOperationForm answers the page authorization form of one
// exact confirmation version: the machine idempotency word (the
// double-click, the webhook repeat and the replay job carry the SAME
// key again — FR-29; the page generates NO key for the operator: the
// word belongs to the calling machine, the operator pastes the one the
// machine carries), then the service command with the honest status
// ladder of the PRG flags.
func (s *Server) authorizeOperationForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if s.authorization == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.operationsProblem(w, r, fmt.Errorf("%w: %s", domain.ErrForbidden, "форма авторизации доступна только из браузера приложения (origin)."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы.")
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	confirmationID, versionNo, err := confirmationPinOfPath(r)
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	idempotencyKey := strings.TrimSpace(r.FormValue("idempotency_key"))
	if idempotencyKey == "" {
		setFlashError(w, tenant, "Idempotency key обязателен: машинное слово повтора (8..128 символов, буквы/цифры/точка/подчёркивание/дефис) — тот же ключ у double-click, повтора webhook и replay-задания.")
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	outcome, err := s.authorization.AuthorizeOperation(r.Context(), actionAPI.AuthorizeOperationCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID,
		ConfirmationID: confirmationID, ConfirmationVersionNo: versionNo,
		IdempotencyKey: idempotencyKey,
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		setFlashError(w, tenant, operationsFormMessage(err))
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	switch {
	case outcome.Replay:
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "replayed", outcome.Operation.OperationID.String()), http.StatusSeeOther)
	case outcome.ReservationReference:
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "referenced", outcome.Operation.OperationID.String()), http.StatusSeeOther)
	default:
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "authorized", outcome.Operation.OperationID.String()), http.StatusSeeOther)
	}
}

// BindAuthorization registers the confirmation→authorization surface:
// the six machine paths (the view read, the confirmation thread read,
// the operation read, the consent, the revocation, the authorization)
// and the operator Operation Review page with the PRG forms. The
// service is the narrow api.AuthorizationService (the PreviewService
// type-assert precedent — the app Service carries the compile-time
// proof).
func (s *Server) BindAuthorization(service actionAPI.AuthorizationService) {
	s.authorization = service
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}/operations", s.readAuthorizationViewAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}/confirmations/{confirmation_id}", s.readConfirmationThreadAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/cases/{case_id}/operations/{operation_id}", s.readOperationAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/action-previews/{preview_id}/versions/{version_no}/confirmations", s.confirmPreviewVersionAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/confirmations/{confirmation_id}/versions/{version_no}/revocations", s.revokeConfirmationAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/cases/{case_id}/confirmations/{confirmation_id}/versions/{version_no}/authorizations", s.authorizeOperationAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/cases/{case_id}/operations", s.operationReviewPage)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/operations/confirmations", s.confirmPreviewVersionForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/confirmations/{confirmation_id}/versions/{version_no}/revocations", s.revokeConfirmationForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/cases/{case_id}/confirmations/{confirmation_id}/versions/{version_no}/authorizations", s.authorizeOperationForm)
}

// ---------------------------------------------------------------------------
// Story 3.6 (step-03): the drain command of the execution contour
// ---------------------------------------------------------------------------

// drainOperationsInput is the strict machine shape of the drain request:
// the body carries NOTHING (the drain is the bounded operator command —
// NO parameters exist; a free field is the honest 400, the strict decode
// refuses it BEFORE the service is ever called).
type drainOperationsInput struct{}

// drainOperationsAPI answers the machine drain of the queued operations:
// the strict decode of the EMPTY body, the operator manage gate and the
// bounded honest report (201 — the report of the run exists now; every
// considered job leaves its spoken line, FR-28). The nil mutation
// adapter of the production root refuses honestly FIRST (AG-4 — the 403
// words, the queue stays pending, zero sends; the
// DrainExternalCommitments precedent of 3-3).
func (s *Server) drainOperationsAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	if s.dispatch == nil {
		s.notFound(w, r, actor)
		return
	}
	var input drainOperationsInput
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	result, err := s.dispatch.DrainDispatchQueue(r.Context(), actionAPI.DrainDispatchCommand{
		Actor: actor, TenantID: tenant, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"drain": result})
}

// drainOperationsForm answers the page drain form of the Operation
// Review surface (the external-commitments precedent of 3-3): the
// explicit confirmation checkbox (the deliberate operator act), the
// hidden case pin of the PRG return, the honest report landing PRG with
// the considered counter (the notice speaks the words, the focus moves
// onto it); the AG-4 refusal words land as the flash (the production
// root wires no adapter — the queue stays pending, the state stays
// honest).
func (s *Server) drainOperationsForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	if s.dispatch == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.operationsProblem(w, r, fmt.Errorf("%w: %s", domain.ErrForbidden, "форма диспетчеризации доступна только из браузера приложения (origin)."))
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
		setFlashError(w, tenant, "Не удалось вернуться на страницу операций: страница кейса не найдена.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases", http.StatusSeeOther)
		return
	}
	if r.FormValue("confirm_drain") != "yes" {
		setFlashError(w, tenant, "Диспетчеризация требует явного подтверждения: отметьте чекбокс согласия.")
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	result, err := s.dispatch.DrainDispatchQueue(r.Context(), actionAPI.DrainDispatchCommand{
		Actor: actor, TenantID: tenant, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		setFlashError(w, tenant, operationsFormMessage(err))
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "drained", strconv.Itoa(result.Considered)), http.StatusSeeOther)
}

// BindDispatch registers the execution-contour surface of story 3.6: the
// machine drain path of the tenant queue and the page drain form (the
// hidden case pin carries the PRG return). The service is the narrow
// api.DispatchService (the BindAuthorization type-assert precedent — the
// app Service carries the compile-time proof); the production root binds
// the REAL service with the nil mutation adapter (AG-4 — the drain
// refuses honestly in words, the queue stays pending).
func (s *Server) BindDispatch(service actionAPI.DispatchService) {
	s.dispatch = service
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/operations/drain", s.drainOperationsAPI)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/operations/drain", s.drainOperationsForm)
}
