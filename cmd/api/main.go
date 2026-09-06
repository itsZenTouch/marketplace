package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/itsZenTouch/marketplace/internal/auth"
	"github.com/itsZenTouch/marketplace/internal/config"
	appmiddleware "github.com/itsZenTouch/marketplace/internal/middleware"
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

	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
	)

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

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(appmiddleware.Slogger(logger))
	router.Use(middleware.Recoverer)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.Post("/api/auth/login", authHandler.Login)

	router.Group(func(r chi.Router) {
		r.Use(auth.AuthMiddleware(jwtService))

		r.Get("/api/auth/me", authHandler.Me)
	})

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
