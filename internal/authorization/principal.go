package authorization

import (
	"context"

	"github.com/google/uuid"
)

type contextKey struct{}

type Principal struct {
	UserID      uuid.UUID
	Roles       []string
	Permissions []string
}

func NewPrincipal(userID uuid.UUID) Principal {
	return Principal{
		UserID: userID,
	}
}

func (p Principal) WithAuthorization(
	roles []string,
	permissions []string,
) Principal {
	p.Roles = roles
	p.Permissions = permissions

	return p
}

func (p Principal) IsValid() bool {
	return p.UserID != uuid.Nil
}

func (p Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}

	return false
}

func (p Principal) HasPermission(permission string) bool {
	for _, p := range p.Permissions {
		if p == permission {
			return true
		}
	}

	return false
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

func RequirePermission(
	ctx context.Context,
	permission string,
) error {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}

	if !principal.HasPermission(permission) {
		return ErrForbidden
	}

	return nil
}

func RequireRole(
	ctx context.Context,
	role string,
) error {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}

	if !principal.HasRole(role) {
		return ErrForbidden
	}

	return nil
}
