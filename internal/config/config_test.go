package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("APP_PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test")
	t.Setenv("DB_MAX_CONNS", "10")
	t.Setenv("DB_MIN_CONNS", "2")
	t.Setenv("DB_MAX_CONN_LIFETIME", "1h")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "30m")

	t.Setenv("JWT_SECRET", "this-is-a-development-secret-with-more-than-32-chars")
	t.Setenv("JWT_ISSUER", "test-api")
	t.Setenv("JWT_ACCESS_TTL", "15m")
	t.Setenv("JWT_REFRESH_TTL", "720h")
	t.Setenv("CORS_ORIGINS", "http://localhost:3000,https://example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppEnv != "test" {
		t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, "test")
	}

	if cfg.AppPort != "9090" {
		t.Errorf("AppPort = %q, want %q", cfg.AppPort, "9090")
	}

	if cfg.JWTAccessTTL != 15*time.Minute {
		t.Errorf(
			"JWTAccessTTL = %v, want %v",
			cfg.JWTAccessTTL,
			15*time.Minute,
		)
	}

	if cfg.JWTRefreshTTL != 720*time.Hour {
		t.Errorf(
			"JWTRefreshTTL = %v, want %v",
			cfg.JWTRefreshTTL,
			720*time.Hour,
		)
	}

	if len(cfg.CORSOrigins) != 2 {
		t.Fatalf(
			"CORSOrigins length = %d, want 2",
			len(cfg.CORSOrigins),
		)
	}

	if cfg.DBMaxConns != 10 {
		t.Errorf("DBMaxConns = %d, want 10", cfg.DBMaxConns)
	}

	if cfg.DBMinConns != 2 {
		t.Errorf("DBMinConns = %d, want 2", cfg.DBMinConns)
	}

	if cfg.DBMaxConnLifetime != time.Hour {
		t.Errorf(
			"DBMaxConnLifetime = %v, want %v",
			cfg.DBMaxConnLifetime,
			time.Hour,
		)
	}

	if cfg.DBMaxConnIdleTime != 30*time.Minute {
		t.Errorf(
			"DBMaxConnIdleTime = %v, want %v",
			cfg.DBMaxConnIdleTime,
			30*time.Minute,
		)
	}
}

func TestValidateRequiresJWTSecret(t *testing.T) {
	cfg := Config{
		AppPort:       "8080",
		DatabaseURL:   "postgres://localhost/test",
		JWTSecret:     "",
		JWTIssuer:     "marketplace-api",
		JWTAccessTTL:  15 * time.Minute,
		JWTRefreshTTL: 720 * time.Hour,
	}

	err := cfg.Validate()

	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
}

func TestValidateRejectsWeakJWTSecret(t *testing.T) {
	cfg := Config{
		AppPort:       "8080",
		DatabaseURL:   "postgres://localhost/test",
		JWTSecret:     "too-short",
		JWTIssuer:     "marketplace-api",
		JWTAccessTTL:  15 * time.Minute,
		JWTRefreshTTL: 720 * time.Hour,
	}

	err := cfg.Validate()

	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
}

func TestGetCSVEnv(t *testing.T) {
	const key = "TEST_CORS"

	t.Setenv(key, "http://localhost:3000, https://example.com")

	got := getCSVEnv(key)

	if len(got) != 2 {
		t.Fatalf("got %d origins, want 2", len(got))
	}

	if got[0] != "http://localhost:3000" {
		t.Errorf("got %q, want %q", got[0], "http://localhost:3000")
	}

	if got[1] != "https://example.com" {
		t.Errorf("got %q, want %q", got[1], "https://example.com")
	}

	_ = os.Unsetenv(key)
}
