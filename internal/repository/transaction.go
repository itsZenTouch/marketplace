package repository

import (
	"context"

	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/repository/db"
	"github.com/jackc/pgx/v5"
)

type TxRepository struct {
	tx pgx.Tx
}

func (r *TxRepository) Users() *userRepository {
	return newUserRepository(r.tx)
}

func (r *TxRepository) AuthSessions() *authSessionRepository {
	return newAuthSessionRepository(r.tx)
}

func (r *Repository) RepoWithTx(
	ctx context.Context,
	fn func(uow UnitOfWork) error,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	uow := &TxRepository{
		tx: tx,
	}

	if err := fn(uow); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *TxRepository) CreateUser(
	ctx context.Context,
	input CreateUserInput,
) (domain.User, error) {
	queries := db.New(r.tx)

	user, err := queries.CreateUser(ctx, db.CreateUserParams{
		ID:           input.ID,
		Email:        input.Email,
		PasswordHash: input.PasswordHash,
		Status:       string(input.Status),
	})
	if err != nil {
		return domain.User{}, err
	}

	return userToDomain(user), nil
}
