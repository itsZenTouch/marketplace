package repository

import (
	"context"
	"net"
	"time"

	"github.com/google/uuid"

	"github.com/itsZenTouch/marketplace/internal/domain"
)

type CreateUserInput struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Status       domain.UserStatus
}

type CreateAuthSessionInput struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	RefreshTokenHash string
	UserAgent        string
	IPAddress        net.IP
	ExpiresAt        time.Time
}

type UnitOfWork interface {
	Users() *userRepository
	AuthSessions() *authSessionRepository
}

type UserRepository interface {
	CreateUser(
		ctx context.Context,
		input CreateUserInput,
	) (domain.User, error)

	GetUserByID(
		ctx context.Context,
		id uuid.UUID,
	) (domain.User, error)

	GetUserByEmail(
		ctx context.Context,
		email string,
	) (domain.User, error)

	IncrementFailedLoginAttempts(
		ctx context.Context,
		id uuid.UUID,
	) (domain.User, error)

	ResetFailedLoginAttempts(
		ctx context.Context,
		id uuid.UUID,
	) (domain.User, error)

	LockUserUntil(
		ctx context.Context,
		id uuid.UUID,
		until *time.Time,
	) (domain.User, error)
}

type AuthSessionRepository interface {
	CreateAuthSession(
		ctx context.Context,
		input CreateAuthSessionInput,
	) (domain.AuthSession, error)

	GetAuthSessionByID(
		ctx context.Context,
		id uuid.UUID,
	) (domain.AuthSession, error)

	RevokeAuthSession(
		ctx context.Context,
		id uuid.UUID,
	) (domain.AuthSession, error)

	ListAuthSessionsByUserID(
		ctx context.Context,
		userID uuid.UUID,
	) ([]domain.AuthSession, error)
}
