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

# 36. Checkpoint database

Sekarang jalankan:

```bash
gofmt -w ./cmd ./internal
```
