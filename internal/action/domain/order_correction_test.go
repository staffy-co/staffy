package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Story 4.8 (step-01) — the order correction domain tests: the
// named-matrix guards, the read-time state derivations (the lifecycle,
// the overdue clock honesty, the reassignment epoch, the closure
// outcomes), the recommended-explanation diff (the explicit per-field
// former → current words), the byte-stability of the frozen words and
// the honest probes of the slice (the mandatory manual-contour word,
// the zero supplier-send words, the «контур 4-10» recommendation, the
// «прежде не отправлялся» boundary).

func correctionFixture() domain.OrderCorrection {
	return domain.OrderCorrection{
		TenantID:         id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a401"),
		CaseID:           id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a402"),
		WorkItemID:       id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a403"),
		CorrectionNo:     1,
		OperationID:      id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a404"),
		VerificationNo:   1,
		ReceiptVersionNo: 1,
		ExternalOrderID:  "ABCP-ORDER-77",
		CreatorID:        id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a405"),
		CreatedAt:        time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC),
	}
}

func correctionOpenedEvent() domain.OrderCorrectionEvent {
	base := time.Date(2026, 10, 25, 9, 5, 0, 0, time.UTC)
	return domain.OrderCorrectionEvent{
		TenantID:        correctionFixture().TenantID,
		CaseID:          correctionFixture().CaseID,
		CorrectionNo:    1,
		EventNo:         1,
		Kind:            domain.OrderCorrectionEventOpened,
		Owner:           id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a406"),
		CorrectDeadline: domain.Correct4h,
		DeadlineAt:      base.Add(4 * time.Hour),
		Positions:       []string{"WF2B196K666AA"},
		Reason:          domain.VerifyBlockReasonWrongPart,
		ActorID:         correctionFixture().CreatorID,
		CreatedAt:       base,
	}
}

// TestCorrectionDeadlineDictionary pins the two-word approved policy
// dictionary with its honest texts and windows (the word is gated, never
// parsed).
func TestCorrectionDeadlineDictionary(t *testing.T) {
	if got := domain.CorrectionDeadlines; strings.Join(got, ",") != "correct_4h,correct_1d" {
		t.Fatalf("словарь дедлайнов коррекции: %v", got)
	}
	for _, word := range domain.CorrectionDeadlines {
		if text := domain.CorrectionDeadlineText(word); strings.TrimSpace(text) == "" {
			t.Fatalf("дедлайн %q без текста", word)
		}
	}
	if domain.CorrectionDeadlineWindow(domain.Correct4h) != 4*time.Hour {
		t.Fatalf("окно correct_4h не 4 часа")
	}
	if domain.CorrectionDeadlineWindow(domain.Correct1d) != 24*time.Hour {
		t.Fatalf("окно correct_1d не 24 часа")
	}
	if domain.CorrectionDeadlineInDictionary("verify_4h") {
		t.Fatalf("слово проверки 4-7 не входит в словарь коррекции (контуры различаются именами)")
	}
	if domain.CorrectionDeadlineInDictionary("correct_2h") {
		t.Fatalf("выдуманное слово входит в словарь дедлайнов коррекции")
	}
}

// TestCorrectionCessationReasonDictionary pins the three-word cessation
// dictionary with its honest texts (the structured reason — never a free
// input).
func TestCorrectionCessationReasonDictionary(t *testing.T) {
	if got := domain.CorrectionCessationReasons; strings.Join(got, ",") != "client_refused,not_needed,abandoned" {
		t.Fatalf("словарь причин прекращения: %v", got)
	}
	for _, word := range domain.CorrectionCessationReasons {
		if text := domain.CorrectionCessationReasonText(word); strings.TrimSpace(text) == "" {
			t.Fatalf("причина %q без текста", word)
		}
	}
	if domain.CorrectionCessationReasonInDictionary("client_agreed") {
		t.Fatalf("выдуманное слово входит в словарь причин прекращения")
	}
}

// TestGuardOrderCorrectionOpenByName pins the open guard branches: the
// owner reference and the deadline dictionary word.
func TestGuardOrderCorrectionOpenByName(t *testing.T) {
	owner := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a406")
	if err := domain.GuardOrderCorrectionOpen(owner, domain.Correct1d); err != nil {
		t.Fatalf("валидное открытие отклонено: %v", err)
	}
	if err := domain.GuardOrderCorrectionOpen(id.UUID(""), domain.Correct1d); err == nil {
		t.Fatalf("пустой owner принят")
	}
	if err := domain.GuardOrderCorrectionOpen(owner, "correct_2h"); err == nil {
		t.Fatalf("чужое слово дедлайна принято")
	}
}

// TestGuardOrderCorrectionReassignByName pins the reassign guard
// branches: the open state, the owner reference and the deadline word.
func TestGuardOrderCorrectionReassignByName(t *testing.T) {
	owner := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a406")
	if err := domain.GuardOrderCorrectionReassign(domain.OrderCorrectionStatePending, owner, domain.Correct4h); err != nil {
		t.Fatalf("валидное переназначение отклонено: %v", err)
	}
	if err := domain.GuardOrderCorrectionReassign(domain.OrderCorrectionStateOverdue, owner, domain.Correct4h); err != nil {
		t.Fatalf("переназначение просроченной коррекции отклонено (путь владельца — 4-7 прецедент): %v", err)
	}
	if err := domain.GuardOrderCorrectionReassign(domain.OrderCorrectionStateCorrected, owner, domain.Correct4h); err == nil {
		t.Fatalf("переназначение закрытой коррекции принято")
	}
	if err := domain.GuardOrderCorrectionReassign(domain.OrderCorrectionStatePending, id.UUID(""), domain.Correct4h); err == nil {
		t.Fatalf("пустой owner принят")
	}
}

// TestGuardOrderCorrectionRereadByName pins the reread guard branches:
// the open pending state (overdue refuses), the actor=owner server
// truth, the CAS epoch, the external object ID bounds and the complete
// NEW confirmed version identity triple.
func TestGuardOrderCorrectionRereadByName(t *testing.T) {
	owner := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a406")
	other := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a409")
	proposal := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a410")
	confirmation := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a411")
	if err := domain.GuardOrderCorrectionReread(domain.OrderCorrectionStatePending, owner, owner, 1, "ABCP-ORDER-78", proposal, 2, confirmation); err != nil {
		t.Fatalf("валидный перечит отклонён: %v", err)
	}
	if err := domain.GuardOrderCorrectionReread(domain.OrderCorrectionStateOverdue, owner, owner, 1, "ABCP-ORDER-78", proposal, 2, confirmation); err == nil {
		t.Fatalf("перечит просроченной коррекции принят (путь — переназначение)")
	}
	if err := domain.GuardOrderCorrectionReread(domain.OrderCorrectionStateCeased, owner, owner, 1, "ABCP-ORDER-78", proposal, 2, confirmation); err == nil {
		t.Fatalf("перечит закрытой коррекции принят")
	}
	if err := domain.GuardOrderCorrectionReread(domain.OrderCorrectionStatePending, other, owner, 1, "ABCP-ORDER-78", proposal, 2, confirmation); err == nil {
		t.Fatalf("не-owner актёр принят")
	} else if !strings.Contains(err.Error(), domain.OrderCorrectionNotOwnerWord) {
		t.Fatalf("отказ не-owner без слова истины актёра: %v", err)
	}
	if err := domain.GuardOrderCorrectionReread(domain.OrderCorrectionStatePending, owner, owner, 0, "ABCP-ORDER-78", proposal, 2, confirmation); err == nil {
		t.Fatalf("нулевая эпоха CAS принята")
	}
	if err := domain.GuardOrderCorrectionReread(domain.OrderCorrectionStatePending, owner, owner, 1, "", proposal, 2, confirmation); err == nil {
		t.Fatalf("пустой внешний идентификатор принят")
	}
	if err := domain.GuardOrderCorrectionReread(domain.OrderCorrectionStatePending, owner, owner, 1, strings.Repeat("x", 201), proposal, 2, confirmation); err == nil {
		t.Fatalf("внешний идентификатор свыше границы принят")
	}
	if err := domain.GuardOrderCorrectionReread(domain.OrderCorrectionStatePending, owner, owner, 1, "ABCP-ORDER-78", id.UUID(""), 2, confirmation); err == nil {
		t.Fatalf("пустой пин предложения принят")
	}
	if err := domain.GuardOrderCorrectionReread(domain.OrderCorrectionStatePending, owner, owner, 1, "ABCP-ORDER-78", proposal, 0, confirmation); err == nil {
		t.Fatalf("нулевая версия принята")
	}
	if err := domain.GuardOrderCorrectionReread(domain.OrderCorrectionStatePending, owner, owner, 1, "ABCP-ORDER-78", proposal, 2, id.UUID("")); err == nil {
		t.Fatalf("пустой пин подтверждения принят")
	}
}

// TestGuardOrderCorrectionCeaseByName pins the cease guard branches: the
// open pending state (overdue refuses), the actor=owner truth, the CAS
// epoch and the fixed cessation dictionary.
func TestGuardOrderCorrectionCeaseByName(t *testing.T) {
	owner := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a406")
	if err := domain.GuardOrderCorrectionCease(domain.OrderCorrectionStatePending, owner, owner, 1, domain.CorrectionCessationClientRefused); err != nil {
		t.Fatalf("валидное прекращение отклонено: %v", err)
	}
	if err := domain.GuardOrderCorrectionCease(domain.OrderCorrectionStateOverdue, owner, owner, 1, domain.CorrectionCessationClientRefused); err == nil {
		t.Fatalf("прекращение просроченной коррекции принято (путь — переназначение)")
	}
	if err := domain.GuardOrderCorrectionCease(domain.OrderCorrectionStatePending, owner, owner, 1, "client_agreed"); err == nil {
		t.Fatalf("чужое слово причины принято")
	}
	if err := domain.GuardOrderCorrectionCease(domain.OrderCorrectionStatePending, owner, owner, 0, domain.CorrectionCessationNotNeeded); err == nil {
		t.Fatalf("нулевая эпоха CAS принята")
	}
	other := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a409")
	if err := domain.GuardOrderCorrectionCease(domain.OrderCorrectionStatePending, other, owner, 1, domain.CorrectionCessationAbandoned); err == nil {
		t.Fatalf("не-owner актёр принят")
	}
}

// TestDeriveOrderCorrectionStateLifecycle pins the full read-time
// lifecycle: opened → pending (with the round material and the epoch),
// reread → corrected (the closure material with the NEW confirmed
// version pins), ceased → ceased (the closure material with the
// cessation reason).
func TestDeriveOrderCorrectionStateLifecycle(t *testing.T) {
	opened := correctionOpenedEvent()
	now := opened.CreatedAt.Add(10 * time.Minute)
	view := domain.DeriveOrderCorrectionState(correctionFixture(), []domain.OrderCorrectionEvent{opened}, now)
	if view.State != domain.OrderCorrectionStatePending {
		t.Fatalf("состояние после opened: %q, ожидалось pending_correction", view.State)
	}
	if view.Owner != opened.Owner {
		t.Fatalf("владелец раунда не из opened события")
	}
	if view.CorrectDeadline != domain.Correct4h {
		t.Fatalf("слово дедлайна раунда не из opened события")
	}
	if view.AssignmentEpoch != 1 {
		t.Fatalf("эпоха назначения не event_no opened: %d", view.AssignmentEpoch)
	}
	if view.Active() != true {
		t.Fatalf("открытая коррекция не активна")
	}
	if view.Outcome() != "" {
		t.Fatalf("у открытой коррекции есть исход: %q", view.Outcome())
	}

	reread := domain.OrderCorrectionEvent{
		TenantID:         opened.TenantID,
		CaseID:           opened.CaseID,
		CorrectionNo:     1,
		EventNo:          2,
		Kind:             domain.OrderCorrectionEventReread,
		ExternalObjectID: "ABCP-ORDER-78",
		ProposalID:       id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a410"),
		VersionNo:        2,
		ConfirmationID:   id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a411"),
		ActorID:          opened.Owner,
		CreatedAt:        opened.CreatedAt.Add(30 * time.Minute),
	}
	view = domain.DeriveOrderCorrectionState(correctionFixture(), []domain.OrderCorrectionEvent{opened, reread}, reread.CreatedAt)
	if view.State != domain.OrderCorrectionStateCorrected {
		t.Fatalf("состояние после reread: %q, ожидалось corrected", view.State)
	}
	if view.Active() {
		t.Fatalf("исправленная коррекция активна")
	}
	if view.Outcome() != domain.OrderCorrectionOutcomeCorrected {
		t.Fatalf("исход reread: %q, ожидался corrected", view.Outcome())
	}
	if view.Closure == nil || view.Closure.ExternalObjectID != "ABCP-ORDER-78" {
		t.Fatalf("материал закрытия (перечит) не выведен")
	}

	ceased := domain.OrderCorrectionEvent{
		TenantID:        opened.TenantID,
		CaseID:          opened.CaseID,
		CorrectionNo:    1,
		EventNo:         2,
		Kind:            domain.OrderCorrectionEventCeased,
		CessationReason: domain.CorrectionCessationClientRefused,
		ActorID:         opened.Owner,
		CreatedAt:       opened.CreatedAt.Add(30 * time.Minute),
	}
	view = domain.DeriveOrderCorrectionState(correctionFixture(), []domain.OrderCorrectionEvent{opened, ceased}, ceased.CreatedAt)
	if view.State != domain.OrderCorrectionStateCeased {
		t.Fatalf("состояние после ceased: %q, ожидалось ceased", view.State)
	}
	if view.Outcome() != domain.OrderCorrectionOutcomeCeased {
		t.Fatalf("исход ceased: %q, ожидался ceased", view.Outcome())
	}
}

// TestDeriveOrderCorrectionStateOverdue pins the honest overdue clock:
// the open round after the deadline derives overdue (read-time, never a
// record); the closure events stay closed forever; the reassignment
// with a NEW epoch overrides the former round.
func TestDeriveOrderCorrectionStateOverdue(t *testing.T) {
	opened := correctionOpenedEvent()
	before := opened.DeadlineAt.Add(-time.Minute)
	after := opened.DeadlineAt.Add(time.Minute)
	view := domain.DeriveOrderCorrectionState(correctionFixture(), []domain.OrderCorrectionEvent{opened}, before)
	if view.State != domain.OrderCorrectionStatePending {
		t.Fatalf("до дедлайна состояние: %q", view.State)
	}
	view = domain.DeriveOrderCorrectionState(correctionFixture(), []domain.OrderCorrectionEvent{opened}, after)
	if view.State != domain.OrderCorrectionStateOverdue {
		t.Fatalf("после дедлайна состояние: %q, ожидалось overdue", view.State)
	}
	if !view.Active() {
		t.Fatalf("просроченная коррекция не активна (переназначение живо)")
	}

	reassigned := domain.OrderCorrectionEvent{
		TenantID:        opened.TenantID,
		CaseID:          opened.CaseID,
		CorrectionNo:    1,
		EventNo:         2,
		Kind:            domain.OrderCorrectionEventReassigned,
		Owner:           id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a412"),
		CorrectDeadline: domain.Correct1d,
		DeadlineAt:      opened.CreatedAt.Add(24 * time.Hour),
		ActorID:         opened.Owner,
		CreatedAt:       after,
	}
	view = domain.DeriveOrderCorrectionState(correctionFixture(), []domain.OrderCorrectionEvent{opened, reassigned}, after.Add(time.Minute))
	if view.State != domain.OrderCorrectionStatePending {
		t.Fatalf("после переназначения состояние: %q", view.State)
	}
	if view.Owner != reassigned.Owner {
		t.Fatalf("владелец не из reassigned события")
	}
	if view.AssignmentEpoch != 2 {
		t.Fatalf("эпоха после переназначения: %d, ожидалась 2", view.AssignmentEpoch)
	}

	closed := domain.OrderCorrectionEvent{
		TenantID:         opened.TenantID,
		CaseID:           opened.CaseID,
		CorrectionNo:     1,
		EventNo:          3,
		Kind:             domain.OrderCorrectionEventReread,
		ExternalObjectID: "ABCP-ORDER-78",
		ProposalID:       id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a410"),
		VersionNo:        2,
		ConfirmationID:   id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a411"),
		ActorID:          reassigned.Owner,
		CreatedAt:        after.Add(10 * time.Minute),
	}
	view = domain.DeriveOrderCorrectionState(correctionFixture(), []domain.OrderCorrectionEvent{opened, reassigned, closed}, closed.CreatedAt.Add(time.Hour))
	if view.State != domain.OrderCorrectionStateCorrected {
		t.Fatalf("закрытая коррекция изменила состояние со временем: %q", view.State)
	}
}

// TestDeriveOrderCorrectionDiff pins the recommended-explanation diff:
// the per-position explicit field differences (the article, the
// originality, the quantity, the price, the conditions), the
// added/removed positions and the case-level known-sum difference — the
// equal fields never appear; the Changed flag rides the union.
func TestDeriveOrderCorrectionDiff(t *testing.T) {
	former := domain.OrderCommitmentMaterial{
		ProfileDigestPin:     strings.Repeat("a", 64),
		PartRequestDigestPin: strings.Repeat("b", 64),
		Positions: []domain.OrderPosition{
			{
				PositionKey: "WF2B196K666AA", SupplierID: "supplier-1", OfferNo: 1,
				Brand: "Bosch", Article: "0986224491", Originality: domain.OrderOriginalityOriginal,
				Quantity: 2, UnitClientPrice: 1000, Currency: "RUB", Conditions: "2 дня",
			},
			{
				PositionKey: "NOS1170", SupplierID: "supplier-2", OfferNo: 2,
				Brand: "Denso", Article: "DCF010", Originality: domain.OrderOriginalityAnalog,
				Quantity: 1, UnitClientPrice: 500, Currency: "RUB", Conditions: "5 дней",
			},
		},
		KnownClientSum: 2500,
		Currency:       "RUB",
	}
	current := domain.OrderCommitmentMaterial{
		ProfileDigestPin:     strings.Repeat("a", 64),
		PartRequestDigestPin: strings.Repeat("b", 64),
		Positions: []domain.OrderPosition{
			{
				PositionKey: "WF2B196K666AA", SupplierID: "supplier-1", OfferNo: 1,
				Brand: "Bosch", Article: "0986224492", Originality: domain.OrderOriginalityAnalog,
				Quantity: 2, UnitClientPrice: 1100, Currency: "RUB", Conditions: "3 дня",
			},
		},
		KnownClientSum: 2200,
		Currency:       "RUB",
	}
	diff := domain.DeriveOrderCorrectionDiff(former, current)
	if !diff.Changed() {
		t.Fatalf("дифф не видит изменений")
	}
	if !diff.KnownSumChanged {
		t.Fatalf("дифф не видит изменения суммы")
	}
	var changedPosition *domain.OrderCorrectionPositionDiff
	for i := range diff.Positions {
		if diff.Positions[i].PositionKey == "WF2B196K666AA" {
			changedPosition = &diff.Positions[i]
		}
	}
	if changedPosition == nil {
		t.Fatalf("изменённая позиция не вошла в дифф")
	}
	fields := map[string]domain.OrderCorrectionFieldDiff{}
	for _, field := range changedPosition.Fields {
		fields[field.Field] = field
	}
	if len(fields) != 4 {
		t.Fatalf("изменённых полей: %d, ожидалось 4 (артикул/оригинал/цена/срок)", len(fields))
	}
	if article, ok := fields["артикул"]; !ok || article.Former != "Bosch 0986224491" || article.Current != "Bosch 0986224492" {
		t.Fatalf("дифф артикула неверен: %+v", article)
	}
	if originality, ok := fields["оригинал/аналог"]; !ok || originality.Former == originality.Current {
		t.Fatalf("дифф оригинальности неверен: %+v", originality)
	}
	if price, ok := fields["цена за единицу"]; !ok || price.Former != "1000.00 RUB" || price.Current != "1100.00 RUB" {
		t.Fatalf("дифф цены неверен: %+v", price)
	}
	if conditions, ok := fields["срок и условия"]; !ok || conditions.Former != "2 дня" || conditions.Current != "3 дня" {
		t.Fatalf("дифф срока неверен: %+v", conditions)
	}
	if _, ok := fields["количество"]; ok {
		t.Fatalf("неизменённое количество попало в дифф")
	}
	var removedPosition *domain.OrderCorrectionPositionDiff
	for i := range diff.Positions {
		if diff.Positions[i].PositionKey == "NOS1170" {
			removedPosition = &diff.Positions[i]
		}
	}
	if removedPosition == nil || removedPosition.InFormer != true || removedPosition.InCurrent != false {
		t.Fatalf("удалённая позиция не выведена диффом")
	}

	// Полное равенство — дифф пуст.
	diff = domain.DeriveOrderCorrectionDiff(former, former)
	if diff.Changed() {
		t.Fatalf("дифф видит изменения при равных материалах")
	}
	if len(diff.Positions) != len(former.Positions) {
		t.Fatalf("дифф не перечисляет позиции при равных материалах (материал рендера)")
	}
	for i := range diff.Positions {
		if len(diff.Positions[i].Fields) != 0 {
			t.Fatalf("равная позиция несёт различия")
		}
	}
}

// TestOrderCorrectionWordsByteStability pins the frozen words of the
// correction contour (the byte-stability discipline — a word change is
// a deliberate act, never an accident).
func TestOrderCorrectionWordsByteStability(t *testing.T) {
	pins := map[string]string{
		domain.OrderCorrectionPendingWord:            "коррекция открыта: заказ заблокирован от продвижения до исправления и новой проверки",
		domain.OrderCorrectionCorrectedWord:          "исправлен: заказ возвращается на проверку менеджера (pending_manager_verification); новая задача проверки — решением владельца",
		domain.OrderCorrectionCeasedWord:             "коррекция прекращена без замены: заказ остаётся заблокированным от продвижения — решение о дальнейших шагах за владельцем",
		domain.OrderCorrectionOverdueWord:            "просрочен: дедлайн коррекции истёк без закрытия — автоматических действий нет; следующий шаг — переназначение владельцем",
		domain.OrderCorrectionNotStartedWord:         "назначение коррекции ≠ начало работы: контакт и начало не фиксируются ничем, кроме событий",
		domain.OrderCorrectionManualContourWord:      "изменение существующего заказа ABCP выполняется вручную в контуре магазина; Staffy не изменяет и не создаёт новый заказ автоматически",
		domain.OrderCorrectionRereadWord:             "перечит подтверждён словами менеджера: Staffy не читает ABCP (AG-4) — состав сверён с подтверждённой клиентом версией",
		domain.OrderCorrectionNewConfirmationWord:    "требуется новое подтверждение клиента: прежнее подтверждение не переносится",
		domain.OrderCorrectionNotSentWord:            "прежний вариант поставщику не отправлялся",
		domain.OrderCorrectionCancellationWord:       "отмена заказа — ручной контур менеджера (контур 4-10): согласие клиента не предполагается — замены без нового подтверждения не существует",
		domain.OrderCorrectionChannelWord:            "канал связи с клиентом — ручной контур магазина (Jivo/телефония): доставка сообщения не фиксируется этим контуром",
		domain.OrderCorrectionAssignGateWord:         "коррекция заказа открыта — проверка вернётся после исправления",
		domain.OrderCorrectionOrderGateWord:          "коррекция заказа открыта: изменение существующего заказа выполняется вручную в контуре магазина; новый заказ не создаётся",
		domain.OrderCorrectionNoBlockedWord:          "коррекция открывается по решению проверки «Нужна коррекция» — заблокированного решения нет",
		domain.OrderCorrectionActiveText:             "коррекция заказа уже открыта — дождитесь исхода либо переназначьте: вторая конкурирующая коррекция не создаётся",
		domain.OrderCorrectionReassignedWord:         "коррекция переназначена: ожидаемая эпоха устарела — перечитайте коррекцию и повторите команду",
		domain.OrderCorrectionNotOwnerWord:           "записать перечит или прекратить коррекцию может только владелец коррекции: назначение не передаёт владение другим участникам",
		domain.OrderCorrectionSupersededWord:         "подтверждение клиента устарело (superseded) — требуется новое подтверждение",
		domain.OrderCorrectionCompositionChangedWord: "состав изменился после подтверждения клиента — требуется обновлённое подтверждение",
		domain.OrderCorrectionPurchasingHiddenWord:   "закупочные данные — внутренние данные магазина; без полномочия не раскрываются",
	}
	for word, want := range pins {
		if word != want {
			t.Fatalf("замороженное слово изменилось:\nесть:    %q\nожидалось: %q", word, want)
		}
	}
}

// TestOrderCorrectionStateWords pins the four derived state words, the
// open/closed split and the outcome dictionary (the states are words,
// never colors).
func TestOrderCorrectionStateWords(t *testing.T) {
	want := []string{"pending_correction", "overdue", "corrected", "ceased"}
	if got := domain.OrderCorrectionStates; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("словарь состояний коррекции: %v, ожидалось %v", got, want)
	}
	for _, word := range domain.OrderCorrectionStates {
		if text := domain.OrderCorrectionStateText(word); strings.TrimSpace(text) == "" {
			t.Fatalf("состояние %q без текста", word)
		}
	}
	if domain.OrderCorrectionStateActive(domain.OrderCorrectionStatePending) != true {
		t.Fatalf("pending_correction не открытое состояние")
	}
	if domain.OrderCorrectionStateActive(domain.OrderCorrectionStateOverdue) != true {
		t.Fatalf("overdue — открытое состояние (переназначение живо)")
	}
	if domain.OrderCorrectionStateActive(domain.OrderCorrectionStateCorrected) {
		t.Fatalf("corrected не открытое состояние")
	}
	if got := domain.OrderCorrectionOutcomes; strings.Join(got, ",") != "corrected,ceased" {
		t.Fatalf("словарь исходов: %v", got)
	}
}

// TestOrderCorrectionManualContourProbe pins the MANDATORY manual-contour
// word of the ABCP change honesty (AG-4/AC-10): the word is a constant
// of the domain — the transport render and the journeys pin its
// presence; the change/send honest negations ride THIS word, never a
// change or send claim.
func TestOrderCorrectionManualContourProbe(t *testing.T) {
	for _, needle := range []string{"вручную в контуре магазина", "Staffy не изменяет", "не создаёт новый заказ автоматически"} {
		if !strings.Contains(domain.OrderCorrectionManualContourWord, needle) {
			t.Fatalf("обязательное слово ручного контура потеряло часть %q: %q", needle, domain.OrderCorrectionManualContourWord)
		}
	}
}

// TestOrderCorrectionSupplierSendProbe pins the structural supplier-send
// fence of the domain words (OQ-3): no dictionary word of the correction
// contour CLAIMS a supplier send — the only words mentioning the send
// are the honest NEGATIONS (the «прежде не отправлялся» boundary).
func TestOrderCorrectionSupplierSendProbe(t *testing.T) {
	words := []string{
		domain.OrderCorrectionPendingWord, domain.OrderCorrectionCorrectedWord,
		domain.OrderCorrectionCeasedWord, domain.OrderCorrectionOverdueWord,
		domain.OrderCorrectionNotStartedWord, domain.OrderCorrectionManualContourWord,
		domain.OrderCorrectionRereadWord, domain.OrderCorrectionNewConfirmationWord,
		domain.OrderCorrectionNotSentWord, domain.OrderCorrectionCancellationWord,
		domain.OrderCorrectionChannelWord, domain.OrderCorrectionAssignGateWord,
		domain.OrderCorrectionOrderGateWord, domain.OrderCorrectionNoBlockedWord,
		domain.OrderCorrectionActiveText, domain.OrderCorrectionReassignedWord,
		domain.OrderCorrectionNotOwnerWord, domain.OrderCorrectionSupersededWord,
		domain.OrderCorrectionCompositionChangedWord, domain.OrderCorrectionPurchasingHiddenWord,
	}
	for _, word := range words {
		lower := strings.ToLower(word)
		if strings.Contains(lower, "отправлен поставщику") || strings.Contains(lower, "отправка выполнена") ||
			strings.Contains(lower, "отправлено поставщику") || strings.Contains(lower, "успешно отправл") {
			t.Fatalf("слово домена коррекции заявляет отправку поставщику (зонд OQ-3): %q", word)
		}
		if strings.Contains(lower, "отправля") {
			// The send-mentioning words must be the honest negations.
			if !strings.Contains(lower, "не отправля") {
				t.Fatalf("слово упоминает отправку без честного отрицания (зонд OQ-3): %q", word)
			}
		}
	}
}

// TestOrderCorrectionContourProbes pins the honest contour boundary
// words: the cancellation recommendation names the contour 4-10 (AC-11),
// the former-variant boundary carries the «прежде не отправлялся»
// honesty (AC-6) and the channel boundary speaks the manual contour
// (AC-5, OQ-1/OQ-2).
func TestOrderCorrectionContourProbes(t *testing.T) {
	if !strings.Contains(domain.OrderCorrectionCancellationWord, "контур 4-10") {
		t.Fatalf("слово отказа клиента не называет контур 4-10: %q", domain.OrderCorrectionCancellationWord)
	}
	if !strings.Contains(domain.OrderCorrectionNotSentWord, "не отправлялся") {
		t.Fatalf("слово границы не несёт «прежде не отправлялся»: %q", domain.OrderCorrectionNotSentWord)
	}
	if !strings.Contains(domain.OrderCorrectionChannelWord, "ручной контур магазина") {
		t.Fatalf("слово канала не называет ручной контур: %q", domain.OrderCorrectionChannelWord)
	}
	if !strings.Contains(domain.OrderCorrectionAssignGateWord, "проверка вернётся") {
		t.Fatalf("гейт-слово assign не несёт возврат проверки: %q", domain.OrderCorrectionAssignGateWord)
	}
	if !strings.Contains(domain.OrderCorrectionOrderGateWord, "новый заказ не создаётся") {
		t.Fatalf("гейт-слово авторизации не несёт отказ нового заказа: %q", domain.OrderCorrectionOrderGateWord)
	}
}

// TestOrderCorrectionContourNameSeparation pins the BY NAME separation
// of the two correction contours: the 2-15 operator's case-fact
// correction and the 4-8 order correction live in different modules
// with different identifiers (the collision is impossible by the naming
// discipline — the migration, the domain file and the tables carry
// order_correction).
func TestOrderCorrectionContourNameSeparation(t *testing.T) {
	if domain.OrderCorrectionEventOpened != "opened" {
		t.Fatalf("вид события открытия коррекции не «opened»")
	}
	// The event kinds of the two lifecycles never collide: the 2-13
	// handoff machine and the 4-7 verification machine live in their own
	// dictionaries; the correction kinds are the four-word set.
	if got := domain.OrderCorrectionEventKinds; strings.Join(got, ",") != "opened,reassigned,reread,ceased" {
		t.Fatalf("словарь видов событий коррекции: %v", got)
	}
	for _, kind := range domain.OrderCorrectionEventKinds {
		if domain.OrderVerificationEventKindInDictionary(kind) {
			t.Fatalf("вид события коррекции %q входит в словарь видов проверки 4-7 (контуры различаются именами)", kind)
		}
	}
}
