package auth

import (
	"net/http"
	"strings"

	"github.com/itsZenTouch/marketplace/internal/authorization"
	"github.com/itsZenTouch/marketplace/internal/platform/token"
)

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

			principal := authorization.NewPrincipal(userID)

			ctx := authorization.WithPrincipal(
				r.Context(),
				principal,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AuthorizationHydration(
	loader authorization.PrincipalLoader,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := authorization.PrincipalFromContext(r.Context())
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "unauthenticated",
				})
				return
			}

			hydratedPrincipal, err := loader.LoadPrincipal(
				r.Context(),
				principal.UserID,
			)
			if err != nil {
				// Untuk sementara kita pertahankan error sebagai 500.
				// Pada commit berikutnya kita akan membedakan
				// user-not-found dari database/internal error.
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "failed to load authorization",
				})
				return
			}

			ctx := authorization.WithPrincipal(
				r.Context(),
				hydratedPrincipal,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
