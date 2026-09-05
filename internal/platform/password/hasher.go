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
		return "", fmt.Errorf("password cannot be empty")
	}

	salt := make([]byte, saltLength)

	// Kita akan menggunakan crypto/rand untuk salt.
	// Implementasi lengkapnya kita buat setelah dependency siap.
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	key := deriveKey(rawPassword, salt)

	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memoryCost,
		timeCost,
		parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

func (h *Hasher) Compare(rawPassword, encodedHash string) error {
	parts := strings.Split(encodedHash, "$")

	if len(parts) != 5 {
		return errors.New("invalid password salt")
	}

	if parts[0] != "argon2id" || parts[1] != "v=19" {
		return errors.New("unsupported password hash")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return errors.New("invalid password salt")
	}

	expected, err := base64.RawStdEncoding.DecodeString(parts[4])
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
