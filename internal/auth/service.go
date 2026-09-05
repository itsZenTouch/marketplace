package auth

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/platform/password"
	"github.com/itsZenTouch/marketplace/internal/platform/token"
	"github.com/itsZenTouch/marketplace/internal/repository"
	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountSuspended   = errors.New("account suspended")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrAccountLocked      = errors.New("account temporarily locked")
)

type Service struct {
	users    repository.UserRepository
	sessions repository.AuthSessionRepository
	password *password.Hasher
	token    *token.JWT
}

func NewService(
	users repository.UserRepository,
	sessions repository.AuthSessionRepository,
	passwordHasher *password.Hasher,
	jwt *token.JWT,
) *Service {
	return &Service{
		users:    users,
		sessions: sessions,
		password: passwordHasher,
		token:    jwt,
	}
}

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IPAddress net.IP
}

type LoginOutput struct {
	User         domain.User
	AccessToken  string
	RefreshToken string
}

func (s *Service) Login(
	ctx context.Context,
	input LoginInput,
) (LoginOutput, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	if email == "" || input.Password == "" {
		return LoginOutput{}, ErrInvalidCredentials
	}

	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LoginOutput{}, ErrInvalidCredentials
		}

		return LoginOutput{}, err
	}

	now := time.Now()

	if user.LockedUntil != nil && user.LockedUntil.After(now) {
		return LoginOutput{}, ErrAccountLocked
	}

	switch user.Status {
	case domain.UserStatusSuspended:
		return LoginOutput{}, ErrAccountSuspended

	case domain.UserStatusDisabled:
		return LoginOutput{}, ErrAccountDisabled
	}

	if err := s.password.Compare(input.Password, user.PasswordHash); err != nil {
		failedUser, _ := s.users.IncrementFailedLoginAttempts(
			ctx,
			user.ID,
		)

		const maxAttempts = 5

		if failedUser.FailedLoginAttempts >= maxAttempts {
			until := time.Now().Add(15 * time.Minute)

			_, _ = s.users.LockUserUntil(
				ctx,
				user.ID,
				&until,
			)
		}

		return LoginOutput{}, ErrInvalidCredentials
	}

	user, err = s.users.ResetFailedLoginAttempts(ctx, user.ID)
	if err != nil {
		return LoginOutput{}, err
	}

	sessionID := uuid.New()

	refreshToken, refreshTokenHash, err := s.token.CreateRefreshToken(sessionID)
	if err != nil {
		return LoginOutput{}, err
	}

	_, err = s.sessions.CreateAuthSession(
		ctx,
		repository.CreateAuthSessionInput{
			ID:               sessionID,
			UserID:           user.ID,
			RefreshTokenHash: refreshTokenHash,
			UserAgent:        input.UserAgent,
			IPAddress:        input.IPAddress,
			ExpiresAt:        now.Add(s.token.RefreshTTL()),
		},
	)
	if err != nil {
		return LoginOutput{}, err
	}

	accessToken, err := s.token.CreateAccessToken(user.ID)
	if err != nil {
		return LoginOutput{}, err
	}

	return LoginOutput{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
