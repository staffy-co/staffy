-- +goose Up
-- Story 4-1 (Epic 4): Pilot Admission Registry — расширение SQL-зеркала
-- словаря gate-определений. gateDictionary домена расширяется спекой 4-1
-- шестью пилотными определениями Олега {OQ-1, OQ-2, OQ-3, OQ-4, OQ-5,
-- OQ-19} (AG-4 пилотного набора — уже присутствующее платформенное
-- определение, тот же канон). CHECK-ограничение employee.gate_instances —
-- материальное зеркало словаря: расширенный словарь требует расширенного
-- зеркала, иначе первая же запись пилотного гейта нарушит констрейнт.
-- Спека заявляла «первый срез Epic 4 без SQL» в смысле «без новых таблиц»
-- (admission — read-time вывод над существующими контурами); расхождение
-- с CHECK-зеркалом словаря обнаружено живой интеграционной матрицей
-- (row-5: OQ-3 in_test) и закрыто этой миграцией — append-only новой
-- миграцией, без переписывания 00012.
-- Дополнение к зеркалу: script-слова поставщика отсутствуют —
-- supplier_form/send не является словом какого-либо гейта (Fixed Oleg
-- pilot fence).

ALTER TABLE employee.gate_instances
 DROP CONSTRAINT gate_instances_definition_id_check;

ALTER TABLE employee.gate_instances
 ADD CONSTRAINT gate_instances_definition_id_check
 CHECK (definition_id IN ('OQ-1','OQ-2','OQ-3','OQ-4','OQ-5','OQ-7B','OQ-11','OQ-12','OQ-13B','OQ-14','OQ-19','AG-1','AG-2','AG-4'));

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM employee.gate_instances WHERE definition_id IN ('OQ-1','OQ-2','OQ-3','OQ-4','OQ-5','OQ-19')) THEN
  RAISE EXCEPTION 'pilot gate instances exist; they are immutable history — export and retain them before rollback';
 END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE employee.gate_instances
 DROP CONSTRAINT gate_instances_definition_id_check;

ALTER TABLE employee.gate_instances
 ADD CONSTRAINT gate_instances_definition_id_check
 CHECK (definition_id IN ('OQ-7B','OQ-11','OQ-12','OQ-13B','OQ-14','AG-1','AG-2','AG-4'));
