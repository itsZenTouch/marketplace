package database

import (
	"context"
	"testing"
	"time"
)

func TestNewPoolInvalidURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second,
	)
	defer cancel()

	_, err := NewPool(
		ctx,
		"not-a-valid-postgres-url",
		Config{
			MaxConns:        10,
			MinConns:        2,
			MaxConnLifetime: time.Hour,
			MaxConnIdleTime: 30 * time.Minute,
		},
	)

	if err == nil {
		t.Fatal("NewPool() expected error, got nil")
	}
}
