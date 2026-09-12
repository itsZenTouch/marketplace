package authorization_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/itsZenTouch/marketplace/internal/authorization"
)

func TestPrincipalContextRoundTrip(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	principal := authorization.NewPrincipal(userID)

	ctx := authorization.WithPrincipal(context.Background(), principal)

	got, ok := authorization.PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("expected principal to exist in context")
	}

	if got.UserID != userID {
		t.Fatalf("expected user ID %s, got %s", userID, got.UserID)
	}
}

func TestPrincipalFromContextMissing(t *testing.T) {
	t.Parallel()

	_, ok := authorization.PrincipalFromContext(context.Background())
	if ok {
		t.Fatal("expected principal to be missing")
	}
}

func TestPrincipalFromContextRejectsZeroValue(t *testing.T) {
	t.Parallel()

	ctx := authorization.WithPrincipal(
		context.Background(),
		authorization.Principal{},
	)

	_, ok := authorization.PrincipalFromContext(ctx)
	if ok {
		t.Fatal("expected invalid principal to be rejected")
	}
}

func TestNewPrincipal(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	principal := authorization.NewPrincipal(userID)

	if !principal.IsValid() {
		t.Fatal("expected principal to be valid")
	}

	if principal.UserID != userID {
		t.Fatalf("expected user ID %s, got %s", userID, principal.UserID)
	}
}

func TestPrincipal_HasRole(t *testing.T) {
	t.Parallel()

	principal := authorization.Principal{
		UserID: uuid.New(),
		Roles: []string{
			"admin",
			"seller",
		},
	}

	tests := []struct {
		name string
		role string
		want bool
	}{
		{
			name: "has role",
			role: "admin",
			want: true,
		},
		{
			name: "has another role",
			role: "seller",
			want: true,
		},
		{
			name: "does not have role",
			role: "customer",
			want: false,
		},
		{
			name: "empty role",
			role: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := principal.HasRole(tt.role)

			if got != tt.want {
				t.Fatalf(
					"HasRole(%q) = %v, want %v",
					tt.role,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestPrincipal_HasPermission(t *testing.T) {
	t.Parallel()

	principal := authorization.Principal{
		UserID: uuid.New(),
		Permissions: []string{
			"user:read",
			"user:update",
		},
	}

	tests := []struct {
		name       string
		permission string
		want       bool
	}{
		{
			name:       "has permission",
			permission: "user:read",
			want:       true,
		},
		{
			name:       "has another permission",
			permission: "user:update",
			want:       true,
		},
		{
			name:       "does not have permission",
			permission: "user:delete",
			want:       false,
		},
		{
			name:       "empty permission",
			permission: "",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := principal.HasPermission(tt.permission)

			if got != tt.want {
				t.Fatalf(
					"HasPermission(%q) = %v, want %v",
					tt.permission,
					got,
					tt.want,
				)
			}
		})
	}
}
