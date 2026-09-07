package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
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
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrAccountSuspended    = errors.New("account suspended")
	ErrAccountDisabled     = errors.New("account disabled")
	ErrAccountLocked       = errors.New("account temporarily locked")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

type Service struct {
	users    repository.UserRepository
	sessions repository.AuthSessionRepository
	uow      repository.UnitOfWorkManager
	password *password.Hasher
	token    *token.JWT
	logger   *slog.Logger
}

func NewService(
	users repository.UserRepository,
	sessions repository.AuthSessionRepository,
	uow repository.UnitOfWorkManager,
	passwordHasher *password.Hasher,
	jwt *token.JWT,
	logger *slog.Logger,
) *Service {
	return &Service{
		users:    users,
		sessions: sessions,
		uow:      uow,
		password: passwordHasher,
		token:    jwt,
		logger:   logger,
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

type RefreshInput struct {
	RefreshToken string
}

type RefreshOutput struct {
	AccessToken string
}

type LogoutInput struct {
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

		s.logger.ErrorContext(
			ctx,
			"failed to get user by email",
			slog.Any("error", err),
		)

		return LoginOutput{}, err
	}

	now := time.Now().UTC()

	if user.LockedUntil != nil &&
		user.LockedUntil.After(now) {
		return LoginOutput{}, ErrAccountLocked
	}

	switch user.Status {
	case domain.UserStatusSuspended:
		return LoginOutput{}, ErrAccountSuspended

	case domain.UserStatusDisabled:
		return LoginOutput{}, ErrAccountDisabled
	}

	passwordErr := s.password.Compare(
		input.Password,
		user.PasswordHash,
	)

	var failedUser domain.User

	if passwordErr != nil {
		err := s.uow.WithTx(ctx, func(uow repository.UnitOfWork) error {
			users := uow.Users()

			failedUser, err = users.RegisterFailedLogin(
				ctx,
				user.ID,
			)
			if err != nil {
				s.logger.ErrorContext(
					ctx,
					"internal error",
					slog.Any("RegisterFailedLogin", err),
				)
				return err
			}

			return nil
		})
		if err != nil {
			s.logger.ErrorContext(
				ctx,
				"failed to update login failure state",
				slog.String("user_id", user.ID.String()),
				slog.Any("error", err),
			)

			return LoginOutput{}, err
		}

		if failedUser.LockedUntil != nil &&
			failedUser.LockedUntil.After(now) {
			return LoginOutput{}, ErrAccountLocked
		}

		return LoginOutput{}, ErrInvalidCredentials
	}

	var result LoginOutput

	err = s.uow.WithTx(ctx, func(uow repository.UnitOfWork) error {
		users := uow.Users()
		sessions := uow.AuthSessions()

		s.logger.InfoContext(
			ctx,
			"resetting failed login attempts",
			slog.String("user_id", user.ID.String()),
			slog.Int("failed_login_attempts", user.FailedLoginAttempts),
		)

		user, err = users.ResetFailedLoginAttempts(
			ctx,
			user.ID,
			int32(user.FailedLoginAttempts),
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				s.logger.ErrorContext(
					ctx,
					"reset failed login attempts returned no rows",
					slog.String("user_id", user.ID.String()),
					slog.Int("failed_login_attempts", user.FailedLoginAttempts),
				)

				return err
			}

			s.logger.ErrorContext(
				ctx,
				"internal error",
				slog.Any("ResetFailedLoginAttempts", err),
			)
			return err
		}

		sessionID := uuid.New()

		refreshToken, refreshTokenHash, err := s.token.CreateRefreshToken(sessionID)
		if err != nil {
			return err
		}

		_, err = sessions.CreateAuthSession(
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
			return err
		}

		accessToken, err := s.token.CreateAccessToken(user.ID)
		if err != nil {
			return err
		}

		result = LoginOutput{
			User:         user,
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		}

		return nil
	})
	if err != nil {
		return LoginOutput{}, err
	}

	s.logger.InfoContext(
		ctx,
		"user login succeeded",
		slog.String("user_id", result.User.ID.String()),
	)

	return result, nil
}

func (s *Service) GetMe(
	ctx context.Context,
	userID uuid.UUID,
) (domain.User, error) {
	s.logger.InfoContext(ctx,
		"get user status succeeded",
		slog.String("user_id", userID.String()))

	return s.users.GetUserByID(ctx, userID)
}

func (s *Service) Refresh(
	ctx context.Context,
	input RefreshInput,
) (RefreshOutput, error) {
	refreshToken := strings.TrimSpace(input.RefreshToken)

	if refreshToken == "" {
		return RefreshOutput{}, ErrInvalidRefreshToken
	}

	sessionID, err := token.ParseRefreshSessionID(refreshToken)
	if err != nil {
		return RefreshOutput{}, ErrInvalidRefreshToken
	}

	session, err := s.sessions.GetActiveAuthSessionByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RefreshOutput{}, ErrInvalidRefreshToken
		}

		s.logger.ErrorContext(
			ctx,
			"failed to get active auth session",
			slog.String("session_id", sessionID.String()),
			slog.Any("error", err),
		)

		return RefreshOutput{}, err
	}

	providedHash := token.HashRefreshToken(refreshToken)

	if subtle.ConstantTimeCompare(
		[]byte(providedHash),
		[]byte(session.RefreshTokenHash),
	) != 1 {
		return RefreshOutput{}, ErrInvalidRefreshToken
	}

	user, err := s.users.GetUserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RefreshOutput{}, ErrInvalidRefreshToken
		}

		return RefreshOutput{}, err
	}

	switch user.Status {
	case domain.UserStatusSuspended:
		return RefreshOutput{}, ErrAccountSuspended

	case domain.UserStatusDisabled:
		return RefreshOutput{}, ErrAccountDisabled
	}

	accessToken, err := s.token.CreateAccessToken(user.ID)
	if err != nil {
		return RefreshOutput{}, err
	}

	return RefreshOutput{
		AccessToken: accessToken,
	}, nil
}

func (s *Service) Logout(
	ctx context.Context,
	input LogoutInput,
) error {
	refreshToken := strings.TrimSpace(input.RefreshToken)

	if refreshToken == "" {
		return ErrInvalidRefreshToken
	}

	sessionID, err := token.ParseRefreshSessionID(refreshToken)
	if err != nil {
		return ErrInvalidRefreshToken
	}

	session, err := s.sessions.GetActiveAuthSessionByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidRefreshToken
		}

		s.logger.ErrorContext(
			ctx,
			"failed to get active auth session",
			slog.String("session_id", sessionID.String()),
			slog.Any("error", err),
		)

		return err
	}

	providedHash := token.HashRefreshToken(refreshToken)

	if subtle.ConstantTimeCompare(
		[]byte(providedHash),
		[]byte(session.RefreshTokenHash),
	) != 1 {
		return ErrInvalidRefreshToken
	}

	_, err = s.sessions.RevokeAuthSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidRefreshToken
		}

		s.logger.ErrorContext(
			ctx,
			"failed to revoke auth session",
			slog.String("session_id", sessionID.String()),
			slog.Any("error", err),
		)

		return err
	}

	s.logger.InfoContext(
		ctx,
		"user logged out",
		slog.String("session_id", sessionID.String()),
		slog.String("user_id", session.UserID.String()),
	)

	return nil
}
