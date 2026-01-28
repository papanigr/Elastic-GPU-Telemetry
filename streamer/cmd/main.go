// Package main is the entry point for the Telemetry Streamer service.
package main

import (
	"context"
	"os"
	"strings"

	"github.com/gpu-telemetry-pipeline/streamer/internal"
	"github.com/rs/zerolog"
)

func main() {
	// Load configuration
	config, err := internal.LoadConfig()
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}

	// Set up logger
	logger := setupLogger(config.LogLevel)
	logger.Info().
		Str("csv_file", config.CSVFilePath).
		Str("mq_broker_addr", config.MQBrokerAddr).
		Str("topic", config.Topic).
		Dur("stream_interval", config.StreamInterval).
		Bool("full_file_batch", config.FullFileBatch).
		Int("batch_size", config.BatchSize).
		Bool("mq_enabled", config.MQEnabled).
		Msg("Starting Telemetry Streamer")

	// Create streamer
	s, err := internal.New(config, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create streamer")
	}

	// Run with signal handler
	ctx := context.Background()
	if err := internal.RunWithSignalHandler(ctx, s); err != nil && err != context.Canceled {
		logger.Fatal().Err(err).Msg("Streamer failed")
	}

	logger.Info().Msg("Telemetry Streamer stopped")
}

// setupLogger creates and configures the zerolog logger.
func setupLogger(level string) zerolog.Logger {
	// Set log level
	var logLevel zerolog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = zerolog.DebugLevel
	case "info":
		logLevel = zerolog.InfoLevel
	case "warn", "warning":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	default:
		logLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(logLevel)

	// Create console writer for development
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02T15:04:05.000Z07:00",
	}

	return zerolog.New(output).
		With().
		Timestamp().
		Str("service", "telemetry-streamer").
		Logger()
}
