-- +goose Up
-- Story 5-1 (Epic 5): Montagio Admission Registry — расширение SQL-зеркала
-- словаря gate-определений. gateDictionary домена расширяется спекой 5-1
-- шестью пилотными определениями Montagio {OQ-7A, OQ-8, OQ-9, OQ-10,
-- OQ-13A, OQ-18} (AG-4 пилотного набора — уже присутствующее платформенное
-- определение, тот же канон). CHECK-ограничение employee.gate_instances —
-- материальное зеркало словаря: расширенный словарь требует расширенного
-- зеркала, иначе первая же запись Montagio-гейта нарушит констрейнт.
-- Урок 4-1 step-04 применён заранее: зеркало расширено той же миграцией,
-- что и словарь (SQLSTATE 23514 живой матрицей больше не ловится —
-- конструкция 4-1, повторённая вторым переносом).
-- Дополнение к зеркалу: script-слова поставщика отсутствуют —
-- supplier_form/send не является словом какого-либо гейта (Fixed Oleg
-- pilot fence, прецедент 00013).

ALTER TABLE employee.gate_instances
 DROP CONSTRAINT gate_instances_definition_id_check;

ALTER TABLE employee.gate_instances
 ADD CONSTRAINT gate_instances_definition_id_check
 CHECK (definition_id IN ('OQ-1','OQ-2','OQ-3','OQ-4','OQ-5','OQ-7A','OQ-7B','OQ-8','OQ-9','OQ-10','OQ-11','OQ-12','OQ-13A','OQ-13B','OQ-14','OQ-18','OQ-19','AG-1','AG-2','AG-4'));

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM employee.gate_instances WHERE definition_id IN ('OQ-7A','OQ-8','OQ-9','OQ-10','OQ-13A','OQ-18')) THEN
  RAISE EXCEPTION 'Montagio gate instances exist; they are immutable history — export and retain them before rollback';
 END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE employee.gate_instances
 DROP CONSTRAINT gate_instances_definition_id_check;

ALTER TABLE employee.gate_instances
 ADD CONSTRAINT gate_instances_definition_id_check
 CHECK (definition_id IN ('OQ-1','OQ-2','OQ-3','OQ-4','OQ-5','OQ-7B','OQ-11','OQ-12','OQ-13B','OQ-14','OQ-19','AG-1','AG-2','AG-4'));
