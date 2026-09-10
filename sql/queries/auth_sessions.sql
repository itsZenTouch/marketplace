-- name: CreateAuthSession :one
INSERT INTO auth_sessions (
    id,
    user_id,
    family_id,
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
    $6,
    $7
)
RETURNING
    id,
    user_id,
    family_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at,
    consumed_at,
    revoked_at,
    revocation_reason,
    created_at,
    updated_at;

-- name: GetAuthSessionByID :one
SELECT
    id,
    user_id,
    family_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at,
    consumed_at,
    revoked_at,
    revocation_reason,
    created_at,
    updated_at
FROM auth_sessions
WHERE id = $1
LIMIT 1;


-- name: GetActiveAuthSessionByID :one
SELECT
    id,
    user_id,
    family_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at,
    consumed_at,
    revoked_at,
    revocation_reason,
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
    revocation_reason = sqlc.arg(revocation_reason),
    updated_at = NOW()
WHERE id = $1
  AND revoked_at IS NULL
RETURNING
    id,
    user_id,
    family_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at,
    consumed_at,
    revoked_at,
    revocation_reason,
    created_at,
    updated_at;

-- name: ListAuthSessionsByUserID :many
SELECT
    id,
    user_id,
    family_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at,
    consumed_at,
    revoked_at,
    revocation_reason,
    created_at,
    updated_at
FROM auth_sessions
WHERE user_id = $1
ORDER BY created_at DESC;


-- name: RevokeAuthSessionFamily :exec
UPDATE auth_sessions
SET
    revoked_at = NOW(),
    revocation_reason = sqlc.arg(revocation_reason),
    updated_at = NOW()
WHERE family_id = sqlc.arg(family_id)
  AND revoked_at IS NULL;



-- name: ConsumeAuthSession :one
UPDATE auth_sessions
SET
    consumed_at = NOW(),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND consumed_at IS NULL
  AND revoked_at IS NULL
  AND expires_at > NOW()
  AND refresh_token_hash = sqlc.arg(current_refresh_token_hash)
RETURNING
    id,
    user_id,
    family_id,
    refresh_token_hash,
    user_agent,
    ip_address,
    expires_at,
    consumed_at,
    revoked_at,
    revocation_reason,
    created_at,
    updated_at;
