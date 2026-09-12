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
		PasswordHash:        user.PasswordHash,
		Status:              domain.UserStatus(user.Status),
		EmailVerifiedAt:     timestamptzPtr(user.EmailVerifiedAt),
		FailedLoginAttempts: int(user.FailedLoginAttempts),
		LockedUntil:         timestamptzPtr(user.LockedUntil),
	}
}

func createAuthSessionToDomain(session db.CreateAuthSessionRow) domain.AuthSession {
	return domain.AuthSession{
		ID:               session.ID,
		UserID:           session.UserID,
		FamilyID:         session.FamilyID,
		RefreshTokenHash: session.RefreshTokenHash,
		UserAgent:        stringValue(session.UserAgent),
		IPAddress:        session.IpAddress,
		ExpiresAt:        session.ExpiresAt,
		ConsumedAt:       timestamptzPtr(session.ConsumedAt),
		RevokedAt:        timestamptzPtr(session.RevokedAt),
		RevocationReason: session.RevocationReason,
	}
}

func getAuthSessionToDomain(session db.GetAuthSessionByIDRow) domain.AuthSession {
	return domain.AuthSession{
		ID:               session.ID,
		UserID:           session.UserID,
		FamilyID:         session.FamilyID,
		RefreshTokenHash: session.RefreshTokenHash,
		UserAgent:        stringValue(session.UserAgent),
		IPAddress:        session.IpAddress,
		ExpiresAt:        session.ExpiresAt,
		ConsumedAt:       timestamptzPtr(session.ConsumedAt),
		RevokedAt:        timestamptzPtr(session.RevokedAt),
		RevocationReason: session.RevocationReason,
	}
}

func getActiveAuthSessionToDomain(session db.GetActiveAuthSessionByIDRow) domain.AuthSession {
	return domain.AuthSession{
		ID:               session.ID,
		UserID:           session.UserID,
		FamilyID:         session.FamilyID,
		RefreshTokenHash: session.RefreshTokenHash,
		UserAgent:        stringValue(session.UserAgent),
		IPAddress:        session.IpAddress,
		ExpiresAt:        session.ExpiresAt,
		ConsumedAt:       timestamptzPtr(session.ConsumedAt),
		RevokedAt:        timestamptzPtr(session.RevokedAt),
		RevocationReason: session.RevocationReason,
	}
}

func revokeAuthSessionToDomain(session db.RevokeAuthSessionRow) domain.AuthSession {
	return domain.AuthSession{
		ID:               session.ID,
		UserID:           session.UserID,
		FamilyID:         session.FamilyID,
		RefreshTokenHash: session.RefreshTokenHash,
		UserAgent:        stringValue(session.UserAgent),
		IPAddress:        session.IpAddress,
		ExpiresAt:        session.ExpiresAt,
		ConsumedAt:       timestamptzPtr(session.ConsumedAt),
		RevokedAt:        timestamptzPtr(session.RevokedAt),
		RevocationReason: session.RevocationReason,
	}
}

func listAuthSessionToDomain(session db.ListAuthSessionsByUserIDRow) domain.AuthSession {
	return domain.AuthSession{
		ID:               session.ID,
		UserID:           session.UserID,
		FamilyID:         session.FamilyID,
		RefreshTokenHash: session.RefreshTokenHash,
		UserAgent:        stringValue(session.UserAgent),
		IPAddress:        session.IpAddress,
		ExpiresAt:        session.ExpiresAt,
		ConsumedAt:       timestamptzPtr(session.ConsumedAt),
		RevokedAt:        timestamptzPtr(session.RevokedAt),
		RevocationReason: session.RevocationReason,
	}
}

func consumeAuthSessionToDomain(
	session db.ConsumeAuthSessionRow,
) domain.AuthSession {
	return domain.AuthSession{
		ID:               session.ID,
		UserID:           session.UserID,
		FamilyID:         session.FamilyID,
		RefreshTokenHash: session.RefreshTokenHash,
		UserAgent:        stringValue(session.UserAgent),
		IPAddress:        session.IpAddress,
		ExpiresAt:        session.ExpiresAt,
		ConsumedAt:       timestamptzPtr(session.ConsumedAt),
		RevokedAt:        timestamptzPtr(session.RevokedAt),
		RevocationReason: session.RevocationReason,
	}
}

func listUserRoleToDomain(
	authz db.ListUserRolesRow,
) domain.Role {
	return domain.Role{
		ID:   authz.ID,
		Name: authz.Name,
	}
}

func listUserPermissions(
	authz db.ListUserPermissionsRow,
) domain.Permission {
	return domain.Permission{
		ID:   authz.ID,
		Name: authz.Name,
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func timestamptzPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	return &value.Time
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}
