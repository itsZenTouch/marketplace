package domain

import (
	"github.com/google/uuid"
)

type Permission struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type Role struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type RolePermission struct {
	RoleID       uuid.UUID `json:"role_id"`
	PermissionID uuid.UUID `json:"permission_id"`
}

type UserAuthorization struct {
	Roles       []string
	Permissions []string
}
