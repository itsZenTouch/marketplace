package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware_MissingAuthorization(t *testing.T) {
	handler := AuthMiddleware(nil)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}
