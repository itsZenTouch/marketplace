package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/repository/db"
)

type authSessionRepository struct {
	db DBTX
}

func NewAuthSessionRepository(pool *pgxpool.Pool) *authSessionRepository {
	return &authSessionRepository{
		db: pool,
	}
}

func newAuthSessionRepository(dbtx DBTX) *authSessionRepository {
	return &authSessionRepository{
		db: dbtx,
	}
}

func (r *authSessionRepository) CreateAuthSession(
	ctx context.Context,
	input CreateAuthSessionInput,
) (domain.AuthSession, error) {
	queries := db.New(r.db)

	session, err := queries.CreateAuthSession(ctx, db.CreateAuthSessionParams{
		ID:               input.ID,
		UserID:           input.UserID,
		FamilyID:         input.FamilyID,
		RefreshTokenHash: input.RefreshTokenHash,
		UserAgent:        stringPtr(input.UserAgent),
		IpAddress:        input.IPAddress,
		ExpiresAt:        input.ExpiresAt,
	})
	if err != nil {
		return domain.AuthSession{}, err
	}

	return createAuthSessionToDomain(session), nil
}

func (r *authSessionRepository) GetAuthSessionByID(
	ctx context.Context,
	id uuid.UUID,
) (domain.AuthSession, error) {
	queries := db.New(r.db)

	session, err := queries.GetAuthSessionByID(ctx, id)
	if err != nil {
		return domain.AuthSession{}, err
	}

	return getAuthSessionToDomain(session), nil
}

func (r *authSessionRepository) GetActiveAuthSessionByID(
	ctx context.Context,
	id uuid.UUID,
) (domain.AuthSession, error) {
	queries := db.New(r.db)

	session, err := queries.GetActiveAuthSessionByID(ctx, id)
	if err != nil {
		return domain.AuthSession{}, err
	}

	return getActiveAuthSessionToDomain(session), nil
}

func (r *authSessionRepository) RevokeAuthSession(
	ctx context.Context,
	id uuid.UUID,
	reason *string,
) (domain.AuthSession, error) {
	queries := db.New(r.db)

	session, err := queries.RevokeAuthSession(
		ctx,
		db.RevokeAuthSessionParams{
			ID:               id,
			RevocationReason: reason,
		},
	)
	if err != nil {
		return domain.AuthSession{}, err
	}

	return revokeAuthSessionToDomain(session), nil
}

func (r *authSessionRepository) RevokeAuthSessionFamily(
	ctx context.Context,
	familyID uuid.UUID,
	reason *string,
) error {
	queries := db.New(r.db)

	return queries.RevokeAuthSessionFamily(
		ctx,
		db.RevokeAuthSessionFamilyParams{
			FamilyID:         familyID,
			RevocationReason: reason,
		},
	)
}

func (r *authSessionRepository) ListAuthSessionsByUserID(
	ctx context.Context,
	userID uuid.UUID,
) ([]domain.AuthSession, error) {
	queries := db.New(r.db)

	sessions, err := queries.ListAuthSessionsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]domain.AuthSession, 0, len(sessions))

	for _, session := range sessions {
		result = append(result, listAuthSessionToDomain(session))
	}

	return result, nil
}

func (r *authSessionRepository) RotateAuthSession(
	ctx context.Context,
	id uuid.UUID,
	newRefreshTokenHash string,
	expiresAt time.Time,
	expectedRefreshTokenHash string,
) (domain.AuthSession, error) {
	queries := db.New(r.db)

	session, err := queries.RotateAuthSession(
		ctx,
		db.RotateAuthSessionParams{
			ID:                      id,
			NewRefreshTokenHash:     newRefreshTokenHash,
			ExpiresAt:               expiresAt,
			CurrentRefreshTokenHash: expectedRefreshTokenHash,
		},
	)
	if err != nil {
		return domain.AuthSession{}, err
	}

	return rotateAuthSessionToDomain(session), nil
}
