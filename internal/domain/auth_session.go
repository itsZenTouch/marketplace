package domain

import (
	"net"
	"time"

	"github.com/google/uuid"
)

type AuthSession struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	RefreshTokenHash string
	UserAgent        string
	IPAddress        net.IP
	ExpiresAt        time.Time
	RevokedAt        *time.Time
}
