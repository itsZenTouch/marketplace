package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/itsZenTouch/marketplace/internal/authorization"
	"github.com/itsZenTouch/marketplace/internal/platform/token"
)

func newTestRouter(
	handler *Handler,
	jwt *token.JWT,
) http.Handler {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)

	router.Post("/api/auth/login", handler.Login)
	router.Post("/api/auth/refresh", handler.Refresh)

	router.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(jwt))

		r.Get("/api/auth/me", handler.Me)
		r.Get("/api/auth/sessions", handler.Sessions)
	})

	return router
}

func TestRouter_RefreshDoesNotRequireAccessToken(t *testing.T) {
	handler := NewHandler(nil)

	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)

	router := newTestRouter(handler, jwt)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/refresh",
		strings.NewReader(`invalid-json`),
	)

	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusBadRequest,
		)
	}
}

func TestRouter_MeRequiresAuthentication(t *testing.T) {
	handler := NewHandler(nil)

	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)

	router := newTestRouter(handler, jwt)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/auth/me",
		nil,
	)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusUnauthorized,
		)
	}
}

func TestRouter_SessionsRequiresAuthentication(t *testing.T) {
	handler := NewHandler(nil)

	jwt := token.NewJWT(
		"test-secret",
		"marketplace-test",
		15*time.Minute,
		24*time.Hour,
	)

	router := newTestRouter(handler, jwt)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/auth/sessions",
		nil,
	)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusUnauthorized,
		)
	}
}

func TestRouter_ProtectedRouteRequiresPermission(t *testing.T) {
	t.Parallel()

	router := chi.NewRouter()

	router.Group(func(r chi.Router) {
		r.Use(RequirePermission("product:create"))

		r.Post("/api/products", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	})

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/products",
		nil,
	)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusUnauthorized,
		)
	}
}

func TestRouter_ProtectedRouteAllowsPermission(t *testing.T) {
	t.Parallel()

	router := chi.NewRouter()

	router.Group(func(r chi.Router) {
		r.Use(RequirePermission("product:create"))

		r.Post("/api/products", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	})

	principal := authorization.NewPrincipal(uuid.New()).WithAuthorization(
		[]string{"seller"},
		[]string{"product:create"},
	)

	ctx := authorization.WithPrincipal(
		context.Background(),
		principal,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/products",
		nil,
	).WithContext(ctx)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusNoContent,
		)
	}
}

func TestRouter_ProtectedRouteForbiddenWithoutPermission(t *testing.T) {
	t.Parallel()

	router := chi.NewRouter()

	router.Group(func(r chi.Router) {
		r.Use(RequirePermission("product:create"))

		r.Post("/api/products", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	})

	principal := authorization.NewPrincipal(uuid.New()).WithAuthorization(
		[]string{"buyer"},
		[]string{"product:read"},
	)

	ctx := authorization.WithPrincipal(
		context.Background(),
		principal,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/products",
		nil,
	).WithContext(ctx)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf(
			"status = %d, want %d",
			rec.Code,
			http.StatusForbidden,
		)
	}
}
