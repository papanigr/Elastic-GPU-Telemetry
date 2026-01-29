// Package models defines the data structures used by the message queue.
package models

import (
	"encoding/json"
	"time"
)

// Message represents a message in the queue.
// @Description Message in the queue
type Message struct {
	ID        string          `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Topic     string          `json:"topic" example:"gpu-telemetry"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp" example:"2026-01-28T15:30:00Z"`
	// Internal fields
	State       MessageState `json:"-"`
	DeliverTo   string       `json:"-"` // Consumer ID this message is assigned to
	DeliveredAt time.Time    `json:"-"`
	// DLQ tracking fields
	RetryCount    int       `json:"-"` // Number of delivery attempts
	DLQRetryCount int       `json:"-"` // Number of DLQ retry attempts
	LastError     string    `json:"-"` // Last error message
	FirstFailedAt time.Time `json:"-"` // When it first failed
	LastFailedAt  time.Time `json:"-"` // When it last failed
	OriginalTopic string    `json:"-"` // Original topic (for DLQ messages)
	IsDLQ         bool      `json:"-"` // Whether this message is in DLQ
}

// MessageState represents the state of a message.
type MessageState int

const (
	// Pending - message is in queue, not yet delivered
	Pending MessageState = iota
	// Delivered - message sent to consumer, awaiting ACK
	Delivered
	// Acked - message acknowledged by consumer
	Acked
	// Dead - message has exceeded max DLQ retries, requires admin action
	Dead
)

// String returns a string representation of the message state.
func (s MessageState) String() string {
	switch s {
	case Pending:
		return "pending"
	case Delivered:
		return "delivered"
	case Acked:
		return "acked"
	case Dead:
		return "dead"
	default:
		return "unknown"
	}
}

// PublishRequest is the request body for publishing a message.
// @Description Request to publish a message
type PublishRequest struct {
	ID        string          `json:"id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp,omitempty"`
}

// PublishResponse is the response for a publish request.
// @Description Response after publishing a message
type PublishResponse struct {
	Status    string `json:"status" example:"ok"`
	MessageID string `json:"message_id" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// ConsumeRequest is the request for consuming messages.
// @Description Request to consume messages
type ConsumeRequest struct {
	ConsumerID  string `json:"consumer_id" example:"collector-1"`
	Group       string `json:"group" example:"telemetry-collectors"`
	MaxMessages int    `json:"max_messages,omitempty" example:"10"`
}

// ConsumeResponse is the response for a consume request.
// @Description Response with consumed messages
type ConsumeResponse struct {
	Messages []*Message `json:"messages"`
}

// AckRequest is the request for acknowledging messages.
// @Description Request to acknowledge messages
type AckRequest struct {
	ConsumerID string   `json:"consumer_id" example:"collector-1"`
	MessageIDs []string `json:"message_ids"`
}

// AckResponse is the response for an ack request.
// @Description Response after acknowledging messages
type AckResponse struct {
	Status string `json:"status" example:"ok"`
	Acked  int    `json:"acked" example:"5"`
}

// SubscribeRequest is the request for subscribing to a topic.
// @Description Request to subscribe to a topic
type SubscribeRequest struct {
	ConsumerID string `json:"consumer_id" example:"collector-1"`
	Group      string `json:"group" example:"telemetry-collectors"`
}

// SubscribeResponse is the response for a subscribe request.
// @Description Response after subscribing
type SubscribeResponse struct {
	Status string `json:"status" example:"subscribed"`
	Topic  string `json:"topic" example:"gpu-telemetry"`
	Group  string `json:"group" example:"telemetry-collectors"`
}

// TopicStats contains statistics about a topic.
// @Description Topic statistics
type TopicStats struct {
	Topic            string `json:"topic" example:"gpu-telemetry"`
	QueueSize        int    `json:"queue_size" example:"150"`
	MaxSize          int    `json:"max_size" example:"10000"`
	PendingMessages  int    `json:"pending_messages" example:"100"`
	ConsumerGroups   int    `json:"consumer_groups" example:"1"`
	TotalConsumers   int    `json:"total_consumers" example:"3"`
	OldestMessageAge string `json:"oldest_message_age,omitempty" example:"5m30s"`
	// Graceful degradation metrics
	DroppedMessages  int64  `json:"dropped_messages" example:"0"`
	QueueUtilization string `json:"queue_utilization,omitempty" example:"1.5%"`
}

// ConsumerInfo contains information about a consumer.
// @Description Consumer information
type ConsumerInfo struct {
	ID          string    `json:"id" example:"collector-1"`
	Group       string    `json:"group" example:"telemetry-collectors"`
	Connected   bool      `json:"connected" example:"true"`
	LastSeen    time.Time `json:"last_seen" example:"2026-01-28T15:30:00Z"`
	PendingAcks int       `json:"pending_acks" example:"5"`
}

// ErrorResponse is the response for an error.
// @Description Error response
type ErrorResponse struct {
	Error   string `json:"error" example:"topic not found"`
	Code    int    `json:"code" example:"404"`
	Details string `json:"details,omitempty"`
}

// HealthResponse is the health check response.
// @Description Health check response
type HealthResponse struct {
	Status string `json:"status" example:"healthy"`
	Time   string `json:"time" example:"2026-01-28T15:30:00Z"`
}

// TopicListResponse is the response for listing topics.
// @Description List of topics
type TopicListResponse struct {
	Topics []string `json:"topics"`
}

// MessagesResponse is the response for getting messages.
// @Description Messages from a topic
type MessagesResponse struct {
	Topic    string     `json:"topic" example:"gpu-telemetry"`
	Count    int        `json:"count" example:"10"`
	Messages []*Message `json:"messages"`
}

// DeleteResponse is the response for deleting a message.
// @Description Delete response
type DeleteResponse struct {
	Status    string `json:"status" example:"deleted"`
	MessageID string `json:"message_id" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// PurgeResponse is the response for purging messages.
// @Description Purge response
type PurgeResponse struct {
	Status          string `json:"status" example:"purged"`
	MessagesDeleted int    `json:"messages_deleted" example:"100"`
}

// ConsumersResponse is the response for getting consumers.
// @Description Consumers for a topic
type ConsumersResponse struct {
	Topic     string          `json:"topic" example:"gpu-telemetry"`
	Consumers []*ConsumerInfo `json:"consumers"`
}

// DLQMessage represents a message in the Dead Letter Queue with failure info.
// @Description Dead Letter Queue message with failure details
type DLQMessage struct {
	ID            string          `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	OriginalTopic string          `json:"original_topic" example:"gpu-telemetry"`
	Payload       json.RawMessage `json:"payload"`
	Timestamp     time.Time       `json:"timestamp" example:"2026-01-28T15:30:00Z"`
	RetryCount    int             `json:"retry_count" example:"3"`
	DLQRetryCount int             `json:"dlq_retry_count" example:"2"`
	LastError     string          `json:"last_error,omitempty" example:"database connection refused"`
	FirstFailedAt time.Time       `json:"first_failed_at" example:"2026-01-28T15:30:00Z"`
	LastFailedAt  time.Time       `json:"last_failed_at" example:"2026-01-28T15:35:00Z"`
	State         string          `json:"state" example:"pending"`
}

// DLQStats contains statistics about a Dead Letter Queue.
// @Description DLQ statistics
type DLQStats struct {
	Topic           string `json:"topic" example:"gpu-telemetry-dlq"`
	OriginalTopic   string `json:"original_topic" example:"gpu-telemetry"`
	TotalMessages   int    `json:"total_messages" example:"10"`
	PendingMessages int    `json:"pending_messages" example:"5"`
	DeadMessages    int    `json:"dead_messages" example:"5"`
	OldestMessage   string `json:"oldest_message,omitempty" example:"5m30s"`
}

// DLQMessagesResponse is the response for getting DLQ messages.
// @Description DLQ messages response
type DLQMessagesResponse struct {
	Topic    string        `json:"topic" example:"gpu-telemetry-dlq"`
	Count    int           `json:"count" example:"10"`
	Messages []*DLQMessage `json:"messages"`
}

// ReplayRequest is the request for replaying DLQ messages.
// @Description Request to replay DLQ messages
type ReplayRequest struct {
	MessageIDs []string `json:"message_ids,omitempty"` // If empty, replay all pending DLQ messages
	Force      bool     `json:"force,omitempty"`       // If true, also replay dead messages
}

// ReplayResponse is the response for replaying DLQ messages.
// @Description Response after replaying DLQ messages
type ReplayResponse struct {
	Status   string `json:"status" example:"replayed"`
	Replayed int    `json:"replayed" example:"5"`
	Failed   int    `json:"failed" example:"0"`
}
