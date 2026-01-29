// Package main is the entry point for the MQ Broker service.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gpu-telemetry-pipeline/mq/internal/api"
	"github.com/gpu-telemetry-pipeline/mq/internal/broker"
	"github.com/gpu-telemetry-pipeline/mq/internal/config"
	grpcserver "github.com/gpu-telemetry-pipeline/mq/internal/grpc"
	"github.com/gpu-telemetry-pipeline/mq/pkg/pb"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	_ "github.com/gpu-telemetry-pipeline/mq/docs" // Auto-generated swagger docs
)

// @title           MQ Broker API
// @version         1.0.0
// @description     Custom Message Queue Broker with topic-based pub/sub, consumer groups, and at-least-once delivery.

// @contact.name    GPU Telemetry Pipeline

// @host            localhost:8082
// @BasePath        /

// @schemes         http

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}

	// Set up logger
	logger := setupLogger(cfg.LogLevel)
	logger.Info().
		Int("grpc_port", cfg.GRPCPort).
		Int("http_port", cfg.HTTPPort).
		Int("max_queue_size", cfg.MaxQueueSize).
		Dur("max_message_age", cfg.MaxMessageAge).
		Dur("ack_timeout", cfg.AckTimeout).
		Bool("dlq_enabled", cfg.DLQEnabled).
		Int("max_retries", cfg.MaxRetries).
		Int("dlq_max_retries", cfg.DLQMaxRetries).
		Dur("dlq_retry_delay", cfg.DLQRetryDelay).
		Msg("Starting MQ Broker")

	// Create broker
	b := broker.NewBroker(cfg, logger)
	defer b.Close()

	// Start gRPC server
	grpcAddr := fmt.Sprintf(":%d", cfg.GRPCPort)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to listen for gRPC")
	}

	grpcSrv := grpc.NewServer()
	pb.RegisterMQServiceServer(grpcSrv, grpcserver.NewServer(b, logger))

	go func() {
		logger.Info().Str("addr", grpcAddr).Msg("gRPC server listening")
		if err := grpcSrv.Serve(grpcListener); err != nil {
			logger.Error().Err(err).Msg("gRPC server error")
		}
	}()

	// Create HTTP router for admin APIs
	router := api.NewRouter(b, logger)

	// Create HTTP server
	httpAddr := fmt.Sprintf(":%d", cfg.HTTPPort)
	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server
	go func() {
		logger.Info().Str("addr", httpAddr).Msg("HTTP server listening (admin APIs)")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("HTTP server error")
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan

	logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop gRPC server
	grpcSrv.GracefulStop()
	logger.Info().Msg("gRPC server stopped")

	// Stop HTTP server
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("HTTP server shutdown error")
	}
	logger.Info().Msg("HTTP server stopped")

	logger.Info().Msg("MQ Broker stopped")
}

// setupLogger creates and configures the zerolog logger.
func setupLogger(level string) zerolog.Logger {
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

	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02T15:04:05.000Z07:00",
	}

	return zerolog.New(output).
		With().
		Timestamp().
		Str("service", "mq-broker").
		Logger()
}
