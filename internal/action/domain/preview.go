package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// Module action — story 3.4 slice 1: the proposal→preview preparation
// contour (FR-28, AC-1..AC-12). This file owns the compile-time
// dictionaries, the strict proposal guard with the bounded repair ladder
// (AD-7), the preparation policy gate order over LIVE sources (the typed
// proposal text is never a policy input — structural in the signature),
// the material snapshot mapping with the explicit allowed_uncertain word,
// the append-only preview shape with the keyed digest (action_preview:v1:),
// the effects derived ONLY from the definition, the staffy/human-only
// partition derivation, and the read-time derivations (readiness,
// freshness with the seven AC-9 reasons, Timeline words). The words are
// frozen in code; runtime or tenant-config extension is NEVER allowed.
//
// Dictionary sources (mirrors below are pinned by the integration contract
// test against the living sources — drift turns the test red; the action
// domain never imports the control/entitlement/connector/cases domain
// packages):
//   - Scoped control modes — control domain 3-2 (byte-exact,
//     the slash inside intake_only/no_promises is part of the word)
//   - Admission states     — entitlement domain 1-11/1-14
//   - Capability states     — connector domain 1-8/1-9
//   - Material fields       — the 2-7 mirror of model.go (3-1)
//   - Freshness reasons     — AC-9 (Fact, Observation, owner epoch,
//     control generation, Release Manifest, policy, capability evidence)

// ---------------------------------------------------------------------------
// The verdict dictionary and the AD-7 repair bound
// ---------------------------------------------------------------------------

// Verdict words: the closed ladder of the proposal validation outcome.
const (
	VerdictValid    = "valid"
	VerdictInvalid  = "invalid"
	VerdictRejected = "rejected"
)

// VerdictWords is the full dictionary (3 words).
var VerdictWords = []string{VerdictValid, VerdictInvalid, VerdictRejected}

// MaxAttemptsProposal is the AD-7 repair bound: attempt 1 plus EXACTLY ONE
// repair attempt. The store enforces it structurally (UNIQUE
// (proposal_key, attempt_no) with attempt_no CHECK {1, 2}) — the third
// attempt does not exist, it is not a discipline.
const MaxAttemptsProposal = 2

// ---------------------------------------------------------------------------
// The snapshot status dictionary and the reader contract
// ---------------------------------------------------------------------------

// Snapshot status words: the five-word status of one material snapshot row.
// fresh/stale/conflicting/unknown arrive from the reader; allowed_uncertain
// is mapped by THIS domain ONLY when the definition explicitly declared the
// tolerance (never by default, never from silence).
const (
	SnapshotFresh            = "fresh"
	SnapshotStale            = "stale"
	SnapshotConflicting      = "conflicting"
	SnapshotUnknown          = "unknown"
	SnapshotAllowedUncertain = "allowed_uncertain"
)

// SnapshotStatusWords is the full dictionary (5 words).
var SnapshotStatusWords = []string{
	SnapshotFresh, SnapshotStale, SnapshotConflicting, SnapshotUnknown, SnapshotAllowedUncertain,
}

// FreshReadStatusWords is the closed reader contract of OwningSystemReader
// (4 words): a fresh read returns one of these per material field; a nil
// reader (production, AG-4) yields no rows at all and the build fills the
// blocking unknown.
var FreshReadStatusWords = []string{SnapshotFresh, SnapshotStale, SnapshotConflicting, SnapshotUnknown}

// ---------------------------------------------------------------------------
// The readiness, Timeline, event-kind and dry-run dictionaries
// ---------------------------------------------------------------------------

// Preview readiness words: readiness is NOT existence (a blocked preview
// exists and shows the truth; the pin_blocked degradation precedent 3-3).
const (
	PreviewReady           = "ready"
	PreviewBlockedMaterial = "blocked_material"
)

// PreviewReadinessWords is the full dictionary (2 words).
var PreviewReadinessWords = []string{PreviewReady, PreviewBlockedMaterial}

// Timeline words (AC-11): proposed existence words of the preparation
// contour only. authorized/executed belong to the 3-5/3-6 dictionaries and
// are NOT derived here — the absence of the future words is pinned by the
// unit matrix.
const (
	TimelineProposal   = "proposal"
	TimelinePreviewed  = "previewed"
	TimelineSuperseded = "superseded"
)

// TimelineWords is the full dictionary (3 words).
var TimelineWords = []string{TimelineProposal, TimelinePreviewed, TimelineSuperseded}

// Preview event kind words (6): the append-only event history of the
// preparation contour. dry_run_recorded and fresh_read_recorded NEVER move
// the Timeline derivation (the structural honesty of AC-8).
const (
	EventProposed          = "proposed"
	EventRepaired          = "repaired"
	EventPreviewed         = "previewed"
	EventSuperseded        = "superseded"
	EventDryRunRecorded    = "dry_run_recorded"
	EventFreshReadRecorded = "fresh_read_recorded"
)

// PreviewEventKindWords is the full dictionary (6 words).
var PreviewEventKindWords = []string{
	EventProposed, EventRepaired, EventPreviewed, EventSuperseded,
	EventDryRunRecorded, EventFreshReadRecorded,
}

// Dry-run result words (3): the validation evidence vocabulary of the
// DryRunner port. dry-run is never an action and never a guarantee — the
// fixed texts below carry the honesty.
const (
	DryRunResultValid   = "valid"
	DryRunResultInvalid = "invalid"
	DryRunResultError   = "error"
)

// DryRunResultWords is the full dictionary (3 words).
var DryRunResultWords = []string{DryRunResultValid, DryRunResultInvalid, DryRunResultError}

// ---------------------------------------------------------------------------
// The freshness reason dictionary (AC-9, exactly seven, checked in order)
// ---------------------------------------------------------------------------

// Freshness reason words: the first non-matching pin of the build-time
// material against the LIVE sources, checked in THIS order (the order is
// normative — the unit matrix pins it): fact, observation, owner epoch,
// control generation, release manifest (profile version), policy outcome,
// capability evidence.
const (
	FreshnessFactChanged             = "fact_changed"
	FreshnessObservationChanged      = "observation_changed"
	FreshnessOwnerEpochMoved         = "owner_epoch_moved"
	FreshnessControlGenerationMoved  = "control_generation_moved"
	FreshnessReleaseManifestMoved    = "release_manifest_moved"
	FreshnessPolicyChanged           = "policy_changed"
	FreshnessCapabilityEvidenceMoved = "capability_evidence_moved"
)

// PreviewFreshnessReasonWords is the full dictionary (7 words, normative order).
var PreviewFreshnessReasonWords = []string{
	FreshnessFactChanged, FreshnessObservationChanged, FreshnessOwnerEpochMoved,
	FreshnessControlGenerationMoved, FreshnessReleaseManifestMoved,
	FreshnessPolicyChanged, FreshnessCapabilityEvidenceMoved,
}

// ---------------------------------------------------------------------------
// The policy dictionaries
// ---------------------------------------------------------------------------

// Policy outcome words (2).
const (
	PolicyAllow = "allow"
	PolicyBlock = "block"
)

// PolicyOutcomeWords is the full dictionary.
var PolicyOutcomeWords = []string{PolicyAllow, PolicyBlock}

// Policy reason words: the closed refusal vocabulary of the preparation
// policy (10 words). capability_class_unsafe is carried in the dictionary
// as the honest marker word of the human-only partition derivation — the
// preparation policy itself NEVER blocks on the unsafe class (unsafe is a
// legal human-only action; the marking belongs to DeriveExecutionPartition);
// definition_not_resolved is emitted at the definition-resolution step of
// the app chain (the honest 404 BEFORE the policy gates — the policy needs
// the resolved definition to run at all).
const (
	PolicyReasonRoleNotAllowed             = "role_not_allowed"
	PolicyReasonConversationModeNotAllowed = "conversation_mode_not_allowed"
	PolicyReasonResponseOwnerChanged       = "response_owner_changed"
	PolicyReasonControlMode                = "control_mode"
	PolicyReasonEntitlementBlocked         = "entitlement_blocked"
	PolicyReasonStageNotAllowed            = "stage_not_allowed"
	PolicyReasonGateNotSatisfied           = "gate_not_satisfied"
	PolicyReasonCapabilityNotVerified      = "capability_not_verified"
	PolicyReasonCapabilityClassUnsafe      = "capability_class_unsafe"
	PolicyReasonDefinitionNotResolved      = "definition_not_resolved"
)

// PolicyReasonWords is the full dictionary (10 words).
var PolicyReasonWords = []string{
	PolicyReasonRoleNotAllowed, PolicyReasonConversationModeNotAllowed,
	PolicyReasonResponseOwnerChanged, PolicyReasonControlMode,
	PolicyReasonEntitlementBlocked, PolicyReasonStageNotAllowed,
	PolicyReasonGateNotSatisfied, PolicyReasonCapabilityNotVerified,
	PolicyReasonCapabilityClassUnsafe, PolicyReasonDefinitionNotResolved,
}

// ---------------------------------------------------------------------------
// The failure-semantics and partition dictionaries
// ---------------------------------------------------------------------------

// Failure semantics words: the effect failure vocabulary of the preview
// card. The words arrive only from the definition side; in this slice the
// single main effect carries the fail-closed single-effect default (the
// 3-1 definition shape has no failure word of its own — the honest
// boundary; the dictionary is complete for the future finite lists).
const (
	FailureAbortRemaining    = "abort_remaining"
	FailureContinueRemaining = "continue_remaining"
	FailureParkAndReport     = "park_and_report"
)

// FailureSemanticsWords is the full dictionary (3 words).
var FailureSemanticsWords = []string{FailureAbortRemaining, FailureContinueRemaining, FailureParkAndReport}

// Execution partition words (AC-5/AC-7): the staffy/human-only split shown
// on the card in words.
const (
	PartitionStaffy    = "staffy"
	PartitionHumanOnly = "human_only"
)

// ExecutionPartitionWords is the full dictionary (2 words).
var ExecutionPartitionWords = []string{PartitionStaffy, PartitionHumanOnly}

// ---------------------------------------------------------------------------
// The mirrors of the live policy sources (byte-exact, pinned by contract)
// ---------------------------------------------------------------------------

// ScopedControlModeWords mirrors the scoped-control mode dictionary of 3-2
// EXACTLY — intake_only/no_promises keeps its slash: the slash is part of
// the word, this mirror never renames it (the ladder order is normative in
// the source; paused is the safest).
var ScopedControlModeWords = []string{
	"accept",
	"intake_only/no_promises",
	"human_only",
	"paused",
}

// Scoped control mode mirror constants (the byte-exact words).
const (
	ScopedControlAccept     = "accept"
	ScopedControlIntakeOnly = "intake_only/no_promises"
	ScopedControlHumanOnly  = "human_only"
	ScopedControlPaused     = "paused"
)

// AdmissionStateWords mirrors the entitlement admission dictionary of
// 1-11/1-14: ok, blocked_new_admissions, degraded. Only
// blocked_new_admissions blocks the preparation; a missing or foreign
// word fails closed (blocked).
var AdmissionStateWords = []string{
	"ok",
	"blocked_new_admissions",
	"degraded",
}

// Admission mirror constants.
const (
	AdmissionOK       = "ok"
	AdmissionBlocked  = "blocked_new_admissions"
	AdmissionDegraded = "degraded"
)

// CapabilityStateWords mirrors the connector capability-state dictionary of
// 1-8/1-9 (6 words): only verified passes the capability gate; everything
// else — including a missing word — fails closed.
var CapabilityStateWords = []string{
	"unverified",
	"verifying",
	"verified",
	"unsupported",
	"failed",
	"unknown",
}

// Capability state mirror constants.
const (
	CapVerified   = "verified"
	CapUnverified = "unverified"
)

// ---------------------------------------------------------------------------
// The native 3-4 dictionaries and bounds
// ---------------------------------------------------------------------------

// TargetKindWords is the closed dictionary of the intended target kinds of
// a proposal: catalog_item (the catalog owning system objects — the pilot
// product cards), client (the case client), order (the ERP objects). Free
// text is structurally absent from every reference.
var TargetKindWords = []string{
	"catalog_item",
	"client",
	"order",
}

// Target kind constants.
const (
	TargetCatalogItem = "catalog_item"
	TargetClient      = "client"
	TargetOrder       = "order"
)

// The typed-input value bound: one field value is bounded by 2000 runes
// (the domain boundary class of 2-9/3-3; the store CHECK is the loyal
// backstop — it never rejects what the domain accepted).
const MaxTypedInputValueRunes = 2000

// MaxInvalidReportRunes is the honest bound of the persisted invalid report
// (the store CHECK; the transport body carries the full list).
const MaxInvalidReportRunes = 2000

// MaxEventDetailsRunes is the safe-details bound of one event row (the
// store CHECK; details never carry material values).
const MaxEventDetailsRunes = 500

// ---------------------------------------------------------------------------
// The proposal: typed pins, the strict guard, the bounded repair ladder
// ---------------------------------------------------------------------------

// ProposalTarget is the typed intended-target pin: a closed kind word plus
// a typed identifier. Free text is structurally absent.
type ProposalTarget struct {
	Kind string  `json:"kind"`
	Ref  id.UUID `json:"ref"`
}

// Proposal is the machine-readable preparation request of one human author
// (AG-4: the AI Runtime does not exist; the typed boundary stays for the
// future Runtime, and the Runtime would receive no write tool either).
// Every reference is a typed pin; the typed input is a strict
// schema-name→value map validated against the definition's TypedInputSchema.
type Proposal struct {
	DefinitionID           string            `json:"definition_id"`
	DefinitionVersion      string            `json:"definition_version"`
	CaseID                 id.UUID           `json:"case_id"`
	WorkItemID             id.UUID           `json:"work_item_id"`
	Target                 ProposalTarget    `json:"target"`
	TypedInput             map[string]string `json:"typed_input"`
	SupportingFacts        []id.UUID         `json:"supporting_facts"`
	SupportingObservations []id.UUID         `json:"supporting_observations"`
	OwnerEpoch             int64             `json:"owner_epoch"`
	AttemptNo              int               `json:"attempt_no"`
	AuthorID               id.UUID           `json:"author_id"`
}

// DeriveProposalKey derives the deterministic identity of one proposal: a
// plain SHA-256 hex over the pin tuple (tenant, author, case, work item,
// definition pin, target). The typed input is DELIBERATELY absent: a
// repair is the SAME key at attempt 2 — the repair bound would leak
// otherwise. The key is a dedup identity derivation, not a secret (the
// 2-1/3-3 idem-key class: a derived identity, never a counter).
func DeriveProposalKey(tenant id.UUID, p Proposal) string {
	pins := []string{
		tenant.String(),
		p.AuthorID.String(),
		p.CaseID.String(),
		p.WorkItemID.String(),
		p.DefinitionID,
		p.DefinitionVersion,
		p.Target.Kind,
		p.Target.Ref.String(),
	}
	sum := sha256.Sum256([]byte(strings.Join(pins, canonicalFieldSep)))
	return hex.EncodeToString(sum[:])
}

// GuardValidateProposal validates the proposal STRICTLY against the pinned
// definition's TypedInputSchema (AD-7) and the typed-pin structure, and
// returns the honest word list of violations (empty list = valid). It is
// fail-closed: any missing pin, any unknown field, any secret value, any
// out-of-bound attempt number is a violation. Unknown fields NEVER enter
// the preview and are never relayed anywhere — the projection below makes
// that structural, this guard makes it loud.
func GuardValidateProposal(def ActionDefinition, p Proposal) []string {
	var violations []string

	if strings.TrimSpace(p.DefinitionID) == "" || strings.TrimSpace(p.DefinitionVersion) == "" {
		violations = append(violations, "Пин определения действия (ID и версия) обязателен.")
	} else if p.DefinitionID != def.ID || p.DefinitionVersion != def.Version {
		violations = append(violations, "Пин определения proposal обязан совпадать с резолвнутым определением — валидация идёт по пину, а не по «похожему» определению.")
	}
	if p.AttemptNo < 1 || p.AttemptNo > MaxAttemptsProposal {
		violations = append(violations, fmt.Sprintf(
			"Номер попытки обязан быть 1 или 2 — третьей попытки не существует (repair-pass ограничен одной повторной попыткой, AD-7)."))
	}
	if !p.CaseID.Valid() {
		violations = append(violations, "Ссылка на кейс обязательна.")
	}
	if !p.WorkItemID.Valid() {
		violations = append(violations, "Ссылка на рабочий элемент кейса обязательна.")
	}
	if !p.AuthorID.Valid() {
		violations = append(violations, "Автор proposal обязателен.")
	}
	if p.OwnerEpoch < 1 {
		violations = append(violations, "Эпоха владельца ответа (owner epoch) обязана быть живым наблюдением (положительное число).")
	}
	if !WordIn(TargetKindWords, p.Target.Kind) {
		violations = append(violations, fmt.Sprintf(
			"Вид цели «%s» вне словаря {%s}.", p.Target.Kind, strings.Join(TargetKindWords, ", ")))
	}
	if !p.Target.Ref.Valid() {
		violations = append(violations, "Ссылка цели обязана быть корректным идентификатором.")
	}
	for _, ref := range append(append([]id.UUID(nil), p.SupportingFacts...), p.SupportingObservations...) {
		if !ref.Valid() {
			violations = append(violations, "Каждая поддерживающая ссылка (факт, наблюдение) обязана быть корректным идентификатором.")
			break
		}
	}

	// The strict typed-input check against the schema: the schema is the
	// ONLY source of legal field names; a secret field never carries a
	// value; a present value must be a well-formed, bounded value.
	byName := map[string]FieldSchema{}
	for _, f := range def.TypedInputSchema {
		byName[f.Name] = f
	}
	// Unknown names first (the DisallowUnknownFields equivalent).
	names := make([]string, 0, len(p.TypedInput))
	for name := range p.TypedInput {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, ok := byName[name]; !ok {
			violations = append(violations, fmt.Sprintf(
				"Поле «%s» неизвестно типовой схеме действия — неизвестное поле не входит в preview и не передаётся дальше.", name))
		}
	}
	// Declared fields: required presence, secret absence, value shape.
	for _, f := range def.TypedInputSchema {
		value, present := p.TypedInput[f.Name]
		if f.Secret && present {
			violations = append(violations, fmt.Sprintf(
				"Секретное поле «%s» не заполняется значением — значение секрета не подаётся и не сохраняется (только ссылка).", f.Name))
			continue
		}
		if !present {
			if f.Required {
				violations = append(violations, fmt.Sprintf("Обязательное поле «%s» отсутствует.", f.Name))
			}
			continue
		}
		if strings.TrimSpace(value) == "" {
			violations = append(violations, fmt.Sprintf("Поле «%s» обязано быть непустым.", f.Name))
			continue
		}
		if utf8.RuneCountInString(value) > MaxTypedInputValueRunes {
			violations = append(violations, fmt.Sprintf(
				"Поле «%s» длиннее %d рун.", f.Name, MaxTypedInputValueRunes))
			continue
		}
		if f.Type == FieldURL {
			u, err := url.Parse(value)
			if err != nil || !u.IsAbs() || u.Host == "" {
				violations = append(violations, fmt.Sprintf(
					"Поле «%s» обязано быть корректным абсолютным URL.", f.Name))
			}
		}
	}
	return violations
}

// VerdictForInvalid derives the verdict of an invalid submission by the
// attempt number — the AD-7 repair ladder: the FIRST invalid attempt is
// honestly invalid (the word list goes back to the author), the SECOND
// invalid attempt of the same proposal key is terminal rejected. A third
// attempt does not exist (the store UNIQUE is the structural bound).
func VerdictForInvalid(attemptNo int) string {
	if attemptNo <= 1 {
		return VerdictInvalid
	}
	return VerdictRejected
}

// InvalidReportText joins the violation list into the persisted report with
// the honest 2000-rune bound: the truncation is spoken, never silent.
func InvalidReportText(violations []string) string {
	report := strings.Join(violations, "; ")
	if utf8.RuneCountInString(report) <= MaxInvalidReportRunes {
		return report
	}
	runes := []rune(report)
	return string(runes[:MaxInvalidReportRunes]) + " … (перечень усечён границей отчёта)"
}

// ProjectTypedInput projects the submission into the preview's typed input
// STRUCTURALLY: only the schema-declared non-secret fields are copied, in
// the schema's own order. An unknown field cannot survive (it is not in
// the schema), a secret value cannot survive (secret fields are never
// projected) — AC-2 is a type-level guarantee, not a convention.
func ProjectTypedInput(def ActionDefinition, submitted map[string]string) map[string]string {
	projected := map[string]string{}
	for _, f := range def.TypedInputSchema {
		if f.Secret {
			continue
		}
		if value, ok := submitted[f.Name]; ok {
			projected[f.Name] = value
		}
	}
	return projected
}

// ---------------------------------------------------------------------------
// The preparation policy: LIVE gates in the normative order
// ---------------------------------------------------------------------------

// PreparePolicyInput carries the LIVE source readings of one gate pass:
// the principal's role (identity), the case's conversation mode, the live
// owner epoch, the scoped control EffectiveFor mode, the entitlement
// admission state, the employee readiness stage, the live PASS instances of
// the required gates (absent = no live source for that gate in this slice —
// an honest documented boundary, not a block), and the capability state.
// The typed proposal TEXT is structurally absent: this struct is the whole
// policy input space, and no field of it carries the submission values.
type PreparePolicyInput struct {
	PrincipalRole  string
	CaseMode       string
	LiveOwnerEpoch int64
	ControlMode    string
	AdmissionState string
	Stage          string
	GatePasses     map[string]bool
	CapState       string
}

// PreparePolicyOutcome is the composite policy derivation: the outcome word,
// the exact reason word of the closed 10-word dictionary (empty on allow),
// and the human-only mark of the control pass (the human_only mode does
// NOT block the preparation — the human acts by hand; the mark travels to
// the card and the autonomous dispatch path is never offered).
type PreparePolicyOutcome struct {
	Outcome       string
	Reason        string
	HumanOnlyMark bool
}

// GuardPreparePolicy derives the preparation policy of one proposal against
// the pinned definition — the LIVE gates in the NORMATIVE order: role →
// conversation mode → owner epoch → scoped control → entitlement → stage →
// gates → capability. The order is part of the contract: the FIRST blocking
// gate answers, and any block happens BEFORE the snapshot and BEFORE the
// preview (the app chain pins it with fake-repo counters). Fail-closed:
// a missing or foreign reading blocks with the exact gate's reason word.
// The unsafe capability class NEVER blocks here — unsafe is a legal
// human-only action; the marking belongs to DeriveExecutionPartition.
func GuardPreparePolicy(def ActionDefinition, pinnedOwnerEpoch int64, live PreparePolicyInput) PreparePolicyOutcome {
	if !WordIn(def.Conditions.AllowedRoles, live.PrincipalRole) {
		return PreparePolicyOutcome{Outcome: PolicyBlock, Reason: PolicyReasonRoleNotAllowed}
	}
	if !WordIn(def.Conditions.AllowedConversationModes, live.CaseMode) {
		return PreparePolicyOutcome{Outcome: PolicyBlock, Reason: PolicyReasonConversationModeNotAllowed}
	}
	if live.LiveOwnerEpoch < 1 || pinnedOwnerEpoch != live.LiveOwnerEpoch {
		return PreparePolicyOutcome{Outcome: PolicyBlock, Reason: PolicyReasonResponseOwnerChanged}
	}
	switch live.ControlMode {
	case ScopedControlAccept:
		// accept passes without a mark
	case ScopedControlHumanOnly:
		// human_only does NOT block the preparation: the human acts by
		// hand; the mark travels with the preview, the autonomous path is
		// never offered (the honest v1 semantics — documented).
		return PreparePolicyOutcome{Outcome: PolicyAllow, HumanOnlyMark: true}
	case ScopedControlIntakeOnly, ScopedControlPaused:
		return PreparePolicyOutcome{Outcome: PolicyBlock, Reason: PolicyReasonControlMode}
	default:
		// No live control reading (nil reader, unresolvable route) or a
		// foreign word: fail-closed, the safest applicable outcome.
		return PreparePolicyOutcome{Outcome: PolicyBlock, Reason: PolicyReasonControlMode}
	}
	switch live.AdmissionState {
	case AdmissionOK, AdmissionDegraded:
		// ok and degraded keep the preparation admissible
	default:
		// blocked_new_admissions, a missing or a foreign word — fail-closed.
		return PreparePolicyOutcome{Outcome: PolicyBlock, Reason: PolicyReasonEntitlementBlocked}
	}
	if !WordIn(def.Conditions.AllowedStages, live.Stage) {
		return PreparePolicyOutcome{Outcome: PolicyBlock, Reason: PolicyReasonStageNotAllowed}
	}
	for _, gate := range def.Conditions.RequiredGates {
		if passed, sourced := live.GatePasses[gate]; sourced && !passed {
			return PreparePolicyOutcome{Outcome: PolicyBlock, Reason: PolicyReasonGateNotSatisfied}
		}
		// A gate absent from the live map has no live source in this
		// slice — the honest boundary, never a silent pass invented here.
	}
	if live.CapState != CapVerified {
		return PreparePolicyOutcome{Outcome: PolicyBlock, Reason: PolicyReasonCapabilityNotVerified}
	}
	return PreparePolicyOutcome{Outcome: PolicyAllow}
}

// PolicyReasonTexts is the fixed honest text of every policy reason word —
// the card and the API refusal speak the exact dictionary reason in words.
var PolicyReasonTexts = map[string]string{
	PolicyReasonRoleNotAllowed:             "Роль не входит в разрешённые роли действия.",
	PolicyReasonConversationModeNotAllowed: "Режим разговора кейса не входит в разрешённые режимы действия.",
	PolicyReasonResponseOwnerChanged:       "Владелец ответа кейса сменился — закреплённая эпоха владения устарела.",
	PolicyReasonControlMode:                "Действие ограничено управлением (scoped control): текущий режим не разрешает подготовку.",
	PolicyReasonEntitlementBlocked:         "Допуск (entitlement) блокирует новые записи.",
	PolicyReasonStageNotAllowed:            "Стадия готовности не входит в разрешённые стадии действия.",
	PolicyReasonGateNotSatisfied:           "Обязательный гейт не пройден.",
	PolicyReasonCapabilityNotVerified:      "Capability коннектора не подтверждена (состояние не verified).",
	PolicyReasonCapabilityClassUnsafe:      "Класс доставки unsafe — действие остаётся в человеческом контуре (human-only).",
	PolicyReasonDefinitionNotResolved:      "Определение действия не резолвится по пину (ID, версия).",
}

// PolicyReasonText speaks the reason word honestly; a foreign word fails
// closed with the dictionary itself (never a masked silence).
func PolicyReasonText(reason string) string {
	if text, ok := PolicyReasonTexts[reason]; ok {
		return text
	}
	return fmt.Sprintf("Причина блокировки «%s» не входит в словарь (%s).", reason, strings.Join(PolicyReasonWords, ", "))
}

// ---------------------------------------------------------------------------
// The material snapshot: the reader mapping and the readiness derivation
// ---------------------------------------------------------------------------

// MaterialSnapshotRow is one material observation of the preview: the
// material field (2-7 word), the five-word status, the external identity
// words of the owning system, the observed time and the evidence reference.
// The price/sum of the card is spoken ONLY from this row's material —
// the preview never invents a material word.
type MaterialSnapshotRow struct {
	MaterialField   string    `json:"material_field"`
	Status          string    `json:"status"`
	ExternalID      string    `json:"external_id"`
	ExternalVersion string    `json:"external_version"`
	ObservedAt      time.Time `json:"observed_at"`
	EvidenceRef     string    `json:"evidence_ref"`
}

// MapSnapshotStatus derives the row status word from the reader status and
// the definition's DECLARED tolerance for that material field. The
// FactRequirement.Freshness word IS the declaration: a definition that
// requires the exact word the reader returned has explicitly allowed that
// uncertainty — allowed_uncertain is the explicit word, never derived from
// silence. Anything else keeps the reader's own blocking word; a missing or
// foreign reader status fails closed to unknown.
func MapSnapshotStatus(readerStatus string, requiredFreshness string) string {
	switch {
	case readerStatus == SnapshotFresh:
		return SnapshotFresh
	case WordIn(FreshReadStatusWords, readerStatus) && readerStatus == requiredFreshness:
		return SnapshotAllowedUncertain
	case WordIn(FreshReadStatusWords, readerStatus):
		return readerStatus
	default:
		return SnapshotUnknown
	}
}

// DerivePreviewReadiness derives the read-time readiness of one preview
// version from its snapshot rows and the definition's required facts
// (AC-4): every required material field must carry fresh status or the
// EXPLICITLY allowed uncertainty; stale, conflicting, unknown — or a
// missing row, or a nil reader's unknown fill — block the readiness with
// the honest blocked_material word. Readiness is NOT existence: the
// blocked preview exists and shows the truth (the pin_blocked
// degradation precedent 3-3).
func DerivePreviewReadiness(def ActionDefinition, snapshot []MaterialSnapshotRow) string {
	rows := map[string]MaterialSnapshotRow{}
	for _, row := range snapshot {
		rows[row.MaterialField] = row
	}
	for _, req := range def.Conditions.RequiredFacts {
		row, ok := rows[req.MaterialField]
		if !ok {
			return PreviewBlockedMaterial
		}
		switch row.Status {
		case SnapshotFresh, SnapshotAllowedUncertain:
			// fresh and the explicitly allowed uncertainty keep going
		default:
			return PreviewBlockedMaterial
		}
	}
	return PreviewReady
}

// PreviewReadinessTexts is the fixed honest text of the readiness words.
var PreviewReadinessTexts = map[string]string{
	PreviewReady:           "Preview готов: свежий материальный snapshot по всем обязательным полям определения.",
	PreviewBlockedMaterial: "Свежие данные владеющей системы недоступны либо недостаточно свежи — готовность к авторизации заблокирована (preview существует, готовность не притворяется).",
}

// PreviewReadinessText speaks the readiness word; a foreign word fails closed.
func PreviewReadinessText(word string) string {
	if text, ok := PreviewReadinessTexts[word]; ok {
		return text
	}
	return fmt.Sprintf("Слово готовности «%s» не входит в словарь (%s).", word, strings.Join(PreviewReadinessWords, ", "))
}

// ---------------------------------------------------------------------------
// The effects: a finite list derived ONLY from the definition
// ---------------------------------------------------------------------------

// PreviewEffect is one effect of the preview: the order, the action pin,
// the depends-on orders and the failure-semantics word. The list is
// finite BY CONSTRUCTION — it is derived exclusively from the definition
// (the submission cannot add an effect: effects live outside the typed
// input schema, and the derivation below takes no proposal at all).
type PreviewEffect struct {
	Order            int    `json:"order"`
	ActionRef        string `json:"action_ref"`
	DependsOn        []int  `json:"depends_on"`
	FailureSemantics string `json:"failure_semantics"`
}

// DerivePreviewEffects derives the finite effect list of the preview from
// the definition ALONE. In this slice the 3-1 ActionDefinition carries no
// related-effect fields — the slice honestly shows EXACTLY ONE main
// effect with the fail-closed single-effect default (abort_remaining: if
// the main effect fails, nothing else runs — vacuously true for a
// one-effect action, and the conservative word). The structure is ready
// for the finite multi-effect lists of a future definition shape; the
// open set is structurally impossible (no proposal parameter exists).
func DerivePreviewEffects(def ActionDefinition) []PreviewEffect {
	return []PreviewEffect{
		{
			Order:            1,
			ActionRef:        def.ID + "@" + def.Version,
			DependsOn:        nil,
			FailureSemantics: FailureAbortRemaining,
		},
	}
}

// ---------------------------------------------------------------------------
// The execution partition: staffy / human-only (AC-7)
// ---------------------------------------------------------------------------

// ExecutionPartition is the derived staffy/human-only split of the action:
// the partition word, the recovery instruction of the human-only case
// (spoken from the definition's cancel/compensation semantics), the
// mandatory-human-owner flag, and the honest markers of WHY the action
// stayed human-only. The unsafe class never blocks the preparation — it
// marks the partition here; so does a missing proven reconciliation path.
type ExecutionPartition struct {
	Partition                  string `json:"partition"`
	RecoveryInstruction        string `json:"recovery_instruction"`
	HumanOwnerRequired         bool   `json:"human_owner_required"`
	UnsafeClass                bool   `json:"unsafe_class"`
	NoProvenReconciliationPath bool   `json:"no_proven_reconciliation_path"`
}

// DeriveExecutionPartition derives the partition words: the unsafe
// capability class OR a reconciliation path that does not resolve against
// the provider declaration keeps the action in the human-only part — with
// the recovery instruction from the declared cancel/compensation semantics
// and the current human response owner as the mandatory owner. The
// autonomous dispatch path is never offered by the interface (the
// transport layer owns that honesty; the words here are the card's truth).
func DeriveExecutionPartition(def ActionDefinition, reconciliationResolves bool) ExecutionPartition {
	unsafeClass := def.DeliveryContract.CapabilityClass == CapabilityUnsafe
	if unsafeClass || !reconciliationResolves {
		return ExecutionPartition{
			Partition:                  PartitionHumanOnly,
			RecoveryInstruction:        def.DeliveryContract.CancelCompensationSem,
			HumanOwnerRequired:         true,
			UnsafeClass:                unsafeClass,
			NoProvenReconciliationPath: !reconciliationResolves,
		}
	}
	return ExecutionPartition{Partition: PartitionStaffy}
}

// ExecutionPartitionTexts is the fixed honest text of the partition words.
var ExecutionPartitionTexts = map[string]string{
	PartitionStaffy:    "Разграничение: действие исполняется в контуре staffy.",
	PartitionHumanOnly: "Разграничение: human-only — действие исполняет только человек-владелец; автономная отправка не предлагается.",
}

// ExecutionPartitionText speaks the partition word; a foreign word fails closed.
func ExecutionPartitionText(word string) string {
	if text, ok := ExecutionPartitionTexts[word]; ok {
		return text
	}
	return fmt.Sprintf("Слово разграничения «%s» не входит в словарь (%s).", word, strings.Join(ExecutionPartitionWords, ", "))
}

// ---------------------------------------------------------------------------
// The preview version: append-only shape, canonical bytes, keyed digest
// ---------------------------------------------------------------------------

// VersionPin is one supporting reference with the version pinned at build
// time — the freshness derivation re-checks these pins against the live
// sources at every read.
type VersionPin struct {
	Ref     id.UUID `json:"ref"`
	Version int64   `json:"version"`
}

// ActionPreview is one append-only version of the preparation contour: the
// stable preview identity, the monotonic version number, the definition
// pin, the typed pins, the projected typed input (schema fields only, no
// secrets, no unknown fields), the material snapshot rows, the finite
// effects, the derived partition, the build-time policy marks and lineage
// pins (owner epoch, control generation, profile version, policy outcome
// number, capability declaration version) and the keyed digest. A changed
// submission is a NEW version row — old rows never mutate (the 2-9/00009
// pattern); old versions stay readable for the audit forever and lose the
// authorization suitability through the freshness derivation (never
// through deletion).
type ActionPreview struct {
	TenantID               id.UUID               `json:"tenant_id"`
	PreviewID              id.UUID               `json:"preview_id"`
	VersionNo              int64                 `json:"version_no"`
	DefinitionID           string                `json:"definition_id"`
	DefinitionVersion      string                `json:"definition_version"`
	CaseID                 id.UUID               `json:"case_id"`
	WorkItemID             id.UUID               `json:"work_item_id"`
	Target                 ProposalTarget        `json:"target"`
	TypedInput             map[string]string     `json:"typed_input"`
	SupportingFacts        []VersionPin          `json:"supporting_facts"`
	SupportingObservations []VersionPin          `json:"supporting_observations"`
	Snapshot               []MaterialSnapshotRow `json:"snapshot"`
	Effects                []PreviewEffect       `json:"effects"`
	Partition              ExecutionPartition    `json:"partition"`
	HumanOnlyControlMark   bool                  `json:"human_only_control_mark"`
	OwnerEpoch             int64                 `json:"owner_epoch"`
	ControlGeneration      int64                 `json:"control_generation"`
	ProfileVersion         int64                 `json:"profile_version"`
	PolicyOutcomeNo        int64                 `json:"policy_outcome_no"`
	CapabilityDeclVersion  string                `json:"capability_decl_version"`
	// Digest is the keyed identity of the WHOLE shown row (the step-02
	// store column, computed by PreviewDigest over the canonical bytes).
	// It deliberately NEVER enters the canonical material itself — the
	// digest is the identity OF the material, never a part of it.
	Digest    string    `json:"digest"`
	AuthorID  id.UUID   `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}

// The canonical separators: the ASCII unit separator between the top-level
// fields (never inside the material words — the 3-3 precedent), the record
// separator inside the nested lists, and the group separator between the
// nested rows.
const (
	canonicalFieldSep  = "\x1f"
	canonicalRecordSep = "\x1e"
	canonicalGroupSep  = "\x1d"
)

// sortedTypedInput renders the projected typed input deterministically:
// the field names sort ascending (map iteration is random — the canonical
// bytes must not be).
func sortedTypedInput(input map[string]string) string {
	names := make([]string, 0, len(input))
	for name := range input {
		names = append(names, name)
	}
	sort.Strings(names)
	records := make([]string, 0, len(names))
	for _, name := range names {
		records = append(records, name+canonicalRecordSep+input[name])
	}
	return strings.Join(records, canonicalGroupSep)
}

// sortedPins renders the version pins deterministically (by the ref word).
func sortedPins(pins []VersionPin) string {
	ordered := append([]VersionPin(nil), pins...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Ref.String() < ordered[j].Ref.String() })
	records := make([]string, 0, len(ordered))
	for _, pin := range ordered {
		records = append(records, pin.Ref.String()+":"+fmt.Sprintf("%d", pin.Version))
	}
	return strings.Join(records, canonicalGroupSep)
}

// sortedSnapshot renders the snapshot rows deterministically (by the
// material field word).
func sortedSnapshot(rows []MaterialSnapshotRow) string {
	ordered := append([]MaterialSnapshotRow(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].MaterialField < ordered[j].MaterialField })
	records := make([]string, 0, len(ordered))
	for _, row := range ordered {
		records = append(records, strings.Join([]string{
			row.MaterialField,
			row.Status,
			row.ExternalID,
			row.ExternalVersion,
			row.ObservedAt.UTC().Format(time.RFC3339Nano),
			row.EvidenceRef,
		}, canonicalRecordSep))
	}
	return strings.Join(records, canonicalGroupSep)
}

// sortedEffects renders the finite effect list deterministically (by the
// order number).
func sortedEffects(effects []PreviewEffect) string {
	ordered := append([]PreviewEffect(nil), effects...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Order < ordered[j].Order })
	records := make([]string, 0, len(ordered))
	for _, effect := range ordered {
		depends := make([]string, 0, len(effect.DependsOn))
		for _, d := range effect.DependsOn {
			depends = append(depends, fmt.Sprintf("%d", d))
		}
		records = append(records, strings.Join([]string{
			fmt.Sprintf("%d", effect.Order),
			effect.ActionRef,
			strings.Join(depends, ","),
			effect.FailureSemantics,
		}, canonicalRecordSep))
	}
	return strings.Join(records, canonicalGroupSep)
}

// boolWord renders a bool canonically (one stable word per side).
func boolWord(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// PreviewCanonicalBytes builds the canonical bytes of one preview version —
// the deterministic byte material of the digest. Every material field
// participates in the fixed order; the nested lists render deterministically
// (sorted). The provenance fields (the author and the creation time) DO
// participate here: unlike the 3-3 envelope token (which binds the terms
// only), the preview digest is the identity of the WHOLE shown row — the
// future Manifest/Operation references (AC-8 of 3-1) bind the exact
// material the human saw, including who prepared it and when.
func PreviewCanonicalBytes(p ActionPreview) []byte {
	fields := []string{
		p.TenantID.String(),
		p.PreviewID.String(),
		fmt.Sprintf("%d", p.VersionNo),
		p.DefinitionID,
		p.DefinitionVersion,
		p.CaseID.String(),
		p.WorkItemID.String(),
		p.Target.Kind,
		p.Target.Ref.String(),
		sortedTypedInput(p.TypedInput),
		sortedPins(p.SupportingFacts),
		sortedPins(p.SupportingObservations),
		sortedSnapshot(p.Snapshot),
		sortedEffects(p.Effects),
		p.Partition.Partition,
		p.Partition.RecoveryInstruction,
		boolWord(p.Partition.HumanOwnerRequired),
		boolWord(p.Partition.UnsafeClass),
		boolWord(p.Partition.NoProvenReconciliationPath),
		boolWord(p.HumanOnlyControlMark),
		fmt.Sprintf("%d", p.OwnerEpoch),
		fmt.Sprintf("%d", p.ControlGeneration),
		fmt.Sprintf("%d", p.ProfileVersion),
		fmt.Sprintf("%d", p.PolicyOutcomeNo),
		p.CapabilityDeclVersion,
		p.AuthorID.String(),
		p.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	return []byte(strings.Join(fields, canonicalFieldSep))
}

// PreviewDigest computes the keyed digest of one preview version:
// HMAC-SHA256(key, "action_preview:v1:" + preview_id + ":" + version_no +
// ":" + canonical bytes) in hex. The prefix separates this token space
// from commitment:v1: (2-9) and external_commitment:v1: (3-3) — the three
// families never collide. The key is the per-tenant commitment key v1
// (commitments_hmac_keys — the 2-9/3-3 reuse, the INSERT-only ensure
// pattern); the digest is the identity of the shown material for the
// future Manifest/Operation references of 3-5.
func PreviewDigest(key []byte, previewID id.UUID, versionNo int64, canonical []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("action_preview:v1:"))
	mac.Write([]byte(previewID.String()))
	mac.Write([]byte(":"))
	mac.Write([]byte(fmt.Sprintf("%d", versionNo)))
	mac.Write([]byte(":"))
	mac.Write(canonical)
	return hex.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// The freshness derivation: seven reasons, checked in order, at EVERY read
// ---------------------------------------------------------------------------

// PreviewFreshnessLive carries the LIVE source readings of one read pass —
// the fact versions, the observation versions, the live owner epoch, the
// live control generation, the live profile version, the live policy
// outcome number, and the live capability evidence (state plus the
// declaration version). Nothing here is stored: the derivation below
// re-checks the build pins against these readings at every read (the
// 2-12/3-3 read-time precedent — the derived word is NEVER a column).
type PreviewFreshnessLive struct {
	FactVersions          map[id.UUID]int64
	ObservationVersions   map[id.UUID]int64
	OwnerEpoch            int64
	ControlGeneration     int64
	ProfileVersion        int64
	PolicyOutcomeNo       int64
	CapState              string
	CapabilityDeclVersion string
}

// PreviewFreshness is the freshness derivation: Stale is false when every
// build pin still matches the live sources; the Reason carries the exact
// first non-matching pin's dictionary word otherwise.
type PreviewFreshness struct {
	Stale  bool
	Reason string
}

// DerivePreviewFreshness derives the read-time freshness of one preview
// version — AC-9, the seven reasons checked in the NORMATIVE order: the
// supporting facts, the supporting observations, the owner epoch, the
// control generation, the release manifest (the employee profile version),
// the policy (the case policy outcome number — the 3-3 lineage precedent),
// the capability evidence (the state must stay verified AND the
// declaration version must not move). The FIRST non-matching pin is the
// reason; a pin absent from the live map is a change (fail-closed — the
// source disappeared). A fresh derivation never authorizes by itself:
// the 3-5 contour will read THIS derivation before any authorization, and
// a stale version stays readable for the audit forever.
func DerivePreviewFreshness(p ActionPreview, live PreviewFreshnessLive) PreviewFreshness {
	for _, pin := range p.SupportingFacts {
		if liveVersion, ok := live.FactVersions[pin.Ref]; !ok || liveVersion != pin.Version {
			return PreviewFreshness{Stale: true, Reason: FreshnessFactChanged}
		}
	}
	for _, pin := range p.SupportingObservations {
		if liveVersion, ok := live.ObservationVersions[pin.Ref]; !ok || liveVersion != pin.Version {
			return PreviewFreshness{Stale: true, Reason: FreshnessObservationChanged}
		}
	}
	if live.OwnerEpoch != p.OwnerEpoch {
		return PreviewFreshness{Stale: true, Reason: FreshnessOwnerEpochMoved}
	}
	if live.ControlGeneration != p.ControlGeneration {
		return PreviewFreshness{Stale: true, Reason: FreshnessControlGenerationMoved}
	}
	if live.ProfileVersion != p.ProfileVersion {
		return PreviewFreshness{Stale: true, Reason: FreshnessReleaseManifestMoved}
	}
	if live.PolicyOutcomeNo != p.PolicyOutcomeNo {
		return PreviewFreshness{Stale: true, Reason: FreshnessPolicyChanged}
	}
	if live.CapState != CapVerified || live.CapabilityDeclVersion != p.CapabilityDeclVersion {
		return PreviewFreshness{Stale: true, Reason: FreshnessCapabilityEvidenceMoved}
	}
	return PreviewFreshness{Stale: false}
}

// FreshnessReasonTexts is the fixed honest text of every freshness reason.
var FreshnessReasonTexts = map[string]string{
	FreshnessFactChanged:             "Факт кейса изменился с момента построения preview.",
	FreshnessObservationChanged:      "Наблюдение владеющей системы изменилось с момента построения preview.",
	FreshnessOwnerEpochMoved:         "Владелец ответа кейса сменился — эпоха владения двинулась.",
	FreshnessControlGenerationMoved:  "Поколение scoped control двинулось.",
	FreshnessReleaseManifestMoved:    "Release Manifest сменился (версия профиля).",
	FreshnessPolicyChanged:           "Policy outcome кейса сменился.",
	FreshnessCapabilityEvidenceMoved: "Доказательство capability изменилось (состояние или версия декларации).",
}

// FreshnessReasonText speaks the reason word; a foreign word fails closed.
func FreshnessReasonText(reason string) string {
	if text, ok := FreshnessReasonTexts[reason]; ok {
		return text
	}
	return fmt.Sprintf("Причина устаревания «%s» не входит в словарь (%s).", reason, strings.Join(PreviewFreshnessReasonWords, ", "))
}

// ---------------------------------------------------------------------------
// The Timeline: three words derived from the append-only events
// ---------------------------------------------------------------------------

// PreviewEvent is one append-only row of the preparation history: the
// six-word kind, the safe reason word (the policy reason of a blocked
// proposal, the foreign-word refusal of the projections), the safe
// details (identifiers and words only — never a material value, never a
// secret) and the actor. The history is never rewritten.
type PreviewEvent struct {
	Kind    string    `json:"kind"`
	Reason  string    `json:"reason"`
	Details string    `json:"details"`
	At      time.Time `json:"at"`
	ActorID id.UUID   `json:"actor_id"`
}

// DerivePreviewState derives the Timeline word of one preview version from
// its append-only events (AC-11): a superseded version answers superseded
// (readable for the audit, unsuitable for the authorization — the
// suitability dies through the freshness derivation, never through a
// deletion); a built version answers previewed; anything else is the
// proposal stage. The dry-run and fresh-read events NEVER move the
// Timeline (the structural honesty of AC-8 — the kinds are simply not in
// the derivation). authorized/executed do not exist in this contour: the
// words belong to the 3-5/3-6 dictionaries and no rule here derives them.
func DerivePreviewState(events []PreviewEvent) string {
	for _, e := range events {
		if e.Kind == EventSuperseded {
			return TimelineSuperseded
		}
	}
	for _, e := range events {
		if e.Kind == EventPreviewed {
			return TimelinePreviewed
		}
	}
	return TimelineProposal
}

// TimelineTexts is the fixed honest text of the Timeline words.
var TimelineTexts = map[string]string{
	TimelineProposal:   "Proposal подан, preview ещё не построен.",
	TimelinePreviewed:  "Preview построен и показан.",
	TimelineSuperseded: "Версия заменена новой — читаема для аудита, к авторизации не пригодна.",
}

// TimelineText speaks the Timeline word; a foreign word fails closed.
func TimelineText(word string) string {
	if text, ok := TimelineTexts[word]; ok {
		return text
	}
	return fmt.Sprintf("Слово Timeline «%s» не входит в словарь (%s).", word, strings.Join(TimelineWords, ", "))
}

// ---------------------------------------------------------------------------
// The dry-run honesty (AC-8) and the fixed card texts
// ---------------------------------------------------------------------------

// GuardDryRunResult refuses a foreign dry-run result word — the evidence
// vocabulary is closed (valid, invalid, error); the adapters speak it or
// the evidence is not recorded.
func GuardDryRunResult(word string) error {
	if WordIn(DryRunResultWords, word) {
		return nil
	}
	return fmt.Errorf("Результат dry-run «%s» не входит в словарь (%s).", word, strings.Join(DryRunResultWords, ", "))
}

// DryRunHonestyText is the fixed card text of the dry-run discipline:
// validation evidence is neither an action nor a guarantee, and the
// Timeline words never move (AC-8).
const DryRunHonestyText = "Dry-run — это проверка валидации: не выполненное действие и не гарантия будущего успеха. Слова Timeline не смещаются; результат сохраняется как validation evidence."

// DryRunUnsupportedText is the honest refusal of the nil adapter (AG-4:
// the production composition root does not wire the DryRunner — real
// providers do not exist; the surface never pretends).
const DryRunUnsupportedText = "Dry-run не поддержан: валидационный адаптер не подключён (реальных провайдеров нет)."

// PreviewPanelText is the fixed intro of the Preview Review surface (the
// render-layer material — it never enters the store): the disciplines of
// the slice spoken once, the app view and the fixture world share the ONE
// text (an app-private copy would drift).
const PreviewPanelText = "Action Preview — подготовительный контур действия: proposal строго валидируется против типовой схемы определения (одна попытка исправления, повторная невалидность — терминальный отказ; неизвестное поле не входит в preview и никому не передаётся), Policy подготовки проверяется живыми источниками по порядку — текст proposal не является входом Policy, материальный snapshot берётся read-only из владеющей системы (stale, conflicting, unknown блокируют готовность; разрешённая неопределённость явна). Preview — неизменяемые версии append-only: редактирование = новая версия с diff, прежние версии читаемы для аудита, к авторизации не пригодны. Эффекты перечислены только из определения — открытый набор невозможен; unsafe и действия без доказанного reconciliation path остаются human-only с обязательным человеком-владельцем. Connector write request из подготовительного контура не существует."

// PreviewEmptyText is the honest reason of the empty view: the absence of
// previews is a readable state, never a defect.
const PreviewEmptyText = "В этом кейсе ещё нет action preview. Участник кейса с правом review или configuration подаёт proposal по определению из каталога: строгая типовая валидация, затем Policy живыми источниками и материальный snapshot построят первую версию preview."

// AuthorizationAbsenceText speaks the 3-5 contour at the primary action
// of the card: the button is the transition into the consent form of the
// Operation Review page, never a hidden dispatch — the contour exists
// (the 3-5 slice) and the transition explains itself in words (the
// «диспетчер не сконфигурирован» precedent 3-3 — the interface never
// pretends; the field name keeps the 3-4 wire contract, the words speak
// the present truth).
const AuthorizationAbsenceText = "Контур авторизации построен (3-5): переход ведёт в форму подтверждения точной версии (preview_id, version_no) на странице Operation Review; скрытой отправки не существует."
