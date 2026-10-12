package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The owning-system observations (Story 2.7): the State/Authority Matrix v1,
// the typed read command and the read-time derivations
// ---------------------------------------------------------------------------
//
// The State/Authority Matrix v1 is the fixed Domain Pack v0 dictionary (the
// ruleTableV1 precedent of 2.6): every declared material field names its
// OWNING system word, the carrier of the external id, the one permitted
// projection, the conflict rule word and the freshness seconds. A field
// outside the matrix is honestly unknown as a CLASS — the read command
// refuses, no row is ever written (the rich per-pack matrices are the Epic
// 4/5 boundary; the matrix is never loaded from a tenant configuration or
// a tool output).
//
// The Observation is the canonical append-only row of one read: the status
// word of the fixed dictionary, the reason word of the fixed failure
// vocabulary, the canonical projection value bytes (ok only), the external
// id and the provider version of the value, the provider-observed time and
// the receive time (never mixed — the 2-2 precedent). A failed or unknown
// read NEVER carries a value: the assumed value is not written (AC-6). The
// CURRENT observation of a field of a Work Item is the read-time derivation
// (the maximal observation number) — a re-read supersedes by a NEW row.
//
// The freshness is a read-time derivation over the matrix word: age >
// freshness_seconds → stale (re-read before a material answer), age ≤
// freshness → fresh. failed/unknown never dress up as fresh. The conflict
// with a confirmed fact of the case (2-2, the same material key) is the
// honest badge word over the canonical BYTES — the resolution always stays
// with the human confirmation, never with an automatic value choice.

// ---------------------------------------------------------------------------
// Dictionaries
// ---------------------------------------------------------------------------

// AuthorityMatrixVersion is the fixed dictionary word of the v1 matrix (the
// snapshot word every observation row carries through the read material).
const AuthorityMatrixVersion = "state-authority-matrix-v1"

// The observation statuses: the fixed dictionary of the read outcomes. ok
// carries the value; failed is the honest refusal of the provider answer;
// unknown is the honest absence of a claimable value.
const (
	ObservationStatusOk      = "ok"
	ObservationStatusFailed  = "failed"
	ObservationStatusUnknown = "unknown"
)

// ObservationStatuses is the full dictionary.
var ObservationStatuses = []string{ObservationStatusOk, ObservationStatusFailed, ObservationStatusUnknown}

// The read reasons: the fixed failure vocabulary of the owning-system reads
// (the reserved words of the future provider discipline — AG-4).
const (
	ReadReasonProviderTimeout   = "provider_timeout"
	ReadReasonMalformedResponse = "malformed_response"
	ReadReasonInsufficientScope = "insufficient_scope"
	ReadReasonReadError         = "read_error"
)

// ReadReasons is the full dictionary.
var ReadReasons = []string{ReadReasonProviderTimeout, ReadReasonMalformedResponse, ReadReasonInsufficientScope, ReadReasonReadError}

// The owning system words: the fixed dictionary of the systems that OWN the
// material fields of the matrix v1 (the words the observation rows snapshot
// at the read moment).
const (
	OwningSystemCatalog = "catalog"
	OwningSystemErp     = "erp"
)

// OwningSystems is the full dictionary.
var OwningSystems = []string{OwningSystemCatalog, OwningSystemErp}

// The material fields of the authority matrix v1 (the fixed Domain Pack v0
// words): the only readable fields of the slice.
const (
	MaterialFieldPrice        = "price"
	MaterialFieldAvailability = "availability"
	MaterialFieldOrderStatus  = "order_status"
)

// MaterialFields is the full dictionary.
var MaterialFields = []string{MaterialFieldPrice, MaterialFieldAvailability, MaterialFieldOrderStatus}

// The projections of the matrix v1: the one permitted projection word per
// material field (the projection is what the owning system is asked for and
// what the observation stores — no projection word outside the matrix).
const (
	ProjectionPriceQuote = "price_quote"
	ProjectionStockCount = "stock_count"
	ProjectionStatusWord = "status_word"
)

// MatrixProjections is the full dictionary.
var MatrixProjections = []string{ProjectionPriceQuote, ProjectionStockCount, ProjectionStatusWord}

// The conflict rule words of the matrix v1: show_conflict is the only word
// of the slice — the panel shows the badge, the resolution stays with the
// human (the auto-resolution never appears as a class).
const ConflictRuleShowConflict = "show_conflict"

// The external id carrier words of the matrix v1: the anchor the owning
// system names the entity by (the v1 carrier of every field is the work
// item of the read scope).
const ExternalIDCarrierWorkItem = "work_item"

// The observation material limits (the fixed dictionary of the slice; the
// value limit mirrors the 2-2 MaxFactValueBytes discipline).
const (
	MaxObservationValueBytes    = 2048
	MaxExternalIDRunes          = 64
	MaxExternalVersionRunes     = 64
	MaxObservationMaterialRunes = 64
)

// ReadDeadline is the fixed dictionary word of the read attempt discipline:
// ONE attempt per command with the bounded deadline — no retry, no circuit
// breaker, no rate limit (the full AD-5 provider discipline comes with the
// real providers, AG-4; the word stays in the dictionary so the future
// rules never rename the honesty).
const ReadDeadline = 5 * time.Second

// ---------------------------------------------------------------------------
// The authority matrix v1
// ---------------------------------------------------------------------------

// AuthorityEntry is one row of the fixed State/Authority Matrix v1: the
// material field word, the owning system that owns it, the carrier of the
// external id, the one permitted projection, the conflict rule word and the
// freshness seconds of the read-time derivation.
type AuthorityEntry struct {
	MaterialKey       string
	OwningSystem      string
	ExternalIDCarrier string
	Projection        string
	ConflictRule      string
	FreshnessSeconds  int
}

// authorityMatrixV1 is the fixed table of the Domain Pack v0 (the
// ruleTableV1 precedent of 2-6): price is owned by the catalog with the
// price_quote projection (5 minutes of freshness), availability by the
// catalog with stock_count (1 minute), order_status by the erp with
// status_word (2 minutes). A field without a row is unknown as a class —
// the read is impossible.
var authorityMatrixV1 = map[string]AuthorityEntry{
	MaterialFieldPrice: {
		MaterialKey: MaterialFieldPrice, OwningSystem: OwningSystemCatalog,
		ExternalIDCarrier: ExternalIDCarrierWorkItem, Projection: ProjectionPriceQuote,
		ConflictRule: ConflictRuleShowConflict, FreshnessSeconds: 300,
	},
	MaterialFieldAvailability: {
		MaterialKey: MaterialFieldAvailability, OwningSystem: OwningSystemCatalog,
		ExternalIDCarrier: ExternalIDCarrierWorkItem, Projection: ProjectionStockCount,
		ConflictRule: ConflictRuleShowConflict, FreshnessSeconds: 60,
	},
	MaterialFieldOrderStatus: {
		MaterialKey: MaterialFieldOrderStatus, OwningSystem: OwningSystemErp,
		ExternalIDCarrier: ExternalIDCarrierWorkItem, Projection: ProjectionStatusWord,
		ConflictRule: ConflictRuleShowConflict, FreshnessSeconds: 120,
	},
}

// AuthorityEntryOf resolves the matrix row of one material field (ok=false
// when the field is outside the matrix — the honest unknown of the class).
func AuthorityEntryOf(materialKey string) (AuthorityEntry, bool) {
	entry, ok := authorityMatrixV1[materialKey]
	return entry, ok
}

// MaterialFieldInMatrix reports whether the material field belongs to the
// fixed matrix (the readable dictionary of the slice).
func MaterialFieldInMatrix(materialKey string) bool {
	_, ok := authorityMatrixV1[materialKey]
	return ok
}

// ---------------------------------------------------------------------------
// The observation material
// ---------------------------------------------------------------------------

// Observation is one append-only row of one owning-system read: the Work
// Item scope with the material field word, the snapshot of the matrix row
// (the owning system word, the projection), the read outcome (the status
// word with the reason word), the canonical projection value bytes (ok
// only), the external id with the provider version and both times (the
// provider-observed and the receive — never mixed), the acting operator
// with the correlation and the monotonic per-(work item, material field)
// number. The CURRENT observation is the maximal number — a re-read
// supersedes by a NEW row, never by an update.
type Observation struct {
	TenantID        id.UUID
	CaseID          id.UUID
	WorkItemID      id.UUID
	ObservationID   id.UUID
	ObservationNo   int64
	MaterialKey     string
	OwningSystem    string
	Projection      string
	Status          string
	Reason          string
	Value           []byte
	ExternalID      string
	ExternalVersion string
	ObservedAt      time.Time
	ReceivedAt      time.Time
	ActorID         id.UUID
	CorrelationID   id.UUID
	CreatedAt       time.Time
}

// CurrentObservation resolves the CURRENT observation of one material field
// of one Work Item: the maximal observation number (nil when no observation
// exists — the honest pre-read state of the panel).
func CurrentObservation(observations []Observation) *Observation {
	var current *Observation
	for i := range observations {
		if current == nil || observations[i].ObservationNo > current.ObservationNo {
			current = &observations[i]
		}
	}
	return current
}

// ObservationValue canonicalizes the projection value bytes of one read and
// bounds them to the fixed dictionary limit: the value is ONE JSON value in
// the canonical form (the same discipline as the fact values — the honest
// CHECK of the jsonb column, never a 500 on the write).
func ObservationValue(raw []byte) ([]byte, error) {
	canonical, err := CanonicalPayload(raw)
	if err != nil {
		return nil, fmt.Errorf("значение наблюдения должно быть одним JSON-значением: %w", err)
	}
	if len(canonical) > MaxObservationValueBytes {
		return nil, fmt.Errorf("значение наблюдения длиннее %d байт канонического JSON", MaxObservationValueBytes)
	}
	return canonical, nil
}

// ---------------------------------------------------------------------------
// Command guards
// ---------------------------------------------------------------------------

// GuardMaterialFieldInMatrix refuses the material field outside the fixed
// matrix honestly: the field is unknown as a class, the read is impossible
// (the full field dictionary travels in the text — the machine-readable
// refusal of the 400 field-not-in-matrix).
func GuardMaterialFieldInMatrix(materialKey string) error {
	if !MaterialFieldInMatrix(materialKey) {
		return fmt.Errorf("материальное поле «%s» вне матрицы State/Authority v1: чтение невозможно, поле unknown как класс; словарь полей матрицы: %s", materialKey, joinQuoted(MaterialFields))
	}
	return nil
}

// GuardReadOwnCommand validates the structure of the read command: the case
// and the work item anchors are required and the material key is a bounded
// non-empty string (the matrix membership is the field guard's own word).
func GuardReadOwnCommand(caseID, workItemID id.UUID, materialKey string) error {
	if !caseID.Valid() {
		return fmt.Errorf("якорь case_id обязателен (UUID): чтение живёт на обращении")
	}
	if !workItemID.Valid() {
		return fmt.Errorf("якорь work_item_id обязателен (UUID): материальное поле читается на элементе работы")
	}
	if materialKey == "" {
		return fmt.Errorf("материальное поле обязательно: выберите поле словаря матрицы")
	}
	if utf8.RuneCountInString(materialKey) > MaxObservationMaterialRunes {
		return fmt.Errorf("материальное поле длиннее %d символов", MaxObservationMaterialRunes)
	}
	return nil
}

// GuardObservationMaterial validates the port answer of one read honestly
// (the fail-closed basis against a misbehaving adapter): the status word is
// the dictionary; ok carries NO reason word and ALWAYS the value, the
// external id, the provider version and the provider-observed time; a
// failed/unknown answer NEVER carries a value (the assumed value is never
// written — AC-6) and ALWAYS names its reason word of the dictionary; the
// external id and the version are bounded.
func GuardObservationMaterial(status, reason string, value []byte, externalID, externalVersion string, observedAt time.Time) error {
	if !inDictionary(ObservationStatuses, status) {
		return fmt.Errorf("статус чтения не входит в словарь (%s)", joinQuoted(ObservationStatuses))
	}
	if status == ObservationStatusOk {
		if reason != "" {
			return fmt.Errorf("успешное чтение не несёт причину отказа: слово причины относится к failed/unknown")
		}
		if len(value) == 0 {
			return fmt.Errorf("успешное чтение обязано нести значение проекции: пустое значение не записывается")
		}
		if len(value) > MaxObservationValueBytes {
			return fmt.Errorf("значение наблюдения длиннее %d байт", MaxObservationValueBytes)
		}
		if externalID == "" {
			return fmt.Errorf("успешное чтение обязано нести внешний ID носителя значения")
		}
		if utf8.RuneCountInString(externalID) > MaxExternalIDRunes {
			return fmt.Errorf("внешний ID длиннее %d символов", MaxExternalIDRunes)
		}
		if externalVersion == "" {
			return fmt.Errorf("успешное чтение обязано нести версию значения провайдера")
		}
		if utf8.RuneCountInString(externalVersion) > MaxExternalVersionRunes {
			return fmt.Errorf("версия провайдера длиннее %d символов", MaxExternalVersionRunes)
		}
		if observedAt.IsZero() {
			return fmt.Errorf("успешное чтение обязано нести время провайдера (observed_at)")
		}
		return nil
	}
	if !inDictionary(ReadReasons, reason) {
		return fmt.Errorf("неудачное чтение обязано назвать причину словарём (%s)", joinQuoted(ReadReasons))
	}
	if len(value) != 0 {
		return fmt.Errorf("неудачное чтение не несёт значение: предполагаемое значение не записывается никогда")
	}
	if externalVersion != "" {
		return fmt.Errorf("неудачное чтение не несёт версию значения провайдера")
	}
	return nil
}

// TerminalWorkItemReadText is the honest refusal of the read of a terminal
// Work Item (the resolved and the merged states are final; the merged item
// lives on in its target — the 2-2 boundary, the 2-6 precedent).
const TerminalWorkItemReadText = "Терминальный элемент работы не читается: resolved и merged — финальные состояния; для merged читайте целевой элемент работы."

// ---------------------------------------------------------------------------
// The read-time derivations
// ---------------------------------------------------------------------------

// The observation states: the read-time freshness derivations (never
// stored). fresh is the value inside the matrix window; stale is the value
// past it (re-read before a material answer — the text badge of the panel);
// failed and unknown never dress up as fresh.
const (
	ObservationStateFresh   = "fresh"
	ObservationStateStale   = "stale"
	ObservationStateFailed  = "failed"
	ObservationStateUnknown = "unknown"
)

// DeriveObservationState derives the state word of one observation: ok with
// age ≤ freshness_seconds → fresh; ok with age > freshness_seconds → stale;
// failed → failed; unknown → unknown. The age is measured from the
// provider-observed time (never the receive time — the 2-2 discipline).
func DeriveObservationState(status string, age time.Duration, freshnessSeconds int) string {
	switch status {
	case ObservationStatusOk:
		if age <= time.Duration(freshnessSeconds)*time.Second {
			return ObservationStateFresh
		}
		return ObservationStateStale
	case ObservationStatusFailed:
		return ObservationStateFailed
	default:
		return ObservationStateUnknown
	}
}

// StaleBadgeText is the honest text of the stale badge of the panel: the
// operator re-reads before a material answer (stale is visible NOT by color
// alone — the word carries the action).
const StaleBadgeText = "перечитайте перед материальным ответом"

// The conflict badge words of the panel: the byte comparison of the
// canonical observation value with the confirmed exact fact value (the same
// material key). equal — the values match; differs — the honest
// «расхождение» (both cards stay side by side, the resolution is the human
// confirmation — never an automatic choice); no_fact — the case has no
// confirmed fact of the field.
const (
	ConflictBadgeEqual   = "equal"
	ConflictBadgeDiffers = "differs"
	ConflictBadgeNoFact  = "no_fact"
)

// ConflictBadgeOf compares the canonical observation value bytes with the
// value of the confirmed exact fact version (2-2, the same material key):
// equal, differs or no_fact (a nil confirmed fact). The comparison is over
// the BYTES of the canonical forms — the badge is a word for the panel,
// never a resolution.
func ConflictBadgeOf(value []byte, confirmed *FactVersion) string {
	if confirmed == nil {
		return ConflictBadgeNoFact
	}
	if bytes.Equal(value, confirmed.Value) {
		return ConflictBadgeEqual
	}
	return ConflictBadgeDiffers
}

// ---------------------------------------------------------------------------
// The outbox payload (the safe canonical JSON)
// ---------------------------------------------------------------------------

// Outbox event types of the observation surface (the 2-1 bounded
// vocabulary).
const (
	OutboxEventOwningSystemObserved   = "case.owning_system_observed"
	OutboxEventOwningSystemObservedV1 = 1
)

// OwningSystemObservedPayload is the safe canonical JSON of the observation
// event: the identifiers, the dictionary state words and the monotonic
// number — the projection VALUE never enters the envelope (the value stays
// in the observation row — the 2-6 «кандидат не покидает строку»
// precedent), no external body travels either.
type OwningSystemObservedPayload struct {
	TenantID        id.UUID   `json:"tenant_id"`
	CaseID          id.UUID   `json:"case_id"`
	WorkItemID      id.UUID   `json:"work_item_id"`
	ObservationID   id.UUID   `json:"observation_id"`
	ObservationNo   int64     `json:"observation_no"`
	MaterialKey     string    `json:"material_key"`
	OwningSystem    string    `json:"owning_system"`
	Projection      string    `json:"projection"`
	Status          string    `json:"status"`
	Reason          string    `json:"reason,omitempty"`
	ExternalID      string    `json:"external_id,omitempty"`
	ExternalVersion string    `json:"external_version,omitempty"`
	ActorID         id.UUID   `json:"actor_id"`
	OccurredAt      time.Time `json:"occurred_at"`
}

// DedupKeyOwningSystemObserved builds the dedup key of one observation
// event.
func DedupKeyOwningSystemObserved(observationID id.UUID) string {
	return "case.owning_system_observed:" + observationID.String()
}

// OwningSystemObservedPayloadJSON renders the safe canonical JSON.
func OwningSystemObservedPayloadJSON(payload OwningSystemObservedPayload) ([]byte, error) {
	return json.Marshal(payload)
}
