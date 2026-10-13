package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	_ "virungapay-be/docs"
	"virungapay-be/internal/config"
	"virungapay-be/internal/database"
	"virungapay-be/internal/mail"
)

func gracefulShutdown(logger zerolog.Logger, apiServer *http.Server, done chan bool) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()

	logger.Info().Msg("shutting down gracefully, press Ctrl+C again to force")
	stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := apiServer.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server forced to shutdown with error")
	}

	logger.Info().Msg("Server exiting")
	done <- true
}

// @title			VirungaPay API
// @version		1.0
// @description	Simulated payments API for the VirungaPay MVP. PWA routes are under /v1. Staff routes are under /admin/v1.
// @host			localhost:8080
// @BasePath		/
// @schemes		http
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Session token from login. A Bearer prefix is optional.
func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Str("service", "virungapay-be").Logger()
	if cfg.AppEnv == "local" || cfg.AppEnv == "test" {
		logger = logger.Level(zerolog.DebugLevel)
	} else {
		logger = logger.Level(zerolog.InfoLevel)
	}
	zerolog.DefaultContextLogger = &logger

	db, err := database.New(cfg.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("database")
	}
	logger.Info().Int("port", cfg.Port).Msg("http server configured")

	apiServer, err := newServer(cfg, db, mail.NewMailjet(cfg.Mail))
	if err != nil {
		logger.Fatal().Err(err).Msg("http server")
	}
	apiServer.RegisterOnShutdown(func() {
		if err := db.Close(); err != nil {
			logger.Error().Err(err).Msg("close database")
		}
	})

	done := make(chan bool, 1)
	go gracefulShutdown(logger, apiServer, done)

	err = apiServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal().Err(err).Msg("http server error")
	}

	<-done
	logger.Info().Msg("Graceful shutdown complete.")
}
