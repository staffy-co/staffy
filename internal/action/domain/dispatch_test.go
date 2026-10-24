package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// The 3.6 step-01 unit matrix of the dispatch/receipt domain — the
// dictionaries by name (with the exhausted-absence probe of the fence
// dictionary), the last-state-bearing-word derivation of the operation
// ledger, the fail-closed guards of the adapter answer (the dictionary
// gate and the committed-evidence refusal — the terminal success is never
// taken on word), the outcome→state/receipt mappings, the confirmation
// dispatch fence WITHOUT exhausted, the retry policy class×guarantee×
// bound matrix, the deterministic rebuild and the keyed-HMAC derivations
// (the external idempotency key stable across attempts, the request
// digest deterministic under input shuffling, the AD-10 probes: no raw
// unsalted hash over material anywhere in the module), the lease
// constants/window, and the honest texts of the operation life.

// dispatchKeyFixture — фиксированный ключ единицы теста (прод-ключ живёт в
// commitments_hmac_keys per-tenant — ensure-паттерн 2-9/3-3/3-4/3-5,
// забота шага-02).
var dispatchKeyFixture = []byte("test-commitment-key-3-6")

// dispatchOperationFixture — Operation, пинящая версию previewFixture()
// (единая тройка Operation/preview/определение для пересборки payload).
func dispatchOperationFixture() Operation {
	preview := previewFixture()
	return Operation{
		TenantID:              preview.TenantID,
		OperationID:           id.MustParse("01994f64-6c00-7000-8000-0000000000e1"),
		DefinitionID:          preview.DefinitionID,
		DefinitionVersion:     preview.DefinitionVersion,
		PreviewID:             preview.PreviewID,
		PreviewVersionNo:      preview.VersionNo,
		ConfirmationID:        id.MustParse("018f0000-0000-7000-8000-000000000003"),
		ConfirmationVersionNo: 1,
		CaseID:                preview.CaseID,
		WorkItemID:            preview.WorkItemID,
		Target:                preview.Target,
		OwnerKind:             OwnerHuman,
		OwnerRef:              id.MustParse("018f0000-0000-7000-8000-000000000005"),
		OwnerEpoch:            preview.OwnerEpoch,
		ReleaseDecisionNo:     42,
		ProfileVersion:        preview.ProfileVersion,
		Partition:             PartitionStaffy,
		Effects:               preview.Effects,
		BusinessIntentKey:     DeriveBusinessIntentKey(dispatchKeyFixture, intentInputFixture()),
		IdempotencyKey:        "order-confirm-2026.10.21_v1",
		MaterialDigest:        DeriveIdempotencyFingerprint(dispatchKeyFixture, preview.TenantID, operationPinsFixture()),
		CreatedAt:             preview.CreatedAt,
	}
}

// dispatchPayloadFixture — материал с двумя typed_input полями, двумя
// строками snapshot и эффектами (перетасовка имеет что перетасовывать).
func dispatchPayloadFixture() AuthorizedActionPayload {
	op := dispatchOperationFixture()
	return AuthorizedActionPayload{
		TenantID:          op.TenantID,
		OperationID:       op.OperationID,
		DefinitionID:      op.DefinitionID,
		DefinitionVersion: op.DefinitionVersion,
		Target:            op.Target,
		TypedInput:        map[string]string{"client_phone": "+79001112233", "template_ref": "order-ready-v3"},
		Snapshot: []MaterialSnapshotRow{
			{MaterialField: "price", Status: SnapshotFresh, ExternalID: "SKU-77", ExternalVersion: "12",
				ObservedAt: time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC), EvidenceRef: "catalog://sku/77"},
			{MaterialField: "availability", Status: SnapshotFresh, ExternalID: "SKU-77", ExternalVersion: "12",
				ObservedAt: time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC), EvidenceRef: "catalog://sku/77"},
		},
		Effects: DerivePreviewEffects(previewFixtureDefinition()),
	}
}

// ledgerHistory строит историю из слов ledger по порядку (append-only
// чтение — записи идут номерами).
func ledgerHistory(kinds ...string) []OperationLedgerEntry {
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	entries := make([]OperationLedgerEntry, 0, len(kinds))
	for i, kind := range kinds {
		entries = append(entries, OperationLedgerEntry{EntryNo: int64(i + 1), Kind: kind, At: at})
	}
	return entries
}

// ---------------------------------------------------------------------------
// The dictionaries: closure, counts, the exhausted-absence probe
// ---------------------------------------------------------------------------

// TestDispatchDictionariesByName pins every dispatch dictionary BY NAME:
// the exact word lists and the exact counts — a runtime or config
// extension of any of them is impossible while this test is green
// (compile-time words only).
func TestDispatchDictionariesByName(t *testing.T) {
	cases := []struct {
		name       string
		dictionary []string
		want       []string
	}{
		{"исходы-адаптера-4", MutationOutcomeWords,
			[]string{MutationOutcomeAccepted, MutationOutcomeCommitted, MutationOutcomeFailed, MutationOutcomeTimeout}},
		{"receipt-исходы-3", ReceiptOutcomeWords,
			[]string{ReceiptOutcomeCommitted, ReceiptOutcomeFailed, ReceiptOutcomeUnknown}},
		{"фенции-подтверждения-3", DispatchFenceWords,
			[]string{DispatchFenceSuperseded, DispatchFenceRevoked, DispatchFenceExpired}},
		{"слова-политики-повтора-4", DispatchRetryWords,
			[]string{DispatchRetryRetry, DispatchRetryParkUnknown, DispatchRetryParkBound, DispatchRetryDecided}},
		{"события-outbox-3", OutboxEventWords,
			[]string{OutboxEventOperationQueued, OutboxEventReceiptCreated, OutboxEventRecoveryCaseCreated}},
	}
	for _, tc := range cases {
		if !reflect.DeepEqual(tc.dictionary, tc.want) {
			t.Fatalf("словарь %s: %v, ожидалось ровно %v", tc.name, tc.dictionary, tc.want)
		}
	}
	// Слова словарей — точные строки эпика/AC.
	if MutationOutcomeWords[0] != "accepted" || MutationOutcomeWords[1] != "committed" ||
		MutationOutcomeWords[2] != "failed" || MutationOutcomeWords[3] != "timeout" {
		t.Fatal("исходы адаптера обязаны быть словами {accepted, committed, failed, timeout}")
	}
	// ЗОНД задокументированного решения: exhausted НЕ фенция dispatch-гейта
	// (потребление пост-авторизации ожидаемо) — слова в словаре фенций нет.
	if WordIn(DispatchFenceWords, ConfirmationExhausted) {
		t.Fatal("exhausted не должен быть фенцией dispatch-гейта — потребление пост-авторизации ожидаемо")
	}
	// Состояния Operation (6) и слова ledger (7) — словари operation.go.
	wantStates := []string{OperationAuthorized, OperationSubmitted, OperationAccepted, OperationCommitted, OperationFailed, OperationUnknown}
	if !reflect.DeepEqual(OperationStateWords, wantStates) {
		t.Fatalf("словарь состояний: %v, ожидалось 6 слов %v", OperationStateWords, wantStates)
	}
	wantLedger := []string{LedgerEntryAuthorized, LedgerEntrySubmitted, LedgerEntryAccepted,
		LedgerEntryCommitted, LedgerEntryFailed, LedgerEntryUnknown, LedgerEntryDispatchBlocked,
		LedgerEntryReconciling}
	if !reflect.DeepEqual(LedgerEntryKindWords, wantLedger) {
		t.Fatalf("словарь ledger: %v, ожидалось 8 слов %v", LedgerEntryKindWords, wantLedger)
	}
}

// ---------------------------------------------------------------------------
// The state derivation: the last state-bearing word (operation.go)
// ---------------------------------------------------------------------------

// TestDeriveOperationStateLastStateBearingWord pins the read-time state
// derivation: the LAST state-bearing ledger word wins, dispatch_blocked is
// skipped (a visible parking cause, never a state), the empty ledger and
// the foreign words refuse fail-closed.
func TestDeriveOperationStateLastStateBearingWord(t *testing.T) {
	cases := []struct {
		name    string
		history []string
		want    string
	}{
		{"резервация", []string{LedgerEntryAuthorized}, OperationAuthorized},
		{"write-ahead-перед-вызовом", []string{LedgerEntryAuthorized, LedgerEntrySubmitted}, OperationSubmitted},
		{"приём-провайдером", []string{LedgerEntryAuthorized, LedgerEntrySubmitted, LedgerEntryAccepted}, OperationAccepted},
		{"терминальный-успех", []string{LedgerEntryAuthorized, LedgerEntrySubmitted, LedgerEntryAccepted, LedgerEntryCommitted}, OperationCommitted},
		{"терминальный-отказ", []string{LedgerEntryAuthorized, LedgerEntrySubmitted, LedgerEntryFailed}, OperationFailed},
		{"неопределённость", []string{LedgerEntryAuthorized, LedgerEntrySubmitted, LedgerEntryUnknown}, OperationUnknown},
		{"timeout-мапится-в-unknown-аппом", []string{LedgerEntryAuthorized, LedgerEntrySubmitted, LedgerEntryUnknown}, OperationUnknown},
	}
	for _, tc := range cases {
		got, err := DeriveOperationState(ledgerHistory(tc.history...))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: состояние %q, ожидалось %q", tc.name, got, tc.want)
		}
	}

	// dispatch_blocked НЕ state-bearing: припарковка до/после любого
	// состояния не меняет его (операция остаётся тем, чем была).
	if got, _ := DeriveOperationState(ledgerHistory(LedgerEntryAuthorized, LedgerEntryDispatchBlocked)); got != OperationAuthorized {
		t.Fatalf("припарковка authorized: %q, ожидалось authorized", got)
	}
	if got, _ := DeriveOperationState(ledgerHistory(LedgerEntryAuthorized, LedgerEntrySubmitted, LedgerEntryDispatchBlocked)); got != OperationSubmitted {
		t.Fatalf("припарковка submitted: %q, ожидалось submitted", got)
	}
	// Повторная припарковка и возврат authorized тем же механизмом.
	if got, _ := DeriveOperationState(ledgerHistory(LedgerEntryAuthorized, LedgerEntryDispatchBlocked, LedgerEntryAuthorized)); got != OperationAuthorized {
		t.Fatalf("история с возвратом: %q, ожидалось authorized", got)
	}
	// reconciling (3.7) НЕ state-bearing: активность сверки не меняет
	// состояние — операция ОСТАЁТСЯ unknown (слэш unknown/reconciling
	// эпика: пара {состояние unknown, активность reconciling}).
	if got, _ := DeriveOperationState(ledgerHistory(LedgerEntryAuthorized, LedgerEntrySubmitted, LedgerEntryUnknown, LedgerEntryReconciling)); got != OperationUnknown {
		t.Fatalf("сверка поверх unknown: %q, ожидалось unknown (reconciling — активность, не состояние)", got)
	}
	if got, _ := DeriveOperationState(ledgerHistory(LedgerEntryAuthorized, LedgerEntryReconciling)); got != OperationAuthorized {
		t.Fatalf("сверка поверх authorized: %q, ожидалось authorized", got)
	}
	// Пустой ledger отказывает.
	if _, err := DeriveOperationState(nil); err == nil {
		t.Fatal("пустой ledger обязан отказывать fail-closed")
	}
	// Ledger без единого state-bearing слова (только припарковки) —
	// доказательство авторизации отсутствует: порча данных, отказ честным
	// словом, не пустое состояние.
	_, err := DeriveOperationState(ledgerHistory(LedgerEntryDispatchBlocked))
	if err == nil {
		t.Fatal("ledger без state-bearing слов обязан отказывать fail-closed")
	}
	if reason := ruleErrorOf(t, err).Reason; !strings.Contains(reason, "authorized") {
		t.Fatalf("отказ обязан называть отсутствующее доказательство: %q", reason)
	}
	// Вне-словарное слово отказывает поимённо (словарь называется в отказе).
	_, err = DeriveOperationState(ledgerHistory(LedgerEntryAuthorized, "executed"))
	if err == nil {
		t.Fatal("слово executed обязано отказывать — вне словаря ledger")
	}
	if reason := ruleErrorOf(t, err).Reason; !strings.Contains(reason, "вне словаря") {
		t.Fatalf("отказ обязан называть словарь: %q", reason)
	}
}

// ---------------------------------------------------------------------------
// The guards of the adapter answer
// ---------------------------------------------------------------------------

// TestGuardMutationOutcomeDictionary pins the dictionary gate: the four
// words pass, every foreign word refuses by name (the caller reads the
// refusal as unknown WITHOUT a guarantee — never a masked success).
func TestGuardMutationOutcomeDictionary(t *testing.T) {
	for _, word := range MutationOutcomeWords {
		if err := GuardMutationOutcome(word); err != nil {
			t.Fatalf("слово %q обязано проходить словарный гейт: %v", word, err)
		}
	}
	for _, foreign := range []string{"delivered", "executed", "unknown", "", "Accepted"} {
		err := GuardMutationOutcome(foreign)
		if err == nil {
			t.Fatalf("чужое слово %q обязано отказывать", foreign)
		}
		reason := ruleErrorOf(t, err).Reason
		if !strings.Contains(reason, "не входит в словарь") || !strings.Contains(reason, foreign) {
			t.Fatalf("отказ обязан называть слово и словарь: %q", reason)
		}
	}
}

// TestGuardCommittedEvidenceRefusesWordOnlyCommitted pins the
// State/Authority discipline (AC-6/AC-7): committed survives ONLY with the
// complete structural evidence triple; a committed claim without evidence
// is refused to unknown — the terminal success is never taken on word.
func TestGuardCommittedEvidenceRefusesWordOnlyCommitted(t *testing.T) {
	complete := MutationEvidence{
		ExternalObjectID:      "msg-2026-77",
		ExternalObjectVersion: "v4",
		ExternalObjectTime:    time.Date(2026, 10, 21, 12, 1, 0, 0, time.UTC),
	}
	// committed + полный тройной evidence → committed.
	got, err := GuardCommittedEvidence(MutationOutcomeCommitted, complete)
	if err != nil || got != MutationOutcomeCommitted {
		t.Fatalf("committed с полным evidence: %q, %v — ожидался committed", got, err)
	}
	// committed без КАЖДОГО из трёх → unknown «evidence неполные».
	for name, broken := range map[string]MutationEvidence{
		"без-идентификатора": {ExternalObjectVersion: "v4", ExternalObjectTime: complete.ExternalObjectTime},
		"без-версии":         {ExternalObjectID: "msg-2026-77", ExternalObjectTime: complete.ExternalObjectTime},
		"без-времени":        {ExternalObjectID: "msg-2026-77", ExternalObjectVersion: "v4"},
		"пустые-пробелы":     {ExternalObjectID: "  ", ExternalObjectVersion: "v4", ExternalObjectTime: complete.ExternalObjectTime},
	} {
		got, err := GuardCommittedEvidence(MutationOutcomeCommitted, broken)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != OperationUnknown {
			t.Fatalf("%s: честный исход %q, ожидался unknown — терминальный успех не берётся на слово", name, got)
		}
	}
	// Не-committed заявки evidence не касаются: слово проходит как есть.
	for _, word := range []string{MutationOutcomeAccepted, MutationOutcomeFailed, MutationOutcomeTimeout} {
		got, err := GuardCommittedEvidence(word, MutationEvidence{})
		if err != nil || got != word {
			t.Fatalf("исход %q без evidence обязан проходить как есть: %q, %v", word, got, err)
		}
	}
	// Чужое слово отказывает (гейт словаря внутри — belt over braces).
	if _, err := GuardCommittedEvidence("delivered", complete); err == nil {
		t.Fatal("чужое слово обязано отказывать и в evidence-гейте")
	}
}

// TestDeriveMutationStateWordMapping pins the outcome→state mapping by
// name: accepted/committed/failed — своими словами, timeout — honest
// unknown, unknown (отказ evidence/чужого слова) — unknown.
func TestDeriveMutationStateWordMapping(t *testing.T) {
	cases := []struct{ outcome, want string }{
		{MutationOutcomeAccepted, OperationAccepted},
		{MutationOutcomeCommitted, OperationCommitted},
		{MutationOutcomeFailed, OperationFailed},
		{MutationOutcomeTimeout, OperationUnknown},
		{OperationUnknown, OperationUnknown},
	}
	for _, tc := range cases {
		got, err := DeriveMutationStateWord(tc.outcome)
		if err != nil || got != tc.want {
			t.Fatalf("исход %q → состояние %q, %v — ожидалось %q", tc.outcome, got, err, tc.want)
		}
	}
	if _, err := DeriveMutationStateWord("delivered"); err == nil {
		t.Fatal("чужое слово обязано отказывать в маппинге состояния")
	}
}

// TestDeriveReceiptOutcomeWordMapping pins the outcome→receipt mapping:
// accepted carries NO receipt (the empty word — не-терминальное, итога
// ещё нет); committed/failed/unknown are the deciding outcomes (AC-9: the
// receipt reflects the uncertainty, never a success at unknown).
func TestDeriveReceiptOutcomeWordMapping(t *testing.T) {
	cases := []struct {
		outcome string
		want    string
	}{
		{MutationOutcomeAccepted, ""},
		{MutationOutcomeCommitted, ReceiptOutcomeCommitted},
		{MutationOutcomeFailed, ReceiptOutcomeFailed},
		{MutationOutcomeTimeout, ReceiptOutcomeUnknown},
		{OperationUnknown, ReceiptOutcomeUnknown},
	}
	for _, tc := range cases {
		got, err := DeriveReceiptOutcomeWord(tc.outcome)
		if err != nil || got != tc.want {
			t.Fatalf("исход %q → receipt %q, %v — ожидалось %q", tc.outcome, got, err, tc.want)
		}
	}
	if got, _ := DeriveReceiptOutcomeWord(MutationOutcomeAccepted); got != "" {
		t.Fatalf("accepted не имеет квитанции: %q, ожидалось пустое слово", got)
	}
	if _, err := DeriveReceiptOutcomeWord("delivered"); err == nil {
		t.Fatal("чужое слово обязано отказывать в маппинге квитанции")
	}
}

// ---------------------------------------------------------------------------
// The confirmation dispatch fence (WITHOUT exhausted)
// ---------------------------------------------------------------------------

// TestDeriveConfirmationDispatchFenceWithoutExhausted pins the fence
// derivation: superseded/revoked/expired fence the queued intent;
// valid AND exhausted pass (the documented decision — the consumption
// after a successful authorization is the EXPECTED state); a foreign
// state word refuses fail-closed.
func TestDeriveConfirmationDispatchFenceWithoutExhausted(t *testing.T) {
	fenced := map[string]string{
		ConfirmationSuperseded: DispatchFenceSuperseded,
		ConfirmationRevoked:    DispatchFenceRevoked,
		ConfirmationExpired:    DispatchFenceExpired,
	}
	for state, want := range fenced {
		got, err := DeriveConfirmationDispatchFence(state)
		if err != nil || got != want {
			t.Fatalf("состояние %q → фенция %q, %v — ожидалось %q", state, got, err, want)
		}
	}
	for _, passing := range []string{ConfirmationValid, ConfirmationExhausted} {
		got, err := DeriveConfirmationDispatchFence(passing)
		if err != nil || got != "" {
			t.Fatalf("состояние %q обязано проходить без фенции: %q, %v", passing, got, err)
		}
	}
	for _, foreign := range []string{"", "unknown", "valid_until", "Authorized"} {
		if _, err := DeriveConfirmationDispatchFence(foreign); err == nil {
			t.Fatalf("чужое состояние %q обязано отказывать", foreign)
		}
	}
}

// ---------------------------------------------------------------------------
// The retry policy: class × guarantee × bound
// ---------------------------------------------------------------------------

// TestDeriveDispatchRetryPolicyMatrix pins the repeat policy: retry ONLY
// provider_idempotent AND the recorded guarantee AND the attempts within
// the bound; the bound is checked FIRST (the normative order); the
// decided operations never repeat; the first attempt and the
// above-the-bound attempt refuse fail-closed. The 3-7 reconciliation
// unlock: failed + the PROVEN no_effect retries WITHOUT the guarantee
// (inside the bound — never above it); a no_effect claim on a
// non-failed state refuses fail-closed (the verdict writes the failed
// state — a mismatch is data corruption).
func TestDeriveDispatchRetryPolicyMatrix(t *testing.T) {
	// Живые вопросы повтора: submitted (крах-восстановление) и unknown.
	live := []string{OperationSubmitted, OperationUnknown}
	// retry: provider_idempotent + гарантия + попытки в пределах.
	for _, state := range live {
		for attempt := 1; attempt < MaxDispatchAttempts; attempt++ {
			got, err := DeriveDispatchRetryPolicy(state, CapabilityProviderIdempotent, true, attempt, false)
			if err != nil || got != DispatchRetryRetry {
				t.Fatalf("%s попытка %d: %q, %v — ожидался retry", state, attempt, got, err)
			}
		}
	}
	// park_bound: предел проверяется ПЕРВЫМ — даже с гарантией.
	for _, state := range live {
		got, err := DeriveDispatchRetryPolicy(state, CapabilityProviderIdempotent, true, MaxDispatchAttempts, false)
		if err != nil || got != DispatchRetryParkBound {
			t.Fatalf("%s на пределе: %q, %v — ожидался park_bound (предел первый)", state, got, err)
		}
	}
	// park_unknown: без гарантии ИЛИ класс не provider_idempotent — слепой
	// повтор запрещён (AC-9).
	for _, state := range live {
		for _, class := range []string{CapabilityQueryReconcilable, CapabilityCompensatable, CapabilityUnsafe} {
			got, err := DeriveDispatchRetryPolicy(state, class, true, 1, false)
			if err != nil || got != DispatchRetryParkUnknown {
				t.Fatalf("%s класс %s: %q, %v — ожидался park_unknown (класс не идемпотентен)", state, class, got, err)
			}
		}
		got, err := DeriveDispatchRetryPolicy(state, CapabilityProviderIdempotent, false, 1, false)
		if err != nil || got != DispatchRetryParkUnknown {
			t.Fatalf("%s без гарантии: %q, %v — ожидался park_unknown (слепой повтор запрещён)", state, got, err)
		}
	}
	// decided: решённые операции не повторяются вовсе (accepted —
	// не-терминальное ожидание без повторной отправки; failed терминален —
	// новый запуск = новая Confirmation 3-5; committed терминален).
	for _, state := range []string{OperationAccepted, OperationCommitted, OperationFailed} {
		got, err := DeriveDispatchRetryPolicy(state, CapabilityProviderIdempotent, true, 1, false)
		if err != nil || got != DispatchRetryDecided {
			t.Fatalf("%s: %q, %v — ожидался decided (повтор — не вопрос политики)", state, got, err)
		}
	}
	// Retry-unlock сверки 3-7 (AC-3/AC-6): failed + доказанное no_effect
	// => retry БЕЗ гарантии и БЕЗ класса provider_idempotent — доказательство
	// пустоты эффекта заменяет гарантию; НОВОГО intent/резервации нет.
	for _, class := range []string{CapabilityProviderIdempotent, CapabilityQueryReconcilable, CapabilityCompensatable, CapabilityUnsafe} {
		got, err := DeriveDispatchRetryPolicy(OperationFailed, class, false, 1, true)
		if err != nil || got != DispatchRetryRetry {
			t.Fatalf("unlock failed класс %s: %q, %v — ожидался retry без гарантии", class, got, err)
		}
	}
	// Unlock работает ВНУТРИ бюджета: на пределе — park_bound, не retry.
	if got, err := DeriveDispatchRetryPolicy(OperationFailed, CapabilityUnsafe, false, MaxDispatchAttempts, true); err != nil || got != DispatchRetryParkBound {
		t.Fatalf("unlock на пределе: %q, %v — ожидался park_bound (предел первый)", got, err)
	}
	// fail-closed: первая попытка — не вопрос политики повтора; попытка
	// выше предела структурно невозможна; чужие слова отказывают.
	for _, attempt := range []int{0, MaxDispatchAttempts + 1} {
		if _, err := DeriveDispatchRetryPolicy(OperationUnknown, CapabilityProviderIdempotent, true, attempt, false); err == nil {
			t.Fatalf("попытка %d обязана отказывать — вне окна повтора", attempt)
		}
	}
	if _, err := DeriveDispatchRetryPolicy("executed", CapabilityProviderIdempotent, true, 1, false); err == nil {
		t.Fatal("чужое состояние обязано отказывать")
	}
	if _, err := DeriveDispatchRetryPolicy(OperationUnknown, "retryable", true, 1, false); err == nil {
		t.Fatal("чужой класс capability обязан отказывать")
	}
	// no_effect вердикт несовместим с любым состоянием кроме failed
	// (вердикт пишет пару {ledger failed, receipt failed}): порча данных,
	// отказ fail-closed, молчаливое игнорирование запрещено.
	for _, state := range []string{OperationAuthorized, OperationSubmitted, OperationAccepted, OperationCommitted, OperationUnknown} {
		if _, err := DeriveDispatchRetryPolicy(state, CapabilityProviderIdempotent, true, 1, true); err == nil {
			t.Fatalf("доказанное no_effect на состоянии %s обязано отказывать — несовместимо", state)
		}
	}
}

// ---------------------------------------------------------------------------
// The external idempotency key: ONE key per operation, stable across
// attempts, keyed (AD-10)
// ---------------------------------------------------------------------------

// TestDeriveExternalIdempotencyKeyProperties pins the external key: ONE
// key per operation (deterministic), STABLE ACROSS ATTEMPTS (the attempt
// number is structurally absent from the signature — the provider
// deduplicates the repeats), distinct per tenant/operation, keyed HMAC
// (never the raw unsalted hash), and separated from every other key
// space of the module.
func TestDeriveExternalIdempotencyKeyProperties(t *testing.T) {
	tenant := dispatchOperationFixture().TenantID
	operation := dispatchOperationFixture().OperationID

	first := DeriveExternalIdempotencyKey(dispatchKeyFixture, tenant, operation)
	if first != DeriveExternalIdempotencyKey(dispatchKeyFixture, tenant, operation) {
		t.Fatal("внешний ключ недетерминирован для одной операции")
	}
	// ЗОНД стабильности между попытками: в сигнатуре НЕТ параметра
	// попытки (и НИКАКОГО материала) — повтор несёт тот же ключ структурно.
	ft := reflect.TypeOf(DeriveExternalIdempotencyKey)
	if ft.NumIn() != 3 || ft.NumOut() != 1 {
		t.Fatalf("сигнатура внешнего ключа обязана быть (ключ, тенент, операция)→строка: %v", ft)
	}
	// Различимость: другая операция/другой тенент — другой ключ.
	if got := DeriveExternalIdempotencyKey(dispatchKeyFixture, tenant, opPreview); got == first {
		t.Fatal("другая операция обязана давать другой внешний ключ")
	}
	if got := DeriveExternalIdempotencyKey(dispatchKeyFixture, opCase, operation); got == first {
		t.Fatal("другой тенент обязан давать другой внешний ключ")
	}
	// Keyed-зонд: другой HMAC-ключ — другой внешний ключ.
	if got := DeriveExternalIdempotencyKey([]byte("another-commitment-key"), tenant, operation); got == first {
		t.Fatal("keyed свойство нарушено: другой ключ обязан давать другой внешний ключ")
	}
	// AD-10 зонд: не сырой несолёный хэш над теми же байтами.
	raw := sha256.Sum256([]byte(strings.Join([]string{tenant.String(), operation.String()}, canonicalFieldSep)))
	if first == hex.EncodeToString(raw[:]) {
		t.Fatal("внешний ключ обязан быть keyed HMAC — сырой несолёный хэш запрещён AD-10")
	}
	// Ручная HMAC-перепроверка пинит префикс dispatch_key:v1:.
	mac := hmac.New(sha256.New, dispatchKeyFixture)
	mac.Write([]byte("dispatch_key:v1:"))
	mac.Write([]byte(strings.Join([]string{tenant.String(), operation.String()}, canonicalFieldSep)))
	if want := hex.EncodeToString(mac.Sum(nil)); want != first {
		t.Fatalf("внешний ключ обязан равняться ручной HMAC-перепроверке:\n ожидалось %s\n получено  %s", want, first)
	}
}

// ---------------------------------------------------------------------------
// The payload rebuild and the request digest
// ---------------------------------------------------------------------------

// TestBuildAuthorizedActionPayloadDeterministicRebuild pins the rebuild:
// the same immutable rows yield the byte-identical payload; mixed rows
// (another tenant/preview version/definition) and a preview whose effects
// are not reproduced by the definition refuse fail-closed — never a
// silent rebuild from corrupted sources.
func TestBuildAuthorizedActionPayloadDeterministicRebuild(t *testing.T) {
	op := dispatchOperationFixture()
	preview := previewFixture()
	def := previewFixtureDefinition()

	payload, err := BuildAuthorizedActionPayload(op, preview, def)
	if err != nil {
		t.Fatal(err)
	}
	// Материал пересобран из неизменяемых строк: пины + цель + typed_input
	// + snapshot + effects — ничего не выдумано.
	if payload.OperationID != op.OperationID || payload.TenantID != op.TenantID {
		t.Fatal("payload обязан пинить операцию и тенент")
	}
	if payload.DefinitionID != def.ID || payload.DefinitionVersion != def.Version {
		t.Fatal("payload обязан пинить определение")
	}
	if payload.Target != op.Target || !reflect.DeepEqual(payload.TypedInput, preview.TypedInput) ||
		!reflect.DeepEqual(payload.Snapshot, preview.Snapshot) || !reflect.DeepEqual(payload.Effects, preview.Effects) {
		t.Fatal("payload обязан нести цель, typed_input, snapshot и effects из неизменяемых строк")
	}
	again, err := BuildAuthorizedActionPayload(op, preview, def)
	if err != nil || !reflect.DeepEqual(payload, again) {
		t.Fatal("пересборка недетерминирована для тех же строк")
	}

	// Смешанные строки отказывают поимённо (integrity).
	mixed := preview
	mixed.VersionNo = preview.VersionNo + 1
	if _, err := BuildAuthorizedActionPayload(op, mixed, def); err == nil {
		t.Fatal("другая версия preview обязана отказывать — смешанные строки")
	}
	foreign := preview
	foreign.TenantID = opCase
	if _, err := BuildAuthorizedActionPayload(op, foreign, def); err == nil {
		t.Fatal("другой тенент preview обязан отказывать")
	}
	otherDef := def
	otherDef.Version = "9.9.9"
	if _, err := BuildAuthorizedActionPayload(op, preview, otherDef); err == nil {
		t.Fatal("другая версия определения обязана отказывать")
	}
	// Effects preview не воспроизводятся определением — integrity-отказ.
	brokenEffects := preview
	brokenEffects.Effects = append([]PreviewEffect(nil), preview.Effects...)
	brokenEffects.Effects[0].ActionRef = "some.other-action@1.0.0"
	if _, err := BuildAuthorizedActionPayload(op, brokenEffects, def); err == nil {
		t.Fatal("невоспроизводимые effects обязаны отказывать — целостность пересборки")
	}
}

// TestDeriveDispatchRequestDigestDeterminismUnderShuffling pins the
// digest: the same payload in ANY input order yields the same digest
// (the canonical bytes sort — map iteration and slice order never
// enter); any material change moves the digest; the derivation is keyed
// HMAC with the dispatch_request:v1: prefix (the manual recomputation),
// never the raw unsalted hash.
func TestDeriveDispatchRequestDigestDeterminismUnderShuffling(t *testing.T) {
	payload := dispatchPayloadFixture()
	baseline := DeriveDispatchRequestDigest(dispatchKeyFixture, payload)
	if baseline != DeriveDispatchRequestDigest(dispatchKeyFixture, payload) {
		t.Fatal("digest недетерминирован для одного payload")
	}

	// Перетасовка входов: typed_input в другом порядке вставки, snapshot и
	// effects в обратном порядке — тот же digest (AD-10/канонизация).
	shuffled := dispatchPayloadFixture()
	shuffled.TypedInput = map[string]string{"template_ref": "order-ready-v3", "client_phone": "+79001112233"}
	shuffled.Snapshot = []MaterialSnapshotRow{shuffled.Snapshot[1], shuffled.Snapshot[0]}
	shuffled.Effects = append([]PreviewEffect(nil), dispatchPayloadFixture().Effects...)
	if got := DeriveDispatchRequestDigest(dispatchKeyFixture, shuffled); got != baseline {
		t.Fatal("порядок входов не обязан менять digest — канонические байты сортированы")
	}

	// Материал двигает digest: значение typed_input, внешние слова
	// snapshot, effects.
	changed := dispatchPayloadFixture()
	changed.TypedInput = map[string]string{"client_phone": "+79001119999", "template_ref": "order-ready-v3"}
	if got := DeriveDispatchRequestDigest(dispatchKeyFixture, changed); got == baseline {
		t.Fatal("другой телефон обязан двигать digest")
	}
	changed = dispatchPayloadFixture()
	changed.Snapshot = []MaterialSnapshotRow{
		{MaterialField: "price", Status: SnapshotFresh, ExternalID: "SKU-99", ExternalVersion: "12"},
		changed.Snapshot[1],
	}
	if got := DeriveDispatchRequestDigest(dispatchKeyFixture, changed); got == baseline {
		t.Fatal("другие внешние слова snapshot обязаны двигать digest")
	}
	changed = dispatchPayloadFixture()
	changed.Effects = []PreviewEffect{{Order: 1, ActionRef: "other@1.0.0", FailureSemantics: FailureAbortRemaining}}
	if got := DeriveDispatchRequestDigest(dispatchKeyFixture, changed); got == baseline {
		t.Fatal("другие effects обязаны двигать digest")
	}

	// Keyed-зонд: другой HMAC-ключ — другой digest.
	if got := DeriveDispatchRequestDigest([]byte("another-commitment-key"), payload); got == baseline {
		t.Fatal("keyed свойство нарушено: другой ключ обязан давать другой digest")
	}
	// AD-10 зонд: не сырой несолёный хэш над каноническими байтами.
	raw := sha256.Sum256(dispatchRequestCanonicalBytes(payload))
	if baseline == hex.EncodeToString(raw[:]) {
		t.Fatal("digest обязан быть keyed HMAC — сырой несолёный хэш материала запрещён AD-10")
	}
	// Ручная HMAC-перепроверка пинит префикс dispatch_request:v1:.
	mac := hmac.New(sha256.New, dispatchKeyFixture)
	mac.Write([]byte("dispatch_request:v1:"))
	mac.Write(dispatchRequestCanonicalBytes(payload))
	if want := hex.EncodeToString(mac.Sum(nil)); want != baseline {
		t.Fatalf("digest обязан равняться ручной HMAC-перепроверке:\n ожидалось %s\n получено  %s", want, baseline)
	}
}

// TestDispatchKeySpacesSeparated pins the domain separation: the external
// dispatch key (dispatch_key:v1:) and the request digest
// (dispatch_request:v1:) never collide with each other nor with the 3-5
// spaces (business_intent:v1:, operation_material:v1:) — none is
// derivable from another.
func TestDispatchKeySpacesSeparated(t *testing.T) {
	op := dispatchOperationFixture()
	payload := dispatchPayloadFixture()

	external := DeriveExternalIdempotencyKey(dispatchKeyFixture, op.TenantID, op.OperationID)
	digest := DeriveDispatchRequestDigest(dispatchKeyFixture, payload)
	intent := DeriveBusinessIntentKey(dispatchKeyFixture, intentInputFixture())
	fingerprint := DeriveIdempotencyFingerprint(dispatchKeyFixture, op.TenantID, operationPinsFixture())

	values := map[string]string{
		"dispatch_key:v1:":       external,
		"dispatch_request:v1:":   digest,
		"business_intent:v1:":    intent,
		"operation_material:v1:": fingerprint,
	}
	seen := map[string]string{}
	for space, value := range values {
		if other, ok := seen[value]; ok {
			t.Fatalf("пространства %s и %s обязаны давать разные значения — ни один ключ не выводится из другого", other, space)
		}
		seen[value] = space
	}
	// И отделены от action_preview:v1: (3-4).
	previewDigest := PreviewDigest(dispatchKeyFixture, op.PreviewID, op.PreviewVersionNo, []byte("canonical-bytes"))
	if external == previewDigest || digest == previewDigest {
		t.Fatal("пространства 3-6 обязаны быть отделены от action_preview:v1:")
	}
}

// ---------------------------------------------------------------------------
// The lease constants and the lease window
// ---------------------------------------------------------------------------

// TestDispatchLeaseConstantsAndWindow pins the lease constants by value
// (the 3-3 precedents) and the lease window derivation: now + the
// definition timeout + the grace; the out-of-bound timeout refuses
// fail-closed.
func TestDispatchLeaseConstantsAndWindow(t *testing.T) {
	if MaxDispatchAttempts != 3 {
		t.Fatalf("MaxDispatchAttempts = %d, ожидалось 3", MaxDispatchAttempts)
	}
	if DispatchLeaseGraceMS != 30000 {
		t.Fatalf("DispatchLeaseGraceMS = %d, ожидалось 30000", DispatchLeaseGraceMS)
	}
	if DispatchDrainLimit != 100 {
		t.Fatalf("DispatchDrainLimit = %d, ожидалось 100", DispatchDrainLimit)
	}
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	got, err := DeriveDispatchLeaseExpiresAt(now, 15000)
	if err != nil {
		t.Fatal(err)
	}
	want := now.Add(15000*time.Millisecond + 30000*time.Millisecond)
	if !got.Equal(want) {
		t.Fatalf("окно lease: %v, ожидалось %v (timeout + grace 30с)", got, want)
	}
	// Максимальный таймаут определения тоже валиден.
	if _, err := DeriveDispatchLeaseExpiresAt(now, TimeoutMSMax); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []int{0, -1, TimeoutMSMax + 1} {
		if _, err := DeriveDispatchLeaseExpiresAt(now, bad); err == nil {
			t.Fatalf("таймаут %d обязан отказывать — вне границ определения", bad)
		}
	}
}

// ---------------------------------------------------------------------------
// The honest texts of the operation life and the next steps (AC-13)
// ---------------------------------------------------------------------------

// TestDispatchHonestTextsPinned pins every honest text by its needle:
// the life states are distinguishable BY TEXT (accepted — итога ещё нет;
// unknown — повтор запрещён; failed терминален), the boundary words name
// the exact cause and the next step.
func TestDispatchHonestTextsPinned(t *testing.T) {
	for text, needle := range map[string]string{
		OperationSubmittedText:           "ответ не записан",
		OperationAcceptedText:            "бизнес-результат не подтверждён",
		OperationCommittedText:           "внешний объект",
		OperationFailedText:              "новой Confirmation",
		OperationUnknownText:             "повтор запрещён",
		DispatchBlindRepeatForbiddenText: "слепой повтор запрещён",
		DispatchAttemptsBoundText:        "предел попыток",
		DispatchRedeliveryDecidedText:    "операция уже решена",
		DispatchEvidenceIncompleteText:   "неполные",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("honest-текст обязан содержать «%s»: %q", needle, text)
		}
	}
	// Каждое состояние жизни различимо текстом (AC-13): authorized и
	// queued — тексты operation.go, остальные — тексты контура.
	for state, text := range map[string]string{
		OperationAuthorized: OperationAuthorizedText,
		OperationSubmitted:  OperationSubmittedText,
		OperationAccepted:   OperationAcceptedText,
		OperationCommitted:  OperationCommittedText,
		OperationFailed:     OperationFailedText,
		OperationUnknown:    OperationUnknownText,
	} {
		if !strings.Contains(text, state) {
			t.Fatalf("текст состояния %q обязан называть слово состояния: %q", state, text)
		}
	}
	// Припарковка: префикс + точная причина (FR-28 — оператор видит
	// стадию и причину).
	if got := DispatchBlockedText("Confirmation отозвана"); got != "Отправка заблокирована: Confirmation отозвана" {
		t.Fatalf("текст припарковки: %q", got)
	}
	// Тексты фенций поимённо; чужое слово — честный отказ словарём.
	for _, fence := range DispatchFenceWords {
		if text := DispatchFenceText(fence); !strings.Contains(text, "Confirmation") {
			t.Fatalf("текст фенции %q обязан говорить про Confirmation: %q", fence, text)
		}
	}
	if text := DispatchFenceText("executed"); !strings.Contains(text, "не входит в словарь") {
		t.Fatalf("чужая фенция обязана отказывать словарём: %q", text)
	}
	// Тексты политики повтора поимённо.
	for _, word := range DispatchRetryWords {
		if text := DispatchRetryText(word); text == "" || strings.Contains(text, "не входит в словарь") {
			t.Fatalf("текст политики %q обязан существовать: %q", word, text)
		}
	}
	if text := DispatchRetryText("again"); !strings.Contains(text, "не входит в словарь") {
		t.Fatalf("чужое слово политики обязано отказывать словарём: %q", text)
	}
}

// ---------------------------------------------------------------------------
// The AD-10 module-wide source probe: no raw unsalted hash over material
// ---------------------------------------------------------------------------

// TestNoRawUnsaltedHashOverMaterial scans the non-test sources of the
// action module and pins the TWO raw SHA-256 sites of the honest present:
// the legacy 3-4 proposal key (preview.go — out of the 3-5 slice, the
// precedent of the 3-5 step-05 QA) and the 4-6 OrderDigest version pin
// (order.go — the content-addressed version of the order composition, the
// OfferDigestOf/SelectionDigestOf class of the cases domain 4-5: a version
// pin, never a receipt/verification digest). Every RECEIPT digest of this
// contour stays keyed HMAC (the value probes above); a future raw hash
// outside these two named sites turns this red.
func TestNoRawUnsaltedHashOverMaterial(t *testing.T) {
	patterns := []string{"*.go", "../*.go", "../store/*.go", "../app/*.go", "../api/*.go", "../port/*.go"}
	total := 0
	files := map[string]int{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range matches {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			content, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			count := strings.Count(string(content), "sha256.Sum256")
			if count > 0 {
				total += count
				files[name] = count
			}
		}
	}
	if total != 2 || files["preview.go"] != 1 || files["order.go"] != 1 {
		t.Fatalf("сырые SHA-256 модуля action — ровно два именинных сайта: легаси DeriveProposalKey (preview.go) и пин версии заказа OrderDigestOf (order.go, 4-6); найдено: %v", files)
	}
}
