// Package client provides the message queue client for publishing messages.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gpu-telemetry-pipeline/streamer/pkg/models"
	"github.com/gpu-telemetry-pipeline/streamer/pkg/pb"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCClient implements the Publisher interface using gRPC.
type GRPCClient struct {
	conn   *grpc.ClientConn
	client pb.MQServiceClient
	logger zerolog.Logger
}

// GRPCConfig holds the configuration for the gRPC client.
type GRPCConfig struct {
	// BrokerAddr is the address of the MQ broker (host:port).
	BrokerAddr string
	// Timeout is the call timeout.
	Timeout time.Duration
}

// DefaultGRPCConfig returns the default gRPC configuration.
func DefaultGRPCConfig() GRPCConfig {
	return GRPCConfig{
		BrokerAddr: "localhost:8081",
		Timeout:    10 * time.Second,
	}
}

// NewGRPCClient creates a new gRPC client.
func NewGRPCClient(cfg GRPCConfig, logger zerolog.Logger) (*GRPCClient, error) {
	conn, err := grpc.NewClient(
		cfg.BrokerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MQ broker: %w", err)
	}

	return &GRPCClient{
		conn:   conn,
		client: pb.NewMQServiceClient(conn),
		logger: logger.With().Str("component", "grpc-client").Logger(),
	}, nil
}

// Publish sends a telemetry record to the specified topic via gRPC.
func (c *GRPCClient) Publish(ctx context.Context, topic string, record models.TelemetryRecord) error {
	// Serialize the record to JSON for the payload
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// Create the protobuf request
	req := &pb.PublishRequest{
		Topic:     topic,
		Id:        uuid.New().String(),
		Payload:   payload,
		Timestamp: timestamppb.Now(),
	}

	// Call the gRPC method
	resp, err := c.client.Publish(ctx, req)
	if err != nil {
		c.logger.Warn().Err(err).Str("topic", topic).Msg("gRPC publish failed")
		return fmt.Errorf("gRPC publish failed: %w", err)
	}

	if resp.Status != "ok" {
		return fmt.Errorf("publish returned status: %s", resp.Status)
	}

	c.logger.Debug().
		Str("topic", topic).
		Str("message_id", resp.MessageId).
		Msg("Published via gRPC")

	return nil
}

// Close closes the gRPC connection.
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
