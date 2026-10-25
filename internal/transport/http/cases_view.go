package httptransport

import (
	"bytes"
	"io"
	"net/http"
	"sort"
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
	// The Story 4.2 vehicle identification panel (the case-level
	// section before the Work Control Cards).
	Vehicle vehiclePanelView
	// The Story 4.3 part request panel (the case-level section right
	// after the vehicle identification panel — the read-time request
	// derivation under the pack part request schema).
	PartRequest partPanelView
	Fitment     fitmentPanelView
	Offers      offersPanelView
	// The Story 4.6 order panel (the case-level section after the offers —
	// the Order Commitment surface of the existing machines).
	Order orderPanelView
	// The Story 4.7 verification panel (the case-level section after the
	// order — the Delegated Task over the committed receipt).
	OrderVerification orderVerificationPanelView
	// The Story 4.8 correction panel (the case-level section after the
	// verification — the Correction Work Item over the blocked decision).
	OrderCorrection orderCorrectionPanelView
	// The Story 4.8 success flags of the PRG forms (the honest status
	// words of the four commands — the reread word of the corrected
	// closure and the cancellation recommendation word ride the panel
	// constants).
	CorrectionOpened     bool
	CorrectionReread     bool
	CorrectionCeased     bool
	CorrectionReassigned bool
	// The Story 4.7 success flags of the PRG forms (the honest status
	// words of the five commands — the manual-contour word of the
	// approved decision rides the panel constant).
	VerificationAssigned     bool
	VerificationAcknowledged bool
	VerificationApproved     bool
	VerificationBlocked      bool
	VerificationReassigned   bool
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
	// The Story 2.7 section: the Owning System panel words (the field
	// cards live on the Work Control Cards of the work items).
	ObservationsPanelText string
	ObservationMatrixText string
	OwningFieldOptions    []workOption
	// The Story 2.8 section: the Objection panel words (the objection
	// cards live on the Work Control Cards of the work items; the record
	// form selects carry the fixed dictionaries, never invented options).
	ObjectionsPanelText    string
	ObjectionVersionText   string
	ObjectionBudgetText    string
	ObjectionTypeOptions   []workOption
	ObjectionSignalOptions []workOption
	ObjectionStepOptions   []workOption
	// The Story 2.9 section: the Commitment panel words (the link options
	// of the sales reply form — the offered current versions of the case
	// Work Items).
	CommitmentsPanelText  string
	CommitmentLinkOptions []workOption

	// The Story 2.10 section: the continuation panel words (the link
	// options of the sales reply form — the VALID links of the case; the
	// purpose options of the issue form — the fixed v1 dictionary) and
	// the revoked announcement flag of the PRG landing.
	ContinuationsPanelText     string
	ContinuationLinkOptions    []workOption
	ContinuationPurposeOptions []workOption
	ContinuationRevoked        bool
	// The Story 2.11 section: the accommodation panel words (the CURRENT
	// set, the append-only revision history, the key options of the record
	// form — the dictionaries are fixed, never invented options), the
	// accuracy qualifier options of the sales reply form (the active
	// alternative words of the CURRENT outcomes: the form hint — the
	// server gate resolves the exact set of the selected Work Item) and
	// the continuation issue form hint (the honest no_web/no_voice words —
	// the operator decides the surface, the machine never blocks).
	AccommodationsPanelText    string
	AccommodationCurrentRows   []accommodationStateRow
	AccommodationHistoryRows   []accommodationRecordRow
	HasAccommodations          bool
	AccommodationKeyOptions    []workOption
	AccommodationRecordsAnchor string
	AccuracyQualifierOptions   []workOption
	ContinuationSurfaceHint    string
	// The Story 2.12 section: the route degradation panel words (the
	// CURRENT set with the honest cause texts, the append-only revision
	// history including the recovery rows, the dictionaries of the record
	// form — the connection options of the case's conversations and the
	// channel dictionary), the delivery states of the reply rows, the
	// update promise options of the reply form (the approved policy
	// dictionary — the select without a silent default) and the evidence
	// kind options of the delivery observation form.
	RouteDegradationsPanelText   string
	RouteDegradationCurrentRows  []routeDegradationStateRow
	RouteDegradationHistoryRows  []routeDegradationRecordRow
	HasRouteDegradations         bool
	DegradationCauseOptions      []workOption
	DegradationConnectionOptions []workOption
	DegradationChannelOptions    []workOption
	DegradationRecordsAnchor     string
	UpdatePromiseOptions         []workOption
	EvidenceKindOptions          []workOption
	ReplyEvidenceOptions         []workOption
	// The Story 2.13 section: the human handoff panel words (the current
	// lifecycle state line with the honest texts, the request rows with
	// the derived states and the event histories, the card of the
	// assignee, the dictionaries of the forms — the target roles, the
	// deadlines, the return results — and the anchors of the double-submit
	// protection).
	HandoffsPanelText      string
	HandoffCurrentText     string
	HandoffClientText      string
	HasHandoffs            bool
	HandoffRequestRows     []handoffRequestRow
	HandoffCardForActor    bool
	HandoffCardStateText   string
	HandoffCardDeadline    string
	HandoffCardAssignee    string
	HandoffRoleWords       []workOption
	HandoffDeadlineOptions []workOption
	// HandoffReasonOptions is the Story 4.3 dictionary of the operator
	// boundary reason words of the request_human form (the SYSTEM
	// dictionary — the three words of the part request boundaries; the
	// empty choice stays the unconditional client ask of 2.13).
	HandoffReasonOptions  []workOption
	HandoffResultOptions  []workOption
	HandoffRequestsAnchor string
	HandoffEventsAnchor   string
	// The Story 2.16 section: the truthful waiting updates — the
	// «Ожидание человека» block of the last round (the client
	// waiting status in the three distinctions of AC-1, the state of
	// the last promise with the honest missed next step — the
	// dictionary texts only, the persona NEVER enters the client
	// line) and the dictionaries of the promise form; the block
	// renders ONLY with the promise material — the control branch
	// renders the page byte-for-byte unchanged.
	Waiting                waitingUpdatePanelView
	PromiseDeadlineOptions []workOption
	UpdatePromised         bool
	// The Story 2.14 section: the decision context panel of the assigned
	// operator («Контекст передачи» — the twelve read-time blocks with
	// the honest marking of the AI summary, the staleness badge and the
	// recommendation projection; the section renders ONLY when the
	// material exists — the open handoff or the recorded summary, the
	// honest absence of both renders the unchanged page byte-for-byte).
	DecisionContext decisionContextPanelView
	// The Story 2.15 flag: the PRG success of the «Исправить» form (the
	// 303 lands on the #decision-context anchor — the honest status line
	// of the append-only record: the pour is NEVER an automatic
	// confirmation, the matrix field disputes without a rewrite).
	CorrectionRecorded bool
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
	// AccommodationWords is the current preference words of the Story 2.11
	// transfer card: the receiving operator reads the client's
	// communication preferences BEFORE the ack (AC-1 «передаются каждому
	// последующему Response Owner»).
	AccommodationWords string
	// DegradationWords is the current degraded route words of the Story
	// 2.12 transfer card: the receiving operator reads the operational
	// route state BEFORE the ack (the preferences 2.11 precedent).
	DegradationWords string
	// The Story 2.12 material: the derived honest delivery states keyed by
	// the reply id (the «Доставка» column) and the reply options of the
	// evidence observation form.
	deliveries           map[string]casesAPI.ReplyDeliveryView
	ReplyEvidenceOptions []workOption
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
	// QualifierText is the accuracy qualifier word of the Story 2.11 reply
	// (the honest limitation of the outcome's active set; empty when the
	// outcome carried none).
	QualifierText string
	// DeliveryText is the derived honest delivery state of the Story 2.12
	// reply (prepared / unknown / confirmed / failed — dispatched without
	// evidence is the UNKNOWN, never the delivered); DeliveryDetail carries
	// the last evidence anchor (the kind word with the time — the timeline
	// of AC-9); PromiseText is the update promise word of the degraded
	// route reply (empty on the healthy route — the field has exactly one
	// meaning).
	DeliveryText   string
	DeliveryDetail string
	PromiseText    string
}

// routeDegradationStateRow is one route slot of the CURRENT degraded set
// of the panel.
type routeDegradationStateRow struct {
	ConnectionID string
	Channel      string
	Cause        string
	CauseText    string
	Revision     string
}

// routeDegradationRecordRow is one append-only revision row of the honest
// history (including the recovery rows).
type routeDegradationRecordRow struct {
	ConnectionID string
	Channel      string
	Cause        string
	CauseText    string
	Revision     string
	Cleared      bool
	Note         string
	ActorID      string
	RecordedAt   string
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

// waitingUpdatePanelView is the «Ожидание человека» block of the case
// page: the honest client waiting status of the last round (the
// dictionary word with the honest text — three distinctions plus the
// honest ending words), the state of the last promise (the word, the
// honest promised time — the empty text of the unknown word, the
// derived state word with the honest texts) and the honest missed next
// step (the update is the EXTERNAL producer — the human sends it by
// the existing reply path). The block renders ONLY with the promise
// material: the zero view contributes NOTHING (the control branch of
// the slice renders the page byte-for-byte unchanged).
type waitingUpdatePanelView struct {
	HasPromise         bool
	ClientStatus       string
	ClientStatusText   string
	PromiseNo          string
	DeadlineWord       string
	DeadlineText       string
	HasDeadline        bool
	PromisedAtText     string
	State              string
	StateText          string
	PromiseLineText    string
	MissedNextStepText string
	DeadlineOptions    []workOption
	PromiseAnchor      string
}

// handoffRequestRow is one request row of the «Запрос человека» panel:
// the derived lifecycle state with the honest texts, the client status
// word (назначение ≠ принятие — two words), the current round material
// (the assignee, the backup, the deadline) and the append-only event
// history of the request.
type handoffRequestRow struct {
	RequestNo    string
	ReasonText   string
	State        string
	StateText    string
	ClientText   string
	ActiveText   string
	RolesText    string
	DeadlineText string
	TurnRef      string
	Assignee     string
	Backup       string
	DeadlineAt   string
	CreatedAt    string
	HasRound     bool
	HasEvents    bool
	Events       []handoffEventRow
}

// handoffEventRow is one lifecycle event row of the request history.
type handoffEventRow struct {
	KindText     string
	MaterialText string
	ActorID      string
	CreatedAt    string
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

func prepareCaseView(r *http.Request, view casesAPI.CaseView, facts casesAPI.FactsView, flash string) caseDetailPageView {
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
	// The Story 4.2 section: the vehicle identification panel (the
	// read-time profile of the pack schema — the full VIN renders
	// here (the operator review surface) and on the protected client
	// form only; the mask word carries the honest boundary).
	out.Vehicle = prepareVehiclePanel(view)
	// The Story 4.3 section: the part request panel (the case-level
	// section right after the vehicle identification panel).
	out.PartRequest = preparePartPanel(view)
	// The Story 4.4 section: the applicability panel (the case-level
	// section right after the part request panel).
	out.Fitment = prepareFitmentPanel(view)
	// The Story 4.5 section: the offers and comparison panel (the
	// case-level section right after the applicability panel).
	out.Offers = prepareOffersPanel(view)
	// The Story 4.3 form dictionary: the operator boundary reason words
	// of the request_human form (the SYSTEM dictionary of 2.13/4.3 —
	// never the pack store material; empty stays the unconditional
	// client ask).
	for _, reason := range casesDomain.HandoffReasons {
		if reason == casesDomain.HandoffReasonClientRequest {
			continue
		}
		out.HandoffReasonOptions = append(out.HandoffReasonOptions, workOption{Value: reason, Text: casesDomain.HandoffReasonText(reason)})
	}
	// The Story 2.7 section: the Owning System panel words — the field
	// cards live on the Work Control Cards (the read form select carries
	// the fixed matrix dictionary, never an invented option).
	out.ObservationsPanelText = view.Observations.PanelText
	out.ObservationMatrixText = view.Observations.MatrixVersion
	for _, field := range view.Observations.FieldWords {
		entry, _ := casesDomain.AuthorityEntryOf(field)
		out.OwningFieldOptions = append(out.OwningFieldOptions, workOption{
			Value: field,
			Text:  field + " → " + entry.OwningSystem + " · " + entry.Projection + " (свежесть " + strconv.Itoa(entry.FreshnessSeconds) + " с)",
		})
	}

	// The Story 2.8 section: the Objection panel words — the fixed
	// dictionaries of the record form and the reply step select (the
	// type, the signal and the step words render honestly; the options
	// never carry invented words).
	out.ObjectionsPanelText = view.Objections.PanelText
	out.ObjectionVersionText = view.Objections.PlaybookVersion
	out.ObjectionBudgetText = "бюджет убеждения — " + strconv.Itoa(view.Objections.Threshold) + " отказ(а) на элемент, lifetime"
	for _, word := range view.Objections.TypeWords {
		out.ObjectionTypeOptions = append(out.ObjectionTypeOptions, workOption{Value: word, Text: objectionTypeText(word)})
	}
	for _, word := range view.Objections.SignalWords {
		out.ObjectionSignalOptions = append(out.ObjectionSignalOptions, workOption{Value: word, Text: objectionSignalText(word)})
	}
	for _, word := range view.Objections.StepWords {
		out.ObjectionStepOptions = append(out.ObjectionStepOptions, workOption{Value: word, Text: objectionStepText(word)})
	}

	// The Story 2.10 section: the continuation panel words, the link
	// options of the sales reply form (the VALID undelivered links of the
	// case — the FR-16 binding is one reply of one link) and the purpose
	// options of the issue form (the fixed v1 dictionary, never invented
	// options).
	out.ContinuationsPanelText = view.Continuations.PanelText
	out.ContinuationRevoked = r.URL.Query().Get("continuation_revoked") == "1"
	// The Story 2.15 PRG success flag of the «Исправить» form (the
	// honest status line of the section — the flag rides the URL, the
	// error rides the flash cookie).
	out.CorrectionRecorded = r.URL.Query().Get("correction_recorded") == "1"
	out.VerificationAssigned = r.URL.Query().Get("verification_assigned") == "1"
	out.VerificationAcknowledged = r.URL.Query().Get("verification_acknowledged") == "1"
	out.VerificationApproved = r.URL.Query().Get("verification_approved") == "1"
	out.VerificationBlocked = r.URL.Query().Get("verification_blocked") == "1"
	out.VerificationReassigned = r.URL.Query().Get("verification_reassigned") == "1"
	out.CorrectionOpened = r.URL.Query().Get("correction_opened") == "1"
	out.CorrectionReread = r.URL.Query().Get("correction_reread") == "1"
	out.CorrectionCeased = r.URL.Query().Get("correction_ceased") == "1"
	out.CorrectionReassigned = r.URL.Query().Get("correction_reassigned") == "1"
	// The Story 2.16 PRG success flag of the «Обещать обновление» form
	// (the honest status line of the waiting section — the flag rides the
	// URL, the error rides the flash cookie).
	out.UpdatePromised = r.URL.Query().Get("update_promised") == "1"
	for _, word := range casesDomain.ContinuationPurposes {
		out.ContinuationPurposeOptions = append(out.ContinuationPurposeOptions, workOption{Value: word, Text: continuationPurposeText(word)})
	}
	for _, link := range view.Continuations.Links {
		if link.State == casesDomain.ContinuationStateValid && !link.Delivered {
			out.ContinuationLinkOptions = append(out.ContinuationLinkOptions, workOption{
				Value: link.LinkID.String(),
				Text:  "ссылка " + link.LinkID.String() + " · " + continuationLinkStateText(link.State) + " · " + link.Question,
			})
		}
	}
	// The Story 2.9 section: the Commitment panel words and the presentation
	// link options of the sales reply form (the offered CURRENT versions of
	// the case Work Items — the draft never presents; the value carries the
	// "<id>:<version>" pair of the link).
	out.CommitmentsPanelText = view.Commitments.PanelText
	for _, section := range view.Commitments.Items {
		for _, card := range section.Versions {
			if card.Current && card.Version.Status == casesDomain.CommitmentStatusOffered {
				out.CommitmentLinkOptions = append(out.CommitmentLinkOptions, workOption{
					Value: card.Version.CommitmentID.String() + ":" + strconv.FormatInt(card.Version.VersionNo, 10),
					Text:  "обязательство " + card.Version.CommitmentID.String() + " · версия №" + strconv.FormatInt(card.Version.VersionNo, 10) + " (offered)",
				})
			}
		}
	}

	// The Story 2.11 section: the accommodation panel — the CURRENT set
	// (the read-time derivation: the cleared keys stay visible with the
	// honest mark), the append-only revision history, the key options of
	// the record form (the fixed dictionary) and the accuracy qualifier
	// options of the sales reply form (the active words of the CURRENT
	// outcomes — the union over the case items; the server gate resolves
	// the exact set of the selected item, the form shows the hint).
	out.AccommodationsPanelText = view.Accommodations.PanelText
	out.HasAccommodations = len(view.Accommodations.History) > 0
	out.AccommodationRecordsAnchor = strconv.Itoa(view.Accommodations.RecordsCount)
	for _, state := range view.Accommodations.Current {
		out.AccommodationCurrentRows = append(out.AccommodationCurrentRows, accommodationStateRow{
			Key: state.Key, KeyText: accommodationKeyText(state.Key),
			Revision: strconv.FormatInt(state.Revision, 10), Cleared: state.Cleared, Value: state.Value,
		})
	}
	for _, record := range view.Accommodations.History {
		out.AccommodationHistoryRows = append(out.AccommodationHistoryRows, accommodationRecordRow{
			Key: record.Key, KeyText: accommodationKeyText(record.Key),
			Revision: strconv.FormatInt(record.Revision, 10), Value: record.Value, Cleared: record.Cleared,
			Note: record.Note, ActorID: record.ActorID.String(), RecordedAt: formatTime(record.RecordedAt),
		})
	}
	for _, word := range view.Accommodations.KeyWords {
		out.AccommodationKeyOptions = append(out.AccommodationKeyOptions, workOption{Value: word, Text: accommodationKeyText(word)})
	}
	qualifierWords := map[string]bool{}
	for _, section := range view.Policy.Items {
		if section.Current != nil {
			for _, word := range section.Current.ActiveAlternatives {
				qualifierWords[word] = true
			}
		}
	}
	for _, word := range casesDomain.AlternativePathWords {
		if qualifierWords[word] {
			out.AccuracyQualifierOptions = append(out.AccuracyQualifierOptions, workOption{Value: word, Text: accuracyQualifierText(word)})
		}
	}
	// The continuation issue form hint: the honest no_web/no_voice words of
	// the current set (the link IS a web page — the operator decides the
	// surface; the machine never blocks without a capability registry).
	surfaceHints := []string{}
	for _, state := range view.Accommodations.Current {
		if !state.Cleared && (state.Key == casesDomain.AccommodationNoWeb || state.Key == casesDomain.AccommodationNoVoice) {
			surfaceHints = append(surfaceHints, accommodationKeyText(state.Key))
		}
	}
	if len(surfaceHints) > 0 {
		out.ContinuationSurfaceHint = "Записаны предпочтения клиента: " + strings.Join(surfaceHints, "; ") + ". Ссылка продолжения — web-страница: сверьте, подходит ли эта поверхность клиенту (машина не блокирует — решение за оператором)."
	}

	// The Story 2.12 section: the route degradation panel — the CURRENT set
	// (the read-time derivation with the honest cause texts), the
	// append-only revision history including the recovery rows, the
	// dictionaries of the record form (the connections of the case's
	// conversations, the channel dictionary) and the reply form promise
	// options (the approved policy dictionary).
	out.RouteDegradationsPanelText = view.RouteDegradations.PanelText
	out.HasRouteDegradations = len(view.RouteDegradations.History) > 0
	out.DegradationRecordsAnchor = strconv.Itoa(view.RouteDegradations.RecordsCount)
	for _, state := range view.RouteDegradations.Current {
		out.RouteDegradationCurrentRows = append(out.RouteDegradationCurrentRows, routeDegradationStateRow{
			ConnectionID: state.ConnectionID.String(), Channel: state.Channel,
			Cause: state.Cause, CauseText: state.CauseText,
			Revision: strconv.FormatInt(state.Revision, 10),
		})
	}
	for _, record := range view.RouteDegradations.History {
		out.RouteDegradationHistoryRows = append(out.RouteDegradationHistoryRows, routeDegradationRecordRow{
			ConnectionID: record.ConnectionID.String(), Channel: record.Channel,
			Cause: record.Cause, CauseText: degradationCauseText(record.Cause),
			Revision: strconv.FormatInt(record.Revision, 10),
			Cleared:  record.Cleared, Note: record.Note,
			ActorID: record.ActorID.String(), RecordedAt: formatTime(record.RecordedAt),
		})
	}
	for _, word := range view.RouteDegradations.CauseWords {
		out.DegradationCauseOptions = append(out.DegradationCauseOptions, workOption{Value: word, Text: degradationCauseText(word)})
	}
	connectionsSeen := map[string]bool{}
	for _, conversation := range view.Conversations {
		if !connectionsSeen[conversation.ConnectionID.String()] {
			connectionsSeen[conversation.ConnectionID.String()] = true
			out.DegradationConnectionOptions = append(out.DegradationConnectionOptions, workOption{
				Value: conversation.ConnectionID.String(),
				Text:  "подключение " + conversation.ConnectionID.String() + " (" + conversation.Provider + ")",
			})
		}
	}
	for _, word := range view.RouteDegradations.ChannelWords {
		out.DegradationChannelOptions = append(out.DegradationChannelOptions, workOption{Value: word, Text: channelWordText(word)})
	}
	for _, word := range casesDomain.UpdatePromiseWords {
		out.UpdatePromiseOptions = append(out.UpdatePromiseOptions, workOption{Value: word, Text: updatePromiseText(word)})
	}
	for _, word := range casesDomain.DeliveryEvidenceWords {
		out.EvidenceKindOptions = append(out.EvidenceKindOptions, workOption{Value: word, Text: evidenceKindText(word)})
	}

	// The Story 2.13 section: the human handoff panel — the current
	// lifecycle state line with the honest texts (the state word, the
	// client status word — назначение ≠ принятие are two words), the
	// request rows with the derived states and the event histories, the
	// card of the assignee (the acting principal IS the assignee of the
	// open assigned round), the dictionaries of the forms (the fixed
	// compile-time words — never the invented options) and the anchors of
	// the double-submit protection.
	out.HandoffsPanelText = view.Handoff.PanelText
	out.HasHandoffs = view.Handoff.RequestsCount > 0
	out.HandoffRequestsAnchor = strconv.Itoa(view.Handoff.RequestsCount)
	eventsCount := 0
	for _, request := range view.Handoff.Requests {
		eventsCount += len(request.Events)
	}
	out.HandoffEventsAnchor = strconv.Itoa(eventsCount)
	if view.Handoff.ClientStatus != "" {
		out.HandoffCurrentText = "Запрос человека: " + handoffStateWordText(view.Handoff)
		out.HandoffClientText = "Клиентский статус: " + view.Handoff.ClientStatusText + " (слово: " + view.Handoff.ClientStatus + ")"
	} else {
		out.HandoffCurrentText = "Запроса человека нет: безусловная просьба клиента не зарегистрирована."
		out.HandoffClientText = "Клиентский статус: специалист не запрашивался."
	}
	for _, request := range view.Handoff.Requests {
		row := handoffRequestRow{
			RequestNo:    strconv.FormatInt(request.RequestNo, 10),
			ReasonText:   request.ReasonText,
			State:        request.State,
			StateText:    request.StateText,
			ClientText:   request.ClientStatusText,
			ActiveText:   "закрыт",
			RolesText:    strings.Join(request.TargetRoles, ", "),
			DeadlineText: request.AckDeadlineText,
			TurnRef:      request.TurnRef.String(),
			Assignee:     request.Assignee.String(),
			Backup:       request.BackupOwner.String(),
			DeadlineAt:   formatTime(request.DeadlineAt),
			CreatedAt:    formatTime(request.CreatedAt),
			HasRound:     request.Assignee != id.UUID(""),
			HasEvents:    len(request.Events) > 0,
		}
		if request.Active {
			row.ActiveText = "открыт"
		}
		for _, event := range request.Events {
			material := ""
			switch event.Kind {
			case casesDomain.HandoffEventAssigned:
				material = "специалист " + event.Assignee.String() + ", резерв " + event.BackupOwner.String() + ", " + casesDomain.HandoffAckDeadlineText(event.AckDeadline)
			case casesDomain.HandoffEventAcknowledged:
				material = "новая эпоха владения " + strconv.FormatInt(event.ResultingEpoch, 10)
			case casesDomain.HandoffEventDeclined:
				material = "владелец не изменился"
			case casesDomain.HandoffEventCancelled:
				material = "якорь отмены — реплика " + event.TurnRef.String()
			case casesDomain.HandoffEventReturned:
				material = casesDomain.HandoffResultText(event.Result)
			}
			row.Events = append(row.Events, handoffEventRow{
				KindText:     event.KindText,
				MaterialText: material,
				ActorID:      event.ActorID.String(),
				CreatedAt:    formatTime(event.CreatedAt),
			})
		}
		out.HandoffRequestRows = append(out.HandoffRequestRows, row)
	}
	// The card of the assignee: the open assigned round the acting
	// principal answers (the confirm and the decline forms — the
	// assignment or the notification is never an acknowledgement).
	out.HandoffCardForActor = view.Handoff.AcknowledgeForActor
	if view.Handoff.AcknowledgeForActor {
		out.HandoffCardStateText = casesDomain.HandoffStateText(casesDomain.HandoffStateAssigned)
		out.HandoffCardDeadline = formatTime(view.Handoff.RoundDeadlineAt)
		out.HandoffCardAssignee = view.Handoff.RoundAssignee.String()
	}
	for _, word := range view.Handoff.RoleWords {
		out.HandoffRoleWords = append(out.HandoffRoleWords, workOption{Value: word, Text: handoffRoleText(word)})
	}
	for _, word := range view.Handoff.DeadlineWords {
		out.HandoffDeadlineOptions = append(out.HandoffDeadlineOptions, workOption{Value: word, Text: casesDomain.HandoffAckDeadlineText(word)})
	}
	for _, word := range view.Handoff.ResultWords {
		out.HandoffResultOptions = append(out.HandoffResultOptions, workOption{Value: word, Text: casesDomain.HandoffResultText(word)})
	}

	// The Story 2.16 section: the truthful waiting updates — the
	// «Ожидание человека» block of the last round (the client status
	// in the three distinctions of AC-1 and the state of the last
	// promise with the honest missed next step; the block carries the
	// dictionary texts ONLY — the persona NEVER enters the client
	// line, the anti-disclosure of AC-1) and the dictionaries of the
	// promise form (the fixed compile-time words — never the invented
	// options). The block renders ONLY with the promise material —
	// the zero view contributes NOTHING (the control branch renders
	// the page byte-for-byte unchanged).
	out.Waiting = waitingUpdatePanelView{
		HasPromise:         view.Waiting.HasPromise,
		ClientStatus:       view.Waiting.ClientStatus,
		ClientStatusText:   view.Waiting.ClientStatusText,
		PromiseNo:          strconv.FormatInt(view.Waiting.PromiseNo, 10),
		DeadlineWord:       view.Waiting.DeadlineWord,
		DeadlineText:       view.Waiting.DeadlineText,
		HasDeadline:        !view.Waiting.PromisedAt.IsZero(),
		PromisedAtText:     formatTime(view.Waiting.PromisedAt),
		State:              view.Waiting.State,
		StateText:          view.Waiting.StateText,
		PromiseLineText:    view.Waiting.PromiseLineText,
		MissedNextStepText: view.Waiting.MissedNextStepText,
		// The double-submit anchor of the promise form: the
		// monotonic promise number of the round (0 — no promises
		// yet; the number bumps with every new promise — the
		// double submit of the SAME material replays honestly,
		// the NEW promise carries the NEW anchor).
		PromiseAnchor: strconv.FormatInt(view.Waiting.PromiseNo, 10),
	}
	for _, word := range view.Waiting.DeadlineWords {
		out.PromiseDeadlineOptions = append(out.PromiseDeadlineOptions, workOption{Value: word, Text: casesDomain.UpdateDeadlineText(word)})
	}

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
	// The Story 2.7 section: the owning-system field cards of this
	// item (the matrix fields with the CURRENT observation, the
	// freshness state and the conflict badge) and the observation
	// anchor of the read form.
	ObsFields []observationFieldView
	ObsCount  string
	// The Story 2.8 section: the objection cards of this item (the
	// CURRENT objection with the verbatim spans, the persuasion budget
	// and the stopped word) and the objection anchor of the record form.
	ObjCurrent objectionCardView
	ObjCount   string
	// The Story 2.9 section: the commitment counts of this item (the
	// versions, the confirmations and the declines) with the review page
	// link (the full cards live on the Commitment Review page).
	CommitCount        string
	CommitConfirmCount string
	CommitDeclineCount string
	CommitReviewURL    string
	// The Story 2.10 section: the continuation link cards of this item
	// (the read-time state derivation, the FR-16 delivery word, the use
	// accounting against the limit) and the client event feed of the
	// links of this item.
	ContLinks []continuationLinkCardView
	ContUses  []continuationUseCardView
	// The form options of this card (empty when the state refuses them).
	AllowedTransitions []transitionOption
	MergeSources       []mergeSourceOption
}

// objectionCardView is the CURRENT objection card of the «Возражения и
// бюджет убеждения» panel of a Work Control Card: the record words (the
// dictionary type and signal with the honest Russian texts, the spans as
// the verbatim client words, the turn reference, the playbook version
// snapshot, the time), the lifetime persuasion budget over the fixed
// threshold and the stopped word with the allowed endings (stated is
// never distinguished from active by the color alone — the badge word
// carries the action).
type objectionCardView struct {
	HasCurrent       bool
	No               string
	TypeText         string
	SignalText       string
	Spans            []string
	TurnRef          string
	CreatedAt        string
	PlaybookVersion  string
	Refusals         string
	Threshold        string
	Stopped          bool
	StoppedText      string
	AcknowledgedText string
	NextStepsText    string
	LinkedCount      string
}

// continuationLinkCardView is one continuation link of the operator
// panel: the identifiers, the purpose words, the read-time state
// derivation (never a stored status), the FR-16 delivery word, the use
// accounting against the limit, the operator texts and the expiry. The
// token never appears — the hash is the only stored trace.
type continuationLinkCardView struct {
	LinkID         string
	PurposesText   string
	StateText      string
	StatusText     string
	UsesText       string
	Question       string
	ParticipantRef string
	ExpiresText    string
	Revocable      bool
	RevokeURL      string
}

// continuationUseCardView is one client action of the event feed (the
// accepted correction consumes the limit; the human and return events
// are free).
type continuationUseCardView struct {
	UseID       string
	LinkID      string
	KindText    string
	FactID      string
	FactVersion string
	SubmittedAt string
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

// observationFieldView is one material field card of the Owning System panel
// of a Work Control Card: the matrix row words, the CURRENT observation (the
// value text, the carrier words, both times), the read-time freshness state
// (the stale state carries the re-read word — never the color alone) and the
// conflict badge against the confirmed fact of the same material key.
type observationFieldView struct {
	MaterialKey      string
	Projection       string
	OwningSystemText string
	HasCurrent       bool
	No               string
	ValueText        string
	Status           string
	StatusText       string
	State            string
	StateText        string
	StaleText        string
	Reason           string
	ReasonText       string
	ExternalID       string
	ExternalVersion  string
	ObservedAt       string
	ReceivedAt       string
	FreshnessText    string
	Conflict         string
	ConflictText     string
}

// observationStatusText renders the status word honestly.
func observationStatusText(status string) string {
	switch status {
	case casesDomain.ObservationStatusOk:
		return "ok (значение прочитано)"
	case casesDomain.ObservationStatusFailed:
		return "failed (чтение не удалось)"
	case casesDomain.ObservationStatusUnknown:
		return "unknown (значение неизвестно)"
	}
	return status
}

// observationReasonText renders the read reason word.
func observationReasonText(reason string) string {
	switch reason {
	case casesDomain.ReadReasonProviderTimeout:
		return "provider_timeout (таймаут провайдера)"
	case casesDomain.ReadReasonMalformedResponse:
		return "malformed_response (ответ не разбирается)"
	case casesDomain.ReadReasonInsufficientScope:
		return "insufficient_scope (нет полномочия чтения)"
	case casesDomain.ReadReasonReadError:
		return "read_error (ошибка чтения)"
	}
	return reason
}

// observationStateText renders the freshness state word: the stale state
// carries the re-read action in the text (stale is NEVER distinguished from
// fresh by the color alone — AC-12).
func observationStateText(state string) string {
	switch state {
	case casesDomain.ObservationStateFresh:
		return "свежее (в окне матрицы)"
	case casesDomain.ObservationStateStale:
		return "устаревшее — " + casesDomain.StaleBadgeText
	case casesDomain.ObservationStateFailed:
		return "чтение не удалось — причина словарём"
	case casesDomain.ObservationStateUnknown:
		return "значение неизвестно — перечитайте"
	}
	return state
}

// observationConflictText renders the conflict badge word (the resolution
// stays with the human confirmation — never an automatic choice).
func observationConflictText(badge string) string {
	switch badge {
	case casesDomain.ConflictBadgeEqual:
		return "совпадает с подтверждённым фактом"
	case casesDomain.ConflictBadgeDiffers:
		return "расхождение с фактом — сверьте значения и подтвердите точную версию (авторазрешения нет)"
	}
	return ""
}

func workItemStateText(state string) string {
	switch state {
	case "untriaged":
		return "Запрос ещё не классифицирован"
	case "in_work":
		return "В работе"
	case "waiting":
		return "Ожидание возврата клиента"
	case "confirmed":
		return "Подтверждено (согласие клиента записано)"
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
// The Story 2.7 field cards of the Owning System panel join the same cards
// (the observations belong to the Work Item scope of the read command).
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
	// The Story 2.7 observation sections per Work Item (the field cards
	// of the matrix + the observation anchor of the read form).
	obsSections := map[id.UUID]casesAPI.ObservationWorkItemView{}
	for _, section := range view.Observations.Items {
		obsSections[section.WorkItemID] = section
	}
	// The Story 2.8 objection sections per Work Item (the CURRENT
	// objection card + the objection anchor of the record form).
	objSections := map[id.UUID]casesAPI.ObjectionWorkItemView{}
	for _, section := range view.Objections.Items {
		objSections[section.WorkItemID] = section
	}
	// The Story 2.9 commitment sections per Work Item (the counts and
	// the review page link; the full cards live on the review page).
	commitSections := map[id.UUID]casesAPI.CommitmentWorkItemView{}
	for _, section := range view.Commitments.Items {
		commitSections[section.WorkItemID] = section
	}
	for index, item := range view.WorkItems {
		card := workCardView{
			WorkItemID: item.WorkItemID.String(), IndexNo: strconv.Itoa(index + 1),
			TypeText: item.Type, TitleText: item.Title, State: item.State,
			StateText: workItemStateText(item.State), Version: strconv.FormatInt(item.Version, 10),
			KindText: workItemKindText(item.Kind), Reason: item.Reason, ActorID: item.ActorID.String(),
			Terminal: item.Terminal, NextAction: casesDomain.RecommendedNextAction(item.State),
			CreatedAt: formatTime(item.CreatedAt), Facts: factsByScope[item.WorkItemID],
		}
		if section, known := obsSections[item.WorkItemID]; known {
			card.ObsFields = observationFieldRows(section)
			card.ObsCount = strconv.Itoa(section.ObservationCount)
		}
		if section, known := objSections[item.WorkItemID]; known {
			card.ObjCurrent = objectionCardOf(section)
			card.ObjCount = strconv.Itoa(section.ObjectionCount)
		}
		// The Story 2.9 commitment counts of this item with the
		// Commitment Review page link (the full cards live there).
		card.CommitReviewURL = "/app/tenants/" + view.TenantID.String() + "/cases/" + view.Case.CaseID.String() + "/work-items/" + item.WorkItemID.String() + "/commitments"
		if section, known := commitSections[item.WorkItemID]; known {
			card.CommitCount = strconv.Itoa(len(section.Versions))
			card.CommitConfirmCount = strconv.Itoa(len(section.Confirmations))
			card.CommitDeclineCount = strconv.Itoa(len(section.Declines))
		}
		// The Story 2.10 continuation cards of this item: the links with
		// the read-time state derivations and the FR-16 delivery words,
		// the event feed of their uses (the token never appears).
		linksOfItem := map[id.UUID]bool{}
		for _, link := range view.Continuations.Links {
			if link.WorkItemID != item.WorkItemID {
				continue
			}
			linksOfItem[link.LinkID] = true
			card.ContLinks = append(card.ContLinks, continuationLinkCardView{
				LinkID:         link.LinkID.String(),
				PurposesText:   continuationPurposesText(link.Purposes),
				StateText:      continuationLinkStateText(link.State),
				StatusText:     continuationLinkStatusText(link, usesOfLink(view.Continuations.Uses, link.LinkID)),
				UsesText:       strconv.Itoa(link.AcceptedUses) + " / " + strconv.Itoa(link.ConsumptionLimit),
				Question:       link.Question,
				ParticipantRef: link.ParticipantRef,
				ExpiresText:    formatTime(link.ExpiresAt),
				Revocable:      link.State == casesDomain.ContinuationStateValid && !item.Terminal,
				RevokeURL:      "/app/tenants/" + view.TenantID.String() + "/cases/" + view.Case.CaseID.String() + "/work-items/" + item.WorkItemID.String() + "/continuation-links/" + link.LinkID.String() + "/revoke",
			})
		}
		for _, use := range view.Continuations.Uses {
			if !linksOfItem[use.LinkID] {
				continue
			}
			card.ContUses = append(card.ContUses, continuationUseCardView{
				UseID: use.UseID.String(), LinkID: use.LinkID.String(),
				KindText: continuationUseKindText(use.UseKind),
				FactID:   use.FactID.String(), FactVersion: strconv.FormatInt(use.FactVersion, 10),
				SubmittedAt: formatTime(use.SubmittedAt),
			})
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

// observationFieldRows flattens one observation section of the case view
// into the field cards of the Work Control Card: the value text, the matrix
// words, the carrier words, both times, the honest state (the aria-live
// region of the panel announces the change without a focus move) and the
// conflict badge text.
func observationFieldRows(section casesAPI.ObservationWorkItemView) []observationFieldView {
	rows := make([]observationFieldView, 0, len(section.Fields))
	for _, field := range section.Fields {
		row := observationFieldView{
			MaterialKey: field.MaterialKey, Projection: field.Projection,
			FreshnessText:    strconv.Itoa(field.FreshnessSeconds) + " с",
			OwningSystemText: field.OwningSystem + " (система-владелец поля)",
		}
		if field.Current != nil {
			current := field.Current
			row.HasCurrent = true
			row.No = strconv.FormatInt(current.ObservationNo, 10)
			row.Status = current.Status
			row.StatusText = observationStatusText(current.Status)
			row.State = field.State
			row.StateText = observationStateText(field.State)
			if field.State == casesDomain.ObservationStateStale {
				row.StaleText = casesDomain.StaleBadgeText
			}
			row.Reason = current.Reason
			row.ReasonText = observationReasonText(current.Reason)
			row.ValueText = string(current.Value)
			row.ExternalID = current.ExternalID
			row.ExternalVersion = current.ExternalVersion
			row.ObservedAt = formatTime(current.ObservedAt)
			row.ReceivedAt = formatTime(current.ReceivedAt)
			row.Conflict = field.ConflictBadge
			row.ConflictText = observationConflictText(field.ConflictBadge)
		}
		rows = append(rows, row)
	}
	return rows
}

// objectionCardOf flattens one objection section of the case view into
// the CURRENT objection card of the Work Control Card: the record words
// with the honest Russian texts, the verbatim spans, the lifetime budget
// and the stopped badge word (the aria-live region of the panel announces
// the change without a focus move).
func objectionCardOf(section casesAPI.ObjectionWorkItemView) objectionCardView {
	card := objectionCardView{
		Refusals:  strconv.Itoa(section.Refusals),
		Threshold: strconv.Itoa(section.Threshold),
		Stopped:   section.Stopped,
	}
	if section.Stopped {
		card.StoppedText = "Убеждение остановлено — разрешены только: вежливое завершение, запрос приватности, передача человеку"
	}
	if section.Acknowledged {
		card.AcknowledgedText = "подтверждено (acknowledge_concern отправлен)"
	}
	if section.Current != nil && section.Current.Record != nil {
		record := section.Current.Record
		card.HasCurrent = true
		card.No = strconv.FormatInt(record.ObjectionNo, 10)
		card.TypeText = objectionTypeText(record.ObjectionType)
		card.SignalText = objectionSignalText(record.ClientSignal)
		card.Spans = append([]string(nil), record.Spans...)
		card.TurnRef = record.TurnRef.String()
		card.CreatedAt = formatTime(record.CreatedAt)
		card.PlaybookVersion = record.PlaybookVersion
	}
	if len(section.LinkedReplies) > 0 {
		card.LinkedCount = strconv.Itoa(len(section.LinkedReplies))
	}
	if len(section.NextSteps) > 0 {
		words := ""
		for i, step := range section.NextSteps {
			if i > 0 {
				words += ", "
			}
			words += step
		}
		card.NextStepsText = words
	}
	return card
}

// objectionTypeText renders the objection type word honestly.
func objectionTypeText(word string) string {
	switch word {
	case casesDomain.ObjectionTypeTrust:
		return "trust (доверие)"
	case casesDomain.ObjectionTypePriceComparison:
		return "price_comparison (сравнение с более дешёвым)"
	case casesDomain.ObjectionTypeGeneralDoubt:
		return "general_doubt (общее сомнение)"
	}
	return word
}

// objectionSignalText renders the client signal word honestly.
func objectionSignalText(word string) string {
	switch word {
	case casesDomain.ClientSignalConcern:
		return "concern (опасение — убеждение продолжается)"
	case casesDomain.ClientSignalRefusal:
		return "refusal (отказ — считает бюджет)"
	case casesDomain.ClientSignalStopIntent:
		return "stop_intent (просьба прекратить — останов немедленно)"
	case casesDomain.ClientSignalHumanRequest:
		return "human_request (требование человека — останов немедленно)"
	}
	return word
}

// objectionStepText renders the playbook step word honestly.
func objectionStepText(word string) string {
	switch word {
	case casesDomain.PlaybookStepAcknowledgeConcern:
		return "acknowledge_concern (признать смысл опасения — первый ответ)"
	case casesDomain.PlaybookStepPresentEvidence:
		return "present_evidence (доказательство)"
	case casesDomain.PlaybookStepHonestNoAnswer:
		return "honest_no_answer (честное отсутствие ответа)"
	case casesDomain.PlaybookStepOfferAlternative:
		return "offer_alternative (предложить альтернативу)"
	case casesDomain.PlaybookStepClarifyQuestion:
		return "clarify_question (уточняющий вопрос)"
	}
	return word
}

// prepareOwnershipPanel flattens the ownership section into the template
// projection: every row rendered as text — accessible without color,
// assigned and acknowledged never confusable, the primary action matching
// the current state.
func prepareOwnershipPanel(view casesAPI.CaseView) ownershipPanelView {
	ownership := view.Ownership
	state := ownership.State
	panel := ownershipPanelView{
		OwnerName:    ownership.OwnerName,
		CurrentEpoch: strconv.FormatInt(0, 10),
		HasTerms:     len(ownership.Terms) > 0,
		TermRows:     make([]ownershipTermRow, 0, len(ownership.Terms)),
		HasReplies:   len(ownership.Replies) > 0,
		ReplyRows:    make([]replyRowView, 0, len(ownership.Replies)),
		// The Story 2.12 delivery states keyed by the reply id (the honest
		// derivation of the panel column — the LAST evidence event decides).
		deliveries:    replyDeliveryIndex(ownership.Deliveries),
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
		// The Story 2.11 transfer card: the current preference words the
		// receiving operator reads BEFORE the ack (AC-1 «передаются
		// каждому последующему Response Owner»); an empty word set stays
		// empty — the honest absence, never an invented default.
		preferenceWords := []string{}
		for _, pref := range view.Accommodations.Current {
			if !pref.Cleared {
				word := accommodationKeyText(pref.Key)
				if pref.Value != "" {
					word += " = " + pref.Value
				}
				preferenceWords = append(preferenceWords, word)
			}
		}
		if len(preferenceWords) > 0 {
			panel.AccommodationWords = strings.Join(preferenceWords, "; ")
		}
		// The Story 2.12 transfer card: the current degraded route words —
		// the receiving operator reads the operational route state BEFORE
		// the ack (the preferences precedent).
		degradedWords := []string{}
		for _, state := range view.RouteDegradations.Current {
			degradedWords = append(degradedWords, state.Channel+" ("+state.CauseText+")")
		}
		if len(degradedWords) > 0 {
			panel.DegradationWords = strings.Join(degradedWords, "; ")
		}
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
		if card.Reply.AccuracyQualifier != "" {
			row.QualifierText = accuracyQualifierText(card.Reply.AccuracyQualifier)
		}
		if card.OutcomeKind != "" {
			row.OutcomeAt = formatTime(card.OutcomeAt)
		}
		// The Story 2.12 columns: the derived honest delivery state with the
		// last evidence anchor and the update promise word of the degraded
		// route reply (the promise select of the form mirrors the options).
		if delivery, known := panel.deliveries[card.Reply.ReplyID.String()]; known {
			row.DeliveryText = deliveryStateText(delivery.State)
			if delivery.LastEventNo > 0 {
				row.DeliveryDetail = evidenceKindText(delivery.LastKind) + " · №" + strconv.FormatInt(delivery.LastEventNo, 10) + " · " + formatTime(delivery.LastEventAt)
			}
		}
		if card.Reply.UpdatePromise != "" {
			row.PromiseText = updatePromiseText(card.Reply.UpdatePromise)
		}
		panel.ReplyRows = append(panel.ReplyRows, row)
	}
	for _, card := range ownership.Replies {
		panel.ReplyEvidenceOptions = append(panel.ReplyEvidenceOptions, workOption{
			Value: card.Reply.ReplyID.String(),
			Text:  "ответ " + card.Reply.ReplyID.String() + " (" + replyClassText(card.Reply.ReplyClass) + ", " + card.Reply.Channel + ")",
		})
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
	Alternatives   []policyAlternativeRow
	DegradedPaths  []policyDegradedPathRow
	PermittedText  string
	DecisionDigest string
	CreatedAt      string
}

// policyAlternativeRow is one active alternative word of the outcome:
// the word with its honest limitation text (the dictionary of the
// Domain Pack — the panel renders the text, the database carries the
// words only).
type policyAlternativeRow struct {
	Word           string
	WordText       string
	LimitationText string
}

// policyDegradedPathRow is one permitted path word of the degraded
// mode outcome (Story 2.12): the word with its honest fixed text — the
// panel renders the text, the database carries the words only (the
// AC-3 dictionary of the Domain Pack).
type policyDegradedPathRow struct {
	Word     string
	PathText string
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
	case casesDomain.OutcomeDegradedMode:
		return "degraded_mode (маршрут деградирован — обещаний нет, разрешённые пути в исходе)"
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
	case casesDomain.ReasonDegradedRoute:
		return "degraded_route (маршрут диалога деградирован)"
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
		Alternatives:   make([]policyAlternativeRow, 0, len(outcome.ActiveAlternatives)),
		DegradedPaths:  make([]policyDegradedPathRow, 0, len(outcome.DegradedPermittedPaths)),
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
	// The active alternatives of the outcome (Story 2.11): every word
	// carries its honest limitation text — the panel renders the word
	// with the text and the next steps of the outcome (AC-11: the
	// variants describe the result, the limitations and the next step
	// in plain words).
	for _, word := range outcome.ActiveAlternatives {
		row.Alternatives = append(row.Alternatives, policyAlternativeRow{
			Word: word, WordText: accuracyQualifierText(word),
			LimitationText: casesDomain.AlternativeLimitationText(word),
		})
	}
	// The permitted paths of the degraded mode outcome (Story 2.12): every
	// word carries its honest fixed text — the panel renders the words with
	// the texts (AC-3: the permitted continuation paths of the degraded
	// route; the sales path is closed machine-wise, the questions stay).
	for _, word := range outcome.DegradedPermittedPaths {
		row.DegradedPaths = append(row.DegradedPaths, policyDegradedPathRow{
			Word: word, PathText: casesDomain.DegradedPathText(word),
		})
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

// continuationLinkStateText renders the honest state word of the panel
// (the read-time derivation — never a stored status).
func continuationLinkStateText(state string) string {
	switch state {
	case casesDomain.ContinuationStateValid:
		return "действует"
	case casesDomain.ContinuationStateExpired:
		return "истёк срок"
	case casesDomain.ContinuationStateRevoked:
		return "отозвана (ожидание прекращено)"
	case casesDomain.ContinuationStateConsumed:
		return "отправка завершена"
	case casesDomain.ContinuationStateOwnerStale:
		return "владение изменилось"
	}
	return state
}

// continuationLinkStatusText renders the FR-16 delivery word of the
// panel: the delivered binding of the outgoing reply, the expected
// answer of a fresh valid link, the finished submission, the expired
// term or the stopped expectation (the statuses are derivations, never
// stored words — stated in text, never by the color alone).
func continuationLinkStatusText(link casesAPI.ContinuationLinkView, useCount int) string {
	switch {
	case link.Delivered:
		return "доставлена (связана с исходящим ответом)"
	case link.State == casesDomain.ContinuationStateValid && useCount == 0:
		return "ожидается ответ"
	case link.State == casesDomain.ContinuationStateConsumed:
		return "отправка завершена"
	case link.State == casesDomain.ContinuationStateExpired:
		return "истёк срок"
	case link.State == casesDomain.ContinuationStateRevoked:
		return "ожидание прекращено"
	case link.State == casesDomain.ContinuationStateOwnerStale:
		return "владение изменилось — ссылка недействительна"
	}
	return "действует"
}

// continuationPurposesText renders the purpose words of the panel card.
func continuationPurposesText(purposes []string) string {
	if len(purposes) == 0 {
		return "—"
	}
	words := make([]string, 0, len(purposes))
	for _, word := range purposes {
		words = append(words, continuationPurposeText(word))
	}
	return strings.Join(words, ", ")
}

// continuationPurposeText renders the honest purpose word (the v1
// dictionary).
func continuationPurposeText(purpose string) string {
	switch purpose {
	case casesDomain.ContinuationPurposeFactCorrection:
		return "исправление факта (fact_correction)"
	}
	return purpose
}

// continuationUseKindText renders the honest event word of the feed.
func continuationUseKindText(kind string) string {
	switch kind {
	case casesDomain.ContinuationUseFactCorrectionSubmitted:
		return "исправление отправлено"
	case casesDomain.ContinuationUseHumanRequested:
		return "позвать человека"
	case casesDomain.ContinuationUseReturnedToConversation:
		return "вернулся в разговор"
	case casesDomain.ContinuationUseVehicleConfirmationSubmitted:
		return "подтверждение автомобиля клиентом"
	}
	return kind
}

// usesOfLink counts the use rows of one link (the events included — the
// FR-16 «ожидается ответ» is the valid link with no rows at all).
func usesOfLink(uses []casesAPI.ContinuationUseView, linkID id.UUID) int {
	count := 0
	for _, use := range uses {
		if use.LinkID == linkID {
			count++
		}
	}
	return count
}

// accommodationStateRow is one key of the CURRENT preferences set of the
// «Предпочтения связи» panel: the key word with the honest text, the
// latest revision number, the clearing mark (a cleared key stays visible
// with the honest mark — the history truth) and the valued material (the
// channel word or the availability line; empty for the flags).
type accommodationStateRow struct {
	Key      string
	KeyText  string
	Revision string
	Cleared  bool
	Value    string
}

// accommodationRecordRow is one append-only revision row of the honest
// history: the key word, the monotonic revision, the valued material, the
// clearing mark, the operator note and the actor with the time.
type accommodationRecordRow struct {
	Key        string
	KeyText    string
	Revision   string
	Value      string
	Cleared    bool
	Note       string
	ActorID    string
	RecordedAt string
}

// accommodationKeyText renders the honest text of one preference key word
// (the fixed dictionary of the Domain Pack v0 — the panel and the forms
// never invent options).
func accommodationKeyText(key string) string {
	switch key {
	case casesDomain.AccommodationNoPhoto:
		return "no_photo — клиент не может присылать фото"
	case casesDomain.AccommodationNoWeb:
		return "no_web — клиент не пользуется web"
	case casesDomain.AccommodationNoVoice:
		return "no_voice — клиент не может говорить голосом"
	case casesDomain.AccommodationTextFirst:
		return "text_first — клиент просит писать текстом"
	case casesDomain.AccommodationSlowedPace:
		return "slowed_pace — клиенту нужен замедленный темп"
	case casesDomain.AccommodationMandatoryReadback:
		return "mandatory_readback — клиенту нужно явное повторение вслух"
	case casesDomain.AccommodationPreferredChannel:
		return "preferred_channel — предпочтительный канал связи"
	case casesDomain.AccommodationAvailableTime:
		return "available_time — доступное время для связи"
	}
	return key
}

// accuracyQualifierText renders the honest word of one alternative path
// word (the qualifier select of the sales reply form and the reply rows of
// the panel; the limitation text renders beside it — AC-11).
func accuracyQualifierText(word string) string {
	switch word {
	case casesDomain.AlternativeTextDescription:
		return "text_description — точность ниже фото"
	case casesDomain.AlternativeRangeEstimate:
		return "range_estimate — только диапазон, не точный результат"
	case casesDomain.AlternativeCallback:
		return "callback — уточнение голосом с человеком"
	case casesDomain.AlternativeHumanHandoff:
		return "human_handoff — продолжит человек"
	}
	return word
}

// replyDeliveryIndex indexes the derived delivery states by the reply id.
func replyDeliveryIndex(deliveries []casesAPI.ReplyDeliveryView) map[string]casesAPI.ReplyDeliveryView {
	index := map[string]casesAPI.ReplyDeliveryView{}
	for _, delivery := range deliveries {
		index[delivery.ReplyID.String()] = delivery
	}
	return index
}

// degradationCauseText renders the honest cause word text of the panel.
func degradationCauseText(word string) string {
	return casesDomain.DegradationCauseText(word)
}

// evidenceKindText renders the honest evidence word text of the panel.
func evidenceKindText(word string) string {
	return casesDomain.DeliveryEvidenceText(word)
}

// deliveryStateText renders the derived delivery state word of the reply
// panel column.
func deliveryStateText(state string) string {
	switch state {
	case casesDomain.ReplyDeliveryPrepared:
		return "подготовлен (ещё не отправлен)"
	case casesDomain.ReplyDeliveryConfirmed:
		return "подтверждена (квитанция есть)"
	case casesDomain.ReplyDeliveryFailed:
		return "отказ провайдера"
	default:
		return "неизвестна (квитанции нет)"
	}
}

// updatePromiseText renders the honest promise word text of the reply form
// and the panel.
func updatePromiseText(word string) string {
	return casesDomain.UpdatePromiseText(word)
}

// channelWordText renders the channel word of the route options.
func channelWordText(word string) string {
	return word
}

// handoffRoleText renders the honest target-role word text of the request
// form (the fixed dictionary — the checkbox labels of the ask).
func handoffRoleText(word string) string {
	switch word {
	case casesDomain.HandoffTargetRoleOperator:
		return "operator — рабочий оператор (полномочие review)"
	case casesDomain.HandoffTargetRoleSupervision:
		return "supervision — надзор (полномочие configuration)"
	}
	return word
}

// handoffStateWordText renders the current lifecycle state line of the
// panel (the state word with its honest text — the words, never the
// colors).
func handoffStateWordText(view casesAPI.HandoffView) string {
	if len(view.Requests) == 0 {
		return "нет зарегистрированного запроса."
	}
	last := view.Requests[len(view.Requests)-1]
	return last.StateText + " (слово: " + last.State + ", запрос №" + strconv.FormatInt(last.RequestNo, 10) + ")"
}

// ---------------------------------------------------------------------------
// The Story 4.2 vehicle identification panel (the operator review
// surface — one of the TWO full-VIN surfaces)
// ---------------------------------------------------------------------------

// vehicleAttributeRow is one attribute of the panel: the label with the
// material key, the raw value (the full VIN renders here — the operator
// review surface), the version with the provenance, the read-time 2.2
// status and the confirmation rendering with the stale word.
type vehicleAttributeRow struct {
	Key         string
	Label       string
	Kind        string
	Value       string
	Version     string
	ObservedAt  string
	ReceivedAt  string
	SourceType  string
	Channel     string
	StatusText  string
	StatusKind  string
	ConfirmText string
	StaleWord   string
	VINWords    []string
}

// vehicleReadbackSymbolRow is one readback symbol (the row word
// distinguishes the latin/digit/cyrillic rows — AC-4).
type vehicleReadbackSymbolRow struct {
	Row  string
	Char string
	Word string
}

// vehicleReadbackRow is one readback block (4-4-4-4-1).
type vehicleReadbackRow struct {
	Block   string
	Symbols []vehicleReadbackSymbolRow
}

// vehiclePanelView is the flattened Story 4.2 panel of the case page:
// the honest degradation note when the pinned pack carries no schema,
// the attribute rows, the readback details of the VIN, the missing keys
// list, the satisfied alternative sets, the ProfileDigest, the stale
// note and the mask boundary word.
type vehiclePanelView struct {
	Present       bool
	Note          string
	Attributes    []vehicleAttributeRow
	Readback      []vehicleReadbackRow
	FocusWords    []string
	MissingKeys   []string
	SatisfiedText string
	Digest        string
	StaleNote     string
	MaskedWord    string
}

// vehicleStatusText renders the honest status word of one attribute.
func vehicleStatusText(status string) string {
	switch status {
	case casesDomain.FactStatusConfirmed:
		return "подтверждён"
	case casesDomain.FactStatusConflicting:
		return "конфликт значений — Staffy не выбирает вариант, уточните у клиента"
	default:
		return "без подтверждения"
	}
}

// prepareVehiclePanel flattens the Story 4.2 vehicle panel view of the
// case API answer into the template rows. The full VIN renders here and
// on the protected client form only; the mask word of the panel note
// carries the honest boundary (AC-9).
func prepareVehiclePanel(view casesAPI.CaseView) vehiclePanelView {
	panel := vehiclePanelView{
		Note:        view.Vehicle.Note,
		MaskedWord:  "",
		Attributes:  []vehicleAttributeRow{},
		Readback:    []vehicleReadbackRow{},
		FocusWords:  []string{},
		MissingKeys: []string{},
	}
	if !view.Vehicle.SchemaDefined {
		return panel
	}
	panel.Present = true
	profile := view.Vehicle.Profile
	for _, attribute := range profile.Attributes {
		row := vehicleAttributeRow{
			Key: attribute.MaterialKey, Label: attribute.Label, Kind: attribute.Kind,
			Value: attribute.RawValue, Version: "v" + strconv.FormatInt(attribute.FactVersion, 10),
			ObservedAt: formatTime(attribute.ObservedAt), ReceivedAt: formatTime(attribute.ReceivedAt),
			SourceType: attribute.SourceType, Channel: attribute.Channel,
			StatusText: vehicleStatusText(attribute.Status), StatusKind: attribute.Status,
		}
		if attribute.Status == casesDomain.FactStatusConfirmed {
			row.StatusKind = "confirmed"
		}
		if attribute.Confirmation != nil {
			row.ConfirmText = "Подтверждает: " + attribute.Confirmation.ConfirmerType +
				" (версия v" + strconv.FormatInt(attribute.Confirmation.VersionNo, 10) +
				", " + formatTime(attribute.Confirmation.At) + ")"
			row.StaleWord = attribute.Confirmation.StaleWord
		}
		if attribute.NormalizedValue != nil {
			row.VINWords = attribute.NormalizedValue.Words
			if len(row.VINWords) == 0 {
				row.VINWords = []string{"проверки пройдены: длина 17, алфавит без I/O/Q, омоглифов нет"}
				if attribute.NormalizedValue.CheckDigit.Applicable && attribute.NormalizedValue.CheckDigit.Match {
					row.VINWords = append(row.VINWords, "контрольная цифра (позиция 9) совпала")
				}
			}
			panel.MaskedWord = casesDomain.MaskedVIN(attribute.NormalizedValue.Normalized)
			panel.Readback = make([]vehicleReadbackRow, 0, len(casesDomain.ReadbackBlocks(attribute.NormalizedValue.Normalized)))
			for _, block := range casesDomain.ReadbackBlocks(attribute.NormalizedValue.Normalized) {
				readbackRow := vehicleReadbackRow{Block: block.Block, Symbols: []vehicleReadbackSymbolRow{}}
				for _, symbol := range block.Symbols {
					readbackRow.Symbols = append(readbackRow.Symbols, vehicleReadbackSymbolRow{
						Row: symbol.Row, Char: symbol.Char, Word: symbol.Word,
					})
				}
				panel.Readback = append(panel.Readback, readbackRow)
			}
			panel.FocusWords = casesDomain.ReadbackFocus(attribute.NormalizedValue.Normalized, attribute.NormalizedValue.CyrillicAmbiguities)
		}
		panel.Attributes = append(panel.Attributes, row)
	}
	panel.MissingKeys = profile.Completeness.MissingKeys
	panel.SatisfiedText = strings.Join(profile.Completeness.SatisfiedBy, "; ")
	panel.Digest = profile.ProfileDigest
	panel.StaleNote = profile.StaleNote
	return panel
}

// ---------------------------------------------------------------------------
// The Story 4.3 part request panel (the operator review surface — the
// read-time request derivation under the pack part request schema)
// ---------------------------------------------------------------------------

// partAttributeRow is one attribute of the part request panel: the label
// with the material key and the role, the raw value, the version with
// the provenance, the read-time 2.2 status, the confirmation rendering
// with the stale word and the number checks words of the oem_number
// role (the flags are TEXT — never only color).
type partAttributeRow struct {
	Key         string
	Label       string
	Role        string
	Kind        string
	Value       string
	Version     string
	ObservedAt  string
	ReceivedAt  string
	SourceType  string
	Channel     string
	StatusText  string
	StatusKind  string
	ConfirmText string
	StaleWord   string
	NumberWords []string
	Normalized  string
}

// partQuestionRow is one missing-key clarify question of the SELECTED
// mode's mandatory set (the minimal question of the not-confirmed key —
// the confirmed material is never re-asked, AC-3).
type partQuestionRow struct {
	Key      string
	Question string
}

// partBoundaryRow is one honesty boundary of the request: the kind word
// (diagnostic | safety | contradiction) and the text — the alert
// material renders as TEXT with role=alert (never only color, AC-13).
type partBoundaryRow struct {
	Kind string
	Text string
}

// partDictionaryRow is one key of the schema dictionary hint of the
// empty material (the operator records the material by the pack
// dictionary — the honest absence, never a guess).
type partDictionaryRow struct {
	Key      string
	Label    string
	Kind     string
	Role     string
	Optional bool
}

// partPanelView is the flattened Story 4.3 panel of the case page: the
// honest degradation note when the pinned pack carries no schema, the
// mode with the basis word (or the unstructured word with the
// dictionary hint), the attribute rows, the missing-key questions, the
// honesty boundaries (the alert material), the number-match constant
// word, the confidence word, the PartRequestDigest, the several-parts
// word and the stale note.
type partPanelView struct {
	Present          bool
	Note             string
	Mode             string
	ModeLabel        string
	ModeBasisWord    string
	UnstructuredWord string
	Dictionary       []partDictionaryRow
	Attributes       []partAttributeRow
	Questions        []partQuestionRow
	MissingKeys      []string
	CompleteWord     string
	Boundaries       []partBoundaryRow
	HandoffWords     []string
	NumberMatchWord  string
	ConfidenceWord   string
	Digest           string
	SeveralPartsWord string
	StaleNote        string
}

// partStatusText renders the honest status word of one attribute (the
// same words as the vehicle panel — the 2.2 canon).
func partStatusText(status string) string {
	switch status {
	case casesDomain.FactStatusConfirmed:
		return "подтверждён"
	case casesDomain.FactStatusConflicting:
		return "конфликт значений — Staffy не выбирает вариант, уточните у клиента"
	default:
		return "без подтверждения"
	}
}

// partRoleText renders the honest role word of one material key (the
// ladder roles of the pack schema).
func partRoleText(role string) string {
	switch role {
	case casesDomain.PartRolePartKind:
		return "деталь"
	case casesDomain.PartRoleSymptom:
		return "симптом"
	case casesDomain.PartRoleOemNumber:
		return "номер детали"
	case casesDomain.PartRolePreference:
		return "предпочтение"
	default:
		return ""
	}
}

// preparePartPanel flattens the Story 4.3 part request panel view of the
// case API answer into the template rows. The mode with the basis word
// is TEXT; the boundaries carry role=alert material; the number checks
// render the flags with the honest words (never only color).
func preparePartPanel(view casesAPI.CaseView) partPanelView {
	panel := partPanelView{
		Note:         view.PartRequest.Note,
		Attributes:   []partAttributeRow{},
		Questions:    []partQuestionRow{},
		Boundaries:   []partBoundaryRow{},
		HandoffWords: []string{},
		Dictionary:   []partDictionaryRow{},
		MissingKeys:  []string{},
	}
	if !view.PartRequest.SchemaDefined {
		return panel
	}
	panel.Present = true
	part := view.PartRequest.Part
	panel.Mode = part.Mode
	panel.ModeLabel = part.ModeLabel
	panel.ModeBasisWord = part.ModeBasisWord
	panel.UnstructuredWord = part.UnstructuredWord
	panel.NumberMatchWord = part.NumberMatchWord
	panel.ConfidenceWord = part.ConfidenceWord
	panel.Digest = part.PartRequestDigest
	panel.SeveralPartsWord = part.SeveralPartsWord
	panel.StaleNote = part.StaleNote
	panel.CompleteWord = part.Completeness.CompleteWord
	panel.MissingKeys = part.Completeness.MissingKeys
	for key, question := range part.Questions {
		panel.Questions = append(panel.Questions, partQuestionRow{Key: key, Question: question})
	}
	sort.Slice(panel.Questions, func(i, j int) bool { return panel.Questions[i].Key < panel.Questions[j].Key })
	for _, key := range part.SchemaDictionary {
		panel.Dictionary = append(panel.Dictionary, partDictionaryRow{
			Key: key.Key, Label: key.Label, Kind: key.Kind, Role: partRoleText(key.Role), Optional: key.Optional,
		})
	}
	for _, boundary := range part.Boundaries {
		panel.Boundaries = append(panel.Boundaries, partBoundaryRow{Kind: boundary.Kind, Text: boundary.Text})
	}
	for _, word := range part.HandoffWords {
		panel.HandoffWords = append(panel.HandoffWords, word+" — "+casesDomain.HandoffReasonText(word))
	}
	for _, attribute := range part.Attributes {
		row := partAttributeRow{
			Key: attribute.MaterialKey, Label: attribute.Label, Role: partRoleText(attribute.Role), Kind: attribute.Kind,
			Value: attribute.RawValue, Version: "v" + strconv.FormatInt(attribute.FactVersion, 10),
			ObservedAt: formatTime(attribute.ObservedAt), ReceivedAt: formatTime(attribute.ReceivedAt),
			SourceType: attribute.SourceType, Channel: attribute.Channel,
			StatusText: partStatusText(attribute.Status), StatusKind: attribute.Status,
		}
		if attribute.Status == casesDomain.FactStatusConfirmed {
			row.StatusKind = "confirmed"
		}
		if attribute.Confirmation != nil {
			row.ConfirmText = "Подтверждает: " + attribute.Confirmation.ConfirmerType +
				" (версия v" + strconv.FormatInt(attribute.Confirmation.VersionNo, 10) +
				", " + formatTime(attribute.Confirmation.At) + ")"
			row.StaleWord = attribute.Confirmation.StaleWord
		}
		if attribute.NumberChecks != nil {
			row.Normalized = attribute.NumberChecks.Normalized
			row.NumberWords = attribute.NumberChecks.Words
			if len(row.NumberWords) == 0 {
				row.NumberWords = []string{"проверки номера пройдены: алфавит чист, омоглифов нет"}
			}
		}
		panel.Attributes = append(panel.Attributes, row)
	}
	return panel
}
