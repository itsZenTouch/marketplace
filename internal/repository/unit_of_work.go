package repository

import "github.com/jackc/pgx/v5"

type txUnitOfWork struct {
	tx pgx.Tx
}

func newTxUnitOfWork(tx pgx.Tx) *txUnitOfWork {
	return &txUnitOfWork{
		tx: tx,
	}
}

func (r *txUnitOfWork) Users() UserRepository {
	return newUserRepository(r.tx)
}

func (r *txUnitOfWork) AuthSessions() AuthSessionRepository {
	return newAuthSessionRepository(r.tx)
}

func (r *txUnitOfWork) Authorization() AuthorizationRepository {
	return newAuthorizationRepository(r.tx)
}
