package repository

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

type poolUnitOfWork struct {
	pool *pgxpool.Pool
}

func newPoolUnitOfWork(pool *pgxpool.Pool) *poolUnitOfWork {
	return &poolUnitOfWork{
		pool: pool,
	}
}

func (r *poolUnitOfWork) Users() UserRepository {
	return newUserRepository(r.pool)
}

func (r *poolUnitOfWork) AuthSessions() AuthSessionRepository {
	return newAuthSessionRepository(r.pool)
}

func (r *poolUnitOfWork) Authorization() AuthorizationRepository {
	return newAuthorizationRepository(r.pool)
}
