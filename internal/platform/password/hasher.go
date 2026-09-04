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

func NewHasher() *Hasher {
	return &Hasher{}
}

func (h *Hasher) Hash(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	salt := make([]byte, saltLength)

	// Kita akan menggunakan crypto/rand untuk salt.
	// Implementasi lengkapnya kita buat setelah dependency siap.
	_ = salt

	return "", nil
}

func (h *Hasher) Compare(password, encodedHash string) error {
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
