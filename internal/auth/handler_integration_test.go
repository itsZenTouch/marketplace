package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestHandlerRefresh(t *testing.T) {
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
	email := "handler-refresh-" + userID.String() + "@example.com"

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

	handler := NewHandler(service)

	sessionID := uuid.New()

	refreshToken, refreshTokenHash, err := jwt.CreateRefreshToken(sessionID)
	if err != nil {
		t.Fatalf("create refresh token: %v", err)
	}

	_, err = sessionRepo.CreateAuthSession(
		ctx,
		repository.CreateAuthSessionInput{
			ID:               sessionID,
			UserID:           userID,
			RefreshTokenHash: refreshTokenHash,
			UserAgent:        "test-agent",
			IPAddress:        net.ParseIP("127.0.0.1"),
			ExpiresAt:        time.Now().UTC().Add(24 * time.Hour),
		},
	)
	if err != nil {
		t.Fatalf("create auth session: %v", err)
	}

	body := `{"refresh_token":"` + refreshToken + `"}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/refresh",
		strings.NewReader(body),
	)

	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body=%s",
			rec.Code,
			http.StatusOK,
			rec.Body.String(),
		)
	}

	var response refreshResponse

	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.AccessToken == "" {
		t.Fatal("access_token is empty")
	}

	gotUserID, err := jwt.ParseAccessToken(response.AccessToken)
	if err != nil {
		t.Fatalf("parse returned access token: %v", err)
	}

	if gotUserID != userID {
		t.Fatalf(
			"access token user ID = %v, want %v",
			gotUserID,
			userID,
		)
	}
}

func TestHandlerRefresh_InvalidToken(t *testing.T) {
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

	handler := NewHandler(service)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/refresh",
		strings.NewReader(
			`{"refresh_token":"invalid-token"}`,
		),
	)

	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusUnauthorized,
		)
	}

	var response map[string]string

	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response["error"] != "invalid refresh token" {
		t.Fatalf(
			"error = %q, want %q",
			response["error"],
			"invalid refresh token",
		)
	}
}

func TestHandlerRefresh_InvalidJSON(t *testing.T) {
	handler := NewHandler(nil)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/refresh",
		strings.NewReader(`{"refresh_token":`),
	)

	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.Refresh(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusBadRequest,
		)
	}
}
