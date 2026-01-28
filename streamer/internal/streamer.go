package internal

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gpu-telemetry-pipeline/streamer/pkg/mq/client"
	"github.com/rs/zerolog"
)

// Streamer is the main telemetry streaming service.
type Streamer struct {
	config    Config
	csvReader *CSVReader
	publisher client.Publisher
	logger    zerolog.Logger

	// Metrics
	recordsStreamed atomic.Int64
	errorsCount     atomic.Int64
	startTime       time.Time
}

// New creates a new Streamer instance.
func New(config Config, logger zerolog.Logger) (*Streamer, error) {
	// Create CSV reader
	csvReader, err := NewCSVReader(config.CSVFilePath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSV reader: %w", err)
	}

	// Create MQ publisher
	var publisher client.Publisher
	if config.MQEnabled {
		mqConfig := client.Config{
			BrokerURL:     config.MQBrokerURL,
			Timeout:       10 * time.Second,
			RetryAttempts: 3,
			RetryDelay:    100 * time.Millisecond,
		}
		publisher = client.NewClient(mqConfig, logger)
		logger.Info().
			Str("broker_url", config.MQBrokerURL).
			Msg("MQ publisher enabled")
	} else {
		publisher = client.NewNoOpPublisher(logger)
		logger.Warn().Msg("MQ publisher disabled, using NoOp publisher")
	}

	return &Streamer{
		config:    config,
		csvReader: csvReader,
		publisher: publisher,
		logger:    logger.With().Str("component", "streamer").Logger(),
	}, nil
}

// Run starts the streaming process.
func (s *Streamer) Run(ctx context.Context) error {
	s.startTime = time.Now()
	s.logger.Info().
		Str("csv_file", s.config.CSVFilePath).
		Str("topic", s.config.Topic).
		Dur("stream_interval", s.config.StreamInterval).
		Int("batch_size", s.config.BatchSize).
		Msg("Starting telemetry streamer")

	// Create ticker for streaming interval
	ticker := time.NewTicker(s.config.StreamInterval)
	defer ticker.Stop()

	// Log stats periodically
	statsTicker := time.NewTicker(10 * time.Second)
	defer statsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info().
				Int64("total_records", s.recordsStreamed.Load()).
				Int64("total_errors", s.errorsCount.Load()).
				Dur("uptime", time.Since(s.startTime)).
				Msg("Streamer shutting down")
			return ctx.Err()

		case <-ticker.C:
			// Stream a batch of records
			if err := s.streamBatch(ctx); err != nil {
				s.errorsCount.Add(1)
				s.logger.Error().Err(err).Msg("Failed to stream batch")
			}

		case <-statsTicker.C:
			s.logStats()
		}
	}
}

// streamBatch reads and publishes a batch of telemetry records.
func (s *Streamer) streamBatch(ctx context.Context) error {
	for i := 0; i < s.config.BatchSize; i++ {
		// Check context before each record
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read next record
		record, err := s.csvReader.ReadNext()
		if err != nil {
			return fmt.Errorf("failed to read record: %w", err)
		}

		// Publish to message queue
		if err := s.publisher.Publish(ctx, s.config.Topic, *record); err != nil {
			return fmt.Errorf("failed to publish record: %w", err)
		}

		s.recordsStreamed.Add(1)

		s.logger.Debug().
			Str("metric", record.MetricName).
			Str("gpu_uuid", record.UUID).
			Str("hostname", record.Hostname).
			Float64("value", record.Value).
			Time("timestamp", record.Timestamp).
			Msg("Streamed telemetry record")
	}

	return nil
}

// logStats logs the current streaming statistics.
func (s *Streamer) logStats() {
	uptime := time.Since(s.startTime)
	records := s.recordsStreamed.Load()
	errors := s.errorsCount.Load()

	rate := float64(records) / uptime.Seconds()

	s.logger.Info().
		Int64("records_streamed", records).
		Int64("errors", errors).
		Float64("records_per_second", rate).
		Dur("uptime", uptime).
		Msg("Streamer statistics")
}

// Close closes the streamer and releases resources.
func (s *Streamer) Close() error {
	s.logger.Info().Msg("Closing streamer")

	var errs []error

	if err := s.csvReader.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close CSV reader: %w", err))
	}

	if err := s.publisher.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close publisher: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	return nil
}

// GetStats returns the current streaming statistics.
func (s *Streamer) GetStats() Stats {
	return Stats{
		RecordsStreamed: s.recordsStreamed.Load(),
		ErrorsCount:     s.errorsCount.Load(),
		Uptime:          time.Since(s.startTime),
	}
}

// Stats holds the streaming statistics.
type Stats struct {
	RecordsStreamed int64
	ErrorsCount     int64
	Uptime          time.Duration
}

// RunWithSignalHandler runs the streamer with signal handling for graceful shutdown.
func RunWithSignalHandler(ctx context.Context, s *Streamer) error {
	// Create context with signal handling
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run streamer in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- s.Run(ctx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		s.logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
		cancel()
		// Wait for streamer to finish
		<-errChan
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			return err
		}
	}

	return s.Close()
}

// SetPublisher sets the publisher (useful for testing).
func (s *Streamer) SetPublisher(p client.Publisher) {
	s.publisher = p
}
