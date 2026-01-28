// Package models defines the data structures used by the message queue.
package models

import (
	"encoding/json"
	"time"
)

// Message represents a message in the queue.
type Message struct {
	ID        string          `json:"id"`
	Topic     string          `json:"topic"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	// Internal fields
	State     MessageState `json:"-"`
	DeliverTo string       `json:"-"` // Consumer ID this message is assigned to
	DeliveredAt time.Time  `json:"-"`
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
)

// PublishRequest is the request body for publishing a message.
type PublishRequest struct {
	ID        string          `json:"id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp,omitempty"`
}

// PublishResponse is the response for a publish request.
type PublishResponse struct {
	Status    string `json:"status"`
	MessageID string `json:"message_id"`
}

// ConsumeRequest is the request for consuming messages.
type ConsumeRequest struct {
	ConsumerID string `json:"consumer_id"`
	Group      string `json:"group"`
	MaxMessages int   `json:"max_messages,omitempty"`
}

// ConsumeResponse is the response for a consume request.
type ConsumeResponse struct {
	Messages []*Message `json:"messages"`
}

// AckRequest is the request for acknowledging messages.
type AckRequest struct {
	ConsumerID string   `json:"consumer_id"`
	MessageIDs []string `json:"message_ids"`
}

// AckResponse is the response for an ack request.
type AckResponse struct {
	Status string `json:"status"`
	Acked  int    `json:"acked"`
}

// SubscribeRequest is the request for subscribing to a topic.
type SubscribeRequest struct {
	ConsumerID string `json:"consumer_id"`
	Group      string `json:"group"`
}

// SubscribeResponse is the response for a subscribe request.
type SubscribeResponse struct {
	Status string `json:"status"`
	Topic  string `json:"topic"`
	Group  string `json:"group"`
}

// TopicStats contains statistics about a topic.
type TopicStats struct {
	Topic           string        `json:"topic"`
	QueueSize       int           `json:"queue_size"`
	MaxSize         int           `json:"max_size"`
	PendingMessages int           `json:"pending_messages"`
	ConsumerGroups  int           `json:"consumer_groups"`
	TotalConsumers  int           `json:"total_consumers"`
	OldestMessageAge string       `json:"oldest_message_age,omitempty"`
}

// ConsumerInfo contains information about a consumer.
type ConsumerInfo struct {
	ID          string    `json:"id"`
	Group       string    `json:"group"`
	Connected   bool      `json:"connected"`
	LastSeen    time.Time `json:"last_seen"`
	PendingAcks int       `json:"pending_acks"`
}

// ErrorResponse is the response for an error.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}
