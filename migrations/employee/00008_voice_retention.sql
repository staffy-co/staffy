-- +goose Up
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'staffy_voice_maintenance') THEN
  CREATE ROLE staffy_voice_maintenance NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
 END IF;
END $$;
ALTER ROLE staffy_voice_maintenance NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (
  SELECT 1 FROM pg_auth_members m
  JOIN pg_roles r ON r.oid = m.member
  WHERE r.rolname = 'staffy_voice_maintenance'
 ) THEN
  RAISE EXCEPTION 'staffy_voice_maintenance must not inherit or SET ROLE into another role';
 END IF;
END $$;
-- +goose StatementEnd
REVOKE ALL ON ALL TABLES IN SCHEMA employee FROM staffy_voice_maintenance;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA employee FROM staffy_voice_maintenance;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA employee FROM staffy_voice_maintenance;
GRANT USAGE ON SCHEMA employee TO staffy_voice_maintenance;

-- Owned by the migration owner; the caller has no table privileges. This is the
-- only permitted cross-tenant action and never returns retained text.
-- +goose StatementBegin
CREATE FUNCTION employee.purge_expired_voice_previews() RETURNS TABLE(deleted_assessments bigint, deleted_previews bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE cutoff timestamptz := clock_timestamp() - interval '24 hours';
BEGIN
 DELETE FROM employee.voice_assessments a
 USING employee.voice_previews p
 WHERE a.tenant_id = p.tenant_id AND a.preview_id = p.preview_id
 AND (p.created_at <= cutoff OR a.assessed_at <= cutoff);
 GET DIAGNOSTICS deleted_assessments = ROW_COUNT;
 DELETE FROM employee.voice_previews p WHERE p.created_at <= cutoff;
 GET DIAGNOSTICS deleted_previews = ROW_COUNT;
 RETURN NEXT;
END $$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION employee.purge_expired_voice_previews() FROM PUBLIC, staffy_app;
GRANT EXECUTE ON FUNCTION employee.purge_expired_voice_previews() TO staffy_voice_maintenance;

-- +goose Down
REVOKE EXECUTE ON FUNCTION employee.purge_expired_voice_previews() FROM staffy_voice_maintenance;
DROP FUNCTION employee.purge_expired_voice_previews();
REVOKE USAGE ON SCHEMA employee FROM staffy_voice_maintenance;
