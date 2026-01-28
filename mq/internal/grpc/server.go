// Package grpc provides the gRPC server implementation for the MQ broker.
package grpc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gpu-telemetry-pipeline/mq/internal/broker"
	"github.com/gpu-telemetry-pipeline/mq/pkg/models"
	"github.com/gpu-telemetry-pipeline/mq/pkg/pb"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the MQServiceServer interface.
type Server struct {
	pb.UnimplementedMQServiceServer
	broker *broker.Broker
	logger zerolog.Logger
}

// NewServer creates a new gRPC server.
func NewServer(b *broker.Broker, logger zerolog.Logger) *Server {
	return &Server{
		broker: b,
		logger: logger.With().Str("component", "grpc-server").Logger(),
	}
}

// Publish handles publishing a message to a topic.
func (s *Server) Publish(ctx context.Context, req *pb.PublishRequest) (*pb.PublishResponse, error) {
	if req.Topic == "" {
		return &pb.PublishResponse{Status: "error"}, nil
	}

	// Generate ID if not provided
	msgID := req.Id
	if msgID == "" {
		msgID = uuid.New().String()
	}

	// Use current time if not provided
	var timestamp time.Time
	if req.Timestamp != nil {
		timestamp = req.Timestamp.AsTime()
	} else {
		timestamp = time.Now()
	}

	// Create message
	msg := &models.Message{
		ID:        msgID,
		Topic:     req.Topic,
		Payload:   req.Payload,
		Timestamp: timestamp,
	}

	if err := s.broker.Publish(req.Topic, msg); err != nil {
		s.logger.Error().Err(err).Str("topic", req.Topic).Msg("Failed to publish")
		return &pb.PublishResponse{Status: "error"}, nil
	}

	s.logger.Debug().
		Str("topic", req.Topic).
		Str("message_id", msgID).
		Msg("Message published via gRPC")

	return &pb.PublishResponse{
		Status:    "ok",
		MessageId: msgID,
	}, nil
}

// Subscribe handles subscribing a consumer to a topic.
func (s *Server) Subscribe(ctx context.Context, req *pb.SubscribeRequest) (*pb.SubscribeResponse, error) {
	if req.Topic == "" || req.ConsumerId == "" || req.Group == "" {
		return &pb.SubscribeResponse{Status: "error"}, nil
	}

	if err := s.broker.Subscribe(req.Topic, req.ConsumerId, req.Group); err != nil {
		s.logger.Error().Err(err).Msg("Failed to subscribe")
		return &pb.SubscribeResponse{Status: "error"}, nil
	}

	s.logger.Debug().
		Str("topic", req.Topic).
		Str("consumer_id", req.ConsumerId).
		Str("group", req.Group).
		Msg("Consumer subscribed via gRPC")

	return &pb.SubscribeResponse{
		Status: "subscribed",
		Topic:  req.Topic,
		Group:  req.Group,
	}, nil
}

// Consume handles consuming messages from a topic.
func (s *Server) Consume(ctx context.Context, req *pb.ConsumeRequest) (*pb.ConsumeResponse, error) {
	if req.Topic == "" || req.ConsumerId == "" || req.Group == "" {
		return &pb.ConsumeResponse{Messages: nil}, nil
	}

	maxMessages := int(req.MaxMessages)
	if maxMessages <= 0 {
		maxMessages = 10
	}

	messages, err := s.broker.Consume(req.Topic, req.ConsumerId, req.Group, maxMessages)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to consume")
		return &pb.ConsumeResponse{Messages: nil}, nil
	}

	// Convert to pb.Message
	pbMessages := make([]*pb.Message, len(messages))
	for i, msg := range messages {
		// Convert payload to bytes if it's json.RawMessage
		var payloadBytes []byte
		if msg.Payload != nil {
			payloadBytes, _ = json.Marshal(msg.Payload)
		}

		pbMessages[i] = &pb.Message{
			Id:        msg.ID,
			Topic:     msg.Topic,
			Payload:   payloadBytes,
			Timestamp: timestamppb.New(msg.Timestamp),
		}
	}

	if len(pbMessages) > 0 {
		s.logger.Debug().
			Str("topic", req.Topic).
			Str("consumer_id", req.ConsumerId).
			Int("count", len(pbMessages)).
			Msg("Messages consumed via gRPC")
	}

	return &pb.ConsumeResponse{Messages: pbMessages}, nil
}

// Ack handles acknowledging messages.
func (s *Server) Ack(ctx context.Context, req *pb.AckRequest) (*pb.AckResponse, error) {
	if req.Topic == "" || req.ConsumerId == "" || len(req.MessageIds) == 0 {
		return &pb.AckResponse{Status: "error", Acked: 0}, nil
	}

	acked, err := s.broker.Ack(req.Topic, req.ConsumerId, req.MessageIds)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to ack")
		return &pb.AckResponse{Status: "error", Acked: 0}, nil
	}

	s.logger.Debug().
		Str("topic", req.Topic).
		Str("consumer_id", req.ConsumerId).
		Int("acked", acked).
		Msg("Messages acknowledged via gRPC")

	return &pb.AckResponse{
		Status: "ok",
		Acked:  int32(acked),
	}, nil
}

// Health handles health check.
func (s *Server) Health(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}
