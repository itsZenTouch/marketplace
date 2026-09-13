package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/repository/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type authorizationRepository struct {
	db DBTX
}

func NewAuthorizationRepository(pool *pgxpool.Pool) *authorizationRepository {
	return &authorizationRepository{
		db: pool,
	}
}

func newAuthorizationRepository(db DBTX) *authorizationRepository {
	return &authorizationRepository{
		db: db,
	}
}

func (a *authorizationRepository) ListUserRoles(
	ctx context.Context,
	userID uuid.UUID,
) ([]domain.Role, error) {
	queries := db.New(a.db)

	userRoles, err := queries.ListUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]domain.Role, 0, len(userRoles))

	for _, userRole := range userRoles {
		result = append(result, listUserRoleToDomain(userRole))
	}

	return result, nil
}

func (a *authorizationRepository) ListUserPermissions(
	ctx context.Context,
	userID uuid.UUID,
) ([]domain.Permission, error) {
	queries := db.New(a.db)

	userPermission, err := queries.ListUserPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]domain.Permission, 0, len(userPermission))

	for _, up := range userPermission {
		result = append(result, listUserPermissions(up))
	}

	return result, nil
}

func (r *authorizationRepository) GetUserAuthorization(
	ctx context.Context,
	userID uuid.UUID,
) (domain.UserAuthorization, error) {
	queries := db.New(r.db)

	result, err := queries.GetUserAuthorization(ctx, userID)
	if err != nil {
		return domain.UserAuthorization{}, err
	}

	return domain.UserAuthorization{
		Status:      domain.UserStatus(result.Status),
		Roles:       result.Roles,
		Permissions: result.Permissions,
	}, nil
}
