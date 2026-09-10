# Marketplace API

Multi-seller marketplace backend built with Go.

## Stack

- Go 1.27.1
- Chi
- PostgreSQL
- pgxpool
- sqlc
- Goose
- JWT
- validator
- x/crypto

## Development

Copy environment configuration:

```bash
cp .env.example .env
```

## Database

Development database:

- PostgreSQL
- Goose migrations
- sqlc generated queries

Run migrations:

```bash
goose -dir migrations postgres "$DATABASE_URL" up
```

Rollback the latest migration:

```bash
goose -dir migrations postgres "$DATABASE_URL" down
```

Check migration status:

```bash
goose -dir migrations postgres "$DATABASE_URL" status
```

## SQLC

Generate type-safe database code:

```bash
sqlc generate
```

Generated code:

`internal/repository/db/`

Do not edit generated files manually.
Update SQL queries/schema and run sqlc generate again.

## Run all test

```bash
RUN_DB_TESTS=1 DATABASE_URL="$DATABASE_URL" go test ./... -v
```

### concurrency verification

```bash
RUN_DB_TESTS=1 DATABASE_URL="$DATABASE_URL" \
go test ./internal/auth -run TestServiceRefresh_ConcurrentSameToken -count=10 -v
```

```bash
RUN_DB_TESTS=1 DATABASE_URL="$DATABASE_URL" \
go test ./internal/auth -run 'TestServiceRefresh' -count=5 -v
```

---
## coming soon...

✅ -> Establish family identity + correct current-session invariants.

✅ -> detect token reuse.

✅ -> Revoke entire family on reuse.

✅ -> Make sessions immutable

✅ Correct concurrency model
✅ Atomic token consumption
✅ New session generation
✅ Token-family preservation
✅ Reuse detection preserved
✅ Integration-tested under repeated concurrency
✅ Full test suite passing

Token rotation
├── new session ID per generation
├── same family ID
├── old session consumed
├── concurrent reuse detected atomically
├── reuse → family revocation
└── descendant session invalidated
