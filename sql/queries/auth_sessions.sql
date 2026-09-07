-- name: CreateAuthSession :one
INSERT INTO auth_sessions (
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at,
    revoked_at,
    created_at,
    updated_at;

-- name: GetAuthSessionByID :one
SELECT
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at,
    revoked_at,
    created_at,
    updated_at
FROM auth_sessions
WHERE id = $1
LIMIT 1;

-- name: GetActiveAuthSessionByID :one
SELECT
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at,
    revoked_at,
    created_at,
    updated_at
FROM auth_sessions
WHERE id = $1
AND revoked_at IS NULL
  AND expires_at > NOW()
LIMIT 1;

-- name: RevokeAuthSession :one
UPDATE auth_sessions
SET
    revoked_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND revoked_at IS NULL
RETURNING
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at,
    revoked_at,
    created_at,
    updated_at;

-- name: ListAuthSessionsByUserID :many
SELECT
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at,
    revoked_at,
    created_at,
    updated_at
FROM auth_sessions
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: RotateAuthSession :one
UPDATE auth_sessions
SET
    refresh_token_hash = sqlc.arg(new_refresh_token_hash),
    expires_at = sqlc.arg(expires_at),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND revoked_at IS NULL
  AND expires_at > NOW()
  AND refresh_token_hash = sqlc.arg(current_refresh_token_hash)
RETURNING
    id,
    user_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at,
    revoked_at,
    created_at,
    updated_at;

