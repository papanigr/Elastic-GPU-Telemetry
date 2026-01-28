// Package client provides the message queue client for publishing and subscribing to messages.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gpu-telemetry-pipeline/streamer/pkg/models"
	"github.com/rs/zerolog"
)

// Publisher defines the interface for publishing messages to the message queue.
type Publisher interface {
	// Publish sends a message to the specified topic.
	Publish(ctx context.Context, topic string, record models.TelemetryRecord) error
	// Close closes the publisher connection.
	Close() error
}

// Config holds the configuration for the MQ client.
type Config struct {
	// BrokerURL is the URL of the message queue broker.
	BrokerURL string
	// Timeout is the HTTP request timeout.
	Timeout time.Duration
	// RetryAttempts is the number of retry attempts for failed publishes.
	RetryAttempts int
	// RetryDelay is the initial delay between retries.
	RetryDelay time.Duration
}

// DefaultConfig returns the default configuration for the MQ client.
func DefaultConfig() Config {
	return Config{
		BrokerURL:     "http://localhost:8081",
		Timeout:       10 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    100 * time.Millisecond,
	}
}

// Client implements the Publisher interface using HTTP.
type Client struct {
	config     Config
	httpClient *http.Client
	logger     zerolog.Logger
}

// NewClient creates a new MQ client.
func NewClient(config Config, logger zerolog.Logger) *Client {
	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		logger: logger.With().Str("component", "mq-client").Logger(),
	}
}

// Publish sends a telemetry record to the specified topic.
func (c *Client) Publish(ctx context.Context, topic string, record models.TelemetryRecord) error {
	// Create the message
	msg := models.Message{
		ID:        uuid.New().String(),
		Topic:     topic,
		Payload:   record,
		Timestamp: time.Now(),
	}

	// Serialize the message
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Build the URL
	url := fmt.Sprintf("%s/api/v1/topics/%s/messages", c.config.BrokerURL, topic)

	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			delay := c.config.RetryDelay * time.Duration(1<<(attempt-1)) // Exponential backoff
			c.logger.Debug().
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("Retrying publish")
			
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err = c.doPublish(ctx, url, payload)
		if err == nil {
			return nil
		}
		lastErr = err
		c.logger.Warn().
			Err(err).
			Int("attempt", attempt+1).
			Str("topic", topic).
			Msg("Publish failed")
	}

	return fmt.Errorf("failed to publish after %d attempts: %w", c.config.RetryAttempts+1, lastErr)
}

// doPublish performs the actual HTTP request to publish a message.
func (c *Client) doPublish(ctx context.Context, url string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("publish failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// NoOpPublisher is a no-op publisher for testing or when MQ is disabled.
type NoOpPublisher struct {
	logger zerolog.Logger
}

// NewNoOpPublisher creates a new no-op publisher.
func NewNoOpPublisher(logger zerolog.Logger) *NoOpPublisher {
	return &NoOpPublisher{
		logger: logger.With().Str("component", "noop-publisher").Logger(),
	}
}

// Publish logs the message but doesn't actually send it anywhere.
func (p *NoOpPublisher) Publish(ctx context.Context, topic string, record models.TelemetryRecord) error {
	p.logger.Debug().
		Str("topic", topic).
		Str("metric", record.MetricName).
		Str("gpu_uuid", record.UUID).
		Float64("value", record.Value).
		Msg("NoOp publish (message not sent)")
	return nil
}

// Close is a no-op.
func (p *NoOpPublisher) Close() error {
	return nil
}
