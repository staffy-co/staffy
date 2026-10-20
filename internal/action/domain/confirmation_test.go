package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// The 3.5 step-01 unit matrix of the Confirmation core — every state
// word BY NAME, every rejection BY NAME, the allowance derivation of all
// three rule words, the valid_until server truth with the clamp, the
// fail-closed guards of the three commands and the closed dictionaries.
// The fixture definition is the shared 3-4 unit fixture (the same package
// — reuse over duplication); the variants flip the two definition words
// this contour reads.

// confirmationNotRequiredDefinition is the honest boundary variant: a
// query definition whose answers live in the observations of 2-7 — the
// contour refuses it with the exact word.
func confirmationNotRequiredDefinition() ActionDefinition {
	d := previewFixtureDefinition()
	d.ID = "query.trunk-status"
	d.Conditions.Confirmation = ConfirmationNotRequired
	d.Conditions.ConsumptionRule = ConsumptionUnlimited
	return d
}

// confirmationBoundedDefinition is the reserved-word variant: bounded
// carries no numeric bound carrier in the 3-1 shape — the allowance is
// underivable and everything fails closed.
func confirmationBoundedDefinition() ActionDefinition {
	d := previewFixtureDefinition()
	d.Conditions.ConsumptionRule = ConsumptionBounded
	return d
}

// confirmationEventFixture appends events of the given kinds at one fixed
// moment by one fixed actor — the history shape the derivations read.
func confirmationEventFixture(kinds ...string) []ConfirmationEvent {
	at := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	actor := id.MustParse("018f0000-0000-7000-8000-0000000000aa")
	events := make([]ConfirmationEvent, 0, len(kinds))
	for _, kind := range kinds {
		events = append(events, ConfirmationEvent{Kind: kind, At: at, ActorID: actor})
	}
	return events
}

// ruleErrorOf unwraps the RuleError of a fail-closed guard — the matrix
// asserts the exact rule and the honest reason word, never a bare error.
func ruleErrorOf(t *testing.T, err error) RuleError {
	t.Helper()
	var ruleErr RuleError
	if !errors.As(err, &ruleErr) {
		t.Fatalf("ожидалась RuleError, получено: %v", err)
	}
	return ruleErr
}

func TestConfirmationStateDictionaryClosed(t *testing.T) {
	// The five-word dictionary, byte-exact, in the derivation order.
	want := []string{"valid", "superseded", "revoked", "expired", "exhausted"}
	if len(ConfirmationStateWords) != len(want) {
		t.Fatalf("словарь состояний Confirmation: %d слов, ожидалось %d", len(ConfirmationStateWords), len(want))
	}
	for i, word := range want {
		if ConfirmationStateWords[i] != word {
			t.Fatalf("слово %d = %q, ожидалось %q", i, ConfirmationStateWords[i], word)
		}
	}
	// Every word of the dictionary has a fixed honest text.
	for _, word := range ConfirmationStateWords {
		if ConfirmationStateText(word) == "" {
			t.Fatalf("слово %q без честного текста", word)
		}
	}
	// A foreign word fails closed with the dictionary itself — the
	// out-of-dictionary state word is impossible to speak.
	foreign := ConfirmationStateText("awaiting")
	if !strings.Contains(foreign, "не входит в словарь") || !strings.Contains(foreign, "valid") {
		t.Fatalf("вне-словарное слово должно отказывать словарём, получено: %q", foreign)
	}
}

func TestConfirmationEventKindDictionaryClosed(t *testing.T) {
	// The six-word append-only history vocabulary, byte-exact.
	want := []string{
		"confirmed", "superseded", "revoked", "consumed",
		"authorization_conflict", "authorization_blocked",
	}
	if len(ConfirmationEventKindWords) != len(want) {
		t.Fatalf("словарь событий Confirmation: %d слов, ожидалось %d", len(ConfirmationEventKindWords), len(want))
	}
	for i, word := range want {
		if ConfirmationEventKindWords[i] != word {
			t.Fatalf("слово %d = %q, ожидалось %q", i, ConfirmationEventKindWords[i], word)
		}
		if err := GuardConfirmationEventKind(word); err != nil {
			t.Fatalf("словарное событие %q обязано проходить гейт: %v", word, err)
		}
	}
	// A foreign kind is refused — UPDATE and DELETE never dress up as
	// history kinds (the append-only honesty).
	for _, foreign := range []string{"updated", "deleted", "", "executed", "confirmed "} {
		err := GuardConfirmationEventKind(foreign)
		if err == nil {
			t.Fatalf("вне-словарное событие %q обязано отказывать", foreign)
		}
		ruleErr := ruleErrorOf(t, err)
		if !strings.Contains(ruleErr.Reason, "вне словаря") {
			t.Fatalf("отказ события %q обязан говорить словарь: %q", foreign, ruleErr.Reason)
		}
	}
}

func TestDeriveConfirmationStateEachWordByName(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 30, 0, 0, time.UTC)
	live := now.Add(30 * time.Minute)

	cases := []struct {
		name string
		rule string
		want string
		evts []ConfirmationEvent
	}{
		{"valid — согласие живо, ничего не сработало", ConsumptionSingleUse, ConfirmationValid,
			confirmationEventFixture(ConfirmationEventConfirmed)},
		{"superseded — есть событие замены версии", ConsumptionSingleUse, ConfirmationSuperseded,
			confirmationEventFixture(ConfirmationEventConfirmed, ConfirmationEventSuperseded)},
		{"revoked — есть событие отзыва человеком", ConsumptionSingleUse, ConfirmationRevoked,
			confirmationEventFixture(ConfirmationEventConfirmed, ConfirmationEventRevoked)},
		{"exhausted — single_use потреблена одной авторизацией", ConsumptionSingleUse, ConfirmationExhausted,
			confirmationEventFixture(ConfirmationEventConfirmed, ConfirmationEventConsumed)},
	}
	for _, tc := range cases {
		got, err := DeriveConfirmationState(tc.rule, live, tc.evts, now)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: получено %q, ожидалось %q", tc.name, got, tc.want)
		}
	}

	// expired — серверные часы прошли valid_until (живые часы, не статус).
	got, err := DeriveConfirmationState(ConsumptionSingleUse, now.Add(-time.Second),
		confirmationEventFixture(ConfirmationEventConfirmed), now)
	if err != nil {
		t.Fatal(err)
	}
	if got != ConfirmationExpired {
		t.Fatalf("expired: получено %q", got)
	}
}

func TestDeriveConfirmationStateOrderIsNormative(t *testing.T) {
	// The FIXED order is part of the contract: superseded beats revoked
	// beats expired beats exhausted — the first matching word answers.
	now := time.Date(2026, 10, 20, 12, 30, 0, 0, time.UTC)
	dead := now.Add(-time.Second) // valid_until прошла — expired условие истинно

	// superseded бьёт revoked + expired + exhausted одновременно.
	got, err := DeriveConfirmationState(ConsumptionSingleUse, dead,
		confirmationEventFixture(ConfirmationEventConfirmed, ConfirmationEventRevoked,
			ConfirmationEventSuperseded, ConfirmationEventConsumed), now)
	if err != nil {
		t.Fatal(err)
	}
	if got != ConfirmationSuperseded {
		t.Fatalf("порядок: superseded обязан бить всё, получено %q", got)
	}

	// revoked бьёт expired + exhausted (superseded нет).
	got, err = DeriveConfirmationState(ConsumptionSingleUse, dead,
		confirmationEventFixture(ConfirmationEventConfirmed, ConfirmationEventConsumed,
			ConfirmationEventRevoked), now)
	if err != nil {
		t.Fatal(err)
	}
	if got != ConfirmationRevoked {
		t.Fatalf("порядок: revoked обязан бить expired/exhausted, получено %q", got)
	}

	// expired бьёт exhausted (superseded/revoked нет).
	got, err = DeriveConfirmationState(ConsumptionSingleUse, dead,
		confirmationEventFixture(ConfirmationEventConfirmed, ConfirmationEventConsumed), now)
	if err != nil {
		t.Fatal(err)
	}
	if got != ConfirmationExpired {
		t.Fatalf("порядок: expired обязан бить exhausted, получено %q", got)
	}
}

func TestDeriveConfirmationStateUnlimitedNeverExhausted(t *testing.T) {
	// unlimited: исчерпание невозможно по построению — сколько бы
	// consumed событий ни насчитала история, слово остаётся valid.
	now := time.Date(2026, 10, 20, 12, 30, 0, 0, time.UTC)
	events := confirmationEventFixture(
		ConfirmationEventConfirmed, ConfirmationEventConsumed, ConfirmationEventConsumed,
		ConfirmationEventConsumed, ConfirmationEventConsumed, ConfirmationEventConsumed)
	got, err := DeriveConfirmationState(ConsumptionUnlimited, now.Add(time.Hour), events, now)
	if err != nil {
		t.Fatal(err)
	}
	if got != ConfirmationValid {
		t.Fatalf("unlimited: получено %q, ожидалось %q — exhausted невозможен", got, ConfirmationValid)
	}
}

func TestDeriveConfirmationStateBoundedFailsClosed(t *testing.T) {
	// bounded: числового носителя границы в определениях 3-1 нет — деривация
	// состояния сама отказывает честным словом, слово exhausted не выдумывается.
	now := time.Date(2026, 10, 20, 12, 30, 0, 0, time.UTC)
	_, err := DeriveConfirmationState(ConsumptionBounded, now.Add(time.Hour),
		confirmationEventFixture(ConfirmationEventConfirmed), now)
	if err == nil {
		t.Fatal("bounded: деривация обязана отказывать fail-closed")
	}
	ruleErr := ruleErrorOf(t, err)
	if ruleErr.Reason != ConfirmationBoundedAllowanceText {
		t.Fatalf("bounded: отказ обязан говорить точный текст границы, получено: %q", ruleErr.Reason)
	}
}

func TestDeriveConsumedCount(t *testing.T) {
	// Потребление — это счёт событий consumed read-time, не колонка:
	// только consumed считается, остальные виды истории не потребление.
	events := confirmationEventFixture(
		ConfirmationEventConfirmed, ConfirmationEventConsumed, ConfirmationEventRevoked,
		ConfirmationEventConsumed, ConfirmationEventAuthorizationBlocked)
	if got := DeriveConsumedCount(events); got != 2 {
		t.Fatalf("consumed-счёт: получено %d, ожидалось 2", got)
	}
	if got := DeriveConsumedCount(nil); got != 0 {
		t.Fatalf("пустая история: получено %d, ожидалось 0", got)
	}
	if got := DeriveConsumedCount(confirmationEventFixture(ConfirmationEventConfirmed)); got != 0 {
		t.Fatalf("подтверждение без авторизаций не потреблено, получено %d", got)
	}
}

func TestDeriveConsumptionAllowance(t *testing.T) {
	// single_use → ровно 1; unlimited → без границы (exhausted невозможен);
	// bounded и любое стороннее слово → fail-closed точным текстом.
	allowance, err := DeriveConsumptionAllowance(ConsumptionSingleUse)
	if err != nil || allowance.Bounded != true || allowance.Limit != 1 {
		t.Fatalf("single_use: %+v, %v — ожидалось {Bounded:true Limit:1}", allowance, err)
	}
	allowance, err = DeriveConsumptionAllowance(ConsumptionUnlimited)
	if err != nil || allowance.Bounded != false {
		t.Fatalf("unlimited: %+v, %v — ожидалось {Bounded:false}", allowance, err)
	}
	for _, rule := range []string{ConsumptionBounded, "sometimes", "", "single"} {
		_, err := DeriveConsumptionAllowance(rule)
		if err == nil {
			t.Fatalf("правило %q обязано отказывать fail-closed", rule)
		}
		ruleErr := ruleErrorOf(t, err)
		if ruleErr.Reason != ConfirmationBoundedAllowanceText {
			t.Fatalf("правило %q: отказ обязан говорить точный текст границы, получено %q", rule, ruleErr.Reason)
		}
	}
}

func TestDeriveValidUntilServerTruth(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	// Кламп снизу: 10 секунд запроса → 60 секунд окна.
	if got := DeriveValidUntil(now, 10); !got.Equal(now.Add(60 * time.Second)) {
		t.Fatalf("кламп снизу: %v, ожидалось +60s", got)
	}
	// Кламп сверху: 99999 секунд запроса → 3600 секунд окна.
	if got := DeriveValidUntil(now, 99999); !got.Equal(now.Add(3600 * time.Second)) {
		t.Fatalf("кламп сверху: %v, ожидалось +3600s", got)
	}
	// В границах окно честное: 600 → 600.
	if got := DeriveValidUntil(now, 600); !got.Equal(now.Add(600 * time.Second)) {
		t.Fatalf("окно в границах: %v, ожидалось +600s", got)
	}
	// Границы включительно: 60 и 3600 проходят без изменения.
	if got := DeriveValidUntil(now, ConfirmationLifetimeMinSeconds); !got.Equal(now.Add(60 * time.Second)) {
		t.Fatalf("нижняя граница: %v", got)
	}
	if got := DeriveValidUntil(now, ConfirmationLifetimeMaxSeconds); !got.Equal(now.Add(3600 * time.Second)) {
		t.Fatalf("верхняя граница: %v", got)
	}
	// Детерминизм: одно и то же now + lifetime → одно и то же valid_until.
	if DeriveValidUntil(now, 600) != DeriveValidUntil(now, 600) {
		t.Fatal("valid_until недетерминирован для одинаковых входов")
	}
}

func TestGuardConfirmDefinitionGates(t *testing.T) {
	// required + потребляемое правило — гейт зелёный.
	if err := GuardConfirm(previewFixtureDefinition()); err != nil {
		t.Fatalf("required+single_use обязан проходить: %v", err)
	}
	unlimitedDef := previewFixtureDefinition()
	unlimitedDef.Conditions.ConsumptionRule = ConsumptionUnlimited
	if err := GuardConfirm(unlimitedDef); err != nil {
		t.Fatalf("required+unlimited обязан проходить: %v", err)
	}

	// not_required — честный отказ точным словом: запросы идут через
	// наблюдения 2-7, контур существует для мутирующих действий.
	err := GuardConfirm(confirmationNotRequiredDefinition())
	if err == nil {
		t.Fatal("not_required обязан отказывать")
	}
	ruleErr := ruleErrorOf(t, err)
	if ruleErr.Rule != "GuardConfirm" || ruleErr.Reason != ConfirmationNotRequiredText {
		t.Fatalf("not_required: отказ обязан говорить точный текст границы, получено %+v", ruleErr)
	}

	// bounded — fail-closed: подтверждение, которое никогда не потребится,
	// не создаётся.
	err = GuardConfirm(confirmationBoundedDefinition())
	if err == nil {
		t.Fatal("bounded обязан отказывать")
	}
	ruleErr = ruleErrorOf(t, err)
	if ruleErr.Reason != ConfirmationBoundedAllowanceText {
		t.Fatalf("bounded: отказ обязан говорить точный текст границы, получено %q", ruleErr.Reason)
	}
}

func TestGuardRevokeDefinitionGate(t *testing.T) {
	// Отзывается только подтверждение мутирующего действия.
	if err := GuardRevoke(previewFixtureDefinition()); err != nil {
		t.Fatalf("required обязан проходить: %v", err)
	}
	err := GuardRevoke(confirmationNotRequiredDefinition())
	if err == nil {
		t.Fatal("not_required обязан отказывать — отзывать нечего")
	}
	ruleErr := ruleErrorOf(t, err)
	if ruleErr.Rule != "GuardRevoke" || ruleErr.Reason != ConfirmationNotRequiredText {
		t.Fatalf("отказ ревокации обязан говорить честную границу, получено %+v", ruleErr)
	}
}

func TestGuardAuthorizeKeyGate(t *testing.T) {
	// Валидные машинные слова повтора: charset [A-Za-z0-9._-], 8..128 рун.
	validKeys := []string{
		"aB3.xY_9",                    // ровно 8 рун — нижняя граница
		"order-confirm-2026.10.20_v1", // живой пример
		strings.Repeat("k", 128),      // ровно 128 рун — верхняя граница
		"A1-b2_C3.d4",                 // все группы charset
	}
	for _, key := range validKeys {
		if err := GuardAuthorize(key); err != nil {
			t.Fatalf("ключ %q обязан проходить: %v", key, err)
		}
	}

	// Длина вне границы — честная причина ДО всего остального.
	short := strings.Repeat("k", 7)
	long := strings.Repeat("k", 129)
	for _, key := range []string{short, long, ""} {
		err := GuardAuthorize(key)
		if err == nil {
			t.Fatalf("ключ %q обязан отказывать по длине", key)
		}
		ruleErr := ruleErrorOf(t, err)
		if ruleErr.Rule != "GuardAuthorize" || !strings.Contains(ruleErr.Reason, "от 8 до 128 рун") {
			t.Fatalf("отказ длины обязан говорить границы, получено %+v", ruleErr)
		}
	}

	// Вне-словарный байт — честная причина с charset в тексте: пробел,
	// кириллица, слеш, двоеточие — машинная поверхность не расширяется.
	for _, key := range []string{"ключ повтора 1", "ab/12345", "ab:12345", "повтор123", "ab 12345"} {
		err := GuardAuthorize(key)
		if err == nil {
			t.Fatalf("ключ %q обязан отказывать по charset", key)
		}
		ruleErr := ruleErrorOf(t, err)
		if !strings.Contains(ruleErr.Reason, "charset") {
			t.Fatalf("отказ charset обязан называть словарь, получено %q", ruleErr.Reason)
		}
	}
}

func TestOwnerKindMirror(t *testing.T) {
	// Зеркало словаря владельцев 2-4: два слова, byte-exact — тройка
	// (kind/ref/epoch) пинит владельца, которым кейс владел в момент
	// подтверждения.
	if len(OwnerKindWords) != 2 || OwnerKindWords[0] != OwnerHuman || OwnerKindWords[1] != OwnerStaffy {
		t.Fatalf("зеркало словаря владельцев: %v, ожидалось [human staffy]", OwnerKindWords)
	}
	if !WordIn(OwnerKindWords, OwnerHuman) || WordIn(OwnerKindWords, "queue") {
		t.Fatal("зеркало владельцев обязано быть закрытым: human/staffy, никаких очередей")
	}
}

func TestConfirmationHonestTextsPinned(t *testing.T) {
	// Лестница отвержений говорит каждое слово поимённо — точные фразы
	// среза, никаких маскирующих умолчаний.
	ladder := map[string]string{
		ConfirmationSuperseded: "заменена новой версией",
		ConfirmationRevoked:    "отозвана",
		ConfirmationExpired:    "valid_until",
		ConfirmationExhausted:  "исчерпана",
		ConfirmationValid:      "действительна",
	}
	for word, needle := range ladder {
		text := ConfirmationStateText(word)
		if !strings.Contains(text, needle) {
			t.Fatalf("текст состояния %q обязан содержать «%s»: %q", word, needle, text)
		}
	}
	// Фиксированные границы среза — точные фразы пиннуты.
	for text, needle := range map[string]string{
		ConfirmationNotRequiredText:      "наблюдения 2-7",
		ConfirmationBoundedAllowanceText: "зарезервировано",
		ConfirmationPreviewChangedText:   "новая версия Preview и Confirmation",
		MaterialConditionChangedText:     "Материальное условие изменилось",
		FreshReadsUnavailableText:        "fail-closed",
		IdempotencyConflictText:          "исходная Operation неизменна",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("фиксированный текст обязан содержать «%s»: %q", needle, text)
		}
	}
}
