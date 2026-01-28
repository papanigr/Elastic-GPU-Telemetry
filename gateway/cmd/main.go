package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gpu-telemetry-pipeline/gateway/internal/config"
	"github.com/gpu-telemetry-pipeline/gateway/internal/handlers"
	"github.com/gpu-telemetry-pipeline/gateway/internal/repository"
	"github.com/gpu-telemetry-pipeline/gateway/internal/router"
	"github.com/rs/zerolog"

	_ "github.com/gpu-telemetry-pipeline/gateway/docs" // Auto-generated swagger docs
)

// @title           GPU Telemetry API
// @version         1.0.0
// @description     REST API for querying GPU telemetry data from the GPU Telemetry Pipeline.

// @contact.name    GPU Telemetry Pipeline

// @host            localhost:8080
// @BasePath        /

// @schemes         http https

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	level, _ := zerolog.ParseLevel(cfg.LogLevel)
	zerolog.SetGlobalLevel(level)

	logger := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}).With().
		Timestamp().
		Str("service", "api-gateway").
		Logger()

	logger.Info().
		Int("http_port", cfg.HTTPPort).
		Int("default_page_size", cfg.DefaultPageSize).
		Int("max_page_size", cfg.MaxPageSize).
		Msg("Starting API Gateway")

	// Create repository
	repoCfg := repository.Config{
		ConnectionString: cfg.DatabaseURL,
		MaxOpenConns:     cfg.DBMaxOpenConns,
		MaxIdleConns:     cfg.DBMaxIdleConns,
		ConnMaxLifetime:  cfg.DBConnMaxLife,
	}

	repo, err := repository.NewPostgresRepository(repoCfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer repo.Close()

	logger.Info().Msg("Connected to PostgreSQL")

	// Create handlers
	handler := handlers.NewHandler(repo, logger, cfg.DefaultPageSize, cfg.MaxPageSize)

	// Create router
	r := router.New(handler, logger)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info().Str("addr", server.Addr).Msg("HTTP server listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("API Gateway stopped")
}
