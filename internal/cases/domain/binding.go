package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Safe channel binding (Story 2.3, first slice)
// ---------------------------------------------------------------------------
//
// A binding links one conversation (the new channel session, born with its
// own case by the ordinary 2-1 ingress) INTO an existing conversation and
// case: the link is a separate append-only row — the conversation NEVER
// moves between cases (the append-only grants forbid the FK update), the
// events of the bound conversation keep attaching by the unchanged 2-1
// session continuation rule, and the unified view of the case is the
// read-time JOIN of the active bindings. The reversal is append-only too:
// after an unlink the history stays, the views recompute, and a new binding
// of the same conversation is a new row.

// Binding methods: the consent handoff (the participant explicitly agreed
// to continue in another channel — the purpose and the evidence of the
// agreement are required) and the duplicate review resolution (the reviewer
// decided the two requests belong to the same client — the resolution row
// is the basis).
const (
	BindingMethodConsentHandoff  = "consent_handoff"
	BindingMethodDuplicateReview = "duplicate_review"
)

// BindingMethods is the full dictionary.
var BindingMethods = []string{BindingMethodConsentHandoff, BindingMethodDuplicateReview}

// ChannelBinding is one append-only link: the target conversation X of the
// case A, the bound conversation Y (its own case B stays the owner of
// record), the method, the declared purpose, the on-behalf flag, the
// verified subject handle the link carries, the evidence (an accepted event
// of the case A or a note — the consent handoff requires one), the actor
// and the time.
type ChannelBinding struct {
	TenantID            id.UUID
	BindingID           id.UUID
	CaseID              id.UUID // the target case A
	ConversationID      id.UUID // the target conversation X of the case A
	BoundCaseID         id.UUID // the case B of the bound conversation Y
	BoundConversationID id.UUID // the bound conversation Y
	Method              string
	Purpose             string
	OnBehalf            bool
	SubjectHandle       string
	EvidenceEventID     id.UUID // zero when the note (or the resolution) carries the basis
	Note                string
	ActorID             id.UUID
	CreatedAt           time.Time
}

// BindingReversal is one append-only unlink: which binding, by whom, why
// and when. The binding row itself is never touched; the ACTIVE derivation
// is the anti-join with the reversals.
type BindingReversal struct {
	TenantID   id.UUID
	ReversalID id.UUID
	BindingID  id.UUID
	Reason     string
	ActorID    id.UUID
	CreatedAt  time.Time
}

// ReversalOf resolves the reversal of one binding (nil when the binding is
// active).
func ReversalOf(bindings []BindingReversal, bindingID id.UUID) *BindingReversal {
	for i := range bindings {
		if bindings[i].BindingID == bindingID {
			return &bindings[i]
		}
	}
	return nil
}

// ActiveBindings filters the bindings without a reversal: the links the
// unified case view joins. The order follows the input (the read orders by
// created_at).
func ActiveBindings(bindings []ChannelBinding, reversals []BindingReversal) []ChannelBinding {
	active := make([]ChannelBinding, 0, len(bindings))
	for _, binding := range bindings {
		if ReversalOf(reversals, binding.BindingID) == nil {
			active = append(active, binding)
		}
	}
	return active
}

// GuardBindChannel validates the command material of the consent handoff:
// the method dictionary, the bounded purpose, the subject handle and the
// evidence requirement. The duplicate review path validates through its own
// resolution command (the purpose is the decision itself).
func GuardBindChannel(method, purpose, subjectHandle string) error {
	if !inDictionary(BindingMethods, method) {
		return fmt.Errorf("метод связывания %s не входит в словарь (%s)", method, joinQuoted(BindingMethods))
	}
	if utf8.RuneCountInString(purpose) > MaxPurposeRunes {
		return fmt.Errorf("цель связывания длиннее %d символов", MaxPurposeRunes)
	}
	if method == BindingMethodConsentHandoff {
		if strings.TrimSpace(purpose) == "" {
			return fmt.Errorf("связывание по согласию требует цель (зачем клиент продолжает в другом канале)")
		}
	}
	handle := strings.TrimSpace(subjectHandle)
	if handle == "" {
		return fmt.Errorf("подтверждённое имя участника обязательно (переносится из проверки)")
	}
	if utf8.RuneCountInString(handle) > MaxSubjectHandleRunes {
		return fmt.Errorf("подтверждённое имя участника длиннее %d символов", MaxSubjectHandleRunes)
	}
	return nil
}

// GuardUnlinkChannel validates the reversal reason.
func GuardUnlinkChannel(reason string) error {
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("причина отмены связи обязательна")
	}
	if utf8.RuneCountInString(reason) > MaxBindingReasonRunes {
		return fmt.Errorf("причина отмены связи длиннее %d символов", MaxBindingReasonRunes)
	}
	return nil
}

// BindingClientConfirmation is the deterministic confirmation text the
// operator relays to the client: WHAT continued (the service in this
// channel) — never the internal identifiers (no case numbers, no
// conversation numbers, no session keys, no subject handles).
func BindingClientConfirmation(method string) string {
	switch method {
	case BindingMethodConsentHandoff:
		return "Ваше обращение продолжается в этом канале: история сохранена, повторно рассказывать не нужно."
	case BindingMethodDuplicateReview:
		return "Ваше обращение объединено с предыдущим: история сохранена, повторно рассказывать не нужно."
	}
	return "Ваше обращение продолжается; история сохранена."
}
