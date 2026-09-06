package main

import (
	"context"
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
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
	)

	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error(
			"failed to load configuration",
			slog.Any("error", err),
		)
		os.Exit(1)
	}

	ctx := context.Background()

	dbPool, err := database.NewPool(ctx, cfg.DatabaseURL, database.Config{
		MaxConns:        cfg.DBMaxConns,
		MinConns:        cfg.DBMinConns,
		MaxConnLifetime: cfg.DBMaxConnLifetime,
		MaxConnIdleTime: cfg.DBMaxConnIdleTime,
	})
	if err != nil {
		logger.Error(
			"failed to connect to database",
			slog.Any("error", err),
		)
		os.Exit(1)
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
		logger,
	)

	authHandler := auth.NewHandler(authService)

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(appmiddleware.RealIP)
	router.Use(appmiddleware.Slogger(logger))
	router.Use(appmiddleware.Recoverer)
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
		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			logger.Error(
				"server failed:",
				slog.Any("error", err),
			)
		}
	}()

	logger.Info("database connected")

	logger.Info(
		"marketplace API starting",
		slog.String("port", cfg.AppPort),
		slog.String("env", cfg.AppEnv),
	)

	stop := make(chan os.Signal, 1)

	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed:",
			slog.Any("error", err))
	}

	logger.Info("server stopped")
}
