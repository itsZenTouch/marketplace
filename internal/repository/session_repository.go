package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/repository/db"
)

type authSessionRepository struct {
	pool *pgxpool.Pool
}

func NewAuthSessionRepository(pool *pgxpool.Pool) *authSessionRepository {
	return &authSessionRepository{
		pool: pool,
	}
}

func (r *authSessionRepository) CreateAuthSession(
	ctx context.Context,
	input CreateAuthSessionInput,
) (domain.AuthSession, error) {
	queries := db.New(r.pool)

	session, err := queries.CreateAuthSession(ctx, db.CreateAuthSessionParams{
		ID:               input.ID,
		UserID:           input.UserID,
		RefreshTokenHash: input.RefreshTokenHash,
		UserAgent:        input.UserAgent,
		IpAddress:        input.IPAddress,
		ExpiresAt:        input.ExpiresAt,
	})
	if err != nil {
		return domain.AuthSession{}, err
	}

	return authSessionToDomain(session), nil
}

func (r *authSessionRepository) GetAuthSessionByID(
	ctx context.Context,
	id uuid.UUID,
) (domain.AuthSession, error) {
	queries := db.New(r.pool)

	session, err := queries.GetAuthSessionByID(ctx, id)
	if err != nil {
		return domain.AuthSession{}, err
	}

	return authSessionToDomain(session), nil
}

func (r *authSessionRepository) RevokeAuthSession(
	ctx context.Context,
	id uuid.UUID,
) (domain.AuthSession, error) {
	queries := db.New(r.pool)

	session, err := queries.RevokeAuthSession(ctx, id)
	if err != nil {
		return domain.AuthSession{}, err
	}

	return authSessionToDomain(session), nil
}

func (r *authSessionRepository) ListAuthSessionsByUserID(
	ctx context.Context,
	userID uuid.UUID,
) ([]domain.AuthSession, error) {
	queries := db.New(r.pool)

	sessions, err := queries.ListAuthSessionsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]domain.AuthSession, 0, len(sessions))

	for _, session := range sessions {
		result = append(result, authSessionToDomain(session))
	}

	return result, nil
}
