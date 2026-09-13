package authorization_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/itsZenTouch/marketplace/internal/authorization"
	"github.com/itsZenTouch/marketplace/internal/domain"
)

type mockAuthorizationRepository struct {
	authz domain.UserAuthorization
	err   error
}

func (m *mockAuthorizationRepository) GetUserAuthorization(
	ctx context.Context,
	userID uuid.UUID,
) (domain.UserAuthorization, error) {
	if m.err != nil {
		return domain.UserAuthorization{}, m.err
	}

	return m.authz, nil
}

func TestService_LoadPrincipal(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	repo := &mockAuthorizationRepository{
		authz: domain.UserAuthorization{
			Roles: []string{
				"admin",
			},
			Permissions: []string{
				"user:read",
				"user:update",
			},
		},
	}

	service := authorization.NewService(repo)

	principal, err := service.LoadPrincipal(
		context.Background(),
		userID,
	)
	if err != nil {
		t.Fatalf("LoadPrincipal: %v", err)
	}

	if principal.UserID != userID {
		t.Fatalf(
			"UserID = %v, want %v",
			principal.UserID,
			userID,
		)
	}

	if !principal.HasRole("admin") {
		t.Fatal("expected admin role")
	}

	if !principal.HasPermission("user:read") {
		t.Fatal("expected user:read permission")
	}

	if !principal.HasPermission("user:update") {
		t.Fatal("expected user:update permission")
	}
}
