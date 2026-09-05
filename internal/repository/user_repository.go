package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/repository/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userRepository struct {
	db DBTX
}

func NewUserRepository(pool *pgxpool.Pool) *userRepository {
	return &userRepository{
		db: pool,
	}
}

func newUserRepository(dbtx DBTX) *userRepository {
	return &userRepository{
		db: dbtx,
	}
}

func (r *userRepository) CreateUser(
	ctx context.Context,
	input CreateUserInput,
) (domain.User, error) {
	queries := db.New(r.db)

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
	queries := db.New(r.db)

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
	queries := db.New(r.db)

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
	queries := db.New(r.db)

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
	queries := db.New(r.db)

	user, err := queries.ResetFailedLoginAttempts(ctx, id)
	if err != nil {
		return domain.User{}, err
	}

	return userToDomain(user), nil
}

func (r *userRepository) LockUserUntil(
	ctx context.Context,
	id uuid.UUID,
	until *time.Time,
) (domain.User, error) {
	queries := db.New(r.db)

	lockedUntil := pgtype.Timestamptz{}

	if until != nil {
		lockedUntil = pgtype.Timestamptz{
			Time:  *until,
			Valid: true,
		}
	}

	user, err := queries.LockUserUntil(
		ctx,
		db.LockUserUntilParams{
			ID:          id,
			LockedUntil: lockedUntil,
		},
	)
	if err != nil {
		return domain.User{}, err
	}

	return userToDomain(user), nil
}
