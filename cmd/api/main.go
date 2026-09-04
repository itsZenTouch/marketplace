package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itsZenTouch/marketplace/internal/config"
	"github.com/itsZenTouch/marketplace/internal/platform/database"
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
	_ = userRepository

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

	_ = shutdownCtx

	log.Println("server stopped")
}
