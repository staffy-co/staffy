package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Work Items, facts and provenance (Story 2.2, first slice)
// ---------------------------------------------------------------------------
//
// Versions are the truth: every command writes one append-only
// work_item_versions row; the current state/type/title is the derivation of
// the maximal version (nothing is ever updated). Facts are append-only
// material keyed by the case and a normalized material key; their values
// carry the full provenance; confirmations bind to an EXACT version; the
// conflicting flag is a read-time derivation — never a stored status.

// ---------------------------------------------------------------------------
// Work item state ladder
// ---------------------------------------------------------------------------

// Work item states: the ladder of the slice. untriaged is the honest state
// of the initial work item written by the ingress; in_work is the
// classified state; waiting is the honest pause; confirmed (2.9) is the
// machine step of the exact-version commitment confirmation — the work
// item holds a confirmed obligation, the execution contour is Epic 3;
// resolved and cancelled are
// the operator completions; split and merged are the terminal states of the
// split/merge commands. A completion of one work item never closes the
// conversation or the case.
const (
	WorkItemStateUntriaged = "untriaged"
	WorkItemStateInWork    = "in_work"
	WorkItemStateWaiting   = "waiting"
	WorkItemStateConfirmed = "confirmed"
	WorkItemStateResolved  = "resolved"
	WorkItemStateCancelled = "cancelled"
	WorkItemStateSplit     = "split"
	WorkItemStateMerged    = "merged"
)

// WorkItemStates is the full dictionary.
var WorkItemStates = []string{
	WorkItemStateUntriaged, WorkItemStateInWork, WorkItemStateWaiting,
	WorkItemStateConfirmed, WorkItemStateResolved, WorkItemStateCancelled,
	WorkItemStateSplit, WorkItemStateMerged,
}

// TerminalWorkItemStates are the states no further command may mutate: the
// split/merge terminal kinds and the operator completions.
var TerminalWorkItemStates = map[string]bool{
	WorkItemStateResolved: true, WorkItemStateCancelled: true,
	WorkItemStateSplit: true, WorkItemStateMerged: true,
}

// transitions is the state ladder: the allowed transition targets of every
// non-terminal state. untriaged is not on the transition command ladder at
// all — it leaves its state only through classification.
var transitions = map[string][]string{
	WorkItemStateInWork:  {WorkItemStateWaiting, WorkItemStateResolved, WorkItemStateCancelled, WorkItemStateConfirmed},
	WorkItemStateWaiting: {WorkItemStateInWork, WorkItemStateResolved, WorkItemStateCancelled, WorkItemStateConfirmed},
	// confirmed (2.9) is the machine step of the exact-version confirmation: the
	// exits stay the operator completions only (no split/merge of a confirmed
	// item in this slice — the honest boundary; the action continuation is Epic 3).
	WorkItemStateConfirmed: {WorkItemStateResolved, WorkItemStateCancelled},
}

// TransitionAllowed reports whether the transition command may move a work
// item from one state to another. Terminal states never transition; the
// untriaged state is classified, not transitioned.
func TransitionAllowed(from, to string) bool {
	if TerminalWorkItemStates[from] || from == WorkItemStateUntriaged {
		return false
	}
	for _, candidate := range transitions[from] {
		if candidate == to {
			return true
		}
	}
	return false
}

// ClassifyAllowedFrom reports whether classification may run from the
// current state: the initial classification (untriaged → in_work) and the
// re-classification of an active item (the type/title change is a new
// version; the state stays). Terminal states are never re-classified.
func ClassifyAllowedFrom(state string) bool {
	switch state {
	case WorkItemStateUntriaged, WorkItemStateInWork, WorkItemStateWaiting:
		return true
	}
	return false
}

// RescopeAllowedFrom reports whether a work item may take part in split,
// merge or fact rescoping: every non-terminal state qualifies.
func RescopeAllowedFrom(state string) bool {
	return !TerminalWorkItemStates[state]
}

// ---------------------------------------------------------------------------
// Version kinds (the honest vocabulary of the append-only ladder)
// ---------------------------------------------------------------------------

// Version kinds: which command wrote the row. create is the version 1 of
// every work item (the ingress, a split target or an operator-created
// intent); classify carries the type/title and the in_work state;
// transition carries the new state; split is the terminal version of the
// split source (ref = the first target); merge is the terminal version of
// a merge source (ref = merged_into target).
const (
	VersionKindCreate     = "create"
	VersionKindClassify   = "classify"
	VersionKindTransition = "transition"
	VersionKindSplit      = "split"
	VersionKindMerge      = "merge"
)

// VersionKinds is the full dictionary.
var VersionKinds = []string{VersionKindCreate, VersionKindClassify, VersionKindTransition, VersionKindSplit, VersionKindMerge}

// WorkItemVersion is one append-only ladder row: the command kind, the
// material it set (type, title, state), the actor, the reason, the
// reference and the time. The zero actor id means the system (the ingress
// wrote the version 1 of the initial work item); every operator command
// carries the acting principal.
type WorkItemVersion struct {
	TenantID       id.UUID
	CaseID         id.UUID
	ConversationID id.UUID
	WorkItemID     id.UUID
	VersionNo      int64
	Kind           string
	Type           string // empty when the row does not set the type
	Title          string // empty when the row does not set the title
	State          string // the state AFTER the command
	Reason         string
	ActorID        id.UUID // zero = the system (ingress)
	Ref            id.UUID // split: the first target; create-from-split: the source; merge: merged_into
	CreatedAt      time.Time
}

// WorkItemCurrent is the derived projection of one work item: the maximal
// version with the material it set (type/title travel with the
// create/classify rows; the state is the state of the maximal version).
type WorkItemCurrent struct {
	WorkItemID     id.UUID
	CaseID         id.UUID
	ConversationID id.UUID
	Version        int64
	Type           string
	Title          string
	State          string
	Kind           string
	Reason         string
	ActorID        id.UUID
	Ref            id.UUID
	Terminal       bool
	CreatedAt      time.Time
}

// RecommendedNextAction is the fixed dictionary of the Work Control Card:
// one honest action per state. It is never a guess and never an AI output.
func RecommendedNextAction(state string) string {
	switch state {
	case WorkItemStateUntriaged:
		return "Классифицируйте запрос: выберите тип из словаря пака и название."
	case WorkItemStateInWork:
		return "Запишите факты и проверьте подтверждения."
	case WorkItemStateWaiting:
		return "Проверьте условие возврата клиента."
	case WorkItemStateConfirmed:
		return "Обязательство подтверждено: исполнение вне системы (контур исполнений — будущие срезы)."
	case WorkItemStateResolved:
		return "Работа завершена; история доступна."
	case WorkItemStateCancelled:
		return "Работа отменена; история доступна."
	case WorkItemStateSplit:
		return "Элемент разделён; продолжайте работу в карточках-целях."
	case WorkItemStateMerged:
		return "Элемент объединён; продолжайте работу в карточке-цели."
	}
	return ""
}

// ---------------------------------------------------------------------------
// Fact provenance dictionaries
// ---------------------------------------------------------------------------

// Fact source types: channel_event (the material of an accepted event of
// this case) and operator (the operator's own knowledge with the command
// key as the source). connector and system are RESERVED dictionary values
// without a producer in this slice — the domain refuses them honestly;
// future write-path connectors will arrive with their own stories.
const (
	FactSourceChannelEvent = "channel_event"
	FactSourceOperator     = "operator"
)

// FactSourceTypes is the reserved dictionary (the order is the dictionary
// order; the last two have no producers).
var FactSourceTypes = []string{FactSourceChannelEvent, FactSourceOperator, "connector", "system"}

// FactSourceProducers are the source types the slice may actually write.
var FactSourceProducers = map[string]bool{FactSourceChannelEvent: true, FactSourceOperator: true}

// Confirmer types of the fact confirmations: client and owning_system are
// reserved dictionary values without a producer (the outgoing dialogs and
// the write-path connectors of the later stories will arrive with them);
// the only producer of this slice is operator_relay — the operator fixes
// the words of the client with an evidence reference.
const ConfirmerOperatorRelay = "operator_relay"

// ConfirmerTypes is the reserved dictionary.
var ConfirmerTypes = []string{"client", "owning_system", ConfirmerOperatorRelay}

// FactChannels is the channel dictionary of the fact provenance: the nine
// profile channels (a channel_event fact inherits the channel of its
// event) plus the operator pseudo-channel of the operator-recorded facts
// and the protected-continuation pseudo-channel of the client-submitted
// corrections (the +1 word of story 2.10 — the provenance channel of the
// protected continuation surface).
var FactChannels = append([]string{"operator", FactChannelProtectedContinuation}, ChannelOrder...)

// Fact material limits (the fixed dictionary of the slice).
const (
	MaxMaterialKeyRunes = 64
	MaxFactValueBytes   = 2048
	MaxNoteRunes        = 200
	MaxWorkTitleRunes   = 200
	MaxReasonRunes      = 500
)

// Fact statuses (read-time derivations, never stored).
const (
	FactStatusConfirmed   = "confirmed"
	FactStatusConflicting = "conflicting"
	FactStatusUnverified  = "unverified"
)

// ---------------------------------------------------------------------------
// Facts: the anchor, the versions and the confirmations
// ---------------------------------------------------------------------------

// FactAnchor is one fact of one case: the material key is unique per case;
// every value change writes a new fact version, never an update.
type FactAnchor struct {
	TenantID    id.UUID
	CaseID      id.UUID
	FactID      id.UUID
	MaterialKey string
	CreatedAt   time.Time
}

// FactVersion is one append-only value with the full provenance: the
// scope (the work item the version belongs to), the canonical value bytes
// with the digest, the source, the channel, the actor and both times
// (observed is the provider time of the event, never the receive time).
type FactVersion struct {
	TenantID    id.UUID
	CaseID      id.UUID
	FactID      id.UUID
	WorkItemID  id.UUID // the scope of this version
	VersionNo   int64
	MaterialKey string
	Value       []byte // canonical JSON bytes
	ValueDigest string
	SourceType  string
	SourceID    string
	Channel     string
	ActorID     id.UUID
	ObservedAt  time.Time
	ReceivedAt  time.Time
	CreatedAt   time.Time
}

// FactConfirmation is one append-only confirmation bound to the EXACT
// version: the confirmer type, the evidence (an accepted event of the case
// or a note — exactly one of them carries the words), the acting operator
// and the time. A confirmation never rewrites a version; a new value is a
// new version.
type FactConfirmation struct {
	TenantID        id.UUID
	CaseID          id.UUID
	FactID          id.UUID
	ConfirmationID  id.UUID
	VersionNo       int64
	ConfirmerType   string
	EvidenceEventID id.UUID // zero when the note carries the words
	Note            string
	ActorID         id.UUID
	CreatedAt       time.Time
}

// NormalizeMaterialKey trims and lowercases the material key: the facts of
// one case are keyed by the normalized spelling. The dictionary limits the
// key to 64 non-space runes; the domain does not limit the vocabulary (the
// facts are arbitrary by FR-10; the pack hints CriticalEntities in the UI).
func NormalizeMaterialKey(raw string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		return "", fmt.Errorf("материальный ключ факта обязателен")
	}
	if utf8.RuneCountInString(key) > MaxMaterialKeyRunes {
		return "", fmt.Errorf("материальный ключ длиннее %d символов", MaxMaterialKeyRunes)
	}
	for _, r := range key {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return "", fmt.Errorf("материальный ключ не содержит пробелов и табуляций")
		}
	}
	return key, nil
}

// FactValue canonicalizes and bounds the fact value: the same
// UseNumber discipline as the ingress payload (the numeric precision is
// material content; 1 and 1.0 are different values), the digest over the
// canonical bytes, the 2 KiB dictionary bound.
//
// The jsonb number discipline: exponential literals (1e10) are legal JSON
// but the PostgreSQL jsonb EXPANDS them into the digit form on storage —
// the canonical bytes (the digest material) and the stored rendering
// would diverge, and a dense exponential array would outrun the CHECK
// backstop of the migration. The fact values of the slice accept the
// digit-form numbers only; the operator rewrites 1e10 as 10000000000 (an
// honest rule refusal, never a 500 on the CHECK).
func FactValue(raw []byte) ([]byte, string, error) {
	canonical, err := CanonicalPayload(raw)
	if err != nil {
		return nil, "", fmt.Errorf("значение факта должно быть одним JSON-значением: %w", err)
	}
	if len(canonical) > MaxFactValueBytes {
		return nil, "", fmt.Errorf("значение факта длиннее %d байт канонического JSON", MaxFactValueBytes)
	}
	if err := assertJsonbStableNumbers(canonical); err != nil {
		return nil, "", err
	}
	return canonical, PayloadDigest(canonical), nil
}

// jsonbNumberLiteral is the digit form of a JSON number — the only literal
// whose jsonb rendering never expands: the PostgreSQL numeric preserves
// every digit and the decimal scale, so the stored text matches the
// canonical bytes (the -0 → 0 shrink is the single safe exception).
var jsonbNumberLiteral = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// assertJsonbStableNumbers walks the decoded canonical value and refuses
// the exponential literals (and any other number spelling whose jsonb
// rendering would diverge from the canonical bytes).
func assertJsonbStableNumbers(canonical []byte) error {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("значение факта должно быть одним JSON-значением: %w", err)
	}
	return walkJsonbStableNumbers(value)
}

func walkJsonbStableNumbers(value any) error {
	switch typed := value.(type) {
	case json.Number:
		if !jsonbNumberLiteral.MatchString(typed.String()) {
			return fmt.Errorf("число %s записано не в цифровой форме: значение факта не принимает экспоненциальную запись (перезапишите, например, 1e10 как 10000000000)", typed.String())
		}
	case []any:
		for _, item := range typed {
			if err := walkJsonbStableNumbers(item); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range typed {
			if err := walkJsonbStableNumbers(item); err != nil {
				return err
			}
		}
	}
	return nil
}

// FactSourceChannel resolves the provenance channel: a channel_event
// source inherits the channel of the event it cites; an operator source
// lives on the operator pseudo-channel of the dictionary.
func FactSourceChannel(sourceType, eventChannel string) (string, error) {
	if !FactSourceProducers[sourceType] {
		return "", fmt.Errorf("источник факта %s зарезервирован словарём без продюсера в этом срезе", sourceType)
	}
	switch sourceType {
	case FactSourceChannelEvent:
		if !AllowedChannels[eventChannel] {
			return "", fmt.Errorf("канал %s не входит в словарь каналов профиля", eventChannel)
		}
		return eventChannel, nil
	default: // operator
		return "operator", nil
	}
}

// ---------------------------------------------------------------------------
// Fact status derivation (read-time, never stored)
// ---------------------------------------------------------------------------

// DeriveFactStatus resolves the honest status of one fact from its
// versions (ordered by version_no) and its confirmations:
//
//   - confirmed: the latest version carries the same digest as the latest
//     CONFIRMED version (a confirmation of that exact material exists);
//   - conflicting: the latest version differs from the latest confirmed
//     version, OR there are no confirmations and at least two distinct
//     digests are held;
//   - unverified: one value without a confirmation.
//
// There is no automatic value choice: the resolution of a conflict is the
// human confirmation of an exact version; the authority rules (the Owning
// System / State-Authority Matrix) are honestly deferred.
func DeriveFactStatus(versions []FactVersion, confirmations []FactConfirmation) (string, string) {
	if len(versions) == 0 {
		return FactStatusUnverified, ""
	}
	latest := versions[len(versions)-1]
	latestConfirmedDigest := ""
	latestConfirmedNo := int64(-1)
	for _, confirmation := range confirmations {
		if confirmation.VersionNo > latestConfirmedNo {
			latestConfirmedNo = confirmation.VersionNo
			latestConfirmedDigest = factDigestOf(versions, confirmation.VersionNo)
		}
	}
	if latestConfirmedNo >= 0 {
		if latest.ValueDigest == latestConfirmedDigest {
			return FactStatusConfirmed, latestConfirmedDigest
		}
		return FactStatusConflicting, latestConfirmedDigest
	}
	distinct := map[string]bool{}
	for _, version := range versions {
		distinct[version.ValueDigest] = true
	}
	if len(distinct) >= 2 {
		return FactStatusConflicting, ""
	}
	return FactStatusUnverified, ""
}

// ConfirmedDigestAt reports whether the exact version carries a
// confirmation (the per-version confirmation view of the history).
func ConfirmedDigestAt(versions []FactVersion, confirmations []FactConfirmation, versionNo int64) bool {
	for _, confirmation := range confirmations {
		if confirmation.VersionNo == versionNo {
			return true
		}
	}
	return false
}

func factDigestOf(versions []FactVersion, versionNo int64) string {
	for _, version := range versions {
		if version.VersionNo == versionNo {
			return version.ValueDigest
		}
	}
	return ""
}

// FactNextAction is the fixed per-status action of the Work Control Card.
func FactNextAction(status string) string {
	switch status {
	case FactStatusConflicting:
		return "Разрешите конфликт: подтвердите точную версию со ссылкой на источник."
	case FactStatusUnverified:
		return "Значение без подтверждения: подтвердите точную версию, если клиент называл её."
	}
	return ""
}

// ---------------------------------------------------------------------------
// Command guards (the pure domain layer of the ten commands)
// ---------------------------------------------------------------------------

// GuardClassify validates the classification command: the current state
// allows it, the type is pinned by the case taxonomy, the title is bounded.
func GuardClassify(current WorkItemCurrent, factType, title string) error {
	if !ClassifyAllowedFrom(current.State) {
		return fmt.Errorf("классификация из состояния %s невозможна: элемент завершён, разделён или объединён", current.State)
	}
	if err := guardWorkType(factType); err != nil {
		return err
	}
	if err := guardWorkTitle(title); err != nil {
		return err
	}
	return nil
}

// GuardTransition validates the state transition command.
func GuardTransition(current WorkItemCurrent, toState string) error {
	if !TransitionAllowed(current.State, toState) {
		return fmt.Errorf("переход %s → %s недопустим лестницей состояний", current.State, toState)
	}
	return nil
}

// GuardSplit validates the split: at least two targets, each typed and
// titled, the source is non-terminal.
func GuardSplit(current WorkItemCurrent, targets []SplitTarget) error {
	if !RescopeAllowedFrom(current.State) {
		return fmt.Errorf("разделение из состояния %s невозможно", current.State)
	}
	if len(targets) < 2 {
		return fmt.Errorf("разделение требует не меньше двух целей, передано %d", len(targets))
	}
	for index, target := range targets {
		if err := guardWorkType(target.Type); err != nil {
			return fmt.Errorf("цель %d: %w", index+1, err)
		}
		if err := guardWorkTitle(target.Title); err != nil {
			return fmt.Errorf("цель %d: %w", index+1, err)
		}
	}
	return nil
}

// GuardMerge validates the merge: at least one source, every source and
// the target are non-terminal, the target is not among the sources.
func GuardMerge(target WorkItemCurrent, sources []WorkItemCurrent) error {
	if !RescopeAllowedFrom(target.State) {
		return fmt.Errorf("цель объединения в состоянии %s не принимает материал", target.State)
	}
	if len(sources) < 1 {
		return fmt.Errorf("объединение требует хотя бы один источник")
	}
	for _, source := range sources {
		if !RescopeAllowedFrom(source.State) {
			return fmt.Errorf("источник %s в состоянии %s не может быть объединён", source.WorkItemID, source.State)
		}
		if source.WorkItemID == target.WorkItemID {
			return fmt.Errorf("цель объединения не может быть его же источником")
		}
		if source.CaseID != target.CaseID {
			return fmt.Errorf("источник %s принадлежит другому обращению", source.WorkItemID)
		}
	}
	return nil
}

// GuardLinkEvent validates the event link: the work item is active enough
// to take material (the merged/split material lives in its destination).
func GuardLinkEvent(current WorkItemCurrent) error {
	if !RescopeAllowedFrom(current.State) {
		return fmt.Errorf("элемент в состоянии %s не принимает связи событий; свяжите с действующим элементом", current.State)
	}
	return nil
}

// GuardRecordFact validates the fact write: the provenance source has a
// producer, the source id is bounded, the scope work item is non-terminal.
func GuardRecordFact(current WorkItemCurrent, sourceType, sourceID string) error {
	if !RescopeAllowedFrom(current.State) {
		return fmt.Errorf("факт нельзя записать в скоуп элемента в состоянии %s", current.State)
	}
	if !FactSourceProducers[sourceType] {
		return fmt.Errorf("источник факта %s зарезервирован словарём без продюсера в этом срезе", sourceType)
	}
	if sourceType == FactSourceChannelEvent {
		if _, err := id.Parse(sourceID); err != nil {
			return fmt.Errorf("источник channel_event требует event_id принятого события этого обращения")
		}
	} else if len(sourceID) < 8 || len(sourceID) > 200 {
		return fmt.Errorf("ключ команды-источника должен быть от 8 до 200 символов")
	}
	return nil
}

// GuardConfirmFactVersion validates the confirmation: the confirmer is
// operator_relay (the only producer), the version exists, the evidence is
// an event id of the case or a bounded note — exactly one of them.
func GuardConfirmFactVersion(versionCount int, versionNo int64, confirmerType string, evidenceEventID id.UUID, note string) error {
	if confirmerType != ConfirmerOperatorRelay {
		return fmt.Errorf("подтверждающий %s зарезервирован словарём без продюсера в этом срезе", confirmerType)
	}
	if versionCount == 0 || versionNo < 1 || versionNo > int64(versionCount) {
		return fmt.Errorf("версия %d не существует у этого факта", versionNo)
	}
	if evidenceEventID.Valid() && strings.TrimSpace(note) != "" {
		return fmt.Errorf("evidence — либо событие кейса, либо нота: ровно одно")
	}
	if !evidenceEventID.Valid() {
		trimmed := strings.TrimSpace(note)
		if trimmed == "" {
			return fmt.Errorf("подтверждение требует evidence: событие кейса или нота")
		}
		if utf8.RuneCountInString(trimmed) > MaxNoteRunes {
			return fmt.Errorf("нота подтверждения длиннее %d символов", MaxNoteRunes)
		}
	} else if _, err := id.Parse(evidenceEventID.String()); err != nil {
		return fmt.Errorf("evidence-событие должно быть event_id")
	}
	return nil
}

// GuardRescopeFact validates the fact rescope: the target work item is
// non-terminal and belongs to the same case.
func GuardRescopeFact(factCaseID, targetCaseID id.UUID, target WorkItemCurrent) error {
	if factCaseID != targetCaseID {
		return fmt.Errorf("факт и целевой элемент принадлежат разным обращениям")
	}
	if !RescopeAllowedFrom(target.State) {
		return fmt.Errorf("целевой элемент в состоянии %s не принимает скоуп факта", target.State)
	}
	return nil
}

// GuardWorkTitle bounds a work item title.
func GuardWorkTitle(title string) error { return guardWorkTitle(title) }

// GuardReason bounds the command reason (the honest why of the history).
func GuardReason(reason string) error {
	if utf8.RuneCountInString(reason) > MaxReasonRunes {
		return fmt.Errorf("причина команды длиннее %d символов", MaxReasonRunes)
	}
	return nil
}

// SplitTarget is one typed and titled target of the split command.
type SplitTarget struct {
	Type  string `json:"type"`
	Title string `json:"title"`
}

func guardWorkType(workType string) error {
	if strings.TrimSpace(workType) == "" {
		return fmt.Errorf("тип рабочего элемента обязателен")
	}
	if utf8.RuneCountInString(workType) > MaxWorkTitleRunes {
		return fmt.Errorf("тип рабочего элемента длиннее %d символов", MaxWorkTitleRunes)
	}
	return nil
}

func guardWorkTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("название рабочего элемента обязательно")
	}
	if utf8.RuneCountInString(title) > MaxWorkTitleRunes {
		return fmt.Errorf("название рабочего элемента длиннее %d символов", MaxWorkTitleRunes)
	}
	return nil
}
