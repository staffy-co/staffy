package httptransport

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
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
	WorkCards        []workCardView
	HasFacts         bool
	FactsAllowed     bool
	FactsNote        string
	ActiveWIOptions  []workOption
	ConversationOpts []workOption
	EventOptions     []workOption
	Flash            string
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
	for _, event := range view.Timeline {
		if event.State == "accepted" {
			out.EventOptions = append(out.EventOptions, workOption{
				Value: event.EventID.String(),
				Text:  event.Channel + " · " + formatTime(event.ReceivedAt) + " · " + event.EventID.String(),
			})
		}
	}
	for _, conversation := range view.Conversations {
		out.Conversations = append(out.Conversations, caseConversationRow{
			ConversationID:    conversation.ConversationID.String(),
			ConversationNo:    strconv.FormatInt(conversation.ConversationNo, 10),
			Provider:          conversation.Provider,
			ProviderSessionID: conversation.ProviderSessionID,
			OwnerID:           conversation.ResponseOwnerID.String(),
			OwnerEpoch:        strconv.FormatInt(conversation.ResponseOwnerEpoch, 10),
			WritableSurface:   conversation.WritableSurface,
			StateText:         "Активна",
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
