package auth

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/platform/password"
	"github.com/itsZenTouch/marketplace/internal/platform/token"
	"github.com/itsZenTouch/marketplace/internal/repository"
)

type mockPasswordHasher struct {
	hashFn func(password string) (string, error)

	hashCalls int
}

func (m *mockPasswordHasher) Hash(password string) (string, error) {
	m.hashCalls++

	if m.hashFn != nil {
		return m.hashFn(password)
	}

	return "hashed-password", nil
}

func (m *mockPasswordHasher) Compare(password, hash string) error {
	return nil
}

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
	var successfulRefresh RefreshOutput

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

			successfulRefresh = result.result

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

	newSessionID, err := token.ParseRefreshSessionID(
		successfulRefresh.RefreshToken,
	)
	if err != nil {
		t.Fatalf("parse new refresh token session ID: %v", err)
	}

	if newSessionID == sessionID {
		t.Fatalf(
			"new session ID reused original session ID: got %s",
			newSessionID,
		)
	}

	newSession, err := sessionRepo.GetAuthSessionByID(ctx, newSessionID)
	if err != nil {
		t.Fatalf("get new auth session after concurrent refresh: %v", err)
	}

	if newSession.FamilyID != familyID {
		t.Fatalf(
			"new session family ID = %s, want %s",
			newSession.FamilyID,
			familyID,
		)
	}

	if newSession.RevokedAt == nil {
		t.Fatal("new session revoked_at = nil, want non-nil after family reuse")
	}

	if newSession.RevocationReason == nil {
		t.Fatal("new session revocation reason = nil, want non-nil")
	}

	if *newSession.RevocationReason != revocationReasonReuse {
		t.Fatalf(
			"new session revocation reason = %q, want %q",
			*newSession.RevocationReason,
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

func TestServiceRefresh_CreatesNewSessionGeneration(t *testing.T) {
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
	email := "auth-refresh-generation-" + userID.String() + "@example.com"

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

	// Generation 1.
	sessionID1 := uuid.New()
	familyID := uuid.New()

	refreshToken1, refreshTokenHash1, err := jwt.CreateRefreshToken(sessionID1)
	if err != nil {
		t.Fatalf("create refresh token #1: %v", err)
	}

	_, err = sessionRepo.CreateAuthSession(
		ctx,
		repository.CreateAuthSessionInput{
			ID:               sessionID1,
			UserID:           userID,
			FamilyID:         familyID,
			RefreshTokenHash: refreshTokenHash1,
			UserAgent:        "test-agent",
			IPAddress:        net.ParseIP("127.0.0.1"),
			ExpiresAt:        time.Now().UTC().Add(24 * time.Hour),
		},
	)
	if err != nil {
		t.Fatalf("create auth session #1: %v", err)
	}

	// Refresh generation 1 -> generation 2.
	result, err := service.Refresh(ctx, RefreshInput{
		RefreshToken: refreshToken1,
	})
	if err != nil {
		t.Fatalf("refresh generation #1: %v", err)
	}

	if result.RefreshToken == "" {
		t.Fatal("refresh generation #1 returned empty refresh token")
	}

	sessionID2, err := token.ParseRefreshSessionID(result.RefreshToken)
	if err != nil {
		t.Fatalf("parse refresh token #2: %v", err)
	}

	if sessionID2 == sessionID1 {
		t.Fatalf(
			"session ID was reused: got %s, want a new session ID",
			sessionID2,
		)
	}

	session1, err := sessionRepo.GetAuthSessionByID(ctx, sessionID1)
	if err != nil {
		t.Fatalf("get session #1: %v", err)
	}

	session2, err := sessionRepo.GetAuthSessionByID(ctx, sessionID2)
	if err != nil {
		t.Fatalf("get session #2: %v", err)
	}

	if session1.ConsumedAt == nil {
		t.Fatal("session #1 was not consumed after rotation")
	}

	if session1.RevokedAt != nil {
		t.Fatal("session #1 should not be revoked after normal rotation")
	}

	if session2.ConsumedAt != nil {
		t.Fatal("session #2 should still be active")
	}

	if session2.FamilyID != familyID {
		t.Fatalf(
			"session #2 family ID = %s, want %s",
			session2.FamilyID,
			familyID,
		)
	}

	if session2.FamilyID != session1.FamilyID {
		t.Fatalf(
			"session family changed across rotation: session #1 = %s, session #2 = %s",
			session1.FamilyID,
			session2.FamilyID,
		)
	}

	if session2.ID == session1.ID {
		t.Fatal("session #2 reused session #1 ID")
	}

	// Refresh generation 2 -> generation 3.
	secondResult, err := service.Refresh(ctx, RefreshInput{
		RefreshToken: result.RefreshToken,
	})
	if err != nil {
		t.Fatalf("refresh generation #2: %v", err)
	}

	sessionID3, err := token.ParseRefreshSessionID(secondResult.RefreshToken)
	if err != nil {
		t.Fatalf("parse refresh token #3: %v", err)
	}

	if sessionID3 == sessionID2 {
		t.Fatalf(
			"session ID was reused on second rotation: got %s, want a new session ID",
			sessionID3,
		)
	}

	if sessionID3 == sessionID1 {
		t.Fatalf(
			"session ID was reused from generation #1: got %s",
			sessionID3,
		)
	}

	session3, err := sessionRepo.GetAuthSessionByID(ctx, sessionID3)
	if err != nil {
		t.Fatalf("get session #3: %v", err)
	}

	session2AfterRotation, err := sessionRepo.GetAuthSessionByID(
		ctx,
		sessionID2,
	)
	if err != nil {
		t.Fatalf("get session #2 after second rotation: %v", err)
	}

	if session2AfterRotation.ConsumedAt == nil {
		t.Fatal("session #2 was not consumed after second rotation")
	}

	if session3.ConsumedAt != nil {
		t.Fatal("session #3 should still be active")
	}

	if session3.FamilyID != familyID {
		t.Fatalf(
			"session #3 family ID = %s, want %s",
			session3.FamilyID,
			familyID,
		)
	}

	if session3.FamilyID != session2AfterRotation.FamilyID {
		t.Fatalf(
			"session family changed on second rotation: session #2 = %s, session #3 = %s",
			session2AfterRotation.FamilyID,
			session3.FamilyID,
		)
	}
}

func TestService_Register(t *testing.T) {
	t.Parallel()

	errHashFailed := errors.New("hash failed")
	errRepositoryFailed := errors.New("repository failed")

	userID := uuid.New()

	tests := []struct {
		name string

		input RegisterInput

		hashResult string
		hashErr    error

		createUserResult domain.User
		createUserErr    error

		wantErr error

		wantCreateUserCalls int

		wantEmail  string
		wantStatus domain.UserStatus
	}{
		{
			name: "valid credentials",
			input: RegisterInput{
				Email:    "  USER@Example.COM  ",
				Password: "password123",
			},
			hashResult: "hashed-password",
			createUserResult: domain.User{
				ID:     userID,
				Email:  "user@example.com",
				Status: domain.UserStatusActive,
			},

			wantCreateUserCalls: 1,
			wantEmail:           "user@example.com",
			wantStatus:          domain.UserStatusActive,
		},
		{
			name: "empty email",
			input: RegisterInput{
				Email:    "",
				Password: "password123",
			},

			wantErr:             ErrInvalidCredentials,
			wantCreateUserCalls: 0,
		},
		{
			name: "whitespace email",
			input: RegisterInput{
				Email:    "   ",
				Password: "password123",
			},

			wantErr:             ErrInvalidCredentials,
			wantCreateUserCalls: 0,
		},
		{
			name: "empty password",
			input: RegisterInput{
				Email:    "user@example.com",
				Password: "",
			},

			wantErr:             ErrInvalidCredentials,
			wantCreateUserCalls: 0,
		},
		{
			name: "hash failure",
			input: RegisterInput{
				Email:    "user@example.com",
				Password: "password123",
			},
			hashErr: errHashFailed,

			wantErr:             errHashFailed,
			wantCreateUserCalls: 0,
		},
		{
			name: "duplicate email",
			input: RegisterInput{
				Email:    "user@example.com",
				Password: "password123",
			},
			createUserErr: domain.ErrUserEmailAlreadyExists,

			wantErr:             ErrEmailAlreadyExists,
			wantCreateUserCalls: 1,
		},
		{
			name: "repository failure",
			input: RegisterInput{
				Email:    "user@example.com",
				Password: "password123",
			},
			createUserErr: errRepositoryFailed,

			wantErr:             errRepositoryFailed,
			wantCreateUserCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := &mockUserRepository{
				createUserFn: func(
					ctx context.Context,
					input repository.CreateUserInput,
				) (domain.User, error) {
					if tt.createUserErr != nil {
						return domain.User{}, tt.createUserErr
					}

					result := tt.createUserResult
					result.ID = input.ID

					return result, nil
				},
			}

			hasher := &mockPasswordHasher{
				hashFn: func(rawPassword string) (string, error) {
					return tt.hashResult, tt.hashErr
				},
			}

			svc := &Service{
				users:    repo,
				password: hasher,
				logger:   slog.Default(),
			}

			got, err := svc.Register(
				context.Background(),
				tt.input,
			)

			// Error assertion.
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}

				if !errors.Is(err, tt.wantErr) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.wantErr,
						err,
					)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}

			// Repository call count.
			if repo.createUserCalls != tt.wantCreateUserCalls {
				t.Fatalf(
					"CreateUser calls = %d, want %d",
					repo.createUserCalls,
					tt.wantCreateUserCalls,
				)
			}

			// Invalid input / hash failure must stop before repository.
			if tt.wantCreateUserCalls == 0 {
				if got != (RegisterOutput{}) {
					t.Fatalf(
						"expected empty output, got %+v",
						got,
					)
				}

				return
			}

			// Repository error must return empty output.
			if tt.wantErr != nil {
				if got != (RegisterOutput{}) {
					t.Fatalf(
						"expected empty output on error, got %+v",
						got,
					)
				}

				return
			}

			// Success assertions.
			if got.User.ID == uuid.Nil {
				t.Error("expected generated UUID, got uuid.Nil")
			}

			if repo.createUserInput.ID == uuid.Nil {
				t.Error("expected generated UUID, got uuid.Nil")
			}

			if got.User.ID != repo.createUserInput.ID {
				t.Errorf(
					"returned user ID = %v, repository ID = %v",
					got.User.ID,
					repo.createUserInput.ID,
				)
			}

			if repo.createUserInput.Email != tt.wantEmail {
				t.Errorf(
					"email = %q, want %q",
					repo.createUserInput.Email,
					tt.wantEmail,
				)
			}

			if repo.createUserInput.PasswordHash != "hashed-password" {
				t.Errorf(
					"password hash = %q, want %q",
					repo.createUserInput.PasswordHash,
					"hashed-password",
				)
			}

			if repo.createUserInput.PasswordHash == tt.input.Password {
				t.Error("password must not be stored as plaintext")
			}

			if repo.createUserInput.Status != tt.wantStatus {
				t.Errorf(
					"status = %q, want %q",
					repo.createUserInput.Status,
					tt.wantStatus,
				)
			}

			if got.User.Email != tt.wantEmail {
				t.Errorf(
					"returned email = %q, want %q",
					got.User.Email,
					tt.wantEmail,
				)
			}

			if got.User.Status != tt.wantStatus {
				t.Errorf(
					"returned status = %q, want %q",
					got.User.Status,
					tt.wantStatus,
				)
			}

			if hasher.hashCalls != 1 {
				t.Errorf(
					"Hash calls = %d, want 1",
					hasher.hashCalls,
				)
			}
		})
	}
}

func TestService_Register_EmailNormalization(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	tests := []struct {
		name          string
		inputEmail    string
		createUserErr error
		wantEmail     string
		wantErr       error
	}{
		{
			name:       "trim and lowercase",
			inputEmail: "  USER@Example.COM  ",
			wantEmail:  "user@example.com",
		},
		{
			name:          "duplicate email with different casing",
			inputEmail:    "  USER@EXAMPLE.COM  ",
			createUserErr: domain.ErrUserEmailAlreadyExists,
			wantEmail:     "user@example.com",
			wantErr:       ErrEmailAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotInput repository.CreateUserInput
			var createCalls int

			repo := &mockUserRepository{
				createUserFn: func(
					ctx context.Context,
					input repository.CreateUserInput,
				) (domain.User, error) {
					createCalls++
					gotInput = input

					if tt.createUserErr != nil {
						return domain.User{}, tt.createUserErr
					}

					return domain.User{
						ID:     userID,
						Email:  input.Email,
						Status: domain.UserStatusActive,
					}, nil
				},
			}

			svc := &Service{
				users:    repo,
				password: password.NewHasher(),
				logger:   slog.Default(),
			}

			got, err := svc.Register(
				context.Background(),
				RegisterInput{
					Email:    tt.inputEmail,
					Password: "password123",
				},
			)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}

				if !errors.Is(err, tt.wantErr) {
					t.Fatalf(
						"expected error %v, got %v",
						tt.wantErr,
						err,
					)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				if got.User.Email != tt.wantEmail {
					t.Errorf(
						"returned email = %q, want %q",
						got.User.Email,
						tt.wantEmail,
					)
				}
			}

			if createCalls != 1 {
				t.Fatalf(
					"CreateUser calls = %d, want 1",
					createCalls,
				)
			}

			if gotInput.Email != tt.wantEmail {
				t.Errorf(
					"CreateUser email = %q, want %q",
					gotInput.Email,
					tt.wantEmail,
				)
			}
		})
	}
}

func TestService_Login_EmailNormalization(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	hasher := password.NewHasher()

	passwordHash, err := hasher.Hash("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	var gotEmail string

	repo := &mockUserRepository{
		getUserByEmailFn: func(
			ctx context.Context,
			email string,
		) (domain.User, error) {
			gotEmail = email

			return domain.User{
				ID:           userID,
				Email:        "user@example.com",
				PasswordHash: passwordHash,
				Status:       domain.UserStatusActive,
			}, nil
		},
	}

	// Login akan masuk ke transaction setelah password benar.
	// Karena test ini hanya ingin memverifikasi normalisasi email,
	// transaction dibuat sebagai no-op.
	uow := &mockUnitOfWorkManager{
		withTxFn: func(
			ctx context.Context,
			fn func(repository.UnitOfWork) error,
		) error {
			return nil
		},
	}

	svc := &Service{
		users:    repo,
		uow:      uow,
		password: hasher,
		logger:   slog.Default(),
	}

	_, err = svc.Login(
		context.Background(),
		LoginInput{
			Email:    "  USER@Example.COM  ",
			Password: "password123",
		},
	)
	if err != nil {
		t.Fatalf("expected login to succeed, got %v", err)
	}

	if gotEmail != "user@example.com" {
		t.Errorf(
			"GetUserByEmail email = %q, want %q",
			gotEmail,
			"user@example.com",
		)
	}
}

func TestService_Login_CaseInsensitiveEmail(t *testing.T) {
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
	email := "auth-case-" + userID.String() + "@example.com"

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

	// Login menggunakan casing berbeda + whitespace.
	loginEmail := "  AUTH-CASE-" + strings.ToUpper(userID.String()) + "@EXAMPLE.COM  "

	result, err := service.Login(
		ctx,
		LoginInput{
			Email:    loginEmail,
			Password: "correct-password",
		},
	)
	if err != nil {
		t.Fatalf(
			"login with different email casing failed: %v",
			err,
		)
	}

	if result.User.ID != userID {
		t.Fatalf(
			"logged-in user ID = %v, want %v",
			result.User.ID,
			userID,
		)
	}

	if result.User.Email != email {
		t.Fatalf(
			"logged-in user email = %q, want %q",
			result.User.Email,
			email,
		)
	}

	if result.AccessToken == "" {
		t.Fatal("access token is empty")
	}

	if result.RefreshToken == "" {
		t.Fatal("refresh token is empty")
	}
}
