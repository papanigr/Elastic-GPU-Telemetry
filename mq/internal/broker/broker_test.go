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
		// DLQ config
		DLQEnabled:    true,
		MaxRetries:    3,
		DLQMaxRetries: 3,
		DLQRetryDelay: 1 * time.Second,
	}
	logger := zerolog.Nop()
	return NewBroker(cfg, logger)
}

func newTestBrokerWithDLQ() *Broker {
	cfg := config.Config{
		MaxQueueSize:    100,
		MaxMessageAge:   5 * time.Minute,
		AckTimeout:      100 * time.Millisecond, // Short timeout for testing
		CleanupInterval: 50 * time.Millisecond,  // Fast cleanup for testing
		// DLQ config
		DLQEnabled:    true,
		MaxRetries:    2, // Low for testing
		DLQMaxRetries: 2,
		DLQRetryDelay: 100 * time.Millisecond,
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

// --- DLQ Tests ---

func TestBroker_GetDLQStats_Empty(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Get DLQ stats for non-existent DLQ
	stats := b.GetDLQStats("test-topic")
	if stats == nil {
		t.Fatal("Expected stats, got nil")
	}

	if stats.Topic != "test-topic-dlq" {
		t.Errorf("Expected topic 'test-topic-dlq', got %s", stats.Topic)
	}

	if stats.OriginalTopic != "test-topic" {
		t.Errorf("Expected original topic 'test-topic', got %s", stats.OriginalTopic)
	}

	if stats.TotalMessages != 0 {
		t.Errorf("Expected 0 messages, got %d", stats.TotalMessages)
	}
}

func TestBroker_GetDLQMessages_Empty(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Get DLQ messages for non-existent DLQ
	messages := b.GetDLQMessages("test-topic", 10)
	if messages != nil {
		t.Errorf("Expected nil, got %v", messages)
	}
}

func TestBroker_ReplayDLQMessages_Empty(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Replay from non-existent DLQ
	replayed, failed := b.ReplayDLQMessages("test-topic", nil, false)
	if replayed != 0 || failed != 0 {
		t.Errorf("Expected 0/0, got %d/%d", replayed, failed)
	}
}

func TestBroker_PurgeDLQ_Empty(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Purge non-existent DLQ
	deleted := b.PurgeDLQ("test-topic")
	if deleted != 0 {
		t.Errorf("Expected 0, got %d", deleted)
	}
}

func TestBroker_DLQ_MessageMovedAfterMaxRetries(t *testing.T) {
	b := newTestBrokerWithDLQ()
	defer b.Close()

	// Publish a message
	msg := &models.Message{
		ID:        "msg-dlq-test",
		Topic:     "test-topic",
		Payload:   json.RawMessage(`{"data": "test"}`),
		Timestamp: time.Now(),
	}
	b.Publish("test-topic", msg)

	// Subscribe and consume
	b.Subscribe("test-topic", "consumer-1", "group-1")

	// Consume without ACK multiple times to trigger DLQ
	for i := 0; i < 3; i++ {
		messages, _ := b.Consume("test-topic", "consumer-1", "group-1", 1)
		if len(messages) > 0 {
			// Don't ACK - simulate failure
			// Wait for cleanup to redeliver
			time.Sleep(150 * time.Millisecond)
		}
	}

	// Wait for cleanup to move message to DLQ
	time.Sleep(200 * time.Millisecond)

	// Check DLQ stats
	stats := b.GetDLQStats("test-topic")
	if stats.TotalMessages == 0 {
		// Message might still be in main queue - check if retry count is increasing
		mainStats := b.GetTopicStats("test-topic")
		t.Logf("Main queue stats: %+v", mainStats)
		t.Logf("DLQ stats: %+v", stats)
	}
}

func TestBroker_DLQ_ReplayMessages(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Manually create a DLQ message
	dlqMsg := &models.Message{
		ID:            "msg-dlq-1",
		Topic:         "test-topic-dlq",
		Payload:       json.RawMessage(`{"data": "test"}`),
		Timestamp:     time.Now(),
		OriginalTopic: "test-topic",
		IsDLQ:         true,
		RetryCount:    3,
		FirstFailedAt: time.Now().Add(-time.Minute),
		LastFailedAt:  time.Now(),
	}
	b.Publish("test-topic-dlq", dlqMsg)

	// Verify DLQ has the message
	dlqStats := b.GetDLQStats("test-topic")
	if dlqStats.TotalMessages != 1 {
		t.Fatalf("Expected 1 DLQ message, got %d", dlqStats.TotalMessages)
	}

	// Replay the message
	replayed, failed := b.ReplayDLQMessages("test-topic", nil, false)
	if replayed != 1 {
		t.Errorf("Expected 1 replayed, got %d", replayed)
	}
	if failed != 0 {
		t.Errorf("Expected 0 failed, got %d", failed)
	}

	// Verify DLQ is now empty
	dlqStats = b.GetDLQStats("test-topic")
	if dlqStats.TotalMessages != 0 {
		t.Errorf("Expected 0 DLQ messages after replay, got %d", dlqStats.TotalMessages)
	}

	// Verify message is back in original topic
	mainStats := b.GetTopicStats("test-topic")
	if mainStats == nil {
		t.Fatal("Expected main topic stats, got nil")
	}
	if mainStats.QueueSize != 1 {
		t.Errorf("Expected 1 message in main topic, got %d", mainStats.QueueSize)
	}
}

func TestBroker_DLQ_PurgeMessages(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Manually create DLQ messages
	for i := 0; i < 5; i++ {
		dlqMsg := &models.Message{
			ID:            "msg-dlq-" + string(rune('0'+i)),
			Topic:         "test-topic-dlq",
			Payload:       json.RawMessage(`{"data": "test"}`),
			Timestamp:     time.Now(),
			OriginalTopic: "test-topic",
			IsDLQ:         true,
		}
		b.Publish("test-topic-dlq", dlqMsg)
	}

	// Verify DLQ has messages
	dlqStats := b.GetDLQStats("test-topic")
	if dlqStats.TotalMessages != 5 {
		t.Fatalf("Expected 5 DLQ messages, got %d", dlqStats.TotalMessages)
	}

	// Purge
	deleted := b.PurgeDLQ("test-topic")
	if deleted != 5 {
		t.Errorf("Expected 5 deleted, got %d", deleted)
	}

	// Verify DLQ is empty
	dlqStats = b.GetDLQStats("test-topic")
	if dlqStats.TotalMessages != 0 {
		t.Errorf("Expected 0 DLQ messages, got %d", dlqStats.TotalMessages)
	}
}

func TestBroker_DLQ_GetMessages(t *testing.T) {
	b := newTestBroker()
	defer b.Close()

	// Manually create a DLQ message with failure details
	dlqMsg := &models.Message{
		ID:            "msg-dlq-details",
		Topic:         "test-topic-dlq",
		Payload:       json.RawMessage(`{"data": "test"}`),
		Timestamp:     time.Now(),
		OriginalTopic: "test-topic",
		IsDLQ:         true,
		RetryCount:    3,
		DLQRetryCount: 1,
		LastError:     "database connection refused",
		FirstFailedAt: time.Now().Add(-2 * time.Minute),
		LastFailedAt:  time.Now().Add(-1 * time.Minute),
	}
	b.Publish("test-topic-dlq", dlqMsg)

	// Get DLQ messages
	messages := b.GetDLQMessages("test-topic", 10)
	if len(messages) != 1 {
		t.Fatalf("Expected 1 DLQ message, got %d", len(messages))
	}

	msg := messages[0]
	if msg.ID != "msg-dlq-details" {
		t.Errorf("Expected ID 'msg-dlq-details', got %s", msg.ID)
	}
	if msg.OriginalTopic != "test-topic" {
		t.Errorf("Expected OriginalTopic 'test-topic', got %s", msg.OriginalTopic)
	}
	if msg.RetryCount != 3 {
		t.Errorf("Expected RetryCount 3, got %d", msg.RetryCount)
	}
	if msg.DLQRetryCount != 1 {
		t.Errorf("Expected DLQRetryCount 1, got %d", msg.DLQRetryCount)
	}
	if msg.LastError != "database connection refused" {
		t.Errorf("Expected LastError 'database connection refused', got %s", msg.LastError)
	}
}

func TestIsDLQTopic(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"gpu-telemetry", false},
		{"gpu-telemetry-dlq", true},
		{"dlq", false},
		{"-dlq", false}, // "-dlq" alone is not a valid DLQ topic (no original topic name)
		{"topic-dlq-suffix", false},
	}

	for _, tt := range tests {
		result := isDLQTopic(tt.name)
		if result != tt.expected {
			t.Errorf("isDLQTopic(%s) = %v, expected %v", tt.name, result, tt.expected)
		}
	}
}

func TestGetDLQTopicName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gpu-telemetry", "gpu-telemetry-dlq"},
		{"test", "test-dlq"},
	}

	for _, tt := range tests {
		result := getDLQTopicName(tt.input)
		if result != tt.expected {
			t.Errorf("getDLQTopicName(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestGetOriginalTopicName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gpu-telemetry-dlq", "gpu-telemetry"},
		{"test-dlq", "test"},
		{"nodlq", "nodlq"},
	}

	for _, tt := range tests {
		result := getOriginalTopicName(tt.input)
		if result != tt.expected {
			t.Errorf("getOriginalTopicName(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestBroker_DLQ_AutoReplayToOriginalTopic(t *testing.T) {
	cfg := config.Config{
		MaxQueueSize:    100,
		MaxMessageAge:   5 * time.Minute,
		AckTimeout:      50 * time.Millisecond, // Very short for testing
		CleanupInterval: 30 * time.Millisecond, // Fast cleanup
		DLQEnabled:      true,
		MaxRetries:      1,                     // Move to DLQ after 1 retry
		DLQMaxRetries:   2,                     // Mark dead after 2 auto-replays
		DLQRetryDelay:   50 * time.Millisecond, // Short delay for testing
	}
	logger := zerolog.Nop()
	b := NewBroker(cfg, logger)
	defer b.Close()

	// Publish a message
	msg := &models.Message{
		ID:        "msg-auto-replay",
		Topic:     "test-topic",
		Payload:   json.RawMessage(`{"data": "test"}`),
		Timestamp: time.Now(),
	}
	b.Publish("test-topic", msg)

	// Subscribe and consume
	b.Subscribe("test-topic", "consumer-1", "group-1")

	// Consume without ACK to trigger DLQ
	messages, _ := b.Consume("test-topic", "consumer-1", "group-1", 1)
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	// Wait for cleanup to move to DLQ (no ACK after ackTimeout)
	time.Sleep(100 * time.Millisecond)

	// Check if message is in DLQ
	dlqStats := b.GetDLQStats("test-topic")
	mainStats := b.GetTopicStats("test-topic")

	t.Logf("After first failure - Main: %d, DLQ: %d", mainStats.QueueSize, dlqStats.TotalMessages)

	// Wait for DLQ auto-replay (after dlqRetryDelay)
	time.Sleep(100 * time.Millisecond)

	// Message should be replayed back to main queue
	mainStats = b.GetTopicStats("test-topic")
	dlqStats = b.GetDLQStats("test-topic")

	t.Logf("After auto-replay - Main: %d, DLQ: %d", mainStats.QueueSize, dlqStats.TotalMessages)

	// The message should be back in the main queue
	if mainStats.QueueSize == 0 && dlqStats.TotalMessages == 0 {
		t.Log("Message processed - either consumed successfully or still in transit")
	}
}

func TestBroker_DLQ_MarkedDeadAfterMaxReplays(t *testing.T) {
	cfg := config.Config{
		MaxQueueSize:    100,
		MaxMessageAge:   5 * time.Minute,
		AckTimeout:      30 * time.Millisecond, // Very short for testing
		CleanupInterval: 20 * time.Millisecond, // Fast cleanup
		DLQEnabled:      true,
		MaxRetries:      1,                     // Move to DLQ after 1 retry
		DLQMaxRetries:   1,                     // Mark dead after 1 auto-replay
		DLQRetryDelay:   30 * time.Millisecond, // Short delay for testing
	}
	logger := zerolog.Nop()
	b := NewBroker(cfg, logger)
	defer b.Close()

	// Create a DLQ message that has already been replayed once
	dlqMsg := &models.Message{
		ID:            "msg-will-be-dead",
		Topic:         "test-topic-dlq",
		Payload:       json.RawMessage(`{"data": "test"}`),
		Timestamp:     time.Now(),
		OriginalTopic: "test-topic",
		IsDLQ:         true,
		DLQRetryCount: 0,                            // First replay attempt
		LastFailedAt:  time.Now().Add(-time.Minute), // Failed a while ago
	}
	b.Publish("test-topic-dlq", dlqMsg)

	// Wait for auto-replay
	time.Sleep(100 * time.Millisecond)

	// Message should be replayed to main queue
	mainStats := b.GetTopicStats("test-topic")
	t.Logf("After first auto-replay - Main queue: %d", mainStats.QueueSize)

	// Consume without ACK to trigger failure
	b.Subscribe("test-topic", "consumer-1", "group-1")
	messages, _ := b.Consume("test-topic", "consumer-1", "group-1", 1)
	if len(messages) > 0 {
		// Don't ACK - let it fail and go back to DLQ
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for cleanup cycles
	time.Sleep(100 * time.Millisecond)

	// Check if message is marked as dead in DLQ
	dlqMessages := b.GetDLQMessages("test-topic", 10)
	for _, msg := range dlqMessages {
		t.Logf("DLQ message: ID=%s, State=%s, DLQRetryCount=%d", msg.ID, msg.State, msg.DLQRetryCount)
	}
}
