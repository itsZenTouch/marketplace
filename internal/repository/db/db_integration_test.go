package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateAndGetUser(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run database integration tests")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(
		ctx,
		databaseURL,
	)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	queries := New(pool)

	userID := uuid.New()

	user, err := queries.CreateUser(ctx, CreateUserParams{
		ID:           userID,
		Email:        "sqlc-test@example.com",
		PasswordHash: "test-hash",
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if user.ID != userID {
		t.Fatalf(
			"CreateUser ID = %v, want %v",
			user.ID,
			userID,
		)
	}

	got, err := queries.GetUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}

	if got.Email != "sqlc-test@example.com" {
		t.Fatalf(
			"Email = %q, want %q",
			got.Email,
			"sqlc-test@example.com",
		)
	}

	_, err = pool.Exec(
		ctx,
		"DELETE FROM users WHERE id = $1",
		userID,
	)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}
