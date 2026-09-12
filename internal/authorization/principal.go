package authorization

import (
	"context"

	"github.com/google/uuid"
)

type contextKey struct{}

type Principal struct {
	UserID uuid.UUID
}

func NewPrincipal(userID uuid.UUID) Principal {
	return Principal{
		UserID: userID,
	}
}

func (p Principal) IsValid() bool {
	return p.UserID != uuid.Nil
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(contextKey{}).(Principal)
	if !ok || !principal.IsValid() {
		return Principal{}, false
	}

	return principal, true
}
