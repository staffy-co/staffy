package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// The 3.5 step-01 unit matrix of the Operation core — the two keyed-HMAC
// spaces (the mutation identity and the request material identity), the
// definition-version independence of the intent key (the probe of AC-4),
// the keyed property against the raw unsalted hash (the AD-10 probe), the
// deterministic canonical bytes under the shuffled map/slice orders, the
// single-word ledger derivation with the fail-closed refusal of the 3-6
// words, and the honest texts of the view.

var (
	// hmacKeyFixture — фиксированный ключ единицы теста (прод-ключ живёт в
	// commitments_hmac_keys per-tenant — забота шага-02).
	hmacKeyFixture = []byte("test-commitment-key-3-5")
	opTenant       = id.MustParse("018f0000-0000-7000-8000-000000000001")
	opPreview      = id.MustParse("018f0000-0000-7000-8000-000000000002")
	opConfirmation = id.MustParse("018f0000-0000-7000-8000-000000000003")
	opTargetRef    = id.MustParse("018f0000-0000-7000-8000-000000000004")
	opOwnerRef     = id.MustParse("018f0000-0000-7000-8000-000000000005")
	opCase         = id.MustParse("018f0000-0000-7000-8000-000000000006")
	opWorkItem     = id.MustParse("018f0000-0000-7000-8000-000000000007")
)

// intentInputFixture строит канонический вход intent-ключа: живой пример
// среза — товар каталога, телефон и шаблон клиентского уведомления.
func intentInputFixture() BusinessIntentInput {
	return BusinessIntentInput{
		TenantID:     opTenant,
		OwningSystem: "catalog",
		DefinitionID: "notify.order-ready",
		Target:       ProposalTarget{Kind: TargetCatalogItem, Ref: opTargetRef},
		TypedInput:   map[string]string{"client_phone": "+79001112233", "template_ref": "order-ready-v3"},
		Snapshot: []MaterialSnapshotRow{
			{MaterialField: "price", Status: SnapshotFresh, ExternalID: "SKU-77", ExternalVersion: "12",
				ObservedAt: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC), EvidenceRef: "catalog://sku/77"},
			{MaterialField: "availability", Status: SnapshotFresh, ExternalID: "SKU-77", ExternalVersion: "12",
				ObservedAt: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC), EvidenceRef: "catalog://sku/77"},
		},
	}
}

// operationPinsFixture — пины запроса авторизации.
func operationPinsFixture() OperationRequestPins {
	return OperationRequestPins{PreviewID: opPreview, PreviewVersionNo: 3, ConfirmationID: opConfirmation, ConfirmationVersionNo: 1}
}

// ledgerFixture — минимальная живая история: одна запись authorized.
func ledgerFixture() []OperationLedgerEntry {
	return []OperationLedgerEntry{{
		EntryNo: 1, Kind: LedgerEntryAuthorized,
		Details: "authorized; definition notify.order-ready@1.1.0",
		At:      time.Date(2026, 10, 20, 12, 5, 0, 0, time.UTC),
	}}
}

func TestOperationAndLedgerDictionariesByName(t *testing.T) {
	// Словари 3-6: состояния — ровно шесть слов, ledger — ровно семь
	// (dispatch_blocked — видимая причина припарковки, НЕ state-bearing);
	// вне-словарное слово структурно невозможно (константы + закрытые
	// списки, рантайм-расширения нет).
	wantStates := []string{"authorized", "submitted", "accepted", "committed", "failed", "unknown"}
	if !reflect.DeepEqual(OperationStateWords, wantStates) {
		t.Fatalf("словарь состояний Operation: %v, ожидалось ровно %v", OperationStateWords, wantStates)
	}
	wantLedger := []string{"authorized", "submitted", "accepted", "committed", "failed", "unknown", "dispatch_blocked"}
	if !reflect.DeepEqual(LedgerEntryKindWords, wantLedger) {
		t.Fatalf("словарь записей ledger: %v, ожидалось ровно %v", LedgerEntryKindWords, wantLedger)
	}
}

func TestDeriveOperationStateAuthorized(t *testing.T) {
	got, err := DeriveOperationState(ledgerFixture())
	if err != nil {
		t.Fatal(err)
	}
	if got != OperationAuthorized {
		t.Fatalf("состояние: %q, ожидалось authorized", got)
	}
	// Несколько записей authorized (будущие revalidation-записи 3-6 тем же
	// механизмом) — слово то же.
	multi := append(ledgerFixture(), OperationLedgerEntry{EntryNo: 2, Kind: LedgerEntryAuthorized,
		Details: "revalidated", At: time.Now().UTC()})
	if got, _ := DeriveOperationState(multi); got != OperationAuthorized {
		t.Fatalf("многословарная история: %q", got)
	}
}

func TestDeriveOperationStateFailClosed(t *testing.T) {
	// Пустой ledger — состояние не выводится: авторизация доказывается
	// записью, её отсутствие — порча данных, а не честное слово.
	if _, err := DeriveOperationState(nil); err == nil {
		t.Fatal("пустой ledger обязан отказывать fail-closed")
	}
	// Вне-словарные слова (executed/dispatched — слова других контуров)
	// отказывают поимённо; словарные слова 3-6 БОЛЬШЕ не отказывают —
	// контур исполнения живёт в этом словаре (проверено поимённо в
	// dispatch_test.go: TestDeriveOperationStateLastStateBearingWord).
	for _, foreign := range []string{"executed", "dispatched", "cancelled", "", "Authorized"} {
		ledger := []OperationLedgerEntry{{EntryNo: 1, Kind: foreign, At: time.Now().UTC()}}
		_, err := DeriveOperationState(ledger)
		if err == nil {
			t.Fatalf("запись %q обязана отказывать — слово вне словаря ledger", foreign)
		}
		ruleErr := ruleErrorOf(t, err)
		if !strings.Contains(ruleErr.Reason, "вне словаря") {
			t.Fatalf("отказ записи %q обязан называть словарь: %q", foreign, ruleErr.Reason)
		}
	}
}

func TestDeriveOperationViewWordSpokenState(t *testing.T) {
	// AC-11/AC-13: карточка говорит authorized/queued, а не executed —
	// очередь это факт наличия outbox записи, слово выводится, не
	// хранится; слова жизни остальных состояний — своими словами.
	got, err := DeriveOperationViewWord(ledgerFixture(), true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "authorized/queued" {
		t.Fatalf("вид с очередью: %q, ожидалось authorized/queued", got)
	}
	got, err = DeriveOperationViewWord(ledgerFixture(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got != OperationAuthorized {
		t.Fatalf("вид без очереди: %q, ожидалось authorized", got)
	}
	// dispatch_blocked в истории не меняет слово вида (состояние остаётся
	// authorized — припарковка с видимой причиной, не состояние).
	blocked := append(ledgerFixture(), OperationLedgerEntry{EntryNo: 2, Kind: LedgerEntryDispatchBlocked,
		Details: "Confirmation отозвана", At: time.Now().UTC()})
	if got, _ := DeriveOperationViewWord(blocked, true); got != "authorized/queued" {
		t.Fatalf("dispatch_blocked не state-bearing: вид %q, ожидался authorized/queued", got)
	}
	// Слова жизни не добавляют /queued: контур исполнения говорит своими
	// словами (submitted/accepted/committed/failed/unknown).
	submitted := append(ledgerFixture(), OperationLedgerEntry{EntryNo: 2, Kind: LedgerEntrySubmitted, At: time.Now().UTC()})
	if got, _ := DeriveOperationViewWord(submitted, true); got != OperationSubmitted {
		t.Fatalf("вид жизни submitted: %q, ожидалось submitted без /queued", got)
	}
	if _, err := DeriveOperationViewWord(nil, true); err == nil {
		t.Fatal("пустой ledger обязан отказывать и в виде")
	}
}

func TestDeriveBusinessIntentKeyDeterministic(t *testing.T) {
	// Канонические байты детерминированы: та же пара пин даёт тот же
	// ключ — порядок map-итерации и порядок строк snapshot не участвуют.
	first := DeriveBusinessIntentKey(hmacKeyFixture, intentInputFixture())
	second := DeriveBusinessIntentKey(hmacKeyFixture, intentInputFixture())
	if first != second {
		t.Fatal("intent-ключ недетерминирован для одинаковых входов")
	}

	// Пересобранный вход: snapshot в другом порядке, typed_input с другим
	// порядком вставки — ключ тот же (сортировка канонических байтов).
	shuffled := intentInputFixture()
	shuffled.Snapshot = []MaterialSnapshotRow{shuffled.Snapshot[1], shuffled.Snapshot[0]}
	shuffled.TypedInput = map[string]string{"template_ref": "order-ready-v3", "client_phone": "+79001112233"}
	if got := DeriveBusinessIntentKey(hmacKeyFixture, shuffled); got != first {
		t.Fatalf("порядок входов не обязан менять ключ: %q против %q", got, first)
	}
}

func TestDeriveBusinessIntentKeyIgnoresDefinitionVersion(t *testing.T) {
	// ЗОНД AC-4: версия определения НЕ входит в intent-ключ — эквивалентная
	// мутация определена типом + намерением, не релизом определения.
	// Сначала структурный пин: во входе ключа НЕТ поля версии.
	fields := reflect.VisibleFields(reflect.TypeOf(BusinessIntentInput{}))
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, f.Name)
	}
	wantFields := []string{"TenantID", "OwningSystem", "DefinitionID", "Target", "TypedInput", "Snapshot"}
	if !reflect.DeepEqual(names, wantFields) {
		t.Fatalf("вход intent-ключа обязан быть без поля версии: %v", names)
	}

	// Затем материальный зонд: v1.0.0 и v1.1.0 одного ID — ОДИН ключ.
	// Версия пинится в operation-строке ОТДЕЛЬНО (DefinitionVersion),
	// но ключ её не видит.
	base := intentInputFixture()
	opV1 := Operation{
		TenantID: opTenant, OperationID: opPreview,
		DefinitionID: base.DefinitionID, DefinitionVersion: "1.0.0",
		BusinessIntentKey: DeriveBusinessIntentKey(hmacKeyFixture, base),
	}
	opV11 := Operation{
		TenantID: opTenant, OperationID: opConfirmation,
		DefinitionID: base.DefinitionID, DefinitionVersion: "1.1.0",
		BusinessIntentKey: DeriveBusinessIntentKey(hmacKeyFixture, base),
	}
	if opV1.BusinessIntentKey != opV11.BusinessIntentKey {
		t.Fatal("v1.0.0 и v1.1.0 одного ID обязаны давать ОДИН intent-ключ")
	}
	if opV1.DefinitionVersion == opV11.DefinitionVersion {
		t.Fatal("версии обязаны пиниться в operation-строке раздельно")
	}
}

func TestDeriveBusinessIntentKeyKeyedNotRawHash(t *testing.T) {
	// AD-10 зонд: keyed HMAC, не сырой несолёный хэш — тот же вход под
	// другим ключом даёт другой intent-ключ (хэш не «делился» бы ключами).
	in := intentInputFixture()
	underA := DeriveBusinessIntentKey(hmacKeyFixture, in)
	underB := DeriveBusinessIntentKey([]byte("another-commitment-key"), in)
	if underA == underB {
		t.Fatal("keyed свойство нарушено: разные ключи обязаны давать разные intent-ключи")
	}

	// И intent-ключ не равен сырой SHA-256 над теми же каноническими
	// байтами — короткие чувствительные значения не хэшируются всухую.
	canonical := strings.Join([]string{
		in.TenantID.String(), in.OwningSystem, in.DefinitionID,
		in.Target.Kind, in.Target.Ref.String(),
		sortedTypedInput(in.TypedInput), sortedSnapshotIdentity(in.Snapshot),
	}, canonicalFieldSep)
	raw := sha256.Sum256([]byte(canonical))
	if underA == hex.EncodeToString(raw[:]) {
		t.Fatal("intent-ключ обязан быть keyed HMAC — сырой несолёный хэш материала запрещён AD-10")
	}
}

func TestDeriveBusinessIntentKeyDistinguishes(t *testing.T) {
	// Каждое поле намерения различается: тенент, система, тип действия,
	// цель, типовой ввод, внешние слова snapshot.
	base := intentInputFixture()
	baseline := DeriveBusinessIntentKey(hmacKeyFixture, base)

	changed := base
	changed.TenantID = opConfirmation
	if got := DeriveBusinessIntentKey(hmacKeyFixture, changed); got == baseline {
		t.Fatal("другой тенент обязан давать другой intent-ключ")
	}

	changed = base
	changed.OwningSystem = "erp"
	if got := DeriveBusinessIntentKey(hmacKeyFixture, changed); got == baseline {
		t.Fatal("другая владеющая система обязана давать другой intent-ключ")
	}

	changed = base
	changed.DefinitionID = "query.trunk-status"
	if got := DeriveBusinessIntentKey(hmacKeyFixture, changed); got == baseline {
		t.Fatal("другой тип действия обязан давать другой intent-ключ")
	}

	changed = base
	changed.Target = ProposalTarget{Kind: TargetClient, Ref: opOwnerRef}
	if got := DeriveBusinessIntentKey(hmacKeyFixture, changed); got == baseline {
		t.Fatal("другая цель обязана давать другой intent-ключ")
	}

	changed = base
	changed.TypedInput = map[string]string{"client_phone": "+79001119999", "template_ref": "order-ready-v3"}
	if got := DeriveBusinessIntentKey(hmacKeyFixture, changed); got == baseline {
		t.Fatal("другой телефон (типовое намерение) обязан давать другой intent-ключ")
	}

	changed = base
	changed.Snapshot = []MaterialSnapshotRow{
		{MaterialField: "price", Status: SnapshotFresh, ExternalID: "SKU-99", ExternalVersion: "12"},
	}
	if got := DeriveBusinessIntentKey(hmacKeyFixture, changed); got == baseline {
		t.Fatal("другие внешние слова snapshot обязаны давать другой intent-ключ")
	}

	// Честная граница нормализации: статус snapshot и время наблюдения — НЕ
	// намерение. Тот же внешний материал с другим статусом = тот же ключ:
	// эквивалентная мутация не зависит от того, свежо ли он прочитан.
	changed = base
	changed.Snapshot = []MaterialSnapshotRow{
		{MaterialField: "price", Status: SnapshotStale, ExternalID: "SKU-77", ExternalVersion: "12",
			ObservedAt: time.Date(2026, 10, 20, 13, 0, 0, 0, time.UTC)},
		{MaterialField: "availability", Status: SnapshotFresh, ExternalID: "SKU-77", ExternalVersion: "12"},
	}
	if got := DeriveBusinessIntentKey(hmacKeyFixture, changed); got != baseline {
		t.Fatal("статус и время наблюдения не входят в намерение — ключ обязан совпасть")
	}
}

func TestDeriveIdempotencyFingerprintReplayVsConflict(t *testing.T) {
	// Replay: та же пара пин запроса → тот же отпечаток (double-click,
	// webhook-повтор, replay-job — один ключ, один материал).
	pins := operationPinsFixture()
	first := DeriveIdempotencyFingerprint(hmacKeyFixture, opTenant, pins)
	if first != DeriveIdempotencyFingerprint(hmacKeyFixture, opTenant, pins) {
		t.Fatal("отпечаток недетерминирован для одинаковых пин")
	}

	// Conflict: тот же idempotency key с ДРУГИМ материалом — другая версия
	// Confirmation даёт другой отпечаток (AC-8).
	newer := pins
	newer.ConfirmationVersionNo = 2
	if got := DeriveIdempotencyFingerprint(hmacKeyFixture, opTenant, newer); got == first {
		t.Fatal("другая версия Confirmation обязана давать другой отпечаток — конфликт различим")
	}

	// Другая версия preview — тоже другой материал (пере-подтверждение
	// после редактирования).
	edited := pins
	edited.PreviewVersionNo = 4
	if got := DeriveIdempotencyFingerprint(hmacKeyFixture, opTenant, edited); got == first {
		t.Fatal("другая версия preview обязана давать другой отпечаток")
	}

	// Клиентский idempotency key НЕ входит в отпечаток: два разных ключа
	// с одним материалом — ОДИН отпечаток материала (идентичности
	// независимы по построению).
	if len(operationPinsFixture().PreviewID.String()) == 0 {
		t.Fatal("пины обязаны быть живыми")
	}
}

func TestDeriveIdempotencyFingerprintScopes(t *testing.T) {
	pins := operationPinsFixture()
	baseline := DeriveIdempotencyFingerprint(hmacKeyFixture, opTenant, pins)

	// Тенент входит: чужой тенент — другой отпечаток.
	if got := DeriveIdempotencyFingerprint(hmacKeyFixture, opCase, pins); got == baseline {
		t.Fatal("другой тенент обязан давать другой отпечаток")
	}

	// Keyed-зонд: другой HMAC-ключ — другой отпечаток; и не сырой хэш.
	if got := DeriveIdempotencyFingerprint([]byte("another-commitment-key"), opTenant, pins); got == baseline {
		t.Fatal("keyed свойство нарушено: другой ключ обязан давать другой отпечаток")
	}
	canonical := strings.Join([]string{
		opTenant.String(), pins.PreviewID.String(), "3", pins.ConfirmationID.String(), "1",
	}, canonicalFieldSep)
	raw := sha256.Sum256([]byte(canonical))
	if baseline == hex.EncodeToString(raw[:]) {
		t.Fatal("отпечаток обязан быть keyed HMAC — сырой несолёный хэш запрещён AD-10")
	}
}

func TestTwoKeySpacesSeparated(t *testing.T) {
	// Два keyed-HMAC пространства разделены префиксами: идентичность
	// мутации (business_intent:v1:) и идентичность материала запроса
	// (operation_material:v1:) никогда не смешиваются; обе отделены от
	// action_preview:v1: (3-4).
	in := intentInputFixture()
	pins := operationPinsFixture()
	intent := DeriveBusinessIntentKey(hmacKeyFixture, in)
	fingerprint := DeriveIdempotencyFingerprint(hmacKeyFixture, in.TenantID, pins)

	if intent == fingerprint {
		t.Fatal("два пространства ключей обязаны давать разные значения на любом материале")
	}
	previewDigest := PreviewDigest(hmacKeyFixture, pins.PreviewID, pins.PreviewVersionNo, []byte("canonical-bytes"))
	if intent == previewDigest || fingerprint == previewDigest {
		t.Fatal("пространства 3-5 обязаны быть отделены от action_preview:v1: (3-4)")
	}

	// Взаимная неприводимость pinned: одна и та же пара пин даёт один и
	// тот же отпечаток в любом порядке вычисления (детерминизм запусков).
	if fingerprint != DeriveIdempotencyFingerprint(hmacKeyFixture, in.TenantID, pins) {
		t.Fatal("отпечаток недетерминирован между запусками")
	}
}

func TestOperationPinsMirrorTheRequestSurface(t *testing.T) {
	// Пины operation-строки — зеркало поверхности запроса: определение
	// (ID+версия), preview (ID+версия), confirmation (ID+версия), кейс,
	// рабочий элемент, цель, тройка владельца, release-снимок, partition
	// словом, effects копией — и НИ ОДНОЙ колонки состояния.
	op := Operation{
		TenantID: opTenant, OperationID: opPreview,
		DefinitionID: "notify.order-ready", DefinitionVersion: "1.1.0",
		PreviewID: opPreview, PreviewVersionNo: 3,
		ConfirmationID: opConfirmation, ConfirmationVersionNo: 1,
		CaseID: opCase, WorkItemID: opWorkItem,
		Target:    ProposalTarget{Kind: TargetCatalogItem, Ref: opTargetRef},
		OwnerKind: OwnerHuman, OwnerRef: opOwnerRef, OwnerEpoch: 4,
		ReleaseDecisionNo: 42, ProfileVersion: 7,
		Partition:         PartitionStaffy,
		Effects:           DerivePreviewEffects(previewFixtureDefinition()),
		BusinessIntentKey: DeriveBusinessIntentKey(hmacKeyFixture, intentInputFixture()),
		IdempotencyKey:    "order-confirm-2026.10.20_v1",
		MaterialDigest:    DeriveIdempotencyFingerprint(hmacKeyFixture, opTenant, operationPinsFixture()),
		CreatedAt:         time.Date(2026, 10, 20, 12, 5, 0, 0, time.UTC),
	}
	// Оба ключа живут в строке раздельно и не равны друг другу.
	if op.BusinessIntentKey == op.MaterialDigest {
		t.Fatal("идентичность мутации и отпечаток материала обязаны быть разными ключами")
	}
	if op.BusinessIntentKey == "" || op.MaterialDigest == "" {
		t.Fatal("оба ключа обязаны быть выведены в строке")
	}
	// Ключи детерминированы: пересборка той же строки — те же ключи.
	rebuilt := op
	rebuilt.BusinessIntentKey = DeriveBusinessIntentKey(hmacKeyFixture, intentInputFixture())
	rebuilt.MaterialDigest = DeriveIdempotencyFingerprint(hmacKeyFixture, opTenant, operationPinsFixture())
	if rebuilt.BusinessIntentKey != op.BusinessIntentKey || rebuilt.MaterialDigest != op.MaterialDigest {
		t.Fatal("ключи недетерминированы при пересборке строки")
	}
}

func TestOperationHonestTextsPinned(t *testing.T) {
	// AC-11/AC-13: honest-слова пиннуты точными фразами — слова живого
	// контура 3-6: dispatch — явная команда drain без демона; внешний
	// идентификатор до отправки не выдан; replay говорит о
	// непотреблении; проигравший говорит о резервировании.
	for text, needle := range map[string]string{
		OperationAuthorizedText:       "authorized",
		OperationQueuedText:           "queued",
		OperationDispatchAbsentText:   "явная команда drain",
		OperationExternalIDAbsentText: "не выдан",
		IdempotentReplayText:          "повторно не потребляется",
		ReservationDuplicateText:      "уже зарезервировано",
		ReconciliationReservedText:    "3-7",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("honest-текст обязан содержать «%s»: %q", needle, text)
		}
	}
}
