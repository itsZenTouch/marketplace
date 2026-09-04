package repository

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/repository/db"
)

func userToDomain(user db.User) domain.User {
	return domain.User{
		ID:                  user.ID,
		Email:               user.Email,
		Status:              domain.UserStatus(user.Status),
		EmailVerifiedAt:     timestamptzPtr(user.EmailVerifiedAt),
		FailedLoginAttempts: int(user.FailedLoginAttempts),
		LockedUntil:         timestamptzPtr(user.LockedUntil),
	}
}

func authSessionToDomain(session db.AuthSession) domain.AuthSession {
	return domain.AuthSession{
		ID:               session.ID,
		UserID:           session.UserID,
		RefreshTokenHash: session.RefreshTokenHash,
		UserAgent:        session.UserAgent,
		IPAddress:        session.IpAddress,
		ExpiresAt:        session.ExpiresAt,
		RevokedAt:        timestamptzPtr(session.RevokedAt),
	}
}

func timestamptzPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	return &value.Time
}
