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
}
