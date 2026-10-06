-- +goose Up
-- Story 1.10: tenant-scoped purge functions for employee-owned artifact classes.
-- The application role never gets table DELETE; each purge is a SECURITY DEFINER
-- function owned by the migration owner and callable only with an explicit
-- tenant_id. Counts are returned to feed the Data Lifecycle Receipt.

-- +goose StatementBegin
CREATE FUNCTION employee.purge_knowledge_payload(p_tenant uuid)
RETURNS TABLE(deleted_sources bigint, dependent_rows bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE chunks bigint; versions bigint; sources bigint; audit_rows bigint; outbox_rows bigint;
BEGIN
  DELETE FROM employee.knowledge_chunks c
   WHERE c.tenant_id = p_tenant;
  GET DIAGNOSTICS chunks = ROW_COUNT;
  DELETE FROM employee.knowledge_versions v
   WHERE v.tenant_id = p_tenant;
  GET DIAGNOSTICS versions = ROW_COUNT;
  DELETE FROM employee.knowledge_audit a
   WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS audit_rows = ROW_COUNT;
  DELETE FROM employee.knowledge_outbox o
   WHERE o.tenant_id = p_tenant;
  GET DIAGNOSTICS outbox_rows = ROW_COUNT;
  DELETE FROM employee.knowledge_sources s
   WHERE s.tenant_id = p_tenant;
  GET DIAGNOSTICS sources = ROW_COUNT;
  deleted_sources := sources;
  dependent_rows := chunks + versions + audit_rows + outbox_rows;
  RETURN NEXT;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION employee.purge_voice_preview_media(p_tenant uuid)
RETURNS TABLE(deleted_previews bigint, dependent_rows bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE previews bigint; assessments bigint;
BEGIN
  DELETE FROM employee.voice_assessments a
   WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS assessments = ROW_COUNT;
  DELETE FROM employee.voice_previews p
   WHERE p.tenant_id = p_tenant;
  GET DIAGNOSTICS previews = ROW_COUNT;
  deleted_previews := previews;
  dependent_rows := assessments;
  RETURN NEXT;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION employee.purge_voice_assessments(p_tenant uuid)
RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE assessments bigint;
BEGIN
  DELETE FROM employee.voice_assessments a
   WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS assessments = ROW_COUNT;
  RETURN assessments;
END $$;
-- +goose StatementEnd

-- The draft aggregate: foreign keys bind knowledge, behavior, voice, preview,
-- assessment, audit, outbox and idempotency rows to employee.drafts, so the
-- honest deletion unit of the employee-draft class is the whole per-employee
-- data aggregate. Every deleted row group is counted for the receipt.
-- +goose StatementBegin
CREATE FUNCTION employee.purge_employee_drafts(p_tenant uuid)
RETURNS TABLE(deleted_drafts bigint, dependent_rows bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE drafts bigint; assessments bigint; previews bigint; voice_versions bigint;
        behavior_versions bigint; chunks bigint; kversions bigint; sources bigint;
        kaudit bigint; koutbox bigint; idem bigint; audit_rows bigint; outbox_rows bigint;
BEGIN
  DELETE FROM employee.voice_assessments a WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS assessments = ROW_COUNT;
  DELETE FROM employee.voice_previews p WHERE p.tenant_id = p_tenant;
  GET DIAGNOSTICS previews = ROW_COUNT;
  DELETE FROM employee.voice_versions v WHERE v.tenant_id = p_tenant;
  GET DIAGNOSTICS voice_versions = ROW_COUNT;
  DELETE FROM employee.behavior_versions b WHERE b.tenant_id = p_tenant;
  GET DIAGNOSTICS behavior_versions = ROW_COUNT;
  DELETE FROM employee.knowledge_chunks c WHERE c.tenant_id = p_tenant;
  GET DIAGNOSTICS chunks = ROW_COUNT;
  DELETE FROM employee.knowledge_versions v WHERE v.tenant_id = p_tenant;
  GET DIAGNOSTICS kversions = ROW_COUNT;
  DELETE FROM employee.knowledge_audit a WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS kaudit = ROW_COUNT;
  DELETE FROM employee.knowledge_outbox o WHERE o.tenant_id = p_tenant;
  GET DIAGNOSTICS koutbox = ROW_COUNT;
  DELETE FROM employee.knowledge_sources s WHERE s.tenant_id = p_tenant;
  GET DIAGNOSTICS sources = ROW_COUNT;
  DELETE FROM employee.idempotency_records i WHERE i.tenant_id = p_tenant;
  GET DIAGNOSTICS idem = ROW_COUNT;
  DELETE FROM employee.audit a WHERE a.tenant_id = p_tenant;
  GET DIAGNOSTICS audit_rows = ROW_COUNT;
  DELETE FROM employee.outbox_events o WHERE o.tenant_id = p_tenant;
  GET DIAGNOSTICS outbox_rows = ROW_COUNT;
  DELETE FROM employee.drafts d WHERE d.tenant_id = p_tenant;
  GET DIAGNOSTICS drafts = ROW_COUNT;
  deleted_drafts := drafts;
  dependent_rows := assessments + previews + voice_versions + behavior_versions +
                    chunks + kversions + sources + kaudit + koutbox + idem + audit_rows + outbox_rows;
  RETURN NEXT;
END $$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION employee.purge_knowledge_payload(uuid), employee.purge_voice_preview_media(uuid), employee.purge_voice_assessments(uuid), employee.purge_employee_drafts(uuid) FROM PUBLIC, staffy_app;
GRANT EXECUTE ON FUNCTION employee.purge_knowledge_payload(uuid), employee.purge_voice_preview_media(uuid), employee.purge_voice_assessments(uuid), employee.purge_employee_drafts(uuid) TO staffy_app;

-- +goose Down
REVOKE EXECUTE ON FUNCTION employee.purge_knowledge_payload(uuid), employee.purge_voice_preview_media(uuid), employee.purge_voice_assessments(uuid), employee.purge_employee_drafts(uuid) FROM staffy_app;
DROP FUNCTION employee.purge_employee_drafts(uuid);
DROP FUNCTION employee.purge_voice_assessments(uuid);
DROP FUNCTION employee.purge_voice_preview_media(uuid);
DROP FUNCTION employee.purge_knowledge_payload(uuid);
