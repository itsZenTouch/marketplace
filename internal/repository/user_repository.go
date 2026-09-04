package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/repository/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *userRepository {
	return &userRepository{
		pool: pool,
	}
}

func (r *userRepository) CreateUser(
	ctx context.Context,
	input CreateUserInput,
) (domain.User, error) {
	queries := db.New(r.pool)

	user, err := queries.CreateUser(ctx, db.CreateUserParams{
		ID:           input.ID,
		Email:        input.Email,
		PasswordHash: input.PasswordHash,
		Status:       string(input.Status),
	})
	if err != nil {
		return domain.User{}, err
	}

	return userToDomain(user), nil
}

func (r *userRepository) GetUserByID(
	ctx context.Context,
	id uuid.UUID,
) (domain.User, error) {
	queries := db.New(r.pool)

	user, err := queries.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, err
	}

	return userToDomain(user), nil
}

func (r *userRepository) GetUserByEmail(
	ctx context.Context,
	email string,
) (domain.User, error) {
	queries := db.New(r.pool)

	user, err := queries.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.User{}, err
	}

	return userToDomain(user), nil
}

func (r *userRepository) IncrementFailedLoginAttempts(
	ctx context.Context,
	id uuid.UUID,
) (domain.User, error) {
	queries := db.New(r.pool)

	user, err := queries.IncrementFailedLoginAttempts(ctx, id)
	if err != nil {
		return domain.User{}, err
	}

	return userToDomain(user), nil
}

func (r *userRepository) ResetFailedLoginAttempts(
	ctx context.Context,
	id uuid.UUID,
) (domain.User, error) {
	queries := db.New(r.pool)

	user, err := queries.ResetFailedLoginAttempts(ctx, id)
	if err != nil {
		return domain.User{}, err
	}

	return userToDomain(user), nil
}
