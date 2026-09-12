package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type (
	UserStatus string
)

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusDisabled  UserStatus = "disabled"
)

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrUserEmailAlreadyExists = errors.New("user email already exists")
)

type User struct {
	ID                  uuid.UUID
	Email               string
	PasswordHash        string
	Status              UserStatus
	EmailVerifiedAt     *time.Time
	FailedLoginAttempts int
	LockedUntil         *time.Time
}
