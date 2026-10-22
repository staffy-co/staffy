package domain

import (
	"reflect"
	"strings"
	"testing"
)

// The 3.9 step-01 unit matrix of the recovery action policy domain —
// the closed dictionaries by name (the FIVE kind words of the standard
// recovery policy of the epic, the TWO human-only decision words listed
// SEPARATELY, the THREE state words, the bounded text constant), the
// DECLARED compile-time attributes of every kind (AC-2 дословно: scope,
// eligibility, требуемые факты, authority, Confirmation — the material
// kinds carry the flag, the read-only kinds do not), the read-time
// derivation ladder over the LIVE facts (the kind × facts matrix: the
// full allowed set of the exhausted case, the paused scoped control
// blocking the MATERIAL kinds with the EXACT control word while the
// read-only kinds stay allowed, the unreadable control fail-closed, the
// unreadable definition → human_only of the dependent kind, the empty
// CancelCompensationSem semantics → human_only, offer_approved_options
// honestly human_only ALWAYS, the AG-4 sentence in the status words),
// the fail-closed foreign fact words (the case state word and the
// scoped control mode word — the honest RuleError names the
// dictionary), the byte-stable idempotence of the derivation (the
// repeated call answers the SAME view), and the honest texts of the
// surfaces (every compile-time text and every derived state word within
// the rune bound; the human-only class boundary words — the PRD FR-39
// consequences verbatim).

// ---------------------------------------------------------------------------
// The dictionaries: closure, the attributes and the bounded constants
// ---------------------------------------------------------------------------

// TestRecoveryPolicyDictionariesByName pins the closed dictionaries of
// the policy: виды — ровно пять слов стандартной recovery policy эпика,
// human-only класс — ровно два слова (ОТДЕЛЬНО от набора видов),
// состояния вида — ровно три; буйнд текстов политики 500 (прецедент
// SAFE-details); словарь 3-8 состояний кейса стабилен (open — единственное
// слово, замороженная секция не растёт).
func TestRecoveryPolicyDictionariesByName(t *testing.T) {
	wantKinds := []string{
		"request_evidence", "show_recorded_conditions", "truthful_status_update",
		"offer_approved_options", "prepare_pause_cancel",
	}
	if !reflect.DeepEqual(RecoveryActionKindWords, wantKinds) {
		t.Fatalf("словарь видов политики: %v, ожидалось ровно %v", RecoveryActionKindWords, wantKinds)
	}
	wantHumanOnly := []string{"liability_or_fault", "nonstandard_outcome"}
	if !reflect.DeepEqual(RecoveryHumanOnlyWords, wantHumanOnly) {
		t.Fatalf("словарь human-only решений: %v, ожидалось ровно %v", RecoveryHumanOnlyWords, wantHumanOnly)
	}
	wantStates := []string{"allowed", "blocked", "human_only"}
	if !reflect.DeepEqual(RecoveryActionStateWords, wantStates) {
		t.Fatalf("словарь состояний вида: %v, ожидалось ровно %v", RecoveryActionStateWords, wantStates)
	}
	if RecoveryPolicyTextRunes != 500 {
		t.Fatalf("буйнд текстов политики: %d, ожидалось 500 (прецедент SAFE-details)", RecoveryPolicyTextRunes)
	}
	// The kinds and the human-only words NEVER mix (AC-1: the class is
	// listed SEPARATELY from the allowed set).
	for _, word := range RecoveryHumanOnlyWords {
		if WordIn(RecoveryActionKindWords, word) {
			t.Fatalf("human-only слово «%s» не должно попадать в набор видов (класс перечислен отдельно)", word)
		}
	}
	// The 3-8 dictionaries stay frozen (the case state word is the ONE
	// word of the slice — the policy reads it, never extends it).
	if !reflect.DeepEqual(RecoveryCaseStateWords, []string{RecoveryCaseStateOpen}) {
		t.Fatalf("словарь состояний Recovery Case 3-8 тронут: %v", RecoveryCaseStateWords)
	}
}

// TestRecoveryActionKindAttributes pins the DECLARED compile-time
// attributes of every kind — AC-2 дословно: «каждый вариант имеет
// scope, eligibility, требуемые факты, authority и Confirmation».
// Материальные виды {request_evidence, prepare_pause_cancel,
// offer_approved_options} несут ConfirmationRequired=true;
// read-only {show_recorded_conditions, truthful_status_update} — false.
func TestRecoveryActionKindAttributes(t *testing.T) {
	if len(RecoveryActionKinds) != len(RecoveryActionKindWords) {
		t.Fatalf("таблица видов %d записей, словарь %d слов — таблица обязана покрывать словарь", len(RecoveryActionKinds), len(RecoveryActionKindWords))
	}
	wantMaterial := map[string]bool{
		RecoveryActionRequestEvidence:        true,
		RecoveryActionShowRecordedConditions: false,
		RecoveryActionTruthfulStatusUpdate:   false,
		RecoveryActionOfferApprovedOptions:   true,
		RecoveryActionPreparePauseCancel:     true,
	}
	seen := map[string]bool{}
	for _, kind := range RecoveryActionKinds {
		if kind.Word == "" {
			t.Fatalf("пустое слово вида в таблице")
		}
		if seen[kind.Word] {
			t.Fatalf("дубликат вида «%s» в таблице", kind.Word)
		}
		seen[kind.Word] = true
		// Every attribute text is non-empty and bounded.
		for name, text := range map[string]string{
			"scope": kind.ScopeText, "eligibility": kind.EligibilityRuleText,
			"required_facts": kind.RequiredFactsText, "authority": kind.AuthorityText,
		} {
			if strings.TrimSpace(text) == "" {
				t.Fatalf("вид «%s»: атрибут %s пуст — AC-2 требует полные атрибуты", kind.Word, name)
			}
			if len([]rune(text)) > RecoveryPolicyTextRunes {
				t.Fatalf("вид «%s»: атрибут %s длиннее буйнда %d рун", kind.Word, name, RecoveryPolicyTextRunes)
			}
		}
		if kind.ConfirmationRequired != wantMaterial[kind.Word] {
			t.Fatalf("вид «%s»: ConfirmationRequired=%v, ожидалось %v (материальность вида)", kind.Word, kind.ConfirmationRequired, wantMaterial[kind.Word])
		}
		if kind.Material != wantMaterial[kind.Word] {
			t.Fatalf("вид «%s»: Material=%v, ожидалось %v", kind.Word, kind.Material, wantMaterial[kind.Word])
		}
	}
	for _, word := range RecoveryActionKindWords {
		if !seen[word] {
			t.Fatalf("вид «%s» словаря отсутствует в таблице атрибутов", word)
		}
	}
	// The resolver answers the declaration of every dictionary word and
	// refuses a foreign word (nil — вне-словарное действие невозможно).
	for _, word := range RecoveryActionKindWords {
		if RecoveryActionKindByWord(word) == nil {
			t.Fatalf("резолвер не отвечает вид «%s»", word)
		}
	}
	if RecoveryActionKindByWord("cancel_everything") != nil {
		t.Fatalf("резолвер ответил вне-словарному виду — словарь обязан быть закрыт")
	}
}

// TestRecoveryHumanOnlyBoundaryTexts pins the human-only class: the
// per-word boundary texts + the class-level word — the PRD FR-39
// consequences verbatim («признание вины, юридический вывод и
// нестандартный возврат/компенсация требуют человека»), bounded.
func TestRecoveryHumanOnlyBoundaryTexts(t *testing.T) {
	if len(RecoveryHumanOnlyDecisions) != len(RecoveryHumanOnlyWords) {
		t.Fatalf("таблица human-only %d записей, словарь %d слов", len(RecoveryHumanOnlyDecisions), len(RecoveryHumanOnlyWords))
	}
	for _, decision := range RecoveryHumanOnlyDecisions {
		if strings.TrimSpace(decision.BoundaryText) == "" {
			t.Fatalf("human-only «%s»: текст границы пуст", decision.Word)
		}
		if len([]rune(decision.BoundaryText)) > RecoveryPolicyTextRunes {
			t.Fatalf("human-only «%s»: текст границы длиннее буйнда", decision.Word)
		}
	}
	if !strings.Contains(RecoveryHumanOnlyClassText, "требуют человека") {
		t.Fatalf("класс-текст human-only обязан нести PRD FR-39 дословно: %q", RecoveryHumanOnlyClassText)
	}
}

// ---------------------------------------------------------------------------
// The derivation ladder: the kind × facts matrix
// ---------------------------------------------------------------------------

// policyFactsFixture is the LIVE facts of the exhausted-case fixture:
// open case, accept control, readable definition with the declared
// CancelCompensationSem semantics, the wired reconciliation (the full
// allowed set of the I/O matrix row 1).
func policyFactsFixture() RecoveryPolicyFacts {
	return RecoveryPolicyFacts{
		CaseStateWord:         RecoveryCaseStateOpen,
		ScopedControlMode:     ScopedControlAccept,
		ScopedControlReason:   "",
		DefinitionReadable:    true,
		CancelCompensationSem: "отмена с компенсацией до accepted",
		ReconciliationWired:   true,
	}
}

// policyActionByKind resolves one kind answer of the derived policy.
func policyActionByKind(t *testing.T, policy RecoveryPolicy, kind string) RecoveryPolicyAction {
	t.Helper()
	for _, action := range policy.Actions {
		if action.Kind == kind {
			return action
		}
	}
	t.Fatalf("вид «%s» отсутствует в выводе политики", kind)
	return RecoveryPolicyAction{}
}

// TestRecoveryPolicyLadderFullAllowedSet pins the full allowed set of
// the open exhausted case (the I/O matrix row 1): показ условий и
// статусные слова allowed (read-only), request_evidence allowed
// (материальный, подтверждение), prepare_pause_cancel allowed
// (LIVE-семантика объявлена), offer_approved_options честно human_only
// («утверждённых вариантов нет — решение за человеком»); human-only
// класс перечислен отдельно — AC-1/AC-2.
func TestRecoveryPolicyLadderFullAllowedSet(t *testing.T) {
	policy, err := DeriveRecoveryActionPolicy(policyFactsFixture())
	if err != nil {
		t.Fatalf("вывод политики отказал: %v", err)
	}
	if len(policy.Actions) != 5 {
		t.Fatalf("вывод обязан нести ровно 5 видов, несёт %d", len(policy.Actions))
	}
	show := policyActionByKind(t, policy, RecoveryActionShowRecordedConditions)
	if show.State != RecoveryActionStateAllowed || !strings.Contains(show.StateText, "существующая поверхность карточки кейса") {
		t.Fatalf("show_recorded_conditions: state=%s text=%q — ожидался allowed со словом поверхности", show.State, show.StateText)
	}
	status := policyActionByKind(t, policy, RecoveryActionTruthfulStatusUpdate)
	if status.State != RecoveryActionStateAllowed || !strings.Contains(status.StateText, "дедлайн назначения кейса") {
		t.Fatalf("truthful_status_update: state=%s text=%q — ожидался allowed со словом дедлайна", status.State, status.StateText)
	}
	if strings.Contains(status.StateText, RecoveryActionAG4NotWiredText) {
		t.Fatalf("truthful_status_update: AG-4 предложение не должно ехать при подключённой сверке")
	}
	evidence := policyActionByKind(t, policy, RecoveryActionRequestEvidence)
	if evidence.State != RecoveryActionStateAllowed || !strings.Contains(evidence.StateText, "отдельного явного подтверждения") {
		t.Fatalf("request_evidence: state=%s text=%q — ожидался allowed со словом подтверждения", evidence.State, evidence.StateText)
	}
	pause := policyActionByKind(t, policy, RecoveryActionPreparePauseCancel)
	if pause.State != RecoveryActionStateAllowed || !strings.Contains(pause.StateText, "CancelCompensationSem") {
		t.Fatalf("prepare_pause_cancel: state=%s text=%q — ожидался allowed со словом LIVE-семантики", pause.State, pause.StateText)
	}
	options := policyActionByKind(t, policy, RecoveryActionOfferApprovedOptions)
	if options.State != RecoveryActionStateHumanOnly || !strings.Contains(options.StateText, "вариантов решения в конфигурации нет") {
		t.Fatalf("offer_approved_options: state=%s text=%q — ожидался human_only со словом отсутствующих вариантов", options.State, options.StateText)
	}
	// The human-only class rides SEPARATELY (AC-1): two words with the
	// boundary texts and the class-level word.
	if len(policy.HumanOnly) != 2 {
		t.Fatalf("human-only класс обязан нести ровно 2 слова, несёт %d", len(policy.HumanOnly))
	}
	if policy.HumanOnly[0].Kind != RecoveryHumanOnlyLiabilityOrFault || policy.HumanOnly[1].Kind != RecoveryHumanOnlyNonstandardOutcome {
		t.Fatalf("human-only класс: %v, ожидались [liability_or_fault nonstandard_outcome]", policy.HumanOnly)
	}
	if policy.HumanOnlyClassText != RecoveryHumanOnlyClassText {
		t.Fatalf("класс-текст human-only не совпадает с константой")
	}
}

// TestRecoveryPolicyLadderPausedControl pins the paused scoped control
// (the I/O matrix row 4): материальные виды {request_evidence,
// prepare_pause_cancel} blocked ТОЧНЫМ словом scoped control (режим с
// причиной — прецедент reconcilePausedWord 3-7); read-only
// {show_recorded_conditions, truthful_status_update} остаются allowed;
// offer_approved_options остаётся human_only (решение человека не ждёт
// разблокировки автоматики).
func TestRecoveryPolicyLadderPausedControl(t *testing.T) {
	facts := policyFactsFixture()
	facts.ScopedControlMode = ScopedControlPaused
	facts.ScopedControlReason = "connector paused оператором"
	policy, err := DeriveRecoveryActionPolicy(facts)
	if err != nil {
		t.Fatalf("вывод политики отказал: %v", err)
	}
	for _, kind := range []string{RecoveryActionRequestEvidence, RecoveryActionPreparePauseCancel} {
		action := policyActionByKind(t, policy, kind)
		if action.State != RecoveryActionStateBlocked {
			t.Fatalf("paused: вид «%s» state=%s, ожидался blocked", kind, action.State)
		}
		if !strings.Contains(action.StateText, "paused: connector paused оператором") {
			t.Fatalf("paused: вид «%s» обязан нести ТОЧНОЕ слово scoped control (режим+причина), text=%q", kind, action.StateText)
		}
		if !strings.Contains(action.StateText, "чтение остаётся") {
			t.Fatalf("paused: вид «%s» обязан нести слово «чтение остаётся» (AC-10 эпика 3.7 наследуется)", kind)
		}
	}
	for _, kind := range []string{RecoveryActionShowRecordedConditions, RecoveryActionTruthfulStatusUpdate} {
		action := policyActionByKind(t, policy, kind)
		if action.State != RecoveryActionStateAllowed {
			t.Fatalf("paused: read-only вид «%s» state=%s, ожидался allowed (чтение остаётся)", kind, action.State)
		}
	}
	options := policyActionByKind(t, policy, RecoveryActionOfferApprovedOptions)
	if options.State != RecoveryActionStateHumanOnly {
		t.Fatalf("paused: offer_approved_options state=%s, ожидался human_only (решение человека не ждёт автоматики)", options.State)
	}
}

// TestRecoveryPolicyLadderPausedWordOnly pins the EXACT word rule: без
// причины честное слово = режим один (прецедент reconcilePausedWord —
// пустая причина говорит слово режима без двоеточия).
func TestRecoveryPolicyLadderPausedWordOnly(t *testing.T) {
	facts := policyFactsFixture()
	facts.ScopedControlMode = ScopedControlPaused
	facts.ScopedControlReason = ""
	policy, err := DeriveRecoveryActionPolicy(facts)
	if err != nil {
		t.Fatalf("вывод политики отказал: %v", err)
	}
	action := policyActionByKind(t, policy, RecoveryActionRequestEvidence)
	if action.State != RecoveryActionStateBlocked || !strings.Contains(action.StateText, "scoped control: paused.") {
		t.Fatalf("paused без причины: ожидалось точное слово «paused.» без хвоста, text=%q", action.StateText)
	}
}

// TestRecoveryPolicyLadderControlUnreadable pins the UNREADABLE scoped
// control ("" — nil порт либо ошибка чтения): материальные виды blocked
// честным словом fail-closed («самое строгое применимое»), read-only
// остаются allowed, зависимый от определения вид читает собственное
// правило определения (нечитаемое определение — отдельный случай ниже).
func TestRecoveryPolicyLadderControlUnreadable(t *testing.T) {
	facts := policyFactsFixture()
	facts.ScopedControlMode = ""
	policy, err := DeriveRecoveryActionPolicy(facts)
	if err != nil {
		t.Fatalf("вывод политики отказал: %v", err)
	}
	for _, kind := range []string{RecoveryActionRequestEvidence, RecoveryActionPreparePauseCancel} {
		action := policyActionByKind(t, policy, kind)
		if action.State != RecoveryActionStateBlocked {
			t.Fatalf("нечитаемый контроль: вид «%s» state=%s, ожидался blocked (fail-closed)", kind, action.State)
		}
		if !strings.Contains(action.StateText, "нечитаем") || !strings.Contains(action.StateText, "чтение остаётся доступным") {
			t.Fatalf("нечитаемый контроль: вид «%s» text=%q — ожидалось честное слово fail-closed", kind, action.StateText)
		}
	}
	for _, kind := range []string{RecoveryActionShowRecordedConditions, RecoveryActionTruthfulStatusUpdate} {
		if action := policyActionByKind(t, policy, kind); action.State != RecoveryActionStateAllowed {
			t.Fatalf("нечитаемый контроль: read-only вид «%s» обязан остаться allowed", kind)
		}
	}
}

// TestRecoveryPolicyLadderOtherControlModesPass pins the conservative
// slice: ONLY the paused word blocks the material kinds — accept,
// intake_only/no_promises и human_only проходят фехту контроля политики
// (recovery-scope-разрешения НЕ вводятся — долг 3-7 задокументирован;
// эти слова дисциплинируют материальную подготовку будущих срезов
// исполнения через собственные гейты GuardPreparePolicy).
func TestRecoveryPolicyLadderOtherControlModesPass(t *testing.T) {
	for _, mode := range []string{ScopedControlAccept, ScopedControlIntakeOnly, ScopedControlHumanOnly} {
		facts := policyFactsFixture()
		facts.ScopedControlMode = mode
		policy, err := DeriveRecoveryActionPolicy(facts)
		if err != nil {
			t.Fatalf("режим «%s»: вывод политики отказал: %v", mode, err)
		}
		for _, kind := range []string{RecoveryActionRequestEvidence, RecoveryActionPreparePauseCancel} {
			if action := policyActionByKind(t, policy, kind); action.State != RecoveryActionStateAllowed {
				t.Fatalf("режим «%s»: вид «%s» state=%s — в срезе блокирует только paused", mode, kind, action.State)
			}
		}
	}
}

// TestRecoveryPolicyLadderDefinitionUnreadable pins the definition read
// error (the I/O matrix row 3): зависимый вид prepare_pause_cancel
// human_only («определение нечитаемо — подготовка за человеком»),
// политика 200-вывода НЕ отказывает (право ограничивается),
// request_evidence blocked словом нечитаемого контроля (маршрут не
// именим — fail-closed чтением), read-only остаются allowed.
func TestRecoveryPolicyLadderDefinitionUnreadable(t *testing.T) {
	facts := policyFactsFixture()
	facts.DefinitionReadable = false
	facts.CancelCompensationSem = ""
	facts.ScopedControlMode = "" // маршрут не именим без определения
	policy, err := DeriveRecoveryActionPolicy(facts)
	if err != nil {
		t.Fatalf("вывод политики отказал: %v", err)
	}
	pause := policyActionByKind(t, policy, RecoveryActionPreparePauseCancel)
	if pause.State != RecoveryActionStateHumanOnly || !strings.Contains(pause.StateText, "нечитаемо каталогом") {
		t.Fatalf("нечитаемое определение: prepare_pause_cancel state=%s text=%q — ожидался human_only со словом", pause.State, pause.StateText)
	}
	evidence := policyActionByKind(t, policy, RecoveryActionRequestEvidence)
	if evidence.State != RecoveryActionStateBlocked {
		t.Fatalf("нечитаемое определение: request_evidence state=%s, ожидался blocked (контроль нечитаем — fail-closed)", evidence.State)
	}
	for _, kind := range []string{RecoveryActionShowRecordedConditions, RecoveryActionTruthfulStatusUpdate} {
		if action := policyActionByKind(t, policy, kind); action.State != RecoveryActionStateAllowed {
			t.Fatalf("нечитаемое определение: read-only вид «%s» обязан остаться allowed", kind)
		}
	}
}

// TestRecoveryPolicyLadderSemanticsNotDeclared pins the empty
// CancelCompensationSem semantics (the I/O matrix row 2):
// prepare_pause_cancel human_only словом «pause/cancel семантика не
// объявлена каталогом — подготовка за человеком» — fail-closed AC-1
// хвост («отсутствующее правило означает human_only»).
func TestRecoveryPolicyLadderSemanticsNotDeclared(t *testing.T) {
	facts := policyFactsFixture()
	facts.CancelCompensationSem = ""
	policy, err := DeriveRecoveryActionPolicy(facts)
	if err != nil {
		t.Fatalf("вывод политики отказал: %v", err)
	}
	pause := policyActionByKind(t, policy, RecoveryActionPreparePauseCancel)
	if pause.State != RecoveryActionStateHumanOnly || !strings.Contains(pause.StateText, "семантика не объявлена каталогом") {
		t.Fatalf("пустая семантика: prepare_pause_cancel state=%s text=%q — ожидался human_only со словом", pause.State, pause.StateText)
	}
	// The semantics word with whitespace only reads as undeclared too
	// (the honest trim discipline of the declaredness halves).
	facts.CancelCompensationSem = "   "
	policy, err = DeriveRecoveryActionPolicy(facts)
	if err != nil {
		t.Fatalf("вывод политики отказал: %v", err)
	}
	if pause := policyActionByKind(t, policy, RecoveryActionPreparePauseCancel); pause.State != RecoveryActionStateHumanOnly {
		t.Fatalf("семантика из пробелов: prepare_pause_cancel state=%s, ожидался human_only", pause.State)
	}
}

// TestRecoveryPolicyLadderAG4NotWired pins the AG-4 composition fact
// (the I/O matrix row 6): статусные слова честны — предложение «сверка
// не подключена» едет в слове статуса, политика НЕ блокируется, набор
// выводится.
func TestRecoveryPolicyLadderAG4NotWired(t *testing.T) {
	facts := policyFactsFixture()
	facts.ReconciliationWired = false
	policy, err := DeriveRecoveryActionPolicy(facts)
	if err != nil {
		t.Fatalf("вывод политики отказал: %v", err)
	}
	status := policyActionByKind(t, policy, RecoveryActionTruthfulStatusUpdate)
	if status.State != RecoveryActionStateAllowed {
		t.Fatalf("AG-4: truthful_status_update state=%s — политика не блокируется фактом композиции", status.State)
	}
	if !strings.Contains(status.StateText, RecoveryActionAG4NotWiredText) {
		t.Fatalf("AG-4: слово «сверка не подключена» обязано ехать в статусном слове, text=%q", status.StateText)
	}
	// The AG-4 sentence rides ONLY the status words (the other kinds
	// carry their own honest words).
	evidence := policyActionByKind(t, policy, RecoveryActionRequestEvidence)
	if strings.Contains(evidence.StateText, RecoveryActionAG4NotWiredText) {
		t.Fatalf("AG-4: предложение не должно ехать в слове request_evidence")
	}
}

// TestRecoveryPolicyLadderFailClosedForeignWords pins the fail-closed
// RuleError of the FOREIGN fact words: состояние кейса вне словаря,
// слово scoped control вне словаря — честный отказ с именем словаря,
// никогда замаскированный вердикт (прецедент DeriveRecoveryEligibility).
func TestRecoveryPolicyLadderFailClosedForeignWords(t *testing.T) {
	facts := policyFactsFixture()
	facts.CaseStateWord = "closed"
	if _, err := DeriveRecoveryActionPolicy(facts); err == nil || !strings.Contains(err.Error(), "вне словаря") {
		t.Fatalf("чужое состояние кейса: ожидался fail-closed RuleError с именем словаря, err=%v", err)
	}
	facts = policyFactsFixture()
	facts.ScopedControlMode = "stopped"
	if _, err := DeriveRecoveryActionPolicy(facts); err == nil || !strings.Contains(err.Error(), "вне словаря") {
		t.Fatalf("чужое слово scoped control: ожидался fail-closed RuleError с именем словаря, err=%v", err)
	}
}

// TestRecoveryPolicyDerivationIdempotent pins the byte-stable
// idempotence of the derivation: повторный вызов над теми же фактами
// отвечает ТОТ ЖЕ вывод (политика идемпотентна выводом — состояние вида
// не хранится, каждый запрос выводит заново).
func TestRecoveryPolicyDerivationIdempotent(t *testing.T) {
	facts := policyFactsFixture()
	facts.ScopedControlMode = ScopedControlPaused
	facts.ScopedControlReason = "tenant paused"
	first, err := DeriveRecoveryActionPolicy(facts)
	if err != nil {
		t.Fatalf("первый вывод отказал: %v", err)
	}
	second, err := DeriveRecoveryActionPolicy(facts)
	if err != nil {
		t.Fatalf("второй вывод отказал: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("повторный вывод политики не байт-стабилен — идемпотентность выводом нарушена")
	}
}

// TestRecoveryPolicyEveryAnswerCarriesAttributes pins AC-2 on EVERY
// answer: атрибуты вида полны в каждом ответе (scope/eligibility/
// required_facts/authority/confirmation_required + state + honest-слово)
// — машинный вид не выводит усечённых ответов ни на одном пути.
func TestRecoveryPolicyEveryAnswerCarriesAttributes(t *testing.T) {
	states := []RecoveryPolicyFacts{
		policyFactsFixture(),
		{CaseStateWord: RecoveryCaseStateOpen, ScopedControlMode: ScopedControlPaused, ScopedControlReason: "r", DefinitionReadable: true, CancelCompensationSem: "sem", ReconciliationWired: false},
		{CaseStateWord: RecoveryCaseStateOpen, DefinitionReadable: false, ReconciliationWired: false},
		{CaseStateWord: RecoveryCaseStateOpen, ScopedControlMode: ScopedControlAccept, DefinitionReadable: true, ReconciliationWired: true},
	}
	for i, facts := range states {
		policy, err := DeriveRecoveryActionPolicy(facts)
		if err != nil {
			t.Fatalf("матрица[%d]: вывод отказал: %v", i, err)
		}
		for _, action := range policy.Actions {
			decl := RecoveryActionKindByWord(action.Kind)
			if decl == nil {
				t.Fatalf("матрица[%d]: ответ несёт вне-словарный вид «%s»", i, action.Kind)
			}
			if action.ScopeText != decl.ScopeText || action.EligibilityRuleText != decl.EligibilityRuleText ||
				action.RequiredFactsText != decl.RequiredFactsText || action.AuthorityText != decl.AuthorityText ||
				action.ConfirmationRequired != decl.ConfirmationRequired {
				t.Fatalf("матрица[%d]: вид «%s» — атрибуты ответа разошлись с декларацией", i, action.Kind)
			}
			if !WordIn(RecoveryActionStateWords, action.State) {
				t.Fatalf("матрица[%d]: вид «%s» — состояние «%s» вне словаря", i, action.Kind, action.State)
			}
			if strings.TrimSpace(action.StateText) == "" {
				t.Fatalf("матрица[%d]: вид «%s» — honest-слово состояния пусто", i, action.Kind)
			}
			if len([]rune(action.StateText)) > RecoveryPolicyTextRunes {
				t.Fatalf("матрица[%d]: вид «%s» — honest-слово длиннее буйнда", i, action.Kind)
			}
		}
	}
}

// TestRecoveryPolicyHonestTextsBounded pins the honest texts of the
// policy surfaces: каждое compile-time слово в буйнде 500 рун —
// поверхности остаются рендеряемыми (буйнд — структурный, не рантайм).
func TestRecoveryPolicyHonestTextsBounded(t *testing.T) {
	texts := []string{
		RecoveryActionPausedTextTemplate, RecoveryActionControlUnreadableText,
		RecoveryActionDefinitionUnreadableText, RecoveryActionSemanticsNotDeclaredText,
		RecoveryActionNoApprovedOptionsText, RecoveryActionRequestEvidenceAllowedText,
		RecoveryActionShowConditionsAllowedText, RecoveryActionStatusUpdateAllowedText,
		RecoveryActionAG4NotWiredText, RecoveryActionPauseCancelAllowedText,
		RecoveryHumanOnlyClassText,
	}
	for _, text := range texts {
		if strings.TrimSpace(text) == "" {
			t.Fatalf("пустое honest-слово политики: %+v", text)
		}
		if len([]rune(text)) > RecoveryPolicyTextRunes {
			t.Fatalf("honest-слово политики длиннее буйнда %d рун: %.80s…", RecoveryPolicyTextRunes, text)
		}
	}
	// The status word with the AG-4 sentence stays within the bound too.
	if len([]rune(RecoveryActionStatusUpdateAllowedText+" "+RecoveryActionAG4NotWiredText)) > RecoveryPolicyTextRunes {
		t.Fatalf("статусное слово с AG-4 предложением длиннее буйнда")
	}
}
