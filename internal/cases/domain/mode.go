package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The conversation mode and the mandatory AI disclosure (Story 2.5)
// ---------------------------------------------------------------------------
//
// The mode of a conversation is an append-only term history: one row per
// transition with the material uniqueness (tenant, conversation, version).
// The CURRENT mode, the Work Item contexts and the disclosure state are
// read-time derivations — nothing is ever updated. The ladder is fixed:
// safety > claim > cancellation > privacy_no_contact > human_requested >
// unclassified > sales; every mode except sales is a non-sales path. A
// proposal can only RAISE the mode (GuardProposeMode refuses the downgrade
// and the equal repeat); the only way down is exit_mode with an authority
// and an honest reason — the system never returns to sales by the absence
// of a repeated signal. The effective mode of a Work Item is the STRICTER
// (max rank) of the conversation mode and the item's own context, so a
// priority mode stops the incompatible items while the independent work
// stays allowed by the explicit v1 rule. The AI disclosure is a record with
// its own append-only delivery events: the disclosure acts for a channel
// while its last event says sent or a playback word — an interruption
// voids it and the operator re-discloses with a parent reference.

// ---------------------------------------------------------------------------
// Dictionaries
// ---------------------------------------------------------------------------

// Conversation modes: the fixed ladder of the slice. The rank is the
// strictness order; everything except sales is a non-sales path.
const (
	ModeSafety           = "safety"
	ModeClaim            = "claim"
	ModeCancellation     = "cancellation"
	ModePrivacyNoContact = "privacy_no_contact"
	ModeHumanRequested   = "human_requested"
	ModeUnclassified     = "unclassified"
	ModeSales            = "sales"
)

// ModeOrder is the ladder as the spec writes it: the priority modes from
// the strictest to the loosest, unclassified above sales (an unclassified
// dialog blocks the sales work — fail-closed), sales at the tail. This is
// the DISPLAY order of the panel and the strictness order of the
// effective derivation (the earlier word blocks the later one).
var ModeOrder = []string{ModeSafety, ModeClaim, ModeCancellation, ModePrivacyNoContact, ModeHumanRequested, ModeUnclassified, ModeSales}

// ModeRank resolves the strictness position of a mode in the ladder: the
// SMALLER number is the STRICTER word (safety blocks everything; sales
// blocks nothing; unclassified is stricter than sales — the fail-closed
// word of an unknown case, AC-5). An unknown mode is a dictionary error
// BEFORE this function runs — the honest -1 is never a ladder word.
func ModeRank(mode string) int {
	for rank, word := range ModeOrder {
		if word == mode {
			return rank
		}
	}
	return -1
}

// proposalRank is the RAISE direction of the proposal ladder — the demo
// order of the owner decision: unclassified is the admission floor (the
// first classification — including sales — raises from it), the five
// priority modes sit above sales. The two orders differ ONLY in the
// unclassified position: as a STATE unclassified blocks sales (ModeRank),
// as a STARTING word it is what any signal raises from (proposalRank) —
// the exact shape of the owner-approved demo path (unclassified→sales is
// «выше», sales→safety is «строго выше», safety→sales is «понизить»).
func proposalRank(mode string) int {
	switch mode {
	case ModeUnclassified:
		return 0
	case ModeSales:
		return 1
	case ModeHumanRequested:
		return 2
	case ModePrivacyNoContact:
		return 3
	case ModeCancellation:
		return 4
	case ModeClaim:
		return 5
	case ModeSafety:
		return 6
	}
	return -1
}

// ModeIsNonSales reports whether the mode word is a non-sales path (every
// dictionary word except sales; the sales replies need the effective sales
// mode — AC-5).
func ModeIsNonSales(mode string) bool {
	return mode != ModeSales
}

// Term scopes: the conversation ladder and the per-Work-Item context.
const (
	ModeScopeConversation = "conversation"
	ModeScopeWorkItem     = "work_item"
)

// ModeScopes is the full dictionary.
var ModeScopes = []string{ModeScopeConversation, ModeScopeWorkItem}

// Term origins: the ingest initial term, the raised proposal and the
// supervised exit.
const (
	ModeOriginIngestInitial = "ingest_initial"
	ModeOriginProposal      = "proposal"
	ModeOriginExitOverride  = "exit_override"
)

// ModeOrigins is the full dictionary.
var ModeOrigins = []string{ModeOriginIngestInitial, ModeOriginProposal, ModeOriginExitOverride}

// Signal sources: the turn event of the client, the operator's own
// observation and the deterministic policy rule word.
const (
	ModeSignalTurnEvent  = "turn_event"
	ModeSignalOperator   = "operator"
	ModeSignalPolicyRule = "policy_rule"
)

// ModeSignalSources is the full dictionary.
var ModeSignalSources = []string{ModeSignalTurnEvent, ModeSignalOperator, ModeSignalPolicyRule}

// Exit authorities: the current owner with the review scope and the
// supervision word of the configuration scope.
const (
	ModeAuthorityOperatorOwner = "operator_owner"
	ModeAuthoritySupervisor    = "supervisor"
)

// ModeAuthorities is the full dictionary.
var ModeAuthorities = []string{ModeAuthorityOperatorOwner, ModeAuthoritySupervisor}

// The reply classes: the producer marks the class of every outgoing client
// reply — the machine never classifies the content (the honest boundary of
// the slice; the future autonomous surfaces pass through the same gates).
const (
	ReplyClassSales            = "sales"
	ReplyClassNeutral          = "neutral"
	ReplyClassProtectedRequest = "protected_request"
)

// ReplyClasses is the full dictionary.
var ReplyClasses = []string{ReplyClassSales, ReplyClassNeutral, ReplyClassProtectedRequest}

// The disclosure delivery kinds: one event — one kind; the states never
// mix. generated is the born word of the record (it does not act yet);
// sent is the honest send observation of the command; the playback words
// are NOT a proof the customer heard or understood anything; the
// interruption voids the acting disclosure until the operator re-discloses.
const (
	DeliveryGenerated         = "generated"
	DeliverySent              = "sent"
	DeliveryPlaybackAck       = "playback_acknowledged"
	DeliveryPlaybackEstimated = "playback_estimated"
	DeliveryCustomerConfirmed = "customer_confirmed"
	DeliveryInterrupted       = "interrupted"
)

// DeliveryKinds is the full dictionary.
var DeliveryKinds = []string{DeliveryGenerated, DeliverySent, DeliveryPlaybackAck, DeliveryPlaybackEstimated, DeliveryCustomerConfirmed, DeliveryInterrupted}

// EffectiveDeliveryKinds is the set of the statuses that keep a disclosure
// acting for its channel (the playback words act — they are never a proof
// of hearing or understanding; the customer-confirmed word has a producer
// in the dictionary only — the future policy decides its meaning).
var effectiveDeliveryKindSet = map[string]bool{
	DeliverySent: true, DeliveryPlaybackAck: true, DeliveryPlaybackEstimated: true, DeliveryCustomerConfirmed: true,
}

// DeliveryKindActs reports whether one delivery kind keeps the disclosure
// acting.
func DeliveryKindActs(kind string) bool {
	return effectiveDeliveryKindSet[kind]
}

// ModePolicyVersion is the fixed rule dictionary word every term carries
// (the deterministic policy of the slice).
const ModePolicyVersion = "mode-policy-v1"

// The material limits of the mode dictionary.
const (
	MaxModeReasonRunes     = 200
	MaxDisclosureTextRunes = 2000
)

// ---------------------------------------------------------------------------
// The mode material
// ---------------------------------------------------------------------------

// ModeTerm is one append-only mode transition: the version (the monotonic
// CAS counter of the conversation ladder), the mode word, the scope (the
// conversation or one Work Item), the origin, the honest reason, the
// signal source, the turn reference, the actor, the previous mode
// snapshot, the exit authority and the rule version.
type ModeTerm struct {
	TenantID       id.UUID
	CaseID         id.UUID
	ConversationID id.UUID
	TermID         id.UUID
	ModeVersion    int64
	Mode           string
	Scope          string
	WorkItemID     id.UUID
	Origin         string
	Reason         string
	SignalSource   string
	TurnRef        id.UUID
	ActorID        id.UUID
	PriorMode      string
	ExitAuthority  string
	RuleVersion    string
	CreatedAt      time.Time
}

// DisclosureRecord is one append-only AI disclosure: the exact text, the
// channel, the release lineage snapshot (the employee, the decision number
// and the profile version of the acting decision — the reply precedent),
// the actor and the parent reference of the re-disclosure.
type DisclosureRecord struct {
	TenantID       id.UUID
	CaseID         id.UUID
	ConversationID id.UUID
	DisclosureID   id.UUID
	DisclosureNo   int64
	Channel        string
	Text           string
	ParentRef      id.UUID
	EmployeeID     id.UUID
	DecisionNo     int64
	ProfileVersion int64
	ActorID        id.UUID
	CreatedAt      time.Time
}

// DisclosureDeliveryEvent is one append-only delivery observation of a
// disclosure record: the kind (one event — one kind), the per-record
// monotonic number (the last event is the record's status) and the actor.
type DisclosureDeliveryEvent struct {
	TenantID     id.UUID
	EventID      id.UUID
	DisclosureID id.UUID
	EventNo      int64
	Kind         string
	ActorID      id.UUID
	CreatedAt    time.Time
}

// OutboundBasis is the synthetic word of the basis of the FIRST outgoing
// partner contact: the partner source word, the purpose word and the
// consent word. The producers of the words are the future outgoing
// connectors (AG-4); the domain gate stays fail-closed without the full
// set — the positive branch is a unit proof over the synthetic words.
type OutboundBasis struct {
	PartnerSource string
	Purpose       string
	Consent       string
}

// ---------------------------------------------------------------------------
// Read-time derivations
// ---------------------------------------------------------------------------

// ActiveConversationMode resolves the term of the maximal version with the
// conversation scope (nil when the conversation has no terms — the honest
// pre-migration state; the gates treat it as the safe non-sales path).
func ActiveConversationMode(terms []ModeTerm) *ModeTerm {
	var active *ModeTerm
	for i := range terms {
		if terms[i].Scope != ModeScopeConversation {
			continue
		}
		if active == nil || terms[i].ModeVersion > active.ModeVersion {
			active = &terms[i]
		}
	}
	return active
}

// WorkItemModeContext resolves the term of the maximal version with the
// Work Item scope of ONE item (nil when the item carries no context).
func WorkItemModeContext(terms []ModeTerm, workItemID id.UUID) *ModeTerm {
	var context *ModeTerm
	for i := range terms {
		if terms[i].Scope != ModeScopeWorkItem || terms[i].WorkItemID != workItemID {
			continue
		}
		if context == nil || terms[i].ModeVersion > context.ModeVersion {
			context = &terms[i]
		}
	}
	return context
}

// EffectiveWorkMode derives the effective mode of one Work Item: the
// STRICTER word of the active conversation mode and the item's own context
// (the earlier ladder position — the conversation applies its strictest
// mode to the incompatible actions, AC-8). A missing side degrades to the
// present side; both missing is the honest pre-migration absence (the
// gates treat it fail-closed as the non-sales path). The independent
// work stays allowed by the explicit v1 rule — the neutral replies pass
// under any mode.
func EffectiveWorkMode(active *ModeTerm, context *ModeTerm) *ModeTerm {
	if active == nil {
		return context
	}
	if context == nil {
		return active
	}
	if ModeRank(context.Mode) < ModeRank(active.Mode) {
		return context
	}
	return active
}

// EffectiveWorkModeWord is the fail-closed word of the effective mode for
// the reply gate: the derivation decides; the pre-migration absence
// degrades to the unclassified semantics (the safe non-sales path — a
// sales reply refuses honestly, AC-5).
func EffectiveWorkModeWord(effective *ModeTerm) string {
	if effective == nil {
		return ModeUnclassified
	}
	return effective.Mode
}

// ModeTermMaxVersion resolves the maximal version of the conversation
// ladder regardless of the scope (the CAS word of the propose/exit
// commands — every term of the conversation consumes one version).
func ModeTermMaxVersion(terms []ModeTerm) int64 {
	var max int64
	for i := range terms {
		if terms[i].ModeVersion > max {
			max = terms[i].ModeVersion
		}
	}
	return max
}

// DisclosureStatus derives the status of one disclosure record: the kind
// of its LAST delivery event (the maximal event number; the empty word
// when no event exists — a record is born with its generated event in the
// same command, so the honest absence means the pre-migration material).
func DisclosureStatus(events []DisclosureDeliveryEvent, disclosureID id.UUID) string {
	var last *DisclosureDeliveryEvent
	for i := range events {
		if events[i].DisclosureID != disclosureID {
			continue
		}
		if last == nil || events[i].EventNo > last.EventNo {
			last = &events[i]
		}
	}
	if last == nil {
		return ""
	}
	return last.Kind
}

// EffectiveDisclosure derives the acting disclosure of one channel of a
// conversation: the LAST record of the channel (the maximal disclosure
// number) — it acts while its own status is a delivery kind that acts
// (sent or a playback word). An interruption or a freshly generated record
// voids the channel until the operator re-discloses with a parent
// reference and sends it (AC-2); an earlier sent record NEVER re-acts
// after a later interruption — the last word wins, the honest repeat is
// the re-disclosure.
func EffectiveDisclosure(records []DisclosureRecord, events []DisclosureDeliveryEvent, conversationID id.UUID, channel string) *DisclosureRecord {
	var last *DisclosureRecord
	for i := range records {
		if records[i].ConversationID != conversationID || records[i].Channel != channel {
			continue
		}
		if last == nil || records[i].DisclosureNo > last.DisclosureNo {
			last = &records[i]
		}
	}
	if last == nil || !DeliveryKindActs(DisclosureStatus(events, last.DisclosureID)) {
		return nil
	}
	return last
}

// ---------------------------------------------------------------------------
// Command guards
// ---------------------------------------------------------------------------

// GuardMode validates the mode dictionary of a command.
func GuardMode(mode string) error {
	if ModeRank(mode) < 0 {
		return fmt.Errorf("режим %s не входит в словарь (%s)", mode, joinQuoted(ModeOrder))
	}
	return nil
}

// GuardModeScope validates the scope dictionary.
func GuardModeScope(scope string) error {
	if !inDictionary(ModeScopes, scope) {
		return fmt.Errorf("скоуп режима %s не входит в словарь (%s)", scope, joinQuoted(ModeScopes))
	}
	return nil
}

// GuardModeReason validates the bounded honest reason.
func GuardModeReason(reason string) error {
	if utf8.RuneCountInString(reason) > MaxModeReasonRunes {
		return fmt.Errorf("причина режима длиннее %d символов", MaxModeReasonRunes)
	}
	return nil
}

// GuardSignalSource validates the signal source dictionary and the turn
// reference discipline: a turn_event signal names its accepted event; the
// operator and the policy words carry no turn.
func GuardSignalSource(source string, turnRef id.UUID) error {
	if !inDictionary(ModeSignalSources, source) {
		return fmt.Errorf("источник сигнала %s не входит в словарь (%s)", source, joinQuoted(ModeSignalSources))
	}
	if source == ModeSignalTurnEvent && !turnRef.Valid() {
		return fmt.Errorf("сигнал turn_event обязан ссылаться на событие кейса (turn_ref)")
	}
	if source != ModeSignalTurnEvent && turnRef.Valid() {
		return fmt.Errorf("turn_ref допустим только при источнике turn_event")
	}
	return nil
}

// GuardExpectedModeVersion validates the CAS expectation of the mode
// transition commands (>= 1).
func GuardExpectedModeVersion(expected int64) error {
	if expected < 1 {
		return fmt.Errorf("expected_mode_version обязателен (>= 1): команда режима сверяет текущую версию лестницы")
	}
	return nil
}

// GuardProposeModeMaterial validates the full command material of the
// proposal: the dictionaries, the scope discipline (a Work Item context
// needs the item reference; a conversation term carries none) and the
// bounded reason.
func GuardProposeModeMaterial(scope, mode, reason string, workItemID id.UUID) error {
	if err := GuardModeScope(scope); err != nil {
		return err
	}
	if err := GuardMode(mode); err != nil {
		return err
	}
	if scope == ModeScopeWorkItem && !workItemID.Valid() {
		return fmt.Errorf("скоуп work_item требует work_item_id: контекст привязан к элементу работы")
	}
	if scope == ModeScopeConversation && workItemID.Valid() {
		return fmt.Errorf("скоуп conversation не принимает work_item_id")
	}
	return GuardModeReason(reason)
}

// GuardProposeTransition is the deterministic ladder decision of the
// proposal (mode-policy-v1): a proposal can only RAISE (the proposal
// ladder — the admission floor unclassified, the five priority modes
// above sales). Against the conversation scope the active mode decides;
// against the Work Item scope the item's current context decides (a first
// context is free — it lowers nothing). The equal word refuses as the
// honest repeat (a repeated signal is not a transition); the lower word
// refuses as the downgrade no surface can ever drive (AC-4/AC-7).
func GuardProposeTransition(scope, mode string, active, context *ModeTerm) error {
	var against *ModeTerm
	switch scope {
	case ModeScopeConversation:
		against = active
	case ModeScopeWorkItem:
		against = context
	}
	if against == nil {
		return nil
	}
	switch {
	case proposalRank(mode) > proposalRank(against.Mode):
		return nil
	case proposalRank(mode) == proposalRank(against.Mode):
		return WorkRuleError{Rule: "propose_mode", Reason: ModeActiveText}
	default:
		return WorkRuleError{Rule: "propose_mode", Reason: ModeNotDowngradableText}
	}
}

// GuardExitModeMaterial validates the full command material of the exit:
// the dictionaries, the bounded mandatory reason and the exit authority
// word.
func GuardExitModeMaterial(mode, reason, authority string) error {
	if err := GuardMode(mode); err != nil {
		return err
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("причина выхода обязательна (до %d символов)", MaxModeReasonRunes)
	}
	if err := GuardModeReason(reason); err != nil {
		return err
	}
	if !inDictionary(ModeAuthorities, authority) {
		return fmt.Errorf("полномочие выхода %s не входит в словарь (%s)", authority, joinQuoted(ModeAuthorities))
	}
	return nil
}

// GuardExitTransition is the deterministic ladder decision of the exit:
// the exit LOWERS the conversation mode on the proposal ladder (the
// admission floor unclassified, the priority modes above sales) — a higher
// or equal target is not an exit (the proposal is the raising path). The
// authority ladder itself is the app's server-side word (the actor's
// scopes and the ownership state decide — never the command claim).
func GuardExitTransition(mode string, active *ModeTerm) error {
	if active == nil {
		return WorkRuleError{Rule: "exit_mode", Reason: ModeNoLadderText}
	}
	switch {
	case proposalRank(mode) < proposalRank(active.Mode):
		return nil
	case proposalRank(mode) == proposalRank(active.Mode):
		return WorkRuleError{Rule: "exit_mode", Reason: ModeActiveText}
	default:
		return WorkRuleError{Rule: "exit_mode", Reason: ExitRaisesText}
	}
}

// ExitAuthorityAllowed resolves the authority ladder of the exit on the
// server side: the safety mode is exit-able by the supervision word only
// (the platform safety is never weakened by an operator override — AC-9);
// every other mode is exit-able by the current owner with the review
// scope or the supervision. The owner word needs the CURRENT ownership
// term (the 2-4 derivation — the owner of the moment, never the command
// claim).
func ExitAuthorityAllowed(activeMode string, isCurrentOwner, hasSupervision bool) bool {
	if activeMode == ModeSafety {
		return hasSupervision
	}
	return isCurrentOwner || hasSupervision
}

// GuardReplyClass validates the reply class dictionary of the client
// reply: the producer marks the class — the machine never classifies the
// content (no silent default; the empty word refuses honestly).
func GuardReplyClass(class string) error {
	if !inDictionary(ReplyClasses, class) {
		return fmt.Errorf("класс ответа %s не входит в словарь (%s): класс размечает продюсер", class, joinQuoted(ReplyClasses))
	}
	return nil
}

// GuardReplyClassAgainstMode is the mode gate of the client reply
// (mode-policy-v1): the sales class needs the effective sales mode of the
// Work Item — a non-sales effective mode (or the fail-closed pre-migration
// absence) refuses with the machine-readable words; the neutral class
// passes by the explicit v1 rule (the safe independent work under a
// restrictive mode — AC-8); the protected class is a disclosure matter,
// never a mode matter.
func GuardReplyClassAgainstMode(class string, effective *ModeTerm) error {
	if class != ReplyClassSales {
		return nil
	}
	if effective != nil && effective.Mode == ModeSales {
		return nil
	}
	refusal := ReplyModeError{EffectiveMode: EffectiveWorkModeWord(effective)}
	if effective != nil {
		refusal.ModeReason = effective.Reason
		refusal.SignalSource = effective.SignalSource
		refusal.RuleVersion = effective.RuleVersion
	}
	if refusal.RuleVersion == "" {
		refusal.RuleVersion = ModePolicyVersion
	}
	return refusal
}

// GuardDisclosureText validates the bounded disclosure text: the exact
// words the customer sees, never empty, never over the limit.
func GuardDisclosureText(text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("текст раскрытия обязателен")
	}
	if utf8.RuneCountInString(text) > MaxDisclosureTextRunes {
		return fmt.Errorf("текст раскрытия длиннее %d символов", MaxDisclosureTextRunes)
	}
	return nil
}

// GuardDisclosureChannel validates the channel of a disclosure record
// against the profile channel dictionary (the 2-1 word).
func GuardDisclosureChannel(channel string) error {
	if !AllowedChannels[channel] {
		return fmt.Errorf("канал %s не входит в словарь каналов", channel)
	}
	return nil
}

// GuardDeliveryKind validates the delivery kind dictionary of the
// disclosure event command.
func GuardDeliveryKind(kind string) error {
	if !inDictionary(DeliveryKinds, kind) {
		return fmt.Errorf("событие доставки %s не входит в словарь (%s)", kind, joinQuoted(DeliveryKinds))
	}
	return nil
}

// GuardOutboundBasis is the fail-closed gate of the FIRST outgoing
// partner contact: without the full basis word (the partner source, the
// purpose, the consent) the contact is blocked (AC-3). The words are
// synthetic in this slice — the real producers are the outgoing connectors
// (AG-4); the positive branch is a unit proof over the synthetic words
// (the evidence-gate precedent of 2-4).
func GuardOutboundBasis(basis OutboundBasis) bool {
	return strings.TrimSpace(basis.PartnerSource) != "" &&
		strings.TrimSpace(basis.Purpose) != "" &&
		strings.TrimSpace(basis.Consent) != ""
}

// OutboundBasisNotEvidencedText is the honest fail-closed refusal.
const OutboundBasisNotEvidencedText = "Исходящий партнёрский контакт без полного основания (источник заявки, цель, согласие) заблокирован: механика оснований fail-closed."

// ---------------------------------------------------------------------------
// The machine-readable refusals
// ---------------------------------------------------------------------------

// ReplyModeError is the machine-readable refusal of the reply mode gate:
// the effective mode word of the Work Item the reply belongs to WITH the
// honest words of the term that governs it (the reason, the signal source
// and the rule version — the Approach(4) snapshot: active_mode, reason,
// signal_source, rule_version, permitted_path). The permitted path word
// is fixed (exit_mode).
type ReplyModeError struct {
	EffectiveMode string
	ModeReason    string
	SignalSource  string
	RuleVersion   string
	Reason        string
}

func (e ReplyModeError) Error() string { return e.Reason }

// ReplyModeGateText is the fixed refusal text of the mode gate.
const ReplyModeGateText = "Продажные ответы недоступны в текущем режиме: эффективный режим элемента работы — несбытовой. Разрешённый путь — явный выход (exit_mode) с полномочием и причиной."

// ModeNotDowngradableText is the honest refusal of the downgrade proposal:
// no surface can ever lower the mode by proposing (AC-4/AC-7).
const ModeNotDowngradableText = "Режим нельзя понизить предложением: понижение невозможно как класс (GuardProposeMode отказывает всегда); выход — только явный exit_mode с полномочием и причиной."

// ModeActiveText is the honest refusal of the equal proposal: a repeated
// signal is not a transition.
const ModeActiveText = "Режим уже активен: повтор сигнала не создаёт переход."

// ExitRaisesText is the honest refusal of an exit that targets a higher
// mode: the exit is the lowering path; the raising path is the proposal.
const ExitRaisesText = "Выход понижает режим: повышение или смену через exit_mode невозможны — используйте propose_mode."

// ModeNoLadderText is the honest refusal of an exit without the ladder
// (the pre-migration absence).
const ModeNoLadderText = "Лестница режимов пуста (до-миграционные данные): выхода не существует."

// ModeConflictText is the honest refusal of the raced transition: the
// ladder moved under the command.
const ModeConflictText = "Версия лестницы режимов изменилась: перечитайте текущий режим и повторите команду."

// DisclosureGateText is the fixed refusal text of the disclosure gate: the
// sales and the protected replies need the acting disclosure of the
// channel first (AC-1).
const DisclosureGateText = "Действующего AI-раскрытия для канала нет: продажи и запросы защищённых данных требуют раскрытия до них. Разрешённый путь — record_disclosure и событие sent."

// ReplyClassRequiredText is the honest refusal of the missing class: the
// producer marks the class — there is no silent default.
const ReplyClassRequiredText = "Класс ответа обязателен (sales|neutral|protected_request): класс размечает продюсер, машина контент не классифицирует."

// TurnRefNotFoundText is the honest refusal of the unresolvable turn
// reference.
const TurnRefNotFoundText = "Событие turn_ref не найдено в обращении: сигнал обязан ссылаться на принятый turn кейса."

// ModeConflictError is the machine-readable raced-transition refusal: the
// CURRENT mode words travel with the conflict (the honest 409 of the
// concurrent proposals — the loser re-reads the state).
type ModeConflictError struct {
	ActiveMode   string
	Reason       string
	SignalSource string
	RuleVersion  string
	ModeVersion  int64
}

func (e ModeConflictError) Error() string { return ModeConflictText }

// ModeExitAuthorityText is the honest refusal of the authority ladder: the
// platform safety is never weakened by an operator override.
const ModeExitAuthorityText = "Выход из режима требует полномочия: safety — только надзор (configuration); остальные — текущий владелец с review либо configuration."

// ProposeTargetAllowed resolves the honest form option: the propose
// targets are the words a raise may reach (strictly above the active mode
// on the proposal ladder). The server refuses the rest anyway — the form
// only offers the honest paths.
func ProposeTargetAllowed(target, active string) bool {
	return proposalRank(target) > proposalRank(active)
}

// ExitTargetAllowed resolves the honest form option: the exit targets are
// the words a supervised exit may reach (strictly below the active mode).
func ExitTargetAllowed(target, active string) bool {
	return proposalRank(target) < proposalRank(active)
}

// InitialModeTerm builds the ingest initial term: version 1, the
// unclassified mode (the 2-1 canon «не классифицирован» — the safe
// non-sales path of an unknown case, AC-5), the ingest origin, the
// operator signal word (the admission itself), the rule version. The
// ladder, the dictionaries and the step order of 2-1 are untouched — this
// is exactly one INSERT of the same admission transaction (the ownership
// initial-term precedent).
func InitialModeTerm(tenantID, caseID, conversationID, termID, actorID id.UUID, now time.Time) ModeTerm {
	return ModeTerm{
		TenantID: tenantID, CaseID: caseID, ConversationID: conversationID, TermID: termID,
		ModeVersion: 1, Mode: ModeUnclassified, Scope: ModeScopeConversation,
		Origin: ModeOriginIngestInitial, Reason: "Запрос ещё не классифицирован.",
		SignalSource: ModeSignalOperator, ActorID: actorID,
		PriorMode: "", RuleVersion: ModePolicyVersion, CreatedAt: now,
	}
}

// ---------------------------------------------------------------------------
// The mode disclosure refusal shape (the problem fields)
// ---------------------------------------------------------------------------

// ReplyDisclosureError is the machine-readable refusal of the disclosure
// gate: the channel of the reply has no acting disclosure.
type ReplyDisclosureError struct {
	Channel string
	Reason  string
}

func (e ReplyDisclosureError) Error() string { return e.Reason }

// The permitted path words of the machine-readable refusals.
const (
	PermittedPathExit             = "exit_mode"
	PermittedPathRecordDisclosure = "record_disclosure"
)

// ---------------------------------------------------------------------------
// The outbox payloads
// ---------------------------------------------------------------------------

// ModeChangedPayload is the safe canonical JSON of the mode change event:
// the term snapshot — the identifiers, the version, the mode, the scope,
// the reason, the signal source and the actor. The disclosure TEXT never
// enters any envelope (the mode reason is the bounded honest dictionary
// word of the term, not a case content).
type ModeChangedPayload struct {
	TenantID       id.UUID   `json:"tenant_id"`
	CaseID         id.UUID   `json:"case_id"`
	ConversationID id.UUID   `json:"conversation_id"`
	TermID         id.UUID   `json:"term_id"`
	ModeVersion    int64     `json:"mode_version"`
	Mode           string    `json:"mode"`
	Scope          string    `json:"scope"`
	Reason         string    `json:"reason"`
	SignalSource   string    `json:"signal_source"`
	ActorID        id.UUID   `json:"actor_id"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// DisclosureRecordedPayload is the safe canonical JSON of the disclosure
// record event: the identifiers, the channel and the actor. The disclosure
// TEXT never enters the envelope — it lives in the record only.
type DisclosureRecordedPayload struct {
	TenantID       id.UUID   `json:"tenant_id"`
	CaseID         id.UUID   `json:"case_id"`
	ConversationID id.UUID   `json:"conversation_id"`
	DisclosureID   id.UUID   `json:"disclosure_id"`
	DisclosureNo   int64     `json:"disclosure_no"`
	Channel        string    `json:"channel"`
	ActorID        id.UUID   `json:"actor_id"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// The outbox event types of the mode surface (each v1).
const (
	OutboxEventModeChanged          = "case.mode_changed"
	OutboxEventModeChangedV1        = 1
	OutboxEventDisclosureRecorded   = "case.disclosure_recorded"
	OutboxEventDisclosureRecordedV1 = 1
)

// The dedup keys of the mode outbox records.
func DedupKeyModeChanged(termID id.UUID) string {
	return "case.mode_changed:" + termID.String()
}

func DedupKeyDisclosureRecorded(disclosureID id.UUID) string {
	return "case.disclosure_recorded:" + disclosureID.String()
}

// ModeChangedPayloadJSON renders the safe canonical JSON.
func ModeChangedPayloadJSON(payload ModeChangedPayload) ([]byte, error) {
	return json.Marshal(payload)
}

// DisclosureRecordedPayloadJSON renders the safe canonical JSON.
func DisclosureRecordedPayloadJSON(payload DisclosureRecordedPayload) ([]byte, error) {
	return json.Marshal(payload)
}
