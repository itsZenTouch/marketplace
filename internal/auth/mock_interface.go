package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/repository"
)

type mockUnitOfWork struct {
	users        repository.UserRepository
	authSessions repository.AuthSessionRepository
}

func (m *mockUnitOfWork) Users() repository.UserRepository {
	return m.users
}

func (m *mockUnitOfWork) AuthSessions() repository.AuthSessionRepository {
	return m.authSessions
}

type mockUnitOfWorkManager struct {
	withTxFn func(
		ctx context.Context,
		fn func(repository.UnitOfWork) error,
	) error
}

func (m *mockUnitOfWorkManager) WithTx(
	ctx context.Context,
	fn func(repository.UnitOfWork) error,
) error {
	if m.withTxFn != nil {
		return m.withTxFn(ctx, fn)
	}

	return nil
}

type mockUserRepository struct {
	createUserFn func(
		ctx context.Context,
		input repository.CreateUserInput,
	) (domain.User, error)

	createUserCalls int
	createUserInput repository.CreateUserInput

	getUserByEmailFn func(
		ctx context.Context,
		email string,
	) (domain.User, error)
}

func (m *mockUserRepository) CreateUser(
	ctx context.Context,
	input repository.CreateUserInput,
) (domain.User, error) {
	m.createUserCalls++
	m.createUserInput = input

	if m.createUserFn != nil {
		return m.createUserFn(ctx, input)
	}

	return domain.User{}, nil
}

func (m *mockUserRepository) GetUserByEmail(
	ctx context.Context,
	email string,
) (domain.User, error) {
	if m.getUserByEmailFn != nil {
		return m.getUserByEmailFn(ctx, email)
	}

	return domain.User{}, nil
}

func (m *mockUserRepository) GetUserByID(
	ctx context.Context,
	id uuid.UUID,
) (domain.User, error) {
	return domain.User{}, nil
}

func (m *mockUserRepository) RegisterFailedLogin(
	ctx context.Context,
	userID uuid.UUID,
) (domain.User, error) {
	return domain.User{}, nil
}

func (m *mockUserRepository) ResetFailedLoginAttempts(
	ctx context.Context,
	userID uuid.UUID,
	failedLoginAttempts int32,
) (domain.User, error) {
	return domain.User{}, nil
}
