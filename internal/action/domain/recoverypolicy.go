package domain

import (
	"fmt"
	"strings"
)

// Module action — story 3.9 slice 1 (step-01): the recovery action
// policy domain core (FR-39, the AC-1..AC-13 blocks of epic 3.9, first
// slice: the READ-TIME derivation of the allowed recovery-action set of
// ONE Recovery Case). This file owns the compile-time dictionaries of the
// policy (the FIVE kind words of the standard recovery policy of the
// epic, the TWO human-only decision words listed SEPARATELY from the
// allowed set — AC-1 «отдельно указывает human-only решения», the THREE
// state words of the per-kind read-time derivation), the DECLARED
// compile-time attributes of every kind (scope, the eligibility rule,
// the required facts, the authority and the Confirmation flag — AC-2
// дословно: «каждый вариант имеет scope, eligibility, требуемые факты,
// authority и Confirmation»), and the derivation ladder itself
// (DeriveRecoveryActionPolicy — a pure function over the facts; every
// answer is a state word with the honest reason text, NEVER a write).
//
// The policy is a DERIVATION, not a record (the slice honesty core): the
// state of a kind is NEVER stored — every request derives it anew over
// the LIVE readings (the case row, the operation facts, the scoped
// control word, the catalog definition declaration, the AG-4 composition
// fact); a repeated request over an unchanged world answers the SAME
// view byte-stably (the idempotence is the derivation itself). Not ONE
// row lands in any table — this slice is the first Epic 3 slice without
// SQL (read-time output only; the migrations stay at 00005). Not ONE
// network call rides the path: the adapters are structurally absent
// (AG-4 — no AI Runtime exists; «текст модели не добавляет новый
// вариант» is STRUCTURAL: the set is the output of deterministic code
// over the recorded, the generative path does not exist — the model
// confidence never participates; a future planner reads THIS policy as
// the machine view to validate its own proposals, AD-7).
//
// The words are frozen in code; runtime or tenant-config extension is
// NEVER allowed (the 3-1 compile-time dictionary discipline — «не
// расширять словари рантаймом»).

// ---------------------------------------------------------------------------
// The dictionaries (compile-time, closed)
// ---------------------------------------------------------------------------

// Recovery action kind words — the standard recovery policy vocabulary
// of the epic (five). request_evidence — the material evidence request
// to the Owning System (a future slice wires the external Commitment;
// this slice declares the right with the Confirmation requirement);
// show_recorded_conditions — the read-only show of the RECORDED
// conditions (the existing 3-8 card surface: the exact-version receipt
// chain, the preview pins, the disputed/unknown words); truthful_status_
// update — the read-only truthful status words (the existing 3-8 honest
// words: the local projection, the AG-4 unknown payments; «время
// следующего обновления» = the assignment deadline — an update promise
// is NEVER created); offer_approved_options — the offer of the approved
// resolution options to the client (honestly human_only in this slice:
// NO approved options exist in the configuration — the AC-1 tail
// «отсутствующее правило означает human_only» materialized); prepare_
// pause_cancel — the preparation of a pause/cancel with compensation
// (allowed ONLY when the catalog definition declares a non-empty
// CancelCompensationSem semantics — the 3-1 vocabulary readiness; the
// execution contour of the kinds is future slices through the Action
// Gateway, AD-6). An out-of-dictionary action is IMPOSSIBLE: the
// dictionary is compile-time, the OpenAPI/domain mirrors are pinned by
// the contract test (a drift turns it red).
const (
	RecoveryActionRequestEvidence        = "request_evidence"
	RecoveryActionShowRecordedConditions = "show_recorded_conditions"
	RecoveryActionTruthfulStatusUpdate   = "truthful_status_update"
	RecoveryActionOfferApprovedOptions   = "offer_approved_options"
	RecoveryActionPreparePauseCancel     = "prepare_pause_cancel"
)

// RecoveryActionKindWords is the full kind dictionary (5 words).
var RecoveryActionKindWords = []string{
	RecoveryActionRequestEvidence,
	RecoveryActionShowRecordedConditions,
	RecoveryActionTruthfulStatusUpdate,
	RecoveryActionOfferApprovedOptions,
	RecoveryActionPreparePauseCancel,
}

// Recovery human-only decision words — the SEPARATE class of the
// decisions that belong to a human ONLY (two), listed APART from the
// allowed set (AC-1: «отдельно указывает human-only решения»).
// liability_or_fault — the admission of fault / the legal liability
// conclusion; nonstandard_outcome — the non-standard compensation,
// refund or exception. The boundary words are the PRD FR-39
// consequences verbatim: the fault admission, the legal conclusion and
// the non-standard refund/compensation require a human — Staffy never
// performs them, in this slice and in the future ones (the class is a
// BOUNDARY declaration, not an action: nothing to execute, nothing to
// confirm — the human decides outside the system).
const (
	RecoveryHumanOnlyLiabilityOrFault   = "liability_or_fault"
	RecoveryHumanOnlyNonstandardOutcome = "nonstandard_outcome"
)

// RecoveryHumanOnlyWords is the full human-only dictionary (2 words).
var RecoveryHumanOnlyWords = []string{
	RecoveryHumanOnlyLiabilityOrFault,
	RecoveryHumanOnlyNonstandardOutcome,
}

// Recovery action state words — the per-kind read-time answer
// vocabulary (three). allowed — the kind is within the right of the
// system at this moment (a material kind still carries its Confirmation
// requirement — «материальный recovery-вариант требует отдельного
// явного подтверждения», the AC-13 minimum); blocked — the boundary
// blocks the kind (the paused scoped control blocks the MATERIAL kinds
// with the exact control word — the reading stays allowed, the AC-10 of
// 3.7 inherited); human_only — the rule is missing or ambiguous, the
// decision belongs to a human (the fail-closed AC-1 tail: «отсутствующее
// либо неоднозначное правило означает human_only» — the right limits
// itself honestly, the derivation NEVER refuses the whole view).
const (
	RecoveryActionStateAllowed   = "allowed"
	RecoveryActionStateBlocked   = "blocked"
	RecoveryActionStateHumanOnly = "human_only"
)

// RecoveryActionStateWords is the full state dictionary (3 words).
var RecoveryActionStateWords = []string{
	RecoveryActionStateAllowed,
	RecoveryActionStateBlocked,
	RecoveryActionStateHumanOnly,
}

// ---------------------------------------------------------------------------
// The bounded constants (the 3-3/3-6/3-7/3-8 precedents)
// ---------------------------------------------------------------------------

// RecoveryPolicyTextRunes is the rune bound of every honest text of the
// policy surfaces (the RecoverySafeDetailsRunes precedent — the words
// stay bounded, the surfaces stay renderable; the unit matrix pins every
// compile-time text and every derived state word against it).
const RecoveryPolicyTextRunes = 500

// ---------------------------------------------------------------------------
// The DECLARED compile-time attributes of every kind (AC-2 дословно)
// ---------------------------------------------------------------------------

// RecoveryActionKind is the compile-time declaration of ONE allowed kind
// of the recovery action policy: the kind word, the scope text (the
// area the word acts over), the eligibility rule text (the read-time
// conditions of the derivation), the required facts text (the RECORDED
// facts the kind leans on), the authority text (whose right the kind
// is: the operator manage word or the human word) and the Confirmation
// flag (the material kinds carry true — «требует отдельного явного
// подтверждения», the AC-13 minimum; the read-only kinds carry false).
// The attributes are CONSTANTS of the kind — they never depend on the
// read-time state (the state rides separately with its own honest word).
type RecoveryActionKind struct {
	Word                 string
	ScopeText            string
	EligibilityRuleText  string
	RequiredFactsText    string
	AuthorityText        string
	ConfirmationRequired bool
	// Material marks the kinds whose execution would carry a material
	// effect (request_evidence, prepare_pause_cancel,
	// offer_approved_options): the paused scoped control BLOCKS these
	// kinds with the exact control word; the read-only kinds stay
	// allowed («приостановленный контроль блокирует материальное
	// действие, чтение остаётся» — the AC-10 of 3.7 inherited, the
	// recovery-scope permissions of a paused connector are NOT
	// introduced: the 3-7 debt stays documented, the reconcile behavior
	// never changes in this slice).
	Material bool
}

// RecoveryActionKinds is the compile-time kind table of the policy —
// the FIVE declarations in the dictionary order (the fixed order of the
// machine view and the page section; the table is frozen — a new kind
// is a dictionary review, never a runtime extension).
var RecoveryActionKinds = []RecoveryActionKind{
	{
		Word:                 RecoveryActionRequestEvidence,
		ScopeText:            "Запрос доказательств у Owning System по операции восстановления кейса (внешний evidence-запрос).",
		EligibilityRuleText:  "Назначение кейса живое (owner/backup/дедлайн — живые чтения 3-8); scoped control маршрута не paused; требуется отдельное явное подтверждение оператора.",
		RequiredFactsText:    "owner назначен, канал, purpose (назначение кейса + маршрут исходной conversation).",
		AuthorityText:        "operator manage (оператор с полномочием operations).",
		ConfirmationRequired: true,
		Material:             true,
	},
	{
		Word:                 RecoveryActionShowRecordedConditions,
		ScopeText:            "Показ записанных условий операции и квитанций на карточке кейса восстановления (read-only чтение записанного).",
		EligibilityRuleText:  "Всегда доступен в срезе: чтение записанного — существующая поверхность карточки кейса 3-8 (цепочка квитанций точными версиями живой деривацией).",
		RequiredFactsText:    "SAFE-пины bundle + живая цепочка квитанций/попыток карточки кейса 3-8.",
		AuthorityText:        "operator manage (оператор с полномочием operations).",
		ConfirmationRequired: false,
		Material:             false,
	},
	{
		Word:                 RecoveryActionTruthfulStatusUpdate,
		ScopeText:            "Правдивые статусные слова о состоянии операции восстановления (read-only слова, без обещания обновления).",
		EligibilityRuleText:  "Всегда доступен в срезе: статусные слова — существующие honest-слова карточки 3-8; «время следующего обновления» = дедлайн назначения кейса; обещание обновления НЕ создаётся.",
		RequiredFactsText:    "Статусные слова 3-8 (локальная проекция, unknown-платежи AG-4) + дедлайн назначения кейса; слово «сверка не подключена» в фактах при AG-4.",
		AuthorityText:        "operator manage (оператор с полномочием operations).",
		ConfirmationRequired: false,
		Material:             false,
	},
	{
		Word:                 RecoveryActionOfferApprovedOptions,
		ScopeText:            "Предложение утверждённых вариантов решения клиенту (выбор варианта = новая версия Commitment).",
		EligibilityRuleText:  "Честно human_only всегда в срезе: утверждённых вариантов решения в конфигурации нет — отсутствующее правило означает human_only (AC-1 хвост, fail-closed).",
		RequiredFactsText:    "Утверждённая конфигурация вариантов решения (отсутствует — словарь вариантов будущего среза, задокументировано).",
		AuthorityText:        "человек (human_only — решение за человеком).",
		ConfirmationRequired: true,
		Material:             true,
	},
	{
		Word:                 RecoveryActionPreparePauseCancel,
		ScopeText:            "Подготовка приостановки/отмены с компенсацией по объявленной семантике каталога (Action Preview будущий срез).",
		EligibilityRuleText:  "Scoped control маршрута не paused; CancelCompensationSem семантика LIVE-декларации каталога непуста и читаема; требуется отдельное явное подтверждение оператора.",
		RequiredFactsText:    "Определение + LIVE CancelCompensationSem + owner (пины определения операции и назначение кейса).",
		AuthorityText:        "operator manage (оператор с полномочием operations).",
		ConfirmationRequired: true,
		Material:             true,
	},
}

// RecoveryActionKindByWord resolves the compile-time declaration of ONE
// kind word (nil — a foreign word; the dictionary is the frozen truth).
func RecoveryActionKindByWord(word string) *RecoveryActionKind {
	for i := range RecoveryActionKinds {
		if RecoveryActionKinds[i].Word == word {
			return &RecoveryActionKinds[i]
		}
	}
	return nil
}

// RecoveryHumanOnlyDecision is the compile-time declaration of ONE
// human-only decision word: the word with its boundary text (the PRD
// FR-39 consequence — the decision belongs to a human ONLY, the system
// never performs it, never prepares it and never confirms it).
type RecoveryHumanOnlyDecision struct {
	Word         string
	BoundaryText string
}

// RecoveryHumanOnlyDecisions is the compile-time human-only table — the
// TWO declarations in the dictionary order, listed SEPARATELY from the
// allowed set (AC-1).
var RecoveryHumanOnlyDecisions = []RecoveryHumanOnlyDecision{
	{
		Word:         RecoveryHumanOnlyLiabilityOrFault,
		BoundaryText: "Только человек — признание вины и юридический вывод: Staffy не признаёт вину и не делает юридических выводов (PRD FR-39).",
	},
	{
		Word:         RecoveryHumanOnlyNonstandardOutcome,
		BoundaryText: "Только человек — нестандартная компенсация, возврат или исключение: вне стандартной recovery policy решение принимает человек (PRD FR-39).",
	},
}

// RecoveryHumanOnlyClassText is the class-level boundary word of the
// human-only decisions — the PRD FR-39 consequence verbatim (the surface
// speaks it with the class, the per-word texts carry the detail).
const RecoveryHumanOnlyClassText = "Признание вины, юридический вывод и нестандартный возврат/компенсация требуют человека."

// ---------------------------------------------------------------------------
// The read-time derivation ladder over the LIVE readings
// ---------------------------------------------------------------------------

// RecoveryPolicyFacts are the LIVE facts of ONE Recovery Case the
// derivation reads — nothing here is a command parameter or a network
// answer: the case state word is the read-time derivation over the
// append-only events (DeriveRecoveryCaseState — «open» is the ONE word
// of the slice), the scoped control half is the effective mode word of
// the tenant/connector route with its winning reason ("" — the control
// is unreadable: the nil port or the resolution error fails closed by
// the honest unreadable word), the definition half reports whether the
// catalog resolved the operation's definition pins and carries the
// declared CancelCompensationSem semantics word ("" — the semantics is
// not declared), and the AG-4 flag is the composition fact of the wired
// reconciliation adapter (nil = the honest production boundary — the
// policy is NEVER blocked by it, the honest word rides the status
// facts). The app composes them in ONE read pass (step-02) — the
// derivation is a pure function over the facts, NEVER a write on any
// path (the fake-repo counters pin the zero-write property).
type RecoveryPolicyFacts struct {
	// CaseStateWord is the read-time state word of the case (the
	// DeriveRecoveryCaseState derivation — «open» in this slice; a
	// foreign word fails closed).
	CaseStateWord string
	// ScopedControlMode is the effective scoped control mode word of
	// the definition's route (accept / intake_only/no_promises /
	// human_only / paused; "" — unreadable: the nil port or the
	// resolution error).
	ScopedControlMode string
	// ScopedControlReason is the winning reason of the effective control
	// (rides the exact paused word of the blocked material kinds).
	ScopedControlReason string
	// DefinitionReadable reports whether the catalog resolved the
	// definition pins of the operation (false — the fail-closed human
	// path of the dependent kinds; the route cannot be named either).
	DefinitionReadable bool
	// CancelCompensationSem is the declared cancel/compensation
	// semantics word of the resolved definition ("" — not declared).
	CancelCompensationSem string
	// ReconciliationWired reports whether the reconciliation adapter is
	// wired (the AG-4 composition fact — nil is the honest production).
	ReconciliationWired bool
}

// RecoveryPolicyAction is ONE kind's policy answer: the DECLARED
// compile-time attributes of the kind (AC-2 дословно — scope,
// eligibility, требуемые факты, authority, Confirmation) with the
// read-time state word and the honest reason text. The state NEVER
// promises the success of the action (the policy is the boundary of the
// RIGHT, not an execution plan — every honest word says what is
// permitted, never what will happen).
type RecoveryPolicyAction struct {
	Kind                 string
	State                string
	StateText            string
	ScopeText            string
	EligibilityRuleText  string
	RequiredFactsText    string
	AuthorityText        string
	ConfirmationRequired bool
}

// RecoveryPolicyHumanOnly is ONE human-only decision word of the
// SEPARATE class with its boundary text (AC-1: the class is listed
// apart from the allowed set — the words never mix).
type RecoveryPolicyHumanOnly struct {
	Kind         string
	BoundaryText string
}

// RecoveryPolicy is the derivation answer of ONE case: the FIVE kind
// answers in the dictionary order + the human-only class (two words
// with their boundary texts) + the class-level boundary word. The view
// is the COMPLETE machine surface (the transport renders it as-is in
// step-03; the future planner validates its proposals against it,
// AD-7).
type RecoveryPolicy struct {
	Actions            []RecoveryPolicyAction
	HumanOnly          []RecoveryPolicyHumanOnly
	HumanOnlyClassText string
}

// The honest texts of the policy states (the fixed surfaces of the
// machine view and the page — the card/API/page share the exact words;
// an app-private copy would drift, the operation-view precedent).
const (
	// RecoveryActionPausedTextTemplate is the blocked word of the
	// material kinds under the PAUSED scoped control: the EXACT control
	// word rides the line (the reconcilePausedWord precedent of 3-7 —
	// the mode with the winning reason; «приостановленный контроль
	// блокирует материальное действие, чтение остаётся» — AC-10 of 3.7
	// inherited; the recovery-scope permissions are NOT introduced —
	// the 3-7 debt stays documented, the reconcile behavior never
	// changes).
	RecoveryActionPausedTextTemplate = "Заблокировано scoped control: %s. Приостановленный контроль блокирует материальное действие — чтение остаётся доступным (read-only виды не блокируются)."
	// RecoveryActionControlUnreadableText is the blocked word of the
	// material kinds under the UNREADABLE scoped control (the nil port
	// or the resolution error — fail-closed by reading, the most safe
	// applicable word: the material kinds never open on an unreadable
	// control, the reading stays).
	RecoveryActionControlUnreadableText = "Scoped control маршрута нечитаем — материальные виды ограничены честным словом (fail-closed чтением, самое строгое применимое); чтение остаётся доступным."
	// RecoveryActionDefinitionUnreadableText is the human_only word of
	// the definition-dependent kinds when the catalog cannot resolve the
	// operation's definition pins (the route cannot be named, the
	// semantics cannot be read — the missing rule means human_only, the
	// AC-1 tail; the view NEVER refuses: the right limits itself).
	RecoveryActionDefinitionUnreadableText = "Определение операции нечитаемо каталогом — pause/cancel семантика не объявлена: подготовка за человеком (отсутствующее правило = human_only, fail-closed)."
	// RecoveryActionSemanticsNotDeclaredText is the human_only word of
	// prepare_pause_cancel when the resolved definition declares NO
	// CancelCompensationSem semantics (the empty declaration — the
	// preparation stays with the human, the AC-1 tail).
	RecoveryActionSemanticsNotDeclaredText = "Pause/cancel семантика не объявлена каталогом — подготовка за человеком (отсутствующее правило = human_only, fail-closed)."
	// RecoveryActionNoApprovedOptionsText is the human_only word of
	// offer_approved_options — ALWAYS in this slice: no approved
	// resolution options exist in the configuration (the options
	// dictionary is a future slice; the AC-1 tail materialized).
	RecoveryActionNoApprovedOptionsText = "Утверждённых вариантов решения в конфигурации нет — только человек (отсутствующее правило = human_only, AC-1 fail-closed)."
	// RecoveryActionRequestEvidenceAllowedText is the allowed word of
	// request_evidence (the material kind — the Confirmation
	// requirement rides the word, the AC-13 minimum).
	RecoveryActionRequestEvidenceAllowedText = "Разрешён: запрос доказательств — материальный вид, требует отдельного явного подтверждения оператора (назначение кейса живое, scoped control не paused)."
	// RecoveryActionShowConditionsAllowedText is the allowed word of
	// show_recorded_conditions (the read-only kind — the existing 3-8
	// card surface is the material).
	RecoveryActionShowConditionsAllowedText = "Разрешён (read-only): записанные условия — существующая поверхность карточки кейса (цепочка квитанций точными версиями, пины превью, спорное/unknown явно)."
	// RecoveryActionStatusUpdateAllowedText is the allowed word of
	// truthful_status_update (the read-only kind — the existing 3-8
	// honest words; the AG-4 sentence appends when the reconciliation
	// adapter is not wired).
	RecoveryActionStatusUpdateAllowedText = "Разрешён (read-only): статусные слова — существующие honest-слова карточки (локальная проекция, unknown-платежи); «время следующего обновления» = дедлайн назначения кейса; обещание обновления не создаётся."
	// RecoveryActionAG4NotWiredText is the AG-4 honest sentence that
	// appends to the status words when the reconciliation adapter is
	// not wired (the composition fact — the policy is never blocked by
	// it, the word rides the facts of the kind).
	RecoveryActionAG4NotWiredText = "Сверка не подключена (AG-4) — актуальность внешнего состояния не подтверждается."
	// RecoveryActionPauseCancelAllowedText is the allowed word of
	// prepare_pause_cancel (the material kind — the LIVE semantics
	// declared, the Confirmation requirement rides the word).
	RecoveryActionPauseCancelAllowedText = "Разрешён: LIVE-семантика CancelCompensationSem объявлена каталогом — подготовка pause/cancel материальный вид, требует отдельного явного подтверждения оператора."
)

// recoveryScopedControlPausedWord renders the exact paused word of the
// scoped control with its winning reason (the reconcilePausedWord canon
// of 3-7: the mode with the reason; an empty reason speaks the mode
// alone — both ride the honest line of the blocked material kinds).
func recoveryScopedControlPausedWord(mode, reason string) string {
	if strings.TrimSpace(reason) == "" {
		return mode
	}
	return mode + ": " + reason
}

// DeriveRecoveryActionPolicy derives the READ-TIME recovery action
// policy of ONE case over the LIVE facts — the ordered ladder per kind
// (the spec order: the case state → the scoped control fence of the
// MATERIAL kinds → the catalog declaration fence of the
// definition-dependent kinds → the AG-4 fact in the status words):
//
//   - show_recorded_conditions / truthful_status_update — the read-only
//     kinds: ALWAYS allowed (the reading stays under every control
//     word — «приостановленный контроль блокирует материальное
//     действие, чтение остаётся»); the status kind appends the AG-4
//     sentence when the reconciliation adapter is not wired;
//   - request_evidence — the material kind: the scoped control fence
//     (paused → blocked with the EXACT control word; unreadable →
//     blocked with the honest fail-closed word), otherwise allowed
//     with the Confirmation requirement;
//   - offer_approved_options — the material kind that is honestly
//     human_only ALWAYS in this slice (no approved options exist in
//     the configuration — the AC-1 tail; the control fence does not
//     apply: a human decision never needs the automation unpaused);
//   - prepare_pause_cancel — the material, definition-dependent kind:
//     the unreadable definition → human_only (the missing rule — the
//     route cannot be named); the paused/unreadable control → blocked;
//     the empty CancelCompensationSem semantics → human_only; otherwise
//     allowed with the Confirmation requirement.
//
// Fail-closed on the foreign fact words (the case state word, the
// scoped control mode): the honest RuleError names the dictionary — a
// foreign word is a structural alarm, never a masked verdict (the
// DeriveRecoveryEligibility precedent; the view-level misses — the
// unreadable definition/control — are the HUMAN paths, they never
// refuse the whole view). The derivation NEVER writes: the callers pin
// it with the fake-repo counters (step-02); the answer is a pure
// function of the facts (the repeated request over the unchanged world
// answers the SAME view byte-stably — the idempotence is the
// derivation itself).
func DeriveRecoveryActionPolicy(f RecoveryPolicyFacts) (RecoveryPolicy, error) {
	if !WordIn(RecoveryCaseStateWords, f.CaseStateWord) {
		return RecoveryPolicy{}, RuleError{Rule: "DeriveRecoveryActionPolicy", Reason: fmt.Sprintf(
			"Состояние Recovery Case «%s» вне словаря (%s).", f.CaseStateWord, strings.Join(RecoveryCaseStateWords, ", "))}
	}
	if f.ScopedControlMode != "" && !WordIn(ScopedControlModeWords, f.ScopedControlMode) {
		return RecoveryPolicy{}, RuleError{Rule: "DeriveRecoveryActionPolicy", Reason: fmt.Sprintf(
			"Слово scoped control «%s» вне словаря (%s).", f.ScopedControlMode, strings.Join(ScopedControlModeWords, ", "))}
	}
	// The scoped control fence of the MATERIAL kinds: the paused word
	// blocks with the EXACT control word (the mode with the winning
	// reason — the reconcilePausedWord canon); the unreadable control
	// blocks with the honest fail-closed word; every other mode word
	// passes (the conservative slice: ONLY paused blocks — the
	// recovery-scope permissions of a paused connector are the
	// documented 3-7 debt, the other mode words discipline the material
	// preparation of the FUTURE execution slices through their own
	// GuardPreparePolicy gates).
	materialBlocked, materialBlockedText := "", ""
	if f.ScopedControlMode == ScopedControlPaused {
		materialBlocked = RecoveryActionStateBlocked
		materialBlockedText = fmt.Sprintf(RecoveryActionPausedTextTemplate,
			recoveryScopedControlPausedWord(f.ScopedControlMode, f.ScopedControlReason))
	} else if f.ScopedControlMode == "" {
		materialBlocked = RecoveryActionStateBlocked
		materialBlockedText = RecoveryActionControlUnreadableText
	}
	// The AG-4 sentence of the status words (the composition fact — the
	// policy is never blocked by it).
	ag4Sentence := ""
	if !f.ReconciliationWired {
		ag4Sentence = " " + RecoveryActionAG4NotWiredText
	}
	policy := RecoveryPolicy{
		Actions:            make([]RecoveryPolicyAction, 0, len(RecoveryActionKinds)),
		HumanOnly:          make([]RecoveryPolicyHumanOnly, 0, len(RecoveryHumanOnlyDecisions)),
		HumanOnlyClassText: RecoveryHumanOnlyClassText,
	}
	for _, kind := range RecoveryActionKinds {
		answer := RecoveryPolicyAction{
			Kind:                 kind.Word,
			State:                RecoveryActionStateAllowed,
			ScopeText:            kind.ScopeText,
			EligibilityRuleText:  kind.EligibilityRuleText,
			RequiredFactsText:    kind.RequiredFactsText,
			AuthorityText:        kind.AuthorityText,
			ConfirmationRequired: kind.ConfirmationRequired,
		}
		switch kind.Word {
		case RecoveryActionShowRecordedConditions:
			answer.StateText = RecoveryActionShowConditionsAllowedText
		case RecoveryActionTruthfulStatusUpdate:
			answer.StateText = RecoveryActionStatusUpdateAllowedText + ag4Sentence
		case RecoveryActionRequestEvidence:
			if materialBlocked != "" {
				answer.State = materialBlocked
				answer.StateText = materialBlockedText
			} else {
				answer.StateText = RecoveryActionRequestEvidenceAllowedText
			}
		case RecoveryActionOfferApprovedOptions:
			// ALWAYS human_only in this slice — the AC-1 tail
			// materialized: no approved options exist in the
			// configuration (the control fence does not apply: the
			// human decision never needs the automation unpaused).
			answer.State = RecoveryActionStateHumanOnly
			answer.StateText = RecoveryActionNoApprovedOptionsText
		case RecoveryActionPreparePauseCancel:
			if !f.DefinitionReadable {
				// The route cannot be named — the missing rule means
				// human_only (the view never refuses).
				answer.State = RecoveryActionStateHumanOnly
				answer.StateText = RecoveryActionDefinitionUnreadableText
			} else if materialBlocked != "" {
				answer.State = materialBlocked
				answer.StateText = materialBlockedText
			} else if strings.TrimSpace(f.CancelCompensationSem) == "" {
				answer.State = RecoveryActionStateHumanOnly
				answer.StateText = RecoveryActionSemanticsNotDeclaredText
			} else {
				answer.StateText = RecoveryActionPauseCancelAllowedText
			}
		}
		policy.Actions = append(policy.Actions, answer)
	}
	for _, decision := range RecoveryHumanOnlyDecisions {
		policy.HumanOnly = append(policy.HumanOnly, RecoveryPolicyHumanOnly{
			Kind:         decision.Word,
			BoundaryText: decision.BoundaryText,
		})
	}
	return policy, nil
}
