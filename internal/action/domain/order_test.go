package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// orderFixtureMaterial — валидный материал заказа одного выбора: пины
// кейс-уровня 64-hex, одна позиция с тремя пинами, положительные количество
// и цена, известная сумма и валюта.
func orderFixtureMaterial() OrderCommitmentMaterial {
	return OrderCommitmentMaterial{
		ProfileDigestPin:     orderTestPin("profile"),
		PartRequestDigestPin: orderTestPin("part-request"),
		Positions: []OrderPosition{
			{
				PositionKey:      "pos-1",
				SupplierID:       "supplier-a",
				OfferNo:          1,
				Brand:            "Bosch",
				Article:          "341167866712",
				Originality:      OrderOriginalityOriginal,
				Quantity:         2,
				UnitClientPrice:  4200.50,
				Currency:         "RUB",
				Conditions:       "самовывоз со склада поставщика",
				FitmentDigestPin: orderTestPin("fitment"),
				OfferDigestPin:   orderTestPin("offer"),
				SelectionDigest:  orderTestPin("selection"),
			},
		},
		KnownClientSum: 8401.00,
		Currency:       "RUB",
	}
}

// orderTestPin — детерминированный 64-hex пин для фиксстур.
func orderTestPin(label string) string {
	sum := sha256.Sum256([]byte("staffy-order-test:" + label))
	return hex.EncodeToString(sum[:])
}

// TestGuardAcceptsTheValidMaterial — валидный материал проходит guard.
func TestGuardAcceptsTheValidMaterial(t *testing.T) {
	if err := GuardOrderCommitmentMaterial(orderFixtureMaterial()); err != nil {
		t.Fatalf("валидный материал заказа обязан проходить guard, получено: %v", err)
	}
}

// TestGuardRefusalMatrixByRule — матрица отказов ПОИМЁННО: каждое нарушение
// материала даёт RuleError с точным именем правила (молчаливых исправлений
// нет — ни одно поле не чинится молча).
func TestGuardRefusalMatrixByRule(t *testing.T) {
	pin := orderTestPin("pin")
	base := func() OrderCommitmentMaterial {
		m := orderFixtureMaterial()
		m.Positions = []OrderPosition{
			{
				PositionKey: "pos-1", SupplierID: "supplier-a", OfferNo: 3,
				Brand: "Denckermann", Article: "XHR101300", Originality: OrderOriginalityAnalog,
				Quantity: 1, UnitClientPrice: 1500, Currency: "RUB",
				FitmentDigestPin: pin, OfferDigestPin: pin, SelectionDigest: pin,
			},
		}
		return m
	}
	cases := []struct {
		name   string
		rule   string
		mutate func(*OrderCommitmentMaterial)
	}{
		{"пин-профиля-пуст", "order.profile_digest_pin", func(m *OrderCommitmentMaterial) { m.ProfileDigestPin = "" }},
		{"пин-профиля-не-hex", "order.profile_digest_pin", func(m *OrderCommitmentMaterial) { m.ProfileDigestPin = "zz" + strings.Repeat("a", 62) }},
		{"пин-запроса-пуст", "order.part_request_digest_pin", func(m *OrderCommitmentMaterial) { m.PartRequestDigestPin = "" }},
		{"пин-запроса-короткий", "order.part_request_digest_pin", func(m *OrderCommitmentMaterial) { m.PartRequestDigestPin = strings.Repeat("a", 63) }},
		{"позиций-ноль", "order.positions", func(m *OrderCommitmentMaterial) { m.Positions = nil }},
		{"позиций-девять", "order.positions", func(m *OrderCommitmentMaterial) {
			m.Positions = make([]OrderPosition, OrderPositionsMax+1)
			for i := range m.Positions {
				m.Positions[i] = base().Positions[0]
				m.Positions[i].PositionKey = "pos-" + string(rune('1'+i))
			}
		}},
		{"дубль-ключа-позиции", "order.positions[1].position_key", func(m *OrderCommitmentMaterial) {
			m.Positions = append(m.Positions, m.Positions[0])
		}},
		{"пустой-ключ-позиции", "order.positions[0].position_key", func(m *OrderCommitmentMaterial) { m.Positions[0].PositionKey = "  " }},
		{"пустой-поставщик", "order.positions[0].supplier_id", func(m *OrderCommitmentMaterial) { m.Positions[0].SupplierID = "" }},
		{"номер-предложения-ноль", "order.positions[0].offer_no", func(m *OrderCommitmentMaterial) { m.Positions[0].OfferNo = 0 }},
		{"пустой-бренд", "order.positions[0].brand_article", func(m *OrderCommitmentMaterial) { m.Positions[0].Brand = "" }},
		{"пустой-артикул", "order.positions[0].brand_article", func(m *OrderCommitmentMaterial) { m.Positions[0].Article = "" }},
		{"чужое-слово-оригинальности", "order.positions[0].originality", func(m *OrderCommitmentMaterial) { m.Positions[0].Originality = "originals" }},
		{"количество-ноль", "order.positions[0].quantity", func(m *OrderCommitmentMaterial) { m.Positions[0].Quantity = 0 }},
		{"цена-отрицательная", "order.positions[0].unit_client_price", func(m *OrderCommitmentMaterial) { m.Positions[0].UnitClientPrice = -1 }},
		{"пустая-валюта-позиции", "order.positions[0].currency", func(m *OrderCommitmentMaterial) { m.Positions[0].Currency = "" }},
		{"пин-применимости-невалиден", "order.positions[0].fitment_digest_pin", func(m *OrderCommitmentMaterial) { m.Positions[0].FitmentDigestPin = "short" }},
		{"пин-предложения-невалиден", "order.positions[0].offer_digest_pin", func(m *OrderCommitmentMaterial) { m.Positions[0].OfferDigestPin = "" }},
		{"пин-выбора-невалиден", "order.positions[0].selection_digest_pin", func(m *OrderCommitmentMaterial) { m.Positions[0].SelectionDigest = "ZZ" + strings.Repeat("0", 62) }},
		{"сумма-отрицательная", "order.known_client_sum", func(m *OrderCommitmentMaterial) { m.KnownClientSum = -0.01 }},
		{"валюта-материала-пуста", "order.currency", func(m *OrderCommitmentMaterial) { m.Currency = " " }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := base()
			tc.mutate(&m)
			err := GuardOrderCommitmentMaterial(m)
			if err == nil {
				t.Fatalf("нарушение «%s» обязано отказываться guard'ом", tc.name)
			}
			ruleErr, ok := err.(RuleError)
			if !ok {
				t.Fatalf("отказ обязан быть RuleError, получено %T: %v", err, err)
			}
			if ruleErr.Rule != tc.rule {
				t.Fatalf("отказ «%s» обязан называть правило «%s», названо «%s»", tc.name, tc.rule, ruleErr.Rule)
			}
		})
	}
}

// TestOrderDigestByteStable — дайджест байт-стабилен: один и тот же материал
// (включая перестановку порядка позиций) даёт ОДНО значение; изменение
// ЛЮБОГО из пяти входов меняет дайджест (AC-5: execute-time revalidation
// сверяет одно значение).
func TestOrderDigestByteStable(t *testing.T) {
	m1 := orderFixtureMaterial()
	m2 := orderFixtureMaterial()
	d1, d2 := OrderDigestOf(m1), OrderDigestOf(m2)
	if d1 != d2 {
		t.Fatalf("дайджест обязан быть байт-стабильным: %q != %q", d1, d2)
	}
	if len(d1) != 64 {
		t.Fatalf("дайджест обязан быть 64-hex словом, получено %d символов", len(d1))
	}

	// Порядок позиций не меняет состав: перестановка даёт тот же дайджест.
	m3 := orderFixtureMaterial()
	m3.Positions = []OrderPosition{m3.Positions[0], {
		PositionKey: "pos-0", SupplierID: "supplier-b", OfferNo: 2,
		Brand: "Febi", Article: "0 986 494 029", Originality: OrderOriginalityAnalog,
		Quantity: 1, UnitClientPrice: 999, Currency: "RUB",
		FitmentDigestPin: orderTestPin("fitment2"), OfferDigestPin: orderTestPin("offer2"), SelectionDigest: orderTestPin("selection2"),
	}}
	m4 := OrderCommitmentMaterial{
		ProfileDigestPin:     m3.ProfileDigestPin,
		PartRequestDigestPin: m3.PartRequestDigestPin,
		Positions:            []OrderPosition{m3.Positions[1], m3.Positions[0]},
		KnownClientSum:       m3.KnownClientSum,
		Currency:             m3.Currency,
	}
	if OrderDigestOf(m3) != OrderDigestOf(m4) {
		t.Fatal("перестановка порядка позиций не меняет канонический состав — дайджест обязан совпасть")
	}

	// Изменение каждого из пяти входов меняет дайджест.
	changed := []struct {
		name   string
		mutate func(*OrderCommitmentMaterial)
	}{
		{"профиль", func(m *OrderCommitmentMaterial) { m.ProfileDigestPin = orderTestPin("profile-new") }},
		{"запрос", func(m *OrderCommitmentMaterial) { m.PartRequestDigestPin = orderTestPin("part-request-new") }},
		{"применимость", func(m *OrderCommitmentMaterial) { m.Positions[0].FitmentDigestPin = orderTestPin("fitment-new") }},
		{"предложение", func(m *OrderCommitmentMaterial) { m.Positions[0].OfferDigestPin = orderTestPin("offer-new") }},
		{"выбор", func(m *OrderCommitmentMaterial) { m.Positions[0].SelectionDigest = orderTestPin("selection-new") }},
		{"количество", func(m *OrderCommitmentMaterial) { m.Positions[0].Quantity = 3 }},
		{"цена", func(m *OrderCommitmentMaterial) { m.Positions[0].UnitClientPrice = 4200.51 }},
	}
	for _, tc := range changed {
		t.Run(tc.name, func(t *testing.T) {
			m := orderFixtureMaterial()
			tc.mutate(&m)
			if OrderDigestOf(m) == d1 {
				t.Fatalf("изменение «%s» обязано менять дайджест — revalidation не слепа", tc.name)
			}
		})
	}
}

// TestOrderDigestJSONIdempotent — канонический JSON идемпотентен: повторная
// сборка канонической формы (маршал ×2) даёт те же байты и тот же дайджест.
func TestOrderDigestJSONIdempotent(t *testing.T) {
	m := orderFixtureMaterial()
	first, second := OrderDigestOf(m), OrderDigestOf(m)
	if first != second {
		t.Fatalf("повторная деривация обязана давать тот же дайджест: %q != %q", first, second)
	}
	var b strings.Builder
	b.WriteString(first)
	if b.String() != OrderDigestOf(m) {
		t.Fatal("дайджест стабилен и при третьем прогоне")
	}
}

// TestOrderMaterialCanonicalTextMandatoryWords — канонический рендер несёт
// ОБЯЗАТЕЛЬНОЕ слово менеджерской проверки (AC-1), слово границы суммы и
// детерминирован (позиции по возрастанию ключа).
func TestOrderMaterialCanonicalTextMandatoryWords(t *testing.T) {
	m := orderFixtureMaterial()
	text := OrderMaterialCanonicalText(m)
	if !strings.Contains(text, OrderManagerCheckWord) {
		t.Fatalf("рендер материала обязан нести обязательное слово менеджерской проверки, получено:\n%s", text)
	}
	if !strings.Contains(text, OrderSumBoundaryWord) {
		t.Fatalf("рендер материала обязан нести слово границы суммы, получено:\n%s", text)
	}
	if !strings.Contains(text, "Bosch / 341167866712 (оригинал)") {
		t.Fatalf("рендер материала обязан нести позицию с брендом/артикулом и словом оригинальности, получено:\n%s", text)
	}
	// Два материала с разными пинами не дают одинаковый рендер.
	other := orderFixtureMaterial()
	other.Positions[0].Quantity = 5
	if OrderMaterialCanonicalText(other) == text {
		t.Fatal("изменение материала обязано менять канонический рендер")
	}
	// Порядок позиций в рендере детерминирован: сортировка по ключу.
	mMulti := orderFixtureMaterial()
	mMulti.Positions = append(mMulti.Positions, OrderPosition{
		PositionKey: "pos-0", SupplierID: "supplier-b", OfferNo: 2,
		Brand: "Febi", Article: "0 986 494 029", Originality: OrderOriginalityAnalog,
		Quantity: 1, UnitClientPrice: 999, Currency: "RUB",
		FitmentDigestPin: orderTestPin("f2"), OfferDigestPin: orderTestPin("o2"), SelectionDigest: orderTestPin("s2"),
	})
	text1 := OrderMaterialCanonicalText(mMulti)
	mSwapped := OrderCommitmentMaterial{
		ProfileDigestPin: mMulti.ProfileDigestPin, PartRequestDigestPin: mMulti.PartRequestDigestPin,
		Positions:      []OrderPosition{mMulti.Positions[1], mMulti.Positions[0]},
		KnownClientSum: mMulti.KnownClientSum, Currency: mMulti.Currency,
	}
	if text1 != OrderMaterialCanonicalText(mSwapped) {
		t.Fatal("рендер материала обязан быть детерминированным: перестановка позиций не меняет канонический рендер")
	}
	if strings.Index(text1, "позиция pos-0") > strings.Index(text1, "позиция pos-1") {
		t.Fatalf("позиции рендера обязаны идти по возрастанию ключа:\n%s", text1)
	}
}

// TestOrderHonestWordsCarryNoSupplierSendClaims — зонды честных слов:
// клиентские слова НЕ утверждают отправку/выкуп/гарантию поставщиком —
// запретные слова встречаются ТОЛЬКО с отрицанием «не », и слово
// менеджерской проверки всегда утверждает забор.
func TestOrderHonestWordsCarryNoSupplierSendClaims(t *testing.T) {
	clientWords := []string{OrderClientCreatedWord, OrderClientBoundaryWord, OrderManagerCheckWord, OrderOneCUnknownWord}
	forbidden := []string{"заказан у поставщика", "выкуплен", "отправлен", "гарантирован", "отправляется поставщику"}
	for _, word := range clientWords {
		for _, claim := range forbidden {
			idx := strings.Index(word, claim)
			if idx < 0 {
				continue
			}
			// Утверждение допустимо только с отрицанием перед ним.
			prefix := word[:idx]
			if !strings.HasSuffix(prefix, "не ") && !strings.HasSuffix(prefix, "не ") {
				t.Fatalf("слово «%s» содержит запретное утверждение «%s» без отрицания", word, claim)
			}
		}
	}
	// Слово менеджерской проверки — константа материала: обязательное
	// утверждение забора (AC-1).
	if !strings.Contains(OrderManagerCheckWord, "проверен менеджером") {
		t.Fatalf("слово менеджерской проверки обязано называть менеджерскую проверку: %q", OrderManagerCheckWord)
	}
	if !strings.Contains(OrderManagerCheckWord, "не отправляется поставщику") {
		t.Fatalf("слово менеджерской проверки обязано нести границу отправки: %q", OrderManagerCheckWord)
	}
}

// TestOrderHonestWordsCarryNoMarketingClaims — зонд маркетинговых слов:
// честные слова среза не содержат рекламных заявлений.
func TestOrderHonestWordsCarryNoMarketingClaims(t *testing.T) {
	allWords := strings.Join([]string{
		OrderManagerCheckWord, OrderManagerAssignmentWord, OrderClientCreatedWord,
		OrderClientBoundaryWord, OrderOneCUnknownWord, OrderSumBoundaryWord,
		OrderCompositionChangedWord, OrderVersionSourceMissingWord, OrderPositionGuardWord,
		OrderNoSelectionWord, OrderOriginalityRenderWord(OrderOriginalityOriginal),
		OrderOriginalityRenderWord(OrderOriginalityAnalog),
	}, "\n")
	marketing := []string{"лучший", "выгодн", "скидк", "акци", "самый", "гарантируем", "бесплатн", "успейте", "премиум-качеств"}
	for _, word := range marketing {
		if strings.Contains(strings.ToLower(allWords), strings.ToLower(word)) {
			t.Fatalf("честные слова заказа не содержат маркетинговых заявлений, найдено «%s»", word)
		}
	}
}

// TestOrderReceiptStateDictionary — словарь производных состояний квитанции
// несёт слово pending_manager_verification с клиентским словом.
func TestOrderReceiptStateDictionary(t *testing.T) {
	word, ok := OrderReceiptStateWords[OrderReceiptStatePendingManagerVerification]
	if !ok {
		t.Fatalf("словарь состояний обязан нести слово %q", OrderReceiptStatePendingManagerVerification)
	}
	if word != OrderClientCreatedWord {
		t.Fatalf("слово состояния %q обязано быть клиентским словом «%s», получено %q",
			OrderReceiptStatePendingManagerVerification, OrderClientCreatedWord, word)
	}
}
