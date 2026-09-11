package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/platform/password"
	"github.com/itsZenTouch/marketplace/internal/platform/token"
	"github.com/itsZenTouch/marketplace/internal/repository"
)

type mockAuthService struct {
	registerFunc func(
		ctx context.Context,
		input RegisterInput,
	) (RegisterOutput, error)

	loginFunc func(
		ctx context.Context,
		input LoginInput,
	) (LoginOutput, error)

	getMeFunc func(
		ctx context.Context,
		userID uuid.UUID,
	) (domain.User, error)

	refreshFunc func(
		ctx context.Context,
		input RefreshInput,
	) (RefreshOutput, error)

	logoutFunc func(
		ctx context.Context,
		input LogoutInput,
	) error

	listSessionsFunc func(
		ctx context.Context,
		userID uuid.UUID,
	) ([]domain.AuthSession, error)

	registerCalls int
	loginCalls    int
	getMeCalls    int
	refreshCalls  int
	logoutCalls   int
	sessionsCalls int

	registerInput RegisterInput
	loginInput    LoginInput
	getMeUserID   uuid.UUID
	refreshInput  RefreshInput
	logoutInput   LogoutInput
}

func (m *mockAuthService) Register(
	ctx context.Context,
	input RegisterInput,
) (RegisterOutput, error) {
	m.registerCalls++
	m.registerInput = input

	if m.registerFunc != nil {
		return m.registerFunc(ctx, input)
	}

	return RegisterOutput{}, nil
}

func (m *mockAuthService) Login(
	ctx context.Context,
	input LoginInput,
) (LoginOutput, error) {
	m.loginCalls++
	m.loginInput = input

	if m.loginFunc != nil {
		return m.loginFunc(ctx, input)
	}

	return LoginOutput{}, nil
}

func (m *mockAuthService) GetMe(
	ctx context.Context,
	userID uuid.UUID,
) (domain.User, error) {
	m.getMeCalls++
	m.getMeUserID = userID

	if m.getMeFunc != nil {
		return m.getMeFunc(ctx, userID)
	}

	return domain.User{}, nil
}

func (m *mockAuthService) Refresh(
	ctx context.Context,
	input RefreshInput,
) (RefreshOutput, error) {
	m.refreshCalls++
	m.refreshInput = input

	if m.refreshFunc != nil {
		return m.refreshFunc(ctx, input)
	}

	return RefreshOutput{}, nil
}

func (m *mockAuthService) Logout(
	ctx context.Context,
	input LogoutInput,
) error {
	m.logoutCalls++
	m.logoutInput = input

	if m.logoutFunc != nil {
		return m.logoutFunc(ctx, input)
	}

	return nil
}

func (m *mockAuthService) ListSessions(
	ctx context.Context,
	userID uuid.UUID,
) ([]domain.AuthSession, error) {
	m.sessionsCalls++
	m.getMeUserID = userID

	if m.listSessionsFunc != nil {
		return m.listSessionsFunc(ctx, userID)
	}

	return nil, nil
}

var _ AuthService = (*mockAuthService)(nil)

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

func TestHandlerLogout(t *testing.T) {
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
	authSessionRepo := repository.NewAuthSessionRepository(pool)
	userRepo := repository.NewUserRepository(pool)

	passwordHasher := password.NewHasher()

	passwordHash, err := passwordHasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	userID := uuid.New()
	email := "handler-logout-" + userID.String() + "@example.com"

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
		authSessionRepo,
		repo,
		passwordHasher,
		jwt,
		slog.Default(),
	)

	handler := NewHandler(service)

	// Login first so we get a real refresh token.
	loginReq := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/login",
		strings.NewReader(`{
			"email":"`+email+`",
			"password":"correct-password"
		}`),
	)
	loginReq.Header.Set("Content-Type", "application/json")

	loginRec := httptest.NewRecorder()

	handler.Login(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf(
			"logout status = %d, want %d; body=%s",
			loginRec.Code,
			http.StatusOK,
			loginRec.Body.String(),
		)
	}

	var loginResult struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.Unmarshal(
		loginRec.Body.Bytes(),
		&loginResult,
	); err != nil {
		t.Fatalf("decode login response: %v", err)
	}

	if loginResult.RefreshToken == "" {
		t.Fatal("refresh token is empty")
	}

	// Logout using the refresh token.
	logoutBody := `{"refresh_token":"` +
		loginResult.RefreshToken +
		`"}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/logout",
		strings.NewReader(logoutBody),
	)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"logout status = %d, want %d; body=%s",
			rec.Code,
			http.StatusNoContent,
			rec.Body.String(),
		)
	}

	// Verify that the session is no longer active.
	sessionID, err := token.ParseRefreshSessionID(
		loginResult.RefreshToken,
	)
	if err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}

	_, err = authSessionRepo.GetActiveAuthSessionByID(ctx, sessionID)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"GetActiveAuthSessionByID error = %v, want pgx.ErrNoRows",
			err,
		)
	}
}

func TestHandlerLogout_InvalidRequest(t *testing.T) {
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

	tests := []struct {
		name string
		body string
	}{
		{
			name: "empty",
			body: `{"refresh_token":""}`,
		},
		{
			name: "missing",
			body: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/auth/logout",
				strings.NewReader(tt.body),
			)
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()

			handler.Logout(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf(
					"status = %d, want %d; body=%s",
					rec.Code,
					http.StatusBadRequest,
					rec.Body.String(),
				)
			}
		})
	}
}

func TestHandlerLogout_InvalidToken(t *testing.T) {
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

	tests := []struct {
		name string
		body string
	}{
		{
			name: "malformed",
			body: `{"refresh_token":"not-a-refresh-token"}`,
		},
		{
			name: "invalid-session-id",
			body: `{"refresh_token":"not-a-uuid.random-token"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/auth/logout",
				strings.NewReader(tt.body),
			)
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()

			handler.Logout(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf(
					"status = %d, want %d; body=%s",
					rec.Code,
					http.StatusUnauthorized,
					rec.Body.String(),
				)
			}
		})
	}
}

func TestHandlerSessions_Unauthorized(t *testing.T) {
	service := &Service{}

	handler := NewHandler(service)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/auth/sessions",
		nil,
	)

	rec := httptest.NewRecorder()

	handler.Sessions(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusUnauthorized,
		)
	}
}

func TestHandlerSessions_Authenticated(t *testing.T) {
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

	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)

	userID := uuid.New()
	otherUserID := uuid.New()

	createUser := func(id uuid.UUID, prefix string) {
		t.Helper()

		passwordHash, err := passwordHasher.Hash("correct-password")
		if err != nil {
			t.Fatalf("hash password: %v", err)
		}

		_, err = userRepo.CreateUser(ctx, repository.CreateUserInput{
			ID:           id,
			Email:        prefix + "-" + id.String() + "@example.com",
			PasswordHash: passwordHash,
			Status:       domain.UserStatusActive,
		})
		if err != nil {
			t.Fatalf("create test user: %v", err)
		}
	}

	createUser(userID, "handler-sessions")
	createUser(otherUserID, "handler-sessions-other")

	defer func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM users WHERE id = ANY($1)",
			[]uuid.UUID{userID, otherUserID},
		)
		if err != nil {
			t.Errorf("cleanup users: %v", err)
		}
	}()

	createSession := func(
		sessionUserID uuid.UUID,
		userAgent string,
		ip string,
	) uuid.UUID {
		t.Helper()

		sessionID := uuid.New()

		_, refreshTokenHash, err := jwt.CreateRefreshToken(sessionID)
		if err != nil {
			t.Fatalf("create refresh token: %v", err)
		}

		_, err = sessionRepo.CreateAuthSession(
			ctx,
			repository.CreateAuthSessionInput{
				ID:               sessionID,
				UserID:           sessionUserID,
				FamilyID:         uuid.New(),
				RefreshTokenHash: refreshTokenHash,
				UserAgent:        userAgent,
				IPAddress:        net.ParseIP(ip),
				ExpiresAt:        time.Now().UTC().Add(24 * time.Hour),
			},
		)
		if err != nil {
			t.Fatalf("create auth session: %v", err)
		}

		return sessionID
	}

	session1ID := createSession(
		userID,
		"test-browser",
		"127.0.0.1",
	)

	session2ID := createSession(
		userID,
		"test-mobile",
		"192.168.1.10",
	)

	// This session must not appear in userID's response.
	createSession(
		otherUserID,
		"other-user-browser",
		"10.0.0.1",
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

	accessToken, err := jwt.CreateAccessToken(userID)
	if err != nil {
		t.Fatalf("create access token: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/auth/sessions",
		nil,
	)

	req.Header.Set(
		"Authorization",
		"Bearer "+accessToken,
	)

	rec := httptest.NewRecorder()

	protectedHandler := AuthMiddleware(jwt)(
		http.HandlerFunc(handler.Sessions),
	)

	protectedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body=%s",
			rec.Code,
			http.StatusOK,
			rec.Body.String(),
		)
	}

	var response sessionsResponse

	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Sessions) != 2 {
		t.Fatalf(
			"session count = %d, want 2",
			len(response.Sessions),
		)
	}

	gotIDs := make(map[string]bool, len(response.Sessions))

	for _, session := range response.Sessions {
		gotIDs[session.ID] = true

		if session.UserAgent == "other-user-browser" {
			t.Fatal("response contains another user's session")
		}
	}

	if !gotIDs[session1ID.String()] {
		t.Fatalf("session %s missing from response", session1ID)
	}

	if !gotIDs[session2ID.String()] {
		t.Fatalf("session %s missing from response", session2ID)
	}
}

func TestHandler_Register(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	tests := []struct {
		name string
		body string

		serviceResult RegisterOutput
		serviceErr    error

		wantStatus int
		wantBody   string
	}{
		{
			name: "success",
			body: `{
				"email": "user@example.com",
				"password": "password123"
			}`,
			serviceResult: RegisterOutput{
				User: domain.User{
					ID:     userID,
					Email:  "user@example.com",
					Status: domain.UserStatusActive,
				},
			},

			wantStatus: http.StatusCreated,
			wantBody: `{
				"user": {
					"id": "` + userID.String() + `",
					"email": "user@example.com",
					"status": "active"
				}
			}`,
		},
		{
			name: "invalid json",
			body: `{"email":`,

			wantStatus: http.StatusBadRequest,
			wantBody: `{
				"error": "invalid request body"
			}`,
		},
		{
			name: "invalid email",
			body: `{
				"email": "not-an-email",
				"password": "password123"
			}`,

			wantStatus: http.StatusBadRequest,
			wantBody: `{
				"error": "invalid request"
			}`,
		},
		{
			name: "password too short",
			body: `{
				"email": "user@example.com",
				"password": "1234567"
			}`,

			wantStatus: http.StatusBadRequest,
			wantBody: `{
				"error": "invalid request"
			}`,
		},
		{
			name: "missing email",
			body: `{
				"password": "password123"
			}`,

			wantStatus: http.StatusBadRequest,
			wantBody: `{
				"error": "invalid request"
			}`,
		},
		{
			name: "duplicate email",
			body: `{
				"email": "user@example.com",
				"password": "password123"
			}`,
			serviceErr: ErrEmailAlreadyExists,

			wantStatus: http.StatusConflict,
			wantBody: `{
				"error": "email already registered"
			}`,
		},
		{
			name: "internal server error",
			body: `{
				"email": "user@example.com",
				"password": "password123"
			}`,
			serviceErr: errors.New("database unavailable"),

			wantStatus: http.StatusInternalServerError,
			wantBody: `{
				"error": "internal server error"
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &mockAuthService{
				registerFunc: func(
					ctx context.Context,
					input RegisterInput,
				) (RegisterOutput, error) {
					return tt.serviceResult, tt.serviceErr
				},
			}

			handler := NewHandler(service)

			req := httptest.NewRequest(
				http.MethodPost,
				"/api/auth/register",
				bytes.NewBufferString(tt.body),
			)

			req.Header.Set(
				"Content-Type",
				"application/json",
			)

			rec := httptest.NewRecorder()

			handler.Register(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body = %s",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			var got any
			if err := json.Unmarshal(
				rec.Body.Bytes(),
				&got,
			); err != nil {
				t.Fatalf(
					"invalid JSON response: %v; body = %s",
					err,
					rec.Body.String(),
				)
			}

			var want any
			if err := json.Unmarshal(
				[]byte(tt.wantBody),
				&want,
			); err != nil {
				t.Fatalf(
					"invalid test JSON: %v",
					err,
				)
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf(
					"body = %#v, want %#v",
					got,
					want,
				)
			}
		})
	}
}
