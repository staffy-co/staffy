-- +goose Up
-- Story 1.11: the commercial scope joins the system dictionary. Owners keep
-- the full dictionary (the default applies to every new owner membership);
-- other members receive the scope only through an explicit membership grant.

ALTER TABLE identity.memberships ALTER COLUMN scopes SET DEFAULT ARRAY['configuration','integrations','operations','review','technical_expertise','claims','audit','data_governance','commercial']::text[];
UPDATE identity.memberships SET scopes = scopes || 'commercial'::text WHERE role='owner' AND status='active';

-- +goose Down
UPDATE identity.memberships SET scopes = array_remove(scopes, 'commercial');
ALTER TABLE identity.memberships ALTER COLUMN scopes SET DEFAULT ARRAY['configuration','integrations','operations','review','technical_expertise','claims','audit','data_governance']::text[];
