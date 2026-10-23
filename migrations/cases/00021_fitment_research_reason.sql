-- +goose Up
-- Story 4.4 (Epic 4): расширение SQL-зеркала словаря причин Human Handoff
-- 2.13. HandoffReasons домена расширяется спекой 4-4 одним операторским
-- словом fitment_research — рекомендация панели «Применимость»: задача
-- менеджеру выполнить каталогическое исследование и записать результат
-- (AC-7: действие только в интерфейсе менеджера → decision-ready задача).
-- CHECK-ограничение cases.human_handoffs.reason — материальное зеркало
-- словаря: расширенный словарь требует расширенного зеркала, иначе первая
-- рекомендация панели нарушит констрейнт (прецеденты расширения:
-- cases/00019 reason 1→4 — append-only новой миграцией, без переписывания
-- 00013; комментарий словаря домена: «extending it is a new spec
-- version» — эта спека та версия). Безусловность клиентского запроса
-- 2.13 НЕ тронута: client_request остаётся первым словом словаря, запрос
-- клиента НИКОГДА не требует причины, новое слово ПАРАЛЛЕЛЬНО клиентскому
-- — его продюсер оператор, а не клиент. Второе и последнее SQL среза 4-4.

ALTER TABLE cases.human_handoffs
 DROP CONSTRAINT human_handoffs_reason_check;

ALTER TABLE cases.human_handoffs
 ADD CONSTRAINT human_handoffs_reason_check
 CHECK (reason IN ('client_request','diagnostic_uncertainty','safety_critical','unresolved_contradiction','fitment_research'));

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM cases.human_handoffs WHERE reason = 'fitment_research') THEN
  RAISE EXCEPTION 'fitment research handoff reasons exist; they are immutable history — export and retain them before rollback';
 END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE cases.human_handoffs
 DROP CONSTRAINT human_handoffs_reason_check;

ALTER TABLE cases.human_handoffs
 ADD CONSTRAINT human_handoffs_reason_check
 CHECK (reason IN ('client_request','diagnostic_uncertainty','safety_critical','unresolved_contradiction'));
