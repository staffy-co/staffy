package httptransport

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/correlation"
	"github.com/artemiosu/staffy/internal/platform/id"
)

func bytesReader(data []byte) io.Reader { return bytes.NewReader(data) }

func correlationFromRequest(r *http.Request) (id.UUID, error) {
	if value, ok := correlation.FromContext(r.Context()); ok {
		return value, nil
	}
	return id.UUID(""), nil
}

// casesPageView is the flattened, template-friendly projection of the
// Cases panel: every row rendered as text — accessible without color,
// keyboard-first, honest statements included.
type casesPageView struct {
	ScopeText     string
	ForbiddenText string
	HasCases      bool
	Cases         []casesRow
	HasExceptions bool
	Exceptions    []casesExceptionRow
}

type casesRow struct {
	CaseID        string
	CaseNo        string
	EmployeeID    string
	StateText     string
	Conversations string
	Events        string
	CreatedAt     string
	LastEventAt   string
	OwnerID       string
}

type casesExceptionRow struct {
	ExceptionID   string
	Kind          string
	KindText      string
	Summary       string
	CorrelationID string
	CreatedAt     string
}

func prepareCasesView(view casesAPI.CasesView) casesPageView {
	out := casesPageView{
		ScopeText:     casesScopeText(true),
		HasCases:      len(view.Cases) > 0,
		Cases:         make([]casesRow, 0, len(view.Cases)),
		HasExceptions: len(view.Exceptions) > 0,
		Exceptions:    make([]casesExceptionRow, 0, len(view.Exceptions)),
	}
	for _, summary := range view.Cases {
		out.Cases = append(out.Cases, casesRow{
			CaseID:        summary.CaseID.String(),
			CaseNo:        strconv.FormatInt(summary.CaseNo, 10),
			EmployeeID:    summary.EmployeeID.String(),
			StateText:     "Открыт",
			Conversations: strconv.FormatInt(int64(summary.Conversations), 10),
			Events:        strconv.FormatInt(int64(summary.Events), 10),
			CreatedAt:     formatTime(summary.CreatedAt),
			LastEventAt:   formatTime(summary.LastEventAt),
			OwnerID:       summary.OwnerID.String(),
		})
	}
	for _, exception := range view.Exceptions {
		out.Exceptions = append(out.Exceptions, casesExceptionRow{
			ExceptionID:   exception.ExceptionID.String(),
			Kind:          exception.Kind,
			KindText:      exceptionKindText(exception.Kind),
			Summary:       exception.Summary,
			CorrelationID: exception.CorrelationID.String(),
			CreatedAt:     formatTime(exception.CreatedAt),
		})
	}
	return out
}

// caseDetailPageView is the flattened projection of one case: the
// conversations with owners and surfaces, the timeline (both times, the
// connection, the state), the Work Control Cards (the derived work items
// with their facts and the honest actions) and the honest statements.
type caseDetailPageView struct {
	ScopeText        string
	CaseNo           string
	CaseID           string
	EmployeeID       string
	PackText         string
	CreatedAt        string
	UntriagedText    string
	NoAutoReplyText  string
	HasConversations bool
	Conversations    []caseConversationRow
	HasTimeline      bool
	Timeline         []caseTimelineRow
	HasWorkItems     bool
	WorkItems        []caseWorkItemRow
	HasExceptions    bool
	Exceptions       []casesExceptionRow
	CanViewPayload   bool
	// The Work Control Card material of Story 2.2:
	CanManageWork    bool
	TaxonomyKnown    bool
	TaxonomyTypes    []string
	MaterialKeyHints string
	WorkCards        []workCardView
	HasFacts         bool
	FactsAllowed     bool
	FactsNote        string
	ActiveWIOptions  []workOption
	ConversationOpts []workOption
	EventOptions     []workOption
	Flash            string
	// The Story 2.3 sections: the assurance panel, the bindings and the
	// duplicate cards.
	Assurance           assurancePanelView
	HasBindings         bool
	BindingRows         []bindingRowView
	HasDuplicates       bool
	DuplicateCards      []duplicateCardView
	VerificationOpts    []workOption
	KindOptions         []workOption
	MethodOptions       []workOption
	ProviderCodeOpts    []workOption
	IdentifierKindsText string
	// The Story 2.4 section: the Response Owner Control panel.
	Ownership ownershipPanelView
	// The Story 2.5 section: the Conversation Mode panel and the
	// disclosure surface.
	Mode modePanelView
	// The Story 2.6 section: the Progressive Intake / Policy panel.
	Policy policyPanelView
}

// assurancePanelView is the assurance section of the case page: the derived
// state text, the records table, the allowed methods and the step-up
// notice.
type assurancePanelView struct {
	IdentityText     string
	IdentityMethod   string
	IdentitySubject  string
	IdentityExpires  string
	AuthorityText    string
	AuthorityScope   string
	AuthorityExpires string
	StepUp           bool
	StepUpText       string
	HasRecords       bool
	Records          []verificationRecordRow
	AllowedMethods   string
	FallbackOptions  []string
}

// verificationRecordRow is one verification record row of the panel.
type verificationRecordRow struct {
	VerificationID string
	Kind           string
	Method         string
	OutcomeText    string
	Level          string
	Subject        string
	Evidence       string
	IssuedAt       string
	ExpiresAt      string
}

// bindingRowView is one binding row of the case page.
type bindingRowView struct {
	BindingID        string
	Method           string
	Purpose          string
	OnBehalf         bool
	Subject          string
	TargetCaseNo     string
	BoundCaseNo      string
	CreatedAt        string
	Active           bool
	ReversalReason   string
	ReversalAt       string
	ConfirmationText string
	CanUnlink        bool
}

// duplicateCardView is one duplicate candidate card of the case page: the
// masked minimal material only.
type duplicateCardView struct {
	CandidateID    string
	NewCaseNo      string
	ExistingCaseNo string
	MaterialKey    string
	MaskedValue    string
	Channel        string
	IsNewSide      bool
	MatchReason    string
	RuleConfidence string
	Resolved       bool
	DecisionText   string
	ResolutionNote string
	ReviewText     string
}

// ownershipPanelView is the Response Owner Control section of the case
// page: the current owner with the honest name, the epoch, the surface,
// the transfer state (the target, the deadline, the remaining minutes),
// the health word, the backup owner and the safe reassignment path; the
// term history and the outgoing replies with their states.
type ownershipPanelView struct {
	OwnerText        string
	OwnerName        string
	Epoch            string
	SurfaceText      string
	HealthText       string
	HealthClass      string
	HasPending       bool
	PendingTarget    string
	PendingDeadline  string
	PendingRemaining string
	PendingRequestID string
	PendingTimedOut  bool
	BackupOwnerText  string
	AckDeadlineText  string
	PrimaryAction    string
	Takeable         bool
	CanTake          bool
	CanRequest       bool
	CanReassign      bool
	CanReply         bool
	CurrentEpoch     string
	HasTerms         bool
	TermRows         []ownershipTermRow
	HasReplies       bool
	ReplyRows        []replyRowView
	MemberOptions    []workOption
}

// ownershipTermRow is one term row of the ownership history.
type ownershipTermRow struct {
	TermID     string
	Epoch      string
	OwnerText  string
	OriginText string
	Surface    string
	Reason     string
	ActorID    string
	CreatedAt  string
}

// replyRowView is one outgoing reply row of the case timeline: the state
// is the honest derivation (prepared — waiting the drain; dispatched —
// the synthetic contour record; stale_needs_review — blocked).
type replyRowView struct {
	ReplyID   string
	Channel   string
	Content   string
	ClassText string
	StateText string
	State     string
	CreatedAt string
	OutcomeAt string
	Epoch     string
	OwnerText string
}

// modePanelView is the Conversation Mode panel projection: the badge word
// of the active mode with the AI role of the owner, the honest words of
// the active term (the reason, the signal source, the rule version), the
// ladder, the term history, the Work Item contexts, the forbidden action
// words, the disclosure records with their delivery statuses and the form
// options (the propose targets, the exit targets, the event options of
// the turn signals, the Work Item options of the item contexts).
type modePanelView struct {
	BadgeText       string
	BadgeRole       string
	ModeVersion     string
	ActiveMode      string
	ActiveReason    string
	ActiveSignal    string
	ActiveRule      string
	HasActive       bool
	ForbiddenText   string
	NeutralRule     string
	Ladder          []workOption
	CurrentVersion  string
	ProposeTargets  []workOption
	ExitTargets     []workOption
	HasTerms        bool
	TermRows        []modeTermRow
	HasItemContexts bool
	ItemContextRows []modeContextRow
	HasDisclosures  bool
	DisclosureRows  []disclosureRowView
	CanManage       bool
}

// modeTermRow is one term row of the mode ladder history.
type modeTermRow struct {
	TermID     string
	Version    string
	ModeText   string
	Scope      string
	ItemText   string
	OriginText string
	Reason     string
	SignalText string
	ActorID    string
	CreatedAt  string
}

// modeContextRow is one Work Item context row of the panel.
type modeContextRow struct {
	WorkItemID string
	ModeText   string
	Version    string
	Reason     string
	SignalText string
	CreatedAt  string
}

// disclosureRowView is one disclosure record row: the exact text (the
// re-readable word of the record), the channel, the derived status and
// the parent reference of the re-disclosure.
type disclosureRowView struct {
	DisclosureID string
	No           string
	Channel      string
	Text         string
	StatusText   string
	Status       string
	ParentRef    string
	CreatedAt    string
}

// externalCaseNo renders the case number of the binding sides: the number
// of the OTHER case is honestly absent in this view (zero degrades to
// «внешний», never to a wrong «№0»).
func externalCaseNo(caseNo int64) string {
	if caseNo == 0 {
		return "внешний"
	}
	return strconv.FormatInt(caseNo, 10)
}

// assuranceLevelText renders the honest level text of the panel.
func assuranceLevelText(level string) string {
	switch level {
	case casesDomain.AssuranceNone:
		return "не проверена"
	case casesDomain.AssuranceBasic:
		return "basic (проверка знанием)"
	case casesDomain.AssuranceVerified:
		return "verified (подтверждена)"
	}
	return level
}

// verificationOutcomeText renders the canonical outcome text.
func verificationOutcomeText(outcome string) string {
	switch outcome {
	case casesDomain.VerificationPassed:
		return "пройдена"
	case casesDomain.VerificationFailed:
		return "не пройдена"
	case casesDomain.VerificationExpired:
		return "истекла"
	case casesDomain.VerificationCancelled:
		return "отменена"
	case casesDomain.VerificationAmbiguous:
		return "неоднозначна"
	}
	return outcome
}

// bindingMethodText renders the binding method text.
func bindingMethodText(method string) string {
	switch method {
	case casesDomain.BindingMethodConsentHandoff:
		return "согласие клиента (consent_handoff)"
	case casesDomain.BindingMethodDuplicateReview:
		return "решение ревьюера дубля (duplicate_review)"
	}
	return method
}

// duplicateDecisionText renders the resolution decision text.
func duplicateDecisionText(decision string) string {
	switch decision {
	case casesDomain.DuplicateDecisionSameClient:
		return "тот же клиент — связаны"
	case casesDomain.DuplicateDecisionSeparate:
		return "разные клиенты — раздельны"
	}
	return decision
}

// workOption is one select option of the WCC forms.
type workOption struct {
	Value string
	Text  string
}

type caseConversationRow struct {
	ConversationID    string
	ConversationNo    string
	Provider          string
	ProviderSessionID string
	OwnerID           string
	OwnerEpoch        string
	WritableSurface   string
	StateText         string
	CreatedAt         string
}

type caseTimelineRow struct {
	EventID           string
	Channel           string
	Modality          string
	Direction         string
	Provider          string
	ProviderEventID   string
	State             string
	StateText         string
	ProviderTimestamp string
	ReceivedAt        string
	PayloadDigest     string
}

type caseWorkItemRow struct {
	WorkItemID string
	StateText  string
	CreatedAt  string
}

// materialKeyHints renders the material key hint of the fact form from the
// CriticalEntities dictionary of the pinned pack (the frozen spec: the
// hints come from the pack, never a hardcoded list); an unreadable or
// empty pin degrades to the generic wording, honestly.
func materialKeyHints(criticalEntities []string) string {
	if len(criticalEntities) == 0 {
		return "Материальный ключ сущности обращения (до 64 символов; словарь пака не закреплён)"
	}
	return "Материальный ключ (до 64 символов; словарь закреплённого пака: " + strings.Join(criticalEntities, ", ") + ")"
}

func prepareCaseView(view casesAPI.CaseView, facts casesAPI.FactsView, flash string) caseDetailPageView {
	out := caseDetailPageView{
		ScopeText:        casesScopeText(true),
		CaseNo:           strconv.FormatInt(view.Case.CaseNo, 10),
		CaseID:           view.Case.CaseID.String(),
		EmployeeID:       view.Case.EmployeeID.String(),
		PackText:         "не закреплён (до-миграционные данные)",
		CreatedAt:        formatTime(view.Case.CreatedAt),
		UntriagedText:    view.UntriagedText,
		NoAutoReplyText:  view.NoAutoReplyText,
		HasConversations: len(view.Conversations) > 0,
		Conversations:    make([]caseConversationRow, 0, len(view.Conversations)),
		HasTimeline:      len(view.Timeline) > 0,
		Timeline:         make([]caseTimelineRow, 0, len(view.Timeline)),
		HasWorkItems:     len(view.WorkItems) > 0,
		WorkItems:        make([]caseWorkItemRow, 0, len(view.WorkItems)),
		HasExceptions:    len(view.Exceptions) > 0,
		Exceptions:       make([]casesExceptionRow, 0, len(view.Exceptions)),
		CanViewPayload:   view.CanViewPayload,
		CanManageWork:    view.CanManageWork,
		TaxonomyKnown:    len(view.TaxonomyTypes) > 0,
		TaxonomyTypes:    view.TaxonomyTypes,
		MaterialKeyHints: materialKeyHints(view.CriticalEntities),
		WorkCards:        prepareWorkCards(view, facts),
		HasFacts:         len(facts.Facts) > 0,
		FactsAllowed:     view.CanViewFacts,
		FactsNote:        "Значения фактов — только полномочия review или audit.",
		Flash:            flash,
	}
	if view.Case.PackID != "" {
		out.PackText = view.Case.PackID + "@" + view.Case.PackVersion
	}
	if !view.CanManageWork {
		out.FactsNote = "Значения фактов — только полномочия review или audit; управление элементами требует полномочия review."
	}
	// The global form options: the active work items (the fact scope), the
	// conversations (the new intent) and the accepted events (the source).
	for _, item := range view.WorkItems {
		if !item.Terminal {
			out.ActiveWIOptions = append(out.ActiveWIOptions, workOption{
				Value: item.WorkItemID.String(),
				Text:  "№" + item.WorkItemID.String() + " (" + workItemStateText(item.State) + ")",
			})
		}
	}
	for _, conversation := range view.Conversations {
		out.ConversationOpts = append(out.ConversationOpts, workOption{
			Value: conversation.ConversationID.String(),
			Text:  "№" + strconv.FormatInt(conversation.ConversationNo, 10) + " (" + conversation.Provider + ")",
		})
	}
	// The Story 2.3 form dictionaries: the kinds, the methods and the
	// provider codes of the synthetic contour (the fixed domain
	// dictionaries, never invented options).
	out.KindOptions = []workOption{
		{Value: casesDomain.VerificationKindIdentity, Text: "Личность (identity)"},
		{Value: casesDomain.VerificationKindAuthority, Text: "Полномочия (authority)"},
	}
	out.MethodOptions = []workOption{
		{Value: casesDomain.MethodKnowledgeCheck, Text: "Проверка знанием (knowledge_check, basic)"},
		{Value: casesDomain.MethodKnownNumberCallback, Text: "Перезвон на известный номер (known_number_callback, verified)"},
		{Value: casesDomain.MethodKnownAddressReply, Text: "Ответ с известного адреса (known_address_reply, verified)"},
		{Value: casesDomain.MethodMandateNote, Text: "Основание полномочий (mandate_note, verified)"},
	}
	out.ProviderCodeOpts = []workOption{
		{Value: casesDomain.ProviderCodeOK, Text: "ok — проверка пройдена"},
		{Value: casesDomain.ProviderCodeNoMatch, Text: "no_match — не совпало"},
		{Value: casesDomain.ProviderCodeExpired, Text: "expired — истек материал"},
		{Value: casesDomain.ProviderCodeCancelled, Text: "cancelled — отменена"},
		{Value: casesDomain.ProviderCodeUnclear, Text: "unclear — неоднозначно"},
	}
	out.IdentifierKindsText = strings.Join(casesDomain.IdentifierKinds, ", ")
	for _, event := range view.Timeline {
		if event.State == "accepted" {
			out.EventOptions = append(out.EventOptions, workOption{
				Value: event.EventID.String(),
				Text:  event.Channel + " · " + formatTime(event.ReceivedAt) + " · " + event.EventID.String(),
			})
		}
	}
	// The Story 2.3 sections: the assurance panel, the binding rows and
	// the duplicate cards (the masked minimal material). The form options
	// of the verification commands come from the same dictionaries the
	// domain declares.
	out.Assurance = assurancePanelView{
		IdentityText:     assuranceLevelText(view.Assurance.State.IdentityLevel),
		IdentityMethod:   view.Assurance.State.IdentityMethod,
		IdentitySubject:  view.Assurance.State.IdentitySubject,
		IdentityExpires:  formatTime(view.Assurance.State.IdentityExpiresAt),
		AuthorityText:    assuranceLevelText(view.Assurance.State.AuthorityLevel),
		AuthorityScope:   view.Assurance.State.AuthorityScope,
		AuthorityExpires: formatTime(view.Assurance.State.AuthorityExpiresAt),
		StepUp:           view.Assurance.State.StepUp,
		StepUpText:       view.Assurance.StepUpText,
		HasRecords:       len(view.Assurance.Records) > 0,
		Records:          make([]verificationRecordRow, 0, len(view.Assurance.Records)),
		AllowedMethods:   strings.Join(view.Assurance.AllowedMethods, ", "),
		FallbackOptions:  view.Assurance.FallbackOptions,
	}
	for _, record := range view.Assurance.Records {
		evidence := "нота: " + record.Note
		if record.EvidenceEventID.Valid() {
			evidence = "событие " + record.EvidenceEventID.String()
		}
		out.Assurance.Records = append(out.Assurance.Records, verificationRecordRow{
			VerificationID: record.VerificationID.String(),
			Kind:           record.Kind, Method: record.Method,
			OutcomeText: verificationOutcomeText(record.Outcome), Level: record.AssuranceLevel,
			Subject: record.SubjectHandle, Evidence: evidence,
			IssuedAt: formatTime(record.IssuedAt), ExpiresAt: formatTime(record.ExpiresAt),
		})
	}
	out.HasBindings = len(view.Bindings) > 0
	out.BindingRows = make([]bindingRowView, 0, len(view.Bindings))
	for _, binding := range view.Bindings {
		row := bindingRowView{
			BindingID:        binding.Binding.BindingID.String(),
			Method:           bindingMethodText(binding.Binding.Method),
			Purpose:          binding.Binding.Purpose,
			OnBehalf:         binding.Binding.OnBehalf,
			Subject:          binding.Binding.SubjectHandle,
			TargetCaseNo:     externalCaseNo(binding.TargetCaseNo),
			BoundCaseNo:      externalCaseNo(binding.BoundCaseNo),
			CreatedAt:        formatTime(binding.Binding.CreatedAt),
			Active:           binding.Reversal == nil,
			ConfirmationText: binding.ConfirmationText,
			CanUnlink:        view.CanManageWork && binding.Reversal == nil,
		}
		if binding.Reversal != nil {
			row.ReversalReason = binding.Reversal.Reason
			row.ReversalAt = formatTime(binding.Reversal.CreatedAt)
		}
		out.BindingRows = append(out.BindingRows, row)
	}
	out.HasDuplicates = len(view.Duplicates) > 0
	out.DuplicateCards = make([]duplicateCardView, 0, len(view.Duplicates))
	for _, card := range view.Duplicates {
		row := duplicateCardView{
			CandidateID:    card.Candidate.CandidateID.String(),
			NewCaseNo:      strconv.FormatInt(card.NewCaseNo, 10),
			ExistingCaseNo: strconv.FormatInt(card.ExistingCaseNo, 10),
			MaterialKey:    card.Candidate.MaterialKey,
			MaskedValue:    card.MaskedValue,
			Channel:        card.Channel,
			IsNewSide:      card.IsNewSide,
			MatchReason:    card.Candidate.MatchReason,
			RuleConfidence: card.Candidate.RuleConfidence,
			ReviewText:     casesDomain.DuplicateReviewText,
		}
		if card.Resolution != nil {
			row.Resolved = true
			row.DecisionText = duplicateDecisionText(card.Resolution.Decision)
			row.ResolutionNote = card.Resolution.Note
		}
		out.DuplicateCards = append(out.DuplicateCards, row)
	}
	// The Story 2.4 section: the Response Owner Control panel — the
	// derivation words, the history, the outgoing replies and the member
	// options of the transfer forms.
	out.Ownership = prepareOwnershipPanel(view)
	out.Mode = prepareModePanel(view)
	out.Policy = preparePolicyPanel(view, out.ConversationOpts, out.ActiveWIOptions, out.EventOptions)

	for _, conversation := range view.Conversations {
		stateText := "Активна"
		if conversation.CaseID != view.Case.CaseID {
			stateText = "Связанный канал (связан с этим кейсом)"
		}
		out.Conversations = append(out.Conversations, caseConversationRow{
			ConversationID:    conversation.ConversationID.String(),
			ConversationNo:    strconv.FormatInt(conversation.ConversationNo, 10),
			Provider:          conversation.Provider,
			ProviderSessionID: conversation.ProviderSessionID,
			OwnerID:           conversation.ResponseOwnerID.String(),
			OwnerEpoch:        strconv.FormatInt(conversation.ResponseOwnerEpoch, 10),
			WritableSurface:   conversation.WritableSurface,
			StateText:         stateText,
			CreatedAt:         formatTime(conversation.CreatedAt),
		})
	}
	for _, event := range view.Timeline {
		out.Timeline = append(out.Timeline, caseTimelineRow{
			EventID:           event.EventID.String(),
			Channel:           event.Channel,
			Modality:          event.Modality,
			Direction:         event.Direction,
			Provider:          event.Provider,
			ProviderEventID:   event.ProviderEventID,
			State:             event.State,
			StateText:         eventStateText(event.State),
			ProviderTimestamp: formatTime(event.ProviderTimestamp),
			ReceivedAt:        formatTime(event.ReceivedAt),
			PayloadDigest:     event.PayloadDigest,
		})
	}
	for _, item := range view.WorkItems {
		out.WorkItems = append(out.WorkItems, caseWorkItemRow{
			WorkItemID: item.WorkItemID.String(),
			StateText:  workItemStateText(item.State),
			CreatedAt:  formatTime(item.CreatedAt),
		})
	}
	for _, exception := range view.Exceptions {
		out.Exceptions = append(out.Exceptions, casesExceptionRow{
			ExceptionID:   exception.ExceptionID.String(),
			Kind:          exception.Kind,
			KindText:      exceptionKindText(exception.Kind),
			Summary:       exception.Summary,
			CorrelationID: exception.CorrelationID.String(),
			CreatedAt:     formatTime(exception.CreatedAt),
		})
	}
	return out
}

func casesScopeText(canView bool) string {
	if canView {
		return "просмотр обращений (review, configuration или audit); payload — только review или audit"
	}
	return "нет полномочий на контур обращений"
}

func eventStateText(state string) string {
	switch state {
	case "accepted":
		return "Принято"
	case "quarantined":
		return "Карантин"
	}
	return state
}

func exceptionKindText(kind string) string {
	switch kind {
	case "digest_conflict":
		return "Digest-конфликт события"
	case "ambiguous_employee":
		return "Неоднозначная маршрутизация"
	case "idempotency_key_conflict":
		return "Конфликт идемпотентного ключа"
	}
	return kind
}

func formatTime(value time.Time) string {
	return value.UTC().Format("02.01.2006 15:04:05 UTC")
}

// ---------------------------------------------------------------------------
// Story 2.2: the Work Control Card projection
// ---------------------------------------------------------------------------

// workCardView is one Work Control Card: the derived current material of a
// work item with the honest next action, its facts (the current scope) and
// the form options of the commands the state allows.
type workCardView struct {
	WorkItemID string
	IndexNo    string
	TypeText   string
	TitleText  string
	State      string
	StateText  string
	Version    string
	KindText   string
	Reason     string
	ActorID    string
	RefText    string
	Terminal   bool
	NextAction string
	CreatedAt  string
	HasFacts   bool
	Facts      []factCardView
	// The form options of this card (empty when the state refuses them).
	AllowedTransitions []transitionOption
	MergeSources       []mergeSourceOption
}

type transitionOption struct {
	Value string
	Text  string
}

type mergeSourceOption struct {
	Value string
	Text  string
}

// factCardView is one fact of the Work Control Card: the derived status,
// the honest next action and the FULL version history with the provenance
// (both values of a conflict are always present).
type factCardView struct {
	FactID      string
	MaterialKey string
	Status      string
	StatusText  string
	NextAction  string
	Versions    []factVersionView
}

type factVersionView struct {
	VersionNo     string
	Value         string
	Digest        string
	SourceType    string
	SourceID      string
	Channel       string
	ActorID       string
	ObservedAt    string
	ReceivedAt    string
	CreatedAt     string
	ConfirmedText string
}

func workItemStateText(state string) string {
	switch state {
	case "untriaged":
		return "Запрос ещё не классифицирован"
	case "in_work":
		return "В работе"
	case "waiting":
		return "Ожидание возврата клиента"
	case "resolved":
		return "Завершено"
	case "cancelled":
		return "Отменено"
	case "split":
		return "Разделён"
	case "merged":
		return "Объединён"
	}
	return state
}

func workItemKindText(kind string) string {
	switch kind {
	case "create":
		return "Создание"
	case "classify":
		return "Классификация"
	case "transition":
		return "Переход"
	case "split":
		return "Разделение"
	case "merge":
		return "Объединение"
	}
	return kind
}

func factStatusText(status string) string {
	switch status {
	case "confirmed":
		return "Подтверждено"
	case "conflicting":
		return "Конфликт значений"
	case "unverified":
		return "Без подтверждения"
	}
	return status
}

// allowedTransitionsOf the state (the honest ladder of the form).
func allowedTransitionsOf(state string) []transitionOption {
	var options []transitionOption
	switch state {
	case "in_work":
		options = []transitionOption{{"waiting", "Ожидание возврата клиента"}, {"resolved", "Завершено"}, {"cancelled", "Отменено"}}
	case "waiting":
		options = []transitionOption{{"in_work", "Вернуть в работу"}, {"resolved", "Завершено"}, {"cancelled", "Отменено"}}
	}
	return options
}

// prepareWorkCards builds the Work Control Cards: the derived work items
// with the fact grouping by the current scope, the honest actions and the
// form options. The facts of a split source stay on its card (the rescope
// is the explicit command); the facts of merged sources live on the target.
func prepareWorkCards(view casesAPI.CaseView, facts casesAPI.FactsView) []workCardView {
	factsByScope := map[id.UUID][]factCardView{}
	for _, fact := range facts.Facts {
		card := factCardView{
			FactID: fact.FactID.String(), MaterialKey: fact.MaterialKey, Status: fact.Status,
			StatusText: factStatusText(fact.Status), NextAction: fact.NextAction,
			Versions: make([]factVersionView, 0, len(fact.Versions)),
		}
		for _, version := range fact.Versions {
			confirmed := ""
			for _, confirmation := range fact.Confirmations {
				if confirmation.VersionNo == version.VersionNo {
					confirmed = "подтверждена"
					if confirmation.EvidenceEventID.Valid() {
						confirmed = "подтверждена (evidence: событие " + confirmation.EvidenceEventID.String() + ")"
					} else if confirmation.Note != "" {
						confirmed = "подтверждена (нота: " + confirmation.Note + ")"
					}
				}
			}
			card.Versions = append(card.Versions, factVersionView{
				VersionNo: strconv.FormatInt(version.VersionNo, 10), Value: string(version.Value),
				Digest: version.ValueDigest, SourceType: version.SourceType, SourceID: version.SourceID,
				Channel: version.Channel, ActorID: version.ActorID.String(),
				ObservedAt: formatTime(version.ObservedAt), ReceivedAt: formatTime(version.ReceivedAt),
				CreatedAt: formatTime(version.CreatedAt), ConfirmedText: confirmed,
			})
		}
		if len(fact.Versions) > 0 {
			scope := fact.Versions[len(fact.Versions)-1].WorkItemID
			factsByScope[scope] = append(factsByScope[scope], card)
		}
	}
	cards := make([]workCardView, 0, len(view.WorkItems))
	for index, item := range view.WorkItems {
		card := workCardView{
			WorkItemID: item.WorkItemID.String(), IndexNo: strconv.Itoa(index + 1),
			TypeText: item.Type, TitleText: item.Title, State: item.State,
			StateText: workItemStateText(item.State), Version: strconv.FormatInt(item.Version, 10),
			KindText: workItemKindText(item.Kind), Reason: item.Reason, ActorID: item.ActorID.String(),
			Terminal: item.Terminal, NextAction: casesDomain.RecommendedNextAction(item.State),
			CreatedAt: formatTime(item.CreatedAt), Facts: factsByScope[item.WorkItemID],
		}
		if item.Type == "" {
			card.TypeText = "— не классифицирован —"
		}
		if item.Title == "" {
			card.TitleText = "— без названия —"
		}
		if item.Ref.Valid() {
			card.RefText = item.Ref.String()
		}
		card.HasFacts = len(card.Facts) > 0
		if !item.Terminal {
			card.AllowedTransitions = allowedTransitionsOf(item.State)
			for _, other := range view.WorkItems {
				if other.WorkItemID != item.WorkItemID && !other.Terminal {
					card.MergeSources = append(card.MergeSources, mergeSourceOption{
						Value: other.WorkItemID.String() + ":" + strconv.FormatInt(other.Version, 10),
						Text:  "№" + other.WorkItemID.String() + " (" + workItemStateText(other.State) + ")",
					})
				}
			}
		}
		cards = append(cards, card)
	}
	return cards
}

// prepareOwnershipPanel flattens the ownership section into the template
// projection: every row rendered as text — accessible without color,
// assigned and acknowledged never confusable, the primary action matching
// the current state.
func prepareOwnershipPanel(view casesAPI.CaseView) ownershipPanelView {
	ownership := view.Ownership
	state := ownership.State
	panel := ownershipPanelView{
		OwnerName:     ownership.OwnerName,
		CurrentEpoch:  strconv.FormatInt(0, 10),
		HasTerms:      len(ownership.Terms) > 0,
		TermRows:      make([]ownershipTermRow, 0, len(ownership.Terms)),
		HasReplies:    len(ownership.Replies) > 0,
		ReplyRows:     make([]replyRowView, 0, len(ownership.Replies)),
		MemberOptions: make([]workOption, 0, len(ownership.Members)),
		CanTake:       view.CanManageWork,
		CanRequest:    view.CanManageWork,
		CanReassign:   view.CanManageWork,
		CanReply:      view.CanManageWork,
	}
	if state.Current != nil {
		panel.Epoch = strconv.FormatInt(state.Current.Epoch, 10)
		panel.CurrentEpoch = panel.Epoch
		panel.SurfaceText = surfaceText(state.Current.WritableSurface)
		if state.Current.OwnerKind == casesDomain.OwnerKindStaffy {
			panel.OwnerText = "цифровой сотрудник"
		} else {
			panel.OwnerText = "участник " + ownership.OwnerName
		}
	}
	panel.HealthText = ownershipHealthText(state.Health)
	panel.HealthClass = "next-note"
	if state.Health != casesDomain.OwnershipHealthStable {
		panel.HealthClass = "alert"
	}
	if state.Pending != nil {
		panel.HasPending = true
		panel.PendingTarget = ownership.PendingTargetName
		panel.PendingDeadline = formatTime(state.Pending.DeadlineAt)
		panel.PendingRequestID = state.Pending.RequestID.String()
		remaining := state.Pending.DeadlineAt.Sub(timeNowHTTP()).Minutes()
		if state.PendingTimedOut {
			panel.PendingTimedOut = true
			panel.PendingRemaining = "дедлайн истёк — запрос погашен"
		} else if remaining >= 0 {
			panel.PendingRemaining = strconv.FormatInt(int64(remaining), 10) + " мин. до дедлайна"
		}
	}
	if state.BackupOwnerID != (id.UUID("")) {
		panel.BackupOwnerText = state.BackupOwnerID.String()
	}
	if state.AckDeadlineMinutes > 0 {
		panel.AckDeadlineText = strconv.FormatInt(state.AckDeadlineMinutes, 10) + " мин."
	}
	panel.Takeable = state.Takeable()
	panel.PrimaryAction = ownershipPrimaryAction(state, view.CanManageWork)
	for _, term := range ownership.Terms {
		row := ownershipTermRow{
			TermID:     term.TermID.String(),
			Epoch:      strconv.FormatInt(term.Epoch, 10),
			Surface:    surfaceText(term.WritableSurface),
			OriginText: ownershipOriginText(term.Origin),
			Reason:     term.Reason,
			ActorID:    term.ActorID.String(),
			CreatedAt:  formatTime(term.CreatedAt),
		}
		if term.OwnerKind == casesDomain.OwnerKindStaffy {
			row.OwnerText = "цифровой сотрудник"
		} else {
			row.OwnerText = "участник " + term.OwnerRef.String()
		}
		panel.TermRows = append(panel.TermRows, row)
	}
	for _, card := range ownership.Replies {
		row := replyRowView{
			ReplyID:   card.Reply.ReplyID.String(),
			Channel:   card.Reply.Channel,
			Content:   card.Reply.Content,
			ClassText: replyClassText(card.Reply.ReplyClass),
			Epoch:     strconv.FormatInt(card.Reply.Epoch, 10),
			CreatedAt: formatTime(card.Reply.CreatedAt),
			State:     card.OutcomeKind,
			StateText: replyStateText(card.OutcomeKind),
			OwnerText: card.Reply.OwnerRef.String(),
		}
		if card.OutcomeKind != "" {
			row.OutcomeAt = formatTime(card.OutcomeAt)
		}
		panel.ReplyRows = append(panel.ReplyRows, row)
	}
	for _, member := range ownership.Members {
		panel.MemberOptions = append(panel.MemberOptions, workOption{
			Value: member.PrincipalID.String(),
			Text:  member.Name + " (" + member.PrincipalID.String() + ")",
		})
	}
	return panel
}

// ownershipHealthText renders the health word (assigned and acknowledged
// are never confusable — AC-12).
func ownershipHealthText(health string) string {
	switch health {
	case casesDomain.OwnershipHealthTransferPending:
		return "Передача назначена, ждёт подтверждения: текущий владелец продолжает отвечать."
	case casesDomain.OwnershipHealthTransferTimedOut:
		return "Дедлайн подтверждения истёк без решения: запрос погашен, владение можно забрать (take)."
	case casesDomain.OwnershipHealthReassignment:
		return "Владелец недоступен: новые исходящие заблокированы до безопасного переназначения."
	}
	return "Владение стабильно."
}

// ownershipOriginText renders the term origin word.
func ownershipOriginText(origin string) string {
	switch origin {
	case casesDomain.OwnershipOriginIngestInitial:
		return "первоначальное назначение (ingest)"
	case casesDomain.OwnershipOriginTake:
		return "забрано (take)"
	case casesDomain.OwnershipOriginTransferAcknowledge:
		return "подтверждённая передача"
	case casesDomain.OwnershipOriginReassign:
		return "переназначение надзора"
	}
	return origin
}

// surfaceText renders the writable surface word.
func surfaceText(surface string) string {
	if surface == casesDomain.WritableSurfaceExternalInbox {
		return "внешний Inbox (по доказательству)"
	}
	return "Staffy inbox"
}

// replyStateText renders the reply state word.
func replyStateText(state string) string {
	switch state {
	case casesDomain.ReplyOutcomeDispatched:
		return "отправлено (запись контура; реальная доставка — исходящие)"
	case casesDomain.ReplyOutcomeStaleNeedsReview:
		return "устарело — требуется решение человека (payload не пересылается)"
	}
	return "подготовлено (ждёт execute-time проверки)"
}

// ownershipPrimaryAction resolves the primary action of the panel: the
// owner answers or transfers; the target acknowledges or declines; a
// bystander requests the transfer.
func ownershipPrimaryAction(state casesDomain.OwnershipState, canManage bool) string {
	if !canManage {
		return "Только просмотр: команды владения требуют полномочия review."
	}
	if state.Health == casesDomain.OwnershipHealthReassignment {
		return "Переназначение: забрать (take) резервному владельцу или переназначить (надзор)."
	}
	if state.Pending != nil && !state.PendingTimedOut {
		return "Действие цели запроса: принять или отклонить передачу."
	}
	if state.PendingTimedOut {
		return "Дедлайн истёк: забрать владение (take) или отменить запрос автору."
	}
	return "Владелец: ответить клиенту или передать владение."
}

// timeNowHTTP is the page clock word (the transport does not carry a
// per-request clock — the remaining minutes render from the view time).
func timeNowHTTP() time.Time {
	return time.Now().UTC()
}

// replyClassText renders the producer's reply class word (the honest
// absence of the pre-migration rows).
func replyClassText(class string) string {
	switch class {
	case casesDomain.ReplyClassSales:
		return "продажной"
	case casesDomain.ReplyClassNeutral:
		return "нейтральный"
	case casesDomain.ReplyClassProtectedRequest:
		return "защищённый запрос"
	}
	return "— класс не размечен (до-миграционные данные) —"
}

// modeText renders the mode word of the panel (the human text, never the
// raw dictionary word alone — unclassified and sales never look alike).
func modeText(mode string) string {
	switch mode {
	case casesDomain.ModeSafety:
		return "safety (безопасность)"
	case casesDomain.ModeClaim:
		return "claim (претензия)"
	case casesDomain.ModeCancellation:
		return "cancellation (отмена)"
	case casesDomain.ModePrivacyNoContact:
		return "privacy_no_contact (запрет контакта)"
	case casesDomain.ModeHumanRequested:
		return "human_requested (запрошен человек)"
	case casesDomain.ModeUnclassified:
		return "unclassified (не классифицирован)"
	case casesDomain.ModeSales:
		return "sales (продажи)"
	}
	return mode
}

// modeOriginText renders the term origin word.
func modeOriginText(origin string) string {
	switch origin {
	case casesDomain.ModeOriginIngestInitial:
		return "первоначальное слово (ingest)"
	case casesDomain.ModeOriginProposal:
		return "предложение сигнала"
	case casesDomain.ModeOriginExitOverride:
		return "явный выход (полномочие)"
	}
	return origin
}

// modeSignalText renders the signal source word.
func modeSignalText(source string) string {
	switch source {
	case casesDomain.ModeSignalTurnEvent:
		return "сигнал реплики клиента (turn)"
	case casesDomain.ModeSignalOperator:
		return "наблюдение оператора"
	case casesDomain.ModeSignalPolicyRule:
		return "правило policy"
	}
	return source
}

// deliveryStatusText renders the disclosure delivery status word (the
// states never mix; the playback words are never a proof of hearing).
func deliveryStatusText(status string) string {
	switch status {
	case casesDomain.DeliveryGenerated:
		return "записано (ещё не действует)"
	case casesDomain.DeliverySent:
		return "отправлено (действует)"
	case casesDomain.DeliveryPlaybackAck:
		return "воспроизведение подтверждено (действует; не доказательство услышал)"
	case casesDomain.DeliveryPlaybackEstimated:
		return "воспроизведение оценено (действует; не доказательство услышал)"
	case casesDomain.DeliveryCustomerConfirmed:
		return "клиент подтвердил (действует)"
	case casesDomain.DeliveryInterrupted:
		return "прервано (погашено — нужно повторное раскрытие)"
	}
	return "— без событий (до-миграционные данные) —"
}

// prepareModePanel flattens the mode section into the template
// projection: the badge text (the active mode + the AI role of the owner
// — the owner_kind word of the ownership derivation), the honest words
// of the active term, the ladder in the display order, the term history,
// the Work Item contexts, the disclosure records with their statuses and
// the form options of the propose/exit/turn forms. The aria-live badge
// announces the mode change without a focus move (the template carries
// the aria-live attribute — AC-11).
func prepareModePanel(view casesAPI.CaseView) modePanelView {
	mode := view.Mode
	panel := modePanelView{
		ForbiddenText:   mode.ForbiddenText,
		NeutralRule:     mode.NeutralRule,
		HasTerms:        len(mode.Terms) > 0,
		TermRows:        make([]modeTermRow, 0, len(mode.Terms)),
		HasItemContexts: len(mode.ItemContexts) > 0,
		ItemContextRows: make([]modeContextRow, 0, len(mode.ItemContexts)),
		HasDisclosures:  len(mode.Disclosures) > 0,
		DisclosureRows:  make([]disclosureRowView, 0, len(mode.Disclosures)),
		CanManage:       view.CanManageWork,
	}
	// The ladder in the display order (the spec's written order — the
	// strictest first, sales at the tail).
	for _, word := range casesDomain.ModeOrder {
		panel.Ladder = append(panel.Ladder, workOption{Value: word, Text: modeText(word)})
	}
	if mode.Active != nil {
		panel.HasActive = true
		panel.ActiveMode = modeText(mode.Active.Mode)
		panel.ActiveReason = mode.Active.Reason
		panel.ActiveSignal = modeSignalText(mode.Active.SignalSource)
		panel.ActiveRule = mode.Active.RuleVersion
		panel.ModeVersion = strconv.FormatInt(mode.Active.ModeVersion, 10)
		panel.CurrentVersion = strconv.FormatInt(casesDomain.ModeTermMaxVersion(mode.Terms), 10)
		panel.BadgeText = modeText(mode.Active.Mode)
	} else {
		panel.BadgeText = "не классифицирован (до-миграционные данные)"
		panel.CurrentVersion = "1"
	}
	// The AI role of the owner: the owner_kind word of the ownership
	// derivation (the badge shows who answers — a human or the digital
	// employee of the case).
	if view.Ownership.State.Current != nil && view.Ownership.State.Current.OwnerKind == casesDomain.OwnerKindStaffy {
		panel.BadgeRole = "цифровой сотрудник"
	} else {
		panel.BadgeRole = "участник-человек"
	}
	// The propose targets: the words ABOVE the active mode on the raise
	// ladder (the honest form — the machine refuses the rest anyway); the
	// exit targets: the words BELOW the active mode.
	if mode.Active != nil {
		for _, word := range casesDomain.ModeOrder {
			if casesDomain.ProposeTargetAllowed(word, mode.Active.Mode) {
				panel.ProposeTargets = append(panel.ProposeTargets, workOption{Value: word, Text: modeText(word)})
			}
			if casesDomain.ExitTargetAllowed(word, mode.Active.Mode) {
				panel.ExitTargets = append(panel.ExitTargets, workOption{Value: word, Text: modeText(word)})
			}
		}
	} else {
		panel.ProposeTargets = panel.Ladder
	}
	for _, term := range mode.Terms {
		row := modeTermRow{
			TermID:     term.TermID.String(),
			Version:    strconv.FormatInt(term.ModeVersion, 10),
			ModeText:   modeText(term.Mode),
			Scope:      term.Scope,
			OriginText: modeOriginText(term.Origin),
			Reason:     term.Reason,
			SignalText: modeSignalText(term.SignalSource),
			ActorID:    term.ActorID.String(),
			CreatedAt:  formatTime(term.CreatedAt),
		}
		if term.WorkItemID.Valid() {
			row.ItemText = term.WorkItemID.String()
		}
		if term.ExitAuthority != "" {
			row.Reason = term.Reason + " (полномочие: " + term.ExitAuthority + ")"
		}
		panel.TermRows = append(panel.TermRows, row)
	}
	for _, context := range mode.ItemContexts {
		panel.ItemContextRows = append(panel.ItemContextRows, modeContextRow{
			WorkItemID: context.WorkItemID.String(),
			ModeText:   modeText(context.Term.Mode),
			Version:    strconv.FormatInt(context.Term.ModeVersion, 10),
			Reason:     context.Term.Reason,
			SignalText: modeSignalText(context.Term.SignalSource),
			CreatedAt:  formatTime(context.Term.CreatedAt),
		})
	}
	for _, card := range mode.Disclosures {
		row := disclosureRowView{
			DisclosureID: card.Record.DisclosureID.String(),
			No:           strconv.FormatInt(card.Record.DisclosureNo, 10),
			Channel:      card.Record.Channel,
			Text:         card.Record.Text,
			Status:       card.Status,
			StatusText:   deliveryStatusText(card.Status),
			CreatedAt:    formatTime(card.Record.CreatedAt),
		}
		if card.Record.ParentRef.Valid() {
			row.ParentRef = card.Record.ParentRef.String()
		}
		panel.DisclosureRows = append(panel.DisclosureRows, row)
	}
	return panel
}

// policyPanelView is the Progressive Intake / Policy panel projection:
// the fixed rule words, the current anchor words of the evaluation form,
// the per-Work-Item sections (the proposals with the proposer word and
// the candidate the operator sees, the CURRENT outcome with the class
// words, the mandatory facts with the states, the next steps, the
// suppressed questions, the outcome history) and the submit form options.
type policyPanelView struct {
	PanelText        string
	RuleVersion      string
	IntentWords      []workOption
	CommercialText   string
	ClassWords       string
	FactsDigest      string
	OwnerEpoch       string
	ModeVersion      string
	CanManage        bool
	Items            []policyItemView
	HasItems         bool
	ConversationOpts []workOption
	ActiveWIOptions  []workOption
	EventOptions     []workOption
	ProposerOptions  []workOption
	ProposerDefault  string
}

// policyItemView is one Work Item section of the policy panel: the
// proposals (the claims audit), the outcomes history and the CURRENT
// outcome (the maximal number) with its honest words.
type policyItemView struct {
	WorkItemID    string
	WorkItemTitle string
	Version       string
	HasProposals  bool
	ProposalRows  []policyProposalRow
	HasOutcomes   bool
	OutcomeRows   []policyOutcomeRow
	Current       *policyOutcomeRow
	EvaluateOpts  []workOption
}

// policyProposalRow is one proposal row: the intent claim, the proposer
// word, the candidate answer (the operator sees the text), the missing
// and the extracted claim words and the submission snapshot.
type policyProposalRow struct {
	ProposalID    string
	Intent        string
	Proposer      string
	ProposerText  string
	Candidate     string
	MissingText   string
	ExtractedText string
	TurnRef       string
	OwnerEpoch    string
	ModeVersion   string
	CreatedAt     string
}

// policyOutcomeRow is one outcome row: the class word, the reason, the
// rule version, the intent snapshot, the mandatory facts with the states,
// the next steps, the reused facts with the provenance signature, the
// suppressed questions and the decision digest.
type policyOutcomeRow struct {
	OutcomeID      string
	No             string
	Class          string
	ClassText      string
	Reason         string
	RuleVersion    string
	Intent         string
	Mandatory      []policyMandatoryRow
	NextSteps      []string
	Reused         []string
	ReusedSigs     []string
	Suppressed     []string
	PermittedText  string
	DecisionDigest string
	CreatedAt      string
}

// policyMandatoryRow is one mandatory fact of the outcome plan: the
// material key, the source word and the state (reused with the
// provenance signature or ask — the question).
type policyMandatoryRow struct {
	MaterialKey string
	Source      string
	State       string
	StateText   string
}

// policyClassText renders the outcome class word honestly.
func policyClassText(class string) string {
	switch class {
	case casesDomain.OutcomeCommercialAnswer:
		return "commercial_answer (разрешённый коммерческий ответ)"
	case casesDomain.OutcomeInformationalAnswer:
		return "informational_answer (информационный ответ)"
	case casesDomain.OutcomeClarify:
		return "clarify (уточнение — только вопросы)"
	case casesDomain.OutcomeModeBlocked:
		return "mode_blocked (режим блокирует коммерческий путь)"
	case casesDomain.OutcomeFailClosed:
		return "fail_closed (закрыт безопасно — неизвестное правило)"
	}
	return class
}

// policyReasonText renders the outcome reason word.
func policyReasonText(reason string) string {
	switch reason {
	case casesDomain.ReasonOkCommercial:
		return "ok_commercial (коммерческий путь открыт)"
	case casesDomain.ReasonOkInformational:
		return "ok_informational (информационный путь)"
	case casesDomain.ReasonMandatoryMissing:
		return "mandatory_missing (обязательный факт отсутствует)"
	case casesDomain.ReasonNoAnswerBasis:
		return "no_answer_basis (нет базиса ответа)"
	case casesDomain.ReasonModeBlocksCommercial:
		return "mode_blocks_commercial_path (режим решает)"
	case casesDomain.ReasonUnknownIntent:
		return "unknown_intent (намерение вне словаря)"
	}
	return reason
}

// policyProposerText renders the proposer word.
func policyProposerText(proposer string) string {
	switch proposer {
	case casesDomain.ProposerSyntheticPort:
		return "synthetic_port (порт-провод)"
	case casesDomain.ProposerOperator:
		return "operator (оператор)"
	}
	return proposer
}

// policyMandatoryStateText renders the mandatory fact state word: reused
// (the confirmed value answers it) or ask (the question remains).
func policyMandatoryStateText(state string) string {
	switch state {
	case casesDomain.MandatoryStateReused:
		return "переиспользован (подтверждённое значение)"
	case casesDomain.MandatoryStateAsk:
		return "вопрос остаётся"
	}
	return state
}

// policyNextStepText renders one next step word with the material key.
func policyNextStepText(step casesDomain.NextStep) string {
	switch step.Step {
	case casesDomain.StepAskFact:
		return "спросить факт: " + step.MaterialKey
	case casesDomain.StepAwaitConfirmation:
		return "дождаться подтверждения"
	case casesDomain.StepHumanHandoff:
		return "передать человеку"
	}
	return step.Step
}

// preparePolicyPanel flattens the policy section into the template
// projection: the fixed rule words and the current anchors of the
// evaluation form, the per-Work-Item sections with the proposals, the
// outcomes and the CURRENT outcome (the aria-live region announces the
// outcome change without a focus move — AC-12 edge of the panel).
func preparePolicyPanel(view casesAPI.CaseView, conversationOpts, activeWIOptions, eventOptions []workOption) policyPanelView {
	policy := view.Policy
	panel := policyPanelView{
		PanelText:      policy.PanelText,
		RuleVersion:    policy.RuleVersion,
		FactsDigest:    policy.FactsDigest,
		OwnerEpoch:     strconv.FormatInt(policy.OwnerEpoch, 10),
		ModeVersion:    strconv.FormatInt(policy.ModeVersion, 10),
		CanManage:      view.CanManageWork,
		Items:          make([]policyItemView, 0, len(policy.Items)),
		HasItems:       len(policy.Items) > 0,
		CommercialText: "Коммерческие намерения: " + strings.Join(policy.Commercial, ", ") + " — продажный путь открыт только исходом commercial_answer.",
	}
	for _, word := range policy.IntentWords {
		panel.IntentWords = append(panel.IntentWords, workOption{Value: word, Text: word})
	}
	panel.ClassWords = strings.Join(policy.ClassWords, ", ")
	for _, word := range casesDomain.ProposalProposers {
		panel.ProposerOptions = append(panel.ProposerOptions, workOption{Value: word, Text: policyProposerText(word)})
	}
	panel.ProposerDefault = casesDomain.ProposerSyntheticPort
	// The form options of the proposal submission (the shared case view
	// options — the conversations, the work items and the accepted
	// events of the case).
	panel.ConversationOpts = append(panel.ConversationOpts, conversationOpts...)
	panel.ActiveWIOptions = append(panel.ActiveWIOptions, activeWIOptions...)
	panel.EventOptions = append(panel.EventOptions, eventOptions...)
	// The Work Item versions for the evaluation prefill.
	versions := map[string]string{}
	titles := map[string]string{}
	for _, item := range view.WorkItems {
		versions[item.WorkItemID.String()] = strconv.FormatInt(item.Version, 10)
		titles[item.WorkItemID.String()] = item.Title
	}
	for _, section := range policy.Items {
		row := policyItemView{
			WorkItemID:    section.WorkItemID.String(),
			WorkItemTitle: titles[section.WorkItemID.String()],
			Version:       versions[section.WorkItemID.String()],
			HasProposals:  len(section.Proposals) > 0,
			ProposalRows:  make([]policyProposalRow, 0, len(section.Proposals)),
			HasOutcomes:   len(section.Outcomes) > 0,
			OutcomeRows:   make([]policyOutcomeRow, 0, len(section.Outcomes)),
			EvaluateOpts:  make([]workOption, 0, len(section.Proposals)),
		}
		for _, proposal := range section.Proposals {
			row.ProposalRows = append(row.ProposalRows, policyProposalRow{
				ProposalID:    proposal.ProposalID.String(),
				Intent:        proposal.Intent,
				Proposer:      proposal.Proposer,
				ProposerText:  policyProposerText(proposal.Proposer),
				Candidate:     proposal.CandidateAnswer,
				MissingText:   policyMissingText(proposal.MissingFacts),
				ExtractedText: policyExtractedText(proposal.ExtractedFacts),
				TurnRef:       proposal.TurnRef.String(),
				OwnerEpoch:    strconv.FormatInt(proposal.OwnerEpoch, 10),
				ModeVersion:   strconv.FormatInt(proposal.ModeVersion, 10),
				CreatedAt:     formatTime(proposal.CreatedAt),
			})
			row.EvaluateOpts = append(row.EvaluateOpts, workOption{Value: proposal.ProposalID.String(), Text: proposal.Intent + " (" + formatTime(proposal.CreatedAt) + ")"})
		}
		for _, outcome := range section.Outcomes {
			outcomeRow := preparePolicyOutcomeRow(outcome)
			row.OutcomeRows = append(row.OutcomeRows, outcomeRow)
		}
		if section.Current != nil {
			current := preparePolicyOutcomeRow(*section.Current)
			row.Current = &current
		}
		panel.Items = append(panel.Items, row)
	}
	return panel
}

// preparePolicyOutcomeRow flattens one outcome into the row projection.
func preparePolicyOutcomeRow(outcome casesDomain.PolicyOutcome) policyOutcomeRow {
	row := policyOutcomeRow{
		OutcomeID:      outcome.OutcomeID.String(),
		No:             strconv.FormatInt(outcome.OutcomeNo, 10),
		Class:          outcome.OutcomeClass,
		ClassText:      policyClassText(outcome.OutcomeClass),
		Reason:         policyReasonText(outcome.Reason),
		RuleVersion:    outcome.RuleVersion,
		Intent:         outcome.Intent,
		Mandatory:      make([]policyMandatoryRow, 0, len(outcome.MandatoryFacts)),
		NextSteps:      make([]string, 0, len(outcome.NextSteps)),
		Reused:         make([]string, 0, len(outcome.ReusedFacts)),
		ReusedSigs:     make([]string, 0, len(outcome.ReusedFacts)),
		Suppressed:     make([]string, 0, len(outcome.SuppressedQuestions)),
		PermittedText:  "Разрешённый путь: " + casesDomain.OutcomePermittedPath(outcome.OutcomeClass),
		DecisionDigest: outcome.DecisionDigest,
		CreatedAt:      formatTime(outcome.CreatedAt),
	}
	for _, fact := range outcome.MandatoryFacts {
		row.Mandatory = append(row.Mandatory, policyMandatoryRow{
			MaterialKey: fact.MaterialKey, Source: fact.Source,
			State: fact.State, StateText: policyMandatoryStateText(fact.State),
		})
	}
	for _, step := range outcome.NextSteps {
		row.NextSteps = append(row.NextSteps, policyNextStepText(step))
	}
	for _, reused := range outcome.ReusedFacts {
		row.Reused = append(row.Reused, reused.MaterialKey)
		row.ReusedSigs = append(row.ReusedSigs, reused.ValueDigest+" (факт "+reused.FactID.String()+", версия "+strconv.FormatInt(reused.VersionNo, 10)+")")
	}
	for _, suppressed := range outcome.SuppressedQuestions {
		row.Suppressed = append(row.Suppressed, suppressed.MaterialKey+" (источник: "+suppressed.Source+")")
	}
	return row
}

// policyMissingText renders the missing-fact claim words of one proposal.
func policyMissingText(missing []casesDomain.MissingFact) string {
	if len(missing) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(missing))
	for _, fact := range missing {
		parts = append(parts, fact.MaterialKey+" ("+fact.Reason+")")
	}
	return strings.Join(parts, ", ")
}

// policyExtractedText renders the extracted-fact claim words of one
// proposal (the AUDIT claims — the facts are never written from them).
func policyExtractedText(extracted []casesDomain.ExtractedFact) string {
	if len(extracted) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(extracted))
	for _, fact := range extracted {
		parts = append(parts, fact.MaterialKey+" = "+fact.ValueDigest+" (событие "+fact.SourceEventID.String()+")")
	}
	return strings.Join(parts, ", ")
}
