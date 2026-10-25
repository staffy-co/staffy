package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// Story 4.7 (step-01) — the manager order verification domain tests: the
// named-matrix guards, the read-time state derivations (the lifecycle, the
// overdue clock honesty, the reassignment epoch), the decision
// applicability, the byte-stability of the frozen words and the honest
// probes of the slice (the mandatory manual-contour word, the zero
// supplier-send words, the «назначен ≠ доставлено ≠ принял» separation).

func verifyTaskFixture() domain.OrderVerification {
	return domain.OrderVerification{
		TenantID:         id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a301"),
		CaseID:           id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a302"),
		WorkItemID:       id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a303"),
		VerificationNo:   1,
		OperationID:      id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a304"),
		ReceiptVersionNo: 1,
		ExternalOrderID:  "ABCP-ORDER-77",
		CreatorID:        id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a305"),
		CreatedAt:        time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC),
	}
}

func verifyEventsFixture() []domain.OrderVerificationEvent {
	base := time.Date(2026, 10, 25, 9, 5, 0, 0, time.UTC)
	return []domain.OrderVerificationEvent{
		{
			TenantID:       verifyTaskFixture().TenantID,
			CaseID:         verifyTaskFixture().CaseID,
			VerificationNo: 1,
			EventNo:        1,
			Kind:           domain.OrderVerificationEventAssigned,
			Assignee:       id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a306"),
			BackupOwner:    id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a307"),
			VerifyDeadline: domain.Verify4h,
			DeadlineAt:     base.Add(4 * time.Hour),
			ActorID:        verifyTaskFixture().CreatorID,
			CreatedAt:      base,
		},
	}
}

// TestVerifyDeadlineDictionary pins the two-word approved policy
// dictionary with its honest texts and windows (the word is gated, never
// parsed).
func TestVerifyDeadlineDictionary(t *testing.T) {
	if got, want := len(domain.VerifyDeadlines), 2; got != want {
		t.Fatalf("словарь дедлайнов проверки: %d слов, ожидалось %d", got, want)
	}
	for _, word := range domain.VerifyDeadlines {
		if text := domain.VerifyDeadlineText(word); strings.TrimSpace(text) == "" {
			t.Fatalf("слово дедлайна %q без честного текста", word)
		}
		if window := domain.VerifyDeadlineWindow(word); window <= 0 {
			t.Fatalf("слово дедлайна %q без окна времени", word)
		}
	}
	if domain.VerifyDeadlineWindow("verify_8h") != 0 {
		t.Fatalf("чужое слово дедлайна дало ненулевое окно")
	}
	if domain.VerifyDeadlineInDictionary("ack_15m") {
		t.Fatalf("слово словаря 2-13 не входит в словарь дедлайнов проверки")
	}
}

// TestVerifyBlockReasonDictionary pins the five-word structured reason
// dictionary of the correction decision (the manager never types a free
// reason).
func TestVerifyBlockReasonDictionary(t *testing.T) {
	want := []string{"wrong_part", "wrong_vehicle", "price_changed", "availability_changed", "confirmation_issue"}
	if got := domain.VerifyBlockReasons; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("словарь причин коррекции: %v, ожидалось %v", got, want)
	}
	for _, word := range domain.VerifyBlockReasons {
		if text := domain.VerifyBlockReasonText(word); strings.TrimSpace(text) == "" {
			t.Fatalf("слово причины %q без честного текста", word)
		}
	}
	if domain.VerifyBlockReasonInDictionary("because") {
		t.Fatalf("свободное слово прошло словарь причин")
	}
}

// TestGuardOrderVerificationAssignmentByName pins the assignment guard
// branches BY NAME: the assignee, the backup, the deadline word.
func TestGuardOrderVerificationAssignmentByName(t *testing.T) {
	assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a306")
	backup := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a307")
	if err := domain.GuardOrderVerificationAssignment(assignee, backup, domain.Verify1d); err != nil {
		t.Fatalf("валидное назначение отказано: %v", err)
	}
	if err := domain.GuardOrderVerificationAssignment(id.UUID(""), backup, domain.Verify1d); err == nil || !strings.Contains(err.Error(), "assignee") {
		t.Fatalf("пустой assignee не назван: %v", err)
	}
	if err := domain.GuardOrderVerificationAssignment(assignee, id.UUID(""), domain.Verify1d); err == nil || !strings.Contains(err.Error(), "backup_owner") {
		t.Fatalf("пустой backup не назван: %v", err)
	}
	if err := domain.GuardOrderVerificationAssignment(assignee, backup, "verify_8h"); err == nil || !strings.Contains(err.Error(), "verify_8h") {
		t.Fatalf("чужое слово дедлайна не названо: %v", err)
	}
}

// TestGuardOrderVerificationAcknowledgeByName pins the acceptance guard
// branches BY NAME: the state ladder, the assignee-only actor, the CAS
// epoch, the overdue refusal with the honest path.
func TestGuardOrderVerificationAcknowledgeByName(t *testing.T) {
	assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a306")
	other := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a308")
	if err := domain.GuardOrderVerificationAcknowledge(domain.OrderVerificationStateAssigned, assignee, assignee, 1); err != nil {
		t.Fatalf("валидное принятие отказано: %v", err)
	}
	if err := domain.GuardOrderVerificationAcknowledge(domain.OrderVerificationStateOverdue, assignee, assignee, 1); err == nil || !strings.Contains(err.Error(), "переназначение") {
		t.Fatalf("просроченный раунд не назвал путь переназначения: %v", err)
	}
	if err := domain.GuardOrderVerificationAcknowledge(domain.OrderVerificationStateAcknowledged, assignee, assignee, 1); err == nil || !strings.Contains(err.Error(), "назначен") {
		t.Fatalf("повторное принятие не названо словом состояния: %v", err)
	}
	if err := domain.GuardOrderVerificationAcknowledge(domain.OrderVerificationStateAssigned, other, assignee, 1); err == nil || err.Error() != domain.OrderVerificationNotAssigneeWord {
		t.Fatalf("не-assignee актёр получил не слово истины: %v", err)
	}
	if err := domain.GuardOrderVerificationAcknowledge(domain.OrderVerificationStateAssigned, assignee, assignee, 0); err == nil || !strings.Contains(err.Error(), "expected_assignment") {
		t.Fatalf("нулевая эпоха CAS не названа: %v", err)
	}
}

// TestGuardOrderVerificationApproveByName pins the approval guard branches
// BY NAME: the acknowledged-only state, the assignee-only actor, the CAS
// epoch, the 64-hex digest pin.
func TestGuardOrderVerificationApproveByName(t *testing.T) {
	assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a306")
	digest := strings.Repeat("a", 64)
	if err := domain.GuardOrderVerificationApprove(domain.OrderVerificationStateAcknowledged, assignee, assignee, 1, digest); err != nil {
		t.Fatalf("валидное подтверждение отказано: %v", err)
	}
	if err := domain.GuardOrderVerificationApprove(domain.OrderVerificationStateAssigned, assignee, assignee, 1, digest); err == nil || err.Error() != domain.OrderVerificationNotAcknowledgedWord {
		t.Fatalf("решение без принятия получило не слово «ещё не принял»: %v", err)
	}
	if err := domain.GuardOrderVerificationApprove(domain.OrderVerificationStateOverdue, assignee, assignee, 1, digest); err == nil || !strings.Contains(err.Error(), "переназначение") {
		t.Fatalf("просроченное решение не назвало путь: %v", err)
	}
	if err := domain.GuardOrderVerificationApprove(domain.OrderVerificationStateApproved, assignee, assignee, 1, digest); err == nil || !strings.Contains(err.Error(), "принял") {
		t.Fatalf("закрытая задача не названа: %v", err)
	}
	if err := domain.GuardOrderVerificationApprove(domain.OrderVerificationStateAcknowledged, assignee, assignee, 1, "zz"); err == nil || !strings.Contains(err.Error(), "expected_order_digest") {
		t.Fatalf("не-hex пин дайджеста не назван: %v", err)
	}
	if err := domain.GuardOrderVerificationApprove(domain.OrderVerificationStateAcknowledged, assignee, assignee, 0, digest); err == nil || !strings.Contains(err.Error(), "expected_assignment") {
		t.Fatalf("нулевая эпоха CAS не названа: %v", err)
	}
}

// TestGuardOrderVerificationBlockByName pins the block guard branches BY
// NAME: the state ladder, the positions bound, the duplicates, the reason
// dictionary.
func TestGuardOrderVerificationBlockByName(t *testing.T) {
	assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a306")
	if err := domain.GuardOrderVerificationBlock(domain.OrderVerificationStateAcknowledged, assignee, assignee, 1,
		[]string{"front-left", "front-right"}, domain.VerifyBlockReasonPriceChanged); err != nil {
		t.Fatalf("валидная блокировка отказана: %v", err)
	}
	if err := domain.GuardOrderVerificationBlock(domain.OrderVerificationStateAcknowledged, assignee, assignee, 1,
		nil, domain.VerifyBlockReasonPriceChanged); err == nil || !strings.Contains(err.Error(), "позиции") {
		t.Fatalf("пустые позиции не названы: %v", err)
	}
	if err := domain.GuardOrderVerificationBlock(domain.OrderVerificationStateAcknowledged, assignee, assignee, 1,
		[]string{"a", "a"}, domain.VerifyBlockReasonPriceChanged); err == nil || !strings.Contains(err.Error(), "повторяется") {
		t.Fatalf("дубль позиции не назван: %v", err)
	}
	if err := domain.GuardOrderVerificationBlock(domain.OrderVerificationStateAcknowledged, assignee, assignee, 1,
		[]string{"a"}, "expensive"); err == nil || !strings.Contains(err.Error(), "словар") {
		t.Fatalf("свободная причина не названа: %v", err)
	}
	if err := domain.GuardOrderVerificationBlock(domain.OrderVerificationStateAssigned, assignee, assignee, 1,
		[]string{"a"}, domain.VerifyBlockReasonWrongPart); err == nil || err.Error() != domain.OrderVerificationNotAcknowledgedWord {
		t.Fatalf("блокировка без принятия получила не слово: %v", err)
	}
}

// TestGuardOrderVerificationReassignByName pins the reassignment guard
// branches BY NAME: the open states only, the valid members, the
// dictionary deadline.
func TestGuardOrderVerificationReassignByName(t *testing.T) {
	assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a306")
	backup := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a307")
	for _, state := range []string{domain.OrderVerificationStateAssigned, domain.OrderVerificationStateAcknowledged, domain.OrderVerificationStateOverdue} {
		if err := domain.GuardOrderVerificationReassign(state, assignee, backup, domain.Verify4h); err != nil {
			t.Fatalf("переназначение из открытого состояния %q отказано: %v", state, err)
		}
	}
	if err := domain.GuardOrderVerificationReassign(domain.OrderVerificationStateApproved, assignee, backup, domain.Verify4h); err == nil || !strings.Contains(err.Error(), "закрыта") {
		t.Fatalf("переназначение закрытой задачи не названо: %v", err)
	}
	if err := domain.GuardOrderVerificationReassign(domain.OrderVerificationStateAssigned, id.UUID(""), backup, domain.Verify4h); err == nil || !strings.Contains(err.Error(), "assignee") {
		t.Fatalf("пустой assignee не назван: %v", err)
	}
}

// TestDeriveOrderVerificationStateLifecycle pins the full lifecycle
// derivation: assigned → acknowledged → approved; the round material
// derives from the LATEST assigned event; the decision material carries
// the Human Verification Receipt pins.
func TestDeriveOrderVerificationStateLifecycle(t *testing.T) {
	task := verifyTaskFixture()
	events := verifyEventsFixture()
	now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	view := domain.DeriveOrderVerificationState(task, events, now)
	if view.State != domain.OrderVerificationStateAssigned {
		t.Fatalf("состояние после назначения: %q", view.State)
	}
	if view.AssignmentEpoch != 1 {
		t.Fatalf("эпоха назначения: %d", view.AssignmentEpoch)
	}
	ack := events[0]
	ack.Kind = domain.OrderVerificationEventAcknowledged
	ack.EventNo = 2
	events = append(events, ack)
	view = domain.DeriveOrderVerificationState(task, events, now)
	if view.State != domain.OrderVerificationStateAcknowledged {
		t.Fatalf("состояние после принятия: %q", view.State)
	}
	approved := domain.OrderVerificationEvent{
		TenantID:         task.TenantID,
		CaseID:           task.CaseID,
		VerificationNo:   1,
		EventNo:          3,
		Kind:             domain.OrderVerificationEventApproved,
		OrderDigest:      strings.Repeat("b", 64),
		ReceiptVersionNo: 1,
		ExternalOrderID:  "ABCP-ORDER-77",
		EvidenceAnchors:  []string{"wi:1", "fit:front-left"},
		ActorID:          view.Assignee,
		CreatedAt:        now,
	}
	events = append(events, approved)
	view = domain.DeriveOrderVerificationState(task, events, now)
	if view.State != domain.OrderVerificationStateApproved {
		t.Fatalf("состояние после решения: %q", view.State)
	}
	if view.Decision == nil || view.Decision.OrderDigest != strings.Repeat("b", 64) {
		t.Fatalf("материал решения не прочитан: %+v", view.Decision)
	}
	if view.Active() {
		t.Fatalf("закрытая задача активна")
	}
	// The round material survives the decision (the card keeps the round).
	if view.Assignee != events[0].Assignee || view.BackupOwner != events[0].BackupOwner {
		t.Fatalf("материал раунда потерян после решения")
	}
}

// TestDeriveOrderVerificationStateOverdue pins the honest clock
// derivation: the open round whose deadline passed derives overdue — both
// without and with the acceptance; the automatic approval NEVER happens
// (the expiry is a word, the closing events are the decisions only).
func TestDeriveOrderVerificationStateOverdue(t *testing.T) {
	task := verifyTaskFixture()
	events := verifyEventsFixture()
	before := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	after := time.Date(2026, 10, 25, 14, 0, 1, 0, time.UTC)
	if view := domain.DeriveOrderVerificationState(task, events, before); view.State != domain.OrderVerificationStateAssigned {
		t.Fatalf("до дедлайна: %q", view.State)
	}
	if view := domain.DeriveOrderVerificationState(task, events, after); view.State != domain.OrderVerificationStateOverdue {
		t.Fatalf("после дедлайна без решения: %q", view.State)
	}
	ack := events[0]
	ack.Kind = domain.OrderVerificationEventAcknowledged
	ack.EventNo = 2
	open := append(events, ack)
	overdueView := domain.DeriveOrderVerificationState(task, open, after)
	if overdueView.State != domain.OrderVerificationStateOverdue {
		t.Fatalf("после дедлайна с принятием без решения: %q", overdueView.State)
	}
	if !overdueView.Active() {
		t.Fatalf("просроченная задача не активна — переназначение закрыто")
	}
	// The decision closes the lifecycle: the deadline never re-opens it.
	approved := ack
	approved.Kind = domain.OrderVerificationEventApproved
	approved.EventNo = 3
	approved.OrderDigest = strings.Repeat("c", 64)
	approved.ReceiptVersionNo = 1
	approved.ExternalOrderID = "ABCP-ORDER-77"
	approved.EvidenceAnchors = []string{"wi:1"}
	closed := append(open, approved)
	if view := domain.DeriveOrderVerificationState(task, closed, after); view.State != domain.OrderVerificationStateApproved {
		t.Fatalf("после дедлайна с решением: %q", view.State)
	}
}

// TestDeriveOrderVerificationStateReassignEpoch pins the reassignment
// ladder: a NEW assigned event opens a NEW round with a NEW epoch — the
// former assignee's stale-epoch attempts answer the honest refusal word.
func TestDeriveOrderVerificationStateReassignEpoch(t *testing.T) {
	task := verifyTaskFixture()
	events := verifyEventsFixture()
	now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	reassigned := domain.OrderVerificationEvent{
		TenantID:       task.TenantID,
		CaseID:         task.CaseID,
		VerificationNo: 1,
		EventNo:        2,
		Kind:           domain.OrderVerificationEventAssigned,
		Assignee:       id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a309"),
		BackupOwner:    id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4a310"),
		VerifyDeadline: domain.Verify1d,
		DeadlineAt:     now.Add(24 * time.Hour),
		ActorID:        task.CreatorID,
		CreatedAt:      now,
	}
	view := domain.DeriveOrderVerificationState(task, append(events, reassigned), now)
	if view.State != domain.OrderVerificationStateAssigned {
		t.Fatalf("переназначение не открыло новый раунд: %q", view.State)
	}
	if view.AssignmentEpoch != 2 {
		t.Fatalf("эпоха нового раунда: %d", view.AssignmentEpoch)
	}
	if view.Assignee != reassigned.Assignee || view.DeadlineAt != reassigned.DeadlineAt {
		t.Fatalf("материал нового раунда не прочитан")
	}
}

// TestDeriveOrderVerificationApplicability pins the decision
// applicability derivation: the equal digests stay applicable; a
// divergence makes the former decision inapplicable with the honest word.
func TestDeriveOrderVerificationApplicability(t *testing.T) {
	pin := strings.Repeat("d", 64)
	if got := domain.DeriveOrderVerificationApplicability(pin, pin); !got.Applicable {
		t.Fatalf("равные дайджесты неприменимы")
	}
	if got := domain.DeriveOrderVerificationApplicability(pin, strings.Repeat("e", 64)); got.Applicable || got.Word != domain.OrderVerificationInapplicableWord {
		t.Fatalf("расхождение дайджестов: применимо=%v слово=%q", got.Applicable, got.Word)
	}
}

// TestOrderVerificationWordsByteStability pins the frozen honest words of
// the slice BYTE-EXACT (the words are the contract — the render, the
// OpenAPI descriptions and the journeys pin them; a change is a conscious
// dictionary review, never a refactor side effect).
func TestOrderVerificationWordsByteStability(t *testing.T) {
	pins := map[string]string{
		domain.OrderVerificationAssignedWord:           "назначен, ещё не принял: задача проверки адресована менеджеру и ждёт явного принятия",
		domain.OrderVerificationAcknowledgedWord:       "принял: менеджер подтвердил владение проверкой — проверка идёт",
		domain.OrderVerificationApprovedWord:           "проверка подтверждена: решение зафиксировано квитанцией проверки с точной версией заказа",
		domain.OrderVerificationBlockedWord:            "продвижение заказа заблокировано; запущена коррекция — контур 4-8; исходные доказательства не изменены",
		domain.OrderVerificationOverdueWord:            "просрочен: дедлайн проверки истёк без решения — автоматического одобрения нет; следующий шаг — переназначение владельцем (резервный владелец виден на карточке)",
		domain.OrderVerificationDeliveryWord:           "назначен ≠ доставлено ≠ принял: внешнего канала доставки уведомления нет — карточка этой поверхности и есть уведомление; доставка отдельно не фиксируется",
		domain.OrderVerificationManualContourWord:      "формирование и отправка поставщику — ручное действие менеджера в контуре магазина; Staffy не отправляет автоматически; статус читается отдельно из ABCP/1С",
		domain.OrderVerificationFenceWord:              "обход проверки руками в ABCP возможен — Staffy гарантирует словами только свой контур: автоматической отправки поставщику не существует, задача проверки видна на странице кейса",
		domain.OrderVerificationPurchasingHiddenWord:   "закупочные данные — внутренние данные магазина; без полномочия не раскрываются",
		domain.OrderVerificationInapplicableWord:       "неприменимо — состав заказа изменился; прежнее решение неприменимо к этой версии: требуется повторная полная проверка",
		domain.OrderVerificationCompositionChangedWord: "состав заказа изменился — решение неприменимо к этой версии; требуется повторная полная проверка",
		domain.OrderVerificationNotAcknowledgedWord:    "менеджер ещё не принял задачу проверки: назначение не считается принятием — подтвердите владение проверкой",
		domain.OrderVerificationActiveText:             "проверка заказа уже активна — дождитесь решения либо переназначьте: вторая конкурирующая задача не создаётся",
		domain.OrderVerificationReassignedWord:         "задача переназначена: ожидаемая эпоха назначения устарела — перечитайте задачу и повторите команду",
		domain.OrderVerificationNoReceiptWord:          "нет подтверждённой квитанции заказа — проверять нечего: задача строится над созданным заказом",
		domain.OrderVerificationNotAssigneeWord:        "подтвердить или решить задачу может только назначенный и принявший менеджер: назначение не считается принятием",
	}
	for word, want := range pins {
		if word != want {
			t.Fatalf("замороженное слово изменилось:\nесть:    %q\nожидалось: %q", word, want)
		}
	}
}

// TestOrderVerificationStateWords pins the five derived state words and
// their presence in the dictionary with the texts (the states are words,
// never colors).
func TestOrderVerificationStateWords(t *testing.T) {
	want := []string{"assigned", "acknowledged", "overdue", "approved", "blocked"}
	if got := domain.OrderVerificationStates; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("словарь состояний проверки: %v, ожидалось %v", got, want)
	}
	for _, word := range domain.OrderVerificationStates {
		if text := domain.OrderVerificationStateText(word); strings.TrimSpace(text) == "" {
			t.Fatalf("состояние %q без текста", word)
		}
	}
	if domain.OrderVerificationStateActive(domain.OrderVerificationStateApproved) {
		t.Fatalf("approved не открытое состояние")
	}
	if !domain.OrderVerificationStateActive(domain.OrderVerificationStateOverdue) {
		t.Fatalf("overdue — открытое состояние (переназначение живо)")
	}
}

// TestOrderVerificationManualContourProbe pins the MANDATORY
// manual-contour word of the approved render (AC-10/AC-11): the word is a
// constant of the domain — the transport render and the journeys pin its
// presence; the supplier-send honest negations ride THIS word, never a
// send claim.
func TestOrderVerificationManualContourProbe(t *testing.T) {
	for _, needle := range []string{"ручное действие менеджера", "Staffy не отправляет автоматически", "статус читается отдельно"} {
		if !strings.Contains(domain.OrderVerificationManualContourWord, needle) {
			t.Fatalf("обязательное слово ручного контура потеряло часть %q: %q", needle, domain.OrderVerificationManualContourWord)
		}
	}
}

// TestOrderVerificationSupplierSendProbe pins the structural supplier-send
// fence of the domain words (OQ-3): no dictionary word of the verification
// contour CLAIMS a supplier send — the only words mentioning the send are
// the honest NEGATIONS (the manual contour, the fence honesty, the
// boundary of the state words).
func TestOrderVerificationSupplierSendProbe(t *testing.T) {
	words := []string{
		domain.OrderVerificationAssignedWord, domain.OrderVerificationAcknowledgedWord,
		domain.OrderVerificationApprovedWord, domain.OrderVerificationBlockedWord,
		domain.OrderVerificationOverdueWord, domain.OrderVerificationDeliveryWord,
		domain.OrderVerificationManualContourWord, domain.OrderVerificationFenceWord,
		domain.OrderVerificationCorrectionWord, domain.OrderVerificationPurchasingHiddenWord,
		domain.OrderVerificationInapplicableWord, domain.OrderVerificationCompositionChangedWord,
		domain.OrderVerificationNotAcknowledgedWord, domain.OrderVerificationActiveText,
		domain.OrderVerificationReassignedWord, domain.OrderVerificationNoReceiptWord,
		domain.OrderVerificationNotAssigneeWord,
	}
	for _, word := range words {
		lower := strings.ToLower(word)
		if strings.Contains(lower, "отправлен поставщику") || strings.Contains(lower, "отправка выполнена") ||
			strings.Contains(lower, "отправлено поставщику") || strings.Contains(lower, "успешно отправл") {
			t.Fatalf("слово домена проверки заявляет отправку поставщику (зонд OQ-3): %q", word)
		}
		if strings.Contains(lower, "поставщик") || strings.Contains(lower, "отправля") {
			// The send-mentioning words must be the honest negations.
			if !strings.Contains(lower, "не отправляет") && !strings.Contains(lower, "не существует") && !strings.Contains(lower, "ручное действие") {
				t.Fatalf("слово упоминает поставщика/отправку без честного отрицания (зонд OQ-3): %q", word)
			}
		}
	}
}

// TestOrderVerificationDeliverySeparationProbe pins the AC-2 honesty:
// «назначен ≠ доставлено ≠ принял» — the assignment, the delivery and the
// acceptance are THREE different truths; the delivery word speaks the
// absent external channel, the assigned and acknowledged state words stay
// two different words.
func TestOrderVerificationDeliverySeparationProbe(t *testing.T) {
	if domain.OrderVerificationAssignedWord == domain.OrderVerificationAcknowledgedWord {
		t.Fatalf("назначение и принятие — одно слово")
	}
	if !strings.Contains(domain.OrderVerificationDeliveryWord, "назначен ≠ доставлено ≠ принял") {
		t.Fatalf("слово доставки не различает три истины: %q", domain.OrderVerificationDeliveryWord)
	}
	if !strings.Contains(domain.OrderVerificationDeliveryWord, "внешнего канала доставки") {
		t.Fatalf("слово доставки не называет отсутствующий внешний канал: %q", domain.OrderVerificationDeliveryWord)
	}
}
