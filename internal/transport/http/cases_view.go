package httptransport

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
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
// connection, the state), the work items and the honest statements.
type caseDetailPageView struct {
	ScopeText        string
	CaseNo           string
	CaseID           string
	EmployeeID       string
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

func prepareCaseView(view casesAPI.CaseView) caseDetailPageView {
	out := caseDetailPageView{
		ScopeText:        casesScopeText(true),
		CaseNo:           strconv.FormatInt(view.Case.CaseNo, 10),
		CaseID:           view.Case.CaseID.String(),
		EmployeeID:       view.Case.EmployeeID.String(),
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
			StateText:  "Запрос ещё не классифицирован",
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
