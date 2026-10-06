-- +goose Up
-- Story 1.10: tenant-scoped purge of expired invitations (identity-owned class).
-- Only rows whose expires_at is at or before the caller-provided cutoff are
-- removed; the cutoff comes from the application clock so the operation stays
-- deterministic and testable. Active memberships are never touched here.

-- The data_governance scope joins the system dictionary: owners keep the full
-- scope set implicitly (their membership lists every scope), so the default
-- for new owner rows includes it and existing owners are upgraded in place.
ALTER TABLE identity.memberships ALTER COLUMN scopes SET DEFAULT ARRAY['configuration','integrations','operations','review','technical_expertise','claims','audit','data_governance']::text[];
UPDATE identity.memberships SET scopes = scopes || 'data_governance'::text WHERE role='owner' AND status='active';

-- +goose StatementBegin
CREATE FUNCTION identity.purge_expired_invitations(p_tenant uuid, p_cutoff timestamptz)
RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog AS $$
DECLARE deleted bigint;
BEGIN
  DELETE FROM identity.invitations i
   WHERE i.tenant_id = p_tenant AND i.expires_at <= p_cutoff;
  GET DIAGNOSTICS deleted = ROW_COUNT;
  RETURN deleted;
END $$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION identity.purge_expired_invitations(uuid, timestamptz) FROM PUBLIC, staffy_app;
GRANT EXECUTE ON FUNCTION identity.purge_expired_invitations(uuid, timestamptz) TO staffy_app;

-- +goose Down
UPDATE identity.memberships SET scopes = array_remove(scopes, 'data_governance');
ALTER TABLE identity.memberships ALTER COLUMN scopes SET DEFAULT ARRAY['configuration','integrations','operations','review','technical_expertise','claims','audit']::text[];
REVOKE EXECUTE ON FUNCTION identity.purge_expired_invitations(uuid, timestamptz) FROM staffy_app;
DROP FUNCTION identity.purge_expired_invitations(uuid, timestamptz);
