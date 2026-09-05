package main

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

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failled to load configuration: %v", err)
	}

	ctx := context.Background()

	dbPool, err := database.NewPool(ctx, cfg.DatabaseURL, database.Config{
		MaxConns:        cfg.DBMaxConns,
		MinConns:        cfg.DBMinConns,
		MaxConnLifetime: cfg.DBMaxConnLifetime,
		MaxConnIdleTime: cfg.DBMaxConnIdleTime,
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	defer dbPool.Close()

	userRepository := repository.NewUserRepository(dbPool)

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

	router := chi.NewRouter()

	router.Post("api/auth/login", authHandler.Login)

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

	log.Println("database connection successfully")

	log.Printf(
		"marketplace API starting on port %s (%s)",
		cfg.AppPort,
		cfg.AppEnv,
	)

	stop := make(chan os.Signal, 1)

	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown failed: %v", err)
	}

	log.Println("server stopped")
}
