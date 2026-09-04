package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv  string
	AppPort string

	DatabaseURL string

	DBMaxConns        int32
	DBMinConns        int32
	DBMaxConnLifetime time.Duration
	DBMaxConnIdleTime time.Duration

	JWTSecret     string
	JWTIssuer     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration

	CORSOrigins []string
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		AppPort:     getEnv("APP_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		JWTIssuer:   getEnv("JWT_ISSUER", "marketplace-api"),
		CORSOrigins: getCSVEnv("CORS_ORIGINS"),

		DBMaxConns: getInt32Env("DB_MAX_CONNS", 10),
		DBMinConns: getInt32Env("DB_MIN_CONNS", 2),
	}

	var err error

	cfg.DBMaxConnLifetime, err = getDurationEnv(
		"DB_MAX_CONN_LIFETIME",
		time.Hour,
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"DB_MAX_CONN_LIFETIME: %w",
			err,
		)
	}

	cfg.DBMaxConnIdleTime, err = getDurationEnv(
		"DB_MAX_CONN_IDLE_TIME",
		30*time.Minute,
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"DB_MAX_CONN_IDLE_TIME: %w",
			err,
		)
	}

	cfg.JWTAccessTTL, err = getDurationEnv("JWT_ACCESS_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, fmt.Errorf("JWT_ACCESS_TTL: %w", err)
	}

	cfg.JWTRefreshTTL, err = getDurationEnv("JWT_REFRESH_TTL", 720*time.Hour)
	if err != nil {
		return Config{}, fmt.Errorf("JWT_REFRESH_TTL: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.AppPort == "" {
		return errors.New("APP_PORT is required")
	}

	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	if c.DBMaxConns <= 0 {
		return errors.New("DB_MAX_CONNS must be greater than zero")
	}

	if c.DBMinConns < 0 {
		return errors.New("DB_MIN_CONNS must not be negative")
	}

	if c.DBMinConns > c.DBMaxConns {
		return errors.New(
			"DB_MIN_CONNS must not be greater than DB_MAX_CONNS",
		)
	}

	if c.DBMaxConnLifetime <= 0 {
		return errors.New(
			"DB_MAX_CONN_LIFETIME must be greater than zero",
		)
	}

	if c.DBMaxConnIdleTime <= 0 {
		return errors.New(
			"DB_MAX_CONN_IDLE_TIME must be greater than zero",
		)
	}

	if c.JWTSecret == "" {
		return errors.New("JWT_SECRET is required")
	}

	if len(c.JWTSecret) < 32 {
		return errors.New("JWT_SECRET must be at least 32 characters")
	}

	if c.JWTIssuer == "" {
		return errors.New("JWT_ISSUER is required")
	}

	if c.JWTAccessTTL <= 0 {
		return errors.New("JWT_ACCESS_TTL must be greater than zero")
	}

	if c.JWTRefreshTTL <= 0 {
		return errors.New("JWT_REFRESH_TTL must be greater than zero")
	}

	return nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))

	if value == "" {
		return fallback
	}

	return value
}

func getCSVEnv(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))

	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")

	result := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

func getIntEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))

	if value == "" {
		return fallback
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return result
}

func getInt32Env(key string, fallback int32) int32 {
	value := strings.TrimSpace(os.Getenv(key))

	if value == "" {
		return fallback
	}

	result, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return fallback
	}

	return int32(result)
}

func getDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))

	if value == "" {
		return fallback, nil
	}

	result, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", value)
	}

	return result, nil
}
