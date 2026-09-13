package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/itsZenTouch/marketplace/internal/authorization"
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
	t.Parallel()

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

			principal, ok := authorization.PrincipalFromContext(r.Context())
			if !ok {
				t.Fatal("principal missing from context")
			}

			if principal.UserID != userID {
				t.Fatalf(
					"user ID = %v, want %v",
					principal.UserID,
					userID,
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
			principal, ok := authorization.PrincipalFromContext(r.Context())
			if !ok {
				t.Fatal("principal missing from context")
			}

			if principal.UserID != expectedUserID {
				t.Fatalf(
					"user ID = %v, want %v",
					principal.UserID,
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

func TestAuthMiddlewareSetsPrincipal(t *testing.T) {
	t.Parallel()

	jwt := testJWT()
	userID := uuid.New()

	accessToken, err := jwt.CreateAccessToken(userID)
	if err != nil {
		t.Fatalf("create access token: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorization.PrincipalFromContext(r.Context())
		if !ok {
			t.Fatal("expected principal in context")
		}

		if principal.UserID != userID {
			t.Fatalf(
				"expected user ID %s, got %s",
				userID,
				principal.UserID,
			)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	handler := AuthMiddleware(jwt)(next)

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

type fakePrincipalLoader struct {
	principal authorization.Principal
	err       error
}

func (f fakePrincipalLoader) LoadPrincipal(
	ctx context.Context,
	userID uuid.UUID,
) (authorization.Principal, error) {
	return f.principal, f.err
}

func TestAuthorizationHydration_LoadsPrincipal(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	loader := fakePrincipalLoader{
		principal: authorization.NewPrincipal(userID).WithAuthorization(
			[]string{"seller"},
			[]string{
				"product:create",
				"product:update",
			},
		),
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := authorization.PrincipalFromContext(r.Context())
		if !ok {
			t.Fatal("principal missing")
		}

		if principal.UserID != userID {
			t.Fatalf(
				"user ID = %s, want %s",
				principal.UserID,
				userID,
			)
		}

		if !principal.HasRole("seller") {
			t.Fatal("expected seller role")
		}

		if !principal.HasPermission("product:create") {
			t.Fatal("expected product:create permission")
		}

		if !principal.HasPermission("product:update") {
			t.Fatal("expected product:update permission")
		}

		w.WriteHeader(http.StatusNoContent)
	})

	handler := AuthorizationHydration(loader)(next)

	// Simulasikan principal yang sudah dibuat oleh AuthMiddleware.
	ctx := authorization.WithPrincipal(
		context.Background(),
		authorization.NewPrincipal(userID),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	).WithContext(ctx)

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

func TestAuthorizationHydration_LoaderError(t *testing.T) {
	t.Parallel()

	userID := uuid.New()

	loader := fakePrincipalLoader{
		err: errors.New("database unavailable"),
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	handler := AuthorizationHydration(loader)(next)

	ctx := authorization.WithPrincipal(
		context.Background(),
		authorization.NewPrincipal(userID),
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	).WithContext(ctx)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusInternalServerError,
		)
	}
}

func TestAuthorizationHydration_MissingPrincipal(t *testing.T) {
	t.Parallel()

	loader := fakePrincipalLoader{}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	handler := AuthorizationHydration(loader)(next)

	req := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
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
