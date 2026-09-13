package auth

import (
	"net/http"
	"strings"

	"github.com/itsZenTouch/marketplace/internal/authorization"
	"github.com/itsZenTouch/marketplace/internal/platform/token"
	"github.com/itsZenTouch/marketplace/internal/repository"
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
	uowManager repository.UnitOfWorkManager,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := authorization.PrincipalFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}

			if !principal.IsValid() {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}

			var authz repository.UserAuthorization

			err := uowManager.WithoutTx(
				r.Context(),
				func(uow repository.UnitOfWork) error {
					var err error

					authz, err = uow.Authorization().GetUserAuthorization(
						r.Context(),
						principal.UserID,
					)

					return err
				},
			)
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			principal.WithAuthorization(
				authz.Roles,
				authz.Permissions,
			)

			ctx := authorization.WithPrincipal(
				r.Context(),
				principal,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
