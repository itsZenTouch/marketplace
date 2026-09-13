package authorization

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	repository AuthorizationRepository
}

func NewService(repository AuthorizationRepository) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) LoadPrincipal(
	ctx context.Context,
	userID uuid.UUID,
) (Principal, error) {
	authz, err := s.repository.GetUserAuthorization(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Principal{}, ErrUserNotFound
		}

		return Principal{}, err
	}

	switch authz.Status {
	case domain.UserStatusSuspended:
		return Principal{}, ErrUserSuspended

	case domain.UserStatusDisabled:
		return Principal{}, ErrUserDisabled

	case domain.UserStatusActive:
		// Continue loading principal.

	default:
		return Principal{}, ErrInvalidUserStatus
	}

	principal := NewPrincipal(userID)

	return principal.WithAuthorization(
		authz.Roles,
		authz.Permissions,
	), nil
}
