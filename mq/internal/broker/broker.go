// Package broker implements the core message queue logic.
package broker

import (
	"context"
	"sync"
	"time"

	"github.com/gpu-telemetry-pipeline/mq/internal/config"
	"github.com/gpu-telemetry-pipeline/mq/pkg/models"
	"github.com/rs/zerolog"
)

// Broker is the main message queue broker.
type Broker struct {
	config config.Config
	topics map[string]*Topic
	mu     sync.RWMutex
	logger zerolog.Logger
	ctx    context.Context
	cancel context.CancelFunc
}

// NewBroker creates a new broker instance.
func NewBroker(cfg config.Config, logger zerolog.Logger) *Broker {
	ctx, cancel := context.WithCancel(context.Background())
	b := &Broker{
		config: cfg,
		topics: make(map[string]*Topic),
		logger: logger.With().Str("component", "broker").Logger(),
		ctx:    ctx,
		cancel: cancel,
	}

	// Start cleanup goroutine
	go b.cleanupLoop()

	return b
}

// GetOrCreateTopic gets an existing topic or creates a new one.
func (b *Broker) GetOrCreateTopic(name string) *Topic {
	b.mu.RLock()
	topic, exists := b.topics[name]
	b.mu.RUnlock()

	if exists {
		return topic
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Double-check after acquiring write lock
	if topic, exists = b.topics[name]; exists {
		return topic
	}

	topic = NewTopic(name, b.config, b.logger)
	b.topics[name] = topic
	b.logger.Info().Str("topic", name).Msg("Created new topic")

	return topic
}

// GetTopic gets an existing topic or returns nil.
func (b *Broker) GetTopic(name string) *Topic {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.topics[name]
}

// ListTopics returns a list of all topic names.
func (b *Broker) ListTopics() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	names := make([]string, 0, len(b.topics))
	for name := range b.topics {
		names = append(names, name)
	}
	return names
}

// Publish publishes a message to a topic.
func (b *Broker) Publish(topicName string, msg *models.Message) error {
	topic := b.GetOrCreateTopic(topicName)
	return topic.Publish(msg)
}

// Subscribe adds a consumer to a topic's consumer group.
func (b *Broker) Subscribe(topicName, consumerID, groupName string) error {
	topic := b.GetOrCreateTopic(topicName)
	return topic.Subscribe(consumerID, groupName)
}

// Consume retrieves messages for a consumer.
func (b *Broker) Consume(topicName, consumerID, groupName string, maxMessages int) ([]*models.Message, error) {
	topic := b.GetTopic(topicName)
	if topic == nil {
		return nil, nil
	}
	return topic.Consume(consumerID, groupName, maxMessages)
}

// Ack acknowledges messages.
func (b *Broker) Ack(topicName, consumerID string, messageIDs []string) (int, error) {
	topic := b.GetTopic(topicName)
	if topic == nil {
		return 0, nil
	}
	return topic.Ack(consumerID, messageIDs)
}

// GetTopicStats returns statistics for a topic.
func (b *Broker) GetTopicStats(topicName string) *models.TopicStats {
	topic := b.GetTopic(topicName)
	if topic == nil {
		return nil
	}
	return topic.GetStats()
}

// GetMessages returns messages from a topic (for admin).
func (b *Broker) GetMessages(topicName string, limit int) []*models.Message {
	topic := b.GetTopic(topicName)
	if topic == nil {
		return nil
	}
	return topic.GetMessages(limit)
}

// DeleteMessage deletes a specific message from a topic.
func (b *Broker) DeleteMessage(topicName, messageID string) bool {
	topic := b.GetTopic(topicName)
	if topic == nil {
		return false
	}
	return topic.DeleteMessage(messageID)
}

// PurgeMessages deletes all messages from a topic.
func (b *Broker) PurgeMessages(topicName string) int {
	topic := b.GetTopic(topicName)
	if topic == nil {
		return 0
	}
	return topic.PurgeMessages()
}

// GetConsumers returns consumers for a topic.
func (b *Broker) GetConsumers(topicName string) []*models.ConsumerInfo {
	topic := b.GetTopic(topicName)
	if topic == nil {
		return nil
	}
	return topic.GetConsumers()
}

// cleanupLoop periodically cleans up old messages and stale consumers.
func (b *Broker) cleanupLoop() {
	ticker := time.NewTicker(b.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.cleanup()
		}
	}
}

// cleanup removes old messages and redelivers unacked messages.
func (b *Broker) cleanup() {
	b.mu.RLock()
	topics := make([]*Topic, 0, len(b.topics))
	for name, t := range b.topics {
		// Skip DLQ topics during normal cleanup iteration
		// They'll be handled separately
		if !isDLQTopic(name) {
			topics = append(topics, t)
		}
	}
	b.mu.RUnlock()

	for _, topic := range topics {
		result := topic.Cleanup(
			b.config.MaxMessageAge,
			b.config.AckTimeout,
			b.config.MaxRetries,
			b.config.DLQMaxRetries,
			b.config.DLQRetryDelay,
			b.config.DLQEnabled,
		)

		// Move messages to DLQ
		if len(result.MoveToDLQ) > 0 {
			dlqTopicName := getDLQTopicName(topic.name)
			dlqTopic := b.GetOrCreateTopic(dlqTopicName)
			for _, msg := range result.MoveToDLQ {
				if err := dlqTopic.Publish(msg); err != nil {
					b.logger.Error().
						Err(err).
						Str("message_id", msg.ID).
						Msg("Failed to move message to DLQ")
				}
			}
		}
	}

	// Also cleanup DLQ topics and handle auto-replay
	b.mu.RLock()
	dlqTopics := make([]*Topic, 0)
	for name, t := range b.topics {
		if isDLQTopic(name) {
			dlqTopics = append(dlqTopics, t)
		}
	}
	b.mu.RUnlock()

	for _, dlqTopic := range dlqTopics {
		dlqResult := dlqTopic.Cleanup(
			0, // No max age for DLQ messages
			b.config.AckTimeout,
			b.config.MaxRetries,
			b.config.DLQMaxRetries,
			b.config.DLQRetryDelay,
			b.config.DLQEnabled,
		)

		// Auto-replay messages back to original topic
		if len(dlqResult.ReplayToOriginal) > 0 {
			for _, msg := range dlqResult.ReplayToOriginal {
				originalTopicName := msg.OriginalTopic
				if originalTopicName == "" {
					originalTopicName = getOriginalTopicName(dlqTopic.name)
				}

				originalTopic := b.GetOrCreateTopic(originalTopicName)

				// Create a new message for the original topic
				// Keep DLQRetryCount to track how many times we've auto-replayed
				replayMsg := &models.Message{
					ID:            msg.ID,
					Topic:         originalTopicName,
					Payload:       msg.Payload,
					Timestamp:     msg.Timestamp,
					State:         models.Pending,
					RetryCount:    0, // Reset main retry count for fresh attempt
					DLQRetryCount: msg.DLQRetryCount,
					FirstFailedAt: msg.FirstFailedAt,
					LastFailedAt:  msg.LastFailedAt,
					OriginalTopic: originalTopicName,
					IsDLQ:         false, // No longer in DLQ
				}

				if err := originalTopic.Publish(replayMsg); err != nil {
					b.logger.Error().
						Err(err).
						Str("message_id", msg.ID).
						Str("original_topic", originalTopicName).
						Msg("Failed to auto-replay DLQ message to original topic")
				} else {
					b.logger.Info().
						Str("message_id", msg.ID).
						Str("original_topic", originalTopicName).
						Int("dlq_retry_count", msg.DLQRetryCount).
						Msg("Auto-replayed DLQ message to original topic")
				}
			}
		}
	}
}

// getDLQTopicName returns the DLQ topic name for a given topic.
func getDLQTopicName(topicName string) string {
	return topicName + "-dlq"
}

// getOriginalTopicName returns the original topic name from a DLQ topic name.
func getOriginalTopicName(dlqTopicName string) string {
	const suffix = "-dlq"
	if len(dlqTopicName) > len(suffix) && dlqTopicName[len(dlqTopicName)-len(suffix):] == suffix {
		return dlqTopicName[:len(dlqTopicName)-len(suffix)]
	}
	return dlqTopicName
}

// isDLQTopic returns true if the topic name is a DLQ topic.
func isDLQTopic(topicName string) bool {
	const suffix = "-dlq"
	return len(topicName) > len(suffix) && topicName[len(topicName)-len(suffix):] == suffix
}

// GetDLQStats returns DLQ statistics for a topic.
func (b *Broker) GetDLQStats(topicName string) *models.DLQStats {
	dlqTopicName := getDLQTopicName(topicName)
	topic := b.GetTopic(dlqTopicName)
	if topic == nil {
		return &models.DLQStats{
			Topic:         dlqTopicName,
			OriginalTopic: topicName,
		}
	}
	return topic.GetDLQStats(topicName)
}

// GetDLQMessages returns DLQ messages for a topic.
func (b *Broker) GetDLQMessages(topicName string, limit int) []*models.DLQMessage {
	dlqTopicName := getDLQTopicName(topicName)
	topic := b.GetTopic(dlqTopicName)
	if topic == nil {
		return nil
	}
	return topic.GetDLQMessages(limit)
}

// ReplayDLQMessages replays messages from DLQ back to the original topic.
func (b *Broker) ReplayDLQMessages(topicName string, messageIDs []string, force bool) (replayed, failed int) {
	dlqTopicName := getDLQTopicName(topicName)
	dlqTopic := b.GetTopic(dlqTopicName)
	if dlqTopic == nil {
		return 0, 0
	}

	// Get messages to replay
	messages := dlqTopic.GetMessagesForReplay(messageIDs, force)
	if len(messages) == 0 {
		return 0, 0
	}

	// Get or create original topic
	originalTopic := b.GetOrCreateTopic(topicName)

	// Track which messages were successfully replayed
	replayedIDs := make([]string, 0, len(messages))

	for _, msg := range messages {
		// Reset message state for replay
		replayMsg := &models.Message{
			ID:        msg.ID,
			Topic:     topicName,
			Payload:   msg.Payload,
			Timestamp: msg.Timestamp,
			State:     models.Pending,
			// Reset retry counts for fresh attempt
			RetryCount:    0,
			DLQRetryCount: 0,
			IsDLQ:         false,
			OriginalTopic: "",
		}

		if err := originalTopic.Publish(replayMsg); err != nil {
			b.logger.Error().
				Err(err).
				Str("message_id", msg.ID).
				Msg("Failed to replay message")
			failed++
		} else {
			replayedIDs = append(replayedIDs, msg.ID)
			replayed++
		}
	}

	// Remove replayed messages from DLQ
	if len(replayedIDs) > 0 {
		dlqTopic.RemoveMessages(replayedIDs)
		b.logger.Info().
			Str("topic", topicName).
			Int("replayed", replayed).
			Int("failed", failed).
			Msg("DLQ messages replayed")
	}

	return replayed, failed
}

// PurgeDLQ removes all messages from a topic's DLQ.
func (b *Broker) PurgeDLQ(topicName string) int {
	dlqTopicName := getDLQTopicName(topicName)
	topic := b.GetTopic(dlqTopicName)
	if topic == nil {
		return 0
	}
	return topic.PurgeMessages()
}

// Close shuts down the broker.
func (b *Broker) Close() error {
	b.cancel()
	b.logger.Info().Msg("Broker closed")
	return nil
}
