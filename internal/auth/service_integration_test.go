package auth

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/platform/password"
	"github.com/itsZenTouch/marketplace/internal/platform/token"
	"github.com/itsZenTouch/marketplace/internal/repository"
)

func TestServiceLoginAccountLocking(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run database integration tests")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	repo := repository.NewRepository(pool)
	userRepo := repository.NewUserRepository(pool)

	passwordHasher := password.NewHasher()

	passwordHash, err := passwordHasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	userID := uuid.New()
	email := "auth-lock-" + userID.String() + "@example.com"

	_, err = userRepo.CreateUser(ctx, repository.CreateUserInput{
		ID:           userID,
		Email:        email,
		PasswordHash: passwordHash,
		Status:       domain.UserStatusActive,
	})
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	defer func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM users WHERE id = $1",
			userID,
		)
		if err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	}()

	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)

	service := NewService(
		userRepo,
		repository.NewAuthSessionRepository(pool),
		repo,
		passwordHasher,
		jwt,
		slog.Default(),
	)

	input := LoginInput{
		Email:    email,
		Password: "wrong-password",
	}

	for attempt := 1; attempt <= 5; attempt++ {
		_, err := service.Login(ctx, input)

		if attempt < 5 {
			if err != ErrInvalidCredentials {
				t.Fatalf(
					"attempt %d: error = %v, want %v",
					attempt,
					err,
					ErrInvalidCredentials,
				)
			}
		} else {
			if err != ErrAccountLocked {
				t.Fatalf(
					"attempt %d: error = %v, want %v",
					attempt,
					err,
					ErrAccountLocked,
				)
			}
		}
	}

	user, err := userRepo.GetUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("get user after lock: %v", err)
	}

	if user.FailedLoginAttempts != 5 {
		t.Fatalf(
			"failed login attempts = %d, want 5",
			user.FailedLoginAttempts,
		)
	}

	if user.LockedUntil == nil {
		t.Fatal("locked_until = nil, want non-nil")
	}

	if !user.LockedUntil.After(time.Now().UTC()) {
		t.Fatalf(
			"locked_until = %v, want future time",
			user.LockedUntil,
		)
	}

	// A sixth attempt must not increase the counter
	// and must still return ErrAccountLocked.
	_, err = service.Login(ctx, input)
	if err != ErrAccountLocked {
		t.Fatalf(
			"sixth attempt: error = %v, want %v",
			err,
			ErrAccountLocked,
		)
	}

	user, err = userRepo.GetUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("get user after sixth attempt: %v", err)
	}

	if user.FailedLoginAttempts != 5 {
		t.Fatalf(
			"failed login attempts after sixth attempt = %d, want 5",
			user.FailedLoginAttempts,
		)
	}
}

func TestServiceLoginResetsFailedAttempts(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run database integration tests")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	repo := repository.NewRepository(pool)
	userRepo := repository.NewUserRepository(pool)

	passwordHasher := password.NewHasher()

	passwordHash, err := passwordHasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	userID := uuid.New()
	email := "auth-reset-" + userID.String() + "@example.com"

	_, err = userRepo.CreateUser(ctx, repository.CreateUserInput{
		ID:           userID,
		Email:        email,
		PasswordHash: passwordHash,
		Status:       domain.UserStatusActive,
	})
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	defer func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM users WHERE id = $1",
			userID,
		)
		if err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	}()

	// Create failed-login state first.
	for attempt := 1; attempt <= 3; attempt++ {
		_, err := userRepo.RegisterFailedLogin(ctx, userID)
		if err != nil {
			t.Fatalf(
				"RegisterFailedLogin attempt %d: %v",
				attempt,
				err,
			)
		}
	}

	user, err := userRepo.GetUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("get user before successful login: %v", err)
	}

	if user.FailedLoginAttempts != 3 {
		t.Fatalf(
			"failed login attempts before login = %d, want 3",
			user.FailedLoginAttempts,
		)
	}

	if user.LockedUntil != nil {
		t.Fatalf(
			"locked_until before login = %v, want nil",
			user.LockedUntil,
		)
	}

	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)

	service := NewService(
		userRepo,
		repository.NewAuthSessionRepository(pool),
		repo,
		passwordHasher,
		jwt,
		slog.Default(),
	)

	result, err := service.Login(ctx, LoginInput{
		Email:    email,
		Password: "correct-password",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if result.User.ID != userID {
		t.Fatalf(
			"result user ID = %v, want %v",
			result.User.ID,
			userID,
		)
	}

	if result.AccessToken == "" {
		t.Fatal("access token is empty")
	}

	if result.RefreshToken == "" {
		t.Fatal("refresh token is empty")
	}

	user, err = userRepo.GetUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("get user after successful login: %v", err)
	}

	if user.FailedLoginAttempts != 0 {
		t.Fatalf(
			"failed login attempts after successful login = %d, want 0",
			user.FailedLoginAttempts,
		)
	}

	if user.LockedUntil != nil {
		t.Fatalf(
			"locked_until after successful login = %v, want nil",
			user.LockedUntil,
		)
	}
}

func TestServiceRefresh(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run database integration tests")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	repo := repository.NewRepository(pool)
	userRepo := repository.NewUserRepository(pool)

	passwordHasher := password.NewHasher()

	passwordHash, err := passwordHasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	userID := uuid.New()
	email := "auth-refresh-" + userID.String() + "@example.com"

	_, err = userRepo.CreateUser(ctx, repository.CreateUserInput{
		ID:           userID,
		Email:        email,
		PasswordHash: passwordHash,
		Status:       domain.UserStatusActive,
	})
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	defer func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM users WHERE id = $1",
			userID,
		)
		if err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	}()

	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)

	service := NewService(
		userRepo,
		repository.NewAuthSessionRepository(pool),
		repo,
		passwordHasher,
		jwt,
		slog.Default(),
	)

	sessionID := uuid.New()
	familyID := uuid.New()

	refreshToken, refreshTokenHash, err := jwt.CreateRefreshToken(sessionID)
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	_, err = repository.NewAuthSessionRepository(pool).CreateAuthSession(
		ctx,
		repository.CreateAuthSessionInput{
			ID:               sessionID,
			UserID:           userID,
			FamilyID:         familyID,
			RefreshTokenHash: refreshTokenHash,
			UserAgent:        "test-agent",
			IPAddress:        net.ParseIP("127.0.0.1"),
			ExpiresAt:        time.Now().UTC().Add(24 * time.Hour),
		},
	)
	if err != nil {
		t.Fatalf("create auth session: %v", err)
	}

	// First refresh: token #1 -> token #2.
	result, err := service.Refresh(ctx, RefreshInput{
		RefreshToken: refreshToken,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if result.AccessToken == "" {
		t.Fatal("access token is empty")
	}

	if result.RefreshToken == "" {
		t.Fatal("refresh token is empty")
	}

	// Second refresh: token #2 must still be valid.
	secondResult, err := service.Refresh(ctx, RefreshInput{
		RefreshToken: result.RefreshToken,
	})
	if err != nil {
		t.Fatalf("refresh with rotated token: %v", err)
	}

	if secondResult.AccessToken == "" {
		t.Fatal("second access token is empty")
	}

	if secondResult.RefreshToken == "" {
		t.Fatal("second refresh token is empty")
	}

	// Reuse the original token #1.
	// This must revoke the entire refresh-token family.
	_, err = service.Refresh(ctx, RefreshInput{
		RefreshToken: refreshToken,
	})

	if !errors.Is(err, ErrRefreshTokenReuse) {
		t.Fatalf(
			"reuse Refresh error = %v, want %v",
			err,
			ErrRefreshTokenReuse,
		)
	}

	// After family revocation, token #2 must no longer work.
	_, err = service.Refresh(ctx, RefreshInput{
		RefreshToken: result.RefreshToken,
	})

	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf(
			"refresh after family revocation error = %v, want %v",
			err,
			ErrInvalidRefreshToken,
		)
	}

	// Token #3 must also no longer work.
	_, err = service.Refresh(ctx, RefreshInput{
		RefreshToken: secondResult.RefreshToken,
	})

	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf(
			"latest token after family revocation error = %v, want %v",
			err,
			ErrInvalidRefreshToken,
		)
	}

	gotUserID, err := jwt.ParseAccessToken(result.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	if gotUserID != userID {
		t.Fatalf(
			"access token user ID = %v, want %v",
			gotUserID,
			userID,
		)
	}
}

func TestServiceRefresh_ConcurrentSameToken(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run database integration tests")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	repo := repository.NewRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	sessionRepo := repository.NewAuthSessionRepository(pool)

	passwordHasher := password.NewHasher()

	passwordHash, err := passwordHasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	userID := uuid.New()
	email := "auth-concurrent-refresh-" + userID.String() + "@example.com"

	_, err = userRepo.CreateUser(ctx, repository.CreateUserInput{
		ID:           userID,
		Email:        email,
		PasswordHash: passwordHash,
		Status:       domain.UserStatusActive,
	})
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	defer func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM users WHERE id = $1",
			userID,
		)
		if err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	}()

	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)

	service := NewService(
		userRepo,
		sessionRepo,
		repo,
		passwordHasher,
		jwt,
		slog.Default(),
	)

	sessionID := uuid.New()
	familyID := uuid.New()

	refreshToken, refreshTokenHash, err := jwt.CreateRefreshToken(sessionID)
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	_, err = sessionRepo.CreateAuthSession(
		ctx,
		repository.CreateAuthSessionInput{
			ID:               sessionID,
			UserID:           userID,
			FamilyID:         familyID,
			RefreshTokenHash: refreshTokenHash,
			UserAgent:        "test-agent",
			IPAddress:        net.ParseIP("127.0.0.1"),
			ExpiresAt:        time.Now().UTC().Add(24 * time.Hour),
		},
	)
	if err != nil {
		t.Fatalf("create auth session: %v", err)
	}

	const requestCount = 2

	start := make(chan struct{})

	type refreshResult struct {
		result RefreshOutput
		err    error
	}

	results := make(chan refreshResult, requestCount)

	for range requestCount {
		go func() {
			<-start

			result, err := service.Refresh(
				context.Background(),
				RefreshInput{
					RefreshToken: refreshToken,
				},
			)

			results <- refreshResult{
				result: result,
				err:    err,
			}
		}()
	}

	close(start)

	var successCount int
	var reuseCount int

	for range requestCount {
		result := <-results

		switch {
		case result.err == nil:
			successCount++

			if result.result.AccessToken == "" {
				t.Error("successful refresh returned empty access token")
			}

			if result.result.RefreshToken == "" {
				t.Error("successful refresh returned empty refresh token")
			}

		case errors.Is(result.err, ErrRefreshTokenReuse):
			reuseCount++

		default:
			t.Errorf("unexpected refresh error: %v", result.err)
		}
	}

	if successCount != 1 {
		t.Fatalf(
			"successful refresh count = %d, want 1",
			successCount,
		)
	}

	if reuseCount != 1 {
		t.Fatalf(
			"refresh-token reuse count = %d, want 1",
			reuseCount,
		)
	}

	session, err := sessionRepo.GetAuthSessionByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("get auth session after concurrent refresh: %v", err)
	}

	if session.RevokedAt == nil {
		t.Fatal("session revoked_at = nil, want non-nil")
	}

	if session.RevocationReason == nil {
		t.Fatal("session revocation reason = nil, want non-nil")
	}

	if *session.RevocationReason != revocationReasonReuse {
		t.Fatalf(
			"session revocation reason = %q, want %q",
			*session.RevocationReason,
			revocationReasonReuse,
		)
	}
}

func TestServiceRefresh_InvalidToken(t *testing.T) {
	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)

	service := NewService(
		nil,
		nil,
		nil,
		password.NewHasher(),
		jwt,
		slog.Default(),
	)

	_, err := service.Refresh(context.Background(), RefreshInput{
		RefreshToken: "not-a-valid-refresh-token",
	})

	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf(
			"error = %v, want %v",
			err,
			ErrInvalidRefreshToken,
		)
	}
}

func TestServiceLogout(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run database integration tests")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	repo := repository.NewRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	sessionRepo := repository.NewAuthSessionRepository(pool)

	passwordHasher := password.NewHasher()

	passwordHash, err := passwordHasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	userID := uuid.New()
	email := "auth-logout-" + userID.String() + "@example.com"

	_, err = userRepo.CreateUser(ctx, repository.CreateUserInput{
		ID:           userID,
		Email:        email,
		PasswordHash: passwordHash,
		Status:       domain.UserStatusActive,
	})
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	defer func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM users WHERE id = $1",
			userID,
		)
		if err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	}()

	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)

	service := NewService(
		userRepo,
		sessionRepo,
		repo,
		passwordHasher,
		jwt,
		slog.Default(),
	)

	sessionID := uuid.New()
	familyID := uuid.New()

	refreshToken, refreshTokenHash, err := jwt.CreateRefreshToken(sessionID)
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	_, err = sessionRepo.CreateAuthSession(
		ctx,
		repository.CreateAuthSessionInput{
			ID:               sessionID,
			UserID:           userID,
			FamilyID:         familyID,
			RefreshTokenHash: refreshTokenHash,
			UserAgent:        "test-agent",
			IPAddress:        net.ParseIP("127.0.0.1"),
			ExpiresAt:        time.Now().UTC().Add(24 * time.Hour),
		},
	)
	if err != nil {
		t.Fatalf("create auth session: %v", err)
	}

	err = service.Logout(ctx, LogoutInput{
		RefreshToken: refreshToken,
	})
	if err != nil {
		t.Fatalf("Logout: %v", err)
	}

	session, err := sessionRepo.GetAuthSessionByID(ctx, sessionID)
	if err != nil {
		t.Fatalf("get auth session after logout: %v", err)
	}

	if session.RevokedAt == nil {
		t.Fatal("revoked_at = nil, want non-nil")
	}

	// The same refresh token must no longer work.
	err = service.Logout(ctx, LogoutInput{
		RefreshToken: refreshToken,
	})
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf(
			"second Logout error = %v, want %v",
			err,
			ErrInvalidRefreshToken,
		)
	}
}

func TestServiceLogout_InvalidToken(t *testing.T) {
	service := &Service{}

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "empty",
			token: "",
		},
		{
			name:  "malformed",
			token: "not-a-refresh-token",
		},
		{
			name:  "invalid-session-id",
			token: "not-a-uuid.random-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.Logout(
				context.Background(),
				LogoutInput{
					RefreshToken: tt.token,
				},
			)

			if !errors.Is(err, ErrInvalidRefreshToken) {
				t.Fatalf(
					"error = %v, want %v",
					err,
					ErrInvalidRefreshToken,
				)
			}
		})
	}
}
