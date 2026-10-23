-- +goose Up
-- Story 4.3 (Epic 4): Part Request Classification — расширение SQL-зеркала
-- словаря причин Human Handoff 2.13. HandoffReasons домена расширяется
-- спекой 4-3 тремя операторскими словами {diagnostic_uncertainty,
-- safety_critical, unresolved_contradiction} — оператор передаёт кейс
-- человеку по рекомендации границы вывода части (DiagnosticBoundary AC-7 /
-- SafetyBoundary AC-8 / ContradictionBoundary AC-9). CHECK-ограничение
-- cases.human_handoffs.reason — материальное зеркало словаря: расширенный
-- словарь требует расширенного зеркала, иначе первая запись границы
-- нарушит констрейнт (прецеденты расширения: employee/00013 8→14,
-- cases/00018 use_kind 3→4 — append-only новой миграцией, без
-- переписывания 00013; комментарий словаря домена: «extending it is a
-- new spec version» — эта спека та версия). Безусловность клиентского
-- запроса 2.13 НЕ тронута: client_request остаётся первым словом
-- словаря, запрос клиента НИКОГДА не требует причины, новые слова
-- ПАРАЛЛЕЛЬНЫ клиентскому — их продюсер оператор, а не клиент.
-- Единственный SQL среза 4-3: Part Request — read-time вывод, новых
-- таблиц и столбцов НЕТ.

ALTER TABLE cases.human_handoffs
 DROP CONSTRAINT human_handoffs_reason_check;

ALTER TABLE cases.human_handoffs
 ADD CONSTRAINT human_handoffs_reason_check
 CHECK (reason IN ('client_request','diagnostic_uncertainty','safety_critical','unresolved_contradiction'));

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM cases.human_handoffs WHERE reason IN ('diagnostic_uncertainty','safety_critical','unresolved_contradiction')) THEN
  RAISE EXCEPTION 'boundary handoff reasons exist; they are immutable history — export and retain them before rollback';
 END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE cases.human_handoffs
 DROP CONSTRAINT human_handoffs_reason_check;

ALTER TABLE cases.human_handoffs
 ADD CONSTRAINT human_handoffs_reason_check
 CHECK (reason IN ('client_request'));
