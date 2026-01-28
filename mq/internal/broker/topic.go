package broker

import (
	"fmt"
	"sync"
	"time"

	"github.com/gpu-telemetry-pipeline/mq/internal/config"
	"github.com/gpu-telemetry-pipeline/mq/pkg/models"
	"github.com/rs/zerolog"
)

// Topic represents a message topic with its queue and consumer groups.
type Topic struct {
	name           string
	config         config.Config
	messages       []*models.Message
	messageIndex   map[string]int // message ID -> index in messages slice
	consumerGroups map[string]*ConsumerGroup
	mu             sync.RWMutex
	logger         zerolog.Logger
}

// NewTopic creates a new topic.
func NewTopic(name string, cfg config.Config, logger zerolog.Logger) *Topic {
	return &Topic{
		name:           name,
		config:         cfg,
		messages:       make([]*models.Message, 0, cfg.MaxQueueSize),
		messageIndex:   make(map[string]int),
		consumerGroups: make(map[string]*ConsumerGroup),
		logger:         logger.With().Str("topic", name).Logger(),
	}
}

// Publish adds a message to the topic.
func (t *Topic) Publish(msg *models.Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check queue size limit
	if len(t.messages) >= t.config.MaxQueueSize {
		// Drop oldest message
		if len(t.messages) > 0 {
			oldest := t.messages[0]
			delete(t.messageIndex, oldest.ID)
			t.messages = t.messages[1:]
			// Update indices
			for id, idx := range t.messageIndex {
				t.messageIndex[id] = idx - 1
			}
			t.logger.Warn().Str("dropped_id", oldest.ID).Msg("Queue full, dropped oldest message")
		}
	}

	msg.State = models.Pending
	msg.Topic = t.name
	t.messageIndex[msg.ID] = len(t.messages)
	t.messages = append(t.messages, msg)

	t.logger.Debug().Str("message_id", msg.ID).Msg("Message published")
	return nil
}

// Subscribe adds a consumer to a consumer group.
func (t *Topic) Subscribe(consumerID, groupName string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	group, exists := t.consumerGroups[groupName]
	if !exists {
		group = NewConsumerGroup(groupName, t.logger)
		t.consumerGroups[groupName] = group
	}

	group.AddConsumer(consumerID)
	t.logger.Info().
		Str("consumer_id", consumerID).
		Str("group", groupName).
		Msg("Consumer subscribed")

	return nil
}

// Consume retrieves pending messages for a consumer.
func (t *Topic) Consume(consumerID, groupName string, maxMessages int) ([]*models.Message, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	group, exists := t.consumerGroups[groupName]
	if !exists {
		return nil, fmt.Errorf("consumer group %s not found", groupName)
	}

	// Update consumer last seen
	group.UpdateConsumer(consumerID)

	if maxMessages <= 0 {
		maxMessages = 10
	}

	result := make([]*models.Message, 0, maxMessages)
	now := time.Now()

	for _, msg := range t.messages {
		if len(result) >= maxMessages {
			break
		}

		// Only deliver pending messages assigned to this consumer
		if msg.State == models.Pending {
			// Assign to next consumer in group (round-robin)
			assignedConsumer := group.NextConsumer()
			if assignedConsumer == consumerID {
				msg.State = models.Delivered
				msg.DeliverTo = consumerID
				msg.DeliveredAt = now
				result = append(result, msg)
			}
		}
	}

	if len(result) > 0 {
		t.logger.Debug().
			Str("consumer_id", consumerID).
			Int("count", len(result)).
			Msg("Messages delivered")
	}

	return result, nil
}

// Ack acknowledges messages from a consumer.
func (t *Topic) Ack(consumerID string, messageIDs []string) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	acked := 0
	for _, msgID := range messageIDs {
		idx, exists := t.messageIndex[msgID]
		if !exists {
			continue
		}

		msg := t.messages[idx]
		if msg.DeliverTo == consumerID && msg.State == models.Delivered {
			msg.State = models.Acked
			acked++
		}
	}

	if acked > 0 {
		t.logger.Debug().
			Str("consumer_id", consumerID).
			Int("acked", acked).
			Msg("Messages acknowledged")
	}

	return acked, nil
}

// GetStats returns topic statistics.
func (t *Topic) GetStats() *models.TopicStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	pending := 0
	var oldestTime time.Time
	for _, msg := range t.messages {
		if msg.State == models.Pending {
			pending++
			if oldestTime.IsZero() || msg.Timestamp.Before(oldestTime) {
				oldestTime = msg.Timestamp
			}
		}
	}

	totalConsumers := 0
	for _, g := range t.consumerGroups {
		totalConsumers += g.ConsumerCount()
	}

	var oldestAge string
	if !oldestTime.IsZero() {
		oldestAge = time.Since(oldestTime).Round(time.Second).String()
	}

	return &models.TopicStats{
		Topic:           t.name,
		QueueSize:       len(t.messages),
		MaxSize:         t.config.MaxQueueSize,
		PendingMessages: pending,
		ConsumerGroups:  len(t.consumerGroups),
		TotalConsumers:  totalConsumers,
		OldestMessageAge: oldestAge,
	}
}

// GetMessages returns messages for admin inspection.
func (t *Topic) GetMessages(limit int) []*models.Message {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 || limit > len(t.messages) {
		limit = len(t.messages)
	}

	result := make([]*models.Message, limit)
	copy(result, t.messages[:limit])
	return result
}

// DeleteMessage removes a specific message.
func (t *Topic) DeleteMessage(messageID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	idx, exists := t.messageIndex[messageID]
	if !exists {
		return false
	}

	// Remove from slice
	t.messages = append(t.messages[:idx], t.messages[idx+1:]...)
	delete(t.messageIndex, messageID)

	// Update indices
	for id, i := range t.messageIndex {
		if i > idx {
			t.messageIndex[id] = i - 1
		}
	}

	t.logger.Info().Str("message_id", messageID).Msg("Message deleted")
	return true
}

// PurgeMessages removes all messages from the topic.
func (t *Topic) PurgeMessages() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	count := len(t.messages)
	t.messages = make([]*models.Message, 0, t.config.MaxQueueSize)
	t.messageIndex = make(map[string]int)

	t.logger.Info().Int("count", count).Msg("Topic purged")
	return count
}

// GetConsumers returns information about consumers.
func (t *Topic) GetConsumers() []*models.ConsumerInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*models.ConsumerInfo
	for _, group := range t.consumerGroups {
		result = append(result, group.GetConsumerInfo()...)
	}
	return result
}

// Cleanup removes old messages and redelivers stale messages.
func (t *Topic) Cleanup(maxAge, ackTimeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	removed := 0
	redelivered := 0

	// Process messages
	newMessages := make([]*models.Message, 0, len(t.messages))
	newIndex := make(map[string]int)

	for _, msg := range t.messages {
		// Remove old acked messages
		if msg.State == models.Acked {
			removed++
			continue
		}

		// Remove messages older than maxAge
		if now.Sub(msg.Timestamp) > maxAge {
			removed++
			continue
		}

		// Redeliver messages that weren't acked in time
		if msg.State == models.Delivered && now.Sub(msg.DeliveredAt) > ackTimeout {
			msg.State = models.Pending
			msg.DeliverTo = ""
			msg.DeliveredAt = time.Time{}
			redelivered++
		}

		newIndex[msg.ID] = len(newMessages)
		newMessages = append(newMessages, msg)
	}

	t.messages = newMessages
	t.messageIndex = newIndex

	if removed > 0 || redelivered > 0 {
		t.logger.Debug().
			Int("removed", removed).
			Int("redelivered", redelivered).
			Msg("Cleanup completed")
	}
}
