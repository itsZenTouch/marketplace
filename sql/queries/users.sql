-- name: CreateUser :one
INSERT INTO users (
    id,
    email,
    password_hash,
    status
  )
VALUES ($1, $2, $3, $4)
RETURNING id,
  email,
  password_hash,
  status,
  email_verified_at,
  failed_login_attempts,
  locked_until,
  created_at,
  updated_at;

-- name: GetUserByID :one
SELECT id,
  email,
  password_hash,
  status,
  email_verified_at,
  failed_login_attempts,
  locked_until,
  created_at,
  updated_at
FROM users
WHERE id = $1
LIMIT 1;

-- name: GetUserByEmail :one
SELECT id,
  email,
  password_hash,
  status,
  email_verified_at,
  failed_login_attempts,
  locked_until,
  created_at,
  updated_at
FROM users
WHERE email = $1
LIMIT 1;

-- name: IncrementFailedLoginAttempts :one
UPDATE users
SET
    failed_login_attempts = failed_login_attempts + 1,
    updated_at = NOW()
WHERE id = $1
RETURNING
    id,
    email,
    password_hash,
    status,
    email_verified_at,
    failed_login_attempts,
    locked_until,
    created_at,
    updated_at;

-- name: ResetFailedLoginAttempts :one
UPDATE users
SET
    failed_login_attempts = 0,
    locked_until = NULL,
    updated_at = NOW()
WHERE id = $1
RETURNING
    id,
    email,
    password_hash,
    status,
    email_verified_at,
    failed_login_attempts,
    locked_until,
    created_at,
    updated_at;

-- name: LockUserUntil :one
UPDATE users
SET
    locked_until = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING
    id,
    email,
    password_hash,
    status,
    email_verified_at,
    failed_login_attempts,
    locked_until,
    created_at,
    updated_at;
