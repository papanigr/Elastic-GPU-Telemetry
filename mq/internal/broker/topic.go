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
	
	// Metrics for graceful degradation monitoring
	droppedCount   int64  // Total messages dropped due to overflow
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

	// Check queue size limit - graceful degradation via drop-oldest strategy
	if len(t.messages) >= t.config.MaxQueueSize {
		// Drop oldest message to make room for new telemetry
		// For real-time telemetry, recent data is more valuable than old data
		if len(t.messages) > 0 {
			oldest := t.messages[0]
			delete(t.messageIndex, oldest.ID)
			t.messages = t.messages[1:]
			// Update indices
			for id, idx := range t.messageIndex {
				t.messageIndex[id] = idx - 1
			}
			t.droppedCount++
			t.logger.Warn().
				Str("dropped_id", oldest.ID).
				Int64("total_dropped", t.droppedCount).
				Int("queue_size", t.config.MaxQueueSize).
				Msg("Queue full, dropped oldest message (graceful degradation)")
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

	// Calculate queue utilization percentage
	utilization := ""
	if t.config.MaxQueueSize > 0 {
		pct := float64(len(t.messages)) / float64(t.config.MaxQueueSize) * 100
		utilization = fmt.Sprintf("%.1f%%", pct)
	}

	return &models.TopicStats{
		Topic:            t.name,
		QueueSize:        len(t.messages),
		MaxSize:          t.config.MaxQueueSize,
		PendingMessages:  pending,
		ConsumerGroups:   len(t.consumerGroups),
		TotalConsumers:   totalConsumers,
		OldestMessageAge: oldestAge,
		DroppedMessages:  t.droppedCount,
		QueueUtilization: utilization,
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

// CleanupResult holds the result of a cleanup operation.
type CleanupResult struct {
	Removed          int
	Redelivered      int
	MoveToDLQ        []*models.Message // Messages that should be moved to DLQ
	ReplayToOriginal []*models.Message // DLQ messages that should be replayed to original topic
	MarkedDead       int               // DLQ messages marked as dead
}

// Cleanup removes old messages and redelivers stale messages.
// Returns messages that should be moved to DLQ or replayed to original topic.
func (t *Topic) Cleanup(maxAge, ackTimeout time.Duration, maxRetries, dlqMaxRetries int, dlqRetryDelay time.Duration, dlqEnabled bool) CleanupResult {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	result := CleanupResult{
		MoveToDLQ:        make([]*models.Message, 0),
		ReplayToOriginal: make([]*models.Message, 0),
	}

	// Process messages
	newMessages := make([]*models.Message, 0, len(t.messages))
	newIndex := make(map[string]int)

	for _, msg := range t.messages {
		// Remove old acked messages
		if msg.State == models.Acked {
			result.Removed++
			continue
		}

		// Remove messages older than maxAge (but not DLQ messages)
		if !msg.IsDLQ && now.Sub(msg.Timestamp) > maxAge {
			result.Removed++
			continue
		}

		// Handle DLQ messages
		if msg.IsDLQ {
			// Skip dead messages (they stay until admin action)
			if msg.State == models.Dead {
				newIndex[msg.ID] = len(newMessages)
				newMessages = append(newMessages, msg)
				continue
			}

			// Auto-replay DLQ messages back to original topic after delay
			if msg.State == models.Pending && now.Sub(msg.LastFailedAt) > dlqRetryDelay {
				// Check if we've exceeded DLQ max retries
				if msg.DLQRetryCount >= dlqMaxRetries {
					// Mark as dead - requires admin action
					msg.State = models.Dead
					msg.LastFailedAt = now
					result.MarkedDead++
					t.logger.Warn().
						Str("message_id", msg.ID).
						Int("dlq_retry_count", msg.DLQRetryCount).
						Msg("DLQ message marked as dead after max auto-replays, requires admin action")
					newIndex[msg.ID] = len(newMessages)
					newMessages = append(newMessages, msg)
				} else {
					// Auto-replay to original topic
					msg.DLQRetryCount++
					result.ReplayToOriginal = append(result.ReplayToOriginal, msg)
					t.logger.Info().
						Str("message_id", msg.ID).
						Str("original_topic", msg.OriginalTopic).
						Int("dlq_retry_count", msg.DLQRetryCount).
						Msg("Auto-replaying DLQ message to original topic")
					// Don't add to newMessages - it will be removed from DLQ
				}
				continue
			}

			newIndex[msg.ID] = len(newMessages)
			newMessages = append(newMessages, msg)
			continue
		}

		// Handle regular messages - Redeliver if not acked in time
		if msg.State == models.Delivered && now.Sub(msg.DeliveredAt) > ackTimeout {
			msg.RetryCount++
			
			// Track first failure
			if msg.FirstFailedAt.IsZero() {
				msg.FirstFailedAt = now
			}
			msg.LastFailedAt = now

			// Check if should move to DLQ
			if dlqEnabled && msg.RetryCount >= maxRetries {
				// Check if this is a replayed message that has exceeded DLQ retries
				if msg.DLQRetryCount >= dlqMaxRetries {
					// Mark as dead immediately - it's been replayed max times and still failing
					msg.IsDLQ = true
					msg.State = models.Dead
					msg.OriginalTopic = t.name
					result.MoveToDLQ = append(result.MoveToDLQ, msg)
					result.MarkedDead++
					
					t.logger.Warn().
						Str("message_id", msg.ID).
						Int("retry_count", msg.RetryCount).
						Int("dlq_retry_count", msg.DLQRetryCount).
						Msg("Message exceeded max retries after DLQ replays, marked as dead")
					continue
				}

				// Prepare for DLQ
				msg.OriginalTopic = t.name
				msg.IsDLQ = true
				msg.State = models.Pending
				msg.DeliverTo = ""
				msg.DeliveredAt = time.Time{}
				result.MoveToDLQ = append(result.MoveToDLQ, msg)
				
				t.logger.Warn().
					Str("message_id", msg.ID).
					Int("retry_count", msg.RetryCount).
					Int("dlq_retry_count", msg.DLQRetryCount).
					Msg("Message exceeded max retries, moving to DLQ")
				continue // Don't add to this topic's messages
			}

			// Redeliver
			msg.State = models.Pending
			msg.DeliverTo = ""
			msg.DeliveredAt = time.Time{}
			result.Redelivered++
		}

		newIndex[msg.ID] = len(newMessages)
		newMessages = append(newMessages, msg)
	}

	t.messages = newMessages
	t.messageIndex = newIndex

	if result.Removed > 0 || result.Redelivered > 0 || len(result.MoveToDLQ) > 0 || len(result.ReplayToOriginal) > 0 {
		t.logger.Debug().
			Int("removed", result.Removed).
			Int("redelivered", result.Redelivered).
			Int("moved_to_dlq", len(result.MoveToDLQ)).
			Int("replay_to_original", len(result.ReplayToOriginal)).
			Int("marked_dead", result.MarkedDead).
			Msg("Cleanup completed")
	}

	return result
}

// GetDLQStats returns DLQ-specific statistics.
func (t *Topic) GetDLQStats(originalTopic string) *models.DLQStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	pending := 0
	dead := 0
	var oldestTime time.Time

	for _, msg := range t.messages {
		if msg.State == models.Pending {
			pending++
		} else if msg.State == models.Dead {
			dead++
		}
		if oldestTime.IsZero() || msg.Timestamp.Before(oldestTime) {
			oldestTime = msg.Timestamp
		}
	}

	var oldestAge string
	if !oldestTime.IsZero() {
		oldestAge = time.Since(oldestTime).Round(time.Second).String()
	}

	return &models.DLQStats{
		Topic:           t.name,
		OriginalTopic:   originalTopic,
		TotalMessages:   len(t.messages),
		PendingMessages: pending,
		DeadMessages:    dead,
		OldestMessage:   oldestAge,
	}
}

// GetDLQMessages returns DLQ messages with failure details.
func (t *Topic) GetDLQMessages(limit int) []*models.DLQMessage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 || limit > len(t.messages) {
		limit = len(t.messages)
	}

	result := make([]*models.DLQMessage, 0, limit)
	for i := 0; i < limit && i < len(t.messages); i++ {
		msg := t.messages[i]
		result = append(result, &models.DLQMessage{
			ID:            msg.ID,
			OriginalTopic: msg.OriginalTopic,
			Payload:       msg.Payload,
			Timestamp:     msg.Timestamp,
			RetryCount:    msg.RetryCount,
			DLQRetryCount: msg.DLQRetryCount,
			LastError:     msg.LastError,
			FirstFailedAt: msg.FirstFailedAt,
			LastFailedAt:  msg.LastFailedAt,
			State:         msg.State.String(),
		})
	}
	return result
}

// GetMessagesForReplay returns messages that can be replayed.
// If messageIDs is nil, returns all pending messages.
// If force is true, also includes dead messages.
func (t *Topic) GetMessagesForReplay(messageIDs []string, force bool) []*models.Message {
	t.mu.Lock()
	defer t.mu.Unlock()

	var result []*models.Message
	idSet := make(map[string]bool)
	for _, id := range messageIDs {
		idSet[id] = true
	}

	for _, msg := range t.messages {
		// Filter by message IDs if provided
		if len(messageIDs) > 0 && !idSet[msg.ID] {
			continue
		}

		// Only replay pending or (if force) dead messages
		if msg.State == models.Pending || (force && msg.State == models.Dead) {
			result = append(result, msg)
		}
	}

	return result
}

// RemoveMessages removes specified messages from the topic.
func (t *Topic) RemoveMessages(messageIDs []string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	removed := 0
	idSet := make(map[string]bool)
	for _, id := range messageIDs {
		idSet[id] = true
	}

	newMessages := make([]*models.Message, 0, len(t.messages))
	newIndex := make(map[string]int)

	for _, msg := range t.messages {
		if idSet[msg.ID] {
			removed++
			continue
		}
		newIndex[msg.ID] = len(newMessages)
		newMessages = append(newMessages, msg)
	}

	t.messages = newMessages
	t.messageIndex = newIndex

	return removed
}
