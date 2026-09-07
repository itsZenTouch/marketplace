package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/itsZenTouch/marketplace/internal/platform/token"
)

func testJWT() *token.JWT {
	return token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)
}

func TestAuthMiddleware_MissingAuthorization(t *testing.T) {
	handler := AuthMiddleware(testJWT())(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			rec.Code,
		)
	}
}

func TestAuthMiddleware_InvalidAuthorizationHeader(t *testing.T) {
	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)

	handler := AuthMiddleware(jwt)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		}),
	)

	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "missing bearer",
			header: "some-token",
		},
		{
			name:   "missing token",
			header: "Bearer",
		},
		{
			name:   "too many parts",
			header: "Bearer token extra",
		},
		{
			name:   "wrong scheme",
			header: "Basic abc",
		},
		{
			name:   "empty scheme",
			header: " token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodGet,
				"/protected",
				nil,
			)

			req.Header.Set("Authorization", tt.header)

			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf(
					"status = %d, want %d",
					rec.Code,
					http.StatusUnauthorized,
				)
			}
		})
	}
}

func TestAuthMiddleware_ValidAccessToken(t *testing.T) {
	jwt := testJWT()
	userID := uuid.New()

	accessToken, err := jwt.CreateAccessToken(userID)
	if err != nil {
		t.Fatalf("CreateAccessToken: %v", err)
	}

	called := false

	handler := AuthMiddleware(jwt)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true

			gotUserID, ok := UserIDFromContext(r.Context())
			if !ok {
				t.Fatal("user ID missing from context")
			}

			if gotUserID != userID {
				t.Fatalf(
					"user ID = %v, want %v",
					gotUserID,
					userID,
				)
			}

			w.WriteHeader(http.StatusNoContent)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler was not called")
	}

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNoContent,
			rec.Code,
		)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)

	expectedUserID := uuid.New()

	accessToken, err := jwt.CreateAccessToken(expectedUserID)
	if err != nil {
		t.Fatalf("create access token: %v", err)
	}

	handler := AuthMiddleware(jwt)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				t.Fatal("user ID missing from context")
			}

			if userID != expectedUserID {
				t.Fatalf(
					"user ID = %v, want %v",
					userID,
					expectedUserID,
				)
			}

			w.WriteHeader(http.StatusNoContent)
		}),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	req.Header.Set(
		"Authorization",
		"Bearer "+accessToken,
	)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		-1*time.Minute,
		24*time.Hour,
	)

	accessToken, err := jwt.CreateAccessToken(uuid.New())
	if err != nil {
		t.Fatalf("create access token: %v", err)
	}

	handler := AuthMiddleware(jwt)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		}),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	req.Header.Set(
		"Authorization",
		"Bearer "+accessToken,
	)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusUnauthorized,
		)
	}
}
