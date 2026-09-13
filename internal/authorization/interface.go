package authorization

import (
	"context"

	"github.com/google/uuid"
	"github.com/itsZenTouch/marketplace/internal/domain"
)

type AuthorizationRepository interface {
	GetUserAuthorization(
		ctx context.Context,
		userID uuid.UUID,
	) (domain.UserAuthorization, error)
}

type PrincipalLoader interface {
	LoadPrincipal(
		ctx context.Context,
		userID uuid.UUID,
	) (Principal, error)
}

type Repository interface {
	ListUserRoles(
		ctx context.Context,
		userID uuid.UUID,
	) ([]domain.Role, error)

	ListUserPermissions(
		ctx context.Context,
		userID uuid.UUID,
	) ([]domain.Permission, error)
}
