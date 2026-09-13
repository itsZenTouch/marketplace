package authorization

import (
	"context"

	"github.com/google/uuid"
)

type UserAuthorization struct {
	Roles       []string
	Permissions []string
}

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
		return Principal{}, err
	}

	principal := NewPrincipal(userID)
	principal.Roles = append([]string(nil), authz.Roles...)
	principal.Permissions = append([]string(nil), authz.Permissions...)

	return principal, nil
}
