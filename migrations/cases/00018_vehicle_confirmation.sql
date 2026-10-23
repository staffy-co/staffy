-- +goose Up
-- Story 4.2 (Epic 4): Vehicle Identification — расширение SQL-зеркала
-- словаря use-kind продолжения. ContinuationUseKinds домена расширяется
-- спекой 4-2 одним словом {vehicle_confirmation_submitted} — клиентское
-- подтверждение идентификационных признаков автомобиля по защищённой
-- ссылке (consumes the link limit по правилам 2-10: подтверждение —
-- data submission, не событие). CHECK-ограничение
-- cases.continuation_link_uses.use_kind — материальное зеркало словаря:
-- расширенный словарь требует расширенного зеркала, иначе первая запись
-- подтверждения автомобиля нарушит констрейнт (прецедент расширения
-- employee/00013 8→14 — append-only новой миграцией, без переписывания
-- 00010; комментарий словаря домена: «extending it is a new spec
-- version» — эта спека та версия). Единственный SQL среза 4-2: профиль
-- read-time, факты/подтверждения/ссылки/uses — существующие таблицы,
-- новых таблиц и столбцов НЕТ. Подтверждение пишет cases.fact_confirmations
-- с confirmer='client' — SQL-CHECK 00002 допускал слово всегда
-- (зарезервированное слово 2-2 получает первого продюсера — без
-- миграции фактов).

ALTER TABLE cases.continuation_link_uses
 DROP CONSTRAINT continuation_link_uses_use_kind_check;

ALTER TABLE cases.continuation_link_uses
 ADD CONSTRAINT continuation_link_uses_use_kind_check
 CHECK (use_kind IN ('fact_correction_submitted','human_requested','returned_to_conversation','vehicle_confirmation_submitted'));

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM cases.continuation_link_uses WHERE use_kind = 'vehicle_confirmation_submitted') THEN
  RAISE EXCEPTION 'vehicle confirmation uses exist; they are immutable history — export and retain them before rollback';
 END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE cases.continuation_link_uses
 DROP CONSTRAINT continuation_link_uses_use_kind_check;

ALTER TABLE cases.continuation_link_uses
 ADD CONSTRAINT continuation_link_uses_use_kind_check
 CHECK (use_kind IN ('fact_correction_submitted','human_requested','returned_to_conversation'));
