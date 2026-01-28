package consumer

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gpu-telemetry-pipeline/collector/internal/repository"
	"github.com/gpu-telemetry-pipeline/collector/internal/worker"
	"github.com/gpu-telemetry-pipeline/collector/pkg/pb"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Consumer consumes messages from the MQ and persists them using a worker pool.
type Consumer struct {
	config     Config
	repo       repository.Repository
	logger     zerolog.Logger
	client     pb.MQServiceClient
	conn       *grpc.ClientConn
	workerPool *worker.Pool

	// Statistics
	messagesConsumed atomic.Int64
	batchesProcessed atomic.Int64
	messagesAcked    atomic.Int64
	errorsCount      atomic.Int64
	startTime        time.Time
}

// Config holds consumer configuration.
type Config struct {
	MQBrokerAddr  string
	Topic         string
	ConsumerGroup string
	ConsumerID    string
	BatchSize     int
	PollInterval  time.Duration
	NumWorkers    int // Number of worker goroutines
}

// New creates a new consumer with worker pool.
func New(cfg Config, repo repository.Repository, logger zerolog.Logger) (*Consumer, error) {
	// Generate consumer ID if not provided
	if cfg.ConsumerID == "" {
		cfg.ConsumerID = fmt.Sprintf("collector-%s", uuid.New().String()[:8])
	}

	// Default workers if not specified
	if cfg.NumWorkers <= 0 {
		cfg.NumWorkers = 3
	}

	// Create worker pool
	pool := worker.NewPool(worker.Config{
		NumWorkers: cfg.NumWorkers,
		BufferSize: cfg.NumWorkers * 2, // Buffer 2 batches per worker
	}, repo, logger)

	c := &Consumer{
		config:     cfg,
		repo:       repo,
		logger:     logger.With().Str("component", "consumer").Str("consumer_id", cfg.ConsumerID).Logger(),
		workerPool: pool,
		startTime:  time.Now(),
	}

	return c, nil
}

// Connect establishes connection to the MQ broker.
func (c *Consumer) Connect(ctx context.Context) error {
	c.logger.Info().
		Str("addr", c.config.MQBrokerAddr).
		Msg("Connecting to MQ broker")

	conn, err := grpc.DialContext(ctx, c.config.MQBrokerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to MQ broker: %w", err)
	}

	c.conn = conn
	c.client = pb.NewMQServiceClient(conn)

	// Check health
	healthResp, err := c.client.Health(ctx, &pb.HealthRequest{})
	if err != nil {
		return fmt.Errorf("MQ broker health check failed: %w", err)
	}
	c.logger.Info().Str("status", healthResp.Status).Msg("MQ broker is healthy")

	// Subscribe to topic
	subResp, err := c.client.Subscribe(ctx, &pb.SubscribeRequest{
		Topic:      c.config.Topic,
		ConsumerId: c.config.ConsumerID,
		Group:      c.config.ConsumerGroup,
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}
	c.logger.Info().
		Str("topic", c.config.Topic).
		Str("group", c.config.ConsumerGroup).
		Str("status", subResp.Status).
		Msg("Subscribed to topic")

	return nil
}

// Run starts the consumer loop with worker pool.
func (c *Consumer) Run(ctx context.Context) error {
	c.logger.Info().
		Str("topic", c.config.Topic).
		Str("group", c.config.ConsumerGroup).
		Int("batch_size", c.config.BatchSize).
		Int("num_workers", c.config.NumWorkers).
		Dur("poll_interval", c.config.PollInterval).
		Msg("Starting consumer with worker pool")

	// Start worker pool
	c.workerPool.Start(ctx)

	// Start result processor (handles acks)
	go c.processResults(ctx)

	// Start fetcher loop
	return c.fetchLoop(ctx)
}

// fetchLoop continuously fetches messages from MQ and submits to worker pool.
func (c *Consumer) fetchLoop(ctx context.Context) error {
	ticker := time.NewTicker(c.config.PollInterval)
	defer ticker.Stop()

	// Log stats periodically
	statsTicker := time.NewTicker(10 * time.Second)
	defer statsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info().
				Int64("total_messages", c.messagesConsumed.Load()).
				Int64("total_acked", c.messagesAcked.Load()).
				Int64("total_batches", c.batchesProcessed.Load()).
				Int64("total_errors", c.errorsCount.Load()).
				Dur("uptime", time.Since(c.startTime)).
				Msg("Consumer shutting down")
			c.workerPool.Stop()
			return ctx.Err()

		case <-ticker.C:
			if err := c.fetchAndSubmit(ctx); err != nil {
				c.errorsCount.Add(1)
				c.logger.Error().Err(err).Msg("Failed to fetch messages")
			}

		case <-statsTicker.C:
			c.logStats()
		}
	}
}

// fetchAndSubmit fetches a batch of messages and submits to worker pool.
func (c *Consumer) fetchAndSubmit(ctx context.Context) error {
	// Consume messages from MQ
	resp, err := c.client.Consume(ctx, &pb.ConsumeRequest{
		Topic:       c.config.Topic,
		ConsumerId:  c.config.ConsumerID,
		Group:       c.config.ConsumerGroup,
		MaxMessages: int32(c.config.BatchSize),
	})
	if err != nil {
		return fmt.Errorf("failed to consume messages: %w", err)
	}

	if len(resp.Messages) == 0 {
		c.logger.Debug().Msg("No messages to consume")
		return nil
	}

	c.messagesConsumed.Add(int64(len(resp.Messages)))
	c.batchesProcessed.Add(1)

	c.logger.Debug().
		Int("count", len(resp.Messages)).
		Msg("Fetched messages, submitting to worker pool")

	// Submit to worker pool
	if !c.workerPool.Submit(ctx, worker.WorkItem{Messages: resp.Messages}) {
		return fmt.Errorf("failed to submit work item (context cancelled)")
	}

	return nil
}

// processResults processes results from workers and sends acks to MQ.
func (c *Consumer) processResults(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-c.workerPool.Results():
			if !ok {
				return
			}

			if result.Error != nil {
				c.errorsCount.Add(1)
				c.logger.Error().
					Err(result.Error).
					Int("message_count", len(result.MessageIDs)).
					Msg("Worker reported error, messages will be redelivered")
				continue
			}

			if len(result.MessageIDs) == 0 {
				continue
			}

			// Acknowledge processed messages
			ackResp, err := c.client.Ack(ctx, &pb.AckRequest{
				Topic:      c.config.Topic,
				ConsumerId: c.config.ConsumerID,
				MessageIds: result.MessageIDs,
			})
			if err != nil {
				c.errorsCount.Add(1)
				c.logger.Error().
					Err(err).
					Int("message_count", len(result.MessageIDs)).
					Msg("Failed to acknowledge messages")
				continue
			}

			c.messagesAcked.Add(int64(ackResp.AckedCount))
			c.logger.Debug().
				Int32("acked", ackResp.AckedCount).
				Int("processed", result.Count).
				Msg("Messages acknowledged")
		}
	}
}

// logStats logs consumer and worker pool statistics.
func (c *Consumer) logStats() {
	uptime := time.Since(c.startTime)
	consumed := c.messagesConsumed.Load()
	acked := c.messagesAcked.Load()
	rate := float64(consumed) / uptime.Seconds()

	poolStats := c.workerPool.GetStats()

	c.logger.Info().
		Int64("messages_consumed", consumed).
		Int64("messages_acked", acked).
		Int64("messages_processed", poolStats.Processed).
		Int64("batches_fetched", c.batchesProcessed.Load()).
		Int64("errors", c.errorsCount.Load()).
		Int("workers", poolStats.NumWorkers).
		Int("pending_work", poolStats.PendingWork).
		Float64("messages_per_second", rate).
		Dur("uptime", uptime).
		Msg("Consumer statistics")
}

// Close closes the consumer connection.
func (c *Consumer) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Stats returns consumer statistics.
type Stats struct {
	MessagesConsumed int64
	MessagesAcked    int64
	BatchesProcessed int64
	ErrorsCount      int64
	NumWorkers       int
	Uptime           time.Duration
}

// GetStats returns current statistics.
func (c *Consumer) GetStats() Stats {
	return Stats{
		MessagesConsumed: c.messagesConsumed.Load(),
		MessagesAcked:    c.messagesAcked.Load(),
		BatchesProcessed: c.batchesProcessed.Load(),
		ErrorsCount:      c.errorsCount.Load(),
		NumWorkers:       c.config.NumWorkers,
		Uptime:           time.Since(c.startTime),
	}
}
