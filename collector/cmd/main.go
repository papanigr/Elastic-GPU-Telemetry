package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gpu-telemetry-pipeline/collector/internal"
	"github.com/gpu-telemetry-pipeline/collector/internal/consumer"
	"github.com/gpu-telemetry-pipeline/collector/internal/repository"
	"github.com/rs/zerolog"
)

func main() {
	// Load configuration
	cfg, err := internal.LoadConfig()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	// Setup logger
	level, _ := zerolog.ParseLevel(cfg.LogLevel)
	zerolog.SetGlobalLevel(level)
	logger := zerolog.New(os.Stdout).
		With().
		Timestamp().
		Str("service", "telemetry-collector").
		Logger()

	logger.Info().
		Str("mq_broker_addr", cfg.MQBrokerAddr).
		Str("topic", cfg.Topic).
		Str("consumer_group", cfg.ConsumerGroup).
		Int("batch_size", cfg.BatchSize).
		Dur("poll_interval", cfg.PollInterval).
		Int("num_workers", cfg.NumWorkers).
		Bool("db_enabled", cfg.DBEnabled).
		Msg("Starting Telemetry Collector")

	// Create repository
	var repo repository.Repository
	if cfg.DBEnabled {
		pgConfig := repository.DefaultPostgresConfig(cfg.DatabaseURL)
		pgRepo, err := repository.NewPostgresRepository(pgConfig, logger)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to create PostgreSQL repository")
		}
		repo = pgRepo
		logger.Info().Str("connection", "***hidden***").Msg("PostgreSQL repository enabled")
	} else {
		logger.Warn().Msg("Database disabled, using NoOp repository (printing to console)")
		repo = repository.NewNoOpRepository(logger)
	}
	defer repo.Close()

	// Create consumer with worker pool
	cons, err := consumer.New(consumer.Config{
		MQBrokerAddr:  cfg.MQBrokerAddr,
		Topic:         cfg.Topic,
		ConsumerGroup: cfg.ConsumerGroup,
		ConsumerID:    cfg.ConsumerID,
		BatchSize:     cfg.BatchSize,
		PollInterval:  cfg.PollInterval,
		NumWorkers:    cfg.NumWorkers,
	}, repo, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create consumer")
	}
	defer cons.Close()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to MQ with timeout
	connectCtx, connectCancel := context.WithTimeout(ctx, 30*time.Second)
	if err := cons.Connect(connectCtx); err != nil {
		connectCancel()
		logger.Fatal().Err(err).Msg("Failed to connect to MQ broker")
	}
	connectCancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run consumer in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- cons.Run(ctx)
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
		cancel()
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			logger.Error().Err(err).Msg("Consumer error")
		}
	}

	// Wait for graceful shutdown
	<-errChan

	logger.Info().Msg("Telemetry Collector stopped")
}
