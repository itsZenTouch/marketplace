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
