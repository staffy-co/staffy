package httptransport

import (
	"strings"
	"time"

	privacyAPI "github.com/artemiosu/staffy/internal/privacy/api"
	privacyDomain "github.com/artemiosu/staffy/internal/privacy/domain"
)

// dataClassView is one class row of the data map page, prepared for the
// template with readable Russian statuses and dates.
type dataClassView struct {
	ID            string
	Title         string
	StatusText    string
	StatusClass   string
	MissingText   string
	Entry         *mapEntryView
	Policy        *policyView
	HoldText      string
	HoldClass     string
	DeletionDue   string
	MandatoryDue  string
	TablesText    string
}

type mapEntryView struct {
	Version       int64
	Purpose       string
	LegalBasis    string
	Controller    string
	Processor     string
	Recipients    string
	SubProcessors string
	Locations     string
	AccessRoles   string
	KnownCopies   string
	PolicyRef     string
	DeletionMethod string
	ExportMethod  string
	PublishedAt   string
}

type policyView struct {
	Version          int64
	Purpose          string
	LegalBasis       string
	Source           string
	TerminalEvent    string
	TermsText        string
	BackupText       string
	HoldException    string
	OwnerNote        string
	PublishedAt      string
}

type receiptLineView struct {
	RequestID   string
	CompletedAt string
	HashPrefix  string
	OutcomeText string
	HasUnknown  bool
	HasBlocked  bool
}

type holdEventView struct {
	Event       string
	EventText   string
	Classes     string
	Basis       string
	Approver    string
	ReviewText  string
	CreatedAt   string
}

type receiptDetailView struct {
	RequestID   string
	ReceiptID   string
	Hash        string
	CompletedAt string
	Authority   string
	Policies    []policyPinView
	Outcomes    []outcomeView
}

type policyPinView struct {
	Class         string
	Version       int64
	TerminalEvent string
}

type outcomeView struct {
	Class          string
	ClassTitle     string
	Outcome        string
	OutcomeText    string
	OutcomeClass   string
	Count          int64
	DueAt          string
	Note           string
	BlockRequired  string
	BlockGate      string
	BlockOwner     string
	Reconciliation string
}

func mapStatusText(status string) string {
	switch status {
	case privacyDomain.MapStatusComplete:
		return "заполнена"
	case privacyDomain.MapStatusIncomplete:
		return "неполная"
	default:
		return "не описан"
	}
}

func outcomeText(outcome string) string {
	switch outcome {
	case privacyDomain.OutcomeDeleted:
		return "удалено"
	case privacyDomain.OutcomeScheduled:
		return "запланировано"
	case privacyDomain.OutcomeRetainedByLaw:
		return "сохранено по закону"
	case privacyDomain.OutcomeUnsupported:
		return "не поддерживается"
	case privacyDomain.OutcomeUnknown:
		return "неопределённо"
	}
	return outcome
}

func holdText(hold privacyDomain.HoldState) (text, class string) {
	switch hold.Status {
	case privacyDomain.HoldActive:
		return "Юридическое удержание активно (основание: " + hold.Basis + "; утвердил: " + hold.Approver + "; пересмотр до " + hold.ReviewDeadline.Format("02.01.2006") + ")", "hold-active"
	case privacyDomain.HoldRequiresDecision:
		return "Юридическое удержание требует решения: срок пересмотра истёк (основание: " + hold.Basis + ")", "hold-decision"
	}
	return "", ""
}

func joinNonEmpty(values []string) string {
	if len(values) == 0 {
		return "—"
	}
	return strings.Join(values, ", ")
}

func formatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("02.01.2006")
}

// prepareDataMapViews projects the derived class views into template values.
func prepareDataMapViews(view privacyAPI.DataViewResult) []dataClassView {
	out := make([]dataClassView, 0, len(view.Classes))
	for _, class := range view.Classes {
		item := dataClassView{
			ID:          string(class.Spec.ID),
			Title:       class.Spec.Title,
			StatusText:  mapStatusText(class.Status),
			TablesText:  strings.Join(class.Spec.Tables, ", "),
			DeletionDue: formatDate(class.DeletionDueAt),
			MandatoryDue: formatDate(class.MandatoryDueAt),
		}
		switch class.Status {
		case privacyDomain.MapStatusComplete:
			item.StatusClass = "map-complete"
		case privacyDomain.MapStatusIncomplete:
			item.StatusClass = "map-incomplete"
		default:
			item.StatusClass = "map-undocumented"
		}
		if len(class.Missing) > 0 {
			item.MissingText = "Не заполнено: " + strings.Join(class.Missing, ", ")
		}
		if class.Entry != nil {
			item.Entry = &mapEntryView{
				Version:        class.Entry.Version,
				Purpose:        class.Entry.Purpose,
				LegalBasis:     class.Entry.LegalBasis,
				Controller:     class.Entry.Controller,
				Processor:      class.Entry.Processor,
				Recipients:     joinNonEmpty(class.Entry.Recipients),
				SubProcessors:  joinNonEmpty(class.Entry.SubProcessors),
				Locations:      joinNonEmpty(class.Entry.Locations),
				AccessRoles:    joinNonEmpty(class.Entry.AccessibleRoles),
				KnownCopies:    joinNonEmpty(class.Entry.KnownCopies),
				PolicyRef:      class.Entry.RetentionPolicy,
				DeletionMethod: class.Entry.DeletionMethod,
				ExportMethod:   class.Entry.ExportMethod,
				PublishedAt:    class.Entry.CreatedAt.Format("02.01.2006 15:04 UTC"),
			}
		}
		if class.Policy != nil {
			item.Policy = &policyView{
				Version:       class.Policy.Version,
				Purpose:       class.Policy.Purpose,
				LegalBasis:    class.Policy.LegalBasis,
				Source:        class.Policy.Source,
				TerminalEvent: class.Policy.TerminalEvent,
				TermsText:     "активное использование " + itoa(class.Policy.ActiveUseDays) + " дн. + архив " + itoa(class.Policy.ArchiveDays) + " дн.; максимальный срок " + itoa(class.Policy.DeletionDays) + " дн.",
				BackupText:    itoa(class.Policy.BackupExpiryDays) + " дн.",
				HoldException: class.Policy.LegalHoldException,
				OwnerNote:     class.Policy.OwnerNote,
				PublishedAt:   class.Policy.CreatedAt.Format("02.01.2006 15:04 UTC"),
			}
		}
		if text, class := holdText(class.Hold); text != "" {
			item.HoldText, item.HoldClass = text, class
		}
		out = append(out, item)
	}
	return out
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}

// prepareReceiptViews projects the latest receipts into list rows.
func prepareReceiptViews(receipts []privacyDomain.Receipt) []receiptLineView {
	out := make([]receiptLineView, 0, len(receipts))
	for _, receipt := range receipts {
		line := receiptLineView{
			RequestID:   receipt.RequestID,
			CompletedAt: receipt.CompletedAt.Format("02.01.2006 15:04 UTC"),
			HashPrefix:  receipt.ReceiptHash[:12],
		}
		for _, outcome := range receipt.Outcomes {
			switch outcome.Outcome {
			case privacyDomain.OutcomeUnknown:
				line.HasUnknown = true
			case privacyDomain.OutcomeUnsupported, privacyDomain.OutcomeRetainedByLaw:
				line.HasBlocked = true
			}
		}
		switch {
		case line.HasUnknown:
			line.OutcomeText = "есть неопределённые результаты"
		case line.HasBlocked:
			line.OutcomeText = "есть честные ограничения"
		default:
			line.OutcomeText = "выполнено"
		}
		out = append(out, line)
	}
	return out
}

// prepareHoldEventViews projects hold events into readable rows.
func prepareHoldEventViews(events []privacyDomain.LegalHoldEvent) []holdEventView {
	out := make([]holdEventView, 0, len(events))
	for _, event := range events {
		classes := make([]string, 0, len(event.Classes))
		for _, class := range event.Classes {
			classes = append(classes, string(class))
		}
		eventText := "снято"
		if event.Event == "activated" {
			eventText = "активировано"
		}
		out = append(out, holdEventView{
			Event:      event.Event,
			EventText:  eventText,
			Classes:    strings.Join(classes, ", "),
			Basis:      event.Basis,
			Approver:   event.Approver,
			ReviewText: formatDate(&event.ReviewDeadline),
			CreatedAt:  event.CreatedAt.Format("02.01.2006 15:04 UTC"),
		})
	}
	return out
}

// prepareReceiptDetail projects one full receipt for the receipt page.
func prepareReceiptDetail(receipt privacyDomain.Receipt) receiptDetailView {
	titles := map[privacyDomain.ArtifactClass]string{}
	for _, spec := range privacyDomain.ClassSpecs() {
		titles[spec.ID] = spec.Title
	}
	detail := receiptDetailView{
		RequestID:   receipt.RequestID,
		ReceiptID:   receipt.ReceiptID,
		Hash:        receipt.ReceiptHash,
		CompletedAt: receipt.CompletedAt.Format("02.01.2006 15:04 UTC"),
		Authority:   receipt.Authority,
	}
	for _, pin := range receipt.PolicyVersions {
		detail.Policies = append(detail.Policies, policyPinView{Class: string(pin.Class), Version: pin.Version, TerminalEvent: pin.TerminalEvent})
	}
	for _, outcome := range receipt.Outcomes {
		view := outcomeView{
			Class:        string(outcome.Class),
			ClassTitle:   titles[outcome.Class],
			Outcome:      outcome.Outcome,
			OutcomeText:  outcomeText(outcome.Outcome),
			Count:        outcome.AffectedCount,
			DueAt:        formatDate(outcome.DueAt),
			Note:         outcome.Note,
		}
		switch outcome.Outcome {
		case privacyDomain.OutcomeDeleted:
			view.OutcomeClass = "outcome-deleted"
		case privacyDomain.OutcomeScheduled:
			view.OutcomeClass = "outcome-scheduled"
		case privacyDomain.OutcomeRetainedByLaw, privacyDomain.OutcomeUnsupported:
			view.OutcomeClass = "outcome-blocked"
		case privacyDomain.OutcomeUnknown:
			view.OutcomeClass = "outcome-unknown"
		}
		if outcome.Block != nil {
			view.BlockRequired = outcome.Block.Required
			view.BlockGate = outcome.Block.Gate
			view.BlockOwner = outcome.Block.Owner
		}
		if outcome.ReconciliationOwner != "" {
			view.Reconciliation = "Владелец сверки: " + outcome.ReconciliationOwner
			if outcome.ReconciliationDeadline != nil {
				view.Reconciliation += "; срок: " + outcome.ReconciliationDeadline.Format("02.01.2006")
			}
		}
		detail.Outcomes = append(detail.Outcomes, view)
	}
	return detail
}
