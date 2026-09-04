package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/itsZenTouch/marketplace/internal/repository/db"
)

type AuthSessionRepository interface {
	CreateAuthSession(
		ctx context.Context,
		arg db.CreateAuthSessionParams,
	) (db.AuthSession, error)

	GetAuthSessionByID(
		ctx context.Context,
		id uuid.UUID,
	) (db.AuthSession, error)

	RevokeAuthSession(
		ctx context.Context,
		id uuid.UUID,
	) (db.AuthSession, error)

	ListAuthSessionsByUserID(
		ctx context.Context,
		userID uuid.UUID,
	) ([]db.AuthSession, error)
}

type authSessionRepository struct {
	pool *pgxpool.Pool
}

func NewAuthSessionRepository(pool *pgxpool.Pool) AuthSessionRepository {
	return &authSessionRepository{
		pool: pool,
	}
}

func (r *authSessionRepository) CreateAuthSession(
	ctx context.Context,
	arg db.CreateAuthSessionParams,
) (db.AuthSession, error) {
	queries := db.New(r.pool)

	return queries.CreateAuthSession(ctx, arg)
}

func (r *authSessionRepository) GetAuthSessionByID(
	ctx context.Context,
	id uuid.UUID,
) (db.AuthSession, error) {
	queries := db.New(r.pool)

	return queries.GetAuthSessionByID(ctx, id)
}

func (r *authSessionRepository) RevokeAuthSession(
	ctx context.Context,
	id uuid.UUID,
) (db.AuthSession, error) {
	queries := db.New(r.pool)

	return queries.RevokeAuthSession(ctx, id)
}

func (r *authSessionRepository) ListAuthSessionsByUserID(
	ctx context.Context,
	userID uuid.UUID,
) ([]db.AuthSession, error) {
	queries := db.New(r.pool)

	return queries.ListAuthSessionsByUserID(ctx, userID)
}
