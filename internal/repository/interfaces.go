package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/itsZenTouch/marketplace/internal/repository/db"
)

type UserRepository interface {
	CreateUser(
		ctx context.Context,
		arg db.CreateUserParams,
	) (db.User, error)

	GetUserByID(
		ctx context.Context,
		id uuid.UUID,
	) (db.User, error)

	GetUserByEmail(
		ctx context.Context,
		email string,
	) (db.User, error)

	IncrementFailedLoginAttempts(
		ctx context.Context,
		id uuid.UUID,
	) (db.User, error)

	ResetFailedLoginAttempts(
		ctx context.Context,
		id uuid.UUID,
	) (db.User, error)
}
