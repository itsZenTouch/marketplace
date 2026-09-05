Bagus. Berarti checkpoint kita sekarang:

STEP 01 ✅ Project bootstrap
STEP 02 ✅ Configuration
STEP 03 ✅ PostgreSQL + pgxpool
STEP 04 ✅ Goose migrations

Sekarang kita masuk STEP 05 — sqlc v1.31.1.

Fokus tahap ini adalah membuat alur:

PostgreSQL
↓
SQL schema
↓
SQL queries
↓
sqlc generate
↓
Type-safe Go code
↓
pgxpool

Kita tidak menggunakan ORM.

STEP 05 — sqlc v1.31.1

1. Pastikan sqlc

Jalankan:

sqlc version

Target:

v1.31.1

Jika binary belum tersedia, instal versi yang kita sepakati:

go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1

Lalu:

sqlc version

2. Prinsip struktur sqlc kita

Ada satu hal penting sebelum membuat file.

Kita tidak akan membuat sqlc membaca migration directory secara langsung sebagai satu-satunya source of truth.

Kita akan memisahkan:

migrations/
↓
database schema evolution

sql/
├── schema/
│ └── ...
│
└── queries/
└── ...
↓
sqlc
↓
internal/repository/db/

Namun schema sqlc harus tetap konsisten dengan database.

Untuk tahap awal, kita bisa menggunakan migration SQL sebagai input schema jika tooling/workflow kita memungkinkan. Tetapi saya lebih menyarankan pola yang eksplisit:

migrations/
│
│ authoritative database history
▼
PostgreSQL

sql/schema/
│
│ sqlc schema input
▼
sqlc

Nanti kita disiplinkan agar keduanya tidak divergen.

3. Buat struktur directory

Jalankan:

mkdir -p sql/schema
mkdir -p sql/queries
mkdir -p internal/repository/db

Struktur:

marketplace/
├── internal/
│ ├── config/
│ ├── platform/
│ │ └── database/
│ └── repository/
│ └── db/
│
├── migrations/
│
└── sql/
├── schema/
└── queries/

4. Buat schema untuk sqlc

Buat:

sql/schema/001_users.sql

Isi:

Writing
CREATE TABLE users (
id UUID PRIMARY KEY,

    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,

    status TEXT NOT NULL DEFAULT 'active',

    email_verified_at TIMESTAMPTZ,

    failed_login_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT users_email_unique
        UNIQUE (email),

    CONSTRAINT users_status_check
        CHECK (
            status IN (
                'active',
                'suspended',
                'disabled'
            )
        ),

    CONSTRAINT users_failed_login_attempts_check
        CHECK (failed_login_attempts >= 0)

);

Kemudian:

sql/schema/002_auth_sessions.sql

Writing
CREATE TABLE auth_sessions (
id UUID PRIMARY KEY,

    user_id UUID NOT NULL,

    refresh_token_hash TEXT NOT NULL,

    user_agent TEXT,
    ip_address INET,

    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT auth_sessions_user_fk
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    CONSTRAINT auth_sessions_expiry_check
        CHECK (expires_at > created_at)

);

CREATE INDEX idx_auth_sessions_user_id
ON auth_sessions(user_id);

CREATE INDEX idx_auth_sessions_expires_at
ON auth_sessions(expires_at);

Perhatikan: file schema ini tidak memiliki -- +goose Up/Down.

Ini adalah input schema untuk sqlc, bukan migration.

5. Buat sqlc configuration

Buat di root:

sqlc.yaml

Isi:

Writing
version: "2"

sql:

- engine: "postgresql"
  queries: "sql/queries"
  schema: "sql/schema"
  gen:
  go:
  package: "db"
  out: "internal/repository/db"
  sql_package: "pgx/v5"

      emit_json_tags: true
      emit_empty_slices: true
      emit_pointers_for_null_types: true
      emit_interface: false

      overrides:
        - db_type: "uuid"
          go_type:
            import: "github.com/google/uuid"
            type: "UUID"

6. Kenapa sql_package: pgx/v5?

Karena stack kita:

pgx/v5
pgxpool

Jadi generated code akan menggunakan:

github.com/jackc/pgx/v5

bukan database/sql.

Ini penting agar seluruh database layer konsisten.

7. Kenapa UUID menjadi google/uuid?

Database:

id UUID

Go:

uuid.UUID

Dengan:

github.com/google/uuid

Jadi kita tidak perlu membawa UUID sebagai:

string

di seluruh application layer.

Ini mengurangi kemungkinan ID invalid masuk terlalu jauh ke sistem.

8. Query pertama — Create User

Buat:

sql/queries/users.sql

Isi:

Writing
-- name: CreateUser :one
INSERT INTO users (
id,
email,
password_hash,
status
)
VALUES (
$1,
$2,
$3,
$4
)
RETURNING
id,
email,
password_hash,
status,
email_verified_at,
failed_login_attempts,
locked_until,
created_at,
updated_at;

Perhatikan:

-- name: CreateUser :one

Ini instruksi sqlc.

Artinya:

CreateUser
↓
1 row

9. Query get user by ID

Tambahkan:

Writing
-- name: GetUserByID :one
SELECT
id,
email,
password_hash,
status,
email_verified_at,
failed_login_attempts,
locked_until,
created_at,
updated_at
FROM users
WHERE id = $1
LIMIT 1;

10. Query get user by email

Tambahkan:

Writing
-- name: GetUserByEmail :one
SELECT
id,
email,
password_hash,
status,
email_verified_at,
failed_login_attempts,
locked_until,
created_at,
updated_at
FROM users
WHERE email = $1
LIMIT 1;

Query ini akan sangat penting ketika login:

email
↓
GetUserByEmail
↓
password hash
↓
verify password

11. Update login failure

Tambahkan:

Writing
-- name: IncrementFailedLoginAttempts :one
UPDATE users
SET
failed_login_attempts = failed_login_attempts + 1,
updated_at = NOW()
WHERE id = $1
RETURNING
id,
email,
password_hash,
status,
email_verified_at,
failed_login_attempts,
locked_until,
created_at,
updated_at;

12. Reset login failures

Tambahkan:

Writing
-- name: ResetFailedLoginAttempts :one
UPDATE users
SET
failed_login_attempts = 0,
locked_until = NULL,
updated_at = NOW()
WHERE id = $1
RETURNING
id,
email,
password_hash,
status,
email_verified_at,
failed_login_attempts,
locked_until,
created_at,
updated_at;

13. Query auth session

Tambahkan ke sql/queries/auth_sessions.sql:

Writing
-- name: CreateAuthSession :one
INSERT INTO auth_sessions (
id,
user_id,
refresh_token_hash,
user_agent,
ip_address,
expires_at
)
VALUES (
$1,
$2,
$3,
$4,
$5,
$6
)
RETURNING
id,
user_id,
refresh_token_hash,
user_agent,
ip_address,
expires_at,
revoked_at,
created_at,
updated_at;

14. Get session

Tambahkan:

Writing
-- name: GetAuthSessionByID :one
SELECT
id,
user_id,
refresh_token_hash,
user_agent,
ip_address,
expires_at,
revoked_at,
created_at,
updated_at
FROM auth_sessions
WHERE id = $1
LIMIT 1;

15. Revoke session

Tambahkan:

Writing
-- name: RevokeAuthSession :one
UPDATE auth_sessions
SET
revoked_at = NOW(),
updated_at = NOW()
WHERE id = $1
AND revoked_at IS NULL
RETURNING
id,
user_id,
refresh_token_hash,
user_agent,
ip_address,
expires_at,
revoked_at,
created_at,
updated_at;

Perhatikan:

AND revoked_at IS NULL

Artinya session yang sudah revoked tidak "direvoke ulang".

16. List sessions user

Tambahkan:

Writing
-- name: ListAuthSessionsByUserID :many
SELECT
id,
user_id,
refresh_token_hash,
user_agent,
ip_address,
expires_at,
revoked_at,
created_at,
updated_at
FROM auth_sessions
WHERE user_id = $1
ORDER BY created_at DESC;

17. Generate code

Sekarang:

sqlc generate

Kalau berhasil, lihat:

find internal/repository/db -type f

Anda akan mendapatkan file generated seperti:

internal/repository/db/
├── db.go
├── models.go
├── users.sql.go
└── auth_sessions.sql.go

Nama file dapat sedikit berbeda tergantung konfigurasi/versi, tetapi intinya generated Go code harus muncul.

18. Jangan edit generated files

Misalnya:

internal/repository/db/users.sql.go

Jangan edit manual.

Jika ada perubahan:

SQL
↓
sqlc generate
↓
generated code

Bukan:

generated code
↓
edit manual

Karena generate berikutnya akan menghapus perubahan manual tersebut.

19. Lihat generated model

Buka:

internal/repository/db/models.go

Anda akan melihat kurang lebih:

type User struct {
ID uuid.UUID
Email string
PasswordHash string
Status string
EmailVerifiedAt pgtype.Timestamptz
FailedLoginAttempts int32
LockedUntil pgtype.Timestamptz
CreatedAt pgtype.Timestamptz
UpdatedAt pgtype.Timestamptz
}

Detail nullable type bisa berbeda sesuai konfigurasi sqlc/pgx.

Hal yang penting:

PostgreSQL UUID
↓
uuid.UUID

PostgreSQL TEXT NOT NULL
↓
string

PostgreSQL nullable timestamp
↓
nullable pgx type

20. Generated Queries

Sekarang lihat:

internal/repository/db/users.sql.go

Anda akan menemukan method semacam:

func (q \*Queries) CreateUser(
ctx context.Context,
arg CreateUserParams,
) (User, error)

Dan:

func (q \*Queries) GetUserByEmail(
ctx context.Context,
email string,
) (User, error)

Inilah manfaat utama sqlc:

Kita menulis:

SELECT ...

tetapi mendapatkan:

GetUserByEmail(...)

dengan parameter dan return type yang compile-time checked.

21. Hubungkan sqlc dengan pgxpool

Generated db.go biasanya memiliki constructor yang menerima interface database executor.

Secara konsep:

pgxpool.Pool
↓
db.New(pool)
↓
db.Queries

Kita akan menggunakan pola ini nanti:

queries := db.New(pool)

Kemudian:

user, err := queries.GetUserByEmail(
ctx,
email,
)

Tidak perlu menulis:

pool.QueryRow(...)

berulang-ulang di service.

22. Penting: jangan letakkan business logic di sqlc

sqlc hanya database access.

Bukan tempat untuk:

password validation
JWT
permission
seller rules
order calculation
payment rules

Layer kita nantinya:

HTTP Handler
↓
Service
↓
Repository / sqlc
↓
PostgreSQL

Contoh login:

POST /auth/login
↓
AuthHandler
↓
AuthService
↓
GetUserByEmail()
↓
verify password
↓
create session
↓
generate tokens
↓
response

23. Test generated code

Pertama:

go test ./...

Kemudian:

go vet ./...

Kemudian:

go build ./cmd/api

Kalau sqlc generated code benar, semuanya harus compile.

24. Buat integration test sederhana

Sekarang kita ingin membuktikan:

Go
↓
sqlc
↓
pgxpool
↓
PostgreSQL

Buat:

internal/repository/db/db_integration_test.go

Untuk sekarang kita buat test yang hanya berjalan ketika environment flag diberikan.

Writing
package db

import (
"context"
"os"
"testing"
"time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"

)

func TestCreateAndGetUser(t \*testing.T) {
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

25. Jalankan integration test

Karena test ini menyentuh database, jangan jadikan default dulu.

Jalankan:

RUN_DB_TESTS=1 go test ./internal/repository/db

Pada Linux/macOS:

RUN_DB_TESTS=1 go test ./internal/repository/db

Windows PowerShell:

$env:RUN_DB_TESTS="1"
go test ./internal/repository/db

Expected:

PASS

Kemudian unset:

PowerShell:

Remove-Item Env:RUN_DB_TESTS

26. Periksa data

Setelah test selesai:

psql "postgres://marketplace:marketplace@localhost:5432/marketplace?sslmode=disable"

SELECT id, email, status
FROM users;

Seharusnya tidak ada:

sqlc-test@example.com

karena test melakukan cleanup.

27. Masalah penting: schema duplication

Sekarang kita punya:

migrations/
├── ...create_users.sql
└── ...create_auth_sessions.sql

sql/schema/
├── 001_users.sql
└── 002_auth_sessions.sql

Ya, ada duplication.

Untuk tahap awal ini disengaja supaya kita memahami dua konsep:

migration history
vs
sqlc schema input

Tetapi duplication berarti ada risiko:

migration ≠ sqlc schema

Dan kita tidak boleh membiarkan itu terjadi.

Pada tahap berikutnya kita akan menetapkan workflow yang lebih ketat untuk menjaga schema source tetap sinkron.

28. Tambahkan sqlc ke developer tooling

Tambahkan ke README:

Writing

## SQLC

Generate type-safe database code:

````bash
sqlc generate


Generated code:

internal/repository/db/


Do not edit generated files manually.
Update SQL queries/schema and run sqlc generate again.

:::

---

# 29. Tambahkan `.gitignore`

Pastikan:

```gitignore
.env


ada di .gitignore.

Tetapi generated sqlc code jangan di-ignore.

Kita ingin:

internal/repository/db/


masuk Git.

Alasannya supaya build/CI tidak membutuhkan sqlc binary untuk sekadar compile project.

30. Jangan commit .env

Periksa:

git status


.env seharusnya tidak muncul sebagai untracked file.

Yang boleh masuk repository:

.env.example


Yang tidak boleh:

.env

31. Security checkpoint

Pada tahap ini kita sudah memiliki beberapa defense:

                    DATABASE
                       │
        ┌──────────────┼──────────────┐
        ▼              ▼              ▼
       FK           UNIQUE          CHECK
        │              │              │
        ▼              ▼              ▼
 integrity       duplicate       invalid state


Dan application layer:

SQL
 ↓
sqlc
 ↓
typed Go
 ↓
compile-time checking


Kita juga tidak menyimpan:

refresh_token


plaintext di database.

Hanya:

refresh_token_hash

32. Checkpoint STEP 05

Sekarang jalankan secara berurutan:

sqlc version


harus:

v1.31.1


Kemudian:

sqlc generate


Lalu:

gofmt -w ./cmd ./internal


Kemudian:

go mod tidy


Lalu:

go test ./...


Kemudian:

go test -race ./...


Lalu:

go vet ./...


Lalu:

go build ./cmd/api


Terakhir integration test:

RUN_DB_TESTS=1 go test ./internal/repository/db

Kondisi yang kita inginkan
STEP 01  Bootstrap                 ✅
STEP 02  Configuration             ✅
STEP 03  PostgreSQL + pgxpool      ✅
STEP 04  Goose                     ✅
STEP 05  sqlc                      ⬅️ sekarang


Dan dependency flow:

                  ┌───────────────┐
                  │   PostgreSQL  │
                  └───────┬───────┘
                          │
             ┌────────────┴────────────┐
             │                         │
             ▼                         ▼
       Goose migrations           sql/schema
             │                         │
             │                         ▼
             │                       sqlc
             │                         │
             │                         ▼
             │                generated Go types
             │                         │
             └────────────┬────────────┘
                          ▼
                       pgxpool
                          │
                          ▼
                       Service


Jangan masuk ke authentication service dulu. Setelah checkpoint STEP 05 hijau, tahap berikutnya sebaiknya kita buat STEP 06 — Repository & Transaction Boundary, karena sebelum JWT/password/session kita perlu menetapkan dengan benar bagaimana sqlc, pgxpool, transaction, repository interface, dan dependency injection bekerja. Ini akan menjadi fondasi agar business logic marketplace nantinya tidak langsung bergantung pada generated SQL code.
````

---

# Repository Foundation & Transaction

Sip. Module path kita sudah jelas:

github.com/itsZenTouch/marketplace

Kita lanjut STEP 06 secara konkret. Saya sarankan jangan membuat semua abstraction sekaligus; kita buat boundary yang kecil dulu, lalu compile.

STEP 06.1 — Repository foundation

Struktur target:

internal/
└── repository/
├── db/ # generated sqlc — jangan edit
├── repository.go
├── interfaces.go
├── user_repository.go
└── transaction.go

1. repository.go

Buat:

internal/repository/repository.go

Isi:

package repository

import "github.com/jackc/pgx/v5/pgxpool"

type Repository struct {
pool \*pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
return &Repository{
pool: pool,
}
}

Ini menjadi root repository yang memegang pgxpool.Pool.

2. interfaces.go

Buat:

internal/repository/interfaces.go

Isi:

package repository

import (
"context"

    "github.com/google/uuid"

    "github.com/itsZenTouch/marketplace/internal/repository/db"

)

type UserRepository interface {
CreateUser(
ctx context.Context,
arg db.CreateUserParams,
) (db.User, error)

    GetUserByID(
    	ctx context.Context,
    	id uuid.UUID,
    ) (db.User, error)

    GetUserByEmail(
    	ctx context.Context,
    	email string,
    ) (db.User, error)

    IncrementFailedLoginAttempts(
    	ctx context.Context,
    	id uuid.UUID,
    ) (db.User, error)

    ResetFailedLoginAttempts(
    	ctx context.Context,
    	id uuid.UUID,
    ) (db.User, error)

}

Untuk sekarang kita masih memakai db.User dan db.CreateUserParams.

Belum kita pisahkan menjadi domain model. Itu sengaja; kita fokus dulu ke repository/transaction boundary.

3. user_repository.go

Buat:

internal/repository/user_repository.go

Isi:

package repository

import (
"context"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/itsZenTouch/marketplace/internal/repository/db"

)

type userRepository struct {
pool \*pgxpool.Pool
}

func NewUserRepository(pool \*pgxpool.Pool) UserRepository {
return &userRepository{
pool: pool,
}
}

func (r \*userRepository) CreateUser(
ctx context.Context,
arg db.CreateUserParams,
) (db.User, error) {
queries := db.New(r.pool)

    return queries.CreateUser(ctx, arg)

}

func (r \*userRepository) GetUserByID(
ctx context.Context,
id uuid.UUID,
) (db.User, error) {
queries := db.New(r.pool)

    return queries.GetUserByID(ctx, id)

}

func (r \*userRepository) GetUserByEmail(
ctx context.Context,
email string,
) (db.User, error) {
queries := db.New(r.pool)

    return queries.GetUserByEmail(ctx, email)

}

func (r \*userRepository) IncrementFailedLoginAttempts(
ctx context.Context,
id uuid.UUID,
) (db.User, error) {
queries := db.New(r.pool)

    return queries.IncrementFailedLoginAttempts(ctx, id)

}

func (r \*userRepository) ResetFailedLoginAttempts(
ctx context.Context,
id uuid.UUID,
) (db.User, error) {
queries := db.New(r.pool)

    return queries.ResetFailedLoginAttempts(ctx, id)

}

Ada satu hal yang sengaja kita lakukan di sini:

queries := db.New(r.pool)

Repository menjadi adapter antara application layer dan generated sqlc.

STEP 06.2 — Transaction boundary

Sekarang buat:

internal/repository/transaction.go

Isi:

package repository

import (
"context"

    "github.com/itsZenTouch/marketplace/internal/repository/db"

)

func (r *Repository) WithTx(
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

    queries := db.New(tx)

    if err := fn(queries); err != nil {
    	return err
    }

    return tx.Commit(ctx)

}

Sekarang boundary-nya menjadi:

Repository
│
├── pool
│
└── WithTx()
│
▼
pgx.Tx
│
▼
db.New(tx)
│
▼
sqlc Queries

Kenapa Rollback() tetap di defer?

Misalnya callback gagal:

return err

maka transaction otomatis dicoba rollback.

Kalau Commit() sudah berhasil, transaction sudah closed sehingga rollback di defer tidak melakukan apa-apa yang berarti. Error-nya sengaja kita abaikan.

STEP 06.3 — Compile checkpoint

Sekarang jangan masuk service dulu.

Jalankan:

gofmt -w ./internal/repository

Kemudian:

go test ./internal/repository/...

Lalu:

go vet ./internal/repository/...

Dan terakhir:

go build ./cmd/api

Ada satu kemungkinan error

Kalau compiler mengeluh pada:

db.New(tx)

atau interface DBTX, jangan kita tebak-tebak solusinya.

Kirim error compiler tersebut karena kita akan menyesuaikan transaction boundary dengan db.go yang benar-benar dihasilkan sqlc v1.31.1 milik project kamu.

Kalau empat command di atas semuanya PASS, checkpoint kita menjadi:

STEP 01 Bootstrap ✅
STEP 02 Configuration ✅
STEP 03 PostgreSQL + pgxpool ✅
STEP 04 Goose ✅
STEP 05 sqlc v1.31.1 ✅
STEP 06.1 Repository foundation ⬅️
STEP 06.2 Transaction boundary ⬅️

Setelah itu baru kita lanjut ke STEP 06.4 — integration test untuk repository + transaction, supaya kita membuktikan bukan cuma compile, tetapi benar-benar:

Repository
↓
sqlc
↓
pgx.Tx
↓
PostgreSQL
↓
COMMIT / ROLLBACK

Jalankan empat command checkpoint di atas dan kirim output-nya.

---

# Password hash

Lanjut. Kita anggap STEP 07 sudah GREEN sesuai checkpoint terakhir. Sekarang masuk:

STEP 08 — Authentication Foundation

Kita belum membuat JWT dan belum membuat HTTP handler login.

Urutan yang lebih aman:

STEP 08
Authentication Foundation
│
├── Domain auth errors
├── Password hashing
├── Password verification
├── Auth service contract
└── Test
↓
STEP 09
Login + session creation
↓
STEP 10
Access/refresh token

Untuk sekarang fokus pertama: password hashing.

08.1 — Prinsip password

Database kita sudah mempunyai:

users.password_hash

Yang disimpan bukan password asli.

Flow:

Register
│
▼
plain password
│
▼
password hasher
│
▼
password_hash
│
▼
PostgreSQL

Login
│
▼
plain password
│
▼
compare dengan password_hash
│
├── valid → lanjut
└── invalid → authentication error

Kita tidak akan menggunakan:

SHA256(password)
MD5(password)
SHA1(password)

untuk password storage.

Gunakan Argon2id.

08.2 — Buat package password

Buat directory:

mkdir -p internal/security/password

Struktur:

internal/
└── security/
└── password/
├── hasher.go
└── hasher_test.go

08.3 — Password hasher interface

Buat:

internal/security/password/hasher.go

Isi:

package password

import (
"fmt"

    "golang.org/x/crypto/argon2"

)

const (
saltLength = 16

    timeCost    = 3
    memoryCost  = 64 * 1024
    parallelism = 2
    keyLength   = 32

)

type Hasher struct{}

func NewHasher() \*Hasher {
return &Hasher{}
}

func (h \*Hasher) Hash(password string) (string, error) {
if password == "" {
return "", fmt.Errorf("password cannot be empty")
}

    salt := make([]byte, saltLength)

    // Kita akan menggunakan crypto/rand untuk salt.
    // Implementasi lengkapnya kita buat setelah dependency siap.
    _ = salt

    return "", nil

}

func (h \*Hasher) Compare(password, encodedHash string) error {
return nil
}

func deriveKey(password string, salt []byte) []byte {
return argon2.IDKey(
[]byte(password),
salt,
timeCost,
memoryCost,
parallelism,
keyLength,
)
}

Tapi jangan jalankan test dulu.

Saya sengaja belum memberikan implementation final di bagian Hash/Compare, karena kita perlu menetapkan format encoded hash terlebih dahulu.

Kita akan menggunakan format yang eksplisit:

$argon2id$v=19$m=65536,t=3,p=2$<salt>$<key>

Kenapa?

Karena parameter Argon2 harus ikut tersimpan bersama hash. Dengan begitu kalau parameter security berubah di masa depan, hash lama masih dapat diverifikasi.

08.4 — Dependency

Jalankan:

go get golang.org/x/crypto

Lalu:

go mod tidy

Setelah itu cek:

go list -m golang.org/x/crypto

Kita tidak perlu mem-pin versi secara manual; go get akan memilih versi yang kompatibel dengan Go toolchain project kamu.

08.5 — Jangan gunakan password langsung di repository

Ada boundary penting di sini:

AuthService
│
│ plain password
▼
Password Hasher
│
│ encoded hash
▼
UserRepository
│
▼
PostgreSQL

Repository hanya menerima:

PasswordHash

dan tidak mengetahui bagaimana password dibuat.

Jadi nanti:

hash, err := passwordHasher.Hash(input.Password)

kemudian:

userRepo.CreateUser(... PasswordHash: hash)

Repository tidak boleh melakukan:

hashPassword(...)

sendiri.

08.6 — Security rules

Untuk package ini kita akan enforce:

password kosong → error
password tidak pernah ditulis ke log
salt random menggunakan crypto/rand
Argon2id digunakan untuk derivation
salt berbeda menghasilkan hash berbeda
hash dapat diverifikasi
password salah harus gagal
malformed encoded hash harus gagal dengan aman
tidak ada plaintext password yang disimpan

Kita juga tidak akan membuat error yang membocorkan detail internal hash parsing kepada user. Detail tersebut bisa masuk internal error/log bila diperlukan, tetapi API authentication nantinya cukup menghasilkan error generik.

08.7 — Test yang akan kita targetkan

hasher_test.go nantinya minimal membuktikan:

Hash
├── menghasilkan hash
├── hash != plaintext
├── dua hash password sama != satu sama lain
└── hash dapat di-verify

Compare
├── password benar → success
├── password salah → error
└── hash corrupt → error

Kita juga akan memastikan test tidak bergantung pada exact salt/hash, karena salt memang random.

Stop checkpoint di sini

Untuk langkah berikutnya, jalankan dulu:

go get golang.org/x/crypto
go mod tidy
go list -m golang.org/x/crypto

Kirim hasil go list -m golang.org/x/crypto.

Setelah itu kita implementasikan Argon2id encoded hash secara lengkap + unit test, baru lanjut ke AuthService.

# Authtentication Login

Worked for 21s

Siap. Aku cek kondisi repository `main` sekarang. Fondasinya sebenarnya sudah ada untuk authentication: tabel `users`, `auth_sessions`, repository user/session, JWT config, dan Argon2 dependency. Tapi **login belum terhubung ke HTTP server**, sementara password hasher masih stub sehingga saat ini belum bisa benar-benar login. GitHub+3

Aku sarankan kita implement **login end-to-end** dengan flow:

```
POST /api/auth/login
        │
        ▼
validate email/password
        │
        ▼
find user by email
        │
        ▼
check status + lock
        │
        ▼
Argon2 password comparison
        │
        ├── gagal → increment failed attempts
        │
        └── sukses
              │
              ├── reset failed attempts
              ├── create auth session
              ├── generate access JWT
              └── generate refresh token
```

Repository kamu memang sudah menyimpan `refresh_token_hash`, `user_agent`, IP, expiry, dan revoke state di `auth_sessions`, jadi desain ini cocok dengan struktur yang sudah dibuat. GitHub+1

## 1\. Perbaiki password hasher

File sekarang masih mengembalikan string kosong dari `Hash()` dan `nil` dari `Compare()`, jadi **semua password secara praktis dianggap valid**. GitHub

Ganti `internal/platform/password/hasher.go` menjadi:

```
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	saltLength = 16

	timeCost    = 3
	memoryCost  = 64 * 1024
	parallelism = 2
	keyLength   = 32
)

type Hasher struct{}

func NewHasher() *Hasher {
	return &Hasher{}
}

func (h *Hasher) Hash(rawPassword string) (string, error) {
	if rawPassword == "" {
		return "", errors.New("password cannot be empty")
	}

	salt := make([]byte, saltLength)

	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key := deriveKey(rawPassword, salt)

	return fmt.Sprintf(
		"argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memoryCost,
		timeCost,
		parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func (h *Hasher) Compare(rawPassword, encodedHash string) error {
	parts := strings.Split(encodedHash, "$")

	if len(parts) != 6 {
		return errors.New("invalid password hash")
	}

	if parts[0] != "argon2id" || parts[1] != "v=19" {
		return errors.New("unsupported password hash")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return errors.New("invalid password salt")
	}

	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return errors.New("invalid password hash")
	}

	actual := deriveKey(rawPassword, salt)

	if len(actual) != len(expected) {
		return errors.New("invalid password")
	}

	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return errors.New("invalid password")
	}

	return nil
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey(
		[]byte(password),
		salt,
		timeCost,
		memoryCost,
		parallelism,
		keyLength,
	)
}
```

Ada satu hal penting: format yang kita pakai adalah:

```
argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
```

Jadi hash password tidak pernah disimpan sebagai plaintext.

---

# 2\. Tambahkan JWT service

Buat:

`internal/platform/token/jwt.go`

```
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWT struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWT(
	secret string,
	issuer string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *JWT {
	return &JWT{
		secret:     []byte(secret),
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

type AccessClaims struct {
	UserID string `json:"uid"`
	Type   string `json:"type"`

	jwt.RegisteredClaims
}

func (j *JWT) CreateAccessToken(userID uuid.UUID) (string, error) {
	now := time.Now()

	claims := AccessClaims{
		UserID: userID.String(),
		Type:   "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Subject:   userID.String(),
			Issuer:    j.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.accessTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(j.secret)
}

func (j *JWT) CreateRefreshToken(sessionID uuid.UUID) (
	string,
	string,
	error,
) {
	randomBytes := make([]byte, 32)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	randomPart := base64.RawURLEncoding.EncodeToString(randomBytes)

	token := sessionID.String() + "." + randomPart

	hash := sha256.Sum256([]byte(token))
	hashString := hex.EncodeToString(hash[:])

	return token, hashString, nil
}

func HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func ParseRefreshSessionID(refreshToken string) (uuid.UUID, error) {
	var sessionID string

	_, err := fmt.Sscanf(refreshToken, "%36s", &sessionID)
	if err != nil {
		return uuid.Nil, errors.New("invalid refresh token")
	}

	// Token format:
	// <uuid>.<random>
	for i := range refreshToken {
		if refreshToken[i] == '.' {
			sessionID = refreshToken[:i]
			break
		}
	}

	id, err := uuid.Parse(sessionID)
	if err != nil {
		return uuid.Nil, errors.New("invalid refresh token")
	}

	return id, nil
}
```

Untuk login sekarang sebenarnya `CreateRefreshToken()` saja sudah cukup.

---

# 3\. Tambahkan AuthSessionRepository interface

Repository kamu sudah punya implementasi `authSessionRepository`, tetapi interface abstraction-nya belum ada. GitHub+1

Tambahkan ke `internal/repository/interfaces.go`:

```
type AuthSessionRepository interface {
	CreateAuthSession(
		ctx context.Context,
		input CreateAuthSessionInput,
	) (domain.AuthSession, error)

	GetAuthSessionByID(
		ctx context.Context,
		id uuid.UUID,
	) (domain.AuthSession, error)

	RevokeAuthSession(
		ctx context.Context,
		id uuid.UUID,
	) (domain.AuthSession, error)

	ListAuthSessionsByUserID(
		ctx context.Context,
		userID uuid.UUID,
	) ([]domain.AuthSession, error)
}
```

---

# 4\. Buat Auth Service

Buat directory:

```
internal/auth/
```

Kemudian:

`internal/auth/service.go`

```
package auth

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/itsZenTouch/marketplace/internal/domain"
	"github.com/itsZenTouch/marketplace/internal/platform/password"
	"github.com/itsZenTouch/marketplace/internal/platform/token"
	"github.com/itsZenTouch/marketplace/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountSuspended   = errors.New("account suspended")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrAccountLocked      = errors.New("account temporarily locked")
)

type Service struct {
	users       repository.UserRepository
	sessions    repository.AuthSessionRepository
	password    *password.Hasher
	token       *token.JWT
}

func NewService(
	users repository.UserRepository,
	sessions repository.AuthSessionRepository,
	passwordHasher *password.Hasher,
	jwt *token.JWT,
) *Service {
	return &Service{
		users:    users,
		sessions: sessions,
		password: passwordHasher,
		token:    jwt,
	}
}

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IPAddress net.IP
}

type LoginOutput struct {
	User         domain.User
	AccessToken  string
	RefreshToken string
}

func (s *Service) Login(
	ctx context.Context,
	input LoginInput,
) (LoginOutput, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	if email == "" || input.Password == "" {
		return LoginOutput{}, ErrInvalidCredentials
	}

	user, err := s.users.GetUserByEmail(ctx, email)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LoginOutput{}, ErrInvalidCredentials
		}

		return LoginOutput{}, err
	}

	now := time.Now()

	if user.LockedUntil != nil && user.LockedUntil.After(now) {
		return LoginOutput{}, ErrAccountLocked
	}

	switch user.Status {
	case domain.UserStatusSuspended:
		return LoginOutput{}, ErrAccountSuspended

	case domain.UserStatusDisabled:
		return LoginOutput{}, ErrAccountDisabled
	}

	if err := s.password.Compare(input.Password, user.PasswordHash); err != nil {
		_, _ = s.users.IncrementFailedLoginAttempts(ctx, user.ID)

		return LoginOutput{}, ErrInvalidCredentials
	}

	user, err = s.users.ResetFailedLoginAttempts(ctx, user.ID)
	if err != nil {
		return LoginOutput{}, err
	}

	sessionID := uuid.New()

	refreshToken, refreshTokenHash, err :=
		s.token.CreateRefreshToken(sessionID)

	if err != nil {
		return LoginOutput{}, err
	}

	_, err = s.sessions.CreateAuthSession(
		ctx,
		repository.CreateAuthSessionInput{
			ID:               sessionID,
			UserID:           user.ID,
			RefreshTokenHash: refreshTokenHash,
			UserAgent:        input.UserAgent,
			IPAddress:        input.IPAddress,
			ExpiresAt:        now.Add(30 * 24 * time.Hour),
		},
	)

	if err != nil {
		return LoginOutput{}, err
	}

	accessToken, err := s.token.CreateAccessToken(user.ID)
	if err != nil {
		return LoginOutput{}, err
	}

	return LoginOutput{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
```

**Catatan:** TTL refresh session sebaiknya jangan hardcode `30 * 24h`. Karena config kamu sudah punya `JWT_REFRESH_TTL=720h`, sebaiknya `JWT` menyediakan method untuk mengambil TTL atau service menerima refresh TTL. Config repository memang sudah mendefinisikan access dan refresh TTL. GitHub+1

Versi lebih bersih:

```
type JWT struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func (j *JWT) RefreshTTL() time.Duration {
	return j.refreshTTL
}
```

Lalu:

```
ExpiresAt: now.Add(s.token.RefreshTTL()),
```

---

# 5\. Buat HTTP handler

Buat:

`internal/auth/handler.go`

```
package auth

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	User struct {
		ID     string `json:"id"`
		Email  string `json:"email"`
		Status string `json:"status"`
	} `json:"user"`

	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req loginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	ip := net.ParseIP(r.RemoteAddr)

	if ip == nil {
		// Remove port if RemoteAddr is "IP:PORT".
		host := r.RemoteAddr

		if idx := strings.LastIndex(host, ":"); idx > -1 {
			ip = net.ParseIP(host[:idx])
		}
	}

	result, err := h.service.Login(
		r.Context(),
		LoginInput{
			Email:     req.Email,
			Password:  req.Password,
			UserAgent: r.UserAgent(),
			IPAddress: ip,
		},
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "invalid email or password",
			})

		case errors.Is(err, ErrAccountLocked):
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error": "account temporarily locked",
			})

		case errors.Is(err, ErrAccountSuspended):
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "account suspended",
			})

		case errors.Is(err, ErrAccountDisabled):
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "account disabled",
			})

		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "internal server error",
			})
		}

		return
	}

	response := loginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}

	response.User.ID = result.User.ID.String()
	response.User.Email = result.User.Email
	response.User.Status = string(result.User.Status)

	writeJSON(w, http.StatusOK, response)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	value any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}
```

Ada sedikit improvement yang perlu kita lakukan untuk IP parsing: lebih aman memakai `net.SplitHostPort()`. Jadi versi production-nya:

```
host, _, err := net.SplitHostPort(r.RemoteAddr)
if err == nil {
	ip = net.ParseIP(host)
} else {
	ip = net.ParseIP(r.RemoteAddr)
}
```

---

# 6\. Router

Saat ini `cmd/api/main.go` **bahkan belum menjalankan HTTP server**; setelah database dibuat, program hanya menunggu SIGTERM. GitHub

Jadi kita perlu sambungkan Chi.

Tambahkan dependency langsung kalau `go.mod` belum mengklasifikasikannya:

```
go get github.com/go-chi/chi/v5
```

Kemudian ubah `main.go`.

Bagian inti:

```
import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/itsZenTouch/marketplace/internal/auth"
	"github.com/itsZenTouch/marketplace/internal/config"
	"github.com/itsZenTouch/marketplace/internal/platform/database"
	"github.com/itsZenTouch/marketplace/internal/platform/password"
	"github.com/itsZenTouch/marketplace/internal/platform/token"
	"github.com/itsZenTouch/marketplace/internal/repository"
)
```

Setelah:

```
userRepository := repository.NewUserRepository(dbPool)
```

tambahkan:

```
sessionRepository := repository.NewAuthSessionRepository(dbPool)

passwordHasher := password.NewHasher()

jwtService := token.NewJWT(
	cfg.JWTSecret,
	cfg.JWTIssuer,
	cfg.JWTAccessTTL,
	cfg.JWTRefreshTTL,
)

authService := auth.NewService(
	userRepository,
	sessionRepository,
	passwordHasher,
	jwtService,
)

authHandler := auth.NewHandler(authService)
```

Kemudian buat router:

```
router := chi.NewRouter()

router.Post(
	"/api/auth/login",
	authHandler.Login,
)
```

Dan server:

```
server := &http.Server{
	Addr:              ":" + cfg.AppPort,
	Handler:           router,
	ReadHeaderTimeout: 5 * time.Second,
}

go func() {
	log.Printf(
		"marketplace API listening on port %s",
		cfg.AppPort,
	)

	if err := server.ListenAndServe(); err != nil &&
		err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}()
```

Kemudian pada bagian shutdown:

```
shutdownCtx, cancel := context.WithTimeout(
	context.Background(),
	10*time.Second,
)
defer cancel()

if err := server.Shutdown(shutdownCtx); err != nil {
	log.Printf("server shutdown failed: %v", err)
}

log.Println("server stopped")
```

---

# 7\. Satu masalah penting: failed login belum melakukan lock

Database kamu sudah punya:

```
failed_login_attempts
locked_until
```

tetapi query saat ini hanya:

```
IncrementFailedLoginAttempts
ResetFailedLoginAttempts
```

Tidak ada query untuk mengisi `locked_until`. GitHub

Jadi **jangan menganggap lockout sudah bekerja** hanya karena kolomnya ada.

Tambahkan ke `sql/queries/users.sql`:

```
-- name: LockUserUntil :one
UPDATE users
SET
    locked_until = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING
    id,
    email,
    password_hash,
    status,
    email_verified_at,
    failed_login_attempts,
    locked_until,
    created_at,
    updated_at;
```

Lalu:

```
sqlc generate
```

Repository:

```
func (r *userRepository) LockUserUntil(
	ctx context.Context,
	id uuid.UUID,
	until time.Time,
) (domain.User, error) {
	queries := db.New(r.db)

	user, err := queries.LockUserUntil(
		ctx,
		db.LockUserUntilParams{
			ID:          id,
			LockedUntil: until,
		},
	)
	if err != nil {
		return domain.User{}, err
	}

	return userToDomain(user), nil
}
```

Dan interface:

```
LockUserUntil(
	ctx context.Context,
	id uuid.UUID,
	until time.Time,
) (domain.User, error)
```

Kemudian di login:

```
if err := s.password.Compare(input.Password, user.PasswordHash); err != nil {
	failedUser, _ := s.users.IncrementFailedLoginAttempts(
		ctx,
		user.ID,
	)

	const maxAttempts = 5

	if failedUser.FailedLoginAttempts >= maxAttempts {
		_, _ = s.users.LockUserUntil(
			ctx,
			user.ID,
			time.Now().Add(15*time.Minute),
		)
	}

	return LoginOutput{}, ErrInvalidCredentials
}
```

Dengan begitu:

```
5x password salah
       ↓
locked_until = NOW() + 15 minutes
       ↓
login berikutnya
       ↓
429 Too Many Requests
```

---

# 8\. Test login

Setelah semua file dibuat:

```
go test ./...
```

Kemudian:

```
sqlc generate
```

Lalu migration:

```
goose -dir migrations postgres "$DATABASE_URL" up
```

Repository kamu memang sudah menggunakan Goose untuk migration dan `sqlc generate` untuk generated database code. GitHub

Untuk membuat user test, kita juga membutuhkan **register** atau seed user, karena tabel `users` mensyaratkan `password_hash`. Schema user saat ini memang menyimpan password sebagai hash, bukan plaintext. GitHub

Setelah ada user:

```
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

Expected:

```
{
  "user": {
    "id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "email": "user@example.com",
    "status": "active"
  },
  "access_token": "eyJ...",
  "refresh_token": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx...."
}
```

## Urutan pengerjaan yang aku rekomendasikan

Jangan langsung membuat semua fitur authentication sekaligus. Kita selesaikan bertahap:

1. **Password hashing Argon2** ← sekarang
2. **Login endpoint** ← sekarang
3. **JWT access token**
4. **Refresh token + session**
5. **Authentication middleware**
6. **`GET /api/auth/me`**
7. **Logout / revoke session**
8. **Register**
9. Email verification
10. Forgot/reset password

Fondasi database untuk session sebenarnya sudah cukup bagus untuk dilanjutkan ke tahap refresh token dan logout. GitHub+1

**Tapi ada satu hal:** aku belum bisa mengubah langsung file di repository GitHub-mu dari sini. Kalau kamu bekerja di local repo, implementasi di atas bisa langsung kamu tempel. Kalau kamu kirimkan **isi `main.go`, `go.mod`, dan struktur `internal` versi local-mu setelah perubahan terakhir**, aku bisa lanjutkan dari situ dan kita rapikan sampai `go test ./...` benar-benar hijau.

Sources

# Refactor jwt.go ParseRefreshSessionID

```go
func ParseRefreshSessionID(refreshToken string) (uuid.UUID, error) {
	sessionID, random, ok := strings.Cut(refreshToken, ".")
	if !ok || random == "" {
		return uuid.Nil, errors.New("invalid refresh token")
	}

	id, err := uuid.Parse(sessionID)
	if err != nil {
		return uuid.Nil, errors.New("invalid refresh token")
	}

	return id, nil
}
```
