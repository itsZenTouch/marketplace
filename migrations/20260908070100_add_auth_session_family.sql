-- +goose Up

ALTER TABLE auth_sessions
ADD COLUMN family_id UUID;

UPDATE auth_sessions
SET family_id = id
WHERE family_id IS NULL;

ALTER TABLE auth_sessions
ALTER COLUMN family_id SET NOT NULL;

ALTER TABLE auth_sessions
ADD COLUMN revocation_reason TEXT;

CREATE INDEX idx_auth_sessions_family_id
ON auth_sessions(family_id);

-- +goose Down

DROP INDEX IF EXISTS idx_auth_sessions_family_id;

ALTER TABLE auth_sessions
DROP COLUMN IF EXISTS revocation_reason;

ALTER TABLE auth_sessions
DROP COLUMN IF EXISTS family_id;
