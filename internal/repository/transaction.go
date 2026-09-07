package repository

import "context"

func (r *Repository) WithTx(
	ctx context.Context,
	fn func(UnitOfWork) error,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	uow := newTxUnitOfWork(tx)

	if err := fn(uow); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}
