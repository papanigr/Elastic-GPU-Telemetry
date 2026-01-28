package worker

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gpu-telemetry-pipeline/collector/internal/models"
	"github.com/gpu-telemetry-pipeline/collector/internal/repository"
	"github.com/gpu-telemetry-pipeline/collector/pkg/pb"
	"github.com/rs/zerolog"
)

// WorkItem represents a batch of messages to process.
type WorkItem struct {
	Messages []*pb.Message
}

// Result represents the result of processing a work item.
type Result struct {
	MessageIDs []string
	Count      int
	Error      error
}

// Pool manages a pool of workers that process messages concurrently.
type Pool struct {
	config      Config
	repo        repository.Repository
	logger      zerolog.Logger
	workChan    chan WorkItem
	resultChan  chan Result
	wg          sync.WaitGroup

	// Statistics
	processed   atomic.Int64
	errors      atomic.Int64
	startTime   time.Time
}

// Config holds worker pool configuration.
type Config struct {
	NumWorkers int           // Number of worker goroutines
	BufferSize int           // Size of work channel buffer
}

// DefaultConfig returns default worker pool configuration.
func DefaultConfig() Config {
	return Config{
		NumWorkers: 3,
		BufferSize: 10,
	}
}

// NewPool creates a new worker pool.
func NewPool(cfg Config, repo repository.Repository, logger zerolog.Logger) *Pool {
	if cfg.NumWorkers <= 0 {
		cfg.NumWorkers = DefaultConfig().NumWorkers
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = DefaultConfig().BufferSize
	}

	return &Pool{
		config:     cfg,
		repo:       repo,
		logger:     logger.With().Str("component", "worker-pool").Logger(),
		workChan:   make(chan WorkItem, cfg.BufferSize),
		resultChan: make(chan Result, cfg.BufferSize),
		startTime:  time.Now(),
	}
}

// Start starts the worker pool.
func (p *Pool) Start(ctx context.Context) {
	p.logger.Info().
		Int("num_workers", p.config.NumWorkers).
		Int("buffer_size", p.config.BufferSize).
		Msg("Starting worker pool")

	for i := 0; i < p.config.NumWorkers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i+1)
	}
}

// Stop stops the worker pool and waits for workers to finish.
func (p *Pool) Stop() {
	close(p.workChan)
	p.wg.Wait()
	close(p.resultChan)
	
	p.logger.Info().
		Int64("total_processed", p.processed.Load()).
		Int64("total_errors", p.errors.Load()).
		Dur("uptime", time.Since(p.startTime)).
		Msg("Worker pool stopped")
}

// Submit submits a work item to the pool.
// Returns false if the context is cancelled or pool is shutting down.
func (p *Pool) Submit(ctx context.Context, item WorkItem) bool {
	select {
	case <-ctx.Done():
		return false
	case p.workChan <- item:
		return true
	}
}

// Results returns the channel for receiving processing results.
func (p *Pool) Results() <-chan Result {
	return p.resultChan
}

// worker is the worker goroutine that processes messages.
func (p *Pool) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	
	logger := p.logger.With().Int("worker_id", id).Logger()
	logger.Debug().Msg("Worker started")

	for {
		select {
		case <-ctx.Done():
			logger.Debug().Msg("Worker shutting down (context cancelled)")
			return
		case item, ok := <-p.workChan:
			if !ok {
				logger.Debug().Msg("Worker shutting down (channel closed)")
				return
			}
			result := p.processItem(ctx, logger, item)
			
			// Send result back
			select {
			case <-ctx.Done():
				return
			case p.resultChan <- result:
			}
		}
	}
}

// processItem processes a work item (batch of messages).
func (p *Pool) processItem(ctx context.Context, logger zerolog.Logger, item WorkItem) Result {
	result := Result{
		MessageIDs: make([]string, 0, len(item.Messages)),
	}

	// Parse messages
	records := make([]models.GPUTelemetry, 0, len(item.Messages))
	for _, msg := range item.Messages {
		record, err := p.parseMessage(msg)
		if err != nil {
			logger.Warn().
				Err(err).
				Str("message_id", msg.Id).
				Msg("Failed to parse message, skipping")
			continue
		}
		records = append(records, record)
		result.MessageIDs = append(result.MessageIDs, msg.Id)
	}

	if len(records) == 0 {
		return result
	}

	// Save to repository
	if err := p.repo.Save(ctx, records); err != nil {
		p.errors.Add(1)
		result.Error = err
		logger.Error().Err(err).Int("count", len(records)).Msg("Failed to save records")
		return result
	}

	result.Count = len(records)
	p.processed.Add(int64(len(records)))

	logger.Debug().
		Int("count", len(records)).
		Msg("Batch processed by worker")

	return result
}

// parseMessage parses a message payload into a GPUTelemetry record.
func (p *Pool) parseMessage(msg *pb.Message) (models.GPUTelemetry, error) {
	var payload models.RawTelemetryPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return models.GPUTelemetry{}, err
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339Nano, payload.Timestamp)
	if err != nil {
		timestamp, err = time.Parse(time.RFC3339, payload.Timestamp)
		if err != nil {
			timestamp = time.Now()
		}
	}

	return models.GPUTelemetry{
		ID:         msg.Id, // Use message ID as record ID
		Timestamp:  timestamp,
		MetricName: payload.MetricName,
		GPUIndex:   payload.GPUIndex,
		Device:     payload.Device,
		UUID:       payload.UUID,
		ModelName:  payload.ModelName,
		Hostname:   payload.Hostname,
		Container:  payload.Container,
		Pod:        payload.Pod,
		Namespace:  payload.Namespace,
		Value:      payload.Value,
		LabelsRaw:  payload.LabelsRaw,
		ReceivedAt: time.Now(),
		MessageID:  msg.Id,
	}, nil
}

// Stats returns pool statistics.
type Stats struct {
	NumWorkers    int
	Processed     int64
	Errors        int64
	Uptime        time.Duration
	PendingWork   int
}

// GetStats returns current pool statistics.
func (p *Pool) GetStats() Stats {
	return Stats{
		NumWorkers:  p.config.NumWorkers,
		Processed:   p.processed.Load(),
		Errors:      p.errors.Load(),
		Uptime:      time.Since(p.startTime),
		PendingWork: len(p.workChan),
	}
}
