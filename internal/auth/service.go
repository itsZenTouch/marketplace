package auth

import (
	"context"
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
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountSuspended   = errors.New("account suspended")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrAccountLocked      = errors.New("account temporarily locked")
)

type Service struct {
	users    repository.UserRepository
	uow      repository.UnitOfWorkManager
	password *password.Hasher
	token    *token.JWT
	logger   *slog.Logger
}

func NewService(
	users repository.UserRepository,
	uow repository.UnitOfWorkManager,
	passwordHasher *password.Hasher,
	jwt *token.JWT,
	logger *slog.Logger,
) *Service {
	return &Service{
		users:    users,
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

	now := time.Now()

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

	if passwordErr != nil {
		err := s.uow.WithTx(ctx, func(uow repository.UnitOfWork) error {
			users := uow.Users()

			failedUser, err := users.IncrementFailedLoginAttempts(
				ctx,
				user.ID,
			)
			if err != nil {
				return err
			}

			const maxAttempts = 5

			if failedUser.FailedLoginAttempts >= maxAttempts {
				until := now.Add(15 * time.Minute)

				_, err := users.LockUserUntil(
					ctx,
					user.ID,
					&until,
				)
				if err != nil {
					return err
				}
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

		return LoginOutput{}, ErrInvalidCredentials
	}

	var result LoginOutput

	err = s.uow.WithTx(ctx, func(uow repository.UnitOfWork) error {
		users := uow.Users()
		sessions := uow.AuthSessions()

		user, err = users.ResetFailedLoginAttempts(
			ctx,
			user.ID,
		)
		if err != nil {
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
