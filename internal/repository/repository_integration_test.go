package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/itsZenTouch/marketplace/internal/domain"
)

func TestRepositoryTransaction(t *testing.T) {
	if os.Getenv("RUN_DB_TESTS") != "1" {
		t.Skip("set RUN_DB_TESTS=1 to run database integration tests")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	repo := NewRepository(pool)

	t.Run("commit", func(t *testing.T) {
		userID := uuid.New()
		email := "repository-commit-" + userID.String() + "@example.com"

		err := repo.RepoWithTx(ctx, func(uow UnitOfWork) error {
			_, err := uow.Users().CreateUser(ctx, CreateUserInput{
				ID:           userID,
				Email:        email,
				PasswordHash: "test-hash",
				Status:       domain.UserStatusActive,
			})

			return err
		})
		if err != nil {
			t.Fatalf("RepoWithTx: %v", err)
		}

		userRepo := NewUserRepository(pool)

		user, err := userRepo.GetUserByID(ctx, userID)
		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}

		if user.Email != email {
			t.Fatalf(
				"Email = %q, want %q",
				user.Email,
				email,
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
	})

	t.Run("rollback", func(t *testing.T) {
		userID := uuid.New()
		email := "repository-rollback-" + userID.String() + "@example.com"

		expectedErr := context.Canceled

		err := repo.RepoWithTx(ctx, func(uow UnitOfWork) error {
			_, err := uow.Users().CreateUser(ctx, CreateUserInput{
				ID:           userID,
				Email:        email,
				PasswordHash: "test-hash",
				Status:       domain.UserStatusActive,
			})
			if err != nil {
				return err
			}

			return expectedErr
		})

		if err != expectedErr {
			t.Fatalf(
				"RepoWithTx error = %v, want %v",
				err,
				expectedErr,
			)
		}

		userRepo := NewUserRepository(pool)

		_, err = userRepo.GetUserByID(ctx, userID)
		if err == nil {
			t.Fatal("expected user not to exist after rollback")
		}
	})
}
