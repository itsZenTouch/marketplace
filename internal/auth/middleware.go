package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/itsZenTouch/marketplace/internal/platform/token"
)

type contextKey string

const userIDContextKey contextKey = "user_id"

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(userIDContextKey).(uuid.UUID)
	return userID, ok
}

func AuthMiddleware(tokenJWT *token.JWT) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")

			if header == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "missing authorization header",
				})
				return
			}

			parts := strings.Fields(header)

			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "invalid authorization header",
				})
				return
			}

			userID, err := tokenJWT.ParseAccessToken(parts[1])
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "invalid access token",
				})
				return
			}
			ctx := context.WithValue(
				r.Context(),
				userIDContextKey,
				userID,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
