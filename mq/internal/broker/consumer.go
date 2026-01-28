package broker

import (
	"sync"
	"time"

	"github.com/gpu-telemetry-pipeline/mq/pkg/models"
	"github.com/rs/zerolog"
)

// Consumer represents a consumer in a group.
type Consumer struct {
	ID          string
	LastSeen    time.Time
	PendingAcks int
}

// ConsumerGroup manages consumers for load balancing.
type ConsumerGroup struct {
	name      string
	consumers map[string]*Consumer
	order     []string // For round-robin
	nextIndex int
	mu        sync.RWMutex
	logger    zerolog.Logger
}

// NewConsumerGroup creates a new consumer group.
func NewConsumerGroup(name string, logger zerolog.Logger) *ConsumerGroup {
	return &ConsumerGroup{
		name:      name,
		consumers: make(map[string]*Consumer),
		order:     make([]string, 0),
		logger:    logger.With().Str("group", name).Logger(),
	}
}

// AddConsumer adds a consumer to the group.
func (g *ConsumerGroup) AddConsumer(consumerID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.consumers[consumerID]; exists {
		// Update existing consumer
		g.consumers[consumerID].LastSeen = time.Now()
		return
	}

	g.consumers[consumerID] = &Consumer{
		ID:       consumerID,
		LastSeen: time.Now(),
	}
	g.order = append(g.order, consumerID)

	g.logger.Info().Str("consumer_id", consumerID).Msg("Consumer added to group")
}

// RemoveConsumer removes a consumer from the group.
func (g *ConsumerGroup) RemoveConsumer(consumerID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.consumers, consumerID)

	// Remove from order
	newOrder := make([]string, 0, len(g.order)-1)
	for _, id := range g.order {
		if id != consumerID {
			newOrder = append(newOrder, id)
		}
	}
	g.order = newOrder

	// Adjust nextIndex if needed
	if g.nextIndex >= len(g.order) {
		g.nextIndex = 0
	}

	g.logger.Info().Str("consumer_id", consumerID).Msg("Consumer removed from group")
}

// UpdateConsumer updates the last seen time for a consumer.
func (g *ConsumerGroup) UpdateConsumer(consumerID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if c, exists := g.consumers[consumerID]; exists {
		c.LastSeen = time.Now()
	}
}

// NextConsumer returns the next consumer in round-robin order.
func (g *ConsumerGroup) NextConsumer() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.order) == 0 {
		return ""
	}

	consumer := g.order[g.nextIndex]
	g.nextIndex = (g.nextIndex + 1) % len(g.order)

	return consumer
}

// ConsumerCount returns the number of consumers in the group.
func (g *ConsumerGroup) ConsumerCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.consumers)
}

// GetConsumerInfo returns information about all consumers.
func (g *ConsumerGroup) GetConsumerInfo() []*models.ConsumerInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]*models.ConsumerInfo, 0, len(g.consumers))
	for _, c := range g.consumers {
		result = append(result, &models.ConsumerInfo{
			ID:          c.ID,
			Group:       g.name,
			Connected:   time.Since(c.LastSeen) < 30*time.Second,
			LastSeen:    c.LastSeen,
			PendingAcks: c.PendingAcks,
		})
	}
	return result
}
