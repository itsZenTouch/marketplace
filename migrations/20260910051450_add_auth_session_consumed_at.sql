-- +goose Up

ALTER TABLE auth_sessions
ADD COLUMN consumed_at TIMESTAMPTZ;

ALTER TABLE auth_sessions
ADD CONSTRAINT auth_sessions_consumed_at_check
CHECK (
    consumed_at IS NULL
    OR consumed_at >= created_at
);

-- +goose Down

ALTER TABLE auth_sessions
DROP CONSTRAINT IF EXISTS auth_sessions_consumed_at_check;

ALTER TABLE auth_sessions
DROP COLUMN IF EXISTS consumed_at;
