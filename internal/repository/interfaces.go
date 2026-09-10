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
	FamilyID         uuid.UUID
	RefreshTokenHash string
	UserAgent        string
	IPAddress        net.IP
	ExpiresAt        time.Time
}

type UnitOfWork interface {
	Users() UserRepository
	AuthSessions() AuthSessionRepository
}

type UnitOfWorkManager interface {
	WithTx(
		ctx context.Context,
		fn func(UnitOfWork) error,
	) error
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

	ResetFailedLoginAttempts(
		ctx context.Context,
		id uuid.UUID,
		expectedAttempts int32,
	) (domain.User, error)

	RegisterFailedLogin(
		ctx context.Context,
		id uuid.UUID,
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

	GetActiveAuthSessionByID(
		ctx context.Context,
		id uuid.UUID,
	) (domain.AuthSession, error)

	ConsumeAuthSession(
		ctx context.Context,
		id uuid.UUID,
		expectedRefreshTokenHash string,
	) (domain.AuthSession, error)

	RevokeAuthSession(
		ctx context.Context,
		id uuid.UUID,
		reason *string,
	) (domain.AuthSession, error)

	RevokeAuthSessionFamily(
		ctx context.Context,
		familyID uuid.UUID,
		reason *string,
	) error

	ListAuthSessionsByUserID(
		ctx context.Context,
		userID uuid.UUID,
	) ([]domain.AuthSession, error)
}
