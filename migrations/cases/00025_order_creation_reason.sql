-- +goose Up
-- Story 4.6 (Epic 4): расширение SQL-зеркала словаря причин Human Handoff
-- 2.13. HandoffReasons домена расширяется спекой 4-6 одним операторским
-- словом order_creation — рекомендация панели заказа на запаркованной
-- операции AG-4 (nil-адаптер): задача менеджеру создать клиентский заказ
-- в интерфейсе ABCP по точной версии материала (OrderDigest над пинами
-- всех пяти входов + слово менеджерской проверки), БЕЗ автоматической
-- отправки поставщику (fence OQ-3: supplier_form/send отсутствует в
-- Action Catalog; скрытая автоматизация интерфейса не является заменой
-- capability — AC-9). CHECK-ограничение cases.human_handoffs.reason —
-- материальное зеркало словаря: расширенный словарь требует расширенного
-- зеркала, иначе первая рекомендация панели нарушит констрейнт
-- (прецеденты расширения: cases/00019 reason 1→4, cases/00021 reason
-- 4→5, cases/00024 reason 5→6 — append-only новой миграцией, без
-- переписывания 00013). Безусловность клиентского запроса 2.13 НЕ
-- тронута: client_request остаётся первым словом словаря, запрос клиента
-- НИКОГДА не требует причины, новое слово ПАРАЛЛЕЛЬНО клиентскому — его
-- продюсер оператор, а не клиент. Единственное SQL-касание замороженного
-- контура 2-13 в срезе 4-6.

ALTER TABLE cases.human_handoffs
 DROP CONSTRAINT human_handoffs_reason_check;

ALTER TABLE cases.human_handoffs
 ADD CONSTRAINT human_handoffs_reason_check
 CHECK (reason IN ('client_request','diagnostic_uncertainty','safety_critical','unresolved_contradiction','fitment_research','offers_research','order_creation'));

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM cases.human_handoffs WHERE reason = 'order_creation') THEN
  RAISE EXCEPTION 'order creation handoff reasons exist; they are immutable history — export and retain them before rollback';
 END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE cases.human_handoffs
 DROP CONSTRAINT human_handoffs_reason_check;

ALTER TABLE cases.human_handoffs
 ADD CONSTRAINT human_handoffs_reason_check
 CHECK (reason IN ('client_request','diagnostic_uncertainty','safety_critical','unresolved_contradiction','fitment_research','offers_research'));
