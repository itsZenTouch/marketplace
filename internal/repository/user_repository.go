package repository

import (
	"context"

	"github.com/google/uuid"

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
	arg db.CreateUserParams,
) (db.User, error) {
	queries := db.New(r.pool)

	return queries.CreateUser(ctx, arg)
}

func (r *userRepository) GetUserByID(
	ctx context.Context,
	id uuid.UUID,
) (db.User, error) {
	queries := db.New(r.pool)

	return queries.GetUserByID(ctx, id)
}

func (r *userRepository) GetUserByEmail(
	ctx context.Context,
	email string,
) (db.User, error) {
	queries := db.New(r.pool)

	return queries.GetUserByEmail(ctx, email)
}

func (r *userRepository) IncrementFailedLoginAttempts(
	ctx context.Context,
	id uuid.UUID,
) (db.User, error) {
	queries := db.New(r.pool)

	return queries.IncrementFailedLoginAttempts(ctx, id)
}

func (r *userRepository) ResetFailedLoginAttempts(
	ctx context.Context,
	id uuid.UUID,
) (db.User, error) {
	queries := db.New(r.pool)

	return queries.ResetFailedLoginAttempts(ctx, id)
}
