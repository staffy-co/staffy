-- +goose Up
ALTER TABLE employee.knowledge_sources ADD COLUMN origin_ref text NOT NULL DEFAULT '';
ALTER TABLE employee.knowledge_sources ADD COLUMN ingest_reason text NOT NULL DEFAULT '';
ALTER TABLE employee.knowledge_sources DROP CONSTRAINT knowledge_sources_source_type_check;
ALTER TABLE employee.knowledge_sources ADD CONSTRAINT knowledge_sources_source_type_check CHECK (source_type IN ('faq','regulation','dialogue','website','document'));
ALTER TABLE employee.knowledge_sources DROP CONSTRAINT knowledge_sources_status_check;
ALTER TABLE employee.knowledge_sources ADD CONSTRAINT knowledge_sources_status_check CHECK (status IN ('processing','ready','preview','quarantined','failed','retired'));
ALTER TABLE employee.knowledge_versions ADD COLUMN origin_ref text NOT NULL DEFAULT '';

-- +goose Down
DO $$ BEGIN
 IF EXISTS (
  SELECT 1 FROM employee.knowledge_sources
  WHERE source_type IN ('website','document') OR status IN ('preview','quarantined','failed')
     OR origin_ref <> '' OR ingest_reason <> ''
 ) OR EXISTS (SELECT 1 FROM employee.knowledge_versions WHERE origin_ref <> '') THEN
  RAISE EXCEPTION 'safe source ingestion rollback would discard provenance or source state';
 END IF;
END $$;
ALTER TABLE employee.knowledge_versions DROP COLUMN origin_ref;
ALTER TABLE employee.knowledge_sources DROP CONSTRAINT knowledge_sources_status_check;
ALTER TABLE employee.knowledge_sources ADD CONSTRAINT knowledge_sources_status_check CHECK (status IN ('processing','ready','retired'));
ALTER TABLE employee.knowledge_sources DROP CONSTRAINT knowledge_sources_source_type_check;
ALTER TABLE employee.knowledge_sources ADD CONSTRAINT knowledge_sources_source_type_check CHECK (source_type IN ('faq','regulation','dialogue'));
ALTER TABLE employee.knowledge_sources DROP COLUMN ingest_reason;
ALTER TABLE employee.knowledge_sources DROP COLUMN origin_ref;
