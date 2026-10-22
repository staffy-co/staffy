package domain

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// The 3.7 step-01 unit matrix of the reconciliation domain — the
// dictionaries by name (the four verdict words, the three terminal-lock
// decision words, the bounded constants), the fail-closed verdict
// normalization (the semantics×evidence matrix: the committed claim
// survives ONLY with the structural evidence triple — the
// GuardCommittedEvidence discipline of 3-6 reused; the no_effect claim
// survives ONLY under the declared absence semantics — a bare 404
// without the declaration reads no_answer, AC-3 дословно; a foreign
// word reads no_answer — never a masked success), the verdict→writes
// mapping (the two slash decisions of the epic: unknown/reconciling and
// failed/no_effect), the receipt terminal lock (three branches: the
// append over unknown, the idempotent skip, the integrity alarm), the
// lease window and the material backoff with the jitter bounds, and the
// honest texts by name.

// ---------------------------------------------------------------------------
// The dictionaries: closure and the bounded constants
// ---------------------------------------------------------------------------

// TestReconciliationDictionariesByName pins the closed dictionaries and
// the bounded constants of the reconciliation contour: the verdicts —
// ровно четыре слова, решения терминального замка — ровно три; константы
// bounded-дисциплины 3-3/3-6 (предел попыток сверки 3, grace 30000,
// backoff base 30000, jitter ≤5000, bounded-лимит команды 100).
func TestReconciliationDictionariesByName(t *testing.T) {
	wantVerdicts := []string{"committed", "no_effect", "in_progress", "no_answer"}
	if !reflect.DeepEqual(ReconciliationVerdictWords, wantVerdicts) {
		t.Fatalf("словарь вердиктов сверки: %v, ожидалось ровно %v", ReconciliationVerdictWords, wantVerdicts)
	}
	wantDecisions := []string{"append", "skip", "alarm"}
	if !reflect.DeepEqual(ReceiptAppendDecisionWords, wantDecisions) {
		t.Fatalf("словарь решений замка квитанций: %v, ожидалось ровно %v", ReceiptAppendDecisionWords, wantDecisions)
	}
	if MaxReconciliationAttempts != 3 {
		t.Fatalf("предел попыток сверки: %d, ожидалось 3 (bounded)", MaxReconciliationAttempts)
	}
	if ReconcileLeaseGraceMS != 30000 {
		t.Fatalf("grace lease сверки: %d, ожидалось 30000", ReconcileLeaseGraceMS)
	}
	if ReconcileBackoffBaseMS != 30000 {
		t.Fatalf("backoff base сверки: %d, ожидалось 30000", ReconcileBackoffBaseMS)
	}
	if ReconcileBackoffJitterMS != 5000 {
		t.Fatalf("backoff jitter сверки: %d, ожидалось 5000", ReconcileBackoffJitterMS)
	}
	if ReconcileDrainLimit != 100 {
		t.Fatalf("bounded-лимит команды сверки: %d, ожидалось 100", ReconcileDrainLimit)
	}
}

// ---------------------------------------------------------------------------
// The verdict normalization: the semantics×evidence matrix
// ---------------------------------------------------------------------------

// fullReconciliationEvidence — полный структурный тройник внешнего
// объекта (идентификатор/версия/время): минимальное доказательство
// терминального успеха.
func fullReconciliationEvidence() MutationEvidence {
	return MutationEvidence{
		ExternalObjectID:      "ext-msg-77",
		ExternalObjectVersion: "v3",
		ExternalObjectTime:    time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC),
	}
}

// declaredSemantics — объявленная семантика отсутствия: LIVE-слово
// коннектора + непустой ReconciliationQuery определения.
var declaredSemantics = DeclaredReconciliationSemantics{
	LiveWord: "provider-receipt",
	Query:    "sms-status-by-key",
}

// TestDeriveReconciliationVerdictMatrix pins the fail-closed
// normalization: каждый вердикт словом, семантика×evidence матрицей —
// committed требует тройник (каждая неполная часть отказывает в
// no_answer), no_effect требует объявленную семантику (обе половины:
// LIVE-слово И запрос), чужое/пустое слово = no_answer.
func TestDeriveReconciliationVerdictMatrix(t *testing.T) {
	full := fullReconciliationEvidence()
	cases := []struct {
		name     string
		claimed  string
		evidence MutationEvidence
		declared DeclaredReconciliationSemantics
		want     string
	}{
		// committed: тройник полон — терминальный успех подтверждён
		// (семантика отсутствия не участвует).
		{"committed полный тройник", ReconciliationVerdictCommitted, full, declaredSemantics, ReconciliationVerdictCommitted},
		{"committed без семантики", ReconciliationVerdictCommitted, full, DeclaredReconciliationSemantics{}, ReconciliationVerdictCommitted},
		// committed без тройника: терминальная заявка отвергнута —
		// State/Authority, попытка no_answer (каждая часть тройника).
		{"committed без идентификатора", ReconciliationVerdictCommitted, MutationEvidence{ExternalObjectVersion: "v3", ExternalObjectTime: full.ExternalObjectTime}, declaredSemantics, ReconciliationVerdictNoAnswer},
		{"committed без версии", ReconciliationVerdictCommitted, MutationEvidence{ExternalObjectID: "ext-msg-77", ExternalObjectTime: full.ExternalObjectTime}, declaredSemantics, ReconciliationVerdictNoAnswer},
		{"committed без времени", ReconciliationVerdictCommitted, MutationEvidence{ExternalObjectID: "ext-msg-77", ExternalObjectVersion: "v3"}, declaredSemantics, ReconciliationVerdictNoAnswer},
		{"committed пустой тройник", ReconciliationVerdictCommitted, MutationEvidence{}, declaredSemantics, ReconciliationVerdictNoAnswer},
		// no_effect: ТОЛЬКО при объявленной семантике отсутствия (обе
		// половины), голый 404 без объявления = no_answer (AC-3 дословно).
		{"no_effect объявлена", ReconciliationVerdictNoEffect, MutationEvidence{}, declaredSemantics, ReconciliationVerdictNoEffect},
		{"no_effect только LIVE-слово", ReconciliationVerdictNoEffect, MutationEvidence{}, DeclaredReconciliationSemantics{LiveWord: "provider-receipt"}, ReconciliationVerdictNoAnswer},
		{"no_effect только запрос", ReconciliationVerdictNoEffect, MutationEvidence{}, DeclaredReconciliationSemantics{Query: "sms-status-by-key"}, ReconciliationVerdictNoAnswer},
		{"no_effect ничего не объявлено", ReconciliationVerdictNoEffect, MutationEvidence{}, DeclaredReconciliationSemantics{}, ReconciliationVerdictNoAnswer},
		// no_effect с тройником или без — семантика решает, не evidence.
		{"no_effect с тройником", ReconciliationVerdictNoEffect, full, declaredSemantics, ReconciliationVerdictNoEffect},
		// in_progress: объявленное промежуточное — никогда не терминальное.
		{"in_progress с семантикой", ReconciliationVerdictInProgress, MutationEvidence{}, declaredSemantics, ReconciliationVerdictInProgress},
		{"in_progress без семантики", ReconciliationVerdictInProgress, MutationEvidence{}, DeclaredReconciliationSemantics{}, ReconciliationVerdictInProgress},
		// no_answer: честное отсутствие вердикта — проходит как есть.
		{"no_answer как есть", ReconciliationVerdictNoAnswer, MutationEvidence{}, declaredSemantics, ReconciliationVerdictNoAnswer},
		// чужие/пустые слова: ошибка вызова, таймаут, чужое слово — всё
		// no_answer (никогда не замаскированный успех).
		{"пустое слово", "", MutationEvidence{}, declaredSemantics, ReconciliationVerdictNoAnswer},
		{"чужое слово delivered", "delivered", MutationEvidence{}, declaredSemantics, ReconciliationVerdictNoAnswer},
		{"чужое слово success", "success", full, declaredSemantics, ReconciliationVerdictNoAnswer},
		{"чужое слово с тройником и семантикой", "COMMITTED", full, declaredSemantics, ReconciliationVerdictNoAnswer},
	}
	for _, tc := range cases {
		if got := DeriveReconciliationVerdict(tc.claimed, tc.evidence, tc.declared); got != tc.want {
			t.Fatalf("%s: %q, ожидалось %q", tc.name, got, tc.want)
		}
	}
}

// TestDeriveReconciliationVerdictEffects pins the two slash decisions of
// the epic: committed пишет пару {ledger committed, receipt committed};
// no_effect пишет пару {ledger failed, receipt failed} — доказательство
// отсутствия едет содержимым квитанции, словари состояний (6) и
// квитанций (3) НЕ растут; in_progress/no_answer не пишут НИ ledger-слова
// НИ квитанции (операция остаётся unknown); чужое слово отказывает.
func TestDeriveReconciliationVerdictEffects(t *testing.T) {
	committed, err := DeriveReconciliationVerdictEffects(ReconciliationVerdictCommitted)
	if err != nil || committed.LedgerWord != LedgerEntryCommitted || committed.ReceiptWord != ReceiptOutcomeCommitted {
		t.Fatalf("committed: %+v, %v — ожидалась пара {committed, committed}", committed, err)
	}
	noEffect, err := DeriveReconciliationVerdictEffects(ReconciliationVerdictNoEffect)
	if err != nil || noEffect.LedgerWord != LedgerEntryFailed || noEffect.ReceiptWord != ReceiptOutcomeFailed {
		t.Fatalf("no_effect: %+v, %v — ожидалась пара {failed, failed} (слэш failed/no_effect)", noEffect, err)
	}
	for _, verdict := range []string{ReconciliationVerdictInProgress, ReconciliationVerdictNoAnswer} {
		effects, err := DeriveReconciliationVerdictEffects(verdict)
		if err != nil || effects.LedgerWord != "" || effects.ReceiptWord != "" {
			t.Fatalf("%s: %+v, %v — ожидается пустой план записей (unknown остаётся)", verdict, effects, err)
		}
	}
	if _, err := DeriveReconciliationVerdictEffects("decided"); err == nil {
		t.Fatal("чужой вердикт обязан отказывать fail-closed — план записей не выводится")
	}
}

// ---------------------------------------------------------------------------
// The receipt terminal lock: three branches (epic AC-5)
// ---------------------------------------------------------------------------

// TestGuardReceiptAppendTerminalLock pins the append discipline: append
// ТОЛЬКО поверх последней unknown (оба терминальных исхода); поверх
// терминала тот же исход = идемпотентный skip «сверка подтверждает
// итог»; другой терминал = integrity alarm «конфликт терминальных
// итогов» без авто-замены; чужие слова и нетерминальные заявки
// отказывают fail-closed.
func TestGuardReceiptAppendTerminalLock(t *testing.T) {
	// append: последняя версия unknown — единственный путь записи v2+.
	for _, claimed := range []string{ReceiptOutcomeCommitted, ReceiptOutcomeFailed} {
		if got, err := GuardReceiptAppend(ReceiptOutcomeUnknown, claimed); err != nil || got != ReceiptAppendAppend {
			t.Fatalf("append поверх unknown (%s): %q, %v — ожидался append", claimed, got, err)
		}
	}
	// skip: терминал + тот же терминал — идемпотентное подтверждение.
	if got, err := GuardReceiptAppend(ReceiptOutcomeCommitted, ReceiptOutcomeCommitted); err != nil || got != ReceiptAppendSkip {
		t.Fatalf("committed поверх committed: %q, %v — ожидался skip (подтверждение)", got, err)
	}
	if got, err := GuardReceiptAppend(ReceiptOutcomeFailed, ReceiptOutcomeFailed); err != nil || got != ReceiptAppendSkip {
		t.Fatalf("failed поверх failed: %q, %v — ожидался skip (подтверждение)", got, err)
	}
	// alarm: терминал + другой терминал — конфликт, человеку.
	if got, err := GuardReceiptAppend(ReceiptOutcomeCommitted, ReceiptOutcomeFailed); err != nil || got != ReceiptAppendAlarm {
		t.Fatalf("failed поверх committed: %q, %v — ожидался alarm (конфликт терминалов)", got, err)
	}
	if got, err := GuardReceiptAppend(ReceiptOutcomeFailed, ReceiptOutcomeCommitted); err != nil || got != ReceiptAppendAlarm {
		t.Fatalf("committed поверх failed: %q, %v — ожидался alarm (конфликт терминалов)", got, err)
	}
	// fail-closed: чужая последняя версия.
	if _, err := GuardReceiptAppend("parked", ReceiptOutcomeCommitted); err == nil {
		t.Fatal("чужой исход последней квитанции обязан отказывать")
	}
	// fail-closed: нетерминальная заявка — квитанцию не пишет, замок не
	// спрашивается (категорическая ошибка, не молчаливый пропуск).
	for _, claimed := range []string{ReceiptOutcomeUnknown, ReconciliationVerdictInProgress, ReconciliationVerdictNoAnswer, "decided"} {
		if _, err := GuardReceiptAppend(ReceiptOutcomeUnknown, claimed); err == nil {
			t.Fatalf("нетерминальная заявка %q обязана отказывать — квитанцию не пишет", claimed)
		}
	}
}

// ---------------------------------------------------------------------------
// The lease window and the material backoff (AC-1)
// ---------------------------------------------------------------------------

// TestReconciliationLeaseWindowAndBackoff pins the lease window (now +
// таймаут определения + grace) и материальный backoff (now + base +
// jitter записан в lease): границы таймаута и jitter отказывают
// fail-closed — деривация не верит вызывающему.
func TestReconciliationLeaseWindowAndBackoff(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	// Окно lease: now + таймаут + grace (окно переживает живой запрос).
	got, err := DeriveReconciliationLeaseExpiresAt(now, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if want := now.Add(35 * time.Second); !got.Equal(want) {
		t.Fatalf("окно lease сверки: %v, ожидалось %v (5000 мс + grace 30000)", got, want)
	}
	for _, timeout := range []int{0, -1, TimeoutMSMax + 1} {
		if _, err := DeriveReconciliationLeaseExpiresAt(now, timeout); err == nil {
			t.Fatalf("таймаут %d вне границ определения обязан отказывать", timeout)
		}
	}
	// Backoff: материальный next_eligible_at = now + base + jitter.
	gotUntil, err := DeriveReconciliationBackoffUntil(now, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := now.Add(30 * time.Second); !gotUntil.Equal(want) {
		t.Fatalf("backoff без jitter: %v, ожидалось %v (base 30000)", gotUntil, want)
	}
	gotUntil, err = DeriveReconciliationBackoffUntil(now, ReconcileBackoffJitterMS)
	if err != nil {
		t.Fatal(err)
	}
	if want := now.Add(35 * time.Second); !gotUntil.Equal(want) {
		t.Fatalf("backoff с полным jitter: %v, ожидалось %v (base 30000 + jitter 5000)", gotUntil, want)
	}
	for _, jitter := range []int{-1, ReconcileBackoffJitterMS + 1} {
		if _, err := DeriveReconciliationBackoffUntil(now, jitter); err == nil {
			t.Fatalf("jitter %d вне окна [0..5000] обязан отказывать", jitter)
		}
	}
}

// ---------------------------------------------------------------------------
// The honest texts: by name, fail-closed on a foreign word
// ---------------------------------------------------------------------------

// TestReconciliationHonestTextsByName pins the honest texts of the
// verdicts and the terminal lock: каждое слово говорит различимый текст,
// чужое слово отказывает честным отказом с именем словаря; текст
// разблокировки повтора говорит причину no_effect точно.
func TestReconciliationHonestTextsByName(t *testing.T) {
	for _, word := range ReconciliationVerdictWords {
		if strings.TrimSpace(ReconciliationVerdictText(word)) == "" {
			t.Fatalf("вердикт %q обязан иметь честный текст", word)
		}
	}
	foreign := ReconciliationVerdictText("decided")
	if !strings.Contains(foreign, "no_answer") || !strings.Contains(foreign, "словарь") {
		t.Fatalf("чужой вердикт обязан называть словарь в отказе: %q", foreign)
	}
	for _, word := range ReceiptAppendDecisionWords {
		switch word {
		case ReceiptAppendSkip:
			if !strings.Contains(ReceiptAppendSkipText, "подтверждает итог") {
				t.Fatal("текст skip обязан говорить «подтверждает итог»")
			}
		case ReceiptAppendAlarm:
			if !strings.Contains(ReceiptAppendAlarmText, "требуется человек") {
				t.Fatal("текст alarm обязан говорить «требуется человек»")
			}
		}
	}
	if !strings.Contains(DispatchNoEffectUnlockText, "повтор разрешён") || !strings.Contains(DispatchNoEffectUnlockText, "no_effect") {
		t.Fatal("текст разблокировки обязан называть причину no_effect и разрешение повтора")
	}
}
