package httptransport

import (
	"errors"
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
// The Story 2.14 transport: the machine JSON contracts of the decision
// context read and the AI summary record (the producer path — the PRG
// forms do NOT exist: the operator never writes the summary, the kind
// dictionary carries the single producer word) and the «Контекст
// передачи» section of the case page. The dictionary refusals (the kind
// word, the rune bounds, the claim discipline with the byte-exact
// spans) are the honest 400 rule words; the raced summary number and
// the idempotency key conflicts are the honest 409; a foreign tenant is
// a plain 404; the audit-only member is the 403 with the scope words
// BEFORE every material guard (the M1 precedent of 2.5..2.13). The
// staleness badge and the summary marking render with aria-live, never
// with a focus move; the section update is the honest GET link of the
// #decision-context anchor (the section position survives, the exact
// scroll position does not — the no-JS boundary of the slice).
// ---------------------------------------------------------------------------

// recordCaseSummaryInput is the request of the producer record: the kind
// word of the single-word dictionary, the text (1..1000 runes) and the
// claims (1..32, every claim either linked with the ref words or
// honestly disputed; the claim text is a byte-exact span of the summary
// text). The summary number and the watermark are NEVER request fields
// — the server truth under the locks.
type recordCaseSummaryInput struct {
	Kind   *string             `json:"kind"`
	Text   *string             `json:"text"`
	Claims []summaryClaimInput `json:"claims"`
}

// summaryClaimInput is one claim of the producer request: the ref-type
// word with the ref id, the disputed mark and the claim text.
type summaryClaimInput struct {
	RefType  *string `json:"ref_type"`
	RefID    *string `json:"ref_id"`
	Disputed *bool   `json:"disputed"`
	Text     *string `json:"text"`
}

// decisionContextPanelView is the flattened projection of the «Контекст
// передачи» section: every block pre-rendered as text — the honest words
// and texts of the dictionaries, the formatted times, the joined term
// lines. The section renders ONLY when the material exists (the open
// handoff or the recorded summary); the honest absence renders nothing.
type decisionContextPanelView struct {
	Present         bool
	PanelText       string
	Handoff         decisionContextHandoffPanel
	HasWorkItems    bool
	WorkItems       []decisionContextWorkItemRow
	Mode            decisionContextModePanel
	Assurance       decisionContextAssurancePanel
	Owner           decisionContextOwnerPanel
	Facts           decisionContextFactsPanel
	HasObservations bool
	Observations    []decisionContextObservationRow
	HasDeliveries   bool
	Deliveries      []decisionContextDeliveryRow
	HasCommitments  bool
	Commitments     []decisionContextCommitmentRow
	Preferences     decisionContextPreferencesPanel
	Recommendation  decisionContextRecommendationPanel
	Summary         decisionContextSummaryPanel
}

type decisionContextHandoffPanel struct {
	Present         bool
	ReasonText      string
	State           string
	StateText       string
	TurnRef         string
	AckDeadlineText string
	DeadlineText    string
}

type decisionContextWorkItemRow struct {
	ID    string
	Title string
	Type  string
	State string
}

type decisionContextModePanel struct {
	Present bool
	Mode    string
	Version string
}

type decisionContextAssurancePanel struct {
	IdentityLine   string
	AuthorityLine  string
	AuthorityScope string
}

type decisionContextOwnerPanel struct {
	Present bool
	Kind    string
	Ref     string
	Epoch   string
}

type decisionContextFactRow struct {
	RowID          string
	MaterialKey    string
	Status         string
	StatusText     string
	Version        string
	ProvenanceText string
	ObservedText   string
	// The Story 2.15 correction state of the row (read-time derivation —
	// the last correction of the fact speaks): the marker with the honest
	// boundary, the source line, the number, the outcome word, the time,
	// the span counter (the spans render behind the disclosure) and the
	// reason. No corrections is the zero state — the row renders
	// byte-identically (the markers appear only with the material).
	HasCorrection        bool
	MarkerText           string
	SourceText           string
	CorrectionNo         string
	CorrectionOutcome    string
	CorrectionAtText     string
	CorrectionSpansCount string
	Spans                []string
	CorrectionReason     string
}

type decisionContextFactsPanel struct {
	HasConfirmed   bool
	Confirmed      []decisionContextFactRow
	HasUnconfirmed bool
	Unconfirmed    []decisionContextFactRow
	HasConflicting bool
	Conflicting    []decisionContextFactRow
}

type decisionContextObservationRow struct {
	WorkItemID    string
	MaterialKey   string
	OwningSystem  string
	ObservedText  string
	Freshness     string
	ConflictRule  string
	Evidence      string
	TechniqueText string
}

type decisionContextDeliveryRow struct {
	ReplyID       string
	State         string
	LastKind      string
	LastEventText string
}

type decisionContextCommitmentRow struct {
	ID               string
	Version          string
	Status           string
	ValidUntilText   string
	Confirmation     string
	TermsKnownText   string
	TermsUnknownText string
	ReceiptText      string
}

type decisionContextModalityRow struct {
	Key  string
	Word string
}

type decisionContextPreferencesPanel struct {
	AccommodationsText string
	AlternativesText   string
	HasModalities      bool
	Modalities         []decisionContextModalityRow
}

type decisionContextCheckRow struct {
	Key   string
	State string
}

type decisionContextRecommendationPanel struct {
	Present         bool
	StepsText       string
	Reason          string
	Checks          []decisionContextCheckRow
	DeadlineText    string
	AckDeadlineText string
	Mode            string
	ProjectionText  string
}

type decisionContextClaimRow struct {
	RefType  string
	RefID    string
	Disputed bool
	Text     string
}

type decisionContextSummaryPanel struct {
	Present               bool
	No                    string
	Kind                  string
	MarkingText           string
	Text                  string
	Staleness             string
	StalenessCountersText string
	Stale                 bool
	StaleText             string
	HasClaims             bool
	Claims                []decisionContextClaimRow
}

// decisionContextProblem maps the read and record errors onto the
// problem contract: the dictionary and bound refusals are the
// machine-readable 400 rule words; the raced summary number and the key
// conflict are the honest 409; a foreign tenant is a plain 404; the
// audit-only member is the 403 with the scope words BEFORE every
// material guard.
func decisionContextProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrStaleVersion):
		server.problem(w, r, http.StatusConflict, "stale_version", "Материал изменился: сверьте текущее состояние и повторите команду.", false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Идемпотентный ключ уже использован другой командой: исход не воспроизводится для другого материала.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение не найдено.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		server.problem(w, r, http.StatusForbidden, "cases_forbidden", "Контекст передачи и запись сводки требуют полномочия review или configuration.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "decision context command failed", "code", "decision_context_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить команду контекста передачи. Повторите позднее.", true, nil)
	}
}

// decisionContextAPI answers the machine GET of the decision context:
// the read-time twelve-block view over ONE consistent snapshot (the
// derivations are never stored statuses; the advisory locks are not
// taken — the consistent snapshot of one transaction is the honest
// boundary of the GET).
func (s *Server) decisionContextAPI(w http.ResponseWriter, r *http.Request) {
	if s.decisionContext == nil {
		s.problem(w, r, http.StatusServiceUnavailable, "cases_unavailable", "Контур обращений не сконфигурирован.", false, nil)
		return
	}
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	view, err := s.decisionContext.DecisionContext(r.Context(), actor, tenant, caseID)
	if err != nil {
		decisionContextProblem(w, r, s, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// recordCaseSummaryAPI answers the producer POST of ONE append-only AI
// summary row: the fresh record is the honest 201, the exact repeat the
// 200 with the Idempotency-Replayed header, the dictionary refusals the
// 400 rule words, the raced conflicts the 409. The PRG form does NOT
// exist (the operator never writes the summary — the machine JSON path
// of the producer only).
func (s *Server) recordCaseSummaryAPI(w http.ResponseWriter, r *http.Request) {
	if s.decisionContext == nil {
		s.problem(w, r, http.StatusServiceUnavailable, "cases_unavailable", "Контур обращений не сконфигурирован.", false, nil)
		return
	}
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input recordCaseSummaryInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	kind := stringOrEmpty(input.Kind)
	if kind == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле kind обязательно: слово словаря вида сводки (продюсерский ai_summary — единственное слово среза).", false, nil)
		return
	}
	text := stringOrEmpty(input.Text)
	if text == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле text обязательно: текст сводки (1..1000 рун).", false, nil)
		return
	}
	if len(input.Claims) == 0 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле claims обязательно: хотя бы одно утверждение (связанное ссылкой либо помеченное спорным).", false, nil)
		return
	}
	claims := make([]casesDomain.SummaryClaim, 0, len(input.Claims))
	for _, claim := range input.Claims {
		claimText := stringOrEmpty(claim.Text)
		if claimText == "" {
			s.problem(w, r, http.StatusBadRequest, "invalid_request", "Утверждение сводки обязано нести текст (1..200 рун — байт-в-байт строка сводки).", false, nil)
			return
		}
		refIDText := stringOrEmpty(claim.RefID)
		var refID id.UUID
		if refIDText != "" {
			parsed, err := id.Parse(refIDText)
			if err != nil {
				s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле ref_id утверждения обязано быть UUID (якорь факта или наблюдения этого обращения).", false, nil)
				return
			}
			refID = parsed
		}
		disputed := claim.Disputed != nil && *claim.Disputed
		claims = append(claims, casesDomain.SummaryClaim{
			RefType: stringOrEmpty(claim.RefType), RefID: refID, Disputed: disputed, Text: claimText,
		})
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.decisionContext.RecordCaseSummary(r.Context(), casesAPI.RecordCaseSummaryCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		Kind: kind, Text: text, Claims: claims,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		decisionContextProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// decisionContextPanelOf flattens the context view onto the
// template-friendly projection of the «Контекст передачи» section: every
// row rendered as text (the words and the honest texts of the
// dictionaries, the times formatted, the terms joined) — accessible
// without color, keyboard-first. The section renders ONLY when the
// material exists (the open handoff or the recorded summary — the
// honest absence of both renders nothing, byte-for-byte the unchanged
// page).
func decisionContextPanelOf(view casesAPI.DecisionContextView) decisionContextPanelView {
	panel := decisionContextPanelView{
		Present:   view.Present,
		PanelText: view.PanelText,
		Handoff: decisionContextHandoffPanel{
			Present:         view.Handoff.Present,
			ReasonText:      view.Handoff.ReasonText,
			State:           view.Handoff.State,
			StateText:       view.Handoff.StateText,
			TurnRef:         view.Handoff.TurnRef,
			AckDeadlineText: view.Handoff.AckDeadlineText,
			DeadlineText:    formatTime(view.Handoff.DeadlineAt),
		},
		HasWorkItems: len(view.WorkItems) > 0,
		Mode: decisionContextModePanel{
			Present: view.Mode.Present, Mode: view.Mode.Mode, Version: formatCounter(view.Mode.ModeVersion),
		},
		Assurance: decisionContextAssurancePanel{
			IdentityLine:   assuranceLine(view.Assurance.IdentityLevel, view.Assurance.IdentityMethod, view.Assurance.IdentityExpiresAt),
			AuthorityLine:  assuranceLine(view.Assurance.AuthorityLevel, view.Assurance.AuthorityMethod, view.Assurance.AuthorityExpiresAt),
			AuthorityScope: view.Assurance.AuthorityScope,
		},
		Owner: decisionContextOwnerPanel{
			Present: view.Owner.Present, Kind: view.Owner.OwnerKind,
			Ref: view.Owner.OwnerRef, Epoch: formatCounter(view.Owner.Epoch),
		},
		Facts: decisionContextFactsPanel{
			HasConfirmed:   len(view.Facts.Confirmed) > 0,
			HasUnconfirmed: len(view.Facts.Unconfirmed) > 0,
			HasConflicting: len(view.Facts.Conflicting) > 0,
		},
		HasObservations: len(view.Observations) > 0,
		HasDeliveries:   len(view.Deliveries) > 0,
		HasCommitments:  len(view.Commitments) > 0,
		Preferences: decisionContextPreferencesPanel{
			AccommodationsText: joinWithSemicolon(view.Preferences.AccommodationKeys),
			AlternativesText:   joinWithSemicolon(view.Preferences.AlternativePaths),
			HasModalities:      len(view.Preferences.UsualModalities) > 0,
		},
		Recommendation: decisionContextRecommendationPanel{
			Present:         view.Recommendation.Present,
			StepsText:       joinWithSemicolon(view.Recommendation.Steps),
			Reason:          view.Recommendation.Reason,
			DeadlineText:    formatTime(view.Recommendation.DeadlineAt),
			AckDeadlineText: view.Recommendation.AckDeadlineText,
			Mode:            view.Recommendation.Mode,
			ProjectionText:  view.Recommendation.ProjectionText,
		},
		Summary: decisionContextSummaryPanel{
			Present:               view.Summary.Present,
			No:                    formatCounter(view.Summary.SummaryNo),
			Kind:                  view.Summary.Kind,
			MarkingText:           view.Summary.MarkingText,
			Text:                  view.Summary.Text,
			Staleness:             view.Summary.Staleness,
			StalenessCountersText: view.Summary.StalenessCountersText,
			Stale:                 view.Summary.Staleness == casesDomain.ContextStalenessStale,
			StaleText:             view.Summary.StalenessText,
			HasClaims:             len(view.Summary.Claims) > 0,
		},
	}
	for _, item := range view.WorkItems {
		panel.WorkItems = append(panel.WorkItems, decisionContextWorkItemRow{
			ID: item.WorkItemID, Title: item.Title, Type: item.Type, State: item.State,
		})
	}
	appendContextFacts := func(rows []casesAPI.ContextFactRow, target *[]decisionContextFactRow) {
		for _, row := range rows {
			panelRow := decisionContextFactRow{
				RowID: row.FactID, MaterialKey: row.MaterialKey, Status: row.Status, StatusText: row.StatusText,
				Version: formatCounter(row.VersionNo), ProvenanceText: factProvenanceText(row), ObservedText: formatTime(row.ObservedAt),
			}
			// The Story 2.15 correction state of the row: the read-time
			// derivation of the consistent snapshot (the zero state
			// renders the row untouched — the control branch).
			if row.HasCorrection {
				panelRow.HasCorrection = true
				panelRow.MarkerText = row.CorrectionMarkerText
				panelRow.SourceText = row.CorrectionSourceText
				panelRow.CorrectionNo = formatCounter(row.CorrectionNo)
				panelRow.CorrectionOutcome = row.CorrectionOutcome
				panelRow.CorrectionAtText = formatTime(row.CorrectionAt)
				panelRow.CorrectionSpansCount = correctionSpanCountText(row.CorrectionSpans)
				panelRow.Spans = append([]string{}, row.CorrectionSpans...)
				panelRow.CorrectionReason = row.CorrectionReason
			}
			*target = append(*target, panelRow)
		}
	}
	appendContextFacts(view.Facts.Confirmed, &panel.Facts.Confirmed)
	appendContextFacts(view.Facts.Unconfirmed, &panel.Facts.Unconfirmed)
	appendContextFacts(view.Facts.Conflicting, &panel.Facts.Conflicting)
	for _, row := range view.Observations {
		panel.Observations = append(panel.Observations, decisionContextObservationRow{
			WorkItemID: row.WorkItemID, MaterialKey: row.MaterialKey, OwningSystem: row.OwningSystem,
			ObservedText: formatTime(row.ObservedAt), Freshness: row.FreshnessWord, ConflictRule: row.ConflictRule,
			Evidence:      row.EvidenceStatus,
			TechniqueText: observationTechniqueText(row),
		})
	}
	for _, row := range view.Deliveries {
		panel.Deliveries = append(panel.Deliveries, decisionContextDeliveryRow{
			ReplyID: row.ReplyID, State: row.State, LastKind: row.LastKind, LastEventText: formatTime(row.LastEventAt),
		})
	}
	for _, row := range view.Commitments {
		panel.Commitments = append(panel.Commitments, decisionContextCommitmentRow{
			ID: row.CommitmentID, Version: formatCounter(row.VersionNo), Status: row.Status,
			ValidUntilText: formatTime(row.ValidUntil), Confirmation: row.ConfirmationStatus,
			TermsKnownText:   joinWithSemicolon(row.TermsKnown),
			TermsUnknownText: joinWithSemicolon(row.TermsUnknown),
			ReceiptText:      row.ReceiptText,
		})
	}
	for _, row := range view.Preferences.UsualModalities {
		panel.Preferences.Modalities = append(panel.Preferences.Modalities, decisionContextModalityRow{
			Key: row.MaterialKey, Word: row.Word,
		})
	}
	for _, row := range view.Recommendation.Checks {
		panel.Recommendation.Checks = append(panel.Recommendation.Checks, decisionContextCheckRow{
			Key: row.MaterialKey, State: row.State,
		})
	}
	for _, claim := range view.Summary.Claims {
		panel.Summary.Claims = append(panel.Summary.Claims, decisionContextClaimRow{
			RefType: claim.RefType, RefID: claim.RefID, Disputed: claim.Disputed, Text: claim.Text,
		})
	}
	return panel
}

// assuranceLine renders one assurance half (the identity or the
// authority): the level word, the method word and the honest expiry.
func assuranceLine(level, method string, expiresAt time.Time) string {
	if level == "" {
		return "нет записи"
	}
	line := level + " (" + method + ")"
	if !expiresAt.IsZero() {
		line += " — истекает " + formatTime(expiresAt)
	}
	return line
}

// factProvenanceText renders the provenance of one fact row: the source
// type with the id and the channel (the honest anchor of the value).
func factProvenanceText(row casesAPI.ContextFactRow) string {
	provenance := row.SourceType
	if row.SourceID != "" {
		provenance += " " + row.SourceID
	}
	if row.Channel != "" {
		provenance += " · " + row.Channel
	}
	return provenance
}

// observationTechniqueText renders the provider technique of one
// observation row (the projection, the external id and the external
// version — the render puts the line behind the progressive disclosure).
func observationTechniqueText(row casesAPI.ContextObservationRow) string {
	parts := make([]string, 0, 3)
	if row.Projection != "" {
		parts = append(parts, "проекция: "+row.Projection)
	}
	if row.ExternalID != "" {
		parts = append(parts, "внешний id: "+row.ExternalID)
	}
	if row.ExternalVersion != "" {
		parts = append(parts, "внешняя версия: "+row.ExternalVersion)
	}
	if len(parts) == 0 {
		return "техника провайдера не передана"
	}
	return strings.Join(parts, "; ")
}

// joinWithSemicolon joins the words of one line with the honest
// separator (the empty list renders the empty string — the honest
// absence).
func joinWithSemicolon(words []string) string {
	if len(words) == 0 {
		return ""
	}
	return strings.Join(words, "; ")
}

// formatCounter renders one monotonic counter of the context words (the
// version, the epoch, the summary number) — the honest empty string of
// the zero counter (the render never invents a number).
func formatCounter(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}
