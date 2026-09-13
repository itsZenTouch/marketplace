-- name: ListUserRoles :many
SELECT
    r.id,
    r.name
FROM roles r
INNER JOIN user_roles ur
    ON ur.role_id = r.id
WHERE ur.user_id = $1
ORDER BY r.name;


-- name: ListUserPermissions :many
SELECT DISTINCT
    p.id,
    p.name
FROM permissions p
INNER JOIN role_permissions rp
    ON rp.permission_id = p.id
INNER JOIN user_roles ur
    ON ur.role_id = rp.role_id
WHERE ur.user_id = $1
ORDER BY p.name;


-- name: AssignRoleToUser :exec
INSERT INTO user_roles (
    user_id,
    role_id
)
VALUES ($1, $2)
ON CONFLICT (user_id, role_id)
DO NOTHING;


-- name: RemoveRoleFromUser :exec
DELETE FROM user_roles
WHERE user_id = $1
  AND role_id = $2;


-- name: AssignPermissionToRole :exec
INSERT INTO role_permissions (
    role_id,
    permission_id
)
VALUES ($1, $2)
ON CONFLICT (role_id, permission_id)
DO NOTHING;


-- name: RemovePermissionFromRole :exec
DELETE FROM role_permissions
WHERE role_id = $1
  AND permission_id = $2;


-- name: GetUserAuthorization :one
SELECT
    COALESCE(
        (
            SELECT array_agg(DISTINCT r.name)
            FROM roles r
            INNER JOIN user_roles ur ON ur.role_id = r.id
            WHERE ur.user_id = u.id
        ),
        ARRAY[]::text[]
    )::text[] AS roles,
    COALESCE(
        (
            SELECT array_agg(DISTINCT p.name)
            FROM permissions p
            INNER JOIN role_permissions rp ON rp.permission_id = p.id
            INNER JOIN user_roles ur ON ur.role_id = rp.role_id
            WHERE ur.user_id = u.id
        ),
        ARRAY[]::text[]
    )::text[] AS permissions
FROM users u
WHERE u.id = $1;
