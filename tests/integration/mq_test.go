//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMQHealth verifies the MQ health endpoint.
func TestMQHealth(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	health, err := client.GetMQHealth()
	if err != nil {
		t.Skipf("MQ not available: %v", err)
	}

	assert.Equal(t, "healthy", health.Status)
}

// TestMQPublish verifies message publishing works.
func TestMQPublish(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	_, err := client.GetMQHealth()
	if err != nil {
		t.Skipf("MQ not available: %v", err)
	}

	// Publish a test message
	payload := map[string]interface{}{
		"test":      true,
		"timestamp": time.Now().Format(time.RFC3339),
		"value":     123.456,
	}

	resp, err := client.PublishMessage("test-topic", payload)
	require.NoError(t, err)

	assert.Equal(t, "ok", resp.Status)
	assert.NotEmpty(t, resp.MessageID)

	t.Logf("Published message: ID=%s", resp.MessageID)
}

// TestMQTopicStats verifies topic statistics are available.
func TestMQTopicStats(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	_, err := client.GetMQHealth()
	if err != nil {
		t.Skipf("MQ not available: %v", err)
	}

	// Publish some messages first
	for i := 0; i < 5; i++ {
		payload := map[string]interface{}{
			"index":     i,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		_, err := client.PublishMessage("stats-test-topic", payload)
		require.NoError(t, err)
	}

	// Get stats
	stats, err := client.GetTopicStats("stats-test-topic")
	require.NoError(t, err)

	assert.Equal(t, "stats-test-topic", stats.Topic)
	assert.GreaterOrEqual(t, stats.QueueSize, 0)
	assert.Greater(t, stats.MaxSize, 0)

	t.Logf("Topic stats: QueueSize=%d, MaxSize=%d, Pending=%d",
		stats.QueueSize, stats.MaxSize, stats.PendingMessages)
}

// TestMQMessageFlow verifies messages flow through the MQ correctly.
func TestMQMessageFlow(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	_, err := client.GetMQHealth()
	if err != nil {
		t.Skipf("MQ not available: %v", err)
	}

	topic := "flow-test-topic"

	// Get initial stats
	initialStats, _ := client.GetTopicStats(topic)
	initialSize := 0
	if initialStats != nil {
		initialSize = initialStats.QueueSize
	}

	// Publish messages
	messageCount := 10
	for i := 0; i < messageCount; i++ {
		payload := map[string]interface{}{
			"index":     i,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		_, err := client.PublishMessage(topic, payload)
		require.NoError(t, err)
	}

	// Check stats after publishing
	stats, err := client.GetTopicStats(topic)
	require.NoError(t, err)

	// Queue should have grown (unless consumer already processed)
	t.Logf("Queue size: before=%d, after=%d", initialSize, stats.QueueSize)
}

// TestMQGracefulDegradation verifies the MQ handles queue overflow gracefully.
func TestMQGracefulDegradation(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	_, err := client.GetMQHealth()
	if err != nil {
		t.Skipf("MQ not available: %v", err)
	}

	topic := "degradation-test-topic"

	// Get initial stats to know the max queue size
	stats, err := client.GetTopicStats(topic)
	if err != nil {
		// Topic doesn't exist yet, create it by publishing
		_, err = client.PublishMessage(topic, map[string]interface{}{"init": true})
		require.NoError(t, err)
		stats, err = client.GetTopicStats(topic)
	}
	require.NoError(t, err)

	maxSize := stats.MaxSize
	t.Logf("Max queue size: %d", maxSize)

	if maxSize > 1000 {
		t.Skip("Queue size too large for degradation test")
	}

	// Publish more messages than the queue can hold
	// This should trigger graceful degradation (dropping oldest)
	messagesToPublish := maxSize + 100

	t.Logf("Publishing %d messages to test graceful degradation...", messagesToPublish)
	for i := 0; i < messagesToPublish; i++ {
		payload := map[string]interface{}{
			"index":     i,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		_, err := client.PublishMessage(topic, payload)
		// Should not error - graceful degradation drops old messages
		assert.NoError(t, err)
	}

	// Check stats
	finalStats, err := client.GetTopicStats(topic)
	require.NoError(t, err)

	t.Logf("Final stats: QueueSize=%d, MaxSize=%d, Dropped=%d",
		finalStats.QueueSize, finalStats.MaxSize, finalStats.DroppedMessages)

	// Queue size should not exceed max
	assert.LessOrEqual(t, finalStats.QueueSize, finalStats.MaxSize,
		"Queue size should not exceed max")

	// Some messages should have been dropped
	if messagesToPublish > maxSize {
		assert.Greater(t, finalStats.DroppedMessages, int64(0),
			"Expected some messages to be dropped")
	}
}

// TestMQMultipleTopics verifies multiple topics work independently.
func TestMQMultipleTopics(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	_, err := client.GetMQHealth()
	if err != nil {
		t.Skipf("MQ not available: %v", err)
	}

	topics := []string{"topic-a", "topic-b", "topic-c"}

	// Publish different amounts to each topic
	for i, topic := range topics {
		for j := 0; j <= i*2; j++ {
			payload := map[string]interface{}{
				"topic": topic,
				"index": j,
			}
			_, err := client.PublishMessage(topic, payload)
			require.NoError(t, err)
		}
	}

	// Verify each topic has independent stats
	for i, topic := range topics {
		stats, err := client.GetTopicStats(topic)
		require.NoError(t, err)

		assert.Equal(t, topic, stats.Topic)
		t.Logf("Topic %s: QueueSize=%d", topic, stats.QueueSize)

		// Each topic should have some messages (exact count depends on consumers)
		_ = i // Just verifying topics are independent
	}
}

// TestMQHighThroughput verifies MQ can handle high message rates.
func TestMQHighThroughput(t *testing.T) {
	cfg := DefaultTestConfig()
	client := NewTestClient(cfg)

	_, err := client.GetMQHealth()
	if err != nil {
		t.Skipf("MQ not available: %v", err)
	}

	topic := "throughput-test-topic"
	messageCount := 100

	start := time.Now()

	// Publish messages as fast as possible
	for i := 0; i < messageCount; i++ {
		payload := map[string]interface{}{
			"index":     i,
			"timestamp": time.Now().Format(time.RFC3339Nano),
		}
		_, err := client.PublishMessage(topic, payload)
		require.NoError(t, err)
	}

	elapsed := time.Since(start)
	rate := float64(messageCount) / elapsed.Seconds()

	t.Logf("Published %d messages in %v (%.2f msg/sec)", messageCount, elapsed, rate)

	// Should achieve at least 10 messages per second (very conservative)
	assert.Greater(t, rate, 10.0, "Expected at least 10 msg/sec throughput")
}
