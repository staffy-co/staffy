package domain

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// The 3.8 step-01 unit matrix of the recovery-case domain — the
// dictionaries by name (the three trigger words, the two event kind
// words, the one case state word, the two repeat decision words, the
// bounded constants, the outbox word growth 2→3 pinned in
// dispatch_test), the read-time eligibility ladder over the RECORDED
// facts (the semantics×state×facts matrix: unknown×attempts×wired×
// declared, failed×retry-policy — every gas is an honest refusal word,
// NEVER a write), the tenant-scoped recovery key (the keyed HMAC space
// recovery_key:v1: — determinism, the 64-hex shape, the key-space
// separation), the SAFE bundle digest (the keyed canonical identity:
// determinism, the field sensitivity, the observations order
// independence), the repeat comparator (idempotent/evidence with the
// fail-closed shape probe), the assignment derivation from the LIVE
// readings ONLY (both fail-closed refusals — the owner-unknown and the
// coverage-unavailable, the request-transfer deadline precedent), the
// read-time case state (open BY CONSTRUCTION), the trigger texts by
// name, and the next-step word of the eligible operations (the journeys
// z36 registration — rule 161: the domain word change is pinned here,
// the frozen inherited texts stay byte-stable).

// ---------------------------------------------------------------------------
// The dictionaries: closure and the bounded constants
// ---------------------------------------------------------------------------

// TestRecoveryDictionariesByName pins the closed dictionaries and the
// bounded constants of the recovery contour: the triggers — ровно три
// слова, kind событий — ровно два, состояние кейса — ровно одно (open),
// решения повтора — ровно два; SAFE-details буйнд 500 (прецедент
// ledger), буйнд пинов наблюдений 8, префикс ключа — четвёртая семья.
func TestRecoveryDictionariesByName(t *testing.T) {
	wantTriggers := []string{"reconciliation_exhausted", "reconciliation_unavailable", "operation_failed"}
	if !reflect.DeepEqual(RecoveryTriggerWords, wantTriggers) {
		t.Fatalf("словарь триггеров восстановления: %v, ожидалось ровно %v", RecoveryTriggerWords, wantTriggers)
	}
	wantKinds := []string{"created", "evidence_appended"}
	if !reflect.DeepEqual(RecoveryEventKindWords, wantKinds) {
		t.Fatalf("словарь kind событий Recovery Case: %v, ожидалось ровно %v", RecoveryEventKindWords, wantKinds)
	}
	if !reflect.DeepEqual(RecoveryCaseStateWords, []string{"open"}) {
		t.Fatalf("словарь состояний Recovery Case: %v, ожидалось ровно [open] (закрывающих слов в срезе нет)", RecoveryCaseStateWords)
	}
	wantRepeat := []string{"idempotent", "evidence"}
	if !reflect.DeepEqual(RecoveryRepeatWords, wantRepeat) {
		t.Fatalf("словарь решений повтора: %v, ожидалось ровно %v", RecoveryRepeatWords, wantRepeat)
	}
	if RecoverySafeDetailsRunes != 500 {
		t.Fatalf("буйнд SAFE details: %d, ожидалось 500 (прецедент ledger)", RecoverySafeDetailsRunes)
	}
	if RecoveryObservationsPinBound != 8 {
		t.Fatalf("буйнд пинов наблюдений: %d, ожидалось 8", RecoveryObservationsPinBound)
	}
	if RecoveryKeyPrefix != "recovery_key:v1:" {
		t.Fatalf("префикс ключа восстановления: %q, ожидался recovery_key:v1: (четвёртая семья)", RecoveryKeyPrefix)
	}
	// Словарь событий outbox вырос 2→3 санкционированно (спека 3-8:
	// ALTER CHECK 2→3, осознанное зеркальное ревью пина dispatch_test).
	if !WordIn(OutboxEventWords, "recovery_case.created") || len(OutboxEventWords) != 3 {
		t.Fatalf("словарь событий outbox: %v, ожидалось ровно 3 слова с recovery_case.created", OutboxEventWords)
	}
	// Замороженные словари 3-1..3-7 стабильны: состояния 6, ledger 8,
	// вердикты сверки 4, receipt-исходы 3 (срез их не трогает).
	if len(OperationStateWords) != 6 || len(LedgerEntryKindWords) != 8 ||
		len(ReconciliationVerdictWords) != 4 || len(ReceiptOutcomeWords) != 3 {
		t.Fatalf("замороженные словари разошлись: states=%d ledger=%d verdicts=%d receipts=%d",
			len(OperationStateWords), len(LedgerEntryKindWords), len(ReconciliationVerdictWords), len(ReceiptOutcomeWords))
	}
}

// ---------------------------------------------------------------------------
// The eligibility ladder: the semantics×state×facts matrix
// ---------------------------------------------------------------------------

// eligibleFacts is the wired-and-declared baseline of the matrix.
func eligibleFacts() RecoveryEligibilityFacts {
	return RecoveryEligibilityFacts{
		StateWord:               OperationUnknown,
		ReconcileAttemptCount:   0,
		ReconciliationQueryWord: "orders.status",
		ReconciliationLiveWord:  "declared",
		ReconciliationWired:     true,
		DispatchAttemptCount:    1,
		CapabilityClass:         CapabilityQueryReconcilable,
		GuaranteeRecorded:       false,
		NoEffectProven:          false,
	}
}

// TestRecoveryEligibilityLadder pins the ordered ladder поимённо: state →
// lease-счётчик сверки → объявленность (обе половины + wired) →
// retry-политика. Каждый газ = honest-слово отказа; НИ одного eligibil-
// ответа вне трёх слов словаря.
func TestRecoveryEligibilityLadder(t *testing.T) {
	cases := []struct {
		name        string
		facts       RecoveryEligibilityFacts
		wantTrigger string
		wantRefusal string
		wantErr     bool
	}{
		// --- committed: терминальный успех — восстановления нет.
		{"committed", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.StateWord = OperationCommitted
			return f
		}(), "", RecoveryIneligibleCommittedText, false},
		// --- в исполнении: очередь/отправка/сверка живы.
		{"authorized", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.StateWord = OperationAuthorized
			f.DispatchAttemptCount = 0
			return f
		}(), "", RecoveryIneligibleInFlightText, false},
		{"submitted", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.StateWord = OperationSubmitted
			return f
		}(), "", RecoveryIneligibleInFlightText, false},
		{"accepted", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.StateWord = OperationAccepted
			return f
		}(), "", RecoveryIneligibleInFlightText, false},
		// --- unknown: исчерпание 3/3 — записанный факт ПОБЕЖДАЕТ
		// объявленность и wired (автоматика свой шанс получила).
		{"unknown-исчерпана-3-3", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.ReconcileAttemptCount = 3
			return f
		}(), RecoveryTriggerReconciliationExhausted, "", false},
		{"unknown-исчерпана-при-невайренной-сверке", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.ReconcileAttemptCount = 3
			f.ReconciliationWired = false
			return f
		}(), RecoveryTriggerReconciliationExhausted, "", false},
		{"unknown-исчерпана-при-необъявленной", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.ReconcileAttemptCount = 3
			f.ReconciliationLiveWord = ""
			return f
		}(), RecoveryTriggerReconciliationExhausted, "", false},
		// --- unknown: сверка недоступна — AG-4 production-путь (nil
		// адаптер = факт композиции) и обе необъявленные половины.
		{"unknown-недоступна-AG4-nil", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.ReconciliationWired = false
			return f
		}(), RecoveryTriggerReconciliationUnavailable, "", false},
		{"unknown-недоступна-пустой-query", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.ReconciliationQueryWord = ""
			return f
		}(), RecoveryTriggerReconciliationUnavailable, "", false},
		{"unknown-недоступна-пустое-LIVE-слово", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.ReconciliationLiveWord = ""
			return f
		}(), RecoveryTriggerReconciliationUnavailable, "", false},
		// --- unknown: объявлена, вайрена, попытки < 3 — право автоматики
		// не отбирается (сначала контур 3-7).
		{"unknown-сверка-не-исчерпана-0", eligibleFacts(), "", RecoveryIneligibleReconcileNotExhaustedText, false},
		{"unknown-сверка-не-исчерпана-2", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.ReconcileAttemptCount = 2
			return f
		}(), "", RecoveryIneligibleReconcileNotExhaustedText, false},
		// --- unknown: попытки есть, но объявленность УМЕРЛА (коннектор
		// снял LIVE-слово) — автоматический контур мёртв, человек.
		{"unknown-объявленность-умерла-после-попыток", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.ReconcileAttemptCount = 1
			f.ReconciliationLiveWord = ""
			return f
		}(), RecoveryTriggerReconciliationUnavailable, "", false},
		// --- failed: предел попыток 3/3 — без повтора, граница побеждает
		// политику (unlock работает внутри бюджета, не сверх).
		{"failed-предел-попыток", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.StateWord = OperationFailed
			f.DispatchAttemptCount = 3
			return f
		}(), RecoveryTriggerOperationFailed, "", false},
		{"failed-предел-при-no-effect-unlock", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.StateWord = OperationFailed
			f.DispatchAttemptCount = 3
			f.NoEffectProven = true
			return f
		}(), RecoveryTriggerOperationFailed, "", false},
		// --- failed: retry-право живо (записанный no_effect unlock внутри
		// бюджета) — восстановление преждевременно.
		{"failed-повтор-доступен-no-effect-1", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.StateWord = OperationFailed
			f.DispatchAttemptCount = 1
			f.NoEffectProven = true
			return f
		}(), "", RecoveryIneligibleRetryAvailableText, false},
		{"failed-повтор-доступен-no-effect-2", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.StateWord = OperationFailed
			f.DispatchAttemptCount = 2
			f.NoEffectProven = true
			return f
		}(), "", RecoveryIneligibleRetryAvailableText, false},
		// --- failed: без no_effect — политика отвечает decided (повтор
		// не вопрос политики) — требуется человек.
		{"failed-без-no-effect-гарантия-есть", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.StateWord = OperationFailed
			f.DispatchAttemptCount = 1
			f.CapabilityClass = CapabilityProviderIdempotent
			f.GuaranteeRecorded = true
			return f
		}(), RecoveryTriggerOperationFailed, "", false},
		{"failed-без-no-effect-без-гарантии", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.StateWord = OperationFailed
			f.DispatchAttemptCount = 1
			f.CapabilityClass = CapabilityProviderIdempotent
			f.GuaranteeRecorded = false
			return f
		}(), RecoveryTriggerOperationFailed, "", false},
		// --- fail-closed: чужое слово состояния; failed без попыток
		// (структурно невозможно — порча данных).
		{"чужое-слово-состояния", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.StateWord = "executed"
			return f
		}(), "", "", true},
		{"failed-без-попыток-порча", func() RecoveryEligibilityFacts {
			f := eligibleFacts()
			f.StateWord = OperationFailed
			f.DispatchAttemptCount = 0
			return f
		}(), "", "", true},
	}
	for _, tc := range cases {
		got, err := DeriveRecoveryEligibility(tc.facts)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%s: ожидался fail-closed отказ, получен %+v", tc.name, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: неожиданный отказ: %v", tc.name, err)
		}
		if got.Trigger != tc.wantTrigger {
			t.Fatalf("%s: триггер %q, ожидался %q", tc.name, got.Trigger, tc.wantTrigger)
		}
		if got.RefusalText != tc.wantRefusal {
			t.Fatalf("%s: слово отказа %q, ожидалось %q", tc.name, got.RefusalText, tc.wantRefusal)
		}
		if got.Eligible() != (tc.wantTrigger != "") {
			t.Fatalf("%s: Eligible() разошёлся с триггером", tc.name)
		}
	}
}

// TestRecoveryEligibilityRefusalWordsByContent pins the refusal texts
// поимённо (лестница честности: каждое слово отказа называет причину и
// следующий контур).
func TestRecoveryEligibilityRefusalWordsByContent(t *testing.T) {
	if !strings.Contains(RecoveryIneligibleCommittedText, "committed") {
		t.Fatalf("отказ committed обязан назвать итог: %q", RecoveryIneligibleCommittedText)
	}
	if !strings.Contains(RecoveryIneligibleInFlightText, "исполнении") {
		t.Fatalf("отказ в-исполнении обязан говорить про исполнение: %q", RecoveryIneligibleInFlightText)
	}
	if !strings.Contains(RecoveryIneligibleReconcileNotExhaustedText, "3-7") {
		t.Fatalf("отказ не-исчерпана обязан назвать контур 3-7: %q", RecoveryIneligibleReconcileNotExhaustedText)
	}
	if !strings.Contains(RecoveryIneligibleRetryAvailableText, "Повтор") {
		t.Fatalf("отказ повтора обязан говорить про повтор: %q", RecoveryIneligibleRetryAvailableText)
	}
}

// ---------------------------------------------------------------------------
// The recovery key: the keyed HMAC space recovery_key:v1:
// ---------------------------------------------------------------------------

// TestDeriveRecoveryKey pins the keyed identity of ONE case per
// operation: детерминизм, 64-hex, разделение ключевых пространств (тот
// же ключ и пин — другой префикс = другой дайджест), чувствительность к
// тененту и операции.
func TestDeriveRecoveryKey(t *testing.T) {
	key := []byte("staffy-test-commitment-key-v1")
	tenant := id.MustParse("01994f64-6c00-7000-8000-0000000000aa")
	op := id.MustParse("01994f64-6c00-7000-8000-0000000000e1")
	first := DeriveRecoveryKey(key, tenant, op)
	second := DeriveRecoveryKey(key, tenant, op)
	if first != second {
		t.Fatalf("ключ восстановления не детерминирован: %q vs %q", first, second)
	}
	if len(first) != 64 || !isHex64(first) {
		t.Fatalf("ключ восстановления обязан быть 64-hex: %q", first)
	}
	// Другая операция — другой ключ (UNIQUE-близнец).
	otherOp := id.MustParse("01994f64-6c00-7000-8000-0000000000e2")
	if first == DeriveRecoveryKey(key, tenant, otherOp) {
		t.Fatal("ключ обязан меняться от пина операции")
	}
	// Другой тенент — другой ключ (tenant-scoped).
	otherTenant := id.MustParse("01994f64-6c00-7000-8000-0000000000bb")
	if first == DeriveRecoveryKey(key, otherTenant, op) {
		t.Fatal("ключ обязан меняться от тенента")
	}
	// Другой ключ — другой дайджест (keyed, не сырой хеш).
	if first == DeriveRecoveryKey([]byte("another-key"), tenant, op) {
		t.Fatal("ключ обязан меняться от HMAC-ключа (keyed свойство)")
	}
	// Разделение пространств: тот же вход в семействе business_intent
	// даёт ДРУГОЙ дайджест (префиксы не пересекаются).
	intent := DeriveBusinessIntentKey(key, BusinessIntentInput{TenantID: tenant, OwningSystem: "erp"})
	if first == intent {
		t.Fatal("префикс recovery_key:v1: обязан отделять пространство от business_intent:v1:")
	}
}

// ---------------------------------------------------------------------------
// The SAFE bundle digest: the keyed canonical identity
// ---------------------------------------------------------------------------

// testBundle is the baseline bundle of the digest matrix.
func testBundle() RecoveryBundle {
	return RecoveryBundle{
		CaseID:                id.MustParse("018f0000-0000-7000-8000-000000000001"),
		WorkItemID:            id.MustParse("018f0000-0000-7000-8000-000000000002"),
		OperationID:           id.MustParse("01994f64-6c00-7000-8000-0000000000e1"),
		DefinitionID:          "erp.create_order",
		DefinitionVer:         "1.0.0",
		PreviewID:             id.MustParse("018f0000-0000-7000-8000-000000000010"),
		PreviewVersionNo:      1,
		ConfirmationID:        id.MustParse("018f0000-0000-7000-8000-000000000003"),
		ConfirmationVersionNo: 2,
		OwnerKind:             "human",
		OwnerRef:              id.MustParse("018f0000-0000-7000-8000-000000000005"),
		OwnerEpoch:            7,
		ReleaseDecisionNo:     11,
		ProfileVersion:        3,
		BusinessIntentKey:     strings.Repeat("a", 64),
		IdempotencyKey:        "client-key-word-1",
		MaterialDigest:        strings.Repeat("b", 64),
		DispatchAttempts:      1,
		ReconcileAttempts:     3,
		ReceiptVersionsCount:  2,
		LatestReceiptVersion:  2,
		LatestReceiptOutcome:  "unknown",
		Observations: []VersionPin{
			{Ref: id.MustParse("018f0000-0000-7000-8000-000000000021"), Version: 4},
			{Ref: id.MustParse("018f0000-0000-7000-8000-000000000022"), Version: 9},
		},
		TargetKind:            "catalog",
		TargetRef:             id.MustParse("018f0000-0000-7000-8000-000000000006"),
		OwningSystem:          "erp",
		ExternalObjectID:      "",
		ExternalObjectVersion: "",
		ExternalObjectTime:    "",
	}
}

// TestDeriveBundleDigest pins the keyed canonical digest of the SAFE
// bundle: детерминизм, 64-hex, keyed свойство, чувствительность к
// каждому пину (версии/счётчики/исходы/внешние id), независимость от
// порядка пинов наблюдений (сортировка каноническая).
func TestDeriveBundleDigest(t *testing.T) {
	key := []byte("staffy-test-commitment-key-v1")
	base := testBundle()
	first := DeriveBundleDigest(key, base)
	if first != DeriveBundleDigest(key, testBundle()) {
		t.Fatal("дайджест bundle не детерминирован")
	}
	if !isHex64(first) {
		t.Fatalf("дайджест bundle обязан быть 64-hex: %q", first)
	}
	if first == DeriveBundleDigest([]byte("another-key"), base) {
		t.Fatal("дайджест bundle обязан быть keyed (HMAC)")
	}
	// Чувствительность поимённо: каждая SAFE-колонка двигает дайджест.
	mutations := map[string]func(*RecoveryBundle){
		"preview-версия":      func(b *RecoveryBundle) { b.PreviewVersionNo = 2 },
		"confirmation-версия": func(b *RecoveryBundle) { b.ConfirmationVersionNo = 3 },
		"owner-epoch":         func(b *RecoveryBundle) { b.OwnerEpoch = 8 },
		"dispatch-попытки":    func(b *RecoveryBundle) { b.DispatchAttempts = 2 },
		"reconcile-попытки":   func(b *RecoveryBundle) { b.ReconcileAttempts = 2 },
		"квант-квитанций":     func(b *RecoveryBundle) { b.ReceiptVersionsCount = 3 },
		"последняя-квитанция": func(b *RecoveryBundle) { b.LatestReceiptVersion = 3 },
		"исход-квитанции":     func(b *RecoveryBundle) { b.LatestReceiptOutcome = "committed" },
		"внешний-id":          func(b *RecoveryBundle) { b.ExternalObjectID = "order-77" },
		"внешняя-версия":      func(b *RecoveryBundle) { b.ExternalObjectVersion = "2" },
		"внешнее-время":       func(b *RecoveryBundle) { b.ExternalObjectTime = "2026-10-22T12:00:00Z" },
		"наблюдение-версия":   func(b *RecoveryBundle) { b.Observations[0].Version = 5 },
		"owning-system":       func(b *RecoveryBundle) { b.OwningSystem = "catalog" },
	}
	for name, mutate := range mutations {
		moved := testBundle()
		mutate(&moved)
		if first == DeriveBundleDigest(key, moved) {
			t.Fatalf("дайджест обязан меняться от %q", name)
		}
	}
	// Порядок пинов наблюдений не меняет дайджест (каноническая
	// сортировка — детерминизм множества).
	reordered := testBundle()
	reordered.Observations = []VersionPin{reordered.Observations[1], reordered.Observations[0]}
	if first != DeriveBundleDigest(key, reordered) {
		t.Fatal("дайджест обязан быть независим от порядка пинов наблюдений")
	}
}

// ---------------------------------------------------------------------------
// The repeat comparator: idempotent / evidence / fail-closed
// ---------------------------------------------------------------------------

// TestDeriveRecoveryRepeatDecision pins the comparator дисциплины
// 3-6/3-7: равные дайджесты — идемпотентный возврат (без второй строки),
// дельта — evidence; битый 64-hex — fail-closed отказ (порча, не
// замаскированное равенство).
func TestDeriveRecoveryRepeatDecision(t *testing.T) {
	digestA := strings.Repeat("c", 64)
	digestB := strings.Repeat("d", 64)
	if got, err := DeriveRecoveryRepeatDecision(digestA, digestA); err != nil || got != RecoveryRepeatIdempotent {
		t.Fatalf("равные дайджесты: %q, %v — ожидался idempotent", got, err)
	}
	if got, err := DeriveRecoveryRepeatDecision(digestA, digestB); err != nil || got != RecoveryRepeatEvidence {
		t.Fatalf("разошедшиеся дайджесты: %q, %v — ожидался evidence", got, err)
	}
	bad := []string{"", strings.Repeat("c", 63), strings.Repeat("C", 64), "zz" + strings.Repeat("c", 62)}
	for _, malformed := range bad {
		if _, err := DeriveRecoveryRepeatDecision(malformed, digestA); err == nil {
			t.Fatalf("битый дайджест %q обязан отказывать (fail-closed)", malformed)
		}
		if _, err := DeriveRecoveryRepeatDecision(digestA, malformed); err == nil {
			t.Fatalf("битый свежий дайджест %q обязан отказывать (fail-closed)", malformed)
		}
	}
}

// ---------------------------------------------------------------------------
// The assignment: the LIVE readings ONLY (both fail-closed refusals)
// ---------------------------------------------------------------------------

// TestDeriveRecoveryAssignment pins правила назначения живыми чтениями:
// владелец — текущий терм исходного Case, backup и дедлайн — human
// coverage маршрута; обе fail-closed причины (термов нет; покрытие
// недоступно ИЛИ без слова дедлайна — минуты < 1, request-transfer
// прецедент) отказывают ДО какой-либо записи.
func TestDeriveRecoveryAssignment(t *testing.T) {
	ownerRef := id.MustParse("018f0000-0000-7000-8000-000000000005")
	backup := id.MustParse("018f0000-0000-7000-8000-000000000007")
	good := RecoveryAssignmentInput{
		HasOwnerTerm:               true,
		OwnerKind:                  "human",
		OwnerRef:                   ownerRef,
		OwnerEpoch:                 7,
		CoverageReadable:           true,
		CoverageBackupOwnerID:      backup,
		CoverageAckDeadlineMinutes: 30,
	}
	got, err := DeriveRecoveryAssignment(good)
	if err != nil {
		t.Fatalf("живые чтения не отказывают: %v", err)
	}
	if got.OwnerKind != "human" || got.OwnerRef != ownerRef || got.OwnerEpoch != 7 {
		t.Fatalf("владелец назначения обязан быть термом исходного Case: %+v", got)
	}
	if got.BackupOwnerID != backup || got.AckDeadlineMinutes != 30 {
		t.Fatalf("backup и дедлайн обязаны прийти из coverage: %+v", got)
	}
	// fail-closed №1: термов владельца нет (pre-migration).
	noTerm := good
	noTerm.HasOwnerTerm = false
	if _, err := DeriveRecoveryAssignment(noTerm); err == nil ||
		!strings.Contains(err.Error(), RecoveryOwnerUnknownText) {
		t.Fatalf("отказ «владелец неизвестен» обязателен: %v", err)
	}
	// fail-closed №2a: покрытие непрочитано (порт nil / ошибка чтения).
	noCoverage := good
	noCoverage.CoverageReadable = false
	if _, err := DeriveRecoveryAssignment(noCoverage); err == nil ||
		!strings.Contains(err.Error(), RecoveryCoverageUnavailableText) {
		t.Fatalf("отказ «покрытие недоступно» обязателен: %v", err)
	}
	// fail-closed №2b: покрытие без слова дедлайна (минуты < 1).
	noDeadline := good
	noDeadline.CoverageAckDeadlineMinutes = 0
	if _, err := DeriveRecoveryAssignment(noDeadline); err == nil ||
		!strings.Contains(err.Error(), RecoveryCoverageUnavailableText) {
		t.Fatalf("покрытие без дедлайна обязано отказывать: %v", err)
	}
}

// TestDeriveRecoveryDeadlineAt pins арифметику дедлайна: now + минуты,
// UTC, усечение до микросекунды (request-transfer прецедент).
func TestDeriveRecoveryDeadlineAt(t *testing.T) {
	base := time.Date(2026, 10, 22, 12, 0, 0, 1_500_000_000, time.FixedZone("MSK", 3*3600))
	got := DeriveRecoveryDeadlineAt(base, 30)
	want := time.Date(2026, 10, 22, 9, 30, 0, 1_500_000_000, time.UTC).Truncate(time.Microsecond)
	if !got.Equal(want) {
		t.Fatalf("дедлайн: %v, ожидался %v", got, want)
	}
	if got.Location() != time.UTC {
		t.Fatalf("дедлайн обязан быть UTC: %v", got)
	}
}

// ---------------------------------------------------------------------------
// The read-time case state: open BY CONSTRUCTION
// ---------------------------------------------------------------------------

// TestDeriveRecoveryCaseState pins read-time деривацию состояния: пустая
// история — open, словарные kind — open (закрывающих слов нет в срезе),
// чужой kind — fail-closed отказ словарём.
func TestDeriveRecoveryCaseState(t *testing.T) {
	empty, err := DeriveRecoveryCaseState(nil)
	if err != nil || empty != RecoveryCaseStateOpen {
		t.Fatalf("пустая история: %q, %v — ожидался open", empty, err)
	}
	created := []RecoveryEventRow{{EventNo: 1, Kind: RecoveryEventCreated}}
	if state, err := DeriveRecoveryCaseState(created); err != nil || state != RecoveryCaseStateOpen {
		t.Fatalf("после created: %q, %v — ожидался open", state, err)
	}
	full := append(created, RecoveryEventRow{EventNo: 2, Kind: RecoveryEventEvidenceAppended})
	if state, err := DeriveRecoveryCaseState(full); err != nil || state != RecoveryCaseStateOpen {
		t.Fatalf("после evidence_appended: %q, %v — ожидался open (закрытия нет в словаре среза)", state, err)
	}
	foreign := append(full, RecoveryEventRow{EventNo: 3, Kind: "closed"})
	if _, err := DeriveRecoveryCaseState(foreign); err == nil {
		t.Fatal("чужой kind события обязан отказывать словарём (fail-closed)")
	}
}

// ---------------------------------------------------------------------------
// The trigger texts and the next-step word (the journeys z36 registration)
// ---------------------------------------------------------------------------

// TestRecoveryTriggerTextsByName pins фиксированные тексты триггеров
// поимённо: каждый триггер — своё слово с фиксированным текстом; чужое
// слово отказывает словарём.
func TestRecoveryTriggerTextsByName(t *testing.T) {
	for _, word := range RecoveryTriggerWords {
		text := RecoveryTriggerText(word)
		if text == "" || strings.Contains(text, "не входит в словарь") {
			t.Fatalf("текст триггера %q обязан существовать: %q", word, text)
		}
	}
	if text := RecoveryTriggerText("pending"); !strings.Contains(text, "не входит в словарь") {
		t.Fatalf("чужой триггер обязан отказывать словарём: %q", text)
	}
	if !strings.Contains(RecoveryTriggerText(RecoveryTriggerReconciliationExhausted), "исчерпаны") {
		t.Fatalf("текст исчерпания обязан говорить про исчерпание: %q", RecoveryTriggerText(RecoveryTriggerReconciliationExhausted))
	}
	if !strings.Contains(RecoveryTriggerText(RecoveryTriggerReconciliationUnavailable), "недоступна") {
		t.Fatalf("текст недоступности обязан говорить про недоступность: %q", RecoveryTriggerText(RecoveryTriggerReconciliationUnavailable))
	}
	if !strings.Contains(RecoveryTriggerText(RecoveryTriggerOperationFailed), "человек") {
		t.Fatalf("текст ошибки обязан звать человека: %q", RecoveryTriggerText(RecoveryTriggerOperationFailed))
	}
}

// TestRecoverySurfaceHonestTexts pins поверхностные honest-тексты
// поимённо (платежи-unknown AG-4, локальная проекция, открытое состояние,
// исход создания/идемпотентности/evidence).
func TestRecoverySurfaceHonestTexts(t *testing.T) {
	texts := []string{
		RecoveryCreatedText, RecoveryIdempotentText, RecoveryEvidenceAppendedText,
		RecoveryPaymentsUnknownText, RecoveryLocalProjectionText, RecoveryCaseOpenText,
		RecoveryNextStepText,
	}
	seen := map[string]bool{}
	for _, text := range texts {
		if strings.TrimSpace(text) == "" {
			t.Fatal("honest-текст пуст")
		}
		if seen[text] {
			t.Fatalf("honest-тексты обязаны быть различимы: %q", text)
		}
		seen[text] = true
	}
	if !strings.Contains(RecoveryPaymentsUnknownText, "unknown") {
		t.Fatalf("платёжное слово обязано говорить unknown: %q", RecoveryPaymentsUnknownText)
	}
	if !strings.Contains(RecoveryLocalProjectionText, "не актуальное внешнее состояние") {
		t.Fatalf("слово проекции обязано называть границу: %q", RecoveryLocalProjectionText)
	}
	if !strings.Contains(RecoveryIdempotentText, "уже существует") {
		t.Fatalf("идемпотентное слово обязано говорить про существование: %q", RecoveryIdempotentText)
	}
	if !strings.Contains(RecoveryEvidenceAppendedText, "evidence_appended") {
		t.Fatalf("слово дельты обязано назвать kind: %q", RecoveryEvidenceAppendedText)
	}
}

// TestDeriveOperationNextStepText pins смену доменного слова следующего
// шага (правило DEVLOG 161 — journeys z36 регистрирует её): eligible-
// операция (триггер непустой) говорит recovery-слово; не-eligible
// наследует замороженный текст состояния (z34-слова неизвестности
// «контур 3-7» байт-стабильны); чужой триггер отказывает словарём.
func TestDeriveOperationNextStepText(t *testing.T) {
	// eligible: каждый триггер — recovery-слово.
	for _, trigger := range RecoveryTriggerWords {
		text := DeriveOperationNextStepText(OperationUnknown, trigger)
		if text != RecoveryNextStepText {
			t.Fatalf("eligible unknown (%s): %q, ожидалось recovery-слово %q", trigger, text, RecoveryNextStepText)
		}
		text = DeriveOperationNextStepText(OperationFailed, trigger)
		if text != RecoveryNextStepText {
			t.Fatalf("eligible failed (%s): %q, ожидалось recovery-слово", trigger, text)
		}
	}
	if !strings.Contains(RecoveryNextStepText, "Recovery Case") || !strings.Contains(RecoveryNextStepText, "3-8") {
		t.Fatalf("recovery-слово обязано назвать контур 3-8: %q", RecoveryNextStepText)
	}
	// не-eligible: наследие 3-6/3-7 байт-стабильно (замороженные тексты).
	if got, want := DeriveOperationNextStepText(OperationUnknown, ""), OperationNextStepText(OperationUnknown); got != want {
		t.Fatalf("unknown без eligibility: %q, ожидалось наследие %q", got, want)
	}
	if !strings.Contains(DeriveOperationNextStepText(OperationUnknown, ""), "контур 3-7") {
		t.Fatal("наследное слово unknown обязано остаться «контуром 3-7» (z34-пин)")
	}
	if got, want := DeriveOperationNextStepText(OperationFailed, ""), OperationNextStepText(OperationFailed); got != want {
		t.Fatalf("failed без eligibility: %q, ожидалось наследие %q", got, want)
	}
	// чужой триггер: fail-closed словарём.
	foreign := DeriveOperationNextStepText(OperationUnknown, "pending")
	if !strings.Contains(foreign, "не входит в словарь") {
		t.Fatalf("чужой триггер обязан отказывать словарём: %q", foreign)
	}
}

// TestDeriveRecoveryEvidenceDelta pins SAFE-дельту evidence-строки
// поимённо: изменившиеся пины называют старое→новое (версии квитанций,
// счётчики попыток, внешний объект), неизменившиеся НЕ называются,
// пустая дельта говорит честное слово, буйнд 500 рун держится.
func TestDeriveRecoveryEvidenceDelta(t *testing.T) {
	stored := testBundle()
	// Ничего не двигалось — честное слово пустой дельты.
	if delta := DeriveRecoveryEvidenceDelta(stored, stored); !strings.Contains(delta, "ни один SAFE-пин") {
		t.Fatalf("пустая дельта обязана говорить честное слово: %q", delta)
	}
	// Позднее terminal evidence: квитанция v3 committed с внешним
	// объектом и ещё одна попытка сверки.
	fresh := testBundle()
	fresh.ReceiptVersionsCount = 3
	fresh.LatestReceiptVersion = 3
	fresh.LatestReceiptOutcome = "committed"
	fresh.ReconcileAttempts = 4
	fresh.ExternalObjectID = "order-77"
	fresh.ExternalObjectVersion = "2"
	delta := DeriveRecoveryEvidenceDelta(stored, fresh)
	for _, pin := range []string{"квант-квитанций 2→3", "последняя-квитанция 2→3", "исход-квитанции unknown→committed", "reconcile-попытки 3→4", "внешний-id →order-77"} {
		if !strings.Contains(delta, pin) {
			t.Fatalf("дельта обязана называть пин %q: %q", pin, delta)
		}
	}
	if strings.Contains(delta, "dispatch-попытки") {
		t.Fatalf("неизменившийся пин НЕ называется: %q", delta)
	}
	// Буйнд: длинная дельта обрезается до 500 рун.
	if got := len([]rune(DeriveRecoveryEvidenceDelta(stored, fresh))); got > RecoverySafeDetailsRunes {
		t.Fatalf("дельта обязана держать буйнд 500: %d", got)
	}
}
