package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// Module action — story 3.5 slice 1 (step-01): the Operation domain core
// (FR-28/FR-29, AC-4..AC-7, AC-9..AC-11). This file owns the IMMUTABLE
// Operation shape (the pins, the effects copy, the client key word, the
// server intent key word, the keyed material digest), the business-intent
// key (the keyed HMAC space business_intent:v1: — the identity of ONE
// MUTATION), the idempotency fingerprint (the space operation_material:v1:
// — the material identity of ONE REQUEST), the single-word ledger
// dictionary and the read-time state derivation (authorized — executed and
// failed belong to 3-6 and are structurally absent here), with the honest
// texts of the view. The words are frozen in code.
//
// ONLY the action module builds the Authorized Action (AD-6/AC-10): the
// type and its derivations live here; no other module imports them (the
// DAG does not allow it). The AI Runtime does not exist (AG-4) — the
// model's confidence never substitutes a Confirmation: the authorize
// input is pins and key words only, and the unknown-field probe of the
// transport pins it structurally.

// ---------------------------------------------------------------------------
// The operation state and ledger dictionaries (1 word each)
// ---------------------------------------------------------------------------

// Operation state words: the authorization contour has EXACTLY ONE state
// word — authorized. The queue is a FACT (the outbox record presence),
// never a stored state; executed/failed are the 3-6 dictionary and are
// structurally absent here (the unit matrix pins the absence — the view
// never invents an external outcome, AC-11).
const OperationAuthorized = "authorized"

// OperationStateWords is the full dictionary (1 word).
var OperationStateWords = []string{OperationAuthorized}

// Ledger entry kind words: the slice writes EXACTLY the authorized entry;
// 3-6 will extend this dictionary with the dispatch words through the
// same append-only mechanism (AD-13: the Operation Ledger is append-only
// — the INSERT/SELECT grants are structural).
const LedgerEntryAuthorized = "authorized"

// LedgerEntryKindWords is the full dictionary (1 word).
var LedgerEntryKindWords = []string{LedgerEntryAuthorized}

// ---------------------------------------------------------------------------
// The Operation: one immutable row per authorized action
// ---------------------------------------------------------------------------

// Operation is the IMMUTABLE row of one authorized action — ONE row per
// operation, NO versions (the authorization is a one-time act; a change
// is a conflict or a NEW operation through a NEW confirmation, never a
// mutation of this row). The state is NEVER stored (DeriveOperationState
// reads the append-only ledger at every read); the external identifier
// and the Action Receipt are structurally absent — no columns exist, 3-6
// owns them, the honest texts below speak the absence. AD-6: only the
// action module builds this type.
//
// The two key words are NOT interchangeable (FR-29): IdempotencyKey is
// the CLIENT's request identity (the repeats carry it again — the
// replay/conflict comparator of the gateway); BusinessIntentKey is the
// SERVER's mutation identity (the cross-case reservation dedup — the
// equivalent mutation from another Case does not pass). The keys live in
// different spaces: the UNIQUE of the operations table against the UNIQUE
// of the reservations table — the replay and the cross-case conflict are
// structurally distinguishable.
//
// DefinitionVersion is pinned in the row SEPARATELY from the intent key:
// DeriveBusinessIntentKey deliberately never sees it (the equivalent
// mutation is defined by the type plus the intent, not by a definition
// release — the unit matrix pins the equivalence with a probe).
type Operation struct {
	TenantID              id.UUID         `json:"tenant_id"`
	OperationID           id.UUID         `json:"operation_id"`
	DefinitionID          string          `json:"definition_id"`
	DefinitionVersion     string          `json:"definition_version"`
	PreviewID             id.UUID         `json:"preview_id"`
	PreviewVersionNo      int64           `json:"preview_version_no"`
	ConfirmationID        id.UUID         `json:"confirmation_id"`
	ConfirmationVersionNo int64           `json:"confirmation_version_no"`
	CaseID                id.UUID         `json:"case_id"`
	WorkItemID            id.UUID         `json:"work_item_id"`
	Target                ProposalTarget  `json:"target"`
	OwnerKind             string          `json:"owner_kind"`
	OwnerRef              id.UUID         `json:"owner_ref"`
	OwnerEpoch            int64           `json:"owner_epoch"`
	ReleaseDecisionNo     int64           `json:"release_decision_no"`
	ProfileVersion        int64           `json:"profile_version"`
	Partition             string          `json:"partition"`
	Effects               []PreviewEffect `json:"effects"`
	BusinessIntentKey     string          `json:"business_intent_key"`
	IdempotencyKey        string          `json:"idempotency_key"`
	MaterialDigest        string          `json:"material_digest"`
	CreatedAt             time.Time       `json:"created_at"`
}

// OperationLedgerEntry is one append-only row of the operation ledger
// (AD-13): the monotonic entry number, the kind word, the safe details
// (identifiers and words only — never a material value; the store CHECK
// keeps the 500-rune bound) and the moment. The history is never
// rewritten; 3-6 will append the dispatch words through the same
// mechanism.
type OperationLedgerEntry struct {
	EntryNo int64     `json:"entry_no"`
	Kind    string    `json:"kind"`
	Details string    `json:"details"`
	At      time.Time `json:"at"`
}

// ---------------------------------------------------------------------------
// The business-intent key: the keyed identity of ONE MUTATION
// ---------------------------------------------------------------------------

// BusinessIntentInput is the canonical input of the business-intent key:
// the tenant, the owning-system word (the 2-7 dictionary — the target
// system of the mutation), the action type (the definition ID — the
// DEFINITION VERSION DELIBERATELY NEVER ENTERS: the equivalent mutation
// is defined by the type plus the intent, not by a definition release),
// and the normalized material intent: the target pin, the sorted typed
// input and the owning system's external identity words of the linked
// snapshot (the external ids/versions — the statuses, the observed times
// and the evidence are NOT intent and never enter).
type BusinessIntentInput struct {
	TenantID     id.UUID
	OwningSystem string
	DefinitionID string
	Target       ProposalTarget
	TypedInput   map[string]string
	Snapshot     []MaterialSnapshotRow
}

// sortedSnapshotIdentity renders the owning-system identity words of the
// snapshot deterministically: material field, external id, external
// version — sorted by the material field word (the status, the observed
// time and the evidence reference never enter the intent).
func sortedSnapshotIdentity(rows []MaterialSnapshotRow) string {
	ordered := append([]MaterialSnapshotRow(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].MaterialField < ordered[j].MaterialField })
	records := make([]string, 0, len(ordered))
	for _, row := range ordered {
		records = append(records, strings.Join([]string{
			row.MaterialField,
			row.ExternalID,
			row.ExternalVersion,
		}, canonicalRecordSep))
	}
	return strings.Join(records, canonicalGroupSep)
}

// DeriveBusinessIntentKey derives the server-side identity of ONE
// MUTATION (FR-29/AC-4): HMAC-SHA256(key, "business_intent:v1:" + tenant
// + FS + owning system + FS + definition ID + FS + target kind + FS +
// target ref + FS + sorted typed input + FS + sorted snapshot identity)
// in hex. The prefix separates this key space from operation_material:v1:
// (this file) and from commitment:v1:/external_commitment:v1:/action_preview:v1:
// (2-9/3-3/3-4) — the families never collide; the key is the per-tenant
// commitment key v1 (commitments_hmac_keys — the 2-9/3-3/3-4 reuse, the
// INSERT-only ensure pattern). The raw unsalted hash of a short sensitive
// value is structurally impossible: every intent byte passes through the
// keyed HMAC (AD-10 — the unit matrix pins the keyed property with a
// probe).
func DeriveBusinessIntentKey(key []byte, in BusinessIntentInput) string {
	fields := []string{
		in.TenantID.String(),
		in.OwningSystem,
		in.DefinitionID,
		in.Target.Kind,
		in.Target.Ref.String(),
		sortedTypedInput(in.TypedInput),
		sortedSnapshotIdentity(in.Snapshot),
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("business_intent:v1:"))
	mac.Write([]byte(strings.Join(fields, canonicalFieldSep)))
	return hex.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// The idempotency fingerprint: the keyed material identity of ONE REQUEST
// ---------------------------------------------------------------------------

// OperationRequestPins are the request pins of one authorization attempt:
// the EXACT preview version and the EXACT confirmation version the attempt
// carries. In this slice the material identity of the request IS the pin
// pair (the append-only rows make the pair a bijection with the shown
// material — the preview version, its snapshot and its effects never
// mutate): the same pair yields the same fingerprint (the replay of AC-7),
// a different confirmation version yields a different fingerprint (the
// conflict of AC-8 — the webhook repeat with the same key after a
// re-confirmation of a new version is the honest conflict).
type OperationRequestPins struct {
	PreviewID             id.UUID
	PreviewVersionNo      int64
	ConfirmationID        id.UUID
	ConfirmationVersionNo int64
}

// DeriveIdempotencyFingerprint derives the keyed material digest of one
// authorization request: HMAC-SHA256(key, "operation_material:v1:" +
// tenant + FS + preview id + FS + preview version + FS + confirmation id
// + FS + confirmation version) in hex — the material_digest column of the
// operation row, the comparator of the replay check (the FIRST check of
// the gateway — before the rejection ladder). The prefix separates the
// space from business_intent:v1: (the two key spaces never mix); the
// CLIENT idempotency key NEVER enters the fingerprint (the two identities
// are independent by construction — the replay compares the stored digest
// against the fresh derivation, the conflict would be impossible
// otherwise).
func DeriveIdempotencyFingerprint(key []byte, tenant id.UUID, pins OperationRequestPins) string {
	fields := []string{
		tenant.String(),
		pins.PreviewID.String(),
		fmt.Sprintf("%d", pins.PreviewVersionNo),
		pins.ConfirmationID.String(),
		fmt.Sprintf("%d", pins.ConfirmationVersionNo),
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("operation_material:v1:"))
	mac.Write([]byte(strings.Join(fields, canonicalFieldSep)))
	return hex.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// The state derivation: the single word from the append-only ledger
// ---------------------------------------------------------------------------

// DeriveOperationState derives the read-time state word of one operation
// from its append-only ledger: every entry kind must be in the ledger
// dictionary (fail-closed on a foreign word — the honest refusal, never a
// masked silence; the executed/failed words of 3-6 refuse here until the
// future slice extends the dictionary), an empty ledger refuses (the
// authorization is evidenced by the authorized entry — its absence is a
// data corruption, not an honest state), and the slice's single word is
// authorized. The queue is a FACT (the outbox record presence) — never a
// stored state.
func DeriveOperationState(ledger []OperationLedgerEntry) (string, error) {
	if len(ledger) == 0 {
		return "", RuleError{Rule: "DeriveOperationState",
			Reason: "Ledger операции пуст — слово состояния не выводится (fail-closed)."}
	}
	for _, e := range ledger {
		if !WordIn(LedgerEntryKindWords, e.Kind) {
			return "", RuleError{Rule: "DeriveOperationState", Reason: fmt.Sprintf(
				"Запись Operation Ledger «%s» вне словаря (%s).", e.Kind, strings.Join(LedgerEntryKindWords, ", "))}
		}
	}
	return OperationAuthorized, nil
}

// DeriveOperationViewWord derives the SPOKEN state of the operation card
// (AC-11: the state shows as authorized/queued, never executed): the
// ledger derivation plus the queue fact — the outbox record presence
// speaks the queued word honestly (the dispatch worker does not exist in
// this slice; the record is the fact, the word is derived, nothing is
// stored).
func DeriveOperationViewWord(ledger []OperationLedgerEntry, outboxPresent bool) (string, error) {
	state, err := DeriveOperationState(ledger)
	if err != nil {
		return "", err
	}
	if outboxPresent {
		return state + "/queued", nil
	}
	return state, nil
}

// ---------------------------------------------------------------------------
// The honest texts of the view (AC-11) and the two return surfaces
// ---------------------------------------------------------------------------

// The fixed honest texts of the operation view and the return surfaces:
// the state words speak the truth, the absence of the external outcome is
// spoken, never masked. The card, the API view and the page share these
// exact texts (an app-private copy would drift).
const (
	// OperationAuthorizedText speaks the single state word.
	OperationAuthorizedText = "Operation авторизована (authorized) — разрешение получено и закреплено записью Operation Ledger."
	// OperationQueuedText speaks the queue fact: the outbox record exists.
	OperationQueuedText = "Operation в очереди на будущий dispatch (queued) — запись operation.queued существует в outbox."
	// OperationDispatchAbsentText speaks the 3-6 boundary honestly.
	OperationDispatchAbsentText = "Dispatch worker не существует — записи outbox честно копятся pending до контура исполнения 3-6."
	// OperationExternalIDAbsentText speaks the structural absence of the
	// external outcome: no column, no word, no invention.
	OperationExternalIDAbsentText = "Внешний идентификатор не выдан — внешний correlation ID и Action Receipt появятся в контуре 3-6 и здесь не выдумываются."
	// IdempotentReplayText is the honest 200 of the replay (AC-7): the
	// return speaks the idempotency contract.
	IdempotentReplayText = "Повтор запроса с тем же idempotency key и тем же материалом: возвращена первоначальная Operation и её состояние — Confirmation повторно не потребляется, вторая outbox запись не создаётся."
	// ReservationDuplicateText is the honest return of the cross-case
	// loser (AC-9): the reference speaks the reservation.
	ReservationDuplicateText = "Эквивалентное бизнес-намерение уже зарезервировано — возвращена ссылка на существующую Operation; потребление Confirmation проигравшего не произошло."
	// ReconciliationReservedText documents the reserved word of 3-7: the
	// honest boundary of the slice (the word is NOT returned here).
	ReconciliationReservedText = "Слово reconciliation_required зарезервировано контуром 3-7 и в этом срезе не возвращается."
)
