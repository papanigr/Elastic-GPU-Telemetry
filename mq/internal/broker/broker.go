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
	for _, t := range b.topics {
		topics = append(topics, t)
	}
	b.mu.RUnlock()

	for _, topic := range topics {
		topic.Cleanup(b.config.MaxMessageAge, b.config.AckTimeout)
	}
}

// Close shuts down the broker.
func (b *Broker) Close() error {
	b.cancel()
	b.logger.Info().Msg("Broker closed")
	return nil
}
