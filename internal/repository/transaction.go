package repository

import (
	"context"

	"github.com/itsZenTouch/marketplace/internal/repository/db"
)

func (r *Repository) RepoWithTx(
	ctx context.Context,
	fn func(q *db.Queries) error,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	q := db.New(tx)

	if err := fn(q); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
