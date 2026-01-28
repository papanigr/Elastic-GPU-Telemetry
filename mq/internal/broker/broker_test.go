package broker

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gpu-telemetry-pipeline/mq/internal/config"
	"github.com/gpu-telemetry-pipeline/mq/pkg/models"
	"github.com/rs/zerolog"
)

func newTestBroker() *Broker {
	cfg := config.Config{
		MaxQueueSize:    100,
		MaxMessageAge:   5 * time.Minute,
		AckTimeout:      30 * time.Second,
		CleanupInterval: 1 * time.Second,
	}
	logger := zerolog.Nop()
	return NewBroker(cfg, logger)
}

func TestBroker_Publish(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	msg := &models.Message{
		ID:        "msg-1",
		Topic:     "test-topic",
		Payload:   json.RawMessage(`{"data": "test"}`),
		Timestamp: time.Now(),
	}

	err := b.Publish("test-topic", msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Verify topic was created
	topics := b.ListTopics()
	if len(topics) != 1 || topics[0] != "test-topic" {
		t.Errorf("Expected topic 'test-topic', got %v", topics)
	}
}

func TestBroker_Subscribe(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Publish first to create topic
	msg := &models.Message{
		ID:        "msg-1",
		Topic:     "test-topic",
		Payload:   json.RawMessage(`{"data": "test"}`),
		Timestamp: time.Now(),
	}
	b.Publish("test-topic", msg)

	// Subscribe
	err := b.Subscribe("test-topic", "consumer-1", "group-1")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Subscribe to non-existent topic creates it (GetOrCreateTopic behavior)
	err = b.Subscribe("new-topic", "consumer-1", "group-1")
	if err != nil {
		t.Errorf("Subscribe to new topic failed: %v", err)
	}

	// Verify topic was created
	topics := b.ListTopics()
	found := false
	for _, topic := range topics {
		if topic == "new-topic" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected new-topic to be created")
	}
}

func TestBroker_Consume(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Publish messages
	for i := 0; i < 5; i++ {
		msg := &models.Message{
			ID:        "msg-" + string(rune('0'+i)),
			Topic:     "test-topic",
			Payload:   json.RawMessage(`{"data": "test"}`),
			Timestamp: time.Now(),
		}
		b.Publish("test-topic", msg)
	}

	// Subscribe
	b.Subscribe("test-topic", "consumer-1", "group-1")

	// Consume
	messages, err := b.Consume("test-topic", "consumer-1", "group-1", 3)
	if err != nil {
		t.Fatalf("Consume failed: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}
}

func TestBroker_Ack(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Publish
	msg := &models.Message{
		ID:        "msg-1",
		Topic:     "test-topic",
		Payload:   json.RawMessage(`{"data": "test"}`),
		Timestamp: time.Now(),
	}
	b.Publish("test-topic", msg)

	// Subscribe and consume
	b.Subscribe("test-topic", "consumer-1", "group-1")
	messages, _ := b.Consume("test-topic", "consumer-1", "group-1", 1)

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	// Ack
	acked, err := b.Ack("test-topic", "consumer-1", []string{messages[0].ID})
	if err != nil {
		t.Fatalf("Ack failed: %v", err)
	}

	if acked != 1 {
		t.Errorf("Expected 1 acked, got %d", acked)
	}
}

func TestBroker_GetTopicStats(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Non-existent topic
	stats := b.GetTopicStats("non-existent")
	if stats != nil {
		t.Error("Expected nil stats for non-existent topic")
	}

	// Create topic with messages
	for i := 0; i < 5; i++ {
		msg := &models.Message{
			ID:        "msg-" + string(rune('0'+i)),
			Topic:     "test-topic",
			Payload:   json.RawMessage(`{"data": "test"}`),
			Timestamp: time.Now(),
		}
		b.Publish("test-topic", msg)
	}

	stats = b.GetTopicStats("test-topic")
	if stats == nil {
		t.Fatal("Expected stats, got nil")
	}

	if stats.QueueSize != 5 {
		t.Errorf("Expected queue size 5, got %d", stats.QueueSize)
	}
}

func TestBroker_GetMessages(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Publish messages
	for i := 0; i < 10; i++ {
		msg := &models.Message{
			ID:        "msg-" + string(rune('0'+i)),
			Topic:     "test-topic",
			Payload:   json.RawMessage(`{"data": "test"}`),
			Timestamp: time.Now(),
		}
		b.Publish("test-topic", msg)
	}

	// Get with limit
	messages := b.GetMessages("test-topic", 5)
	if len(messages) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(messages))
	}

	// Non-existent topic
	messages = b.GetMessages("non-existent", 10)
	if messages != nil {
		t.Error("Expected nil for non-existent topic")
	}
}

func TestBroker_DeleteMessage(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	msg := &models.Message{
		ID:        "msg-to-delete",
		Topic:     "test-topic",
		Payload:   json.RawMessage(`{"data": "test"}`),
		Timestamp: time.Now(),
	}
	b.Publish("test-topic", msg)

	// Delete
	deleted := b.DeleteMessage("test-topic", "msg-to-delete")
	if !deleted {
		t.Error("Expected message to be deleted")
	}

	// Delete again should return false
	deleted = b.DeleteMessage("test-topic", "msg-to-delete")
	if deleted {
		t.Error("Expected false for already deleted message")
	}

	// Delete from non-existent topic
	deleted = b.DeleteMessage("non-existent", "msg-1")
	if deleted {
		t.Error("Expected false for non-existent topic")
	}
}

func TestBroker_PurgeMessages(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Publish messages
	for i := 0; i < 5; i++ {
		msg := &models.Message{
			ID:        "msg-" + string(rune('0'+i)),
			Topic:     "test-topic",
			Payload:   json.RawMessage(`{"data": "test"}`),
			Timestamp: time.Now(),
		}
		b.Publish("test-topic", msg)
	}

	// Purge
	deleted := b.PurgeMessages("test-topic")
	if deleted != 5 {
		t.Errorf("Expected 5 deleted, got %d", deleted)
	}

	// Verify empty
	stats := b.GetTopicStats("test-topic")
	if stats.QueueSize != 0 {
		t.Errorf("Expected queue size 0, got %d", stats.QueueSize)
	}
}

func TestBroker_GetConsumers(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Create topic
	msg := &models.Message{
		ID:        "msg-1",
		Topic:     "test-topic",
		Payload:   json.RawMessage(`{"data": "test"}`),
		Timestamp: time.Now(),
	}
	b.Publish("test-topic", msg)

	// No consumers yet
	consumers := b.GetConsumers("test-topic")
	if len(consumers) != 0 {
		t.Errorf("Expected 0 consumers, got %d", len(consumers))
	}

	// Add consumer
	b.Subscribe("test-topic", "consumer-1", "group-1")
	consumers = b.GetConsumers("test-topic")
	if len(consumers) != 1 {
		t.Errorf("Expected 1 consumer, got %d", len(consumers))
	}

	// Non-existent topic
	consumers = b.GetConsumers("non-existent")
	if consumers != nil {
		t.Error("Expected nil for non-existent topic")
	}
}

func TestBroker_ListTopics(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// No topics initially
	topics := b.ListTopics()
	if len(topics) != 0 {
		t.Errorf("Expected 0 topics, got %d", len(topics))
	}

	// Create topics
	for _, topicName := range []string{"topic-a", "topic-b", "topic-c"} {
		msg := &models.Message{
			ID:        "msg-1",
			Topic:     topicName,
			Payload:   json.RawMessage(`{}`),
			Timestamp: time.Now(),
		}
		b.Publish(topicName, msg)
	}

	topics = b.ListTopics()
	if len(topics) != 3 {
		t.Errorf("Expected 3 topics, got %d", len(topics))
	}
}
